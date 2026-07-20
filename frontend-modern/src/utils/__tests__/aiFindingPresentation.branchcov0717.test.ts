import { describe, expect, it } from 'vitest';

import type { InvestigationOutcome, InvestigationStatus } from '@/api/patrol';
import type { UnifiedFinding } from '@/stores/aiIntelligence';
import {
  buildFindingFilterOptions,
  buildPatrolFindingDisplayGroups,
  doesFindingNeedAttention,
  formatFindingLifecycleType,
  formatFindingLoopState,
  formatOperatorStateDismissCauseLabel,
  getFindingActiveRuntimeSortOrder,
  getFindingEmptyStateCopy,
  getFindingEvidencePresentation,
  getFindingLoopStateBadgeClasses,
  getFindingLoopStateBadgeTone,
  getFindingManualControlsPresentation,
  getFindingPatrolWorkflowPresentation,
  getFindingPrimaryActionPresentation,
  getFindingSeverityCompactLabel,
  getFindingSourceBadgeTone,
  getFindingStatusBadgeClasses,
  getFindingStatusBadgeTone,
  getFindingStatusLabel,
  getFindingSubjectPresentation,
  getInvestigationOutcomeBadgeClasses,
  getInvestigationOutcomeBadgeTone,
  getInvestigationOutcomeLabel,
  getInvestigationStatusBadgeClasses,
  getInvestigationStatusBadgeTone,
  getInvestigationStatusLabel,
  getPatrolFindingClassification,
  getPatrolFindingIssueCountLabel,
  hasFindingInvestigationDetails,
  isPatrolInvestigationFixApproval,
  sortFindingsForAttentionQueue,
} from '@/utils/aiFindingPresentation';

// `formatBoundedFindingTime` and `isFailedFixOutcome` are module-private helpers,
// exercised here indirectly through the exported entry points that call them.

// Shared concrete Tailwind class strings, copied verbatim from the module's
// lookup tables so each assertion pins the exact rendered value.
const DEFAULT_BADGE_CLASSES = 'border-border bg-surface-alt text-muted';
const DEFAULT_LOOP_STATE_CLASSES = 'border-border bg-surface-alt text-muted';
const SKY_RUNTIME_CLASSES =
  'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-800 dark:bg-sky-900 dark:text-sky-300';
const BLUE_CLASSES =
  'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300';
const GREEN_CLASSES =
  'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900 dark:text-green-300';
const INDIGO_CLASSES =
  'border-indigo-200 bg-indigo-50 text-indigo-700 dark:border-indigo-800 dark:bg-indigo-900 dark:text-indigo-300';
const AMBER_CLASSES =
  'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300';
const RED_CLASSES =
  'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900 dark:text-red-300';
const EMERALD_CLASSES =
  'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-900 dark:text-emerald-300';

// A finding whose resourceId marks it as a Patrol runtime finding.
const RUNTIME_SUBJECT = { resourceId: 'ai-service', resourceName: 'other', title: 'other' };
// A finding that is plainly infrastructure (no runtime markers).
const INFRA_SUBJECT = {
  resourceId: 'host-1',
  resourceName: 'db-1',
  title: 'disk full',
};

// Minimal builder for the Pick shape buildPatrolFindingDisplayGroups requires.
type DisplayFindingInput = Pick<
  UnifiedFinding,
  'correlatedFindingIds' | 'id' | 'node' | 'resourceId' | 'resourceName' | 'resourceType' | 'title'
>;
const displayFinding = (
  overrides: Partial<DisplayFindingInput> & Pick<DisplayFindingInput, 'id'>,
): DisplayFindingInput => ({
  id: overrides.id,
  resourceId: overrides.resourceId ?? '',
  resourceName: overrides.resourceName ?? '',
  resourceType: overrides.resourceType ?? '',
  title: overrides.title ?? '',
  node: overrides.node,
  correlatedFindingIds: overrides.correlatedFindingIds,
});

// Minimal builder for the Pick shape getFindingSubjectPresentation requires.
type SubjectFinding = Pick<
  UnifiedFinding,
  'resourceId' | 'resourceName' | 'resourceType' | 'title'
>;
const subjectFinding = (overrides: Partial<SubjectFinding>): SubjectFinding => ({
  resourceId: '',
  resourceName: '',
  resourceType: '',
  title: '',
  ...overrides,
});

