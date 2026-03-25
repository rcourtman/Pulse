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
4. `scripts/build-release.sh`
5. `scripts/release_ldflags.sh`
6. `scripts/install.ps1`
7. `scripts/install.sh`
8. `scripts/install-container-agent.sh`
9. `scripts/pulse-auto-update.sh`
10. `scripts/hot-dev.sh`
11. `scripts/hot-dev-bg.sh`
12. `tests/integration/scripts/managed-dev-runtime.mjs`
13. `package.json`
14. `frontend-modern/package.json`
15. `scripts/dev-check.sh`
16. `scripts/toggle-mock.sh`
17. `scripts/clean-mock-alerts.sh`
18. `scripts/dev-launchd-setup.sh`
19. `scripts/dev-launchd-wrapper.sh`
20. `scripts/com.pulse.hot-dev.plist.template`
21. `.github/workflows/create-release.yml`
22. `.github/workflows/release-dry-run.yml`
23. `.github/workflows/publish-docker.yml`
24. `.github/workflows/promote-floating-tags.yml`

## Shared Boundaries

1. `frontend-modern/src/api/updates.ts` shared with `api-contracts`: the updates frontend client is both a deployment-installability control surface and a canonical API payload contract boundary.
2. `internal/api/updates.go` shared with `api-contracts`: update handlers are both a deployment-installability control surface and a canonical API payload contract boundary.
3. `scripts/install.ps1` shared with `agent-lifecycle`: the Windows installer is both a deployment installability entry point and a canonical agent lifecycle runtime continuity boundary.
4. `scripts/install.sh` shared with `agent-lifecycle`: the shell installer is both a deployment installability entry point and a canonical agent lifecycle runtime continuity boundary.

## Extension Points

1. Add or change deployment-type detection, update planning, or apply behavior through `internal/updates/`
2. Add or change release-build metadata injection, release artifact assembly, or governed promotion metadata resolution through `scripts/build-release.sh`, `scripts/release_ldflags.sh`, and `scripts/release_control/resolve_release_promotion.py`, plus the container Dockerfile and governed release workflows that consume those same contracts
3. Add or change shell installer, Windows installer, container-agent installer, or auto-update script behavior through `scripts/install.sh`, `scripts/install.ps1`, `scripts/install-container-agent.sh`, and `scripts/pulse-auto-update.sh`
4. Add or change server update transport through `internal/api/updates.go` and `frontend-modern/src/api/updates.ts`
5. Add or change local dev-runtime orchestration, managed ownership, browser-runtime proof wiring, frontend/backend coherence diagnostics, canonical developer entry wrappers, or dev-runtime helper control surfaces through `scripts/hot-dev.sh`, `scripts/hot-dev-bg.sh`, `tests/integration/scripts/managed-dev-runtime.mjs`, `package.json`, `frontend-modern/package.json`, `scripts/dev-check.sh`, `scripts/toggle-mock.sh`, `scripts/clean-mock-alerts.sh`, `scripts/dev-launchd-setup.sh`, `scripts/dev-launchd-wrapper.sh`, and `scripts/com.pulse.hot-dev.plist.template`
6. Add or change governed release-promotion workflow inputs, operator-facing promotion metadata, prerelease lineage enforcement, or stable-promotion rehearsal summaries through `.github/workflows/create-release.yml`, `.github/workflows/release-dry-run.yml`, `.github/workflows/publish-docker.yml`, and `.github/workflows/promote-floating-tags.yml`

## Forbidden Paths

1. Leaving deployment bootstrap, installer, or update-runtime files unowned under broad monitoring or generic API ownership
2. Duplicating deployment-type update planning, installer release resolution, or updater handoff behavior outside the canonical update engine and installer scripts
3. Treating update transport as payload-only contract work when it also defines live deployment and upgrade behavior

## Completion Obligations

1. Update this contract when canonical deployment or installer entry points move
2. Keep deployment runtime and shared API proof routing aligned in `registry.json`
3. Preserve explicit coverage for installer parity, update planning, and deployment bootstrap behavior when these surfaces change

## Current State

