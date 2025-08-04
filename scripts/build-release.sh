#!/bin/bash

# Tunnel Release Build Script
# Builds binaries for multiple architectures and prepares release assets

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
VERSION=${1:-"v1.0.0"}
BUILD_DIR="artifacts"
LDFLAGS="-s -w -X main.version=$VERSION"

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

# Supported platforms
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "linux/386"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
    "windows/386"
)

create_build_dir() {
    log_info "Creating build directory..."
    rm -rf "$BUILD_DIR"
    mkdir -p "$BUILD_DIR"
    log_success "Created build directory: $BUILD_DIR"
}

build_binaries() {
    log_info "Building binaries for multiple platforms..."
    
    for platform in "${PLATFORMS[@]}"; do
        IFS='/' read -r -a platform_split <<< "$platform"
        GOOS="${platform_split[0]}"
        GOARCH="${platform_split[1]}"
        
        log_info "Building for $GOOS/$GOARCH..."
        
        # Determine file extension
        EXT=""
        if [ "$GOOS" = "windows" ]; then
            EXT=".exe"
        fi
        
        # Build server
        SERVER_OUTPUT="$BUILD_DIR/tunnel-server-$GOOS-$GOARCH$EXT"
        GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 go build \
            -ldflags="$LDFLAGS" \
            -trimpath \
            -o "$SERVER_OUTPUT" \
            server/main.go
        
        # Build client
        CLIENT_OUTPUT="$BUILD_DIR/tunnel-client-$GOOS-$GOARCH$EXT"
        GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 go build \
            -ldflags="$LDFLAGS" \
            -trimpath \
            -o "$CLIENT_OUTPUT" \
            client/main.go
        
        log_success "Built $GOOS/$GOARCH binaries"
    done
}

create_archives() {
    log_info "Creating release archives..."
    
    cd "$BUILD_DIR"
    
    for platform in "${PLATFORMS[@]}"; do
        IFS='/' read -r -a platform_split <<< "$platform"
        GOOS="${platform_split[0]}"
        GOARCH="${platform_split[1]}"
        
        # Determine file extension and archive format
        EXT=""
        ARCHIVE_EXT="tar.gz"
        if [ "$GOOS" = "windows" ]; then
            EXT=".exe"
            ARCHIVE_EXT="zip"
        fi
        
        SERVER_BINARY="tunnel-server-$GOOS-$GOARCH$EXT"
        CLIENT_BINARY="tunnel-client-$GOOS-$GOARCH$EXT"
        ARCHIVE_NAME="tunnel-$VERSION-$GOOS-$GOARCH"
        
        if [ "$GOOS" = "windows" ]; then
            # Create ZIP for Windows
            zip -q "$ARCHIVE_NAME.zip" "$SERVER_BINARY" "$CLIENT_BINARY"
            log_success "Created $ARCHIVE_NAME.zip"
        else
            # Create tar.gz for Unix-like systems
            tar -czf "$ARCHIVE_NAME.tar.gz" "$SERVER_BINARY" "$CLIENT_BINARY"
            log_success "Created $ARCHIVE_NAME.tar.gz"
        fi
        
        # Remove individual binaries after archiving
        rm -f "$SERVER_BINARY" "$CLIENT_BINARY"
    done
    
    cd ..
}

create_checksums() {
    log_info "Creating checksums..."
    
    cd "$BUILD_DIR"
    # Create checksums for all release assets (exclude directories and text files)
    sha256sum *.tar.gz *.zip daemon.tar.gz install-*.sh > checksums.txt 2>/dev/null || true
    log_success "Created checksums.txt"
    cd ..
}

copy_installation_files() {
    log_info "Copying installation files..."
    
    # Copy daemon installation scripts
    cp -r daemon "$BUILD_DIR/"
    
    # Copy documentation
    cp README.md "$BUILD_DIR/"
    cp docs/TLS-SETUP.md "$BUILD_DIR/" 2>/dev/null || true
    
    log_success "Copied installation files and documentation"
}

