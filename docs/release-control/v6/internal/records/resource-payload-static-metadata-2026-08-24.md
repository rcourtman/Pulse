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

## 2026-09-24 reliability audit measurement

The gap remains open after the September telemetry coalescing improvements.
An isolated production frontend build on `pulse-dev`, based on
`0abe518219f75512fc3f9ca0fdbd4ce549e30e90` plus the alert-admission and installer
guidance repair, used the 50-node synthetic estate. Random metrics changed
every two seconds. The measured `/api/state` response was 4,015,466 bytes.
This is a REST response measurement, not a measurement of every websocket
frame or proof of the contribution of an individual field.

The existing browser performance recipe ran on 2026-09-24 from 13:27:23 to
13:29:56 UTC. At 1440 x 900 without CPU throttling, the 30-second idle window
contained seven long tasks totalling 3,531 ms, with a maximum of 1,037 ms.
At 390 x 844 with 4x CPU throttling, seven long tasks totalled 10,104 ms, with
a maximum of 2,748 ms. Host load was 0.76/1.55/2.22 before the run and
1.47/1.55/2.12 afterwards. The recipe exited successfully, but its desktop
top-level navigation locator no longer matched the UI. Those missing steps
are not navigation proof. The idle measurements do not establish causality
between static metadata and all observed stalls.

A separate settled-server 30.02-second CPU profile contained 3.54 seconds
of CPU samples, or 11.79% of one core. Canonical identity matching accounted
for 880 ms cumulative in that profile. Three 1,000-resource component runs
measured registry rebuilds at 14.7–15.3 ms, full snapshot construction at
9.4–9.5 ms and delta construction at 0.107–0.109 ms. These results neither
reproduce a reported production CPU incident nor qualify browser
responsiveness. The remaining browser stalls require their own trace-led
investigation and cross-client contract proof before this gap can close.

Reproduction uses the performance rig recipe in
`pulse-dev-infra/docs/perf-rig.md`, with its existing `perf-audit.mjs` pointed
at the isolated synthetic server. Script SHA-256:
`b068bb3f96009418e21c63d249458447c3041212152536925ef65f694b79cde6`.
CPU profile SHA-256:
`49a75bb9ad6323238e32dfdffa5342cbc77b77282f0ddd4687df012b14488e30`.
Raw logs and profiles are retained in the local reliability audit evidence
bundle under `pulse-audit-implementation-20260924/verification/performance-baseline/`.

### Final reliability-source observation

The same recipe was repeated after the alert lifecycle, startup reconciliation,
retained-row rendering and stable synthetic identity repairs, using final
backend SHA-256
`77d17ff9e21ccdef3f5fff9f7b91b841abe3570bf8e04e1eda501c554e3a5889`
and frontend index SHA-256
`b9f1288bd188c8e549ccd4cdfa7aa75b56d49df3fc0f59de9e11dac90d449172`.
The functional browser was closed before measurement. The run lasted from
16:03:53 to 16:07:21 UTC. Although the preceding one-minute load sample was
1.88, the actual start sample rose to 2.49/2.35/3.93 and the end sample was
4.08/3.42/4.05. It therefore does not qualify as a controlled comparison or
performance improvement proof.

Desktop idle recorded seven long tasks totalling 5,949 ms, maximum 2,705 ms.
The phone viewport with 4x CPU throttling recorded eight totalling 12,971 ms,
maximum 3,732 ms. The authenticated REST snapshots were 4,003,769 and
4,003,907 bytes respectively, both HTTP 200. The unchanged desktop navigation
locators still missed several routes. The harness exit code was zero, but
those steps do not count as navigation acceptance. Functional route and policy
acceptance was performed separately in the final browser interaction matrix.

These observations leave this gap open. They do not attribute the change in
long-task cost to the source repairs or prove a particular payload field is
responsible. Stable fixture machine IDs are created at graph construction,
not during each metric refresh. The raw final run and host-load receipts are
retained under `pulse-audit-implementation-20260924/verification/performance-final/`.
