import { describe, expect, it } from 'vitest';

import { estimateTextWidth } from '@/utils/format';

import type { MetricBarProps } from '../metricBarModel';
import { buildMetricBarPresentation } from '../metricBarModel';

// estimateTextWidth(text) = text.length * 5.5 + 8 (mirrored from @/utils/format).
// For label='CPU' + sublabel='8c' the composed threshold text is 'CPU (8c)'
// (length 8) -> estimateTextWidth = 8 * 5.5 + 8 = 52. So a containerWidth of 52
// trips the >= true arm of the showSublabel check, and 51 trips the false arm.
const LABEL = 'CPU';
const SUBLABEL = '8c';
const SUBLABEL_THRESHOLD = estimateTextWidth(`${LABEL} (${SUBLABEL})`); // 52

// Tailwind background class mirrored from metricThresholds.BG_CLASSES.normal.
// Default cpu thresholds are warning 80 / critical 90 (METRIC_THRESHOLDS.cpu),
// used whenever thresholds is undefined or null. value 42 < 80 -> normal.
const NORMAL_CLASS = 'bg-metric-normal-bg dark:bg-metric-normal-bg';

const makeProps = (overrides: Partial<MetricBarProps> = {}): MetricBarProps => ({
  value: 42,
  label: LABEL,
  sublabel: SUBLABEL,
  ...overrides,
});

describe('buildMetricBarPresentation (branch coverage 0720pm)', () => {
  describe('showLabel branch (props.showLabel !== false && label.trim().length > 0)', () => {
    it('returns showLabel=true when showLabel is omitted (true arm of !== false)', () => {
      // showLabel omitted -> props.showLabel is undefined -> !== false is true.
      // label 'CPU' trims to a non-empty string -> showLabel true.
      const p = buildMetricBarPresentation(makeProps(), 200);
      expect(p.showLabel).toBe(true);
      // The other emitted fields are also concrete and verified here.
      expect(p.width).toBe(42);
      expect(p.progressColorClass).toBe(NORMAL_CLASS);
      // With a wide container the sublabel is also shown.
      expect(p.showSublabel).toBe(true);
    });

    it('returns showLabel=true when showLabel is explicitly true (true arm of !== false)', () => {
      const p = buildMetricBarPresentation(makeProps({ showLabel: true }), 200);
      expect(p.showLabel).toBe(true);
    });

    it('returns showLabel=false when showLabel is explicitly false (false arm of !== false)', () => {
      // Even with a wide container and a non-empty sublabel, showLabel=false
      // propagates straight through to showLabel and forces showSublabel=false
      // via the leading && of the showSublabel expression.
      const p = buildMetricBarPresentation(makeProps({ showLabel: false }), 200);
      expect(p.showLabel).toBe(false);
      expect(p.showSublabel).toBe(false);
      // width and progressColorClass are computed independently of showLabel.
      expect(p.width).toBe(42);
      expect(p.progressColorClass).toBe(NORMAL_CLASS);
    });

    it('returns showLabel=false when the label is whitespace-only (label.trim().length > 0 false arm)', () => {
      // showLabel !== false is true, but label.trim().length === 0 -> the
      // right operand of the && is false -> showLabel false -> showSublabel false.
      const p = buildMetricBarPresentation(makeProps({ label: '   ', showLabel: true }), 200);
      expect(p.showLabel).toBe(false);
      expect(p.showSublabel).toBe(false);
    });
  });

  describe('showSublabel threshold branch (containerWidth >= estimateTextWidth(...))', () => {
    it('returns showSublabel=true when containerWidth meets the threshold exactly (>= true arm)', () => {
      // estimateTextWidth('CPU (8c)') === 52; containerWidth 52 -> 52 >= 52 true.
      expect(SUBLABEL_THRESHOLD).toBe(52);
      const p = buildMetricBarPresentation(makeProps(), SUBLABEL_THRESHOLD);
      expect(p.showLabel).toBe(true);
      expect(p.showSublabel).toBe(true);
    });

    it('returns showSublabel=false when containerWidth is one below the threshold (false arm)', () => {
      // containerWidth 51 < 52 -> the >= arm is false -> showSublabel false.
      // showLabel itself is unaffected by the width check (still true here).
      const p = buildMetricBarPresentation(makeProps(), SUBLABEL_THRESHOLD - 1);
      expect(p.showLabel).toBe(true);
      expect(p.showSublabel).toBe(false);
    });

    it('returns showSublabel=false when sublabel is empty (Boolean(props.sublabel) false arm)', () => {
      // Even with a very wide container, Boolean('') is false -> showSublabel false.
      const p = buildMetricBarPresentation(makeProps({ sublabel: '' }), 1000);
      expect(p.showLabel).toBe(true);
      expect(p.showSublabel).toBe(false);
    });
  });

  describe('width field (Math.min(props.value, 100))', () => {
    it('returns the raw value when value <= 100', () => {
      const p = buildMetricBarPresentation(makeProps({ value: 42 }), 200);
      expect(p.width).toBe(42);
    });

    it('clamps to 100 when value > 100', () => {
      // Math.min(150, 100) -> 100.
      const p = buildMetricBarPresentation(makeProps({ value: 150 }), 200);
      expect(p.width).toBe(100);
    });
  });

  describe('progressColorClass threading (type/thresholds)', () => {
    it('maps a sub-warning value to the normal class with default cpu thresholds', () => {
      // value 42 < 80 (warning) -> normal; metric defaults to 'cpu' when type
      // is omitted.
      const p = buildMetricBarPresentation(makeProps({ value: 42 }), 200);
      expect(p.progressColorClass).toBe(NORMAL_CLASS);
    });

    it('honors type=generic by mapping it to cpu for color purposes', () => {
      // metric = props.type || 'cpu' -> 'generic'; metricType = metric ===
      // 'generic' ? 'cpu' : metric -> 'cpu'. Same normal-band class.
      const p = buildMetricBarPresentation(makeProps({ value: 42, type: 'generic' }), 200);
      expect(p.progressColorClass).toBe(NORMAL_CLASS);
    });
  });
});
