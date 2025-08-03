#!/bin/bash

# Tunnel Server Daemon Installation Script
# Run with: sudo ./install-server-daemon.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
INSTALL_DIR="/opt/tunnel-server"
CONFIG_DIR="/etc/tunnel-server"
LOG_DIR="/var/log/tunnel-server"
CERTS_DIR="/etc/tunnel-server/certs"
USER="tunnel-server"
GROUP="tunnel-server"
SERVICE_NAME="tunnel-server"

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root (use sudo)"
        exit 1
    fi
}

check_systemd() {
    if ! command -v systemctl &> /dev/null; then
        log_error "systemd is required but not found"
        exit 1
    fi
}

create_user() {
    log_info "Creating tunnel server user and group..."
    
    if ! getent group "$GROUP" &> /dev/null; then
        groupadd --system "$GROUP"
        log_success "Created group: $GROUP"
    else
        log_info "Group $GROUP already exists"
    fi
    
    if ! getent passwd "$USER" &> /dev/null; then
        useradd --system --gid "$GROUP" --home-dir "$INSTALL_DIR" --shell /usr/sbin/nologin --comment "Tunnel Server Service User" "$USER"
        log_success "Created user: $USER"
    else
        log_info "User $USER already exists"
    fi
}

create_directories() {
    log_info "Creating directories..."
    
    # Install directory
    mkdir -p "$INSTALL_DIR"/{bin,logs}
    chown -R "$USER:$GROUP" "$INSTALL_DIR"
    chmod 755 "$INSTALL_DIR"
    chmod 755 "$INSTALL_DIR/bin"
    chmod 755 "$INSTALL_DIR/logs"
    
    # Config directory
    mkdir -p "$CONFIG_DIR"
    chown root:root "$CONFIG_DIR"
    chmod 755 "$CONFIG_DIR"
    
    # Certificates directory
    mkdir -p "$CERTS_DIR"
    chown root:root "$CERTS_DIR"
    chmod 755 "$CERTS_DIR"
    
    # Log directory
    mkdir -p "$LOG_DIR"
    chown "$USER:$GROUP" "$LOG_DIR"
    chmod 755 "$LOG_DIR"
    
    log_success "Created directories"
}

install_binary() {
    log_info "Installing tunnel server binary..."
    
    # Check for binary in main bin directory first
    if [[ -f "bin/tunnel-server-linux" ]]; then
        BINARY_PATH="bin/tunnel-server-linux"
    elif [[ -f "daemon/bin/tunnel-server-linux" ]]; then
        BINARY_PATH="daemon/bin/tunnel-server-linux"
        log_info "Using binary from daemon/bin directory"
    else
        log_error "Binary not found in bin/ or daemon/bin/"
        log_error "Please run this script from the tunnel project root directory"
        log_error "Make sure bin/tunnel-server-linux exists (run: GOOS=linux GOARCH=amd64 go build -o bin/tunnel-server-linux server/main.go)"
        exit 1
    fi
    
    cp "$BINARY_PATH" "$INSTALL_DIR/bin/tunnel-server-linux"
    chown "$USER:$GROUP" "$INSTALL_DIR/bin/tunnel-server-linux"
    chmod 755 "$INSTALL_DIR/bin/tunnel-server-linux"
    
    log_success "Installed binary from $BINARY_PATH to $INSTALL_DIR/bin/tunnel-server-linux"
}

install_config() {
    log_info "Installing configuration..."
    
    if [[ ! -f "$CONFIG_DIR/config" ]]; then
        cp "daemon/server/server-config.example" "$CONFIG_DIR/config"
        chown root:root "$CONFIG_DIR/config"
        chmod 640 "$CONFIG_DIR/config"
        log_success "Created configuration file: $CONFIG_DIR/config"
        log_warning "IMPORTANT: Edit $CONFIG_DIR/config with your settings before starting the service"
    else
        log_info "Configuration file already exists: $CONFIG_DIR/config"
    fi
}

install_service() {
    log_info "Installing systemd service..."
    
    if [[ ! -f "daemon/server/tunnel-server.service" ]]; then
        log_error "Service file not found: daemon/server/tunnel-server.service"
        exit 1
    fi
    
    cp "daemon/server/tunnel-server.service" "/etc/systemd/system/"
    chmod 644 "/etc/systemd/system/tunnel-server.service"
    
    systemctl daemon-reload
    
    log_success "Installed systemd service"
}

generate_certificates() {
    log_info "Generating self-signed certificates..."
    
    if [[ ! -f "$CERTS_DIR/server.crt" ]]; then
        # Get server's public IP or hostname
        SERVER_IP=$(curl -s ifconfig.me 2>/dev/null || echo "localhost")
        
        openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
            -keyout "$CERTS_DIR/server.key" \
            -out "$CERTS_DIR/server.crt" \
            -subj "/C=US/ST=State/L=City/O=Tunnel/CN=$SERVER_IP" \
            -extensions v3_req \
            -config <(cat <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no
[req_distinguished_name]
C = US
ST = State
L = City
O = Tunnel
CN = $SERVER_IP
[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = localhost
DNS.2 = $SERVER_IP
IP.1 = 127.0.0.1
IP.2 = $SERVER_IP
EOF
        )
        
        # Set proper permissions for tunnel server to read certificates
        chmod 644 "$CERTS_DIR/server.crt"
        chmod 640 "$CERTS_DIR/server.key"
        chown root:$GROUP "$CERTS_DIR/server.crt" "$CERTS_DIR/server.key"
        
        log_success "Generated self-signed certificates for $SERVER_IP"
        log_info "Certificate: $CERTS_DIR/server.crt"
        log_info "Private key: $CERTS_DIR/server.key"
    else
        log_info "Certificates already exist in $CERTS_DIR"
        
        # Ensure existing certificates have correct permissions
        if [[ -f "$CERTS_DIR/server.crt" ]]; then
            chmod 644 "$CERTS_DIR/server.crt"
            chown root:$GROUP "$CERTS_DIR/server.crt"
            log_info "Updated certificate permissions"
        fi
        
        if [[ -f "$CERTS_DIR/server.key" ]]; then
            chmod 640 "$CERTS_DIR/server.key"
            chown root:$GROUP "$CERTS_DIR/server.key"
            log_info "Updated private key permissions"
        fi
    fi
}

main() {
    log_info "Starting tunnel server daemon installation..."
    
    check_root
    check_systemd
    create_user
    create_directories
    install_binary
    install_config
    install_service
    generate_certificates
    
    log_success "Installation completed successfully!"
    echo
    log_info "Next steps:"
    echo "1. Edit configuration: sudo nano $CONFIG_DIR/config"
    echo "2. Enable service: sudo systemctl enable $SERVICE_NAME"
    echo "3. Start service: sudo systemctl start $SERVICE_NAME"
    echo "4. Check status: sudo systemctl status $SERVICE_NAME"
    echo "5. View logs: sudo journalctl -u $SERVICE_NAME -f"
    echo
    log_info "Certificate files:"
    echo "  Certificate: $CERTS_DIR/server.crt"
    echo "  Private key: $CERTS_DIR/server.key"
    echo
    log_warning "For production, consider using Let's Encrypt certificates"
    echo "Generate with: sudo certbot certonly --standalone -d your-domain.com"
}

# Run main function
main "$@"