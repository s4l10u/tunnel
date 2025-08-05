# Tunnel Daemon Installation

This directory contains files for installing and managing both tunnel client and server as Linux systemd daemons.

## 🌟 NEW SECURE ARCHITECTURE

**Client-Controlled Targets**: The tunnel now uses a more secure architecture where:
- **Clients specify targets**: Server only knows ports and client IDs, not your internal network topology
- **Enhanced security**: Air-gapped network topology remains private from the server
- **Zero-trust design**: Server has no knowledge of internal services
- **Client-side access control**: You control which services are accessible through the tunnel

## 📁 Directory Structure

```
daemon/
├── client/                    # Client daemon installation
│   ├── tunnel-client.service  # Systemd service unit file
│   ├── install-daemon.sh      # Installation script (run as root)
│   ├── uninstall-daemon.sh    # Uninstallation script (run as root)
│   └── config.example         # Configuration example
└── server/                    # Server daemon installation
    ├── tunnel-server.service  # Systemd service unit file
    ├── install-server-daemon.sh # Installation script (run as root)
    ├── uninstall-server-daemon.sh # Uninstallation script (run as root)
    └── server-config.example  # Configuration example
```

## 🚀 Quick Installation

## **Client Installation (Air-gapped environment)**

### 1. Install the Client Daemon

```bash
# Make sure you're in the tunnel project root directory
# The script will look for bin/tunnel-client-linux
sudo ./daemon/client/install-daemon.sh
```

### 2. Configure the Service

```bash
# Edit the configuration file
sudo nano /etc/tunnel-client/config

# Required settings (NEW ARCHITECTURE):
# TUNNEL_SERVER_URL=wss://your-server.com:8443/tunnel  # Use wss:// for TLS
# TUNNEL_TOKEN=your-production-token
# TUNNEL_FORWARD=6443:kubernetes.default.svc.cluster.local:443  # CLIENT CONTROLS TARGET
# TUNNEL_CLIENT_ID=airgap-k8s-api

# TLS settings:
# TUNNEL_SKIP_VERIFY=false  # Set to true for self-signed certificates

# NEW: Client-controlled target format
# TUNNEL_FORWARD=serverPort:localTarget:localPort
# Examples:
#   Web app: TUNNEL_FORWARD=8080:webapp:80
#   Database: TUNNEL_FORWARD=5432:database:5432
#   SSH: TUNNEL_FORWARD=2222:ssh-server:22
```

### 3. Start the Service

```bash
# Enable auto-start on boot
sudo systemctl enable tunnel-client

# Start the service
sudo systemctl start tunnel-client

# Check status
sudo systemctl status tunnel-client
```

## **Server Installation (Internet-facing)**

### 1. Install the Server Daemon

```bash
# Make sure you're in the tunnel project root directory
# The script will look for bin/tunnel-server-linux
sudo ./daemon/server/install-server-daemon.sh
```

### 2. Configure the Server

```bash
# RECOMMENDED: Edit YAML configuration (modern, flexible)
sudo nano /etc/tunnel-server/config.yaml

# OR edit legacy environment configuration  
sudo nano /etc/tunnel-server/config

# Required settings (NEW ARCHITECTURE):
# TUNNEL_LISTEN_ADDR=:8443
# TUNNEL_TOKEN=your-production-token
# TUNNEL_CERT_PATH=/etc/tunnel-server/certs/server.crt
# TUNNEL_KEY_PATH=/etc/tunnel-server/certs/server.key

# NEW: Server only defines ports and client IDs
# Targets are controlled by clients for security
# Example forwarder configuration:
#   Port 8080 -> Client "airgap-web" controls target
#   Port 5432 -> Client "airgap-db" controls target
```

### 3. Start the Server Service

```bash
# Enable auto-start on boot
sudo systemctl enable tunnel-server

# Start the service
sudo systemctl start tunnel-server

# Check status
sudo systemctl status tunnel-server
```

## 📋 Management Commands

### Client Service Control
```bash
# Start service
sudo systemctl start tunnel-client

# Stop service
sudo systemctl stop tunnel-client

# Restart service
sudo systemctl restart tunnel-client

# Reload configuration (sends SIGHUP)
sudo systemctl reload tunnel-client

# Enable auto-start
sudo systemctl enable tunnel-client

# Disable auto-start
sudo systemctl disable tunnel-client

# Check status
sudo systemctl status tunnel-client
```

