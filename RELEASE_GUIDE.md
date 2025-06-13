# Release Guide for Pulse

A comprehensive guide for handling commits, releases, and development workflow.

## Branch Strategy

**IMPORTANT**: Pulse uses a two-branch workflow:
- `main` - Stable releases only (protected)
- `develop` - Daily development work (default working branch)

Always check current branch: `git branch --show-current`

## Versioning System

**Dynamic RC Versioning:**
- **Develop branch**: Automatically calculates RC versions from git commits (e.g., "3.24.0-rc5")
- **Main branch**: Uses stable package.json version (e.g., "3.24.0")
- **Package.json**: Always contains base stable version, never RC versions
- **Local display**: Shows dynamic RC version when on develop branch
- **Automatic sync**: RC versions increment with each commit, no manual management needed

## Pre-flight Checklist

Before starting any release process, verify:
- [ ] Clean working directory: `git status`
- [ ] Docker logged in: `docker login` (check with `docker info | grep Username`)
- [ ] GitHub CLI authenticated: `gh auth status`
- [ ] Docker buildx available: `docker buildx ls || docker buildx create --name mybuilder --use`
- [ ] All tests passing: `npm test`

## Prerequisites Check

First run (only if user mentions "clean context" or you see errors):

```bash
# Check required tools
which git || echo "ERROR: git not installed"
which node || echo "ERROR: Node.js not installed" 
which npm || echo "ERROR: npm not installed"
which gh || echo "WARNING: GitHub CLI not installed (needed for releases)"
which docker || echo "WARNING: Docker not installed (needed for Docker releases)"

# If in a fresh clone, run:
npm install
```

## Quick Reference

