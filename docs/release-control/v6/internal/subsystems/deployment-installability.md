# Deployment Installability Contract

## Contract Metadata

```json
{
  "subsystem_id": "deployment-installability",
  "lane": "L1",
  "contract_file": "docs/release-control/v6/internal/subsystems/deployment-installability.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own server installation, deployment bootstrap behavior, provider-hosted MSP
deployment artifacts, update planning, and server-side update execution
surfaces.

Provider-hosted MSP deploy artifacts must package the provider control plane as
a least-privilege Docker provisioner. The packaged compose/setup path must avoid
whole-host and Docker-data read mounts, expose storage admission only through
narrow marker directories, broker Docker daemon access through the socket proxy,
pin trusted proxy CIDRs to the provider network, block tenant bridge access to
cloud metadata endpoints at the host firewall when possible, and pin the Traefik
TLS floor in the dynamic config.

## Canonical Files

1. `internal/updates/`
2. `internal/api/updates.go`
3. `frontend-modern/src/api/updates.ts`
4. `frontend-modern/src/components/UpdateBanner.tsx`
5. `frontend-modern/src/components/WhatsNewCard.tsx`
6. `frontend-modern/src/components/whatsNewModel.ts`
7. `frontend-modern/src/utils/localStorage.ts`
4. `cmd/pulse-control-plane/main.go`
5. `cmd/pulse-control-plane/mobile_proof_cmd.go`
6. `cmd/pulse-control-plane/provider_msp.go`
7. `cmd/pulse-control-plane/provider_msp_backup.go`
8. `cmd/pulse-control-plane/provider_msp_install_proof.go`
9. `cmd/pulse-control-plane/provider_msp_preflight.go`
10. `cmd/pulse-control-plane/provider_msp_proof.go`
11. `cmd/pulse-control-plane/provider_msp_recover.go`
12. `cmd/pulse-control-plane/provider_msp_status.go`
13. `internal/cloudcp/provider_msp_backup.go`
14. `internal/cloudcp/provider_msp_recovery.go`
15. `internal/cloudcp/docker/manager.go`
16. `internal/cloudcp/docker/labels.go`
17. `internal/cloudcp/tenant_runtime_rollout.go`
13. `.github/workflows/build-release-candidate.yml`
14. `.github/workflows/create-release.yml`
14. `.github/workflows/deploy-demo-server.yml`
15. `.github/workflows/helm-pages.yml`
16. `.github/workflows/promote-floating-tags.yml`
17. `.github/workflows/publish-docker.yml`
18. `.github/workflows/publish-helm-chart.yml`
19. `.github/workflows/release-dry-run.yml`
20. `.github/workflows/update-demo-server.yml`
21. `.github/workflows/validate-release-assets.yml`
22. `.github/workflows/install-sh-smoke.yml`
23. `.github/ISSUE_TEMPLATE/v6_rc_feedback.yml`
23. `docs/RELEASE_NOTES.md`
24. `docs/releases/`
25. `docs/UPGRADE_v6.md`
26. `docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md`
27. `docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md`
28. `docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md`
29. `package.json`
30. `package-lock.json`
31. `frontend-modern/package.json`
32. `frontend-modern/package-lock.json`
33. `frontend-modern/vite.config.ts`
34. `go.mod`
35. `go.sum`
36. `scripts/build-release.sh`
37. `scripts/generate-release-notes.sh`
37. `scripts/check-workflow-dispatch-inputs.py`
38. `scripts/clean-mock-alerts.sh`
39. `scripts/com.pulse.hot-dev.plist.template`
40. `scripts/dev-check.sh`
41. `scripts/dev-deploy-agent.sh`
42. `scripts/dev-launchd-setup.sh`
43. `scripts/dev-launchd-wrapper.sh`
44. `scripts/hot-dev-bg.sh`
45. `scripts/hot-dev.sh`
46. `scripts/lib/hot-dev-runtime.sh`
47. `scripts/lib/hot-dev-auth.sh`
48. `scripts/install-container-agent.sh`
49. `install.sh`
50. `scripts/install.ps1`
51. `scripts/install.sh`
52. `scripts/install-mcp.sh`
53. `scripts/install-mcp.ps1`
55. `scripts/pulse-auto-update.sh`
56. `scripts/release_control/internal/record_rc_to_ga_rehearsal.py`
57. `scripts/release_control/record_rc_to_ga_rehearsal.py`
58. `scripts/release_control/release_promotion_policy_support.py`
59. `scripts/release_control/resolve_release_promotion.py`
60. `scripts/release_control/mobile_release_gate.py`
61. `scripts/release_control/mobile_release_gate_test.py`
62. `scripts/release_candidate_manifest.py`
63. `scripts/release_control/validate_artifact_release_line.py`
63. `scripts/release_ldflags.sh`
64. `scripts/run_cloud_public_signup_smoke.sh`
65. `scripts/run_demo_public_browser_smoke.sh`
66. `scripts/demo_public_browser_smoke.cjs`
67. `scripts/run_hosted_staging_smoke.sh`
68. `scripts/trigger-release-dry-run.sh`
69. `scripts/trigger-release.sh`
70. `scripts/toggle-mock.sh`
71. `deploy/provider-msp/`
72. `deploy/helm/pulse/`
73. `tests/integration/playwright.config.ts`
74. `tests/integration/QUICK_START.md`
75. `tests/integration/README.md`
76. `tests/integration/scripts/bootstrap-hosted-mobile-onboarding.mjs`
77. `tests/integration/scripts/hosted-mobile-token-runtime.mjs`
78. `tests/integration/scripts/hosted-tenant-approval-store.mjs`
79. `tests/integration/scripts/hosted-tenant-runtime.mjs`
80. `tests/integration/scripts/hosted-tenant-runtime-restart.mjs`
81. `tests/integration/scripts/managed-dev-runtime.mjs`
82. `tests/integration/scripts/relay-mobile-token-helper.go`
83. `tests/integration/tests/helpers.ts`
84. `tests/integration/tests/runtime-defaults.ts`
85. `docker-compose.yml`
86. `scripts/install-docker.sh`
87. `scripts/validate-published-release.sh`
88. `scripts/validate-release.sh`
89. `scripts/release_asset_common.sh`
90. `scripts/backfill-release-assets.sh`
91. `.github/workflows/backfill-release-assets.yml`
92. `.github/scripts/check-demo-reachability.sh`
93. `.github/scripts/setup-demo-ssh.sh`
94. `scripts/trigger-stable-patch.sh`

## Shared Boundaries

1. `frontend-modern/src/api/updates.ts` shared with `api-contracts`: the updates frontend client is both a deployment-installability control surface and a canonical API payload contract boundary.
   The version payload consumed by this client must preserve the distinction
   between the running app build `version` and the deployable
   `agentUpdateTargetVersion`; update/install surfaces may display the app
   build, but agent update prompts must only use the agent target when the
   backend exposes one.
2. `internal/api/updates.go` shared with `api-contracts`: update handlers are both a deployment-installability control surface and a canonical API payload contract boundary.
3. `internal/cloudcp/docker/labels.go` shared with `cloud-paid`: hosted tenant Docker labels are both a Pulse Cloud runtime contract boundary and a deployment-installability rollout boundary.
4. `internal/cloudcp/docker/manager.go` shared with `cloud-paid`: hosted tenant container management is both a Pulse Cloud runtime contract boundary and a deployment-installability rollout boundary.
   Tenant runtime containers must use bounded Docker `json-file`
   logging so rollout and canary fleets cannot consume unbounded production
   host storage while they remain running.
   Tenant runtime creation and rollout must also resolve the workspace display
   name from the tenant registry and inject it as `PULSE_TENANT_NAME` next to
   `PULSE_TENANT_ID`; the rollout path is the canonical way a display-name
   change reaches a running client container, because rollout recreates the
   container with freshly resolved environment.
   Provider-hosted MSP workspace creation and preflight must prepare or report
   the configured tenant runtime image, configured Docker network, Docker
   daemon reachability, and storage-admission guardrails before the provider
   treats a fresh install as ready for client onboarding.
   Provider-hosted MSP installability treats `CP_DOCKER_NETWORK` as the
   provider ingress/support network, not the client runtime network. Each
   workspace runtime must be created on a per-tenant bridge derived by the
   Docker manager, with Traefik routing pinned to that bridge through
   `traefik.docker.network`. The packaged compose stack must label the Traefik
   and control-plane support containers so the control plane can attach them to
   each tenant bridge before starting the client runtime.
   The client runtime must be started as the rootless Pulse UID/GID by the
   Docker manager, with tenant data ownership prepared on the host before
   container creation. Provider-hosted installability proof must therefore
   exercise the actual `CreateAndStart` path with the real Pulse entrypoint
   shape, not only raw Docker container creation, so capability drops and the
   read-only root filesystem cannot break first tenant startup unnoticed.
   `pulse-control-plane provider-msp proof` must exercise the first-client
   onboarding path through workspace creation, client-bound install token
   generation, tenant-local unified-agent report ingest, tenant-bound install
   token rotation, rotated-out token rejection, handoff exchange,
   tenant-runtime report schedule creation, portal-visible active-alert rollup
   facts, and duplicate-hostname isolation before provider-hosted MSP
   installability is treated as proven. The proof is license-backed by default:
   `license_file` must be the
   resolved provider MSP plan source unless the operator explicitly opts into
   the local-development `--allow-env-plan` escape hatch.
   The same proof surface must also keep adversarial client-boundary probes in
   scope: workspace-limit check/create must be locked against concurrent cap
   bypass, handoff tokens must reject cross-workspace retargeting without being
   consumed, org-bound agent report tokens must not write into another client
   runtime, and rotated-out install tokens must be rejected immediately.
   `pulse-control-plane provider-msp install-proof` is the packaged fresh
   install rehearsal: it must bootstrap the provider owner, run license-backed
   preflight and status checks, run the workspace/runtime proof with cleanup
   delayed until after backup capture, create and verify a recovery archive,
   dry-run restore into a separate target data dir, dry-run failed-workspace
   recovery, remove proof workspaces when requested, and report final
   operational status.
   The packaged provider MSP compose bundle defaults to
   `CP_CONTROL_PLANE_MODE=provider_hosted_msp`, but must allow
   `pulse_hosted_msp` as an operator override for Pulse-operated MSP stacks
   without forking the deployment artifact. Both modes use the same
   `provider-msp` command group, license-backed proof path, isolated client
   runtime containers, and runtime URL shape `https://<client-id>.${DOMAIN}/`.
   `deploy/provider-msp/run-install-proof.sh` is the compose-level operator
   wrapper for that rehearsal. It must validate the provider `.env` and compose
   config, require a reachable Docker daemon, optionally pull the pinned
   provider images, start Traefik before proof workspace creation so isolated
   tenant bridges can attach their ingress support container, run the one-off
   `provider-msp install-proof` command through the packaged control-plane
   service, start the long-running provider stack, and finish with
   `provider-msp status`.
   `deploy/provider-msp/upgrade.sh` is the compose-level pre-upgrade and
   pre-maintenance runner for provider-hosted MSP. It must keep dry-run mode
   non-mutating, validate the provider `.env` and compose config, check
   provider status and preflight, create and verify a fresh backup before
   apply, dry-run restore into a separate target data dir, require backup
   readiness before and after provider service replacement, update the packaged
   Traefik/control-plane services, print the tenant runtime rollout plan for
   `CP_PULSE_IMAGE`, and only execute `tenant-runtime rollout --all --image
   <CP_PULSE_IMAGE>` when the operator explicitly asks for tenant rollout.
   `deploy/provider-msp/setup.sh` is the first-time provider host setup
   artifact. It must install the Docker/compose host prerequisites, create the
   provider data and backup layout, validate the provider Docker network when
   it already exists and otherwise let compose create it with the configured
   subnet, copy the provider MSP deploy bundle into a stable operator
   directory, create a private `.env` from the provider template when needed,
   fail closed on placeholder image refs, missing signed MSP license files,
   Dockerless production provisioning, disabled storage guardrails, or
   Stripe/cloud-signup variables, validate compose, and optionally hand off to
   `run-install-proof.sh` when the provider account name and owner email are
   supplied. Because provider-hosted MSP provisions tenant containers through
   the host Docker socket, the provider data directory must be mounted at the
   same absolute path inside the control-plane container that the host Docker
   daemon will later use for tenant runtime bind mounts.
   The setup artifact must also generate strong provider secrets when the
   template leaves `CP_ADMIN_KEY` or `CP_TRIAL_ACTIVATION_PRIVATE_KEY` blank,
   enforce minimum admin-key strength and a valid activation signing key before
   compose starts, require `CP_TRUSTED_PROXY_CIDRS` to include the provider
   Docker subnet, create the storage-admission marker directories, and install a
   host-level `DOCKER-USER` rule blocking `169.254.169.254` from tenant
   containers when iptables is available.
   The setup summary must leave the operator on a working next step, not a
   dead end: it must print the `provider-msp bootstrap` command that creates
   the operator account and portal sign-in link, and the day-2 sign-in
   guidance (re-running `bootstrap` for a fresh owner link and
   `provider-msp portal-link --email` for invited teammates), because the
   bundle default ships without a transactional email provider and the portal
   cannot send sign-in links in that state. `.env.example` must document the
   same commands next to `RESEND_API_KEY` so the runbook and the portal
   sign-in page agree. `provider-msp portal-link` is part of the packaged
   day-2 surface and mints links only for existing account members or pending
   invitees.
   Provider-hosted MSP installability must also pass provider-default report
   branding through the packaged tenant environment rather than requiring
   report-specific operator provisioning. The deployable control-plane config
   may carry `CP_REPORT_BRAND_*` values, and `internal/cloudcp/docker/manager.go`
   translates those into generic tenant runtime `PULSE_REPORT_PROVIDER_BRAND_*`
   variables; tenant Pulse runtimes still own report rendering and entitlement
   enforcement.
   `pulse-control-plane provider-msp status` is the non-mutating operational
   companion to that proof: it must report registry readiness, tenant
   state/health counts, stuck provisioning workspaces, Docker runtime
   prerequisites, storage guardrails, and the same license-backed plan identity
   without pulling tenant images unless the operator asks for it. It must also
   surface backup readiness for upgrades and recovery drills by identifying the
   latest verified provider MSP backup archive when one exists, warning when no
   backup is available yet, and offering a strict `--require-backup` status gate
   for pre-upgrade or pre-maintenance checks.
5. `internal/cloudcp/provider_msp_backup.go` shared with `cloud-paid`: provider-hosted MSP backup is both a cloud-paid license/account/runtime continuity boundary and a deployment-installability recovery artifact boundary.
   `pulse-control-plane provider-msp backup create`, `backup verify`, and
   `backup restore` must
   create a Stripe-free recovery archive outside the live
   control-plane/tenant source trees, snapshot SQLite control-plane databases
   through an online backup path, include tenant runtime directories for all
   non-deleted registry workspaces, include the signed MSP license file when
   the plan source is license-backed, verify the manifest, tenant registry
   snapshot, license artifact, and tenant runtime directories, and fail closed
   on restore when target provider MSP state already exists unless the operator
   explicitly uses the replace gate after stopping the control plane.
6. `internal/cloudcp/provider_msp_recovery.go` shared with `cloud-paid`: provider-hosted MSP failed-workspace recovery is both a cloud-paid license/account/runtime continuity boundary and a deployment-installability recovery artifact boundary.
   `pulse-control-plane provider-msp recover` must offer a dry-run plan and an
   explicit execution path for failed, stuck provisioning, and unhealthy active
   client workspaces; it must require the signed provider MSP license source by
   default, refuse to recover from missing tenant data, and reuse the canonical
   tenant-runtime rollout path before marking the workspace active again.
7. `internal/cloudcp/tenant_runtime_rollout.go` shared with `cloud-paid`: hosted tenant runtime rollout is both a Pulse Cloud runtime contract boundary and a deployment-installability release-rollout boundary.
7. `scripts/install.ps1` shared with `agent-lifecycle`: the Windows installer is both a deployment installability entry point and a canonical agent lifecycle runtime continuity boundary.
   It must expose a non-mutating preflight for the exact Windows agent
   architecture before Administrator-only install changes, accept token-file
   enrollment input, and avoid interactive download-failure prompts when
   launched by generated non-interactive onboarding commands. A completed
   install must own a durable rotating ProgramData log, verify that log
   together with local `/readyz`, and fail closed if required SCM recovery
   actions or non-crash recovery cannot be configured. The Windows native CI
   path must run the reusable lifecycle harness rather than stopping at a
   parser check or foreground self-test.
8. `scripts/install.sh` shared with `agent-lifecycle`: the shell installer is both a deployment installability entry point and a canonical agent lifecycle runtime continuity boundary.
   A caller-supplied `--state-dir` must remain canonical across rendered
   systemd, launchd, OpenRC, rc.d, SysV, NAS wrapper, bootstrap, and reference
   environment artifacts. Update and uninstall without a repeated custom path
   must discover it from the active process or managed service before looking
   at default-path state; explicit custom-path operations must not fall back
   to another default instance. `connection.env` records the canonical state
   and token-file paths without storing the token value, update rewrites the
   same secure service shape, and uninstall removes the discovered canonical
   directory rather than only `/var/lib/pulse-agent`.
   Existing-agent update commands copied from the settings UI must use the
   installer-owned `--update` mode rather than serializing a fresh enrollment
   token into platform notice links. In `--update` mode, `scripts/install.sh`
   must recover the server URL, token-file state, identity material, CA trust
   settings, insecure flag, and persisted agent id from the local installed
   agent state, must fail closed when no existing installation or connection
   state is present, and must refuse to silently become a new install command.
   That recovery must not depend only on a v6 `connection.env`: v5.1.x agents
   that predate persisted connection state may recover the existing URL, token,
   feature flags, identity, and trust posture from the running `pulse-agent`
   process or its systemd service definition, then persist the upgraded runtime
   back through the v6 token-file service-argument path rather than keeping the
   raw token in process arguments. That fallback remains required when the
   operator supplies `--url` on the update command but token, identity,
   feature-flag, or trust continuity still exists only in legacy process or
   service state. Legacy v5.1.x Linux services that relied on the Go agent's
   implicit `/var/lib/pulse-agent/token` fallback may recover that default
   token file only after local process, service, or saved-state context has
   supplied the agent connection shape; the token file alone is not enough to
   convert a missing-state update into a new install. Because v5.1.x agents
   used Go's single-dash flag spelling, the installer-owned recovery path must
   accept both single-dash and double-dash forms for recovered agent args
   without weakening the existing missing-state failure behavior.
   FreeBSD and pfSense updates have the same continuity obligation without
   Linux procfs or systemd: the installer must read live process arguments via
   `ps` (and environment via `procstat` when available), then fall back to the
   installed rc.d service's `command_args` and `PULSE_*` exports. The parser
   must preserve quoted argument values without evaluating service-file shell
   content, and the rewritten rc.d service must use `--token-file` rather than
   retaining a recovered raw token.
   FreeBSD-family uninstall must stop the rc.d daemon(8) supervisor before
   removing the binary, then remove service registration, rc.conf enablement,
   boot wrappers, PID files, token/state, and residual processes before it can
   report success. A checksum-verified native rehearsal must cover install,
   update, reboot persistence, and clean uninstall rather than treating a
   cross-build as complete lifecycle proof.
   The shell installer must disclose `--enable-commands` as Pulse command
   execution, disabled by default, and must name both Patrol actions and
   Proxmox LXC Docker inventory as the operator-visible reasons to enable it.
   When enabled, the terminal summary must also state that Proxmox LXC Docker
   inventory still requires explicit server-side
   `PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY=true`.
   For command-enabled PVE agents, the generated systemd unit must keep the
   normal service hardening except for the two flags that block host-side
   `pct exec` / `lxc-attach`: `NoNewPrivileges=false` and
   `RestrictSUIDSGID=false`. That exception is deployment-owned operator truth
   for the Proxmox LXC Docker inventory path and must not leak into non-PVE or
   non-command agent installs.

