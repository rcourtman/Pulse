# Pulse v6.9.0-rc.1 Release Notes

This release candidate opens the `v6.9.0` candidate line after stable `v6.8.0`, which remains the rollback target.

Pulse v6.9.0 adds a map of your infrastructure, gives every plan access to metric history, and brings Compose service checks, logs and updates into Docker. It also adds notification rehearsals and shows when backup information is incomplete.

## Highlights

- Map in the dashboard header shows apps, machines and their network connections, with an option to save an image with names hidden.
- Docker → Applications groups Compose services by runtime and project, with service checks and recent failures beside each project.
- Metric history charts now offer up to 90 days of retained readings on every plan, including Community.

## What's improved

- **Proxmox node rows show hardware at a glance.** See the CPU model, thread count, installed RAM, fans and physical ports, with link speeds when reported. Disconnected ports dim when confirmed, and View can hide the strip.
- **Proxmox node charts include temperature.** Hover a node in History mode to see CPU, memory, disk and temperature together. Trends also graphs temperature and keeps its threshold colors.
- **Linked agents supply VM memory first.** Proxmox uses a current Pulse Agent reading and skips the guest memory file read. TrueNAS VM memory now agrees with the TrueNAS page by counting ZFS ARC as reclaimable cache.
- **Patrol Inbox shows the answer beside the issue.** Read the investigation conclusion and open its proposed fix for review from the same item. Action review feedback shows whether the action ran and where to see its result.
- **Organization settings keep verified details visible.** If the workspace directory fails, Settings keeps the selected workspace's saved members, billing details and outgoing shares. Unchecked counts and incoming shares show as unavailable.
- **The workspace switcher keeps your place during outages.** Retry the workspace list without losing your selection. A successful switch replaces live dashboard data and loads the new workspace's alert policy and delivery confirmations.
- **Organization Access keeps readable information during invitation outages.** Saved members and readable invitations stay visible, with changes paused until all lists load. Only workspace managers can read pending invitations.
- **Access forms are clearer by keyboard.** Name and member edits keep focus through saves and retries. Member fields support Enter, flag missing IDs and name immediate changes before submission.
- **User Access shows what saved rules do.** Rules show Allow or Deny and their conditions. Unchanged roles skip the write, submitted choices stay fixed, and errors or reloads keep focus with the affected role or user.
- **Sharing is easier to complete by keyboard.** Selectors distinguish identical names, and saved shares identify recipients, resources and access levels. Managers can press Enter in Create Share, and errors appear beside the relevant field.
- **SSO controls keep results in view.** Provider status uses text, test results stay visible while you edit, and deletion errors stay in the dialog. Read retries preserve keyboard context.
- **Token and audit controls keep errors in context.** Token errors stay in the edit dialog, and revocations and webhook removals restore focus. Audit Log filters remain usable while loading, and verification covers the page shown.

## Alerts and notifications

- **Rehearse alert delivery with saved routes.** Alerts → Notifications sends labelled alert and recovery messages through normal email, webhook and Apprise delivery, including grouping, quiet hours and retries.
- **See rehearsal results per destination.** Results distinguish accepted, failed, suppressed, pending and unknown outcomes, with transport details and seven days of history. Confirm receipt in your inbox or service.
- **Maintenance covers degraded connections and offline datastores.** Matching windows, including recurring ones, now cover Proxmox and PBS connection warnings and PBS datastore-offline alerts.
- **Reminders no longer look like flapping.** Reminders, escalations and delivery retries count as one firing, preventing steady alerts from being suppressed. Notification and Patrol consumers receive independent copies.
- **Finished container jobs can stay quiet.** Successful jobs under an hour without a restart expected skip offline alerts once the agent reports restart policy. Set Warn or Crit in Alerts → Thresholds → Application containers to include them.
- **Mute container alerts for one Docker runtime.** The host-level control leaves machine alerts and service checks on their own settings. Per-container controls also let you turn offline alerts off.
- **Mail Gateway queue alerts wait for confirmed recovery.** Partial counts or ambiguous ages cannot clear an active alert or downgrade a critical one. Alerts shows gateway-wide and node-specific Message Age warnings together.
- **Telegram text and ntfy recovery delivery work more reliably.** Telegram displays special characters and plain dashboard references without a URL. ntfy recovery messages include the verification and event headers expected by receivers.
- **Notification delivery uses less temporary memory.** Email attachments, tag routing and built-in webhook templates avoid repeated work. Apprise no longer holds command output in memory.
- **Alerting setup has a guide.** It covers routing, delivery checks, incident controls and an external watchdog. The API reference explains how to test a saved webhook or send a draft configuration.

