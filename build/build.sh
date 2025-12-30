#!/bin/bash

# UTP-Core Build Script for Linux
# This script builds the UTP-Core binary for Linux platforms

set -e

# Configuration
BINARY_NAME="utp-core"
BUILD_DIR="build"
CMD_DIR="cmd/utp-core"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Functions
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed. Please install Go 1.21 or higher."
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
print_info "Go version: $GO_VERSION"

# Get version information
VERSION=${VERSION:-"dev"}
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

print_info "Building UTP-Core..."
print_info "Version: $VERSION"
print_info "Commit: $COMMIT"

# Create build directory
mkdir -p "$BUILD_DIR"

# Build flags
LDFLAGS="-s -w -checklinkname=0 -X main.version=$VERSION -X main.commit=$COMMIT"

# Build for Linux AMD64
print_info "Building for Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -v -ldflags "$LDFLAGS" -o "$BUILD_DIR/${BINARY_NAME}-linux-amd64" "./$CMD_DIR"

if [ $? -eq 0 ]; then
    print_info "Build successful!"
    print_info "Binary location: $BUILD_DIR/${BINARY_NAME}-linux-amd64"
    
    # Make binary executable
    chmod +x "$BUILD_DIR/${BINARY_NAME}-linux-amd64"
    
    # Show binary info
    ls -lh "$BUILD_DIR/${BINARY_NAME}-linux-amd64"
else
    print_error "Build failed!"
    exit 1
fi

print_info "Build complete!"
