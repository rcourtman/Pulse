# Privacy

Pulse is designed to run locally. By default, your monitoring data stays on your server.

## Usage Data

Pulse has one outbound usage-data scope: **anonymous outbound telemetry** to
help me understand active installations, release uptake, and feature use in
aggregate.

Commercial activation and license-recovery runtime records stay on the Pulse
instance where they were created. They are not exported to Pulse infrastructure,
third-party analytics, support diagnostics, or ordinary Settings surfaces.

### Anonymous outbound telemetry

Pulse includes anonymous outbound telemetry that is **enabled by default**. It sends a lightweight ping on startup and once every 24 hours to help me understand how many active installations exist, which releases are actually deployed, and which features are in use.

No hostnames, credentials, infrastructure identifiers, IP addresses, prompts, chat messages, or personally identifiable information is ever sent. See the full field list below.

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
| Agent hosts | `4` | Understand Pulse Agent adoption for node-local telemetry in aggregate |
| Docker hosts | `2` | Understand Docker monitoring adoption in aggregate |
| Docker containers | `18` | Understand Docker / Podman workload scale in aggregate |
| Kubernetes clusters | `0` | Understand Kubernetes monitoring adoption in aggregate |
| Kubernetes nodes | `3` | Understand Kubernetes node scale in aggregate |
| Kubernetes pods | `42` | Understand Kubernetes workload scale in aggregate |
| Kubernetes deployments | `8` | Understand Kubernetes deployment adoption in aggregate |
| Storage pools | `6` | Understand storage monitoring adoption in aggregate |
| Physical disks | `24` | Understand disk-health monitoring adoption in aggregate |
| Ceph clusters | `1` | Understand Ceph monitoring adoption in aggregate |
| Network shares | `5` | Understand NAS/share monitoring adoption in aggregate |
| TrueNAS systems | `1` | Understand TrueNAS integration adoption in aggregate |
| TrueNAS VMs | `2` | Understand TrueNAS VM visibility adoption in aggregate |
| TrueNAS apps | `7` | Understand TrueNAS app visibility adoption in aggregate |
| VMware hosts | `3` | Understand VMware vSphere host monitoring adoption in aggregate |
| VMware VMs | `35` | Understand VMware vSphere VM monitoring adoption in aggregate |
| VMware datastores | `4` | Understand VMware datastore visibility adoption in aggregate |
| Availability targets | `9` | Understand agentless availability-check adoption in aggregate |
| AI enabled | `true`/`false` | See whether AI features are actually used before expanding or removing them |
| Patrol enabled | `true`/`false` | See whether proactive AI health patrol is used |
| Discovery enabled | `true`/`false` | See whether network or AI-assisted discovery is used |
| Notifications enabled | `true`/`false` | See whether alert notification delivery is configured |
| AI actions enabled | `true`/`false` | See whether AI control tools are enabled without sending action history or command content |
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

- **AI providers**: when AI features are configured, Pulse sends only the context required for your request to the provider you chose. Local providers stay on your network; non-local providers such as OpenAI or Anthropic receive provider-bound context directly from your Pulse instance. AI prompts from self-managed installs do not transit Pulse infrastructure. Before non-local model requests leave the instance, governed resource details use the same resource-policy redaction shown in Data Handling: local-only resource details are omitted from detailed prompt sections or replaced with policy-safe summaries, and known restricted resource identifiers are redacted where they appear in provider-bound context. See `docs/AI.md`.
- **Relay / Remote Access**: when relay is enabled, Pulse connects to the configured relay endpoint to enable secure remote web access, Pulse Mobile pairing for handoff, and push notifications. See Settings → Remote Access.
- **Update checks**: Pulse can check for new releases/updates (for example via GitHub release metadata) depending on your deployment and configuration.
