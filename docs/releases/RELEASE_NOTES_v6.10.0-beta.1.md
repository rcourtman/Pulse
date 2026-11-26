# Pulse v6.10.0-beta.1 Release Notes

This beta opens the `v6.10.0` candidate line after stable `v6.9.0`, which remains the rollback target.

Pulse v6.10.0 gives service checks their own page, adds DNS monitoring and supports Proxmox VE 9.3 guest identity. It keeps readable backups visible through partial outages and gives you clearer signs when polling falls behind.

## Highlights

- Availability is now a top-level page where you can add, edit, test and pause checks.
- Availability check setup now queries DNS resolvers directly and can check for an expected answer.
- Proxmox VE 9.3 monitoring keeps guest charts through live migration and separates reused VMIDs and same-named clusters.

## What's improved

- **Availability check details chart outages and response times.** Open Overview for a status timeline and a response-time chart with averages and ranges. Choose 24 hours, 7 days, 30 days or 1 year.
- **Availability remembers your layout.** Table and Fleet selections persist, and old Machines links open the new page. Open Availability from Search or with G then C.
- **What's New is easier to scan and reopen.** Longer sections expand on demand. Reopen it from the footer version or Settings → System → Updates.
- **Infrastructure Map is available in preview builds.** Search all resources, filter for problems, open groups and networks, or pan and zoom. If the map fails to load, use Try again.
- **Machines → VMs is available in preview builds.** Browse reported Libvirt/KVM and XCP-ng guests by host, state or name. Live updates preserve guest state and vCPU counts.
- **Infrastructure's Proxmox editor puts saved settings first.** Connected clusters have a Cluster Name field, healthy members and setup scripts fold away, and agent options sit above Save changes.
- **Resource details put the time range beside the tabs.** Empty alert space is removed, repeated status text appears once, and viewers can read saved Manage settings without edit controls.

## Alerts and notifications

- **Alerts Overview can search and filter by severity.** Thresholds shows a loading state and keeps Save Changes and Discard in view as you scroll.
- **PBS CPU alerts use the displayed percentage.** Rolling alerts use the backup server's own samples, so a linked host agent or same-named server can't supply the wrong readings.
- **Platform connection alerts stay in order under load.** A delayed check can no longer apply its result after a newer one. Notification health checks also run without holding up monitoring.
- **A recovered destination can retry recent missed alerts.** A fresh successful send can reopen eligible failures for the same saved destination, including after a restart.
- **Certificate failures stop retrying.** Email and Webhook tests point to certificate trust problems and give guidance specific to those destinations.
- **Delivery diagnostics keep attempts and queued items easy to match.** Queue IDs link delivery, retry and rehearsal records. Look in the delivery log for the recorded failure reason.
- **Notification controls are easier to read.** Quiet Hours gives timezone and suppression choices more room, and disabled Apprise test controls match Email.
- **Telegram Help covers failures after a passing test.** Troubleshooting explains what to check when the test works but a real alert fails to send.

## Disks and storage

- **Slow Proxmox storage responses get the configured time.** Backup listings on slow mounts can wait past the old 10-second cutoff. Timeouts are reported separately from permission denials.
- **Small Proxmox and PBS readings keep their scale.** Disk and memory values keep their actual percentages in Resources and AI reads. Proxmox Nodes bars and Trends show small readings to two decimal places.
- **Ceph shows Unknown when it can't report health.** Unknown health stays out of Healthy. The table also shows quorum and summarizes running services.
- **Physical Disks makes host names easier to recognize.** Disks use the matched node name, with the reported name on hover when it differs. Pool, array or filesystem usage has its own label.
- **Unraid array faults appear on the collapsed Machines row.** See the severity and short reason for that array. The status tooltip and expanded details carry the full explanation.

## Proxmox, PBS and backups

