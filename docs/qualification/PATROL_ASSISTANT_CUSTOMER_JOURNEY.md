# Patrol and Assistant customer journey qualification

The customer job is: "Tell me what needs my attention, explain why, and help me
deal with it without creating more work." Patrol owns the issue and investigation.
Assistant explains that same issue and uses existing governed action contracts.

## Active redesign plan, 2026-09-05

The maintainer requested a whole-design assessment and an explicit goal to
complete the resulting plan. The existing customer-outcomes candidate remains
the active scope under `v6-product-lane-expansion`. This is an architecture and
qualification effort, not further cosmetic refinement of the current surface.

### Product and architecture decision

Patrol owns proactive investigation of the user's infrastructure and retained
operating intent. Assistant explains and continues that same issue and its
existing investigation/action records. An observed signal, model hypothesis,
proposed action, accepted plan, executed operation and independently verified
outcome are different facts. No transition may promote one into another merely
because a tool accepted a structurally valid record.

Keep canonical resource identity, observation provenance, retained evidence,
operator intent and existing governed action records. Keep permissions, approval,
mutual exclusion, idempotency, explicit resource/cost limits, execution and
independent postcondition verification deterministic. Interpretation, relevance,
causal diagnosis, investigation choices and action judgment belong to the model.
Every orchestration pass must identify its objective invariant. A pass whose
purpose is to manufacture or force a diagnosis from proxy counts must be removed
or replaced with better model context and an explicit model decision.

The design review reproduced a concrete trust violation in
`internal/ai/chat/agentic_investigation_budget.go`: accepted proposal rationale is
called an evidence checkpoint, inserted into the final Root Cause section, and
the completion prompt forbids downgrading it in the reviewed baseline. Acceptance establishes that a
proposal was recorded, not that its causal claim is true. This mechanism must
be corrected before the diagnosis/action journey can qualify. The current
working change removes both insertion paths and the duplicated proposal prompt
state. A full-service regression now preserves the exact uncertain conclusion in the
stream, returned result and persisted session after evidence and proposal turns.
The full live investigation journey remains to be qualified.

### Baseline and measurement limits

The recorded 2026-09-05 telemetry review in the owning coverage gap used latest
reports from monitoring-active, multi-ping installations, excluding development
and deployment proof. It contained 127 paid installations, 71 with Patrol enabled
and 23 with Assistant calls. Fourteen reported verified resolutions came from one
installation. These are the previously recorded aggregate review, not a fresh
query made during this redesign. Paid includes all non-free tiers. Cooccurring
usage does not establish a linked successful task. Schema 17 outcome/provider/cost
fields had no adoption in that review, so those fields cannot establish present
customer effectiveness or model cost.

Local live proof has independently exposed lost evidence, incorrect history
coordinates, premature compaction, unsupported causal claims and roughly
three-minute interactive investigations. One maintainer installation is useful
reproduction evidence, not a representative customer success rate.

### Execution order and acceptance

| Step | Work | Acceptance | Current state |
|---|---|---|---|
| 1. Product contract and baseline | Map the current loop and sources of judgment. Record telemetry populations and gaps. | Every identified decision has an owner. Activity is not labelled usefulness. | Complete for this redesign scope. Contract, ownership decisions and baseline limits are recorded. |
| 2. Shared evidence | Preserve canonical risk reasons and SMART counters, source/time semantics and history across tools/turns. | Regression tests preserve unknown versus zero and all canonical evidence. Real responses can inspect the same facts as the product. | Implemented and qualified for the named shared-evidence defects. Canonical disk detail, risk and cadence pass real data-path proof. Affected package and concurrency checks pass. Integrated CI later exposed remaining query and allocation regressions. The final bounded query-reuse correction passes complete selected exact-base worker comparisons and full metrics/database and focused race checks. Final landing CI passed and PRs #1928 and #1929 merged. Real-model interpretation failures remain tracked in step 5. |
| 3. Diagnostic orchestration | Correct proposal-as-proof. Audit triage budgets, unmatched-signal evaluation, assessment completion and investigation cutoffs. | No code-written causal conclusion. No quality inferred from tool, flag or finding counts. Each retained pass has an objective reason. Safety boundaries and incomplete outcomes remain explicit. | Proposal promotion and capture inference were removed in c5d2f56dda. Commit 668af3fe6b removes investigation success-call floors, checkpoint instructions and generic call-count wrap-up rules. The detection slice removes contextless follow-up passes, flag/report-count policy and first-finding completion modes. Full chat and AI suites, focused API and conversation race tests pass. Real-model/action outcome qualification remains open. |
| 4. Issue through verified outcome | Follow existing issue/investigation/action records into Assistant, approval, execution and independent readback. | Accepted proposal is visibly distinct from execution and verification. Rejected or unsupported actions do not become success. Uncertainty can survive an action proposal. | Existing foundation, full journey qualification pending. |
| 5. Ground-truth qualification and landing | Extend existing qualification tooling only where necessary. Exercise healthy/unhealthy, dependency, missing-access, storage/backup and approved/rejected action cases. Inspect the final browser journey at desktop and narrow widths. | Record exact source/model/permissions, evidence, decisions, faults/misses, latency and verification. Fix in-scope failures, pass appropriate proofs and land scoped commits. | Pending. |

Use one shared runtime and the existing qualification runner, not a second
product intelligence engine or a new parallel lifecycle. Preserve independent
negative controls and fault oracles. Recorded fixtures prove contracts and
reproducibility, not live model competence. Do not tune success wording or
scoring to make the model pass.

### Diagnostic orchestration audit

| Mechanism | Current implementation | Decision for implementation review |
|---|---|---|
| Proposal rationale inserted as Root Cause | The reviewed baseline amended the conclusion after acceptance in the agent loop and service. Both mutations are removed in the working change. | Prove uncertain prose survives accepted proposals through stream, persistence and linked Assistant display. Keep the proposal record as an attributed model decision and allow uncertainty in the diagnosis. |
| Causal-resource validator | Removed the duplicate resource graph and name/status inference from the working capture boundary. Causal attribution is optional when unknown. | Prove capability/schema validation, parameter isolation and invocation integrity remain enforced. Dependency evidence remains available to the model through canonical queries. |
| Flag-count turn ladder | Removed in the detection slice. Ordinary runs have a fixed forty-turn limit, explicit quick runs retain four. | Verify flags and inventory size cannot change the execution limit or mandate findings. |
| Unmatched-signal evaluation | Removed the separate evaluator and its signal-count report budget. | Verify the original model conclusion and usage are retained without a second diagnostic session. Ground-truth missed-fault qualification remains required. |
| Missing-finding assessment sweep | Removed the separate session. | Preserve explicit assessments in the original conversation and incomplete status for omissions. No old finding excerpt may become a fresh verdict. |
| First accepted finding ends investigation | Removed the post-write summary, continuation and repair modes. | Preserve evidence tools and original context within the run limit. Accepted decisions survive a later provider failure without hiding that failure. |
| Investigation evidence-call floor | Removed in the current slice. Seed-only and failed-read conclusions survive without forced extra calls. | Completion is not diagnostic correctness. Preserve explicit limits, failed/unavailable evidence and independent action freshness checks. |
| Generic wrap-up counters | Removed the 12/18-call tool-result instructions and four silent-turn cutoff. | Explicit run limits bound work. Counts and silence do not establish evidential sufficiency. A twenty-read regression preserves available tools, observations and the model conclusion. |
| Authority and execution boundaries | Tenant identity, capability schemas, approvals, invocation IDs, parameter redaction and independent readback. | Keep and prove unchanged when diagnostic policy is simplified. These enforce objective invariants. |

This is an audited change list, not a claim that the changes are already made.
Each removal must run its focused regression and affected complete journey.

### Completion and external dependencies

The local implementation goal remains open until required qualification is
performed. Ordinary Assistant requests work with the current subscription route,
but autonomous Patrol has an explicit provider-policy refusal and remains
blocked. Do not rephrase the refused probe, bypass the readiness boundary or
count an interactive request as an autonomous Patrol pass. A supported provider
path is required for that qualification. Prepare other work while resolving the
provider dependency through supported configuration.

Release publication and wider product readiness are separate. Independent
volunteered Pro environments are still required before claiming repeatable
customer value. Capture that rollout requirement in the owning qualification gap
rather than presenting one homelab result as completion of it.

## Implemented interaction

`Explain with Assistant` on an attention item or active alert starts an explicit
explanation using the selected evidence. Opening the drawer without choosing a
task remains context only. The request preserves the composer draft and current
conversation, uses normal send/queue/retry handling, and does not enable autonomy.
Pending initialization is cancelled when the organisation changes or the drawer
component is disposed.

Attention handoffs use the canonical finding builder when exactly one finding
is linked. Otherwise they pass bounded attention evidence and typed resource and
action references without inventing a finding identity. Server-refreshed findings,
permissions, approval records, and execution policy remain authoritative.

The drawer names the selected issue, hides unrelated workflow starters while
context is attached, and explains that discovery adds service detail. Discovery
being off does not mean inventory, metrics, and alerts are unavailable.

## Browser interaction matrix

Run against the current authenticated local development build:

```sh
node scripts/check-patrol-assistant-journey.mjs
```

The script uses synthetic attention evidence and model responses, with other
non-GET application requests blocked. It writes screenshots and a result receipt
under `tmp/patrol-assistant-journey/`. It neither qualifies model reasoning nor
executes infrastructure actions. Authentication stays in browser memory.

| Surface | States and interactions |
|---|---|
| `/patrol`, 1440, 900 and 390 pixels wide | Start review, expand evidence, focus and activate Explain with Enter, inspect issue title and response, verify selected evidence/resource and one request |
| Assistant drawer | Draft preserved across close/reopen, no inference on ordinary open, no competing workflow starters, normal failed-request display and retry, reload without resubmission |
| `/patrol` Activity, 1440, 900 and 390 pixels wide | Ordinary and mirrored-alert findings, keyboard review, completed-but-unresolved investigation, failed read and nested transcript, collapse, same linked finding and uncertainty in Assistant |
| `/alerts`, desktop and narrow | Open secondary action menu, dismiss with Escape and outside click, reopen, choose explanation, verify selected alert context and no Patrol trigger |
| Initialization regression tests | Newer context cannot be cleared by an earlier send, rejected send retains context, preparation failure reports error, organisation switch cancels old evidence |

Inspect screenshots for clipping, wrapping, readable responses, reachable input,
menu placement, and visible error/retry controls. The browser verification receipt
binds the pass to exact source hashes. Rerun affected states after source edits.

## Real outcome qualification still required

A scripted response passing the interaction matrix is not evidence that the
features deliver repeatable customer value. Qualify the following jobs on a
disposable environment with known ground truth before expanding claims or autonomy.

| Job | Useful result | Controls |
|---|---|---|
| Unhealthy service | Current evidence identifies the failed service and likely cause, then gives a supported next step | Healthy service, intentional stop, missing access, correlated dependency failure |
| Backup or capacity risk | Explains actual coverage or growth risk, what is uncertain, and a concrete next step | Healthy protection, stale evidence, unavailable backup source, capacity with no safe automatic fix |
| Supported VM/LXC change | Correct canonical targets, reviewable plan, approval before execution, independent result verification attached to the original issue | Rejected approval, unsupported operation, stale target, partial failure |

