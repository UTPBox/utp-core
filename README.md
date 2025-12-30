# UTP-Core

Universal Tunnel Protocol Core - A high-performance proxy core based on Sing-box.

## Overview

UTP-Core is a cross-platform proxy solution built on top of the powerful Sing-box core. It provides a unified interface for managing advanced networking capabilities including proxying, routing, and traffic management.

## Features

- 🚀 **High Performance**: Built on Sing-box for optimal speed and efficiency
- 🔧 **Flexible Configuration**: JSON-based configuration system
- 🌐 **Cross-Platform**: Supports Linux and Windows
- 📦 **Easy Deployment**: Single binary with no external dependencies
- 🔒 **Secure**: Advanced routing and DNS capabilities

## Installation

### Prerequisites

- Go 1.21 or higher (for building from source)

### Building from Source

#### Linux

```bash
# Clone the repository
git clone https://github.com/UTPBox/utp-core.git
cd utp-core

# Download dependencies
make deps

# Build
make build-linux

# Or use the build script
chmod +x build/build.sh
./build/build.sh
```

#### Windows

```bash
# Clone the repository
git clone https://github.com/UTPBox/utp-core.git
cd utp-core

# Download dependencies
go mod download

# Build
make build-windows

# Or use the build script
build\build.bat
```

### Build All Platforms

```bash
make build-all
```

## Usage

### Running UTP-Core

```bash
# Run with default config.json
./build/utp-core run -c config.json

# On Windows
.\build\utp-core-windows-amd64.exe run -c config.json
```

### Command Line Options

```bash
# Show help
./build/utp-core --help

# Show version
./build/utp-core version

# Run with custom config
./build/utp-core run -c /path/to/custom-config.json
```

## Configuration

UTP-Core uses JSON configuration files compatible with Sing-box. Here's a basic example:

```json
{
  "log": {
    "level": "info",
    "timestamp": true
  },
  "inbounds": [
    {
      "type": "mixed",
      "tag": "mixed-in",
      "listen": "127.0.0.1",
      "listen_port": 2080,
      "sniff": true
    }
  ],
  "outbounds": [
    {
      "type": "direct",
      "tag": "direct"
    }
  ]
}
```

### Configuration Options

- **log**: Logging configuration
  - `level`: Log level (debug, info, warn, error)
  - `timestamp`: Include timestamps in logs

- **inbounds**: Incoming connection handlers
  - `type`: Protocol type (mixed, socks, http, etc.)
  - `listen`: Listen address
  - `listen_port`: Listen port

- **outbounds**: Outgoing connection handlers
  - `type`: Protocol type (direct, block, socks, etc.)
  - `tag`: Identifier for routing rules

- **dns**: DNS configuration
- **route**: Routing rules

For complete configuration documentation, see the [Sing-box documentation](https://sing-box.sagernet.org/).

## Project Structure

```
utp-core/
├── cmd/
│   └── utp-core/          # CLI entrypoint
│       └── main.go
├── internal/              # Internal packages
│   └── config/
│       └── loader.go      # Configuration loading
├── extensions/            # Future extensions
├── build/                 # Build scripts and binaries
│   ├── build.sh          # Linux build script
│   └── build.bat         # Windows build script
├── config.json           # Sample configuration
├── Makefile              # Build automation
├── go.mod                # Go module definition
└── README.md             # This file
```

## Development

### Available Make Targets

```bash
make build         # Build for current platform
make build-linux   # Build for Linux
make build-windows # Build for Windows
make build-all     # Build for all platforms
make run           # Build and run with config.json
make clean         # Remove build artifacts
make deps          # Download dependencies
make test          # Run tests
make help          # Show help
```

### Running Tests

```bash
make test
```

## Roadmap

### Phase 1: Core Setup ✅
- [x] Project initialization
- [x] Sing-box integration
- [x] Basic CLI
- [x] Configuration loading
- [x] Cross-platform builds

### Phase 2: Extensions (Planned)
- [ ] Custom protocol support
- [ ] Plugin system
- [ ] Advanced routing

### Phase 3: Management (Planned)
- [ ] Web UI
- [ ] API server
- [ ] Metrics and monitoring

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

See [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built on [Sing-box](https://github.com/SagerNet/sing-box)
- Inspired by modern proxy solutions

## Support

For issues and questions:
- GitHub Issues: https://github.com/UTPBox/utp-core/issues
- Documentation: https://github.com/UTPBox/utp-core/wiki

## Authors

UTPBox Team
