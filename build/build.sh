#!/bin/bash

# UTP-Core Build Script
# Supports building with different protocol sets using Go build tags

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Default values
OUTPUT_DIR="./bin"
TAGS=""
PROTOCOLS=("all")
PLATFORMS=("linux" "windows" "darwin")
ARCHS=("amd64" "arm64")

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -o|--output)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        -t|--tags)
            TAGS="$2"
            shift 2
            ;;
        -p|--protocols)
            PROTOCOLS=($2)
            shift 2
            ;;
        --platform)
            PLATFORMS=($2)
            shift 2
            ;;
        --arch)
            ARCHS=($2)
            shift 2
            ;;
        --protocol)
            TAGS="$TAGS with_$2"
            shift
            ;;
        --all-protocols)
            TAGS="with_openvpn with_ssh with_dns with_obfs with_warp with_psiphon with_httpinject with_stealth with_legacyvpn with_experimental"
            shift
            ;;
        --minimal)
            TAGS="with_openvpn with_ssh with_warp with_psiphon"
            shift
            ;;
        --standard)
            TAGS="with_openvpn with_ssh with_dns with_obfs with_warp with_psiphon"
            shift
            ;;
        --full)
            TAGS="with_openvpn with_ssh with_dns with_obfs with_warp with_psiphon with_httpinject with_stealth with_legacyvpn with_experimental"
            shift
            ;;
        --clean)
            print_status "Cleaning output directory..."
            rm -rf "$OUTPUT_DIR"
            mkdir -p "$OUTPUT_DIR"
            exit 0
            ;;
        -h|--help)
            echo "UTP-Core Build Script"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -o, --output DIR          Output directory (default: ./bin)"
            echo "  -t, --tags TAGS           Go build tags"
            echo "  -p, --protocols LIST      Comma-separated list of protocols to build"
            echo "      --platform LIST       Comma-separated list of platforms (default: linux,windows,darwin)"
            echo "      --arch LIST          Comma-separated list of architectures (default: amd64,arm64)"
            echo "      --protocol NAME      Add protocol by name (openvpn, ssh, dns, obfs, warp, psiphon, httpinject, stealth, legacyvpn, experimental)"
            echo "      --all-protocols      Build with all protocols"
            echo "      --minimal           Build with minimal protocol set (openvpn, ssh, warp, psiphon)"
            echo "      --standard          Build with standard protocol set (openvpn, ssh, dns, obfs, warp, psiphon)"
            echo "      --full              Build with all protocols"
            echo "      --clean             Clean output directory and exit"
            echo "  -h, --help               Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0 --minimal"
            echo "  $0 --standard --platform linux,darwin --arch amd64"
            echo "  $0 --all-protocols -o ./dist"
            echo "  $0 --protocol openvpn --protocol ssh"
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Create output directory
mkdir -p "$OUTPUT_DIR"

print_status "Starting UTP-Core build..."
print_status "Output directory: $OUTPUT_DIR"
if [ -n "$TAGS" ]; then
    print_status "Build tags: $TAGS"
fi

# Build for each platform and architecture combination
for platform in "${PLATFORMS[@]}"; do
    for arch in "${ARCHS[@]}"; do
        # Set GOOS and GOARCH
        GOOS="$platform"
        GOARCH="$arch"
        
        # Determine binary name and extension
        if [ "$platform" = "windows" ]; then
            BINARY_NAME="utp-core.exe"
        else
            BINARY_NAME="utp-core"
        fi
        
        OUTPUT_FILE="$OUTPUT_DIR/${platform}-${arch}/${BINARY_NAME}"
        
        print_status "Building for $GOOS/$GOARCH..."
        
        # Build the binary
        if [ -n "$TAGS" ]; then
            CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "-s -w" -tags "$TAGS" -o "$OUTPUT_FILE" ./cmd/utp-core
        else
            CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "-s -w" -o "$OUTPUT_FILE" ./cmd/utp-core
        fi
        
        if [ $? -eq 0 ]; then
            print_success "Built $OUTPUT_FILE"
        else
            print_error "Failed to build for $GOOS/$GOARCH"
            exit 1
        fi
    done
done

# Create checksums
print_status "Creating checksums..."
cd "$OUTPUT_DIR"
find . -name "utp-core*" -type f -exec sha256sum {} \; > checksums.txt
cd - > /dev/null

print_success "Build completed successfully!"
print_status "Output directory: $OUTPUT_DIR"
print_status "To install: sudo cp $OUTPUT_DIR/linux-amd64/utp-core /usr/local/bin/"

# Show binary sizes
print_status "Binary sizes:"
for binary in "$OUTPUT_DIR"/*/utp-core*; do
    if [ -f "$binary" ]; then
        size=$(du -h "$binary" | cut -f1)
        print_status "  $binary: $size"
    fi
done

