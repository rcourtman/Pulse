# Pulse v6.9.0-rc.3 Release Notes

This release candidate follows `v6.9.0-rc.2` on the way to stable `v6.9.0`. Stable `v6.8.0` remains the rollback target.

Pulse v6.9.0 makes longer metric history available to everyone, adds a home for Docker Compose apps and standalone VMs, and gives you a way to test alert routes. Proxmox and PBS show more of what they know when a backup poll is incomplete, and setup and update errors give clearer next steps.

## Highlights

- History charts now show up to 90 days of retained metrics on every plan, including Community.
- Docker → Applications now groups Compose services by runtime and project and shows linked checks and recent failures.
- Alerts → Notifications now sends rehearsal alerts and recoveries through saved email, webhook and Apprise routes.

## What's improved

- **Machines lists standalone VMs.** The VMs tab appears when agents report Unraid, libvirt/KVM or XCP-ng guests. Host VM counts open a filtered list, and libvirt guests use balloon memory readings when available.
- **Patrol Inbox shows the conclusion and proposed fix together.** Open the review from the finding to see the affected resource, what approval did, what ran and where to find the result.
- **Availability checks show uptime over longer windows.** Choose 24 hours, 7 days, 30 days or 1 year in Table or Fleet. The choice stays in place when switching views or retrying, and sparse history shows how much time was observed.
- **Backups writes less to disk.** Healthy polls skip rewriting unchanged restore points. Large PBS inventories use less memory to count backups per guest, and slow Proxmox storage polls no longer pile up.
- **Pulse respects the memory ceiling in more Proxmox LXCs.** It uses the container's reported memory when the cgroup limit is hidden. Backup processing and alert history also use less temporary memory.
- **Configuration restores protect other saves.** Settings writes wait for the restore to finish, keeping a failed restore from undoing another successful save.
- **CLI backups preserve the previous file until the new copy is complete.** Passphrase prompts stay out of redirected archives, and incomplete output reports failure.

## Alerts and notifications

- **Rehearsal reports show what happened at each destination.** Results distinguish delivery, failure, suppression, waiting and uncertainty. Reports survive restarts for seven days. Confirm receipt in the destination itself.
- **Maintenance windows cover more alerts.** Matching one-time and recurring windows cover Proxmox and PBS connection warnings and PBS datastore-offline alerts. A node window applies only when Pulse can match the connection to that node.
- **Repeat notices no longer look like flapping.** Reminders, escalations and retries count as one firing. Older settings with missing defaults regain flapping protection and stale-alert cleanup, while explicit choices are preserved.
- **Alert History keeps the final value and message.** Resolved entries retain the latest reading and time. Metric alerts reach every registered listener, even if a listener fails or changes its copy.
- **Finished container jobs can stay quiet.** With restart-policy data from the agent, jobs that finish successfully in under an hour and won't restart skip offline alerts. Choose Off, Warn or Crit for individual containers.
- **One switch mutes container alerts on a Docker runtime.** Alerts → Thresholds → Docker → Container Runtimes covers existing and future containers. Runtime-offline alerts, Swarm services and endpoint checks have their own controls.
- **Mail Gateway alerts wait for confirmed recovery.** Missing queue counts or ages leave alerts open and preserve critical severity. Gateway-wide and node-specific Message Age warnings can appear together.
- **Telegram handles special characters and long groups.** Built-in messages escape names, split or shorten long batches and wait when rate-limited. Recovery messages link to Alert History when a Pulse URL is configured.
- **ntfy recoveries carry authentication and event headers.** Subscribers can authenticate and identify recovery messages the same way they handle firing alerts.
- **Delivery retries cover longer outages.** The queue retries temporary failures for about 33 minutes, respects webhook cooldown requests up to 15 minutes, and reports permanent setup errors without repeated attempts.
- **Recovered destinations can receive recent missed messages.** A successful live send can retry up to 25 eligible failures from the past 24 hours for the same saved destination.
- **Notifications adds email branding and destination guidance.** Set a subject prefix, brand name and footer under Email → Advanced options. Alerts Overview prompts for setup when delivery is active and Pulse confirms zero enabled destinations.

## Disks and storage

