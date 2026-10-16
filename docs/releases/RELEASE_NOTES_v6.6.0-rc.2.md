# Pulse v6.6.0-rc.2 Release Notes

This release candidate follows `v6.6.0-rc.1` on the way to stable `v6.6.0`. Stable `v6.5.0` remains the rollback target.

Pulse v6.6.0 brings alerts, charts and useful next steps together, making it easier to understand what needs attention across your homelab. Readings are clearer when collection pauses, backups are incomplete or hardware changes. Agent setup, notifications, updates and access controls are also more reliable. Warnings about reported guest freezes, Windows restarts and elevated server memory remain relevant.

## Highlights

- Find problems faster with alerts, trends and useful next steps together.
- Understand backup coverage and distinguish current readings from last-known data.
- Install cluster agents and configure Patrol with fewer manual steps.

## What's improved

- **Charts sit beside the problem.** Resource details combine alerts, recent trends and longer history on Overview for guests, nodes, agents, Docker hosts and containers.
- **Brief spikes stay visible.** Charts show minimum and maximum readings around average lines, helping distinguish short peaks from sustained load.
- **History is easier to follow.** Consistent ranges, clearer axes, dated values and source labels explain readings. Inspected values survive resizing, gaps remain visible, and dense histories retain the newest observations.
- **Charts show relevant alerts.** Resource history includes its own alerts and trigger lines without unrelated alerts inherited from its parent.
- **Useful details come first.** Resource details lead with warnings and failed availability checks. Inventory sits under Details, with readable identifiers and fewer repeated values.
- **Web links can be edited together.** Bulk-edit saved interfaces for workloads and Docker containers, then open a guest's saved interface from its header.
- **Guests keep their identity when moved.** Proxmox migrations preserve metadata, links, selection and history. Matching names or guest IDs at separate installations stay distinct.
- **Manual grouping stays yours.** Links, chained links and splits survive refreshes and identity changes. Unlinking separates rows, while reporting a bad merge removes only the responsible links.
- **Large dashboards do less repeated work.** Inventory updates reduce copying, grouping and sorting. Tables fetch only displayed history, hidden charts pause collection, and live updates preserve large counters accurately.

## Alerts and notifications

- **Alerts open the right place.** Overview prioritises active problems. Guest and filesystem alerts open their guest, storage alerts open their pool, and notification links open the specific alert's timeline when a public URL is configured.
- **Open alerts explain their state.** Threshold alerts show the current reading, averaging window, clear level and recovery progress while preserving the original breach value and date.
- **Missing readings interrupt confirmation.** Metric, storage, connection and disk-temperature timers restart after observation gaps instead of treating separated readings as continuous trouble.
- **Off stays off.** Disabled Docker, Mail Gateway and backup defaults remain disabled after saving. Low thresholds keep their clear level below the trigger, and fresh installations receive Docker defaults.
- **Settings preserve your edits.** Omitted settings and node overrides survive other saves. Drafts remain until confirmed, editing waits for saved policy to load, and threshold controls have accessible names.
- **Reporting changes preserve coverage.** Agent handovers avoid duplicate node alerts without losing temperature monitoring. Proxmox cleanup preserves alerts belonging to other platforms.
- **History explains closures.** Moved alerts remain distinct from recovered alerts in timelines, reports and notifications. Host, container, service, disk, array and sensor history stays with its resource.
- **History is clearer and bounded.** Messages use available space, narrow panels show cards, and frequency bars use severity colours and local times. Acknowledgements sync between tabs, read failures stay visible, and caches have limits.
- **Delivery settings explain their effect.** Destinations come first, disabled services have compact summaries, and pause messages distinguish detection from delivery. Routing help explains combined severity and tag filters.
- **Notification drafts survive interruptions.** Tab changes and uncertain saves preserve edits. Webhook changes wait for confirmation, explicit edits survive masked secrets, and partial failures explain what happened.
- **Webhook sends match their templates.** Built-in services receive required headers, including Telegram's JSON header. Clearing optional headers restores defaults, and copied examples escape alert text safely.
- **Email identifies the affected host.** Subjects include resource and host names, including non-ASCII names. Rate-limit changes persist, and unsupported reply-to and retry controls are removed.

## Disks and storage

