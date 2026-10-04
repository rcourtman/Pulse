# Pulse v6.4.5-rc.5 Release Notes

This preview replaces published `v6.4.5-rc.4` for an alert-delivery regression. A grouped child symptom could notify repeatedly, or remain unnotified after its primary recovered. The reviewed repair fixes that lifecycle. All other product changes are inherited from RC.4. Stable promotion requires exact qualification and a new clean 24-hour soak after this candidate publishes. Stable `v6.4.1` remains the rollback target.

## What's improved

- **Continuing symptoms are notified once** - Grouped child alerts no longer escape through duplicate escalation. A child that outlives its primary receives its first notification after the primary recovers (#2336).

- **Same-name systems stay separate** - Stronger host, Docker, Kubernetes and Proxmox identity checks keep alerts, metrics, actions and removal tied to the right machine even when short names or shared tokens repeat (#1753, #1930).
- **Windows agent delivery is restored** - Unified Agent downloads resolve canonical signed `.exe` assets and their signature sidecars, so installation and auto-update no longer fail with HTTP 404 (#1820, #2125).
- **Large Availability estates scan faster** - Estates with 20 or more checks open in fleet view, and the chosen table or fleet presentation stays stable across refreshes and shareable URLs.
- **Slow starts are recoverable** - A delayed or stalled startup now shows connection status and a retry, and large alert histories catch up incrementally instead of blocking the server (#2129).
- **Disk I/O totals are more accurate** - Partition accounting no longer inflates whole-device traffic, and endurance counters stop raising false disk-wear warnings (#2112).
- **Duplicate disk rows are reduced** - Linked PVE and agent observations can safely correlate the same USB disk when one source lacks a stable ID, while conflicting and ambiguous devices remain separate (#2076).
- **Backup alerts stay truthful** - A PBS host or config backup no longer raises a repeated guest backup-age alert, while a guest backed up locally and copied to a PBS is still counted once (#1741, #1721, #2136).
- **PBS History keeps its identity** - Linked rows correlate only with proven identity, and datastores stay distinct across refreshes (#1723). Some API-plus-Agent PBS History layouts remain unresolved.
- **PBS capacity follows policy** - Datastore capacity health uses the configured warning and critical thresholds instead of fixed percentages (#1448).
- **Alert history reads stay bounded** - Repeated attention polls reuse the folded alert history instead of re-walking the whole event log, so a large history no longer starves metric writes or freezes an LXC (#2146).
- **Alert thresholds lists stay complete** - The Alert Thresholds instance list keeps every host row visible while scrolling instead of dropping rows as the window estimate drifts (#2130).
- **Alerts overview counts and layout align** - Overview statistics match the underlying incidents, and the alert-card footer Started run now shares the baseline of the delivery-status run (#2119).
- **Alert checkpoint writes are bounded** - Byte-identical pending-intent checkpoints and oversized alert event-log snapshots are no longer rewritten on every cycle, reducing write volume on flash storage (#1966).
- **Resolved alert bursts respect grouping** - Concurrent recoveries use the configured destination and grouping window across restarts instead of creating one delivery per alert and overwhelming the queue (#2160).
- **Due notifications survive startup** - A persisted notification already due at startup is no longer processed before saved webhook, email and Apprise destinations are applied, so a grouped recovery is not cancelled as "delivery disabled" (#2160).
- **Critical escalations are delivered** - When an active warning becomes critical, the higher severity can bypass only the same-occurrence cooldown and records its destination delivery in notification history (#1801).
- **Escalations use current policy** - Escalation callbacks re-check the current rule, occurrence, acknowledgement and snooze state before admitting a delayed escalation, and an automatic acknowledgement survives a re-fire (#2173).
- **The resource drawer keeps its tab** - Selecting a drawer tab no longer snaps back to Overview when a live data refresh briefly omits a field that gates that tab, such as a merged metrics target on the Proxmox Backups History view (#1723).
- **Filesystem paths stay distinguishable** - Long mount paths use the full Filesystems-panel width and wrap above the usage bar instead of collapsing to identical prefixes (#2121).
- **TrueNAS compatibility improves** - AMD temperature aggregates are rejected instead of inflated, CORE memory uses the legacy mapping, and idle sessions follow a bounded poll cadence (#2122, #2077, #1893).
- **vSphere enrichment recovers** - VMware requests send the expected JSON type names, probe the current 8.0.3 release shape, and surface API fault detail instead of failing with HTTP 500 (#2070).
- **Container update checks match the running image** - All local RepoDigests are treated as the current image, so PostgreSQL and other containers stop reporting a false update (#2110).
- **Failed updates leave no stale state** - A failed staging attempt discards its configuration backup, and a completed update clears its trap so success is not reported as failure (#2127, #2128).
- **FreeBSD and pfSense installs are safer** - The installer verifies agent checksums without GNU coreutils, and commands copied from the web interface preserve the line breaks required by Bash (#2123).
- **Derived agent identity heals safely** - Re-enrolling a forked identity honours the removal block, and the event-log volume stays bounded through the heal (#2113, #1586).
- **Host continuity survives upgrades** - v5 to v6 upgrade paths preserve live host state instead of losing continuity (#1913).
- **AI Patrol prompts are cached** - The stable system prompt prefix is cached so repeated Patrol runs cost less (#2118).
- **Saved Patrol objectives recover** - A saved Patrol objective whose in-memory observer was lost or rejected is reconciled from retained intent and rechecked at run admission instead of staying uncovered (#2147).
- **Provider-hosted clients survive upgrades** - Client workspaces reconcile their own networks and health during provider MSP upgrades, avoiding stranded clients and interrupted renewal (#2247).
- **Provider client networks stay separate** - An existing provider install does not reuse another tenant's Docker network when provisioning or cleaning up clients (#2226).
- **Organization owners see available settings** - Owners can reach the settings routes their own organization is allowed to use (#2208).
- **Hosted agent setup mints scoped tokens** - Agent installation tokens are created inside the selected client workspace rather than from another provider context (#2209).
- **AI knowledge saves stay consistent** - Concurrent AI knowledge-store saves are serialized behind a single writer, so a rapid sequence of saves can no longer defeat the atomic temp-file rename or recreate files during cleanup.
- **Proxmox LXC memory uses the linked agent** - A correlated online Pulse agent inside a Proxmox LXC supplies the container memory reading instead of the cache-inclusive cluster value when the agent total matches the provisioned limit (#2148).
- **Ollama and security settings persist** - Ollama Basic Auth configuration, the configured-admin authorizer and security-setup preferences survive save and restart.
- **Verify SSL choices stick** - A node's "Verify SSL Certificate" setting is preserved through agent re-registration and PVE instance consolidation, and an explicit PBS choice is no longer silently overwritten (#2140).
- **Community-scripts update guidance is corrected** - The upgrade documentation warns that `/bin/update` is the community-scripts updater on some Proxmox helper-script containers and points those users at the signed, version-pinned installer flow (#2129).
- **Metric samples stay consistent** - Replayed unified metric samples are dropped, and metrics store shutdown, rollups and upgrades are hardened.

- **Stable updates become installable again** - Full release metadata is read, offers require the matching server archive, and ARMv6/ARMv7 installs select the correct server build (#2282).
- **Older previews may need manual repair** - An install stuck on a broken self-update path may need a manual, version-pinned update instead of relying on that updater (#2282).
- **Disk wearout warnings follow confirmed recovery** - Corrected PVE SMART warning handling avoids repeating a warning for a recovered or explicitly excluded disk (#2112). The reporter's same-disk email result still needs an installed retest.
- **Retired resources stop generating incidents** - Operator-suppressed or removed resources no longer create the repeated alert activity observed in #2237, installation-level confirmation is pending.
- **TrueNAS CORE charts retain CPU and memory data** - The legacy REST request shape and optional graph failures no longer suppress otherwise available host telemetry (#2077).
- **Linked guests show physical disks** - VM and container storage views retain linked-agent filesystem, SMART and RAID details when that evidence exists (#2263).
- **Queued notifications wait for destinations** - Startup dispatch waits until destination configuration is available, avoiding lost or misrouted recovery messages (#2160).

## Known issues

- The corrected retired-resource alert path has not yet been confirmed on the affected installation (#2237).

- Installed confirmation of the corrected PBS History identity on API-only nodes, agent-linked nodes, standalone PBS systems and multi-datastore systems remains outstanding.
- Native pfSense GUI installation, service start and reboot confirmation remains outstanding.
- Native confirmation of the mixed-source disk correlation remains outstanding.
- Aggregate alert-engine write volume and old duplicate incidents reported in #1966 are reduced but not fully resolved. Monitor write activity on flash storage.
- Some v5 to v6 deployment-specific upgrade problems remain unresolved (#1913).
- TrueNAS CPU and memory graph acceptance on affected CORE appliances remains incomplete.
- Notification-provider acceptance for the corrected escalation and recovery journeys remains incomplete. Notification delivery still depends on destination configuration and provider responses. Do not delete alert or queue files as a workaround.

## Before you upgrade

- This candidate carries every change from the `v6.4.2` packet, which was tagged but never published. If you run `v6.4.1`, read the `v6.4.2` notes for the administrator-boundary changes as well.
- On an SSO-only deployment, map at least one trusted IdP group to the built-in `admin` role before upgrading so an intended administrator retains access.
- Back up the complete Pulse data directory and configuration before opting into the preview channel, and keep the backup until health and data continuity are verified.
- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile remains compatible. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.4.1`. On systemd and Proxmox LXC installs, use `sudo /bin/update --version v6.4.1` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.4.1` and recreate the container.
