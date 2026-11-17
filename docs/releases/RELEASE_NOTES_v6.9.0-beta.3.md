# Pulse v6.9.0-beta.3 Release Notes

This beta follows `v6.9.0-beta.2` on the way to stable `v6.9.0`. Stable `v6.8.0` remains the rollback target.

Pulse v6.9.0 adds Docker service checks and optional failure logs, makes Patrol investigations easier to follow, and improves Proxmox guest memory readings. Notification delivery, agent setup, updates and access settings handle failures more clearly.

## Highlights

- Docker now offers availability checks for published web ports, testing HTTP responses and showing host addresses before adding checks.
- Patrol Inbox now shows investigation conclusions and opens proposed fixes for review beside the affected item.
- Proxmox VM memory now prefers current linked-agent readings, counting TrueNAS ZFS ARC as reclaimable cache.

## What's improved

- **Configuration restores protect other saves.** Settings changes wait until a restore finishes, so a failed restore cannot discard an unrelated successful save.
- **CLI backups preserve the previous file.** Exports replace it only after a complete private write. Passphrase prompts stay out of redirected archives, and incomplete output reports failure.
- **Sharing distinguishes repeated names.** Organization and resource selectors show identifiers beside names. Saved shares retain readable identities, access levels and decisions in narrow panels.
- **Organization Access preserves keyboard position.** Saves, reloads and failed reads return lost focus to the relevant result or recovery message. Member fields have clearer labels, focus missing IDs and support Enter to submit.
- **Organization names stay editable by keyboard.** Settings → Organization Overview keeps focus with the name field, saved name or recovery message after saves and reloads.
- **User Access keeps saves consistent.** Unchanged roles no longer trigger saves. Submitted choices stay fixed while saving, and saved results or failed reads return focus to the relevant user or recovery message.
- **Roles show complete rules and usable errors.** Lists and reviews show Allow or Deny and all conditions. Editing retains focus, removal buttons name their permission, Enter submits, and refused saves stay inside the editor.
- **API token errors stay in context.** Refused renames and scope changes show feedback inside the dialog. Revoking a token returns keyboard focus to the saved result or recovery notice.
- **SSO settings and sign-ins stay consistent.** Providers show Enabled or Disabled in text and deletion errors in the dialog. Login input is bounded, provider changes stop delayed setup, and concurrent SAML setup keeps pending logins.
- **Sharing forms support keyboard completion.** Enter submits Create Share, validation messages identify their fields, and saved changes or failed reloads return lost focus to the relevant control or message.
- **Audit controls are easier to use by keyboard.** Settings → Audit Log has reachable help and distinct verification buttons. Removing an audit webhook keeps focus with its saved endpoint.
- **Large fleets do less repeated work.** Workload charts and snapshot discovery avoid copying unused inventory details. Disk and sensor alert cleanup processes only relevant pending resources.

## Alerts and notifications

- **Delivery rehearsals use saved notification routes.** Alerts → Notifications can send labelled alert and recovery samples through email, webhook and Apprise routing, grouping, quiet hours and retries without creating an infrastructure incident.
- **Rehearsal results explain each destination.** Results show accepted, failed, suppressed, pending and unknown outcomes, retain history across reloads and restarts, and show transport details. Confirm receipt in your inbox or service.
- **Maintenance covers connection and datastore alerts.** Proxmox and PBS connection-degraded alerts and PBS datastore-offline alerts now respect matching maintenance windows, including recurring schedules.
- **Steady alerts no longer look like flapping.** Reminders, escalations and delivery retries count as the same firing, avoiding false notification suppression.
- **Finished Docker jobs stay quiet.** Jobs running under an hour that exit successfully without a restart expected avoid offline alerts. Older agents retain previous behavior until they report restart policies.
- **Container offline alerts have visible controls.** Alerts → Thresholds → Application containers lets you disable offline alerts or choose Warn or Crit per container. An explicit level also alerts for finished jobs.
- **Alert consumers cannot disrupt each other.** A failure or change in one notification or Patrol consumer no longer blocks or alters another consumer's alert.
- **Telegram handles special characters.** Built-in alert and recovery templates escape names and messages correctly. Without a dashboard URL, the dashboard pointer stays plain text.
- **Notifications use less temporary memory.** Email attachments, tag routing and built-in webhooks avoid repeated allocation work. Apprise no longer retains command output in memory.

