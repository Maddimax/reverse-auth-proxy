.PHONY: build run clean test docker-build docker-push

# Variables
BINARY_NAME=reverse-auth-proxy
DOCKER_IMAGE=maddimax/reverse-auth-proxy:go
GO111MODULE=on

# Build the Go binary
build:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) .
	@echo "Build complete!"

# Build for linux/amd64 (useful for Docker builds on Mac)
build-linux:
	@echo "Building $(BINARY_NAME) for linux/amd64..."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -installsuffix cgo -o $(BINARY_NAME) .
	@echo "Linux build complete!"

# Run the application
run:
	@echo "Running $(BINARY_NAME)..."
	go run .

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	go clean
	@echo "Clean complete!"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .

# Build Docker image for linux/amd64
docker-buildx:
	@echo "Building Docker image for linux/amd64..."
	docker buildx build --platform linux/amd64 -t $(DOCKER_IMAGE) --push .

# Push Docker image
docker-push:
	@echo "Pushing Docker image..."
	docker push $(DOCKER_IMAGE)

# Build and push Docker image in one command
docker-release: docker-buildx
	@echo "Docker release complete!"

# Format Go code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint Go code (requires golangci-lint)
lint:
	@echo "Linting code..."
	golangci-lint run

# Display help
help:
	@echo "Available targets:"
	@echo "  build         - Build the Go binary"
	@echo "  build-linux   - Build the Go binary for linux/amd64"
	@echo "  run           - Run the application"
	@echo "  clean         - Remove build artifacts"
	@echo "  test          - Run tests"
	@echo "  deps          - Download and tidy dependencies"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-buildx - Build and push Docker image for linux/amd64"
	@echo "  docker-push   - Push Docker image"
	@echo "  docker-release- Build and push Docker image"
	@echo "  fmt           - Format Go code"
	@echo "  lint          - Lint Go code"
	@echo "  help          - Display this help message"