## Disks and storage

- **Longer SMART intervals stay selectable.** Proxmox disk-health settings offer 3, 6, 12 and 24 hours, preserve saved custom values and keep the standby warning beside the selector.
- **Disk investigations can follow the largest folder.** Patrol and Assistant can drill down several levels on the same filesystem, report unreadable paths and suggest a directory read when a wildcard is refused.
- **Storage alerts point to the right place.** Patrol gets the full mount path and where to inspect it. PBS datastore capacity alerts open the datastore, and Proxmox guest filesystem alerts open the affected VM or LXC.

## Proxmox, PBS and backups

- **Backup health flags incomplete coverage.** The protection summary shows pending checks, unevaluated targets, running or failed jobs and unmatched backups. Backup Jobs keeps available counts and offers a retry for unreadable history.
- **Proxmox keeps failures beyond overlapping task pages.** Polling continues when a page overlaps but also contains new tasks. An overlapping scan cannot count as complete backup history.
- **Shared backup storage is checked separately.** A successful scan of same-named local storage cannot cover unread shared storage. Last-known files and PBS links remain visible without counting as fresh protection.
- **PBS preserves last-known failures and restore points.** Unreadable job lists keep sync, verify and prune failures visible, including after restart. Incomplete backup group listings keep prior restore points until a complete read.
- **PBS distinguishes unknown from unreachable.** Backup servers separates warnings and unknown states from unreachable servers, and unconfirmed datastore capacity stays unknown. Replication has an Unknown filter for unconfirmed sync outcomes.
- **VM snapshots show whether memory was saved.** Pulse uses the snapshot's reported memory flag and leaves missing or invalid values unconfirmed.
- **Proxmox node details restore custom readings.** Linked-agent readings show fresh values, including zero, and keep their status and error messages.
- **Stopped guests can start through a node agent.** After admin approval, VM and LXC Start can run on a confirmed node agent with commands enabled. Stop and restart still require an action runner.
- **Patrol checks why a guest stopped.** Manual and alert-triggered checks consult Proxmox tasks for deliberate stops and failed restarts. If task history is unreadable, check intent before approving Start.
- **Patrol can read inside running guests through their node agent.** It can still diagnose a guest when power actions are unavailable and explains whether commands, a confirmed agent or an action runner are missing.
- **AI backup reads find what is using space.** Ask Patrol or Assistant for a guest by name or VMID, a Proxmox storage or a PBS datastore. Counts, sizes and dates appear before the list, with incomplete totals marked.

## TrueNAS, vSphere and Docker

