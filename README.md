# UTP-Core - Universal Multi-Platform Tunneling Framework

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org/dl/)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20Windows%20%7C%20macOS%20%7C%20Android%20%7C%20iOS-lightgrey.svg)]()

UTP-Core is a comprehensive tunneling framework built on top of Sing-box that extends it with support for 40+ proxy, VPN, and obfuscation protocols. It's designed to be both a command-line application and a backend core for mobile VPN applications.

## 🚀 Features

### Core Capabilities
- **40+ Protocol Support** - OpenVPN, SSH (all variants), DNS (DoH/DoT/DNSCrypt/DoQ), Obfuscation, WARP, Psiphon, HTTP Injection, Stealth, Legacy VPN, and Experimental protocols
- **Multi-Platform** - Native binaries for Linux, Windows, macOS, and mobile platforms
- **Mobile Ready** - Android AAR and iOS Framework for Flutter VPN apps
- **WebAssembly** - Browser-compatible WASM version for Chrome/Edge/Firefox
- **Sing-box Compatible** - Full compatibility with Sing-box configuration format
- **Modular Design** - Use only the protocols you need with Go build tags

### Supported Protocols

#### Traditional VPN & Proxy
- **OpenVPN** (TCP/UDP)
- **L2TP/IPsec**
- **IKEv2/IPsec**
- **SSTP**
- **PPTP**
- **SoftEther**

#### SSH Family (All Variants)
- SSH-Direct, SSH-Proxy, SSH-Payload
- SSH-Proxy-Payload, SSH-TLS, SSH-TLS-Proxy
- SSH-TLS-Payload, SSH-TLS-Proxy-Payload
- SSH-DNSTT, SSH-QUIC, SSH-over-WebSocket

#### DNS & Covert Channels
- DNS-over-HTTPS (DoH)
- DNS-over-TLS (DoT)
- DNSCrypt
- DNS-over-QUIC (DoQ)
- SlowDNS (SSH-over-DNS)
- ICMP Tunneling
- TCP-over-DNS / UDP-over-DNS

#### Obfuscation & Pluggable Transports
- **Obfs4** (Pluggable Transport)
- **Meek** (Amazon/Azure/Google fronting)
- **NaiveProxy** (HTTPS with method obfuscation)
- **Cloak** (Protocol obfuscation)
- **FTEProxy** (Format-Transforming Encryption)
- **ScrambleSuit** (Probabilistic encryption)
- **UDP2RAW** (UDP over TCP)
- **Snowflake** (Broker-based proxy)

#### Modern Services
- **Cloudflare WARP** (Standard, Plus, Team modes)
- **Psiphon** (SSH, QUIC-Go, Meek, Obfs3 variants)

#### HTTP Injection & Manipulation
- HTTP Payload Injection
- WebSocket Injection
- HTTP CONNECT tunneling
- HTTP Fronting (Traffic disguise)
- Chunked HTTP encoding
- Header/payload manipulation

#### Steganography & Covert Tunnels
- Image Steganography (LSB method)
- Audio Steganography (Spread Spectrum)
- Email-based Tunneling
- DNS-based Tunneling
- ICMP-based Tunneling
- Carrier File Steganography

#### Legacy & Experimental
- **L2TP**, **IKEv2**, **SSTP**, **PPTP**, **GRE**
- **MASQUE** (HTTP/3 tunneling)
- **Oblivious HTTP** (Privacy proxy)
- **WebRTC DataChannel** tunneling
- **ZeroTier**, **Nebula**, **N2N** mesh networks
- **MQTT VPN** (Experimental)

## 📦 Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/UTPBox/utp-core.git
cd utp-core

# Build with all protocols
./build/build.sh --full

# Build with minimal protocol set
./build/build.sh --minimal

# Build for specific platforms
./build/build.sh --platform linux,darwin --arch amd64
```

### Mobile Build

```bash
# Build Android and iOS libraries
./build/mobile.sh --full

# Build only Android AAR
./build/mobile.sh --android --minimal

# Build only iOS Framework
./build/mobile.sh --ios --standard
```

### WebAssembly Build

```bash
# Build WASM version
./build/wasm.sh --minimal

# Serve locally for testing
cd wasm
python -m http.server 8000
# Open http://localhost:8000 in browser
```

## 🛠️ Configuration

UTP-Core uses Sing-box compatible JSON configuration files:

```json
{
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
    },
    {
      "type": "openvpn",
      "server": "vpn.example.com",
      "port": 1194,
      "protocol": "udp",
      "ca": "-----BEGIN CERTIFICATE-----\n...",
      "cert": "-----BEGIN CERTIFICATE-----\n...",
      "key": "-----BEGIN PRIVATE KEY-----\n..."
    }
  ],
  "route": {
    "rules": [
      {
        "domain": ["*.google.com"],
        "outbound": "warp"
      },
      {
        "protocol": "http",
        "outbound": "openvpn"
      }
    ]
  }
}
```

## 🚀 Usage

### Command Line

```bash
# Run with default config
./utp-core

# Run with custom config
./utp-core config.json

# Test specific protocol
./utp-core --protocol openvpn --server vpn.example.com
```

### As a Service

```bash
# Install as system service (Linux)
sudo cp utp-core /usr/local/bin/
sudo cp config.json /etc/utp-core/
sudo systemctl enable utp-core
sudo systemctl start utp-core
```

### Flutter Integration

#### Android (AAR)
```dart
// pubspec.yaml
dependencies:
  flutter:
    sdk: flutter
  utp_core_android:
    path: ./mobile/utp-core-android.aar
```

```dart
import 'package:utp_core_android/utp_core.dart';

final utpCore = UTPCore();
await utpCore.start(config: configJson);
```

#### iOS (Framework)
```swift
import UTPFramework

