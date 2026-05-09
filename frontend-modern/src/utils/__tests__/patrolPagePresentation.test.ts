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
    // the Patrol surface. It must name the loop the page actually owns —
    // investigation, evidence capture, governed action — instead of the
    // prior passive "verify, review, control" framing. Pin the exact
    // wording so the IA reframe doesn't silently regress.
    expect(PATROL_PAGE_TITLE).toBe('Patrol');
    expect(PATROL_PAGE_DESCRIPTION).toBe(
      'Pulse investigates your infrastructure, gathers evidence for every finding, and proposes safe fixes under your approval policy.',
    );
    // Tooltip must share the same framing so hover and inline don't tell
    // different stories about what Patrol does.
    expect(PATROL_PAGE_TITLE_TOOLTIP).toBe(PATROL_PAGE_DESCRIPTION);
    expect(getPatrolPageHeaderMeta()).toEqual({
      title: 'Patrol',
      description:
        'Pulse investigates your infrastructure, gathers evidence for every finding, and proposes safe fixes under your approval policy.',
      titleTooltip:
        'Pulse investigates your infrastructure, gathers evidence for every finding, and proposes safe fixes under your approval policy.',
    });
  });
});
