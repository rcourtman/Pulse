# Pulse v6.3.0-rc.5 Release Notes

`v6.3.0-rc.5` is a release candidate for the next Pulse v6 minor release. It
follows `v6.3.0-rc.4` and stable `v6.2.1`, while keeping stable and latest
users on that stable release.

This candidate carries the complete 6.3 product packet from `rc.4`. Its new
work is focused on release integrity and turnaround: release payloads are
compiled and qualified on the dedicated PVE worker, independent checks run in
parallel, and every public and paid-runtime artifact remains bound to the exact
source commit that passed qualification.

## Highlights

- Release payload compilation now uses the dedicated Ryzen PVE worker and its
  persistent caches instead of leaving that hardware idle.
- Archive, backend, container, Helm, installer, and paid-runtime checks overlap
  where their trust boundaries permit it.
- Container publication promotes the already-qualified exact-candidate payload
  rather than rebuilding release binaries after qualification.

## Improved

- Patrol now works from durable outcomes, scoped investigations, and verified
  work receipts instead of treating the chat stream as operational state.
- Read-only observers extend Patrol coverage between full model investigations
  without granting mutation authority.
- Actions and Patrol identify whether a decision originated from a finding,
  alert, objective, or explicit operator request, making review context clearer.
- Action refusal telemetry now classifies target changes, prerequisites,
  contract failures, capability limits, policy decisions, and stale plans.
- Subscription-backed turns now complete their idle timeout promptly even when
  a canceled CLI descendant still holds an inherited output pipe open.
- Platform pages now lead with estate totals, status facets, and search that
  share the same predicates as their underlying tables.
- Notification settings show the outcome of real delivery attempts instead of
  relying on test sends as a proxy for live delivery health.
- Docker-in-LXC discovery is explicitly controlled and backs off against slow
  or failing Proxmox hosts instead of creating a probe storm.
- Unified Agent installs can opt into a supported least-privilege profile with
  narrowly scoped elevation for the capabilities that require it.
- Platform release archives are validated concurrently, while each archive is
  decompressed only once for its required-entry checks.
- Public and private release payloads are staged during qualification so later
  publication jobs consume immutable evidence instead of repeating builds.
- Helm convergence verifies and publishes the chart artifact already produced
  by the source release run, avoiding duplicate packaging and cluster smoke.
- Paid-runtime convergence separates independent public/download-page checks
  from the lease-bound customer-path proof so both can run at the same time.
- Activation and child-workflow polling use short bounded intervals, reducing
  avoidable idle time without weakening their timeouts or failure reporting.
- The API runtime now has explicit alerting, configuration, agent-token,
  agent-binding, request-context, and HTTP-scope package boundaries. Existing
  routes and extension contracts remain compatible while the test graph gains
  useful package-level parallelism.

## Fixed

- Architecture-specific server signatures are validated against the matching
  platform binary rather than incorrectly requiring every platform signature
  to be identical.
- Windows compatibility aliases in the container payload are validated as
  exact symlinks and then recreated by the image build, keeping the immutable
  manifest limited to regular files.
- Candidate qualification now runs even when intentionally disabled native
  signing jobs are skipped, while still failing closed on a required signing
  failure.
- Backend qualification prefix-compresses the exact API test selector, keeping
  a memory-driven one-shard fallback in one ordered process when safe while
  still byte-bounding deterministic batches below the operating system limit.

## Security

- PVE compilation remains credential-free. GitHub-hosted jobs retain signing,
  release mutation, and publication credentials.
- Container images, Helm charts, public archives, private archives, checksums,
  signatures, and convergence proof remain tied to the release commit and the
  source workflow run.
- The API decomposition preserves tenant scope, install-token binding, command
  channel binding, and notification delivery boundaries with focused contract
  tests at the new package seams.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.3.0-rc.5` only when you are
comfortable testing a release candidate. The rollback target is `v6.2.1`:

```bash
./scripts/install.sh --version v6.2.1
```

The changes since `v6.3.0-rc.4` do not require a Pulse Mobile client change and
preserve the existing mobile, Relay, onboarding, and mobile-facing API
contracts, so no companion mobile build is required.

macOS artifacts follow the normal signing and notarization path. Windows
Unified Agent binaries in this prerelease retain exact-SHA, checksum, and
detached-signature verification but are not Authenticode-signed, so Windows may
display an Unknown Publisher warning. Stable `v6.3.0` still requires the normal
SignPath Authenticode lane.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