- **Capacity problems lead the storage view.** Tables emphasise filling pools and explain attention states. Collapsing groups no longer pins them, reopens them or scrolls back to an earlier pool.
- **Storage history stays with its resource.** Expanded pools and disks include collapsible history charts with minimum-to-maximum bands.
- **Unrelated disks stay separate.** Matching serials on different hosts and generic QEMU serials no longer merge different hardware. Agent disks stay out of unrelated cluster-member inventory.
- **Replacement disks start clean.** New hardware no longer inherits the old disk's SMART readings, slot identity, alert history or operator settings when its identity differs.
- **Retained readings look retained.** Standby disks and silent agents show last-known temperatures, usage and RAID state. Merged disks preserve each source's collection state.
- **Heat limits agree across views.** Alerts, colours, reports and Patrol use disk-type limits and host overrides. TrueNAS disk limits are editable, and Unraid uses its agent's reported temperature.
- **Heat alerts close for the right reason.** Removing a disk or disabling its temperature threshold clears its alert. Missing reports preserve unresolved SMART problems without presenting old temperatures as current.
- **Risk labels explain the problem.** Heat alone reads Running Hot rather than Replace Now. SSD wear is labelled life remaining, and disk alerts are logged once.
- **History follows the hardware.** Controller-member and Unraid serial-based charts resolve correctly. Same-serial disks on different hosts retain separate charts.
- **Real local mounts remain visible.** Mounts beside system directories are no longer excluded just because their paths resemble filtered locations.

## Proxmox, PBS and backups

- **Guest memory explains its limits.** Tables, details and alert settings distinguish current, retained and cache-inclusive readings. Percentages use the capacity measured with that reading, including ballooned VMs.
- **Windows avoids unsuitable memory requests.** Pulse learns the guest OS before optional Linux memory reads, preventing unsupported requests from blocking later filesystem collection.
- **Filesystem totals avoid false duplicates.** Shared filesystems count once regardless of mount order. Distinct Windows volumes remain separate even when equally sized, and invalid counters remain unknown.
- **Partial filesystem reads remain useful.** Valid rows survive incomplete responses without implying complete capacity or recovery. Stopped guests no longer publish retained filesystems as current measurements.
- **LXC collection works within the helper sandbox.** Running containers and mounted filesystems can be read without Proxmox or LXC control sockets. Unmounted paths no longer borrow parent filesystem usage.
- **Guest commands respect more backup safeguards.** Reads and actions defer for locks, uncertain completion and malformed replies. Equivalent endpoints share coordination, and cancellation cannot prematurely release another command.
- **Slow polls retain valid inventory.** Completed inventory survives optional-detail deadlines. Retired connections cannot republish old readings, temporary OS-read failures can recover, and concurrent polls share session renewal.
- **PBS coverage survives incomplete polls.** Missing namespace or snapshot results preserve known backups without marking them freshly checked. Root-namespace coverage includes LXC guests, and restored backup alerts are checked before startup delivery.
- **Backup coverage is easier to inspect.** Health counts filter matching workloads. Open details track completed backups, distinguish freshness from verification and restore the PBS interface shortcut. Each PBS server shows host statistics once.
- **Amber shields explain their limit.** Backup details name the configured age limit and link to Recovery thresholds. Saved changes update shields immediately, making longer backup schedules easier to configure.
- **Replication shows observed results.** Status comes from source-node job observations. Slow reads do not pile up, failed reads preserve jobs, and saved configuration alone no longer implies successful replication.
- **Access messages give useful next steps.** Denied package-update checks explain their separate permissions without treating limited access as a node fault. Outdated sensor notices appear across Proxmox tabs.

## TrueNAS, vSphere and Docker

- **TrueNAS CORE tolerates unknown disk metadata.** Unknown or string-valued rotation rates no longer abort inventory. Missing or conflicting pool counters remain unknown instead of looking empty.
- **TrueNAS charts and app readings recover.** Native disk-temperature history works again, with a bounded fallback for empty reporting results. Fractional SCALE app CPU readings no longer interrupt reporting sessions.
- **TrueNAS explains unhealthy states.** Pools and topology give concrete reasons for attention. Services and VMs distinguish intentionally stopped items from those expected to run, with status text matching its indicator.
- **vSphere leads with health.** The overview highlights vCenter signals and explains datastore and network problems. Host groups use cluster names, and the vCenter column appears only when several are connected.
- **vSphere activity reads clearly.** Events use vCenter's descriptions, VM records retain the correct platform, and state text matches its status indicator's colour.
- **Container health is consistent.** Tables show health and uptime, details include health-check results, and indicators reflect crash loops, failed health checks and missing state.
- **Docker networks summarise their containers.** Each network has one status indicator. Running counts use Docker's running state, while attached-container state text reflects health.
- **Swarm rollouts use accurate names.** Rolling back, Update paused and Rollback paused are distinct. Missing running replicas takes priority, and task, replica and node status text reflects the problem.
- **Image checks explain uncertainty.** Unknown local digests no longer clear update alerts. Explicit Docker Hub references reach the correct registry, unavailable checks explain why, and repository names and tags remain readable.
- **Updates sit beside the warning.** Container details offer Update and View image tags. Review dialogs survive refreshes, and update plans retain the saved deployment target.
- **Uncertain restarts get read back.** After a missing Docker restart response, Pulse checks the container within the original deadline without repeating the restart. Observed health stays distinct from confirmed execution.
- **Podman CPU uses the measured interval.** Reports, history and alerts agree on usage. Small positive readings remain visible as fractions, with help explaining the percentage scale.

