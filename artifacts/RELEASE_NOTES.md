# Tunnel System Release v1.1.0

Secure tunnel system for establishing connections between air-gapped environments and external networks.

## 🚀 Quick Installation

### One-Line Install (Recommended)

**Client (Air-gapped environment):**
```bash
curl -fsSL https://github.com/s4l10u/tunnel/releases/latest/download/install-client.sh | sudo bash
```

**Server (Internet-facing):**
```bash
curl -fsSL https://github.com/s4l10u/tunnel/releases/latest/download/install-server.sh | sudo bash
```

### Manual Installation

1. Download the appropriate binary for your platform
2. Extract the archive
3. Run the installation script:
   - Client: `sudo ./daemon/client/install-daemon.sh`
   - Server: `sudo ./daemon/server/install-server-daemon.sh`

## 📁 Release Assets

| File | Description |
|------|-------------|
| `install-client.sh` | One-line client installer |
| `install-server.sh` | One-line server installer |
| `tunnel-v1.1.0-\<os\>-\<arch\>.tar.gz` | Binary archives for each platform |
| `daemon.tar.gz` | Daemon installation files |
| `checksums.txt` | SHA256 checksums for verification |

## 🔧 Supported Platforms

- **Linux**: amd64, arm64, 386
- **macOS**: amd64, arm64  
- **Windows**: amd64, 386

## 🔒 Security Features

- TLS/SSL encryption support
- Token-based authentication
- Non-root daemon execution
- Systemd security hardening
- Automatic certificate generation

## 📋 Configuration Examples

### Kubernetes API Server Tunneling
```bash
TUNNEL_SERVER_URL=wss://your-server.com:8443/tunnel
TUNNEL_TOKEN=your-production-token
TUNNEL_FORWARD=6443:kubernetes.default.svc.cluster.local:443
TUNNEL_CLIENT_ID=airgap-k8s-api
```

### Web Application Tunneling
```bash
TUNNEL_SERVER_URL=wss://your-server.com:8443/tunnel
TUNNEL_TOKEN=your-production-token
TUNNEL_FORWARD=8080:webapp:80
TUNNEL_CLIENT_ID=airgap-web
```

## 🔍 Verification

Verify download integrity:
```bash
sha256sum -c checksums.txt
```

## 📚 Documentation

- [Installation Guide](README.md)
- [TLS Setup](TLS-SETUP.md)

## 🆕 What's New in v1.1.0

- Separated daemon installations for client and server
- One-line installation scripts
- Multi-architecture binary releases
- Enhanced security with systemd hardening
- Improved TLS certificate handling
- Fixed critical TLS verification bug

## 🐛 Bug Fixes

- Fixed TLS certificate verification logic in client
- Resolved race condition in TCP session cleanup
- Improved error handling in daemon installation
- Fixed certificate permissions for Docker compatibility
