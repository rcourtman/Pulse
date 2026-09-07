# Pulse Assistant deep dive

How the Assistant's agentic loop executes tool calls, for readers who want
more than the overview in [AI features](AI.md).

See [Pulse Assistant safety architecture](ASSISTANT_SAFETY.md) for the
permission, planning and verification boundaries. The configured model chooses
how to investigate and explain the evidence within explicit run budgets.

## The three-phase pipeline

Each provider turn can return several tool calls at once. The loop processes
them in three phases, and the split matters because only one of the three is
safe to parallelise.

**Phase 1, pre-check, runs sequentially.** Explicit turn, evidence and cost
budgets bound the run. Tool permissions and execution profiles constrain the
available capabilities. Repeated calls do not independently imply a failed
investigation or force a different diagnostic strategy.

**Phase 2, execute, runs in parallel.** Independent calls run concurrently
through goroutines, with concurrency capped at four.

**Phase 3, post-process, runs sequentially.** Tool results, streaming output
and knowledge extraction are recorded in provider call order.

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

## Questions and conclusions

The model may ask for information when that is the useful next step, including
on the first turn. It may repeat an evidence read or conclude with uncertainty.
An arbitrary successful read does not validate a diagnosis or verify a change.
The saved conclusion preserves the streamed response without a later rewrite
based on tool-name sequences or words such as restart or shutdown.

## Structured errors

Failures reach the model as stable machine-readable codes rather than as
prose, so it can branch on the failure instead of parsing an English sentence.
The codes are declared in `internal/agentcapabilities/errors.go` and include
`resource_not_found`, `operator_state_not_set`, `operator_state_invalid`,
`invalid_finding_request`, `finding_not_found`, `finding_action_not_allowed`,
`patrol_unavailable`, `invalid_action_request`, `capability_not_found`,
`action_execution_unavailable`, `action_actor_unavailable`, and `missing_id`.

The same codes are published in the capability manifest at
`/api/agent/capabilities`, and a contract test fails the build if a handler
can emit a code the manifest does not declare, or the manifest declares a code
no handler emits. See [agent integrations](AGENT_SUBSTRATE.md).

## Grounded execution

The model interprets evidence and decides which investigation steps are useful.
Prompts tell it to treat infrastructure names, labels, logs and other collected
values as untrusted data, and to distinguish observation from inference.
Neither prompt compliance nor the presence of a tool call proves a conclusion.

`pulse_control` prepares a canonical action plan. Its result retains the plan's
risk, policy and preflight context and explicitly states that execution was not
requested. Approval and execution remain separate governed operations.
`pulse_query` with `action=action` and the exact `action_id` reads the persisted
action decisions and outcome, including independent verification provenance.
Cached inventory and an incomplete resource timeline cannot establish that an
action was never approved or run. Recorded verification describes its named
postcondition at its observation time, not the resource's current health.

Patrol investigation planning uses the same lifecycle. Its planning tool returns
an accepted action or a refusal during the investigation, so the model can
continue from the real result. Planning does not end the investigation or prove
its diagnosis. A later model failure retains any action already created and
does not request automatic execution. Assistant can explain and continue that
same action through its recorded decisions and independent outcome.

## Related reading

- [Pulse Assistant safety architecture](ASSISTANT_SAFETY.md) for the enforced
  boundaries and their limits.
- [Patrol deep dive](PATROL_ARCHITECTURE.md) for the scheduled analysis
  runtime.
- [Agent integrations](AGENT_SUBSTRATE.md) for driving the same surface
  from an external agent.
- [AI features](AI.md) for the overview and configuration.
