import { describe, expect, it } from 'vitest';
import { SETTINGS_NAV_GROUPS, getSettingsNavItem } from '../settingsNavCatalog';
import type { SettingsTab } from '../settingsNavigationModel';
import {
  canMountSettingsPanel,
  shouldBlockSettingsRouteItem,
  shouldHideSettingsNavItem,
  type SettingsNavVisibilityContext,
} from '../settingsNavVisibility';

const ADMIN_ONLY_TABS: ReadonlyArray<{
  tab: SettingsTab;
  capability: keyof NonNullable<SettingsNavVisibilityContext['settingsCapabilities']>;
}> = [
  { tab: 'monitoring-availability', capability: 'availabilityRead' },
  { tab: 'system-ai', capability: 'pulseIntelligenceRead' },
  { tab: 'system-ai-patrol', capability: 'pulseIntelligenceRead' },
  { tab: 'system-ai-assistant', capability: 'pulseIntelligenceRead' },
  { tab: 'support-diagnostics', capability: 'diagnosticsRead' },
  { tab: 'support-reporting', capability: 'reportingRead' },
  { tab: 'support-logs', capability: 'systemLogsRead' },
];

const createContext = (
  overrides: Partial<SettingsNavVisibilityContext> = {},
): SettingsNavVisibilityContext => ({
  hasFeature: () => true,
  runtimeCapabilitiesLoaded: () => true,
  hostedModeEnabled: false,
  ...overrides,
});

describe('admin-only settings nav gates', () => {
  it.each(ADMIN_ONLY_TABS)('$tab declares $capability', ({ tab, capability }) => {
    expect(getSettingsNavItem(tab)?.requiredCapability).toBe(capability);
  });

  it.each(ADMIN_ONLY_TABS)(
    '$tab is hidden, route-blocked, and unmountable when $capability is withheld',
    ({ tab, capability }) => {
      const settingsCapabilities = { [capability]: false };
      const context = createContext({
        settingsCapabilitiesResolved: true,
        settingsCapabilities,
      });

      expect(shouldHideSettingsNavItem(tab, context)).toBe(true);
      expect(shouldBlockSettingsRouteItem(tab, context)).toBe(true);
      expect(canMountSettingsPanel(tab, settingsCapabilities)).toBe(false);
    },
  );

  it.each(ADMIN_ONLY_TABS)(
    '$tab remains reachable and mountable when $capability is granted',
    ({ tab, capability }) => {
      const settingsCapabilities = { [capability]: true };
      const context = createContext({
        settingsCapabilitiesResolved: true,
        settingsCapabilities,
      });

      expect(shouldHideSettingsNavItem(tab, context)).toBe(false);
      expect(shouldBlockSettingsRouteItem(tab, context)).toBe(false);
      expect(canMountSettingsPanel(tab, settingsCapabilities)).toBe(true);
    },
  );

  it.each(ADMIN_ONLY_TABS)(
    '$tab cannot mount before capabilities resolve even though the nav avoids flashing',
    ({ tab }) => {
      const context = createContext({
        settingsCapabilitiesResolved: false,
        settingsCapabilities: null,
      });

      expect(shouldHideSettingsNavItem(tab, context)).toBe(false);
      expect(shouldBlockSettingsRouteItem(tab, context)).toBe(false);
      expect(canMountSettingsPanel(tab, null)).toBe(false);
    },
  );

  it('keeps the user-scoped General panel mountable without a capability payload', () => {
    expect(getSettingsNavItem('system-general')?.requiredCapability).toBeUndefined();
    expect(canMountSettingsPanel('system-general', null)).toBe(true);
  });

  it('leaves General reachable after every admin-only capability is withheld', () => {
    const withheld = createContext({
      settingsCapabilitiesResolved: true,
      settingsCapabilities: {},
      hasFeature: () => false,
    });
    const reachable = SETTINGS_NAV_GROUPS.flatMap((group) => group.items)
      .filter((item) => !shouldBlockSettingsRouteItem(item.id, withheld))
      .map((item) => item.id);

    expect(reachable).toContain('system-general');
    expect(reachable).not.toContain('monitoring-availability');
    expect(reachable).not.toContain('support-diagnostics');
  });
});
