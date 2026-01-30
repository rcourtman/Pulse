# Pulse AI

Pulse Pro unlocks **AI Patrol** for continuous, automated health checks. Learn more at <https://pulserelay.pro> or see the technical overview in [PULSE_PRO.md](PULSE_PRO.md).

## What Patrol Actually Does (Technical)

Patrol is a scheduled analysis pipeline that builds a rich, system-wide snapshot and produces actionable findings.

**Inputs (real data, not guesses):**
- Live state: nodes, VMs/CTs, storages, backups, docker, kubernetes.
- Recent metrics history (trends + rate of change).
- Alert state and recent failures.
- Diagnostics (connectivity, permissions, agent status).

**Enrichment:**
- Normalizes metrics across resource types.
- Applies context rules (e.g., ignore idle test nodes, de-noise transient blips).
- Correlates related issues to avoid duplicate noise.

**Outputs:**
- Findings with severity, category, and remediation hints.
- Summary stats for coverage and risk level.
- Optional AI analysis on the highest impact issues (Pro).

## Why Patrol Is Different From Traditional Alerts

Alerts are threshold-based and narrow. Patrol is context-based and cross-system.

- **Alerts**: "Disk > 90%."  
- **Patrol**: "ZFS pool is 86% but trending +4%/day; projected to hit 95% within a week. Largest consumer is datastore X. Recommend prune or expand."

## Examples of Patrol Findings (Realistic)

- Backup jobs succeeded but datastore usage jumped 20% since last run.
- Node health is OK but cluster clock drift is growing.
- Multiple VMs in the same storage pool are all retrying snapshots.
- Docker host is healthy but containers in one stack are flapping.

## Controls and Limits

- **Schedule**: from 10 minutes to 7 days (default 6 hours).
- **Scope**: only configured resources and connected agents.
- **Safety**: command execution remains disabled by default.
- **Cost control**: Pro uses model selection and rate limits; free tier uses heuristic-only Patrol.

## Privacy

Patrol runs on your server and only sends the minimal context needed for analysis to the configured provider (when Pro is enabled). No telemetry is sent to Pulse by default.

Pulse AI adds an optional assistant for troubleshooting and proactive monitoring. It is **off by default** and can be enabled per instance.

## What Makes AI Patrol Different

Unlike chatting with a generic AI where you manually describe your infrastructure, Patrol runs automatically and sees **your entire infrastructure at once** - every node, VM, container, storage pool, backup job, and Kubernetes cluster. It's not just a static checklist; it's an LLM analyzing real-time data enriched with historical context.

### Context Patrol Receives (That Generic LLMs Can't See)

Every patrol run passes the LLM comprehensive context about your environment:

| Data Category | What's Included |
|---------------|-----------------|
| **Proxmox Nodes** | Status, CPU%, memory%, uptime, 24h/7d trend analysis |
| **VMs & Containers** | Full metrics, backup status, OCI images, historical trends, anomaly flags |
| **Storage Pools** | Usage %, capacity predictions, type (ZFS/LVM/Ceph), growth rates |
| **Docker/Podman** | Container counts, health states, unhealthy container lists |
| **Kubernetes** | Nodes, pods, deployments, services, DaemonSets, StatefulSets, namespaces |
| **PBS/PMG** | Datastore status, backup jobs, job failures, verification status |
| **Ceph** | Cluster health, OSD states, PG status |
| **Agent Hosts** | Load averages, memory, disk, RAID status, temperatures |

### Enriched Context (The Real Differentiator)

Beyond raw metrics, Patrol enriches the context with intelligence that transforms raw data into actionable insights:

- **Trend analysis** - 24h and 7d patterns showing `growing`, `stable`, `declining`, or `volatile` behavior
- **Learned baselines** - Z-score anomaly detection based on what's *normal for your environment*
- **Capacity predictions** - "Storage pool will be full in 12 days at current growth rate"
- **Infrastructure changes** - Detected config changes, VM migrations, new deployments  
- **Resource correlations** - Pattern detection across related resources (e.g., containers on same host)
- **User notes** - Your annotations explaining expected behavior ("runs hot for transcoding")
- **Dismissed findings** - Respects your feedback and suppressed alerts
- **Incident memory** - Learns from past investigations and successful remediations

