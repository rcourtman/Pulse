# Pending release notes

## Changed

When usage statistics are enabled, schema v23 adds independent investigation
run outcomes, fixed categories explaining human handoffs, and explicit
measurement availability and observation age. Completed run measurements
survive finding cleanup; retries count as separate runs, and later action
outcomes update their originating run. No customer content, commands, logs,
identifiers, narrative reasons or evidence are sent.

## Before you upgrade

Measurement starts after upgrading. Earlier findings-based counters remain
separate and cannot provide an investigation success rate. Include this notice
in the first release carrying schema v23. Deploy the compatible receiver and
aggregate reader before interpreting the new fields; source landing alone does
not activate fleet measurement. See [Privacy](../PRIVACY.md).
