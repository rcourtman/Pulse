# Pulse v6.9.0-rc.2 Release Notes

This release candidate follows `v6.9.0-rc.1` on the way to stable `v6.9.0`. Stable `v6.8.0` remains the rollback target.

Pulse v6.9.0 opens longer chart history to every plan, groups Compose projects in Docker and lets you try your saved alert routes before you need them. Proxmox and PBS keep more backup results visible when a poll returns incomplete data.

## Highlights

- History charts show up to 90 days of retained metrics on every plan, including Community.
- Docker → Applications groups Compose services by runtime and project, with their service checks and recent failures.
- Alerts → Notifications sends rehearsal alerts and recoveries through your saved email, webhook and Apprise routes.

## What's improved

- **Proxmox node drawers show hardware details.** See the CPU model, thread count, installed RAM, fans and physical ports. Each port shows its name, reported speed and link status, with older readings marked Last known.
- **Proxmox node charts include temperature.** Hover over a node in History mode to see CPU, memory, disk and temperature together. Temperature sparklines in Trends keep their threshold colors.
- **Patrol Inbox shows the answer beside the issue.** Read the investigation conclusion and open the proposed fix from the same item. After Approve and Run, the review says what ran and where to see the result.
- **The workspace switcher keeps your selection.** Retry a failed workspace list in place, and use IDs to tell same-named workspaces apart. After switching, Alerts loads the destination policy and keeps confirmations with the workspace that sent them.
- **Organization Settings keeps saved details visible.** Restart stalled reads in Overview, Access, Billing & Usage or Sharing. Details that Pulse can still confirm stay visible, and controls that depend on a missing list pause.
- **Organization member edits work from the keyboard.** Submit with Enter, jump to a missing ID and see which changes take effect immediately. If an invitation reply is uncertain, Check again reads your access without resending the reply.
- **Sharing makes recipients and resources easier to identify.** Incoming and outgoing shares show the parties, resource and access level. Managers can submit Create Share with Enter, and field errors appear beside the relevant input.
- **Roles and User Access keep your draft during retries.** Restart stalled role, roster and permission reads in place. Rules show Allow or Deny with their conditions, and controls keep focus through saves and refusals.
- **Role edits catch duplicates and skip unnecessary saves.** New Role rejects an ID already in the list. Saving an unchanged role skips the write, and removing a permission moves focus to a remaining control.
- **SSO controls keep results in view.** Provider status uses text, test results stay visible while you edit, and deletion errors stay in the confirmation. Restart a stalled provider check or close SAML metadata Preview without losing your keyboard position.
- **Audit Log keeps key details visible in narrow layouts.** The Event cell shows who acted and when. Filters stay usable while results load, checks apply to the displayed page, and a stalled load can be restarted in place.
- **Audit Webhooks keeps your place after a change.** Restart a stalled read or check the saved destinations after an uncertain change without resending it. After removal, focus returns to a nearby saved endpoint.

## Alerts and notifications

- **Rehearsal reports show what each destination did.** See whether a message was accepted, failed, was suppressed, is waiting or has an unknown result. Reports remain available for seven days, including after restart. Check the receiving service for receipt.
- **Maintenance windows cover more alerts.** Matching one-time and recurring windows now apply to Proxmox and PBS connection warnings and PBS datastore-offline alerts.
- **Reminders no longer inflate flapping counts.** Reminders, escalations and retries count as part of one firing. A problem in one alert listener can no longer change or drop another listener’s delivery.
- **Short container jobs can finish quietly.** Jobs that exit successfully in under an hour and won’t restart skip offline alerts once the agent reports restart policy. Set Warn or Crit in the container row to include it, or Off to silence that container.
- **Mute current and future containers on one Docker runtime.** Find the control under Alerts → Thresholds → Docker → Container Runtimes. Machine alerts, Swarm services, endpoint checks and other runtimes keep their own settings.
- **Mail Gateway queue alerts wait for a confirmed recovery.** Partial counts and unclear message ages no longer clear alerts or downgrade critical ones. Alerts shows gateway-wide and node-specific Message Age warnings together.
- **Built-in Telegram delivery handles special characters and long groups.** Pulse splits oversized groups, shortens messages when needed and waits when rate-limited. Recovery messages link to Alert History when a Pulse URL is configured.
- **ntfy recovery messages include the expected headers.** Receivers can authenticate recoveries and identify their event type the same way they handle firing notifications.
- **Notification delivery uses less temporary memory.** Pulse allocates less memory for email attachments, tag routing and built-in webhook templates, and drains Apprise command output as it arrives.
- **Help explains how to check alert delivery.** The alerting guide covers routes, reminders, incident controls and an external watchdog. The API guide shows how to test a saved webhook or a draft destination.

