import {
  For,
  Show,
  Suspense,
  createEffect,
  createMemo,
  createSignal,
  onCleanup,
} from 'solid-js';
import type { JSX } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import BoxesIcon from 'lucide-solid/icons/boxes';
import LayoutDashboardIcon from 'lucide-solid/icons/layout-dashboard';
import ServerIcon from 'lucide-solid/icons/server';
import HardDriveIcon from 'lucide-solid/icons/hard-drive';
import ArchiveIcon from 'lucide-solid/icons/archive';
import BellIcon from 'lucide-solid/icons/bell';
import SettingsIcon from 'lucide-solid/icons/settings';
import Maximize2Icon from 'lucide-solid/icons/maximize-2';
import Minimize2Icon from 'lucide-solid/icons/minimize-2';
import ActivityIcon from 'lucide-solid/icons/activity';
import { MobileNavBar } from '@/components/shared/MobileNavBar';
import { OrgSwitcher } from '@/components/OrgSwitcher';
import { PulsePatrolLogo } from '@/components/Brand/PulsePatrolLogo';
import { MONITORING_READ_SCOPE } from '@/constants/apiScopes';
import type { Organization } from '@/api/orgs';
import type { VersionInfo } from '@/api/updates';
import type { Alert, State } from '@/types/api';
import { useKioskMode } from '@/hooks/useKioskMode';
import { layoutStore } from '@/utils/layout';
import { getActiveTabForPath } from '@/routing/navigation';
import {
  buildInfrastructurePath,
  buildWorkloadsPath,
  DASHBOARD_PATH,
} from '@/routing/resourceLinks';
import { buildStorageRecoveryTabSpecs } from '@/routing/platformTabs';
import { getKioskModePreference, setKioskMode } from '@/utils/url';
import { updateStore } from '@/stores/updates';
import { aiChatStore } from '@/stores/aiChat';
import { isMultiTenantEnabled, isPro } from '@/stores/license';
import type { AppConnectionStatus } from '@/useAppRuntimeState';

const ROOT_INFRASTRUCTURE_PATH = buildInfrastructurePath();
const ROOT_WORKLOADS_PATH = buildWorkloadsPath();

type PlatformTab = {
  id: string;
  label: string;
  route: string;
  settingsRoute: string;
  tooltip: string;
  enabled: boolean;
  live: boolean;
  icon: JSX.Element;
  alwaysShow: boolean;
  badge?: string;
};

type UtilityTab = {
  id: 'alerts' | 'ai' | 'operations' | 'settings';
  label: string;
  route: string;
  tooltip: string;
  badge: 'update' | 'pro' | null;
  count: number | undefined;
  breakdown: { warning: number; critical: number } | undefined;
  icon: JSX.Element;
};

export interface AppLayoutProps {
  connectionStatus: () => AppConnectionStatus;
  dataUpdated: () => boolean;
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
    return 'disconnected bg-surface-hover text-base-content min-w-6 h-6 group-hover:px-3';
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

  const [headerVisible, setHeaderVisible] = createSignal(true);
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

  const toggleKioskMode = () => {
    setKioskMode(!kioskMode());
  };

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
    const blockedPrefixes = ['/settings', '/operations', '/ai'];
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

