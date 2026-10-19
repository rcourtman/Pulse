# Pulse v6.6.0-rc.5 Release Notes

This release candidate follows `v6.6.0-rc.4` on the way to stable `v6.6.0`. Stable `v6.5.0` remains the rollback target.

Pulse v6.6.0 puts alerts, trends and useful next steps together so you can see what needs attention across your homelab. Backup coverage, disk health and guest readings explain when information is old or incomplete. Cluster agent installation, scheduled reports, notifications and updates need less manual work. Reports of guest freezes, Windows restarts and elevated server memory remain unresolved.

## Highlights

- See problems, recent trends and longer history together in resource details.
- Install cluster agents and schedule inventory and backup reports with fewer manual steps.
- Get clearer alerts when readings stop, hardware changes or guests move.

## What's improved

- **Charts sit beside the problem.** Resource Overview combines alerts, recent trends and longer history for guests, nodes, agents, Docker hosts and containers. The separate History tab is removed.
- **Brief spikes stay visible.** Charts show minimum and maximum readings around average lines, helping distinguish short peaks from sustained load.
- **History is easier to follow.** Consistent ranges, clearer axes, dated values and source labels explain readings. Inspected values survive resizing, gaps remain visible, and bounded histories retain the newest observations.
- **Charts show relevant alerts.** Resource history includes its own alerts and trigger lines without unrelated parent alerts. Row history also charts network and disk I/O.
- **Useful details come first.** Resource details lead with warnings and failed availability checks. Inventory sits under Details, with complete identifiers and fewer repeated values.
- **Web links can be edited together.** Bulk-edit saved interfaces for workloads and Docker containers, then open a guest's saved interface from its header.
- **Guests keep their identity when moved.** Proxmox migrations preserve metadata, links, selection and history. Matching names or guest IDs at separate installations stay distinct.
- **Manual grouping stays yours.** Links, chained links and splits survive refreshes and identity changes. Unlinking separates rows, while reporting a bad merge removes only the responsible links.
- **Large dashboards do less repeated work.** Inventory updates reduce copying, grouping and sorting. Tables fetch only displayed history, hidden charts pause collection, and stored chart queries do less decoding and disk lookup work.
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
- **History is clearer and bounded.** Messages wrap, frequency bars show severity and local times, and acknowledgements sync between tabs. Read errors stay visible, caches have limits, and retained failures say when retries stopped.
- **Delivery controls explain their effect.** Destinations come first, schedules explain timing, and pause messages distinguish detection from delivery. Notification drafts survive tab changes, uncertain saves and masked-secret updates.
- **Webhook sends match their templates.** Required headers reach real alerts, including Telegram. Cleared optional headers restore defaults, examples escape text safely, and invalid templates stop with a configuration error.
- **Email works with more SMTP servers.** Provider defaults, LOGIN prompts and port 465 work consistently in tests and delivery. Concurrent sends keep recipients separate. Subjects name the host, and rate limits persist while email is off.
- **Connection failures get better handling.** Notifications can reach another approved address when the first stalls, and cancelled DNS work stops. Interrupted replies cannot count as completed delivery, while temporary disconnects remain eligible for retry.

## Disks and storage

