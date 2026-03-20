import { createSignal, Show, For, createMemo, createEffect, onMount, onCleanup } from 'solid-js';
import { useBeforeLeave } from '@solidjs/router';
import type { JSX } from 'solid-js';

import type { Resource } from '@/types/resource';
import type { RawOverrideConfig, BackupAlertConfig, SnapshotAlertConfig } from '@/types/alerts';
import { AlertsAPI } from '@/api/alerts';
import { NotificationsAPI } from '@/api/notifications';
import {
  hasFeature,
  licenseLoaded,
  licenseLoading as entitlementsLoading,
  loadLicenseStatus,
} from '@/stores/license';
import { useLocation, useNavigate } from '@solidjs/router';
import { logger } from '@/utils/logger';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';

import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import Calendar from 'lucide-solid/icons/calendar';

import { useWebSocket } from '@/App';
import { useResources } from '@/hooks/useResources';
import { aiChatStore } from '@/stores/aiChat';
import { trackPaywallViewed } from '@/utils/upgradeMetrics';
import type { PBSInstance, PMGInstance } from '@/types/api';
import type { EmailConfig, AppriseConfig } from '@/api/notifications';
import { pbsInstanceFromResource, pmgInstanceFromResource } from '@/utils/resourceStateAdapters';
import { isAppContainerDiscoveryResourceType } from '@/utils/discoveryTarget';
import { getActionableAgentIdFromResource, hasAgentFacet } from '@/utils/agentResources';
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
} from '@/utils/alertTabsPresentation';
import {
  getAlertDestinationsConfigLoadError,
} from '@/utils/alertDestinationsPresentation';
import {
  getAlertsPageHeaderMeta,
} from '@/utils/alertOverviewPresentation';
import {
  getAlertConfigDiscardedSuccess,
  getAlertConfigDiscardLabel,
  getAlertConfigReloadFailure,
  getAlertConfigSaveFailure,
  getAlertConfigSaveSuccess,
  getAlertConfigSaveChangesLabel,
  getAlertConfigUnsavedChangesLabel,
  getAlertConfigLeaveConfirmation,
} from '@/utils/alertConfigPresentation';
import { useAlertsActivation } from '@/stores/alertsActivation';
import LayoutDashboard from 'lucide-solid/icons/layout-dashboard';
import History from 'lucide-solid/icons/history';
import Gauge from 'lucide-solid/icons/gauge';
import Send from 'lucide-solid/icons/send';
import { OverviewTab } from '@/features/alerts/OverviewTab';
import { DestinationsTab } from '@/features/alerts/tabs/DestinationsTab';
import { HistoryTab } from '@/features/alerts/tabs/HistoryTab';
import { ScheduleTab } from '@/features/alerts/tabs/ScheduleTab';
import { ThresholdsTab } from '@/features/alerts/tabs/ThresholdsTab';
import {
  pathForTab,
  tabFromPath,
  type AlertTab,
  type DestinationsRef,
  type Override,
  type UIEmailConfig,
  type UIAppriseConfig,
  type QuietHoursConfig,
  type CooldownConfig,
  type GroupingConfig,
  type EscalationConfig,
  type EscalationNotifyTarget,
  GROUPING_WINDOW_DEFAULT_SECONDS,
  clampCooldownMinutes,
} from '@/features/alerts/types';
import {
  clampMaxAlertsPerHour,
  createDefaultQuietHours,
  createDefaultCooldown,
  createDefaultGrouping,
  createDefaultResolveNotifications,
  createDefaultAppriseConfig,
  createDefaultEmailConfig,
  fallbackMaxAlertsPerHour,
  normalizeEmailConfigFromAPI,
  formatAppriseTargets,
  normalizeMetricDelayMap,
  parseAppriseTargets,
  createDefaultEscalation,
  getTriggerValue,
  extractTriggerValues,
  platformData,
  guessNumericId,
  getAlertResourceDisplayLabel,
  DEFAULT_DELAY_SECONDS,
} from '@/features/alerts/helpers';

