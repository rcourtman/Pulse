# Pulse v6.6.0-rc.1 Release Notes

This release candidate opens the `v6.6.0` candidate line after stable `v6.5.0`, which remains the rollback target.

Pulse v6.6.0 makes it easier to find problems and understand the readings behind them. Resource details bring alerts and charts together, backup coverage explains its limits, and retained data is clearly separated from current measurements. This release also improves disk tracking, Proxmox guest monitoring, notification editing, agent installation, updates and access controls.

## Highlights

- Find problems faster with alerts, charts and useful next steps together.
- Get clearer readings through backups, missing reports, disk replacements and guest moves.
- Install cluster agents and set up Patrol with fewer manual steps.

## What's improved

- **Charts beside the problem.** Resource details show alerts, recent trends and longer history on Overview, replacing the separate History tab for guests, nodes, agents, Docker hosts and containers.
- **Short spikes stay visible.** Charts show minimum and maximum readings behind average lines, helping distinguish brief peaks from sustained load.
- **History is easier to read.** Consistent range names, clearer axes, dated values and source labels explain each reading. Inspected values stay aligned through resizing, and gaps remain visible.
- **Charts show relevant alerts.** Resource history displays that resource's own alerts and trigger lines, without unrelated alerts from its parent.
- **Useful details come first.** Resource details lead with warnings and failed availability checks. Inventory sits under Details, and identifiers remain readable without repeated values.
- **Web links can be edited together.** Bulk-edit saved interfaces for workloads and Docker containers, then open a guest's saved interface from its header.
- **Guests keep their identity when moved.** Proxmox migrations preserve saved metadata, links, selection and history. Matching names or guest IDs at separate installations stay distinct.
- **Manual grouping stays yours.** Links, chained links and splits survive refreshes and identity changes. Unlinking separates rows, and reporting a bad merge removes only the responsible links.
- **Large dashboards do less repeated work.** Inventory updates reduce copying, grouping and sorting. Tables fetch only displayed history, and live updates preserve large numeric counters accurately.

## Alerts and notifications

- **Find the affected resource.** Alerts Overview leads with active problems. Guest and filesystem alerts open their guest, while Proxmox storage alerts open the affected pool.
- **See why an alert remains open.** Threshold alerts show the current reading, averaging window, clear level and recovery progress. The original breach value and date remain available.
- **Missing readings break confirmation periods.** Metric, storage, connection and disk-temperature timers restart after observation gaps instead of treating separated readings as continuous trouble.
- **Off stays off.** Disabled defaults remain disabled after saving, including Docker, Mail Gateway and backup limits. Low thresholds keep their clear level below their trigger, and fresh installs receive Docker defaults.
- **Settings saves preserve your edits.** Omitted settings and node overrides survive other saves. Drafts remain until confirmed, editing waits for saved policy to load, and threshold controls have screen-reader names.
- **Alerts survive reporting changes.** Agent handovers avoid duplicate node alerts without losing temperature coverage. Proxmox cleanup no longer removes alerts belonging to other platforms.
- **History explains closures.** Moved alerts are distinguished from recovered alerts in timelines, reports and notifications. Hosts, containers, services, disks, arrays and sensors retain the correct history owner.
- **History stays readable and bounded.** Messages wrap, frequency bars reflect severity, and acknowledgement changes sync between tabs. Closed alerts leave the active list, and history caches have memory limits.
- **Notification setup explains delivery.** Destinations lead the page, disabled services have compact summaries, and pause messages explain detection, snooze and activation. Routing help clarifies combined severity and tag filters.
- **Notification edits survive interruptions.** Drafts survive tab changes and uncertain saves. Webhook changes wait for confirmation, preserve explicit edits beside masked secrets, and explain partial failures.
- **Real webhook sends match their templates.** Built-in services receive required headers, including Telegram's JSON header. Clearing optional headers restores defaults, and copied examples safely escape alert text.
- **Email identifies the affected host.** Subjects include resource and host names, including non-ASCII names. Rate-limit changes persist, and unsupported reply-to and retry controls have been removed.

## Disks and storage

