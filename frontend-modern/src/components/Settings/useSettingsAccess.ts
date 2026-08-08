import { Accessor, createEffect, createMemo, createSignal, onMount } from 'solid-js';
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
import {
  canMountSettingsPanel,
  shouldBlockSettingsRouteItem,
  shouldHideSettingsNavItem,
} from './settingsNavVisibility';

const SETTINGS_FALLBACK_TAB_ORDER: readonly SettingsTab[] = [
  DEFAULT_SETTINGS_TAB,
  'system-general',
];

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
    // A failed status request is a resolved denial, not permission to expose
    // capability-gated routes. Keep an existing successful status usable while
    // it refreshes, but fail closed once the first request settles without one.
    const settingsCapabilitiesResolved = securityStatus() !== null || !securityStatusLoading();

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
          const matchKeyword = (item.keywords ?? []).some((keyword) =>
            keyword.toLowerCase().includes(q),
          );
          return matchLabel || matchDesc || matchKeyword;
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
      // The default tab is itself gated now (Infrastructure needs
      // settings:read), so falling back to it unconditionally would strand a
      // non-admin session on a blocked tab. Use an explicit safe preference:
      // catalog order can move and currently puts the Plans surface before the
      // user-scoped General tab once the admin-only entries are removed.
      const reachable = flatTabs().filter((item) =>
        canMountSettingsPanel(item.id, securityStatus()?.settingsCapabilities),
      );
      const fallbackTab =
        SETTINGS_FALLBACK_TAB_ORDER.find((tab) => reachable.some((item) => item.id === tab)) ??
        reachable[0]?.id ??
        DEFAULT_SETTINGS_TAB;
      setActiveTab(fallbackTab);
    }
  });

  let securityStatusRequest: Promise<void> | null = null;

  async function fetchSecurityStatus() {
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

  function loadSecurityStatus(): Promise<void> {
    if (securityStatusRequest) {
      return securityStatusRequest;
    }
    const request = fetchSecurityStatus().finally(() => {
      securityStatusRequest = null;
    });
    securityStatusRequest = request;
    return request;
  }

  // Capability resolution belongs to the access model. Other Settings hooks
  // may await the same request, but panel safety must not depend on an
  // unrelated infrastructure bootstrap continuing to mount.
  onMount(() => {
    void loadSecurityStatus();
  });

  return {
    securityStatus,
    securityStatusLoading,
    visibleTabGroups,
    flatTabs,
    filteredTabGroups,
    loadSecurityStatus,
  };
}
