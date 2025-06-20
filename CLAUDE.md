# Claude Development Notes

## 🚨 CRITICAL: Git Commit Policy

**NEVER COMMIT ANYTHING WITHOUT EXPLICIT USER REQUEST**

- Only commit when the user explicitly asks you to commit
- Never commit proactively or automatically after completing work
- Always wait for explicit permission before running `git commit` or `git push`
- If you complete work, simply report what you did - don't commit it

## Git Workflow and Automated Releases

### 🤖 Ultra-Simple Release Philosophy

**Key Principle**: Linear flow with auto-merging. Your workflow:
1. Work on `develop` branch (push freely, no releases)
2. Create PR to `main` when ready to test with users → RC created + auto-merged
3. Manually trigger stable release when happy with main

**What happens automatically**:
- PR from develop→main creates RC and merges immediately
- No lingering PRs, no conflicts
- `main` always has your latest tested code
- Version numbers handled by workflows

**What requires manual action**:
- Creating PR (when you want an RC)
- Triggering stable release (when RC is tested)

### Release Workflow Overview

1. **Development Phase (develop branch):**
   - Push commits freely all day
   - NO automatic releases
   - Work normally without noise

2. **RC Testing Phase (PR to main):**
   - `gh pr create --base main --head develop --title "Release: feature X"`
   - RC automatically created (e.g., v3.29.0-rc1)
   - PR automatically merged to main
   - Test RC with users
   - Need changes? Push to develop, create new PR → new RC

3. **Stable Release Phase (manual):**
   - When happy with testing
   - Actions → "Manual Stable Release" → Run
   - Creates stable from current main
   - Updates Docker :latest tag

### 🌿 Branch Strategy - When to Stay vs. Switch