Use the existing `cmd/patrol-qualify` scenario runner and
`tests/qualification/patrol/scenarios/`, with explicit live-fault and remediation
authority for the disposable lab. Validate the catalog with:

```sh
go run ./cmd/patrol-qualify -mode validate
```

The existing Docker watch/investigation/remediation catalog provides service
cases. Backup/capacity cases and the complete VM/LXC customer journey need separate
ground-truth fixtures. The existing `ProxmoxBulkLifecycleActionScenario` checks
canonical planning but does not by itself prove execution and verification.

Record source SHA, runtime version, exact provider/model, effective permissions,
scenario ground truth, observed evidence, proposed action, approval and result,
useful/incorrect/missed diagnosis, latency and tokens/cost. A correct evidenced
"no action needed" or manual hardware replacement is a useful outcome. A tool
call or an action plan alone is not.

Compare model configurations on the same cases. Establish repeatability across
independent volunteered Pro environments before treating one successful install
as a product-wide result. Keep customer content out of default telemetry.

## Measurement boundaries

Use eligible paid cohorts and latest valid installation reports. Keep current
enablement separate from historical rolling usage. Do not divide findings by
investigations or actions when their retention and provenance differ. Do not
interpret Assistant/Patrol activity cooccurrence as a linked completed task.

Provider, cost and outcome telemetry requires schema adoption before it can be
used for assessment. Link outcomes locally to canonical findings/actions, export
only content-free aggregates, and distinguish useful diagnosis, justified no-op,
blocked access, rejected action, executed action and verified resolution.

The owning governance gap is `patrol-assistant-customer-outcome-qualification`.
This interaction repair does not close that gap or authorize release publication.

## Maintainer homelab evaluation, 2026-09-05

The local development runtime used real existing connections with mock mode
disabled, `claude-subscription:claude-opus-5` through Claude CLI 2.1.211 and the
maintainer's Max login. Assistant was read-only, automatic fixes were disabled,
and the development background-work guard stayed enabled. No injected faults
or infrastructure writes were needed. This is one maintainer environment,
not independent paid-customer qualification.

The initial fleet question completed five provider turns and fourteen read-only
tool calls in 172 seconds. The transport recorded 12 input tokens and 8,406 output
tokens. Those CLI counters exclude cached input and are not complete context
usage or billable cost evidence. The visible answer was not a qualification pass:

- The temperature tool omitted a hot Proxmox node despite canonical telemetry
  reporting roughly 94 degrees Celsius. A direct read-only sensor check confirmed
  elevated temperature. The shared tool now projects Proxmox and agent readings
  with canonical identity and source, retaining sensor timestamps when available.
- The answer treated a warning-filtered count as the total alert count.
- The resource summary reported healthy while its own observations included a
  critical memory-pressure observation and omitted the thermal evidence.
- The visible final answer stopped mid-sentence. The shared artifact guard drops
  the remainder of a turn when it recognises provider-call text. The original
  pre-sanitized final response was not retained, so the precise trigger for this
  run is unproven. Completion handling and ordinary tool-reference prose need
  a focused reproduction rather than an inferred parser fix.
- Storage-risk wording exceeded the observed evidence about recoverability, and
  shared storage was asserted as the cause without a captured topology check.

The simple native subscription transport probe passed. The separate real Patrol
readiness probe passed initial connectivity and tool/context checks but its
continuation returned a provider policy refusal. The UI misleadingly framed the
incomplete probe as a latency problem. This did not qualify Patrol or justify
bypassing the provider failure. Preserve the unresolved classification issue,
provider suitability, response completeness, evidence reconciliation and actual
Patrol outcomes under the existing customer-outcome qualification gap.

The temperature-only retest received canonical per-core, package and disk
readings from the repaired tool. Its final answer still claimed that no thermal
alert existed, although that same conversation contained an active thermal
alert. Runtime tracing identified unconditional compaction after two tool turns:
by the final answer, message context had fallen to roughly 968 estimated tokens,
far below its 128,000-token fallback window. This replaced earlier observations
with abbreviated summaries before they were needed for the conclusion.

The runtime now preserves tool observations while the full request fits the
model context window. Existing overflow handling remains authoritative. A
five-turn executable regression verifies that the final request retains the
original alert, temperature and collection timestamp from all four tool rounds.
This repairs the evidence-loss mechanism without claiming that a particular
model will always reconcile evidence correctly. Canonical lookup type mismatches,
summary health, provider readiness and complete customer outcomes remain separate
qualification gaps.

With both repairs applied, the final live temperature question completed in
181 seconds. The answer retained the per-core/package and NVMe readings, the
source timestamp, and the independently returned active thermal alert with its
threshold. It delivered a complete response and distinguished missing history
and disk-health coverage from observed sensor data. This is a bounded diagnosis
improvement, not a complete pass: hardware-cause inference from a 24-hour CPU
average remained stronger than the evidence warranted, canonical lookup type
mismatches still consumed tool calls, and the roughly three-minute answer latency
remained below the interactive product bar. No remediation was requested or run.

Browser verification exercised `/patrol` at 1440x1000 and 390x1000 with the real
provider, inspected temperature tool details expanded with Enter and collapsed
with a click, scrolled the evidence and complete answer, closed and reopened the
drawer with Escape, and reloaded the persisted session. Provider selection was
also checked at `/settings/pulse-intelligence/provider`. The local receipt binds
the two changed runtime files by SHA-256. No provider responses were mocked.

A final identical readiness retry again received the provider policy refusal.
The normal manual Patrol trigger returned HTTP 409 `patrol_readiness_not_ready`.
No Patrol run or infrastructure action was accepted. Actual Patrol evaluation
with this model remains blocked on a working supported provider path.


## Resource evidence continuation, 2026-09-05

The next real read-only run exposed independent model-facing contract defects.
A canonical resource or hostname was passed directly to the in-memory metrics
store, while `pulse_summarize` already resolved the registry's metrics target.
Physical disk queries used the operation selector `type=disks` as a hardware
format filter. The advertised `node` get type was rejected. Canonical agent
projection also dropped Proxmox CPU topology. These defects are repaired in the
shared tools, with ambiguous identity rejected and canonical response IDs kept.

The first repaired performance query returned only post-restart observations.
The production tool adapter now receives the current monitor's retained SQLite
store. Resource-scoped reads use that store and return errors rather than
silently substituting shorter in-memory history. A close/reopen regression
covers Proxmox nodes, agents, VMs and system containers. Merged points are
chronological before the existing output-size limit is applied. Unscoped
summaries and baselines retain their existing providers and remain a
modernization residual.

A subsequent live Opus 5 response retrieved pre-restart observations and the
correct delly2 NVMe disk, whose SMART assessment passed at 33 degrees Celsius.
It correctly identified that the retained series covered about 31 minutes,
not the requested full day. More retained data cannot be invented. The model
still inferred a probable wear-related reason for a warning without the risk
record, and the tool omitted explicit wear units. Disk risk explanation and
coverage reconciliation remain unqualified.

The corrected incomplete-readiness projection is browser-tested using the
backend regression result, explicitly as presentation proof rather than a new
provider qualification. No provider refusal was bypassed or repeatedly retried.
The real manual Patrol endpoint remains blocked with HTTP 409. The existing
provider policy refusal still prevents a qualified autonomous Patrol run.


The named-resource repeat also misused `current_resource` from the Patrol
page, then asked which machine was intended despite `delly2` being named.
The shared boundary error had instructed the model to ask the user before any
further targeted read. It now distinguishes the unattached shortcut from an
explicit name and directs name resolution through canonical query search.
The shortcut remains blocked until attached context exists.

Live gate testing exposed an execution defect while correcting the saved
readiness verdict: both runtime and API paths treated an unassessed mode as an
executable warning. One manual read-only test run started unexpectedly and
reassessed an existing finding before the development backend was restarted to
stop it. No infrastructure mutation was performed. It is not a qualified
Patrol outcome. The runtime now blocks a provider-failed unassessed Watch result
for every selected mode, and the API consumes that execution verdict instead
of authorizing from display warnings. Operator cancellation retains its
existing non-failure behavior. The repaired live endpoint returned HTTP 409,
and the actual saved result displays latency and Watch-only as not assessed.
No new provider probe was needed to correct that persisted result.


The final named-resource retest completed without clarification in 110 seconds.
It resolved delly2 to the correct canonical agent, reported 12 CPU cores,
retrieved chronological pre-restart metrics, and attributed the KIOXIA NVMe
SMART PASSED result to the correct host. It identified the 31-minute retained
window and roughly 44-minute age of its newest point, and did not claim that
unchecked alerts were absent. Reasoning remains imperfect: child resources
were later described as guests without workload enumeration, and coincident
metric changes were treated as a probable workload start without attribution.
This is a bounded evidence-path pass, not full diagnosis qualification.

Final browser verification used `/patrol` and
`/settings/pulse-intelligence/patrol` at 1440x1000 and 390x1000. It exercised
Assistant submission and completion, keyboard/pointer tool-result expansion
and collapse, output scrolling, final-answer pixels, Escape, session history
and reload. The actual cached readiness result and actual HTTP 409 gate were
verified after the final source changes. Local receipts bind source hashes,
transcripts and screenshots under the homelab resource-browser artifact set.


## Evidence interpretation continuation, 2026-09-05

The provider refusal is now a first-class `provider_refusal` diagnostic. The
subscription transport preserves the terminal refusal before either structured
completion or a preceding tool call can be recovered. Both process exit paths
are covered. Saved legacy evidence is corrected only from a complete terminal
JSON envelope with `stop_reason=refusal`, preserving the original time and probe
counts. This produces clear policy/support guidance without a new provider
request, weakened CLI restrictions or a verified Patrol mode. The real saved
result was inspected in desktop/mobile settings and the manual endpoint still
returned HTTP 409.

The model-facing summary no longer invokes the report narrator. Its canonical
`MetricEvidenceProvider` contract reads the report engine's retained data and
returns units, extrema, means, latest values, point counts and observation
timestamps. Extrema include recorded bucket ranges and identify their buckets.
Means and latest values can be bucket averages. The response names alerts,
findings, disk health, backups and topology as not queried, requiring their own
tools before health or causal conclusions. A 92% memory reading is returned as
evidence without a heuristic critical or healthy label. Existing export/report
narratives and health cards remain separate behavior and have not been qualified
or repaired by this change.

The first live summary investigation correctly separated high utilisation from
proven memory pressure and disclosed the short, stale retained window. It still
dismissed a recorded historical temperature maximum because the current sensor
reading was lower. That is an incorrect inference, not proof that the recorded
peak is an artefact. The summary now explicitly distinguishes preserved bucket
extrema from bucket averages and includes the extrema's bucket timestamps.
Unsupported workload-change inference and assumptions about a previously
working agent channel remain model-reasoning concerns.

A separate local fixture reproduced the shared history query coverage defect.
It wrote CPU=10 in the minute tier at 21:37 and CPU=20 in raw data at 22:16.
Both 24-hour `Store.Query` and `Store.QueryAll` returned only the older point.
A two-hour query returned the newer raw point. The preferred non-empty tier
therefore hides newer evidence. Temporal tier reconciliation, including batch
queries and aggregation semantics, is recorded in the performance-and-scalability
contract under the existing outcome gap. The evidence tool exposes its returned
coverage and cannot make the underlying query complete. This is separate from
missing observations that were never collected.

