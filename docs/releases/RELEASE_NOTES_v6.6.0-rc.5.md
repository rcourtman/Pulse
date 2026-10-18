# Pulse v6.6.0-rc.5 Release Notes

This release candidate follows `v6.6.0-rc.4` on the way to stable `v6.6.0`. Stable `v6.5.0` remains the rollback target.

Pulse v6.6.0 puts alerts, trends and useful next steps together so you can see what needs attention across your homelab. Backup coverage, disk health and guest readings explain when information is incomplete or old. Cluster agent setup, scheduled reports, notifications and updates need less manual work. Guest freezes, Windows restarts and elevated server memory remain under investigation.

## Highlights

- See problems, recent trends and longer history together in resource details.
- Automate inventory exports and backup health reports with clearer delivery results.
- Keep useful monitoring through missing readings, hardware replacements and guest moves.

## What's improved

- **Charts sit beside the problem.** Resource Overview combines alerts, 24-hour trends and longer history for guests, nodes, agents, Docker hosts and containers. It replaces the separate History tab.
- **Brief spikes stay visible.** Charts show minimum and maximum readings around average lines, helping distinguish short peaks from sustained load.
- **History is easier to follow.** Consistent ranges, clearer axes, dated values and source labels explain readings. Inspected values survive resizing, gaps stay visible, and large histories retain the newest observations.
- **Charts show relevant alerts.** History includes the resource's own alerts and trigger lines without unrelated parent alerts. Row history also charts network and disk I/O.
- **Useful details come first.** Resource details lead with warnings and failed availability checks. Details shows complete identifiers, interfaces, operating systems and agent information with less repetition.
- **Web links can be edited together.** Bulk-edit saved interfaces for workloads and Docker containers, then open a guest's saved interface from its header.
- **Guests keep their identity when moved.** Proxmox migrations preserve metadata, links, selection and history. Matching names or guest IDs at separate installations stay distinct.
- **Manual grouping stays yours.** Links, chained links and splits survive refreshes and identity changes. Unlinking separates rows, while reporting a bad merge removes only the responsible links.
- **Large dashboards do less repeated work.** Monitoring reduces inventory copying, grouping and sorting. Tables fetch displayed history, hidden charts pause collection, and live updates preserve large counters accurately.
- **VM inventory exports can run on a schedule.** Export the organization's current VM inventory as CSV weekly or monthly, including newly discovered VMs. Email it or save it to disk.
- **Client backup health gets a report.** Compatible Pro runtimes offer weekly or monthly PDF and CSV summaries of Proxmox backup attempts, including jobs writing to PBS. Failures, missing attempts and incomplete history stay explicit.
- **Report delivery is easier to recover.** Settings shows readiness and results. Temporary email failures retry within limits, retries retain the report period, and unconfirmed sends warn about duplicates. Schedules survive monitor reloads.

## Alerts and notifications

- **Alerts lead to the problem.** Overview prioritises active trouble. Guest, filesystem and storage alerts open the affected guest or pool. Notifications open the alert timeline when a public URL is configured.
- **Open alerts explain their state.** Threshold alerts show the current reading, averaging window, clear level and recovery progress while preserving the original breach value and date. Missing readings restart confirmation timers.
- **Saved limits stay consistent.** Off remains off for Docker, Mail Gateway and backup defaults. Low limits keep recovery below the trigger. Colours and save-time checks use the same filesystem, node and pulse-relaxed limits as live alerts.
- **Settings preserve your edits.** Other saves retain omitted settings and node overrides. Drafts wait for confirmation, editing waits for saved policy, defaults come first, and threshold controls have accessible names.
- **Agent handovers preserve coverage.** Linked agents avoid duplicate node alerts without losing temperature monitoring. Cleanup preserves other platforms' alerts. Timelines and reports distinguish moved alerts from recovered ones.
- **History is clearer and bounded.** Alerts stay with their host, container, service, disk, array or sensor. Messages wrap, frequency bars show severity and local time, acknowledgements sync across tabs, and read failures stay visible.
- **Delivery controls explain their effect.** Destinations come first. Delivery Schedule explains timing, routing help explains combined filters, and pause messages distinguish detection from delivery. Audit Webhooks points to alert destinations.
- **Notification drafts survive interruptions.** Tab changes and uncertain saves preserve edits. Webhook changes wait for confirmation, explicit edits survive masked secrets, and partial failures explain what happened.
- **Webhook sends match their templates.** Required headers fix real Telegram alerts when tests already worked. Cleared headers restore defaults, examples escape text safely, and invalid templates fail clearly without pointless retries.
- **SMTP works with more providers.** Default usernames enable authentication for SendGrid, Postmark, SparkPost and Resend. LOGIN handles varied prompts, authentication choices match server support, and port 465 tests use the delivery TLS mode.
- **Email keeps the right recipients.** Concurrent sends keep separate senders and recipients. Subjects support non-ASCII host names. Rate limits persist and can be edited while off. Unsupported reply-to and retry controls are removed.
- **Delivery failures are more accurate.** Alternate approved addresses get a chance when one stalls. Temporary disconnects can retry, invalid settings stop clearly, and interrupted Apprise responses no longer count as successful delivery.

