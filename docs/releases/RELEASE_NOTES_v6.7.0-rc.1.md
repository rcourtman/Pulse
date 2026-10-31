# Pulse v6.7.0-rc.1 Release Notes

This release candidate opens the `v6.7.0` candidate line after stable `v6.6.0`, which remains the rollback target.

Pulse v6.7.0 makes backup status clearer when reads fail, dates are uncertain or tasks disagree. It strengthens configuration recovery, restores TrueNAS CORE readings, reduces chart processing and lets external agents read fleet changes and metric history.

## Highlights

- Proxmox Backups now counts completed restore points separately from running and failed entries.
- TrueNAS CORE monitoring now loads CPU, memory and History readings previously blocked by rejected reporting requests.
- Settings Recovery now rejects incomplete configuration backups before they can replace existing settings.

## What's improved

- **Charts need less processing.** Workload history uses less decoding, copying and temporary memory. Proxmox live updates also create fewer temporary objects.
- **History stays steady while browsing.** Drawer charts reserve room for changing values and times. Peak labels stay beside plotted spikes and identify peaks above the plotted average.
- **Expanded lists scroll correctly.** Platform and backup tables account for expanded details, keeping lower rows reachable without unexpected gaps or jumps.
- **Backup browsing survives refreshes.** Backups updates readings and badge colours while preserving filters, expanded details, scroll position and chart focus. Keyboard focus follows the inspected point through sorting, date regrouping and failed reads.
- **Rows and details stay readable.** Machines, Workloads, Infrastructure, thresholds and platform tables keep status cues and actions compact. Detail tables fit their drawer or split pane.
- **Controls remain reachable.** Settings actions stay visible, deep drawers scroll properly, and Infrastructure and Patrol actions have clearer accessible names. Agent Doctor commands support keyboard scrolling.
- **Sorting works from the keyboard.** Platform table headings have focusable sort buttons that respond to Enter and Space. Backup summary buttons no longer toggle twice with Space.
- **SSO addresses stay readable.** Settings keeps complete callback, metadata and ACS URLs beside their copy controls within the panel.
- **Dark labels have clearer contrast.** Muted text is easier to read in dark mode.
- **What's New is easier to scan.** After an update, changes appear as plain lists with warnings first, without hidden groups.
- **Connection failures preserve saved access.** Startup keeps credentials and workspace selection when discovery fails. Sign-in offers a read-only retry, and public Help stays reachable during session-check failures.
- **Polling logs are quieter and more private.** Routine PBS datastore readings move to debug logging. Failed-response diagnostics record response size instead of copying provider contents into logs.

## Alerts and notifications

- **Alert History avoids repeated names.** Rows show the condition beside the resource without naming it again. Compact actions leave more room for descriptions.
- **Deleted destinations stay deleted.** Webhook removal confirms success in the editor. Queued deliveries cannot borrow another destination's permission because it shares the same URL.
- **Audit webhook drafts survive errors.** Rejected entries remain available for correction, and a completed save does not erase a newer draft. Unread destination settings block replacement until they can be loaded.
- **Delivery activity supports keyboard browsing.** The notification log can be focused and scrolled, and refreshes preserve keyboard position.
- **PBS alert timing restarts after gaps.** Missing CPU or memory readings interrupt activation and recovery timing without clearing existing warnings.
- **Rolling alerts use the right history.** Averaged alerts find their own stored readings for host agents, Docker hosts and containers, and Proxmox LXCs, including resources combined with another monitoring source.
- **Docker alert controls show their scope.** Thresholds removes ineffective runtime switches. Help explains that Offline Alerts covers runtime connectivity, while containers have separate settings.
- **Recovery rules are clearer.** Troubleshooting explains why falling usage can leave an alert open, including clear thresholds, recovery delays, averaged readings and missing observations.

## Disks and storage

