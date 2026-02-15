require('dotenv').config();

// Parse server configurations from SERVERS environment variable
// Format: PORT:UPSTREAM_URL,PORT:UPSTREAM_URL,...
// Example: 3000:http://localhost:8080,3001:http://localhost:8081
function parseServers() {
  if (process.env.SERVERS) {
    return process.env.SERVERS.split(',').map(serverConfig => {
      const [port, upstreamUrl] = serverConfig.trim().split(':');
      // Rejoin the URL parts (http:// was split)
      const url = serverConfig.trim().substring(port.length + 1);
      return {
        port: parseInt(port, 10),
        upstreamUrl: url
      };
    });
  }
  // Fallback to single server configuration for backward compatibility
  return [{
    port: parseInt(process.env.PORT || '3000', 10),
    upstreamUrl: process.env.UPSTREAM_URL || 'http://localhost:8080'
  }];
}

module.exports = {
  servers: parseServers(),
  redirectUrl: process.env.REDIRECT_URL || 'http://localhost:3001/login',
  jwtKeyPath: process.env.JWT_KEY_PATH,
  jwtCookieName: process.env.JWT_COOKIE_NAME || 'auth_token',
  publicPaths: process.env.PUBLIC_PATHS ? process.env.PUBLIC_PATHS.split(',').map(p => p.trim()) : []
};