The final live read-only investigation completed in 195 seconds with ten visible
tool controls. It resolved the named resource, preserved the extrema bucket
timestamps, explained the difference between extrema and plotted averages,
disclosed the 83-point / 82-minute returned span and roughly 45-minute age,
and distinguished high utilisation from demonstrated memory pressure. The
failed host pressure read was correctly treated as unavailable evidence.
However, the answer still speculated about a sensor/startup artefact, inferred
monitoring restart from coincident timestamps, claimed continuous coverage from
minute spacing, and called the returned span a retention limit. None of those
claims was established by the tool results. Recommending simply waiting a day
is insufficient given the independently reproduced tier query defect. This
is a bounded evidence-contract pass, with diagnosis qualification still open.

Final browser inspection covered `/patrol` and
`/settings/pulse-intelligence/patrol` at 1440x1000 and 390x1000. The interaction
matrix included Assistant submission/completion, keyboard and pointer expansion
and collapse, scrollable tool outputs through their final entries, final answer
pixels, Escape, session-history selection and reload. It also covered the actual
cached provider-refusal settings and blocked manual execution, with no repeated
provider readiness probe. The final saved-session pixel pass followed a correction
to the tool's descriptive governance metadata. Runtime source hashes, transcript
and screenshots are retained in the local homelab evidence-trust artifact set.
These receipts do not qualify autonomous Patrol or infrastructure mutations.


## Retained history reconciliation, 2026-09-05

The shared metrics query now reconciles each resource/metric across retained
tiers before downsampling. Preferred buckets exclude overlapping lower-priority
observations, and uncovered times and new metrics remain visible to single,
all-series and batch consumers. Rollups preserve stored extrema through later
aggregation stages. This does not recover historical peaks already discarded
or observations that were never collected. Query-plan checks exercise the actual
shared SQL builder and require indexed overlap probes.

The original local fixture now returns both the minute and newer raw point for
24-hour single/all-series reads. Regression cases cover older fallback buckets,
internal gaps, newer raw tails, overlap precedence at minute/hour/day boundaries,
resource-family isolation, per-metric filters, downsampling after reconciliation,
and single/batch parity. Customer issue #1717 independently documents the value
of API-only Proxmox history, although its earlier canonical-ID defect is distinct
from this query-tier defect. The full issue and comments were read. It has no
supplied screenshots and this work makes no new claim about that reporter's fix.

The live read-only Assistant repeat completed in 161 seconds. Its 24-hour
summary now contained 444 CPU/memory points and 436 temperature points, ending
at 22:43:54 BST, seven seconds before the summary query. The previous run returned
83 points ending at 21:39, about 45 minutes stale. The returned observation span
is now roughly 2 hours 27 minutes, with a largest timestamp gap of 61 seconds.
This validates the recovered recent tail, not a full day of collection.

The model again correctly distinguished high utilisation from demonstrated
pressure and disclosed the unavailable host agent evidence. Diagnosis remains
unqualified. It asserted continuous collection from point spacing, inferred a
monitoring/store restart from coincident timestamps, and called two temperature
readings contradictory without ruling out change between their different
observation times. It suggested treating a rising memory floor as evidence of
a leak without workload attribution. These are model-reasoning failures, not
additional proof of a storage defect or homelab fault. No infrastructure mutation
or new refused provider-readiness probe was performed.

Playwright exercised `/patrol` Assistant submission/completion, all eleven tool
controls with keyboard and pointer expansion/collapse, inner output scrolling,
answer pixels, Escape, session selection and reload at 1440x1000 and 390x1000.
The actual cached provider-refusal state at
`/settings/pulse-intelligence/patrol` and the manual HTTP 409 gate still hold.
The local retained-coverage artifact set contains transcript, current source
hashes and final browser screenshots.


The adjacent `/proxmox` node History check found a second evidence-path defect.
The view inferred a linked Agent from discovery routing, requested `agent/delly2`,
and showed Collecting history while the real source was `node/homelab-delly2`.
The canonical unified metrics target now advertises the collector's `node`
storage family and source ID for API-only Proxmox hosts, while preserving real
Agent-source precedence. The node projection carries that target to the drawer
and takes Agent linkage only from explicit metadata. Summary tools consume the
corrected target without their former Proxmox-specific exception.

Current-build browser verification exercised the node History tab at 1h, 24h
and 7d on 1440x1000 and 390x1000 viewports. Utilisation, network and thermal
charts all contained retained points. Chart hover, range changes, Overview/History
switching and reload were checked. API-only nodes no longer show Agent-only disk
throughput or a fabricated linked-Agent label. A real linked-Agent case remains
covered by drawer regression tests. No historical data was fabricated to fill
the unavailable portion of the selected range.

Worker verification passed the metrics package with the race detector
(excluding timing-sensitive SLO tests), all query SLO tests without the race
detector, and the full unified-resources package. Focused summary/reporting
regressions, the two changed frontend suites (58 tests) and frontend type checking
also passed. The node History receipt binds its final source hashes and returned
range/point metadata separately from the model transcript.

The final repeat after the canonical node-target correction completed in 215
seconds with thirteen visible tool controls. It read 194 retained points through
22:58:55 BST at 22:59:06, approximately eleven seconds old. Point count changed
as the normal rollup replaced raw points with preferred minute buckets, while
the recent tail remained available. The answer preserved the pressure-versus-
utilisation distinction and recognised a resolved thermal alert, but still
inferred a restart, continuous collection and a previously working Agent without
sufficient evidence. It also repeated the known disk-health/wear explanation
gap. These residuals remain in the outcome qualification gap. The final
retained-canonical-coverage receipts bind the current code to Assistant
expansion, output scrolling, answer pixels and reload at both viewports, plus
the actual saved provider refusal and HTTP 409 gate.


## Redesign baseline retest, 2026-09-05

The unchanged performance question completed in 224 seconds with eleven visible
tool controls after the local disk and timestamp-context changes. Playwright
exercised `/patrol` at 1440x1000 and 390x1000, including keyboard/pointer tool
expansion, output scrolling, Escape, session selection and reload. Initial answer
pixels were inspected. Final conclusion pixels and the focused disk question
remain pending, so this is not full browser qualification for the slice.

The model explicitly repeated the timestamp caveat while still calling the
returned series unbroken and inferring that the alert start almost certainly
marked collection start. It also treated a present-time pressure read as a
definitive resolution of the historical pressure question. These are failed
diagnoses, not evidence that more caveat text is sufficient.

The changed disk tool returned SMART `PASSED`, canonical `warning`, and remaining
life 63, but neither SMART counters nor risk reasons. A direct canonical resource
API read confirmed those fields are absent upstream too. The subsequent source-status read identified `proxmox.status=stale`, which
explains a freshness warning without a hardware-risk reason. The disk tool had
omitted that field too. Preserve the canonical source status alongside health
and risk before qualifying this explanation. Do not fabricate a wear warning
from 63 percent remaining.

Artifacts remain private under the local
`tmp/patrol-outcome-telemetry/homelab/diagnostic-evidence-performance` directory.
An initial browser attempt never reached a provider because the configured SSD
temporary directory was absent. The directory was restored before the recorded
run. Installed Chrome was used because the expected cached Playwright browser
was unavailable. No refused readiness probe or infrastructure mutation occurred.

The qualification scorer also needs an explicit trust review: current root-cause
grounding checks match resource names/IDs in a named Markdown section, and other
diagnosis checks use required words. Combined with code-inserted proposal prose,
this can reward a formatted assertion without independent diagnostic support.
Keep the physical fault oracles and action readback checks, but do not treat
those text checks as semantic proof of cause.

### Regression work discovered during redesign

PR #1920 CI exposed a retained-history performance regression, including the
bounded chart query and shared store reads. Reconciliation must remain correct,
but display aggregation belongs inside SQLite rather than scanning all retained
points into Go. The working correction also reuses scan destinations. Initial paired
worker benchmarks confirm reduced allocations but still show material runtime
regressions. Performance correction and source-bound browser proof remain open.

The resource API test selected the first canonical `agent` resource and assumed
it was Agent-backed. A Proxmox-only node has the same canonical resource type,
but different history storage coordinates. The test now selects the fixture's
actual Agent identity and separately asserts the Proxmox node target.

Disk warning investigation exposed another canonical gap. Proxmox physical disks
poll every five minutes by default, but source freshness currently uses the
shorter general Proxmox threshold. Registry merge status selection also requires
a regression check for recovery from stale Proxmox data. Preserve source
freshness in the tool now, and correct cadence/recovery at the owning registry
and monitoring boundary before qualifying those warnings as useful diagnosis.

### Disk live retest, 2026-09-06 local time

The current local Assistant answered the disk question through Claude Opus 5
in about three minutes. The source freshness field was visible and correctly
identified as distinct from SMART failure. The answer still failed qualification:
it invented a midnight polling restart from coincident timestamps, treated the
remaining-life measurement as ambiguous, and hit three rejected disk-detail calls.
The tool advertised `physical-disk` for get but its handler did not support it.

The next working correction routes exact canonical disk get/health requests
through the same shared projection and labels remaining life with the explicit
`life_remaining_percent` field. The live answer is a recorded failure, not proof
of completed diagnosis. The screenshots exercised `/patrol`, Assistant, tool
expansions, keyboard activation, narrow wrapping, Escape and session reload at
1440x1000 and 390x1000. Fresh browser proof is required after these later changes.

### Canonical freshness correction under verification

Physical disk collection now carries its independent polling interval into the
canonical per-source freshness record. Staleness uses at least two expected
intervals, while retaining a longer configured source threshold. A new observation
from the highest-priority available source may refresh its resource status,
including a Proxmox-only resource that previously kept its first warning.
The targeted regression covers five- and fifteen-minute disk schedules, stale
transition, identity-preserving recovery and measured hardware risk after recovery.
The live collector exposed a further loss in the quick temperature refresh:
`physicalDiskFromReadStateView` discarded cadence before writing disk records
back to monitoring state. That conversion now preserves the typed source status
schedule. Broader registry regressions and final browser proof remain required. This does not make timestamp coincidences evidence of collector restarts.

The retained-query candidate also encountered a SQLite native fault during the
full metrics package run. The same baseline package passed. This is unresolved
until the failing context and current candidate are verified. Narrow benchmark
and query-plan success do not close this failure.

### Continued orchestration review

The assessment sweep currently receives only the retained finding title, severity,
resource and up to 500 characters of old evidence. It receives neither the main
run's observations nor their collection times. That is insufficient context for a
new present/resolved judgment. The completion obligation may remain mechanical,
but any continuation must share the actual investigation context. It must not
turn old evidence into a fresh assessment.

The investigation evidence-call floor also misclassifies a failed or
approval-blocked read as no evidence at all. Such a result establishes a limit
on available access, while a successful tool call alone establishes no diagnostic
quality. Remove the quality inference and forced start-repair turn. Preserve
explicit call/turn/time limits and let an incomplete or uncertain conclusion
record the unavailable evidence. Action authority remains independently enforced.

