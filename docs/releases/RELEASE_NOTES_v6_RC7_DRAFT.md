# Pulse v6.0.0-rc.7 Draft Release Notes

_Draft only. Do not treat this as published until the governed
`v6.0.0-rc.7` tag and GitHub prerelease exist._

## What this RC is, and what it is not

`v6.0.0-rc.7` is an opt-in prerelease for renewed v6 testing. It is not the
stable v6 release and should not be presented as general availability.

Pulse v5.1.35 remains the current stable line. The stable rollback target for
this candidate is:

`./scripts/install.sh --version v5.1.35`

This RC deliberately keeps v6 on the prerelease channel after the earlier GA
packet preparation. The branch currently contains stable-release documentation
drafts, but GitHub does not yet have a published `v6.0.0` release. Treat RC7
as a fresh evaluation packet for the current `pulse/v6-release` head.

## Why cut RC7

`rc.6` reset the frontend to platform-shaped navigation while keeping the v6
unified resource model. Since then, the branch has taken a large set of product,
runtime, release-pipeline, and security changes that should be exercised as a
prerelease before any stable v6 promotion:

- Assistant and Patrol were reshaped around monitor-first operations, contextual
  investigation, live progress, and safer action handoff.
- Availability checks now attach back to the known resource they monitor instead
  of always appearing as disconnected network endpoints.
- Discovery gained service-context readiness and availability probe suggestion
  flows for existing and newly discovered systems.
- Provider MSP, Cloud, commercial continuity, and release packaging paths were
  hardened further.
- Platform surfaces kept the v5-shaped navigation, but gained substantial
  table, drawer, filter, and action consistency work.
- The release and install pipeline has newer branch-policy, workflow-pin,
  installer, update, and proof hardening than the shipped RC6 packet.

## What changed since `rc.6`

### Monitor-first Patrol and Assistant operation

Patrol is now more clearly the checking-loop surface for findings, approvals,
and verification work. Alert investigation routes into Patrol, the navigation
exposes open work, finding rows show actionable state, expanded findings explain
what Pulse checked, and Assistant handoff copy is contextual instead of making
generic chat promises.

Assistant gained a large reliability and usability pass: live tool progress,
streaming status, provider-route recovery, queued follow-ups, slash commands,
tool-output previews, transcript export, cleaner error recovery, contextual
resource targeting, and stricter output hygiene. The goal for RC7 testing is
to confirm that Assistant helps explain and act on selected context without
becoming the primary destination.

### Availability checks as resource facets

Agentless availability checks can now attach to the known resource they monitor.
The backend supports explicit resource links and unambiguous address or hostname
correlation before falling back to standalone network endpoints. Platform rows
surface compact availability readouts, and discovery can suggest availability
probes from detected service types and existing discoveries.

### Platform surface depth and consistency

The platform-shaped top level from RC6 remains. The work since RC6 focuses on
making those pages denser, calmer, and more consistent:

- Proxmox backup, recovery, node, and coverage tables gained stronger
  filtering, sorting, row density, identity labels, and backup visibility.
- Docker and Podman rows gained governed lifecycle actions, action-readiness
  reasons, Docker host identity fixes, and better nested context in drawers.
- Kubernetes gained namespace and cluster scope filters, richer drawer detail,
  status visibility, deployment/service/configuration filter coverage, and
  restored healthy-total fractions.
- Machines, storage, and workload rows regained or improved v5-style at-a-glance
  signals while keeping the v6 resource contract underneath.
- Shared table, filter, tab, badge, callout, button, loading, and drawer
  primitives were expanded to reduce visual drift across platform pages.

### Commercial, hosted, and MSP hardening

The self-hosted free-first posture carries through: core monitoring is included
on current public self-hosted plans, and paid value is explicit through Relay,
mobile handoff, support, history, AI operations, automation, Cloud, MSP, and
account surfaces.

RC7 also carries provider MSP control-plane install, preflight, status, proof,
backup, recovery, token-rotation, and isolation work. Cloud and account copy,
commercial migration messaging, billing and support surfaces, entitlement
recovery, and private Pro release proof all need another prerelease pass before
stable promotion.

### Security, release, and install hardening

Notable release-readiness changes since RC6 include:

- restricted outbound HTTP proxy bypass hardening
- local-network connection routing through a subprocess path to avoid Tailscale
  NECP failures
- installer and update resilience fixes
- workflow action pin refreshes
- patched Go toolchain wiring for v6 release builds
- release dry-run and promotion policy guardrail updates
- release asset validation and installer smoke improvements
- audit log, webhook, tenant, token, and bootstrap handling fixes

### Monitoring and correctness fixes

RC7 includes many correctness fixes across alerts, metrics, telemetry, storage,
unified resources, Proxmox, TrueNAS, VMware, Docker, and Kubernetes. Examples
worth retesting include physical disk I/O metrics when SMART data is empty,
Proxmox cluster snapshot polling, Ceph multi-source alert identity, ZFS alert
flapping, metrics database pruning, private CIDR webhook allowlist retention,
guest memory fallback, stale resource sightings, and stable platform source
health filters.

## Validation

This packet is audited against the commit range from the published
`v6.0.0-rc.6` tag through the current candidate head:

- `v6.0.0-rc.6`: `c25e95cb2b071551df95c8add62773905ba0628b`
- candidate head: `5c2e465cde2f6202ef76fcdb6874555a8636a583`
- range: `v6.0.0-rc.6..5c2e465cde2f6202ef76fcdb6874555a8636a583`
- commit count: `934`
- changed scope: `1961` files, `236190` insertions, `46825` deletions

## Retest plan

1. Confirm install and update paths resolve to `v6.0.0-rc.7` only when the
   prerelease channel is explicitly selected.
2. Validate rollback to v5.1.35 with `./scripts/install.sh --version v5.1.35`.
3. Exercise Patrol from alert investigation through finding expansion,
   approval, verification, and resolved-state handling.
4. Exercise Assistant on selected resources, failed providers, queued
   follow-ups, tool progress, and recovery after interruption.
5. Confirm availability checks attach to known Proxmox, Docker, Kubernetes, and
   standalone resources when the target is unambiguous.
6. Re-test discovery suggestion flows for detected services and existing
   discoveries.
7. Walk Proxmox, Docker, Kubernetes, TrueNAS, vSphere, Machines, Alerts, Patrol,
   and Settings in-browser with real or representative data.
8. Re-test release assets, checksums, signatures, installer scripts, Docker
   images, Helm smoke, and preview demo routing before publishing broadly.
9. Re-test provider MSP and Cloud control-plane flows only against the governed
   staging or proof environments.
10. Verify self-hosted commercial posture: no monitored-system cap on current
    public self-hosted plans, no default trial pressure in normal self-hosted
    surfaces, and continuity messaging for existing paid customers.

## Evidence appendix

- `docs/releases/V6_CHANGELOG_RC7_DRAFT.md`
- `docs/releases/V6_RC7_OPERATOR_SUPPORT_PACK_DRAFT.md`
- `docs/release-control/v6/internal/status.json`
- `docs/release-control/v6/internal/subsystems/api-contracts.md`
- `docs/release-control/v6/internal/subsystems/unified-resources.md`
- `docs/release-control/v6/internal/subsystems/patrol-intelligence.md`
- `docs/release-control/v6/internal/subsystems/ai-runtime.md`
- `docs/release-control/v6/internal/subsystems/frontend-primitives.md`
- `frontend-modern/src/features/platformPage/columnAlignment.ts`
- `.github/workflows/create-release.yml`
- `.github/workflows/release-dry-run.yml`
