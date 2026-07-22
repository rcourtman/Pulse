# Upgrade to Pulse v6

This guide covers practical upgrade steps for existing Pulse installs moving to v6.

For the current stable v6 packet, see:

- `docs/releases/RELEASE_NOTES_v6.1.0.md`
- `docs/releases/V6_CHANGELOG_v6.1.0.md`

For earlier stable v6 packets and rollout references, see:

- `docs/releases/RELEASE_NOTES_v6.0.5.md`
- `docs/releases/V6_CHANGELOG_v6.0.5.md`
- `docs/releases/RELEASE_NOTES_v6.0.4.md`
- `docs/releases/V6_CHANGELOG_v6.0.4.md`
- `docs/releases/RELEASE_NOTES_v6.0.3.md`
- `docs/releases/V6_CHANGELOG_v6.0.3.md`
- `docs/releases/RELEASE_NOTES_v6.md`
- `docs/releases/V6_CHANGELOG.md`

The published GitHub release is the authority for what users can install as
stable. Keep v5.1.35 as the explicit rollback target for the v6.0.0 cutover.

## Before You Upgrade

- Create an encrypted config backup: **Settings → System → Recovery → Create Backup** (older versions labeled this **Backups**)
- Open **Settings → System → Updates** and review the upgrade checks on the update plan. Pulse checks the server update path, current agent continuity, and agent reporting token scope before you install. These checks describe the currently reported fleet; they do not prove every installed agent is online or already updated.
- Confirm you can access the host/container console (for rollback and bootstrap token retrieval)
- If you have any external integrations or scripts: review the **API Changes** section below

## Upgrade Paths

### systemd and Proxmox LXC installs

Preferred path:

- **Settings → System → Updates**

If you prefer CLI, use the installed update helper for the target version:

```bash
sudo /bin/update --version vX.Y.Z
```

`/bin/update` is installed by the supported systemd and Proxmox LXC server installer. If your host does not have it yet, follow the signed server-installer flow in [INSTALL.md](INSTALL.md). Agent updates and v5-to-v6 agent upgrades still use the `/install.sh` command generated in **Settings → Infrastructure → Install on a host**; that screen is for both first installs and in-place agent upgrades.

Operator note for builds after `v6.0.0-rc.2`: the historical Pulse update
signer was not recovered. Hosts pinned to the `rc.2` trust root should not
assume unattended continuity into newer prerelease or GA artifacts; plan a
manual reinstall or other explicit trust migration before testing those builds.

### Docker

```bash
docker pull rcourtman/pulse:vX.Y.Z
docker compose up -d
```

### Kubernetes (Helm)

```bash
helm repo update
helm upgrade pulse pulse/pulse -n pulse
```

## Post-Upgrade Checklist

- Confirm version: `GET /api/version`
- Confirm scheduler health: `GET /api/monitoring/scheduler/health`
- Confirm unified resources API is responding: `GET /api/resources`
- Confirm nodes are polling and no breakers are stuck open
- Confirm notifications still send (send a test)
- Confirm agents are connected (if used)

## v5 to v6 Operator FAQ

### Do I need to uninstall Pulse v5 first?

No. Upgrade the existing Pulse server installation in place.

### Do I need to uninstall my existing Pulse Unified Agents first?

No. Use the unified installer to upgrade existing agent deployments in place. Generate the current command from **Settings → Infrastructure → Install on a host**, then run it on the host that already has the v5 agent service. You do not need to remove the old service first.

### Does upgrading the Pulse server prove that all agents have upgraded?

No. The server and Unified Agent have separate update lifecycles. After the
server changes the target version, eligible v6 agents normally discover and
apply that update asynchronously during their update checks. The server being
current is not proof that every installed agent has checked in or converged.

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

Yes. Keep a rollback path available while you evaluate v6. The final release
on the v5 line is 5.1.36, so the stable rollback command is:

```bash
./scripts/install.sh --version v5.1.36
```

### Why did my v5 install upgrade itself to v6?

Pulse 5.1.29 and later pin the built-in updater to the 5.1.x line and never
offer v6, so upgrading from those versions is always a manual step. Pulse
5.1.28 and older have no such pin: installs with auto-update enabled follow
the newest stable GitHub release, which is now v6. If that happened to you,
your data and configuration carry over; run through the Post-Upgrade
Checklist above to confirm everything still works. To return to v5, run:

```bash
./scripts/install.sh --version v5.1.36
```

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
unambiguous. Attached checks disappear from the standalone Availability checks
inventory and appear on the owning platform row/detail instead. API consumers
should accept the additive `availabilityChecks`, correlation, evidence, and
`checks` relationship fields; the existing singular `availability` field
remains as a compatibility summary. Ambiguous or invalid links stay
standalone/unresolved and are never guessed.

### License and Entitlements

Pulse v6 feature gating is driven by the entitlements endpoint:

- `GET /api/license/entitlements`

For self-hosted v6, Pulse no longer sells monitored-system volume. Core
monitoring stays available across Community, Relay, and Pro, while Relay and
Pro sell convenience, history, AI operations, and advanced administration.
Relay raises history to 14 days, while Pro raises it to 90 days.

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
- New self-hosted v6 purchases use the current Community / Relay / Pro plan model with core monitoring included.

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
but the primary v6 runtime has one writable owner for each domain. Pulse Mobile
now reads the canonical Patrol attention queue. Operators should upgrade
desktop and mobile clients together when they depend on acknowledgement,
evidence, protection, or action-verification parity.

Before the upgrade, back up the Pulse data directory and confirm the service
account can write the alert, notification, recovery, and action stores. After
startup, verify that the Patrol navigation count matches the active queue,
inspect one evidence/protection drill-down, and confirm stale collection does
not appear resolved. Pulse Pro users should also verify entitlement
connectivity before relying on restart offers.

### Multi-Tenant (Opt-In)

Multi-tenant mode is opt-in and additionally license-gated:

- Enablement flag: `PULSE_MULTI_TENANT_ENABLED=true`
- Capability gate: `multi_tenant`

See any multi-tenant operational docs under `docs/architecture/` if you plan to run this mode.
