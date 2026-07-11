import { describe, expect, it } from 'vitest';

import type { PatrolRunRecord, PatrolTriggerStatus } from '@/api/patrol';
import {
  formatPatrolActivityBreakdown,
  getPatrolActivityBreakdown,
  getPatrolLatestRunPresentation,
  getPatrolRunCoverageSummary,
  getPatrolRunOperatorRecordPresentation,
  getPatrolRunRecordSummaryPresentation,
  getPatrolRunResourcesHeading,
  getPatrolTriggerStatusSummary,
} from '@/utils/patrolRunPresentation';
import type { PatrolActivityBreakdown } from '@/utils/patrolRunPresentation';

const REFERENCE_DATE = new Date('2026-03-12T12:00:00Z');

const makeRun = (overrides: Partial<PatrolRunRecord> = {}): PatrolRunRecord => ({
  id: 'run-1',
  started_at: '2026-03-12T10:00:00Z',
  completed_at: '2026-03-12T10:01:00Z',
  duration_ms: 60_000,
  type: 'patrol',
  resources_checked: 0,
  nodes_checked: 0,
  guests_checked: 0,
  docker_checked: 0,
  storage_checked: 0,
  hosts_checked: 0,
  truenas_checked: 0,
  pbs_checked: 0,
  pmg_checked: 0,
  kubernetes_checked: 0,
  new_findings: 0,
  existing_findings: 0,
  rejected_findings: 0,
  resolved_findings: 0,
  auto_fix_count: 0,
  findings_summary: 'ok',
  finding_ids: [],
  error_count: 0,
  status: 'healthy',
  triage_flags: 0,
  tool_call_count: 0,
  ...overrides,
});

const makeTriggerStatus = (
  overrides: Partial<PatrolTriggerStatus> = {},
): PatrolTriggerStatus => ({
  running: false,
  pending_triggers: 0,
  current_interval_ms: 300_000,
  recent_events: 0,
  is_busy_mode: false,
  alert_triggers_enabled: true,
  anomaly_triggers_enabled: true,
  ...overrides,
});

