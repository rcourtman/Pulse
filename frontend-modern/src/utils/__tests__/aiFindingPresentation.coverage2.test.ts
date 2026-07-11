import { describe, expect, it } from 'vitest';

import type { UnifiedFinding } from '@/stores/aiIntelligence';
import type { ApprovalRequest } from '@/api/ai';
import {
  formatFindingForClipboard,
  getFindingPatrolWorkflowPresentation,
  getFindingResolutionReason,
  getFindingResourceCriticalitySortOrder,
  getFindingSeverityBadgeClasses,
  getFindingSeverityPresentation,
  getFindingSeveritySortOrder,
  getFindingSeverityToneClasses,
  getFindingSourceBadgeClasses,
  getFindingSourceLabel,
  getFindingTitlePresentation,
  getOperatorStateDismissCause,
  getPatrolFindingsBadgePresentation,
  getPatrolFindingResourceGroupKey,
  getPatrolFindingRowScaffold,
  getPatrolWorkTypeCompositionClause,
  isPatrolRuntimeFinding,
  normalizePatrolRuntimeFindingLabel,
  sortFindingsForAttentionQueue,
} from '@/utils/aiFindingPresentation';
import type { PatrolWorkTypeComposition } from '@/utils/aiFindingPresentation';

// ---------------------------------------------------------------------------
// Fixture builder — minimal finding with sensible defaults so each test
// overrides only the fields relevant to the branch under examination.
// ---------------------------------------------------------------------------

const NOW = Date.parse('2026-07-01T00:00:00Z');

function makeFinding(overrides: Partial<UnifiedFinding>): UnifiedFinding {
  return {
    id: overrides.id ?? 'finding-1',
    source: overrides.source ?? 'ai-patrol',
    resourceId: overrides.resourceId ?? 'vm:101',
    resourceName: overrides.resourceName ?? 'db-primary',
    resourceType: overrides.resourceType ?? 'vm',
    category: overrides.category ?? 'performance',
    severity: overrides.severity ?? 'warning',
    title: overrides.title ?? 'CPU saturated',
    description: overrides.description ?? 'CPU is high',
    detectedAt: overrides.detectedAt ?? '2026-06-30T08:00:00Z',
    lastSeenAt: overrides.lastSeenAt ?? overrides.detectedAt ?? '2026-06-30T08:00:00Z',
    status: overrides.status ?? 'active',
    ...overrides,
  };
}

/** A live pending investigation_fix approval targeting `findingId`. */
function liveApproval(findingId: string, now = NOW): ApprovalRequest {
  return {
    id: 'approval-1',
    status: 'pending',
    toolId: 'investigation_fix',
    targetId: findingId,
    expiresAt: new Date(now + 60_000).toISOString(),
  } as ApprovalRequest;
}

// ===================================================================
// getFindingSourceLabel
// ===================================================================

describe('getFindingSourceLabel', () => {
  it('maps every known source to its display label', () => {
    expect(getFindingSourceLabel('threshold')).toBe('Alert');
    expect(getFindingSourceLabel('ai-patrol')).toBe('Pulse Patrol');
    expect(getFindingSourceLabel('anomaly')).toBe('Anomaly');
    expect(getFindingSourceLabel('ai-chat')).toBe('Pulse Assistant');
    expect(getFindingSourceLabel('correlation')).toBe('Correlation');
    expect(getFindingSourceLabel('forecast')).toBe('Forecast');
  });

  it('falls back to the raw source string for an unknown source', () => {
    expect(getFindingSourceLabel('custom-tool')).toBe('custom-tool');
  });

  it('falls back to the raw value for the empty string', () => {
    expect(getFindingSourceLabel('')).toBe('');
  });
});

// ===================================================================
// getFindingSourceBadgeClasses
// ===================================================================

describe('getFindingSourceBadgeClasses', () => {
  it('returns the threshold (orange) classes', () => {
    expect(getFindingSourceBadgeClasses('threshold')).toContain('border-orange-200');
  });

  it('returns the ai-patrol (blue) classes', () => {
    expect(getFindingSourceBadgeClasses('ai-patrol')).toContain('border-blue-200');
  });

  it('returns the anomaly (blue) classes', () => {
    expect(getFindingSourceBadgeClasses('anomaly')).toContain('border-blue-200');
  });

  it('returns the ai-chat (teal) classes', () => {
    expect(getFindingSourceBadgeClasses('ai-chat')).toContain('border-teal-200');
  });

  it('returns the correlation (sky) classes', () => {
    expect(getFindingSourceBadgeClasses('correlation')).toContain('border-sky-200');
  });

  it('returns the forecast (emerald) classes', () => {
    expect(getFindingSourceBadgeClasses('forecast')).toContain('border-emerald-200');
  });

  it('falls back to the ai-patrol classes for an unknown source', () => {
    expect(getFindingSourceBadgeClasses('unknown')).toBe(getFindingSourceBadgeClasses('ai-patrol'));
  });

  it('falls back to the ai-patrol classes for the empty string', () => {
    expect(getFindingSourceBadgeClasses('')).toBe(getFindingSourceBadgeClasses('ai-patrol'));
  });
});

// ===================================================================
// getFindingSeverityBadgeClasses
// ===================================================================

describe('getFindingSeverityBadgeClasses', () => {
  it('returns the critical (red) classes', () => {
    expect(getFindingSeverityBadgeClasses('critical')).toContain('border-red-200');
    expect(getFindingSeverityBadgeClasses('critical')).toContain('text-red-700');
  });

  it('returns the warning (amber) classes', () => {
    expect(getFindingSeverityBadgeClasses('warning')).toContain('border-amber-200');
  });

  it('returns the info (blue) classes', () => {
    expect(getFindingSeverityBadgeClasses('info')).toContain('border-blue-200');
  });

  it('returns the watch (neutral surface) classes', () => {
    expect(getFindingSeverityBadgeClasses('watch')).toBe('border-border bg-surface-alt text-base-content');
  });

  it('falls back to the default badge classes for an unknown severity', () => {
    expect(getFindingSeverityBadgeClasses('severe')).toBe('border-border bg-surface-alt text-muted');
  });

  it('falls back to the default badge classes for the empty string', () => {
    expect(getFindingSeverityBadgeClasses('')).toBe('border-border bg-surface-alt text-muted');
  });
});

// ===================================================================
// getFindingSeverityToneClasses  (named "getFindingSeverityTone" in the
// coverage target list — the exported function is getFindingSeverityToneClasses)
// ===================================================================