- **Compose service updates follow the deployment source.** An approved local update edits the registered Compose file and replaces one service. Git mode pushes a review branch for you to merge through your usual deployment workflow.
- **Compose deployments show what actually rolled out.** Service deployments reports the running image and fresh linked HTTP results separately. Registered projects use the source review for image changes.
- **Compose projects offer opt-in live logs.** In Docker → Applications, admins can follow the latest 30 lines from a selected container after enabling live logs on the host agent. Enable only where admins may read application logs.
- **Failed containers can show recent logs.** Enable failure logs when installing a host or rerun with --enable-failure-logs. The agent sends up to 30 lines and 4 KB from failed Docker or Podman containers and keeps your choice across updates.
- **Failure logs stay off by default.** Use --disable-failure-logs to turn them off. Masking can miss secrets, and AI reads send log text to your provider, so leave the option off for sensitive logs.
- **Docker health checks explain failures.** Details, State tooltips, alerts and Patrol show reasons such as DNS, HTTP, TLS, timeout or a missing command, without sending raw probe output.
- **Container updates avoid needless replacement.** Failed pulls and unchanged images leave the container alone. After an interrupted stop or rename, Pulse checks and reports whether the original container was restored.
- **Updates to stopped containers explain the result.** The action review says the replacement stays stopped and its health remains unchecked.
- **Home Assistant Supervisor stops getting false update alerts.** Pulse leaves Supervisor updates to Home Assistant and clears old alerts caused solely by its latest image alias.
- **TrueNAS respects selected collection scopes.** Pool, dataset and replication checkboxes work, and disabling datasets also skips snapshots. Remaining resources attach to a collected parent.
- **TrueNAS snapshots take less work to process.** On SCALE 25.04 and 25.10, fallback polling asks only for properties Pulse uses. Large snapshot inventories also use less temporary server memory when building restore points.
- **vSphere respects VM, host and datastore collection settings.** Skipped resources cannot fail the poll, empty views explain where to enable collection, and a rejected vCenter address leaves your draft available to correct.

## Install, updates and agents

- **Agent startup protects saved identity.** Unreadable credentials or IDs stop startup with a diagnostic. Restore unavailable storage before restarting. Read-only installs can keep using an explicit ID set by --agent-id or PULSE_AGENT_ID.
- **Enrollment confirms access was saved.** The agent must save its token and identity before authenticated startup. An incomplete success response stops instead of retrying a setup token that may have been consumed.
- **Agent installers prepare replacements before stopping services.** Windows stages the new executable, and Unix prepares the collector and optional helper at the destination. Preparation failures leave the running installation intact.
- **TrueNAS CORE agents start at boot again.** The standalone installer enables the service and reports failed stops, starts or unconfirmed running state.
- **APT reporting works without an action runner.** Linux agents can report pending package updates locally and flag incomplete output. Installing packages still requires the runner.
- **Settings → Updates explains delivery delays.** It shows when a release is still being prepared for your installation or the latest version cannot be confirmed. Changing channels refreshes results, and pending releases are checked sooner.
- **Automatic server updates require a confirmed newer version.** The installer refuses unknown versions, and the unattended updater rechecks after taking its lock so it skips stale offers and preserves a newly installed Pro binary.
- **Updater failures are clearer.** Help, cancellation and removal no longer claim an update completed. Unattended updates limit bootstrap downloads and journal waits and do less repeated service-manager work.
- **Proxmox registration has a deadline.** If the installer cannot confirm registration, check Pulse's node list before rerunning. Errors omit response details that could contain credentials.
- **Scripted first-run setup is documented.** The guide covers admin setup, agent enrollment and enabling alerts via the API. Supplied setup tokens need at least 32 characters, and install command errors list host alongside pve and pbs.
- **The MCP installer handles help and custom paths.** --help and -h print usage without installing. Unsupported arguments stop early, and custom install directories are prepared before downloading.
- **Local helper calls stop waiting on cancellation.** Setup and execution share a deadline, shutdown drains connections and temporary failures back off. Work already received by the helper may finish.

## Pulse Pro, AI and hosted

