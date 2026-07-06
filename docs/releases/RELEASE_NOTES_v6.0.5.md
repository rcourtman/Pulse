# Pulse v6.0.5 Release Notes

`v6.0.5` is a stable patch release for the Pulse v6 line. It follows
`v6.0.4` and bundles support fixes for Patrol Gemini model readiness and
remembered-login submit persistence, plus a Proxmox SMART temperature fallback
for direct SATA/SAT disks.

## Fixes

- Fixed Patrol readiness checks for Gemini models so tool-call capability is
  detected from Gemini candidate parts as well as top-level tool-call lists.
- Fixed the login form so enabling "remember me" during submit persists the
  remembered username immediately.
- Fixed Proxmox SMART temperature collection for direct SATA/SAT disks where
  smartctl auto-detection returned disk health but no temperature until retried
  with an explicit SAT probe.
- Refreshed Docker, Helm, installer, and release-helper metadata for the stable
  patch line.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.0.5`. The rollback target for
this patch release is `v6.0.4`.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