## Extension Points

1. Add or change deployment-type detection, update planning, or apply behavior through `internal/updates/`
2. Add or change release-build metadata injection, Docker build-context allowlists, release artifact assembly, governed promotion metadata resolution, artifact release-line validation, the canonical version file, operator-facing release packet content, prerelease feedback intake wording, historical published-release integrity backfill, release asset validation status publication, download endpoint checksum/signature header proof, end-to-end install.sh smoke against the published release, or the canonical in-repo v6 upgrade guide through `scripts/build-release.sh`, `scripts/release_asset_common.sh`, `scripts/backfill-release-assets.sh`, `scripts/release_ldflags.sh`, `scripts/check-workflow-dispatch-inputs.py`, `scripts/release_control/mobile_release_gate.py`, `scripts/release_control/render_release_body.py`, `scripts/release_control/resolve_release_promotion.py`, `scripts/release_control/validate_artifact_release_line.py`, `scripts/release_control/record_rc_to_ga_rehearsal.py`, `scripts/release_control/internal/record_rc_to_ga_rehearsal.py`, `scripts/release_control/release_promotion_policy_support.py`, `.dockerignore`, `Dockerfile`, `.github/ISSUE_TEMPLATE/v6_rc_feedback.yml`, `docs/RELEASE_NOTES.md`, `docs/releases/`, `docs/UPGRADE_v6.md`, `docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md`, `docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md`, `docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md`, `scripts/validate-release.sh`, `scripts/validate-published-release.sh`, the operator dispatch helpers `scripts/trigger-release.sh` and `scripts/trigger-release-dry-run.sh`, and the governed release workflows `.github/workflows/backfill-release-assets.yml`, `.github/workflows/create-release.yml`, `.github/workflows/deploy-demo-server.yml`, `.github/workflows/helm-pages.yml`, `.github/workflows/install-sh-smoke.yml`, `.github/workflows/publish-docker.yml`, `.github/workflows/publish-helm-chart.yml`, `.github/workflows/promote-floating-tags.yml`, `.github/workflows/release-dry-run.yml`, `.github/workflows/update-demo-server.yml`, and `.github/workflows/validate-release-assets.yml`
   Normal releases are single-build promotions. The exact pushed SHA must
   produce one signed candidate through
   `.github/workflows/build-release-candidate.yml` while independent release
   checks run in parallel. `create-release.yml` may publish only that candidate
   after `scripts/release_candidate_manifest.py` verifies its version, source
   SHA, filenames, sizes, and SHA-256 values. Standard post-upload validation
   must compare that manifest with GitHub's server-side asset digests instead
   of downloading the complete release packet again. Historical repair and
   release-edit validation may use the full-download fallback because those
   paths do not have a same-run candidate manifest.
   The candidate job timeout must cover signed multi-platform assembly, full
   local packet validation, manifest creation, and artifact upload; the
   observed release path requires a 60-minute ceiling even though the build
   itself is expected to finish much earlier.
   Tarball entry validation must extract the requested files once per archive;
   it must not decompress a multi-gigabyte release archive again for every
   required entry, and the release-promotion contract test must reject a return
   to per-entry archive streaming.
   A manually dispatched release rehearsal must activate the same signed
   candidate build whenever its required `version` input is non-empty and must
   apply the same channel-specific native-signing policy as a publish run.
   macOS notarization remains mandatory for both prerelease and stable
   candidates. Windows Authenticode remains mandatory for stable candidates;
   prerelease candidates may retain checksum and detached-signature
   verification without Authenticode while the release packet explicitly
   discloses the unknown-publisher warning and stable promotion remains
   blocked. A cheap signing-configuration job must report every missing secret
   for the platforms required by that candidate before either platform runner
   is allocated. Stable Windows signing must use SignPath's GitHub
   trusted-build-system action by default, submit an immutable GitHub artifact
   by id, verify every returned executable, and retain evidence binding the
   request, source SHA, signer identity, and file digests. A repository-secret
   PFX backend is an explicitly selected break-glass fallback only.
   macOS command-line agent notarization must fail closed unless
   `notarytool --wait --output-format json` reports `Accepted`, then verify the
   exact candidate bytes with strict `codesign`. Bare Mach-O command-line
   binaries are not app bundles, so `spctl --assess --type execute` is not a
   valid post-notarization gate for this artifact shape.
   Scheduled watchdog rehearsals omit that input and must skip candidate
   signing while retaining the non-publish policy and integration checks.
   Release-facing agent-paradigm blurbs under `docs/releases/` must describe
   `pulse-mcp` as a generic MCP adapter for MCP-speaking clients, not a
   client-specific release artifact, and full-surface token guidance must come
   from the manifest-owned `requiredScopes` list so release notes cannot drift
   away from the shipped adapter.
   The `install-sh-smoke.yml` workflow runs end-to-end against the
   published release in a privileged systemd container: it downloads
   `install.sh` and `install.sh.sshsig` from the GitHub Release URL,
   runs the README-documented `ssh-keygen -Y verify` step against the
   real signed asset using the README's pinned key, re-checks the
   server-installer banner / `--version)` arg handler / agent-banner
   absence against the published bytes (not just the local build), then
   actually runs `bash install.sh --archive <tarball> --disable-auto-updates`
   inside the container and asserts `systemctl is-active pulse`, a 200
   from `/api/health`, and a version match from `/api/version`.
   `create-release.yml` must call this workflow as a downstream
   `workflow_call` after `validate-release-assets.yml` succeeds for every
   release that is not a `historical_asset_backfill_only` run; without
   that wiring the smoke gate exists but never protects a release. Draft-only
   release runs are not a publication boundary and must skip downstream
   install smoke, Helm chart publication, and floating tag promotion because
   draft assets are not publicly downloadable and those publish steps would
   advance externally visible state before operator publication.
   The README's pinned `pulse-installer` ed25519 key must verify
   `install.sh.sshsig` for the published release; this is enforced by
   `scripts/validate-release.sh` at build time and re-verified by
   `install-sh-smoke.yml` against the served asset.
