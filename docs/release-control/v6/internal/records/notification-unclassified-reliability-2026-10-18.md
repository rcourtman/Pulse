# Notification unclassified-failure investigation, 18 October 2026

## Signal and limits

The fixed aggregate view available during investigation was retrieved at
12:23:42 UTC, with status available and no missing columns. The clean weekly
cohort contains 8,537 rotating IDs. Terminal-failure coverage is 8,457 IDs, with
498 reporting failures. Rolling counters total 642,663 attempts, 248,005
deliveries and 115,318 terminal failures. The unclassified sum is 17,748 with
classification coverage of 8,117 IDs. Connectivity totals 38,299 and
authentication 22,462. Server-error coverage is narrower at 5,428 IDs.

The view is cached and exposes no transport, provider, endpoint or error text.
Rotating IDs are not customers, latest rolling counters are not distinct new
incidents, and retained counts may predate upgrades. Neither these source
reproductions nor changes between aggregate reads establish what proportion
of fleet failures they explain or that the fleet has recovered.

## Reproduced source defects

The unchanged baseline is public main
`98817ebce4e02545ca8870519ae929cee9c724c8`, including the preceding SMTP defaults,
connectivity and LOGIN repairs. Evidence is retained under the owned worker
directory `/opt/pulse-release-worker/task-proofs/notification-unclassified-20261018/`.

`baseline.log` records real loopback HTTP connections closed before headers
and SMTP connections closed before greeting or during AUTH. Plain EOF and
opaque failed network operations classify as unknown, while a close during
AUTH is misclassified as authentication through the sender's stage text.
This can stop a transient SMTP disconnect on the first attempt. Explicit
HTTP/SMTP verdicts and certificate failures must still outrank network evidence.

Go documents [network operation errors](https://pkg.go.dev/net#OpError) as
structured operation/network evidence, and [HTTP response bodies](https://pkg.go.dev/net/http#Response)
as streams that can fail after headers have arrived. The real receivers here
establish the concrete client failures; a bare error message is not evidence
of a particular destination or provider fault.

HTTP Apprise discarded the error from its bounded response-body drain.
All firing, recovery and Test controls accept a truncated HTTP 200 as success.
The exhausted-budget queue control consequently persists sent status, a
successful audit and a delivery receipt despite the interrupted response.
The unchanged complete-response control succeeds legitimately.

Invalid custom payload templates are another deterministic unknown path:
parse, execution and generated-JSON errors enter the ordinary queue's retry
ladder before any request can be sent. `baseline-templates.log` additionally
shows enhanced ntfy using a duplicate renderer with the same untyped verdict,
private template text in errors and rendered invalid payload text in logs.
All test credentials, messages and templates are synthetic.

The initial production Notifications browser pass also showed retained
configuration failures labelled "Failed, retries exhausted". Dead-letter
status does not establish exhaustion when a permanent verdict stops the first
attempt. The shared alerts presentation helper now says "Failed, retries
stopped" and describes retention using that same boundary. Existing outcome
assertions are updated at the helper and activity card, without changing
queue policy or retention.

`baseline-retained.log` records malformed retained email, webhook and Apprise
configuration, plus an unsupported retained notification type, each classified
unknown and rescheduled. These are invalid retained-job controls, not evidence
that Pulse normally writes malformed configuration or a fleet corruption claim.

## Canonical repair and proof boundaries

The shared failure classifier consumes propagated EOF/truncation and failed
network-operation evidence before prose, retaining declared classes, structured
SMTP replies and certificate/TLS precedence. Explicit unconfirmed SMTP
acceptance after DATA retains unknown classification and its existing transport
retry stop. Truly opaque failures stay unknown.
This supplies a diagnosis and preserves transient retry authority; it does not
repair the remote connection itself.

HTTP Apprise delegates its bounded response verdict to the existing webhook
reader, retains authoritative HTTP rejection classes even for interrupted
bodies, and preserves the read cause through safe diagnostics. A 2xx body-read
failure observed within the existing read cap cannot create a successful
receipt or clear recovery receipts. Reaching the cap retains the existing
bounded-reader semantics and does not prove whole-response completion.

Template and retained-configuration failures are declared configuration errors
at their owners. Enhanced ntfy delegates to the canonical renderer and retains
plain-text semantics. Fixed stage diagnostics preserve errors.Is/As without
copying template, payload or configuration contents into errors or logs. Valid
templates, receiver authentication verdicts, original queue snapshots, explicit
operator retry and dismissal semantics remain governed by their existing owners.

The tests exercise real HTTP/SMTP receivers and the actual persistent queue,
reopening its audit, status, receipt and telemetry records. The Apprise
terminal-verdict control deliberately admits one retained job with a
one-attempt budget; it does not simulate a natural retry schedule or establish
provider acceptance. Template admission uses ordinary SendAlert and verifies
that the receiver gets no request. Retained-config controls directly enqueue
invalid retained jobs and exercise the real decoder and processor.

The positive SMTP retry control loses the first connection during AUTH and
forwards the next connection at the same configured endpoint to the real
credential-checking receiver. It requires two connections and exactly one
accepted message. Existing plain/TLS/STARTTLS acceptance controls retain the
unknown verdict and single transport attempt after a missing DATA reply.
A real TLS handshake-refusal control preserves the TLS diagnosis of native
TCP alerts. [Go's pinned TLS implementation](https://github.com/golang/go/blob/go1.26.9/src/crypto/tls/conn.go)
wraps its unexported alert cause in a network-operation error, so generic
network classification must retain that native cause's TLS meaning.

Current-source focused controls pass in 0.293s, the full notifications package
passes in 136.861s, and the targeted notification race controls pass in 9.520s.
Those include the real TLS-alert and SMTP retry controls and unchanged
plain/TLS/STARTTLS missing-acceptance tests. Backend source is unchanged after
that pass.
Browser verification uses the existing production Notifications tab and API
readers with explicitly synthetic response data. It verifies display of the
reproduced verdicts, retained unknowns and recovery controls, separately from
the backend wire and persistence proofs. It is not native provider acceptance.
The affected matrix is the production tab fixture at
`/browser-tests/notification-evidence.html`, at 1440x1000 and 390x1100:
retained configuration/connectivity/opaque failures, safe diagnostics, shared
terminal label and retention description, refresh, cancelled retry
confirmation, focus return and reload. The fixture supplies read responses
only and requires zero API mutations. Pixel review checks wrapping, clipping,
control visibility and readable failure details at both widths.
The final helper/card test run passes 51 tests across three files. The final
browser pass succeeds after the copy change, with zero page errors, zero API
mutations and no horizontal overflow. All four desktop/narrow page and activity
captures were inspected. The frontend receipt binds the final helper content;
the retained worker proof also hashes all three changed backend source files.

## Remaining scope

The existing notification-delivery-terminal-failure-diagnosis gap stays triaged.
Destination-level repeated failures, destination recovery evidence and precise
delivery-health guidance remain active. Opaque CLI/provider failures and
unrecognised verdicts retain unknown classification when available evidence
does not support a more precise class. Historical audit rows and aggregate
counters are not rewritten. This slice does not publish or deploy a release.
