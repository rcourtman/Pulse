# Pulse v6.0.0-rc.4 Draft Release Notes

_Draft only. Do not treat this as published until the governed
`v6.0.0-rc.4` tag and GitHub prerelease exist._

`v6.0.0-rc.4` is a targeted hardening RC after the published `rc.3` prerelease.
Pulse v5.1.29 remains the current stable line.

The purpose of this RC is to carry the latest post-`rc.3` release-branch
hardening into a retestable v6 candidate:

- hosted, checkout, magic-link, SSO, Stripe webhook, and organization identity
  paths now rely on stable principals and fail closed instead of falling back
  to ambiguous email-shaped identifiers
- agent-ready operations now have API-first and CLI-first action planning,
  capability discovery, fleet connection reads, action-decision, execution,
  and audit surfaces
- self-hosted v6 licensing continuity keeps monitored-system and child-resource
  volume unmetered under the current public policy rather than writing raw
  monitored-system caps back into runtime state
- Proxmox onboarding, setup-token ACLs, runtime-token ACLs, snapshot polling,
  guest memory fallback handling, TrueNAS CORE agent restart handling, mdadm
  fallback discovery, and Ceph pool threshold identity were tightened
- Workloads empty-state detection, Patrol mobile header controls, mock-mode
  legacy sidecar cleanup, and agent-security guidance were refreshed

This packet was audited against all `51` feature and runtime commits in the
exact `rc.3` to `rc.4` candidate range, from the published `v6.0.0-rc.3` tag commit
`f1744d36d0bde3c8735ae75a190af45c35087841` through candidate commit
`3f16d7845a92d6bf0c5700728bd70e1f4fe32966`. The final prerelease target also
includes RC4 packet and release-validation commits that set the governed
version, pin Docker install defaults to `6.0.0-rc.4`, and align migration tests
with the canonical self-hosted licensing contract. The final validation slice
also makes tenant monitor state broadcasts no-op safely when a headless or test
runtime has no WebSocket hub wired.

## Support Stance

- Pulse v5.1.29 remains the current stable line.
- Pulse v6 `rc.4` is still an opt-in evaluation build, not the default
  production recommendation.
- Existing v5 users should still prefer staging, lab, or otherwise controlled
  evaluation first.
- Hosts already pinned to the historical `rc.2` update trust root should not
  assume unattended auto-update continuity into later prerelease or GA builds.
  Use a manual reinstall or an explicit trust migration path for `rc.4`.
- The stable rollback target for this candidate is `v5.1.29`:
  `./scripts/install.sh --version v5.1.29`

## What Changed Since `rc.3`

### Identity, Auth, And Hosted Trust Boundaries

- Hosted tenant keys, hosted signup owners, hosted handoff identities, and
  workspace-owner proof now use stable user and organization principals.
- Blank, ambiguous, or contact-email-derived principals fail closed in
  magic-link, checkout, and contact-email resolution paths.
- API token minting records owner metadata and binds owner identity across
  token creation.
- Stripe webhook fixtures and organization identity invariants now use stable
  principals, matching the production identity contract.
- SSO runtime paths use stable principals, preserving the Community-tier SSO
  posture from `rc.3`.

### Agent-Ready Operations, CLI, And Auditability

- Pulse now exposes API-first action planning and action-decision paths.
- The CLI can plan actions, discover action capabilities, read action audits,
  and read fleet connection state.
- Action plans persist into the audit trail, AI action audits align with the
  execution lifecycle, and dry-run action execution fails closed when a request
  cannot be safely represented.
- The release-control record now pins the API/CLI-first agent-ready operations
  direction so MCP remains an adapter over the governed API and CLI contracts.

### Self-Hosted Licensing Continuity

- Current public self-hosted v6 plans keep monitored-system and child-resource volume unmetered.
- Legacy continuity paths avoid writing raw monitored-system caps back into
  runtime state.
- Relay wording remains the `rc.2` model: secure remote access to the Pulse web
  UI, Pulse Mobile pairing for handoff, push notifications, and 14-day history.

### Agent, Proxmox, TrueNAS, RAID, Ceph, And Monitoring Correctness

- Root agent service defaults were hardened.
- Proxmox onboarding is API-first, setup-token and runtime-token ACLs are
  tightened, guest snapshots survive transient polling gaps, and guest memory
  fallback handling is more reliable.
- TrueNAS CORE agent supervisor restart handling was corrected.
- mdadm RAID fallback discovery is more robust.
- Ceph pool threshold checks now preserve the resource identity needed for
  correct alert attribution.
- Metrics rollup writes are less noisy after duplicate or repeated rollup
  opportunities.
- Tenant monitor state broadcasts now tolerate runtimes without a WebSocket hub
  instead of panicking during background state refresh.

### Product Surface And Operator Guidance

- Workloads empty-state source detection is corrected.
- Patrol header controls behave better on mobile viewports.
- Mock mode no longer leaves legacy sidecar drift in the primary runtime path.
- The Agent Security documentation entry now points operators at the current
  privilege guidance without leaving a stale support-pack reference.
- Public demo admin reads stay hidden from the demo surface.
- Docker Compose and turnkey Docker installer defaults now pin the RC4 image
  tag instead of the historical RC3 tag.

## What Existing v5 Users Should Re-Test In `rc.4`

1. Server upgrade from the current v5 stable line to v6, including the manual
   or explicit trust-migration path needed for builds after `rc.2`.
2. Fresh Proxmox LXC install and rollback to v5.1.29.
3. Proxmox host onboarding, setup-token handling, runtime-token handling,
   snapshots, and guest memory reporting.
4. TrueNAS CORE agent restart handling, mdadm RAID fallback discovery, Ceph
   pool thresholds, and storage issue impact reporting.
5. Hosted signup, checkout, SSO, magic-link, webhook, and organization-admin
   flows that depend on stable user and organization principals.
6. CLI action planning, capability discovery, action audit reads, fleet
   connection reads, and dry-run action execution.
7. Workloads empty states, Patrol header controls on mobile, and mock-mode
   toggling.
8. Release asset download, checksum/signature, installer, and draft-release
   validation paths before broader retesting.

## Feedback

Use the `Pulse v6 pre-release feedback` issue template for regressions, upgrade
failures, licensing continuity problems, platform-specific breakage, or
actionable UX friction:

- `https://github.com/rcourtman/Pulse/issues/new?template=v6_rc_feedback.yml`

When reporting an `rc.4` problem, include:

- Pulse version
- upgrade path or fresh-install path
- installation type
- whether the host was previously on `rc.1`, `rc.2`, `rc.3`, or v5
- whether a manual reinstall or trust migration was used after `rc.2`
- what you expected
- what happened instead
- sanitized logs, screenshots, or diagnostics when helpful

## Operator References

- `docs/releases/V6_RC4_OPERATOR_SUPPORT_PACK_DRAFT.md`
- `docs/releases/V6_CHANGELOG_RC4_DRAFT.md`
- `docs/UPGRADE_v6.md`
- `docs/AGENT_SECURITY.md`
- `docs/MIGRATION_UNIFIED_NAV.md`