- **Capacity problems lead the storage view.** Tables emphasise filling pools and explain attention states. Collapsing groups no longer pins them, reopens them or scrolls back to an earlier pool.
- **Pool and disk charts sit with their details.** Storage history is available directly in the expanded resource, with collapsible charts and minimum-to-maximum bands.
- **Unrelated disks stay separate.** Matching serials on different hosts and generic QEMU serials no longer merge different hardware. Agent disks also stay out of unrelated cluster-member inventory.
- **Replacement disks start clean.** New hardware no longer inherits the old disk's SMART readings, slot identity, alert history or operator settings when its identity does not match.
- **Retained readings look retained.** Standby disks and silent agents show last-known temperatures, usage and RAID state. Merged disks keep each source's collection state.
- **Heat limits agree across views.** Alerts, colours, reports and Patrol use disk-type limits and host overrides. TrueNAS disk limits are editable, and Unraid uses the temperature reported by its agent.
- **Heat alerts close for the right reason.** Removing a disk or disabling its temperature threshold clears its alert. Missing reports preserve unresolved SMART problems without presenting old heat readings as current.
- **Risk labels explain the problem.** Heat alone reads Running Hot rather than Replace Now. SSD wear is labelled life remaining, and disk alerts are logged once.
- **History follows the hardware.** Controller-member and Unraid serial-based charts resolve correctly. Same-serial disks on different hosts retain separate charts.
- **Real local mounts remain visible.** Mounts beside system directories are no longer excluded merely because their paths resemble filtered locations.

## Proxmox, PBS and backups

- **Guest memory explains its limits.** Tables, details and alert settings distinguish current, retained and cache-inclusive readings. Percentages use the total measured with that reading, including ballooned VMs.
- **Windows collection avoids Linux-only requests.** Pulse learns the guest OS before optional Linux memory reads, preventing unsupported requests from blocking later filesystem collection.
- **Filesystem totals avoid false duplicates.** Shared filesystems count once regardless of mount order. Distinct Windows volumes remain separate even when equally sized, and invalid counters remain unknown.
- **Partial filesystem reads remain useful.** Valid rows survive incomplete responses without implying complete capacity or recovery. Stopped guests no longer publish retained filesystems as current measurements.
- **LXC collection works within the helper sandbox.** Running containers and mounted filesystems can be read without Proxmox or LXC control sockets. Unmounted paths no longer borrow parent filesystem usage.
- **Guest commands have stronger backup safeguards.** Reads and actions defer for locks, uncertain completion and malformed replies. Equivalent endpoints share coordination, and cancellation cannot prematurely free another command.
- **Slow polls retain valid inventory.** Completed inventory survives optional-detail deadlines, while retired connections cannot republish old readings. Temporary OS-read failures can recover, and concurrent polls share session renewal.
- **PBS coverage survives incomplete polls.** Missing namespace or snapshot results preserve known backups without marking them freshly checked. Root-namespace coverage includes LXC guests, and restored backup alerts are checked before startup delivery.
- **Coverage is easier to inspect.** Health counts filter matching workloads. Completed-backup details stay current in open views, distinguish freshness from verification, and restore the PBS web-interface shortcut.
- **Amber backup shields explain their limit.** Details name the configured age limit and link to Recovery thresholds. Saved changes update shields without reloading, making longer backup schedules easier to configure.
- **Replication shows observed results.** Status comes from source-node job observations. Slow reads do not pile up, failed reads do not erase jobs, and saved configuration alone no longer implies successful replication.
- **Node access messages give useful next steps.** Denied package-update checks explain their separate permissions without treating intentionally limited access as a fault. Outdated sensor notices appear across Proxmox tabs.

## TrueNAS, vSphere and Docker

- **TrueNAS CORE inventory tolerates unknown metadata.** Unknown or string-valued disk rotation rates no longer abort disk collection. Missing or conflicting pool counters remain unknown instead of looking empty.
- **TrueNAS charts and app readings recover.** Native disk-temperature history is available again, with a bounded fallback for empty reporting results. Fractional SCALE app CPU readings no longer interrupt reporting sessions.
- **TrueNAS explains unhealthy states.** Pools and topology give concrete reasons for attention. Services and VMs distinguish intentionally stopped items from those expected to run, with status text matching its indicator.
- **vSphere leads with health.** The overview highlights vCenter signals and explains datastore and network problems. Host groups use cluster names, and the vCenter column appears only when several are connected.
- **vSphere activity is easier to understand.** Events use vCenter's descriptions, VM records keep their correct platform, and state text uses the same colour as its status indicator.
- **Container health is visible and consistent.** Tables show health and uptime, details include health-check results, and status indicators reflect crash loops, health failures and missing state consistently.
- **Docker networks summarise attached containers.** Each network has one status indicator. Running counts use Docker's running state, while attached-container state text reflects its health.
- **Swarm rollouts use accurate names.** Rolling back, Update paused and Rollback paused are distinct. No running replicas takes priority, and task, replica and node status text reflects the underlying problem.
- **Image references remain readable.** Repository names, tags, update badges and lifecycle columns fit their tables more reliably.
- **Image checks preserve uncertainty.** Unknown local digests no longer clear update alerts. Explicit Docker Hub references reach the correct registry API, and unavailable checks explain their reason.
- **Updates are available beside the warning.** Container details offer Update and View image tags. Open review dialogs survive refreshes, and update plans retain the saved deployment target.
- **Podman CPU readings use the measured interval.** Reports, history and alerts agree on CPU usage. Small positive readings remain visible as fractions, with help explaining the percentage scale.

