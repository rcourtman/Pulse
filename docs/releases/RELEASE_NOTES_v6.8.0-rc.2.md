# Pulse v6.8.0-rc.2 Release Notes

This release candidate follows `v6.8.0-rc.1` on the way to stable `v6.8.0`. Stable `v6.7.0` remains the rollback target.

Pulse v6.8.0 makes backup problems easier to identify, adds individual sensor limits and improves notification recovery after restarts. It keeps same-named systems separate, reduces routine polling and disk writes, and makes Patrol investigations and settings recovery clearer.

## Highlights

- Backups → Backup health now groups protection problems by cause and opens Coverage filtered to affected workloads.
- Alerts → Thresholds → Machine Sensors now lets you set a separate temperature limit for each Linux sensor.
- Backups → Backup servers now shows datastore fill forecasts when a reliable growth trend is available.

## What's improved

- **Larger fleets need less repeated work.** Proxmox, PBS and PMG polling copies less unrelated data. Resource lists, workload charts, custom alert rules and Agent Doctor also avoid repeated work.
- **Latest readings stay visible.** Resource drawers keep available CPU, memory and other readings when history fails or exceeds your plan, with older readings still labelled.
- **Settings offers access recovery.** Failed permission checks clear outdated controls and offer retry instead of leaving stale permissions on screen.
- **Infrastructure failures explain recovery.** Unread connection lists and settings offer retry, with help for saved changes awaiting reload and direct HTTPS recovery that preserves connections and certificate checks.
- **Role edits preserve saved choices.** Roles keeps permissions outside the shortcuts, inheritance and priority. Inherited administrator grants work while explicit denials still apply.
- **Uncertain access changes prompt a check.** Roles, User Access, API tokens and SSO require reading saved state before another edit when a change cannot be confirmed, preserving relevant drafts.
- **SSO saves protect existing settings.** Delayed edits cannot restore older secrets or hidden settings. Failed startup reads block provider changes until an authenticated read recovers the saved configuration.
- **Missing organization access has a recovery screen.** Users can answer invitations, check access again or sign out. Failed invitation reads offer retry and identify the organization being joined.
- **Refreshes preserve unfinished work.** Unchanged access checks retain access, sharing, organization and authentication drafts and pending results. Concurrent system saves preserve unrelated fields and reach workspaces still starting.
- **Conflicting organization saves preserve current access.** Overlapping edits cannot restore removed members, invitations or shares, or recreate a deleted workspace. Reload current details before trying again.
- **Docs is easier to use.** Load failures explain their cause, retry keeps the guide, heading links restore focus, and wide tables support keyboard scrolling. French is available in supported interface areas and the getting-started guide.
- **Needs attention uses the drawer width.** Alert details fill available space and wrap into readable rows.

## Alerts and notifications

- **Queued notifications survive graceful restarts.** Pending deliveries and buffered groups retain their work and grouping deadline. Sending resumes after destinations, quiet hours and delivery settings load.
- **Restarts preserve notification cooldowns.** Delivered active alerts avoid duplicate firing messages during their cooldown. New occurrences, increased severity and new recipients can still notify.
- **Long-running alerts keep recovery messages.** Cleanup retains recoveries still owed for active alerts and pending work. Changed recipients cannot inherit another destination's delivery record.
- **Confirmed deliveries avoid repeat sends.** Once provider success is saved, Pulse retries unfinished local bookkeeping without resending. A crash before success is saved can still cause a repeat.
- **Shutdown preserves notification handoffs.** Monitoring finishes handing pending firing and recovery messages to delivery before stopping it, including when organization access ends.
- **Alert history handles storage faults better.** Pending events replay after storage recovers. Active alerts and controls survive restarts without resending old notifications.
- **Acknowledgements avoid duplicate requests.** Individual, group and bulk actions share pending state across navigation. Partial failures report successful and failed counts separately.
- **Threshold markers and notes stay accurate.** Active-alert dots return to metric rows, including filesystems and disk temperatures. Override notes survive reopening and edits that change only the note.
- **Thresholds distinguishes repeated names.** Proxmox and Docker names stay readable with details separating namesakes. Searching Docker thresholds no longer places overrides under the wrong host.
- **Delivery errors stay readable and private.** Recent delivery activity preserves ordinary errors while hiding webhook credentials. Diagnostics omit private destination identifiers, and temporary HTTP clients release completed connections.
- **Organization managers can configure destinations.** Providers can open client alert settings without instance-wide access. New Apprise command-line targets require an instance administrator, and option-like targets are rejected.
- **Unavailable delivery explains recovery.** Alerts distinguishes unread settings, storage failures and uncertain alert state while detection continues. Imports say when saved notification settings need a restart.

