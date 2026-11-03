# Pulse v6.8.0-beta.1 Release Notes

This beta opens the `v6.8.0` candidate line after stable `v6.7.0`, which remains the rollback target.

Pulse v6.8.0 makes backup problems easier to identify, adds individual temperature sensor limits, and improves notification recovery. It also makes agent installation failures clearer and prevents outdated settings, logs and downloads from following you between accounts or workspaces.

## Highlights

- Backups now groups protection problems by cause and opens Coverage filtered to the affected workloads.
- Alerts → Thresholds → Machine Sensors now lets you set a separate temperature limit for each Linux sensor.
- Backups → Backup servers now shows datastore fill forecasts when a reliable growth trend is available.

## What's improved

- **Latest readings stay visible.** Resource drawers keep available CPU, memory and other readings when history fails or the selected range exceeds your plan. Retained readings remain labelled.
- **Settings offers recovery after access checks fail.** Restricted panels hide outdated controls and offer retry instead of continuing with stale permissions.
- **Infrastructure errors no longer look like first setup.** Failed connection-list and saved-configuration reads show retry controls instead of empty onboarding or incomplete editors.
- **Role editing preserves saved choices.** Existing permissions remain visible outside the shortcut list, and unchanged access refreshes preserve drafts.
- **Help explains loading failures.** Docs distinguishes missing guides, refused access and connection failures. Retry keeps the same guide, and repeated heading links restore keyboard focus.
- **French is available.** Supported interface surfaces and the getting-started guide now include French.

## Alerts and notifications

- **Notifications survive graceful restarts.** Queued deliveries and buffered alert groups keep their work and grouping deadline. Saved destinations and delivery policy load before sending resumes.
- **Acknowledgements avoid duplicate requests.** Individual, group and bulk actions share pending state across navigation. Partial group failures report successful and failed counts separately.
- **Active-alert markers return.** Threshold rows show the red dot for matching open alerts, including machine filesystem and disk-temperature alerts. Help distinguishes these markers from readings and delivery confirmation.
- **Threshold notes survive editing.** Saved notes reappear when reopening an override, and edits that change only the note now refresh correctly.
- **Alert settings stay with your current access.** Thresholds, destinations, delivery history and alert-intent controls discard previous-account or workspace replies. Unread saved destinations stay hidden until retry succeeds.
- **Delivery errors stay readable.** Recent delivery activity preserves ordinary error messages while continuing to hide credentials in webhook URLs.
- **Failed PBS maintenance jobs raise datastore alerts.** Sync, verify, prune and garbage-collection failures also appear in Backup health, separately from guest protection. Outages, partial reads and running retries no longer clear known failures.

## Disks and storage

- **Full disks remain visible in summaries.** Compact Proxmox node rows use the fullest measured disk even when another disk lacks a reading. Wide host rows retain individual disk bars.
- **Disk controls are easier to read.** Disk, alert and view toggles show the selected option more clearly. Narrow disk cells preserve the percentage beside the disk label.
- **Storage guidance protects retained history.** Help recommends persistent metric storage and explains that changing the database path neither migrates history nor fixes excessive writes.

## Proxmox, PBS and backups

- **Backup jobs show two weeks of outcomes.** Backups adds daily Proxmox VE job results and a last-24-hours summary, distinguishing failures, warnings, running jobs and gaps in observed history.
- **PBS job results match the datastore.** Backup servers shows GC, prune, verify and sync outcomes from PBS run-status endpoints. Plain error text counts as failure, and unclear unrelated tasks no longer make every datastore unknown.
- **Confirmed backups retain their status.** A freshly read backup can remain protected when another storage is unreadable. Coverage names affected nodes, storage, datastores or namespaces without treating retained entries as fresh confirmation.
- **Backups opens on Coverage.** The page orders health, backup servers and restore points, removes the restore-points-per-day chart, and uses the dashboard's usual metric-bar colours.
- **Backup servers are counted and grouped.** Each PBS server has one header with reachability and datastore count. Datastore names lead the rows, and host figures remain in the server drawer.
- **Backup lists stay compact and distinct.** Restore-point and Coverage rows remain on one line and distinguish connections when names overlap.
- **Ceph counts and health are more accurate.** Standby managers count as running for API and agent readings. Critical filters recognise ERR, and healthy reports stop inheriting old incident details.
- **Node alert colours match the affected node.** Proxmox rows include their own physical-disk and local-storage alerts without borrowing a guest's alert because its node name matches.
- **Node details include power readings.** Proxmox hardware details show measurements from the linked agent and label retained values as last known.
- **HTTPS recovery keeps saved connections intact.** Help explains how to correct a Proxmox or PBS endpoint to direct HTTPS while preserving credentials, certificate verification and saved fingerprints.