describe('getPatrolRunOperatorRecordPresentation (branch coverage)', () => {
  it('falls back to generic review copy when status is error but runtimeFailure summary is unavailable', () => {
    const result = getPatrolRunOperatorRecordPresentation(
      makeRun({
        status: 'error',
        error_count: 0,
        resources_checked: 10,
        duration_ms: 60_000,
        scope_resource_ids: [],
        effective_scope_resource_ids: [],
      }),
    );
    expect(result).toEqual({
      headline: 'Patrol needs attention',
      detail: 'Checked 10 resources in 1m. It ended with issues that need review.',
    });
  });

  it('reports a completed-check headline for legacy runs without finding_ids', () => {
    const result = getPatrolRunOperatorRecordPresentation(
      makeRun({
        status: 'healthy',
        error_count: 0,
        finding_ids: undefined,
        resources_checked: 10,
        duration_ms: 60_000,
      }),
    );
    expect(result).toEqual({
      headline: 'Completed check',
      detail: 'Checked 10 resources in 1m. This older run has no issue list.',
    });
  });

  it('reports fixed count without new findings', () => {
    const result = getPatrolRunOperatorRecordPresentation(
      makeRun({
        auto_fix_count: 3,
        new_findings: 0,
        resources_checked: 10,
        duration_ms: 60_000,
      }),
    );
    expect(result).toEqual({
      headline: 'Fixed 3 issues',
      detail: 'Checked 10 resources in 1m. Fixed 3 issues.',
    });
  });

  it('reports singular fixed count', () => {
    const result = getPatrolRunOperatorRecordPresentation(
      makeRun({
        auto_fix_count: 1,
        new_findings: 0,
        resources_checked: 10,
        duration_ms: 60_000,
      }),
    );
    expect(result.headline).toBe('Fixed 1 issue');
  });

  it('reports confirmed-resolved findings', () => {
    const result = getPatrolRunOperatorRecordPresentation(
      makeRun({
        resolved_findings: 2,
        resources_checked: 10,
        duration_ms: 60_000,
      }),
    );
    expect(result).toEqual({
      headline: 'Confirmed 2 issues resolved',
      detail: 'Checked 10 resources in 1m. Confirmed 2 issues resolved.',
    });
  });

  it('reports new findings ready for review when nothing was fixed', () => {
    const result = getPatrolRunOperatorRecordPresentation(
      makeRun({
        new_findings: 5,
        auto_fix_count: 0,
        resources_checked: 10,
        duration_ms: 60_000,
      }),
    );
    expect(result).toEqual({
      headline: 'Found 5 new issues',
      detail: 'Checked 10 resources in 1m. Ready for review.',
    });
  });

  it('reports singular new finding headline', () => {
    const result = getPatrolRunOperatorRecordPresentation(
      makeRun({
        new_findings: 1,
        resources_checked: 10,
        duration_ms: 60_000,
      }),
    );
    expect(result.headline).toBe('Found 1 new issue');
  });

  it('reports existing findings still open', () => {
    const result = getPatrolRunOperatorRecordPresentation(
      makeRun({
        existing_findings: 4,
        resources_checked: 10,
        duration_ms: 60_000,
      }),
    );
    expect(result).toEqual({
      headline: '4 issues still open',
      detail: 'Checked 10 resources in 1m. No new issues.',
    });
  });

  it('reports singular existing finding headline', () => {
    const result = getPatrolRunOperatorRecordPresentation(
      makeRun({
        existing_findings: 1,
        resources_checked: 10,
        duration_ms: 60_000,
      }),
    );
    expect(result.headline).toBe('1 issue still open');
  });

  it('flags critical status with finding_ids as needing attention even without errors', () => {
    const result = getPatrolRunOperatorRecordPresentation(
      makeRun({
        status: 'critical',
        error_count: 0,
        finding_ids: [],
        resources_checked: 10,
        duration_ms: 60_000,
      }),
    );
    expect(result).toEqual({
      headline: 'Patrol needs attention',
      detail: 'Checked 10 resources in 1m. It ended with issues that need review.',
    });
  });

  // formatRunActionContext: exercised through operator presentation detail string
  it('exercises formatRunActionContext with coverage but no duration', () => {
    const result = getPatrolRunOperatorRecordPresentation(
      makeRun({
        duration_ms: 0,
        resources_checked: 7,
        scope_resource_ids: [],
        effective_scope_resource_ids: [],
      }),
    );
    expect(result.detail).toBe('Checked 7 resources. No issues recorded.');
  });

  it('exercises formatRunActionContext with duration but no coverage', () => {
    const result = getPatrolRunOperatorRecordPresentation(
      makeRun({
        duration_ms: 120_000,
        resources_checked: 0,
        scope_resource_ids: [],
        effective_scope_resource_ids: [],
      }),
    );
    expect(result.detail).toBe('Patrol ran in 2m. No issues recorded.');
  });

  it('exercises formatRunActionContext with neither coverage nor duration', () => {
    const result = getPatrolRunOperatorRecordPresentation(
      makeRun({
        duration_ms: 0,
        resources_checked: 0,
        scope_resource_ids: [],
        effective_scope_resource_ids: [],
      }),
    );
    expect(result.detail).toBe('Patrol ran. No issues recorded.');
  });
});

