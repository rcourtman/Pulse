import { describe, expect, it } from 'vitest';
import {
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
});
