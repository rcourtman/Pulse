# Pulse v6.9.0 Release Notes

`v6.9.0` is a stable minor release. It promotes the `v6.9.0-rc.4` release candidate and replaces stable `v6.8.0`.

Pulse v6.9.0 opens longer chart history to Community users, brings Compose services and their checks together in Docker, and lets you rehearse alert delivery. Proxmox and PBS keep more backup results visible when a poll is incomplete.

## Highlights

- History charts now show up to 90 days of retained metrics on every plan, including Community.
- Docker → Applications now groups Compose services by runtime and project and shows linked checks and recent failures.
- Alerts → Notifications now sends practice alerts and recovery messages through saved email, webhook and Apprise routes.

## What's improved

- **Availability checks show uptime over longer windows.** Choose 24 hours, 7 days, 30 days or 1 year in Table or Fleet. Pulse keeps your choice when switching views or retrying and shows how much time it observed when history is sparse.
- **Patrol Inbox shows the conclusion and proposed fix together.** Open the review from the finding to see the affected resource, what was approved, what ran and where to find the result.
- **Backup polling uses less disk and memory.** Healthy polls skip unchanged restore points, and large PBS inventories count backups per guest with less memory. Slow Proxmox storage polls run one at a time per connection.
- **Pulse uses the memory ceiling in more Proxmox LXCs.** It uses the container’s reported RAM when the system hides its memory limit. Backup processing and alert history also use less temporary memory.
- **Libvirt memory readings use guest balloon stats.** With an updated agent, the resource API and history use these counters when available. Shut-off guests report zero usage.
- **Approved Start can use a Proxmox node agent.** Patrol checks how a VM or LXC stopped before proposing Start through a confirmed agent with commands enabled. Stop and Restart still require the action runner.
- **Configuration restores protect other saves.** Settings writes wait for the restore to finish, so a failed restore can’t undo another successful save.
- **CLI backups preserve the previous file until the new copy is complete.** Passphrase prompts stay out of redirected archives, and incomplete output reports failure.
- **CLI reads flag incomplete replies.** Fleet flags missing inventories, and action replies must match the request. Check the action audit before retrying an unconfirmed action.
- **Optional usage statistics include new aggregate counts.** The daily heartbeat can count agents with commands or failure logs enabled and report Patrol investigation outcomes.
- **Community upgrade links open again.** Patrol findings, Actions in Watch-only mode, External Probes and Application branding link to Pro details or Plans & Billing.

## Alerts and notifications

- **Delivery rehearsal keeps destination results for seven days.** Results survive restarts and show whether each send was delivered, blocked, pending, failed or unconfirmed. Check the destination for receipt.
- **Maintenance windows cover more alerts.** Matching one-time and recurring windows cover Proxmox and PBS connection warnings and PBS datastore-offline alerts. A node window applies when Pulse can match the connection to that node.
- **Repeat notices count as one firing.** Reminders, escalations and retries no longer inflate flapping counts. Older settings regain missing flapping and stale-alert defaults while keeping explicit choices.
- **Alert History keeps the final reading and message.** Resolved entries retain the latest reading and time. Metric alerts reach every registered listener even if another listener fails.
- **Finished container jobs can stay quiet.** With restart-policy data from the agent, jobs that finish successfully in under an hour and won’t restart skip offline alerts. Choose Off, Warn or Crit for individual containers.
- **One switch mutes container alerts on a Docker runtime.** Alerts → Thresholds → Docker → Container Runtimes covers existing and future containers. Runtime-offline alerts, Swarm services and endpoint checks have their own controls.
- **Proxmox Mail Gateway alerts wait for confirmed recovery.** Missing queue counts or ages leave alerts open at their existing severity. Gateway-wide and node-specific Message Age warnings can appear together.
- **Telegram handles special characters and long groups.** Built-in messages escape names, split or shorten long batches and wait when rate-limited. Recovery messages link to Alert History when a Pulse URL is configured.
- **ntfy recoveries carry authentication and event headers.** Subscribers can authenticate and identify recovery messages the same way they handle firing alerts.
- **Delivery retries cover longer outages.** Temporary failures retry for about 33 minutes. The queue honors webhook cooldowns up to 15 minutes and skips repeated attempts for permanent errors.
- **Recovered destinations can receive recent missed messages.** A successful live send can retry up to 25 eligible failures from the past 24 hours for the same saved destination.
- **Email alerts can use your branding.** Set a subject prefix, brand name and footer under Notifications → Email → Advanced options. Alerts Overview offers to add or enable a destination when delivery is on and none are enabled.

## Disks and storage

