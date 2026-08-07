import { describe, expect, it } from 'vitest';
import { getSettingsNavItem } from '../settingsNavCatalog';
import {
  shouldBlockSettingsRouteItem,
  shouldHideSettingsNavItem,
  type SettingsNavVisibilityContext,
} from '../settingsNavVisibility';

// System → Network, Pulse server updates, and Recovery are pure instance
// administration: the public URL and CORS boundaries, the server update
// channel, and backup polling plus config export/import. Every route behind
// them is RequireAdmin + settings:read, so a session without it was offered
// three tabs it could open and never populate. This is the sibling of the
// infrastructure-systems gate; the two capabilities share a derivation but
// name different surfaces so tightening one cannot silently hide the other.

const SYSTEM_ADMIN_TABS = ['system-network', 'system-updates', 'system-recovery'] as const;

const createContext = (
  overrides: Partial<SettingsNavVisibilityContext> = {},
): SettingsNavVisibilityContext => ({
  hasFeature: () => false,
  runtimeCapabilitiesLoaded: () => true,
  hostedModeEnabled: false,
  ...overrides,
});

describe('admin-only System tab capability gate', () => {
  it('declares systemSettingsRead as the required capability on each tab', () => {
    for (const tab of SYSTEM_ADMIN_TABS) {
      expect(getSettingsNavItem(tab)?.requiredCapability).toBe('systemSettingsRead');
    }
  });

  it('hides the nav items and blocks their routes when the capability is withheld', () => {
    const context = createContext({
      settingsCapabilitiesResolved: true,
      settingsCapabilities: { systemSettingsRead: false },
    });

    for (const tab of SYSTEM_ADMIN_TABS) {
      expect(shouldHideSettingsNavItem(tab, context)).toBe(true);
      expect(shouldBlockSettingsRouteItem(tab, context)).toBe(true);
    }
  });

  it('keeps the nav items and routes for a session that holds the capability', () => {
    const context = createContext({
      settingsCapabilitiesResolved: true,
      settingsCapabilities: { systemSettingsRead: true },
    });

    for (const tab of SYSTEM_ADMIN_TABS) {
      expect(shouldHideSettingsNavItem(tab, context)).toBe(false);
      expect(shouldBlockSettingsRouteItem(tab, context)).toBe(false);
    }
  });

  it('does not hide the tabs before the security status resolves', () => {
    // Hiding on an unresolved status would flash three tabs away from an admin
    // on every load.
    const context = createContext({
      settingsCapabilitiesResolved: false,
      settingsCapabilities: null,
    });

    for (const tab of SYSTEM_ADMIN_TABS) {
      expect(shouldHideSettingsNavItem(tab, context)).toBe(false);
      expect(shouldBlockSettingsRouteItem(tab, context)).toBe(false);
    }
  });

  it('leaves System → General reachable without the capability', () => {
    // Theme, language, and unit preferences there are user-scoped, so gating
    // the tab would take personal settings away from every non-admin.
    const context = createContext({
      settingsCapabilitiesResolved: true,
      settingsCapabilities: { systemSettingsRead: false },
    });

    expect(getSettingsNavItem('system-general')?.requiredCapability).toBeUndefined();
    expect(shouldHideSettingsNavItem('system-general', context)).toBe(false);
    expect(shouldBlockSettingsRouteItem('system-general', context)).toBe(false);
  });
});
