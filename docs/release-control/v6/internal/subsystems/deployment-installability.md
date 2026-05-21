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

Own server installation, deployment bootstrap behavior, update planning, and
server-side update execution surfaces.

## Canonical Files

1. `internal/updates/`
2. `internal/api/updates.go`
3. `frontend-modern/src/api/updates.ts`
4. `cmd/pulse-control-plane/main.go`
5. `cmd/pulse-control-plane/mobile_proof_cmd.go`
6. `internal/cloudcp/docker/manager.go`
7. `internal/cloudcp/docker/labels.go`
8. `internal/cloudcp/tenant_runtime_rollout.go`
8. `.github/workflows/create-release.yml`
9. `.github/workflows/deploy-demo-server.yml`
10. `.github/workflows/helm-pages.yml`
11. `.github/workflows/promote-floating-tags.yml`
12. `.github/workflows/publish-docker.yml`
13. `.github/workflows/publish-helm-chart.yml`
14. `.github/workflows/release-dry-run.yml`
15. `.github/workflows/update-demo-server.yml`
16. `.github/workflows/validate-release-assets.yml`
17. `.github/workflows/install-sh-smoke.yml`
16. `.github/ISSUE_TEMPLATE/v6_rc_feedback.yml`
17. `docs/RELEASE_NOTES.md`
18. `docs/releases/`
19. `docs/UPGRADE_v6.md`
20. `docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md`
21. `docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md`
22. `docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md`
23. `package.json`
24. `package-lock.json`
25. `frontend-modern/package.json`
26. `frontend-modern/package-lock.json`
27. `frontend-modern/vite.config.ts`
28. `go.mod`
29. `go.sum`
30. `scripts/build-release.sh`
31. `scripts/check-workflow-dispatch-inputs.py`
32. `scripts/clean-mock-alerts.sh`
33. `scripts/com.pulse.hot-dev.plist.template`
34. `scripts/dev-check.sh`
35. `scripts/dev-deploy-agent.sh`
36. `scripts/dev-launchd-setup.sh`
37. `scripts/dev-launchd-wrapper.sh`
38. `scripts/hot-dev-bg.sh`
39. `scripts/hot-dev.sh`
40. `scripts/lib/hot-dev-runtime.sh`
41. `scripts/lib/hot-dev-auth.sh`
42. `scripts/install-container-agent.sh`
43. `install.sh`
44. `scripts/install.ps1`
45. `scripts/install.sh`
46. `scripts/install-mcp.sh`
47. `scripts/install-mcp.ps1`
48. `cmd/pulse-mcp/`
49. `scripts/pulse-auto-update.sh`
50. `scripts/release_control/internal/record_rc_to_ga_rehearsal.py`
51. `scripts/release_control/record_rc_to_ga_rehearsal.py`
52. `scripts/release_control/release_promotion_policy_support.py`
53. `scripts/release_control/resolve_release_promotion.py`
54. `scripts/release_ldflags.sh`
55. `scripts/run_cloud_public_signup_smoke.sh`
56. `scripts/run_demo_public_browser_smoke.sh`
57. `scripts/demo_public_browser_smoke.cjs`
58. `scripts/run_hosted_staging_smoke.sh`
59. `scripts/trigger-release-dry-run.sh`
60. `scripts/trigger-release.sh`
61. `scripts/toggle-mock.sh`
62. `deploy/helm/pulse/`
63. `tests/integration/playwright.config.ts`
64. `tests/integration/QUICK_START.md`
65. `tests/integration/README.md`
66. `tests/integration/scripts/bootstrap-hosted-mobile-onboarding.mjs`
67. `tests/integration/scripts/hosted-mobile-token-runtime.mjs`
68. `tests/integration/scripts/hosted-tenant-approval-store.mjs`
69. `tests/integration/scripts/hosted-tenant-runtime.mjs`
70. `tests/integration/scripts/hosted-tenant-runtime-restart.mjs`
71. `tests/integration/scripts/managed-dev-runtime.mjs`
72. `tests/integration/scripts/relay-mobile-token-helper.go`
73. `tests/integration/tests/helpers.ts`
74. `tests/integration/tests/runtime-defaults.ts`
75. `docker-compose.yml`
76. `scripts/install-docker.sh`
77. `scripts/validate-published-release.sh`
78. `scripts/validate-release.sh`
79. `scripts/release_asset_common.sh`
80. `scripts/backfill-release-assets.sh`
81. `.github/workflows/backfill-release-assets.yml`