## TrueNAS, vSphere and Docker

- **Namesake TrueNAS systems stay separate.** Connections with the same hostname no longer merge into one displayed machine.
- **TrueNAS history stays with the selected system.** Drawer requests query only that appliance and discard replaced-connection results. Native disk I/O remains beside stored CPU history, with accurate source labels.
- **TrueNAS and vSphere settings ignore outdated replies.** Connection tests, saves and access errors stay with the account, workspace and editor that requested them. Failed saved-configuration reads offer recovery.
- **Docker health indicators update live.** Container row dots follow refreshed health results instead of retaining the initial healthy state.
- **Compose respects the image healthcheck.** Bundled and installer-generated Compose files use the HTTPS-aware image probe and preserve its failure result.

## Install, updates and agents

- **Failed startup means failed installation.** The agent installer returns an error when startup verification fails, even if files were installed, and directs you to service logs.
- **Invalid server URLs leave existing installs intact.** The downloaded agent checks URL policy before services or credentials are replaced. Certificate bypass does not silently permit plain HTTP.
- **NAS installation handles older utilities.** URL, hostname and fingerprint processing works with older NAS text tools.
- **Incomplete installer downloads are not executed.** The container-agent wrapper downloads the whole script into a private file and checks its syntax before running it.
- **Command-access setup preserves installed credentials.** Changing command execution prepares a new token without revoking tokens agents may still use. Concurrent requests no longer create the same install credential twice.
- **Agent updates report more reliable outcomes.** Binary replacement and persistent NAS copies are more durable. Failed recovery no longer presents an earlier update as newly completed.
- **Agent Doctor refreshes expanded rows.** Version, diagnosis and repair commands update as fresh fleet information arrives.
- **Privilege-helper updates have explicit instructions.** Help and agent logs explain that automatic updates replace only the collector. Updating the helper requires the agent installer's update flow.
- **Uninstall failures describe stopped services.** If server removal is unconfirmed, the installer distinguishes retained files from running monitoring and explains how to reconcile the existing registration.
- **Compose setup separates image choice from authentication.** The environment example explains which values Compose forwards, where runtime settings belong and why legacy token variables do not create v6 tokens.
- **Community-scripts LXC installs are recognised again.** Pulse detects the newer update helper so these installations receive the appropriate update controls.

## Pulse Pro, AI and hosted

- **Reporting distinguishes outages from empty scope.** Failed resource reads offer retry. Earlier-account selections, results and downloads cannot populate the current reporting panel, including after live updates reconnect.
- **Organization and billing actions use current details.** Workspace switches and failed reads clear outdated controls. Sharing drafts cannot reuse removed destinations.
- **Container update checks stay in the selected organization.** Infrastructure update lists, summaries and check commands use that organization's agents.
- **Audit administration stays with the current view.** Audit results, verification feedback and webhook actions discard replies from an earlier account, workspace or page.
- **Patrol cannot clear sensor alerts without readings.** Individual sensor-temperature alerts remain open when the review snapshot contains no reading for that sensor.
- **Retired organization monitors stay stopped.** Late requests cannot reopen their stores or restart monitoring during shutdown or runtime replacement.
- **MSP evaluation instructions use the published bundle.** Setup guidance now points to the v6.7.0 provider bundle.
- **MSP networking guidance explains overlapping subnets.** Separate workspaces still share the server's routes, so clients with identical private addresses need distinct reachable addresses or tunnel NAT.
- **Pro terms name the current AI modes.** The terms describe Ask first, Safe auto-fix, Autopilot and connected external agents, including which modes can run fixes.

