import { describe, expect, it } from 'vitest';
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
      text: 'No patrol runs yet. Trigger a run to populate history.',
    });
  });

  it('returns investigation section loading and empty states', () => {
    expect(getInvestigationSectionState(true, false)).toEqual({
      text: 'Loading investigation...',
      empty: false,
    });
    expect(getInvestigationSectionState(false, false)).toEqual({
      text: 'No investigation data available. Enable patrol autonomy to investigate findings.',
      empty: true,
    });
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
                'Patrol coverage is incomplete: recent activity was limited to scoped runs and ended with errors, so overall health is not fully verified.',
              category: 'coverage',
            },
          ],
          prediction:
            'Patrol coverage is incomplete: recent activity was limited to scoped runs and ended with errors, so overall health is not fully verified.',
        },
        runtimeState: 'active',
      }),
    ).toEqual({
      title: 'No active findings',
      body: 'Patrol coverage is incomplete: recent activity was limited to scoped runs and ended with errors, so overall health is not fully verified.',
      tone: 'warning',
    });
  });

  it('returns the patrol runtime explanation when the runtime is blocked', () => {
    expect(
      getPatrolFindingsEmptyState({
        filter: 'active',
        runtimeState: 'blocked',
        blockedReason: 'Quickstart credits exhausted. Connect your API key to continue using AI Patrol.',
      }),
    ).toEqual({
      title: 'Patrol paused',
      body: 'Quickstart credits exhausted. Connect your API key to continue using AI Patrol.',
      tone: 'warning',
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
      title: 'No active findings',
      body: 'Your infrastructure looks healthy!',
      tone: 'success',
    });
  });
});
