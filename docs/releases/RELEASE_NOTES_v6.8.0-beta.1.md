# Pulse v6.8.0-beta.1 Release Notes

This beta opens the `v6.8.0` candidate line after stable `v6.7.0`, which remains the rollback target.

Pulse v6.8.0 makes backup health easier to inspect, adds individual temperature limits and improves notification recovery. It also reduces guest-agent reads, keeps separate installations from mixing readings or commands, and gives clearer recovery steps when settings or installation changes cannot be confirmed.

## Highlights

- Backups → Backup health now groups protection problems by cause and opens Coverage filtered to affected workloads.
- Alerts → Thresholds → Machine Sensors now lets you set a separate temperature limit for each Linux sensor.
- Backups → Backup servers now shows datastore fill forecasts when a reliable growth trend is available.

## What's improved

- **Latest readings stay visible.** Resource drawers retain available CPU, memory and other readings when history fails or exceeds your plan. Older readings remain labelled.
- **Settings recovers from failed access checks.** Restricted panels clear outdated controls and offer retry instead of retaining stale permissions.
- **Infrastructure failures explain recovery.** Failed connection-list and saved-settings reads offer retry instead of showing first-time setup or incomplete editors.
- **Role edits preserve saved choices.** Roles retains permissions outside the shortcut list, inheritance and priority. Drafts survive unchanged access refreshes.
- **Access forms stay usable.** Roles and User Access keep save controls and errors reachable in short windows. User searches distinguish filtered results from an empty directory.
- **Uncertain access changes prompt a check.** Roles and User Access ask you to read the saved state before another edit when a change cannot be confirmed.
- **Docs explains loading failures.** Missing guides, refused access and connection failures have distinct messages. Retry keeps the selected guide, and repeated heading links restore keyboard focus.
- **French is available.** Supported interface areas and the getting-started guide now include French.
- **Backups opens on Coverage.** Health, backup servers and restore points follow that order. The restore-points-per-day chart is removed, and bars use the dashboard's usual metric colours.
- **Needs attention uses the drawer width.** Alert details fill available space and wrap into readable rows.
- **Missing organization access has a recovery screen.** Users can answer invitations, check access again or sign out. Unread invitations show a separate failure with retry instead of appearing absent.
- **Concurrent settings saves keep their changes.** System settings no longer lose unrelated fields from overlapping saves. New workspaces pick up current instance settings while they start.

## Alerts and notifications

- **Queued notifications survive graceful restarts.** Pending deliveries and buffered alert groups retain their work and grouping deadline. Sending resumes after destinations, quiet hours and delivery settings load.
- **Restarting preserves notification cooldowns.** Previously delivered active alerts avoid duplicate firing messages within their cooldown. New occurrences and increased severity can still notify.
- **Long-running alerts retain recovery messages.** Delivery-record cleanup preserves recoveries still owed for active alerts and pending deliveries.
- **Alert history rebuilds more reliably.** Late-written lifecycle events are included when rebuilding alert views after restart, without sending old notifications again.
- **Acknowledgements avoid duplicate requests.** Individual, group and bulk actions share pending state across navigation. Partial failures report successful and failed counts separately.
- **Active-alert dots return to Thresholds.** Matching open alerts mark metric rows, including machine filesystems and disk temperatures. Help explains that a dot does not confirm notification delivery.
- **Threshold notes stay saved.** Reopening an override restores its note, and edits that change only the note refresh correctly.
- **Alert settings follow current access.** Thresholds, destinations, delivery history and alert controls discard replies from earlier accounts or workspaces. Failed destination reads offer retry.
- **Delivery errors stay readable.** Recent delivery activity preserves ordinary error messages while hiding credentials in webhook URLs.
- **Failed PBS maintenance jobs raise datastore alerts.** Sync, verify, prune and garbage-collection failures appear separately from guest protection. Known failures survive outages, partial reads, running retries and Pulse restarts.

## Disks and storage

- **Full disks remain visible in summaries.** Compact Proxmox node rows show the fullest measured disk even when another disk lacks a reading. Wide machine rows retain individual disk bars.
- **Disk controls are easier to read.** Disk, alert and view toggles make the selected option clearer. Narrow disk cells preserve the percentage beside the label.
- **Storage guidance protects history.** Help recommends persistent metric storage and explains that changing the database path neither moves existing history nor fixes excessive writes.

## Proxmox, PBS and backups

