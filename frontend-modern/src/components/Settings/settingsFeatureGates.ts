import type { SettingsTab } from './settingsTypes';
import { trackPaywallViewed } from '@/utils/conversionEvents';

export const tabFeatureRequirements: Partial<Record<SettingsTab, string[]>> = {
  'system-relay': ['relay'],
  reporting: ['advanced_reporting'],
  'security-webhooks': ['audit_logging'],
  'organization-overview': ['multi_tenant'],
  'organization-access': ['multi_tenant'],
  'organization-sharing': ['multi_tenant'],
  'organization-billing': ['multi_tenant'],
};

export function isFeatureLocked(
  features: string[] | undefined,
  hasFeature: (feature: string) => boolean,
  licenseLoaded: () => boolean,
): boolean {
  if (!features || features.length === 0) return false;
  if (!licenseLoaded()) return false;
  return !features.every((feature) => hasFeature(feature));
}

export function isTabLocked(
  tab: SettingsTab,
  hasFeature: (feature: string) => boolean,
  licenseLoaded: () => boolean,
): boolean {
  const requiredFeatures = tabFeatureRequirements[tab];
  return isFeatureLocked(requiredFeatures, hasFeature, licenseLoaded);
}

export function getTabLockReason(
  tab: SettingsTab,
  hasFeature: (feature: string) => boolean,
  licenseLoaded: () => boolean,
): string | null {
  const requiredFeatures = tabFeatureRequirements[tab];
  if (!requiredFeatures || requiredFeatures.length === 0) return null;
  if (!licenseLoaded()) return null;
  if (requiredFeatures.every((feature) => hasFeature(feature))) return null;
  const primaryRequiredFeature = requiredFeatures[0];
  if (primaryRequiredFeature) {
    trackPaywallViewed(primaryRequiredFeature, 'settings_tab');
  }
  return 'This settings section requires Pulse Pro.';
}
