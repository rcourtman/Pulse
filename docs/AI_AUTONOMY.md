# Pulse Intelligence Modes and Safety Configuration

This guide covers how to configure Patrol mode, Pulse Assistant command access, and the safety guardrails that apply before Pulse can change infrastructure.

For a general overview of Pulse Intelligence, see [AI.md](AI.md). For plan-level feature availability, see [PULSE_PRO.md](PULSE_PRO.md).

---

## Two Axes of Control

Pulse separates AI permissions into two independent axes:

1. **Patrol Mode** — What Patrol may handle automatically after it finds an issue: watch only, ask before changes, handle safe fixes, or use policy autopilot.
2. **Assistant Command Access** — Whether the interactive chat assistant can execute commands during a chat session.

Patrol mode is configured on the **Patrol** page. Assistant command access is configured in **Settings → Pulse Intelligence → Assistant**.

---

## Patrol Modes

Patrol mode sets how far Pulse can go when Patrol finds something that needs attention.

| Mode | Key | Detect | Investigate | Fix warning-level issues | Fix critical issues | Plan |
|-------|-----|:------:|:-----------:|:------------------------:|:-------------------:|------|
| **Watch only** | `monitor` | Yes | No | No | No | Community |
| **Ask before changes** | `approval` | Yes | Yes | Approval required | Approval required | Pro / legacy Pro+ / Cloud |
| **Auto-fix safe issues** | `assisted` | Yes | Yes | Execute automatically | Approval required | Pro / legacy Pro+ / Cloud |
| **Policy autopilot** | `full` | Yes | Yes | Execute automatically | Execute automatically | Pro / legacy Pro+ / Cloud |

- **Watch only** (default): Patrol creates findings but takes no action. This is the Community and Relay baseline. Suitable for learning what Patrol detects before enabling investigation or fix execution.
- **Ask before changes** (Pro and above): Patrol investigates findings and proposes fixes. All fixes queue for manual approval before execution.
- **Auto-fix safe issues** (Pro and above): Warning-level safe fix plans can execute automatically. Critical findings still require approval. This is the recommended starting point for most Pro and legacy Pro+ users who enable fix execution.
- **Policy autopilot** (Pro and above): Safe fix plans can execute without approval. Requires an explicit toggle and a Pro, legacy Pro+, or Cloud license. Recommended only for environments with thorough alert coverage.

### Configuration

**UI:** Patrol → Patrol mode

**API:**
```bash
# Get current Patrol mode settings
curl -s -u admin:admin http://localhost:7655/api/ai/patrol/autonomy

# Update Patrol mode.
# The API keeps the autonomy_level field name for compatibility.
curl -X PUT http://localhost:7655/api/ai/patrol/autonomy \
  -u admin:admin \
  -H "Content-Type: application/json" \
  -d '{"autonomy_level": "approval", "investigation_budget": 15, "investigation_timeout_sec": 600}'
```

### License Requirements

- `monitor`: Available on all plans. Community and Relay can run Patrol with BYOK.
- `approval`, `assisted`, and `full`: Require the `ai_autofix` capability (Pro, legacy Pro+, or Cloud license).

Without the `ai_autofix` capability, the effective Patrol mode is clamped to `monitor` at runtime, regardless of the saved configuration. If you previously had a Pro license and downgraded, your saved setting is preserved but enforcement reverts to `monitor`.

---

## Assistant Control Levels

Control levels govern what the interactive Pulse Assistant can do during chat sessions.

| Level | Key | Query | Execute Commands | Plan |
|-------|-----|:-----:|:----------------:|------|
| **Read-only** | `read_only` | Yes | No | Community |
| **Controlled** | `controlled` | Yes | With approval | Community |
| **Autonomous** | `autonomous` | Yes | Yes | Pro / legacy Pro+ / Cloud |

- **Read-only** (default): The assistant can query metrics, storage, and resource status but cannot execute any control actions.
- **Controlled**: The assistant can propose commands but pauses for your explicit approval before execution. Each command shows a detailed approval card in the chat UI.
- **Autonomous**: The assistant executes commands without prompting. Requires a Pro, legacy Pro+, or Cloud license.

