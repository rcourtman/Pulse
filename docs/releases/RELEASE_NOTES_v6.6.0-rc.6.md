# Pulse v6.6.0-rc.6 Release Notes

This release candidate follows `v6.6.0-rc.5` on the way to stable `v6.6.0`. Stable `v6.5.0` remains the rollback target.

Pulse v6.6.0 brings alerts, charts and useful next steps together so you can see what needs attention across your homelab. Backup coverage, disk health and guest readings explain when information is old or incomplete. Cluster agent installation, scheduled reports and notification delivery need less manual work. Reports of guest freezes, Windows restart problems and elevated server memory remain unresolved.

## Highlights

- Find problems and inspect their alerts, trends and relevant controls together.
- Install agents across a Proxmox cluster and schedule routine reports with fewer manual steps.
- Distinguish current readings from retained data, incomplete collection and missing backup history.

## What's improved

- **Charts sit beside the problem.** Resource Overview combines alerts, recent trends and longer history for guests, nodes, agents and containers. Minimum-to-maximum bands preserve brief spikes. The separate History tab is removed.
- **History is easier to interpret.** Consistent ranges, clearer axes, dated values and source labels explain readings. Inspected values survive resizing, missing readings leave gaps, and limited histories retain the newest observations.
- **Charts show relevant alerts.** Resource history includes its own alerts and trigger lines without unrelated parent alerts. Row history also charts network and disk I/O.
- **Details lead with useful information.** Drawers share clear headings, status indicators that match their rows, visible warnings and a closed Details section for inventory. Names and identifiers wrap so you can read and copy them.
- **Web links can be edited together.** Bulk-edit saved interfaces for workloads and Docker containers, then open a guest's saved interface from its header.
- **Guests keep their identity when moved.** Proxmox migrations preserve metadata, links, selection and history. Matching names or guest IDs at separate installations stay distinct.
- **Manual grouping stays yours.** Links, chained links and splits survive refreshes and identity changes. Unlinking separates rows, while reporting a bad merge removes only the responsible links.
- **Large dashboards do less repeated work.** Inventory updates reduce copying, grouping and sorting. Tables fetch only displayed history, hidden charts pause collection, and stored chart queries reduce decoding and disk lookups.
- **History uses less temporary memory.** Database caches have fixed limits. Alert history copies only requested entries and retains the latest diagnostic detail, reducing memory used while browsing large histories.
- **VM inventory exports can run on a schedule.** Export the organization's current VM inventory as CSV weekly or monthly, including newly discovered VMs. Email it or save it to disk.
- **Backup health gets a scheduled report.** Compatible Pro runtimes offer weekly or monthly PDF and CSV summaries of Proxmox backup attempts, including jobs writing to PBS. Failures, missing attempts and incomplete history remain explicit.
- **Report delivery is easier to recover.** Settings shows readiness and results. Temporary email failures retry within limits, retries preserve the report period, and uncertain sends warn about duplicates. Schedules survive monitor reloads.

## Alerts and notifications

- **Alerts open the right place.** Overview prioritises active problems. Guest and filesystem alerts open their guest, storage alerts open their pool, and notifications open the alert's timeline when a public URL is configured.
- **Open alerts explain their state.** Threshold alerts show the current reading, averaging window, clear level and recovery progress while preserving the original breach value and date.
- **Missing readings interrupt confirmation.** Metric, storage, connection and disk-temperature timers restart after observation gaps instead of treating separated readings as continuous trouble.
- **Alert choices survive saves.** Disabled Docker, Mail Gateway and backup limits stay off. Zero-delay alerts remain immediate, low limits keep a lower clear level, and pulse-relaxed tags agree across colours, polling and saved settings.
- **Edits survive uncertain saves.** Omitted settings and node overrides remain intact. Alert and notification drafts wait for confirmed saves, editors wait for saved policy, and threshold controls have accessible names.
- **Fresh installs get consistent defaults.** Docker alerts start with factory thresholds, automatic acknowledgement starts off, and quiet hours use the server's local time zone unless configured.
- **History explains closures.** Moved alerts stay distinct from recovered alerts in timelines, reports and notifications. Host, container, service, disk, array and sensor history stays with its resource.
- **History stays readable.** Messages wrap, frequency bars show severity and local times, and acknowledgements sync between tabs. Read failures remain visible, caches have limits, and delivery history explains when retries stopped.
- **Delivery controls explain their effect.** Destinations come first, schedules explain timing, and pause messages distinguish detection from delivery. Email settings remain editable while delivery is off, without resetting rate limits.
- **Webhooks send what you configured.** Required template headers reach real alerts, including Telegram. Clearing optional headers restores defaults, examples escape alert text, and invalid templates produce configuration errors.
- **Email works with more SMTP servers.** Provider defaults, LOGIN prompts and port 465 work consistently. Concurrent sends keep recipients separate. Subjects identify hosts and support non-English names.
- **Delivery failures get clearer handling.** Notifications try another approved address when one stalls, cancelled DNS work stops, and interrupted replies cannot count as completed sends. Temporary disconnects remain eligible for retry.

