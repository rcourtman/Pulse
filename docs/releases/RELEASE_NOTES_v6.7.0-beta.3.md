# Pulse v6.7.0-beta.3 Release Notes

This beta follows `v6.7.0-beta.2` on the way to stable `v6.7.0`. Stable `v6.6.0` remains the rollback target.

Pulse v6.7.0 makes backup status clearer when reads fail, dates are uncertain or tasks disagree. It strengthens configuration recovery, reduces chart processing, restores TrueNAS CORE readings, and improves disk visibility, agent updates and SSO settings.

## Highlights

- Proxmox Backups now counts completed restore points separately from running and failed entries.
- TrueNAS CORE monitoring now loads CPU, memory and History readings previously blocked by rejected reporting requests.
- Settings Recovery now rejects incomplete configuration backups before they can replace existing settings.

## What's improved

- **Charts need less processing.** Workload history uses less decoding, copying and temporary memory. Proxmox live updates also create fewer temporary objects.
- **History stays still while browsing.** Drawer charts reserve room for changing values and times, keeping controls steady under the pointer. Peak labels stay beside plotted spikes.
- **Expanded lists scroll correctly.** Platform and backup tables account for expanded details, keeping lower rows reachable without unexpected gaps or jumps.
- **Backup browsing survives refreshes.** Backups updates readings and badge colours while preserving the selected view, filters, expanded details, scroll position and chart focus.
- **Retry preserves keyboard focus.** Backups keeps a usable focus position through failed reads, retries and recovery, including when an error message disappears.
- **Sorting works from the keyboard.** Platform table headings have focusable sort buttons that respond to Enter and Space.
- **Rows and details stay readable.** Machines, Workloads, Infrastructure, thresholds and platform tables keep status cues and actions compact. Detail tables fit their available panel width.
- **Controls remain reachable.** Settings actions stay visible, deep drawers scroll properly, and Infrastructure and Patrol actions have clearer accessible names. Agent Doctor commands support keyboard scrolling.
- **SSO addresses stay readable.** Settings keeps complete callback, metadata and ACS URLs beside their copy controls, including in narrow panels.
- **Dark labels have clearer contrast.** Muted text is easier to read in dark mode.
- **What's New is easier to scan.** After an update, changes appear as plain lists with warnings first, without hidden groups.

## Alerts and notifications

- **Alert History avoids repeated names.** Rows show the condition beside the resource without naming it again. Compact actions leave more room for descriptions.
- **Deleted destinations stay deleted.** Webhook removal confirms success in the editor. Queued deliveries cannot use another destination's permission because it shares the same URL.
- **Audit webhook drafts survive errors.** Rejected entries remain available for correction, and a completed save does not erase a newer draft.
- **Delivery activity supports keyboard browsing.** The notification log can be focused and scrolled, and refreshes preserve keyboard position.
- **PBS timing restarts after missing readings.** Gaps in CPU or memory data interrupt activation and recovery timing without clearing existing warnings.
- **Docker alert controls show their scope.** Thresholds removes ineffective runtime switches. Help explains that Offline Alerts covers runtime connectivity, while containers have separate settings.
- **Recovery rules are clearer.** Troubleshooting explains why falling usage can leave an alert open, including clear thresholds, recovery delays, averaged readings and missing observations.

## Disks and storage

- **Real container data disks remain visible.** Filesystem monitoring includes real mounts under /var/lib/docker and /var/lib/containers while continuing to exclude container layers.
- **Disk totals avoid counting aliases twice.** Workloads summaries use the supplied aggregate capacity when available. Individual mount readings and their warning colours remain inspectable.
- **Disk cards show the correct percentage.** Resource disk summaries no longer interpret a usage ratio as an already calculated percentage.
- **Missing disk readings stay unknown.** Host charts and History no longer record unavailable filesystem usage as zero or substitute an unmeasured live value.
- **Disk warnings need recovery evidence.** Missing agent filesystems or unavailable Proxmox readings cannot clear alerts or advance confirmation timing. Linked hosts retain the established alert owner to prevent duplicates.
- **Storage warnings require usable capacity.** Usage thresholds and forecasts ignore missing or contradictory capacity readings. Explicitly disabling a warning still takes effect.
- **macOS excludes hidden mounts.** Time Machine and other hidden system mounts no longer appear as ordinary disks by default. Explicit disk includes remain available.
- **Local backup copies stay independent.** Identical paths on separate PVE nodes remain separate copies. Failed reads, disabled storage and running files stay scoped to the correct local disk.
- **Archives and snapshots stay with the right guest type.** PVE archive paths identify VM and LXC backups. Reused VMIDs, retained snapshots and unknown archive types cannot give the wrong guest protection credit.
- **Missing-filesystem help avoids storage changes.** Guidance distinguishes real data disks from excluded layers and remote mounts without asking users to move data or change backup selection.

