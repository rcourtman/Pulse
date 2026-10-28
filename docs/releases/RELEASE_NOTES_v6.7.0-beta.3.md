# Pulse v6.7.0-beta.3 Release Notes

This beta follows `v6.7.0-beta.2` on the way to stable `v6.7.0`. Stable `v6.6.0` remains the rollback target.

Pulse v6.7.0 makes backup status clearer when reads fail, dates are uncertain or tasks disagree. It also strengthens configuration recovery, reduces chart processing, restores TrueNAS CORE readings, and improves disk visibility, agent updates and SSO settings.

## Highlights

- Proxmox Backups now counts completed restore points separately from running and failed entries.
- TrueNAS CORE monitoring now loads CPU and memory readings previously blocked by rejected reporting requests.
- Settings Recovery now rejects incomplete configuration backups before they can replace existing settings.

## What's improved

- **Charts use less server work.** Workload history needs less decoding, copying and temporary memory. Proxmox live updates also create fewer temporary objects.
- **History stays still while browsing.** Drawer charts reserve space for changing values and times, keeping controls steady under the pointer. Peak labels stay beside the plotted spike.
- **Expanded lists scroll correctly.** Platform and backup tables account for expanded details, keeping lower rows reachable without unexpected gaps or jumps.
- **Backup browsing survives refreshes.** Open Backups views update inventory, readings and badge colours while preserving the selected view, filters, expanded details and chart focus.
- **Retry keeps keyboard focus.** Backups preserves a usable focus position through failed reads, retries and recovery, including when an error message disappears.
- **Tables work better with keyboards.** Platform headings have focusable sort buttons that respond to Enter and Space.
- **Rows and details stay readable.** Machines, workloads, Infrastructure, thresholds and platform tables keep status cues and actions compact. Detail tables adapt to their available panel width.
- **Controls remain reachable.** Settings actions stay visible, deep drawers scroll properly, and Infrastructure and Patrol actions have clearer accessible names. Agent Doctor commands support keyboard scrolling.
- **Dark text has clearer contrast.** Muted labels are easier to read in dark mode.
- **What's New is easier to scan.** After an update, changes appear as plain lists with warnings first, without hidden groups.

## Alerts and notifications

- **Alert History avoids repeated names.** Rows show the condition beside the resource rather than naming it again. Compact actions leave more room for descriptions.
- **Deleted destinations stay deleted.** Webhook removal confirms success in the editor. Queued deliveries cannot borrow another destination's permission because it shares the same URL.
- **Audit webhook drafts survive errors.** Rejected entries remain available for correction, and a completed save does not erase a newer draft.
- **Delivery activity supports keyboard browsing.** The notification log can be focused and scrolled, and refreshes preserve keyboard position.
- **PBS timing restarts after missing readings.** Gaps in CPU or memory data interrupt activation and recovery timing without clearing existing warnings.
- **Docker alert controls show their actual scope.** Thresholds removes ineffective runtime switches. Guidance explains that Offline Alerts covers runtime connectivity, while containers need their own settings.
- **Recovery rules are easier to understand.** Troubleshooting explains why falling usage can leave an alert open, including clear thresholds, recovery delays, averaged readings and missing observations.

## Disks and storage

- **Real container data disks remain visible.** Filesystem monitoring includes real mounts under /var/lib/docker and /var/lib/containers while continuing to exclude container layers.
- **Missing disk readings stay unknown.** Host charts and History no longer record unavailable filesystem usage as zero or substitute an unmeasured live value.
- **Disk warnings need recovery evidence.** Missing agent filesystems or unavailable Proxmox node readings cannot clear alerts or advance confirmation timing. Linked hosts retain the established alert owner to prevent duplicates.
- **Storage warnings require usable capacity.** Usage thresholds and forecasts ignore missing or contradictory capacity readings. Explicitly disabling the warning still takes effect.
- **macOS lists exclude hidden mounts.** Time Machine and other hidden system mounts no longer appear as ordinary disks by default. Explicit disk includes remain available.
- **Local backup copies stay independent.** Identical paths on separate PVE nodes remain separate copies. Reads, failures, disabled storage and running files stay scoped to the correct local disk.
- **Archives stay with the right guest type.** PVE uses archive paths rather than storage names to identify VM and LXC backups. Reused VMIDs and explicitly unknown archive types cannot give the wrong guest protection credit.
- **Missing-filesystem help avoids storage changes.** Guidance distinguishes real data disks from excluded layers and remote mounts, without asking users to move data or change backup selection.