create_install_scripts() {
    log_info "Creating one-line install scripts..."
    
    # Client install script
    cat > "$BUILD_DIR/install-client.sh" << 'EOF'
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
    
    log_success "Tunnel client installed successfully!"
    echo
    log_info "Next steps:"
    echo "1. Edit configuration: sudo nano /etc/tunnel-client/config"
    echo "2. Enable service: sudo systemctl enable tunnel-client"
    echo "3. Start service: sudo systemctl start tunnel-client"
    echo "4. Check status: sudo systemctl status tunnel-client"
}

main "$@"
EOF

    # Server install script
    cat > "$BUILD_DIR/install-server.sh" << 'EOF'
#!/bin/bash
# Tunnel Server One-Line Installer
# Usage: curl -fsSL https://github.com/s4l10u/tunnel/releases/latest/download/install-server.sh | sudo bash

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
    log_info "Installing Tunnel Server..."
    
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
    cp "tunnel-server-$os-$arch" bin/tunnel-server-linux
    
    # Run installation
    log_info "Installing daemon..."
    ./daemon/server/install-server-daemon.sh
    
    # Cleanup
    cd /
    rm -rf "$INSTALL_DIR"
    
    log_success "Tunnel server installed successfully!"
    echo
    log_info "Next steps:"
    echo "1. Edit configuration: sudo nano /etc/tunnel-server/config"
    echo "2. Enable service: sudo systemctl enable tunnel-server"
    echo "3. Start service: sudo systemctl start tunnel-server"
    echo "4. Check status: sudo systemctl status tunnel-server"
    echo "5. Check firewall: sudo ufw allow 8443/tcp"
}

main "$@"
EOF

    chmod +x "$BUILD_DIR/install-client.sh" "$BUILD_DIR/install-server.sh"
    
    log_success "Created one-line install scripts"
}

create_daemon_archive() {
    log_info "Creating daemon installation archive..."
    
    cd "$BUILD_DIR"
    tar -czf daemon.tar.gz daemon/
    rm -rf daemon/
    cd ..
    
    log_success "Created daemon.tar.gz"
}

