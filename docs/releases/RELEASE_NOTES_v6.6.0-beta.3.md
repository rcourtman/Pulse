# Pulse v6.6.0-beta.3 Release Notes

This beta follows `v6.6.0-beta.2` on the way to stable `v6.6.0`. Stable `v6.5.0` remains the rollback target.

Pulse v6.6.0 makes it easier to see what needs attention and trust the readings behind it. Resource drawers bring alerts and charts together, backup coverage is clearer, and stale data no longer looks like a fresh measurement. This release also improves disk tracking, Proxmox guest monitoring, notification editing, agent installation and recovery, and access controls.

## Highlights

- Find problems faster with clearer alerts, resource drawers and backup coverage.
- Trust monitoring readings through backups, missing data, disk replacements and workload moves.
- Set up cluster agents and Patrol with fewer manual steps.

## What's improved

- **Charts beside the problem.** Guest and node drawers show alerts and 24-hour charts on Overview. Longer ranges are selectable there, replacing the separate History tab.
- **Readable chart values.** Charts show real scale labels, round markers, alert trigger lines and dated readings. Values at an inspected time stay aligned, with missing readings clearly marked.
- **Useful details first.** Drawers bring warnings and failed availability checks forward, put inventory under Details, and show full identity values without truncation or repeated identifiers.
- **Web links in bulk.** Edit saved web interfaces for multiple workloads or Docker containers together, then open a guest's saved interface from its drawer header.
- **Workloads keep their identity.** Proxmox guests retain saved metadata, links, selection and history when they move between nodes. Similar names at different installations stay separate.
- **Manual grouping stays yours.** Links, chained links and splits behave consistently across refreshes and restarts. Unlinking separates rows, and reporting a bad merge removes only the links responsible.
- **Less dashboard overhead.** Large inventories require less repeated copying, grouping and sorting. Tables fetch only the history they display, and live updates preserve large numeric counters accurately.

## Alerts and notifications

- **Find the affected resource.** Alerts Overview puts active problems first. Platform links, guest alerts and filesystem alerts open the resource they belong to, including guests after a node move.
- **See why an alert remains open.** Threshold alerts show the current reading, averaging window, clear level and recovery progress. The last breach keeps its original value and date.
- **Missing readings interrupt confirmation.** Metric, storage, connection and disk-temperature timers restart after observation gaps, preventing separated readings from counting as continuous trouble.
- **Off stays off.** Disabled defaults remain disabled after saving, including Docker, Mail Gateway and backup thresholds. Low thresholds retain a clear level below their trigger.
- **Settings survive other saves.** Saving configuration preserves omitted alert settings and node overrides. Filesystem limits and pulse-relaxed thresholds agree across editing, polling and row colours.
- **Alerts survive source changes.** Agent handovers avoid duplicate node alerts without losing temperature coverage. Proxmox cleanup no longer removes Kubernetes, TrueNAS or vSphere alerts.
- **History explains closures.** Moved alerts are distinguished from recovered alerts in timelines, reports and notifications. Docker hosts, containers, services, disks, arrays and sensors keep the correct history owner.
- **History remains usable.** Messages wrap instead of truncating, acknowledgement changes sync between tabs, and closed alerts disappear from the active list. History caching uses bounded memory.
- **Notification setup is clearer.** Destinations lead the page, disabled services have compact summaries, and paused delivery explains whether detection, snooze or activation is responsible.
- **Edits are safer to save.** Notification drafts survive tab changes and uncertain saves. Webhook saves, toggles and deletes wait for confirmation, preserve explicit edits beside masked secrets, and report partial failures.
- **Real webhook sends match tests.** Built-in services receive the required headers, including Telegram's JSON header. Clearing an optional header restores defaults, and copied payload examples escape alert text.
- **Email is easier to identify and configure.** Subjects include resource and host names, including non-ASCII names. Rate-limit edits persist, and unsupported reply-to and retry controls have been removed.

## Disks and storage

