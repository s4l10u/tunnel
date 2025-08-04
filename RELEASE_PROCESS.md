# Release Process

This document describes the improved release process for the Tunnel System.

## 🚀 Quick Release (Recommended)

For a complete release in one command:

```bash
make quick-release VERSION=v1.2.1
```

This will:
1. ✅ Build all platform binaries
2. ✅ Create release archives  
3. ✅ Generate install scripts
4. ✅ Create daemon.tar.gz
5. ✅ Generate checksums
6. ✅ Validate all assets
7. ✅ Create GitHub release
8. ✅ Upload all assets

## 📋 Step-by-Step Release

### 1. Build Release Assets

```bash
make release VERSION=v1.2.1
```

This creates:
- **Platform binaries**: `tunnel-v1.2.1-{os}-{arch}.{tar.gz|zip}`
- **Install scripts**: `install-client.sh`, `install-server.sh`  
- **Daemon package**: `daemon.tar.gz`
- **Documentation**: `README.md`, `RELEASE_NOTES.md`, `TLS-SETUP.md`
- **Checksums**: `checksums.txt`

### 2. Validate Release

```bash
make validate-release
```

Checks for:
- ✅ Required files (daemon.tar.gz, install scripts, etc.)
- ✅ All platform binaries (7 platforms)
- ✅ File integrity

### 3. Create GitHub Release

```bash
make github-release
```

This will:
- ✅ Create git tag if needed
- ✅ Push tag to GitHub
- ✅ Create/update GitHub release
- ✅ Upload all assets with --clobber (overwrites existing)

## 🛠️ Manual Release Process

### Prerequisites

- **GitHub CLI**: `brew install gh` or `apt install gh`
- **Git access**: Authenticated with push permissions
- **Go toolchain**: For building binaries

### Build Only

```bash
./scripts/build-release.sh v1.2.1
```

### Manual GitHub Release

```bash
# Create tag
git tag -a v1.2.1 -m "Release v1.2.1"
git push origin v1.2.1

# Create release
gh release create v1.2.1 artifacts/* \
  --title "🎆 Tunnel System v1.2.1" \
  --notes-file artifacts/RELEASE_NOTES.md
```

## 📁 Release Assets

Each release includes:

| File | Description |
|------|-------------|
| `install-client.sh` | One-line client installer |
| `install-server.sh` | One-line server installer |
| `daemon.tar.gz` | Complete daemon installation files |
| `tunnel-v1.X.X-{os}-{arch}.{tar.gz\|zip}` | Platform-specific binaries |
| `checksums.txt` | SHA256 checksums for verification |
| `README.md` | Installation and usage guide |
| `RELEASE_NOTES.md` | What's new in this release |
| `TLS-SETUP.md` | TLS configuration guide |

## 🔍 Validation & Testing

### Pre-Release Checks

```bash
# Validate all assets are present
make validate-release

# Test local build
make build

# Test Docker setup  
make test-full
```

### Post-Release Verification

```bash
# Test one-line installer
curl -fsSL https://github.com/user/repo/releases/latest/download/install-server.sh | sudo bash

# Verify all assets downloadable
curl -I https://github.com/user/repo/releases/latest/download/daemon.tar.gz
```

## 🔧 Troubleshooting

### Missing daemon.tar.gz Error

If users get 404 for daemon.tar.gz:

```bash
# Re-upload daemon package
gh release upload v1.2.1 artifacts/daemon.tar.gz --clobber
```

### Missing Platform Binary

```bash
# Rebuild specific platform
GOOS=linux GOARCH=amd64 go build -o tunnel-server-linux-amd64 ./server
GOOS=linux GOARCH=amd64 go build -o tunnel-client-linux-amd64 ./client

# Create archive
tar -czf tunnel-v1.2.1-linux-amd64.tar.gz tunnel-server-linux-amd64 tunnel-client-linux-amd64

# Upload to release
gh release upload v1.2.1 tunnel-v1.2.1-linux-amd64.tar.gz --clobber
```

### Release Already Exists

The `make github-release` command handles existing releases by uploading additional assets with `--clobber`.

## 📊 Release Metrics

Track release success:

```bash
# Check release downloads
gh release view v1.2.1 --json assets --jq '.assets[] | "\(.name): \(.download_count) downloads"'

# List all releases
gh release list
```

## 🔄 Version Management

### Version Format

Use semantic versioning: `v{MAJOR}.{MINOR}.{PATCH}`

- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)  
- **PATCH**: Bug fixes

### Release Notes

The release script auto-generates release notes, but you can customize `artifacts/RELEASE_NOTES.md` before running `make github-release`.

## 🎯 Best Practices

1. **Always validate** before releasing: `make validate-release`
2. **Test install scripts** on clean systems
3. **Update CHANGELOG.md** with user-facing changes
4. **Tag releases** consistently with semantic versioning
5. **Monitor download metrics** to ensure adoption

## 🚨 Emergency Release

For critical bug fixes:

```bash
# Quick patch release
make quick-release VERSION=v1.2.2

# Immediately test install
curl -fsSL https://github.com/user/repo/releases/latest/download/install-server.sh | sudo bash
```

---

**The improved release process ensures reliable, complete releases with all required assets every time!** 🎉