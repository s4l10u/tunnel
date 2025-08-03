# Air-Gapped Tunnel Architecture

## Network Topology

```
┌─────────────────────────────────────────────────────────────────────┐
│                         External Network                             │
│                        (172.20.0.0/24)                              │
│                                                                     │
│  ┌─────────────┐         ┌─────────────────┐    ┌──────────────┐  │
│  │   External  │         │  Tunnel Server  │    │  Tunnel      │  │
│  │    User     │◄────────┤   (Port 8443)   │◄───┤  Clients     │  │
│  │             │         │                 │    │              │  │
│  └─────────────┘         └─────────────────┘    └──────┬───────┘  │
│                                                         │          │
└─────────────────────────────────────────────────────────┼──────────┘
                                                          │
                                                          │ WebSocket
                                                          │ Connection
                                                          │
┌─────────────────────────────────────────────────────────┼──────────┐
│                      Air-Gapped Network                 │          │
│                      (172.21.0.0/24)                    ▼          │
│                      internal: true              ┌──────────────┐  │
│                                                  │  Tunnel      │  │
│  ┌─────────────┐    ┌─────────────┐            │  Clients     │  │
│  │   Web App   │◄───┤  PostgreSQL │            │              │  │
│  │  (Port 80)  │    │ (Port 5432) │            └──────┬───────┘  │
│  └─────────────┘    └─────────────┘                    │          │
│                                                         │          │
│  ┌─────────────┐                                       │          │
│  │ SSH Server  │◄──────────────────────────────────────┘          │
│  │  (Port 22)  │                                                  │
│  └─────────────┘                                                  │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

## Connection Flow

1. **Client Initialization**
   - Air-gapped client starts and connects to tunnel server
   - Authenticates using shared token
   - Establishes persistent WebSocket connection

2. **External User Access**
   - User connects to tunnel server on exposed port (e.g., 8080)
   - Server identifies target service from port mapping
   - Creates new session for this connection

3. **Data Flow**
   - Server sends "connect" message to appropriate client
   - Client connects to local service in air-gapped network
   - Bi-directional data forwarding begins
   - All data is base64 encoded for WebSocket transport

## Security Layers

### Network Isolation
- Air-gapped network marked as `internal: true` in Docker
- No direct routes between external and air-gapped networks
- Only tunnel clients bridge the networks

### Authentication
- Token-based authentication required for all connections
- Tokens should be rotated regularly
- Use environment variables, never hardcode

### Encryption
- TLS/SSL for WebSocket connections (wss://)
- Certificate validation on production
- Optional certificate pinning for extra security

### Access Control
- Each client has unique ID
- Port mappings explicitly configured
- No dynamic port opening

## Docker Network Configuration

```yaml
networks:
  external-network:
    driver: bridge
    ipam:
      config:
        - subnet: 172.20.0.0/24
        
  airgapped-network:
    driver: bridge
    internal: true  # Critical: prevents external access
    ipam:
      config:
        - subnet: 172.21.0.0/24
```

## Component Responsibilities

### Tunnel Server
- Accepts WebSocket connections from clients
- Authenticates clients
- Routes incoming TCP connections to appropriate clients
- Manages session state
- Handles reconnections

### Tunnel Client
- Initiates outbound connection to server
- Maintains persistent WebSocket connection
- Forwards traffic to/from local services
- Handles automatic reconnection
- Enforces access policies

### Message Protocol

```json
// Registration
{"type": "register", "id": "client-id"}

// Forward data
{
  "type": "forward",
  "data": {
    "type": "connect|data|disconnect",
    "sessionId": "unique-session-id",
    "target": "service:port",
    "data": "base64-encoded-payload"
  }
}

// Health check
{"type": "ping"}
{"type": "pong"}
```

## Deployment Considerations

### High Availability
- Run multiple tunnel servers behind load balancer
- Client auto-reconnects to available servers
- Session state needs to be shared (Redis)

### Monitoring
- Track active connections
- Monitor bandwidth usage
- Alert on authentication failures
- Log all connection attempts

### Performance
- Tune buffer sizes for throughput
- Consider compression for large transfers
- Monitor WebSocket ping/pong latency
- Scale horizontally as needed