describe('getPatrolLatestRunPresentation (branch coverage)', () => {
  it('returns undefined for an empty runs array', () => {
    expect(getPatrolLatestRunPresentation([])).toBeUndefined();
  });

  it('returns undefined when no run has a non-empty timestamp', () => {
    expect(
      getPatrolLatestRunPresentation([
        makeRun({ id: 'blank', started_at: '  ', completed_at: '' }),
        makeRun({ id: 'blank2', started_at: '', completed_at: '   ' }),
      ]),
    ).toBeUndefined();
  });

  it('falls back to started_at for the timestamp when completed_at is empty', () => {
    const result = getPatrolLatestRunPresentation([
      makeRun({
        id: 'started-only',
        started_at: '2026-03-12T09:30:00Z',
        completed_at: '',
        type: 'patrol',
        status: 'healthy',
        error_count: 0,
        finding_ids: ['finding-1'],
        resources_checked: 0,
        scope_resource_ids: [],
        effective_scope_resource_ids: [],
      }),
    ]);
    expect(result).toBeDefined();
    expect(result?.timestamp).toBe('2026-03-12T09:30:00Z');
    expect(result?.findingsSnapshotAvailable).toBe(true);
    expect(result?.kindLabel).toBe('Patrol check');
  });

  it('reports findingsSnapshotAvailable true when finding_ids is an empty array', () => {
    const result = getPatrolLatestRunPresentation([
      makeRun({
        finding_ids: [],
        status: 'healthy',
        error_count: 0,
        resources_checked: 5,
        scope_resource_ids: [],
        effective_scope_resource_ids: [],
        type: 'scoped',
      }),
    ]);
    expect(result?.findingsSnapshotAvailable).toBe(true);
    expect(result?.kindLabel).toBe('Targeted check');
  });

  it('skips runs with empty timestamps and finds the first run with a usable one', () => {
    const result = getPatrolLatestRunPresentation([
      makeRun({ id: 'empty-ts', started_at: '', completed_at: '' }),
      makeRun({ id: 'real-ts', started_at: '2026-03-12T08:00:00Z', completed_at: '' }),
    ]);
    expect(result).toBeDefined();
    expect(result?.timestamp).toBe('2026-03-12T08:00:00Z');
  });
});

describe('getPatrolRunRecordSummaryPresentation (branch coverage)', () => {
  it('returns generic review outcome when unhealthy without runtime failure detail', () => {
    const result = getPatrolRunRecordSummaryPresentation(
      makeRun({
        status: 'critical',
        error_count: 0,
        finding_ids: [],
        resources_checked: 10,
        duration_ms: 60_000,
        scope_resource_ids: [],
        effective_scope_resource_ids: [],
      }),
    );
    expect(result.summary).toBe('Checked 10 resources in 1m.');
    expect(result.outcome).toBe('Patrol ended with issues requiring review.');
    expect(result.action).toBeUndefined();
  });

  it('explains legacy runs without finding records', () => {
    const result = getPatrolRunRecordSummaryPresentation(
      makeRun({
        status: 'healthy',
        error_count: 0,
        finding_ids: undefined,
        resources_checked: 10,
        duration_ms: 60_000,
      }),
    );
    expect(result.summary).toBe('Checked 10 resources in 1m.');
    expect(result.outcome).toBe(
      'This older run has no finding record, so Patrol cannot show its issue list.',
    );
  });

  it('reports new findings without any fixes', () => {
    const result = getPatrolRunRecordSummaryPresentation(
      makeRun({
        new_findings: 3,
        auto_fix_count: 0,
        resources_checked: 10,
        duration_ms: 60_000,
      }),
    );
    expect(result.outcome).toBe('Patrol found 3 new issues.');
  });

  it('reports new findings with fixes applied', () => {
    const result = getPatrolRunRecordSummaryPresentation(
      makeRun({
        new_findings: 2,
        auto_fix_count: 1,
        resources_checked: 10,
        duration_ms: 60_000,
      }),
    );
    expect(result.outcome).toBe('Patrol found 2 new issues. Patrol fixed 1 issue.');
  });

  it('uses singular form for one new finding', () => {
    const result = getPatrolRunRecordSummaryPresentation(
      makeRun({
        new_findings: 1,
        auto_fix_count: 0,
        resources_checked: 10,
        duration_ms: 60_000,
      }),
    );
    expect(result.outcome).toBe('Patrol found 1 new issue.');
  });

  it('reports existing findings with plural remain form', () => {
    const result = getPatrolRunRecordSummaryPresentation(
      makeRun({
        existing_findings: 3,
        new_findings: 0,
        resources_checked: 10,
        duration_ms: 60_000,
      }),
    );
    expect(result.outcome).toBe('No new issues. 3 existing issues remain.');
  });

  it('reports existing findings with singular remains form', () => {
    const result = getPatrolRunRecordSummaryPresentation(
      makeRun({
        existing_findings: 1,
        new_findings: 0,
        resources_checked: 10,
        duration_ms: 60_000,
      }),
    );
    expect(result.outcome).toBe('No new issues. 1 existing issue remains.');
  });

  it('omits duration from summary when duration_ms is zero', () => {
    const result = getPatrolRunRecordSummaryPresentation(
      makeRun({
        duration_ms: 0,
        resources_checked: 4,
        scope_resource_ids: [],
        effective_scope_resource_ids: [],
      }),
    );
    expect(result.summary).toBe('Checked 4 resources.');
  });
});

