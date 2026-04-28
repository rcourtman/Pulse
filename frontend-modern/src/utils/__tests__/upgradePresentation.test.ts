import { describe, expect, it } from 'vitest';
import {
  getUpgradeActionButtonClass,
  UPGRADE_ACTION_LABEL,
} from '@/utils/upgradePresentation';

describe('upgradePresentation', () => {
  it('returns canonical upgrade labels', () => {
    expect(UPGRADE_ACTION_LABEL).toBe('View plans');
  });

  it('returns the primary upgrade action button classes by default', () => {
    const classes = getUpgradeActionButtonClass();
    expect(classes).toContain('bg-blue-600');
    expect(classes).toContain('w-full sm:w-auto');
    expect(classes).toContain('text-white');
  });

  it('returns the warning upgrade action button classes when requested', () => {
    const classes = getUpgradeActionButtonClass({ tone: 'warning', mobileFullWidth: false });
    expect(classes).toContain('border-amber-300');
    expect(classes).toContain('bg-amber-100');
    expect(classes).toContain('w-auto');
  });
});