### Server Service Control
```bash
# Start service
sudo systemctl start tunnel-server

# Stop service
sudo systemctl stop tunnel-server

# Restart service
sudo systemctl restart tunnel-server

# Reload configuration (sends SIGHUP)
sudo systemctl reload tunnel-server

# Enable auto-start
sudo systemctl enable tunnel-server

# Disable auto-start
sudo systemctl disable tunnel-server

# Check status
sudo systemctl status tunnel-server
```

### Monitoring & Logs
```bash
# View real-time logs
sudo journalctl -u tunnel-client -f

# View recent logs
sudo journalctl -u tunnel-client -n 50

# View logs from today
sudo journalctl -u tunnel-client --since today

# View logs with timestamps
sudo journalctl -u tunnel-client -o short-iso
```

### Configuration
```bash
# Edit configuration
sudo nano /etc/tunnel-client/config

# Test configuration (dry run)
sudo -u tunnel /opt/tunnel-client/bin/tunnel-client-linux --help

# Reload configuration
sudo systemctl reload tunnel-client
```

## 🔧 Configuration Examples (NEW ARCHITECTURE)

### Kubernetes API Server
```bash
# CLIENT CONTROLS TARGET (more secure)
TUNNEL_SERVER_URL=wss://tunnel.example.com:8443/tunnel
TUNNEL_TOKEN=your-k8s-token
TUNNEL_FORWARD=6443:kubernetes.default.svc.cluster.local:443  # Client specifies target
TUNNEL_CLIENT_ID=airgap-k8s-api
```

### Web Application
```bash
# CLIENT CONTROLS TARGET (server doesn't know about "webapp")
TUNNEL_SERVER_URL=wss://tunnel.example.com:8443/tunnel
TUNNEL_TOKEN=your-web-token
TUNNEL_FORWARD=8080:webapp:80  # Client decides target
TUNNEL_CLIENT_ID=airgap-web
```

### Database
```bash
# CLIENT CONTROLS TARGET (air-gapped topology stays private)
TUNNEL_SERVER_URL=wss://tunnel.example.com:8443/tunnel
TUNNEL_TOKEN=your-db-token
TUNNEL_FORWARD=5432:database:5432  # Client controls which database
TUNNEL_CLIENT_ID=airgap-db
```

## 🔒 TLS Configuration

### **Production with Valid Certificates**
```bash
TUNNEL_SERVER_URL=wss://tunnel.example.com:8443/tunnel
TUNNEL_TOKEN=your-production-token
TUNNEL_FORWARD=6443:kubernetes.default.svc.cluster.local:443
TUNNEL_CLIENT_ID=airgap-k8s-api
TUNNEL_SKIP_VERIFY=false
```

### **Development with Self-Signed Certificates**
```bash
TUNNEL_SERVER_URL=wss://tunnel.example.com:8443/tunnel
TUNNEL_TOKEN=your-dev-token
TUNNEL_FORWARD=6443:kubernetes.default.svc.cluster.local:443
TUNNEL_CLIENT_ID=airgap-k8s-api
TUNNEL_SKIP_VERIFY=true  # Only for development!
```

### **Custom CA Certificate**
```bash
# Copy your CA certificate
sudo cp your-ca.crt /etc/tunnel-client/ca.crt
sudo chown root:root /etc/tunnel-client/ca.crt
sudo chmod 644 /etc/tunnel-client/ca.crt

# Configure daemon
TUNNEL_SERVER_URL=wss://tunnel.example.com:8443/tunnel
TUNNEL_TOKEN=your-token
TUNNEL_FORWARD=6443:kubernetes.default.svc.cluster.local:443
TUNNEL_CLIENT_ID=airgap-k8s-api
TUNNEL_SKIP_VERIFY=false
TUNNEL_CA_CERT=/etc/tunnel-client/ca.crt
```

## 🔍 Troubleshooting

### Check Service Status
```bash
# Detailed status
sudo systemctl status tunnel-client -l

# Check if enabled
sudo systemctl is-enabled tunnel-client

# Check if active
sudo systemctl is-active tunnel-client
```

