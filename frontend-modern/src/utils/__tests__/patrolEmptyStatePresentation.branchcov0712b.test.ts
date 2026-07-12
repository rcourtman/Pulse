import { describe, expect, it } from 'vitest';

import type { PatrolRunRecord } from '@/api/patrol';
import type { IntelligenceHealthScore } from '@/types/aiIntelligence';
import {
  getInvestigationMessagesState,
  getPatrolFindingsEmptyState,
} from '@/utils/patrolEmptyStatePresentation';

// Branch-coverage supplement for patrolEmptyStatePresentation. The three target
// functions exercised here are getInvestigationMessagesState (exported),
// getPatrolRunSnapshotEmptyState (module-private -> driven through
// getPatrolFindingsEmptyState with filter 'all' + empty finding_ids), and
// getLatestRunCoverageContext (module-private -> driven through the healthy
// 'active' clear-state path).

const makeRun = (overrides: Partial<PatrolRunRecord> = {}): PatrolRunRecord => ({
  id: 'run-1',
  started_at: '2026-07-12T10:00:00Z',
  completed_at: '2026-07-12T10:01:00Z',
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

const HEALTHY_OVERALL: IntelligenceHealthScore = {
  score: 100,
  grade: 'A',
  trend: 'stable',
  factors: [],
  prediction: 'Infrastructure is healthy with no significant issues detected.',
};

describe('getInvestigationMessagesState (branchcov2)', () => {
  it('prefers the loading state even when messages are already present', () => {
    // Exercises the first `if (loading)` arm and confirms it short-circuits the
    // hasMessages check (loading=true, hasMessages=true).
    expect(getInvestigationMessagesState(true, true)).toStrictEqual({
      text: 'Loading messages...',
      empty: false,
    });
  });

  it('returns a neutral non-empty state when messages exist and nothing is loading', () => {
    // Exercises the final fall-through return (loading=false, hasMessages=true),
    // the only arm the sibling test file does not reach.
    expect(getInvestigationMessagesState(false, true)).toStrictEqual({
      text: '',
      empty: false,
    });
  });
});

describe('getPatrolRunSnapshotEmptyState (exercised via getPatrolFindingsEmptyState)', () => {
  it('uses the info tone with a coverage prefix for a healthy run that covered part of the scope', () => {
    // Healthy arm (status healthy, no errors) + truthy coverageSummary arm of
    // the `coveragePrefix` ternary.
    expect(
      getPatrolFindingsEmptyState({
        filter: 'all',
        runSnapshot: {
          resources_checked: 1,
          scope_resource_ids: ['seed-resource'],
          effective_scope_resource_ids: ['expanded-a', 'expanded-b'],
          finding_ids: [],
          status: 'healthy',
          error_count: 0,
        },
      }),
    ).toStrictEqual({
      title: 'No findings recorded for this run',
      body: 'Checked 1 of 2 scoped resources. This run recorded no Patrol findings.',
      tone: 'info',
    });
  });

  it('uses the info tone with no coverage prefix for a healthy run that checked nothing', () => {
    // Healthy arm + falsy coverageSummary arm (empty prefix) of the ternary.
    expect(
      getPatrolFindingsEmptyState({
        filter: 'all',
        runSnapshot: {
          resources_checked: 0,
          scope_resource_ids: [],
          effective_scope_resource_ids: [],
          finding_ids: [],
          status: 'healthy',
          error_count: 0,
        },
      }),
    ).toStrictEqual({
      title: 'No findings recorded for this run',
      body: 'This run recorded no Patrol findings.',
      tone: 'info',
    });
  });

  it('uses the warning tone with no coverage prefix for an unhealthy run that checked nothing', () => {
    // Unhealthy arm + falsy coverageSummary arm. The sibling test only covers
    // the unhealthy arm together with a non-empty coverage prefix, so this
    // newly exercises the empty-prefix combination.
    expect(
      getPatrolFindingsEmptyState({
        filter: 'all',
        runSnapshot: {
          resources_checked: 0,
          scope_resource_ids: [],
          effective_scope_resource_ids: [],
          finding_ids: [],
          status: 'error',
          error_count: 1,
        },
      }),
    ).toStrictEqual({
      title: 'No findings recorded for this run',
      body: 'This run recorded no Patrol findings, but it ended with issues requiring review.',
      tone: 'warning',
    });
  });
});

describe('getLatestRunCoverageContext (exercised via getPatrolFindingsEmptyState)', () => {
  it('returns no body context when the runs array is empty', () => {
    // Exercises the `runs.length === 0` arm of the first guard (distinct from
    // the `!runs` arm already covered when runs is omitted entirely).
    const result = getPatrolFindingsEmptyState({
      filter: 'active',
      overallHealth: HEALTHY_OVERALL,
      runtimeState: 'active',
      runs: [],
    });
    expect(result).toStrictEqual({
      title: 'No current issues',
      body: undefined,
      tone: 'success',
    });
  });

  it('returns no body context when the latest run has no coverage summary', () => {
    // Exercises the `!coverageSummary` early return. runs is non-empty, but the
    // latest run checked zero resources with no scope, so coverageSummary is ''.
    const result = getPatrolFindingsEmptyState({
      filter: 'active',
      overallHealth: HEALTHY_OVERALL,
      runtimeState: 'active',
      runs: [
        makeRun({
          id: 'run-empty-coverage',
          resources_checked: 0,
          scope_resource_ids: [],
          effective_scope_resource_ids: [],
        }),
      ],
    });
    expect(result).toStrictEqual({
      title: 'No current issues',
      body: undefined,
      tone: 'success',
    });
  });

  it('appends a scoped coverage summary with a trailing period when the latest run covered a known scope', () => {
    // Exercises the final `return \`${coverageSummary}.\`` arm with the
    // "Checked N scoped resources" form of getPatrolRunCoverageSummary.
    const result = getPatrolFindingsEmptyState({
      filter: 'active',
      overallHealth: HEALTHY_OVERALL,
      runtimeState: 'active',
      runs: [
        makeRun({
          id: 'run-full-scope',
          resources_checked: 5,
          scope_resource_ids: ['r1', 'r2', 'r3', 'r4', 'r5'],
          effective_scope_resource_ids: ['r1', 'r2', 'r3', 'r4', 'r5'],
        }),
      ],
    });
    expect(result).toStrictEqual({
      title: 'No current issues',
      body: 'Checked 5 scoped resources.',
      tone: 'success',
    });
  });
});