- **Patrol follows through on firing alerts.** In Ask first and Autopilot it opens a finding when an alert run files none, including sustained warnings it can help with. Manual checks start with the active alert and use current readings.
- **Findings match the actual resource and problem.** Separate guest problems get separate findings. Patrol corrects a guessed ID when a single resource matches and shows resource IDs in triage to distinguish namesakes.
- **Removed resources stop leaving open findings.** Patrol checks that a resource is gone before closing its findings. Same-named replacements get their own findings, and cleared alerts on removed resources stop causing failed runs.
- **Investigations turn written advice into reviewable actions.** Pulse asks once for a supported proposal or conclusion if the AI ends without recording one. The final turn can still propose a fix.
- **Patrol stays connected after monitoring reloads.** Adding connections or changing polling settings no longer disconnects alert-triggered investigations. Weekly summaries count verified fixes completed between scans.
- **Patrol explains stopped workloads.** It can flag a stopped service even without an alert and use exit codes or out-of-memory status to explain why it failed or needs human attention.
- **AI reads use the resource under investigation.** Reads default to the attached resource's host when it is clear which host to use. Unsupported host configuration queries steer toward file, log or disk reads.
- **MSP plans stay usable when some prices are missing.** Account marks missing plans or billing periods and offers Reload. Larger self-hosted fleets see a dismissible MSP option in Settings → Infrastructure.
- **Account verifies changes before showing them as saved.** If a client change cannot be confirmed, Account labels the last loaded details and offers a read-only refresh. Verification codes share a visible one-minute wait per email.
- **Hosted sessions open their own workspace.** Fresh client sign-ins land in their client workspace, and the empty default workspace is hidden.
- **Hosted billing follows current subscriptions.** New and resumed workspaces check current billing, missing billing can be retried, and delayed checkout or client edits cannot restore withdrawn access.
- **Hosted recovery waits for writers to stop.** Unverified recovery stays unavailable. Hosted lifecycle changes reject incomplete requests and return results without exposing pending invitations.

## Monitoring and service health

- **Map fits the available space.** It draws on first load, adjusts when banners appear and highlights connected tiles on hover or tap. Click a tile to open its details, or tap it again on touch screens.
- **Map uses collected network data.** Proxmox guests appear by bridge and VLAN even without IPs. Docker shows shared container networking and known Traefik routes, and keeps the LAN visible alongside macvlan networks.
- **Agents report more network details.** Resource data exposes Linux IPv4 default gateways and DNS servers, Docker network mode and Proxmox guest bridge, VLAN and firewall settings when available.
- **Docker suggests checks for published web ports.** Review host addresses and responding HTTP or HTTPS endpoints before adding checks. Pulse revalidates the reviewed ports and discards the offer when you switch organizations.
- **Docker shows when a running service stops responding.** State tooltips name failing attached checks. Checks follow a recreated container with the same name on the same host only when it still publishes the checked port.
- **TCP checks catch immediately closed connections.** A port that closes or resets immediately without sending data now fails, and silent connections take up to 250 ms longer. UDP exact matching now compares the whole reply.
- **The availability guide explains DNS monitoring.** Use an HTTP health endpoint that queries the resolver and verifies a known answer. The guide also covers local and remote probes and what each protocol can tell you.
- **History API ranges are clearer.** Metric and availability history use the same limits on every plan, up to 365 days per request where supported, with results limited to stored readings.
- **Large fleets do less repeated work.** Workload chart filters and snapshot discovery avoid copying unused inventory. Disk and sensor alert cleanup only processes relevant pending resources.

## Security

- **Host commands require execute access.** Assistant host reads, Kubernetes logs, Docker update scans and agent checks during action previews require permission to execute actions and the appropriate token scope.
- **Action reviews enforce your permissions.** Inbox fixes require permission to plan, approve and execute. Assistant planning needs plan permission, and investigating an existing finding again requires plan and execute permissions.
- **Automatic approval checks command arguments.** Options that change systems, expose process environments or read extra files require approval. Automatic log reads only cover files under /var/log, and environment overrides are filtered.
- **Recovery tokens require a saved, active recovery.** Pulse records creation and one-time use before granting access. An expired or ended recovery is rejected without consuming its token.
- **API token changes use current access.** Token writes recheck the caller's scopes and organization. Incomplete inventories preserve working tokens, restored files can reload with older timestamps, and failed saves preserve backups.
- **Old requests cannot disrupt a new sign-in.** Replies begun before a credential change cannot retry as the new user, sign them out or switch their workspace. Sharing, ownership and role changes also recheck access before applying.
- **Proxy rules identify clients more accurately.** Configure each trusted hop in PULSE_TRUSTED_PROXY_CIDRS or replace the forwarded client address at the final proxy. Same-host proxies using a loopback Host must send X-Forwarded-For.
- **Local setup and recovery check request origin.** Forwarded or cross-origin requests cannot qualify as direct local access. Security Overview describes the address Pulse observed for the current request.
- **Connection destinations have stricter limits.** Organization connections, SMTP and saved appliance polls reject local-only and metadata addresses. Pulse Cloud also blocks private addresses, while self-hosted LANs remain supported.
- **Connection and agent errors protect private details.** Appliance redirects stay on the configured host, agent certificate discovery checks permissions before probing, and refused redirects omit private destinations.
- **SSO protects pending sign-ins.** OIDC and SAML limit login input. OIDC checks custom CA files and keeps pending logins after incomplete callbacks, and SAML keeps pending logins during concurrent setup.
- **Security updates cover HTTP and telemetry dependencies.** The Go runtime fixes an HTTP request and response race, and OpenTelemetry enforces its attribute length limit.

