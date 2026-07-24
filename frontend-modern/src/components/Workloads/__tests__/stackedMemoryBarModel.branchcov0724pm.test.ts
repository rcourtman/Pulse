import { describe, expect, it } from 'vitest';

import type { AnomalyReport } from '@/types/aiIntelligence';

import type { StackedMemoryBarProps } from '../stackedMemoryBarModel';
import { buildStackedMemoryBarPresentation } from '../stackedMemoryBarModel';

// The four module-private helpers (getEffectiveCache, getUtilizationPercent,
// getSegments, getTooltipRows) are exercised through the exported
// buildStackedMemoryBarPresentation. The happy-path spec
// (stackedMemoryBarModel.test.ts) covers total > 0 with used/cache/balloon
// combinations but never drives: total <= 0 (the division guard that falls
// through to percentOnly), any swap input, or any anomaly input. Those nine
// branches are the sole focus below — every assertion checks a concrete
// observable value rather than a recomputed source expression.

const GiB = 1024 ** 3;

// Normal-band RGBA for memory (default thresholds warning 75 / critical 85);
// a 70% utilization sits below warning -> 'normal' severity color.
const NORMAL_RGBA = 'rgba(34, 197, 94, 0.6)';

const makeProps = (overrides: Partial<StackedMemoryBarProps> = {}): StackedMemoryBarProps => ({
  used: 0,
  total: 0,
  ...overrides,
});

const makeAnomaly = (overrides: Partial<AnomalyReport> = {}): AnomalyReport => ({
  resource_id: 'r1',
  resource_name: 'resource-1',
  resource_type: 'guest',
  metric: 'memory',
  current_value: 30,
  baseline_mean: 10,
  baseline_std_dev: 1,
  z_score: 20,
  severity: 'high',
  description: 'Memory spike',
  ...overrides,
});