- **Storage puts capacity problems first.** Tables emphasise filling pools and explain attention states. Collapsing groups no longer pins them, reopens them or scrolls back to an earlier pool.
- **Different disks stay separate.** Matching serials on different machines and generic QEMU serials no longer merge unrelated hardware. Agent disks also stay out of unrelated cluster-member state.
- **Replacement disks start clean.** A new disk no longer inherits the old disk's SMART data, slot identity, alert history or operator settings when the hardware no longer matches.
- **Temperature follows its source.** Merged disks retain their own collection state. Standby disks and silent agents show last-known temperatures, usage and RAID state instead of live readings.
- **Heat limits agree everywhere.** Disk alerts, colours, reports and Patrol use disk-type limits and host overrides. TrueNAS disk limits are editable, and Unraid temperatures use the readings shown by the agent.
- **Heat alerts clear correctly.** Removing a disk or switching its temperature threshold off clears its alert. Missing reports preserve unresolved SMART problems without treating old heat readings as current.
- **Risk labels are clearer.** Heat alone reads Running Hot instead of Replace Now. SSD wear is labelled life remaining, and disk alerts are recorded once.
- **Disk charts follow the right hardware.** Controller-member and Unraid serial-based history resolves correctly, with separate charts for same-serial disks on different hosts.
- **Local mounts remain visible.** Real disk mounts beside system directories are no longer filtered out merely because their paths resemble excluded locations.

## Proxmox, PBS and backups

- **Memory readings explain their limits.** Guest tables, drawers and alert settings distinguish current, retained and cache-inclusive memory. Guest percentages use the measured guest total, including ballooned VMs.
- **Windows guest readings recover.** Pulse learns the guest OS before optional Linux memory reads, avoiding unsupported requests that could delay filesystem collection.
- **Filesystem totals are more accurate.** Shared filesystems count once regardless of mount order, while distinct Windows volumes remain separate even when their sizes match. Invalid counters stay unknown.
- **Partial reads preserve useful data.** Valid filesystem rows survive incomplete responses without implying complete capacity or recovery. Stopped VMs no longer publish retained filesystems as current readings.
- **LXC collection works inside the helper sandbox.** Running containers and mounted filesystems can be read without Proxmox or LXC control sockets. Unmounted paths no longer borrow parent filesystem usage.
- **Guest commands respect backup safety.** Reads and actions defer for locks, uncertain completion and malformed replies. Equivalent endpoints share coordination, and cancelled commands cannot release another command prematurely.
- **Slow polls keep valid inventory.** Completed guest inventory survives optional-detail deadlines. Retired connections and cancelled cycles cannot republish old readings, and temporary access failures no longer permanently suppress OS reads.
- **Busy connections share login renewal.** Concurrent Proxmox reads renew expired sessions together, reducing repeated authentication and avoiding late failures against an already replaced session.
- **Backup protection stays honest.** Incomplete PBS namespace or snapshot reads preserve known backups without marking them freshly checked or deleting them. Root-namespace coverage also works for LXC guests.
- **Backup coverage is easier to inspect.** Health counts filter Coverage to matching workloads. Badges expose completed-backup details, update in open drawers, and explain freshness, verification and longer backup cycles separately from alert policy.
- **Replication reflects observed results.** Job status comes from source-node observations. Slow reads do not pile up, failed reads do not erase jobs, and configuration alone no longer implies a successful sync.
- **Node setup messages are actionable.** Denied package-update checks explain their separate access requirements without flagging narrower permissions as a fault. Outdated sensor setup notices appear across Proxmox tabs.

## TrueNAS, vSphere and Docker

- **TrueNAS CORE inventory survives unknown metadata.** An unknown or string-valued disk rotation rate no longer breaks disk collection.
- **TrueNAS capacity stays credible.** Missing or conflicting pool counters remain unknown instead of looking empty. Fractional SCALE app CPU readings no longer interrupt the reporting session.
- **Native TrueNAS temperature charts return.** Disk drawers can use stored native history, with a bounded Netdata fallback when the normal reporting method returns an empty series.
- **TrueNAS explains unhealthy storage.** Pools and topology show concrete reasons for attention. Services and VMs distinguish stopped items that should be running from intentionally stopped items.
- **vSphere leads with health.** The overview highlights vCenter health and explains datastore and network problems. Host groups use cluster names, and the vCenter column appears only with multiple connections.
- **vSphere activity reads naturally.** Events use vCenter's own descriptions instead of event-class names, and vSphere VM records retain their correct platform ownership.
- **Docker health is visible.** Container tables show health and uptime, drawers include health-check results, and silent hosts are clearly identified. Host rows also reflect their open alerts.
- **Image references are readable.** Docker images display repository and tag clearly, with update badges and lifecycle columns fitting narrower layouts.
- **Update checks handle uncertainty.** Missing local image digests no longer clear existing update alerts. Explicit Docker Hub references reach the correct manifest API, and unavailable checks explain why.
- **Container CPU scales are accurate.** Podman readings use the measured interval consistently across reports, history and alerts. Small fractional usage stays visible, with help explaining CPU percentage scales.

