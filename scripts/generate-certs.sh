#!/bin/bash

# Certificate Generation Script for Tunnel System
# Usage: ./scripts/generate-certs.sh [hostname]

set -e

# Configuration
HOSTNAME=${1:-localhost}
CERT_DIR="certs"
DAYS=365
KEY_SIZE=2048

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔐 Tunnel Certificate Generator${NC}"
echo "Generating certificates for: ${HOSTNAME}"
echo

# Create certificate directory
mkdir -p "$CERT_DIR"

# Generate private key
echo -e "${YELLOW}📝 Generating private key...${NC}"
openssl genrsa -out "$CERT_DIR/server.key" $KEY_SIZE

# Generate certificate
echo -e "${YELLOW}📜 Generating certificate...${NC}"
openssl req -x509 -nodes -days $DAYS -key "$CERT_DIR/server.key" \
  -out "$CERT_DIR/server.crt" \
  -subj "/C=US/ST=Development/L=Local/O=Tunnel/CN=$HOSTNAME" \
  -extensions v3_req \
  -config <(cat <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
C = US
ST = Development
L = Local
O = Tunnel
CN = $HOSTNAME

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = $HOSTNAME
DNS.2 = localhost
DNS.3 = *.localhost
IP.1 = 127.0.0.1
IP.2 = ::1
EOF
)

# Set permissions (Docker-friendly)
chmod 644 "$CERT_DIR/server.key"
chmod 644 "$CERT_DIR/server.crt"
# Alternative: chmod 644 "$CERT_DIR"/*

echo -e "${GREEN}✅ Certificates generated successfully!${NC}"
echo
echo "Files created:"
echo "  📄 Certificate: $CERT_DIR/server.crt"
echo "  🔑 Private Key: $CERT_DIR/server.key"
echo
echo "Certificate Details:"
openssl x509 -in "$CERT_DIR/server.crt" -noout -text | grep -A 2 "Subject:"
openssl x509 -in "$CERT_DIR/server.crt" -noout -text | grep -A 5 "Subject Alternative Name"
echo
echo "Valid until:"
openssl x509 -in "$CERT_DIR/server.crt" -noout -enddate

echo
echo -e "${BLUE}🚀 Usage:${NC}"
echo "Start server:"
echo "  ./bin/tunnel-server-linux -cert=$CERT_DIR/server.crt -key=$CERT_DIR/server.key"
echo
echo "For development with self-signed certificates:"
echo "  Set TUNNEL_SKIP_VERIFY=true in client configuration"