## Proxmox, PBS and backups

- **Incomplete reads preserve inventory.** Failed or malformed PVE snapshot and storage listings cannot erase retained entries. PBS preserves unread namespaces when siblings succeed, while respecting explicit access refusals.
- **Old evidence cannot become fresh again.** Retained PVE snapshots and cached PBS backups from failed reads no longer renew protection evidence. Recognised older guest identities still match Coverage.
- **Failed assessments withdraw outdated claims.** Coverage rejects failed, malformed or inconsistent assessment responses and offers Retry. Large inventories use complete batches, and leaving the view cancels outstanding reads.
- **Uncertain evidence remains inspectable.** Coverage keeps restore details available when assessments fail. Partial history shows Attention where appropriate, provider scope is identified, and empty details distinguish current from retained evidence.
- **Dates must describe a real instant.** Backup ages, Workloads completion labels and assessments reject malformed or ambiguous dates. Future backup or verification dates remain uncertain and explain which clocks to check.
- **Ended jobs stop looking active.** Completed jobs clear running-backup indicators. Missing guest results stay Unknown, and unread cached logs cannot keep renewing activity.
- **Activity follows inventory filters.** Search, node, type, source, location and verification filters narrow the chart. Selecting a day narrows only the list, and calendar dates remain correct across clock changes.
- **Unknown backup sizes remain visible.** Volume charts mark days containing completed points with unavailable sizes. Bars show known volume, so missing sizes no longer look like zero activity.
- **Prune retention cannot hide verification.** Backups kept during pruning still show Verified or Unverified. Retention appears separately, and unavailable verification is labelled Not reported.
- **Task failures have inspectable details.** Expanded Coverage rows show status, start time, duration, error, task ID and reported origin. Missing, future or tied start times expose possible latest tasks instead of choosing an OK result.
- **Backup sources and guest placement stay distinct.** Restore evidence names its reading connection and supports searching it. Workloads retain their owning PVE identity, while repository names cannot become guest node names.
- **Location filters keep their boundaries.** Similar repositories on different connections stay separate. Ambiguous saved filters request a new selection. Notices explain that filtered ages and evidence do not recalculate workload-wide protection.

## TrueNAS, vSphere and Docker

- **History finds the correct source.** TrueNAS and vSphere resource drawers preserve the provider's history identity when loading through the resource API.
- **TrueNAS dates reflect completed work.** Collection time no longer becomes snapshot creation time, including older stored records. Start times and running replication timestamps cannot appear as completed work.
- **Undated replication problems stay visible.** Failed, warning and running states remain visible without invented dates. A reported success without a known run time stays Unknown.
- **Failed protection refreshes are visible.** TrueNAS Protection warns when displaying retained events and offers Refresh. Its guide explains snapshot, replication and collection limits.
- **Docker updates wait for health checks.** Pulse waits up to three minutes for replacement containers' initial checks and detects restarting containers. Verification remains inconclusive while health is still starting.

## Install, updates and agents

