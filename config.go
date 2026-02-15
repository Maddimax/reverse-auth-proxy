package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ServerConfig represents configuration for a single server instance
type ServerConfig struct {
	Port        int
	UpstreamURL string
}

// Config represents the application configuration
type Config struct {
	Servers       []ServerConfig
	RedirectURL   string
	JWTTimeoutURL string
	JWTKeyPath    string
	JWTCookieName string
	PublicPaths   []string
}

// parseServers parses the SERVERS environment variable
// Format: PORT:UPSTREAM_URL,PORT:UPSTREAM_URL,...
// Example: 3000:http://localhost:8080,3001:http://localhost:8081
func parseServers() ([]ServerConfig, error) {
	serversEnv := os.Getenv("SERVERS")

	if serversEnv != "" {
		var servers []ServerConfig
		serverConfigs := strings.Split(serversEnv, ",")

		for _, serverConfig := range serverConfigs {
			parts := strings.SplitN(strings.TrimSpace(serverConfig), ":", 2)
			if len(parts) < 2 {
				return nil, fmt.Errorf("invalid server configuration: %s", serverConfig)
			}

			port, err := strconv.Atoi(parts[0])
			if err != nil {
				return nil, fmt.Errorf("invalid port in server configuration: %s", serverConfig)
			}

			// Rejoin the URL parts (http:// was split)
			url := strings.TrimSpace(serverConfig)[len(parts[0])+1:]

			servers = append(servers, ServerConfig{
				Port:        port,
				UpstreamURL: url,
			})
		}

		return servers, nil
	}

	// Fallback to single server configuration for backward compatibility
	portStr := os.Getenv("PORT")
	if portStr == "" {
		portStr = "3000"
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid PORT: %s", portStr)
	}

	upstreamURL := os.Getenv("UPSTREAM_URL")
	if upstreamURL == "" {
		upstreamURL = "http://localhost:8080"
	}

	return []ServerConfig{
		{
			Port:        port,
			UpstreamURL: upstreamURL,
		},
	}, nil
}

// parsePublicPaths parses the PUBLIC_PATHS environment variable
func parsePublicPaths() []string {
	publicPathsEnv := os.Getenv("PUBLIC_PATHS")
	if publicPathsEnv == "" {
		return []string{}
	}

	paths := strings.Split(publicPathsEnv, ",")
	var publicPaths []string
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed != "" {
			publicPaths = append(publicPaths, trimmed)
		}
	}

	return publicPaths
}

// LoadConfig loads the application configuration from environment variables
func LoadConfig() (*Config, error) {
	servers, err := parseServers()
	if err != nil {
		return nil, err
	}

	redirectURL := os.Getenv("REDIRECT_URL")
	if redirectURL == "" {
		redirectURL = "http://localhost:3001/login"
	}

	jwtTimeoutURL := os.Getenv("JWT_TIMEOUT_URL")
	if jwtTimeoutURL == "" {
		jwtTimeoutURL = redirectURL
	}

	jwtKeyPath := os.Getenv("JWT_KEY_PATH")

	jwtCookieName := os.Getenv("JWT_COOKIE_NAME")
	if jwtCookieName == "" {
		jwtCookieName = "auth_token"
	}

	publicPaths := parsePublicPaths()

	return &Config{
		Servers:       servers,
		RedirectURL:   redirectURL,
		JWTTimeoutURL: jwtTimeoutURL,
		JWTKeyPath:    jwtKeyPath,
		JWTCookieName: jwtCookieName,
		PublicPaths:   publicPaths,
	}, nil
}
