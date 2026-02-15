package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	jwtPublicKey interface{}
	config       *Config
)

// Claims represents the JWT claims
type Claims struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Sub    string `json:"sub"`
	jwt.RegisteredClaims
}

// parseCookies extracts cookies from the Cookie header
func parseCookies(cookieHeader string) map[string]string {
	cookies := make(map[string]string)
	if cookieHeader == "" {
		return cookies
	}

	for _, cookie := range strings.Split(cookieHeader, ";") {
		parts := strings.SplitN(strings.TrimSpace(cookie), "=", 2)
		if len(parts) == 2 {
			cookies[parts[0]] = parts[1]
		}
	}
	return cookies
}

// verifyJWT verifies the JWT token and returns the claims
func verifyJWT(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtPublicKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// isPublicPath checks if the given path matches any public path patterns
func isPublicPath(path string) bool {
	for _, publicPath := range config.PublicPaths {
		if strings.HasPrefix(path, publicPath) {
			return true
		}
	}
	return false
}

// authMiddleware handles JWT verification
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if path is public
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Get token from cookie
		cookie, err := r.Cookie(config.JWTCookieName)
		if err != nil {
			log.Printf("No token found in cookie '%s', redirecting to %s", config.JWTCookieName, config.RedirectURL)
			http.Redirect(w, r, config.RedirectURL, http.StatusFound)
			return
		}

		// Verify JWT
		claims, err := verifyJWT(cookie.Value)
		if err != nil {
			// Token exists but failed validation - redirect to JWT timeout/refresh URL
			authURL := config.JWTTimeoutURL + url.QueryEscape(r.RequestURI)
			log.Printf("JWT invalid, redirecting to %s", authURL)
			http.Redirect(w, r, authURL, http.StatusFound)
			return
		}

		// Store claims in context
		ctx := context.WithValue(r.Context(), "claims", claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// createReverseProxy creates a reverse proxy for the given target URL
func createReverseProxy(targetURL string, port int) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Custom director to add user headers
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// Add user information from claims if available
		if claims, ok := req.Context().Value("claims").(*Claims); ok {
			userID := claims.UserID
			if userID == "" {
				userID = claims.Sub
			}
			req.Header.Set("X-User-Id", userID)
			req.Header.Set("X-User-Email", claims.Email)
		}

		log.Printf("[Port %d] Proxying %s %s to %s", port, req.Method, req.URL.Path, targetURL)
	}

	// Custom error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("[Port %d] Proxy error: %v", port, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, `{"error":"Bad Gateway","message":"Failed to connect to upstream server"}`)
	}

	return proxy, nil
}

// healthCheckHandler returns a health check handler
func healthCheckHandler(port int, upstreamURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","timestamp":"%s","port":%d,"upstream":"%s"}`,
			time.Now().Format(time.RFC3339), port, upstreamURL)
	}
}

// handleWebSocket handles WebSocket upgrade requests with JWT verification
func handleWebSocket(proxy *httputil.ReverseProxy, port int, upstreamURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if it's a WebSocket upgrade request
		if !isWebSocketUpgrade(r) {
			proxy.ServeHTTP(w, r)
			return
		}

		// Check if path is public
		if !isPublicPath(r.URL.Path) {
			// Verify JWT for WebSocket connections
			cookieHeader := r.Header.Get("Cookie")
			cookies := parseCookies(cookieHeader)
			token, ok := cookies[config.JWTCookieName]

			if !ok {
				log.Printf("WebSocket: No token found in cookie '%s'", config.JWTCookieName)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			claims, err := verifyJWT(token)
			if err != nil {
				log.Printf("WebSocket: JWT validation failed: %v", err)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Store claims in context for the proxy
			ctx := context.WithValue(r.Context(), "claims", claims)
			r = r.WithContext(ctx)
		}

		log.Printf("[Port %d] Proxying WebSocket upgrade for %s to %s", port, r.URL.Path, upstreamURL)

		// Hijack the connection
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "WebSocket not supported", http.StatusInternalServerError)
			return
		}

		// Parse target URL
		target, err := url.Parse(upstreamURL)
		if err != nil {
			http.Error(w, "Invalid upstream URL", http.StatusInternalServerError)
			return
		}

		// Connect to upstream
		targetConn, err := net.Dial("tcp", target.Host)
		if err != nil {
			http.Error(w, "Failed to connect to upstream", http.StatusBadGateway)
			return
		}
		defer targetConn.Close()

		// Hijack client connection
		clientConn, _, err := hijacker.Hijack()
		if err != nil {
			http.Error(w, "Failed to hijack connection", http.StatusInternalServerError)
			return
		}
		defer clientConn.Close()

		// Modify request to include user headers
		if claims, ok := r.Context().Value("claims").(*Claims); ok {
			userID := claims.UserID
			if userID == "" {
				userID = claims.Sub
			}
			r.Header.Set("X-User-Id", userID)
			r.Header.Set("X-User-Email", claims.Email)
		}

		// Write the upgrade request to upstream
		err = r.Write(targetConn)
		if err != nil {
			log.Printf("Failed to write upgrade request: %v", err)
			return
		}

		// Copy data bidirectionally
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			io.Copy(targetConn, clientConn)
		}()

		go func() {
			defer wg.Done()
			io.Copy(clientConn, targetConn)
		}()

		wg.Wait()
	}
}

