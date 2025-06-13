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
<<<<<<< HEAD
   - **Important:** PRs to main require admin merge due to branch protection

### PR to Main Branch Process

```bash
# Create PR from develop to main
gh pr create --base main --head develop --title "Your title" --body "Description"

# Merge with admin privileges (required due to branch protection)
# IMPORTANT: Use --merge (not --squash) for develop→main to preserve merge commit
gh pr merge PRNUMBER --merge --admin
```

**Why admin flag is needed:**
- Main branch requires 1 approval for merges
- You cannot approve your own PRs
- `--admin` flag bypasses the approval requirement
- This maintains security while allowing owner to merge critical fixes

**Why --merge (not --squash) for develop→main:**
- Stable release workflow detects merges by commit message patterns
- Squash merges lose the "Merge pull request #X from develop" message
- Merge commits preserve branch lineage and PR context
- This ensures automatic stable release detection works correctly

### PR Merge Strategy by Branch

```bash
# Feature branch → develop: Use squash (clean history)
gh pr merge PRNUMBER --squash

# Develop → main: Use merge (preserve merge commit for workflow detection)
gh pr merge PRNUMBER --merge --admin
```

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
5. **Stable release not triggered after PR merge** - Usually caused by using --squash instead of --merge

### 🚨 CRITICAL: Keep It Simple (Anti-Pattern Warnings)

**❌ DON'T do these common mistakes:**
- DON'T switch between branches unnecessarily - stay on `develop` unless specifically needed
- DON'T run complex git operations when simple ones work
- DON'T try to "fix" things that are already working correctly
- DON'T use `git pull` without `--rebase` on develop (causes merge commits)
- DON'T fight with git conflicts when workflows are running fine
- DON'T check out main branch just to view releases - use `gh release view` from any branch

**✅ DO follow this simple workflow:**
```bash
# Standard development cycle:
# 1. Work on develop branch (stay here!)
git checkout develop

# 2. Make changes and commit
git add .
git commit -m "fix: your change description"

# 3. Always rebase before pushing (RC workflow commits auto-happen)
git pull --rebase origin develop
git push origin develop

# 4. Check RC release status (from develop branch - don't switch!)
gh run list --workflow=rc-release.yml --limit=3
gh release list --limit=3

# 5. Create PR to main when ready for stable release
gh pr create --base main --head develop --title "stable: description" --body "Release description"

# 6. Merge PR (triggers stable release)
gh pr merge PRNUMBER --merge --admin

# 7. Check stable release status (still from develop - don't switch!)
gh run list --workflow=stable-release.yml --limit=3
gh release view TAG_NAME

# That's it! Stay on develop and continue working.
```

**Key principle:** Trust the automation. The workflows handle version bumps, releases, and git operations correctly. Your job is to make code changes and trigger releases with simple commands, not to manage complex git state.

### Manual Stable Release Trigger

If a stable release wasn't automatically triggered (e.g., due to squash merge), manually trigger one:

```bash
# Method 1: Create a new PR with merge commit (recommended)
PR_NUMBER=$(gh pr create --base main --head develop --title "trigger: stable release" --body "Manual trigger" | grep -o '[0-9]*$')
echo "Created PR #$PR_NUMBER"

# If merge conflicts occur, resolve them:
if ! gh pr merge $PR_NUMBER --merge --admin 2>/dev/null; then
  echo "Merge conflicts detected, resolving..."
  gh pr checkout $PR_NUMBER
  git fetch origin main && git merge origin/main
  # Resolve conflicts manually, then:
  git add . && git commit -m "resolve: merge conflicts for stable release trigger"
  git push origin develop
  gh pr merge $PR_NUMBER --merge --admin
fi

# Method 2: Direct push with merge commit message (if PR method fails)
git checkout main
git pull origin main
git commit --allow-empty -m "Merge pull request #XXX from rcourtman/develop"
git push origin main
```

**Note:** Method 1 often has merge conflicts due to RC workflow auto-commits. This is normal - just resolve conflicts in package.json (use main branch version) and CLAUDE.md (keep develop version).
