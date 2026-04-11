import { createSignal, Show, For, createMemo, createEffect, onMount, onCleanup } from 'solid-js';
import { useBeforeLeave } from '@solidjs/router';
import type { JSX } from 'solid-js';

import {
  hasFeature,
  runtimeCapabilitiesLoaded,
  runtimeCapabilitiesLoading as entitlementsLoading,
  loadRuntimeCapabilities,
} from '@/stores/license';
import { useLocation, useNavigate } from '@solidjs/router';
import { logger } from '@/utils/logger';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';

import { notificationStore } from '@/stores/notifications';
import Calendar from 'lucide-solid/icons/calendar';

import { useWebSocket } from '@/contexts/appRuntime';
import { useResources } from '@/hooks/useResources';
import { aiChatStore } from '@/stores/aiChat';
import { trackPaywallViewed } from '@/utils/upgradeMetrics';
import {
  getAlertActivationFailure,
  getAlertActivationPresentation,
  getAlertActivationSuccess,
  getAlertDeactivationFailure,
  getAlertDeactivationSuccess,
} from '@/utils/alertActivationPresentation';
import {
  getAlertsTabGroups,
  getAlertsMobileTabClass,
  getAlertsSidebarTabClass,
  getAlertsTabTitle,
  isAlertsConfigurationTab,
} from '@/utils/alertTabsPresentation';
import {
  getAlertsPageHeaderMeta,
} from '@/utils/alertOverviewPresentation';
import {
  getAlertConfigLeaveConfirmation,
} from '@/utils/alertConfigPresentation';
import { useAlertsActivation } from '@/stores/alertsActivation';
import LayoutDashboard from 'lucide-solid/icons/layout-dashboard';
import History from 'lucide-solid/icons/history';
import Gauge from 'lucide-solid/icons/gauge';
import Send from 'lucide-solid/icons/send';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import { AlertsConfigurationSurface } from '@/features/alerts/AlertsConfigurationSurface';
import { OverviewTab } from '@/features/alerts/OverviewTab';
import { HistoryTab } from '@/features/alerts/tabs/HistoryTab';
import {
  pathForTab,
  tabFromPath,
  type AlertTab,
  type Override,
} from '@/features/alerts/types';

