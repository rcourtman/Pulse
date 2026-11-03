# Pulse v6.8.0-beta.1 Release Notes

This beta opens the `v6.8.0` candidate line after stable `v6.7.0`, which remains the rollback target.

Pulse v6.8.0 adds clearer backup summaries, two weeks of backup job outcomes and individual host sensor thresholds. It also improves notification recovery, agent installation checks and how settings, logs and downloads behave when access changes.

## Highlights

- Backups now groups protection problems by cause and opens Coverage filtered to the affected workloads.
- Alerts → Thresholds → Machine Sensors now lets you set a separate temperature limit for each Linux sensor.
- Backups → Backup servers now shows datastore fill forecasts when a reliable growth trend is available.

## What's improved

- **Latest readings remain visible.** Resource drawers keep CPU, memory and other available readings when history fails or the selected range exceeds the plan. Older readings stay labelled.
- **Settings failures have a recovery path.** Restricted panels hide after an unsuccessful access check and offer a retry instead of retaining stale controls.
- **Saved systems are distinguished from failed reads.** Infrastructure shows connection-list and saved-configuration errors with retry controls instead of presenting empty onboarding or incomplete editors.
- **Role editing preserves existing choices.** Saved permissions remain visible even outside the shortcut list, and unchanged access refreshes preserve drafts.
- **Help handles loading failures clearly.** Missing guides, refused access and connection failures have distinct messages. Retry keeps the same guide, and repeated heading links restore keyboard focus.
- **French is available.** Supported interface surfaces and the getting-started guide now include French.

## Alerts and notifications

- **Notifications survive graceful restarts.** Queued deliveries and buffered alert groups retain their work and grouping deadline. Startup restores saved destinations and delivery policy before sending.
- **Acknowledgements avoid duplicate requests.** Individual, group and bulk actions share pending state across navigation. Partial group failures report both successful and failed counts.
- **Active-alert markers return.** Threshold rows show the red dot for matching open alerts, including machine filesystem and disk-temperature alerts. Help explains what the marker means.
- **Threshold notes survive editing.** Saved notes reappear when reopening an override, and edits that change only the note now refresh correctly.
- **Alert settings recover safely.** Thresholds, destinations, delivery history and alert-intent controls discard results from a previous account or workspace. Unread saved destinations stay hidden until retry succeeds.
- **Delivery errors stay readable.** Recent delivery activity preserves ordinary error messages while continuing to redact credentials in webhook URLs.

## Disks and storage

- **Full disks remain visible in summaries.** Compact Proxmox node rows use the fullest measured disk, including when another disk lacks a reading. Wide host rows retain individual disk bars.
- **Disk controls are easier to read.** Disk, alert and view toggles make the selected option clearer. Narrow disk cells preserve the percentage instead of clipping it beside the disk label.
- **Storage guidance protects retained history.** Help now recommends persistent metric storage and explains that changing the database path neither migrates history nor fixes excessive writes.

## Proxmox, PBS and backups

- **Backup jobs show two weeks of outcomes.** Backups adds daily Proxmox VE job results and a last-24-hours summary, distinguishing failures, warnings, running jobs and gaps in observed history.
- **PBS maintenance results appear per datastore.** Backup servers shows garbage collection, prune, verify and sync outcomes. Status reads now use PBS run-status endpoints, and unclear unrelated task entries no longer mark every datastore unknown.
- **Confirmed backups retain their status.** A freshly read backup can remain protected when another storage location is unreadable. Coverage names the affected node, storage, datastore or namespace without treating retained entries as fresh confirmation.
- **Backups follows a clearer order.** Health comes first, then backup servers and restore points. The restore-points-per-day chart is removed, and metric bars use the dashboard’s usual colours.
- **Backup rows identify the right location.** Server rows lead with datastore names, with each PBS server named once. Restore-point and Coverage rows stay compact and distinguish connections when names overlap. Host figures remain in the server drawer.
- **Ceph manager counts include standby daemons.** Healthy standby managers count as running for API and agent readings. Critical status filters also recognise ERR.
- **Ceph recovery clears old incident details.** A current healthy report no longer inherits warnings from an older or lower-priority source.
- **Node alerts match the affected node.** Proxmox rows include their own physical-disk and local-storage alerts without taking on a guest’s alert just because its node name matches.
- **Node details include power readings.** Proxmox hardware details show power measurements from the linked agent and label retained readings as last known.
- **HTTPS redirect recovery is documented.** Help explains how to correct an existing Proxmox or PBS connection to direct HTTPS while retaining credentials, certificate verification and saved fingerprints.

## TrueNAS, vSphere and Docker

- **Namesake TrueNAS systems stay separate.** Connections with the same hostname no longer merge into one displayed machine.
- **TrueNAS history stays with the selected system.** Drawer requests read only that appliance, discard results from replaced connections and retain native disk I/O beside stored CPU history without claiming one source supplied everything.
- **TrueNAS settings ignore outdated replies.** Tests, saves and access errors remain tied to the account and editor that requested them, including when a draft changes.
- **Docker health indicators update live.** Container row dots follow refreshed health results instead of retaining the first healthy state.
- **Compose respects the image healthcheck.** Bundled and installer-generated Compose files use the HTTPS-aware image probe and preserve its failure result.

## Install, updates and agents

