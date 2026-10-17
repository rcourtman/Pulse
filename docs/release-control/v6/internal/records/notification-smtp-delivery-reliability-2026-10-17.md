# SMTP notification delivery reliability

## Scope and baseline

The 17 October 2026 production aggregate retrieved at 22:01:16 UTC contains
8,553 clean rotating installation IDs. Of 8,474 IDs with terminal notification
failure coverage, 503 report at least one failure. The rolling seven-day sums
include 121,418 terminal failures, with 37,480 classified as connectivity and
30,847 as authentication. Class coverage differs by sender schema. These are
latest-ping rolling reports, not distinct customers or new incidents since an
earlier read. The aggregate does not identify the notification transport or
provider responsible for those failures.

Baseline source is `8f10dfa9438151db41ea01ccf19e58b669601e96`.
Real loopback SMTP regression tests against that source reproduce two defects:

- Supported provider-default usernames pass configuration validation but do
  not enable authentication in the delivery manager. SendGrid, Postmark,
  SparkPost and Resend default-username controls all receive SMTP 530 because
  Pulse sends MAIL without AUTH, for both saved and standalone managers.
- Acquiring two delivery configurations before sending either overwrites the
  first delivery's sender and recipients. The receiver accepts the second
  envelope for the first message.

## Current boundary

`NewEnhancedEmailManager` resolves provider identity and authentication once
and owns a copy of the email configuration. The notification manager prepares
independent per-delivery configurations while retaining shared rate-limit state
for the cached transport. It does not rewrite admitted queue configuration,
refresh old credentials, replay retained failures or change transport retries.
The notification contract owns this rule; the reporting API contract references
that shared delivery boundary.

`email_delivery_isolation_test.go` exercises real SMTP authentication, accepted
envelopes, overlapping preparation, eight concurrent sends, the shared rate
budget and persistent queue/audit readback after restart. A rejected-credential
control remains one authentication dead letter with no successful receipt.
Existing grouped, direct and recovery tests now inspect the SMTP envelope
instead of requiring mutation of another delivery's cached configuration.

## Verification

Verification uses the exact source copy on `pulse-dev`, selected Go toolchain
`go1.26.9`, through the normal `pulse-dev-validate` admission wrapper.

- Focused SMTP controls passed, including provider-default authentication,
  independent envelopes, concurrent sends, shared rate limits, queue restart,
  and the existing grouped/direct/recovery envelope paths.
- `go test ./internal/notifications -count=1 -timeout=15m` passed in 192.179s.
- The same focused controls with `go test -race` passed in 1.628s.

The focused selection for both ordinary and race execution was:

```text
^(TestEmailDelivery|TestQueuedEmailProviderAuthentication|TestSendGroupedEmail_UsesFromAsRecipient$|TestSendEmail_UsesSharedDeliveryExecutor$|TestSendResolvedNotificationsDirectDispatchesEnabledTargets$|TestSendHTMLEmailWithError_ExistingEmailManager$)
```

Both commands used `./internal/notifications`, `-count=1`, and respective
timeouts of 90s and 180s. The ordinary focused run used `-json`. Retained logs
are `focused.jsonl`, `notifications.log` and `race.log` under
`/opt/pulse-release-worker/task-proofs/notification-failures-20261017/` on
`pulse-dev`. The verification wrapper exited zero.

Runtime source SHA-256 bindings:

```text
e8550b65718c0c6456bc0a0811bf5886c9c1bc8ff5fa3ff1165b62c180e1f0d5  internal/notifications/email_enhanced.go
981c8c445ed885f93c4eafc7ff8772da5b3e06c19af5c74f2c65b14f71a6c705  internal/notifications/notifications.go
```

## Limits and residual work

Synthetic SMTP acceptance proves the sender's protocol and queue verdict. It
does not establish delivery to a real mailbox, native provider compatibility,
release inclusion or the proportion of fleet failures caused by either defect.
The production aggregate cannot attribute those counts to these SMTP paths.

The existing `notification-delivery-terminal-failure-diagnosis` coverage gap
remains open for destination-level repeated-failure handling and recovery
guidance. Invalid destination credentials and external connectivity failures
remain real failures. Historical audits and telemetry are not rewritten, and
these source corrections do not establish fleet recovery.
