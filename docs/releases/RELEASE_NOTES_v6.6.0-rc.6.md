# Pulse v6.6.0-rc.6 Release Notes

This release candidate follows `v6.6.0-rc.5` on the way to stable `v6.6.0`. Stable `v6.5.0` remains the rollback target.

Pulse v6.6.0 puts alerts, charts and useful next steps together so you can see what needs attention across your homelab. Backup coverage and hardware health explain incomplete or older readings more clearly. Cluster agent installation, scheduled reports and notification delivery need less manual work.

## Highlights

- See problems, recent trends and longer history together.
- Understand backup coverage and distinguish current readings from last-known data.
- Install cluster agents and schedule routine reports with fewer manual steps.

## What's improved

- **Charts sit beside the problem.** Resource Overview combines alerts, recent trends and longer history across guests, hosts and containers. The separate History tab is removed.
- **Brief spikes stay visible.** Charts show minimum and maximum readings around average lines, helping distinguish short peaks from sustained load.
- **History is easier to interpret.** Consistent ranges, clearer axes, dated values and source labels explain readings. Inspected values survive resizing, missing readings leave gaps, and limited histories retain the newest observations.
- **Charts show relevant alerts.** Resource history includes its own alerts and trigger lines without unrelated parent alerts. Expanded row history also charts network and disk I/O.
- **Details lead with useful information.** Consistent detail panels put warnings and failed availability checks first, with inventory under Details. Complete identifiers and clearer headings replace repeated or cut-off values.
- **Web links can be edited together.** Bulk-edit saved interfaces for workloads and Docker containers, then open a guest's saved interface from its header.
- **Guests keep their identity when moved.** Proxmox migrations preserve metadata, links, selection and history. Matching names or guest IDs at separate installations stay distinct.
- **Manual grouping stays yours.** Links, chained links and splits survive refreshes and identity changes. Unlinking separates rows, while reporting a bad merge removes only the responsible links.
- **Large dashboards do less repeated work.** Inventory updates reduce copying, grouping and sorting. Tables fetch visible history, hidden charts pause collection, chart queries do less decoding and disk work, and database and alert-history caches have limits.
- **VM inventory exports can run on a schedule.** Export the organization's current VM inventory as CSV weekly or monthly, including newly discovered VMs. Email it or save it to disk.
- **Client backup health gets a report.** Compatible Pro runtimes offer weekly or monthly PDF and CSV summaries of Proxmox backup attempts, including jobs writing to PBS. Failures, missing attempts and incomplete history remain explicit.
- **Report delivery is easier to recover.** Settings shows readiness and results. Temporary email failures retry within limits, retries preserve the report period, and uncertain sends warn about duplicates. Schedules survive monitor reloads.

## Alerts and notifications

- **Alerts open the right place.** Overview prioritises active problems. Guest and filesystem alerts open their guest, storage alerts open their pool, and notifications open the alert's timeline when a public URL is configured.
- **Open alerts explain their state.** Threshold alerts show the current reading, averaging window, clear level and recovery progress while preserving the original breach value and date.
- **Missing readings interrupt confirmation.** Metric, storage, connection and disk-temperature timers restart after observation gaps instead of treating separated readings as continuous trouble.
- **Off stays off.** Disabled Docker, Mail Gateway and backup defaults remain disabled after saving. Low thresholds keep their clear level below the trigger, and fresh installations receive Docker defaults.
- **Settings preserve your edits.** Omitted settings and node overrides survive other saves. Drafts wait for confirmed saves, editors wait for saved policy, and threshold controls have accessible names.
- **Agent handovers preserve coverage.** Linked agents avoid duplicate node alerts without losing temperature monitoring. Proxmox cleanup preserves alerts belonging to other platforms.
- **History explains closures.** Moved alerts stay distinct from recovered alerts in timelines, reports and notifications. Host, container, service, disk, array and sensor history stays with its resource.
- **History stays readable and responsive.** Messages wrap, frequency bars show severity and local times, and acknowledgements sync between tabs. Read errors remain visible, caches have limits, and retained failures explain when retries stopped.
- **Delivery controls explain their effect.** Destinations come first, schedules explain timing, and pause messages distinguish detection from delivery. Notification drafts survive tab changes, uncertain saves and masked-secret updates.
- **Webhooks send what you configured.** Required template headers reach real alerts, including Telegram. Clearing optional headers restores defaults, examples escape alert text, and invalid templates produce configuration errors.
- **Email works with more SMTP servers.** Provider defaults, LOGIN prompts and port 465 work consistently, while concurrent sends keep recipients separate. Subjects identify hosts and support non-English names, and rate limits survive switching email off.
- **Delivery failures get clearer handling.** Notifications try another approved address when one stalls, cancelled DNS work stops, and interrupted replies cannot count as completed sends. Temporary disconnects remain eligible for retry.

