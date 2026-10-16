# Pending release notes

## Telemetry

When telemetry is enabled, schema v19 reports `successful_history_reads_over_7d_30d`,
an aggregate count of successful
history reads that return metric points older than seven days. This helps
measure actual longer-history use before changing free and paid features.
It sends no resource names, metric values, timestamps, selected ranges or
customer identifiers. Existing opt-out and retention settings are unchanged.
See [Privacy](../PRIVACY.md) for the full field contract.

Include this notice in the first release carrying schema v19. Deployment of
the compatible telemetry receiver must precede interpreting this signal.
