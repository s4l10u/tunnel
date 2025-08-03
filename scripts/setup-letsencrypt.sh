#!/bin/bash

# Let's Encrypt Setup Script for Tunnel System
# Usage: ./scripts/setup-letsencrypt.sh your-domain.com

set -e

DOMAIN=$1
EMAIL=${2:-admin@$DOMAIN}

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

if [[ -z "$DOMAIN" ]]; then
    echo -e "${RED}❌ Error: Domain name required${NC}"
    echo "Usage: $0 <domain> [email]"
    echo "Example: $0 tunnel.example.com admin@example.com"
    exit 1
fi

echo -e "${BLUE}🔐 Let's Encrypt Setup for Tunnel System${NC}"
echo "Domain: $DOMAIN"
echo "Email: $EMAIL"
echo

# Check if certbot is installed
if ! command -v certbot &> /dev/null; then
    echo -e "${YELLOW}📦 Installing certbot...${NC}"
    if command -v apt &> /dev/null; then
        sudo apt update
        sudo apt install -y certbot
    elif command -v yum &> /dev/null; then
        sudo yum install -y certbot
    else
        echo -e "${RED}❌ Error: Please install certbot manually${NC}"
        exit 1
    fi
fi

# Check if port 80 is available
if ss -tlnp | grep -q ":80 "; then
    echo -e "${YELLOW}⚠️  Warning: Port 80 is in use. Stopping services...${NC}"
    # Try to stop common web servers
    sudo systemctl stop nginx 2>/dev/null || true
    sudo systemctl stop apache2 2>/dev/null || true
    sudo systemctl stop httpd 2>/dev/null || true
fi

# Generate certificate
echo -e "${YELLOW}📜 Generating Let's Encrypt certificate...${NC}"
sudo certbot certonly \
    --standalone \
    --non-interactive \
    --agree-tos \
    --email "$EMAIL" \
    -d "$DOMAIN"

if [[ $? -eq 0 ]]; then
    echo -e "${GREEN}✅ Certificate generated successfully!${NC}"
    
    CERT_PATH="/etc/letsencrypt/live/$DOMAIN"
    
    echo
    echo "Certificate files:"
    echo "  📄 Certificate: $CERT_PATH/fullchain.pem"
    echo "  🔑 Private Key: $CERT_PATH/privkey.pem"
    echo
    
    # Show certificate details
    echo "Certificate Details:"
    sudo openssl x509 -in "$CERT_PATH/fullchain.pem" -noout -text | grep -A 2 "Subject:"
    echo
    echo "Valid until:"
    sudo openssl x509 -in "$CERT_PATH/fullchain.pem" -noout -enddate
    
    echo
    echo -e "${BLUE}🚀 Usage:${NC}"
    echo "Start tunnel server:"
    echo "  sudo ./bin/tunnel-server-linux \\"
    echo "    -listen=:8443 \\"
    echo "    -token=your-secure-token \\"
    echo "    -cert=$CERT_PATH/fullchain.pem \\"
    echo "    -key=$CERT_PATH/privkey.pem"
    
    echo
    echo -e "${BLUE}🔄 Auto-renewal:${NC}"
    echo "Setup automatic renewal:"
    echo "  echo '0 3 * * * root certbot renew --quiet && systemctl restart tunnel-server' | sudo tee /etc/cron.d/tunnel-cert-renewal"
    
    echo
    echo -e "${BLUE}⚙️  Client Configuration:${NC}"
    echo "Update /etc/tunnel-client/config:"
    echo "  TUNNEL_SERVER_URL=wss://$DOMAIN:8443/tunnel"
    echo "  TUNNEL_SKIP_VERIFY=false"
    
else
    echo -e "${RED}❌ Certificate generation failed!${NC}"
    echo
    echo "Common issues:"
    echo "  - Domain must point to this server's public IP"
    echo "  - Port 80 must be accessible from the internet"
    echo "  - Firewall must allow inbound traffic on port 80"
    echo
    echo "Check DNS:"
    echo "  dig +short $DOMAIN"
    echo
    echo "Check firewall:"
    echo "  sudo ufw status"
    echo "  sudo iptables -L"
    exit 1
fi