- **Capacity problems lead the storage view.** Tables emphasise filling pools and explain attention states. Collapsing groups no longer pins them, reopens them or scrolls back to an earlier pool.
- **Storage history stays with its resource.** Expanded pools and disks include collapsible history charts with minimum-to-maximum bands.
- **Unrelated disks stay separate.** Matching serials on different hosts and generic QEMU serials no longer merge different hardware. Agent disks stay out of unrelated cluster-member inventory.
- **Hardware matching handles different ID formats.** Equivalent WWNs from Proxmox and agents match correctly, including controller disks. Empty or placeholder identifiers cannot establish a match.
- **Replacement disks start clean.** New hardware no longer inherits the old disk's SMART readings, slot identity, alert history or operator settings when its identity differs.
- **Retained readings look retained.** Standby disks and silent agents show last-known temperatures, usage and RAID state. Merged disks preserve each source's collection state.
- **Heat limits agree across views.** Alerts, colours, reports and Patrol use disk-type limits and host overrides. TrueNAS disk limits are editable, and Unraid uses its agent's reported temperature.
- **Heat alerts close for the right reason.** Removing a disk or disabling its temperature threshold clears its alert. Missing reports preserve unresolved SMART problems without presenting old temperatures as current.
- **Risk labels explain the problem.** Heat alone reads Running Hot rather than Replace Now. SSD wear is labelled life remaining, and disk alerts are logged once.
- **History follows the hardware.** Controller-member and Unraid serial-based charts resolve correctly. Same-serial disks on different hosts retain separate charts.
- **Real local mounts remain visible.** Mounts such as /snapshots and /runtime are no longer excluded merely because their paths resemble filtered system directories.

## Proxmox, PBS and backups

- **Guest memory explains its limits.** Views and alerts distinguish current, retained and cache-inclusive VM readings. Percentages use sampled capacity, including ballooned VMs. LXC listing memory is treated as cache-adjusted usage.
- **Windows collection handles recovery better.** Guest reads can resume after cooldowns and restarts. Pulse learns the OS before optional Linux memory requests and collects filesystems first, so unsuitable requests cannot block later disk readings.
- **Filesystem totals avoid false duplicates.** Shared filesystems count once regardless of mount order. Distinct Windows volumes remain separate even when equally sized, and invalid counters remain unknown.
- **Partial filesystem reads remain useful.** Valid rows survive incomplete responses without implying complete capacity or recovery. Stopped guests no longer publish retained filesystems as current measurements.
- **LXC collection works within the helper sandbox.** Running containers and mounted filesystems can be read without Proxmox control sockets. Unmounted paths cannot borrow parent usage, and nodes without containers report an empty inventory.
- **Guest commands respect backup safeguards.** Reads and actions defer for locks, uncertain completion and malformed replies. Equivalent endpoints share coordination, and cancellation cannot prematurely release another command.
- **Slow polls retain valid inventory.** Completed inventory survives optional-detail deadlines. Retired connections cannot republish readings, temporary OS-read failures can recover, and shared login renewal survives one caller cancelling.
- **PBS coverage survives incomplete polls.** Missing namespace or snapshot results preserve known backups without marking them freshly checked. Root-namespace coverage includes LXC guests, and restored backup alerts are checked before startup delivery.
- **Backup coverage is easier to inspect.** Health counts filter matching workloads. Details track completed backups, distinguish freshness from verification and restore the PBS interface shortcut. Each PBS server shows host statistics once.
- **Amber shields explain their limit.** Backup details name the configured age limit and link to Recovery thresholds. Saved changes update shields immediately, making longer backup schedules easier to configure.
- **Replication shows observed results.** Status comes from source-node job observations. Slow reads do not pile up, failed reads preserve jobs, and saved configuration alone no longer implies successful replication.
- **Access messages give useful next steps.** Denied package-update checks explain their separate permissions without treating limited access as a node fault. Outdated sensor notices appear across Proxmox tabs.

## TrueNAS, vSphere and Docker

