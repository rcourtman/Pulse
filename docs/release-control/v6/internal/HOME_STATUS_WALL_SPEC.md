# Home Status Wall Spec

Last updated: 2026-07-10
Status: PLANNED
Owner of record: Richard (product decisions), implementing agent (execution)
Related evidence: GitHub issues #1478, #1433, #1460

## Intent

Pulse gets a home surface: a single page that answers "is everything OK?"
for the whole fleet by showing every monitored resource as its own named
tile, colored by a composite health verdict. It becomes the default
workspace-entry route.

This is not a widget dashboard. The prior Pulse dashboard (hero StatusStrip
plus navigation cards, removed via 579b7c1e6 / ae65c5e9a) failed because it
showed counts about the infrastructure instead of the infrastructure. Tiles
are the resources themselves; each tile is the click target into its detail
surface.

The differentiator is tile semantics, not layout:

1. Green means verified. A tile is green only when the resource is running,
   its data is fresh, nothing is alerting on it, and (where applicable) its
   last backup is recent. Liveness alone is never green.
2. A red or amber tile carries its reason inline (short) and click-through
   detail (full).
3. Stale data is never green. If Pulse cannot currently verify a resource,
   the tile says so.

## Governing Rules

All of these must stay true through implementation:

1. No stat cards. The posture summary above the wall is one inline text
   line, not tiles of counts. (Standing product rule.)
2. No topology/node-link rendering, no animated packet/flow visuals. Pulse
   has containment data, not wiring data; flows are not collected. Rejected
   by decision on 2026-07-10.
3. No customization canvas (user-arranged widgets, bookmarks, iframes).
4. No new AI-synthesis surface. Patrol involvement is limited to the
   existing nav badge and the existing targeted-check handoff.
5. The verdict computation lives server-side in one canonical place and is
   shared by the wall, the summary endpoint, and future consumers. No
   page-local health heuristics in the frontend.
6. Copy: sentence case, no em dashes anywhere (including locale JSON), all
   strings through i18n dotted keys.

## Current State (verified 2026-07-10)

- `GET /api/state/summary` already exists: handler `handleStateSummary` in
  `internal/api/state_summary.go` (registered in
  `internal/api/router_routes_auth_security.go:149-150`), returning
  `{activeAlerts, nodes, vms, containers, dockerHosts[], lastUpdate}`.
  Shipped in bd6f77e09 (2026-06-04). Issue #1478 asks for this and has not
  been told; replying/closing is Richard's action, NOT the implementing
  agent's.
- Canonical resource model: `unifiedresources.Resource`
  (`internal/unifiedresources/types.go:12-82`), projected to
  `models.ResourceFrontend` (`internal/models/models_frontend.go:983-1057`)
  with `Status`, `LastSeen` (unix ms), per-source blobs including
  `Availability`.
- Availability checks (ICMP/TCP/HTTP) are already merged into unified
  resources via `SupplementalRecords` in
  `internal/monitoring/availability_poller.go:130`; probe state struct is
  `AvailabilityProbeStatus` (`availability_poller.go:25-39`).
- Active alerts: `alerts.Manager.GetActiveAlerts()`
  (`internal/alerts/read_model.go:15`); also on `StateFrontend.ActiveAlerts`.
- Backup recency: `models.VM.LastBackup` / `models.Container.LastBackup`
  (`internal/models/models.go:166,213`) plus PBS/PVE backup collections on
  `StateSnapshot`.
- No server-side trend/forecast infrastructure exists (no disk-full ETA
  anywhere; `storagehealth/topology.go:326` has a static percent threshold
  only). Predictive verdicts are therefore OUT of v1 (see Deferred).
- Frontend is SolidJS. Pages read live data via `useResources()`
  (`frontend-modern/src/hooks/useResources.ts:91`, REST `/api/resources` +
  websocket-triggered refetch), so a new page works in mock mode with no
  extra work. Routes register in `frontend-modern/src/App.tsx:556-579`;
  default entry is `getDefaultWorkspaceRoute()` (`App.tsx:118-125`); nav
  tabs live in `frontend-modern/src/AppLayout.tsx` (primary tabs memo ending
  ~:477, utility tabs :482-538).
