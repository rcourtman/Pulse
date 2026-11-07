# Pulse v6.8.0-beta.2 Release Notes

This beta follows `v6.8.0-beta.1` on the way to stable `v6.8.0`. Stable `v6.7.0` remains the rollback target.

Pulse v6.8.0 makes backup problems easier to identify, adds individual temperature limits and improves notification recovery after restarts. It also keeps same-named systems separate, strengthens access controls and gives clearer recovery steps when settings or updates cannot be confirmed.

## Highlights

- Backups → Backup health now groups protection problems by cause and opens Coverage filtered to affected workloads.
- Alerts → Thresholds → Machine Sensors now lets you set a separate temperature limit for each Linux sensor.
- Backups → Backup servers now shows datastore fill forecasts when a reliable growth trend is available.

## What's improved

- **Proxmox polling does less copying.** Node and storage checks avoid copying unrelated fleet data, reducing temporary allocations as your monitored fleet grows.
- **Latest readings stay visible.** Resource drawers keep available CPU, memory and other readings when history fails or exceeds your plan, with older readings still labelled.
- **Settings offers access recovery.** Failed permission checks clear outdated controls and offer retry instead of leaving stale permissions on screen.
- **Infrastructure failures explain recovery.** Unread connection lists and saved settings offer retry instead of showing first-time setup or incomplete editors.
- **Role edits preserve saved choices.** Roles keeps permissions outside the shortcuts, inheritance and priority. Inherited administrator grants work while explicit denials still apply.
- **Uncertain access changes prompt a check.** Roles, User Access and SSO require reading saved state before another edit when a change cannot be confirmed. SSO keeps the draft visible during that check.
- **SSO saves preserve newer settings.** Delayed provider edits no longer restore older secrets or hidden settings over concurrent saves. Explicit replacements and cleared policy fields still take effect.
- **Unread SSO settings cannot be overwritten.** Failed startup reads block provider changes until an authenticated management read recovers the saved configuration.
- **Missing organization access has a recovery screen.** Users can answer invitations, check access again or sign out. Unread invitations offer retry and clearly identify the organization being joined.
- **Refreshes preserve unfinished work.** Unchanged access checks keep drafts and pending results in access, sharing and organization forms. Concurrent system saves retain unrelated fields and reach workspaces still starting up.
- **Docs is easier to recover and navigate.** Load failures explain their cause, retry keeps the guide, heading links restore focus, and wide tables support keyboard scrolling.
- **French is available.** Supported interface areas and the getting-started guide now include French.

## Alerts and notifications

- **Queued notifications survive graceful restarts.** Pending deliveries and buffered groups retain their work and grouping deadline. Sending resumes after destinations, quiet hours and delivery settings load.
- **Restarting preserves notification cooldowns.** Delivered active alerts avoid duplicate firing messages during their cooldown. New occurrences, increased severity and new recipients can still notify.
- **Long-running alerts keep recovery messages.** Cleanup retains recoveries still owed for active alerts and pending work. Changed recipients cannot inherit another destination's delivery record.
- **Confirmed deliveries avoid repeat sends.** Once provider success is saved, Pulse retries unfinished local bookkeeping without resending. A crash before success is saved can still cause a repeat.
- **Shutdown preserves notification handoffs.** Monitoring finishes handing pending firing and recovery messages to delivery before stopping it.
- **Alert history survives storage faults better.** Pulse retains pending events for replay after storage recovers and preserves active alerts and controls across restarts without resending old notifications.
- **Acknowledgements avoid duplicate requests.** Individual, group and bulk actions share pending state across navigation. Partial failures report successful and failed counts separately.
- **Threshold markers and notes stay accurate.** Active-alert dots return to metric rows, including filesystems and disk temperatures. Override notes survive reopening and edits that change only the note.
- **Thresholds distinguishes repeated names.** Proxmox and Docker names stay readable with details separating namesakes. Searching Docker thresholds no longer places overrides under the wrong host.
- **Delivery errors stay readable.** Recent delivery activity preserves ordinary errors while hiding webhook credentials. Notification diagnostics omit private destination identifiers.
- **Organization managers can configure destinations.** Providers can open client alert settings without instance-wide access. New Apprise command-line targets require an instance administrator, and option-like targets are rejected.
- **Unavailable delivery explains recovery.** Alerts distinguishes unread settings, storage failures and uncertain alert state while detection continues. Imports say when saved notification settings need a restart.

## Disks and storage

