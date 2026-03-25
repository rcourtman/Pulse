import { describe, expect, it } from 'vitest';
import {
  getPatrolAssessmentPresentation,
  getPatrolNoIssuesPresentation,
  getPatrolSummaryPresentation,
  PATROL_NO_ISSUES_LABEL,
} from '@/utils/patrolSummaryPresentation';

describe('getPatrolSummaryPresentation', () => {
  it('returns active critical presentation', () => {
    expect(getPatrolSummaryPresentation('critical', true)).toEqual({
      iconClass: 'text-red-500 dark:text-red-400',
      iconContainerClass: 'bg-red-50 dark:bg-red-900 border-red-200 dark:border-red-800',
      valueClass: 'text-red-600 dark:text-red-400',
    });
  });

  it('returns quiet presentation when the metric is zero', () => {
    expect(getPatrolSummaryPresentation('warning', false)).toEqual({
      iconClass: 'text-muted',
      iconContainerClass: 'bg-surface border-border',
      valueClass: 'text-muted',
    });
  });

  it('exports canonical patrol empty-state copy', () => {
    expect(PATROL_NO_ISSUES_LABEL).toBe('No issues found');
  });

  it('promotes coverage gaps into the primary patrol assessment state', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: {
          score: 70,
          grade: 'C',
          trend: 'stable',
          factors: [
            {
              name: 'Patrol coverage incomplete',
              impact: -0.35,
              description: 'Patrol coverage is incomplete.',
              category: 'coverage',
            },
          ],
          prediction: 'Patrol coverage is incomplete.',
        },
      }),
    ).toEqual({
      title: 'Coverage incomplete',
      eyebrow: 'Patrol assessment',
      compactLabel: 'Coverage incomplete',
      tone: 'warning',
    });
  });

  it('reports issues detected when active warning findings exist', () => {
    expect(
      getPatrolAssessmentPresentation({
        overallHealth: {
          score: 84,
          grade: 'B',
          trend: 'stable',
          factors: [],
          prediction: 'Warnings require review.',
        },
        warningFindings: 2,
      }),
    ).toEqual({
      title: 'Issues detected',
      eyebrow: 'Patrol assessment',
      compactLabel: 'Issues detected',
      tone: 'warning',
    });
  });

  it('keeps no-issues copy only for fully healthy patrol states', () => {
    expect(
      getPatrolNoIssuesPresentation({
        overallHealth: {
          score: 100,
          grade: 'A',
          trend: 'stable',
          factors: [],
          prediction: 'Infrastructure is healthy with no significant issues detected.',
        },
      }),
    ).toEqual({
      label: 'No issues found',
      tone: 'success',
    });
  });
});
