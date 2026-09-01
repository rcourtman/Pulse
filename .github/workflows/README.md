# GitHub Actions Workflows

## Trust contract

`scripts/check_workflow_trust.py` validates every workflow during the script
smoke suite. Remote actions and reusable workflows must use full commit SHAs,
container actions must use SHA-256 digests, and GitHub-hosted runners must use
dated image labels rather than moving `-latest` aliases.

The audit also requires trust-bearing workflow structure to remain directly
visible in canonical block YAML. Escaped or explicit mapping keys,
anchors/aliases/tags, non-empty flow mappings, inline `jobs` or `steps`, and
noncanonical job declarations are rejected because YAML expansion can
otherwise hide permissions, runner identity, dependencies, or executable
steps from lexical policy checks.

Checkout pins additionally belong to a reviewed allowlist whose current floor
includes GitHub's fail-closed fork-PR protection for privileged events. The
audit prohibits `pull_request_target` entirely and rejects checkout's
`allow-unsafe-pr-checkout` opt-out; privileged work must remain isolated from
pull-request code rather than bypassing the upstream guard.

`workflow_run` is also a privileged trigger. Every handler must filter its
upstream workflow to the literal canonical branch `main`, and checkout steps
must not select code through triggering-run head metadata. These handlers also
cannot download upstream workflow artifacts or reacquire repository code with
command-line Git/GitHub clients or Actions artifact APIs. Upstream artifacts
remain untrusted data; move any future artifact consumer behind an independently
authenticated, non-`workflow_run` handoff before privileged use.

Every `actions/checkout` step must also set `persist-credentials` explicitly.
Use `false` unless a later command in the same job performs an authenticated
Git write. The small number of write-path exceptions use `true` with the
machine-checked `# required: authenticated git writes` rationale.

Every job that executes on a runner declares one literal `timeout-minutes`
budget. Reusable-workflow caller jobs cannot set a timeout and delegate that
responsibility to each runner job in the called workflow. The trust check
rejects omitted, dynamic, duplicate, zero, or platform-invalid budgets so a
stalled check or publisher cannot silently occupy a runner for GitHub's
six-hour default.

Each workflow declares its default `GITHUB_TOKEN` permissions explicitly, and
both workflow defaults and job-level overrides enumerate scopes instead of
using `read-all`, `write-all`, or dynamic grants. Workflow inputs, secrets,
`github.token`, step and job outputs, and attacker-controlled GitHub event
metadata are passed to `run` steps through `env`; they are data and must never
be interpolated into the generated shell program. Outputs remain data even
when an intermediate step parsed or validated them, because later substitution
would turn their value back into shell source.

Jobs that receive confidential repository secrets or a write-capable
`GITHUB_TOKEN` do not restore or save caches. This includes setup-action
dependency caches, direct Actions caches, and external BuildKit cache imports:
cache contents are unsigned mutable build input, while provenance only records
what the workflow produced. Read-only jobs may still cache locked dependencies;
the intentionally public legacy license key is not treated as a confidential
credential.

Passing data through `env` does not make it safe to append to the runner's
`GITHUB_OUTPUT`, `GITHUB_ENV`, `GITHUB_PATH`, or `GITHUB_STATE` command files.
The audit follows workflow data and values read from the event payload through
local Bash and PowerShell assignments before rejecting command-file writes, so
renaming a value is not mistaken for validation. Use
`scripts/write_github_output.py` for output data: it validates the output name
and chooses a random multiline delimiter that cannot collide with the value.
This prevents embedded newlines from creating additional outputs or
environment entries.

Workflows triggered by `pull_request` cannot reference confidential repository
secrets. Canonical governance therefore keeps its pull-request checks local to
the public checkout. `canonical-private-governance.yml` performs cross-repo
status, control-plane, subsystem-registry, subsystem-contract, mobile
compatibility, and repo-governance checks only after a push to `main`, so
unmerged pull-request code cannot replace the instructions that receive
`WORKFLOW_PAT`. The public job still audits contract structure and every public
path; only private path existence is deferred to that credential-isolated job.
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
those identities. Every run requests 90-day retention for a machine-readable
evidence packet, including partial outcomes when a check fails; GitHub applies
the repository's configured retention maximum and reports any clamp in the
workflow warning. Six-hour lock-watch evidence explicitly records its narrower
`release_lock` mode and skipped full-surface checks. The job is read-only and
requires the public `PULSE_UPDATE_SIGNING_PUBLIC_KEY` repository variable. A
failed release-trust check still permits activation-marker inspection when the
tag, numeric release ID, and exact source SHA are structurally valid. This
exposes independent marker damage in the same evidence packet; it never admits
the release or enables later delivery checks unless both trust checks pass.
Activation inspection also requires exactly one uploaded marker and compares
the downloaded byte count and SHA-256 value with GitHub's release-asset
metadata, so a valid-looking JSON response cannot silently replace or truncate
the packet that the release advertises.

`stable-install-continuity.yml` complements those byte-identity checks with a
weekly reinstall of the advertised stable release from its public assets. It
admits only the immutable stable identity accepted by the same continuity
validator, calls the release install-and-boot smoke under a read-only token,
and verifies the live service health and exact version. The privileged systemd
smoke environment is digest-pinned because a floating container image would
otherwise be an unreviewed code path inside the release gate.

Future release candidates also carry
`release-build-provenance.sigstore.json`, produced by the hosted
`build-release-candidate.yml` job after complete candidate validation. The
bundle is covered by the immutable candidate manifest and lets consumers
verify downloaded files offline against the candidate-builder identity.

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
2. **Target serialization**: Enters the bounded FIFO queue for the shared `stable-demo-runtime` concurrency lock also used by emergency recovery; GitHub environment admission alone does not serialize work on the host, and the default single-pending concurrency mode would discard superseded pending operations
3. **Target selection**: Stable tags deploy to `demo-stable`; prerelease tags are skipped because the public v6 preview target is retired after GA
4. **Service identity**: Stable runs default to the `pulse` service identity
5. **Governance check**: Validates the selected tag is reachable from the governed release branch for that version
6. **Latest check**: Refuses to update the public demo unless the published tag is the latest stable release
7. **Network attach**: Joins Tailscale before any SSH step so governed demo targets can stay on private hostnames or Tailscale IPs
8. **Update**: SSHs to the selected demo host and runs the tag-matched root installer from that exact git tag
9. **Host identity check**: Verifies the SSH target reports the governed expected hostname before running installer or deploy steps
10. **Verify**: Checks that the new version is running, mock mode is active, and the public demo HTML serves the same frontend entry asset as the target service
11. **Browser smoke**: Uses the governed Playwright helper to prove the public demo still renders the login shell in a real browser
12. **Cleanup**: Removes SSH key from runner

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
