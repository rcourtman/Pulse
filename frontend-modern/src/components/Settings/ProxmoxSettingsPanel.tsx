import { Component, Accessor, Setter, Show } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import type { Resource } from '@/types/resource';
import type { PBSInstance, PMGInstance } from '@/types/api';
import type { NodeConfig, NodeConfigWithStatus } from '@/types/nodes';
import type { SecurityStatus as SecurityStatusInfo } from '@/types/config';
import { CalloutCard } from '@/components/shared/CalloutCard';
import type { ToggleChangeEvent } from '@/components/shared/Toggle';
import { ProxmoxConfiguredNodesTable } from './ProxmoxConfiguredNodesTable';
import { ProxmoxDeleteNodeDialog } from './ProxmoxDeleteNodeDialog';
import { ProxmoxDirectConnectionsCard } from './ProxmoxDirectConnectionsCard';
import { ProxmoxDiscoveryResultsCard } from './ProxmoxDiscoveryResultsCard';
import { ProxmoxNodeModalStack } from './ProxmoxNodeModalStack';
import { SettingsSectionNav } from './SettingsSectionNav';
import { useProxmoxSettingsPanelState } from './useProxmoxSettingsPanelState';
import type {
  DiscoveredServer,
  DiscoveryScanStatus,
  NodeType,
} from './useInfrastructureSettingsState';

type DiscoveryMode = 'auto' | 'custom';

export interface ProxmoxSettingsPanelProps {
  selectedAgent: Accessor<NodeType>;
  onSelectAgent: (agent: NodeType) => void;
  initialLoadComplete: Accessor<boolean>;
  discoveryEnabled: Accessor<boolean>;
  discoveryMode: Accessor<DiscoveryMode>;
  discoveryScanStatus: Accessor<DiscoveryScanStatus>;
  discoveredNodes: Accessor<DiscoveredServer[]>;
  savingDiscoverySettings: Accessor<boolean>;
  envOverrides: Accessor<Record<string, boolean>>;
  agentStateResources: Accessor<Resource[]>;
  pbsInstances: Accessor<PBSInstance[]>;
  pmgInstances: Accessor<PMGInstance[]>;
  pveNodes: Accessor<NodeConfigWithStatus[]>;
  pbsNodes: Accessor<NodeConfigWithStatus[]>;
  pmgNodes: Accessor<NodeConfigWithStatus[]>;
  temperatureMonitoringEnabled: Accessor<boolean>;
  triggerDiscoveryScan: (options?: { quiet?: boolean }) => Promise<void>;
  loadDiscoveredNodes: () => Promise<void>;
  handleDiscoveryEnabledChange: (enabled: boolean) => Promise<boolean>;
  testNodeConnection: (nodeId: string) => void;
  requestDeleteNode: (node: NodeConfigWithStatus) => void;
  refreshClusterNodes: (nodeId: string) => Promise<void>;
  setShowNodeModal: Setter<boolean>;
  editingNode: Accessor<NodeConfigWithStatus | null>;
  setEditingNode: Setter<NodeConfigWithStatus | null>;
  setCurrentNodeType: Setter<NodeType>;
  modalResetKey: Accessor<number>;
  setModalResetKey: Setter<number>;
  isNodeModalVisible: (type: NodeType) => boolean;
  securityStatus: Accessor<SecurityStatusInfo | null>;
  resolveTemperatureMonitoringEnabled: (node?: NodeConfigWithStatus | null) => boolean;
  temperatureMonitoringLocked: Accessor<boolean>;
  savingTemperatureSetting: Accessor<boolean>;
  handleTemperatureMonitoringChange: (enabled: boolean) => Promise<void>;
  handleNodeTemperatureMonitoringChange: (nodeId: string, enabled: boolean | null) => Promise<void>;
  saveNode: (nodeData: Partial<NodeConfig>) => Promise<void>;
  showDeleteNodeModal: Accessor<boolean>;
  cancelDeleteNode: () => void;
  deleteNode: () => Promise<void>;
  deleteNodeLoading: Accessor<boolean>;
  nodePendingDeleteLabel: () => string;
  nodePendingDeleteHost: () => string;
  nodePendingDeleteType: () => string;
  nodePendingDeleteTypeLabel: () => string;
  embedded?: boolean;
}

export const ProxmoxSettingsPanel: Component<ProxmoxSettingsPanelProps> = (props) => {
  const navigate = useNavigate();
  const state = useProxmoxSettingsPanelState(props);

  const handleDiscoveryToggle = async (event: ToggleChangeEvent) => {
    const nextValue = await state.handleDiscoveryToggle(event.currentTarget.checked);
    if (nextValue !== event.currentTarget.checked) {
      event.preventDefault();
      event.currentTarget.checked = nextValue;
    }
  };

  return (
    <>
      <SettingsSectionNav
        current={props.selectedAgent()}
        onSelect={props.onSelectAgent}
        class="mb-6"
      />

      <Show when={!props.embedded}>
        <CalloutCard
          class="mb-6"
          icon={
            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
          }
          description={
            <>
              <p>
                <strong>Recommended:</strong> use the unified agent for Proxmox hosts. It
                auto-creates the API token, links the host, and unlocks temperature monitoring plus
                Pulse Patrol automation.
              </p>
              <p class="text-xs text-blue-700 dark:text-blue-300">
                Use this fallback path only when you cannot install the unified agent on the host.
              </p>
            </>
          }
        >
          <button
            type="button"
            onClick={() => navigate('/settings')}
            class="text-sm font-medium text-blue-700 underline hover:text-blue-800 dark:text-blue-300 dark:hover:text-blue-200"
          >
            Open infrastructure setup →
          </button>
        </CalloutCard>
      </Show>

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
