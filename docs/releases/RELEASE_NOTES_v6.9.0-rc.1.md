# Pulse v6.9.0-rc.1 Release Notes

This release candidate opens the `v6.9.0` candidate line after stable `v6.8.0`, which remains the rollback target.

Pulse v6.9.0 opens longer metric history to every plan, brings Compose projects into the Docker dashboard, and lets you rehearse alert delivery through your saved routes. Proxmox and PBS keep more backup results visible when collection is incomplete, and agent installers prepare replacements before stopping running services.

## Highlights

- History charts show up to 90 days of retained metrics on every plan, including Community.
- Docker → Applications groups Compose services by runtime and project and shows their service checks and recent failures.
- Alerts → Notifications can send rehearsal alerts and recoveries through your saved email, webhook and Apprise routes.

## What's improved

- **Proxmox node rows show hardware at a glance.** See CPU model, thread count, installed RAM, fans and physical ports, with link speeds when reported. Confirmed disconnected ports dim, and the View menu can hide the strip.
- **Proxmox node charts include temperature.** Hover a node in History mode to see CPU, memory, disk and temperature together. Trends also graphs temperature and keeps its threshold colors.
- **Patrol Inbox puts the answer beside the issue.** Read the investigation conclusion and open the proposed fix from the same item. After approval, the review says what ran and where to see the result.
- **The workspace switcher keeps your place during outages.** Retry the workspace list without losing your selection. After a switch, Alerts loads the destination workspace’s policy and keeps delivery confirmations with their own workspace.
- **Organization Settings keeps verified details visible.** Overview, Access, Billing & Usage and Sharing retain the saved information they can still verify. Restart stalled reads in place. Changes pause where a required list is unavailable.
- **User Access keeps your place during saves and checks.** Rules show Allow or Deny and their conditions. Restart stalled roster and permission checks without losing your draft. New Role catches duplicate IDs and unchanged roles skip the save.
- **Organization member edits are easier by keyboard.** Press Enter to submit, jump to missing IDs and see which changes take effect immediately. After an unconfirmed invitation reply, Check again reads your access without resending the answer.
- **Sharing names the people and resources involved.** Selectors distinguish identical names and saved shares show recipients, resources and access levels. Managers can submit Create Share with Enter, and errors appear beside the relevant field.
- **SSO controls keep results in view.** Provider status uses text, test results stay visible while you edit, and deletion errors stay in the confirmation. A stalled provider check can be restarted in place.
- **Audit controls keep filters and drafts during recovery.** Audit Log filters stay usable while loading, and verification covers the displayed page. Restart stalled reads or check saved webhook destinations after an unconfirmed change without resending it.

## Alerts and notifications

- **Rehearsal results show what each destination did.** See whether a message was accepted, failed, was suppressed, is waiting or could not be verified. Recent results stay visible for seven days. Check your inbox or service to confirm receipt.
- **Maintenance windows cover more alerts.** Matching one-time and recurring windows now apply to Proxmox and PBS connection warnings and PBS datastore-offline alerts.
- **Reminders no longer count as flapping.** Reminders, escalations and retries count as one firing. Notification and Patrol listeners receive separate copies, so one listener cannot alter another’s alert.
- **Successful short container jobs can finish quietly.** Jobs that exit successfully within an hour and won’t restart skip offline alerts once the agent reports restart policy. Set Warn or Crit on the container to include it.
- **Mute container alerts for one Docker runtime.** The runtime control leaves machine alerts and service checks on their own settings. Container rows also let you set their offline alert level or turn it off.
- **Mail Gateway queue alerts wait for confirmed recovery.** Partial counts and ambiguous message ages cannot clear an alert or downgrade a critical one. Alerts shows gateway-wide and node-specific Message Age warnings together.
- **Built-in Telegram messages handle special characters and long groups.** Pulse splits oversized groups, shortens single messages when needed and waits when rate-limited. Recoveries link to Alert History when a Pulse URL is configured.
- **ntfy recovery messages include the expected verification and event headers.** Receivers can verify recoveries the same way they verify firing notifications.
- **Notification delivery does less work.** Email attachments, tag routing and built-in webhook templates use less temporary memory. Apprise command output is drained as it arrives.
- **Help explains how to check alert delivery.** The alerting guide covers routes, reminders, incident controls and an external watchdog. The API guide explains how to test a saved webhook or a draft destination.

