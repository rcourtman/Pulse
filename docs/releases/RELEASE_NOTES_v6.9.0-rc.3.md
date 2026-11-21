# Pulse v6.9.0-rc.3 Release Notes

This release candidate follows `v6.9.0-rc.2` on the way to stable `v6.9.0`. Stable `v6.8.0` remains the rollback target.

Pulse v6.9.0 opens longer chart history to every plan, groups Compose projects in Docker, and lets you rehearse alert delivery. Proxmox and PBS keep backup results visible through incomplete polls, and agent setup and updates explain where they got stuck.

## Highlights

- History charts show up to 90 days of retained metrics on every plan, including Community.
- Docker → Applications groups Compose services by runtime and project and shows their checks and recent failures.
- Alerts → Notifications sends rehearsal alerts and recoveries through saved email, webhook and Apprise routes.

## What's improved

- **Patrol Inbox puts the conclusion beside the finding.** Open the proposed fix from the same item. The review names the target and shows what ran and where to find the result.
- **Workspace switching keeps your place.** Retry a failed list in place and use IDs to tell same-named workspaces apart. Alerts loads the selected workspace's policy and keeps confirmations with the workspace that sent them.
- **Organization Settings keeps saved details visible.** Restart stalled checks in Overview, Access, Billing & Usage and Sharing. Confirmed details stay visible, dependent controls pause when their data is missing, and long names wrap in Overview.
- **Organization Access shows when invitations are empty.** A confirmed empty list is distinct from a pending or failed check. Member fields identify missing IDs, support Enter and explain which edits take effect immediately.
- **Invitation replies can be checked without sending them again.** Invitees can restart a stalled check. After an uncertain reply, Check again reads saved access and returns keyboard focus to the result.
- **Sharing identifies both parties and the resource.** Incoming and outgoing shares show access and confirmed decisions, even if a refresh fails. Create Share supports Enter and points to the field that needs correction.
- **Roles and User Access keep drafts through stalled reads.** Restart role, roster and permission checks in place. Rules show Allow or Deny with conditions, and saves or refusals return focus to the relevant control.
- **Role editing catches duplicates and skips unchanged saves.** New Role rejects an existing ID. Permission edits keep focus, and removing a permission moves it to a remaining control.
- **Audit Log keeps details and controls in reach.** The Event cell includes who acted and when on narrow screens. Filters stay usable while results load, verification applies to the displayed page, and stalled loads can be restarted.
- **Audit Webhooks can confirm an uncertain change.** Check saved destinations without resending the change, or restart a stalled read. After removal, focus returns to a nearby endpoint.
- **API Access keeps the affected token readable.** Long names wrap in scope and revocation dialogs, and errors stay with the open dialog. Restart a stalled list and keep your place after revoking.
- **Help clarifies HTTPS and agent URL checks.** The systemd example uses the configured port and explains which settings take precedence. Plain HTTP needs a local or private address, even with `--insecure`.

## Alerts and notifications

- **Rehearsal reports explain each destination's result.** See what was accepted, failed, suppressed, waiting or unknown for seven days, even after restart. Confirm receipt in the receiving service.
- **More alerts respect maintenance windows.** Matching one-time and recurring windows cover Proxmox and PBS connection warnings and PBS datastore-offline alerts.
- **Reminders no longer inflate flapping counts.** Reminders, escalations and retries belong to the original firing. A problem in one alert listener can't change or drop another listener's alert.
- **Finished container jobs can stay quiet.** With an agent that reports restart policy, jobs that exit successfully in under an hour and won't restart skip offline alerts. Set Warn or Crit on a container to include it, or Off to mute it.
- **Mute every container on one Docker runtime.** Alerts → Thresholds → Docker → Container Runtimes covers current and future containers. Monitoring, runtime-offline alerts, Swarm services and endpoint checks have separate controls.
- **Mail Gateway queue alerts wait for confirmed recovery.** Partial counts and unclear ages no longer clear alerts or downgrade critical ones. Alerts shows gateway-wide and node-specific Message Age warnings together.
- **Built-in Telegram handles long groups and special characters.** Pulse escapes alert text, splits or shortens long messages and waits when rate-limited. Recoveries link to Alert History when a Pulse URL is configured.
- **ntfy recoveries include authentication and event headers.** Receivers can authenticate and identify recoveries the same way they handle firing notifications.
- **Delivery uses less temporary memory.** Email attachments, tag routing and built-in webhooks allocate less, and Apprise output is drained as it arrives.
- **Help shows how to check delivery.** The alerting guide covers routes, reminders, incidents and an external watchdog. The API guide shows how to test a saved webhook or draft destination.