- **Backup job outcomes cover two weeks.** Backups adds daily Proxmox VE results and labelled last-24-hours totals, distinguishing failures, warnings, running jobs and gaps in observed history.
- **PBS job results match each datastore.** Backup servers reads GC, prune, verify and sync run status from PBS. Plain error text counts as failure, and unclear unrelated tasks no longer make every datastore unknown.
- **Confirmed backups keep their status.** A freshly read backup can remain protected when another storage is unreadable. Coverage names unread sources without treating retained entries as fresh confirmation.
- **Backup servers are counted and grouped.** Each PBS server has one header with reachability and datastore count. Datastore rows show capacity and jobs, while server CPU, memory and uptime remain in its drawer.
- **Datastore drawers lead with the selected store.** Its name, free space, capacity and job results appear first, with the hosting server one click away.
- **Restore-point totals count completed backups.** By date counts running and failed entries separately, so completed totals agree with Backup health.
- **Backup lists distinguish workloads.** Rows identify overlapping connections, preserve more of each workload name in narrow windows and show reasons such as Failed, Overdue and No backup.
- **Coverage opens individual restore points.** Expanding a workload shows one row per point, the newest backup and failed-job details. Opening a point reveals its location, readable date, size and identifier.
- **PBS backups stay with the correct guest.** Reused names and VMIDs no longer let another connection claim a backup. Badges, Coverage, datastore counts and Patrol agree, using namespace and connection details to resolve shared names.
- **Backup wording distinguishes guest snapshots.** PBS restore points are called PBS backups, keeping them separate from guest snapshots that do not count as independent backups.
- **Ceph counts and health are more accurate.** Standby managers count as running in API and agent readings. Critical filters recognise ERR, and healthy reports stop inheriting old incident details.
- **Node details reflect the node itself.** Row colours include its storage and physical-disk alerts without borrowing guest alerts. Hardware details show linked-agent power readings, with retained values labelled last known.

## TrueNAS, vSphere and Docker

- **Namesake TrueNAS systems stay separate.** Connections with the same hostname no longer merge into one displayed machine.
- **TrueNAS history follows the selected system.** Drawer reads query only that appliance and discard results from removed or replaced connections.
- **Machine I/O history stays visible.** Native disk I/O can appear beside stored CPU history when those disk series are missing. Mixed history no longer claims a single source.
- **TrueNAS and vSphere editors ignore outdated replies.** Tests, saves and access errors stay with the account, workspace and editor that requested them. Unread saved settings offer recovery.
- **Docker health indicators update live.** Container row dots follow refreshed health results. Failure words remain readable in narrow rows.
- **Container action results use plain language.** Start, stop, restart and update results explain what Docker did and what Pulse confirmed, including failing or still-starting health checks.
- **Disabled Docker actions explain why.** Live updates refresh refusal reasons. Touch controls expose those explanations, and locked update badges point to Remote commands on the affected machine.
- **Docker commands reach the host's own agent.** Matching names cannot send commands to another machine or guest. A linked agent inside a VM remains reachable when its Proxmox node has no agent.
- **Guest Docker hosts remain visible.** Docker keeps runtimes linked to Proxmox guests on its page, including containers and their counts. Separately probed LXCs keep their own host rows.
- **LXC Docker discovery uses the guest's own node.** Checks require a linked agent on the correct Proxmox connection. Missing links skip the check, and failures on one node no longer pause checks on its namesake elsewhere.
- **Compose preserves healthcheck failures.** Bundled and installer-generated Compose files use the image's HTTPS-aware probe without masking failures with an HTTP fallback.

## Install, updates and agents

- **Agent installation reports failures honestly.** URL validation runs before replacing services or credentials. Failed startup returns an error and directs you to logs instead of declaring success.
- **Server installation stops on uncertain service state.** An unreadable or changing service state prevents installation changes. Service-manager checks and startup waits have enforced time limits.
- **Agent profile changes require known recovery state.** Migration stops if collector or helper service state is uncertain. Recovery files remain available until both services have stopped.
- **NAS installation handles older utilities.** URL, hostname and certificate fingerprint processing works with older NAS text tools.
- **Container installers download fully before running.** The wrapper stages the script privately and checks its syntax. It accepts private token files and rejects empty credential-file arguments.
- **Command-access setup preserves credentials.** Preparing a replacement token leaves existing agent tokens valid. Concurrent requests no longer create the same install credential twice.
- **Agent updates retain clearer outcomes.** Binary replacement and persistent NAS copies are more durable. Failed recovery no longer reports an earlier update as newly completed.
- **Agent Doctor refreshes open rows.** Versions, diagnoses and repair commands update as fresh fleet information arrives.
- **Uninstall failures describe stopped services.** If agent removal is unconfirmed, the installer distinguishes retained files from running monitoring and explains how to reconcile registration.
- **Community-scripts LXC installs are recognised again.** Pulse detects the newer update helper and shows appropriate update controls.
- **Restart instructions name the service.** Security settings give Proxmox LXC users the Pulse service name, including the older name, instead of an updater that may exit without restarting it.
- **Update progress stays with the running update.** Background checks no longer overwrite progress or the final result. Saved channel changes reach update checks consistently, and slow progress connections no longer hold shutdown open.

