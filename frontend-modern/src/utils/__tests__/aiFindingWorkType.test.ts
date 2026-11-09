import { describe, expect, it } from 'vitest';

import {
  classifyPatrolFindingWorkType,
  getPatrolWorkTypeComposition,
  getPatrolWorkTypeCompositionClause,
  getPatrolFindingActionableState,
  getPatrolFindingQueueSummary,
  hasUnavailablePatrolApprovalStatus,
  getFindingPatrolWorkflowPresentation,
} from '@/utils/aiFindingPresentation';

type ClassifyInput = Parameters<typeof classifyPatrolFindingWorkType>[0];

function makeFinding(overrides: Partial<ClassifyInput> = {}): ClassifyInput {
  return {
    id: 'finding-1',
    status: 'active',
    investigationStatus: undefined,
    investigationOutcome: undefined,
    regressionCount: undefined,
    timesRaised: undefined,
    ...overrides,
  };
}

describe('classifyPatrolFindingWorkType', () => {
  it('classifies a plain active finding as new', () => {
    expect(classifyPatrolFindingWorkType(makeFinding())).toBe('new');
  });

  it('classifies an unbound queued outcome as recovery, not approval', () => {
    expect(classifyPatrolFindingWorkType(makeFinding({ investigationOutcome: 'fix_queued' }))).toBe(
      'recovery',
    );
  });

  it.each(['fix_failed', 'fix_verification_failed', 'cannot_fix', 'timed_out'] as const)(
    'classifies %s as failed',
    (outcome) => {
      expect(classifyPatrolFindingWorkType(makeFinding({ investigationOutcome: outcome }))).toBe(
        'failed',
      );
    },
  );

  it.each(['unclassified', 'interrupted'] as const)(
    'does not count %s as a failed fix, since no fix was tried',
    (outcome) => {
      expect(classifyPatrolFindingWorkType(makeFinding({ investigationOutcome: outcome }))).toBe(
        'new',
      );
    },
  );

  it('classifies investigation running as in_progress', () => {
    expect(classifyPatrolFindingWorkType(makeFinding({ investigationStatus: 'running' }))).toBe(
      'in_progress',
    );
  });

  it('classifies fix_executed as in_progress (verification pending)', () => {
    expect(
      classifyPatrolFindingWorkType(makeFinding({ investigationOutcome: 'fix_executed' })),
    ).toBe('in_progress');
  });

  it('classifies regression as recurring', () => {
    expect(classifyPatrolFindingWorkType(makeFinding({ regressionCount: 2 }))).toBe('recurring');
  });

  it('classifies multiple raises as recurring', () => {
    expect(classifyPatrolFindingWorkType(makeFinding({ timesRaised: 3 }))).toBe('recurring');
  });

  it('does not classify a first-raise finding as recurring', () => {
    expect(classifyPatrolFindingWorkType(makeFinding({ timesRaised: 1 }))).toBe('new');
  });

  it('treats non-active findings as new regardless of investigation state', () => {
    expect(
      classifyPatrolFindingWorkType(
        makeFinding({ status: 'resolved', investigationOutcome: 'fix_queued' }),
      ),
    ).toBe('new');
  });

  it('prioritises queued recovery over recurrence', () => {
    expect(
      classifyPatrolFindingWorkType(
        makeFinding({ investigationOutcome: 'fix_queued', regressionCount: 5 }),
      ),
    ).toBe('recovery');
  });

  it('prioritises failed over recurring', () => {
    expect(
      classifyPatrolFindingWorkType(
        makeFinding({ investigationOutcome: 'fix_failed', regressionCount: 5 }),
      ),
    ).toBe('failed');
  });
});

describe('getPatrolFindingQueueSummary', () => {
  it('prioritises the operator-facing consequence', () => {
    expect(
      getPatrolFindingQueueSummary({
        impact: 'Hosted workloads may slow down.',
        description: 'CPU stayed above the warning threshold.',
        recommendation: 'Review host load.',
      }),
    ).toBe('Hosted workloads may slow down.');
  });

  it('falls back through description, recommendation, and review copy', () => {
    expect(getPatrolFindingQueueSummary({ description: 'Disk usage is rising.' })).toBe(
      'Disk usage is rising.',
    );
    expect(getPatrolFindingQueueSummary({ recommendation: 'Review the affected disk.' })).toBe(
      'Review the affected disk.',
    );
    expect(getPatrolFindingQueueSummary({})).toBe(
      'Open this issue to review what Patrol found and decide the next step.',
    );
  });
});

