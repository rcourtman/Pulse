import { Component, Accessor, Setter, Show, For, createMemo, createSignal } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { Dynamic } from 'solid-js/web';
import Server from 'lucide-solid/icons/server';
import HardDrive from 'lucide-solid/icons/hard-drive';
import Mail from 'lucide-solid/icons/mail';
import Loader from 'lucide-solid/icons/loader';
import type { Resource } from '@/types/resource';
import type { PBSInstance, PMGInstance } from '@/types/api';
import type { NodeConfig, NodeConfigWithStatus } from '@/types/nodes';
import type { SecurityStatus as SecurityStatusInfo } from '@/types/config';
import { notificationStore } from '@/stores/notifications';
import { formatRelativeTime } from '@/utils/format';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { Toggle } from '@/components/shared/Toggle';
import type { ToggleChangeEvent } from '@/components/shared/Toggle';
import { NodeModal } from './NodeModal';
import { PveNodesTable, PbsNodesTable, PmgNodesTable } from './ConfiguredNodeTables';
import { SettingsSectionNav } from './SettingsSectionNav';
import type {
  DiscoveredServer,
  DiscoveryScanStatus,
  NodeType,
} from './useInfrastructureSettingsState';

type DiscoveryMode = 'auto' | 'custom';

interface ProxmoxSettingsPanelProps {
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
}

type VariantConfig = {
  title: string;
  addLabel: string;
  emptyTitle: string;
  emptyDescription: string;
  scanningLabel: string;
  emptyIcon: Component<{ class?: string }>;
  nameFromServer: (server: DiscoveredServer) => string;
  titleFromServer: (server: DiscoveredServer) => string;
};

const VARIANT_CONFIG: Record<NodeType, VariantConfig> = {
  pve: {
    title: 'Proxmox VE nodes',
    addLabel: 'Add PVE API Connection',
    emptyTitle: 'No PVE API connections configured',
    emptyDescription: 'Add a Proxmox VE API connection when the unified agent is not available',
    scanningLabel: 'Scanning your network for Proxmox VE servers…',
    emptyIcon: Server,
    nameFromServer: (server) => server.hostname || `pve-${server.ip}`,
    titleFromServer: (server) => server.hostname || `Proxmox VE at ${server.ip}`,
  },
  pbs: {
    title: 'Proxmox Backup Server nodes',
    addLabel: 'Add PBS API Connection',
    emptyTitle: 'No PBS API connections configured',
    emptyDescription:
      'Add a Proxmox Backup Server API connection when the unified agent is not available',
    scanningLabel: 'Scanning your network for Proxmox Backup Servers…',
    emptyIcon: HardDrive,
    nameFromServer: (server) => server.hostname || `pbs-${server.ip}`,
    titleFromServer: (server) => server.hostname || `Backup Server at ${server.ip}`,
  },
  pmg: {
    title: 'Proxmox Mail Gateway nodes',
    addLabel: 'Add PMG API Connection',
    emptyTitle: 'No PMG API connections configured',
    emptyDescription:
      'Add a Proxmox Mail Gateway API connection when the unified agent is not available',
    scanningLabel: 'Scanning your network for Proxmox Mail Gateway servers…',
    emptyIcon: Mail,
    nameFromServer: (server) => server.hostname || `pmg-${server.ip}`,
    titleFromServer: (server) => server.hostname || `Mail Gateway at ${server.ip}`,
  },
};