## Shared Boundaries

1. `frontend-modern/src/api/updates.ts` shared with `api-contracts`: the updates frontend client is both a deployment-installability control surface and a canonical API payload contract boundary.
2. `internal/api/updates.go` shared with `api-contracts`: update handlers are both a deployment-installability control surface and a canonical API payload contract boundary.
3. `internal/cloudcp/docker/labels.go` shared with `cloud-paid`: hosted tenant Docker labels are both a Pulse Cloud runtime contract boundary and a deployment-installability rollout boundary.
4. `internal/cloudcp/docker/manager.go` shared with `cloud-paid`: hosted tenant container management is both a Pulse Cloud runtime contract boundary and a deployment-installability rollout boundary.
   Tenant runtime containers must be created with bounded Docker `json-file`
   logging so rollout and canary fleets cannot consume unbounded production
   host storage while they remain running.
5. `internal/cloudcp/tenant_runtime_rollout.go` shared with `cloud-paid`: hosted tenant runtime rollout is both a Pulse Cloud runtime contract boundary and a deployment-installability release-rollout boundary.
6. `scripts/install.ps1` shared with `agent-lifecycle`: the Windows installer is both a deployment installability entry point and a canonical agent lifecycle runtime continuity boundary.
7. `scripts/install.sh` shared with `agent-lifecycle`: the shell installer is both a deployment installability entry point and a canonical agent lifecycle runtime continuity boundary.

## Extension Points

1. Add or change deployment-type detection, update planning, or apply behavior through `internal/updates/`
2. Add or change release-build metadata injection, Docker build-context allowlists, release artifact assembly, governed promotion metadata resolution, the canonical version file, operator-facing release packet content, prerelease feedback intake wording, historical published-release integrity backfill, release asset validation status publication, download endpoint checksum/signature header proof, end-to-end install.sh smoke against the published release, or the canonical in-repo v6 upgrade guide through `scripts/build-release.sh`, `scripts/release_asset_common.sh`, `scripts/backfill-release-assets.sh`, `scripts/release_ldflags.sh`, `scripts/check-workflow-dispatch-inputs.py`, `scripts/release_control/render_release_body.py`, `scripts/release_control/resolve_release_promotion.py`, `scripts/release_control/record_rc_to_ga_rehearsal.py`, `scripts/release_control/internal/record_rc_to_ga_rehearsal.py`, `scripts/release_control/release_promotion_policy_support.py`, `.dockerignore`, `Dockerfile`, `.github/ISSUE_TEMPLATE/v6_rc_feedback.yml`, `docs/RELEASE_NOTES.md`, `docs/releases/`, `docs/UPGRADE_v6.md`, `docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md`, `docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md`, `docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md`, `scripts/validate-release.sh`, `scripts/validate-published-release.sh`, the operator dispatch helpers `scripts/trigger-release.sh` and `scripts/trigger-release-dry-run.sh`, and the governed release workflows `.github/workflows/backfill-release-assets.yml`, `.github/workflows/create-release.yml`, `.github/workflows/deploy-demo-server.yml`, `.github/workflows/helm-pages.yml`, `.github/workflows/install-sh-smoke.yml`, `.github/workflows/publish-docker.yml`, `.github/workflows/publish-helm-chart.yml`, `.github/workflows/promote-floating-tags.yml`, `.github/workflows/release-dry-run.yml`, `.github/workflows/update-demo-server.yml`, and `.github/workflows/validate-release-assets.yml`
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
   The top-level `install.sh` asset published on GitHub Releases must be the
   root Pulse SERVER installer (the LXC / systemd / Proxmox VE installer that
   accepts `--version vX.Y.Z`, `--rc`, `--stable`, and friends). The rendered
   AGENT installer (`scripts/install.sh`) ships only inside release tarballs
   at `./scripts/install.sh` and inside Docker images at
   `/opt/pulse/scripts/install.sh`, and is served at the running server's
   `/install.sh` endpoint; it is intentionally never the top-level GitHub
   Releases asset. `internal/updates/adapter_installsh.go`,
   `scripts/pulse-auto-update.sh`, and the root `install.sh`'s own
   `--rc` / `--stable` / `--version` self-refetch flows all fetch
   `releases/<tag>/install.sh` and execute it via `bash -s -- --version vX.Y.Z`,
   and the README quickstart documents the same pattern. Publishing the agent
   installer in that slot silently breaks every one of those flows because the
   agent installer rejects `--version` as an unknown argument; this drift
   shipped across v6 rc.1 → rc.5 (April 12 → May 11, 2026) before being caught.
   `scripts/validate-release.sh` must therefore fail the release if the
   published `install.sh` does not carry the server-installer banner, does not
   handle `--version)` in its argument parser, contains the agent installer
   banner string, or does not print the server installer's version-pinning
   help line when invoked with `--help`.
   Deployment bootstrap token behavior remains a deployment-installability
   trust boundary even when the handler is API-owned. `internal/api/deploy_handlers.go`
   must preserve server-derived `owner_user_id` lineage on bootstrap tokens and
   enrollment runtime tokens while keeping deploy binding metadata limited to
   deploy facts such as cluster, job, target, source agent, and expected node.
