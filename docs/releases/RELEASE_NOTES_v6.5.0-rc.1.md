# Pulse v6.5.0-rc.1 Release Notes

This release candidate opens the `v6.5.0` candidate line after stable `v6.4.5`, which remains the rollback target.

Pulse v6.5.0 makes everyday monitoring more dependable, especially when backups are running, connections drop or readings are incomplete. Alerts recover more reliably, backup coverage is clearer, and charts distinguish missing data from real measurements. Patrol gains weekly summaries and clearer cost controls, while setup and updates give you more control over credentials and unattended changes.

## Highlights

- Understand what needs attention with clearer readings, backup coverage and delivery status.
- Keep monitoring steady through connection failures, restarts and busy systems.
- See what Patrol accomplished and make informed choices about its running costs.

## What's improved

- **Details stay open and current.** Dashboard refreshes preserve expanded rows, selected tabs, focus and unsaved connection edits without shifting the page.
- **History is easier to inspect.** Charts support keyboard and touch inspection, align readings to one time window and show individual observations without inventing a trend.
- **Failed refreshes are clearer.** History distinguishes empty results from refresh failures and discards responses for old targets, ranges or organisations.
- **Tables are easier to read.** Storage and platform tables fit values more consistently, metric labels use their actual width, and filesystem paths remain readable.
- **Navigation is more accessible.** Search history, the command palette, tabs, dialogs and Assistant suggestions work more consistently with keyboards and screen readers.
- **Kubernetes job timing is clearer.** Controller details show job times as readable ages, helping you understand when work started or finished.

## Alerts and notifications

- **Recurring alerts deliver reliably.** Resolution, restart and delayed retries preserve each occurrence without replaying notifications from an earlier incident.
- **Recovery messages match delivery.** Pulse tracks destination results for partial deliveries and distinguishes a notification being dispatched from its receipt.
- **Retries handle failures better.** Temporary destination failures respect retry delays, while permanent credential, SMTP and configuration failures stop futile retries.
- **Quiet hours remain effective.** Queued notifications are checked against current quiet hours and local time before sending.
- **Escalations reach you.** Severity increases can notify through cooldowns, and continuing symptoms can notify after the primary problem recovers.
- **Suppression remains respected.** Monitor-only alerts, operator suppression and warning preferences stay effective during escalation and queued delivery.
- **Acknowledgements survive cleanup.** Active acknowledgements and observed unacknowledged alerts remain intact, including automatic acknowledgement across repeat firings.
- **Flapping follows your settings.** Thresholds above ten are honoured, and expired flapping episodes stop holding later notifications.
- **Missing readings do not imply recovery.** Invalid metrics and uncertain connectivity no longer clear storage incidents or produce misleading capacity decisions.
- **Delivery status stays current.** Overlapping requests and monitoring reloads no longer replace current notification health with stale results.
- **Incident history stays relevant.** Alert timelines and Assistant handoffs retain the correct resource and occurrence, including Docker events.
- **Webhook tests match saved settings.** Custom fields use the configured values, and destination server failures are easier to identify.

## Disks and storage

- **Disk readings stay tied to their source.** Physical disks retain their identity across refreshes, and SMART history avoids replaying old measurements as new samples.
- **Disk health is more accurate.** Endurance values no longer inflate remaining life, and Proxmox disk alerts use the correct physical-disk policy and recovery readings.
- **Included mounts remain included.** Explicit disk selections survive ingestion, including tmpfs mounts, and collection uses the agent's mount namespace.
- **RAID details are clearer.** VM details show linked disks and RAID, while MD RAID spare counts are corrected without hiding failed members.
- **Unraid avoids misleading warnings.** Empty slots and explicitly pool-only arrays no longer trigger inappropriate filesystem or parity warnings.
- **Storage details preserve missing data.** Unavailable pool capacity stays unavailable, disk facts remain on their owning host, and stored disk history remains accessible when collection stops.
- **Temperatures appear more consistently.** Disk temperature can display without extended SMART data, with improved CPU and other sensor fallbacks, including ARM systems.

## Proxmox, PBS and backups

