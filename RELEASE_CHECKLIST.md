# Release Checklist for Pulse v4

This checklist guides the release process for Pulse v4 (Go version). When asked to create a release, follow these steps in order.

**IMPORTANT:** All "testing" in this checklist means Claude Code should manually verify things work correctly. We do NOT have automated test suites. Testing means Claude Code running commands to verify the build works, artifacts are valid, and releases are properly created.

## Pre-Flight Checks

### 1. Verify Prerequisites
- [ ] Ensure you're on main branch: `git branch --show-current`
- [ ] Check for uncommitted changes: `git status`
- [ ] Pull latest changes: `git pull origin main`
- [ ] Ensure gh CLI is authenticated: `gh auth status`
- [ ] Check disk space for builds: `df -h` (need ~500MB free)
- [ ] Verify Go is installed: `go version` (need 1.21+)

## Pre-Release Steps

### 2. Determine Version Number
- [ ] Check current version: `cat VERSION`
- [ ] Check latest releases: `gh release list --repo rcourtman/Pulse --limit 5`
- [ ] **CRITICAL**: Ensure version doesn't already exist
- [ ] Ask user what version they want to release
- [ ] Suggest next logical version based on:
  - For stable releases: increment minor (e.g., v4.0.0 → v4.1.0)
  - For patch releases: increment patch (e.g., v4.0.0 → v4.0.1)
  - For RC releases: increment RC number (e.g., v4.0.0-rc.1 → v4.0.0-rc.2)
  - For new RC series: start with -rc.1 (e.g., v4.1.0-rc.1)

### 3. Update Version
- [ ] Update version in `VERSION` file (without 'v' prefix): `echo "4.0.0" > VERSION`
- [ ] Commit the version change: `git commit -am "chore: bump version to v4.X.X"`
- [ ] Push changes: `git push origin main`

### 4. Test the Build (Claude Code Manual Testing)
- [ ] **Claude Code: Manually test the build works**
- [ ] Build test binary: `go build -o test-pulse ./cmd/pulse`
- [ ] Verify binary runs without errors: `./test-pulse config --help`
- [ ] Check binary size is reasonable: `ls -lh test-pulse` (should be ~10-15MB)
- [ ] Clean up: `rm test-pulse`
- [ ] **Note:** This means Claude Code should manually verify functionality, NOT run automated test suites

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
- [ ] Push tag: `git push origin v4.X.X`

### 7. Prepare Release Notes
Use this EXACT template for consistency across all releases:

```markdown
## What's Changed

### New Features
- Brief description of new feature (#issue_number if applicable)

### Bug Fixes
- Brief description of what was fixed (#issue_number)

### Improvements
- Brief description of improvement

### Breaking Changes (if any)
- Description of breaking change and migration path

## Installation

### Docker
```bash
docker pull rcourtman/pulse:vX.X.X
```

### Manual Install
Download the universal package that auto-detects your architecture:
- `pulse-vX.X.X.tar.gz`

Or choose architecture-specific:
- `pulse-vX.X.X-linux-amd64.tar.gz` - Intel/AMD 64-bit
- `pulse-vX.X.X-linux-arm64.tar.gz` - ARM 64-bit
- `pulse-vX.X.X-linux-armv7.tar.gz` - ARM 32-bit

## Notes
- Any important notes or warnings
- Migration instructions if needed
```

**IMPORTANT:**
- Keep sections even if empty (e.g., "Breaking Changes" with "None" if no breaking changes)
- Use issue numbers when applicable
- Be concise - one line per item
- NO emoji in headers (keep it professional)
- Only mention specific version Docker tag (vX.X.X) and avoid listing all tag variants

### 8. Build and Push Docker Images

**Option A: Using docker-builder container via pct exec (Preferred)**
```bash
# Clean clone and build - most reliable method
ssh root@delly.lan "pct exec 135 -- bash -c 'cd /root && rm -rf Pulse && git clone https://github.com/rcourtman/Pulse.git && cd Pulse && docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t rcourtman/pulse:vX.X.X --push .'"
```

**Option B: Using docker-builder container (192.168.0.174)**
```bash
ssh root@192.168.0.174
cd /root/Pulse
git pull
# Then run the docker buildx commands below
```

**Option C: Using local Docker with buildx**
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
- [ ] **IMPORTANT: Check if release already exists**: `gh release view v4.X.X --repo rcourtman/Pulse 2>/dev/null`
  - If exists, decide whether to:
    - Update release notes: `gh release edit v4.X.X --repo rcourtman/Pulse --notes "..."`
    - Delete and recreate: `gh release delete v4.X.X --repo rcourtman/Pulse --yes`
    - Skip if already complete
- [ ] **CRITICAL**: Upload checksums.txt FIRST to prevent update failures (related to #671)
  - Users who check for updates immediately after release need checksums.txt
  - If it's uploaded last, auto-updates will fail with "no checksum file found"
- [ ] For RC/beta releases:
  ```bash
  gh release create v4.X.X \
    --repo rcourtman/Pulse \
    --prerelease \
    --title "v4.X.X-rc.X" \
    --notes-file release-notes.md \
    release/checksums.txt release/*.tar.gz release/*.zip release/*.tgz release/*.sh release/*.sha256
  ```
- [ ] For stable releases:
  ```bash
  gh release create v4.X.X \
    --repo rcourtman/Pulse \
    --title "v4.X.X" \
    --notes-file release-notes.md \
    release/checksums.txt release/*.tar.gz release/*.zip release/*.tgz release/*.sh release/*.sha256
  ```
- [ ] Verify release was created with all ~30 artifacts (check asset count matches previous releases)
- [ ] Check download links work
- [ ] Share the release URL with the user

## Post-Release Steps

### 10. Test Installation Methods (Claude Code Manual Verification)
- [ ] **Claude Code: Verify release artifacts are downloadable**
  ```bash
  # Check that the release files can be downloaded
  curl -I https://github.com/rcourtman/Pulse/releases/download/v4.X.X/pulse-v4.X.X.tar.gz
  ```
- [ ] **Claude Code: Test extraction of tarball**
  ```bash
  # Download and verify tarball contents
  wget https://github.com/rcourtman/Pulse/releases/download/v4.X.X/pulse-v4.X.X.tar.gz
  tar -tzf pulse-v4.X.X.tar.gz | head -20
  ```
- [ ] **Claude Code: Verify Docker image exists**
  ```bash
  # Check Docker Hub for the image (via curl or browser)
  curl -s https://hub.docker.com/v2/repositories/rcourtman/pulse/tags/v4.X.X
  ```
- [ ] **Note:** Claude Code should manually verify these work, not necessarily run full installations

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

### Release Coordination
- **Always check for existing releases** before creating new ones
- **RC releases** should be marked as pre-release to prevent auto-updates
- **If a release exists**, either update it or increment version
- **Never assume** a version is available without checking

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
