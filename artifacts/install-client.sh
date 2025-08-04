#!/bin/bash
# Tunnel Client One-Line Installer
# Usage: curl -fsSL https://github.com/s4l10u/tunnel/releases/latest/download/install-client.sh | sudo bash

set -e

# Configuration
REPO="s4l10u/tunnel"
GITHUB_API="https://api.github.com/repos/$REPO"
GITHUB_RELEASES="https://github.com/$REPO/releases"
INSTALL_DIR="/tmp/tunnel-install"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Detect architecture
detect_arch() {
    local arch
    arch=$(uname -m)
    case $arch in
        x86_64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        i386|i686) echo "386" ;;
        *) log_error "Unsupported architecture: $arch"; exit 1 ;;
    esac
}

# Detect OS
detect_os() {
    local os
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case $os in
        linux) echo "linux" ;;
        darwin) echo "darwin" ;;
        *) log_error "Unsupported OS: $os"; exit 1 ;;
    esac
}

# Get latest release
get_latest_version() {
    curl -s "$GITHUB_API/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
}

main() {
    log_info "Installing Tunnel Client..."
    
    # Check if running as root
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root (use sudo)"
        exit 1
    fi
    
    # Detect system
    local os arch version
    os=$(detect_os)
    arch=$(detect_arch)
    version=$(get_latest_version)
    
    log_info "Detected: $os/$arch, Version: $version"
    
    # Create temp directory
    mkdir -p "$INSTALL_DIR"
    cd "$INSTALL_DIR"
    
    # Download and extract
    local archive_name="tunnel-$version-$os-$arch.tar.gz"
    local download_url="$GITHUB_RELEASES/download/$version/$archive_name"
    
    log_info "Downloading $archive_name..."
    curl -fsSL "$download_url" -o "$archive_name"
    tar -xzf "$archive_name"
    
    # Download daemon files
    log_info "Downloading installation files..."
    curl -fsSL "$GITHUB_RELEASES/download/$version/daemon.tar.gz" -o daemon.tar.gz
    tar -xzf daemon.tar.gz
    
    # Copy binary to expected location
    mkdir -p bin
    cp "tunnel-client-$os-$arch" bin/tunnel-client-linux
    
    # Run installation
    log_info "Installing daemon..."
    ./daemon/client/install-daemon.sh
    
    # Cleanup
    cd /
    rm -rf "$INSTALL_DIR"
    
    log_success "🎉 Tunnel client installed successfully!"
    echo
    log_info "📋 Next steps:"
    echo "1. 🔧 Edit configuration: sudo nano /etc/tunnel-client/config"
    echo "   - Set TUNNEL_SERVER_URL (wss://your-server:8443/tunnel)"
    echo "   - Set TUNNEL_TOKEN (must match server token)"
    echo "   - Set TUNNEL_FORWARD (e.g., 8080:webapp:80)"
    echo "   - Set TUNNEL_CLIENT_ID (e.g., airgap-web)"
    echo "2. 🔄 Enable service: sudo systemctl enable tunnel-client"
    echo "3. ▶️  Start service: sudo systemctl start tunnel-client"
    echo "4. 📊 Check status: sudo systemctl status tunnel-client"
    echo "5. 📜 View logs: sudo journalctl -u tunnel-client -f"
    echo
    log_info "🚀 Quick Start Example:"
    echo "  # Edit /etc/tunnel-client/config with your settings:"
    echo "  TUNNEL_SERVER_URL=wss://tunnel.example.com:8443/tunnel"
    echo "  TUNNEL_TOKEN=your-production-token"
    echo "  TUNNEL_FORWARD=8080:webapp:80"
    echo "  TUNNEL_CLIENT_ID=airgap-web"
    echo
    echo "  # Then start:"
    echo "  sudo systemctl enable tunnel-client"
    echo "  sudo systemctl start tunnel-client"
}

main "$@"