// isWebSocketUpgrade checks if the request is a WebSocket upgrade request
func isWebSocketUpgrade(r *http.Request) bool {
	return strings.ToLower(r.Header.Get("Connection")) == "upgrade" &&
		strings.ToLower(r.Header.Get("Upgrade")) == "websocket"
}

// startServer starts a server instance on the specified port
func startServer(serverConfig ServerConfig, wg *sync.WaitGroup) *http.Server {
	defer wg.Done()

	// Create reverse proxy
	proxy, err := createReverseProxy(serverConfig.UpstreamURL, serverConfig.Port)
	if err != nil {
		log.Fatalf("Failed to create proxy for port %d: %v", serverConfig.Port, err)
	}

	// Create router
	mux := http.NewServeMux()

	// Health check endpoint (bypasses authentication)
	mux.HandleFunc("/health", healthCheckHandler(serverConfig.Port, serverConfig.UpstreamURL))

	// WebSocket and HTTP proxy handler with auth middleware
	mux.HandleFunc("/", handleWebSocket(proxy, serverConfig.Port, serverConfig.UpstreamURL))

	// Apply auth middleware to all routes
	handler := authMiddleware(mux)

	// Create server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", serverConfig.Port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Reverse Auth Proxy running on port %d", serverConfig.Port)
		log.Printf("  -> Proxying to: %s", serverConfig.UpstreamURL)
		log.Printf("  -> Redirect URL: %s", config.RedirectURL)
		log.Printf("  -> JWT Cookie: %s", config.JWTCookieName)
		log.Printf("  -> WebSocket support: enabled")
		if len(config.PublicPaths) > 0 {
			log.Printf("  -> Public paths: %s", strings.Join(config.PublicPaths, ", "))
		}

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server on port %d failed: %v", serverConfig.Port, err)
		}
	}()

	return server
}

// loadJWTKey loads the JWT public key from the specified file
func loadJWTKey(keyPath string) (interface{}, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read JWT key file: %w", err)
	}

	// Try to parse as PEM
	block, _ := pem.Decode(keyData)
	if block != nil {
		// Try RSA public key
		if block.Type == "PUBLIC KEY" || block.Type == "RSA PUBLIC KEY" {
			pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
			if err != nil {
				// Try parsing as PKCS1 RSA public key
				rsaKey, err2 := x509.ParsePKCS1PublicKey(block.Bytes)
				if err2 != nil {
					return nil, fmt.Errorf("failed to parse public key: %w", err)
				}
				return rsaKey, nil
			}

			switch k := pubKey.(type) {
			case *rsa.PublicKey:
				return k, nil
			default:
				return pubKey, nil
			}
		}
	}

	// If not PEM or parsing failed, treat as symmetric secret
	return keyData, nil
}

func main() {
	// Load configuration
	var err error
	config, err = LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Load JWT key
	if config.JWTKeyPath == "" {
		log.Fatal("ERROR: JWT_KEY_PATH is not configured. Please set it in your environment variables.")
	}

	jwtPublicKey, err = loadJWTKey(config.JWTKeyPath)
	if err != nil {
		log.Fatalf("ERROR: Failed to load JWT key from %s: %v", config.JWTKeyPath, err)
	}
	log.Printf("Loaded JWT key from: %s", config.JWTKeyPath)

	// Start all servers
	var wg sync.WaitGroup
	servers := make([]*http.Server, len(config.Servers))

	for i, serverConfig := range config.Servers {
		wg.Add(1)
		servers[i] = startServer(serverConfig, &wg)
	}

	log.Printf("\nTotal servers started: %d", len(config.Servers))

	// Wait for interrupt signal to gracefully shut down
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down servers...")

	// Gracefully shut down all servers
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, server := range servers {
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}

	log.Println("All servers stopped")
}