The unmatched-signal follow-up rebuilds a reduced evidence list after the main
model run and prohibits further investigation. Its candidate selection ranks
health/reliability/backup/connectivity/anomaly categories and caps them at twenty.
This duplicates interpretation outside the model and discards the main reasoning
context. The planned replacement is the original model assessment with complete
seed/tool evidence, measured against independent missed-fault controls.

A fresh live canonical disk read after the cadence round-trip correction returned
`online` and `expectedUpdateIntervalSeconds=300`. This verifies the actual
collector-to-canonical-resource path. Full affected package tests and the current
Assistant answer remain separate acceptance checks.

### Current disk answer and browser evidence

The 2026-09-06 00:28 BST Assistant retest completed through the configured Claude
subscription route in 2m38s, with 12,436 reported tokens. All nine evidence calls
completed, including the exact canonical disk detail request that failed before.
The answer correctly interpreted 63 percent remaining life, source recency and
the distinction between disk and node temperature evidence.

The overall answer still fails qualification. It asserted that recovery exists
because an online backup datastore has space, without reading actual backup
coverage or restore evidence. It also offered host commands without checking
that execution access exists. The conclusion should remain bounded to the
observed disk state. Latency and unnecessary fleet-wide explanation remain
product defects to address in the shared diagnostic contract.

Playwright exercised `/patrol` and the Assistant drawer at 1440x1000 and
390x1000, expanded all nine evidence calls with keyboard activation, scrolled
long outputs, collapsed them, used Escape, reloaded and reopened the retained
session. Root inspection covered actual disk-list/detail output pixels, retained
summary output and the final answer through its last paragraph. The answer and
evidence remain readable at both widths. The source-bound receipt and screenshots
are private under `tmp/patrol-outcome-telemetry/homelab/diagnostic-evidence-disk-canonical-final`.
This is interface and data-path proof, not a passing diagnostic outcome.

### Retained-performance retest and transport correction

The 00:32 BST read-only performance request completed in 3m36s with 17,677
reported tokens. It correctly reported unavailable agent access and separated
high memory occupancy from established pressure. It still called the retained
slice unbroken and inferred collector startup from coincident timestamps despite
explicitly repeating the evidence caveat. That diagnosis remains failed.

This run also exposed raw serialized routing JSON in the visible conversation.
The subscription fallback had accumulated native CLI text before the first
declared tool call and forwarded it as Assistant content. It also retained only
one call from a native batch. The working provider correction returns the entire
first declared batch in order, emits no routing-turn prose, and keeps the
structured final answer unchanged. Targeted provider tests pass. All explicit
refusal and local-tool isolation boundaries remain. Fresh browser evidence is
required after this runtime change.

The transport retest exceeded the browser's five-minute limit and was cancelled
when that browser closed. The saved session contains four ordered routing turns
with 2, 5, 4 and 3 calls respectively. Their answer content is empty, so the
protocol text leak is absent from the persisted transport result. Fourteen calls
did not produce a final answer before cancellation. This is incomplete browser
qualification and failed interactive latency, not a completed diagnostic run.
The response instruction currently demands thorough investigation and suggested
next steps even after the user's specific question can be answered. Review that
shared product instruction before collecting another equally broad run.

### Scoped response and rendering retest

The 00:56 BST performance request completed in 2m42s with 13,021 reported
tokens after the shared response prompt was scoped to the user's actual job.
Routing JSON was absent, missing agent access remained explicit and memory
pressure was correctly left unknown. The answer still overstated continuous
coverage from retained timestamps, inferred history start causes, treated an
alert on the same utilization measurement as corroboration, and mixed UTC/BST.
This remains a failed diagnostic outcome. It does not qualify autonomous Patrol.

That real answer exposed a six-column table expanding the narrow conversation.
The shared sanitized renderer now gives tables their own keyboard-scrollable
region. Playwright reopened the real session at 1440x1000, 900x1000 and 390x1000,
reached the last column, verified fixed conversation width, and exercised Escape,
reload and session reopening. Browser-only short and streaming table fixtures
confirmed preserved focus, region identity and scroll position. Actual pixels
were inspected. The source-bound frontend receipt is
`frontend-modern/browser-verification.json`; private artifacts are under
`tmp/patrol-outcome-telemetry/homelab/markdown-table-final`. These fixture results
establish rendering behavior only. No additional model calls were made.

### Final evidence-foundation proof

The affected tools, chat, providers, models, unified resources, monitoring and
resource API package suites pass on the worker. The final metrics implementation
(`store.go` SHA256 `5ef4b22506ecc73131bcd881a5bbc72d2a45f43f393f2dccbfba52dc287b18e8`)
passes the full metrics suite in 90.446s, the database suite in 0.131s and the
focused concurrent read/write race proof in 19.625s. Tests cover new/deleted
history visibility, resource isolation, bounded compiled-statement retention,
one-connection operation and transaction-bound query instrumentation.

The unchanged bounded chart benchmark measures 889.3 microseconds baseline
versus 888.8 microseconds candidate (p=.796, n=10), with no significant slowdown.
Final paired metrics benchmarks pass the existing regression checker. The
single-metric downsample case reports +10.00% (p=.005, n=10), close to the
checker's greater-than-10% threshold. Raw single-series and both multi-metric
queries have no significant latency difference. These are bounded worker
measurements, not a production latency guarantee. The earlier +20.34% chart
regression is corrected by reusing compiled presence SQL while evaluating it
inside every current read snapshot. No history or tier-presence result is cached.

One earlier metrics package run terminated inside SQLite native binding code.
Subsequent full and focused race runs passed, including the final implementation.
The original fault has no established root cause and is not labelled harmless
or explained by the successful reruns. Its private worker log is
`paired-metrics-local-20260905T235637/full-metrics-final.log`.

### Diagnostic orchestration slice, 2026-09-06

The investigation loop no longer rejects a conclusion for having no successful
read or forces extra tool calls when seed context or unavailable access supports
an honest unknown conclusion. Evidence attempts still consume the configured
limit, which remains visible as a system fact. Removed completion checkpoints
and the Assistant/Watch 12/18-call wrap-up instructions from tool results, along
with the four silent-tool-turn cutoff. Overall turns, advertised capability
boundaries, configured budgets and repeated-call/error recovery remain bounded.

Full worker chat and AI packages pass against the exact changed runtime source.
The longer-read regression executes twenty distinct reads over five silent tool
turns, retains unchanged observations, and preserves the model's conclusion.
Seed-only and failed/policy-blocked read cases retain uncertainty. These are
contract tests, not proof of real diagnosis.

Browser qualification reproduced a linked-issue defect: a finding folded under
an existing alert could not stay selected because selection searched only the
non-mirrored display groups. Selection now resolves the canonical filtered
findings, preserving the existing interactive inline surface. The final
Playwright matrix opens ordinary and mirrored findings at 1440/900/390 widths,
opens the nested failed-read transcript, collapses the review and
passes the same finding, unknown conclusion and access limit to Assistant.
Scripted inference and blocked writes make this a presentation/context proof.

PR #1920's initial worker proof passed against local source 5288b64d, but its CI
comparison against the actual base 4f9de86e failed nine benchmark comparisons,
including small single-resource retained reads. Landing is blocked while the
exact base/candidate comparison is reproduced and corrected. Do not treat the
previous narrower performance pass as complete CI qualification.

### Exact-base retained-read correction

The small retained-read path performed an extra tier-presence query and opened
an explicit transaction even when the reconciliation was already one SQLite
statement. Plain reads now reuse bounded compiled reconciliation statements.
All-metric plain reads retain index time order instead of sorting by metric.
Each output series remains chronological. Display aggregation retains its
required series grouping and same-snapshot presence optimization. No data or
tier-presence result is cached.

The corrected `store.go` SHA256 is
`41132f7eae5a987e72092aceb8813a20f3dc44986c849023ffa0654e5bfe58aa`.
Ten alternating 100 ms worker samples against exact PR base 4f9de86e report:

| Query | Base | Correction | Comparison |
|---|---:|---:|---|
| Bounded single-metric API chart | 848.6 us | 894.5 us | No significant difference, p=.436 |
| Single series across resources | 916.98 us | 47.12 us | -94.86%, p<.001 |
| Single query, 500-node load fixture | 18.78 ms | 51.50 us | -99.73%, p<.001 |
| All-metric dashboard | 309.3 us | 174.3 us | -43.66%, p<.001 |
| Batch dashboard, 10 nodes | 1.912 ms | 1.723 ms | -9.86%, p=.007 |
| Batch dashboard, 50 nodes | 8.807 ms | 8.337 ms | -5.34%, p=.023 |
| Batch dashboard, 100 nodes | 17.56 ms | 16.08 ms | -8.43%, p=.002 |
| Batch dashboard, 500 nodes | 87.56 ms | 82.34 ms | No significant difference, p=.052 |

These are worker measurements, not a production latency guarantee. An exact
query-plan reproduction shows the old single-query SQL choosing the broad
tier/time index on this worker. The corrected runtime uses the series/range
index explicitly. This explains the large worker single-query difference but
does not establish which plan the prior CI runner chose. No benchmark threshold
was relaxed. The final full metrics suite passes in 87.114s, the database
suite in 0.119s and focused concurrent read/write race proof in 19.065s. The
remaining failed CI comparisons have no significant worker latency difference:
API memory fallback, 163.6 versus 167.3 us (p=.315), and the fifty-guest chart
batch, 128.0 versus 121.3 ms (p=.190). These comparisons cover the earlier failed
cases. A fresh CI run must still qualify the final landing commit. The earlier
unexplained native SQLite fault remains an open observation.

Private worker evidence is retained under
`/opt/pulse-release-worker/pr1920-bench-4f9-c5d2/`, with source-bound
`full-metrics-second.log`, `full-db-second.log`, `race-metrics-second.log`,
`second-candidate-*` and `adj-*` artifacts.


### Detection conversation redesign

The original model conversation now owns reads, finding decisions and correction
of rejected calls within the explicit run limit. Removed the separate evaluator,
old-evidence assessment sweep, signal-count report budget and post-finding prompt
replacement. The standing prompt describes Pulse's evidence, existing alerts,
active-finding obligation and action boundaries without fixed tool sequences or
forcing symptom reports before investigation. A run with no new finding may be called all clear only when the model's
evidence supports that conclusion. Final-turn and output-limit recovery instructions
also retain missing/stale evidence and avoid repeating accepted assessments.

The 01:55 UTC ordinary Assistant recheck did not yield a final answer. Server
logs record `context canceled` at 01:57:06 UTC, before the qualification runner
was interrupted at about 01:57:44 UTC. The request's eleven tool calls and partial
session remain reproduction evidence. It is a runtime failure with an unresolved
cancellation source, not a completed diagnosis. Its source/binary correspondence
has not been established as an exact final-build proof. No autonomous request or
paid alternate-provider call was made.

