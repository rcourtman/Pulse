# Pulse v6.10.0-beta.2 Release Notes

This beta follows `v6.10.0-beta.1` on the way to stable `v6.10.0`. Stable `v6.9.0` remains the rollback target.

Pulse gives service checks their own page, adds DNS checks for resolvers like Pi-hole and AdGuard, and keeps Proxmox VE 9.3 guest charts intact through live migration. Backup views keep the results Pulse could read, and more dashboards flag readings that have gone stale.

## Highlights

- Availability now lets you add, edit, test and pause service checks from its own page in the main navigation.
- Availability’s DNS resolver check queries Pi-hole, AdGuard and other resolvers directly and can verify an expected answer.
- Proxmox Workloads keeps guest chart history through live migration on VE 9.3 and separates guests that reuse a VM ID.

## What's improved

- **Availability charts outages and response times.** Open a check’s Overview for a status timeline and latency ranges over 24 hours, 7 days, 30 days or 1 year. Table and Fleet choices persist, old Machines links redirect, and G then C opens the page.
- **What’s New is easier to scan and reopen.** Expand longer sections as needed, or reopen the dialog from the footer version or Settings → System → Updates.
- **Resource drawers put the time range beside the tabs.** Healthy drawers summarize passing checks, repeated status messages are trimmed, and viewers can read saved Manage settings and open or copy web links.
- **Infrastructure Map is available in preview builds.** Switch between host and role layouts, search folded resources, filter for problems, inspect groups or networks, and pan or zoom. Layout, density and live traffic settings are remembered.
- **The preview Map shows live network and disk activity.** Open Live traffic to find busy resources. Host views show link speed, saturation, down links, CPU, memory, temperature, physical disks and storage pool warnings. Old rates drop out.
- **The preview Map shows how services connect.** Published & routed lists ports and proxy routes, including Caddy and Kubernetes Ingress. Hover to see shared Docker mounts and observed internal peers. Hosts and workloads can show accelerator badges.
- **The preview Map adds keyboard and sharing controls.** Use `/` to search and `+` or `-` to zoom. Save either layout as an image, or turn on Hide names to clear active searches and network filters and conceal identifying details.
- **Machines → VMs is available in preview builds.** Browse reported Libvirt/KVM and XCP-ng guests by host, state or name. Live updates preserve guest state and vCPU counts.

## Alerts and notifications

- **Alerts Overview can search and filter by severity.** Thresholds shows a loading state and keeps Save Changes and Discard in view while you scroll. Alert History opens its frequency chart at the latest period on narrow screens.
- **PBS CPU alerts use the displayed percentage.** Rolling alerts use the backup server’s own samples, keeping readings from linked agents and same-named servers out of the calculation.
- **Connection alerts stay in order under load.** Slow checks apply their results in order, and notification health checks can run while monitoring continues.
- **A successful send can recover recent missed notifications.** Eligible failures for the same saved destination can return to the queue, including after Pulse restarts.
- **Resolved notifications keep the firing severity recorded for that destination.** A critical alert that quietly drops to warning before clearing can still resolve as critical. For automation matching, use the alert ID, which also works with older deliveries.
- **Certificate failures stop retrying.** Email and Webhook tests identify certificate trust problems and give guidance for the affected destination.
- **Delivery records are easier to match.** A queue ID ties together delivery, retry and test records. The delivery log keeps the recorded failure reason, and the dead-letter list summarizes failed deliveries.
- **Quiet Hours and Apprise controls are clearer.** Quiet Hours gives timezone and suppression settings more space, and unavailable Apprise test controls behave like Email’s.
- **Alerting Help explains how to trace an alert.** Compare its live threshold readings with the saved message, then check delivery separately. Help also covers Telegram alerts that fail after a passing test and built-in Ceph health alerts.

## Disks and storage

- **Slow Proxmox storage requests get the configured time.** Backup listings on slow mounts can wait beyond the old 10-second cutoff. Pulse reports timeouts separately from permission denials.
- **Small Proxmox and PBS readings keep their scale.** Disk and memory percentages retain their actual values in Resources and AI reads. Proxmox Nodes bars and Trends show small values to two decimal places.
- **Proxmox Nodes makes nearly full disks visible.** The compact disk bar uses the fullest reported filesystem, so a small disk near capacity stays visible.
- **Ceph distinguishes stale readings and unknown health.** Both stay out of Healthy. The table retains last-known quorum, services and capacity, and the drawer flags old readings without plotting them as live samples.
- **Physical Disks uses recognizable host names.** Disks show the matched node name, with the reported name on hover when it differs. Pool, array or filesystem usage has its own label.
- **Unraid array faults show on collapsed Machines rows.** The row shows severity and a short reason. The tooltip and expanded details carry the full explanation.

## Proxmox, PBS and backups