## Disks and storage

- **Proxmox offers longer SMART polling intervals.** Disk-health settings offer 3, 6, 12 and 24 hours and preserve existing custom intervals. The standby warning stays beside the selector.
- **Machines explains critical Unraid array alerts on the row.** A collapsed row shows the severity and reason for the affected machine, including when an assigned disk is missing.
- **Storage alerts point to the affected resource.** PBS capacity alerts open the datastore and Proxmox filesystem alerts open the affected VM or LXC. Patrol gets the full mount path and where to inspect it.
- **AI disk investigations can follow the largest folder.** Patrol and Assistant can drill down several levels on the same filesystem, report unreadable paths and suggest a directory read when a wildcard is refused.

## Proxmox, PBS and backups

- **Backup health flags incomplete coverage.** The protection summary shows pending checks, unevaluated targets, running or failed jobs and unmatched backups. Backup Jobs keeps available counts and offers a retry when history can’t be read.
- **Proxmox keeps results from incomplete task scans.** Overlapping pages with fresh tasks are still read. A shorter log, a later timeout or a subsequent empty task list leaves already observed guest successes and failures in history.
- **Backup Jobs ages out old Running results.** A guest becomes Unknown after five minutes without fresh evidence. When the parent job ends without a guest end time, Coverage leaves that guest’s duration unreported.
- **Shared backup storage is checked separately.** A scan of same-named local storage cannot cover unread shared storage. Last-known files and PBS links stay visible without counting as fresh protection.
- **PBS preserves last-known failures and restore points.** Unreadable job lists keep sync, verify and prune failures visible across restarts. Incomplete backup group listings keep previous restore points until a complete read.
- **PBS distinguishes unknown from unreachable.** Backup servers separates warning and unknown states from confirmed connection failures. Unconfirmed datastore capacity stays unknown, and Replication has an Unknown filter for unconfirmed outcomes.
- **VM snapshots show whether memory was saved.** Pulse displays the reported memory flag and leaves missing or invalid values unconfirmed.
- **Proxmox node drawers restore linked-agent readings.** Current custom readings appear again, including zero values, with their status and errors.
- **Proxmox prefers current memory from the VM’s linked agent.** Pulse skips the guest memory file read when it can use that sample, even if Proxmox VM status is unavailable. TrueNAS ZFS ARC counts as reclaimable cache.
- **Stopped guests can start through a confirmed node agent.** An admin can approve VM or LXC Start on a supported Proxmox systemd agent with commands enabled. Stop and restart still require an action runner.
- **Patrol checks why a guest stopped.** Manual and alert-triggered checks consult Proxmox tasks for deliberate stops and failed restarts. If task history is unreadable, check why the guest stopped before approving Start.
- **AI backup reads summarize what is using space.** Ask for a guest by name or VMID, Proxmox storage or a PBS datastore. Counts, sizes and dates appear before the list, with incomplete totals marked.

## TrueNAS, vSphere and Docker

