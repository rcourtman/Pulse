# Privacy

Pulse is designed to run locally. By default, your monitoring data stays on your server.

## Anonymous Telemetry

Pulse includes anonymous telemetry that is **enabled by default**. It sends a lightweight ping on startup and once every 24 hours to help the developer understand how many active installations exist and which features are in use.

No hostnames, credentials, IP addresses, or personally identifiable information is ever sent. See the full field list below.

### How to disable

- **Settings → System → General → Anonymous telemetry** (toggle off), or
- Set the environment variable `PULSE_TELEMETRY=false`

### Exactly what is sent

Every field is listed below — nothing else leaves your server:

| Field | Example | Purpose |
|-------|---------|---------|
| Install ID | `a1b2c3d4-...` | Random UUID generated locally, not tied to any account |
| Version | `6.0.0` | Pulse version |
| Platform | `docker` or `binary` | Deployment method |
| OS | `linux` | Operating system |
| Arch | `amd64` | CPU architecture |
| Event | `startup` or `heartbeat` | Whether this is a startup or daily ping |
| PVE nodes | `3` | Number of Proxmox VE nodes connected |
| PBS instances | `1` | Number of Proxmox Backup Server instances |
| PMG instances | `0` | Number of Proxmox Mail Gateway instances |
| VMs | `25` | Total VM count |
| Containers | `12` | Total LXC container count |
| Docker hosts | `2` | Number of Docker hosts monitored |
| Kubernetes clusters | `0` | Number of Kubernetes clusters |
| AI enabled | `true`/`false` | Whether AI features are turned on |
| Active alerts | `4` | Number of active alerts |
| Relay enabled | `true`/`false` | Whether remote access is enabled |
| SSO enabled | `true`/`false` | Whether OIDC/SSO is configured |
| Multi-tenant | `true`/`false` | Whether multi-tenant mode is on |
| License tier | `free`, `pro`, etc. | Current license tier |
| API tokens | `3` | Number of API tokens configured |

### What is NOT sent

- No IP addresses are stored server-side
- No hostnames, node names, VM names, or any infrastructure identifiers
- No Proxmox credentials, API tokens, or passwords
- No alert content, AI prompts, or chat messages
- No personally identifiable information of any kind

### Source code

The telemetry implementation is in [`internal/telemetry/telemetry.go`](../internal/telemetry/telemetry.go). You can read the `Ping` struct to see every field that is transmitted.

## No Third-Party Analytics

- There is no third-party analytics SDK in the frontend.
- Telemetry pings go only to the Pulse license server (`license.pulserelay.pro`), not to any third-party service.

## Optional Outbound Connections (Explicitly Enabled)

Pulse can make outbound connections when you enable specific features:

- **AI (BYOK)**: when AI features are enabled, Pulse sends only the context required for your request to the provider you configured (OpenAI, Anthropic, etc.). See `docs/AI.md`.
- **Relay / Remote Access**: when relay is enabled, Pulse connects to the configured relay endpoint to enable mobile access. See Settings → Remote Access.
- **Update checks**: Pulse can check for new releases/updates (for example via GitHub release metadata) depending on your deployment and configuration.

## Local Upgrade Metrics (Can Be Disabled)

Pulse can record local-only events such as "paywall viewed" or "trial started" to improve and debug in-app upgrade flows.

- These events are stored locally and are not exported to third parties.
- Disable via **Settings → System → General → Disable local upgrade metrics** or set:
  - `PULSE_DISABLE_LOCAL_UPGRADE_METRICS=true`

If you prefer fewer upgrade prompts, you can also enable:
- **Settings → System → General → Reduce Pro prompts**
