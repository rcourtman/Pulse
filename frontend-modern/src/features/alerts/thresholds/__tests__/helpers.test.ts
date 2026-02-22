import { describe, expect, it } from 'vitest';
import {
  normalizeThresholdLabel,
  normalizeDockerIgnoredInput,
  formatMetricValue,
} from '@/features/alerts/thresholds/helpers';

describe('alerts thresholds helpers', () => {
  describe('normalizeThresholdLabel', () => {
    it('trims whitespace', () => {
      expect(normalizeThresholdLabel('  cpu  ')).toBe('cpu');
    });

    it('converts to lowercase', () => {
      expect(normalizeThresholdLabel('CPU')).toBe('cpu');
    });

    it('removes percentage symbol', () => {
      expect(normalizeThresholdLabel('cpu %')).toBe('cpu');
    });

    it('removes celsius symbol', () => {
      expect(normalizeThresholdLabel('temperature °c')).toBe('temperature');
    });

    it('removes mb/s symbol', () => {
      expect(normalizeThresholdLabel('diskread mb/s')).toBe('diskread');
    });

    it('normalizes disk r to diskRead', () => {
      expect(normalizeThresholdLabel('disk r')).toBe('diskRead');
    });

    it('normalizes disk w to diskWrite', () => {
      expect(normalizeThresholdLabel('disk w')).toBe('diskWrite');
    });

    it('normalizes net in to networkIn', () => {
      expect(normalizeThresholdLabel('net in')).toBe('networkIn');
    });

    it('normalizes net out to networkOut', () => {
      expect(normalizeThresholdLabel('net out')).toBe('networkOut');
    });

    it('normalizes disk temp to diskTemperature', () => {
      expect(normalizeThresholdLabel('disk temp')).toBe('diskTemperature');
    });
  });

  describe('normalizeDockerIgnoredInput', () => {
    it('returns empty array for empty string', () => {
      expect(normalizeDockerIgnoredInput('')).toEqual([]);
    });

    it('splits by newlines', () => {
      expect(normalizeDockerIgnoredInput('line1\nline2')).toEqual(['line1', 'line2']);
    });

    it('trims whitespace from each line', () => {
      expect(normalizeDockerIgnoredInput('  line1  \n  line2  ')).toEqual(['line1', 'line2']);
    });

    it('filters empty lines', () => {
      expect(normalizeDockerIgnoredInput('line1\n\nline2\n')).toEqual(['line1', 'line2']);
    });

    it('handles single line', () => {
      expect(normalizeDockerIgnoredInput('single')).toEqual(['single']);
    });
  });

  describe('formatMetricValue', () => {
    it('returns 0 for undefined', () => {
      expect(formatMetricValue('cpu', undefined)).toBe('0');
    });

    it('returns 0 for null', () => {
      expect(formatMetricValue('cpu', null as unknown as number)).toBe('0');
    });

    it('returns Off for zero or negative values', () => {
      expect(formatMetricValue('cpu', 0)).toBe('Off');
      expect(formatMetricValue('cpu', -1)).toBe('Off');
    });

    it('formats cpu as percentage', () => {
      expect(formatMetricValue('cpu', 80)).toBe('80%');
    });

    it('formats memory as percentage', () => {
      expect(formatMetricValue('memory', 50)).toBe('50%');
    });

    it('formats disk as percentage', () => {
      expect(formatMetricValue('disk', 75)).toBe('75%');
    });

    it('formats memoryWarnPct as percentage', () => {
      expect(formatMetricValue('memoryWarnPct', 60)).toBe('60%');
    });

    it('formats restartWindow with seconds', () => {
      expect(formatMetricValue('restartWindow', 300)).toBe('300s');
    });

    it('formats restartCount as string', () => {
      expect(formatMetricValue('restartCount', 3)).toBe('3');
    });

    it('formats size in GiB with one decimal', () => {
      expect(formatMetricValue('warningSizeGiB', 10.55)).toBe('10.6 GiB');
    });

    it('formats diskRead in MB/s', () => {
      expect(formatMetricValue('diskRead', 100)).toBe('100 MB/s');
    });

    it('formats diskWrite in MB/s', () => {
      expect(formatMetricValue('diskWrite', 50)).toBe('50 MB/s');
    });

    it('formats networkIn in MB/s', () => {
      expect(formatMetricValue('networkIn', 25)).toBe('25 MB/s');
    });

    it('formats networkOut in MB/s', () => {
      expect(formatMetricValue('networkOut', 75)).toBe('75 MB/s');
    });

    it('returns string for unknown metric', () => {
      expect(formatMetricValue('unknown', 123)).toBe('123');
    });
  });
});
