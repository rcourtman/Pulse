# Pulse v6.10.0-beta.1 Release Notes

This beta opens the `v6.10.0` candidate line after stable `v6.9.0`, which remains the rollback target.

Pulse gives service checks their own page and adds DNS monitoring. Proxmox VE 9.3 guests keep their chart history during live migration. Backup views show what Pulse was able to read, and monitoring diagnostics help you spot when collection falls behind.

## Highlights

- Availability now lets you add, edit, test and pause service checks from its own page in the main navigation.
- Availability’s DNS resolver check queries Pi-hole and other resolvers directly and can verify an expected answer.
- Proxmox VE 9.3 monitoring keeps guest charts through live migration and separates reused VM IDs and same-named clusters.

## What's improved

- **Availability check details chart outages and response times.** Open a check’s Overview for a status timeline and a response-time chart with averages and ranges. Choose 24 hours, 7 days, 30 days or 1 year.
- **Availability remembers your layout.** Table and Fleet choices persist. Old Machines links redirect to the new page, which you can also open from Search or with G then C.
- **What's New is easier to scan and reopen.** Expand longer sections as needed. Reopen the dialog from the footer version or Settings → System → Updates.
- **Infrastructure Map is available in preview builds.** Search the full map, filter for problems, inspect groups and networks, pan and zoom, or retry a failed load.
- **Machines → VMs is available in preview builds.** Browse reported Libvirt/KVM and XCP-ng guests by host, state or name. Live updates preserve guest state and vCPU counts.
- **Infrastructure's Proxmox editor puts saved settings first.** Edit the cluster name directly, expand healthy members and setup scripts when needed, and find Pulse Agent options above Save changes.
- **Resource details put the time range beside the tabs.** Healthy drawers skip the empty alert area and summarize passing checks. Viewers can read saved Manage settings and open or copy web links.

## Alerts and notifications

- **Alerts Overview can search and filter by severity.** Thresholds shows a loading state and keeps Save Changes and Discard in view while you scroll.
- **PBS CPU alerts use the displayed percentage.** Rolling alerts use the backup server’s own samples, keeping readings from linked agents and same-named servers out of the calculation.
- **Connection alerts stay in order under load.** Slow connection checks apply their results in order, and notification health checks can run while monitoring continues.
- **A successful send can recover recent missed notifications.** Eligible failures for the same saved destination can return to the queue, including after Pulse restarts.
- **Certificate failures stop retrying.** Email and Webhook tests identify certificate trust problems and show guidance for the affected destination.
- **Delivery records are easier to match.** A queue ID ties together delivery, retry and test records. The delivery log preserves the recorded failure reason.
- **Notification settings are easier to scan.** Quiet Hours gives timezone and suppression settings more space, and Apprise test controls follow the same disabled-state behavior as Email.
- **Alerting Help explains how to check progress.** Compare the live summary on an open threshold alert with its saved message, then check delivery separately. Telegram Help covers real alerts that fail after a passing test.

## Disks and storage

- **Slow Proxmox storage requests get the configured time.** Backup listings on slow mounts can wait beyond the old 10-second cutoff. Pulse reports timeouts separately from permission denials.
- **Small Proxmox and PBS readings keep their scale.** Disk and memory percentages retain their actual values in Resources and AI reads. Proxmox Nodes bars and Trends show small values to two decimal places.
- **Proxmox Nodes makes nearly full disks visible.** The compact disk bar uses the fullest reported filesystem instead of averaging across disks.
- **Ceph shows Unknown when health is unavailable.** Unknown clusters stay out of Healthy. The table also shows quorum and summarizes running services.
- **Physical Disks uses recognizable host names.** Disks show the matched node name, with the reported name on hover when it differs. Pool, array or filesystem usage has its own label.
- **Unraid array faults show on collapsed Machines rows.** The row shows severity and a short reason. The status tooltip and expanded details carry the full explanation.

## Proxmox, PBS and backups

