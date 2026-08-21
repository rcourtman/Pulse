# Pulse v6.3.0-rc.6 Release Notes

`v6.3.0-rc.6` is a release candidate for the next Pulse v6 minor release. It
follows `v6.3.0-rc.5` and stable `v6.2.1`, while keeping stable and latest users
on that stable release.

This candidate carries the complete 6.3 product packet and concentrates on
release turnaround. The release graph now makes fuller use of the dedicated
Ryzen PVE worker, moves more API production domains into independently
schedulable packages, and overlaps public and private publication work without
moving any customer-facing pointer before exact-SHA readiness.

## Highlights

- Chart and resource-query services now qualify independently from the residual
  API router, shrinking the root test critical path.
- Backend admission waits for sibling compilers, then requires measured
  headroom for two race-enabled API shards.
- Public server and provider control-plane images publish and attest in
  parallel from one verified exact-candidate payload.

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
- Exact-version Docker staging begins as soon as the immutable candidate
  exists. Candidate container qualification and every other immutable gate
  still join before release activation.
- Private release artifacts use a purpose-built Pro packaging profile and
  compressed transfer rather than shipping unused frontend, MCP, public-server,
  and control-plane products through the credential boundary.
- Paid-runtime convergence uses the runner's installed Chrome with the pinned
  Playwright client and runs Docker and direct-binary mismatch proofs in
  parallel on separate ports.

## Fixed

- Release activation recovery now joins the canonical readiness result instead
  of maintaining a stale duplicate catalog of historical job names.
- Helm convergence binds repository operations explicitly when the workflow is
  executing from its nested Pages checkout.
- Every hosted paid-runtime proof establishes its own tailnet connection before
  calling the private license service.
- The backend planner reports a capacity failure if the dedicated worker cannot
  admit the required two-shard release shape after its bounded wait.

## Security

- PVE compilation remains credential-free. GitHub-hosted jobs retain signing,
  release mutation, and publication credentials.
- Container images, Helm charts, public archives, private archives, checksums,
  signatures, and convergence proof remain tied to the release commit and the
  source workflow run.
- Each parallel Docker matrix leg independently verifies the exact checkout,
  anticipated release line, and immutable candidate manifest before publishing.
- The chart and resource boundaries preserve tenant scope, router authorization,
  action ownership, and operator-state mutation at explicit package seams.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.3.0-rc.6` only when you are
comfortable testing a release candidate. The rollback target is `v6.2.1`:

```bash
./scripts/install.sh --version v6.2.1
```

The changes since `v6.3.0-rc.5` do not require a Pulse Mobile client change and
preserve the existing mobile, Relay, onboarding, and mobile-facing API
contracts, so no companion mobile build is required.

macOS artifacts follow the normal signing and notarization path. Windows
Unified Agent binaries in this prerelease retain exact-SHA, checksum, and
detached-signature verification but are not Authenticode-signed, so Windows may
display an Unknown Publisher warning. Stable `v6.3.0` still requires the normal
SignPath Authenticode lane.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
