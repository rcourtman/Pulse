# Pulse v6.4.5-rc.3

This changelog describes the changes since `v6.4.5-rc.2`. This candidate carries the complete `v6.4.2` change set, which was tagged but never activated as a public release, along with all previously documented v6.4.5 reliability repairs. Since the prior published RC, this cut adds KnowledgeStore save serialization, PBS History target retention, provider client network isolation and upgrade recovery, hosted agent token scoping, organization-owner settings parity, the stable-promotion resolver repair and a Profile L test timeout correction; stable remains `v6.4.1` until exact qualification, publication and a clean 72-hour soak support promotion.

## Reliability corrections since v6.4.1

- **Provider-hosted clients recover through upgrades** - Client workspace networks and health are reconciled during provider MSP upgrades (#2247), while network ownership stays scoped to the installation (#2226).
- **Organization and agent setup boundaries hold** - Organization owners reach settings their own routes serve (#2208), and hosted agent install tokens are minted inside the selected client workspace (#2209).
- **AI knowledge saves serialize** - Consecutive KnowledgeStore saves no longer race an atomic file rename or cleanup (#2186).
- **PBS History remains on its host** - IP- or alias-connected PBS History stays attached to its host target (#2196, #2201).

- **Due notifications survive startup** - A persisted notification already due
  at startup is no longer processed before saved webhook, email and Apprise
  destinations are applied, so a grouped recovery is no longer terminally
  cancelled as "delivery disabled" (#2160).
- **Escalations use current policy** - Escalation callbacks re-check the current
  rule, occurrence, acknowledgement and snooze state before admitting a delayed
  escalation, and an automatic acknowledgement is retained across a re-fire
  instead of being dropped (#2173).
- **The resource drawer keeps its tab** - Selecting a drawer tab no longer snaps
  back to Overview when a live data refresh briefly omits a field that gates that
  tab, such as a merged metrics target on the Proxmox Backups History view
  (#1723).
- **Backup age alerts stay truthful** - A PBS host or config backup no longer
  raises a repeated guest backup-age alert, while a guest backed up locally and
  copied to a PBS is still counted once (#1741, #1721, #2136).
- **Alert history reads stay bounded** - Repeated attention polls reuse the
  folded alert history instead of re-walking the whole event log, so a large
  history no longer starves metric writes or freezes an LXC (#2146).
- **Alert thresholds lists stay complete** - The Alert Thresholds instance list
  keeps every host row visible while scrolling instead of dropping rows as the
  window estimate drifts (#2130).
- **Alerts overview counts and layout align** - Overview statistics match the
  underlying incidents, and the alert-card footer Started run shares the
  baseline of the delivery-status run (#2119).
- **Alert checkpoint writes are bounded** - Byte-identical pending-intent
  checkpoints and oversized alert event-log snapshots are no longer rewritten
  on every cycle, reducing write volume on flash storage (#1966).
- **Resolved alert bursts respect grouping** - Concurrent recoveries use the
  configured destination and grouping window across restarts instead of
  creating one delivery per alert and overwhelming the queue (#2160).
- **Critical escalations are delivered** - When an active warning becomes
  critical, the higher severity can bypass only the same-occurrence cooldown and
  records its destination delivery in notification history (#1801).
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
- **Saved Patrol objectives recover** - A saved Patrol objective whose
  in-memory observer was lost or rejected is reconciled from retained intent
  and rechecked at run admission instead of staying uncovered (#2147).
- **Proxmox LXC memory uses the linked agent** - A correlated online Pulse
  agent inside a Proxmox LXC supplies the container memory reading instead of
  the cache-inclusive cluster value when the agent total matches the
  provisioned limit (#2148).
- **Ollama and security settings persist** - Ollama Basic Auth configuration,
  the configured-admin authorizer and security-setup preferences survive save
  and restart.
- **Verify SSL choices persist** - A node's "Verify SSL Certificate" choice is
  recorded explicitly and preserved through agent re-registration and PVE
  instance consolidation, and an explicit PBS choice is no longer silently
  overwritten (#2140).
- **Community-scripts update guidance is corrected** - The upgrade
  documentation warns that `/bin/update` is the community-scripts updater on
  some Proxmox helper-script containers and points those users at the signed,
  version-pinned installer flow (#2129).
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
- Release-line release candidates are qualified against their own governed
  branch, so the secure-runtime qualification no longer requires an
  `origin/main` ancestor.
- Security disclosures route to `security@pulserelay.pro`.

## Changes since published v6.4.5-rc.2

- KnowledgeStore saves are serialized so concurrent AI knowledge writes do not overwrite one another (#2186).
- PBS Backups History remains on its host target across refreshes and when a connection uses a hostname, IP address or DNS alias (#1723, #2196, #2201).
- Exact RC-to-stable promotion accepts the governed installer pin update and Go `_test.go` changes required by the stable cut (#2184, #2205).
- The Profile L row-windowing test receives its declared wait budget under qualification load; product behavior is unchanged.

## Release Metadata

- Version: `v6.4.5-rc.3`
- Previous candidate: `v6.4.5-rc.2`, published 2026-09-23. This cut adds KnowledgeStore serialization, PBS History target retention, provider-hosted client isolation and upgrade recovery, organization-owner settings and hosted agent token scoping, the promotion-resolver repair and a qualification test timeout correction to that published preview.
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