- **Failed startup means failed installation.** The agent installer returns an error when startup verification fails, even if files were installed, and points to service logs.
- **Invalid server URLs leave existing installs intact.** The downloaded agent checks URL policy before the installer replaces services or credentials. Certificate bypass does not silently permit plain HTTP.
- **NAS installation handles older utilities.** URL, hostname and fingerprint processing works with older NAS versions of the text tools used by the installer.
- **Incomplete installer downloads are not executed.** The container-agent wrapper downloads the complete script into a private file and checks its syntax before running it.
- **Command-access setup preserves installed credentials.** Changing the command-execution choice prepares a new token without revoking earlier tokens that agents may still use. Concurrent requests no longer mint the same credential twice.
- **Agent updates retain clearer outcomes.** Binary replacement and persistent NAS copies are made more durable. Failed recovery no longer reports an old update as newly completed.
- **Agent Doctor stays current while open.** Expanded rows update their version, diagnosis and repair commands when fresh fleet information arrives.
- **Helper updates have explicit instructions.** Help and agent logs explain that automatic updates replace the collector only. Updating the privilege helper requires the agent installer’s update flow.
- **Uninstall failures describe stopped services.** When server removal cannot be confirmed, the installer distinguishes retained files from running monitoring and explains how to reconcile the existing registration.
- **Compose setup separates image choices from authentication.** The environment example now explains which values Compose forwards, where runtime settings belong and why legacy token variables do not create v6 tokens.

## Pulse Pro, AI and hosted

- **Reporting preserves selected resources during outages.** A failed resource read offers retry instead of appearing as an empty scope. Previous-account results and downloads cannot populate the current reporting panel.
- **Organization and billing actions require current reads.** Workspace switches and failed reads clear outdated details and controls. Sharing drafts cannot reuse a removed destination.
- **Audit administration stays with the current view.** Audit results, verification feedback and webhook actions discard replies from an earlier account, workspace or page.
- **Patrol requires sensor-specific recovery evidence.** It no longer clears an individual sensor-temperature alert using a snapshot that contains no reading for that sensor.

## Monitoring and service health

- **Buffered reports no longer inflate rates.** Host and Docker disk and network rates use collection times instead of rapid delivery after reconnect. Libvirt CPU and I/O rates use their collection interval too. Invalid intervals remain unknown.
- **System Logs shows connection loss.** Connecting, reconnecting and disconnected states replace a misleading Live label. Uncertain access clears buffered lines and provides an explicit reconnect path.
- **Diagnostics results stay with their request.** Changing account, workspace or panel discards outdated results and feedback instead of displaying them in the new context.
- **Linux sensor collection retains more detail.** Agents report individual temperature and fan inputs with stable identities, including stopped fans and invalid readings. Sensor thresholds remain off until configured.

## Security

- **Sign-out confirms the Pulse session ended.** Proxy logout navigation waits for local confirmation and browser cleanup. Failed confirmation or incomplete cleanup produces a visible warning, and the previous live connection is closed.
- **Private reads stop when access ends.** Metric and availability history, audit data, configuration exports, reports, support bundles and protected progress streams recheck the requesting session or token before sending further data.
- **Settings actions stay with the active account.** API tokens, SSO, User Access and workload metadata discard outdated replies and confirmations after navigation or access changes. Returning requires a fresh read.
- **Support-bundle summaries expose less configuration.** System information uses a limited diagnostic summary and selected environment fields, with the actual Pulse, Go and operating-system versions. Included logs still need private handling.
- **API clients refuse credential-bearing redirects.** CLI, MCP and external-agent requests stay within the configured Pulse origin instead of forwarding credentials to another scheme, host or port.
- **Saved credentials use safer file writes.** Configuration, bootstrap and billing saves avoid reusing stale temporary or backup paths that could expose secrets or overwrite other files.
- **Assistant responses remain non-executable.** Protected JSON responses reject unsafe response formats, and duplicate-import checks better protect retained passphrase fingerprints.
- **Token reveals keep current copy feedback.** Replacing or dismissing a reveal clears outdated dialog feedback. Help explains that closing the dialog does not revoke the token or erase clipboard copies.
- **Network dependencies address security advisories.** The updated Go networking dependency fixes HTML parser and renderer vulnerabilities.

## Other improvements

- **Registration guidance matches authentication requirements.** Help distinguishes public discovery from protected registration and directs users to scoped tokens created in Settings.
- **API examples keep body secrets private.** Guidance uses private request files for passwords and tokens instead of placing them in shell arguments or history.
- **Recovery guidance preserves authentication.** Help prefers authenticated configuration transfers and explains that direct-loopback recovery creates a client-bound session rather than disabling authentication for everyone.

## Known issues

- **Guest freezes and Windows restarts remain unresolved.** Related fixes do not establish recovery for every guest. If monitoring was stopped after a guest froze, do not restart it solely to test this beta or this release.
- **Affected backups still need precautions.** Stop Pulse server monitoring before a freeze-enabled backup. Keep it stopped until independent checks confirm thaw, fresh writes on every backed-up filesystem and workload liveness. An OK task does not prove thaw.
- **Kubernetes pauses need controller and storage checks.** Confirm retained server storage before stopping pods and prevent controllers from replacing them. Deleting a pod or stopping only the collector does not confirm server polling has stopped.
- **Windows disk readings can remain unavailable.** Keep using the existing working Machines disk view. Do not remove or reinstall agents, restart guests or force guest-agent checks solely to recover Proxmox disk readings.
- **Elevated server memory remains unresolved.** Keep the working setup and report elevated steady usage or continued growth from existing readings at comparable uptimes and workloads. These changes do not establish a lower baseline.
- **Previously affected hosts still need caution.** If agents or Patrol were disabled after a host lockup, keep that setup. Preserve existing evidence privately without re-enabling them, changing GPU load or altering hardware to recreate the incident.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.7.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.7.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.7.0` and recreate the container.
