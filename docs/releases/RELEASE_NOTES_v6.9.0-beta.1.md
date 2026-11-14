# Pulse v6.9.0-beta.1 Release Notes

This beta opens the `v6.9.0` candidate line after stable `v6.8.0`, which remains the rollback target.

Pulse v6.9.0 makes Patrol's findings and fixes easier to follow, adds optional failure logs for Docker and Podman, and corrects memory readings for linked Proxmox guests. Agent enrollment, configuration recovery and notification handling are more reliable.

## Highlights

- Patrol Inbox now shows investigation conclusions and opens proposed fixes for review beside the affected item.
- Proxmox guest actions now start stopped VMs and LXCs after admin approval through a node agent with command access.
- Proxmox VM memory now prefers current linked-agent readings, treating ZFS ARC as cache for TrueNAS guests.

## What's improved

- **Configuration restores protect concurrent saves.** Settings changes made during a restore wait for it to finish, so a failed restore cannot silently discard an unrelated successful save.
- **CLI backups preserve the previous file.** Exports replace it only after a complete private write. Passphrase prompts stay out of redirected archives, and incomplete output reports failure.
- **Sharing distinguishes repeated names.** Organization and resource selectors show identifiers beside names. Saved shares keep identities, access levels and decisions readable in narrow panels.
- **Organization Access preserves keyboard focus.** Failed reads and explicit reloads move lost focus to the recovery message or refreshed access details.
- **User Access avoids unchanged saves.** Save Changes stays disabled until assigned roles differ from the saved selection.
- **Large fleets do less repeated work.** Workload charts avoid copying every node's inventory, and disk and sensor alert cleanup collects only the relevant host's pending resources.

## Alerts and notifications

- **Maintenance windows cover more alerts.** Matching Proxmox and PBS connection-degraded alerts and PBS datastore-offline alerts now respect resource maintenance windows, including recurring schedules.
- **Steady alerts no longer look like flapping.** Reminders, escalations and delivery retries count as the same firing, avoiding false notification suppression.
- **Finished Docker jobs stay quiet.** Short jobs that exit cleanly and will not restart avoid offline alerts. Alerts → Thresholds → Application containers adds Offline Alerts controls to override this per container.
- **Alert consumers cannot disrupt each other.** A failure or change in one notification or Patrol consumer no longer blocks or alters another consumer's alert.
- **Telegram handles special characters.** Built-in alert and recovery templates escape names and messages correctly. Without a dashboard URL, the dashboard pointer stays plain text.
- **Notifications use less temporary memory.** Email attachments, tag routing and built-in webhooks avoid repeated allocation work. Apprise no longer retains its command output in memory.

## Proxmox, PBS and backups

- **Patrol checks who stopped a guest.** Proxmox task history distinguishes deliberate stops from failures and unsuccessful restarts. Unread history adds a warning to check intent before approving Start.
- **Stopped guests are checked sooner.** Alert-triggered and manual Patrol runs now run the stopped-guest check, without waiting for the next full scan.
- **Guest filesystem alerts open the correct guest.** Disk alerts now link to their VM or LXC for resource history and Patrol investigations.
- **Backup health keeps unrated counts visible.** Checking and not evaluated totals remain beside the protection summary when space is limited.

## TrueNAS, vSphere and Docker

- **TrueNAS CORE agents start at boot again.** The standalone bootstrap enables the service correctly and reports failed stops, starts or unconfirmed running state.
- **Docker health checks explain failures.** Container details, state tooltips, alerts and Patrol show connection, DNS, timeout, HTTP, TLS or missing-command reasons without sending raw probe output.
- **Failed containers can show their last logs.** Enable the local option in Settings → Infrastructure → Install on a host, or rerun the installer with --enable-failure-logs, to show up to 30 lines and 4 KB in container details.
- **Failure logs stay under your control.** Collection is off by default and updates preserve your choice, but masking can miss secrets and AI investigations can send logs to your provider, so leave it off for sensitive logs.
- **Stopped-container updates explain their limits.** The action description now says the container stays stopped and the replacement image is not started or health-checked.

## Install, updates and agents

- **Agents preserve saved identity and access.** Unreadable credentials or agent IDs stop startup with recovery guidance instead of replacing identity or enrolling again. Restore unavailable storage or linked files before restarting.
- **Enrollment reports success only after saving access.** Tokens and identities must be saved before authenticated startup. Incomplete success responses stop rather than repeat an exchange that may have consumed the setup token.
- **Read-only agent deployments keep explicit IDs.** Agents configured with --agent-id or PULSE_AGENT_ID can start without writable identity storage. Keep that setting across restarts.
- **Update checks distinguish readiness from publication.** Settings → Updates shows when a release is still being prepared for your installation, or its status cannot be confirmed, instead of claiming you are up to date.
- **Update channels refresh immediately after saving.** Manual checks replace the old channel's result, and releases still being prepared are checked again sooner.
- **Update commands report what actually happened.** Help, cancellation or removal no longer ends with a misleading update-complete message. System logger calls have a time limit during unattended update recovery.