describe('getPatrolWorkTypeComposition', () => {
  it('counts saved queued findings as unknown during a failed read, without losing other work', () => {
    const findings = [
      makeFinding({ investigationOutcome: 'fix_queued' }),
      makeFinding({ investigationOutcome: 'fix_queued' }),
      makeFinding({ investigationOutcome: 'fix_failed' }),
      makeFinding({ investigationStatus: 'running' }),
      makeFinding({ timesRaised: 3 }),
    ];
    const unavailable = getPatrolWorkTypeComposition(findings, true);
    expect(unavailable).toEqual({
      total: 5,
      approval: 0,
      approvalUnknown: 2,
      failed: 1,
      inProgress: 1,
      recurring: 1,
      newIssues: 0,
    });
    expect(getPatrolWorkTypeCompositionClause(unavailable)).toBe(
      ' — 2 queued · approval status unavailable, 1 failed fix, 1 in progress, 1 recurring',
    );
    expect(getPatrolWorkTypeComposition(findings, false)).toMatchObject({
      total: 5,
      approval: 0,
      recovery: 2,
    });
    expect(getPatrolWorkTypeCompositionClause(getPatrolWorkTypeComposition(findings))).toContain(
      '2 need action recovery',
    );
  });

  it('keeps typed action facts and ignores inactive queued outcomes during an unavailable read', () => {
    const typed: Parameters<typeof getPatrolWorkTypeComposition>[0][number] = {
      ...makeFinding({ investigationOutcome: 'fix_queued' }),
      investigationRecord: {
        id: 'investigation-1',
        finding_id: 'finding-1',
        subject: { resource_id: 'host-1' },
        trigger: { detected_at: '2026-11-09T12:00:00Z' },
        status: 'completed',
        evidence: [],
        verification: [],
        rollback: [],
        tools_used: [],
        started_at: '2026-11-09T12:00:00Z',
        action: {
          action_id: 'saved-action',
          resource_id: 'host-1',
          capability_name: 'restart',
          state: 'pending_approval',
          plan: {
            actionId: 'saved-action',
            requestId: 'request-1',
            allowed: false,
            requiresApproval: true,
            approvalPolicy: 'admin',
            rollbackAvailable: false,
          },
        },
      },
    };
    expect(hasUnavailablePatrolApprovalStatus(typed, true)).toBe(false);
    const composition = getPatrolWorkTypeComposition(
      [
        typed,
        makeFinding({ investigationOutcome: 'fix_queued' }),
        makeFinding({ status: 'resolved', investigationOutcome: 'fix_queued' }),
      ],
      true,
    );
    expect(composition).toMatchObject({ total: 3, approval: 1, approvalUnknown: 1, newIssues: 1 });
    expect(getPatrolWorkTypeCompositionClause(composition)).toBe(
      ' — 1 needs approval, 1 queued · approval status unavailable',
    );
  });

  it('returns zero counts for an empty list', () => {
    expect(getPatrolWorkTypeComposition([])).toEqual({
      total: 0,
      approval: 0,
      failed: 0,
      inProgress: 0,
      recurring: 0,
      newIssues: 0,
    });
  });

  it('classifies and counts a mixed set of findings', () => {
    const findings: ClassifyInput[] = [
      makeFinding({ investigationOutcome: 'fix_queued' }),
      makeFinding({ investigationOutcome: 'fix_failed' }),
      makeFinding({ investigationStatus: 'running' }),
      makeFinding({ regressionCount: 1 }),
      makeFinding({}),
      makeFinding({}),
    ];
    expect(getPatrolWorkTypeComposition(findings)).toEqual({
      total: 6,
      approval: 0,
      recovery: 1,
      failed: 1,
      inProgress: 1,
      recurring: 1,
      newIssues: 2,
    });
  });
});

describe('getPatrolWorkTypeCompositionClause', () => {
  it('returns empty string when all findings are new', () => {
    expect(
      getPatrolWorkTypeCompositionClause({
        total: 2,
        approval: 0,
        failed: 0,
        inProgress: 0,
        recurring: 0,
        newIssues: 2,
      }),
    ).toBe('');
  });

  it('returns a single-type clause', () => {
    expect(
      getPatrolWorkTypeCompositionClause({
        total: 3,
        approval: 1,
        failed: 0,
        inProgress: 0,
        recurring: 0,
        newIssues: 2,
      }),
    ).toBe(' — 1 needs approval');
  });

  it('pluralises correctly', () => {
    expect(
      getPatrolWorkTypeCompositionClause({
        total: 4,
        approval: 0,
        failed: 2,
        inProgress: 0,
        recurring: 0,
        newIssues: 2,
      }),
    ).toBe(' — 2 failed fixes');
  });

  it('joins multiple notable types in priority order', () => {
    expect(
      getPatrolWorkTypeCompositionClause({
        total: 5,
        approval: 1,
        failed: 1,
        inProgress: 0,
        recurring: 2,
        newIssues: 1,
      }),
    ).toBe(' — 1 needs approval, 1 failed fix, 2 recurring');
  });
});