## Disks and storage

- **Capacity problems lead the storage view.** Tables emphasise filling pools and explain attention states. Collapsing groups no longer pins them, reopens them or scrolls back to an earlier pool.
- **Storage details explain what needs attention.** Pool drawers surface reduced protection, ZFS errors and failing or hot members. Disk drawers bring alerts, SMART details and collection limits together without repeating the same warning.
- **Storage charts stay close.** Pool and disk details start with 24-hour charts and offer longer history. Disk charts include temperature, I/O and relevant wear counters where available. Configuration, topology and datasets sit under Details.
- **Unrelated disks stay separate.** Matching serials on different hosts and generic QEMU serials no longer merge different hardware. Agent disks stay out of unrelated cluster-member inventory.
- **Hardware matching handles different ID formats.** Equivalent WWNs from Proxmox and agents match correctly, including controller disks. Empty or placeholder identifiers cannot establish a match.
- **Replacement disks start clean.** New hardware no longer inherits the old disk's SMART readings, slot identity, alert history or operator settings when its identity differs.
- **Retained readings show their age.** Standby disks and silent agents show last-known temperatures, usage and RAID state. Merged disks preserve each source's collection state.
- **Heat limits agree across views.** Alerts, colours, reports and Patrol use disk-type limits and host overrides. TrueNAS disk limits are editable, and Unraid uses its agent's reported temperature.
- **Heat alerts close for the right reason.** Removing a disk or disabling its temperature threshold clears its alert. Missing reports preserve unresolved SMART problems without presenting old temperatures as current.
- **Risk labels explain the problem.** Heat alone reads Running Hot rather than Replace Now. SSD wear is labelled life remaining, and disk alerts are logged once.
- **History follows the hardware.** Controller-member and Unraid serial-based charts resolve correctly. Same-serial disks on different hosts retain separate charts, and TrueNAS replacements cannot inherit the previous disk's native history.
- **Real local mounts remain visible.** Mounts such as /snapshots and /runtime are no longer excluded merely because their paths resemble filtered system directories.

## Proxmox, PBS and backups

- **Guest memory explains its limits.** Views and alerts distinguish current, retained and cache-inclusive VM readings. Percentages use sampled capacity, including ballooned VMs. LXC listing memory is treated as cache-adjusted usage.
- **Windows readings can recover.** Guest reads can resume after cooldowns and restarts. Pulse learns the OS before optional Linux memory requests and collects filesystems first, preventing unsuitable requests from blocking disk readings.
- **Filesystem totals avoid false duplicates.** Shared filesystems count once regardless of mount order. Distinct Windows volumes remain separate even when equally sized, and invalid counters remain unknown.
- **Partial filesystem reads remain useful.** Valid rows survive incomplete responses without implying complete capacity or recovery. Stopped guests no longer publish retained filesystems as current. Disk percentages stay distinct from byte totals.
- **LXC collection works within the helper sandbox.** Running containers and mounted filesystems can be read without Proxmox control sockets. Unmounted paths cannot borrow parent usage, and nodes without containers report an empty inventory.
- **Guest commands respect backup safeguards.** Reads and actions defer for locks, uncertain completion and malformed replies. Equivalent endpoints share coordination, and cancellation cannot prematurely release another command.
- **Incomplete polls preserve inventory.** Failed node enumeration retains known guests without refreshing their readings. Completed inventory survives optional-detail deadlines, and retired connections cannot republish old results.
- **PBS coverage survives incomplete polls.** Missing namespace or snapshot results preserve known backups without marking them freshly checked. Root-namespace coverage includes LXC guests, and restored backup alerts are checked before startup delivery.
- **Backup coverage is easier to inspect.** Health counts filter matching workloads. Details show completed backup times, distinguish freshness from verification and restore the PBS interface shortcut. Each PBS server shows host statistics once.
- **Amber shields explain their limit.** Backup details name the configured age limit and link to Recovery thresholds. Saved changes update shields immediately, making longer backup schedules easier to configure.
- **Replication shows observed results.** Status comes from source-node job observations. Slow reads do not pile up, failed reads preserve jobs, and saved configuration alone no longer implies successful replication.
- **Access messages give useful next steps.** Denied package-update checks explain their separate permissions without treating limited access as a node fault. Outdated sensor notices appear across Proxmox tabs.

