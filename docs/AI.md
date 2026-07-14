# Pulse Intelligence

Pulse Patrol is available to everyone on the Community plan with BYOK (your own AI provider). Pro adds hands-on Patrol modes, issue investigation, governed fixes, verified outcomes, and 90-day history, while hosted Cloud carries those capabilities for hosted environments. Learn more at <https://pulserelay.pro> or see [PULSE_PRO.md](PULSE_PRO.md).

---

## Overview

<!-- pulse-intelligence-overview:start -->
Pulse Intelligence is built around a shared **Pulse Intelligence Core**: Canonical context, governed actions, safety gates, approval state, action audit, and verification shared by Pulse Assistant, Pulse MCP, and Pulse Patrol.

That core is deliberately surfaced with Patrol as the primary built-in operator and Assistant plus MCP as access paths over the same governed capabilities:

1. **Pulse Patrol**: Patrol is the first-party operations surface: it checks infrastructure, investigates issues, follows the chosen Patrol mode before acting, verifies outcomes, and records what happened.
2. **Pulse Assistant**: The contextual explanation, approval, and handoff surface for Patrol findings, governed actions, verification, and operator questions. Affordances: tools and interactive questions.
3. **Pulse MCP**: The external-agent adapter that projects canonical Pulse Intelligence capabilities as MCP tools. Affordances: tools, resources, prompts, and capability metadata.
<!-- pulse-intelligence-overview:end -->

These surfaces are built on the same action-driven architecture: the configured LLM owns diagnosis, prioritization, fix reasoning, and action choice; Pulse supplies context, capabilities, safety gates, approval state, and audit trails. Verification is part of the governed action lifecycle rather than a separate model-owned feature.

### Not Just Another Chatbot

Pulse Assistant is a **protocol-driven, safety-gated LLM tool surface** that:

- **Provides governed context** — attaches explicit resource mentions and recent session facts without rewriting user intent
- **Caches session facts** — extracts bounded tool facts to avoid redundant queries during the current conversation
- **Enforces workflow invariants** — FSM prevents dangerous state transitions
- **Supports parallel tool execution** — efficient batch operations with concurrency control
- **Grounds answers in real tool work** — visible tool traces, read-after-write verification, and transcript hygiene prevent unsupported execution claims from being treated as facts
- **Returns structured tool errors** — the model can recover from clear, machine-readable failures

📖 **For a deep technical dive into the Assistant architecture, see [architecture/pulse-assistant-deep-dive.md](architecture/pulse-assistant-deep-dive.md).**

### Not Just Another Alerting System

Pulse Patrol is a **scheduled and event-triggered governed operator** that:

- **Assembles evidence** from metrics, storage, backups, discovery, alerts, and resource timelines
- **Provides statistical context** such as baselines, trend summaries, capacity estimates, and event relationships
- **Lets the configured LLM reason** over that evidence and decide whether to call tools or report findings
- **Routes governed actions** through approval, entitlement, policy, verification, and audit boundaries
- **Preserves operator feedback** as context for future model runs without converting it into Pulse-authored fixes

All while running entirely on your infrastructure with BYOK for complete privacy.

📖 **For a deep technical dive into the Patrol runtime, see [architecture/pulse-patrol-deep-dive.md](architecture/pulse-patrol-deep-dive.md).**

🧪 **For independent live-fault qualification, safety gates, model comparison,
and release-claim rules, see [AI_PATROL_QUALIFICATION.md](AI_PATROL_QUALIFICATION.md).**

See [architecture/pulse-assistant.md](architecture/pulse-assistant.md) for the original safety architecture documentation.

### Assistant And MCP

Pulse Assistant and `pulse-mcp` are sibling surfaces over Pulse Intelligence,
not competing implementations, and neither replaces the other. Assistant remains
the in-app Pro surface for current resource/finding/run handoffs, approval
cards, governed action status, and operator-friendly timelines. `pulse-mcp`
owns the external-agent bridge: it fetches `/api/agent/capabilities`, projects
those canonical API capabilities as MCP tools, and preserves the same stable
error envelopes and approval/audit contracts. New operational capabilities
should be added to the canonical API manifest first, then consumed by Assistant
or MCP as appropriate; MCP-only actions and Assistant-only copies of the same
business logic are drift.

---

## Pulse Patrol

Patrol is a scheduled model workflow that builds a rich, system-wide snapshot and gives your configured LLM the tools it needs to produce actionable findings.