### Common Issues

#### 1. Service Won't Start
```bash
# Check configuration syntax
sudo -u tunnel /opt/tunnel-client/bin/tunnel-client-linux --help

# Check permissions
ls -la /opt/tunnel-client/bin/
ls -la /etc/tunnel-client/

# Check logs for errors
sudo journalctl -u tunnel-client --since "5 minutes ago"
```

#### 2. Connection Issues
```bash
# Test network connectivity
sudo -u tunnel curl -v wss://your-server:8443/tunnel

# Check firewall
sudo ufw status
sudo iptables -L

# Verify DNS resolution
nslookup your-server.com
```

#### 3. Permission Issues
```bash
# Fix ownership
sudo chown -R tunnel:tunnel /opt/tunnel-client/
sudo chown root:root /etc/tunnel-client/config
sudo chmod 640 /etc/tunnel-client/config
```

#### 4. TLS Certificate Issues
```bash
# For self-signed certificates in development
sudo sed -i 's/TUNNEL_SKIP_VERIFY=false/TUNNEL_SKIP_VERIFY=true/' /etc/tunnel-client/config
sudo systemctl restart tunnel-client

# Test TLS connection manually
openssl s_client -connect your-server.com:8443

# Check certificate validity
curl -v https://your-server.com:8443/health

# For certificate verification errors
# Check server certificate matches the hostname you're connecting to
```

### Log Analysis
```bash
# Error patterns
sudo journalctl -u tunnel-client | grep -i error

# Connection patterns  
sudo journalctl -u tunnel-client | grep -i "connect"

# Performance metrics
sudo journalctl -u tunnel-client | grep -i "metrics"
```

## 🔒 Security Features

The daemon runs with enhanced security:

### System Security
- **Non-root user**: Runs as dedicated `tunnel` user
- **Minimal privileges**: No new privileges, restricted system calls
- **Filesystem protection**: Read-only root filesystem, private /tmp
- **Resource limits**: Memory and CPU limits enforced
- **Automatic restart**: Restarts on failure with backoff

### Architecture Security (NEW)
- **Client-controlled targets**: Server has no knowledge of internal services
- **Zero-trust tunnel design**: Air-gapped network topology remains private
- **Client-side access control**: You control which services are accessible
- **Reduced attack surface**: Server cannot be used to discover internal services

## 📊 Monitoring Integration

### Systemd Integration
- Service status available via `systemctl status`
- Logs integrated with journald
- Automatic restart on failure
- Start/stop dependency management

### Log Monitoring
```bash
# Setup log monitoring with logrotate
sudo tee /etc/logrotate.d/tunnel-client << EOF
/var/log/tunnel-client/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 644 tunnel tunnel
    postrotate
        systemctl reload tunnel-client
    endscript
}
EOF
```

## 🗑️ Uninstallation

```bash
# Client removal
sudo ./daemon/client/uninstall-daemon.sh

# Server removal
sudo ./daemon/server/uninstall-server-daemon.sh

# Each uninstall script will:
# - Stop and disable the respective service
# - Remove systemd service file
# - Delete installation directory
# - Remove user and group
# - Optionally remove configuration
```

## 📁 File Locations

| Type | Location | Owner | Permissions |
|------|----------|-------|-------------|
| Binary | `/opt/tunnel-client/bin/tunnel-client-linux` | tunnel:tunnel | 755 |
| Config | `/etc/tunnel-client/config` | root:root | 640 |
| Logs | `/var/log/tunnel-client/` | tunnel:tunnel | 755 |
| Service | `/etc/systemd/system/tunnel-client.service` | root:root | 644 |
| Working Dir | `/opt/tunnel-client/` | tunnel:tunnel | 755 |

## 🔄 Updates

To update the daemon:

```bash
# Stop service
sudo systemctl stop tunnel-client

# Replace binary
sudo cp bin/tunnel-client-linux /opt/tunnel-client/bin/
sudo chown tunnel:tunnel /opt/tunnel-client/bin/tunnel-client-linux

# Start service
sudo systemctl start tunnel-client

# Verify
sudo systemctl status tunnel-client
```