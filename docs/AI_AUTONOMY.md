# Pulse Intelligence Modes and Safety Configuration

This guide covers how to configure Patrol mode and the safety guardrails that apply before Pulse can change infrastructure.

For a general overview of Pulse Intelligence, see [AI.md](AI.md). For plan-level feature availability, see [PULSE_PRO.md](PULSE_PRO.md).

---

## Patrol Mode Is the Control

**Patrol Mode** decides what Patrol may handle automatically after it finds an
issue: watch only, ask before changes, handle safe fixes, or use policy
autopilot. Patrol mode is configured on the **Patrol** page.

The in-app Pulse Assistant chat, and its separate chat action setting, have been
retired. Pulse Mobile's chat keeps the level saved before then until the app
retires on 31 March 2027 (see [Pulse Mobile chat control level](#pulse-mobile-chat-control-level)).

---

## Patrol Modes

Patrol mode sets how far Pulse can go when Patrol finds something that needs attention.

| Mode | Key | Detect | Investigate | Fix warning-level issues | Fix critical issues | Plan |
|-------|-----|:------:|:-----------:|:------------------------:|:-------------------:|------|
| **Watch only** | `monitor` | Yes | No | No | No | Community |
| **Ask first** | `approval` | Yes | Yes | Approval required | Approval required | Pro / legacy Pro+ / Cloud |
| **Safe auto-fix** | `assisted` | Yes | Yes | Execute automatically | Approval required | Pro / legacy Pro+ / Cloud |
| **Autopilot** | `full` | Yes | Yes | Execute automatically | Execute automatically | Pro / legacy Pro+ / Cloud |

- **Watch only**: Patrol creates findings but takes no action. This is the Community and Relay baseline and the default without Pro. Suitable for learning what Patrol detects before enabling investigation or fix execution.
- **Ask first** (Pro and above): Patrol investigates findings and proposes fixes. All fixes queue for manual approval before execution.
- **Safe auto-fix** (Pro and above): Warning-level safe fix plans can execute automatically. Critical findings still require approval. This is the recommended starting point for most Pro and legacy Pro+ users who enable fix execution.
- **Autopilot** (Pro and above): Safe fix plans can execute without approval. Requires an explicit toggle and a Pro, legacy Pro+, or Cloud license. Recommended only for environments with thorough alert coverage.

On Pro, an install that has never chosen a mode runs **Ask first**, so new warning and critical findings are investigated without an extra step. Until you choose, the Patrol page explains this and offers to keep Ask first or switch to Watch only; either choice is saved and the notice goes away. Investigations use your AI provider and count toward the 30-day budget. A mode you have saved, including Watch only, is never changed.

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

## Pulse Mobile chat control level

The control level governs Pulse Mobile chat queries and action planning, not
Patrol's autonomy or permission to execute infrastructure work. The web
settings page that edited it was retired with the in-app chat; the saved value
still applies to Pulse Mobile until the app retires on 31 March 2027.

| Mode | Key | Query | Plan infrastructure actions | Availability |
|------|-----|:-----:|:---------------------------:|--------------|
| **Read-only** | `read_only` | Yes | No | Community |
| **Ask first** | `controlled` | Yes | For operator review in **Actions** | Community |

Chat does not execute the plans it saves. A legacy saved `autonomous`
preference acts as **Ask first** and keeps its stored value and existing
entitlement checks for older clients. It does not bypass approval. Web settings
saves never rewrite that preference. Unknown values fail closed to read-only.
Patrol mode and Autopilot acknowledgements remain separate.

To keep chat and Patrol away from a specific guest, lock it: open the guest,
choose **Manage**, turn on **Never auto-remediate** under Operator overrides,
and save. Pulse then refuses typed actions that target that resource, including
approved ones (see [Keep a Resource Off-Limits](#keep-a-resource-off-limits)).

### Configuration

**API only.** This intentionally changes Pulse Mobile chat action planning
access. Use the same private administrator header file with `settings:write`;
the token must also have permission to change settings. Read
`control_level` from `GET /api/settings/ai` afterwards to confirm the saved
value. Do not use a write as an authentication test.

Use the same `pulse_api` helper defined above, with its private response file
and request limits. Supporting PUT in the helper grants no additional access.
If the response is lost, read the saved `control_level` before deciding
whether another change is needed; do not repeat the write blindly.

```bash
pulse_api PUT /api/settings/ai/update <<'JSON'
{"control_level":"controlled"}
JSON
```

### Reviewing plans (Ask first)

1. Chat requests a typed action for an explicitly identified resource with
   the advertised capability.
2. Pulse saves the canonical plan in **Actions** without requesting execution.
3. Review the target, operation, approval policy and expiry in **Actions**.
4. Use the existing authorised approval and run controls; the chat control
   level grants no additional access or automatic approval.
5. Read the recorded execution and independent verification outcome before
   concluding that the operation worked. Do not repeat an uncertain operation.

---

## Investigation Configuration

When Patrol mode is `approval`, `assisted`, or `full`, Patrol investigates findings. These parameters tune investigation behavior:

| Setting | Default | Range | Description |
|---------|---------|-------|-------------|
| `patrol_investigation_budget` | 10 | 5–30 | Maximum evidence-tool calls per investigation; Patrol derives a separate model-response safety ceiling |
| `patrol_investigation_timeout_sec` | 600 | 60–1800 | Maximum seconds per investigation |

The following limits are fixed and cannot be changed: up to 3 investigations run in parallel, up to 3 attempts per finding, a 1 hour cooldown before re-investigating a finding, and a shorter 10 minute cooldown after a timeout failure.

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

The AI runtime cannot operate on resources it hasn't first discovered. This prevents hallucinated resource IDs from reaching infrastructure commands.

### Verification-After-Write

After executing any control action, the AI runtime must verify the result with a read operation before reporting success. This is enforced by the FSM — the runtime cannot return to idle state without verification.

### Keep a Resource Off-Limits

To stop Pulse acting on a specific guest, host or container, open its drawer, choose **Manage**, turn on **Never auto-remediate** under Operator overrides, confirm the prompt, and click **Save overrides**. Pulse then refuses typed actions that target that exact resource: chat declines to plan one when it can read the lock, Patrol will not run one, and the action lifecycle checks the lock again before it runs an action that was already planned, even when approved. The lock covers that resource only, not its host or other resources, and it does not stop read-only investigation. Turn it off and save again when you want Pulse to act on the resource again. A resource whose lifecycle is Retired is blocked the same way until the lifecycle is set back to Active.

---

## Recommended Progression

For new deployments, gradually increase Patrol mode:

1. **Start with Watch only** — Run Patrol for a few cycles to see what it detects. Dismiss false positives.
2. **Move to Ask first where available** — Enable investigation. Review proposed fixes to build confidence.
3. **Use Safe auto-fix when fix execution is enabled** — Let Patrol execute warning-level fixes while you approve critical fixes.
4. **Consider Autopilot** — Only if your environment has comprehensive alerting and you trust the fix patterns.

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
