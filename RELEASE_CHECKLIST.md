# Release Checklist for Pulse v4

This checklist guides the release process for Pulse v4 (Go version). When asked to create a release, follow these steps in order.

## Pre-Flight Checks

### 1. Verify Prerequisites
- [ ] Ensure you're on main branch: `git branch --show-current`
- [ ] Check for uncommitted changes: `git status`
- [ ] Pull latest changes: `git pull origin main && git pull public main`
- [ ] Verify both remotes are in sync: `git log --oneline -1 origin/main public/main`
- [ ] Ensure gh CLI is authenticated: `gh auth status`
- [ ] Check disk space for builds: `df -h` (need ~500MB free)
- [ ] Verify Go is installed: `go version` (need 1.21+)

## Pre-Release Steps

### 2. Determine Version Number
- [ ] Check current version: `cat VERSION`
- [ ] Check latest releases: `gh release list --repo rcourtman/Pulse --limit 5`
- [ ] Ask user what version they want to release
- [ ] Suggest next logical version based on:
  - For stable releases: increment minor (e.g., v4.0.0 → v4.1.0)
  - For patch releases: increment patch (e.g., v4.0.0 → v4.0.1)
  - For RC releases: increment RC number (e.g., v4.0.0-rc.1 → v4.0.0-rc.2)
  - For new RC series: start with -rc.1 (e.g., v4.1.0-rc.1)

### 3. Update Version
- [ ] Update version in `VERSION` file (without 'v' prefix): `echo "4.0.0" > VERSION`
- [ ] Commit the version change: `git commit -am "chore: bump version to v4.X.X"`
- [ ] Push to both repos: `git push origin main && git push public main`

### 4. Run Tests
- [ ] Run Go tests: `go test ./...`
- [ ] Build test binary: `go build -o test-pulse ./cmd/pulse`
- [ ] Test binary starts: `./test-pulse --version`
- [ ] Clean up: `rm test-pulse`

### 5. Build Release Artifacts
- [ ] Run build script: `./build-release.sh`
- [ ] Verify all architecture files created:
  - `release/pulse-v4.X.X-linux-amd64.tar.gz`
  - `release/pulse-v4.X.X-linux-arm64.tar.gz`
  - `release/pulse-v4.X.X-linux-armv7.tar.gz`
  - `release/pulse-v4.X.X.tar.gz` (universal package)
- [ ] Check file sizes are reasonable (~3-4MB each, ~10MB for universal)
- [ ] Verify checksums file: `cat release/checksums.txt`

## Release Steps

### 6. Create Git Tag
- [ ] Create tag: `git tag v4.X.X`
- [ ] Push tag to both repos: `git push origin v4.X.X && git push public v4.X.X`

### 7. Prepare Release Notes
Create compelling release notes that explain:
- [ ] **WARNING for v3 users** about manual migration requirement (if applicable)
- [ ] New features with clear benefits
- [ ] Bug fixes with impact described
- [ ] Performance improvements with metrics
- [ ] Security enhancements (if any)
- [ ] Migration instructions (if needed)
- [ ] Download links for each architecture

Example structure:
```markdown
# ⚠️ CRITICAL: DO NOT AUTO-UPDATE FROM v3.x ⚠️

## 🚀 What's New

### ✨ New Features
- **Feature Name** - Clear description of benefit to users

### 🐛 Bug Fixes
- **Issue Fixed** - What was broken and now works

### 🔧 Improvements
- **Performance** - Specific metrics (e.g., 50% faster startup)

## 📦 Downloads

### Universal Package (Auto-detects architecture)
- [pulse-v4.X.X.tar.gz](link)

### Architecture-Specific
- [pulse-v4.X.X-linux-amd64.tar.gz](link) - Intel/AMD 64-bit
- [pulse-v4.X.X-linux-arm64.tar.gz](link) - ARM 64-bit
- [pulse-v4.X.X-linux-armv7.tar.gz](link) - ARM 32-bit
```

### 8. Build and Push Docker Images
- [ ] Ensure Docker is logged in: `docker login`
- [ ] Ensure buildx is available: `docker buildx ls`
- [ ] If buildx shows 'inactive', create/start it: `docker buildx create --name multiarch --use`
- [ ] Verify all Dockerfile paths are correct (check cmd/pulse vs cmd/pulse/)
- [ ] Build multi-arch images:
  ```bash
  # For RC releases
  docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 \
    -t rcourtman/pulse:v4.X.X \
    -t rcourtman/pulse:rc \
    --push .
  ```
  ```bash
  # For stable releases  
  docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 \
    -t rcourtman/pulse:v4.X.X \
    -t rcourtman/pulse:4.X.X \
    -t rcourtman/pulse:4.X \
    -t rcourtman/pulse:4 \
    -t rcourtman/pulse:latest \
    --push .
  ```
