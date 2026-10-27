# Pulse v6.7.0-beta.2 Release Notes

This beta follows `v6.7.0-beta.1` on the way to stable `v6.7.0`. Stable `v6.6.0` remains the rollback target.

Pulse v6.7.0 makes backup status clearer when readings fail or dates are missing, fixes TrueNAS CORE monitoring, and strengthens configuration recovery. Charts use less server work, while agent updates, cluster certificate settings and SSO setup handle more failure cases.

## Highlights

- Proxmox Backups now counts completed restore points separately from running and failed entries.
- TrueNAS CORE monitoring fixes reporting requests that prevented CPU and memory readings from loading.
- Settings Recovery now rejects incomplete configuration backups before they can replace existing settings.

## What's improved

- **Charts use less server work.** Workload charts prepare stored measurements with less decoding, copying and temporary allocation. Proxmox live updates also create fewer temporary objects.
- **History stays still while browsing.** Drawer charts reserve space for changing values and times, preventing movement under the pointer. Peak labels stay beside the plotted spike.
- **Expanded tables scroll reliably.** Long platform and backup lists account for expanded details, keeping lower rows reachable without incorrect gaps or sudden jumps.
- **Backup views stay current.** Open Proxmox Backups views refresh inventory automatically. Expanded evidence and badge colours update as backup states change.
- **Sorting works from the keyboard.** Platform table headings have focusable sort buttons that respond to Enter and Space.
- **Rows stay compact.** Machines, workloads, Infrastructure, thresholds and platform tables keep status cues and actions on one line more consistently.
- **Details fit their panel.** Evidence tables and detail cards adapt to their drawer or split pane, keeping long values readable.
- **Controls are easier to reach.** Settings actions remain visible, deep drawers scroll correctly, and Infrastructure and Patrol actions have clearer accessible names. Agent Doctor command blocks support keyboard scrolling.
- **Dark text is clearer.** Muted text has stronger contrast in dark mode.
- **What's New is easier to scan.** After an update, change groups appear as plain lists without hidden disclosures, with warnings presented first.

## Alerts and notifications

- **Alert History avoids repeated names.** Rows show the condition beside the resource instead of naming it again. Compact actions leave more room for descriptions.
- **Deleted destinations stay deleted.** Webhook removal confirms success in the editor. Queued deliveries cannot use another destination's permission just because it shares the same URL.
- **Audit webhook drafts survive errors.** Rejected entries remain available for correction, and finishing a save does not erase a newer draft.
- **Delivery activity supports keyboard browsing.** The retained notification log can be focused and scrolled, and refreshes preserve keyboard position.
- **PBS alert timing restarts after gaps.** Missing CPU or memory readings interrupt activation and recovery timing without clearing existing warnings.

## Disks and storage

- **Missing disk readings stay unknown.** Host charts and History no longer record unavailable filesystem usage as zero or substitute an unmeasured live value.
- **Disk warnings need recovery evidence.** Missing agent filesystems or unavailable Proxmox node readings no longer clear open alerts or advance confirmation timing. Linked hosts retain the established alert owner to avoid duplicates.
- **Storage warnings require usable capacity.** Usage thresholds and forecasts ignore missing or contradictory capacity readings. Explicitly disabling the warning still takes effect.
- **macOS disk lists exclude hidden mounts.** Time Machine and other hidden system mounts no longer appear as ordinary disks by default. Explicit disk includes remain available.
- **Local backups stay independent.** Identical archive paths on separate PVE nodes remain separate copies. A successful read on one node cannot erase retained backups from another node's unread or disabled local disk.
- **Archives stay with the right guest.** PVE identifies VM and LXC backups from archive paths rather than storage names, preserves their type when a VMID is reused, and matches recognised older guest IDs to Coverage.

## Proxmox, PBS and backups

