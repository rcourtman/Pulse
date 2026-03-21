import { Component, Show } from 'solid-js';
import type { ToggleChangeEvent } from '@/components/shared/Toggle';
import { ProxmoxConfiguredNodesTable } from './ProxmoxConfiguredNodesTable';
import { ProxmoxDeleteNodeDialog } from './ProxmoxDeleteNodeDialog';
import { ProxmoxDirectConnectionsCard } from './ProxmoxDirectConnectionsCard';
import { ProxmoxDiscoveryResultsCard } from './ProxmoxDiscoveryResultsCard';
import { ProxmoxNodeModalStack } from './ProxmoxNodeModalStack';
import type { ProxmoxSettingsPanelProps } from './proxmoxSettingsModel';
import { useProxmoxDirectWorkspaceState } from './useProxmoxDirectWorkspaceState';

export const ProxmoxDirectWorkspace: Component<ProxmoxSettingsPanelProps> = (props) => {
  const state = useProxmoxDirectWorkspaceState(props);

  const handleDiscoveryToggle = async (event: ToggleChangeEvent) => {
    const nextValue = await state.handleDiscoveryToggle(event.currentTarget.checked);
    if (nextValue !== event.currentTarget.checked) {
      event.preventDefault();
      event.currentTarget.checked = nextValue;
    }
  };

  return (
    <>
      <div class="space-y-6 mt-6">
        <div class="space-y-4">
          <ProxmoxDirectConnectionsCard
            activeAgent={state.activeAgent()}
            activeConfig={state.activeConfig()}
            activeConfiguredNodes={state.activeConfiguredNodes()}
            activeDiscoveredNodes={state.activeDiscoveredNodes()}
            configuredTable={
              <ProxmoxConfiguredNodesTable
                activeAgent={state.activeAgent()}
                pveNodes={props.pveNodes()}
                pbsNodes={props.pbsNodes()}
                pmgNodes={props.pmgNodes()}
                agentStateResources={props.agentStateResources()}
                pbsInstances={props.pbsInstances()}
                pmgInstances={props.pmgInstances()}
                temperatureMonitoringEnabled={props.temperatureMonitoringEnabled()}
                onTestConnection={props.testNodeConnection}
                onEditNode={state.openEditNode}
                onDeleteNode={props.requestDeleteNode}
                onRefreshClusterNodes={props.refreshClusterNodes}
              />
            }
            discoveryEnabled={props.discoveryEnabled()}
            envOverrides={props.envOverrides()}
            initialLoadComplete={props.initialLoadComplete()}
            onDiscoveryToggle={handleDiscoveryToggle}
            onOpenCreateNode={state.openCreateNode}
            onRefreshDiscovery={state.handleRefreshDiscovery}
            savingDiscoverySettings={props.savingDiscoverySettings()}
          />

          <Show when={props.discoveryEnabled()}>
            <ProxmoxDiscoveryResultsCard
              activeConfig={state.activeConfig()}
              activeDiscoveredNodes={state.activeDiscoveredNodes()}
              discoveryScanStatus={props.discoveryScanStatus()}
              hasDiscoveryTimeouts={state.hasDiscoveryTimeouts()}
              onOpenDiscoveredNode={state.openDiscoveredNode}
            />
          </Show>
        </div>
      </div>

      <Show when={props.showDeleteNodeModal()}>
        <ProxmoxDeleteNodeDialog
          deleteNodeLoading={props.deleteNodeLoading()}
          nodePendingDeleteHost={props.nodePendingDeleteHost()}
          nodePendingDeleteLabel={props.nodePendingDeleteLabel()}
          nodePendingDeleteType={props.nodePendingDeleteType()}
          nodePendingDeleteTypeLabel={props.nodePendingDeleteTypeLabel()}
          onCancel={props.cancelDeleteNode}
          onDelete={props.deleteNode}
        />
      </Show>

      <ProxmoxNodeModalStack
        modalResetKey={props.modalResetKey()}
        prefillNode={state.prefillNode()}
        editingNodeType={props.editingNode()?.type ?? null}
        editingNode={props.editingNode}
        isNodeModalVisible={props.isNodeModalVisible}
        securityStatus={props.securityStatus()}
        resolveTemperatureMonitoringEnabled={props.resolveTemperatureMonitoringEnabled}
        temperatureMonitoringLocked={props.temperatureMonitoringLocked()}
        savingTemperatureSetting={props.savingTemperatureSetting()}
        handleNodeTemperatureMonitoringChange={props.handleNodeTemperatureMonitoringChange}
        handleTemperatureMonitoringChange={props.handleTemperatureMonitoringChange}
        saveNode={props.saveNode}
        onClose={state.closeNodeModal}
      />
    </>
  );
};
