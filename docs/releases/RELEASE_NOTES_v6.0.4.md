# Pulse v6.0.4 Release Notes

`v6.0.4` is a stable patch release for the Pulse v6 line. It follows
`v6.0.3` and bundles support fixes for audit log compatibility, remembered
login state, agent update recovery, platform metrics, Docker metadata, and
PBS/recovery memory reporting.

## Fixes

- Fixed Audit Log reads when older SQLite rows contain Go wall-clock timestamp
  strings with monotonic suffixes such as `m=+0.009025344`. These rows no
  longer make `/api/audit` return `query_failed`.
- Fixed the login form so the "remember me" choice persists the username
  correctly across reloads.
- Fixed v5-to-v6 agent update recovery for additional legacy and local-network
  URL cases, including local DNS names that intentionally use HTTP.
- Fixed Proxmox, Docker, host-agent, and unified-resource metric normalization
  so CPU percentages and live charts use the same canonical percent contract.
- Fixed PBS/recovery memory retention so backup and recovery rows keep memory
  usage details instead of dropping them during read-state processing.
- Fixed Docker container metadata identity and empty Docker facet handling so
  platform tabs and drawers keep the right container URLs and admission state.
- Fixed OIDC provider detail persistence so saved provider metadata survives
  provider edit flows.
- Refreshed Docker, Helm, and installer release metadata for the stable patch
  line.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.0.4`. The rollback target for
this patch release is `v6.0.3`.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
