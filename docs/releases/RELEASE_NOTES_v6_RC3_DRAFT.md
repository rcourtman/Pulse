# Pulse v6.0.0-rc.3 Draft Release Notes

_Draft only. Do not treat this as published until the governed
`v6.0.0-rc.3` tag and GitHub prerelease exist._

`v6.0.0-rc.3` is intended to be a broad hardening RC after `rc.2` and the
first candidate after the `v5.1.29` maintenance sweep. Pulse v5.1.29 remains
the current stable line.

The purpose of this RC is to carry late v5 maintenance fixes, recent RC
feedback, and the post-`rc.2` release-readiness work into the v6 candidate
before broader retesting:

- release artifacts, draft metadata, upload retries, signing, validation, and
  installer resolution should match the current release workflow
- stable installer resolution should stay on the latest stable semver tag even
  when GitHub's floating latest release points at an RC
- auth, token, update, hosted callback, transport, and workflow trust
  boundaries should fail closed where the `rc.2` line was too loose
- v5-to-v6 installs and updates should avoid avoidable installer, disk-space,
  token-display, and rollback surprises
- agent identity and reconnect behavior should remain stable across Docker LXC,
  recreated containers, and Proxmox setup-script paths
- alert, backup, snapshot, and storage views should not regress from the
  current v5 maintenance line
- Workloads, Storage, and Infrastructure should be retestable with the current
  table-first and responsive UI corrections
- self-hosted SSO entitlement, SAML login method handling, and OIDC group
  claim provider settings should match the current Community-tier model
- Pulse Account, hosted signup, MSP, mobile, policy-aware data, resource
  change, action governance, platform admission, and fleet governance proofs
  should remain represented in the candidate
- the support packet should be explicit about the post-`rc.2` update-signer
  transition and the stable rollback target
- backend RC validation should no longer fail the mock metrics history proof
  because an hourly downsample bucket was compared with a single point sample

This packet was audited against all `605` commits in the current candidate
range, from
`2868b44cf91b59bca85cd886711d78cd3c376fab` through
`158d65ccdb81077c35b9237a1652b2774ddb5d5c`.

## Support Stance

- Pulse v5.1.29 remains the current stable line.
- Pulse v6 `rc.3` is still an opt-in evaluation build, not the default
  production recommendation.
- Existing v5 users should still prefer staging, lab, or otherwise controlled
  evaluation first.
- Hosts already pinned to the historical `rc.2` update trust root should not
  assume unattended auto-update continuity into later prerelease or GA builds.
  Use a manual reinstall or an explicit trust migration path for `rc.3`.
- The stable rollback target for this candidate is `v5.1.29`:
  `./scripts/install.sh --version v5.1.29`

## What Changed Since `rc.2`

### Release Packaging And Publish Readiness

- The release workflow now validates signed release sidecars, generated SBOM
  assets, prerelease metadata, and expected artifact completeness before
  treating the RC draft as publishable.
- GitHub draft-release validation preserves draft metadata instead of
  accidentally changing release visibility during validation.
- Release asset uploads use bounded retries so transient upload failures are
  less likely to leave a partially populated RC draft.
- Released images and release builds keep clean VCS metadata for version
  reporting and validation.
- The release packet records the exact `rc.2` to `rc.3` commit coverage audit
  in the release-control evidence appendix.
- Mock metrics store downsample validation now matches the backend query
  contract, so RC backend race proof is not sensitive to the minute inside the
  current hourly bucket.

### Security, Auth, And Trust Boundaries

- Workflow token permissions, CI image pins, Docker base images, signed
  installer downloads, and local release sidecars are restricted for the
  release path.
- Setup tokens, bootstrap state, update tokens, self-update preflight tokens,
  and installer-support paths avoid leaking sensitive values through logs,
  arguments, or overly broad fallback behavior.
- Local loopback, websocket origin, trusted proxy, TLS, outbound HTTP, hosted
  callback, ownership-transfer, invitation, webhook, and license-persistence
  paths were hardened after `rc.2`.
- Skip-auth local/dev login now treats the expected unauthenticated response as
  auth state instead of surfacing it as a client-side request failure.
- SAML login explicitly rejects unsupported HTTP methods, and the SSO provider
  settings model now serializes the OIDC groups-claim field used for allowed
  groups and role mappings.

### Installer And Update Continuity

- The stable install path remains anchored to v5.1.29 instead of accidentally
  resolving to a v6 RC from the fresh Proxmox LXC script path.
- The root installer now filters stable-channel release resolution to stable
  semver tags and downloads the installer from that stable release asset, so
  the stable path does not follow an RC-shaped GitHub latest-release redirect.
- The installer now checks available disk space before stopping the current
  service, so a low-space update should fail before interrupting a working
  install.
- The installer bundle fallback no longer depends on an external helper being
  available at the point where the fallback is needed.
- Bootstrap-token display now uses the supported `pulse bootstrap-token`
  command path instead of surfacing encrypted `.bootstrap_token` JSON.
- The update-progress modal remains dismissible after an update completes or
  fails.
- The release packet records `v5.1.29` as the governed rollback target for
  `rc.3`.

### Agent Identity, Reconnect, And Host Setup

- Docker agents running inside Proxmox LXC keep the canonical host identity and
  reconnect token binding after agent restarts or container recreation.
- The v5-to-v6 server and agent continuity path keeps stable identity files
  and avoids avoidable duplicate-host behavior.
- Proxmox setup scripts preserve existing `authorized_keys` symlinks instead
  of replacing the symlink target with a regular file.