describe('getFindingSeverityToneClasses', () => {
  it('returns the critical tone classes', () => {
    expect(getFindingSeverityToneClasses('critical')).toBe(
      'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
    );
  });

  it('returns the warning tone classes', () => {
    expect(getFindingSeverityToneClasses('warning')).toBe(
      'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
    );
  });

  it('returns the info tone classes', () => {
    expect(getFindingSeverityToneClasses('info')).toBe(
      'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300',
    );
  });

  it('returns the watch tone classes', () => {
    expect(getFindingSeverityToneClasses('watch')).toBe('bg-surface-alt text-base-content');
  });

  it('falls back to the muted surface classes for an unknown severity', () => {
    expect(getFindingSeverityToneClasses('unknown')).toBe('bg-surface-alt text-muted');
  });

  it('falls back to the muted surface classes for the empty string', () => {
    expect(getFindingSeverityToneClasses('')).toBe('bg-surface-alt text-muted');
  });
});

// ===================================================================
// getFindingSeveritySortOrder
// ===================================================================

describe('getFindingSeveritySortOrder', () => {
  it('maps each known severity to its sort weight', () => {
    expect(getFindingSeveritySortOrder('critical')).toBe(0);
    expect(getFindingSeveritySortOrder('warning')).toBe(1);
    expect(getFindingSeveritySortOrder('watch')).toBe(2);
    expect(getFindingSeveritySortOrder('info')).toBe(3);
  });

  it('defaults an unknown severity to 4', () => {
    expect(getFindingSeveritySortOrder('catastrophic')).toBe(4);
  });

  it('defaults the empty string to 4', () => {
    expect(getFindingSeveritySortOrder('')).toBe(4);
  });

  it('ranks critical before warning before watch before info', () => {
    expect(getFindingSeveritySortOrder('critical')).toBeLessThan(getFindingSeveritySortOrder('warning'));
    expect(getFindingSeveritySortOrder('warning')).toBeLessThan(getFindingSeveritySortOrder('watch'));
    expect(getFindingSeveritySortOrder('watch')).toBeLessThan(getFindingSeveritySortOrder('info'));
  });
});

// ===================================================================
// getFindingResourceCriticalitySortOrder
// ===================================================================

describe('getFindingResourceCriticalitySortOrder', () => {
  it('maps each known criticality to its exact sort weight', () => {
    expect(getFindingResourceCriticalitySortOrder('high')).toBe(0);
    expect(getFindingResourceCriticalitySortOrder('medium')).toBe(1);
    expect(getFindingResourceCriticalitySortOrder('low')).toBe(3);
  });

  it('treats the empty string as the middle tier (2)', () => {
    expect(getFindingResourceCriticalitySortOrder('')).toBe(2);
  });

  it('treats undefined as the middle tier (2)', () => {
    expect(getFindingResourceCriticalitySortOrder(undefined)).toBe(2);
  });

  it('normalises to lowercase and trims whitespace before lookup', () => {
    expect(getFindingResourceCriticalitySortOrder('  HIGH ')).toBe(0);
    expect(getFindingResourceCriticalitySortOrder('Medium')).toBe(1);
  });

  it('defaults an unrecognised criticality to the middle tier (2)', () => {
    expect(getFindingResourceCriticalitySortOrder('urgent')).toBe(2);
  });
});

// ===================================================================
// getFindingSeverityPresentation
// ===================================================================

describe('getFindingSeverityPresentation', () => {
  it('returns the raw severity label for a non-runtime finding with uppercase true', () => {
    const result = getFindingSeverityPresentation(
      makeFinding({ severity: 'warning', resourceId: 'vm:1' }),
    );
    expect(result.label).toBe('warning');
    expect(result.uppercase).toBe(true);
    expect(result.badgeClasses).toContain('border-amber-200');
    expect(result.badgeTone).toBe('warning');
  });

  it('returns the raw severity for an unknown severity on a non-runtime finding', () => {
    const result = getFindingSeverityPresentation(
      makeFinding({ severity: 'unknown' as UnifiedFinding['severity'], resourceId: 'vm:1' }),
    );
    expect(result.label).toBe('unknown');
    expect(result.uppercase).toBe(true);
  });

  it('returns "Runtime critical" for a patrol-runtime finding with critical severity', () => {
    const result = getFindingSeverityPresentation(
      makeFinding({ severity: 'critical', resourceId: 'ai-service' }),
    );
    expect(result.label).toBe('Runtime critical');
    expect(result.uppercase).toBe(false);
    expect(result.badgeTone).toBe('danger');
    expect(result.badgeClasses).toContain('border-red-200');
  });

  it('returns "Runtime issue" for a patrol-runtime finding with non-critical severity', () => {
    const result = getFindingSeverityPresentation(
      makeFinding({ severity: 'warning', resourceId: 'ai-service' }),
    );
    expect(result.label).toBe('Runtime issue');
    expect(result.uppercase).toBe(false);
    expect(result.badgeTone).toBe('sky');
    expect(result.badgeClasses).toContain('border-sky-200');
  });

  it('returns "Runtime issue" even when the runtime finding has critical severity via title match', () => {
    // resourceId is not 'ai-service' but title starts with 'Pulse Patrol:'
    const result = getFindingSeverityPresentation(
      makeFinding({ severity: 'warning', title: 'Pulse Patrol: some issue' }),
    );
    expect(result.label).toBe('Runtime issue');
  });
});

// ===================================================================
// isPatrolRuntimeFinding
// ===================================================================

describe('isPatrolRuntimeFinding', () => {
  it('is true when resourceId is "ai-service"', () => {
    expect(isPatrolRuntimeFinding({ resourceId: 'ai-service', resourceName: '', title: '' })).toBe(true);
  });

  it('is true when resourceName is "Pulse Patrol Service" (case-insensitive)', () => {
    expect(
      isPatrolRuntimeFinding({ resourceId: '', resourceName: 'Pulse Patrol Service', title: '' }),
    ).toBe(true);
  });

  it('is true when title starts with "Pulse Patrol:" (case-insensitive)', () => {
    expect(
      isPatrolRuntimeFinding({ resourceId: '', resourceName: '', title: 'pulse patrol: credits' }),
    ).toBe(true);
  });

  it('is false when none of the runtime identifiers match', () => {
    expect(isPatrolRuntimeFinding({ resourceId: 'vm:1', resourceName: 'node-1', title: 'CPU high' })).toBe(
      false,
    );
  });

  it('is false when all fields are empty strings', () => {
    expect(isPatrolRuntimeFinding({ resourceId: '', resourceName: '', title: '' })).toBe(false);
  });

  it('trims whitespace before comparing resourceId', () => {
    expect(
      isPatrolRuntimeFinding({ resourceId: '  ai-service  ', resourceName: '', title: '' }),
    ).toBe(true);
  });

  it('does not match partial resourceId like "ai-service-backup"', () => {
    expect(
      isPatrolRuntimeFinding({ resourceId: 'ai-service-backup', resourceName: '', title: '' }),
    ).toBe(false);
  });

  it('does not match a title that contains but does not start with "Pulse Patrol:"', () => {
    expect(
      isPatrolRuntimeFinding({ resourceId: '', resourceName: '', title: 'Issue: Pulse Patrol: down' }),
    ).toBe(false);
  });
});

