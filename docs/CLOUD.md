# Pulse Cloud (Hosted)

Pulse Cloud is the hosted version of Pulse. It is a fully managed monitoring instance that runs in the cloud so you don't have to self-host.

Pulse Cloud is for a hosted Pulse instance. Pulse MSP is a separate provider path: the MSP normally runs a Stripe-free provider-hosted control plane with one isolated Pulse runtime per client. Pulse-hosted MSP is available only as a request-assisted option where Pulse operates that provider stack for the MSP.

## How It Works

1. **Sign up** at the Pulse Cloud portal.
2. **Connect your agents** — install the Pulse agent on your infrastructure pointing to your cloud URL.
3. **Monitor** — access your dashboard from any browser or supported Pulse Mobile client.

Each Cloud account gets a dedicated, isolated Pulse instance with its own subdomain (e.g., `yourname.cloud.pulserelay.pro`).

## Features

Pulse Cloud includes everything in the **Pro** plan, plus:

| Feature | Description |
|---|---|
| **Fully managed hosting** | No server to manage, no updates to apply |
| **Automatic updates** | Your instance is always on the latest version |
| **Automatic backups** | Daily encrypted backups with 7-day retention |
| **Dedicated instance** | Your data runs in an isolated container — not shared with other tenants |
| **Wildcard TLS** | HTTPS with auto-renewing certificates |
| **Mobile ready** | Relay is pre-configured for secure Pulse Mobile remote access |

### Cloud Enterprise (Add-On)

For organisations that need internal multi-organization management under one owner:

| Feature | Capability Key |
|---|---|
| Multi-Tenant Mode | `multi_tenant` |
| Multi-User Mode | `multi_user` |
| Hosted Capacity Policy | `unlimited` |

See [Plans & Entitlements](PULSE_PRO.md) for the full feature matrix.

Cloud Enterprise shared-process organizations are for one owner separating internal sites, departments, or environments. They are not the default MSP model for unrelated customer businesses.

## Getting Started

### 1. Create Your Account

Sign up via the Pulse Cloud portal. Your instance is provisioned automatically after checkout.

### 2. Connect Agents

Once your instance is running, open **Settings → Infrastructure → Install on a
host** in your cloud dashboard and choose the target platform and collector
profile. Run the installer on the host being monitored, not inside the hosted
Pulse server.