- **Proxmox VE 9.3 keeps same-named clusters separate.** Existing guests keep their identity when upgraded, and independent clusters and standalone nodes with matching names stay separate.
- **Backups keeps the restore points it could read.** Retry only the failed source and stay on the selected inspection. Matching stays unchecked if guest inventory is unavailable, and an empty readable source leaves other restore points visible.
- **Backups names the connection that needs attention.** Warnings identify the saved Proxmox or direct PBS reader, distinguish denied access from connection problems and explain when the list is still being checked.
- **Coverage explains what counts.** Checking and Not evaluated stay outside the rated total. Exception filters are removable, the newest point’s age reflects the listed results, and protection status can be retried separately.
- **Backup Jobs keeps warnings with the selected day.** Open By date for that day’s restore points or clear the date for undated entries. Running backups show In progress. Help explains direct PBS’s fixed seven-day verification window.
- **Backup Servers labels listing counts and stale capacity.** Counts reflect the entries Pulse could list, unread counts are unavailable, and retained capacity and forecasts are flagged. Failed datastore jobs affect server status.
- **PBS removes datastores after a complete empty read.** Deleted or excluded stores leave Storage. A timed-out read can no longer publish an incomplete list as healthy, and paused PBS or Mail Gateway connections stop recording poll successes.
- **Proxmox Nodes shows reading status by name.** Stale readings, API errors and degraded connections have distinct labels. Trends uses the recorded node series when another resource shares its name or ID.
- **Replication keeps job identity and freshness visible.** Search stays broad until you pick a job by guest and connection. Stale jobs leave Healthy, with last-polled details and any reported failure kept visible.
- **Cluster refreshes respect saved edits and pauses.** Member checks keep the address they actually tested. Canceled or paused discovery cannot overwrite saved settings, overlapping edits are preserved, and failed refreshes are reported.
- **Proxmox and Mail Gateway report startup failures.** Infrastructure and Scheduler Health show errors such as denied access or unreachable hosts. Connections wait for a successful poll before showing healthy, and Mail Gateway retries after startup outages.
- **Mail Gateway flags stale mail and queue readings.** Stale gateways leave Healthy. The Mail tab loads in place, renders the spam histogram, explains queue and quarantine counts, and shows why a gateway is degraded.

## TrueNAS, vSphere and Docker

- **TrueNAS Storage counts matching rows.** Status and storage-type totals include matching datasets below a degraded pool.
- **TrueNAS marks old readings across its views.** Systems, storage, apps, shares and Protection flag retained values while keeping faults visible. Stale protection records leave Healthy and Running, and warnings clear when fresh readings arrive.
- **TrueNAS refreshes respect timeouts and connection edits.** A valid edit can retain the last readings until a fresh poll. An invalid replacement stops use of the old connection, and repaired or re-enabled connections poll promptly.
- **vSphere flags stale datastores, hosts, networks and activity.** Fresh vCenter updates take priority, missing capacity shows as unavailable, and last-known alarms and failures stay visible. The early-access integration remains available.
- **TrueNAS and vSphere respect connection removal.** Late results cannot restore deleted or paused connections or overwrite their replacements. Stopping Pulse no longer records canceled reads as failures.
- **Proxmox and vSphere keep readable workloads visible.** Workload lists and search results show what Pulse could read and name the connection with missing inventory.
- **Docker Applications can search and filter Compose apps.** Find apps by name or health, open their details and spot failed HTTP checks. Hosts and Containers have clearer names, image labels and action columns.
- **Docker tabs use filters that fit their data.** Images can show updates, Storage can show active or reclaimable items, and Networks can show networks in use. Readiness lists reuse inventory on large Docker setups.
- **Docker marks old inventory readings.** Compose apps, Swarm services and records, Images, Storage and Networks flag stale data while retaining last-known counts and failures. Stale networks move into Attention.
- **Platform alerts flag stale source data.** Docker, Kubernetes, TrueNAS and vSphere alert tables and details keep the last reported severity while showing when collection has fallen behind.

## Install, updates and agents

- **Windows agent updates avoid a file-in-use failure.** The updater finishes saving the staged file without reopening it. If Agent Doctor repair fails on certificates, use a Pulse HTTPS address whose certificate Windows trusts.
- **Manual updates keep the selected agent’s Proxmox setup.** If you deliberately opted out with `--disable-proxmox`, include it again with `--update`.
- **Agent Doctor shows repair details for the affected install.** See reported and target versions, OS and profile. After generating a token, copy the repair command from that agent’s details.
- **Install Pulse Agent puts the token and command in reach.** Standalone agent management opens the installer for the detected profile and shows uninstall commands for the detected OS.
- **Locked Docker actions open Enable remote commands.** Admins can open it from Update, Start, Stop or Restart, copy the host command and continue when the agent reconnects with commands enabled.
- **Pulse detects memory limits in more Linux containers.** Delegated and inherited cgroup mounts can supply the configured cap, including in Proxmox LXCs.
- **Install Help lists server resource needs.** Start with 1 to 2 vCPUs, 1 GiB of RAM and 4 GiB of disk. Keep at least 1.2 GiB free for an in-place upgrade.
- **Update Help explains readiness and recovery checks.** It covers persistent container storage and how to confirm new polls for each connection and fresh readings from agents after the update.