## Install, updates and agents

- **Install agents across a Proxmox cluster.** A connected member agent can install on uncovered siblings using existing cluster SSH trust and individual bootstrap credentials. Progress confirms a live connection.
- **Enrollment happens before collection.** Agent modules start with the exchanged runtime credential, and deployed command channels use Pulse's resolved identity so they remain connected after restarts.
- **Monitoring authority can be reduced.** Reporting agents can establish their identity without opening a command channel, allowing collector permissions to be reduced correctly.
- **Removal preserves recovery files.** Unix, Windows, Synology DSM and TrueNAS SCALE uninstall paths confirm services and boot tasks have stopped before removing credentials or recovery state.
- **Saved credentials remain usable.** Installers respect the selected token file, preserve caller-owned files, and retain existing credentials when the saved server is unavailable. Copied removal commands use saved credentials.
- **Server replacement is safer.** Updates prepare and verify binaries before replacement, confirm Pulse has stopped, and refuse ambiguous service state. Source-built installs also use atomic replacement.
- **Automatic updates respect your settings.** Update consent, channels and systemd masks are read reliably. Unknown editions are excluded, and success requires the installed version and running executable to match.
- **Update failures are reported accurately.** Installer and log-collector results remain separate, including final output without a newline. Recovery material is retained when completion cannot be confirmed.
- **Startup import protects existing state.** Automatic import requires an empty destination and checks the actual persistence path again before writing, including agent-only, TrueNAS and vSphere installations.
- **Agent setup is clearer.** First-run Infrastructure screens avoid claiming complete coverage prematurely. Update notices explain what changes, CPU counts are clearer, and Linux distribution names use OS metadata.

## Pulse Pro, AI and hosted

- **Set up Patrol where you use it.** The Patrol page connects a provider, selects a guided model, previews costs and checks readiness. Unreachable providers lead to Provider & Models.
- **Pro Patrol starts with Ask first.** Until a mode is explicitly chosen, proposed fixes require review. Safe auto-fix and Autopilot remain available through an explicit mode choice.
- **Assistant plans are reviewed in Actions.** Chat offers Read-only and Ask first controls. It saves proposed actions for review instead of exposing a separate autonomous chat execution mode.
- **Guest protection uses working controls.** The inactive Assistant Protected guests field is removed. Never auto-remediate in a resource's Manage tab blocks typed actions, including previously approved plans.
- **Protection survives regrouping.** Remediation blocks remain effective across manual links, splits and identity changes. Human-approved actions check current operator policy before dispatch.
- **Suppression rules are reversible.** Patrol Activity lists manually created rules with their scope and reason, and lets you remove them. Ending suppression leaves reviewed findings reviewed.
- **Maintenance affects the right findings.** Patrol honours recurring windows and maintenance covering descendants. Expected-offline settings silence availability findings while allowing other problems through.
- **Investigations target the correct resource.** Host filesystem and disk alerts reach their canonical owner. Repeated names or VMIDs across installations no longer select an arbitrary action target.
- **AI uses trustworthy readings.** Cache-inclusive memory does not prove memory pressure, and missing or retained readings do not prove recovery. Unavailable storage is distinguished from storage you disabled.
- **Coverage warnings can clear.** A successful complete Patrol run removes an obsolete partial-coverage caveat. Run ages and future schedule times update without refreshing the page.
- **AI Discovery has been removed.** Service context scans, the Discovery tab and automatic web-link and availability proposals are no longer offered.
- **Provider-hosted portals start reliably.** Setup can keep the portal reachable without an initial licence, and a saved paid renewal remains effective after restart. Tenant shutdown waits for active work and history writes.

## Monitoring and service health