## Disks and storage

- **Capacity problems lead the storage view.** Tables emphasise filling pools and explain attention states. Collapsing groups no longer pins them, reopens them or scrolls back to an earlier pool.
- **Storage history stays close.** Expanded pools and disks include collapsible history charts with minimum-to-maximum bands.
- **Unrelated disks stay separate.** Matching serials on different hosts and generic QEMU serials no longer merge different hardware. Agent disks stay out of unrelated cluster-member inventory.
- **Hardware matching handles different ID formats.** Equivalent WWNs from Proxmox and agents match correctly, including controller disks. Empty or placeholder identifiers cannot establish a match.
- **Replacement disks start clean.** New hardware no longer inherits the old disk's SMART readings, slot identity, alert history or operator settings when its identity differs.
- **Retained readings show their age.** Standby disks and silent agents show last-known temperatures, usage and RAID state. Merged disks preserve each source's collection state.
- **Heat limits agree across views.** Alerts, colours, reports and Patrol use disk-type limits and host overrides. TrueNAS disk limits are editable, and Unraid uses its agent's reported temperature.
- **Heat alerts close for the right reason.** Removing a disk or disabling its temperature threshold clears its alert. Missing reports preserve unresolved SMART problems without presenting old temperatures as current.
- **Risk labels explain the problem.** Heat alone reads Running Hot rather than Replace Now. SSD wear is labelled life remaining, and disk alerts are logged once. Help explains Unraid empty slots and RAID spares without changing the array.
- **History follows the hardware.** Controller-member and Unraid serial-based charts resolve correctly. Same-serial disks on different hosts retain separate charts.
- **Real local mounts remain visible.** Mounts such as /snapshots and /runtime are no longer excluded merely because their paths resemble filtered system directories.

## Proxmox, PBS and backups

- **Guest memory explains its limits.** Views and alerts distinguish current, retained and cache-inclusive VM readings. Percentages use sampled capacity, including ballooned VMs. LXC listing memory is treated as cache-adjusted usage.
- **Windows readings can recover.** Guest reads resume after cooldowns and restarts. Pulse learns the OS before optional Linux memory requests and collects filesystems first, preventing unsuitable requests from blocking disk readings.
- **Filesystem totals avoid false duplicates.** Shared filesystems count once regardless of mount order. Distinct Windows volumes remain separate even when equally sized, and invalid counters remain unknown.
- **Partial filesystem reads remain useful.** Valid rows survive incomplete responses without implying complete capacity or recovery. Stopped guests no longer publish retained filesystems as current measurements.
- **LXC collection works within the helper sandbox.** Running containers and mounted filesystems can be read without Proxmox control sockets. Unmounted paths cannot borrow parent usage, and nodes without containers report an empty inventory.
- **Guest commands respect backup safeguards.** Reads and actions defer for locks, uncertain completion and malformed replies. Equivalent endpoints share coordination, and cancellation cannot prematurely release another command.
- **Incomplete polls preserve inventory.** Failed node enumeration retains known guests without refreshing their readings. Completed inventory survives optional-detail deadlines, and retired connections cannot republish old results.
- **Partial backup scans preserve history.** PVE and PBS retain known backups without making them look freshly checked. Failed namespace discovery stays incomplete, root PBS coverage includes LXC guests, and restored alerts are checked before startup delivery.
- **Backup removal is reflected correctly.** PBS refreshes inventory when pruning removes the newest backup, even if the count stays the same. Failed guest snapshot listings retain known snapshots, including LXC snapshots.
- **Unknown dates cannot imply recovery.** Missing or future backup dates preserve established backup-age warnings. Snapshot warnings also survive unknown creation dates, while valid recent backups and confirmed removals still clear the relevant warnings.
- **Coverage explains its evidence.** Health counts filter workloads, details show completion times, and missing access stays distinct from no backups. Amber shields link to their Recovery age limit, saved limits apply immediately, and the PBS link returns.
- **Replication shows observed results.** Status comes from source-node job observations. Slow reads do not pile up, failed reads preserve jobs, and saved configuration alone no longer implies successful replication.

## TrueNAS, vSphere and Docker

