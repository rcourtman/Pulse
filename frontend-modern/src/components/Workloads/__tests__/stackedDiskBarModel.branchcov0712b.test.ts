import { describe, expect, it } from 'vitest';

import type { AnomalyReport } from '@/types/aiIntelligence';
import type { Disk } from '@/types/api';

import { buildStackedDiskBarPresentation } from '../stackedDiskBarModel';

// The four small helpers under test (getDiskUsagePercent, getShortDiskLabel,
// getInlineDiskText, getStackedDiskColor) are module-private, so every branch
// is exercised through the exported buildStackedDiskBarPresentation and
// asserted on concrete fields (miniDisks[].percent / shortLabel / inlineText,
// segments[].color, verticalBars[].fillPercent, ...).

// Palette mirrored from the module under test (not exported).
const SEGMENT_COLORS = [
  'rgba(34, 197, 94, 0.6)',
  'rgba(59, 130, 246, 0.6)',
  'rgba(168, 85, 247, 0.6)',
  'rgba(249, 115, 22, 0.6)',
  'rgba(236, 72, 153, 0.6)',
  'rgba(20, 184, 166, 0.6)',
];
const NORMAL = 'rgba(34, 197, 94, 0.6)';
const WARNING = 'rgba(234, 179, 8, 0.6)';
const CRITICAL = 'rgba(239, 68, 68, 0.6)';

const makeDisk = (overrides: Partial<Disk> = {}): Disk => ({
  total: 100,
  used: 0,
  free: 100,
  usage: 0,
  mountpoint: '/',
  ...overrides,
});

const makeAnomaly = (overrides: Partial<AnomalyReport> = {}): AnomalyReport => ({
  resource_id: 'r1',
  resource_name: 'resource-1',
  resource_type: 'guest',
  metric: 'disk',
  current_value: 25,
  baseline_mean: 10,
  baseline_std_dev: 1,
  z_score: 15,
  severity: 'high',
  description: 'Disk usage spike',
  ...overrides,
});

