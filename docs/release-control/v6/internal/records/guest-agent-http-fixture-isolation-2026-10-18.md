# Guest-agent HTTP fixture isolation, 18 October 2026

This is a separately claimed L13 test-stabilization slice discovered while
checking the chart-read changes. It does not change runtime guest admission,
the monitoring lane's completion state or release qualification.

## Failure and correction

The required REST-0 check for chart head
`ea3d71a7990798c026a39480635a43ae1f3ea6c0` failed in
`TestGuestAgentHTTPFailureDefersEveryRead/unknown-client-error/os`.
Its first read returned `agent-cooldown` before sending any guest command,
where the fixture expected one command followed by completion uncertainty.
See [job 117775441898](https://github.com/rcourtman/Pulse/actions/runs/39161106967/job/117775441898).
The complete log is retained with the chart investigation evidence.

Runtime uncertainty fences intentionally outlive individual clients and
closed HTTP fixtures. The HTTP-status tests reused bare loopback endpoint
identities with the same VMID, so a recycled listener address could inherit
another fixture's retained cooldown. Lock-evidence tests already avoided
this through a per-case base-path namespace.

The existing helper is now shared between lock-evidence and HTTP-status
fixtures. All clients and declared cluster aliases within a case use the
same namespace; independent cases have separate identities. Existing
completion, VM-wide cooldown, no-failover, status/counter and recovery
assertions remain unchanged. No runtime guard is reset, expired or weakened
by the isolation mechanism.

## Verification

`TestGuestAgentHTTPFailureFixtureIsolation` gives two independent uncertain
HTTP cases one listener, then checks that both original fences still reject
every diagnostic guest command without another config or guest request.
The earlier entries must remain byte-for-byte equal. Replacing the helper
with the old bare-endpoint behaviour deterministically reproduced the same
pre-wire `agent-cooldown` failure. The expected failure log is retained as
`fixture-negative.log`; it is not a passing test or an idle-worker result.

With the namespaced helper restored, targeted HTTP/lock-evidence tests
passed in 0.779 seconds and the complete Proxmox package passed in 17.437
seconds. All three source hashes matched after the negative control and
the full suite. Raw output and `fixture-source.sha256` remain under
`/opt/pulse-release-worker/task-proofs/chart-latency-slo-20261018/`.
This establishes the fixture reuse mechanism and source correction, not
native guest-agent recovery or the chart's 90 ms performance claim.
