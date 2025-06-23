# Release Instructions for Claude

This document contains detailed instructions for creating releases. Only reference this when the user asks you to create a release.

## Release Process and Versioning

### Version Management
All version changes should be done locally before creating release PRs to avoid sync issues with automated workflows.

### Versioning Rules
Based on commit analysis using conventional commit format:
- **Major version (X.0.0)**: Breaking changes (commits with "BREAKING CHANGE" or "!:")
- **Minor version (x.X.0)**: New features (commits starting with "feat:" or "feature:")  
- **Patch version (x.x.X)**: Bug fixes (commits starting with "fix:" or "bugfix:")

### Prerequisites for Automated Releases
**CRITICAL**: The release workflows MUST exist on the main branch for GitHub to trigger them properly.

Required workflows on main branch:
- `.github/workflows/release.yml` - Runs when releases are published
- `.github/workflows/build-release.yml` - Runs when PRs from develop are merged to main
- `.github/workflows/stable-release.yml` - Manual trigger for stable releases

Before creating any release:
1. Check if all workflows exist on main branch
2. If not, create a PR from develop to main to sync the workflows
3. Merge the PR before creating the release

### Automated Release on PR Merge
When you merge a PR from develop to main, the `build-release.yml` workflow will automatically:
1. Extract version from package.json
2. Look for a changelog file (CHANGELOG_vX.X.X.md)
3. Build CSS assets
4. Create release tarball with production dependencies
5. Build and push multi-arch Docker images (linux/amd64, linux/arm64)
6. Create a **DRAFT** GitHub release with changelog and Docker info
7. **IMPORTANT**: The release is created as a DRAFT and requires manual publishing

### Release Process

#### 1. Analyze Changes First
```bash
# Ensure you're on develop branch and synced
git checkout develop
git pull origin develop

# Find last stable release
LAST_STABLE=$(git tag -l "v*" | grep -v "rc\|alpha\|beta" | sort -V | tail -1)
echo "Last stable: $LAST_STABLE"

# Count and review ALL changes
echo "Total commits since $LAST_STABLE:"
git log $LAST_STABLE..HEAD --oneline --no-merges | wc -l

# Review features and fixes
echo -e "\nFeatures added:"
git log $LAST_STABLE..HEAD --pretty=format:"%s" --no-merges | grep "^feat:" | wc -l

echo -e "\nBugs fixed:"
git log $LAST_STABLE..HEAD --pretty=format:"%s" --no-merges | grep "^fix:" | wc -l

# Use versionUtils for suggestion
node -e "
const { analyzeCommitsForVersionBump } = require('./server/versionUtils');
const analysis = analyzeCommitsForVersionBump();
console.log('\nVersion analysis:');
console.log('Current stable:', analysis.currentStableVersion);
console.log('Suggested version:', analysis.suggestedVersion);
console.log('Reasoning:', analysis.reasoning);
"
```

#### 2. Decide Version Based on Changes
After reviewing all changes, decide the version:
- **Major (X.0.0)**: Breaking changes, major UI overhauls
- **Minor (x.X.0)**: New features (email notifications, namespace support, etc.)
- **Patch (x.x.X)**: Only bug fixes, no new features

#### 3. Update Version Locally
```bash
# For RC release (on develop branch)
npm version 3.34.0-rc1 --no-git-tag-version

# For stable release (after PR is merged to main)
npm version 3.34.0 --no-git-tag-version

# Commit the version change
git add package.json package-lock.json
git commit -m "chore: bump version to X.X.X"
git push origin develop
```

#### 3. Merge to Main

**⚠️ CRITICAL: Ensure ALL develop commits are included!**

```bash
# First, verify what will be merged
git checkout main
git pull origin main
git log --oneline main..develop | wc -l  # Should show number of new commits

# If the number looks correct, proceed with merge
git merge develop

# If there are conflicts, resolve them by preferring develop version
# git checkout --theirs <conflicted-files>

# Commit and push
git push origin main
```

**Common Issues:**
- If a PR was used previously, it may not include commits made after PR creation
- Always verify the merge includes ALL commits from develop
- Check: `git log --oneline main..origin/develop` should show 0 commits after merge

#### 4. Create Release on GitHub

