import { describe, expect, it } from 'vitest';
import {
  formatTimestamp,
  investigationStatusLabels,
  investigationOutcomeLabels,
  investigationOutcomeColors,
  severityColors,
} from '@/api/patrol';

describe('patrol utilities', () => {
  describe('formatTimestamp', () => {
    it('returns empty string for empty input', () => {
      expect(formatTimestamp('')).toBe('');
    });

    it('returns empty string for undefined', () => {
      expect(formatTimestamp(undefined as unknown as string)).toBe('');
    });

    it('returns empty string for invalid date', () => {
      expect(formatTimestamp('invalid-date')).toBe('');
    });

    it('returns just now for very recent times', () => {
      const now = new Date().toISOString();
      expect(formatTimestamp(now)).toBe('just now');
    });

    it('returns minutes ago', () => {
      const fiveMinutesAgo = new Date(Date.now() - 5 * 60 * 1000).toISOString();
      expect(formatTimestamp(fiveMinutesAgo)).toBe('5m ago');
    });

    it('returns hours ago', () => {
      const twoHoursAgo = new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString();
      expect(formatTimestamp(twoHoursAgo)).toBe('2h ago');
    });

    it('returns days ago for less than 7 days', () => {
      const threeDaysAgo = new Date(Date.now() - 3 * 24 * 60 * 60 * 1000).toISOString();
      expect(formatTimestamp(threeDaysAgo)).toBe('3d ago');
    });

    it('returns locale date for 7+ days', () => {
      const tenDaysAgo = new Date(Date.now() - 10 * 24 * 60 * 60 * 1000).toISOString();
      const result = formatTimestamp(tenDaysAgo);
      expect(result).toMatch(/\d{1,2}\/\d{1,2}\/\d{2,4}/);
    });
  });

  describe('investigationStatusLabels', () => {
    it('has correct label for pending', () => {
      expect(investigationStatusLabels.pending).toBe('Pending');
    });

    it('has correct label for running', () => {
      expect(investigationStatusLabels.running).toBe('Investigating...');
    });

    it('has correct label for completed', () => {
      expect(investigationStatusLabels.completed).toBe('Completed');
    });

    it('has correct label for failed', () => {
      expect(investigationStatusLabels.failed).toBe('Failed');
    });

    it('has correct label for needs_attention', () => {
      expect(investigationStatusLabels.needs_attention).toBe('Needs Attention');
    });
  });

  describe('investigationOutcomeLabels', () => {
    it('has correct label for resolved', () => {
      expect(investigationOutcomeLabels.resolved).toBe('Resolved');
    });

    it('has correct label for fix_queued', () => {
      expect(investigationOutcomeLabels.fix_queued).toBe('Fix Queued');
    });

    it('has correct label for fix_executed', () => {
      expect(investigationOutcomeLabels.fix_executed).toBe('Fix Executed');
    });

    it('has correct label for fix_failed', () => {
      expect(investigationOutcomeLabels.fix_failed).toBe('Fix Failed');
    });

    it('has correct label for needs_attention', () => {
      expect(investigationOutcomeLabels.needs_attention).toBe('Needs Attention');
    });

    it('has correct label for cannot_fix', () => {
      expect(investigationOutcomeLabels.cannot_fix).toBe('Cannot Auto-Fix');
    });

    it('has correct label for timed_out', () => {
      expect(investigationOutcomeLabels.timed_out).toBe('Timed Out — Will Retry');
    });

    it('has correct label for fix_verified', () => {
      expect(investigationOutcomeLabels.fix_verified).toBe('Fix Verified');
    });

    it('has correct label for fix_verification_failed', () => {
      expect(investigationOutcomeLabels.fix_verification_failed).toBe('Verification Failed');
    });

    it('has correct label for fix_verification_unknown', () => {
      expect(investigationOutcomeLabels.fix_verification_unknown).toBe('Verification Inconclusive');
    });
  });

  describe('investigationOutcomeColors', () => {
    it('has green colors for resolved', () => {
      expect(investigationOutcomeColors.resolved).toContain('green');
    });

    it('has blue colors for fix_queued', () => {
      expect(investigationOutcomeColors.fix_queued).toContain('blue');
    });

    it('has red colors for fix_failed', () => {
      expect(investigationOutcomeColors.fix_failed).toContain('red');
    });

    it('has amber colors for needs_attention', () => {
      expect(investigationOutcomeColors.needs_attention).toContain('amber');
    });

    it('has slate colors for cannot_fix', () => {
      expect(investigationOutcomeColors.cannot_fix).toContain('slate');
    });
  });

  describe('severityColors', () => {
    it('has colors for critical', () => {
      expect(severityColors.critical).toHaveProperty('bg');
      expect(severityColors.critical).toHaveProperty('text');
    });

    it('has colors for warning', () => {
      expect(severityColors.warning).toHaveProperty('bg');
      expect(severityColors.warning).toHaveProperty('text');
    });

    it('has colors for info', () => {
      expect(severityColors.info).toHaveProperty('bg');
      expect(severityColors.info).toHaveProperty('text');
    });

    it('has colors for watch', () => {
      expect(severityColors.watch).toHaveProperty('bg');
      expect(severityColors.watch).toHaveProperty('text');
    });
  });
});
