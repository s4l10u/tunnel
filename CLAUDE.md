# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a secure tunnel system for establishing connections between air-gapped environments and external networks. The system uses WebSocket for transport and supports bi-directional TCP forwarding with reverse tunnel architecture where the air-gapped client initiates outbound connections.

## Architecture

```
[Air-gapped Network]          [Internet]              [User]
     Client        <-WS->    Tunnel Server    <-TCP->  Application
   (Initiates)              (Receives conn)           (Accesses)
```

## Key Components

### Tunnel Server (`server/main.go`)
- **Purpose**: Internet-facing WebSocket server that accepts tunnel connections
- **Port**: 8443 (WebSocket tunnel endpoint)
- **Exposed Ports**: 8080 (web), 5432 (database), 2222 (SSH)
- **Authentication**: Token-based authentication
- **TLS**: Optional (recommended for production)

### Tunnel Client (`client/main.go`)
- **Purpose**: Air-gapped side client that initiates outbound tunnel connections
- **Connection**: Connects to `ws://server:8443/tunnel` via WebSocket
- **Forwarding**: Configurable port forwarding (e.g., `8080:webapp:80`)
- **Auto-reconnect**: Automatically reconnects on disconnect

### Core Tunnel Logic (`pkg/tunnel/`)
- **server.go**: Server implementation with TCP forwarding
- **client.go**: Client implementation with local service connections
- **forward.go**: TCP forwarding and session management

## Common Commands

### Quick Start (Docker Compose)
```bash
# Start the complete air-gapped simulation
make docker-up

# Test all tunnel connections
make docker-test

# View logs
make docker-logs

# Stop all services
make docker-down
```

### Local Development
```bash
# Build binaries
make build

# Run server (development mode without TLS)
make run-server

# Run client with port forwarding
make run-client

# Run tests
make test
```

### Docker Services Access
After running `make docker-up`, access services via:
- **Web app**: http://localhost:8080 → air-gapped nginx
- **PostgreSQL**: `localhost:5432` (user: airgapped, password: airgapped-password)
- **SSH**: `ssh airgapped@localhost -p 2222` (password: airgapped)

## Environment Configuration

### Docker Compose Setup
The system creates two isolated networks:
- **external-network** (172.20.0.0/24): Internet-facing network
- **airgapped-network** (172.22.0.0/24): Isolated network with `internal: true`

### Environment Variables
```bash
TUNNEL_TOKEN=development-token-do-not-use-in-production
```

### Production Deployment
```bash
# Generate certificates
make certs

# Deploy with TLS
make docker-prod
```

## Troubleshooting

### Common Issues

1. **Connection refused on port 8080**
   - Check if tunnel server is running: `docker logs tunnel-server`
   - Restart server: `docker-compose restart tunnel-server`
   - Verify client connection: `docker logs tunnel-client-web`

2. **Empty reply from server**
   - Server may be crashing due to race condition in TCP handler
   - Restart both server and client: `docker-compose restart tunnel-server tunnel-client-web`
   - Wait 5-10 seconds for connection establishment

3. **Client not connecting**
   - Check WebSocket connection: Server should show "Client connected" logs
   - Verify token matches between server and client
   - Check network connectivity between containers

4. **Tunnel server panic: "close of closed channel"**
   - Known issue in `server.go:245` TCP connection handler
   - Restart tunnel server: `docker-compose restart tunnel-server`
   - May require code fix in the TCP session cleanup logic

### Debug Commands
```bash
# Check container status
docker-compose ps

# View all logs with timestamps
docker-compose logs -f --timestamps

# Test specific services
docker exec external-user curl -v webapp:80          # Should fail (isolated)
docker exec external-user curl -v tunnel-server:8080 # Should work via tunnel

# Check network connectivity
docker exec tunnel-client-web nc -zv webapp 80       # Should work
docker exec external-user nc -zv webapp 80           # Should fail
```

### Log Analysis
- **Server logs**: Look for "Client connected/disconnected" and TCP forwarder messages
- **Client logs**: Look for "Connected and registered" and local service connections
- **Panic stack traces**: Usually indicate race conditions in session handling

## Development Guidelines

### Code Structure
```
tunnel/
├── server/main.go           # Server entry point
├── client/main.go           # Client entry point
├── pkg/tunnel/
│   ├── server.go           # WebSocket server + TCP forwarding
│   ├── client.go           # WebSocket client + local connections
│   └── forward.go          # TCP session management
├── examples/webapp/        # Test air-gapped web app
└── scripts/test-tunnel.sh  # Integration testing
```

### Key Configuration Points
- **server/main.go:44-46**: Defines forwarded ports (8080, 5432, 2222)
- **docker-compose.yml:58-62**: Client forwarding configuration
- **pkg/tunnel/server.go**: TCP forwarding and session management

### Testing Strategy
1. **Unit tests**: `go test ./...`
2. **Integration tests**: `./scripts/test-tunnel.sh`
3. **Manual testing**: `curl localhost:8080` after `make docker-up`

## Security Considerations

1. **Token Security**: Change default token in production
2. **TLS**: Always use TLS certificates in production
3. **Network Isolation**: Air-gapped network must have `internal: true`
4. **Access Controls**: Limit which services the tunnel can access
5. **Monitoring**: Log all tunnel connections and data transfers

## Known Limitations

- TCP only (no UDP support)
- No built-in compression
- Single authentication method (token)
- Race condition in TCP session cleanup (causes server crashes)
- No connection multiplexing

## Common Development Tasks

### Adding New Forwarded Port
1. Update `server/main.go` to add new `StartTCPForwarder` call
2. Add new tunnel client in `docker-compose.yml`
3. Update test script to verify new port

### Debugging Connection Issues
1. Check container network connectivity
2. Verify token configuration
3. Review WebSocket handshake logs
4. Test direct service access from client container

### Performance Testing
```bash
# Test concurrent connections
for i in {1..10}; do curl -s localhost:8080 & done

# Monitor tunnel server resources
docker stats tunnel-server
```