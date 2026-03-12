import { describe, expect, it } from 'vitest';
import {
  buildFindingFilterOptions,
  formatFindingLifecycleType,
  formatFindingLoopState,
  getFindingEmptyStateCopy,
  getFindingSeverityCompactLabel,
  getFindingSeveritySortOrder,
  getFindingResolutionReason,
  getFindingLoopStateBadgeClasses,
  getFindingSeverityBadgeClasses,
  getFindingStatusBadgeClasses,
  getFindingStatusLabel,
  getFindingSeverityToneClasses,
  getFindingSourceBadgeClasses,
  getFindingSourceLabel,
  hasFindingInvestigationDetails,
  getInvestigationOutcomeBadgeClasses,
  getInvestigationOutcomeLabel,
  getInvestigationOutcomeSortOrder,
  getInvestigationStatusLabel,
  getInvestigationStatusBadgeClasses,
} from '@/utils/aiFindingPresentation';

describe('aiFindingPresentation', () => {
  describe('severity presentation', () => {
    it('has correct sort order for critical', () => {
      expect(getFindingSeveritySortOrder('critical')).toBe(0);
    });

    it('has correct sort order for warning', () => {
      expect(getFindingSeveritySortOrder('warning')).toBe(1);
    });

    it('has correct sort order for watch', () => {
      expect(getFindingSeveritySortOrder('watch')).toBe(2);
    });

    it('has correct sort order for info', () => {
      expect(getFindingSeveritySortOrder('info')).toBe(3);
    });

    it('returns compact severity labels', () => {
      expect(getFindingSeverityCompactLabel('critical')).toBe('CRIT');
      expect(getFindingSeverityCompactLabel('warning')).toBe('WARN');
      expect(getFindingSeverityCompactLabel('watch')).toBe('WATCH');
      expect(getFindingSeverityCompactLabel('info')).toBe('INFO');
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

  describe('findingStatusPresentation', () => {
    it('returns canonical badge classes', () => {
      expect(getFindingStatusBadgeClasses('resolved')).toContain('green');
      expect(getFindingStatusBadgeClasses('snoozed')).toContain('blue');
      expect(getFindingStatusBadgeClasses('dismissed')).toContain('bg-surface-alt');
      expect(getFindingStatusBadgeClasses('unexpected')).toContain('bg-surface-alt');
    });

    it('returns canonical labels', () => {
      expect(getFindingStatusLabel('resolved')).toBe('Resolved');
      expect(getFindingStatusLabel('snoozed')).toBe('Snoozed');
      expect(getFindingStatusLabel('dismissed')).toBe('Dismissed');
      expect(getFindingStatusLabel('unexpected')).toBe('Dismissed');
    });
  });

  describe('filterPresentation', () => {
    it('builds canonical filter options', () => {
      expect(
        buildFindingFilterOptions({
          needsAttentionCount: 2,
          pendingApprovalCount: 1,
        }),
      ).toEqual([
        { value: 'active', label: 'Active' },
        { value: 'all', label: 'All' },
        { value: 'resolved', label: 'Resolved' },
        { value: 'attention', label: 'Needs Attention', tone: 'warning', count: 2 },
        { value: 'approvals', label: 'Approvals', tone: 'warning', count: 1 },
      ]);
    });

    it('returns canonical empty-state copy', () => {
      expect(getFindingEmptyStateCopy('active')).toEqual({
        title: 'No active findings',
        body: 'Your infrastructure looks healthy!',
      });
      expect(getFindingEmptyStateCopy('attention')).toEqual({
        title: 'No findings need attention right now.',
      });
      expect(getFindingEmptyStateCopy('approvals')).toEqual({
        title: 'No pending approvals.',
      });
      expect(getFindingEmptyStateCopy('resolved')).toEqual({
        title: 'No Patrol findings to display',
      });
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

    it('returns canonical status labels', () => {
      expect(getInvestigationStatusLabel('pending')).toBe('Pending');
      expect(getInvestigationStatusLabel('running')).toBe('Running');
      expect(getInvestigationStatusLabel('completed')).toBe('Completed');
      expect(getInvestigationStatusLabel('failed')).toBe('Failed');
      expect(getInvestigationStatusLabel('needs_attention')).toBe('Needs Attention');
    });
  });

  describe('investigationOutcomePresentation', () => {
    it('returns canonical outcome labels and badge classes', () => {
      expect(getInvestigationOutcomeLabel('fix_verified')).toBe('Fix verified');
      expect(getInvestigationOutcomeBadgeClasses('fix_failed')).toContain('red');
      expect(getInvestigationOutcomeBadgeClasses('cannot_fix')).toContain('bg-surface-alt');
      expect(getInvestigationOutcomeSortOrder('fix_failed')).toBe(0);
      expect(getInvestigationOutcomeSortOrder('needs_attention')).toBe(1);
      expect(getInvestigationOutcomeSortOrder('fix_queued')).toBe(2);
      expect(getInvestigationOutcomeSortOrder(undefined)).toBe(3);
    });

    it('treats any investigation metadata as enough to render investigation details', () => {
      expect(
        hasFindingInvestigationDetails({
          investigationSessionId: '',
          investigationStatus: 'failed',
          investigationOutcome: undefined,
          investigationAttempts: 0,
        } as never),
      ).toBe(true);
      expect(
        hasFindingInvestigationDetails({
          investigationSessionId: '',
          investigationStatus: undefined,
          investigationOutcome: 'fix_queued',
          investigationAttempts: 0,
        } as never),
      ).toBe(true);
      expect(
        hasFindingInvestigationDetails({
          investigationSessionId: '',
          investigationStatus: undefined,
          investigationOutcome: undefined,
          investigationAttempts: 2,
        } as never),
      ).toBe(true);
      expect(
        hasFindingInvestigationDetails({
          investigationSessionId: 'session-1',
          investigationStatus: undefined,
          investigationOutcome: undefined,
          investigationAttempts: 0,
        } as never),
      ).toBe(true);
      expect(
        hasFindingInvestigationDetails({
          investigationSessionId: '   ',
          investigationStatus: undefined,
          investigationOutcome: undefined,
          investigationAttempts: 0,
        } as never),
      ).toBe(false);
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

  describe('resolutionReasonPresentation', () => {
    it('returns canonical threshold resolution reasons', () => {
      expect(
        getFindingResolutionReason(
          {
            isThreshold: true,
            source: 'threshold',
            alertType: 'cpu',
            investigationOutcome: undefined,
          } as never,
          '2m ago',
        ),
      ).toBe('CPU returned to normal 2m ago');
    });

    it('returns canonical patrol resolution reasons', () => {
      expect(
        getFindingResolutionReason(
          {
            isThreshold: false,
            source: 'ai-patrol',
            alertType: undefined,
            investigationOutcome: 'fix_verified',
          } as never,
          '1h ago',
        ),
      ).toBe('Fixed by Patrol 1h ago');
    });
  });
});