## Disks and storage

- **Full disks remain visible in summaries.** Compact Proxmox rows show the fullest measured disk despite another missing reading. Wide machine rows keep individual bars, and narrow cells preserve percentages.
- **Disk controls show the selected view clearly.** Disk, alert and view toggles are easier to distinguish on machine tables.
- **VM history keeps readings with their source.** Fallback polling cannot record old readings as fresh or allocated capacity as measured filesystem usage. Guest history stays within its Proxmox connection.
- **Machine I/O history stays visible.** Native disk I/O can appear beside stored CPU history when disk series are missing. Mixed history no longer claims a single source.
- **Routine monitoring writes less to disk.** Metric samples are normally saved in minute batches and host report bookkeeping is grouped, while chart reads still request fresh data. An abrupt crash can lose the newest buffered minute.
- **Saved configuration and agent credentials use safer writes.** Private, complete-file replacement protects configuration, bootstrap state and enrollment from interrupted writes and unsafe temporary paths, including relocated agent storage.

## Proxmox, PBS and backups

- **Backups opens on Coverage.** The page places health first, then backup servers and restore points, with consistent metric colours. The restore-points-per-day chart is removed.
- **Backup jobs shows two weeks of outcomes.** Daily results and last-24-hours totals distinguish failures, warnings, running jobs and missing history. Hover shows details, and selecting a day filters and reveals matching rows.
- **PBS job results match each datastore.** Backup servers reads GC, prune, verify and sync status directly from PBS. Plain error text counts as failure, while unrelated unclear tasks cannot make every datastore unknown.
- **Failed PBS jobs raise datastore alerts.** Sync, verify, prune and garbage-collection failures stay separate from guest protection. Known failures survive outages, partial reads, running retries and Pulse restarts.
- **Confirmed backups keep their status.** Freshly read backups can remain protected when another storage is unreadable. Coverage names unread sources without treating retained entries as fresh confirmation.
- **Uncertain backup ownership stays Unknown.** Guests that could own an unassigned PBS backup no longer appear definitely unprotected. Removed connections stop leaving stale ownership uncertainty behind.
- **Backup servers are grouped by server.** Headers show reachability and datastore count. Rows and datastore drawers lead with capacity and jobs, while server CPU, memory and uptime remain in its drawer.
- **Restore-point totals count completed backups.** By date counts running and failed entries separately, so completed totals agree with Backup health.
- **Coverage opens individual restore points.** Workload and connection labels lead to expanded rows showing each point, newest backup and failures. Details include location, readable date, size and identifier.
- **PBS backups stay with the correct guest.** Reused names and VMIDs cannot let another connection claim a backup. Badges, Coverage, datastore counts and Patrol share namespace-aware ownership rules.
- **Backup wording separates guest snapshots.** PBS restore points are called PBS backups, keeping them distinct from guest snapshots that do not count as independent backups.
- **Ceph and node details are more accurate.** Standby managers count as running, ERR matches critical filters, and healthy Ceph reports lose old incident details. Node rows include storage alerts, and hardware details show linked-agent power.

## TrueNAS, vSphere and Docker

- **Namesake TrueNAS systems stay separate.** Connections with the same hostname no longer merge into one displayed machine.
- **TrueNAS history follows the selected system.** Drawer reads query only that appliance and discard results from removed or replaced connections.
- **TrueNAS and vSphere editors ignore outdated replies.** Tests, saves and access errors stay with the requesting account, workspace and editor. Unread saved settings offer recovery.
- **Docker distinguishes same-named hosts.** Host lists, selectors and resource tables use each host's identity and show extra details when names repeat.
- **Docker health indicators update live.** Container row dots follow refreshed health results, and failure words remain readable in narrow rows.
- **Container actions explain their results.** Start, stop, restart and update feedback says what Docker did and what Pulse confirmed, including failing or still-starting health checks.
- **Disabled Docker actions explain why.** Live updates refresh refusal reasons, disabled controls expose explanations, and locked update badges point to Remote commands on the affected machine.
- **Docker commands reach the host's own agent.** Matching names cannot route commands elsewhere. An agent inside a VM remains reachable when its Proxmox node has no agent.
- **Guest Docker hosts remain visible.** Docker keeps runtimes linked to Proxmox guests, including containers and counts. Separately probed LXCs keep their own host rows.
- **LXC Docker discovery uses the guest's own node.** Checks require a linked agent on the correct Proxmox connection, skip missing links and no longer pause another namesake node after a failure.
- **Docker setup needs fewer privileges.** Configuration preparation works without root, Docker or host access and clearly says Pulse has not started. Compose health checks respect HTTPS without masking failures.
- **Docker update instructions require recreation.** The update overview explains that pulling an image alone does not replace the running Pulse container. Container SSH warnings appear only when SSH actually runs.

