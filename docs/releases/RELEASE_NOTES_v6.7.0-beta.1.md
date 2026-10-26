# Pulse v6.7.0-beta.1 Release Notes

This beta opens the `v6.7.0` candidate line after stable `v6.6.0`, which remains the rollback target.

Pulse v6.7.0 makes backup status more reliable when reads fail or dates are missing, strengthens configuration recovery, and reduces chart overhead. It also fixes TrueNAS CORE reporting, Mail Gateway queue totals, and several agent setup and Docker update problems.

## Highlights

- Proxmox Backups now counts completed restore points separately from running and failed entries.
- TrueNAS CORE monitoring fixes reporting requests that prevented CPU and memory readings from loading.
- Settings Recovery now rejects incomplete configuration backups before they can replace existing settings.

## What's improved

- **Charts use less server work.** Workload charts read and prepare retained measurements with less decoding, copying and temporary allocation, especially across larger inventories.
- **History stays still while browsing.** Drawer charts reserve space for changing readouts, preventing movement under the pointer. Labels identify peaks that rise above the plotted average.
- **Expanded tables scroll reliably.** Long platform lists account for the height of expanded details, keeping lower content reachable without incorrect gaps or jumps.
- **Sorting works from the keyboard.** Platform table headings now have focusable sort buttons that respond to Enter and Space.
- **Rows stay compact.** Machines, workloads, infrastructure, thresholds and platform tables keep status cues and actions on one line more consistently.
- **Details fit their panel.** Evidence tables and detail cards adapt to the space inside their drawer or split pane, keeping long values readable.
- **Controls are easier to reach.** Settings header actions remain visible, deep drawers scroll correctly, and Infrastructure and Patrol controls have clearer accessible names. Read-only update indicators stay inactive.
- **Dark text is clearer.** Muted text has stronger contrast in dark mode.
- **What's New shows the full changelog.** After an update, change groups appear as plain lists without hidden disclosures, with warnings presented first.

## Alerts and notifications

- **Alert History is easier to scan.** Messages avoid repeating the resource name. Compact actions leave more room for descriptions, while narrower tables wrap consistently.
- **Deleted destinations stay deleted.** Removing a webhook now confirms success to the editor. Queued deliveries cannot borrow permission from another destination using the same URL.
- **Audit webhook drafts survive errors.** Rejected entries remain available for correction, and a completed save does not erase a newer draft typed while it was running.

## Disks and storage

- **Missing disk readings stay unknown.** Host charts and History no longer record unavailable filesystem usage as zero or substitute an unmeasured live value.
- **Disk warnings need real recovery evidence.** Missing agent filesystems or unavailable Proxmox node readings no longer clear open alerts or advance confirmation timing. Linked hosts retain the existing disk alert owner to avoid duplicates.
- **Storage warnings require usable capacity.** Usage thresholds and forecasts ignore missing or contradictory capacity readings. Explicitly disabling the warning still takes effect.

## Proxmox, PBS and backups

- **Incomplete reads preserve history.** Failed or malformed snapshot and storage listings no longer look like successful empty inventories. Disabled or unreadable backup storage cannot erase known history or establish complete coverage.
- **Old evidence does not become fresh again.** Retained PVE snapshots and cached PBS backups from failed reads no longer renew their protection evidence.
- **Coverage explains uncertainty.** Failed protection refreshes withdraw outdated assessments and offer a retry. Provider details include their scope, and empty-list messages distinguish visible rows from previously recorded evidence.
- **Backup dates cannot imply impossible freshness.** Future backup or verification dates leave protection uncertain and explain which clocks to check. Recognised older Proxmox guest identifiers now join the correct Coverage assessment.
- **Backup activity ends correctly.** Finished jobs clear the guest's running-backup indicator. Missing guest log results remain Unknown, and unread cached logs cannot keep renewing running activity.
- **PBS alert timing restarts after gaps.** Missing node-status readings interrupt CPU and memory activation or recovery timing without clearing existing warnings.
- **Cluster links open the right node.** Discovered members use their reported Proxmox address when a short hostname would be unsuitable for the browser.
- **Startup handles duplicate clusters earlier.** Cluster consolidation finishes before monitoring becomes available, avoiding conflicts with agent registration.
- **Backup help explains disagreements.** Recovery guidance distinguishes inventory entries, completed points and daily volume, and shows how to compare local PVE archives with Coverage without starting another backup.
- **Mail Gateway queues show real totals.** Queue counts now come from Postfix queue summaries. Delay estimates use the oldest reported age band and exclude intentionally held mail.

## TrueNAS, vSphere and Docker

- **TrueNAS and vSphere History finds the correct source.** Resource drawers retain the provider's history identity when loading through the resource API.
- **TrueNAS protection dates reflect completion.** Collection time no longer becomes snapshot creation time, including older stored records. Start times and running replication timestamps no longer appear as completed work.
- **Undated replication problems remain visible.** Failed, warning and running states stay visible without invented dates. A reported success without a known run time remains Unknown.
- **Failed protection refreshes are visible.** TrueNAS Protection warns when it is showing retained events and offers Refresh. Its guide explains snapshot, replication and collection limits.
- **Docker updates wait for health checks.** Pulse waits up to three minutes for a replacement's initial checks, detects restarting containers, and leaves verification inconclusive if health is still starting.

