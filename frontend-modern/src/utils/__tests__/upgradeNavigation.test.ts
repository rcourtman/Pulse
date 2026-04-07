import { describe, expect, it, vi } from 'vitest';
import {
  isExternalUpgradeHref,
  navigateToUpgradeDestination,
  openExternalUpgradeDestination,
  resolveUpgradeDestination,
} from '@/utils/upgradeNavigation';

describe('upgradeNavigation', () => {
  it('classifies external upgrade hrefs without treating product routes as external', () => {
    expect(isExternalUpgradeHref('https://pulserelay.pro/pricing')).toBe(true);
    expect(isExternalUpgradeHref('http://127.0.0.1:3000/upgrade')).toBe(true);
    expect(isExternalUpgradeHref('/settings/system/billing')).toBe(false);
    expect(isExternalUpgradeHref('   /cloud  ')).toBe(false);
    expect(isExternalUpgradeHref('')).toBe(false);
    expect(isExternalUpgradeHref(undefined)).toBe(false);
  });

  it('normalizes upgrade destinations and keeps the trimmed href', () => {
    expect(resolveUpgradeDestination('  /cloud  ')).toEqual({
      href: '/cloud',
      external: false,
      hardNavigation: false,
      newTab: false,
      preserveOpener: false,
    });
    expect(resolveUpgradeDestination(' https://pulserelay.pro/pricing ')).toEqual({
      href: 'https://pulserelay.pro/pricing',
      external: true,
      hardNavigation: true,
      newTab: true,
      preserveOpener: false,
    });
  });

  it('routes internal destinations through the provided navigate callback', () => {
    const navigate = vi.fn();
    const openExternal = vi.fn();

    navigateToUpgradeDestination(
      { href: '/settings/system/billing', external: false },
      navigate,
      openExternal,
    );

    expect(navigate).toHaveBeenCalledWith('/settings/system/billing');
    expect(openExternal).not.toHaveBeenCalled();
  });

  it('routes external destinations through the provided external opener', () => {
    const navigate = vi.fn();
    const openExternal = vi.fn();

    navigateToUpgradeDestination(
      { href: 'https://pulserelay.pro/pricing?feature=relay', external: true },
      navigate,
      openExternal,
    );

    expect(openExternal).toHaveBeenCalledWith(
      'https://pulserelay.pro/pricing?feature=relay',
      false,
    );
    expect(navigate).not.toHaveBeenCalled();
  });

  it('does nothing for empty destinations', () => {
    const navigate = vi.fn();
    const openExternal = vi.fn();

    navigateToUpgradeDestination({ href: '', external: false }, navigate, openExternal);

    expect(navigate).not.toHaveBeenCalled();
    expect(openExternal).not.toHaveBeenCalled();
  });

  it('opens external upgrade destinations with a safe new-tab policy', () => {
    const windowOpen = vi.fn();
    vi.stubGlobal('window', { open: windowOpen });

    openExternalUpgradeDestination('https://pulserelay.pro/pricing?feature=relay');

    expect(windowOpen).toHaveBeenCalledWith(
      'https://pulserelay.pro/pricing?feature=relay',
      '_blank',
      'noopener,noreferrer',
    );
  });

  it('opens preserve-opener destinations without noopener so the return bridge can refresh the opener', () => {
    const windowOpen = vi.fn();
    vi.stubGlobal('window', { open: windowOpen });

    openExternalUpgradeDestination('/auth/license-purchase-start?feature=relay', true);

    expect(windowOpen).toHaveBeenCalledWith(
      '/auth/license-purchase-start?feature=relay',
      '_blank',
    );
  });
});
