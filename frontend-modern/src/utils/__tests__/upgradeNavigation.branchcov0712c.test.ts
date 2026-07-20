import { describe, expect, it, vi } from 'vitest';
import {
  navigateToUpgradeDestination,
  openExternalUpgradeDestination,
} from '@/utils/upgradeNavigation';
import type { UpgradeDestination } from '@/utils/upgradeNavigation';

describe('upgradeNavigation (branch coverage)', () => {
  describe('openExternalUpgradeDestination', () => {
    it('skips window.open under SSR (typeof window === "undefined") and opens when window is present', () => {
      const openSpy = vi.fn();

      // SSR guard: typeof window === 'undefined' -> early return before window.open.
      // If the guard were absent, `undefined.open(...)` would throw.
      vi.stubGlobal('window', undefined);
      expect(() =>
        openExternalUpgradeDestination('https://example.com/pricing', false),
      ).not.toThrow();
      vi.unstubAllGlobals();
      // The held spy was never wired during the SSR call -> concrete proof the guard fired.
      expect(openSpy).toHaveBeenCalledTimes(0);

      // Contrast: with window present the same call reaches window.open with the
      // safe new-tab policy.
      vi.stubGlobal('window', { open: openSpy });
      openExternalUpgradeDestination('https://example.com/pricing', false);
      vi.unstubAllGlobals();
      expect(openSpy).toHaveBeenCalledTimes(1);
      expect(openSpy).toHaveBeenLastCalledWith(
        'https://example.com/pricing',
        '_blank',
        'noopener,noreferrer',
      );
    });

    it('omits the noopener,noreferrer feature string when preserveOpener=true (preserveOpener truthy arm)', () => {
      const openSpy = vi.fn();
      vi.stubGlobal('window', { open: openSpy });

      openExternalUpgradeDestination('/auth/license-purchase-start?feature=relay', true);

      vi.unstubAllGlobals();
      // preserveOpener truthy branch: window.open is called with exactly two
      // args — the third "noopener,noreferrer" arg must NOT be present, so the
      // opened page can post messages back to refresh the opener.
      expect(openSpy).toHaveBeenCalledTimes(1);
      expect(openSpy).toHaveBeenCalledWith('/auth/license-purchase-start?feature=relay', '_blank');
      expect(openSpy).toHaveBeenNthCalledWith(
        1,
        '/auth/license-purchase-start?feature=relay',
        '_blank',
      );
    });
  });

  describe('navigateToUpgradeDestination', () => {
    it('honors an explicit newTab=true even when external is false (?? left arm for newTab)', () => {
      const navigate = vi.fn();
      const openExternal = vi.fn();
      const destination: UpgradeDestination = {
        href: '/cloud',
        external: false,
        newTab: true,
      };

      navigateToUpgradeDestination(destination, navigate, openExternal);

      // newTab=true overrides external=false -> opens externally; preserveOpener
      // omitted so the ?? falls back to false.
      expect(openExternal).toHaveBeenCalledWith('/cloud', false);
      expect(navigate).not.toHaveBeenCalled();
    });

    it('honors explicit newTab=false and hardNavigation=false even when external is true (?? left arms, navigate fall-through)', () => {
      const navigate = vi.fn();
      const openExternal = vi.fn();
      const hardNavigate = vi.fn();
      const destination: UpgradeDestination = {
        href: 'https://example.com/pricing',
        external: true,
        newTab: false,
        hardNavigation: false,
      };

      navigateToUpgradeDestination(destination, navigate, openExternal, hardNavigate);

      expect(openExternal).not.toHaveBeenCalled();
      expect(hardNavigate).not.toHaveBeenCalled();
      expect(navigate).toHaveBeenCalledWith('https://example.com/pricing');
    });

    it('routes through hardNavigate when hardNavigation=true and newTab=false (hardNavigation true arm)', () => {
      const navigate = vi.fn();
      const openExternal = vi.fn();
      const hardNavigate = vi.fn();
      const destination: UpgradeDestination = {
        href: '/reload-me',
        external: false,
        newTab: false,
        hardNavigation: true,
      };

      navigateToUpgradeDestination(destination, navigate, openExternal, hardNavigate);

      expect(hardNavigate).toHaveBeenCalledWith('/reload-me');
      expect(openExternal).not.toHaveBeenCalled();
      expect(navigate).not.toHaveBeenCalled();
    });

    it('falls back to external for hardNavigation when hardNavigation is omitted and newTab=false (?? right arm)', () => {
      const navigate = vi.fn();
      const openExternal = vi.fn();
      const hardNavigate = vi.fn();
      const destination: UpgradeDestination = {
        href: 'https://example.com/x',
        external: true,
        newTab: false,
        // hardNavigation omitted -> ?? falls back to external=true -> hard navigate.
      };

      navigateToUpgradeDestination(destination, navigate, openExternal, hardNavigate);

      expect(hardNavigate).toHaveBeenCalledWith('https://example.com/x');
      expect(openExternal).not.toHaveBeenCalled();
      expect(navigate).not.toHaveBeenCalled();
    });

    it('forwards preserveOpener=true to openExternal when opening in a new tab (?? left arm for preserveOpener)', () => {
      const navigate = vi.fn();
      const openExternal = vi.fn();
      const destination: UpgradeDestination = {
        href: '/auth/license-purchase-start',
        external: false,
        newTab: true,
        preserveOpener: true,
      };

      navigateToUpgradeDestination(destination, navigate, openExternal);

      expect(openExternal).toHaveBeenCalledWith('/auth/license-purchase-start', true);
      expect(navigate).not.toHaveBeenCalled();
    });

    it('uses the default openExternalUpgradeDestination when openExternal is omitted (default param)', () => {
      const openSpy = vi.fn();
      vi.stubGlobal('window', { open: openSpy });
      const navigate = vi.fn();
      const destination: UpgradeDestination = {
        href: 'https://example.com/pricing',
        external: true,
        // openExternal omitted -> default openExternalUpgradeDestination runs.
      };

      navigateToUpgradeDestination(destination, navigate);

      vi.unstubAllGlobals();
      expect(openSpy).toHaveBeenCalledWith(
        'https://example.com/pricing',
        '_blank',
        'noopener,noreferrer',
      );
      expect(navigate).not.toHaveBeenCalled();
    });

    it('uses the default hardNavigate (window.location.assign) and respects its SSR guard', () => {
      // jsdom's window.location is a non-configurable own property, so swap the
      // whole descriptor (same pattern as pricingHandoff tests) and restore it
      // afterwards.
      const originalDescriptor = Object.getOwnPropertyDescriptor(window, 'location');
      const assignSpy = vi.fn();
      Object.defineProperty(window, 'location', {
        configurable: true,
        value: { assign: assignSpy },
      });
      try {
        const navigate = vi.fn();
        const openExternal = vi.fn();
        const destination: UpgradeDestination = {
          href: '/hard-reload',
          external: false,
          newTab: false,
          hardNavigation: true,
          // hardNavigate omitted -> default closure calls window.location.assign.
        };

        navigateToUpgradeDestination(destination, navigate, openExternal);

        // Default hardNavigate, window present -> assign reached.
        expect(assignSpy).toHaveBeenCalledWith('/hard-reload');
        expect(assignSpy).toHaveBeenCalledTimes(1);
        expect(openExternal).not.toHaveBeenCalled();
        expect(navigate).not.toHaveBeenCalled();

        // SSR guard inside the default hardNavigate: typeof window !== 'undefined'
        // is false, so assign is skipped and the count is unchanged.
        vi.stubGlobal('window', undefined);
        navigateToUpgradeDestination(
          { href: '/hard-reload-2', external: false, newTab: false, hardNavigation: true },
          navigate,
          openExternal,
        );
        vi.unstubAllGlobals();

        expect(assignSpy).toHaveBeenCalledTimes(1);
      } finally {
        if (originalDescriptor) {
          Object.defineProperty(window, 'location', originalDescriptor);
        }
      }
    });
  });
});
