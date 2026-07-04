# Pulse v6.0.2 Release Notes

`v6.0.2` is a stable patch release for the Pulse v6 line. It follows
`v6.0.1` and focuses on agent upgrade recovery, release publication, and demo
verification after the GA cutover.

## Fixes

- Fixed legacy agent update recovery when an installed agent has connection
  details in its running process but no persisted v6 connection state. The
  generated update command can now recover the URL and token instead of
  failing with "No existing Pulse Agent connection state found."
- Restored release asset validation for stable patch hotfixes so a patch can
  publish without inventing a same-version prerelease tag when the explicit
  hotfix path is selected.
- Restored demo verification entitlement handling after release updates so the
  public demo can recover its fixture entitlement and verify mock mode after
  deployment.
- Refreshed Helm chart release metadata and documentation for the stable patch
  line.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.0.2`. The rollback target for
this patch release is `v6.0.1`.