## Install, updates and agents

- **Agent installation reports failures honestly.** URL validation precedes service or credential replacement. Failed startup returns an error with log guidance instead of declaring success.
- **Server installation protects working services.** Uncertain service state stops changes, checks have time limits, and failed replacement writes preserve existing service files.
- **Agent profile changes preserve recovery files.** Migration requires known collector and helper state, and keeps recovery files until both stop. Unconfirmed removal explains retained files, service state and registration recovery.
- **Older NAS utilities and curl work again.** Installers handle older text tools and curl versions. Downloads ignore implicit curl settings so local defaults cannot silently change installation requests.
- **Container installers download before running.** Downloads have size and time limits and are staged privately for syntax checks. The wrapper accepts private token files and rejects empty credential-file arguments.
- **Command-access setup preserves credentials.** Preparing a replacement token leaves existing agent tokens valid. Concurrent requests no longer create the same install credential twice.
- **Agent updates check version and architecture.** Downloads must contain the expected release for the intended platform before activation. ARMv6 builds select ARMv6 updates rather than guessing ARMv7.
- **Agent updates retain honest outcomes.** Binary replacement and persistent NAS copies are more durable. Failed recovery no longer reports an earlier update as newly completed.
- **Automatic server updates cannot overlap.** Competing runs cannot update the same installation at once. A busy lock or missing flock stops the update before installation changes.
- **Unattended update status is more accurate.** Failed timer checks report errors. Optional setup failures do not claim updates are enabled, temporary-file cleanup does not stop monitoring, and recovery waits obey elapsed time limits.
- **Community-scripts LXC installs are recognised again.** Pulse detects the newer update helper and shows suitable controls. Security settings names the current and older Pulse service in restart instructions.
- **Update progress stays with the running update.** Background checks cannot overwrite progress or final results. Channel changes reach checks consistently, slow connections do not delay shutdown, and replacement assets use private staging.

## Pulse Pro, AI and hosted

- **Reporting distinguishes outages from empty scope.** Failed resource reads offer retry. Earlier-account selections, results and downloads cannot populate the current report, including after live updates reconnect.
- **Organization controls use current details.** Billing and sharing discard outdated actions after workspace changes or failed reads. Sharing requires a named recipient, and ownership transfers identify the workspace before confirmation.
- **Losing workspace access preserves sign-in.** Live data disconnects without silently switching workspaces or repeatedly reconnecting. Select another authorized workspace, or reload after access is restored.
- **Workspace tools use their own systems.** Patrol findings, notifications, learning and guest probes stay in their organization. Infrastructure updates, deployment choices and scheduler health use the selected workspace.
- **Patrol follows recurring alerts.** Each new firing can trigger investigation even when notifications are off. Runs find the correct resource, and findings that repeat an alert close when that alert clears.
- **Patrol checks lasting alerts.** In Ask first and Autopilot, alerts still active after 10 minutes can receive one investigation, even below the immediate severity setting, with a limit of four per hour and the alert-trigger switch and filters respected.
- **Waiting investigations start when space opens.** Patrol keeps findings queued while all investigation slots are busy, prioritizing critical findings and then those waiting longest.
- **Patrol shows current action and approval status.** Summaries, rows and resource links agree on the next step. Failed approval reads retain known requests and usable findings instead of implying an empty queue or suggesting premature recovery.
- **Patrol explains missing command access.** Reporting agents with commands off are distinguished from disconnected agents. Patrol names affected hosts and opens the install command with command access selected.
- **Patrol notifications follow each finding.** Recovery uses resolution and recurrence history, rather than dismissal, snooze or suppression. Stalled resource lookups no longer hold up shutdown.
- **Plans & Billing explains more options.** Locked history ranges link to plan details, which also introduces the separate Pulse MSP installation and evaluation.
- **MSP clients keep monitoring after paid access ends.** Clients fall back to Community features. Renewal supports private provider networks, and clients still being created retain credentials for another attempt.

