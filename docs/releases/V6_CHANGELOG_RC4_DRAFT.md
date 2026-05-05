# Pulse v6.0.0-rc.4 Draft Changelog

_Draft only. This changelog describes the current `pulse/v6-release` delta
since the published `v6.0.0-rc.3` tag. Do not treat it as published until the
governed `v6.0.0-rc.4` prerelease exists._

## What `rc.4` changes compared with `rc.3`

`v6.0.0-rc.4` is a targeted release-branch hardening RC. It does not reopen the
`rc.2` commercial model or replace v5.1.29 as the stable line. It carries the
post-`rc.3` identity, API/CLI, action governance, self-hosted licensing
continuity, agent setup, monitoring, storage, and frontend corrections into a
single retestable candidate.

The main release risk addressed here is identity drift: hosted, checkout,
magic-link, SSO, webhook, token, and organization paths should not use
email-shaped fallbacks where stable user and organization principals are
required. The secondary risk is operational drift: agent-ready operations,
Proxmox onboarding, storage attribution, and mobile Patrol controls should
match the current governed v6 architecture before wider RC retesting.

## Commit Coverage Audit

The changelog was audited against every feature/runtime commit in the exact
release range for the current candidate head:

- `v6.0.0-rc.3`: `f1744d36d0bde3c8735ae75a190af45c35087841`
- candidate commit: `3f16d7845a92d6bf0c5700728bd70e1f4fe32966`
- range: `v6.0.0-rc.3..3f16d7845a92d6bf0c5700728bd70e1f4fe32966`
- commit count: `51`
- changed scope: `325` files, `15911` insertions, `11356` deletions

Those commits are grouped in this changelog rather than listed one by one. The
range includes identity hardening, hosted signup and checkout principal
cleanup, API-first action planning, CLI action and fleet reads, action audit
execution proof, self-hosted licensing continuity, root-agent and Proxmox
setup hardening, TrueNAS/RAID/Ceph/storage correctness, Workloads empty-state
handling, Patrol mobile controls, mock-mode cleanup, and release-control
evidence. The final RC4 prerelease target also includes packet and
release-validation commits that pin Docker install defaults to `6.0.0-rc.4` and
remove stale migration-test expectations for retired monitored-system caps, plus
a tenant monitor broadcast guard for runtimes without a WebSocket hub.

## Major Changes

### 1. Hosted and organization identity paths use stable principals

The post-`rc.3` range hardens identity ownership across hosted and local
runtime paths:

- hosted tenant keys and hosted signup owner IDs are canonicalized
- hosted handoff, checkout magic-link, and blank magic-link principal paths
  fail closed instead of deriving authority from weak email fallback state
- contact-email principal takeover and ambiguous email principal resolution
  are blocked
- API token minting records owner metadata and binds tokens to stable owner
  identity
- organization runtime access and workspace-owner proof use stable user IDs
- Stripe webhook fixtures and strict organization identity invariants now
  match the stable-principal model
- SSO paths use stable principals while keeping SSO as a Community-tier
  capability

### 2. Agent-ready operations are API-first and CLI-first

The candidate now carries the governed action surface needed for agent-ready
operations:

- API-first action planning endpoint
- action-decision API
- CLI action planning
- CLI action capability discovery
- CLI action audit reads
- CLI fleet connection reads
- persisted action plans in the audit trail
- action execution safety contract
- AI action audits aligned with the execution lifecycle
- dry-run action execution that fails closed when the requested operation
  cannot be represented safely

The release-control direction is explicit: MCP may remain as a compatibility
adapter, but the stable HTTP API and CLI contracts own the product behavior.

### 3. Self-hosted licensing continuity stays free-first

`rc.4` preserves the public self-hosted v6 policy:

- monitored-system and child-resource volume are not metered in the current
  public self-hosted plans
- continuity paths do not write raw monitored-system caps back into runtime
  state
- Relay remains secure remote access to the Pulse web UI, Pulse Mobile pairing for handoff,
  push notifications, and 14-day history
- Pro remains Relay plus AI operations, automation, advanced admin features,
  and 90-day history

### 4. Agent setup and infrastructure polling are safer

The RC includes additional infrastructure and agent hardening:

- root agent service defaults are stricter
- Proxmox onboarding follows the API-first path
- Proxmox setup-token and runtime-token ACLs are tightened
- Proxmox snapshot polling preserves guest snapshots through transient polling
  gaps
- Proxmox guest memory fallback behavior is corrected
- TrueNAS CORE agent supervisor restart handling is fixed
- mdadm RAID fallback discovery is more robust

### 5. Monitoring, storage, and alert attribution are corrected

The range also includes:

- reduced metrics rollup write amplification
- storage primary issue impact handling
- Ceph pool threshold resource identity preservation
- Workloads empty-state source detection
- mock-mode legacy sidecar cleanup
- tenant monitor state broadcasts no-op safely when no WebSocket hub is wired

### 6. Patrol and docs are aligned with the current RC

Patrol header controls now fit mobile viewports more reliably. The Agent
Security documentation entry points to the current guidance instead of leaving
operators with a stale RC support-pack reference. Public demo admin reads stay
hidden.

## What existing v5 users should re-test in `rc.4`

1. v5.1.29 to v6 server upgrade and rollback to v5.1.29.
2. The explicit post-`rc.2` trust migration or manual reinstall path.
3. Hosted signup, checkout, SSO, magic-link, token, webhook, and organization
   ownership flows.
4. CLI action planning, capability discovery, action audit reads, fleet
   connection reads, and dry-run action execution.
5. Proxmox onboarding, setup-token ACLs, runtime-token ACLs, snapshots, and
   guest memory reporting.
6. TrueNAS CORE, mdadm RAID fallback, Ceph pool thresholds, and storage issue
   impact presentation.
7. Workloads empty states, Patrol header controls on mobile, and mock-mode
   toggling.
8. Release artifact download, checksum/signature, and installer validation
   paths before broad retesting.

## Evidence Appendix

For the code-backed evidence packet that maps these claims to the current
release line, see:

- `docs/release-control/v6/internal/records/documentation-currentness-and-legacy-cleanup-v6-rc4-packet-2026-05-05.md`
- `docs/release-control/v6/internal/IDENTITY_INVARIANTS.md`
- `docs/release-control/v6/internal/records/agent-lifecycle-root-agent-hardening-2026-05-05.md`
- `docs/release-control/v6/internal/records/agent-lifecycle-proxmox-api-first-onboarding-2026-05-05.md`
- `docs/release-control/v6/internal/records/agent-lifecycle-proxmox-setup-permission-proof-2026-05-05.md`
- `docs/release-control/v6/internal/records/agent-lifecycle-proxmox-runtime-token-permission-proof-2026-05-05.md`
- `docs/release-control/v6/internal/records/known-rc-issue-closure-for-ga-metrics-write-amplification-2026-05-03.md`