### Configuration

**UI:** Settings → Pulse Intelligence → Assistant → Chat command mode

**API:**
```bash
curl -X PUT http://localhost:7655/api/settings/ai/update \
  -u admin:admin \
  -H "Content-Type: application/json" \
  -d '{"control_level": "controlled"}'
```

### Approval Flow (Controlled Mode)

When control level is `controlled`, write operations follow this flow:

1. The assistant proposes a command (e.g., `qm start 100`).
2. An `APPROVAL_REQUIRED` response is emitted with an `approval_id`.
3. The UI displays an approval card showing the exact command.
4. You click **Approve** or **Deny**.
5. On approval, the command executes and the assistant verifies the result.

Approvals expire after 5 minutes if not acted upon.

---

## Investigation Configuration

When Patrol mode is `approval`, `assisted`, or `full`, Patrol investigates findings. These parameters tune investigation behavior:

| Setting | Default | Range | Description |
|---------|---------|-------|-------------|
| `patrol_investigation_budget` | 15 | 5–30 | Maximum agentic turns per investigation |
| `patrol_investigation_timeout_sec` | 600 | 60–1800 | Maximum seconds per investigation |
| `max_concurrent_investigations` | 3 | — | Parallel investigation limit |
| `max_attempts_per_finding` | 3 | — | Retries before marking as `needs_attention` |
| `investigation_cooldown_sec` | 3600 | — | Cooldown before re-investigating a finding |
| `timeout_cooldown_sec` | 600 | — | Shorter cooldown after timeout failures |

---

## Safety Guardrails

Regardless of Patrol mode, Pulse enforces multiple safety layers:

### Blocked Commands

Certain destructive commands are always blocked (defined in `pkg/aicontracts/safety.go`):
- Disk format/partition operations
- Cluster-wide destructive operations
- Commands that could cause data loss

### Risk Classification

Proposed fixes are classified by risk level in the approval system. Risk classification is surfaced in approval requests so operators can make informed decisions.

### Circuit Breaker

If the AI provider experiences consecutive failures, the circuit breaker (`internal/ai/circuit/breaker.go`) trips and temporarily disables AI operations. It auto-resets after a cooldown period.

### Discovery-Before-Action

The assistant cannot operate on resources it hasn't first discovered. This prevents hallucinated resource IDs from reaching infrastructure commands.

### Verification-After-Write

After executing any control action, the assistant must verify the result with a read operation before reporting success. This is enforced by the FSM — the assistant cannot return to idle state without verification.

---

## Recommended Progression

For new deployments, gradually increase Patrol mode:

1. **Start with Watch only** — Run Patrol for a few cycles to see what it detects. Dismiss false positives.
2. **Move to Ask before changes where available** — Enable investigation. Review proposed fixes to build confidence.
3. **Use Auto-fix safe issues when fix execution is enabled** — Let Patrol execute warning-level fixes while you approve critical fixes.
4. **Consider Policy autopilot** — Only if your environment has comprehensive alerting and you trust the fix patterns.

---

## Monitoring AI Activity

### Patrol Metrics

Prometheus counters (prefix `pulse_patrol_*`) track:
- Patrol runs, findings, investigations, fixes
- Fix outcomes (success, failure, verification status)
- Circuit breaker trips

### Cost Tracking

Token usage and estimated costs are tracked per provider:
- **UI:** Settings → Pulse Intelligence → Provider & Models → Provider Usage & Spend
- **API:** `GET /api/ai/cost/summary`
- Set monthly budget limits to cap spending

### Investigation Status

- **API:** `GET /api/ai/patrol/findings` — List all findings with investigation status
- **API:** `GET /api/ai/circuit/status` — Check circuit breaker state

---

## Related Documentation

- [Pulse Intelligence Overview](AI.md) — Full Pulse Intelligence system documentation
- [Plans and Entitlements](PULSE_PRO.md) — Feature availability by plan
- [API Reference](API.md) — Complete API documentation
- [Pulse Patrol Deep Dive](architecture/pulse-patrol-deep-dive.md) — Technical architecture details