When user says:
- **"commit this"** → Check branch first! Then go to [Commit Process](#commit-process)
- **"create a release"** → Use [Automated Stable Release Process](#automated-stable-release-process-new) via PR
- **"create an RC"** or **"release candidate"** → Automatic from `develop` push (see below)
- **"what changed?"** → `git status -s` and `git diff --stat`
- **"run tests"** → `npm test` (also check for lint/typecheck scripts)
- **"merge to main"** → Use [Automated Stable Release Process](#automated-stable-release-process-new)
- **"manual release"** → Only for hotfixes → [Manual Release Process](#manual-release-process-legacy)

## Commit Process

### 1. Check Branch & Summarize
```bash
# CRITICAL: Check current branch
git branch --show-current
# If not on develop: git checkout develop
git status -s
git diff --stat
```
Tell user: "You're on [branch]. You've modified X files. Main changes: [brief summary]"

### 2. Stage & Commit
```bash
git add .
git commit -m "<type>: <description>"
```

**Commit types**: `feat:` (new feature), `fix:` (bug fix), `docs:`, `chore:`, `refactor:`

### 3. Push
```bash
# For daily work (most common)
git push origin develop  # This triggers automatic RC release!

# For hotfixes on main (rare)
git push origin main
```

**If rejected**: 
```bash
git pull --rebase origin develop  # or main
git push origin develop  # or main
```

## Pre-Release Process (Automatic)

**NEW**: RC releases are now fully automated when you push to `develop` branch!

### How It Works
1. Make changes on `develop` branch
2. Commit and push: `git push origin develop`
3. GitHub Actions automatically:
   - Calculates new RC version (increments from last RC)
   - Updates package.json with new RC version
   - Commits version bump back to develop
   - Creates RC release with proper versioning
   - Builds multi-arch Docker images

### Dynamic Local Versioning
- **Local develop branch**: Shows dynamic RC versions calculated from git commits
- **Example**: If you have 5 commits since v3.24.0, version shows as "3.24.0-rc5"
- **Auto-increment**: Each new commit increments the RC number instantly
- **No manual version management**: Version automatically stays in sync

### Manual RC Release (if needed)
Only use if automatic process fails or for special cases:
```bash
# On develop branch
git tag v3.24.0-rc1
git push origin v3.24.0-rc1
gh release create v3.24.0-rc1 --title "v3.24.0-rc1" --prerelease --generate-release-notes
```

### Docker Images for RC
The GitHub Action handles Docker builds automatically, including:
- Multi-arch builds (amd64, arm64)
- Tagged with RC version
- Does NOT update `:latest` tag
- Rolling `:rc` tag always points to latest RC

## Automated Stable Release Process (NEW!)

**IMPORTANT**: Stable releases are now fully automated when you merge develop to main!

### How It Works
1. When develop branch is ready for stable release, create a pull request from develop to main
2. Once the PR is approved and merged, GitHub Actions automatically:
   - Analyzes all commits since the last stable release using semantic versioning
   - Determines appropriate version bump (major/minor/patch)
   - Updates package.json with the new stable version
   - Creates a git tag and GitHub release
   - Builds multi-arch Docker images with both version tag and `:latest`
   - Generates comprehensive changelog from commit analysis

### Semantic Commit Analysis
The automation analyzes commit messages to determine version bumps:
- **Major bump**: Commits with `BREAKING CHANGE` or `!:` in message
- **Minor bump**: Commits starting with `feat:` or `feature:`
- **Patch bump**: Commits starting with `fix:` or `bugfix:`
- **Patch bump**: All other commits (chore, docs, refactor, etc.)

### Creating a Stable Release
```bash
# 1. Ensure develop is ready for release
git checkout develop
git pull

# 2. Create pull request to main (preferred method)
gh pr create --base main --head develop --title "Release v3.25.0" --body "Ready for stable release

This PR includes:
- 8 new features
- 5 bug fixes
- Various improvements

The automated workflow will analyze commits and create the appropriate version bump."

# 3. Get PR approved and merge it
# 4. GitHub Actions automatically handles the rest!
```

### Alternative: Direct Merge (if needed)
```bash
# 1. Ensure develop is up to date
git checkout develop
git pull

# 2. Switch to main and merge
git checkout main
git pull
git merge develop

# 3. Push (this triggers automated stable release)
git push origin main

# GitHub Actions will detect the merge and create the stable release automatically
```

## Manual Release Process (Legacy)

**NOTE**: Manual releases are now rarely needed since the automated stable release process handles most cases. Use this only for hotfixes or special circumstances.

**IMPORTANT**: Manual releases must be done from `main` branch only!

### 1. Pre-release Verification
```bash
# Verify on main branch
git branch --show-current  # Must show "main"
# If not: echo "ERROR: Must be on main branch for releases!"

# Verify pre-flight checklist items
git status  # Should be clean
docker info | grep Username || echo "WARNING: Not logged into Docker Hub"
gh auth status || echo "ERROR: GitHub CLI not authenticated. Run: gh auth login"
docker buildx ls || echo "WARNING: Docker buildx not available"
```

### 2. Analyze Changes
```bash
# Get current version (should be stable base version like "3.24.0")
node -p "require('./package.json').version"

# Get dynamic version from API if running locally
curl -s http://localhost:7655/api/version | grep -o '"version":"[^"]*"' | cut -d'"' -f4

# Analyze commits since last stable tag
git log $(git describe --tags --abbrev=0)..HEAD --oneline
```

### 3. Determine Version
- Breaking changes → Major (1.0.0 → 2.0.0)
- New features → Minor (1.0.0 → 1.1.0)
- Bug fixes → Patch (1.0.0 → 1.0.1)
- Testing release → RC: X.Y.Z-rc.N (release candidate)

Ask user: "Based on changes, I suggest version X.Y.Z. OK?"

### 4. Update Version & Run Tests

**IMPORTANT**: For stable releases, package.json should contain the base stable version (e.g., "3.24.0"), not RC versions. The dynamic versioning system handles RC display automatically.

```bash
# Update package.json and package-lock.json to stable version
npm version X.Y.Z --no-git-tag-version

# Alternatively, if npm version fails:
node -e "const p=require('./package.json');p.version='X.Y.Z';require('fs').writeFileSync('./package.json',JSON.stringify(p,null,2)+'\n')"
npm install  # This updates package-lock.json

# Run tests
npm test

# Check for deprecation warnings
npm audit || echo "Check audit results - warnings don't block release"

# Build CSS
npm run build:css

# Run linting if available
npm run lint || echo "No lint script found"

# Run type checking if available  
npm run typecheck || echo "No typecheck script found"
```

**If tests fail**: Try to fix or ask user how to proceed

### 5. Commit & Tag
```bash
git add package.json package-lock.json
git commit -m "chore: release vX.Y.Z"
git push origin main

git tag -a vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z
```

### 6. Build Release Tarball & Create Changelog

**IMPORTANT CHANGELOG RULES**:
- Analyze ALL changes since last release: `git diff v[last]..v[current] --stat`
- Focus ONLY on user-visible changes that matter to users
- DO NOT include: dependency updates, docs changes, gitignore, dev tooling, Docker file restoration
- DO NOT repeat version number in changelog (GitHub already shows it)
- Use `echo` instead of heredoc to avoid EOF issues
- Look for big picture changes: major refactors, feature removals, new functionality

```bash
# First understand the scope of changes
git diff $(git describe --tags --abbrev=0)..HEAD --stat
echo "Total changes: $(git diff $(git describe --tags --abbrev=0)..HEAD --stat | tail -1)"

# Create user-focused changelog - use echo, NOT heredoc
echo "## Changes
- [List only user-visible changes]
- [Major feature additions/removals]
- [Breaking changes]

This release [brief summary of main theme]." > CHANGELOG_TEMP.md

# Build tarball
echo "X.Y.Z" | ./scripts/create-release.sh

# If create-release.sh fails, manual fallback:
if [ ! -f pulse-vX.Y.Z.tar.gz ]; then
  echo "Release script failed, creating tarball manually..."
  tar -czf pulse-vX.Y.Z.tar.gz \
    --exclude=node_modules \
    --exclude=.git \
    --exclude=.env \
    --exclude=data \
    --exclude=*.log \
    server src/public package.json package-lock.json README.md LICENSE CHANGELOG.md
fi
```

### 7. Build and Test Docker Images
```bash
# Ensure buildx is available
docker buildx ls || docker buildx create --name mybuilder --use

# IMPORTANT: First check what port the app uses (usually 7655, not 3000!)
grep "const PORT" server/index.js || grep "Server listening on port" server/index.js

# Build single-platform image for testing
docker build -t rcourtman/pulse:vX.Y.Z .

# Test with real .env file if available (RECOMMENDED)
if [ -f .env ]; then
  echo "Testing with real .env configuration..."
  docker run --rm -d --name pulse-test --env-file .env -p 7656:7655 rcourtman/pulse:vX.Y.Z
else
  echo "Testing with minimal config (less thorough)..."
  docker run --rm -d --name pulse-test \
    -e PROXMOX_HOST=test.example.com \
    -e PROXMOX_TOKEN_ID=test@pam!test \
    -e PROXMOX_TOKEN_SECRET=test-secret \
    -p 7656:7655 rcourtman/pulse:vX.Y.Z
fi

sleep 5  # Give it time to start

# Check if container is running
docker ps | grep pulse-test || (echo "Container failed to start!"; docker logs pulse-test; exit 1)

# Check container logs for successful startup
docker logs pulse-test 2>&1 | grep "Server listening on port" || (echo "Server didn't start!"; docker logs pulse-test; exit 1)

# If using real config, verify data collection
if [ -f .env ]; then
  docker logs pulse-test 2>&1 | grep -E "(nodes:|VMs:|CTs:)" && echo "✓ Data collection working"
fi

# Try to access the main page
curl -s -o /dev/null -w "%{http_code}" http://localhost:7656/ | grep -q "200" && echo "✓ Web interface accessible"

# Check for API health (if endpoint exists)
curl -s http://localhost:7656/api/health 2>/dev/null | grep -q "ok" && echo "✓ API health check passed" || echo "Note: API health endpoint may not exist in this version"

# Check logs for critical errors (ignore connection errors if using test config)
docker logs pulse-test 2>&1 | grep -i error | grep -v "ENOTFOUND\|ECONNREFUSED\|getaddrinfo" | head -5

# Stop test container
docker stop pulse-test
echo "✓ Docker test completed successfully"

# If tests pass, build and push multi-arch images
echo "Building and pushing multi-arch images..."
docker buildx build --platform linux/amd64,linux/arm64 -t rcourtman/pulse:vX.Y.Z -t rcourtman/pulse:latest --push .

# Verify images were pushed
docker manifest inspect rcourtman/pulse:vX.Y.Z || echo "WARNING: Failed to verify Docker push"
```

**Important Notes**:
- The app runs on port 7655, not 3000!
- Always test with real .env file when available for better validation
- Connection errors are expected with test config, but not with real config
- Main page (/) is more reliable to test than /api/health endpoint (which may not exist)

### 8. Create GitHub Release
```bash
# Verify GitHub CLI is authenticated  
gh auth status || (echo "ERROR: Must authenticate with GitHub CLI first: gh auth login"; exit 1)

# Create GitHub release with tarball
gh release create vX.Y.Z --title "Release vX.Y.Z" --notes-file CHANGELOG_TEMP.md pulse-vX.Y.Z.tar.gz

# Clean up temporary files
rm -f CHANGELOG_TEMP.md pulse-vX.Y.Z.tar.gz
```

### 9. Post-Release
```bash
# Verify the release was created
gh release view vX.Y.Z

# Check Docker Hub for new images
docker manifest inspect rcourtman/pulse:vX.Y.Z
docker manifest inspect rcourtman/pulse:latest

echo "✅ Release vX.Y.Z completed successfully!"
echo "🐳 Docker: docker pull rcourtman/pulse:vX.Y.Z"
echo "📦 GitHub: https://github.com/rcourtman/pulse/releases/tag/vX.Y.Z"
```

## Error Recovery

### If Docker push fails:
```bash
# Re-authenticate
docker login
# Retry the build and push
docker buildx build --platform linux/amd64,linux/arm64 -t rcourtman/pulse:vX.Y.Z -t rcourtman/pulse:latest --push .
```

### If GitHub release fails:
```bash
# Delete failed release if it was created
gh release delete vX.Y.Z --yes
# Try again
gh release create vX.Y.Z --title "Release vX.Y.Z" --notes-file CHANGELOG_TEMP.md pulse-vX.Y.Z.tar.gz
```

### If git push is rejected:
```bash
git pull --rebase origin main
# Resolve any conflicts, then:
git push origin main
git push origin vX.Y.Z
```