- **TrueNAS CORE keeps usable inventory.** Unknown or string-valued disk rotation rates no longer abort collection. Pool capacity can use top-level vdev statistics when flat fields are absent. Missing or conflicting readings remain unknown.
- **TrueNAS charts and app readings recover.** Native disk-temperature history works again, with a bounded fallback for empty reporting results. Fractional SCALE app CPU readings no longer interrupt reporting sessions.
- **TrueNAS explains unhealthy states.** Pools and topology give concrete reasons for attention. Services and VMs distinguish intentionally stopped items from those expected to run, with status text matching its indicator.
- **vSphere leads with health.** The overview highlights vCenter signals and explains datastore and network problems. Host groups use cluster names, and the vCenter column appears only when several are connected.
- **vSphere activity reads clearly.** Events use vCenter's descriptions, VM records retain the correct platform, and state text matches its status indicator's colour.
- **Container health is consistent.** Tables show health and uptime, details include health-check results, and indicators reflect crash loops, failed health checks and missing state.
- **Docker networks summarise their containers.** Each network has one status indicator. Running counts use Docker's running state, while attached-container state text reflects health.
- **Swarm rollout states are distinct.** Updating, rolling back, Update paused and Rollback paused have separate labels. Missing running replicas takes priority, and task, replica and node status text reflects the problem.
- **Image checks preserve update warnings.** Unknown local digests and daily cleanup no longer clear update alerts incorrectly. Explicit Docker Hub references reach the correct registry, unavailable checks explain why, and image names stay readable.
- **Updates sit beside the warning.** Container details offer Update and View image tags. Review dialogs survive refreshes, and update plans retain the saved deployment target.
- **Uncertain restarts get checked.** After a missing Docker restart response, Pulse reads container state within the original deadline without repeating the restart. Observed health stays distinct from confirmed execution.
- **Podman CPU uses the measured interval.** Reports, history and alerts agree on usage. Small positive readings remain visible as fractions, with help explaining the percentage scale.

## Install, updates and agents

- **Cluster agents need fewer manual steps.** A connected Proxmox member can install agents on uncovered siblings using existing cluster SSH trust and individual bootstrap credentials. Progress confirms a live connection.
- **Enrollment finishes before collection starts.** On non-Windows agents, modules use the exchanged runtime credential from their first start. Command channels use Pulse's resolved identity and remain connected after restarts.
- **Monitoring agents can reduce permissions.** Reporting agents can establish identity without a command channel. Action-runner setup recognises restored certificate trust while keeping command connections verified.
- **Docker commands reach their host.** Container and other Docker child identities no longer make a unique reporting host ambiguous. Genuine collisions are reported clearly instead of appearing as a missing agent connection.
- **Removal preserves recovery files.** Unix, Windows, Synology DSM and TrueNAS SCALE removal confirms services and boot tasks have stopped before deleting credentials or recovery state.
- **Agent setup protects credentials.** Installers reject unsafe paths and incomplete downloads, respect selected token files and preserve credentials when saved server details are missing. Copied removal commands use saved credentials.
- **Server updates prepare before stopping Pulse.** Replacement binaries are checked first, shutdown is confirmed, and uncertain service state stops replacement. Archives stage on the install filesystem, avoiding failures with a separate /tmp.
- **Updates respect saved choices.** Manual and automatic updates read saved channels reliably. Automatic updates honour consent and systemd masks, exclude unknown editions, and confirm the running executable matches the installed version.
- **Update failures remain diagnosable.** Installer and log-collector results stay separate, including final output without a newline. Recovery files remain when completion is uncertain, and source-built installs use atomic replacement.
- **Imports and resets protect existing installs.** Startup import requires the actual destination to be empty, including agent-only, TrueNAS and vSphere setups. Reset and removal wait for affected services to stop.
- **Setup gives clearer information.** Forms ask for the address before its name. First-run coverage, CPU counts, Linux distribution names and agent update notices are clearer. Usage-statistics opt-out no longer reports a false failure.
- **Empty service inventories work correctly.** No matching service files means no service was found, while genuine discovery failures remain visible.

## Pulse Pro, AI and hosted

