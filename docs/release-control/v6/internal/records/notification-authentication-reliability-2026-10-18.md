# Notification authentication investigation, 18 October 2026

## Signal and attribution limits

The fixed production summary retrieved at 10:01:04 UTC is available with no
missing columns. Its clean seven-day cohort has 8,534 rotating IDs. Terminal
failure coverage is 8,455 IDs, of which 498 report a terminal notification
failure. Rolling totals are 653,287 attempts, 246,419 deliveries and 120,040
terminal failures. Authentication totals 27,959 with classification coverage of
8,113 IDs. Connectivity remains the largest classified sum at 38,088.

These are each rotating ID's latest rolling counters, not distinct customers
or new incidents. Cohorts and retained counters change between reads and may
predate upgrades. The view exposes no destination, transport, provider or error
text, so neither the earlier SMTP fix nor this source slice can be credited
with a measured fraction of fleet authentication failures or fleet recovery.

## Source findings

The baseline is public main
`13c99c17c13659ecce920fd04b24a2617479cca8`, including the prior provider-default
authentication and connectivity repairs.

LOGIN selected credentials by matching literal Username:/Password: text.
Valid legacy prompts such as User Name/Password and empty prompts therefore
failed before correct credentials could reach the server. The historical
[LOGIN draft, section 2.1](https://datatracker.ietf.org/doc/html/draft-murchison-sasl-login-00#section-2.1)
describes the two responses by exchange order and treats challenge content as
opaque. This is legacy compatibility documentation, not a current RFC.

AUTH negotiation used substring matches. A server advertising
PLAIN-CLIENTTOKEN and LOGIN could make Pulse choose unadvertised PLAIN and
reject an otherwise usable LOGIN exchange. SMTP AUTH advertises separate
mechanism names, as defined by [RFC 4954, section 3](https://www.rfc-editor.org/rfc/rfc4954.html#section-3).
An unsupported nonempty list also lacked a declared configuration verdict,
misclassifying the capability mismatch as authentication through error prose.

Connection tests selected implicit TLS for port 465, then could attempt STARTTLS
inside that established TLS session when the explicit TLS flag was false and
STARTTLS remained enabled. Delivery already selected implicit TLS in that case.

## Canonical repair

`internal/notifications/email_enhanced.go` remains the shared authentication
owner for notification email, threaded messages and report attachments.
LOGIN now sends username then password independently of prompt text. It resets
on each Start, refuses premature success and excess challenges, and does not
copy receiver-controlled challenges into errors. Mechanism selection matches
whole names and retains PLAIN preference. Unsupported lists fail as typed
configuration errors before AUTH. Missing/empty advertisement retains the
existing legacy PLAIN probe, without laundering underlying greeting failures.

One implicit-TLS predicate now owns delivery and TestConnection decisions.
Saved settings and the existing TLS trust policy are retained. Actual SMTP
535 replies remain terminal authentication failures and temporary 454 replies
retain retries. Existing queue snapshots, operator retry authority and retained
credentials keep their existing semantics.

## Executable evidence

Owned evidence is retained under
`/opt/pulse-release-worker/task-proofs/notification-authentication-20261018/`.
The new controls exercise the production SMTP client against real loopback
plain, TLS and STARTTLS receivers. AUTH is advertised after the STARTTLS upgrade.
Connection tests and real ordinary, threaded and attachment sends use the same
receiver. Only synthetic credentials and mail are used.

The port-465 control redirects only the configured destination port to an owned
ephemeral receiver through the existing dial seam. It retains the actual TLS
handshake, configured TLS policy and SMTP commands. Negative controls cover
unsupported mechanism names, premature/excess challenges, real bad-password
535 rejection, temporary 454 replies and untrusted certificates.

The queue control admits a normal alert and reopens its queue and audit after
delivery. Correct LOGIN credentials retain one successful attempt, one receiver
acceptance and no failure. A bad password retains one authentication dead letter,
one failed audit and its authentication telemetry count, without accepted mail.
`baseline-authentication.log` records failures on the unchanged runtime:
nonliteral challenges reject valid credentials across all three transports,
substring selection sends an unadvertised mechanism, unsupported capability
is misclassified as authentication, and the valid queued alert becomes an
authentication dead letter. Classic LOGIN controls pass.
`baseline-tls-mode.log` records the independent port-465 failure with classic
LOGIN prompts: TestConnection returns STARTTLS failed, 503 Already using TLS.
The focused authentication controls pass (`focused-authentication.log`, 0.339s),
as does the complete notification package (`packages-authentication.log`,
155.777s). Targeted authentication, delivery, queue and retry race controls pass
(`race-authentication.log`, 2.957s). The source was then fast-forwarded to
`82b2b3a78b2d7c3c9718681062b873b2a0bea4a6`; its intervening changes are confined
to roadmap governance and do not change the runtime exercised by these proofs.
These controls do not establish native provider acceptance or fleet recovery.

## Remaining scope

The existing notification-delivery-terminal-failure-diagnosis gap stays triaged.
Destination-level repeated-failure handling, evidence of destination recovery
and precise delivery-health guidance remain active residual work. Aggregate
authentication counters alone cannot distinguish wrong credentials, provider
policy or a client defect. This slice does not add OAuth, refresh retained
credentials, replay dead letters automatically or publish a release.
