# Reverse Auth Proxy

A simple Express-based reverse proxy with JWT authentication. This proxy verifies JWT tokens from cookies before forwarding requests to an upstream server. If authentication fails, users are redirected to a configurable login URL.

## Features

- 🔒 JWT token verification via cookies
- 🔄 Reverse proxy to a single upstream HTTP server
- ↪️ Configurable redirect URL for authentication failures
- 🚀 Simple configuration via environment variables
- 🔓 Support for public paths (bypass authentication)
- 📝 Automatic user info forwarding to upstream via headers

## Installation

```bash
npm install
```

## Configuration

1. Copy the example environment file:
```bash
cp .env.example .env
```

2. Edit `.env` with your settings:

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
| `PORT` | No | 3000 | Port the proxy server listens on |
| `UPSTREAM_URL` | No | http://localhost:8080 | Target server to proxy requests to |
| `REDIRECT_URL` | No | http://localhost:3001/login | URL to redirect to when auth fails |
| `JWT_KEY_PATH` | **Yes** | - | Path to .pem file containing JWT public key |
| `JWT_COOKIE_NAME` | No | auth_token | Name of the cookie containing the JWT |
| `PUBLIC_PATHS` | No | [] | Comma-separated list of paths that bypass auth |

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

## Usage

### Start the server

```bash
npm start
```

### Development mode (with auto-reload)

```bash
npm run dev
```

## How It Works

1. **Request arrives** at the proxy server
2. **Cookie check**: The proxy looks for a JWT token in the specified cookie
3. **Verification**:
   - If the token is valid, the request is forwarded to the upstream server
   - If the token is missing or invalid, the user is redirected to the login URL
4. **Headers**: Valid JWT claims (userId, email) are forwarded to upstream as headers:
   - `X-User-Id`: User ID from JWT
   - `X-User-Email`: Email from JWT

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
# Without token (should redirect)
curl -i http://localhost:3000/api/test

# With token
curl -i http://localhost:3000/api/test \
  --cookie "auth_token=your.jwt.token"
```

## Error Handling

- **No token / Invalid token**: Redirects to `REDIRECT_URL`
- **Upstream connection fails**: Returns 502 Bad Gateway
- **Server errors**: Returns 500 Internal Server Error

## License

ISC