## Disks and storage

- **Proxmox adds longer SMART polling intervals.** Disk-health settings offer 3, 6, 12 and 24 hours and preserve custom values. The drive standby warning stays by the selector.
- **Machines names critical Unraid array problems.** The collapsed row shows the severity and reason on the affected host, including when an assigned disk is missing.
- **Storage alerts open the affected resource.** PBS capacity alerts open the datastore, and Proxmox filesystem alerts open the VM or LXC. Patrol gets the full mount path and where to inspect it.
- **AI can follow the largest folder on a filesystem.** Patrol and Assistant can drill down several levels on the same mount, report unreadable paths and suggest a directory read if a wildcard needs approval.
- **TrueNAS CORE shows each disk's own read and write history.** Disk History keeps paired samples together and identifies appliance readings, readings held in Pulse memory, or a mix. Charts stay reachable in narrow drawers.
- **TrueNAS CORE host Disk I/O fetches only the missing disk graphs.** Stored CPU, memory, temperature and network readings can load independently. Use TrueNAS Reporting if the host total is blank because a reporting disk is missing a sample.

## Proxmox, PBS and backups

- **Proxmox drawers show node hardware and linked readings.** See CPU, thread count, RAM, fans and ports with speed, link state and last-known labels. Current custom readings appear again, including zeros and collection errors.
- **Node History adds temperature to hover charts.** Hover over a node to see CPU, memory, disk and temperature. Temperature sparklines in Trends keep their threshold colors.
- **Backup health shows incomplete coverage.** See pending and unevaluated targets, running or failed jobs and unmatched backups. Backup Jobs offers a retry when history fails to load and keeps the counts it has.
- **Proxmox backup polls preserve observed guest results.** Overlapping pages still contribute new tasks. Shorter logs, later timeouts and task-list interruptions preserve observed successes and failures where evidence is still current.
- **Backup Jobs expires stale Running results.** After five minutes without fresh evidence, the guest becomes Unknown. Coverage shows each guest's result and leaves duration blank if Pulse only knows when the parent job ended.
- **Protection dates exclude incomplete archives.** An archive still being written can't make a guest look protected. Unread shared storage keeps last-known backups and PBS links even if a same-named local store was scanned.
- **PBS keeps last-known failures and restore points.** Unread job lists preserve sync, verify and prune failures across restarts. Incomplete backup group lists keep earlier restore points until a complete read.
- **Backup Servers and Replication show uncertainty.** Backup Servers separates warning, unknown and unreachable states and leaves unconfirmed capacity unknown. Replication adds an Unknown filter for unconfirmed sync outcomes.
- **VM snapshots show whether memory was saved.** Pulse uses the flag reported by Proxmox and marks the result unconfirmed if the flag is missing or invalid.
- **VM memory prefers a current reading from its linked Pulse Agent.** It works even when Proxmox VM status is unavailable and skips the guest memory file read. See the source and time under Overview → Details → Memory reading.
- **Patrol and action reviews can start stopped VMs and LXCs.** An admin approves Start through a confirmed systemd node agent with commands enabled. The action runner is still required for stop or restart.
- **Patrol checks how a guest stopped.** Manual and alert-triggered checks read Proxmox tasks for deliberate stops and failed restarts. If history is unreadable, check why the guest stopped before approving Start.

## TrueNAS, vSphere and Docker