- **Docker setup preserves existing deployments.** The helper stops when a Compose configuration already exists, protecting custom ports, mounts, image pins and credentials. It distinguishes prepared configuration from a running Pulse server.
- **Unix installation handles existing permissions.** Generated commands recognise passwordless sudo. Non-root preflight no longer fails merely because the saved installer directory requires root access.
- **Agent names survive profile migration.** Moving to the safe profile preserves the hostname already reported instead of replacing it with a fully qualified name.
- **Updates finish more reliably.** The agent version endpoint matches the executable's version format. Successful automatic server updates finish without waiting on an unsupported notification request.
- **Manual update helpers remain usable.** Replacement avoids truncating a running script, and preparation failures preserve existing helpers. Bootstrap downloads have deadlines and require HTTPS through redirects.
- **Installer guidance keeps tokens private.** Examples use a private token file and a downloaded installer users can inspect, with clearer privilege and cleanup instructions.
- **NAS upgrades preserve other startup jobs.** Unraid and QNAP updates replace recognised Pulse entries while retaining unrelated boot commands. Unrecognised arrangements stop with recovery files retained.
- **Unraid removal protects active boot scripts.** Agent uninstall refuses to proceed while an unrelated live boot shell could be affected.
- **Legacy cleanup preserves recovery access.** Cleanup requires complete inventory, preserves shared SSH key targets, and retains recovery keys when local cleanup cannot finish.
- **New Proxmox installs leave existing guests alone.** Creating Pulse no longer performs legacy mount cleanup on other LXCs. Failed or interrupted installs retain the new container for inspection and recovery.
- **Failed removals preserve recovery data.** Server removal retains configuration until earlier removal steps succeed. Windows agent removal keeps identity, credentials, logs and binaries until server unregistration is confirmed.
- **Update and removal failures are clearer.** Agent help distinguishes download, validation and activation failures before another attempt. Unattended Windows removal errors return without waiting for a dialogue.

## Pulse Pro, AI and hosted

- **Patrol opens the right resource.** Inbox alerts, findings and actions use consistent resource identities. Findings repeating an alert group under that specific alert, while separate problems retain their own decisions.
- **Resource filters cover the whole Patrol view.** Open work counts, filter badges, approvals and summaries match the resource being browsed.
- **Active Docker problems stay open.** Patrol skips resolution reviews when current container evidence still shows the fault. Missing condition readings or healthy unrelated metrics cannot imply recovery.
- **Demo findings match their resources.** Demo Patrol findings appear in the corresponding resource views and group with matching alerts.
- **Workspace agent enrolment works consistently.** Install tokens live where agents authenticate. Registration, diagnostics, command permissions and revocation use the same organisation-bound records.
- **Workspace recovery stays isolated.** Imports reload only the selected organisation, preserve instance-wide settings, and avoid copying another workspace's connections or credentials.
- **MSP billing respects cancellation.** The Plan page points to Manage billing for renewal or end dates. The guide explains self-service purchasing, upgrades and applying a purchase.

## Monitoring and service health

- **Restarted servers show agents awaiting reports.** Machines, Infrastructure and resource drawers distinguish saved enrolment from fresh telemetry until the first valid report arrives.
- **Silent agents stop supplying live values.** Expired reporting stops cached readings from appearing current in charts. Stored history remains available, and open drawers refresh when reporting stops or resumes.
- **Incomplete guest status is rejected.** Proxmox responses missing usable status or identifying the wrong guest cannot supply monitoring readings or action checks.
- **Cluster links open the right node.** Discovered members use their reported Proxmox address when a short hostname would be unsuitable for the browser.
- **Startup handles duplicate clusters earlier.** Consolidation finishes before monitoring becomes available, avoiding conflicts with agent registration.
- **Expired backup scans stop promptly.** Storage scans stop when their time budget expires or monitoring stops, instead of continuing retries across remaining nodes.
- **Mail Gateway queues show real totals.** Counts come from Postfix queue summaries. Delay estimates use the oldest reported age band and exclude intentionally held mail.
- **Mail Gateway read failures stay specific.** Failed optional reads no longer throttle the whole connection. Reachable gateways with missing readings show Degraded, and decoding failures remain distinct from network outages.
- **Mail Gateway help explains collection controls.** Guidance clarifies which saved switches stop reads and why disabling a failing read removes coverage rather than repairing it.

## Security

