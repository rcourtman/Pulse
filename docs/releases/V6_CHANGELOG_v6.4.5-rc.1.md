# Pulse v6.4.5-rc.1

This changelog describes the changes since `v6.4.1`, the latest published
stable release. The `v6.4.2` tag was staged but never activated as a public
release, so this candidate carries the complete `v6.4.2` change set recorded
in `V6_CHANGELOG_v6.4.2.md` plus the corrections that landed on the
`release/v6.4` line after that tag and the preview corrections exercised in
`v6.4.5-beta.1`.

## Reliability corrections since v6.4.1

- **Backup age alerts stay truthful** - A guest backed up locally on Proxmox
  and copied to a PBS is counted once, and a host that stops reporting no
  longer raises repeated critical backup-age alerts (#1741, #1721, #2136).
- **Alert thresholds lists stay complete** - The Alert Thresholds instance list
  keeps every host row visible while scrolling instead of dropping rows as the
  window estimate drifts (#2130).
- **Alerts overview counts and layout align** - Overview statistics and text
  alignment match the underlying incidents after retries and refreshes
  (#2119).
- **Alert checkpoint writes are bounded** - Byte-identical pending-intent
  checkpoints and oversized alert event-log snapshots are no longer rewritten
  on every cycle, reducing write volume on flash storage (#1966).
- **Guest panels order predictably** - The workload drawer lists Filesystems
  before Tags, matching the rest of the interface (#2121).
- **TrueNAS telemetry is accurate** - AMD temperature aggregates are rejected
  instead of inflated, CORE memory is read through the legacy REST path, and
  idle sessions follow a bounded poll cadence (#2122, #2077, #1893).
- **vSphere enrichment recovers** - VMware requests send the expected JSON type
  names, probe the current 8.0.3 release shape, and surface API fault detail
  instead of failing with HTTP 500 (#2070).
- **Container update checks match the running image** - All local RepoDigests
  are treated as the current image, so PostgreSQL and other containers stop
  reporting a false update (#2110).
- **Failed updates leave no stale state** - A failed staging attempt discards
  its configuration backup, and a completed update clears its trap so success
  is not reported as failure (#2127, #2128).
- **FreeBSD and pfSense installs verify checksums** - The installer verifies
  downloaded agent checksums without relying on GNU coreutils (#2123).
- **Custom sensor warnings are delivered** - Health-assessment escalations for
  custom sensors now send their warning notifications (#1801).
- **Derived agent identity heals safely** - Re-enrolling a forked identity
  honours the removal block, and the event-log volume stays bounded through
  the heal (#2113, #1586).
- **Host continuity survives upgrades** - v5 to v6 upgrade paths preserve live
  host state instead of losing continuity (#1913).
- **Startup stalls are recoverable** - A startup watchdog records a stalled
  bootstrap and the delayed browser start offers a truthful retry path
  (#2129).
- **AI Patrol prompts are cached** - The stable system prompt prefix is cached
  so repeated Patrol runs cost less (#2118).
- **Ollama and security settings persist** - Ollama Basic Auth configuration,
  the configured-admin authorizer and security-setup preferences survive save
  and restart.
- **Verify SSL choices persist** - A node's "Verify SSL Certificate" choice is
  recorded explicitly and preserved through agent re-registration and PVE
  instance consolidation, so a deliberately disabled or enabled setting is no
  longer silently overwritten (#2140).
- **Metric samples stay consistent** - Replayed unified metric samples are
  dropped, and metrics store shutdown, rollups and upgrades are hardened.

## Carried from the unpublished v6.4.2 packet

- Infrastructure actions and administrator routes honor the canonical
  administrator boundary for browser, proxy and SSO sessions, SAML allowlists
  fail closed without an email claim, and security-sensitive setup request
  bodies are bounded.
- Failed backups and completed PBS-to-PBS sync copies no longer pin a guest in
  Backup Running, and incomplete artifacts are excluded from recoverable
  latest-backup pointers (#1815).
- Two standalone sites that reuse one short node name and one shared install
  token no longer collapse into a single host or Docker record, and removing
  such a record no longer revokes the shared token for every surviving agent
  (#1753).
- Windows Unified Agent auto-update no longer fails with HTTP 404 after
  upgrading the server: the update endpoint serves the signed `.exe` and its
  detached signatures from the canonical release assets (#1820).
- Same-name standalone Proxmox connections keep their own node labels and
  agent links after an agent merges into the site.
- Disk I/O collection skips numbered partitions of whole devices whose names
  end in a digit, so MMC, MD and persistent block devices no longer double
  count partition traffic.
- Health evaluation reports `unknown` with a `telemetry_missing` reason
  instead of green when health telemetry is absent.

## Changed

- Machines > Availability defaults estates with 20 or more checks to the
  compact fleet view, and the chosen table or fleet presentation stays stable
  across refreshes and shareable through the URL.
- A slow application bootstrap shows a recoverable status after ten seconds
  instead of a blank page.
- Platform pages announce filter result counts, help and infrastructure
  dialogs are keyboard accessible, and the product honors reduced-motion
  preferences.
- Container image checks fall back to a validated manifest response when a
  registry HEAD request lacks a digest (#2048).

## Security and release integrity

- Control-plane, agent-capability, discovery-probe, container-stats, remote
  configuration, updater metadata, AI provider and Proxmox API responses are
  read with explicit size bounds.
- Typed agent actions run in systemd-contained subprocesses, abandoned typed
  operations are cancelled, and helper quarantine and rollback recovery state
  are hardened.
- Release activation verifies exact asset bytes, and the private Pro source
  pair is verified before a draft release object is created.
- Security disclosures route to `security@pulserelay.pro`.

## Release Metadata

- Version: `v6.4.5-rc.1`
- Previous candidate: `v6.4.5-beta.3`, published 2026-09-20. This cut moves the
  `v6.4.5` line from beta to release candidate
- Previous stable: `v6.4.1`
- Rollback target: `v6.4.1`
- Rollback command: `sudo /bin/update --version v6.4.1`
- Promotion path: exact-SHA single-build release candidate from `release/v6.4`
- Unpublished predecessor: `v6.4.2` was tagged but never activated as a
  public release, so this candidate carries its complete change set
- Windows signing decision: prereleases publish checksum- and
  detached-signature-verified Windows agents without Authenticode while
  SignPath remains unavailable. Windows Unified Agent binaries may display an
  Unknown Publisher warning
- Mobile decision: `no-mobile-impact`. No governed mobile route, payload,
  Relay, pairing, approval, push or onboarding contract changed from `v6.4.1`,
  so no companion mobile build or store rollout is required