- **Machine details include Physical Disks & SMART.** Open Overview on an agent-backed machine that reports disks, then expand a drive to see its health, warnings, history and SMART attributes.
- **Unraid array problems appear on the Machines row.** See the severity and reason without expanding the host, including when an assigned disk is missing.
- **Storage alerts open the affected resource.** PBS capacity alerts go to the datastore, and Proxmox filesystem alerts go to the VM or LXC. Patrol gets the full mount path to inspect.
- **Disk investigations can follow the largest folder.** Patrol and Assistant can drill down several levels on the same filesystem, identify unreadable paths and suggest a directory read when a wildcard needs approval.
- **Proxmox Nodes shows actual disk use below 1%.** The Disk column, sorting and History use the measured percentage, so 0.88% appears as 0.88%.
- **Proxmox disk health offers longer SMART intervals.** Choose 3, 6, 12 or 24 hours or keep a saved custom interval. The drive standby warning stays beside the selector.
- **TrueNAS CORE shows each disk’s own I/O history.** Read and write samples stay paired, and Disk History identifies appliance readings, readings held in Pulse memory, or a mix.
- **TrueNAS host History can wait for slow disk reads.** CORE fetches only missing disk graphs and loads other charts independently. Reads can take up to 150 seconds and cancel when you leave or change the range.

## Proxmox, PBS and backups

- **Proxmox node drawers show hardware and linked agent readings.** See CPU, threads, RAM, fans and network ports with link speed and state. Custom readings show zeros, last-known values and collection errors.
- **Node History includes temperature on hover.** CPU, memory, disk and temperature appear together. Temperature sparklines keep their threshold colors.
- **VM memory prefers a current linked Pulse Agent reading.** It works when Proxmox status is unavailable and skips the guest memory file read. See the source and time in Overview → Details → Memory reading.
- **VM snapshots show whether memory was saved.** Missing or invalid flags stay unconfirmed. Recovery keeps historical snapshots for guests that weren’t queried during the latest poll.
- **Backup Jobs and Coverage show guest outcomes.** A fresh task can show Running, then become Unknown after five minutes, and failed reads offer Retry. Guest duration stays blank when only the parent job’s end is known.
- **Backup Health shows what Pulse could evaluate.** See pending and unevaluated workloads, failed or running jobs and unmatched backups. Unknown shields open the guest’s Coverage, and Restore points flags partial totals and empty results.
- **Proxmox keeps observed task results through incomplete polls.** Overlapping pages still contribute new tasks. Shorter logs, later timeouts and unread nodes preserve the last usable results for the affected connection.
- **Protection dates use completed archives.** An archive still being written leaves the date alone. An unread shared store keeps last-known points and PBS links even when Pulse reads a same-named local store.
- **PBS retains last-known failures and restore points.** Unread job lists preserve sync, verify and prune failures through restarts. Capped or incomplete history keeps earlier points until a complete inventory can confirm removals.
- **Backup Servers and Replication show uncertainty.** PBS warnings, unknown states and unreachable servers are distinct, and unread capacity stays unknown. Replication has an Unknown filter and keeps missing failure counts blank.
- **Proxmox connection tests stay with the tested settings.** They use the intended credential, including saved PMG tokens, and distinguish network failures from TLS or access problems. Cluster results label the members the endpoint listed.
- **Recovery Help explains ignored VMIDs and partial PBS History.** Guests you skip still appear in Coverage. Ignore VMIDs can mute backup-age alerts for guests with the same ID on another connection.

## TrueNAS, vSphere and Docker

- **TrueNAS honors its collection switches.** Turn off pools, datasets or replication independently. Turning off datasets skips snapshots, and turning off all three still allows system, disk and native alert readings.
- **TrueNAS snapshots take less work to poll.** SCALE 25.04 and 25.10 fallback queries request only the fields Pulse uses, and large snapshot inventories use less temporary memory.
- **TrueNAS identifies the native API and installed agents.** The drawer shows where readings come from, and Agent Profiles lists TrueNAS hosts only when they have an installed agent.
- **TrueNAS Memory separates reclaimable ZFS cache from free RAM.** Open the bar in Overview for the breakdown. Linked agent readings also account for ZFS cache when used for Proxmox VM memory.
- **TrueNAS and vSphere tests report what they read.** Feedback stays in the editor and names confirmed details and gaps. A refused destination leaves the draft open for correction.
- **vSphere honors VM, host and datastore collection choices.** Skipped inventory calls can’t fail the poll. Related resources keep readable names, and empty views explain how to enable collection.
- **Approved Compose updates change one service.** With a command-enabled agent and a registered source, Pulse updates the local Compose file and replaces that service, or pushes a Git branch for your usual review and deployment.
- **Service deployments shows what actually rolled out.** Pulse compares the running image with the reviewed image and shows a fresh linked HTTP check separately.
- **Compose admins can opt in to live logs.** Enable live logs on the agent to follow the latest 30 lines from a selected container in Docker → Applications.
- **Docker and Podman can show recent failure logs.** Opt in under Install or with `--enable-failure-logs` for up to 30 lines and 4 KB. Leave it off for sensitive logs because masking can miss secrets and AI can send logs to your provider.
- **Docker health checks explain failures.** Container details, State tooltips, alerts and Patrol show causes such as DNS, HTTP, TLS, a timeout or a missing command. Raw probe output stays on the host.
- **Container updates protect the original.** Failed pulls or unchanged images leave it in place, and Pulse checks whether it was restored after an interrupted stop or rename. A replacement for a stopped container stays stopped.