## TrueNAS, vSphere and Docker

- **TrueNAS CORE keeps usable inventory.** Unknown or string-valued disk rotation rates no longer abort collection. Pool capacity can use top-level vdev statistics when flat fields are absent. Missing or conflicting readings remain unknown.
- **TrueNAS charts and app readings recover.** Native disk-temperature history works again, with a limited fallback for empty reporting results. Fractional SCALE app CPU readings no longer interrupt reporting sessions.
- **TrueNAS explains unhealthy states.** Pools and topology give concrete reasons for attention. Services and VMs distinguish intentionally stopped items from those expected to run, with status text matching its indicator.
- **vSphere leads with health.** Overview explains vCenter, datastore and network problems. Events use vCenter descriptions, host groups use cluster names, and VM records keep their platform. The vCenter column appears only with several connections.
- **Container health is consistent.** Tables show health and uptime, details include health-check results, and indicators reflect crash loops, failed health checks and missing state.
- **Docker networks summarise their containers.** Each network has one status indicator. Running counts use Docker's running state, while attached-container state text reflects health.
- **Swarm rollout states explain pauses.** Active updates, paused updates and paused reversions have distinct labels. Missing running replicas takes priority, and task, replica and node status text reflects the problem.
- **Image warnings stay accurate.** Missing local digests and daily cleanup no longer clear update alerts. Explicit Docker Hub names reach the right registry. Locally built and digest-pinned images have neutral labels instead of failed checks.
- **Updates sit beside the warning.** Container details offer Update and View image tags. Review dialogs survive refreshes, and update plans retain the saved deployment target.
- **Update controls explain what is possible.** Tables and details agree when inventory is stale, an agent is disconnected or command access is missing. Unavailable updates give a reason and point to the host's container manager.
- **Uncertain restarts get checked.** After a missing Docker restart response, Pulse reads container state within the original deadline without repeating the restart. Observed health stays distinct from confirmed execution.
- **Podman CPU uses the measured interval.** Reports, history and alerts agree on usage. Small positive readings remain visible as fractions, with help explaining the percentage scale.

## Install, updates and agents

- **Cluster agents need fewer manual steps.** A connected Proxmox member can install agents on uncovered siblings using existing cluster SSH trust and individual bootstrap credentials. Progress confirms a live connection.
- **Enrollment finishes before collection starts.** Non-Windows agent modules use the exchanged runtime credential from their first start. Command channels use Pulse's resolved identity and remain connected after restarts.
- **Command access is an explicit choice.** Setup explains root command access before creating credentials. Leave it off for monitoring only. To add container controls or Patrol actions to an existing agent, rerun installation with command access selected.
- **Docker commands reach their host.** Container and other Docker child identities no longer make a unique reporting host ambiguous. Genuine collisions produce clear errors instead of appearing as a missing connection.
- **Removal preserves recovery files.** Agent removal on Unix, Windows and NAS devices confirms services, boot tasks and watchdogs have stopped before deleting credentials or recovery state.
- **Agent setup protects credentials.** Installers reject unsafe paths and incomplete downloads, keep token entry private, respect selected token files and retain credentials when saved server details are missing. Removal commands use saved credentials.
- **NAS updates survive reboot correctly.** Agent updates preserve executable permissions and refresh persistent binary copies on supported NAS installations.
- **Server updates prepare before stopping Pulse.** Replacement binaries are checked first, shutdown is confirmed, and uncertain service state stops replacement. Archives stage on the install filesystem, avoiding failures with a separate /tmp.
- **Updates respect saved choices.** Manual and automatic updates read saved channels reliably. Automatic updates honour consent and systemd masks, exclude unknown editions, and confirm the running executable matches the installed version.
- **Update failures remain diagnosable.** Installer and log-collector results stay separate, including final output without a newline. Recovery files remain when completion is uncertain, and source-built installs use atomic replacement.
- **Imports and resets protect existing installs.** Startup import requires the actual destination to be empty, including agent-only, TrueNAS and vSphere setups. Reset and removal wait for affected services to stop.
- **Setup reports more accurately.** Forms ask for the address before its name. First-run coverage, CPU counts, distribution names and agent notices are clearer. Statistics opt-out avoids a false failure, and empty service inventories are handled correctly.

## Pulse Pro, AI and hosted