describe('stackedMemoryBarModel (branch coverage 0724pm)', () => {
  describe('getUtilizationPercent — total <= 0 division guard (percentOnly fallback)', () => {
    it('uses percentOnly when total <= 0 and percentOnly is finite', () => {
      // Branch 8: false arm of `if (props.total > 0)` -> percentOnly path.
      // Math.max(0, Math.min(70, 100)) === 70.
      const p = buildStackedMemoryBarPresentation(makeProps({ total: 0, percentOnly: 70 }), 400);
      expect(p.displayPercentValue).toBe(70);
      expect(p.displayLabel).toBe('70%');
    });

    it('clamps percentOnly > 100 down to 100', () => {
      // Math.max(0, Math.min(150, 100)) === 100 — the upper clamp arm.
      const p = buildStackedMemoryBarPresentation(makeProps({ total: 0, percentOnly: 150 }), 400);
      expect(p.displayPercentValue).toBe(100);
      expect(p.displayLabel).toBe('100%');
    });

    it('clamps percentOnly < 0 up to 0', () => {
      // Math.max(0, Math.min(-5, 100)) === 0 — the lower clamp arm. The 0%
      // utilization then propagates to getSegments (no segment) and to the
      // tooltip else arm.
      const p = buildStackedMemoryBarPresentation(makeProps({ total: 0, percentOnly: -5 }), 400);
      expect(p.displayPercentValue).toBe(0);
      expect(p.segments).toEqual([]);
    });

    it('returns 0 when total <= 0 and percentOnly is NaN (not finite)', () => {
      // Number.isFinite(NaN) === false -> final `return 0` arm.
      const p = buildStackedMemoryBarPresentation(
        makeProps({ total: 0, percentOnly: Number.NaN }),
        400,
      );
      expect(p.displayPercentValue).toBe(0);
    });

    it('returns 0 when total <= 0 and percentOnly is absent', () => {
      // percentOnly undefined -> Number.isFinite(undefined) === false -> 0.
      const p = buildStackedMemoryBarPresentation(makeProps({ total: 0 }), 400);
      expect(p.displayPercentValue).toBe(0);
      expect(p.displayLabel).toBe('0%');
    });
  });

  describe('getSegments — total <= 0 path', () => {
    it('emits a single Utilization segment when total <= 0 and utilization > 0', () => {
      // Branch 12: true arm of `if (props.total <= 0)`; utilizationPercent 70
      // exceeds the inner `<= 0` guard -> single Utilization segment colored
      // via the default memory thresholds (70 < 75 -> normal).
      const p = buildStackedMemoryBarPresentation(makeProps({ total: 0, percentOnly: 70 }), 400);
      expect(p.segments).toHaveLength(1);
      expect(p.segments[0]).toStrictEqual({
        color: NORMAL_RGBA,
        label: 'Utilization',
        leftPercent: 0,
        widthPercent: 70,
      });
    });

    it('emits no segments when total <= 0 and utilization <= 0', () => {
      // Branch 12 entered; inner `if (utilizationPercent <= 0)` true -> [].
      const p = buildStackedMemoryBarPresentation(makeProps({ total: 0 }), 400);
      expect(p.segments).toEqual([]);
    });
  });

  describe('getTooltipRows — else arm (unavailable=false AND total <= 0)', () => {
    it('renders a single Utilization tooltip row carrying the percent-only label', () => {
      // Branch 32: the final else arm (unavailable false, total <= 0) pushes a
      // Utilization row whose value is the formatted displayLabel, not a byte
      // reading — distinct from both the unavailable and total > 0 arms.
      const p = buildStackedMemoryBarPresentation(makeProps({ total: 0, percentOnly: 70 }), 400);
      expect(p.tooltipRows).toEqual([
        {
          borderTop: true,
          label: 'Utilization',
          labelClass: 'text-blue-300',
          value: '70%',
        },
      ]);
    });
  });

  describe('swap integration (showSwapBar, swapBarPercent, Swap tooltip row)', () => {
    it('enables the swap bar, reports the swap ratio, and adds a Swap tooltip row', () => {
      // Branches 41 (showSwapBar && true), 42 (swapTotal && >0 true),
      // 43 (ternary ? arm -> Math.min computation):
      //   swapBarPercent = Math.min((2 GiB / 8 GiB) * 100, 100) === 25.
      // Branch 33: tooltip Swap row pushed because total > 0 && hasSwap.
      const p = buildStackedMemoryBarPresentation(
        makeProps({ used: 4 * GiB, total: 16 * GiB, swapUsed: 2 * GiB, swapTotal: 8 * GiB }),
        400,
      );
      expect(p.showSwapBar).toBe(true);
      expect(p.swapBarPercent).toBe(25);
      const swap = p.tooltipRows.find((row) => row.label === 'Swap');
      expect(swap).toBeDefined();
      expect(swap?.borderTop).toBe(true);
      expect(swap?.labelClass).toBe('text-amber-400');
      expect(swap?.value).toBe('2.00 GB / 8.00 GB');
    });

    it('clamps swapBarPercent to 100 when swapUsed exceeds swapTotal', () => {
      // Branch 43: Math.min((16 GiB / 8 GiB) * 100, 100) === Math.min(200, 100).
      const p = buildStackedMemoryBarPresentation(
        makeProps({ used: 4 * GiB, total: 16 * GiB, swapUsed: 16 * GiB, swapTotal: 8 * GiB }),
        400,
      );
      expect(p.swapBarPercent).toBe(100);
      expect(p.showSwapBar).toBe(true);
    });

    it('defaults swapUsed to 0 when absent (Swap row still shown, swap bar suppressed)', () => {
      // Drives the `|| 0` fallback arms for swapUsed at line 246 (tooltip
      // value), 278 (showSwapBar), and 281 (swapBarPercent). swapTotal > 0
      // keeps the Swap tooltip row visible (hasSwap true); swapUsed absent ->
      // showSwapBar false, swapBarPercent 0, tooltip reads '0 B / 8.00 GB'.
      const p = buildStackedMemoryBarPresentation(
        makeProps({ used: 4 * GiB, total: 16 * GiB, swapTotal: 8 * GiB }),
        400,
      );
      expect(p.showSwapBar).toBe(false);
      expect(p.swapBarPercent).toBe(0);
      const swap = p.tooltipRows.find((row) => row.label === 'Swap');
      expect(swap).toBeDefined();
      expect(swap?.value).toBe('0 B / 8.00 GB');
    });
  });

  describe('anomaly threading (anomalyClass, anomalyDescription, anomalyRatio)', () => {
    it('maps a known severity to its anomaly class and surfaces the description', () => {
      // Branches 39 (props.anomaly truthy -> ANOMALY_SEVERITY_CLASS[severity])
      // and 40 (props.anomaly?.description non-null/undefined access).
      // severity 'high' -> 'text-orange-400'; ratio 30/10 = 3.0 -> '3.0x'.
      const p = buildStackedMemoryBarPresentation(
        makeProps({ total: 16 * GiB, anomaly: makeAnomaly() }),
        400,
      );
      expect(p.anomalyClass).toBe('text-orange-400');
      expect(p.anomalyDescription).toBe('Memory spike');
      expect(p.anomalyRatio).toBe('3.0x');
    });

    it('falls back to the default anomaly class for an unknown severity', () => {
      // Branch 39 true arm + the ?? 'text-yellow-400' fallback:
      // ANOMALY_SEVERITY_CLASS has no 'weird' key -> undefined ?? fallback.
      const p = buildStackedMemoryBarPresentation(
        makeProps({ total: 16 * GiB, anomaly: makeAnomaly({ severity: 'weird' }) }),
        400,
      );
      expect(p.anomalyClass).toBe('text-yellow-400');
      // description is still threaded through the optional-chain true arm.
      expect(p.anomalyDescription).toBe('Memory spike');
    });
  });
});