- **Guest reads respect backup locks.** Pulse coordinates guest-agent checks around backups and defers uncertain requests without automatically repeating commands.
- **Retained guest readings show their age.** Memory survives repeated backup locks with its original observation time, while paused or incomplete filesystem readings remain clearly unavailable or last known.
- **Backup coverage uses the right guest.** PBS snapshots and structured comments are matched only to attributable guests, avoiding shared-node and cross-site mix-ups.
- **Backup dates stay honest.** Missing, invalid or future timestamps do not count as healthy protection, and completed backups remain visible while another backup is active.
- **Inventory failures stay local.** Unreadable backup responses do not publish misleading inventory, and a failure at one source does not invalidate independent results.
- **PBS capacity alerts follow your policy.** Live polls apply datastore settings and preserve existing incidents when node status or measurements are incomplete.
- **PBS capacity failures are visible.** Failed datastore reads no longer appear as healthy capacity in the backup dashboard.
- **PBS History follows the actual machine.** Agent and Proxmox readings use corroborated host identity, rejecting ambiguous matches and withdrawing stale links.
- **Proxmox navigation follows available data.** Tabs reflect observed capabilities, filters stay within their source, and node rows let you view that node's guests.
- **Repeated site names stay separate.** Proxmox clusters with matching labels retain their own resources in live updates.
- **Backup troubleshooting is safer.** Passive VM disk diagnostics avoid guest-agent commands, and guidance explains monitoring pauses without presenting them as proof that a backup is safe.

## TrueNAS, vSphere and Docker

- **TrueNAS CORE reporting works more consistently.** CORE 12 compatibility and native reporting improve memory, ARC, CPU, disk and network readings, including temperature history.
- **TrueNAS polling recovers from silent peers.** Requests have bounded waits, idle sessions are maintained, and ended subscriptions no longer leave reads stalled.
- **TrueNAS missing data stays distinct from zero.** Empty app statistics and incomplete reporting no longer become misleading measurements or synthetic CPU temperatures.
- **TrueNAS incidents retain useful severity.** Emergency and actionable NOTICE alerts reach incident delivery, informational messages stay out, and finished replication jobs are recognised.
- **Connection tests stay separate from monitoring.** TrueNAS probe results do not imply successful polling, while saved TrueNAS and vSphere connections retain runtime connection alerts.
- **vSphere JSON API compatibility improves.** Calls support vSphere 8.0.3 requirements, tolerate absent values and expose more useful API fault details.
- **Container metrics are more accurate.** Host and Docker CPU sampling remain independent, Podman CPU uses advancing interval readings, and Docker storage retains observed values.
- **Docker image checks avoid false updates.** All local image digests count as current, and invalid registry responses are rejected.
- **Container updates show confirmed outcomes.** Actions remain pending until results are known, require consistent running and health checks, and retain their age across restarts.

## Install, updates and agents

- **Automatic updates require a choice.** New installations ask before enabling unattended updates, and manual version changes preserve your existing preference.
- **Update checks reflect installability.** Pulse checks for the correct server archive and ARM build variant, with more reliable handling of large release listings and unknown dates.
- **Update progress stays responsive.** Quiet update streams trigger status checks, and Pulse confirms server readiness before reloading the dashboard.
- **Successful updates report success.** Updater cleanup no longer causes a completed automatic update to exit with an error.
- **FreeBSD installation is more dependable.** Checksum verification works without coreutils, and agent logs are collected on FreeBSD and pfSense.
- **Agent continuity improves.** Credentials survive monitoring reloads, live state takes priority over old enrolment records, and removed agents remain blocked from re-enrolling.
- **Windows host detection recovers.** Agents handle MachineGuid values with braces when collecting host information.
- **Peer sensor collection is optional.** Proxmox agents can skip SSH sensor collection from cluster peers.
- **Disconnected commands fail promptly.** Typed action requests stop waiting when their agent session drops, and stale action responses no longer overwrite current review state.
- **Setup instructions are clearer.** First-source guidance explains what agents add, offers a container-console route to the setup token and separates monitoring access from command permissions.

## Pulse Pro, AI and hosted