- **Machine details include Physical Disks & SMART.** Open Overview on an agent-backed machine that reports disks. Expand a drive to see its health, warnings, history and SMART attributes.
- **Unraid array problems appear on the Machines row.** See the severity and reason without expanding the host, including when an assigned disk is missing.
- **Storage alerts open the affected resource.** PBS capacity alerts go to the datastore, and Proxmox filesystem alerts go to the VM or LXC. Patrol gets the full mount path to inspect.
- **Disk investigations can follow the largest folder.** Patrol and Assistant can drill down several levels on the same filesystem, identify unreadable paths and suggest a directory read when a wildcard needs approval.
- **Proxmox Nodes shows actual disk use below 1%.** The Disk column, sorting and History use the measured percentage, so 0.88% appears as 0.88% instead of 88%.
- **Proxmox disk health offers longer SMART intervals.** Choose 3, 6, 12 or 24 hours or keep a saved custom interval. The drive standby warning stays beside the selector.
- **TrueNAS CORE shows each disk's own I/O history.** Read and write samples stay paired, and Disk History identifies appliance readings, readings held in Pulse memory, or a mix.
- **TrueNAS host History handles slow reads.** CORE fetches only the missing disk graphs and lets other charts load independently. Reads can take up to 150 seconds and cancel when you leave or change the range.

## Proxmox, PBS and backups

- **Node details show hardware and linked agent readings.** See CPU, threads, RAM, fans and network ports with link speed, link state and last-known labels. Custom readings show zeros and collection errors.
- **Node History includes temperature on hover.** CPU, memory, disk and temperature appear together. Temperature sparklines keep their threshold colors.
- **VM memory prefers a current linked Pulse Agent reading.** It works when Proxmox status is unavailable and skips the guest memory file read. See the source and time in Overview → Details → Memory reading.
- **VM snapshots show whether memory was saved.** Missing or invalid flags stay unconfirmed. Recovery keeps historical snapshots for guests that weren't queried during the latest poll.
- **Backup Jobs and Coverage show each guest's outcome.** Running becomes Unknown after five minutes without fresh evidence. A parent job's end time leaves guest duration blank, and failed reads offer Retry.
- **Backup Health shows what Pulse could evaluate.** See pending and unevaluated workloads, failed or running jobs and unmatched backups. Unknown shields open the guest's Coverage, and Restore points flags partial totals and empty results.
- **Proxmox keeps observed task results through incomplete polls.** Overlapping pages still contribute new tasks. Shorter logs, later timeouts and unread nodes preserve the last usable results for the affected connection.
- **Protection dates come from completed archives.** A backup still being written can't move the protection date forward. Unread shared storage keeps last-known points and PBS links even when a same-named local store was read.
- **PBS retains last-known failures and restore points.** Unread job lists preserve sync, verify and prune failures through restarts. Capped or incomplete history keeps earlier points until a complete inventory can confirm removals.
- **Backup Servers and Replication show uncertainty.** PBS warnings, unknown states and unreachable servers are distinct, and unread capacity stays unknown. Replication has an Unknown filter and keeps missing failure counts blank.
- **Proxmox connection tests explain what was checked.** Feedback stays with the tested draft and credential, including saved PMG tokens. It distinguishes credentials, read permissions, TLS and network errors and labels cluster members the endpoint listed.
- **Approved Start works through a confirmed Proxmox node agent.** Patrol checks how the VM or LXC stopped before proposing Start. The agent needs commands enabled, and Stop and Restart still require the action runner.

## TrueNAS, vSphere and Docker

- **TrueNAS honors its collection switches.** Turn off pools, datasets or replication independently. Turning off datasets skips snapshots, and turning off all three still allows system, disk and native alert readings.
- **TrueNAS snapshots take less work to poll.** SCALE 25.04 and 25.10 fallback queries request only the fields Pulse uses, and large snapshot inventories use less temporary memory.
- **TrueNAS shows where its readings come from.** The drawer distinguishes the native API from a real Pulse Agent. Agent profiles list only hosts with an installed agent.
- **TrueNAS Memory separates reclaimable ZFS cache from free RAM.** Open the bar in Overview for the breakdown. Linked agent readings also account for ARC when used for Proxmox VM memory.
- **TrueNAS and vSphere tests stay with the draft.** Results say which details were read or left unconfirmed. Refused destinations keep the editor open for correction.
- **vSphere honors VM, host and datastore collection choices.** Skipped inventory calls can't fail the poll. Related resources keep readable names, and empty views explain how to enable collection.
- **Reviewed Compose updates use a registered source.** An approved local update edits the Compose file and replaces one service. Git mode publishes a branch for review and deployment through your usual process.
- **Service deployments shows what actually rolled out.** Pulse compares the running image with the reviewed image and shows a fresh linked HTTP check separately.
- **Compose admins can opt in to live logs.** Enable live logs on the agent to follow the latest 30 lines from a selected container in Docker → Applications.
- **Docker and Podman can show recent failure logs.** Opt in under Install on a host or with `--enable-failure-logs` for up to 30 lines and 4 KB. Leave it off for sensitive logs. Masking can miss secrets and AI can send logs to your provider.
- **Docker health checks explain failures.** Container details, State tooltips, alerts and Patrol show causes such as DNS, HTTP, TLS, a timeout or a missing command. Raw probe output stays on the host.
- **Container updates protect the original.** Failed pulls or unchanged images leave it in place. After an interrupted stop or rename, Pulse checks whether it was restored. A replacement for a stopped container stays stopped.

