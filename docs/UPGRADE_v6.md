# Upgrade to Pulse v6

This guide covers the v5-to-v6 migration and its compatibility changes. For
updates within v6, use [Updating Pulse](AUTO_UPDATE.md); the historical packets
below are context, not an instruction to install an old version or downgrade.
Choose an actually [published software release](https://github.com/rcourtman/Pulse/releases),
not a draft, source tag or Helm-chart asset. Keep the existing runtime edition:
public Community downloads do not replace a private Pro build.

For the current stable v6 packet, see:

- `docs/releases/RELEASE_NOTES_v6.6.0.md`
- `docs/releases/V6_CHANGELOG_v6.6.0.md`

For the preceding stable v6 packet, see:

- `docs/releases/RELEASE_NOTES_v6.5.0.md`
- `docs/releases/V6_CHANGELOG_v6.5.0.md`

For the earlier stable v6 packet, see:

- `docs/releases/RELEASE_NOTES_v6.4.1.md`
- `docs/releases/V6_CHANGELOG_v6.4.1.md`

For the earlier stable v6 packet, see:

- `docs/releases/RELEASE_NOTES_v6.4.0.md`
- `docs/releases/V6_CHANGELOG_v6.4.0.md`

The `v6.4.2` packet was tagged but never published; its changes ship through the v6.4.3 candidate line:

- `docs/releases/RELEASE_NOTES_v6.4.2.md`
- `docs/releases/V6_CHANGELOG_v6.4.2.md`

For the earlier stable v6 packet, see:

- `docs/releases/RELEASE_NOTES_v6.3.2.md`
- `docs/releases/V6_CHANGELOG_v6.3.2.md`

For the preceding v6.4 release candidate packet, see:

- `docs/releases/RELEASE_NOTES_v6.4.0-rc.13.md`
- `docs/releases/V6_CHANGELOG_v6.4.0-rc.13.md`

For the earlier stable v6 packet, see:

- `docs/releases/RELEASE_NOTES_v6.3.1.md`
- `docs/releases/V6_CHANGELOG_v6.3.1.md`

For the earlier v6.4 release candidate packet, see:

- `docs/releases/RELEASE_NOTES_v6.4.0-rc.12.md`
- `docs/releases/V6_CHANGELOG_v6.4.0-rc.12.md`

For earlier v6.4 release candidate packets, see:
- `docs/releases/RELEASE_NOTES_v6.4.0-rc.11.md`
- `docs/releases/V6_CHANGELOG_v6.4.0-rc.11.md`
- `docs/releases/RELEASE_NOTES_v6.4.0-rc.10.md`
- `docs/releases/V6_CHANGELOG_v6.4.0-rc.10.md`
- `docs/releases/RELEASE_NOTES_v6.4.0-rc.9.md`
- `docs/releases/V6_CHANGELOG_v6.4.0-rc.9.md`

- `docs/releases/RELEASE_NOTES_v6.4.0-rc.8.md`
- `docs/releases/V6_CHANGELOG_v6.4.0-rc.8.md`
- `docs/releases/RELEASE_NOTES_v6.4.0-rc.7.md`
- `docs/releases/V6_CHANGELOG_v6.4.0-rc.7.md`
- `docs/releases/RELEASE_NOTES_v6.4.0-rc.6.md`
- `docs/releases/V6_CHANGELOG_v6.4.0-rc.6.md`

- `docs/releases/RELEASE_NOTES_v6.4.0-rc.5.md`
- `docs/releases/V6_CHANGELOG_v6.4.0-rc.5.md`
- `docs/releases/RELEASE_NOTES_v6.4.0-rc.4.md`
- `docs/releases/V6_CHANGELOG_v6.4.0-rc.4.md`
- `docs/releases/RELEASE_NOTES_v6.4.0-rc.3.md`
- `docs/releases/V6_CHANGELOG_v6.4.0-rc.3.md`
- `docs/releases/RELEASE_NOTES_v6.4.0-rc.2.md`
- `docs/releases/V6_CHANGELOG_v6.4.0-rc.2.md`
- `docs/releases/RELEASE_NOTES_v6.4.0-rc.1.md`
- `docs/releases/V6_CHANGELOG_v6.4.0-rc.1.md`

For earlier stable v6.2 packets, see:

- `docs/releases/RELEASE_NOTES_v6.2.1.md`
- `docs/releases/V6_CHANGELOG_v6.2.1.md`

For historical v6.3 release candidate packets, see:

- `docs/releases/RELEASE_NOTES_v6.3.0-rc.6.md`
- `docs/releases/V6_CHANGELOG_v6.3.0-rc.6.md`

- `docs/releases/RELEASE_NOTES_v6.2.0.md`
- `docs/releases/V6_CHANGELOG_v6.2.0.md`

For historical v6.2 support release candidate packets, see:

- `docs/releases/RELEASE_NOTES_v6.2.2-rc.2.md`
- `docs/releases/V6_CHANGELOG_v6.2.2-rc.2.md`
- `docs/releases/RELEASE_NOTES_v6.2.2-rc.1.md`
- `docs/releases/V6_CHANGELOG_v6.2.2-rc.1.md`
- `docs/releases/RELEASE_NOTES_v6.2.0-rc.11.md`
- `docs/releases/V6_CHANGELOG_v6.2.0-rc.11.md`

- `docs/releases/RELEASE_NOTES_v6.2.0-rc.10.md`
- `docs/releases/V6_CHANGELOG_v6.2.0-rc.10.md`
- `docs/releases/RELEASE_NOTES_v6.2.0-rc.9.md`
- `docs/releases/V6_CHANGELOG_v6.2.0-rc.9.md`
- `docs/releases/RELEASE_NOTES_v6.2.0-rc.8.md`
- `docs/releases/V6_CHANGELOG_v6.2.0-rc.8.md`
- `docs/releases/RELEASE_NOTES_v6.2.0-rc.7.md`
- `docs/releases/V6_CHANGELOG_v6.2.0-rc.7.md`
- `docs/releases/RELEASE_NOTES_v6.2.0-rc.6.md`
- `docs/releases/V6_CHANGELOG_v6.2.0-rc.6.md`
- `docs/releases/RELEASE_NOTES_v6.2.0-rc.5.md`
- `docs/releases/V6_CHANGELOG_v6.2.0-rc.5.md`
- `docs/releases/RELEASE_NOTES_v6.2.0-rc.4.md`
- `docs/releases/V6_CHANGELOG_v6.2.0-rc.4.md`
- `docs/releases/RELEASE_NOTES_v6.2.0-rc.3.md`
- `docs/releases/V6_CHANGELOG_v6.2.0-rc.3.md`
- `docs/releases/RELEASE_NOTES_v6.2.0-rc.2.md`
- `docs/releases/V6_CHANGELOG_v6.2.0-rc.2.md`
- `docs/releases/RELEASE_NOTES_v6.2.0-rc.1.md`
- `docs/releases/V6_CHANGELOG_v6.2.0-rc.1.md`

For earlier stable v6 packets and rollout references, see:

- `docs/releases/RELEASE_NOTES_v6.1.2.md`
- `docs/releases/V6_CHANGELOG_v6.1.2.md`

- `docs/releases/RELEASE_NOTES_v6.1.1.md`
- `docs/releases/V6_CHANGELOG_v6.1.1.md`
- `docs/releases/RELEASE_NOTES_v6.1.0.md`
- `docs/releases/V6_CHANGELOG_v6.1.0.md`
- `docs/releases/RELEASE_NOTES_v6.0.5.md`
- `docs/releases/V6_CHANGELOG_v6.0.5.md`
- `docs/releases/RELEASE_NOTES_v6.0.4.md`
- `docs/releases/V6_CHANGELOG_v6.0.4.md`
- `docs/releases/RELEASE_NOTES_v6.0.3.md`
- `docs/releases/V6_CHANGELOG_v6.0.3.md`
- `docs/releases/RELEASE_NOTES_v6.md`
- `docs/releases/V6_CHANGELOG.md`

The published GitHub release is the authority for what users can install as
stable. The v6.0.0 cutover recorded v5.1.35 as its rollback target; that
historical choice is not a safe downgrade procedure for a current installation.

## Before You Upgrade

- Keep a private, consistent backup of the existing installation's active data,
  matching encryption/signing keys and deployment configuration. **Create Backup**
  in **Settings → System → Recovery** (older versions labelled this **Backups**)
  is a configuration export, **not a full-state backup**: history and agent
  enrolment state are excluded. Follow [full-state recovery](MIGRATION.md#full-state-recovery);
  do not copy live data or assume an updater snapshot includes every active store.
- Confirm existing administrator access to Pulse and its host/container console.
  Preserve credentials, connection state, agent identities and the existing
  service and data/config paths. An upgrade does not require first-time setup.
- Choose a maintenance window: updates interrupt monitoring and alert delivery.
  Arrange independent coverage. Do not update during a backup, freeze/thaw or
  an unresponsive-workload incident; use the [backup safety precaution](VM_DISK_MONITORING.md#backup-safety)
  before restoring paused monitoring.
- Open **Settings → System → Updates** and review the upgrade checks on the
  update plan. These describe the currently reported fleet and server update
  path, not every installed agent's continuity or convergence.
- If the instance uses SSO without a configured local administrator, verify the
  trusted IdP group's existing built-in `admin` mapping before upgrading. SSO
  authentication alone does not grant instance-administrator access; an unmapped,
  `operator` or `viewer` SSO session cannot manage settings or transfer configuration.
  Do not remove role gating to repair a denial.
- Review [API changes](#api-changes) for existing integrations and
  [paid licensing connectivity](#breaking-change-paid-licensing-requires-connectivity)
  before changing a paid installation. Preserve the private Pro runtime.

## Upgrade Paths

### systemd and Proxmox LXC installs

Use **Settings → System → Updates** where supported. For CLI updates, follow
[the existing-installation manual update procedure](INSTALL.md#manual-update):
it checks the helper's ownership and preserves the existing service and
data/config paths. Work inside the existing Pulse LXC, not on the Proxmox host
or a monitored guest. Do not execute an absent or unknown helper; its filename
`/bin/update` alone does not identify its owner or prove it accepts an exact version.
Do not apply default installation paths over a custom deployment.

If a community helper selects a Helm-chart asset, use
[chart-selection recovery](AUTO_UPDATE.md#a-helper-selects-a-helm-chart-release),
not another blind attempt, a GitHub token or a recreated LXC. The public signed
server-installer flow is for Community; do not replace a private Pro runtime
with it. Preserve the running version, edition and service health first.

Agent updates and v5-to-v6 agent upgrades are separate from server updates.
Use the [trusted-instance agent procedure](UNIFIED_AGENT.md#private-file-installation-linux-macos-and-nas)
and **Settings → Infrastructure → Install on a host** for first installs and
in-place agent upgrades; do not use the GitHub server installer for `pulse-agent`.

For hosts pinned to `v6.0.0-rc.2`, the historical Pulse update signer was not
recovered. This records a trust-continuity limit, not an instruction to reinstall
or bypass signature verification. Arrange an explicit verified trust migration
through the deployment's recovery procedure, preserving its identity, data and
private edition; do not assume unattended continuity from that early trust root.

### Docker

Follow [Docker server updates](DOCKER.md#-updates) for the existing deployment.
Change the configured image to the exact published tag first, then pull and
recreate only the Pulse service, preserving mounted data, credentials and
collector settings. Pulling a different tag does not change a hardcoded Compose
image or the running container. Keep a private Pro image private; do not replace
it with `rcourtman/pulse`. Do not run `docker compose down`, remove volumes or
recreate unrelated services for a Pulse-only update.

### Kubernetes (Helm)

Follow [the Helm deployment guidance](KUBERNETES.md#1-deploying-the-pulse-server),
preserving the existing release, namespace, values and persistent volumes.
Select the intended published chart and server image separately; a chart version
alone does not establish the running server version or edition. Keep existing
private Pro image configuration and Secret references; do not paste credentials
into Helm command arguments or share resolved values or Secret YAML.

## Post-Upgrade Checklist

Use existing authenticated browser/API access and passive observations, not
new diagnostic traffic or active probes just to fill this checklist. For API reads use an [authenticated browser or private
header file](API.md#-authentication), never a token or session cookie in a
command, URL or report.

- Confirm the **running server version and edition**, not just the selected
  release or a completed download. `GET /api/version` reports the server version;
  a valid signature is not completed installation or healthy operation.
- Observe ordinary polling, current readings and their original source/time.
  A green connection badge or response from `/api/health` does not prove every
  collector recovered. Read-only `GET /api/monitoring/scheduler/health` and
  `GET /api/resources` can supplement, not replace, those observations.
- Check ordinary notification delivery already observed and its destination/
  resource identity. A successful Test does not prove ordinary delivery; do not
  send a Test, trigger an alert or replay a queue just to complete this checklist.
- Check agents individually where used. A current server does not prove that
  every installed agent upgraded, authenticated or resumed reporting.
- If monitoring was paused for backup safety, restore only its previously active
  services/timers after independent thaw, every-covered-filesystem writes and
  workload liveness checks pass. An OK backup task is not that proof.

### Failed or interrupted upgrade

Check the running version, edition and service health before another attempt.
Keep the original attempt time/timezone, error and any already available updater
or host stop evidence privately. A broken pipe does not establish why a service
or its whole LXC stopped; a later boot is a new run, not the stop's cause.

Use [update failure checks](AUTO_UPDATE.md#update-failed) and, if needed,
[timer-only containment](AUTO_UPDATE.md#pause-further-unattended-attempts-without-stopping-pulse)
without stopping a working Pulse server. Do not start, cancel or repeat an
uncertain in-progress updater. Do not repeat the update, reboot, restore, run
Diagnostics or delete rollback backups merely to obtain evidence.

An older binary may not understand newer data. Use the
[stopped-service recovery procedure](AUTO_UPDATE.md#manual-rollback) for matching
data and keys, preserving reversible copies of the failed state. An image or
binary change alone is not a data rollback. Share only relevant locally reviewed,
redacted observations via [Getting Help](TROUBLESHOOTING.md#-getting-help), not
backups, full logs or deployment configuration.

## v5 to v6 Operator FAQ

### Do I need to uninstall Pulse v5 first?

No. Upgrade the existing Pulse server installation in place.

### Do I need to uninstall my existing Pulse Unified Agents first?

No. Use the unified installer to upgrade existing agent deployments in place. Generate the current command from **Settings → Infrastructure → Install on a host**, then run it on the host that already has the v5 agent service. You do not need to remove the old service first.

### Does upgrading the Pulse server prove that all agents have upgraded?

No. The server and Unified Agent have separate update lifecycles. After the
server changes the target version, eligible v6 agents normally discover and
apply that update asynchronously: once about five seconds after process start,
then hourly while the process remains running. Linux, Windows, and
Docker-enabled installations all use this same Unified Agent updater. It moves
only to a higher semantic version, so RC-to-later-RC, RC-to-stable, and
stable-to-later-stable updates are eligible; a server advertising an older
target never causes an automatic downgrade. Version checks are cache-unique,
and the binary request is bound to the exact target version. Offline checks,
download failures, and pre-replacement failures leave the current process and
binary in place and are retried on a later check. The server being current is
not proof that every installed agent has checked in or converged.

Use the manual path for v5 agents, PVE host agents, agents with auto-update
disabled, and agents blocked by authentication, missing connection state,
download, trust, or self-test failures. For v5-to-v6 upgrades, generate the
current command from **Settings → Infrastructure → Install on a host** and run it
on the host with the existing agent service. A v5 agent can be missing from v6
Reporting until it has upgraded, authenticated, and sent its first v6 report.

For agents already visible to v6, open an outdated-agent notice or **Agent
Doctor** at `/settings/infrastructure?agentDoctor=1`. It shows per-host commands
for the operator to copy and run; it does not remotely execute fleet updates. Agent
self-update and manual update both still depend on valid authentication, a
reachable trusted update channel, and accepted release signing keys.

Early v6 prerelease agents could report successfully to a private plain-HTTP
Pulse URL while their separate updater rejected that same URL before making a
version request. Such an agent cannot download the release that fixes its own
transport policy. Re-run the current per-host installer command once to
preserve the URL/token/trust settings in the current lifecycle format; normal
automatic checks resume after that migration. The same manual recovery rule
applies to an agent whose installed signing trust cannot accept the current
release.

### Will an upgraded v5 agent keep the same identity in v6?

Yes. The v5-to-v6 agent path is expected to preserve one canonical agent
identity rather than creating a duplicate record during the upgrade.

### Do I need new agent tokens just because the server moved to v6?

No. Existing installed agents are expected to continue through the v6
compatibility boundary for legacy persisted agent scopes.

If you create a replacement token during the upgrade, install or reconfigure the
agent with the replacement before revoking the old token. Revoking the token
currently used by an agent stops that agent from authenticating until it is
reinstalled or reconfigured with a valid token.

### Where do I check installed agent versions in v6?

After an agent reports to v6, check the relevant platform page or **Machines**
view for the agent-backed host and version/status details. Version and outdated
agent notices appear only after the agent has successfully reported; they are
not an offline inventory of every v5 service that existed before the server
upgrade. On the host itself, confirm the local binary with:

```bash
pulse-agent --version
systemctl status pulse-agent
```

### Can one installed Pulse Unified Agent report to two Pulse instances at the same time?

Yes. Configure one instance as the primary with `--url` and its token, then add
the other as a report-only observer with `--observers-file`. Only the primary
can supply remote configuration, commands, enrollment, or updates; observer
delivery and retries are isolated. Each instance needs its own Pulse API token,
and Proxmox observers use separate PVE/PBS tokens. See
[Observer destinations](UNIFIED_AGENT.md#observer-destinations) for the file
format and security requirements. Use a v6-capable agent for this topology;
older v5 agents do not understand observer configuration.

### Can I keep Pulse v5 stable while I test Pulse v6?

Keep the original installation and a verified recovery backup. Evaluate a
restored copy only in an [isolated recovery instance](AUTO_UPDATE.md#manual-rollback)
with no access to monitored systems or notification destinations; do not start
a second connected copy. Moving the binary back does not reverse data migrated
by v6. The final v5 release, v5.1.36, is end-of-life; it is historical context,
not a supported recovery target for every current installation.

Returning to an earlier version requires a deployment-specific recovery plan,
matching data and keys, a compatible binary and the same edition. Follow the
[rollback limits](INSTALL.md#rollback), not a generic `/bin/update` downgrade
against the live migrated data.

### Why did my v5 install upgrade itself to v6?

Pulse 5.1.29 and later pin the built-in updater to the 5.1.x line. Pulse 5.1.28
and older have no such pin: an enabled updater can follow a newer stable release.
This version-selection behaviour does not prove that the resulting upgrade or
all collectors recovered. Check the running version and edition using the
[post-upgrade checklist](#post-upgrade-checklist); preserve the original state
and [recovery evidence](#failed-or-interrupted-upgrade) if anything remains wrong.
Do not blindly run another updater or downgrade just because the target changed.

## Migration Notes (v6)

### Unified Navigation (Bookmarks and Deep Links)

Pulse v6.0.0-rc.6 and later prereleases ship with the platform-shaped top-level
navigation existing v5 operators already know: Proxmox, Docker, Kubernetes,
TrueNAS, vSphere, Machines, Alerts, Patrol, and Settings.

The backend unified resource model and `/api/resources` contract remain
canonical, but the retired rc.1 through rc.5 `/infrastructure`, `/workloads`,
`/storage`, and `/recovery` layout is not the shipped v6 user interface.

- Reference: `docs/MIGRATION_UNIFIED_NAV.md`
- If you used rc.1 through rc.5, update bookmarks or runbooks from the retired
  unified routes to the platform-shaped equivalents listed in that guide.
- If you are upgrading directly from v5, start from the familiar platform pages
  rather than looking for the temporary unified pages from early v6 RCs.

### Configuration Compatibility

Pulse v6 honors the legacy `PORT` environment variable as a deprecated fallback
only when `FRONTEND_PORT` is unset, so existing installs keep their listener
port after upgrade. Move deployments to `FRONTEND_PORT`; when both variables
are set, `FRONTEND_PORT` wins.

### API Changes

Unified Resources is now the canonical model and endpoint family:

- Canonical: `/api/resources`

Availability checks now attach to an existing canonical resource when an
explicit `linkedResourceId` resolves or one normalized IP/hostname match is
unambiguous. Every configured check remains individually visible in the
Availability checks inventory; an attached check also appears as an additive
facet on the owning platform row/detail. API consumers should treat the
source-owned `network-endpoint` as the check identity and accept the additive
`availabilityChecks`, correlation, evidence, and outgoing `checks`
relationship fields; the existing singular `availability` field remains as a
compatibility summary. Ambiguous or invalid links stay visible as
standalone/unresolved and are never guessed.

### License and Entitlements

Pulse v6 feature gating is driven by the entitlements endpoint:

- `GET /api/license/entitlements`

For self-hosted v6, Pulse no longer sells monitored-system volume. Core
monitoring stays available across Community and Pro. Relay is no longer sold;
existing Relay subscribers receive Pro features at their current price while
their subscription continues. Preserve those entitlements and the private
runtime when upgrading; see [current plans](PULSE_PRO.md).

Self-hosted v6 does not expose a general in-app trial, trial-return callback,
or hosted AI quickstart path. Ordinary upgraded self-hosted installs should use
activation, recovery, or BYOK/local AI setup instead; any exceptional
support-issued entitlement is reflected through hosted entitlement state rather
than a local in-app trial acquisition flow.

#### Breaking Change: Paid Licensing Requires Connectivity

Pulse v5 validated paid license keys entirely locally. Once activated, a v5
Pro or Lifetime install never needed to reach a licensing service again, so
fully offline and air-gapped installs kept paid features indefinitely.

Pulse v6 does not work that way. This is a breaking change for paid installs:

- **What changed.** v6 activates a paid license against
  `license.pulserelay.pro` and then refreshes a short-lived entitlement grant
  in the background (several times a day by default). The grant is valid for
  72 hours, and after it expires Pulse allows a further 7 day grace window.
- **Offline tolerance.** A paid v6 instance that cannot reach
  `license.pulserelay.pro` keeps its paid features for roughly 10 days from
  the last successful refresh (72 hour grant lifetime plus 7 day grace).
  After that, paid features drop to Community behavior until connectivity
  returns. Core monitoring keeps running throughout; this affects paid
  surfaces such as extended history and AI operations, not data collection.
- **Recovery is automatic.** When connectivity returns, the background
  refresh (or a restart) reactivates the license without re-entering the key.
- **Who is affected.** Every paid self-hosted install, including Lifetime.
  Air-gapped or egress-restricted environments are affected the most: v6
  cannot keep paid features active without periodic outbound HTTPS to
  `license.pulserelay.pro`.
- **What to do.** Allow outbound HTTPS (port 443) from the Pulse server to
  `license.pulserelay.pro`. If your environment is air-gapped or cannot
  allow that egress, contact `support@pulserelay.pro` before upgrading to
  discuss options for your install.

#### Paid Pulse Pro Runtime

Paid Pulse Pro, Relay, and eligible legacy customers should not use public
GitHub release assets or the public `rcourtman/pulse` Docker image for paid
runtime features. Those public downloads are community builds. They can accept
an activation key, but they do not include the private Pulse Pro runtime hooks.

Use <https://pulserelay.pro/download.html> with your activation key instead.
Docker users should run the private registry login and
`PULSE_IMAGE=license.pulserelay.pro/pulse-pro:<version>` compose commands shown
there. Those commands require your compose file image line to use the
`PULSE_IMAGE` variable. If your compose file hardcodes
`image: rcourtman/pulse:...`, replace that line with the variable form from
`docker-compose.yml` or directly with the private image shown on the download
page. Direct Linux users should download the private Pulse Pro archive from the
same page.

When a paid activation runs on a community build, **Settings → Updates** offers
the switch itself. Systemd and Proxmox LXC installs switch in place: Pulse
downloads the signed Pro archive for its own version (or a newer one) from the
license server, backs up the community build, and restarts; **Roll back** on
that Update history entry returns to the community build. Docker installs see
the exact registry login and digest-pinned compose commands for their license.

#### v5 License Migration

Pulse v6 uses the activation/grant model for active licensing, but it can migrate valid Pulse v5 paid JWT-style licenses, including legacy Pro and Lifetime licenses.

- If you upgrade an existing v5 instance and Pulse finds a persisted v5 license with no v6 activation state yet, v6 will try to auto-exchange it on startup.
- If auto-exchange cannot complete, your old key is left in place and the instance will prompt you to retry activation manually.
- In the v6 license panel, you can paste either:
  - a Pulse v6 activation key, or
  - a valid Pulse v5 paid license key, which Pulse will try to exchange automatically into the v6 activation model
- If the exchange service cannot complete the migration, retry from the v6 license panel or use the self-serve retrieval flow to fetch the current v6 activation key. Email is only a backup copy of that key.
- A migrated v5 key can be active on a limited number of v6 installations at
  a time (currently 3). v5 never counted installations, so if you run the
  same key on more instances than that, the extra instances will report that
  the key has reached its installation limit and will stay on Community.
  Retrying does not help; contact `support@pulserelay.pro` to release an
  installation you no longer use or to raise the limit.
- The exchanged v6 entitlement depends on the original cohort. Lifetime,
  active pre-cutover recurring Pro, and other migrated legacy paid installs do
  not all land on the same commercial continuity posture.
- Legacy recurring Pulse Pro subscriptions already active before the public v6 pricing cutover keep their grandfathered recurring price until cancellation. Self-hosted monitoring and child-resource volume are not metered under the current v6 policy. If they cancel and later return, current v6 pricing applies for paid features.

#### Paid Upgrade Truth Table

When an existing paid user asks what changes for them specifically, use this rule set:

- Legacy recurring Pulse Pro subscriptions from v5 or earlier that were already active before the public v6 pricing cutover keep their current recurring price while the subscription remains continuously active. Self-hosted monitoring and child-resource volume are not metered under the current v6 policy.
- Existing lifetime customers remain permanently valid, with self-hosted monitoring and child-resource volume not metered under the current v6 policy.
- Legacy paid v5 licenses migrated into v6 outside the recurring grandfathered path can still exchange into the v6 activation model without repurchasing. Migration records can preserve the original cohort for support and audit, but self-hosted monitoring volume is no longer the paid gate.
- Former recurring customers who already canceled, or who cancel and later return, do not resume the old grandfathered pricing automatically; they re-enter on current public v6 pricing for paid features while self-hosted monitoring remains included without a monitored-system volume gate.
- New self-hosted v6 purchases use the current Community / Pro plan model with core monitoring included; Relay is no longer sold.

If a self-hosted v6 install sees a new monitored-system, guest, or child-resource volume cap after moving to v6, treat that as a regression, not as expected upgrade behavior.

Practical recommendation:

- Before upgrading, keep console access available so you can retry activation from the v6 license panel if the exchange service is temporarily unavailable.

## Operational Trust migration

Pulse v6 consolidates alerts, Patrol attention, evidence, protection posture,
attached availability checks, notifications, and governed action verification
onto one Operational Trust lifecycle. See
[`OPERATIONAL_TRUST.md`](OPERATIONAL_TRUST.md) for the operator contract and
post-upgrade checks.

The migrations are additive:

- existing alert state is normalized into operational records and transitions;
- notification delivery keeps exact operational-record and transition links;
- recovery points and provider observations materialize provider-aware
  protection posture;
- unified-resource relationships and availability facets gain stable evidence
  linkage;
- action audit records preserve execution and verification as separate truth.

Supported legacy JSON fields remain readable where older clients need them,
but the primary v6 runtime has one writable owner for each domain. Server and
agent/client versions remain separate; a server upgrade does not establish
client convergence or every consuming surface's recovery.

Before the upgrade, keep the consistent [full-state backup](MIGRATION.md#full-state-recovery)
and confirm the existing service account can write the active stores without
widening its permissions just to diagnose a failure. After
startup, verify that the Patrol navigation count matches the active queue,
inspect one evidence/protection drill-down, and confirm stale collection does
not appear resolved. Pulse Pro users should also verify entitlement
connectivity before relying on restart offers.

### Multi-Tenant (Opt-In)

Multi-tenant mode is opt-in and additionally license-gated:

- Enablement flag: `PULSE_MULTI_TENANT_ENABLED=true`
- Capability gate: `multi_tenant`
