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

Once your instance is running, install agents on your infrastructure:

```bash
curl -fsSL https://yourname.cloud.pulserelay.pro/install.sh | \
  bash -s -- --url https://yourname.cloud.pulserelay.pro --token <api-token>
```

Generate installation commands from **Settings → Infrastructure → Install on a host** in your cloud dashboard.

### 3. Add Proxmox / TrueNAS Connections

Add your Proxmox VE, PBS, PMG, or TrueNAS systems via **Settings → Infrastructure → Platform connections**.

### 4. Set Up Mobile Access

Relay is enabled by default on Cloud instances. Open **Settings → Relay** to prepare pairing and connect once mobile beta/public access is enabled.

## Data & Privacy

- Your monitoring data runs in an **isolated container** — no shared databases.
- Data is stored encrypted at rest.
- Backups are automated and encrypted.
- You can **export** your configuration at any time via **Settings → System → Recovery** and migrate to self-hosted if needed.
- See [Privacy](PRIVACY.md) for full details.

## Billing

Pulse Cloud billing is handled by Stripe. You can manage your subscription from the Cloud portal:

- View current plan and usage
- Update payment method
- Cancel or change plans

## Migrating To/From Cloud

### Self-Hosted → Cloud

1. **Export** from your self-hosted instance: **Settings → System → Recovery → Create Backup**.
2. **Import** into your Cloud instance: **Settings → System → Recovery → Restore Configuration**.
3. Update agent `--url` flags to point to your cloud URL.

### Cloud → Self-Hosted

1. **Export** from Cloud: **Settings → System → Recovery → Create Backup**.
2. Install Pulse on your own server (see [Install Guide](INSTALL.md)).
3. **Import** the backup.
4. Re-activate your license key (if switching to Pro self-hosted).
5. Update agent `--url` flags.

See [Migration Guide](MIGRATION.md) for detailed steps.

## FAQ

### Can I use my own domain?

Custom domain support is planned for a future release. Currently, instances use `*.cloud.pulserelay.pro` subdomains.

### Is my data shared with other users?

No. Each Cloud account runs in a dedicated, isolated container with its own data directory.

### What happens if I cancel?

Your data is retained for 30 days after cancellation. You can export your configuration at any time before deletion.

### Can I switch between Cloud and self-hosted?

Yes. Use the export/import workflow described above. Your monitoring configuration is fully portable.

## See Also

- [Plans & Entitlements](PULSE_PRO.md) — feature comparison across Community, Relay, Pro, legacy Pro+, and Cloud
- [Installation (Self-Hosted)](INSTALL.md) — self-hosted installation guide
- [Relay / Mobile Access](RELAY.md) — relay setup and mobile rollout status (pre-configured on Cloud)
- [Multi-Tenant](MULTI_TENANT.md), Enterprise/internal multi-organization mode
