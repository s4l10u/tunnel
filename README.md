# Secure Air-Gapped Tunnel System

A secure tunnel system for establishing connections between air-gapped environments and external networks. Uses WebSocket for transport and supports bi-directional TCP forwarding with reverse tunnel architecture.

## 🎯 Overview

```
[Air-gapped Network]          [Internet]              [User]
     Client        <-WSS->    Tunnel Server    <-TCP->  Application
   (Initiates)              (Receives conn)           (Accesses)
```

## 🚀 Quick Installation

### **One-Line Install (Recommended)**

**Client (Air-gapped environment):**
```bash
curl -fsSL https://github.com/s4l10u/tunnel/releases/latest/download/install-client.sh | sudo bash
```

**Server (Internet-facing):**
```bash
curl -fsSL https://github.com/s4l10u/tunnel/releases/latest/download/install-server.sh | sudo bash
```

### **Manual Installation**

#### 1. **Download Release**
Visit [Releases](https://github.com/s4l10u/tunnel/releases) and download the appropriate binary for your platform.

#### 2. **Extract and Install**
```bash
# Extract archive
tar -xzf tunnel-v1.0.0-linux-amd64.tar.gz

# Install daemon
sudo ./daemon/client/install-daemon.sh    # For client
sudo ./daemon/server/install-server-daemon.sh  # For server
```

#### 3. **Configure Service**
```bash
# Client configuration
sudo nano /etc/tunnel-client/config

# Server configuration  
sudo nano /etc/tunnel-server/config
```

#### 4. **Start Services**
```bash
# Enable and start client
sudo systemctl enable tunnel-client
sudo systemctl start tunnel-client

# Enable and start server
sudo systemctl enable tunnel-server
sudo systemctl start tunnel-server
```

## 🛠️ Development Setup

### **Build from Source**
```bash
# Build for Linux (production)
make build-linux

# Build for local development
make build
```

### **Deploy with Docker**
```bash
# Using Docker Compose
docker-compose up -d

# Or deploy manually
./bin/tunnel-server-linux -listen=:8443 -token=your-secure-token
```

#### **Option B: Kubernetes Deployment**
```bash
# Configure for your environment
nano k8s/configmap.yaml
nano k8s/secret.yaml

# Deploy
kubectl apply -k k8s/
```

#### **Option C: Manual**
```bash
./bin/tunnel-client-linux \
  -server=wss://your-server:8443/tunnel \
  -token=your-secure-token \
  -forward=6443:kubernetes.default.svc.cluster.local:443 \
  -id=airgap-k8s-api
```

## 📁 Project Structure

```
tunnel/
├── bin/                    # Compiled binaries
│   ├── tunnel-server-linux
│   └── tunnel-client-linux
├── client/main.go          # Client entry point
├── server/main.go          # Server entry point
├── pkg/tunnel/             # Core tunnel logic
│   ├── server.go          # Server implementation
│   ├── server_improved.go # Enhanced server
│   ├── client.go          # Client implementation
│   ├── client_improved.go # Enhanced client
│   └── forward.go         # TCP forwarding
├── daemon/                # Linux daemon installation
│   ├── install-daemon.sh  # Installation script
│   ├── tunnel-client.service
│   └── config.example
├── k8s/                   # Kubernetes deployment
│   ├── deployment-k8s-api.yaml
│   ├── configmap.yaml
│   └── kustomization.yaml
├── docker-compose.yml     # Docker development setup
├── Dockerfile.server      # Server container
├── Dockerfile.client      # Client container
└── docs/                  # Documentation
    ├── ARCHITECTURE.md
    └── TROUBLESHOOTING.md
```

## 🔧 Configuration

### **Server Configuration**
- **Port**: 8443 (WebSocket tunnel endpoint)
- **Exposed Ports**: 6443 (Kubernetes API)
- **Authentication**: Token-based
- **TLS**: Optional (recommended for production)

### **Client Configuration**
- **Connection**: Connects to `wss://server:8443/tunnel`
- **Forwarding**: Configurable port forwarding
- **Auto-reconnect**: Automatic reconnection on disconnect

## 📋 Common Use Cases

### **Kubernetes API Access**
```bash
# Daemon config
TUNNEL_SERVER_URL=wss://tunnel.example.com:8443/tunnel
TUNNEL_TOKEN=your-k8s-token
TUNNEL_FORWARD=6443:kubernetes.default.svc.cluster.local:443
TUNNEL_CLIENT_ID=airgap-k8s-api
```

### **Database Access**
```bash
TUNNEL_FORWARD=5432:database:5432
TUNNEL_CLIENT_ID=airgap-db
```

### **Web Application**
```bash
TUNNEL_FORWARD=8080:webapp:80
TUNNEL_CLIENT_ID=airgap-web
```

## 🔒 Security Features

- **Token-based authentication**
- **TLS encryption** (WebSocket Secure)
- **Network isolation** (air-gapped client initiates)
- **Systemd security** (non-root user, restricted permissions)
- **Certificate validation** (configurable)

## 📊 Management

### **Daemon Management**
```bash
# Control service
sudo systemctl start/stop/restart tunnel-client

# View logs
sudo journalctl -u tunnel-client -f

# Check status
sudo systemctl status tunnel-client

# Update configuration
sudo nano /etc/tunnel-client/config
sudo systemctl reload tunnel-client
```

### **Kubernetes Management**
```bash
# Check pods
kubectl get pods -n tunnel-system

# View logs
kubectl logs -n tunnel-system -l app.kubernetes.io/name=tunnel-client-k8s-api -f

# Update config
kubectl edit configmap tunnel-config -n tunnel-system
```

## 🔍 Monitoring

### **Connection Status**
- Service status via `systemctl status`
- Logs via `journalctl` or `kubectl logs`
- Metrics in logs (connection attempts, active sessions)

### **Health Checks**
- HTTP health endpoint: `/health`
- Systemd readiness probes
- Kubernetes liveness/readiness probes

## 🐛 Troubleshooting

### **Common Issues**

1. **Connection Refused**
   ```bash
   # Check server status
   curl -v http://server:8443/health
   
   # Check client logs
   sudo journalctl -u tunnel-client -n 50
   ```

2. **Certificate Errors**
   ```bash
   # For development, skip TLS verification
   TUNNEL_SKIP_VERIFY=true
   ```

3. **Permission Denied**
   ```bash
   # Check file permissions
   ls -la /opt/tunnel-client/bin/
   
   # Fix if needed
   sudo chown tunnel:tunnel /opt/tunnel-client/bin/tunnel-client-linux
   ```

## 🔧 Supported Platforms

| Platform | Architecture | Status |
|----------|--------------|--------|
| **Linux** | amd64, arm64, 386 | ✅ Full support |
| **macOS** | amd64, arm64 | ✅ Client only |
| **Windows** | amd64, 386 | ✅ Client only |

## 📦 Release Management

### **Create Release**
```bash
# Build release assets
make release VERSION=v1.2.0

# Create GitHub release (requires gh CLI)
make github-release
```

### **Download Releases**
- **Latest Release**: [GitHub Releases](https://github.com/s4l10u/tunnel/releases/latest)
- **All Releases**: [Release History](https://github.com/s4l10u/tunnel/releases)

### **Verify Downloads**
```bash
# Check integrity
sha256sum -c checksums.txt

# Verify signature (if available)
gpg --verify tunnel-v1.0.0-linux-amd64.tar.gz.sig
```

## 📖 Documentation

- **[TLS Setup](docs/TLS-SETUP.md)** - TLS/SSL encryption configuration
- **[Client Daemon](daemon/client/README.md)** - Client installation guide
- **[Server Daemon](daemon/server/README.md)** - Server installation guide
- **[Troubleshooting](docs/TROUBLESHOOTING.md)** - Common issues and solutions
- **[Daemon Setup](daemon/README.md)** - Linux systemd installation
- **[Kubernetes Deployment](k8s/README.md)** - Container orchestration

## 🏗️ Development

### **Build Commands**
```bash
# Build all
make build

# Build for Linux
make build-linux

# Build server only
make build-server

# Build client only  
make build-client

# Clean build artifacts
make clean
```

### **Testing**
```bash
# Start development environment
make dev

# Test connection
make test

# View logs
make logs
```

## 📄 License

This project is part of the Internal Developer Platform (IDP) and follows the same licensing terms.

## 🤝 Contributing

1. Follow Go conventions and best practices
2. Add tests for new features
3. Update documentation
4. Ensure security standards are maintained# tunnel
# tunnel
