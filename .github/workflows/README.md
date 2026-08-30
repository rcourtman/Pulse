# GitHub Actions Workflows

## Trust contract

`scripts/check_workflow_trust.py` validates every workflow during the script
smoke suite. Remote actions and reusable workflows must use full commit SHAs,
container actions must use SHA-256 digests, and GitHub-hosted runners must use
dated image labels rather than moving `-latest` aliases.

Every `actions/checkout` step must also set `persist-credentials` explicitly.
Use `false` unless a later command in the same job performs an authenticated
Git write. The small number of write-path exceptions use `true` with the
machine-checked `# required: authenticated git writes` rationale.

Each workflow declares its default `GITHUB_TOKEN` permissions explicitly and
enumerates scopes instead of using `read-all` or `write-all`. Workflow inputs,
secrets, `github.token`, and attacker-controlled GitHub event metadata are
passed to `run` steps through `env`; they are data and must never be
interpolated into the generated shell program.

Workflows triggered by `pull_request` cannot reference confidential repository
secrets. Canonical governance therefore keeps its pull-request checks local to
the public checkout. `canonical-private-governance.yml` performs cross-repo
status, control-plane, subsystem-registry, mobile compatibility, and
repo-governance checks only after a push to `main`, so unmerged pull-request
code cannot replace the instructions that receive `WORKFLOW_PAT`.
`PULSE_LICENSE_PUBLIC_KEY` is the sole explicit PR exception because that
legacy secret value is intentionally non-confidential.

## Release Continuity

The `security-scan.yml` backstop checks the latest stable release lock and
activation identity every six hours. Its weekly run and an immediate read-back
after every stable release convergence perform the full verification from the
public surfaces customers use. They bind the immutable
GitHub release and activation marker to one source commit, authenticate every
checksummed release asset and SSH signature, re-verify release and build
attestations, and require the exact Docker Hub, GHCR, and OCI Helm identities
to remain equal to the digests committed at activation. Stable Docker Hub and
GHCR discovery aliases (`latest`, major, and major-minor) must also retain
those identities. Every run retains a
machine-readable evidence packet for 90 days, including partial outcomes when
a check fails. Six-hour lock-watch evidence explicitly records its narrower
`release_lock` mode and skipped full-surface checks. The job is read-only and
requires the public `PULSE_UPDATE_SIGNING_PUBLIC_KEY` repository variable.

## Issue Triage Automation

**Files**:

- `issue-version-label-sync.yml`
- `issue-version-retest-comment.yml`

Issue intake is split deliberately:

- `issue-version-label-sync.yml` is the silent metadata path. It runs on `opened`, `edited`, and `reopened` issue events so version labels, `needs-version-info`, `needs-retest-on-latest`, and the explicit `needs-decomposition` topic-integrity signal stay correct when maintainers tidy issue metadata.
- `issue-version-retest-comment.yml` is the scheduled public guidance path. It gives maintainers a grace window, then posts reporter-facing retest guidance only when an older-version bug report from a non-maintainer has no existing maintainer response. This prevents generic stable-version advice from contradicting a specific maintainer fix or prerelease boundary.
- Both workflows load the shared helper at `.github/scripts/issue-version-triage.cjs` so parsing and classification logic lives in one place instead of drifting across duplicated inline scripts.
- `needs-decomposition` is driven only by the structured **Additional actionable topics** form field. The [triage contract](../../docs/ISSUE_TRIAGE.md) requires human or agent judgment to create linked dispositions; the workflow does not infer or auto-create issues from free text.

## Update Demo Server

**File**: `update-demo-server.yml`

Automatically updates the governed demo target after a release is published.
Stable releases update the public demo. Prerelease tags no longer update a
separate v6 preview demo after GA.

### Configuration Required

Create one GitHub Environment:

1. `demo-stable`

The environment must define the secret names used by the governed demo target.

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

1. **TS_OAUTH_CLIENT_ID** and **TS_OAUTH_SECRET**
   - Tailscale OAuth client (business tailnet `tawny-powan.ts.net`, scope Auth Keys write, tag `tag:infra`) used by the governed demo deploy/update workflows before SSH
   - The action mints an ephemeral, pre-authorized, tagged node key per run, so runners join and garbage-collect themselves; unlike the retired static `TS_AUTHKEY`, the OAuth secret does not expire every 90 days
   - Allows GitHub-hosted runners to reach private demo targets such as the stable `pulse-relay` Tailscale host
   - May be stored as repository secrets or repeated in the selected environment if desired

Required environment variables:

1. **DEMO_EXPECTED_HOSTNAME**
   - The remote `hostname` value the stable demo environment is expected to report
   - Stable example: `pulse-relay`
   - This is a host-identity guard: the workflow fails closed if the SSH secret points at the wrong machine

2. **DEMO_LOCAL_BASE_URL**
   - Local URL used on the target host for version and mock-mode verification
   - Example stable value: `http://localhost:7655`

3. **DEMO_PUBLIC_HEALTH_URL**
   - Public health endpoint for the stable demo target
   - Example stable value: `https://demo.pulserelay.pro/api/health`

Optional environment variables:

1. **DEMO_SERVICE_NAME**
   - Stable default: `pulse`
   - When set, the server installer derives the instance-specific install dir,
     config dir, update helper, and update timer from this service identity.

2. **DEMO_AUTH_USER** / **DEMO_AUTH_PASS**
   - Demo credentials used for post-update mock verification
   - Defaults to `demo` / `demo` when omitted

### How It Works

1. **Trigger**: Runs from the lease-owning Release Convergence workflow after an exact activation marker is committed
2. **Target selection**: Stable tags deploy to `demo-stable`; prerelease tags are skipped because the public v6 preview target is retired after GA
3. **Service identity**: Stable runs default to the `pulse` service identity
4. **Governance check**: Validates the selected tag is reachable from the governed release branch for that version
5. **Latest check**: Refuses to update the public demo unless the published tag is the latest stable release
6. **Network attach**: Joins Tailscale before any SSH step so governed demo targets can stay on private hostnames or Tailscale IPs
7. **Update**: SSHs to the selected demo host and runs the tag-matched root installer from that exact git tag
8. **Host identity check**: Verifies the SSH target reports the governed expected hostname before running installer or deploy steps
9. **Verify**: Checks that the new version is running, mock mode is active, and the public demo HTML serves the same frontend entry asset as the target service
10. **Browser smoke**: Uses the governed Playwright helper to prove the public demo still renders the login shell in a real browser
11. **Cleanup**: Removes SSH key from runner

### Testing

Use `Release Dry Run` for the governed no-mutation demo-path preflight, or run
`Verify Demo Server` to verify the current committed stable target manually.

### Benefits

- ✅ The public demo follows the stable v6 release line after GA
- ✅ Prereleases no longer require a second public v6 preview surface
- ✅ Validates the real server installer path on the selected target
- ✅ Removes release-operator guesswork about which demo should move

## Verify Demo Server

**File**: `deploy-demo-server.yml`

The former branch-build deploy path is retired because it could replace the
stable public demo with unreleased `main`. Its manual dispatch is now a
non-mutating wrapper that verifies the current committed stable target. All
stable demo writes require an exact stable tag and activation marker and run
from `release-convergence.yml` under the global customer-promotion lease.

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
- Verifies the pushed OCI chart can be read from GHCR without registry credentials
- Requires no additional secrets—uses the built-in `GITHUB_TOKEN` with `packages: write` permission
