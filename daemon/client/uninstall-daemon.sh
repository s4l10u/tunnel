#!/bin/bash

# Tunnel Client Daemon Uninstallation Script
# Run with: sudo ./uninstall-daemon.sh

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

confirm_uninstall() {
    echo
    log_warning "This will completely remove the tunnel client daemon from your system."
    log_warning "All configuration files and logs will be deleted."
    echo
    read -p "Are you sure you want to continue? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Uninstallation cancelled."
        exit 0
    fi
}

stop_service() {
    log_info "Stopping and disabling service..."
    
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        systemctl stop "$SERVICE_NAME"
        log_success "Stopped service: $SERVICE_NAME"
    else
        log_info "Service $SERVICE_NAME is not running"
    fi
    
    if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
        systemctl disable "$SERVICE_NAME"
        log_success "Disabled service: $SERVICE_NAME"
    else
        log_info "Service $SERVICE_NAME is not enabled"
    fi
}

remove_service() {
    log_info "Removing systemd service..."
    
    if [[ -f "/etc/systemd/system/$SERVICE_NAME.service" ]]; then
        rm -f "/etc/systemd/system/$SERVICE_NAME.service"
        systemctl daemon-reload
        log_success "Removed systemd service"
    else
        log_info "Service file not found"
    fi
}

remove_files() {
    log_info "Removing installation files..."
    
    # Remove install directory
    if [[ -d "$INSTALL_DIR" ]]; then
        rm -rf "$INSTALL_DIR"
        log_success "Removed directory: $INSTALL_DIR"
    else
        log_info "Directory not found: $INSTALL_DIR"
    fi
    
    # Remove log directory
    if [[ -d "$LOG_DIR" ]]; then
        rm -rf "$LOG_DIR"
        log_success "Removed directory: $LOG_DIR"
    else
        log_info "Directory not found: $LOG_DIR"
    fi
}

remove_config() {
    log_info "Removing configuration..."
    
    if [[ -d "$CONFIG_DIR" ]]; then
        echo
        read -p "Remove configuration directory $CONFIG_DIR? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            rm -rf "$CONFIG_DIR"
            log_success "Removed directory: $CONFIG_DIR"
        else
            log_info "Keeping configuration directory: $CONFIG_DIR"
        fi
    else
        log_info "Configuration directory not found: $CONFIG_DIR"
    fi
}

remove_user() {
    log_info "Removing user and group..."
    
    if getent passwd "$USER" &> /dev/null; then
        userdel "$USER"
        log_success "Removed user: $USER"
    else
        log_info "User $USER not found"
    fi
    
    if getent group "$GROUP" &> /dev/null; then
        groupdel "$GROUP" 2>/dev/null || log_warning "Could not remove group $GROUP (may be in use by other services)"
    else
        log_info "Group $GROUP not found"
    fi
}

cleanup_systemd() {
    log_info "Cleaning up systemd..."
    systemctl daemon-reload
    systemctl reset-failed 2>/dev/null || true
}

main() {
    log_info "Starting tunnel client daemon uninstallation..."
    
    check_root
    confirm_uninstall
    stop_service
    remove_service
    remove_files
    remove_config
    remove_user
    cleanup_systemd
    
    log_success "Uninstallation completed successfully!"
    echo
    log_info "The tunnel client daemon has been completely removed from your system."
}

# Run main function
main "$@"