### How Patrol Works

```
Scheduled/Event Trigger
        │
        ▼
buildSeedContext()  ── infrastructure evidence and policy context
        │
        ▼
LLM analysis (with tools) ← pulse_storage, pulse_metrics, pulse_alerts, etc.
        │
        ▼
patrol_report_finding() / patrol_assess_finding() / patrol_resolve_finding()
        │                                      └── explicit verdict for every known finding
        │
        ├── DetectSignals() ── deterministic evidence extraction from tool outputs
        │       │
        │       ▼
        │   Evaluation pass ── focused LLM review of unmatched evidence
        │
        ▼
model-reported findings ── validated, deduplicated, stored
        │
        ▼ (if configured)
MaybeInvestigateFinding() ── model investigation + governed fix planning/execution
```

### What Patrol Sees

Every patrol run passes the LLM comprehensive context about your environment:

| Data Category | What's Included |
|---------------|-----------------|
| **Proxmox Nodes** | Status, CPU%, memory%, uptime, 24h/7d trend analysis |
| **VMs & Containers** | Full metrics, backup status, OCI images, historical trends, anomaly evidence |
| **Storage Pools** | Usage %, capacity estimates, type (ZFS/LVM/Ceph), growth rates |
| **Docker/Podman** | Container counts, health states, unhealthy container lists |
| **Kubernetes** | Nodes, pods, deployments, services, DaemonSets, StatefulSets, namespaces |
| **TrueNAS** | Pools, datasets, disk health, SMART status, replication, alerts |
| **PBS/PMG** | Datastore status, backup jobs, job failures, verification status |
| **Ceph** | Cluster health, OSD states, PG status |
| **Agent Hosts** | Load averages, memory, disk, RAID status, temperatures |

### Model-Bound Context

Beyond raw metrics, Patrol prepares structured evidence for the model:

- **Trend summaries** — 24h and 7d samples showing `growing`, `stable`, `declining`, or `volatile` behavior
- **Baseline evidence** — Z-score anomaly evidence from historical metrics
- **Capacity estimates** — "Storage pool reaches 95% in about 12 days at current growth rate"
- **Infrastructure changes** — Detected config changes, VM migrations, new deployments
- **Resource relationships** — Related events and topology context
- **User notes** — Your annotations explaining expected behavior
- **Dismissed findings** — Respects your feedback and suppressed alerts
- **Investigation context** — Uses prior alert context, Patrol run history, and resource timelines

### Deterministic Evidence Extraction

Patrol parses tool outputs for concrete evidence such as backup failures, storage pressure, and disk health failures. These signals are not final findings by themselves: unmatched signals are sent to a focused LLM evaluation pass, and if the model still declines to report them, Pulse does not convert them into Pulse-authored findings.

| Signal Type | Trigger | Default Threshold |
|------------|---------|-------------------|
| `smart_failure` | SMART health status not OK/PASSED, or critical SMART counters such as pending sectors, offline uncorrectable sectors, or NVMe media errors | N/A |
| `high_cpu` | Average CPU usage | 70% |
| `high_memory` | Average memory usage | 80% |
| `high_disk` | Storage pool usage | 75% (warning), 95% (critical) |
| `backup_failed` | Recent backup task with error status | Within 48h |
| `backup_stale` | No backup completed for VM/CT | 48+ hours |

Thresholds can be configured via alert settings to match user-defined values.

### Examples of What Patrol Catches

| Issue | Severity | Example |
|-------|----------|---------|
| **Disk approaching capacity** | Warning/Critical | Storage growing toward full with concrete time-to-threshold evidence |
| **Backup failures** | Warning | PBS job failed, no backup in 48+ hours |
| **Storage issues** | Critical | PBS datastore errors, ZFS pool degraded |
| **Ceph problems** | Warning/Critical | Degraded OSDs, unhealthy PGs |
| **Kubernetes issues** | Warning | Pods stuck in Pending/CrashLoopBackOff |
| **SMART failures** | Critical | Disk health check failed, pending sectors, offline uncorrectable sectors, or NVMe media errors |
| **Alert-triggered investigations** | Pro / Cloud | A fired alert prompts the model to gather surrounding context and explain likely cause |

### What Patrol Ignores (by design)

Patrol is **intentionally conservative** to avoid noise:

- Small baseline deviations ("CPU at 15% vs typical 10%")
- Low utilization that's "elevated" but fine (disk at 40%)
- Stopped VMs/containers that were intentionally stopped
- Brief spikes that resolve on their own
- Anything that doesn't require human action
- Conditions already fully covered by the normal alert lifecycle unless the model finds additional context that changes the operator decision

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

Every active finding shown or returned to a Patrol run must receive an
explicit `present`, `resolved`, or `uncertain` assessment. Silence is not an
all-clear signal. `present` refreshes current evidence, `resolved` remains
subject to deterministic verification, and `uncertain` keeps the finding open
and makes the run visibly inconclusive.

### Patrol model qualification

The Assistant model matrix below proves Assistant orchestration only. Patrol
recommendations are published separately from live, reversible canary faults,
healthy controls, normal collection paths, scenario-owned ground truth, and
track-specific launch gates. See
[Pulse Patrol autonomous operations and real-world qualification](AI_PATROL_QUALIFICATION.md)
for the catalogue, methodology, safe lab boundary, full-track local suite,
privacy-allowlisted community evidence export, and publication command.

---

## Patrol Modes

Patrol supports four modes that decide how far Pulse can go after it finds an issue:

| Mode | Behavior | Plan |
|-------|----------|------|
| **Watch only** | Detect issues only. No investigation or fixes. | Community (BYOK) |
| **Ask before changes** | Investigates findings and proposes fixes. All fixes require approval before execution. | Pro / hosted Cloud |
| **Auto-fix safe issues** | Runs warning-level governed fixes automatically and verifies results. Critical findings still require approval by default. | Pro / hosted Cloud |
| **Policy autopilot** | Runs eligible governed fixes automatically and verifies results. Use only in environments where this is acceptable. | Pro / hosted Cloud |

Community and Relay installs can still run scheduled Patrol findings with BYOK. Watch only remains the free-first baseline; investigation, proposed fixes, and fix execution are paid AI-operations capabilities rather than a core monitoring limit.

### Investigation Flow

When a finding is created in a Pro Patrol mode:

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

### Tool Inventory

The Assistant tool list is registry-owned at runtime, not hand-maintained in
this public overview. Each turn receives an available-tool manifest generated
from Pulse's governed tool registry, including action mode (`read`, `mixed`,
`write`) and approval policy (`scope_only`, `action_plan`). That same registry
feeds the Assistant system prompt, provider tool declarations, tool-result
handling, approval boundaries, and Patrol-only tool filtering.

For the current source-owned inventory, see the native tool registry and
governance projection in `internal/ai/tools/` and
`internal/agentcapabilities/`. For external agents, use the live
`/api/agent/capabilities` manifest or `pulse-mcp` `tools/list`; those surfaces
project the canonical agent capabilities rather than a separate MCP-only tool
table.
The same manifest also carries reusable `workflowPrompts` metadata so Pulse
Assistant-compatible starters and MCP `prompts/list` clients discover the same
fleet triage, resource investigation, and Patrol finding review workflows.

### Safety Gates

The assistant enforces multiple safety gates:

1. **Discovery Before Action** — Action tools cannot operate on resources that weren't first discovered
2. **Verification After Write** — After any write, the model must perform a read/status check before providing a final answer
3. **Read/Write Separation** — Read operations route through `pulse_read` (stays in READING state); write operations route through `pulse_control` (enters VERIFYING state)
4. **Grounded Execution Guardrails** — Visible tool traces and read-after-write checks prevent unsupported execution claims from being treated as facts
5. **Approval Mode** — In Controlled mode, every write requires explicit user approval
6. **Execution Context Binding** — Commands execute within the resolved resource's context, not on parent hosts

### Control Levels

| Level | Behavior | Plan |
|-------|----------|---------|
| **Read-only** | AI can observe and query data only | Community |
| **Controlled** | AI asks for approval before executing commands | Community |
| **Autonomous** | AI executes actions without prompting | Pro / hosted Cloud |

### Using Approvals (Controlled Mode)

When control level is **Controlled**, write actions pause for approval:

1. Tool returns `APPROVAL_REQUIRED: { approval_id, command, ... }`
2. Agentic loop emits `approval_needed` SSE event
3. UI shows approval card with the proposed command
4. **Approve** to execute and verify, or **Deny** to cancel
5. Only users with admin privileges can approve/deny

---

## Configuration

Configure providers in the UI: **Settings → Pulse Intelligence → Provider & Models**

