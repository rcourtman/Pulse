# Unsaved edit retention — 7 September 2026

`scripts/check-recovery-feedback.mjs` now gives the Destinations fixture a
reactive ping URL and an observable unsaved flag instead of no-op setters.
Each Destinations case enters a synthetic `example.invalid` URL and checks
both the rendered input and parent signal, plus the unsaved flag, after:

- rejected retry and toast expiry;
- cancelled dismissal;
- accepted retry followed by unavailable health refresh;
- subsequent recovery, failed dismissal, healthy-card removal and message clearing.

Validation: `pulse-heavy-run -- node scripts/check-recovery-feedback.mjs` passed
12 Chromium cases: Overview and Destinations at 1440, 900 and 390 pixels in
light and dark themes. Six cases include the unsaved edit assertions.

This is component qualification with scripted APIs. It does not establish
full settings-parent integration, saved configuration, installed recovery,
recipient receipt or assistive-technology announcements. Existing screenshots
are not refreshed by this evidence record; no new visual acceptance is claimed.

Independent rationale: W3C's [redundant-entry guidance](https://www.w3.org/WAI/WCAG22/Understanding/redundant-entry.html),
retrieved through search on 7 September (direct retrieval returned HTTP 429),
describes the burden and error risk of re-entering information. This supports
protecting input retention, not a claim of WCAG conformance or new product demand.