## Install, updates and agents

- **Cluster agents need fewer manual steps.** A connected Proxmox member can install agents on uncovered siblings using existing cluster SSH trust and individual bootstrap credentials. Progress confirms a live connection.
- **Enrollment finishes before collection starts.** Agent modules use the exchanged runtime credential. Command channels register under Pulse's resolved identity and remain connected after restarts.
- **Monitoring agents can reduce permissions.** Reporting agents can establish their identity without opening a command channel, allowing monitoring access to be reduced correctly.
- **Docker commands reach their host.** Container and other Docker child identities no longer make a unique reporting host ambiguous. Genuine collisions are reported clearly instead of appearing as a missing agent connection.
- **Removal preserves recovery files.** Unix, Windows, Synology DSM and TrueNAS SCALE removal confirms services and boot tasks have stopped before deleting credentials or recovery state.
- **Saved credentials remain usable.** Installers respect selected token files and caller-owned files. Existing credentials survive missing saved server details, and copied removal commands use saved credentials.
- **Server updates prepare before stopping Pulse.** Replacement binaries are verified first, shutdown is confirmed, and ambiguous service state stops replacement. Source-built installations also use atomic replacement.
- **Automatic updates respect saved choices.** Consent, channels and systemd masks are read reliably. Unknown editions are excluded, and success requires the installed version and running executable to agree.
- **Update failures remain diagnosable.** Installer and log-collector results stay separate, including final output without a newline. Recovery material remains when completion cannot be confirmed.
- **Startup import protects existing installations.** Import requires an empty destination and checks the actual storage path before writing, including agent-only, TrueNAS and vSphere setups.
- **Setup gives clearer information.** Connection forms ask for the address before its name. First-run coverage, CPU counts, Linux distribution names and agent update notices are clearer. Usage-statistics opt-out no longer reports a false failure.
- **Empty service inventories are handled correctly.** No matching service files means no service was found, while genuine discovery failures remain visible.

## Pulse Pro, AI and hosted

- **Patrol setup lives on Patrol.** Connect a provider, choose a guided model, preview costs and check readiness from the page. Unreachable providers lead to Provider & Models.
- **Actions start with review.** Pro Patrol defaults to Ask first until a mode is chosen. Safe auto-fix and Autopilot remain available. Assistant offers Read-only and Ask first, with proposed actions saved in Actions.
- **Guest protection stays effective.** Never auto-remediate blocks typed actions and previously approved plans across links, splits and identity changes. Approved actions check current policy. The inactive Assistant Protected guests field is removed.
- **Suppression rules can be removed.** Patrol Activity lists manually created rules with their scope and reason. Ending suppression leaves reviewed findings reviewed.
- **Maintenance silences the right findings.** Patrol honours recurring windows and maintenance covering child resources. Expected-offline settings silence availability findings while allowing other problems through.
- **Investigations target the right resource.** Filesystem and disk findings resolve to their owner. Repeated names stay distinct, while decision, action-review and threshold links open the relevant resource or settings row.
- **Investigations use collected backup data.** AI reads backup inventory, attempts, errors, verification details and storage history from the same sources as Pulse's monitoring views.
- **Missing data stays uncertain.** Retained readings cannot establish current health, cache-inclusive memory alone cannot prove pressure, and unavailable storage remains distinct from disabled storage.
- **Patrol keeps checking after restarts.** Saved schedules resume their remaining wait, overdue checks run once, and complete runs clear obsolete coverage warnings. Run ages and future schedule times update automatically.
- **Recurring problems get fresh investigation.** Historical fixes cannot close a newly recurring finding. Inconclusive action results leave unresolved findings available for follow-up after Watch completes.
- **AI Discovery is removed.** Service context scans, the Discovery tab and automatic web-link and availability proposals are no longer offered.
- **Paid and hosted installs recover better.** Eligible systemd and Proxmox LXC installs can switch to Pro through the API. Docker receives image commands. Hosted setup works without a licence, renewals survive restart, and shutdown waits for active work.

