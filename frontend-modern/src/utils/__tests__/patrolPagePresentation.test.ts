import { describe, expect, it } from 'vitest';
import {
  PATROL_PAGE_DESCRIPTION,
  PATROL_PAGE_TITLE,
  PATROL_PAGE_TITLE_TOOLTIP,
  getPatrolPageHeaderMeta,
} from '@/utils/patrolPagePresentation';

describe('patrolPagePresentation', () => {
  it('returns canonical Patrol page header metadata that names the proactive trust loop', () => {
    // The page header is the most visible piece of operator-facing copy on
    // the Patrol surface. It must keep the product boundary clear: Pulse
    // probes and assembles evidence, the configured model reasons over it,
    // and action stays approval-bound.
    expect(PATROL_PAGE_TITLE).toBe('Patrol');
    expect(PATROL_PAGE_DESCRIPTION).toBe(
      'Pulse checks your infrastructure on a schedule, gives your configured model the right evidence, and keeps fixes behind your approval policy.',
    );
    // Tooltip must share the same framing so hover and inline don't tell
    // different stories about what Patrol does.
    expect(PATROL_PAGE_TITLE_TOOLTIP).toBe(PATROL_PAGE_DESCRIPTION);
    expect(getPatrolPageHeaderMeta()).toEqual({
      title: 'Patrol',
      description:
        'Pulse checks your infrastructure on a schedule, gives your configured model the right evidence, and keeps fixes behind your approval policy.',
      titleTooltip:
        'Pulse checks your infrastructure on a schedule, gives your configured model the right evidence, and keeps fixes behind your approval policy.',
    });
  });
});