- **Real container data disks remain visible.** Filesystem monitoring includes real mounts under /var/lib/docker and /var/lib/containers while excluding container layers. Help explains missing mounts without asking users to move data.
- **Disk totals avoid counting aliases twice.** Workloads summaries use the supplied aggregate capacity when available. Individual mount readings and warning colours remain inspectable.
- **Disk cards show the correct percentage.** Resource disk summaries no longer interpret a usage ratio as an already calculated percentage.
- **Missing disk readings stay unknown.** Host charts and History no longer record unavailable filesystem usage as zero or substitute an unmeasured live value.
- **Disk warnings need recovery evidence.** Missing agent filesystems or unavailable Proxmox readings cannot clear alerts or advance confirmation timing. Linked hosts retain the established alert owner to prevent duplicates.
- **Storage warnings require usable capacity.** Usage thresholds and forecasts ignore missing or contradictory capacity readings. Explicitly disabling a warning still takes effect.
- **macOS excludes hidden mounts.** Time Machine and other hidden system mounts no longer appear as ordinary disks by default. Explicit disk includes remain available.
- **Local backup copies stay independent.** Identical paths on separate PVE nodes remain separate copies. Failed reads, disabled storage and running files stay scoped to the correct local disk.
- **Archives and snapshots protect the right guest.** PVE archive paths identify VM and LXC backups. Reused VMIDs, conflicting file IDs, retained snapshots and unknown archive types cannot give another guest protection credit.
- **Backup locations keep their boundaries.** Similar repositories on different connections stay separate. Restore evidence names its reading connection, while notices explain that location filters do not recalculate workload-wide protection.
- **Guest SMART failures stay visible.** A failed scoped read hides cached SMART values instead of presenting them as current. Closing and reopening the drawer preserves the error and recovery path.
- **Linux temperatures exclude other sensor units.** Fan RPM, voltage and power no longer become Celsius readings. Help shows where existing extra temperatures and fans appear in Machine or Host details, without promising per-sensor alerts.

## Proxmox, PBS and backups

- **Incomplete reads preserve inventory.** Failed, partial or malformed PVE and PBS listings cannot erase retained entries. PBS checks datastore, namespace, group, snapshot and file identities before reconciling history.
- **Access refusals remain meaningful.** Backup collection stops continuity reads after explicit permission refusals. A readable sibling namespace cannot establish that an unread namespace is empty, and access warnings persist until enumeration recovers.
- **Old evidence cannot become fresh again.** Retained PVE snapshots and cached PBS backups from failed reads no longer renew protection evidence. Accepted listings survive later timeouts, throttling and optional size-read failures.
- **Failed assessments withdraw outdated claims.** Coverage rejects failed, malformed, duplicate or inconsistent results and offers Retry. Large fleets use complete batches with limited parallel requests, cancelled when leaving the view.
- **Uncertain evidence remains inspectable.** Coverage keeps restore details when assessments fail and distinguishes partial history from no backups. Provider reads, job results and restore-point states have separate labels explaining their limits.
- **Backup identities and dates are visible.** Coverage and By date show native restore-point IDs, locations and reported timestamps without hovering. Coverage also shows assessment, latest-success and verification times.
- **Uncertain dates cannot imply protection.** Ages, completion labels and verification summaries reject malformed or ambiguous dates. Future dates stay uncertain and identify clocks to check, while recognised older guest IDs still match Coverage.
- **Backup activity follows the evidence.** Completed jobs clear running indicators. Missing guest results stay Unknown, conflicting task results cannot complete archives, and unread writer lists cannot declare backups failed.
- **Activity follows inventory filters.** Search, node, type, source, location and verification filters narrow the chart. Day selection narrows only the list, and calendar dates remain correct across clock changes.
- **Unknown sizes and retention stay distinct.** Charts mark completed points with unknown sizes and show known volume, while restore details distinguish unknown size from zero. Prune retention stays separate from verification, including Not reported.
- **Task failures have inspectable details.** Coverage shows outcomes, errors, IDs and original task location, including migrated guests. Missing durations stay unknown, uncertain ordering keeps possible latest tasks visible, and Show more limits long lists.
- **Cluster trust is editable per endpoint.** Settings lets users inspect, test and save certificate trust for existing PVE polling addresses. Other members' trust and disabled collection settings are preserved, and tests identify the address checked.

## TrueNAS, vSphere and Docker