// ===================================================================
// normalizePatrolRuntimeFindingLabel
// ===================================================================

describe('normalizePatrolRuntimeFindingLabel', () => {
  it('strips a leading "Pulse Patrol:" prefix (case-insensitive)', () => {
    expect(normalizePatrolRuntimeFindingLabel('Pulse Patrol: CPU high')).toBe('CPU high');
  });

  it('strips a lowercase "pulse patrol:" prefix', () => {
    expect(normalizePatrolRuntimeFindingLabel('pulse patrol: CPU high')).toBe('CPU high');
  });

  it('returns "Provider billing or quota issue" for the insufficient-credits label', () => {
    expect(normalizePatrolRuntimeFindingLabel('Insufficient API credits')).toBe(
      'Provider billing or quota issue',
    );
  });

  it('returns "Provider billing or quota issue" even when the prefix is present', () => {
    expect(normalizePatrolRuntimeFindingLabel('Pulse Patrol: Insufficient API credits')).toBe(
      'Provider billing or quota issue',
    );
  });

  it('returns the trimmed title unchanged when it has no special form', () => {
    expect(normalizePatrolRuntimeFindingLabel('  Disk full  ')).toBe('Disk full');
  });

  it('returns empty string for undefined', () => {
    expect(normalizePatrolRuntimeFindingLabel(undefined)).toBe('');
  });

  it('returns empty string for an empty or whitespace-only title', () => {
    expect(normalizePatrolRuntimeFindingLabel('   ')).toBe('');
    expect(normalizePatrolRuntimeFindingLabel('')).toBe('');
  });
});

// ===================================================================
// getFindingTitlePresentation
// ===================================================================

describe('getFindingTitlePresentation', () => {
  it('returns the trimmed raw title for a non-runtime finding', () => {
    expect(getFindingTitlePresentation(makeFinding({ title: '  CPU high  ', resourceId: 'vm:1' }))).toEqual({
      label: 'CPU high',
    });
  });

  it('returns the empty label for a non-runtime finding with no title', () => {
    expect(getFindingTitlePresentation(makeFinding({ title: '', resourceId: 'vm:1' }))).toEqual({
      label: '',
    });
  });

  it('returns the normalized label for a runtime finding with a standard title', () => {
    expect(
      getFindingTitlePresentation(makeFinding({ title: 'Pulse Patrol: CPU high', resourceId: 'ai-service' })),
    ).toEqual({ label: 'CPU high' });
  });

  it('returns "Patrol runtime issue" when the runtime finding title normalises to empty', () => {
    expect(
      getFindingTitlePresentation(makeFinding({ title: '', resourceId: 'ai-service' })),
    ).toEqual({ label: 'Patrol runtime issue' });
  });

  it('returns "Provider billing or quota issue" for the insufficient-credits runtime title', () => {
    expect(
      getFindingTitlePresentation(
        makeFinding({ title: 'Pulse Patrol: Insufficient API credits', resourceId: 'ai-service' }),
      ),
    ).toEqual({ label: 'Provider billing or quota issue' });
  });
});

// ===================================================================
// getPatrolFindingResourceGroupKey
// ===================================================================

describe('getPatrolFindingResourceGroupKey', () => {
  it('groups by subject label when resourceName is present', () => {
    const key = getPatrolFindingResourceGroupKey(
      makeFinding({ id: 'f1', resourceName: 'db-primary', resourceType: '', resourceId: 'vm:1' }),
    );
    expect(key).toBe('subject:db-primary');
  });

  it('groups by subject label (with formatted type) when only resourceType is present', () => {
    const key = getPatrolFindingResourceGroupKey(
      makeFinding({ id: 'f1', resourceName: '', resourceType: 'k8s_pod', resourceId: 'vm:1' }),
    );
    // getFindingSubjectPresentation formats as "resourceName (type)" but resourceName
    // falls back to resourceId when empty. Here resourceName is '' so it uses resourceId.
    // resourceType present → label = `${resourceName-or-id} (${formatIdentifierLabel(type)})`
    expect(key).toBe('subject:vm:1 (k8s pod)');
  });

  it('groups by subject label using resourceName when both name and type are present', () => {
    const key = getPatrolFindingResourceGroupKey(
      makeFinding({ id: 'f1', resourceName: 'web-node', resourceType: 'vm', resourceId: 'vm:1' }),
    );
    expect(key).toBe('subject:web-node (vm)');
  });

  it('groups by resourceId when neither resourceName nor resourceType is present', () => {
    const key = getPatrolFindingResourceGroupKey(
      makeFinding({ id: 'f1', resourceName: '', resourceType: '', resourceId: 'vm:42' }),
    );
    expect(key).toBe('id:vm:42');
  });

  it('groups by finding id when nothing is present', () => {
    const key = getPatrolFindingResourceGroupKey(
      makeFinding({ id: 'lonely', resourceName: '', resourceType: '', resourceId: '' }),
    );
    expect(key).toBe('finding:lonely');
  });

  it('lowercases the subject label for consistent grouping', () => {
    const key = getPatrolFindingResourceGroupKey(
      makeFinding({ id: 'f1', resourceName: 'DB-Primary', resourceType: '', resourceId: 'vm:1' }),
    );
    expect(key).toBe('subject:db-primary');
  });
});

// ===================================================================
// getPatrolFindingsBadgePresentation
// ===================================================================

