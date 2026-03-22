import { createSignal, onCleanup, onMount } from 'solid-js';
import type { Accessor } from 'solid-js';

import { AlertsAPI } from '@/api/alerts';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import type { Alert } from '@/types/api';
import type { ActivationState, BackupAlertConfig, SnapshotAlertConfig } from '@/types/alerts';
import type { Resource, ResourceType } from '@/types/resource';
import {
  getAlertConfigDiscardedSuccess,
  getAlertConfigReloadFailure,
  getAlertConfigSaveSuccess,
} from '@/utils/alertConfigPresentation';
import { logger } from '@/utils/logger';

import {
  ALERT_DOCKER_GAP_VALIDATION_ERROR,
  createDefaultAlertsConfigurationSnapshot,
  buildAlertsConfigurationPayload,
  FACTORY_AGENT_DEFAULTS,
  FACTORY_BACKUP_DEFAULTS,
  FACTORY_DOCKER_DEFAULTS,
  FACTORY_DOCKER_STATE_DISABLE_CONNECTIVITY,
  FACTORY_DOCKER_STATE_SEVERITY,
  FACTORY_GUEST_DEFAULTS,
  FACTORY_NODE_DEFAULTS,
  FACTORY_PBS_DEFAULTS,
  FACTORY_SNAPSHOT_DEFAULTS,
  FACTORY_STORAGE_DEFAULT,
  readAlertsConfigurationSnapshot,
  type AlertsConfigurationSnapshot,
} from './alertsConfigurationModel';
import { useAlertDestinationsState } from './useAlertDestinationsState';
import { useAlertOverridesState } from './useAlertOverridesState';
import type {
  AlertTab,
  CooldownConfig,
  EscalationConfig,
  GroupingConfig,
  Override,
  QuietHoursConfig,
} from './types';

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