**⚠️ CRITICAL: ALWAYS CREATE RELEASES AS DRAFTS FIRST**
- **NEVER** publish a release directly
- **NEVER** use the publish flag without explicit user approval
- All releases MUST be created as drafts for review

**Using GitHub CLI (Recommended):**
```bash
# For RC releases - ALWAYS create as draft
gh release create v3.34.0-rc1 \
  --title "v3.34.0-rc1 - <Brief Description>" \
  --notes-file CHANGELOG_v3.34.0-rc1.md \
  --target develop \
  --prerelease \
  --draft

# DO NOT add --publish flag unless user explicitly requests it
# The draft URL will be provided for review
```

**Using GitHub Web UI:**
1. Go to https://github.com/rcourtman/Pulse/releases
2. Click "Draft a new release"
3. Click "Choose a tag" → Create new tag: `v3.34.0-rc1`
4. Target: `develop` branch (for RC) or `main` branch (for stable)
5. Release title: `v3.34.0-rc1 - <Brief Description>`
6. **✅ Check "Set as a pre-release"** for RC releases
7. Paste your changelog content
8. **⚠️ ALWAYS Click "Save draft" (NEVER "Publish release")**

**After reviewing the draft:**
- Check the changelog is accurate
- Verify the version tag is correct
- Ensure target branch is correct
- **Only publish when user explicitly approves**
- User must click "Publish release" to trigger automated builds

This will automatically (via release.yml workflow):
- Create the git tag
- Build CSS assets
- Create release tarball with production dependencies
- Upload tarball to the release
- Build and push Docker images:
  - `rcourtman/pulse:v3.34.0-rc1` (specific version)
  - `rcourtman/pulse:rc` (latest RC)
- Update release notes with Docker pull commands

#### 5. Sync Local Branches
```bash
# Update local tags and branches
git fetch --tags
git checkout develop
git pull origin develop
```

### Creating a Stable Release (After RC Testing)

Once RC testing is complete and you're ready for a stable release:

#### 1. Ensure Clean State
```bash
# Must be on main branch after RC merge
git checkout main
git pull origin main
git status  # Should be clean
```

#### 2. Update to Stable Version
```bash
# Remove RC suffix for stable release
npm version 3.34.0 --no-git-tag-version
git add package.json package-lock.json
git commit -m "chore: release v3.34.0"
git push origin main
```

#### 3. Trigger Stable Release Workflow

**⚠️ IMPORTANT: This workflow creates a DRAFT release**
- The workflow will NOT publish the release automatically
- You must manually publish the draft after review

1. Go to GitHub repository
2. Click "Actions" tab
3. Select "Manual Stable Release" workflow
4. Click "Run workflow"
5. Use main branch
6. Click green "Run workflow" button

The workflow will:
- Create GitHub release as a DRAFT
- Create release tarball
- Tag the release

**After workflow completes:**
- Go to the releases page
- Review the draft release
- Only publish when explicitly approved
- Docker images are built automatically when you publish the release on GitHub (not by this workflow)

#### 4. Sync Develop Branch
```bash
# After stable release, sync develop with main
git checkout develop
git pull origin develop
git merge main
git push origin develop
```

## Changelog Creation Instructions

### When Creating Release Changelogs

You should create changelogs that are concise, user-focused, and highlight what matters to users. Follow these steps:

#### 1. Determine Comparison Point
- **For RC releases**: Compare against the last STABLE release (not the previous RC)
- **For stable releases**: Compare against the previous stable release
- Find the comparison tag:
  ```bash
  # For last stable release
  git tag -l "v*" | grep -v "rc\|alpha\|beta" | sort -V | tail -1
  ```

#### 2. Analyze Commits THOROUGHLY
```bash
# CRITICAL: Get ALL commits and save to file for complete review
git log PREV_TAG..HEAD --oneline --no-merges > /tmp/all_commits.txt
wc -l /tmp/all_commits.txt  # See total - could be 70+ commits!
cat /tmp/all_commits.txt     # READ EVERY SINGLE COMMIT

# Search for MAJOR features that might be buried:
git log PREV_TAG..HEAD --oneline | grep -iE "alert|threshold|notification|email|webhook" 
git log PREV_TAG..HEAD --oneline | grep -iE "new:|feature:|dashboard|modal|view"
git log PREV_TAG..HEAD --oneline | grep -iE "namespace|pbs|backup|snapshot"
git log PREV_TAG..HEAD --oneline | grep -iE "system|overhaul|refactor|integrate"

# Check for new files indicating major features:
git diff --name-status PREV_TAG..HEAD | grep "^A.*\.js" | grep -v test

# Look for removed features/systems:
git log PREV_TAG..HEAD --oneline | grep -iE "remove|deprecate|replace"
```