## Pulse Pro, AI and hosted

- **Reporting distinguishes outages from empty scope.** Failed resource reads offer retry. Earlier-account selections, results and downloads cannot populate the current report, including after live updates reconnect.
- **Organization and billing actions use current details.** Workspace switches and failed reads clear outdated controls. Sharing drafts cannot reuse removed destinations, and recovery feedback stays beside the controls.
- **Infrastructure updates use the selected organization.** Update lists, summaries and check commands use that organization's agents.
- **Patrol and AI stay within their organization.** Findings, events, notifications, anomaly learning and guest probes use the owning workspace. Deployment choices and scheduler health also use the selected workspace.
- **Locked history ranges link to Plans & Billing.** The range notice offers a direct route to plan details. Plans & Billing also introduces the separate Pulse MSP installation and its evaluation.
- **MSP setup collects required values first.** First-run setup asks for the domain, certificate email and DNS token before package changes. Evaluation instructions point to the published provider bundle.
- **MSP installs can obtain TLS certificates.** Traefik uses the configured certificate email and DNS provider. Setup checks certificate and DNS readiness and explains failures.
- **MSP evaluation failures explain the next step.** Setup distinguishes rejected requests, rate limits, server failures and connection problems. A running portal no longer implies active MSP capabilities.
- **MSP client monitoring remains available.** Lost paid access falls back to Community features. Renewal supports private provider networks, and clients still being created retain credentials for another attempt.
- **Portal forms support the keyboard.** Enter submits sign-in and Add client once while pending. Failed client creation preserves the name, and opening a client from the portal avoids an extra sign-in form.
- **Portal account controls are clearer.** Signed-in identity and Sign out stay visible. Recovery commands wrap in narrow windows, and access controls name the member they affect.
- **Assistant distinguishes same-named Proxmox nodes.** Topology and command availability use each node's connection and linked agent, so another installation's agent cannot make an offline node appear ready for commands.

## Monitoring and service health

- **Buffered reports no longer inflate rates.** Machine and Docker disk and network rates use collection times after reconnect. Libvirt CPU and I/O rates use their collection interval. Invalid intervals remain unknown.
- **System Logs shows connection loss.** Connecting, reconnecting and disconnected states replace a misleading Live label. Uncertain access clears buffered lines and offers an explicit reconnect.
- **Diagnostics clears outdated results.** Switching account, workspace or panel clears earlier results and feedback. Failed reads no longer leave old results available to export.
- **Linux sensors retain more detail.** Agents report individual temperature and fan inputs with stable identities, including stopped fans and invalid readings. New sensor alerts stay off until configured.
- **Agent links survive repeated names and NAT.** Adding installations with matching machine or guest names preserves established links when appropriate. Ambiguous new matches stay unlinked, and guest history stays within its connection.
- **Guest reads pause during possible replication freezes.** Pulse defers guest-agent commands during discovered replication windows and when replication status cannot be verified.
- **Guest filesystem reads run about once a minute.** Pulse reuses successful replies between reads, preserving their original time and clearing them after a guest restart.
- **Current linked agents avoid extra guest reads.** Pulse skips QEMU filesystem queries when the linked agent supplies disks and prefers its current memory reading over reading a guest file.
- **Unread VM disks stay out of History.** Fallback polling no longer records retained readings as fresh samples or allocated capacity as measured filesystem usage.
- **Monitoring reloads avoid overlapping writers.** Pulse refuses to replace a runtime that has not fully stopped. Retired workspace monitors cannot reopen stores or restart monitoring through late requests.
- **Startup leaves existing monitoring available.** Starting another workspace no longer blocks live reads from others. Discovery requests also avoid racing with initialization.
- **Patrol handles missing readings and slow lookups.** Missing sensor data cannot close an existing sensor alert. Stalled resource lookups no longer hold up Patrol shutdown.

## Security

