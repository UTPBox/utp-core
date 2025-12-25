#!/bin/bash

# UTP-Core Mobile Build Script
# Creates Android AAR and iOS Framework for Flutter VPN apps

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
OUTPUT_DIR="./mobile"
TAGS="with_openvpn with_ssh with_warp with_psiphon with_dns"
PLATFORMS=("android" "ios")
ANDROID_ABI=("arm64-v8a" "armeabi-v7a" "x86_64")
IOS_ARCHS=("arm64" "x86_64")

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
        --android)
            PLATFORMS=("android")
            shift
            ;;
        --ios)
            PLATFORMS=("ios")
            shift
            ;;
        --android-abi)
            ANDROID_ABI=($2)
            shift 2
            ;;
        --ios-arch)
            IOS_ARCHS=($2)
            shift 2
            ;;
        --minimal)
            TAGS="with_openvpn with_ssh with_warp with_psiphon"
            shift
            ;;
        --standard)
            TAGS="with_openvpn with_ssh with_warp with_psiphon with_dns"
            shift
            ;;
        --full)
            TAGS="with_openvpn with_ssh with_dns with_obfs with_warp with_psiphon with_httpinject with_stealth with_legacyvpn with_experimental"
            shift
            ;;
        --clean)
            print_status "Cleaning mobile output directory..."
            rm -rf "$OUTPUT_DIR"
            mkdir -p "$OUTPUT_DIR"
            exit 0
            ;;
        -h|--help)
            echo "UTP-Core Mobile Build Script"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -o, --output DIR          Output directory (default: ./mobile)"
            echo "  -t, --tags TAGS           Go build tags"
            echo "      --android             Build only Android AAR"
            echo "      --ios                 Build only iOS Framework"
            echo "      --android-abi LIST    Comma-separated list of Android ABIs (default: arm64-v8a,armeabi-v7a,x86_64)"
            echo "      --ios-arch LIST       Comma-separated list of iOS architectures (default: arm64,x86_64)"
            echo "      --minimal            Build with minimal protocol set"
            echo "      --standard           Build with standard protocol set"
            echo "      --full              Build with all protocols"
            echo "      --clean             Clean output directory and exit"
            echo "  -h, --help               Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0 --minimal --android"
            echo "  $0 --standard --ios"
            echo "  $0 --full -o ./dist/mobile"
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Check if gomobile is installed
if ! command -v gomobile &> /dev/null; then
    print_error "gomobile not found. Installing gomobile..."
    go install golang.org/x/mobile/cmd/gomobile@latest
    gomobile init
fi

# Create output directory
mkdir -p "$OUTPUT_DIR"

print_status "Starting UTP-Core mobile build..."
print_status "Output directory: $OUTPUT_DIR"
print_status "Build tags: $TAGS"

# Build for each platform
for platform in "${PLATFORMS[@]}"; do
    if [ "$platform" = "android" ]; then
        print_status "Building Android AAR..."
        
        # Build Android AAR
        OUTPUT_AAR="$OUTPUT_DIR/utp-core-android.aar"
        
        # Set environment for Android build
        export CGO_ENABLED=1
        export GOOS=android
        export GOARCH=arm64
        
        # Build AAR
        gomobile bind -target=android -tags "$TAGS" -o "$OUTPUT_AAR" ./cmd/utp-core
        
        if [ $? -eq 0 ]; then
            print_success "Built Android AAR: $OUTPUT_AAR"
        else
            print_error "Failed to build Android AAR"
            exit 1
        fi
        
        # Build for multiple Android ABIs
        for abi in "${ANDROID_ABI[@]}"; do
            print_status "Building Android for ABI: $abi"
            
            OUTPUT_SO="$OUTPUT_DIR/utp-core-android-$abi.aar"
            
            case $abi in
                "arm64-v8a")
                    export GOARCH=arm64
                    ;;
                "armeabi-v7a")
                    export GOARCH=arm
                    ;;
                "x86_64")
                    export GOARCH=amd64
                    ;;
            esac
            
            gomobile bind -target=android -tags "$TAGS" -o "$OUTPUT_SO" ./cmd/utp-core
            
            if [ $? -eq 0 ]; then
                print_success "Built Android AAR for $abi: $OUTPUT_SO"
            else
                print_warning "Failed to build Android AAR for $abi"
            fi
        done
        
    elif [ "$platform" = "ios" ]; then
        print_status "Building iOS Framework..."
        
        # Build iOS Framework
        OUTPUT_FRAMEWORK="$OUTPUT_DIR/UTPFramework.framework"
        
        # Set environment for iOS build
        export CGO_ENABLED=1
        export GOOS=darwin
        export GOARCH=arm64
        
        # Build Framework
        gomobile bind -target=ios -tags "$TAGS" -o "$OUTPUT_FRAMEWORK" ./cmd/utp-core
        
        if [ $? -eq 0 ]; then
            print_success "Built iOS Framework: $OUTPUT_FRAMEWORK"
        else
            print_error "Failed to build iOS Framework"
            exit 1
        fi
        
        # Build for multiple iOS architectures
        for arch in "${IOS_ARCHS[@]}"; do
            print_status "Building iOS for architecture: $arch"
            
            OUTPUT_ARCH_FRAMEWORK="$OUTPUT_DIR/UTPFramework-$arch.framework"
            
            case $arch in
                "arm64")
                    export GOARCH=arm64
                    ;;
                "x86_64")
                    export GOARCH=amd64
                    ;;
            esac
            
            gomobile bind -target=ios -tags "$TAGS" -o "$OUTPUT_ARCH_FRAMEWORK" ./cmd/utp-core
            
            if [ $? -eq 0 ]; then
                print_success "Built iOS Framework for $arch: $OUTPUT_ARCH_FRAMEWORK"
            else
                print_warning "Failed to build iOS Framework for $arch"
            fi
        done
    fi
