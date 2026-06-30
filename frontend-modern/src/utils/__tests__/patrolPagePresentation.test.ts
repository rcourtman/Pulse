import { describe, expect, it } from 'vitest';
import {
  PATROL_PAGE_DESCRIPTION,
  PATROL_PAGE_MONITOR_DESCRIPTION,
  PATROL_PAGE_TITLE,
  PATROL_PAGE_TITLE_TOOLTIP,
  PATROL_PAGE_WATCH_ONLY_DESCRIPTION,
  getPatrolPageHeaderMeta,
} from '@/utils/patrolPagePresentation';

describe('patrolPagePresentation', () => {
  it('returns canonical Patrol page header metadata in simple operator language', () => {
    // The page header is the most visible piece of operator-facing copy on
    // the Patrol surface. It must name the product boundary without making
    // users learn the full operations-loop model.
    expect(PATROL_PAGE_TITLE).toBe('Patrol');
    expect(PATROL_PAGE_DESCRIPTION).toBe(
      'Patrol checks your infrastructure, explains what it found, follows your mode before acting, and records the result.',
    );
    expect(PATROL_PAGE_TITLE_TOOLTIP).toBe(PATROL_PAGE_DESCRIPTION);
    expect(getPatrolPageHeaderMeta()).toEqual({
      title: 'Patrol',
      description:
        'Patrol checks your infrastructure, explains what it found, follows your mode before acting, and records the result.',
      titleTooltip:
        'Patrol checks your infrastructure, explains what it found, follows your mode before acting, and records the result.',
    });
  });

  it('separates locked watch-only capability from Pro Watch only mode', () => {
    expect(PATROL_PAGE_WATCH_ONLY_DESCRIPTION).toBe(
      'Patrol checks your infrastructure and shows current issues.',
    );
    expect(PATROL_PAGE_MONITOR_DESCRIPTION).toBe(
      'Watch only: Patrol checks infrastructure and reports issues only.',
    );
    expect(getPatrolPageHeaderMeta({ autonomyLocked: true })).toEqual({
      title: 'Patrol',
      description: PATROL_PAGE_WATCH_ONLY_DESCRIPTION,
      titleTooltip: PATROL_PAGE_WATCH_ONLY_DESCRIPTION,
    });
    expect(getPatrolPageHeaderMeta({ autonomyLevel: 'monitor' })).toEqual({
      title: 'Patrol',
      description: PATROL_PAGE_MONITOR_DESCRIPTION,
      titleTooltip: PATROL_PAGE_MONITOR_DESCRIPTION,
    });
  });

  it('describes each governed Patrol mode level by what it can actually do', () => {
    expect(getPatrolPageHeaderMeta({ autonomyLevel: 'approval' })).toMatchObject({
      description:
        'Ask first: Patrol investigates and prepares fixes, but every change waits for approval.',
    });
    expect(getPatrolPageHeaderMeta({ autonomyLevel: 'assisted' })).toMatchObject({
      description:
        'Safe auto-fix: Patrol may run low- or medium-risk fixes allowed by policy.',
    });
    expect(getPatrolPageHeaderMeta({ autonomyLevel: 'full' })).toMatchObject({
      description:
        'Autopilot: Patrol may act automatically within policy and still asks when approval is required.',
    });
  });
});