### Supported Providers

- **Anthropic** (API key)
- **OpenAI**
- **OpenRouter**
- **DeepSeek**
- **Google Gemini**
- **Ollama** (self-hosted, with tool/function calling support)
- **Codex subscription (local)** — uses an installed Codex CLI signed in with
  ChatGPT; no OpenAI API key is required or forwarded
- **Claude subscription (local)** — uses an installed Claude CLI signed in
  with a Claude plan; no Anthropic API key is required or forwarded
- **OpenAI-compatible base URL** (for providers that implement the OpenAI API shape)

Legacy Anthropic OAuth fields may still appear in stored settings so existing
installs can disconnect and clear old tokens, but Anthropic OAuth is not a
supported runtime authentication method and does not make Anthropic configured.

### Local subscription-agent routes

The local subscription routes are explicit, same-machine transports for
self-hosted Pulse. Enable one under **Provider & Models** only when the Pulse
process runs as a user that can execute the corresponding CLI and read that
CLI's existing login. Pulse does not copy, store, refresh, or expose the CLI's
OAuth credentials. It also constructs a strict child-process environment that
does not forward API-key environment variables such as `OPENAI_API_KEY` or
`ANTHROPIC_API_KEY`, Pulse secrets, cloud credentials, or unrelated tokens,
preventing an installed API key from silently changing the billing route.

The child CLI is not given infrastructure authority. Each invocation runs in a
new temporary directory, with user extensions disabled, no Pulse MCP server,
no approval capability, and a structured output schema. It returns one proposed
provider turn. Pulse validates tool names, IDs, argument JSON, and tool-choice
constraints. Pulse retains tool execution and policy enforcement: the normal
Pulse tool loop independently applies control level, license,
protected-resource, approval, action, and verification policy. The CLI never
executes a Patrol tool itself.

This is still a local agent process, not a remote chat-completions API. Pulse
rejects a turn if Codex reports command, file, MCP, web, computer, or image-tool
activity, and Claude is launched with its built-in filesystem, shell, web, and
task tools denied. Codex's read-only sandbox is the remaining operating-system
boundary; operators should run self-hosted Pulse under a dedicated,
least-privilege OS account that cannot read unrelated user secrets. Do not
enable a subscription-agent route on a broadly privileged service account
merely to avoid API charges.

Install and authenticate the CLI before enabling its route:

```bash
codex login
codex login status

claude auth login
claude auth status --json
```

Use model IDs such as `codex-subscription:gpt-5.6-luna`,
`claude-subscription:sonnet`, or `claude-subscription:opus`. Model availability
and plan limits remain controlled by the installed CLI and the user's plan.
These routes are unsuitable for a container or service account unless that
runtime deliberately has the CLI and its own valid login. Pulse reports missing
binaries, logged-out sessions, plan limits, and model access failures as
provider readiness failures; it never falls back to a metered API provider.

The opt-in live transport probe is:

```bash
PULSE_TEST_SUBSCRIPTION_AGENTS=1 \
  go test ./internal/ai/providers -run '^TestSubscriptionAgentLive$' -count=1 -v
```

Qualification reports record `inference_route=local_subscription_agent` so
subscription-backed runs cannot be confused with direct API or local-model
runs. Token counts depend on what the CLI exposes, and a subscription allowance
is not represented as a zero-dollar API price. The report keeps its monetary
cost unknown and marks the per-run metered-API budget as not applicable; plan
limits, provider errors, latency, and any usage the CLI exposes remain visible.

Z.ai requests sent through a configured `/api/coding/paas/` endpoint are
recorded as `inference_route=coding_plan_allowance`. Qualification keeps their
per-run monetary cost unknown and the metered-API dollar budget not applicable,
while still scoring tokens, latency, provider or plan failures, and model
quality. The standard Z.ai `/api/paas/` endpoint remains a `metered_api` route.

### Models

Pulse uses model identifiers in the form: `provider:model-name`

You can set separate models for:
- Chat (`chat_model`)
- Patrol (`patrol_model`)
- Patrol fix model (`auto_fix_model`, retained as the compatibility settings key)

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
- **Alert-triggered analysis (Pro and above)**: Runs when an alert fires
- **API call**: `POST /api/ai/patrol/run`

---

## Model Context Layer

Pulse includes a model-context layer that aggregates evidence from AI runtime subsystems:

### Components

