# Pulse Assistant safety architecture

> **Status:** the in-app Assistant chat window has been retired. This runtime
> still runs Patrol investigations, and Pulse Mobile's chat until the app
> retires on 31 March 2027. External agents reach the same governed tools
> through `pulse-mcp`.

Pulse enforces authority at the shared tool and action boundaries. The model
owns interpretation, investigation and action judgment within those boundaries.
A sequence of tool calls cannot establish that a diagnosis is correct.

## Tool kinds

Every tool call uses the shared `agentcapabilities` classification.

| Kind | Meaning |
|---|---|
| `resolve` | Discovery and query tools that find resources |
| `read` | Read-only tools such as logs, metrics, status and config |
| `write` | Tools that change Pulse state or infrastructure through governed operations |
| `user_input` | Retired interactive prompt classification (`pulse_question` is no longer offered by any profile and is refused if called) |

`ClassifyToolCall` delegates to the shared classifier, and unknown tool names
default to `write` so newly introduced tools cannot bypass governed-action
checks accidentally. A write classification is not proof that infrastructure
executed or that an action succeeded. Canonical planning itself can persist
Pulse state without changing a resource.

## Enforced authority

Execution profiles and tool permissions limit the capabilities available to a
run. The canonical action lifecycle validates the target, capability, parameters,
actor and current policy. It persists the action plan and requires the applicable
approval before execution. Raw command execution is retired
(`raw_command_retired`, HTTP `410`) in favor of advertised typed resource
actions, and callers whose Pulse role cannot run actions receive a host-command
authority refusal rather than host-command access. The model cannot grant itself
permission by describing a change as safe or by performing an unrelated read
first.

Assistant sessions (used by legacy Pulse Mobile chat until 31 March 2027) are
scoped to the caller who created them (user identity for interactive sign-ins,
or the specific API token ID for token callers) unless the caller holds
action-execution authority. Patrol tool outputs in run history, live streams,
and investigation transcripts, along with prompt-steering finding, dismissal,
and resolution notes, follow the same action-execution authority boundary.

Turn, evidence and cost budgets remain explicit bounds. Scheduling, tenant and
actor identity, idempotency and execution verification belong to their owning
services. They do not infer the quality or completeness of a diagnosis.

## Evidence and outcomes

A plan is an intended change. An approval is an authorization decision. An
execution receipt reports what the executor did. Independent verification
establishes the recorded postcondition through a separate observer at a named
time. These facts remain distinct even when one action record links them.

Assistant can read the canonical action record with `pulse_query action=action`
and its exact `action_id`. The result retains plan context, recorded decisions
and `ActionResultV2` provenance. Missing records or missing access remain unknown.
A cached resource status or absent timeline entry cannot negate an execution
receipt. A successful execution alone does not prove that the original problem
was resolved.

## Model responsibility and limits

The model chooses when to read, plan or conclude within the available
capabilities and explicit budgets. Interactive `pulse_question` prompts are
retired alongside the in-app web chat window. Assistant does not require an
unrelated read after every write, infer verification from a call sequence, or
rewrite saved answers because they contain lifecycle words. Model conclusions
can still be wrong. Regression tests, real-model qualification and independent
observations are needed to assess useful diagnosis and verified customer
outcomes.

The wider Patrol and Assistant redesign remains under qualification. Local
fixture results do not establish reliability across customer environments.

## Related reading

- [Assistant deep dive](ASSISTANT_ARCHITECTURE.md) for the execution loop.
- [AI features](AI.md) for configuration.
- [Patrol deep dive](PATROL_ARCHITECTURE.md) for the separate scheduled runtime.