- **Patrol setup lives on Patrol.** Connect a provider, choose a guided model, preview costs and check readiness from the page. Unreachable providers lead to Provider & Models.
- **Actions start with review.** Pro Patrol defaults to Ask first until a mode is chosen. Safe auto-fix and Autopilot remain available. Assistant offers Read-only and Ask first, with proposed actions saved in Actions.
- **Guest protection stays effective.** Never auto-remediate blocks actions and previously approved plans across links, splits and identity changes. Approved actions check current policy. The inactive Assistant Protected guests field is removed.
- **Patrol respects planned silence.** Recurring maintenance covers child resources, expected-offline settings silence availability only, and manually added suppression rules can be removed in Activity without reopening reviewed findings.
- **Investigations target the right resource.** Filesystem and disk findings resolve to their owner. Repeated names stay distinct, while decision, action-review and threshold links open the relevant resource or settings row.
- **Investigations use collected data.** AI can read backup inventory, attempts, errors, verification and storage history. Old readings cannot prove health, cache-inclusive VM memory cannot prove pressure, and unavailable storage stays explicit.
- **Patrol resumes its schedule after restart.** Saved schedules retain their remaining wait, overdue checks run once, and complete runs clear obsolete coverage warnings. Historical fixes cannot close newly recurring findings.
- **Incomplete investigations keep a next step.** Empty replies, summary timeouts, tool calls printed as prose and uncertain action results remain eligible for scheduled follow-up within policy limits. Chat preserves its streamed summary.
- **AI Discovery is removed.** Service context scans, the Discovery tab and automatic web-link and availability proposals are no longer offered.
- **Paid installs can switch to Pro in place.** Eligible systemd and Proxmox LXC installs can switch through the API. Docker gets image commands. Older Pro versions are refused, and the stable channel cannot switch to a prerelease.
- **Plans and purchases are easier to find.** Free installs can open Plans & Billing. The Pro card shows its price and grouped features, and Buy opens the working purchase page.
- **Hosted installs recover better.** Initial setup stays reachable without a licence, paid renewals survive restart, and tenant deletion and shutdown wait for active monitoring, alert checks and history writes.

## Monitoring and service health

- **Freshness follows actual polling.** Proxmox, PBS, Mail Gateway, connections and resource APIs use the live fixed or adaptive schedule. Connection alerts also continue after monitor reloads.
- **Silent agents get accurate wording.** Pulse says an agent stopped reporting without claiming its host is down. Offline source states remain effective, linked agents use their own heartbeat, and failed availability checks cannot keep nodes online.
- **Availability views agree.** Settings and Machines show consistent results. Old successful locations cannot look current because another location failed more recently, and licence changes restore local checks correctly.
- **Agent handovers preserve coverage.** Linked agents avoid duplicate node alerts without losing temperature monitoring. Monitoring-only agents can reduce permissions, and Proxmox cleanup preserves alerts belonging to other platforms.
- **Retained readings cannot create fresh trends.** Old guest measurements no longer become new history points or bridge gaps. Memory and disk searches exclude unavailable measurements instead of treating them as zero.
- **Primary IP selection respects interfaces.** Normal guest interfaces take priority over container and overlay interfaces. Interface names and their addresses stay together, and all reported addresses remain available.
- **CPU temperatures agree across views.** Tables, details, history and alerts use the hottest current CPU reading. Node details show real low and record values without mixing readings from same-named nodes at other sites.
- **Ages keep moving.** Resource, backup, replication, alert, delivery, Patrol, token, installer and diagnostic ages update without refreshing. A stalled watchdog request no longer leaves Refresh waiting indefinitely.
- **Connections recover more reliably.** Startup retries continue beyond five minutes, backoff stays capped, and cached DNS tries reachable alternatives. Shared Proxmox and PBS login renewal survives one caller cancelling.
- **Kubernetes problems stand out.** Not Ready nodes lead the overview. Details surface cordoned nodes, unready containers, replica shortfalls and autoscalers at their limit. Completed pods avoid false warnings, and rows reflect their alerts.
- **Mail Gateway 9 collection works again.** String-valued cluster IDs no longer break collection or trigger guessed backup requests. Failed reads retain known backups, gateway alerts stay attached, and saved collection opt-outs remain explicit.
- **Health follows the correct source.** Pulse service failures stay separate from resource alerts, and guest-agent fallback requires real replies. Demo switches clear old alerts, while slower demo polls keep guest, storage and Ceph sightings current.

## Security