- [ ] Verify images on Docker Hub: https://hub.docker.com/r/rcourtman/pulse/tags
- [ ] Test pull works: `docker pull rcourtman/pulse:v4.X.X`

### 9. Create GitHub Release
- [ ] For RC/beta releases:
  ```bash
  gh release create v4.X.X \
    --repo rcourtman/Pulse \
    --prerelease \
    --title "v4.X.X-rc.X" \
    --notes-file release-notes.md \
    release/*.tar.gz
  ```
- [ ] For stable releases:
  ```bash
  gh release create v4.X.X \
    --repo rcourtman/Pulse \
    --title "v4.X.X" \
    --notes-file release-notes.md \
    release/*.tar.gz
  ```
- [ ] Verify release was created with all 4 artifacts
- [ ] Check download links work
- [ ] Share the release URL with the user

## Post-Release Steps

### 10. Test Installation Methods
- [ ] Test Proxmox helper script (when PR is merged):
  ```bash
  bash -c "$(wget -qLO - https://raw.githubusercontent.com/community-scripts/ProxmoxVE/main/ct/pulse.sh)"
  ```
- [ ] Test manual installation:
  ```bash
  wget https://github.com/rcourtman/Pulse/releases/download/v4.X.X/pulse-v4.X.X.tar.gz
  tar -xzf pulse-v4.X.X.tar.gz
  ./install.sh
  ```
- [ ] Test Docker installation:
  ```bash
  docker run -d --name pulse-test -p 7655:7655 rcourtman/pulse:v4.X.X
  curl http://localhost:7655/api/version
  docker stop pulse-test && docker rm pulse-test
  ```

### 11. Update Documentation
- [ ] Update README.md if needed (installation instructions, features)
- [ ] Ensure CLAUDE.md reflects any new development workflows
- [ ] Check if any example configurations need updating

### 12. Monitor Release
- [ ] Check GitHub issues for any immediate problems
- [ ] Monitor for crash reports or installation issues
- [ ] Be ready to create a patch release if critical issues found

### 13. Clean Up
- [ ] Remove release artifacts: `rm -rf release/`
- [ ] Clean build directory: `rm -rf build/`

## Important Notes

### Version Management
- **VERSION file** - Contains version without 'v' prefix (e.g., `4.0.0`)
- **Git tags** - Always include 'v' prefix (e.g., `v4.0.0`)
- **Release artifacts** - Include 'v' in filename (e.g., `pulse-v4.0.0.tar.gz`)

### Architecture Support
- **amd64** - Standard Intel/AMD 64-bit
- **arm64** - 64-bit ARM (Raspberry Pi 4/5, modern ARM servers)
- **armv7** - 32-bit ARM (older Raspberry Pi, some NAS devices)
- **Universal package** - Contains all architectures with auto-detection

### Release Types
- **Stable** (v4.1.0) - Production ready, wide distribution
- **Release Candidate** (v4.1.0-rc.1) - Testing phase, early adopters
- **Patch** (v4.0.1) - Bug fixes only, no new features

### Migration Warnings
- Always warn v3 users about manual migration requirement
- v3 to v4 auto-update will break installations
- Provide clear migration instructions in release notes

## Quick Commands Reference

```bash
# Check current version
cat VERSION

# List recent releases
gh release list --repo rcourtman/Pulse --limit 5

# Build all architectures
./build-release.sh

# Test specific architecture
cd build && ./pulse-linux-amd64 --version

# Create RC release
gh release create v4.X.X-rc.X --repo rcourtman/Pulse --prerelease --title "v4.X.X-rc.X" --notes "Release notes here" release/*.tar.gz

# Create stable release
gh release create v4.X.X --repo rcourtman/Pulse --title "v4.X.X" --notes "Release notes here" release/*.tar.gz

# Check release artifacts
ls -lh release/

# Build Docker images (RC)
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t rcourtman/pulse:v4.X.X -t rcourtman/pulse:rc --push .

# Build Docker images (stable)
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t rcourtman/pulse:v4.X.X -t rcourtman/pulse:latest --push .
```

## Troubleshooting

### Build Failures
- Ensure Go 1.21+ is installed
- Check frontend was built: `ls frontend-modern/dist/`
- Verify enough disk space
- Try cleaning first: `rm -rf build/ release/`

### Release Upload Issues
- Check GitHub token is valid: `gh auth status`
- Ensure all artifacts exist: `ls release/*.tar.gz`
- Try uploading individually if batch fails

### Version Confusion
- VERSION file = no 'v' prefix
- Git tags = with 'v' prefix  
- Keep everything else consistent with tags