- **Patrol setup lives on Patrol.** Connect a provider, choose a guided model, preview costs and check readiness from the page. Unreachable providers lead to Provider & Models.
- **Actions start with review.** Pro Patrol defaults to Ask first until a mode is chosen. Safe auto-fix and Autopilot remain available. Assistant offers Read-only and Ask first, with proposed actions saved in Actions.
- **Guest protection stays effective.** Never auto-remediate blocks actions and previously approved plans across links, splits and identity changes. Approved actions check current policy. The inactive Assistant Protected guests field is removed.
- **Suppression rules can be removed.** Patrol Activity lists manually created rules with their scope and reason. Ending suppression leaves reviewed findings reviewed.
- **Maintenance silences the right findings.** Patrol honours recurring windows and maintenance covering child resources. Expected-offline settings silence availability findings while allowing other problems through.
- **Investigations target the right resource.** Filesystem and disk findings resolve to their owner. Repeated names stay distinct, while decision, action-review and threshold links open the relevant resource or settings row.
- **Investigations use collected evidence.** AI can read backup inventory, attempts, errors, verification and storage history. Old readings cannot prove health, cache-inclusive VM memory cannot prove pressure, and unavailable storage stays explicit.
- **Patrol resumes its schedule after restart.** Saved schedules retain their remaining wait, overdue checks run once, and complete runs clear obsolete coverage warnings. Historical fixes cannot close newly recurring findings.
- **Incomplete investigations keep a next step.** Empty replies, summary timeouts, tool calls printed as prose and uncertain action results remain eligible for scheduled follow-up within policy limits. Chat preserves its streamed summary.
- **AI Discovery is removed.** Service context scans, the Discovery tab and automatic web-link and availability proposals are no longer offered.
- **Paid installs can switch to Pro in place.** Eligible systemd and Proxmox LXC installs can switch through the API. Docker gets image commands. Older Pro versions are refused, and the stable channel cannot switch to a prerelease.
- **Hosted installs recover better.** Initial setup stays reachable without a licence, paid renewals survive restart, and tenant deletion and shutdown wait for active monitoring, alert checks and history writes.

## Monitoring and service health

- **Freshness follows actual polling.** Proxmox, PBS, Mail Gateway, connections and resource APIs use the live fixed or adaptive schedule. Connection alerts also continue after monitor reloads.
- **Silent agents get accurate wording.** Pulse says an agent stopped reporting without claiming its host is down. Offline source states remain effective, linked agents use their own heartbeat, and failed availability checks cannot keep nodes online.
- **Availability views agree.** Settings and Machines show consistent results. Old successful locations cannot look current because another location failed more recently, and licence changes restore local checks correctly.
- **Retained readings cannot create fresh trends.** Old guest measurements no longer become new history points or bridge gaps. Memory and disk searches exclude unavailable measurements instead of treating them as zero.
- **Primary IP selection respects interfaces.** Normal guest interfaces take priority over container and overlay interfaces. Addresses stay associated with their interface, and all reported addresses remain available.
- **CPU temperatures agree across views.** Tables, details, history and alerts use the hottest current CPU reading. Node details show real low and record values without mixing readings from same-named nodes at other sites.
- **Ages keep moving.** Resource, backup, replication, alert, delivery, Patrol, token, installer and diagnostic ages update without refreshing. A stalled watchdog request no longer leaves Refresh waiting indefinitely.
- **Long outages avoid rapid retry loops.** Polling backoff retains its maximum delay through persistent failures.
- **Kubernetes problems stand out.** Not Ready nodes lead the overview, rows reflect their alerts, and status words match their indicators. Long names remain readable and copy without inserted breaks.
- **Mail Gateway 9 collection works again.** String-valued cluster IDs no longer break collection or trigger guessed backup requests. Failed reads retain known backups, gateway alerts stay attached, and saved collection opt-outs remain explicit.
- **Pulse health stays distinct.** Pulse service failures are separated from failed TrueNAS polling and other monitored-resource alerts. Guest-agent fallback requires real replies, rather than merely having the agent enabled.
- **Demo monitoring behaves consistently.** Mode changes update all tenant monitors and clear old sample alerts. Demo workloads have stable parents, distinct names and one history timeline per resource.

## Security

