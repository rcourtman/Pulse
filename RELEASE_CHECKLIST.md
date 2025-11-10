# Release Requirements for Pulse v4

This document defines what must be true for a successful Pulse release. You (AI) are responsible for figuring out HOW to accomplish these requirements. Focus on outcomes, not specific commands.

## Pre-Flight Requirements

### 1. Environment Ready
**Must be true before starting:**
- Working directory is on `main` branch with no uncommitted changes
- Local main is up-to-date with remote (`git pull` successful)
- GitHub CLI is authenticated and working
- At least 500MB free disk space available
- Go 1.21+ is installed

### 2. Version Determined
**Must be true:**
- New version number decided (ask user if unclear)
- Version doesn't conflict with existing releases
- Version follows semantic versioning:
  - **Minor bump** for new features (v4.26.x → v4.27.0)
  - **Patch bump** for bug fixes only (v4.27.0 → v4.27.1)
  - **RC format** for pre-releases (v4.27.0-rc.1)
- VERSION file updated (without 'v' prefix: `4.27.0`)
- Version change committed and pushed to main

### 3. Build Validated
**Must be true:**
- Test build succeeds: binary compiles without errors
- Binary is functional (can run `--help` or `--version`)
- Binary size is reasonable (~10-15MB for main pulse binary)

### 4. Release Artifacts Built
**Must be true:**
- Build script (`./scripts/build-release.sh`) completes successfully
- All artifacts created by build script exist in `release/` directory
- Artifact types expected (verify these exist, list is not exhaustive):
  - Main Pulse tarballs for all architectures + SHA256s
  - Host agent packages (macOS, Windows, Linux) + SHA256s
  - Sensor proxy binaries + SHA256s
  - Helm chart + SHA256
  - install.sh + SHA256
  - checksums.txt
- File sizes are reasonable (spot-check a few: tarballs typically 3-17MB, universal ~47MB)
- checksums.txt contains valid SHA256 hashes
- **Compare artifact count with recent successful releases** (v4.26.5, v4.27.0) to ensure nothing is missing

## Release Execution Requirements

### 5. Git Tagged
**Must be true:**
- Git tag created with 'v' prefix (e.g., `v4.27.0`)
- Tag pushed to remote repository
- Tag matches VERSION file content (minus 'v' prefix)

