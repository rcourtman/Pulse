# Resource payload static-metadata bloat (browser performance audit, 2026-08-24)

## Context

A read-only browser performance audit (2026-08-23/24, production build served by
the dev backend against the 50-node / 929-guest / 1,508-resource mock estate)
measured the client-facing resource stream as a primary scalability cost:

- The full state snapshot is **4.73 MB** of JSON in a single websocket frame
  (`/api/state` REST recovery returns the same payload). `resources` accounts
  for 4.63 MB — ~3 KB average per resource.
- Idle on the Proxmox overview, the browser main thread was blocked for
  **12.0 s out of every 45 s** (individual long tasks of 0.8–1.4 s roughly
  every 3–4 s, aligned with resource delta frames of up to ~500 KB) on an
  unthrottled M-series desktop. Under a 4x CPU throttle (mid-range phone
  class), tab taps landing inside those windows blocked for multiple seconds.

## Measured payload composition (per-field bytes across 1,508 resources)

| Field | Bytes | Notes |
| --- | --- | --- |
| `canonicalIdentity` | 0.82 MB | 12 aliases incl. 7 superseded hash ids on a typical VM; identity history ships on every snapshot |
| `proxmox` + `platformData` | 1.36 MB | platform payloads |
| `capabilities` | 0.34 MB | only **10 distinct blobs** across all 1,508 resources — duplicated per resource, including human-readable `description` strings |
| `aiSafeSummary` | 0.14 MB | AI-oriented prose on every resource, shipped to every browser client |
| `policy` | 0.12 MB | near-constant routing metadata per resource |

The websocket delta path is already field-level (JSON merge patches in
`internal/websocket/state_delta.go`), so delta size is driven by estate scale,
not by a diffing defect. The static-metadata weight is paid on every full
snapshot, REST recovery, reconnect, and mobile tab resume.

## Why this is a governed gap rather than a quick fix

Slimming the stream means changing the client-facing resource shape (for
example: a capabilities catalog referenced by id instead of inlined blobs,
identity history behind a detail endpoint instead of inline aliases, and
audience-scoped fields so `aiSafeSummary`/`policy` do not ship to browser
sessions that never read them). That is a wire-format change with consumers
beyond frontend-modern — pulse-mobile (OTA-before-server-release constraint),
Pro/enterprise surfaces, and the AI runtime — and payload fields are not
contract-neutral under the governance rules. It needs an owned slice with
contract updates and cross-client verification, not an opportunistic patch.

## Companion fixes already landed separately

- Nav tab identity stabilization (`frontend-modern/src/components/shared/stableNavTabs.ts`)
  so websocket ticks no longer recreate every nav button.
- `preloadDynamicChunks: false` in `frontend-modern/vite.config.ts` so cold
  start no longer fetches and compiles all ~3.1 MB of lazy chunks up front.
- Per-tab scoped hydration and realtime gating on the Proxmox surface
  (`17bb2b3b7`, Performance lane).

## 2026-08-25 partial remediation (interactive session, governed claim on this gap)

Landed on `main` (same-day follow-up to the tick-pipeline fixes in
`7bac525af`):

- **Capabilities catalog**: broadcast payloads dedupe the estate's distinct
  capability blobs into a content-addressed state-level `capabilityCatalog`
  referenced per resource via `capabilitiesRef`; the websocket store expands
  refs back to inline `capabilities` at ingestion. On the 50-node mock the
  catalog is 7 entries / 2.5KB where 946 resources previously inlined the
  blobs.
- **Audience-scoped policy metadata**: default-posture resources (internal
  sensitivity, cloud-summary routing, no redactions) omit `policy` and
  `aiSafeSummary` from the stream; ingestion synthesizes the default posture
  so consumer semantics are unchanged. 806 of 1,509 mock resources shed both
  fields.
- **Alias dedupe**: broadcast `canonicalIdentity.aliases` no longer
  duplicates `supersededIds`; client identity resolution now consults
  `supersededIds` explicitly (alert overrides, thresholds, and workload
  matching already did).

Measured: `/api/state` 4.75MB -> 4.09MB (-13.9%) at the pinned mock estate.
Cross-client check: pulse-mobile and pulse-enterprise contain no reads of
`capabilities`/`canonicalIdentity`/`aiSafeSummary`/resource `policy` from
this stream; the AI runtime consumes the internal resource model, which is
untouched.

**Remaining residual (why the gap stays open):** the canonicalIdentity alias
vocabulary itself (~0.8MB) still ships inline because superseded-spelling
resolution is load-bearing client-side while
`host-identity-fork-heal-on-reenrollment` remains open; moving identity
history behind a detail endpoint needs the alert-override/threshold matching
reworked onto an on-demand lookup. Platform source payloads (~1.4MB) are
live data, not static metadata, and are out of scope for this gap.
