# Pulse Patrol deep dive

How a Patrol run actually works, for readers who want more than the overview
in [AI features](AI.md).

The short version is that Patrol does not trust the model to notice
everything. A run pairs model judgement with deterministic detection, and the
deterministic half acts as a safety net over the model's output rather than as
an input to it.

## A run, end to end

A run moves through the following stages. Each is described in more detail
below.

1. A pre-flight budget check, which fails fast before any context is built.
2. Guest intelligence gathering, covering discovery and reachability probing.
3. Deterministic triage, which narrows the surface the model will look at.
4. Seed context construction from the triage result.
5. The agentic loop, where the model calls tools and files findings.
6. Deterministic signal detection over the tool calls the model actually made.
7. An evaluation pass for signals the model saw but did not file.
8. An assessment sweep for findings the model left unassessed.

## Budget check first

The budget check runs before context assembly and before a chat service is
acquired. A run that cannot afford to complete is stopped at the cheapest
possible point rather than after the expensive work.

## Guest intelligence

Discovery and reachability probing run before the seed context is built, so
the context the model receives already reflects which guests are reachable.
Reachability results are kept, because they feed the deterministic detection
stage later in the run.

## Deterministic triage and seed context

Triage narrows the surface first, and the seed context is then built from what
triage returned rather than from everything Pulse knows. This keeps the
model's starting context focused on what looks interesting.

Quiet infrastructure still goes through the configured model. Patrol does not
skip the model when nothing looks wrong, because "nothing is wrong" is itself
a judgement worth making explicitly.

The turn budget for the loop is computed from the size of the surface. See
`computePatrolMaxTurns` and `computeTriageMaxTurns` in
`internal/ai/patrol_ai.go`.

## The agentic loop

The model runs an agentic loop with tool access. Findings are created through
tool calls during the loop rather than parsed out of prose afterwards, so a
finding exists because the model deliberately filed it.

## Deterministic signal detection

After the loop completes, Patrol collects the tool calls that ran and scans
their results for known problem signals. This scan is deterministic and does
not depend on model judgement at all. The entry point is `DetectSignals` in
`internal/ai/patrol_signals.go`.

The signal types are:

| Signal | Meaning |
|---|---|
| `smart_failure` | A disk reporting SMART failure |
| `high_cpu` | Sustained CPU above the threshold |
| `high_memory` | Sustained memory above the threshold |
| `high_disk` | Disk or pool usage above the threshold |
| `backup_failed` | A backup job that failed |
| `backup_stale` | No backup inside the staleness window |
| `guest_unreachable` | A guest that failed reachability probing |

Thresholds come from your own alert configuration rather than from a separate
set of numbers, so detection agrees with what Pulse would alert on.
`SignalThresholdsFromPatrol` maps your configured alert thresholds onto the
signal thresholds, falling back per field to these defaults when a value is
not configured.

| Threshold | Default |
|---|---|
| Storage warning | 75% |
| Storage critical | 95% |
| High CPU | 70% |
| High memory | 80% |
| Backup staleness | 48 hours |

Backup staleness has no user-facing setting and always uses the default.

Reachability signals detected during the guest intelligence stage are merged
into the same set at this point.

## The evaluation pass

Detected signals are compared against the findings the model actually filed.
Any signal with no corresponding finding is an unmatched signal, meaning the
data showed a problem and the model did not report it.

Unmatched signals go to a bounded evaluation pass scoped to exactly those
signals. This is the safety net. It is why a run can still surface a failing
disk when the model overlooked one in a large sweep.

## The assessment sweep

Smaller models sometimes finish the main pass without recording an assessment
for every finding they filed. Patrol detects the missing assessments and runs
one bounded follow-up pass scoped to exactly those findings, rather than
rerunning the whole analysis or leaving the findings unassessed.

## Investigation

A finding may be investigated after it is filed. Investigation is a separate
loop from the run that produced the finding, coordinated through the
`InvestigationOrchestrator` contract in `pkg/aicontracts/interfaces.go`.

Whether a finding is investigated at all depends on your autonomy level and on
the finding's own state. `Finding.ShouldInvestigate` in
`internal/ai/findings.go` is the gate, and it refuses in every one of these
cases.

- Autonomy is unset or `monitor`. Investigation only runs at `approval`,
  `assisted`, or `full`.
- The finding is resolved, dismissed, suppressed, or snoozed.
- The severity is not warning or critical. Info and watch findings are not
  investigated.
- A fix is already queued for approval, so the next move belongs to you.
- The finding reached a terminal outcome, such as fix verified, resolved, fix
  rejected, cannot fix, or needs attention.
- The finding has already been investigated three times.
- The last investigation was inside the cooldown, which is one hour by
  default and shorter for timeout failures, since those are transient.

Patrol also recovers investigations that were left running and retries ones
that timed out. See `recoverStuckInvestigations` and
`retryTimedOutInvestigations` in `internal/ai/patrol_findings.go`.

## Why it is built this way

Model judgement is good at explaining and correlating, and unreliable at
exhaustive enumeration. Deterministic detection is the reverse. Patrol uses
each for what it is good at, and the unmatched-signal check is the seam where
the deterministic half can correct the model.

## Related reading

- [AI features](AI.md) for the overview and configuration.
- [Patrol autonomy](AI_AUTONOMY.md) for what each autonomy level permits.
- [Patrol qualification](AI_PATROL_QUALIFICATION.md) for the evidence and
  release-claim rules.