## Disks and storage

- **Capacity problems lead the storage view.** Tables emphasise filling pools and explain attention states. Collapsing groups no longer pins them, reopens them or scrolls back to an earlier pool.
- **Storage history stays beside the reading.** Expanded pools and disks include collapsible history charts with minimum-to-maximum bands.
- **Unrelated disks stay separate.** Matching serials on different hosts and generic QEMU serials no longer merge different hardware. Agent disks stay out of unrelated cluster-member inventory.
- **Hardware matching tolerates different ID formats.** Equivalent WWNs from Proxmox and agents match correctly, including controller disks. Empty or placeholder identifiers cannot establish a match.
- **Replacement disks start clean.** Different hardware no longer inherits the old disk's SMART readings, slot identity, alert history or operator settings.
- **Retained readings look retained.** Standby disks and silent agents show last-known temperatures, usage and RAID state. Merged disks preserve each source's collection state.
- **Heat limits agree across views.** Alerts, colours, reports and Patrol use disk-type limits and host overrides. TrueNAS limits are editable, and Unraid uses its agent's reported temperature.
- **Heat alerts close for the right reason.** Removing a disk or disabling its temperature threshold clears its alert. Missing reports preserve unresolved SMART problems without presenting old temperatures as current.
- **Risk labels explain the problem.** Heat alone reads Running Hot rather than Replace Now. SSD wear is labelled life remaining, and disk alerts are logged once.
- **History follows the hardware.** Controller-member and Unraid serial-based charts resolve correctly. Same-serial disks on different hosts retain separate charts.
- **Disk usage stays measurable.** Capacity alone no longer looks like observed usage. Fractional percentages, zero usage and valid byte totals display correctly, while invalid counters remain unknown in bars, searches and details.
- **Storage guidance explains missing data.** RAID help distinguishes spares, empty Unraid slots and missing members. Real mounts such as /snapshots and /runtime remain visible instead of matching overly broad system-directory filters.

## Proxmox, PBS and backups

- **Guest memory explains its limits.** Views and alerts distinguish current, retained and cache-inclusive VM readings. Percentages use sampled capacity, including ballooned VMs. LXC listing memory is treated as cache-adjusted usage.
- **Windows avoids unsuitable memory requests.** Pulse learns the guest OS before optional Linux memory reads and collects filesystems first, preventing unsupported requests from blocking later disk readings.
- **Filesystem totals avoid false duplicates.** Shared filesystems count once regardless of mount order. Distinct Windows volumes stay separate even when equally sized. Valid rows survive partial replies without implying complete capacity.
- **LXC collection works within the helper sandbox.** Running containers and mounted filesystems can be read without Proxmox control sockets. Unmounted paths cannot borrow parent usage, and an empty container list stays empty.
- **Guest commands respect backup safeguards.** Reads and actions defer for locks, uncertain completion and malformed or mismatched replies. Equivalent endpoints share coordination. Cancelling a request cannot prematurely release another command.
- **Incomplete polls retain known guests.** Failed node enumeration and malformed inventory cannot erase accepted guests or renew old readings. Completed inventory survives optional-detail deadlines, and retired connections cannot republish it.
- **Proxmox connections recover more reliably.** Password sessions and node-list reads must succeed before recovery is reported. Shared login renewal survives one caller cancelling, and startup failures keep retrying beyond five minutes.
- **PBS reads survive interruptions.** Concurrent reads share login renewal. Failed namespace or snapshot reads retain known backups without marking them freshly checked. Root-namespace coverage includes LXC guests.
- **Backup screens survive partial failures.** Known PBS servers remain visible when backup inventory fails. Each source explains loading or failure and can retry separately. Counts stay explicit about incomplete results.
- **Backup coverage is easier to inspect.** Health counts filter workloads. Details distinguish freshness from verification and restore the PBS shortcut. Completed backups show one UTC completion time, and each PBS server shows host statistics once.
- **Backup limits explain amber shields.** Details name the configured age limit and link to Recovery thresholds. Saved changes update shields immediately. Restored backup alerts are checked before startup delivery.
- **Replication and access stay honest.** Replication uses observed source-node results, retains jobs after failed reads and avoids overlapping requests. Denied update checks explain permissions, and outdated sensor notices appear across Proxmox tabs.

