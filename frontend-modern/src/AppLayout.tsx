import { For, Show, Suspense, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import type { JSX } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import BellIcon from 'lucide-solid/icons/bell';
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
  buildPrimaryPlatformNavigationVisibility,
  primaryPlatformNavigationIsVisible,
  selectFirstVisiblePrimaryPlatformNavigationId,
  type PrimaryPlatformNavId,
} from '@/features/platformNavigation/platformNavigationModel';
import { dialogStackHasBlockingDialog } from '@/components/shared/useDialogState';
import { OrgSwitcher } from '@/components/OrgSwitcher';
import { PulsePatrolLogo } from '@/components/Brand/PulsePatrolLogo';
import { MONITORING_READ_SCOPE } from '@/constants/apiScopes';
import type { Organization } from '@/api/orgs';
import type { VersionInfo } from '@/api/updates';
import type { Alert, State } from '@/types/api';
import { useKioskMode } from '@/hooks/useKioskMode';
import { layoutStore } from '@/utils/layout';
import { logger } from '@/utils/logger';
import { getActiveTabForPath } from '@/routing/navigation';
import { preloadRouteModule } from '@/routing/routePreload';
import {
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
import { isPro } from '@/stores/licenseCommercial';
import { presentationPolicyHidesUpgradePrompts } from '@/stores/sessionPresentationPolicy';
import { AI_CHAT_LAUNCHER_ARIA_LABEL, getAIChatLauncherTitle } from '@/utils/aiChatPresentation';
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
const NAV_TAB_ICON_CLASS = 'w-4 h-4 shrink-0';
const AI_CHAT_LAUNCHER_BUTTON_CLASS =
  'fixed right-4 bottom-[calc(5rem+env(safe-area-inset-bottom,0px))] z-40 flex h-11 w-11 items-center justify-center rounded-full border border-border bg-surface text-blue-600 shadow-lg transition-colors duration-200 hover:bg-surface-hover hover:text-blue-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 dark:text-blue-400 dark:hover:text-blue-300 lg:right-0 lg:top-1/2 lg:bottom-auto lg:h-auto lg:w-auto lg:min-h-9 lg:min-w-10 lg:-translate-y-1/2 lg:rounded-l-lg lg:rounded-r-none lg:border-r-0 lg:px-2.5 lg:py-2.5 lg:shadow-none';

function getDesktopUtilityTabAriaLabel(tab: UtilityTab): string {
  if (tab.id === 'alerts') {
    const count = tab.count ?? 0;
    if (count > 0) {
      return `${count} ${tab.label}`;
    }
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
  const brandMotionActive = createMemo(() => props.connectionStatus().tone === 'healthy');

  const [headerVisible, setHeaderVisible] = createSignal(true);
  const [skipLinkFocused, setSkipLinkFocused] = createSignal(false);
  let headerEl: HTMLDivElement | undefined;
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
    ai: 'Needs Attention',
    settings: 'Settings',
  };
  createEffect(() => {
    const active = getActiveTabForPath(location.pathname);
    document.title = active ? `${tabTitleByActive[active]} · Pulse` : 'Pulse';
  });

  const toggleKioskMode = () => {
    setKioskMode(!kioskMode());
  };

  const platformNavigationVisibility = createMemo(() =>
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
      if (event.key === 'Escape') {
        toggleKioskMode();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    onCleanup(() => window.removeEventListener('keydown', onKeyDown));
  });

  createEffect(() => {
    if (!kioskMode()) return;
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

  const getActiveTabDesktop = () => getActiveTabForPath(location.pathname);
  const getActiveTabMobile = () => getActiveTabForPath(location.pathname);

  // Platform/runtime nav is resource-admitted. A platform or runtime lens only
  // appears when the support manifest says the surface is supported and the
  // current resource snapshot proves that surface is actually present.
  const primaryTabs = createMemo<PrimaryTab[]>(() => {
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

  const utilityTabs = createMemo(() => {
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

    const scopes = props.tokenScopes();
    const hasSettingsAccess =
      !scopes || scopes.length === 0 || scopes.includes('*') || scopes.includes('settings:read');

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
        label: 'Needs Attention',
        route: '/patrol',
        tooltip: 'Review issues and approvals that need action',
        badge: null,
        count: undefined,
        breakdown: undefined,
        icon: PulsePatrolLogo,
      },
    ];

    if (hasSettingsAccess) {
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

  const handlePrimaryClick = (tab: PrimaryTab) => {
    const targetRoute = tab.enabled ? tab.route : tab.settingsRoute;
    void (async () => {
      try {
        await preloadRouteModule(targetRoute);
      } catch (error) {
        logger.warn('Failed to preload navigation target', {
          route: targetRoute,
          error: error instanceof Error ? error.message : String(error),
        });
      }
      navigate(targetRoute);
    })();
  };

  const handleUtilityClick = (tab: UtilityTab) => {
    void (async () => {
      try {
        await preloadRouteModule(tab.route);
      } catch (error) {
        logger.warn('Failed to preload navigation target', {
          route: tab.route,
          error: error instanceof Error ? error.message : String(error),
        });
      }
      navigate(tab.route);
    })();
  };

  const warmNavigationTarget = (route: string) => {
    void preloadRouteModule(route).catch((error) => {
      logger.warn('Failed to warm navigation target', {
        route,
        error: error instanceof Error ? error.message : String(error),
      });
    });
  };

  const getPrimaryTargetRoute = (tab: PrimaryTab) => {
    if (tab.enabled) {
      return tab.route;
    }
    return tab.settingsRoute;
  };

  const renderPrimaryNavigationTab = (tab: PrimaryTab) => {
    const isActive = () => getActiveTabDesktop() === tab.id;
    const disabled = () => !tab.enabled;
    const Icon = tab.icon;
    const baseClasses =
      'tab relative px-1.5 sm:px-3 py-1.5 text-xs sm:text-sm font-medium flex items-center gap-1 sm:gap-1.5 rounded-t border border-transparent transition-colors whitespace-nowrap cursor-pointer';

    const className = () => {
      if (isActive()) {
        return `${baseClasses} bg-surface text-blue-600 dark:text-blue-400 border-border border-b border-b-surface shadow-sm font-semibold`;
      }
      if (disabled()) {
        return `${baseClasses} cursor-not-allowed text-muted opacity-70 bg-base`;
      }
      return `${baseClasses} text-muted hover:text-base-content hover:bg-surface-hover`;
    };

    const title = () =>
      disabled() ? `${tab.label} is not configured yet. Click to open settings.` : tab.tooltip;

    return (
      <div
        class={className()}
        role="tab"
        tabIndex={0}
        aria-label={tab.label}
        aria-disabled={disabled()}
        onMouseEnter={() => warmNavigationTarget(getPrimaryTargetRoute(tab))}
        onClick={() => handlePrimaryClick(tab)}
        onKeyDown={(event) => {
          if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault();
            handlePrimaryClick(tab);
          }
        }}
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
      </div>
    );
  };

  return (
    <div
      class={`pulse-shell ${layoutStore.isFullWidth() || kioskMode() ? 'pulse-shell--full-width' : ''} ${!kioskMode() ? 'pb-safe-or-20 lg:pb-0' : ''}`}
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
        class={`header mb-3 flex items-center gap-2 ${
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
          <div class="flex items-center gap-2 sm:flex-initial sm:gap-2 sm:col-start-2 sm:col-end-3 sm:justify-self-center">
            <div
              class={`pulse-brand-lockup flex items-center gap-2 ${brandMotionActive() ? 'animate-pulse-brand' : ''}`}
              data-testid="pulse-brand-lockup"
            >
              <svg
                width="20"
                height="20"
                viewBox="0 0 256 256"
                xmlns="http://www.w3.org/2000/svg"
                class="pulse-brand-logo"
              >
                <title>Pulse Logo</title>
                <circle
                  class="pulse-bg fill-blue-600 dark:fill-blue-500"
                  cx="128"
                  cy="128"
                  r="122"
                />
                <circle
                  class="pulse-ring fill-none stroke-white stroke-[14] opacity-[0.92]"
                  cx="128"
                  cy="128"
                  r="84"
                />
                <circle
                  class="pulse-center fill-white dark:fill-[#dbeafe]"
                  cx="128"
                  cy="128"
                  r="26"
                />
              </svg>
              <span class="pulse-brand-wordmark text-lg font-medium text-base-content">Pulse</span>
              <Show when={props.versionInfo()?.channel === 'rc'}>
                <span class="text-xs px-1.5 py-0.5 bg-orange-500 text-white rounded font-bold">
                  Preview
                </span>
              </Show>
            </div>
          </div>
        </Show>
        <div
          class={`header-controls flex items-center gap-2 ${kioskMode() ? '' : 'justify-end sm:col-start-3 sm:col-end-4 sm:w-auto sm:justify-end sm:justify-self-end'}`}
        >
          <Show when={props.hasAuth() && !props.needsAuth()}>
            <div class="flex items-center gap-2">
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
                class={`group relative flex h-11 w-11 items-center justify-center rounded-full text-xs transition focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 sm:h-10 sm:w-10 ${
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
                class="group relative flex h-11 w-11 items-center justify-center rounded-full bg-surface-hover text-xs text-base-content transition hover:bg-border focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 sm:h-10 sm:w-10"
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
        <div
          class="tabs mb-2 hidden lg:flex items-end gap-2 overflow-x-auto overflow-y-hidden whitespace-nowrap border-b border-border scrollbar-hide"
          role="tablist"
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
                    'tab relative px-1.5 sm:px-3 py-1.5 text-xs sm:text-sm font-medium flex items-center gap-1 sm:gap-1.5 rounded-t border border-transparent transition-colors whitespace-nowrap cursor-pointer';

                  const className = () => {
                    if (isActive()) {
                      return `${baseClasses} bg-surface text-blue-600 dark:text-blue-400 border-border border-b border-b-surface shadow-sm font-semibold`;
                    }
                    return `${baseClasses} text-muted hover:text-base-content hover:bg-surface-hover`;
                  };

                  return (
                    <div
                      class={className()}
                      role="tab"
                      tabIndex={0}
                      aria-label={getDesktopUtilityTabAriaLabel(tab)}
                      aria-disabled={false}
                      onMouseEnter={() => warmNavigationTarget(tab.route)}
                      onClick={() => handleUtilityClick(tab)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' || event.key === ' ') {
                          event.preventDefault();
                          handleUtilityClick(tab);
                        }
                      }}
                      title={tab.tooltip}
                    >
                      <span aria-hidden="true" class="inline-flex items-center justify-center">
                        <Icon class={NAV_TAB_ICON_CLASS} />
                      </span>
                      <span class="flex items-center gap-1">
                        <span class="hidden xs:inline">{tab.label}</span>
                        <span class="xs:hidden">{tab.label.charAt(0)}</span>
                        {tab.id === 'alerts' &&
                          (() => {
                            const total = tab.count ?? 0;
                            if (total <= 0) {
                              return null;
                            }
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
                    </div>
                  );
                }}
              </For>
            </div>
          </div>
        </div>
      </Show>

      <main
        id="main"
        class="tab-content block bg-surface rounded-b rounded-tr rounded-tl shadow mb-2"
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
          onPrimaryClick={handlePrimaryClick}
          onUtilityClick={handleUtilityClick}
        />
      </Show>

      <Show when={!kioskMode()}>
        <footer class="px-4 py-4 text-xs leading-relaxed text-muted">
          <div class="text-center">
            <span>Pulse | Version: </span>
            <a
              href="https://github.com/rcourtman/Pulse/releases"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex min-h-10 sm:min-h-9 items-center break-all rounded px-1 py-1 text-blue-600 dark:text-blue-400 hover:underline"
            >
              {props.versionInfo()?.version || 'loading...'}
            </a>
            {props.versionInfo()?.isDevelopment && ' (Development)'}
            {props.versionInfo()?.isDocker && ' - Docker'}
          </div>
          <div class="mt-1 flex flex-wrap items-center justify-center gap-x-2 gap-y-1 text-center">
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
                class="inline-flex min-h-10 sm:min-h-9 items-center rounded px-1 py-1 text-blue-600 dark:text-blue-400 hover:underline"
              >
                Get Support
              </a>
            </Show>
          </div>
        </footer>
      </Show>

      <Show
        when={
          aiChatStore.enabled === true &&
          !aiChatStore.isOpenSignal() &&
          !kioskMode() &&
          !dialogStackHasBlockingDialog()
        }
      >
        <button
          type="button"
          onClick={() => aiChatStore.toggle()}
          class={AI_CHAT_LAUNCHER_BUTTON_CLASS}
          title={getAIChatLauncherTitle(aiChatStore.context.context?.name)}
          aria-label={AI_CHAT_LAUNCHER_ARIA_LABEL}
        >
          <SparklesIcon class="h-5 w-5 flex-shrink-0" />
        </button>
      </Show>
    </div>
  );
}