## Install, updates and agents

- **Docker setup preserves existing deployments.** The setup helper stops when a Compose configuration already exists, protecting custom ports, mounts, image pins and credentials. It clearly says when configuration is prepared but Pulse has not started.
- **Passwordless sudo works during installation.** The generated Unix agent command recognises NOPASSWD access before asking for a sudo password.
- **Agent names survive profile migration.** Moving to the safe profile preserves the hostname already reported instead of changing it to a fully qualified name.
- **Manual updates avoid helper corruption.** Replacing the update command no longer truncates a script that is still running. Repository-configuration failures preserve existing updater files.
- **Update downloads cannot hang indefinitely.** Manual update helpers use connection and download deadlines and require HTTPS throughout redirects.
- **Installer help keeps tokens private.** Agent examples use a private token file and a downloaded installer that users can inspect, with clearer privilege and cleanup instructions.
- **Unraid removal protects active boot scripts.** Agent uninstall refuses to proceed while an unrelated live boot shell could be affected.

## Pulse Pro, AI and hosted

- **Patrol opens the right resource.** Inbox alerts, findings and resource actions use consistent identities. Findings that repeat an alert fold under that specific alert, while separate problems retain their own decisions.
- **Resource filters cover the whole Patrol view.** Open work counts, filter badges, approvals and summaries now match the resource being browsed.
- **Active Docker problems stay open.** Patrol skips resolution reviews when current container evidence still shows the fault. Missing condition readings or healthy unrelated metrics no longer imply recovery.
- **Demo findings match their resources.** Demo Patrol findings now appear in the corresponding resource views and group with matching alerts.
- **Workspace agent enrolment works consistently.** Install tokens are stored where agent authentication reads them. Registration, diagnostics, command permissions and revocation use the same organisation-bound records.
- **Workspace recovery stays isolated.** Importing configuration reloads only the selected organisation, preserves instance-wide settings, and avoids copying another workspace's connections or credentials into it.
- **MSP billing wording respects cancellation.** The Plan page directs users to Manage billing for renewal or end dates instead of promising automatic renewal. The guide also clarifies purchasing and upgrades.

## Monitoring and service health

- **Restarted servers show agents awaiting reports.** Machines, Infrastructure and resource drawers distinguish saved enrolment from fresh telemetry until the first valid report arrives.
- **Silent agents stop supplying live chart fallbacks.** Expired reporting stops cached readings from appearing current. Stored history remains available while drawers refresh when reporting stops or resumes.

## Security

- **CSV text stays text in spreadsheets.** Report and VM inventory exports protect externally supplied names and other text from being interpreted as formulas.
- **Workspace tokens use current permissions.** API requests recheck the live organisation token store after configuration changes or revocation. An invalid explicit token cannot fall through to browser-session access.
- **Administrative requests reject conflicting fields.** SSO configuration, access recovery, audit filters and audit webhook updates reject ambiguous inputs before applying changes or broadening a read.
- **Audit destination removal must be explicit.** Missing or null destination lists cannot silently remove every audit webhook. An explicit empty list is required.

## Other improvements

- **Configuration recovery saves related data together.** Guest metadata commits with settings and credentials, preventing partial file replacement when a write fails.
- **Repeated imports cannot overwrite newer edits.** Concurrent imports are blocked per workspace, and recently submitted backups are temporarily blocked from being applied again.
- **Transfers stay with the confirmed workspace.** Export and import dialogs show the workspace ID, stop when the selection changes, and keep progress and errors beside the relevant inputs. Saving an API token does not automatically repeat a transfer.
- **Saved and applied are distinguished.** Import failures identify live components that did not reload. An uncertain response warns that changes may already have applied and tells users to inspect the destination before repeating anything.
- **Recovery explains its limits before transfer.** Settings identifies configuration exports as partial backups and lists key exclusions. Migration guidance covers isolated trials, retained storage and handing monitoring over to one server.
- **Unavailable inventory fails clearly.** Manual and scheduled VM inventory exports report unavailable source data instead of producing a successful empty download.
- **Missing web assets return proper errors.** Missing scripts, styles and documentation files return Not Found instead of a successful response containing the application page.

## Known issues

- **Guest safety remains under investigation.** These changes do not confirm a fix for every guest freeze or Windows restart. If monitoring was stopped after a guest froze, do not restart it solely to test this beta or this release.
- **Affected backups still need precautions.** Stop Pulse monitoring before a freeze-enabled backup. Keep it stopped until independent checks confirm thaw, fresh writes on every backed-up filesystem and workload liveness. An OK task does not prove thaw.
- **Kubernetes pauses need controller and storage checks.** Confirm retained server storage before stopping pods, and prevent controllers from replacing them. Deleting a pod or stopping only the collector does not confirm that Pulse server polling has stopped.
- **Elevated server memory remains unresolved.** Reduced chart overhead does not confirm a lower long-running baseline. Report higher steady usage and continued growth using existing readings at comparable uptimes and workloads.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.6.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.6.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.6.0` and recreate the container.
