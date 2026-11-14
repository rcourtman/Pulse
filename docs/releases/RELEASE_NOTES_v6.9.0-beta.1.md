# Pulse v6.9.0-beta.1 Release Notes

This beta opens the `v6.9.0` candidate line after stable `v6.8.0`, which remains the rollback target.

Pulse v6.9.0 makes Patrol investigations and proposed fixes easier to follow, adds optional container failure logs, and improves Proxmox guest memory readings. It also strengthens agent enrollment, configuration recovery, notifications and access controls.

## Highlights

- Patrol Inbox now shows investigation conclusions and opens proposed fixes for review beside the affected item.
- Proxmox guest actions now start stopped VMs and LXCs after approval through a confirmed node agent with commands enabled.
- Proxmox VM memory now prefers current linked-agent readings, counting TrueNAS ZFS ARC as reclaimable cache.

## What's improved

- **Configuration restores protect concurrent saves.** Settings changes wait until a restore finishes, so a failed restore cannot discard an unrelated successful save.
- **CLI backups preserve the previous file.** Exports replace it only after a complete private write. Passphrase prompts stay out of redirected archives, and incomplete output reports failure.
- **Sharing distinguishes repeated names.** Organization and resource selectors show identifiers beside names. Saved shares keep identities, access levels and decisions readable in narrow panels.
- **Organization Access preserves keyboard focus.** Failed reads and explicit reloads direct lost focus to the recovery message or refreshed access details.
- **User Access keeps saves consistent.** Save Changes stays disabled until roles change. While saving, the submitted selection stays fixed and the dialog cannot close or start another permissions request.
- **Role editing keeps keyboard focus.** Changing a permission's action or resource no longer replaces the focused control. Remove buttons identify the permission they affect.
- **Settings errors stay beside the affected controls.** Refused API token renames, scope changes and SSO provider deletions show feedback inside their open dialogs.
- **Large fleets do less repeated work.** Workload charts avoid copying entire node inventories. Snapshot discovery skips unused guest details, and disk and sensor alert cleanup collects only relevant pending resources.

## Alerts and notifications

- **Maintenance windows cover more alerts.** Matching Proxmox and PBS connection-degraded alerts and PBS datastore-offline alerts now respect resource maintenance windows, including recurring schedules.
- **Steady alerts no longer look like flapping.** Reminders, escalations and delivery retries count as the same firing, avoiding false notification suppression.
- **Finished Docker jobs stay quiet.** Jobs running under an hour that exit successfully with no restart expected avoid offline alerts. Older agents keep the previous behavior until they report restart policies.
- **Container offline alerts have visible controls.** Alerts → Thresholds → Application containers lets you disable offline alerts or choose Warn or Crit per container. An explicit level also alerts for finished jobs.
- **Alert consumers cannot disrupt each other.** A failure or change in one notification or Patrol consumer no longer blocks or alters another consumer's alert.
- **Telegram handles special characters.** Built-in alert and recovery templates escape names and messages correctly. Without a dashboard URL, the dashboard pointer stays plain text.
- **Notifications use less temporary memory.** Email attachments, tag routing and built-in webhooks avoid repeated allocation work. Apprise no longer retains its command output in memory.

## Proxmox, PBS and backups

- **Patrol checks who stopped a guest.** Proxmox task history distinguishes deliberate stops from failures and unsuccessful restarts. Unread history adds a warning to check intent before approving Start.
- **Stopped guests are checked sooner.** Alert-triggered and manual Patrol runs now check stopped guests without waiting for the next full scan.
- **Guest filesystem alerts open the correct guest.** Disk alerts now link to their VM or LXC for resource history and Patrol investigations.
- **Snapshot memory flags reflect saved memory.** VM snapshots no longer appear to include memory simply because they belong to a VM. Missing or invalid flags cannot establish that memory was saved.
- **PBS connection states distinguish uncertainty from failure.** Proxmox → Backup servers shows warning or unknown states separately from unreachable servers. Unconfirmed datastore capacity stays unknown rather than appearing empty.
- **Replication distinguishes unknown from pending.** Proxmox → Replication adds an Unknown filter for missing or unrecognized sync outcomes. Missing failure counts no longer appear as zero.
- **Backup health keeps incomplete information visible.** Narrow views retain checking and not evaluated counts, inventory completion, running or failed entries and unmatched backups beside the protection summary.

## TrueNAS, vSphere and Docker

