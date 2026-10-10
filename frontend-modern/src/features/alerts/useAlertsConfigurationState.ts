import { createSignal, onCleanup, onMount } from 'solid-js';
import type { Accessor } from 'solid-js';

import { AlertsAPI } from '@/api/alerts';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import type { Alert } from '@/types/api';
import type { ActivationState } from '@/types/alerts';
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
  readAlertsConfigurationSnapshot,
} from './alertsConfigurationModel';
import { useAlertDestinationsState } from './useAlertDestinationsState';
import { useAlertsConfigurationSnapshotState } from './useAlertsConfigurationSnapshotState';
import { useAlertOverridesState } from './useAlertOverridesState';
import type { AlertTab, Override } from './types';

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
  const [isReloadingConfig, setIsReloadingConfig] = createSignal(false);
  const [isSavingConfig, setIsSavingConfig] = createSignal(false);
  let draftRevision = 0;
  let configurationVersion = 0;
  const [suppressDirtyFlag, setSuppressDirtyFlag] = createSignal(false);
  const guardedSetHasUnsavedChanges = (value: boolean) => {
    if (value && suppressDirtyFlag()) return;
    if (value) ++draftRevision;
    props.setHasUnsavedChanges(value);
  };
  const configurationSnapshotState = useAlertsConfigurationSnapshotState({
    setHasUnsavedChanges: guardedSetHasUnsavedChanges,
  });
  const destinationsState = useAlertDestinationsState({
    activeTab: props.activeTab,
    canReload: () => !props.hasUnsavedChanges() && !isSavingConfig() && !isReloadingConfig(),
  });
  const overridesState = useAlertOverridesState({
    allResources: props.allResources,
    byType: props.byType,
    children: props.children,
    hasUnsavedChanges: props.hasUnsavedChanges,
    setOverviewOverrides: props.setOverviewOverrides,
  });

  const loadAlertConfiguration = async (options: { notify?: boolean } = {}) => {
    const thisVersion = ++configurationVersion;
    // A context change invalidates unsent writes from the old editor. It cannot
    // cancel or undo a request that has already reached the server.
    setIsSavingConfig(false);
    setIsReloadingConfig(true);
    setSuppressDirtyFlag(true);
    props.setHasUnsavedChanges(false);
    destinationsState.resetDestinations();
    configurationSnapshotState.applyConfigurationSnapshot(
      createDefaultAlertsConfigurationSnapshot(),
    );

    try {
      const config = await AlertsAPI.getConfig();
      if (thisVersion !== configurationVersion) return;
      configurationSnapshotState.applyConfigurationSnapshot(
        readAlertsConfigurationSnapshot(config),
      );

      overridesState.replaceRawOverridesConfig(config.overrides || {});

      await destinationsState.loadDestinations();
      if (thisVersion !== configurationVersion) return;

      if (options.notify) {
        notificationStore.success(getAlertConfigDiscardedSuccess());
      }
    } catch (error) {
      if (thisVersion !== configurationVersion) return;
      logger.error('Failed to load alert configuration:', error);
      if (options.notify) {
        notificationStore.error(getAlertConfigReloadFailure());
      }
    } finally {
      if (thisVersion === configurationVersion) {
        setIsReloadingConfig(false);
        queueMicrotask(() => {
          if (thisVersion === configurationVersion) setSuppressDirtyFlag(false);
        });
      }
    }
  };

  const saveAlertConfiguration = async () => {
    if (
      isSavingConfig() ||
      isReloadingConfig() ||
      destinationsState.isLoadingDestinations() ||
      destinationsState.destConfigLoadError()
    )
      return;
    const result = buildAlertsConfigurationPayload({
      snapshot: configurationSnapshotState.captureConfigurationSnapshot(),
      rawOverridesConfig: overridesState.rawOverridesConfig(),
      alertsActivationState: props.alertsActivationState(),
      alertsActivationConfig: props.alertsActivationConfig(),
    });
    if (result.dockerValidationError) {
      notificationStore.error(ALERT_DOCKER_GAP_VALIDATION_ERROR);
      return;
    }

    const destinationsSnapshot = destinationsState.captureDestinations();
    const savedRevision = draftRevision;
    const savedVersion = configurationVersion;
    const ownsSave = () => savedVersion === configurationVersion;
    setIsSavingConfig(true);
    try {
      await AlertsAPI.updateConfig(result.alertConfig!);
      if (!ownsSave()) return;
      await destinationsState.saveDestinations(destinationsSnapshot, ownsSave);
      if (!ownsSave()) return;

      const hasNewerChanges = savedRevision !== draftRevision;
      if (!hasNewerChanges) props.setHasUnsavedChanges(false);
      notificationStore.success(getAlertConfigSaveSuccess(hasNewerChanges));
    } catch (error) {
      if (ownsSave()) throw error;
    } finally {
      if (ownsSave()) setIsSavingConfig(false);
    }
  };

  onMount(() => {
    void loadAlertConfiguration();
    const unsubscribeOrgSwitched = eventBus.on('org_switched', () => {
      void loadAlertConfiguration();
    });
    onCleanup(() => {
      ++configurationVersion;
      unsubscribeOrgSwitched();
    });
  });

  return {
    isReloadingConfig,
    isSavingConfig,
    guardedSetHasUnsavedChanges,
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
    deadManPingUrl: destinationsState.deadManPingUrl,
    setDeadManPingUrl: destinationsState.setDeadManPingUrl,
    pushMinimumSeverity: destinationsState.pushMinimumSeverity,
    setPushMinimumSeverity: destinationsState.setPushMinimumSeverity,
    webhooks: destinationsState.webhooks,
    setWebhooks: destinationsState.setWebhooks,
    ...configurationSnapshotState,
    allGuests: overridesState.allGuests,
    agentResources: overridesState.agentResources,
    virtualizationHostResources: overridesState.virtualizationHostResources,
    containerRuntimeResources: overridesState.containerRuntimeResources,
    pbsInstances: overridesState.pbsInstances,
    pmgInstances: overridesState.pmgInstances,
    loadAlertConfiguration,
    saveAlertConfiguration,
  };
}
