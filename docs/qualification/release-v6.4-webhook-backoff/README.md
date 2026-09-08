# Webhook retry backoff candidate repair

8 September 2026. Base: `9541af16200226c65301cad81126d9c904bcd867`.
Narrow backport of the notification code and test in
https://github.com/rcourtman/Pulse/pull/1976 at
`46b1bff4a0ef1dd53026db5205f6044314d8f075` (Core originating candidate
`4573fe6fcad2c58b30709137ca1691a96bed6736`). No frontend or dependency changes.

The supplied candidate overwrites its exponential schedule with a response's
Retry-After value. Zero then remains zero on subsequent failures, exhausting
transport retries without the intended recovery interval. Selected source
`6329b41c1edbea3fd65aab2425f65758b62a2cf7` also contains this assignment
at internal/notifications/webhook_enhanced.go:333 (source inspection only).

With the upstream regression test added but production code unchanged:

```
go test ./internal/notifications -run '^TestSendWebhookWithRetry_ZeroRetryAfterPreservesLaterBackoff$' -count=1
```

FAIL: subsequent 503 waited 137.442us; subsequent headerless 429 waited
110.62us; both expected >=2s. Local IPv4 synthetic destination only.
The repair keeps the response-specific wait without rewriting the independent
exponential schedule. RFC9110 section10.2.3 describes Retry-After as a wait
before a follow-up request, not a permanent retry policy:
https://www.rfc-editor.org/rfc/rfc9110.html#name-retry-after

Release Train rules 2/4 permit candidate defect backports; this is not a new
surface. This proposal does not replace the immutable selected beta, assert
installed delivery or grant publication approval. Steward review must decide
whether this warrants rejecting that candidate or inclusion in a later release.

After repair:

```
go test -race ./internal/notifications -run 'Test(SendWebhookWithRetry|ParseRetryAfterBackoff)' -count=1
ok github.com/rcourtman/pulse-go-rewrite/internal/notifications 11.285s
```

Focused race-enabled retry/parser coverage passed; git diff --check passed.
No full suite, build, release qualification or installed test was run.