### Examples of What Patrol Catches

Because it's an LLM with full context, Patrol catches issues that static threshold-based alerting misses:

| Issue | Severity | Example |
|-------|----------|---------|
| **Node offline** | Critical | Proxmox node not responding |
| **Disk approaching capacity** | Warning/Critical | Storage at 85%+, or growing toward full |
| **Backup failures** | Warning | PBS job failed, no backup in 48+ hours |
| **Service down** | Critical | Docker container crashed, agent offline |
| **High resource usage** | Warning | Sustained memory >90%, CPU >85% |
| **Storage issues** | Critical | PBS datastore errors, ZFS pool degraded |
| **Ceph problems** | Warning/Critical | Degraded OSDs, unhealthy PGs |
| **Kubernetes issues** | Warning | Pods stuck in Pending/CrashLoopBackOff |
| **Restart loops** | Warning | VMs that keep restarting without errors |
| **Clock drift** | Warning | Node time drift affecting Ceph/HA |
| **Unusual patterns** | Varies | Any anomaly the LLM identifies as unusual for your setup |

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
- **Patrol**: Background checks periodically (default: 6 hours) that generate findings. Interval is configurable from 10 minutes to 7 days, or set to 0 to disable.
- **Alert-triggered analysis (Pro)**: Optional token-efficient analysis when alerts fire.
- **Kubernetes AI analysis (Pro)**: Deep cluster analysis beyond basic monitoring.
- **Command execution**: When enabled, AI can run commands via connected agents.
- **Finding management**: Dismiss, resolve, or suppress findings to prevent recurrence.
- **Cost tracking**: Tracks token usage and supports monthly budget limits.

Alert-triggered analysis runs attach a timeline event to the alert, so investigations remain auditable alongside acknowledgements and remediation steps.

> **License note**: Kubernetes AI analysis is gated by the `kubernetes_ai` Pulse Pro feature.

## Pulse Assistant (Chat): How It Works

Pulse Assistant is **tool-driven**. It does not "guess" system state — it calls live tools and reports their outputs.

### The Model's Workflow (Discover → Investigate → Act)
- **Discover**: Uses `pulse_query` (or `pulse_discovery`) to find real resources and IDs.
- **Investigate**: Uses `pulse_read` to run bounded, read-only commands and check status/logs.
- **Act** (optional): Uses `pulse_control` for changes, then verifies with a read.

### Safety Gates That Make It Trustworthy
- **Strict Resolution (optional)**: When enabled, the assistant must discover a resource before it can act on it. This prevents fabricated IDs.
- **Read/Write separation**: Read-only commands go through `pulse_read`; write actions go through `pulse_control`. This keeps the workflow state machine honest.
- **Verification after writes**: After any write, the assistant must perform a read check before it can finish the response.
- **Non‑interactive guardrails**: Commands that could hang (e.g., `tail -f`) are rewritten into bounded, safe forms.
- **Approval mode**: In Controlled mode, every write requires explicit user approval. Autonomous mode is available only with Pro.

### What You See As a User
- **Clear tool usage**: Each step shows which tool ran and what it returned.
- **Structured recovery**: If a tool is blocked, the assistant adapts (e.g., runs discovery, switches tools, or asks for approval).
- **Verified outcomes**: Changes are followed by a read check before the assistant claims success.

## Why It's Impressive (and Reliable)

Pulse Assistant behaves like a careful operator:
- It **grounds answers in live data** instead of assumptions.
- It **adapts** when guardrails block an action.
- It **verifies** changes before reporting success.
- It **keeps you in control** with explicit approval gates.

## Configuration

Configure in the UI: **Settings → System → AI Assistant**

AI settings are stored encrypted at rest in `ai.enc` under the Pulse config directory. Patrol findings and history are stored in `ai_findings.json`, `ai_patrol_runs.json`, and usage data in `ai_usage_history.json`. These files are located in `/etc/pulse` for systemd installs, or `/data` for Docker/Kubernetes.

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