## TrueNAS, vSphere and Docker

- **TrueNAS CORE keeps usable inventory.** Unknown or string-valued disk rotation rates no longer abort collection. Pool capacity can use top-level vdev statistics when flat fields are absent. Missing or conflicting values remain unknown.
- **TrueNAS charts and app readings recover.** Native disk-temperature history works again, including disks without usable serials. Empty reporting results have a bounded fallback, and fractional SCALE app CPU no longer breaks reporting sessions.
- **TrueNAS explains unhealthy states.** Pools and topology name concrete problems. Services and VMs distinguish intentionally stopped items from those expected to run, with status text matching its indicator.
- **vSphere leads with health.** Overview highlights vCenter signals and explains datastore and network problems. Groups use cluster names, activity uses vCenter descriptions, and the vCenter column appears only with several connections.
- **vSphere records keep their platform.** VM details identify vSphere correctly, and state text matches the status indicator's colour.
- **Container health is consistent.** Tables show health and uptime, details include health-check results, and indicators reflect crash loops, failed checks and missing state.
- **Docker networks summarise their containers.** Each network has one status indicator. Running counts use Docker's running state, while attached-container status reflects health.
- **Swarm rollout states are distinct.** Active and paused updates and reversions have separate labels. Missing running replicas takes priority, and task, replica and node status text reflects the problem.
- **Image checks preserve update warnings.** Unknown local digests and daily cleanup no longer clear warnings incorrectly. Explicit Docker Hub references reach the right registry, unavailable checks explain why, and image names stay readable.
- **Updates sit beside the warning.** Container details offer Update and View image tags. Review dialogs survive refreshes, and plans retain the saved deployment target.
- **Uncertain restarts get checked.** After a missing Docker restart response, Pulse reads container state within the original deadline without repeating the restart. Observed health stays distinct from confirmed execution.
- **Container CPU stays consistent.** Podman uses the measured interval across reports, history and alerts. Small positive readings stay visible as fractions, with help explaining the percentage scale.

## Install, updates and agents

- **Cluster agents need fewer manual steps.** A connected Proxmox member can install agents on uncovered siblings using existing cluster SSH trust and individual bootstrap credentials. Progress confirms a live connection.
- **Enrollment finishes before collection starts.** Non-Windows modules use the exchanged runtime credential from first start. Command channels use Pulse's resolved identity and stay connected after restarts.
- **Monitoring agents can reduce permissions.** Reporting agents establish identity without a command channel. Action-runner setup recognises restored certificate trust while keeping command connections verified.
- **Docker commands reach their host.** Container and other Docker child identities no longer make a unique reporting host ambiguous. Real collisions get a clear error instead of appearing as a missing agent connection.
- **Removal preserves recovery files.** Unix, Windows and NAS removal waits for services, boot tasks and watchdogs to stop before deleting credentials or recovery state. Runner-only removal keeps its separate scope.
- **Agent setup protects credentials.** Installers reject unsafe paths and incomplete downloads, respect selected token files and retain credentials when server details are missing. Copied install input stays private on BusyBox hosts.
- **Server updates prepare before stopping Pulse.** Replacement binaries are checked first, uncertain service state stops replacement, and archives stage on the install filesystem to work with a separate /tmp.
- **Updates respect saved choices.** Manual and automatic updates use saved channels. Automatic updates honour consent and systemd masks, exclude unknown editions, and check that the running executable matches the installed version.
- **Update failures remain diagnosable.** Installer and log-collector results stay separate, including output without a final newline. Recovery files remain when completion is uncertain, and source-built installs use atomic replacement.
- **NAS agent updates stay executable.** Unraid and QNAP persistent copies retain executable permissions. QNAP avoids replacing the installed binary with a second copy of itself.
- **Imports and resets protect existing installs.** Startup import checks the actual destination is empty, including agent-only, TrueNAS and vSphere setups. Reset and removal wait for affected services to stop.
- **Setup explains more clearly.** Forms ask for the address first. Coverage, CPU counts, Linux distribution names and agent notices are clearer. Statistics opt-out no longer reports false failure, and an empty service inventory is handled correctly.

## Pulse Pro, AI and hosted