- **Compose updates follow the registered deployment source.** An approved local update edits the Compose file and replaces one service. Git mode pushes a review branch you can merge through your normal deployment process.
- **Service deployments shows what rolled out.** Pulse compares the running image with the reviewed image and shows the result of fresh linked HTTP checks separately.
- **Compose projects offer opt-in live logs.** Admins can follow the latest 30 lines from a selected container after enabling live logs on the Docker agent. Enable this where admins can read application logs.
- **Failed containers can report recent logs.** Enable failure logs during agent setup or with `--enable-failure-logs`. Pulse collects up to 30 lines and 4 KB per Docker or Podman failure. `--disable-failure-logs` turns it off.
- **Failure logs are off by default.** Masking can miss secrets, and logs can reach your AI provider when Patrol or Assistant reads the container. Leave this option off for sensitive logs.
- **Docker health checks explain failures.** Container details, State tooltips, alerts and Patrol show causes such as DNS, HTTP, TLS, timeout or a missing command. Raw probe output stays on the host.
- **Container updates avoid needless replacements.** Failed pulls and unchanged images leave the container alone. After an interrupted stop or rename, Pulse checks whether the original was restored. A stopped container’s replacement stays stopped.
- **Home Assistant Supervisor stops getting false update alerts.** Pulse leaves Supervisor updates to Home Assistant and clears old alerts caused solely by the `latest` image alias.
- **TrueNAS honors all three collection switches.** Pool, dataset and replication choices stay saved even with all three off. Turning off datasets also skips snapshots. System, disk and native alert readings continue.
- **TrueNAS snapshot polling does less work.** The fallback on SCALE 25.04 and 25.10 requests only the properties Pulse uses. Large snapshot lists also use less temporary server memory when building restore points.
- **vSphere honors VM, host and datastore collection choices.** Skipped resources can’t fail a poll. Empty views explain how to enable collection, and a rejected vCenter address leaves the connection draft open for correction.

## Install, updates and agents

- **Agent startup protects saved identity.** Unreadable credentials or IDs stop startup with a diagnostic. Restore unavailable storage before restarting. Read-only installs can still use `--agent-id` or `PULSE_AGENT_ID`.
- **Enrollment confirms that access was saved.** The agent saves both its token and identity before authenticated startup. An incomplete response stops the attempt so the agent doesn’t resend a setup token that may have been used.
- **Agent installers prepare replacements before stopping services.** Windows stages the executable and Unix prepares the collector and optional helper at the destination. A preparation failure leaves the running installation available.
- **TrueNAS CORE agents start at boot again.** The standalone installer enables the service and reports failed stops, starts or an unconfirmed running state.
- **APT reporting works without an action runner.** Linux agents report pending package updates locally and flag incomplete results. Installing packages still requires the runner.
- **Settings → Updates explains delivery delays.** See when a release is being prepared for your installation or when Pulse can’t confirm the latest version. Changing channels refreshes the result, and pending releases are checked sooner.
- **Automatic server updates require a confirmed newer version.** The unattended updater rechecks the installed version after it gets the update lock, so it skips stale offers and preserves a newly installed Pro binary.
- **Updater results reflect what actually happened.** Help, cancellation and removal no longer claim a successful update. Unattended updates also cap bootstrap downloads and journal waits and make fewer service-manager calls.
- **Proxmox setup stops waiting when registration times out.** Check the node list before rerunning the installer. The diagnostic omits response details that could contain credentials.
- **The install guide covers scripted first setup.** Set up the admin, enroll a host and activate alerts through the API. A supplied API token needs at least 32 characters, and install command errors now list `host` alongside `pve` and `pbs`.
- **The MCP installer handles help and custom paths.** `--help` and `-h` print usage without installing. Unknown arguments stop early, and custom install directories are prepared before download.
- **Local helper waits respect cancellation.** Setup and execution share a deadline, temporary failures back off, and shutdown lets connections finish. Work the helper already received may still complete.

## Pulse Pro, AI and hosted

- **Patrol follows up on firing alerts.** In Ask first and Autopilot, it opens a finding when an alert run records none, including sustained warnings it can investigate. Manual checks use fresh readings and name active alerts even without a finding.
- **Findings follow the actual resource and problem.** Separate problems on one guest get separate findings. Patrol corrects guessed IDs when one resource matches and shows IDs in triage to distinguish same-named resources.
- **Removed resources stop leaving findings open.** Patrol confirms removal before closing a finding, gives same-named replacements their own findings and avoids failed runs for cleared alerts on removed resources.
- **Investigations can turn written advice into a reviewable fix.** Pulse asks once for a supported proposal or conclusion when the AI stops without recording one. The final turn can still propose a fix.
- **Monitoring reloads keep Patrol connected to alerts.** Adding connections or changing polling settings no longer stops alert-triggered investigations. Weekly summaries count verified fixes completed between scans.
- **Patrol explains workloads that stopped without an alert.** It can flag a stopped service and use its exit code or out-of-memory status to explain what happened and whether someone needs to intervene.
- **AI reads use the resource under investigation.** Reads default to its host when the target is clear. Patrol can read inside running Proxmox guests through a confirmed node agent and explains when missing commands or a runner block a power action.
- **MSP plan choices stay visible when prices are missing.** Account flags unavailable plans or billing periods and offers Reload. Larger self-hosted fleets see a dismissible MSP option in Settings → Infrastructure.
- **Account verifies client changes before showing them as saved.** If a change can’t be confirmed, it labels the last loaded details and offers a read-only refresh. Verification code requests share a visible one-minute wait per email address.
- **Hosted sign-ins open the client’s own workspace.** Fresh client sessions land in their workspace, and the empty default workspace stays hidden.
- **Hosted billing follows the current subscription.** New and resumed workspaces check current billing. Missing billing can be retried, and delayed checkout or client edits can’t restore withdrawn access.
- **Hosted recovery waits for existing processes to stop writing.** Recovery remains unavailable while Pulse can’t verify that state. Workspace changes reject incomplete requests and keep pending invitations private.

