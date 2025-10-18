# GitHub Actions Workflows

## Update Demo Server

**File**: `update-demo-server.yml`

Automatically updates the public demo server (`pulse-relay`) when a new stable release is published.

### Configuration Required

Add these secrets to your GitHub repository settings (`Settings` → `Secrets and variables` → `Actions`):

1. **DEMO_SERVER_SSH_KEY**
   - The private SSH key for accessing the demo server
   - Generate with: `cat ~/.ssh/id_ed25519` (or your key file)
   - Should be the full private key including `-----BEGIN` and `-----END` lines

2. **DEMO_SERVER_HOST**
   - The hostname or IP of the demo server
   - Value: `174.138.72.137` (or hostname if using DNS)

3. **DEMO_SERVER_USER**
   - The SSH username for the demo server
   - Value: `root` (or the appropriate user with sudo access)

### How It Works

1. **Trigger**: Runs automatically when a GitHub release is published
2. **Filter**: Only runs for stable releases (skips RC/pre-releases)
3. **Update**: SSHs to demo server and runs the install script
4. **Verify**: Checks that the new version is running and mock mode is active
5. **Cleanup**: Removes SSH key from runner

### Testing

To test without publishing a release:
1. Go to `Actions` tab in GitHub
2. Select `Update Demo Server` workflow
3. Click `Run workflow` (if manual trigger is enabled)

Or test manually:
```bash
ssh pulse-relay "curl -fsSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | sudo bash"
```

### Benefits

- ✅ Demo server always showcases latest stable release
- ✅ Validates install script works on real server
- ✅ Removes manual step from release process
- ✅ Free to run (public repos get unlimited GitHub Actions minutes)

## Helm CI

**File**: `helm-ci.yml`

Runs `helm lint --strict` and renders the chart with common configuration combinations on every pull request that touches Helm content (and on pushes to `main`). This prevents regressions before they land.

- Triggered by PRs/pushes touching `deploy/helm/**`, docs, or the workflow itself
- Uses Helm v3.15.2
- Renders both the default deployment and an agent-enabled configuration to catch template issues

## Publish Helm Chart

**File**: `publish-helm-chart.yml`

Packages the Helm chart and pushes it to the GitHub Container Registry (OCI) whenever a GitHub Release is published. Also makes the packaged `.tgz` available as both an Actions artifact and a release asset. The same behaviour can be triggered locally via `./scripts/package-helm-chart.sh <version> [--push]`.

- Triggered automatically on `release: published`, or manually via workflow dispatch (requires `chart_version` input)
- Chart and app versions mirror the Pulse release tag (e.g., `v4.24.0` → `4.24.0`)
- Publishes to `oci://ghcr.io/<owner>/pulse-chart`
- Requires no additional secrets—uses the built-in `GITHUB_TOKEN` with `packages: write` permission

## Helm Integration (Kind)

**File**: `helm-integration.yml`

Creates a disposable Kind cluster, installs the chart, waits for the hub deployment to report ready, and performs a `/health` smoke check from inside the cluster.

- Triggered alongside the lint workflow for PRs/pushes touching Helm content
- Disables persistence to keep the Kind cluster lightweight
- Provides early detection of runtime issues (missing secrets, invalid probes, etc.)