describe('getPatrolFindingsBadgePresentation', () => {
  it('returns danger for an active non-runtime critical finding', () => {
    expect(
      getPatrolFindingsBadgePresentation([
        makeFinding({ status: 'active', severity: 'critical', resourceId: 'vm:1' }),
      ]),
    ).toEqual({ tone: 'danger' });
  });

  it('returns info for an active runtime critical finding', () => {
    expect(
      getPatrolFindingsBadgePresentation([
        makeFinding({ status: 'active', severity: 'critical', resourceId: 'ai-service' }),
      ]),
    ).toEqual({ tone: 'info' });
  });

  it('returns warning for an active non-runtime warning finding', () => {
    expect(
      getPatrolFindingsBadgePresentation([
        makeFinding({ status: 'active', severity: 'warning', resourceId: 'vm:1' }),
      ]),
    ).toEqual({ tone: 'warning' });
  });

  it('returns info for an active runtime warning finding', () => {
    expect(
      getPatrolFindingsBadgePresentation([
        makeFinding({ status: 'active', severity: 'warning', resourceId: 'ai-service' }),
      ]),
    ).toEqual({ tone: 'info' });
  });

  it('returns muted when only non-critical/non-warning active findings exist', () => {
    expect(
      getPatrolFindingsBadgePresentation([
        makeFinding({ status: 'active', severity: 'info', resourceId: 'vm:1' }),
      ]),
    ).toEqual({ tone: 'muted' });
  });

  it('returns muted for an empty findings list', () => {
    expect(getPatrolFindingsBadgePresentation([])).toEqual({ tone: 'muted' });
  });

  it('returns muted when all critical/warning findings are resolved (not active)', () => {
    expect(
      getPatrolFindingsBadgePresentation([
        makeFinding({ status: 'resolved', severity: 'critical', resourceId: 'vm:1' }),
        makeFinding({ status: 'dismissed', severity: 'warning', resourceId: 'vm:2' }),
      ]),
    ).toEqual({ tone: 'muted' });
  });

  it('prioritises non-runtime critical (danger) over runtime warning (info)', () => {
    expect(
      getPatrolFindingsBadgePresentation([
        makeFinding({ status: 'active', severity: 'warning', resourceId: 'ai-service' }),
        makeFinding({ status: 'active', severity: 'critical', resourceId: 'vm:1' }),
      ]),
    ).toEqual({ tone: 'danger' });
  });

  it('returns info when both runtime-critical and non-runtime-warning are active', () => {
    expect(
      getPatrolFindingsBadgePresentation([
        makeFinding({ status: 'active', severity: 'critical', resourceId: 'ai-service' }),
        makeFinding({ status: 'active', severity: 'warning', resourceId: 'vm:1' }),
      ]),
    ).toEqual({ tone: 'info' });
  });
});

// ===================================================================
// getFindingPatrolWorkflowPresentation
// ===================================================================

describe('getFindingPatrolWorkflowPresentation', () => {
  it('returns undefined when the finding source is not ai-patrol', () => {
    expect(
      getFindingPatrolWorkflowPresentation(makeFinding({ source: 'threshold' })),
    ).toBeUndefined();
  });

  it('returns the recorded stage for a resolved finding', () => {
    const result = getFindingPatrolWorkflowPresentation(
      makeFinding({ source: 'ai-patrol', status: 'resolved' }),
    );
    expect(result).toEqual({
      stage: 'recorded',
      label: 'Outcome recorded',
      detail: 'Patrol has a verified or cleared outcome for this finding.',
      tone: 'success',
    });
  });

  it('returns the paused stage for a snoozed finding', () => {
    const result = getFindingPatrolWorkflowPresentation(
      makeFinding({ source: 'ai-patrol', status: 'snoozed' }),
    );
    expect(result?.stage).toBe('paused');
    expect(result?.label).toBe('Paused until reminder');
  });

  it('returns undefined for a dismissed finding (status not active/resolved/snoozed)', () => {
    expect(
      getFindingPatrolWorkflowPresentation(makeFinding({ source: 'ai-patrol', status: 'dismissed' })),
    ).toBeUndefined();
  });

  it('returns the attention stage for a runtime finding even when active', () => {
    const result = getFindingPatrolWorkflowPresentation(
      makeFinding({ source: 'ai-patrol', status: 'active', resourceId: 'ai-service' }),
    );
    expect(result?.stage).toBe('attention');
    expect(result?.label).toBe('Fix Patrol setup');
  });

  it('returns the approval stage with "Approve or reject" when a live approval exists', () => {
    const finding = makeFinding({ id: 'f1', source: 'ai-patrol', status: 'active', resourceId: 'vm:1' });
    const result = getFindingPatrolWorkflowPresentation(finding, [liveApproval('f1')], NOW);
    expect(result?.stage).toBe('approval');
    expect(result?.label).toBe('Approve or reject');
    expect(result?.tone).toBe('warning');
  });

  it('returns the investigating stage when investigationStatus is running', () => {
    const result = getFindingPatrolWorkflowPresentation(
      makeFinding({ source: 'ai-patrol', status: 'active', resourceId: 'vm:1', investigationStatus: 'running' }),
    );
    expect(result?.stage).toBe('investigating');
    expect(result?.label).toBe('Patrol investigating');
    expect(result?.tone).toBe('indigo');
  });

  it('returns the investigating stage when investigationStatus is pending', () => {
    const result = getFindingPatrolWorkflowPresentation(
      makeFinding({ source: 'ai-patrol', status: 'active', resourceId: 'vm:1', investigationStatus: 'pending' }),
    );
    expect(result?.stage).toBe('investigating');
  });

  it('returns the investigating stage when loopState is investigating', () => {
    const result = getFindingPatrolWorkflowPresentation(
      makeFinding({ source: 'ai-patrol', status: 'active', resourceId: 'vm:1', loopState: 'investigating' }),
    );
    expect(result?.stage).toBe('investigating');
  });

  it('returns the recorded stage for an active finding with fix_verified outcome', () => {
    const result = getFindingPatrolWorkflowPresentation(
      makeFinding({ source: 'ai-patrol', status: 'active', resourceId: 'vm:1', investigationOutcome: 'fix_verified' }),
    );
    expect(result?.stage).toBe('recorded');
    expect(result?.label).toBe('Outcome recorded');
  });

  it('returns the recorded stage for an active finding with resolved outcome', () => {
    const result = getFindingPatrolWorkflowPresentation(
      makeFinding({ source: 'ai-patrol', status: 'active', resourceId: 'vm:1', investigationOutcome: 'resolved' }),
    );
    expect(result?.stage).toBe('recorded');
  });

  it('returns the approval stage with "Recover action" for fix_queued without a live approval', () => {
    const result = getFindingPatrolWorkflowPresentation(
      makeFinding({ id: 'f1', source: 'ai-patrol', status: 'active', resourceId: 'vm:1', investigationOutcome: 'fix_queued' }),
    );
    expect(result?.stage).toBe('approval');
    expect(result?.label).toBe('Recover action');
  });

  it('returns the verification stage for fix_executed', () => {
    const result = getFindingPatrolWorkflowPresentation(
      makeFinding({ source: 'ai-patrol', status: 'active', resourceId: 'vm:1', investigationOutcome: 'fix_executed' }),
    );
    expect(result?.stage).toBe('verification');
    expect(result?.label).toBe('Verify outcome');
  });

  it('returns the attention stage for fix_failed', () => {
    const result = getFindingPatrolWorkflowPresentation(
      makeFinding({ source: 'ai-patrol', status: 'active', resourceId: 'vm:1', investigationOutcome: 'fix_failed' }),
    );
    expect(result?.stage).toBe('attention');
    expect(result?.label).toBe('Fix failed');
    expect(result?.tone).toBe('danger');
  });

  it('returns the attention stage for fix_verification_failed', () => {
    const result = getFindingPatrolWorkflowPresentation(
      makeFinding({ source: 'ai-patrol', status: 'active', resourceId: 'vm:1', investigationOutcome: 'fix_verification_failed' }),
    );
    expect(result?.stage).toBe('attention');
  });

  it('returns the attention stage for fix_rejected with "Decide follow-up"', () => {
    const result = getFindingPatrolWorkflowPresentation(
      makeFinding({ source: 'ai-patrol', status: 'active', resourceId: 'vm:1', investigationOutcome: 'fix_rejected' }),
    );
    expect(result?.stage).toBe('attention');
    expect(result?.label).toBe('Decide follow-up');
    expect(result?.tone).toBe('warning');
  });

  it('returns the verification stage for fix_verification_unknown', () => {
    const result = getFindingPatrolWorkflowPresentation(
      makeFinding({ source: 'ai-patrol', status: 'active', resourceId: 'vm:1', investigationOutcome: 'fix_verification_unknown' }),
    );
    expect(result?.stage).toBe('verification');
    expect(result?.label).toBe('Check outcome');
  });

  it('returns the attention stage with "Needs input" for needs_attention', () => {
    const result = getFindingPatrolWorkflowPresentation(
      makeFinding({ source: 'ai-patrol', status: 'active', resourceId: 'vm:1', investigationOutcome: 'needs_attention' }),
    );
    expect(result?.stage).toBe('attention');
    expect(result?.label).toBe('Needs input');
  });

  it('returns the attention stage for cannot_fix', () => {
    const result = getFindingPatrolWorkflowPresentation(
      makeFinding({ source: 'ai-patrol', status: 'active', resourceId: 'vm:1', investigationOutcome: 'cannot_fix' }),
    );
    expect(result?.stage).toBe('attention');
  });

  it('returns the attention stage for timed_out', () => {
    const result = getFindingPatrolWorkflowPresentation(
      makeFinding({ source: 'ai-patrol', status: 'active', resourceId: 'vm:1', investigationOutcome: 'timed_out' }),
    );
    expect(result?.stage).toBe('attention');
  });

  it('returns undefined for an active finding with an unrecognised outcome and no investigation status', () => {
    expect(
      getFindingPatrolWorkflowPresentation(
        makeFinding({ source: 'ai-patrol', status: 'active', resourceId: 'vm:1', investigationOutcome: undefined }),
      ),
    ).toBeUndefined();
  });
});

