# Notification connectivity investigation, 18 October 2026

## Signal and limits

The fixed production summary retrieved at 06:20:56 UTC covers the latest ping
per rotating ID in a seven-day window. The clean cohort has 8,545 rotating IDs.
Notification-failure coverage is 8,466 IDs, of which 495 report a terminal
failure. Rolling totals are 670,938 attempts, 245,365 deliveries and 122,341
terminal failures. Connectivity is the largest classified sum at 37,688,
with classification coverage of 8,122 IDs. Authentication is 30,627 and TLS is
1,722. Missing columns are empty.

The by-release view places connectivity totals mainly in established versions:
19,302 for 6.4.1, 10,741 for 6.5.0 and 3,277 for 6.4.5. These are latest-ping
rolling counters, not distinct customers, new incidents, a regression rate or
post-upgrade recovery. The aggregate exposes neither destination/provider nor
endpoint, error text or connection trace. It cannot attribute a fraction of
fleet failures to either source defect below.

## Reproduced defects and canonical repair

At public source `4b69b8c7ea015362f554a15e7701ab6c50758818`, both the secure
notification client and the shared restricted HTTP client tried approved DNS
addresses serially, allowing ten seconds for each. A stalled first TCP connect
could consume a shorter request deadline while a later approved address was
healthy. Existing refusal tests did not exercise a connect that stalled.

The Linux control fills an owned loopback listener's accept queue on
127.0.0.2. A separate real HTTP receiver on 127.0.0.1 accepts the same port and
passes an independent HTTP control before the queue is filled. Only DNS is
substituted. Both production transports fail with a two-second request timeout
on the baseline, despite the healthy approved second address.

Both clients now use `pkg/securityutil/pinned_dial.go`, with explicit
security/privacy ownership and proof routing. Two staggered workers share a
single ten-second connection budget. Family fallback and later same-family
addresses receive time without hostname re-resolution. Candidate input is
numeric, normalised and deduplicated. A winner cancels other work and closes
late successful connections. The HTTP payload uses only that winning socket.
Notification allowlisting, restricted-fetch policy, TLS and redirect admission
remain with their existing callers.

A separate control uses the production DNS lookup with a controlled resolver
connection that waits for cancellation. The original webhook lookup continues
after the dial context is cancelled. DNS now retains caller cancellation and a
five-second ceiling, shared by URL admission and dialing. Redirect admission
retains the request context and an empty resolution fails closed.

## Verification

Retained worker evidence is under
`/opt/pulse-release-worker/task-proofs/notification-connectivity-20261018/`.
`baseline-connectivity.log` records both original real-network failures.
`baseline-dns.log` records DNS continuing after caller cancellation. The first
focused post-repair proof passes for both HTTP clients and the pinned helper.

The connected queue control admits a synthetic webhook alert, reaches the real
healthy receiver once, stops and reopens the manager, then reads one sent
attempt, one successful audit, one delivery and zero terminal failures.
Additional controls cover family preference, short caller deadlines, later candidates, the two-connect
ceiling, cancellation, late successful socket closure, immediate refusal,
numeric-only input, empty DNS and refused private destinations.

`go test ./pkg/securityutil ./internal/securityutil ./internal/notifications
-count=1 -timeout=8m` passes, retained in `packages-connectivity.log`.
Targeted race checks for the helper, both HTTP clients, queued restart verdict,
DNS cancellation and existing secure transport controls pass, retained in
`race-connectivity.log` and `final-connectivity.log`. Governance checks run
against the staged source with the normal commit hooks.
These controls prove the repaired source behavior. They do not prove
native provider acceptance or fleet recovery.

## Remaining scope

The existing `notification-delivery-terminal-failure-diagnosis` coverage gap
remains triaged. Destination-level repeated-failure handling, evidence of
destination recovery and precise delivery-health guidance remain governed
follow-up. Existing retained failures and credentials are not rewritten or
automatically replayed. No release or deployment is part of this change.