## Pulse Pro, AI and hosted

- **Background Patrol proposals stay open for 24 hours.** New and refreshed plans give you more time to review them. Pulse checks current conditions and permissions before running a fix.
- **Patrol gives slow summaries more time.** Final responses use the investigation’s timeout for local and OpenAI-compatible providers. Overdue runs are recorded as timed out.
- **Patrol Inbox can load findings beyond the first 50.** Counts agree with pagination, the weekly digest reflects the active automation mode, and Watch only describes reviewing issues.
- **Assistant and Patrol enforce session and tool permissions.** Access to sessions, finding controls and raw tool output follows current permissions. File, URL and command reads have tighter limits.
- **AI provider keys stay with their saved server.** Changing the server address requires credentials for the new destination. Provider errors and tool results expose less sensitive data.
- **Hosted access follows workspace membership.** Promotions, demotions and removals sync to the workspace. An old sign-in link can no longer restore access for a removed member.

## Monitoring and service health

- **DNS checks can run from remote agents with Pulse Pro.** Pick an observation location to test its resolver. Results distinguish a wrong or failed DNS answer from a resolver the agent cannot reach.
- **HTTPS checks offer certificate controls.** Pin a fingerprint or explicitly allow an unverified certificate in the editor. A failed test can show the fingerprint the server presented.
- **Availability uses current check settings.** Canceled probes leave no false outage, late results cannot revive paused, edited or deleted checks, and resumed remote checks get a fresh window to report.
- **A Proxmox lock conflict no longer stalls collection.** It could stop VE, PBS and Mail Gateway polls and back up agent reports while the dashboard still answered.
- **Scheduled polls keep their turn.** Due work stays eligible, earlier work wakes idle workers, a slow poll holds one slot, and workers survive a poll crash. Retries respect backoff, and a successful poll clears the old terminal failure.
- **Scheduler Health shows overdue work and partial reads.** Compare the oldest queued poll, recent outcomes and scheduled retries by connection. Same-named connections no longer hide each other’s stale polls.
- **Prometheus distinguishes waiting, running and failed polls.** Overdue and retry metrics age between results, stopped queues drop out, and retired failure series expire. Help shows how to alert on stalled polls and agent reports.
- **External watchdog status tracks recovery and endpoint changes.** Signals continue if local alerts are blocked. Recovery clears stale stall alerts, delivery errors get their own status, and a replacement endpoint starts with fresh timestamps.
- **The dashboard waits for its first readings.** It shows loading while the first snapshot arrives, even if the live connection is already open. Connection status badges have clearer contrast in both themes.
- **Large inventories use less temporary memory.** Proxmox guest and backup polls, PBS snapshot matching, Recovery, agent checks, broadcasts and scheduler scrapes make fewer copies. History reuses space as old points expire.
- **History Help explains where readings come from.** Devices can supply chart points from during a Pulse outage. Check fresh live readings to confirm collection resumed. Remote-site Help explains how Pulse reaches Proxmox and PBS.

## Security

- **Proxmox VE 9+ setup skips guest file-read permission by default.** Setup Doctor can remove the old grant from PulseMonitor. Pulse skips disabled Linux file commands without pausing other guest reads. Use ballooning or an existing agent for memory detail.
- **Proxmox and PBS setup protect saved connections.** Discovery respects saved certificate pins and credential scope. Cluster merging checks both endpoints, and Proxmox address changes require credentials for the new destination.
- **Configuration imports respect workspace permissions.** Scoped imports cannot replace protected server credentials or tokens, and imported destinations go through the same checks as saved settings.
- **Monitoring-only agents keep their command limits.** Windows service startup, Docker updates and agent settings enforce the limits. Host, Docker, Kubernetes and Ceph reports stay tied to their owning tokens and workspaces.
- **Remote actions stay on the authorized resource.** Sessions check agent token ownership, host and guest actions stay on target, and log reads stay within allowed paths.
- **Sensitive requests recheck current access.** Credential rotation ends admin sessions. Action and settings changes recheck access before applying, and SSH setup requires admin settings access.
- **SSO binds identities and roles to their provider.** Changed group mappings remove old grants. An explicitly unverified OIDC email cannot satisfy an email or domain restriction.
- **Large or invalid inputs are bounded.** Limits cover agent reports, alert operations, platform inventories and live requests. Metric history, backup alerts and recovery totals stay scoped to the right resources and workspaces.
- **Notification secrets stay with their saved destinations.** SMTP, Webhook and Apprise validate destination changes. Webhooks keep credentials and alert content within the original destination when following redirects.
- **Installers enforce pinned server certificates throughout downloads.** Least-privilege Linux installs keep the agent executable root-owned and give the service account ownership of its state.
- **Hosted audit records use the authenticated actor.** Pulse takes the actor from the signed-in session and limits oversized entries.