## Disks and storage

- **Longer SMART intervals stay selectable.** Settings → Infrastructure's Proxmox disk-health editor offers 3, 6, 12 and 24 hours, displays saved custom intervals and keeps the standby warning beside the selector.
- **Disk investigations follow space usage deeper.** Patrol and Assistant can follow the largest folder through several levels in one read, staying on the same filesystem and reporting unreadable paths.
- **Refused disk reads suggest useful alternatives.** AI investigations get directory-based alternatives to blocked wildcards and guidance to split commands that require approval.
- **Storage alerts retain useful investigation context.** Patrol receives the full mount path and where to inspect its contents. A PBS datastore capacity alert now opens the datastore rather than its server.

## Proxmox, PBS and backups

- **Stopped guests can start through a node agent.** After admin approval, VM and LXC Start can use a confirmed node agent with commands enabled. Stop and restart still require the action runner.
- **Patrol checks who stopped a guest.** Proxmox task history distinguishes deliberate stops from failures and unsuccessful restarts. Unread history adds a warning to check intent before approving Start.
- **Stopped guests are checked sooner.** Alert-triggered and manual Patrol runs check stopped guests without waiting for the next full scan.
- **Guest filesystem alerts open the correct guest.** Disk alerts now link to their VM or LXC for resource history and Patrol investigations.
- **Guest investigations keep available read access.** Patrol can read inside a running guest through its node agent even when guest power actions are unavailable.
- **Snapshot memory flags reflect saved memory.** VM snapshots no longer appear to include memory simply because they belong to a VM. Missing or invalid flags cannot establish that memory was saved.
- **PBS distinguishes uncertainty from failure.** Proxmox → Backup servers separates warning and unknown states from unreachable servers. Unconfirmed datastore capacity stays unknown instead of appearing empty.
- **Replication distinguishes unknown from pending.** Proxmox → Replication adds an Unknown filter for missing or unrecognized sync outcomes. Missing failure counts no longer appear as zero.
- **Backup health keeps incomplete information visible.** Compact views retain checking and not evaluated counts, inventory completion, running or failed entries and unmatched backups beside the protection summary.
- **AI backup reads find the requested guest or storage.** Patrol and Assistant can list backups by guest name or VMID, Proxmox storage or PBS datastore, avoiding false empty results when a name was supplied.
- **Backup summaries reveal what fills storage.** AI reads put backup counts, sizes and oldest and newest dates by guest before long lists, marking unknown sizes and incomplete totals.

## TrueNAS, vSphere and Docker

- **TrueNAS collection controls take effect.** Settings → Infrastructure honors pool, dataset and replication checkboxes. Disabling datasets also skips ZFS snapshots, while remaining resources stay attached to a collected parent.
- **vSphere collection controls take effect.** VM, host and datastore checkboxes stop detailed collection for excluded resources. Related names stay readable, excluded failures do not fail the poll, and empty views explain where to enable collection.
- **TrueNAS snapshot polling does less work.** On SCALE 25.04 and 25.10, the fallback query requests only the ZFS properties Pulse uses, reducing appliance CPU work with many snapshots.
- **Docker health checks explain failures.** Container details, State tooltips, alerts and Patrol show connection, DNS, timeout, HTTP, TLS or missing-command reasons without sending raw probe output.
- **Failed containers can show recent logs.** Enable the local option in Settings → Infrastructure → Install on a host, or rerun the installer with --enable-failure-logs, to show up to 30 lines and 4 KB from failed Docker or Podman containers.
- **Failure logs stay optional.** Collection defaults off, updates preserve the choice, and --disable-failure-logs turns it off. Masking can miss secrets, and AI reads send logs to your provider. Leave it off for sensitive logs.
- **Docker updates stop on failed image pulls.** Registry errors, interrupted downloads and timeouts report failure before replacing the container. If the image is unchanged, Pulse skips recreation and an unnecessary restart.
- **Interrupted container changes report their outcome.** Failed stops or renames check what actually happened and report whether the original container was restored.
- **Stopped-container updates explain their limits.** The action description says the container stays stopped and the replacement image is not started or health-checked.
- **vCenter errors preserve the connection draft.** Tests and saves keep correctable endpoint errors beside the editor, with guidance to consult the Pulse instance administrator.

