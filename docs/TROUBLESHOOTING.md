# Troubleshooting Guide

## Common Issues

### 1. Client Cannot Connect to Server

**Symptoms:**
- "connection refused" or "connection timeout" errors
- Client logs show repeated connection attempts

**Solutions:**
```bash
# Check if server is running
docker-compose ps tunnel-server

# Check server logs
docker-compose logs tunnel-server

# Test server health endpoint
curl http://localhost:8443/health

# Verify token matches
echo $TUNNEL_TOKEN
```

### 2. Services Not Accessible Through Tunnel

**Symptoms:**
- Connection established but services return errors
- "connection refused" when accessing forwarded ports

**Solutions:**
```bash
# Check if all clients are running
docker-compose ps | grep tunnel-client

# Verify air-gapped services are running
docker-compose ps webapp database ssh-server

# Test direct connection from client container
docker-compose exec tunnel-client-web wget -O- http://webapp:80

# Check client logs for errors
docker-compose logs tunnel-client-web
```

### 3. WebSocket Connection Drops

**Symptoms:**
- Frequent reconnections in logs
- Intermittent service availability

**Solutions:**
```bash
# Increase timeouts in server
-timeout=120s

# Check network stability
docker-compose exec tunnel-client-web ping -c 10 tunnel-server

# Monitor resource usage
docker stats tunnel-server tunnel-client-web
```

### 4. Authentication Failures

**Symptoms:**
- "Unauthorized" errors in logs
- Clients immediately disconnect

**Solutions:**
```bash
# Verify token format (no extra spaces/newlines)
echo -n "$TUNNEL_TOKEN" | wc -c

# Check Bearer token format in logs
docker-compose logs tunnel-server | grep Authorization

# Regenerate token
export TUNNEL_TOKEN=$(openssl rand -base64 32)
docker-compose down
docker-compose up -d
```

### 5. Port Already in Use

**Symptoms:**
- "bind: address already in use" errors
- Services fail to start

**Solutions:**
```bash
# Find process using port
lsof -i :8443
lsof -i :8080

# Kill process or change port
kill -9 <PID>
# OR
sed -i 's/8443/9443/g' docker-compose.yml
```

### 6. TLS Certificate Issues

**Symptoms:**
- "x509: certificate signed by unknown authority"
- "certificate verify failed"

**Solutions:**
```bash
# For development, use -skip-verify flag
-skip-verify

# Generate proper certificates
make certs

# For production, use Let's Encrypt
certbot certonly --standalone -d tunnel.example.com
```

## Debugging Tools

### 1. Network Connectivity Test
```bash
# Test from external-user container
docker-compose exec external-user sh -c "
  echo '=== Network Test ==='
  echo 'Tunnel server: ' && nc -zv tunnel-server 8443
  echo 'Web (should fail): ' && nc -zv webapp 80 2>&1 | grep -E '(refused|timeout)'
"
```

### 2. Real-time Logs
```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f tunnel-server

# With timestamps
docker-compose logs -f -t tunnel-client-web
```

### 3. Container Shell Access
```bash
# Access client container
docker-compose exec tunnel-client-web sh

# Access server container
docker-compose exec tunnel-server sh

# Test internal connectivity
docker-compose exec tunnel-client-web nc -zv webapp 80
```

### 4. Performance Monitoring
```bash
# CPU and memory usage
docker stats

# Network traffic
docker-compose exec tunnel-server netstat -an | grep ESTABLISHED

# WebSocket connections
docker-compose exec tunnel-server ss -tn | grep :8443
```

## Log Analysis

### Server Logs to Watch For
```
INFO  Starting tunnel server        # Server started successfully
INFO  Client connected             # Client connection established
ERROR Failed to upgrade connection # WebSocket upgrade failed
WARN  Unknown message type         # Protocol mismatch
```

### Client Logs to Watch For
```
INFO  Starting tunnel client       # Client started
INFO  Connected and registered      # Successfully connected
ERROR Connection failed           # Cannot reach server
INFO  Started port forwarding      # Forwarding active
```

## Health Checks

### Manual Health Check Script
```bash
#!/bin/bash
echo "=== Tunnel Health Check ==="

# Server health
echo -n "Server API: "
curl -s http://localhost:8443/health || echo "FAIL"

# Service availability
for port in 8080 5432 2222; do
  echo -n "Port $port: "
  nc -zv localhost $port 2>&1 | grep -q succeeded && echo "OK" || echo "FAIL"
done

# Connection count
echo "Active connections:"
docker-compose exec tunnel-server netstat -an | grep -c ESTABLISHED
```

## Emergency Recovery

### Full System Reset
```bash
# Stop everything
docker-compose down -v

# Clean up
docker system prune -f
rm -rf certs/

# Regenerate certificates
make certs

# Start fresh
make docker-up
```

### Partial Restart
```bash
# Restart only tunnel components
docker-compose restart tunnel-server tunnel-client-web

# Restart with new configuration
docker-compose up -d --force-recreate tunnel-server
```