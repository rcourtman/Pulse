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

Operator-facing prerelease notes may accumulate already-landed customer
outcomes before the next candidate is cut. Each added product claim must stay
in the existing customer-facing section and have exact packet proof in
`render_release_body_test.py`; a note about repository-scoped Proxmox backup
review must name both PBS server and datastore rather than implying that a
workload or node filter provides the same boundary.
The next-candidate release notes and changelog must also describe newly stable
integration fields when external receivers need them to consume the release;
packet proof keeps the customer-facing summary and detailed changelog aligned.

A published alpha, beta, or RC is a cohort checkpoint rather than a delivery
vehicle for each individual fix. Public checkpoints at the same maturity stage
on one version line are separated by at least 24 hours of observation.
Compatible fixes and packet evidence may accumulate during that window, while
draft creation and Release Dry Run remain available. Narrow reporter validation
uses immutable issue-and-commit test images and does not force or bypass a new
public preview. Beta is the normal user-validation stage. RC is reserved for a
build believed capable of becoming stable without product changes and must run
the stable-depth integration gate.

Release-note comparison ranges are maturity-specific. Each prerelease compares
against the immediately preceding checkpoint at the same stage. A first beta
compares against the latest alpha when one exists, and a first RC compares
against the latest beta or alpha. The first alpha falls back to the previous
stable release. A stable GA release compares against the previous stable
release, not the final RC, so it can retell the complete release train as a
small number of user-recognizable themes. Stable notes must synthesize the final
product outcomes instead of concatenating prerelease notes or enumerating the
underlying commits.

Release-note synthesis is model-led. The harness supplies the comparison range,
read-only repository and GitHub tools, one unbounded factual investigation, one
independent draft that cannot see the investigation, and a final model that
receives both. The models decide what to inspect, what matters, and how to tell
the release story. The harness owns only the factual release boundary, public
format, safety constraints, and optional per-pass traces. The canonical
release-body validator runs afterward, and one constrained repair pass may
correct formatting without changing the selected meaning. Public bullets stay
individually bounded for readability, but the harness does not impose an item
count that would choose which otherwise-valid user outcomes must disappear.
New customer release notes use plain punctuation and fail validation when they
contain a semicolon or em dash. Already-published packets remain historical
artifacts and retain the punctuation with which they were released.

Visual release-note evidence follows the same model-led boundary. After the
customer story is settled, a model may select no visual views or a bounded set
of views that materially improve it. The harness supplies only safe same-origin
navigation, accessible click and wait actions, deterministic generated data,
and identical rendering of the channel-specific comparison tag and candidate.
It does not prescribe product areas or visual themes. Visual evidence discovery
is separate from structured capture selection, so tool-led investigation is not
forced to share one response with rigid JSON. Capture selection uses native JSON
Schema bounds without prescribing its subject matter. An independent omission
audit checks the customer story against the complete factual account before
visual selection. Selected before and now images are staged as draft release
assets, linked from a `See the difference` section, and must be publicly
retrievable before the activation marker commits publication. A current-only
image is permitted when a truthful before view is not available.
Canonical publish triggers rerun visual evidence discovery and selection for the
exact notes and comparison range being dispatched. A committed visual sidecar is
review material only and cannot substitute for that run. The model may still
select zero captures when its investigation finds that screenshots add no
meaningful customer value. Committed sidecars remain schema-valid review
records and retain the evidence-backed reason for selecting captures or none.

Customer-facing notes use one outcome list for features and fixes. Each visible
change is described once under `What's improved`; a parallel `Fixes` section is
forbidden for packets from `v6.4.0-rc.6` onward because it encourages the same
change to be restated with slightly different implementation detail. Internal
toolchain and architecture work stays in the detailed changelog unless it
changes something users can recognize or act on.

Provider-hosted MSP deploy artifacts must package the provider control plane as
a least-privilege Docker provisioner. The packaged compose/setup path must avoid
whole-host and Docker-data read mounts, expose storage admission only through
narrow marker directories, broker Docker daemon access through the socket proxy,
pin trusted proxy CIDRs to the provider network, block tenant bridge access to
cloud metadata endpoints at the host firewall when possible, and pin the Traefik
TLS floor in the dynamic config.

Installer argument persistence includes repeated `--disk-include` values. An
upgrade must recover those values from both split and `--key=value` service
arguments and reproduce them in the generated systemd command without
discarding existing explicit disk exclusions.

Unified Agent installers persist `--command-authority` as an explicit local
service invariant. Fresh installs default to `monitoring-only`; an explicit
command-enabled install uses `command-capable`; an upgrade of an older service
with no marker writes `legacy` to preserve compatibility. A contradictory
monitoring-only plus command-enabled request fails closed. Linux systemd units
must grant ambient `CAP_SETUID`/`CAP_SETGID` only to the explicit command-capable
PVE profile; fresh monitoring PVE, PBS/PMG, and least-privilege units retain no
such ambient capabilities. Windows service replacement recovers the same
authority marker from the existing service command line before reinstalling it.

The safe Linux collector profile packages `pulse-agent-helper` as a separate
root-owned binary and activates it through a root-owned systemd socket/service
pair. `/run/pulse-agent/helper.sock` is `root:pulse-agent` mode `0660`; the
service has no Pulse URL or token and is sandboxed with `PrivateNetwork=true`,
`RestrictAddressFamilies=AF_UNIX`, `NoNewPrivileges=true`,
`ProtectSystem=strict`, and `ProtectHome=true`. `PrivateDevices=true` is
forbidden because it would hide the block devices required for SMART telemetry.
The collector binary and unit remain root-owned and non-writable by the service
account, while mutable state is confined to the collector-owned state
directory. Ordinary updates preserve an installed profile and may not migrate a
legacy root/full-trust service implicitly; safe-profile migration, health
verification, and rollback are explicit transactions.
The typed-helper profile keeps automatic updates behind a fixed filesystem
transaction: `/var/lib/pulse-agent/update-quarantine` is collector-owned and
read-only to the helper sandbox, `/var/lib/pulse-agent-helper` is root-only
activation state/staging, and the helper has write access to `/usr/local/bin`
solely to atomically replace the protocol-fixed `pulse-agent` target and its
last-known-good copy. The collector can select no privileged source, target,
path, command, or argument, and direct collector-owned replacement is not a
fallback.
Activation persists a pending identity and bounded rollback deadline before
the new binary is trusted. The replacement collector must pass local readiness
and have a fresh authoritative primary report accepted before it sends the
typed commit; helper startup/restart recovery and the watchdog restore the
last-known-good binary when activation is interrupted, expires, or never
commits. Before replacement, the helper proves the signed artifact is the Pulse
agent command, reports the requested advancing version, and cannot be committed
by a different process or executable digest. Commit and rollback durably reap
the fixed staging and quarantine artifacts. A socket-active check is
insufficient: installer migration and the collector both exercise the
versioned helper health protocol.

The same supported Linux systemd profile can install `pulse-agent-runner` only
through the separate `--enable-action-runner` choice, a private token file,
and the already selected typed helper profile. The runner binary, unit,
configuration, credential, health record, and receipt database are root-owned
and independent from collector state. Before any restart or activation, the
installer stages the bearer and environment on the same root-owned filesystem,
synchronizes the complete temporary file, atomically renames it into place,
and synchronizes the containing directory. It disables and runtime-masks an
existing unit before either file changes, without interrupting the
already-running predecessor, and unmask/re-enables it only after both files are
durably complete. The runtime mask suppresses `Restart=on-failure` during the
transaction; persistent disablement keeps it closed after reboot. A write, ownership,
identity, sync, or rename failure cannot expose a partially written live file
or make mixed authority state bootable. Indeterminate recovery must rebuild
both files before re-enable, and readiness failure re-establishes the
stop/disable/runtime-mask fence plus an authoritative inactive-state check
before classification or rewrite. Stop failure or a still-active process never
restores predecessor files beneath that process. Failure leaves the unit
masked, disabled, and backed up. Activation is transactional: the server
keeps the prior credential valid while a bounded replacement registers without
dispatch authority in a separate pending session slot. Pending reconnects can
replace only that slot and cannot evict or interrupt the active dispatch
transport. The runner durably writes an installer-nonce-bound health
marker in pending state, commits activation, and then replaces the marker with
activated state before the installer removes backups. Commit durably writes
the activated token inventory and, under the same inventory transaction,
performs a bounded exact pending-to-active session-map swap; displaced-socket
cleanup happens only after transaction locks are released. A missing or
superseded pending transport causes a durable inventory rollback and conflict.
If that compensating save fails, memory follows the last known durable active
inventory and activation is reported indeterminate so recovery retains repair
material rather than claiming success. The
installer deletes the prior marker before restart, does not trust filesystem
timestamps, and never restores a prior marker. If readiness fails, it stops the
replacement and restores previous runner-only files only when a bodyless self-
cancellation durably removes the exact pending replacement under the activation
transaction lock. Activation conflict, transport/TLS failure, persistence
failure, or admission-fence uncertainty retains the new credential/runtime and returns a
repair-required failure rather than restoring a potentially revoked secret. It
must first atomically and durably restore the exact requested replacement
bearer to the root-only token file; if that write fails, the runner remains
stopped and re-enrollment is required without predecessor restoration.
Disable and uninstall
remove only remediation and leave monitoring running. The action
credential is never placed in argv or reused as the collector token. The
installer persists the canonical enrollment hostname for runner admission. If
the canonical agent ID is already known, it also writes that non-secret ID to
the root-owned runner environment so the separate runner can register before
the collector's first report populates its identity file; the private identity
file remains the fallback for later-generated IDs. The direct binding takes
precedence consistently during runner startup, activation health, and
self-revocation, and issuance rejects identities outside the runner's bounded
action-identity vocabulary. The installer
uses the Go lifecycle client with a root-only token file for exact durable self-
revocation before local teardown, so the bearer secret does not enter argv. If
any runner artifact exists, server unreachability, TLS/auth failure, or missing
credential stops and disables the runner but retains every artifact for retry or
manual server-side revocation. Runner WS and lifecycle HTTP require HTTPS/WSS
with system/custom CA trust or exact certificate-DER pinning; plaintext is
loopback-only and generic installer insecure/curl `-k` is never inherited.
The runner must retain a writable host filesystem because its closed protocol
performs real package, storage, guest, and container mutations;
`ProtectSystem=strict` is therefore forbidden for that unit. This exception
does not apply to the collector or helper and does not relax the runner's
separate identity, credential, network, capability, state, or receipt bounds.

Safe-profile migration is never an ordinary update side effect.
`--safe-profile-inspect` is read-only and reports the current authority,
platform support, provider differences, and unchanged action-runner state;
`--safe-profile-apply` snapshots the collector/helper files and identity before
installing the typed-helper monitoring-only profile; and
`--safe-profile-rollback` restores that committed snapshot without broadening
privilege or changing the independently enrolled runner. These operations fail
closed outside Linux systemd. Appliance, non-systemd, Windows, and macOS
profiles retain an explicitly named legacy/full-trust path until their service,
filesystem, update, helper, and runner boundaries have separate proof.
Apply also fails before mutation when the effective collector fragment differs
from the installer-owned unit or any systemd drop-in is present. Commit requires
the effective non-root/no-ambient-capability unit, a live helper protocol
response, and a registration `lastSeen` newer than the frozen legacy
collector's value. Before the local transition, apply calls the exact
host-bound authority-reduction API and fails closed unless `agent:exec` and
`agent:manage` are durably removed. The rollback manifest restores only the
installer-owned state-root metadata plus the Proxmox registration markers the
apply path can mutate; it never replays collector-controlled descendant paths,
and every rollback strips the legacy command flag rather than reversing the
server-side reduction. Rootful Docker remains an explicit migration
degradation: without a collector-owned, readable and writable rootless runtime
socket, the typed helper supplies summary-only container inventory and the
report carries `collectionMode: typed-helper-summary`. Stats, secondary
inventory, update checks, and actions remain unavailable; the safe profile
never restores Docker by adding the collector to a root-equivalent group.
Safe-profile rootless discovery is deferred until `pulse-agent` exists and
selects across Docker and Podman as one set. Exactly one live socket must be
owned and readable/writable by the collector UID before its URI and runtime
directory enter the service environment. Root-owned, cross-user, unreadable,
remote, and simultaneously usable Docker/Podman endpoints fail closed; the
agent repeats this exact-one admission before the first API probe and on every
reconnect, then requires daemon metadata to attest rootless mode. An exact pin
is recovered only from the root-owned collector service unit when it is not
group/world-writable, so
updates preserve the choice while the socket is offline. The running collector
can move between direct rootless monitoring and typed-helper summary fallback
without a restart or collector action authority. Legacy Docker report-response
commands and autonomous cleanup/update work remain disabled in the safe
collector even while direct rootless monitoring is active. Legacy/root discovery retains
its separate rootful-Docker-first behavior for compatibility.

Release builds and archives carry both helper and runner binaries for the five
Linux targets (`amd64`, `arm64`, `armv7`, `armv6`, and `386`) with checksum,
Ed25519, and SSH signature sidecars. Exact archive/container-context validation
must prove those assets rather than inferring them from collector packaging.
Every canonical candidate also carries one Linux amd64 secure-runtime
qualification packet compiled on a GitHub-hosted runner. That compiler emits
three version-distinct predecessor collectors plus byte-for-byte
reproductions of the ordinary release collector, helper, and runner, a closed
build contract, and portable SLSA compiler provenance. Hosted candidate
assembly rejects the packet unless those three current binaries reproduce the
ordinary payload exactly, then publishes only the predecessor collectors next
to the ordinary current binaries and binds the compiler provenance and build
contract through the normal SBOM, checksum, Ed25519, SSH-signature, immutable
manifest, and assembly-provenance path. The update signing private key remains
absent from compilation and is used only by hosted candidate assembly.
Published prerelease packets automatically enter the separate disposable
Ubuntu/systemd qualification workflow. The release workflow dispatches that
qualification from the exact annotated RC tag, and the child fails unless its
workflow execution ref and SHA equal the peeled tag commit. That workflow consumes the immutable
release assets and four release signatures, runs the canonical twenty-three-scenario
schema-v7 lab, and applies the release-candidate attester against the exact
GitHub release ID, tag, source commit, checksums, compiler provenance, assembly
provenance, and update-key fingerprint. The v7 host starts a rootful Docker
daemon inside the disposable systemd container without mounting the hosted
runner socket or exposing a TCP listener. It imports an offline, source-bound
fixture image and proves legacy inventory, summary-only typed-helper parity,
helper restart continuity, degraded status-only reporting during helper loss,
and complete recovery while the collector remains unable to open the rootful
socket. Its retained receipt remains explicit
self-attestation rather than external security review, and successful packet
production or execution does not change the safe profile from opt-in.
This packaging proof does not establish live platform qualification: helper
update staging/activation/restart/rollback on a real systemd host, real
Docker/Podman and Proxmox action execution, systemd migration rehearsal, and
appliance support remain residual qualification work and must not be described
as complete.

Published exact-version install and rollback guidance must preserve the server
and Unified Agent installer boundary. Supported systemd and Proxmox LXC
deployments use the signed `/bin/update --version vX.Y.Z` server helper;
`/opt/pulse/scripts/install.sh` and release archives' `scripts/install.sh` are
Unified Agent installers and must never be presented as server rollback
commands. Docker guidance instead pins the target image and recreates the
container. The promotion resolver, manual release trigger, rendered release
body, current release packet, and shipped upgrade guide must agree on those
deployment-specific paths.

Images assembled from immutable release payload contexts must restore
executable mode on copied Unified Agent binaries before creating the
architecture-resolved `/usr/local/bin/pulse-agent` link. Detached signature
sidecars remain non-executable. Exact-candidate container qualification must
prove both mode boundaries on the assembled server image before publication,
rather than relying on post-publication image validation. Release validation
may delete invalid assets and rewrite validation annotations only while a
release is still a draft. A post-publication edit is observation, not authority
to mutate or destroy an immutable release; failed revalidation records a
failing status and requires an explicit corrective release path.
The activation marker is part of that complete draft packet: its stored digest
must be checked before publication, and customer convergence is forbidden until
GitHub reports the release immutable and its signed release attestation verifies.
The marker must also retain the verified server and provider control-plane image
digests. Normal and activation-recovery publication both fail closed without
those identities, and convergence may forward only the values read from the
immutable marker to public-container alias promotion.

The accelerated exact-SHA release worker must preserve release-gate fidelity
under its own resource envelope. Bounded frontend static checks and integration
image preparation may overlap, but the full frontend test suite and the
race-enabled backend release suite must run serially. Both suites saturate the
dedicated worker when combined; concurrent execution can stretch otherwise
passing monitoring tests into load-induced failures and must not be used as a
release-latency optimization.

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
13a. `.github/workflows/compile-release-payload.yml`
13b. `.github/workflows/qualify-secure-runtime-release.yml`
14. `.github/workflows/build-and-test.yml`
14. `.github/workflows/create-release.yml`
14. `.github/workflows/deploy-demo-server.yml`
15. `.github/workflows/helm-pages.yml`
16. `.github/workflows/promote-floating-tags.yml`
17. `.github/workflows/promote-private-pro-runtime.yml`
18. `.github/workflows/publish-docker.yml`
19. `.github/workflows/publish-helm-chart.yml`
20. `.github/workflows/qualify-release-containers.yml`
20. `.github/workflows/release-convergence.yml`
21. `.github/workflows/release-dry-run.yml`
22. `.github/workflows/retry-release-convergence.yml`
23. `.github/workflows/update-demo-server.yml`
23a. `.github/workflows/recover-demo-server.yml`
23b. `.github/scripts/recover-demo-runtime.sh`
23c. `.github/scripts/resolve-demo-runtime-profile.sh`
24. `.github/workflows/validate-release-assets.yml`
25. `.github/workflows/install-sh-smoke.yml`
25a. `scripts/check-github-release-immutability.sh`
26. `scripts/release_control/customer_promotion_lease.sh`
27. `pulse-enterprise:.github/workflows/build-pro-release.yml`
28. `pulse-enterprise:scripts/build-pro-binaries.sh`
29. `pulse-enterprise:scripts/build-pro-release.sh`
30. `pulse-enterprise:scripts/validate-pro-release-line.sh`
31. `pulse-enterprise:scripts/validate-pro-release-line_test.sh`
32. `pulse-pro:.github/workflows/promote-paid-runtime-release.yml`
33. `pulse-pro:scripts/validate_paid_runtime_distribution.py`
34. `pulse-pro:scripts/tests/test_validate_paid_runtime_distribution.py`
23. `.github/ISSUE_TEMPLATE/v6_rc_feedback.yml`
23. `docs/RELEASE_NOTES.md`
24. `docs/releases/`
25. `docs/MSP.md`
26. `frontend-modern/public/docs/MSP.md`
27. `docs/UPGRADE_v6.md`
28. `frontend-modern/public/docs/UPGRADE_v6.md`
28. `docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md`
29. `docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md`
30. `docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md`
29. `package.json`
30. `package-lock.json`
31. `frontend-modern/package.json`
32. `frontend-modern/package-lock.json`
33. `frontend-modern/vite.config.ts`
34. `go.mod`
35. `go.sum`
36. `scripts/build-release.sh`
37. `scripts/build-release-binaries.sh`
37a. `scripts/build-secure-runtime-qualification.sh`
38. `scripts/release_build_targets.sh`
39. `scripts/run-release-backend-tests.sh`
40. `scripts/shard_go_tests.py`
37. `scripts/generate-release-notes.sh`
37a. `scripts/capture-release-note-visuals.sh`
37b. `scripts/release_control/capture_release_note_visuals.mjs`
37c. `scripts/release_control/release_note_visuals.py`
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
52. `scripts/uninstall-sensor-proxy.sh`
52. `scripts/install-mcp.sh`
53. `scripts/install-mcp.ps1`
55. `scripts/pulse-auto-update.sh`
56. `scripts/release_control/internal/record_rc_to_ga_rehearsal.py`
57. `scripts/release_control/record_rc_to_ga_rehearsal.py`
58. `scripts/release_control/release_promotion_policy_support.py`
59. `scripts/release_control/resolve_release_promotion.py`
60. `scripts/release_control/mobile_release_gate.py`
61. `scripts/release_control/mobile_release_gate_test.py`
62. `scripts/release_control/live_runtime_proof.py`
63. `scripts/release_control/live_runtime_proof_test.py`
64. `scripts/release_candidate_manifest.py`
65. `scripts/release_control/validate_artifact_release_line.py`
65a. `scripts/release_control/secure_runtime_attestation_v6.py`
65b. `scripts/release_control/secure_runtime_source_manifest_v6.json`
65c. `scripts/release_control/secure_runtime_attestation_v7.py`
65d. `scripts/release_control/secure_runtime_source_manifest_v7.json`
65e. `scripts/installtests/testdata/secure_runtime_docker_fixture.go`
66. `scripts/release_ldflags.sh`
66a. `scripts/release_update_key.go`
67. `scripts/run_cloud_public_signup_smoke.sh`
68. `scripts/run_demo_public_browser_smoke.sh`
69. `scripts/demo_public_browser_smoke.cjs`
70. `scripts/run_hosted_staging_smoke.sh`
71. `scripts/trigger-release-dry-run.sh`
72. `scripts/trigger-release.sh`
73. `scripts/toggle-mock.sh`
74. `deploy/provider-msp/`
75. `deploy/helm/pulse/`
76. `tests/integration/playwright.config.ts`
77. `tests/integration/QUICK_START.md`
78. `tests/integration/README.md`
79. `tests/integration/scripts/bootstrap-hosted-mobile-onboarding.mjs`
80. `tests/integration/scripts/hosted-mobile-token-runtime.mjs`
81. `tests/integration/scripts/hosted-tenant-approval-store.mjs`
82. `tests/integration/scripts/hosted-tenant-runtime.mjs`
83. `tests/integration/scripts/hosted-tenant-runtime-restart.mjs`
84. `tests/integration/scripts/managed-dev-runtime.mjs`
85. `tests/integration/scripts/relay-mobile-token-helper.go`
86. `tests/integration/tests/helpers.ts`
87. `tests/integration/tests/runtime-defaults.ts`
88. `docker-compose.yml`
89. `scripts/install-docker.sh`
90. `scripts/validate-published-release.sh`
91. `scripts/validate-release.sh`
92. `scripts/release_asset_common.sh`
93. `scripts/backfill-release-assets.sh`
94. `.github/workflows/backfill-release-assets.yml`
95. `.github/scripts/check-demo-reachability.sh`
96. `.github/scripts/setup-demo-ssh.sh`
97. `scripts/trigger-stable-patch.sh`
98. `scripts/verify-github-release-integrity.sh`
99. `scripts/verify-release-container-images.sh`
100. `scripts/release_control/verify_release_container_images_test.py`

