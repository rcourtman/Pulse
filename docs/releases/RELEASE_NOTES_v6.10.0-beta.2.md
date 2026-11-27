# Pulse v6.10.0-beta.2 Release Notes

This beta follows `v6.10.0-beta.1` on the way to stable `v6.10.0`. Stable `v6.9.0` remains the rollback target.

Pulse adds a dedicated Availability page for service and DNS checks. Proxmox VE 9.3 guest charts keep their history through live migrations. Backup and platform views make it easier to see which readings are current and which sources Pulse could only partly read.

## Highlights

- Availability in the main navigation lets you add, edit, test, and pause service checks without leaving the page.
- Availability’s DNS resolution check queries Pi-hole and other resolvers directly and can verify an expected answer.
- Proxmox guest History keeps charts continuous when a VM or container live-migrates between nodes on Proxmox VE 9.3.

## What's improved

- **Availability charts outages and response times.** Open a check’s Overview for a status timeline and average, minimum, and maximum response times. The charts cover 24 hours, 7 days, 30 days, or 1 year.
- **Availability remembers Table or Fleet view.** Old Machines links redirect to the new page, which also opens with G then C.
- **What's New is easier to scan and reopen.** Expand longer sections as needed. Open it again from the footer version or Settings → System → Updates.
- **Map preview adds host and role layouts.** From the header’s Map button, search across the full inventory, filter for problems, inspect groups, and pan or zoom. Layout and density choices are saved in your browser.
- **Map preview shows live traffic and exposed services.** Inspect connected peers, busiest workloads, and busy or down links. Open Published & routed to find ports and routes from Docker, Swarm, TrueNAS, Kubernetes, Traefik, or Caddy.
- **Map preview puts host and storage health on the diagram.** Host frames show load, temperatures, and physical disks. Pool bars show capacity, rebuilds, and faults. Linux agents can report observed connections between monitored devices.
- **Machines → VMs adds a preview for standalone guests.** Browse reported Libvirt/KVM and XCP-ng guests by host, state, or name. Live updates preserve VM state and vCPU counts.
- **Resource details give charts and settings more room.** The time range sits beside the tabs, healthy resources skip the empty alerts area, and viewers can read saved Manage settings and open or copy web links.

## Alerts and notifications

- **Alerts Overview adds search and severity filters.** Thresholds shows when resources are loading and keeps Save Changes and Discard in view as you scroll.
- **PBS CPU alerts use the server’s own CPU history.** Both the live percentage and rolling averages use the correct scale and stay separate from linked agents and same-named servers.
- **Connection alerts stay in order under load.** Slow checks apply their results in order, and notification health checks can run while monitoring continues.
- **Resolved notifications keep the severity each destination received.** A critical alert that drops to warning before clearing can still send a critical recovery to the webhook that received it as critical.
- **A fresh successful send can recover recent delivery failures.** Eligible failures for the same saved destination can rejoin the queue, even after a restart. Deliveries older than 24 hours stay out of automatic replay.
- **Email and Webhook tests explain certificate failures.** Pulse stops retrying trust errors. Delivery, retry, and test records share a queue ID, and the delivery log keeps the recorded failure reason.
- **Notification settings are easier to scan.** Quiet Hours gives timezone and suppression settings more room, and Apprise test controls follow the same disabled-state behavior as Email.
- **Alerting Help explains how to trace an open alert.** Compare its live threshold summary with the saved message, then check delivery separately. Telegram Help covers live alerts that fail after a passing test.

## Disks and storage

- **Proxmox storage listings honor the configured timeout.** Slow backup mounts can take longer than the old 10-second cutoff. Pulse distinguishes a timeout from denied access.
- **Small Proxmox and PBS readings keep their scale.** Disk and memory percentages stay accurate in Resources and AI reads. Proxmox Nodes bars and Trends show small readings to two decimal places.
- **Proxmox Nodes exposes the fullest filesystem.** The compact disk bar uses the fullest reported filesystem so a nearly full mount stays visible.
- **Ceph shows unknown and stale health separately.** Both stay out of Healthy. The table keeps the last quorum, OSD, and pool readings visible, and stale values stay out of the live chart.
- **Physical Disks uses recognizable host names.** Disks show the matched node name, with the reported name on hover when it differs. Pool, array, and filesystem usage have clear labels.
- **Machines flags Unraid array faults on collapsed rows.** The row shows severity and a short reason, with the full explanation in the status tooltip and expanded details.

## Proxmox, PBS and backups

