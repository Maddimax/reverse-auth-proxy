#!/bin/bash
set -e

# Build script for reverse-auth-proxy

echo "Building Go application for linux/amd64..."

# Build and push Docker image
docker buildx build --platform linux/amd64 -t maddimax/reverse-auth-proxy:go --push .

echo "Build complete!"
echo "Image pushed to: maddimax/reverse-auth-proxy:go"

# Display image info
docker buildx imagetools inspect maddimax/reverse-auth-proxy:go
