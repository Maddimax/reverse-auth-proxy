const express = require('express');
const { createProxyMiddleware } = require('http-proxy-middleware');
const jwt = require('jsonwebtoken');
const cookieParser = require('cookie-parser');
const fs = require('fs');
const path = require('path');
const http = require('http');
const config = require('./config');

// Load JWT key from file
let jwtKey;
try {
  if (!config.jwtKeyPath) {
    console.error('ERROR: JWT_KEY_PATH is not configured. Please set it in your .env file.');
    process.exit(1);
  }
  const keyPath = path.resolve(config.jwtKeyPath);
  jwtKey = fs.readFileSync(keyPath, 'utf8');
  console.log(`Loaded JWT key from: ${keyPath}`);
} catch (error) {
  console.error(`ERROR: Failed to load JWT key from ${config.jwtKeyPath}:`, error.message);
  process.exit(1);
}

// Helper function to parse cookies from header string
function parseCookies(cookieHeader) {
  const cookies = {};
  if (cookieHeader) {
    cookieHeader.split(';').forEach(cookie => {
      const parts = cookie.trim().split('=');
      if (parts.length === 2) {
        cookies[parts[0]] = parts[1];
      }
    });
  }
  return cookies;
}

// Helper function to verify JWT for WebSocket upgrades
function verifyWebSocketJWT(req) {
  // Check if path is public
  const url = new URL(req.url, `http://${req.headers.host}`);
  if (config.publicPaths.some(path => url.pathname.startsWith(path))) {
    return true;
  }

  const cookies = parseCookies(req.headers.cookie);
  const token = cookies[config.jwtCookieName];

  if (!token) {
    console.log(`WebSocket: No token found in cookie '${config.jwtCookieName}'`);
    return false;
  }

  try {
    const decoded = jwt.verify(token, jwtKey);
    req.user = decoded;
    return true;
  } catch (error) {
    console.log(`WebSocket: JWT verification failed: ${error.message}`);
    return false;
  }
}

// Function to create a server instance for a specific port/upstream pair
function createServerInstance(serverConfig) {
  const app = express();

  // Middleware
  app.use(cookieParser());

  // Health check endpoint (bypasses authentication)
  app.get('/health', (req, res) => {
    res.status(200).json({ 
      status: 'ok', 
      timestamp: new Date().toISOString(),
      port: serverConfig.port,
      upstream: serverConfig.upstreamUrl
    });
  });

  // JWT verification middleware
  const verifyJWT = (req, res, next) => {
    // Check if path is public
    if (config.publicPaths.some(path => req.path.startsWith(path))) {
      return next();
    }

    const token = req.cookies[config.jwtCookieName];

    if (!token) {
      console.log(`No token found in cookie '${config.jwtCookieName}', redirecting to ${config.redirectUrl}`);
      return res.redirect(config.redirectUrl);
    }

    try {
      const decoded = jwt.verify(token, jwtKey);
      req.user = decoded;
      next();
    } catch (error) {
      console.log(`JWT verification failed: ${error.message}, redirecting to ${config.redirectUrl}`);
      return res.redirect(config.redirectUrl);
    }
  };

  // Apply JWT verification to all routes
  app.use(verifyJWT);

  // Reverse proxy middleware
  const proxyMiddleware = createProxyMiddleware({
    target: serverConfig.upstreamUrl,
    changeOrigin: true,
    ws: true, // Enable WebSocket support
    onProxyReq: (proxyReq, req, res) => {
      // Add user information to request headers if available
      if (req.user) {
        proxyReq.setHeader('X-User-Id', req.user.userId || req.user.sub || '');
        proxyReq.setHeader('X-User-Email', req.user.email || '');
      }
      console.log(`[Port ${serverConfig.port}] Proxying ${req.method} ${req.path} to ${serverConfig.upstreamUrl}`);
    },
    onProxyReqWs: (proxyReq, req, socket, options, head) => {
      // Add user information to WebSocket upgrade request headers if available
      if (req.user) {
        proxyReq.setHeader('X-User-Id', req.user.userId || req.user.sub || '');
        proxyReq.setHeader('X-User-Email', req.user.email || '');
      }
      console.log(`[Port ${serverConfig.port}] Proxying WebSocket upgrade for ${req.url} to ${serverConfig.upstreamUrl}`);
    },
    onError: (err, req, res) => {
      console.error(`[Port ${serverConfig.port}] Proxy error:`, err);
      if (res.writeHead) {
        res.status(502).json({ error: 'Bad Gateway', message: 'Failed to connect to upstream server' });
      }
    }
  });

  app.use('/', proxyMiddleware);

  // Error handling
  app.use((err, req, res, next) => {
    console.error(`[Port ${serverConfig.port}] Server error:`, err);
    res.status(500).json({ error: 'Internal Server Error' });
  });

  // Create HTTP server
  const server = http.createServer(app);

  // Handle WebSocket upgrades with JWT verification
  server.on('upgrade', (req, socket, head) => {
    if (!verifyWebSocketJWT(req)) {
      socket.write('HTTP/1.1 401 Unauthorized\r\n\r\n');
      socket.destroy();
      return;
    }

    // Let the proxy middleware handle the upgrade
    proxyMiddleware.upgrade(req, socket, head);
  });

  return { server, port: serverConfig.port, upstreamUrl: serverConfig.upstreamUrl };
}

// Create and start all server instances
const servers = config.servers.map(serverConfig => {
  const instance = createServerInstance(serverConfig);
  instance.server.listen(instance.port, () => {
    console.log(`Reverse Auth Proxy running on port ${instance.port}`);
    console.log(`  -> Proxying to: ${instance.upstreamUrl}`);
    console.log(`  -> Redirect URL: ${config.redirectUrl}`);
    console.log(`  -> JWT Cookie: ${config.jwtCookieName}`);
    console.log(`  -> WebSocket support: enabled`);
    if (config.publicPaths.length > 0) {
      console.log(`  -> Public paths: ${config.publicPaths.join(', ')}`);
    }
  });
  return instance;
});

console.log(`\nTotal servers started: ${servers.length}`);
