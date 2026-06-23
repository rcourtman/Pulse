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
      'Patrol watches, investigates, acts within your chosen mode, verifies outcomes, and records what happened.',
    );
    expect(PATROL_PAGE_TITLE_TOOLTIP).toBe(PATROL_PAGE_DESCRIPTION);
    expect(getPatrolPageHeaderMeta()).toEqual({
      title: 'Patrol',
      description:
        'Patrol watches, investigates, acts within your chosen mode, verifies outcomes, and records what happened.',
      titleTooltip:
        'Patrol watches, investigates, acts within your chosen mode, verifies outcomes, and records what happened.',
    });
  });

  it('separates locked watch-only capability from Pro Watch only mode', () => {
    expect(PATROL_PAGE_WATCH_ONLY_DESCRIPTION).toBe(
      'Patrol watches infrastructure and shows current issues.',
    );
    expect(PATROL_PAGE_MONITOR_DESCRIPTION).toBe(
      'Watch only: Patrol reports issues without making changes.',
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
      description: 'Patrol investigates issues and asks before every change.',
    });
    expect(getPatrolPageHeaderMeta({ autonomyLevel: 'assisted' })).toMatchObject({
      description: 'Patrol handles safe policy-allowed fixes and asks before anything riskier.',
    });
    expect(getPatrolPageHeaderMeta({ autonomyLevel: 'full' })).toMatchObject({
      description:
        'Patrol handles policy-approved work automatically and asks only when approval is required.',
    });
  });
});
