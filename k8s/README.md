# Kubernetes Deployment for Air-Gapped Tunnel Client

This directory contains Kubernetes manifests for deploying the tunnel client in an air-gapped environment.

## Quick Start

### 1. Build and Load Container Image

Since this is for an air-gapped environment, you need to build and transfer the image:

```bash
# Build the image
docker build -f Dockerfile.client -t tunnel-client:latest .

# Save the image to a tarball
docker save tunnel-client:latest > tunnel-client.tar

# Transfer tunnel-client.tar to your air-gapped cluster
# Load the image on each node (or in your registry)
docker load < tunnel-client.tar
```

### 2. Configure for Your Environment

Edit the configuration files:

```bash
# Update tunnel server URL
kubectl edit configmap tunnel-config -n tunnel-system

# Update authentication token (REQUIRED)
kubectl edit secret tunnel-auth -n tunnel-system
```

### 3. Deploy

```bash
# Apply all manifests
kubectl apply -k k8s/

# Or apply individually
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/secret.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/deployment-web.yaml
kubectl apply -f k8s/deployment-k8s-api.yaml
kubectl apply -f k8s/service-k8s-api.yaml
kubectl apply -f k8s/deployment-db.yaml
kubectl apply -f k8s/deployment-ssh.yaml
kubectl apply -f k8s/deployment-mongodb.yaml
```

## Configuration

### Required Changes

Before deploying, you **must** update these values:

1. **Tunnel Server URL** in `configmap.yaml`:
   ```yaml
   TUNNEL_SERVER_URL: "wss://your-tunnel-server.example.com:8443/tunnel"
   ```

2. **Authentication Token** in `secret.yaml`:
   ```yaml
   TUNNEL_TOKEN: "your-production-token-here"
   ```

### Optional Configuration

- **Service Endpoints**: Update the forward configurations in deployments to match your actual service names and ports
  - Web: `webapp:80` → `8080` (HTTP)
  - Database: `database:5432` → `5432` (PostgreSQL)
  - SSH: `ssh-server:22` → `2222` (SSH)
  - MongoDB: `mongodb:27017` → `27017` (MongoDB)
  - **Kubernetes API**: `kubernetes.default.svc.cluster.local:443` → `6443` (K8s API Server)
- **Resource Limits**: Adjust CPU/memory limits based on your cluster capacity
- **Replicas**: Scale deployments as needed (typically 1 replica per tunnel type)

## Monitoring

### Check Deployment Status

```bash
# Check all tunnel client pods
kubectl get pods -n tunnel-system -l app.kubernetes.io/component=tunnel-client

# Check specific tunnel type
kubectl get pods -n tunnel-system -l tunnel.type=web
kubectl get pods -n tunnel-system -l tunnel.type=k8s-api
kubectl get pods -n tunnel-system -l tunnel.type=mongodb

# View logs
kubectl logs -n tunnel-system -l app.kubernetes.io/name=tunnel-client-web -f
kubectl logs -n tunnel-system -l app.kubernetes.io/name=tunnel-client-k8s-api -f
```

### Metrics

Each client is configured with `-metrics=true` to show periodic connection statistics in the logs.

## Troubleshooting

### Common Issues

1. **Image Pull Errors**:
   - Ensure the image is available on all nodes
   - Check `imagePullPolicy: IfNotPresent` in deployments

2. **Connection Failures**:
   - Verify tunnel server URL is accessible from the cluster
   - Check authentication token matches server configuration
   - Review firewall rules for outbound connections

3. **Service Discovery**:
   - Ensure target services (webapp, database, ssh-server, mongodb) exist in the cluster
   - Update service names in deployment args if different

### Debug Commands

```bash
# Check connectivity to tunnel server
kubectl run debug --rm -it --image=alpine -- sh
# Inside pod: nc -zv tunnel-server.external.example.com 8443

# Check service resolution
kubectl run debug --rm -it --image=alpine -- sh  
# Inside pod: nslookup webapp
# Inside pod: nslookup mongodb

# View detailed pod events
kubectl describe pod -n tunnel-system tunnel-client-web-xxx
```

## Security Considerations

- Pods run as non-root user (UID 1000)
- Read-only root filesystem
- All capabilities dropped
- Resource limits enforced
- Secrets managed via Kubernetes secrets

## Architecture

```
[Air-gapped K8s Cluster]          [Internet]              [External Users]
   tunnel-client-web     <-WSS->  Tunnel Server    <-TCP->  Web Access (8080)
   tunnel-client-k8s-api <-WSS->  Tunnel Server    <-TCP->  K8s API Access (6443)
   tunnel-client-db      <-WSS->  Tunnel Server    <-TCP->  DB Access (5432)
   tunnel-client-ssh     <-WSS->  Tunnel Server    <-TCP->  SSH Access (2222)
   tunnel-client-mongodb <-WSS->  Tunnel Server    <-TCP->  MongoDB Access (27017)
```

Each tunnel client:
- Initiates outbound WebSocket connection to tunnel server
- Forwards specific port/service combinations
- Maintains persistent connection with auto-reconnect
- Runs as separate deployment for isolation and scaling