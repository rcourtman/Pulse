# AI Runtime Contract

## Contract Metadata

```json
{
  "subsystem_id": "ai-runtime",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/internal/subsystems/ai-runtime.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own Pulse Assistant and Patrol backend runtime behavior, AI orchestration,
runtime cost control, and shared AI transport surfaces.

## Canonical Files

1. `internal/ai/`
2. `internal/api/ai_handler.go`
3. `internal/api/ai_handlers.go`
4. `internal/api/ai_intelligence_handlers.go`
5. `frontend-modern/src/api/ai.ts`
6. `frontend-modern/src/api/patrol.ts`
7. `frontend-modern/src/components/AI/Chat/`
8. `frontend-modern/src/utils/textPresentation.ts`

## Shared Boundaries

1. `frontend-modern/src/api/ai.ts` shared with `api-contracts`: the AI frontend client is both an AI runtime control surface and a canonical API payload contract boundary.
2. `frontend-modern/src/api/patrol.ts` shared with `api-contracts`: the Patrol frontend client is both an AI runtime control surface and a canonical API payload contract boundary.
3. `internal/api/ai_handler.go` shared with `api-contracts`: Pulse Assistant handlers are both an AI runtime control surface and a canonical API payload contract boundary.
4. `internal/api/ai_handlers.go` shared with `api-contracts`: AI settings and remediation handlers are both an AI runtime control surface and a canonical API payload contract boundary.
5. `internal/api/ai_intelligence_handlers.go` shared with `api-contracts`: AI intelligence handlers are both an AI runtime control surface and a canonical API payload contract boundary.

## Extension Points

1. Add or change chat runtime, Patrol orchestration, findings generation, or remediation behavior through `internal/ai/`
2. Add or change Pulse Assistant request flow through `internal/api/ai_handler.go` and `frontend-modern/src/api/ai.ts`
3. Add or change Patrol, alert-analysis, or remediation transport through `internal/api/ai_handlers.go`, `internal/api/ai_intelligence_handlers.go`, and `frontend-modern/src/api/patrol.ts`
4. Keep AI chat presentation helpers aligned through `frontend-modern/src/components/AI/Chat/` and the shared `frontend-modern/src/utils/textPresentation.ts`

## Forbidden Paths

1. Leaving new `internal/ai/` runtime entry points unowned under broad architecture or generic API ownership
2. Duplicating AI orchestration, Patrol runtime, or cost-tracking logic outside `internal/ai/`
3. Treating AI transport files as payload-only boundaries when they also define live runtime control behavior

## Completion Obligations

1. Update this contract when canonical AI runtime or transport entry points move
2. Keep AI runtime and shared API proof routing aligned in `registry.json`
3. Preserve explicit coverage for chat, Patrol, remediation, and cost-control behavior when AI runtime changes
4. Preserve auditability for outbound model-bound context exports and keep the export record aligned with the prompt boundary that actually reaches the provider
5. Keep AI resource and incident context aligned with the canonical unified-resource timeline before falling back to patrol-local change detectors

## Current State

This subsystem now makes Pulse Assistant and Patrol backend runtime ownership
explicit inside the current architecture lane instead of leaving those
surfaces implicit inside broad architecture or generic API ownership. A later
lane split can still promote this area into its own product lane once the
governed floor is ready.

`internal/ai/` is the live backend AI engine. It owns chat execution, Patrol
orchestration, findings generation, investigation support, quickstart and
provider selection, remediation flow, and cost persistence.

The AI transport files are shared with `api-contracts`, not delegated away to
it. `frontend-modern/src/api/ai.ts`,
`frontend-modern/src/api/patrol.ts`,
`internal/api/ai_handler.go`,
`internal/api/ai_handlers.go`, and
`internal/api/ai_intelligence_handlers.go` are runtime control surfaces for
the AI product while also remaining canonical payload contract boundaries.
The same ownership includes the Pulse query tool schema under
`internal/ai/tools/`: topology-query input names must stay canonical inside
the AI runtime itself, so new tool arguments such as `max_proxmox_nodes`
cannot reintroduce parallel legacy aliases once the backend query contract is
renamed.
Tenant-scoped AI services must now also follow canonical runtime ownership:
Patrol may initialize and operate from tenant `ReadState` and unified-resource
providers without requiring a tenant snapshot-provider bridge, and
`internal/api/ai_handlers.go` must not mint tenant-local `StateSnapshot`
adapters purely to satisfy Patrol when canonical tenant read-state is already
available.
That same AI ownership also extends to persisted runtime state under
`internal/config/persistence.go`: AI findings, usage history, patrol run
history, and chat sessions must not keep legacy plaintext files on the runtime
primary path once the process can read them. Plaintext AI persistence files may
only serve as migration input and must be rewritten immediately into
encrypted-at-rest storage on load.
The same migration-only rule applies to guest knowledge under
`internal/ai/knowledge/`: legacy `.json` knowledge files and plaintext `.enc`
knowledge files may only serve as migration input, and the knowledge store
must rewrite canonical encrypted-at-rest storage immediately on load instead
of leaving guest knowledge plaintext on disk until a future note update.
That trust boundary also applies when the store is constructed: if the
knowledge store cannot initialize encryption, construction must fail closed
instead of silently creating a plaintext-at-rest runtime store.

Unified-resource-backed AI context now also consumes the canonical
policy-aware metadata contract. The AI runtime may summarize governed resource
policy counts for context, and it must switch to `aiSafeSummary` when a
resource is marked `local-only` instead of leaking raw resource names or local
identifiers for restricted resources through ad hoc context formatting.
That governed context should also surface the canonical routing posture and
redaction hints that were derived from the shared policy model, so prompts
reflect the same sensitivity, routing, and scrub decisions that the runtime
uses for export boundaries instead of rebuilding privacy posture locally.
The same shared policy presenter also owns the routing-scope labels used in
the AI-facing policy surfaces, so the policy wording stays canonical instead
of being rendered inline by the consumer.
That same policy boundary now applies to chat structured-mention prefetch and
resource-summary formatting: mention resolution must consume canonical
unified-resource policy metadata, skip discovery fan-out when governed
redaction already blocks cloud-safe raw context, and withhold routing
coordinates, bind-mount paths, hostnames, and discovery file paths whenever
resource policy marks those identifiers as redacted.
The governed mention formatter must also render the policy line and redaction
list through the shared unified-resource policy presentation helper so the
chat prefetch path stays aligned with the same canonical sensitivity, routing,
and redaction labels used by the AI summary and resource drawer.
The decision to show that governed mention block now comes from the shared
unified-resource policy helper as well, so the local gate stays aligned with
the same routing and redaction rules as the rendered summary itself.
The governed mention preamble and footer text now also come from the shared
policy presenter, so the warning copy around the block does not drift from the
canonical policy wording.
The complete governed mention block is also assembled by the shared policy
presenter, so chat prefetch only decides when to render it and never rebuilds
the summary layout locally.
The chat prefetch path now also calls the shared governed-summary predicate
directly at each mention site, so it no longer carries a local wrapper around
the canonical policy decision or a separate mention-summary trim helper.
Structured mention resolution also uses the shared AI tools discovery
canonicalization helpers now, so chat prefetch and discovery responses agree
on resource-type and target-ID formatting instead of maintaining chat-local
copies.
The chat mention picker now also carries the canonical preferred resource
label as `label` through the structured mention payload, and the insertion
path uses that same label for prompt text and cursor placement, so mention
search, selection, and submission do not depend on a raw `displayName` field
fork.
The same governed-context rule also applies to the main unified AI resource
overview: infrastructure, workload, alert-label, and top-consumer summaries
must not leak raw resource names, cluster labels, IP addresses, or unresolved
topology identifiers once canonical resource policy marks aliases, hostnames,
platform IDs, or addresses as redacted. Sensitive resources should remain
useful through `aiSafeSummary` and explicit redaction markers rather than
falling back to raw local identifiers in list or summary sections.
That same governed policy boundary now extends through AI tool payloads and
chat-memory extraction. Resource-bearing `pulse_query` results must carry the
canonical `policy` and `ai_safe_summary` fields derived from unified resources,
and deterministic knowledge extraction must prefer those governed summaries
when policy redaction covers aliases, hostnames, or platform IDs instead of
persisting raw resource labels into cached AI facts.
That same requirement includes `pulse_query action=config`: guest-config
payloads must carry canonical resource policy metadata, and config-fact
extraction must not persist raw guest hostnames when governed redaction covers
hostname or platform identity fields.
Outbound model-bound context exports now also belong to this runtime
boundary. When the AI service assembles unified-resource context for a model
request, it must record a durable export audit with the active destination
model and governed redaction decision instead of treating the prompt boundary
as a transient formatting step.
That export decision must come from the shared unified-resource privacy
helpers, so sensitivity floors and redaction-triggered routing stay aligned
with the canonical policy contract instead of being recomputed in AI-local
code.
The export audit should also record canonical human-readable redaction labels
from the shared policy presentation helper, so the audit trail and the
resource-context surfaces speak the same governed redaction language instead
of reformatting hint names locally.
The canonical AI-safe summary builder also owns the `sensitive` and
`restricted` suffix phrases, so downstream AI consumers should treat those
ending fragments as shared policy output instead of inventing their own
wording.
The same AI runtime boundary now also consumes the canonical unified-resource
timeline when it assembles rich resource or incident context. Recent-change
context should come from the shared resource store first so AI prompts reflect
the same change record that powers the resource API, with patrol-local change
detectors only serving as fallback coverage when the canonical store is not
available. When that patrol-local fallback is used, it must render through the
shared memory change presentation helper so the same heading, scope prefix, and
change-type labels are reused instead of being rebuilt ad hoc in AI-local code.
The AI correlation root-cause engine also consumes the canonical unified-
resource relationship model directly, so cross-resource reasoning stays aligned
with the same relationship edges that back the resource API instead of
maintaining a parallel relationship vocabulary inside AI correlation.
The canonical relationship-summary helper also feeds resource change records,
so AI timeline prompts read the same relationship wording and edge labels that
the unified-resource contract emits instead of building another summary shape
in AI-local code.
The same shared change presenter also owns the resource state, restart,
incident, and config summary fragments used by change emission, so the AI
timeline prompt can reuse the canonical from/to wording before it formats the
markdown section itself.
The Patrol-backed correlation endpoint, resource-intelligence payload, and
seed prompt correlations now flow through the shared AI intelligence facade
first, so the detector remains an implementation detail behind one canonical
correlation access path instead of being routed directly by handlers or prompt
builders.
AI-facing policy metadata must also be cloned through the shared unified-
resource policy helper so chat and tools consumers do not maintain their own
policy copy logic. Chat mention prefetch now calls that shared helper directly
at each resolved mention site rather than going through an AI-local wrapper.
AI resource and intelligence consumers now also refresh canonical identity and
policy through the shared unified-resource metadata helper, so the AI runtime
no longer keeps its own slice-level normalization shim for the same
composition.
Chat knowledge extraction and resource-context rendering now also consume the
shared unified-resource label helpers directly, so governed labels and
redacted values stay consistent without AI-local presentation shims.
Those same paths also use the shared resource display-name helper, so the
name-or-ID fallback stays aligned across chat extraction, resource context,
and unified adapter presentation.
The unified resource context's IP summaries now also route through the shared
policy redaction helper, so the local "IPs" line follows the same governed
redaction decision and label vocabulary as the rest of the policy-aware
resource presentation layer. Cluster labels for AI resource context now also
come from the shared unified-resource presentation helper, so the same policy
rules govern cluster names and IP summaries instead of leaving the fallback
logic in the AI package.
The policy-posture aggregate itself now also comes from
`internal/unifiedresources/policy_posture.go`, so AI summaries and resource
context reuse the same canonical sensitivity, routing, and redaction counts
instead of collecting governance posture in an AI-local helper.
That shared presentation layer also owns the elapsed-time and "ago" wording
utilities, so the same "time ago" phrasing stays consistent across resource,
incident, and fallback memory summaries instead of being reformatted
independently.
The canonical resource-change kind, source type, and source adapter labels
now also come from the shared change presentation helper, so the resource
summary card and drawer history use the same badge vocabulary instead of
hardcoding their own labels.
Action-plan stale-plan protection now keys the durable audit payload on the
canonical `resourceVersion`, `policyVersion`, and `planHash` fields only,
so the audit record stays on the minimal deterministic contract instead of
carrying extra versioning for relationship topology.
Resource-only incident context should follow the same rule: if an alert
timeline is absent, the incident prompt path should fall back to the canonical
unified-resource timeline rather than depending only on patrol-local change
memory.

The same runtime boundary now also owns durable action execution auditing.
`internal/ai/chat/service.go` initializes the unified-resource audit store on
startup, and the write-action tool paths under `internal/ai/tools/` persist
append-only action lifecycle and action audit records through that shared
store instead of leaving command execution state in memory-only tool helpers.
The patrol-local `memory.ChangeDetector.GetChangesSummary` path now also
delegates to the shared memory recent-change presentation helper, so any
future fallback summary entry point inherits the same heading, resource
prefixing, and change-type labels without re-implementing the markdown shape.
Those unified-resource action and export audit records are now also exposed
through the enterprise audit read surface so operators can inspect the
execution trail without reaching into storage internals.
AI resource and incident context now also surfaces a canonical relationship
section from unified-resource relationships, so relationship wording and edge
provenance stay aligned with the same shared resource model instead of being
reconstructed from the drawer or prompt helpers.
That relationship section is now rendered by the shared
`internal/unifiedresources.FormatResourceRelationshipContext` helper, so the
service layer only resolves the canonical resource and does not rebuild the
section format locally.
The canonical recent-change sentence formatting also lives in
`internal/unifiedresources.FormatResourceChangeSummary`, so AI runtime prompt
sections and Patrol seed context reuse the same change wording instead of
keeping another lane-local formatter.
The confidence percentage wording used by the drawer's change timeline rows
also flows through a shared frontend formatter, so the same `50%`-style
labels stay consistent across timeline surfaces instead of being re-derived
in the component.
The remaining fallback token humanization used by those same timeline and
drawer surfaces also flows through one shared frontend helper, so the
title-casing and underscore cleanup used for change and drawer labels stay
centralized instead of being reimplemented locally.
The canonical recent-change section wrapper also lives in
`internal/unifiedresources.FormatResourceRecentChangesContext`, so the AI
summary and resource-specific context share the same heading and prefix rules
instead of rebuilding that section layout locally.
The canonical memory conversion helpers also live in
`internal/ai/memory/presentation.go`, so the Patrol fallback feed and the
AI summary path translate between unified-resource changes and memory.Change
through one shared adapter boundary instead of keeping local shims.
The related-resource correlation section now also comes from the shared
correlation formatter in `internal/ai/correlation`, so resource chat and
incident prompts reuse the same learned-edge wording instead of rebuilding a
second patrol-local bullet format.
The Patrol intelligence page now also fetches the learned correlation list
from the canonical AI correlations endpoint, so the global AI surface and the
resource drawer both expose the same learned edge evidence instead of only
showing a correlation count. The same page and drawer now render that list
through the shared `frontend-modern/src/components/Infrastructure/ResourceCorrelationSummary.tsx`
card, so the learned-correlation layout and edge wording stay aligned across
both surfaces. That shared card also owns the correlation ordering and
truncation rule, so callers pass raw learned edges instead of page-specific
top-N slices.
The same page and drawer now also render their recent-change timeline through
the shared `frontend-modern/src/components/Infrastructure/ResourceChangeSummary.tsx`
card, so the canonical recent-change layout and relative-time wording stay
aligned across both surfaces instead of being rebuilt as page-local feeds.
The Patrol intelligence seed context now also prefers the canonical
unified-resource timeline before falling back to the patrol-local change
detector, so deterministic patrol context and resource detail context share
the same change source of truth.
The unified intelligence summary should follow the same rule when it counts
recent activity, so the shared AI summary and the Patrol seed context stay
aligned with the canonical timeline.
The same unified intelligence summary now also surfaces a canonical policy
posture snapshot derived from unified resources, so sensitivity, routing, and
redaction counts stay aligned with the governed resource model that the
runtime uses for prompt export and context rendering.
That posture snapshot must render redaction labels through the canonical
unified-resource hint order, not alphabetically, so the AI summary, drawer,
and any future policy surfaces all present the same redaction precedence.
Its sensitivity and routing counts must also follow the canonical
unified-resource order and shared human-readable count summaries, so both the
backend summary and the frontend policy card stay aligned on the same
presentation sequence.
The unified AI resource data-governance block must also use the shared
unified-resource redaction-label helper directly, so the same canonical
policy labels back both the posture summary and the governed prompt context
without an AI-local wrapper.
The governed query-fact and resource-context paths must also use the shared
unified-resource policy helpers for the `aiSafeSummary` decision and
redaction predicates, so the same local-only and redaction rules are applied
consistently instead of being reimplemented in chat-local helpers.
The frontend unified-resource hook now trusts backend canonical `policy` and
`aiSafeSummary` values directly, so the canonical summary and policy posture
stay aligned with the same resource-policy boundary that governs
policy-aware routing and redaction without any frontend-local re-normalization.
The resource detail drawer now also resolves the visible AI-safe summary
through the same shared policy helper, so governed resources still show the
canonical redacted label if the backend summary is missing instead of
silently dropping the summary block.
The per-resource intelligence payload returned from
`/api/ai/intelligence?resource_id=...` now carries recent changes,
dependencies, dependents, correlations, and knowledge only; policy posture
stays on the system-wide intelligence summary and the Patrol governance card
instead of riding the resource-detail payload.
That same resource-intelligence payload also carries dependency and
dependent correlation context from unified-resource correlations, so the drawer
can show canonical correlation relationships without reconstructing them from the
relationship timeline alone.
The shared AI resource and infrastructure prompt contexts should also surface
the same canonical recent changes section before any patrol-local fallback so
the model sees the same timeline entries that power the resource API and
intelligence summary counts.
The `/api/ai/intelligence/changes` endpoint should also route through the
canonical unified-intelligence recent-change accessor before any
patrol-local detector fallback, so the API surface reads the same unified
timeline source that powers the summary payload.
Those backend AI and Patrol change summaries should derive their canonical
labels and provenance fragments from
`internal/unifiedresources/change_presentation.go`, so the resource-model
semantics are shared before any surface-specific markdown styling is applied.
The patrol-local recent-change fallback itself should derive its section layout
and change labels from `internal/ai/memory/presentation.go`, so detector-based
fallbacks stay consistent across AI runtime entry points when the canonical
resource timeline is unavailable.
The per-resource intelligence payload returned from
`/api/ai/intelligence?resource_id=...` should also include the canonical
`recent_changes` history so UI and API consumers can read the same timeline
slice that the prompt context uses.
The system-wide `/api/ai/intelligence` summary should also surface the same
canonical recent-change slice, alongside the count, so the aggregate payload
and the prompt context stay aligned on the same shared timeline source.
The frontend Patrol intelligence page now also consumes that canonical
summary payload directly through the shared AI client and store, so the
visible summary card stays aligned with the same recent-change slice that the
runtime and API contracts expose.
AI chat tool-name labels, pending-tool headers, and assistant status copy now
also route through the shared frontend identifier-label helper, so the chat
surfaces do not keep their own underscore-stripping behavior separate from
the rest of the governed presentation helpers.
AI chat stream matching and mention dedupe now route through the shared
frontend chat identifier helper, so tool-name prefix stripping and mention-key
normalization stay aligned across the chat runtime instead of being redefined
inline in the stream processor or container component.
