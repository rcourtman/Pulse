# Pulse v6.1.0-rc.3 Release Notes

`v6.1.0-rc.3` is a release candidate for the next Pulse v6 minor line. It
follows stable `v6.0.5`, retains the substantial monitor-first product update,
typed Pulse Intelligence action lifecycle, dedicated Actions workspace, safer
native-agent update and recovery behavior, and broad security and reliability
hardening from the earlier candidates, and supersedes `v6.1.0-rc.2` with fixes
driven directly by release-candidate feedback across agent command enrolment,
Docker update actions, storage identity, TrueNAS accuracy, Patrol reliability,
and update selection.

## Highlights

- Patrol findings can now move through one reviewed Actions inbox with clear
  approval, execution, and verification state.
- Pulse can safely carry out a wider set of explicitly governed Docker,
  Proxmox, host-update, package-maintenance, and storage-cleanup actions.
- Platform pages, connected systems, responsive layouts, and Assistant
  conversations are more task-focused and easier to operate day to day.
- Native updates, agent recovery, authentication boundaries, and service
  hardening fail closed across more installation and recovery paths.
- Patrol investigations now keep evidence and model-turn budgets bounded,
  preserve multiple grounded findings, and separate Watch detection from
  model-led investigation.
- Claude subscription-backed models can use schema-bound streaming and native
  typed tools without creating a parallel action-execution path.
- Enabling command execution when adding a Pulse Agent now mints an install
  token that actually carries the command permission, and the command channel
  accepts it on first registration.
- TrueNAS storage surfaces read the API shapes TrueNAS actually serves, and
  physical disks resolve their ZFS pool membership for nvme-eui and
  namespace-suffixed member references.
- Patrol findings can notify through the same channels as alerts, and manual
  Patrol runs no longer report a false timeout while a slow self-hosted model
  is still warming up.

## Added

- Pulse Intelligence now has explicit detection and investigation profiles,
  a typed proposal-to-action lifecycle, and post-action verification for
  supported Docker and Kubernetes operations.
- Patrol action state now reconciles from the authoritative action audit and
  stays current across investigation history, desktop approval controls, and
  Pulse Mobile approve or reject flows.
- Actions provides a dedicated inbox for reviewing proposed work, checking
  policy and verification details, and seeing pending approvals without
  searching through Assistant history.
- Patrol can authorize low-risk Docker and Podman restarts through explicit
  per-resource capability allowlists and optional recurring maintenance
  windows, while unsupported, out-of-window, or downgraded-mode actions remain
  approval-gated and fail closed.
- Governed host updates, Debian and Ubuntu package maintenance, storage-pressure
  cleanup, and supported Proxmox guest lifecycle operations now use reviewed
  plans, durable execution receipts, and independent outcome verification.
- Local AI setup includes a guided Ollama quickstart for `qwen3:8b`, with
  clearer Provider & Models readiness guidance.
- Cluster members can override their connection addresses when the discovered
  address is not the one Pulse should use.
- The Unified Agent Windows service now writes owner-controlled rotating logs,
  verifies logged readiness during installation, and carries native lifecycle
  proof for install, replacement, recovery, persistence, and uninstall.
- A live Patrol qualification path exercises model-led investigations, finding
  quality, remediation planning, typed tool use, and negative controls.
- Claude subscription-backed models support bounded preflight, streaming native
  tool calls, retry-safe durable outcomes, and explicit separation from
  API-billed provider routes.
- Docker inventory warns when two machines report the same agent identity, and
  registry pulls can negotiate bearer tokens from authentication challenges.
- New Patrol findings can be routed to the configured alert notification
  channels, with settings exposed in the API and the Patrol settings page.
- The Docker thresholds page exposes the container update-alert delay,
  including an off state that fully disables update alerts.
- Relay is discoverable where users configure alerts, the Assistant is
  surfaced during first AI setup, and the external-agent (MCP) connector
  setup is findable from sidebar search.

## Improved

- Platform and connected-system pages lead with monitor-first attention and
  task-oriented workflows, with more coherent responsive and mobile layouts.
- Pulse Intelligence settings and daily-use surfaces use one consistent
  product vocabulary while keeping Patrol focused on detection and
  investigation.
- The provider MSP portal uses the product design system, supports dark mode,
  and presents self-service behavior honestly when an email provider is not
  configured.
- Update execution now uses one canonical lifecycle with clearer completion,
  rollback, and operator feedback.
- Investigation prompts receive the real typed capability catalog, including
  approval requirements and parameter constraints, instead of asking the
  model to guess which actions are available.
- Assistant conversations can be retried, regenerated, edited and resent, and
  steered while a response is running. Long pasted input is collapsed into a
  manageable composer attachment, and the last-turn summary reports estimated
  model cost when available.
- Patrol handoffs now open the related Actions review directly, background
  Patrol work stays out of the Assistant quick-resume list, and the Actions tab
  shows its pending-approval count.
- Docker, Kubernetes, TrueNAS, vSphere, and Proxmox node tables preserve
  user-controlled column sorting through one shared platform-table model.