- **Full disks remain visible in summaries.** Compact Proxmox rows show the fullest measured disk despite another missing reading. Wide machine rows keep individual bars, and narrow cells preserve percentages.
- **Disk controls show the selected view clearly.** Disk, alert and view toggles are easier to distinguish on machine tables.
- **Missing VM readings stay out of History.** Fallback polling no longer records retained readings as fresh samples or allocated capacity as measured filesystem usage.
- **Storage guidance protects history.** Help recommends persistent metric storage and explains that changing the database path neither moves existing history nor fixes excessive writes.
- **Machine I/O history stays visible.** Native disk I/O can appear beside stored CPU history when disk series are missing. Mixed history no longer claims a single source.

## Proxmox, PBS and backups

- **Backups opens on Coverage.** The page places health first, then backup servers and restore points, with consistent metric colours. The restore-points-per-day chart is removed.
- **Backup jobs shows two weeks of outcomes.** Daily results and last-24-hours totals distinguish failures, warnings, running jobs and missing history. Hover shows details, and selecting a day filters and reveals matching rows.
- **PBS job results match each datastore.** Backup servers reads GC, prune, verify and sync status directly from PBS. Plain error text counts as failure, while unrelated unclear tasks cannot make every datastore unknown.
- **Failed PBS jobs raise datastore alerts.** Sync, verify, prune and garbage-collection failures stay separate from guest protection. Known failures survive outages, partial reads, running retries and Pulse restarts.
- **Confirmed backups keep their status.** Freshly read backups can remain protected when another storage is unreadable. Coverage names unread sources without treating retained entries as fresh confirmation.
- **Uncertain backup ownership stays Unknown.** Guests that could own an unassigned PBS backup no longer appear definitely unprotected. Removed connections stop leaving stale ownership uncertainty behind.
- **Backup servers are grouped by server.** Headers show reachability and datastore count. Rows and datastore drawers lead with capacity and jobs, while server CPU, memory and uptime remain in its drawer.
- **Restore-point totals count completed backups.** By date counts running and failed entries separately, so completed totals agree with Backup health.
- **Coverage opens individual restore points.** Clearer workload and connection labels lead to expanded rows showing each point, newest backup and failures. Details include location, readable date, size and identifier.
- **PBS backups stay with the correct guest.** Reused names and VMIDs cannot let another connection claim a backup. Badges, Coverage, datastore counts and Patrol share namespace-aware ownership rules.
- **Backup wording separates guest snapshots.** PBS restore points are called PBS backups, keeping them distinct from guest snapshots that do not count as independent backups.
- **Ceph and node details are more accurate.** Standby managers count as running, ERR matches critical filters, and healthy Ceph reports lose old incident details. Node rows include their own storage alerts, and hardware details show linked-agent power.

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
- **Server installation requires a known service state.** Unreadable or changing state stops installation changes. Service-manager checks and startup waits have enforced time limits.
- **Agent profile changes preserve recovery files.** Migration stops if collector or helper state is uncertain. Recovery files remain until both services have stopped.
- **Uninstall failures explain what stopped.** If removal cannot be confirmed, the installer distinguishes retained files from running monitoring and explains how to reconcile registration.
- **NAS installation handles older utilities.** URL, hostname and certificate fingerprint processing works with older NAS text tools.
- **Container installers download before running.** The wrapper privately stages and syntax-checks the script, accepts private token files and rejects empty credential-file arguments.
- **Command-access setup preserves credentials.** Preparing a replacement token leaves existing agent tokens valid. Concurrent requests no longer create the same install credential twice.
- **Agent updates check version and architecture.** Downloads must contain the expected release for the intended platform before activation. ARMv6 builds select ARMv6 updates rather than guessing ARMv7.
- **Agent updates retain honest outcomes.** Binary replacement and persistent NAS copies are more durable. Failed recovery no longer reports an earlier update as newly completed.
- **Automatic server updates cannot overlap.** Competing runs cannot update the same installation at once. A busy lock or missing flock stops the update before installation changes.
- **Community-scripts LXC installs are recognised again.** Pulse detects the newer update helper and shows suitable controls. Security settings names the current and older Pulse service in restart instructions.
- **Update progress stays with the running update.** Background checks cannot overwrite progress or final results. Saved channel changes reach checks consistently, and slow progress connections no longer delay shutdown.

## Pulse Pro, AI and hosted