- **Patrol setup lives on Patrol.** Connect a provider, choose a guided model, preview costs and check readiness there. Unreachable providers lead to Provider & Models.
- **Actions start with review.** Pro Patrol defaults to Ask first until a mode is chosen. Safe auto-fix and Autopilot remain available. Assistant offers Read-only and Ask first, saving proposed actions in Actions.
- **Guest protection stays effective.** Never auto-remediate blocks actions and approved plans across links, splits and identity changes. Actions check current policy before running. The inactive Assistant Protected guests setting is removed.
- **Suppression rules can be removed.** Patrol Activity lists manually created rules with their scope and reason. Ending suppression leaves reviewed findings reviewed.
- **Maintenance silences the right findings.** Patrol honours recurring windows and maintenance covering child resources. Expected-offline settings silence availability findings while allowing other problems through.
- **Investigations find the right resource.** Filesystem and disk findings resolve to their owner. Repeated names stay distinct. Decision, action-review and threshold links open the relevant resource or settings row.
- **Investigations use available evidence.** AI reads backup inventory, attempts, errors, verification and storage history. Old readings cannot prove health, cache-inclusive VM memory cannot prove pressure, and unavailable storage stays explicit.
- **Patrol resumes its schedule after restart.** Saved schedules retain the remaining wait, overdue checks run once, and complete runs clear obsolete coverage warnings. Historical fixes cannot close recurring findings.
- **Incomplete investigations keep a next step.** Empty replies, summary timeouts, tool calls printed as prose and uncertain actions remain eligible for scheduled follow-up within policy limits. Interactive chat preserves its streamed summary.
- **AI Discovery is removed.** Service context scans, the Discovery tab and automatic web-link and availability proposals are no longer offered.
- **Paid installs can switch to Pro in place.** Eligible systemd and Proxmox LXC installs switch through the API. Docker gets image commands. Older Pro versions are refused, and stable-channel installs cannot switch to a prerelease.
- **Hosted installs recover better.** Setup stays reachable without a licence, renewals survive restart, and tenant deletion and shutdown wait for monitoring, alert checks and history writes.

## Monitoring and service health

- **Freshness follows actual polling.** Proxmox, PBS, Mail Gateway, connections and resource APIs use the live fixed or adaptive schedule. Connection alerts continue after monitor reloads.
- **Silent agents get accurate wording.** Pulse says an agent stopped reporting without claiming its host is down. Source offline states remain effective, linked agents use their own heartbeat, and failed checks cannot keep nodes online.
- **Availability views agree.** Settings and Machines show consistent results. Old successful locations cannot look current because another location failed more recently. Licence changes restore local checks correctly.
- **Old readings cannot create fresh trends.** Retained memory and disk values expire from their original observation time, stay out of new history points and alert decisions, and leave visible gaps. Stopped guests cannot publish current filesystems.
- **Unknown measurements stay unknown.** Silent in-guest agents, rejected memory replies and incomplete filesystem reads cannot create current usage or false recovery. Memory searches exclude unavailable readings instead of treating them as zero.
- **Reading explanations are easier to reach.** Memory, disk and backup notices can be opened with keyboard or touch. Details explain deferred collection and missing measurements without requiring hover.
- **IP addresses keep their interfaces.** Normal guest interfaces take priority over container and overlay interfaces. Workloads retains interface names and all reported addresses. Distinct guest domain names no longer cause false agent matches.
- **CPU temperatures agree across views.** Tables, details, history and alerts use the hottest current CPU reading. Node details show actual low and record values without mixing same-named nodes at different sites.
- **Ages keep moving.** Resource, backup, replication, alert, delivery, Patrol, token, installer and diagnostic ages update without refreshing. Stalled watchdog requests no longer leave Refresh waiting indefinitely.
- **Kubernetes problems stand out.** Not Ready nodes lead Overview, rows reflect their alerts, and status words match their indicators. Long names remain readable and copy without inserted breaks.
- **Mail Gateway 9 collection works again.** String-valued cluster IDs no longer break collection or trigger guessed backup requests. Failed reads retain known backups, alerts stay with their gateway, and collection opt-outs remain explicit.
- **Service health stays meaningful.** Pulse failures stay separate from monitored-resource failures. Guest-agent fallback needs actual replies. Persistent outages retain maximum retry delays, and demo mode updates all tenants and clears old sample alerts.

## Security