- **Proxmox Backups keeps data from the source that answered.** Retry only the failed read and keep your place. Until the guest list loads, restore points stay visible and guest matches stay unchecked.
- **Backups identifies the connection that needs attention.** It names the saved Proxmox or direct PBS connection and distinguishes an access denial from a connection problem, even when another connection succeeds.
- **Backup health labels the rated total.** Checking and Not evaluated stay visible outside the protected ratio, so a fully protected rated group can't give the whole page a green all-clear.
- **Coverage labels the age of the newest listed point.** Partial reads and location filters can narrow the result. Protection status has its own retry when the backup inventory is already visible.
- **Backup Jobs keeps named warnings beside the selected day.** Jump to By date to see that day's restore points. Clear the day to see undated entries again. Running backups show In progress in place of a size.
- **PBS counts show how many backup entries Pulse listed.** Backup Servers keeps counts visible during incomplete history and labels the affected connection. An unread count shows as unavailable.
- **Backup Servers flags stale readings and job failures.** Retained capacity and forecasts are labeled stale. Failed datastore jobs affect the server status, and unread capacity is flagged in the datastore details.
- **Coverage Help explains Not verified recently.** Direct PBS expects a verified backup point at most seven days old, even without a verify job. The window is fixed. Use By date to check backup arrivals separately.
- **Proxmox Nodes shows the reading status beside the name.** An old feed, API error and degraded connection get distinct labels. Trends uses the recorded node series when another resource shares its name or ID.
- **Replication search keeps same-numbered jobs separate.** Results identify the guest and connection. Typing stays broad until you select a specific job.
- **Backup Help explains what each connection can read.** Proxmox can report storage and tasks without listing archives. Direct PBS can list its own snapshots. Check the source system for anything outside Pulse's access.

## TrueNAS, vSphere and Docker

- **TrueNAS Storage counts only matching rows.** Status and storage-type totals reflect the rows that match, including healthy datasets beneath a degraded pool.
- **Docker Applications is searchable and filterable.** Find Compose apps by name or health, open their details, and spot failing HTTP checks. Hosts and containers also have clearer names, image labels and action columns.
- **Docker Images, Storage and Networks use their own filters.** Find images with updates, active or reclaimable storage, and networks in use. Capability displays do less repeated work when listing many runtimes.
- **vSphere is being phased out.** The early-access integration is still present in this release. The README's supported platform list has dropped it.
- **Proxmox and vSphere show partial workload results.** Readable workloads and search results stay visible beside a warning that identifies the connection with missing inventory.

## Install, updates and agents

- **Windows agent updates avoid reopening the staged download.** This fixes one source of file-in-use update errors. Agent Doctor Help gives separate steps for staging failures and PowerShell certificate errors.
- **Manual updates keep the selected agent's Proxmox setup.** Updates retain its saved local setup choice. If you deliberately opted out with `--disable-proxmox`, include that flag again with `--update`.
- **Agent Doctor puts repair details with the affected installation.** See reported and target versions, OS and profile, and find the copyable command in the agent's details after generating a token.
- **Install Pulse Agent puts token and command steps in reach.** Standalone agent management opens the installer for the detected profile and shows uninstall commands for the detected OS.
- **Installers enforce pinned server certificates throughout downloads.** Least-privilege Linux installs keep the agent executable root-owned while the service account owns its state.
- **Pulse detects memory limits in more Linux containers.** Delegated and inherited cgroup mounts can supply the actual cap, including in Proxmox LXCs.
- **Install Help lists server resource needs.** The suggested baseline is 1 to 2 vCPUs, 1 GiB RAM and 4 GiB disk, with at least 1.2 GiB free for in-place upgrades.
- **Update Help shows how to check monitoring resumed.** Look for a History point from an ordinary poll after the restart. Startup and Troubleshooting point to the Proxmox host kernel log after an LXC memory stop.

## Pulse Pro, AI and hosted

- **Patrol action proposals stay open for 24 hours.** Background proposals and refreshed plans give you time to review them. Pulse checks current conditions again before running a fix.
- **Patrol gives slow summaries more time.** Final responses use the investigation's timeout, including for local and OpenAI-compatible providers. Overdue runs are reported as timed out.
- **Patrol Inbox can load findings beyond the first 50.** Counts and pagination agree, the weekly digest uses the effective automation mode, and Watch only refers to reviewing issues.
- **AI sessions and tools respect access boundaries.** Assistant sessions belong to their owners, Patrol limits who can see raw tool output or steer a finding, and file, URL and command reads enforce tighter limits.
- **AI provider credentials stay with their saved destination.** Changing the server URL can't silently reuse its saved key. Provider errors and tool results disclose less sensitive data.
- **Hosted workspace access follows membership changes.** Promotions, demotions and removals sync to the workspace. A sign-in link can't restore access for a removed member.