done

# Create universal binaries for iOS (combining arm64 and x86_64)
if [[ " ${PLATFORMS[@]} " =~ " ios " ]]; then
    print_status "Creating universal iOS Framework..."
    
    if [ -d "$OUTPUT_DIR/UTPFramework-arm64.framework" ] && [ -d "$OUTPUT_DIR/UTPFramework-x86_64.framework" ]; then
        # Create universal framework by combining architectures
        mkdir -p "$OUTPUT_DIR/UTPFramework-universal.framework"
        
        # Copy structure from arm64 framework
        cp -r "$OUTPUT_DIR/UTPFramework-arm64.framework/Headers" "$OUTPUT_DIR/UTPFramework-universal.framework/"
        cp -r "$OUTPUT_DIR/UTPFramework-arm64.framework/Modules" "$OUTPUT_DIR/UTPFramework-universal.framework/"
        
        # Copy Info.plist
        cp "$OUTPUT_DIR/UTPFramework-arm64.framework/Info.plist" "$OUTPUT_DIR/UTPFramework-universal.framework/"
        
        # Use lipo to create universal binary
        lipo -create \
            "$OUTPUT_DIR/UTPFramework-arm64.framework/UTPFramework" \
            "$OUTPUT_DIR/UTPFramework-x86_64.framework/UTPFramework" \
            -output "$OUTPUT_DIR/UTPFramework-universal.framework/UTPFramework"
        
        print_success "Created universal iOS Framework: $OUTPUT_DIR/UTPFramework-universal.framework"
    else
        print_warning "Cannot create universal framework - missing individual architectures"
    fi
fi

# Create mobile build summary
print_status "Creating mobile build summary..."
cat > "$OUTPUT_DIR/README.md" << EOF
# UTP-Core Mobile Libraries

This directory contains UTP-Core mobile libraries built with the following configuration:

## Build Tags
\`\`\`
$TAGS
\`\`\`

## Android

### Universal AAR
- \`utp-core-android.aar\` - Contains all supported ABIs

### Per-ABI AARs
EOF

for abi in "${ANDROID_ABI[@]}"; do
    if [ -f "$OUTPUT_DIR/utp-core-android-$abi.aar" ]; then
        echo "- \`utp-core-android-$abi.aar\` - For $abi" >> "$OUTPUT_DIR/README.md"
    fi
done

cat >> "$OUTPUT_DIR/README.md" << EOF

## iOS

### Frameworks
EOF

if [ -d "$OUTPUT_DIR/UTPFramework-universal.framework" ]; then
    echo "- \`UTPFramework-universal.framework\` - Universal binary (recommended)" >> "$OUTPUT_DIR/README.md"
fi

for arch in "${IOS_ARCHS[@]}"; do
    if [ -d "$OUTPUT_DIR/UTPFramework-$arch.framework" ]; then
        echo "- \`UTPFramework-$arch.framework\` - For $arch" >> "$OUTPUT_DIR/README.md"
    fi
done

cat >> "$OUTPUT_DIR/README.md" << EOF

## Usage

### Android
Add the AAR file to your Flutter project:

\`\`\`dart
// pubspec.yaml
dependencies:
  flutter:
    sdk: flutter
  utp_core_android:
    path: ./mobile/utp-core-android.aar
\`\`\`

### iOS
Add the framework to your Xcode project:

1. Add \`UTPFramework-universal.framework\` to your project
2. Link the framework in Build Phases
3. Import in Swift/Objective-C code:

\`\`\`swift
import UTPFramework

let utpCore = UTPFramework()
\`\`\`

## Supported Protocols
- OpenVPN
- SSH (all variants)
- DNS (DoH, DoT, DNSCrypt, DoQ, SlowDNS)
- Obfuscation (Obfs4, Meek, NaiveProxy, Cloak, etc.)
- WARP (standard, plus, team)
- Psiphon
- HTTP Injection
- Steganography
- Legacy VPN (L2TP, IKEv2, SSTP, PPTP, etc.)
- Experimental (MASQUE, OHTTP, WebRTC, etc.)
EOF

print_success "Mobile build completed successfully!"
print_status "Output directory: $OUTPUT_DIR"
print_status "See $OUTPUT_DIR/README.md for usage instructions"

# Show file sizes
print_status "Mobile library sizes:"
for file in "$OUTPUT_DIR"/*.aar "$OUTPUT_DIR"/*.framework; do
    if [ -f "$file" ] || [ -d "$file" ]; then
        if [ -d "$file" ]; then
            size=$(du -sh "$file" | cut -f1)
        else
            size=$(du -h "$file" | cut -f1)
        fi
        print_status "  $file: $size"
    fi
done

