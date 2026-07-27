# Pulse v6.2.0-rc.1

_This changelog describes the changes since stable `v6.1.2`.
`v6.2.0-rc.1` remains a prerelease and rolls back to stable `v6.1.2`._

## Added

- External Probes (Pulse Pro): availability targets carry an optional probe
  agent assignment, set through the availability check editor's "Run from"
  control and gated by the `external_probe` entitlement at assignment time.
- Assigned targets are delivered to their agent over the signed agent-config
  channel, skipped by the local poller, and resume local execution
  automatically if the entitlement lapses.
- The unified agent gains an availability module that runs each assigned target
  on its own clamped interval through the shared probe core and returns results
  in its regular reports through a bounded delivery queue.
- Probe-reported results are accepted only from the currently assigned agent,
  share the local failure-threshold accounting, carry source attribution to the
  UI, and derive to indeterminate at read time when reports go stale.
- Host-agent identity-collapse detection for cloned `/etc/machine-id`:
  hostname and report-IP revisits within the monitoring flap window publish an
  active conflict and warn on the Machines page.
- Telemetry counts `availability_probe_targets` and
  `availability_probe_agents`, counts only, disclosed in `docs/PRIVACY.md`.

## Changed

- Approved-action telemetry now reconciles successes, refusals, execution
  failures, unverified outcomes, stuck work, in-flight work, and unclassified
  attempts.
- Pre-dispatch refusal counts use stable plan, policy, capability, and other
  categories while remaining content-free.
- Finding-resolution telemetry requires exact Patrol finding and investigation
  linkage plus independently verified action evidence.
- Adoption reporting targets the latest stable release by default and exposes
  outcome-accounting gaps.
- Availability probe execution moved into a shared package so the host agent
  and the monitoring poller run the identical ICMP, TCP, HTTP, and UDP core.
- `postcss` build tooling is updated to `8.5.23`.

## Fixed

- Same-name Proxmox clusters, nodes, and agents at separate sites remain
  isolated when TLS identity evidence contradicts, even when private addresses
  overlap.
- Legitimate duplicate Proxmox views can still consolidate when their captured
  TLS identities agree.
- Weak-evidence cross-instance folds now require positive same-machine proof
  whenever cluster identity is in play, and PVE polling detects cluster
  membership before the cycle's node-state commit, closing the first-poll
  window where an unclassified node could be folded into another site's
  cluster slot.
- Failed unattended updates restart a previously active Pulse service and no
  longer abort on non-writable helper or symlink locations (#1630).
- TrueNAS JSON-RPC websocket connections follow safe same-host HTTPS redirects
  while rejecting cross-host redirects and downgrades (#1631).
- Resolved-loop telemetry cannot be synthesized from unrelated finding and
  action aggregates.

## Release Metadata

- Version: `v6.2.0-rc.1`
- Previous stable: `v6.1.2`
- Rollback target: `v6.1.2`
- Rollback command: `./scripts/install.sh --version v6.1.2`
- Promotion path: exact-SHA single-build release candidate from `main`,
  published as a support prerelease that does not move stable or latest
  install pointers
- Windows signing decision: Authenticode through SignPath is the mandatory
  signing backend and no unsigned-Windows exception applies to any `v6.2.0`
  release; this candidate publishes Windows agents under the standing
  prerelease path with exact-SHA, checksum, and detached-signature
  verification
- Mobile decision: `no-mobile-impact`; no companion build upload or public
  store rollout is part of this candidate
- Entitlement rollout: the license-server entitlement catalog carrying
  `external_probe` was deployed on 2026-07-27, before this cut