- **Backups keeps restore points from readable sources.** Retry the failed source without losing your inspection. Warnings name the configured reader and distinguish denied access, connection trouble, and checks still in progress.
- **Backup health shows which workloads count.** Checking and Not evaluated stay outside the rated total. A health exception adds a removable filter, and listed backup ages reflect the selected sources and locations.
- **Backup Jobs keeps warnings with the selected day.** Use By date for that day’s restore points or clear the date to see undated entries. Running backups show In progress in place of a size.
- **Backup Servers distinguishes listed snapshots from complete history.** Unread counts and datastore capacity show as unavailable, retained capacity and forecasts are marked stale, and failed datastore jobs affect server status.
- **PBS removes datastores after a complete empty inventory.** Timed-out reads leave prior data in place and report a failure. Coverage Help explains the seven-day verification window for direct PBS.
- **Proxmox Nodes shows reading status beside each name.** Stale readings, API errors, and degraded connections have distinct labels. Trends uses the recorded node series when another resource shares its name or ID.
- **Proxmox VE 9.3 keeps reused VM IDs and same-named clusters separate.** A new guest using an old ID stays separate from the previous guest’s readings.
- **Replication keeps same-numbered jobs separate and flags stale readings.** Search identifies the guest and connection. Stale jobs leave Healthy, and details keep the last reported result and poll time when available.
- **Cluster refreshes respect saved edits and paused connections.** Member checks show the address they tested. Canceled discovery leaves settings alone, and Pulse reports when a saved refresh failed to apply.
- **Proxmox reports failed connections at startup.** Infrastructure shows authentication and reachability failures instead of leaving the connection pending. Health starts with the first completed poll.
- **Mail Gateway recovers after startup outages.** Pulse retries failed connections automatically, flags stale mail and queue readings, and explains degraded states. The Mail tab loads in place and shows the spam histogram and count breakdowns.
- **Linux guest reads keep working when file commands are disabled.** Pulse skips or backs off the optional memory file read while continuing other guest-agent reads. Leave deliberately disabled commands off.

## TrueNAS, vSphere and Docker

- **TrueNAS labels retained readings as stale.** Systems, storage, apps, shares, and services keep their last values and any more serious failures visible. Details update when fresh readings arrive.
- **TrueNAS Protection flags stale snapshots and replication tasks.** The Stale filter and details show the last reported outcome. Stale tasks stay out of the Healthy and Running counts until readings resume.
- **TrueNAS Storage counts the rows that match.** Status and storage-type totals include matching datasets beneath a degraded pool.
- **TrueNAS refreshes have a time limit.** A timed-out partial refresh reports a failure, and a repaired connection can poll immediately. Late results from retired TrueNAS and vSphere connections can no longer bring them back.
- **vSphere flags stale hosts, networks, and datastore readings.** Datastores and Capacity History retain the last reading and show missing capacity as unknown. Existing outages and alarms stay visible.
- **Proxmox and vSphere keep partial workload results visible.** Readable workloads and search results stay alongside a warning that names the connection with missing inventory.
- **Docker Applications adds Compose search and health filters.** Open an app to see failed HTTP checks. Hosts and Containers also have clearer host names, image labels, and action columns.
- **Docker tabs use filters that match their data.** Images can show updates, Storage can show active or reclaimable items, and Networks can show networks in use.
- **Docker flags stale Compose, Swarm, image, storage, and network readings.** Tables and details keep last reported values visible without counting stale entries as current or healthy.

## Install, updates and agents

- **Windows agent updates fix one file-in-use failure.** The updater saves the download without reopening it. If Agent Doctor cannot download the repair script, use a Pulse HTTPS address with a certificate Windows trusts.
- **Manual updates keep the selected agent’s Proxmox setup.** The installer reads that installation’s saved choice. If you deliberately opted out, include `--disable-proxmox` again with `--update`.
- **Agent Doctor shows repair details for the affected installation.** See the reported and target versions, OS, and profile. Generate a token and copy the repair command from the agent’s details.
- **Install Pulse Agent opens the matching setup profile.** Standalone agent management opens the installer for the detected profile and shows uninstall commands for the detected OS. Token generation and the install command are easier to reach.
- **Pulse detects memory limits in more Linux containers.** It can read delegated and inherited limits, including on Proxmox LXC. Startup and troubleshooting Help explain which limit Pulse found.
- **Install Help lists server resource needs.** Start with 1 to 2 vCPUs, 1 GiB of RAM, and 4 GiB of disk, with at least 1.2 GiB free for an in-place upgrade.
- **Update Help shows how to confirm collection resumed.** Look for a new poll on each connection and a fresh reading alongside an advancing Last seen time for each agent.