    if ((isBlocked || isAlertConfigTab) && normalizedPath !== DASHBOARD_PATH) {
      navigate(DASHBOARD_PATH, { replace: true });
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

  const platformTabsDesktop = createMemo(() => {
    const allPlatforms: PlatformTab[] = [
      {
        id: 'dashboard',
        label: 'Dashboard',
        route: DASHBOARD_PATH,
        settingsRoute: '/settings',
        tooltip: 'Environment overview and command center',
        enabled: true,
        live: true,
        icon: <LayoutDashboardIcon class="w-4 h-4 shrink-0" />,
        alwaysShow: true,
      },
      {
        id: 'infrastructure',
        label: 'Infrastructure',
        route: ROOT_INFRASTRUCTURE_PATH,
        settingsRoute: '/settings',
        tooltip: 'All agents and nodes across platforms',
        enabled: true,
        live: true,
        icon: <ServerIcon class="w-4 h-4 shrink-0" />,
        alwaysShow: true,
      },
      {
        id: 'workloads',
        label: 'Workloads',
        route: ROOT_WORKLOADS_PATH,
        settingsRoute: '/settings/workloads/docker',
        tooltip: 'VMs, containers, and Kubernetes workloads',
        enabled: true,
        live: true,
        icon: <BoxesIcon class="w-4 h-4 shrink-0" />,
        alwaysShow: true,
      },
      ...buildStorageRecoveryTabSpecs().map((tab) => ({
        ...tab,
        enabled: true,
        live: true,
        icon: tab.id.startsWith('storage') ? (
          <HardDriveIcon class="w-4 h-4 shrink-0" />
        ) : (
          <ArchiveIcon class="w-4 h-4 shrink-0" />
        ),
        alwaysShow: true,
      })),
    ];

    return allPlatforms.filter((platform) => platform.alwaysShow || platform.enabled);
  });

  const platformTabsMobile = createMemo(() => {
    const allPlatforms: PlatformTab[] = [
      {
        id: 'dashboard',
        label: 'Dashboard',
        route: DASHBOARD_PATH,
        settingsRoute: '/settings',
        tooltip: 'Environment overview and command center',
        enabled: true,
        live: true,
        icon: <LayoutDashboardIcon class="w-4 h-4 shrink-0" />,
        alwaysShow: true,
      },
      {
        id: 'infrastructure',
        label: 'Infrastructure',
        route: ROOT_INFRASTRUCTURE_PATH,
        settingsRoute: '/settings',
        tooltip: 'All agents and nodes across platforms',
        enabled: true,
        live: true,
        icon: <ServerIcon class="w-4 h-4 shrink-0" />,
        alwaysShow: true,
      },
      {
        id: 'workloads',
        label: 'Workloads',
        route: ROOT_WORKLOADS_PATH,
        settingsRoute: '/settings/workloads/docker',
        tooltip: 'VMs, containers, and Kubernetes workloads',
        enabled: true,
        live: true,
        icon: <BoxesIcon class="w-4 h-4 shrink-0" />,
        alwaysShow: true,
      },
      ...buildStorageRecoveryTabSpecs().map((tab) => ({
        ...tab,
        enabled: true,
        live: true,
        icon: tab.id.startsWith('storage') ? (
          <HardDriveIcon class="w-4 h-4 shrink-0" />
        ) : (
          <ArchiveIcon class="w-4 h-4 shrink-0" />
        ),
        alwaysShow: true,
      })),
    ];

    return allPlatforms.filter((platform) => platform.alwaysShow || platform.enabled);
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
        icon: <BellIcon class="w-4 h-4 shrink-0" />,
      },
      {
        id: 'ai',
        label: 'Patrol',
        route: '/ai',
        tooltip: 'Pulse Patrol monitoring and analysis',
        badge: null,
        count: undefined,
        breakdown: undefined,
        icon: <PulsePatrolLogo class="w-4 h-4 shrink-0" />,
      },
      {
        id: 'operations',
        label: 'Operations',
        route: '/operations',
        tooltip: 'System operations, diagnostics, and reporting',
        badge: null,
        count: undefined,
        breakdown: undefined,
        icon: <ActivityIcon class="w-4 h-4 shrink-0" />,
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
        icon: <SettingsIcon class="w-4 h-4 shrink-0" />,
      });
    }

