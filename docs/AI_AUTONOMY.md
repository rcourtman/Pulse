# AI Autonomy and Safety Configuration

This guide covers how to configure and manage Pulse's AI autonomy levels, control levels, and safety guardrails.

For a general overview of Pulse AI, see [AI.md](AI.md). For plan-level feature availability, see [PULSE_PRO.md](PULSE_PRO.md).

---

## Two Axes of Control

Pulse separates AI permissions into two independent axes:

1. **Patrol Autonomy Level** — How much Patrol can do on its own (detect, investigate, fix).
2. **Assistant Control Level** — Whether the interactive chat assistant can execute commands.

These are configured independently in **Settings → System → AI Assistant**.

---

## Patrol Autonomy Levels

Patrol autonomy controls how aggressively Patrol responds to findings.

| Level | Key | Detect | Investigate | Fix (Warning) | Fix (Critical) | Plan |
|-------|-----|:------:|:-----------:|:-------------:|:--------------:|------|
| **Monitor** | `monitor` | Yes | No | No | No | Community |
| **Approval** | `approval` | Yes | Yes | Approval required | Approval required | Pro / Cloud |
| **Assisted** | `assisted` | Yes | Yes | Auto-fix | Approval required | Pro / Cloud |
| **Full** | `full` | Yes | Yes | Auto-fix | Auto-fix | Pro / Cloud |

- **Monitor** (default): Patrol creates findings but takes no action. This is the only level available on the Community plan. Suitable for learning what Patrol detects before enabling automation.
- **Approval** (Pro): Patrol investigates every finding and proposes fixes. All fixes queue for manual approval before execution.
- **Assisted** (Pro): Warning-level findings are auto-fixed. Critical findings still require approval. This is the recommended starting point for most Pro users.
- **Full** (Pro): All findings are auto-fixed without approval. Requires an explicit toggle and a Pro or Cloud license. Recommended only for environments with thorough alert coverage.

### Configuration

**UI:** Settings → System → AI Assistant → Patrol Autonomy Level

**API:**
```bash
# Get current patrol autonomy settings
curl -s -u admin:admin http://localhost:7655/api/ai/patrol/autonomy

# Update autonomy level
curl -X PUT http://localhost:7655/api/ai/patrol/autonomy \
  -u admin:admin \
  -H "Content-Type: application/json" \
  -d '{"autonomy_level": "approval", "investigation_budget": 15, "investigation_timeout_sec": 600}'
```

### License Requirements

- `monitor`: Available on all plans (Community with BYOK).
- `approval`, `assisted`, and `full`: Require the `ai_autofix` capability (Pro or Cloud license).

Without the `ai_autofix` capability, the effective autonomy level is clamped to `monitor` at runtime, regardless of the saved configuration. If you previously had a Pro license and downgraded, your saved setting is preserved but enforcement reverts to `monitor`.

---

## Assistant Control Levels

Control levels govern what the interactive Pulse Assistant can do during chat sessions.

| Level | Key | Query | Execute Commands | Plan |
|-------|-----|:-----:|:----------------:|------|
| **Read-only** | `read_only` | Yes | No | Community |
| **Controlled** | `controlled` | Yes | With approval | Community |
| **Autonomous** | `autonomous` | Yes | Yes | Pro / Cloud |

- **Read-only** (default): The assistant can query metrics, storage, and resource status but cannot execute any control actions.
- **Controlled**: The assistant can propose commands but pauses for your explicit approval before execution. Each command shows a detailed approval card in the chat UI.
- **Autonomous**: The assistant executes commands without prompting. Requires a Pro or Cloud license.

### Configuration

**UI:** Settings → System → AI Assistant → Control Level

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

When autonomy is `approval`, `assisted`, or `full`, Patrol investigates findings. These parameters tune investigation behavior:

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

Regardless of autonomy level, Pulse enforces multiple safety layers:

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

For new deployments, we recommend gradually increasing autonomy:

1. **Start with Monitor** — Run Patrol for a few cycles to see what it detects. Dismiss false positives.
2. **Move to Approval** — Enable investigation. Review proposed fixes to build confidence.
3. **Upgrade to Assisted** — Let Patrol auto-fix warnings while you approve critical fixes.
4. **Consider Full** — Only if your environment has comprehensive alerting and you trust the fix patterns.

---

## Monitoring AI Activity

### Patrol Metrics

Prometheus counters (prefix `pulse_patrol_*`) track:
- Patrol runs, findings, investigations, fixes
- Fix outcomes (success, failure, verification status)
- Circuit breaker trips

### Cost Tracking

Token usage and estimated costs are tracked per provider:
- **UI:** Settings → System → AI Assistant → Usage
- **API:** `GET /api/ai/cost/summary`
- Set monthly budget limits to cap spending

### Investigation Status

- **API:** `GET /api/ai/patrol/findings` — List all findings with investigation status
- **API:** `GET /api/ai/circuit/status` — Check circuit breaker state

---

## Related Documentation

- [Pulse AI Overview](AI.md) — Full AI system documentation
- [Plans and Entitlements](PULSE_PRO.md) — Feature availability by plan
- [API Reference](API.md) — Complete API documentation
- [Pulse Patrol Deep Dive](architecture/pulse-patrol-deep-dive.md) — Technical architecture details
