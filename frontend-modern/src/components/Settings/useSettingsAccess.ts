import { Accessor, createEffect, createMemo, createSignal } from 'solid-js';
import {
  presentationPolicyHidesCommercialSurfaces,
  presentationPolicyHidesOrganizationSurfaces,
  presentationPolicyIsDemoMode,
  presentationPolicyIsReadOnly,
  sessionPresentationPolicyResolved,
  syncSessionPresentationPolicy,
} from '@/stores/sessionPresentationPolicy';
import type { SecurityStatus } from '@/types/config';
import { logger } from '@/utils/logger';
import {
  hasFeature,
  isHostedModeEnabled,
  isRuntimeCapabilityBlocked,
  runtimeCapabilitiesLoaded,
} from '@/stores/license';
import { DEFAULT_SETTINGS_TAB, type SettingsTab } from './settingsNavigationModel';
import { tabFeatureRequirements } from './settingsFeatureGates';
import { getSettingsHeaderMeta } from './settingsHeaderMeta';
import { getSettingsNavGroups, getSettingsNavItem } from './settingsNavCatalog';
import { shouldBlockSettingsRouteItem, shouldHideSettingsNavItem } from './settingsNavVisibility';

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
  const demoMode = createMemo(() => {
    const resolvedSecurityStatus = securityStatus();
    if (resolvedSecurityStatus) {
      return (
        resolvedSecurityStatus.presentationPolicy?.demoMode === true ||
        resolvedSecurityStatus.sessionCapabilities?.demoMode === true
      );
    }
    return presentationPolicyIsDemoMode();
  });
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
  const readOnly = createMemo(() => {
    const resolvedSecurityStatus = securityStatus();
    if (resolvedSecurityStatus) {
      return resolvedSecurityStatus.presentationPolicy?.readOnly === true || demoMode();
    }
    return presentationPolicyIsReadOnly();
  });

  const routeAccessContext = createMemo(() => {
    const hostedModeEnabled = isHostedModeEnabled();
    const settingsCapabilities = securityStatus()?.settingsCapabilities ?? null;
    const settingsCapabilitiesResolved = securityStatus() !== null;

    return {
      hasFeature,
      runtimeCapabilitiesLoaded,
      presentationPolicyHidesCommercial: commercialSurfacesHidden(),
      presentationPolicyIsDemoMode: demoMode(),
      presentationPolicyIsReadOnly: readOnly(),
      presentationPolicyHidesOrganizations: organizationSurfacesHidden(),
      presentationPolicyResolved: presentationPolicyResolved(),
      hostedModeEnabled,
      settingsCapabilities,
      settingsCapabilitiesResolved,
      isRuntimeCapabilityBlocked,
    };
  });

  const settingsNavGroups = createMemo(() => getSettingsNavGroups());
  const settingsHeaderMeta = createMemo(() => getSettingsHeaderMeta());

  const routeTabGroups = createMemo(() =>
    settingsNavGroups()
      .map((group) => ({
        ...group,
        items: group.items.filter(
          (item) => !shouldBlockSettingsRouteItem(item.id, routeAccessContext()),
        ),
      }))
      .filter((group) => group.items.length > 0),
  );

  const navTabGroups = createMemo(() =>
    settingsNavGroups()
      .map((group) => ({
        ...group,
        items: group.items.filter(
          (item) => !shouldHideSettingsNavItem(item.id, routeAccessContext()),
        ),
      }))
      .filter((group) => group.items.length > 0),
  );

  const flatTabs = createMemo(() => routeTabGroups().flatMap((group) => group.items));

  const visibleTabGroups = createMemo(() =>
    navTabGroups()
      .map((group) => ({
        ...group,
        items: group.items.filter((item) => !item.hideFromSidebar),
      }))
      .filter((group) => group.items.length > 0),
  );

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
          const description = settingsHeaderMeta()[item.id]?.description?.toLowerCase() || '';
          const matchDesc = description.includes(q);
          return matchLabel || matchDesc;
        });
        return { ...group, items: filteredItems };
      })
      .filter((group) => group.items.length > 0);
  });

  createEffect(() => {
    const current = activeTab();
    const currentItem = getSettingsNavItem(current);
    const requiresFeatureResolution = Boolean(
      tabFeatureRequirements[current]?.length || currentItem?.features?.length,
    );
    const requiresCapabilityResolution = Boolean(currentItem?.requiredCapability);
    const requiresPresentationPolicyResolution = Boolean(
      currentItem?.hideWhenCommercialHidden ||
      currentItem?.hideWhenOrganizationHidden ||
      currentItem?.hideWhenReadOnly ||
      currentItem?.hideWhenDemoMode,
    );
    if (
      (requiresFeatureResolution && !runtimeCapabilitiesLoaded()) ||
      (requiresCapabilityResolution && securityStatusLoading()) ||
      (requiresPresentationPolicyResolution && !presentationPolicyResolved())
    ) {
      return;
    }

    if (!flatTabs().some((tab) => tab.id === current)) {
      const currentRouteStillAllowed =
        currentItem && !shouldBlockSettingsRouteItem(current, routeAccessContext());

      if (currentRouteStillAllowed) {
        return;
      }
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