## Pulse Pro, AI and hosted

- **Background Patrol proposals stay open for 24 hours.** New and refreshed plans give you more time to review them. Pulse checks current conditions and permissions before running a fix.
- **Patrol gives slow summaries more time.** Final responses use the investigation timeout for local and OpenAI-compatible providers. Overdue runs are marked timed out.
- **Patrol Inbox can load findings beyond the first 50.** Counts agree with pagination, the weekly digest reflects the active automation mode, and Watch only describes reviewing issues.
- **Assistant and Patrol enforce session and tool permissions.** Opening sessions, steering findings, and seeing raw tool output require the appropriate access. File, URL, and command reads have tighter limits.
- **AI provider keys stay with their saved server.** Changing the server address requires credentials for the new destination. Provider errors and tool results expose less sensitive data.
- **Hosted access follows workspace membership.** Promotions, demotions, and removals sync to the workspace. Old sign-in links cannot restore removed access, and hosted audit entries name the authenticated actor.

## Monitoring and service health

- **DNS checks can run from remote agents with Pulse Pro.** Choose an observation location to test its resolver. Results distinguish an unreachable resolver from a failed or unexpected DNS answer.
- **HTTPS checks offer certificate controls.** Pin a certificate fingerprint or explicitly allow an unverified certificate in the check editor. A failed test can show the fingerprint presented by the server.
- **Availability drops results from canceled or superseded checks.** Old results cannot revive deleted checks, overwrite edits, or add false outages on pause. Resuming a remote check starts a fresh reporting grace period.
- **Polling and agent reports keep moving during Proxmox connection reads.** The fix addresses stalls that could stop Proxmox VE, PBS, and Mail Gateway readings while the dashboard still answered.
- **Scheduled polls keep their turn.** Due work stays eligible, earlier work wakes idle workers, and a slow poll holds only one slot. Workers keep going after an individual poll crashes.
- **Poll retries follow connection backoff.** A successful poll clears the old terminal failure. Any reads that still failed remain visible as partial results.
- **Scheduler Health shows overdue work and retry context.** Compare results by connection, see how long the oldest queued poll is overdue, and identify partial reads, errors, and scheduled retries.
- **Prometheus separates Pulse uptime from collection progress.** Poll metrics distinguish overdue queued and running work, retries, and terminal failures. Agent and Docker report metrics help show whether requests are completing.
- **External watchdog reports which part failed.** Heartbeat delivery continues if local alerts are blocked. Status distinguishes old heartbeats, failed reads, and stalled monitoring, clears obsolete alerts on recovery, and resets when you change destinations.
- **The dashboard waits for its first readings.** It shows loading until the first snapshot arrives, even if the live connection is already open.
- **Large inventories use less temporary memory.** Proxmox and PBS polling, Recovery, agent liveness checks, scheduler diagnostics, and live broadcasts create fewer temporary copies.

## Security

- **Proxmox VE 9+ setup stops granting guest file reads by default.** Setup Doctor can remove the old PulseMonitor grant. Use ballooning or an existing in-guest agent for Linux memory detail if needed.
- **Proxmox and PBS setup protects saved connections.** Discovery and automatic registration respect saved certificates, token scope, and cluster boundaries. Proxmox address changes require credentials for the new endpoint.
- **Configuration imports respect workspace permissions.** Scoped imports cannot replace protected server credentials or tokens. Imported destinations go through the same checks as saved settings.
- **Monitoring-only agents retain their command limits.** Windows service startup, Docker updates, and remote settings enforce them. Host, Docker, Kubernetes, and Ceph reports stay tied to their owning tokens and workspaces.
- **Remote actions stay on the authorized resource.** Agent sessions check current token ownership, host and guest actions stay on target, and log reads stay within allowed paths.
- **Sensitive settings changes enforce current access.** Rotating admin credentials ends existing admin sessions. Sensitive requests recheck access before saving, and SSH setup requires admin settings access.
- **SSO binds accounts and roles to the right provider.** Changed or deleted group mappings revoke their old grants. An explicitly unverified OIDC email cannot satisfy an email or domain restriction.
- **Notification credentials stay with the saved destination.** SMTP, Webhook, and Apprise enforce destination checks. Webhooks refuse redirects to a different origin without forwarding headers or alert data.
- **Installers preserve server pins and Linux file ownership.** Pinned certificates apply throughout downloads. Least-privilege installs keep the executable root-owned and give the service account ownership of its state.
- **Large requests and invalid readings have limits.** Pulse bounds agent reports, platform inventories, and history queries. Backup alerts, metrics, and recovery records stay scoped to their source.