describe('getPatrolFindingActionableState', () => {
  it('returns undefined for a plain new finding', () => {
    expect(getPatrolFindingActionableState(makeFinding())).toBeUndefined();
  });

  it('returns recovery for an unbound fix_queued outcome', () => {
    expect(
      getPatrolFindingActionableState(makeFinding({ investigationOutcome: 'fix_queued' })),
    ).toEqual({ label: 'Recover action', tone: 'warning' });
  });

  it.each(['fix_failed', 'fix_verification_failed', 'cannot_fix', 'timed_out'] as const)(
    'returns fix failed for %s',
    (outcome) => {
      expect(
        getPatrolFindingActionableState(makeFinding({ investigationOutcome: outcome })),
      ).toEqual({ label: 'Fix failed', tone: 'danger' });
    },
  );

  it('returns investigating for a running investigation', () => {
    expect(
      getPatrolFindingActionableState(makeFinding({ investigationStatus: 'running' })),
    ).toEqual({ label: 'Investigating', tone: 'info' });
  });

  it('returns verifying fix for fix_executed', () => {
    expect(
      getPatrolFindingActionableState(makeFinding({ investigationOutcome: 'fix_executed' })),
    ).toEqual({ label: 'Verifying fix', tone: 'info' });
  });

  it('tells the operator Patrol will retry an interrupted run', () => {
    expect(
      getPatrolFindingActionableState(
        makeFinding({ investigationStatus: 'failed', investigationOutcome: 'interrupted' }),
      ),
    ).toEqual({ label: 'Will retry', tone: 'neutral' });
  });

  it('leaves an unclassified run on the default review state', () => {
    expect(
      getPatrolFindingActionableState(
        makeFinding({ investigationStatus: 'completed', investigationOutcome: 'unclassified' }),
      ),
    ).toBeUndefined();
  });

  it('returns undefined for non-active findings', () => {
    expect(
      getPatrolFindingActionableState(
        makeFinding({ status: 'resolved', investigationOutcome: 'fix_queued' }),
      ),
    ).toBeUndefined();
  });
});

// Actual typed references must outrank a stale legacy outcome, in every
// presentation of the same finding, even when the approval-list read fails.
const typedFinding = (state: import('@/types/actionAudit').ActionAuditState): ClassifyInput => ({
  ...makeFinding({ investigationOutcome: 'fix_queued', regressionCount: 3 }),
  investigationRecord: {
    id: 'investigation-1',
    finding_id: 'finding-1',
    subject: { resource_id: 'host-1' },
    trigger: { detected_at: '2026-11-09T12:00:00Z' },
    status: 'completed',
    evidence: [],
    verification: [],
    rollback: [],
    tools_used: [],
    started_at: '2026-11-09T12:00:00Z',
    action: {
      action_id: 'action-1',
      resource_id: 'host-1',
      capability_name: 'restart',
      state,
      plan: {
        actionId: 'action-1',
        requestId: 'request-1',
        allowed: false,
        requiresApproval: true,
        approvalPolicy: 'admin',
        rollbackAvailable: false,
      },
    },
  },
});
const approvalNow = Date.parse('2026-11-09T12:10:00Z');
const liveApproval = {
  status: 'pending',
  toolId: 'investigation_fix',
  targetId: 'finding-1',
  expiresAt: '2026-11-09T12:11:00Z',
};