// ===================================================================
// getPatrolFindingVerificationSummary (private, reached via
// getPatrolFindingRowScaffold — the scaffold guard requires source
// 'ai-patrol', status 'active', and NOT a patrol-runtime finding)
// ===================================================================

describe('getPatrolFindingVerificationSummary (via getPatrolFindingRowScaffold)', () => {
  const verificationValue = (overrides: Partial<UnifiedFinding>): string | undefined => {
    const scaffold = getPatrolFindingRowScaffold(
      makeFinding({ source: 'ai-patrol', status: 'active', resourceId: 'vm:1', ...overrides }),
    );
    return scaffold?.items.find((item) => item.id === 'verification')?.value;
  };

  it('reports waiting-for-approval for fix_queued', () => {
    expect(verificationValue({ investigationOutcome: 'fix_queued' })).toBe(
      'Waiting for the governed action record before any change runs.',
    );
  });

  it('reports verification-in-progress for fix_executed', () => {
    expect(verificationValue({ investigationOutcome: 'fix_executed' })).toBe(
      'Fix ran; verification is in progress.',
    );
  });

  it('reports verified outcome for fix_verified', () => {
    expect(verificationValue({ investigationOutcome: 'fix_verified' })).toBe(
      'Verified outcome recorded.',
    );
  });

  it('reports verified outcome for resolved', () => {
    expect(verificationValue({ investigationOutcome: 'resolved' })).toBe(
      'Verified outcome recorded.',
    );
  });

  it('reports verification failed for fix_verification_failed', () => {
    expect(verificationValue({ investigationOutcome: 'fix_verification_failed' })).toBe(
      'Verification failed and needs review.',
    );
  });

  it('reports inconclusive for fix_verification_unknown', () => {
    expect(verificationValue({ investigationOutcome: 'fix_verification_unknown' })).toBe(
      'Verification was inconclusive.',
    );
  });

  it('reports no verified fix for fix_failed', () => {
    expect(verificationValue({ investigationOutcome: 'fix_failed' })).toBe(
      'No verified fix; action needs review.',
    );
  });

  it('reports no verified fix for cannot_fix', () => {
    expect(verificationValue({ investigationOutcome: 'cannot_fix' })).toBe(
      'No verified fix; action needs review.',
    );
  });

  it('reports no verified fix for timed_out', () => {
    expect(verificationValue({ investigationOutcome: 'timed_out' })).toBe(
      'No verified fix; action needs review.',
    );
  });

  it('reports no verified fix for needs_attention', () => {
    expect(verificationValue({ investigationOutcome: 'needs_attention' })).toBe(
      'No verified fix; action needs review.',
    );
  });

  it('reports rejected fix for fix_rejected', () => {
    expect(verificationValue({ investigationOutcome: 'fix_rejected' })).toBe(
      'No change ran because the fix was rejected.',
    );
  });

  it('reports investigating when investigationStatus is running and no outcome', () => {
    expect(verificationValue({ investigationStatus: 'running' })).toBe(
      'Patrol is investigating; no fix has run yet.',
    );
  });

  it('reports investigating when investigationStatus is pending and no outcome', () => {
    expect(verificationValue({ investigationStatus: 'pending' })).toBe(
      'Patrol is investigating; no fix has run yet.',
    );
  });

  it('reports no-fix-yet when no outcome and investigationStatus is completed', () => {
    expect(verificationValue({ investigationStatus: 'completed' })).toBe('No fix has run yet.');
  });

  it('reports no-fix-yet when no outcome and no investigation status', () => {
    expect(verificationValue({})).toBe('No fix has run yet.');
  });
});

// ===================================================================
// getPatrolFindingWorkflowSummary (private, reached via
// getPatrolFindingRowScaffold). The 'paused' stage is unreachable here
// because the scaffold guard requires status 'active' and 'paused'
// only arises from status 'snoozed'.
// ===================================================================