## Install, updates and agents

- **Agents preserve saved identity and access.** Unreadable credentials or IDs stop startup instead of replacing identity or enrolling again. State reads require regular files of at most 64 KiB, so restore unavailable storage or linked files before restarting.
- **Enrollment confirms saved access.** Tokens and identities must be stored before authenticated startup. Incomplete success responses stop instead of repeating an exchange that may have consumed the setup token.
- **Read-only deployments keep explicit agent IDs.** Agents configured with --agent-id or PULSE_AGENT_ID can start without writable identity storage. Keep that setting across restarts.
- **TrueNAS CORE agents start at boot again.** The standalone bootstrap enables the service correctly and reports failed stops, starts or unconfirmed running state.
- **Package checks work without an action runner.** Linux agents can report pending APT updates through local inspection. Incomplete output reports failure instead of an incorrect update count, while package changes still require the runner.
- **Update checks explain delivery delays.** Settings → Updates shows when a release is still being prepared for your installation or its status cannot be confirmed, instead of claiming you are up to date.
- **Update channels refresh after saving.** Manual checks replace the old channel's result, and releases still being prepared are checked again sooner.
- **Update commands report what happened.** Help, cancellation or removal no longer ends with an update-complete message. System logger calls have a time limit during unattended update recovery.
- **Unattended updates recheck the installed version.** After waiting for another update, the updater skips an obsolete offer and refuses to replace a newly installed Pro binary or an installation it cannot identify.
- **Proxmox registration stops waiting indefinitely.** Installer API requests have time limits. Unconfirmed registration advises checking Pulse's node list before rerunning, and errors omit response details that could contain credentials.
- **Scripted setup has clearer requirements.** The install guide covers admin, agent and notification setup through the API. Supplied API tokens need at least 32 characters, and install-command errors list host alongside pve and pbs.
- **MCP installer help leaves installations alone.** --help and -h display usage without downloads or changes. Unsupported arguments stop before installation.

## Pulse Pro, AI and hosted

- **Firing alerts still get investigated.** In Ask first and Autopilot, Pulse opens a finding if an alert-triggered scan files none. Sustained warnings also get findings when Pulse can help diagnose or fix them.
- **Manual Patrol checks address the active alert.** Targeted checks examine its cause first and can ask once for a missing finding decision. Summaries no longer call a resource healthy while its alert remains active.
- **Patrol uses the latest collected readings.** Investigations refresh inventory before reading it, so a newly cleared disk alert is not investigated using an older full-disk reading.
- **Separate guest problems stay separate.** A full disk and missing backups on one guest no longer merge into a single storage finding with mismatched investigation details.
- **Findings stay attached to the right resource.** Patrol corrects names or guessed IDs when one resource matches, avoiding duplicate or prematurely closed findings. Unrecognized IDs prompt the AI with matching resources in scope.
- **Removed resources stop leaving open findings.** Patrol closes findings after a short absence check. A replacement container with the same name gets its own investigation, and cleared alerts on removed resources avoid false failed runs.
- **Patrol follows monitoring reloads.** Adding connections or changing polling settings no longer disconnects alert-triggered investigations, flapping checks or threshold reads.
- **Investigations can turn a written fix into a proposal.** If the AI ends with advice but no recorded decision, Pulse asks once for a supported action proposal or conclusion. The final turn can still propose a fix for review.
- **Weekly summaries count verified fixes.** Issues resolved includes findings closed by a fix Pulse ran and confirmed, even between scans.
- **MSP plan choices survive partial results.** The portal keeps available plans usable, labels missing plans or billing periods and offers a reload. Larger self-hosted fleets get a dismissible MSP suggestion in Settings → Infrastructure.
- **Portal verification codes are clearer.** Requests for the same email share a visible one-minute wait across actions. Resend controls and uncertain-delivery messages help avoid conflicting requests.
- **Hosted access follows current billing.** Workspace creation and renewal use current subscriptions. Missing billing remains retryable, while delayed checkout or client edits cannot restore withdrawn access.