- **Patrol shows its weekly work.** The Activity page summarises checks, findings, actions and estimated spend, with clear notices when retained history covers less than a week.
- **Weekly summaries can arrive by email.** Settings > Reporting lets you schedule a Patrol summary with a weekday, time, timezone and recipients.
- **Model costs are easier to compare.** Patrol setup previews estimated spending across schedules and clearly explains when an exhausted budget pauses analysis.
- **Cached AI usage is counted.** Anthropic cache reads and writes contribute to usage, budgets and projections, including fully cached runs.
- **Patrol findings are less repetitive.** Findings that mirror alerts are folded together, flapping is grouped, and lasting decisions remain visible.
- **AI explanations retain context.** Patrol and alert actions can start scoped explanations, with resource history, action outcomes and uncertainty carried through investigations.
- **Assistant access is more consistent.** Monitored targets remain available without command access, saved Patrol objectives recover, and unnecessary continuation prompts are removed.
- **Provider configuration works better.** Ollama Basic Auth settings are restored, GPT-5 requests use supported completion limits, and stable prompt prefixes can be cached.
- **AI controls reflect availability.** Patrol navigation is hidden when AI is off, and setup blocks leave existing attention items visible.
- **Hosted clients survive upgrades more reliably.** Provider-managed client recovery, licence refresh and tenant cleanup respect the owning provider.
- **Hosted billing reflects applied plans.** Provider-hosted platforms can buy or renew their own licence, and billing returns confirm the running plan before announcing success.
- **Organisation settings are clearer.** Owners see the settings their organisation supports, hosted workspaces can create agent install tokens, and Relay labels and upsells are removed.

## Monitoring and service health

- **Larger installations do less repeated work.** Agent refreshes, resource lookups and dashboard broadcasts reduce copying and repeated processing.
- **Metric storage is more resilient.** History handles startup write locks, retries, rollups and shutdown more reliably, with fewer duplicate samples across storage tiers.
- **Samples retain their measurement time.** Unified metrics use source observation times, and used-memory history expires within its configured retention window.
- **Statistics stay responsive.** Statistics reads are isolated from heavier history queries, and metric synchronisation batches database writes.
- **Service checks are bounded.** Local health probes support IPv4 and IPv6 without waiting indefinitely, and cancelled reloads release waiting callers.
- **Resource linking avoids false matches.** Host-local addresses are excluded from automatic matching, while explicit host links and custom bridge information are preserved.
- **Linked memory remains available.** Proxmox VMs recover correlated agent memory, and LXC monitoring prefers linked agent readings.
- **Saved platforms keep polling.** Missing organisation metadata no longer prevents monitoring configured connections.

## Security

- **Setup commands expose fewer secrets.** Proxmox bootstrap and agent tokens stay out of copied commands and hosted setup downloads, with private credential entry.
- **Notification diagnostics mask credentials.** Webhook, Slack, Discord, Telegram, ntfy and Apprise diagnostics redact more credential forms and private payloads.
- **Diagnostics exports redact more detail.** Downloads mask nested guest identities, token references, mount paths and upstream response details, with guidance to review files before sharing.
- **History respects access changes.** Denied readings are withdrawn, and cached responses or drawer fallbacks cannot substitute data from another organisation.
- **TLS choices remain yours.** Explicit certificate-verification settings survive Proxmox registration and consolidation.
- **Login recovery preserves configuration.** Local authentication recovery and rotation retain authentication state and system preferences, with admin permissions synchronised during setup.
- **SSO mappings are more compatible.** OIDC handles mixed key sets, and settings preserve group mappings containing spaces.
- **Security dependencies are updated.** Go cryptography and frontend dependencies include fixes for newly identified vulnerabilities.

## Other improvements

- **Telemetry is a setup choice.** Consent moves into onboarding, and test or development builds avoid production telemetry reporting.
- **Recovery help is more practical.** Updated guides explain data-preserving removal, configuration backup limits, trusted proxy headers and safe incident reporting.
- **The demo is easier to enter.** Visitors are signed in automatically, and the banner leads directly to installation instructions.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile remains compatible. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.4.5`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.4.5` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.4.5` and recreate the container.