- **Reporting distinguishes outages from empty scope.** Failed resource reads offer retry. Earlier-account selections, results and downloads cannot populate the current report, including after live updates reconnect.
- **Organization and billing controls use current details.** Workspace switches and failed reads clear outdated actions. Sharing requires a named recipient, rejects removed destinations, and ownership transfers identify the workspace before confirmation.
- **Infrastructure updates use the selected organization.** Update lists, summaries and check commands use that organization's agents.
- **Patrol and AI stay within their organization.** Findings, events, notifications, anomaly learning and guest probes use the owning workspace. Deployment choices and scheduler health follow the selected workspace.
- **Patrol notifications retain their own lifecycle.** Delivery cleanup uses the finding's resolution and recurrence history. Dismissal, snooze, suppression or unread findings cannot stand in for resolution.
- **Locked history links to plan details.** Range notices open Plans & Billing, which also introduces the separate Pulse MSP installation and evaluation.
- **MSP setup supports domain-free trials.** Setup collects needed values before package changes and can use a public-IP trial address. Moving to your own domain later requires agent reinstallation.
- **MSP certificates have clearer recovery.** Setup checks TLS and DNS readiness, supports DNS-based or token-free port-80 verification, and explains certificate failures and shared trial limits.
- **MSP plan failures explain the next step.** Setup distinguishes rejection, rate limits, server failures and connection problems. A running portal does not imply paid features, and refresh feedback avoids assuming whether payment arrived.
- **MSP client monitoring survives lost paid access.** Clients fall back to Community features. Renewal supports private provider networks, and clients still being created retain credentials for another attempt.
- **Portal tasks are easier to complete.** Keyboard submission avoids duplicate requests, failed client creation keeps the name, and opening clients avoids another sign-in. Alert setup opens alert settings, and Access identifies provider staff.
- **Assistant distinguishes same-named Proxmox nodes.** Topology and command availability use each node's connection and linked agent. Another installation's agent cannot make an offline node appear ready for commands.

## Monitoring and service health

- **Buffered reports no longer inflate rates.** Machine and Docker disk and network rates use collection times after reconnect. Libvirt CPU and I/O rates use their collection interval and leave invalid intervals unknown.
- **System Logs shows connection loss.** Connecting, reconnecting and disconnected states replace a misleading Live label. Uncertain access clears buffered lines and offers explicit reconnect.
- **Diagnostics and Agent Doctor stay current.** Diagnostics clears results when its account, workspace or panel changes and blocks stale exports after failures. Open Doctor rows refresh versions, diagnoses and repair commands.
- **Linux sensors report more detail.** Agents collect individual temperature and fan inputs with stable identities, including stopped fans and invalid readings. New sensor alerts remain off until configured.
- **Agent links survive repeated names and NAT.** Established links remain stable where appropriate, ambiguous new matches stay unlinked, and guest history stays within its connection. Repeated Proxmox names show connection details.
- **Same-named machines can both receive commands.** Separate agents retain their own command sessions. Healthy sessions reset reconnect delays so later outages do not inherit an old long wait.
- **Guest reads pause during possible replication freezes.** Pulse defers guest-agent commands during discovered replication windows and when replication status cannot be verified.
- **Guest filesystem reads run about once a minute.** Successful replies are reused between reads with their original observation time and cleared after a guest restart.
- **Linked agents avoid extra guest reads.** Pulse skips QEMU filesystem queries when the linked agent supplies disks and prefers its current memory reading over reading a guest file.
- **Monitoring reloads avoid overlapping writers.** Pulse refuses to replace a runtime that has not fully stopped. Retired workspace monitors cannot reopen stores or restart monitoring through late requests.
- **Workspace startup leaves other monitoring available.** Starting another workspace no longer blocks existing live reads. Discovery avoids racing initialization, and backup polling follows elapsed cycles rather than a shared counter.
- **Missing readings do not imply recovery.** Missing sensor data cannot close an existing sensor alert. Stalled Patrol resource lookups no longer hold up shutdown.

## Security

- **Sign-out explains uncertainty.** Pulse confirms local session closure before proxy navigation, warns about failed confirmation or browser cleanup, and preserves unfinished portal work with a retry option.
- **Private responses stop when access ends.** History, audit data, configuration exports, reports, support bundles, Assistant results and progress streams recheck the requesting session or token before sending more data.
- **Browser results stay with the active account.** Resource lists, agents, workload metadata and audit controls discard outdated replies and confirmations after navigation or access changes.
- **Default workspace membership is restricted.** In multi-org mode, use Settings → Organization access to manage membership, review migrated local-user grants and explicitly grant proxy users access where needed.
- **Workspace permissions cover more requests.** Basic authentication respects membership, configuration transfers require workspace management permission, and malformed role, user-access and token changes are refused before saving.
- **Proxy credentials cannot authorise cross-site writes.** Browser writes from unapproved origins are refused when proxies attach identity headers, Basic credentials or API tokens, including sites sharing the same parent domain.
- **Diagnostics exposes less private data.** Support bundles limit configuration and environment fields and show actual versions. Browser diagnostics omit private response bodies, while included server logs still need private handling.
- **API clients keep credentials at the configured address.** CLI, MCP and external-agent requests refuse redirects to another scheme, host or port. Invalid agent URLs stay out of startup logs.
- **Saved credentials use safer file writes.** Configuration, bootstrap, billing and enrollment avoid unsafe temporary paths and interrupted writes. Relocated agent state remains private across enrollment and restart.
- **Protected content is safer to open.** Downloads and Assistant results reject executable formats, HTML base tags cannot redirect Pulse links, import checks protect passphrase fingerprints, and libraries include HTML security fixes.
- **Token controls preserve intended access.** Custom scopes stay separate from Full access. Uncertain changes require a fresh inventory check, and closing or replacing a token reveal clears old copy feedback.
- **Authentication stays responsive.** Slow session saves and token readers no longer block sign-ins. Basic API polling stops creating browser sessions, while OIDC refresh cannot overwrite a replacement session, reuse a rotated token or undo sign-out.

