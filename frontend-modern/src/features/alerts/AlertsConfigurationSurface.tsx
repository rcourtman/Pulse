import { Show } from 'solid-js';

import { Card } from '@/components/shared/Card';
import {
  getAlertConfigDiscardLabel,
  getAlertConfigSaveChangesLabel,
  getAlertConfigSaveFailure,
  getAlertConfigUnsavedChangesLabel,
} from '@/utils/alertConfigPresentation';
import { logger } from '@/utils/logger';
import { notificationStore } from '@/stores/notifications';

import { DestinationsTab } from './tabs/DestinationsTab';
import { ScheduleTab } from './tabs/ScheduleTab';
import { ThresholdsTab } from './tabs/ThresholdsTab';
import {
  useAlertsConfigurationState,
  type AlertsConfigurationSurfaceProps,
} from './useAlertsConfigurationState';

export function AlertsConfigurationSurface(props: AlertsConfigurationSurfaceProps) {
  const state = useAlertsConfigurationState(props);

  return (
    <>
      <Show
        when={
          props.hasUnsavedChanges() &&
          props.activeTab() !== 'overview' &&
          props.activeTab() !== 'history'
        }
      >
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
                disabled={state.isReloadingConfig() || !!state.destConfigLoadError()}
                onClick={async () => {
                  try {
                    await state.saveAlertConfiguration();
                  } catch (error) {
                    logger.error('Failed to save configuration:', error);
                    notificationStore.error(
                      error instanceof Error ? error.message : getAlertConfigSaveFailure(),
                    );
                  }
                }}
              >
                {getAlertConfigSaveChangesLabel()}
              </button>
              <button
                class="flex-1 px-4 py-2 text-sm transition-colors border border-border rounded-md text-base-content hover:bg-surface-hover sm:flex-initial disabled:opacity-60 disabled:cursor-not-allowed"
                disabled={state.isReloadingConfig()}
                onClick={async () => {
                  await state.loadAlertConfiguration({ notify: true });
                }}
              >
                {getAlertConfigDiscardLabel(state.isReloadingConfig())}
              </button>
            </div>
          </div>
        </Card>
      </Show>

      <Show when={props.activeTab() === 'thresholds'}>
        <ThresholdsTab
          overrides={state.overrides}
          setOverrides={state.setOverrides}
          rawOverridesConfig={state.rawOverridesConfig}
          setRawOverridesConfig={state.setRawOverridesConfig}
          allGuests={state.allGuests}
          pbsInstances={state.pbsInstances()}
          pmgInstances={state.pmgInstances()}
          nodes={props.byType('agent')}
          agents={state.agentResources()}
          storage={props
            .allResources()
            .filter((resource) => resource.type === 'storage' || resource.type === 'datastore')}
          dockerHosts={props.byType('docker-host')}
          allResources={props.allResources()}
          guestDefaults={state.guestDefaults}
          guestDisableConnectivity={state.guestDisableConnectivity}
          setGuestDefaults={state.setGuestDefaults}
          setGuestDisableConnectivity={state.setGuestDisableConnectivity}
          guestPoweredOffSeverity={state.guestPoweredOffSeverity}
          setGuestPoweredOffSeverity={state.setGuestPoweredOffSeverity}
          nodeDefaults={state.nodeDefaults}
          setNodeDefaults={state.setNodeDefaults}
          agentDefaults={state.agentDefaults}
          setAgentDefaults={state.setAgentDefaults}
          pbsDefaults={state.pbsDefaults}
          setPBSDefaults={state.setPBSDefaults}
          dockerDefaults={state.dockerDefaults}
          dockerDisableConnectivity={state.dockerDisableConnectivity}
          setDockerDisableConnectivity={state.setDockerDisableConnectivity}
          dockerPoweredOffSeverity={state.dockerPoweredOffSeverity}
          setDockerPoweredOffSeverity={state.setDockerPoweredOffSeverity}
          setDockerDefaults={state.setDockerDefaults}
          dockerIgnoredPrefixes={state.dockerIgnoredPrefixes}
          setDockerIgnoredPrefixes={state.setDockerIgnoredPrefixes}
          ignoredGuestPrefixes={state.ignoredGuestPrefixes}
          setIgnoredGuestPrefixes={state.setIgnoredGuestPrefixes}
          guestTagWhitelist={state.guestTagWhitelist}
          setGuestTagWhitelist={state.setGuestTagWhitelist}
          guestTagBlacklist={state.guestTagBlacklist}
          setGuestTagBlacklist={state.setGuestTagBlacklist}
          storageDefault={state.storageDefault}
          setStorageDefault={state.setStorageDefault}
          resetGuestDefaults={state.resetGuestDefaults}
          resetNodeDefaults={state.resetNodeDefaults}
          resetAgentDefaults={state.resetAgentDefaults}
          timeThresholds={state.timeThresholds}
          metricTimeThresholds={state.metricTimeThresholds}
          setMetricTimeThresholds={state.setMetricTimeThresholds}
          backupDefaults={state.backupDefaults}
          setBackupDefaults={state.setBackupDefaults}
          snapshotDefaults={state.snapshotDefaults}
          setSnapshotDefaults={state.setSnapshotDefaults}
          pmgThresholds={state.pmgThresholds}
          setPMGThresholds={state.setPMGThresholds}
          activeAlerts={props.activeAlerts}
          setHasUnsavedChanges={props.setHasUnsavedChanges}
          removeAlerts={props.removeAlerts}
          disableAllNodes={state.disableAllNodes}
          setDisableAllNodes={state.setDisableAllNodes}
          disableAllGuests={state.disableAllGuests}
          setDisableAllGuests={state.setDisableAllGuests}
          disableAllAgents={state.disableAllAgents}
          setDisableAllAgents={state.setDisableAllAgents}
          disableAllStorage={state.disableAllStorage}
          setDisableAllStorage={state.setDisableAllStorage}
          disableAllPBS={state.disableAllPBS}
          setDisableAllPBS={state.setDisableAllPBS}
          disableAllPMG={state.disableAllPMG}
          setDisableAllPMG={state.setDisableAllPMG}
          disableAllDockerHosts={state.disableAllDockerHosts}
          setDisableAllDockerHosts={state.setDisableAllDockerHosts}
          disableAllDockerServices={state.disableAllDockerServices}
          setDisableAllDockerServices={state.setDisableAllDockerServices}
          disableAllDockerContainers={state.disableAllDockerContainers}
          setDisableAllDockerContainers={state.setDisableAllDockerContainers}
          disableAllNodesOffline={state.disableAllNodesOffline}
          setDisableAllNodesOffline={state.setDisableAllNodesOffline}
          disableAllGuestsOffline={state.disableAllGuestsOffline}
          setDisableAllGuestsOffline={state.setDisableAllGuestsOffline}
          disableAllAgentsOffline={state.disableAllAgentsOffline}
          setDisableAllAgentsOffline={state.setDisableAllAgentsOffline}
          disableAllPBSOffline={state.disableAllPBSOffline}
          setDisableAllPBSOffline={state.setDisableAllPBSOffline}
          disableAllPMGOffline={state.disableAllPMGOffline}
          setDisableAllPMGOffline={state.setDisableAllPMGOffline}
          disableAllDockerHostsOffline={state.disableAllDockerHostsOffline}
          setDisableAllDockerHostsOffline={state.setDisableAllDockerHostsOffline}
          resetPBSDefaults={state.resetPBSDefaults}
          resetDockerDefaults={state.resetDockerDefaults}
          resetDockerIgnoredPrefixes={state.resetDockerIgnoredPrefixes}
          resetStorageDefault={state.resetStorageDefault}
          resetSnapshotDefaults={state.resetSnapshotDefaults}
          resetBackupDefaults={state.resetBackupDefaults}
          factoryGuestDefaults={state.factoryGuestDefaults}
          factoryNodeDefaults={state.factoryNodeDefaults}
          factoryPBSDefaults={state.factoryPBSDefaults}
          factoryAgentDefaults={state.factoryAgentDefaults}
          factoryDockerDefaults={state.factoryDockerDefaults}
          factoryStorageDefault={state.factoryStorageDefault}
          snapshotFactoryDefaults={state.snapshotFactoryDefaults}
          backupFactoryDefaults={state.backupFactoryDefaults}
        />
      </Show>

      <Show when={props.activeTab() === 'destinations'}>
        <DestinationsTab
          setHasUnsavedChanges={props.setHasUnsavedChanges}
          emailConfig={state.emailConfig}
          setEmailConfig={state.setEmailConfig}
          appriseConfig={state.appriseConfig}
          setAppriseConfig={state.setAppriseConfig}
          configLoadError={state.destConfigLoadError}
          isRetrying={state.isReloadingConfig}
          isLoadingDestinations={state.isLoadingDestinations}
          onRetryLoad={() => void state.loadAlertConfiguration()}
        />
      </Show>

      <Show when={props.activeTab() === 'schedule'}>
        <ScheduleTab
          setHasUnsavedChanges={props.setHasUnsavedChanges}
          quietHours={state.scheduleQuietHours}
          setQuietHours={state.setScheduleQuietHours}
          cooldown={state.scheduleCooldown}
          setCooldown={state.setScheduleCooldown}
          grouping={state.scheduleGrouping}
          setGrouping={state.setScheduleGrouping}
          notifyOnResolve={state.notifyOnResolve}
          setNotifyOnResolve={state.setNotifyOnResolve}
          escalation={state.scheduleEscalation}
          setEscalation={state.setScheduleEscalation}
        />
      </Show>
    </>
  );
}