- **TrueNAS collection controls take effect.** Settings → Infrastructure now honors the pool, dataset and replication checkboxes. Disabling datasets also skips ZFS snapshots, while remaining resources stay attached to a collected parent.
- **vSphere collection controls take effect.** The connection's VM, host and datastore checkboxes stop detailed collection for excluded resources. Remaining resources retain readable related names, and excluded inventory failures do not fail the poll.
- **TrueNAS snapshot polling does less work.** On SCALE 25.04 and 25.10, the fallback query requests only the ZFS properties Pulse uses, reducing appliance CPU work when many snapshots exist.
- **TrueNAS CORE agents start at boot again.** The standalone bootstrap enables the service correctly and reports failed stops, starts or unconfirmed running state.
- **Docker health checks explain failures.** Container details, state tooltips, alerts and Patrol show connection, DNS, timeout, HTTP, TLS or missing-command reasons without sending raw probe output.
- **Failed Docker and Podman containers can show recent logs.** Enable the local option in Settings → Infrastructure → Install on a host, or rerun the installer with --enable-failure-logs, to show up to 30 lines and 4 KB in container details.
- **Failure log collection stays optional.** It is off by default, updates preserve the choice, and --disable-failure-logs turns it off. Masking can miss secrets, and AI investigations send collected logs to your provider. Leave it off for sensitive logs.
- **Stopped-container updates explain their limits.** The action description says the container stays stopped and the replacement image is not started or health-checked.
- **vCenter destination refusals preserve the draft.** Connection tests and saves keep correctable endpoint errors beside the editor, with guidance to consult the Pulse instance administrator.

## Install, updates and agents

- **Agents preserve saved identity and access.** Unreadable credentials or agent IDs stop startup with recovery guidance instead of replacing identity or enrolling again. Restore unavailable storage or linked files before restarting.
- **Enrollment succeeds only after access is saved.** Tokens and identities must be stored before authenticated startup. Incomplete success responses stop rather than repeat an exchange that may have consumed the setup token.
- **Read-only agent deployments keep explicit IDs.** Agents configured with --agent-id or PULSE_AGENT_ID can start without writable identity storage. Keep that setting across restarts.
- **Update checks distinguish readiness from publication.** Settings → Updates shows when a release is still being prepared for your installation, or its status cannot be confirmed, instead of claiming you are up to date.
- **Update channels refresh immediately after saving.** Manual checks replace the old channel's result, and releases still being prepared are checked again sooner.
- **Update commands report what happened.** Help, cancellation or removal no longer ends with a misleading update-complete message. System logger calls have a time limit during unattended update recovery.
- **Unattended updates recheck the installed version.** After waiting for another update, the updater skips an obsolete offer and refuses to replace a newly installed Pro binary or an installation it cannot identify.

## Pulse Pro, AI and hosted

- **Firing alerts still get investigated.** In Ask first and Autopilot, Pulse opens a finding from a firing alert if the completed scan files none. Manual and sustained-alert checks can ask once for a missing finding decision.
- **Manual Patrol checks address the active alert.** Targeted checks examine its cause before unrelated facts. Run summaries no longer call a resource healthy while its alert remains active.
- **Separate guest problems stay separate.** A full disk and missing backups on one guest no longer merge into a single storage finding with mismatched investigation details.
- **Removed resources stop leaving open findings.** Patrol closes findings after a short absence check between runs. A replacement container with the same name gets its own investigation, and cleared alerts on removed resources avoid false failed runs.
- **Stopped services can still get findings.** Patrol can flag a service mistaken for a finished job and report failed workloads even when only a person can fix them. Container exit codes and out-of-memory status help explain failures.
- **Patrol follows monitoring reloads.** Adding connections or changing polling settings no longer disconnects alert-triggered investigations, flapping checks or threshold reads.
- **Unavailable actions explain the next step.** Patrol and guest controls distinguish commands being off, an old or unconfirmed node agent, and a missing action runner. Stop and restart still require the runner.
- **Disk investigations adapt to refused commands.** Patrol receives directory-size alternatives when wildcards are blocked and guidance to split commands that require approval.
- **Weekly summaries count verified fixes.** Issues resolved includes findings closed by a fix Pulse ran and confirmed, even when that happened between scans.
- **MSP plan offers survive partial results.** The portal keeps available plans usable, labels missing plans or billing periods and offers a reload after a short wait.
- **Portal verification codes are clearer.** Requests for the same email share a visible one-minute wait across actions. Resend controls and uncertain-delivery messages help avoid conflicting requests.

## Monitoring and service health

- **Local helper calls stop waiting after cancellation.** Setup and execution share a time limit, shutdown drains connections, and temporary failures back off. An operation already received by the helper may still complete.

## Security

