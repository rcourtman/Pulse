import { describe, expect, it } from 'vitest';

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
      expect(sourceLabels.threshold).toBe('Alert');
    });

    it('has correct label for ai-patrol', () => {
      expect(sourceLabels['ai-patrol']).toBe('Pulse Patrol');
    });

    it('has correct label for anomaly', () => {
      expect(sourceLabels.anomaly).toBe('Anomaly');
    });

    it('has correct label for ai-chat', () => {
      expect(sourceLabels['ai-chat']).toBe('Pulse Assistant');
    });

    it('has correct label for correlation', () => {
      expect(sourceLabels.correlation).toBe('Correlation');
    });

    it('has correct label for forecast', () => {
      expect(sourceLabels.forecast).toBe('Forecast');
    });
  });

  describe('severityColors', () => {
    it('contains critical color classes', () => {
      expect(severityColors.critical).toContain('red-200');
      expect(severityColors.critical).toContain('red-700');
    });

    it('contains warning color classes', () => {
      expect(severityColors.warning).toContain('amber-200');
      expect(severityColors.warning).toContain('amber-700');
    });

    it('contains info color classes', () => {
      expect(severityColors.info).toContain('blue-200');
      expect(severityColors.info).toContain('blue-700');
    });

    it('contains watch color classes', () => {
      expect(severityColors.watch).toContain('bg-surface-alt');
    });
  });

  describe('sourceColors', () => {
    it('has threshold color', () => {
      expect(sourceColors.threshold).toContain('orange');
    });

    it('has ai-patrol color', () => {
      expect(sourceColors['ai-patrol']).toContain('blue');
    });

    it('has ai-chat color', () => {
      expect(sourceColors['ai-chat']).toContain('teal');
    });
  });

  describe('investigationStatusColors', () => {
    it('has pending color', () => {
      expect(investigationStatusColors.pending).toContain('bg-surface-alt');
    });

    it('has running color', () => {
      expect(investigationStatusColors.running).toContain('blue');
    });

    it('has completed color', () => {
      expect(investigationStatusColors.completed).toContain('green');
    });

    it('has failed color', () => {
      expect(investigationStatusColors.failed).toContain('red');
    });
  });

  describe('loopStateColors', () => {
    it('has detected color', () => {
      expect(loopStateColors.detected).toContain('blue');
    });

    it('has resolved color', () => {
      expect(loopStateColors.resolved).toContain('green');
    });

    it('has remediation_failed color', () => {
      expect(loopStateColors.remediation_failed).toContain('red');
    });
  });

  describe('formatLoopState', () => {
    it('replaces underscores with spaces', () => {
      expect(formatLoopState('in_progress')).toBe('in progress');
    });

    it('handles single word', () => {
      expect(formatLoopState('detected')).toBe('detected');
    });

    it('handles multiple underscores', () => {
      expect(formatLoopState('remediation_planned')).toBe('remediation planned');
    });
  });

  describe('lifecycleLabels', () => {
    it('has detected label', () => {
      expect(lifecycleLabels.detected).toBe('Detected');
    });

    it('has resolved label', () => {
      expect(lifecycleLabels.resolved).toBe('Resolved');
    });

    it('has snoozed label', () => {
      expect(lifecycleLabels.snoozed).toBe('Snoozed');
    });

    it('has dismissed label', () => {
      expect(lifecycleLabels.dismissed).toBe('Dismissed');
    });
  });

  describe('formatLifecycleType', () => {
    it('returns known label for known value', () => {
      expect(formatLifecycleType('detected')).toBe('Detected');
    });

    it('replaces underscores for unknown value', () => {
      expect(formatLifecycleType('some_unknown_event')).toBe('some unknown event');
    });

    it('handles auto_resolved', () => {
      expect(formatLifecycleType('auto_resolved')).toBe('Auto-resolved');
    });

    it('handles verification_passed', () => {
      expect(formatLifecycleType('verification_passed')).toBe('Fix verified');
    });
  });
});

const severityOrder: Record<string, number> = {
  critical: 0,
  warning: 1,
  watch: 2,
  info: 3,
};

const sourceLabels: Record<string, string> = {
  threshold: 'Alert',
  'ai-patrol': 'Pulse Patrol',
  anomaly: 'Anomaly',
  'ai-chat': 'Pulse Assistant',
  correlation: 'Correlation',
  forecast: 'Forecast',
};

const severityColors: Record<string, string> = {
  critical:
    'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900 dark:text-red-300',
  warning:
    'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
  info: 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  watch: 'border-border bg-surface-alt text-base-content',
};

const sourceColors: Record<string, string> = {
  threshold:
    'border-orange-200 bg-orange-50 text-orange-700 dark:border-orange-800 dark:bg-orange-900 dark:text-orange-300',
  'ai-patrol':
    'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  anomaly:
    'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  'ai-chat':
    'border-teal-200 bg-teal-50 text-teal-700 dark:border-teal-800 dark:bg-teal-900 dark:text-teal-300',
  correlation:
    'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-800 dark:bg-sky-900 dark:text-sky-300',
  forecast:
    'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-900 dark:text-emerald-300',
};

type InvestigationStatus = 'pending' | 'running' | 'completed' | 'failed' | 'needs_attention';

const investigationStatusColors: Record<InvestigationStatus, string> = {
  pending: 'border-border bg-surface-alt text-muted',
  running:
    'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  completed:
    'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900 dark:text-green-300',
  failed:
    'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900 dark:text-red-300',
  needs_attention:
    'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
};

const loopStateColors: Record<string, string> = {
  detected:
    'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  investigating:
    'border-indigo-200 bg-indigo-50 text-indigo-700 dark:border-indigo-800 dark:bg-indigo-900 dark:text-indigo-300',
  remediation_planned:
    'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
  remediating:
    'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
  remediation_failed:
    'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900 dark:text-red-300',
  needs_attention:
    'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
  timed_out: 'border-border bg-surface-alt text-base-content',
  resolved:
    'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900 dark:text-green-300',
  dismissed: 'border-border bg-surface-alt text-muted',
  snoozed:
    'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
  suppressed: 'border-border bg-surface-alt text-muted',
};

const formatLoopState = (s: string) => s.replace(/_/g, ' ');

const lifecycleLabels: Record<string, string> = {
  detected: 'Detected',
  regressed: 'Regressed',
  acknowledged: 'Acknowledged',
  snoozed: 'Snoozed',
  unsnoozed: 'Unsnoozed',
  dismissed: 'Dismissed',
  undismissed: 'Undismissed',
  suppressed: 'Suppressed',
  resolved: 'Resolved',
  auto_resolved: 'Auto-resolved',
  verification_passed: 'Fix verified',
  investigation_updated: 'Investigation updated',
  investigation_outcome: 'Investigation outcome',
  user_note_updated: 'Note updated',
  loop_state: 'Loop state changed',
  seen_while_suppressed: 'Seen while suppressed',
  loop_transition_violation: 'Invalid transition blocked',
};

const formatLifecycleType = (value: string) => lifecycleLabels[value] || value.replace(/_/g, ' ');
