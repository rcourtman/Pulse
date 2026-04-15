# GitHub Actions Workflows

## Issue Triage Automation

**Files**:
- `issue-version-label-sync.yml`
- `issue-version-retest-comment.yml`

Issue intake is split deliberately:

- `issue-version-label-sync.yml` is the silent metadata path. It runs on `opened`, `edited`, and `reopened` issue events so version labels, `needs-version-info`, and `needs-retest-on-latest` stay correct when maintainers tidy issue metadata.
- `issue-version-retest-comment.yml` is the public guidance path. It only runs on `opened` and `reopened`, and only posts reporter-facing retest guidance when the issue is an older-version bug report from a non-maintainer.
- Both workflows load the shared helper at `.github/scripts/issue-version-triage.cjs` so parsing and classification logic lives in one place instead of drifting across duplicated inline scripts.

## Update Demo Server

**File**: `update-demo-server.yml`

Automatically updates the governed demo target after a release is published.
Stable releases update the stable public demo. Prerelease tags update the
separate v6 preview demo.

### Configuration Required

Create two GitHub Environments:

1. `demo-stable`
2. `demo-preview-v6`

Each environment must define the same secret names so the workflow can select
the target by environment instead of hardcoding separate workflows.

Required environment secrets:

1. **DEMO_SERVER_SSH_KEY**
   - The private SSH key for accessing the demo server
   - Generate with: `cat ~/.ssh/id_ed25519` (or your key file)
   - Should be the full private key including `-----BEGIN` and `-----END` lines

2. **DEMO_SERVER_HOST**
   - The hostname or IP of the demo server

3. **DEMO_SERVER_USER**
   - The SSH username for the demo server (e.g. `root` or a deploy user with sudo access)

Required shared secret:

1. **TS_AUTHKEY**
   - Tailscale auth key used by the governed demo deploy/update workflows before SSH
   - Allows GitHub-hosted runners to reach private demo targets such as the stable `pulse-relay` Tailscale host
   - May be stored as a repository secret or repeated in the selected environment if desired

Required environment variables:

1. **DEMO_EXPECTED_HOSTNAME**
   - The remote `hostname` value the selected environment is expected to report
   - Stable example: `pulse-relay`
   - Preview example: `pulse-v6-preview`
   - This is a host-identity guard: the workflow fails closed if the SSH secret points at the wrong machine

2. **DEMO_LOCAL_BASE_URL**
   - Local URL used on the target host for version and mock-mode verification
   - Example stable value: `http://localhost:7655`
   - Example preview value: `http://localhost:8665`

3. **DEMO_PUBLIC_HEALTH_URL**
   - Public health endpoint for the selected demo target
   - Example stable value: `https://demo.pulserelay.pro/api/health`
   - Example preview value: `https://v6-demo.pulserelay.pro/api/health`

Optional environment variables:

1. **DEMO_SERVICE_NAME**
   - Stable default: `pulse`
   - Preview example: `pulse-v6-preview`
   - When set, the server installer derives the instance-specific install dir,
     config dir, update helper, and update timer from this service identity.

2. **DEMO_AUTH_USER** / **DEMO_AUTH_PASS**
   - Demo credentials used for post-update mock verification
   - Defaults to `demo` / `demo` when omitted

### How It Works

1. **Trigger**: Runs automatically when a GitHub release is published
2. **Target selection**: Stable tags deploy to `demo-stable`; prerelease tags deploy to `demo-preview-v6`
3. **Service identity guard**: Preview runs default to `pulse-v6-preview` and refuse to target the stable `pulse` service identity
4. **Governance check**: Validates the selected tag is reachable from the governed release branch for that version
5. **Latest check**: Refuses to update a target unless the published tag is the latest release for that target channel
6. **Network attach**: Joins Tailscale before any SSH step so governed demo targets can stay on private hostnames or Tailscale IPs
7. **Update**: SSHs to the selected demo host and runs the tag-matched root installer from that exact git tag
8. **Host identity check**: Verifies the SSH target reports the governed expected hostname before running installer or deploy steps
9. **Verify**: Checks that the new version is running, mock mode is active, and the public demo HTML serves the same frontend entry asset as the target service
10. **Browser smoke**: Uses the governed Playwright helper to prove the public demo still renders the login shell in a real browser
11. **Cleanup**: Removes SSH key from runner

### Testing

To test without publishing a release:
1. Go to `Actions` tab in GitHub
2. Select `Update Demo Server` workflow
3. Provide a tag and choose `stable`, `preview-v6`, or `auto`

### Benefits

- ✅ Stable and preview demos stay on separate governed targets
- ✅ Prereleases no longer require a stable demo overwrite or a manual skip
- ✅ Validates the real server installer path on the selected target
- ✅ Removes release-operator guesswork about which demo should move

### Preview Bootstrap Note

The preview environment must be bootstrapped once on the host before the update
workflow can keep it current. The supported path is a separate service identity
such as `pulse-v6-preview` plus a separate public route such as
`v6-demo.pulserelay.pro`; do not reuse the stable `pulse.service` instance.

## Deploy Demo Server

**File**: `deploy-demo-server.yml`

Manually deploys the current branch build to either the stable or preview demo
environment without changing the governed release workflow.

- Uses the same `demo-stable` / `demo-preview-v6` environment contract as the
  release-driven updater
- Joins Tailscale before SSH so governed demo targets can stay on private
  addresses instead of requiring public runner reachability
- Requires `DEMO_EXPECTED_HOSTNAME`, `DEMO_LOCAL_BASE_URL`, and `DEMO_PUBLIC_HEALTH_URL`
- Supports optional `DEMO_SERVICE_NAME`, `DEMO_INSTALL_DIR`, `DEMO_TEST_PORT`,
  `DEMO_AUTH_USER`, and `DEMO_AUTH_PASS`
- Assumes the target service and install directory already exist on the host
- Defaults preview runs to `pulse-v6-preview` and refuses to target the stable
  `pulse` service identity
- Verifies the SSH target reports the governed expected hostname before deploy
- Verifies that the public demo shell serves the same frontend entry asset that
  was built and deployed
- Uses `scripts/run_demo_public_browser_smoke.sh` to prove the public demo
  still renders the login shell in Chromium after deploy/update verification

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
