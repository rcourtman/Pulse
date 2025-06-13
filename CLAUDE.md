# Claude Development Notes

## Git Workflow and Automated Releases

### Important: RC Workflow Commits to Develop

**Issue:** When you push to develop, the RC workflow automatically commits a version bump back to develop. This can cause push conflicts if you try to push again immediately.

**Solution:** Always use `git pull --rebase origin develop` before pushing:

```bash
# Correct workflow for pushing to develop:
git add .
git commit -m "your changes"
git pull --rebase origin develop  # This is crucial!
git push origin develop
```

### Release Workflow Overview

1. **Develop Branch:** 
   - Any push triggers RC release (v3.25.5-rc1, v3.25.5-rc2, etc.)
   - RC workflow auto-commits version bump to develop
   - RCs appear at top of releases page if package.json version > latest stable

2. **Main Branch:**
   - Merges from develop trigger stable releases
   - Creates new stable version with proper changelog
   - Updates Docker images with :latest tag

3. **Version Management:**
   - Keep package.json version ahead of current stable release
   - Example: If stable is v3.25.4, set package.json to "3.25.5"
   - This ensures RCs appear at top of releases page

### Update System Components

- **Installer script:** Auto-included in all release tarballs
- **UI updates:** Available through Settings > Software Updates
- **Docker:** Automatic builds for both stable and RC channels

### Testing Commands

```bash
# Check workflow status
gh run list --workflow=rc-release.yml --limit=3

# View latest releases
gh release list --limit=5

# Test installer (includes API rate limit fallback)
./install-pulse.sh --version v3.25.5-rc2
```

### Common Issues

1. **"tarballAsset is not defined"** - Fixed in v3.25.5-rc2
2. **Missing install script in releases** - Fixed in stable-release.yml
3. **RCs appearing below stable releases** - Fixed by updating package.json version
4. **Git push conflicts** - Always pull --rebase before pushing to develop