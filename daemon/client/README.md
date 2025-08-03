# Tunnel Client Daemon

Install and manage the tunnel client as a Linux systemd daemon. The client runs in air-gapped environments and initiates outbound connections to the tunnel server.

## 📁 Files

- **`tunnel-client.service`** - Systemd service unit file
- **`install-daemon.sh`** - Installation script (run as root)
- **`uninstall-daemon.sh`** - Uninstallation script (run as root)
- **`config.example`** - Configuration example

## 🚀 Quick Installation

### 1. Install the Client Daemon

```bash
# From the tunnel project root directory
sudo ./daemon/client/install-daemon.sh
```

### 2. Configure the Service

```bash
# Edit the configuration file
sudo nano /etc/tunnel-client/config

# Required settings:
# TUNNEL_SERVER_URL=wss://your-server.com:8443/tunnel
# TUNNEL_TOKEN=your-production-token
# TUNNEL_FORWARD=6443:kubernetes.default.svc.cluster.local:443
# TUNNEL_CLIENT_ID=airgap-k8s-api
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

## 🔧 Configuration Examples

### Kubernetes API Server
```bash
TUNNEL_SERVER_URL=wss://tunnel.example.com:8443/tunnel
TUNNEL_TOKEN=your-k8s-token
TUNNEL_FORWARD=6443:kubernetes.default.svc.cluster.local:443
TUNNEL_CLIENT_ID=airgap-k8s-api
TUNNEL_SKIP_VERIFY=false
```

### Web Application
```bash
TUNNEL_SERVER_URL=wss://tunnel.example.com:8443/tunnel
TUNNEL_TOKEN=your-web-token
TUNNEL_FORWARD=8080:webapp:80
TUNNEL_CLIENT_ID=airgap-web
```

## 📋 Management Commands

```bash
# Service control
sudo systemctl start tunnel-client
sudo systemctl stop tunnel-client
sudo systemctl restart tunnel-client
sudo systemctl status tunnel-client

# Logs
sudo journalctl -u tunnel-client -f
sudo journalctl -u tunnel-client --since today

# Configuration
sudo nano /etc/tunnel-client/config
sudo systemctl reload tunnel-client
```

## 🗑️ Uninstallation

```bash
sudo ./daemon/client/uninstall-daemon.sh
```

## 📁 File Locations

| Type | Location | Owner | Permissions |
|------|----------|-------|-------------|
| Binary | `/opt/tunnel-client/bin/tunnel-client-linux` | tunnel:tunnel | 755 |
| Config | `/etc/tunnel-client/config` | root:root | 640 |
| Logs | `/var/log/tunnel-client/` | tunnel:tunnel | 755 |
| Service | `/etc/systemd/system/tunnel-client.service` | root:root | 644 |