describe('Patrol current action facts', () => {
  it.each([
    ['pending_approval', 'approval', 'Approval required', '1 needs approval', 'Approve or reject'],
    ['planned', 'ready', 'Ready to run', '1 ready to run', 'Run action'],
    ['approved', 'approved', 'Approved to run', '1 approved to run', 'Run approved action'],
    ['executing', 'in_progress', 'Action running', '1 in progress', 'Action running'],
    ['completed', 'completed', 'Action completed', '1 completed action', 'Review completed action'],
    ['failed', 'failed', 'Fix failed', '1 failed fix', 'Fix failed'],
    ['rejected', 'rejected', 'Action rejected', '1 rejected action', 'Review rejected action'],
    ['expired', 'expired', 'Action expired', '1 expired action', 'Review expired action'],
  ] as const)(
    'keeps %s consistent across counts, row and workflow',
    (state, workType, badge, clause, handoff) => {
      const finding = typedFinding(state);
      const unchanged = JSON.stringify(finding);
      for (const unavailable of [false, true]) {
        expect(classifyPatrolFindingWorkType(finding, [liveApproval], approvalNow)).toBe(workType);
        expect(getPatrolFindingActionableState(finding, [liveApproval], approvalNow)?.label).toBe(
          badge,
        );
        const composition = getPatrolWorkTypeComposition(
          [finding],
          unavailable,
          [liveApproval],
          approvalNow,
        );
        expect(composition.total).toBe(1);
        expect(getPatrolWorkTypeCompositionClause(composition)).toBe(' — ' + clause);
        expect(
          getFindingPatrolWorkflowPresentation(
            {
              ...finding,
              id: 'finding-1',
              source: 'ai-patrol',
              resourceId: 'host-1',
              resourceName: 'host-1',
              title: 'Finding',
            },
            [liveApproval],
            approvalNow,
            unavailable,
          )?.label,
        ).toBe(handoff);
      }
      expect(JSON.stringify(finding)).toBe(unchanged);
    },
  );

  it('uses only a live pending investigation-fix request for the same finding', () => {
    const finding = makeFinding({ investigationOutcome: 'fix_queued' });
    expect(classifyPatrolFindingWorkType(finding, [liveApproval], approvalNow)).toBe('approval');
    expect(getPatrolFindingActionableState(finding, [liveApproval], approvalNow)?.label).toBe(
      'Approval required',
    );
    for (const request of [
      { ...liveApproval, targetId: 'other-finding' },
      { ...liveApproval, toolId: 'other-tool' },
      { ...liveApproval, status: 'approved' },
      { ...liveApproval, expiresAt: '2026-11-09T12:10:00Z' },
    ]) {
      expect(classifyPatrolFindingWorkType(finding, [request], approvalNow)).toBe('recovery');
      expect(getPatrolFindingActionableState(finding, [request], approvalNow)?.label).toBe(
        'Recover action',
      );
    }
    expect(
      getPatrolWorkTypeComposition([finding], true, [liveApproval], approvalNow),
    ).toMatchObject({ total: 1, approval: 0, approvalUnknown: 1 });
    expect(
      getPatrolWorkTypeComposition([finding], false, [liveApproval], approvalNow),
    ).toMatchObject({ total: 1, approval: 1 });
    expect(
      getPatrolWorkTypeComposition([finding], false, [liveApproval], approvalNow + 60000),
    ).toMatchObject({ total: 1, approval: 0, recovery: 1 });
  });

  it('keeps inactive typed findings out of current action counts', () => {
    for (const status of ['resolved', 'dismissed', 'snoozed'] as const) {
      const finding = { ...typedFinding('pending_approval'), status };
      expect(classifyPatrolFindingWorkType(finding)).toBe('new');
      expect(getPatrolFindingActionableState(finding)).toBeUndefined();
      expect(getPatrolWorkTypeComposition([finding])).toMatchObject({
        total: 1,
        approval: 0,
        newIssues: 1,
      });
    }
  });

  it('retains all mixed work and resource-independent facts without reclassifying new issues', () => {
    const composition = getPatrolWorkTypeComposition([
      typedFinding('planned'),
      typedFinding('approved'),
      typedFinding('executing'),
      typedFinding('completed'),
      typedFinding('rejected'),
      typedFinding('expired'),
      makeFinding({ investigationOutcome: 'fix_failed' }),
      makeFinding({ timesRaised: 3 }),
      makeFinding(),
    ]);
    expect(composition).toEqual({
      total: 9,
      approval: 0,
      ready: 1,
      approved: 1,
      inProgress: 1,
      completed: 1,
      rejected: 1,
      expired: 1,
      failed: 1,
      recurring: 1,
      newIssues: 1,
    });
    expect(getPatrolWorkTypeCompositionClause(composition)).toBe(
      ' — 1 ready to run, 1 approved to run, 1 failed fix, 1 in progress, 1 completed action, 1 rejected action, 1 expired action, 1 recurring',
    );
  });
});