- **CORE memory separates Free and ARC correctly.** Compatible TrueNAS CORE 13 systems supply both values from one authenticated native reading instead of combining differently timed measurements.
- **Memory composition is accessible.** The TrueNAS Overview memory bar opens a breakdown with the keyboard or a click. Help explains Used, reclaimable ARC cache and Free.
- **CORE disk throughput reaches History.** TrueNAS Storage retains physical-disk read and write rates through charts and stored history. Help distinguishes individual disk activity from host totals and capacity.
- **History finds the correct source.** TrueNAS and vSphere drawers preserve the provider's history identity when loading through the resource API.
- **Protection dates reflect completed work.** Collection time no longer becomes snapshot creation time, including older records. Start times, running replication and malformed dates cannot appear as completed work.
- **Undated replication problems stay visible.** Failed, warning and running states remain visible without invented dates. A reported success without a known run time stays Unknown.
- **Unread protection history stays distinct from empty.** TrueNAS Protection rejects malformed or duplicate records, warns when showing retained events and offers Refresh. Help explains snapshot and replication limits.
- **Docker updates wait for health checks.** Pulse waits up to three minutes for replacement containers' initial checks and detects restarting containers. Verification stays inconclusive while health is still starting.
- **Image checks follow local replacements.** Docker refreshes cached tag information when a new local image needs comparison, while respecting registry backoff. Manual checks discard older in-flight results rather than restoring stale cache entries.
- **Image-check failures open their details.** Docker Images makes Check failed a keyboard-accessible button, so errors can be read without hovering.
- **Private registry setup is clearer.** Docker Help explains HTTPS requirements, agent-local login files and credential helpers, container mounts and login caching. It distinguishes pulls, anonymous checks and cached update results.
- **Docker credentials get safer setup guidance.** Authentication-file examples refuse symlinks and non-regular files, preserve existing contents and restore private permissions before launch.

## Install, updates and agents

- **Docker setup preserves existing deployments.** The helper stops when Compose configuration already exists, protecting custom ports, mounts, image pins and credentials.
- **Docker images avoid repeated large files.** Server packaging no longer stores another copy of binaries just to change permissions. Help distinguishes image layers from retained Pulse data without asking users to delete data.
- **Installation success requires startup.** The server installer stops on directory-setup failures and confirms startup before reporting success. Help distinguishes prepared configuration, a running service and a usable dashboard.
- **Agent installer files arrive together.** Server installation publishes the served agent scripts as a complete set, preserving recovery files if publication fails.
- **Installer snapshots stay private and separate.** Configuration copies use private permissions. Rotation preserves unusual path characters, stays within the selected instance and requires ownership evidence before removing older copies.
- **Unix setup handles existing permissions.** Agent commands recognise passwordless sudo, and non-root preflight handles root-owned installer directories. Moving to the safe profile preserves the hostname already reported.
- **Agent updates match the executable version.** Checks use the installed binary's version format. Lifecycle discovery also recovers when a peer stream is cancelled.
- **Server updates finish reliably.** Helper replacement avoids truncating running scripts, and preparation failures preserve helpers. Completion avoids an unsupported API call and checks that the installed executable still matches the running service.
- **Installer downloads are bounded.** Saved installer and manual-update downloads use HTTPS through redirects and connection and download deadlines. Help explains verification before executing downloaded installers.
- **NAS upgrades preserve other startup jobs.** Unraid and QNAP updates retain unrelated boot commands and stop on unrecognised arrangements. Unraid removal refuses to affect an unrelated live boot shell.
- **Proxmox installation leaves existing guests alone.** Creating Pulse no longer performs legacy mount cleanup on other LXCs. Failed or interrupted installs retain the new container for inspection and recovery.
- **Removal failures preserve recovery data.** Windows keeps files and credentials if unregistration fails, even if its service is gone, and unattended errors exit without a dialogue. Server removal retains configuration until earlier steps succeed.

## Pulse Pro, AI and hosted

