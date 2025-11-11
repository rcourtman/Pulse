# Pulse Release Procedure

**IMPORTANT: This document describes the automated release process. Follow these steps exactly when creating a new release.**

## Prerequisites

Before starting a release, ensure:

1. All changes for the release are merged to `main` branch
2. You have GitHub CLI (`gh`) installed and authenticated
3. The following GitHub secrets are configured:
   - `DOCKER_USERNAME` - Docker Hub username
   - `DOCKER_PASSWORD` - Docker Hub password
   - `ANTHROPIC_API_KEY` - API key for Claude Haiku 4.5 (changelog generation)

## Release Steps

### 1. Determine Version Number

Follow semantic versioning (MAJOR.MINOR.PATCH):
- **MAJOR**: Breaking changes
- **MINOR**: New features, backward compatible
- **PATCH**: Bug fixes only

Example: `4.29.0`

### 2. Trigger Automated Release Workflow

The entire release process is automated via GitHub Actions. To trigger it:

```bash
gh workflow run release.yml -f version=<VERSION>
```

**Example:**
```bash
gh workflow run release.yml -f version=4.29.0
```

This single command initiates the complete release pipeline.

### 3. What the Workflow Does (Automatically)

The workflow performs these steps in order:

#### Phase 1: Build Docker Images (~8-9 minutes)
- Builds multi-architecture Docker images (linux/amd64, linux/arm64)
- Adds OCI labels (version, git commit, build date, etc.)
- Pushes to both Docker Hub and GHCR:
  - `rcourtman/pulse:vX.Y.Z`
  - `rcourtman/pulse:latest`
  - `rcourtman/pulse-docker-agent:vX.Y.Z`
  - `rcourtman/pulse-docker-agent:latest`
  - `ghcr.io/rcourtman/pulse:vX.Y.Z`
  - `ghcr.io/rcourtman/pulse:latest`
  - `ghcr.io/rcourtman/pulse-docker-agent:vX.Y.Z`
  - `ghcr.io/rcourtman/pulse-docker-agent:latest`

**Critical:** Docker images are built FIRST. If this phase fails, no release is created.

#### Phase 2: Create Release (~3-4 minutes)
- Builds frontend (React/TypeScript SPA)
- Builds all Go binaries for multiple platforms:
  - Linux: amd64, arm64, armv7, armv6, 386
  - macOS: amd64, arm64
  - Windows: amd64, arm64, 386
- Creates tarballs, zip files, and Helm chart
- Generates SHA256 checksums for all artifacts
- Validates all artifacts (checksums, version strings, binary contents)
- **Generates release notes using Claude Haiku 4.5**:
  - Analyzes all commits since previous release
  - Produces formatted markdown following established template
  - Includes: New Features, Bug Fixes, Improvements, Breaking Changes, Installation instructions
- Creates **draft** GitHub release with LLM-generated notes
- Uploads ~56 assets:
  - 6 Linux tarballs (per architecture)
  - 1 Universal tarball (auto-detects architecture)
  - 5 Host agent packages (macOS, Windows variants)
  - 5 Sensor proxy binaries (Linux variants)
  - Helm chart
  - install.sh script
  - checksums.txt + individual .sha256 files

### 4. Monitor Workflow Progress

Watch the workflow at:
```
https://github.com/rcourtman/Pulse/actions/workflows/release.yml
```

Or via CLI:
```bash
gh run list --workflow=release.yml --limit 1
gh run watch <run-id>
```

**Expected duration:** 12-13 minutes total

**If the workflow fails:**
- Check logs: `gh run view <run-id> --log-failed`
- All artifacts are automatically cleaned up on failure
- No release will be created if validation fails
- Docker images may remain (tagged with version) but `:latest` won't be updated

### 5. Review Draft Release

Once the workflow completes:

1. **View the draft release:**
   ```bash
   gh release view v<VERSION>
   ```
   Or visit: `https://github.com/rcourtman/Pulse/releases`

2. **Review the LLM-generated release notes:**
   - Verify all features, bug fixes, and improvements are accurately described
   - Check that commit references are correct
   - Ensure breaking changes (if any) are prominently noted
   - Confirm installation instructions are correct

3. **Verify assets:**
   ```bash
   gh release view v<VERSION> --json assets -q '.assets[].name'
   ```
   Expected: ~56 assets including tarballs, binaries, checksums, Helm chart

4. **Test Docker images:**
   ```bash
   docker pull rcourtman/pulse:v<VERSION>
   docker run --rm rcourtman/pulse:v<VERSION> --version
   ```

### 6. Edit Release Notes (If Needed)

The LLM-generated notes are usually accurate, but you may want to:
- Add context about specific features
- Clarify breaking changes
- Add upgrade warnings or migration steps
- Adjust wording for clarity

Edit via GitHub UI or CLI:
```bash
gh release edit v<VERSION> --notes-file updated-notes.md
```

### 7. Publish the Release

**CRITICAL: This makes the release public to all users.**

When ready to publish:

```bash
gh release edit v<VERSION> --draft=false
```

Or use the GitHub UI "Publish release" button.