describe('formatPatrolActivityBreakdown (branch coverage)', () => {
  it('formats alert-cleared segment', () => {
    expect(
      formatPatrolActivityBreakdown({
        totalRuns: 1,
        fullPatrols: 0,
        verificationChecks: 0,
        alertTriggeredRuns: 0,
        anomalyTriggeredRuns: 0,
        alertClearedRuns: 1,
        otherScopedRuns: 0,
        newFindings: 0,
      } satisfies PatrolActivityBreakdown),
    ).toBe('1 alert-cleared check');
  });

  it('formats verification segment', () => {
    expect(
      formatPatrolActivityBreakdown({
        totalRuns: 1,
        fullPatrols: 0,
        verificationChecks: 1,
        alertTriggeredRuns: 0,
        anomalyTriggeredRuns: 0,
        alertClearedRuns: 0,
        otherScopedRuns: 0,
        newFindings: 0,
      } satisfies PatrolActivityBreakdown),
    ).toBe('1 follow-up check');
  });

  it('formats other-scoped segment', () => {
    expect(
      formatPatrolActivityBreakdown({
        totalRuns: 1,
        fullPatrols: 0,
        verificationChecks: 0,
        alertTriggeredRuns: 0,
        anomalyTriggeredRuns: 0,
        alertClearedRuns: 0,
        otherScopedRuns: 1,
        newFindings: 0,
      } satisfies PatrolActivityBreakdown),
    ).toBe('1 targeted check');
  });

  it('returns empty string when all segments are zero', () => {
    expect(
      formatPatrolActivityBreakdown({
        totalRuns: 0,
        fullPatrols: 0,
        verificationChecks: 0,
        alertTriggeredRuns: 0,
        anomalyTriggeredRuns: 0,
        alertClearedRuns: 0,
        otherScopedRuns: 0,
        newFindings: 0,
      } satisfies PatrolActivityBreakdown),
    ).toBe('');
  });

  it('renders all segments in canonical order with plurals', () => {
    expect(
      formatPatrolActivityBreakdown({
        totalRuns: 8,
        fullPatrols: 2,
        verificationChecks: 2,
        alertTriggeredRuns: 2,
        anomalyTriggeredRuns: 1,
        alertClearedRuns: 1,
        otherScopedRuns: 1,
        newFindings: 0,
      } satisfies PatrolActivityBreakdown),
    ).toBe(
      '2 full checks, 2 alert-triggered checks, 1 anomaly-triggered check, 1 alert-cleared check, 2 follow-up checks, 1 targeted check',
    );
  });
});