## Monitoring and service health

- **DNS checks can run from remote agents with Pulse Pro.** Choose an observation location to check that network's resolver. Results distinguish DNS errors and wrong answers from a resolver you can't reach.
- **HTTPS checks offer certificate controls.** In the check editor, pin a certificate fingerprint or explicitly allow an unverified certificate. Failed tests can show the fingerprint the server presented.
- **Proxmox connection reads no longer stall polling.** A lock conflict could freeze PVE, PBS and Mail Gateway readings and block agent reports while the web server still answered.
- **Scheduled polls keep their turn.** Earlier work wakes idle workers, slow connections can't claim a second slot, and a task that crashes no longer takes its worker out of service.
- **Poll retries wait until the connection can be tried again.** Retries respect the connection's backoff. A later success clears old terminal failures, and a partial read still shows which data is missing.
- **Scheduler Health shows how long the oldest queued poll is overdue.** Its Help compares saved results by connection, including fixed-interval polling, partial readings, and scheduled retries versus past failures.
- **Prometheus shows poll progress between results.** Poll age grows between scrapes, and separate gauges track queued polls, overdue polls held by workers and terminal failures. Help shows how to spot missing polls and stalled agent reports.
- **External watchdog signals continue when local alerts are blocked.** Diagnostics distinguishes stale readings from a failed status request or confirmed stall. An alert queued before recovery is dropped.
- **The dashboard shows loading until its first readings arrive.** This applies even when the live connection is already open.
- **Mail Gateway labels old mail and queue readings.** Stale gateways leave the Healthy filter. The Mail tab loads in place, draws the spam histogram and shows counts and reasons for degraded status that agree with the table.
- **Large inventories use less temporary memory.** Pulse copies less data during Proxmox guest, storage and backup polls, PBS snapshot matching, Recovery, agent liveness checks and live dashboard broadcasts.
- **Help explains how to check a collection gap.** Compare live readings, Trends, saved History and agent report times. For remote sites, the Pulse server still needs a private path to the Proxmox and PBS APIs.

## Security

- **Proxmox VE 9+ setup drops automatic guest file-read access.** Use Setup Doctor to remove old grants from PulseMonitor. For Linux guest memory detail, use ballooning or an in-guest agent. Denied file reads back off longer.
- **Proxmox registration protects saved endpoints.** Cluster discovery honors existing TLS pins and keeps tokens within their authorized endpoints. Address changes require credentials for the new target.
- **Configuration imports respect workspace permissions.** Scoped imports can't replace protected server credentials or tokens. Imported destinations follow the same outbound checks as saved settings.
- **Monitoring-only agents stay monitoring-only.** Windows service startup, Docker updates and agent settings enforce the limit. Host, Docker, Kubernetes and Ceph reports stay tied to their owning tokens and workspaces.
- **Remote actions stay on the authorized resource.** Live agent sessions check token ownership, host and guest actions keep their target, and log reads stay within allowed paths.
- **Sessions and API changes enforce current access.** Credential changes invalidate admin sessions. Requests recheck access before applying sensitive changes, and SSH setup requires admin settings access.
- **SSO keeps identities and mapped roles separate.** Issuer identity stays tied to the provider, changed mappings remove old grants, and an explicitly unverified OIDC email can't satisfy an email or domain rule.
- **Requests and reports have tighter size limits.** Pulse bounds large API bodies, agent reports, platform inventory and recovery reads to protect memory. Metrics and histories stay scoped to their resource and workspace.
- **Saved notification secrets stay with their destination.** SMTP, webhook and Apprise settings check destination changes and reject malformed inputs. Outbound checks also reject disallowed local targets.
- **Cloud audit records use the authenticated actor.** Hosted audits take the actor from the signed-in session and limit oversized entries.

## Other improvements