- **Patrol follows the selected resource.** Alerts and findings, including demo findings, use consistent identities and group only matching problems. Open work counts, filter badges, approvals and summaries match the resource being browsed.
- **Active Docker problems stay open.** Patrol skips paid resolution reviews when current container evidence still shows the fault. Missing condition readings or healthy unrelated metrics cannot imply recovery.
- **External agents can read recent changes.** Community API and MCP tool list_recent_changes expose fleet changes with time, resource, kind and source filters. Results indicate when more changes matched than were returned.
- **External agents can read metric history.** API and MCP tool get_metric_history return recorded resource metrics under the same limits as Pulse charts, including supported Kubernetes resources.
- **Agent event streams announce more changes.** API and MCP subscribers receive alerts firing or resolving and resource status changes. Rapid status changes are grouped, with individual transitions available in history.
- **External-agent activity is counted separately.** Pulse browsing and demo reads do not count as external use. Telemetry counts days with successful agent history reads without including queries, resource IDs or results.
- **Workspace agent enrolment works consistently.** Install tokens live where agents authenticate. Registration, diagnostics, command permissions and revocation use the same organisation-bound records.
- **Workspace recovery stays isolated.** Imports reload only the selected organisation, preserve instance-wide settings and avoid copying another workspace's connections or credentials.
- **Unread investigations offer a safe retry.** Patrol distinguishes failed history and message reads from an absent investigation, labels retained results and blocks another run until history is readable. Retry can recover failed startup reads.
- **Licence recovery reports beside its controls.** Manual recovery keeps progress and errors visible. Help distinguishes activating a licence from recovering the private runtime.
- **AI provider Help explains where data goes.** Privacy guidance describes provider processing, local endpoints and retained context, and explains that turning off usage telemetry does not stop AI requests.
- **MSP billing respects cancellation.** Plans points to Manage billing for renewal or end dates instead of promising renewal. Help explains self-service purchasing, upgrades and applying a purchase.

## Monitoring and service health

- **Restarted servers show agents awaiting reports.** Machines, Infrastructure and resource drawers distinguish saved enrolment from fresh telemetry until the first valid report arrives.
- **Silent agents stop supplying live values.** Expired reporting stops cached readings from appearing current in charts. Stored history remains available, and open drawers refresh when reporting stops or resumes.
- **Missing backup times stay cautionary.** Workloads says No completed backup reported rather than implying a guest has never been backed up. A running backup is no longer described as its first.
- **Incomplete guest status is rejected.** Proxmox responses missing usable status or identifying the wrong guest cannot supply readings or action checks. Healthy powered-state observations reset diagnostic alert runs.
- **Cluster startup and links are more reliable.** Duplicate clusters consolidate before monitoring and agent registration. Discovered members open their reported Proxmox address when a short hostname would not work in the browser.
- **Backup polling stops and resumes cleanly.** Expired or stopped scans cancel waiting work. Replacing a connection starts its own scan, and accepted progress survives later log timeouts.
- **Busy backup jobs share polling time.** PVE rotates unfinished log reads so live jobs stay current without starving ended jobs. Malformed task logs cannot invent guest backup results.
- **Replication failures stay visible.** Proxmox Replication distinguishes unread inventory from no jobs and offers recovery. Missing or malformed sync times stay Unknown instead of implying a successful run.
- **PBS readings preserve uncertainty.** Datastore counts stay tied to its server and datastore and distinguish unread from zero. Capacity percentages follow bytes, while memory preserves valid percentages and marks missing readings unavailable or stale.
- **Mail Gateway reports useful partial results.** Queue totals use Postfix summaries, and delay uses the oldest age band excluding held mail. Optional failures show Degraded without throttling working reads, while parsing errors stay distinct from outages.
- **Change history recognises restarts.** Uptime resets record guest, node and host restarts. Restarting Pulse avoids rediscovering the fleet and records changes after downtime without inventing their times.
- **Optional Proxmox checks back off after refusals.** Permission-denied update and guest-memory reads stop retrying every poll. Retained update counts stay stale, and a successful zero count clears the displayed old value.

## Security

- **CSV text stays text in spreadsheets.** Reports, VM inventory and audit exports prevent supplied text from becoming formulas. Audit checks and JSON exports retain original event data.
- **Tokens use current permissions.** Requests recheck live workspace tokens after configuration changes or revocation. Dashboard and agent event streams end when authority is withdrawn, and invalid explicit tokens cannot fall through to browser sessions.
- **Agent access requires appropriate permissions.** Non-admin browser and proxy users cannot submit telemetry, acknowledge Docker commands or read agent configuration containing probe credentials.
- **Monitoring reads protect execution details.** Recent changes and agent action reads hide command text, output and runbook messages unless the caller also has execution permission.
- **Oversized security inputs are bounded.** Pulse limits decompressed WebSocket messages before buffering them. Updated OIDC key and token verification libraries address security flaws and limit oversized RSA key input.
- **Administrative selections must be unambiguous.** SSO, access recovery, role checks and audit requests reject conflicting fields or malformed selections. Removing all audit destinations requires an explicit empty list.
- **Audit exports have a clear limit.** Exports and summaries reject selections above 10,000 events instead of returning partial results. Narrow the time range, event type or user.
- **SSO edits preserve saved settings and live sign-ins.** Ordinary edits retain advanced options and secrets, while environment-managed providers keep their authority. Name, icon and ordering changes preserve sign-ins and logouts already in progress.
- **SSO tests describe their limits.** OIDC tests match the sign-in issuer, including trailing slashes, and SAML checks require an identity and usable SSO URL. Tests do not claim a completed sign-in, and provider labels retain complete characters.
- **SSO errors preserve useful drafts.** Failed saves give readable guidance without automatic write retries. OIDC tests and SAML previews keep private responses out of errors, and preview failures restore focus.
- **Local-login Help explains the limit.** Hiding the password form does not disable local password authentication or enforce SSO-only access. Guidance identifies the separate administrator recovery path.
- **Legacy cleanup preserves recovery access.** Cleanup requires complete inventory, protects shared SSH key targets and retains recovery keys when local removal cannot finish.