## Disks and storage

- **Proxmox offers longer SMART polling intervals.** Disk-health settings now offer 3, 6, 12 and 24 hours and preserve existing custom intervals. The drive standby warning stays beside the selector.
- **Machines explains critical Unraid array alerts on the row.** The collapsed row shows the severity and reason for the affected machine, including when an assigned disk is missing.
- **Storage alerts open the affected resource.** PBS capacity alerts open the datastore, and Proxmox filesystem alerts open the affected VM or LXC. Patrol receives the full mount path and where to inspect it.
- **AI can follow the largest folder on a filesystem.** Patrol and Assistant can drill down several levels on the same mount, report unreadable paths and suggest reading the directory if a wildcard needs approval.
- **TrueNAS CORE disk History keeps read and write activity together.** Each disk retains distinct paired samples as time advances, so differing activity from the appliance can appear in the chart.
- **Disk History explains where its readings came from.** It identifies readings supplied by the appliance or held in Pulse’s memory, including charts that use both. Available history remains accessible in narrow drawers without moving the collapse control.

## Proxmox, PBS and backups

- **Backup health flags incomplete coverage.** The protection summary shows pending checks, unevaluated targets, running or failed jobs and unmatched backups. Backup Jobs keeps available counts and offers a retry when history can’t be read.
- **Proxmox keeps results from incomplete task scans.** Overlapping pages with new tasks are still read. Shorter logs, later timeouts and empty task lists preserve guest successes and failures already observed.
- **Backup Jobs ages out old Running results.** A guest becomes Unknown after five minutes without fresh evidence. Coverage labels each guest’s result and leaves its duration unreported if only the parent job’s end is known.
- **Shared backup storage is checked separately.** A scan of same-named local storage can’t cover unread shared storage. Last-known backup files and PBS links stay visible while Pulse waits for a fresh check.
- **PBS preserves last-known failures and restore points.** Unreadable job lists keep sync, verify and prune failures visible across restarts. Incomplete backup group lists keep previous restore points until a complete read.
- **PBS distinguishes unknown from unreachable.** Backup Servers separates warnings and unknown states from confirmed connection failures. Unconfirmed datastore capacity stays unknown, and Replication has an Unknown filter for unconfirmed outcomes.
- **VM snapshots show whether memory was saved.** Pulse displays the memory flag reported by Proxmox and marks it unconfirmed when the value is missing or invalid.
- **Proxmox node drawers restore linked-agent readings.** Current custom readings appear again, including zero values, with their status and any collection errors.
- **Proxmox prefers current memory from the VM’s linked agent.** Pulse can use it even when VM status is unavailable and skips the guest memory file read. Overview → Details shows the source and time under Memory reading.
- **Stopped guests can start through their node agent.** An admin can approve VM or LXC Start when Pulse confirms a supported systemd agent on the Proxmox node with commands enabled. Stop and restart still require an action runner.
- **Patrol checks why a guest stopped.** Manual and alert-triggered checks consult Proxmox tasks for deliberate stops and failed restarts. If task history is unreadable, check why the guest stopped before approving Start.
- **AI backup reads summarize what is using space.** Ask about a guest by name or VMID, a Proxmox storage location or a PBS datastore. Counts, sizes and dates appear before the list, and incomplete totals are marked.

## TrueNAS, vSphere and Docker

