# Pulse v6.8.0-beta.1 Release Notes

This beta opens the `v6.8.0` candidate line after stable `v6.7.0`, which remains the rollback target.

Pulse v6.8.0 adds clearer backup health and job history, individual temperature sensor limits and more reliable notification recovery. It also improves agent installation feedback and keeps private settings, logs and downloads tied to your current account and workspace.

## Highlights

- Backups now groups protection problems by cause and opens Coverage filtered to the affected workloads.
- Alerts → Thresholds → Machine Sensors now lets you set a separate temperature limit for each Linux sensor.
- Backups → Backup servers now shows datastore fill forecasts when a reliable growth trend is available.

## What's improved

- **Latest readings stay visible.** Resource drawers keep available CPU, memory and other readings when history fails or a range exceeds your plan. Retained readings stay labelled.
- **Settings recovers from failed access checks.** Restricted panels clear outdated controls and offer retry instead of continuing with stale permissions.
- **Infrastructure failures no longer look like first setup.** Failed connection-list and saved-settings reads offer recovery instead of empty onboarding or incomplete editors.
- **Role editing preserves saved permissions.** Roles shows existing choices outside the shortcut list and keeps drafts through unchanged access refreshes.
- **Access editor actions stay reachable.** Roles and User Access keep save controls and errors visible on short screens, with scrolling inside the forms.
- **Docs explains loading failures.** Missing guides, refused access and connection failures have distinct messages. Retry keeps the same guide, and repeated heading links restore keyboard focus.
- **French is available.** Supported interface surfaces and the getting-started guide now include French.

## Alerts and notifications

- **Notifications survive graceful restarts.** Queued deliveries and buffered alert groups retain their work and grouping deadline. Sending resumes after saved destinations, quiet hours and delivery policy load.
- **Acknowledgements avoid duplicate requests.** Individual, group and bulk actions share pending state across navigation. Partial group failures report successful and failed counts separately.
- **Active-alert dots return to Thresholds.** Matching open alerts mark their metric rows, including machine filesystems and disk temperatures. Help explains that a dot does not confirm notification delivery.
- **Threshold notes stay saved.** Reopening an override restores its note, and edits that change only the note refresh correctly.
- **Alert settings follow current access.** Thresholds, destinations, delivery history and alert-intent controls discard earlier-account or workspace replies. Failed destination reads offer retry.
- **Delivery errors stay readable.** Recent delivery activity preserves ordinary error messages while hiding credentials in webhook URLs.
- **Failed PBS maintenance jobs raise datastore alerts.** Sync, verify, prune and garbage-collection failures appear separately from guest protection. Known failures survive outages, partial reads, running retries and Pulse restarts.

## Disks and storage

- **Full disks remain visible in summaries.** Compact Proxmox node rows show the fullest measured disk even when another disk lacks a reading. Wide host rows retain individual disk bars.
- **Disk controls are easier to read.** Disk, alert and view toggles make the selected option clearer. Narrow disk cells preserve the percentage beside the label.
- **Storage guidance protects history.** Help recommends persistent metric storage and explains that changing the database path neither migrates history nor fixes excessive writes.

## Proxmox, PBS and backups

- **Backup job outcomes cover two weeks.** Backups adds daily Proxmox VE results and clearly labelled last-24-hours totals, distinguishing failures, warnings, running jobs and gaps in observed history.
- **PBS job results match each datastore.** Backup servers reads GC, prune, verify and sync run status from PBS. Plain error text counts as failure, and unclear unrelated tasks no longer make every datastore unknown.
- **Confirmed backups keep their status.** A freshly read backup can remain protected when another storage is unreadable. Coverage names unread nodes, storage, datastores and namespaces without treating retained entries as fresh confirmation.
- **Backups opens on Coverage.** Health, backup servers and restore points follow that order. The restore-points-per-day chart is removed, and bars use the dashboard's usual metric colours.
- **Backup servers are counted and grouped.** Each PBS server has one header with reachability and datastore count. Datastore rows lead with their own names, capacity and jobs. Server CPU, memory and uptime remain in its drawer.
- **Datastore drawers lead with the selected store.** Its name, free space, capacity and job results appear first, with the hosting server one click away.
- **Restore-point totals count completed backups.** By date counts running and failed entries separately, so completed restore-point totals agree with Backup health.
- **Backup lists identify workloads more clearly.** Compact rows distinguish connections with overlapping names. Narrow Coverage rows preserve more of each name and show reasons such as Failed, Overdue and No backup.
- **Backup wording distinguishes guest snapshots.** PBS restore points are called PBS backups, keeping them distinct from guest snapshots that do not count as separate backups.
- **Ceph counts and health are more accurate.** Standby managers count as running in API and agent readings. Critical filters recognise ERR, and healthy reports stop inheriting old incident details.
- **Node colours reflect their own alerts.** Proxmox rows include local-storage and physical-disk alerts without borrowing a guest's alert because its node name matches.
- **Node hardware details show power readings.** Measurements from the linked agent appear in the Proxmox node drawer, with retained values labelled last known.