describe('getPatrolFindingWorkflowSummary (via getPatrolFindingRowScaffold)', () => {
  const workflowValue = (overrides: Partial<UnifiedFinding>, approvals: ApprovalRequest[] = []): string | undefined => {
    const scaffold = getPatrolFindingRowScaffold(
      makeFinding({ source: 'ai-patrol', status: 'active', resourceId: 'vm:1', ...overrides }),
      approvals,
      NOW,
    );
    return scaffold?.items.find((item) => item.id === 'workflow')?.value;
  };

  it('returns the default review message when no workflow stage is determined', () => {
    // Active, non-runtime, no investigation status/outcome → workflow undefined
    expect(workflowValue({})).toBe(
      'Review evidence, decide the next action, and verify any outcome before closing.',
    );
  });

  it('returns the approve-or-reject message when a live approval is pending', () => {
    expect(
      workflowValue({ id: 'f1' }, [liveApproval('f1')]),
    ).toBe(
      'Review evidence first; no change runs until the typed action is approved, then Patrol verifies the outcome.',
    );
  });

  it('returns the recover-queued-fix message for fix_queued without a live approval (Recover action stage)', () => {
    expect(
      workflowValue({ investigationOutcome: 'fix_queued' }),
    ).toBe(
      'Recover the queued action before any change can run, then verify the outcome after a decision.',
    );
  });

  it('returns the verification message for fix_executed', () => {
    expect(
      workflowValue({ investigationOutcome: 'fix_executed' }),
    ).toBe('The governed action ran; review follow-up evidence before closing the issue.');
  });

  it('returns the attention message for fix_failed', () => {
    expect(
      workflowValue({ investigationOutcome: 'fix_failed' }),
    ).toBe('Review the blocked or failed step before approving another change or resolving manually.');
  });

  it('returns the investigating message for running investigationStatus', () => {
    expect(
      workflowValue({ investigationStatus: 'running' }),
    ).toBe('Patrol is explaining the issue and preparing the next decision point.');
  });

  it('returns the recorded message for fix_verified outcome', () => {
    expect(
      workflowValue({ investigationOutcome: 'fix_verified' }),
    ).toBe('Patrol recorded the outcome; use history if you need the completed trail.');
  });
});

// ===================================================================
// getFindingResolutionReason
// ===================================================================

describe('getFindingResolutionReason', () => {
  const base = {
    isThreshold: false as boolean | undefined,
    source: 'ai-chat' as UnifiedFinding['source'],
    alertType: undefined as string | undefined,
    investigationOutcome: undefined as UnifiedFinding['investigationOutcome'],
    autoResolved: undefined as boolean | undefined,
  };

  describe('manual resolution priority', () => {
    it('returns "Resolved by you" when autoResolved is explicitly false and outcome is not a patrol fix', () => {
      expect(
        getFindingResolutionReason({ ...base, autoResolved: false }, 'today'),
      ).toBe('Resolved by you today');
    });

    it('does NOT return "Resolved by you" when autoResolved is false but outcome is fix_verified', () => {
      const reason = getFindingResolutionReason(
        { ...base, autoResolved: false, investigationOutcome: 'fix_verified' },
        'today',
      );
      expect(reason).not.toContain('Resolved by you');
    });

    it('does NOT return "Resolved by you" when autoResolved is false but outcome is fix_executed', () => {
      const reason = getFindingResolutionReason(
        { ...base, autoResolved: false, investigationOutcome: 'fix_executed' },
        'today',
      );
      expect(reason).not.toContain('Resolved by you');
    });

    it('does NOT return "Resolved by you" when autoResolved is false but outcome is resolved', () => {
      const reason = getFindingResolutionReason(
        { ...base, autoResolved: false, investigationOutcome: 'resolved' },
        'today',
      );
      expect(reason).not.toContain('Resolved by you');
    });

    it('does NOT return "Resolved by you" when autoResolved is undefined', () => {
      const reason = getFindingResolutionReason({ ...base, autoResolved: undefined }, 'today');
      expect(reason).not.toContain('Resolved by you');
    });
  });

  describe('threshold alert types', () => {
    const thresholdBase = { ...base, isThreshold: true, source: 'threshold' as const };

    it('returns guest-online for powered-off alertType', () => {
      expect(
        getFindingResolutionReason({ ...thresholdBase, alertType: 'powered-off' }, 'now'),
      ).toBe('Guest came online now');
    });

    it('returns agent-online for host-offline alertType', () => {
      expect(
        getFindingResolutionReason({ ...thresholdBase, alertType: 'host-offline' }, 'now'),
      ).toBe('Agent came online now');
    });

    it('returns cpu-normal for cpu alertType', () => {
      expect(
        getFindingResolutionReason({ ...thresholdBase, alertType: 'cpu' }, 'now'),
      ).toBe('CPU returned to normal now');
    });

    it('returns memory-normal for memory alertType', () => {
      expect(
        getFindingResolutionReason({ ...thresholdBase, alertType: 'memory' }, 'now'),
      ).toBe('Memory returned to normal now');
    });

    it('returns disk-normal for disk alertType', () => {
      expect(
        getFindingResolutionReason({ ...thresholdBase, alertType: 'disk' }, 'now'),
      ).toBe('Disk usage returned to normal now');
    });

    it('returns network-recovered for network alertType', () => {
      expect(
        getFindingResolutionReason({ ...thresholdBase, alertType: 'network' }, 'now'),
      ).toBe('Network recovered now');
    });

    it('returns condition-cleared for an unrecognised alertType', () => {
      expect(
        getFindingResolutionReason({ ...thresholdBase, alertType: 'custom' }, 'now'),
      ).toBe('Condition cleared now');
    });

    it('returns condition-cleared when alertType is undefined', () => {
      expect(
        getFindingResolutionReason({ ...thresholdBase, alertType: undefined }, 'now'),
      ).toBe('Condition cleared now');
    });

    it('treats source=threshold (without isThreshold) as threshold', () => {
      expect(
        getFindingResolutionReason(
          { ...base, isThreshold: false, source: 'threshold', alertType: 'cpu' },
          'now',
        ),
      ).toBe('CPU returned to normal now');
    });
  });

  describe('ai-patrol investigation outcomes', () => {
    const patrolBase = { ...base, source: 'ai-patrol' as const };

    it('returns "Fixed by Patrol" for fix_verified', () => {
      expect(
        getFindingResolutionReason({ ...patrolBase, investigationOutcome: 'fix_verified' }, 'now'),
      ).toBe('Fixed by Patrol now');
    });

    it('returns "Fix applied by Patrol" for fix_executed', () => {
      expect(
        getFindingResolutionReason({ ...patrolBase, investigationOutcome: 'fix_executed' }, 'now'),
      ).toBe('Fix applied by Patrol now');
    });

    it('returns "Resolved by Patrol" for resolved', () => {
      expect(
        getFindingResolutionReason({ ...patrolBase, investigationOutcome: 'resolved' }, 'now'),
      ).toBe('Resolved by Patrol now');
    });

    it('returns "Resolved after fix failed" for fix_failed', () => {
      expect(
        getFindingResolutionReason({ ...patrolBase, investigationOutcome: 'fix_failed' }, 'now'),
      ).toBe('Resolved after fix failed now');
    });

    it('returns "Resolved after rejected fix" for fix_rejected', () => {
      expect(
        getFindingResolutionReason({ ...patrolBase, investigationOutcome: 'fix_rejected' }, 'now'),
      ).toBe('Resolved after rejected fix now');
    });

    it('returns "Resolved while fix was pending" for fix_queued', () => {
      expect(
        getFindingResolutionReason({ ...patrolBase, investigationOutcome: 'fix_queued' }, 'now'),
      ).toBe('Resolved while fix was pending now');
    });

    it('returns "Resolved after failed verification" for fix_verification_failed', () => {
      expect(
        getFindingResolutionReason(
          { ...patrolBase, investigationOutcome: 'fix_verification_failed' },
          'now',
        ),
      ).toBe('Resolved after failed verification now');
    });

    it('returns "Resolved after inconclusive verification" for fix_verification_unknown', () => {
      expect(
        getFindingResolutionReason(
          { ...patrolBase, investigationOutcome: 'fix_verification_unknown' },
          'now',
        ),
      ).toBe('Resolved after inconclusive verification now');
    });

    it('returns "Resolved after investigation timeout" for timed_out', () => {
      expect(
        getFindingResolutionReason({ ...patrolBase, investigationOutcome: 'timed_out' }, 'now'),
      ).toBe('Resolved after investigation timeout now');
    });

    it('returns "Resolved manually" for cannot_fix', () => {
      expect(
        getFindingResolutionReason({ ...patrolBase, investigationOutcome: 'cannot_fix' }, 'now'),
      ).toBe('Resolved manually now');
    });

    it('returns "Resolved after manual review" for needs_attention', () => {
      expect(
        getFindingResolutionReason({ ...patrolBase, investigationOutcome: 'needs_attention' }, 'now'),
      ).toBe('Resolved after manual review now');
    });

    it('returns "Fix applied by Patrol" for fix_executed even when autoResolved is false', () => {
      expect(
        getFindingResolutionReason(
          { ...patrolBase, investigationOutcome: 'fix_executed', autoResolved: false },
          'now',
        ),
      ).toBe('Fix applied by Patrol now');
    });

    it('returns the default patrol fallback when no outcome is set', () => {
      expect(
        getFindingResolutionReason({ ...patrolBase, investigationOutcome: undefined }, 'now'),
      ).toBe('Issue no longer detected now');
    });
  });

  describe('fallback for non-threshold non-patrol sources', () => {
    it('returns the generic "Resolved" message', () => {
      expect(getFindingResolutionReason({ ...base, source: 'anomaly' }, 'now')).toBe('Resolved now');
    });

    it('returns the generic "Resolved" message for ai-chat source', () => {
      expect(getFindingResolutionReason({ ...base, source: 'ai-chat' }, 'now')).toBe('Resolved now');
    });
  });
});

