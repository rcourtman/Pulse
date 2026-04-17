import { describe, expect, it } from 'vitest';
import {
  PATROL_PAGE_DESCRIPTION,
  PATROL_PAGE_TITLE,
  getPatrolPageHeaderMeta,
} from '@/utils/patrolPagePresentation';

describe('patrolPagePresentation', () => {
  it('returns canonical Patrol page header metadata', () => {
    expect(PATROL_PAGE_TITLE).toBe('Patrol');
    expect(PATROL_PAGE_DESCRIPTION).toBe(
      'Continuously verify infrastructure health, review findings, and control Patrol runtime behavior.',
    );
    expect(getPatrolPageHeaderMeta()).toEqual({
      title: 'Patrol',
      description:
        'Continuously verify infrastructure health, review findings, and control Patrol runtime behavior.',
    });
  });
});
