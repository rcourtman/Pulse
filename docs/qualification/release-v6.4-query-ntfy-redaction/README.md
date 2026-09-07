# Encoded-query and resolved-ntfy diagnostic security backport

Release Train rules 2/4: named security defects reproduced on supplied candidate
71c721ccee8bea8a64a130fdbc0ea78292ff748b, not new product scope.
Source: Core 21ed7a89270a5ee1850254d0289c013567871fa8.
Only that correction is backported; mixed main is not imported. The HTTP test
context was absent here, so retain the complete diagnostic-context test from
that source when resolving the conflict. Production HTTP code is unchanged.

Before repair, TestDeliveryEncodedQueryConfidentiality and
TestDeliveryResolvedNtfyConfidentiality fail using synthetic credentials and
in-memory transports. After repair, the focused notification/redaction tests
and TestGetDeliveryLog HTTP tests pass. Logs are retained in
/var/lib/pulse-maintainer/queue/staging/20260907T181017Z-release-line/
(before.log, after.log, api.log; race.log records the separate repeated matrix).

The repair decodes supported query names once and masks repeated occurrences;
resolved ntfy projects transport errors before logging and returning. The
matrix checks unchanged destination, payload, event identity, safe userinfo
rejection and error-cause unwrapping. This is bounded diagnostic protection,
not arbitrary-secret detection, evidence of customer exposure, recipient
receipt, full-suite qualification or release approval.

Fresh external acceptance context: OWASP Logging Cheat Sheet, Data to exclude
and Verification, inspected 2026-09-07:
https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html
Credentials should not be logged; diagnostic failure paths need verification.

Delivery still owns protected-PR integration and exact-candidate qualification.
The retained 940f788d latency failure is not cleared by these focused tests;
Benchmarks remains advisory for source landing. Stable needs exact RC soak
and founder packet approval. No excluded crash investigation was performed.