## Install, updates and agents

- **Cluster agents need fewer manual steps.** A connected Proxmox member can install agents on uncovered siblings using existing cluster SSH trust and individual bootstrap credentials. Progress confirms a live connection.
- **Enrollment completes before collection starts.** Agent modules use the exchanged runtime credential. Command channels register under Pulse's resolved identity and remain connected after restarts.
- **Collectors can reduce their authority.** Reporting agents can establish their identity without opening a command channel, allowing monitoring permissions to be reduced correctly.
- **Removal preserves recovery state.** Unix, Windows, Synology DSM and TrueNAS SCALE removal checks that services and boot tasks have stopped before deleting credentials or recovery files.
- **Saved credentials remain usable.** Installers respect selected token files and caller-owned files. Existing credentials survive missing saved server details, and copied removal commands use saved credentials.
- **Server updates prepare before stopping Pulse.** Replacement binaries are verified first, service shutdown is confirmed, and ambiguous service state stops replacement. Source-built installs also use atomic replacement.
- **Automatic updates respect existing choices.** Consent, update channels and systemd masks are read reliably. Unknown editions are excluded, and success requires the installed version and running executable to agree.
- **Update failures remain diagnosable.** Installer and log-collector results stay separate, including final output without a newline. Recovery material remains when completion cannot be confirmed.
- **Startup import protects existing installations.** Import requires an empty destination and checks the actual storage path before writing, including agent-only, TrueNAS and vSphere setups.
- **Agent setup gives clearer information.** First-run screens avoid claiming complete coverage prematurely. Update notices explain what changes, CPU counts are clearer, and Linux distribution names use OS metadata.
- **Empty service inventories are handled correctly.** No matching service files means no service was found, while genuine discovery failures remain visible.

## Pulse Pro, AI and hosted

- **Patrol setup lives on Patrol.** Connect a provider, choose a guided model, preview costs and check readiness from the page. Unreachable providers lead to Provider & Models.
- **Patrol starts with Ask first.** Pro installations require an explicit mode choice before using Safe auto-fix or Autopilot. Assistant offers Read-only and Ask first, with proposed actions saved for review in Actions.
- **Guest protection uses active controls.** The inactive Assistant Protected guests field is removed. Never auto-remediate in Manage blocks typed actions, including previously approved plans.
- **Protection survives regrouping.** Remediation blocks remain effective across links, splits and identity changes. Human-approved actions check current operator policy before dispatch.
- **Suppression rules can be removed.** Patrol Activity lists manually created rules with their scope and reason. Ending suppression leaves reviewed findings reviewed.
- **Maintenance silences the right findings.** Patrol honours recurring windows and maintenance covering child resources. Expected-offline settings silence availability findings while allowing other problems through.
- **Investigations target the correct resource.** Filesystem and disk alerts resolve to their actual owner. Repeated names or guest IDs across installations no longer select an arbitrary action target.
- **AI distinguishes missing data from recovery.** Retained readings cannot establish current health, cache-inclusive memory alone cannot prove pressure, and unavailable storage is distinguished from disabled storage.
- **Coverage warnings can clear.** A complete successful Patrol run removes an obsolete partial-coverage warning. Run ages and future schedule times update without refreshing.
- **AI Discovery has been removed.** Service context scans, the Discovery tab and automatic web-link and availability proposals are no longer offered.
- **Paid community installs can switch through the API.** Eligible systemd and Proxmox LXC installs can apply the Pro runtime in place. Docker installs receive image commands, and older Pro versions are refused.
- **Hosted portals start more reliably.** Initial setup can remain reachable without a licence, paid renewals survive restart, and tenant shutdown waits for active work and history writes.

## Monitoring and service health

