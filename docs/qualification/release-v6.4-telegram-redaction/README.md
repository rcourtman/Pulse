# Telegram diagnostic credential backport — 7 September 2026

Release Train rules 2/4 permit this named security repair on base
`dcf7e499613679c941abe49800f1294b5714fde1`. Backports only code, tests and
notification contract from reviewed Core `192a72e05c06b2a2ff3a04bb2ef53ded78950e47`.
Existing Discord masking and diagnostic context handling are preserved.

Fresh independent primary evidence: https://core.telegram.org/bots/api#making-requests
identifies bot authentication tokens in request paths and supports local API
servers. This is diagnostic containment, not a new product surface.

Tests imported alone failed (before.log, exit 1):
`go test ./internal/notifications -run '^Test(RedactWebhookURLSecrets|TelegramWebhookDiagnosticsRedactPath)$' -count=1`.
Synthetic escaped path credentials survived helper and transport diagnostics;
non-credential bot hostnames/query URLs also lost diagnostic context.

Repair validation (after.log):
`go test -race ./internal/notifications -run '^Test(RedactWebhook|WebhookRateLimitLogsRedactURLSecrets|SlackWebhookDiagnosticsRedactPath|DiscordWebhookDiagnosticsRedactPath|TelegramWebhookDiagnosticsRedactPath)' -count=20`.

No full qualification, installed change, customer exposure assertion, historical
stored-data cleanup or recipient receipt proof. Protected integration and all
eight enforced checks remain required. Existing adverse latency and excluded
crash evidence are not cleared. Delivery owns release maturity and publication.

Result: exit 0, all selected tests passed with race detection (20 repeats).