**What happens when published:**
- Release appears on public releases page
- Users see it in their update checks
- Docker `:latest` tags point to this version
- GitHub sends notifications to watchers
- Helm chart becomes available

### 8. Verify Published Release

After publishing:

1. **Check update endpoint:**
   ```bash
   curl https://api.github.com/repos/rcourtman/Pulse/releases/latest
   ```

2. **Verify Docker latest tag:**
   ```bash
   docker pull rcourtman/pulse:latest
   docker inspect rcourtman/pulse:latest | jq '.[0].Config.Labels'
   ```

3. **Test install script:**
   ```bash
   curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --version
   ```

## Troubleshooting

### Workflow Fails During Docker Build

**Symptom:** Job `build-docker-images` fails
**Impact:** No release is created (as intended)
**Fix:**
1. Check Dockerfile for syntax errors
2. Verify Docker Hub credentials in secrets
3. Check for platform-specific build issues
4. Re-run: `gh workflow run release.yml -f version=<VERSION>`

### Workflow Fails During Release Creation

**Symptom:** Job `create-release` fails after Docker images built
**Impact:** Docker images exist but no GitHub release
**Fix:**
1. Check `scripts/build-release.sh` for errors
2. Verify `scripts/validate-release.sh` passes locally
3. Check GitHub token permissions
4. Delete orphaned Docker tags if needed
5. Re-run workflow

### LLM-Generated Notes are Incorrect

**Symptom:** Release notes missing features or include wrong information
**Cause:** LLM misinterpreted commits or hallucinated
**Fix:**
1. Edit draft release notes manually via GitHub UI
2. Consider adjusting prompt in `scripts/generate-release-notes.sh`
3. Report issue to improve future generations

### Assets Missing from Release

**Symptom:** Some tarballs or binaries not uploaded
**Cause:** Build or upload step failed
**Fix:**
1. Check `scripts/build-release.sh` completed successfully
2. Verify all architectures built: `ls release/`
3. Check upload step logs for errors
4. May need to manually upload missing assets

### Duplicate Asset Upload Error

**Symptom:** Workflow fails with "asset under the same name already exists"
**Cause:** Asset already uploaded in previous step or previous failed run
**Fix:**
1. Delete the draft release: `gh release delete v<VERSION> --yes`
2. Delete the tag: `git push origin :refs/tags/v<VERSION>`
3. Re-run workflow

## Emergency Rollback

If a published release has critical issues:

### Option 1: Quick Patch Release

1. Fix the issue in `main`
2. Release v<VERSION+1> (e.g., 4.29.0 → 4.29.1)
3. Users will auto-update to the fix

### Option 2: Unpublish (Not Recommended)

```bash
# Mark as draft (hides from users)
gh release edit v<VERSION> --draft=true

# Or delete entirely (breaks existing downloads)
gh release delete v<VERSION> --yes
git push origin :refs/tags/v<VERSION>
```

**Warning:** Unpublishing breaks existing download URLs and confuses users. Prefer quick patch release.

## Workflow Architecture

The release workflow is designed with these principles:

1. **Docker-first:** Images build before release creation. If Docker fails, no release exists with broken images.
2. **Validation gates:** Multiple validation steps prevent bad releases.
3. **Draft by default:** Releases are created as drafts for manual review before publishing.
4. **Automated cleanup:** Failed builds clean up artifacts automatically.
5. **Deterministic builds:** Sorted checksums, reproducible binaries, no race conditions.
6. **LLM-powered docs:** Changelog generation reduces manual effort and ensures consistency.

## Files Involved in Release Process

- `.github/workflows/release.yml` - Main workflow orchestration
- `scripts/build-release.sh` - Builds all release artifacts
- `scripts/validate-release.sh` - Validates artifacts before release
- `scripts/generate-release-notes.sh` - LLM-powered changelog generation
- `VERSION` - Unused by workflow (version passed as input parameter)
- `Dockerfile` - Multi-stage build for server and agent images
- `Makefile` - Not used by workflow (for local development only)

## Maintenance

### Updating Changelog Template

Edit the prompt in `scripts/generate-release-notes.sh` to adjust LLM output format.

### Updating Supported Architectures

1. Add new platform/arch to `scripts/build-release.sh` (builds array)
2. Update `scripts/validate-release.sh` to validate new architecture
3. Update release notes template in `scripts/generate-release-notes.sh`

### Rotating API Keys

**Anthropic API Key:**
```bash
gh secret set ANTHROPIC_API_KEY --body "<new-key>"
```

**Docker Hub Password:**
```bash
gh secret set DOCKER_PASSWORD --body "<new-password>"
```

## Common Mistakes to Avoid

❌ **Don't manually create tags** - The workflow creates them automatically
❌ **Don't skip validation** - Trust the automated checks
❌ **Don't edit published releases** - Users may have already downloaded; prefer patch release
❌ **Don't bypass the workflow** - Manual releases are error-prone (see issues #671, #685, #683)
❌ **Don't use version numbers with 'v' prefix** - Input should be `4.29.0`, not `v4.29.0`

## Questions?

If something is unclear or the workflow needs updates, check:
- Workflow logs: `gh run view <run-id> --log`
- Recent commits: `git log --oneline -20`
- Open issues: `gh issue list --label release`