## Proxmox, PBS and backups

- **Incomplete reads preserve inventory.** Failed or malformed PVE snapshot and storage listings cannot erase retained entries. PBS keeps unread namespaces and files through temporary failures, including timeouts and throttling.
- **Access refusals remain meaningful.** Backup collection stops continuity reads after explicit permission refusals. A readable sibling namespace cannot establish that an unread namespace is empty.
- **Old evidence cannot become fresh again.** Retained PVE snapshots and cached PBS backups from failed reads no longer renew protection evidence. Recognised older guest identities still match Coverage.
- **Failed assessments withdraw outdated claims.** Coverage rejects failed, malformed or inconsistent assessment responses and offers Retry. Large inventories use complete batches, and leaving the view cancels outstanding reads.
- **Uncertain evidence remains inspectable.** Coverage retains restore details when assessments fail, identifies provider scope and distinguishes current from retained evidence. Partial history shows Attention where appropriate.
- **Assessment times are visible.** Expanded Coverage rows show when protection was evaluated, the latest successful point and the last verification, independently of the currently filtered list.
- **Uncertain dates cannot imply protection.** Backup ages, Workloads completion labels, TrueNAS Protection and verification summaries reject malformed or ambiguous dates. Future dates stay uncertain and identify clocks to check.
- **Ended jobs stop looking active.** Completed jobs clear running-backup indicators. Missing guest results stay Unknown, and unread cached logs cannot keep renewing activity.
- **Activity follows inventory filters.** Search, node, type, source, location and verification filters narrow the chart. Day selection narrows only the list. Calendar dates remain correct across clock changes.
- **Unknown sizes and retention stay distinct.** Activity charts mark completed points with unavailable sizes. Prune retention appears separately from Verified or Unverified, and missing verification says Not reported.
- **Task failures have inspectable details.** Coverage shows status, start time, duration, error, task ID and origin. Uncertain task ordering exposes possible latest tasks. Workload and restore-point identities remain readable without hover.
- **Backup locations keep their boundaries.** Similar repositories on different connections stay separate. Restore evidence names its reading connection, while notices explain that location filters do not recalculate workload-wide protection.

## TrueNAS, vSphere and Docker

- **CORE memory separates Free and ARC correctly.** Compatible TrueNAS CORE 13 systems can supply both values from one authenticated native reading instead of combining differently timed measurements.
- **Memory composition is accessible.** The TrueNAS Overview memory bar opens a breakdown with the keyboard or a click. Help explains Used, reclaimable ARC cache and Free.
- **CORE disk throughput reaches History.** TrueNAS Storage retains physical-disk read and write rates through charts and stored history. Help distinguishes individual disk activity from host totals and capacity.
- **History finds the correct source.** TrueNAS and vSphere drawers preserve the provider's history identity when loading through the resource API.
- **Protection dates reflect completed work.** Collection time no longer becomes snapshot creation time, including older stored records. Start times and running replication timestamps cannot appear as completed work.
- **Undated replication problems stay visible.** Failed, warning and running states remain visible without invented dates. A reported success without a known run time stays Unknown.
- **Failed protection refreshes are visible.** TrueNAS Protection warns when showing retained events and offers Refresh. Help explains the limits of snapshot, replication and collection evidence.
- **Docker updates wait for health checks.** Pulse waits up to three minutes for replacement containers' initial checks and detects restarting containers. Verification stays inconclusive while health is still starting.

## Install, updates and agents