## Monitoring and service health

- **Docker shows services that are not responding.** A running container with a failing attached availability check shows Not responding in its State column, with the failing checks named in the tooltip.
- **Container checks survive recreation.** Availability checks follow a replacement container with the same name on the same host only while it still publishes the checked port, avoiding links to a different service.
- **Reviewed Docker ports stay fixed.** Background refreshes cannot replace the ports being reviewed. Adding checks revalidates those endpoints, and switching organizations discards the previous offer.
- **Stopped services can still get findings.** Patrol can flag a service mistaken for a finished job and report failed workloads even when only a person can fix them. Exit codes and out-of-memory status help explain failures.
- **Unavailable actions explain the next step.** Patrol and guest controls distinguish commands being off, an old or unconfirmed node agent, and a missing action runner.
- **Local helper calls stop waiting after cancellation.** Setup and execution share a time limit, shutdown drains connections, and temporary failures back off. An operation already received by the helper may still complete.
- **Hosted recovery protects workspace data.** Recovery waits for running writers to stop and cannot revive an old workspace. Unfinished recovery stays visibly unavailable instead of reporting a healthy client.

## Security

- **Host commands require execute permission.** Assistant file and log reads, Kubernetes logs, Docker update scans and agent checks during action previews now require action-execution permission and the appropriate API token scope.
- **Action reviews follow your permissions.** Inbox fixes require permission to plan, approve and execute. Assistant planning requires plan permission, and starting another finding investigation requires plan and execute permissions.
- **Automatic approval checks command arguments.** Options that change systems, expose process environments or read extra files need approval. Log reads qualify only when every file is under /var/log, and environment overrides are filtered.
- **Recovery tokens require saved records.** Pulse saves token creation and one-time use before granting access, so storage failures cannot authorize an unsaved recovery.
- **API token reloads preserve working access.** Missing or incomplete files cannot erase live tokens. Restored inventories are detected even with older timestamps, failed saves preserve backups, and credential reloads coordinate with concurrent saves.
- **Old requests cannot disrupt replacement sign-ins.** Replies started before a credential change cannot retry with new credentials, sign out the replacement session or switch its organization.
- **Proxy client addresses are checked more carefully.** Configure every proxy hop in PULSE_TRUSTED_PROXY_CIDRS or replace the client-IP header at the final proxy. Shared hosted networks trust only configured ingress proxies.
- **Local access checks are stricter.** Setup and recovery reject forwarded or cross-origin requests as direct local access. A same-host proxy using a loopback Host must send X-Forwarded-For.
- **Organization connections have address limits.** Connections, tests and SMTP reject local-only or metadata addresses, plus private addresses on Pulse Cloud. Saved TrueNAS and vSphere polls retain these limits. Self-hosted LANs remain supported.
- **Registration and redirects protect credentials.** Agent certificate discovery probes only permitted addresses. TrueNAS and vSphere redirects stay on the configured host, and agent errors omit private destinations and response bodies.
- **Saved login files stay private and recoverable.** Passwords, sessions, CSRF and recovery tokens use complete private writes, preserving unread or incomplete inventories before replacement. Invalid file types no longer block startup waiting for a read.
- **The HTTP server avoids a known crash.** Pulse uses an updated Go runtime that fixes a race between reading limited request bodies and writing response headers.

