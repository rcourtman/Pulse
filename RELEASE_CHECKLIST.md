# Release Checklist

This checklist guides the release process for Pulse. When asked to create a release, follow these steps in order.

## Pre-Flight Checks

### 1. Verify Prerequisites
- [ ] Ensure you're on main branch: `git branch --show-current`
- [ ] Check for uncommitted changes: `git status`
- [ ] Pull latest main: `git pull origin main`
- [ ] Check Docker is logged in: `docker login`
- [ ] Verify buildx is available: `docker buildx ls`
- [ ] Ensure gh CLI is authenticated: `gh auth status`
- [ ] Check disk space for builds: `df -h` (need ~2GB free)

## Pre-Release Steps

### 2. Determine Version Number
- [ ] Check current version in `package.json`
- [ ] Check latest releases with `gh release list --limit 5`
- [ ] Ask user what version they want to release
- [ ] Suggest next logical version based on:
  - For stable releases: increment from last stable (e.g., v3.30.0 → v3.31.0)
  - For RC releases: increment RC number (e.g., v3.31.0-rc1 → v3.31.0-rc2)
  - For new RC series: start with -rc1 (e.g., v3.31.0 → v3.32.0-rc1)

### 3. Update Version
- [ ] Update version in `package.json` to match release version
- [ ] Commit the version change: `git commit -am "chore: bump version to vX.X.X"`
- [ ] Push to main: `git push origin main`

### 4. Generate Changelog
- [ ] Check last stable release: `git tag -l | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -1`
- [ ] Count commits since last stable: `git rev-list --count <last-stable-tag>..HEAD`
- [ ] Analyze ALL commits since last stable release
- [ ] Filter for user-visible changes only (exclude dev-only changes)
- [ ] Create/update `CHANGELOG.md` following this style:
  ```markdown
  ## 🧪 Release Candidate X  (or ## 🎉 Version X.X.X for stable)
  
  Brief description of the release (e.g., "This is a release candidate for testing")
  
  ---
  
  ## What's Changed
  
  ### ✨ New Features
  - **Feature name** - Clear description of what it does
  
  ### 🚀 Improvements  
  - **Improvement name** - What was enhanced
  
  ### 🐛 Bug Fixes
  - **Fix description** - What issue was resolved (#issue if applicable)
  
  ---
  
  ## 📥 Installation & Updates
  
  See the [Updating Pulse](https://github.com/rcourtman/Pulse#-updating-pulse) section in README for detailed update instructions.
  
  ### 🐳 Docker Images
  - `docker pull rcourtman/pulse:vX.X.X`
  - `docker pull rcourtman/pulse:rc` (RC releases)
  - `docker pull rcourtman/pulse:latest` (stable releases)
  ```
- [ ] Use emojis for section headers
- [ ] Use **bold** for feature/fix names
- [ ] Keep descriptions concise and user-friendly

### 5. Create Release Tarball
- [ ] Ensure CHANGELOG.md exists (required by script)
- [ ] Run `./scripts/create-release.sh`
- [ ] Enter the version when prompted (should match package.json)
- [ ] Verify tarball was created: `pulse-vX.X.X.tar.gz`
- [ ] Check tarball size is reasonable (should be ~5-6MB)

## Release Steps

### 6. Create Git Tag
- [ ] Create tag: `git tag vX.X.X`
- [ ] Push tag: `git push origin vX.X.X`

### 7. Build and Push Docker Images
- [ ] Ensure Docker is logged in: `docker login`
- [ ] Ensure buildx is available: `docker buildx ls`
- [ ] Build multi-arch images:
  ```bash
  # For RC releases
  docker buildx build --platform linux/amd64,linux/arm64 \
    -t rcourtman/pulse:vX.X.X \
    -t rcourtman/pulse:rc \
    --push .
  ```
  ```bash
  # For stable releases  
  docker buildx build --platform linux/amd64,linux/arm64 \
    -t rcourtman/pulse:vX.X.X \
    -t rcourtman/pulse:latest \
    --push .
  ```
- [ ] Verify images on Docker Hub: https://hub.docker.com/r/rcourtman/pulse/tags
- [ ] Test pull works: `docker pull rcourtman/pulse:vX.X.X`

### 8. Create GitHub Release
- [ ] For RC/beta releases:
  ```bash
  gh release create vX.X.X \
    --draft \
    --prerelease \
    --title "vX.X.X" \
    --notes-file CHANGELOG.md \
    pulse-vX.X.X.tar.gz
  ```
- [ ] For stable releases:
  ```bash
  gh release create vX.X.X \
    --draft \
    --title "vX.X.X" \
    --notes-file CHANGELOG.md \
    pulse-vX.X.X.tar.gz
  ```
- [ ] Verify release was created as draft
- [ ] The CHANGELOG.md should already include Docker info and link to UPDATE.md
- [ ] Share the release URL with the user

## Post-Release Steps

### 9. Prepare for Next Development
- [ ] If this was a stable release, bump to next dev version:
  - Update `package.json` to next version with `-dev` suffix
  - Example: v3.31.0 → v3.32.0-dev
  - Commit: `git commit -am "chore: bump version to vX.X.X-dev"`
  - Push: `git push origin main`

### 10. Clean Up
- [ ] Remove the release tarball: `rm pulse-vX.X.X.tar.gz`
- [ ] Ensure no temporary files remain: `rm -rf pulse-release-staging/`

## Important Notes

- **Main branch workflow** - All development and releases happen directly on main branch
- **No branch protection** - Since we're working directly on main, commits are allowed
- **ALWAYS create releases as drafts** - never publish directly
- **For RC releases**: Always use `--prerelease` flag
- **Version consistency**: Ensure package.json, tag, and release all use same version
- **Changelog focus**: Only include user-visible changes, not development tasks
- **No automation**: This is a manual process - do not try to automate with GitHub Actions
- **Docker tags**: RC releases update `:rc` tag, stable releases update `:latest` tag

## Quick Commands Reference

```bash
# Check current version
node -p "require('./package.json').version"

# List recent releases
gh release list --limit 5

# Count commits since last stable
LAST_STABLE=$(git tag -l | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -1)
git rev-list --count $LAST_STABLE..HEAD

# Create release tarball
./scripts/create-release.sh

# Build Docker images (RC)
docker buildx build --platform linux/amd64,linux/arm64 -t rcourtman/pulse:vX.X.X -t rcourtman/pulse:rc --push .

# Build Docker images (stable)
docker buildx build --platform linux/amd64,linux/arm64 -t rcourtman/pulse:vX.X.X -t rcourtman/pulse:latest --push .

# Create draft release (RC)
gh release create vX.X.X --draft --prerelease --title "vX.X.X" --notes-file CHANGELOG.md pulse-vX.X.X.tar.gz

# Create draft release (stable)
gh release create vX.X.X --draft --title "vX.X.X" --notes-file CHANGELOG.md pulse-vX.X.X.tar.gz
```