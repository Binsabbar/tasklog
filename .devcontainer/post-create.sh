#!/bin/bash

set -e

echo "🚀 Setting up Tasklog development environment..."

# Install Go tools
echo "📦 Installing Go development tools..."
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.2
go install golang.org/x/vuln/cmd/govulncheck@v1.0.4
go install github.com/goreleaser/goreleaser/v2@v2.0.1

# Download Go dependencies
echo "📥 Downloading Go dependencies..."
go mod download

# Build the project to verify everything works
echo "🔨 Building project..."
make go-build

# Run tests to ensure environment is ready
echo "🧪 Running tests..."
if ! make go-test; then
  echo "⚠️ Tests failed, but continuing environment setup. Please run 'make go-test' manually after fixing issues."
fi

echo "✅ Development environment is ready!"
echo ""
echo "Available commands:"
echo "  make help              - Show all available commands"
echo "  make go-build          - Build the binary"
echo "  make go-test           - Run tests"
echo "  make go-lint           - Run linter"
echo "  make go-fmt            - Format code"
echo ""
echo "Happy coding! 🎉"