- i18n: dotted keys in `frontend-modern/src/i18n/messages.ts` (`t()` from
  `src/i18n/index.ts`), de/es lazy override catalogs. Note some existing
  surfaces bypass i18n; this page must not.

## Workstream A — canonical health verdict (backend)

### A1. Verdict model

Add a per-resource health verdict computed during unified-resource
projection, exposed on `ResourceFrontend` as a `health` object:

```json
{
  "verdict": "ok | attention | critical | stale | off | unknown",
  "reasons": [{ "code": "backup_stale", "detail": "9d" }]
}
```

Evaluation order (first match wins), applied per resource:

1. `critical` — an active critical alert targets the resource; or the
   resource is an availability check with `Available=false` past its
   failure threshold; or an infrastructure resource (node/host/agent
   platform) is offline.
2. `attention` — an active warning alert targets the resource; or backup
   staleness (rule below); or existing `capacity_runway_low` style risk
   reasons from `storagehealth`.
3. `stale` — `LastSeen` older than the staleness threshold for its source
   type. A stale resource is never `ok` regardless of last-known status.
   (Exception: an unresolved alert older than the staleness window still
   wins per rules 1-2 — the alert is a live fact even if telemetry stopped.)
4. `off` — workload (vm/container types) in a stopped state with no active
   alerts. Off is neutral, not a failure.
5. `ok` — running/online, fresh, unalerted, backup-fresh where applicable.
6. `unknown` — inputs missing (e.g. resource type carries no status).

Backup staleness rule (v1): applies only to `vm` / `system-container`
resources that have at least one recorded backup; reason `backup_stale`
fires when the newest backup is older than 7 days. The threshold is a
single named constant with a code comment pointing at issue #839
(configurable freshness) as the future knob. Resources with no backup
history get no backup reason in v1.

Staleness thresholds: reuse whatever per-source staleness notion already
exists in the projection layer if one is found; otherwise a named constant
per source family. Do not invent per-resource configurability in v1.

Deliverable: one pure function (input: resource + active alerts + backup
index + now) in `internal/unifiedresources/` or adjacent, with table-driven
unit tests covering every verdict and precedence collision (critical alert
on stale resource, stopped workload with warning alert, availability check
below threshold, etc.).

### A2. Summary endpoint extension

Extend `stateSummaryResponse` (`internal/api/state_summary.go:14-21`)
additively — existing fields keep their names and types (external users may
already consume them):

- `verdicts`: object of counts `{ok, attention, critical, stale, off, unknown}`
- `attention`: array (capped at 50, most severe first) of
  `{id, name, type, platformType, verdict, topReason}`

Update `router_state_test.go` / `state_summary` tests accordingly. Payload
target stays well under 5 KB for a 200-resource fleet; the `attention` cap
guarantees boundedness.

Governance: this changes an API surface — update the api-contracts
subsystem notes honestly. As of 2026-07-10
`docs/release-control/v6/internal/subsystems/api-contracts.md` is dirty
with another agent's in-progress edits; if that is still true at
implementation time, stop and coordinate rather than editing around it.

## Workstream B — home page (frontend)

### B1. Route and navigation

- New path constant `HOME_PATH = '/home'` in `src/routing/resourceLinks.ts`;
  lazy route in `App.tsx`; new leftmost primary tab "Home" in
  `AppLayout.tsx` (always visible — it owns the empty state).
- `getDefaultWorkspaceRoute()` returns `HOME_PATH` when any resources are
  visible. THIS FLIP IS GATED — see Sequencing. Until the gate clears, the
  page ships reachable via nav tab only, and the default-route change is a
  separate one-line commit at the end.
- `src/__tests__/App.architecture.test.ts` pins route architecture — extend
  it, never renumber or reorder existing pinned entries.

### B2. Page structure

Component `src/features/home/HomePageSurface.tsx` with all decision logic
in `src/features/home/homePageModel.ts` (pure, unit-tested):

1. Posture strip: one inline text line — "2 need attention · 41 of 44
   healthy · 1 stale · updated 4s ago" (i18n with params, pluralization via
   params not concatenation). No card, no tiles. Do not reuse
   `StandalonePostureCard` (hardcoded English, standalone-specific); a new
   small component is correct here.
