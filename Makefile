.PHONY: build install clean test lint proto-gen

# Build the clawchaind binary
build:
	@echo "Building clawchaind..."
	@go build -o build/clawchaind ./cmd/clawchaind
	@echo "Built: build/clawchaind"

# Install the binary to $GOPATH/bin
install:
	@echo "Installing clawchaind..."
	@go install ./cmd/clawchaind

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf build/

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run

# Tidy dependencies
tidy:
	@echo "Tidying go.mod..."
	@go mod tidy

# Generate protobuf (when implemented)
proto-gen:
	@echo "Protobuf generation not yet implemented"
	@echo "Add buf or protoc commands here in Phase 1"

# Show version
version: build
	@./build/clawchaind version

# Show help
help:
	@echo "ClawChain Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  build      - Build the clawchaind binary"
	@echo "  install    - Install binary to GOPATH/bin"
	@echo "  clean      - Remove build artifacts"
	@echo "  test       - Run all tests"
	@echo "  lint       - Run linter"
	@echo "  tidy       - Tidy go.mod dependencies"
	@echo "  proto-gen  - Generate protobuf code"
	@echo "  version    - Build and show version"
	@echo "  help       - Show this help message"