- **TrueNAS honors the pool, dataset and replication switches.** All three can stay off. Disabling datasets also skips snapshots, while system, disk and native alert readings continue.
- **TrueNAS snapshots take less work to poll.** SCALE 25.04 and 25.10 fallback queries ask only for the properties Pulse uses, and large snapshot inventories use less temporary memory to build restore points.
- **TrueNAS separates reclaimable ZFS cache from free memory.** Open the Memory bar in TrueNAS → Overview for the ARC breakdown. Linked Pulse Agent readings account for ARC when used for Proxmox VM memory.
- **Slow TrueNAS host History reads can finish.** Refreshes let requests run for up to 150 seconds. Changing range or leaving cancels abandoned CORE REST work without interrupting SCALE polling.
- **Agent profiles list TrueNAS hosts with an installed agent.** NAS systems connected through their native API remain available for monitoring and History.
- **vSphere honors VM, host and datastore collection choices.** Skipped inventories can't fail a poll. Empty views explain how to enable collection, and rejected vCenter addresses leave the draft open for correction.
- **Reviewed Compose updates use your registered source.** An admin-approved local update edits the Compose file and replaces one service. Git mode publishes a review branch for your normal deployment process.
- **Service deployments shows what rolled out.** Pulse compares the running image to the reviewed image and shows fresh linked HTTP check results separately.
- **Compose admins can opt in to live logs.** Docker → Applications follows the latest 30 lines from a selected container after live logs are enabled on its agent. Use it only where administrators may read application logs.
- **Docker and Podman can show failure logs.** Opt in under Install on a host or with `--enable-failure-logs` for up to 30 lines and 4 KB per failure. Leave it off for sensitive logs since masking can miss secrets and AI can send logs to your provider.
- **Docker health checks explain failures.** Container details, State tooltips, alerts and Patrol show causes such as DNS, HTTP, TLS, timeout or a missing command. Raw probe output stays on the host.
- **Container updates protect the original.** Failed pulls or unchanged images leave it in place. After an interrupted stop or rename, Pulse checks whether it was restored. A replacement for a stopped container stays stopped.

## Install, updates and agents

- **Agent startup protects saved identity.** Unreadable credentials or IDs stop startup with recovery guidance. Restore unavailable storage before restarting. Read-only installs can set an ID with `--agent-id` or `PULSE_AGENT_ID`.
- **Enrollment confirms that identity and credentials were saved.** An incomplete enrollment reply stops the attempt because the setup token may already have been used.
- **Agent installers prepare replacements before stopping services.** Windows stages the executable. Unix checks the collector at its destination and readies the optional helper. A preparation failure leaves the existing service available.
- **Unix installers distinguish launch failures from URL refusals.** Preflight checks reachability and download availability, then the downloaded agent checks the address. TrueNAS SCALE can run that check from its install filesystem.
- **TrueNAS CORE agents start at boot again.** The standalone installer enables the service and reports failed stops or starts, or when it can't confirm the service is running.
- **APT reports pending updates without an action runner.** Linux agents check locally and mark incomplete results. Installing packages still requires the runner.
- **Settings → Updates explains delivery delays.** See when a release is being prepared for your installation or when Pulse can't confirm the latest version. Channel changes refresh the result and pending releases are checked sooner.
- **Automatic server updates require a confirmed newer Community version.** The updater rechecks what is installed after getting its lock, skips stale offers and protects an installed Pro binary.
- **Manual server updates protect the installed edition.** Public updates require a confirmed Community install. For Pro, use the matching private image or Linux archive. Run `/bin/update` only if the Pulse installer provided it.
- **Updater results say what happened.** Help, cancellation and removal stop reporting success. Unattended updates limit downloads and journal waits, and Proxmox registration times out with advice to check the node list before trying again.
- **Install Help covers scripted first setup.** Create the admin, enroll a host and activate alerts through the API. Supplied setup API tokens need at least 32 characters, and installer usage lists the host option.
- **The MCP installer handles help and custom paths.** Help prints usage without installing, unknown options stop early, and custom install directories are prepared before downloading.

## Pulse Pro, AI and hosted

- **Patrol follows up on firing alerts.** In Ask first and Autopilot, a run that records no finding can still open one from a firing alert. Manual checks use fresh readings, address the alert first and avoid calling an alerting resource healthy.
- **Findings follow the resource and the problem.** Separate issues on one guest get separate findings. Patrol corrects guessed IDs when one resource matches and shows IDs in triage to distinguish same-named resources.
- **Removed resources stop leaving findings open.** Patrol confirms absence, investigates same-named replacements separately and avoids failed runs for cleared alerts on removed resources.
- **Written advice can become a reviewable fix.** If an investigation ends with advice alone, Pulse asks once for a supported proposal or a conclusion. The final turn can still propose a fix.
- **Patrol stays connected when monitoring reloads.** Adding connections or changing polling settings keeps alert investigations working. Weekly summaries include confirmed fixes completed between scans.
- **Patrol can explain why a workload stopped without an alert.** It can flag stopped services and use exit codes or out-of-memory status to explain what happened and whether someone needs to intervene.
- **AI reads default to the resource under investigation.** Patrol can read inside a running Proxmox guest through a confirmed node agent. Power-action refusals explain when commands are off, the agent is old or unconfirmed, or a runner is needed.
- **AI backup reads summarize space before listing files.** Ask about a guest by name or VMID, Proxmox storage or a PBS datastore. See counts, sizes and dates with incomplete totals marked.
- **MSP plans stay visible if some prices are missing.** Account marks unavailable plans or billing periods and offers Reload. Larger self-hosted fleets see a dismissible MSP option under Settings → Infrastructure.
- **Account preserves client details after an uncertain change.** Refresh to confirm the saved result without resending it. Sign-in code requests for the same email share a visible one-minute wait and explain uncertain delivery.
- **Hosted clients land in their own workspace.** The empty default workspace stays hidden. New and resumed workspaces recheck billing, with a retry if it's missing, and delayed checkout or edits can't restore withdrawn access.
- **Hosted recovery waits for old processes to stop writing.** Workspace changes require complete requests, and pending invitations are kept out of hosted lifecycle replies.

