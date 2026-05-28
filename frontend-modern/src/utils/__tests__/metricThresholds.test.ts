import { describe, expect, it } from 'vitest';
import {
  getMetricSeverity,
  getMetricColorClass,
  getMetricColorRgba,
  getMetricColorHex,
  getMetricTextColorClass,
  getDefaultMetricDisplayThresholds,
  resolveMetricDisplayThresholds,
  METRIC_THRESHOLDS,
  type MetricType,
} from '@/utils/metricThresholds';
import {
  FACTORY_KUBERNETES_DEFAULTS,
  FACTORY_TRUENAS_DEFAULTS,
  FACTORY_TRUENAS_DISK_DEFAULTS,
  FACTORY_VMWARE_DEFAULTS,
} from '@/utils/alertThresholdDefaults';
import type { AlertConfig } from '@/types/alerts';

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

  describe('alert-backed display thresholds', () => {
    it('derives guest display thresholds from alert trigger and clear defaults', () => {
      expect(getDefaultMetricDisplayThresholds('cpu')).toEqual({ warning: 75, critical: 80 });
      expect(getDefaultMetricDisplayThresholds('memory')).toEqual({ warning: 80, critical: 85 });
      expect(getDefaultMetricDisplayThresholds('disk')).toEqual({ warning: 85, critical: 90 });
      expect(getDefaultMetricDisplayThresholds('generic')).toEqual({
        warning: 75,
        critical: 90,
      });
    });

    it('resolves configured defaults and resource override candidates', () => {
      const config = {
        enabled: true,
        guestDefaults: {
          cpu: { trigger: 82, clear: 77 },
        },
        nodeDefaults: {},
        storageDefault: { trigger: 85, clear: 80 },
        overrides: {
          'guest:cluster-a:100': {
            cpu: { trigger: 95, clear: 90 },
          },
        },
      } as AlertConfig;

      expect(resolveMetricDisplayThresholds(config, 'guest', 'cpu', 'missing')).toEqual({
        warning: 77,
        critical: 82,
      });
      expect(
        resolveMetricDisplayThresholds(config, 'guest', 'cpu', [
          'cluster-a:node-a:100',
          'guest:cluster-a:100',
        ]),
      ).toEqual({
        warning: 90,
        critical: 95,
      });
    });

    it('uses default display coloring when alert thresholds are disabled', () => {
      const config = {
        enabled: true,
        guestDefaults: {},
        nodeDefaults: {},
        storageDefault: { trigger: 85, clear: 80 },
        overrides: {
          'guest:cluster-a:100': {
            disk: { trigger: -1, clear: 0 },
          },
        },
      } as AlertConfig;

      expect(
        resolveMetricDisplayThresholds(config, 'guest', 'disk', 'guest:cluster-a:100'),
      ).toBeNull();
      expect(getMetricColorClass(99, 'disk', null)).toContain('bg-metric-critical-bg');
    });

    it('honors disabled docker defaults and storage usage aliases', () => {
      const config = {
        enabled: true,
        guestDefaults: {},
        nodeDefaults: {},
        dockerDefaults: {
          cpu: { trigger: 0, clear: 0 },
          memory: { trigger: 0, clear: 0 },
          disk: { trigger: 0, clear: 0 },
        },
        storageDefault: { trigger: 92, clear: 86 },
        overrides: {},
      } as AlertConfig;

      expect(resolveMetricDisplayThresholds(config, 'docker', 'cpu')).toBeNull();
      expect(resolveMetricDisplayThresholds(config, 'storage', 'usage')).toEqual({
        warning: 86,
        critical: 92,
      });
    });

    it('keeps Kubernetes, TrueNAS, and vSphere factory defaults explicit for alert configuration', () => {
      expect(FACTORY_KUBERNETES_DEFAULTS).toEqual({
        cpu: 80,
        memory: 85,
        disk: 90,
        diskRead: -1,
        diskWrite: -1,
        networkIn: -1,
        networkOut: -1,
      });
      expect(FACTORY_TRUENAS_DEFAULTS).toMatchObject({
        cpu: 80,
        memory: 85,
        disk: 85,
        usage: 85,
        temperature: 80,
      });
      expect(FACTORY_TRUENAS_DISK_DEFAULTS).toEqual({ temperature: 55 });
      expect(FACTORY_VMWARE_DEFAULTS).toEqual({
        cpu: 80,
        memory: 85,
        disk: 90,
        usage: 85,
        diskRead: -1,
        diskWrite: -1,
        networkIn: -1,
        networkOut: -1,
      });
    });
  });
});
