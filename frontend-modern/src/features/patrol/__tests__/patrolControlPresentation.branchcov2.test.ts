import { describe, expect, it } from 'vitest';

import type { PatrolRunStatus } from '@/api/patrol';

import {
  getMonitorContextPatrolProtectionPosture,
  getPatrolQueueWorkspaceDescription,
  getPatrolSetupIssueReason,
  getPatrolWorkspaceWorkGroups,
  isPatrolCoverageStale,
} from '../patrolControlPresentation';

// Five of the nine target functions (`normalizeStatus`, `getPatrolCoverageLabel`,
// `formatMonitorCoverageLabel`, `getPatrolQueueActionDetail`,
// `shouldSuppressMonitorContextPatrolPosture`) are module-private, so this file
// drives them through their exported callers and asserts on the concrete
// observable outputs that those private branches produce.

describe('patrolControlPresentation branch coverage (set 2)', () => {
  const NOW_MS = Date.parse('2026-06-30T15:00:00Z');

  describe('getMonitorContextPatrolProtectionPosture', () => {
    it('returns no summaries when neither a latest run nor patrol status exists', () => {
      // Branch: `(!input.latestRun && !patrolStatus)` early-return [].
      expect(getMonitorContextPatrolProtectionPosture({ monitoredResourceCount: 4 })).toEqual([]);
    });

    it('uses warning tones throughout when the latest Patrol status is unhealthy', () => {
      // Branch: `patrolStatus?.healthy === false` -> healthyTone 'warning'.
      expect(
        getMonitorContextPatrolProtectionPosture({
          findingCount: 0,
          latestRun: {
            error_count: 0,
            resources_checked: 2,
            status: 'healthy',
          },
          monitoredResourceCount: 3,
          nowMs: NOW_MS,
          patrolStatus: {
            enabled: true,
            error_count: 0,
            findings_count: 0,
            healthy: false,
            next_patrol_at: '2026-07-01T10:00:00Z',
            resources_checked: 2,
            running: false,
            runtime_state: 'active',
          },
          pendingApprovalCount: 0,
          workTypeComposition: {
            total: 0,
            approval: 0,
            failed: 0,
            inProgress: 0,
            recurring: 0,
            newIssues: 0,
          },
        }),
      ).toEqual([
        {
          detail: 'Latest Patrol evidence is available while you review this monitor view.',
          id: 'coverage',
          label: 'Patrol checked 2 resources',
          tone: 'warning',
        },
        {
          detail: 'Current Patrol findings and approvals stay in Patrol; none are waiting now.',
          id: 'open-work',
          label: 'No Patrol work waiting',
          tone: 'warning',
        },
        {
          detail: 'Patrol is scheduled to check monitored resources again.',
          id: 'schedule',
          label: 'Next check scheduled',
          tone: 'info',
        },
      ]);
    });

    it('surfaces a paused-schedule summary when scheduled checks are disabled', () => {
      // Branch: `patrolStatus?.enabled === false` schedule arm. next_patrol_at is
      // intentionally set to prove the enabled===false check wins.
      expect(
        getMonitorContextPatrolProtectionPosture({
          findingCount: 0,
          latestRun: {
            error_count: 0,
            resources_checked: 2,
            status: 'healthy',
          },
          monitoredResourceCount: 3,
          nowMs: NOW_MS,
          patrolStatus: {
            enabled: false,
            error_count: 0,
            findings_count: 0,
            healthy: true,
            next_patrol_at: '2026-07-01T10:00:00Z',
            resources_checked: 2,
            running: false,
            runtime_state: 'active',
          },
          pendingApprovalCount: 0,
          workTypeComposition: {
            total: 0,
            approval: 0,
            failed: 0,
            inProgress: 0,
            recurring: 0,
            newIssues: 0,
          },
        }),
      ).toEqual([
        {
          detail: 'Latest Patrol evidence is available while you review this monitor view.',
          id: 'coverage',
          label: 'Patrol checked 2 resources',
          tone: 'success',
        },
        {
          detail: 'Current Patrol findings and approvals stay in Patrol; none are waiting now.',
          id: 'open-work',
          label: 'No Patrol work waiting',
          tone: 'success',
        },
        {
          detail: 'Run Patrol manually or enable scheduled checks to keep coverage fresh.',
          id: 'schedule',
          label: 'Scheduled checks paused',
          tone: 'warning',
        },
      ]);
    });

    it('falls back to the ready-to-run schedule summary when no next check is set', () => {
      // Branch: schedule `else` arm (enabled !== false && no next_patrol_at).
      expect(
        getMonitorContextPatrolProtectionPosture({
          findingCount: 0,
          latestRun: null,
          monitoredResourceCount: 3,
          nowMs: NOW_MS,
          patrolStatus: {
            enabled: true,
            error_count: 0,
            findings_count: 0,
            healthy: true,
            next_patrol_at: undefined,
            resources_checked: 0,
            running: false,
            runtime_state: 'active',
          },
          pendingApprovalCount: 0,
          workTypeComposition: {
            total: 0,
            approval: 0,
            failed: 0,
            inProgress: 0,
            recurring: 0,
            newIssues: 0,
          },
        }),
      ).toEqual([
        {
          detail: 'Run Patrol to refresh current coverage for monitored resources.',
          id: 'coverage',
          label: 'Patrol coverage needs refresh',
          tone: 'warning',
        },
        {
          detail: 'Current Patrol findings and approvals stay in Patrol; none are waiting now.',
          id: 'open-work',
          label: 'No Patrol work waiting',
          tone: 'success',
        },
        {
          detail: 'Run Patrol from the Patrol page any time to refresh coverage.',
          id: 'schedule',
          label: 'Ready to run Patrol',
          tone: 'info',
        },
      ]);
    });
  });

  describe('shouldSuppressMonitorContextPatrolPosture (via getMonitorContextPatrolProtectionPosture)', () => {
    const baseMonitorInput = {
      findingCount: 0,
      latestRun: {
        error_count: 0,
        resources_checked: 4,
        status: 'healthy' as const,
      },
      monitoredResourceCount: 4,
      nowMs: NOW_MS,
      patrolStatus: {
        enabled: true,
        error_count: 0,
        findings_count: 0,
        healthy: true,
        next_patrol_at: '2026-07-01T10:00:00Z',
        resources_checked: 4,
        running: false,
        runtime_state: 'active' as const,
      },
      pendingApprovalCount: 0,
      workTypeComposition: {
        total: 0,
        approval: 0,
        failed: 0,
        inProgress: 0,
        recurring: 0,
        newIssues: 0,
      },
    };

    it('does not suppress when every work, runtime, and schedule signal is clear', () => {
      // Baseline: every OR operand in shouldSuppress... evaluates false, so the
      // monitor-context posture is returned in full.
      expect(getMonitorContextPatrolProtectionPosture(baseMonitorInput)).toHaveLength(3);
    });

    it.each([
      ['failed work-type composition', { failed: 1 }],
      ['approval work-type composition', { approval: 1 }],
      ['in-progress work-type composition', { inProgress: 1 }],
      ['recurring work-type composition', { recurring: 1 }],
    ] as const)('suppresses the posture when %s is present', (_label, compositionOverride) => {
      expect(
        getMonitorContextPatrolProtectionPosture({
          ...baseMonitorInput,
          workTypeComposition: {
            total: 1,
            approval: 0,
            failed: 0,
            inProgress: 0,
            recurring: 0,
            newIssues: 0,
            ...compositionOverride,
          },
        }),
      ).toEqual([]);
    });

    it('suppresses the posture when the status reports open findings', () => {
      // OR operand: `statusFindingCount > 0`.
      expect(
        getMonitorContextPatrolProtectionPosture({
          ...baseMonitorInput,
          patrolStatus: { ...baseMonitorInput.patrolStatus, findings_count: 1 },
        }),
      ).toEqual([]);
    });

    it('suppresses the posture while a Patrol run is in flight', () => {
      // OR operand: `patrolStatus?.running`.
      expect(
        getMonitorContextPatrolProtectionPosture({
          ...baseMonitorInput,
          patrolStatus: { ...baseMonitorInput.patrolStatus, running: true },
        }),
      ).toEqual([]);
    });

    it('suppresses the posture when the status carries runtime errors', () => {
      // OR operand: `statusErrorCount > 0` (latest run stays healthy).
      expect(
        getMonitorContextPatrolProtectionPosture({
          ...baseMonitorInput,
          patrolStatus: { ...baseMonitorInput.patrolStatus, error_count: 1 },
        }),
      ).toEqual([]);
    });

    it('suppresses the posture when the runtime is not active', () => {
      // OR operand: `!isActivePatrolRuntime(patrolStatus)`.
      expect(
        getMonitorContextPatrolProtectionPosture({
          ...baseMonitorInput,
          patrolStatus: { ...baseMonitorInput.patrolStatus, runtime_state: 'disabled' },
        }),
      ).toEqual([]);
    });

    it('suppresses the posture when a scheduled patrol is overdue', () => {
      // OR operand: `isScheduledPatrolOverdue(patrolStatus, nowMs)`.
      expect(
        getMonitorContextPatrolProtectionPosture({
          ...baseMonitorInput,
          patrolStatus: {
            ...baseMonitorInput.patrolStatus,
            next_patrol_at: '2026-06-30T14:00:00Z',
          },
        }),
      ).toEqual([]);
    });
  });

  describe('getPatrolCoverageLabel (via getMonitorContextPatrolProtectionPosture)', () => {
    it('derives the coverage label from status resources when no latest run is available', () => {
      // Branch: latestRun null -> latestRunCoverage '' -> status resources > 0 ->
      // `Checked N resource(s)`.
      const summaries = getMonitorContextPatrolProtectionPosture({
        findingCount: 0,
        latestRun: null,
        monitoredResourceCount: 3,
        nowMs: NOW_MS,
        patrolStatus: {
          enabled: true,
          error_count: 0,
          findings_count: 0,
          healthy: true,
          next_patrol_at: '2026-07-01T10:00:00Z',
          resources_checked: 3,
          running: false,
          runtime_state: 'active',
        },
        pendingApprovalCount: 0,
        workTypeComposition: {
          total: 0,
          approval: 0,
          failed: 0,
          inProgress: 0,
          recurring: 0,
          newIssues: 0,
        },
      });
      expect(summaries[0]).toStrictEqual({
        detail: 'Latest Patrol evidence is available while you review this monitor view.',
        id: 'coverage',
        label: 'Patrol checked 3 resources',
        tone: 'success',
      });
    });

    it('returns undefined coverage when neither latest run nor status has checked resources', () => {
      // Branch: latestRun null AND statusResourcesChecked === 0 -> undefined.
      const summaries = getMonitorContextPatrolProtectionPosture({
        findingCount: 0,
        latestRun: null,
        monitoredResourceCount: 3,
        nowMs: NOW_MS,
        patrolStatus: {
          enabled: true,
          error_count: 0,
          findings_count: 0,
          healthy: true,
          next_patrol_at: undefined,
          resources_checked: 0,
          running: false,
          runtime_state: 'active',
        },
        pendingApprovalCount: 0,
        workTypeComposition: {
          total: 0,
          approval: 0,
          failed: 0,
          inProgress: 0,
          recurring: 0,
          newIssues: 0,
        },
      });
      expect(summaries[0]).toStrictEqual({
        detail: 'Run Patrol to refresh current coverage for monitored resources.',
        id: 'coverage',
        label: 'Patrol coverage needs refresh',
        tone: 'warning',
      });
    });
  });

  describe('formatMonitorCoverageLabel (via getMonitorContextPatrolProtectionPosture)', () => {
    it('lowercases the first character of a non-empty coverage label', () => {
      // Branch: coverageLabel truthy -> `Patrol ` + lowercase-first + rest.
      // 'Checked 3 resources' (status-derived) becomes 'Patrol checked 3 resources'.
      const [coverage] = getMonitorContextPatrolProtectionPosture({
        findingCount: 0,
        latestRun: null,
        monitoredResourceCount: 3,
        nowMs: NOW_MS,
        patrolStatus: {
          enabled: true,
          error_count: 0,
          findings_count: 0,
          healthy: true,
          next_patrol_at: '2026-07-01T10:00:00Z',
          resources_checked: 5,
          running: false,
          runtime_state: 'active',
        },
        pendingApprovalCount: 0,
        workTypeComposition: {
          total: 0,
          approval: 0,
          failed: 0,
          inProgress: 0,
          recurring: 0,
          newIssues: 0,
        },
      });
      expect(coverage?.label).toBe('Patrol checked 5 resources');
    });
  });

  describe('normalizeStatus (via getPatrolWorkspaceWorkGroups)', () => {
    it('treats a whitespace-padded, mixed-case "error" status as a failed check', () => {
      // normalizeStatus('  Error  ') -> 'error', which feeds hasFailedPatrolCheck.
      // resources_checked 0 also exercises the generic failed-check detail arm.
      expect(
        getPatrolWorkspaceWorkGroups({
          latestRun: {
            error_count: 0,
            resources_checked: 0,
            status: '  Error  ' as unknown as PatrolRunStatus,
          },
        }),
      ).toEqual([
        {
          detail: 'The last Patrol check ended with runtime issues.',
          id: 'failed-check',
          label: 'Latest check needs review',
          tone: 'danger',
        },
      ]);
    });

    it('does not mark a multi-word non-error status as a failed check', () => {
      // normalizeStatus('Error Recovery') -> 'error_recovery' (!== 'error'), and
      // with error_count 0 the run is not failed.
      expect(
        getPatrolWorkspaceWorkGroups({
          latestRun: {
            error_count: 0,
            resources_checked: 5,
            status: 'Error Recovery' as unknown as PatrolRunStatus,
          },
        }),
      ).toEqual([]);
    });
  });

  describe('isPatrolCoverageStale', () => {
    it('never treats a running patrol as stale', () => {
      expect(
        isPatrolCoverageStale({
          latestRun: { completed_at: '2020-01-01T00:00:00Z' },
          nowMs: NOW_MS,
          patrolStatus: { running: true },
        }),
      ).toBe(false);
    });

    it('treats an overdue scheduled patrol as stale before checking freshness', () => {
      // isScheduledPatrolOverdue short-circuits ahead of the lastCheckAt logic.
      expect(
        isPatrolCoverageStale({
          nowMs: NOW_MS,
          patrolStatus: {
            next_patrol_at: '2026-06-30T14:05:00Z',
            running: false,
          },
        }),
      ).toBe(true);
    });

    it('is not stale when there is no last-check timestamp to compare against', () => {
      // Branch: `!lastCheckAt` -> false.
      expect(
        isPatrolCoverageStale({
          latestRun: { completed_at: undefined },
          nowMs: NOW_MS,
          patrolStatus: { running: false },
        }),
      ).toBe(false);
    });

    it('is not stale when the only timestamp is in the future', () => {
      // Branch: `lastCheckMs > nowMs` -> false.
      expect(
        isPatrolCoverageStale({
          latestRun: { completed_at: '2026-07-02T00:00:00Z' },
          nowMs: NOW_MS,
          patrolStatus: { running: false },
        }),
      ).toBe(false);
    });

    it('is not stale when the last-check timestamp cannot be parsed', () => {
      // Branch: `!Number.isFinite(lastCheckMs)` -> false.
      expect(
        isPatrolCoverageStale({
          latestRun: { completed_at: 'not-a-real-date' },
          nowMs: NOW_MS,
          patrolStatus: { running: false },
        }),
      ).toBe(false);
    });

    it('falls back to patrol status last_patrol_at when the run lacks completed_at', () => {
      // Branch: `completed_at || last_patrol_at` picks the status timestamp.
      expect(
        isPatrolCoverageStale({
          latestRun: { completed_at: undefined },
          nowMs: NOW_MS,
          patrolStatus: {
            interval_ms: 6 * 60 * 60 * 1000,
            last_patrol_at: '2026-06-28T12:00:00Z',
            running: false,
          },
        }),
      ).toBe(true);
    });

    it('uses the 24h minimum freshness window when no interval is configured', () => {
      // 25h-old check exceeds the minimum window (interval_ms 0 -> max(24h, 0)).
      expect(
        isPatrolCoverageStale({
          latestRun: { completed_at: '2026-06-29T14:00:00Z' },
          nowMs: NOW_MS,
          patrolStatus: { running: false },
        }),
      ).toBe(true);

      // 12h-old check is inside the 24h minimum window.
      expect(
        isPatrolCoverageStale({
          latestRun: { completed_at: '2026-06-30T03:00:00Z' },
          nowMs: NOW_MS,
          patrolStatus: { running: false },
        }),
      ).toBe(false);
    });
  });

  describe('getPatrolSetupIssueReason', () => {
    it('returns the readiness summary verbatim when it has no tool-call wording', () => {
      // Branch: readinessSummary truthy but neither regex matches -> return it.
      expect(getPatrolSetupIssueReason({ readinessSummary: 'Provider rate limit hit.' })).toBe(
        'Provider rate limit hit.',
      );
    });

    it('maps a "run tools" readiness summary to the fixed tool-call reason', () => {
      // Branch: second regex /\brun tools?\b/i matches.
      expect(
        getPatrolSetupIssueReason({
          readinessSummary: 'The selected model cannot reliably run tools.',
        }),
      ).toBe('Selected model cannot run Patrol tools.');
    });

    it('falls through to the blocked reason when the trigger reason is blank', () => {
      // Branch: triggerDisabledReason empty -> blockedReason wins.
      expect(
        getPatrolSetupIssueReason({
          triggerDisabledReason: '   ',
          blockedReason: 'Patrol config is invalid',
        }),
      ).toBe('Patrol config is invalid');
    });

    it('skips a whitespace-only readiness summary and falls through to other reasons', () => {
      // Branch: normalizeText(readinessSummary) -> '' fails the truthy guard.
      expect(
        getPatrolSetupIssueReason({
          readinessSummary: '   ',
          triggerDisabledReason: 'Patrol is paused',
        }),
      ).toBe('Patrol is paused');
    });

    it('skips a whitespace-only setup finding title to reach the readiness summary', () => {
      // Branch: normalizeText(setupFindingTitle) -> '' fails the truthy guard.
      expect(
        getPatrolSetupIssueReason({
          setupFindingTitle: '   ',
          readinessSummary: 'Model quota exhausted',
        }),
      ).toBe('Model quota exhausted');
    });
  });

  describe('getPatrolQueueActionDetail (via getPatrolQueueWorkspaceDescription)', () => {
    it('returns the locked-control copy even when autonomy would allow fixes', () => {
      // Branch: `input.autonomyLocked` short-circuits before the autonomy switch.
      expect(
        getPatrolQueueWorkspaceDescription({
          autonomyLevel: 'full',
          autonomyLocked: true,
          findingCount: 2,
          affectedResourceCount: 1,
        }),
      ).toBe(
        'Patrol found 2 issues on 1 affected resource. Open a row to review evidence and record the outcome.',
      );
    });
  });

  describe('getPatrolWorkspaceWorkGroups', () => {
    it('surfaces a failed-check group from status error count alone, with checked-resource detail', () => {
      // Branch: `latestRunFailed || statusErrorCount > 0` -> statusErrorCount path
      // (latest run healthy), and checkedResources > 0 -> detailed detail string.
      expect(
        getPatrolWorkspaceWorkGroups({
          latestRun: {
            error_count: 0,
            resources_checked: 3,
            status: 'healthy',
          },
          nowMs: NOW_MS,
          patrolStatus: {
            error_count: 1,
            next_patrol_at: '2026-07-01T10:00:00Z',
            running: false,
          },
        }),
      ).toEqual([
        {
          detail: 'Patrol checked 3 resources but ended with runtime issues.',
          id: 'failed-check',
          label: 'Latest check needs review',
          tone: 'danger',
        },
      ]);
    });

    it('uses the generic failed-check detail when no resources were checked', () => {
      // Branch: statusErrorCount > 0 with latestRun null -> checkedResources 0.
      expect(
        getPatrolWorkspaceWorkGroups({
          latestRun: null,
          nowMs: NOW_MS,
          patrolStatus: {
            error_count: 1,
            next_patrol_at: '2026-07-01T10:00:00Z',
            running: false,
          },
        }),
      ).toEqual([
        {
          detail: 'The last Patrol check ended with runtime issues.',
          id: 'failed-check',
          label: 'Latest check needs review',
          tone: 'danger',
        },
      ]);
    });

    it('does not add a failed-check group for an error status that normalizes away from "error"', () => {
      // status 'errored_retry' normalizes to 'errored_retry' (!== 'error'); with
      // no status error count and no other triggers, no group is produced.
      expect(
        getPatrolWorkspaceWorkGroups({
          latestRun: {
            error_count: 0,
            resources_checked: 4,
            status: 'errored_retry' as unknown as PatrolRunStatus,
          },
        }),
      ).toEqual([]);
    });
  });
});
