#!/bin/bash

set -e

echo "🚀 Setting up Tasklog development environment..."

# Install Go tools
echo "📦 Installing Go development tools..."
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
go install github.com/goreleaser/goreleaser/v2@latest

# Download Go dependencies
echo "📥 Downloading Go dependencies..."
go mod download

# Build the project to verify everything works
echo "🔨 Building project..."
make go-build

# Run tests to ensure environment is ready
echo "🧪 Running tests..."
make go-test

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