// ===================================================================
// formatFindingForClipboard
// ===================================================================

describe('formatFindingForClipboard', () => {
  it('formats a fully-populated finding with all optional sections', () => {
    const result = formatFindingForClipboard(
      makeFinding({
        severity: 'critical',
        title: 'DB down',
        resourceName: 'db-primary',
        resourceType: 'vm',
        description: 'The database is unreachable.',
        impact: 'All writes are failing.',
        recommendation: 'Restart the service.',
        investigationRecord: { confidence: 'high' } as UnifiedFinding['investigationRecord'],
        regressionCount: 2,
      }),
    );
    expect(result).toContain('**[CRITICAL] DB down**');
    expect(result).toContain('Resource: db-primary (vm)');
    expect(result).toContain('Description: The database is unreachable.');
    expect(result).toContain('Impact: All writes are failing.');
    expect(result).toContain('Confidence: high');
    expect(result).toContain('Regressed: 2×');
  });

  it('uses "FINDING" when severity is empty', () => {
    const result = formatFindingForClipboard(
      makeFinding({ severity: '' as UnifiedFinding['severity'], title: 'X' }),
    );
    expect(result).toContain('**[FINDING] X**');
  });

  it('uses "Untitled finding" when title is empty', () => {
    const result = formatFindingForClipboard(makeFinding({ title: '' }));
    expect(result).toContain('**[WARNING] Untitled finding**');
  });

  it('omits the resource line when neither resourceName nor resourceType is set', () => {
    const result = formatFindingForClipboard(
      makeFinding({ resourceName: '', resourceType: '', title: 'Issue' }),
    );
    expect(result).not.toContain('Resource:');
  });

  it('includes resource with only resourceName (no type parenthetical)', () => {
    const result = formatFindingForClipboard(
      makeFinding({ resourceName: 'web-node', resourceType: '', title: 'Issue' }),
    );
    expect(result).toContain('Resource: web-node');
    expect(result).not.toContain('(');
  });

  it('omits description and impact sections when they are absent', () => {
    const result = formatFindingForClipboard(
      makeFinding({ description: '', impact: undefined, title: 'Issue' }),
    );
    expect(result).not.toContain('Description:');
    expect(result).not.toContain('Impact:');
  });

  it('omits the trust line when neither confidence nor regressionCount is present', () => {
    const result = formatFindingForClipboard(
      makeFinding({ title: 'Issue', investigationRecord: undefined, regressionCount: 0 }),
    );
    expect(result).not.toContain('Confidence:');
    expect(result).not.toContain('Regressed:');
  });

  it('includes only confidence (no regression) in the trust line', () => {
    const result = formatFindingForClipboard(
      makeFinding({
        title: 'Issue',
        investigationRecord: { confidence: 'low' } as UnifiedFinding['investigationRecord'],
        regressionCount: 0,
      }),
    );
    expect(result).toContain('Confidence: low');
    expect(result).not.toContain('Regressed:');
  });

  it('includes only regression (no confidence) in the trust line', () => {
    const result = formatFindingForClipboard(
      makeFinding({ title: 'Issue', investigationRecord: undefined, regressionCount: 3 }),
    );
    expect(result).toContain('Regressed: 3×');
    expect(result).not.toContain('Confidence:');
  });

  it('does not include regression when regressionCount is zero', () => {
    const result = formatFindingForClipboard(
      makeFinding({ title: 'Issue', regressionCount: 0 }),
    );
    expect(result).not.toContain('Regressed:');
  });
});

// ===================================================================
// getOperatorStateDismissCause
// ===================================================================