| Component | Purpose |
|-----------|---------|
| **Baseline Store** | Maintains statistical metric summaries and anomaly evidence |
| **Pattern Store** | Records recurring event evidence and trend context |
| **Correlation Store** | Links related events and resource relationships for model context |
| **Investigation Context** | Uses alert history, Patrol runs, and resource timelines |
| **Knowledge Store** | Persists user annotations and model-safe context |
| **Forecast Service** | Estimates capacity trajectories from historical samples |

### Health Scoring

The Patrol UI can show an operational score (A-F) based on active findings, Patrol coverage, runtime errors, and structured evidence. This score is a presentation aid, not a replacement for model diagnosis.

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

Run the resource-context Assistant handoff eval against a live resource:
```
EVAL_RESOURCE_CONTEXT_ID=delly:delly:101 \
EVAL_RESOURCE_CONTEXT_NAME=homeassistant \
EVAL_RESOURCE_CONTEXT_TYPE=system-container \
EVAL_RESOURCE_CONTEXT_NODE=delly \
EVAL_RESOURCE_CONTEXT_FORBIDDEN="/mnt/pve/finance-db,/var/lib/homeassistant,literal-provider-token-123" \
go run ./cmd/eval -scenario resource-context -url http://127.0.0.1:7655 -user admin -pass "$PULSE_EVAL_PASS"
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

- **Patrol modes (Pro and above)**: Lets you choose whether Patrol only watches, asks before changes, handles safe fixes, or uses policy autopilot
- **Governed fixes (Pro and above)**: Allows Patrol to propose, approve, run, verify, and record fixes under the Patrol mode you choose
- **Issue investigation (Pro and above)**: Lets Patrol investigate findings with surrounding infrastructure context
- **Policy autopilot unlock (Pro and above)**: Permits eligible critical fixes without per-fix approval after an explicit opt-in

If you enable execution features, ensure agent tokens and scopes are appropriately restricted.

### Advanced Network Restrictions

Pulse blocks AI tool HTTP fetches to loopback and link-local addresses by default. For local development:

- `PULSE_AI_ALLOW_LOOPBACK=true`

Use this only in trusted environments.

---

## Privacy

Patrol runs on your server and only sends the minimal context needed for analysis to the configured provider (when AI is enabled). Outbound usage telemetry (a rotating pseudonymous install ID, counts, feature flags, and coarse Patrol mode and governed Pulse Intelligence operations adoption flags and counters only; no hostnames, credentials, prompts, chat messages, command text, action output, token values, IP addresses, or resource identifiers in the payload) is enabled by default and can be disabled any time. See [Privacy](PRIVACY.md) for details.

---

## Why Patrol Is Different From Traditional Alerts

Alerts are threshold-based and narrow. Patrol gives the selected model a broader, tool-backed operating picture.

- **Alerts**: "Disk > 90%"
- **Patrol**: "The model sees ZFS pool usage, growth rate, datastore consumers, backup context, and governed actions, then decides whether that evidence warrants a finding or action recommendation."

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
| Assistant or Patrol not responding | Verify provider credentials in **Settings → Pulse Intelligence → Provider & Models** |
| No execution capability | Confirm at least one agent is connected |
| Findings not persisting | Check Pulse has write access to `ai_findings.json` in the config directory |
| Too many findings | This shouldn't happen — please report if it does |
| Investigation stuck | Check circuit breaker status at `/api/ai/circuit/status`; may auto-reset after cooldown |
| Model not available | Ensure provider API key is valid and model ID matches provider format |

## Related Documentation

### Deep Dives (Recommended for Technical Audiences)

- **[Pulse Assistant Deep Dive](architecture/pulse-assistant-deep-dive.md)** — Complete technical breakdown of the model-owned tool surface: explicit context, session fact caching, FSM enforcement, parallel execution, grounded execution guardrails, structured errors
- **[Pulse Patrol Deep Dive](architecture/pulse-patrol-deep-dive.md)** — Patrol runtime documentation: evidence assembly, deterministic signal extraction, model evaluation, investigation context, investigation orchestration

### Reference Documentation

- [Architecture: Pulse Assistant (Safety Gates)](architecture/pulse-assistant.md) — Detailed FSM states, tool protocol, and invariants
- [API Reference](API.md) — Complete API endpoint documentation
- [Plans and entitlements](PULSE_PRO.md) — Community/Relay/Pro/Cloud features and licensing