2. Attention section: when any `critical`/`attention` tiles exist, they
   render first in their own group, most severe first, regardless of
   platform.
3. Platform groups: remaining resources grouped via `useResources().byPlatform`
   ordering consistent with the nav tab order; group label = platform name
   plus host context where cheap. Within a group: tiles sorted
   non-ok-first, then name.
4. Tile: name + status tint + short reason when not ok (e.g. "down",
   "backup stale 9d", "stale 3h"). Stale = dashed border + muted. Off =
   neutral surface. Whole tile is a link to the resource's existing detail
   route (reuse `resourceLinks` helpers; do not invent new detail surfaces).
5. Group caps: groups render up to 60 ok/off tiles with a "show all (N)"
   expander. Non-ok tiles are NEVER hidden by the cap.
6. Empty state: no resources → the existing onboarding/connect pointer
   (invitation copy, no apology), consistent with what platform pages show
   pre-connection.

Live updates arrive through the existing store (websocket-triggered
refetch); no new subscription machinery. No polling loops.

### B3. Styling constraints

- Tailwind consistent with neighboring features; status tints via the
  existing status tone system (`StatusDot` / `statusBadgeModel.ts` tones),
  not hand-picked hex.
- No tables are expected on this page; if one is added anyway, column
  alignment goes through `getPlatformTableHeadClassForKind` /
  `getPlatformTableCellClassForKind` (canonical rule, pre-push lint
  enforces).
- Motion: none in v1 except the existing tone-transition defaults. No
  ambient animation.

### B4. Tests

- `homePageModel` unit tests: grouping, ordering, posture-line counts,
  cap behavior (non-ok never capped), verdict-to-tone mapping.
- Component smoke test in `src/features/home/__tests__/` following the
  StandalonePageSurface test pattern.
- Run `npx vitest run src/features/home` yourself; pre-commit hooks do not
  run vitest.

## Sequencing and gates

- Workstream A can start immediately.
- Workstream B1/B2 can start once A lands (the page renders server
  verdicts; it does not compute its own).
- The default-route flip (B1, last commit) is gated on the open
  data-integrity work: v5 parity audit closure, backups-page regression
  fixes, and platform-agent staleness generalization. A home page that
  shows false greens on day one repeats the failure that got the last
  dashboard deleted. If those items are still open when B2 is done, ship
  the page tab-reachable and leave the flip commit unmade; note it in the
  handoff report.

## Explicitly out of scope (v1)

- Predictive verdicts (disk-full ETA, backup-streak forecasting). No trend
  infrastructure exists server-side; building it is its own lane. Listed
  here so nobody bolts a regression slope onto the wall ad hoc.
- Verdict threshold configurability (backup freshness knob → issue #839).
- Any GitHub communication. Do not comment on or close #1478/#1433/#1460.
- Landing page / docs / release notes copy.
- pulse-pro / enterprise surfaces (the wall is OSS core; license-gated
  extensions come later if ever).

## Acceptance criteria

1. A fleet with one down node, one warning-alerted VM, one stale agent, one
   stopped container, and one failing availability check renders: red node
   tile with reason, amber VM tile, dashed stale tile, neutral off tile,
   red check tile — and the posture strip counts match exactly.
2. Everything healthy → posture strip reads all-healthy, wall is all green,
   nothing animates, no element demands action (quiet when green).
3. `GET /api/state/summary` returns verdict counts + capped attention list;
   existing fields unchanged; payload < 5 KB at 200 resources.
4. Verdict unit tests cover every enum value and the precedence collisions
   listed in A1.
5. Works identically in mock mode (`PULSE_MOCK_MODE=true` dev backend).
6. All new strings resolve through i18n keys in en/de/es; no em dashes in
   any added string in any locale.
7. Exercised live (dev stack, :5173) to its deepest states — expanders,
   empty state, mock fleet — before being called done.

## Handoff boundaries for the implementing agent

- Commit discipline per `AGENTS.md § Commit Discipline`: porcelain check
  first, explicit-path staging only, one scoped commit per workstream
  chunk, never `--no-verify`, stop on entangled dirty files.
- No releases, tags, GitHub posts, or emails.
- If a governing rule above conflicts with something discovered in code,
  stop and report rather than reinterpreting the rule.