## Other improvements

- **CLI distinguishes missing data from empty lists.** Fleet reads reject absent connection inventories. Action capabilities, history and confirmations must match the requested resource or action.
- **Unconfirmed CLI actions give recovery advice.** Invalid plan, decision or execution replies tell you to check action audit before retrying a request that may already have taken effect.
- **Action reviews explain execution outcomes.** Feedback distinguishes approval from running and points to the recorded result after dispatch. Proposed fixes name the resource they change, and container results use clearer wording.
- **AI reads use the resource under investigation.** Reads that omit a host use the attached resource when it is unambiguous. Unsupported host configuration queries direct the AI to file, log or disk reads instead.
- **Organization controls identify their targets.** Workspace and Billing Admin controls include IDs for repeated names. Settings → Billing & Usage returns lost keyboard focus to refreshed details or a recovery message after reload.
- **Hosted sign-ins open the right workspace.** Fresh sessions land in their client workspace, and an unused default organization stays hidden.
- **Account changes show uncertain outcomes.** If a client change cannot be confirmed, Account labels the last loaded facts and offers Refresh current state without repeating the change. Keyboard position survives recovery.
- **Access settings stop stale edits.** Sharing, ownership and role changes cannot apply an old choice after the session loses the required capability.
- **Alerting and availability have setup guides.** Help now explains routing, delivery checks, incident controls, watchdogs and service checks. The API reference documents the webhook test request body.
- **Settings controls remain reachable.** Change password scrolls on short screens so errors and recovery controls stay accessible. Monitoring toggles show visible keyboard focus.
- **Agent observer files are checked before use.** Observer configuration and token reads validate opened files, enforce size limits and reject unsafe permissions.
- **Telemetry discloses two additional counts.** The preview and privacy documentation include hosts with commands enabled and Docker hosts with failure logs enabled, without host identities or log text.

## Known issues

- **Guest freezes and Windows restarts remain unresolved.** Related fixes do not establish safety for every guest. If monitoring was stopped after a guest froze, do not restart it solely to test this beta or release.
- **Freeze-enabled backups need precautions.** Stop Pulse server monitoring beforehand. Keep it stopped until independent checks confirm thaw, fresh writes on every backed-up filesystem and workload liveness, regardless of an OK backup result.
- **Some freezes have no detectable VM lock.** Cloning or disk imports can freeze a VM without detection. Reads already running before a backup lock may collide with thaw, so avoid manual guest-agent probes during freeze or thaw.
- **Kubernetes pauses need storage and controller checks.** Confirm persistent server storage and prevent controllers replacing stopped pods. Deleting a pod or stopping only the collector does not confirm server polling stopped.
- **Windows disk readings may remain unavailable.** Keep using the working Machines disk view without removing or reinstalling agents, restarting guests or forcing guest-agent checks solely to recover Proxmox readings.
- **Elevated server memory remains unresolved.** Keep the working setup and compare existing readings at similar uptimes and workloads. These optimizations do not prove a lower baseline, and no restart or heap dump is needed solely for this report.
- **Previously affected machines still need caution.** Keep agents or Patrol disabled after a lockup. Preserve existing evidence privately without re-enabling them, changing GPU load or altering hardware to recreate the incident.
- **Resume only previously active monitoring.** Avoid hooks that ignore failed stops or restart Pulse after an unanswered thaw check. Restore only previously active services and timers after independent guest checks.
- **Agent updates still need a version check.** Compare the running agent's reported version with the intended release before treating an update as complete. A successful agent update may leave the privilege helper on an older version.
- **Earlier alert-history gaps cannot be recreated.** Preserve logs and delivery records after storage failures. A healthy current history store does not establish recovery of previously missing entries.
- **Older ARMv6 agents need a manual update.** Their updater can still request ARMv7. Use the agent installer's saved-state update flow with the correct architecture while preserving the agent ID and connection state.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.8.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.8.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.8.0` and recreate the container.