- **TrueNAS honors all three collection switches.** Pool, dataset and replication choices stay saved even with all three off. Turning off datasets also skips snapshots. System, disk and native alert readings continue.
- **TrueNAS snapshot polling does less work.** The fallback on SCALE 25.04 and 25.10 requests only the properties Pulse uses. Large snapshot lists also use less temporary memory when building restore points.
- **TrueNAS shows reclaimable ZFS cache separately from free memory.** Open the Memory bar on TrueNAS → Overview for the breakdown. The updated guide explains how to compare it with the Proxmox VM reading.
- **TrueNAS host History gives slow reads time to finish.** Routine refreshes let a read run for up to 150 seconds. Leaving the chart or changing range cancels the old request and stops abandoned CORE REST work without interrupting SCALE polling.
- **Agent profile assignments exclude native-only TrueNAS systems.** A NAS connected through its API remains available for monitoring and History. The profile list requires evidence of an installed Pulse Agent.
- **vSphere honors VM, host and datastore collection choices.** Skipped resources can’t fail a poll. Empty views show how to enable collection, and a rejected vCenter address leaves the draft open for correction.
- **Compose updates use the registered deployment source.** An approved local update edits the Compose file and replaces one service. Git mode pushes a review branch for your normal deployment process.
- **Service deployments shows what rolled out.** Pulse compares the running image with the reviewed image and shows fresh linked HTTP check results separately.
- **Compose projects offer opt-in live logs.** In Docker → Applications, admins can follow the latest 30 lines from a selected container after enabling live logs on its agent. Enable this only where admins may read application logs.
- **Failed containers can report recent logs.** Enable this on the host to collect up to 30 lines and 4 KB per Docker or Podman failure. It’s off by default. Masking can miss secrets, and Patrol or Assistant can send logs to your AI provider.
- **Docker health checks explain failures.** Container details, State tooltips, alerts and Patrol show causes such as DNS, HTTP, TLS, timeout or a missing command. Raw probe output stays on the host.
- **Container updates preserve the original when a pull fails or the image is unchanged.** After an interrupted stop or rename, Pulse checks whether the original was restored. A replacement for a stopped container stays stopped.

## Install, updates and agents

- **Agent startup protects saved identity.** Unreadable credentials or IDs stop startup with an explanation. Restore unavailable storage before restarting. Read-only installs can supply an ID with `--agent-id` or `PULSE_AGENT_ID`.
- **Enrollment confirms that credentials were saved.** The agent saves its token and identity before authenticated startup. An incomplete enrollment reply stops the attempt to avoid reusing a setup token that may already have been spent.
- **Installers prepare replacements before stopping services.** Windows stages the executable. Unix checks the collector at its destination and prepares the optional helper. A preparation failure leaves the running installation available.
- **Unix install errors distinguish URL checks from launch failures.** Preflight says when it hasn’t run a binary. If HTTP validation fails, check that the address resolves only to local or private IPs, or use the server’s HTTPS address.
- **TrueNAS CORE agents start at boot again.** The standalone installer enables the service and reports a failed stop or start, or when it can’t confirm the service is running.
- **APT reporting works without an action runner.** Linux agents report pending package updates locally and mark incomplete results. Installing packages still requires the runner.
- **Settings → Updates explains delivery delays.** See when a release is still being prepared for your installation or when Pulse can’t confirm the latest version. Changing channels refreshes the result, and pending releases are checked sooner.
- **Automatic server updates require a confirmed newer version.** The updater rechecks the installed version after getting its lock, so it skips stale offers and preserves a newly installed Pro binary.
- **Updater results reflect what happened.** Help, cancellation and removal stop reporting a successful update. Unattended updates also limit download size and journal waits and make fewer service-manager calls.
- **Proxmox setup stops waiting when registration times out.** Check the node list before running the installer again. The error leaves out response details that could contain credentials.
- **The install guide covers scripted first setup.** Use the API to create the admin, enroll a host and activate alerts. Supplied API tokens need at least 32 characters, and install help lists `host` alongside `pve` and `pbs`.
- **The MCP installer handles help and custom paths.** `--help` and `-h` print usage without installing. Unknown arguments stop early, and custom install directories are prepared before download.

## Pulse Pro, AI and hosted

- **Patrol follows up on firing alerts.** In Ask first and Autopilot, it opens a finding when an alert run records none, including sustained warnings it can investigate. Manual checks use fresh readings and name active alerts even without a finding.
- **Findings follow the resource and the problem.** Separate problems on one guest get their own findings. Patrol corrects guessed IDs when one resource matches and shows IDs in triage so you can tell same-named resources apart.
- **Removed resources stop leaving findings open.** Patrol checks that a resource is gone before closing a finding, treats same-named replacements separately and avoids failed runs for cleared alerts on removed resources.
- **Investigations can turn written advice into a reviewable fix.** If the AI stops with advice alone, Pulse asks once for a supported proposal or a conclusion. The final turn can still propose a fix.
- **Monitoring reloads keep Patrol connected to alerts.** Adding connections or changing polling settings preserves alert-triggered investigations. Weekly summaries also count confirmed fixes completed between scans.
- **Patrol explains why a workload stopped even without an alert.** It can flag a stopped service and use its exit code or out-of-memory status to explain what happened and whether someone needs to intervene.
- **AI reads use the resource under investigation.** Reads default to its host when the target is clear. Patrol can read inside a running Proxmox guest through a confirmed node agent and explain when missing commands or a runner block a power action.
- **MSP plan choices stay visible when prices are missing.** Account marks unavailable plans or billing periods and offers Reload. Larger self-hosted fleets see a dismissible MSP option in Settings → Infrastructure.
- **Account preserves the last loaded client details after an uncertain change.** Refresh to confirm the saved state without resending the change. Sign-in code requests share a visible one-minute wait for the same email address.
- **Hosted sign-ins open the client’s own workspace.** Fresh client sessions land in that workspace, and the empty default workspace stays hidden.
- **Hosted billing follows the current subscription.** New and resumed workspaces check current billing. Missing billing can be retried, and delayed checkout or client edits can’t restore withdrawn access.
- **Hosted workspace recovery waits for existing processes to stop writing.** Recovery stays unavailable until Pulse can confirm it is safe to proceed. Workspace changes reject incomplete requests and keep pending invitations private.