## Monitoring and service health

- **Docker suggests availability checks for published web ports.** Review responding HTTP or HTTPS addresses before adding checks. Pulse checks them again before saving and clears offers on workspace changes.
- **Docker shows when a running service stops responding.** State tooltips name failing checks. A check follows a recreated container on the same host when its name and checked port still match.
- **Related Compose failures appear together in Alerts.** Container failures and linked service-check failures can share an incident. Each check keeps its own notification settings.
- **Home Assistant Supervisor avoids false Docker update alerts.** Pulse clears alerts caused solely by the `latest` image alias and leaves Supervisor updates to Home Assistant.
- **Availability checks catch TCP ports that immediately close.** A port that closes or resets before sending data fails. Silent connections can take up to a quarter-second longer. Exact UDP matching checks the entire reply.
- **Availability checks identify what they monitor.** Label routers, switches, access points, machines or services. Alerts opens the matching check, and Fleet moves focus to an available control if a check disappears.
- **Help explains how to check DNS answers.** Use an HTTP health endpoint that asks the resolver for a known answer. The guide covers what each protocol can verify and where probes can run.
- **History APIs and MCP read retained data on every plan.** Metrics and availability APIs support up to 365 days per request, subject to retention. Ending a paid plan no longer deletes saved history or generated reports.
- **Resource APIs include more network details.** Updated agents can report Linux IPv4 default gateways, DNS servers and Docker network mode. Proxmox guests can include bridge, VLAN and firewall settings.
- **Health shows stalled dashboard updates on standalone Pulse.** A saved `/api/health` response can show `websocketStateStalled` after 30 seconds while liveness stays healthy. If reporting a stall, share the flag and time.
- **Local helper calls stop waiting on cancellation.** Setup and execution share a deadline, temporary failures back off, and shutdown lets connections drain. Work the helper already received can still finish.
- **Large fleets do less repeated work.** Startup does less database work, and chart filters, snapshot discovery, disk and sensor alert cleanup and Proxmox backup polls process less data.

## Security

- **Host commands require execute access.** Assistant host reads, Kubernetes logs, Docker update scans and agent checks during action previews require execution permission and the appropriate token scope.
- **Action reviews enforce your role.** Inbox fixes require plan, approve and execute permissions. Assistant planning requires plan permission, and starting another investigation of an existing finding requires plan and execute permissions.
- **Automatic command approval checks arguments.** Options that change systems, expose process environments or read extra files require approval. Automatic log reads stay inside `/var/log`, and Pulse filters environment overrides.
- **Recovery tokens must be active and saved.** Pulse records creation and one-time use before granting access. An expired or ended recovery leaves the token unused.
- **API token changes check current access.** Writes recheck the caller's scopes and workspace. Incomplete inventories preserve live tokens, restored older files can reload, and failed saves keep recovery copies.
- **Old replies can't disrupt a replacement sign-in.** Requests from earlier sessions can't sign the current user out or switch their workspace. Sharing, ownership and role changes recheck access before applying.
- **Client addresses follow the configured proxy chain.** List every trusted hop in `PULSE_TRUSTED_PROXY_CIDRS` or have the final proxy replace the forwarded address. Same-host proxies using a loopback Host need `X-Forwarded-For`.
- **Local setup and recovery check request origin.** Forwarded and cross-origin requests can't use direct local access. Security Overview shows the observed address, and shared hosted networks trust only configured ingress.
- **Workspace connections enforce destination limits.** Appliance connections and SMTP reject local-only and cloud metadata addresses. Pulse Cloud also blocks private addresses. Self-hosted LAN connections remain supported.
- **Connection checks protect credentials and private details.** Appliance redirects stay on the configured host, certificate discovery checks registration access before connecting, and errors omit private redirect destinations.
- **SSO protects pending sign-ins.** OIDC and SAML limit login input. OIDC checks custom CA files and keeps pending logins after incomplete callbacks. Concurrent SAML setup preserves pending sign-ins.
- **Paid Docker update instructions keep keys out of shell commands.** Settings → Updates uses Docker's hidden password prompt and shows both pinned Compose commands.