- **Host commands require execute permission.** Assistant file and log reads, Kubernetes logs, Docker update scans and agent checks during action previews now require permission to execute actions, including the appropriate API token scope.
- **Action reviews follow the caller's permissions.** Inbox fixes require permission to plan, approve and execute. Assistant planning requires plan permission, and starting another finding investigation requires plan and execute permissions.
- **Automatic command approval checks arguments.** Options that change systems, expose process environments or read extra files need approval. Log reads qualify only when every file is under /var/log, and environment overrides are filtered.
- **Recovery tokens require a saved record.** Pulse saves token creation and one-time use before granting access, so storage failures cannot authorize an unsaved recovery.
- **API token reloads preserve working access.** Missing, truncated or incomplete files cannot erase live tokens. Restored inventories are detected even with older timestamps, and failed saves preserve recovery backups.
- **Old requests cannot disrupt replacement sign-ins.** Replies started before a credential change cannot retry with new credentials, sign out the replacement session or switch its organization.
- **Proxy client addresses are checked more carefully.** Configure every proxy hop in PULSE_TRUSTED_PROXY_CIDRS or replace the client-IP header at the final proxy. Shared hosted networks trust only configured ingress proxies.
- **Local access checks are stricter.** Setup and recovery no longer treat forwarded or cross-origin requests as direct local access. A same-host proxy using a loopback Host must send X-Forwarded-For.
- **Organization connections have address limits.** Connections, checks and SMTP reject local-only or metadata addresses, plus private addresses on Pulse Cloud. Saved TrueNAS and vSphere polls retain the limits. Self-hosted LANs remain supported.
- **Agent registration checks access before probing.** Certificate discovery contacts only addresses the token may use, preventing registration from probing unrelated endpoints.
- **TrueNAS and vSphere redirects stay on the configured host.** Appliance responses cannot redirect API clients and their credentials to another hostname.
- **Agent diagnostics expose fewer private details.** Redirect refusals and enrollment errors omit destinations and response bodies that could contain credentials. Observer configuration and token reads validate opened files and enforce size limits.

## Other improvements

- **CLI results distinguish missing data from empty lists.** Fleet reads reject absent connection inventories. Action capabilities, history and confirmations must match the requested resource or action.
- **Unconfirmed CLI actions give recovery advice.** Invalid plan, decision or execution replies tell you to check action audit before retrying a request that may already have taken effect.
- **Action reviews explain execution outcomes.** Feedback distinguishes approval from running and points to the recorded result after dispatch. Proposed fixes name the resource they change, and container action results use clearer wording.
- **Telemetry discloses two additional counts.** The preview and privacy documentation include hosts with commands enabled and Docker hosts with failure logs enabled. These counts contain no host identities or log text.

## Known issues

- **Guest freezes and Windows restarts remain unresolved.** Related fixes do not establish safety for every guest. If monitoring was stopped after a guest froze, do not restart it solely to test this beta or release.
- **Freeze-enabled backups still need precautions.** Stop Pulse server monitoring beforehand. Keep it stopped until independent checks confirm thaw, fresh writes on every backed-up filesystem and workload liveness, regardless of an OK backup result.
- **Some freezes have no detectable VM lock.** Cloning or disk imports can freeze a VM without detection. Reads already running before a backup lock may collide with thaw, so avoid manual guest-agent probes during freeze or thaw.
- **Kubernetes pauses need storage and controller checks.** Confirm persistent server storage and prevent controllers replacing stopped pods. Deleting a pod or stopping only the collector does not confirm server polling stopped.
- **Some Windows disk readings may remain unavailable.** Keep using the working Machines disk view without removing or reinstalling agents, restarting guests or forcing guest-agent checks solely to recover Proxmox readings.
- **Elevated server memory remains unresolved.** Keep the working setup and compare existing readings at similar uptimes and workloads. These optimizations do not prove a lower baseline. No restart or heap dump is needed solely for this report.
- **Previously affected machines still need caution.** Keep agents or Patrol disabled after a lockup. Preserve existing evidence privately without re-enabling them, changing GPU load or altering hardware to recreate the incident.
- **Resume only previously active monitoring.** Avoid hooks that ignore failed stops or restart Pulse after an unanswered thaw check. Restore only previously active services and timers after independent guest checks.
- **Agent updates still need a version check.** Compare the running agent's reported version with the intended release before treating an update as complete, especially for older installed agents.
- **Earlier alert-history gaps cannot be recreated.** Preserve existing logs and delivery records after storage failures. A healthy current history store does not establish recovery of previously missing entries.
- **Older ARMv6 agents need a manual update.** Their updater can still request ARMv7. Use the agent installer's saved-state update flow with the correct architecture while preserving the agent ID and connection state.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.8.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.8.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.8.0` and recreate the container.