export function useAlertsConfigurationState(props: AlertsConfigurationSurfaceProps) {
  const defaultSnapshot = createDefaultAlertsConfigurationSnapshot();
  const [isReloadingConfig, setIsReloadingConfig] = createSignal(false);
  const [scheduleQuietHours, setScheduleQuietHours] = createSignal<QuietHoursConfig>(
    defaultSnapshot.scheduleQuietHours,
  );
  const [scheduleCooldown, setScheduleCooldown] = createSignal<CooldownConfig>(
    defaultSnapshot.scheduleCooldown,
  );
  const [scheduleGrouping, setScheduleGrouping] = createSignal<GroupingConfig>(
    defaultSnapshot.scheduleGrouping,
  );
  const [scheduleEscalation, setScheduleEscalation] = createSignal<EscalationConfig>(
    defaultSnapshot.scheduleEscalation,
  );
  const [notifyOnResolve, setNotifyOnResolve] = createSignal<boolean>(defaultSnapshot.notifyOnResolve);
  const [guestDefaults, setGuestDefaults] = createSignal<Record<string, number | undefined>>(
    defaultSnapshot.guestDefaults,
  );
  const [guestDisableConnectivity, setGuestDisableConnectivity] = createSignal(
    defaultSnapshot.guestDisableConnectivity,
  );
  const [guestPoweredOffSeverity, setGuestPoweredOffSeverity] = createSignal<
    'warning' | 'critical'
  >(defaultSnapshot.guestPoweredOffSeverity);
  const [nodeDefaults, setNodeDefaults] = createSignal<Record<string, number | undefined>>(
    defaultSnapshot.nodeDefaults,
  );
  const [pbsDefaults, setPBSDefaults] = createSignal<Record<string, number | undefined>>(
    defaultSnapshot.pbsDefaults,
  );
  const [agentDefaults, setAgentDefaults] = createSignal<Record<string, number | undefined>>(
    defaultSnapshot.agentDefaults,
  );
  const [dockerDefaults, setDockerDefaults] = createSignal(defaultSnapshot.dockerDefaults);
  const [dockerDisableConnectivity, setDockerDisableConnectivity] = createSignal(
    defaultSnapshot.dockerDisableConnectivity,
  );
  const [dockerPoweredOffSeverity, setDockerPoweredOffSeverity] = createSignal<
    'warning' | 'critical'
  >(defaultSnapshot.dockerPoweredOffSeverity);
  const [dockerIgnoredPrefixes, setDockerIgnoredPrefixes] = createSignal<string[]>(
    defaultSnapshot.dockerIgnoredPrefixes,
  );
  const [ignoredGuestPrefixes, setIgnoredGuestPrefixes] = createSignal<string[]>(
    defaultSnapshot.ignoredGuestPrefixes,
  );
  const [guestTagWhitelist, setGuestTagWhitelist] = createSignal<string[]>(
    defaultSnapshot.guestTagWhitelist,
  );
  const [guestTagBlacklist, setGuestTagBlacklist] = createSignal<string[]>(
    defaultSnapshot.guestTagBlacklist,
  );
  const [storageDefault, setStorageDefault] = createSignal(defaultSnapshot.storageDefault);
  const [backupDefaults, setBackupDefaults] = createSignal<BackupAlertConfig>(
    defaultSnapshot.backupDefaults,
  );
  const [timeThresholds, setTimeThresholds] = createSignal(defaultSnapshot.timeThresholds);
  const [metricTimeThresholds, setMetricTimeThresholds] = createSignal<
    Record<string, Record<string, number>>
  >(defaultSnapshot.metricTimeThresholds);
  const [snapshotDefaults, setSnapshotDefaults] = createSignal<SnapshotAlertConfig>(
    defaultSnapshot.snapshotDefaults,
  );
  const [pmgThresholds, setPMGThresholds] = createSignal(defaultSnapshot.pmgThresholds);
  const [disableAllNodes, setDisableAllNodes] = createSignal(defaultSnapshot.disableAllNodes);
  const [disableAllGuests, setDisableAllGuests] = createSignal(defaultSnapshot.disableAllGuests);
  const [disableAllAgents, setDisableAllAgents] = createSignal(defaultSnapshot.disableAllAgents);
  const [disableAllStorage, setDisableAllStorage] = createSignal(defaultSnapshot.disableAllStorage);
  const [disableAllPBS, setDisableAllPBS] = createSignal(defaultSnapshot.disableAllPBS);
  const [disableAllPMG, setDisableAllPMG] = createSignal(defaultSnapshot.disableAllPMG);
  const [disableAllDockerHosts, setDisableAllDockerHosts] = createSignal(
    defaultSnapshot.disableAllDockerHosts,
  );
  const [disableAllDockerServices, setDisableAllDockerServices] = createSignal(
    defaultSnapshot.disableAllDockerServices,
  );
  const [disableAllDockerContainers, setDisableAllDockerContainers] = createSignal(
    defaultSnapshot.disableAllDockerContainers,
  );
  const [disableAllNodesOffline, setDisableAllNodesOffline] = createSignal(
    defaultSnapshot.disableAllNodesOffline,
  );
  const [disableAllGuestsOffline, setDisableAllGuestsOffline] = createSignal(
    defaultSnapshot.disableAllGuestsOffline,
  );
  const [disableAllAgentsOffline, setDisableAllAgentsOffline] = createSignal(
    defaultSnapshot.disableAllAgentsOffline,
  );
  const [disableAllPBSOffline, setDisableAllPBSOffline] = createSignal(
    defaultSnapshot.disableAllPBSOffline,
  );
  const [disableAllPMGOffline, setDisableAllPMGOffline] = createSignal(
    defaultSnapshot.disableAllPMGOffline,
  );
  const [disableAllDockerHostsOffline, setDisableAllDockerHostsOffline] = createSignal(
    defaultSnapshot.disableAllDockerHostsOffline,
  );
  const destinationsState = useAlertDestinationsState({ activeTab: props.activeTab });
  const overridesState = useAlertOverridesState({
    allResources: props.allResources,
    byType: props.byType,
    children: props.children,
    hasUnsavedChanges: props.hasUnsavedChanges,
    setOverviewOverrides: props.setOverviewOverrides,
  });

  const applyConfigurationSnapshot = (snapshot: AlertsConfigurationSnapshot) => {
    setScheduleQuietHours({
      ...snapshot.scheduleQuietHours,
      days: { ...snapshot.scheduleQuietHours.days },
      suppress: { ...snapshot.scheduleQuietHours.suppress },
    });
    setScheduleCooldown({ ...snapshot.scheduleCooldown });
    setScheduleGrouping({ ...snapshot.scheduleGrouping });
    setScheduleEscalation({
      enabled: snapshot.scheduleEscalation.enabled,
      levels: snapshot.scheduleEscalation.levels.map((level) => ({ ...level })),
    });
    setNotifyOnResolve(snapshot.notifyOnResolve);
    setGuestDefaults({ ...snapshot.guestDefaults });
    setGuestDisableConnectivity(snapshot.guestDisableConnectivity);
    setGuestPoweredOffSeverity(snapshot.guestPoweredOffSeverity);
    setNodeDefaults({ ...snapshot.nodeDefaults });
    setPBSDefaults({ ...snapshot.pbsDefaults });
    setAgentDefaults({ ...snapshot.agentDefaults });
    setDockerDefaults({ ...snapshot.dockerDefaults });
    setDockerDisableConnectivity(snapshot.dockerDisableConnectivity);
    setDockerPoweredOffSeverity(snapshot.dockerPoweredOffSeverity);
    setDockerIgnoredPrefixes([...snapshot.dockerIgnoredPrefixes]);
    setIgnoredGuestPrefixes([...snapshot.ignoredGuestPrefixes]);
    setGuestTagWhitelist([...snapshot.guestTagWhitelist]);
    setGuestTagBlacklist([...snapshot.guestTagBlacklist]);
    setStorageDefault(snapshot.storageDefault);
    setBackupDefaults({
      ...snapshot.backupDefaults,
      ignoreVMIDs: [...(snapshot.backupDefaults.ignoreVMIDs ?? [])],
    });
    setTimeThresholds({ ...snapshot.timeThresholds });
    setMetricTimeThresholds(structuredClone(snapshot.metricTimeThresholds));
    setSnapshotDefaults({ ...snapshot.snapshotDefaults });
    setPMGThresholds({ ...snapshot.pmgThresholds });
    setDisableAllNodes(snapshot.disableAllNodes);
    setDisableAllGuests(snapshot.disableAllGuests);
    setDisableAllAgents(snapshot.disableAllAgents);
    setDisableAllStorage(snapshot.disableAllStorage);
    setDisableAllPBS(snapshot.disableAllPBS);
    setDisableAllPMG(snapshot.disableAllPMG);
    setDisableAllDockerHosts(snapshot.disableAllDockerHosts);
    setDisableAllDockerServices(snapshot.disableAllDockerServices);
    setDisableAllDockerContainers(snapshot.disableAllDockerContainers);
    setDisableAllNodesOffline(snapshot.disableAllNodesOffline);
    setDisableAllGuestsOffline(snapshot.disableAllGuestsOffline);
    setDisableAllAgentsOffline(snapshot.disableAllAgentsOffline);
    setDisableAllPBSOffline(snapshot.disableAllPBSOffline);
    setDisableAllPMGOffline(snapshot.disableAllPMGOffline);
    setDisableAllDockerHostsOffline(snapshot.disableAllDockerHostsOffline);
  };

  const captureConfigurationSnapshot = (): AlertsConfigurationSnapshot => ({
    scheduleQuietHours: {
      ...scheduleQuietHours(),
      days: { ...scheduleQuietHours().days },
      suppress: { ...scheduleQuietHours().suppress },
    },
    scheduleCooldown: { ...scheduleCooldown() },
    scheduleGrouping: { ...scheduleGrouping() },
    scheduleEscalation: {
      enabled: scheduleEscalation().enabled,
      levels: scheduleEscalation().levels.map((level) => ({ ...level })),
    },
    notifyOnResolve: notifyOnResolve(),
    guestDefaults: { ...guestDefaults() },
    guestDisableConnectivity: guestDisableConnectivity(),
    guestPoweredOffSeverity: guestPoweredOffSeverity(),
    nodeDefaults: { ...nodeDefaults() },
    pbsDefaults: { ...pbsDefaults() },
    agentDefaults: { ...agentDefaults() },
    dockerDefaults: { ...dockerDefaults() },
    dockerDisableConnectivity: dockerDisableConnectivity(),
    dockerPoweredOffSeverity: dockerPoweredOffSeverity(),
    dockerIgnoredPrefixes: [...dockerIgnoredPrefixes()],
    ignoredGuestPrefixes: [...ignoredGuestPrefixes()],
    guestTagWhitelist: [...guestTagWhitelist()],
    guestTagBlacklist: [...guestTagBlacklist()],
    storageDefault: storageDefault(),
    backupDefaults: {
      ...backupDefaults(),
      ignoreVMIDs: [...(backupDefaults().ignoreVMIDs ?? [])],
    },
    timeThresholds: { ...timeThresholds() },
    metricTimeThresholds: structuredClone(metricTimeThresholds()),
    snapshotDefaults: { ...snapshotDefaults() },
    pmgThresholds: { ...pmgThresholds() },
    disableAllNodes: disableAllNodes(),
    disableAllGuests: disableAllGuests(),
    disableAllAgents: disableAllAgents(),
    disableAllStorage: disableAllStorage(),
    disableAllPBS: disableAllPBS(),
    disableAllPMG: disableAllPMG(),
    disableAllDockerHosts: disableAllDockerHosts(),
    disableAllDockerServices: disableAllDockerServices(),
    disableAllDockerContainers: disableAllDockerContainers(),
    disableAllNodesOffline: disableAllNodesOffline(),
    disableAllGuestsOffline: disableAllGuestsOffline(),
    disableAllAgentsOffline: disableAllAgentsOffline(),
    disableAllPBSOffline: disableAllPBSOffline(),
    disableAllPMGOffline: disableAllPMGOffline(),
    disableAllDockerHostsOffline: disableAllDockerHostsOffline(),
  });

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

  const loadAlertConfiguration = async (options: { notify?: boolean } = {}) => {
    setIsReloadingConfig(true);
    props.setHasUnsavedChanges(false);
    destinationsState.resetDestinations();
    applyConfigurationSnapshot(createDefaultAlertsConfigurationSnapshot());

    try {
      const config = await AlertsAPI.getConfig();
      applyConfigurationSnapshot(readAlertsConfigurationSnapshot(config));

      overridesState.replaceRawOverridesConfig(config.overrides || {});

      await destinationsState.loadDestinations();

      if (options.notify) {
        notificationStore.success(getAlertConfigDiscardedSuccess());
      }
    } catch (error) {
      logger.error('Failed to load alert configuration:', error);
      if (options.notify) {
        notificationStore.error(getAlertConfigReloadFailure());
      }
    } finally {
      setIsReloadingConfig(false);
    }
  };

  const saveAlertConfiguration = async () => {
    const result = buildAlertsConfigurationPayload({
      snapshot: captureConfigurationSnapshot(),
      rawOverridesConfig: overridesState.rawOverridesConfig(),
      alertsActivationState: props.alertsActivationState(),
      alertsActivationConfig: props.alertsActivationConfig(),
    });
    if (result.dockerValidationError) {
      notificationStore.error(ALERT_DOCKER_GAP_VALIDATION_ERROR);
      return;
    }

    await AlertsAPI.updateConfig(result.alertConfig!);

    await destinationsState.saveDestinations();
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

  return {
    isReloadingConfig,
    isLoadingDestinations: destinationsState.isLoadingDestinations,
    destConfigLoadError: destinationsState.destConfigLoadError,
    overrides: overridesState.overrides,
    setOverrides: overridesState.setOverrides,
    rawOverridesConfig: overridesState.rawOverridesConfig,
    setRawOverridesConfig: overridesState.setRawOverridesConfig,
    emailConfig: destinationsState.emailConfig,
    setEmailConfig: destinationsState.setEmailConfig,
    appriseConfig: destinationsState.appriseConfig,
    setAppriseConfig: destinationsState.setAppriseConfig,
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
    allGuests: overridesState.allGuests,
    agentResources: overridesState.agentResources,
    pbsInstances: overridesState.pbsInstances,
    pmgInstances: overridesState.pmgInstances,
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
