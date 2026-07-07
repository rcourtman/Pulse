# Pulse v6.0.5-rc.3 Release Notes

`v6.0.5-rc.3` is a release candidate for the next Pulse v6 patch line. It
follows stable `v6.0.4` and supersedes `v6.0.5-rc.2` with additional support
fixes for Pulse Mobile onboarding handoff URLs and SSO browser-session display
labels. It retains the earlier support fixes for Patrol Gemini model readiness,
remembered-login submit persistence, Proxmox SMART temperature fallback for
direct SATA/SAT disks, legacy agent update token recovery, threshold-aware
temperature display severity, PBS backup polling memory bounds, physical disk
SMART/Proxmox merge identity, Proxmox token preservation diagnostics, legacy
OIDC SSO discovery with CSP nonce handling, and route-aware Proxmox host URLs.

## Fixes

- Fixed Patrol readiness checks for Gemini models so tool-call capability is
  detected from Gemini candidate parts as well as top-level tool-call lists.
- Fixed the login form so enabling "remember me" during submit persists the
  remembered username immediately.
- Fixed Proxmox SMART temperature fallback for direct SATA/SAT disks where
  smartctl auto-detection returned disk health but no temperature until retried
  with an explicit SAT probe.
- Fixed legacy agent update token recovery for v5-to-v6 update paths that could
  not find stored connection state even though a running agent still had usable
  connection details.
- Fixed temperature display severity so node, workload, Docker, standalone
  agent, and drawer temperature surfaces use configured alert thresholds instead
  of hardcoded warning colors.
- Bounded PBS backup polling memory use and added coverage for read-state
  polling behavior.
- Fixed physical disk SMART/Proxmox merge identity so NVMe and SMART-enriched
  disk records keep their canonical identity and temperature metadata.
- Fixed Proxmox token preservation regressions during install/update flows and
  added smoke diagnostics for token setup and validation.
- Fixed legacy OIDC SSO discovery and CSP nonce handling so configured SSO
  providers are advertised to the login surface after upgrade.
- Fixed Pulse Mobile pairing QR/deep-link payloads so non-HTTPS resolved Pulse
  web URLs are omitted from `instance_url` with a structured diagnostic instead
  of being embedded in mobile handoff links.
- Fixed SSO browser-session display labels so OIDC and SAML sessions keep
  stable provider-scoped principals for authorization while app chrome shows the
  IdP username, email, or display label.
- Preferred route-aware Proxmox host URLs when generating host setup details.
- Refreshed Docker, Helm, installer, and release-helper metadata for the
  release candidate.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.0.5-rc.3` only when you are
comfortable testing an RC. The rollback target for this release candidate is
`v6.0.4`.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
