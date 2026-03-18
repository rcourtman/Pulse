import { describe, expect, it } from 'vitest';
import {
  statusBadgeClass,
  formatPercent,
  formatDelta,
  deltaColorClass,
  PRIORITY_ORDER,
  priorityBadgeClass,
  MAX_ACTION_ITEMS,
} from '@/pages/DashboardPanels/dashboardHelpers';

describe('dashboardHelpers', () => {
  describe('statusBadgeClass', () => {
    it('returns correct class for online', () => {
      const result = statusBadgeClass('online');
      expect(result).toContain('bg-emerald-100');
      expect(result).toContain('text-emerald-700');
    });

    it('returns correct class for offline', () => {
      const result = statusBadgeClass('offline');
      expect(result).toContain('bg-surface-alt');
    });

    it('returns correct class for warning', () => {
      const result = statusBadgeClass('warning');
      expect(result).toContain('bg-amber-100');
      expect(result).toContain('text-amber-700');
    });

    it('returns correct class for critical', () => {
      const result = statusBadgeClass('critical');
      expect(result).toContain('bg-red-100');
      expect(result).toContain('text-red-700');
    });

    it('returns correct class for unknown', () => {
      const result = statusBadgeClass('unknown');
      expect(result).toContain('bg-surface-alt');
      expect(result).toContain('text-muted');
    });
  });

  describe('formatPercent', () => {
    it('formats positive percentage', () => {
      expect(formatPercent(50)).toBe('50%');
    });

    it('formats zero', () => {
      expect(formatPercent(0)).toBe('0%');
    });

    it('rounds to nearest integer', () => {
      expect(formatPercent(49.6)).toBe('50%');
      expect(formatPercent(49.4)).toBe('49%');
    });
  });

  describe('formatDelta', () => {
    it('returns null for null input', () => {
      expect(formatDelta(null)).toBeNull();
    });

    it('formats positive delta with plus sign', () => {
      expect(formatDelta(5)).toBe('+5.0%');
      expect(formatDelta(0.5)).toBe('+0.5%');
    });

    it('formats negative delta with minus sign', () => {
      expect(formatDelta(-5)).toBe('-5.0%');
      expect(formatDelta(-0.5)).toBe('-0.5%');
    });

    it('formats zero as +0.0%', () => {
      expect(formatDelta(0)).toBe('+0.0%');
    });
  });

  describe('deltaColorClass', () => {
    it('returns muted for null', () => {
      expect(deltaColorClass(null)).toBe('text-muted');
    });

    it('returns red for delta > 5', () => {
      expect(deltaColorClass(10)).toContain('red-500');
    });

    it('returns amber for delta > 0 and <= 5', () => {
      expect(deltaColorClass(3)).toContain('amber-500');
    });

    it('returns emerald for delta < -5', () => {
      expect(deltaColorClass(-10)).toContain('emerald-500');
    });

    it('returns blue for delta < 0 and >= -5', () => {
      expect(deltaColorClass(-3)).toContain('blue-500');
    });

    it('returns muted for delta = 0', () => {
      expect(deltaColorClass(0)).toBe('text-muted');
    });
  });

  describe('PRIORITY_ORDER', () => {
    it('has correct order for critical', () => {
      expect(PRIORITY_ORDER.critical).toBe(0);
    });

    it('has correct order for high', () => {
      expect(PRIORITY_ORDER.high).toBe(1);
    });

    it('has correct order for medium', () => {
      expect(PRIORITY_ORDER.medium).toBe(2);
    });

    it('has correct order for low', () => {
      expect(PRIORITY_ORDER.low).toBe(3);
    });
  });

  describe('priorityBadgeClass', () => {
    it('returns correct class for critical', () => {
      const result = priorityBadgeClass('critical');
      expect(result).toContain('bg-red-100');
    });

    it('returns correct class for high', () => {
      const result = priorityBadgeClass('high');
      expect(result).toContain('bg-orange-100');
    });

    it('returns correct class for medium', () => {
      const result = priorityBadgeClass('medium');
      expect(result).toContain('bg-amber-100');
    });

    it('returns correct class for low', () => {
      const result = priorityBadgeClass('low');
      expect(result).toContain('bg-blue-100');
    });
  });

  describe('MAX_ACTION_ITEMS', () => {
    it('is 5', () => {
      expect(MAX_ACTION_ITEMS).toBe(5);
    });
  });
});