**⚠️ CRITICAL: Don't just skim! Major features can be buried in 70+ commits. The alert system overhaul might be spread across 20+ commits with various titles.**

#### 3. Filter and Categorize
**EXCLUDE these commits (users don't care about):**
- Version bumps (`chore: bump version`, `chore: release`)
- Merge commits
- Documentation updates (README, CLAUDE.md, release docs)
- Internal tooling (lint, formatting, variable renames)
- Debug/logging additions or removals
- Workflow/CI changes
- Development process changes (release workflows, changelog generators)
- Code refactoring with no user-visible impact
- Test additions or modifications

**CATEGORIZE remaining commits:**
- 🔒 **Security**: Anything with "security", "vulnerability", "CVE"
- ✨ **Features**: Commits starting with `feat:`
- 🐛 **Fixes**: Commits starting with `fix:`
- 💫 **Improvements**: Performance, UI/UX enhancements that users will notice

#### 4. Write User-Friendly Descriptions
Transform technical commit messages into user language:

| Original | User-Friendly |
|----------|---------------|
| `fix: resolve PBS namespace filtering logic` | `Fixed backup filtering when using namespaces` |
| `feat: implement group by node in backups table` | `Added ability to group backups by node` |
| `perf: optimize chart rendering with debounce` | `Improved chart performance and responsiveness` |
| `fix: handle VMID collisions in PBS snapshots` | `Fixed incorrect backup assignments for VMs with duplicate IDs` |

#### 5. Changelog Template

```markdown
## Major New Features

### 🚨 [Major System/Feature Name]
- **NEW: [Key capability]** - What it does for users
- **NEW: [Related feature]** - How it helps
- Additional related improvements

### 📊 [Another Major Feature]
- Description focusing on user benefits
- Why this matters

### 🎯 [Third Major Feature]
- User-facing description

## Notable Fixes
- Only fixes users would have noticed
- Group related fixes together

## Update Instructions

- **Web Interface**: Settings → System → Check for Updates
- **Script**: `./install.sh --update`
- **Docker**: `docker pull rcourtman/pulse:[version]`
```

**Example of GOOD changelog identification:**
```
Alert System (spread across 20+ commits):
- "feat: add per-guest alert configuration"
- "feat: integrate alert creation into threshold"  
- "feat: enhance alert email system"
- "feat: add toast notifications for alerts"
- "fix: alert duration dropdown"
- "feat: remove Alert Monitor modal"
= MAJOR FEATURE: Complete alert system overhaul!
```

#### 6. Adaptive Guidelines

**Use your judgment to:**
- Combine related commits into single changelog entries
- Omit minor fixes that users wouldn't notice
- Highlight the most impactful changes first
- Add context when a fix addresses a commonly reported issue
- Group similar improvements together

**Example adaptations:**
- If 5 commits all relate to "backup filtering", combine them: "Comprehensive improvements to backup filtering and display"
- If a feature has a fix in the same release, just list the feature
- If commits are too technical, ask yourself: "What does this mean for the user?"
- Group related features under categories (e.g., "Backup Management", "Alert System", "Performance")
- Be comprehensive - RC releases often have 50-100+ commits since last stable
- Focus on what users will notice: new buttons, changed behavior, performance improvements

### Important Notes
- **NEVER** let workflows handle version changes - this causes sync issues
- **ALWAYS** sync immediately after workflow completes
- **RC releases** increment the RC number for each PR (3.34.0-rc1, 3.34.0-rc2, etc.)
- **Stable releases** remove the RC suffix from the version
- **Stable releases** are triggered manually from Actions tab, not via PR

### Fallback Script
If you need consistency or are unsure, use `scripts/generate-changelog-v2.js` as a reference, but feel free to improve upon its output based on context and user needs.