## Monitoring and service health

- **Docker suggests checks for published web ports.** Review the responding HTTP or HTTPS endpoints before adding checks. Pulse rechecks the selected ports before saving and clears the offer when you switch workspaces.
- **Docker shows when a running service stops responding.** State tooltips name failing attached checks. A check follows a recreated container on the same host when its name and checked port still match.
- **Compose-related failures appear together in Alerts.** Container failures and attached service-check failures can appear in one incident. Each check keeps its own notification settings.
- **Home Assistant Supervisor stops getting false Docker update alerts.** Pulse leaves Supervisor updates to Home Assistant and clears old alerts caused solely by the `latest` image alias.
- **Availability checks catch TCP ports that immediately close.** A port that closes or resets without sending data now fails. Silent connections can take up to a quarter-second longer. Exact UDP matching checks the whole reply.
- **Availability checks identify the device being checked.** Label checks for routers, switches, access points, machines or services. Alerts open the matching check, and Fleet moves focus to an available control if a check disappears.
- **Help explains how to check DNS answers.** Use an HTTP health endpoint that queries the resolver and verifies a known answer. The guide covers local and remote probes and what each protocol can check.
- **History APIs serve the same ranges on every plan.** Metric and availability reads support ranges up to 365 days per request, subject to stored readings. Ending a paid plan no longer deletes saved history or generated reports.
- **Resource APIs report more network details.** Updated agents can report Linux IPv4 default gateways and DNS servers and Docker network mode. Proxmox guests can include bridge, VLAN and firewall settings when available.
- **Large fleets do less repeated work.** Chart filters and snapshot discovery avoid copying unused inventories, and disk and sensor cleanup visits only relevant pending alerts. Startup also does less database work when loading a large first inventory.

## Security

- **Host commands require execute access.** Assistant host reads, Kubernetes logs, Docker update scans and agent checks during action previews require permission to execute actions and the appropriate token scope.
- **Action reviews enforce your role.** Inbox fixes require plan, approve and execute permissions. Assistant planning requires plan permission, and asking Patrol to investigate an existing finding again requires plan and execute permissions.
- **Automatic command approval checks arguments.** Options that change the system, expose process environments or read extra files need approval. Automatic log reads are limited to `/var/log`, and Pulse filters environment overrides.
- **Recovery tokens must be active and saved before use.** Pulse records creation and one-time use before granting access. An expired or ended recovery leaves its token unused.
- **API token changes use current access.** Writes recheck the caller’s scopes and workspace. Incomplete files preserve working tokens, restored inventories can reload with older timestamps, and failed saves retain recovery copies.
- **Old requests can’t disrupt a new sign-in.** Replies from earlier sessions can’t sign out the current user or switch their workspace. Sharing, ownership and role changes recheck access before applying.
- **Client addresses respect the configured proxy chain.** List every trusted hop in `PULSE_TRUSTED_PROXY_CIDRS` or have the last proxy replace the forwarded address. Same-host proxies using a loopback Host need `X-Forwarded-For`.
- **Local setup and recovery check the request origin.** Forwarded and cross-origin requests can’t use direct local access. Security Overview shows the client address Pulse observed, and shared hosted networks trust only configured ingress.
- **Workspace connections enforce destination limits.** Appliance connections and SMTP reject local-only and cloud metadata addresses. Pulse Cloud also blocks private addresses, while self-hosted LAN connections remain supported.
- **Connection checks protect private details.** Appliance redirects stay on the configured host, certificate discovery checks registration permission before connecting, and redirect errors omit private destinations.
- **SSO protects pending sign-ins.** OIDC and SAML limit login input. OIDC checks custom CA files and preserves pending logins after incomplete callbacks. Concurrent SAML setup preserves pending sign-ins.
- **Paid Docker update instructions keep keys out of shell commands.** Settings → Updates directs you to Docker’s hidden password prompt and keeps both pinned Compose commands available.