- **TrueNAS CORE keeps usable inventory.** Unknown or string-valued disk rotation rates no longer abort collection. Pool capacity can use top-level vdev statistics when flat fields are absent, while missing or conflicting readings remain unknown.
- **TrueNAS readings recover.** Native disk-temperature history works again, with a limited fallback for empty reporting results. Fractional SCALE app CPU readings keep reporting connected, and native aggregate CPU readings include idle zero.
- **TrueNAS explains unhealthy states.** Pools and topology give concrete reasons for attention. Services and VMs distinguish intentionally stopped items from those expected to run, with status text matching its indicator.
- **vSphere leads with health.** The overview highlights vCenter signals and explains datastore and network problems. Host groups use cluster names, events use vCenter descriptions, and VM records retain the correct platform.
- **Slower vCenter refreshes can get more time.** PULSE_VMWARE_POLL_TIMEOUT sets the whole inventory refresh limit, with a 20-second default. Failed refreshes retain known inventory and its original last-success time.
- **Container health is consistent.** Tables show health and uptime, details include health-check results, and indicators reflect crash loops, failed health checks and missing state.
- **Docker networks summarise their containers.** Each network has one status indicator. Running counts use Docker's running state, while attached-container state text reflects health.
- **Swarm rollout states are distinct.** Updating, rolling back, Update paused and Rollback paused have separate labels. Missing running replicas takes priority, and task, replica and node status text reflects the problem.
- **Image checks preserve update warnings.** Unknown local digests and daily cleanup no longer clear update alerts incorrectly. Explicit Docker Hub references reach the correct registry, unavailable checks explain why, and image names stay readable.
- **Updates sit beside the warning.** Supported containers offer Update and View image tags. Pulse explains unavailable updates, review dialogs survive refreshes, and update plans retain the saved deployment target.
- **Uncertain restarts get checked.** After a missing Docker restart response, Pulse reads container state within the original deadline without repeating the restart. Observed health stays distinct from confirmed execution.
- **Podman CPU uses the measured interval.** Reports, history and alerts agree on usage. Small positive readings remain visible as fractions, with help explaining the percentage scale.

## Install, updates and agents

- **Cluster agents need fewer manual steps.** A connected Proxmox member can install agents on uncovered siblings using existing cluster SSH trust and individual bootstrap credentials. Progress confirms a live connection.
- **Enrollment finishes before collection starts.** Non-Windows agent modules use the exchanged runtime credential from their first start. Command channels use Pulse's resolved identity and remain connected after restarts.
- **Command access is an explicit choice.** Setup explains and asks for command access before creating credentials. Reporting agents can establish identity without a command channel, and action runners recognise restored certificate trust.
- **Docker commands reach their host.** Container and other Docker child identities no longer make a unique reporting host ambiguous. Genuine collisions produce clear errors instead of appearing as a missing connection.
- **Removal preserves recovery files.** Unix, Windows, Synology DSM and TrueNAS SCALE removal confirms services and boot tasks have stopped before deleting credentials or recovery state.
- **Agent setup protects credentials.** Installers reject unsafe paths and incomplete downloads, keep token entry private, respect selected token files and retain credentials when saved server details are missing. Removal commands use saved credentials.
- **NAS agents survive reboot correctly.** Updates preserve executable permissions and persistent binary copies. Unraid starts its agent in the background so later boot commands can run, while preserving unrelated boot entries.
- **Server updates prepare before stopping Pulse.** Replacement binaries are checked first, shutdown is confirmed, and uncertain service state stops replacement. Archives stage on the install filesystem, avoiding failures with a separate /tmp.
- **Updates respect saved choices.** Manual and automatic updates read saved channels reliably. Automatic updates honour consent and systemd masks, exclude unknown editions, and confirm the running executable matches the installed version.
- **Update failures remain diagnosable.** Installer and log-collector results stay separate, including final output without a newline. Recovery files and earlier configuration snapshots survive retries, and source-built installs use atomic replacement.
- **Imports and resets protect existing installs.** Startup import requires the actual destination to be empty, including agent-only, TrueNAS and vSphere setups. Reset and removal wait for affected services to stop.
- **Setup reports more accurately.** Forms ask for the address before its name. Coverage, CPU counts, distribution names and agent notices are clearer. Statistics opt-out avoids a false failure, and empty service inventories are handled correctly.

## Pulse Pro, AI and hosted

