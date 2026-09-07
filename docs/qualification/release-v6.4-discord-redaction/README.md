# Discord diagnostic credential backport

Security backport of Core commit 3c77ecb338ee3d68f1dd7d57a367b4727040a61c,
limited to the notification redactor and its regression tests. Eligible under
Release Train rules 2/4: synthetic tests on release-line base
acb841d3d667545a493033fad48a810db84586b5 exposed Discord webhook path tokens.
Discord documents secure webhook tokens and token-authorised operations:
https://docs.discord.com/developers/resources/webhook (read 7 September 2026).

The change masks the suffix after /webhooks/ on exact Discord hosts, including
legacy/versioned paths and escaped credentials. It modifies diagnostics, not
request destinations, and preserves transport error causes.

Validation: importing tests alone reproduced failures in helper output and
transport diagnostics. With the repair, focused race tests repeated 20 times
passed, covering helper cases, transport errors and actual rate-limit logs.
Logs are retained in the lane packet 20260907T170524Z-release-line.
No full suite, release qualification or installed delivery test was performed.

This does not scrub historical delivery text or establish customer exposure.
Telegram parsing review remains separate. Protected PR checks, steward risk
assessment and exact-candidate qualification remain required; prior adverse
latency and excluded crash evidence are not cleared by these focused tests.
