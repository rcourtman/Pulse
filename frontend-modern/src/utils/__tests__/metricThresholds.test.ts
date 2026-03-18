import { describe, expect, it } from 'vitest';
import {
  getMetricSeverity,
  getMetricColorClass,
  getMetricColorRgba,
  getMetricColorHex,
  getMetricTextColorClass,
  METRIC_THRESHOLDS,
  type MetricType,
} from '@/utils/metricThresholds';

describe('metricThresholds', () => {
  describe('getMetricSeverity', () => {
    describe('cpu', () => {
      const metric: MetricType = 'cpu';

      it('returns normal for value below warning threshold', () => {
        expect(getMetricSeverity(50, metric)).toBe('normal');
        expect(getMetricSeverity(0, metric)).toBe('normal');
        expect(getMetricSeverity(79, metric)).toBe('normal');
      });

      it('returns warning for value at or above warning threshold', () => {
        expect(getMetricSeverity(80, metric)).toBe('warning');
        expect(getMetricSeverity(85, metric)).toBe('warning');
        expect(getMetricSeverity(89, metric)).toBe('warning');
      });

      it('returns critical for value at or above critical threshold', () => {
        expect(getMetricSeverity(90, metric)).toBe('critical');
        expect(getMetricSeverity(95, metric)).toBe('critical');
        expect(getMetricSeverity(100, metric)).toBe('critical');
      });
    });

    describe('memory', () => {
      const metric: MetricType = 'memory';

      it('returns normal for value below warning threshold', () => {
        expect(getMetricSeverity(50, metric)).toBe('normal');
        expect(getMetricSeverity(74, metric)).toBe('normal');
      });

      it('returns warning for value at or above warning threshold', () => {
        expect(getMetricSeverity(75, metric)).toBe('warning');
        expect(getMetricSeverity(80, metric)).toBe('warning');
      });

      it('returns critical for value at or above critical threshold', () => {
        expect(getMetricSeverity(85, metric)).toBe('critical');
        expect(getMetricSeverity(100, metric)).toBe('critical');
      });
    });

    describe('disk', () => {
      const metric: MetricType = 'disk';

      it('returns normal for value below warning threshold', () => {
        expect(getMetricSeverity(50, metric)).toBe('normal');
        expect(getMetricSeverity(79, metric)).toBe('normal');
      });

      it('returns warning for value at or above warning threshold', () => {
        expect(getMetricSeverity(80, metric)).toBe('warning');
        expect(getMetricSeverity(89, metric)).toBe('warning');
      });

      it('returns critical for value at or above critical threshold', () => {
        expect(getMetricSeverity(90, metric)).toBe('critical');
        expect(getMetricSeverity(100, metric)).toBe('critical');
      });
    });
  });

  describe('getMetricColorClass', () => {
    it('returns correct class for normal severity', () => {
      const result = getMetricColorClass(50, 'cpu');
      expect(result).toContain('bg-metric-normal-bg');
    });

    it('returns correct class for warning severity', () => {
      const result = getMetricColorClass(85, 'cpu');
      expect(result).toContain('bg-metric-warning-bg');
    });

    it('returns correct class for critical severity', () => {
      const result = getMetricColorClass(95, 'cpu');
      expect(result).toContain('bg-metric-critical-bg');
    });
  });

  describe('getMetricColorRgba', () => {
    it('returns green for normal severity', () => {
      const result = getMetricColorRgba(50, 'cpu');
      expect(result).toBe('rgba(34, 197, 94, 0.6)');
    });

    it('returns yellow for warning severity', () => {
      const result = getMetricColorRgba(85, 'cpu');
      expect(result).toBe('rgba(234, 179, 8, 0.6)');
    });

    it('returns red for critical severity', () => {
      const result = getMetricColorRgba(95, 'cpu');
      expect(result).toBe('rgba(239, 68, 68, 0.6)');
    });
  });

  describe('getMetricColorHex', () => {
    it('returns green for normal severity', () => {
      const result = getMetricColorHex(50, 'cpu');
      expect(result).toBe('#22c55e');
    });

    it('returns yellow for warning severity', () => {
      const result = getMetricColorHex(85, 'cpu');
      expect(result).toBe('#eab308');
    });

    it('returns red for critical severity', () => {
      const result = getMetricColorHex(95, 'cpu');
      expect(result).toBe('#ef4444');
    });
  });

  describe('getMetricTextColorClass', () => {
    it('returns muted for normal severity', () => {
      const result = getMetricTextColorClass(50, 'cpu');
      expect(result).toContain('text-muted');
    });

    it('returns yellow for warning severity', () => {
      const result = getMetricTextColorClass(85, 'cpu');
      expect(result).toContain('text-yellow-600');
    });

    it('returns red for critical severity', () => {
      const result = getMetricTextColorClass(95, 'cpu');
      expect(result).toContain('text-red-600');
    });
  });

  describe('METRIC_THRESHOLDS', () => {
    it('has correct values for cpu', () => {
      expect(METRIC_THRESHOLDS.cpu).toEqual({ warning: 80, critical: 90 });
    });

    it('has correct values for memory', () => {
      expect(METRIC_THRESHOLDS.memory).toEqual({ warning: 75, critical: 85 });
    });

    it('has correct values for disk', () => {
      expect(METRIC_THRESHOLDS.disk).toEqual({ warning: 80, critical: 90 });
    });
  });
});
