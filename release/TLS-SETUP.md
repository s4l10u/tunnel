# TLS Configuration Guide

This guide explains how to configure TLS encryption for the secure tunnel system.

## 🔒 **Why Use TLS?**

- **Encryption**: All WebSocket traffic is encrypted
- **Authentication**: Server certificate validation
- **Integrity**: Data tampering protection
- **Compliance**: Meet security requirements

## 🎯 **Quick Setup (Self-Signed)**

### 1. **Generate Certificates**
```bash
# Using Makefile
make certs

# Or manually
mkdir -p certs
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout certs/server.key \
  -out certs/server.crt \
  -subj "/C=US/ST=State/L=City/O=Organization/CN=your-server.com"
```

### 2. **Start Server with TLS**
```bash
./bin/tunnel-server-linux \
  -listen=:8443 \
  -token=your-secure-token \
  -cert=certs/server.crt \
  -key=certs/server.key
```

### 3. **Configure Client**
```bash
# Use wss:// (secure WebSocket)
./bin/tunnel-client-linux \
  -server=wss://your-server.com:8443/tunnel \
  -token=your-secure-token \
  -forward=6443:kubernetes.default.svc.cluster.local:443 \
  -id=airgap-k8s-api
```

## 🐳 **Docker with TLS**

### 1. **Generate Certificates**
```bash
make certs
```

### 2. **Set Environment Variables**
```bash
# Create .env file
cat > .env << EOF
TUNNEL_TOKEN=your-secure-production-token
TLS_CERT_PATH=/certs/server.crt
TLS_KEY_PATH=/certs/server.key
EOF
```

### 3. **Start Services**
```bash
docker-compose up -d
```

The certificates in `./certs/` are automatically mounted to `/certs/` in the container.

## ☸️ **Kubernetes with TLS**

### 1. **Create TLS Secret**
```bash
# Generate certificates first
make certs

# Create Kubernetes secret
kubectl create secret tls tunnel-tls \
  --cert=certs/server.crt \
  --key=certs/server.key \
  -n tunnel-system
```

### 2. **Update Server Deployment**
Add to your tunnel server deployment:
```yaml
spec:
  template:
    spec:
      containers:
      - name: tunnel-server
        args:
        - "-cert=/etc/tls/tls.crt"
        - "-key=/etc/tls/tls.key"
        volumeMounts:
        - name: tls-certs
          mountPath: /etc/tls
          readOnly: true
      volumes:
      - name: tls-certs
        secret:
          secretName: tunnel-tls
```

### 3. **Update Client Configuration**
```bash
kubectl edit configmap tunnel-config -n tunnel-system
```

Set the server URL to use `wss://`:
```yaml
data:
  TUNNEL_SERVER_URL: "wss://your-server.com:8443/tunnel"
  SKIP_VERIFY: "false"  # Enable certificate validation
```

## 🔧 **Daemon Configuration**

### 1. **Update Configuration**
```bash
sudo nano /etc/tunnel-client/config
```

Configure for TLS:
```bash
# Use wss:// for secure connection
TUNNEL_SERVER_URL=wss://your-server.com:8443/tunnel
TUNNEL_TOKEN=your-secure-token
TUNNEL_FORWARD=6443:kubernetes.default.svc.cluster.local:443
TUNNEL_CLIENT_ID=airgap-k8s-api

# Certificate validation (set to false for self-signed certs in dev)
TUNNEL_SKIP_VERIFY=false
```

### 2. **Restart Service**
```bash
sudo systemctl restart tunnel-client
```

## 🏭 **Production TLS Setup**

### **1. Using Let's Encrypt (Recommended)**
```bash
# Install certbot
sudo apt install certbot

# Generate certificate
sudo certbot certonly --standalone -d your-server.com

# Certificates will be in:
# /etc/letsencrypt/live/your-server.com/fullchain.pem
# /etc/letsencrypt/live/your-server.com/privkey.pem

# Start server
./bin/tunnel-server-linux \
  -listen=:8443 \
  -token=your-secure-token \
  -cert=/etc/letsencrypt/live/your-server.com/fullchain.pem \
  -key=/etc/letsencrypt/live/your-server.com/privkey.pem
```

