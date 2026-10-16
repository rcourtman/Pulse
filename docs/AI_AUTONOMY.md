# Pulse Intelligence Modes and Safety Configuration

This guide covers Patrol mode, Assistant action planning and
[what the guardrails establish](#safety-guardrails). Permission to plan or run
an action is not a guarantee of a correct diagnosis or a recovered workload.

For a general overview of Pulse Intelligence, see [AI.md](AI.md). For plan-level feature availability, see [PULSE_PRO.md](PULSE_PRO.md).

---

## Two Axes of Control

Pulse separates AI permissions into two independent axes:

1. **Patrol Mode** — What Patrol may handle automatically after it finds an issue: watch only, ask before changes, handle safe fixes, or use policy autopilot.
2. **Assistant Action Planning** — Whether the interactive chat assistant may
   plan infrastructure actions for review in **Actions**. Chat does not execute
   those plans.

Patrol mode is configured on the **Patrol** page. Assistant action planning is
configured in **Settings → Pulse Intelligence → Assistant → Chat action mode**.

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

## Assistant Control Levels

Control levels govern interactive Assistant queries and action planning, not
Patrol's autonomy or permission to execute infrastructure work.

| Mode | Key | Query | Plan infrastructure actions | Availability |
|------|-----|:-----:|:---------------------------:|--------------|
| **Read-only** | `read_only` | Yes | No | Community |
| **Ask first** | `controlled` | Yes | For operator review in **Actions** | Community |

Assistant chat does not execute the plans it saves. A legacy saved `autonomous`
preference displays as **Ask first** and keeps its stored value and existing
entitlement checks for older clients. It does not bypass approval. An unrelated
settings save does not rewrite that preference. Unknown values fail closed to
read-only. Patrol mode and Autopilot acknowledgements remain separate.

To keep Assistant away from a specific guest, lock it: open the guest, choose
**Manage**, turn on **Never auto-remediate** under Operator overrides, and save.
Pulse then refuses typed actions that target that resource, including
approved ones (see [Keep a Resource Off-Limits](#keep-a-resource-off-limits)).
Older versions also showed a **Protected guests** list here. It never excluded a
guest from action plans, so it has been removed.

### Configuration

**UI:** Settings → Pulse Intelligence → Assistant → Chat action mode

**API:** This intentionally changes Assistant action planning access. Use the same private
administrator header file with `settings:write`; the token must also have
permission to change settings. Prefer the UI for one-off changes, and verify
the saved **Chat action mode** there afterwards. Do not use a write as an
authentication test.

Use the same `pulse_api` helper defined above, with its private response file
and request limits. Supporting PUT in the helper grants no additional access.
If the response is lost, check the saved **Chat action mode** in the UI before
deciding whether another change is needed; do not repeat the write blindly.

```bash
pulse_api PUT /api/settings/ai/update <<'JSON'
{"control_level":"controlled"}
JSON
```

### Reviewing plans (Ask first)

1. Assistant requests a typed action for an explicitly identified resource with
   the advertised capability.
2. Pulse saves the canonical plan in **Actions** without requesting execution.
3. Review the target, operation, approval policy and expiry in **Actions**.
4. Use the existing authorised approval and run controls; Chat action mode grants
   no additional access or automatic approval.
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

These controls constrain authority; they do not establish that a proposed fix
is appropriate or that the original problem is resolved. Review the existing
action and its evidence before relying on a success message. Do not run a
command, change mode or provoke a failure just to test a safety claim.

### Blocked Commands

Command-policy paths use a fixed list of blocked patterns, including disk
formatting and destructive cluster commands. This is not a proof that every
other operation is safe. Typed lifecycle capabilities have their own target,
permission and policy checks: an authorised action can intentionally stop or
restart a workload. A blocked shell-command pattern is not a promise that the
corresponding typed capability can never run.

### Risk Classification

Review the target, capability, parameters, risk and applicable approval policy
in **Actions**. A low-risk classification or approval is not an execution
receipt, a recovery guarantee or permission to bypass a resource lock. A
Patrol mode that allows automatic fixes does not turn Assistant chat into an
execution path.

### Circuit Breaker

Consecutive AI-provider failures can trip the provider circuit breaker and
temporarily prevent further model requests until its cooldown permits another
attempt. This does not reverse an action already dispatched or establish the
workload's current state.

### Discovery-Before-Action

Pulse resolves action targets to canonical resources and advertised
capabilities; the action lifecycle checks current authority before execution.
A discovered name or an earlier read is not permission to act, proof of the
right diagnosis or a guarantee that the recorded state is still current.

### Verification-After-Write

**Execution and verification are separate results.** A completed chat, saved
plan, approval or successful executor result does not establish independent
verification. Reading something after a write does not by itself verify that
write either; Pulse does not enforce a blanket chat-state-machine sequence
that guarantees recovery.

Review the existing action in **Actions**, keeping these observations separate:

| Recorded observation | What it establishes |
| --- | --- |
| Plan or approval | An intended operation or authority decision, not proof that it ran. |
| Execution succeeded | The executor reported success; check verification separately. |
| **Confirmed by independent observer** | The recorded postcondition at its observation time, not current health or resolution of every original symptom. |
| **Confirmed by executing agent** | The executing agent's observation, not an independent observer's confirmation. |
| **Outcome not verified** or **Outcome inconclusive** | Verification is absent or cannot establish the postcondition; unknown is not success. |
| **Outcome contradicted** | The recorded observation does not confirm the intended postcondition, even if execution succeeded. |

Use the existing outcome's target, evidence source and time, alongside safe
observations of the original problem. Do not repeat an uncertain operation or
send a new infrastructure probe merely to obtain a green result. Missing access
or an unavailable action record leaves the outcome unknown; a quiet timeline
does not prove that nothing ran. See the
[Assistant safety architecture](ASSISTANT_SAFETY.md) for the recorded boundaries
and their limits.

### Keep a Resource Off-Limits

To stop Pulse acting on a specific guest, host or container, open its drawer, choose **Manage**, turn on **Never auto-remediate** under Operator overrides, confirm the prompt, and click **Save overrides**. Pulse then refuses typed actions that target that exact resource: the Assistant declines to plan one when it can read the lock, Patrol will not run one, and the action lifecycle checks the lock again before it runs an action that was already planned, even when approved. The lock covers that resource only, not its host or other resources, and it does not stop read-only investigation. Turn it off and save again when you want Pulse to act on the resource again. A resource whose lifecycle is Retired is blocked the same way until the lifecycle is set back to Active.

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