## Install, updates and agents

- **Install options are available before generating credentials.** The general installer and Proxmox VE and PBS Host Telemetry Agent forms let you set the connection URL, TLS options and whether commands are allowed.
- **Agents protect saved identity and credentials.** Unreadable state stops startup with recovery guidance. Restore unavailable storage first. Read-only installations can set a persistent ID with `--agent-id` or `PULSE_AGENT_ID`.
- **Enrollment confirms the saved identity before reporting success.** An incomplete reply ends the attempt because the setup token may already have been used.
- **Installers prepare agent replacements before stopping services.** Windows stages the executable. Unix checks the collector on its destination filesystem and prepares the optional helper, including on TrueNAS SCALE.
- **Unix setup explains URL and launch failures separately.** A successful download check can still be followed by a refused agent URL or an executable that can't run. Errors identify the filesystem checked.
- **NAS and firewall agents keep working boot settings.** TrueNAS CORE enables startup at boot and reports unconfirmed service state. On pfSense, a failed install preserves the previous startup settings and credentials when recovery succeeds.
- **APT reports pending packages without an action runner.** The Linux collector checks locally and marks incomplete results. Installing packages still requires the runner.
- **Updates explains when a release is still on its way.** Settings distinguishes pending delivery from an unconfirmed status, checks pending releases sooner and refreshes the result when you change channels.
- **Server updates protect the installed edition.** Automatic updates recheck the running version and require a confirmed newer Community release. Public manual updates also require a confirmed Community installation.
- **Updates recover better from low disk space and broken downloads.** Pulse clears stale staging files, chooses a staging location with more room, and retries incomplete transfers.
- **Docker warns when Pulse data can disappear on recreation.** Look in startup logs and Settings → Updates for a warning when the active data directory has no persistent mount.
- **Installer commands report what finished.** Help, cancellation and removal avoid false success. Downloads and journal waits have limits, and a registration timeout tells you to check the Proxmox node list before retrying.

## Pulse Pro, AI and hosted

- **Plans & Billing is under Settings → System.** The Pro comparison now calls out scheduled reports, exports and agent profiles. Eligible Community installs can switch to Pro in place or copy pinned Docker Compose commands.
- **Pro Docker updates check the running image.** Settings shows the private registry's hidden password prompt and both Compose commands. Check the running image after recreation to confirm the update.
- **Patrol follows up on firing alerts.** Ask first and Autopilot can open a finding if a completed scan records none. Manual checks use fresh readings and examine the active alert first.
- **Findings stay with the right problem and resource.** Separate problems on one guest get separate findings. Patrol corrects a guessed ID when one resource matches, shows IDs in triage and closes findings after confirming removal.
- **Written advice can become a reviewable fix.** Patrol asks once for a supported proposal or conclusion when it receives advice alone. An investigation can still propose a fix on its final turn.
- **Patrol stays connected after monitoring reloads.** Changes to connections or polling settings preserve alert-driven investigations. Weekly summaries count fixes confirmed between scans.
- **Stopped workloads can get findings without an alert.** Patrol uses exit codes, out-of-memory status and available actions to explain what happened and whether someone needs to intervene.
- **AI reads start with the resource under investigation.** Patrol can read inside a running Proxmox guest through a confirmed node agent. Refused actions explain whether commands, an updated agent or an action runner are needed.
- **AI backup queries summarize space before listing files.** Ask by guest name or VMID, Proxmox storage or PBS datastore. Results include counts, sizes and dates and flag incomplete totals.
- **MSP offers stay usable when prices are missing.** Account marks the affected plans or billing periods and offers Reload. Larger self-hosted fleets see a dismissible MSP option in Infrastructure.
- **Account can confirm an uncertain client change.** Refresh to read the saved result without sending it again. Sign-in codes for the same email share a visible one-minute wait, with feedback when delivery is uncertain.
- **Hosted clients land in their own workspace.** New and resumed workspaces recheck billing and offer retry. Delayed checkouts or edits can't restore withdrawn access, and recovery waits for old processes to stop writing.