- **Tokens stay within their authority.** Callers cannot grant scopes they lack or create instance-wide credentials from limited access. Token lists, diagnostics, editing, deletion and rotation respect organizations and permissions.
- **Instance administration stays with instance admins.** Organization owners and organization-bound tokens cannot change global credentials, security, licensing or updates. They retain their own organization's administration.
- **Organizations keep separate data.** Live events, reports and AI summaries stay with their organization. Missing tenant monitors cannot expose default-organization data, and custom metrics database paths apply only to the default organization.
- **Changes require the right permissions.** Links and attention changes require monitoring:write, AI report narration requires ai:execute, and browser agent removal requires admin access. Read-only tokens no longer see link editors.
- **Assistant checks command arguments.** Read-only decisions inspect options and every pipeline stage so familiar command names cannot disguise file writes, service changes or other actions requiring approval.
- **SAML replies stay tied to their request.** Sign-in and logout responses must match the outstanding browser request. Replays are refused, and manually configured identity providers use the saved certificate for signature checks.
- **Certificate trust survives reconnection.** Proxmox recovery cannot replace established pins. Conflicting pins for equivalent addresses block reuse until reconciled. The editor explains pin precedence and independent checks before certificate renewal.
- **Redirects keep credentials at their destination.** Infrastructure and notification requests refuse redirects to another origin. Configure the final notification URL directly when a receiver redirects elsewhere.
- **Diagnostics expose less private data.** Credentials, receiver replies, template contents and unsafe retry hints stay out of connection and delivery errors. Saved secrets remain masked, and DNS lookups stop when requests are cancelled.
- **Webhook destination checks cover more address forms.** Forbidden IPv4 addresses embedded in IPv6 cannot bypass destination restrictions.
- **Demo restrictions remain enforced.** WebSocket headers cannot bypass blocked changes, and profiling endpoints stay hidden. Plain HTTP is critical only on publicly reachable installs, with an HTTPS recommendation on private networks.
- **Sign-in settings update consistently.** SSO changes publish together after saving. Session, settings and logout changes reject read-only HTTP methods. Security fixes update Go, networking and frontend dependencies.

## Other improvements

- **Tables behave predictably.** Selecting text no longer toggles rows, expanded details span visible columns, and workload selection no longer restores an obsolete scroll position.
- **Kiosk views show more monitoring.** Filter toolbars and section tabs stay hidden, and table preview limits are lifted.
- **Controls are easier to read.** Light-mode banners have stronger contrast, disabled toggles show their actual state, and Manage explains operator controls in plain language. Production styling works under Pulse's content security policy.
- **Connection setup is easier to check.** Infrastructure entries open the platform showing their data. Proxmox, PBS and Mail Gateway tests use edited certificate settings with saved credentials before saving.
- **Connection errors give safer next steps.** Timeouts, sign-in failures, denied access and certificate problems have distinct guidance. Cluster diagnostics work with keyboard and touch, without suggesting broader permissions or disabled verification.
- **Logs stay responsive.** Slow viewers no longer block server logging, and the browser batches incoming lines. Pause, clear and download explain their effects.
- **Views can be bookmarked.** Patrol and Actions have dedicated routes, and links to Patrol's mode selector open the control.
- **Troubleshooting gives safer next steps.** Guides cover backup safety, PBS permissions, Kubernetes credentials, OIDC, proxy access, CORS and NAS setup. Checks have time limits and keep credentials private.
- **Bug reports need less guesswork.** Guidance separates guests from the Pulse server, explains damaged tokens without needless replacement, and uses existing versions and logs without repeating an update or live diagnosis.
- **Audit requests fail clearly.** Invalid export and summary time filters are rejected, while concurrent tenant incident records keep distinct identifiers.
- **The GitHub star prompt is less abrupt.** It appears on the first return day with clearer presentation.
- **Optional statistics explain more usage.** Telemetry Preview includes 30-day counts of older-history reads, report generation, delivery and repeated use without report contents or resource identities. Opt-out remains effective.

## Known issues

- **Guest freezes remain unresolved.** Related safeguards do not confirm every case is fixed. If a guest previously froze and monitoring was stopped, do not restart monitoring solely to test this beta or release.
- **Affected backups still need precautions.** Stop Pulse monitoring before planned backups. Keep it stopped until independent checks confirm thaw, fresh writes on every backed-up filesystem and workload liveness. An OK backup alone is insufficient.
- **Windows restarts remain under investigation.** Guest-reading fixes do not confirm all reported restart or missing-disk cases are resolved. Use existing logs and independent health checks, without restarting monitoring solely to reproduce the problem.
- **Elevated server memory remains unresolved.** Lower allocation work does not prove lower sustained usage. Report both higher steady usage and continued growth, comparing existing readings at similar uptimes and workloads.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile is being retired on 31 March 2027. It keeps working until then. For alerts on your phone afterwards, add an ntfy, Gotify, or Pushover destination, and open Pulse in your phone's browser. Relay is no longer sold, and existing Relay subscribers now have Pro features at their current price. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.5.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.5.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.5.0` and recreate the container.