- **Tokens stay within their authority.** Callers cannot grant scopes they lack or turn limited access into instance-wide credentials. Token lists, diagnostics, editing, deletion and rotation respect the caller's organizations and permissions.
- **Instance administration stays with instance admins.** Organization owners and organization-bound tokens cannot change global credentials, security, licensing or updates. They retain access to their own organization's administration.
- **Organizations keep separate data.** Live events, reports and AI metric summaries stay with their organization. Missing tenant monitors cannot expose default-org data, and a custom metrics database path applies only to the default organization.
- **Changes require the right permissions.** Links and attention changes require monitoring:write, AI report narration requires ai:execute, and browser agent removal requires admin access. Read-only tokens no longer see link-editing controls.
- **Assistant checks command arguments.** Read-only decisions inspect options and each pipeline stage, preventing familiar command names from disguising file writes, service changes or other actions requiring approval.
- **SAML replies stay tied to their request.** Sign-in and logout responses must match the outstanding browser request. Replays are refused, and manually configured identity providers use the saved certificate for signature checks.
- **Certificate trust survives recovery.** Proxmox preserves saved pins and rejects conflicts for equivalent addresses. Action runners recognise restored trust. TLS checks reject compromised intermediates, and browser dependencies receive security fixes.
- **Redirects keep credentials within their destination.** Infrastructure and notification requests refuse redirects to another origin. Configure the final notification URL directly when a receiver redirects elsewhere.
- **Webhook handling exposes less.** Receiver text, private template content and unsafe retry hints stay out of logs. Saved secrets remain masked, and destination checks reject forbidden IPv4 addresses embedded in IPv6.
- **Demo restrictions remain enforced.** WebSocket headers cannot bypass blocked changes, and profiling endpoints are hidden on demo instances.
- **Security status reflects exposure.** Plain HTTP is critical when Pulse is publicly reachable. Private-network installations receive an HTTPS recommendation.
- **Browser settings apply consistently.** SSO changes take effect together. State changes refuse read-only HTTP methods. Clearing allowed origins restores same-origin access, and environment-controlled origin settings remain locked.

## Other improvements

- **Tables behave predictably.** Selecting text no longer toggles rows, expanded details span visible columns, and workload selection avoids obsolete scroll positions. Long Kubernetes names copy correctly, badges remain whole, and filter focus stays put.
- **More inventory is inspectable.** Kubernetes details expose deployment, autoscaler, service, ingress and volume facts. Docker image and volume details show their inventory, including values hidden by narrower table layouts.
- **Kiosk views show more monitoring.** Filter toolbars and section tabs are hidden, and table preview limits are lifted.
- **Controls are easier to read.** Light-mode banners have stronger contrast, disabled toggles show their actual state, and Manage explains each operator control in plain language.
- **Connections are easier to check.** Infrastructure entries open the platform showing that system's data. Proxmox, PBS and Mail Gateway connection tests use edited certificate settings with saved credentials before you save changes.
- **Logs stay responsive.** Slow viewers no longer block server logging, and the browser batches incoming lines. Pause, clear and download controls explain their effects.
- **Views can be bookmarked.** Patrol and Actions have dedicated routes, and links to Patrol's mode selector open the relevant control.
- **Troubleshooting gives safer next steps.** Guides cover backup safety, permissions, Kubernetes, OIDC, CORS and NAS setup with limited checks that keep credentials private. Bug reports distinguish guests from the server and make extra diagnostics optional.
- **Audit requests fail clearly.** Malformed export and summary time filters are rejected, while concurrent tenant incident records keep distinct identifiers.
- **The GitHub star prompt is less abrupt.** It appears on the first return day with clearer presentation.
- **Optional telemetry explains more usage.** Preview includes counts of successful extended-history reads and aggregate report outcomes and repeat use. Resource identities and readings stay excluded, and opt-out is respected.
- **Keyboard access is clearer.** Cluster diagnostics, safety notices and setup dialogs handle focus more predictably. Production styling displays correctly under Pulse's content security policy.

## Known issues

- **Guest safety remains under investigation.** These changes do not confirm a fix for every guest freeze or Windows restart problem. If monitoring was stopped after a guest froze, do not restart it solely to test this beta or this release.
- **Affected backups still need precautions.** Stop Pulse monitoring before a planned backup. Keep it stopped until the backup ends and independent checks confirm thaw, fresh writes on every backed-up filesystem and workload liveness.
- **Elevated server memory remains unresolved.** Smaller caches and reduced copying do not confirm a lower long-running baseline. Report higher steady usage as well as continued growth, comparing existing readings at similar uptimes and workloads.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.5.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.5.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.5.0` and recreate the container.
