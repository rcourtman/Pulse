# Pulse v6.1.0 Release Notes

`v6.1.0` is a stable minor release for the Pulse v6 line. It follows stable
`v6.0.5` and promotes the monitor-first product work, reviewed action lifecycle,
Operational Trust workflow, Unified Agent improvements, and release-candidate
feedback fixes validated across the `v6.1.0-rc.1` through `v6.1.0-rc.4` line,
plus the final compatibility and reliability fixes in the stable cutoff.

## Highlights

- Patrol brings protection gaps, availability failures, and actionable
  findings into one attention workbench with a reviewed path from evidence and
  policy through approval, execution, receipt, and independent verification.
- Actions provides a dedicated inbox for governed Docker, Proxmox, host-update,
  package-maintenance, and storage-cleanup work instead of hiding approvals in
  Assistant history.
- Unified Agents can report inventory to explicitly configured report-only
  observer destinations, while Agent Doctor now has a routed, filterable
  diagnostic workflow with copyable reports and platform-correct local cleanup
  guidance.
- Infrastructure identity, disk health, metrics concurrency, audit retention,
  native updates, authentication, and installer recovery are more accurate and
  fail closed across the supported self-hosted paths.
- The stable server contract is compatible with the existing Pulse Mobile
  candidate and now gives relay-mobile tokens the Patrol attention access the
  companion needs.

## Added

- Pulse Intelligence has explicit detection and investigation profiles, typed
  proposal and action state, durable execution receipts, and post-action
  verification for supported operations.
- Patrol can route findings to alert notification channels, qualify model-led
  investigations with bounded evidence and turn budgets, and preserve multiple
  grounded findings from one run.
- Claude subscription-backed models support bounded preflight, schema-bound
  streaming, native typed tools, and retry-safe outcomes without creating a
  parallel action-execution path.
- Unified Agents support report-only observer destinations with per-destination
  identity, token, retry state, health, and plaintext-HTTP policy.
- `--report-ip` can set the Unified Agent's reported host address and becomes
  the leading host-identity signal.
- Disk monitoring understands SAS transport and SCSI SMART attributes.
- Windows Unified Agent artifacts remain bound to the exact release SHA and
  retain checksum, detached-signature, manifest, and published-digest
  verification.

## Improved

- Platform and connected-system pages lead with monitor-first attention and
  task-oriented workflows, with more coherent responsive layouts.
- The Assistant supports retry, regenerate, edit-and-resend, in-flight steering,
  long-input handling, and clearer model-cost summaries where available.
- Docker and Proxmox operations remain bound to reviewed typed plans and can
  recover durable outcomes after server or agent reconnection.
- Physical interface addresses lead virtual bridges in host identity, ESXi
  hosts group under their owning vCenter, and workload metadata follows the
  canonical event path.
- Agent Doctor retains the last-known platform for removed agents so it can
  provide the correct host-local uninstall command without implying remote
  removal authority.
- Post-update What's New highlights open in a readable dialog while preserving
  once-per-release dismissal behavior.
- Metrics reads can proceed concurrently with writes, and audit retention uses
  incremental vacuum and bounded reclamation.

## Fixed

- Integration-monitored machines no longer appear as fabricated Unified Agent
  rows or misleading Agent Doctor targets.
- Agent install tokens carry the requested command-execution scope and bind the
  command channel correctly on first registration.
- Manual agent update commands remain available until an update is actually
  applying, and replacement binaries are self-tested before the swap.
- TrueNAS storage reads the served API shapes, ZFS membership resolves
  `nvme-eui` and namespace-suffixed references, and Proxmox disk health uses one
  canonical vocabulary.
- Patrol attention routes accept the relay-mobile scope used by the existing
  companion build, and the checked-in core/mobile contract guards route,
  method, scope, request, response, pairing, and push compatibility.
- Patrol assessment lookup recovers active findings from mangled identifiers,
  and action origins retain the proposal evidence that led to review.
- In-app updates select the highest valid version, wait for the new version to
  serve before reloading, and reject malformed release notes.
- The DOM sanitization dependency includes the fix for
  `GHSA-c2j3-45gr-mqc4`.
- ICMP permission failures explain a missing `CAP_NET_RAW` grant, and Docker,
  storage, OIDC, session, and native-agent recovery paths retain the additional
  fail-closed fixes proven during the release-candidate line.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.1.0`. Existing Unified Agent
configurations keep one authoritative Pulse server; observer reporting remains
opt-in and no observer is created during upgrade.

Windows Unified Agent binaries in `v6.1.0` are not Authenticode-signed and may
show an Unknown Publisher warning. Verify the published checksums and detached
`.sig` or `.sshsig` signatures before installation. This is a one-release owner
exception; later stable releases restore the Windows Authenticode requirement.

The rollback target is `v6.0.5`. The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.0.5
```

Pulse Mobile `1.0.0` iOS build `11` and Android versionCode `9` remain the
compatible candidate builds. This server release does not upload a companion
build and does not start a public mobile-store rollout.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
Unproved self-service commercial plan or billing-cadence transitions remain
disabled and are not introduced by this release.
