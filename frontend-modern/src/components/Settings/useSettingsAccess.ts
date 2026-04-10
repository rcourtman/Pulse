import { Accessor, createEffect, createMemo, createSignal } from 'solid-js';
import {
  presentationPolicyHidesCommercialSurfaces,
  presentationPolicyHidesOrganizationSurfaces,
  sessionPresentationPolicyResolved,
  syncSessionPresentationPolicy,
} from '@/stores/sessionPresentationPolicy';
import type { SecurityStatus } from '@/types/config';
import { logger } from '@/utils/logger';
import { hasFeature, isHostedModeEnabled, runtimeCapabilitiesLoaded } from '@/stores/license';
import { DEFAULT_SETTINGS_TAB, type SettingsTab } from './settingsNavigationModel';
import { tabFeatureRequirements } from './settingsFeatureGates';
import { SETTINGS_HEADER_META } from './settingsHeaderMeta';
import { getSettingsNavItem, SETTINGS_NAV_GROUPS } from './settingsNavCatalog';
import { shouldHideSettingsNavItem } from './settingsNavVisibility';

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
  const commercialSurfacesHidden = createMemo(() => {
    const resolvedSecurityStatus = securityStatus();
    if (resolvedSecurityStatus) {
      return (
        resolvedSecurityStatus.presentationPolicy?.hideCommercial === true ||
        resolvedSecurityStatus.sessionCapabilities?.demoMode === true
      );
    }
    return presentationPolicyHidesCommercialSurfaces();
  });
  const presentationPolicyResolved = createMemo(
    () => securityStatus() !== null || sessionPresentationPolicyResolved(),
  );
  const organizationSurfacesHidden = createMemo(() => {
    const resolvedSecurityStatus = securityStatus();
    if (resolvedSecurityStatus) {
      return (
        resolvedSecurityStatus.presentationPolicy?.demoMode === true ||
        resolvedSecurityStatus.sessionCapabilities?.demoMode === true
      );
    }
    return presentationPolicyHidesOrganizationSurfaces();
  });

  const visibleTabGroups = createMemo(() => {
    const hostedModeEnabled = isHostedModeEnabled();
    const settingsCapabilities = securityStatus()?.settingsCapabilities ?? null;
    const settingsCapabilitiesResolved = securityStatus() !== null;

    return SETTINGS_NAV_GROUPS
      .map((group) => ({
        ...group,
        items: group.items.filter(
          (item) =>
            !shouldHideSettingsNavItem(item.id, {
              hasFeature,
              runtimeCapabilitiesLoaded,
              presentationPolicyHidesCommercial: commercialSurfacesHidden(),
              presentationPolicyHidesOrganizations: organizationSurfacesHidden(),
              presentationPolicyResolved: presentationPolicyResolved(),
              hostedModeEnabled,
              settingsCapabilities,
              settingsCapabilitiesResolved,
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
    const requiresPresentationPolicyResolution = Boolean(
      getSettingsNavItem(current)?.hideWhenCommercialHidden ||
        getSettingsNavItem(current)?.hideWhenOrganizationHidden,
    );
    if (
      (requiresFeatureResolution && !runtimeCapabilitiesLoaded()) ||
      (requiresCapabilityResolution && securityStatusLoading()) ||
      (requiresPresentationPolicyResolution && !presentationPolicyResolved())
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
        const status = (await response.json()) as SecurityStatus;
        logger.debug('Security status loaded', status);
        syncSessionPresentationPolicy(status);
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
