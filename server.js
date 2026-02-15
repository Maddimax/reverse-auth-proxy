const express = require('express');
const { createProxyMiddleware } = require('http-proxy-middleware');
const jwt = require('jsonwebtoken');
const cookieParser = require('cookie-parser');
const fs = require('fs');
const path = require('path');
const config = require('./config');

const app = express();

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

// Middleware
app.use(cookieParser());

// Health check endpoint (bypasses authentication)
app.get('/health', (req, res) => {
  res.status(200).json({ status: 'ok', timestamp: new Date().toISOString() });
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
app.use('/', createProxyMiddleware({
  target: config.upstreamUrl,
  changeOrigin: true,
  onProxyReq: (proxyReq, req, res) => {
    // Add user information to request headers if available
    if (req.user) {
      proxyReq.setHeader('X-User-Id', req.user.userId || req.user.sub || '');
      proxyReq.setHeader('X-User-Email', req.user.email || '');
    }
    console.log(`Proxying ${req.method} ${req.path} to ${config.upstreamUrl}`);
  },
  onError: (err, req, res) => {
    console.error('Proxy error:', err);
    res.status(502).json({ error: 'Bad Gateway', message: 'Failed to connect to upstream server' });
  }
}));

// Error handling
app.use((err, req, res, next) => {
  console.error('Server error:', err);
  res.status(500).json({ error: 'Internal Server Error' });
});

// Start server
app.listen(config.port, () => {
  console.log(`Reverse Auth Proxy running on port ${config.port}`);
  console.log(`Proxying to: ${config.upstreamUrl}`);
  console.log(`Redirect URL: ${config.redirectUrl}`);
  console.log(`JWT Cookie: ${config.jwtCookieName}`);
  if (config.publicPaths.length > 0) {
    console.log(`Public paths: ${config.publicPaths.join(', ')}`);
  }
});
