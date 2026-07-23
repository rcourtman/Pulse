# Known RC Issue Closure For GA Proxmox Cluster Address Authority Record

- Date: `2026-07-23`
- Gate: `known-rc-issue-closure-for-ga`
- Issues: `#1437`, `#1493`
- Result: `fixed-main-proof`

## Context

Issue `#1437` supplied a support bundle from a cluster configured through one
reachable management URL. Proxmox discovery also returned member/corosync
addresses that the Pulse container could not route. Snapshot and replication
polls repeatedly waited on those addresses until their task context expired,
while independent Unified Agent data remained visible. The earlier v6
snapshot-read-state correction ensured clustered guests reached the snapshot
poller, but it did not govern which cluster address served the resulting API
requests.

Issue `#1493` reported the presentation side of the same address model after a
cluster network move: Settings retained old member IP reachability errors and
historically repeated the connection-level API badge on every member. The
periodic member refresh and single parent API badge shipped in
`v6.0.0-rc.7`, but the cluster client still treated the saved URL as a final
fallback inside a random endpoint pool.

## Disposition

The operator-saved Proxmox URL is now the ordered cluster API authority. It and
one credential set serve ordinary polling; auto-discovered member addresses
remain ordered failover candidates and direct reachability evidence rather
than becoming per-node credentials or randomly selected peers.

When the configured authority is healthy, member recovery runs as one bounded
background sweep. Snapshot, storage-content, replication, and other API-only
requests therefore do not wait on cluster-private member addresses. If no
endpoint is healthy, recovery remains synchronous so a newly reachable member
can restore service. Recovery is rate-limited and retains member health/error
evidence for Settings and diagnostics.

Periodic cluster discovery still replaces changed member addresses and rebuilds
the failover client. Reachability evidence is carried forward only when the
effective dial URL is unchanged; a re-IP resets the old success/error until the
new target is checked. Settings keeps one API badge on the cluster connection
while member addresses and node-local Agent evidence remain visible.

## Proof

- `go test ./pkg/proxmox -count=1`
- `go test -race ./pkg/proxmox -run 'TestClusterClient_ConfiguredAuthorityDoesNotWaitForMemberRecovery|TestExecuteWithFailoverMovesToAnotherEndpoint|TestClusterClient_RecoverUnhealthyNodes' -count=1`
- `go test ./internal/monitoring -count=1`
- `go test ./internal/monitoring -run 'TestBuildClusterEndpointsForInit_RespectsDiscoveryPolicy|TestDetectClusterMembership_RefreshesStaleClusterEndpoints|TestMergeRefreshedClusterEndpoints|TestBuildClusterEndpoints_PreservesConfiguredAuthority' -count=1`
- `vitest run src/components/Settings/__tests__/InfrastructureSourceManager.test.tsx src/components/Settings/__tests__/useConnectionsLedger.test.ts src/components/Settings/ConnectionEditor/CredentialSlots/__tests__/NodeCredentialSlot.test.tsx`
- subsystem registry, contract, status, control-plane, staged-shape, and
  canonical-completion audits

## Outcome

The stale-address refresh and single cluster API presentation from `#1493`
remain available from `v6.0.0-rc.7` onward, including `v6.1.1`. The configured
authority and non-blocking member-recovery hardening in this record is on
`main` for a future v6 release; it is not retroactively part of `v6.1.1`, no v5
backport is claimed, and no publication date is promised by this proof. Both
issues remain open for reporter confirmation after a release containing the
change.