describe('getPatrolTriggerStatusSummary (branch coverage)', () => {
  it('returns undefined when status is undefined', () => {
    expect(getPatrolTriggerStatusSummary(undefined)).toBeUndefined();
  });

  it('returns undefined when triggers are unblocked and all flags are clean', () => {
    expect(getPatrolTriggerStatusSummary(makeTriggerStatus())).toBeUndefined();
  });

  it('returns undefined when triggers are blocked but manual run is explicitly available', () => {
    expect(
      getPatrolTriggerStatusSummary(
        makeTriggerStatus({ event_triggers_blocked: true }),
        { manualRunAvailable: true },
      ),
    ).toBeUndefined();
  });

  it('returns undefined when blocked, manual unavailable, but no blocked reason given', () => {
    expect(
      getPatrolTriggerStatusSummary(
        makeTriggerStatus({ event_triggers_blocked: true }),
        { manualRunAvailable: false },
      ),
    ).toBeUndefined();
  });

  it('returns alerts-off note when only alert triggers are disabled', () => {
    expect(
      getPatrolTriggerStatusSummary(
        makeTriggerStatus({
          alert_triggers_enabled: false,
        }),
      ),
    ).toBe('alerts off');
  });

  it('combines queued and busy-mode notes', () => {
    expect(
      getPatrolTriggerStatusSummary(
        makeTriggerStatus({
          pending_triggers: 2,
          is_busy_mode: true,
        }),
      ),
    ).toBe('2 queued · busy mode');
  });

  it('combines anomalies-off and alerts-off notes', () => {
    expect(
      getPatrolTriggerStatusSummary(
        makeTriggerStatus({
          alert_triggers_enabled: false,
          anomaly_triggers_enabled: false,
        }),
      ),
    ).toBe('alerts off · anomalies off');
  });

  it('returns the in-progress summary when manual is blocked by an already-running reason', () => {
    expect(
      getPatrolTriggerStatusSummary(
        makeTriggerStatus({ event_triggers_blocked: true }),
        {
          manualRunAvailable: false,
          manualRunBlockedReason: 'Patrol run is in progress',
        },
      ),
    ).toBe(
      'A Patrol run is already in progress. New automatic and manual runs are paused until it finishes.',
    );
  });
});

describe('getPatrolActivityBreakdown + isSameLocalDay (branch coverage)', () => {
  it('counts alert-cleared triggered scoped runs', () => {
    const result = getPatrolActivityBreakdown(
      [
        makeRun({
          id: 'cleared-1',
          type: 'scoped',
          trigger_reason: 'alert_cleared',
        }),
      ],
      REFERENCE_DATE,
    );
    expect(result.alertClearedRuns).toBe(1);
    expect(result.totalRuns).toBe(1);
  });

  it('counts verification-type runs as follow-up checks', () => {
    const result = getPatrolActivityBreakdown(
      [
        makeRun({
          id: 'verif-1',
          type: 'verification',
          trigger_reason: 'alert_fired',
        }),
      ],
      REFERENCE_DATE,
    );
    expect(result.verificationChecks).toBe(1);
    expect(result.alertTriggeredRuns).toBe(0);
  });

  it('counts unknown trigger reasons as other scoped runs', () => {
    const result = getPatrolActivityBreakdown(
      [
        makeRun({
          id: 'manual-1',
          type: 'scoped',
          trigger_reason: 'manual',
        }),
      ],
      REFERENCE_DATE,
    );
    expect(result.otherScopedRuns).toBe(1);
  });

  it('counts scoped runs with no trigger reason as other scoped runs', () => {
    const result = getPatrolActivityBreakdown(
      [
        makeRun({
          id: 'no-trigger',
          type: 'scoped',
          trigger_reason: undefined,
        }),
      ],
      REFERENCE_DATE,
    );
    expect(result.otherScopedRuns).toBe(1);
  });

  it('skips runs on a different local day', () => {
    const result = getPatrolActivityBreakdown(
      [
        makeRun({
          id: 'yesterday',
          started_at: '2026-03-11T10:00:00Z',
          completed_at: '2026-03-11T10:01:00Z',
          type: 'patrol',
        }),
      ],
      REFERENCE_DATE,
    );
    expect(result.totalRuns).toBe(0);
    expect(result.fullPatrols).toBe(0);
  });

  it('skips runs with an invalid started_at timestamp (isSameLocalDay invalid-date branch)', () => {
    const result = getPatrolActivityBreakdown(
      [
        makeRun({
          id: 'bad-date',
          started_at: 'not-a-valid-date',
        }),
      ],
      REFERENCE_DATE,
    );
    expect(result.totalRuns).toBe(0);
  });

  it('skips runs with an empty started_at string (isSameLocalDay empty-timestamp branch)', () => {
    const result = getPatrolActivityBreakdown(
      [
        makeRun({
          id: 'empty-date',
          started_at: '',
        } as unknown as PatrolRunRecord),
      ],
      REFERENCE_DATE,
    );
    expect(result.totalRuns).toBe(0);
  });

  it('clamps negative new_findings to zero', () => {
    const result = getPatrolActivityBreakdown(
      [
        makeRun({
          id: 'neg-findings',
          type: 'patrol',
          new_findings: -5,
        }),
      ],
      REFERENCE_DATE,
    );
    expect(result.totalRuns).toBe(1);
    expect(result.newFindings).toBe(0);
  });

  it('accumulates new findings across multiple same-day runs', () => {
    const result = getPatrolActivityBreakdown(
      [
        makeRun({ id: 'a', type: 'patrol', new_findings: 2 }),
        makeRun({ id: 'b', type: 'patrol', new_findings: 3 }),
      ],
      REFERENCE_DATE,
    );
    expect(result.newFindings).toBe(5);
    expect(result.fullPatrols).toBe(2);
  });
});