export function Alerts() {
  const { activeAlerts, updateAlert, removeAlerts } = useWebSocket();
  const { get: getResource, resources: allResources, byType, children } = useResources();
  const navigate = useNavigate();
  const location = useLocation();
  const alertsActivation = useAlertsActivation();
  const [isSwitchingActivation, setIsSwitchingActivation] = createSignal(false);
  const readOnlySession = createMemo(() => presentationPolicyIsReadOnly());
  const isAlertsActive = createMemo(() => alertsActivation.activationState() === 'active');
  const areAlertsDisabled = createMemo(() => !isAlertsActive());
  const alertsConfigurationLocked = createMemo(
    () => !readOnlySession() && areAlertsDisabled(),
  );
  const alertActivationPresentation = createMemo(() =>
    getAlertActivationPresentation({
      isActive: isAlertsActive(),
      isBusy: alertsActivation.isLoading() || isSwitchingActivation(),
    }),
  );

  const handleActivateAlerts = async () => {
    if (alertsActivation.isLoading() || isSwitchingActivation()) {
      return;
    }
    setIsSwitchingActivation(true);
    try {
      const success = await alertsActivation.activate();
      if (success) {
        notificationStore.success(getAlertActivationSuccess());
        try {
          await alertsActivation.refreshActiveAlerts();
        } catch (error) {
          logger.error('Failed to refresh alerts after activation', error);
        }
      } else {
        notificationStore.error(getAlertActivationFailure());
      }
    } finally {
      setIsSwitchingActivation(false);
    }
  };

  const handleDeactivateAlerts = async () => {
    if (isSwitchingActivation()) {
      return;
    }
    setIsSwitchingActivation(true);
    try {
      const success = await alertsActivation.deactivate();
      if (success) {
        notificationStore.success(getAlertDeactivationSuccess());
        try {
          await alertsActivation.refreshActiveAlerts();
        } catch (error) {
          logger.error('Failed to refresh alerts after deactivation', error);
        }
      } else {
        notificationStore.error(getAlertDeactivationFailure());
      }
    } catch (error) {
      logger.error('Deactivate alerts failed', error);
      notificationStore.error(getAlertDeactivationFailure());
    } finally {
      setIsSwitchingActivation(false);
    }
  };

  const [activeTab, setActiveTab] = createSignal<AlertTab>(tabFromPath(location.pathname));
  const [overviewOverrides, setOverviewOverrides] = createSignal<Override[]>([]);
  const alertsPageHeaderMeta = getAlertsPageHeaderMeta();

  const headerMeta = () =>
    alertsPageHeaderMeta[activeTab()] ?? alertsPageHeaderMeta.default;

  createEffect(() => {
    const currentPath = location.pathname;
    const requestedTab = tabFromPath(currentPath);
    const tab =
      readOnlySession() && isAlertsConfigurationTab(requestedTab) ? 'overview' : requestedTab;

    if (tab !== activeTab()) {
      setActiveTab(tab);
    }

    const expectedPath = pathForTab(tab);

    // Allow sub-paths for thresholds tab (e.g., /alerts/thresholds/infrastructure)
    const isThresholdsSubPath =
      tab === 'thresholds' && currentPath.startsWith('/alerts/thresholds/');

    if (currentPath !== expectedPath && !isThresholdsSubPath) {
      navigate(expectedPath, { replace: true });
    }
  });

  createEffect(() => {
    const activation = alertsActivation.activationState();
    if (activation === null || readOnlySession()) {
      return;
    }
    if (activation !== 'active' && activeTab() !== 'overview') {
      handleTabChange('overview');
    }
  });

  const handleTabChange = (tab: AlertTab) => {
    const targetPath = pathForTab(tab);
    if (location.pathname !== targetPath) {
      navigate(targetPath);
    }
  };

  const [hasUnsavedChanges, setHasUnsavedChanges] = createSignal(false);
  const [showAcknowledged, setShowAcknowledged] = createSignal(true);
  // Quick tip visibility state
  const [showQuickTip, setShowQuickTip] = createSignal(
    localStorage.getItem('hideAlertsQuickTip') !== 'true',
  );

  const runtimeCapabilitiesLoading = createMemo(() => !runtimeCapabilitiesLoaded() || entitlementsLoading());
  const hasAIAlertsFeature = createMemo(() => !runtimeCapabilitiesLoaded() || hasFeature('ai_alerts'));

  createEffect((wasPaywallVisible) => {
    const isPaywallVisible =
      runtimeCapabilitiesLoaded() && aiChatStore.enabled === true && !hasFeature('ai_alerts');
    if (isPaywallVisible && !wasPaywallVisible) {
      trackPaywallViewed('ai_alerts', 'alerts_page');
    }
    return isPaywallVisible;
  }, false);

  onMount(() => {
    void loadRuntimeCapabilities();
  });

  const dismissQuickTip = () => {
    setShowQuickTip(false);
    localStorage.setItem('hideAlertsQuickTip', 'true');
  };

  // Add beforeunload listener to warn about unsaved changes
  createEffect(() => {
    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      if (hasUnsavedChanges()) {
        e.preventDefault();
        e.returnValue = ''; // Standard way to show confirmation dialog
      }
    };

    window.addEventListener('beforeunload', handleBeforeUnload);
    onCleanup(() => {
      window.removeEventListener('beforeunload', handleBeforeUnload);
    });
  });

  // Warn when navigating within the app
  useBeforeLeave((e) => {
    if (hasUnsavedChanges()) {
      if (!confirm(getAlertConfigLeaveConfirmation())) {
        e.preventDefault();
      }
    }
  });

  const tabGroups = createMemo<
    {
      id: 'status' | 'configuration';
      label: string;
      items: { id: AlertTab; label: string; icon: JSX.Element }[];
    }[]
  >(() =>
    getAlertsTabGroups({ readOnly: readOnlySession() }).map((group) => ({
      ...group,
      items: group.items.map((item) => ({
        ...item,
        icon:
          item.id === 'overview' ? (
            <LayoutDashboard class="w-4 h-4" strokeWidth={2} />
          ) : item.id === 'history' ? (
            <History class="w-4 h-4" strokeWidth={2} />
          ) : item.id === 'thresholds' ? (
            <Gauge class="w-4 h-4" strokeWidth={2} />
          ) : item.id === 'destinations' ? (
            <Send class="w-4 h-4" strokeWidth={2} />
          ) : (
            <Calendar class="w-4 h-4" strokeWidth={2} />
          ),
      })),
    })),
  );

  const flatTabs = createMemo(() => tabGroups().flatMap((group) => group.items));
  // Sidebar always starts expanded for discoverability (consistent with Settings)
  // Users can collapse during session but it resets on page reload
  const [sidebarCollapsed, setSidebarCollapsed] = createSignal(false);

  return (
    <div class="space-y-4">
      {/* Header with better styling */}
      <Card padding="md">
        <div class="flex items-center justify-between gap-4">
          <SectionHeader
            title={headerMeta().title}
            description={headerMeta().description}
            size="lg"
          />
          <Show when={activeTab() === 'overview' && !readOnlySession()}>
            <div class="flex items-center gap-3">
              <span class={`text-sm font-medium ${alertActivationPresentation().labelClass}`}>
                {alertActivationPresentation().label}
              </span>
              <label class="relative inline-flex items-center cursor-pointer">
                <span class="sr-only">Toggle alerts</span>
                <input
                  type="checkbox"
                  class="sr-only peer"
                  checked={isAlertsActive()}
                  disabled={alertsActivation.isLoading() || isSwitchingActivation()}
                  onChange={(event) => {
                    if (event.currentTarget.checked) {
                      void handleActivateAlerts();
                    } else {
                      void handleDeactivateAlerts();
                    }
                  }}
                />
                <div class={alertActivationPresentation().trackClass}>
                  <span class={alertActivationPresentation().thumbClass} />
                </div>
              </label>
            </div>
          </Show>
        </div>
      </Card>

      <div>
        <Card padding="none" class="relative lg:flex overflow-hidden">
          <div
            class={`hidden lg:flex lg:flex-col ${sidebarCollapsed() ? 'w-16' : 'w-72'} ${sidebarCollapsed() ? 'lg:min-w-[4rem] lg:max-w-[4rem] lg:basis-[4rem]' : 'lg:min-w-[18rem] lg:max-w-[18rem] lg:basis-[18rem]'} relative border-b border-border lg:border-b-0 lg:border-r lg:align-top flex-shrink-0 transition-all duration-200`}
            aria-label="Alerts navigation"
            aria-expanded={!sidebarCollapsed()}
          >
            <div
              class={`sticky top-0 ${sidebarCollapsed() ? 'px-2' : 'px-4'} py-5 space-y-5 transition-all duration-200`}
            >
              <Show when={!sidebarCollapsed()}>
                <div class="flex items-center justify-between pb-2 border-b border-border">
                  <h2 class="text-sm font-semibold text-base-content">Alerts</h2>
                  <button
                    type="button"
                    onClick={() => setSidebarCollapsed(true)}
                    class="p-1 rounded-md hover:bg-surface-hover transition-colors"
                    aria-label="Collapse sidebar"
                  >
                    <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M11 19l-7-7 7-7m8 14l-7-7 7-7"
                      />
                    </svg>
                  </button>
                </div>
              </Show>
              <Show when={sidebarCollapsed()}>
                <button
                  type="button"
                  onClick={() => setSidebarCollapsed(false)}
                  class="w-full p-2 rounded-md hover:bg-surface-hover transition-colors"
                  aria-label="Expand sidebar"
                >
                  <svg
                    class="w-5 h-5 mx-auto"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M13 5l7 7-7 7M5 5l7 7-7 7"
                    />
                  </svg>
                </button>
              </Show>
              <div id="alerts-sidebar-menu" class="space-y-5">
                <For each={tabGroups()}>
                  {(group) => (
                    <div class="space-y-2">
                      <Show when={!sidebarCollapsed()}>
                        <p class="text-xs font-semibold uppercase tracking-wide text-muted">
                          {group.label}
                        </p>
                      </Show>
                      <div class="space-y-1.5">
                        <For each={group.items}>
                          {(item) => (
                            <button
                              type="button"
                              aria-current={activeTab() === item.id ? 'page' : undefined}
                              aria-disabled={alertsConfigurationLocked()}
                              disabled={alertsConfigurationLocked()}
                              class={getAlertsSidebarTabClass({
                                isActive: activeTab() === item.id,
                                isDisabled: alertsConfigurationLocked(),
                                collapsed: sidebarCollapsed(),
                              })}
                              onClick={() => handleTabChange(item.id)}
                              title={getAlertsTabTitle({
                                isDisabled: alertsConfigurationLocked(),
                                collapsed: sidebarCollapsed(),
                                label: item.label,
                              })}
                            >
                              {item.icon}
                              <Show when={!sidebarCollapsed()}>
                                <span class="truncate">{item.label}</span>
                              </Show>
                            </button>
                          )}
                        </For>
                      </div>
                    </div>
                  )}
                </For>
              </div>
            </div>
          </div>

          <div class="flex-1 overflow-hidden">
            <Show when={flatTabs().length > 0}>
              <div class="lg:hidden border-b border-border">
                <div class="p-1">
                  <div
                    class="flex rounded-md bg-surface-hover p-0.5 w-full overflow-x-auto"
                    style="-webkit-overflow-scrolling: touch;"
                  >
                    <For each={flatTabs()}>
                      {(tab) => (
                        <button
                          type="button"
                          aria-disabled={alertsConfigurationLocked()}
                          disabled={alertsConfigurationLocked()}
                          class={getAlertsMobileTabClass({
                            isActive: activeTab() === tab.id,
                            isDisabled: alertsConfigurationLocked(),
                          })}
                          onClick={() => handleTabChange(tab.id)}
                          title={getAlertsTabTitle({
                            isDisabled: alertsConfigurationLocked(),
                            label: tab.label,
                          })}
                        >
                          <span class="w-full text-center truncate block">{tab.label}</span>
                        </button>
                      )}
                    </For>
                  </div>
                </div>
              </div>
            </Show>

            {/* Tab Content */}
            <div class="p-2 sm:p-6">
              <Show when={activeTab() === 'overview'}>
                <OverviewTab
                  overrides={overviewOverrides()}
                  activeAlerts={activeAlerts}
                  updateAlert={updateAlert}
                  showQuickTip={showQuickTip}
                  dismissQuickTip={dismissQuickTip}
                  showAcknowledged={showAcknowledged}
                  setShowAcknowledged={setShowAcknowledged}
                  alertsDisabled={alertsConfigurationLocked}
                  hasAIAlertsFeature={hasAIAlertsFeature}
                  runtimeCapabilitiesLoading={runtimeCapabilitiesLoading}
                />
              </Show>

              <Show when={!readOnlySession()}>
                <AlertsConfigurationSurface
                  activeTab={activeTab}
                  allResources={allResources}
                  byType={byType}
                  children={children}
                  activeAlerts={activeAlerts}
                  removeAlerts={removeAlerts}
                  setOverviewOverrides={setOverviewOverrides}
                  hasUnsavedChanges={hasUnsavedChanges}
                  setHasUnsavedChanges={setHasUnsavedChanges}
                  alertsActivationState={alertsActivation.activationState}
                  alertsActivationConfig={alertsActivation.config}
                />
              </Show>

              <Show when={activeTab() === 'history'}>
                <HistoryTab
                  hasAIAlertsFeature={hasAIAlertsFeature}
                  runtimeCapabilitiesLoading={runtimeCapabilitiesLoading}
                  getResource={getResource}
                  allResources={allResources}
                />
              </Show>
            </div>
          </div>
        </Card>
      </div>
    </div>
  );
}

// Overview Tab - Shows current alert status
// Thresholds Tab - Improved design