## Other improvements

- **Recovery saves related data together.** Settings imports commit guest metadata with settings and credentials. Failed writes restore previous files and preserve older recovery backups.
- **Repeated imports cannot overwrite newer edits.** Concurrent imports are blocked per workspace, and recently submitted backups cannot be applied again for ten minutes.
- **Transfers stay with the confirmed workspace.** Recovery dialogs show the workspace ID and stop if selection changes. Feedback stays beside the inputs, and saving a token does not repeat the transfer.
- **Saved and applied are distinguished.** Import errors identify components that did not reload. Uncertain responses warn that changes may already have applied and advise inspecting the destination before repeating the import.
- **Backup passphrases are independent.** Recovery exports ask users to choose and confirm a backup passphrase of at least 12 characters, rather than suggesting their sign-in password.
- **Oversized imports stop before upload.** Recovery checks file and request sizes before reading or sending oversized backups, with an error beside the transfer controls.
- **Recovery Help explains backup limits.** Guides distinguish configuration exports and installer snapshots from full data backups. Migration guidance covers isolated trials, retained storage and handing monitoring over to one server.
- **Access settings distinguish failed reads from empty lists.** API Tokens offers Retry when inventory fails. User assignments and organisation access stay tied to the current user, workspace and completed read, blocking edits based on stale permissions.
- **Unavailable inventory fails clearly.** Manual and scheduled VM inventory exports report unavailable source data instead of producing a successful empty download.
- **Setup Help explains access choices.** Agent examples keep tokens in private files. Guides separate Linux privilege profiles, observer telemetry and Proxmox access, optional guest file reads and filesystem auditing, and container SSH setup restrictions.
- **Troubleshooting covers the right checks.** Help explains backup disagreements, update failures and stalled dashboards behind proxies. The overnight guide demonstrates agent history tools, the index lists shipped guides, and missing assets return Not Found.
- **Private reporting guidance is easier to find.** Security Help explains private vulnerability reporting and credential recovery. Configuration Help identifies findings, conversations, tool results and operator notes as private data.

## Known issues

- **Guest freezes and Windows restarts are not fully resolved.** If monitoring was stopped after a guest froze, do not restart it solely to test this beta or this release. Related fixes do not establish recovery for every affected guest.
- **Affected backups still need precautions.** Stop Pulse server monitoring before a freeze-enabled backup. Keep it stopped until independent checks confirm thaw, fresh writes on every backed-up filesystem and workload liveness, regardless of an OK task.
- **Kubernetes pauses need controller and storage checks.** Confirm retained server storage before stopping pods, and prevent controllers from replacing them. Deleting a pod or stopping only the collector does not confirm server polling has stopped.
- **Windows disk readings can still be unavailable.** Use the existing working Machines disk view. Do not remove or reinstall agents, restart guests or force guest-agent checks just to recover Proxmox disk readings.
- **Elevated server memory remains unresolved.** Chart improvements do not establish a lower baseline. Keep the working setup and report elevated steady usage or continued growth using existing readings at comparable uptimes and workloads.
- **Previously affected hosts need caution.** If agents or Patrol were disabled after a host lockup, keep that setup. Preserve existing evidence privately, without re-enabling them, changing GPU load or altering hardware just to recreate the incident.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.6.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.6.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.6.0` and recreate the container.
