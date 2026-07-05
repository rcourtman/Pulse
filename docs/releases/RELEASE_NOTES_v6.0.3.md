# Pulse v6.0.3 Release Notes

`v6.0.3` is a stable patch release for the Pulse v6 line. It follows
`v6.0.2` and focuses on the remaining v5-to-v6 agent upgrade failure reported
after the first two v6 patch releases.

## Fixes

- Fixed v5 agent update recovery when the generated update command supplies an
  explicit Pulse URL but the old running v5 agent process is still the only
  source for the token and enabled telemetry scopes. The installer now merges
  the explicit URL with the recovered process state instead of failing with
  "No existing Pulse Agent connection state found."
- Treated incomplete saved v6 agent state as recoverable during `--update`.
  This keeps the update path on the legacy recovery branch whenever any
  required connection detail is still missing.
- Allowed carrier-grade NAT / overlay network addresses in `100.64.0.0/10` as
  local-network HTTP and WebSocket targets for Pulse agent connections. This
  fixes deployments that legitimately use addresses such as
  `http://100.100.100.5:7655` while keeping public non-local addresses on the
  HTTPS-required path.
- Refreshed Docker, Helm, and installer release metadata for the stable patch
  line.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.0.3`. The rollback target for
this patch release is `v6.0.2`.

If a v5.1.35 agent upgrade failed on `v6.0.0`, `v6.0.1`, or `v6.0.2`, rerun the
current update command from **Settings -> Infrastructure** after updating the
Pulse server to `v6.0.3`.