- Multi-finding Patrol runs preserve accepted siblings and sequential findings,
  while provider retries and replay handling retain durable outcomes without
  upgrading incomplete evidence.
- Docker update and restart work stays on reviewed typed plans with durable
  receipts and recovery after server or agent reconnection.
- Commercial plan, cadence, entitlement, revocation, and downgrade handling now
  shares an installation-scoped lifecycle that preserves customer data.

## Fixed

- Add Pulse Agent minted install tokens without the command-execution scope
  even when Enable Pulse command execution was ticked, so agents could never
  register the command channel. Tokens are now minted server-side with the
  scopes the checkbox asks for and the binding the command channel requires,
  the token regenerates when the checkbox is toggled, and the rejection
  message names the actual recovery step.
- Pool membership resolution missed disks referenced as `nvme-eui.<hex>` or
  with a trailing namespace suffix in zpool member links, so those disks
  showed a generic ZFS label instead of their pool name.
- TrueNAS datasets, disks, and pools no longer show Offline, Attention, or
  Unknown when the underlying system is healthy, and per-system storage no
  longer swaps names with the system selected in Overview.
- Disabling all Docker container or service alerts now clears already-active
  update alerts instead of letting them keep notifying, and the update-alert
  delay has an explicit off state.
- The container update button is disabled up front when the server refuses
  the update capability, failed update plans surface the refusal reason, and
  the agent-too-old refusal explains what to do.
- Manual Patrol runs reported a connection timeout when a slow provider had
  not streamed within fifteen seconds even though the run had started;
  failure is now only reported once a status refresh confirms no run is in
  progress.
- In-app updates select releases by highest version rather than GitHub list
  order, and malformed release notes fail closed.
- Webhook delivery and outbound security dialing try every permitted
  resolved IP instead of pinning the first.
- Approved-action dispatch survives client disconnects, unavailable reviewed
  actions surface their real cause, and the Docker agent bounds its collect
  cycle with a watchdog so a hung container daemon cannot stall reporting.
- With no systems connected, the page-header and setup-band calls to action
  are now distinctly named, and telemetry pings are suppressed while mock
  mode is enabled.
- Native updates self-test the replacement binary before swapping it in,
  reject silent edition downgrades, preserve a sanctioned rollback path, and
  fail fast when signing configuration is incomplete.
- Native updates fail closed instead of silently falling back to a community
  build, preserve writable configuration backups under hardened services, and
  publish verification keys in the exact OpenSSH `allowed_signers` form used
  by the documented verification command.
- Docker updates now recreate the container instead of attempting a restart
  that cannot apply a new image.
- Docker containers retain their grouped-by-host view and open configured web
  links consistently after REST resource snapshot hydration.
- Docker and Kubernetes agents tolerate realistic clock skew when evaluating
  liveness, and posture alerts no longer ignore the intended guest-suppression
  rules.
- Legacy OIDC callbacks recover the initiating provider correctly.
- Provider/runtime failures and proposal-validation failures remain separate,
  so a failed investigation cannot be misreported as a completed
  needs-attention result.
- First-run, request parsing, storage, cookie, remediation-lock, and remote
  deployment boundaries now fail closed across the hardened paths included in
  this candidate.
- FreeBSD agent update recovery and Windows service recovery now preserve a
  usable runtime across replacement and restart paths.
- Cluster re-registration preserves an operator-selected member address, moved
  guests keep their alert ownership aligned with the new node, and unavailable
  guest-agent disk data is no longer presented as a real measurement.
- Physical disks no longer disappear on wide node layouts, standby SSDs no
  longer report misleading state, shared Docker network namespaces survive
  container updates, and SSO administrators retain the expected settings
  authority.
- Patrol rejects untrusted prompt instructions, unsupported reconfirmation
  shortcuts, and ungrounded health claims; repeated restarts and Docker OOM
  events now use authoritative evidence.
- OIDC sessions without refresh tokens remain valid where allowed, mixed-auth
  startup avoids deadlock, and Basic-auth identity reaches action authorization.
- Deleted hosts can re-enroll with fresh credentials, agent configuration stays
  available from continuity state during reload windows, and Windows version
  checks normalize a leading `v`.
- Constrained NAS installs no longer require `od`; recovery-point, TrueNAS, and
  `nvme-eui` ZFS disk identity reconciliation retain authoritative sources.
- Availability polling honors its configured interval, alert email times include
  their timezone, and no-op Docker update status no longer creates false history.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.1.0-rc.3` only when you are
comfortable testing an RC. The rollback target for this release candidate is
`v6.0.5`.

The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.0.5
```

This candidate changes authentication and native installer/updater boundaries,
so it is intentionally using the governed RC path rather than the direct
stable-patch path.

Pulse Mobile iOS candidate build 10 and Android candidate versionCode 8 carry
the matching plan-bound action review and approval client. They remain on the
TestFlight and Google Play internal-testing tracks; no public store rollout is
part of this RC.

Windows Unified Agent binaries in this release candidate retain the same
checksum and detached-signature verification used by `v6.0.5`, but they are
not yet Authenticode-signed and Windows may show an unknown-publisher warning.
Public Windows Authenticode signing remains required before stable promotion.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