## Pulse Pro, AI and hosted

- **Finding reviews lead with the conclusion.** Earlier guesses and full investigation details move into expandable sections. Proposed fixes name the resource they actually change.
- **Firing alerts still get investigated.** In Ask first and Autopilot, Pulse opens a finding from a firing alert if the completed scan files none. Manual and sustained-alert checks can ask once for a missing finding decision.
- **Manual Patrol checks address the active alert.** Targeted checks look at its cause before unrelated facts. Run summaries no longer call a resource healthy while its alert remains active.
- **Patrol keeps separate guest problems separate.** A full disk and missing backups on one guest no longer merge into a single storage finding with mismatched investigation details.
- **Removed resources stop leaving open findings.** Patrol closes findings after a short absence check between runs. A replacement container with the same name gets its own investigation, and cleared alerts on removed resources avoid false failed runs.
- **Stopped services can still get findings.** Patrol can flag a service mistaken for a finished job and report failed workloads even when only a person can fix them. Container exit codes and out-of-memory status help explain the failure.
- **Patrol follows monitoring reloads.** Adding connections or changing polling settings no longer disconnects alert-triggered investigations, flapping checks or threshold reads.
- **Unavailable actions explain the next step.** Patrol and guest action controls distinguish commands being off, an old or unconfirmed node agent, and a missing action runner. Stop and restart still require the runner.
- **Disk investigations adapt to refused commands.** Patrol gets safe directory-size alternatives when wildcards are blocked and guidance to split commands that need approval.
- **Weekly summaries count verified fixes.** Issues resolved includes findings closed by a fix Pulse ran and confirmed, even when that happened between scans.
- **MSP plan offers survive partial results.** The portal keeps available plans usable, labels missing plans or billing periods and offers a reload after a short wait.
- **Portal verification codes are clearer.** Requests for the same email share a visible one-minute wait across actions. Resend controls and uncertain-delivery messages help avoid conflicting requests.

## Monitoring and service health

- **Local helper calls stop waiting after cancellation.** Setup and execution share a time limit, shutdown drains connections, and temporary failures back off. An operation already received by the helper may still complete.

## Security

- **Assistant host commands require action permission.** Reading host files, logs or command output, Kubernetes logs and Docker update scans now require a role and token scope allowed to run actions.
- **Automatic command approval checks arguments.** Options that change systems, expose process environments or read extra files need approval. Log-reading commands qualify only when every file is under /var/log, with environment overrides filtered.
- **Recovery tokens require a saved record.** Pulse saves token creation and one-time use before granting access, so storage failures cannot authorize an unsaved recovery.
- **API token reloads preserve working access.** Missing, truncated or incomplete token files cannot erase the live inventory. Restored inventories are detected even with older timestamps, and failed saves preserve recovery backups.
- **Old requests cannot disrupt replacement sign-ins.** Replies started before a credential change cannot retry with new credentials, sign out the replacement session or switch its organization.
- **Reverse proxies identify clients more safely.** Pulse checks every forwarded hop. For multiple proxies, configure PULSE_TRUSTED_PROXY_CIDRS for each hop or have the final proxy replace the client-IP header.
- **Local access checks are stricter.** Setup and recovery no longer treat forwarded or cross-origin requests as direct local access. A same-host proxy using a loopback Host must send X-Forwarded-For.
- **Organization admins have safer connection limits.** Connections, checks and SMTP block loopback, link-local and metadata addresses, plus private addresses on Pulse Cloud. Self-hosted private LANs work, and instance admins can add restricted addresses.
- **Agent registration checks access before probing.** Certificate discovery contacts only addresses the token may use, preventing registration from probing unrelated endpoints.
- **TrueNAS and vSphere redirects stay on the configured host.** Appliance responses cannot redirect API clients and their credentials to another hostname.
- **Agent diagnostics keep private details out.** Redirect refusals and enrollment errors omit destinations and response bodies that could contain credentials. Observer configuration and token reads validate the opened file and enforce size limits.

## Other improvements

- **CLI results distinguish missing data from empty lists.** Fleet reads reject absent connection inventories. Action capabilities, history and confirmations must match the requested resource or action.
- **Unconfirmed CLI actions give recovery advice.** Invalid plan, decision or execution replies tell you to check action audit before retrying a request that may already have taken effect.
- **Action reviews explain approval and execution.** Feedback distinguishes approval from running, points to the recorded outcome after dispatch and uses plain container-action wording.

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