**Default: Always work on `develop`**
- Always pull before starting work: `git pull origin develop`
- Make all code changes on develop
- Push to develop (no automatic releases)
- Check release status from develop (don't switch just to view)

**Git Config (already set up):**
- `pull.rebase = true` - Automatically rebases on pull to avoid merge commits
- This prevents the "divergent branches" error when remote has new commits

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

### Release Process (Dead Simple!)

**Creating RC Release:**
```bash
# IMPORTANT: First check ALL commits that will be included in the PR
git log --oneline main..develop

# When ready to test with users:
gh pr create --base main --head develop --title "Release: Your feature"

# That's it! The workflow will:
# 1. Create RC release (e.g., v3.29.0-rc1)
# 2. Auto-merge PR to main
# 3. No manual merge needed!
```

**RC PR Creation Instructions for Claude Code:**
When creating RC PRs, Claude Code should:

1. **Analyze commits since last RC**: Run `git log --oneline <last-rc-tag>..develop`
2. **Create summarized PR description**: Look at all the commits and create a clean, user-friendly summary grouped by:
   - ✨ New Features
   - 🐛 Bug Fixes  
   - 🔧 Improvements
   - 📚 Documentation
3. **Consolidate related commits**: Instead of listing every individual commit, group related changes:
   - ❌ "Prevent X flash, Prevent Y flash, Prevent Z flash" 
   - ✅ "Prevent UI flashing issues across multiple components"
4. **Include attribution**: Reference commits and issues/users when available:
   - Format: "Feature description (abc1234, addressing #123 by @username)"
   - For grouped commits: "Fix description (abc1234, def5678, addressing #123)"
   - If no issue: "Fix description (abc1234)"
5. **Use this description in PR body**: The summarized description becomes the RC changelog

**Example RC PR Description:**
```
## ✨ New Features
- Add backup source visibility improvements (77f0ad2, addressing #156)
- Implement PBS namespace filtering (fd612c2, 8590a91)

## 🐛 Bug Fixes  
- Prevent UI flashing and double refresh issues across components (c665cfa, 353af56, bc53ade, fb8f1c1, 0a08e23)
- Fix storage type categorization in diagnostics (bc256d1, 308a00f, addressing #145)
- Resolve version parsing issues with git describe format (1ff7552, 6c3dd41)

## 🔧 Improvements
- Enhanced PBS UI with relative timestamps (aa40829, f855aad)
- Optimize backup tab performance (3bbcaa0)
```

**Creating Stable Release:**
```bash
# After RC testing is complete:

# Option 1: Via GitHub UI
# Actions → "Manual Stable Release" → Run workflow → Run

# Option 2: Via CLI
gh workflow run stable-release.yml --ref main
```

**IMPORTANT: All releases are created as drafts**
- Releases require manual publishing after review
- **RC Releases**: Created with `draft: true` and `prerelease: true`
- **Stable Releases**: Created with `--draft` flag
- **Publishing**: Manual step required in GitHub UI after review

**Stable Release Changelog Instructions for Claude Code:**
When creating stable releases, Claude Code should:

1. **Find all RC releases since last stable**: Use `gh release list` to find RC releases since last stable
2. **Analyze RC changelogs**: Read the description/changelog from each RC release
3. **Create consolidated stable changelog**: Summarize all RC changes into one cohesive changelog:
   - Combine similar features across RCs
   - Deduplicate bug fixes that were refined across RCs  
   - Group improvements by component/area
   - Create a comprehensive "What's Changed" summary
4. **Preserve attribution**: Maintain commit references and issue attribution from RC changelogs
5. **Focus on user impact**: Emphasize features and fixes users will notice

**Example Process:**
```bash
# Find RC releases since last stable
gh release list --limit 10 | grep "rc"

# View each RC release changelog  
gh release view v3.30.0-rc1
gh release view v3.30.0-rc2
# ... etc

# Create consolidated stable changelog combining all RC improvements
```

**That's the entire process!** No complex merge strategies, no lingering PRs, no conflicts.

### Version Management

**Automatic Version Handling:**
- RC workflow analyzes commits and determines next version
- Stable workflow increments from current stable version
- No manual version management needed
- Workflows handle all version bumps automatically

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

#### RC Workflow Issues
```bash
# If RC creation fails, check:
gh run list --workflow=rc-release.yml --limit=3

# RC should auto-merge, but if stuck:
# Just create a new PR - it will handle everything
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

#### Changelog Generation
- Stable releases use `scripts/generate-changelog.js` for proper markdown formatting
- The script generates clean markdown with real newlines and backticks
- If changelog looks broken (shows `\n` instead of line breaks), the script may be missing
- The standalone script avoids bash escaping issues that occur with embedded Node.js code

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
- Use current main branch state (already has your RC changes)
- Create the stable release with proper changelog
- Build and push Docker images with :latest tag

### 🚨 Behavior Preferences

**Keep it simple:** Trust automation, stay on `develop`, minimal changes only

**✅ ALWAYS do this simple workflow:**
```bash
# 1. Work on develop (stay here!)
git checkout develop

# 2. Make your changes and test them
npm run test          # or whatever test command exists

# 3. Commit and push (NO releases)
git add .
git commit -m "type: descriptive message addressing #123"
git push origin develop

# 4. When ready to test with users, check commits and create PR
git log --oneline main..develop  # CHECK ALL COMMITS FIRST!
gh pr create --base main --head develop --title "Release: feature X"
# RC created and PR auto-merged!

# 5. Test the RC with users
# If issues: fix in develop, create new PR for new RC

# 6. When happy, trigger stable release
gh workflow run stable-release.yml --ref main
# Or use GitHub UI: Actions → Manual Stable Release → Run
```

**Remember**: If something seems complex, it's probably wrong. The workflows handle complexity - you handle simplicity.

### 📝 Commit & Issue Preferences

**Commit Consolidation:**
- Group related changes into single commits (avoid commit spam)
- Theme-based commits: Group changes by the problem they solve
- Example: Instead of 5 commits for "prevent X flash", "prevent Y flash", etc.
  Use: `fix: prevent UI flashing and double refresh issues` with detailed body

**Commit Messages:**
- Always reference GitHub issues: `addressing #123`  
- Avoid `fixes/closes` - let users test first
- Use descriptive commit bodies for multi-component changes
- Run tests before committing when available

**GitHub References:**
- **Commit references in comments**: Use just the short hash `abc1234` (GitHub auto-links)
- **Issue references in commits**: Use `addressing #123` (creates reference without auto-closing)
- **Auto-closing keywords**: `fixes #123`, `closes #123`, `resolves #123` (avoid - let users test first)

**Examples:**
✅ Good: `fix: prevent UI flashing and double refresh issues addressing #123`
✅ Good: `feat: add backup source visibility improvements addressing #156`
✅ Good: In issue comments: "Fixed in commit abc1234"
❌ Bad: 5 separate commits for each component's flash fix
❌ Bad: Missing issue reference in commit message
❌ Bad: In comments: "Fixed in commit abc1234f" (extra characters break linking)

## Summary of Ultra-Simple Workflow

**Three simple steps:**
1. 🔨 **Develop**: Push to develop freely (no releases)
2. 🧪 **Test**: Create PR → RC + auto-merge
3. 🚀 **Release**: Manual trigger → stable release

**No more:**
- ❌ Multiple open PRs
- ❌ Merge conflicts
- ❌ Manual PR merging
- ❌ Complex version management
- ❌ Excessive RC releases

**Just:**
- ✅ Linear progression
- ✅ Auto-merging PRs
- ✅ Clean release flow

# Development Behavior Preferences

- **🚨 NEVER COMMIT WITHOUT EXPLICIT REQUEST** - Only commit when user explicitly asks
- Do exactly what's asked - no more, no less
- Edit existing files over creating new ones
- No proactive documentation creation
- Use TodoWrite for complex tasks - helps user track progress
- Be concise in responses unless detail requested
- When fixing bugs, test the fix but DO NOT commit unless asked

## Service Management

**IMPORTANT**: This development environment runs Pulse as a systemd service.

**Hot reloading enabled:**
- File changes automatically restart the service
- No need to manually restart service during development
- Just save files and changes take effect immediately

**Service commands (if needed):**
- Check status: `systemctl status pulse`
- View logs: `journalctl -u pulse -f`
- Manual restart: `systemctl restart pulse` (rarely needed)

**Never use:**
- `npm start` or `npm run start` (service handles this)
- Direct Node.js execution
- Manual server startup commands

The service automatically handles development mode with hot reloading via the configured npm scripts.

## Commit Consolidation for Claude Code

**IMPORTANT**: When working iteratively and making multiple related fixes, consolidate them before final commit:

1. **During development**: Make incremental commits as needed while debugging/fixing
2. **Before pushing**: Use `git reset --soft HEAD~N` to uncommit recent related changes
3. **Consolidate**: Make one meaningful commit with all related changes
4. **Push**: Single clean commit to develop

**Example workflow**:
```bash
# After making 5 commits fixing UI flashing in different components:
git reset --soft HEAD~5  # Uncommit last 5 commits (keeps changes staged)
git commit -m "fix: prevent UI flashing and double refresh issues across components"
git push origin develop
```

**When to consolidate**:
- Multiple commits addressing the same root cause
- Iterative debugging commits (fix A, fix B, fix C for same issue)
- UI improvements across multiple components
- Related bug fixes discovered during testing

**Result**: Clean git history with meaningful commit messages instead of commit spam

## Technical Notes

### Proxmox API Bulk Endpoint Limitation

The `/cluster/resources?type=vm` bulk endpoint has a limitation with I/O counter updates:

- **Issue**: Network and disk I/O counters update only every ~10 seconds (not real-time)
- **Impact**: Using bulk endpoint alone causes I/O rates to show 0 B/s for several polling cycles
- **Solution**: Hybrid approach implemented in `dataFetcher.js`:
  - Use bulk endpoint for CPU, memory, disk usage (efficient)
  - Fetch fresh I/O counters from individual `/nodes/{node}/{type}/{vmid}/status/current` endpoints
  - This adds one extra API call per VM but ensures accurate 2-second I/O rate updates

This trade-off prioritizes Pulse's real-time monitoring accuracy over minimal API calls.