## Shared Boundaries

`frontend-modern/src/utils/localStorage.ts` is a shared browser-preference key
registry, not deployment state. Workload presentation preferences added there,
including the Proxmox guest-memory comparison basis and per-surface workload
column widths, must remain optional, client-local, and backwards-compatible. A
missing or invalid stored memory basis falls back to the shipped guest-allocation
view; missing or invalid column widths fall back to the responsive table layout.
The shareable `cols` query parameter may temporarily override the viewer's
column layout, but it must not rewrite their stored preference. None of these
presentation values can affect install, upgrade, update, release, or
artifact-selection behaviour.

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
   When the root-running provider proof mutates files on behalf of an already
   running rootless tenant, it must reconcile the full tenant mount tree to the
   Docker manager's configured runtime UID/GID before the next live-runtime
   stage. Credential rotation, handoff, report ingest, and portal-rollup proof
   must not leave owner-only tenant state readable only by the control plane.
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
   Setup must perform its first compose parse with a non-secret placeholder
   when no evaluation file exists yet, pull each resolved digest-pinned image
   without requiring compose secret interpolation, issue the evaluation only
   after those image pulls succeed, and then repeat normal compose validation
   against the installed signed license. Image resolution must accept the
   manifest JSON exposed by current Buildx and retain a human-output digest
   fallback for supported distro-packaged Buildx variants.
   The setup artifact must also generate strong provider secrets when the
   template leaves `CP_ADMIN_KEY` or `CP_TRIAL_ACTIVATION_PRIVATE_KEY` blank,
   enforce minimum admin-key strength and a valid activation signing key before
   compose starts, require `CP_TRUSTED_PROXY_CIDRS` to include the provider
   Docker subnet, create the storage-admission marker directories, and install a
   host-level `DOCKER-USER` rule blocking `169.254.169.254` from tenant
   containers when iptables is available.
   The packaged edge wiring must keep the operator `.env` out of the Traefik
   container: Traefik's environment carries only ACME/DNS material (the
   Cloudflare token passthrough plus the `dns-credentials.env` file that
   `setup.sh` creates with 0600 permissions), never `CP_ADMIN_KEY` or
   `CP_ENTITLEMENT_SIGNING_PRIVATE_KEY`. The wildcard-TLS DNS-01 provider is
   operator-configurable through `ACME_DNS_PROVIDER` (default `cloudflare`):
   with the default provider `CF_DNS_API_TOKEN` is required; with any other
   Traefik dnsChallenge provider, setup must fail closed until that provider's
   credential variables are present in `dns-credentials.env`.
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
   Restore must also fail closed on the size of what it writes. Archive entries
   are attacker-influenced, so the size a tar header declares is a hint used
   only for an early reject, never the bound: extraction must enforce a
   per-entry cap and a cumulative whole-restore cap against the bytes actually
   copied, so a decompression bomb, a sparse entry declaring a huge logical
   size, or a corrupt stream cannot fill the target volume. An entry that
   overruns either cap must fail the restore and remove its partial file rather
   than truncate it into a file that looks complete.
   A restore that fails partway must roll the target back to an empty state.
   The replace gate has already deleted whatever it replaced, so leaving the
   half-written tree behind hands the operator a control plane that looks
   bootable and forces the retry through the replace gate again; the failure
   must instead clear the restore targets it wrote and say so in the error.
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
   Action-runner recovery must keep the root service disabled and runtime-masked
   when its effective systemd target fails the installer-owned FragmentPath,
   drop-in, executable, identity, environment, network, or hardening checks.
   An indeterminate server activation may retain the replacement credential and
   files, but it may restart them only after a fresh daemon reload and complete
   effective-target validation; repair-required state is never permission to
   start a unit the same transaction rejected.
   If re-establishing that repair fence fails after inspection temporarily
   unmasks the unit, the installer must surface the fence failure and retain
   predecessor repair artifacts; it must not claim the unit is masked or
   discard the only local evidence needed for manual recovery.
   A caller-supplied `--state-dir` must remain canonical across rendered
   systemd, launchd, OpenRC, rc.d, SysV, NAS wrapper, bootstrap, and reference
   environment artifacts. Update and uninstall without a repeated custom path
   must discover it from the active process or managed service before looking
   at default-path state; explicit custom-path operations must not fall back
   to another default instance. `connection.env` records the canonical state
   and token-file paths without storing the token value, update rewrites the
   same secure service shape, and uninstall removes the discovered canonical
   directory rather than only `/var/lib/pulse-agent`.
   Post-install verification must not declare server registration
   unconfirmed from a single lookup: the local `/readyz` gate flips before the
   agent's first report cycle completes, so the installer polls the server
   lookup for a bounded retry window before warning. Once the canonical local
   agent ID exists, verification must prefer it over hostname so a previous
   registration cannot impersonate the just-installed process. A 401 or a 403
   caused by missing reporting scope is definitive and short-circuits; a 403
   `agent_lookup_forbidden` during post-start verification is a transient
   first-use ownership race and must keep polling for the new ownership rather
   than falsely rejecting the fresh install credential (#1644).
   When Proxmox integration is enabled, the installer must also report the
   agent's Proxmox registration outcome in its own output: it waits a bounded
   window on the agent state directory, surfaces the reason recorded in a
   `proxmox-<type>-registration-blocked` marker as an installer error with
   remediation, reports success on a `proxmox-<type>-registered` marker, and
   clears those marker families when resetting Proxmox state for a fresh
   registration.
   That verdict is per Proxmox product, not per host. A machine running both
   PVE and PBS registers each product separately and now holds one bootstrap
   grant per canonical type (#1644), so the installer must read the agent's
   `proxmox-detected-types` marker, wait for an outcome from every product it
   names, and print a success or denial line for each. It must not let the
   first marker it finds decide the whole install: a blocked second product
   cannot suppress the first product's confirmed registration, and a confirmed
   first product cannot mask the second's denial. Agents that predate the
   detected-types marker keep the previous first-outcome-wins timing so a
   single-product host does not wait out the full window, and the reset path
   clears the detected-types marker alongside the rest.
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
   Disk exclusions share that continuity contract. `--disk-exclude` accepts
   device names, device paths, and mount-point patterns, and every repeated
   value must survive live-process or service recovery, update re-entry,
   service rendering, and restart without being collapsed or reordered.
   FreeBSD and pfSense updates have the same continuity obligation without
   Linux procfs or systemd: the installer must read live process arguments via
   `ps` (and environment via `procstat` when available), then fall back to the
   installed rc.d service's `command_args` and `PULSE_*` exports. The parser
   must preserve quoted argument values without evaluating service-file shell
   content, and the rewritten rc.d service must use `--token-file` rather than
   retaining a recovered raw token.
   Every process-termination step the installer performs, and every one it
   writes into a generated boot or watchdog wrapper, must match only the agent
   that installer instance owns. `pkill -f` applies its pattern to the whole
   command line and a leading `^` anchors only the start, so a pattern that
   does not bound its far end also matches a co-installed agent whose binary
   name extends the same prefix, and installs, upgrades, and wrapper restarts
   then take that second agent down with no diagnostic. Binary-anchored
   patterns must therefore terminate at a whitespace-or-end boundary, and a
   deliberately path-agnostic sweep must match the process name exactly rather
   than a bare command-line substring.
   Platforms whose agent runs under a generated watchdog wrapper rather than an
   init system must also stop the previous wrapper before starting a new one,
   and must stop it before the agent it supervises. Killing a supervised agent
   while its wrapper still loops merely races the respawn, and appending a
   second wrapper leaves two loops contending for one agent id with no install
   error to show for it. NAS install paths must be symmetric on this point:
   every branch that writes and launches a wrapper owes the same teardown.
   Uninstall and teardown carry the same ordering obligation for the same
   reason: a wrapper left looping while its agent is removed simply restarts
   the agent that was just stopped. Teardown may match wrappers more broadly
   than install, because it must also reach a wrapper invoked by a relative
   path or stranded at a superseded location, but it stays bound at the far end
   so a backup copy of a wrapper, or an editor holding one open, is never a
   termination target.
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
   Independently of the install-time command flag, the unit for **any** PVE
   agent must grant `AmbientCapabilities=CAP_SETUID CAP_SETGID`. `lxc-attach`
   into an unprivileged guest writes `/proc/<pid>/uid_map`, which requires
   `CAP_SETUID` in the parent user namespace; `NoNewPrivileges` removes it from
   the effective set and simultaneously blocks the setuid
   `newuidmap`/`newgidmap` fallback, so the socket probe fails with
   `write_id_mapping: 61 Operation not permitted`. Gating this grant on the
   install-time flag is not sufficient, because command execution is also
   enabled from the server at runtime (`applyRemoteConfig`) without rewriting
   the unit; an agent switched on that way could run commands but never attach
   to unprivileged guests, so Docker inside them silently vanished from the
   Proxmox page. The grant is deliberately narrower than the
   `NoNewPrivileges=false` exception above: it restores only the privilege
   `lxc-attach` needs and leaves the rest of the sandbox intact.

## Extension Points

1. Add or change deployment-type detection, update planning, or apply behavior through `internal/updates/`
2. Add or change release-build metadata injection, Docker build-context allowlists, release artifact assembly, hosted secure-runtime qualification compilation and attestation, governed promotion metadata resolution, artifact release-line validation, post-install live-runtime claim proof, the canonical version file, operator-facing release packet content, model-selected visual release-note capture, prerelease feedback intake wording, historical published-release integrity backfill, release asset validation status publication, download endpoint checksum/signature header proof, end-to-end install.sh smoke against staged or published release assets, or the canonical in-repo v6 upgrade guide through `scripts/build-release.sh`, `scripts/build-release-binaries.sh`, `scripts/build-secure-runtime-qualification.sh`, `scripts/release_build_targets.sh`, `scripts/run-release-backend-tests.sh`, `scripts/shard_go_tests.py`, `scripts/release_asset_common.sh`, `scripts/backfill-release-assets.sh`, `scripts/release_ldflags.sh`, `scripts/release_update_key.go`, `scripts/check-workflow-dispatch-inputs.py`, `scripts/capture-release-note-visuals.sh`, `scripts/release-preflight-worker.sh`, `scripts/run-release-preflight.sh`, `scripts/verify-github-release-integrity.sh`, `scripts/verify-release-container-images.sh`, `scripts/release_control/secure_runtime_attestation_v6.py`, `scripts/release_control/secure_runtime_source_manifest_v6.json`, `scripts/release_control/secure_runtime_attestation_v7.py`, `scripts/release_control/secure_runtime_source_manifest_v7.json`, `scripts/release_control/verify_release_container_images_test.py`, `scripts/release_control/capture_release_note_visuals.mjs`, `scripts/release_control/release_note_visuals.py`, `scripts/release_control/live_runtime_proof.py`, `scripts/release_control/live_runtime_proof_test.py`, `scripts/release_control/mobile_release_gate.py`, `scripts/release_control/render_release_body.py`, `scripts/release_control/resolve_release_promotion.py`, `scripts/release_control/validate_artifact_release_line.py`, `scripts/release_control/record_rc_to_ga_rehearsal.py`, `scripts/release_control/internal/record_rc_to_ga_rehearsal.py`, `scripts/release_control/release_promotion_policy_support.py`, `pulse-enterprise:scripts/build-pro-binaries.sh`, `pulse-enterprise:scripts/build-pro-release.sh`, `pulse-enterprise:scripts/validate-pro-release-line.sh`, `.dockerignore`, `Dockerfile`, `.github/ISSUE_TEMPLATE/v6_rc_feedback.yml`, `docs/RELEASE_NOTES.md`, `docs/releases/`, `docs/UPGRADE_v6.md`, `docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md`, `docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md`, `docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md`, `scripts/validate-release.sh`, `scripts/validate-published-release.sh`, the operator dispatch helpers `scripts/trigger-release.sh` and `scripts/trigger-release-dry-run.sh`, and the governed release workflows `.github/workflows/backfill-release-assets.yml`, `.github/workflows/build-release-candidate.yml`, `.github/workflows/compile-release-payload.yml`, `.github/workflows/create-release.yml`, `.github/workflows/deploy-demo-server.yml`, `.github/workflows/helm-pages.yml`, `.github/workflows/install-sh-smoke.yml`, `.github/workflows/promote-floating-tags.yml`, `.github/workflows/promote-private-pro-runtime.yml`, `.github/workflows/publish-docker.yml`, `.github/workflows/publish-helm-chart.yml`, `.github/workflows/qualify-secure-runtime-release.yml`, `.github/workflows/recover-release-activation.yml`, `.github/workflows/release-convergence.yml`, `.github/workflows/release-dry-run.yml`, `.github/workflows/retry-release-convergence.yml`, `.github/workflows/update-demo-server.yml`, `.github/workflows/validate-release-assets.yml`, and `pulse-enterprise:.github/workflows/build-pro-release.yml`
   The governed release-build surface also includes
   `scripts/prepare-release-container-context.sh` for exact-candidate container
   assembly.
   Operators may configure an external Linux amd64 worker for the two trigger
   helpers. `scripts/run-release-preflight.sh` must resolve an immutable pushed
   commit, stream the worker implementation stored in that commit over SSH,
   and select either the rehearsal or release profile.
   `scripts/release-preflight-worker.sh` must use a dedicated checkout and
   persistent dependency/build caches while running the portable frontend,
   backend, image-build, and browser-smoke checks that historically caused
   costly hosted-workflow restarts. It must not receive signing keys, package
   publication credentials, or any other release authority. An unconfigured
   worker remains an operator acceleration choice, not a new canonical release
   gate; the typed release gates and the self-contained GitHub release workflow
   remain authoritative.
   Worker startup must compare the complete `go`-prefixed toolchain identity
   from `go.mod` with `go env GOVERSION` so a formatting mismatch cannot reject
   an otherwise exact toolchain or conceal a real version drift.
   Race-instrumented Go builds must place `GOTMPDIR` under the worker's
   persistent run directory and remove that bounded scratch directory on exit;
   a small WSL `/tmp` tmpfs must not turn release qualification into a false
   product failure.
   The exact-SHA worker must start frontend quality, backend qualification, and
   integration-environment preparation as independent lanes after producing
   the frontend embed bundle. Its receipt must distinguish elapsed wall time
   from the sum of overlapping phase times.
   The canonical public prerelease workflow may run its credential-free prepare
   job on the PVE compilation identity so hosted-runner allocation cannot hold
   the entire dependency graph. Stable releases must use GitHub-hosted runners
   for preparation, the frontend embed bundle, and the backend race gate
   regardless of whether Windows signing is required or a version-bound
   unsigned-Windows exception applies. Runner selection is a release-channel
   reliability decision and must not be coupled to signing policy. Prerelease
   preparation must release the PVE identity before exact-SHA compilation
   becomes eligible and must use a complete exact-SHA checkout: leaving
   sparse-worktree state in the persistent runner can make the following
   compilation checkout appear complete while required root files remain
   absent.
   Signing, package publication, and release mutation authority must remain on
   hosted jobs. The bundle job must be independent from frontend quality so
   backend and browser-smoke lanes can start as soon as the bundle is available.
   Cross-platform compilation for every release channel runs once on the
   dedicated, credential-free PVE compiler identity using only public embedding
   keys in a separately dispatched GitHub Actions workflow run. The release
   candidate and SignPath workflow run must contain only GitHub-hosted jobs;
   its hosted handoff job may dispatch and wait for the isolated compiler run,
   but the self-hosted job must never appear in the signing run. The compiler
   workflow must execute from the same exact workflow SHA as the parent release
   and must produce one exact-version, exact-source-SHA manifest
   covering the complete frontend and binary payload. That manifest may cover
   canonical relative paths in the payload tree but must reject absolute,
   traversing, or noncanonical names. The upload step must expose GitHub's
   immutable artifact id and archive SHA-256 digest. The hosted candidate job
   must retrieve that exact id, fail closed unless GitHub's artifact API binds
   its name, digest, isolated compiler workflow-run id, and head SHA to the
   exact release source,
   verify the downloaded archive digest itself, and then verify the inner
   payload manifest
   before applying required native binaries, packaging, update-signing, SBOM
   generation, validation, or upload. It must record that verification beside
   the final candidate manifest and must not rebuild the verified payload.
   Every candidate step that reads the requested version under `set -u` must
   bind `inputs.version` explicitly in that step's environment. In particular,
   the exact-SHA compiled-payload verifier must not depend on a version binding
   from an earlier sibling step, because GitHub Actions does not preserve
   step-local environment variables across steps.
   Private signing material and publication credentials must never enter the
   PVE compilation job.
   Post-publication secure-runtime qualification must authenticate before it
   executes. `.github/workflows/qualify-secure-runtime-release.yml` may download
   caller-owned release assets only into a non-executable holding directory.
   `scripts/release_control/secure_runtime_attestation_v7.py` must copy the six
   binaries, four collector signatures, checksum manifest, assembly and
   compiler provenance, and build contract into one private snapshot, verify
   the immutable release/tag/source identity, hosted compiler chain, canonical
   build contract, production update-key signatures, and exact digests against
   those copies, and publish the snapshot only after every check passes. The
   privileged systemd container must mount that exact snapshot read-only and
   must never execute directly from the download directory. The post-run
   attester must consume the same snapshot paths.
   PVE jobs must consume their runner users' persistent local Go and npm caches
   directly; disposable-runner Actions cache restore/save phases must remain
   disabled because archiving those same caches adds network work after the
   useful test or compilation process has already completed.
   Private Pro compilation must additionally bind the exact Pulse and
   pulse-enterprise commits in a manifest-covered identity record. It runs once
   on the dedicated credential-free PVE enterprise compiler for every channel,
   then crosses the same immutable GitHub artifact-id, archive-digest, run-id,
   head-SHA, and inner-manifest verification boundary before any hosted private
   signing or publication step. The compiler must build only the public Unified Agent matrix
   actually embedded in Pro archives plus the Pro server matrix. The public
   frontend build is a required source-checkout prerequisite because every Pro
   server embeds that exact-SHA
   bundle, so it must overlap the public-agent matrix but must not enter the
   transferred payload. Rebuilding the unused public MCP, server, or
   control-plane payload is not part of this boundary. The
   cross-repository handoff must remain manifest-bound, use compressed artifact
   transfer, and let a hosted runner with sufficient free space skip destructive
   image/toolchain cleanup. The hosted private job must verify both SHAs before
   release signing, R2 upload, private-registry publication, or paid-runtime
   staging.
   Full public compilation may likewise overlap the frontend with agent and
   MCP targets, but its matrix wait set must include only active compilation
   children so the independent frontend child cannot be counted as a completed
   binary task. Every matrix child must be joined successfully before the
   compiled manifest is created, and successful frontend completion must be
   joined before any server or control-plane target that consumes the embed
   directory starts.
   Public and Pro server archives must use the shared canonical staging helper
   and may assemble independent target archives concurrently with a bounded
   worker count. The verified dual-SHA Pro path must stage its five archives
   directly from the precompiled public-agent and Pro-server payloads; it must
   not build, sign, and then discard a complete public release packet first.
   Container qualification must consume the verified immutable candidate
   archives through `scripts/prepare-release-container-context.sh`, assemble
   the prebuilt runtime, agent, and provider control-plane targets without
   recompiling source, and compare every embedded executable digest with the
   candidate bytes
   before exercising the same local runtime through the Helm install/upgrade
   smoke. The reusable `qualify-release-containers.yml` workflow owns this
   proof. That reusable qualifier must not accept a caller-selected source
   revision: it inherits the caller event's immutable `github.sha`, checks out
   that commit without persisting credentials, verifies the checked-out HEAD,
   and binds candidate-manifest verification to the same SHA before executing
   repository code. Standalone candidate dispatches and dry runs invoke it inside the
   candidate workflow; publishing releases invoke it as a sibling of inert
   draft staging so qualification and upload overlap without weakening the
   activation join.
   The server executable and its detached Minisign and SSH signatures are one
   architecture-bound payload unit; cross-archive deduplication must allow
   those three files to differ while continuing to reject drift in every
   universal agent, installer, and script input.
   The container payload must validate the three Windows no-extension aliases
   against their exact `.exe` targets and then omit them, because the image
   recreates those aliases deterministically and the immutable payload
   manifest remains regular-file-only.
   Helm Pages convergence must promote the immutable chart artifact produced
   and qualified by the exact create-release run. It must bind that artifact
   to the activated source run, tag, commit, and activation marker, and must
   not repeat chart packaging or the pre-activation kind install/upgrade smoke.
   Release-to-convergence and cross-repository child-run observation should
   use short bounded polls so GitHub indexing cannot add tens of seconds after
   a required exact run or activation marker has already completed.
   Paid-runtime convergence must retain the download-page browser audit, live
   private broker/image proof, and public-runtime mismatch proof, but may run
   the independent browser and public-boundary checks as a sibling of the
   lease-bound broker mutation so customer-path evidence is not serialized.
   Every sibling proof that calls the tailnet-only license or broker endpoint
   must independently establish the pinned Tailscale connection before that
   call. A connection in the mutation sibling cannot supply network or DNS
   state to another hosted job, and the private distribution validator must
   enforce this placement structurally.
   Candidate archive validation must extract the complete required member set
   from each archive in one gzip pass and validate independent platform and
   universal archives concurrently, allowing the hosted runner to schedule
   decompression across all available virtual CPUs.
   This credential-free lane may run against the isolated rootless
   Docker daemon owned by the low-priority PVE build identity; it must not gain
   host-Docker access, signing keys, registry login, or package-write authority.
   The public orchestrator may stage the exact private packet and private image
   concurrently with public qualification when it binds the anticipated tag to
   the immutable public commit, uses a run-scoped R2 prefix, and keeps the live
   paid-runtime broker unchanged. Public readiness must still verify the final
   tag and every public artifact before convergence may activate that inert
   private staging packet for customers.
   The same rule applies to public exact-version Docker images. Once the
   immutable candidate exists, their credentialed publication may overlap
   container qualification and draft creation when release-line validation is
   bound to the anticipated exact 40-character source SHA, verifies that SHA is
   reachable from the governed release branch, and rejects an existing tag at
   any other commit. Exact-version tags are inert staging surfaces; the
   canonical `release_readiness` join must still require candidate container
   qualification, draft release validation, exact Docker publication, installer
   smoke, and every other immutable gate before activation or floating-alias
   promotion. The exact-version server and provider control-plane image builds
   are independent consumers of the same immutable container payload and must
   publish and attest in separate matrix jobs. Each matrix leg independently
   verifies the exact checkout and candidate manifest. After both legs finish,
   the reusable workflow must resolve the `v`-prefixed and unprefixed tags on
   Docker Hub and GHCR to one digest per image, verify each registry's keyless
   provenance against the exact source SHA and reusable signer workflow, and
   export those two digests. Parallel assembly therefore cannot weaken the
   readiness join or leave activation trusting a mutable registry tag.
   The backend runner must compile the race-enabled `internal/api` test binary
   once, enumerate every top-level test from that exact binary, and generate a
   deterministic manifest proving a complete, disjoint partition. Each
   partition must preserve the exact order emitted by that binary; sorting or
   hash distribution is not equivalent while legacy tests still exercise
   package-global state. The planner must prefix-compress the exact test-name
   regex and keep a shard in one test-binary process whenever that encoded
   argument fits below the configured byte ceiling. That ceiling must never
   exceed 120,000 bytes, retaining explicit headroom below Linux's 131,072-byte
   per-argument limit. A shard may split only at deterministic contiguous
   batch boundaries when its compressed exact-name regex exceeds the configured
   ceiling. Tests at those boundaries must initialize their own package-global
   prerequisites rather than inheriting state from a prior process. On the
   dedicated 8-vCPU release worker, three API shards and the remaining Go packages may then
   execute concurrently with isolated data directories. A failed shard must
   fail the complete backend gate and terminate every descendant test process;
   sharding must never select a coverage subset or leave orphan race binaries
   consuming the worker. Auto-sharding must not make a permanent one-shard
   decision from the transient memory peak created by sibling credential-free
   release compilers. On an 8-vCPU worker it may wait up to 120 seconds for
   those bounded compile processes to exit, and it must then admit the widest
   shard count the measured available guest memory supports — 10 GiB for three
   race binaries plus the concurrent package graph, 8 GiB for two — degrading
   the shard count instead of failing the release at admission. Those floors
   are grounded in the 2026-08-21 direct worker probe that measured a ~7.5 GiB
   footprint for the complete three-shard gate (8.9 GiB MemAvailable floor
   from a 16.4 GiB idle start, zero swap); the prior 16 GiB requirement
   exceeded the worker's own idle availability and could fail an otherwise
   healthy release. Shard CPU width must be weighted by planned test volume.
   Top-level tests execute serially inside one test-binary process, so a
   shard's wall time tracks the serial duration of its planned range rather
   than its CPU width; the width that matters is runtime, GC, and race-
   detector headroom for the prefix shard's thousands of unit tests, which
   the ~15-test wait-bound integration tails cannot use (prefix shard
   measured 569s at 2 procs versus 484s at 4 on the 2026-08-21 worker
   probes). The volume-weighted allocation must never oversubscribe the
   worker's vCPUs across concurrent shards. The exact RC.6 graph uses measured named boundaries: the fast
   prefix ends at
   `TestWebSocketOriginAllowsTrustedForwardedHostedOriginIPv6Loopback`, and
   the repeated integration-server tail is divided after
   `TestServerInfoEndpointMethodNotAllowed`. The planner fails closed if those
   anchors disappear or reorder, and the manifest records them while
   continuing to prove exact ordered, complete, disjoint coverage. The backend
   job owns a 40-minute ceiling, preserving post-step and runner-cleanup
   headroom above both the 1,106-second slowest API shard observed during the
   first `v6.4.0-rc.9` qualification attempt and the inner watchdog, while each
   invocation retains the canonical 30-minute Go timeout as protection against
   a stuck package.
   The warm-path release-control performance objective is 15 minutes or less
   from dispatch to definitive publication/convergence. This objective is an
   optimization target, not permission to weaken exact-SHA qualification,
   signing, artifact integrity, installer smoke, or convergence proof.
   Customer convergence should be dispatched as soon as the immutable draft
   and applicable private packet are staged, so its credential-free activation
   waiter can absorb hosted-runner startup while the readiness join finishes.
   That early run remains inert until it verifies the exact public activation
   marker. If the exact source release run terminates without an uploaded
   marker, the waiter must fail promptly without acquiring the promotion lease
   or mutating a customer surface. If GitHub's release API proves the exact
   marker asset is already uploaded but the public download edge still returns
   a transient 404, the waiter must allow a separate bounded propagation grace
   before failing. A publicly readable marker with the wrong immutable identity
   fails immediately rather than consuming that grace.
   The rehearsal diagnostic spec is opt-in by design, so both the hosted
   rehearsal and its worker profile must set `PULSE_E2E_DIAGNOSTIC=1`; invoking
   that spec while leaving it skipped is not browser proof.
   Normal releases are single-build promotions. The exact pushed SHA must
   produce one release candidate with the policy-required native signing lanes
   through `.github/workflows/build-release-candidate.yml` while independent
   release checks run in parallel. `create-release.yml` may stage only that
   candidate after `scripts/release_candidate_manifest.py` verifies its version, source
   SHA, filenames, sizes, and SHA-256 values. The GitHub release must remain an
   unpublished draft until every exact-version customer artifact required by
   the cut has passed its owned proof. It must then be published and publicly
   verified before any mutable customer pointer or live environment advances.
   After complete local packet validation, the GitHub-hosted candidate assembly
   job must issue SLSA v1 provenance over every file represented by the
   pre-provenance candidate manifest. It must preserve the resulting portable
   Sigstore bundle as `release-build-provenance.sigstore.json` before sealing
   the final immutable candidate manifest. The publication job may transport
   only that manifest-bound bundle; it must not replace candidate-build
   provenance with a later publication-bound attestation.
   Standard post-upload validation
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
   A manually dispatched release rehearsal must activate the same
   candidate build whenever its required `version` input is non-empty and must
   apply the same channel-specific native-signing policy as a publish run.
   macOS notarization remains mandatory for both prerelease and stable
   candidates. Windows Authenticode is unavailable for stable candidates from
   `v6.3.2` onward under the standing owner policy until production credentials
   and certificate authorization are explicitly confirmed ready. Prerelease
   candidates and stable candidates under that standing unavailable policy may
   retain checksum and detached-signature verification without Authenticode
   while the release packet explicitly discloses the unknown-publisher warning.
   New stable versions must not require per-version unsigned allowlist updates
   while this state is active. Restoring signing requires a reviewed policy and
   code change after the release owner confirms readiness; external account
   state alone must not change release behavior. When Authenticode is enabled,
   a cheap signing-configuration job
   must report every missing secret for the platforms required by that
   candidate before either platform runner is allocated. Stable Windows signing must use SignPath's GitHub
   trusted-build-system action by default, submit an immutable GitHub artifact
   by id, verify every returned executable, and retain evidence binding the
   request, source SHA, signer identity, and file digests. A repository-secret
   PFX backend is an explicitly selected break-glass fallback only.
   Production SignPath signing requests require manual approval in the
   SignPath UI, so the Windows signing lane must split submission from
   collection: the build job submits the immutable artifact without waiting
   and records the signing request id, and a separate collection job absorbs
   approval latency, downloads the signed result for the recorded request,
   verifies it, and writes evidence. A collection timeout must fail with
   operator re-run guidance, and re-running the failed collection job must
   reuse the recorded request rather than rebuilding or resubmitting.
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
   The `install-sh-smoke.yml` workflow runs end-to-end against staged or
   published release assets in a privileged systemd container. During a
   release cut it downloads the exact draft assets through the authenticated
   GitHub Release API; manual post-publication checks may use the public release
   URL. It downloads `install.sh` and `install.sh.sshsig`,
   runs the README-documented `ssh-keygen -Y verify` step against the
   real signed asset using the README's pinned key, re-checks the
   server-installer banner / `--version)` arg handler / agent-banner
   absence against the published bytes (not just the local build), then
   actually runs `bash install.sh --archive <tarball> --disable-auto-updates`
   inside the container and asserts `systemctl is-active pulse`, a 200
   from `/api/health`, and a version match from `/api/version`.
   `create-release.yml` must call this workflow as a downstream
   `workflow_call` after `validate-release-assets.yml` succeeds and before
   customer activation for every release that is neither a draft-only nor a
   `historical_asset_backfill_only` run; without
   that wiring the smoke gate exists but never protects a release. Draft-only
   release runs are not a publication boundary and must stop after staged
   release validation, skipping the install smoke, Helm chart staging, mutable
   pointer promotion, private Pro publication, and final activation sequence.
   The README's pinned `pulse-installer` ed25519 key must verify
   `install.sh.sshsig` for the published release; this is enforced by
   `scripts/validate-release.sh` at build time and re-verified by
   `install-sh-smoke.yml` against the served asset.
   Provider MSP evaluation installation must use a dedicated
   `pulse-provider-msp-v<version>.tar.gz` release asset, never a source-branch
   archive. The asset must contain the complete `deploy/provider-msp/` bundle,
   stamp the control-plane and tenant Pulse images to the same exact release
   tag before `setup.sh` resolves them to registry digests, and participate in
   the normal immutable candidate manifest, checksum, detached-signature,
   release upload, and activation-read checks. Until such an asset is actually
   published, `docs/MSP.md` and its shipped frontend copy must fail closed and
   describe evaluation onboarding as request-assisted rather than directing
   operators to execute mutable source as root.
3. Add or change root server installer, shell installer, Docker bootstrap installer, Windows installer, container-agent installer, legacy sensor-proxy cleanup, repo-root compose defaults, or auto-update script behavior through `install.sh`, `scripts/install.sh`, `scripts/install-docker.sh`, `scripts/install.ps1`, `scripts/install-container-agent.sh`, `scripts/uninstall-sensor-proxy.sh`, `docker-compose.yml`, and `scripts/pulse-auto-update.sh`
   Canonical server deployment paths also stamp the privacy-bounded outbound
   telemetry deployment label without changing runtime behavior: the image
   defaults to `container_other`, repo-root Compose overrides it to
   `docker_compose`, documented direct Docker commands set `docker_run`, and
   the root server installer writes `systemd` into its generated unit. The
   runtime accepts only the fixed labels documented in `docs/PRIVACY.md` and
   must collapse an arbitrary operator value to its container/binary fallback
   rather than exporting an image name, path, command, URL, or free-form text.
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
   Unattended update execution must fail closed on service availability
   (#1630): `scripts/pulse-auto-update.sh` `perform_update` must leave the
   service running on every exit path when it was active before the attempt,
   including the installer-exits-nonzero rollback branch. The generated
   `pulse-update.service` gates on `ExecCondition=systemctl is-active`, so a
   service left stopped also silently disables every future unattended run.
   This is enforced by a `service_was_active`-guarded restart in each rollback
   branch plus the `ensure_service_restarted` RETURN-trap backstop, and pinned
   by `scripts/installtests/pulse_auto_update_test.go`
   (`TestPerformUpdateRestartsServiceWhenInstallerFails`,
   `TestEnsureServiceRestartedHonorsPriorServiceState`) and
   `scripts/tests/test-pulse-auto-update.sh`. For the same reason, root
   `install.sh` writes outside the hardened update unit's writable set
   (`ProtectSystem=strict` with `ReadWritePaths` covering the install dir,
   config dir, `/tmp`, the auto-update helper's directory and the unit
   directory — everything else, including `/bin` and `/etc/profile`, stays
   read-only) must be idempotent and non-fatal warnings rather than errexit
   aborts: the `/bin/update` helper heredoc, the PATH appends to
   `/etc/profile` and `/etc/bash.bashrc`, and the `/usr/local/bin/pulse`
   convenience symlink (`install_binary_symlink`, which must also keep an
   already-correct link without rewriting it). An abort in any of these kills
   the installer after the new binary is installed and the service is
   stopped, landing in the rollback branch above; transient read-only
   remounts hit the same paths on unhardened installs. Pinned by
   `scripts/installtests/root_install_sh_test.go`
   (`TestRootInstallScriptUpdateHelperWriteIsNonFatalOnReadOnlyPath`,
   `TestRootInstallScriptBinarySymlinkIsIdempotentAndNonFatal`) and
   `scripts/tests/test-install-update-resilience.sh`.
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
   while the post-update card must offer the release's categorized user-facing
   change sections (`Added`, `Improved`/`Changed`, `Fixed`, `Security`,
   `Breaking changes`, `Deprecated`, or `Removed`) as a changelog once per
   later installed release. Automatic release communication is limited to a
   compact non-blocking update notice; the detailed changelog may open only
   after explicit operator action. Preparing that notice records the version
   immediately so a reload cannot turn it into a recurring prompt. When the
   one-time telemetry disclosure owns the same session, it suppresses the
   lower-priority release notice instead of creating consecutive notices.
   `frontend-modern/src/utils/localStorage.ts` owns that browser-session notice
   reservation boundary so the release notice, telemetry disclosure, and
   GitHub gratitude prompt cannot create a one-two sequence. The post-update
   surface must not reuse the Highlights summary as its content,
   and must stay silent for a first baseline, malformed or development
   versions, missing releases, and releases without categorized changes.
   `Highlights` remains a pre-update overview only: release rendering
   keeps it to at most three short plain-text bullets of no more than 140
   characters each, with links, code, issue references, and nested structure
   reserved for the categorized or full release notes.
   The same post-update communication boundary owns the one-time schema-v2
   telemetry payload notice. It must use a non-blocking shared notice banner,
   appear only for existing installations on a published build, stay silent
   for fresh installs and development/source builds, persist acknowledgement,
   and provide direct payload-preview, disable, and privacy-disclosure actions.
   The corresponding next-release disclosure must enumerate the added coarse
   signal categories and exclusions without inventing a release version before
   the packet is cut.
5. Add or change local dev-runtime orchestration, managed ownership, browser-runtime proof wiring, frontend/backend coherence diagnostics, canonical developer entry wrappers, deterministic dev auth seeding, dependency manifest floors, frontend build chunking, or dev-runtime helper control surfaces through `scripts/hot-dev.sh`, `scripts/hot-dev-bg.sh`, `scripts/lib/hot-dev-runtime.sh`, `scripts/lib/hot-dev-auth.sh`, `scripts/dev-deploy-agent.sh`, `Makefile`, `package.json`, `package-lock.json`, `frontend-modern/package.json`, `frontend-modern/package-lock.json`, `frontend-modern/vite.config.ts`, `go.mod`, `go.sum`, `scripts/dev-check.sh`, `scripts/toggle-mock.sh`, `scripts/clean-mock-alerts.sh`, `scripts/dev-launchd-setup.sh`, `scripts/dev-launchd-wrapper.sh`, `scripts/run_demo_public_browser_smoke.sh`, `scripts/demo_public_browser_smoke.cjs`, `scripts/com.pulse.hot-dev.plist.template`, `tests/integration/scripts/managed-dev-runtime.mjs`, `tests/integration/playwright.config.ts`, `tests/integration/tests/helpers.ts`, `tests/integration/tests/runtime-defaults.ts`, `tests/integration/README.md`, and `tests/integration/QUICK_START.md`
   First-run browser helpers are part of that dev-runtime proof boundary. They
   must preserve the setup-created API token in the shared runtime state, prefer
   deterministic token authentication after setup, and may use the server setup
   API as a fallback only when the UI wizard fails to complete cleanly under the
   managed hot-dev runtime.
   The built `index.html` must not modulepreload lazy route chunks: the SRI
   plugin runs with `preloadDynamicChunks: false` so cold start fetches only the
   entry and its static vendor imports (route-level code splitting stays
   effective on slow devices), while dynamic-import integrity remains enforced
   through the generated import map `integrity` block. Reintroducing whole-app
   preloading is a governed regression, not a tuning knob.
   `frontend-modern/scripts/check-bundle-size.mjs` pins this posture against the
   built output (modulepreload links limited to the entry's static imports,
   import map `integrity` coverage of every built JS asset) and is the accepted
   build-output verification proof for `frontend-modern/vite.config.ts` changes
   under the `frontend-build-output` path policy, alongside the dev-runtime
   orchestration proofs for dev-server-facing edits to the same file.
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
6. Add or change governed release-promotion workflow inputs, operator-facing promotion metadata, the canonical version file, prerelease feedback intake prompts, artifact publication lineage enforcement, release note or changelog packet composition, stable-promotion rehearsal summaries, or the optional exact-SHA external-worker acceleration through `.github/workflows/create-release.yml`, `.github/workflows/helm-pages.yml`, `.github/workflows/promote-floating-tags.yml`, `.github/workflows/promote-private-pro-runtime.yml`, `.github/workflows/publish-docker.yml`, `.github/workflows/publish-helm-chart.yml`, `.github/workflows/recover-release-activation.yml`, `.github/workflows/release-convergence.yml`, `.github/workflows/release-dry-run.yml`, `.github/workflows/retry-release-convergence.yml`, `.github/workflows/update-demo-server.yml`, `.github/ISSUE_TEMPLATE/v6_rc_feedback.yml`, `docs/RELEASE_NOTES.md`, `docs/releases/`, `docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md`, `docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md`, `docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md`, `scripts/check-workflow-dispatch-inputs.py`, `scripts/release-preflight-worker.sh`, `scripts/run-release-preflight.sh`, `scripts/release_control/mobile_release_gate.py`, `scripts/release_control/mobile_release_gate_test.py`, `scripts/release_control/render_release_body.py`, `scripts/release_control/validate_artifact_release_line.py`, `scripts/release_control/record_rc_to_ga_rehearsal.py`, `scripts/release_control/internal/record_rc_to_ga_rehearsal.py`, `scripts/release_control/release_promotion_policy_support.py`, `scripts/trigger-release.sh`, and `scripts/trigger-release-dry-run.sh`
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
   From the release after `v6.4.0-rc.1` onward, that renderer also owns a
   customer-facing communication contract. Public notes lead with one short
   outcome paragraph, use a scannable `What's improved` section with no more
   than six concrete items, keep fixes symptom-led, and reserve `Before you
   upgrade` or `Known issues` for information users must act on or understand.
   Qualification counts, readiness assertions, release gates, workflow
   narration, artifact identity, and promotion metadata stay in governed
   workflow summaries and evidence records rather than the public changelog.
   The renderer appends only concise `Install` and exact `Roll back` sections;
   `docs/releases/RELEASE_NOTES_TEMPLATE.md` and
   `scripts/generate-release-notes.sh` are the canonical manual and generated
   authoring entry points for this shape.
   `docs/UPGRADE_v6.md` and its shipped frontend copy
   `frontend-modern/public/docs/UPGRADE_v6.md` must also stay byte-aligned with
   the current RC support packet so upgrade guidance does not keep pointing
   operators at retired rollout/support docs after a later RC packet is
   prepared.
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
   for every non-draft v6 release, `create-release.yml` must call the private
   `rcourtman/pulse-enterprise` `Build Pro Release` workflow as soon as the
   governed tag and unpublished draft exist, in parallel with public asset
   validation. It must pass the exact public tag/version, set
   `upload_to_r2=true` and `publish_docker_image=true`, and wait for the exact
   private image and signed R2 packet to succeed. The dispatcher must request
   workflow-run details from GitHub and poll the exact returned run ID; it must
   never infer its child from the newest matching workflow/branch/timestamp,
   because version-scoped release concurrency and manual dispatches can overlap.
   Only after public release asset validation, staged install smoke, exact
   public Docker publication, exact Helm OCI publication, durable convergence
   dispatch, and the publicly readable activation-commit marker may the
   convergence run call the private `rcourtman/pulse-pro` `Promote Paid Runtime
   Release` workflow with the same version and R2 prefix.
   The promotion workflow downloads the signed proof packet and runs
   `scripts/promote_paid_runtime_release_packet.sh --release-dir <proof-packet-dir> --execute-live`
   from `repos/pulse-pro`. That command is the canonical live-broker promotion
   path because it validates the signed proof packet, installs the exact
   manifest on `pulse-license`, runs the customer-path live proof, and restores
   the previous remote manifest if the gate fails. GA promotions also require
   `--allow-ga-prefix`. A failed private build leaves the GitHub release
   unpublished. A failed live promotion leaves the already committed GitHub
   release public, restores the previous broker manifest, and fails the
   separately retriable convergence workflow. It must not turn the activation
   workflow red or imply that GitHub publication rolled back.
   The activation marker must bind the exact staged R2 prefix. The Pulse caller
   must pass that prefix with its exact customer-promotion lease SHA and
   convergence run ID to `pulse-pro:.github/workflows/promote-paid-runtime-release.yml`.
   That mutator must reject missing or stale ownership, verify the live public
   Pulse lock ref, lease commit message, active workflow run, and activation
   marker before packet handling and again immediately before broker mutation.
   Those public cross-repository REST reads must be deliberately unauthenticated
   with bounded retry rather than depend on the repository-scoped `pulse-pro`
   `GITHUB_TOKEN`. Its Actions concurrency key may serialize only the validated
   Pulse lease SHA. That makes copied valid dispatches target-identical and
   sequential without letting arbitrary invalid values replace the legitimate
   pending child. The ref lease remains the cross-release serialization primitive.
   A promotion-only failure must be recoverable by rerunning the complete
   convergence workflow: the paid-runtime R2 prefix is derived from
   run-stable values (the run's creation date and run id, never the
   wall-clock date at attempt time), and the private build dispatch sets
   `reuse_existing_packet=true` so `Build Pro Release` validates a complete
   signed packet already present at that prefix, skips the rebuild, and lets
   the promotion re-execute against the packet the earlier attempt uploaded.
   A rebuilt packet from identical inputs is waste and lineage churn; a
   non-empty prefix that fails packet validation must fail the private build
   instead of being overwritten.
   A successfully uploaded `release-activation.json` asset is the irreversible
   release commit, not the earlier `draft=false` PATCH by itself. Before that
   marker, `activate_release` must verify the draft identity and every required
   public asset and return failures to draft quarantine. The convergence run is
   dispatched first but must wait for a marker bound to the exact tag, target
   commit, release ID, source release run, and convergence run. Successful
   marker upload is the irreversible commit because convergence can observe it
   immediately; activation-side read-back failure after upload must not
   quarantine the release. Once that marker is public, the
   release remains committed and mutable-surface failures become explicit
   convergence debt rather than a fictional release rollback.
   GitHub retains historical `published_at` metadata when a release is returned
   to draft. Retry eligibility and staged activation identity must therefore use
   the current `draft=true` state plus the absence of the irreversible marker,
   rather than treating a retained timestamp as current publication. A draft
   that already contains `release-activation.json` must never be retargeted or
   resumed. Checkout-free activation jobs must also pass the repository
   explicitly to `gh release upload`; local git discovery is not a valid
   dependency at the irreversible marker boundary.
   The same explicit-repository rule applies to reusable convergence jobs that
   check out only a nested mutation branch such as `gh-pages`: every `gh
   release` read or write must pass `--repo "${GITHUB_REPOSITORY}"` rather than
   depending on current-directory repository discovery.
   A failed activation after immutable readiness has passed may use the
   activation-only recovery workflow. Recovery must accept only a completed
   failed `create-release.yml` run whose failures are confined to activation,
   require the successful `release_readiness` DAG join as the canonical proof
   that every immutable gate succeeded, and reject every failure outside the
   activation boundary. Recovery must not duplicate reusable-workflow display
   names as a parallel gate catalog. It must revalidate GitHub's stored
   asset digests against that source run's unexpired candidate manifest, and
   require the same draft release ID, tag, target commit, and absent activation
   marker. It then dispatches a fresh durable convergence owner and repeats the
   activation commit without rebuilding or replacing any candidate artifact.
   Because a newly dispatched Actions run can be visible before its workflow
   name and display title are coherently indexed, both normal and recovered
   activation use a bounded metadata-propagation wait before rejecting the
   convergence owner; a terminal owner still fails immediately.
   Once recovery publishes the exact activation marker, a later failed-job
   rerun of the source release workflow must treat that committed state as an
   idempotent success rather than reject it merely because the release is no
   longer a draft. That adoption must fail closed unless the public marker
   still binds the source run's exact tag, target commit, release ID, R2 prefix,
   and numeric convergence owner; the marker names a completed successful
   `recover-release-activation.yml` dispatch for that same tag and source run;
   and its convergence owner is an exact mainline
   `release-convergence.yml` dispatch for the same lineage. An unrelated public
   marker, an unqualified recovery run, or mismatched convergence metadata must
   remain a release failure and must never be overwritten by the rerun.
   Every convergence job that calls a reusable workflow must explicitly grant
   all permissions requested by that callee. In particular, Helm Pages
   convergence requires both `actions: read` to retrieve the exact source-run
   chart and `contents: write` to update the versioned Pages index; omitting a
   required caller permission is a workflow startup failure, not retriable
   customer-surface debt.
   One immutable-readiness join must cover the staged release packet, staged
   install smoke, exact public Docker images, exact Helm OCI chart, and (for
   v6) the exact Pro image and signed packet. After that join, activation must
   publish the marker before the Docker floating aliases,
   paid-runtime broker manifest, public Helm Pages index, or stable demo may
   advance. The separate convergence workflow must hold the global
   `release-customer-promotion-lock` ref lease across those mutations. This
   avoids version-scoped release runs racing global customer state and avoids
   GitHub Actions concurrency's replaceable pending slot. Under the lease,
   committed releases are compared per stable/RC channel. Every committed
   release must add its chart to Helm Pages under the lease, including an older
   release that finishes after a newer one. Superseded runs skip only Docker
   aliases, paid-runtime broker state, and stable demo deployment, so overlapping
   or out-of-order completion cannot move a rollback-prone channel backward or
   omit a committed chart version. The reusable alias, Pages, and demo mutators
   must not expose direct workflow dispatch paths that bypass the lease; the dry
   run retains non-mutating demo verification. A failed surface fails only convergence;
   the full workflow is retried so it reacquires the lease and safely replays
   all idempotent surfaces.
   The activation marker preserves its original convergence run ID. Once that
   run is completed, a fresh manual convergence dispatch from fixed workflow
   code may adopt the same immutable tag, source run, target commit, release ID,
   R2 prefix, and activation-marker digest. Adoption happens only after lease
   acquisition and writes a unique owner record into the exact lease commit.
   The record name includes the new run ID and run attempt; downstream demo and
   paid broker mutation must read it from the passed lease commit and verify the
   exact filename and SHA-256. A unique evidence tag retains that exact commit
   after the active lease ref is released. The release remains sealed after
   publication, while every successor gets immutable, run-scoped Git evidence.
   Reading a floating ref or a clobbered constant record is forbidden because
   cached prior bytes could authorize stale ownership.
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
   metadata-only release note. If a customer-visible runtime fix lands after a
   stable packet is prepared but before publication, the next dispatch must
   update both the release notes and changelog and bind that named fix in the
   release-promotion packet proof.
   Release integration failures must leave enough evidence to classify the
   failure after the compose stack is torn down. `create-release.yml` must
   upload the Playwright report and a `release-integration-failures` artifact
   containing Playwright `test-results/` plus
   `release-integration-diagnostics/docker.log`; that Docker log must capture
   container state and the Pulse test server plus mock GitHub server logs.
   The release integration job must also name at least one current,
   non-quarantined browser spec. For the v6.1.0 release line that proof is
   `tests/66-organization-sharing-approval-ui.spec.ts`; the job must not point
   back at the wholly quarantined `tests/03-multi-tenant.spec.ts`, because
   Playwright would correctly select zero tests and turn every publication
   attempt into the same deterministic harness failure.
7. Preserve release-matched installer and Helm operator documentation links through `scripts/install.sh`, `.github/workflows/helm-pages.yml`, `.github/workflows/publish-helm-chart.yml`, and the chart metadata itself so deployment guidance and packaged chart metadata do not drift back to branch-tip `main` docs when a release line or promoted tag already exists.
   The same governed Helm boundary also owns `deploy/helm/pulse/` itself:
   chart metadata, default values, templates, and generated chart docs must
   stay on the validated release line rather than mutating `main` or packaging
   from whatever branch GitHub happened to check out.
   The chart's `agent.enabled=true` workload must point at an image that is
   actually published. The default `agent.image.repository` must be the main
   `rcourtman/pulse` image (the only agent-capable image `publish-docker.yml`
   pushes; the same workflow also pushes the MSP `pulse-control-plane` image,
   which is not an agent runtime); the agent template must override the server
   ENTRYPOINT via
   `agent.command` so the pod runs as a unified agent; and the runtime stage
   of `Dockerfile` must ship an arch-resolved `/usr/local/bin/pulse-agent`
   symlink that picks `pulse-agent-linux-{amd64,arm64,armv7}` per `TARGETARCH`
   so a single command default works across multi-arch nodes. The unmaintained
   `ghcr.io/rcourtman/pulse-agent` is forbidden as a chart default: no release
   workflow publishes it, and the only release tag it ever carried is a stray
   `v6.0.0-rc.3`. No workflow may reference that package at all, buildcache
   refs included — a `cache-to` there recreates an empty package in the
   repository's Packages sidebar that reads like a pullable agent image. The
   `agent_runtime` build cache belongs at
   `ghcr.io/<owner>/pulse:agent-buildcache`, alongside the runtime stage's own
   `pulse:buildcache` tag. `scripts/validate-release.sh` must assert the
   `/usr/local/bin/pulse-agent` symlink exists, points at one of the
   supported Linux arch binaries, and is executable in the published image.
   `create-release.yml` must trigger `publish-helm-chart.yml` via an explicit
   `workflow_call` after `validate_release_assets` succeeds and before final
   activation, not rely on
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
   contains `version: <version>` before the workflow exits green. It must be an
   awaited `workflow_call` from `create-release.yml`, not an asynchronous
   `workflow_run` child of Docker publication. It must package from the exact
   validated release tag without committing generated metadata back onto the
   governed source branch. It must refuse a draft or quarantined GitHub release,
   then prove the public Pages repository can resolve and download that exact
   chart version as a post-activation promotion gate.
   After pushing the OCI chart, `publish-helm-chart.yml` must prove the
   pushed chart is readable from GHCR without registry credentials by logging
   out of `ghcr.io` and running `helm show chart` against the versioned chart
   reference. The workflow must not mask package-visibility drift with
   best-effort GitHub Packages visibility API calls: invalid or unauthorized
   visibility endpoints create false success and noisy release logs, while the
   unauthenticated chart read is the customer-facing availability contract.
   `create-release.yml` must apply the same explicit `workflow_call` to
   `promote-floating-tags.yml`. The legacy `workflow_run` chain off
   `publish-docker.yml` is forbidden because it creates a second, implicit
   owner that may move aliases outside the activation sequence. The exact-image
   publisher must publish only immutable version tags; `promote-floating-tags`
   is the sole owner of `rc`, `latest`, major, and major/minor aliases for both
   `pulse` and `pulse-control-plane` on Docker Hub and GHCR. It must expose
   `workflow_call` inputs for the tag, channel, source SHA, and both
   activation-committed image digests; refuse a draft or quarantined GitHub
   release; and depend on successful activation in the create-release wiring.
   Immediately before mutation it must re-resolve all exact-version tags and
   reverify both registries' provenance against those committed identities.
   Alias creation must use registry-specific `image@sha256:...` sources rather
   than dereferencing the mutable version tag again, and every resulting alias
   must resolve back to the expected digest before convergence succeeds.
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
4. Disabling SSH host-key verification, enrolling unknown keys, or hiding remote trust failures in unattended legacy cleanup

## Completion Obligations

1. Update this contract when canonical deployment or installer entry points move
2. Keep deployment runtime and shared API proof routing aligned in `registry.json`
3. Preserve explicit coverage for installer parity, update planning, and deployment bootstrap behavior when these surfaces change. Shell installer update recovery changes must keep `scripts/installtests/install_sh_test.go` covering both persisted `connection.env` recovery and legacy running-process/service recovery across Linux and FreeBSD/rc.d, including single-dash v5 agent flags, non-procfs process inspection, and the rule that upgraded service args use `--token-file` instead of raw `--token`.
   Legacy sensor-proxy cleanup changes must keep `scripts/installtests/uninstall_sensor_proxy_test.go` covering strict host-key options, explicit known_hosts isolation, missing or mismatched trust failure, and the SSH-free local-only path, while `scripts/release_control/ssh_host_key_policy_test.py` continues to ban verification bypasses repo-wide.
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
   must not depend on floating `:latest` base tags. Workflow dispatch inputs,
   secrets, and attacker-controlled event metadata must enter generated runner
   scripts through explicit environment variables; `${{ }}` interpolation in
   a `run` program is not an acceptable data boundary.
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
14. Keep `docs/MSP.md` and `frontend-modern/public/docs/MSP.md` byte-synchronized
   and fail closed on provider MSP evaluation installation whenever a signed,
   evaluation-capable exact-version release asset is not yet published. Any
   future installation command must verify the dedicated archive with the
   pinned release key before extraction or root execution.
15. Keep `docs/UPGRADE_v6.md` and
   `frontend-modern/public/docs/UPGRADE_v6.md` byte-synchronized. The shipped
   copy is part of every release packet, and `docsLinks.test.ts` must fail when
   release preparation changes only one side.
16. Keep generated install and rollback instructions deployment-specific.
   `scripts/installtests/build_release_assets_test.go` must reject a release
   trigger, promotion resolver, rendered release body, current upgrade guide,
   or current release packet that routes systemd/LXC rollback through the
   Unified Agent installer, and must retain explicit Docker image guidance.

## Current State

### Candidate notes cover restored Proxmox node network details

The current v6.4 candidate notes record that configured PVE node interface
names and IPv4/IPv6 addresses are visible without a linked agent. This is a
runtime presentation addition only: it changes no installer permission,
artifact, upgrade, rollback, signing, or promotion boundary.
`frontend-modern/src/utils/__tests__/docsLinks.test.ts` pins the candidate-note
line so the release packet cannot silently omit the user-visible restoration.

Pulse v6.2.1 is the first active published release with the signed,
evaluation-capable provider MSP deploy bundle. Public MSP guidance pins that
exact version, verifies its detached SSH signature and checksum before root
execution, and continues to reject the moving `main` archive. Future release
guidance may advance the pin only after the new exact-version provider asset
and sidecars pass the same publication barrier.

The shell installer's legacy/root container-runtime discovery prefers a working
rootful Docker daemon over any rootless socket (#1647).
`discover_rootless_container_runtime` in `scripts/install.sh` first calls
`system_docker_runtime_is_active` — a `docker info` check with
`DOCKER_HOST`/`CONTAINER_HOST` stripped, falling back to probing
`/var/run/docker.sock` directly — and returns without discovering anything
when the system daemon answers. Only when no rootful Docker is live does it
glob `/run/user/*` for rootless docker/podman sockets and pin
`PULSE_DOCKER_RUNTIME`/`CONTAINER_HOST`/`PODMAN_HOST`/`XDG_RUNTIME_DIR` into
the agent service environment. This applies to auto-detection and explicit
`--enable-docker` runs alike, so a transient socket-activated rootless Podman
API socket can no longer capture the agent unit on hosts whose real runtime is
rootful Docker.

The typed-helper safe profile deliberately does not reuse that broad
`/run/user/*` selection. It waits until the dedicated collector account has
been provisioned, filters sockets by exact UID ownership and collector
read/write access, combines Docker and Podman candidates into one ambiguity
set, and writes service environment pins only for one exact match. The runtime
independently enforces one live collector-owned Unix endpoint before probing
daemon metadata, requires the selected daemon to attest rootless mode, and
repeats the check when recovering a lost socket. An exact standard-path pin
from a root-owned existing service unit survives update-time socket downtime;
the live process can recover direct monitoring from helper summary mode and can
fall back to a complete helper summary after repeated direct loss.

Stable and stable-dry-run callers now select SignPath as the canonical Windows
Authenticode backend. The reusable builder fails fast on missing configuration,
submits the GitHub-hosted unsigned artifact through the pinned official action,
verifies all returned executables, and stores request/source/signer/digest
evidence beside the candidate manifest. Release Dry Run now has a terminal
verdict covering the exact-SHA candidate and no-mutation demo lane. Stable
rehearsal `29927692302` confirmed that the external SignPath project was not
configured and stopped without creating a public release. The release owner
subsequently approved and exercised a `v6.1.0`-only unsigned-Windows exception.
On 2026-07-23 the owner separately approved a `v6.1.1`-only exception for the
emergency patch addressing active customer update harm. The new decision must
flow through the normal exact-SHA candidate, checksum, detached-signature,
manifest, published-digest, and definitive-verdict controls with an explicit
owner reason and public Unknown Publisher disclosure.
On 2026-07-26 the owner approved the same bounded path for `v6.1.2` because the
company's SignPath verification application remains in processing. This
decision is limited to `v6.1.2`, retains the exact-SHA and integrity controls,
and requires the public release notes to disclose that Windows binaries are
not Authenticode-signed and may show Unknown Publisher.
Every caller of the reusable release-candidate builder must delegate
`actions: write` alongside `contents: read`; only the hosted compiler-handoff
job receives that effective permission so it can dispatch the isolated PC
workflow, while the Windows signing and final assembly jobs narrow themselves
back to `actions: read`. GitHub validates that nested permission even when a
prerelease skips Authenticode signing.
SignPath Foundation also requires every job leading up to an open-source
signing request to execute on GitHub-hosted runners. Stable release
preparation, the parallel frontend bundle, backend qualification, signing
configuration, and the Windows build/submission dependency chain therefore
remain GitHub-hosted regardless of an unsigned-Windows exception. The
credential-free PVE compiler executes in an entirely separate workflow run and
never supplies the artifact submitted to SignPath. The hosted release workflow
dispatches it by exact source SHA, validates the returned compiler-run identity,
and only the hosted final assembler joins its separately verified output after
native signing. This keeps every job in the SignPath workflow run hosted rather
than relying on a sibling-job interpretation of SignPath's OSS rule. Rehearsals `32631653966` and
`32635525554` lost different matrix compiler processes on the PVE runner under
four-way compiler and frontend pressure. The single-build compiler now admits
only a worker with explicit memory/disk headroom, limits the matrix to two
concurrent workers, uploads one immutable GitHub artifact, and lets the hosted
assembler verify the API identity, archive SHA-256, exact source/version inner
manifest, and complete payload without recompiling it. Stable container and Helm
qualification also runs hosted after rehearsal `32636149901` exhausted the
persistent PVE build runner's disk while assembling the first candidate image;
the candidate payload had already verified successfully. Prerelease container
qualification retains PVE acceleration so runner-capacity maintenance remains
separate from the stable release boundary.

The provider MSP proof command validates its handoff target with the same
host-local redirect contract as runtime token minting and exchange. Proof input
must reject absolute, scheme-relative, backslash-authority, encoded-separator,
and control-character targets before constructing the handoff request.

The preceding stable `v6.2.0` cut set the repo-root `VERSION`, repo-root
`docker-compose.yml` image default, `scripts/install-docker.sh` fallback, and
Helm chart release metadata to the same `6.2.0` release version. This stable
minor release uses `promoted_from_tag=v6.2.0-rc.11`,
`rollback_version=v6.1.2`, and the explicit version-bound decision recorded on
2026-08-09 to waive the remainder of the normal 72-hour soak. The workflow
input `hotfix_exception=true` transports that approved waiver through the
shared promotion resolver; it does not reclassify v6.2.0 as a patch hotfix.
The exact stable `main` SHA must pass the no-publication dry run before the same
SHA is dispatched through the single-build publish workflow. The release moves
stable/latest install pointers and stable semver aliases only after the exact
public and private candidate paths pass. The v6.2.0 line validates
request-derived origins, verifies legacy-cleanup SSH hosts, aligns Settings and
resource reads with session authority, recovers oversized WebSocket state,
converges agent and PBS lifecycle behavior, restores the deliberate self-hosted
commercial opt-in posture, closes the historical credential-containment gate,
and retries GET when Proxmox reports HTTP 405 or 501 for unsupported HEAD
availability probes. The exact `main` SHA must pass the integrated release
checks and immutable-candidate build before the single-build workflow crosses
its public mutation boundary.
Every release cut, including a prerelease, now gates that mutation boundary on
the complete frontend unit suite, frontend type-checking, and a deterministic
render smoke against the verified frontend bundle. The smoke must render
Proxmox nodes and workloads, Docker hosts and containers, Kubernetes clusters
and pods, and Alert thresholds with an omitted-zero disk payload; an error
boundary or uncaught browser error fails the cut. Failures retain Playwright
diagnostics. The same release workflow also executes the generated self-signed
and custom-CA Windows installer commands through Windows PowerShell 5.1 before
release assembly, so the first HTTPS fetch is release proof rather than a
string-shape assertion.
The active stable `v6.4.0` cut sets the repo-root `VERSION`, repo-root
`docker-compose.yml` image default, `scripts/install-docker.sh` fallback, and
Helm chart release metadata to the same `6.4.0` release version. This stable
minor release uses `promoted_from_tag=v6.4.0-rc.12`,
`rollback_version=v6.3.2`, and the explicit version-bound owner decision
recorded on 2026-08-28 to accept a shortened soak and the bounded product fixes
after that published candidate. The workflow input `hotfix_exception=true`
transports that approved waiver through the shared promotion resolver. It does
not reclassify v6.4.0 as a patch hotfix. The integrated single-build workflow
must pass its exact-SHA preflight and immutable readiness gates before
publication. Under the current single-build policy, a separate Release Dry Run
is optional. Stable/latest install pointers and stable semver aliases move only
after the exact public and private candidate paths pass. The release makes the append-only
event log authoritative for alert history and active-state reconstruction;
adds per-alert snooze, recurring scoped maintenance, destination severity
routing, repeatable escalation schedules, and external dead-man monitoring;
keeps informational alert severity distinct across configuration, persistence,
API responses, filters, email, ntfy, and mobile push presentation;
adds rolling-window metric policy and predictive storage-capacity alerts;
makes host SMART policy configurable without duplicating Proxmox disk risk;
keeps empty Unraid storage slots neutral instead of degrading array health;
converges infrastructure detail presentation; and strengthens independently
verified Docker actions plus atomic deployment enrollment and credential
persistence. The rc.11 corrective set extends that boundary to token creation,
agent-install issuance, and agent-removal revocation; corrects version-specific
Anthropic cost estimates; merges complete cross-source disk identities; retains
uniquely resolved dismissed TrueNAS SMART risk; and stabilizes narrow alert
investigation, shared chart interaction, responsive inline View preferences,
and the mobile Settings workspace.
The rc.12 corrective set makes the GitHub Atom release-feed fallback retain a
publication timestamp and resolve the exact architecture-specific archive URL,
so an API rate limit cannot leave the in-app Start Update action without an
installable asset. Missing URLs also produce an explicit operator-facing error,
and the Unraid sentinel boundary now keeps empty array slots neutral through
both agent collection and server monitoring. The final cutoff also prevents
standalone Proxmox provider instances with reused short node names from sharing
one linked agent without exact endpoint or TLS-fingerprint proof.
The rc.13 corrective set bounds unchanged stopped-container detail inspection
to a 15-minute refresh ceiling while preserving live running and lifecycle
state, which removes one daemon call per historical container from each normal
30-second report. It also restores same-name standalone Proxmox host-agent
links when unique provider-observed interface addresses disambiguate the sites,
while reused addresses remain ambiguous and fail closed.
The post-rc.12 cutoff also checkpoints a crash-safe active-state recovery envelope after durable
lifecycle failure and prevents any Pulse host interface from satisfying an
external dead-man signal. The stable server cut is classified
`existing-mobile-build-compatible`. The changes since
`v6.4.0-rc.6` add the canonical `alert_fired`
mobile push type, but preserve the existing `view_alert` navigation action and
all route, request/response, pairing, and authorization contracts. Published
Pulse Mobile iOS build 12 and Android versionCode 9 already route
`action_type=view_alert`, so the server cut is classified
`existing-mobile-build-compatible`; no companion upload or public mobile-store
rollout is part of this stable release. Published candidate source revision
`763e95138b840bae795ad6ca5affe930cfd0ef80` contains that navigation behavior,
and Pulse Mobile revision `471d158e7bca7348a2cd8e7e36b8b44f343934bb`
synchronizes the generated compatibility inventory with no required runtime
navigation change.
Stable `v6.4.0` skips SignPath under the standing unavailable policy and
retains exact-SHA, checksum, detached-signature, immutable-manifest,
published-digest, and Unknown Publisher disclosure controls. Signing returns
only after the release owner explicitly confirms that production credentials
and certificate authorization are ready and a reviewed policy/code change
restores it.

The preceding `v6.4.0-rc.9` qualification used exact source SHA
`dac72894a2ccaa6af2458ff88f38344cd5ce1abd`. Release run `33130438386`
passed preparation, frontend qualification, Windows installer smoke, release
smoke, private staging, immutable multi-platform candidate assembly, macOS
signing and notarization, exact-candidate container and Helm smoke, public
Docker staging, Helm staging, release-asset validation, and installer smoke.
All three race-enabled backend shards also reported `PASS`, with the slowest
shard completing in 1,106 seconds, but the job was cancelled by its 20-minute
job ceiling during completion. That cancellation caused immutable readiness to
skip and the activation verdict to fail closed. GitHub release `378204006`
therefore remains an unpublished draft, its annotated tag and exact-version
artifacts remain immutable, convergence run `33131402971` made no customer
pointer changes, and stable/latest remains `v6.3.2`. At the
`v6.4.0-rc.10` source SHA, the backend job had a 40-minute ceiling while
retaining the per-invocation 30-minute Go timeout, so setup, bundle transfer,
planning, and cleanup overhead could no longer cancel a successfully completed
suite. `v6.4.0-rc.10` fixed forward from that infrastructure failure.

The preceding `v6.4.0-rc.10` qualification used exact source SHA
`827017e196ede706d27876d27637712b5b165dcc` and release run `33132798478`.
Its first attempt staged the immutable tag, draft, 217 exact-version assets,
container images, Helm chart, and private runtime, while every non-backend
release gate passed. The three-shard backend plan assigned all eight worker
CPUs to API processes (`4/2/2`) while also running the non-API package graph
with default Go concurrency. On a cold worker the prefix shard reached its
cumulative 30-minute test-binary watchdog after completing 2,910 of its 3,624
top-level tests; the two 15- and 16-test tails passed independently, so this
was bounded runner oversubscription rather than an individual hung test. The
unchanged immutable retry completed the full backend job in 11 minutes 52
seconds. Activation recovery run `33135968998` then committed GitHub release
`378221878` with 220 assets and an exact-lineage activation marker; canonical
convergence owner `33135994780` passed Docker RC aliases, Helm Pages, and the
private paid-runtime broker and released the global promotion lease. The
release is public as a prerelease, while stable/latest remains `v6.3.2`.

Future release qualification on `main` must not depend on a warm worker. On
the canonical 8-vCPU, three-shard PVE plan, the backend runner reserves two
single-CPU package workers for the non-API graph, gives the API shards a
volume-weighted `4/1/1` allocation, passes `-p 2` to `go test`, and sets
`GOMAXPROCS=1` for every non-API test binary. The combined declared width is
therefore eight CPUs instead of eight API CPUs plus unbounded non-API package
concurrency. Each
API shard has a 45-minute cumulative watchdog, the non-API graph retains its
30-minute per-package watchdog, and the enclosing job has a 70-minute ceiling.
Stable `v6.4.2` rehearsal `33417470872` supplied the measured hosted-worker
evidence for that outer budget: all three race-enabled API shards passed in
1,911, 1,437, and 1,777 seconds, while the non-API graph continued passing
packages until the former 55-minute ceiling canceled it during the expanded
secure-runtime install-test package. The added headroom does not weaken or
extend either inner watchdog. Constrained
one-vCPU hosts run the non-API graph before the API shard rather than knowingly
oversubscribing the host. The deterministic CPU allocator and release guards
pin this partition and reject a job ceiling that could pre-empt the API
watchdog.

The first stable `v6.4.0` qualification attempt, release run `33220050114` at
exact source SHA `41428f46a3573802f3e89a51a40f92cf4434d0b6`, exposed the
other release worker class. Stable releases use a four-vCPU GitHub-hosted
runner, where auto mode selected two equal-count API shards of 1,829 and 1,828
tests. Shard 1 passed in 1,797 seconds, but shard 2 was still making forward
progress at `TestLimitedAPITokenCannotCreateBroaderToken` when its cumulative
2,700-second watchdog fired. Every other release gate passed, activation
failed closed, release `378812881` remained an unpublished draft, and
convergence run `33220806414` changed no customer pointers. This is measured
partition imbalance rather than a hung product test. Four-vCPU release workers
must therefore use the same three-way cost boundary as the PVE worker, with a
non-oversubscribed `1/1/1` API allocation plus one non-API package worker. The
existing 10-GiB admission floor still degrades the shard count on constrained
hosts instead of weakening the watchdog.

Activation-only recovery must distinguish the immutable binary packet from
governed release-note presentation sidecars. Release run `33223712880` passed
its immutable readiness join and retained draft release `378812881`, but its
first recovery rejected the generated comparison PNGs as assets outside the
candidate manifest. `verify-release` therefore permits only bounded,
non-empty `release-note-*-before.png` and `release-note-*-now.png` sidecars
with GitHub-provided SHA-256 metadata. Missing or changed candidate assets and
every other extra filename remain hard failures; recovery does not rebuild or
replace the qualified candidate.

Recovery run `33223074961` retargeted the unpublished draft and annotated tag
to corrected source SHA `148b9a0e329c488d1a4401060008543988c81113`, but its two
exact-version Docker publication legs validated release-line policy before the
independent draft-resume job had moved the tag. Both failed closed on the old
tag target without publishing or activating customer pointers. Exact-version
Docker staging may continue to overlap container qualification, backend tests,
and the other independent release gates, but it must depend on successful
draft creation or recovery so release-line validation always observes the
current immutable tag target.

Stable release run `33223712880` at exact source SHA
`58b07ef36cc815c2309ba43874ca5365f928ff77` subsequently passed every
immutable release gate on failed-job attempt 2, including the complete
race-enabled backend suite and release readiness. Activation remained
fail-closed because the first attempt's convergence owner had already ended
after the original backend failure. Activation-only recovery run `33227557895`
then exposed two independent draft-integrity conditions before publication:
the draft retained two unreferenced screenshot assets from a superseded visual
plan, and the manifest verifier treated all release-note screenshots as
unexpected even when the validated release body referenced them. The reusable
validation workflow had detected that mismatch but lost the Python exit status
through `tee`, so its job incorrectly reported success. Release validation and
activation recovery must preserve the verifier's pipeline status, require the
immutable candidate asset set plus exactly the non-empty, server-digested
release-note screenshots referenced by the validated body, and reject every
unreferenced asset. Recovery remains activation-only and may not rebuild or
replace the qualified candidate.

Activation recovery run `33227705135` published stable `v6.4.0` and bound
customer convergence to run `33227719830`. Docker aliases, Helm Pages, and the
paid-runtime broker converged, but the stable demo's large-estate restore left
the v6.4.0 process unresponsive and the public health endpoint unavailable.
The convergence owner was cancelled after the restore exceeded its bounded
health window; its global promotion lease released successfully. Dedicated
demo recovery run `33228777096` reproduced the failure with the exact installed
binary and unit, preserved the Relay process, restored the prior runtime
configuration, and compensated by stopping only `pulse.service`. A tagged
runtime may therefore enter the 50-node, 48-hour-seed public demo profile only
when it carries an explicit `mockLargeEstateStartupReady` marker earned by that
complete startup and browser proof. The eager-history and update-cohort source
markers remain necessary but are not sufficient. Runtimes without the explicit
proof marker use the governed 8-node, two-hour-seed legacy-bounded profile.
Both normal convergence and recovery force
`PULSE_MOCK_SEED_METRICS_STORE=false`: persistent mock-store backfill is an
isolated local-development opt-in, and carrying it into the public demo can
block startup while its existing metrics database is synchronously backfilled.
Failed stable-demo recovery must also retain bounded, redacted service evidence:
structured systemd state, bounded process metadata, and only startup- or
health-relevant recent journal lines. When the listener never opens, the
recovery sends `SIGQUIT` only to the already-failed Pulse PID before stopping
it, retaining a bounded Go goroutine dump instead of guessing which persisted
store is blocking synchronous startup. Diagnostics run after compensation-safe
recovery failure, carry a hard SSH timeout, and remain part of the existing
14-day recovery artifact rather than widening the recovery mutation boundary.
The retained stack for run `33229793872` proved that the failed large bootstrap
left more than 5,000 `IncidentStore.saveAsync` goroutines contending on one
persistence mutex while durable mock alert lifecycle events were replayed.
Stable-demo recovery therefore archives and clears only the fixed generated
demo operational-memory files (`ai_incidents.json` and `alerts/events.db` plus
SQLite sidecars), together with the fixed retired `alert-history` JSON recovery
sources that otherwise repopulate the cleared event log, before restart. A
failed bounded restart restores that archive;
success discards it. Customer configuration, active-alert recovery state,
metrics history, the binary, unit, and Relay remain outside this reset.
The authenticated browser proof waits on the visible connection-status role
and its accessible backend/stream detail, not on the `Connected` label itself:
the responsive header deliberately keeps that text visually collapsed until
hover even when the live stream is healthy.

The preceding `v6.4.0-rc.8` qualification attempt used exact source SHA
`bac7e5d9526d76a6b4e34738511b07609dda80ed`. Release run `33128595650`
passed preparation, frontend bundle, Windows installer smoke, release smoke,
private staging, immutable candidate assembly, and macOS signing and
notarization, then failed the frontend gate because stale severity coverage
treated the newly first-class `info` value as an unknown warning. Review also
found that the overview presentation sent truly unknown severity strings to
the informational palette instead of the contract's warning fallback. The run
was cancelled before a public tag or GitHub release was created. Exact fix
commit `fcf872fb5` gives `info` an explicit blue branch, fails unknown values
safe to warning, adds canonical and coverage proof, and carries the corrected
browser receipt. `v6.4.0-rc.9` fixes forward from that failed immutable
qualification without moving stable install pointers or stable semver aliases
from `v6.3.2`.

The preceding `v6.4.0-rc.7` publication attempt built and staged exact candidate
artifacts from source SHA `595c369d85796f86855b4cf8335b9bb371d28462`,
but the backend release gate failed before public activation. Its tag and
versioned artifacts remain immutable, and `v6.4.0-rc.9` supersedes the failed
candidate without moving stable install pointers or stable semver aliases from
`v6.3.2`.

The preceding `v6.4.0-rc.6` cut published from exact source SHA
`8fde82b8a24229fffb628732d10fc320be643099`. Its tag and versioned artifacts
remain immutable; later `v6.4.0` prereleases supersede it without moving stable install
pointers or stable semver aliases from `v6.3.2`.

The preceding `v6.4.0-rc.5` cut published from exact source SHA
`3b21d4c257a5e140af05af0973ce6cb1f1effc4d`. Its tag and versioned artifacts
remain immutable; later `v6.4.0` prereleases supersede it without moving stable
install pointers or stable semver aliases from `v6.3.2`.

The preceding `v6.4.0-rc.4` cut published from exact source SHA
`8fb7b3764183168f93140d83e2b18b4e953b6cd8`. Its tag and versioned artifacts
remain immutable; later `v6.4.0` prereleases supersede it without moving stable
install pointers or stable semver aliases from `v6.3.2`.

The preceding `v6.4.0-rc.3` cut published from exact source SHA
`cf0ca6f127540e9997c2eb97eeed32f27619d242`. Its tag and versioned artifacts
remain immutable; later `v6.4.0` prereleases supersede it without moving stable
install pointers or stable semver aliases from `v6.3.2`.

The preceding `v6.4.0-rc.2` publication attempt built and staged the exact
candidate artifacts but failed release convergence before public activation.
Its tag is immutable and the candidate is superseded by later `v6.4.0`
prereleases; stable
install pointers and stable semver aliases remained on `v6.3.2`.

The preceding prerelease `v6.4.0-rc.1` cut set the repo-root `VERSION`,
repo-root `docker-compose.yml` image default, `scripts/install-docker.sh`
fallback, and Helm chart release metadata to the same `6.4.0-rc.1` release
version. It opened the `v6.4.0` candidate line from `main` with
`rollback_version=v6.3.1` and did not move stable/latest install pointers or
stable semver aliases.

The active stable `v6.4.2` cut sets the repo-root `VERSION`, repo-root
`docker-compose.yml` image default, `scripts/install-docker.sh` fallback, and
Helm chart release metadata to the same `6.4.2` release version. This patch
release uses the stable hotfix path with `rollback_version=v6.4.1`,
`hotfix_exception=true`, a release-owner reason, and no fabricated same-version
RC tag. The governed branch is `main`. The active customer harm is two stable
authorization failures: authenticated non-administrator organization members
can reach infrastructure action control, and SSO-only deployments can treat
every authenticated IdP user as an instance administrator without an explicit
grant. The exact pushed `main` SHA must pass the integrated candidate checks
before publication. No governed mobile-facing path changed from `v6.4.1`, so
the release decision is `no-mobile-impact`. The standing
SignPath-unavailable policy from `v6.3.2` onward still applies, with public
Unknown Publisher disclosure and the existing signed integrity controls.

The preceding stable `v6.4.1` cut sets the repo-root `VERSION`, repo-root
`docker-compose.yml` image default, `scripts/install-docker.sh` fallback, and
Helm chart release metadata to the same `6.4.1` release version. This patch
release uses the stable hotfix path with `rollback_version=v6.4.0`,
`hotfix_exception=true`, a release-owner reason, and no fabricated same-version
RC tag. The active customer harm is the published `v6.4.0` server image's
non-executable embedded Unified Agent, which breaks Helm agent workloads and
direct agent entrypoints. The exact pushed `main` SHA must pass the integrated
candidate checks before publication, including the new pre-publication image
mode proof. No governed mobile-facing path changed from `v6.4.0`, so the
release decision is `no-mobile-impact`. The standing SignPath-unavailable
policy from `v6.3.2` onward still applies, with Public Unknown Publisher
disclosure and the existing signed integrity controls.

The preceding stable `v6.3.2` cut set the repo-root `VERSION`, repo-root
`docker-compose.yml` image default, `scripts/install-docker.sh` fallback, and
Helm chart release metadata to the same `6.3.2` release version. This patch
used the stable hotfix path with `rollback_version=v6.3.1`,
`hotfix_exception=true`, a release-owner reason, and no fabricated same-version
RC tag. The emergency reason was metrics-history memory growth that could wedge
a memory-limited Pulse runtime plus offline-policy alert noise on stable. The
exact pushed `release/v6.3.2` SHA passed the integrated candidate checks before
publication. No governed mobile-facing path changed from `v6.3.1`, so the
release decision was `no-mobile-impact`. Release run `32896554952` failed
closed during SignPath request submission; the release owner then established
the standing SignPath-unavailable policy from `v6.3.2` onward. Public Unknown
Publisher disclosure plus checksum, detached-signature, immutable-manifest and
published-digest controls remain mandatory.

The preceding stable `v6.3.1` cut set the repo-root `VERSION`, repo-root
`docker-compose.yml` image default, `scripts/install-docker.sh` fallback, and
Helm chart release metadata to the same `6.3.1` release version. This patch
release uses the stable hotfix path with `rollback_version=v6.3.0`,
`hotfix_exception=true`, a release-owner reason, and no fabricated same-version
RC tag. The emergency reason is active customer harm across notification
delivery recovery, Docker command continuity, local subscription setup, and
Synology Docker host load. The exact pushed `main` SHA must pass the
no-publication dry run and integrated candidate checks before the same SHA is
dispatched through the single-build publication workflow. No governed
mobile-facing path changed from `v6.3.0`, so the release decision is
`no-mobile-impact` and no companion build or store rollout is required.
Windows Authenticode was mandatory for `v6.3.1` unless the release owner
recorded a new explicit version-bound unsigned-Windows exception; the prior
`v6.3.0` decision could not be reused for this patch. On 2026-08-23 the release
owner recorded that separate `v6.3.1` exception after exact-SHA rehearsal
`32634435531` reached SignPath and proved the production certificate remained
`CSR PENDING`, leaving the `release-signing` policy invalid. This changes only
the Authenticode requirement: public Unknown Publisher disclosure and the
exact-SHA, checksum, detached-signature, immutable-manifest, and
published-digest controls remain mandatory.
Exact-SHA dry run `32637439888` and release run `32639641733` subsequently
passed on `2f3d2249973293da3ae6783592c68ebd0f5d1280`. Convergence run
`32640756239` initially failed without acquiring the promotion lease because
GitHub's public asset edge returned HTTP 404 for the already-uploaded activation
marker after the source run completed. Attempt 2 adopted that immutable marker
and passed Docker aliases, Helm Pages, paid-runtime broker promotion, stable
demo update and browser proof, the definitive convergence verdict, and lease
release. The waiter now treats a successful source run with an API-visible
uploaded marker as bounded propagation rather than an absent commit, while a
failed source run, a successful run with no uploaded marker, or a readable
marker with the wrong immutable identity still fails closed.

The preceding stable `v6.3.0` cut set the repo-root `VERSION`,
repo-root `docker-compose.yml` image default, `scripts/install-docker.sh`
fallback, and Helm chart release metadata to the same `6.3.0` release version.
This stable minor release uses `promoted_from_tag=v6.3.0-rc.6`,
`rollback_version=v6.2.1`, and the explicit version-bound decision recorded on
2026-08-22 to accept the shortened soak and bounded post-RC cutoff after the
canonical production telemetry review. The workflow input
`hotfix_exception=true` transports that approved waiver through the shared
promotion resolver; it does not reclassify v6.3.0 as a patch hotfix. The exact
stable `main` SHA must pass the no-publication dry run before the same SHA is
dispatched through the single-build publish workflow. The release moves
stable/latest install pointers and stable semver aliases only after the exact
public and private candidate paths pass. This release establishes the new
minor-release packet with durable scoped Patrol
objectives, validated read-only observers, verified work receipts, agent action
preflight with stable refusal codes, large-estate response improvements, and
monitoring correctness fixes. The advanced branch also carries the decision-first
Patrol inbox, first-class Actions workspace, canonical platform-admission
projection, and bounded concurrent unified-resource hydration. Subscription-backed
Patrol turns bound command cleanup after an idle deadline so descendant-held
output pipes cannot extend the caller-owned stall budget. This candidate also
adds canonical estate summaries and search, a real-delivery activity log,
least-privilege agent installation, and bounded Docker-in-LXC discovery while
preserving unreadable settings and AI state. The stable server cut is
classified `existing-mobile-build-compatible`. The changes since
`v6.3.0-rc.6` do not require a Pulse Mobile client change and preserve the
existing mobile, Relay, onboarding, and mobile-facing API contracts, so no
companion upload or public mobile-store rollout is part of this stable release.
For `v6.3.0-rc.5`, credential-free public and private payload compilation ran
on dedicated PVE workers with persistent caches. Platform archive validation,
exact-candidate container and Helm smoke, backend shards, frontend checks,
mobile qualification, and paid-runtime checks overlap where their credential
and mutation boundaries permit it. Publication consumes the immutable payloads
and manifests from that exact source run; it must not rebuild release binaries
after qualification. Post-publication Helm and paid-runtime convergence likewise
consume source-run artifacts and retain exact-SHA, installer, signature,
public/private artifact, and definitive convergence proof.
For `v6.3.0-rc.6`, the release path retains credential-free PVE compilation
while requiring the measured memory floor for API race shards after a
bounded admission wait. Production chart and resource-query services move
their complete handler, cache, tenant-store, and high-scale query suites out of
the residual root package so the Go scheduler can overlap real domains rather
than test-only shells. Public server and provider control-plane image
publication uses independent matrix jobs that each revalidate the exact
checkout and candidate manifest. Private packaging transfers only the products
consumed by Pro assembly, and paid-runtime Docker and direct-binary mismatch
proofs execute concurrently without weakening either proof. All immutable
candidate, signing, installer, public/private integrity, activation, and final
convergence joins remain mandatory.
The v6.3.0 stable cutoff at
`53ba9786c5522a6839f9cbd3d01c02402556f9eb` adds fail-closed release dry-run
diagnostics, native-agent fixture portability, measured backend shard
admission, Patrol provider-unavailable recovery, per-resource alert override
correctness, and cross-estate host identity preservation on top of the
published `v6.3.0-rc.6` lineage. The 2026-08-22 release-owner record binds the
shortened-soak and post-RC risk acceptance to aggregate production telemetry:
18 active `rc.6` installs across binary and Docker, 56 recorded update
successes and zero update failures, no rollback or version-departure signal,
and no follow-up notification-failure or governed-action-failure increase; the
preceding `rc.5` cohort likewise showed no rollback and one forward transition
to `rc.6`. The record states the modest sample and young `rc.6` follow-up
window explicitly rather than presenting them as 72-hour soak evidence.
The release owner separately approved a v6.3.0-only unsigned-Windows
exception because Authenticode signing is not yet available. This independent
decision does not broaden the soak waiver. The Windows packet must disclose
the Unknown Publisher warning and retain exact-SHA, checksum,
detached-signature, immutable-manifest, and published-digest verification.
Release run `32493044910` exposed the memory-driven one-shard backend fallback
before publication: all 3,736 top-level API tests were encoded into one
`-test.run` argument and Linux rejected the invocation with `Argument list too
long` before any API test executed. The canonical planner now bounds each
deterministic contiguous batch by encoded regex bytes as well as test count.
That failed run and its inert private staging child were cancelled; no public
tag or GitHub release was created, and the corrected candidate must run from a
new exact source SHA.
Release run `32497250921` then proved that naïve byte-bounded alternation was
not semantically sufficient: its third fresh test-binary process reached
`TestEstablishSession` without the package-global session store historically
initialized earlier in the ordered suite. Prefix compression reduces the exact
3,736-test expression from 182,429 bytes to 110,384 bytes, so the release
planner can keep the measured fast prefix in one process at the 120,000-byte
ceiling instead of fragmenting it into naïve alternation batches. The two session-establishment tests
now initialize their own persistent session store, removing the boundary-order
dependency instead of relying on a larger process. The planner still records
the complete test-name digest, proves each compressed expression is an exact
ordered partition, and rejects any configured ceiling above 120,000 bytes.
That failed run and its private child were cancelled before publication; no
public tag or release was created.
The publish body keeps its `Highlights` section within the governed three-item
limit while preserving the packet's operator-facing summary of read-only
observer coverage, estate-first platform search and facets, and real delivery-
attempt visibility. The detailed packet must also retain the bounded Docker-in-
LXC and supported least-privilege Unified Agent statements instead of dropping
them during publish-body condensation.
The preceding prerelease Windows path retained exact-SHA, checksum, and
detached-signature verification without Authenticode. Stable `v6.3.0` uses the
separately recorded version-bound unsigned-Windows exception: the public notes
must state that its Windows binaries are not Authenticode-signed and may
display an Unknown Publisher warning. The immutable-candidate, checksum,
detached-signature, manifest, and published-digest controls remain mandatory.

The preceding `v6.3.0-rc.1` publication attempt was quarantined before the
GitHub release commit point when its exact private Pro staging run was
cancelled. No GitHub release or remote git tag remains. Because `main` advanced
materially after that attempt, the next dispatch uses `v6.3.0-rc.2` rather than
retargeting the earlier version to a different exact SHA.

The preceding `v6.2.2-rc.2` candidate used the same support-prerelease path and
rollback target. It added host-local Docker/Podman registry credentials for
private-image update checks and smartmontools 7.5 `power_mode` object decoding
for guarded rotational-disk probes while retaining the first-candidate packet.

The preceding `v6.2.2-rc.1` candidate used the same support-prerelease path and
rollback target. It established the cumulative security, monitoring-scale,
resource-policy, agent, Relay, alerting, update, and release-qualification
packet and recorded the then-relevant existing-mobile-build compatibility
decision for Pulse Mobile 1.0.0 iOS build 12 and Android versionCode 9.

The preceding stable `v6.2.1` cut sets the repo-root `VERSION`, repo-root
`docker-compose.yml` image default, `scripts/install-docker.sh` fallback, and
Helm chart release metadata to the same `6.2.1` release version. This emergency
patch uses the integrated exact-SHA candidate and definitive release verdict
because the release range repairs active customer
harm in agent download preflight, Agent Doctor credential recovery, strict
subscription-provider tool schemas, and fresh Pro activation discovery. The
same cut includes the frontend-primitives-owned update verdict age from #1601,
so a cached "Up to date" result identifies when its backing check ran. The
installer/updater risk boundary requires the explicit patch hotfix reason even
though the workflow performs the normal immutable-candidate checks before its
publication boundary. Windows executables must be Authenticode-signed through
SignPath by default. For `v6.2.1`, failed release run `31343128024` proved that
the external `Release certificate 2026` CSR remained pending, and the release
owner approved a version-bound unsigned-Windows exception. The Windows packet
must disclose that it is not Authenticode-signed and may display an Unknown
Publisher warning while retaining exact-SHA, checksum, detached-signature,
manifest, and published-digest verification. Stable `v6.2.2` and later restore
mandatory Authenticode unless another explicit version-bound decision is
recorded. The mobile decision is `no-mobile-impact` because the patch does not
change the checked-in mobile API, Relay, pairing, approval, push,
authentication, or onboarding contracts.
This patch release uses the stable hotfix path with
`rollback_version=v6.2.0`, `hotfix_exception=true`, a release-owner reason, and
no fabricated same-version RC tag.
The stable server cut is classified `existing-mobile-build-compatible`. The
synchronized Pulse Mobile 1.0.0 iOS
build 12 and Android versionCode 9 candidates, both using runtime version 2,
remain distributed to the existing beta cohort through TestFlight and Play
open testing. The v6.2.0 changes preserve the checked-in mobile API, Relay,
pairing, approval, push, authentication, and onboarding contracts; no
additional companion upload or public store rollout is part of the stable
server cut. Exact-SHA dry run `31306697834` failed closed before candidate
assembly because the SignPath `Release certificate 2026` CSR remained pending.
The release owner then separately approved a v6.2.0-only unsigned-Windows
exception. The Windows packet must disclose the Unknown Publisher warning and
retain exact-SHA, checksum, detached-signature, manifest, and published-digest
verification; this decision is not inherited by later releases.
The separately authorized `v6.2.1` exception is recorded in
`docs/release-control/v6/internal/records/v6.2.1-unsigned-windows-owner-approval-2026-08-10.md`.
The first RC11 publication attempt, run `31274524321`, passed every immutable
release gate and briefly crossed the draft boundary, then failed before the
irreversible activation marker because the checkout-free activation job relied
on local git repository discovery. The error trap returned the release to draft
quarantine and the orphaned convergence run was cancelled. The metadata-only
correction targets the marker upload repository explicitly, adds a qualified
activation-only recovery path that reuses the already validated RC11 packet,
and accepts GitHub's retained historical `published_at` value only while the
release's current state remains `draft=true`; RC11 keeps the same version
identity and code-backed validation-risk head.
The preceding `v6.2.0-rc.10` attempt used the same support-prerelease path with
`rollback_version=v6.1.2`, but it is an abandoned partial candidate rather than
the current public-testing identity. Run `31267947317` staged its annotated tag,
unpublished draft and exact-version release assets, public Docker images, public
OCI Helm chart, and private Pro runtime from
`76e07be290892ed8453bbed942855c1e7f673232` before install smoke failed. Because
those immutable-looking versioned artifacts escaped the draft boundary, RC10
must not be retargeted to another source revision; RC11 provides the clean
successor identity while the RC10 evidence remains available for audit.
The preceding `v6.2.0-rc.9` candidate used the same support-prerelease path
with `rollback_version=v6.1.2` and pinned the same four install surfaces to
`6.2.0-rc.9`. It is superseded by this cut and no longer governs the install
pins; its packet stays in `docs/releases/` as the historical candidate record
for the `v6.2.0` line.
The preceding `v6.2.0-rc.8` candidate used the same support-prerelease path
with `rollback_version=v6.1.2` and pinned the same four install surfaces to
`6.2.0-rc.8`. It is superseded by this cut and no longer governs the install
pins; its packet stays in `docs/releases/` as the historical candidate record
for the `v6.2.0` line.
The preceding `v6.2.0-rc.7` candidate used the same support-prerelease path
with `rollback_version=v6.1.2` and pinned the same four install surfaces to
`6.2.0-rc.7`. It is superseded by this cut and no longer governs the install
pins; its packet stays in `docs/releases/` as the historical candidate record
for the `v6.2.0` line.
The preceding `v6.2.0-rc.6` candidate used the same support-prerelease path
with `rollback_version=v6.1.2` and pinned the same four install surfaces to
`6.2.0-rc.6`. It is superseded by this cut and no longer governs the install
pins; its packet stays in `docs/releases/` as the historical candidate record
for the `v6.2.0` line.
The preceding `v6.2.0-rc.5` candidate used the same support-prerelease path
with `rollback_version=v6.1.2` and pinned the same four install surfaces to
`6.2.0-rc.5`. It is superseded by this cut and no longer governs the install
pins; its packet stays in `docs/releases/` as the historical candidate record
for the `v6.2.0` line.
The preceding `v6.2.0-rc.4` candidate used the same support-prerelease path
with `rollback_version=v6.1.2` and pinned the same four install surfaces to
`6.2.0-rc.4`. It is superseded by this cut and no longer governs the install
pins; its packet stays in `docs/releases/` as the historical candidate record
for the `v6.2.0` line.
The preceding `v6.2.0-rc.3` candidate used the same support-prerelease path
with `rollback_version=v6.1.2` and pinned the same four install surfaces to
`6.2.0-rc.3`. It is superseded by this cut and no longer governs the install
pins; its packet stays in `docs/releases/` as the historical candidate record
for the `v6.2.0` line.
The preceding `v6.2.0-rc.2` candidate used the same support-prerelease path
with `rollback_version=v6.1.2` and pinned the same four install surfaces to
`6.2.0-rc.2`. It is superseded by this cut and no longer governs the install
pins; its packet stays in `docs/releases/` as the historical candidate record
for the `v6.2.0` line.
The preceding `v6.2.0-rc.1` candidate used the same support-prerelease path
with `rollback_version=v6.1.2` and pinned the same four install surfaces to
`6.2.0-rc.1`. It is superseded by this cut and no longer governs the install
pins; its packet stays in `docs/releases/` as the historical candidate record
for the `v6.2.0` line.
Authenticode signing through SignPath is the canonical Windows signing backend
for the `v6.2.0` line. After exact-SHA dry run `31306697834` failed closed on
the pending SignPath release-certificate CSR, the release owner approved a
`v6.2.0`-only unsigned-Windows exception. It preserves the exact-SHA candidate,
checksum, detached-signature, manifest, published-digest, owner-reason, and
public Unknown Publisher disclosure controls. Stable `v6.2.1` and later
restore mandatory Authenticode unless policy records another explicit,
version-bound decision.

The active stable `v6.1.2` cut sets the repo-root `VERSION`, repo-root
`docker-compose.yml` image default, `scripts/install-docker.sh` fallback, and
Helm chart release metadata to the same `6.1.2` release version. This patch
release uses the stable hotfix path with `rollback_version=v6.1.1`,
`hotfix_exception=true`, a release-owner reason, and no fabricated
same-version RC tag. It consolidates the post-`v6.1.1` Proxmox, TrueNAS,
metrics, availability, agent lifecycle, operator-state, and security fixes.
The exact stable `main` SHA must pass the integrated release checks and
immutable-candidate build before the single-build workflow crosses its public
mutation boundary. The same workflow must finish Docker, Helm, stable demo,
install-smoke, public-health, floating-tag, paid-runtime, and
definitive-verdict lanes before the cut is complete. No mobile-facing path
changed from `v6.1.1`, so the release decision is `no-mobile-impact` and no
companion build or public mobile-store rollout is required. Stable `v6.1.2`
uses an owner-approved, version-bound unsigned-Windows exception while the
SignPath company-verification application is still processing. Windows users
must receive an Unknown Publisher disclosure, and the release candidate must
retain exact-SHA, checksum, detached-signature, manifest, and published-digest
verification. Stable `v6.1.3` and later restore mandatory Windows Authenticode
unless policy records another explicit version-bound decision.

The preceding stable `v6.1.1` cut used the stable hotfix path with
`rollback_version=v6.1.0` and an owner-approved, version-bound unsigned Windows
exception. Pulse Mobile `1.0.0` iOS build `11` and Android versionCode `9`
remained the compatible candidate builds without a companion upload or public
store rollout.

The preceding stable `v6.1.0` cut used
`promoted_from_tag=v6.1.0-rc.4`, `rollback_version=v6.0.5`, and the
one-version release-owner cutoff exception
recorded on 2026-07-22. The workflow input `hotfix_exception=true` carries that
approved soak bypass through the existing promotion resolver; it does not
reclassify the release as a patch hotfix. The exact stable `main` SHA must pass
the no-publication dry run before the same SHA is dispatched through the
single-build publish workflow. The release publishes versioned GitHub, Docker,
and Helm artifacts and advances the stable/latest install pointers and stable
semver aliases. It promotes
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
The stable cutoff includes the fifth-candidate work that sharpens
infrastructure identity and fleet truth, moves
Agent Doctor into a routed, filterable diagnostic workflow with copyable
reports and platform-correct host-local cleanup handoff, adds SAS and SCSI SMART
coverage, restores actionable agent-update controls, fixes Patrol finding and
proposal provenance, improves metrics and audit-store concurrency, and prepares
the stable Windows SignPath signing path. For `v6.1.0`, the release owner
explicitly waived Authenticode after the first stable rehearsal exposed
unavailable external SignPath configuration. The unsigned Windows binaries
remain exact-SHA and manifest-bound with checksum, detached `.sig`/`.sshsig`,
and published-digest verification, and the release notes disclose the Unknown
Publisher state. That `v6.1.0` decision does not itself authorize another
version.
The `v6.1.0` stable server cut was classified
`existing-mobile-build-compatible`. Pulse
Mobile `1.0.0` iOS build `11` and Android versionCode `9` remain the existing
candidate builds; the canonical core/mobile contract proves that `v6.1.0`
serves their route, scope, payload, pairing, and push requirements, including
the relay-mobile Patrol attention scopes corrected after rc.4. No companion
build upload or public mobile-store rollout is part of this server release.
The same release boundary now provides one canonical in-app release-note
experience. Update checks can preview a curated `Highlights` section before an
update, while the authenticated running-version endpoint lets the post-update
surface extract the published `Added`, `Improved`/`Changed`, `Fixed`,
`Security`, `Breaking changes`, `Deprecated`, and `Removed` sections into a
categorized changelog after an upgrade. Summary-only or uncategorized releases
stay quiet in the post-update dialog, and source or development builds never
masquerade as published releases. The shared scrollable dialog keeps the
categories readable without pushing the dashboard down; every close path still
records the running release as seen.
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
For the active stable `v6.1.2` cut, the repo-root compose default and
`scripts/install-docker.sh` fallback must both pin `6.1.2` whenever the
governed `VERSION` is that stable cut. The stable promotion guard remains in
force and rejects leftover `-rc.` defaults.
For the active stable `v6.4.2` cut, the repo-root compose default and
`scripts/install-docker.sh` fallback must both pin `6.4.2` until the next
governed release moves them forward. The stable promotion guard remains in
force and rejects leftover `-rc.` defaults. Each new release moves
these two pins together with the repo-root `VERSION` and the Helm chart metadata
in the same commit; a cut that leaves any of the four on a superseded value is a
release-packet blocker.
For the preceding stable `v6.4.1` cut, the repo-root compose default and
`scripts/install-docker.sh` fallback both pinned `6.4.1`.
For the preceding stable `v6.4.0` cut, the repo-root compose default and
`scripts/install-docker.sh` fallback both pinned `6.4.0`.
For the preceding stable `v6.3.2` cut, the repo-root compose default and
`scripts/install-docker.sh` fallback both pinned `6.3.2`.
For the preceding stable `v6.3.1` cut, the repo-root compose default and
`scripts/install-docker.sh` fallback both pinned `6.3.1`.
For the preceding stable `v6.3.0` cut, the repo-root compose default and
`scripts/install-docker.sh` fallback both pinned `6.3.0`.
For the preceding stable `v6.2.1` cut, the repo-root compose default and
`scripts/install-docker.sh` fallback must both pin `6.2.1`. The stable
promotion guard remains in force and rejects leftover `-rc.` defaults. Each
new release moves these two pins together with the repo-root `VERSION` and the
Helm chart metadata in the same commit; a cut that leaves any of the four on a
superseded value is a release-packet blocker.
The RC11 packet records `2018aa8a9a965d693982e260f525f6cc4f49aa41` as
the code-backed validation-risk head. That head covers 68 commits and 241 files
since RC9 across request-origin and SSH trust, settings RBAC and responsive
layout, WebSocket recovery and resource deltas, agent and PBS lifecycle,
commercial-surface rollback, Proxmox availability fallback, release-control
hardening, and customer artifact promotion. The monitoring-contract correction
and metadata-only release-preparation commits may be the workflow dispatch head
because they do not change that code-backed release-risk range.

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
for the published server and provider control-plane images, and attest the
generated release packet assets from the `release/` directory. The exact-SHA
candidate compiler must validate and embed the governed license and update
public keys once, record the resulting runtime and control-plane binaries in
the immutable candidate manifest, and send only those verified bytes to image
qualification and publication. `publish-docker.yml` must not receive release
signing material or license-key build inputs and must not recompile either
binary.
Source-built release-grade Docker targets remain a fail-closed diagnostic and
development boundary. When those targets are used, they must pass the license
public key through a BuildKit secret mount rather than a Docker build argument,
pair it with the non-secret `PULSE_LICENSE_PUBLIC_KEY_SHA256` cache key, and
verify the fingerprint before embedding the key. The matching installability
proof lives in `scripts/installtests/build_release_assets_test.go` and
`scripts/release_control/release_promotion_policy_test.py`.
The standalone hosted control-plane image is part of the same release-license
boundary. `deploy/provider-msp/Dockerfile.control-plane` must build
`cmd/pulse-control-plane` with `-tags release`, canonical
`scripts/release_ldflags.sh server` metadata, and the embedded governed license
public key. Its source-built target must retain the BuildKit secret and
`PULSE_LICENSE_PUBLIC_KEY_SHA256` fingerprint gate, while its published
prebuilt target must consume the manifest-bound control-plane binaries from
the exact candidate. Provider-hosted MSP uses
that control-plane image for signed MSP-license enforcement, so it must not be
possible to publish a provider MSP control-plane image that accepts
`PULSE_LICENSE_DEV_MODE` or `PULSE_LICENSE_PUBLIC_KEY` runtime overrides.
`.github/workflows/publish-docker.yml` must publish and attest
`rcourtman/pulse-control-plane` and
`ghcr.io/<owner>/pulse-control-plane` from that Dockerfile, with the same
version tags and prerelease/latest tag policy as the main Pulse runtime image.
The reusable Docker publisher must accept the exact container artifact name
and source SHA from its owning release run, verify both against the exact
checkout and candidate manifest, and expose no standalone dispatch that could
silently rebuild different bytes for an existing release tag. Before draft-tag
creation it may validate an anticipated tag only through that exact SHA; if the
tag already exists, its peeled commit must match the anticipated SHA.
That same supply-chain boundary also owns the checked-in build roots
themselves. `Dockerfile` must pin its Node, Go, and Alpine bases by immutable
manifest-list digest so multi-arch release builds do not silently drift onto a
different upstream filesystem just because a mutable tag was republished.
The governed frontend and container lines are Node.js `24` and Alpine `3.24`.
Every `actions/setup-node` release, governance, security, integration, and
native-agent job, the release-preflight worker, the developer container, and
the release frontend stages must stay on Node.js `24`; local integration setup
must reject a different major instead of treating newer non-LTS majors as
equivalent. Shipped and test Dockerfiles must pin every external base to a full
manifest-list digest, and checked-in integration Compose images must do the
same. Node.js `24` is normally supported through `2028-04-30` and Alpine
`3.24` through `2028-06-01`; a weekly repository check must fail once either
governed line has less than 180 days of normal upstream support remaining so a
replacement can be qualified before delivery depends on an unsupported line.
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
As of 2026-08-27, the governed release floor is Go `1.26.7`. It supersedes
`1.26.5`, whose standard library is reachable through seven vulnerable Pulse
call paths reported by `govulncheck`, including HTTP/TLS, URL parsing, SAML XML
decoding, HTML templating, and public-key parsing. Both source-built container
stages pin the Docker Official Images Linux amd64 manifest
`sha256:28d89ee9cc0ff9fec75c82ca201e6bf7fdf9a679d4b7b24dfa04f2bb766bb468`;
the checked-in toolchain files and release-script guards must reject an older
compiler so local, exact-candidate, provider control-plane, and container builds
cannot silently reintroduce the vulnerable runtime.
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
The shared Pulse server runtime must also ship the local Apprise notification
CLI promised by the alert destinations surface. `Dockerfile` must install an
explicitly pinned Apprise release in a separate builder stage, copy that
environment into `pulse-runtime-base`, retain only the runtime Python
interpreter there, and verify the exact CLI version in both stages. Because
hosted, E2E, and self-hosted server images all derive from that base, local
Apprise delivery—including Telegram `chat_id:topic` targets—must not depend on
an operator modifying a running container or on a mutable Alpine package
version.
That same Docker build graph must keep hosted tenant runtime images separate
from release-installer assembly. `Dockerfile` must expose a `hosted_runtime`
target derived from the shared Pulse server runtime base that copies only the
server runtime assets and does not depend on rendered installers, embedded
agent binaries, or installer signing material. The published self-hosted
`runtime` and `agent_runtime` targets must keep using the release-assets stage
so official release images still carry signed installer and agent download
assets, and any build that declares `PULSE_UPDATE_SIGNING_PUBLIC_KEY` must
continue to fail closed unless the matching signing secret is mounted.
The `agent_runtime` image is a Unified Agent deployment and defaults to both
host and Docker modules (`PULSE_ENABLE_HOST=true`,
`PULSE_ENABLE_DOCKER=true`). Operators may still set
`PULSE_ENABLE_HOST=false` for an intentional workload-only deployment, but
the image must not silently force that narrower mode and remove a previously
reported machine from the Hosts surface.
The legacy `scripts/install-container-agent.sh` compatibility wrapper must
match that default by forwarding `--enable-docker --enable-host`; it may point
operators who need workload-only behavior to the canonical installer's
explicit `--enable-host=false` option, but must not choose that opt-out for
them.
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
state. Every normal `.github/workflows/create-release.yml` cut validates the
uploaded packet while the release is still a draft and must pass `draft=true`
into `.github/workflows/validate-release-assets.yml`. Draft-only runs stop at
that state; publication runs continue through the readiness barrier. Neither
path may misclassify staged validation as post-publication revalidation.
That same reusable-validation call boundary also owns permission handoff.
`.github/workflows/create-release.yml` must explicitly grant the nested
`.github/workflows/validate-release-assets.yml` call the write scopes it
requests (`contents: write` and `issues: write`), rather than inheriting the
release pipeline's top-level read-only default and failing at workflow startup.
The staged installer-smoke boundary owns the same fail-closed handoff for
unpublished assets. GitHub requires write-level repository access to read draft
release metadata and asset bytes, even when the consumer only performs GET
requests. The `install_sh_smoke` call in `.github/workflows/create-release.yml`
and the `smoke` job in `.github/workflows/install-sh-smoke.yml` must therefore
grant `contents: write`; the workflow-level default remains read-only and the
smoke job must not receive unrelated write scopes.
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
That same release-trust boundary owns the distinction between source proof,
release-artifact proof, and post-install live-runtime proof. Publication or
successful installation may establish `release-validated`; neither may be
described as `live-verified` or fixed on hardware without a fresh passing
receipt from `scripts/release_control/live_runtime_proof.py` for the named
target and exact running version. For the Proxmox protection-posture
persistence regression, collection must fail on an empty successful-posture
cohort or while any posture with `lastSuccessfulPointAt` remains `unknown`.
The receipt must bind the expected and observed versions, normalized target
origin, packaged-versus-development runtime state, TLS verification, UTC
collection time, aggregate posture results, failing resource IDs, and response
SHA-256 values. The verifier must reject failed, stale, edited, source-build,
development-build, TLS-unverified, wrong-target, and wrong-version receipts.
Credential values belong only in named environment variables and must never be
written to command arguments or receipts. A missing or failed receipt is an
enforced lower claim level, not an operator-waivable proof gap.
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
User-visible fixes added to an assembled candidate packet must remain bound to
release-body verification: the exact claim is pinned in
`scripts/release_control/render_release_body_test.py`, and the complete packet
must still pass canonical shape validation. The `v6.4.0-rc.2` packet therefore
binds the non-running container stale-health correction under **Fixes** without
changing candidate artifact identity or dispatch authority.
The stable `v6.4.0` packet synthesizes the complete difference from `v6.3.2`
rather than concatenating candidate notes. It retains the supported TrueNAS
typed-counter claim, post-rc.12 Docker and Proxmox corrections, rolling-history
snapshot fix, standing unsigned-Windows disclosure, mobile compatibility
decision, rollback target, and owner-authorized expedited-promotion record.
Packet proof must reject notes that drop those required release boundaries or
cease to pass canonical shape validation.
Release-note transport is file-backed and fail-closed: operator helpers must
send the Markdown through JSON input rather than multiline form-field
substitution, and every `gh workflow run --json` input value must be encoded as
a string because the GitHub CLI unmarshals that payload into a string map before
GitHub applies the workflow's declared input types. The renderer must reject
missing standalone title/section structure before any tag or draft mutation,
and draft creation must compare GitHub's stored body with the exact rendered
file before asset upload.
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
must treat `demo-stable` as the only active public demo target. Stable releases
may update it only from the lease-owning convergence workflow with an exact
stable tag and matching activation marker. The historical deploy workflow is a
non-mutating verification wrapper and must never deploy branch-tip source.
Prerelease tags must not create or update a second public v6 preview target by
default, and any future preview surface requires a new explicitly governed
target instead of reusing the retired v6-preview path.
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
The governed demo update workflow also owns the runner-to-host network path.
It must establish the canonical Tailscale connectivity step
before SSH setup so stable or preview targets may stay on governed private
hostnames or Tailscale IPs, rather than silently depending on public SSH
reachability from GitHub-hosted runners. The workflow must use the current
pinned Tailscale GitHub Action, its target `ping` readiness gate, and the shared
`.github/scripts/check-demo-reachability.sh` TCP/22 diagnostic before SSH key
capture. A successful tailnet join alone is not connectivity proof. After that
network preflight, shared SSH
setup must wait for configured demo hostnames to resolve, accept configured IP
literals without a DNS precheck, and then capture host keys with bounded
short retries before any installer or binary copy runs; a long `ssh-keyscan`
loop must not hide an ACL, peer-propagation, firewall, or sshd failure.
`release-convergence.yml` must call the update workflow as a reusable job after
the irreversible activation commit. Its `Customer Promotion Convergence
Verdict` must require stable demo runtime, frontend, public health, and browser
proof. During a stable release cut, the
update workflow must wait for the activated public release assets and install
the exact requested version through the normal signed release path. It must not
accept an unpublished draft release ID or install draft assets into the live
demo. The stable demo update may run in parallel with the other mutable
customer pointers only after activation, and the definitive verdict must await
its proof. A demo failure is retriable convergence debt and cannot retroactively
unpublish the committed GitHub release. An
asynchronous dispatch or manual SSH deployment is not release completion. A one-shot `ssh-keyscan`
against a private demo target is not sufficient release or deploy proof.
Checkout-free mutation guards must address the Pulse repository explicitly
when querying release state; they must not rely on `gh` discovering a local git
checkout before the workflow's checkout step has intentionally been admitted.
The repository binding must use GitHub's exact `GITHUB_REPOSITORY` context so
the guard remains explicit without hard-coding an owner or fork identity.
Those same workflows also own customer-visible browser truth for the public
demo shell. Health checks and entry-asset parity are necessary but not
sufficient; after those checks pass, the governed helpers
`scripts/run_demo_public_browser_smoke.sh` and
`scripts/demo_public_browser_smoke.cjs` must exercise the public demo in a real
Chromium session and prove both that the login shell renders and that a public
demo login reaches the connected workspace instead of failing open on API-only
reachability or remaining on the authenticated loading shell. The proof must
use visible UI state rather than Playwright `networkidle`, because the public
demo can keep background activity alive after the page is already usable.
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
`.github/workflows/create-release.yml` fans out to governed staging or
post-activation workflows such as Docker publication or demo updates, it must dispatch those
workflows on `needs.prepare.outputs.required_branch` rather than GitHub's
default-branch workflow definition, so prerelease automation cannot silently
fall back onto stale `main`-branch inputs or older demo verification logic.
That same release-fidelity boundary also owns governed Helm publication. The
Helm release workflows must derive the owning branch from the target version via
`control_plane.py --branch-for-version` before any chart mutation or packaging,
must check out either that governed release branch or the validated release tag
before touching chart contents, and must never hardcode `main` as the push or
package source for prerelease Helm publication.
Pre-activation release proof and versioned chart publication have different
trust jobs and must stay that way: `.github/workflows/create-release.yml`
must smoke the Helm chart against a locally built release-line image before the
tag is activated, while `.github/workflows/helm-pages.yml` must continue
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
When GitHub's release API is rate-limited, the Atom fallback must also return
an installable current-platform archive URL and the entry's release timestamp,
not merely a version string. The archive URL may be derived only from the
validated Pulse tag and the governed deterministic release-asset naming
contract; the normal apply pipeline must still verify its pinned SSHSIG and
checksum before installation. A fallback response that advertises an update
with an empty download URL is a release-blocking defect because the UI cannot
start the apply request. Proof: `internal/updates/manager_retry_test.go`.
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
The canonical Proxmox mock density targets a large public-demo estate so
platform-first pages exercise multi-cluster grouping and the production
workload-windowing path on first boot. `mock_default_entries()` in
`scripts/toggle-mock.sh`, the public demo deployment environment in
`.github/workflows/update-demo-server.yml`, and `internal/mock.DefaultConfig`
carry the same baseline: 50 Proxmox nodes with 10 VMs and 8 LXCs each, arranged
as eight six-node clusters plus two standalone nodes; 5 Docker hosts with 14 containers
each, 4 standalone Pulse-managed hosts, and 3 Kubernetes clusters
(production + staging + edge) with 5 nodes, 40 pods, and 14
deployments each. Bumping any owner requires bumping the others (and
the matching `scripts/tests/test-toggle-mock.sh` fixtures) so toggle
CLIs, managed runtime restarts, public demo convergence, and the in-binary
default never drift apart. Existing installations with explicit custom
`PULSE_MOCK_*` values retain those values during legacy-sidecar migration.
Because the reusable deployment workflow executes from its caller revision
while installing an exact tagged binary, it must inspect that tag before
converging the runtime environment. The 50-node profile is permitted only when
the tagged runtime contains both bounded eager guest history and cohort-based
metric updates. A stable tag that predates either capability must receive the
bounded compatibility profile across every fixture family: 8 Proxmox nodes
with 6 VMs and 4 LXCs per node, 2 Docker hosts with 8 containers each, 2
generic hosts, and 1 Kubernetes cluster with 3 nodes, 12 pods, and 4
deployments. That profile retains two hours of eager history and uses a
five-minute history sample interval plus a 15-second full-estate update cadence,
preventing newer workflow defaults from imposing an unqualified synchronous
bootstrap, memory, and CPU load on an older binary before it binds health.
The manually dispatched stable-demo recovery path consumes that same resolver
and may mutate only the resolver-owned bounded mock-profile values before
restarting the already-installed `pulse` service. It must retain the current stable binary,
unit, drop-ins, Relay process, and billing state; prove the expected hostname
and release version plus public health, frontend parity, and authenticated
connected-workspace browser readiness;
and restore the prior runtime configuration while stopping the failed service
if post-mutation validation fails. It must not install artifacts, invoke release
convergence, or accept caller-selected target/version inputs.
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
release uses, first verifying `checksums.txt.sshsig` against the pinned
Pulse release key and failing closed if either integrity artifact is missing,
invalid, or ambiguous, and place the binary at `~/.local/bin/pulse-mcp`
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
audit trail of signed-manifest and SHA256 verification preserved.
Because the shell installer is itself an executable bootstrap trust boundary,
release assembly must include `install-mcp.sh` in the authenticated checksum
manifest and emit its checksum and detached-signature sidecars before upload;
validating only the binary that an unauthenticated installer chooses is not a
complete install chain. Local and post-publication validation require every
published installer entry exactly once in that signed manifest.
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
`system.json` and the timer's enabled/started state untouched. The rendered
timer must schedule exactly one update attempt per day — a single
`OnCalendar` entry at 02:00 with the 4h randomized spread — because a second
`OnCalendar=daily` line silently doubled every box's daily update attempts
(issue #1643). The rendered service sandbox must keep the helper's directory
and the unit directory inside `ReadWritePaths`: unattended updates run
`install.sh` (and therefore `refresh_auto_updates`) under that very sandbox,
and a sandbox that excludes those paths freezes every installed helper and
unit at whatever a manual install last wrote (issue #1637 triage). Those two
`ReadWritePaths` entries are deliberately directory grants rather than the
four individual file paths: every write commits with a rename from a sibling
staging file, and rename needs write access on the containing directory. The
tradeoff — the update unit can write any file under `/etc/systemd/system` —
is accepted because the unit already runs the full installer as root, so the
grant does not widen what a compromised installer could reach; it only means
the sandbox no longer contains a *buggy* installer's unit writes. Because
the unattended path replaces the helper script that bash is currently
executing, `install_auto_update_assets` must stage the new helper in the
destination directory and swap it in with an atomic same-filesystem rename
only after repo configuration succeeds; no failure path may delete or
truncate a previously working helper (the old `rm -f` on configure failure
left the enabled timer with a dangling `ExecStart`). Every step that produces
one of those three files must be status-checked and must fail the function:
the staging copy of the bundled helper (both call sites invoke
`install_auto_update_assets` under `if !`, which suppresses errexit for its
whole body, so an unchecked `cp` fell through and
`configure_auto_update_script_repo`'s awk turned the empty file into a
plausible one-line shebang-less "helper" that silently disabled updates), and
both unit renders, which are staged to `${path}.tmp` and committed by rename
rather than written with a truncating `cat >` — the function's last statement
is `safe_systemctl daemon-reload`, which returns 0 by design, so an unchecked
write reported success over a truncated unit. The staged helper must also be
rejected before the swap unless it is non-empty and begins with `#!`.

Every update-shaped flow over an existing box (the `--version` path and the
interactive update action, like the reinstall action before them) must repair
a half-removed installation rather than assume the previous install's
environment survived: the flow runs `setup_directories` before anything
writes into the config dir, and recreates the systemd unit via
`ensure_systemd_service_installed` when
`/etc/systemd/system/<service>.service` is missing — while leaving an
existing unit untouched so operator customizations (e.g. a non-default
`FRONTEND_PORT`) survive normal updates. A box whose operator deleted
`/etc/pulse`, the unit file and the binary symlink but kept
`/opt/pulse/bin/pulse` takes exactly this path ("Reinstalling version ..."),
and previously crashed writing `system.json` into the missing config dir
when auto-updates were enabled, or — without them — printed a success
completion while `systemctl enable/start` had failed with "Unit
pulse.service could not be found" behind the unprivileged-container note
(issue #1663). `start_pulse` must never report success when the unit does
not exist at all: the installer always writes the unit file itself, even
where systemctl cannot run, so a missing unit is a broken installation
rather than an unprivileged-container quirk and must fail the run loudly.

The generated `pulse.service` unit also owns the local subscription-agent
execution identity. It pins `HOME` to the Pulse install directory and prepends
that identity's `.local/bin` plus the install `bin` directory to `PATH`, while
retaining `User=pulse` and `ProtectHome=true`. This makes a Claude or Codex CLI
installed and authenticated for the Pulse account discoverable without
exposing a root or interactive user's home and lets updates migrate the unit
contract onto existing systemd installations (#1742).

Changes to the generated units must be able to reach already-deployed boxes.
A box installed before the sandbox was widened runs the installer from a
`pulse-update.service` whose `ReadWritePaths` excludes the helper and unit
directories, so the very run that would install the corrected unit cannot
write it (EROFS) and auto-update alone would never migrate it — the
corrected unit would reach only manually reinstalled boxes. The in-app Go
update pipeline (`internal/updates` `ApplyUpdate`) cannot carry the migration
either: `pulse.service` runs as `User=pulse` under its own
`ProtectSystem=strict` with `ReadWritePaths` limited to the install and
config dirs. `install_auto_update_assets` therefore probes each destination
directory for writability up front and, when one is blocked, hands off to
`migrate_auto_update_assets_outside_sandbox`, which re-execs this same
already-signature-verified installer under `systemd-run` with the internal
`--repair-auto-update-units` entry point. PID 1 forks the transient unit, so
it starts in the host mount namespace instead of inheriting the update unit's
sandbox; the system bus stays reachable from inside that sandbox because
`ProtectSystem=` only remounts the hierarchy read-only and a read-only mount
does not block `connect()` on an `AF_UNIX` socket. The installer is copied
into the install dir before the escape because the calling unit's
`PrivateTmp=yes` hides the auto-update helper's `/tmp` copy of it from PID 1.
The escape must be inert unless it can work and must not recurse: it requires
root and a `systemd-run` on `PATH`, and the escaped run (marked by
`PULSE_AUTO_UPDATE_ASSET_REPAIR=1`) never escapes again. The generated units are
the only unit source — no checked-in reference copies of
`pulse-update.service` / `pulse-update.timer` may exist to drift from the
heredocs. The rendered-unit execution, schedule, sandbox-writability,
failure-preservation, staging-guard, atomic-unit-write, sandbox-escape
migration, refresh-behavior, and call-site wiring tests in
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
to. The standalone helper's documented run-on-each-node path uses
`--local-only` and never invokes SSH. Its optional cluster-wide path must use
`StrictHostKeyChecking=yes` with OpenSSH's already-provisioned trust sources or
an explicit non-empty `--ssh-known-hosts` file isolated from global trust; it
must set `UpdateHostKeys=no` rather than mutate trust during uninstall, never
enroll an unknown key, and missing, unreadable, empty, or mismatched
trust must make the remote portion fail after local cleanup completes.
`scripts/installtests/root_install_sh_test.go` is the owned proof surface for
the root installer's local sensor-proxy cleanup;
`scripts/installtests/uninstall_sensor_proxy_test.go` and
`scripts/release_control/ssh_host_key_policy_test.py` own the standalone
helper's trust and behavior proof.
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
Frontend dependency-security changes use their own proof route rather than
borrowing the local dev-runtime orchestration tests. The canonical
`.github/workflows/build-and-test.yml` frontend job must run both the complete
`npm audit` and the production-only `npm audit --omit=dev` after a clean
install. `frontend-modern/src/security/__tests__/dependencySecurity.test.ts`
pins the known safe floors for advisories remediated by commit `6ba85a185`,
including DOMPurify `GHSA-55q2-fjhq-7xh7`, brace-expansion
`GHSA-mh99-v99m-4gvg` and `GHSA-rgw5-rvv9-x895`, and nanoid
`GHSA-2v37-7h3g-55p8`, while
`scripts/installtests/build_release_assets_test.go` prevents either CI audit
gate from being removed silently. A later advisory must advance these floors
and its sanitizer or dependency-specific regression proof together; audit
suppression is not a valid closure.
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
That same repo-root entry surface owns the mock-mode controls. The
`mock:on`, `mock:off`, `mock:status`, and `mock:edit` wrappers in
`package.json` must delegate to `scripts/toggle-mock.sh`, which is the single
authority for the canonical `PULSE_MOCK_MODE` flag in `tmp/dev-config/.env`.
They may not reimplement the flag write inline. An inline rewrite targets the
repo-root `.env`, which `scripts/hot-dev.sh` does not consult when choosing
the data directory, so the wrapper silently fails to switch modes while
leaving the operator believing it worked. Inline `sed -i` rewrites are also
GNU-only and fail on the default macOS BSD `sed`, and an appending fallback
then accumulates contradictory duplicate flag lines on every invocation.
Any mock control advertised by the `scripts/hot-dev.sh` startup banner must
exist as a wrapper in `package.json`.
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
When Git has already supplied an executable `scripts/dev-launchd-wrapper.sh`,
the setup helper must leave its mode untouched so installation works from
shared checkouts where the developer can write content but cannot change file
modes. It may add the executable bit only when the wrapper is not executable.
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
(`internal/api/configapi/setup_script_render.go`) so token capture does not silently fail
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
Any such instruction that an operator runs through `pct exec` must additionally
name the binary by absolute path. `pct exec` runs with
`PATH=/sbin:/bin:/usr/sbin:/usr/bin`, which excludes the `/usr/local/bin` link
the installer creates, so a bare command name fails with exit 127 at the
operator's very first step. Routing an operator to a correct but unresolvable
command does not satisfy this contract.
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
before installation if the server cannot provide that binary. Both checks must
follow redirects to the operator's final Pulse origin before deciding
reachability or inspecting checksum metadata; an HTTP redirect accepted as a
successful transport response is not the agent artifact contract.
A server may only hand out an agent binary that carries its own agent version.
Local agent artifacts are build outputs that nothing refreshes on their own, so
they go stale silently, and staleness is not cosmetic: the installer renders its
service wrapper from this server's current template, so a binary predating a
flag that template now passes fails to start and crash-loops under its
supervisor. The download path therefore refuses a binary that does not carry the
expected version, alongside the existing report-contract and signature checks.
The expected version resolves from the same source the agent build stamps in,
never from a compiled-in placeholder, because development builds carry
placeholders that no version parser accepts and those are exactly the builds
whose artifacts go stale. Refusal is loud rather than silent: a development
server answers 404 naming the stale path and the build command, and a published
release falls through to the release-asset proxy and fetches the matching
version.
The installer's own version diagnostics answer to the same identity. When it
compares the agent it downloaded against the server that served it, it compares
release identity, stripping a leading `v` and semver build metadata while
keeping the prerelease suffix, because a server built from a working tree
reports metadata the agent never carries. Comparing raw strings made the
mismatch warning fire on every correct development install, and a warning that
fires when nothing is wrong is worse than no warning: it is the only
client-side signal that a stale agent was downloaded, and one that cries wolf
gets skipped the time it is real. Token-bearing
copy-paste commands must pass credentials through ephemeral `--token-file`
transport and leave the installed service configured with the persistent
runtime token file, never a raw `--token` process argument.
The same rule governs credential repair after a server restore, token
revocation, or expiry: the Unix repair handoff must run the normal installer
preflight, pass the fresh scoped credential through a temporary mode-0600 token
file, and invoke update mode without exposing the token in the agent service
arguments. Installer completion is authenticated completion, not merely local
process health. When the post-start registration lookup returns 401 or 403,
`scripts/install.sh` must emit the structured `auth_rejected` completion and
exit 18; it must not print the normal install/upgrade success message or report
the Proxmox registration outcome as though the agent were enrolled.
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
release packet, and make post-publication validation authenticate
`checksums.txt` and every listed artifact's `.sshsig` against the configured
`PULSE_UPDATE_SIGNING_PUBLIC_KEY`, not merely test that sidecars are present.
Validation must fail if the trust root is unavailable, if any signature is
invalid, if any published installer is absent from the authenticated checksum
manifest, if any published artifact or `checksums.txt` is missing its
`.sshsig` sidecar, if the authenticated checksum manifest contains duplicate
asset filenames or trailing fields, or if the canonical
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
digest controls. Stable publication and the stable-path dry-run must skip the
Windows native-signing lane from `v6.3.2` onward while the standing
SignPath-unavailable owner policy is active. New stable versions inherit that
state without per-version allowlist changes. Authenticode returns only through
an explicit reviewed policy/code change after the release owner confirms
production credentials and certificate authorization are ready.
`scripts/build-release.sh` must replace
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
QTS/QuTS hero. Because that root is a small RAM-backed volume that can lack
the headroom to stage or hold the agent at all, the installer must stage,
install, and run the agent binary from the data volume itself, defaulting the
staging `TMPDIR` there when the operator has not chosen one, reclaiming any
pre-relocation runtime copy left under `/usr/local/bin`, and keeping the
boot-time runtime copy only for split layouts where an operator-supplied
state directory separates the stored and runtime binaries.
Before any agent artifact download or replacement, that boundary must also
prove adequate space on the effective temporary and install filesystems,
deduplicating the requirement when both paths share a filesystem and honoring
an operator-supplied `TMPDIR`. `--preflight-only` must exercise the same
storage check and report an actionable alternate-temporary-directory hint.
Generated QNAP and Unraid watchdogs must use the agent's rotating `--log-file`
path rather than an unbounded stdout append, while keeping watchdog-owned
messages independently size-bounded. Because QNAP may launch the same
persistent wrapper from both flash-backed `autorun.sh` and an installer or
upgrader, that wrapper must acquire one portable atomic singleton lock before
killing or starting an agent, track its owned child, recover stale lock state,
and remove only its own PID and lock artifacts on shutdown. Installer
verification must exercise concurrent wrapper launches and prove that they
produce one watchdog/agent pair with no orphan after termination.

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

### OpenShift Helm installation is SCC-owned and socket-free

`openShift.enabled` is the chart-owned compatibility switch for installing the
Pulse server under OpenShift Security Context Constraints. It suppresses the
chart's fixed UID, GID, and fsGroup defaults while retaining non-root,
no-privilege-escalation container posture, so the namespace SCC assigns valid
runtime identities.

When `openShift.kubernetesAgent.enabled` is also true, the chart deploys one
cluster-level Kubernetes collector as a `Deployment`, creates its service
account plus a read-only ClusterRole/Binding, assigns a stable cluster agent
ID, enables Kubernetes collection, disables host collection, and omits the
Docker socket. Normal chart behavior is unchanged when the OpenShift switch is
false. The default role does not grant Secrets or `nodes/proxy`; the metrics
API is the supported OpenShift node/pod usage path, and unsupported
OpenShift-native Routes and DeploymentConfigs remain outside this slice.

`.github/workflows/helm-ci.yml` must render the profile and fail if the output
contains a Docker socket mount or fixed `runAsUser`, `runAsGroup`, or `fsGroup`
values. `scripts/installtests/build_release_assets_test.go` pins the packaged
values, templates, RBAC, render assertions, and operator documentation.

The provider MSP bundle must be installable without a prior exchange with
Pulse. `deploy/provider-msp/setup.sh` treats a blank
`CP_PROVIDER_MSP_LICENSE_FILE` as evaluation and proceeds, reporting the
two-workspace cap and printing the lease signing public key for a later licence
request. A non-empty licence path that does not resolve to a file remains a
hard failure, because that is a misconfiguration rather than a choice.

`setup.sh` resolves any blank or placeholder image variable
(`TRAEFIK_IMAGE`, `DOCKER_SOCKET_PROXY_IMAGE`, `CONTROL_PLANE_IMAGE`,
`CP_PULSE_IMAGE`) to an immutable digest from its published tag using
`docker buildx imagetools inspect`, and writes the resolved digest back to
`.env`. Operator-supplied values are left untouched. The shipped
`.env.example` must therefore carry blank image variables rather than
unfillable `@sha256:<pin>` placeholders, and a blank
`CP_PROVIDER_MSP_LICENSE_FILE`, so the default install path requires no
credentials and no correspondence. All four images are publicly readable, so
digest resolution must not assume registry authentication.

When no licence path is set, `setup.sh` self-issues a capped evaluation licence
from the Pulse licence server (`POST /v1/provider-msp/eval-license`) bound to
the lease signing public key derived from the locally generated private key,
which never leaves the host. Without this the evaluation is hollow: an
unlicensed control plane starts, but release-build client runtimes reject its
unchained entitlement leases and the client workspaces run without the
capabilities being evaluated.

Current setup code issues an evaluation only after validating the operator
configuration and confirming that every immutable provider image is reachable,
and records the fixed `setup_stage=images_ready` value. That combination makes
the stored `msp_eval` row an activation signal rather than a download-intent
signal created before the install can succeed. Setup may
include an optional evaluator email and a fixed signup-source label so support
can match an assisted paid upgrade to the deployment. Those fields must remain
optional and bounded; the request must never contain client inventory,
credentials, private keys, or free-form runtime telemetry. The returned public
evaluation licence ID may be carried in the upgrade URL as a non-secret
correlation key.

That ordering and attribution contract applies only when the request carries
the fixed `setup_stage=images_ready` marker. The immutable v6.2.1 provider
bundle predates the marker and requests its evaluation earlier, so its stored
`msp_eval` row proves issuance only, not install readiness, and remains
anonymous. Public v6.2.1 guidance must state that exact behavior. It must not
claim automatic contact correlation or post-image-ready activation until a
newer signed bundle containing the contract is published.

Self-issue must degrade rather than block. A missing signing key, an
unreachable licence server, or a response carrying no licence leaves the
install unlicensed with an explicit warning, and `PULSE_PROVIDER_MSP_SKIP_EVAL_LICENSE`
skips the request outright for air-gapped hosts. An existing evaluation licence
on disk is reused rather than re-requested. The install must never abort
because an evaluation licence could not be obtained.

### Least-privilege agent install profile

The unified agent installer (`scripts/install.sh`) offers
`--least-privilege` on standard Linux systemd hosts: a dedicated nologin
`pulse-agent` system user owns the service, state directory, and binary;
docker-group membership covers socket reads; and the optional
`--grant-smart` / `--grant-pct` flags install visudo-validated,
exact-command sudoers rules with root-owned wrapper helpers wired through
`PULSE_SMARTCTL_PATH` / `PULSE_PCT_PATH`. Installability boundaries: the
flag is refused (never silently downgraded to root) on appliance platforms
and non-systemd init systems, is mutually exclusive with
`--enable-commands`, and `--update` preserves an installed profile and its
grants by reading the existing unit. A unit with an active grant sets
`NoNewPrivileges=false` because NNP blocks sudo (proven on a live systemd
host); a grantless profile keeps `NoNewPrivileges=true`. Uninstall removes
the sudoers file and helper directory. `scripts/installtests/install_sh_test.go`
(`TestInstallSHLeastPrivilegeProfile`) pins these invariants.

### Publication locks the complete release packet

GitHub release immutability is a mandatory activation control. The release
workflow must create and validate a draft, stage `release-activation.json`, and
compare GitHub's stored SHA-256 digest for that marker with the local bytes
before publication. Publication, not a later asset upload, is the irreversible
boundary. Immediately before both normal and recovery publication, an
authenticated Administration-read request to GitHub's repository immutable
releases endpoint must prove that the setting is enabled. An unavailable,
unauthorized, malformed, or disabled response fails closed while the release is
still a draft. GitHub must also return `immutable: true` after publication;
otherwise the workflow must fail and compensate the still-mutable publication
back to a marker-free draft.

`scripts/verify-github-release-integrity.sh` is the shared post-publication
check. It binds the release database ID, tag, exact source SHA, immutable state,
and single digest-bearing activation marker, then requires `gh release verify`
to validate GitHub's signed release attestation. It must then download the
activation marker from that release and require `gh release verify-asset` to
bind the exact consumed bytes to the signed release attestation. Filename,
stored digest presence, and marker JSON identity are not substitutes for this
asset proof. The source release verdict, activation-only recovery, and
`release-convergence.yml` must all use that check. Convergence must not acquire
the customer-promotion lease or mutate a floating image tag, Helm index,
paid-runtime pointer, or live environment until the check passes. Repository
release immutability must therefore be enabled before merging or running this
activation path.

Attestation policy decisions require GitHub CLI 2.97.0 or newer so signer
repository and workflow names are matched literally. The shared verifier must
also bind the downloaded `checksums.txt` bytes to the immutable release and to
SLSA v1 provenance from the exact expected source SHA while rejecting
self-hosted provenance. A manifest-bound
`release-build-provenance.sigstore.json` must itself pass the immutable release
asset proof and must verify `checksums.txt` from the exact
`build-release-candidate.yml` signer using that local bundle. Immutable
historical releases without a portable candidate bundle retain verification
against their original `create-release.yml` publication provenance. Multi-asset
download retries clear the activation marker, checksum manifest, and portable
bundle first so a partial attempt cannot poison every later retry.

### Stable v6.4.2 patch cutoff

The stable v6.4.2 packet remains centered on the infrastructure-action and SSO
administrator-boundary fixes. Its final mainline cutoff also names every
customer-visible change that landed after the initial packet preparation:
bounded security setup request decoding, accurate PBS running and incomplete
backup state, Overview-level notification retry and dismissal, canonical
Assistant command-help dialog behavior, and agent server-address migration
guidance. The cutoff also explains the Preview channel's distinct beta and
release-candidate maturity and records native systemd journal severity for the
generated Pulse service. It also records provider-scoped Proxmox identity-pin
recovery that keeps same-name estates distinct after restart, per-node Proxmox
cluster agent coverage that cannot hide uncovered members, and bounded Agent
Doctor reporting for typed privilege-helper degradation. Release-integrity
detail records authenticated Unified Agent download validation, exact-step
candidate version binding, separate GHCR authentication for Helm chart upload
and OCI provenance attestation, and the protected checkout baseline across
release automation. Packet proof must retain those outcomes before v6.4.2 can
be dispatched for publication.

### Systemd journal output preserves Pulse severity

The generated Pulse server unit keeps stdout and stderr attached to the journal,
enables `SyslogLevelPrefix`, and opts the process into the logging package's
level-aware stream writer. That writer maps zerolog warning, error, fatal,
panic, info, debug, and trace records to systemd priorities before the message
crosses the journal stream. systemd consumes the prefix, so `MESSAGE` remains
the original structured log record and `journalctl -p` plus downstream syslog
forwarding can use `PRIORITY` without a shell wrapper.

The opt-in belongs only to the generated systemd service. Container and
terminal output remain unprefixed, and the logger tees the unmodified record to
the rotating file sink and authenticated live-log broadcaster. Installer and
logging tests pin the unit directives, level mapping, and sink isolation.

### Release automation preserves protected checkout semantics

Every repository checkout in build, packaging, publication, qualification,
recovery, and deployment automation uses the reviewed immutable
`actions/checkout` v7.0.1 pin. That baseline refuses fork pull-request checkout
on privileged events unless a workflow explicitly opts out; Pulse prohibits
that opt-out and the `pull_request_target` trigger. Dependency refreshes must
update the central workflow-trust allowlist and its regression proof together,
so a routine pin change cannot silently remove this release-automation trust
boundary. `scripts/check_workflow_trust.py`,
`scripts/tests/test_workflow_trust.py`, and
`scripts/installtests/build_release_assets_test.go` pin the policy and the
release-workflow integration.