## Other improvements

- **Access forms stay usable.** Roles and User Access keep actions, saved permissions and errors reachable in short windows. Searches distinguish filtered results from an empty directory.
- **Needs attention uses the drawer width.** Alert details fill available space and wrap into readable rows.
- **Account and invitation controls stay clear.** Identity and Sign out remain visible, invitation identities stay readable, and access actions name the affected member. Account responses disable HTTP caching.
- **API guidance protects secrets and saved access.** Examples use private request files, support existing token edits, explain optional expiry and scope defaults, and show complete custom-role replacement without dropping restrictions.
- **Recovery guidance preserves access and connections.** Help covers authenticated transfers, local recovery limited to the connecting client, missing role controls and direct HTTPS corrections without deleting connections or weakening certificate checks.
- **Saved changes have clearer recovery steps.** Help separates saved settings from failed live reloads, including SSO provider changes and role-assignment audit events, so uncertain results do not invite repeated writes.
- **Wrong agent links have recovery instructions.** Help explains how to unlink a node without removing or reinstalling its agent.
- **VM I/O guidance explains passed-through storage.** Help clarifies that linking an agent does not change Proxmox VM I/O charts and points to existing agent-backed disk readings.
- **Repeated TrueNAS curves have comparison guidance.** Help explains comparing existing history sources without repeating workloads, backups or guest-agent probes.
- **Privilege-helper updates have instructions.** Help and agent logs explain that automatic updates replace only the collector and that helper updates use the agent installer's update flow. Install commands name the current release.
- **MSP guidance clarifies access and networking.** Providers are not promised client user or audit administration. Read-only details explain existing restrictions, and overlapping private subnets require distinct reachable addresses or tunnel NAT.
- **MCP updates preserve the working adapter.** Failed replacement copies leave the installed adapter intact. Downloads have size and time limits, and unsafe destination types are refused. Pro terms also explain current AI modes and which can run fixes.

## Known issues

- **Guest freezes and Windows restarts remain unresolved.** Related fixes do not establish safety for every guest. If monitoring was stopped after a guest froze, do not restart it solely to test this beta or release.
- **Freeze-enabled backups still need precautions.** Stop Pulse server monitoring beforehand. Keep it stopped until independent checks confirm thaw, fresh writes on every backed-up filesystem and workload liveness, regardless of an OK backup result.
- **Some freezes have no detectable VM lock.** Cloning or disk imports can freeze a VM without detection. Reads already running before a backup lock may collide with thaw, so avoid manual guest-agent probes during freeze or thaw.
- **Kubernetes pauses need storage and controller checks.** Confirm persistent server storage and prevent controllers replacing stopped pods. Deleting a pod or stopping only the collector does not confirm server polling stopped.
- **Windows disk readings may remain unavailable.** Keep using the working Machines disk view without removing or reinstalling agents, restarting guests or forcing guest-agent checks solely to recover Proxmox disk readings.
- **Elevated server memory remains unresolved.** Keep the working setup and compare existing readings at similar uptimes and workloads. These changes do not establish a lower baseline or require a restart or heap dump solely for this report.
- **Previously affected machines still need caution.** Keep agents or Patrol disabled after a lockup. Preserve existing evidence privately without re-enabling them, changing GPU load or altering hardware to recreate the incident.
- **Resume only previously active monitoring.** Avoid hooks that ignore failed stops or restart Pulse after an unanswered thaw check. Restore only previously active services and timers after independent guest checks.
- **Agent updates still need a version check.** Compare the running agent's reported version with the intended release before treating an update as complete, especially for older installed agents.
- **Earlier alert-history gaps cannot be recreated.** Preserve existing logs and delivery records after storage failures. A healthy current history store does not establish recovery of previously missing entries.
- **Older ARMv6 agents need a manual update.** Their updater can still request ARMv7. Use the agent installer's saved-state update flow with the correct architecture while preserving the agent ID and connection state.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.7.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.7.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.7.0` and recreate the container.