- **Incomplete reads preserve history.** Failed or malformed PVE snapshot and storage listings no longer appear as successful empty inventories. PBS retains unread namespaces when another namespace succeeds, while respecting explicit access refusals.
- **Old evidence cannot become fresh again.** Retained PVE snapshots and cached PBS backups from failed reads no longer renew protection evidence.
- **Coverage handles failed assessments clearly.** Failed or malformed refreshes withdraw outdated protection claims and offer Retry. Large inventories use complete batches, and leaving the view cancels outstanding reads.
- **Partial history gets a clearer warning.** Coverage shows Attention for partially readable backup history where appropriate. Provider details identify their scope, and empty details distinguish the current list from previously recorded evidence.
- **Future dates cannot imply fresh protection.** Backup or verification dates ahead of the Pulse server leave the assessment uncertain and explain which clocks to check.
- **Backup activity ends correctly.** Finished jobs clear the guest's running-backup indicator. Missing guest log results remain Unknown, and unread cached logs cannot keep renewing running activity.
- **Backup activity follows the list filters.** Search, node, type, source, location and verification filters also narrow the chart. Selecting a day narrows only the list, and calendar dates remain correct across clock changes.
- **Unknown sizes remain visible.** Backup volume charts mark days containing completed restore points with unavailable sizes. Bars show known volume, so missing sizes no longer look like zero activity.
- **Prune protection cannot hide verification.** Backups kept during pruning still show Verified or Unverified. Retention appears separately, and unavailable verification is labelled Not reported.
- **Latest task details explain failures.** Expanded Coverage rows show status, start time, duration, error and task ID. The view reminds users that task status does not verify a restore or guest responsiveness.
- **Backup origins are easier to distinguish.** Restore evidence names its reading connection, while workload rows retain their owning PVE connection and guest identity. Search includes backup connection names.
- **Location filters select one repository.** Similar names on different connections no longer merge. Ambiguous older saved filters ask users to choose a location again instead of broadening the results.

## TrueNAS, vSphere and Docker

- **History finds the correct source.** TrueNAS and vSphere resource drawers retain the provider's history identity when loading through the resource API.
- **TrueNAS dates reflect completed work.** Collection time no longer becomes snapshot creation time, including older stored records. Start times and running replication timestamps no longer appear as completed work.
- **Undated replication problems remain visible.** Failed, warning and running states stay visible without invented dates. A reported success without a known run time remains Unknown.
- **Failed protection refreshes are visible.** TrueNAS Protection warns when it shows retained events and offers Refresh. Its guide explains snapshot, replication and collection limits.
- **Docker updates wait for health checks.** Pulse waits up to three minutes for replacement containers' initial checks and detects restarting containers. Verification remains inconclusive if health is still starting.

## Install, updates and agents

- **Docker setup preserves existing deployments.** The helper stops when a Compose configuration already exists, protecting custom ports, mounts, image pins and credentials. It clearly says when configuration is prepared but Pulse has not started.
- **Unix installation handles existing permissions.** Generated commands recognise passwordless sudo, and non-root preflight no longer fails merely because the saved installer directory requires root access.
- **Agent names survive profile migration.** Moving to the safe profile preserves the hostname already reported instead of changing it to a fully qualified name.
- **Automatic updates use the correct version format.** The agent version endpoint now matches the executable's reported version, avoiding a version-format rejection by existing update helpers.
- **Manual updates preserve their helpers.** Replacing the update command no longer truncates a script still running. Repository-configuration failures preserve existing updater files.
- **Update downloads have deadlines.** Manual update helpers bound connection and download time and require HTTPS throughout redirects.
- **Installer help keeps tokens private.** Examples use a private token file and a downloaded installer users can inspect, with clearer privilege and cleanup instructions.
- **NAS upgrades preserve unrelated startup jobs.** Unraid and QNAP updates replace recognised Pulse entries while retaining other boot commands. Unrecognised launch arrangements stop for reconciliation.
- **Unraid removal protects active boot scripts.** Agent uninstall refuses to proceed while an unrelated live boot shell could be affected.
- **Legacy cleanup preserves recovery access.** Cleanup requires a complete inventory, preserves shared SSH key targets, and retains recovery keys when local cleanup cannot finish.
- **Update help explains the failure stage.** Agent guidance separates reporting, download, validation and activation failures, and explains when an uncertain result needs inspection before another manual attempt.

## Pulse Pro, AI and hosted

- **Patrol opens the right resource.** Inbox alerts, findings and actions use consistent resource identities. Findings repeating an alert group under that specific alert, while separate problems keep their own decisions.
- **Resource filters cover the whole Patrol view.** Open work counts, filter badges, approvals and summaries match the resource being browsed.
- **Active Docker problems stay open.** Patrol skips resolution reviews when current container evidence still shows the fault. Missing condition readings or healthy unrelated metrics no longer imply recovery.
- **Demo findings match their resources.** Demo Patrol findings appear in the corresponding resource views and group with matching alerts.
- **Workspace agent enrolment works consistently.** Install tokens are stored where agents authenticate. Registration, diagnostics, command permissions and revocation use the same organisation-bound records.
- **Workspace recovery stays isolated.** Configuration imports reload only the selected organisation, preserve instance-wide settings, and avoid copying another workspace's connections or credentials into it.
- **MSP billing wording respects cancellation.** The Plan page points to Manage billing for renewal or end dates. The guide also explains self-service purchasing, upgrades and applying a purchase.

