import { describe, expect, it } from 'vitest';
import {
  ACTION_REVIEW_QUERY_PARAM,
  buildActionReviewPath,
  parseActionReviewId,
} from '../actionRouting';

describe('action review routing', () => {
  it('builds an exact URL-safe action review handoff', () => {
    expect(buildActionReviewPath(' action/with spaces ')).toBe(
      `/actions?${ACTION_REVIEW_QUERY_PARAM}=action%2Fwith+spaces`,
    );
  });

  it('falls back to the Actions inbox when no exact action is available', () => {
    expect(buildActionReviewPath()).toBe('/actions');
    expect(buildActionReviewPath('   ')).toBe('/actions');
  });

  it('parses and normalizes the exact action review identity', () => {
    expect(parseActionReviewId('?action=%20action-1%20')).toBe('action-1');
    expect(parseActionReviewId('?view=settled')).toBe('');
  });
});