## TrueNAS, vSphere and Docker

- **Namesake TrueNAS systems stay separate.** Connections with the same hostname no longer merge into one displayed machine.
- **TrueNAS history follows the selected system.** Drawer reads query only that appliance and discard results from removed or replaced connections.
- **Host I/O history stays visible.** Native disk I/O can appear beside stored CPU history when those disk series are missing. Mixed history no longer claims a single source.
- **TrueNAS and vSphere editors ignore outdated replies.** Tests, saves and access errors stay with the account, workspace and editor that requested them. Unread saved settings offer recovery.
- **Docker health indicators update live.** Container row dots follow refreshed health results. Failure words remain readable in narrow rows.
- **Container action results use plain language.** Start, stop, restart and update results explain what Docker did and what Pulse confirmed, including failing or still-starting health checks.
- **Locked container updates point to setup.** The update badge directs you to Settings → Infrastructure → Manage → Remote commands on the affected host.
- **Compose preserves image healthcheck failures.** Bundled and installer-generated Compose files use the HTTPS-aware image probe without masking failures with an HTTP fallback.

## Install, updates and agents

- **Agent installation reports failures honestly.** URL validation runs before replacing services or credentials. Failed startup returns an error and directs you to logs instead of declaring success.
- **NAS installation handles older utilities.** URL, hostname and certificate fingerprint processing works with older NAS text tools.
- **Incomplete container installers are not executed.** The wrapper downloads the complete script into a private file and checks its syntax before running it.
- **Command-access setup preserves installed credentials.** Preparing a replacement token leaves existing agent tokens valid. Concurrent requests no longer create the same install credential twice.
- **Agent updates retain clearer outcomes.** Binary replacement and persistent NAS copies are more durable. Failed recovery no longer reports an earlier update as newly completed.
- **Agent Doctor refreshes open rows.** Versions, diagnoses and repair commands update as fresh fleet information arrives.
- **Privilege-helper updates have instructions.** Help and agent logs explain that automatic updates replace only the collector. Updating the helper requires the agent installer's update flow.
- **Uninstall failures describe stopped services.** If server removal is unconfirmed, the installer distinguishes retained files from running monitoring and explains how to reconcile registration.
- **Compose setup separates image choice from authentication.** The environment example explains forwarded values, runtime settings and why legacy token variables do not create v6 tokens.
- **Community-scripts LXC installs are recognised again.** Pulse detects the newer update helper and shows appropriate update controls.
- **Security settings give usable restart instructions.** Proxmox LXC guidance names the Pulse service, including the older service name, instead of an updater that may exit without restarting it.
- **MCP updates preserve the working adapter.** Failed replacement copies leave the installed adapter intact, and unsafe destination types are refused.

## Pulse Pro, AI and hosted

- **Reporting distinguishes outages from empty scope.** Failed resource reads offer retry. Earlier-account selections, results and downloads cannot populate the current report, including after live updates reconnect.
- **Organization and billing actions use current details.** Workspace switches and failed reads clear outdated controls. Sharing drafts cannot reuse removed destinations, and recovery feedback stays beside the controls.
- **Infrastructure updates use the selected organization.** Update lists, summaries and check commands use that organization's agents.
- **Patrol and AI stay within their organization.** Findings, counts, events, notifications, baseline learning and guest probes use the owning workspace. Deployment choices and scheduler health also use the selected workspace.
- **Patrol keeps sensor alerts open without readings.** A review snapshot with no reading for the affected temperature sensor cannot establish recovery.
- **Stalled lookups no longer block Patrol shutdown.** Slow resource-state reads no longer hold the findings lock while Pulse is stopping.
- **Locked history ranges link to Plans & Billing.** The range notice offers a direct route to plan details. Plans & Billing also introduces the separate Pulse MSP installation and its evaluation.
- **MSP setup asks for values before installing.** First-run setup collects the domain, certificate email and DNS token before package changes. Evaluation instructions point to the published provider bundle.
- **MSP installs can obtain TLS certificates.** Traefik uses the configured certificate email and DNS provider. Setup checks certificate and DNS readiness and explains failures.
- **Add client supports Enter.** The MSP portal submits once while creation is pending and preserves the name for an explicit retry after failure.
- **MSP networking guidance explains overlapping subnets.** Client workspaces still share the server's routes. Identical private addresses need distinct reachable addresses or tunnel NAT.
- **Pro terms describe current AI modes.** The terms explain Ask first, Safe auto-fix, Autopilot and connected external agents, including which modes can run fixes.

## Monitoring and service health