- **Docker setup preserves existing deployments.** The helper stops when Compose configuration already exists, protecting custom ports, mounts, image pins and credentials. Prepared configuration is distinguished from a running server.
- **Unix installation handles existing permissions.** Generated commands recognise passwordless sudo. Non-root preflight no longer fails because the saved installer directory requires root access.
- **Agent names survive profile migration.** Moving to the safe profile preserves the hostname already reported instead of replacing it with a fully qualified name.
- **Agent updates match the executable version.** Version checks use the same format as the installed binary. Lifecycle discovery also recovers when a peer stream is cancelled.
- **Automatic server updates finish promptly.** Successful updates no longer wait on an unsupported completion-notification request.
- **Manual update helpers remain usable.** Replacement avoids truncating a running script, and preparation failures preserve existing helpers. Bootstrap downloads have deadlines and require HTTPS through redirects.
- **Installer guidance keeps tokens private.** Examples use a private token file and a downloaded installer users can inspect, with clearer privilege and cleanup instructions.
- **NAS upgrades preserve other startup jobs.** Unraid and QNAP updates replace recognised Pulse entries while retaining unrelated boot commands. Unrecognised arrangements stop with recovery files retained.
- **Unraid removal protects active boot scripts.** Agent uninstall refuses to proceed while an unrelated live boot shell could be affected.
- **Legacy cleanup preserves recovery access.** Cleanup requires complete inventory, preserves shared SSH key targets and retains recovery keys when local cleanup cannot finish.
- **Proxmox installation leaves existing guests alone.** Creating Pulse no longer performs legacy mount cleanup on other LXCs. Failed or interrupted installs retain the new container for inspection and recovery.
- **Removal failures preserve recovery data.** Server configuration stays until earlier removal steps succeed. Windows agent state remains until unregistration is confirmed, and unattended failures return without waiting for a dialogue.

## Pulse Pro, AI and hosted

- **Patrol opens the right resource.** Inbox alerts, findings and actions use consistent identities. Findings repeating an alert group under that specific alert, while separate problems retain their own decisions.
- **Resource filters cover the whole Patrol view.** Open work counts, filter badges, approvals and summaries match the resource being browsed.
- **Active Docker problems stay open.** Patrol skips resolution reviews when current container evidence still shows the fault. Missing condition readings or healthy unrelated metrics cannot imply recovery.
- **Demo findings match their resources.** Demo Patrol findings appear in the corresponding resource views and group with matching alerts.
- **External agents can read recent changes.** The API and MCP tool list_recent_changes expose observed fleet changes with time, resource, kind and source filters. Results identify when more changes matched than were returned.
- **Agent event streams announce more changes.** API and MCP subscribers receive alerts firing or resolving and resource status changes. Rapid status changes are grouped, with individual transitions available in history.
- **Pulse browsing no longer counts as external agent use.** Opening Pulse pages with setup or kiosk authentication no longer inflates external-agent activity records.
- **Workspace agent enrolment is consistent.** Install tokens live where agents authenticate. Registration, diagnostics, command permissions and revocation use the same organisation-bound records.
- **Workspace recovery stays isolated.** Imports reload only the selected organisation, preserve instance-wide settings and avoid copying another workspace's connections or credentials.
- **MSP billing respects cancellation.** The Plan page points to Manage billing for renewal or end dates. Help explains self-service purchasing, upgrades and applying a purchase.

## Monitoring and service health

- **Restarted servers show agents awaiting reports.** Machines, Infrastructure and resource drawers distinguish saved enrolment from fresh telemetry until the first valid report arrives.
- **Silent agents stop supplying live values.** Expired reporting stops cached readings from appearing current in charts. Stored history remains available, and open drawers refresh when reporting stops or resumes.
- **Missing backup times stay cautionary.** Workloads says No completed backup reported rather than implying a guest has never been backed up. A running backup is no longer described as its first.
- **Incomplete guest status is rejected.** Proxmox responses missing usable status or identifying the wrong guest cannot supply monitoring readings or action checks.
- **Cluster links open the right node.** Discovered members use their reported Proxmox address when a short hostname would be unsuitable for the browser.
- **Startup handles duplicate clusters earlier.** Consolidation finishes before monitoring becomes available, avoiding conflicts with agent registration.
- **Expired backup scans stop promptly.** Storage scans stop when their time budget expires or monitoring stops, instead of continuing retries across remaining nodes.
- **PBS datastore fullness matches its bytes.** Backups calculates the percentage and warning colour from the displayed used and total space, avoiding contradictory optional percentages.
- **Mail Gateway queues show real totals.** Counts come from Postfix queue summaries. Delay estimates use the oldest reported age band and exclude intentionally held mail.
- **Mail Gateway failures stay specific.** Failed optional reads no longer throttle the whole connection. Reachable gateways with missing readings show Degraded, while decoding failures remain distinct from network outages.
- **Mail Gateway collection controls are explained.** Help clarifies which saved switches stop reads and why disabling a failing read removes coverage rather than repairing it.

