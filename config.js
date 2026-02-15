require('dotenv').config();

module.exports = {
  port: process.env.PORT || 3000,
  upstreamUrl: process.env.UPSTREAM_URL || 'http://localhost:8080',
  redirectUrl: process.env.REDIRECT_URL || 'http://localhost:3001/login',
  jwtKeyPath: process.env.JWT_KEY_PATH,
  jwtCookieName: process.env.JWT_COOKIE_NAME || 'auth_token',
  publicPaths: process.env.PUBLIC_PATHS ? process.env.PUBLIC_PATHS.split(',').map(p => p.trim()) : []
};