The rebuilt local runtime retains the same unknown cause and failed-read evidence
through ordinary and alert-mirrored finding review and the linked Assistant.
The scripted browser matrix passes at 1440x1000, 900x1000 and 390x1000, including
nested transcript expansion/collapse, keyboard review and explanation, selected
issue identity, draft preservation, menu dismissal and error/retry. Actual pixels
were inspected. This proves presentation and context continuity, not diagnosis.
The private source-bound receipt is `tmp/patrol-assistant-journey/result.json`.
Runtime binary SHA-256 before/after the pass was
`724941045a17714460f703724528a040645c752aabe01c114fc1aae6cecca52c`.

Final affected worker proof passes: full chat 13.670s, full AI 10.216s,
Patrol/API bridge tests 24.421s and focused conversation race tests 1.043s.
The race cases cover mixed accepted/rejected finding calls, a fresh evidence read
between independent findings, provider failure after an accepted finding and
exact accepted-call idempotency. The integration case retains an omitted
assessment as active and incomplete after exactly one original model run.
Zero, one and fifteen heuristic flags do not change the run limit, trigger an
auxiliary session, manufacture findings or inflate model usage. Existing
control-mode, capability and objective-observer tests remain part of the passing
full packages. Private proof logs are `/opt/pulse-release-worker/final-affected2-chat.log`
and `final-affected3-{ai,api,race}.log`.

The subsequent ordinary Assistant check on the unchanged binary completed from
02:45:25.486 to 02:48:18.734 UTC, about 173 seconds, with thirteen visible tool
records. It distinguished high utilisation from proven pressure and preserved
failed access. It still inferred observation startup from timestamp coincidence
and asserted that a change across retained resolutions was not an aggregation
artefact without supporting evidence. The temporal conclusion therefore remains
unqualified. Completion of this request does not explain the earlier cancellation
and does not qualify autonomous Patrol.

This real run also reproduced a canonical result defect: `pulse_read` with
`action=file` returned missing-agent text but a successful result. The shared
file-read boundary now returns a tool error for absent agents and nonzero
host/container command exits, preserving the original explanation. Success
continues to require actual file content. Focused negative controls cover absent
agents and both stderr/stdout error paths. The final real Assistant check executed the exact file read once and returned
`tool_end.success=false`. The failed badge and original explanation were visible
in desktop and narrow views, survived expansion/collapse and reload, and the
model described a collection limit without claiming file evidence. The single
request ran from 02:52:16.552 to 02:52:48.461 UTC. The scripted linked-issue matrix
also passed again at 1440/900/390 widths on the final rebuilt runtime.

Final binary SHA-256 was
`a575a40a640af1aaed479b56636dc5a3205c76df6999bbb20546ccf09fe70659`,
unchanged throughout both passes. `tools_file.go` SHA-256 was
`a5585c787e8a24aa5bc694b40a6c945709ecab06d9f581105131531dd16d3ac3`.
Private receipts remain under the task's
`diagnostic-evidence-file-read-failure-final/` and the repository's
`tmp/patrol-assistant-journey/`. This qualifies the named ordinary read failure
and presentation contract, not autonomous investigation or action outcomes.

The final full tools package passes in 59.413s and focused file-read race proof
in 1.033s, covering successful reads alongside the failed-read controls.
Private worker logs are `file-read-full.log` and `file-read-race.log` under
`/opt/pulse-release-worker/`.

## Integration qualification, 2026-09-06

The detection and failed-read slice is committed as `61607333cc9e`. Integration
with main `3f74c0c27304` required a fresh browser receipt. Intermediate-width
inspection exposed the shared delivery-health card squeezing its explanation
under the action buttons. The shared card now wraps by available width and uses
an opaque semantic heading colour. Permanent fixtures include 900 pixels and
check control bounds and heading overflow.

Final worker proof passes 18 delivery-ordering cases and 12 Overview refresh
cases at 1440, 900 and 390 pixels, with Overview checked in light and dark themes.
Unavailable health, retained retry/dismiss actions, pending refresh and healthy
recovery were exercised. Final card pixels were inspected at all three widths.
Frontend lint, type checking and three affected test files pass. Focused merged
Patrol/API/adapter/Docker-result proof passes. The final local scripted
`/patrol` and `/alerts` journey passes at 1440x1000, 900x1000 and 390x1000,
including nested failed-read evidence and the linked Assistant. This qualifies
the integrated presentation and context contract only. Exact staged hook and
remote landing are subsequent delivery checks. Real diagnosis and autonomous
action outcomes remain unqualified, and the provider refusal remains enforced.

## Qualification catalogue contract, 2026-09-06

The published JSON schema omitted `health_process_stop`, although the runtime
implements it and all three approved, rejected and autonomous restart scenarios
use it. A regression against the checked-in catalogue reproduced all three
rejections. The schema now accepts that implemented injector. The regression
reads the published enum and checks actual catalogue faults rather than keeping
a second injector list. This is a fault-type compatibility check, not a full
JSON Schema validator or a live-model result.

At source base `f779bf064ab4`, the corrected catalogue validates all eleven
scenarios and both qualification packages pass on pulse-dev (4.232s and 0.005s).
The private worker log is `/opt/pulse-release-worker/patrol-schema-f779.log`.
No provider request, fault injection or infrastructure mutation was performed.

The existing live runner covers Docker health, process exit/restart and network
dependency faults. It has no missing-access or storage/backup scenario or
corresponding independent lab probe. The ordinary homelab file-read failure
proof remains useful but does not close that autonomous qualification gap.
Those cases require suitable independent ground truth through the existing
qualification framework before the overall goal can complete.

## Retained-read performance correction, 2026-09-06

Build and Test run `34008823529` for integration `f779bf064ab4` failed its
benchmark comparison against exact main `3f74c0c27304`. All other jobs passed,
and Core E2E run `34008823534` passed all eight browser shards. The benchmark
failure comprised fifteen time/allocation comparisons, including batch reads
up to 59% slower and small-read bytes about 198% higher. Earlier narrow worker
comparisons omitted the actual QueryAllBatch cases and did not establish this
integrated performance result. The initial status report calling that CI job
passed was incorrect and was explicitly corrected.

The canonical reader now reuses bounded SQL templates and numbered bindings,
checks absent preferred tiers once within the current statement, and appends
consecutive results directly to their series. It retains tier reconciliation,
current snapshots, scope isolation and output semantics. No benchmark threshold
was relaxed. An initial correction still regressed the plain raw-read case by
11% and was revised before landing.

Final production `pkg/metrics/store.go` SHA-256:
`9a66d1acea82c17ca540abe9a9ee66e089a36420620e2ebb8ca7e6d102191a8d`.
Ten alternating 100ms samples on pulse-dev, Go 1.26.8 and GOMAXPROCS=4,
compare the final implementation with exact base `3f74c0c27304`. The complete
selected Query, QueryAllBatch, RollupCandidate, fleet dashboard, history API,
chart batch and NormalizeRoute benchmark families have no statistically
significant greater-than-10% regression in time, bytes or allocations. Plain
raw reads show no significant time change. Batch reads improve 16–42%, with
bytes reduced 28–34%. Single-metric downsampling is 5.66% slower and the
1,000-point rollup candidate is 6.04% slower, both inside the unchanged gate.
The bounded history API shows no significant time change. These are selected
worker comparisons, not a claim that final remote CI has passed.

Private raw samples and benchstat outputs are under
`tmp/patrol-f779-v2-selected/` and `tmp/patrol-f779-v2-normalize/` at the
workspace root. Focused tests cover newly appearing preferred buckets after a
cached absent-tier read, current resource family, metric, window and display
step bindings, per-series overlap and batch parity. Integration with main
`6c000837e27b` brings three test-only changes and no additional runtime changes.
Full metrics and database packages pass on pulse-dev in 77.265s and 0.272s.
Focused retained coverage, fresh bindings and batch identity race proof passes
in 5.349s. The two latency-based concurrent SLO tests run in the ordinary suite
but explicitly skip under the race detector. A separate canonical hot-path
regression exercises eight concurrent query scopes and mixed display steps
through a one-connection pool without latency assertions. It passes normally
(0.043s) and under race (2.106s), with no cross-query binding leakage or lock
inversion. Its worker log is `patrol-retained-concurrent-bindings.log`.
Private full/race logs are under
`tmp/patrol-f779-final-proof/`. Exact staged hook and remote landing remain
pending. This correction does not close the outstanding real-model/action outcome gap.

## Independent service-storage oracle, 2026-09-06

The qualification framework now includes
`investigation.docker-storage-pressure`, with a driver-owned fixed 8 MiB tmpfs,
a real ENOSPC write fault and an independent filesystem-statistics probe. The
service's normal writes fail and its health degrades, while an unrelated control
stays healthy. Reversion removes only the fixed fill file. The driver verifies
the prepared container ID, exact ownership labels, filesystem type and capacity
before writing. Existing fill files and symlinks are refused.

The final qualification and CLI packages pass on pulse-dev (4.223s and 0.005s).
The initial provider-free oracle run passed in 8.478s. The final live test adds
an explicit symlink outside the scratch mount, proves the target is unchanged
after refusal and reversion, and passes in 8.65s. The catalogue contains twelve
valid manifests. These tests make no Pulse API or model request.

The independent measurements establish 8,380,416 free bytes at baseline,
zero during the fault and restored free space after reversion. The service
remains running during the fault, its health changes to unhealthy, and the
control remains healthy. Final cleanup removes both disposable containers and
their network, the second cleanup is a no-op, and pre-existing inventory is
unchanged. The worker's preloaded Alpine 3.20 image digest is
`sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc`.

Production driver `internal/ai/qualification/lab_storage.go` SHA-256 is
`9e204767ddd20496c52e2566fae5690df22f840f8aac32b3af00ddc3cffc8bc8`.
Final live-test SHA-256 is
`0350997f3dbc6cbaa1877a9942e75ce500d28ecdfeff7e16fc8d8bb2cce588d1`.
Private worker logs are `patrol-storage-qualification.log` and
`patrol-storage-symlink-final.log` under `/opt/pulse-release-worker/`.

This closes the missing service-storage fault/oracle implementation, not the
required real-model diagnosis. A normal Pulse collector and supported provider
route are still required to qualify Patrol on this scenario. Host/storage-pool
and backup faults are outside this specific fixture. Missing access must be
qualified at Pulse's source or tool boundary, not substituted with an app's
unrelated file-permission fault. The overall goal and provider-refusal boundary
remain unchanged. Final staged hook and landing for this slice remain pending.

Actual published-schema validation initially rejected the new storage manifest
and three existing action manifests because `required_summary_terms` remained
mandatory despite runtime support for equivalent-term groups. The schema now
accepts either form and retains required evidence fields. A full JSON Schema
catalogue test, with negative controls for missing/empty expectations and missing
evidence, is wired into the existing Patrol regression workflow. This is distinct
from `-mode validate`, which exercises only the Go manifest validator.

All twelve manifests now pass the published schema with the CI-pinned
`jsonschema` 4.26.0. All five schema regression tests pass, including missing
and empty summary expectations and missing evidence controls. The worker log
is `/opt/pulse-release-worker/patrol-storage-schema-final.log`. Schema SHA-256
is `01ea739e2a68dc0a7b6a0041de15f5e2edcf59f31826b02a01b51a1e2b5488d`.