## Model Matrix (Pulse Assistant)

This table summarizes the most recent **Pulse Assistant** eval runs per model. Patrol is still in development and is not scored yet.
Time/tokens reflect the combined **Smoke + Read-only** matrix run.
Transient provider errors (rate limits, unavailable chat endpoints) are skipped when rendering the table.

Update the table from eval reports:
```
EVAL_REPORT_DIR=tmp/eval-reports go run ./cmd/eval -scenario matrix -auto-models
python3 scripts/eval/render_model_matrix.py tmp/eval-reports --write-doc docs/AI.md
```
Or use the helper script:
```
scripts/eval/run_model_matrix.sh
```

<!-- MODEL_MATRIX_START -->
| Model | Smoke | Read-only | Time (matrix) | Tokens (matrix) | Last run (UTC) |
| --- | --- | --- | --- | --- | --- |
| anthropic:claude-3-haiku-20240307 | ✅ | ❌ | 2m 42s | — | 2026-01-29 |
| anthropic:claude-haiku-4-5-20251001 | ✅ | ✅ | 8s | 18,923 | 2026-01-29 |
| anthropic:claude-opus-4-5-20251101 | ✅ | ✅ | 9m 31s | 1,120,530 | 2026-01-29 |
| gemini:gemini-3-flash-preview | ✅ | ✅ | 7m 4s | — | 2026-01-29 |
| gemini:gemini-3-pro-preview | ✅ | ✅ | 3m 54s | 1,914 | 2026-01-29 |
| openai:gpt-5.2 | ✅ | ✅ | 5s | 12,363 | 2026-01-29 |
| openai:gpt-5.2-chat-latest | ✅ | ✅ | 8s | 12,595 | 2026-01-29 |
<!-- MODEL_MATRIX_END -->

### Testing

- Test provider connectivity: `POST /api/ai/test` and `POST /api/ai/test/{provider}`
- List available models: `GET /api/ai/models`

## Patrol Service (Pro Feature)

Patrol runs automated health checks on a configurable schedule (default: every 6 hours). It passes comprehensive infrastructure context to the LLM (see "Context Patrol Receives" above) and generates findings when issues are detected.

Pulse Pro users get full LLM-powered analysis. Free users still benefit from **Heuristic Patrol**, which uses local rule-based logic to detect common issues (offline nodes, disk exhaustion, etc.) without requiring an external AI provider. Free users also get full access to the AI Chat assistant (BYOK).

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

- **Autonomous mode (Pro)**: When enabled, AI may execute safe commands without approval.
- **Patrol auto-fix (Pro)**: Allows patrol to attempt automatic remediation.
- **Alert-triggered analysis (Pro)**: Limits AI to analyzing specific events when alerts occur.

If you enable execution features, ensure agent tokens and scopes are appropriately restricted.

### Control Levels

Pulse uses three AI permission levels for infrastructure control:

- **Read-only**: AI can observe and query data only.
- **Controlled**: AI asks for approval before executing commands or control actions.
- **Autonomous (Pro)**: AI executes actions without prompting.

### Using Approvals (Controlled Mode)

When control level is **Controlled**, write actions pause for approval:

- In chat, you’ll see an approval card with the proposed command.
- **Approve** to execute and verify the change, or **Deny** to cancel it.
- Only users with admin privileges can approve/deny.

### Advanced Network Restrictions

Pulse blocks AI tool HTTP fetches to loopback and link-local addresses by default. For local development, you can allow loopback targets:

- `PULSE_AI_ALLOW_LOOPBACK=true`

Use this only in trusted environments.

## Troubleshooting

| Issue | Solution |
|-------|----------|
| AI not responding | Verify provider credentials in **Settings → System → AI Assistant** |
| No execution capability | Confirm at least one agent is connected |
| Findings not persisting | Check Pulse has write access to `ai_findings.json` in the config directory |
| Too many findings | This shouldn't happen - please report if it does |
