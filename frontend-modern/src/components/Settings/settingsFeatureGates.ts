import type { SettingsTab } from './settingsTypes';

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
