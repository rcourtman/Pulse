import { For, Show, Suspense, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { Portal } from 'solid-js/web';
import type { JSX } from 'solid-js';
import { A, useLocation, useNavigate } from '@solidjs/router';
import BellIcon from 'lucide-solid/icons/bell';
import ListChecksIcon from 'lucide-solid/icons/list-checks';
import SettingsIcon from 'lucide-solid/icons/settings';
import Maximize2Icon from 'lucide-solid/icons/maximize-2';
import Minimize2Icon from 'lucide-solid/icons/minimize-2';
import SparklesIcon from 'lucide-solid/icons/sparkles';
import { getPlatformIcon } from '@/features/platformPage/platformIcon';
import {
  MobileNavBar,
  type MobileNavBarPrimaryTab as PrimaryTab,
  type MobileNavBarUtilityTab as UtilityTab,
} from '@/components/shared/MobileNavBar';
import {
  createStableTabList,
  primaryNavTabEquals,
  utilityNavTabEquals,
} from '@/components/shared/stableNavTabs';
import {
  buildPrimaryPlatformNavigationVisibility,
  primaryPlatformNavigationIsVisible,
  selectFirstVisiblePrimaryPlatformNavigationId,
  type PlatformNavigationVisibility,
  type PrimaryPlatformNavId,
} from '@/features/platformNavigation/platformNavigationModel';
import { dialogStackHasBlockingDialog } from '@/components/shared/useDialogState';
import { OrgSwitcher } from '@/components/OrgSwitcher';
import { PulseBrandMark } from '@/components/Brand/PulseBrandMark';
import { PulsePatrolLogo } from '@/components/Brand/PulsePatrolLogo';
import { MONITORING_READ_SCOPE, SETTINGS_READ_SCOPE } from '@/constants/apiScopes';
import type { Organization } from '@/api/orgs';
import type { VersionInfo } from '@/api/updates';
import type { Alert, State } from '@/types/api';
import { useKioskMode } from '@/hooks/useKioskMode';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { layoutStore } from '@/utils/layout';
import { logger } from '@/utils/logger';
import { getActiveTabForPath } from '@/routing/navigation';
import { preloadRouteModule } from '@/routing/routePreload';
import {
  DOCKER_PATH,
  KUBERNETES_PATH,
  PROXMOX_PATH,
  STANDALONE_PATH,
  TRUENAS_PATH,
  VMWARE_PATH,
  buildDockerPath,
  buildKubernetesPath,
  buildProxmoxPath,
  buildStandalonePath,
  buildTrueNASPath,
  buildVmwarePath,
} from '@/routing/resourceLinks';
import { getKioskModePreference, setKioskMode } from '@/utils/url';
import { updateStore } from '@/stores/updates';
import { aiChatStore } from '@/stores/aiChat';
import { getActionApprovalBadgePresentation } from '@/features/actions/actionPresentation';
import { actionInboxStore } from '@/stores/actionInbox';
import { patrolAttentionStore } from '@/stores/patrolAttention';
import { isPro } from '@/stores/licenseCommercial';
import { runtimeBranding } from '@/stores/systemSettings';
import { presentationPolicyHidesUpgradePrompts } from '@/stores/sessionPresentationPolicy';
import { getAssistantPageContext } from '@/utils/assistantPageContext';
import type { AppConnectionStatus } from '@/useAppRuntimeState';
import { buildInfrastructureWorkspacePath } from '@/components/Settings/infrastructureWorkspaceModel';

const ROOT_PROXMOX_PATH = buildProxmoxPath();
const ROOT_DOCKER_PATH = buildDockerPath();
const ROOT_KUBERNETES_PATH = buildKubernetesPath();
const ROOT_TRUENAS_PATH = buildTrueNASPath();
const ROOT_VMWARE_PATH = buildVmwarePath();
const ROOT_STANDALONE_PATH = buildStandalonePath();
const ROOT_INFRASTRUCTURE_SETTINGS_PATH = buildInfrastructureWorkspacePath();
const ROOT_ALERTS_PATH = '/alerts';
const PRIMARY_ROUTE_PREFIX_BY_ID: Record<PrimaryPlatformNavId, string> = {
  proxmox: PROXMOX_PATH,
  docker: DOCKER_PATH,
  kubernetes: KUBERNETES_PATH,
  truenas: TRUENAS_PATH,
  vmware: VMWARE_PATH,
  standalone: STANDALONE_PATH,
};
type PrimaryRouteMemory = Partial<Record<PrimaryPlatformNavId, string>>;
let primaryRouteMemory: PrimaryRouteMemory = {};

function resolveStandaloneSubTabTitle(pathname: string): string {
  const normalized = pathname.replace(/\/+$/, '');
  if (normalized === buildStandalonePath('availability')) return 'Availability checks';
  return 'Machines';
}

function routeBelongsToPrimaryTab(route: string, tabId: PrimaryPlatformNavId): boolean {
  const prefix = PRIMARY_ROUTE_PREFIX_BY_ID[tabId];
  const pathname = route.split(/[?#]/, 1)[0]?.replace(/\/+$/, '') || '/';
  return pathname === prefix || pathname.startsWith(`${prefix}/`);
}

function isPrimaryPlatformNavId(tabId: string | null | undefined): tabId is PrimaryPlatformNavId {
  return Boolean(tabId && tabId in PRIMARY_ROUTE_PREFIX_BY_ID);
}

function currentPrimaryRoute(pathname: string, search: string, hash: string): string {
  return `${pathname}${search}${hash}`;
}

function resolvePrimaryNavigationRoute(tab: PrimaryTab, routeMemory: PrimaryRouteMemory): string {
  if (!tab.enabled) {
    return tab.settingsRoute;
  }
  const remembered = routeMemory[tab.id as PrimaryPlatformNavId];
  if (remembered && routeBelongsToPrimaryTab(remembered, tab.id as PrimaryPlatformNavId)) {
    return remembered;
  }
  return tab.route;
}

export function resetPrimaryNavigationRouteMemory() {
  primaryRouteMemory = {};
}

/**
 * Whether this session is allowed to reach settings surfaces.
 *
 * Session/cookie auth carries no token scopes, so an absent or empty scope list
 * means "unrestricted". A scoped API token needs the wildcard or settings:read.
 * Kiosk tokens are minted with monitoring:read only, so they fail this check and
 * must never be handed a settings tab, a settings route, or a banner that links
 * into settings (#1650).
 */
export function sessionHasSettingsAccess(scopes: string[] | undefined): boolean {
  if (!scopes || scopes.length === 0) return true;
  return scopes.includes('*') || scopes.includes(SETTINGS_READ_SCOPE);
}
const NAV_TAB_ICON_CLASS = 'w-4 h-4 shrink-0';
const AI_CHAT_MOBILE_LAUNCHER_BUTTON_CLASS =
  'group relative flex h-11 w-11 flex-shrink-0 items-center justify-center rounded-full bg-surface-hover text-blue-600 transition-colors hover:bg-border hover:text-blue-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 dark:text-blue-400 dark:hover:text-blue-300';
const AI_CHAT_DESKTOP_LAUNCHER_BUTTON_CLASS =
  'fixed right-0 top-1/2 z-40 flex min-h-9 min-w-10 -translate-y-1/2 items-center justify-center rounded-l-lg border border-r-0 border-border bg-surface px-2.5 py-2.5 text-blue-600 transition-colors duration-200 hover:bg-surface-hover hover:text-blue-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 dark:text-blue-400 dark:hover:text-blue-300';

function getDesktopUtilityTabAriaLabel(tab: UtilityTab): string {
  const count = tab.count ?? 0;
  if (count > 0) {
    if (tab.id === 'alerts') {
      return `${count} ${tab.label}`;
    }
    return `${tab.label}: ${tab.countLabel ?? `${count} items`}`;
  }
  return tab.label;
}

export interface AppLayoutProps {
  connectionStatus: () => AppConnectionStatus;
  lastUpdateText: () => string;
  versionInfo: () => VersionInfo | null;
  hasAuth: () => boolean;
  needsAuth: () => boolean;
  proxyAuthInfo: () => { username?: string; logoutURL?: string } | null;
  handleLogout: () => void;
  state: () => State;
  platformVisibility?: () => PlatformNavigationVisibility;
  tokenScopes: () => string[] | undefined;
  organizations: () => Organization[];
  activeOrgID: () => string;
  orgsLoading: () => boolean;
  showOrgSwitcher: () => boolean;
  onSwitchOrg: (orgID: string) => void;
  children?: JSX.Element;
}

export function ConnectionStatusBadge(props: {
  connectionStatus: () => AppConnectionStatus;
  class?: string;
}) {
  const status = () => props.connectionStatus();
  const showSpinner = () =>
    status().kind === 'sync-reconnecting' || status().kind === 'reconnecting';
  const showLabelByDefault = () => status().tone !== 'healthy';
  const containerClass = () => {
    if (status().tone === 'healthy') {
      return 'connected bg-green-200 dark:bg-green-700 text-green-700 dark:text-green-300 min-w-6 h-6 group-hover:px-3';
    }
    if (status().tone === 'warning') {
      return 'degraded bg-amber-200 dark:bg-amber-700 text-amber-800 dark:text-amber-200 py-1 px-2';
    }
    if (status().kind === 'reconnecting') {
      return 'reconnecting bg-yellow-200 dark:bg-yellow-700 text-yellow-700 dark:text-yellow-300 py-1 px-2';
    }
    return 'disconnected bg-surface-hover text-base-content py-1 px-2';
  };
  const indicatorClass = () => {
    if (status().tone === 'healthy') {
      return 'bg-green-600 dark:bg-green-400';
    }
    if (status().tone === 'warning') {
      return 'bg-amber-600 dark:bg-amber-300';
    }
    return 'bg-slate-600';
  };

  return (
    <div
      class={`group status text-xs rounded-full flex items-center justify-center transition-all duration-500 ease-in-out ${containerClass()} ${props.class ?? ''}`}
      title={status().detail}
      role="status"
      aria-label={status().detail}
    >
      <Show when={showSpinner()}>
        <svg class="animate-spin h-3 w-3 flex-shrink-0" fill="none" viewBox="0 0 24 24">
          <circle
            class="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            stroke-width="4"
          />
          <path
            class="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          />
        </svg>
      </Show>
      <Show when={!showSpinner()}>
        <span class={`h-2.5 w-2.5 rounded-full flex-shrink-0 ${indicatorClass()}`} />
      </Show>
      <span
        class={`whitespace-nowrap overflow-hidden transition-all duration-500 ${
          showLabelByDefault()
            ? 'max-w-[170px] ml-2 opacity-100'
            : 'max-w-0 group-hover:max-w-[120px] group-hover:ml-2 group-hover:mr-1 opacity-0 group-hover:opacity-100'
        }`}
      >
        {status().label}
      </span>
    </div>
  );
}

export function AppLayout(props: AppLayoutProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const kioskMode = useKioskMode();
  const viewport = useBreakpoint();
  const brandMotionActive = createMemo(() => props.connectionStatus().tone === 'healthy');
  const customBrandLogo = createMemo(() =>
    runtimeBranding().enabled ? runtimeBranding().logoDataUrl : '',
  );
  const customBrandName = createMemo(() =>
    runtimeBranding().enabled ? runtimeBranding().displayName.trim() : '',
  );
  const browserBrandName = createMemo(() => customBrandName() || 'Pulse');

  const [headerVisible, setHeaderVisible] = createSignal(true);
  const [skipLinkFocused, setSkipLinkFocused] = createSignal(false);
  const [primaryRouteMemoryVersion, setPrimaryRouteMemoryVersion] = createSignal(0);
  let headerEl: HTMLDivElement | undefined;
  let assistantLauncherEl: HTMLButtonElement | undefined;
  let restoreAssistantLauncherFocus = false;
  let headerHideTimeout: ReturnType<typeof setTimeout> | undefined;

  const clearHeaderHideTimeout = () => {
    if (headerHideTimeout !== undefined) {
      clearTimeout(headerHideTimeout);
      headerHideTimeout = undefined;
    }
  };

  const showHeader = () => {
    clearHeaderHideTimeout();
    setHeaderVisible(true);
  };

  const scheduleHideHeader = (delayMs: number) => {
    clearHeaderHideTimeout();
    headerHideTimeout = setTimeout(() => {
      setHeaderVisible(false);
      headerHideTimeout = undefined;
    }, delayMs);
  };

  createEffect(() => {
    const scopes = props.tokenScopes();
    if (scopes && scopes.length === 1 && scopes[0] === MONITORING_READ_SCOPE) {
      const preference = getKioskModePreference();
      if (preference === null) {
        setKioskMode(true);
      }
    }
  });

  // Reflect the active tab in the browser tab title so multi-tab use,
  // browser history, and screen-reader page-title announcements all
  // identify the current Pulse surface instead of every page reading
  // as the bare app name.
  const tabTitleByActive: Record<NonNullable<ReturnType<typeof getActiveTabForPath>>, string> = {
    proxmox: 'Proxmox',
    docker: 'Docker',
    kubernetes: 'Kubernetes',
    truenas: 'TrueNAS',
    vmware: 'vSphere',
    standalone: 'Machines',
    alerts: 'Alerts',
    actions: 'Actions',
    ai: 'Patrol',
    settings: 'Settings',
  };
  createEffect(() => {
    const active = getActiveTabForPath(location.pathname);
    if (!active) {
      document.title = browserBrandName();
      return;
    }
    // The standalone (Machines) section has sub-tabs with their own labels.
    // Show the sub-tab title when one is active so the browser tab reflects
    // the actual page (e.g. "Availability checks" not "Machines").
    if (active === 'standalone') {
      const subTab = resolveStandaloneSubTabTitle(location.pathname);
      document.title = `${subTab} · ${browserBrandName()}`;
      return;
    }
    document.title = `${tabTitleByActive[active]} · ${browserBrandName()}`;
  });

  const toggleKioskMode = () => {
    setKioskMode(!kioskMode());
  };

  // Scope-limited sessions (kiosk API tokens) have no way back into the full
  // shell, so kiosk is not theirs to leave. Sessions that can reach settings
  // chose kiosk themselves and keep the escape hatch.
  const hasSettingsAccess = createMemo(() => sessionHasSettingsAccess(props.tokenScopes()));

  const platformNavigationVisibility = createMemo(
    () =>
      props.platformVisibility?.() ??
      buildPrimaryPlatformNavigationVisibility(props.state().resources || []),
  );
  const primaryInfrastructureRouteById: Record<PrimaryPlatformNavId, string> = {
    proxmox: ROOT_PROXMOX_PATH,
    docker: ROOT_DOCKER_PATH,
    kubernetes: ROOT_KUBERNETES_PATH,
    truenas: ROOT_TRUENAS_PATH,
    vmware: ROOT_VMWARE_PATH,
    standalone: ROOT_STANDALONE_PATH,
  };
  const primaryWorkspacePath = createMemo(() => {
    const navId = selectFirstVisiblePrimaryPlatformNavigationId(platformNavigationVisibility());
    return navId ? primaryInfrastructureRouteById[navId] : ROOT_ALERTS_PATH;
  });

  createEffect(() => {
    const activeTab = getActiveTabForPath(location.pathname);
    if (!isPrimaryPlatformNavId(activeTab)) return;
    const route = currentPrimaryRoute(location.pathname, location.search, location.hash);
    if (!routeBelongsToPrimaryTab(route, activeTab)) return;
    if (primaryRouteMemory[activeTab] === route) return;
    primaryRouteMemory = { ...primaryRouteMemory, [activeTab]: route };
    setPrimaryRouteMemoryVersion((version) => version + 1);
  });

  createEffect(() => {
    if (kioskMode()) {
      setHeaderVisible(true);
      scheduleHideHeader(1500);
    } else {
      clearHeaderHideTimeout();
      setHeaderVisible(true);
    }
  });

  createEffect(() => {
    if (!kioskMode()) return;

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      if (!hasSettingsAccess()) {
        // A kiosk-token session cannot exit kiosk: setKioskMode(false) is sticky
        // for the rest of the session and would strand the display on chrome it
        // has no authority to use. Give it the same temporary header peek the
        // hover and touch affordances give instead.
        showHeader();
        scheduleHideHeader(3000);
        return;
      }
      toggleKioskMode();
    };
    window.addEventListener('keydown', onKeyDown);
    onCleanup(() => window.removeEventListener('keydown', onKeyDown));
  });

  createEffect(() => {
    // Kiosk hides the navigation, but the redirect is not a kiosk decoration:
    // a session whose token cannot read settings must stay off these routes
    // whether or not kiosk is currently on (#1650).
    if (!kioskMode() && hasSettingsAccess()) return;
    const normalizedPath = location.pathname.replace(/\/+$/, '') || '/';
    const blockedPrefixes = ['/settings', '/patrol'];
    const isBlocked = blockedPrefixes.some(
      (prefix) => normalizedPath === prefix || normalizedPath.startsWith(prefix + '/'),
    );

    const isAlertsPath = normalizedPath === '/alerts' || normalizedPath.startsWith('/alerts/');
    const isAlertConfigTab =
      isAlertsPath &&
      normalizedPath !== '/alerts' &&
      normalizedPath !== '/alerts/overview' &&
      normalizedPath !== '/alerts/history' &&
      !normalizedPath.startsWith('/alerts/overview/') &&
      !normalizedPath.startsWith('/alerts/history/');

    const targetPath = primaryWorkspacePath();
    if ((isBlocked || isAlertConfigTab) && normalizedPath !== targetPath) {
      navigate(targetPath, { replace: true });
    }
  });

  createEffect(() => {
    if (!kioskMode()) return;

    const onPointerDown = (event: PointerEvent) => {
      if (event.pointerType !== 'touch') return;
      if (event.clientY > 60) return;
      showHeader();
      scheduleHideHeader(3000);
    };

    window.addEventListener('pointerdown', onPointerDown, { passive: true, capture: true });
    onCleanup(() => window.removeEventListener('pointerdown', onPointerDown, true));

    if (typeof (window as { PointerEvent?: unknown }).PointerEvent === 'undefined') {
      const onTouchStart = (event: TouchEvent) => {
        const touch = event.touches?.[0];
        if (!touch || touch.clientY > 60) return;
        showHeader();
        scheduleHideHeader(3000);
      };
      window.addEventListener('touchstart', onTouchStart, { passive: true, capture: true });
      onCleanup(() => window.removeEventListener('touchstart', onTouchStart, true));
    }
  });

  onCleanup(() => {
    clearHeaderHideTimeout();
  });

  const getNavigationActiveTab = () => getActiveTabForPath(location.pathname);
  const getActiveTabDesktop = getNavigationActiveTab;
  const getActiveTabMobile = getNavigationActiveTab;
  const assistantPageContext = createMemo(() => getAssistantPageContext(location.pathname));
  const openAssistantFromLauncher = () => {
    restoreAssistantLauncherFocus = true;
    aiChatStore.open(assistantPageContext().context);
  };
  const assistantLauncherVisible = createMemo(
    () =>
      aiChatStore.enabled === true &&
      !aiChatStore.isOpenSignal() &&
      !kioskMode() &&
      !dialogStackHasBlockingDialog(),
  );
  const renderAssistantLauncher = (className: string) => (
    <button
      ref={assistantLauncherEl}
      type="button"
      onClick={openAssistantFromLauncher}
      class={className}
      title={assistantPageContext().title}
      aria-label={assistantPageContext().ariaLabel}
    >
      <SparklesIcon class="h-5 w-5 flex-shrink-0" />
    </button>
  );
  createEffect(() => {
    if (aiChatStore.isOpenSignal() || !restoreAssistantLauncherFocus) return;
    restoreAssistantLauncherFocus = false;
    queueMicrotask(() => {
      if (assistantLauncherEl?.isConnected) assistantLauncherEl.focus();
    });
  });
  const actionApprovalBadge = createMemo(() =>
    getActionApprovalBadgePresentation(actionInboxStore.pendingActionCount),
  );
  const patrolAttentionCount = createMemo(() => patrolAttentionStore.summary()?.activeCount ?? 0);
  const patrolAttentionCountLabel = createMemo(() => {
    const count = patrolAttentionCount();
    if (count <= 0) return undefined;
    return `${count} active attention ${count === 1 ? 'item' : 'items'}`;
  });

  // Platform/runtime nav is resource-admitted. A platform or runtime lens only
  // appears when the support manifest says the surface is supported and the
  // current resource snapshot proves that surface is actually present.
  const primaryTabsSource = createMemo<PrimaryTab[]>(() => {
    const visible = platformNavigationVisibility();
    const isVisible = (id: PrimaryTab['id']) =>
      primaryPlatformNavigationIsVisible(visible, id as PrimaryPlatformNavId);
    const allPrimaryTabs: PrimaryTab[] = [
      {
        id: 'proxmox',
        label: 'Proxmox',
        route: ROOT_PROXMOX_PATH,
        settingsRoute: ROOT_INFRASTRUCTURE_SETTINGS_PATH,
        tooltip: 'Proxmox VE, Backup Server, Mail Gateway, storage, backups, and guests',
        enabled: isVisible('proxmox'),
        live: isVisible('proxmox'),
        icon: getPlatformIcon('proxmox'),
        alwaysShow: false,
      },
      {
        id: 'docker',
        label: 'Docker',
        route: ROOT_DOCKER_PATH,
        settingsRoute: ROOT_INFRASTRUCTURE_SETTINGS_PATH,
        tooltip: 'Docker / Podman runtime lens: hosts, containers, and Swarm services',
        enabled: isVisible('docker'),
        live: isVisible('docker'),
        icon: getPlatformIcon('docker'),
        alwaysShow: false,
      },
      {
        id: 'kubernetes',
        label: 'Kubernetes',
        route: ROOT_KUBERNETES_PATH,
        settingsRoute: ROOT_INFRASTRUCTURE_SETTINGS_PATH,
        tooltip: 'Kubernetes clusters, nodes, pods, deployments, and services',
        enabled: isVisible('kubernetes'),
        live: isVisible('kubernetes'),
        icon: getPlatformIcon('kubernetes'),
        alwaysShow: false,
      },
      {
        id: 'truenas',
        label: 'TrueNAS',
        route: ROOT_TRUENAS_PATH,
        settingsRoute: ROOT_INFRASTRUCTURE_SETTINGS_PATH,
        tooltip: 'TrueNAS hosts, storage, and apps',
        enabled: isVisible('truenas'),
        live: isVisible('truenas'),
        icon: getPlatformIcon('truenas'),
        alwaysShow: false,
      },
      {
        id: 'vmware',
        label: 'vSphere',
        route: ROOT_VMWARE_PATH,
        settingsRoute: ROOT_INFRASTRUCTURE_SETTINGS_PATH,
        tooltip: 'VMware vSphere hosts, virtual machines, datastores, and networks',
        enabled: isVisible('vmware'),
        live: isVisible('vmware'),
        icon: getPlatformIcon('vmware'),
        alwaysShow: false,
      },
      {
        id: 'standalone',
        label: 'Machines',
        route: ROOT_STANDALONE_PATH,
        settingsRoute: ROOT_INFRASTRUCTURE_SETTINGS_PATH,
        tooltip: 'Pulse Agent machines, agentless computers, and availability checks',
        enabled: isVisible('standalone'),
        live: isVisible('standalone'),
        icon: getPlatformIcon('standalone'),
        alwaysShow: false,
      },
    ];

    return allPrimaryTabs.filter((tab) => tab.alwaysShow || tab.enabled);
  });

  // Both tab memos rebuild their arrays from live store reads (activeAlerts is
  // replaced wholesale on every websocket state message), so without identity
  // stabilization every reference-keyed <For> consumer recreates all nav
  // buttons each tick — dropping taps that land mid-rebuild.
  const primaryTabs = createStableTabList(primaryTabsSource, primaryNavTabEquals);

  const utilityTabsSource = createMemo<UtilityTab[]>(() => {
    const allAlerts = props.state().activeAlerts || [];
    const breakdown = allAlerts.reduce(
      (accumulator, alert: Alert) => {
        if (alert?.acknowledged) return accumulator;
        const level = String(alert?.level || '').toLowerCase();
        if (level === 'critical') {
          accumulator.critical += 1;
        } else {
          accumulator.warning += 1;
        }
        return accumulator;
      },
      { warning: 0, critical: 0 },
    );
    const activeAlertCount = breakdown.warning + breakdown.critical;

    const tabs: UtilityTab[] = [
      {
        id: 'alerts',
        label: 'Alerts',
        route: '/alerts',
        tooltip: 'Review active alerts and automation rules',
        badge: null,
        count: activeAlertCount,
        breakdown,
        icon: BellIcon,
      },
      {
        id: 'ai',
        label: 'Patrol',
        route: '/patrol',
        tooltip: 'Review active operational attention and recent Patrol checks',
        badge: null,
        count: patrolAttentionCount() || undefined,
        countLabel: patrolAttentionCountLabel(),
        breakdown: undefined,
        icon: PulsePatrolLogo,
      },
      {
        id: 'actions',
        label: 'Actions',
        route: '/actions',
        tooltip: 'Review approvals, run ready work, and see recorded outcomes',
        badge: null,
        count: actionApprovalBadge()?.count,
        countLabel: actionApprovalBadge()?.label,
        breakdown: undefined,
        icon: ListChecksIcon,
      },
    ];

    if (hasSettingsAccess()) {
      tabs.push({
        id: 'settings',
        label: 'Settings',
        route: '/settings',
        tooltip: 'Configure Pulse preferences and integrations',
        badge: updateStore.isUpdateVisible() ? 'update' : null,
        count: undefined,
        breakdown: undefined,
        icon: SettingsIcon,
      });
    }

    return tabs;
  });

  const utilityTabs = createStableTabList(utilityTabsSource, utilityNavTabEquals);

  const warmNavigationTarget = (route: string) => {
    void preloadRouteModule(route).catch((error) => {
      logger.warn('Failed to warm navigation target', {
        route,
        error: error instanceof Error ? error.message : String(error),
      });
    });
  };

  const handlePrimaryClick = (tab: PrimaryTab) => {
    const targetRoute = resolvePrimaryNavigationRoute(tab, primaryRouteMemory);
    warmNavigationTarget(targetRoute);
    navigate(targetRoute);
  };

  const handleUtilityClick = (tab: UtilityTab) => {
    warmNavigationTarget(tab.route);
    navigate(tab.route);
  };

  const getPrimaryTargetRoute = (tab: PrimaryTab) => {
    // Route memory lives across AppLayout remounts. Track local writes with a
    // signal so link hrefs update as users move within a platform, including
    // for native middle-click and context-menu navigation.
    void primaryRouteMemoryVersion();
    return resolvePrimaryNavigationRoute(tab, primaryRouteMemory);
  };

  const renderPrimaryNavigationTab = (tab: PrimaryTab) => {
    const isActive = () => getActiveTabDesktop() === tab.id;
    const needsSetup = () => !tab.enabled;
    const targetRoute = () => getPrimaryTargetRoute(tab);
    const Icon = tab.icon;
    const baseClasses =
      'tab relative px-1.5 xl:px-2 2xl:px-3 py-1.5 text-xs xl:text-sm font-medium flex items-center gap-1 2xl:gap-1.5 rounded-t border border-transparent transition-colors whitespace-nowrap cursor-pointer select-none';

    const className = () => {
      if (isActive()) {
        return `${baseClasses} bg-surface text-blue-600 dark:text-blue-400 border-border border-b border-b-surface shadow-sm font-semibold`;
      }
      if (needsSetup()) {
        return `${baseClasses} text-muted opacity-70 bg-base hover:bg-surface-hover`;
      }
      return `${baseClasses} text-muted hover:text-base-content hover:bg-surface-hover`;
    };

    const title = () =>
      needsSetup() ? `${tab.label} is not configured yet. Click to open settings.` : tab.tooltip;

    return (
      <A
        href={targetRoute()}
        class={className()}
        aria-label={tab.label}
        aria-current={isActive() ? 'page' : undefined}
        onMouseEnter={() => warmNavigationTarget(targetRoute())}
        onFocus={() => warmNavigationTarget(targetRoute())}
        title={title()}
      >
        <span aria-hidden="true" class="inline-flex items-center justify-center">
          <Icon class={NAV_TAB_ICON_CLASS} />
        </span>
        <span class="hidden xs:inline-flex items-center gap-1">
          <span>{tab.label}</span>
          <Show when={tab.badge}>
            <span class="px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-muted bg-surface-hover rounded">
              {tab.badge}
            </span>
          </Show>
        </span>
        <span class="xs:hidden">{tab.label.charAt(0)}</span>
      </A>
    );
  };

  return (
    <div
      class={`pulse-shell ${layoutStore.isFullWidth() || kioskMode() ? 'pulse-shell--full-width' : ''} ${!kioskMode() ? 'pb-safe-or-14 xl:pb-0' : ''}`}
    >
      {/* Skip-to-content link: visually hidden until focused, then
          appears as a button at the top-left. Lets keyboard users
          jump past the chrome straight into the page content. */}
      <a
        href="#main"
        onFocus={() => setSkipLinkFocused(true)}
        onBlur={() => setSkipLinkFocused(false)}
        class={
          skipLinkFocused()
            ? 'absolute left-2 top-2 z-[100] rounded bg-blue-600 px-3 py-2 text-sm font-medium text-white shadow-lg outline outline-2 outline-offset-2 outline-white'
            : 'sr-only'
        }
      >
        Skip to main content
      </a>
      <Show when={kioskMode()}>
        <div
          class="fixed top-0 left-0 right-0 z-40 h-4 bg-transparent"
          aria-hidden="true"
          onMouseEnter={() => {
            if (!kioskMode()) return;
            showHeader();
          }}
          onMouseLeave={() => {
            if (!kioskMode()) return;
            scheduleHideHeader(500);
          }}
        />
      </Show>
      <div
        class={`header mb-1 flex items-center gap-1 sm:mb-3 sm:gap-2 ${
          kioskMode()
            ? 'fixed top-0 left-0 right-0 z-50 justify-end bg-surface shadow-sm'
            : 'justify-between sm:grid sm:grid-cols-[1fr_auto_1fr] sm:items-center sm:gap-0'
        }`}
        style={
          kioskMode()
            ? {
                transform: headerVisible() ? 'translateY(0)' : 'translateY(-100%)',
                opacity: headerVisible() ? 1 : 0,
                transition: `transform ${headerVisible() ? 200 : 300}ms ease, opacity ${headerVisible() ? 200 : 300}ms ease`,
                'pointer-events': headerVisible() ? 'auto' : 'none',
              }
            : undefined
        }
        ref={(element) => {
          headerEl = element;
        }}
        onMouseEnter={() => {
          if (!kioskMode()) return;
          showHeader();
        }}
        onMouseLeave={() => {
          if (!kioskMode()) return;
          scheduleHideHeader(500);
        }}
        onFocusIn={() => {
          if (!kioskMode()) return;
          showHeader();
        }}
        onFocusOut={(event) => {
          if (!kioskMode()) return;
          const next = event.relatedTarget as Node | null;
          if (next && headerEl?.contains(next)) return;
          scheduleHideHeader(500);
        }}
      >
        <Show when={!kioskMode()}>
          <div class="flex items-center gap-1.5 sm:col-start-2 sm:col-end-3 sm:flex-initial sm:justify-self-center sm:gap-2">
            <div
              class={`pulse-brand-lockup flex items-center gap-1.5 sm:gap-2 ${!customBrandLogo() && brandMotionActive() ? 'animate-pulse-brand' : ''}`}
              data-testid="pulse-brand-lockup"
            >
              <Show when={customBrandLogo()} fallback={<PulseBrandMark class="h-5 w-5" />}>
                {(logoDataUrl) => (
                  <img
                    src={logoDataUrl()}
                    alt={customBrandName() ? `${customBrandName()} logo` : 'Custom logo'}
                    class="max-h-8 max-w-[12rem] object-contain"
                    data-testid="custom-brand-logo"
                  />
                )}
              </Show>
              <Show when={customBrandName() || !customBrandLogo()}>
                <span class="pulse-brand-wordmark text-base font-medium text-base-content sm:text-lg">
                  {customBrandName() || 'Pulse'}
                </span>
              </Show>
              <Show when={props.versionInfo()?.channel === 'rc'}>
                <span class="text-xs px-1.5 py-0.5 bg-orange-500 text-white rounded font-bold">
                  Preview
                </span>
              </Show>
            </div>
          </div>
        </Show>
        <div
          class={`header-controls flex items-center gap-1 sm:gap-2 ${kioskMode() ? '' : 'justify-end sm:col-start-3 sm:col-end-4 sm:w-auto sm:justify-end sm:justify-self-end'}`}
        >
          <Show when={assistantLauncherVisible() && viewport.isBelow('lg')}>
            {renderAssistantLauncher(AI_CHAT_MOBILE_LAUNCHER_BUTTON_CLASS)}
          </Show>
          <Show when={props.hasAuth() && !props.needsAuth()}>
            <div class="flex items-center gap-1 sm:gap-2">
              <Show when={props.showOrgSwitcher()}>
                <OrgSwitcher
                  orgs={props.organizations()}
                  selectedOrgId={props.activeOrgID()}
                  loading={props.orgsLoading()}
                  onChange={props.onSwitchOrg}
                />
              </Show>
              <button
                type="button"
                onClick={toggleKioskMode}
                class={`group relative flex h-9 w-9 items-center justify-center rounded-full text-xs transition focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 sm:h-10 sm:w-10 ${
                  kioskMode()
                    ? 'bg-blue-100 text-blue-700 hover:bg-blue-200 dark:bg-blue-900 dark:text-blue-300 dark:hover:bg-blue-800'
                    : 'bg-surface-hover text-base-content hover:bg-border'
                }`}
                title={
                  kioskMode()
                    ? 'Exit kiosk mode (show navigation)'
                    : 'Enter kiosk mode (hide navigation)'
                }
                aria-label={kioskMode() ? 'Exit kiosk mode' : 'Enter kiosk mode'}
                aria-pressed={kioskMode()}
              >
                <Show when={kioskMode()} fallback={<Maximize2Icon class="h-4 w-4 flex-shrink-0" />}>
                  <Minimize2Icon class="h-4 w-4 flex-shrink-0" />
                </Show>
              </button>
              <Show when={props.proxyAuthInfo()?.username}>
                <span class="text-xs px-2 py-1 text-muted">{props.proxyAuthInfo()?.username}</span>
              </Show>
              <button
                type="button"
                onClick={props.handleLogout}
                class="group relative flex h-9 w-9 items-center justify-center rounded-full bg-surface-hover text-xs text-base-content transition hover:bg-border focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 sm:h-10 sm:w-10"
                title="Logout"
                aria-label="Logout"
              >
                <svg
                  class="h-4 w-4 flex-shrink-0"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="2"
                  aria-hidden="true"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
                  />
                </svg>
              </button>
            </div>
          </Show>
          <ConnectionStatusBadge connectionStatus={props.connectionStatus} class="flex-shrink-0" />
        </div>
      </div>

      <Show when={!kioskMode()}>
        <nav
          class="tabs mb-2 hidden xl:flex items-end gap-2 overflow-x-auto overflow-y-hidden whitespace-nowrap border-b border-border scrollbar-hide"
          aria-label="Primary navigation"
        >
          <div class="flex items-end gap-1" role="group" aria-label="Infrastructure">
            <For each={primaryTabs()}>{renderPrimaryNavigationTab}</For>
          </div>
          <div class="flex items-end gap-1 ml-auto" role="group" aria-label="System">
            <div class="flex items-end gap-1 pl-1 sm:pl-4">
              <For each={utilityTabs()}>
                {(tab) => {
                  const isActive = () => getActiveTabDesktop() === tab.id;
                  const Icon = tab.icon;
                  const baseClasses =
                    'tab relative px-1.5 xl:px-2 2xl:px-3 py-1.5 text-xs xl:text-sm font-medium flex items-center gap-1 2xl:gap-1.5 rounded-t border border-transparent transition-colors whitespace-nowrap cursor-pointer select-none';

                  const className = () => {
                    if (isActive()) {
                      return `${baseClasses} bg-surface text-blue-600 dark:text-blue-400 border-border border-b border-b-surface shadow-sm font-semibold`;
                    }
                    return `${baseClasses} text-muted hover:text-base-content hover:bg-surface-hover`;
                  };

                  return (
                    <A
                      href={tab.route}
                      class={className()}
                      aria-label={getDesktopUtilityTabAriaLabel(tab)}
                      aria-current={isActive() ? 'page' : undefined}
                      onMouseEnter={() => warmNavigationTarget(tab.route)}
                      onFocus={() => warmNavigationTarget(tab.route)}
                      title={tab.tooltip}
                    >
                      <span aria-hidden="true" class="inline-flex items-center justify-center">
                        <Icon class={NAV_TAB_ICON_CLASS} />
                      </span>
                      <span class="flex items-center gap-1">
                        <span class="hidden xs:inline">{tab.label}</span>
                        <span class="xs:hidden">{tab.label.charAt(0)}</span>
                        {(() => {
                          const total = tab.count ?? 0;
                          if (total <= 0) {
                            return null;
                          }
                          if (tab.id === 'alerts') {
                            return (
                              <span class="inline-flex items-center gap-1">
                                {tab.breakdown && tab.breakdown.critical > 0 && (
                                  <span class="inline-flex items-center justify-center min-w-[18px] h-[18px] px-1 text-[10px] font-bold text-white bg-red-600 dark:bg-red-500 rounded-full">
                                    {tab.breakdown.critical}
                                  </span>
                                )}
                                {tab.breakdown && tab.breakdown.warning > 0 && (
                                  <span class="inline-flex items-center justify-center min-w-[18px] h-[18px] px-1 text-[10px] font-semibold text-amber-900 dark:text-amber-100 bg-amber-200 dark:bg-amber-500 rounded-full">
                                    {tab.breakdown.warning}
                                  </span>
                                )}
                              </span>
                            );
                          }
                          return (
                            <span class="inline-flex items-center justify-center min-w-[18px] h-[18px] px-1 text-[10px] font-semibold text-amber-900 dark:text-amber-100 bg-amber-200 dark:bg-amber-500 rounded-full">
                              {total}
                            </span>
                          );
                        })()}
                      </span>
                      <Show when={tab.badge === 'update'}>
                        <span class="ml-1 flex items-center">
                          <span class="sr-only">Update available</span>
                          <span
                            aria-hidden="true"
                            class="block h-2 w-2 rounded-full bg-red-500 animate-pulse"
                          />
                        </span>
                      </Show>
                      <Show when={tab.badge === 'pro' && !presentationPolicyHidesUpgradePrompts()}>
                        <span class="ml-1.5 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-blue-700 dark:text-blue-300 bg-blue-100 dark:bg-blue-900 rounded">
                          Pro
                        </span>
                      </Show>
                    </A>
                  );
                }}
              </For>
            </div>
          </div>
        </nav>
      </Show>

      <main
        id="main"
        class="tab-content mb-1 block rounded-b rounded-tl rounded-tr bg-surface shadow sm:mb-2"
      >
        <div class="pulse-panel">
          <Suspense fallback={<div class="p-6 text-sm text-muted">Loading view...</div>}>
            {props.children}
          </Suspense>
        </div>
      </main>

      <Show when={!kioskMode()}>
        <MobileNavBar
          activeTab={getActiveTabMobile}
          primaryTabs={primaryTabs}
          utilityTabs={utilityTabs}
          getPrimaryHref={getPrimaryTargetRoute}
          onPrimaryClick={handlePrimaryClick}
          onUtilityClick={handleUtilityClick}
        />
      </Show>

      <Show when={!kioskMode()}>
        <footer class="pulse-footer px-2 py-2 text-xs leading-relaxed text-muted sm:px-4 sm:py-4">
          <div class="text-center">
            <span>Pulse | Version: </span>
            <a
              href="https://github.com/rcourtman/Pulse/releases"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex min-h-8 items-center break-all rounded px-1 py-1 text-blue-600 hover:underline dark:text-blue-400 sm:min-h-9"
            >
              {props.versionInfo()?.version || 'loading...'}
            </a>
            {props.versionInfo()?.isDevelopment && ' (Development)'}
            {props.versionInfo()?.isDocker && ' - Docker'}
          </div>
          <div class="mt-0.5 flex flex-wrap items-center justify-center gap-x-2 gap-y-1 text-center sm:mt-1">
            <Show when={props.lastUpdateText()}>
              <>
                <span>Last refresh: {props.lastUpdateText()}</span>
                <Show when={isPro()}>
                  <span aria-hidden="true">|</span>
                </Show>
              </>
            </Show>
            <Show when={isPro()}>
              <a
                href={`mailto:support@pulserelay.pro?subject=${encodeURIComponent(`Support Request - Pulse ${props.versionInfo()?.version || ''}`)}`}
                class="inline-flex min-h-8 items-center rounded px-1 py-1 text-blue-600 hover:underline dark:text-blue-400 sm:min-h-9"
              >
                Get Support
              </a>
            </Show>
          </div>
        </footer>
      </Show>

      <Show when={assistantLauncherVisible() && viewport.isAtLeast('lg')}>
        {/* Portaled out of .app-scroll-shell: as a descendant the scroll
            container's own scrollbar hit-tests above the tab's right edge,
            leaving the screen-edge click zone dead. */}
        <Portal>{renderAssistantLauncher(AI_CHAT_DESKTOP_LAUNCHER_BUTTON_CLASS)}</Portal>
      </Show>
    </div>
  );
}