describe('getPatrolRunResourcesHeading (branch coverage)', () => {
  it('falls back to plain count when scoped resources exist but nothing was checked', () => {
    expect(
      getPatrolRunResourcesHeading({
        resources_checked: 0,
        scope_resource_ids: ['r1'],
        effective_scope_resource_ids: ['r1'],
      }),
    ).toBe('Resources checked (0)');
  });

  it('falls back to plain count when all scoped resources were checked', () => {
    expect(
      getPatrolRunResourcesHeading({
        resources_checked: 2,
        scope_resource_ids: ['r1', 'r2'],
        effective_scope_resource_ids: ['r1', 'r2'],
      }),
    ).toBe('Resources checked (2)');
  });

  it('returns zero count when no resources checked and no scope', () => {
    expect(
      getPatrolRunResourcesHeading({
        resources_checked: 0,
        scope_resource_ids: [],
        effective_scope_resource_ids: [],
      }),
    ).toBe('Resources checked (0)');
  });

  it('shows partial count when fewer resources checked than scoped', () => {
    expect(
      getPatrolRunResourcesHeading({
        resources_checked: 1,
        scope_resource_ids: ['r1', 'r2', 'r3'],
        effective_scope_resource_ids: ['r1', 'r2', 'r3'],
      }),
    ).toBe('Resources checked (1 of 3 scoped)');
  });
});

describe('formatResourceCount via getPatrolRunCoverageSummary (branch coverage)', () => {
  it('produces singular scoped-resource label for one checked of one scoped', () => {
    expect(
      getPatrolRunCoverageSummary({
        resources_checked: 1,
        scope_resource_ids: ['r1'],
        effective_scope_resource_ids: ['r1'],
      }),
    ).toBe('Checked 1 scoped resource');
  });

  it('produces plural scoped-resource label for multiple checked meeting scope', () => {
    expect(
      getPatrolRunCoverageSummary({
        resources_checked: 3,
        scope_resource_ids: ['r1', 'r2', 'r3'],
        effective_scope_resource_ids: ['r1', 'r2', 'r3'],
      }),
    ).toBe('Checked 3 scoped resources');
  });

  it('produces singular resource label without qualifier for one non-scoped resource', () => {
    expect(
      getPatrolRunCoverageSummary({
        resources_checked: 1,
        scope_resource_ids: [],
        effective_scope_resource_ids: [],
      }),
    ).toBe('Checked 1 resource');
  });

  it('produces empty coverage summary when nothing checked and no scope', () => {
    expect(
      getPatrolRunCoverageSummary({
        resources_checked: 0,
        scope_resource_ids: [],
        effective_scope_resource_ids: [],
      }),
    ).toBe('');
  });
});