describe('aiFindingPresentation branch coverage (supplemental)', () => {
  describe('getFindingSourceBadgeTone', () => {
    it('maps every canonical source to its tone', () => {
      expect(getFindingSourceBadgeTone('threshold')).toBe('orange');
      expect(getFindingSourceBadgeTone('ai-patrol')).toBe('info');
      expect(getFindingSourceBadgeTone('anomaly')).toBe('info');
      expect(getFindingSourceBadgeTone('ai-chat')).toBe('teal');
      expect(getFindingSourceBadgeTone('correlation')).toBe('sky');
      expect(getFindingSourceBadgeTone('forecast')).toBe('success');
    });

    it('falls back to the ai-patrol tone for unknown and empty sources (|| arm)', () => {
      expect(getFindingSourceBadgeTone('mystery')).toBe('info');
      expect(getFindingSourceBadgeTone('')).toBe('info');
    });
  });

  describe('getFindingStatusBadgeClasses', () => {
    it('returns the colored class for each known status and the default for others', () => {
      expect(getFindingStatusBadgeClasses('resolved')).toBe(GREEN_CLASSES);
      expect(getFindingStatusBadgeClasses('snoozed')).toBe(BLUE_CLASSES);
      expect(getFindingStatusBadgeClasses('dismissed')).toBe(DEFAULT_BADGE_CLASSES);
      expect(getFindingStatusBadgeClasses('active')).toBe(DEFAULT_BADGE_CLASSES);
      expect(getFindingStatusBadgeClasses('')).toBe(DEFAULT_BADGE_CLASSES);
    });
  });

  describe('getFindingStatusBadgeTone', () => {
    it('maps known statuses to tones and falls back to muted', () => {
      expect(getFindingStatusBadgeTone('resolved')).toBe('success');
      expect(getFindingStatusBadgeTone('snoozed')).toBe('info');
      expect(getFindingStatusBadgeTone('dismissed')).toBe('muted');
      expect(getFindingStatusBadgeTone('active')).toBe('muted');
    });
  });

  describe('getFindingStatusLabel', () => {
    it('maps known statuses to labels and falls back to Dismissed', () => {
      expect(getFindingStatusLabel('resolved')).toBe('Resolved');
      expect(getFindingStatusLabel('snoozed')).toBe('Snoozed');
      expect(getFindingStatusLabel('dismissed')).toBe('Dismissed');
      // default (DEFAULT_FINDING_STATUS_LABEL) is also 'Dismissed'
      expect(getFindingStatusLabel('active')).toBe('Dismissed');
    });
  });

  describe('getFindingSeverityCompactLabel', () => {
    it('returns the compact token for each known severity', () => {
      expect(getFindingSeverityCompactLabel('critical')).toBe('CRIT');
      expect(getFindingSeverityCompactLabel('warning')).toBe('WARN');
      expect(getFindingSeverityCompactLabel('watch')).toBe('WATCH');
      expect(getFindingSeverityCompactLabel('info')).toBe('INFO');
    });

    it('uppercases an unknown severity via String(severity).toUpperCase() (|| arm)', () => {
      expect(getFindingSeverityCompactLabel('degraded')).toBe('DEGRADED');
      expect(getFindingSeverityCompactLabel('')).toBe('');
    });
  });

  describe('getPatrolFindingClassification', () => {
    it('classifies a Patrol runtime finding with the sky runtime badge', () => {
      expect(getPatrolFindingClassification(RUNTIME_SUBJECT)).toStrictEqual({
        kind: 'runtime',
        label: 'Patrol runtime',
        badgeClasses: SKY_RUNTIME_CLASSES,
      });
    });

    it('classifies an infrastructure finding with the default badge (else arm)', () => {
      expect(getPatrolFindingClassification(INFRA_SUBJECT)).toStrictEqual({
        kind: 'infrastructure',
        label: 'Infrastructure',
        badgeClasses: DEFAULT_BADGE_CLASSES,
      });
    });
  });

  describe('getPatrolFindingIssueCountLabel', () => {
    it('singularises exactly one issue', () => {
      expect(getPatrolFindingIssueCountLabel(1)).toBe('1 issue');
    });

    it('pluralises zero and many', () => {
      expect(getPatrolFindingIssueCountLabel(0)).toBe('0 issues');
      expect(getPatrolFindingIssueCountLabel(2)).toBe('2 issues');
      expect(getPatrolFindingIssueCountLabel(7)).toBe('7 issues');
    });

    it('truncates fractional counts toward zero', () => {
      expect(getPatrolFindingIssueCountLabel(3.9)).toBe('3 issues');
    });

    it('clamps negative counts to zero', () => {
      expect(getPatrolFindingIssueCountLabel(-5)).toBe('0 issues');
    });

    it('treats non-finite counts (NaN/Infinity) as zero', () => {
      expect(getPatrolFindingIssueCountLabel(Number.NaN)).toBe('0 issues');
      expect(getPatrolFindingIssueCountLabel(Number.POSITIVE_INFINITY)).toBe('0 issues');
    });
  });

  describe('getFindingPrimaryActionPresentation', () => {
    it('routes Patrol runtime findings to the provider-settings action', () => {
      expect(getFindingPrimaryActionPresentation(RUNTIME_SUBJECT)).toStrictEqual({
        label: 'Check Patrol model',
        href: '/settings/pulse-intelligence/patrol',
      });
    });

    it('returns undefined for infrastructure findings (else arm)', () => {
      expect(getFindingPrimaryActionPresentation(INFRA_SUBJECT)).toBeUndefined();
    });
  });

  describe('getFindingManualControlsPresentation', () => {
    it('disables all manual controls for Patrol runtime findings', () => {
      expect(getFindingManualControlsPresentation(RUNTIME_SUBJECT)).toStrictEqual({
        acknowledge: false,
        snooze: false,
        dismiss: false,
      });
    });

    it('enables all manual controls for infrastructure findings (else arm)', () => {
      expect(getFindingManualControlsPresentation(INFRA_SUBJECT)).toStrictEqual({
        acknowledge: true,
        snooze: true,
        dismiss: true,
      });
    });
  });

  describe('getInvestigationStatusBadgeClasses', () => {
    it('returns the mapped class for each canonical status', () => {
      expect(getInvestigationStatusBadgeClasses('pending')).toBe(DEFAULT_BADGE_CLASSES);
      expect(getInvestigationStatusBadgeClasses('running')).toBe(BLUE_CLASSES);
      expect(getInvestigationStatusBadgeClasses('completed')).toBe(GREEN_CLASSES);
      expect(getInvestigationStatusBadgeClasses('failed')).toBe(RED_CLASSES);
      expect(getInvestigationStatusBadgeClasses('needs_attention')).toBe(AMBER_CLASSES);
    });

    it('falls back to the default badge for an unrecognized status', () => {
      expect(
        getInvestigationStatusBadgeClasses('frobnicated' as unknown as InvestigationStatus),
      ).toBe(DEFAULT_BADGE_CLASSES);
    });
  });

  describe('getInvestigationStatusBadgeTone', () => {
    it('maps every canonical status to its tone and unknown to muted', () => {
      expect(getInvestigationStatusBadgeTone('pending')).toBe('muted');
      expect(getInvestigationStatusBadgeTone('running')).toBe('info');
      expect(getInvestigationStatusBadgeTone('completed')).toBe('success');
      expect(getInvestigationStatusBadgeTone('failed')).toBe('danger');
      expect(getInvestigationStatusBadgeTone('needs_attention')).toBe('warning');
      expect(getInvestigationStatusBadgeTone('unknown')).toBe('muted');
    });
  });

  describe('getInvestigationStatusLabel', () => {
    it('maps every canonical status to its label and unknown to the raw value', () => {
      expect(getInvestigationStatusLabel('pending')).toBe('Pending');
      expect(getInvestigationStatusLabel('running')).toBe('Running');
      expect(getInvestigationStatusLabel('completed')).toBe('Completed');
      expect(getInvestigationStatusLabel('failed')).toBe('Failed');
      expect(getInvestigationStatusLabel('needs_attention')).toBe('Needs Attention');
      expect(getInvestigationStatusLabel('unknown')).toBe('unknown');
    });
  });

  describe('getInvestigationOutcomeBadgeClasses', () => {
    const expected: Record<InvestigationOutcome, string> = {
      resolved: GREEN_CLASSES,
      fix_queued: BLUE_CLASSES,
      fix_executed: EMERALD_CLASSES,
      fix_failed: RED_CLASSES,
      fix_rejected: AMBER_CLASSES,
      needs_attention: AMBER_CLASSES,
      cannot_fix: DEFAULT_BADGE_CLASSES,
      timed_out: 'border-border bg-surface-alt text-base-content',
      fix_verified: GREEN_CLASSES,
      fix_verification_failed: RED_CLASSES,
      fix_verification_unknown: AMBER_CLASSES,
    };
    it.each(Object.entries(expected))('maps %s to its badge class', (_outcome, classes) => {
      // Cast back through the union type; runtime resolves via Record lookup.
      expect(getInvestigationOutcomeBadgeClasses(_outcome as InvestigationOutcome)).toBe(classes);
    });

    it('falls back to the default badge for an unrecognized outcome', () => {
      expect(getInvestigationOutcomeBadgeClasses('mystery')).toBe(DEFAULT_BADGE_CLASSES);
    });
  });

  describe('getInvestigationOutcomeBadgeTone', () => {
    const expected: Record<InvestigationOutcome, string> = {
      resolved: 'success',
      fix_queued: 'info',
      fix_executed: 'success',
      fix_failed: 'danger',
      fix_rejected: 'warning',
      needs_attention: 'warning',
      cannot_fix: 'muted',
      timed_out: 'neutral',
      fix_verified: 'success',
      fix_verification_failed: 'danger',
      fix_verification_unknown: 'warning',
    };
    it.each(Object.entries(expected))('maps %s to its tone', (outcome, tone) => {
      expect(getInvestigationOutcomeBadgeTone(outcome as InvestigationOutcome)).toBe(tone);
    });

    it('falls back to the muted tone for an unrecognized outcome', () => {
      expect(getInvestigationOutcomeBadgeTone('mystery')).toBe('muted');
    });
  });

  describe('getInvestigationOutcomeLabel', () => {
    const expected: Record<InvestigationOutcome, string> = {
      resolved: 'Resolved',
      fix_queued: 'Fix queued',
      fix_executed: 'Fix applied',
      fix_failed: 'Fix failed',
      fix_rejected: 'Fix rejected',
      needs_attention: 'Needs attention',
      cannot_fix: 'Cannot remediate',
      timed_out: 'Timed out',
      fix_verified: 'Fix verified',
      fix_verification_failed: 'Verification failed',
      fix_verification_unknown: 'Verification unknown',
    };
    it.each(Object.entries(expected))('maps %s to its label', (outcome, label) => {
      expect(getInvestigationOutcomeLabel(outcome as InvestigationOutcome)).toBe(label);
    });

    it('falls back to the raw string for an unrecognized outcome', () => {
      expect(getInvestigationOutcomeLabel('mystery')).toBe('mystery');
    });
  });

  describe('hasFindingInvestigationDetails', () => {
    it('returns false when no investigation field is set', () => {
      expect(hasFindingInvestigationDetails({})).toBe(false);
      expect(
        hasFindingInvestigationDetails({
          investigationSessionId: '   ',
          investigationAttempts: 0,
        }),
      ).toBe(false);
    });

    it('returns true when a non-empty investigation session id is present', () => {
      expect(hasFindingInvestigationDetails({ investigationSessionId: 'sess-1' })).toBe(true);
    });

    it('returns true when an investigation status is present', () => {
      expect(hasFindingInvestigationDetails({ investigationStatus: 'running' })).toBe(true);
    });

    it('returns true when an investigation outcome is present', () => {
      expect(hasFindingInvestigationDetails({ investigationOutcome: 'fix_queued' })).toBe(true);
    });

    it('returns true when investigation attempts is greater than zero', () => {
      expect(hasFindingInvestigationDetails({ investigationAttempts: 2 })).toBe(true);
    });

    it('treats undefined attempts as zero via the ?? coalescer', () => {
      expect(hasFindingInvestigationDetails({ investigationAttempts: undefined })).toBe(false);
    });
  });

  describe('isPatrolInvestigationFixApproval', () => {
    it('returns true only for the investigation_fix tool id', () => {
      expect(isPatrolInvestigationFixApproval({ toolId: 'investigation_fix' })).toBe(true);
      expect(isPatrolInvestigationFixApproval({ toolId: 'something_else' })).toBe(false);
    });
  });

  describe('getFindingLoopStateBadgeClasses', () => {
    it('returns the mapped class for each canonical loop state', () => {
      expect(getFindingLoopStateBadgeClasses('detected')).toBe(BLUE_CLASSES);
      expect(getFindingLoopStateBadgeClasses('investigating')).toBe(INDIGO_CLASSES);
      expect(getFindingLoopStateBadgeClasses('remediation_planned')).toBe(AMBER_CLASSES);
      expect(getFindingLoopStateBadgeClasses('remediating')).toBe(AMBER_CLASSES);
      expect(getFindingLoopStateBadgeClasses('remediation_failed')).toBe(RED_CLASSES);
      expect(getFindingLoopStateBadgeClasses('needs_attention')).toBe(AMBER_CLASSES);
      expect(getFindingLoopStateBadgeClasses('timed_out')).toBe(
        'border-border bg-surface-alt text-base-content',
      );
      expect(getFindingLoopStateBadgeClasses('resolved')).toBe(GREEN_CLASSES);
      expect(getFindingLoopStateBadgeClasses('dismissed')).toBe(DEFAULT_LOOP_STATE_CLASSES);
      expect(getFindingLoopStateBadgeClasses('snoozed')).toBe(BLUE_CLASSES);
      expect(getFindingLoopStateBadgeClasses('suppressed')).toBe(DEFAULT_LOOP_STATE_CLASSES);
    });

    it('falls back to the default loop-state class for an unrecognized state', () => {
      expect(getFindingLoopStateBadgeClasses('mystery')).toBe(DEFAULT_LOOP_STATE_CLASSES);
    });
  });

  describe('getFindingLoopStateBadgeTone', () => {
    const expected: Record<string, string> = {
      detected: 'info',
      investigating: 'indigo',
      remediation_planned: 'warning',
      remediating: 'warning',
      remediation_failed: 'danger',
      needs_attention: 'warning',
      timed_out: 'neutral',
      resolved: 'success',
      dismissed: 'muted',
      snoozed: 'info',
      suppressed: 'muted',
    };
    it.each(Object.entries(expected))('maps %s to its tone', (state, tone) => {
      expect(getFindingLoopStateBadgeTone(state)).toBe(tone);
    });

    it('falls back to the muted tone for an unrecognized state', () => {
      expect(getFindingLoopStateBadgeTone('mystery')).toBe('muted');
    });
  });

  describe('formatFindingLoopState', () => {
    it('replaces underscores with spaces via formatIdentifierLabel', () => {
      expect(formatFindingLoopState('remediation_planned')).toBe('remediation planned');
      expect(formatFindingLoopState('detected')).toBe('detected');
      expect(formatFindingLoopState('')).toBe('');
    });
  });

  describe('formatFindingLifecycleType', () => {
    it('maps known lifecycle types to their human labels', () => {
      expect(formatFindingLifecycleType('detected')).toBe('Detected');
      expect(formatFindingLifecycleType('auto_resolved')).toBe('Auto-resolved');
      expect(formatFindingLifecycleType('verification_passed')).toBe('Fix verified');
      expect(formatFindingLifecycleType('loop_transition_violation')).toBe(
        'Invalid transition blocked',
      );
    });

    it('falls back to formatIdentifierLabel for an unknown type (|| arm)', () => {
      expect(formatFindingLifecycleType('custom_thing')).toBe('custom thing');
      expect(formatFindingLifecycleType('')).toBe('');
    });
  });

  describe('buildFindingFilterOptions', () => {
    it('returns only the three base options when neither count is positive', () => {
      const options = buildFindingFilterOptions({
        needsAttentionCount: 0,
        pendingApprovalCount: 0,
      });
      expect(options.map((o) => o.value)).toStrictEqual(['active', 'all', 'resolved']);
    });

    it('appends the attention option when needsAttentionCount is positive', () => {
      const options = buildFindingFilterOptions({
        needsAttentionCount: 3,
        pendingApprovalCount: 0,
      });
      expect(options).toHaveLength(4);
      expect(options[3]).toStrictEqual({
        value: 'attention',
        label: 'Needs Attention',
        tone: 'warning',
        count: 3,
      });
    });

    it('appends the approvals option when pendingApprovalCount is positive', () => {
      const options = buildFindingFilterOptions({
        needsAttentionCount: 0,
        pendingApprovalCount: 2,
      });
      expect(options).toHaveLength(4);
      expect(options[3]).toStrictEqual({
        value: 'approvals',
        label: 'Approvals',
        tone: 'warning',
        count: 2,
      });
    });

    it('appends both conditional options when both counts are positive', () => {
      const options = buildFindingFilterOptions({
        needsAttentionCount: 1,
        pendingApprovalCount: 4,
      });
      expect(options.map((o) => o.value)).toStrictEqual([
        'active',
        'all',
        'resolved',
        'attention',
        'approvals',
      ]);
    });
  });

  describe('getFindingEmptyStateCopy', () => {
    it('returns the active copy with a body', () => {
      expect(getFindingEmptyStateCopy('active')).toStrictEqual({
        title: 'No active findings',
        body: 'Your infrastructure looks healthy!',
      });
    });

    it('returns title-only copy for the attention filter', () => {
      expect(getFindingEmptyStateCopy('attention')).toStrictEqual({
        title: 'No findings need attention right now.',
      });
    });

    it('returns title-only copy for the approvals filter', () => {
      expect(getFindingEmptyStateCopy('approvals')).toStrictEqual({
        title: 'No pending approvals.',
      });
    });

    it('falls through to the default copy for all/resolved (default arm)', () => {
      expect(getFindingEmptyStateCopy('all')).toStrictEqual({
        title: 'No Patrol findings to display',
      });
      expect(getFindingEmptyStateCopy('resolved')).toStrictEqual({
        title: 'No Patrol findings to display',
      });
    });
  });

  describe('formatOperatorStateDismissCauseLabel', () => {
    it('maps the canonical causes to short labels', () => {
      expect(formatOperatorStateDismissCauseLabel('maintenance_window')).toBe('maintenance');
      expect(formatOperatorStateDismissCauseLabel('intentionally_offline')).toBe(
        'intentionally offline',
      );
    });

    it('returns empty string for unknown and empty causes (default arm)', () => {
      expect(formatOperatorStateDismissCauseLabel('mystery')).toBe('');
      expect(formatOperatorStateDismissCauseLabel('')).toBe('');
    });
  });

  describe('getFindingActiveRuntimeSortOrder (branch arm: active + runtime)', () => {
    it('returns 0 only for an active Patrol runtime finding', () => {
      expect(
        getFindingActiveRuntimeSortOrder({
          status: 'active',
          resourceId: 'ai-service',
          resourceName: 'other',
          title: 'other',
        }),
      ).toBe(0);
    });

    it('returns 1 for an active non-runtime finding', () => {
      expect(
        getFindingActiveRuntimeSortOrder({
          status: 'active',
          resourceId: 'host-1',
          resourceName: 'db-1',
          title: 'disk full',
        }),
      ).toBe(1);
    });

    it('returns 1 for a non-active finding even when it is runtime', () => {
      expect(
        getFindingActiveRuntimeSortOrder({
          status: 'resolved',
          resourceId: 'ai-service',
          resourceName: 'other',
          title: 'other',
        }),
      ).toBe(1);
    });
  });

  describe('getFindingSubjectPresentation (runtime + resourceName fallback arms)', () => {
    it('short-circuits to "Patrol runtime" for a runtime finding', () => {
      expect(getFindingSubjectPresentation(subjectFinding(RUNTIME_SUBJECT))).toStrictEqual({
        label: 'Patrol runtime',
      });
    });

    it('combines name and formatted type when both are present', () => {
      expect(
        getFindingSubjectPresentation(
          subjectFinding({
            resourceId: 'host-1',
            resourceName: 'db-1',
            resourceType: 'qemu_guest',
            title: 'disk full',
          }),
        ),
      ).toStrictEqual({ label: 'db-1 (qemu guest)' });
    });

    it('uses just the resource name when resourceType is absent (!resourceType arm)', () => {
      expect(
        getFindingSubjectPresentation(
          subjectFinding({ resourceId: 'host-1', resourceName: 'db-1', title: 'x' }),
        ),
      ).toStrictEqual({ label: 'db-1' });
    });

    it('falls back to resourceId when resourceName is blank (|| right operand)', () => {
      expect(
        getFindingSubjectPresentation(
          subjectFinding({ resourceId: 'host-1', resourceName: '   ', title: 'x' }),
        ),
      ).toStrictEqual({ label: 'host-1' });
    });

    it('takes the resourceId || "" right arm when resourceName is empty and resourceId is set', () => {
      // resourceName is falsy (""), so `resourceName || ""` short-circuits to "".
      expect(
        getFindingSubjectPresentation(
          subjectFinding({ resourceId: 'host-1', resourceName: '', title: 'x' }),
        ),
      ).toStrictEqual({ label: 'host-1' });
    });

    it('returns an empty label when neither resourceName nor resourceId is set', () => {
      // Both `|| ""` right arms and the outer `||` resolve to "".
      expect(getFindingSubjectPresentation(subjectFinding({ title: 'x' }))).toStrictEqual({
        label: '',
      });
    });
  });

  describe('getFindingEvidencePresentation (formatBoundedFindingTime + match arms)', () => {
    it('renders the plural host-update observation with reboot authorized boundary', () => {
      const out = getFindingEvidencePresentation({
        key: 'apt-host-updates',
        evidence:
          'pending_updates=2 inventory=host-1 checked_at=2026-07-17T10:00:00Z received_at=2026-07-17T10:00:05Z reboot_required=true',
      });
      expect(out).toContain('2 operating system updates were pending');
      expect(out).toContain(
        'reboot required: Yes. No reboot is authorized by this finding or action',
      );
      expect(out).not.toContain('Pulse could not safely present');
    });

    it('renders the singular host-update observation with reboot not required', () => {
      const out = getFindingEvidencePresentation({
        key: 'apt-host-updates',
        evidence:
          'pending_updates=1 inventory=host-1 checked_at=2026-07-17T10:00:00Z received_at=2026-07-17T10:00:05Z reboot_required=false',
      });
      expect(out).toContain('1 operating system update was pending');
      expect(out).toContain('The host already reported reboot required: No.');
    });

    it('emits the safety fallback when a matched date does not parse (NaN arm of formatBoundedFindingTime)', () => {
      // Month 13 passes the shape regex but yields an Invalid Date -> NaN -> undefined.
      const out = getFindingEvidencePresentation({
        key: 'apt-host-updates',
        evidence:
          'pending_updates=2 inventory=host-1 checked_at=2026-13-01T00:00:00Z received_at=2026-07-17T10:00:00Z reboot_required=false',
      });
      expect(out).toBe(
        'Pulse could not safely present the bounded host-update observation. Refresh before acting.',
      );
    });

    it('renders the package-cleanup observation with formatted bytes and usage (valid arm)', () => {
      const out = getFindingEvidencePresentation({
        key: 'apt-package-cache-pressure',
        evidence:
          'reclaimable_bytes=1048576 filesystem_usage=75.5 fingerprint=abc checked_at=2026-07-17T10:00:00Z received_at=2026-07-17T10:00:05Z',
      });
      expect(out).toContain('1.00 MB of downloaded package data was reclaimable');
      expect(out).toContain('75.5% full');
      expect(out).not.toContain('Pulse could not safely present');
    });

    it('passes trimmed evidence through verbatim when the key is not a bounded format', () => {
      expect(
        getFindingEvidencePresentation({
          key: 'something-else',
          evidence: '  raw evidence body  ',
        }),
      ).toBe('raw evidence body');
    });

    it('returns an empty string when evidence is absent and the key is not bounded', () => {
      expect(getFindingEvidencePresentation({ key: 'something-else' })).toBe('');
    });
  });

  describe('getFindingPatrolWorkflowPresentation (typed action state arms)', () => {
    type WorkflowFinding = Parameters<typeof getFindingPatrolWorkflowPresentation>[0];
    const base = {
      id: 'wf-1',
      source: 'ai-patrol' as const,
      status: 'active' as const,
      resourceId: 'host-1',
      resourceName: 'host-1',
      title: 'disk full',
      investigationStatus: undefined,
      investigationOutcome: undefined,
      loopState: undefined,
    };

    it('returns the pending_approval approval stage', () => {
      expect(
        getFindingPatrolWorkflowPresentation(
          {
            ...base,
            investigationRecord: { action: { state: 'pending_approval' } },
          } as unknown as WorkflowFinding,
          [],
        ),
      ).toStrictEqual({
        stage: 'approval',
        label: 'Approve or reject',
        detail: 'A typed governed action is waiting for an operator decision.',
        tone: 'warning',
      });
    });

    it('returns the planned approval stage', () => {
      expect(
        getFindingPatrolWorkflowPresentation(
          {
            ...base,
            investigationRecord: { action: { state: 'planned' } },
          } as unknown as WorkflowFinding,
          [],
        ),
      ).toStrictEqual({
        stage: 'approval',
        label: 'Run action',
        detail: 'The typed action plan is ready to run under its declared policy.',
        tone: 'info',
      });
    });

    it('returns the approved approval stage', () => {
      expect(
        getFindingPatrolWorkflowPresentation(
          {
            ...base,
            investigationRecord: { action: { state: 'approved' } },
          } as unknown as WorkflowFinding,
          [],
        ),
      ).toStrictEqual({
        stage: 'approval',
        label: 'Run approved action',
        detail: 'The typed action is approved; execution remains a separate operator step.',
        tone: 'success',
      });
    });

    it('returns the executing verification stage', () => {
      expect(
        getFindingPatrolWorkflowPresentation(
          {
            ...base,
            investigationRecord: { action: { state: 'executing' } },
          } as unknown as WorkflowFinding,
          [],
        ),
      ).toStrictEqual({
        stage: 'verification',
        label: 'Action running',
        detail: 'The governed action is running and will publish its verification result.',
        tone: 'info',
      });
    });
  });

  describe('doesFindingNeedAttention (fix_queued + investigationRecord?.action arm)', () => {
    type AttentionFinding = Parameters<typeof doesFindingNeedAttention>[0];
    const queuedBase = {
      id: 'f-1',
      status: 'active' as const,
      investigationOutcome: 'fix_queued' as const,
    };

    it('needs attention when fix is queued with no live approval and no action record', () => {
      expect(doesFindingNeedAttention({ ...queuedBase } as unknown as AttentionFinding, [])).toBe(
        true,
      );
    });

    it('needs attention when the record exists but carries no action (?. non-short-circuit arm)', () => {
      expect(
        doesFindingNeedAttention(
          { ...queuedBase, investigationRecord: {} } as unknown as AttentionFinding,
          [],
        ),
      ).toBe(true);
    });

    it('does not need attention once a typed action exists (!action false arm)', () => {
      expect(
        doesFindingNeedAttention(
          {
            ...queuedBase,
            investigationRecord: { action: { state: 'executing' } },
          } as unknown as AttentionFinding,
          [],
        ),
      ).toBe(false);
    });
  });

  describe('sortFindingsForAttentionQueue (runtime tie-break arm)', () => {
    it('ranks an active runtime finding above an active infrastructure finding when other keys tie', () => {
      const runtime = {
        id: 'runtime',
        source: 'ai-patrol',
        resourceId: 'ai-service',
        resourceName: 'other',
        resourceType: 'service',
        category: 'runtime',
        severity: 'critical',
        title: 'other',
        description: '',
        detectedAt: '2026-07-17T00:00:00Z',
        status: 'active',
        resourceCriticality: 'high',
      };
      const infra = {
        id: 'infra',
        source: 'ai-patrol',
        resourceId: 'host-1',
        resourceName: 'db-1',
        resourceType: 'vm',
        category: 'infra',
        severity: 'critical',
        title: 'disk full',
        description: '',
        detectedAt: '2026-07-17T00:00:00Z',
        status: 'active',
        resourceCriticality: 'high',
      };
      const sorted = sortFindingsForAttentionQueue([
        infra as unknown as UnifiedFinding,
        runtime as unknown as UnifiedFinding,
      ]);
      expect(sorted.map((f) => f.id)).toStrictEqual(['runtime', 'infra']);
    });
  });

  describe('buildPatrolFindingDisplayGroups', () => {
    it('returns an empty array for no findings', () => {
      expect(buildPatrolFindingDisplayGroups([])).toStrictEqual([]);
    });

    it('produces a single-finding group of kind "finding" with the subject label', () => {
      const f1 = displayFinding({
        id: 'f1',
        resourceId: 'r1',
        resourceName: 'Host One',
        resourceType: 'qemu_guest',
        title: 'CPU high',
      });
      const groups = buildPatrolFindingDisplayGroups([f1]);
      expect(groups).toHaveLength(1);
      expect(groups[0]).toMatchObject({
        id: 'group:f1',
        kind: 'finding',
        label: 'Host One (qemu guest)',
        affectedResourceCount: 1,
      });
      expect(groups[0].relatedFindings).toStrictEqual([]);
      expect(groups[0].findings).toStrictEqual([f1]);
      expect(groups[0].primaryFinding).toBe(f1);
    });

    it('groups findings sharing a resource into a "resource" group', () => {
      const f1 = displayFinding({
        id: 'f1',
        resourceId: 'r1',
        resourceName: 'Host One',
        resourceType: 'vm',
        title: 'CPU high',
      });
      const f2 = displayFinding({
        id: 'f2',
        resourceId: 'r1',
        resourceName: 'Host One',
        resourceType: 'vm',
        title: 'Mem high',
      });
      const groups = buildPatrolFindingDisplayGroups([f1, f2]);
      expect(groups).toHaveLength(1);
      expect(groups[0]).toMatchObject({
        id: 'group:f1:f2',
        kind: 'resource',
        label: 'Host One (vm)',
        affectedResourceCount: 1,
      });
      expect(groups[0].relatedFindings).toHaveLength(1);
      expect(groups[0].relatedFindings[0]).toBe(f2);
      expect(groups[0].findings).toHaveLength(2);
    });

    it('groups findings sharing a node into a "node" group with a title-cased label', () => {
      const groups = buildPatrolFindingDisplayGroups([
        displayFinding({
          id: 'f1',
          resourceId: 'r1',
          resourceName: 'A',
          resourceType: 'vm',
          title: 't1',
          node: 'node-1',
        }),
        displayFinding({
          id: 'f2',
          resourceId: 'r2',
          resourceName: 'B',
          resourceType: 'vm',
          title: 't2',
          node: 'node-1',
        }),
      ]);
      expect(groups).toHaveLength(1);
      expect(groups[0]).toMatchObject({
        kind: 'node',
        label: 'Node 1',
        affectedResourceCount: 2,
      });
    });

    it('groups explicitly correlated findings into a "correlated" group with the resource-count label', () => {
      const groups = buildPatrolFindingDisplayGroups([
        displayFinding({
          id: 'f1',
          resourceId: 'r1',
          resourceName: 'A',
          resourceType: 'vm',
          title: 't1',
          correlatedFindingIds: ['f2'],
        }),
        displayFinding({
          id: 'f2',
          resourceId: 'r2',
          resourceName: 'B',
          resourceType: 'vm',
          title: 't2',
        }),
      ]);
      expect(groups).toHaveLength(1);
      expect(groups[0]).toMatchObject({
        kind: 'correlated',
        label: '2 related resources',
        affectedResourceCount: 2,
      });
    });

    it('uses the singular "1 related resource" fallback when explicit correlation shares one resource', () => {
      // Explicit correlation wins over the size===1 "resource" kind, so a
      // single-resource correlated pair reaches the resource-count label and
      // hits the singular `resourceKeys.size === 1 ? "resource"` arm.
      const groups = buildPatrolFindingDisplayGroups([
        displayFinding({
          id: 'f1',
          resourceId: 'r1',
          resourceName: 'A',
          resourceType: 'vm',
          title: 't1',
          correlatedFindingIds: ['f2'],
        }),
        displayFinding({
          id: 'f2',
          resourceId: 'r1',
          resourceName: 'A',
          resourceType: 'vm',
          title: 't2',
          correlatedFindingIds: ['f1'],
        }),
      ]);
      expect(groups).toHaveLength(1);
      expect(groups[0]).toMatchObject({
        kind: 'correlated',
        label: '1 related resource',
        affectedResourceCount: 1,
      });
    });

    it('falls through to the final "correlated" kind via a transitive resource+node union', () => {
      // f1<->f2 share resource r1; f2<->f3 share node node-b. All three end up in
      // one component with two resources and two nodes and no explicit correlation,
      // hitting the final 'correlated' kind and the resource-count fallback label.
      const groups = buildPatrolFindingDisplayGroups([
        displayFinding({
          id: 'f1',
          resourceId: 'r1',
          resourceName: 'A',
          resourceType: 'vm',
          title: 't1',
          node: 'node-a',
        }),
        displayFinding({
          id: 'f2',
          resourceId: 'r1',
          resourceName: 'A',
          resourceType: 'vm',
          title: 't2',
          node: 'node-b',
        }),
        displayFinding({
          id: 'f3',
          resourceId: 'r2',
          resourceName: 'B',
          resourceType: 'vm',
          title: 't3',
          node: 'node-b',
        }),
      ]);
      expect(groups).toHaveLength(1);
      expect(groups[0]).toMatchObject({
        kind: 'correlated',
        label: '2 related resources',
        affectedResourceCount: 2,
      });
      expect(groups[0].findings).toHaveLength(3);
      expect(groups[0].relatedFindings).toHaveLength(2);
    });

    it('ignores correlated ids that reference findings not in the set (findingById.has false arm)', () => {
      const groups = buildPatrolFindingDisplayGroups([
        displayFinding({
          id: 'f1',
          resourceId: 'r1',
          resourceName: 'A',
          resourceType: 'vm',
          title: 't1',
          correlatedFindingIds: ['ghost'],
        }),
      ]);
      expect(groups).toHaveLength(1);
      expect(groups[0].kind).toBe('finding');
    });

    it('keeps disjoint resources as separate single-finding groups', () => {
      const groups = buildPatrolFindingDisplayGroups([
        displayFinding({
          id: 'f1',
          resourceId: 'r1',
          resourceName: 'A',
          resourceType: 'vm',
          title: 't1',
        }),
        displayFinding({
          id: 'f2',
          resourceId: 'r2',
          resourceName: 'B',
          resourceType: 'vm',
          title: 't2',
        }),
      ]);
      expect(groups).toHaveLength(2);
      expect(groups.map((g) => g.kind)).toStrictEqual(['finding', 'finding']);
    });
  });
});
