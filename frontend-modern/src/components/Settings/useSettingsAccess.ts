import {
  Accessor,
  batch,
  createEffect,
  createMemo,
  createSignal,
  onCleanup,
  onMount,
} from 'solid-js';
import { syncSessionSettingsCapabilities } from '@/stores/sessionSettingsCapabilities';
import {
  presentationPolicyHidesCommercialSurfaces,
  presentationPolicyHidesOrganizationSurfaces,
  presentationPolicyHidesUpgradePrompts,
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
  const [securityStatusError, setSecurityStatusError] = createSignal(false);
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
  const upgradePromptsHidden = createMemo(() => {
    const resolvedSecurityStatus = securityStatus();
    if (resolvedSecurityStatus) {
      return resolvedSecurityStatus.presentationPolicy?.hideUpgrade !== false;
    }
    return presentationPolicyHidesUpgradePrompts();
  });
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
    // it refreshes, but withdraw it if that refresh cannot verify access.
    const settingsCapabilitiesResolved = securityStatus() !== null || !securityStatusLoading();

    return {
      hasFeature,
      runtimeCapabilitiesLoaded,
      presentationPolicyHidesCommercial: commercialSurfacesHidden(),
      presentationPolicyHidesUpgrade: upgradePromptsHidden(),
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
      currentItem?.hideWhenUpgradeHidden ||
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
  let securityStatusController: AbortController | null = null;
  let disposed = false;

  onCleanup(() => {
    disposed = true;
    securityStatusController?.abort();
  });

  async function fetchSecurityStatus() {
    const controller = new AbortController();
    securityStatusController = controller;
    setSecurityStatusLoading(true);
    try {
      const { apiFetch } = await import('@/utils/apiClient');
      if (disposed) return;
      const response = await apiFetch('/api/security/status', {
        signal: controller.signal,
        retry: false,
      });
      if (disposed) return;
      if (!response.ok) {
        throw new Error(`Security status request failed (${response.status})`);
      }
      const status = (await response.json()) as SecurityStatus;
      if (disposed) return;
      if (!status || typeof status !== 'object' || Array.isArray(status)) {
        throw new Error('Invalid security status');
      }
      batch(() => {
        syncSessionPresentationPolicy(status);
        setSecurityStatus(status);
        setSecurityStatusError(false);
      });
    } catch (error) {
      if (disposed) return;
      logger.error('Failed to fetch security status', error);
      batch(() => {
        // A previous grant is not evidence of current access. Unmount restricted
        // panels and withdraw their links, without resetting the session's
        // separate read-only/demo presentation policy to permissive defaults.
        setSecurityStatus(null);
        syncSessionSettingsCapabilities(null);
        setSecurityStatusError(true);
      });
    } finally {
      if (!disposed) {
        securityStatusController = null;
        setSecurityStatusLoading(false);
      }
    }
  }

  function loadSecurityStatus(): Promise<void> {
    if (disposed) return Promise.resolve();
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
    securityStatusError,
    visibleTabGroups,
    flatTabs,
    filteredTabGroups,
    loadSecurityStatus,
  };
}