## Monitoring and service health

- **Buffered reports no longer inflate rates.** Host and Docker disk and network rates use collection times after reconnect. Libvirt CPU and I/O rates use their collection interval too. Invalid intervals remain unknown.
- **System Logs shows connection loss.** Connecting, reconnecting and disconnected states replace a misleading Live label. Uncertain access clears buffered lines and offers an explicit reconnect.
- **Diagnostics discards outdated results.** Switching account, workspace or panel clears earlier results and feedback.
- **Linux sensor collection retains more detail.** Agents report individual temperature and fan inputs with stable identities, including stopped fans and invalid readings. New sensor alerts stay off until configured.

## Security

- **Sign-out confirms the Pulse session ended.** Proxy logout navigation waits for local confirmation and browser cleanup. Failed confirmation or cleanup shows a warning, and the previous live connection closes.
- **Private responses stop when access ends.** History, audit data, configuration exports, reports, support bundles, Assistant results and protected progress streams recheck the requesting session or token before sending more data.
- **Browser results stay with the active account.** Resource lists, agent snapshots, API tokens, SSO, User Access and workload metadata discard outdated replies and confirmations after navigation or access changes.
- **Support-bundle summaries expose less configuration.** System information includes a limited diagnostic summary, selected environment fields and actual software versions. Included logs still need private handling.
- **API clients refuse credential-bearing redirects.** CLI, MCP and external-agent requests stay within the configured Pulse scheme, host and port instead of forwarding credentials elsewhere.
- **Saved credentials use safer file writes.** Configuration, bootstrap and billing saves avoid stale temporary or backup paths that could expose secrets or overwrite other files.
- **Protected responses reject executable formats.** Assistant results and private downloads enforce their expected response formats. Duplicate-import checks also better protect retained passphrase fingerprints.
- **Token reveals keep copy feedback current.** Replacing or dismissing a reveal clears outdated feedback. Help explains that closing the dialog neither revokes the token nor clears clipboard copies.
- **Security dependencies are updated.** Go networking fixes address HTML parser and renderer vulnerabilities, alongside updates to authentication and other runtime libraries.

## Other improvements

- **Registration guidance matches authentication requirements.** Help distinguishes public discovery from protected registration and directs users to scoped tokens in Settings.
- **API examples keep body secrets private.** Password and token examples use private request files instead of shell arguments or history.
- **Recovery guidance preserves authentication.** Help prefers authenticated configuration transfers and explains that direct-loopback recovery creates a client-bound session.
- **Wrong agent-to-node links have recovery instructions.** Help documents the existing unlink API so separate machines can keep reporting without removal or reinstallation.
- **Organization recovery avoids duplicate changes.** Help explains how to reload saved details, access and sharing after failed reads or uncertain saves without recreating missing-looking configuration.
- **Missing Roles controls have clearer guidance.** Help distinguishes licence, Pro runtime and administrator requirements without recommending broader access or a data reset.

## Known issues

- **Guest freezes and Windows restarts remain unresolved.** Related fixes do not establish recovery for every guest. If monitoring was stopped after a guest froze, do not restart it solely to test this beta or release.
- **Affected backups still need precautions.** Stop Pulse server monitoring before freeze-enabled backups. Keep it stopped until independent checks confirm thaw, fresh writes on every backed-up filesystem and workload liveness. An OK task does not prove thaw.
- **Kubernetes pauses need storage and controller checks.** Confirm persistent server storage before stopping pods and prevent controllers replacing them. Deleting a pod or stopping only the collector does not confirm server polling stopped.
- **Windows disk readings may remain unavailable.** Keep using the working Machines disk view. Do not remove or reinstall agents, restart guests or force guest-agent checks solely to recover Proxmox disk readings.
- **Elevated server memory remains unresolved.** Keep the working setup. Use existing readings at comparable uptimes and workloads when reporting high steady usage or growth. These changes do not establish a lower memory baseline.
- **Previously affected hosts still need caution.** If agents or Patrol were disabled after a host lockup, keep that setup. Preserve existing evidence privately without re-enabling them, changing GPU load or altering hardware to recreate the incident.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.7.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.7.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.7.0` and recreate the container.
