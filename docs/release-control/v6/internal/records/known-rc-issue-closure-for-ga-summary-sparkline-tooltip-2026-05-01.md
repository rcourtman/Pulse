# Known RC Issue Closure For GA Summary Sparkline Tooltip Record

- Date: `2026-05-01`
- Gate: `known-rc-issue-closure-for-ga`
- Result: `passed`

## Context

Issue `#1452` reported that graph hover tooltips could overlap the highlighted
graph point and make the selected chart area hard to inspect. The screenshots
attached to the issue showed the tooltip covering the vertical hover guide on
v5 chart surfaces.

The earlier RC3 follow-up fixed the shared history-chart tooltip, but browser
inspection of the v6 Workloads route showed the same overlap class still
affected shared summary sparklines. The Workloads summary charts are owned by
`frontend-modern/src/components/shared/InteractiveSparkline.tsx` and
`frontend-modern/src/components/shared/interactiveSparklineModel.ts`, not the
history-chart model.

## Disposition

The v6 candidate now side-offsets summary sparkline tooltips through the shared
sparkline model boundary:

- `interactiveSparklineModel.ts` derives a side anchor from the hovered cursor,
  a fixed tooltip gap, the known tooltip width, and the viewport width.
- `InteractiveSparkline.tsx` renders the portal tooltip with left alignment so
  the model-owned side anchor is respected by the shared tooltip shell.
- Near the right viewport edge, the model flips the tooltip to the left of the
  cursor instead of letting the scrub guide fall under the tooltip.
- Shared primitive guardrails now assert that summary sparkline tooltip
  placement stays in the shared sparkline model and portal contract.

## Proof

- From `frontend-modern`: `npm exec vitest run src/components/shared/__tests__/InteractiveSparkline.test.tsx src/components/shared/SharedPrimitives.guardrails.test.ts`
- Desktop Playwright proof on `http://127.0.0.1:5173/workloads` with the
  onboarding overlay disabled:
  - mouse x: `224.1`
  - tooltip left: `245.0`
  - tooltip right: `365.5`
  - `separated=true`
  - `guideInsideTooltip=false`
  - screenshot: `/tmp/pulse-issue-1452/after-tooltip-desktop.png`
- Narrow mobile Playwright smoke on `http://127.0.0.1:5173/workloads`:
  - `chartSurfaces=4`
  - `visibleChartSurfaces=0`
  - screenshot: `/tmp/pulse-issue-1452/after-tooltip-mobile.png`

## Outcome

The v6 candidate no longer knowingly carries the `#1452` tooltip-overlap
failure mode on shared summary sparklines. History-chart and summary-sparkline
tooltip paths both have RC3 evidence, and the remaining behavior is governed
through the `frontend-primitives` shared chart contract.