For manual setup, follow the [private-file installation steps](UNIFIED_AGENT.md#private-file-installation-linux-macos-and-nas)
(or the Windows section in that guide), replacing `https://pulse.example.com`
with your instance's HTTPS address in both the download and install commands.
Save the agent token in a private file and pass only its path with
`--token-file`; never paste a token into command arguments, a URL or a report.
Download the agent installer from your own instance's `/install.sh`, stop if
the download fails, and inspect the saved script before running it. Keep TLS
verification enabled; do not pipe an unchecked response into a privileged shell
or substitute the top-level GitHub server installer.

### 3. Add Proxmox / TrueNAS Connections

Add your Proxmox VE, PBS, PMG, or TrueNAS systems via **Settings → Infrastructure → Platform connections**.

### 4. Set Up Mobile Access

Relay is enabled by default on Cloud instances. Open **Settings → Pulse Mobile** to pair a phone. Pulse Mobile is being retired on 31 March 2027.

## Data & Privacy

- Your monitoring data runs in an **isolated container** — no shared databases.
- Data is stored encrypted at rest.
- Backups are automated and encrypted.
- **Create Backup** in **Settings → System → Recovery** downloads an encrypted
  configuration export, not a complete copy of the hosted installation. Check
  the [migration scope](#migrating-tofrom-cloud) before relying on it for a move.
- See [Privacy](PRIVACY.md) for full details.

## Billing

Pulse Cloud billing is handled by Stripe. You can manage your subscription from the Cloud portal:

- View current plan and usage
- Update payment method
- Cancel or change plans

## Migrating To/From Cloud

**Create Backup transfers only the included configuration, not the full
installation.** Metrics and audit history, incidents and queued notifications,
server-side agent inventory and enrolment state, profiles and assignments are
not included. Neither are TrueNAS, vSphere and Machine Availability connections,
local login credentials or deployment overrides. Contents depend on the
exporting version; see the [configuration-transfer scope](MIGRATION.md#configuration-transfer)
for the complete account. Imported API-token records do not reveal the original
tokens or establish that every agent is admitted.

Hosted automatic backups and this downloaded export are different things. If
you need excluded state, retain the source and establish a recoverable,
[consistent full-state backup](MIGRATION.md#full-state-recovery) before moving.
Do not assume access to the hosted filesystem or that a hosted backup can be
restored into your self-hosted installation. Do not mix encrypted files and keys
from different installations. Keep exports, passphrases and credentials private.

Import can activate monitoring and notification settings. Use an isolated trial
where available; if the hosted destination cannot be isolated, arrange a
single-active cutover before importing and retain independent monitoring during
the pause. Browser administrator access does not grant hosted filesystem or
service-control access. If a necessary hosting-side operation is unavailable,
stop before importing rather than cancel the source or run two connected copies.

### Self-Hosted → Cloud

1. Keep the self-hosted source intact and preserve the separate full-state
   backup if you need its history or excluded state.
2. Set up destination-local administrator access on Cloud. Keep the trial
   destination isolated from monitored systems, agents and notification
   destinations so two instances do not send duplicate alerts or remote actions.
3. **Export** from the source through **Settings → System → Recovery → Create
   Backup**, using a strong, unique passphrase stored separately. If Cloud
   already has useful configuration, export it first. **Restore Configuration**
   replaces the included settings and API-token records; it does not merge them.
4. Re-create the excluded connections and settings you need. Retarget existing
   agents using the [address-change procedure](UNIFIED_AGENT.md#moving-pulse-to-a-new-address),
   not by uninstalling or re-enrolling them. It preserves the saved identity and
   credential; use the procedure supported by that host's platform.
5. Complete the [cutover checks](#verify-the-move-before-retiring-the-source)
   before retiring the self-hosted source.

### Cloud → Self-Hosted

1. Keep the Cloud instance available until validation is complete; do not
   cancel it or discard the source merely because an export succeeded. Establish
   recovery for any excluded state you need before proceeding.
2. Install Pulse on your own server using the [Install Guide](INSTALL.md).
   Set up destination-local administrator access and keep the trial isolated
   from monitored systems, agents and notification destinations.
3. **Export** from Cloud through **Settings → System → Recovery → Create
   Backup**. Keep its passphrase separately and privately. Export any existing
   destination configuration first, then use **Restore Configuration** there;
   import replaces the included settings and API-token records, not a merge.
4. Re-create excluded connections and settings, and verify your Pro licence
   separately if using Pro self-hosted. Retarget existing agents with the
   [address-change procedure](UNIFIED_AGENT.md#moving-pulse-to-a-new-address),
   preserving their identity and private credential rather than creating a
   replacement token just for the move.
5. Complete the checks below before retiring the Cloud source.

### Verify the move before retiring the source

- Verify destination-local login, imported SSO mappings and the new callback
  address. Imported settings do not override every deployment-managed value.
- Keep HTTPS verification enabled for both the new installer download and the
  agent connection. An old endpoint's custom CA or certificate pin is not
  automatically trust for the new endpoint; follow the retargeting guide.
- Check fresh ordinary reports and agent admission at the destination, not
  just cached rows or a successful import. Re-create profiles and assignments
  before enabling remote actions. Missing history after configuration-only
  transfer does not prove collection failed.
- Check every expected platform and your existing notification delivery
  evidence without running two instances against the same destinations. Do
  not induce alerts, replay a queue or run a workload action merely to test the
  move; importing settings does not transfer queued notifications.
- If an import reports a reload/apply failure, settings may already have been
  written. Preserve both instances and inspect the error before repeating it.
  If validation fails, stop the trial destination before returning traffic to
  the intact source, check agent URLs and avoid two active writers.

See the [Migration Guide](MIGRATION.md#cut-over-and-verify) for the complete
cutover and recovery procedure. A successful configuration import is not proof
of full recovery.

## FAQ

### Can I use my own domain?

Custom domain support is planned for a future release. Currently, instances use `*.cloud.pulserelay.pro` subdomains.

### Is my data shared with other users?

No. Each Cloud account runs in a dedicated, isolated container with its own data directory.

### What happens if I cancel?

Your data is retained for 30 days after cancellation. You can export your configuration at any time before deletion.

### Can I switch between Cloud and self-hosted?

You can transfer the included configuration using the workflow above. It is
not a full installation copy: history, some connections and agent enrolment
state need separate handling. Verify the destination before retiring the source.

## See Also

- [Plans & Entitlements](PULSE_PRO.md) — feature comparison across Community, Relay, Pro, legacy Pro+, and Cloud
- [Installation (Self-Hosted)](INSTALL.md) — self-hosted installation guide
- [Relay / Mobile Access](RELAY.md) — relay setup and mobile rollout status (pre-configured on Cloud)
- [Multi-Tenant](MULTI_TENANT.md), Enterprise/internal multi-organization mode
