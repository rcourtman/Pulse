# Pulse v6.4.3-rc.1

This changelog describes the changes since `v6.4.1`, the latest published
stable release. The `v6.4.2` tag was staged on 2026-08-31 but never activated
as a public release, so this candidate carries the complete `v6.4.2` change
set recorded in `V6_CHANGELOG_v6.4.2.md` plus the corrections that landed on
`main` after that tag.

## Carried from the unpublished v6.4.2 packet

- Infrastructure actions and administrator routes honor the canonical
  administrator boundary for browser, proxy, and SSO sessions, SAML allowlists
  fail closed without an email claim, and security-sensitive setup request
  bodies are bounded.
- Failed backups and completed PBS-to-PBS sync copies no longer pin a guest in
  Backup Running, and incomplete artifacts are excluded from recoverable
  latest-backup pointers (#1815).
- Retained delivery failures can be retried or dismissed from the Alerts
  overview, Assistant command help behaves as a complete dialog, systemd
  services preserve native journal priorities, and durable Proxmox identity
  recovery keeps same-name estates distinct after restart.
- Infrastructure settings name uncovered Proxmox cluster nodes, Agent Doctor
  reports privilege-helper degradation, TrueNAS API-key connections identify
  their owner, and configuration forms expose programmatic labels.

## Fixed

- Two standalone sites that reuse one short node name and one shared install
  token no longer collapse into a single host or Docker record, and removing
  such a record no longer revokes the shared token for every surviving agent
  (#1753).
- Windows Unified Agent auto-update no longer fails with HTTP 404 after
  upgrading the server: the update endpoint serves the signed `.exe` and its
  detached signatures from the canonical release assets, and the server image
  carries the extensionless compatibility aliases for them (#1820).
- Same-name standalone Proxmox connections keep their own node labels and
  agent links after an agent merges into the site, and Proxmox agent links stay
  within their provider scope.
- Kubernetes hosts link through node identity, and ambiguous enrichment that
  could attach a node to the wrong host is rejected.
- Disk I/O collection skips numbered partitions of whole devices whose names
  end in a digit, so MMC, MD, and persistent block devices no longer double
  count partition traffic.
- Health evaluation reports `unknown` with a `telemetry_missing` reason instead
  of green when health telemetry is absent.
- Alert lifecycle replay is bounded by a durable projection watermark, so a
  large event log no longer stalls startup with repeated full replays.
- Uninstalling the Unified Agent removes the safe-profile and privileged-helper
  state it created, and full uninstall preserves shared agent credentials that
  other machines still use.
- Oversized agent reports are classified with an explicit payload-size reason
  instead of an opaque decode failure.

## Changed

- Machines > Availability defaults estates with 20 or more checks to the
  compact fleet view, and the chosen table or fleet presentation stays stable
  across refreshes and shareable through the URL.
- The Alerts overview keeps delivery warnings concise and offers a refresh
  action beside retry and dismiss.
- A slow application bootstrap shows a recoverable status after ten seconds
  instead of a blank page.
- Platform pages announce filter result counts, help and infrastructure dialogs
  are keyboard accessible and named from visible context, resource action menus
  and compact controls meet accessible target sizes, history charts and toasts
  are readable by assistive technology, and the product honors reduced-motion
  preferences.
- Patrol finding options open as a true disclosure and finding handoffs keep
  their context.
- TrueNAS onboarding and troubleshooting guidance matches the JSON-RPC runtime
  connection test.

## Security and release integrity

- Control-plane, agent-capability, discovery-probe, container-stats, remote
  configuration, PDM and updater metadata, AI provider, and Proxmox API
  responses are read with explicit size bounds.
- Typed agent actions run in systemd-contained subprocesses, abandoned typed
  operations are cancelled, runner activation session authority is fenced, and
  helper quarantine and rollback recovery state are hardened.
- Stable installs are continuously verified by a scheduled continuity
  workflow, release continuity failures surface with complete actionable
  detail, and release activation verifies exact asset bytes.
- Security disclosures route to `security@pulserelay.pro`, and the Go crypto
  dependency is updated.

## Release Metadata

- Version: `v6.4.3-rc.1`
- Previous candidate: none. This cut opens the `v6.4.3` candidate line
- Previous stable: `v6.4.1`
- Rollback target: `v6.4.1`
- Rollback command: `sudo /bin/update --version v6.4.1`
- Promotion path: exact-SHA single-build release candidate from `main`
- Unpublished predecessor: `v6.4.2` was tagged at `0c76b5d756` on 2026-08-31;
  its release pipeline run was cancelled after the private Pro build failed
  its compiler memory gate, and no public release, Docker alias, or Helm
  chart was activated for it
- Windows signing decision: prereleases publish checksum- and
  detached-signature-verified Windows agents without Authenticode while
  SignPath remains unavailable. Windows Unified Agent binaries may display an
  Unknown Publisher warning
- Mobile decision: `no-mobile-impact`. No governed mobile route, payload,
  Relay, pairing, approval, push, or onboarding contract changed from
  `v6.4.1`, so no companion mobile build or store rollout is required