The final shipped guide is byte-identical to the source and passes Playwright
at `/docs/AI_PATROL_QUALIFICATION`, 1440x1000, 900x1000 and 390x1000.
The new catalogue row, storage section, command and deepest qualification
limits were inspected as pixels. The page stays within its viewport and the
long command scrolls within its own block. Reload, keyboard navigation to the
documentation index and browser-history return pass. Private matrix, screenshots
and receipt are under `tmp/patrol-storage-docs-proof/` at the workspace root.
The served guide SHA-256 is
`1970ed5cd70e976d2acbeaf7d36a7b78b5dedf65355c50a19abdba8ca96c9770`.
No model or infrastructure action is used by this browser proof.

## Disk probe completion ordering, 2026-09-06

Build and Test run `34011979848` on `f26668aa6ddc` reports a failure in
`TestCollectDisksExcludesFreeBSDFdescfsBeforeUsage`: the expected root filesystem
read did not invoke its usage probe. The shared in-flight registry published a
result before removing the completed entry. A subsequent collection could reuse
that completed result instead of taking a fresh measurement.

A controlled regression holds the registry lock while the syscall completes.
It fails on the preceding implementation because the caller returns while its
completed probe remains discoverable. Retirement and completion publication now
share one critical section. The syscall and caller waits remain outside the lock,
and overlapping collectors still share genuinely running probes. This corrects
the shared ordering contract instead of clearing state or retrying the test.

On pulse-dev with Go 1.26.8, twenty full hostmetrics package runs pass in
16.687s and three complete race-detector runs pass in 1.864s. These include
excluded mounts, stalled mounts, recovery, shared results and cancellation.
Production source SHA-256 is
`cbf5efa6bfcc163faa061ccf7c70bb6738c4ef2f78473d7d66fa00c589f87555`.
Regression file SHA-256 is
`e7a3adfe7f2cbc36cd8c315ef71ec019fa34db45b75458fa0b2212e3533b09b7`.
Both hashes match before and after proof. The raw worker log is
`/opt/pulse-release-worker/patrol-disk-probe-completion.log`.

The preceding storage slice was committed and pushed as `618700db5e` to
PR #1928, which remains open. Its exact staged hook passed 163 tests in 127.989s with all fourteen
file hashes unchanged. This supersedes the pending-hook statements above for that
slice only. Current remote CI is not a completed pass. The f266 benchmark job
also failed and its comparison remains under investigation. The overall goal,
real-model/action qualification and provider refusal remain open and unchanged.

## Large-scope query binding correction, 2026-09-06

The disk-probe slice is committed as `b964eea767` and pushed to open PR #1928.
Its final exact staged hook passes all 163 tests in 128.317s, with six unchanged
source hashes. This supersedes the pending disk-hook status above.

Build and Test run `34011979848` compares `f26668aa6ddc` against PR base
`3347f561ec7b`, rather than the earlier `3f74c0c27304` worker baseline. Its
remaining benchmark failure is the 500-node dashboard batch read: allocation
events increase 24.73%, allocated bytes increase 0.62% and runtime has no
statistically significant change. Ten alternating 100ms samples on pulse-dev
with Go 1.26.8 and GOMAXPROCS=4 reproduce all three results. The unchanged
repository benchmark checker rejects that candidate.

The pinned SQLite driver matches each numbered parameter against argument
ordinals, converting each compared ordinal to a string. Large scopes repeatedly
pay those conversions. The allocation profile identifies that conversion path.
The correction retains shared parameter values across query branches but uses
alphabetic names and `database/sql.Named`. Every read still supplies current
values. Resource identity, retention coverage, snapshots and cached shape bounds
are unchanged. Parameter names are internal positions, never user values.

An initial source-bound experiment against the same exact baseline reduces the
500-node allocation difference to +0.26% and allocated bytes to -1.68%. Runtime
has no statistically significant difference. Ten samples per variant pass the
actual repository benchmark checker. Experiment source SHA-256 is
`b73ea7984bded06b2a3319ed3a62fbe63ee87f63fd317a40b51d451f5c21b89c`.
Query-plan, fresh snapshot/window/step, concurrent binding and large-scope
binding tests pass in 0.201s. The new large-scope test reuses one cached shape
with 500 current IDs and changed family, metric filters and time window. IDs
that resemble SQL or parameter names remain data.

Private raw experiment comparisons and profiles are under
`tmp/patrol-disk-bench-go1268/` at the workspace root. The final product source
also includes an explanatory comment. Full affected package/race proof, broader
final-source comparisons and the final staged hook remain pending. The goal
remains open for real-model diagnosis, missing-access and action-outcome
qualification. The provider refusal is unchanged.

Final product source SHA-256 is
`72a7674a67ed1114be74038fb9a3d1e9c9d0507fb0d9597995f38aec0c8cbc56`.
Final test source SHA-256 is
`76be5a315e8b6dafd4a93b09711e01854087ce9f6f5a94ddb8c2ff20b3e8f708`.
Full metrics and database packages pass on Go 1.26.8 in 77.390s and 0.203s.
Focused retained/tier/binding/batch race proof, including the new 500-resource
binding case, passes in 7.707s. Both source hashes match before and after proof.
The worker log is `/opt/pulse-release-worker/patrol-named-binding-final-tests.log`.
Broader final-source performance comparison and staged-hook qualification
remain pending.

The two latency-based concurrent SLO tests are included in the ordinary full
suite and deliberately skip under the race detector. The separate concurrent
and large-scope binding regressions execute under race without latency gates.

The final broader comparison uses ten paired 100ms rounds on pulse-dev, with
baseline before candidate in each round, against exact CI base `3347f561ec7b`.
It covers all Query, QueryAllBatch, QueryManyResources and 500-node dashboard
query/concurrent-load cases, plus history API, memory-fallback control and
workload/summary chart APIs. All forty invocations succeed. Both metrics and
API comparisons pass the unchanged repository checker for time, bytes and
allocations. The 500-node read remains +0.26% in allocation events and -1.68%
in bytes versus base, with no statistically significant runtime difference.

Base source hashes match the actual git object and final source hashes match
the product manifest above. The local and worker benchmark checker hashes are
identical. An earlier setup used the invalid package path `./pkg/api` and is
discarded. The complete final run uses `./internal/api` and an actual frontend
build artifact. Its raw worker directory is
`/opt/pulse-release-worker/patrol-wide-results-corrected/`, with the final
comparisons, gates and logs copied to `tmp/patrol-named-binding-final-proof/` at
the workspace root. This establishes the selected local performance proof,
not a completed remote CI pass. Final staged-hook qualification and landing
remain pending.


### Missing diagnostic access preserves monitored identity

The named-binding correction is committed as `1f41fa174d7d` and pushed to PR
#1928 after the exact five-file staged hook passed 163 tests in 127.438s.
This supersedes the pending-hook statement above. Remote landing remains open.

The missing-access case reproduced a shared routing defect before any new
infrastructure fault was introduced. With no connected command agents, the
resolver returned before consulting monitoring topology. Known hosts, VMs and
system containers consequently lost their kind, parent and required transport.
The regression failed for all three known targets in the original source.

The shared resolver now retains topology when no server or connection exists.
A known target cannot fall through to an unrelated agent with a colliding ID.
No-target routing still requires exactly one connected agent. The file-read,
file-write, file-append, read-only execution and retained legacy command handlers
return the existing NO_AGENT failure envelope, including known resource kind and
parent node when available. The requested operation did not run. Missing access
no longer produces a successful file-write result or unsupported installation
advice. Absence of a connection does not establish a policy denial, missing
installation, guest capability, fresh observation or healthy workload.

This concerns diagnostic command access. Advertised Proxmox lifecycle actions
retain their canonical hypervisor action authority and do not acquire an
in-guest diagnostic prerequisite. Issue #1782's full body and two comments were
read as adjacent evidence of the customer harm caused by invented prerequisites.
Its requested reporter confirmation remains outstanding. No comment was sent.

The final full tools package passes on pulse-dev with Go 1.26.8 in 59.437s.
Targeted local regression passes on Go 1.27.1 in 0.538s. Worker source hashes are
unchanged across the full-package proof. Private reproduction, source manifest
and browser-result export are under `tmp/patrol-access-routing-proof/` at the
workspace root. The export invokes actual current tool handlers with controlled
connection fixtures. Its initial connected mock lacked a GetConnectedAgents
expectation and failed before export. The corrected export passes in 0.513s.
Browser fixtures prove rendering only. They do not qualify model judgment or
real command-scope enforcement. Final browser, race and staged proofs follow.


Final focused race proof explicitly runs all five new/missing-read tests and
passes in 1.051s on Go 1.26.8. An earlier broader name pattern passed but omitted
three newly named routing tests, so it is not used to claim their race coverage.
Existing API regressions for implicit monitoring-token scope and real WebSocket
rejection without agent-exec scope also pass in 0.122s. These preserve the access
boundary. They are not a model-led missing-access investigation.

Playwright and pixel inspection pass for known disconnected target, unknown
target and ordinary failed-read controls at `/patrol`, 1440x1000, 900x1000 and
390x1000. The 18 cases cover ordinary and mirrored findings, keyboard review,
completed-but-unresolved records, readable error evidence, expanded investigation
transcripts, collapse, linked Assistant explanation and deepest failed-tool
input/output, Escape and context-only reopen, plus reload without resubmission.
Nine chat requests are intercepted, with no infrastructure writes. The backend
results are serialized from actual final-source handlers with controlled
connection/command fixtures. Model conclusions and transport are scripted.
The generic Patrol toolbar opens context-only Assistant, so the proof checks
no automatic submission there rather than expecting a persisted issue session.
One permission-control pass exhausted a five-second wait while capabilities
were still loading. Its complete matrix passes with a twenty-second request
wait. This does not establish a latency SLO. Final receipts/screenshots are in
`tmp/patrol-access-browser-proof/{known-vm,unknown-target,permission-denied}/`
at the workspace root. No frontend runtime source changed.

The final routing source hashes are:

- `internal/ai/tools/tools_control.go`: `f6ac2e05542657a1d859196b06c3a68288e08b436db6dabf4f691ded744dc751`

- `internal/ai/tools/tools_file.go`: `c42002e1bf012024e1b38aa64738e0a8d9e0609699a49362675e01e7891d763e`

- `internal/ai/tools/tools_read.go`: `643a02f70432adff75355ef3083d84ab24f43c3b811bba7730cfca2ca4fd8de7`

- `internal/ai/tools/strict_resolution_test.go`: `7f88e16933c0b91b1ef7754cb7df977501fdc7278540a6ec70c1c4283e08c62d`

- `internal/ai/tools/file_docker_test.go`: `6bb8f5df1291175b9f7c52dd863293bfe89a4b189ef3ed79daeaf2f8d7adb45b`


The missing-access slice is ready for its exact staged hook. The broader goal
remains open for real-model diagnosis and approved/rejected action outcomes.
On head `1f41fa174d7d`, governance, all eight Core E2E shards and CodeQL pass.
Build and Test run `34015148620` is still pending with no jobs, so omitted PR
checks are not treated as success. PR #1928 remains open with auto-merge enabled.


### Live dependency and restart fault contracts