describe('stackedDiskBarModel (branch coverage 2)', () => {
  describe('getDiskUsagePercent (via miniDisks[].percent)', () => {
    it('uses the used/total ratio when total > 0', () => {
      // Branch: disk.total > 0 -> (used / total) * 100.
      const p = buildStackedDiskBarPresentation(
        { disks: [makeDisk({ total: 200, used: 50, free: 150, usage: 25 })] },
        400,
      );
      expect(p.miniDisks[0].percent).toBe(25);
    });

    it('scales a fractional usage (<=1) by 100 when total <= 0', () => {
      // Branch: total <= 0, Number.isFinite(usage), usage <= 1 -> usage * 100.
      const p = buildStackedDiskBarPresentation(
        { disks: [makeDisk({ total: 0, used: 0, free: 0, usage: 0.5 })] },
        400,
      );
      expect(p.miniDisks[0].percent).toBe(50);
    });

    it('returns a usage value > 1 unchanged when total <= 0', () => {
      // Branch: total <= 0, usage finite, usage > 1 -> usage.
      const p = buildStackedDiskBarPresentation(
        { disks: [makeDisk({ total: 0, used: 0, free: 0, usage: 75 })] },
        400,
      );
      expect(p.miniDisks[0].percent).toBe(75);
    });

    it('returns 0 when total <= 0 and usage is not finite', () => {
      // Branch: total <= 0, !Number.isFinite(usage) -> 0.
      const p = buildStackedDiskBarPresentation(
        { disks: [makeDisk({ total: 0, used: 0, free: 0, usage: Number.NaN })] },
        400,
      );
      expect(p.miniDisks[0].percent).toBe(0);
    });
  });

  describe('getShortDiskLabel (via miniDisks[].shortLabel)', () => {
    it('strips /dev/, returns root, takes the last path segment, and returns short/long labels unchanged', () => {
      // Drives all five return arms in array order:
      //   '/dev/sda1' -> slice '/dev/' -> 'sda1'
      //   '/'         -> trimmed === '/' -> '/'
      //   '/var/log'  -> absolute path -> last segment 'log'
      //   'data'      -> length <= 12 -> 'data'
      //   'verylongmount' -> fall-through -> 'verylongmount'
      const p = buildStackedDiskBarPresentation(
        {
          disks: [
            makeDisk({ mountpoint: '/dev/sda1' }),
            makeDisk({ mountpoint: '/' }),
            makeDisk({ mountpoint: '/var/log' }),
            makeDisk({ mountpoint: 'data' }),
            makeDisk({ mountpoint: 'verylongmount' }),
          ],
        },
        400,
      );
      expect(p.miniDisks.map((d) => d.shortLabel)).toEqual([
        'sda1',
        '/',
        'log',
        'data',
        'verylongmount',
      ]);
    });
  });

  describe('getInlineDiskText (via miniDisks[].inlineText)', () => {
    // estimateInlineTextWidth(text) = text.length * 5.4 + 4 (module-internal).
    it('returns the full text when slotWidth <= 0 (short-circuit)', () => {
      // slotWidth = containerWidth(0) / 1 = 0 -> first operand of the || .
      const p = buildStackedDiskBarPresentation(
        { disks: [makeDisk({ mountpoint: '/dev/sda1', total: 100, used: 50, free: 50, usage: 50 })] },
        0,
      );
      expect(p.miniDisks[0].inlineText).toBe('sda1 50%');
    });

    it('returns the full text when it fits within the slot', () => {
      // 'sda1 50%' width = 8 * 5.4 + 4 = 47.2 <= slotWidth(60).
      const p = buildStackedDiskBarPresentation(
        { disks: [makeDisk({ mountpoint: '/dev/sda1', total: 100, used: 50, free: 50, usage: 50 })] },
        60,
      );
      expect(p.miniDisks[0].inlineText).toBe('sda1 50%');
    });

    it('falls back to the percent label when only it fits', () => {
      // shortLabel 'a', percentLabel '100%'. full 'a 100%' width 36.4 > 30;
      // '100%' width 25.6 <= 30 -> returns '100%'.
      const p = buildStackedDiskBarPresentation(
        { disks: [makeDisk({ mountpoint: '/dev/a', total: 100, used: 100, free: 0, usage: 100 })] },
        30,
      );
      expect(p.miniDisks[0].inlineText).toBe('100%');
    });

    it('falls back to the short label when the percent label does not fit but the short label does', () => {
      // slotWidth 20: full 36.4 > 20; percent '100%' 25.6 > 20; short 'a' 9.4 <= 20 -> 'a'.
      const p = buildStackedDiskBarPresentation(
        { disks: [makeDisk({ mountpoint: '/dev/a', total: 100, used: 100, free: 0, usage: 100 })] },
        20,
      );
      expect(p.miniDisks[0].inlineText).toBe('a');
    });

    it('returns an empty string when neither label fits', () => {
      // slotWidth 5: every estimateInlineTextWidth > 5 -> ''.
      const p = buildStackedDiskBarPresentation(
        { disks: [makeDisk({ mountpoint: '/dev/a', total: 100, used: 100, free: 0, usage: 100 })] },
        5,
      );
      expect(p.miniDisks[0].inlineText).toBe('');
    });
  });

  describe('getStackedDiskColor (via segments[].color, mode=stacked)', () => {
    it('maps to warning/critical severity colors and the palette by percent with default thresholds', () => {
      // Default thresholds (undefined -> ?? 90 / ?? 80).
      //   85 -> warning arm; 10 -> palette arm (index 1); 95 -> critical arm.
      const p = buildStackedDiskBarPresentation(
        {
          mode: 'stacked',
          disks: [
            makeDisk({ mountpoint: '/a', total: 100, used: 85, free: 15, usage: 85 }),
            makeDisk({ mountpoint: '/b', total: 100, used: 10, free: 90, usage: 10 }),
            makeDisk({ mountpoint: '/c', total: 100, used: 95, free: 5, usage: 95 }),
          ],
        },
        400,
      );
      expect(p.segments.map((s) => s.color)).toEqual([WARNING, SEGMENT_COLORS[1], CRITICAL]);
      expect(p.segments.map((s) => s.diskUsagePercent)).toEqual([85, 10, 95]);
      expect(p.segments.map((s) => s.index)).toEqual([0, 1, 2]);
      // widthPercent = used / totalCapacity(300) * 100.
      expect(p.segments[0].widthPercent).toBeCloseTo(28.33, 1);
      expect(p.segments[2].widthPercent).toBeCloseTo(31.67, 1);
    });

    it('honors custom warning/critical thresholds', () => {
      // thresholds { warning: 60, critical: 70 } (no ?? defaults).
      //   65 -> warning (>=60, <70); 75 -> critical (>=70); 50 -> palette[2].
      const p = buildStackedDiskBarPresentation(
        {
          mode: 'stacked',
          thresholds: { warning: 60, critical: 70 },
          disks: [
            makeDisk({ mountpoint: '/a', total: 100, used: 65, free: 35, usage: 65 }),
            makeDisk({ mountpoint: '/b', total: 100, used: 75, free: 25, usage: 75 }),
            makeDisk({ mountpoint: '/c', total: 100, used: 50, free: 50, usage: 50 }),
          ],
        },
        400,
      );
      expect(p.segments.map((s) => s.color)).toEqual([WARNING, CRITICAL, SEGMENT_COLORS[2]]);
    });

    it('falls back to default thresholds when thresholds is null and wraps the palette index past the end', () => {
      // thresholds null -> thresholds?.critical ?? 90 (nullish arm). All 5% (<80)
      // -> palette arm; index 6 wraps to SEGMENT_COLORS[0].
      const p = buildStackedDiskBarPresentation(
        {
          mode: 'stacked',
          thresholds: null,
          disks: Array.from({ length: 7 }, () =>
            makeDisk({ total: 100, used: 5, free: 95, usage: 5 }),
          ),
        },
        400,
      );
      expect(p.segments.length).toBe(7);
      expect(p.segments[5].color).toBe(SEGMENT_COLORS[5]);
      expect(p.segments[6].color).toBe(SEGMENT_COLORS[0]);
    });
  });

  describe('buildStackedDiskBarPresentation', () => {
    it('renders an empty state when no disks and no aggregate disk are given', () => {
      // disks ?? [] arm (undefined); aggregateDisk absent; overallPercent final 0 arm.
      const p = buildStackedDiskBarPresentation({}, 400);
      expect(p.hasDisks).toBe(false);
      expect(p.hasMultipleDisks).toBe(false);
      expect(p.displayPercentValue).toBe(0);
      expect(p.barPercent).toBe(0);
      expect(p.displayLabel).toBe('0%');
      expect(p.segments).toEqual([]);
      expect(p.miniDisks).toEqual([]);
      expect(p.verticalBars).toEqual([]);
      expect(p.tooltipContent).toEqual([]);
      expect(p.tooltipTitle).toBe('Disk Usage');
      // No mode + no inline disk mode -> centered container class.
      expect(p.containerClass).toBe('metric-text w-full h-4 flex items-center justify-center');
      expect(p.anomalyClass).toBe('text-yellow-400');
      expect(p.anomalyRatio).toBe('');
    });

    it('renders a single disk without stacked segments', () => {
      // hasDisks true, hasMultipleDisks false; maxInfo from a single disk;
      // tooltip uses getStackedDiskColor palette (useUsageColors false).
      const p = buildStackedDiskBarPresentation(
        { disks: [makeDisk({ mountpoint: '/', total: 100, used: 25, free: 75, usage: 25 })] },
        400,
      );
      expect(p.hasDisks).toBe(true);
      expect(p.hasMultipleDisks).toBe(false);
      expect(p.displayPercentValue).toBe(25);
      expect(p.barPercent).toBe(25);
      expect(p.displayLabel).toBe('25%');
      expect(p.displaySublabel).toBe('25.0 B/100 B');
      expect(p.maxLabelShort).toBe('max 25%');
      expect(p.maxLabelFull).toBe('Highest usage: / 25%');
      expect(p.useStackedSegments).toBe(false);
      expect(p.segments).toEqual([]);
      expect(p.miniDisks[0].shortLabel).toBe('/');
      expect(p.miniDisks[0].percent).toBe(25);
      expect(p.miniDisks[0].title).toBe('/: 25% (25.0 B/100 B)');
      expect(p.tooltipTitle).toBe('Disk Usage');
      expect(p.tooltipContent[0]).toStrictEqual({
        color: SEGMENT_COLORS[0],
        label: '/',
        percent: '25%',
        total: '100 B',
        used: '25.0 B',
      });
    });

    it('enables stacked segments and the disk-count flag in stacked mode with multiple disks', () => {
      // useStackedSegments = hasMultipleDisks && explicitStackedMode; totalCapacity > 0
      // -> segments built; showDiskCount = useStackedSegments || showDiskCount.
      const p = buildStackedDiskBarPresentation(
        {
          mode: 'stacked',
          disks: [
            makeDisk({ mountpoint: '/a', total: 100, used: 10, free: 90, usage: 10 }),
            makeDisk({ mountpoint: '/b', total: 100, used: 20, free: 80, usage: 20 }),
          ],
        },
        400,
      );
      expect(p.useStackedSegments).toBe(true);
      expect(p.segments.length).toBe(2);
      expect(p.showDiskCount).toBe(true);
      expect(p.tooltipTitle).toBe('Disk Breakdown');
      // explicitStackedMode suppresses inlineDiskMode.
      expect(p.inlineDiskMode).toBe(false);
    });

    it('uses the total summary strategy in aggregate mode (overall percent, max label shown, usage-colored tooltip)', () => {
      // aggregateMode true; summaryStrategy defaults to 'total' -> useMaxSummary false;
      // displayPercentValue = overallPercent; barColor uses maxInfo.percent;
      // showMaxLabel true via the aggregate width check; tooltip useUsageColors true.
      const p = buildStackedDiskBarPresentation(
        {
          mode: 'aggregate',
          disks: [
            makeDisk({ mountpoint: '/a', total: 100, used: 80, free: 20, usage: 80 }),
            makeDisk({ mountpoint: '/b', total: 100, used: 20, free: 80, usage: 20 }),
          ],
        },
        400,
      );
      expect(p.aggregateMode).toBe(true);
      expect(p.displayPercentValue).toBe(50); // 100 / 200 * 100
      expect(p.barPercent).toBe(50);
      expect(p.maxLabelShort).toBe('max 80%');
      expect(p.barColor).toBe(WARNING); // getMetricColorRgba(maxInfo.percent = 80)
      expect(p.showMaxLabel).toBe(true);
      expect(p.showSublabel).toBe(true);
      expect(p.tooltipContent.map((t) => t.color)).toEqual([WARNING, NORMAL]);
    });

    it('uses the max summary strategy in aggregate mode (display = max percent, sublabel suppressed)', () => {
      // useMaxSummary = aggregateMode && hasMultipleDisks && 'max' && maxInfo -> true;
      // displayPercentValue = maxInfo.percent; maxLabelShort = 'max'; displaySublabel = ''.
      const p = buildStackedDiskBarPresentation(
        {
          mode: 'aggregate',
          summaryStrategy: 'max',
          disks: [
            makeDisk({ mountpoint: '/a', total: 100, used: 30, free: 70, usage: 30 }),
            makeDisk({ mountpoint: '/b', total: 100, used: 90, free: 10, usage: 90 }),
          ],
        },
        400,
      );
      expect(p.displayPercentValue).toBe(90);
      expect(p.barPercent).toBe(90);
      expect(p.maxLabelShort).toBe('max');
      expect(p.displaySublabel).toBe('');
      expect(p.showSublabel).toBe(false);
      expect(p.showMaxLabel).toBe(true);
      expect(p.barColor).toBe(CRITICAL); // getMetricColorRgba(90)
    });

    it('builds vertical bars clamped to 0..100 and the vertical-bars container class', () => {
      // verticalBarsMode = mode === 'vertical-bars' && hasDisks; fillPercent uses
      // Math.max(0, Math.min(percent, 100)) -> exercises both clamps.
      const p = buildStackedDiskBarPresentation(
        {
          mode: 'vertical-bars',
          disks: [
            makeDisk({ mountpoint: '/x', total: 100, used: 40, free: 60, usage: 40 }),
            makeDisk({ mountpoint: '/y', total: 100, used: 120, free: 0, usage: 120 }),
            makeDisk({ mountpoint: '/z', total: 100, used: -5, free: 105, usage: -5 }),
          ],
        },
        400,
      );
      expect(p.verticalBarsMode).toBe(true);
      expect(p.containerClass).toBe('metric-text w-full h-4 min-w-0');
      expect(p.verticalBars.map((b) => b.fillPercent)).toEqual([40, 100, 0]);
      expect(p.verticalBars[0]).toStrictEqual({
        color: NORMAL,
        fillPercent: 40,
        title: '/x: 40% (40.0 B/100 B)',
      });
      expect(p.verticalBars[1].color).toBe(CRITICAL); // 120% -> critical
    });

    it('enables inline disk mode and its container class in mini mode', () => {
      // miniMode true -> inlineDiskMode true (miniMode && !aggregate && !stacked
      // && !verticalBars); containerClass = inline-disk arm.
      const p = buildStackedDiskBarPresentation(
        {
          mode: 'mini',
          disks: [makeDisk({ mountpoint: '/dev/nvme0', total: 100, used: 75, free: 25, usage: 75 })],
        },
        400,
      );
      expect(p.miniMode).toBe(true);
      expect(p.inlineDiskMode).toBe(true);
      expect(p.containerClass).toBe('metric-text w-full h-4 min-w-0');
      expect(p.miniDisks[0].shortLabel).toBe('nvme0');
    });

    it('derives capacity and a Total tooltip from aggregateDisk when no disks array is given', () => {
      // hasDisks false -> totalCapacity/totalUsed from aggregateDisk;
      // tooltip aggregateDisk arm (total > 0) -> single Total item.
      const p = buildStackedDiskBarPresentation(
        { aggregateDisk: makeDisk({ total: 200, used: 50, free: 150, usage: 25 }) },
        400,
      );
      expect(p.hasDisks).toBe(false);
      expect(p.displayPercentValue).toBe(25);
      expect(p.displaySublabel).toBe('50.0 B/200 B');
      expect(p.tooltipContent).toStrictEqual([
        {
          color: NORMAL,
          label: 'Total',
          percent: '25%',
          total: '200 B',
          used: '50.0 B',
        },
      ]);
    });

    it('falls back to getDiskUsagePercent(aggregateDisk) for overall percent when aggregate total is 0', () => {
      // totalCapacity 0 -> overallPercent = aggregateDisk ? getDiskUsagePercent(...) : 0.
      // getDiskUsagePercent: total <= 0, usage 75 (>1) -> 75. Tooltip total>0 arm
      // skipped (total 0) -> empty.
      const p = buildStackedDiskBarPresentation(
        { aggregateDisk: makeDisk({ total: 0, used: 0, free: 0, usage: 75 }) },
        400,
      );
      expect(p.displayPercentValue).toBe(75);
      expect(p.barPercent).toBe(75);
      expect(p.displaySublabel).toBe('0 B/0 B');
      expect(p.tooltipContent).toEqual([]);
    });

    it('maps anomaly severity to a class and formats each anomaly-ratio tier', () => {
      // ratio >= 2 -> `${ratio}x`; 1.5..2 -> '↑↑'; 1..1.5 -> '↑'; baseline 0 -> ''.
      const high = buildStackedDiskBarPresentation(
        { anomaly: makeAnomaly({ severity: 'high', baseline_mean: 10, current_value: 25 }) },
        400,
      );
      expect(high.anomalyClass).toBe('text-orange-400');
      expect(high.anomalyRatio).toBe('2.5x');
      expect(high.anomalyDescription).toBe('Disk usage spike');

      const critical = buildStackedDiskBarPresentation(
        { anomaly: makeAnomaly({ severity: 'critical', baseline_mean: 10, current_value: 17 }) },
        400,
      );
      expect(critical.anomalyClass).toBe('text-red-400');
      expect(critical.anomalyRatio).toBe('↑↑');

      const lowTier = buildStackedDiskBarPresentation(
        { anomaly: makeAnomaly({ severity: 'low', baseline_mean: 10, current_value: 13 }) },
        400,
      );
      expect(lowTier.anomalyClass).toBe('text-blue-400');
      expect(lowTier.anomalyRatio).toBe('↑');

      // Unknown severity -> ANOMALY_SEVERITY_CLASS[sev] ?? 'text-yellow-400';
      // baseline_mean 0 -> formatAnomalyRatio null -> ?? ''.
      const unknown = buildStackedDiskBarPresentation(
        { anomaly: makeAnomaly({ severity: 'weird', baseline_mean: 0, current_value: 0 }) },
        400,
      );
      expect(unknown.anomalyClass).toBe('text-yellow-400');
      expect(unknown.anomalyRatio).toBe('');
    });

    it('suppresses the sublabel when containerWidth is too narrow to fit it', () => {
      // displaySublabel non-empty but containerWidth < estimateTextWidth(...) ->
      // showSublabel false (the width-failing arm of the && ).
      const p = buildStackedDiskBarPresentation(
        { disks: [makeDisk({ mountpoint: '/', total: 100, used: 25, free: 75, usage: 25 })] },
        5,
      );
      expect(p.showSublabel).toBe(false);
      expect(p.displaySublabel).toBe('25.0 B/100 B');
    });
  });
});
