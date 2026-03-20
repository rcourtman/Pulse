import { Component, Accessor, Setter, Show, createMemo, createSignal } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import type { Resource } from '@/types/resource';
import type { PBSInstance, PMGInstance } from '@/types/api';
import type { NodeConfig, NodeConfigWithStatus } from '@/types/nodes';
import type { SecurityStatus as SecurityStatusInfo } from '@/types/config';
import { notificationStore } from '@/stores/notifications';
import { CalloutCard } from '@/components/shared/CalloutCard';
import type { ToggleChangeEvent } from '@/components/shared/Toggle';
import { NodeModal } from './NodeModal';
import { PveNodesTable, PbsNodesTable, PmgNodesTable } from './ConfiguredNodeTables';
import { ProxmoxDeleteNodeDialog } from './ProxmoxDeleteNodeDialog';
import { ProxmoxDirectConnectionsCard } from './ProxmoxDirectConnectionsCard';
import { ProxmoxDiscoveryResultsCard } from './ProxmoxDiscoveryResultsCard';
import { SettingsSectionNav } from './SettingsSectionNav';
import {
  buildProxmoxDiscoveryPrefillNode,
  getProxmoxVariantPresentation,
} from '@/utils/proxmoxSettingsPresentation';
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
  const [prefillNode, setPrefillNode] = createSignal<Partial<NodeConfig> | null>(null);

  const activeAgent = () => props.selectedAgent();
  const activeConfig = createMemo(() => getProxmoxVariantPresentation(activeAgent()));
  const activeDiscoveredNodes = createMemo(() =>
    props.discoveredNodes().filter((node) => node.type === activeAgent()),
  );
  const activeConfiguredNodes = createMemo(() => {
    switch (activeAgent()) {
      case 'pve':
        return props.pveNodes();
      case 'pbs':
        return props.pbsNodes();
      case 'pmg':
        return props.pmgNodes();
    }
  });
  const hasDiscoveryTimeouts = () =>
    props.discoveryMode() === 'auto' &&
    (props.discoveryScanStatus().errors || []).some((error) => /timed out|timeout/i.test(error));

  const openCreateNode = (type: NodeType) => {
    setPrefillNode(null);
    props.setEditingNode(null);
    props.setCurrentNodeType(type);
    props.setModalResetKey((previous) => previous + 1);
    props.setShowNodeModal(true);
  };

  const openEditNode = (type: NodeType, node: NodeConfigWithStatus) => {
    setPrefillNode(null);
    props.setEditingNode(node);
    props.setCurrentNodeType(type);
    props.setShowNodeModal(true);
  };

  const openDiscoveredNode = (server: DiscoveredServer) => {
    setPrefillNode(buildProxmoxDiscoveryPrefillNode(server));
    props.setEditingNode(null);
    props.setCurrentNodeType(server.type);
    props.setModalResetKey((previous) => previous + 1);
    props.setShowNodeModal(true);
  };

  const closeNodeModal = () => {
    setPrefillNode(null);
    props.setShowNodeModal(false);
    props.setEditingNode(null);
    props.setModalResetKey((previous) => previous + 1);
  };

  const handleRefreshDiscovery = async () => {
    notificationStore.info('Refreshing discovery...', 2000);
    try {
      await props.triggerDiscoveryScan({ quiet: true });
    } finally {
      await props.loadDiscoveredNodes();
    }
  };

  const handleDiscoveryToggle = async (event: ToggleChangeEvent) => {
    if (props.envOverrides().discoveryEnabled || props.savingDiscoverySettings()) {
      event.preventDefault();
      return;
    }

    const success = await props.handleDiscoveryEnabledChange(event.currentTarget.checked);
    if (!success) {
      event.currentTarget.checked = props.discoveryEnabled();
    }
  };

  const renderConfiguredTable = () => {
    switch (activeAgent()) {
      case 'pve':
        return (
          <PveNodesTable
            nodes={props.pveNodes()}
            stateNodes={props.agentStateResources()}
            globalTemperatureMonitoringEnabled={props.temperatureMonitoringEnabled()}
            onTestConnection={props.testNodeConnection}
            onEdit={(node) => openEditNode('pve', node)}
            onDelete={props.requestDeleteNode}
            onRefreshCluster={props.refreshClusterNodes}
          />
        );
      case 'pbs':
        return (
          <PbsNodesTable
            nodes={props.pbsNodes()}
            statePbs={props.pbsInstances()}
            globalTemperatureMonitoringEnabled={props.temperatureMonitoringEnabled()}
            onTestConnection={props.testNodeConnection}
            onEdit={(node) => openEditNode('pbs', node)}
            onDelete={props.requestDeleteNode}
          />
        );
      case 'pmg':
        return (
          <PmgNodesTable
            nodes={props.pmgNodes()}
            statePmg={props.pmgInstances()}
            globalTemperatureMonitoringEnabled={props.temperatureMonitoringEnabled()}
            onTestConnection={props.testNodeConnection}
            onEdit={(node) => openEditNode('pmg', node)}
            onDelete={props.requestDeleteNode}
          />
        );
    }
  };

  const renderNodeModal = (type: NodeType) => (
    <Show when={props.isNodeModalVisible(type)}>
      <NodeModal
        isOpen={true}
        resetKey={props.modalResetKey()}
        onClose={closeNodeModal}
        nodeType={type}
        editingNode={
          props.editingNode()?.type === type ? (props.editingNode() ?? undefined) : undefined
        }
        prefillNode={prefillNode()?.type === type ? (prefillNode() ?? undefined) : undefined}
        securityStatus={props.securityStatus() ?? undefined}
        temperatureMonitoringEnabled={props.resolveTemperatureMonitoringEnabled(
          props.editingNode()?.type === type ? props.editingNode() : null,
        )}
        temperatureMonitoringLocked={props.temperatureMonitoringLocked()}
        savingTemperatureSetting={props.savingTemperatureSetting()}
        onToggleTemperatureMonitoring={
          props.editingNode()?.id
            ? (enabled: boolean) =>
                props.handleNodeTemperatureMonitoringChange(props.editingNode()!.id, enabled)
            : props.handleTemperatureMonitoringChange
        }
        onSave={props.saveNode}
      />
    </Show>
  );

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
            activeAgent={activeAgent()}
            activeConfig={activeConfig()}
            activeConfiguredNodes={activeConfiguredNodes()}
            activeDiscoveredNodes={activeDiscoveredNodes()}
            configuredTable={renderConfiguredTable()}
            discoveryEnabled={props.discoveryEnabled()}
            envOverrides={props.envOverrides()}
            initialLoadComplete={props.initialLoadComplete()}
            onDiscoveryToggle={handleDiscoveryToggle}
            onOpenCreateNode={openCreateNode}
            onRefreshDiscovery={handleRefreshDiscovery}
            savingDiscoverySettings={props.savingDiscoverySettings()}
          />

          <Show when={props.discoveryEnabled()}>
            <ProxmoxDiscoveryResultsCard
              activeConfig={activeConfig()}
              activeDiscoveredNodes={activeDiscoveredNodes()}
              discoveryScanStatus={props.discoveryScanStatus()}
              hasDiscoveryTimeouts={hasDiscoveryTimeouts()}
              onOpenDiscoveredNode={openDiscoveredNode}
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

      {renderNodeModal('pve')}
      {renderNodeModal('pbs')}
      {renderNodeModal('pmg')}
    </>
  );
};