## Monitoring and service health

- **Docker suggests checks for published web ports.** Review the responding HTTP or HTTPS endpoints before adding checks. Pulse verifies the reviewed ports again and clears the offer when you switch workspaces.
- **Docker shows when a running service stops responding.** State tooltips name failing attached checks. A check follows a recreated container on the same host when its name and checked port still match.
- **Compose-related failures appear together in Alerts.** Container failures and attached service-check failures can appear in the same incident, with each check keeping its own notification settings.
- **TCP checks catch ports that close right away.** A port that closes or resets immediately without sending data now fails. Silent connections can take up to a quarter-second longer. UDP exact matching checks the whole reply.
- **Availability checks identify the device being checked.** Label checks for routers, switches, access points, machines or services. Alert links open the matching check, and Fleet moves keyboard focus to the next available control if a check disappears.
- **The availability guide explains DNS checks.** Use an HTTP health endpoint that queries the resolver and verifies a known answer. The guide covers local and remote probes and what each protocol can verify.
- **History reads use the same ranges across plans.** The metric and availability APIs serve supported ranges up to 365 days per request, subject to stored readings. Ending a paid plan no longer triggers deletion of saved history or generated reports.
- **Resource APIs expose more network details.** New agent reports can include Linux IPv4 default gateways and DNS servers, Docker network mode, and Proxmox guest bridge, VLAN and firewall settings when available.
- **Large fleets do less repeated work.** Workload chart filters and snapshot discovery skip unused inventory copies. Disk and sensor alert cleanup processes only the relevant pending resources.

## Security

- **Host commands require execute access.** Assistant host reads, Kubernetes logs, Docker update scans and agent checks during action previews require permission to execute actions and the appropriate token scope.
- **Action reviews enforce your role.** Inbox fixes require plan, approve and execute permissions. Assistant planning needs plan permission, and asking Patrol to investigate an existing finding again needs plan and execute permissions.
- **Automatic command approval checks arguments.** Options that change the system, expose process environments or read extra files require approval. Automatic log reads are limited to `/var/log`, and environment overrides are filtered.
- **Recovery tokens must be active and saved before use.** Pulse records creation and one-time use before granting access. An expired or ended recovery leaves its token unused.
- **API token changes use current access.** Writes recheck the caller’s scopes and workspace. Incomplete inventories preserve working tokens, restored files can reload with older timestamps, and failed saves retain recovery copies.
- **Old requests can’t disrupt a new sign-in.** Replies from an earlier session can’t sign out the new user or switch their workspace. Sharing, ownership and role changes recheck access before applying.
- **Proxy rules identify clients more accurately.** Add every trusted hop to `PULSE_TRUSTED_PROXY_CIDRS` or have the final proxy replace the forwarded client address. Same-host proxies using a loopback Host must send `X-Forwarded-For`.
- **Local setup and recovery check request origin.** Forwarded and cross-origin requests can’t qualify for direct local access. Security Overview shows the address Pulse observed for the current request.
- **Workspace connections have stricter address limits.** Appliance connections and SMTP reject local-only and metadata addresses. Pulse Cloud also blocks private addresses. Self-hosted LAN connections remain supported.
- **Connection errors protect private details.** Appliance redirects stay on the configured host, certificate discovery checks registration permission before connecting, and redirect refusal messages omit private destinations.
- **SSO protects pending sign-ins.** OIDC and SAML limit login input. OIDC checks custom CA files and preserves pending logins after incomplete callbacks. Concurrent SAML setup preserves pending sign-ins.
- **HTTP and telemetry dependencies received fixes.** The Go runtime fixes a race in HTTP handling, and OpenTelemetry enforces its attribute length limit.