## Monitoring and service health

- **Docker suggests checks for published web ports.** Review responding HTTP or HTTPS addresses before adding them. Pulse verifies the selected endpoint again on save and clears suggestions when workspace access changes.
- **Docker shows when a running container stops responding.** State tooltips name the failing checks. A check follows a recreated container on the same runtime when its name and checked port still match.
- **Related Compose failures can appear in one incident.** Alerts groups container failures with linked service-check failures while each check keeps its own notification settings.
- **Home Assistant Supervisor avoids false update alerts.** Pulse clears alerts caused solely by the `latest` image alias and leaves Supervisor updates to Home Assistant.
- **TCP and UDP checks verify more than a response prefix or open port.** TCP fails if the port immediately closes or resets. Exact UDP matching checks the whole reply, and Help shows how to verify DNS answers through an HTTP endpoint.
- **Availability checks identify the equipment they monitor.** Label routers, switches, access points, machines or services. Alerts opens the affected check, and Fleet keeps keyboard focus usable when a check disappears.
- **History APIs and MCP can read retained data on every plan.** Metrics and availability requests support up to 365 days, subject to retention. Ending a paid plan preserves saved history and generated reports.
- **Resource APIs report more network details.** Updated agents can supply Linux IPv4 gateways, DNS servers and Docker network mode. Proxmox guests can include bridge, VLAN and firewall settings.
- **Standalone Pulse health can flag stalled dashboard updates.** An existing `/api/health` response can show `websocketStateStalled` after 30 seconds while liveness stays healthy.
- **Local helper waits end when the caller cancels.** Setup and execution share a time limit, temporary failures back off, and shutdown lets connections drain. Work the helper already received can still finish.
- **Large fleets do less repeated work.** Startup does less database work. Workload charts, snapshot discovery and disk and sensor alert cleanup process less unrelated data.

## Security

- **Host commands require execute access.** Assistant file and log reads, Kubernetes logs, Docker update scans and agent checks during previews require action execution permission and the appropriate API token scope.
- **Action reviews enforce your role.** Inbox fixes require plan, approve and execute permissions. Assistant planning requires plan permission. Starting another investigation of an existing finding requires plan and execute permissions.
- **Automatic command approval checks arguments.** Options that change systems, expose process environments or read extra files require approval. Automatic log reads stay within `/var/log`, and Pulse filters environment overrides.
- **Recovery tokens must be active and saved.** Pulse records creation and one-time use before granting access. An expired or ended recovery leaves the token unused.
- **API token changes check current access.** Writes recheck the caller's permissions and workspace. Incomplete files preserve live tokens, restored older files can reload, and failed saves keep recovery copies.
- **Old replies stay with their original sign-in.** Requests made before a credential change can't sign out the current user or switch their workspace. Sharing, ownership and role changes recheck access before applying.
- **Client addresses follow the configured proxy chain.** List each trusted hop in `PULSE_TRUSTED_PROXY_CIDRS` or have the final proxy replace the forwarded address. Same-host proxies using a loopback Host need `X-Forwarded-For`.
- **Local setup and recovery check request origin.** Only direct local requests can use local access. Security Overview shows the observed address and describes configured authentication across the instance.
- **Workspace connections enforce destination limits.** Appliance connections, checks and SMTP block local-only and cloud metadata addresses. Pulse Cloud also blocks private addresses. Self-hosted LAN connections remain supported.
- **Connection checks protect credentials.** TrueNAS and vSphere redirects stay on the configured host. Agent registration checks access before certificate discovery, and errors omit private redirect addresses and enrollment response bodies.
- **SSO protects pending logins.** OIDC and SAML limit login input. OIDC checks custom CA files and preserves pending logins after incomplete callbacks. Concurrent SAML setup preserves pending logins.
- **Saved access files stay recoverable.** Writes preserve unread password, session, CSRF and recovery records. Unsupported file types return errors without blocking startup. Private agent settings also check file type, size, permissions and conflicts.

## Other improvements