The missing-access slice is committed and pushed as `58caeda69ba3` to PR #1928.
Its exact eight-file staged hook passes all 163 tests in 127.706s with unchanged
hashes. The first hook environment lacked PyYAML. The complete rerun used
PyYAML 6.0.3 and jsonschema 4.26.0, with the initial failure retained separately.
This supersedes the pending-hook statement above. Remote CI remains open.

The existing dependency and action scenarios had schema/mocked-command coverage
but no explicit live Docker oracle regression alongside the storage oracle.
`TestDockerDependencyAndRestartOraclesLive` now exercises the checked-in
investigation dependency manifest and all three approved/rejected/autonomous
service-restart manifests. It shares the existing DockerLab, explicit daemon
selection and exact run-scoped cleanup. It never contacts Pulse or a model.

On pulse-dev with Go 1.26.8 and pre-existing Alpine 3.20 image digest
`sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc`,
the live package run passes in 65.080s. The dependency case takes 12.53s, and
the three service cases take 17.51s, 17.51s and 17.52s. Each records baseline,
fault, unchanged fault after refused duplicate injection, explicit fixture
recovery and restored baseline. Stopping the dependency makes its running
client unhealthy, and starting it restores both. Stopping the service health
process leaves its container running and unhealthy until explicit restart.
Every cleanup passes, second cleanup is a no-op, and pre-existing containers,
volumes, networks and images are unchanged. No image was pulled.

This proves the fault/oracle contracts, including that observation does not
repair the fixture. Direct fixture recovery is teardown. It is not a Pulse
approval, rejected-action execution, autonomous action or verified customer
outcome. The required real-model and canonical action journeys remain open.
Raw log: `/opt/pulse-release-worker/patrol-dependency-action-oracles-live.log`,
copied to `tmp/patrol-dependency-action-oracles/` at the workspace root.
The live test source SHA-256 is
`78b72dc44231cd3ecbd0b2ee925d53a5836af3141e695152046aa032580f1e48`.
The ordinary qualification and CLI packages pass in 4.224s and 0.008s on Go 1.26.8 with the explicit live environment unset. The test hash is unchanged. Commit `173d74a8e422` is pushed to PR #1928 after the exact four-file staged hook passed all 163 tests in 129.364s with unchanged source hashes. Remote landing remains pending.


### Current ordinary Assistant diagnosis remains unqualified

One read-only ordinary Assistant request on 2026-09-06 ran from
06:49:50.441Z to 06:53:26.724Z, taking 216.283s with 15 tool calls. The
configured `claude-subscription:claude-opus-5` route returned HTTP 200. The
request explicitly used `autonomous_mode=false`. No alternate paid provider,
Patrol readiness retry or infrastructure mutation was performed. This single
request establishes neither an aggregate success rate nor a latency SLO.

The missing-access correction works in this real run. One diagnostic read
returns failed `NO_AGENT` while preserving the monitored node identity, and
the answer correctly retains that access limit without retrying the read.
Diagnosis still fails factual review. The answer quotes the tool contract that
returned timestamps cannot establish collection uptime or missing-history cause,
then claims that a coincident alert timestamp identifies observation start. It
calls 181 seven-day returned points hourly even though their returned span is
under twelve hours. It moves the 83.3% memory minimum from the previous evening
at 20:34 to around 04:20, and uses low average CPU as evidence against pressure
without direct pressure measurements. Correct tool metadata did not prevent
these unsupported interpretations. Do not count a completed request or an
accurate missing-access message as successful diagnosis.

Private receipts, persisted tool outputs, request/source/binary bindings and
factual evaluation are at workspace-relative
`tmp/patrol-current-assistant-check/`. The backend binary hash remains
`1e1c3a9c0c758a4a751afa16200c6fba793acc93fd56ea62e7eb432edcf5f6c8`
before and after the request. The Playwright run exercised the persisted answer
and expanded failed-tool input/output at `/patrol`, 1440x1000 and 390x1000,
including reload. Pixel inspection confirms readable answer and error evidence.
The test intercepted non-GET requests other than the one ordinary chat. Its
blocked readiness check caused the selected-route warning in the screenshots,
so those screenshots do not establish the route's unmodified readiness UI.

The supported alternate-provider test remains awaiting its separately billed
US$20 maximum approval. The cached subscription refusal for autonomous Patrol
remains enforced. Further prompt or orchestration rules are not justified merely
because this response ignored already explicit evidence limitations. Real-model
diagnosis, approved/rejected action outcomes and wider customer readiness remain
open qualification requirements.


### Landing benchmark follow-through

Run `34017211910` on `173d74a8e422` completed with the frontend, full API
race shard, both remaining backend shards and build/smoke checks passing. Its
only failed job was the unchanged benchmark gate, with four NormalizeSegment
time regressions against exact base `3347f561ec7bc7ae30903e64998b0f90b5fb5217`.
A ten-pair, 500ms-sample worker reproduction confirmed three segment regressions,
while full middleware time and allocations remained unchanged. Both source
files and normalized compiled instruction streams matched between base and
candidate. The binary addresses differed. Layout sensitivity is an inference
from these observations, not a proven functional defect.

The canonical route-label classifiers now inspect ASCII bytes directly instead
of decoding Unicode runes that cannot satisfy the numeric/hexadecimal checks.
Existing label precedence and non-ASCII behavior are preserved, with regression
cases for long numeric IDs, Unicode digits/names and malformed UTF-8. The full
HTTP-metrics test file passes under the race detector in 1.057s on Go 1.26.8.
Ten new alternating baseline/candidate pairs pass the existing greater-than-10%,
p-less-than-0.05 gate with 500ms samples. Segment numeric, UUID, long-token,
short-name and medium-name time changes are -32.19%, -33.59%, -38.83%, -37.06%
and -37.11%. Bytes and allocations are unchanged. Adjacent route and full
middleware benchmarks have no significant regressions. These are local
microbenchmark results, not a claim about customer-perceived application speed.

Final source hashes:

- `internal/api/http_metrics.go`: `916c27ad5ff07e00170dd94df0b75eb509dd329e34288647f5358a0759353d37`
- `internal/api/http_metrics_test.go`: `4d425559d84052a50de56286101a6b37422027629d076c845920d8d02f9aff41`

Worker comparison: `/opt/pulse-release-worker/patrol-normalize-final-bench/`.
Private copies, the failed initial comparison and exact CI failure output remain
at workspace-relative `tmp/patrol-current-assistant-check/`. The preceding
diagnostic-record commit `ba69933da352` passed its two-file staged hook, all
163 tests in 127.527s with unchanged hashes. The current runtime change still
requires its final staged hook and exact-head remote CI. Overall diagnostic and
action-outcome qualification remains open.


## Healthy and dependency Assistant qualification, 2026-09-06

Two ordinary read-only Assistant requests used the configured
`claude-subscription:claude-opus-5` route with explicit `autonomous_mode=false`.
These are single-case observations, not an autonomous Patrol pass or an estimate
of customer success, false-alarm or missed-problem rates. The earlier policy
refusal remains in force. No paid-provider request was made.

The unchanged Docker dependency manifest ran through the existing `DockerLab`
on the monitored Tower host. Only run-owned Alpine containers and their private
network were created. The existing production container was untouched. Independent
Docker observations and Pulse resource convergence established healthy client and
dependency, then a stopped dependency and running-but-unhealthy client. Fault
injection used a deliberate stop with a five-second grace period. Its exit 137
therefore does not establish OOM.

| Case | Observed result | Limit |
|---|---|---|
| Healthy client | 82.835s, seven tool calls, no failed tools. Assistant correctly recommended no action. Independent observations before and after the request retained both healthy containers and unchanged identities. | The answer incorrectly inferred no contribution to or effect from host storage pressure from empty mounts and zero sampled writes. The primary decision passes this case, but the full explanation does not. |
| Stopped dependency | 204.384s, sixteen tool calls, three failed reads. Assistant identified the stopped sibling and treated the dependency explanation as a hypothesis. It preserved the command-access limit and did not claim exit 137 proved OOM. | It excluded storage causality from zero sampled I/O, overstated what `OOMKilled=false` establishes, recommended restarting the client without first establishing that need, and claimed continued failure after dependency recovery must be a client-healthcheck fault. Those claims exceed the observations. This is partial diagnostic evidence, not a qualification pass. |

The fault case also received two `app-container not found` responses from config
reads despite successful canonical get responses. That tool capability/error
contract remains an in-scope follow-up. No post-answer Docker fault observation or
explicit recovery phase was recorded before the fixture's 25-minute deadline.
Deadline cleanup completed at 08:16:40.820Z, removed both run-owned containers and
their network, then passed a second no-op cleanup with unchanged original
inventory. The helper exited on its deadline. This proves cleanup, not a completed
recovery or action-outcome journey. Neither Assistant request mutated the lab.

Private evidence root:
`/Volumes/Development/pulse/tmp/patrol-assistant-lab-readiness/runs/asst-20260906075136356-380d4d/`.
It contains independent `oracle/` receipts, healthy/fault Pulse convergence,
complete SSE streams, persisted sessions, requests, screenshots and before/after
source/binary bindings. Both requests used unchanged runtime binary SHA256
`ebd0c2c74e4dbee330284a12a99137cfcecaecfe1ec2545c25432f06183a10c1`.
HTTP windows were 07:53:58.742Z to 07:55:21.577Z and 07:58:04.234Z to
08:01:28.618Z. Displayed token counts are not complete context or billable-cost
measurements. Browser interception blocked unrelated non-GET requests, including
route checks, producing an artificial selected-route warning. It did not test
route recovery or retry the autonomous refusal.

### Tool evidence identity correction

The captured browser responses exposed a separate reproducible evidence defect.
Concurrent starts/progress used name fallback even when invocation IDs differed.
Completing one same-name action could remove another invocation's approval card.
The shared reducer now treats supplied IDs as authoritative through start,
progress, cancellation, completion and approval cleanup. Older ID-less name
matching remains the existing compatibility path.

A second reproduction showed deep Solid store reconciliation mutating objects
shared by `toolCalls` and `streamEvents`. Removing a workflow row could change the
first query's input and output into a later alerts call. Message rendering now
keys rows by message ID and reads the immutable message through an accessor.
It keeps DOM stability without copying or mutating the evidence graph. Regression
proof includes the actual status-row-removal trigger, concurrent same-name calls,
sibling approval retention and cancellation. The affected test files pass all
167 tests, including existing message mount-stability checks.

Browser matrix: `/patrol` at 1440x1000, 900x1000 and 390x1000. Captured-response
replay checks all seven healthy and sixteen dependency tool inputs and outputs
against their exact terminal SSE records. It also exercises hover/focus, keyboard
expansion/collapse, output scrolling and restored persisted sessions. A controlled
stream fixture exercises concurrent pending tools, repeated starts, progress,
out-of-order completion, independent approval cards, cancellation and failed
completion. These are renderer/identity checks, not model or action qualification.
Private replay and state receipts live under
`/Volumes/Development/pulse/tmp/patrol-assistant-lab-readiness/identity-*`.

PR #1928 merged its earlier scope at `6d2d188867430f653e1bf9ada634fd2b90440786`.
The later diagnosis record and route-label performance correction merged through
PR #1929 at `cf98358c0eb46987a82def5776fa41db5f54210a`. Its backend, frontend,
benchmarks, governance, CodeQL and eight Core E2E shards passed. The identity
correction above is a separate scoped change and requires its own landing checks.
The redesign remains open for reliable interpretation, the config-read contract,
storage/backup, approved/rejected action outcomes and supported autonomous Patrol
qualification. Wider customer readiness still requires independent Pro environments.