## Other improvements

- **Saved access files remain recoverable.** Writes preserve unread password, session, CSRF and recovery records. Unsupported file types in sign-in or organization data return an error instead of holding up startup or settings.
- **Configuration restores protect concurrent saves.** Other settings writes wait until the restore finishes so a failed restore can’t erase a successful save.
- **CLI exports preserve your previous backup.** Pulse replaces the destination only after writing a complete private copy. Prompts stay out of redirected archives, and incomplete output reports failure.
- **CLI action results match the request.** Missing fleet inventories return an error. Capabilities, history and confirmations must match the requested action or resource. Check the audit record before retrying an unconfirmed action.
- **Sign-in explains why the local form is hidden.** When Pulse lists no SSO provider, the page explains the missing form. It also keeps keyboard focus during sign-in and explains when an automatic demo attempt didn’t complete.
- **Settings controls stay reachable by keyboard.** Focus stays clear of sticky bars, token errors stay in their dialogs and Change password scrolls in short windows. Restart a stalled API Access list in place, and keep your position after revoking a token.
- **Billing Admin distinguishes same-named organizations.** Controls show IDs where needed, and reloading billing restores focus to the affected row.
- **Monitoring toggles and Help respond predictably.** Toggles show keyboard focus, and Help stays closed if you dismiss it before it finishes opening.
- **Private agent settings reject unsafe files.** Reads check file type, size and permissions, and conflicting settings are refused before they can affect saved credentials.
- **Local agent helper waits respect cancellation.** Setup and execution share a deadline, temporary failures back off, and shutdown lets connections finish. Work the helper already received may still complete.
- **Optional usage statistics show more aggregate counts.** The preview includes hosts with commands or failure logs enabled and how Patrol investigations end after upgrade. Host names and log text stay out of these counts.
- **The HTTP runtime and telemetry dependencies received fixes.** Go fixes a race in HTTP handling, and OpenTelemetry enforces its attribute length limit.

## Known issues

- **Guest freezes and Windows restarts remain unresolved.** If you stopped monitoring after a guest froze, do not restart it solely to test this beta or release.
- **Pause Pulse server monitoring before freeze-enabled backups.** Keep it paused until independent checks confirm thaw, fresh writes on every backed-up filesystem and a running workload, even if the backup reports OK.
- **Some freezes occur without a detectable VM lock.** Cloning a running VM or importing disks can freeze it, and a read already in progress when a backup lock appears can interfere with thaw. Avoid manual guest-agent probes during freeze or thaw.
- **Resume only the services and timers that were active before the pause.** Avoid hooks that ignore failed stops or restart Pulse after a failed or unanswered thaw check. Monitoring and alerts are unavailable while the server is stopped.
- **Kubernetes pauses require storage and controller checks.** Confirm persistent Pulse server storage and prevent controllers from replacing stopped pods. Check that server polling has stopped before starting the backup.
- **Some Windows guests may still lack disk readings in Proxmox.** Use the working Machines disk view. Avoid removing or reinstalling agents, restarting guests or forcing guest-agent checks solely to restore the readings.
- **Elevated Pulse server memory remains under investigation.** Keep your working setup and compare existing readings at similar uptimes and workloads. Skip restarts and heap dumps solely for this report.
- **Keep monitoring paused after a host lockup.** Leave agents or Patrol off if you disabled them. Preserve existing evidence privately and avoid changing GPU load or hardware to reproduce the lockup.
- **Check the running agent version after an update.** Automatic updates can leave the privilege helper on an older version. Use installer maintenance if you need a helper fix and keep the existing agent enrollment.
- **Older ARMv6 agents may need a manual update.** If the updater asks for ARMv7, use the installer with the correct architecture and keep the saved agent ID and connection state.
- **Earlier gaps in alert history can’t be recreated.** Keep existing logs and delivery records after a storage failure, even after the history store is working again.
- **An online TrueNAS CORE pool can show unavailable capacity.** If Usage / Size shows a dash, check the same pool in TrueNAS Storage. Keep the existing key and permissions, and use the pool’s own figures because dataset figures can reflect quotas.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.8.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.8.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.8.0` and recreate the container.