const buildDiscoveryPrefillNode = (server: DiscoveredServer): Partial<NodeConfig> => {
  const baseNode = {
    type: server.type,
    name: VARIANT_CONFIG[server.type].nameFromServer(server),
    host: `https://${server.ip}:${server.port}`,
    verifySSL: false,
  } as const;

  switch (server.type) {
    case 'pve':
      return {
        ...baseNode,
        user: '',
        tokenName: '',
        tokenValue: '',
        monitorVMs: true,
        monitorContainers: true,
        monitorStorage: true,
        monitorBackups: true,
        monitorPhysicalDisks: false,
      };
    case 'pbs':
      return {
        ...baseNode,
        user: '',
        tokenName: '',
        tokenValue: '',
        monitorDatastores: true,
        monitorSyncJobs: true,
        monitorVerifyJobs: true,
        monitorPruneJobs: true,
        monitorGarbageJobs: true,
      };
    case 'pmg':
      return {
        ...baseNode,
        user: '',
        monitorMailStats: true,
        monitorQueues: true,
        monitorQuarantine: true,
        monitorDomainStats: false,
      };
  }
};

export const ProxmoxSettingsPanel: Component<ProxmoxSettingsPanelProps> = (props) => {
  const navigate = useNavigate();
  const [prefillNode, setPrefillNode] = createSignal<Partial<NodeConfig> | null>(null);

  const activeAgent = () => props.selectedAgent();
  const activeConfig = createMemo(() => VARIANT_CONFIG[activeAgent()]);
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
    setPrefillNode(buildDiscoveryPrefillNode(server));
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
        editingNode={props.editingNode()?.type === type ? (props.editingNode() ?? undefined) : undefined}
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
      <SettingsSectionNav current={props.selectedAgent()} onSelect={props.onSelectAgent} class="mb-6" />

      <div class="rounded-md border border-blue-200 bg-blue-50 px-4 py-3 mb-6 dark:border-blue-800 dark:bg-blue-900">
        <div class="flex items-start gap-3">
          <svg
            class="w-5 h-5 text-blue-600 dark:text-blue-400 mt-0.5 flex-shrink-0"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          <div class="flex-1">
            <p class="text-sm text-blue-800 dark:text-blue-200">
              <strong>Recommended:</strong> use the unified agent for Proxmox hosts. It
              auto-creates the API token, links the host, and unlocks temperature monitoring plus
              Pulse Patrol automation.
            </p>
            <p class="mt-1 text-xs text-blue-700 dark:text-blue-300">
              This page is for API-only connections when you cannot install the unified agent on
              the host.
            </p>
            <button
              type="button"
              onClick={() => navigate('/settings')}
              class="mt-2 text-sm font-medium text-blue-700 hover:text-blue-800 dark:text-blue-300 dark:hover:text-blue-200 underline"
            >
              Open infrastructure setup →
            </button>
          </div>
        </div>
      </div>

      <div class="space-y-6 mt-6">
        <div class="space-y-4">
          <Show when={!props.initialLoadComplete()}>
            <div class="flex items-center justify-center rounded-md border border-dashed border-border bg-surface-alt py-12 text-sm text-muted">
              Loading configuration...
            </div>
          </Show>

          <Show when={props.initialLoadComplete()}>
            <Card padding="none" tone="glass">
              <div class="px-3 py-4 sm:px-6 sm:py-6 space-y-4">
                <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                  <h4 class="text-base font-semibold text-base-content">{activeConfig().title}</h4>
                  <div class="flex flex-wrap items-center justify-start gap-2 sm:justify-end">
                    <div
                      class="flex items-center gap-2 sm:gap-3"
                      title="Enable automatic discovery of Proxmox servers on your network"
                    >
                      <span class="text-xs sm:text-sm text-muted">Discovery</span>
                      <Toggle
                        checked={props.discoveryEnabled()}
                        onChange={handleDiscoveryToggle}
                        disabled={
                          props.envOverrides().discoveryEnabled || props.savingDiscoverySettings()
                        }
                        containerClass="gap-2"
                        label={
                          <span class="text-xs font-medium text-muted">
                            {props.discoveryEnabled() ? 'On' : 'Off'}
                          </span>
                        }
                      />
                    </div>

                    <Show when={props.discoveryEnabled()}>
                      <button
                        type="button"
                        onClick={handleRefreshDiscovery}
                        class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-gray-600 text-white rounded-md hover:bg-gray-700 transition-colors flex items-center gap-1"
                        title="Refresh discovered servers"
                      >
                        <svg
                          width="16"
                          height="16"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          stroke-width="2"
                        >
                          <polyline points="23 4 23 10 17 10"></polyline>
                          <polyline points="1 20 1 14 7 14"></polyline>
                          <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path>
                        </svg>
                        <span class="hidden sm:inline">Refresh</span>
                      </button>
                    </Show>

                    <button
                      type="button"
                      onClick={() => openCreateNode(activeAgent())}
                      class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors flex items-center gap-1"
                    >
                      <svg
                        width="16"
                        height="16"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2"
                      >
                        <line x1="12" y1="5" x2="12" y2="19"></line>
                        <line x1="5" y1="12" x2="19" y2="12"></line>
                      </svg>
                      <span class="sm:hidden">Add</span>
                      <span class="hidden sm:inline">{activeConfig().addLabel}</span>
                    </button>
                  </div>
                </div>

                <Show when={activeConfiguredNodes().length > 0}>{renderConfiguredTable()}</Show>

                <Show
                  when={
                    activeConfiguredNodes().length === 0 && activeDiscoveredNodes().length === 0
                  }
                >
                  <div class="flex flex-col items-center justify-center py-12 px-4 text-center">
                    <div class="rounded-full bg-surface-alt p-4 mb-4">
                      <Dynamic component={activeConfig().emptyIcon} class="h-8 w-8 text-muted" />
                    </div>
                    <p class="text-base font-medium text-base-content mb-1">
                      {activeConfig().emptyTitle}
                    </p>
                    <p class="text-sm text-muted">{activeConfig().emptyDescription}</p>
                  </div>
                </Show>
              </div>
            </Card>
          </Show>

          <Show when={props.discoveryEnabled()}>
            <div class="space-y-3">
              <div class="flex items-center gap-2 text-xs text-muted">
                <Show when={props.discoveryScanStatus().scanning}>
                  <span class="flex items-center gap-2">
                    <Loader class="h-4 w-4 animate-spin" />
                    <span>{activeConfig().scanningLabel}</span>
                  </span>
                </Show>
                <Show
                  when={
                    !props.discoveryScanStatus().scanning &&
                    (props.discoveryScanStatus().lastResultAt ||
                      props.discoveryScanStatus().lastScanStartedAt)
                  }
                >
                  <span>
                    Last scan{' '}
                    {formatRelativeTime(
                      props.discoveryScanStatus().lastResultAt ??
                        props.discoveryScanStatus().lastScanStartedAt,
                      { compact: true },
                    )}
                  </span>
                </Show>
              </div>

              <Show
                when={
                  props.discoveryScanStatus().errors && props.discoveryScanStatus().errors!.length
                }
              >
                <div class="text-xs text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-800 rounded-md p-2">
                  <span class="font-medium">Discovery issues:</span>
                  <ul class="list-disc ml-4 mt-1 space-y-0.5">
                    <For each={props.discoveryScanStatus().errors || []}>
                      {(error) => <li>{error}</li>}
                    </For>
                  </ul>
                  <Show when={hasDiscoveryTimeouts()}>
                    <p class="mt-2 text-[0.7rem] font-medium text-amber-700 dark:text-amber-300">
                      Large networks can time out in auto mode. Switch to a custom subnet for
                      faster, targeted scans.
                    </p>
                  </Show>
                </div>
              </Show>

              <Show
                when={
                  props.discoveryScanStatus().scanning && activeDiscoveredNodes().length === 0
                }
              >
                <div class="flex items-center gap-2 text-xs text-muted">
                  <Loader class="h-4 w-4 animate-spin" />
                  <span>
                    Waiting for responses… this can take up to a minute depending on your network
                    size.
                  </span>
                </div>
              </Show>

              <For each={activeDiscoveredNodes()}>
                {(server) => (
                  <button
                    type="button"
                    class="w-full bg-surface-hover rounded-md p-4 border border-border text-left opacity-75 hover:opacity-100 hover:border-blue-300 dark:hover:border-blue-700 transition-all"
                    onClick={() => openDiscoveredNode(server)}
                  >
                    <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-2">
                      <div class="flex-1 min-w-0">
                        <div class="flex items-start gap-3">
                          <div class="flex-shrink-0 w-3 h-3 mt-1.5 rounded-full bg-gray-400 animate-pulse"></div>
                          <div class="flex-1 min-w-0">
                            <h4 class="font-medium text-base-content">
                              {activeConfig().titleFromServer(server)}
                            </h4>
                            <p class="text-sm mt-1">
                              {server.ip}:{server.port}
                            </p>
                            <div class="flex items-center gap-2 mt-2">
                              <span class="text-xs px-2 py-1 bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-400 rounded">
                                Discovered
                              </span>
                              <span class="text-xs text-muted">
                                Click to configure an API connection
                              </span>
                            </div>
                          </div>
                        </div>
                      </div>
                      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" class="mt-1">
                        <path
                          d="M12 5v14m-7-7h14"
                          stroke="currentColor"
                          stroke-width="2"
                          stroke-linecap="round"
                        />
                      </svg>
                    </div>
                  </button>
                )}
              </For>
            </div>
          </Show>
        </div>
      </div>

      <Show when={props.showDeleteNodeModal()}>
        <div class="fixed inset-0 z-50 flex items-center justify-center bg-black p-4">
          <Card padding="lg" class="w-full max-w-lg space-y-5">
            <SectionHeader title={`Remove ${props.nodePendingDeleteLabel()}`} size="md" class="mb-1" />
            <div class="space-y-3 text-sm text-gray-600">
              <p>
                Removing this {props.nodePendingDeleteTypeLabel().toLowerCase()} also scrubs the
                Pulse footprint on the host — the proxy service, SSH key, API token, and bind
                mount are all cleaned up automatically.
              </p>
              <div class="rounded-md border border-blue-200 bg-blue-50 p-3 text-sm leading-relaxed dark:border-blue-800 dark:bg-blue-900 dark:text-blue-100">
                <p class="font-medium text-blue-900 dark:text-blue-100">What happens next</p>
                <ul class="mt-2 list-disc space-y-1 pl-4 text-blue-800 dark:text-blue-200 text-sm">
                  <li>Pulse removes the node entry and clears related alerts.</li>
                  <li>
                    {props.nodePendingDeleteHost() ? (
                      <>
                        The host <span class="font-semibold">{props.nodePendingDeleteHost()}</span>{' '}
                        loses the proxy service, SSH key, and API token.
                      </>
                    ) : (
                      'The host loses the proxy service, SSH key, and API token.'
                    )}
                  </li>
                  <li>
                    If the host comes back later, rerunning the setup script reinstalls everything
                    with a fresh key.
                  </li>
                  <Show when={props.nodePendingDeleteType() === 'pbs'}>
                    <li>
                      Backup user tokens on the PBS are removed, so jobs referencing them will no
                      longer authenticate until the node is re-added.
                    </li>
                  </Show>
                  <Show when={props.nodePendingDeleteType() === 'pmg'}>
                    <li>
                      Mail gateway tokens are removed as part of the cleanup; re-enroll to restore
                      outbound telemetry.
                    </li>
                  </Show>
                </ul>
              </div>
            </div>

            <div class="flex items-center justify-end gap-3 pt-2">
              <button
                type="button"
                onClick={props.cancelDeleteNode}
                class="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
                disabled={props.deleteNodeLoading()}
              >
                Keep node
              </button>
              <button
                type="button"
                onClick={props.deleteNode}
                disabled={props.deleteNodeLoading()}
                class="rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-red-500 dark:hover:bg-red-400"
              >
                {props.deleteNodeLoading() ? 'Removing…' : 'Remove node'}
              </button>
            </div>
          </Card>
        </div>
      </Show>

      {renderNodeModal('pve')}
      {renderNodeModal('pbs')}
      {renderNodeModal('pmg')}
    </>
  );
};