- **Tokens stay within their authority.** Callers cannot grant scopes they lack or turn limited access into instance-wide credentials. Token lists, diagnostics, editing, deletion and rotation respect the caller's organizations and permissions.
- **Instance administration stays with instance admins.** Organization owners and organization-bound tokens cannot change global credentials, security, licensing or updates. They retain access to their own organization's administration.
- **Organizations keep separate data.** Live events, reports and AI metric summaries stay with their organization. Missing tenant monitors cannot expose default-org data, and a custom metrics database path applies only to the default organization.
- **Changes require the right permissions.** Links and attention changes require monitoring:write, AI report narration requires ai:execute, and browser agent removal requires admin access. Read-only tokens no longer see link-editing controls.
- **Assistant checks command arguments.** Read-only decisions inspect options and each pipeline stage, preventing familiar command names from disguising file writes, service changes or other actions requiring approval.
- **SAML replies stay tied to their request.** Sign-in and logout responses must match the outstanding browser request. Replays are refused, and manually configured identity providers use the saved certificate for signature checks.
- **Certificate trust survives reconnection.** Proxmox recovery cannot replace an established certificate pin. Diagnostics distinguish timeouts from trust failures and inspect the configured endpoint.
- **Redirects keep credentials within their destination.** Infrastructure and notification requests refuse redirects to another origin. Configure the final notification URL directly when a receiver redirects elsewhere.
- **Webhook handling exposes less.** Receiver text, private template content and unsafe retry hints stay out of logs. Saved secrets remain masked, and destination checks reject forbidden IPv4 addresses embedded in IPv6.
- **Demo restrictions remain enforced.** WebSocket headers cannot bypass blocked changes, and profiling endpoints are hidden on demo instances.
- **Security status reflects exposure.** Plain HTTP is critical when Pulse is publicly reachable. Private-network installations receive an HTTPS recommendation.
- **Sign-in settings update consistently.** SSO changes publish together after saving, avoiding mixed settings during concurrent sign-ins. Session, settings and logout changes reject read-only HTTP methods.

## Other improvements

- **Tables behave predictably.** Selecting text no longer toggles rows, expanded details span visible columns, and workload selection no longer restores an obsolete scroll position.
- **Kiosk views show more monitoring.** Filter toolbars and section tabs are hidden, and table preview limits are lifted.
- **Controls are easier to read.** Light-mode banners have stronger contrast, disabled toggles show their actual state, and Manage explains each operator control in plain language.
- **Connections are easier to check.** Infrastructure entries open the platform showing that system's data. Proxmox, PBS and Mail Gateway connection tests use edited certificate settings with saved credentials before you save changes.
- **Logs stay responsive.** Slow viewers no longer block server logging, and the browser batches incoming lines. Pause, clear and download controls explain their effects.
- **Views can be bookmarked.** Patrol and Actions have dedicated routes, and links to Patrol's mode selector open the relevant control.
- **Troubleshooting gives safer next steps.** Guides cover guest backup safety, PBS permissions, Kubernetes credentials, OIDC, CORS and NAS setup, with bounded checks that keep credentials private.
- **Bug reports need less guesswork.** Guidance separates monitored guests from the Pulse server, explains damaged tokens without needless replacement, and makes specialist diagnostics optional.
- **Audit requests fail clearly.** Malformed export and summary time filters are rejected, while concurrent tenant incident records keep distinct identifiers.
- **The GitHub star prompt is less abrupt.** It appears on the first return day with clearer presentation.
- **Optional telemetry explains more usage.** Preview includes counts of successful extended-history reads and aggregate report outcomes and repeat use. Resource identities and readings stay excluded, and opt-out is respected.
- **Dependencies include security fixes.** Go, networking and frontend dependencies are updated. Production styling also works under Pulse's content security policy, and unsupported email reply-to and retry controls are removed.

## Known issues

- **Guest safety remains under investigation.** These changes do not confirm a fix for every guest freeze or Windows restart. If monitoring was stopped after a guest froze, do not restart it solely to test this beta or this release.
- **Affected backups still need precautions.** Stop Pulse monitoring before a planned backup. Keep it stopped until the backup ends and independent checks confirm thaw, fresh writes on every backed-up filesystem and workload liveness.
- **Elevated server memory remains unresolved.** Reduced allocation work does not confirm a lower long-running baseline. Report higher steady usage as well as continued growth, comparing existing readings at similar uptimes and workloads.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.5.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.5.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.5.0` and recreate the container.
