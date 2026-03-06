import { Accessor, createEffect, createMemo, createSignal } from 'solid-js';
import type { SecurityStatus } from '@/types/config';
import { logger } from '@/utils/logger';
import { hasFeature, isHostedModeEnabled, licenseLoaded } from '@/stores/license';
import { DEFAULT_SETTINGS_TAB } from './settingsRouting';
import { tabFeatureRequirements } from './settingsFeatureGates';
import { SETTINGS_HEADER_META } from './settingsHeaderMeta';
import { baseTabGroups, getSettingsNavItem, shouldHideSettingsNavItem } from './settingsTabs';
import type { SettingsTab } from './settingsTypes';

interface UseSettingsAccessParams {
  activeTab: Accessor<SettingsTab>;
  setActiveTab: (tab: SettingsTab) => void;
  searchQuery: Accessor<string>;
}

export function useSettingsAccess({
  activeTab,
  setActiveTab,
  searchQuery,
}: UseSettingsAccessParams) {
  const [securityStatus, setSecurityStatus] = createSignal<SecurityStatus | null>(null);
  const [securityStatusLoading, setSecurityStatusLoading] = createSignal(true);

  const visibleTabGroups = createMemo(() => {
    const hostedModeEnabled = isHostedModeEnabled();
    const settingsCapabilities = securityStatus()?.settingsCapabilities ?? null;

    return baseTabGroups
      .map((group) => ({
        ...group,
        items: group.items.filter(
          (item) =>
            !shouldHideSettingsNavItem(item.id, {
              hasFeature,
              licenseLoaded,
              hostedModeEnabled,
              settingsCapabilities,
            }),
        ),
      }))
      .filter((group) => group.items.length > 0);
  });

  const flatTabs = createMemo(() => visibleTabGroups().flatMap((group) => group.items));

  const filteredTabGroups = createMemo(() => {
    const q = searchQuery().trim().toLowerCase();
    const groups = visibleTabGroups();
    if (!q) {
      return groups;
    }

    return groups
      .map((group) => {
        const filteredItems = group.items.filter((item) => {
          const matchLabel = item.label.toLowerCase().includes(q);
          const description = SETTINGS_HEADER_META[item.id]?.description?.toLowerCase() || '';
          const matchDesc = description.includes(q);
          return matchLabel || matchDesc;
        });
        return { ...group, items: filteredItems };
      })
      .filter((group) => group.items.length > 0);
  });

  createEffect(() => {
    const current = activeTab();
    const requiresFeatureResolution = Boolean(tabFeatureRequirements[current]?.length);
    const requiresCapabilityResolution = Boolean(getSettingsNavItem(current)?.requiredCapability);
    if (
      (requiresFeatureResolution && !licenseLoaded()) ||
      (requiresCapabilityResolution && securityStatusLoading())
    ) {
      return;
    }

    if (!flatTabs().some((tab) => tab.id === current)) {
      setActiveTab(DEFAULT_SETTINGS_TAB);
    }
  });

  async function loadSecurityStatus() {
    setSecurityStatusLoading(true);
    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch('/api/security/status');
      if (response.ok) {
        const status = await response.json();
        logger.debug('Security status loaded', status);
        setSecurityStatus(status);
      } else {
        logger.error('Failed to fetch security status', { status: response.status });
      }
    } catch (error) {
      logger.error('Failed to fetch security status', error);
    } finally {
      setSecurityStatusLoading(false);
    }
  }

  return {
    securityStatus,
    securityStatusLoading,
    visibleTabGroups,
    flatTabs,
    filteredTabGroups,
    loadSecurityStatus,
  };
}