## Other improvements

- **Saved sign-in files remain recoverable.** Private writes preserve unread password, session, CSRF and recovery records. Invalid file types no longer block startup waiting for a read.
- **Configuration restores protect other saves.** Settings writes wait for the restore to finish so a failed restore cannot discard an unrelated successful save.
- **CLI backups preserve the previous file.** An export replaces the file only after a complete private write. Passphrase prompts stay out of redirected archives and incomplete output reports failure.
- **CLI checks the data behind actions.** Missing fleet inventories report an error, and capabilities, history and confirmations must match the request. Check the action audit before retrying a change Pulse could not confirm.
- **Organization controls identify repeated names.** The workspace selector and Billing Admin show IDs when needed. Billing & Usage restores keyboard focus after a reload, and organization writes reject incomplete requests.
- **Settings controls stay reachable.** Change password scrolls on short screens, monitoring toggles show keyboard focus and Help stays closed if dismissed before it opens.
- **Agent observer files reject unsafe input.** Reads check file type, size and permissions. Conflicting settings are refused before they can affect saved credentials.
- **The telemetry preview shows added counts.** Optional telemetry counts hosts with commands or failure logs enabled, Map opens and image saves, and Patrol outcomes since upgrade. It sends no names, images or log text.

## Known issues

- **Guest freezes and Windows restarts remain unresolved.** If you stopped monitoring after a guest froze, do not restart it solely to test this beta or release.
- **Pause Pulse server monitoring before freeze-enabled backups.** Keep it stopped until independent checks confirm thaw, fresh writes on every backed-up filesystem and that the workload is running. An OK backup alone is insufficient.
- **Some guest freezes have no detectable VM lock.** Cloning or disk imports can trigger this, and reads already running before a backup lock can interfere with thaw. Avoid manual guest-agent probes during freeze or thaw.
- **Kubernetes pauses need storage and controller checks.** Confirm persistent Pulse server storage and prevent controllers from replacing stopped pods. Deleting a pod or stopping only the collector may leave server polling active.
- **Some Windows guests may still lack Proxmox disk readings.** Keep using the working Machines disk view. Avoid removing or reinstalling agents, restarting guests or forcing guest-agent checks solely to restore those readings.
- **Elevated Pulse server memory remains under investigation.** Keep your working setup and compare existing readings at similar uptimes and workloads. Skip restarts and heap dumps solely for this report.
- **Keep monitoring paused after a host lockup.** If you disabled the agent or Patrol, leave it off. Preserve existing evidence privately and avoid changing GPU load or hardware to reproduce the lockup.
- **Resume only previously active monitoring.** Avoid hooks that ignore failed stops or restart Pulse after an unanswered thaw check. Restore only the services and timers that were active before, after independent guest checks.
- **Verify the running agent version after updating.** Compare it with the intended release. Automatic collector updates can leave the privilege helper on an older version, which you can update through installer maintenance.
- **Earlier gaps in alert history cannot be recreated.** Preserve existing logs and delivery records after a storage failure, even once the history store is working again.
- **Older ARMv6 agents may require a manual update.** If the updater requests ARMv7, use the saved-state installer flow with the correct architecture and keep the agent ID and connection state.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.8.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.8.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.8.0` and recreate the container.