- **Backups keeps the restore points it could read.** Retry only the failed source and stay on the selected inspection. When guest inventory is unavailable, matching stays unchecked. A readable source with zero results leaves other restore points visible.
- **Backups names the connection that needs attention.** Warnings identify the saved Proxmox or direct PBS reader, distinguish denied access from connection problems and explain when the list is still being checked.
- **Backup health shows which workloads count.** Checking and Not evaluated stay visible outside the rated total. Selecting a health exception shows a removable filter.
- **Coverage labels the age of the newest listed point.** Partial reads and location filters can narrow the result. Retry protection status separately when inventory is already visible.
- **Backup Jobs keeps warnings with the selected day.** Open By date for that day’s restore points, or clear the date to see undated entries. A running backup shows In progress in place of a size.
- **Backup Servers labels PBS listing counts.** Counts show how many entries Pulse listed, even when history is incomplete. Unread counts show as unavailable.
- **Backup Servers flags stale readings and job failures.** Retained capacity and forecasts are marked stale, failed datastore jobs affect server status, and datastore details flag unread capacity.
- **Coverage Help explains the fixed PBS verification window.** Direct PBS expects a verified point within seven days, even on datastores without a verify job. Use By date to check when backups arrived.
- **Proxmox Nodes shows reading status beside each name.** Stale readings, API errors and degraded connections have distinct labels. Trends uses the recorded node series when another resource shares its name or ID.
- **Replication search keeps same-numbered jobs separate.** Results identify the guest and connection. Typing keeps the search broad until you select a specific job.
- **Cluster refreshes respect saved edits.** Member checks keep the address they actually tested. A canceled discovery leaves saved settings alone, overlapping edits are preserved, and Pulse reports a saved refresh that failed to apply.
- **Backup Help explains what each connection can see.** Proxmox can report storage and tasks without access to list archives. Direct PBS can list its own snapshots. Check the source system for anything outside Pulse’s access.

## TrueNAS, vSphere and Docker

- **TrueNAS Storage counts the rows that match.** Status and storage-type totals include matching datasets beneath a degraded pool.
- **Docker Applications can search and filter Compose apps.** Find apps by name or health, open their details and spot failed HTTP checks. Hosts and Containers have clearer names, image labels and action columns.
- **Docker tabs use filters that fit their data.** Images can show updates, Storage can show active or reclaimable items, and Networks can show networks in use. Readiness displays reuse inventory when listing many runtimes.
- **Proxmox and vSphere keep partial workload results visible.** Readable workloads and search results stay beside a warning that names the connection with missing inventory.
- **vSphere is being phased out.** The early-access integration remains available in this release. The supported-platform list in the README has dropped it.

## Install, updates and agents

- **Windows agent updates fix one file-in-use failure.** The updater finishes saving the staged file without reopening it. If an Agent Doctor repair fails on certificates, use a Pulse HTTPS address whose certificate Windows trusts.
- **Manual updates keep the selected agent’s Proxmox setup.** They use that installation’s saved choice. If you deliberately opted out with `--disable-proxmox`, include it again with `--update`.
- **Agent Doctor shows repair details for the affected install.** See reported and target versions, OS and profile. After generating a token, copy the repair command from that agent’s details.
- **Install Pulse Agent puts token and command steps in reach.** Standalone agent management opens the installer for the detected profile and shows uninstall commands for the detected OS.
- **Pulse detects memory limits in more Linux containers.** Delegated and inherited cgroup mounts can supply the configured cap, including in Proxmox LXCs.
- **Install Help lists server resource needs.** Start with 1 to 2 vCPUs, 1 GiB of RAM and 4 GiB of disk, with at least 1.2 GiB free for an in-place upgrade.
- **Update Help shows how to confirm monitoring resumed.** Check each connection for a new poll after restart. For agents, look for a fresh reading alongside an advancing Last seen time.

## Pulse Pro, AI and hosted

- **Background Patrol proposals stay open for 24 hours.** New and refreshed plans give you more time to review them. Pulse checks current conditions and permissions before running a fix.
- **Patrol gives slow summaries more time.** Final responses use the investigation’s timeout for local and OpenAI-compatible providers. Overdue runs are marked timed out.
- **Patrol Inbox can load findings beyond the first 50.** Counts agree with pagination, the weekly digest reflects the active automation mode, and Watch only describes reviewing issues.
- **Assistant and Patrol respect session and tool permissions.** Only authorized users can open sessions, steer findings or see raw tool output. File, URL and command reads have tighter limits.
- **AI provider keys stay with their saved server.** Changing the server address requires credentials for the new destination. Provider errors and tool results expose less sensitive data.
- **Hosted access follows workspace membership.** Promotions, demotions and removals sync to the workspace. An old sign-in link can no longer restore access for a removed member.