- **Buffered reports no longer inflate rates.** Host and Docker disk and network rates use collection times after reconnect. Libvirt CPU and I/O rates use their collection interval. Invalid intervals remain unknown.
- **System Logs shows connection loss.** Connecting, reconnecting and disconnected states replace a misleading Live label. Uncertain access clears buffered lines and offers an explicit reconnect.
- **Diagnostics clears outdated results.** Switching account, workspace or panel clears earlier results and feedback. Failed reads no longer leave old results available to export.
- **Linux sensors retain more detail.** Agents report individual temperature and fan inputs with stable identities, including stopped fans and invalid readings. New sensor alerts stay off until configured.
- **Retired workspace monitors stay stopped.** Late requests cannot reopen stores or restart monitoring during shutdown or runtime replacement.
- **Discovery handles startup requests safely.** Startup and requests now share access to the discovery service safely.

## Security

- **Sign-out confirms the Pulse session ended.** Proxy logout waits for local confirmation and browser cleanup. Failed confirmation or cleanup shows a warning, and the previous live connection closes.
- **Private responses stop when access ends.** History, audit data, configuration exports, reports, support bundles, Assistant results and progress streams recheck the requesting session or token before sending more data.
- **Browser results stay with the active account.** Resource lists, agent snapshots, API tokens, SSO and workload metadata discard outdated replies and confirmations after navigation or access changes.
- **Audit administration follows the current view.** Audit results, verification feedback and webhook configuration discard replies from an earlier account, workspace or page.
- **Default workspace access is restricted in multi-org mode.** Admins manage members in Settings → Organization access. Eligible existing local users migrate. Review those grants there. Proxy users may need an explicit grant.
- **Administrator checks respect role rules.** Inherited administrator permissions count, while explicit denials still block access.
- **Support bundles expose less configuration.** System summaries use limited diagnostic fields, selected environment values and actual software versions. Included logs still need private handling.
- **API clients refuse redirects to other addresses.** CLI, MCP and external-agent requests keep credentials within the configured Pulse scheme, host and port.
- **Saved credentials use safer file writes.** Configuration, bootstrap and billing saves avoid stale temporary or backup paths that could expose secrets or overwrite other files.
- **Protected responses reject executable formats.** Assistant results and private downloads enforce expected formats. Duplicate-import checks better protect passphrase fingerprints, and portal account responses disable HTTP caching.
- **Private token files work in the container installer.** The wrapper accepts token-file paths. Empty token-file and action-token-file arguments fail instead of falling back to other credentials.
- **Token reveals keep copy feedback current.** Replacing or closing a reveal clears old feedback. Help explains that closing it neither revokes the token nor clears clipboard copies.

## Other improvements

- **Registration guidance matches authentication.** Help distinguishes public discovery from protected registration and directs users to scoped tokens in Settings.
- **API examples keep body secrets private.** Password and token examples use private request files instead of shell arguments or history.
- **Recovery guidance preserves authentication.** Help prefers authenticated configuration transfers and explains that direct-loopback recovery creates a client-bound session.
- **Infrastructure recovery preserves saved connections.** Help explains access failures and direct HTTPS endpoint corrections without deleting connections, replacing credentials or weakening certificate checks.
- **Wrong agent-to-node links have recovery instructions.** Help documents the existing unlink API so separate machines can keep reporting without removal or reinstallation.
- **Organization and role recovery is clearer.** Help explains safe reloads after uncertain changes and distinguishes missing licence, runtime and administrator access without suggesting a data reset.
- **VM I/O guidance explains passed-through storage.** Help clarifies that linking an agent does not change Proxmox VM I/O charts and points to existing agent-backed disk readings.
- **Security libraries are updated.** Go networking fixes address HTML parser and renderer vulnerabilities, alongside updates to authentication and other runtime libraries.

## Known issues

- **Guest freezes and Windows restarts remain unresolved.** Related fixes do not prove recovery for every guest. If monitoring was stopped after a guest froze, do not restart it solely to test this beta or release.
- **Affected backups still need precautions.** Stop Pulse server monitoring before freeze-enabled backups. Keep it stopped until independent checks confirm thaw, fresh writes on every backed-up filesystem and workload liveness. An OK task does not prove thaw.
- **Kubernetes pauses need storage and controller checks.** Confirm persistent server storage before stopping pods and prevent controllers replacing them. Deleting a pod or stopping only the collector does not confirm server polling stopped.
- **Windows disk readings may remain unavailable.** Keep using the working Machines disk view. Do not remove or reinstall agents, restart guests or force guest-agent checks solely to recover Proxmox disk readings.
- **Elevated server memory remains unresolved.** Keep the working setup. Report high steady usage or growth using existing readings at comparable uptimes and workloads. These changes do not establish a lower memory baseline.
- **Previously affected hosts still need caution.** If agents or Patrol were disabled after a host lockup, keep that setup. Preserve evidence privately without re-enabling them, changing GPU load or altering hardware to recreate the incident.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.7.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.7.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.7.0` and recreate the container.
