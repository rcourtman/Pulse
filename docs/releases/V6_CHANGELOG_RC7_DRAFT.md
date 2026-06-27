# Pulse v6.0.0-rc.7 Draft Changelog

_Draft only. This changelog describes the current `pulse/v6-release` delta
since the published `v6.0.0-rc.6` tag. Do not treat it as published until the
governed `v6.0.0-rc.7` prerelease exists._

## What `rc.7` changes compared with `rc.6`

`v6.0.0-rc.7` is a renewed prerelease validation build. It keeps the
platform-shaped top-level navigation restored in `rc.6`, but it carries a much
larger post-RC6 branch delta across Assistant, Patrol, availability checks,
platform-table consistency, provider MSP, commercial continuity, release
tooling, installers, security hardening, and monitoring correctness.

The branch also contains earlier `v6.0.0` stable-promotion packet work. RC7
supersedes that operationally by keeping the next public v6 artifact on the
prerelease channel for another evaluation pass.

## Commit coverage audit

The changelog is audited against every feature/runtime commit in the exact
validation-risk range. A later packet-only refresh may be the workflow dispatch
head; the validation range below is the code-backed release-risk range.

- `v6.0.0-rc.6`: `c25e95cb2b071551df95c8add62773905ba0628b`
- validation-risk commit: `55204cde9b93004fb04850b638de38ac3abaa27e`
- range: `v6.0.0-rc.6..55204cde9b93004fb04850b638de38ac3abaa27e`
- commit count: `940`
- changed scope: `1966` files, `236770` insertions, `46839` deletions

Those commits are grouped here by operator-visible behavior and release risk
instead of listed one by one.

## Major changes

### 1. Patrol is the visible checking-loop surface

- Patrol open work is visible in navigation.
- Alerts route investigation into Patrol instead of generic Assistant copy.
- Findings show actionable collapsed-state badges, expanded recommendations,
  and clearer evidence under "What Pulse checked".
- Patrol run history, approval sections, finding lifecycle, and verification
  handling received additional proof and UI consistency work.
- Free/self-hosted Patrol surfaces stay clean of default paid-feature pressure;
  plan-locked modes are disclosed deliberately.

### 2. Assistant became more contextual and recoverable

- Assistant now has live tool progress, tool starts, tool output previews,
  streamed argument progress, queued follow-up controls, and stronger status
  handling while work is running.
- Provider fallback, route validation, failed-turn recovery, retry behavior,
  and model-route labels were hardened.
- Resource-context targeting and inventory answers were tightened so Assistant
  can explain selected infrastructure without fabricating unsupported discovery.
- Chat output hygiene, markdown rendering, transcript export, slash-command
  help, mention autocomplete, and command help were improved.
- Browser notifications can fire when Assistant finishes or needs attention
  while the tab is in the background.

### 3. Availability checks attach to known resources

- Agentless availability checks can attach as facets on the resource they
  monitor through explicit resource links or unambiguous address/hostname
  correlation.
- Standalone network endpoints remain the fallback for genuinely unowned
  targets.
- Platform rows gained compact availability readouts and protocol identity.
- Discovery suggests availability probes from detected service types and from
  existing discoveries.

### 4. Platform pages gained consistency and density

- Shared platform tables, filters, tabs, badges, callouts, buttons, loading
  states, drawer sections, icon actions, scalar values, and empty states were
  moved onto canonical primitives.
- Proxmox backups gained scoped filters, saved views, flattened tables, better
  PBS identity, backup-age and coverage signals, restored node signals, and
  corrected backup row labeling.
- Docker and Podman gained governed lifecycle actions, action-readiness reasons,
  nested Docker context in LXC drawers, and host identity fixes.
- Kubernetes gained namespace and cluster filters, richer drawers, deployment
  and configuration filtering, restored cluster fractions, and status filtering.
- Machines, storage, workloads, and recovery views regained or tightened
  at-a-glance signals without moving away from the unified resource contract.

### 5. Commercial, Cloud, and provider MSP paths hardened

- The self-hosted free-first commercial posture remains the default: current
  public self-hosted plans include core monitoring.
- Relay value remains framed around secure remote access to the Pulse web UI,
  Pulse Mobile pairing for handoff, push notifications, and longer history.
- Cloud, account, billing, support, migration, and entitlement copy were aligned
  with the current v6 commercial model.
- Provider MSP gained control-plane mode, install artifacts, preflight/status
  commands, proof commands, backup/recovery operations, token-rotation proof,
  isolation tests, and rollout upgrade proof.
- Private Pro release and paid runtime build-attribution proof were recorded.

### 6. Security, installer, update, and release hardening

- Restricted outbound HTTP proxy bypass hardening landed.
- Local-network API connections can route through subprocess networking to
  avoid Tailscale NECP failures.
- Workflow action pins, Go toolchain selection, release dry-run, release asset
  validation, release promotion policy support, installer smoke tests, Docker
  and Helm publication paths, and update readiness checks were tightened.
- RC7 Docker install defaults now pin the governed `6.0.0-rc.7` image in both
  the repo-root Compose sample and Docker bootstrap installer fallback, while
  keeping stable-promotion proof guarded against leftover prerelease defaults.
- Installer scripts gained additional update resilience, Windows agent install
  coverage, root install tests, uninstall sensor-proxy support, and bundled
  agent installer fail-closed behavior.
- Bootstrap token, webhook tenant identity, audit log resilience, API token,
  and tenant boundary tests were expanded.

### 7. Monitoring, metrics, and resource correctness

- Metrics database pruning, metrics-store freelist behavior, and chart history
  coverage were improved.
- Physical disk I/O metrics no longer disappear when SMART data is empty.
- ZFS/Ceph/Proxmox/TrueNAS/Docker/Kubernetes/vSphere telemetry and alert
  handling gained targeted fixes.
- Unified resource identities, sightings, relationship presentation, retention,
  availability linking, canonical IDs, and top-level system projections were
  tightened.
- WebSocket lifecycle, alerts activation state, notification scheduling, and
  monitor reload behavior received correctness fixes.

### 8. Localization and public documentation foundation

- German and Spanish public documentation entry points were added.
- Locale preference persistence and i18n catalog support landed for selected
  settings, first-session, and alert-overview journeys.
- Public docs and shipped frontend docs were synchronized for configuration,
  privacy, security, README, webhooks, MSP, Cloud, and upgrade surfaces.

## Validation focus for RC7

- Release dry run and draft release workflow on `pulse/v6-release`.
- Release assets, checksums, signatures, Docker image, Helm smoke, and preview
  demo routing.
- Upgrade from v5 stable and from earlier RCs, especially `rc.2` trust-root
  continuity.
- Patrol and Assistant with real alert/finding/resource context.
- Availability facet attachment on real mixed-estate resources.
- Provider MSP and Cloud proof environments.
- Self-hosted commercial continuity and paid-license migration.
- Proxmox, Docker, Kubernetes, TrueNAS, vSphere, Machines, Storage, and Alerts
  browser walkthroughs.

## Known release posture

- `v6.0.0-rc.7` is prerelease only.
- Pulse v5.1.35 remains stable.
- The rollback command for this candidate is
  `./scripts/install.sh --version v5.1.35`.
- Do not describe VMware as broadly supported beyond the governed support floor.
- Do not describe Cloud or provider MSP flows as ready for broad public rollout
  unless their current proof environment has been exercised for this RC.