## Other improvements

- **Kubernetes marks stale objects throughout inventory.** Clusters, nodes, workloads, autoscalers, services, networking, storage, configuration, policy and events flag old readings in tables and details while preserving failures and event severity.
- **Kubernetes Review opens workloads that need attention.** It keeps cluster and namespace filters, includes degraded autoscalers, and Workloads adds Kind. Workloads, Services and Configuration hide empty tables while filters are active.
- **Kubernetes filters fit each table.** Events, Services, Storage and Configuration show relevant categories. Metadata-only ConfigMaps and Secrets collapse, and event messages have more space.
- **Infrastructure makes connected systems easier to find.** Discovery sits above the searchable list. Coverage includes Docker, Kubernetes and cluster host telemetry. Standalone rows link to Machines, and profiles group by OS.
- **The Proxmox editor puts saved settings first.** Edit the cluster name directly, expand healthy members and setup scripts as needed, and find Pulse Agent options above Save changes.
- **Settings is easier to navigate.** Search recognizes more terms, tabs open at the top, Diagnostics shows readable uptime, and System Logs can filter and wrap lines.
- **Docs is easier to browse.** Filter the index, use the table of contents or choose Back to Pulse. Direct `.md` links open in the built-in reader.
- **The command palette adds Actions, Docs, themes and kiosk mode.** Open or close it with Cmd/Ctrl+K. G shortcuts work while type-to-search is enabled, and keyboard focus is easier to see.
- **Sign-in preserves your username after a failed attempt.** Failed session checks link to Help. SSO explains when a session could not be confirmed, and Authentication shows how to reveal the hidden local form.
- **SSO setup keeps registration details available.** Copy the saved OIDC callback or open a disabled SAML provider to copy its URLs. Tests flag unreadable certificates, and group mappings preserve punctuation and flag invalid rules or missing OIDC scopes.
- **API Access shows last recorded token use before revocation.** If automatic copy fails, select the token in its reveal dialog. Audit Log shows the event outcome separately from its integrity check.
- **Workspace and role dialogs explain who gets access.** The picker shows the full workspace name, Sharing explains its audience, role deletion explains which grants or denies go away, and declining an invite asks for confirmation.

## Known issues

- **Guest freezes and unexpected Windows guest restarts remain unresolved.** If you stopped monitoring after a guest froze, do not restart it solely to test this release or a beta.
- **Stop Pulse before freeze-enabled backups.** Confirm polling stopped. Even if the backup reports OK, verify thaw, fresh writes on every backed-up filesystem and normal workload operation before resuming. Monitoring and alerts pause while Pulse is stopped.
- **Resume only services and timers that were active.** Avoid hooks that ignore a failed stop or resume after a failed or unanswered thaw check. On Kubernetes, confirm persistent Pulse storage and prevent controllers from replacing stopped pods.
- **Guests can freeze without a VM lock.** Cloning a running VM or importing disks can freeze it. Reads already in progress can interfere with thaw. Avoid manual guest-agent probes during freeze or thaw.
- **Some Windows guests can still lack Proxmox disk readings.** Use working Machines readings or check in Windows. Keep existing agents and avoid guest restarts or forced probes just to recover the reading.
- **High server memory and out-of-memory stops are still reported.** Keep rollout staged if pressure rises or readings stop. Compare LXC memory and swap at similar uptime, workload and limits. Keep readings separate across restarts or counter resets.
- **Use existing logs after a memory stop or host lockup.** Check the Proxmox host kernel and Pulse logs privately. Leave affected agents or Patrol off if paused. Avoid restarts, heap dumps or GPU load changes just to reproduce the problem.
- **Check agent and helper versions after updates.** Use installer maintenance if the helper stayed old. If an older ARMv6 agent offers ARMv7, update manually for ARMv6 and keep its ID and enrollment.
- **Earlier Alert History gaps cannot be recreated.** Keep existing logs and delivery records after a storage failure, even after the history store recovers.
- **TrueNAS CORE can still show blank pool capacity or Disk I/O.** Check pool capacity in TrueNAS Storage and whole-host Disk I/O in TrueNAS Reporting. Keep the existing API key and permissions.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.9.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.9.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.9.0` and recreate the container.
