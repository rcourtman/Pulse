# Pulse Intelligence Modes and Safety Configuration

This guide covers how to configure Patrol mode, the Pulse Assistant chat action mode, and the safety guardrails that apply before Pulse can change infrastructure.

For a general overview of Pulse Intelligence, see [AI.md](AI.md). For plan-level feature availability, see [PULSE_PRO.md](PULSE_PRO.md).

---

## Two Axes of Control

Pulse separates AI permissions into two independent axes:

1. **Patrol Mode** — What Patrol may handle automatically after it finds an issue: watch only, ask before changes, handle safe fixes, or use policy autopilot.
2. **Assistant Chat Action Mode** — Whether the interactive chat assistant can plan actions for you to review in **Actions**.

Patrol mode is configured on the **Patrol** page. The Assistant chat action mode is configured in **Settings → Pulse Intelligence → Assistant**.

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

**API:** Use an administrator API token with `settings:write` in the
[private header file](API.md#api-token-recommended). This endpoint requires
that scope even for GET; a `monitoring:read` token is not sufficient. For a
one-off read, you can instead open the path in your signed-in administrator
browser without extracting its cookie. There is no shared example password
to configure or substitute into these commands.

Define the [API guide's `pulse_api` helper](API.md#api-token-recommended)
in the same Bash session before using these examples (curl 7.76 or later).
It is local example code, not an installed Pulse command. Each call sends one
request, prints only the HTTP status and saves the response to a new owner-only
file, preserving earlier responses. It uses a five-second connection limit and
a twenty-second whole-request limit, ignores local curl defaults and does not
follow redirects or retry.

These are separate operations, not a script to run from top to bottom. Read
the current settings first, then inspect the reported private response file:

```bash
pulse_api GET /api/ai/patrol/autonomy
```

The next command **changes Patrol mode**, not just connection health. Use it
only when you intend to enable investigation and queue fixes for approval on
a plan with that capability. The API retains `autonomy_level` for compatibility:

```bash
pulse_api PUT /api/ai/patrol/autonomy <<'JSON'
{"autonomy_level":"approval","investigation_budget":15,"investigation_timeout_sec":600}
JSON
```

The helper's loopback origin is for requests made on the Pulse host. For remote
use, change the origin and protocol restriction **in the helper** as the API
guide describes, using your Pulse **HTTPS** URL with certificate verification
enabled. Do not add `--insecure`, verbose/trace output or redirect following.

Inspect settings and error bodies privately; share only a relevant redacted
error, never the response file, token or cookie. A nonzero exit, including HTTP
401, 402 or 403, is not evidence that a change was saved. A partial response
is not a complete settings result, and HTTP success is not proof of the
effective mode or completed investigation. After an uncertain write, read the
saved settings before retrying: a timeout or lost response can occur **after
a change was applied**. If that read is unavailable, stop rather than repeat
the write. Check the saved and effective Patrol mode on the Patrol page;
licence and policy enforcement still apply.

### License Requirements

- `monitor`: Available on all plans. Community and Relay can run Patrol with BYOK.
- `approval`, `assisted`, and `full`: Require the `ai_autofix` capability (Pro, legacy Pro+, or Cloud license).

Without the `ai_autofix` capability, the effective Patrol mode is clamped to `monitor` at runtime, regardless of the saved configuration. If you previously had a Pro license and downgraded, your saved setting is preserved but enforcement reverts to `monitor`.

---

## Assistant Control Levels

Control levels govern what the interactive Pulse Assistant can do during chat sessions. The assistant never changes infrastructure from chat: an action it plans is saved to **Actions**, where you review and run it.

| Level | Key | Answers questions | Plans actions | Plan |
|-------|-----|:-----------------:|:-------------:|------|
| **Read-only** | `read_only` | Yes | No | Community |
| **Ask first** | `controlled` | Yes | Yes, for review in Actions | Community |

- **Read-only** (default): The assistant can query metrics, storage, and resource status but cannot plan actions.
- **Ask first**: The assistant can plan an action a resource advertises, such as restarting a container, and saves the plan to **Actions** for you to review and run.

Earlier versions offered a third `autonomous` level on Pro. Since July 2026, chat has run every request approval-required, so that level no longer changed what chat did. Pulse now saves a submitted `autonomous` value, and reads a stored one, as `controlled`.

### Configuration

**UI:** Settings → Pulse Intelligence → Assistant → Chat action mode

**API:** This intentionally changes the chat action mode. Use the same private
administrator header file with `settings:write`; the token must also have
permission to change settings. Prefer the UI for one-off changes, and verify
the saved **Chat action mode** there afterwards. Do not use a write as an
authentication test.

Use the same `pulse_api` helper defined above, with its private response file
and request limits. Supporting PUT in the helper grants no additional access.
If the response is lost, check the saved **Chat command mode** in the UI before
deciding whether another change is needed; do not repeat the write blindly.

```bash
pulse_api PUT /api/settings/ai/update <<'JSON'
{"control_level":"controlled"}
JSON
```

### Reviewing Planned Actions (Ask First)

When the level is `controlled`, an action the assistant proposes follows this flow:

1. The assistant calls `pulse_control` with a capability the resource advertises, such as `restart` on a container.
2. Pulse saves the plan in **Actions**; the chat does not run it.
3. You review the plan in **Actions**, where approving and running it are separate recorded steps.
4. The assistant can read the recorded outcome to confirm what happened.

---

## Investigation Configuration

When Patrol mode is `approval`, `assisted`, or `full`, Patrol investigates findings. These parameters tune investigation behavior:

| Setting | Default | Range | Description |
|---------|---------|-------|-------------|
| `patrol_investigation_budget` | 10 | 5–30 | Maximum evidence-tool calls per investigation; Patrol derives a separate model-response safety ceiling |
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
- [Pulse Patrol Deep Dive](PATROL_ARCHITECTURE.md) — Technical architecture details
