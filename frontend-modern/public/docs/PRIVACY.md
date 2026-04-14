# Privacy

Pulse is designed to run locally. By default, your monitoring data stays on your server.

## Usage Data

Pulse currently has two usage-data scopes:

1. **Anonymous outbound telemetry** to help me understand active installations, release uptake, and feature use in aggregate.
2. **Local-only upgrade events** to help debug and improve upgrade flows on the instance where they were recorded.

These are separate scopes on purpose. The outbound telemetry path stays coarse and anonymous. The local-only upgrade-event path stays on the Pulse instance and is not exported to third parties.

### Anonymous outbound telemetry

Pulse includes anonymous outbound telemetry that is **enabled by default**. It sends a lightweight ping on startup and once every 24 hours to help me understand how many active installations exist, which releases are actually deployed, and which features are in use.

No hostnames, credentials, IP addresses, or personally identifiable information is ever sent. See the full field list below.

#### How to disable

- **Settings → System → General → Anonymous outbound telemetry** (toggle off), or
- Set the environment variable `PULSE_TELEMETRY=false`

#### How to inspect or rotate it

- **Settings → System → General → Preview payload** shows the exact heartbeat JSON Pulse would send with the current runtime state.
- **Settings → System → General → Reset ID** immediately rotates the local telemetry install ID and refreshes the previewed payload.
- If telemetry is currently disabled, the preview still shows the payload Pulse would send if you enable it.

#### Exactly what is sent

Every field is listed below with the reason it exists — nothing else leaves your server:

| Field | Example | Purpose |
|-------|---------|---------|
| Install ID | `a1b2c3d4-...` | Distinguish active installations within one rotation window without tying telemetry to an account or person |
| Version | `6.0.0-rc.1` | Track the canonical release identity currently deployed |
| Version raw | `v6.0.0-rc.1-45-gabcdef` | Preserve the original build string when it differs so manual/dev builds do not pollute release reporting |
| Version channel | `stable`, `rc`, `dev` | Distinguish published stable/RC assets from development or prerelease builds |
| Version build | `git.45.gabcdef` | Preserve build metadata for git-describe and other non-release builds |
| Version is development | `true`/`false` | Mark manual or source-built development installs explicitly |
| Version is published release | `true`/`false` | Mark whether the running build matches a published stable or RC release asset |
| Platform | `docker` or `binary` | Understand whether runtime behavior differs between container and non-container installs |
| OS | `linux` | See whether operating-system-specific issues exist |
| Arch | `amd64` | See whether CPU-architecture-specific issues exist |
| Event | `startup` or `heartbeat` | Distinguish first-run/session starts from daily active-install heartbeats |
| PVE nodes | `3` | Understand Proxmox VE deployment size in aggregate |
| PBS instances | `1` | Understand Proxmox Backup Server adoption in aggregate |
| PMG instances | `0` | Understand Proxmox Mail Gateway adoption in aggregate |
| VMs | `25` | Understand approximate infrastructure scale in aggregate |
| Containers | `12` | Understand approximate LXC usage in aggregate |
| Docker hosts | `2` | Understand Docker monitoring adoption in aggregate |
| Kubernetes clusters | `0` | Understand Kubernetes monitoring adoption in aggregate |
| AI enabled | `true`/`false` | See whether AI features are actually used before expanding or removing them |
| Active alerts | `4` | Understand how noisy or quiet installations are in aggregate |
| Relay enabled | `true`/`false` | See whether remote-access features are being used |
| SSO enabled | `true`/`false` | See whether single-sign-on support is being used |
| Multi-tenant | `true`/`false` | See whether multi-tenant/runtime-org features are being used |
| Paid license | `true`/`false` | Distinguish free from paid posture without sending the exact commercial tier |
| Has API tokens | `true`/`false` | See whether token-based automation/integration is being used without sending token counts |

#### Server-side handling and retention

- Telemetry pings are stored on the Pulse license server only for aggregate install/use analysis.
- The license server stores only the same coarse telemetry fields listed above; it does not expand them into exact commercial tiers or exact API-token counts.
- Telemetry rows older than **90 days** are purged automatically.
- The license server uses client IP addresses transiently for abuse/rate limiting, but it does **not** store IP addresses in telemetry rows.

#### What is NOT sent

- No IP addresses are stored in telemetry rows
- No hostnames, node names, VM names, or any infrastructure identifiers
- No Proxmox credentials, API tokens, or passwords
- No alert content, AI prompts, or chat messages
- No personally identifiable information of any kind

#### Install ID rotation

The telemetry install ID is pseudonymous and rotates automatically every 30 days.
Pulse keeps it only to avoid treating every startup ping as a brand-new install
while still limiting long-term linkage from one heartbeat window to the next.
Operators can also rotate it immediately from **Settings → System → General → Reset ID**.

#### Source code

The telemetry implementation is in [`internal/telemetry/telemetry.go`](../internal/telemetry/telemetry.go). You can read the `Ping` struct to see every field that is transmitted.

## No Third-Party Analytics

- There is no third-party analytics SDK in the frontend.
- Telemetry pings go only to the Pulse license server (`license.pulserelay.pro`), not to any third-party service.

## Optional Outbound Connections (Explicitly Enabled)

Pulse can make outbound connections when you enable specific features:

- **AI (BYOK)**: when AI features are enabled, Pulse sends only the context required for your request to the provider you configured (OpenAI, Anthropic, etc.). See `docs/AI.md`.
- **Relay / Remote Access**: when relay is enabled, Pulse connects to the configured relay endpoint to enable mobile access. See Settings → Remote Access.
- **Update checks**: Pulse can check for new releases/updates (for example via GitHub release metadata) depending on your deployment and configuration.

### Local-only upgrade events

Pulse can record local-only usage events such as "paywall viewed" or "trial started" to improve and debug in-app upgrade flows.

- These events are stored locally and are not exported to third parties.
- Disable via **Settings → System → General → Disable local-only upgrade events** or set:
  - `PULSE_DISABLE_LOCAL_UPGRADE_METRICS=true`

If you prefer fewer upgrade prompts, you can also enable:
- **Settings → System → General → Reduce Pro prompts**