- **Workspace switching keeps the selected workspace visible.** Retry failed lists in place and use IDs to distinguish repeated names. Alerts loads the selected workspace's policy and keeps confirmations with the workspace that sent them.
- **Organization Settings keeps confirmed details visible.** Restart stalled Overview, Access, Billing & Usage and Sharing checks. Ownership controls explain available changes, and long workspace names stay readable.
- **Organization Access and invitations show what's pending.** A confirmed empty list is distinct from loading or failure. Member forms explain immediate changes. After an uncertain invitation reply, Check again reads saved access without resending it.
- **Sharing identifies both parties and the resource.** Saved shares keep confirmed identities, access levels and outcomes through failed refreshes. Create Share supports Enter and points to the field that needs correction.
- **Roles and User Access preserve drafts and keyboard focus.** Retry stalled reads, see Allow or Deny and all conditions, and skip unchanged saves. New Role catches duplicate IDs, and User Access explains why you can't remove your own account.
- **Audit Log and webhooks keep recovery controls in reach.** Filters stay usable while results load and verification applies to the displayed page. Restart a stalled read or confirm an uncertain webhook change without resending it.
- **API Access shows observed token use before revocation.** See matching agents and Docker or Podman runtimes. All-off scopes are labeled, errors stay inside their dialogs, and token lists can retry a stalled load.
- **Infrastructure keeps tests and scope with their settings.** API detection clears results when the address changes. Saved collection scope can show all options off alongside agent coverage, and PBS exclusions stay visible near Save.
- **SSO editors explain exactly what a test checked.** OIDC flags issuer mismatch and shows saved secret status. SAML identifies failed checks, labels manual URL checks and preserves an unset email attribute. Help covers callbacks and safe sign-in checks.
- **Sign-in keeps the last SSO error visible.** Retry stalled checks, follow failed OIDC sign-in to recovery, and see why local sign-in is hidden. Field labels stay visible and provider names remain readable.
- **Setup Help covers scripted installs and MCP options.** The first-run guide covers creating an admin and enrolling a host. Supplied setup tokens need 32 characters. The MCP installer shows help without installing and prepares custom destinations.
- **CLI and usage reports are clearer.** CLI fleet reads flag missing inventories and action replies must match the request. Check action audit before retrying an unconfirmed action. Optional usage stats show aggregate agent settings and Patrol outcomes.

## Known issues

- **Guest freezes and Windows restarts remain unresolved.** If you stopped monitoring after a guest froze, do not restart it solely to test this release or a beta.
- **Stop Pulse server monitoring before freeze-enabled backups.** Keep it stopped until independent checks confirm thaw, fresh writes on every backed-up filesystem and a running workload. Verify those conditions even if the backup reports OK.
- **Guests can freeze without a detectable VM lock.** Cloning a running VM or importing disks can do this. Reads already in progress when a lock appears can interfere with thaw. Avoid manual guest-agent probes during freeze or thaw.
- **Resume only services and timers that were active before the pause.** Avoid hooks that ignore failed stops or restart Pulse after a failed or unanswered thaw check. Monitoring and alerts are unavailable while Pulse is stopped.
- **Kubernetes pauses require storage and controller checks.** Confirm persistent Pulse server storage, prevent controllers from replacing stopped pods and verify server polling has stopped before the backup.
- **Some Windows guests may still lack Proxmox disk readings.** Use the working Machines disk view or check in Windows. Keep existing agents and avoid guest restarts or forced guest-agent probes solely to recover the reading.
- **Elevated server memory and out-of-memory stops remain under investigation.** Keep your working setup and compare existing readings at similar uptimes and workloads. Skip restarts and heap dumps solely for this report.
- **Keep affected monitoring paused after a host lockup.** Leave agents or Patrol off if you disabled them. Preserve existing evidence privately and avoid changing GPU load or hardware to recreate the lockup.
- **Check the running agent version after an update.** Automatic updates can leave the privilege helper on an older version. Use installer maintenance if you need a helper fix and keep the existing enrollment.
- **Older ARMv6 agents may need a manual update.** If the updater requests ARMv7, use the installer with the correct architecture and preserve the saved agent ID and connection state.
- **Earlier gaps in alert history can't be recreated.** Keep existing logs and delivery records after a storage failure, including after the history store is working again.
- **An online TrueNAS CORE pool can still lack capacity readings.** If Usage / Size shows a dash, check that pool in TrueNAS Storage. Keep the existing API key and permissions, and use the pool's figures since datasets can have quotas.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.8.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.8.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.8.0` and recreate the container.
