import { describe, expect, it } from 'vitest';
import {
  formatFindingLifecycleType,
  formatFindingLoopState,
  getFindingLoopStateBadgeClasses,
  getFindingSeverityBadgeClasses,
  getFindingSeverityToneClasses,
  getFindingSourceBadgeClasses,
  getFindingSourceLabel,
  getInvestigationStatusBadgeClasses,
} from '@/utils/aiFindingPresentation';

describe('FindingsPanel constants', () => {
  describe('severityOrder', () => {
    it('has correct order for critical', () => {
      expect(severityOrder.critical).toBe(0);
    });

    it('has correct order for warning', () => {
      expect(severityOrder.warning).toBe(1);
    });

    it('has correct order for watch', () => {
      expect(severityOrder.watch).toBe(2);
    });

    it('has correct order for info', () => {
      expect(severityOrder.info).toBe(3);
    });
  });

  describe('sourceLabels', () => {
    it('has correct label for threshold', () => {
      expect(getFindingSourceLabel('threshold')).toBe('Alert');
    });

    it('has correct label for ai-patrol', () => {
      expect(getFindingSourceLabel('ai-patrol')).toBe('Pulse Patrol');
    });

    it('has correct label for anomaly', () => {
      expect(getFindingSourceLabel('anomaly')).toBe('Anomaly');
    });

    it('has correct label for ai-chat', () => {
      expect(getFindingSourceLabel('ai-chat')).toBe('Pulse Assistant');
    });

    it('has correct label for correlation', () => {
      expect(getFindingSourceLabel('correlation')).toBe('Correlation');
    });

    it('has correct label for forecast', () => {
      expect(getFindingSourceLabel('forecast')).toBe('Forecast');
    });
  });

  describe('severityColors', () => {
    it('contains critical color classes', () => {
      expect(getFindingSeverityBadgeClasses('critical')).toContain('red-200');
      expect(getFindingSeverityBadgeClasses('critical')).toContain('red-700');
    });

    it('contains warning color classes', () => {
      expect(getFindingSeverityBadgeClasses('warning')).toContain('amber-200');
      expect(getFindingSeverityBadgeClasses('warning')).toContain('amber-700');
    });

    it('contains info color classes', () => {
      expect(getFindingSeverityBadgeClasses('info')).toContain('blue-200');
      expect(getFindingSeverityBadgeClasses('info')).toContain('blue-700');
    });

    it('contains watch color classes', () => {
      expect(getFindingSeverityBadgeClasses('watch')).toContain('bg-surface-alt');
    });

    it('contains compact tone classes for critical severity', () => {
      expect(getFindingSeverityToneClasses('critical')).toContain('bg-red-100');
    });
  });

  describe('sourceColors', () => {
    it('has threshold color', () => {
      expect(getFindingSourceBadgeClasses('threshold')).toContain('orange');
    });

    it('has ai-patrol color', () => {
      expect(getFindingSourceBadgeClasses('ai-patrol')).toContain('blue');
    });

    it('has ai-chat color', () => {
      expect(getFindingSourceBadgeClasses('ai-chat')).toContain('teal');
    });
  });

  describe('investigationStatusColors', () => {
    it('has pending color', () => {
      expect(getInvestigationStatusBadgeClasses('pending')).toContain('bg-surface-alt');
    });

    it('has running color', () => {
      expect(getInvestigationStatusBadgeClasses('running')).toContain('blue');
    });

    it('has completed color', () => {
      expect(getInvestigationStatusBadgeClasses('completed')).toContain('green');
    });

    it('has failed color', () => {
      expect(getInvestigationStatusBadgeClasses('failed')).toContain('red');
    });
  });

  describe('loopStateColors', () => {
    it('has detected color', () => {
      expect(getFindingLoopStateBadgeClasses('detected')).toContain('blue');
    });

    it('has resolved color', () => {
      expect(getFindingLoopStateBadgeClasses('resolved')).toContain('green');
    });

    it('has remediation_failed color', () => {
      expect(getFindingLoopStateBadgeClasses('remediation_failed')).toContain('red');
    });
  });

  describe('formatLoopState', () => {
    it('replaces underscores with spaces', () => {
      expect(formatFindingLoopState('in_progress')).toBe('in progress');
    });

    it('handles single word', () => {
      expect(formatFindingLoopState('detected')).toBe('detected');
    });

    it('handles multiple underscores', () => {
      expect(formatFindingLoopState('remediation_planned')).toBe('remediation planned');
    });
  });

  describe('lifecycleLabels', () => {
    it('has detected label', () => {
      expect(formatFindingLifecycleType('detected')).toBe('Detected');
    });

    it('has resolved label', () => {
      expect(formatFindingLifecycleType('resolved')).toBe('Resolved');
    });

    it('has snoozed label', () => {
      expect(formatFindingLifecycleType('snoozed')).toBe('Snoozed');
    });

    it('has dismissed label', () => {
      expect(formatFindingLifecycleType('dismissed')).toBe('Dismissed');
    });
  });

  describe('formatLifecycleType', () => {
    it('returns known label for known value', () => {
      expect(formatFindingLifecycleType('detected')).toBe('Detected');
    });

    it('replaces underscores for unknown value', () => {
      expect(formatFindingLifecycleType('some_unknown_event')).toBe('some unknown event');
    });

    it('handles auto_resolved', () => {
      expect(formatFindingLifecycleType('auto_resolved')).toBe('Auto-resolved');
    });

    it('handles verification_passed', () => {
      expect(formatFindingLifecycleType('verification_passed')).toBe('Fix verified');
    });
  });
});

const severityOrder: Record<string, number> = {
  critical: 0,
  warning: 1,
  watch: 2,
  info: 3,
};
