# Pulse Assistant safety architecture

The state machine that governs what the Pulse Assistant is allowed to do
during a chat session, the tool classification it runs on, and the invariants
it holds.

The point of this machine is structural. Prompt wording can be argued with by
a model, and drifts as prompts are edited. These rules are enforced in code,
in `internal/ai/chat/fsm.go`, so neither a model nor a future prompt change
can talk its way past them.

## Tool kinds

Every tool call is classified before it runs.

| Kind | Meaning |
|---|---|
| `resolve` | Discovery and query tools that find resources |
| `read` | Read-only tools such as logs, metrics, status, and config |
| `write` | Mutating tools such as restart, stop, start, delete, and file write |
| `user_input` | Interactive tools that ask you something |

Classification is `ClassifyToolCall`, which delegates to the shared
`agentcapabilities` classifier so the Assistant and the rest of the agent
surface agree on what counts as a write.

## States

A session starts in `RESOLVING` and moves between four states.

| State | What it means |
|---|---|
| `RESOLVING` | No validated target yet, so resources must be discovered first |
| `READING` | A target is established and querying is allowed |
| `WRITING` | Transitional, entered around a mutation |
| `VERIFYING` | A write happened and evidence has not been gathered since |

Transitions on a successful tool call are as follows.

- A `resolve` or `read` in `RESOLVING` moves the session to `READING`.
- A `write` from any state moves the session to `VERIFYING`, records the tool
  and timestamp, and clears the read-after-write flag.
- A `resolve` or `read` while in `VERIFYING` sets read-after-write, which is
  what satisfies the verification requirement.
- A `user_input` call does not advance state at all, because asking you a
  question is neither discovery nor verification.

`CompleteVerification` returns a verified session from `VERIFYING` to
`READING` so further writes become possible.

## The invariants

**No writing without a validated target.** A `write` attempted in `RESOLVING`
is blocked. The model must establish what it is acting on before it acts.

**No writing again until the last write is verified.** A `write` attempted in
`VERIFYING` is blocked until a read or resolve has run since the write.

**No final answer about an unverified change.** `CanFinalAnswer` refuses while
the session is in `VERIFYING` with no read-after-write. The Assistant cannot
tell you it restarted something and then decline to look at whether the
restart worked.

**Repeated attempts do not wear the gate down.** Consecutive blocked writes in
`VERIFYING` increment a counter, and that counter is telemetry only. There is
no attempt threshold after which the verification requirement is waived.

**Reads are never blocked.** No state blocks a `read`, `resolve`, or
`user_input`. The machine constrains mutation and the claims made about
mutation, not information gathering.

## Blocked calls and recovery

A blocked call returns an `FSMBlockedError` carrying the state, the tool, the
tool kind, a reason, and a recoverable flag. It surfaces to the model with the
stable code `ErrCodeFSMBlocked` rather than as prose, so the model can branch
on the code.

Blocks are recoverable rather than terminal. The session tracks a pending
recovery per blocked operation, and a later successful call of the same tool
clears it. Pending recoveries expire after ten minutes.

## Resetting

`Reset` returns the session to `RESOLVING` and clears all tracking, which is
what a full session clear does.

`ResetKeepProgress` is the softer variant used when context is cleared but
pinned items are kept. It drops verification tracking and moves a `VERIFYING`
session back to `READING`, without discarding that a target was established.

Note that `WroteThisEpisode` means "wrote at all during this session" rather
than "wrote during the current verification cycle", and `CompleteVerification`
deliberately leaves it set.

## Related reading

- [AI features](AI.md) for the overview and configuration.
- [Patrol deep dive](PATROL_ARCHITECTURE.md) for the scheduled analysis
  runtime, which is a separate loop from the Assistant.