4. Add or change server update transport through `internal/api/updates.go` and `frontend-modern/src/api/updates.ts`
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
6. Add or change governed release-promotion workflow inputs, operator-facing promotion metadata, the canonical version file, prerelease feedback intake prompts, artifact publication lineage enforcement, release note or changelog packet composition, or stable-promotion rehearsal summaries through `.github/workflows/create-release.yml`, `.github/workflows/helm-pages.yml`, `.github/workflows/publish-docker.yml`, `.github/workflows/publish-helm-chart.yml`, `.github/workflows/promote-floating-tags.yml`, `.github/workflows/release-dry-run.yml`, `.github/workflows/update-demo-server.yml`, `.github/ISSUE_TEMPLATE/v6_rc_feedback.yml`, `docs/RELEASE_NOTES.md`, `docs/releases/`, `docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md`, `docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md`, `docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md`, `scripts/check-workflow-dispatch-inputs.py`, `scripts/release_control/render_release_body.py`, `scripts/release_control/record_rc_to_ga_rehearsal.py`, `scripts/release_control/internal/record_rc_to_ga_rehearsal.py`, `scripts/release_control/release_promotion_policy_support.py`, `scripts/trigger-release.sh`, and `scripts/trigger-release-dry-run.sh`
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
   Later corrective RCs such as `rc.3`, `rc.4`, and `rc.5` must also carry the live stable
   rollback target and any prerelease trust-root continuity caveat in the
   current release notes, changelog, operator support pack, upgrade guide, and
   release-control evidence record before the release workflow is dispatched.
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
   Customer-facing private Pro RC/GA promotion is part of that same boundary:
   after the `pulse-enterprise` Pro release workflow publishes private archives,
   the private Docker image, and the paid-runtime proof packet, the operator must
   run `scripts/promote_paid_runtime_release_packet.sh --release-dir <proof-packet-dir> --admin-token-file <explicit-token-file> --execute-live`
   from `repos/pulse-pro` before sending customer instructions. That command is
   the canonical live-broker promotion path because it validates the signed proof
   packet, installs the exact manifest on `pulse-license`, runs the customer-path
   live proof, and restores the previous remote manifest if the gate fails. GA
   promotions also require `--allow-ga-prefix`.
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
3. Preserve explicit coverage for installer parity, update planning, and deployment bootstrap behavior when these surfaces change
4. Keep stable and prerelease packet lineage explicit when `docs/releases/` or
   `VERSION` changes: preserve already-shipped RC packets under dedicated
   historical filenames before reusing canonical stable names, keep
   `docs/RELEASE_NOTES.md` and `docs/UPGRADE_v6.md` coherent with that
   lineage, and prove the result through the release-promotion metadata path.
5. Keep paid Pro runtime packaging explicit whenever release runbooks, release
   packets, or paid-user GA guidance changes: public OSS release archives are
   not sufficient proof of paid self-hosted Pro readiness unless the matching
   `pulse-enterprise` Pro artifact/image path is built, identified, and linked
   for paid users.
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
11. Keep forward release signing pinned to an explicit trust root. Governed
   release scripts, Docker release builds, and historical backfill paths must
   accept the active private signing key only alongside a non-secret expected
   public key or equivalent pinned identity, and they must fail closed before
   publication if the signer drifts from that expected trust root.
12. When the governed update signer changes, the canonical operator-facing
   release docs under `docs/releases/` and the governed upgrade guide
   `docs/UPGRADE_v6.md` must state the continuity impact explicitly. Those docs
   must not imply automatic updater continuity from a historical signer unless
   the actual trust-migration path is already shipped and exercised.