    return tabs;
  });

  const handlePlatformClick = (platform: PlatformTab) => {
    if (platform.enabled) {
      navigate(platform.route);
    } else {
      navigate(platform.settingsRoute);
    }
  };

  const handleUtilityClick = (tab: UtilityTab) => {
    navigate(tab.route);
  };

  return (
    <div
      class={`pulse-shell ${layoutStore.isFullWidth() || kioskMode() ? 'pulse-shell--full-width' : ''} ${!kioskMode() ? 'pb-safe-or-20 md:pb-0' : ''}`}
    >
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
            <div class="flex items-center gap-2">
              <svg
                width="20"
                height="20"
                viewBox="0 0 256 256"
                xmlns="http://www.w3.org/2000/svg"
                class={`pulse-logo ${props.connectionStatus().kind === 'connected' && props.dataUpdated() ? 'animate-pulse-logo' : ''}`}
              >
                <title>Pulse Logo</title>
                <circle class="pulse-bg fill-blue-600 dark:fill-blue-500" cx="128" cy="128" r="122" />
                <circle
                  class="pulse-ring fill-none stroke-white stroke-[14] opacity-[0.92]"
                  cx="128"
                  cy="128"
                  r="84"
                />
                <circle class="pulse-center fill-white dark:fill-[#dbeafe]" cx="128" cy="128" r="26" />
              </svg>
              <span class="text-lg font-medium text-base-content">Pulse</span>
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
              <Show when={isMultiTenantEnabled()}>
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
                <Show
                  when={kioskMode()}
                  fallback={<Maximize2Icon class="h-4 w-4 flex-shrink-0" />}
                >
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
          <ConnectionStatusBadge
            connectionStatus={props.connectionStatus}
            class="flex-shrink-0"
          />
        </div>
      </div>

      <Show when={!kioskMode()}>
        <div
          class="tabs mb-2 hidden md:flex items-end gap-2 overflow-x-auto overflow-y-hidden whitespace-nowrap border-b border-border scrollbar-hide"
          role="tablist"
          aria-label="Primary navigation"
        >
          <div class="flex items-end gap-1" role="group" aria-label="Infrastructure">
            <For each={platformTabsDesktop()}>
              {(platform) => {
                const isActive = () => getActiveTabDesktop() === platform.id;
                const disabled = () => !platform.enabled;
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
                  disabled()
                    ? `${platform.label} is not configured yet. Click to open settings.`
                    : platform.tooltip;

                return (
                  <div
                    class={className()}
                    role="tab"
                    aria-disabled={disabled()}
                    onClick={() => handlePlatformClick(platform)}
                    title={title()}
                  >
                    {platform.icon}
                    <span class="hidden xs:inline-flex items-center gap-1">
                      <span>{platform.label}</span>
                      <Show when={platform.badge}>
                        <span class="px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-muted bg-surface-hover rounded">
                          {platform.badge}
                        </span>
                      </Show>
                    </span>
                    <span class="xs:hidden">{platform.label.charAt(0)}</span>
                  </div>
                );
              }}
            </For>
          </div>
          <div class="flex items-end gap-1 ml-auto" role="group" aria-label="System">
            <div class="flex items-end gap-1 pl-1 sm:pl-4">
              <For each={utilityTabs()}>
                {(tab) => {
                  const isActive = () => getActiveTabDesktop() === tab.id;
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
                      aria-disabled={false}
                      onClick={() => handleUtilityClick(tab)}
                      title={tab.tooltip}
                    >
                      {tab.icon}
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
                      <Show when={tab.badge === 'pro'}>
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
          platformTabs={platformTabsMobile}
          utilityTabs={utilityTabs}
          onPlatformClick={handlePlatformClick}
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

      <Show when={aiChatStore.enabled === true && !aiChatStore.isOpenSignal() && !kioskMode()}>
        <button
          type="button"
          onClick={() => aiChatStore.toggle()}
          class="fixed right-0 top-1/2 -translate-y-1/2 z-40 flex min-h-10 sm:min-h-9 min-w-10 items-center justify-center px-2.5 py-2.5 rounded-l-xl bg-blue-600 text-white shadow-sm hover:bg-blue-700 transition-colors duration-200 group sm:top-1/2 sm:translate-y-[-50%] top-auto bottom-[calc(5rem+env(safe-area-inset-bottom,0px))] translate-y-0"
          title={
            aiChatStore.context.context?.name
              ? `Pulse Assistant - ${aiChatStore.context.context.name}`
              : 'Pulse Assistant (⌘K)'
          }
          aria-label="Expand Pulse Assistant"
        >
          <svg
            class="h-5 w-5 flex-shrink-0"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
            stroke-width="1.5"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 00-2.456 2.456zM16.894 20.567L16.5 21.75l-.394-1.183a2.25 2.25 0 00-1.423-1.423L13.5 18.75l1.183-.394a2.25 2.25 0 001.423-1.423l.394-1.183.394 1.183a2.25 2.25 0 001.423 1.423l1.183.394-1.183.394a2.25 2.25 0 00-1.423 1.423z"
            />
          </svg>
        </button>
      </Show>
    </div>
  );
}
