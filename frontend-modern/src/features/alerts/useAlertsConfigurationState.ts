import { createEffect, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import type { Accessor } from 'solid-js';

import { AlertsAPI } from '@/api/alerts';
import { NotificationsAPI } from '@/api/notifications';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import type { Alert, PBSInstance, PMGInstance } from '@/types/api';
import type { AppriseConfig, EmailConfig } from '@/api/notifications';
import type {
  ActivationState,
  BackupAlertConfig,
  RawOverrideConfig,
  SnapshotAlertConfig,
} from '@/types/alerts';
import type { Resource, ResourceType } from '@/types/resource';
import {
  getAlertConfigDiscardedSuccess,
  getAlertConfigReloadFailure,
  getAlertConfigSaveSuccess,
} from '@/utils/alertConfigPresentation';
import { getAlertDestinationsConfigLoadError } from '@/utils/alertDestinationsPresentation';
import { getActionableAgentIdFromResource, hasAgentFacet } from '@/utils/agentResources';
import { isAppContainerDiscoveryResourceType } from '@/utils/discoveryTarget';
import { logger } from '@/utils/logger';
import { pbsInstanceFromResource, pmgInstanceFromResource } from '@/utils/resourceStateAdapters';

import {
  clampMaxAlertsPerHour,
  createDefaultAppriseConfig,
  createDefaultCooldown,
  createDefaultEmailConfig,
  createDefaultEscalation,
  createDefaultGrouping,
  createDefaultQuietHours,
  createDefaultResolveNotifications,
  DEFAULT_DELAY_SECONDS,
  extractTriggerValues,
  fallbackMaxAlertsPerHour,
  formatAppriseTargets,
  getAlertResourceDisplayLabel,
  getTriggerValue,
  guessNumericId,
  normalizeEmailConfigFromAPI,
  normalizeMetricDelayMap,
  parseAppriseTargets,
  platformData,
} from './helpers';
import type {
  AlertTab,
  CooldownConfig,
  EscalationConfig,
  EscalationNotifyTarget,
  GroupingConfig,
  Override,
  QuietHoursConfig,
  UIAppriseConfig,
  UIEmailConfig,
} from './types';
import { GROUPING_WINDOW_DEFAULT_SECONDS, clampCooldownMinutes } from './types';

export interface AlertsConfigurationSurfaceProps {
  activeTab: Accessor<AlertTab>;
  allResources: Accessor<Resource[]>;
  byType: (resourceType: ResourceType) => Resource[];
  children: (resourceId: string) => Resource[];
  activeAlerts: Record<string, Alert>;
  removeAlerts: (predicate: (alert: Alert) => boolean) => void;
  setOverviewOverrides: (value: Override[]) => void;
  hasUnsavedChanges: Accessor<boolean>;
  setHasUnsavedChanges: (value: boolean) => void;
  alertsActivationState: () => ActivationState | null;
  alertsActivationConfig: () => {
    enabled?: boolean;
    activationTime?: string | null;
    observationWindowHours?: number | null;
  } | null;
}

export const FACTORY_GUEST_DEFAULTS = {
  cpu: 80,
  memory: 85,
  disk: 90,
  diskRead: -1,
  diskWrite: -1,
  networkIn: -1,
  networkOut: -1,
};

export const FACTORY_NODE_DEFAULTS = {
  cpu: 80,
  memory: 85,
  disk: 90,
  temperature: 80,
};

export const FACTORY_PBS_DEFAULTS = {
  cpu: 80,
  memory: 85,
};

export const FACTORY_AGENT_DEFAULTS = {
  cpu: 80,
  memory: 85,
  disk: 90,
  diskTemperature: 55,
};

export const FACTORY_DOCKER_DEFAULTS = {
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

export const FACTORY_DOCKER_STATE_DISABLE_CONNECTIVITY = false;
export const FACTORY_DOCKER_STATE_SEVERITY: 'warning' | 'critical' = 'warning';
export const FACTORY_STORAGE_DEFAULT = 85;
export const FACTORY_SNAPSHOT_DEFAULTS: SnapshotAlertConfig = {
  enabled: false,
  warningDays: 30,
  criticalDays: 45,
};
export const FACTORY_BACKUP_DEFAULTS: BackupAlertConfig = {
  enabled: false,
  warningDays: 7,
  criticalDays: 14,
  freshHours: 24,
  staleHours: 72,
  alertOrphaned: true,
  ignoreVMIDs: [],
};

export function useAlertsConfigurationState(props: AlertsConfigurationSurfaceProps) {
  const [isReloadingConfig, setIsReloadingConfig] = createSignal(false);
  const [isLoadingDestinations, setIsLoadingDestinations] = createSignal(false);
  const [destConfigLoadError, setDestConfigLoadError] = createSignal<string | null>(null);
  const [overrides, setOverrides] = createSignal<Override[]>([]);
  const [rawOverridesConfig, setRawOverridesConfig] = createSignal<
    Record<string, RawOverrideConfig>
  >({});
  const [emailConfig, setEmailConfig] = createSignal<UIEmailConfig>(createDefaultEmailConfig());
  const [appriseConfig, setAppriseConfig] = createSignal<UIAppriseConfig>(
    createDefaultAppriseConfig(),
  );
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
  const [disableAllNodes, setDisableAllNodes] = createSignal(false);
  const [disableAllGuests, setDisableAllGuests] = createSignal(false);
  const [disableAllAgents, setDisableAllAgents] = createSignal(false);
  const [disableAllStorage, setDisableAllStorage] = createSignal(false);
  const [disableAllPBS, setDisableAllPBS] = createSignal(false);
  const [disableAllPMG, setDisableAllPMG] = createSignal(false);
  const [disableAllDockerHosts, setDisableAllDockerHosts] = createSignal(false);
  const [disableAllDockerServices, setDisableAllDockerServices] = createSignal(false);
  const [disableAllDockerContainers, setDisableAllDockerContainers] = createSignal(false);
  const [disableAllNodesOffline, setDisableAllNodesOffline] = createSignal(false);
  const [disableAllGuestsOffline, setDisableAllGuestsOffline] = createSignal(false);
  const [disableAllAgentsOffline, setDisableAllAgentsOffline] = createSignal(false);
  const [disableAllPBSOffline, setDisableAllPBSOffline] = createSignal(false);
  const [disableAllPMGOffline, setDisableAllPMGOffline] = createSignal(false);
  const [disableAllDockerHostsOffline, setDisableAllDockerHostsOffline] = createSignal(false);

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

  const allGuests = createMemo(
    () => [
      ...props.byType('vm'),
      ...props.byType('system-container'),
      ...props.byType('oci-container'),
    ],
    [],
    {
      equals: (prev, next) => {
        if (prev.length !== next.length) return false;
        return prev.every(
          (current, index) => current.id === next[index].id && current.name === next[index].name,
        );
      },
    },
  );

  const agentResources = createMemo(() =>
    props.allResources().filter(
      (resource) =>
        (resource.type === 'agent' ||
          resource.type === 'pbs' ||
          resource.type === 'pmg' ||
          resource.type === 'truenas') &&
        hasAgentFacet(resource),
    ),
  );
  const pbsInstances = createMemo<PBSInstance[]>(() =>
    props
      .allResources()
      .filter((resource) => resource.type === 'pbs')
      .map(pbsInstanceFromResource)
      .filter((resource): resource is PBSInstance => Boolean(resource)),
  );
  const pbsInstanceById = createMemo(
    () => new Map(pbsInstances().map((instance) => [instance.id, instance])),
  );
  const pmgInstances = createMemo<PMGInstance[]>(() =>
    props
      .allResources()
      .filter((resource) => resource.type === 'pmg')
      .map(pmgInstanceFromResource)
      .filter((resource): resource is PMGInstance => Boolean(resource)),
  );

  const resetGuestDefaults = () => {
    setGuestDefaults({ ...FACTORY_GUEST_DEFAULTS });
    props.setHasUnsavedChanges(true);
  };
  const resetNodeDefaults = () => {
    setNodeDefaults({ ...FACTORY_NODE_DEFAULTS });
    props.setHasUnsavedChanges(true);
  };
  const resetPBSDefaults = () => {
    setPBSDefaults({ ...FACTORY_PBS_DEFAULTS });
    props.setHasUnsavedChanges(true);
  };
  const resetAgentDefaults = () => {
    setAgentDefaults({ ...FACTORY_AGENT_DEFAULTS });
    props.setHasUnsavedChanges(true);
  };
  const resetDockerDefaults = () => {
    setDockerDefaults({ ...FACTORY_DOCKER_DEFAULTS });
    setDockerDisableConnectivity(FACTORY_DOCKER_STATE_DISABLE_CONNECTIVITY);
    setDockerPoweredOffSeverity(FACTORY_DOCKER_STATE_SEVERITY);
    props.setHasUnsavedChanges(true);
  };
  const resetDockerIgnoredPrefixes = () => {
    setDockerIgnoredPrefixes([]);
    props.setHasUnsavedChanges(true);
  };
  const resetStorageDefault = () => {
    setStorageDefault(FACTORY_STORAGE_DEFAULT);
    props.setHasUnsavedChanges(true);
  };
  const resetBackupDefaults = () => {
    setBackupDefaults({ ...FACTORY_BACKUP_DEFAULTS });
    props.setHasUnsavedChanges(true);
  };
  const resetSnapshotDefaults = () => {
    setSnapshotDefaults({ ...FACTORY_SNAPSHOT_DEFAULTS });
    props.setHasUnsavedChanges(true);
  };

  createEffect(() => {
    if (props.hasUnsavedChanges()) {
      return;
    }

    const rawConfig = rawOverridesConfig();
    if (Object.keys(rawConfig).length === 0 || props.byType('agent').length === 0) {
      return;
    }

    const nodeResources = props.byType('agent');
    const vmResources = props.byType('vm');
    const containerResources = [
      ...props.byType('system-container'),
      ...props.byType('oci-container'),
    ];
    const storageResources = props
      .allResources()
      .filter((resource) => resource.type === 'storage' || resource.type === 'datastore');
    const agentResourceList = agentResources();
    const dockerHostResources = props.byType('docker-host');
    const overridesList: Override[] = [];
    const dockerHostMap = new Map<string, Resource>();
    const dockerContainerMap = new Map<
      string,
      { host: Resource; container: Resource; containerShortId: string }
    >();
    const agentMap = new Map<string, Resource>();

    const storageCoords = (resource: Resource): { node: string; instance: string } => {
      const data = pd(resource);
      if (resource.type === 'datastore') {
        const instance =
          (data?.pbsInstanceId as string | undefined) ||
          resource.parentId ||
          resource.platformId ||
          'pbs';
        const node = (data?.pbsInstanceName as string | undefined) || instance;
        return { node, instance };
      }
      return {
        node: (data?.node as string | undefined) || '',
        instance: (data?.instance as string | undefined) || resource.platformId || '',
      };
    };

    dockerHostResources.forEach((host) => {
      dockerHostOverrideIdCandidates(host).forEach((id) => {
        dockerHostMap.set(id, host);
      });
      const containers = props.children(host.id).filter((resource) => resource.type === 'app-container');
      containers.forEach((container) => {
        const shortId = container.id.includes('/') ? container.id.split('/').pop()! : container.id;
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

      const diskMatch = key.match(/^agent:(.+)\/disk:(.+)$/);
      if (diskMatch) {
        const [, agentId, diskLabel] = diskMatch;
        const agent = agentMap.get(agentId);
        overridesList.push({
          id: key,
          name: diskLabel.replace(/-/g, '/'),
          type: 'agentDisk',
          resourceType: 'Agent Disk',
          node: agent ? getAlertResourceDisplayLabel(agent) : agentId,
          disabled: thresholds.disabled || false,
          thresholds: extractTriggerValues(thresholds),
        });
        return;
      }

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
        return;
      }

      const node = nodeResources.find((resource) => resource.id === key);
      if (node) {
        overridesList.push({
          id: key,
          name: getAlertResourceDisplayLabel(node),
          type: 'agent',
          resourceType: 'Agent',
          disableConnectivity: thresholds.disableConnectivity || false,
          thresholds: extractTriggerValues(thresholds),
        });
        return;
      }

      const storage = storageResources.find((resource) => resource.id === key);
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
        return;
      }

      const guest =
        vmResources.find((resource) => resource.id === key) ||
        containerResources.find((resource) => resource.id === key);
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
    });

    const currentOverrides = overrides();
    const hasChanged =
      overridesList.length !== currentOverrides.length ||
      overridesList.some((newOverride) => {
        const existing = currentOverrides.find((override) => override.id === newOverride.id);
        if (!existing) return true;
        return (
          JSON.stringify(newOverride.thresholds) !== JSON.stringify(existing.thresholds) ||
          Boolean(newOverride.disableConnectivity) !== Boolean(existing.disableConnectivity) ||
          Boolean(newOverride.disabled) !== Boolean(existing.disabled) ||
          (newOverride.poweredOffSeverity ?? null) !== (existing.poweredOffSeverity ?? null) ||
          JSON.stringify(newOverride.backup ?? null) !== JSON.stringify(existing.backup ?? null) ||
          JSON.stringify(newOverride.snapshot ?? null) !==
            JSON.stringify(existing.snapshot ?? null)
        );
      });

    if (hasChanged) {
      setOverrides(overridesList);
    }
  });

  createEffect(() => {
    props.setOverviewOverrides(overrides());
  });

  const loadAlertConfiguration = async (options: { notify?: boolean } = {}) => {
    setIsReloadingConfig(true);
    props.setHasUnsavedChanges(false);
    setDestConfigLoadError(null);

    setGuestDefaults({ ...FACTORY_GUEST_DEFAULTS });
    setGuestDisableConnectivity(false);
    setGuestPoweredOffSeverity('warning');
    setNodeDefaults({ ...FACTORY_NODE_DEFAULTS });
    setPBSDefaults({ ...FACTORY_PBS_DEFAULTS });
    setAgentDefaults({ ...FACTORY_AGENT_DEFAULTS });
    setDockerDefaults({ ...FACTORY_DOCKER_DEFAULTS });
    setDockerDisableConnectivity(FACTORY_DOCKER_STATE_DISABLE_CONNECTIVITY);
    setDockerPoweredOffSeverity(FACTORY_DOCKER_STATE_SEVERITY);
    setDockerIgnoredPrefixes([]);
    setIgnoredGuestPrefixes([]);
    setGuestTagWhitelist([]);
    setGuestTagBlacklist([]);
    setStorageDefault(FACTORY_STORAGE_DEFAULT);
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
    setNotifyOnResolve(createDefaultResolveNotifications());
    setEmailConfig(createDefaultEmailConfig());
    setAppriseConfig(createDefaultAppriseConfig());
    setBackupDefaults({ ...FACTORY_BACKUP_DEFAULTS });
    setSnapshotDefaults({ ...FACTORY_SNAPSHOT_DEFAULTS });

    try {
      const config = await AlertsAPI.getConfig();

      if (config.guestDefaults) {
        setGuestDefaults({
          cpu: getTriggerValue(config.guestDefaults.cpu) ?? FACTORY_GUEST_DEFAULTS.cpu,
          memory: getTriggerValue(config.guestDefaults.memory) ?? FACTORY_GUEST_DEFAULTS.memory,
          disk: getTriggerValue(config.guestDefaults.disk) ?? FACTORY_GUEST_DEFAULTS.disk,
          diskRead:
            getTriggerValue(config.guestDefaults.diskRead) ?? FACTORY_GUEST_DEFAULTS.diskRead,
          diskWrite:
            getTriggerValue(config.guestDefaults.diskWrite) ?? FACTORY_GUEST_DEFAULTS.diskWrite,
          networkIn:
            getTriggerValue(config.guestDefaults.networkIn) ?? FACTORY_GUEST_DEFAULTS.networkIn,
          networkOut:
            getTriggerValue(config.guestDefaults.networkOut) ?? FACTORY_GUEST_DEFAULTS.networkOut,
        });
        setGuestDisableConnectivity(Boolean(config.guestDefaults.disableConnectivity));
        setGuestPoweredOffSeverity(
          config.guestDefaults.poweredOffSeverity === 'critical' ? 'critical' : 'warning',
        );
      }

      if (config.nodeDefaults) {
        setNodeDefaults({
          cpu: getTriggerValue(config.nodeDefaults.cpu) ?? FACTORY_NODE_DEFAULTS.cpu,
          memory: getTriggerValue(config.nodeDefaults.memory) ?? FACTORY_NODE_DEFAULTS.memory,
          disk: getTriggerValue(config.nodeDefaults.disk) ?? FACTORY_NODE_DEFAULTS.disk,
          temperature:
            getTriggerValue(config.nodeDefaults.temperature) ?? FACTORY_NODE_DEFAULTS.temperature,
        });
      }

      if (config.pbsDefaults) {
        setPBSDefaults({
          cpu: getTriggerValue(config.pbsDefaults.cpu) ?? FACTORY_PBS_DEFAULTS.cpu,
          memory: getTriggerValue(config.pbsDefaults.memory) ?? FACTORY_PBS_DEFAULTS.memory,
        });
      }

      if (config.agentDefaults) {
        setAgentDefaults({
          cpu: getTriggerValue(config.agentDefaults.cpu) ?? FACTORY_AGENT_DEFAULTS.cpu,
          memory: getTriggerValue(config.agentDefaults.memory) ?? FACTORY_AGENT_DEFAULTS.memory,
          disk: getTriggerValue(config.agentDefaults.disk) ?? FACTORY_AGENT_DEFAULTS.disk,
          diskTemperature:
            getTriggerValue(config.agentDefaults.diskTemperature) ??
            FACTORY_AGENT_DEFAULTS.diskTemperature,
        });
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
          cpu: getTriggerValue(config.dockerDefaults.cpu) ?? FACTORY_DOCKER_DEFAULTS.cpu,
          memory: getTriggerValue(config.dockerDefaults.memory) ?? FACTORY_DOCKER_DEFAULTS.memory,
          disk: getTriggerValue(config.dockerDefaults.disk) ?? FACTORY_DOCKER_DEFAULTS.disk,
          restartCount:
            config.dockerDefaults.restartCount ?? FACTORY_DOCKER_DEFAULTS.restartCount,
          restartWindow:
            config.dockerDefaults.restartWindow ?? FACTORY_DOCKER_DEFAULTS.restartWindow,
          memoryWarnPct:
            config.dockerDefaults.memoryWarnPct ?? FACTORY_DOCKER_DEFAULTS.memoryWarnPct,
          memoryCriticalPct:
            config.dockerDefaults.memoryCriticalPct ??
            FACTORY_DOCKER_DEFAULTS.memoryCriticalPct,
          serviceWarnGapPercent: serviceWarnGap,
          serviceCriticalGapPercent: serviceCriticalGap,
        });
        setDockerDisableConnectivity(Boolean(config.dockerDefaults.stateDisableConnectivity));
        setDockerPoweredOffSeverity(
          config.dockerDefaults.statePoweredOffSeverity === 'critical' ? 'critical' : 'warning',
        );
      }

      setDockerIgnoredPrefixes(config.dockerIgnoredContainerPrefixes ?? []);
      setIgnoredGuestPrefixes(config.ignoredGuestPrefixes ?? []);
      setGuestTagWhitelist(config.guestTagWhitelist ?? []);
      setGuestTagBlacklist(config.guestTagBlacklist ?? []);

      if (config.storageDefault) {
        setStorageDefault(getTriggerValue(config.storageDefault) ?? FACTORY_STORAGE_DEFAULT);
      }
      if (config.timeThresholds) {
        setTimeThresholds({
          guest: config.timeThresholds.guest ?? DEFAULT_DELAY_SECONDS,
          node: config.timeThresholds.node ?? DEFAULT_DELAY_SECONDS,
          storage: config.timeThresholds.storage ?? DEFAULT_DELAY_SECONDS,
          pbs: config.timeThresholds.pbs ?? DEFAULT_DELAY_SECONDS,
          agent: config.timeThresholds.agent ?? DEFAULT_DELAY_SECONDS,
        });
      }
      if (config.metricTimeThresholds) {
        setMetricTimeThresholds(normalizeMetricDelayMap(config.metricTimeThresholds));
      }

      if (config.backupDefaults) {
        const enabled = Boolean(config.backupDefaults.enabled);
        const rawWarning =
          config.backupDefaults.warningDays ?? FACTORY_BACKUP_DEFAULTS.warningDays;
        const rawCritical =
          config.backupDefaults.criticalDays ?? FACTORY_BACKUP_DEFAULTS.criticalDays;
        const safeCritical = Math.max(0, rawCritical);
        const normalizedWarning = Math.max(0, rawWarning);
        const warningDays =
          safeCritical > 0 && normalizedWarning > safeCritical ? safeCritical : normalizedWarning;
        const criticalDays = Math.max(safeCritical, warningDays);
        const freshHours =
          config.backupDefaults.freshHours ?? FACTORY_BACKUP_DEFAULTS.freshHours;
        const staleHours =
          config.backupDefaults.staleHours ?? FACTORY_BACKUP_DEFAULTS.staleHours;
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
      }

      if (config.snapshotDefaults) {
        const enabled = Boolean(config.snapshotDefaults.enabled);
        const rawWarning =
          config.snapshotDefaults.warningDays ?? FACTORY_SNAPSHOT_DEFAULTS.warningDays;
        const rawCritical =
          config.snapshotDefaults.criticalDays ?? FACTORY_SNAPSHOT_DEFAULTS.criticalDays;
        const safeCritical = Math.max(0, rawCritical);
        const normalizedWarning = Math.max(0, rawWarning);
        const warningDays =
          safeCritical > 0 && normalizedWarning > safeCritical ? safeCritical : normalizedWarning;
        const criticalDays = Math.max(safeCritical, warningDays);
        setSnapshotDefaults({ enabled, warningDays, criticalDays });
      }

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

      setDisableAllNodes(config.disableAllNodes ?? false);
      setDisableAllGuests(config.disableAllGuests ?? false);
      setDisableAllAgents(config.disableAllAgents ?? false);
      setDisableAllStorage(config.disableAllStorage ?? false);
      setDisableAllPBS(config.disableAllPBS ?? false);
      setDisableAllPMG(config.disableAllPMG ?? false);
      setDisableAllDockerHosts(config.disableAllDockerHosts ?? false);
      setDisableAllDockerServices(config.disableAllDockerServices ?? false);
      setDisableAllDockerContainers(config.disableAllDockerContainers ?? false);
      setDisableAllNodesOffline(config.disableAllNodesOffline ?? false);
      setDisableAllGuestsOffline(config.disableAllGuestsOffline ?? false);
      setDisableAllAgentsOffline(config.disableAllAgentsOffline ?? false);
      setDisableAllPBSOffline(config.disableAllPBSOffline ?? false);
      setDisableAllPMGOffline(config.disableAllPMGOffline ?? false);
      setDisableAllDockerHostsOffline(config.disableAllDockerHostsOffline ?? false);

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
          const quietHours = config.schedule.quietHours;
          const days = Array.isArray(quietHours.days)
            ? {
                sunday: quietHours.days.includes(0),
                monday: quietHours.days.includes(1),
                tuesday: quietHours.days.includes(2),
                wednesday: quietHours.days.includes(3),
                thursday: quietHours.days.includes(4),
                friday: quietHours.days.includes(5),
                saturday: quietHours.days.includes(6),
              }
            : ((quietHours.days as Record<string, boolean>) || createDefaultQuietHours().days);
          setScheduleQuietHours({
            enabled: quietHours.enabled || false,
            start: quietHours.start || '22:00',
            end: quietHours.end || '08:00',
            timezone:
              quietHours.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
            days,
            suppress: {
              performance: quietHours.suppress?.performance ?? false,
              storage: quietHours.suppress?.storage ?? false,
              offline: quietHours.suppress?.offline ?? false,
            },
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
            typeof groupingConfig.window === 'number'
              ? groupingConfig.window
              : GROUPING_WINDOW_DEFAULT_SECONDS;
          const normalizedGroupingWindowSeconds = Math.max(0, rawGroupingWindowSeconds);
          setScheduleGrouping({
            enabled:
              groupingConfig.enabled !== undefined
                ? Boolean(groupingConfig.enabled)
                : normalizedGroupingWindowSeconds > 0,
            window: Math.round(normalizedGroupingWindowSeconds / 60),
            byNode: groupingConfig.byNode !== undefined ? groupingConfig.byNode : true,
            byGuest: groupingConfig.byGuest !== undefined ? groupingConfig.byGuest : false,
          });
        }

        if (config.schedule.notifyOnResolve !== undefined) {
          setNotifyOnResolve(Boolean(config.schedule.notifyOnResolve));
        }

        if (config.schedule.escalation) {
          const levels = (config.schedule.escalation.levels || []).map((level) => ({
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
      } catch (emailError) {
        logger.error('Failed to load email configuration:', emailError);
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
      } catch (appriseError) {
        logger.error('Failed to load Apprise configuration:', appriseError);
        setDestConfigLoadError(getAlertDestinationsConfigLoadError());
      }

      if (options.notify) {
        notificationStore.success(getAlertConfigDiscardedSuccess());
      }
    } catch (error) {
      logger.error('Failed to load alert configuration:', error);
      setDestConfigLoadError(getAlertDestinationsConfigLoadError());
      if (options.notify) {
        notificationStore.error(getAlertConfigReloadFailure());
      }
    } finally {
      setIsReloadingConfig(false);
    }
  };

  const saveAlertConfiguration = async () => {
    const createHysteresisThreshold = (trigger: number | undefined, clearMargin = 5) => {
      const normalized = typeof trigger === 'number' ? trigger : 0;
      return {
        trigger: normalized,
        clear: Math.max(0, normalized - clearMargin),
      };
    };

    const snapshotConfig = snapshotDefaults();
    const normalizedSnapshotWarning = Math.max(0, snapshotConfig.warningDays ?? 0);
    const normalizedSnapshotCritical = Math.max(0, snapshotConfig.criticalDays ?? 0);
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
      dockerDefaultsValue.serviceWarnGapPercent > dockerDefaultsValue.serviceCriticalGapPercent
    ) {
      notificationStore.error(
        'Swarm service critical gap must be greater than or equal to the warning gap when enabled.',
      );
      return;
    }

    const normalizedCooldownMinutes = scheduleCooldown().enabled
      ? clampCooldownMinutes(scheduleCooldown().minutes)
      : 0;
    const normalizedMaxAlertsHour = clampMaxAlertsPerHour(scheduleCooldown().maxAlerts);
    const groupingState = scheduleGrouping();
    const groupingWindowSeconds =
      groupingState.enabled && groupingState.window >= 0 ? groupingState.window * 60 : 0;
    const groupingEnabled = groupingState.enabled && groupingWindowSeconds > 0;
    const activationConfig = props.alertsActivationConfig();

    const alertConfig = {
      enabled: activationConfig?.enabled ?? true,
      activationState: props.alertsActivationState() ?? undefined,
      activationTime: activationConfig?.activationTime ?? undefined,
      observationWindowHours: activationConfig?.observationWindowHours ?? undefined,
      disableAllNodes: disableAllNodes(),
      disableAllGuests: disableAllGuests(),
      disableAllAgents: disableAllAgents(),
      disableAllStorage: disableAllStorage(),
      disableAllPBS: disableAllPBS(),
      disableAllPMG: disableAllPMG(),
      disableAllDockerHosts: disableAllDockerHosts(),
      disableAllDockerContainers: disableAllDockerContainers(),
      disableAllDockerServices: disableAllDockerServices(),
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
      overrides: rawOverridesConfig(),
      schedule: {
        quietHours: scheduleQuietHours(),
        cooldown: normalizedCooldownMinutes,
        notifyOnResolve: notifyOnResolve(),
        maxAlertsHour: normalizedMaxAlertsHour,
        escalation: scheduleEscalation(),
        grouping: {
          enabled: groupingEnabled,
          window: groupingWindowSeconds,
          byNode: groupingState.byNode,
          byGuest: groupingState.byGuest,
        },
      },
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

    const currentEmailConfig = emailConfig();
    await NotificationsAPI.updateEmailConfig({
      enabled: currentEmailConfig.enabled,
      provider: currentEmailConfig.provider,
      server: currentEmailConfig.server,
      port: currentEmailConfig.port,
      username: currentEmailConfig.username,
      password: currentEmailConfig.password,
      from: currentEmailConfig.from,
      to: currentEmailConfig.to,
      tls: currentEmailConfig.tls,
      startTLS: currentEmailConfig.startTLS,
    } as EmailConfig);

    const currentAppriseConfig = appriseConfig();
    const updatedApprise = await NotificationsAPI.updateAppriseConfig({
      enabled: currentAppriseConfig.enabled,
      mode: currentAppriseConfig.mode,
      targets: parseAppriseTargets(currentAppriseConfig.targetsText),
      cliPath: currentAppriseConfig.cliPath,
      timeoutSeconds: currentAppriseConfig.timeoutSeconds,
      serverUrl: currentAppriseConfig.serverUrl,
      configKey: currentAppriseConfig.configKey,
      apiKey: currentAppriseConfig.apiKey,
      apiKeyHeader: currentAppriseConfig.apiKeyHeader,
      skipTlsVerify: currentAppriseConfig.skipTlsVerify,
    } as AppriseConfig);
    setAppriseConfig({
      enabled: updatedApprise.enabled ?? false,
      mode: updatedApprise.mode === 'http' ? 'http' : 'cli',
      targetsText: formatAppriseTargets(updatedApprise.targets),
      cliPath: updatedApprise.cliPath || 'apprise',
      timeoutSeconds:
        typeof updatedApprise.timeoutSeconds === 'number' && updatedApprise.timeoutSeconds > 0
          ? updatedApprise.timeoutSeconds
          : 15,
      serverUrl: updatedApprise.serverUrl || '',
      configKey: updatedApprise.configKey || '',
      apiKey: updatedApprise.apiKey || '',
      apiKeyHeader: updatedApprise.apiKeyHeader || 'X-API-KEY',
      skipTlsVerify: Boolean(updatedApprise.skipTlsVerify),
    });
    props.setHasUnsavedChanges(false);
    notificationStore.success(getAlertConfigSaveSuccess());
  };

  onMount(() => {
    void loadAlertConfiguration();
    const unsubscribeOrgSwitched = eventBus.on('org_switched', () => {
      void loadAlertConfiguration();
    });
    onCleanup(() => {
      unsubscribeOrgSwitched();
    });
  });

  let destReloadVersion = 0;
  createEffect(() => {
    if (props.activeTab() !== 'destinations') {
      return;
    }

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
      const failed = results.some((result) => result.status === 'rejected');
      if (failed) {
        results
          .filter((result): result is PromiseRejectedResult => result.status === 'rejected')
          .forEach((result) => {
            logger.error('Failed to reload notification configuration:', result.reason);
          });
        setDestConfigLoadError(getAlertDestinationsConfigLoadError());
      } else {
        setDestConfigLoadError(null);
      }
      setIsLoadingDestinations(false);
    });
  });

  return {
    isReloadingConfig,
    isLoadingDestinations,
    destConfigLoadError,
    overrides,
    setOverrides,
    rawOverridesConfig,
    setRawOverridesConfig,
    emailConfig,
    setEmailConfig,
    appriseConfig,
    setAppriseConfig,
    scheduleQuietHours,
    setScheduleQuietHours,
    scheduleCooldown,
    setScheduleCooldown,
    scheduleGrouping,
    setScheduleGrouping,
    scheduleEscalation,
    setScheduleEscalation,
    notifyOnResolve,
    setNotifyOnResolve,
    guestDefaults,
    setGuestDefaults,
    guestDisableConnectivity,
    setGuestDisableConnectivity,
    guestPoweredOffSeverity,
    setGuestPoweredOffSeverity,
    nodeDefaults,
    setNodeDefaults,
    pbsDefaults,
    setPBSDefaults,
    agentDefaults,
    setAgentDefaults,
    dockerDefaults,
    setDockerDefaults,
    dockerDisableConnectivity,
    setDockerDisableConnectivity,
    dockerPoweredOffSeverity,
    setDockerPoweredOffSeverity,
    dockerIgnoredPrefixes,
    setDockerIgnoredPrefixes,
    ignoredGuestPrefixes,
    setIgnoredGuestPrefixes,
    guestTagWhitelist,
    setGuestTagWhitelist,
    guestTagBlacklist,
    setGuestTagBlacklist,
    storageDefault,
    setStorageDefault,
    backupDefaults,
    setBackupDefaults,
    timeThresholds,
    setTimeThresholds,
    metricTimeThresholds,
    setMetricTimeThresholds,
    snapshotDefaults,
    setSnapshotDefaults,
    pmgThresholds,
    setPMGThresholds,
    disableAllNodes,
    setDisableAllNodes,
    disableAllGuests,
    setDisableAllGuests,
    disableAllAgents,
    setDisableAllAgents,
    disableAllStorage,
    setDisableAllStorage,
    disableAllPBS,
    setDisableAllPBS,
    disableAllPMG,
    setDisableAllPMG,
    disableAllDockerHosts,
    setDisableAllDockerHosts,
    disableAllDockerServices,
    setDisableAllDockerServices,
    disableAllDockerContainers,
    setDisableAllDockerContainers,
    disableAllNodesOffline,
    setDisableAllNodesOffline,
    disableAllGuestsOffline,
    setDisableAllGuestsOffline,
    disableAllAgentsOffline,
    setDisableAllAgentsOffline,
    disableAllPBSOffline,
    setDisableAllPBSOffline,
    disableAllPMGOffline,
    setDisableAllPMGOffline,
    disableAllDockerHostsOffline,
    setDisableAllDockerHostsOffline,
    allGuests,
    agentResources,
    pbsInstances,
    pmgInstances,
    resetGuestDefaults,
    resetNodeDefaults,
    resetPBSDefaults,
    resetAgentDefaults,
    resetDockerDefaults,
    resetDockerIgnoredPrefixes,
    resetStorageDefault,
    resetBackupDefaults,
    resetSnapshotDefaults,
    loadAlertConfiguration,
    saveAlertConfiguration,
    factoryGuestDefaults: FACTORY_GUEST_DEFAULTS,
    factoryNodeDefaults: FACTORY_NODE_DEFAULTS,
    factoryPBSDefaults: FACTORY_PBS_DEFAULTS,
    factoryAgentDefaults: FACTORY_AGENT_DEFAULTS,
    factoryDockerDefaults: FACTORY_DOCKER_DEFAULTS,
    factoryStorageDefault: FACTORY_STORAGE_DEFAULT,
    snapshotFactoryDefaults: FACTORY_SNAPSHOT_DEFAULTS,
    backupFactoryDefaults: FACTORY_BACKUP_DEFAULTS,
  };
}
