# Air-Gapped Tunnel Examples

## Scenario 1: Access Web Application in Air-Gapped Network

Your air-gapped network has a web application running on `10.0.0.50:80` that you need to access from outside.

### On the tunnel server (internet-accessible):
```bash
./bin/tunnel-server \
  -token=your-secure-token \
  -cert=/path/to/cert.pem \
  -key=/path/to/key.pem \
  -listen=:443
```

### On the air-gapped client:
```bash
./bin/tunnel-client \
  -server=wss://tunnel.example.com:443/tunnel \
  -token=your-secure-token \
  -forward=8080:10.0.0.50:80 \
  -id=webapp-tunnel
```

### Access from outside:
Users can now access `http://tunnel.example.com:8080` to reach the air-gapped web app.

## Scenario 2: Multiple Services

Access database, SSH, and monitoring services from air-gapped network.

### Create a config file (client-config.sh):
```bash
#!/bin/bash
TOKEN="your-secure-token"
SERVER="wss://tunnel.example.com:443/tunnel"

# Database access
./bin/tunnel-client -server=$SERVER -token=$TOKEN -forward=5432:db.internal:5432 -id=db &

# SSH access
./bin/tunnel-client -server=$SERVER -token=$TOKEN -forward=2222:10.0.0.10:22 -id=ssh &

# Monitoring dashboard
./bin/tunnel-client -server=$SERVER -token=$TOKEN -forward=3000:monitoring.internal:3000 -id=monitoring &
```

## Scenario 3: Development Environment Access

Give developers secure access to air-gapped development resources.

### Server configuration:
```bash
# Use environment variables for sensitive data
export TUNNEL_TOKEN=$(openssl rand -base64 32)

./bin/tunnel-server \
  -token=$TUNNEL_TOKEN \
  -cert=/etc/ssl/tunnel/cert.pem \
  -key=/etc/ssl/tunnel/key.pem
```

### Client configuration:
```bash
# Git server access
./bin/tunnel-client \
  -server=wss://tunnel.dev.company.com/tunnel \
  -token=$TUNNEL_TOKEN \
  -forward=8443:gitlab.internal:443 \
  -id=gitlab

# CI/CD dashboard
./bin/tunnel-client \
  -server=wss://tunnel.dev.company.com/tunnel \
  -token=$TUNNEL_TOKEN \
  -forward=8080:jenkins.internal:8080 \
  -id=jenkins
```

## Security Best Practices

1. **Strong tokens**: Generate with `openssl rand -base64 32`
2. **TLS certificates**: Use proper certificates, not self-signed
3. **Network segmentation**: Limit what the tunnel client can access
4. **Monitoring**: Log all tunnel connections
5. **Access control**: Implement additional authentication on exposed services

## Docker Deployment

### Server Dockerfile:
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o tunnel-server ./server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/tunnel-server /tunnel-server
EXPOSE 8443
ENTRYPOINT ["/tunnel-server"]
```

### Client Dockerfile:
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o tunnel-client ./client

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/tunnel-client /tunnel-client
ENTRYPOINT ["/tunnel-client"]
```

### Docker Compose:
```yaml
version: '3.8'

services:
  tunnel-server:
    build:
      context: .
      dockerfile: Dockerfile.server
    ports:
      - "8443:8443"
    environment:
      - TUNNEL_TOKEN=${TUNNEL_TOKEN}
    volumes:
      - ./certs:/certs:ro
    command: ["-token=${TUNNEL_TOKEN}", "-cert=/certs/server.crt", "-key=/certs/server.key"]

  tunnel-client:
    build:
      context: .
      dockerfile: Dockerfile.client
    environment:
      - TUNNEL_TOKEN=${TUNNEL_TOKEN}
    command: [
      "-server=wss://tunnel-server:8443/tunnel",
      "-token=${TUNNEL_TOKEN}",
      "-skip-verify",
      "-forward=8080:host.docker.internal:80"
    ]
    depends_on:
      - tunnel-server
```

## Monitoring and Logging

### Server logging:
```bash
./bin/tunnel-server -token=$TOKEN 2>&1 | tee tunnel-server.log
```

### Client logging with rotation:
```bash
./bin/tunnel-client -server=$SERVER -token=$TOKEN -forward=$FORWARD 2>&1 | \
  rotatelogs -n 10 tunnel-client.log 10M
```

### Prometheus metrics (future enhancement):
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'tunnel'
    static_configs:
      - targets: ['tunnel-server:9090']
```