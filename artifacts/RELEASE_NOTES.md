# Tunnel System Release v2.0.0

🌟 **MAJOR RELEASE**: Secure Client-Controlled Target Architecture

Secure tunnel system for establishing connections between air-gapped environments and external networks with enhanced zero-trust security model.

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
| `tunnel-v2.0.0-\<os\>-\<arch\>.tar.gz` | Binary archives for each platform |
| `daemon.tar.gz` | Daemon installation files |
| `checksums.txt` | SHA256 checksums for verification |

## 🔧 Supported Platforms

- **Linux**: amd64, arm64, 386
- **macOS**: amd64, arm64  
- **Windows**: amd64, 386

## 🔒 Security Features

### NEW v2.0.0 Architecture Security
- **Client-controlled targets**: Server only knows ports and client IDs
- **Zero-trust tunnel design**: Air-gapped network topology stays private
- **Enhanced security posture**: Server cannot discover internal services
- **Client-side access control**: You control which services are accessible

### System Security
- TLS/SSL encryption support
- Token-based authentication
- Non-root daemon execution
- Systemd security hardening
- Automatic certificate generation

## 📋 Configuration Examples

### Kubernetes API Server Tunneling (NEW v2.0.0 Format)
```bash
# CLIENT CONTROLS TARGET (more secure)
TUNNEL_SERVER_URL=wss://your-server.com:8443/tunnel
TUNNEL_TOKEN=your-production-token
TUNNEL_FORWARD=6443:kubernetes.default.svc.cluster.local:443  # Client specifies target
TUNNEL_CLIENT_ID=airgap-k8s-api
```

### Web Application Tunneling (NEW v2.0.0 Format)
```bash
# CLIENT CONTROLS TARGET (server doesn't know about internal services)
TUNNEL_SERVER_URL=wss://your-server.com:8443/tunnel
TUNNEL_TOKEN=your-production-token
TUNNEL_FORWARD=8080:webapp:80  # Client decides target
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

## 🌟 What's New in v2.0.0

### ⚡ BREAKING CHANGES - New Secure Architecture
- **Client-controlled targets**: Clients now specify which services are accessible
- **Zero-trust design**: Server has no knowledge of air-gapped network topology
- **Enhanced security**: Internal services remain completely private from server
- **New configuration format**: `TUNNEL_FORWARD=serverPort:localTarget:localPort`

### 🔧 Architecture Improvements
- Refactored server to be target-agnostic for better security
- Enhanced client to control target resolution
- Updated message protocol for secure target handling
- Improved Docker Compose with security-focused architecture

### 📚 Documentation Updates
- Updated all daemon installation scripts with new architecture benefits
- Enhanced README with client-controlled target examples
- Improved configuration examples throughout
- Added migration guide for v1.x users

### 💪 Previous Features (v1.2.3)
- Separated daemon installations for client and server
- One-line installation scripts
- Multi-architecture binary releases
- Enhanced security with systemd hardening
- Improved TLS certificate handling

## ⚠️ Migration from v1.x

1. **Server Configuration**: Remove target specifications from server configs
2. **Client Configuration**: Update to new format `TUNNEL_FORWARD=port:target:port`
3. **Docker Compose**: Use new architecture with client-controlled targets
4. **Restart Services**: Deploy new binaries and restart all services

## 🐛 Bug Fixes

- Fixed TLS certificate verification logic in client
- Resolved race condition in TCP session cleanup
- Improved error handling in daemon installation
- Fixed certificate permissions for Docker compatibility
- Removed legacy hardcoded target fallbacks