- **Freshness follows the polling schedule.** Proxmox, PBS, Mail Gateway, connection summaries and resource APIs use the live fixed or adaptive cadence, avoiding false stale states between healthy polls.
- **Offline means offline.** A recently delivered report cannot override its source's offline verdict. Linked agents use their own heartbeat, and failed availability checks no longer keep nodes online.
- **Availability checks agree.** Settings and Machines show consistent results. Stale successful locations cannot be kept current by newer failing locations, and licence changes return checks to local observation correctly.
- **Missing data stays out of trends.** Retained guest readings do not create new history points or bridge gaps. Memory and disk searches exclude unavailable measurements instead of treating them as zero.
- **CPU temperature is consistent.** Tables, drawers, history and alerts use the hottest current CPU reading. Node drawers show real low and record values, and same-named nodes at different sites stay separate.
- **Status ages keep moving.** Resource, replication, alert, delivery and watchdog ages update on a shared clock. A stalled watchdog read no longer prevents Refresh from completing.
- **Persistent failures stay bounded.** Polling backoff keeps its maximum delay through long outages rather than overflowing into rapid retries.
- **Kubernetes problems stand out.** Not Ready nodes lead the overview, node rows reflect their alerts, and long names remain readable and copy correctly across platform tables.
- **Mail Gateway alerts stay attached.** Rows and drawers show the gateway's own alerts and highlight the affected count. Saved collection opt-outs remain explicit.
- **Demo monitoring behaves consistently.** Mode changes update all tenant monitors and clear old fixture alerts. Demo workloads have stable parents, distinct names and one chart timeline per resource.

## Security

- **Tokens cannot widen their own authority.** Token callers cannot grant scopes they lack or create instance-wide credentials from narrower access. Credential rotation requires the appropriate instance-wide authority.
- **Organization boundaries apply to credentials.** Token lists, diagnostics, editing, deletion and rotation expose only credentials the caller may manage within its organizations.
- **Mutation routes enforce permissions.** Resource links and attention changes require monitoring:write. AI report narration requires ai:execute, and session or settings mutations reject read-only HTTP methods.
- **Administrative removal requires an admin.** Browser callers cannot uninstall agents without administrator rights. Read-only tokens no longer see web-link editing controls.
- **SAML sign-in works securely.** Login responses are tied to the initiating browser and outstanding request. Logout replies must match the session's request, and replayed or altered responses are refused.
- **Saved certificate trust is preserved.** Proxmox recovery cannot replace an established certificate pin. Diagnostics distinguish timeouts from trust failures and capture certificates from the configured endpoint.
- **Redirects cannot move credentials elsewhere.** Infrastructure requests and notifications keep credentials and private bodies within their configured origin. Foreign redirects fail before their destination enters diagnostics.
- **Webhook diagnostics reveal less.** Receiver response bodies and unsafe retry hints are withheld from logs. Saved secret fields remain masked in API responses.
- **Demo restrictions remain enforced.** WebSocket headers cannot bypass blocked writes, and profiling endpoints are hidden on demo instances.
- **Security status matches exposure.** Plain HTTP is marked critical when Pulse is publicly reachable, with a recommended HTTPS step for private-network instances.
- **Security dependencies are refreshed.** Go, networking and frontend dependencies include fixes for reported vulnerabilities. Production styling also works under Pulse's content security policy.

## Other improvements

- **Tables behave more predictably.** Selecting text no longer toggles rows, drawers keep spanning visible columns, and workload selection no longer restores an obsolete scroll position.
- **Kiosk views show more monitoring.** Filter toolbars and section tabs are hidden, and table preview limits are lifted.
- **Controls are easier to read.** Light-mode banners have stronger contrast, disabled toggles show their actual state, and Manage explains each operator control in plain language.
- **Logs stay responsive.** Slow viewers no longer block server logging, and the browser batches incoming lines. Pause, clear and download controls explain their effects.
- **Views can be bookmarked.** Patrol and Actions views have dedicated routes, and links to Patrol's mode selector open the relevant control.
- **Troubleshooting is more practical.** Updated guides cover backup-safe guest checks, PBS permissions and verification, Kubernetes credentials, OIDC setup, CORS and NAS installation with private, bounded diagnostics.
- **Reporting guidance is clearer.** Bug-report paths distinguish the monitored target from the Pulse server, explain damaged tokens without unnecessary credential changes, and make specialist diagnostics optional.
- **Audit requests fail clearly.** Malformed export and summary time filters are rejected, and concurrent tenant incident records retain distinct identifiers.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile remains compatible. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.5.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.5.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.5.0` and recreate the container.