### **2. Using Custom CA**
```bash
# Create CA private key
openssl genrsa -out ca.key 4096

# Create CA certificate
openssl req -new -x509 -days 365 -key ca.key -out ca.crt \
  -subj "/C=US/ST=State/L=City/O=MyOrg/CN=MyCA"

# Create server private key
openssl genrsa -out server.key 4096

# Create certificate signing request
openssl req -new -key server.key -out server.csr \
  -subj "/C=US/ST=State/L=City/O=MyOrg/CN=your-server.com"

# Sign server certificate with CA
openssl x509 -req -days 365 -in server.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out server.crt

# Distribute ca.crt to clients for validation
```

### **3. Certificate Management**
```bash
# Auto-renewal script for Let's Encrypt
cat > /etc/cron.d/tunnel-cert-renewal << EOF
0 3 * * * root certbot renew --quiet && systemctl restart tunnel-server
EOF
```

## 🔍 **Troubleshooting TLS**

### **Common Issues**

#### 1. **Certificate Verification Failed**
```bash
# Error: x509: certificate signed by unknown authority

# Solution: Skip verification for self-signed certs
TUNNEL_SKIP_VERIFY=true

# Or add CA certificate to system trust store
sudo cp ca.crt /usr/local/share/ca-certificates/
sudo update-ca-certificates
```

#### 2. **Hostname Mismatch**
```bash
# Error: x509: certificate is valid for localhost, not your-server.com

# Solution: Generate certificate with correct hostname
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout server.key -out server.crt \
  -subj "/CN=your-actual-server.com"
```

#### 3. **Connection Refused**
```bash
# Check if server is listening on correct port
ss -tlnp | grep 8443

# Test TLS connection
openssl s_client -connect your-server.com:8443
```

#### 4. **Mixed Content Issues**
```bash
# Ensure both server and client use same protocol
# Server: https://server:8443 (TLS)
# Client: wss://server:8443/tunnel (WSS)
```

### **Debug Commands**
```bash
# Test certificate
openssl x509 -in certs/server.crt -text -noout

# Check certificate expiry
openssl x509 -in certs/server.crt -noout -dates

# Test TLS handshake
curl -v https://your-server.com:8443/health

# Test WebSocket with TLS
wscat -c wss://your-server.com:8443/tunnel
```

## 🔐 **Security Best Practices**

### **Certificate Management**
- **Use strong key sizes**: Minimum 2048-bit RSA or 256-bit ECDSA
- **Regular rotation**: Rotate certificates every 90 days
- **Secure storage**: Protect private keys with proper file permissions
- **Monitoring**: Monitor certificate expiry dates

### **TLS Configuration**
- **Minimum TLS 1.2**: Disable older protocols
- **Strong cipher suites**: Use AEAD ciphers (AES-GCM, ChaCha20-Poly1305)
- **Certificate validation**: Never skip verification in production
- **HSTS**: Use HTTP Strict Transport Security headers

### **File Permissions**
```bash
# Certificate files should have restrictive permissions
chmod 644 certs/server.crt    # Certificate can be world-readable
chmod 600 certs/server.key    # Private key should be owner-only
chown tunnel:tunnel certs/    # Owned by tunnel service user
```

## 📋 **TLS Configuration Summary**

| Environment | Certificate Type | Configuration |
|-------------|------------------|---------------|
| **Development** | Self-signed | `TUNNEL_SKIP_VERIFY=true` |
| **Staging** | Let's Encrypt | `wss://`, proper validation |
| **Production** | CA-signed | `wss://`, strict validation |

## 🚀 **Quick Reference**

```bash
# Generate self-signed cert
make certs

# Start server with TLS
./bin/tunnel-server-linux -cert=certs/server.crt -key=certs/server.key

# Connect client with TLS
./bin/tunnel-client-linux -server=wss://server:8443/tunnel

# Test TLS connection
curl -k https://server:8443/health
```