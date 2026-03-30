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
  const configurationSnapshotState = useAlertsConfigurationSnapshotState({
    setHasUnsavedChanges: props.setHasUnsavedChanges,
  });
  const destinationsState = useAlertDestinationsState({ activeTab: props.activeTab });
  const overridesState = useAlertOverridesState({
    allResources: props.allResources,
    byType: props.byType,
    children: props.children,
    hasUnsavedChanges: props.hasUnsavedChanges,
    setOverviewOverrides: props.setOverviewOverrides,
  });

  const loadAlertConfiguration = async (options: { notify?: boolean } = {}) => {
    setIsReloadingConfig(true);
    props.setHasUnsavedChanges(false);
    destinationsState.resetDestinations();
    configurationSnapshotState.applyConfigurationSnapshot(createDefaultAlertsConfigurationSnapshot());

    try {
      const config = await AlertsAPI.getConfig();
      configurationSnapshotState.applyConfigurationSnapshot(readAlertsConfigurationSnapshot(config));

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
      snapshot: configurationSnapshotState.captureConfigurationSnapshot(),
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
    ...configurationSnapshotState,
    allGuests: overridesState.allGuests,
    agentResources: overridesState.agentResources,
    containerRuntimeResources: overridesState.containerRuntimeResources,
    pbsInstances: overridesState.pbsInstances,
    pmgInstances: overridesState.pmgInstances,
    loadAlertConfiguration,
    saveAlertConfiguration,
  };
}