## Other improvements

- **Kubernetes Review opens workloads that need attention.** Cluster and namespace filters carry over. Workloads adds Kind, and filtered Workloads, Services, and Configuration hide empty tables.
- **Kubernetes tables show relevant filters and stale readings.** Events have more room for messages, routine metadata-only ConfigMaps and Secrets collapse, and retained cluster, node, and workload readings are flagged as stale.
- **Infrastructure makes connected systems easier to find.** Discovery sits above the searchable list. Coverage includes Docker, Kubernetes, and cluster host telemetry. The Proxmox editor puts the cluster name and Pulse Agent options near the saved settings.
- **Settings navigation is easier to use.** Search recognizes more terms, tabs open at the top, Diagnostics shows readable uptime, and System Logs can filter and wrap long lines.
- **Docs is easier to browse.** Search the index, use the table of contents, or choose Back to Pulse. Direct `.md` links open in the built-in reader.
- **Dashboard Search adds Actions, Docs, themes, and kiosk mode.** Use Cmd/Ctrl+K to open or close Search. G shortcuts keep working, and navigation focus is easier to see.
- **Sign-in preserves your username after a failed attempt.** An unconfirmed SSO return links to troubleshooting, and Authentication explains how to reveal the hidden local form.
- **SSO keeps setup URLs available and checks settings before saving.** Copy the OIDC callback after saving or open a disabled SAML provider for its URLs. SAML Test flags unreadable certificates, and group mappings preserve directory punctuation.
- **Community OIDC requires `openid` for enabled providers.** Pulse checks new or changed scope selections and flags duplicate or incomplete group mappings before saving.
- **API Access shows the last recorded token use before revocation.** If automatic copy fails, select the token in its reveal dialog. Audit Log shows the event outcome separately from its integrity check.
- **Workspace and role dialogs explain who gets access.** The picker shows the full workspace name, Sharing explains its audience, role deletion explains which rules it removes, and declining an invite asks for confirmation.
- **History Help explains readings that arrive after an outage.** A device can supply older chart points after reconnecting. Check fresh live readings to confirm collection resumed.

## Known issues

- **Guest freezes and Windows guest restarts remain unresolved.** If you stopped monitoring after a guest froze, do not restart it solely to test this release or a beta.
- **Stop Pulse before freeze-enabled backups and confirm polling stopped.** An OK backup is insufficient. Verify thaw, fresh writes on every backed-up filesystem, and workload health before resuming. Monitoring and alerts pause while Pulse is stopped.
- **Resume only the services and timers that were active.** Avoid hooks that ignore failed stops or resume after failed or unanswered thaw checks. On Kubernetes, confirm persistent storage and prevent controllers from replacing stopped Pulse pods.
- **Guests can freeze without a VM lock.** Cloning a running VM, importing disks, or replication can freeze a guest. A read already in progress can interfere with thaw. Avoid manual guest-agent probes during freeze or thaw.
- **Some Windows guests may still lack Proxmox disk readings.** Use the working Machines view or check in Windows. Keep existing agents and avoid guest restarts or forced probes just to recover the reading.
- **High server memory use and out-of-memory stops are still reported.** Keep rollout staged if pressure rises or readings stop. Compare LXC memory and swap at similar uptime, workload, and limits. Keep samples separate across restarts or counter resets.
- **Use existing logs after a memory stop or host lockup.** Check the Proxmox host kernel and Pulse logs privately. Leave affected agents or Patrol off if already paused. Avoid restarts, heap dumps, or GPU load changes just to reproduce it.
- **Check the running agent and helper versions after updating.** Use installer maintenance if the helper stayed old. If an ARMv6 agent offers ARMv7, update manually for ARMv6 and preserve its ID and enrollment.
- **Earlier Alert History gaps cannot be recreated.** Keep existing logs and delivery records after a storage failure, even after the history store recovers.
- **TrueNAS CORE can still have missing pool capacity or Disk I/O.** Check the pool in TrueNAS Storage and host Disk I/O in TrueNAS Reporting. Keep the existing API key and permissions.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.9.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.9.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.9.0` and recreate the container.