## Monitoring and service health

- **DNS checks can run from remote agents with Pulse Pro.** Choose an observation location to test its network resolver. Results distinguish a wrong or failed answer from a resolver the agent cannot reach.
- **HTTPS checks offer certificate controls.** Pin a certificate fingerprint or explicitly allow an unverified certificate in the check editor. A failed test can show the fingerprint presented by the server.
- **Deleted Availability checks leave the poll queue.** Removing a check drops pending work and prevents a poll already in progress from scheduling it again.
- **Proxmox connection reads no longer block polling and agent reports.** A lock conflict could stop Proxmox VE, PBS and Mail Gateway readings while the dashboard still answered.
- **Scheduled polls keep their turn.** Due work stays eligible across planning ticks, earlier work wakes idle workers, slow polls hold one slot, and workers survive a poll crash.
- **Retries respect connection backoff.** A later successful poll clears the old terminal failure. Any readings that still failed remain visible as a partial error.
- **Scheduler Health shows how long the oldest queued poll is overdue.** Its Help explains how to compare saved results by connection, including fixed-interval polling, partial reads and scheduled retries.
- **Prometheus shows aging polls and retries between results.** Scrapes track overdue polls waiting in the queue, overdue polls held by workers and terminal failures separately. Help covers missing poll successes and stalled agent reports.
- **External watchdog signals continue when local alerts are blocked.** Notifications distinguishes an old heartbeat, an overdue or failed status read, and a monitoring stall. Pulse drops a queued stall alert once monitoring recovers.
- **The dashboard waits for its first readings.** It shows loading while the first snapshot arrives, even if the live connection is already open.
- **Mail Gateway flags stale mail and queue readings.** Stale gateways leave Healthy. The Mail tab loads in place, draws the spam histogram, explains queue and quarantine counts and shows why a gateway is degraded.
- **Large inventories create fewer temporary copies.** Proxmox guest and backup polls, PBS snapshot matching, Recovery, agent liveness checks and live broadcasts use less temporary memory.

## Security

- **Proxmox VE 9+ setup stops granting guest file reads by default.** Setup Doctor can remove the old grant from PulseMonitor. Use ballooning or an in-guest agent for Linux memory detail. Denied file reads wait longer before retrying.
- **Proxmox and PBS setup protect saved connections.** Discovery respects saved certificate pins and credential scope. Proxmox address changes require credentials for the new endpoint.
- **Configuration imports respect workspace permissions.** Scoped imports cannot replace protected server credentials or tokens. Imported destinations go through the same checks as saved settings.
- **Monitoring-only agents retain their command limits.** Windows service startup, Docker updates and agent settings enforce them. Host, Docker, Kubernetes and Ceph reports stay tied to their owning tokens and workspaces.
- **Remote actions stay on the authorized resource.** Agent sessions check token ownership, host and guest actions stay on target, and log reads stay within allowed paths.
- **Sessions and sensitive API changes enforce current access.** Credential rotation ends admin sessions, sensitive requests recheck access before saving, and SSH setup requires admin settings access.
- **SSO binds identities and roles to the right provider.** Changed group mappings remove old grants. An explicitly unverified OIDC email cannot satisfy an email or domain restriction.
- **Oversized requests and invalid readings are bounded.** Limits protect Pulse from large reports and platform inventories, including TrueNAS and vSphere. Metrics, backup alerts and recovery history stay scoped to their source.
- **Notification secrets stay with their saved destinations.** SMTP, Webhook and Apprise validate destination changes and reject malformed settings. Outbound checks block disallowed targets.
- **Installers enforce pinned server certificates throughout downloads.** Least-privilege Linux installs keep the agent executable root-owned and give the service account ownership of its state.
- **Hosted audit records use the signed-in actor.** Pulse takes the actor from the authenticated session and limits oversized entries.

## Other improvements

