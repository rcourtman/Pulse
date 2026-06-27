# Pulse v6 RC7 Draft Operator Support Pack

_Draft only. Use this as the working support brief for the planned
`v6.0.0-rc.7` candidate until the final prerelease notes are published._

## Support stance

- Pulse v5.1.35 remains the current stable line.
- Pulse v6 `rc.7` is an opt-in evaluation build, not the default production
  recommendation.
- `rc.7` is the renewed prerelease pass after the branch accumulated a large
  post-RC6 delta. It should be framed as "test this before stable v6", not as
  "this is already GA".
- The stable rollback target is v5.1.35:

  `./scripts/install.sh --version v5.1.35`

- Systems pinned to the historical `rc.2` update trust root should use a manual
  reinstall or explicit trust migration for later prerelease or GA builds.
- Stable-channel installer resolution must remain on the latest stable semver
  tag unless the operator intentionally selects the prerelease channel.

## Short answers

### Is `rc.7` the stable release?

No. It is a v6 prerelease for controlled evaluation. The current stable line is
v5.1.35.

### Why another RC instead of publishing v6.0.0?

The branch contains enough post-RC6 product, runtime, release-pipeline, and
security change that another prerelease pass is the safer release move. RC7
lets operators test the current branch head without treating it as stable v6.

### What changed from `rc.6` that users should notice?

- Patrol is more clearly the checking-loop surface for alerts, findings,
  approvals, and verification.
- Assistant is more contextual, shows live progress, handles provider failures
  better, previews tool output, and recovers failed turns more cleanly.
- Availability checks can attach to the resource they monitor instead of
  always appearing as separate endpoints.
- Discovery can suggest availability probes from detected services and existing
  discoveries.
- Platform tables, drawers, filters, action controls, and empty states are more
  consistent across Proxmox, Docker, Kubernetes, TrueNAS, vSphere, Machines,
  Alerts, and Settings.
- Provider MSP, Cloud, commercial continuity, installer, update, and release
  proof paths were hardened.
- Security and correctness fixes landed across outbound HTTP, webhooks, audit
  logging, tenant boundaries, metrics, alerts, and unified resources.

### Does the frontend keep the RC6 platform-shaped layout?

Yes. Proxmox, Docker, Kubernetes, TrueNAS, vSphere, Machines, Alerts, Patrol,
and Settings remain the top-level shape. The retired `/infrastructure`,
`/workloads`, `/storage`, and `/recovery` aggregate top-level pages do not
return in RC7.

### Does self-hosted v6 still cap monitored systems?

No for current public self-hosted plans. Community, Relay, and Pro include core
monitoring. Paid value remains explicit through Relay convenience,
Pulse Mobile pairing for handoff, push delivery, longer history, AI operations,
automation, support, Cloud, MSP, and commercial account surfaces.

Current shorthand: Community, Relay, and Pro have core monitoring included;
paid value is explicit service, history, support, automation, and hosted
operations value rather than monitoring-volume access.

### What happens to existing paid Pulse Pro customers in v6?

Use this cohort breakdown:

- Legacy recurring monthly or annual subscribers from v5 or earlier who were
  already active before the public v6 pricing cutover: keep the current recurring price, with self-hosted monitoring and
  child-resource volume not metered while the subscription remains continuously
  active under the current v6 policy.
- Existing lifetime customers remain permanently valid, with self-hosted
  monitoring and child-resource volume not metered under the current v6 policy.
- Legacy paid v5 licenses migrated into v6 outside the recurring grandfathered
  path can still exchange into the v6 activation model without repurchasing.
- Former recurring subscribers who already canceled or later lapse use current
  public v6 pricing if they return.

### What if a user sees a v6.0.0 stable release note in the repo?

Treat that as prepared stable-promotion material, not as proof that stable v6
has shipped. The published GitHub release is the authority for what users can
install. RC7 is the active prerelease packet.

### What if availability checks show up as duplicates?

Collect the target URL or address, the intended backing resource, resource IDs,
platform source, and discovery records. RC7 should attach availability checks
to known resources when there is an explicit link or unambiguous address or
hostname match. Duplicate standalone network endpoints are valid only when the
target cannot be safely owned by an existing resource.

### What if Assistant invents discovery results or acts without context?