## Current State

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
`scripts/installtests/install_docker_sh_test.go` must assert both the repo-root
compose image default and the standalone installer fallback constant. A draft
release workflow failure caused by stale Docker image pins is a release-packet
blocker until the defaults, tests, and evidence record are refreshed from the
new branch head.

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
That same supply-chain boundary also owns the checked-in build roots
themselves. `Dockerfile` must pin its Node, Go, and Alpine bases by immutable
manifest-list digest so multi-arch release builds do not silently drift onto a
different upstream filesystem just because a mutable tag was republished.
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
That same promotion-governance boundary also owns the release-dispatch helpers
and artifact follow-on workflows that consume those same decisions. Demo
deployment, Docker publication, Helm chart publication, Helm Pages release, and
the manual `trigger-release*.sh` entrypoints must all derive their governed
release line from control-plane metadata before they touch public artifacts or
deployment targets, rather than treating tag names or workflow triggers as
enough proof on their own.
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
That same frontend-release boundary also owns shared header-composition proof.
`.github/workflows/release-dry-run.yml` and `.github/workflows/create-release.yml`
must both run the same `lint:headers` audit so a branch that would be rejected
by the real publish workflow cannot pass the governed dry run only because the
rehearsal skipped that header-composition gate.
That same governed demo-deployment boundary now owns target separation between
the public stable demo and the opt-in v6 preview demo. `.github/workflows/create-release.yml`,
`.github/workflows/update-demo-server.yml`, and `.github/workflows/deploy-demo-server.yml`
must route stable tags to the stable demo environment and prerelease tags to a
separate preview environment instead of skipping prerelease demo updates or
reusing the stable runtime in place.
That same preview deployment boundary also owns service-identity isolation and
public-shell parity proof. Preview demo runs must fail closed onto the
dedicated preview service identity instead of defaulting back to the stable
`pulse` instance, must prove that the SSH target reports the governed expected
hostname before any installer or binary copy runs, and demo deploy/update
verification must prove that the public demo HTML serves the same frontend
entry asset as the target service or freshly built preview artifact rather than
treating a passing `/api/health` response as enough evidence that the public
shell actually updated. That proof
must use a deterministic HTML parser for the actual module entry script rather
than brittle escaped shell regex or a first-match asset scrape that can fail
differently over SSH or select the wrong preloaded chunk.
Those same governed demo deploy/update workflows also own the runner-to-host
network path. They must establish the canonical Tailscale connectivity step
before SSH setup so stable or preview targets may stay on governed private
hostnames or Tailscale IPs, rather than silently depending on public SSH
reachability from GitHub-hosted runners.
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
That same boundary also governs install.sh rollback restore targets:
`adapter_installsh.go` may not hardcode `/etc/pulse` for rollback safety
backups or config restore, and must derive the rollback config directory
through that same shared runtime data-dir helper.
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
machine to drive Pulse from Claude Desktop, Claude Code, or other
MCP-speaking clients. The installers fetch a published
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
That same installer boundary now owns instance identity for side-by-side server
installs too: the root `install.sh`, generated update helper, and
`scripts/pulse-auto-update.sh` must preserve an explicitly selected service
identity across install, update, reset, uninstall, and timer/service wiring so
stable and preview Pulse runtimes can coexist on one host without drifting back
onto the default `pulse.service` paths.
That same server-installer boundary also owns release trust fail-closed: the
root `install.sh`, its generated update helper, and
`scripts/pulse-auto-update.sh` must verify downloaded release tarballs and
installer scripts against the pinned release `.sshsig` sidecars before
execution, rather than treating same-origin checksum files as a sufficient
trust anchor. The in-app updater binds to the same invariant: every
release artifact the Go updater fetches before applying or rolling back —
the update tarball in `internal/updates/manager.go::ApplyUpdate`, the
`install.sh` piped into bash by `internal/updates/adapter_installsh.go::downloadInstallScript`,
and the rollback binary tarball in
`internal/updates/adapter_installsh.go::downloadBinary` — must verify
its `.sshsig` sidecar against the pinned `pulse-installer` ed25519 key
(identity `pulse-installer`, namespace `pulse-install`) and refuse to
proceed if the sidecar is missing, malformed, or fails verification. The
in-app and unattended paths must share the same trust root so the UI's
"Update now" button cannot run at a lower bar than the systemd timer.
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
`npm run dev:status`, `npm run dev:restart`, `npm run dev:backend-restart`,
`npm run dev:verify`, `npm run dev:stop`, and `npm run dev:foreground`) rather
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
rather than relying on user-only grants or shared-token inheritance. A
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