export function Alerts() {
  const { activeAlerts, updateAlert, removeAlerts } = useWebSocket();
  const { get: getResource, resources: allResources, byType, children } = useResources();
  const navigate = useNavigate();
  const location = useLocation();
  const alertsActivation = useAlertsActivation();
  const [isSwitchingActivation, setIsSwitchingActivation] = createSignal(false);
  const isAlertsActive = createMemo(() => alertsActivation.activationState() === 'active');
  const areAlertsDisabled = createMemo(() => !isAlertsActive());
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
  const alertsPageHeaderMeta = getAlertsPageHeaderMeta();

  const headerMeta = () =>
    alertsPageHeaderMeta[activeTab()] ?? alertsPageHeaderMeta.default;

  createEffect(() => {
    const currentPath = location.pathname;
    const tab = tabFromPath(currentPath);

    if (tab !== activeTab()) {
      setActiveTab(tab);
    }

    const expectedPath = pathForTab(tab);

    // Allow sub-paths for thresholds tab (e.g., /alerts/thresholds/proxmox)
    const isThresholdsSubPath =
      tab === 'thresholds' && currentPath.startsWith('/alerts/thresholds/');

    if (currentPath !== expectedPath && !isThresholdsSubPath) {
      navigate(expectedPath, { replace: true });
    }
  });

  createEffect(() => {
    const activation = alertsActivation.activationState();
    if (activation === null) {
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
  const [isReloadingConfig, setIsReloadingConfig] = createSignal(false);
  const [isLoadingDestinations, setIsLoadingDestinations] = createSignal(false);
  const [destConfigLoadError, setDestConfigLoadError] = createSignal<string | null>(null);
  const [showAcknowledged, setShowAcknowledged] = createSignal(true);
  // Quick tip visibility state
  const [showQuickTip, setShowQuickTip] = createSignal(
    localStorage.getItem('hideAlertsQuickTip') !== 'true',
  );

  const licenseLoading = createMemo(() => !licenseLoaded() || entitlementsLoading());
  const hasAIAlertsFeature = createMemo(() => !licenseLoaded() || hasFeature('ai_alerts'));

  createEffect((wasPaywallVisible) => {
    const isPaywallVisible =
      licenseLoaded() && aiChatStore.enabled === true && !hasFeature('ai_alerts');
    if (isPaywallVisible && !wasPaywallVisible) {
      trackPaywallViewed('ai_alerts', 'alerts_page');
    }
    return isPaywallVisible;
  }, false);

  onMount(() => {
    void loadLicenseStatus();
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

  // Store references to child component data
  let destinationsRef: DestinationsRef = {};

  const [overrides, setOverrides] = createSignal<Override[]>([]);
  const [rawOverridesConfig, setRawOverridesConfig] = createSignal<
    Record<string, RawOverrideConfig>
  >({}); // Store raw config

  // Email configuration state moved to parent to persist across tab changes
  const [emailConfig, setEmailConfig] = createSignal<UIEmailConfig>({
    enabled: false,
    provider: '',
    server: '', // Fixed: use 'server' not 'smtpHost'
    port: 587, // Fixed: use 'port' not 'smtpPort'
    username: '',
    password: '',
    from: '',
    to: [] as string[],
    tls: true,
    startTLS: false,
    replyTo: '',
    maxRetries: 3,
    retryDelay: 5,
    rateLimit: 60,
  });

  const [appriseConfig, setAppriseConfig] = createSignal<UIAppriseConfig>(
    createDefaultAppriseConfig(),
  );

  // Schedule configuration state moved to parent to persist across tab changes
  const [scheduleQuietHours, setScheduleQuietHours] =
    createSignal<QuietHoursConfig>(createDefaultQuietHours());

  const [scheduleCooldown, setScheduleCooldown] =
    createSignal<CooldownConfig>(createDefaultCooldown());

  const [scheduleGrouping, setScheduleGrouping] =
    createSignal<GroupingConfig>(createDefaultGrouping());

  const [scheduleEscalation, setScheduleEscalation] =
    createSignal<EscalationConfig>(createDefaultEscalation());

  const [notifyOnResolve, setNotifyOnResolve] = createSignal<boolean>(
    createDefaultResolveNotifications(),
  );

  // Set up destinationsRef.emailConfig function immediately
  destinationsRef.emailConfig = () => {
    const config = emailConfig();
    return {
      enabled: config.enabled,
      provider: config.provider,
      server: config.server, // Fixed: use correct property name
      port: config.port, // Fixed: use correct property name
      username: config.username,
      password: config.password,
      from: config.from,
      to: config.to,
      tls: config.tls,
      startTLS: config.startTLS,
    } as EmailConfig;
  };

  destinationsRef.appriseConfig = () => {
    const config = appriseConfig();
    return {
      enabled: config.enabled,
      mode: config.mode,
      targets: parseAppriseTargets(config.targetsText),
      cliPath: config.cliPath,
      timeoutSeconds: config.timeoutSeconds,
      serverUrl: config.serverUrl,
      configKey: config.configKey,
      apiKey: config.apiKey,
      apiKeyHeader: config.apiKeyHeader,
      skipTlsVerify: config.skipTlsVerify,
    } as AppriseConfig;
  };

  const pd = platformData;
  const asRecord = (value: unknown): Record<string, unknown> | undefined =>
    value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;
  const asString = (value: unknown): string | undefined =>
    typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;
  const uniqueIds = (...values: unknown[]): string[] => {
    const ids: string[] = [];
    const seen = new Set<string>();
    values.forEach((value) => {
      const normalized = asString(value);
      if (!normalized || seen.has(normalized)) return;
      seen.add(normalized);
      ids.push(normalized);
    });
    return ids;
  };
  const hostOverrideIdCandidates = (resource: Resource): string[] => {
    const data = pd(resource);
    const agent = asRecord(data?.agent);
    return uniqueIds(
      getActionableAgentIdFromResource(resource),
      resource.discoveryTarget?.agentId,
      resource.agent?.agentId,
      agent?.agentId,
      data?.agentId,
      resource.id,
    );
  };
  const dockerHostOverrideIdCandidates = (resource: Resource): string[] => {
    const data = pd(resource);
    const docker = asRecord(data?.docker);
    const discoveryTarget = resource.discoveryTarget;
    return uniqueIds(
      isAppContainerDiscoveryResourceType(discoveryTarget?.resourceType)
        ? discoveryTarget?.resourceId
        : undefined,
      docker?.hostSourceId,
      data?.hostSourceId,
      discoveryTarget?.agentId,
      resource.id,
    );
  };
  const dockerContainerOverrideIdCandidates = (host: Resource, shortId: string): string[] =>
    uniqueIds(
      ...dockerHostOverrideIdCandidates(host).map((hostId) => `docker:${hostId}/${shortId}`),
    );
  const agentResources = createMemo(() =>
    allResources().filter(
      (resource) =>
        (resource.type === 'agent' ||
          resource.type === 'pbs' ||
          resource.type === 'pmg' ||
          resource.type === 'truenas') &&
        hasAgentFacet(resource),
    ),
  );
  const pbsInstances = createMemo<PBSInstance[]>(() =>
    allResources()
      .filter((resource) => resource.type === 'pbs')
      .map(pbsInstanceFromResource)
      .filter((resource): resource is PBSInstance => Boolean(resource)),
  );
  const pbsInstanceById = createMemo(
    () => new Map(pbsInstances().map((instance) => [instance.id, instance])),
  );
  const pmgInstances = createMemo<PMGInstance[]>(() =>
    allResources()
      .filter((resource) => resource.type === 'pmg')
      .map(pmgInstanceFromResource)
      .filter((resource): resource is PMGInstance => Boolean(resource)),
  );

  // Process raw overrides config when state changes
  createEffect(() => {
    // Skip this effect if there are unsaved changes to prevent losing focus
    if (hasUnsavedChanges()) {
      return;
    }

    const rawConfig = rawOverridesConfig();
    if (Object.keys(rawConfig).length > 0 && byType('agent').length > 0) {
      const nodeResources = byType('agent');
      const vmResources = byType('vm');
      const containerResources = [...byType('system-container'), ...byType('oci-container')];
      const storageResources = allResources().filter(
        (r) => r.type === 'storage' || r.type === 'datastore',
      );
      const agentResourceList = agentResources();
      const dockerHostResources = byType('docker-host');

      // Convert overrides object to array format
      const overridesList: Override[] = [];

      const dockerHostMap = new Map<string, Resource>();
      const dockerContainerMap = new Map<
        string,
        { host: Resource; container: Resource; containerShortId: string }
      >();
      const agentMap = new Map<string, Resource>();

      const storageCoords = (r: Resource): { node: string; instance: string } => {
        const data = pd(r);
        if (r.type === 'datastore') {
          const instance =
            (data?.pbsInstanceId as string | undefined) || r.parentId || r.platformId || 'pbs';
          const node = (data?.pbsInstanceName as string | undefined) || instance;
          return { node, instance };
        }
        return {
          node: (data?.node as string | undefined) || '',
          instance: (data?.instance as string | undefined) || r.platformId || '',
        };
      };

      dockerHostResources.forEach((host) => {
        dockerHostOverrideIdCandidates(host).forEach((id) => {
          dockerHostMap.set(id, host);
        });
        const containers = children(host.id).filter((r) => r.type === 'app-container');
        containers.forEach((container) => {
          const shortId = container.id.includes('/')
            ? container.id.split('/').pop()!
            : container.id;
          dockerContainerOverrideIdCandidates(host, shortId).forEach((resourceId) => {
            dockerContainerMap.set(resourceId, { host, container, containerShortId: shortId });
          });
        });
      });
      agentResourceList.forEach((agentResource) => {
        hostOverrideIdCandidates(agentResource).forEach((id) => {
          agentMap.set(id, agentResource);
        });
      });

      Object.entries(rawConfig).forEach(([key, thresholds]) => {
        // Docker host override stored by host ID
        const dockerHost = dockerHostMap.get(key);
        if (dockerHost) {
          overridesList.push({
            id: key,
            name: getAlertResourceDisplayLabel(dockerHost),
            type: 'dockerHost',
            resourceType: 'Container Runtime',
            disableConnectivity: thresholds.disableConnectivity || false,
            thresholds: extractTriggerValues(thresholds),
          });
          return;
        }

        // Docker container override stored as docker:hostId/containerId
        const dockerContainer = dockerContainerMap.get(key);
        if (dockerContainer) {
          const { host, container, containerShortId } = dockerContainer;
          const containerName = getAlertResourceDisplayLabel(container, containerShortId);
          overridesList.push({
            id: key,
            name: containerName,
            type: 'dockerContainer',
            resourceType: 'Container',
            node: getAlertResourceDisplayLabel(host),
            instance: getAlertResourceDisplayLabel(host),
            disabled: thresholds.disabled || false,
            disableConnectivity: thresholds.disableConnectivity || false,
            poweredOffSeverity:
              thresholds.poweredOffSeverity === 'critical'
                ? 'critical'
                : thresholds.poweredOffSeverity === 'warning'
                  ? 'warning'
                  : undefined,
            thresholds: extractTriggerValues(thresholds),
          });
          return;
        }

        if (key.startsWith('docker:')) {
          // Handle docker overrides where the host/container is no longer reporting
          const [, rest] = key.split(':', 2);
          const [hostId, containerId] = (rest || '').split('/', 2);

          if (containerId) {
            overridesList.push({
              id: key,
              name: containerId,
              type: 'dockerContainer',
              resourceType: 'Container',
              node: hostId,
              disabled: thresholds.disabled || false,
              disableConnectivity: thresholds.disableConnectivity || false,
              poweredOffSeverity:
                thresholds.poweredOffSeverity === 'critical'
                  ? 'critical'
                  : thresholds.poweredOffSeverity === 'warning'
                    ? 'warning'
                    : undefined,
              thresholds: extractTriggerValues(thresholds),
            });
            return;
          }

          overridesList.push({
            id: key,
            name: hostId || key,
            type: 'dockerHost',
            resourceType: 'Container Runtime',
            disableConnectivity: thresholds.disableConnectivity || false,
            thresholds: extractTriggerValues(thresholds),
          });
          return;
        }

        // Agent disk override stored as agent:<agentId>/disk:<mountpoint>.
        const diskMatch = key.match(/^agent:(.+)\/disk:(.+)$/);
        if (diskMatch) {
          const [, agentId, diskLabel] = diskMatch;
          const agent = agentMap.get(agentId);
          const displayName = diskLabel.replace(/-/g, '/');

          overridesList.push({
            id: key,
            name: displayName,
            type: 'agentDisk',
            resourceType: 'Agent Disk',
            node: agent ? getAlertResourceDisplayLabel(agent) : agentId,
            disabled: thresholds.disabled || false,
            thresholds: extractTriggerValues(thresholds),
          });
          return;
        }

        // Agent override stored by agent ID
        const agentResource = agentMap.get(key);
        if (agentResource) {
          const displayName = getAlertResourceDisplayLabel(agentResource);
          const data = pd(agentResource);
          const agent = asRecord(data?.agent);

          overridesList.push({
            id: key,
            name: displayName,
            type: 'agent',
            resourceType: 'Agent',
            node: displayName,
            instance:
              asString(agent?.platform) ||
              asString(agent?.osName) ||
              asString(data?.platform) ||
              asString(data?.osName) ||
              '',
            disabled: thresholds.disabled || false,
            disableConnectivity: thresholds.disableConnectivity || false,
            thresholds: extractTriggerValues(thresholds),
          });
          return;
        }

        // Check if it's a PBS server override (starts with "pbs-")
        if (key.startsWith('pbs-')) {
          const pbs = pbsInstanceById().get(key);
          if (pbs) {
            overridesList.push({
              id: key,
              name: pbs.name,
              type: 'pbs',
              resourceType: 'PBS',
              disableConnectivity: thresholds.disableConnectivity || false,
              thresholds: extractTriggerValues(thresholds),
            });
          }
        } else {
          // Check if it's a node override by looking for matching node
          const node = nodeResources.find((n) => n.id === key);
          if (node) {
            overridesList.push({
              id: key,
              name: getAlertResourceDisplayLabel(node),
              type: 'agent',
              resourceType: 'Agent',
              disableConnectivity: thresholds.disableConnectivity || false,
              thresholds: extractTriggerValues(thresholds),
            });
          } else {
            // Check if it's a storage device
            const storage = storageResources.find((s) => s.id === key);
            if (storage) {
              const coords = storageCoords(storage);
              overridesList.push({
                id: key,
                name: getAlertResourceDisplayLabel(storage),
                type: 'storage',
                resourceType: 'Storage',
                node: coords.node,
                instance: coords.instance,
                disabled: thresholds.disabled || false,
                thresholds: extractTriggerValues(thresholds),
              });
            } else {
              // Find the guest by matching the full ID
              const vm = vmResources.find((g) => g.id === key);
              const container = containerResources.find((g) => g.id === key);
              const guest = vm || container;
              if (guest) {
                const data = pd(guest);
                overridesList.push({
                  id: key,
                  name: getAlertResourceDisplayLabel(guest),
                  type: 'guest',
                  resourceType: guest.type === 'vm' ? 'VM' : 'Container',
                  vmid: (data?.vmid as number | undefined) ?? guessNumericId(guest.id),
                  node: (data?.node as string | undefined) ?? '',
                  instance: (data?.instance as string | undefined) ?? guest.platformId,
                  disabled: thresholds.disabled || false,
                  disableConnectivity: thresholds.disableConnectivity || false,
                  poweredOffSeverity:
                    thresholds.poweredOffSeverity === 'critical'
                      ? 'critical'
                      : thresholds.poweredOffSeverity === 'warning'
                        ? 'warning'
                        : undefined,
                  thresholds: extractTriggerValues(thresholds),
                  backup: thresholds.backup,
                  snapshot: thresholds.snapshot,
                });
              }
            }
          }
        }
      });

      // Only update if there's an actual change to prevent losing edit state
      const currentOverrides = overrides();
      const hasChanged =
        overridesList.length !== currentOverrides.length ||
        overridesList.some((newOverride) => {
          const existing = currentOverrides.find((o) => o.id === newOverride.id);
          if (!existing) return true;
          const thresholdsChanged =
            JSON.stringify(newOverride.thresholds) !== JSON.stringify(existing.thresholds);
          const connectivityChanged =
            Boolean(newOverride.disableConnectivity) !== Boolean(existing.disableConnectivity);
          const disabledChanged = Boolean(newOverride.disabled) !== Boolean(existing.disabled);
          const severityChanged =
            (newOverride.poweredOffSeverity ?? null) !== (existing.poweredOffSeverity ?? null);
          const backupChanged =
            JSON.stringify(newOverride.backup ?? null) !== JSON.stringify(existing.backup ?? null);
          const snapshotChanged =
            JSON.stringify(newOverride.snapshot ?? null) !==
            JSON.stringify(existing.snapshot ?? null);
          return (
            thresholdsChanged ||
            connectivityChanged ||
            disabledChanged ||
            severityChanged ||
            backupChanged ||
            snapshotChanged
          );
        });

      if (hasChanged) {
        setOverrides(overridesList);
      }
    }
  });

  const loadAlertConfiguration = async (options: { notify?: boolean } = {}) => {
    setIsReloadingConfig(true);
    setHasUnsavedChanges(false);
    setDestConfigLoadError(null);

    // Reset to defaults before applying server state
    setGuestDefaults({
      cpu: 80,
      memory: 85,
      disk: 90,
      diskRead: -1,
      diskWrite: -1,
      networkIn: -1,
      networkOut: -1,
    });
    setGuestDisableConnectivity(false);
    setGuestPoweredOffSeverity('warning');
    setNodeDefaults({
      cpu: 80,
      memory: 85,
      disk: 90,
      temperature: 80,
    });
    setStorageDefault(85);
    setTimeThresholds({
      guest: DEFAULT_DELAY_SECONDS,
      node: DEFAULT_DELAY_SECONDS,
      storage: DEFAULT_DELAY_SECONDS,
      pbs: DEFAULT_DELAY_SECONDS,
      agent: DEFAULT_DELAY_SECONDS,
    });
    setMetricTimeThresholds({});
    setScheduleQuietHours(createDefaultQuietHours());
    setScheduleCooldown(createDefaultCooldown());
    setScheduleGrouping(createDefaultGrouping());
    setScheduleEscalation(createDefaultEscalation());

    setEmailConfig(createDefaultEmailConfig());

    setAppriseConfig(createDefaultAppriseConfig());

    try {
      const config = await AlertsAPI.getConfig();

      if (config.guestDefaults) {
        setGuestDefaults({
          cpu: getTriggerValue(config.guestDefaults.cpu) ?? 80,
          memory: getTriggerValue(config.guestDefaults.memory) ?? 85,
          disk: getTriggerValue(config.guestDefaults.disk) ?? 90,
          diskRead: getTriggerValue(config.guestDefaults.diskRead) ?? -1,
          diskWrite: getTriggerValue(config.guestDefaults.diskWrite) ?? -1,
          networkIn: getTriggerValue(config.guestDefaults.networkIn) ?? -1,
          networkOut: getTriggerValue(config.guestDefaults.networkOut) ?? -1,
        });
        setGuestDisableConnectivity(Boolean(config.guestDefaults.disableConnectivity));
        setGuestPoweredOffSeverity(
          config.guestDefaults.poweredOffSeverity === 'critical' ? 'critical' : 'warning',
        );
      } else {
        setGuestDisableConnectivity(false);
        setGuestPoweredOffSeverity('warning');
      }

      if (config.nodeDefaults) {
        setNodeDefaults({
          cpu: getTriggerValue(config.nodeDefaults.cpu) ?? 80,
          memory: getTriggerValue(config.nodeDefaults.memory) ?? 85,
          disk: getTriggerValue(config.nodeDefaults.disk) ?? 90,
          temperature: getTriggerValue(config.nodeDefaults.temperature) ?? 80,
        });
      }

      if (config.pbsDefaults) {
        setPBSDefaults({
          cpu: getTriggerValue(config.pbsDefaults.cpu) ?? 80,
          memory: getTriggerValue(config.pbsDefaults.memory) ?? 85,
        });
      } else {
        setPBSDefaults({ ...FACTORY_PBS_DEFAULTS });
      }

      if (config.agentDefaults) {
        setAgentDefaults({
          cpu: getTriggerValue(config.agentDefaults.cpu) ?? 80,
          memory: getTriggerValue(config.agentDefaults.memory) ?? 85,
          disk: getTriggerValue(config.agentDefaults.disk) ?? 90,
          diskTemperature: getTriggerValue(config.agentDefaults.diskTemperature) ?? 55,
        });
      } else {
        setAgentDefaults({ ...FACTORY_AGENT_DEFAULTS });
      }

      if (config.dockerDefaults) {
        const normalizeGap = (value: unknown, fallback: number) => {
          const numeric = Number(value);
          if (!Number.isFinite(numeric)) {
            return fallback;
          }
          return Math.max(0, Math.min(100, numeric));
        };

        const serviceWarnGap = normalizeGap(
          config.dockerDefaults.serviceWarnGapPercent,
          FACTORY_DOCKER_DEFAULTS.serviceWarnGapPercent,
        );
        let serviceCriticalGap = normalizeGap(
          config.dockerDefaults.serviceCriticalGapPercent,
          FACTORY_DOCKER_DEFAULTS.serviceCriticalGapPercent,
        );
        if (serviceCriticalGap > 0 && serviceWarnGap > serviceCriticalGap) {
          serviceCriticalGap = serviceWarnGap;
        }

        setDockerDefaults({
          cpu: getTriggerValue(config.dockerDefaults.cpu) ?? 80,
          memory: getTriggerValue(config.dockerDefaults.memory) ?? 85,
          disk: getTriggerValue(config.dockerDefaults.disk) ?? FACTORY_DOCKER_DEFAULTS.disk,
          restartCount: config.dockerDefaults.restartCount ?? 3,
          restartWindow: config.dockerDefaults.restartWindow ?? 300,
          memoryWarnPct: config.dockerDefaults.memoryWarnPct ?? 90,
          memoryCriticalPct: config.dockerDefaults.memoryCriticalPct ?? 95,
          serviceWarnGapPercent: serviceWarnGap,
          serviceCriticalGapPercent: serviceCriticalGap,
        });
        setDockerDisableConnectivity(Boolean(config.dockerDefaults.stateDisableConnectivity));
        setDockerPoweredOffSeverity(
          config.dockerDefaults.statePoweredOffSeverity === 'critical' ? 'critical' : 'warning',
        );
      } else {
        setDockerDefaults({ ...FACTORY_DOCKER_DEFAULTS });
        setDockerDisableConnectivity(FACTORY_DOCKER_STATE_DISABLE_CONNECTIVITY);
        setDockerPoweredOffSeverity(FACTORY_DOCKER_STATE_SEVERITY);
      }
      setDockerIgnoredPrefixes(config.dockerIgnoredContainerPrefixes ?? []);
      setIgnoredGuestPrefixes(config.ignoredGuestPrefixes ?? []);
      setGuestTagWhitelist(config.guestTagWhitelist ?? []);
      setGuestTagBlacklist(config.guestTagBlacklist ?? []);

      if (config.storageDefault) {
        setStorageDefault(getTriggerValue(config.storageDefault) ?? 85);
      }
      if (config.timeThresholds) {
        setTimeThresholds({
          guest: config.timeThresholds.guest ?? DEFAULT_DELAY_SECONDS,
          node: config.timeThresholds.node ?? DEFAULT_DELAY_SECONDS,
          storage: config.timeThresholds.storage ?? DEFAULT_DELAY_SECONDS,
          pbs: config.timeThresholds.pbs ?? DEFAULT_DELAY_SECONDS,
          agent: config.timeThresholds.agent ?? DEFAULT_DELAY_SECONDS,
        });
      } else {
        setTimeThresholds({
          guest: DEFAULT_DELAY_SECONDS,
          node: DEFAULT_DELAY_SECONDS,
          storage: DEFAULT_DELAY_SECONDS,
          pbs: DEFAULT_DELAY_SECONDS,
          agent: DEFAULT_DELAY_SECONDS,
        });
      }
      if (config.metricTimeThresholds) {
        setMetricTimeThresholds(normalizeMetricDelayMap(config.metricTimeThresholds));
      } else {
        setMetricTimeThresholds({});
      }

      // Load backup thresholds
      if (config.backupDefaults) {
        const enabled = Boolean(config.backupDefaults.enabled);
        const rawWarning = config.backupDefaults.warningDays ?? FACTORY_BACKUP_DEFAULTS.warningDays;
        const rawCritical =
          config.backupDefaults.criticalDays ?? FACTORY_BACKUP_DEFAULTS.criticalDays;
        const safeCritical = Math.max(0, rawCritical);
        const normalizedWarning = Math.max(0, rawWarning);
        const warningDays =
          safeCritical > 0 && normalizedWarning > safeCritical ? safeCritical : normalizedWarning;
        const criticalDays = Math.max(safeCritical, warningDays);
        const freshHours = config.backupDefaults.freshHours ?? FACTORY_BACKUP_DEFAULTS.freshHours;
        const staleHours = config.backupDefaults.staleHours ?? FACTORY_BACKUP_DEFAULTS.staleHours;
        const alertOrphaned =
          config.backupDefaults.alertOrphaned ?? FACTORY_BACKUP_DEFAULTS.alertOrphaned ?? true;
        const ignoreVMIDs = Array.from(
          new Set(
            (config.backupDefaults.ignoreVMIDs ?? FACTORY_BACKUP_DEFAULTS.ignoreVMIDs ?? [])
              .map((value) => value.trim())
              .filter((value) => value.length > 0),
          ),
        );
        setBackupDefaults({
          enabled,
          warningDays,
          criticalDays,
          freshHours,
          staleHours,
          alertOrphaned,
          ignoreVMIDs,
        });
      } else {
        setBackupDefaults({ ...FACTORY_BACKUP_DEFAULTS });
      }

      // Load snapshot thresholds
      if (config.snapshotDefaults) {
        const enabled = Boolean(config.snapshotDefaults.enabled);
        const rawWarning = config.snapshotDefaults.warningDays ?? 30;
        const rawCritical = config.snapshotDefaults.criticalDays ?? 45;
        const safeCritical = Math.max(0, rawCritical);
        const normalizedWarning = Math.max(0, rawWarning);
        const warningDays =
          safeCritical > 0 && normalizedWarning > safeCritical ? safeCritical : normalizedWarning;
        const criticalDays = Math.max(safeCritical, warningDays);
        setSnapshotDefaults({
          enabled,
          warningDays,
          criticalDays,
        });
      } else {
        setSnapshotDefaults({
          enabled: false,
          warningDays: 30,
          criticalDays: 45,
        });
      }

      // Load PMG thresholds
      if (config.pmgDefaults) {
        setPMGThresholds({
          queueTotalWarning: config.pmgDefaults.queueTotalWarning ?? 500,
          queueTotalCritical: config.pmgDefaults.queueTotalCritical ?? 1000,
          oldestMessageWarnMins: config.pmgDefaults.oldestMessageWarnMins ?? 30,
          oldestMessageCritMins: config.pmgDefaults.oldestMessageCritMins ?? 60,
          deferredQueueWarn: config.pmgDefaults.deferredQueueWarn ?? 200,
          deferredQueueCritical: config.pmgDefaults.deferredQueueCritical ?? 500,
          holdQueueWarn: config.pmgDefaults.holdQueueWarn ?? 100,
          holdQueueCritical: config.pmgDefaults.holdQueueCritical ?? 300,
          quarantineSpamWarn: config.pmgDefaults.quarantineSpamWarn ?? 2000,
          quarantineSpamCritical: config.pmgDefaults.quarantineSpamCritical ?? 5000,
          quarantineVirusWarn: config.pmgDefaults.quarantineVirusWarn ?? 2000,
          quarantineVirusCritical: config.pmgDefaults.quarantineVirusCritical ?? 5000,
          quarantineGrowthWarnPct: config.pmgDefaults.quarantineGrowthWarnPct ?? 25,
          quarantineGrowthWarnMin: config.pmgDefaults.quarantineGrowthWarnMin ?? 250,
          quarantineGrowthCritPct: config.pmgDefaults.quarantineGrowthCritPct ?? 50,
          quarantineGrowthCritMin: config.pmgDefaults.quarantineGrowthCritMin ?? 500,
        });
      }

      // Load global disable flags
      setDisableAllNodes(config.disableAllNodes ?? false);
      setDisableAllGuests(config.disableAllGuests ?? false);
      setDisableAllAgents(config.disableAllAgents ?? false);
      setDisableAllStorage(config.disableAllStorage ?? false);
      setDisableAllPBS(config.disableAllPBS ?? false);
      setDisableAllPMG(config.disableAllPMG ?? false);
      setDisableAllDockerHosts(config.disableAllDockerHosts ?? false);
      setDisableAllDockerServices(config.disableAllDockerServices ?? false);
      setDisableAllDockerContainers(config.disableAllDockerContainers ?? false);

      // Load global disable offline alerts flags
      setDisableAllNodesOffline(config.disableAllNodesOffline ?? false);
      setDisableAllGuestsOffline(config.disableAllGuestsOffline ?? false);
      setDisableAllAgentsOffline(config.disableAllAgentsOffline ?? false);
      setDisableAllPBSOffline(config.disableAllPBSOffline ?? false);
      setDisableAllPMGOffline(config.disableAllPMGOffline ?? false);
      setDisableAllDockerHostsOffline(config.disableAllDockerHostsOffline ?? false);

      // Clean up any agent disk override keys that used old underscore sanitization.
      const rawOverrides = config.overrides || {};
      const cleanedOverrides: typeof rawOverrides = {};
      for (const [key, value] of Object.entries(rawOverrides)) {
        const diskMatch = key.match(/^(agent:.+\/disk:)(.+)$/);
        if (diskMatch) {
          const normalized =
            diskMatch[2]
              .toLowerCase()
              .replace(/[^a-z0-9]/g, '-')
              .replace(/-{2,}/g, '-')
              .replace(/^-|-$/g, '') || 'unknown';
          cleanedOverrides[diskMatch[1] + normalized] = value;
        } else {
          cleanedOverrides[key] = value;
        }
      }
      setRawOverridesConfig(cleanedOverrides);

      if (config.schedule) {
        if (config.schedule.quietHours) {
          const qh = config.schedule.quietHours;
          let days: Record<string, boolean>;
          if (Array.isArray(qh.days)) {
            days = {
              sunday: qh.days.includes(0),
              monday: qh.days.includes(1),
              tuesday: qh.days.includes(2),
              wednesday: qh.days.includes(3),
              thursday: qh.days.includes(4),
              friday: qh.days.includes(5),
              saturday: qh.days.includes(6),
            };
          } else {
            days = (qh.days as Record<string, boolean>) || createDefaultQuietHours().days;
          }
          const suppress = {
            performance: qh.suppress?.performance ?? false,
            storage: qh.suppress?.storage ?? false,
            offline: qh.suppress?.offline ?? false,
          };

          setScheduleQuietHours({
            enabled: qh.enabled || false,
            start: qh.start || '22:00',
            end: qh.end || '08:00',
            timezone: qh.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
            days,
            suppress,
          });
        }

        if (config.schedule.cooldown !== undefined) {
          const rawCooldown = config.schedule.cooldown;
          const cooldownEnabled = rawCooldown > 0;
          setScheduleCooldown({
            enabled: cooldownEnabled,
            minutes: cooldownEnabled ? clampCooldownMinutes(rawCooldown) : 0,
            maxAlerts: fallbackMaxAlertsPerHour(config.schedule.maxAlertsHour),
          });
        }

        if (config.schedule.grouping) {
          const groupingConfig = config.schedule.grouping;
          const rawGroupingWindowSeconds =
            typeof groupingConfig?.window === 'number'
              ? groupingConfig.window
              : GROUPING_WINDOW_DEFAULT_SECONDS;
          const normalizedGroupingWindowSeconds = Math.max(0, rawGroupingWindowSeconds);
          const groupingWindowMinutes = Math.round(normalizedGroupingWindowSeconds / 60);

          setScheduleGrouping({
            enabled:
              groupingConfig?.enabled !== undefined
                ? Boolean(groupingConfig.enabled)
                : normalizedGroupingWindowSeconds > 0,
            window: groupingWindowMinutes,
            byNode: groupingConfig?.byNode !== undefined ? groupingConfig.byNode : true,
            byGuest: groupingConfig?.byGuest !== undefined ? groupingConfig.byGuest : false,
          });
        }

        if (config.schedule.notifyOnResolve !== undefined) {
          setNotifyOnResolve(Boolean(config.schedule.notifyOnResolve));
        } else {
          setNotifyOnResolve(createDefaultResolveNotifications());
        }

        if (config.schedule.escalation) {
          const rawLevels = config.schedule.escalation.levels || [];
          const levels = rawLevels.map((level) => ({
            after: typeof level.after === 'number' ? level.after : 15,
            notify: (level.notify as EscalationNotifyTarget) || 'all',
          }));
          setScheduleEscalation({
            enabled: Boolean(config.schedule.escalation.enabled),
            levels,
          });
        }
      }

      try {
        const emailConfigData = await NotificationsAPI.getEmailConfig();
        setEmailConfig(normalizeEmailConfigFromAPI(emailConfigData));
      } catch (emailErr) {
        logger.error('Failed to load email configuration:', emailErr);
        setDestConfigLoadError(getAlertDestinationsConfigLoadError());
      }

      try {
        const appriseData = await NotificationsAPI.getAppriseConfig();
        setAppriseConfig({
          enabled: appriseData.enabled ?? false,
          mode: appriseData.mode === 'http' ? 'http' : 'cli',
          targetsText: formatAppriseTargets(appriseData.targets),
          cliPath: appriseData.cliPath || 'apprise',
          timeoutSeconds:
            typeof appriseData.timeoutSeconds === 'number' && appriseData.timeoutSeconds > 0
              ? appriseData.timeoutSeconds
              : 15,
          serverUrl: appriseData.serverUrl || '',
          configKey: appriseData.configKey || '',
          apiKey: appriseData.apiKey || '',
          apiKeyHeader: appriseData.apiKeyHeader || 'X-API-KEY',
          skipTlsVerify: Boolean(appriseData.skipTlsVerify),
        });
      } catch (appriseErr) {
        logger.error('Failed to load Apprise configuration:', appriseErr);
        setDestConfigLoadError(getAlertDestinationsConfigLoadError());
      }

      if (options.notify) {
        notificationStore.success(getAlertConfigDiscardedSuccess());
      }
    } catch (err) {
      logger.error('Failed to load alert configuration:', err);
      // If the top-level config fetch failed, destination state may still hold
      // defaults from the reset above.  Re-flag so Save stays disabled.
      setDestConfigLoadError(getAlertDestinationsConfigLoadError());
      if (options.notify) {
        notificationStore.error(getAlertConfigReloadFailure());
      }
    } finally {
      setIsReloadingConfig(false);
    }
  };

  // Load existing alert configuration on mount and when org context changes.
  onMount(() => {
    void loadAlertConfiguration();

    const unsubscribeOrgSwitched = eventBus.on('org_switched', () => {
      void loadAlertConfiguration();
    });

    onCleanup(() => {
      unsubscribeOrgSwitched();
    });
  });

  // Reload email and apprise config when switching to destinations tab.
  // Error is only cleared after both fetches complete successfully to avoid a
  // timing window where Save is enabled while the reload is still in flight.
  // A version counter prevents stale responses from overwriting fresh state.
  let destReloadVersion = 0;
  createEffect(() => {
    if (activeTab() === 'destinations') {
      const thisVersion = ++destReloadVersion;
      setIsLoadingDestinations(true);

      const emailPromise = NotificationsAPI.getEmailConfig().then((emailConfigData) => {
        if (thisVersion === destReloadVersion) {
          setEmailConfig(normalizeEmailConfigFromAPI(emailConfigData));
        }
      });

      const apprisePromise = NotificationsAPI.getAppriseConfig().then((appriseData) => {
        if (thisVersion === destReloadVersion) {
          setAppriseConfig({
            enabled: appriseData.enabled ?? false,
            mode: appriseData.mode === 'http' ? 'http' : 'cli',
            targetsText: formatAppriseTargets(appriseData.targets),
            cliPath: appriseData.cliPath || 'apprise',
            timeoutSeconds:
              typeof appriseData.timeoutSeconds === 'number' && appriseData.timeoutSeconds > 0
                ? appriseData.timeoutSeconds
                : 15,
            serverUrl: appriseData.serverUrl || '',
            configKey: appriseData.configKey || '',
            apiKey: appriseData.apiKey || '',
            apiKeyHeader: appriseData.apiKeyHeader || 'X-API-KEY',
            skipTlsVerify: Boolean(appriseData.skipTlsVerify),
          });
        }
      });

      void Promise.allSettled([emailPromise, apprisePromise]).then((results) => {
        if (thisVersion !== destReloadVersion) return;
        const failed = results.some((r) => r.status === 'rejected');
        if (failed) {
          const reasons = results
            .filter((r): r is PromiseRejectedResult => r.status === 'rejected')
            .map((r) => r.reason);
          for (const reason of reasons) {
            logger.error('Failed to reload notification configuration:', reason);
          }
          setDestConfigLoadError(getAlertDestinationsConfigLoadError());
        } else {
          setDestConfigLoadError(null);
        }
        setIsLoadingDestinations(false);
      });
    }
  });

  // Get all guests from alert resource selectors - memoize to prevent unnecessary updates
  const allGuests = createMemo(
    () => [...byType('vm'), ...byType('system-container'), ...byType('oci-container')],
    [],
    {
      equals: (prev, next) => {
        if (prev.length !== next.length) return false;
        return prev.every((p, i) => p.id === next[i].id && p.name === next[i].name);
      },
    },
  );

  // Factory defaults - constants for reset functionality
  const FACTORY_GUEST_DEFAULTS = {
    cpu: 80,
    memory: 85,
    disk: 90,
    diskRead: -1,
    diskWrite: -1,
    networkIn: -1,
    networkOut: -1,
  };

  const FACTORY_NODE_DEFAULTS = {
    cpu: 80,
    memory: 85,
    disk: 90,
    temperature: 80,
  };
  const FACTORY_PBS_DEFAULTS = {
    cpu: 80,
    memory: 85,
  };

  const FACTORY_AGENT_DEFAULTS = {
    cpu: 80,
    memory: 85,
    disk: 90,
    diskTemperature: 55,
  };

  const FACTORY_DOCKER_DEFAULTS = {
    cpu: 80,
    memory: 85,
    disk: 85,
    restartCount: 3,
    restartWindow: 300,
    memoryWarnPct: 90,
    memoryCriticalPct: 95,
    serviceWarnGapPercent: 10,
    serviceCriticalGapPercent: 50,
  };
  const FACTORY_DOCKER_STATE_DISABLE_CONNECTIVITY = false;
  const FACTORY_DOCKER_STATE_SEVERITY: 'warning' | 'critical' = 'warning';

  const FACTORY_STORAGE_DEFAULT = 85;
  const FACTORY_SNAPSHOT_DEFAULTS: SnapshotAlertConfig = {
    enabled: false,
    warningDays: 30,
    criticalDays: 45,
  };
  const FACTORY_BACKUP_DEFAULTS: BackupAlertConfig = {
    enabled: false,
    warningDays: 7,
    criticalDays: 14,
    freshHours: 24,
    staleHours: 72,
    alertOrphaned: true,
    ignoreVMIDs: [],
  };

  // Threshold states - using trigger values for display
  const [guestDefaults, setGuestDefaults] = createSignal<Record<string, number | undefined>>({
    ...FACTORY_GUEST_DEFAULTS,
  });
  const [guestDisableConnectivity, setGuestDisableConnectivity] = createSignal(false);
  const [guestPoweredOffSeverity, setGuestPoweredOffSeverity] = createSignal<
    'warning' | 'critical'
  >('warning');

  const [nodeDefaults, setNodeDefaults] = createSignal<Record<string, number | undefined>>({
    ...FACTORY_NODE_DEFAULTS,
  });
  const [pbsDefaults, setPBSDefaults] = createSignal<Record<string, number | undefined>>({
    ...FACTORY_PBS_DEFAULTS,
  });
  const [agentDefaults, setAgentDefaults] = createSignal<Record<string, number | undefined>>({
    ...FACTORY_AGENT_DEFAULTS,
  });

  const [dockerDefaults, setDockerDefaults] = createSignal({ ...FACTORY_DOCKER_DEFAULTS });
  const [dockerDisableConnectivity, setDockerDisableConnectivity] = createSignal(
    FACTORY_DOCKER_STATE_DISABLE_CONNECTIVITY,
  );
  const [dockerPoweredOffSeverity, setDockerPoweredOffSeverity] = createSignal<
    'warning' | 'critical'
  >(FACTORY_DOCKER_STATE_SEVERITY);
  const [dockerIgnoredPrefixes, setDockerIgnoredPrefixes] = createSignal<string[]>([]);
  const [ignoredGuestPrefixes, setIgnoredGuestPrefixes] = createSignal<string[]>([]);
  const [guestTagWhitelist, setGuestTagWhitelist] = createSignal<string[]>([]);
  const [guestTagBlacklist, setGuestTagBlacklist] = createSignal<string[]>([]);

  const [storageDefault, setStorageDefault] = createSignal(FACTORY_STORAGE_DEFAULT);
  const [backupDefaults, setBackupDefaults] = createSignal<BackupAlertConfig>({
    ...FACTORY_BACKUP_DEFAULTS,
  });

  // Reset functions
  const resetGuestDefaults = () => {
    setGuestDefaults({ ...FACTORY_GUEST_DEFAULTS });
    setHasUnsavedChanges(true);
  };

  const resetNodeDefaults = () => {
    setNodeDefaults({ ...FACTORY_NODE_DEFAULTS });
    setHasUnsavedChanges(true);
  };
  const resetPBSDefaults = () => {
    setPBSDefaults({ ...FACTORY_PBS_DEFAULTS });
    setHasUnsavedChanges(true);
  };

  const resetAgentDefaults = () => {
    setAgentDefaults({ ...FACTORY_AGENT_DEFAULTS });
    setHasUnsavedChanges(true);
  };

  const resetDockerDefaults = () => {
    setDockerDefaults({ ...FACTORY_DOCKER_DEFAULTS });
    setDockerDisableConnectivity(FACTORY_DOCKER_STATE_DISABLE_CONNECTIVITY);
    setDockerPoweredOffSeverity(FACTORY_DOCKER_STATE_SEVERITY);
    setHasUnsavedChanges(true);
  };

  const resetDockerIgnoredPrefixes = () => {
    setDockerIgnoredPrefixes([]);
    setHasUnsavedChanges(true);
  };

  const resetStorageDefault = () => {
    setStorageDefault(FACTORY_STORAGE_DEFAULT);
    setHasUnsavedChanges(true);
  };
  const resetBackupDefaults = () => {
    setBackupDefaults({ ...FACTORY_BACKUP_DEFAULTS });
    setHasUnsavedChanges(true);
  };
  const resetSnapshotDefaults = () => {
    setSnapshotDefaults({ ...FACTORY_SNAPSHOT_DEFAULTS });
    setHasUnsavedChanges(true);
  };
  const [timeThresholds, setTimeThresholds] = createSignal({
    guest: DEFAULT_DELAY_SECONDS,
    node: DEFAULT_DELAY_SECONDS,
    storage: DEFAULT_DELAY_SECONDS,
    pbs: DEFAULT_DELAY_SECONDS,
    agent: DEFAULT_DELAY_SECONDS,
  });
  const [metricTimeThresholds, setMetricTimeThresholds] = createSignal<
    Record<string, Record<string, number>>
  >({});
  const [snapshotDefaults, setSnapshotDefaults] = createSignal<SnapshotAlertConfig>({
    ...FACTORY_SNAPSHOT_DEFAULTS,
  });

  const [pmgThresholds, setPMGThresholds] = createSignal({
    queueTotalWarning: 500,
    queueTotalCritical: 1000,
    oldestMessageWarnMins: 30,
    oldestMessageCritMins: 60,
    deferredQueueWarn: 200,
    deferredQueueCritical: 500,
    holdQueueWarn: 100,
    holdQueueCritical: 300,
    quarantineSpamWarn: 2000,
    quarantineSpamCritical: 5000,
    quarantineVirusWarn: 2000,
    quarantineVirusCritical: 5000,
    quarantineGrowthWarnPct: 25,
    quarantineGrowthWarnMin: 250,
    quarantineGrowthCritPct: 50,
    quarantineGrowthCritMin: 500,
  });

  // Global disable flags per resource type
  const [disableAllNodes, setDisableAllNodes] = createSignal(false);
  const [disableAllGuests, setDisableAllGuests] = createSignal(false);
  const [disableAllAgents, setDisableAllAgents] = createSignal(false);
  const [disableAllStorage, setDisableAllStorage] = createSignal(false);
  const [disableAllPBS, setDisableAllPBS] = createSignal(false);
  const [disableAllPMG, setDisableAllPMG] = createSignal(false);
  const [disableAllDockerHosts, setDisableAllDockerHosts] = createSignal(false);
  const [disableAllDockerServices, setDisableAllDockerServices] = createSignal(false);
  const [disableAllDockerContainers, setDisableAllDockerContainers] = createSignal(false);

  // Global disable offline alerts flags
  const [disableAllNodesOffline, setDisableAllNodesOffline] = createSignal(false);
  const [disableAllGuestsOffline, setDisableAllGuestsOffline] = createSignal(false);
  const [disableAllAgentsOffline, setDisableAllAgentsOffline] = createSignal(false);
  const [disableAllPBSOffline, setDisableAllPBSOffline] = createSignal(false);
  const [disableAllPMGOffline, setDisableAllPMGOffline] = createSignal(false);
  const [disableAllDockerHostsOffline, setDisableAllDockerHostsOffline] = createSignal(false);

  const tabGroups = getAlertsTabGroups().map((group) => ({
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
  })) satisfies {
    id: 'status' | 'configuration';
    label: string;
    items: { id: AlertTab; label: string; icon: JSX.Element }[];
  }[];

  const flatTabs = tabGroups.flatMap((group) => group.items);
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
          <Show when={activeTab() === 'overview'}>
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

      {/* Save notification bar - only show when there are unsaved changes */}
      <Show when={hasUnsavedChanges() && activeTab() !== 'overview' && activeTab() !== 'history'}>
        <Card tone="warning" padding="sm" class="border-yellow-200 dark:border-yellow-800 sm:p-4">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div class="flex items-center gap-2 text-yellow-800 dark:text-yellow-200">
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <circle cx="12" cy="12" r="10"></circle>
                <line x1="12" y1="8" x2="12" y2="12"></line>
                <line x1="12" y1="16" x2="12.01" y2="16"></line>
              </svg>
              <span class="text-sm font-medium">{getAlertConfigUnsavedChangesLabel()}</span>
            </div>
            <div class="flex w-full gap-2 sm:w-auto">
              <button
                class="flex-1 px-4 py-2 text-sm text-white transition-colors sm:flex-initial bg-blue-600 rounded-md hover:bg-blue-700 disabled:opacity-60 disabled:cursor-not-allowed"
                disabled={isReloadingConfig() || !!destConfigLoadError()}
                onClick={async () => {
                  try {
                    // Save alert configuration with hysteresis format
                    const createHysteresisThreshold = (
                      trigger: number | undefined,
                      clearMargin: number = 5,
                    ) => {
                      const normalized = typeof trigger === 'number' ? trigger : 0;
                      return {
                        trigger: normalized,
                        clear: Math.max(0, normalized - clearMargin),
                      };
                    };

                    const snapshotConfig = snapshotDefaults();
                    const normalizedSnapshotWarning = Math.max(0, snapshotConfig.warningDays ?? 0);
                    const normalizedSnapshotCritical = Math.max(
                      0,
                      snapshotConfig.criticalDays ?? 0,
                    );
                    const finalSnapshotCritical =
                      normalizedSnapshotCritical > 0
                        ? Math.max(normalizedSnapshotCritical, normalizedSnapshotWarning)
                        : normalizedSnapshotWarning;

                    const backupConfig = backupDefaults();
                    const normalizedBackupWarning = Math.max(0, backupConfig.warningDays ?? 0);
                    const normalizedBackupCritical = Math.max(0, backupConfig.criticalDays ?? 0);
                    const finalBackupCritical =
                      normalizedBackupCritical > 0
                        ? Math.max(normalizedBackupCritical, normalizedBackupWarning)
                        : normalizedBackupWarning;

                    const dockerDefaultsValue = dockerDefaults();
                    if (
                      dockerDefaultsValue.serviceCriticalGapPercent > 0 &&
                      dockerDefaultsValue.serviceWarnGapPercent >
                        dockerDefaultsValue.serviceCriticalGapPercent
                    ) {
                      notificationStore.error(
                        'Swarm service critical gap must be greater than or equal to the warning gap when enabled.',
                      );
                      return;
                    }

                    const normalizedCooldownMinutes = scheduleCooldown().enabled
                      ? clampCooldownMinutes(scheduleCooldown().minutes)
                      : 0;
                    const normalizedMaxAlertsHour = clampMaxAlertsPerHour(
                      scheduleCooldown().maxAlerts,
                    );
                    const groupingState = scheduleGrouping();
                    const groupingWindowSeconds =
                      groupingState.enabled && groupingState.window >= 0
                        ? groupingState.window * 60
                        : 0;
                    const groupingEnabled = groupingState.enabled && groupingWindowSeconds > 0;

                    const existingActivationState = alertsActivation.activationState();
                    const existingActivationTime = alertsActivation.config()?.activationTime;
                    const existingObservationWindowHours =
                      alertsActivation.config()?.observationWindowHours;

                    const alertConfig = {
                      enabled: alertsActivation.config()?.enabled ?? true,
                      activationState: existingActivationState ?? undefined,
                      activationTime: existingActivationTime,
                      observationWindowHours: existingObservationWindowHours,
                      // Global disable flags per resource type
                      disableAllNodes: disableAllNodes(),
                      disableAllGuests: disableAllGuests(),
                      disableAllAgents: disableAllAgents(),
                      disableAllStorage: disableAllStorage(),
                      disableAllPBS: disableAllPBS(),
                      disableAllPMG: disableAllPMG(),
                      disableAllDockerHosts: disableAllDockerHosts(),
                      disableAllDockerContainers: disableAllDockerContainers(),
                      disableAllDockerServices: disableAllDockerServices(),
                      // Global disable offline alerts flags
                      disableAllNodesOffline: disableAllNodesOffline(),
                      disableAllGuestsOffline: disableAllGuestsOffline(),
                      disableAllPBSOffline: disableAllPBSOffline(),
                      disableAllAgentsOffline: disableAllAgentsOffline(),
                      disableAllPMGOffline: disableAllPMGOffline(),
                      disableAllDockerHostsOffline: disableAllDockerHostsOffline(),
                      guestDefaults: {
                        cpu: createHysteresisThreshold(guestDefaults().cpu),
                        memory: createHysteresisThreshold(guestDefaults().memory),
                        disk: createHysteresisThreshold(guestDefaults().disk),
                        diskRead: createHysteresisThreshold(guestDefaults().diskRead),
                        diskWrite: createHysteresisThreshold(guestDefaults().diskWrite),
                        networkIn: createHysteresisThreshold(guestDefaults().networkIn),
                        networkOut: createHysteresisThreshold(guestDefaults().networkOut),
                        disableConnectivity: guestDisableConnectivity(),
                        poweredOffSeverity: guestPoweredOffSeverity(),
                      },
                      nodeDefaults: {
                        cpu: createHysteresisThreshold(nodeDefaults().cpu),
                        memory: createHysteresisThreshold(nodeDefaults().memory),
                        disk: createHysteresisThreshold(nodeDefaults().disk),
                        temperature: createHysteresisThreshold(nodeDefaults().temperature),
                      },
                      agentDefaults: {
                        cpu: createHysteresisThreshold(agentDefaults().cpu),
                        memory: createHysteresisThreshold(agentDefaults().memory),
                        disk: createHysteresisThreshold(agentDefaults().disk),
                        diskTemperature: createHysteresisThreshold(agentDefaults().diskTemperature),
                      },
                      pbsDefaults: {
                        cpu: createHysteresisThreshold(pbsDefaults().cpu),
                        memory: createHysteresisThreshold(pbsDefaults().memory),
                      },
                      dockerDefaults: {
                        cpu: createHysteresisThreshold(dockerDefaultsValue.cpu),
                        memory: createHysteresisThreshold(dockerDefaultsValue.memory),
                        disk: createHysteresisThreshold(dockerDefaultsValue.disk),
                        restartCount: dockerDefaultsValue.restartCount,
                        restartWindow: dockerDefaultsValue.restartWindow,
                        memoryWarnPct: dockerDefaultsValue.memoryWarnPct,
                        memoryCriticalPct: dockerDefaultsValue.memoryCriticalPct,
                        serviceWarnGapPercent: dockerDefaultsValue.serviceWarnGapPercent,
                        serviceCriticalGapPercent: dockerDefaultsValue.serviceCriticalGapPercent,
                        stateDisableConnectivity: dockerDisableConnectivity(),
                        statePoweredOffSeverity: dockerPoweredOffSeverity(),
                      },
                      dockerIgnoredContainerPrefixes: dockerIgnoredPrefixes()
                        .map((prefix) => prefix.trim())
                        .filter((prefix) => prefix.length > 0),
                      ignoredGuestPrefixes: ignoredGuestPrefixes()
                        .map((prefix) => prefix.trim())
                        .filter((prefix) => prefix.length > 0),
                      guestTagWhitelist: guestTagWhitelist()
                        .map((tag) => tag.trim())
                        .filter((tag) => tag.length > 0),
                      guestTagBlacklist: guestTagBlacklist()
                        .map((tag) => tag.trim())
                        .filter((tag) => tag.length > 0),
                      storageDefault: createHysteresisThreshold(storageDefault()),
                      minimumDelta: 2.0,
                      suppressionWindow: 5,
                      hysteresisMargin: 5.0,
                      timeThresholds: timeThresholds(),
                      metricTimeThresholds: normalizeMetricDelayMap(metricTimeThresholds()),
                      snapshotDefaults: {
                        enabled: snapshotConfig.enabled,
                        warningDays: normalizedSnapshotWarning,
                        criticalDays: finalSnapshotCritical,
                      },
                      backupDefaults: {
                        enabled: backupConfig.enabled,
                        warningDays: normalizedBackupWarning,
                        criticalDays: finalBackupCritical,
                        freshHours: backupConfig.freshHours ?? 24,
                        staleHours: backupConfig.staleHours ?? 72,
                        alertOrphaned: backupConfig.alertOrphaned ?? true,
                        ignoreVMIDs: (backupConfig.ignoreVMIDs ?? [])
                          .map((value) => value.trim())
                          .filter((value) => value.length > 0),
                      },
                      pmgDefaults: pmgThresholds(),
                      // Use rawOverridesConfig which is already properly formatted with disabled flags
                      overrides: rawOverridesConfig(),
                      schedule: {
                        quietHours: scheduleQuietHours(),
                        cooldown: normalizedCooldownMinutes,
                        notifyOnResolve: notifyOnResolve(),
                        maxAlertsHour: normalizedMaxAlertsHour,
                        escalation: scheduleEscalation(),
                        grouping: {
                          enabled: groupingEnabled,
                          window: groupingWindowSeconds, // Convert minutes to seconds
                          byNode: groupingState.byNode,
                          byGuest: groupingState.byGuest,
                        },
                      },
                      // Add missing required fields
                      aggregation: {
                        enabled: true,
                        timeWindow: 10,
                        countThreshold: 3,
                        similarityWindow: 5.0,
                      },
                      flapping: {
                        enabled: true,
                        threshold: 5,
                        window: 10,
                        suppressionTime: 30,
                        minStability: 0.8,
                      },
                      ioNormalization: {
                        enabled: true,
                        vmDiskMax: 500.0,
                        containerDiskMax: 300.0,
                        networkMax: 1000.0,
                      },
                    };

                    await AlertsAPI.updateConfig(alertConfig);

                    // Save email config if it exists (regardless of active tab)
                    if (destinationsRef.emailConfig) {
                      const emailData = destinationsRef.emailConfig();
                      await NotificationsAPI.updateEmailConfig(emailData);
                    }

                    if (destinationsRef.appriseConfig) {
                      const appriseData = destinationsRef.appriseConfig();
                      const updatedApprise =
                        await NotificationsAPI.updateAppriseConfig(appriseData);
                      setAppriseConfig({
                        enabled: updatedApprise.enabled ?? false,
                        mode: updatedApprise.mode === 'http' ? 'http' : 'cli',
                        targetsText: formatAppriseTargets(updatedApprise.targets),
                        cliPath: updatedApprise.cliPath || 'apprise',
                        timeoutSeconds:
                          typeof updatedApprise.timeoutSeconds === 'number' &&
                          updatedApprise.timeoutSeconds > 0
                            ? updatedApprise.timeoutSeconds
                            : 15,
                        serverUrl: updatedApprise.serverUrl || '',
                        configKey: updatedApprise.configKey || '',
                        apiKey: updatedApprise.apiKey || '',
                        apiKeyHeader: updatedApprise.apiKeyHeader || 'X-API-KEY',
                        skipTlsVerify: Boolean(updatedApprise.skipTlsVerify),
                      });
                    }

                    setHasUnsavedChanges(false);
                    notificationStore.success(getAlertConfigSaveSuccess());
                  } catch (err) {
                    logger.error('Failed to save configuration:', err);
                    notificationStore.error(
                      err instanceof Error ? err.message : getAlertConfigSaveFailure(),
                    );
                  }
                }}
              >
                {getAlertConfigSaveChangesLabel()}
              </button>
              <button
                class="flex-1 px-4 py-2 text-sm transition-colors border border-border rounded-md text-base-content hover:bg-surface-hover sm:flex-initial disabled:opacity-60 disabled:cursor-not-allowed"
                disabled={isReloadingConfig()}
                onClick={async () => {
                  await loadAlertConfiguration({ notify: true });
                }}
              >
                {getAlertConfigDiscardLabel(isReloadingConfig())}
              </button>
            </div>
          </div>
        </Card>
      </Show>

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
                <For each={tabGroups}>
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
                              aria-disabled={areAlertsDisabled()}
                              disabled={areAlertsDisabled()}
                              class={getAlertsSidebarTabClass({
                                isActive: activeTab() === item.id,
                                isDisabled: areAlertsDisabled(),
                                collapsed: sidebarCollapsed(),
                              })}
                              onClick={() => handleTabChange(item.id)}
                              title={getAlertsTabTitle({
                                isDisabled: areAlertsDisabled(),
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
            <Show when={flatTabs.length > 0}>
              <div class="lg:hidden border-b border-border">
                <div class="p-1">
                  <div
                    class="flex rounded-md bg-surface-hover p-0.5 w-full overflow-x-auto"
                    style="-webkit-overflow-scrolling: touch;"
                  >
                    <For each={flatTabs}>
                      {(tab) => (
                        <button
                          type="button"
                          aria-disabled={areAlertsDisabled()}
                          disabled={areAlertsDisabled()}
                          class={getAlertsMobileTabClass({
                            isActive: activeTab() === tab.id,
                            isDisabled: areAlertsDisabled(),
                          })}
                          onClick={() => handleTabChange(tab.id)}
                          title={getAlertsTabTitle({
                            isDisabled: areAlertsDisabled(),
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
                  overrides={overrides()}
                  activeAlerts={activeAlerts}
                  updateAlert={updateAlert}
                  showQuickTip={showQuickTip}
                  dismissQuickTip={dismissQuickTip}
                  showAcknowledged={showAcknowledged}
                  setShowAcknowledged={setShowAcknowledged}
                  alertsDisabled={areAlertsDisabled}
                  hasAIAlertsFeature={hasAIAlertsFeature}
                  licenseLoading={licenseLoading}
                />
              </Show>

              <Show when={activeTab() === 'thresholds'}>
                <ThresholdsTab
                  overrides={overrides}
                  setOverrides={setOverrides}
                  rawOverridesConfig={rawOverridesConfig}
                  setRawOverridesConfig={setRawOverridesConfig}
                  allGuests={allGuests}
                  pbsInstances={pbsInstances()}
                  pmgInstances={pmgInstances()}
                  nodes={byType('agent')}
                  agents={agentResources()}
                  storage={allResources().filter(
                    (r) => r.type === 'storage' || r.type === 'datastore',
                  )}
                  dockerHosts={byType('docker-host')}
                  allResources={allResources()}
                  guestDefaults={guestDefaults}
                  guestDisableConnectivity={guestDisableConnectivity}
                  setGuestDefaults={setGuestDefaults}
                  setGuestDisableConnectivity={setGuestDisableConnectivity}
                  guestPoweredOffSeverity={guestPoweredOffSeverity}
                  setGuestPoweredOffSeverity={setGuestPoweredOffSeverity}
                  nodeDefaults={nodeDefaults}
                  setNodeDefaults={setNodeDefaults}
                  agentDefaults={agentDefaults}
                  setAgentDefaults={setAgentDefaults}
                  pbsDefaults={pbsDefaults}
                  setPBSDefaults={setPBSDefaults}
                  dockerDefaults={dockerDefaults}
                  dockerDisableConnectivity={dockerDisableConnectivity}
                  setDockerDisableConnectivity={setDockerDisableConnectivity}
                  dockerPoweredOffSeverity={dockerPoweredOffSeverity}
                  setDockerPoweredOffSeverity={setDockerPoweredOffSeverity}
                  setDockerDefaults={setDockerDefaults}
                  dockerIgnoredPrefixes={dockerIgnoredPrefixes}
                  setDockerIgnoredPrefixes={setDockerIgnoredPrefixes}
                  ignoredGuestPrefixes={ignoredGuestPrefixes}
                  setIgnoredGuestPrefixes={setIgnoredGuestPrefixes}
                  guestTagWhitelist={guestTagWhitelist}
                  setGuestTagWhitelist={setGuestTagWhitelist}
                  guestTagBlacklist={guestTagBlacklist}
                  setGuestTagBlacklist={setGuestTagBlacklist}
                  storageDefault={storageDefault}
                  setStorageDefault={setStorageDefault}
                  resetGuestDefaults={resetGuestDefaults}
                  resetNodeDefaults={resetNodeDefaults}
                  resetAgentDefaults={resetAgentDefaults}
                  timeThresholds={timeThresholds}
                  metricTimeThresholds={metricTimeThresholds}
                  setMetricTimeThresholds={setMetricTimeThresholds}
                  backupDefaults={backupDefaults}
                  setBackupDefaults={setBackupDefaults}
                  snapshotDefaults={snapshotDefaults}
                  setSnapshotDefaults={setSnapshotDefaults}
                  pmgThresholds={pmgThresholds}
                  setPMGThresholds={setPMGThresholds}
                  activeAlerts={activeAlerts}
                  setHasUnsavedChanges={setHasUnsavedChanges}
                  hasUnsavedChanges={hasUnsavedChanges}
                  removeAlerts={removeAlerts}
                  disableAllNodes={disableAllNodes}
                  setDisableAllNodes={setDisableAllNodes}
                  disableAllGuests={disableAllGuests}
                  setDisableAllGuests={setDisableAllGuests}
                  disableAllAgents={disableAllAgents}
                  setDisableAllAgents={setDisableAllAgents}
                  disableAllStorage={disableAllStorage}
                  setDisableAllStorage={setDisableAllStorage}
                  disableAllPBS={disableAllPBS}
                  setDisableAllPBS={setDisableAllPBS}
                  disableAllPMG={disableAllPMG}
                  setDisableAllPMG={setDisableAllPMG}
                  disableAllDockerHosts={disableAllDockerHosts}
                  setDisableAllDockerHosts={setDisableAllDockerHosts}
                  disableAllDockerServices={disableAllDockerServices}
                  setDisableAllDockerServices={setDisableAllDockerServices}
                  disableAllDockerContainers={disableAllDockerContainers}
                  setDisableAllDockerContainers={setDisableAllDockerContainers}
                  disableAllNodesOffline={disableAllNodesOffline}
                  setDisableAllNodesOffline={setDisableAllNodesOffline}
                  disableAllGuestsOffline={disableAllGuestsOffline}
                  setDisableAllGuestsOffline={setDisableAllGuestsOffline}
                  disableAllAgentsOffline={disableAllAgentsOffline}
                  setDisableAllAgentsOffline={setDisableAllAgentsOffline}
                  disableAllPBSOffline={disableAllPBSOffline}
                  setDisableAllPBSOffline={setDisableAllPBSOffline}
                  disableAllPMGOffline={disableAllPMGOffline}
                  setDisableAllPMGOffline={setDisableAllPMGOffline}
                  disableAllDockerHostsOffline={disableAllDockerHostsOffline}
                  setDisableAllDockerHostsOffline={setDisableAllDockerHostsOffline}
                  resetPBSDefaults={resetPBSDefaults}
                  resetDockerDefaults={resetDockerDefaults}
                  resetDockerIgnoredPrefixes={resetDockerIgnoredPrefixes}
                  resetStorageDefault={resetStorageDefault}
                  resetSnapshotDefaults={resetSnapshotDefaults}
                  resetBackupDefaults={resetBackupDefaults}
                  factoryGuestDefaults={FACTORY_GUEST_DEFAULTS}
                  factoryNodeDefaults={FACTORY_NODE_DEFAULTS}
                  factoryPBSDefaults={FACTORY_PBS_DEFAULTS}
                  factoryAgentDefaults={FACTORY_AGENT_DEFAULTS}
                  factoryDockerDefaults={FACTORY_DOCKER_DEFAULTS}
                  factoryStorageDefault={FACTORY_STORAGE_DEFAULT}
                  snapshotFactoryDefaults={FACTORY_SNAPSHOT_DEFAULTS}
                  backupFactoryDefaults={FACTORY_BACKUP_DEFAULTS}
                />
              </Show>

              <Show when={activeTab() === 'destinations'}>
                <DestinationsTab
                  setHasUnsavedChanges={setHasUnsavedChanges}
                  emailConfig={emailConfig}
                  setEmailConfig={setEmailConfig}
                  appriseConfig={appriseConfig}
                  setAppriseConfig={setAppriseConfig}
                  configLoadError={destConfigLoadError}
                  isRetrying={isReloadingConfig}
                  isLoadingDestinations={isLoadingDestinations}
                  onRetryLoad={() => void loadAlertConfiguration()}
                />
              </Show>

              <Show when={activeTab() === 'schedule'}>
                <ScheduleTab
                  setHasUnsavedChanges={setHasUnsavedChanges}
                  quietHours={scheduleQuietHours}
                  setQuietHours={setScheduleQuietHours}
                  cooldown={scheduleCooldown}
                  setCooldown={setScheduleCooldown}
                  grouping={scheduleGrouping}
                  setGrouping={setScheduleGrouping}
                  notifyOnResolve={notifyOnResolve}
                  setNotifyOnResolve={setNotifyOnResolve}
                  escalation={scheduleEscalation}
                  setEscalation={setScheduleEscalation}
                />
              </Show>

              <Show when={activeTab() === 'history'}>
                <HistoryTab
                  hasAIAlertsFeature={hasAIAlertsFeature}
                  licenseLoading={licenseLoading}
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
