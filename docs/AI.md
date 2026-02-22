# Pulse AI

Pulse Patrol is available to everyone on the Community plan with BYOK (your own AI provider). Pro and Cloud unlock auto-fix and advanced analysis. Learn more at <https://pulserelay.pro> or see [PULSE_PRO.md](PULSE_PRO.md).

---

## Overview

Pulse includes two AI-powered systems:

1. **Pulse Assistant** — An interactive chat interface for ad-hoc troubleshooting, investigations, and infrastructure control.
2. **Pulse Patrol** — A scheduled, context-aware analysis service that continuously monitors your infrastructure, learns what's normal, predicts issues, and generates actionable findings.

Both systems are built on the same tool-driven architecture where the LLM acts as a proposer and Go code enforces safety gates.

### Not Just Another Chatbot

Pulse Assistant is a **protocol-driven, safety-gated agentic system** that:

- **Proactively gathers context** — understands resources before you ask (context prefetcher)
- **Learns within sessions** — extracts and caches facts to avoid redundant queries (knowledge accumulator)
- **Enforces workflow invariants** — FSM prevents dangerous state transitions
- **Supports parallel tool execution** — efficient batch operations with concurrency control
- **Detects and prevents hallucinations** — phantom execution detection
- **Auto-recovers from errors** — structured error envelopes enable self-correction

📖 **For a deep technical dive into the Assistant architecture, see [architecture/pulse-assistant-deep-dive.md](architecture/pulse-assistant-deep-dive.md).**

### Not Just Another Alerting System

Pulse Patrol is a **multi-layered intelligence platform** that:

- **Learns** what's normal for your environment (baseline engine)
- **Predicts** issues before they become critical (pattern detection + forecasting)
- **Correlates** events across your entire infrastructure (root cause analysis)
- **Remembers** past incidents and successful remediations (incident memory)
- **Investigates** issues autonomously when configured (investigation orchestrator)
- **Verifies** fixes and tracks remediation effectiveness (verification loops)

All while running entirely on your infrastructure with BYOK for complete privacy.

📖 **For a deep technical dive into the intelligence subsystems, see [architecture/pulse-patrol-deep-dive.md](architecture/pulse-patrol-deep-dive.md).**

See [architecture/pulse-assistant.md](architecture/pulse-assistant.md) for the original safety architecture documentation.

---

## Pulse Patrol

Patrol is a scheduled analysis pipeline that builds a rich, system-wide snapshot and produces actionable findings.

### How Patrol Works

```
Scheduled/Event Trigger
        │
        ▼
buildSeedContext()  ── infrastructure snapshot
        │
        ▼
LLM analysis (with tools) ← pulse_storage, pulse_metrics, pulse_alerts, etc.
        │
        ▼
DetectSignals() ── deterministic signal detection from tool outputs
        │
        ▼
createFinding() ── validated, deduplicated findings stored
        │
        ▼ (if configured)
MaybeInvestigateFinding() ── automatic investigation + remediation
```

### What Patrol Sees

Every patrol run passes the LLM comprehensive context about your environment:

| Data Category | What's Included |
|---------------|-----------------|
| **Proxmox Nodes** | Status, CPU%, memory%, uptime, 24h/7d trend analysis |
| **VMs & Containers** | Full metrics, backup status, OCI images, historical trends, anomaly flags |
| **Storage Pools** | Usage %, capacity predictions, type (ZFS/LVM/Ceph), growth rates |
| **Docker/Podman** | Container counts, health states, unhealthy container lists |
| **Kubernetes** | Nodes, pods, deployments, services, DaemonSets, StatefulSets, namespaces |
| **TrueNAS** | Pools, datasets, disk health, SMART status, replication, alerts |
| **PBS/PMG** | Datastore status, backup jobs, job failures, verification status |
| **Ceph** | Cluster health, OSD states, PG status |
| **Agent Hosts** | Load averages, memory, disk, RAID status, temperatures |

### Enriched Context

Beyond raw metrics, Patrol enriches the context with intelligence:

- **Trend analysis** — 24h and 7d patterns showing `growing`, `stable`, `declining`, or `volatile` behavior
- **Learned baselines** — Z-score anomaly detection based on what's *normal for your environment*
- **Capacity predictions** — "Storage pool will be full in 12 days at current growth rate"
- **Infrastructure changes** — Detected config changes, VM migrations, new deployments
- **Resource correlations** — Pattern detection across related resources
- **User notes** — Your annotations explaining expected behavior
- **Dismissed findings** — Respects your feedback and suppressed alerts
- **Incident memory** — Learns from past investigations and successful remediations

### Deterministic Signal Detection

Patrol doesn't rely solely on LLM judgment. It parses tool call outputs and fires deterministic signals for known problems:

| Signal Type | Trigger | Default Threshold |
|------------|---------|-------------------|
| `smart_failure` | SMART health status not OK/PASSED | N/A |
| `high_cpu` | Average CPU usage | 70% |
| `high_memory` | Average memory usage | 80% |
| `high_disk` | Storage pool usage | 75% (warning), 95% (critical) |
| `backup_failed` | Recent backup task with error status | Within 48h |
| `backup_stale` | No backup completed for VM/CT | 48+ hours |
| `active_alert` | Critical/warning alert in list | N/A |

Thresholds can be configured via alert settings to match user-defined values.

### Examples of What Patrol Catches

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
| **SMART failures** | Critical | Disk health check failed |

### What Patrol Ignores (by design)

Patrol is **intentionally conservative** to avoid noise:

- Small baseline deviations ("CPU at 15% vs typical 10%")
- Low utilization that's "elevated" but fine (disk at 40%)
- Stopped VMs/containers that were intentionally stopped
- Brief spikes that resolve on their own
- Anything that doesn't require human action

> **Philosophy**: If a finding wouldn't be worth waking someone up at 3am, Patrol won't create it.

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

---

## Autonomy Levels

Patrol supports three autonomy modes that control how much action it can take:

| Mode | Behavior | Plan |
|------|----------|------|
| **Monitor** | Detect issues only. No investigation or fixes. | Community (BYOK) |
| **Investigate** | Investigates findings and proposes fixes. All fixes require approval before execution. | Community (BYOK) |
| **Auto-fix** | Automatically fixes issues and verifies results. Critical findings still require approval by default. | Pro / Cloud |

### Investigation Flow

When a finding is created in Investigate or Auto-fix mode:

```
Finding created
      │
      ▼
MaybeInvestigateFinding()
      │
      ├─ Has orch + chatService?
      │        │
      │        ▼
      │   InvestigateFinding()
      │        │
      │        ▼
      │   Create chat session
      │        │
      │        ▼
      │   AI analysis (with tools)
      │        │
      │        ▼
      │   [Fix proposed?] ──Yes──► Queue approval (or auto-execute in full mode)
      │        │
      │        No
      │        ▼
      │   Update finding with outcome
      │
      └─ Skip investigation
```

### Investigation Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `MaxTurns` | 15 | Maximum agentic turns per investigation |
| `Timeout` | 10 min | Maximum duration per investigation |
| `MaxConcurrent` | 3 | Maximum concurrent investigations |
| `MaxAttemptsPerFinding` | 3 | Maximum investigation attempts per finding |
| `CooldownDuration` | 1 hour | Cooldown before re-investigating |
| `TimeoutCooldownDuration` | 10 min | Shorter cooldown for timeout failures |
| `VerificationDelay` | 30 sec | Wait before verifying fix |

### Investigation Outcomes

| Outcome | Meaning |
|---------|---------|
| `resolved` | Issue resolved during investigation |
| `fix_queued` | Fix proposed, awaiting approval |
| `fix_executed` | Fix auto-executed successfully |
| `fix_failed` | Fix attempted but failed |
| `fix_verified` | Fix worked, issue confirmed resolved |
| `fix_verification_failed` | Fix ran but issue persists |
| `needs_attention` | Requires human intervention |
| `cannot_fix` | Issue cannot be automatically fixed |
| `timed_out` | Investigation timed out (will retry sooner) |

---

## Pulse Assistant (Chat)