- **Kubernetes Review opens workloads that need attention.** It keeps cluster and namespace filters. Workloads adds Kind, and Workloads, Services and Configuration hide empty tables while filters are active.
- **Kubernetes tables use resource-specific filters.** Events, Services, Storage and Configuration show relevant categories. Metadata-only ConfigMaps and Secrets collapse, and event messages have more space.
- **Infrastructure makes connected systems easier to find.** Discovery sits above the searchable list. Coverage includes Docker, Kubernetes and cluster host telemetry. Standalone rows link to Machines, and profiles group by OS.
- **Settings navigation is easier to use.** Search recognizes more terms, tabs open at the top, Diagnostics shows readable uptime, and System Logs can filter and wrap lines.
- **Docs is easier to browse.** Search the index, use the table of contents or choose Back to Pulse. Direct `.md` links open in the built-in reader.
- **Dashboard Search adds Actions, Docs, themes and kiosk mode.** G shortcuts keep working, and the same shortcut can close the dialog it opened. Navigation focus is easier to see.
- **Sign-in preserves your username after a failed attempt.** Failed session checks link to Help. SSO explains when it could not confirm a session, and Authentication shows how to reveal the hidden local form.
- **SSO setup keeps registration details available.** Copy the OIDC callback after saving or open a disabled SAML provider to copy its URLs. SAML Test flags missing readable certificates.
- **SSO validates group mappings and scopes before saving.** Directory group punctuation is preserved, duplicate or incomplete rules are flagged, and enabled Community OIDC providers require the `openid` scope.
- **API Access shows the last recorded token use before revocation.** If automatic copy fails, select the token in its reveal dialog. Audit Log shows the event outcome separately from its integrity check.
- **Workspace and role dialogs explain who gets access.** The picker shows the full workspace name, Sharing explains its audience, role deletion explains which grants or denies go away, and declining an invite asks for confirmation.
- **History Help explains where readings come from.** A device can supply chart points from during a Pulse outage. Check fresh live readings to confirm collection resumed. Remote-site Help also explains the private path Pulse needs to Proxmox and PBS.

## Known issues

- **Guest freezes and Windows guest restarts remain unresolved.** If you stopped monitoring after a guest froze, do not restart it solely to test this release or a beta.
- **Pause Pulse before freeze-enabled backups.** Confirm polling stopped. Even if the backup reports OK, verify thaw, fresh writes on every backed-up filesystem and workload health before resuming. Monitoring and alerts pause while Pulse is stopped.
- **Resume only services and timers that were active.** Avoid hooks that ignore a failed stop or resume after a failed or unanswered thaw check. On Kubernetes, confirm persistent storage and prevent controllers from replacing stopped Pulse pods.
- **Guests can freeze without a VM lock.** Cloning a running VM or importing disks can freeze it. Reads already in progress can interfere with thaw. Avoid manual guest-agent probes during freeze or thaw.
- **Some Windows guests can still lack Proxmox disk data.** Use working Machines readings or check in Windows. Keep existing agents and avoid guest restarts or forced probes just to recover the reading.
- **Linux guest reads may show Deferred when file commands are disabled.** Leave deliberately disabled commands off. Use ballooning or an existing in-guest agent for memory readings.
- **High server memory and out-of-memory stops are still reported.** Keep the rollout staged if pressure rises or readings stop. Compare LXC memory and swap at similar uptime, workload and limits, and keep readings separate across restarts or counter resets.
- **Use existing logs after a memory stop or host lockup.** Check the Proxmox host kernel and Pulse logs privately. Leave affected agents or Patrol off if you paused them. Avoid restarts, heap dumps or changes in GPU load just to reproduce the problem.
- **Check agent and helper versions after updates.** Use installer maintenance if you need a helper fix. If an ARMv6 agent offers ARMv7, update manually for ARMv6 and keep its ID and enrollment.
- **Earlier Alert History gaps cannot be recreated.** Keep existing logs and delivery records after a storage failure, even after the history store recovers.
- **TrueNAS CORE can show blank pool capacity or Disk I/O.** Use TrueNAS Storage for pool capacity and TrueNAS Reporting for host Disk I/O. Keep the existing API key and permissions.
- **A resolved webhook can carry a lower severity than the firing webhook.** If an automation closes notifications, match on the alert ID instead of severity.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.9.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.9.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.9.0` and recreate the container.