describe('getOperatorStateDismissCause', () => {
  it('returns empty string when lifecycle is undefined', () => {
    expect(getOperatorStateDismissCause({ lifecycle: undefined })).toBe('');
  });

  it('returns empty string when lifecycle is an empty array', () => {
    expect(getOperatorStateDismissCause({ lifecycle: [] })).toBe('');
  });

  it('returns the cause from the most recent dismissed event', () => {
    expect(
      getOperatorStateDismissCause({
        lifecycle: [
          { at: 't1', type: 'detected' },
          { at: 't2', type: 'dismissed', metadata: { operator_state_cause: 'maintenance_window' } },
        ],
      }),
    ).toBe('maintenance_window');
  });

  it('returns the cause even when a non-dismissed event follows the dismissed one', () => {
    expect(
      getOperatorStateDismissCause({
        lifecycle: [
          { at: 't1', type: 'dismissed', metadata: { operator_state_cause: 'intentionally_offline' } },
          { at: 't2', type: 'resolved' },
        ],
      }),
    ).toBe('intentionally_offline');
  });

  it('returns empty string for a dismissed event without a cause (manual dismissal)', () => {
    expect(
      getOperatorStateDismissCause({
        lifecycle: [{ at: 't1', type: 'dismissed' }],
      }),
    ).toBe('');
  });

  it('stops scanning at the first dismissed-without-cause event so a stale earlier cause does not leak', () => {
    expect(
      getOperatorStateDismissCause({
        lifecycle: [
          { at: 't1', type: 'dismissed', metadata: { operator_state_cause: 'maintenance_window' } },
          { at: 't2', type: 'dismissed' },
        ],
      }),
    ).toBe('');
  });

  it('returns empty string when no dismissed events exist', () => {
    expect(
      getOperatorStateDismissCause({
        lifecycle: [
          { at: 't1', type: 'detected' },
          { at: 't2', type: 'resolved' },
        ],
      }),
    ).toBe('');
  });

  it('returns empty string when the dismissed metadata exists but has no operator_state_cause key', () => {
    expect(
      getOperatorStateDismissCause({
        lifecycle: [{ at: 't1', type: 'dismissed', metadata: { other_key: 'val' } }],
      }),
    ).toBe('');
  });
});

// ===================================================================
// getPatrolWorkTypeCompositionClause
// ===================================================================

describe('getPatrolWorkTypeCompositionClause', () => {
  const emptyComp: PatrolWorkTypeComposition = {
    total: 0,
    approval: 0,
    failed: 0,
    inProgress: 0,
    recurring: 0,
    newIssues: 0,
  };

  it('returns empty string when all counts are zero', () => {
    expect(getPatrolWorkTypeCompositionClause(emptyComp)).toBe('');
  });

  it('uses singular "needs approval" for exactly one approval', () => {
    expect(
      getPatrolWorkTypeCompositionClause({ ...emptyComp, approval: 1 }),
    ).toBe(' — 1 needs approval');
  });

  it('uses plural "need approval" for multiple approvals', () => {
    expect(
      getPatrolWorkTypeCompositionClause({ ...emptyComp, approval: 3 }),
    ).toBe(' — 3 need approval');
  });

  it('uses singular "failed fix" for exactly one failed', () => {
    expect(
      getPatrolWorkTypeCompositionClause({ ...emptyComp, failed: 1 }),
    ).toBe(' — 1 failed fix');
  });

  it('uses plural "failed fixes" for multiple failed', () => {
    expect(
      getPatrolWorkTypeCompositionClause({ ...emptyComp, failed: 2 }),
    ).toBe(' — 2 failed fixes');
  });

  it('includes in-progress count', () => {
    expect(
      getPatrolWorkTypeCompositionClause({ ...emptyComp, inProgress: 4 }),
    ).toBe(' — 4 in progress');
  });

  it('includes recurring count', () => {
    expect(
      getPatrolWorkTypeCompositionClause({ ...emptyComp, recurring: 5 }),
    ).toBe(' — 5 recurring');
  });

  it('joins all parts with commas in priority order', () => {
    expect(
      getPatrolWorkTypeCompositionClause({
        ...emptyComp,
        approval: 1,
        failed: 2,
        inProgress: 3,
        recurring: 4,
      }),
    ).toBe(' — 1 needs approval, 2 failed fixes, 3 in progress, 4 recurring');
  });
});

// ===================================================================
// sortFindingsForAttentionQueue — outcome branch and recency tiebreaker
// (existing tests cover severity + resource-criticality branches)
// ===================================================================

describe('sortFindingsForAttentionQueue (outcome ordering + recency tiebreaker)', () => {
  it('orders a failed-verification outcome before a queued-fix outcome', () => {
    const sorted = sortFindingsForAttentionQueue([
      makeFinding({ id: 'queued', investigationOutcome: 'fix_queued' }),
      makeFinding({ id: 'failed-verify', investigationOutcome: 'fix_verification_failed' }),
    ]);
    expect(sorted.map((f) => f.id)).toEqual(['failed-verify', 'queued']);
  });

  it('orders an active finding with a fix outcome before a resolved finding with the same severity', () => {
    const sorted = sortFindingsForAttentionQueue([
      makeFinding({ id: 'resolved', status: 'resolved', investigationOutcome: undefined }),
      makeFinding({ id: 'active-failed', status: 'active', investigationOutcome: 'fix_failed' }),
    ]);
    expect(sorted.map((f) => f.id)).toEqual(['active-failed', 'resolved']);
  });

  it('uses recency as the final tiebreaker (newer lastSeenAt first) when outcome, severity, and criticality are equal', () => {
    const sorted = sortFindingsForAttentionQueue([
      makeFinding({ id: 'older', lastSeenAt: '2026-06-30T08:00:00Z' }),
      makeFinding({ id: 'newer', lastSeenAt: '2026-06-30T09:00:00Z' }),
    ]);
    expect(sorted.map((f) => f.id)).toEqual(['newer', 'older']);
  });

  it('does not mutate the input array', () => {
    const input = [
      makeFinding({ id: 'b', investigationOutcome: 'fix_queued' }),
      makeFinding({ id: 'a', investigationOutcome: 'fix_failed' }),
    ];
    const inputIdsBefore = input.map((f) => f.id);
    sortFindingsForAttentionQueue(input);
    expect(input.map((f) => f.id)).toEqual(inputIdsBefore);
  });

  it('returns an empty array for an empty input', () => {
    expect(sortFindingsForAttentionQueue([])).toEqual([]);
  });

  it('uses detectedAt for recency of a resolved finding (not lastSeenAt)', () => {
    const sorted = sortFindingsForAttentionQueue([
      makeFinding({
        id: 'recent-detected',
        status: 'resolved',
        detectedAt: '2026-06-30T10:00:00Z',
        lastSeenAt: '2026-06-30T08:00:00Z',
      }),
      makeFinding({
        id: 'older-detected',
        status: 'resolved',
        detectedAt: '2026-06-30T09:00:00Z',
        lastSeenAt: '2026-06-30T12:00:00Z',
      }),
    ]);
    // Both resolved, same outcome weight (3), same severity/criticality/runtime.
    // Recency uses detectedAt for non-active: 10:00 > 09:00 → recent-detected first.
    expect(sorted.map((f) => f.id)).toEqual(['recent-detected', 'older-detected']);
  });
});
