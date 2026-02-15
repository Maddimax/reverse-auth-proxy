# Reverse Auth Proxy

A lightweight, high-performance reverse proxy with JWT authentication written in Go. This proxy verifies JWT tokens from cookies before forwarding requests to an upstream server. If authentication fails, users are redirected to a configurable login URL.

## Features

- 🔒 JWT token verification via cookies
- 🔄 Reverse proxy to single or multiple upstream HTTP servers
- 🚪 Multiple ports support - run multiple proxies simultaneously
- 🔌 WebSocket support with JWT authentication
- ↪️ Configurable redirect URL for authentication failures
- 🚀 Simple configuration via environment variables
- 🔓 Support for public paths (bypass authentication)
- 📝 Automatic user info forwarding to upstream via headers
- ⚡ High performance with low memory footprint
- 🐳 Docker support with minimal Alpine-based image

## Installation

### Prerequisites

- Go 1.21 or later (for building from source)
- Docker (for containerized deployment)

### From Source

```bash
# Clone the repository
git clone https://github.com/Maddimax/reverse-auth-proxy.git
cd reverse-auth-proxy

# Download dependencies
go mod download

# Build the application
go build -o reverse-auth-proxy .

# Run
./reverse-auth-proxy
```

### Using Docker

```bash
docker pull maddimax/reverse-auth-proxy:go
```

## Configuration

1. Copy the example environment file:
```bash
cp .env.example .env
```

2. Edit `.env` with your settings:

**Option 1: Multiple Servers (Recommended for multiple upstreams)**

```env
# Multiple server configuration
# Format: PORT:UPSTREAM_URL,PORT:UPSTREAM_URL,...
SERVERS=3000:http://localhost:8080,3001:http://localhost:8081,3002:http://localhost:8082

# Redirect URL when JWT verification fails
REDIRECT_URL=http://localhost:3001/login

# JWT Configuration
# Path to the .pem file containing the JWT public key
JWT_KEY_PATH=./keys/jwt-public.pem
JWT_COOKIE_NAME=auth_token

# Optional: Skip authentication for specific paths (comma-separated)
PUBLIC_PATHS=/health,/public
```

**Option 2: Single Server (Backward compatible)**

```env
# Server port
PORT=3000

# Upstream server to proxy requests to
UPSTREAM_URL=http://localhost:8080

# Redirect URL when JWT verification fails
REDIRECT_URL=http://localhost:3001/login

# JWT Configuration
# Path to the .pem file containing the JWT public key
JWT_KEY_PATH=./keys/jwt-public.pem
JWT_COOKIE_NAME=auth_token

# Optional: Skip authentication for specific paths (comma-separated)
PUBLIC_PATHS=/health,/public
```

### Configuration Options

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SERVERS` | No* | - | Multiple server configs (PORT:URL,PORT:URL,...) |
| `PORT` | No* | 3000 | Port the proxy server listens on (single server) |
| `UPSTREAM_URL` | No* | http://localhost:8080 | Target server to proxy to (single server) |
| `REDIRECT_URL` | No | http://localhost:3001/login | URL to redirect to when auth fails |
| `JWT_TIMEOUT_URL` | No | Same as `REDIRECT_URL` | URL to redirect to when JWT token is invalid/expired. Include parameter name (e.g., `https://example.com/auth?redirect_url=`) - original URI will be appended |
| `JWT_KEY_PATH` | **Yes** | - | Path to .pem file containing JWT public key |
| `JWT_COOKIE_NAME` | No | auth_token | Name of the cookie containing the JWT |
| `PUBLIC_PATHS` | No | [] | Comma-separated list of paths that bypass auth |

*Note: Either use `SERVERS` for multiple servers, OR use `PORT` + `UPSTREAM_URL` for a single server.

### JWT Key Setup

The proxy requires a public key (or shared secret) in PEM format to verify JWT tokens. 

For **asymmetric keys** (RS256/ES256), generate a key pair:

```bash
# Generate private key (used by your auth service to sign tokens)
openssl genrsa -out jwt-private.pem 2048

# Extract public key (used by this proxy to verify tokens)
openssl rsa -in jwt-private.pem -outform PEM -pubout -out jwt-public.pem
```

For **symmetric keys** (HS256), create a secret file:

```bash
mkdir -p keys
echo "your-secret-key-here" > keys/jwt-secret.pem
```

Update your `.env` file to point to the key:
```env
JWT_KEY_PATH=./keys/jwt-public.pem
```

**Note:** For production use, asymmetric keys (RS256) are recommended as they allow the auth service to sign tokens with the private key while the proxy only needs the public key.

### Multiple Servers

The proxy supports running multiple server instances simultaneously, each listening on a different port and proxying to a different upstream server. This is useful when you need to:

- Proxy multiple backend services with the same authentication
- Run different services on different ports with unified JWT authentication
- Create a multi-tenant proxy setup

**Example: Running 3 proxies simultaneously**

```env
SERVERS=3000:http://localhost:8080,3001:http://backend2:8081,3002:http://api.example.com
```

This will create:
- Port 3000 → proxies to http://localhost:8080
- Port 3001 → proxies to http://backend2:8081  
- Port 3002 → proxies to http://api.example.com

All servers share the same JWT authentication configuration and redirect URL. Each server independently verifies JWT tokens and proxies requests to its configured upstream.

## Usage

### Local Development

**Build and run:**

```bash
# Build
go build -o reverse-auth-proxy .

# Run
./reverse-auth-proxy
```

**Using go run (for development):**

```bash
go run .
```

### Docker Deployment

**Build the Docker image:**

```bash
docker build -t reverse-auth-proxy .
```

**Run with Docker:**

```bash
docker run -d \
  -p 3000:3000 \
  -e UPSTREAM_URL=http://backend:8080 \
  -e REDIRECT_URL=http://localhost:3001/login \
  -e JWT_KEY_PATH=/app/keys/jwt-public.pem \
  -e JWT_COOKIE_NAME=auth_token \
  -v $(pwd)/keys/jwt-public.pem:/app/keys/jwt-public.pem:ro \
  --name reverse-auth-proxy \
  reverse-auth-proxy
```

**Run with Docker Compose:**

```bash
docker-compose up -d
```

The `docker-compose.yml` file includes example configuration. Update the environment variables and volume mounts as needed.

**Important for Docker:**
- Mount your JWT key file as a read-only volume
- Ensure the key file has proper permissions (readable by UID 1001)
- The container runs as a non-root user for security

## How It Works

### HTTP Requests

1. **Request arrives** at the proxy server
2. **Cookie check**: The proxy looks for a JWT token in the specified cookie
3. **Verification**:
   - If the token is valid, the request is forwarded to the upstream server
   - If the token is missing or invalid, the user is redirected to the login URL
4. **Headers**: Valid JWT claims (userId, email) are forwarded to upstream as headers:
   - `X-User-Id`: User ID from JWT
   - `X-User-Email`: Email from JWT

### WebSocket Connections

WebSocket connections are also protected by JWT authentication:

1. **WebSocket upgrade request** arrives at the proxy
2. **Cookie check**: The proxy extracts the JWT token from the WebSocket upgrade request cookies
3. **Verification**:
   - If the token is valid, the WebSocket connection is upgraded and proxied to the upstream server
   - If the token is missing or invalid, the connection is rejected with 401 Unauthorized
4. **Headers**: User information is forwarded in the WebSocket upgrade headers

### Public Paths

You can configure certain paths to bypass authentication by setting the `PUBLIC_PATHS` environment variable:

```env
PUBLIC_PATHS=/health,/public,/api/status
```

Requests to these paths will be proxied without JWT verification.

## Example JWT Token

Your authentication service should set a cookie with a valid JWT. Example JWT payload:

```json
{
  "userId": "12345",
  "email": "user@example.com",
  "sub": "user-subject",
  "iat": 1234567890,
  "exp": 1234567890
}
```

## Testing

You can test the proxy with curl:

```bash
# Health check (no authentication required)
# The health endpoint returns info about the current server instance
curl http://localhost:3000/health

# Response includes port and upstream info:
# {"status":"ok","timestamp":"...","port":3000,"upstream":"http://localhost:8080"}

# Without token (should redirect)
curl -i http://localhost:3000/api/test

# With token
curl -i http://localhost:3000/api/test \
  --cookie "auth_token=your.jwt.token"
```

**Testing with multiple servers:**

```bash
# Test each server's health endpoint
curl http://localhost:3000/health
curl http://localhost:3001/health
curl http://localhost:3002/health

# Each server independently handles requests to its upstream
curl -i http://localhost:3000/api/data --cookie "auth_token=your.jwt.token"
curl -i http://localhost:3001/api/data --cookie "auth_token=your.jwt.token"
```ebsocat`:

```bash
# Install websocat (https://github.com/vi/websocat)
# macOS
brew install websocat

# Or download from GitHub releases

# Connect to WebSocket with authentication cookie
websocatl wscat
npm install -g wscat

# Connect to WebSocket with authentication cookie
wscat -c ws://localhost:3000/websocket \
  -H "Cookie: auth_token=your.jwt.token"
```

WebSocket connections without a valid JWT token will be rejected with a 401 Unauthorized response.

## Error Handling

- **No token / Invalid token**: 
  - HTTP requests: Redirects to `REDIRECT_URL`
  - WebSocket connections: Rejected with 401 Unauthorized
- **Upstream connection fails**: Returns 502 Bad Gateway
- **Server errors**: Returns 500 Internal Server Error

## License

ISC
