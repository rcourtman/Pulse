import { describe, expect, it } from 'vitest';
import runHistoryPanelSource from '@/components/patrol/RunHistoryPanel.tsx?raw';
import patrolEmptyStatePresentationSource from '@/utils/patrolEmptyStatePresentation.ts?raw';
import type { PatrolRunRecord } from '@/api/patrol';
import {
  getInvestigationMessagesState,
  getInvestigationSectionState,
  getPatrolFindingsEmptyState,
  getRunHistoryEmptyState,
} from '@/utils/patrolEmptyStatePresentation';

describe('patrolEmptyStatePresentation', () => {
  it('returns investigation message loading and empty states', () => {
    expect(getInvestigationMessagesState(true, false)).toEqual({
      text: 'Loading messages...',
      empty: false,
    });
    expect(getInvestigationMessagesState(false, false)).toEqual({
      text: 'No investigation messages available.',
      empty: true,
    });
  });

  it('returns the patrol run history empty state', () => {
    expect(getRunHistoryEmptyState()).toEqual({
      text: 'No Patrol checks yet. Run Patrol to start history.',
    });
    expect(runHistoryPanelSource).toContain('@/components/shared/EmptyState');
    expect(runHistoryPanelSource).toContain('variant="panel"');
    expect(runHistoryPanelSource).not.toContain('text-center py-8');
  });

  it('returns investigation section loading and empty states', () => {
    expect(getInvestigationSectionState(true, false)).toEqual({
      text: 'Loading investigation...',
      empty: false,
    });
    expect(getInvestigationSectionState(false, false)).toEqual({
      text: 'No investigation yet. Patrol adds notes after it runs in a mode that investigates.',
      empty: true,
    });
  });

  it('keeps the active Patrol empty queue out of all-clear wording', () => {
    expect(patrolEmptyStatePresentationSource).not.toContain('Nothing needs attention');
    expect(patrolEmptyStatePresentationSource).not.toContain('all clear');
  });

  it('suppresses the healthy findings empty state when patrol health is not fully verified', () => {
    expect(
      getPatrolFindingsEmptyState({
        filter: 'active',
        overallHealth: {
          score: 70,
          grade: 'C',
          trend: 'stable',
          factors: [
            {
              name: 'Patrol coverage incomplete',
              impact: -0.35,
              description:
                'Recent Patrol activity only covered targeted checks and ended with errors. Run Patrol to check everything.',
              category: 'coverage',
            },
          ],
          prediction:
            'Recent Patrol activity only covered targeted checks and ended with errors. Run Patrol to check everything.',
        },
        runtimeState: 'active',
      }),
    ).toEqual({
      title: 'Check needed',
      body: 'Run Patrol to check everything and refresh open work.',
      tone: 'warning',
    });
  });

  it('uses run history to keep incomplete coverage visible when health summary is unavailable', () => {
    expect(
      getPatrolFindingsEmptyState({
        filter: 'active',
        runtimeState: 'active',
        runs: [
          {
            id: 'run-errored-full',
            started_at: '2026-05-06T12:00:00Z',
            completed_at: '2026-05-06T12:01:00Z',
            type: 'full',
            status: 'error',
            error_count: 1,
            resources_checked: 72,
          } as unknown as PatrolRunRecord,
        ],
      }),
    ).toEqual({
      title: 'Check needed',
      body: 'Run Patrol to check everything and refresh open work.',
      tone: 'warning',
    });
  });

  describe('coverage factor after a clean full run', () => {
    // Ten minutes after the clean full run below completed.
    const NOW = Date.parse('2026-10-07T12:15:00Z');
    const recentErrorsFactor = (impact: number) => ({
      name: 'Recent Patrol errors',
      impact,
      description:
        'Recent Patrol runs encountered errors, so the current health summary may be incomplete.',
      category: 'coverage',
    });
    const fullRun = (overrides: Partial<PatrolRunRecord>) =>
      ({
        id: 'run-full',
        started_at: '2026-10-07T12:00:00Z',
        completed_at: '2026-10-07T12:05:00Z',
        type: 'patrol',
        status: 'healthy',
        error_count: 0,
        resources_checked: 1377,
        finding_ids: [],
        ...overrides,
      }) as unknown as PatrolRunRecord;
    const erroredEarlierRun = fullRun({
      id: 'run-errored-earlier',
      started_at: '2026-10-07T06:00:00Z',
      completed_at: '2026-10-07T06:05:00Z',
      status: 'error',
      error_count: 2,
    });

    it('reads as no current issues once the latest full run checked resources cleanly', () => {
      expect(
        getPatrolFindingsEmptyState({
          filter: 'active',
          overallHealth: {
            score: 90,
            grade: 'A',
            trend: 'stable',
            factors: [recentErrorsFactor(-0.1)],
            prediction:
              'Recent Patrol runs encountered errors, so the current health summary may be incomplete.',
          },
          runtimeState: 'active',
          nowMs: NOW,
          runs: [fullRun({}), erroredEarlierRun],
        }),
      ).toEqual({
        title: 'No current issues',
        body: 'Checked 1377 resources.',
        tone: 'success',
      });
    });

    it('does not ask for review when the coverage factor alone lowered the grade', () => {
      expect(
        getPatrolFindingsEmptyState({
          filter: 'active',
          overallHealth: {
            score: 70,
            grade: 'C',
            trend: 'stable',
            factors: [
              recentErrorsFactor(-0.3),
              {
                name: 'Knowledge learned',
                impact: 0.05,
                description: 'Pulse Patrol has learned about 12 resources',
                category: 'learning',
              },
            ],
            prediction:
              'Most recent Patrol runs encountered errors (6 of 10); the current health summary is not reliable until coverage stabilizes.',
          },
          runtimeState: 'active',
          nowMs: NOW,
          runs: [fullRun({}), erroredEarlierRun],
        }),
      ).toEqual({
        title: 'No current issues',
        body: 'Checked 1377 resources.',
        tone: 'success',
      });
    });

    it('still asks for review when another factor lowered the grade', () => {
      expect(
        getPatrolFindingsEmptyState({
          filter: 'active',
          overallHealth: {
            score: 70,
            grade: 'C',
            trend: 'declining',
            factors: [
              recentErrorsFactor(-0.2),
              {
                name: 'Predicted issue',
                impact: -0.1,
                description: 'disk_full predicted within 2.0 days',
                category: 'prediction',
              },
            ],
            prediction: 'Recent Patrol runs encountered errors (2 of 5).',
          },
          runtimeState: 'active',
          nowMs: NOW,
          runs: [fullRun({}), erroredEarlierRun],
        }),
      ).toEqual({
        title: 'Patrol needs review',
        body: 'No current issues are listed, but Patrol health needs review.',
        tone: 'warning',
      });
    });

    it('keeps the coverage caveat when the clean full run checked no resources', () => {
      expect(
        getPatrolFindingsEmptyState({
          filter: 'active',
          overallHealth: {
            score: 90,
            grade: 'A',
            trend: 'stable',
            factors: [recentErrorsFactor(-0.1)],
            prediction: '',
          },
          runtimeState: 'active',
          nowMs: NOW,
          runs: [fullRun({ resources_checked: 0 }), erroredEarlierRun],
        }),
      ).toEqual({
        title: 'Check needed',
        body: 'Run Patrol to check everything and refresh open work.',
        tone: 'warning',
      });
    });

    it('keeps the coverage caveat when only targeted runs completed', () => {
      expect(
        getPatrolFindingsEmptyState({
          filter: 'active',
          overallHealth: {
            score: 100,
            grade: 'A',
            trend: 'stable',
            factors: [],
            prediction: 'Infrastructure is healthy with no significant issues detected.',
          },
          runtimeState: 'active',
          nowMs: NOW,
          runs: [fullRun({ type: 'scoped', resources_checked: 3 })],
        }),
      ).toEqual({
        title: 'Check needed',
        body: 'Run Patrol to check everything and refresh open work.',
        tone: 'warning',
      });
    });
    it('keeps the coverage caveat when a targeted run failed after the clean full run', () => {
      expect(
        getPatrolFindingsEmptyState({
          filter: 'active',
          overallHealth: {
            score: 90,
            grade: 'A',
            trend: 'stable',
            factors: [recentErrorsFactor(-0.1)],
            prediction: '',
          },
          runtimeState: 'active',
          nowMs: NOW,
          runs: [
            fullRun({
              id: 'run-scoped-failed',
              type: 'scoped',
              started_at: '2026-10-07T12:08:00Z',
              completed_at: '2026-10-07T12:09:00Z',
              resources_checked: 0,
              status: 'error',
              error_count: 1,
            }),
            fullRun({}),
          ],
        }),
      ).toEqual({
        title: 'Check needed',
        body: 'Run Patrol to check everything and refresh open work.',
        tone: 'warning',
      });
    });

    it('asks for a check when a targeted run failed after the clean full run, even without a coverage factor', () => {
      expect(
        getPatrolFindingsEmptyState({
          filter: 'active',
          overallHealth: {
            score: 100,
            grade: 'A',
            trend: 'stable',
            factors: [],
            prediction: 'Infrastructure is healthy with no significant issues detected.',
          },
          runtimeState: 'active',
          nowMs: NOW,
          runs: [
            fullRun({
              id: 'run-scoped-failed',
              type: 'scoped',
              started_at: '2026-10-07T12:08:00Z',
              completed_at: '2026-10-07T12:09:00Z',
              resources_checked: 0,
              status: 'error',
              error_count: 1,
            }),
            fullRun({}),
          ],
        }),
      ).toEqual({
        title: 'Check needed',
        body: 'Run Patrol to check everything and refresh open work.',
        tone: 'warning',
      });
    });

    it('keeps the coverage caveat when the clean full run is older than a day', () => {
      expect(
        getPatrolFindingsEmptyState({
          filter: 'active',
          overallHealth: {
            score: 80,
            grade: 'B',
            trend: 'stable',
            factors: [
              {
                name: 'Patrol coverage incomplete',
                impact: -0.2,
                description:
                  'Recent Patrol activity only covered targeted checks. Run Patrol to check everything.',
                category: 'coverage',
              },
            ],
            prediction: '',
          },
          runtimeState: 'active',
          nowMs: NOW + 2 * 24 * 60 * 60 * 1000,
          runs: [
            fullRun({
              id: 'run-scoped-recent',
              type: 'scoped',
              started_at: '2026-10-09T12:00:00Z',
              completed_at: '2026-10-09T12:01:00Z',
              resources_checked: 2,
            }),
            fullRun({}),
          ],
        }),
      ).toEqual({
        title: 'Check needed',
        body: 'Run Patrol to check everything and refresh open work.',
        tone: 'warning',
      });
    });
  });

  it('uses an attention-focused empty state when patrol health is degraded for non-coverage reasons', () => {
    expect(
      getPatrolFindingsEmptyState({
        filter: 'active',
        overallHealth: {
          score: 45,
          grade: 'D',
          trend: 'declining',
          factors: [
            {
              name: 'Critical unresolved risk',
              impact: -0.55,
              description: 'Recent Patrol evidence indicates unresolved infrastructure risk.',
              category: 'findings',
            },
          ],
          prediction: 'Critical infrastructure risk still requires attention.',
        },
        runtimeState: 'active',
      }),
    ).toEqual({
      title: 'Patrol needs review',
      body: 'No current issues are listed, but Patrol health needs review.',
      tone: 'error',
    });
  });

  it('normalizes retired hosted runtime explanations when the runtime is blocked', () => {
    expect(
      getPatrolFindingsEmptyState({
        filter: 'active',
        runtimeState: 'blocked',
        blockedReason:
          'Quickstart credits exhausted. Connect your API key to continue using Patrol.',
      }),
    ).toEqual({
      title: 'Patrol paused',
      body: 'Connect your own AI provider or local model to use Pulse Patrol.',
      tone: 'warning',
    });
  });

  it('treats a running Patrol as active work instead of an empty state', () => {
    expect(
      getPatrolFindingsEmptyState({
        filter: 'active',
        runtimeState: 'running',
        overallHealth: {
          score: 100,
          grade: 'A',
          trend: 'stable',
          factors: [],
          prediction: 'Infrastructure is healthy with no significant issues detected.',
        },
      }),
    ).toEqual({
      title: 'Patrol is checking now',
      body: 'If Patrol finds an issue or needs approval, it will add it here.',
      tone: 'info',
    });
  });

  it('keeps the healthy findings empty state when patrol health is fully healthy', () => {
    expect(
      getPatrolFindingsEmptyState({
        filter: 'active',
        overallHealth: {
          score: 100,
          grade: 'A',
          trend: 'stable',
          factors: [],
          prediction: 'Infrastructure is healthy with no significant issues detected.',
        },
        runtimeState: 'active',
      }),
    ).toEqual({
      title: 'No current issues',
      tone: 'success',
    });
  });

  it('does not present stale Patrol evidence as a healthy clear state', () => {
    expect(
      getPatrolFindingsEmptyState({
        filter: 'active',
        coverageStale: true,
        overallHealth: {
          score: 100,
          grade: 'A',
          trend: 'stable',
          factors: [],
          prediction: 'Infrastructure is healthy with no significant issues detected.',
        },
        runtimeState: 'active',
      }),
    ).toEqual({
      title: 'Check needed',
      body: 'Patrol coverage is stale. Run Patrol to check current infrastructure state.',
      tone: 'warning',
    });
  });

  it('keeps past regressions informational when there are no current issues', () => {
    expect(
      getPatrolFindingsEmptyState({
        filter: 'active',
        overallHealth: {
          score: 100,
          grade: 'A',
          trend: 'stable',
          factors: [],
          prediction: 'Infrastructure is healthy with no significant issues detected.',
        },
        historicalRegressionCount: 1,
        runtimeState: 'active',
      }),
    ).toEqual({
      title: 'No current issues',
      body: 'Past issues are in History if you need the record.',
      tone: 'info',
    });
  });

  it('describes a selected run snapshot with no recorded findings using coverage-aware copy', () => {
    expect(
      getPatrolFindingsEmptyState({
        filter: 'all',
        runSnapshot: {
          resources_checked: 1,
          scope_resource_ids: ['seed-resource'],
          effective_scope_resource_ids: ['expanded-a', 'expanded-b'],
          finding_ids: [],
          status: 'error',
          error_count: 1,
        },
      }),
    ).toEqual({
      title: 'No findings recorded for this run',
      body: 'Checked 1 of 2 scoped resources. This run recorded no Patrol findings, but it ended with issues requiring review.',
      tone: 'warning',
    });
  });

  it('fails closed when a selected run has no findings snapshot ids', () => {
    expect(
      getPatrolFindingsEmptyState({
        filter: 'all',
        runSnapshot: {
          resources_checked: 1,
          scope_resource_ids: ['seed-resource'],
          effective_scope_resource_ids: ['expanded-a'],
          finding_ids: undefined,
          status: 'issues_found',
          error_count: 0,
        },
      }),
    ).toEqual({
      title: 'Finding record unavailable for this run',
      body: 'This older Patrol run has no finding record, so Patrol cannot show its issue list.',
      tone: 'warning',
    });
  });

  it('includes latest-run coverage context in the healthy empty state body', () => {
    const result = getPatrolFindingsEmptyState({
      filter: 'active',
      overallHealth: {
        score: 100,
        grade: 'A',
        trend: 'stable',
        factors: [],
        prediction: 'Infrastructure is healthy with no significant issues detected.',
      },
      runtimeState: 'active',
      runs: [
        {
          id: 'run-1',
          started_at: '2026-06-27T10:00:00Z',
          status: 'no_issues',
          error_count: 0,
          resources_checked: 42,
          scope_resource_ids: [],
          effective_scope_resource_ids: [],
          finding_ids: [],
        } as unknown as PatrolRunRecord,
      ],
    });
    expect(result.title).toBe('No current issues');
    expect(result.body).toBe('Checked 42 resources.');
    expect(result.tone).toBe('success');
  });

  it('omits coverage context when no runs are available', () => {
    const result = getPatrolFindingsEmptyState({
      filter: 'active',
      overallHealth: {
        score: 100,
        grade: 'A',
        trend: 'stable',
        factors: [],
        prediction: 'Infrastructure is healthy with no significant issues detected.',
      },
      runtimeState: 'active',
    });
    expect(result.body).toBeUndefined();
  });
});