- **CSV text stays text in spreadsheets.** Reports, VM inventory and audit exports protect supplied names and other text from becoming formulas. Audit checks and JSON exports continue using the original event data.
- **Workspace tokens use current permissions.** Requests recheck the live organisation token store after configuration changes or revocation. An invalid explicit token cannot fall through to browser-session access.
- **Agent access requires appropriate permissions.** Non-admin browser and proxy users cannot submit telemetry, acknowledge Docker commands or read agent configuration containing probe credentials. Scoped agent tokens retain their intended access.
- **Compressed messages have a size limit.** Pulse bounds decompressed incoming WebSocket messages before buffering them, reducing exposure to oversized compressed input.
- **Administrative requests must be unambiguous.** SSO, access recovery and audit requests reject conflicting fields or malformed selections. Removing all audit destinations requires an explicit empty list.
- **Audit exports have a clear limit.** Exports and summaries reject selections above 10,000 events instead of returning partial results. Narrow the time range, event type or user.
- **SSO edits preserve protected settings.** Ordinary provider edits retain advanced options the editor cannot display. Environment-managed providers cannot be replaced, removed or taken over through saved settings.
- **SSO tests describe their limits.** OIDC requires the same issuer match as sign-in, including trailing slashes. SAML checks require an identity and usable SSO URL. Neither test claims to have completed sign-in.
- **SSO errors preserve useful drafts.** Failed saves give readable guidance without automatic write retries. OIDC tests and SAML previews keep private responses out of errors, and preview failures restore focus.
- **Local-login help explains the limit.** Hiding the password form does not disable local password authentication or enforce SSO-only access. Guidance clarifies the separate administrator recovery path.
- **Cluster certificate trust is editable per endpoint.** Settings lets users inspect, test and save trust for existing PVE polling endpoints while preserving other members' trust and disabled collection settings.
- **OIDC verification is hardened.** Updated key and token verification dependencies address security flaws and bound oversized RSA key input.

## Other improvements

- **Configuration recovery saves related data together.** Guest metadata commits with settings and credentials. Failed writes restore previous files and preserve older recovery backups instead of leaving a partial replacement.
- **Repeated imports cannot overwrite newer edits.** Concurrent imports are blocked per workspace, and recently submitted backups are temporarily blocked from being applied again.
- **Transfers stay with the confirmed workspace.** Export and import dialogs show the workspace ID and stop if selection changes. Feedback stays beside the inputs, and saving a token does not repeat the transfer.
- **Saved and applied are distinguished.** Import failures identify components that did not reload. Uncertain responses warn that changes may already have applied and advise inspecting the destination before repeating the import.
- **Recovery explains its limits before transfer.** Settings identifies configuration exports as partial backups. Migration guidance covers excluded data, access requirements, isolated trials, retained storage and monitoring handover.
- **Backup help explains conflicting readings.** Guidance separates inventory entries, completed points, daily volume, task outcomes and guest placement, helping users compare existing records without starting another backup.
- **Unavailable inventory fails clearly.** Manual and scheduled VM inventory exports report unavailable source data instead of producing a successful empty download.
- **Missing web assets return proper errors.** Missing scripts, styles and documentation return Not Found instead of a successful response containing the application page.
- **Update guidance distinguishes policy from availability.** Help explains that an approved stable-patch policy or a merged fix does not mean an update has been published.

## Known issues

- **Guest safety remains unresolved.** These changes do not confirm a fix for every guest freeze or Windows restart. If monitoring was stopped after a guest froze, do not restart it solely to test this beta or this release.
- **Affected backups still need precautions.** Stop Pulse monitoring before a freeze-enabled backup. Keep it stopped until independent checks confirm thaw, fresh writes on every backed-up filesystem and workload liveness. An OK task does not prove thaw.
- **Kubernetes pauses need controller and storage checks.** Confirm retained server storage before stopping pods, and prevent controllers from replacing them. Deleting a pod or stopping only the collector does not confirm server polling has stopped.
- **Elevated server memory remains unresolved.** Reduced chart overhead does not establish a lower long-running baseline. Keep the working setup and report elevated steady usage or continued growth from existing readings at comparable uptimes and workloads.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.6.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.6.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.6.0` and recreate the container.