## Monitoring and service health

- **Buffered reports no longer inflate rates.** Machine and Docker disk and network rates use collection times after reconnect. Libvirt CPU and I/O rates use their collection interval and leave invalid intervals unknown.
- **System Logs shows connection loss.** Connecting, reconnecting and disconnected states replace a misleading Live label. Uncertain access clears buffered lines and offers explicit reconnect.
- **Diagnostics and Agent Doctor stay current.** Diagnostics clears results when its account, workspace or panel changes and blocks stale exports after failures. Open Doctor rows refresh versions, diagnoses and repair commands.
- **Linux sensors report more detail.** Agents collect individual temperatures and fan speeds with stable names, including stopped fans and invalid readings. New alerts start disabled, and missing readings cannot close an existing sensor alert.
- **Agent links and commands distinguish namesakes.** Links survive NAT and reused names, ambiguous matches stay unlinked, and Assistant uses each node's own agent. Separate command sessions reset reconnect delays after a healthy connection.
- **Guest reads pause during possible replication freezes.** Pulse defers guest-agent commands during discovered replication windows and when replication status cannot be verified.
- **Guest filesystem reads run about once a minute.** Successful replies are reused between reads with their original observation time and cleared after a guest restart.
- **Linked agents avoid extra guest reads.** Pulse skips QEMU filesystem queries when the linked agent supplies disks and prefers its current memory reading over reading a guest file.
- **Monitoring reloads avoid overlapping writers.** Pulse refuses to replace a runtime that has not fully stopped. Retired workspace monitors cannot reopen stores or restart monitoring through late requests.
- **Workspace startup leaves other monitoring available.** Starting another workspace no longer blocks existing live reads. Discovery avoids racing initialization, and backup polling follows elapsed cycles rather than a shared counter.
- **Slower agent reports stay fresh between updates.** Agent rows, SMART disks and Unraid storage use the configured reporting interval, avoiding false stale warnings for on-time reports.
- **Proxmox temperatures follow the reporting agent.** Valid readings from slower agents remain usable without unnecessary SSH checks. Old sibling readings expire even when the Proxmox connection remains live.

## Security

- **Private responses stop when access ends.** History, audit data, configuration exports, reports, support bundles, Assistant results and progress streams recheck the requesting session or token before sending more data.
- **Browser data and credentials stay with their owner.** Resource lists and audit controls discard old replies after access changes, password changes use the current local account, and uncertain credential changes keep attempted values for recovery.
- **Default workspace membership is restricted.** In multi-org mode, use Settings → Organization access to manage membership, review migrated local-user grants and explicitly grant proxy users access where needed.
- **Access checks cover more requests.** Basic authentication respects workspace membership, configuration transfers require management access, and AI review uses action permissions. Unread roles or policy and malformed access edits are refused.
- **Proxy credentials cannot authorize cross-site writes.** Unapproved origins cannot make browser writes using proxy identity headers, Basic credentials or API tokens, including sites sharing the same parent domain.
- **Diagnostics exposes less private data.** Support bundles limit configuration and environment fields and show actual versions. Browser diagnostics omit private response bodies, while included server logs still need private handling.
- **API clients keep credentials at the configured address.** CLI, MCP and external-agent requests refuse redirects to another scheme, host or port. Agent tokens can auto-register only their recorded source.
- **Saved keys and approvals survive read failures.** Pulse preserves invalid encryption keys for recovery from the original backup, keeps billing secrets when encryption is unavailable, and refuses to replace unread approvals with an empty store.
- **AI commands have stronger limits.** Read tools block credentials, process environments and database clients, and restrict outbound connections. Command approvals must be marked used on disk before execution, preventing reuse after restart.
- **Protected content is safer to open.** Downloads and Assistant results reject executable formats, HTML base tags cannot redirect Pulse links, import checks protect passphrase fingerprints, and libraries include HTML security fixes.
- **Authentication stays responsive.** Slow persistence and token checks no longer block sign-in. Basic API polling avoids browser sessions, unread session files are preserved, and OIDC refresh cannot overwrite a newer session or undo sign-out.
- **SSO rejects obsolete login attempts.** OIDC and SAML callbacks honor provider removal and policy changes. SAML requires bearer confirmation and a matching audience. Replace RSA PKCS#1 v1.5 key transport with supported encryption.