## Monitoring and service health

- **Freshness follows actual polling.** Proxmox, PBS, Mail Gateway, connection summaries and resource APIs use the live fixed or adaptive schedule, avoiding false stale states between healthy polls.
- **Silent agents get accurate wording.** Pulse says an agent stopped reporting without claiming its host is down. Source offline states remain effective, linked agents use their own heartbeat, and failed availability checks cannot keep nodes online.
- **Availability views agree.** Settings and Machines show consistent results. Old successful locations cannot look current because another location failed more recently, and licence changes restore local checks correctly.
- **Retained readings cannot create fresh trends.** Old guest measurements no longer become new history points or bridge gaps. Memory and disk searches exclude unavailable measurements instead of treating them as zero.
- **Primary IP selection respects interfaces.** Normal guest interfaces take priority over container and overlay interfaces. Addresses stay associated with their interface, and all reported addresses remain available.
- **CPU temperatures agree across views.** Tables, details, history and alerts use the hottest current CPU reading. Node details show real low and record values without mixing same-named nodes at different sites.
- **Ages keep moving.** Resource, replication, alert, delivery and watchdog ages update together. A stalled watchdog request no longer leaves Refresh waiting indefinitely.
- **Long outages avoid rapid retry loops.** Polling backoff retains its maximum delay through persistent failures.
- **Kubernetes problems stand out.** Not Ready nodes lead the overview, rows reflect their alerts, and status words match their indicators. Long names remain readable and copy without inserted breaks.
- **Mail Gateway 9 collection works again.** String-valued cluster IDs no longer break collection or trigger guessed backup requests. Gateway alerts stay attached to the right row, affected counts stand out, and saved collection opt-outs remain explicit.
- **Pulse health stays distinct.** Pulse service failures are separated from failed TrueNAS polling and other monitored-resource alerts.
- **Demo monitoring behaves consistently.** Mode changes update all tenant monitors and clear old sample alerts. Demo workloads have stable parents, distinct names and one history timeline per resource.

## Security

- **Tokens cannot expand their permissions.** Callers cannot grant scopes they lack or turn narrower access into instance-wide credentials. Credential rotation requires suitable instance-wide authority.
- **Credential management respects organizations.** Lists, diagnostics, editing, deletion and rotation expose only tokens the caller may manage within its organizations.
- **Changes require the right permissions.** Resource links and attention changes require monitoring:write. AI report narration requires ai:execute, and session or settings changes reject read-only HTTP methods.
- **Agent removal requires administrative access.** Browser callers need administrator rights to uninstall agents. Read-only tokens no longer see web-link editing controls.
- **SAML replies stay tied to their request.** Sign-in and logout responses must match the outstanding browser request. Replayed or altered responses are refused.
- **Certificate trust survives reconnection.** Proxmox recovery cannot replace an established certificate pin. Diagnostics distinguish timeouts from trust failures and inspect the configured endpoint.
- **Redirects keep credentials within their destination.** Infrastructure and notification requests refuse redirects to another origin. Configure the final notification URL directly when a receiver redirects elsewhere.
- **Webhook diagnostics disclose less.** Receiver response text and unsafe retry hints stay out of logs. Saved secret fields remain masked in API responses.
- **Demo restrictions remain enforced.** WebSocket headers cannot bypass blocked changes, and profiling endpoints are hidden on demo instances.
- **Security status reflects exposure.** Plain HTTP is critical when Pulse is publicly reachable. Private-network installations receive an HTTPS recommendation.
- **Dependencies include security fixes.** Go, networking and frontend dependencies are refreshed. Production styling also works under Pulse's content security policy.

## Other improvements

- **Tables behave predictably.** Selecting text no longer toggles rows, expanded details span visible columns, and workload selection no longer restores an obsolete scroll position.
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
- **Elevated server memory remains unresolved.** Reduced allocation work does not confirm a lower long-running baseline. Check memory after upgrading and report higher steady usage as well as continued growth.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.5.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.5.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.5.0` and recreate the container.
