# Tunnel System Release v1.2.0

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
| `tunnel-v1.2.0-\<os\>-\<arch\>.tar.gz` | Binary archives for each platform |
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

## 🆕 What's New in v1.2.0

### 🎆 **MAJOR: Modern YAML Configuration System**
- **✨ Unlimited Custom Services** - Add Redis, Elasticsearch, any service without code changes
- **⚙️ Environment Variable Overrides** - Runtime configuration with `TUNNEL_FORWARDER_<NAME>_PORT=9090`
- **🛡️ Advanced Validation** - Port conflict detection, range validation, descriptive errors
- **🔄 100% Backward Compatible** - Legacy environment configs still fully supported
- **📝 Self-Documenting** - Rich descriptions and examples in config files

### 🚀 **Enhanced Deployment Experience**
- **🎨 Modern Installation UI** - Enhanced scripts with emojis, progress indicators, clear guidance
- **🎛️ Dual Configuration Support** - Automatic YAML + legacy config generation
- **📚 Comprehensive Documentation** - Updated guides with migration paths and examples
- **🔧 Professional Systemd Integration** - Smart config detection and fallback

### 🛠️ **Technical Improvements**
- **Configuration Priority System** - Command line → YAML → Environment → Defaults
- **Runtime Service Management** - Enable/disable services via environment variables
- **Professional Error Messages** - Descriptive validation with helpful suggestions
- **Enhanced Logging** - Better visibility into configuration loading and validation

## 🎯 Configuration Revolution

### Before (Limited)
```bash
# Only 5 hardcoded services
TUNNEL_WEB_PORT=8080
TUNNEL_DB_PORT=5432
# Can't add custom services
```

### After (Unlimited)
```yaml
forwarders:
  # Add ANY service you want!
  - name: "redis"
    port: 6379
    target: "redis-server:6379"
    client_id: "airgap-redis"
    enabled: true
    description: "Redis cache tunnel"
    
  - name: "elasticsearch"
    port: 9200
    target: "elasticsearch:9200"
    client_id: "airgap-elasticsearch"
    enabled: true
    description: "Search engine tunnel"
```

### Environment Override Power
```bash
# Override any service configuration at runtime
TUNNEL_FORWARDER_REDIS_PORT=7000
TUNNEL_FORWARDER_WEB_TARGET=custom-app:8080
TUNNEL_FORWARDER_ELASTICSEARCH_ENABLED=true
```

## 🐛 Bug Fixes

- **Fixed unused import warnings** in server build
- **Improved YAML dependency management** with go.mod updates
- **Enhanced error messages** for configuration validation
- **Better systemd service configuration** with environment variable support
- **Resolved build issues** with proper dependency handling

## 💡 Migration Guide

### From Legacy Environment Config
```bash
# Old way (still works)
sudo nano /etc/tunnel-server/config

# New way (recommended)
sudo nano /etc/tunnel-server/config.yaml
```

### Configuration Priority
1. **Command line flags** (`-token`, `-listen`, etc.)
2. **YAML config file** (`/etc/tunnel-server/config.yaml`) ← **NEW**
3. **Environment variables** (from `/etc/tunnel-server/config`)
4. **Built-in defaults**

---

## 🚀 Breaking Changes: NONE

This release is **100% backward compatible**. All existing environment variable configurations continue to work without any changes required.

**Upgrade Path**: Simply update binaries and optionally migrate to YAML configuration for enhanced features.
