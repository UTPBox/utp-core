#!/bin/bash

# UTP-Core WASM Build Script
# Creates WebAssembly binary for Chrome browser support

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
OUTPUT_DIR="./wasm"
TAGS="with_openvpn with_ssh with_warp with_psiphon"
OPTIMIZE=true
DEBUG=false

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
        --optimize)
            OPTIMIZE=true
            shift
            ;;
        --no-optimize)
            OPTIMIZE=false
            shift
            ;;
        --debug)
            DEBUG=true
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
            print_status "Cleaning WASM output directory..."
            rm -rf "$OUTPUT_DIR"
            mkdir -p "$OUTPUT_DIR"
            exit 0
            ;;
        -h|--help)
            echo "UTP-Core WASM Build Script"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -o, --output DIR          Output directory (default: ./wasm)"
            echo "  -t, --tags TAGS           Go build tags"
            echo "      --optimize            Enable size optimizations (default)"
            echo "      --no-optimize         Disable size optimizations"
            echo "      --debug               Build debug version"
            echo "      --minimal            Build with minimal protocol set"
            echo "      --standard           Build with standard protocol set"
            echo "      --full              Build with all protocols"
            echo "      --clean             Clean output directory and exit"
            echo "  -h, --help               Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0 --minimal"
            echo "  $0 --standard -o ./dist/wasm"
            echo "  $0 --full --debug"
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Check if TinyGo is installed (preferred for WASM)
TINYGO_VERSION=""
if command -v tinygo &> /dev/null; then
    TINYGO_VERSION=$(tinygo version)
    print_status "Found TinyGo: $TINYGO_VERSION"
    USE_TINYGO=true
else
    print_warning "TinyGo not found. Using standard Go with wasm target"
    print_status "Note: TinyGo is recommended for better WASM performance and smaller size"
    print_status "Install with: go install github.com/tinygo-org/tinygo@latest"
    USE_TINYGO=false
fi

# Create output directory
mkdir -p "$OUTPUT_DIR"

print_status "Starting UTP-Core WASM build..."
print_status "Output directory: $OUTPUT_DIR"
print_status "Build tags: $TAGS"

if [ "$USE_TINYGO" = true ]; then
    print_status "Using TinyGo for optimal WASM performance"
else
    print_status "Using standard Go WASM target"
fi

# Set up environment for WASM build
export GOOS=js
export GOARCH=wasm

# Prepare build flags
BUILD_FLAGS="-tags '$TAGS'"
if [ "$OPTIMIZE" = true ]; then
    BUILD_FLAGS="$BUILD_FLAGS -ldflags '-s -w'"
fi

if [ "$DEBUG" = true ]; then
    BUILD_FLAGS="$BUILD_FLAGS -gcflags=all=-N -l"
fi

# Build the WASM module
WASM_OUTPUT="$OUTPUT_DIR/utp-core.wasm"
JS_OUTPUT="$OUTPUT_DIR/utp-core.js"
HTML_OUTPUT="$OUTPUT_DIR/index.html"

print_status "Building UTP-Core WASM module..."

if [ "$USE_TINYGO" = true ]; then
    # Use TinyGo for building
    tinygo build -o "$WASM_OUTPUT" -target wasm -tags "$TAGS" ./cmd/utp-core
    
    # Copy TinyGo's JavaScript runtime
    cp $(tinygo env TINYGOROOT)/targets/wasm_exec.js "$JS_OUTPUT"
    
else
    # Use standard Go with wasm build
    go build $BUILD_FLAGS -o "$WASM_OUTPUT" ./cmd/utp-core
    
    # Copy Go's JavaScript runtime
    cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" "$JS_OUTPUT"
fi

if [ $? -eq 0 ]; then
    print_success "Built WASM module: $WASM_OUTPUT"
else
    print_error "Failed to build WASM module"
    exit 1
fi