## Other improvements

- **Access controls identify saved targets.** Users, roles, API tokens, SSO providers, organizations and audit webhooks show their saved identities beside actions. Permissions, destinations and dialog buttons remain reachable in short windows.
- **Token details show saved expiry clearly.** API Access shows saved expiry without implying a token still has access, keeps custom scopes separate from Full access, and clears old copy feedback when a reveal closes or changes.
- **Account controls explain sign-out.** Identity and Sign out stay visible. Pulse confirms local closure before proxy navigation and offers recovery without losing drafts. Portal account responses prevent caching.
- **API and recovery help preserves access.** Examples use private request files and full custom-role replacement, with help for authenticated transfers, client-local recovery and enabling organizations per deployment.
- **SSO setup uses the actual login addresses.** Registration instructions reflect saved provider details and the public URL. Matching OIDC initialization preserves pending logins, and custom SAML paths stay on the configured host.
- **OIDC guidance explains login rules.** Saved providers take precedence over environment settings. Deployment-managed providers use their existing configuration, and every non-empty group, domain and email allowlist must match.
- **Agent and storage help avoids unnecessary changes.** Guidance covers wrong node links, separate Proxmox I/O sources, existing TrueNAS history and persistent metric storage, explaining that changing its path does not move history or reduce writes.
- **Privilege-helper updates have instructions.** Help and agent logs explain that automatic updates replace only the collector. Update helpers through the agent installer, and use install commands naming the current release.
- **MSP setup supports domain-free trials and certificates.** Setup collects values before package changes, checks DNS and TLS, and supports DNS or port-80 verification. Moving a trial to your own domain requires agent reinstallation.
- **MSP portal errors explain recovery.** Plan checks distinguish rejection, rate limits and outages. Forms support Enter without duplicate requests, retain failed client names and open clients without another sign-in. Paid features require a plan.
- **MSP guidance clarifies access and networking.** Provider access excludes client user and audit administration, read-only details explain restrictions, and overlapping private subnets need distinct reachable addresses or tunnel NAT.
- **MCP updates preserve the working adapter.** Failed replacement copies leave it intact, downloads have size and time limits, and unsafe destinations are refused. Pro terms explain current AI modes and which can run fixes.

## Known issues

- **Guest freezes and Windows restarts remain unresolved.** Related fixes do not establish safety for every guest. If monitoring was stopped after a guest froze, do not restart it solely to test this beta or release.
- **Freeze-enabled backups still need precautions.** Stop Pulse server monitoring beforehand. Keep it stopped until independent checks confirm thaw, fresh writes on every backed-up filesystem and workload liveness, regardless of an OK backup result.
- **Some freezes have no detectable VM lock.** Cloning or disk imports can freeze a VM without detection. Reads already running before a backup lock may collide with thaw, so avoid manual guest-agent probes during freeze or thaw.
- **Kubernetes pauses need storage and controller checks.** Confirm persistent server storage and prevent controllers replacing stopped pods. Deleting a pod or stopping only the collector does not confirm server polling stopped.
- **Some Windows disk readings may remain unavailable.** Keep using the working Machines disk view without removing or reinstalling agents, restarting guests or forcing guest-agent checks solely to recover Proxmox readings.
- **Elevated server memory remains unresolved.** Keep the working setup and compare existing readings at similar uptimes and workloads. Reduced polling work does not establish a lower baseline or require a restart or heap dump solely for this report.
- **Previously affected machines still need caution.** Keep agents or Patrol disabled after a lockup. Preserve existing evidence privately without re-enabling them, changing GPU load or altering hardware to recreate the incident.
- **Resume only previously active monitoring.** Avoid hooks that ignore failed stops or restart Pulse after an unanswered thaw check. Restore only previously active services and timers after independent guest checks.
- **Agent updates still need a version check.** Compare the running agent's reported version with the intended release before treating an update as complete, especially for older installed agents.
- **Earlier alert-history gaps cannot be recreated.** Preserve existing logs and delivery records after storage failures. A healthy current history store does not establish recovery of previously missing entries.
- **Older ARMv6 agents need a manual update.** Their updater can still request ARMv7. Use the agent installer's saved-state update flow with the correct architecture while preserving the agent ID and connection state.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.7.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.7.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.7.0` and recreate the container.