- **Sign-out reports uncertainty.** Pulse confirms the local session ended before proxy navigation. Failed confirmation or browser cleanup shows a warning. Portal sign-out preserves unfinished work and lets you try again.
- **Private responses stop when access ends.** History, audit data, configuration exports, reports, support bundles, Assistant results and progress streams recheck the requesting session or token before sending more data.
- **Browser results stay with the active account.** Resource lists, agents, API tokens, SSO, workload metadata and audit controls discard outdated replies and confirmations after navigation or access changes.
- **Default workspace access is restricted in multi-org mode.** Admins manage members in Settings → Organization access. Eligible existing local users migrate. Review those grants there. Proxy users may need an explicit grant.
- **Administrator checks respect role rules.** Inherited administrator permissions count, while explicit denials still block access.
- **Tenant boundaries apply to more requests.** Basic-authenticated requests respect workspace access. Configuration imports and exports require permission to manage that workspace.
- **Proxy-authenticated browser writes require the right origin.** Requests from a different site origin are refused, including sites sharing the same parent domain.
- **Support bundles expose less configuration.** System summaries use limited diagnostic fields, selected environment values and actual software versions. Included logs still need private handling.
- **API clients keep credentials at the configured address.** CLI, MCP and external-agent requests refuse redirects to another scheme, host or port. Invalid agent URLs stay out of startup logs.
- **Saved credentials use safer file writes.** Configuration, bootstrap, billing and agent enrollment avoid unsafe temporary paths and interrupted writes. Relocated agent state remains private across enrollment and restart.
- **Protected content is handled more safely.** Private downloads and Assistant results reject executable formats. Import checks better protect passphrase fingerprints, and portal account responses disable HTTP caching.
- **Token controls preserve intended access.** Custom scopes stay distinct from Full access. Unconfirmed changes require a fresh inventory check before another edit. Closing or replacing a token reveal clears old copy feedback.

## Other improvements

- **Registration guidance matches authentication.** Help distinguishes public discovery from protected registration and directs users to scoped tokens in Settings.
- **API examples keep body secrets private.** Password and token examples use private request files instead of shell arguments or history.
- **Recovery guidance preserves authentication.** Help prefers authenticated configuration transfers and explains that direct-loopback recovery creates a session tied to the requesting client.
- **Infrastructure recovery preserves connections.** Help explains access failures and direct HTTPS endpoint corrections without deleting connections, replacing credentials or weakening certificate checks.
- **Saved-but-unapplied changes have recovery guidance.** Help distinguishes saved connection edits or removals from live monitoring, so an apply error does not lead users to repeat a completed removal.
- **Wrong agent links have recovery instructions.** Help documents unlinking a node without removing or reinstalling its agent. Organization and role guides explain read-only recovery after uncertain access changes.
- **VM I/O guidance explains passed-through storage.** Help clarifies that linking an agent does not change Proxmox VM I/O charts and points to existing agent-backed disk readings.
- **Privilege-helper updates have instructions.** Help and agent logs explain that automatic updates replace only the collector. Updating the helper requires the agent installer's update flow.
- **MSP networking guidance explains overlapping subnets.** Client workspaces share the server's routes. Identical private addresses need distinct reachable addresses or tunnel NAT.
- **Pro terms describe current AI modes.** The terms explain Ask first, Safe auto-fix, Autopilot and connected external agents, including which modes can run fixes.
- **MCP updates preserve the working adapter.** Failed replacement copies leave the installed adapter intact, and unsafe destination types are refused.
- **Setup and security guidance is current.** Compose examples separate image selection from authentication, token help explains clipboard limits, and updated runtime libraries include HTML parser and renderer security fixes.

## Known issues

- **Guest freezes and Windows restarts remain unresolved.** Related fixes do not establish safety for every guest. If monitoring was stopped after a guest froze, do not restart it solely to test this beta or release.
- **Freeze-enabled backups still need precautions.** Stop Pulse server monitoring beforehand. Keep it stopped until independent checks confirm thaw, fresh writes on every backed-up filesystem and workload liveness. An OK backup does not prove thaw.
- **Some freezes have no detectable VM lock.** Cloning a running VM or importing disks can freeze it without Pulse detecting it. Reads already running before a backup lock may also collide with thaw. Avoid manual guest-agent probes during freeze or thaw.
- **Kubernetes pauses need storage and controller checks.** Confirm persistent server storage before stopping pods and prevent controllers replacing them. Deleting a pod or stopping only the collector does not confirm server polling stopped.
- **Windows disk readings may remain unavailable.** Keep using the working Machines disk view. Do not remove or reinstall agents, restart guests or force guest-agent checks solely to recover Proxmox disk readings.
- **Elevated server memory remains unresolved.** Keep the working setup and compare existing readings at similar uptimes and workloads. These changes do not establish a lower memory baseline. No restart or heap dump is needed solely for this report.
- **Previously affected machines still need caution.** If agents or Patrol were disabled after a machine lockup, keep that setup. Preserve existing evidence privately without re-enabling them, changing GPU load or altering hardware to recreate the incident.
- **Alert history can lose entries after storage failures.** A restart can show healthy history while entries remain missing. Preserve existing logs and delivery records. Do not treat a healthy history indicator as proof the record is complete.
- **Agent update versions still need checking.** Update checks can accept a different binary version from the one advertised. Check the version reported by the running agent before treating an update as complete.
- **ARMv6 automatic updates can keep failing.** Older clients may request an ARMv7 binary. Use the agent installer's saved-state update flow with the correct architecture rather than repeatedly retrying automatic updates.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.7.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.7.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.7.0` and recreate the container.
