# Pulse v6 RC4 Draft Operator Support Pack

_Draft only. Use this as the working support brief for the planned
`v6.0.0-rc.4` candidate until the final prerelease notes are published._

## Support Stance

- Pulse v5.1.29 remains the current stable line.
- Pulse v6 `rc.4` is an opt-in evaluation build, not the default production
  recommendation.
- `rc.4` should be described as a targeted post-`rc.3` hardening RC covering
  stable identity principals, agent-ready API/CLI operations, action audit
  execution, self-hosted licensing continuity, Proxmox and root-agent
  hardening, storage/monitoring corrections, Workloads empty states, and
  Patrol mobile controls.
- Self-hosted SSO is included with Community and higher tiers. Do not describe
  SAML or multi-provider SSO as a Pro-only upgrade path for this RC.
- Stable-channel installer resolution must stay on the latest stable semver
  tag even if GitHub's floating latest-release redirect currently points at an
  RC.
- Systems pinned to the historical `rc.2` update trust root should use a manual
  reinstall or explicit trust migration for later prerelease or GA builds.

## Short Answers

### Is `rc.4` the stable release?

No. The current stable release is v5.1.29. `rc.4` is still a v6 prerelease for
controlled evaluation.

### Should production v5 users upgrade immediately?

No. The recommended RC posture is still staging, lab, or controlled evaluation
first.

### What is the rollback target?

Use v5.1.29:

`./scripts/install.sh --version v5.1.29`

### Can `rc.2` systems auto-update directly to `rc.4`?

Do not promise unattended continuity from `rc.2` to `rc.4`. Hosts pinned to the
historical `rc.2` update trust root need a manual reinstall or explicit trust
migration path for later prerelease or GA builds.

### Does self-hosted v6 still cap monitored systems?

No for the current public self-hosted plans. Community, Relay, and Pro include
core self-hosted monitoring by default.

Current plan shorthand:

- Community:
  core monitoring included, OIDC/SAML SSO with multi-provider support, 7-day
  history
- Relay:
  core monitoring included, secure remote access to the Pulse web UI,
  Pulse Mobile pairing for handoff, push notifications, and 14-day history
- Pro:
  Relay plus AI operations, automation, advanced admin features, and 90-day
  history

### What happens to existing paid Pulse Pro customers in v6?

Use this cohort breakdown:

- Legacy recurring monthly or annual subscribers from v5 or earlier who were
  already active before the public v6 pricing cutover:
  keep the current recurring price, with self-hosted monitoring and
  child-resource volume not metered while the subscription remains continuously
  active under the current v6 policy.
- Existing lifetime customers:
  remain permanently valid, with self-hosted monitoring and child-resource
  volume not metered under the current v6 policy.
- Legacy paid v5 licenses migrated into v6 outside the recurring grandfathered
  path:
  can still exchange into the v6 activation model without repurchasing.
- Former recurring subscribers who already canceled or later lapse:
  any later return uses current public v6 pricing rather than resuming the old
  grandfathered terms.

### What changed from `rc.3` that users should notice immediately?

- hosted signup, checkout, magic-link, SSO, webhook, API token, and
  organization ownership paths now depend on stable principals
- ambiguous email fallback, contact-email takeover, and blank principal paths
  fail closed
- action planning, capability discovery, action audit reads, fleet connection
  reads, and action decisions are available through governed API/CLI surfaces
- action plans persist into the audit trail and dry-run action execution fails
  closed when unsafe
- monitored-system volume caps are removed from the current self-hosted v6
  runtime model
- Proxmox onboarding is API-first, token ACLs are tightened, snapshots are more
  stable through polling gaps, and guest memory fallbacks are corrected
- TrueNAS CORE agent restart handling, mdadm fallback discovery, Ceph pool
  threshold identity, and storage issue impact handling are corrected
- Workloads empty states and Patrol header controls behave better
- mock mode no longer leaves legacy sidecar drift in the primary runtime path

### What if a hosted, checkout, magic-link, or SSO flow still keys access by email?

Escalate it as an identity regression. `rc.4` expects stable user and
organization principals at those trust boundaries.

### What if CLI action planning or dry-run execution fails?

Collect the CLI command, API response, action plan ID if present, audit entry
if present, current Pulse version, and sanitized server logs. Dry-run execution
should fail closed when the request cannot be represented safely, but the
failure should still be inspectable.

### What if a fresh Proxmox LXC stable install lands on a v6 RC?

Treat that as a release-blocking install regression. The stable path should
resolve to v5.1.29 unless the user intentionally chose a v6 prerelease.

### What if a Docker agent duplicates or loses identity after recreation?

Collect logs and escalate. `rc.4` keeps the prior reconnect-token and
host-identity binding work and adds stricter root-agent and Proxmox token ACL
hardening.

### Are public issue comments or closures required for the RC?

No public GitHub state changes are required just to prepare this packet. Draft
comments, closures, or retitles still need explicit maintainer approval before
posting.

## Recommended Evaluation Path

1. Back up the current system and keep direct console access available.
2. Confirm the current stable rollback command:
   `./scripts/install.sh --version v5.1.29`
3. If the host was on `rc.2`, use a manual reinstall or explicit trust
   migration path rather than assuming unattended auto-update continuity.
4. Upgrade the Pulse server in a staging or otherwise controlled environment.
5. Verify server health, version, logs, and update UI before upgrading agents.
6. Re-test hosted signup, checkout, SSO, magic-link, token, webhook, and
   organization ownership flows.
7. Re-test CLI action planning, capability discovery, action audit reads, fleet
   connection reads, and dry-run execution.
8. Re-test Proxmox onboarding, setup-token ACLs, runtime-token ACLs, snapshots,
   guest memory, TrueNAS CORE agent restart, mdadm fallback discovery, Ceph
   thresholds, and storage issue impact presentation.
9. Re-test Workloads empty states, Patrol header controls on mobile, and mock
   mode toggling.
10. Re-test release artifact download, checksum/signature, installer, and
   draft validation paths before broader retesting.
11. Upgrade agents separately only when the user is explicitly testing the
   v5-to-v6 agent path.

## Ask For These Details

When a user reports an `rc.4` problem, ask for:

- current version and prior version
- install type
- whether the host was previously on v5, `rc.1`, `rc.2`, or `rc.3`
- whether a manual reinstall or trust migration was used after `rc.2`
- whether the issue happened during server upgrade, agent upgrade, identity
  handoff, checkout, SSO, action planning, alerting, backup/recovery, platform
  inventory, or first use
- whether Unified Agents were upgraded yet
- expected result
- actual result
- sanitized logs, screenshots, and diagnostics

## Escalate Immediately

Escalate without asking the user to keep experimenting when the report involves:

- failed install or failed upgrade with no recovery path
- stable install path unexpectedly landing on a v6 prerelease
- duplicate or missing agent identity after a v5-to-v6 upgrade
- hosted, checkout, magic-link, SSO, webhook, or token access granted to the
  wrong principal
- action execution that proceeds when dry-run or plan validation failed
- monitoring or reporting that stops entirely after upgrade
- rollback failure or inability to return to v5.1.29
- SSO setup or login blocked by an unexpected paid-license requirement
- data-loss, destructive behavior, or security-sensitive regressions

## Canonical References

- `docs/releases/RELEASE_NOTES_v6_RC4_DRAFT.md`
- `docs/releases/V6_CHANGELOG_RC4_DRAFT.md`
- `docs/UPGRADE_v6.md`
- `docs/AGENT_SECURITY.md`
- `docs/MIGRATION_UNIFIED_NAV.md`