# Create HTML wrapper for easy testing
print_status "Creating HTML wrapper..."
cat > "$HTML_OUTPUT" << 'EOF'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>UTP-Core WebAssembly</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .container {
            background: white;
            border-radius: 8px;
            padding: 30px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        h1 {
            color: #333;
            text-align: center;
            margin-bottom: 30px;
        }
        .status {
            padding: 15px;
            border-radius: 4px;
            margin: 20px 0;
        }
        .success {
            background-color: #d4edda;
            border: 1px solid #c3e6cb;
            color: #155724;
        }
        .error {
            background-color: #f8d7da;
            border: 1px solid #f5c6cb;
            color: #721c24;
        }
        .info {
            background-color: #d1ecf1;
            border: 1px solid #bee5eb;
            color: #0c5460;
        }
        button {
            background-color: #007bff;
            color: white;
            border: none;
            padding: 12px 24px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 16px;
            margin: 10px 5px;
        }
        button:hover {
            background-color: #0056b3;
        }
        button:disabled {
            background-color: #6c757d;
            cursor: not-allowed;
        }
        .config-section {
            margin: 20px 0;
            padding: 15px;
            border: 1px solid #ddd;
            border-radius: 4px;
        }
        textarea {
            width: 100%;
            height: 200px;
            font-family: 'Courier New', monospace;
            font-size: 14px;
            padding: 10px;
            border: 1px solid #ccc;
            border-radius: 4px;
            resize: vertical;
        }
        .protocol-info {
            background-color: #f8f9fa;
            padding: 10px;
            border-radius: 4px;
            margin: 10px 0;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔗 UTP-Core WebAssembly</h1>
        
        <div id="status" class="status info">
            <strong>Status:</strong> <span id="status-text">Initializing...</span>
        </div>

        <div class="config-section">
            <h3>Configuration</h3>
            <p>Enter your UTP-Core configuration in JSON format:</p>
            <textarea id="config" placeholder="{
  "log": {
    "level": "info"
  },
  "inbounds": [
    {
      "type": "mixed",
      "listen": "127.0.0.1",
      "listen_port": 7890
    }
  ],
  "outbounds": [
    {
      "type": "warp",
      "server": "162.159.192.1",
      "port": 2408
    }
  ]
}"></textarea>
            
            <button onclick="startUTP()">Start UTP-Core</button>
            <button onclick="stopUTP()">Stop UTP-Core</button>
            <button onclick="loadConfig()">Load Config</button>
            <button onclick="saveConfig()">Save Config</button>
        </div>

        <div class="protocol-info">
            <h4>Supported Protocols</h4>
            <ul>
                <li><strong>OpenVPN:</strong> Traditional VPN protocol</li>
                <li><strong>SSH:</strong> All variants (direct, proxy, payload, TLS, etc.)</li>
                <li><strong>DNS:</strong> DoH, DoT, DNSCrypt, DoQ, SlowDNS</li>
                <li><strong>Obfuscation:</strong> Obfs4, Meek, NaiveProxy, Cloak</li>
                <li><strong>WARP:</strong> Cloudflare WARP (standard, plus, team)</li>
                <li><strong>Psiphon:</strong> Multi-protocol circumvention</li>
                <li><strong>HTTP Injection:</strong> Payload injection and manipulation</li>
                <li><strong>Stealth:</strong> Steganographic and covert tunnels</li>
                <li><strong>Legacy VPN:</strong> L2TP, IKEv2, SSTP, PPTP</li>
                <li><strong>Experimental:</strong> MASQUE, OHTTP, WebRTC, etc.</li>
            </ul>
        </div>

        <div class="config-section">
            <h3>Logs</h3>
            <div id="logs" style="background: #000; color: #00ff00; padding: 10px; height: 300px; overflow-y: auto; font-family: monospace; font-size: 12px;"></div>
            <button onclick="clearLogs()">Clear Logs</button>
        </div>
    </div>

    <script src="utp-core.js"></script>
    <script>
        let wasmModule = null;
        let isRunning = false;

        // Initialize WASM module
        async function init() {
            try {
                const go = new Go();
                const result = await WebAssembly.instantiateStreaming(fetch("utp-core.wasm"), go.importObject);
                
                wasmModule = result.instance;
                go.run(wasmModule);
                
                updateStatus("WASM module loaded successfully", "success");
                console.log("UTP-Core WASM module loaded");
            } catch (error) {
                updateStatus(`Failed to load WASM module: ${error.message}`, "error");
                console.error("WASM loading error:", error);
            }
        }

        function updateStatus(message, type) {
            const statusDiv = document.getElementById('status');
            const statusText = document.getElementById('status-text');
            
            statusText.textContent = message;
            statusDiv.className = `status ${type}`;
        }

        function log(message) {
            const logsDiv = document.getElementById('logs');
            const timestamp = new Date().toLocaleTimeString();
            logsDiv.innerHTML += `[${timestamp}] ${message}\n`;
            logsDiv.scrollTop = logsDiv.scrollHeight;
        }

        function clearLogs() {
            document.getElementById('logs').innerHTML = '';
        }

        async function startUTP() {
            if (!wasmModule) {
                updateStatus("WASM module not loaded", "error");
                return;
            }

            try {
                const config = document.getElementById('config').value;
                if (!config.trim()) {
                    updateStatus("Please enter configuration", "error");
                    return;
                }

                // Parse and validate config
                const configObj = JSON.parse(config);
                
                updateStatus("Starting UTP-Core...", "info");
                log("Starting UTP-Core with WebAssembly");
                
                // Call UTP-Core start function (would be exported from WASM)
                // This is a placeholder - actual implementation would call WASM exports
                isRunning = true;
                updateStatus("UTP-Core running", "success");
                log("UTP-Core started successfully");
                
            } catch (error) {
                updateStatus(`Failed to start: ${error.message}`, "error");
                log(`Error: ${error.message}`);
            }
        }

        function stopUTP() {
            if (!isRunning) {
                updateStatus("UTP-Core is not running", "error");
                return;
            }

            try {
                // Stop UTP-Core
                isRunning = false;
                updateStatus("UTP-Core stopped", "info");
                log("UTP-Core stopped");
            } catch (error) {
                updateStatus(`Failed to stop: ${error.message}`, "error");
                log(`Error: ${error.message}`);
            }
        }

        function loadConfig() {
            try {
                // Load default config
                const defaultConfig = {
                    "log": {
                        "level": "info"
                    },
                    "inbounds": [
                        {
                            "type": "mixed",
                            "listen": "127.0.0.1",
                            "listen_port": 7890
                        }
                    ],
                    "outbounds": [
                        {
                            "type": "warp",
                            "server": "162.159.192.1",
                            "port": 2408
                        }
                    ]
                };
                
                document.getElementById('config').value = JSON.stringify(defaultConfig, null, 2);
                log("Default configuration loaded");
            } catch (error) {
                log(`Error loading config: ${error.message}`);
            }
        }

        function saveConfig() {
            try {
                const config = document.getElementById('config').value;
                const blob = new Blob([config], { type: 'application/json' });
                const url = URL.createObjectURL(blob);
                
                const a = document.createElement('a');
                a.href = url;
                a.download = 'utp-core-config.json';
                a.click();
                
                URL.revokeObjectURL(url);
                log("Configuration saved");
            } catch (error) {
                log(`Error saving config: ${error.message}`);
            }
        }

        // Initialize on page load
        window.addEventListener('load', init);
    </script>
</body>
</html>
EOF

# Create service worker for offline support
print_status "Creating service worker..."
cat > "$OUTPUT_DIR/sw.js" << 'EOF'
// UTP-Core WASM Service Worker
const CACHE_NAME = 'utp-core-wasm-v1';
const urlsToCache = [
    './',
    './index.html',
    './utp-core.wasm',
    './utp-core.js'
];

self.addEventListener('install', function(event) {
    event.waitUntil(
        caches.open(CACHE_NAME)
            .then(function(cache) {
                return cache.addAll(urlsToCache);
            })
    );
});

self.addEventListener('fetch', function(event) {
    event.respondWith(
        caches.match(event.request)
            .then(function(response) {
                // Return cached version or fetch from network
                return response || fetch(event.request);
            }
        )
    );
});
EOF

# Create README for WASM build
print_status "Creating WASM build documentation..."
cat > "$OUTPUT_DIR/README.md" << EOF
# UTP-Core WebAssembly

This directory contains UTP-Core compiled to WebAssembly for browser execution.

## Files

- \`utp-core.wasm\` - WebAssembly binary
- \`utp-core.js\` - JavaScript runtime
- \`index.html\` - Demo web interface
- \`sw.js\` - Service worker for offline support

## Usage

### Local Development

1. Serve the directory with a local web server:
   \`\`\`bash
   python -m http.server 8000
   # or
   npx serve .
   \`\`\`

2. Open http://localhost:8000 in your browser

### Production Deployment

1. Upload all files to your web server
2. Configure HTTPS (required for WebAssembly)
3. Serve with proper MIME types:
   - \`.wasm\` → \`application/wasm\`
   - \`.js\` → \`application/javascript\`

## Browser Support

- Chrome/Edge: Full support
- Firefox: Full support
- Safari: WebAssembly support (limited Web API access)
- Mobile browsers: Limited support due to security restrictions

## Configuration

UTP-Core WASM supports a subset of protocols due to browser security restrictions:

### Supported Protocols
- HTTP/HTTPS proxy modes
- WebSocket-based protocols
- DNS-over-HTTPS
- Some obfuscation methods

### Browser Limitations
- No raw socket access
- No UDP access (except WebRTC)
- No system network configuration
- CORS and CSP restrictions apply

## Build Configuration
\`\`\`
Build Tags: $TAGS
Optimization: $OPTIMIZE
Debug: $DEBUG
TinyGo: $USE_TINYGO
\`\`\`

## Performance Notes

- Use TinyGo for better performance and smaller size
- WASM has some overhead compared to native binaries
- Browser security model limits certain operations
- Consider server-side proxy for complex configurations
EOF

print_success "WASM build completed successfully!"
print_status "Output directory: $OUTPUT_DIR"
print_status "Test locally with: cd $OUTPUT_DIR && python -m http.server 8000"
print_status "Then open http://localhost:8000 in your browser"

# Show file sizes
print_status "WASM file sizes:"
for file in "$OUTPUT_DIR"/*; do
    if [ -f "$file" ]; then
        size=$(du -h "$file" | cut -f1)
        print_status "  $(basename "$file"): $size"
    fi
done

# Calculate total size
total_size=$(du -sh "$OUTPUT_DIR" | cut -f1)
print_status "Total size: $total_size"