## Install, updates and agents

- **Install options are available before generating credentials.** The general installer and Proxmox VE and PBS Host Telemetry Agent forms let you set the connection URL, TLS options and whether commands are allowed.
- **Agents protect saved identity and credentials.** Unreadable state stops startup with guidance to restore unavailable storage first. Read-only installations can set a persistent ID with `--agent-id` or `PULSE_AGENT_ID`.
- **Enrollment confirms the saved identity before reporting success.** An incomplete reply ends the attempt because the setup token may already have been used.
- **Installers prepare agent replacements before stopping services.** Windows stages the executable. Unix checks the collector on its destination filesystem and prepares the optional helper, including on TrueNAS SCALE.
- **Unix setup explains URL and launch failures separately.** A successful download check can still be followed by a refused agent URL or an executable that can’t run. Errors identify the filesystem checked.
- **NAS and firewall agents keep working boot settings.** TrueNAS CORE enables startup at boot and reports unconfirmed service state. On pfSense, a failed install preserves the previous startup settings and credentials when recovery succeeds.
- **APT reports pending packages without an action runner.** The Linux collector checks locally and marks incomplete results. Installing packages still requires the runner.
- **Updates shows when a release is still on its way.** Settings distinguishes pending delivery from an unconfirmed status, checks pending releases sooner and refreshes the result when you change channels.
- **Server updates protect the installed edition.** Automatic updates recheck the running version and require a confirmed newer Community release. Public manual updates also require a confirmed Community installation.
- **Updates recover better from low disk space and broken downloads.** Pulse clears stale staging files, chooses a staging location with more room, and retries incomplete transfers.
- **Docker warns when recreating Pulse could erase its data.** Startup logs and Settings → Updates flag a data directory without a persistent mount. Export a configuration backup before moving Pulse data to a persistent volume.
- **Installer commands report what finished.** Help, cancellation and removal avoid false success, and downloads and journal waits have time limits. A registration timeout tells you to check the Proxmox node list before retrying.

## Pulse Pro, AI and hosted

- **Plans & Billing moved to Settings → System.** The comparison lists scheduled reports, exports and agent profiles. Eligible Community installs can switch to Pro in place or copy Docker Compose commands for a pinned image.
- **Pro Docker updates keep registry passwords out of copied commands.** Settings shows a hidden password prompt and both Compose commands. The Docker guide shows how to confirm the running image after recreation.
- **Patrol follows up when an alert scan records no finding.** Ask first and Autopilot can open a finding on a follow-up. Manual checks start with the active alert, and investigations read fresh monitoring data.
- **Findings stay with the right problem and resource.** Separate problems on one guest get separate findings. Patrol corrects a guessed ID when one resource matches, shows IDs in triage and closes findings after confirming removal.
- **Written advice can become a reviewable fix.** Patrol asks once for a supported proposal or conclusion when it receives advice alone. An investigation can still propose a fix on its final turn.
- **Patrol stays connected after monitoring reloads.** Changes to connections or polling settings preserve alert-driven investigations. Weekly summaries count fixes confirmed between scans.
- **Stopped workloads can get findings without an alert.** Patrol uses exit codes, out-of-memory status and available actions to explain what happened and whether someone needs to intervene.
- **AI reads start with the resource under investigation.** Patrol can read inside a running Proxmox guest through a confirmed node agent. Refused actions explain whether commands, an updated agent or an action runner are needed.
- **AI backup queries summarize space before listing backups.** Ask by guest name or VMID, Proxmox storage or PBS datastore. Results include counts, sizes and dates and flag incomplete totals.
- **MSP offers stay usable when prices are missing.** Account marks the affected plans or billing periods and offers Reload. Larger self-hosted fleets see a dismissible MSP option in Infrastructure.
- **Account can confirm an uncertain client change.** Refresh to read the saved result without sending it again. Sign-in codes for the same email share a visible one-minute wait, with feedback when delivery is uncertain.
- **Hosted clients open in their own workspace.** New and resumed workspaces recheck billing and offer Retry. Delayed checkouts and edits respect withdrawn access, and recovery waits for old processes to stop writing.