This subsystem now makes deployment planning, updater orchestration, and the
non-shell installer/update scripts explicit inside the current self-hosted
release-confidence lane instead of leaving them as implied behavior around the
core runtime.

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
That release-build metadata path is now explicit too: `scripts/release_ldflags.sh`
is the canonical owner for server and agent build ldflags, and release artifact
assembly must route through it instead of hand-writing overlapping `main.Version`,
`internal/updates.BuildVersion`, `internal/dockeragent.Version`, or license-key
injection fragments across `scripts/build-release.sh`, `Dockerfile`, and the
demo-deploy workflow. Shipped binaries, installable container images, and
governed deployment-build workflows must all carry the same build metadata
contract rather than depending on whichever local ldflags string happened to be
updated last.
The same governed promotion path must now stay explicit too:
`scripts/release_control/resolve_release_promotion.py` is the canonical owner
for stable-versus-prerelease metadata validation shared by `.github/workflows/release-dry-run.yml`
and `.github/workflows/create-release.yml`. Promotion rollback targets, promoted
prerelease lineage, soak checks, and GA/v5 notice metadata may not drift between those
two workflows through duplicated inline shell validation.
Those same governed release workflows also own the operator-facing wording for
that promotion metadata. Human-visible workflow inputs, summaries, and error
messages must describe the path as a prerelease or preview flow rather than
implying a near-ready release candidate, while machine-owned identifiers such
as `rc`, `rc-to-ga-*`, and `v6.0.0-rc.1` remain the canonical internal keys.
That same prerelease framing requirement also applies to installer and update
runtime copy: `install.sh`, `scripts/pulse-auto-update.sh`, and
`internal/updates/manager.go` must present `rc`-tagged builds as prerelease or
preview paths in menus, CLI help text, operator diagnostics, and runtime logs
rather than as release-candidate promises.
Those same workflows must also fetch and dispatch the governed release branch
derived from release-control metadata instead of hardcoding `pulse/v6`,
`pulse/v6-release`, or any later branch literal inline.
That same `internal/updates/` boundary now also owns runtime data-dir
authority for temp, backup, and cleanup behavior: `manager.go` must resolve
its working directories through the shared runtime data-dir helper instead of
rebuilding `PULSE_DATA_DIR` plus `/etc/pulse` fallback logic inside each
update stage.
That same boundary also governs install.sh rollback restore targets:
`adapter_installsh.go` may not hardcode `/etc/pulse` for rollback safety
backups or config restore, and must derive the rollback config directory
through that same shared runtime data-dir helper.
That same runtime env contract also governs `pulse mock`: the CLI may not keep
writing a separate `mock.env` sidecar when supported runtime installs already
carry mock-mode ownership through `.env`. Mock enable/disable/status must use
the canonical runtime `.env` path, with any install-dir `.env` probe treated as
compatibility only.
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
The local dev-runtime launcher now sits on that same installability boundary.
`scripts/hot-dev.sh` and `scripts/hot-dev-bg.sh` are the canonical owned entry
points for a coherent local Pulse runtime, so frontend shell health, proxy
health, backend health, and listener ownership diagnostics may not drift into
ad hoc shell snippets or undocumented operator lore outside those scripts.
When the managed launcher reports runtime status, it must tell operators which
browser URL to use and whether the frontend shell, proxied API path, and
direct backend health endpoint all agree, instead of leaving `5173` versus
`7655` interpretation to manual inference from whichever process still happens
to be listening.
Changes to `scripts/hot-dev.sh` and `scripts/hot-dev-bg.sh` must therefore
stay on their own direct dev-runtime orchestration proof path instead of
piggybacking on installer proof coverage for unrelated deployment scripts.
That same dev-runtime orchestration boundary also owns watcher stability for
the managed local stack: `scripts/hot-dev.sh` may only rebuild the backend for
runtime Go sources, not `*_test.go` churn, and it must suppress `pulse` binary
change events produced by its own successful managed rebuilds, managed backend
restarts, or startup build.
Otherwise unrelated parallel test edits or hot-dev's own binary output can
tear down `7655`, produce transient `5173` proxy failures, and undermine the
canonical browser-runtime proof path.
`scripts/hot-dev-bg.sh` must also supervise `scripts/hot-dev.sh` in an isolated
child session so an unexpected owner-process death cannot leave orphaned
watchers or health monitors behind. When the supervisor replaces the managed
child, it must terminate the old child process group before starting the next
one.
`scripts/hot-dev-bg.sh verify` must also establish a managed verification lock
for the duration of the proof pack, pass that lock path into the integration
runner, and keep the lock owned by the actual browser-proof process lifetime
across pretest, Playwright, and posttest. `scripts/hot-dev.sh` must honor that
lock by suppressing source-triggered rebuilds and manual `pulse` binary restart
churn while the owning proof process is still alive. Stale verify locks must
clear themselves automatically once the owning process exits.
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
That same installer-owned bootstrap step against `/api/setup-script-url` must
also validate the returned canonical `type`, normalized `host`, and live
`expires` metadata before using the one-time setup token, so install-time
registration cannot drift onto a stale or mismatched bootstrap response. A
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
