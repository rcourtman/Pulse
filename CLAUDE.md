# Claude Development Notes

## Git Workflow and Automated Releases

### 🤖 Automation-First Philosophy

**Key Principle**: Trust the automated workflows. Your primary job is to:
1. Make code changes on the `develop` branch
2. Push to trigger RC releases for testing  
3. Create PRs to `main` to trigger stable releases
4. Only intervene manually when automation fails

**When NOT to manually manage releases**:
- Don't manually bump versions (workflows handle this)
- Don't manually create tags (workflows handle this)
- Don't manually resolve "normal" merge conflicts (workflows have retry logic)
- Don't manually trigger workflows unless they fail completely

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
   - **Important:** PRs to main require admin merge due to branch protection

### 🌿 Branch Strategy - When to Stay vs. Switch

**Default: Always work on `develop`**
- Make all code changes on develop
- Push to develop to trigger RC releases
- Check release status from develop (don't switch just to view)

**Only switch branches when:**
1. **Resolving PR merge conflicts**: `gh pr checkout <PR_NUMBER>`
2. **Fixing critical issues on main**: `git checkout main` (rare)
3. **User explicitly requests working on a different branch**

**Never switch branches to:**
- View releases (`gh release list` works from any branch)
- Check workflow status (`gh run list` works from any branch)  
- "Clean up" git state (usually unnecessary)

**After switching branches, always return to develop:**
```bash
git checkout develop
```

### PR to Main Branch Process

**⚠️ IMPORTANT: Develop→Main PRs ALWAYS have merge conflicts due to RC version bumps**

```bash
# Create PR from develop to main
gh pr create --base main --head develop --title "Release: Your title" --body "Description"

# PRs will ALWAYS fail to merge due to package.json version conflicts
# This is expected! Follow this resolution process:

# 1. Checkout the PR and resolve conflicts
gh pr checkout <PR_NUMBER>
git fetch origin main && git merge origin/main

# 2. Resolve package.json conflict (ALWAYS keep main branch version)
# Edit package.json to remove conflict markers and keep main's version
# Example: If main=3.27.0 and develop=3.27.1-rc5, keep 3.27.0

# 3. Commit and push the resolution
git add package.json && git commit -m "resolve: merge conflicts - keep main branch version"
git push origin develop

# 4. Merge the PR
gh pr merge <PR_NUMBER> --merge --admin
```

**Why conflicts always happen:**
- RC workflow auto-commits version bumps to develop (3.27.0 → 3.27.1-rc1, etc.)
- Main branch stays at base version (3.27.0)
- package.json conflicts are guaranteed on every develop→main PR

**Why keep main's version:**
- Stable release workflow increments from main's current version
- If main=3.27.0, workflow creates 3.27.1 stable release
- If we kept develop's RC version, workflow would be confused

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

### 🔄 Update System Architecture

**Components Overview:**
- **Installer script**: Auto-included in all release tarballs (`scripts/install-pulse.sh`)
- **UI updates**: Available through Settings > Software Updates  
- **Docker**: Automatic builds for both stable (`:latest`) and RC (`:rc`) channels
- **Version checking**: Footer checks for updates using user's configured channel

**Key Update Features:**
- **Channel persistence**: User's channel preference (stable/RC) persists across updates
- **Auto-refresh**: UI automatically refreshes 8 seconds after update restart
- **Channel-specific checking**: Stable users only see stable updates, RC users only see RC updates

**Update Flow:**
1. User clicks "Apply Update" in Settings
2. System downloads and installs new version
3. Service restarts automatically  
4. Frontend auto-refreshes after 8-second delay
5. User remains on their selected channel (stable/RC)

**Related Files:**
- `src/public/js/ui/settings.js` - Update UI and channel persistence
- `src/public/js/main.js` - Footer version checking
- `server/routes/api.js` - Version API endpoints
- `scripts/install-pulse.sh` - Installation and update logic

### Testing Commands

```bash
# Check workflow status
gh run list --workflow=rc-release.yml --limit=3

# View latest releases
gh release list --limit=5

# Test installer (includes API rate limit fallback)
./install-pulse.sh --version v3.25.5-rc2
```

### 🔧 Common Issues and Recovery Patterns

#### Merge Conflicts in PRs
```bash
# When PR has merge conflicts:
gh pr checkout <PR_NUMBER>
git fetch origin main && git merge origin/main
# Resolve conflicts (typically keep main version for package.json)
git add . && git commit -m "resolve: merge conflicts"
git push origin develop
gh pr merge <PR_NUMBER> --merge --admin
```

#### Workflow Failures
```bash
# Check workflow status first:
gh run list --workflow=<workflow-name> --limit=3

# If workflow failed due to conflicts, it usually retries automatically
# Wait 5-10 minutes before manual intervention

# For persistent failures, check workflow logs:
gh run view <run-id> --log
```

#### Missing Files in Releases
- RC releases: Check if CSS build step exists in workflow
- Stable releases: Check if all necessary files are copied to staging directory
- Both: Verify workflow has proper Node.js setup before building

#### Version Number Issues
- RC and stable workflows use version analysis logic
- Don't manually fix version numbers - fix the workflow logic instead
- Check versionUtils.js for version determination logic

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

### 🚨 CRITICAL: Keep It Simple (Anti-Pattern Warnings)

**The Golden Rule**: Trust the automation, make minimal changes, stay on `develop`.

**❌ NEVER do these:**
- Switch branches unnecessarily (stay on `develop` unless resolving PR conflicts)
- Manually manage version numbers (workflows handle this automatically)
- Fight with git when workflows are running fine (they have retry logic)
- Run complex git operations when simple ones work
- Check out `main` just to view releases (use `gh release view` from any branch)
- Try to "fix" things that are already working correctly

**✅ ALWAYS do this simple workflow:**
```bash
# 1. Work on develop (stay here!)
git checkout develop

# 2. Make your changes and test them
npm run test          # or whatever test command exists

# 3. Commit and push (triggers RC)
git add .
git commit -m "description of change"
git pull --rebase origin develop
git push origin develop

# 4. For stable release, create PR (EXPECT merge conflicts!)
gh pr create --base main --head develop --title "Release: description"

# 5. Resolve the guaranteed package.json conflict
gh pr checkout <PR_NUMBER>
git fetch origin main && git merge origin/main
# Edit package.json: keep main's version, remove conflict markers
git add package.json && git commit -m "resolve: merge conflicts - keep main branch version"
git push origin develop

# 6. Merge the PR (triggers stable release)
gh pr merge <PR_NUMBER> --merge --admin

# 7. Check status without switching branches
gh run list --workflow=stable-release.yml --limit=3
gh release view <tag>
```

**Remember**: If something seems complex, it's probably wrong. The workflows handle complexity - you handle simplicity.

# important-instruction-reminders
Do what has been asked; nothing more, nothing less.
NEVER create files unless they're absolutely necessary for achieving your goal.
ALWAYS prefer editing an existing file to creating a new one.
NEVER proactively create documentation files (*.md) or README files. Only create documentation files if explicitly requested by the User.