- **Patrol setup lives on Patrol.** Connect a provider, choose a guided model and preview costs on the page, with unreachable providers linking to Provider & Models. Pro defaults to Ask first until you choose a mode, with Safe auto-fix and Autopilot available.
- **Alert investigations run through Patrol.** The web Assistant chat window is removed, and Investigate starts a resource-specific Patrol run with findings on Patrol. External AI clients can continue using Pulse MCP.
- **Guest protection stays effective.** Never auto-remediate blocks actions and previously approved plans across links, splits and identity changes. Approved actions check current policy. The inactive Assistant Protected guests field is removed.
- **Planned silence stays under your control.** Maintenance includes recurring windows and child resources, while expected-offline settings silence availability only. Activity lets you remove manual suppression rules without reopening reviewed findings.
- **Investigations target the right resource.** Filesystem and disk findings resolve to their owner. Repeated names stay distinct, while decision, action-review and threshold links open the relevant resource or settings row.
- **AI uses collected evidence.** AI can read backup and storage history without treating old or cache-inclusive readings as proof of health or pressure. Unavailable storage stays explicit, and MCP filters can select only resources with findings.
- **Patrol resumes its schedule after restart.** Saved schedules retain their remaining wait, overdue checks run once, and complete runs clear obsolete coverage warnings. Historical fixes cannot close newly recurring findings.
- **Interrupted investigations keep a next step.** Empty replies, timeouts, invalid tool replies and uncertain actions allow scheduled follow-up within policy limits. Hung runs finish with a clear failure, and older runs cannot overwrite newer findings.
- **Investigation results stay consistent.** Conclusions record explicit outcomes, including interrupted and unclassified runs. Reviews survive refreshes, queued fixes hide Re-investigate, and the weekly digest counts each finished investigation once.
- **AI Discovery is removed.** Service context scans, the Discovery tab and automatic web-link and availability proposals are no longer offered.
- **Pro is easier to activate and find.** Eligible paid systemd and Proxmox LXC installs can switch through the API, while Docker gets image commands, with older builds and stable-channel prereleases refused. Plans & Billing shows pricing and a working Buy.
- **Hosted installs recover better.** Initial setup stays reachable without a licence, paid renewals survive restart, and tenant deletion and shutdown wait for active monitoring, alert checks and history writes.

## Monitoring and service health

- **Freshness follows actual polling.** Proxmox, PBS, Mail Gateway, connections and resource APIs use the live fixed or adaptive schedule. Connection alerts also continue after monitor reloads.
- **Silent agents get accurate wording.** Pulse says an agent stopped reporting without claiming its host is down. Offline source states remain effective, linked agents use their own heartbeat, and failed availability checks cannot keep nodes online.
- **Availability views agree.** Settings and Machines show consistent results. Old successful locations cannot look current because another location failed more recently, and licence changes restore local checks correctly.
- **Retained readings cannot create fresh trends.** Old guest measurements no longer become new history points or bridge gaps. Memory and disk searches exclude unavailable measurements instead of treating them as zero.
- **Primary IP selection respects interfaces.** Normal guest interfaces take priority over container and overlay interfaces. Interface names and their addresses stay together, and all reported addresses remain available.
- **CPU temperatures agree across views.** Tables, details, history and alerts use the hottest current CPU reading. Node details show real low and record values without mixing readings from same-named nodes at other sites.
- **Temperature preferences reach platform tables.** Saved temperature units apply consistently to readings and headings. Missing or retained disk readings keep their collection status when units change.
- **Ages keep moving.** Resource, backup, replication, alert, delivery, Patrol, token, installer and diagnostic ages update without refreshing. A stalled watchdog request no longer leaves Refresh waiting indefinitely.
- **Connections recover more reliably.** Startup retries continue beyond five minutes, backoff stays capped, and cached DNS tries reachable alternatives. Shared Proxmox and PBS login renewal survives one caller cancelling.
- **Kubernetes problems stand out.** Not Ready nodes lead the overview. Details surface cordoned nodes, unready containers, replica shortfalls and autoscalers at their limit. Completed pods avoid false warnings, and rows reflect their alerts.
- **Mail Gateway 9 collection works again.** String-valued cluster IDs no longer break collection or trigger guessed backup requests. Failed reads retain known backups, gateway alerts stay attached, and saved collection opt-outs remain explicit.
- **Health follows the correct source.** Pulse service failures stay separate from resource alerts, and guest-agent fallback requires real replies. Demo switches clear old alerts, slower polls keep sightings current, and sample memory stays within capacity.

## Security

