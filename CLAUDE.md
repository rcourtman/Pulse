# Claude Development Notes

## Git Workflow and Automated Releases

### 🤖 New Simplified Release Philosophy

**Key Principle**: Controlled releases with clear separation between RC and stable. Your workflow:
1. Make code changes on the `develop` branch (push freely, no automatic releases)
2. Create PR to `main` when ready to test (triggers ONE RC release)
3. Manually trigger stable release when RC is tested and ready

**What happens automatically**:
- Creating PR from develop→main creates an RC release
- RC versions are auto-incremented (rc1, rc2, etc.)
- Version numbers are managed by workflows
- Docker images are built and tagged

**What requires manual action**:
- Creating PR to trigger RC
- Triggering stable release (via GitHub Actions UI)
- Merging PRs to main

### Release Workflow Overview

1. **Development Phase (develop branch):**
   - Push commits freely throughout the day
   - NO automatic releases on push
   - Work normally without release noise

2. **Release Candidate Phase (PR to main):**
   - Create PR from develop→main when ready to test
   - RC release created automatically (e.g., v3.29.0-rc1)
   - Test the RC thoroughly
   - If issues found: close PR, fix, create new PR → new RC

3. **Stable Release Phase (manual trigger):**
   - After RC testing passes and PR is merged
   - Go to Actions → "Manual Stable Release" → Run workflow
   - Validates conditions and creates stable release
   - Updates Docker :latest tag

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

**Creating PR (triggers RC release):**
```bash
# Create PR from develop to main
gh pr create --base main --head develop --title "Release: Your title" --body "Description"

# This automatically triggers RC release creation
# Check the Actions tab to see RC build progress
```

**Handling merge conflicts (if any):**
```bash
# If conflicts exist (usually in package.json):
gh pr checkout <PR_NUMBER>
git fetch origin main && git merge origin/main

# Resolve conflicts (typically keep main's version for package.json)
git add .
git commit -m "resolve: merge conflicts"
git push origin develop

# Merge the PR
gh pr merge <PR_NUMBER> --merge --admin
```

**Creating Stable Release (after PR is merged):**
```bash
# Option 1: Via GitHub UI
# Go to Actions → "Manual Stable Release" → Run workflow → Run

# Option 2: Via CLI (coming soon)
# gh workflow run stable-release.yml
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

### Manual Stable Release Process

After your PR is merged to main and RC testing is complete:

1. **Via GitHub UI (recommended):**
   - Go to the repository's Actions tab
   - Find "Manual Stable Release" in the left sidebar
   - Click "Run workflow"
   - Select `main` branch
   - (Optional) Specify version or leave blank for auto-detection
   - Click "Run workflow"

2. **Via GitHub CLI:**
   ```bash
   # Run with auto-determined version
   gh workflow run stable-release.yml --ref main
   
   # Run with specific version
   gh workflow run stable-release.yml --ref main -f version=3.29.0
   ```

The workflow will:
- Validate that no open PRs exist from develop→main
- Ensure main includes all RC changes
- Create the stable release with proper changelog
- Build and push Docker images with :latest tag

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

# 3. Commit and push (NO automatic releases!)
git add .
git commit -m "description of change"
git push origin develop

# 4. When ready to release, create PR (triggers RC)
gh pr create --base main --head develop --title "Release: description"
# Check Actions tab for RC build

# 5. If merge conflicts exist, resolve them
gh pr checkout <PR_NUMBER>
git fetch origin main && git merge origin/main
# Resolve conflicts (usually keep main's version for package.json)
git add . && git commit -m "resolve: merge conflicts"
git push origin develop

# 6. Test the RC release thoroughly
# Download and test RC from releases page

# 7. Merge PR when RC passes testing
gh pr merge <PR_NUMBER> --merge --admin

# 8. Trigger stable release manually
# Go to Actions → "Manual Stable Release" → Run workflow
# Or: gh workflow run stable-release.yml --ref main
```

**Remember**: If something seems complex, it's probably wrong. The workflows handle complexity - you handle simplicity.

# important-instruction-reminders
Do what has been asked; nothing more, nothing less.
NEVER create files unless they're absolutely necessary for achieving your goal.
ALWAYS prefer editing an existing file to creating a new one.
NEVER proactively create documentation files (*.md) or README files. Only create documentation files if explicitly requested by the User.