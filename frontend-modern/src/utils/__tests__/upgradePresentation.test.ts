import { describe, expect, it } from 'vitest';
import { UPGRADE_ACTION_LABEL } from '@/utils/upgradePresentation';

describe('upgradePresentation', () => {
  it('returns canonical upgrade labels', () => {
    expect(UPGRADE_ACTION_LABEL).toBe('View plans');
  });
});