## Monitoring and service health

- **Docker suggests checks for published web ports.** Review responding HTTP or HTTPS addresses before adding them. Pulse checks the endpoint again on save and clears suggestions when workspace access changes.
- **Docker shows when a running container stops responding.** State tooltips name the failing checks. A check follows a recreated container on the same runtime when its name and checked port still match.
- **Related Compose failures can appear in one incident.** Alerts groups container failures with linked service-check failures while each check keeps its own notification settings.
- **Home Assistant Supervisor avoids false update alerts.** Pulse clears alerts caused solely by the `latest` image alias and leaves Supervisor updates to Home Assistant.
- **TCP and UDP checks validate replies more closely.** TCP fails if the port immediately closes or resets, and exact UDP matching checks the whole reply. Help shows how to verify DNS answers through an HTTP endpoint.
- **Availability checks identify what they monitor.** Label routers, switches, access points, machines or services. Alerts opens the affected check, and Fleet keeps keyboard focus usable when a check disappears.
- **History APIs and MCP open retained history to every plan.** Requests can cover up to 365 days, subject to retention. Pulse rejects invalid metrics ranges and preserves stored history and reports when a paid plan ends.
- **Resource APIs include more network details.** Updated agents can report Linux IPv4 gateways, DNS servers and Docker network mode. Proxmox guests can include bridge, VLAN and firewall settings.
- **Pulse health can flag a stalled dashboard.** On standalone installs, `/api/health` can show `websocketStateStalled` after 30 seconds while liveness stays healthy. Help explains how to use a saved health response.
- **Local helper waits end when the caller cancels.** Setup and execution share a time limit, temporary failures back off, and shutdown lets open connections finish. Work the helper already received can still finish.
- **Large fleets do less repeated work.** Startup does less database work. Workload charts, snapshot discovery and disk and sensor alert cleanup process less unrelated data.
- **Help covers everyday alert and availability setup.** Find steps for routing notifications, choosing check types and using an independent watchdog when Pulse itself goes offline.

## Security

- **Host commands require execute access.** Assistant file and log reads, Kubernetes logs, Docker update scans and agent checks during previews require action execution permission and the appropriate API token scope.
- **Action reviews enforce your role.** Inbox fixes require plan, approve and execute permissions. Assistant planning requires plan permission, and starting another investigation of an existing finding requires plan and execute.
- **Automatic command approval checks arguments.** Options that change systems, expose process environments or read extra files require approval. Automatic log reads stay within `/var/log`, and Pulse filters environment overrides.
- **Recovery tokens must be active and saved.** Pulse records creation and one-time use before granting access. An expired or ended recovery leaves the token unused.
- **API token changes check current access.** Writes recheck the caller’s permissions and workspace. Incomplete files preserve live tokens, restored older files can reload, and failed saves keep recovery copies.
- **Old replies stay with their original sign-in.** Requests made before a credential change can’t sign out the current user or switch their workspace. Sharing, ownership and role changes recheck access before applying.
- **Client addresses follow the configured proxy chain.** List each trusted hop in `PULSE_TRUSTED_PROXY_CIDRS` or have the final proxy replace the forwarded address. Same-host proxies using a loopback Host need `X-Forwarded-For`.
- **Local setup and recovery check request origin.** Only direct local requests can use local access. Security Overview shows the observed address and describes configured authentication across the instance.
- **Workspace connections check the destination.** Organization administrators cannot point appliance connections, checks or SMTP at loopback, link-local or cloud metadata addresses. Pulse Cloud also blocks private addresses.
- **Connection checks protect credentials.** TrueNAS and vSphere redirects stay on the configured host. Agent registration checks access before certificate discovery, and errors omit private redirect addresses and enrollment response bodies.
- **SSO protects pending logins.** OIDC and SAML limit login input. OIDC checks custom CA files, and incomplete OIDC callbacks or concurrent SAML setup preserve pending logins.
- **Saved access files stay recoverable.** Writes preserve unread credentials and session, CSRF and recovery records, and unsupported file types return errors without blocking startup. Private agent files check size, permissions and conflicting settings.