Escalate it. RC7 should abstain when commands cannot run or context is missing.
Collect the selected resource, Assistant route/provider state, the transcript,
tool events, and relevant `/api/agent/resource-context/{id}` output.

### What if a Patrol investigation does not line up with an alert?

Collect the alert ID, related resource ID, finding ID, Patrol run ID, and the
expanded finding evidence. Alert investigation should route into Patrol with
the selected breach context.

### What if `/api/actions/{id}/execute` returns `plan_drift:` or `resource_remediation_locked:`?

That is expected fail-closed behavior:

- `plan_drift:` means the executed payload no longer matches the approved plan
  hash.
- `resource_remediation_locked:` means the target resource is locked against
  remediation.

The operator should review the plan or lock state before re-approving.

### Are public issue comments or closures required for the RC?

No public GitHub state changes are required just to prepare this packet. Draft
comments, closures, or retitles still need explicit maintainer approval before
posting.

## Recommended evaluation path

1. Back up the current system and keep direct console access available.
2. Confirm rollback works with `./scripts/install.sh --version v5.1.35`.
3. If the host was on `rc.2`, use a manual reinstall or explicit trust
   migration instead of assuming unattended update continuity.
4. Upgrade the Pulse server in a staging or controlled environment.
5. Confirm server health, version, logs, update UI, and release asset checksums
   before upgrading agents.
6. Walk the top-level pages: Proxmox, Docker, Kubernetes, TrueNAS, vSphere,
   Machines, Alerts, Patrol, and Settings.
7. Exercise Patrol from an alert: Investigate, expand finding evidence, approve
   or verify where applicable, and confirm resolved-state handling.
8. Exercise Assistant on selected resources, failed providers, queued
   follow-ups, tool progress, and interruption recovery.
9. Create or confirm availability checks for known resources and verify they
   attach to the intended row.
10. Re-test discovery service-context and availability suggestion flows.
11. Re-test provider MSP and Cloud flows only in governed proof or staging
   environments.
12. Confirm self-hosted commercial posture: no monitored-system cap on current
   public self-hosted plans and no default trial pressure in normal self-hosted
   surfaces.
13. Re-test install, update, Docker, Helm, preview demo, and release asset
   workflows before broader retesting.

## Ask for these details

When a user reports an `rc.7` problem, ask for:

- current version and prior version
- install type
- whether the host was previously on v5, `rc.1`, `rc.2`, `rc.3`, `rc.4`,
  `rc.5`, or `rc.6`
- whether a manual reinstall or trust migration was used after `rc.2`
- whether the issue happened during server upgrade, agent upgrade, release
  asset install, Patrol, Assistant, discovery, availability checks, platform
  inventory, provider MSP, Cloud, billing, SSO, webhooks, alerts, metrics,
  recovery, or first use
- expected result
- actual result
- sanitized logs, screenshots, diagnostics, resource IDs, finding IDs, alert
  IDs, and installer output

## Escalate immediately

Escalate without asking the user to keep experimenting when the report involves:

- failed install or failed upgrade with no recovery path
- stable install path unexpectedly landing on a v6 prerelease
- duplicate or missing agent identity after a v5-to-v6 upgrade
- hosted, checkout, magic-link, SSO, webhook, token, or tenant access granted
  to the wrong principal
- action execution proceeding after dry-run or plan-hash validation failed
- Patrol or Assistant acting on the wrong resource
- availability checks attaching to the wrong resource
- provider MSP tenant data crossing tenant boundaries
- monitoring or reporting stopping entirely after upgrade
- rollback failure or inability to return to v5.1.35
- SSO setup or login blocked by an unexpected paid-license requirement
- data loss, destructive behavior, or security-sensitive regressions

## Canonical references

- `docs/releases/RELEASE_NOTES_v6_RC7_DRAFT.md`
- `docs/releases/V6_CHANGELOG_RC7_DRAFT.md`
- `docs/releases/RELEASE_NOTES_v6_RC6_DRAFT.md`
- `docs/releases/V6_CHANGELOG_RC6_DRAFT.md`
- `docs/UPGRADE_v6.md`
- `docs/AGENT_SUBSTRATE.md`
- `docs/AGENT_SECURITY.md`
- `docs/CLOUD.md`
- `docs/MSP.md`
