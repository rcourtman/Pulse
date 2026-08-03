# Pulse Assistant deep dive

How the Assistant's agentic loop executes tool calls, for readers who want
more than the overview in [AI features](AI.md).

The safety state machine has its own page. See
[Pulse Assistant safety architecture](ASSISTANT_SAFETY.md) for the states,
transitions, and invariants. This page covers the loop that runs around it.

## The three-phase pipeline

Each provider turn can return several tool calls at once. The loop processes
them in three phases, and the split matters because only one of the three is
safe to parallelise.

**Phase 1, pre-check, runs sequentially.** This is where the state machine
gate, loop detection, and budget checks happen. Every call is judged before
any call runs.

**Phase 2, execute, runs in parallel.** Independent calls run concurrently
through goroutines, with concurrency capped at four.

**Phase 3, post-process, runs sequentially.** Streaming output, state machine
transitions, and knowledge extraction happen in a deterministic order, so
concurrent execution cannot produce non-deterministic session state.

## What is not allowed to run in parallel

Parallelism is bounded by real ordering requirements rather than applied
uniformly.

A same-turn read-before-write dependency forces sequential execution. If one
turn contains both a `patrol_get_findings` read and a finding lifecycle write,
the batch runs in order, because the write's deduplication and assessment
precondition is established by that read. Letting them race inside one turn
would mean writing against a precondition that had not been checked.
Independent reads and independent finding writes still run in parallel. The
provider's original call order stays authoritative.

Interactive input is also excluded. `pulse_question` never runs in parallel
with other tools, since asking you something is not an independent operation.

## The look-before-asking gate

The Assistant is discouraged from asking you a question before it has tried to
find the answer. If the model attempts to ask without having attempted any
tool call, the attempt is blocked and it is pushed to look first.

The gate is bounded rather than absolute. It allows at most two blocks per
turn, after which the question goes through. A model that genuinely cannot
proceed without input is not trapped in a loop, and any real tool attempt
satisfies the gate immediately.

## Structured errors

Failures reach the model as stable machine-readable codes rather than as
prose, so it can branch on the failure instead of parsing an English sentence.
The codes are declared in `internal/agentcapabilities/errors.go` and include
`resource_not_found`, `operator_state_not_set`, `operator_state_invalid`,
`invalid_finding_request`, `finding_not_found`, `finding_action_not_allowed`,
`patrol_unavailable`, `invalid_action_request`, `capability_not_found`,
`action_execution_unavailable`, `action_actor_unavailable`, and `missing_id`.

A call blocked by the state machine uses the same mechanism, returning the
`fsm_blocked` code with the state and tool that were involved.

The same codes are published in the capability manifest at
`/api/agent/capabilities`, and a contract test fails the build if a handler
can emit a code the manifest does not declare, or the manifest declares a code
no handler emits. See [agent integrations](AGENT_SUBSTRATE.md).

## Grounded execution

Several guardrails exist to keep the model's claims tied to evidence it
actually gathered.

The state machine supplies the structural half. A write moves the session into
verification, and the Assistant cannot deliver a final answer about that write
until it has read something afterwards.

The prompts supply the rest. Instructions repeated across the agentic prompts
tell the model to treat infrastructure names, labels, logs, and other
collected values as untrusted data rather than as instructions, and not to
invent evidence, root cause, verification, remediation, or a claim that an
action was taken.

Prompt instructions are the weaker of the two, which is exactly why the
verification requirement lives in code instead. Where a guarantee needs to
hold, it is enforced structurally.

## Related reading

- [Pulse Assistant safety architecture](ASSISTANT_SAFETY.md) for the state
  machine in detail.
- [Patrol deep dive](PATROL_ARCHITECTURE.md) for the scheduled analysis
  runtime.
- [Agent integrations](AGENT_SUBSTRATE.md) for driving the same surface
  from an external agent.
- [AI features](AI.md) for the overview and configuration.