## Monitoring and service health

- **Restarted servers show agents awaiting reports.** Machines, Infrastructure and resource drawers distinguish saved enrolment from fresh telemetry until the first valid report arrives.
- **Silent agents stop supplying live chart values.** Expired reporting stops cached readings from appearing current. Stored history remains available, and open drawers refresh when reporting stops or resumes.
- **Incomplete guest status is rejected.** Proxmox responses missing usable status or identifying the wrong guest no longer supply monitoring readings or action checks.
- **Cluster links open the right node.** Discovered members use their reported Proxmox address when a short hostname would be unsuitable for the browser.
- **Startup handles duplicate clusters earlier.** Cluster consolidation finishes before monitoring becomes available, avoiding conflicts with agent registration.
- **Expired backup scans stop promptly.** Storage scans stop when their time budget expires or monitoring stops, instead of continuing delays and retries across remaining nodes.
- **Mail Gateway queues show real totals.** Counts come from Postfix queue summaries. Delay estimates use the oldest reported age band and exclude intentionally held mail.
- **Mail Gateway read failures stay specific.** Failed optional reads no longer throttle the entire connection. A reachable gateway with missing readings shows Degraded, and decoding failures are distinguished from network outages.

## Security

- **CSV text stays text in spreadsheets.** Report and VM inventory exports protect externally supplied names and other text from being interpreted as formulas.
- **Workspace tokens use current permissions.** Requests recheck the live organisation token store after configuration changes or revocation. An invalid explicit token cannot fall through to browser-session access.
- **Agent access requires the right authority.** Non-admin browser and proxy users cannot submit agent telemetry, acknowledge Docker commands or read agent configuration containing probe credentials. Scoped agent tokens retain their intended access.
- **Compressed messages have a size limit.** Pulse bounds decompressed incoming WebSocket messages before buffering them, reducing exposure to oversized compressed input.
- **Administrative inputs must be unambiguous.** SSO, access recovery and audit requests reject conflicting fields or malformed selections before changing settings or broadening reads.
- **Audit destination removal must be explicit.** Missing or null destination lists cannot silently remove every audit webhook. An explicit empty list is required.
- **Audit exports have a clear limit.** Exports and summaries reject selections above 10,000 events instead of returning partial results. Narrow the time range, event type or user.
- **Environment-managed SSO stays protected.** Saving other SSO settings cannot replace, remove or take control of a provider configured through environment variables.
- **OIDC tests match sign-in requirements.** Test now rejects missing or mismatched discovery issuers, including trailing-slash differences. Success clearly states that client credentials and sign-in were not tested.
- **SSO errors preserve useful drafts.** Failed saves show readable guidance without automatic write retries. SAML metadata preview errors avoid exposing private responses and return focus to the preview control.
- **Local-login help explains the limit.** Hiding the password form does not disable local password authentication or enforce SSO-only access. The guide clarifies the separate administrator recovery path.
- **Cluster certificate trust is editable per endpoint.** Settings lets users inspect, test and save trust for existing PVE polling endpoints. Changes apply to that endpoint while preserving other members' trust and disabled collection settings.

## Other improvements

- **Configuration recovery saves related data together.** Guest metadata commits with settings and credentials, preventing partial file replacement when a write fails.
- **Repeated imports cannot overwrite newer edits.** Concurrent imports are blocked per workspace, and recently submitted backups are temporarily blocked from being applied again.
- **Transfers stay with the confirmed workspace.** Export and import dialogs show the workspace ID and stop when the selection changes. Progress and errors remain beside the inputs, and saving a token does not repeat the transfer.
- **Saved and applied are distinguished.** Import failures identify live components that did not reload. Uncertain responses warn that changes may already have applied and advise inspecting the destination before repeating the import.
- **Recovery explains its limits before transfer.** Settings identifies configuration exports as partial backups. Migration guidance covers excluded data, access requirements, isolated trials, retained storage and handing monitoring over to one server.
- **Backup help explains disagreements.** Recovery guidance distinguishes inventory entries, completed points and daily volume, and explains comparing local PVE archives with Coverage without starting another backup or changing protection policy.
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