## Security

- **CSV text stays text in spreadsheets.** Reports, VM inventory and audit exports protect supplied names and other text from becoming formulas. Audit checks and JSON exports retain original event data.
- **Workspace tokens use current permissions.** Requests recheck the live organisation token store after configuration changes or revocation. An invalid explicit token cannot fall through to browser-session access.
- **Agent access requires appropriate permissions.** Non-admin browser and proxy users cannot submit telemetry, acknowledge Docker commands or read agent configuration containing probe credentials.
- **Recent change reads protect command details.** Monitoring-read tokens receive redacted command text, output and runbook messages unless they also have execution permission.
- **Compressed messages have a size limit.** Pulse limits decompressed incoming WebSocket messages before buffering them, reducing exposure to oversized compressed input.
- **Administrative selections must be unambiguous.** SSO, access recovery, RBAC checks and audit requests reject conflicting fields or malformed selections. Removing all audit destinations requires an explicit empty list.
- **Audit exports have a clear limit.** Exports and summaries reject selections above 10,000 events instead of returning partial results. Narrow the time range, event type or user.
- **SSO edits preserve protected settings.** Ordinary edits retain advanced options the editor cannot display. Environment-managed providers cannot be replaced, removed or taken over through saved settings.
- **SSO tests describe their limits.** OIDC requires the same issuer match as sign-in, including trailing slashes. SAML checks require an identity and usable SSO URL. Neither test claims to have completed sign-in.
- **SSO errors preserve useful drafts.** Failed saves give readable guidance without automatic write retries. OIDC tests and SAML previews keep private responses out of errors, and preview failures restore focus.
- **Cluster certificate trust is editable per endpoint.** Settings lets users inspect, test and save trust for existing PVE polling endpoints while preserving other members' trust and disabled collection settings.
- **OIDC verification is hardened.** Updated key and token verification dependencies address security flaws and limit oversized RSA key input.

## Other improvements

- **Configuration recovery saves related data together.** Guest metadata commits with settings and credentials. Failed writes restore previous files and preserve older recovery backups instead of leaving a partial replacement.
- **Repeated imports cannot overwrite newer edits.** Concurrent imports are blocked per workspace, and recently submitted backups are temporarily blocked from being applied again.
- **Transfers stay with the confirmed workspace.** Export and import dialogs show the workspace ID and stop if selection changes. Feedback stays beside the inputs, and saving a token does not repeat the transfer.
- **Saved and applied are distinguished.** Import failures identify components that did not reload. Uncertain responses warn that changes may already have applied and advise inspecting the destination before repeating the import.
- **Recovery explains its limits before transfer.** Settings identifies configuration exports as partial backups. Migration help covers excluded data, access requirements, isolated trials, retained storage and monitoring handover.
- **Backup help explains conflicting readings.** Guidance separates inventory entries, completed points, daily volume, task outcomes and guest placement so users can compare existing records without starting another backup.
- **Agent update failures are clearer.** Help distinguishes download, validation and activation failures before another manual attempt.
- **Local-login help explains the limit.** Hiding the password form does not disable local password authentication or enforce SSO-only access. Guidance identifies the separate administrator recovery path.
- **Unavailable inventory fails clearly.** Manual and scheduled VM inventory exports report unavailable source data instead of producing a successful empty download.
- **Help stays within available documentation.** The index avoids unshipped guides. Missing scripts, styles and documentation return Not Found instead of a successful response containing the application page.
- **Update guidance distinguishes policy from availability.** Help explains that an approved stable patch or a merged fix does not mean an update has been published.

## Known issues

- **Guest safety remains unresolved.** These fixes do not establish recovery for every guest freeze or Windows restart. If monitoring was stopped after a guest froze, do not restart it solely to test this beta or this release.
- **Affected backups still need precautions.** Stop Pulse monitoring before a freeze-enabled backup. Keep it stopped until independent checks confirm thaw, fresh writes on every backed-up filesystem and workload liveness. An OK task does not prove thaw.
- **Kubernetes pauses need controller and storage checks.** Confirm retained server storage before stopping pods, and prevent controllers from replacing them. Deleting a pod or stopping only the collector does not confirm server polling has stopped.
- **Elevated server memory remains unresolved.** Chart improvements do not establish a lower baseline. Keep the working setup and report elevated steady usage or continued growth from existing readings at comparable uptimes and workloads.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.6.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.6.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.6.0` and recreate the container.