Pulse Assistant is a **tool-driven** chat interface. It does not "guess" system state — it calls live tools and reports their outputs.

### The Model's Workflow (Discover → Investigate → Act)

1. **Discover**: Uses `pulse_query` or `pulse_discovery` to find real resources and IDs
2. **Investigate**: Uses `pulse_read` to run bounded, read-only commands and check status/logs
3. **Act** (optional): Uses `pulse_control` for changes, then verifies with a read

### Available Tools

| Tool | Classification | Purpose |
|------|---------------|---------|
| `pulse_query`, `pulse_discovery` | Resolve | Resource discovery and query |
| `pulse_read` | Read | Read-only operations: exec, file, find, tail, logs |
| `pulse_metrics` | Read | Performance metrics and baselines |
| `pulse_storage` | Read | Storage pools, backups, snapshots, Ceph, RAID, disk health |
| `pulse_kubernetes` | Read | Kubernetes cluster info |
| `pulse_pmg` | Read | Proxmox Mail Gateway stats |
| `pulse_alerts` | Read/Write | Alert management (resolve/dismiss are writes) |
| `pulse_docker` | Read/Write | Docker operations (control/update are writes) |
| `pulse_knowledge` | Read/Write | Knowledge persistence (remember/note/save are writes) |
| `pulse_file_edit` | Read/Write | File operations (write/append are writes) |
| `pulse_control` | Write | Guest control, service management |
| `patrol_report_finding` | Patrol | Report a new finding (patrol runs only) |
| `patrol_resolve_finding` | Patrol | Resolve an active finding (patrol runs only) |
| `patrol_get_findings` | Patrol | List active findings (patrol runs only) |

### Safety Gates

The assistant enforces multiple safety gates:

1. **Discovery Before Action** — Action tools cannot operate on resources that weren't first discovered
2. **Verification After Write** — After any write, the model must perform a read/status check before providing a final answer
3. **Read/Write Separation** — Read operations route through `pulse_read` (stays in READING state); write operations route through `pulse_control` (enters VERIFYING state)
4. **Phantom Detection** — Detects when the model claims execution without tool calls
5. **Approval Mode** — In Controlled mode, every write requires explicit user approval
6. **Execution Context Binding** — Commands execute within the resolved resource's context, not on parent hosts

### Control Levels

| Level | Behavior | Plan |
|-------|----------|---------|
| **Read-only** | AI can observe and query data only | Community |
| **Controlled** | AI asks for approval before executing commands | Community |
| **Autonomous** | AI executes actions without prompting | Pro / Cloud |

### Using Approvals (Controlled Mode)

When control level is **Controlled**, write actions pause for approval:

1. Tool returns `APPROVAL_REQUIRED: { approval_id, command, ... }`
2. Agentic loop emits `approval_needed` SSE event
3. UI shows approval card with the proposed command
4. **Approve** to execute and verify, or **Deny** to cancel
5. Only users with admin privileges can approve/deny

---

## Configuration

Configure in the UI: **Settings → System → AI Assistant**

### Supported Providers

- **Anthropic** (API key or OAuth)
- **OpenAI**
- **OpenRouter**
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

### Storage

AI settings are stored encrypted at rest in `ai.enc` under the Pulse config directory. Related files:

| File | Purpose |
|------|---------|
| `ai.enc` | Encrypted AI configuration and credentials |
| `ai_findings.json` | Patrol findings |
| `ai_patrol_runs.json` | Patrol run history |
| `ai_usage_history.json` | Token usage data |
| `ai_chat_sessions.json` | Legacy chat sessions (UI sync) |
| `baselines.json` | Learned resource baselines |
| `ai_correlations.json` | Resource correlation data |
| `ai_patterns.json` | Detected patterns |

Config directory: `/etc/pulse` (systemd) or `/data` (Docker/Kubernetes)

### Testing

- Test provider connectivity: `POST /api/ai/test` and `POST /api/ai/test/{provider}`
- List available models: `GET /api/ai/models`

---

## Schedule and Triggers

Patrol runs on a configurable schedule:

| Interval | Description |
|----------|-------------|
| Disabled | Patrol runs only when manually triggered |
| 10 min – 7 days | Configurable interval (default: 6 hours) |

Patrol can also be triggered by:
- **Manual run**: Click "Run Patrol" in the UI
- **Alert-triggered analysis (Pro)**: Runs when an alert fires
- **API call**: `POST /api/ai/patrol/run`

---

## AI Intelligence Layer

Pulse includes a unified intelligence system that aggregates data from all AI subsystems:

### Components

| Component | Purpose |
|-----------|---------|
| **Baseline Engine** | Learns normal behavior, detects anomalies via z-score |
| **Pattern Detector** | Identifies recurring issues and trends |
| **Correlation Engine** | Links related issues across resources |
| **Incident Memory** | Tracks past incidents and successful remediations |
| **Knowledge Store** | Persists user annotations and learned preferences |
| **Forecast Engine** | Predicts capacity issues and resource exhaustion |

### Health Scoring

Each resource receives a health score (A–F) based on:
- Current metrics vs baseline
- Active findings and alerts
- Recent incidents
- Trend direction (improving/stable/declining)

---

## Model Matrix (Pulse Assistant)

This table summarizes the most recent **Pulse Assistant** eval runs per model.

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

---

## Safety Controls

Pulse includes settings that control how "active" AI features are:

- **Autonomous mode (Pro)**: When enabled, AI may execute safe commands without approval
- **Patrol auto-fix (Pro)**: Allows patrol to attempt automatic remediation
- **Alert-triggered analysis (Pro)**: Limits AI to analyzing specific events when alerts occur
- **Full autonomy unlock (Pro)**: Enables auto-fix for critical findings without approval (requires explicit toggle)

If you enable execution features, ensure agent tokens and scopes are appropriately restricted.

### Advanced Network Restrictions

Pulse blocks AI tool HTTP fetches to loopback and link-local addresses by default. For local development:

- `PULSE_AI_ALLOW_LOOPBACK=true`

Use this only in trusted environments.

---

## Privacy

Patrol runs on your server and only sends the minimal context needed for analysis to the configured provider (when AI is enabled). No telemetry is sent to Pulse by default.

---

## Why Patrol Is Different From Traditional Alerts

Alerts are threshold-based and narrow. Patrol is context-based and cross-system.

- **Alerts**: "Disk > 90%"
- **Patrol**: "ZFS pool is 86% but trending +4%/day; projected to hit 95% within a week. Largest consumer is datastore X. Recommend prune or expand."

---

## Cost Tracking

Pulse tracks token usage and costs:

- View usage summary: `GET /api/ai/cost/summary`
- Reset counters: `POST /api/ai/cost/reset` (admin)
- Set monthly budget limits in AI settings

---

## Troubleshooting

| Issue | Solution |
|-------|----------|
| AI not responding | Verify provider credentials in **Settings → System → AI Assistant** |
| No execution capability | Confirm at least one agent is connected |
| Findings not persisting | Check Pulse has write access to `ai_findings.json` in the config directory |
| Too many findings | This shouldn't happen — please report if it does |
| Investigation stuck | Check circuit breaker status at `/api/ai/circuit/status`; may auto-reset after cooldown |
| Model not available | Ensure provider API key is valid and model ID matches provider format |

## Related Documentation

### Deep Dives (Recommended for Technical Audiences)

- **[Pulse Assistant Deep Dive](architecture/pulse-assistant-deep-dive.md)** — Complete technical breakdown of the agentic architecture: context prefetching, knowledge accumulation, FSM enforcement, parallel execution, phantom detection, auto-recovery
- **[Pulse Patrol Deep Dive](architecture/pulse-patrol-deep-dive.md)** — Full intelligence layer documentation: baseline learning, pattern detection, forecasting, correlation analysis, incident memory, investigation orchestration

### Reference Documentation

- [Architecture: Pulse Assistant (Safety Gates)](architecture/pulse-assistant.md) — Detailed FSM states, tool protocol, and invariants
- [API Reference](API.md) — Complete API endpoint documentation
- [Plans and entitlements](PULSE_PRO.md) — Community/Pro/Cloud features and licensing