### Docker measurement correction plan, 2026-09-06

The next shared-source correction distinguishes absent block-I/O observations
from measured idle zero and removes the Docker layer-size ratio from filesystem
capacity. Counter presence uses the existing rate tracker contract. Missing
reports must not reset the baseline or fabricate samples. Container layer sizes
remain descriptive metadata.

Persisted Docker-family disk series previously mixed invalid capacity ratios and
unobserved I/O zeros with real measurements. New disk observations use separate
physical series keys while public metric names remain unchanged. Retained reads
exclude ambiguous legacy disk series without deleting or relabelling them. The
shared app-container storage family also serves non-Docker providers, so new
valid capacity observations must remain supported. Non-Docker series retain
existing behavior. Explicit zero must survive every retained-read API and rollup.

Browser verification is required after the final backend build. Interaction
matrix: `/docker` at 1440x1000, 900x1000 and 390x1000, container selection,
resource drawer open/close, current metrics, history expansion, measured idle,
unavailable readings, and reload. Inspect actual pixels, scrolling, focus and
Escape dismissal. `/patrol` evidence rendering must preserve absent versus zero
in current-resource and retained-history tool results. Controlled responses may
qualify rendering but cannot qualify diagnosis. Live read-only API observations
must bind to the rebuilt backend. No autonomous subscription retry or paid-model
request is authorized by this correction.

Collection also carries optional presence for each I/O direction. Explicit zero
entries survive the report JSON. For older reports without presence, only
positive counters establish an observation, so ambiguous zeros remain unavailable
until the agent is updated or a positive baseline exists. This does not require
re-enrollment. Docker-host first-disk history and network-counter presence are
adjacent limits outside this container block-I/O correction.

The final browser matrix also covers the shared host I/O table, Docker host
Overview and Machines table/tooltip at the same three widths. Partial read/write
observations must show a missing marker for the absent direction, retain measured
zero, and remain excluded from sums used for sorting and comparison. Exercise
column selection, hover/focus, tooltip dismissal and scrolling where present.

### Docker correction qualification and scope

The implementation carries per-direction presence from collection and report
JSON into the existing rate tracker, canonical resource metrics, persisted
history and resource-to-browser conversion. REST resource adaptation also
preserves optional rates. Shared rate formatting keeps missing values distinct
from zero in Machines and Docker host details. Incomplete rates do not become
complete throughput totals for sorting or comparison. A browser-discovered
first-user column migration bug is corrected in the shared preference hook, so
showing Disk I/O survives the first reload.

Legacy workload conversion in `frontend-modern/src/hooks/useWorkloads.ts` still
uses numeric direction fields with grouped availability. Its direction-level
modernization remains a separate consumer follow-up. Docker-host first-disk
history and network presence are also outside this container measurement slice.
The correction must not be represented as complete coverage of all metrics or
all monitoring surfaces. No new model competence or autonomous action result is
claimed.

Affected Go package checks and targeted race checks ran on pulse-dev with
Go1.26.8. The changed websocket assertion now expects observed read zero with
absent write omitted. Targeted frontend suites and type checking cover optional
rates, REST conversion, sorting, formatting and column persistence. Ten paired
read-benchmark rounds used the unchanged parent store via Go overlay. The
canonical >10%, p<0.05 regression gate passed, with +0.82% timing geomean in this
scoped comparison. This is not a fleet-load or full-product performance claim.

The Pro backend was cross-built on pulse-dev from the changed source and
installed into the existing local dev runtime. Binary SHA256:
`59f05f954ff8080bd3e8f3054b2b059281c49172ee771b2e454c255241158a4a`.
No production agent was replaced. Older agents remain compatible and treat
ambiguous zero counters conservatively.

Private receipts: `/Volumes/Development/pulse/tmp/patrol-docker-observed-metrics/`.
Worker logs: `/opt/pulse-release-worker/patrol-docker-observed-proof/`.
PR1934's preceding identity correction merged at
`6b0abc3bee9ffa81f6ab298b5b67ee11369688a0` with all checks passing. The current
measurement slice passed final-source Playwright inspection on `/docker`,
`/standalone/machines` and `/patrol` at 1440x1000, 900x1000 and 390x1000.
Live history, controlled absence/idle/loading/error, partial host rates, column
persistence, nested picker dismissal, tooltip focus, and expanded Assistant
evidence were exercised. The source-bound receipt is
`frontend-modern/browser-verification.json`. The unused shared host table card
has type and selector coverage, not an active-route browser claim. Controlled
responses qualify rendering only. Landing checks remain separate from the
unperformed diagnosis, approved/rejected action and recovery qualifications.

### Configuration-read correction plan

A successful canonical container get followed by a false config `not found`
result is a source contract defect. Native configuration reads must use current
canonical inventory for identity and provider capability. Optional session
resolution preserves continuity for later actions, not proof of existence.
Explicit query restrictions must be checked before registering or refreshing a
resource. Unsupported adapters, missing configuration providers, unavailable
placement and empty provider responses remain distinct from missing inventory.
No action validation or native log-read authority changes in this slice.

Regression matrix: TrueNAS config with absent, empty and existing session
context, canonical identity across aliases, explicit query denial without a
provider call, Docker unsupported capability, genuinely missing inventory,
unavailable placement, provider failure and nil provider response. Reproduce
the failing cases before changing runtime code. Run affected Go tools checks
and focused race coverage on pulse-dev.

Browser matrix after the final build: `/patrol` Assistant tool result details
at 1440x1000, 900x1000 and 390x1000, available configuration, unsupported
capability, true missing resource and denied/provider-failed results. Exercise
open/closed details, hover and keyboard focus, Enter/Space, deepest output
scrolling, Escape, and persisted/reloaded evidence. Use captured actual tool
results to qualify rendering without claiming model diagnosis or native
provider integration. No autonomous subscription retry or separately billed
provider request is part of this correction.

### Configuration-read correction qualification

The baseline reproduced absent/empty session failures, stale session placement
and false not-found results after successful canonical gets. The corrected
read path uses canonical resource identity and current provider placement. It
checks explicit query restrictions before registration and preserves an existing
query-only session's action limits. Unsupported configuration, unavailable
provider/placement and nil provider responses carry explicit reasons and the
tool error bit. Unavailable inventory and missing read state also remain failures
rather than evidence of resource absence. Actual inventory absence remains the
existing not-found lookup result.

Fourteen focused contract cases pass with strict resolution enabled. The
existing native-config regression, full tools package and focused race proof
passed on pulse-dev with Go1.26.8. The final-source Pro binary SHA256 is
`552699cdf2e61a4ca1cea2ac5ef4e065735cbd1dbca01665456e184bd4fc3533`.
It is installed only in the existing local dev stack. No production agent or
provider configuration was changed.

Playwright passed on `/patrol` at 1440x1000, 900x1000 and 390x1000. Eight actual
tool results were replayed and inspected, including successful, unavailable,
missing, denied and failed reads. Expanded inputs/outputs, keyboard toggles,
scrolling, Escape, reload and controlled session restoration preserve exact
evidence and error state. Controlled session responses prove rendering and
reload behavior, not server persistence or a new model/native-provider result.
The source-bound browser receipt records those limits. Private artifacts are
under `/Volumes/Development/pulse/tmp/patrol-config-read-contract/` and worker
logs under `/opt/pulse-release-worker/patrol-config-read-proof/`.

PR1935's Docker correction required two legacy partial-total test expectations
to be updated in `0fcb2ee147354de770dfc4b0b9672d8c2c9dceb2`. The focused 55-test
file and scoped hook passed. Its latest CI has no failures and remains pending
completion. The configuration correction still requires its own landing checks.
Real-model retest, temporal/storage interpretation, approved and rejected
action outcomes and independent recovery proof remain open. Autonomous
subscription refusal and separately billed provider approval boundaries remain
unchanged.


### Corrected storage evidence, ordinary Assistant retest

The Docker observation and canonical configuration corrections are pushed to
PR #1935 at `355ac1f0a481d6dbc9a7ff3977bced0956711979`. Their exact staged
worker pre-commit checks passed without source changes. Remote checks remain
pending. The current configuration runtime also passed all fourteen contract
cases, the complete tools package and focused race checks.

One ordinary read-only storage assessment ran on 2026-09-06 from
12:27:05.857Z to 12:30:03.726Z, an HTTP window of 177.869s. It used the existing
`claude-subscription:claude-opus-5` route, explicit `autonomous_mode=false`,
read-only control and thirteen successful tool reads. No infrastructure change,
paid-model request or autonomous readiness retry occurred. This is a single
assessment, not a success-rate or latency estimate.

The answer identified the backup datastore at 90.6% utilisation and its active
capacity warning affecting seven workloads. It used the corrected container I/O
history, separated cumulative device counters from rates, acknowledged missing
container filesystem usage, and retained the reason for missing older history
as unknown. It did not attribute older host I/O peaks to the container whose
returned I/O window starts later. These are useful observations.

The complete diagnosis still does not qualify. Its opening assurance that the
container is not short of space contradicts the later acknowledgement that
container filesystem usage is unavailable. It treats high retained host rates
as bucket/counter artifacts without establishing that mechanism. It includes a
host CPU maximum timestamped 21:00 the previous evening in a 03:00-04:00 window.
The suggested retention explanation is not established by capacity alone, and
available PBS job reads were not performed. Its rough growth extrapolation uses
retained extrema, not a measured first-to-last slope, and must retain that limit.
Corrected observations have not established reliable interpretation.

The run also highlights a tool-context distinction to review: Docker-host
`agent_connected` describes the command connection, while telemetry may still
arrive through other collection paths. The model treated current telemetry and
that false connection flag as an unresolved inconsistency. Its storage-pools
request supplied `host`, although the tool schema only advertises that filter
for RAID and Ceph detail. That call returned all pools. Neither observation
justifies fabricating resource absence or collection downtime.

Playwright exercised `/patrol` at 1440x1000 and 390x1000, the actual answer,
all thirteen expanded tool records, keyboard activation, deepest output
scrolling, Escape, reload and the persisted session. Every displayed input and
output matches the persisted tool records. Pixel review includes the answer,
evidence and the mobile table scrolled to its rightmost state. The table's
400-pixel content is reachable inside its 309-pixel horizontal viewport.
The artificial selected-route warning comes from blocked non-GET route checks,
so this does not qualify the unmodified provider-readiness UI.

The runtime binary SHA256 stayed
`552699cdf2e61a4ca1cea2ac5ef4e065735cbd1dbca01665456e184bd4fc3533`
through the request and browser pass. Private request, source, binary, tool,
persistence, evaluation and pixel receipts are under workspace-relative
`tmp/patrol-storage-assistant-check/`. No native config action was requested,
so this assessment does not qualify model use of that corrected action.
Storage-fault ground truth, reliable diagnosis, approved/rejected actions and
independent recovery remain open. The supported autonomous provider dependency
and wider independent-Pro-environment gate remain unchanged.
