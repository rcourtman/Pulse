# Pending release notes

## Telemetry

When usage statistics are enabled, schema v20 adds aggregate reporting
telemetry: successful generation by report kind, saved files, SMTP acceptance,
definite and uncertain delivery outcomes, backup-report missing-data states,
distinct active reporting days (including manual downloads),
and successful and repeat reporting periods. Retries do not count as repeat
use. No report contents, recipients, customer identifiers or schedule identifiers
are sent. Existing opt-out, pricing and customer entitlements are unchanged.
See [Privacy](../PRIVACY.md) for every field and its limits.

Include this notice in the first release carrying schema v20. The compatible
receiver must be deployed before interpreting these signals. Source landing
alone does not activate collection. Receiver deployment and a client release
are required before fleet measurement.