## Other improvements

- **Saved access files remain recoverable.** Writes preserve unread password, session, CSRF and recovery records. Unsupported file types in sign-in or organization data return an error instead of holding up startup or settings.
- **Configuration restores protect concurrent saves.** Other settings writes wait for the restore to finish so a failed restore can’t erase a successful save.
- **CLI exports preserve your previous backup.** The destination file is replaced only after a complete private write. Prompts stay out of redirected archives, and incomplete output reports failure.
- **CLI actions check the data behind their results.** Missing fleet inventories return an error. Capabilities, history and confirmations must match the requested action or resource. Check the audit record before retrying an unconfirmed action.
- **Settings controls stay reachable by keyboard.** Sign-in keeps focus and shows when a request is pending. Token errors stay in the dialog, revocations and webhook removals restore focus, and Change password scrolls on short screens.
- **Workspace and Billing Admin controls distinguish repeated names.** IDs appear where needed, and billing reloads restore focus to the affected row.
- **Monitoring toggles and Help respond predictably.** Toggles show keyboard focus, and Help stays closed if you dismiss it before it finishes opening.
- **Private agent settings reject unsafe files.** Reads check file type, size and permissions, and conflicting settings are refused before they can affect saved credentials.
- **The optional telemetry preview shows more aggregate counts.** It includes how many hosts enable commands or failure logs and how Patrol investigations end after upgrade. Names and log text stay out of these counts.

## Known issues

- **Guest freezes and Windows restarts remain unresolved.** If you stopped monitoring after a guest froze, do not restart it solely to test this beta or release.
- **Pause Pulse server monitoring before freeze-enabled backups.** Keep it paused until independent checks confirm thaw, fresh writes on every backed-up filesystem and a running workload. An OK backup alone is insufficient.
- **Some freezes occur without a detectable VM lock.** Cloning and disk imports can freeze a guest, and a read already in progress when a backup lock appears can interfere with thaw. Avoid manual guest-agent probes during freeze or thaw.
- **Resume only the monitoring services and timers that were active before the pause.** Avoid hooks that ignore failed stops or restart Pulse after a failed or unanswered thaw check.
- **Kubernetes pauses require checking storage and controllers.** Confirm persistent Pulse server storage and prevent controllers from replacing stopped pods. Deleting a pod or stopping just the collector can leave server polling active.
- **Some Windows guests may still lack disk readings in Proxmox.** Keep using the working Machines disk view. Avoid removing or reinstalling agents, restarting guests or forcing guest-agent checks solely to restore the readings.
- **Elevated Pulse server memory remains under investigation.** Keep your working setup and compare existing readings at similar uptimes and workloads. Skip restarts and heap dumps solely for this report.
- **Keep monitoring paused after a host lockup.** Leave agents or Patrol off if you disabled them. Preserve existing evidence privately and avoid changing GPU load or hardware to reproduce the lockup.
- **Check the running agent version after an update.** Automatic updates can leave the privilege helper on an older version. Use installer maintenance if you need a helper fix, keeping the existing agent enrollment.
- **Older ARMv6 agents may need a manual update.** If the updater asks for ARMv7, use the installer with the correct architecture and retain the saved agent ID and connection state.
- **Earlier gaps in alert history can’t be recreated.** Keep existing logs and delivery records after a storage failure, even after the history store is working again.
- **A TrueNAS CORE data pool can be online with capacity unavailable.** If Usage / Size shows a dash, check the same pool in TrueNAS Storage. Keep the existing key and permissions. Dataset figures can reflect quotas and can’t substitute for pool capacity.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.8.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.8.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.8.0` and recreate the container.