- **Freshness follows actual polling.** Proxmox, PBS, Mail Gateway, connection summaries and resource APIs use the live fixed or adaptive schedule, avoiding false stale states between healthy polls.
- **Offline sources stay offline.** A recent report cannot override its source's offline verdict. Linked agents use their own heartbeat, and failed availability checks no longer keep nodes online.
- **Availability views agree.** Settings and Machines show consistent results. Old successful locations cannot look current because another location failed more recently, and licence changes restore local checks correctly.
- **Retained readings do not create new trends.** Old guest measurements cannot become fresh history points or bridge gaps. Memory and disk searches exclude unavailable measurements instead of treating them as zero.
- **Primary IP selection respects interfaces.** Normal guest interfaces take priority over container and overlay interfaces. Addresses remain associated with their interface, and all reported addresses stay available.
- **CPU temperatures agree across views.** Tables, details, history and alerts use the hottest current CPU reading. Node details show real low and record values, without mixing same-named nodes at different sites.
- **Ages keep moving.** Resource, replication, alert, delivery and watchdog ages update together. A stalled watchdog request no longer leaves Refresh waiting indefinitely.
- **Long outages avoid rapid retry loops.** Polling backoff retains its maximum delay through persistent failures.
- **Kubernetes problems stand out.** Not Ready nodes lead the overview, rows reflect their alerts, and status words match their indicators. Long names remain readable and copy without inserted breaks.
- **Mail Gateway alerts stay attached.** Rows and details show the gateway's own alerts and highlight the affected count. Saved collection opt-outs remain explicit.
- **Pulse health stays distinct from monitored systems.** Pulse service failures are separated from failed TrueNAS polling and other resource alerts.
- **Demo monitoring behaves consistently.** Mode changes update all tenant monitors and clear old sample alerts. Demo workloads have stable parents, distinct names and one history timeline per resource.

## Security

- **Tokens cannot expand their own permissions.** Callers cannot grant scopes they lack or turn narrower access into instance-wide credentials. Credential rotation requires appropriate instance-wide authority.
- **Credential management respects organizations.** Lists, diagnostics, editing, deletion and rotation expose only tokens the caller may manage within its organizations.
- **Changes require the right permissions.** Resource links and attention changes require monitoring:write. AI report narration requires ai:execute, and session or settings changes reject read-only HTTP methods.
- **Agent removal requires administrative access.** Browser callers need administrator rights to uninstall agents. Read-only tokens no longer see web-link editing controls.
- **SAML responses stay tied to their browser request.** Sign-in and logout replies must match the outstanding request. Replayed or altered responses are refused.
- **Saved certificate trust survives reconnection.** Proxmox recovery cannot replace an established certificate pin. Diagnostics distinguish timeouts from trust failures and inspect the configured endpoint.
- **Redirects keep credentials within their destination.** Infrastructure and notification requests refuse redirects to another origin. Configure the final notification URL directly when a receiver redirects elsewhere.
- **Webhook diagnostics disclose less.** Receiver response text and unsafe retry hints stay out of logs. Saved secret fields remain masked in API responses.
- **Demo restrictions remain enforced.** WebSocket headers cannot bypass blocked changes, and profiling endpoints are hidden on demo instances.
- **Security status reflects exposure.** Plain HTTP is critical when Pulse is publicly reachable. Private-network installations receive an HTTPS recommendation.
- **Dependencies include security fixes.** Go, networking and frontend dependencies are refreshed. Production styling also works under Pulse's content security policy.

## Other improvements

- **Tables behave more predictably.** Selecting text no longer toggles rows, expanded details span visible columns, and workload selection no longer restores an obsolete scroll position.
- **Kiosk views show more monitoring.** Filter toolbars and section tabs are hidden, and table preview limits are lifted.
- **Controls are easier to read.** Light-mode banners have stronger contrast, disabled toggles show their actual state, and Manage explains each operator control in plain language.
- **Connected systems lead to their data.** Infrastructure connection entries open the platform page showing that system's monitoring.
- **Logs stay responsive.** Slow viewers no longer block server logging, and the browser batches incoming lines. Pause, clear and download controls explain their effects.
- **Views can be bookmarked.** Patrol and Actions have dedicated routes, and links to Patrol's mode selector open the relevant control.
- **Troubleshooting gives safer next steps.** Guides cover guest backup safety, PBS permissions, Kubernetes credentials, OIDC, CORS and NAS setup, with bounded checks that keep credentials private.
- **Bug reports need less guesswork.** Guidance separates monitored guests from the Pulse server, explains damaged tokens without needless replacement, and makes specialist diagnostics optional.
- **Audit requests fail clearly.** Malformed export and summary time filters are rejected, while concurrent tenant incident records keep distinct identifiers.
- **The GitHub star prompt is less abrupt.** It appears on the first return day with clearer presentation.

## Known issues

- **Guest safety remains under investigation.** These changes do not confirm a fix for every reported guest freeze or Windows restart. If monitoring was stopped after a frozen guest, do not restart it solely to test this beta.
- **Elevated server memory remains unresolved.** Reduced allocation work does not confirm a lower long-running memory baseline. Check memory after upgrading and report higher steady usage as well as continued growth.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.5.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.5.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.5.0` and recreate the container.