create_release_notes() {
    log_info "Creating release notes..."
    
    cat > "$BUILD_DIR/RELEASE_NOTES.md" << EOF
# Tunnel System Release $VERSION

Secure tunnel system for establishing connections between air-gapped environments and external networks.

## 🚀 Quick Installation

### One-Line Install (Recommended)

**Client (Air-gapped environment):**
\`\`\`bash
curl -fsSL https://github.com/s4l10u/tunnel/releases/latest/download/install-client.sh | sudo bash
\`\`\`

**Server (Internet-facing):**
\`\`\`bash
curl -fsSL https://github.com/s4l10u/tunnel/releases/latest/download/install-server.sh | sudo bash
\`\`\`

### Manual Installation

1. Download the appropriate binary for your platform
2. Extract the archive
3. Run the installation script:
   - Client: \`sudo ./daemon/client/install-daemon.sh\`
   - Server: \`sudo ./daemon/server/install-server-daemon.sh\`

## 📁 Release Assets

| File | Description |
|------|-------------|
| \`install-client.sh\` | One-line client installer |
| \`install-server.sh\` | One-line server installer |
| \`tunnel-$VERSION-\<os\>-\<arch\>.tar.gz\` | Binary archives for each platform |
| \`daemon.tar.gz\` | Daemon installation files |
| \`checksums.txt\` | SHA256 checksums for verification |

## 🔧 Supported Platforms

- **Linux**: amd64, arm64, 386
- **macOS**: amd64, arm64  
- **Windows**: amd64, 386

## 🔒 Security Features

- TLS/SSL encryption support
- Token-based authentication
- Non-root daemon execution
- Systemd security hardening
- Automatic certificate generation

## 📋 Configuration Examples

### Kubernetes API Server Tunneling
\`\`\`bash
TUNNEL_SERVER_URL=wss://your-server.com:8443/tunnel
TUNNEL_TOKEN=your-production-token
TUNNEL_FORWARD=6443:kubernetes.default.svc.cluster.local:443
TUNNEL_CLIENT_ID=airgap-k8s-api
\`\`\`

### Web Application Tunneling
\`\`\`bash
TUNNEL_SERVER_URL=wss://your-server.com:8443/tunnel
TUNNEL_TOKEN=your-production-token
TUNNEL_FORWARD=8080:webapp:80
TUNNEL_CLIENT_ID=airgap-web
\`\`\`

## 🔍 Verification

Verify download integrity:
\`\`\`bash
sha256sum -c checksums.txt
\`\`\`

## 📚 Documentation

- [Installation Guide](README.md)
- [TLS Setup](TLS-SETUP.md)

## 🆕 What's New in $VERSION

- Separated daemon installations for client and server
- One-line installation scripts
- Multi-architecture binary releases
- Enhanced security with systemd hardening
- Improved TLS certificate handling
- Fixed critical TLS verification bug

## 🐛 Bug Fixes

- Fixed TLS certificate verification logic in client
- Resolved race condition in TCP session cleanup
- Improved error handling in daemon installation
- Fixed certificate permissions for Docker compatibility
EOF

    log_success "Created release notes"
}

validate_release_assets() {
    log_info "Validating release assets..."
    
    cd "$BUILD_DIR"
    
    # Check required files exist
    local required_files=("daemon.tar.gz" "install-client.sh" "install-server.sh" "checksums.txt" "RELEASE_NOTES.md")
    
    for file in "${required_files[@]}"; do
        if [[ ! -f "$file" ]]; then
            log_error "Missing required file: $file"
            exit 1
        fi
    done
    
    # Check platform binaries exist
    local missing_platforms=()
    for platform in "${PLATFORMS[@]}"; do
        IFS='/' read -r -a platform_split <<< "$platform"
        GOOS="${platform_split[0]}"
        GOARCH="${platform_split[1]}"
        
        if [ "$GOOS" = "windows" ]; then
            if [[ ! -f "tunnel-$VERSION-$GOOS-$GOARCH.zip" ]]; then
                missing_platforms+=("$GOOS-$GOARCH")
            fi
        else
            if [[ ! -f "tunnel-$VERSION-$GOOS-$GOARCH.tar.gz" ]]; then
                missing_platforms+=("$GOOS-$GOARCH")
            fi
        fi
    done
    
    if [ ${#missing_platforms[@]} -gt 0 ]; then
        log_error "Missing platform binaries: ${missing_platforms[*]}"
        exit 1
    fi
    
    log_success "All required release assets validated"
    cd ..
}

show_summary() {
    log_success "Build completed successfully!"
    echo
    log_info "Release assets created in $BUILD_DIR/:"
    ls -la "$BUILD_DIR/"
    echo
    log_info "Asset count: $(ls -1 $BUILD_DIR/ | wc -l) files"
    echo
    log_info "To create a GitHub release:"
    echo "1. Commit and push changes: git add . && git commit -m 'Release $VERSION' && git push"
    echo "2. Create tag: git tag -a $VERSION -m 'Release $VERSION' && git push origin $VERSION"
    echo "3. Create release: gh release create $VERSION $BUILD_DIR/* --title 'Tunnel System $VERSION' --notes-file $BUILD_DIR/RELEASE_NOTES.md"
    echo
    log_info "Or use the Makefile:"
    echo "make github-release"
    echo
    log_info "One-line install commands (after release):"
    echo "Client: curl -fsSL https://github.com/s4l10u/tunnel/releases/latest/download/install-client.sh | sudo bash"
    echo "Server: curl -fsSL https://github.com/s4l10u/tunnel/releases/latest/download/install-server.sh | sudo bash"
}

main() {
    log_info "Starting release build for version: $VERSION"
    
    create_build_dir
    build_binaries
    create_archives
    copy_installation_files
    create_install_scripts
    create_daemon_archive
    create_checksums
    create_release_notes
    validate_release_assets
    show_summary
}

# Check if version argument provided
if [ -z "$1" ]; then
    log_warning "No version specified, using default: v1.0.0"
    log_info "Usage: $0 <version> (e.g., $0 v1.2.0)"
fi

main "$@"