## Other improvements

- **SSO setup reports what each check verified.** Manual SAML checks label URL format results, and observed expired certificates are flagged. Test results and deletion errors stay in view, with keyboard recovery for provider checks and metadata preview.
- **SSO explains how sign-in rules combine.** Each nonempty domain, email and group list must match. Group Role Mappings assigns roles separately. Help shows how to verify OIDC or SAML in another session while keeping an independent admin login.
- **Sign-in keeps the previous SSO error visible.** Errors survive session checks and empty provider lists. Retry a stalled check, see why the local form is hidden, and read full provider names on their buttons.
- **OIDC Help explains rejected client authentication.** If code exchange reports `invalid_client` after a save, check the saved Client ID and the provider's existing secret while keeping your working admin session open.
- **Saved access files stay recoverable.** Writes preserve unread password, session, CSRF and recovery records. Unsupported file types in sign-in or organization data return errors without holding up startup or Settings.
- **Configuration restores protect concurrent saves.** Other settings writes wait until the restore finishes so a failed restore can't erase a successful save.
- **CLI exports preserve your existing backup.** Pulse replaces the destination only after writing a complete private copy. Prompts stay out of redirected archives, and incomplete output reports failure.
- **CLI action results match the request.** Missing fleet inventories return an error. Capabilities, history and confirmations must match the requested resource or action. Check action audit before retrying an unconfirmed request.
- **Keyboard controls stay reachable.** Focus stays clear of sticky bars, Change password scrolls in short windows, and monitoring toggles show focus. Help stays dismissed if closed before opening, and Billing Admin keeps IDs and row focus.
- **Private agent settings are checked before use.** Observer files are checked for type, size and permissions. Conflicting settings are refused before they can affect saved credentials.
- **Optional usage statistics show more aggregate counts.** The preview includes agents with commands or failure logs enabled and how Patrol investigations end after upgrade. Host identities and log text stay out of these counts.
- **HTTP and telemetry dependencies received fixes.** Go fixes a race in HTTP handling, and OpenTelemetry enforces its attribute length limit.

## Known issues

- **Guest freezes and Windows restarts remain unresolved.** If you stopped monitoring after a guest froze, do not restart it solely to test this beta or release.
- **Stop Pulse server monitoring before freeze-enabled backups.** Keep it stopped until independent checks confirm thaw, fresh writes on every backed-up filesystem and a running workload, even if the backup reports OK.
- **A guest can freeze without a detectable VM lock.** Cloning a running VM or importing disks can cause this, and reads already in progress when a backup lock appears can interfere with thaw. Avoid manual guest-agent probes during freeze or thaw.
- **Resume only services and timers that were active before the pause.** Avoid hooks that ignore failed stops or restart Pulse after a failed or unanswered thaw check. Monitoring and alerts are unavailable while Pulse is stopped.
- **Kubernetes pauses require storage and controller checks.** Confirm persistent Pulse server storage and prevent controllers from replacing stopped pods. Verify server polling has stopped before the backup.
- **Some Windows guests may still lack Proxmox disk readings.** Use the working Machines disk view. Avoid removing or reinstalling agents, restarting guests or forcing guest-agent checks just to restore readings.
- **Elevated Pulse server memory remains under investigation.** Keep your working setup and compare existing readings at similar uptimes and workloads. Skip restarts and heap dumps solely for this report.
- **Keep affected monitoring paused after a host lockup.** Leave agents or Patrol off if you disabled them. Preserve existing evidence privately and avoid changing GPU load or hardware to reproduce the lockup.
- **Check the running agent version after an update.** Automatic updates can leave the privilege helper on an older version. Use installer maintenance if you need a helper fix and keep the existing enrollment.
- **Older ARMv6 agents may need a manual update.** If the updater asks for ARMv7, use the installer with the correct architecture and keep the saved agent ID and connection state.
- **Earlier gaps in alert history can't be recreated.** Keep existing logs and delivery records after a storage failure, including after the history store is working again.
- **An online TrueNAS CORE pool can lack capacity readings.** If Usage / Size shows a dash, check the same pool in TrueNAS Storage. Keep its existing API key and permissions, and use the pool's figures since dataset figures can reflect quotas.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.8.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.8.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.8.0` and recreate the container.
