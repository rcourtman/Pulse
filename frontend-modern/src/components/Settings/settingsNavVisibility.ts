import type { SecurityStatusSettingsCapabilities } from '@/types/config';
import { isTabLocked } from './settingsFeatureGates';
import { getSettingsNavItem } from './settingsNavCatalog';
import type { SettingsTab } from './settingsNavigationModel';

export interface SettingsNavVisibilityContext {
  hasFeature: (feature: string) => boolean;
  licenseLoaded: () => boolean;
  demoModeEnabled?: boolean;
  demoModeResolved?: boolean;
  hostedModeEnabled?: boolean;
  settingsCapabilities?: Partial<SecurityStatusSettingsCapabilities> | null;
  settingsCapabilitiesResolved?: boolean;
}

function hasRequiredFeatures(
  tab: SettingsTab,
  hasFeature: (feature: string) => boolean,
): boolean {
  const requiredFeatures = getSettingsNavItem(tab)?.features ?? [];
  return requiredFeatures.every((feature) => hasFeature(feature));
}

export function shouldHideSettingsNavItem(
  tab: SettingsTab,
  context: SettingsNavVisibilityContext,
): boolean {
  const item = getSettingsNavItem(tab);
  if (!item) return false;

  if (item.hostedOnly && !context.hostedModeEnabled) {
    return true;
  }

  if (item.hideInDemoMode && context.demoModeEnabled) {
    return true;
  }

  if (
    item.requiredCapability &&
    context.settingsCapabilitiesResolved &&
    context.settingsCapabilities?.[item.requiredCapability] !== true
  ) {
    return true;
  }

  if (item.hideWhenUnavailable && !hasRequiredFeatures(tab, context.hasFeature)) {
    return true;
  }

  return false;
}

export function isSettingsNavItemLocked(
  tab: SettingsTab,
  context: SettingsNavVisibilityContext,
): boolean {
  const item = getSettingsNavItem(tab);
  if (!item || item.hideWhenUnavailable) {
    return false;
  }

  return isTabLocked(tab, context.hasFeature, context.licenseLoaded);
}