- **Kubernetes Review opens the workloads that need attention.** It keeps cluster and namespace filters. Workloads adds Kind, and Workloads, Services and Configuration hide empty tables while filters are active.
- **Kubernetes tables use resource-specific filters.** Events, Services, Storage and Configuration show relevant categories. Metadata-only ConfigMaps and Secrets collapse, and event messages have more space.
- **Infrastructure puts Discovery above the connected systems.** Search the list and see Docker, Kubernetes and cluster host telemetry in Coverage. Standalone rows link to Machines, and agent profiles group by OS.
- **Proxmox connection edits preserve the last checked address.** Member checks keep their original target until a fresh read confirms the saved address. Overlapping edits and refreshes preserve saved changes, and failed reloads are reported.
- **Settings search recognizes more common terms.** Tabs open at the top, Diagnostics shows readable uptime, and System Logs can filter lines and wrap long entries.
- **Docs adds index search, a table of contents and Back to Pulse.** Direct links ending in `.md` open in the built-in reader.
- **Dashboard Search adds destinations and display commands.** Find Actions, Docs, themes and kiosk mode from the header. Search leaves G shortcuts alone, and shortcut keys can close the dialog they opened.
- **Sign-in keeps your username after a failed attempt.** A failed session check links to Help. SSO return errors say when the session is unconfirmed, and Authentication explains how to reveal the hidden local form.
- **SSO setup keeps the saved registration details in reach.** Copy the OIDC callback after saving, or open a disabled SAML provider to copy its URLs. SAML Test flags missing readable certificates.
- **SSO validates group rules and OIDC scopes before saving.** Group names keep their punctuation, duplicate or incomplete mappings are flagged, and an enabled Community OIDC provider needs the `openid` scope.
- **API Access shows the last recorded token use before revocation.** If automatic copying fails, select the token in the reveal dialog. Audit Log shows event outcomes and audit check results separately.
- **Workspace and role dialogs explain who gets access.** The picker shows the full workspace name. Sharing explains its audience and keeps focus on refresh, role deletion explains lost grants or denies, and declining an invite asks for confirmation.

## Known issues

- **Guest freezes and Windows restarts remain unresolved.** If you stopped monitoring after a guest froze, do not restart it solely to test this beta or release.
- **Pause Pulse server monitoring before freeze-enabled backups.** Even if the backup reports OK, independently confirm thaw, fresh writes on every backed-up filesystem and a running workload before resuming.
- **A guest can freeze without a VM lock.** Cloning a running VM or importing its disks can do this, and a read already in progress can interfere with thaw. Avoid manual guest-agent probes during freeze or thaw.
- **Resume only services and timers that were active before the pause.** Avoid hooks that ignore a failed stop or restart Pulse after a failed or unanswered thaw check. Monitoring and alerts pause too.
- **Kubernetes backup pauses need storage and controller checks.** Confirm persistent Pulse storage, prevent controllers from replacing stopped pods and check that polling stopped before the backup.
- **Some Windows guests can still lack Proxmox disk data.** Use working Machines readings or check in Windows. Keep existing agents and avoid guest restarts or forced guest-agent probes just to recover the reading.
- **High server memory and out-of-memory stops are still reported.** Keep the rollout staged if memory pressure rises or readings stop. Compare existing LXC memory and swap readings at similar uptime, workload and limits.
- **Keep memory readings separate across restarts or counter resets.** Use existing Proxmox kernel and Pulse logs after a stop. Avoid restarting Pulse, taking heap dumps or adding load just to investigate.
- **Keep affected monitoring paused after a host lockup.** Leave agents or Patrol off if you disabled them. Save existing evidence privately and avoid changing GPU load or hardware to recreate the lockup.
- **Check the running agent and helper after an automatic update.** Use installer maintenance if the helper stayed old. If an ARMv6 agent offers ARMv7, update manually for ARMv6 and keep its ID and enrollment.
- **Earlier Alert History gaps can't be recreated.** Keep existing logs and delivery records after a storage failure, even after the history store works again.
- **An online TrueNAS CORE pool can still lack capacity readings.** If Usage / Size shows a dash, check the pool in TrueNAS Storage. Keep the existing API key and permissions and use the pool's figures.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.9.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.9.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.9.0` and recreate the container.
