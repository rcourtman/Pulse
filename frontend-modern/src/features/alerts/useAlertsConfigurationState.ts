import { createSignal, onCleanup, onMount } from 'solid-js';
import type { Accessor } from 'solid-js';

import { AlertsAPI } from '@/api/alerts';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import type { Alert } from '@/types/api';
import type { ActivationState } from '@/types/alerts';
import type { Resource, ResourceType } from '@/types/resource';
import {
  getAlertConfigDiscardedSuccess,
  getAlertConfigLoadError,
  getAlertConfigReloadFailure,
  getAlertConfigSaveSuccess,
} from '@/utils/alertConfigPresentation';
import { logger } from '@/utils/logger';
import { getOrgID } from '@/utils/apiClient';

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
  isCurrentReader?: Accessor<boolean>;
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
  const org = getOrgID() || 'default';
  const requestOptions = { expectedOrgID: org, retry: false } as const;
  let disposed = false;
  let readController: AbortController | undefined;
  const isCurrentReader = () =>
    !disposed && props.isCurrentReader?.() !== false && (getOrgID() || 'default') === org;
  const alertsActivation = useAlertsActivation();
  const [isReloadingConfig, setIsReloadingConfig] = createSignal(false);
  const [isConfigLoaded, setIsConfigLoaded] = createSignal(false);
  const [configLoadError, setConfigLoadError] = createSignal<string | null>(null);
  const [isSavingConfig, setIsSavingConfig] = createSignal(false);
  let draftRevision = 0;
  let configurationVersion = 0;
  const [suppressDirtyFlag, setSuppressDirtyFlag] = createSignal(false);
  const guardedSetHasUnsavedChanges = (value: boolean) => {
    if (!isCurrentReader()) return;
    if (value && suppressDirtyFlag()) return;
    if (value) ++draftRevision;
    props.setHasUnsavedChanges(value);
  };
  const configurationSnapshotState = useAlertsConfigurationSnapshotState({
    setHasUnsavedChanges: guardedSetHasUnsavedChanges,
  });
  const destinationsState = useAlertDestinationsState({
    activeTab: props.activeTab,
    isCurrentReader,
    canReload: () =>
      isConfigLoaded() && !props.hasUnsavedChanges() && !isSavingConfig() && !isReloadingConfig(),
  });
  const overridesState = useAlertOverridesState({
    allResources: props.allResources,
    byType: props.byType,
    children: props.children,
    hasUnsavedChanges: props.hasUnsavedChanges,
    setOverviewOverrides: props.setOverviewOverrides,
  });

  const loadAlertConfiguration = async (options: { notify?: boolean } = {}) => {
    if (!isCurrentReader()) return;
    readController?.abort();
    const controller = new AbortController();
    readController = controller;
    const thisVersion = ++configurationVersion;
    const ownsRead = () => isCurrentReader() && thisVersion === configurationVersion;
    // A context change invalidates unsent writes from the old editor. It cannot
    // cancel or undo a request that has already reached the server.
    setIsSavingConfig(false);
    setIsReloadingConfig(true);
    setIsConfigLoaded(false);
    setConfigLoadError(null);
    setSuppressDirtyFlag(true);
    props.setHasUnsavedChanges(false);
    destinationsState.resetDestinations();
    configurationSnapshotState.applyConfigurationSnapshot(
      createDefaultAlertsConfigurationSnapshot(),
    );

    try {
      const config = await AlertsAPI.getConfig({
        ...requestOptions,
        cache: 'no-store',
        signal: controller.signal,
      });
      if (!ownsRead()) return;
      configurationSnapshotState.applyConfigurationSnapshot(
        readAlertsConfigurationSnapshot(config),
      );

      overridesState.replaceRawOverridesConfig(config.overrides || {});

      await destinationsState.loadDestinations();
      if (!ownsRead()) return;
      setIsConfigLoaded(true);

      if (options.notify) {
        notificationStore.success(getAlertConfigDiscardedSuccess());
      }
    } catch (error) {
      if (!ownsRead()) return;
      setConfigLoadError(getAlertConfigLoadError());
      logger.error('Failed to load alert configuration:', error);
      if (options.notify) {
        notificationStore.error(getAlertConfigReloadFailure());
      }
    } finally {
      if (ownsRead()) {
        readController = undefined;
        setIsReloadingConfig(false);
        queueMicrotask(() => {
          if (ownsRead()) setSuppressDirtyFlag(false);
        });
      }
    }
  };

  const saveAlertConfiguration = async () => {
    if (
      !isCurrentReader() ||
      !isConfigLoaded() ||
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
    const ownsSave = () => isCurrentReader() && savedVersion === configurationVersion;
    setIsSavingConfig(true);
    let configSaved = false;
    try {
      await AlertsAPI.updateConfig(result.alertConfig!, requestOptions);
      configSaved = true;
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
      // Backup shields and metric bars on every page colour by the shared
      // activation config; re-read it so a saved limit shows without a
      // reload, even when a later destinations write fails.
      if (configSaved && ownsSave()) void alertsActivation.refreshConfig();
    }
  };

  onMount(() => {
    void loadAlertConfiguration();
    // The page's keyed reader owns workspace replacement. Standalone callers
    // without that owner retain the existing explicit reload event.
    const unsubscribeOrgSwitched = props.isCurrentReader
      ? () => undefined
      : eventBus.on('org_switched', () => {
          void loadAlertConfiguration();
        });
    onCleanup(() => {
      disposed = true;
      readController?.abort();
      ++configurationVersion;
      unsubscribeOrgSwitched();
    });
  });

  return {
    isReloadingConfig,
    isConfigLoaded,
    configLoadError,
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