## Other improvements

- **Workspace switching keeps your choice visible.** Retry failed lists in place, see the workspace name in the header and use IDs to distinguish duplicate names. Alerts uses the selected workspace’s policy and keeps confirmations with it.
- **Organization Settings keeps confirmed details visible.** Restart stalled Overview, Access, Billing & Usage and Sharing checks. Ownership controls explain available changes, and long workspace names stay readable.
- **Organization Access shows what is pending.** Member forms explain immediate changes, and empty invitation lists are labeled. After an uncertain invitation reply, Check again reads saved access without sending the decision again.
- **Sharing identifies both parties and the resource.** Saved shares keep confirmed identities, access levels and outcomes through failed refreshes. Create Share supports Enter and points to the field that needs correction.
- **Roles and User Access preserve drafts and focus.** Retry stalled reads, see Allow or Deny and all conditions, and skip unchanged saves. New Role catches duplicate IDs, and User Access explains why you can’t remove your own account.
- **Audit Log and webhooks keep recovery controls in reach.** Filters stay usable while results load and verification applies to the displayed page. Restart a stalled read or confirm an uncertain webhook change without resending it.
- **API Access shows observed token use before revocation.** See matching agents and Docker or Podman runtimes. All-off scopes are labeled, errors stay inside their dialogs, and token lists can retry a stalled load.
- **Infrastructure keeps test results with their settings.** Changing the address clears API detection results. Saved scope shows when all API collection options are off, and PBS datastore exclusions stay visible near Save.
- **SSO tests explain what was checked.** OIDC shows issuer mismatches, missing discovery URLs and saved secret status. SAML names failed checks, labels URL-only tests and keeps an unset email attribute.
- **Sign-in keeps the last SSO error visible.** Retry stalled checks, follow failed OIDC sign-in to recovery and see why local sign-in is hidden. Field labels stay visible and long provider names remain readable.
- **Setup Help covers scripted installs and MCP options.** Follow the first-run steps to create an admin and enroll a host with a token of at least 32 characters. The MCP installer prepares custom destinations and shows help on request.
- **SSO keeps settings read errors by the provider.** Use Edit again to retry without submitting changes. Help covers checking callbacks, keeping a recovery login and verifying sign-in from a separate session.

## Known issues

- **Guest freezes and Windows restarts remain unresolved.** If you stopped monitoring after a guest froze, do not restart it solely to test this release or a beta.
- **Stop Pulse server monitoring before freeze-enabled backups.** Resume only after independent checks confirm thaw, fresh writes on every backed-up filesystem and a running workload, even when the backup reports OK.
- **Guests can freeze without a detectable VM lock.** Cloning a running VM or importing its disks can do this, and reads already in progress when a lock appears can interfere with thaw. Avoid manual guest-agent probes during freeze or thaw.
- **Resume only services and timers that were active before the pause.** Avoid hooks that ignore failed stops or restart Pulse after a failed or unanswered thaw check. Monitoring and alerts are unavailable while Pulse is stopped.
- **Kubernetes backup pauses require storage and controller checks.** Confirm persistent Pulse server storage, prevent controllers from replacing stopped pods and verify that Pulse has stopped polling before the backup.
- **Some Windows guests may still lack Proxmox disk readings.** Use the working Machines disk view or check in Windows. Keep existing agents and avoid guest restarts or forced guest-agent probes solely to recover the reading.
- **High server memory and out-of-memory stops remain under investigation.** Keep your working setup and compare existing readings at similar uptimes and workloads. After Pulse resumes, check for fresh History readings during normal polling.
- **Use existing evidence after a memory stop.** Keep before and after readings separate if Pulse restarts or the container’s memory counters reset. Avoid restarts, heap dumps and test loads solely to investigate.
- **Keep affected monitoring paused after a host lockup.** Leave agents or Patrol off if you disabled them. Preserve existing evidence privately and avoid changing GPU load or hardware to recreate the lockup.
- **Check the running agent and helper after automatic updates.** Use installer maintenance if the helper stayed old. If an older ARMv6 agent asks for ARMv7, update manually for ARMv6 and keep its ID and enrollment.
- **Earlier gaps in alert history can’t be recreated.** Keep existing logs and delivery records after a storage failure, including after the history store is working again.
- **An online TrueNAS CORE pool can still lack capacity readings.** If Usage / Size shows a dash, check that pool in TrueNAS Storage. Keep the existing API key and permissions, and use the pool’s figures since datasets can have quotas.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This release does not require a companion mobile release.
- The rollback target is stable `v6.8.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.8.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.8.0` and recreate the container.