- **Tokens stay within their authority.** Callers cannot grant scopes they lack or turn limited access into instance-wide credentials. Token lists, diagnostics, editing, deletion and rotation respect the caller's organizations and permissions.
- **Instance administration stays with instance admins.** Organization owners and organization-bound tokens cannot change global credentials, security, licensing or updates. They retain access to their own organization's administration.
- **Organizations keep separate data.** Live events, reports and AI metric summaries stay with their organization. Missing tenant monitors cannot expose default-org data, and a custom metrics database path applies only to the default organization.
- **Changes require the right permissions.** Links and attention changes require monitoring:write, AI report narration requires ai:execute, and browser agent removal requires admin access. Read-only tokens no longer see link-editing controls.
- **AI checks command arguments.** Read-only decisions inspect options and each pipeline stage, preventing familiar command names from disguising file writes, service changes or other actions requiring approval.
- **SAML replies stay tied to their request.** Sign-in and logout responses must match the outstanding browser request. Replays are refused, and manually configured identity providers use the saved certificate for signature checks.
- **Certificate trust survives recovery.** Proxmox preserves saved pins and rejects conflicts for equivalent addresses. TLS checks reject compromised intermediates, and runtime and browser dependencies receive security fixes.
- **Redirects keep credentials within their destination.** Infrastructure and notification requests refuse redirects to another origin. Configure the final notification URL directly when a receiver redirects elsewhere.
- **Webhook handling exposes less.** Receiver text, private template content and unsafe retry hints stay out of logs. Saved secrets remain masked, and destination checks reject forbidden IPv4 addresses embedded in IPv6.
- **Demo restrictions remain enforced.** WebSocket headers cannot bypass blocked changes, and profiling endpoints are hidden on demo instances.
- **Security status reflects exposure.** Plain HTTP is critical when Pulse is publicly reachable. Private-network installations receive an HTTPS recommendation.
- **Browser settings apply consistently.** SSO changes take effect together, and changes refuse read-only HTTP methods. Clearing allowed origins restores same-origin access, while environment-controlled origin settings remain locked.

## Other improvements

- **Tables behave predictably.** Selecting text no longer toggles rows, expanded details span visible columns, and workload selection avoids obsolete scroll positions. Long Kubernetes names copy correctly, badges remain whole, and filter focus stays put.
- **Host rows reveal their workloads.** Docker and vSphere hosts open their own workload lists. Fleet details open beneath the selected tile, and the vCenter column appears only when several are connected.
- **More inventory is inspectable.** Kubernetes details expose deployment, autoscaler, service, ingress and volume facts. Docker image and volume details show their inventory, and TrueNAS record details open the relevant object.
- **Kiosk views show more monitoring.** Filter toolbars and section tabs are hidden, and table preview limits are lifted.
- **Controls are easier to read.** Light-mode banners have stronger contrast, disabled toggles show their actual state, and Manage explains each operator control. Long explanations wrap, and styling works with Pulse's security policy.
- **Connections are easier to check.** Infrastructure entries open the platform showing that system's data. Proxmox, PBS and Mail Gateway tests use edited certificate settings with saved credentials. Denied update checks explain the separate permissions needed.
- **Logs stay responsive.** Slow viewers no longer block server logging, and the browser batches incoming lines. Pause, clear and download controls explain their effects.
- **Views can be bookmarked.** Patrol and Actions have dedicated routes, and links to Patrol's mode selector open the relevant control.
- **Troubleshooting gives safer next steps.** Guides cover backup safety, permissions, Kubernetes, OIDC, CORS and NAS setup with limited checks that keep credentials private. Bug reports distinguish guests from the server and make extra diagnostics optional.
- **SSO feedback stays with your edits.** Save, copy, metadata and test results stay inside the editor, with reachable save controls and no duplicate saves. Group names retain spaces, and errors stay visible above dialogs.
- **Audit requests fail clearly.** Malformed export and summary time filters are rejected, while concurrent tenant incident records keep distinct identifiers. Cluster diagnostics, safety notices and setup dialogs handle keyboard focus more predictably.
- **Optional telemetry explains usage.** Previews show extended-history reads and aggregate report use without resource identities or readings, respecting opt-out. The star prompt waits until the first return day.

## Known issues

- **Guest safety remains under investigation.** These changes do not confirm a fix for every guest freeze or Windows restart. If monitoring was stopped after a guest froze, do not restart it solely to test this beta or this release.
- **Affected backups still need precautions.** Stop Pulse monitoring before a freeze-enabled backup and keep it stopped until independent checks confirm thaw, fresh writes on every backed-up filesystem and workload liveness. An OK backup does not prove thaw.
- **Elevated server memory remains unresolved.** Smaller caches and reduced copying do not confirm a lower long-running baseline. Report higher steady usage as well as continued growth using existing readings at similar uptimes and workloads.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.5.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.5.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.5.0` and recreate the container.
