# Pulse AI

Pulse AI adds an optional assistant for troubleshooting and proactive monitoring. It is **off by default** and can be enabled per instance.

## Immediate Value

Pulse AI Patrol monitors your infrastructure 24/7 and alerts you to issues that matter:

### What Patrol Catches

| Issue | Severity | Example |
|-------|----------|---------|
| **Node offline** | Critical | Proxmox node not responding |
| **Disk filling up** | Warning/Critical | Storage at 85%+ capacity |
| **Backup failures** | Warning | PBS job failed, no backup in 48+ hours |
| **Service down** | Critical | Docker container crashed, agent offline |
| **High resource usage** | Warning | Sustained memory >90%, CPU >85% |
| **Storage issues** | Critical | PBS datastore errors, ZFS problems |

### What Patrol Ignores (by design)

Patrol is **intentionally conservative** to avoid noise:

- Small baseline deviations ("CPU at 15% vs typical 10%")
- Low utilization that's "elevated" but fine (disk at 40%)
- Stopped VMs/containers that were intentionally stopped
- Brief spikes that resolve on their own
- Anything that doesn't require human action

> **Philosophy**: If a finding wouldn't be worth waking someone up at 3am, Patrol won't create it.

## Features

- **Interactive chat**: Ask questions about current cluster state and get AI-assisted troubleshooting.
- **Patrol**: Background checks every 15 minutes (configurable) that generate findings.
- **Alert analysis**: Optional token-efficient analysis when alerts fire.
- **Command execution**: When enabled, AI can run commands via connected agents.
- **Finding management**: Dismiss, resolve, or suppress findings to prevent recurrence.
- **Cost tracking**: Tracks token usage and supports monthly budget limits.

## Configuration

Configure in the UI: **Settings → AI**

AI settings are stored encrypted at rest in `ai.enc` under the Pulse config directory (`/etc/pulse` for systemd installs, `/data` for Docker/Kubernetes).

### Supported Providers

- **Anthropic** (API key or OAuth)
- **OpenAI**
- **DeepSeek**
- **Google Gemini**
- **Ollama** (self-hosted, with tool/function calling support)
- **OpenAI-compatible base URL** (for providers that implement the OpenAI API shape)

### Models

Pulse uses model identifiers in the form: `provider:model-name`

You can set separate models for:
- Chat (`chat_model`)
- Patrol (`patrol_model`)
- Auto-fix remediation (`auto_fix_model`)

### Testing

- Test provider connectivity: `POST /api/ai/test` and `POST /api/ai/test/{provider}`
- List available models: `GET /api/ai/models`

## Patrol Service

Patrol runs automated health checks on a configurable schedule (default: 15 minutes). It analyzes:

- Proxmox nodes, VMs, and containers
- PBS backup status and datastore health
- Host agent metrics (RAID, sensors, services)
- Docker/Podman containers
- Kubernetes clusters
- Resource utilization trends

### Finding Severity

- **Critical**: Immediate attention required (service down, data at risk)
- **Warning**: Should be addressed soon (disk filling, backup stale)

Note: `info` and `watch` level findings are filtered out to reduce noise.

### Managing Findings

Findings can be managed via the UI or API:

- **Get help**: Chat with AI to troubleshoot the issue
- **Resolve**: Mark as fixed (finding will reappear if the issue resurfaces)
- **Dismiss**: Mark as expected behavior (creates suppression rule)

Dismissed and resolved findings persist across Pulse restarts.

### AI-Assisted Remediation

When chatting with AI about a patrol finding, the AI can:
- Run diagnostic commands on connected agents
- Propose fixes with explanations
- Automatically resolve findings after successful remediation

## Safety Controls

Pulse includes settings that control how "active" AI features are:

- **Autonomous mode**: When enabled, AI may execute safe commands without approval.
- **Patrol auto-fix**: Allows patrol to attempt automatic remediation.
- **Alert-triggered analysis**: Limits AI to analyzing specific events when alerts occur.

If you enable execution features, ensure agent tokens and scopes are appropriately restricted.

## Troubleshooting

| Issue | Solution |
|-------|----------|
| AI not responding | Verify provider credentials in **Settings → AI** |
| No execution capability | Confirm at least one agent is connected |
| Findings not persisting | Check Pulse has write access to config directory |
| Too many findings | This shouldn't happen - please report if it does |

