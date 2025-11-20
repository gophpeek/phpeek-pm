.PHONY: build clean test install

# Build variables
BINARY_NAME=phpeek-pm
VERSION=1.0.0
BUILD_DIR=build
CMD_DIR=cmd/phpeek-pm

# Go build flags for static binary
LDFLAGS=-ldflags "-w -s -X main.version=$(VERSION)"
STATIC_FLAGS=CGO_ENABLED=0

# Build the binary
build:
	@echo "🔨 Building PHPeek Process Manager..."
	@mkdir -p $(BUILD_DIR)
	$(STATIC_FLAGS) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "✅ Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for multiple platforms
build-all:
	@echo "🔨 Building for all platforms..."
	@mkdir -p $(BUILD_DIR)

	@echo "Building for Linux AMD64..."
	GOOS=linux GOARCH=amd64 $(STATIC_FLAGS) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./$(CMD_DIR)

	@echo "Building for Linux ARM64..."
	GOOS=linux GOARCH=arm64 $(STATIC_FLAGS) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./$(CMD_DIR)

	@echo "Building for macOS AMD64..."
	GOOS=darwin GOARCH=amd64 $(STATIC_FLAGS) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./$(CMD_DIR)

	@echo "Building for macOS ARM64..."
	GOOS=darwin GOARCH=arm64 $(STATIC_FLAGS) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./$(CMD_DIR)

	@echo "✅ All builds complete"
	@ls -lh $(BUILD_DIR)

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@echo "✅ Clean complete"

# Run tests
test:
	@echo "🧪 Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "✅ Tests complete"

# Install dependencies
deps:
	@echo "📦 Installing dependencies..."
	go mod download
	go mod tidy
	@echo "✅ Dependencies installed"

# Install binary to system
install: build
	@echo "📥 Installing $(BINARY_NAME)..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "✅ Installed to /usr/local/bin/$(BINARY_NAME)"

# Development: build and run
dev: build
	@echo "🚀 Running $(BINARY_NAME)..."
	@$(BUILD_DIR)/$(BINARY_NAME)

# Show help
help:
	@echo "PHPeek Process Manager - Make targets:"
	@echo "  build      - Build binary for current platform"
	@echo "  build-all  - Build for all platforms (Linux, macOS, AMD64, ARM64)"
	@echo "  clean      - Remove build artifacts"
	@echo "  test       - Run tests"
	@echo "  deps       - Install/update dependencies"
	@echo "  install    - Install binary to /usr/local/bin"
	@echo "  dev        - Build and run for development"
	@echo "  help       - Show this help message"
