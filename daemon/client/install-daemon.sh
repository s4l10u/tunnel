#!/bin/bash

# Tunnel Client Daemon Installation Script
# Run with: sudo ./install-daemon.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
INSTALL_DIR="/opt/tunnel-client"
CONFIG_DIR="/etc/tunnel-client"
LOG_DIR="/var/log/tunnel-client"
USER="tunnel"
GROUP="tunnel"
SERVICE_NAME="tunnel-client"

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
    log_info "Creating tunnel user and group..."
    
    if ! getent group "$GROUP" &> /dev/null; then
        groupadd --system "$GROUP"
        log_success "Created group: $GROUP"
    else
        log_info "Group $GROUP already exists"
    fi
    
    if ! getent passwd "$USER" &> /dev/null; then
        useradd --system --gid "$GROUP" --home-dir "$INSTALL_DIR" --shell /usr/sbin/nologin --comment "Tunnel Client Service User" "$USER"
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
    
    # Log directory
    mkdir -p "$LOG_DIR"
    chown "$USER:$GROUP" "$LOG_DIR"
    chmod 755 "$LOG_DIR"
    
    log_success "Created directories"
}

install_binary() {
    log_info "Installing tunnel client binary..."
    
    # Check for binary in main bin directory first
    if [[ -f "bin/tunnel-client-linux" ]]; then
        BINARY_PATH="bin/tunnel-client-linux"
    elif [[ -f "daemon/bin/tunnel-client-linux" ]]; then
        BINARY_PATH="daemon/bin/tunnel-client-linux"
        log_info "Using binary from daemon/bin directory"
    else
        log_error "Binary not found in bin/ or daemon/bin/"
        log_error "Please run this script from the tunnel project root directory"
        log_error "Make sure bin/tunnel-client-linux exists (run: GOOS=linux GOARCH=amd64 go build -o bin/tunnel-client-linux client/main.go)"
        exit 1
    fi
    
    cp "$BINARY_PATH" "$INSTALL_DIR/bin/tunnel-client-linux"
    chown "$USER:$GROUP" "$INSTALL_DIR/bin/tunnel-client-linux"
    chmod 755 "$INSTALL_DIR/bin/tunnel-client-linux"
    
    log_success "Installed binary from $BINARY_PATH to $INSTALL_DIR/bin/tunnel-client-linux"
}

install_config() {
    log_info "Installing configuration..."
    
    if [[ ! -f "$CONFIG_DIR/config" ]]; then
        cp "daemon/client/config.example" "$CONFIG_DIR/config"
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
    
    if [[ ! -f "daemon/client/tunnel-client.service" ]]; then
        log_error "Service file not found: daemon/client/tunnel-client.service"
        exit 1
    fi
    
    cp "daemon/client/tunnel-client.service" "/etc/systemd/system/"
    chmod 644 "/etc/systemd/system/tunnel-client.service"
    
    systemctl daemon-reload
    
    log_success "Installed systemd service"
}

main() {
    log_info "Starting tunnel client daemon installation..."
    
    check_root
    check_systemd
    create_user
    create_directories
    install_binary
    install_config
    install_service
    
    log_success "Installation completed successfully!"
    echo
    log_info "Next steps:"
    echo "1. Edit configuration: sudo nano $CONFIG_DIR/config"
    echo "2. Enable service: sudo systemctl enable $SERVICE_NAME"
    echo "3. Start service: sudo systemctl start $SERVICE_NAME"
    echo "4. Check status: sudo systemctl status $SERVICE_NAME"
    echo "5. View logs: sudo journalctl -u $SERVICE_NAME -f"
}

# Run main function
main "$@"