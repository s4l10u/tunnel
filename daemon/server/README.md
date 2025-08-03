# Tunnel Server Daemon

Install and manage the tunnel server as a Linux systemd daemon. The server runs on internet-facing infrastructure and accepts WebSocket connections from tunnel clients.

## 📁 Files

- **`tunnel-server.service`** - Systemd service unit file
- **`install-server-daemon.sh`** - Installation script (run as root)
- **`uninstall-server-daemon.sh`** - Uninstallation script (run as root)
- **`server-config.example`** - Configuration example

## 🚀 Quick Installation

### 1. Install the Server Daemon

```bash
# From the tunnel project root directory
sudo ./daemon/server/install-server-daemon.sh
```

### 2. Configure the Server

```bash
# Edit the configuration file
sudo nano /etc/tunnel-server/config

# Required settings:
# TUNNEL_LISTEN_ADDR=:8443
# TUNNEL_TOKEN=your-production-token
# TUNNEL_CERT_PATH=/etc/tunnel-server/certs/server.crt
# TUNNEL_KEY_PATH=/etc/tunnel-server/certs/server.key
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

## 🔧 Configuration Examples

### Production with Self-Signed Certificates
```bash
TUNNEL_LISTEN_ADDR=:8443
TUNNEL_TOKEN=your-production-token
TUNNEL_CERT_PATH=/etc/tunnel-server/certs/server.crt
TUNNEL_KEY_PATH=/etc/tunnel-server/certs/server.key
TUNNEL_USE_IMPROVED=true
```

### Production with Let's Encrypt
```bash
TUNNEL_LISTEN_ADDR=:8443
TUNNEL_TOKEN=your-production-token
TUNNEL_CERT_PATH=/etc/letsencrypt/live/your-domain.com/fullchain.pem
TUNNEL_KEY_PATH=/etc/letsencrypt/live/your-domain.com/privkey.pem
TUNNEL_USE_IMPROVED=true
```

### Development without TLS
```bash
TUNNEL_LISTEN_ADDR=:8080
TUNNEL_TOKEN=development-token
TUNNEL_CERT_PATH=
TUNNEL_KEY_PATH=
TUNNEL_USE_IMPROVED=true
```

## 📋 Management Commands

```bash
# Service control
sudo systemctl start tunnel-server
sudo systemctl stop tunnel-server
sudo systemctl restart tunnel-server
sudo systemctl status tunnel-server

# Logs
sudo journalctl -u tunnel-server -f
sudo journalctl -u tunnel-server --since today

# Configuration
sudo nano /etc/tunnel-server/config
sudo systemctl reload tunnel-server
```

## 🔒 Certificate Management

The installation script automatically generates self-signed certificates. For production, use Let's Encrypt:

```bash
# Install certbot
sudo apt install certbot

# Generate certificate
sudo certbot certonly --standalone -d your-domain.com

# Update configuration
sudo nano /etc/tunnel-server/config
# Set TUNNEL_CERT_PATH=/etc/letsencrypt/live/your-domain.com/fullchain.pem
# Set TUNNEL_KEY_PATH=/etc/letsencrypt/live/your-domain.com/privkey.pem

# Restart service
sudo systemctl restart tunnel-server
```

## 🗑️ Uninstallation

```bash
sudo ./daemon/server/uninstall-server-daemon.sh
```

## 📁 File Locations

| Type | Location | Owner | Permissions |
|------|----------|-------|-------------|
| Binary | `/opt/tunnel-server/bin/tunnel-server-linux` | tunnel-server:tunnel-server | 755 |
| Config | `/etc/tunnel-server/config` | root:root | 640 |
| Certs | `/etc/tunnel-server/certs/` | root:root | 755 |
| Logs | `/var/log/tunnel-server/` | tunnel-server:tunnel-server | 755 |
| Service | `/etc/systemd/system/tunnel-server.service` | root:root | 644 |