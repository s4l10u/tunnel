# Release Checklist v1.2.0

## ✅ Pre-Release Validation

### Code Quality
- [x] All Go code compiles without errors
- [x] No unused imports or variables  
- [x] YAML dependency properly added to go.mod
- [x] Server starts successfully with YAML config
- [x] Backward compatibility with environment variables maintained

### Configuration System
- [x] YAML configuration loading works
- [x] Environment variable expansion works (`${TUNNEL_TOKEN}`)
- [x] Environment variable overrides work (`TUNNEL_FORWARDER_<NAME>_PORT`)
- [x] Port validation prevents conflicts
- [x] Graceful fallback from YAML to environment variables
- [x] Configuration priority order working correctly

### Deployment Scripts  
- [x] `install-server.sh` updated with YAML config support
- [x] `install-client.sh` enhanced with better guidance
- [x] `daemon/server/install-server-daemon.sh` creates both config formats
- [x] Systemd service files updated for new configuration
- [x] All installation scripts have enhanced UX with emojis and clear steps

### Documentation
- [x] `artifacts/README.md` updated with YAML configuration section
- [x] Migration guide created for legacy to YAML transition
- [x] Release notes comprehensive and accurate
- [x] Configuration examples provided for common use cases

## 📦 Release Preparation Steps

### 1. Version Bumping
```bash
# Update version in relevant files
# artifacts/RELEASE_NOTES.md: v1.1.0 → v1.2.0
# build scripts if they reference version
```

### 2. Build Binaries
```bash
# Build for all supported platforms
make build-linux-amd64
make build-linux-arm64  
make build-linux-386
make build-darwin-amd64
make build-darwin-arm64
make build-windows-amd64
make build-windows-386

# Or use existing build script
./scripts/build-release.sh
```

### 3. Create Archive Files
```bash
# Create platform-specific archives
tar -czf tunnel-v1.2.0-linux-amd64.tar.gz tunnel-server-linux-amd64 tunnel-client-linux-amd64
tar -czf tunnel-v1.2.0-darwin-amd64.tar.gz tunnel-server-darwin-amd64 tunnel-client-darwin-amd64
# ... for each platform

# Create daemon archive
tar -czf daemon.tar.gz daemon/

# Generate checksums
sha256sum *.tar.gz *.zip > checksums.txt
```

### 4. Prepare Release Assets
```bash
# Copy to artifacts directory
cp tunnel-v1.2.0-*.tar.gz artifacts/
cp tunnel-v1.2.0-*.zip artifacts/
cp daemon.tar.gz artifacts/
cp checksums.txt artifacts/
cp install-*.sh artifacts/
```

## 🚀 GitHub Release Steps

### 1. Create Git Tag
```bash
git add .
git commit -m "feat: add modern YAML configuration system

- Add unlimited custom service support
- Environment variable overrides  
- Advanced validation and error handling
- Enhanced deployment experience
- Backward compatible with legacy configs

🎆 Generated with Claude Code
Co-Authored-By: Claude <noreply@anthropic.com>"

git tag -a v1.2.0 -m "Release v1.2.0: Modern YAML Configuration System"
git push origin main
git push origin v1.2.0
```

### 2. Create GitHub Release
1. Go to GitHub repository → Releases → "Create a new release"
2. **Tag version**: `v1.2.0`
3. **Release title**: `v1.2.0: Modern YAML Configuration System 🎆`
4. **Description**: Copy content from `artifacts/RELEASE_NOTES_v1.2.0.md`
5. **Upload Assets**:
   - All `tunnel-v1.2.0-*.tar.gz` files
   - All `tunnel-v1.2.0-*.zip` files  
   - `daemon.tar.gz`
   - `checksums.txt`
   - `install-server.sh`
   - `install-client.sh`

### 3. Test Release
```bash
# Test one-line install from release
curl -fsSL https://github.com/s4l10u/tunnel/releases/latest/download/install-server.sh | sudo bash

# Verify YAML config is created
ls -la /etc/tunnel-server/config.yaml

# Test server startup
sudo systemctl start tunnel-server
sudo systemctl status tunnel-server
```

## 🔍 Post-Release Validation

### Installation Testing
- [ ] One-line server install works from GitHub releases
- [ ] One-line client install works from GitHub releases  
- [ ] YAML configuration file is automatically created
- [ ] Legacy configuration file is created for backward compatibility
- [ ] Systemd services start successfully
- [ ] Logs show YAML config being loaded

### Functionality Testing  
- [ ] Server loads YAML configuration correctly
- [ ] Environment variable overrides work
- [ ] Port validation prevents conflicts
- [ ] Unlimited services can be added via YAML
- [ ] Legacy environment variable configs still work
- [ ] WebSocket tunnel connections establish successfully

### Documentation Testing
- [ ] README examples work as documented
- [ ] Migration guide is accurate
- [ ] Configuration examples are valid
- [ ] One-line install commands in README work

## 📋 Communication

### Release Announcement
- [ ] Update project README with v1.2.0 features
- [ ] Social media announcement (if applicable)
- [ ] Internal team notification
- [ ] Update any dependent projects

### Key Messaging Points
1. **🎆 Major Feature**: Modern YAML configuration system
2. **✨ Unlimited Services**: Add any service without code changes  
3. **🔄 Backward Compatible**: Zero breaking changes
4. **🚀 Enhanced UX**: Professional deployment experience
5. **⚙️ Runtime Flexibility**: Environment variable overrides

---

## 🎯 Success Criteria

Release is successful when:
- ✅ All binaries build and work on target platforms
- ✅ One-line installers work from GitHub releases
- ✅ YAML configuration system works as documented
- ✅ Legacy configurations continue to work
- ✅ No breaking changes for existing users
- ✅ Enhanced user experience is evident
- ✅ Documentation is accurate and helpful

**Ready for Release!** 🚀