- Agent privilege expectations are documented for RC testing, including why the
  current agent still needs the documented local privileges.
- Patrol local discovery probes are aligned with the agent command policy.

### Alerts, Backup, Snapshot, And Storage Correctness

- Disabled alert notification channels no longer continue sending cooldown
  reminders after the user turns notification delivery off.
- Docker update-alert disable cleanup and alert-threshold metric coloring match
  the configured alert state.
- PBS threshold handling is corrected for the recent v5 maintenance feedback.
- Backup orphan/template readiness and recovery inventory handling now avoid
  stale or missing-state regressions.
- Synology RAID scrub activity is gated on mdstat operation state so normal
  scrubs are less likely to look like unexpected rebuild alerts.
- Ceph monitor count handling uses monitor arrays where available.
- Proxmox snapshots are preserved for guests that are temporarily missed by a
  poll.
- PBS and host-linked ZFS filesystems merge into guest overviews where the host
  relationship is known.

### Monitoring History, AI, And Runtime Behavior

- Duplicate metrics are handled idempotently instead of creating noisy history
  rows.
- Local Ollama calls use a shorter keep-alive window so test systems are less
  likely to hold local model resources longer than needed.
- Backup, recovery, and inventory guardrails were rechecked against the late
  RC3 issue sweep.

### Workloads, Storage, And Infrastructure UI

- Workloads, Storage, and Infrastructure keep the current table-first layout,
  filter deck, saved-view, chart-toggle, and responsive wrapping fixes from the
  v6 release branch.
- On narrow/mobile Workloads views, summary charts stay out of the primary
  table flow so the table remains usable.
- Storage summary tiles keep labels and values inside the shared sticky summary
  grid at narrow widths.
- Summary-chart hover tooltips now offset from the guide line instead of
  covering the exact value under the cursor.

### Commercial, Hosted, Mobile, And Governance Readiness

- The `rc.2` self-hosted plan direction remains intact: current public
  self-hosted plans include core monitoring, Relay adds remote/mobile
  convenience and 14-day history, and Pro adds AI operations, automation,
  advanced admin features, and 90-day history.
- OIDC, SAML, and multi-provider SSO are Community-tier capabilities. The
  `advanced_sso` entitlement key remains as compatibility metadata rather than
  a paid SAML wall.
- Stale self-hosted trial, quickstart, upgrade, monitored-system cap, and
  customer-side commercial analytics paths were retired or hidden from public
  runtime and docs.
- Hosted signup, Pulse Account, workspace/tenant, MSP, mobile approval, and
  mobile companion-role proofs were refreshed.
- Policy-aware data handling, resource-change intelligence, action governance,
  platform admission, and fleet-governance projections are included in the
  candidate rather than left as untracked post-`rc.2` drift.

### Dependencies And Public Feedback Flow

- Frontend sanitization and Go network dependencies are current for this
  release branch.
- Public contribution guidance now steers unsolicited work through issues and
  discussions first while keeping the dedicated v6 RC feedback path available.
- The late open issue and discussion pass did not identify another unhandled
  narrow RC3 code blocker in the current candidate.

### Paid-Plan Wording Stays On The `rc.2` Model

`rc.3` does not reopen the self-hosted paid model settled in `rc.2`. Relay is
still described as secure remote access to the Pulse web UI,
Pulse Mobile pairing for handoff, push notifications, and 14-day history. Pro
remains Relay plus AI operations, automation, advanced admin features, and
90-day history. SSO is included with Community and higher tiers rather than
being positioned as a Pro-only feature.

## What Existing v5 Users Should Re-Test In `rc.3`

1. Server upgrade from the current v5 stable line to v6, including the manual
   or explicit trust-migration path needed for builds after `rc.2`.
2. Fresh Proxmox LXC install and rollback to v5.1.29.
3. Docker Unified Agent inside LXC after restart and container recreation.
4. Alert notification settings, cooldown behavior, PBS thresholds, Docker
   update alerts, and configured alert-threshold colors.
5. Backup, snapshot, recovery, and storage views, including Proxmox snapshots
   and PBS/ZFS filesystem attribution.
6. Workloads, Storage, and Infrastructure table layouts at desktop and mobile
   sizes, including summary-chart hover behavior.
7. Skip-auth local/dev login if that mode is used for testing.
8. OIDC and SAML SSO provider creation, login, allowed groups, and role
   mapping behavior, including Community-tier entitlement behavior.
9. AI/Patrol local discovery and Ollama-backed flows if those features are in
   use.
10. Release asset download, checksum/signature, installer, and draft-release
   validation paths before broader retesting.

## Feedback

Use the `Pulse v6 pre-release feedback` issue template for regressions, upgrade
failures, licensing continuity problems, platform-specific breakage, or
actionable UX friction:

- `https://github.com/rcourtman/Pulse/issues/new?template=v6_rc_feedback.yml`

When reporting an `rc.3` problem, include:

- Pulse version
- upgrade path or fresh-install path
- installation type
- whether the host was previously on `rc.1`, `rc.2`, or v5
- whether a manual reinstall or trust migration was used after `rc.2`
- what you expected
- what happened instead
- sanitized logs, screenshots, or diagnostics when helpful

## Operator References

- `docs/releases/V6_RC3_OPERATOR_SUPPORT_PACK_DRAFT.md`
- `docs/releases/V6_CHANGELOG_RC3_DRAFT.md`
- `docs/UPGRADE_v6.md`
- `docs/AGENT_SECURITY.md`
- `docs/MIGRATION_UNIFIED_NAV.md`
