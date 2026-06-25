import { describe, expect, it } from 'vitest';

import {
  classifyPatrolFindingWorkType,
  getPatrolWorkTypeComposition,
  getPatrolWorkTypeCompositionClause,
  getPatrolFindingActionableState,
} from '@/utils/aiFindingPresentation';

type ClassifyInput = Parameters<typeof classifyPatrolFindingWorkType>[0];

function makeFinding(overrides: Partial<ClassifyInput> = {}): ClassifyInput {
  return {
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

  it('classifies fix_queued as approval', () => {
    expect(
      classifyPatrolFindingWorkType(makeFinding({ investigationOutcome: 'fix_queued' })),
    ).toBe('approval');
  });

  it.each([
    'fix_failed',
    'fix_verification_failed',
    'cannot_fix',
    'timed_out',
  ] as const)('classifies %s as failed', (outcome) => {
    expect(classifyPatrolFindingWorkType(makeFinding({ investigationOutcome: outcome }))).toBe(
      'failed',
    );
  });

  it('classifies investigation running as in_progress', () => {
    expect(
      classifyPatrolFindingWorkType(makeFinding({ investigationStatus: 'running' })),
    ).toBe('in_progress');
  });

  it('classifies fix_executed as in_progress (verification pending)', () => {
    expect(
      classifyPatrolFindingWorkType(makeFinding({ investigationOutcome: 'fix_executed' })),
    ).toBe('in_progress');
  });

  it('classifies regression as recurring', () => {
    expect(
      classifyPatrolFindingWorkType(makeFinding({ regressionCount: 2 })),
    ).toBe('recurring');
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

  it('prioritises approval over failed when both conditions could apply', () => {
    expect(
      classifyPatrolFindingWorkType(
        makeFinding({ investigationOutcome: 'fix_queued', regressionCount: 5 }),
      ),
    ).toBe('approval');
  });

  it('prioritises failed over recurring', () => {
    expect(
      classifyPatrolFindingWorkType(
        makeFinding({ investigationOutcome: 'fix_failed', regressionCount: 5 }),
      ),
    ).toBe('failed');
  });
});

describe('getPatrolWorkTypeComposition', () => {
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
      approval: 1,
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

  it('returns approval required for fix_queued', () => {
    expect(
      getPatrolFindingActionableState(makeFinding({ investigationOutcome: 'fix_queued' })),
    ).toEqual({ label: 'Approval required', tone: 'warning' });
  });

  it.each(['fix_failed', 'fix_verification_failed', 'cannot_fix', 'timed_out'] as const)(
    'returns fix failed for %s',
    (outcome) => {
      expect(getPatrolFindingActionableState(makeFinding({ investigationOutcome: outcome }))).toEqual(
        { label: 'Fix failed', tone: 'danger' },
      );
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

  it('returns undefined for non-active findings', () => {
    expect(
      getPatrolFindingActionableState(
        makeFinding({ status: 'resolved', investigationOutcome: 'fix_queued' }),
      ),
    ).toBeUndefined();
  });
});
