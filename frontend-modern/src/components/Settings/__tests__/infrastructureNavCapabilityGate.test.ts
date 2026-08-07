import { describe, expect, it } from 'vitest';
import { getSettingsNavItem } from '../settingsNavCatalog';
import {
  shouldBlockSettingsRouteItem,
  shouldHideSettingsNavItem,
  type SettingsNavVisibilityContext,
} from '../settingsNavVisibility';

// Settings → Infrastructure reads /api/connections, /api/config/nodes,
// /api/system/settings, /api/truenas/connections and /api/vmware/connections
// on mount, then polls /api/connections every 15s and /api/discover every 30s.
// All of those are RequireAdmin, so a session without settings:read used to
// render an all-empty page whose pollers reprinted "Non-admin user attempted
// to access admin endpoint" at warn level for as long as the tab stayed open.
// The served `infrastructureRead` capability is what stops the page mounting.

const createContext = (
  overrides: Partial<SettingsNavVisibilityContext> = {},
): SettingsNavVisibilityContext => ({
  hasFeature: () => false,
  runtimeCapabilitiesLoaded: () => true,
  hostedModeEnabled: false,
  ...overrides,
});

describe('infrastructure-systems capability gate', () => {
  it('declares infrastructureRead as its required capability', () => {
    expect(getSettingsNavItem('infrastructure-systems')?.requiredCapability).toBe(
      'infrastructureRead',
    );
  });

  it('hides the nav item and blocks the route when the capability is withheld', () => {
    const context = createContext({
      settingsCapabilitiesResolved: true,
      settingsCapabilities: { infrastructureRead: false },
    });

    expect(shouldHideSettingsNavItem('infrastructure-systems', context)).toBe(true);
    expect(shouldBlockSettingsRouteItem('infrastructure-systems', context)).toBe(true);
  });

  it('keeps the nav item and route for a session that holds the capability', () => {
    const context = createContext({
      settingsCapabilitiesResolved: true,
      settingsCapabilities: { infrastructureRead: true },
    });

    expect(shouldHideSettingsNavItem('infrastructure-systems', context)).toBe(false);
    expect(shouldBlockSettingsRouteItem('infrastructure-systems', context)).toBe(false);
  });

  it('does not hide the page before the security status resolves', () => {
    // Hiding on an unresolved status would flash the default settings tab away
    // from an admin on every load.
    const context = createContext({
      settingsCapabilitiesResolved: false,
      settingsCapabilities: null,
    });

    expect(shouldHideSettingsNavItem('infrastructure-systems', context)).toBe(false);
    expect(shouldBlockSettingsRouteItem('infrastructure-systems', context)).toBe(false);
  });
});