let utpCore = UTPFramework()
try utpCore.start(config: configData)
```

## 🏗️ Build Tags

Control which protocols are included in your build:

```bash
# Minimal (faster compilation, smaller binary)
-tags "with_openvpn with_ssh with_warp with_psiphon"

# Standard (recommended for most use cases)
-tags "with_openvpn with_ssh with_dns with_obfs with_warp with_psiphon"

# Full (all protocols)
-tags "with_openvpn with_ssh with_dns with_obfs with_warp with_psiphon with_httpinject with_stealth with_legacyvpn with_experimental"

# Individual protocols
-tags "with_openvpn with_ssh"
-tags "with_warp with_psiphon"
```

## 📱 Mobile Development

### Flutter VPN App Integration

1. **Build Mobile Libraries**
   ```bash
   ./build/mobile.sh --full
   ```

2. **Add to Flutter Project**
   ```dart
   // android/app/build.gradle
   dependencies {
       implementation files('libs/utp-core-android.aar')
   }
   ```

3. **Use in Flutter Code**
   ```dart
   import 'package:utp_core_android/utp_core.dart';
   
   class VPNService {
     late UTPCore utpCore;
     
     Future<void> start() async {
       utpCore = UTPCore();
       await utpCore.start(config: _getConfig());
     }
     
     String _getConfig() {
       // Return UTP-Core configuration
       return jsonEncode({
         'log': {'level': 'info'},
         'inbounds': [
           {
             'type': 'mixed',
             'listen': '127.0.0.1',
             'listen_port': 7890
           }
         ],
         'outbounds': [
           {
             'type': 'warp',
             'server': '162.159.192.1',
             'port': 2408
           }
         ]
       });
     }
   }
   ```

## 🌐 WebAssembly Usage

The WASM version supports a subset of protocols due to browser security restrictions:

- HTTP/HTTPS proxy modes
- WebSocket-based protocols
- DNS-over-HTTPS
- Some obfuscation methods

```html
<!-- Include in your web app -->
<script src="utp-core.js"></script>
<script>
  const go = new Go();
  WebAssembly.instantiateStreaming(fetch("utp-core.wasm"), go.importObject)
    .then(result => {
      go.run(result.instance);
      // UTP-Core is now available
    });
</script>
```

## 🔧 Development

### Project Structure

```
utp-core/
├── cmd/utp-core/
│   └── main.go                 # Main entry point
├── extensions/                 # Protocol implementations
│   ├── openvpn/               # OpenVPN support
│   ├── ssh/                   # SSH variants
│   ├── dns/                   # DNS protocols
│   ├── obfs/                  # Obfuscation
│   ├── warp/                  # Cloudflare WARP
│   ├── psiphon/               # Psiphon protocol
│   ├── httpinject/            # HTTP injection
│   ├── stealth/               # Steganography
│   ├── legacyvpn/             # Legacy VPN protocols
│   └── experimental/          # Experimental protocols
├── internal/utils/            # Shared utilities
├── build/                     # Build scripts
│   ├── build.sh               # Desktop build
│   ├── mobile.sh              # Mobile build
│   └── wasm.sh                # WASM build
├── config.json               # Example configuration
├── go.mod                    # Go module
└── README.md                 # This file
```

### Adding New Protocols

1. Create extension directory: `extensions/newprotocol/`
2. Implement outbound in `outbound.go`
3. Register in `init()` function
4. Update build scripts with new tags
5. Add example config to `config.json`

### Testing

```bash
# Run tests
go test ./...

# Test specific extension
go test ./extensions/openvpn/...

# Build and test
go build ./cmd/utp-core
./utp-core config.json
```

## 📊 Performance

### Binary Sizes (approximate)
- **Minimal**: ~8-12 MB
- **Standard**: ~15-25 MB  
- **Full**: ~25-40 MB
- **Mobile AAR**: ~5-10 MB per ABI
- **iOS Framework**: ~8-15 MB
- **WASM**: ~2-5 MB

### Protocol Performance
- **High Performance**: SSH, HTTP Injection, WARP
- **Medium Performance**: OpenVPN, DNS protocols, Obfuscation
- **Specialized**: Steganography, Experimental protocols

## 🔒 Security Considerations

- All protocols use standard security implementations
- TLS configurations support custom certificates
- SSH implementations use modern key exchange algorithms
- Obfuscation protocols provide traffic analysis resistance
- Steganography supports various carrier files

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature-name`
3. Commit changes: `git commit -am 'Add feature'`
4. Push to branch: `git push origin feature-name`
5. Submit a Pull Request

### Development Guidelines

- Follow Go coding standards
- Add tests for new protocols
- Update documentation
- Use appropriate build tags
- Maintain Sing-box compatibility

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Sing-box](https://github.com/Sagernet/sing-box) - Core framework
- [Go](https://golang.org/) - Programming language
- [gomobile](https://github.com/golang/go/wiki/Mobile) - Mobile bindings
- [TinyGo](https://tinygo.org/) - WebAssembly compiler

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/UTPBox/utp-core/issues)
- **Discussions**: [GitHub Discussions](https://github.com/UTPBox/utp-core/discussions)
- **Wiki**: [Project Wiki](https://github.com/UTPBox/utp-core/wiki)

## 🗺️ Roadmap

- [ ] More SSH protocol variants
- [ ] Additional obfuscation methods
- [ ] Enhanced steganography algorithms
- [ ] Performance optimizations
- [ ] More mobile platform support
- [ ] WebRTC improvements
- [ ] Configuration validation
- [ ] Plugin system for custom protocols

---

**UTP-Core** - Making tunneling universal across all platforms and protocols.