### 6. Release Notes Prepared
**Must be true:**
- Release notes follow standard template format (see below)
- Sections included in order: New Features, Bug Fixes, Improvements, Breaking Changes
- Each change is descriptive with context, not just "fix X" (explain impact)
- Issue numbers referenced where applicable (#123)
- No emoji in headers
- Installation section includes all 4 methods: Quick Install, Docker, Manual Binary, Helm
- Each installation method has complete, copy-pasteable commands
- Downloads section lists what's available
- Breaking Changes section present (say "None" if no breaking changes)
- Notes section has key highlights or migration guidance
- For RC releases: Include backup warning at the top of release notes

**Template structure:**
```markdown
## What's Changed

### New Features
- Brief but descriptive change with context (#issue)

### Bug Fixes
- What was broken and how it's fixed (#issue)

### Improvements
- Enhancement with user impact described (#issue)

### Breaking Changes
None

## Installation

**Quick Install (systemd / LXC / Proxmox VE):**
```bash
curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash
```

**Docker:**
```bash
docker pull rcourtman/pulse:vX.X.X
docker stop pulse && docker rm pulse
docker run -d --name pulse \
  --restart unless-stopped \
  -p 7655:7655 -p 7656:7656 \
  -v /opt/pulse/data:/data \
  rcourtman/pulse:vX.X.X
```

**Manual Binary (amd64 example):**
```bash
curl -LO https://github.com/rcourtman/Pulse/releases/download/vX.X.X/pulse-vX.X.X-linux-amd64.tar.gz
sudo systemctl stop pulse
sudo tar -xzf pulse-vX.X.X-linux-amd64.tar.gz -C /usr/local/bin pulse
sudo systemctl start pulse
```

**Helm:**
```bash
helm upgrade --install pulse oci://ghcr.io/rcourtman/pulse-chart \
  --version X.X.X \
  --namespace pulse \
  --create-namespace
```

## Downloads
- Universal tarball (auto-detects architecture): `pulse-vX.X.X.tar.gz`
- Architecture-specific: `amd64`, `arm64`, `armv7`
- Host agent packages: macOS, Windows, Linux
- Helm chart: `pulse-X.X.X.tgz`
- SHA256 checksums: `checksums.txt`

## Notes
- Key highlights, important warnings, or migration instructions
```

### 7. Docker Images Published
**Must be true:**
- Multi-architecture images built for: linux/amd64, linux/arm64, linux/arm/v7
- Images tagged correctly:
  - **For RC**: `vX.X.X` and `rc`
  - **For stable**: `vX.X.X`, `X.X.X`, `X.X`, `X`, `latest`
- Images pushed to Docker Hub (rcourtman/pulse)
- Images verified on Docker Hub (can pull successfully)

**Options for building (choose best available):**
- Option A: docker-builder container via `ssh root@delly "pct exec 135..."`
- Option B: docker-builder container at 192.168.0.174
- Option C: Local Docker with buildx (create multiarch builder if needed)

### 8. GitHub Release Created
**Must be true:**
- Release doesn't already exist (check first, handle conflicts)
- **CRITICAL**: `checksums.txt` uploaded FIRST (prevents auto-update race condition #671)
- All artifacts from `release/` directory uploaded to GitHub release
- Release marked as pre-release if RC version
- Release notes attached
- **Asset count matches recent successful releases** (compare with v4.26.5, v4.27.0)
  - If count differs, investigate: did build script change? Are files missing?
- Download links work (spot-check at least one tarball)

**Why checksums.txt must be first:**
Auto-updater downloads checksums.txt immediately. If it's uploaded last, users who check for updates right after release will get "no checksum file found" errors.

### 9. Installation Methods Verified
**Must be true:**
- Tarball is downloadable from GitHub releases
- Docker image is pullable from Docker Hub
- Verification doesn't require full installation, just confirm artifacts are accessible

## Post-Release Requirements

### 10. Documentation Updated (if needed)
**Must be true:**
- README.md reflects any new features or installation changes
- No stale information about features removed
- Version numbers updated if hard-coded anywhere

### 11. Monitoring Begins
**Must be true:**
- GitHub issues checked for immediate problems
- Ready to create hotfix release if critical issues reported
- User informed that release is complete (provide release URL)

### 12. Cleanup
**Must be true:**
- `release/` directory removed
- `build/` directory cleaned
- No leftover artifacts consuming disk space

## Critical Constraints

### Version Format Rules
- **VERSION file**: No 'v' prefix (e.g., `4.27.0`)
- **Git tags**: Always 'v' prefix (e.g., `v4.27.0`)
- **Filenames**: Include 'v' prefix (e.g., `pulse-v4.27.0-linux-amd64.tar.gz`)
- **Docker tags**: No 'v' for version tags (`4.27.0`), include 'v' for specific version (`v4.27.0`)

### Architecture Coverage
Every release must support:
- **amd64**: Intel/AMD 64-bit
- **arm64**: ARM 64-bit (Raspberry Pi 4/5, modern ARM)
- **armv7**: ARM 32-bit (older Raspberry Pi, some NAS)
- **Universal package**: Auto-detects architecture at install time

### Release Type Behavior
- **Stable releases** (v4.X.X): Wide distribution, auto-updates enabled
- **RC releases** (v4.X.X-rc.X): Mark as pre-release, prevents auto-updates
- **Patch releases** (v4.X.1): Bug fixes only, no new features

### Migration Warnings
If release contains breaking changes or affects v3 users:
- Warn about migration requirements
- Provide clear upgrade path
- Document what will break if auto-updated

## Validation Checklist

Before announcing release, verify:
- [ ] Git tag exists and matches VERSION
- [ ] GitHub release asset count matches recent successful releases
- [ ] checksums.txt was uploaded first (check asset upload timestamps if possible)
- [ ] Docker images exist on Docker Hub with correct tags
- [ ] Release notes follow template format
- [ ] At least one tarball and Docker image are downloadable
- [ ] Release marked as pre-release if RC
- [ ] User provided with release URL

## Important Context

### Why compare asset counts?
Complete releases include binaries and checksums for:
- Main Pulse server (multiple architectures)
- Host agent (macOS, Windows, Linux)
- Sensor proxy (Linux architectures)
- Helm chart
- Install script
- Master checksums file

Recent successful releases (v4.26.5, v4.27.0) have similar asset counts. Significant differences indicate:
- Build script changed (new artifacts added)
- Missing files (incomplete build)
- Upload error (some files didn't make it to GitHub)

Compare, investigate differences, don't just accept a hardcoded number.

### Why checksums.txt ordering matters?
The auto-updater's first action is downloading checksums.txt to verify artifacts. If this file doesn't exist yet, the entire update fails. GitHub uploads assets sequentially, so checksums.txt must be first in the upload queue.

### Why verify before announcing?
Once users are notified, they'll immediately start updating. Any missing artifacts or broken downloads create support burden and erode trust. Silent verification prevents public-facing failures.

## Troubleshooting Guidance

**If build fails:**
- Check Go version (need 1.21+)
- Verify frontend built (`ls frontend-modern/dist/`)
- Check disk space
- Clean and retry (`rm -rf build/ release/`)

**If Docker push fails:**
- Verify `docker login` succeeded
- Check buildx is active (`docker buildx ls`)
- Create multiarch builder if needed
- Try remote builder if local fails

**If GitHub release fails:**
- Check `gh auth status`
- Verify all artifacts exist in `release/`
- Check for existing release (delete or skip)
- Upload assets individually if batch fails

**If version conflicts:**
- Never reuse version numbers
- Increment and try again
- Delete bad tags if necessary

## Quick Reference

**Check if release exists:**
```bash
gh release view v4.X.X --repo rcourtman/Pulse
```

**Count release assets:**
```bash
gh release view v4.X.X --repo rcourtman/Pulse --json assets --jq '.assets | length'
```

**Verify Docker image:**
```bash
docker pull rcourtman/pulse:v4.X.X
```

**List recent releases:**
```bash
gh release list --repo rcourtman/Pulse --limit 5
```