3. Add or change root server installer, shell installer, Docker bootstrap installer, Windows installer, container-agent installer, repo-root compose defaults, or auto-update script behavior through `install.sh`, `scripts/install.sh`, `scripts/install-docker.sh`, `scripts/install.ps1`, `scripts/install-container-agent.sh`, `docker-compose.yml`, and `scripts/pulse-auto-update.sh`
   The root `install.sh` server installer owns its fresh-host dependency
   bootstrap for supported Debian, Ubuntu, and Proxmox targets. It must install
   `curl`, `wget`, `ca-certificates`, and `openssh-client` before installing
   release archives, with `jq` as an optional reliability dependency; release
   signature verification depends on `ssh-keygen` from `openssh-client` and
   must not fail on a minimal supported host solely because that package was
   absent before installation started.
   The server systemd unit that root `install.sh` writes
   (`install_systemd_service`) hardens with `NoNewPrivileges=true`, which
   strips setuid and file capabilities from every child the server executes.
   ICMP availability probes exec the system `ping` binary, so the same
   hardening block must also grant `AmbientCapabilities=CAP_NET_RAW` with a
   matching `CapabilityBoundingSet=CAP_NET_RAW`; dropping either regresses
   ICMP availability checks to permanent failure on every systemd install
   (discussion #1554). `scripts/installtests/root_install_sh_test.go`
   (`TestRootInstallServiceGrantsIcmpProbeCapability`) pins the pairing, and
   `docs/CONFIGURATION.md` documents the `systemctl edit` override for units
   written before the grant existed.
   The top-level `install.sh` asset published on GitHub Releases must be the
   root Pulse SERVER installer (the LXC / systemd / Proxmox VE installer that
   accepts `--version vX.Y.Z`, `--rc`, `--stable`, and friends). The rendered
   AGENT installer (`scripts/install.sh`) ships only inside release tarballs
   at `./scripts/install.sh` and inside Docker images at
   `/opt/pulse/scripts/install.sh`, and is served at the running server's
   `/install.sh` endpoint; it is intentionally never the top-level GitHub
   Releases asset. `scripts/pulse-auto-update.sh` and the root `install.sh`'s
   own `--rc` / `--stable` / `--version` self-refetch flows all fetch
   `releases/<tag>/install.sh` and execute it via `bash -s -- --version vX.Y.Z`,
   and the README quickstart documents the same pattern. Publishing the agent
   installer in that slot silently breaks every one of those flows because the
   agent installer rejects `--version` as an unknown argument; this drift
   shipped across v6 rc.1 → rc.5 (April 12 → May 11, 2026) before being caught.
   Installer-facing command-execution copy must remain aligned with the served
   agent installer: Proxmox LXC Docker inventory may be described only as an
   opt-in host-side path that requires both agent command execution and server
   guest-Docker inventory opt-in.
   `scripts/validate-release.sh` must therefore fail the release if the
   published `install.sh` does not carry the server-installer banner, does not
   handle `--version)` in its argument parser, contains the agent installer
   banner string, or does not print the server installer's version-pinning
   help line when invoked with `--help`.
   The served `/install.sh` endpoint must only ever hand out the AGENT installer,
   never the top-level GitHub `install.sh` release asset (which, per the rule
   above, is the SERVER installer and rejects the "Install on Linux" wizard's
   `--url` / `--token-file` with "Unknown option"). Two layers enforce this and
   both must hold: (a) `internal/api/unified_agent.go::handleDownloadInstallScriptCommon`
   serves the locally bundled agent installer and has no GitHub fallback at all —
   it must never proxy the GitHub `install.sh` asset; a present-but-unsigned local
   agent installer is served as-is (nothing on the `curl ... | bash` agent path
   verifies the signature headers, so an unsigned-but-correct local script beats a
   signed-but-wrong proxied one) and a genuinely-missing local script fails closed
   with 503; and (b) a server install should still deploy the script's
   `.sig` / `.sshsig` sidecars next to it (`/opt/pulse/scripts/install.sh.sig`,
   `.sshsig`) so the served script carries signatures. The Docker image deploys
   these sidecars (`Dockerfile`); `install.sh`'s `deploy_agent_scripts` must
   deploy them for LXC / systemd installs too. The original gap (sidecars never
   deployed on LXC, so the endpoint proxied the SERVER installer) shipped as the
   rc.6 agent-wizard regression (issue #1470).
   The agent installer update path must also cover legacy v5 process-state
   recovery as a first-class installability behavior: a tokenless
   `--update --url <server>` command copied from the v6 UI must be able to
   upgrade an already-running v5.1.x `pulse-agent` that was launched with
   `--url`, `--token`, feature flags, and identity arguments even when
   `connection.env` is missing or incomplete. The upgraded service must be
   rendered through the shared exec-argument builder and use the secure
   `--token-file` runtime path, and explicit update URL arguments must not
   suppress legacy recovery of the remaining connection, identity, feature, or
   trust fields.
   Root `install.sh` Proxmox server auto-registration must not persist a newly
   created monitoring token into Pulse until the installer has applied the
   token ACLs and smoke-tested that exact `PVEAPIToken` against the local
   Proxmox `/api2/json/nodes` endpoint. A failed token smoke check must leave
   the installer in a manual-completion state instead of POSTing
   `/api/auto-register`.
   Deployment bootstrap token behavior remains a deployment-installability
   trust boundary even when the handler is API-owned. `internal/api/deploy_handlers.go`
   must preserve server-derived `owner_user_id` lineage on bootstrap tokens and
   enrollment runtime tokens while keeping deploy binding metadata limited to
   deploy facts such as cluster, job, target, source agent, and expected node.
4. Add or change server update transport and release-note presentation through
   `internal/api/updates.go`, `internal/updates/`,
   `frontend-modern/src/api/updates.ts`,
   `frontend-modern/src/components/UpdateBanner.tsx`,
   `frontend-modern/src/components/WhatsNewCard.tsx`,
   `frontend-modern/src/components/whatsNewModel.ts`, and
   `frontend-modern/src/utils/localStorage.ts`
   Server update planning must attach the canonical upgrade-readiness verdict
   to `/api/updates/plan` responses before an operator starts a v6 update, and
   `POST /api/updates/apply` must recompute the same verdict and reject
   `blocked` updates server-side rather than trusting the settings UI alone.
   The verdict belongs to the update plan, not to a separate migration wizard:
   it must combine updater capability, rollback availability, registered agent
   continuity, and agent reporting token scope so v5-to-v6 continuity problems
   are visible before relaunch. The root `install.sh` non-UI path must run a
   conservative v5-to-v6 local preflight before replacing the binary, blocking
   unreadable token state and warning about missing, expired, or soon-expiring
   agent reporting scopes without pretending shell-only inspection can prove
   live registered-agent continuity.
   The authenticated running-release notes endpoint must fetch only the exact
   published tag for the running release, cache both hits and misses, stay
   unavailable for source/development builds, and return the canonical release
   body without inventing a second changelog source. The update banner may
   preview only the curated `Highlights` section from update-check metadata,
   while the post-update card may show that same section once per later
   installed release and must stay silent for a first baseline, malformed or
   development versions, missing releases, and releases without highlights.
5. Add or change local dev-runtime orchestration, managed ownership, browser-runtime proof wiring, frontend/backend coherence diagnostics, canonical developer entry wrappers, deterministic dev auth seeding, dependency manifest floors, frontend build chunking, or dev-runtime helper control surfaces through `scripts/hot-dev.sh`, `scripts/hot-dev-bg.sh`, `scripts/lib/hot-dev-runtime.sh`, `scripts/lib/hot-dev-auth.sh`, `scripts/dev-deploy-agent.sh`, `Makefile`, `package.json`, `package-lock.json`, `frontend-modern/package.json`, `frontend-modern/package-lock.json`, `frontend-modern/vite.config.ts`, `go.mod`, `go.sum`, `scripts/dev-check.sh`, `scripts/toggle-mock.sh`, `scripts/clean-mock-alerts.sh`, `scripts/dev-launchd-setup.sh`, `scripts/dev-launchd-wrapper.sh`, `scripts/run_demo_public_browser_smoke.sh`, `scripts/demo_public_browser_smoke.cjs`, `scripts/com.pulse.hot-dev.plist.template`, `tests/integration/scripts/managed-dev-runtime.mjs`, `tests/integration/playwright.config.ts`, `tests/integration/tests/helpers.ts`, `tests/integration/tests/runtime-defaults.ts`, `tests/integration/README.md`, and `tests/integration/QUICK_START.md`
   First-run browser helpers are part of that dev-runtime proof boundary. They
   must preserve the setup-created API token in the shared runtime state, prefer
   deterministic token authentication after setup, and may use the server setup
   API as a fallback only when the UI wizard fails to complete cleanly under the
   managed hot-dev runtime.
   Managed browser verification must also restart an existing hot-dev session
   when a verification lock is active or the runtime auth file no longer matches
   the deterministic dev user/hash. `tests/integration/scripts/run-playwright.mjs`
   owns the run-scoped `HOT_DEV_VERIFY_LOCK_FILE` handoff so overlapping browser
   proof cannot reuse stale first-run credentials.
   The managed and foreground hot-dev entrypoints must share one network-default
   contract: local dev binds frontend and backend traffic to loopback by default
   so installed LAN agents cannot accidentally treat a developer laptop as the
   active Pulse control plane. LAN exposure for agent/mobile testing must be an
   explicit `PULSE_DEV_LAN=true` opt-in; only that mode may bind Vite/backend
   listeners to `0.0.0.0`, include LAN origins, or advertise the detected LAN
   browser entrypoint.
   The canonical shortcut for the installed homelab-agent case is lab-agent
   mode: `PULSE_DEV_LAB_AGENTS=true` must enable LAN exposure plus the
   Proxmox guest Docker detection/inventory opt-ins together, and the
   repo-root wrapper surface must expose that mode through `npm run dev:lab`,
   `npm run dev:restart:lab`, `npm run dev:status:lab`,
   `npm run dev:verify:lab`, and `npm run dev:foreground:lab`. The frontend
   workspace package and Makefile must delegate their lab-agent targets through
   those same repo-root npm wrappers rather than duplicating raw launcher
   commands or teaching developers to paste one-off environment strings.
   After a developer explicitly starts lab-agent mode, the managed and
   foreground hot-dev launchers may remember that local workspace opt-in in
   ignored `tmp/` state so ordinary managed restarts do not silently strand
   already-installed LAN agents or lose the Proxmox guest Docker inventory
   flags. Clean checkouts must still default to loopback-only development, and
   an explicit `PULSE_DEV_LAB_AGENTS=false` / `PULSE_DEV_LAN=false` launch must
   clear the remembered opt-in and return the workspace to local-only binding.
   The hot-dev supervisor must also recover its managed PID file from a live
   `hot-dev-bg.sh supervise` process before treating the runtime as unmanaged.
   Backend health monitoring must distinguish HTTP startup grace from a missing
   backend process: a missing `./pulse` process may be tolerated only for the
   short configured missing-process grace, after which the managed runtime must
   restart it instead of waiting for the full HTTP warmup window.
   Managed hot-dev first-run recovery is part of the same proof boundary:
   non-production dev data directories must seed the deterministic E2E
   bootstrap token when no token file exists, and browser helpers must prove
   the target first-run handoff UI rendered instead of accepting a route match
   while the setup wizard is still blocking the app.
   Hot-dev must also recreate the local Pro audit signing key env binding when
   first-run reset removes the runtime `.env`; otherwise the Pro backend fails
   closed before binding the API port and the supervisor loops without ever
   reaching browser-verifiable health.
   Hot-dev backend launches, supervisor child launches, and takeover restarts
   must also preserve `LOG_LEVEL` and the Proxmox guest-Docker opt-in
   environment (`PULSE_ENABLE_PROXMOX_GUEST_DOCKER_DETECTION`,
   `PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY`, and
   `PULSE_PROXMOX_GUEST_DOCKER_INVENTORY_VMIDS`) so live dev verification of
   host-side LXC Docker inventory does not silently restart into default-off
   monitoring.
   Browser authentication helpers used by release and managed-runtime E2E must
   keep session creation below the backend login limiter. Shared helpers must
   treat an HTTP 429 response from `POST /api/login` as the retryable
   `Too many requests` outcome instead of collapsing it into a generic
   connection failure, and release suites that run many scenarios against one
   compose backend must prefer worker-scoped authenticated storage state over
   repeated per-test password logins.
   Multi-tenant release helpers must also treat organization switching as a
   visible runtime-state contract: after persisting `pulse_org_id` and reloading,
   the helper must wait for the Organization selector to hold the requested org
   before a scenario navigates onward, so an interrupted org-list bootstrap
   cannot fall back to `default` and mask the scoped UI under test.
6. Add or change governed release-promotion workflow inputs, operator-facing promotion metadata, the canonical version file, prerelease feedback intake prompts, artifact publication lineage enforcement, release note or changelog packet composition, or stable-promotion rehearsal summaries through `.github/workflows/create-release.yml`, `.github/workflows/helm-pages.yml`, `.github/workflows/publish-docker.yml`, `.github/workflows/publish-helm-chart.yml`, `.github/workflows/promote-floating-tags.yml`, `.github/workflows/release-dry-run.yml`, `.github/workflows/update-demo-server.yml`, `.github/ISSUE_TEMPLATE/v6_rc_feedback.yml`, `docs/RELEASE_NOTES.md`, `docs/releases/`, `docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md`, `docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md`, `docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md`, `scripts/check-workflow-dispatch-inputs.py`, `scripts/release_control/mobile_release_gate.py`, `scripts/release_control/mobile_release_gate_test.py`, `scripts/release_control/render_release_body.py`, `scripts/release_control/validate_artifact_release_line.py`, `scripts/release_control/record_rc_to_ga_rehearsal.py`, `scripts/release_control/internal/record_rc_to_ga_rehearsal.py`, `scripts/release_control/release_promotion_policy_support.py`, `scripts/trigger-release.sh`, and `scripts/trigger-release-dry-run.sh`
   That release-promotion boundary also owns prerelease note packet lineage:
   shipped RC notes must remain historically accurate, the top-level
   `docs/RELEASE_NOTES.md` index must continue to point at the current shipped
   and draft packets coherently, and each later RC should get its own draft or
   published release-notes, changelog, and support packet instead of silently
   rewriting the already-shipped `rc.1` operator context in place.
   The same boundary also owns packet discoverability and operator execution
   clarity: the release index must point at the full current RC packet rather
   than only one markdown file, and prerelease runbook commands should stay
   parameterized to the current candidate version instead of hard-coding stale
   `rc.1` examples once later RCs exist.
   Published release bodies must also stay publication-safe even when operators
   feed draft packet content into the workflow: `.github/workflows/create-release.yml`
   must render the public release body through a canonical sanitizer instead of
   publishing `Draft Release Notes` framing, `_DRAFT.md` packet links, or
   duplicate appended `Installation` / `Promotion Metadata` sections verbatim.
   `docs/UPGRADE_v6.md` must also stay aligned with the current RC support
   packet so upgrade guidance does not keep pointing operators at retired
   rollout/support docs after a later RC packet is prepared.
   The upgrade guide's license and entitlement guidance must also stay aligned
   with the free-first self-hosted GA posture: it may describe activation,
   recovery, and signed support handoffs, but it must not teach ordinary
   self-hosted users to start a general in-app trial or depend on hosted AI
   quickstart acquisition as part of the upgrade path.
   The same guide must treat bounded monitored-system, guest, or
   child-resource volume caps after self-hosted v6 activation or migration as
   regressions, not as upgrade outcomes or paid-plan differentiators.
   Release notes, changelog packets, and operator support packets under
   `docs/releases/` must follow the same rule when they mention licensing:
   historical RC context may be preserved, but current self-hosted v6 guidance
   must not present monitored-system volume, child-resource volume, guest
   capacity, or trial eligibility as the active paid model.
   When those packets describe Relay, they must use the same paid-feature
   wording as the pricing contract: secure remote access to the Pulse web UI,
   Pulse Mobile pairing for handoff, push notifications, and 14-day history,
   not generic mobile-app monitoring access.
   Current stable release notes, changelog packets, and operator support packs
   must also preserve the Infrastructure-first navigation contract: they may
   mention Dashboard only as historical context or generic custom-user tooling,
   not as the current default landing route or a current primary v6 surface.
   The active prerelease cut must keep the repo-root `VERSION` file aligned
   with the current RC packet itself: when the governed line moves from `rc.1`
   to `rc.2` or later, the staged release-notes packet, changelog packet, and
   operator support packet must describe that same candidate instead of leaving
   the branch on a newer version string while the in-repo packet still speaks
   for an older RC.
   Later corrective RCs such as `rc.3`, `rc.4`, `rc.5`, `rc.6`, and `rc.7`
   must also carry the live stable rollback target and any prerelease
   trust-root continuity caveat in the
   current release notes, changelog, operator support pack, upgrade guide, and
   release-control evidence record before the release workflow is dispatched.
   The `rc.7` prerelease packet keeps v6 on the opt-in prerelease channel,
   records the current stable rollback target as `v5.1.35`, and must take
   precedence over prepared stable `v6.0.0` packet wording until an actual
   stable GitHub release exists.
   When a draft packet is updated after the candidate tag or draft release has
   already been prepared, the packet must record an exact previous-RC to
   current-candidate commit coverage audit, include any new artifact
   validation or release-pipeline assertions in the release-control evidence,
   and refresh the draft release from the new branch head before publication.
   If more candidate commits land after that audit but before the release
   workflow is dispatched, the same packet must be refreshed again against the
   new candidate head, including the exact commit count, candidate commit hash,
   changed-scope summary, and any new release-risk themes introduced by those
   commits.
   Installer-resolution fixes that affect stable versus prerelease selection
   are one of those release-risk themes and must be named in the current RC
   packet before the release workflow is restarted.
   Release-validation proof corrections that unblock an RC draft, including
   backend CI proof fixes that do not change runtime behavior and runtime
   guard fixes discovered by the release workflow itself, must still be named
   in the audit record and reflected in the candidate commit hash, commit
   count, and changed-scope summary before the workflow is restarted.
   Release workflows and demo-update workflows must derive the OpenSSH
   installer trust key from `PULSE_UPDATE_SIGNING_PUBLIC_KEY`, not from a
   duplicated hand-copied key. The release workflow must fail before
   publication if the repo-root server installer or auto-update script does
   not trust that configured signing key, and the demo-update workflow may
   patch the derived trust key into an immutable historical tagged installer
   copy before executing that installer for an already-published RC.
   A metadata-only packet refresh may identify the last validation commit that
   introduced release risk separately from the packet-refresh commit itself,
   but it must make that distinction explicit in the release notes and audit
   record before dispatch.
   For the `rc.4` release packet, that distinction is explicit: the
   code-backed validation-risk range ends at the config watcher lifecycle fix,
   while a later packet-only refresh may be the branch head used for the final
   release workflow dispatch.
   The prerelease feedback intake template and active demo/update metadata must
   also stay on generic or current-RC wording instead of hard-coding stale
   `rc.1` examples once later candidates exist.
   GA signoff must also treat prerelease feedback intake as a live surface, not
   a one-time issue export: the owned checklist and release runbook must force
   a last-pass review of new issues, new issue comments, the pinned prerelease
   feedback hub, and equivalent actionable RC reports before a candidate is
   declared feature-complete.
   Paid-user GA is part of that same release boundary: the public Pulse release
   workflow builds OSS `pulse-v...` artifacts only, so release docs and runbooks
   must require a same-tag/same-version `pulse-enterprise` Pro package for
   customer-facing publication, verify `pulse-pro-v...` archives identify
   `Pulse Pro`, and keep the paid install/upgrade path pointed at Pro artifacts
   or a verified paid image before any paid-user Pro runtime claim is made.
   During the v6 RC phase, private Pulse Pro archive prefixes and Docker tags
   must retain the RC suffix from the exact public Pulse RC tag; GA-shaped
   `6.0.0` Pro archive names, R2 prefixes, and Docker tags are reserved for the
   intentional v6 GA publish. Public GitHub release assets and the
   public `rcourtman/pulse` Docker image must be described as community builds
   where paid customers are likely to install or upgrade, and generated public
   release bodies must send Relay, Pulse Pro, and eligible legacy customers to
   `https://pulserelay.pro/download.html` for the private Pulse Pro Docker image
   or Linux/LXC archive. Public Docker and install docs must also preserve a
   `PULSE_IMAGE`-aware compose image line and warn that any hardcoded
   `image: rcourtman/pulse:...` line must be replaced before the private
   Pulse Pro compose commands can move an existing Docker install off the
   community image. The root installer must accept private `pulse-pro-v...`
   archive filenames through `--archive` so direct Linux and Proxmox LXC users
   can keep the normal service setup while installing the private Pulse Pro
   runtime.
   The in-app updater must never install a public community build on the
   compiled Pulse Pro binary: when the running edition is Pro (recorded by
   `pkg/edition`, flipped to `pro` in `enterpriseruntime.Initialize` alongside
   `coreaudit.SetLogger`/`server.SetBusinessHooks`, and keyed off the compiled
   binary — never license-active state), `internal/updates` checks and applies
   updates exclusively through the license server download broker
   (`GET /v1/downloads/pulse-pro` with the installation token and instance
   fingerprint, per `internal/updates/pro_update.go`), verifying the private
   archive against the same pinned `pulse-installer` SSHSIG key plus the
   broker manifest sha256, and refusing GitHub-shaped download URLs outright.
   The broker is dual-channel (pulse-pro side, 2026-07-22): the stable
   manifest slot answers every request by default, and a separate rc slot
   answers `channel=rc`. The Pro updater must send `channel=rc` on the broker
   request for rc-channel installs and must leave the channel parameter unset
   for stable-channel installs, keeping the stable request byte-identical for
   brokers that predate the parameter. A stable-channel install therefore
   tracks the broker's stable slot; the existing client-side guard (a
   stable-channel install refuses a prerelease broker pin and reports "no
   update" with a warning) stays as the backstop for a single-manifest or
   drifted broker.
   An unactivated Pro binary refuses to apply and directs the operator to
   `https://pulserelay.pro/download.html` and the `install.sh --archive` path.
   This is required because the community self-update flow (the in-app GitHub
   path, `install.sh` defaults, and the unattended
   `scripts/pulse-auto-update.sh` timer — which must skip when the installed
   binary reports `Pulse Pro`) targets the public `rcourtman/Pulse` community
   assets and would replace the Pro binary and silently strip Audit, RBAC,
   Reporting, and SSO from a paying customer. A community binary with an
   active paid license is still community and must keep its normal
   self-update; the `frontend-modern` update banner keeps the in-app apply
   affordance for auto-updatable Pro deployments (the broker path preserves
   the Pro runtime) and surfaces the portal path for deployments the updater
   cannot drive, such as Docker.
   A Docker deployment of the compiled Pro binary is part of that same
   boundary: the container cannot self-replace its binary and a Pro compose
   file pins the previous image digest, so `internal/updates/pro_update.go`
   must relay the broker manifest's Docker command block (login plus compose
   pull/up referencing `image@sha256:<digest>`, never a mutable tag) as
   `UpdateInfo.dockerUpdate` on the update-check response for Docker
   deployments, behind the same stable/rc channel guard as the binary path,
   failing closed when the broker block is missing or not digest-pinned.
   In-app update guidance (the Settings updates surfaces, the update banner,
   and the docker update plan in `internal/updates/adapter_installsh.go`)
   must never show the community `rcourtman/pulse` pull commands when the
   compiled runtime is Pro.
   Customer-facing private Pro RC/GA promotion is part of that same boundary:
   for every non-draft v6 public release, `create-release.yml` must call the
   private `rcourtman/pulse-enterprise` `Build Pro Release` workflow after
   `validate_release_assets` succeeds, pass the exact public tag/version, set
   `upload_to_r2=true` and `publish_docker_image=true`, wait for that workflow
   to succeed, then call the private `rcourtman/pulse-pro`
   `Promote Paid Runtime Release` workflow with the same version and R2 prefix.
   The promotion workflow downloads the signed proof packet and runs
   `scripts/promote_paid_runtime_release_packet.sh --release-dir <proof-packet-dir> --execute-live`
   from `repos/pulse-pro`. That command is the canonical live-broker promotion
   path because it validates the signed proof packet, installs the exact
   manifest on `pulse-license`, runs the customer-path live proof, and restores
   the previous remote manifest if the gate fails. GA promotions also require
   `--allow-ga-prefix`. A failed private build or failed live promotion must
   fail the public release workflow; future private Pro publication must not
   depend on an operator noticing a manual checklist step after the public RC
   has shipped.
   A support-only private Pro prerelease image is a narrower exception for
   customer verification of an already-fixed defect. It may dispatch the private
   `Build Pro Release` workflow with `publish_docker_image=true`,
   `upload_to_r2=false`, the exact `vX.Y.Z-rc.N` `pulse_ref`, and the matching
   `X.Y.Z-rc.N` version. That path may publish only the explicit private Docker
   tag, for example `license.pulserelay.pro/pulse-pro:X.Y.Z-rc.N`; it must not
   move `latest`, stable semver tags, R2 manifests, broker download metadata,
   public GitHub release assets, or the public `rcourtman/pulse` image.
   The repo-root VERSION file is part of the same governed boundary and must
   not drift as an
   unowned release-cut switch: changing the version string for a new RC or
   stable cut belongs to this subsystem and its release-promotion proof path.
   Stable promotion is part of that same lineage boundary: once a governed
   `6.0.0` candidate is prepared, the canonical stable packet names under
   `docs/releases/` may only be reused after the already-shipped RC packet is
   preserved under explicit historical filenames, the top-level
   `docs/RELEASE_NOTES.md` index keeps both the stable packet and the preserved
   RC packet discoverable, and `docs/UPGRADE_v6.md` points operators at the
   live stable support transition instead of a retired prerelease packet.
   The first stable `6.0.0` GA packet must keep the promoted prerelease tag,
   rollback target, exact GA date, and exact v5 end-of-support date aligned
   across release notes, upgrade guidance, support policy, promotion records,
   and release-promotion resolver proof before workflow dispatch. For the
   2026-07-04 cutover candidate, that packet is
   `promoted_from_tag=v6.0.0-rc.7`, `rollback_version=v5.1.35`,
   `ga_date=2026-07-04`, and `v5_eos_date=2026-10-02`.
   That stable cut must also move the repo-root Docker compose default and
   `scripts/install-docker.sh` fallback from the final RC image tag to the
   stable `6.0.0` image tag in the same commit as `VERSION=6.0.0`.
   Stable patch releases after `6.0.0` stay on this same governed release
   boundary but do not need a fabricated same-version RC tag for a routine
   patch. `resolve_release_promotion.py` owns the machine boundary: the
   rollback target must be the latest preceding stable tag, the candidate must
   descend from it, no same-version RC may already exist, and the diff may not
   touch authentication/tenant isolation, licensing/billing authority,
   persisted-data/schema migration, relay/mobile trust protocol, or
   installer/updater/rollback execution. Those risk classes require exercised
   RC lineage unless active customer harm is recorded with
   `hotfix_exception=true` and a non-empty `hotfix_reason`. First-GA and minor
   stable promotions still require explicit promoted prerelease lineage and
   soak proof. Stable patch release
   packets must also enumerate every customer-visible support fix included in
   the cut, and the release-asset proof must pin the current packet to those
   runtime fixes so a patch that includes support work cannot ship as a
   metadata-only release note.
   Release integration failures must leave enough evidence to classify the
   failure after the compose stack is torn down. `create-release.yml` must
   upload the Playwright report and a `release-integration-failures` artifact
   containing Playwright `test-results/` plus
   `release-integration-diagnostics/docker.log`; that Docker log must capture
   container state and the Pulse test server plus mock GitHub server logs.
7. Preserve release-matched installer and Helm operator documentation links through `scripts/install.sh`, `.github/workflows/helm-pages.yml`, `.github/workflows/publish-helm-chart.yml`, and the chart metadata itself so deployment guidance and packaged chart metadata do not drift back to branch-tip `main` docs when a release line or promoted tag already exists.
   The same governed Helm boundary also owns `deploy/helm/pulse/` itself:
   chart metadata, default values, templates, and generated chart docs must
   stay on the validated release line rather than mutating `main` or packaging
   from whatever branch GitHub happened to check out.
   The chart's `agent.enabled=true` workload must point at an image that is
   actually published. The default `agent.image.repository` must be the main
   `rcourtman/pulse` image (which is the only image `publish-docker.yml`
   pushes); the agent template must override the server ENTRYPOINT via
   `agent.command` so the pod runs as a unified agent; and the runtime stage
   of `Dockerfile` must ship an arch-resolved `/usr/local/bin/pulse-agent`
   symlink that picks `pulse-agent-linux-{amd64,arm64,armv7}` per `TARGETARCH`
   so a single command default works across multi-arch nodes. The
   never-published `ghcr.io/rcourtman/pulse-agent` is forbidden as a chart
   default. `scripts/validate-release.sh` must assert the
   `/usr/local/bin/pulse-agent` symlink exists, points at one of the
   supported Linux arch binaries, and is executable in the published image.
   `create-release.yml` must trigger `publish-helm-chart.yml` via an explicit
   `workflow_call` after `validate_release_assets` succeeds, not rely on
   GitHub's `release: published` webhook. The webhook does not fire when a
   release is created as draft and later PATCHed to `draft=false` (the path
   `create-release.yml` uses for draft validation), so without the explicit
   call the chart silently never publishes — v6 rc.1 through rc.5 all
   shipped without any chart on the GitHub Pages helm index. The
   `publish-helm-chart.yml` workflow must therefore expose a `workflow_call`
   input schema (`chart_version`, `app_version`) alongside the legacy
   `release` and `workflow_dispatch` triggers, and its chart-version
   resolver must prefer inputs over the release-event tag when inputs are
   present so all three entry paths converge on the same identity.
   `helm-pages.yml` must not treat chart-releaser's "no chart changes
   detected" no-op as a successful Pages publication for a newly published
   release version. A successful Pages workflow must create or update the
   `helm-chart-<version>` release asset and assert that `gh-pages/index.yaml`
   contains `version: <version>` before the workflow exits green.
   After pushing the OCI chart, `publish-helm-chart.yml` must prove the
   pushed chart is readable from GHCR without registry credentials by logging
   out of `ghcr.io` and running `helm show chart` against the versioned chart
   reference. The workflow must not mask package-visibility drift with
   best-effort GitHub Packages visibility API calls: invalid or unauthorized
   visibility endpoints create false success and noisy release logs, while the
   unauthenticated chart read is the customer-facing availability contract.
   `create-release.yml` must apply the same explicit `workflow_call` to
   `promote-floating-tags.yml`. Its legacy `workflow_run` chain off
   `publish-docker.yml` silently stops promoting `latest` / major / minor
   tags whenever `publish-docker.yml` fails (rc.3 → rc.5 all failed at the
   removed pulse-agent push step), leaving customers on stale floating
   tags with no warning. `promote-floating-tags.yml` must expose
   `workflow_call` inputs (`tag`, `prerelease`) and its tag resolver must
   prefer those over the workflow_run-derived tag, and the create-release
   wiring must gate on `validate_release_assets` succeeding so the docker
   image is guaranteed pullable before promotion.
   Generated chart docs are part of the packaged release artifact, not a
   disposable byproduct: when the stable candidate version changes, the checked
   in `deploy/helm/pulse/README.md` output must be regenerated from the same
   chart metadata and release line so published Helm docs, chart version
   badges, and packaged archive metadata all describe the identical cut.
   Chart monitoring surfaces must only expose metrics emitted by the shipped
   runtime. Retired Pulse Assistant explore-prepass metrics, values, schema
   entries, README rows, and PrometheusRule templates must not remain in
   `deploy/helm/pulse/` after interactive Assistant chat routes directly
   through the operator-selected model.
   External helper binaries fetched by governed release workflows are part of
   the same supply-chain boundary and must be checksum-verified before they are
   executed.
   Release-grade Go builds must use `scripts/release_ldflags.sh` as the
   canonical source for embedded version, commit, license, and update trust-root
   identity, and must disable Go's automatic VCS stamping with
   `-buildvcs=false` in `scripts/build-release.sh`, `Dockerfile`, and the demo
   deployment build so generated frontend or release-packet files cannot leak a
   misleading dirty-tree marker into published binary metadata.
   Release validation must prove that installer script download endpoints return
   signature headers, and unified-agent download endpoints must return checksum and signature headers whose checksum value matches the served binary.
8. Add or change the non-secret Pulse Cloud public signup route smoke through
   `scripts/run_cloud_public_signup_smoke.sh`. That smoke must prove either
   the open signup route contract or the intentionally closed redirect contract,
   and valid magic-link probes must remain opt-in so routine public checks do
   not send email accidentally.
9. Add or change operator-facing hosted tenant runtime canary rollout, tenant
   runtime container log-retention bounds, batch runtime contract
   reconciliation, canonical hosted route/public URL generation, or
   control-plane runtime-registry reconciliation through
   `cmd/pulse-control-plane/main.go`, `internal/cloudcp/docker/manager.go`,
   `internal/cloudcp/docker/labels.go`, and
   `internal/cloudcp/tenant_runtime_rollout.go`
   The batch reconcile command must be restorative as well as corrective:
   when a tenant registry row and tenant data remain but the canonical or
   recorded Docker container is missing, dry-run must classify the tenant for
   mutation and the live command must recreate the container, prove health, and
   rewrite the registry runtime identity through the same control-plane path.
10. Add or change the canonical hosted staging smoke operator path through `scripts/run_hosted_staging_smoke.sh`, `tests/integration/scripts/bootstrap-hosted-mobile-onboarding.mjs`, `tests/integration/scripts/hosted-mobile-token-runtime.mjs`, `tests/integration/scripts/hosted-tenant-approval-store.mjs`, `tests/integration/scripts/hosted-tenant-runtime.mjs`, `tests/integration/scripts/hosted-tenant-runtime-restart.mjs`, and `tests/integration/scripts/relay-mobile-token-helper.go`.
    Hosted mobile proof helpers must create and delete only disposable
    proof-shaped workspaces through the normal control-plane provisioner,
    fetch onboarding payloads without logging bearer tokens or mobile deep-link
    secrets, and seed hosted approvals through a single explicit tenant runtime
    restart when a release proof needs transactionally visible approval state.

## Forbidden Paths

1. Leaving deployment bootstrap, installer, or update-runtime files unowned under broad monitoring or generic API ownership
2. Duplicating deployment-type update planning, installer release resolution, or updater handoff behavior outside the canonical update engine and installer scripts
3. Treating update transport as payload-only contract work when it also defines live deployment and upgrade behavior

## Completion Obligations

1. Update this contract when canonical deployment or installer entry points move
2. Keep deployment runtime and shared API proof routing aligned in `registry.json`
3. Preserve explicit coverage for installer parity, update planning, and deployment bootstrap behavior when these surfaces change. Shell installer update recovery changes must keep `scripts/installtests/install_sh_test.go` covering both persisted `connection.env` recovery and legacy running-process/service recovery across Linux and FreeBSD/rc.d, including single-dash v5 agent flags, non-procfs process inspection, and the rule that upgraded service args use `--token-file` instead of raw `--token`.
4. Keep stable and prerelease packet lineage explicit when `docs/releases/` or
   `VERSION` changes: preserve already-shipped RC packets under dedicated
   historical filenames before reusing canonical stable names, keep
   `docs/RELEASE_NOTES.md` and `docs/UPGRADE_v6.md` coherent with that
   lineage, and prove the result through the release-promotion metadata path.
5. Keep paid Pro runtime packaging explicit whenever release runbooks, release
   packets, or paid-user GA guidance changes: public OSS release archives are
   not sufficient proof of paid self-hosted Pro readiness unless the matching
   `pulse-enterprise` Pro artifact/image path is built, identified, and linked
   for paid users. Support-only private Pro prerelease Docker images may be cut
   from an exact governed prerelease tag, but they must keep the explicit
   prerelease tag in the customer pull command and must not be treated as a
   stable, latest, R2, or download-page promotion.
6. Keep `deploy/helm/pulse/README.md` regenerated and release-matched whenever
   chart metadata or the governed release version changes so packaged Helm docs
   remain on the same validated cut as `Chart.yaml`.
7. Keep managed-runtime first-session helpers deterministic: shared browser
   helpers under `tests/integration/tests/helpers.ts` may only drive the live
   setup wizard through the current managed runtime after refreshing the
   canonical dev reset route, authenticated completion must expect the
   Infrastructure landing path rather than the retired `/dashboard` route, and
   any helper changes that rely on hot-dev browser/backend behavior must keep a
   managed-runtime recovery proof updated in the same slice.
   When those helpers complete first-run setup, they must preserve the API token
   emitted through the setup handoff and write it back to the managed runtime
   state before later authenticated entry attempts. Stale configured tokens may
   be discarded after backend auth failure, but reset and re-entry must still
   use backend-owned dev reset, admin-bypass, session-login, or token-auth paths
   instead of deleting runtime files, rebuilding bootstrap state, or accepting
   the retired dashboard route as proof of authentication.
   Release E2E suites that use those helpers must avoid turning scenario count
   into repeated password-login pressure: worker-scoped authenticated storage
   state is the canonical multi-scenario shape, and helper retry proof must
   preserve explicit 429 login-rate classification.
8. Keep root-level Playwright wrapper routing on the canonical managed browser
   truth. `playwright.config.ts`, `tests/integration/playwright.config.ts`,
   and `tests/integration/tests/runtime-defaults.ts` must resolve the same
   browser base URL precedence so repo-root browser proofs attach to the live
   managed hot-dev shell or runtime-state browser URL instead of silently
   falling back to the embedded `:7655` frontend when a managed browser shell
   is already the active truth. When both `PLAYWRIGHT_BASE_URL` and
   `PULSE_BASE_URL` are present, browser attachment must prefer
   `PLAYWRIGHT_BASE_URL` while backend-oriented setup and health helpers may
   still use `PULSE_BASE_URL`. That shared helper must also honor
   `PULSE_E2E_REPO_ROOT` for runtime-state and managed-session discovery so
   isolated verification harnesses can relocate managed runtime state without
   mutating the live repo root.
9. Keep hosted staging smoke fail-closed and repo-tracked. `scripts/run_hosted_staging_smoke.sh`
   and the hosted onboarding helpers under `tests/integration/scripts/` must
   require explicit target environment input, compose the canonical hosted
   signup/billing Playwright evals with the hosted mobile onboarding proof, and
   avoid implicit production defaults or lane-local shell fragments that bypass
   the checked-in proof pack.
10. Keep governed release, publish, and deployment automation supply-chain
   pinned. The canonical workflow surface under `.github/workflows/` must use
   immutable action SHAs, GitHub-hosted jobs must target an explicit Ubuntu LTS
   runner image instead of `ubuntu-latest`, and checked-in CI/test Dockerfiles
   under this subsystem must pin base images by immutable `@sha256` digest and
   must not depend on floating `:latest` base tags.
   Whenever that policy changes, update the owning workflow/install proof files
   in `scripts/installtests/build_release_assets_test.go` and
   `scripts/release_control/release_promotion_policy_*` in the same slice.
11. Keep mobile impact explicit on governed server releases. Every release
   publish and manual release dry run must record one of the canonical mobile
   decisions (`no-mobile-impact`, `existing-mobile-build-compatible`,
   `mobile-candidate-uploaded`, or `mobile-candidate-required`), and
   `mobile-candidate-required` is a blocking state until the mobile candidate
   is built/submitted and the release is rerun with `mobile-candidate-uploaded`
   evidence. Compatibility or uploaded-candidate decisions must carry evidence
   text rather than relying on memory. A `mobile-candidate-uploaded` release
   packet must also name the exact iOS build number and Android version code in
   its release notes and changelog, and must distinguish TestFlight or Play
   internal-testing availability from a public store rollout.
12. Keep forward release signing pinned to an explicit trust root. Governed
   release scripts, Docker release builds, and historical backfill paths must
   accept the active private signing key only alongside a non-secret expected
   public key or equivalent pinned identity, and they must fail closed before
   publication if the signer drifts from that expected trust root.
13. When the governed update signer changes, the canonical operator-facing
   release docs under `docs/releases/` and the governed upgrade guide
   `docs/UPGRADE_v6.md` must state the continuity impact explicitly. Those docs
   must not imply automatic updater continuity from a historical signer unless
   the actual trust-migration path is already shipped and exercised.

## Current State

Stable and stable-dry-run callers now select SignPath as the canonical Windows
Authenticode backend. The reusable builder fails fast on missing configuration,
submits the GitHub-hosted unsigned artifact through the pinned official action,
verifies all returned executables, and stores request/source/signer/digest
evidence beside the candidate manifest. Release Dry Run now has a terminal
verdict covering the signed candidate and no-mutation demo lane. The gate stays
blocked until the external SignPath project is configured and one stable dry
run passes for an exact `main` SHA.

The provider MSP proof command validates its handoff target with the same
host-local redirect contract as runtime token minting and exchange. Proof input
must reject absolute, scheme-relative, backslash-authority, encoded-separator,
and control-character targets before constructing the handoff request.

The active support prerelease `v6.1.0-rc.5` cut sets the repo-root `VERSION`,
repo-root `docker-compose.yml` image default, `scripts/install-docker.sh`
fallback, and Helm chart release metadata to the same `6.1.0-rc.5` release
version. This support prerelease keeps `rollback_version=v6.0.5`, publishes a
versioned public GitHub prerelease plus versioned Docker and Helm artifacts, and
does not move stable/latest install pointers or stable semver aliases. It puts
the expanded Pulse Intelligence action and verification lifecycle, the
operator-facing Actions inbox, monitor-first product workflows, governed host
and storage operations, native-agent update safety, Windows logged-readiness
and recovery proof, OIDC callback recovery, and fail-closed security hardening
behind RC validation before the next stable minor release. The second candidate
extends that cumulative scope with model-led Patrol qualification,
subscription-backed Claude transport, typed Docker update and restart recovery,
a governed commercial lifecycle, and additional fail-closed authentication and
installer hardening. The third candidate
carries release-candidate feedback fixes: host agent install tokens mint the
command-execution scope the operator asks for with first-use command-channel
binding, ZFS pool membership resolves nvme-eui and namespace-suffixed member
references, TrueNAS storage reads the served API shapes, Patrol findings can
notify through alert channels, and in-app updates select releases by highest
version.
The fourth candidate adds the canonical Operational Trust attention and action
loop, availability and protection posture across unified resources, more
reliable alert state transitions, report-only Unified Agent observer
destinations with destination-scoped transport policy, and release-candidate
feedback fixes across Assistant, storage, Docker, responsive tables, and
session continuity.
The fifth candidate sharpens infrastructure identity and fleet truth, moves
Agent Doctor into a routed, filterable diagnostic workflow with copyable
reports and platform-correct host-local cleanup handoff, adds SAS and SCSI SMART
coverage, restores actionable agent-update controls, fixes Patrol finding and
proposal provenance, improves metrics and audit-store concurrency, and prepares
the stable Windows SignPath signing path while retaining the governed
unsigned-Windows prerelease exception.
The rc.5 server cut is classified `no-mobile-impact`; no companion build upload
is part of this cut. The existing mobile candidate programme remains separate,
and the release packet must not describe a public store rollout.
The same release boundary now provides one canonical in-app release-note
experience. Update checks can preview a curated `Highlights` section, and an
authenticated running-version endpoint lets the update surface show those
same published highlights once after a later upgrade. Missing highlights stay
quiet by design, and source or development builds never masquerade as
published releases.
The initial GA promotion
metadata remains
`promoted_from_tag=v6.0.0-rc.7`, `rollback_version=v5.1.35`,
`ga_date=2026-07-04`, and `v5_eos_date=2026-10-02` for the first stable
`6.0.0` cut.

The shell-installer boundary carries root-agent service hardening for Linux
installs. Installer-rendered agent units must keep the health/metrics listener
loopback by default, allow explicit disablement or network-scrape opt-in
through `--health-addr` / `PULSE_HEALTH_ADDR`, and preserve conservative
systemd sandboxing alongside the root full-telemetry service model instead of
silently reopening an all-interface root HTTP listener.

Generated TrueNAS CORE rc.d services must use `/usr/sbin/daemon -r` with a
supervisor pidfile (`-P`) separate from the child pidfile (`-p`) and must stop
older child-pidfile installs by killing the daemon supervisor before the child
so installer upgrades do not leave the old agent process running.

This subsystem now makes deployment planning, updater orchestration, and the
non-shell installer/update scripts explicit inside the current self-hosted
release-confidence lane instead of leaving them as implied behavior around the
core runtime.

The canonical v6 upgrade guide now follows the free-first self-hosted GA
posture for install and support guidance: it describes activation, recovery,
and BYOK/local AI setup, while explicitly keeping general in-app trials,
trial-return callbacks, and hosted AI quickstart acquisition out of the
ordinary upgrade path.
That same upgrade guidance and the current shipped release notes must describe
v5-to-v6 agent upgrades through the current Infrastructure install surface:
`Settings → Infrastructure → Install on a host` is the supported path for both
first installs and in-place agent upgrades, and v6 may only show agent
version/status details after the upgraded agent authenticates and sends a fresh
report rather than from an offline inventory of pre-upgrade v5 services.

That same release-confidence lane now also owns the shipped Helm chart path,
so release automation, packaged chart metadata, and chart-runtime smoke no
longer depend on unowned `deploy/helm/pulse/` files while the governed
release workflows package and publish those artifacts.

That same lane also owns version-pinned Docker bootstrap defaults. The repo
root `docker-compose.yml` sample and `scripts/install-docker.sh` must default
to the governed `VERSION` cut instead of floating `:latest`, so self-hosted
operators only move to a newer image when they choose a newer explicit tag or
override `PULSE_IMAGE`.
For every RC or stable release cut, those Docker defaults must move with the
same governed `VERSION` change and the installer proof in
`scripts/installtests/install_docker_sh_test.go` and
`scripts/installtests/build_release_assets_test.go` must assert the repo-root
compose image default, standalone installer fallback constant, and packaged
Helm metadata. A draft release workflow failure caused by stale image or chart
pins is a release-packet blocker until the defaults, tests, and evidence
record are refreshed from the new branch head.
For the active support prerelease `v6.1.0-rc.5` cut, the repo-root compose
default and `scripts/install-docker.sh` fallback must both pin `6.1.0-rc.5`
until the next governed stable cut moves them forward. The stable promotion
guard remains in force and must reject leftover `-rc.` defaults when the
governed `VERSION` returns to a stable release.
The RC7 packet refresh records `fc10de9b5477613316473267b72b05b6b2b7aaff`
as the current validation-risk commit. That head includes the earlier
Docker-default correction plus the follow-on capacity-forecast and Patrol
history hardening commits. Later metadata-only packet refreshes may be the
workflow dispatch head only when they do not change the code-backed
release-risk range.

`internal/updates/` is the live deployment and upgrade planner. It owns
deployment-type detection, update-plan generation, adapter selection, server
update sequencing, and rollback-aware update state for supported Pulse
deployments.
That same boundary also owns the canonical running-version contract for
release binaries: `internal/updates/version.go` must prefer the build-injected
version string provided by the runtime entrypoint over git or working-tree
fallbacks, so shipped release builds report the exact promoted version even
when the install path has no `.git` metadata or a stale `VERSION` file nearby.
Runtime bootstrap must seed that build version before the server starts rather
than leaving version detection to deployment-local filesystem guesses.
That same version boundary now also owns the working-line development base:
the checked-in `VERSION` file is the canonical intended semver base for
current v6 development, and source/dev runtime detection must append git build
metadata to that base instead of inheriting accidental prerelease tag lineage
from `git describe`. Non-published prerelease bases such as `6.0.0-dev`
therefore remain prerelease for branch policy, release-control blocked
records, and future release promotion planning, but they must not be treated
as shipped RC lineage or as published release-asset versions.
That same version boundary now also owns the canonical usage-data release
identity. `internal/updates/version.go` must classify raw runtime version
strings into normalized release identity fields for browser preview payloads
and operator telemetry reporting, so unpublished `git describe` / manual / dev
builds cannot pollute published stable or RC adoption reads just because they
share a semver-looking prefix.
That release-build metadata path is now explicit too: `scripts/release_ldflags.sh`
is the canonical owner for server and agent build ldflags, and release artifact
assembly must route through it instead of hand-writing overlapping `main.Version`,
`internal/updates.BuildVersion`, `internal/dockeragent.Version`, or license-key
injection fragments across `scripts/build-release.sh`, `Dockerfile`, and the
demo-deploy workflow. Shipped binaries, installable container images, and
governed deployment-build workflows must all carry the same build metadata
contract rather than depending on whichever local ldflags string happened to be
updated last.
That same governed release lineage now also owns artifact attestation and
secret-safe container builds. Release workflows must publish max-level image
provenance plus SBOM attestations, push keyless GitHub/Sigstore attestations
for the published server and agent images, attest the generated release packet
assets from the `release/` directory, and pass the embedded license public key
through BuildKit secret mounts instead of Docker build arguments so release
metadata and image history cannot re-expose it.
Because BuildKit secret contents are intentionally excluded from layer cache
keys, those Docker builds must also pass a non-secret SHA-256 fingerprint of
the mounted license public key through `PULSE_LICENSE_PUBLIC_KEY_SHA256` and
the `Dockerfile` must verify that fingerprint before embedding the key. A
release image build must fail closed if the fingerprint is present but the
secret is missing, malformed, or mismatched, so cached no-key binaries cannot
be reused for release-grade hosted or self-hosted runtime images. The matching
installability proof lives in `scripts/installtests/build_release_assets_test.go`
and `scripts/release_control/release_promotion_policy_test.py`, and both must
assert the secret mount and non-secret fingerprint argument together.
The standalone hosted control-plane image is part of the same release-license
boundary. `deploy/provider-msp/Dockerfile.control-plane` must build
`cmd/pulse-control-plane` with `-tags release`, canonical
`scripts/release_ldflags.sh server` metadata, an embedded license public key
from the BuildKit `pulse_license_public_key` secret, and the same
`PULSE_LICENSE_PUBLIC_KEY_SHA256` fingerprint gate. Provider-hosted MSP uses
that control-plane image for signed MSP-license enforcement, so it must not be
possible to publish a provider MSP control-plane image that accepts
`PULSE_LICENSE_DEV_MODE` or `PULSE_LICENSE_PUBLIC_KEY` runtime overrides.
`.github/workflows/publish-docker.yml` must publish and attest
`rcourtman/pulse-control-plane` and
`ghcr.io/<owner>/pulse-control-plane` from that Dockerfile, with the same
version tags and prerelease/latest tag policy as the main Pulse runtime image.
That same supply-chain boundary also owns the checked-in build roots
themselves. `Dockerfile` must pin its Node, Go, and Alpine bases by immutable
manifest-list digest so multi-arch release builds do not silently drift onto a
different upstream filesystem just because a mutable tag was republished.
The governed v6 release Go patch level is part of that same boundary:
`go.mod`, `scripts/.go-version`, `scripts/install-go-toolchain.sh`,
`scripts/build-release.sh`, the Go builder stages in `Dockerfile` and
`deploy/provider-msp/Dockerfile.control-plane`, and the Pro release workflows
must stay aligned on the same patched `1.26.x` floor before a release can be
treated as shippable. When `govulncheck` reports called standard-library
vulnerabilities in the current patch level, the canonical fix is to advance the
governed release toolchain and immutable Go builder digest together, not to
suppress the scanner or produce release artifacts with an older patched-over
runtime.
That same dev-runtime dependency-manifest boundary now also owns the maintained
Docker engine module floor. `go.mod`, `go.sum`, and
`internal/cloudcp/docker/manager.go` must route hosted runtime orchestration
through the maintained `github.com/moby/moby/api` and
`github.com/moby/moby/client` modules instead of reviving the legacy
`github.com/docker/docker` line, so managed-runtime manifests and hosted
runtime rollout control do not silently inherit an unfixed Docker SDK
advisory path.
That same Docker release-build boundary also owns the embedded frontend's
shipped-doc inputs and the Docker context allowlist that makes those files
available to release builds in the first place. When the frontend embed build
syncs public docs from the repo root, `Dockerfile` and `.dockerignore` must
jointly stage the canonical shipped docs set into the container build context
before `npm run build` runs, rather than relying on a workstation-local
checkout layout or leaving hosted runtime image builds unable to resolve
`/app/docs/*.md`, `SECURITY.md`, or `TERMS.md`.
That same Docker build graph must keep hosted tenant runtime images separate
from release-installer assembly. `Dockerfile` must expose a `hosted_runtime`
target derived from the shared Pulse server runtime base that copies only the
server runtime assets and does not depend on rendered installers, embedded
agent binaries, or installer signing material. The published self-hosted
`runtime` and `agent_runtime` targets must keep using the release-assets stage
so official release images still carry signed installer and agent download
assets, and any build that declares `PULSE_UPDATE_SIGNING_PUBLIC_KEY` must
continue to fail closed unless the matching signing secret is mounted.
That same update-runtime boundary now also owns bounded rollback retention and
disk-space fail-closed behavior for self-hosted app updates. `internal/updates/`
must prune stale retained rollback snapshots, clear history references when an
old snapshot ages out of retention, choose a backup root with enough free
space, and reject extraction/backup work early with a concrete space error
instead of drifting into partial update failure on small LXC or single-disk
installs.
The root server installer shares that same fail-closed update-space boundary:
`install.sh` must preflight staging and install-directory filesystem headroom
before it stops the running Pulse service or downloads/applies a release
archive, combining the required headroom when `/tmp` and the install directory
share one filesystem. The install.sh update adapter must advertise that same
operator prerequisite so in-app updates do not understate the staging
requirement.
The same governed promotion path must now stay explicit too:
`scripts/release_control/resolve_release_promotion.py` is the canonical owner
for stable-versus-prerelease metadata validation shared by `.github/workflows/release-dry-run.yml`
and `.github/workflows/create-release.yml`. Promotion rollback targets, promoted
prerelease lineage, soak checks, and GA/v5 notice metadata may not drift between those
two workflows through duplicated inline shell validation.
One scoped exception keeps the weekly drift watchdog viable: scheduled
`release-dry-run.yml` runs carry no `workflow_dispatch` inputs (GitHub does
not apply input defaults to `schedule` events), so the rehearsal step passes
`--derive-rollback-latest-stable` and the resolver fills the empty
`rollback_version` with the latest stable repository tag preceding the
rehearsal version. The derivation flag is gated on the `schedule` event in
the workflow; manual rehearsal dispatches and real promotions must still
supply an explicit stable `rollback_version`, and the resolver still fails
closed when the input is empty and the flag is absent.
`scripts/release_control/validate_artifact_release_line.py` is the canonical
owner for follow-on artifact workflow release-line validation shared by Docker
publication, floating-tag promotion, Helm chart publication, and Helm Pages
publication. It must keep first stable/minor promotions tied to matching
prerelease lineage while allowing stable patch tags to follow the previous
stable tag without fabricating same-version RC tags.
That same promotion-governance boundary also owns the release-dispatch helpers
and artifact follow-on workflows that consume those same decisions. Demo
deployment, Docker publication, Helm chart publication, Helm Pages release, and
the manual `trigger-release*.sh` entrypoints must all derive their governed
release line from control-plane metadata before they touch public artifacts or
deployment targets, rather than treating tag names or workflow triggers as
enough proof on their own.
For routine stable patches, `scripts/trigger-stable-patch.sh` is the
noninteractive operator path. It derives the latest stable rollback, consumes
the canonical `docs/releases/RELEASE_NOTES_vX.Y.Z.md` packet, infers
`no-mobile-impact` only when no mobile-facing path changed, and dispatches one
dry-run or publish workflow. `create-release.yml` must independently require a
successful `workflow_dispatch` `Release Dry Run` for the exact candidate SHA
and version from the previous 24 hours, so the UI and alternate helpers cannot
bypass the preflight. That dry run must call `update-demo-server.yml` in
verification-only mode against the latest stable release. It must prove
Tailscale, SSH host identity, runtime version, frontend parity, public health,
and browser smoke without changing the host.
That same release-validation boundary also owns draft-versus-published asset
state. When `.github/workflows/create-release.yml` runs in `draft_only` mode,
it must pass the real draft state into `.github/workflows/validate-release-assets.yml`
so validation blocks or annotates the draft release as a draft, rather than
misclassifying the run as post-publish revalidation.
That same reusable-validation call boundary also owns permission handoff.
`.github/workflows/create-release.yml` must explicitly grant the nested
`.github/workflows/validate-release-assets.yml` call the write scopes it
requests (`contents: write` and `issues: write`), rather than inheriting the
release pipeline's top-level read-only default and failing at workflow startup.
That same validation status boundary must preserve release identity when it
annotates a draft or failed release. Every release-body or draft-state PATCH
from `.github/workflows/validate-release-assets.yml` must carry the intended
`tag_name` and `target_commitish`, then verify the API response still matches
those values, so validation status updates cannot detach a draft release back
onto GitHub's generated `untagged-*` placeholder.
That same governed release boundary also owns unpublished draft retry
reconciliation. Re-running `.github/workflows/create-release.yml` for the same
unpublished tag must locate the existing draft release, retarget its git tag
and release `target_commitish` to the current governed release-line head, and
continue publication without requiring an operator to delete the tag manually;
published tags remain immutable and must still fail closed.
That same release-dispatch boundary now also owns mobile impact gating for
server releases. `.github/workflows/create-release.yml`,
`.github/workflows/release-dry-run.yml`, `scripts/trigger-release.sh`, and
`scripts/trigger-release-dry-run.sh` must require an explicit mobile release
decision before a governed release packet can proceed. A server-only release may
record `no-mobile-impact`; a mobile/relay/onboarding/API-compatible release may
record `existing-mobile-build-compatible` with proof; a release that already
has a matching TestFlight/Play candidate may record `mobile-candidate-uploaded`
with build evidence; and `mobile-candidate-required` must fail closed until the
mobile candidate exists. This gate does not auto-submit App Store/TestFlight or
Play builds, but it prevents release packets from silently ignoring the mobile
track.
That same upload boundary must tolerate transient GitHub release-asset API
failures. `.github/workflows/create-release.yml` must retry every
`gh release upload` operation with bounded backoff before failing the release
job, because a single 5xx response during upload can otherwise strand a draft
release with a partial asset set and no validation run.
That same public release-body boundary also owns publish-safe packet rendering.
When operators pass draft packet markdown to `.github/workflows/create-release.yml`,
the workflow must sanitize draft-only framing and append the standardized
installation and promotion metadata sections exactly once, rather than trusting
raw packet text to already be publish-safe.
Release-note transport is file-backed and fail-closed: operator helpers must
send the Markdown through JSON input rather than multiline form-field
substitution, the renderer must reject missing standalone title/section
structure before any tag or draft mutation, and draft creation must compare
GitHub's stored body with the exact rendered file before asset upload.
`validate-release-assets.yml` must repeat the structural check before validating
assets, preserve the authored body through validation-status edits, and compare
the API response with the pre-edit body. A malformed edited body is quarantined
as a draft without deleting otherwise valid assets.
That same frontend-release boundary also owns shared header-composition proof.
`.github/workflows/release-dry-run.yml` and `.github/workflows/create-release.yml`
must both run the same `lint:headers` audit so a branch that would be rejected
by the real publish workflow cannot pass the governed dry run only because the
rehearsal skipped that header-composition gate.
That same dry-run backend gate must run non-race Go package tests serially with
`go test -p 1 ./...` so release SLO proof reflects product behavior rather
than cross-package contention on 2-core hosted runners.
That same governed demo-deployment boundary now owns the post-GA single-demo
contract. `.github/workflows/create-release.yml`,
`.github/workflows/update-demo-server.yml`, and `.github/workflows/deploy-demo-server.yml`
must treat `demo-stable` as the only active public demo target: stable releases
may update it, prerelease tags must not create or update a second public v6
preview target by default, and any future preview surface requires a new
explicitly governed target instead of reusing the retired v6-preview path.
That same demo deployment boundary also owns service-identity and public-shell
parity proof. Stable demo runs default to the `pulse` service identity, must
prove that the SSH target reports the governed expected hostname before any
installer or binary copy runs, and demo deploy/update verification must prove
that the public demo HTML serves the same frontend entry asset as the target
service or freshly built artifact rather than treating a passing `/api/health`
response as enough evidence that the public shell actually updated. That proof
must use a deterministic HTML parser for the actual module entry script rather
than brittle escaped shell regex or a first-match asset scrape that can fail
differently over SSH or select the wrong preloaded chunk.
Those same governed demo deploy/update workflows also own the runner-to-host
network path. They must establish the canonical Tailscale connectivity step
before SSH setup so stable or preview targets may stay on governed private
hostnames or Tailscale IPs, rather than silently depending on public SSH
reachability from GitHub-hosted runners. The workflows must use the current
pinned Tailscale GitHub Action, its target `ping` readiness gate, and the shared
`.github/scripts/check-demo-reachability.sh` TCP/22 diagnostic before SSH key
capture. A successful tailnet join alone is not connectivity proof. After that
network preflight, shared SSH
setup must wait for configured demo hostnames to resolve, accept configured IP
literals without a DNS precheck, and then capture host keys with bounded
short retries before any installer or binary copy runs; a long `ssh-keyscan`
loop must not hide an ACL, peer-propagation, firewall, or sshd failure.
`create-release.yml` must call the update workflow as an awaited reusable job,
and its terminal `Definitive Release Verdict` must require stable demo runtime,
frontend, public health, and browser proof. An asynchronous dispatch or manual
SSH deployment is not release completion. A one-shot `ssh-keyscan`
against a private demo target is not sufficient release or deploy proof.
Those same workflows also own customer-visible browser truth for the public
demo shell. Health checks and entry-asset parity are necessary but not
sufficient; after those checks pass, the governed helpers
`scripts/run_demo_public_browser_smoke.sh` and
`scripts/demo_public_browser_smoke.cjs` must exercise the public demo in a real
Chromium session and prove the login shell actually renders instead of failing
open on API-only reachability. That proof must treat the visible login controls
as the readiness signal and must not block on Playwright `networkidle`, because
the public demo shell can keep background activity alive after the page is
already usable.
That same demo-update verification boundary also owns the canonical v6 mock
runtime state contract. `.github/workflows/update-demo-server.yml` must verify
mock-mode readiness through the unified `/api/state` `resources[]` collection,
not legacy `nodes`, because v6 intentionally strips per-type arrays from the
state payload.
That same stable demo-update boundary must also restore the canonical demo
runtime `.env` before verification on every run, including when the service is
already on the requested version and the binary update is skipped. The workflow
must set `DEMO_MODE=true`, converge the governed `PULSE_MOCK_*` fixture
defaults in the service's resolved runtime `.env`, seed the hidden
`demo_fixtures` capability into the default-org demo `billing.json` entitlement
state, restart the selected service, force the release-build demo-fixture
entitlement sync through authenticated `/api/license/runtime-capabilities`, and
then prove both
`/api/system/mock-mode.enabled` and `/api/state.resources[]` converge. A
passing `/api/version` or `/api/health` response alone is not demo readiness.
If the workflow mutates an existing demo `billing.json`, it must remove the old
`integrity` field so the running application re-signs the entitlement state
through the canonical billing-state migration path instead of silently treating
the privileged deployment mutation as tampering.
That same operator-proof boundary also now owns the canonical hosted staging
smoke entrypoint. `scripts/run_hosted_staging_smoke.sh` must stay as the
repo-tracked operator command that composes the hosted signup/billing eval pack
with the hosted mobile onboarding bootstrap helpers under
`tests/integration/scripts/`, and those helpers must fail closed onto explicit
target cloud host and control-plane URL input instead of silently defaulting to
production infrastructure. When the operator does not pin
`PULSE_E2E_HOSTED_TENANT_ID`, that entrypoint may auto-select the newest active
tenant exposed by the authenticated `/admin/tenants?state=active` control-plane
view, but it must still fail closed when no active tenant is available.
Those same governed release workflows also own the operator-facing wording for
that promotion metadata. Human-visible workflow inputs, summaries, and error
messages must describe the path as a prerelease or preview flow rather than
implying a near-ready release candidate, while machine-owned identifiers such
as `rc`, `rc-to-ga-*`, and `v6.0.0-rc.1` remain the canonical internal keys.
That same downstream-dispatch boundary also owns release-ref fidelity. When
`.github/workflows/create-release.yml` fans out to governed post-publish
workflows such as Docker publication or demo updates, it must dispatch those
workflows on `needs.prepare.outputs.required_branch` rather than GitHub's
default-branch workflow definition, so prerelease automation cannot silently
fall back onto stale `main`-branch inputs or older demo verification logic.
That same release-fidelity boundary also owns governed Helm publication. The
Helm release workflows must derive the owning branch from the target version via
`control_plane.py --branch-for-version` before any chart mutation or packaging,
must check out either that governed release branch or the validated release tag
before touching chart contents, and must never hardcode `main` as the push or
package source for prerelease Helm publication.
Pre-publication release proof and post-publication chart publication have
different trust jobs and must stay that way: `.github/workflows/create-release.yml`
must smoke the Helm chart against a locally built release-line image before the
tag is published, while `.github/workflows/helm-pages.yml` must continue
smoking the immutable published tag image so chart publication cannot silently
pass on branch-only fixes that never made it into the released artifact.
That same promotion-governance package also owns the dated rehearsal-record
materialization path. The public recorder
`scripts/release_control/record_rc_to_ga_rehearsal.py` and its internal module
must remain the canonical route from a `Release Dry Run` run ID or summary
artifact to `docs/release-control/v6/internal/records/`, and they must fail
closed on missing artifact metadata or silent record overwrites rather than
encouraging hand-written repair of governed promotion fields.
That same operator packet boundary also owns the exact stable-promotion command
sequence and public self-hosted GA flip or rollback packet. The canonical
commands for `trigger-release-dry-run.sh`, rehearsal-record materialization,
preview public deploy or audit, production public deploy or audit, and rollback
back to the approved v5 posture must live in the governed release docs rather
than only in chat, tickets, or operator memory.
That same prerelease framing requirement also applies to installer and update
runtime copy: `install.sh`, `scripts/pulse-auto-update.sh`, and
`internal/updates/manager.go` must present `rc`-tagged builds as prerelease or
preview paths in menus, CLI help text, operator diagnostics, and runtime logs
rather than as release-candidate promises.
That same update-check boundary also owns release selection order: because
v5-line maintenance releases interleave with v6 releases in the same GitHub
repo (v5.1.36 was created the day before v6.0.5), `internal/updates/manager.go`
— both the release-list path and the Atom feed fallback — and
`scripts/pulse-auto-update.sh` must select the highest parseable version
eligible for the channel rather than trusting GitHub's `created_at` ordering
or the `/releases/latest` pointer, so a more recently created lower-version
release can never mask the newest stable and strand installs until the next
release ships. Stable-channel selection must keep excluding draft releases,
metadata-flagged prereleases, and prerelease- or non-semver-shaped tags
(`helm-chart-*`), and the `rc` channel must keep offering the newest stable to
prerelease installs so an rc line that lands on stable (6.0.0-rc.x → 6.0.5)
moves its installs forward instead of stranding them. Proof:
`internal/updates/manager_stranded_upgrade_test.go` and
`scripts/installtests/pulse_auto_update_test.go`
(`TestGetLatestStableVersionPrefersHighestVersion`).
Those same workflows must also fetch and dispatch the governed release branch
derived from release-control metadata instead of hardcoding `pulse/v6`,
`pulse/v6-release`, `main`, or any later branch literal inline; when a stable
maintenance line such as `5.1.x` remains live after the active profile has
moved on, that branch routing must come from an explicit control-plane release
line override instead of being guessed inside the workflow.
That same branch-policy contract must survive step boundaries inside the
workflows themselves: `.github/workflows/create-release.yml` and
`.github/workflows/release-dry-run.yml` must pass the resolved
`steps.branch_policy.outputs.required_branch` value into the promotion-policy
validation step environment before that step fetches refs or invokes
`resolve_release_promotion.py`, rather than assuming a shell-local
`REQUIRED_BRANCH` variable still exists from an earlier step.
That same `internal/updates/` boundary now also owns runtime data-dir
authority for temp, backup, and cleanup behavior: `manager.go` must resolve
its working directories through the shared runtime data-dir helper instead of
rebuilding `PULSE_DATA_DIR` plus `/etc/pulse` fallback logic inside each
update stage.
That same boundary also owns outbound update transport safety: env-configured
update server bases must normalize to absolute HTTP(S) URLs without userinfo,
and release API, feed, download, and checksum requests must resolve from
validated URL objects instead of raw string concatenation or request creation
from unchecked inputs. `ApplyUpdate` must canonicalize the supplied download
URL through that shared validator before version inference, history emission,
or transfer work begins.
That same boundary also governs owned filesystem scans inside the update
manager: when `internal/updates/manager.go` enumerates already-owned extract,
temp, backup, or restore directories, it must rejoin discovered entry names
through the shared storage-path helper instead of rebuilding raw
`filepath.Join(dir, entry.Name())` paths.
That same storage boundary also governs update-history persistence:
`internal/updates/history.go` must normalize its owned data directory and
resolve the fixed `update-history.jsonl` leaf through the shared storage-path
helper instead of joining raw caller-provided directory strings.
That same runtime env contract also governs `pulse mock`: the CLI may not keep
writing a separate `mock.env` sidecar when supported runtime installs already
carry mock-mode ownership through `.env`. Mock enable/disable/status must use
the canonical runtime `.env` path, with any install-dir `.env` probe treated as
compatibility only.
`scripts/toggle-mock.sh` is under the same rule: it may read legacy
`mock.env` sidecars only to migrate existing local demo settings into the
canonical runtime `.env`, but mode changes must write canonical `.env` files
only and must not recreate root, dev, or runtime `mock.env` sidecars.
That same dev-runtime boundary also owns the default mock density used for
local demos. `scripts/toggle-mock.sh` must seed the same `PULSE_MOCK_*`
defaults as `internal/mock.DefaultConfig`, so managed runtime toggles, local
demo restarts, and CLI mock status all converge on one canonical dataset
instead of drifting across shell helpers.
The canonical mock density today targets a mature small-to-mid homelab /
SMB environment so platform-first pages exercise real table density on
first boot. Both `mock_default_entries()` in `scripts/toggle-mock.sh`
and `internal/mock.DefaultConfig` carry the same baseline: 5 Proxmox
nodes with 6 VMs and 8 LXCs each, 5 Docker hosts with 14 containers
each, 4 standalone Pulse-managed hosts, and 3 Kubernetes clusters
(production + staging + edge) with 5 nodes, 40 pods, and 14
deployments each. Bumping either side requires bumping the other (and
the matching `scripts/tests/test-toggle-mock.sh` fixtures) so toggle
CLIs, managed runtime restarts, and the in-binary default never drift
apart.
Mock toggles are runtime transitions, not just environment-file edits. A
successful `scripts/toggle-mock.sh on|off` run must leave the managed browser
entrypoint serving the requested `/api/system/mock-mode` state through the
frontend proxy before the command reports success. When macOS launchd or
another managed wrapper hands ownership to a replacement `hot-dev-bg.sh supervise`
process during the restart, `scripts/hot-dev-bg.sh` must adopt the healthy
supervisor, refresh the managed pid file, and continue instead of surfacing a
false startup failure. `scripts/toggle-mock.sh` may only continue after a
non-clean managed restart when that browser-entrypoint proof confirms the
requested mock state; otherwise it must fail explicitly or use an intentional
fallback.
That same hosted runtime rollout boundary also owns public routing identity for
managed tenants. `internal/cloudcp/docker/labels.go`,
`internal/cloudcp/docker/manager.go`, and
`internal/cloudcp/tenant_runtime_rollout.go` must derive one canonical hosted
route host and `PULSE_PUBLIC_URL`, keep that addressing lowercase-safe for
mixed-case tenant IDs, and treat same-image runtime routing drift as rollout
drift that requires a canonical recreate rather than silently reconciling the
registry against stale Traefik labels or runtime env.
That same operator boundary also owns fleet remediation of that runtime
contract. `cmd/pulse-control-plane/main.go` and
`internal/cloudcp/tenant_runtime_rollout.go` must expose one canonical
batch-reconcile path that preserves each tenant runtime's current image line,
supports dry-run planning before mutation, and converges existing hosted
tenants onto the canonical runtime contract without relying on ad hoc host
scripts or one-off manual tenant loops.
That same hosted runtime container boundary owns startup ownership repair:
entrypoints may repair writable runtime data paths, but must not recursively
`chown` immutable image paths such as `/app` or `/opt/pulse`, because overlayfs
copy-up makes every tenant recreate consume image-sized writable disk.
That same rule applies to live runtime behavior too: config loading and reload
watching may not treat `mock.env` as a parallel primary-path control surface.
Supported mock-mode runtime state must come from the canonical `.env` contract,
and `.env` reload handling must own `PULSE_MOCK_*` updates and monitor reload
triggers directly.

The shell installer, Windows installer, container-agent installer, and
unattended auto-update scripts are part of the same runtime boundary, not just
release artifacts. `scripts/install.sh`, `scripts/install.ps1`,
`scripts/install-container-agent.sh`, and `scripts/pulse-auto-update.sh`
define supported deployment entry points and update behavior, even when the
shell and Windows installers also sit on the shared agent-lifecycle boundary.

`scripts/install-mcp.sh` and `scripts/install-mcp.ps1` extend the
installer family with a fourth entry point: a stdio MCP server
adapter (`cmd/pulse-mcp/`) that integrators run on their own
machine to expose the same Pulse Intelligence capability manifest
used by Pulse Assistant to OpenCode, Claude Desktop, Claude Code, or
other MCP-speaking clients. The installers fetch a published
`pulse-mcp-<os>-<arch>` binary from the latest GitHub Release,
verify SHA256 against the same `checksums.txt` the rest of the
release uses, and place the binary at `~/.local/bin/pulse-mcp`
(Unix) or `$LOCALAPPDATA\pulse-mcp\pulse-mcp.exe` (Windows). The
binary takes no version ldflags because it reads the manifest
from the Pulse instance it points at. `scripts/build-release.sh`
builds `pulse-mcp` for the same multi-OS matrix as the unified
agent (linux-amd64/arm64/armv7/armv6/386, darwin-amd64/arm64,
freebsd-amd64/arm64, windows-amd64/arm64/386), packages
per-platform tarballs and zips into `RELEASE_DIR`, and the
`.github/workflows/create-release.yml` upload step attaches both
the bare binaries and the install scripts as release assets so
`https://github.com/rcourtman/Pulse/releases/latest/download/pulse-mcp-<os>-<arch>`
and `.../install-mcp.sh` are stable redirect targets the
installers consume. macOS notarization is intentionally skipped
for v1: the README documents the Gatekeeper bypass and the
install-script flow downloads the same unsigned binary, with the
audit trail of SHA256 verification preserved.
The adapter's complete request/response tool-list projection, manifest
projection, capability and governance metadata formatting, request/response
tool filtering, typed input-schema projection, and API route/body call
projection semantics remain owned by `ai-runtime` and `api-contracts`;
deployment-installability owns building, publishing, installing, and launching
the same binary. README guidance may describe the manifest-provided typed
`inputSchema` arguments that MCP clients receive, including operator-state,
finding, and action tools, but those schemas remain an API/AI contract rather
than an installer or release-asset behavior.
README and startup guidance may describe API-token setup for the installed
adapter, but the set and order of advertised token scopes must be derived from
the manifest-owned `internal/agentcapabilities.RequiredCapabilityScopeList`
helper or the README generator's Markdown projection over that helper, not from
a deployment-local hardcoded scope list. Packaging may ship that guidance, but
it must not become a second owner of which scopes the current Pulse
Intelligence surface requires.
README guidance may also describe client setup for the installed adapter, but
server name, command, base URL flag/default, token environment variable, and
supported config families must be derived from
`internal/agentcapabilities.MCPClientConfigMarkdown` over
`Manifest.MCPAdapter`, not from deployment-local OpenCode, Claude, or
`pulse-mcp` setup snippets. Packaging may ship the generated prose and
installers, but it must not become a second owner of MCP client configuration.
Patrol finding tool scopes follow the same boundary: release assets may ship
the generated guidance, but the `ai:execute` requirement for Patrol finding
review and lifecycle calls comes from the manifest/API authorization contract,
not deployment-local monitoring-scope wording.
README guidance may also describe MCP workflow prompts, but the prompt
inventory must be derived from the shared `internal/agentcapabilities`
`ProjectPulseWorkflowPrompts` / `MCPPromptInventoryMarkdown` path. Packaging
may ship the generated prose, but it must not carry a deployment-local prompt
catalog.
README guidance may also describe capability-specific stable error codes, but
the error-code inventory must be derived from the shared
`internal/agentcapabilities` manifest through `MCPErrorCodeInventoryMarkdown`.
Packaging may ship the generated prose, but it must not carry a
deployment-local error-code catalog.
The shared manifest declaration and wire type in `internal/agentcapabilities/`
follow the same split: deployment-installability may package `cmd/pulse-mcp`,
but it must not fork or reinterpret the capability schema, shared
`ProjectedTool`/`ProjectTools` projection, shared `FindCapability` /
`ResolveCapability` lookup contract, shared named capability HTTP execution,
shared structured tool schema, provider-projection helpers, schema-envelope
helper, typed action-mode, approval-policy, or stable error-code contract
locally.
The shared event vocabulary follows that same split. Deployment-installability
may document and package MCP notification support, but event names advertised
by `cmd/pulse-mcp`, the `subscribe_events` manifest description, and
transport-event filtering, SSE record parsing, SSE-to-MCP notification
bridging, and MCP notification method projection remain owned by
`internal/agentcapabilities` plus the API/AI
contracts; release/install artifacts must not carry a separate event registry
or stream parser. The same boundary owns the event-stream HTTP subscription
primitive, including the `Accept: text/event-stream` request convention and
subscribe status handling; packaging may launch `cmd/pulse-mcp` but must not
fork that transport or notification-bridge behavior into install scripts or
release artifacts.
The shared MCP JSON-RPC, request decoding, line-delimited stdio request
serving, notification response policy, stable JSON-RPC encoding,
manifest-backed tool-server semantics, tool-server method dispatch,
initialize instruction/tool-call/resource/prompt payloads, `tools/call` params decode, tool-call
parameter normalization/validation, tool-server initialize result construction,
capability lookup translation, named HTTP invocation, MCP resource URI
projection, context-backed `resources/list` / `resources/read` projection,
manifest-backed `prompts/list` / `prompts/get` workflow prompt projection, and
result envelopes follow the same boundary. Deployment-installability may
document, package, and launch
`cmd/pulse-mcp`, but protocol versions, JSON-RPC error codes,
event-notification projection, method payload JSON, initialize
operating-instruction projection, MCP content/result JSON,
request decoding, line-framing loops, JSON-RPC response serialization,
notification response policy, SSE-to-MCP notification bridging,
manifest-backed tool handlers, method dispatch, initialize response
construction, the MCP tools/call raw bridge, neutral capability tool HTTP
execution, resource URI construction, context-capability resource projection,
prompt catalog and rendering, prompt-argument validation, `HTTPCallResponse` to
shared tool-result wrapping, text/marker interpretation, and the shared rule
that trims/requires tool names while cloning/initializing argument maps must
remain owned by `internal/agentcapabilities` so the installed adapter cannot
drift from Assistant method, tool-call parameter, resource, prompt, or
tool-result execution.
The same split applies to governed Assistant tool markers: packaging may
document approval-required and policy-blocked outcomes, but marker prefixes,
payload `type` values, formatting, and parsing remain owned by
`internal/agentcapabilities`.
The shared agent HTTP substrate follows the same boundary:
deployment-installability may describe how to pass a token and base URL to the
installed adapter, but manifest fetch paths, API-token header spelling, request
content-type behavior, capability HTTP execution, request/response body-return
helpers, status-derived MCP `isError` behavior, and stable non-2xx
error-envelope formatting remain owned by
`internal/agentcapabilities`.
That same installer boundary now owns instance identity for side-by-side server
installs too: the root `install.sh`, generated update helper, and
`scripts/pulse-auto-update.sh` must preserve an explicitly selected service
identity across install, update, reset, uninstall, and timer/service wiring so
stable and preview Pulse runtimes can coexist on one host without drifting back
onto the default `pulse.service` paths.
The generated auto-update systemd wiring is itself contract surface: the root
`install.sh` writes the `pulse-update.service` / `pulse-update.timer` units
(or the service-scoped equivalents) through one shared
`install_auto_update_assets` helper, and the rendered units must contain no
unexpanded `$` reference — every variable is substituted at render time, and
in particular `$$` (which bash expands to the installer's PID inside the
unquoted heredoc) must never reach the unit, because a PID-corrupted
`ExecCondition` makes systemd silently skip every scheduled run. The rendered
`ExecCondition` must gate the run on the detected Pulse service identity
being active. Because updates and reinstalls only run the opt-in
`setup_auto_updates` flow when the operator asks for it, every install flow
over an existing box (update, reinstall, `--version`, `--source`, and the
fresh-install tail behind a leftover timer) must instead refresh
already-installed auto-update assets unconditionally via
`refresh_auto_updates` when the update timer already exists — replacing the
helper script and rewriting the units so a version-pinned helper from a
previous major (which never selects newer releases and reports "Already
running latest version" forever) cannot survive an upgrade — while leaving
`system.json` and the timer's enabled/started state untouched. The
rendered-unit execution, refresh-behavior, and call-site wiring tests in
`scripts/installtests/root_install_sh_test.go` are the owned proof surface
for these invariants.
That same server-installer uninstall must also leave no legacy companion
footprint behind on the host: `install.sh --uninstall` removes the local
`pulse-sensor-proxy` artifacts a v5-era Proxmox host may still carry — the
binary, its systemd units, runtime/state directories, the dedicated service
user/group, and the managed `# pulse-managed-key` / `# pulse-proxy-key` entries
in root's `authorized_keys` — through one installer-owned
`cleanup_local_sensor_proxy` helper that is presence-gated (a silent no-op when
no proxy was installed). The aggressive cluster-wide authorized_keys removal and
`pulse-monitor@pam` API-user deletion stay behind the explicit standalone
`scripts/uninstall-sensor-proxy.sh`, which the installer only prints a pointer
to. `scripts/installtests/root_install_sh_test.go` is the owned proof surface
for that local sensor-proxy cleanup.
That same server-installer boundary also owns release trust fail-closed: the
root `install.sh`, its generated update helper, and
`scripts/pulse-auto-update.sh` must verify downloaded release tarballs and
installer scripts against the pinned release `.sshsig` sidecars before
execution, rather than treating same-origin checksum files as a sufficient
trust anchor. The in-app updater binds to the same invariant: the only
place the Go updater fetches release artifacts is the apply pipeline in
`internal/updates/manager.go::ApplyUpdate` (the adapters in
`internal/updates/adapter_installsh.go` are plan providers only and download
nothing), and every artifact that pipeline fetches — community tarballs via
`downloadAndVerifyReleaseSignature`, Pro broker artifacts via their explicit
sidecar URL — must verify
its `.sshsig` sidecar against the pinned `pulse-installer` ed25519 key
(identity `pulse-installer`, namespace `pulse-install`) and refuse to
proceed if the sidecar is missing, malformed, or fails verification. The
in-app and unattended paths must share the same trust root so the UI's
"Update now" button cannot run at a lower bar than the systemd timer.
The in-app apply pipeline additionally owns pre-install binary validation:
after extraction and before any backup or file replacement,
`internal/updates/manager.go::ApplyUpdate` must locate the extracted `pulse`
binary and prove it executes on this host and reports the apply-target
version, via the `--version` probe in
`internal/updates/selftest.go::selfTestNewBinary`. Checksum and signature
verification prove artifact integrity, not host runnability, so a
wrong-architecture, truncated-yet-published, or unstamped artifact must fail
the update with zero changes applied rather than being swapped in for systemd
to restart into. `internal/updates/selftest_test.go` and the corrupt-binary
apply subtest in `internal/updates/manager_pro_update_test.go` are the owned
proof surface for that validation.
The unattended auto-update path is also fail-closed on prerelease channel
crossing: `scripts/pulse-auto-update.sh` must refuse to act on any tag that
carries a semver prerelease suffix (`-rc.N`, `-beta.N`, `-alpha.N`,
`-nightly`, etc.) regardless of what GitHub's `/releases/latest` endpoint
returns, and must also honour the response's explicit `"prerelease": true`
flag. The release-selection, candidate-evaluation, and installer-invocation
layers of the script must each enforce that guard independently, so a single
miswritten upstream signal cannot cross a stable-channel install onto a
preview tag. Dedicated prerelease-refusal tests in
`scripts/installtests/pulse_auto_update_test.go` are the owned proof surface
for that guard.
That same boundary also owns operator-facing management entry points for
existing self-hosted installs: the installer's printed update/reset/uninstall
commands and the active install or upgrade docs must route supported
systemd/LXC servers through the installed local update helper (`/bin/update`
or the service-scoped equivalent), rather than telling operators to pipe a
freshly downloaded installer into `bash`.
The local dev-runtime launcher and dependency manifest floor now sit on that
same installability boundary.
`scripts/hot-dev.sh` and `scripts/hot-dev-bg.sh` are the canonical owned entry
points for a coherent local Pulse runtime, so frontend shell health, proxy
health, backend health, and listener ownership diagnostics may not drift into
ad hoc shell snippets or undocumented operator lore outside those scripts.
The managed launcher must tolerate a canonical dev environment file that does
not yet contain `PULSE_MOCK_MODE`. Missing mock-mode configuration falls back
to the existing environment/default instead of terminating under `set -e`;
`scripts/tests/test-hot-dev-runtime.sh` pins this startup contract.
Root and frontend workspace dependency manifests, their lockfiles, the
frontend build config, and the Go module graph are canonical inputs to that
developer/runtime bootstrap. Changes to `package.json`, `package-lock.json`,
`frontend-modern/package.json`, `frontend-modern/package-lock.json`,
`frontend-modern/vite.config.ts`, `go.mod`, and `go.sum` must remain governed
with that entrypoint boundary rather than floating as unowned dependency or
build-runtime drift.
Security-driven lockfile bumps for packages shipped in the release frontend
are part of the same governed bootstrap input even when the package manifest
range already permits the newer version; the lockfile must identify the
resolved package version and integrity that the release build will actually
consume.
Security-driven Go module graph bumps follow the same rule: `go.mod` and
`go.sum` must move together when a reachable vulnerability is remediated, and
the slice must carry direct vulnerability or dependency-floor proof so the
release and local dev runtimes consume the intended module graph rather than a
stale transitive floor.
When the managed launcher reports runtime status, it must tell operators which
browser URL to use and whether the frontend shell, proxied API path, and
direct backend health endpoint all agree, instead of leaving `5173` versus
`7655` interpretation to manual inference from whichever process still happens
to be listening.
Changes to `scripts/hot-dev.sh` and `scripts/hot-dev-bg.sh` must therefore
stay on their own direct dev-runtime orchestration proof path instead of
piggybacking on installer proof coverage for unrelated deployment scripts.
That same dev-runtime helper boundary also owns trusted-host behavior for the
developer agent deploy wrapper: `scripts/dev-deploy-agent.sh` may TOFU new SSH
targets, but it must persist host keys in a known_hosts file and fail closed
on host-key changes instead of disabling verification with
`StrictHostKeyChecking=no`.
That same dev-runtime orchestration boundary also owns watcher stability for
the managed local stack: `scripts/hot-dev.sh` may only rebuild the backend for
runtime Go sources, not `*_test.go` churn, and it must suppress `pulse` binary
change events produced by its own successful managed rebuilds, managed backend
restarts, or startup build through shared watcher-state markers rather than
per-subshell timing alone. Parallel watcher streams must not start duplicate
managed rebuilds for the same backend artifact change.
That same boundary also owns backend-liveness recovery, not just process-
existence. The managed health monitor in `scripts/hot-dev.sh` must probe
`http://127.0.0.1:${PULSE_DEV_API_PORT}/api/health` in addition to checking
that a `./pulse` process exists, so an alive-but-unresponsive backend (hung
goroutine, panic-recovery loop, port-bind failure with the process still
running) is detected and restarted instead of leaving the dev frontend
talking to a dead listener. Two consecutive missed health probes must trigger
a managed kill and restart of the unresponsive process only after the managed
backend startup/restart grace has elapsed; the monitor must not kill a backend
merely because the server has bound its listener before the HTTP health route
is ready.
That same watcher boundary also owns backend-served demo coherence:
`internal/api/frontend-modern/dist` changes must trigger a managed backend
rebuild so the `go:embed` frontend on `:7655` cannot drift behind a freshly
synced embedded frontend bundle.
Otherwise unrelated parallel test edits or hot-dev's own binary output can
tear down `7655`, produce transient `5173` proxy failures, and undermine the
canonical browser-runtime proof path.
That same shared helper boundary now also owns browser-versus-API request
truth inside Playwright helpers. `tests/integration/tests/helpers.ts` may
offer request trackers for browser-shell contract proofs, but those helpers
must observe page-originated traffic only and must not blur browser runtime
requests together with `page.request` or other direct API helper calls.
The shared authenticated browser fixture must not report the default mock
runtime ready from inventory alone while historical chart initialization is
still running. When Core E2E requests the default mock-readiness gate, the
helper must also prove that storage-pool and physical-disk history cover the
suite's deepest seven-day chart window. The compose harness must seed that
same seven-day window by default instead of forcing every parallel shard to
build unrelated 90-day preview history.
Managed runtime recovery and browser bootstrap proofs therefore need to keep
helper coverage that demonstrates browser-shell request tracking remains
trustworthy when the same test also performs direct health or security-status
API probes, and that authenticated bootstrap does not fall back to the retired
Dashboard route.
That proof pack must also cover first-session helper re-entry under the managed
runtime: after the dev reset route drives the live setup wizard to Add
infrastructure, the helper must persist the current primary API token into the
runtime-state file and use that token for a later authenticated browser entry
instead of depending on leftover session storage or a dashboard redirect.
`scripts/hot-dev-bg.sh` must also supervise `scripts/hot-dev.sh` in an isolated
child session so an unexpected owner-process death cannot leave orphaned
watchers or health monitors behind. When the supervisor replaces the managed
child, it must terminate the old child process group before starting the next
one.
`scripts/hot-dev-bg.sh verify` must also establish a managed verification lock
for the duration of the proof pack, pass that lock path into the integration
runner, and keep the lock owned by the actual browser-proof process lifetime
rather than dropping it as soon as the launcher command itself exits.
That same deployment boundary now owns hosted tenant canary rollouts too.
`cmd/pulse-control-plane/main.go`, `internal/cloudcp/docker/manager.go`, and
`internal/cloudcp/tenant_runtime_rollout.go` must replace tenant runtime
containers through the canonical Docker manager, snapshot tenant data before
swap, and reconcile the control-plane registry to the live container that
actually serves traffic instead of relying on ad hoc host-local scripts that
swap containers behind the control plane's back. That snapshot-and-restore path
must be self-contained inside the shipped control-plane command rather than
depending on undeclared host binaries such as `rsync`.
across pretest, Playwright, and posttest. `scripts/hot-dev.sh` must honor that
lock by suppressing source-triggered rebuilds and manual `pulse` binary restart
churn while the owning proof process is still alive. Stale verify locks must
clear themselves automatically once the owning process exits.
That deployment boundary also owns hosted storage admission: production
control-plane deployments must mount host root and Docker runtime storage
read-only for inspection, expose explicit root/data/Docker/build-cache
thresholds, and provide `pulse-control-plane cloud audit` as the operator proof
for tenant counts, unhealthy managed containers, disk pressure, stale proof
tenants/accounts, and orphan paid hosted entitlements before GA or rollout
evidence is accepted.
That same verification contract also applies before Playwright attaches: if a
managed hot-dev session is already running when the verify lock is active, the
integration launcher must restart that session instead of silently attaching to
an old frontend process, so browser proof reflects the current branch-tip
source rather than whatever Vite shell happened to be left alive.
That same launcher boundary also owns its CLI contract: managed commands such
as `start --takeover` and `restart --takeover` must preserve the takeover flag
through the actual script entrypoint instead of silently dropping second-arg
control flow and falling back to refusal behavior that contradicts the command
the operator just ran.
That takeover contract also has to reclaim the old dev runtime, not merely
launch another wrapper beside it. When takeover is requested, the launcher
must stop the prior port-owning hot-dev session or direct listeners before the
new managed session starts, otherwise stale watchers can immediately respawn
on `5173` or `7655` and leave split ownership behind.
On macOS that same takeover boundary also includes the optional
`com.pulse.hot-dev` LaunchAgent installed by the local dev launchd helper:
managed takeover must surface that competing job in diagnostics and boot it
out before starting a new managed session, otherwise launchd can immediately
recreate the legacy `0.0.0.0` dev runtime beside the managed `127.0.0.1`
session.
That same managed dev-runtime boundary now also owns operator-safe recovery
control and browser proof. `scripts/hot-dev-bg.sh` must provide a canonical
managed-backend restart command instead of forcing operators or tests to kill
listener PIDs ad hoc, and the integration harness must be able to attach
Playwright to the browser entrypoint on `5173` rather than only the backend
port on `7655`. Recovery proof for this surface must run through the managed
browser runtime, cover both stream-only reconnect degradation and full backend
loss, bounce the real backend through the launcher contract when needed, and
prove that the shell degrades and recovers through the proxy instead of
relying on backend-only API checks that miss browser/runtime drift.
That same managed browser proof pack must also keep the desktop Recovery page
layout guard on the canonical entrypoint, so `dev:verify` catches right-edge
history-table overflow regressions introduced by more human-readable subject
labels instead of leaving that check as a hidden one-off Playwright command.
That same proof pack must also keep the Patrol blocked-runtime page contract on
the canonical entrypoint, so `dev:verify` catches stale healthy-summary
regressions where the real `/ai` route would otherwise look healthy even after
the backend reports `runtime_state=blocked`.
The managed-runtime proof helper that drives those browser checks must also
wait for stable recovered ownership after backend or owner-process restarts,
not just the first transient `200` health probe, otherwise later specs can hit
`ERR_CONNECTION_REFUSED` while the supervisor is still finishing a second
recovery cycle.
That same launcher boundary now also owns the one-command verification entry
point for that proof. `./scripts/hot-dev-bg.sh verify` must prepare a coherent
managed runtime, run the canonical browser recovery proof with the managed dev
credentials and browser entrypoint defaults, and fail with ownership or health
diagnostics instead of leaving operators to remember the exact Playwright
command and env combination by hand.
That same launcher boundary also owns the managed dev auth source of truth.
`scripts/hot-dev.sh` must seed the watched runtime auth `.env` from one
canonical managed-dev credential contract before it reloads runtime overrides,
so stale quick-setup changes under `tmp/dev-config/.env` cannot silently
change the default local login between launches. Repo-root developer docs,
verification wrappers, and integration helper defaults must therefore advertise
the same managed login and treat custom dev credentials as explicit
`HOT_DEV_AUTH_*` or `PULSE_E2E_*` overrides instead of inheriting leftover auth
state from a prior session.
That same runtime override boundary also owns agent reachability coherence:
when a managed dev runtime advertises a local-interface `PULSE_PUBLIC_URL` or
agent connect URL for installed agents, a stale loopback `BIND_ADDRESS` in
runtime `.env` must be reconciled before the backend starts or restarts so
remote agents can report host telemetry instead of buffering indefinitely.
That same takeover path must remain safe on the default macOS Bash runtime and
must not tear down the operator's current shell lineage while reclaiming a
foreground `hot-dev.sh` session. When the canonical ports are already owned by
that foreground session, the managed wrapper should reclaim the occupied
listener processes without relying on Bash-4-only shell features or killing
the terminal that invoked the takeover.
That same launcher boundary now also owns the canonical repo-root developer
entry surface. `package.json` must expose the managed runtime as the default
local dev path, including explicit status, log, stop, backend-restart, and
verification wrappers, instead of requiring developers to remember
lane-specific shell paths or continue discovering the runtime through a stale
unmanaged `5173` process by accident.
That same canonical dev-entry boundary also includes the frontend workspace
package and developer health helper. `frontend-modern/package.json` may not
advertise raw `vite` as the default `dev` command, and `scripts/dev-check.sh`
must route operators back to the managed runtime entrypoint before falling back
to process-killing folklore, otherwise the repo keeps reintroducing the same
split-ownership `5173` versus `7655` confusion through secondary entry
surfaces.
That same `scripts/dev-check.sh` helper must treat `hot-dev-bg status` as the
canonical dev diagnosis surface instead of re-deriving its own competing
frontend-versus-backend health story from ad hoc curls and process scans. Any
secondary checks it adds should be clearly subordinate to the managed runtime
ownership and health report, and unhealthy runtime guidance must point back to
the repo-root managed controls such as `npm run dev` or `npm run dev:restart`.
When the frontend workspace exposes managed runtime wrappers, they must stay in
operational parity with the repo-root entry surface for the canonical controls:
start, status, logs, stop, restart, managed backend restart, verification, and
the explicit foreground escape hatch. The only intentionally narrower frontend
workspace exception is the named `dev:frontend-only` raw Vite escape hatch.
That parity may not be maintained by duplicating raw script paths in two
package manifests. `frontend-modern/package.json` must delegate those managed
commands back to the repo-root npm wrapper surface so the workspace-local entry
points cannot silently drift away from the one canonical operator contract.
That foreground escape hatch contract also applies to `scripts/hot-dev.sh`
itself: its self-description and usage guidance must point operators back to
the canonical managed `npm run dev` path for normal work and reserve
`hot-dev.sh` for explicit foreground/manual troubleshooting.
That same self-description rule applies to `scripts/hot-dev-bg.sh`: even
though it is the managed control surface underneath the wrappers, its usage
guidance must still point operators to the canonical repo-root `npm run dev`
entrypoint for routine startup and label raw subcommands as secondary
troubleshooting controls instead of teaching direct script invocation as the
primary habit.
That operator-guidance rule also applies to the managed launcher's recovery and
diagnostic messages: when `hot-dev-bg` tells users how to start, restart,
verify, supervise, or inspect the routine local dev runtime, it must route them
to the repo-root `npm run dev`, `npm run dev:verify`, and `npm run dev:logs`
wrappers instead of teaching direct raw script invocations for those day-to-day
flows.
That same wrapper rule also applies to the managed recovery-proof docs in
`tests/integration/README.md`: when those instructions tell operators how to
bounce or verify the local managed runtime, they must use the repo-root wrapper
surface such as `npm run dev:backend-restart` instead of documenting raw
launcher commands directly.
That same operator-clarity rule applies anywhere the repo names a local browser
target. Docs that refer to the backend-served standalone or docker UI on
`http://127.0.0.1:7655` or `http://localhost:7655` must label it explicitly as
the embedded frontend or test/standalone UI. They may not present `7655` as
the generic local Pulse browser target in a way that can be confused with the
managed hot-dev shell, whose canonical browser entrypoint remains
`http://127.0.0.1:5173`.
That same browser-target rule applies to the integration harness defaults.
`tests/integration/playwright.config.ts` and the shared integration browser/API
helpers may only fall back to `http://localhost:7655` after honoring an
explicit base URL, runtime-state file, and any active managed `hot-dev`
session. If `hot-dev-bg` is already running, ad hoc Playwright/browser helpers
must prefer the managed shell on `http://127.0.0.1:5173` instead of silently
teaching the backend port as the default local browser target.
When both `PLAYWRIGHT_BASE_URL` and `PULSE_BASE_URL` are set, the shared
browser helper must treat `PLAYWRIGHT_BASE_URL` as the browser truth and leave
`PULSE_BASE_URL` available for backend-oriented health checks and setup
traffic, so split browser/backend proof can target fresh frontend code without
rewiring the API-side contract.
The Playwright managed-local-backend harness is part of that same canonical
integration runtime boundary. `tests/integration/scripts/managed-local-backend.mjs`
must seed a per-run audit signing key for local proof startup, while honoring
explicit `PULSE_AUDIT_SIGNING_KEY` or deterministic `PULSE_E2E_AUDIT_SIGNING_KEY`
overrides, so the runtime audit logger can remain fail-closed without breaking
managed local backend tests.
That same integration-README ownership includes the retired local commercial
trial probe guidance. The snapshot-clean trial instructions for
`tests/integration/scripts/retired-trial-acquisition-contract.sh` must describe
`POST /api/license/trial/start` as retired in ordinary self-hosted v6 and must
expect `404` plus unchanged entitlements. The reused-instance browser-proof
entry in `tests/integration/README.md` must carry that same retired-route
posture, so the shared trial-start docs guard can auto-discover that README
alongside the rest of the governed trial-start surface instead of relying on
README-only fallback checks. That README must also keep the named Pulse Pro
browser proof, `tests/58-self-hosted-trial-rate-limit-ui.spec.ts`, on the
owned paid-prompt surface so the user-facing no-trial-CTA proof does not drift
into an orphaned integration spec. The eval-pack metadata in
`tests/integration/evals/scenarios.json` must carry those same anchors for the
`retired-trial-acquisition` scenario description, so deterministic and agentic trial runs
inherit the same canonical contract wording instead of teaching a drifted
summary path.
Playwright-driven public/commercial specs that support scenario-specific
endpoint overrides such as `PULSE_CLOUD_BASE_URL` or
`PULSE_COMMERCIAL_BASE_URL` must layer those values through that same shared
route helper instead of duplicating `PLAYWRIGHT_BASE_URL` versus
`PULSE_BASE_URL` precedence locally.
That defaulting rule must live in one shared integration helper rather than
being duplicated between config and helper files, so future browser-target
changes cannot leave Playwright navigation and browser/API helper calls
disagreeing about whether the managed shell or embedded frontend is canonical.
That runtime-guidance rule also applies to successful launcher startup output:
`hot-dev-bg` must identify `http://127.0.0.1:5173` as the browser entrypoint
and present `7655` as the managed backend dependency, rather than advertising
frontend and backend URLs as if they were equal browser targets.
The managed repo-root `npm run dev` path must also be self-healing at the
launcher layer: `hot-dev-bg` may not treat a detached `hot-dev.sh` child as
sufficient management. The default managed runtime must supervise that child
and restart it when the owner process exits unexpectedly, so a killed or wedged
foreground owner does not leave both `5173` and `7655` down until a human
notices.
That self-healing guarantee must be covered by the canonical managed browser
proof pack as well: `dev:verify` must prove backend-bounce recovery,
owner-process-death recovery, and the Patrol blocked-runtime page contract on
the browser entrypoint, rather than leaving supervision and Patrol-shell drift
to shell-only smoke tests.
The same wrapper-first rule applies to launcher help text: `hot-dev-bg` usage
output must present the repo-root npm entrypoints first and reserve raw
subcommands as secondary script-local controls for direct troubleshooting.
That same dev-runtime helper boundary also includes the auxiliary operator
controls that start, stop, restart, or recover local development. The repo-root
Makefile targets, `scripts/toggle-mock.sh`, and `scripts/clean-mock-alerts.sh` must
route through the managed runtime control plane when they are operating on the
local dev stack, instead of resurrecting lane-local `hot-dev.sh` or raw Vite
process management through separate shell folklore. For Makefile targets, that
means dispatching through the canonical repo-root npm wrappers (`npm run dev`,
`npm run dev:lab`, `npm run dev:status`, `npm run dev:status:lab`,
`npm run dev:restart`, `npm run dev:restart:lab`,
`npm run dev:backend-restart`, `npm run dev:verify`,
`npm run dev:verify:lab`, `npm run dev:stop`, `npm run dev:foreground`, and
`npm run dev:foreground:lab`) rather
than shelling directly into `scripts/hot-dev-bg.sh`.
When `scripts/clean-mock-alerts.sh` needs to quiesce a local dev runtime, it
must stop the managed session through `hot-dev-bg` before touching legacy
compatibility services, and its operator recovery guidance must point back to
the canonical repo-root `npm run dev` and `npm run dev:foreground` controls
instead of treating `pulse-hot-dev` service management as the primary dev path.
That same rule now extends to the macOS auto-start surface. The launchd helper
may not boot a separate legacy foreground runtime beside the managed dev stack:
`scripts/dev-launchd-wrapper.sh`, `scripts/dev-launchd-setup.sh`, and the
generated `com.pulse.hot-dev` LaunchAgent template must supervise the same
managed `hot-dev-bg` control plane, so login-time auto-start, crash restart,
and takeover diagnostics all operate on one runtime model.
That same launchd helper must also advertise the canonical managed runtime
controls as its primary operator surface. After installation it should point
developers back to the browser entrypoint on `http://127.0.0.1:5173` and the
repo-root `npm run dev`, `npm run dev:restart`, `npm run dev:status`, and
`npm run dev:logs` commands for daily use, while keeping raw `launchctl`
commands clearly secondary as LaunchAgent maintenance operations.
That shared `scripts/install.sh` boundary must also keep one canonical service
argument builder for the runtime flags it persists. Token-bearing install
paths, token-file systemd paths, wrapper-script launches, and later service
materialization must all derive their flag set from the same installer-owned
argument item list instead of rebuilding overlapping `--url`, `--token`,
feature toggles, identity flags, and disk-exclude transport in separate shell
blocks.
That shared `scripts/install.sh` boundary must also stay aligned with the
canonical auto-register contract: when the installer performs Proxmox
auto-registration after creating a local token, it must submit that token
completion on the canonical /api/auto-register contract using the canonical
`tokenId`/`tokenValue` payload shape and the explicit `source="script"`
marker, that marker must stay exactly `script` rather than a lane-local alias,
the node `type` must stay on the supported `pve` or `pbs` set,
the `tokenId` must stay on the canonical Pulse-managed
`pulse-monitor@{pve|pbs}!pulse-<canonical-scope-slug>` identity matching the
selected node type, and the locally created Proxmox token name must stay on the same
deterministic Pulse-managed `pulse-<canonical-scope-slug>` contract used by the
other v6 registration callers instead of appending timestamp or rerun-local
entropy,
and it must fail closed unless the response comes back on the canonical
`status="success"` plus `action="use_token"` completion shape. That same
installer response handling must also use the returned canonical
`nodeId`/`nodeName` identity instead of continuing to report the caller's local
hostname after Pulse stores a disambiguated node record.
When Proxmox is only auto-detected rather than explicitly profile-pinned, that
same installer-owned boundary must enable Proxmox without persisting a forced
`--proxmox-type` service argument. Auto mode must stay unpinned so the runtime
can detect and register every supported local Proxmox service it finds; only
an operator-selected install profile may lock the persisted runtime to one
specific `pve` or `pbs` type.
That same installer-owned bootstrap step against `/api/setup-script-url` must
also validate the returned canonical `type`, normalized `host`, and live
`expires` metadata before using the one-time setup token, so install-time
registration cannot drift onto a stale or mismatched bootstrap response.
Install-time PVE auto-registration must also create privilege-separated
Pulse-managed monitor tokens and mirror effective ACLs to the concrete token id
rather than relying on user-only grants or shared-token inheritance. Its
`PulseMonitor` role setup must prefer `VM.GuestAgent.Audit` plus
`VM.GuestAgent.FileRead` when those PVE 9+ privileges are available, and fall
back to legacy `VM.Monitor` only when the guest-agent privilege probe is
unavailable. That same install-time token creation must extract the token
secret deterministically: it must request the machine-readable
`pveum ... --output-format json` form first and parse the `value` field,
falling back to the legacy box-drawing table layout only when an older pveum
rejects the JSON flag — matching the hardened web-setup render path
(`internal/api/setup_script_render.go`) so token capture does not silently fail
or mis-parse when pveum's table formatting drifts across versions/locales.
`scripts/installtests/root_install_sh_test.go` is the owned proof surface for
that install-time extraction. A
non-empty `expires` field alone is not sufficient; the installer must reject
bootstrap responses whose expiry is already in the past. That same bootstrap
consumer must also fail closed unless the runtime-owned setup metadata is
present and coherent: installer-side Proxmox auto-register must reject missing
or mismatched `url`, `scriptFileName`, `command`, `commandWithEnv`, or
`commandWithoutEnv` fields instead of treating `/api/setup-script-url` as a
setup-token-only side channel. It must also require the canonical
token-bearing `downloadURL` and masked `tokenHint` fields, so the installer is
validating the same full bootstrap artifact contract as the governed settings
surface instead of accepting an older reduced response shape. Those installer
checks must also validate command transport coherence, not just field presence:
the returned token-bearing commands must reference the canonical setup-script
URL and carry the setup token through the governed root-or-sudo wrapper, while
the preview `commandWithoutEnv` transport must stay on the same canonical URL
without leaking the setup token back into the non-secret path. That bootstrap
request itself must stay on the real setup-script-url auth boundary too:
install-time Proxmox auto-register must not model `/api/setup-script-url` as a
setup-token-authenticated endpoint or depend on scraping a plaintext
`.bootstrap_token` file just to call it. The supported operator retrieval path
for first-session bootstrap is `pulse bootstrap-token`, and runtime bootstrap
token persistence must stay encrypted at rest.
Root installer completion output, LXC post-install guidance, and copied
first-session setup instructions must also route operators through
`pulse bootstrap-token` with the correct runtime data directory instead of
printing or instructing users to `cat` `.bootstrap_token`, because the file is
an encrypted persistence artifact rather than the raw setup token.
That same bootstrap artifact contract must now be backend-owned as one
canonical install artifact model rather than a handler-local bootstrap struct
plus a second response envelope. Shell downloads, setup-script-url responses,
and rerun guidance must all read from that same backend artifact shape.
Generated PVE and PBS setup-script bodies must also render through shared
backend install helpers instead of a handler-local shell template engine in
`config_setup_handlers.go`, so installability ownership stays at the shared
artifact/render boundary rather than inside one route handler.
That same post-install discovery refresh path must treat discovery string
errors as compatibility-only output derived from canonical structured runtime
errors, so setup/install handlers do not become a second owner of legacy
discovery payload state.
That shared `scripts/install.ps1` boundary must also stay under explicit proof
routing on both sides instead of relying only on broad installer-script
coverage: Windows installer changes must continue to carry the direct
`windows-agent-installer-runtime` lifecycle proof together with the direct
`deployment-script-runtime` installability proof.
That same installability proof rule also applies to
`scripts/install-container-agent.sh`: changes to the container-agent installer
must stay on the direct `deployment-script-runtime` proof path instead of
relying only on broad script ownership.
That same rule also applies to `scripts/pulse-auto-update.sh`: changes to the
unattended auto-update script must stay on the direct
`deployment-script-runtime` proof path instead of relying only on broad script
ownership.
That Windows installer boundary must also stay aligned with token-optional
Pulse deployments: when the server does not require API tokens, the installer
must accept a missing token and persist service arguments without `--token`
instead of advertising an optional-auth install path that still fails local
parameter validation.
The same Windows installer boundary must also preserve profile-target parity
with the governed settings surface: when PowerShell install transport sets
`PULSE_ENABLE_PROXMOX` and `PULSE_PROXMOX_TYPE`, `scripts/install.ps1` must
validate and persist those Proxmox flags into the service command line rather
than discarding the selected profile at install time.
That same Windows installer boundary must also preserve governed transport and
runtime toggles from the settings surface: when PowerShell install transport
sets `PULSE_INSECURE_SKIP_VERIFY` or `PULSE_ENABLE_COMMANDS`,
`scripts/install.ps1` must persist those settings into the service command line
instead of dropping TLS-mode or command-execution intent on Windows installs.
The same insecure-TLS boundary must also affect installer-owned network calls:
when `PULSE_INSECURE_SKIP_VERIFY` is enabled, `scripts/install.ps1` must use
that relaxed certificate policy for its own agent download and uninstall
deregistration requests so self-signed deployments do not fail before the
persisted Windows service ever starts.
Windows installability proof must also verify the installed service's local
readiness endpoint, not just SCM `Running` state: the Windows service runtime
must start the shared Pulse Agent health/readiness server so `/readyz` can prove
the agent modules initialized after install. That proof must also require the
installer-advertised ProgramData log to exist and contain startup evidence,
exercise configured SCM crash recovery, replace one real agent version with a
second, and prove uninstall removes the service, binary, state, token/log
artifacts, and readiness listener. OS-reboot-capable labs use the harness's
split install/update and post-reboot/uninstall phases; hosted CI uses its full
service-lifecycle phase without rebooting the ephemeral runner.
Copied PowerShell uninstall commands must preserve that same
`PULSE_INSECURE_SKIP_VERIFY` setting so the governed deregistration request can
still reach self-signed Pulse deployments during removal.
Copied per-agent uninstall commands must also preserve the canonical agent
identity when the settings surface already knows it, so `scripts/install.sh`
and `scripts/install.ps1` do not have to fall back to local state-file recovery
or hostname lookup just to deregister the selected agent.
Those copied uninstall commands must also preserve the canonical hostname as
the fallback identity and the installer runtimes must honor it first during
lookup recovery, so removal stays bound to the selected Pulse inventory row
instead of drifting to the local machine name.
That same identity continuity must persist across later shell-managed removal:
the saved `connection.env` state must retain explicit agent and hostname
identity when install or upgrade supplied them, so offline uninstall does not
lose the selected node identity just because the runtime state file is absent.
That same saved shell artifact must now stay installer-owned as one canonical
writer/reader path: `scripts/install.sh` may not keep a heredoc writer plus a
second inline field parser for the same `connection.env` contract, because
offline uninstall must consume the same persisted install-state artifact the
installer wrote instead of reconstructing it ad hoc.
That same shell-agent update recovery path must fail closed on partial
legacy process or service-unit state: a recovered URL without a recovered token
is not usable connection state and must not be logged or treated as recovered.
Fallback recovery may merge URL and token across process args, environment, and
systemd unit data, but it may report success only once both values are present;
otherwise the update command must fall through to the explicit missing-state
error instead of implying recovery succeeded. Explicit update arguments that
provide only the URL must still run legacy process/service recovery before this
decision so v5 agents without `connection.env` can recover their token and
identity.
That same installer ownership now also applies to service lifecycle control:
upgrade, reinstall, and platform-specific start/restart flows may not each
carry their own stop/start command sequence for the same agent runtime.
`scripts/install.sh` must route systemd, OpenRC, SysV, and service-command
control through explicit installer-owned helpers so service behavior does not
drift by platform block.
The same canonical ownership must cover teardown and removal too: uninstall,
reinstall cleanup, and platform-specific disable/remove flows may not each
re-author stop, disable, remove, and daemon-reload sequences inline.
`scripts/install.sh` must route service teardown through shared installer
helpers so removal semantics stay consistent across systemd, OpenRC, SysV,
and service-command runtimes.
TrueNAS boot recovery must follow the same rule: SCALE and CORE bootstrap
scripts may differ only in their service-manager adapter, while binary sync,
service-link recreation, and boot-time start flow stay on one installer-owned
renderer instead of two separate heredocs.
That same ownership rule applies to persisted service definitions: DSM, Linux,
TrueNAS, and FreeBSD service/unit files may not keep re-authoring the same
runtime contract in separate heredocs. `scripts/install.sh` must route shared
systemd and FreeBSD rc.d rendering through canonical installer-owned helpers,
with platform branches only supplying the adapter-specific inputs.
That same installer ownership must also cover completion reporting: platform
branches may not each rebuild their own health-verification result handling,
`json_event` completion payloads, or uninstall guidance. `scripts/install.sh`
must route final save-state, healthy/unhealthy status output, and completion
event emission through one canonical installer-owned helper.
FreeBSD enablement must follow the same rule: direct rc.d installs and
TrueNAS CORE bootstrap may not keep separate inline `pulse_agent_enable`
mutation logic. `scripts/install.sh` must own one canonical rc.conf enablement
snippet/helper and reuse it across runtime and boot-recovery paths. That helper
must execute the shared snippet in-process before applying it, rather than
defining the function in a throwaway subshell that leaves the enable step
silently undone.
SysV enable-on-boot registration must follow the same rule: install-time
`update-rc.d`, `chkconfig`, and manual rc.d symlink fallback may not live as a
separate inline block when teardown already has a canonical owner. The
installer must route SysV registration through one shared helper so service
registration semantics do not drift between install and removal paths.
Windows installability must follow the same rule: installer-owned state under
ProgramData must retain explicit connection identity from install or upgrade so
later PowerShell uninstall can still deregister the intended agent record when
runtime-local state is missing or stale.
The same uninstall lookup transport rule applies across both canonical
installers: when fallback identity recovery calls `/api/agents/agent/lookup`,
the resolved hostname must be percent-encoded before it is placed in the query
string.
The same copied uninstall commands must also fail closed on token-required
deployments: when auth is required, command builders must preserve the required
token contract instead of silently emitting tokenless removal transport.
The same copied Unix lifecycle commands must also preserve shell-safe argument
transport, so canonical URL, token, agent ID, and hostname values survive copy
and paste without being re-tokenized by the local shell.
The same copied Windows lifecycle commands must preserve PowerShell-safe
argument transport, so canonical URL, token, agent ID, and hostname values do
not get reinterpreted by PowerShell during uninstall or upgrade. That same
Windows upgrade transport must also quote the resolved `install.ps1` URL, so
custom canonical URLs with spaces still survive copied PowerShell reruns.
The same uninstall transport must quote that resolved script URL as well, so
Windows removal on custom canonical URLs does not regress back to unquoted
PowerShell invocation.
The same copied install commands must preserve shell-safe and PowerShell-safe
transport for canonical URL/token values, so copy-paste install flows do not
reinterpret those inputs before the installer even starts.
That same Windows interactive install transport must preserve the selected
canonical server URL in `PULSE_URL`, so a copied PowerShell install command
cannot drift back to a different prompted target after downloading the script.
When the settings surface already has a selected token, that same interactive
Windows install transport must preserve it in `PULSE_TOKEN` as well, so copied
PowerShell installs do not regress to a second credential prompt after the user
already generated the governed token.
Before a real token exists, the same interactive Windows transport must stay
prompt-based instead of exporting a placeholder token value into `PULSE_TOKEN`.
On token-optional Pulse instances, that same governed install surface must
support both valid paths: no-token transport after explicit confirmation, and
credentialed transport when the operator still generates a real token. Optional
auth may not silently downgrade the settings surface to tokenless-only mode.
For Unix-family copied host installs, the deployment-owned shell installer must
support preflight before privilege escalation: `--preflight-only` may run
without root, must check both `/api/health` and the exact
`/download/pulse-agent?arch=...` artifact for checksum metadata, and must fail
before installation if the server cannot provide that binary. Token-bearing
copy-paste commands must pass credentials through ephemeral `--token-file`
transport and leave the installed service configured with the persistent
runtime token file, never a raw `--token` process argument.
That Windows installer-owned state must also be cleared after successful
PowerShell uninstall, so a removed installation does not leave stale ProgramData
identity or transport continuity behind for later lifecycle commands.
The same saved uninstall state must preserve insecure/self-signed transport
mode for both canonical installers, so an offline uninstall on a self-signed
Pulse deployment does not regress from the original operator-approved transport
policy back to strict TLS.
For the shell installer, saved uninstall state must also preserve custom CA
bundle transport so offline removal can still reach Pulse when trust depends on
an explicit `--cacert` path instead of insecure mode.
The Windows installer must preserve the same installer-owned custom CA
transport continuity: when install or upgrade ran with `PULSE_CACERT`,
`scripts/install.ps1` must validate that certificate file, use it for its own
download and uninstall-time API transport, and persist the path so later
offline uninstall can recover governed trust without falling back to insecure
mode or strict default trust.
That same installer-owned custom-CA continuity must also reach the Windows
service it provisions: `scripts/install.ps1` must persist `--cacert` into the
created `pulse-agent` service command line so the installed agent keeps using
the same governed trust chain for runtime update, remote-config, and reporting
transport instead of narrowing `PULSE_CACERT` to installer-only HTTPS.
That offline shell uninstall recovery must trigger on partial operator-supplied
context, not only when URL or token are absent, so persisted identity and
transport continuity still reload when a later uninstall command provides only
part of the canonical connection tuple.
The same copied-upgrade path must preserve canonical agent and hostname
identity when the settings surface already knows them, so rerunning the
installer for an outdated node does not reset service/runtime identity back to
ambient local machine defaults.
The same Windows installer boundary must keep uninstall deregistration aligned
with token-optional deployments: when URL and agent identity are known, the
PowerShell uninstall path must still call the canonical agent-uninstall API
without requiring an API token, adding `X-API-Token` only when a real token is
available.

`internal/api/updates.go` and `frontend-modern/src/api/updates.ts` are shared
boundaries with `api-contracts`: they are the product-facing update transport
surface while canonical payload-shape governance remains explicit in the API
contract boundary.
That shared update transport boundary must also stay under explicit proof
routing on both sides instead of relying only on generic API fallback
coverage: update transport changes must continue to carry the direct
`updates-api-surface` installability proof together with a direct
API-contract proof path.
That same governed release-promotion boundary now also owns detached agent and
installer signatures. `scripts/build-release.sh`,
`scripts/release_asset_common.sh`, `scripts/backfill-release-assets.sh`,
`scripts/release_update_key.go`, `scripts/render_installers.go`,
`scripts/release_ldflags.sh`, `Dockerfile`,
`.github/workflows/backfill-release-assets.yml`,
`.github/workflows/create-release.yml`, `.github/workflows/publish-docker.yml`,
`scripts/validate-release.sh`, and `scripts/validate-published-release.sh`
must derive the embedded update trust root and installer SSH trust root from
the governed release signing key, invoke release signing helpers from the
module-root package path so Go `internal/` boundaries stay valid in local and
CI release builds, render release installers with that pinned SSH verifier,
emit both `.sig` and `.sshsig` sidecars for shipped agent
binaries and installer assets, emit a standalone SPDX JSON SBOM for the
assembled release packet, upload those security artifacts with the matching
release packet, and fail validation if any published artifact or
`checksums.txt` is missing its `.sshsig` sidecar or if the canonical
release-packet SBOM is absent so published RC/stable downloads can keep the
updater and installer trust chain fail-closed instead of downgrading to
checksum-only trust and can publish a shareable non-image software inventory
alongside the signed binaries.
The immutable candidate builder must model macOS Developer ID/notarization and
Windows Authenticode as independent native-signing requirements rather than one
all-or-nothing platform switch. Governed RC publication may require signed and
notarized macOS agent binaries while Windows Authenticode approval is still an
externally owned bounded residual, but only when the RC packet explicitly
discloses the unsigned Windows publisher state and the Windows binaries retain
the exact-SHA candidate, checksum, detached-signature, and post-publication
digest controls. Stable publication and the stable-path dry-run must continue
to require both native signing lanes. `scripts/build-release.sh` must replace
only the native targets required by those independent inputs and must fail
closed when a required native-binary directory or target is absent.
Historical published-release repair must flow through
`scripts/backfill-release-assets.sh` and
`.github/workflows/backfill-release-assets.yml` or the canonical
`.github/workflows/create-release.yml` historical backfill mode, which download
the already-published packet and regenerate only the derived integrity assets
(`checksums.txt`, `.sha256`, `.sig`, `.sshsig`, and the canonical
release-packet SBOM`) from those shipped bytes instead of rebuilding binaries
from the current branch tip.
The shell-installer boundary now also owns the QNAP boot bootstrap and
teardown contract end to end: `scripts/install.sh` must persist the wrapper on
the writable data volume, write a flash-backed `autorun.sh` block that waits
for that volume before launching the wrapper, recover the same state during
uninstall, and keep the persisted boot copy aligned with updater-owned runtime
binary replacements instead of assuming `/usr/local/bin` survives reboot on
QTS/QuTS hero.

The in-app updater's apply pipeline now owns a downgrade guard on the normal
apply path. A syntactically valid release asset URL can name a release older
than the running binary, so `internal/updates/manager.go` `ApplyUpdate` must
reject any resolved target version at or below the running version, on both
the community release-asset path and the Pro broker path, before any history
entry is written or byte is downloaded. Sanctioned downgrades are an explicit
opt-in through the `AllowDowngrade` request flag carried by
`POST /api/updates/apply`, never a silent side effect of a stale download
URL. The guard fails open only for versions that do not parse as semver
(development builds), which stay covered by the existing URL and channel
validation. `internal/updates/manager_rollback_test.go` and the apply handler
tests in `internal/api/updates_test.go` are the direct proof surface for this
rule.

That same updater boundary now also owns the sanctioned rollback path from
retained update backups. `RollbackToBackup` in `internal/updates/manager.go`
restores the backup directory recorded on an update history entry after
re-validating that the path still names a managed update backup on disk,
shares the single update-in-flight slot with `ApplyUpdate`, records the
rollback as its own history entry with `Action` `rollback` and a
`RelatedEventID` back to the rolled-back update, marks that source entry
`rolled_back`, streams progress through the existing update status and SSE
machinery as the `restoring` stage, and restarts through the same
exit-for-systemd path as a normal update. The transport surface is
`POST /api/updates/rollback`, admin plus `settings:write` gated exactly like
apply, and the Settings update history table in
`frontend-modern/src/components/Settings/UpdateHistorySection.tsx` is the
user-facing rollback surface. Rollback is a purely local restore: it must not
touch the Pro download broker or any edition gate, so it behaves identically
on community and Pro binaries. The rollback tests in
`internal/updates/manager_rollback_test.go`, the rollback handler tests in
`internal/api/updates_test.go`, and the route inventory pin for
`/api/updates/rollback` are the proof surface for this path.

### Observer destination installation continuity

Unix and Windows installers accept `--observers-file` and preserve the absolute
path in the installed service command. Unix installation rejects relative,
missing, and symlink configuration paths before service mutation. The runtime
remains the final schema, permission, token-file, URL, and TLS-policy validator.
Updates recover the observer-file argument from the existing service command so
an in-place binary refresh does not silently collapse a multi-destination
installation back to primary-only reporting.
