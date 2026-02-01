import { Component, Show, For, Accessor, Setter, JSX } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { Toggle } from '@/components/shared/Toggle';
import type { ToggleChangeEvent } from '@/components/shared/Toggle';
import {
  PveNodesTable,
  PbsNodesTable,
  PmgNodesTable,
} from './ConfiguredNodeTables';
import { notificationStore } from '@/stores/notifications';
import Server from 'lucide-solid/icons/server';
import HardDrive from 'lucide-solid/icons/hard-drive';
import Mail from 'lucide-solid/icons/mail';
import Loader from 'lucide-solid/icons/loader-2';
import type { NodeConfig, NodeConfigWithStatus } from '@/types/nodes';
import type { Node, PBSInstance, PMGInstance, Host } from '@/types/api';

type AgentType = 'pve' | 'pbs' | 'pmg';

interface DiscoveredServer {
  ip: string;
  port: number;
  type: 'pve' | 'pbs' | 'pmg';
  hostname?: string;
}

interface DiscoveryScanStatus {
  scanning: boolean;
  lastScanStartedAt?: string;
  lastResultAt?: string;
  errors?: string[];
}



interface ProxmoxAgentNodesPanelProps {
  agentType: AgentType;

  // Node data
  nodes: Accessor<NodeConfigWithStatus[]>;
  discoveredNodes: Accessor<DiscoveredServer[]>;

  // State data for tables
  stateNodes?: Node[];
  stateHosts?: Host[];
  statePbs?: PBSInstance[];
  statePmg?: PMGInstance[];

  // Temperature monitoring
  temperatureMonitoringEnabled: Accessor<boolean>;

  // Discovery settings
  discoveryEnabled: Accessor<boolean>;
  discoveryMode: Accessor<'auto' | 'custom'>;
  discoveryScanStatus: Accessor<DiscoveryScanStatus>;
  envOverrides: Accessor<Record<string, boolean>>;
  savingDiscoverySettings: Accessor<boolean>;

  // Loading state
  initialLoadComplete: Accessor<boolean>;

  // Actions
  handleDiscoveryEnabledChange: (enabled: boolean) => Promise<boolean>;
  triggerDiscoveryScan: (opts: { quiet: boolean }) => Promise<void>;
  loadDiscoveredNodes: () => Promise<void>;
  testNodeConnection: (nodeId: string) => void;
  requestDeleteNode: (node: NodeConfig) => void;
  refreshClusterNodes?: (nodeId: string) => void;

  // Modal controls
  setEditingNode: Setter<NodeConfigWithStatus | null>;
  setCurrentNodeType: Setter<AgentType>;
  setModalResetKey: Setter<number>;
  setShowNodeModal: Setter<boolean>;

  // For PMG edit workaround
  allNodes?: Accessor<NodeConfig[]>;

  // Utility
  formatRelativeTime: (date: string | undefined) => string;
}

const AGENT_CONFIG: Record<AgentType, {
  title: string;
  addButtonText: string;
  emptyTitle: string;
  emptyDescription: string;
  scanningText: string;
  icon: () => JSX.Element;
  discoveryTooltip: string;
}> = {
  pve: {
    title: 'Proxmox VE nodes',
    addButtonText: 'Add PVE Node',
    emptyTitle: 'No PVE nodes configured',
    emptyDescription: 'Add a Proxmox VE node to start monitoring your infrastructure',
    scanningText: 'Scanning your network for Proxmox VE servers…',
    icon: () => <Server class="h-8 w-8 text-gray-400 dark:text-gray-500" />,
    discoveryTooltip: 'Enable automatic discovery of Proxmox servers on your network',
  },
  pbs: {
    title: 'Proxmox Backup Server nodes',
    addButtonText: 'Add PBS Node',
    emptyTitle: 'No PBS nodes configured',
    emptyDescription: 'Add a Proxmox Backup Server to monitor your backup infrastructure',
    scanningText: 'Scanning your network for Proxmox Backup Servers…',
    icon: () => <HardDrive class="h-8 w-8 text-gray-400 dark:text-gray-500" />,
    discoveryTooltip: 'Enable automatic discovery of PBS servers on your network',
  },
  pmg: {
    title: 'Proxmox Mail Gateway nodes',
    addButtonText: 'Add PMG Node',
    emptyTitle: 'No PMG nodes configured',
    emptyDescription: 'Add a Proxmox Mail Gateway to monitor mail queue and quarantine metrics',
    scanningText: 'Scanning network...',
    icon: () => <Mail class="h-8 w-8 text-gray-400 dark:text-gray-500" />,
    discoveryTooltip: 'Enable automatic discovery of PMG servers on your network',
  },
};

export const ProxmoxAgentNodesPanel: Component<ProxmoxAgentNodesPanelProps> = (props) => {
  const config = () => AGENT_CONFIG[props.agentType];
  const filteredDiscoveredNodes = () => props.discoveredNodes().filter((n) => n.type === props.agentType);

  const handleAddNode = () => {
    props.setEditingNode(null);
    props.setCurrentNodeType(props.agentType);
    props.setModalResetKey((prev) => prev + 1);
    props.setShowNodeModal(true);
  };

  const handleRefreshDiscovery = async () => {
    notificationStore.info('Refreshing discovery...', 2000);
    try {
      await props.triggerDiscoveryScan({ quiet: true });
    } finally {
      await props.loadDiscoveredNodes();
    }
  };

  const handleDiscoveredNodeClick = (server: DiscoveredServer) => {
    if (props.agentType === 'pmg') {
      // PMG uses a different approach - sets up modal then fills input
      props.setEditingNode(null);
      props.setCurrentNodeType('pmg');
      props.setModalResetKey((prev) => prev + 1);
      props.setShowNodeModal(true);
      setTimeout(() => {
        const hostInput = document.querySelector(
          'input[placeholder*="192.168"]',
        ) as HTMLInputElement;
        if (hostInput) {
          hostInput.value = server.ip;
          hostInput.dispatchEvent(new Event('input', { bubbles: true }));
        }
      }, 50);
    } else {
      // PVE and PBS pre-fill the node object
      const baseNode = {
        id: '',
        type: props.agentType,
        name: server.hostname || `${props.agentType}-${server.ip}`,
        host: `https://${server.ip}:${server.port}`,
        user: '',
        tokenName: '',
        tokenValue: '',
        verifySSL: false,
        status: 'pending' as const,
      } as NodeConfigWithStatus;

      if (props.agentType === 'pve') {
        Object.assign(baseNode, {
          monitorVMs: true,
          monitorContainers: true,
          monitorStorage: true,
          monitorBackups: true,
          monitorPhysicalDisks: false,
        });
      } else if (props.agentType === 'pbs') {
        Object.assign(baseNode, {
          monitorDatastores: true,
          monitorSyncJobs: true,
          monitorVerifyJobs: true,
          monitorPruneJobs: true,
          monitorGarbageJobs: true,
        });
      }

      props.setEditingNode(baseNode);
      props.setCurrentNodeType(props.agentType);
      props.setShowNodeModal(true);
    }
  };

  const renderNodesTable = () => {
    if (props.agentType === 'pve') {
      return (
        <PveNodesTable
          nodes={props.nodes()}
          stateNodes={props.stateNodes ?? []}
          stateHosts={props.stateHosts ?? []}
          globalTemperatureMonitoringEnabled={props.temperatureMonitoringEnabled()}
          onTestConnection={props.testNodeConnection}
          onEdit={(node) => {
            props.setEditingNode(node as NodeConfigWithStatus);
            props.setCurrentNodeType('pve');
            props.setShowNodeModal(true);
          }}
          onDelete={(node) => props.requestDeleteNode(node)}
          onRefreshCluster={props.refreshClusterNodes}
        />
      );
    } else if (props.agentType === 'pbs') {
      return (
        <PbsNodesTable
          nodes={props.nodes()}
          statePbs={props.statePbs ?? []}
          globalTemperatureMonitoringEnabled={props.temperatureMonitoringEnabled()}
          onTestConnection={props.testNodeConnection}
          onEdit={(node) => {
            props.setEditingNode(node as NodeConfigWithStatus);
            props.setCurrentNodeType('pbs');
            props.setShowNodeModal(true);
          }}
          onDelete={(node) => props.requestDeleteNode(node)}
        />
      );
    } else {
      return (
        <PmgNodesTable
          nodes={props.nodes()}
          statePmg={props.statePmg ?? []}
          globalTemperatureMonitoringEnabled={props.temperatureMonitoringEnabled()}
          onTestConnection={props.testNodeConnection}
          onEdit={(node) => {
            props.setEditingNode(props.allNodes?.().find((n) => n.id === node.id) as NodeConfigWithStatus ?? null);
            props.setCurrentNodeType('pmg');
            props.setModalResetKey((prev) => prev + 1);
            props.setShowNodeModal(true);
          }}
          onDelete={(node) => props.requestDeleteNode(node)}
        />
      );
    }
  };

  return (
    <div class="space-y-6 mt-6">
      <div class="space-y-4">
        {/* Loading state */}
        <Show when={!props.initialLoadComplete()}>
          <div class="flex items-center justify-center rounded-lg border border-dashed border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/40 py-12 text-sm text-gray-500 dark:text-gray-400">
            Loading configuration...
          </div>
        </Show>

        {/* Main content */}
        <Show when={props.initialLoadComplete()}>
          <Card padding="lg">
            <div class="space-y-4">
              {/* Header with actions */}
              <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                <h4 class="text-base font-semibold text-gray-900 dark:text-gray-100">
                  {config().title}
                </h4>
                <div class="flex flex-wrap items-center justify-start gap-2 sm:justify-end">
                  {/* Discovery toggle */}
                  <div
                    class="flex items-center gap-2 sm:gap-3"
                    title={config().discoveryTooltip}
                  >
                    <span class="text-xs sm:text-sm text-gray-600 dark:text-gray-400">
                      Discovery
                    </span>
                    <Toggle
                      checked={props.discoveryEnabled()}
                      onChange={async (e: ToggleChangeEvent) => {
                        if (
                          props.envOverrides().discoveryEnabled ||
                          props.savingDiscoverySettings()
                        ) {
                          e.preventDefault();
                          return;
                        }
                        const success = await props.handleDiscoveryEnabledChange(
                          e.currentTarget.checked,
                        );
                        if (!success) {
                          e.currentTarget.checked = props.discoveryEnabled();
                        }
                      }}
                      disabled={
                        props.envOverrides().discoveryEnabled || props.savingDiscoverySettings()
                      }
                      containerClass="gap-2"
                      label={
                        <span class="text-xs font-medium text-gray-600 dark:text-gray-400">
                          {props.discoveryEnabled() ? 'On' : 'Off'}
                        </span>
                      }
                    />
                  </div>

                  {/* Refresh button */}
                  <Show when={props.discoveryEnabled()}>
                    <button
                      type="button"
                      onClick={handleRefreshDiscovery}
                      class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-gray-600 text-white rounded-lg hover:bg-gray-700 transition-colors flex items-center gap-1"
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
                        <path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"></path>
                      </svg>
                      <span class="hidden sm:inline">Refresh</span>
                    </button>
                  </Show>

                  {/* Add button */}
                  <button
                    type="button"
                    onClick={handleAddNode}
                    class="px-2 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors flex items-center gap-1"
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
                    <span class="hidden sm:inline">{config().addButtonText}</span>
                  </button>
                </div>
              </div>

              {/* Nodes table */}
              <Show when={props.nodes().length > 0}>
                {renderNodesTable()}
              </Show>

              {/* Empty state */}
              <Show
                when={
                  props.nodes().length === 0 &&
                  filteredDiscoveredNodes().length === 0
                }
              >
                <div class="flex flex-col items-center justify-center py-12 px-4 text-center">
                  <div class="rounded-full bg-gray-100 dark:bg-gray-800 p-4 mb-4">
                    {config().icon()}
                  </div>
                  <p class="text-base font-medium text-gray-900 dark:text-gray-100 mb-1">
                    {config().emptyTitle}
                  </p>
                  <p class="text-sm text-gray-500 dark:text-gray-400">
                    {config().emptyDescription}
                  </p>
                </div>
              </Show>
            </div>
          </Card>
        </Show>

        {/* Discovery section */}
        <Show when={props.discoveryEnabled()}>
          <div class="space-y-3">
            {/* Scan status */}
            <div class="flex items-center gap-2 text-xs text-gray-600 dark:text-gray-400">
              <Show when={props.discoveryScanStatus().scanning}>
                <span class="flex items-center gap-2">
                  <Loader class="h-4 w-4 animate-spin" />
                  <span>{config().scanningText}</span>
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
                  {props.formatRelativeTime(
                    props.discoveryScanStatus().lastResultAt ??
                    props.discoveryScanStatus().lastScanStartedAt,
                  )}
                </span>
              </Show>
            </div>

            {/* Discovery errors */}
            <Show
              when={
                props.discoveryScanStatus().errors && props.discoveryScanStatus().errors!.length
              }
            >
              <div class="text-xs text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-2">
                <span class="font-medium">Discovery issues:</span>
                <ul class="list-disc ml-4 mt-1 space-y-0.5">
                  <For each={props.discoveryScanStatus().errors || []}>
                    {(err) => <li>{err}</li>}
                  </For>
                </ul>
                <Show
                  when={
                    props.discoveryMode() === 'auto' &&
                    (props.discoveryScanStatus().errors || []).some((err) =>
                      /timed out|timeout/i.test(err),
                    )
                  }
                >
                  <p class="mt-2 text-[0.7rem] font-medium text-amber-700 dark:text-amber-300">
                    Large networks can time out in auto mode. Switch to a custom subnet
                    for faster, targeted scans.
                  </p>
                </Show>
              </div>
            </Show>

            {/* Scanning placeholder */}
            <Show
              when={
                props.discoveryScanStatus().scanning &&
                filteredDiscoveredNodes().length === 0
              }
            >
              <Show when={props.agentType === 'pmg'}>
                <div class="text-center py-6 text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-800/50 rounded-lg border-2 border-dashed border-gray-300 dark:border-gray-600">
                  <svg
                    class="h-8 w-8 mx-auto mb-2 animate-pulse text-purple-500"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <circle cx="11" cy="11" r="8" />
                    <path d="m21 21-4.35-4.35" />
                  </svg>
                  <p class="text-sm">Scanning for PMG servers...</p>
                </div>
              </Show>
              <Show when={props.agentType !== 'pmg'}>
                <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                  <Loader class="h-4 w-4 animate-spin" />
                  <span>
                    Waiting for responses… this can take up to a minute depending on your
                    network size.
                  </span>
                </div>
              </Show>
            </Show>

            {/* Discovered nodes list */}
            <For each={filteredDiscoveredNodes()}>
              {(server) => (
                <Show when={props.agentType === 'pmg'}>
                  {/* PMG style */}
                  <div
                    class="bg-purple-50 dark:bg-purple-900/20 border-l-4 border-purple-500 rounded-lg p-4 cursor-pointer hover:shadow-md transition-all"
                    onClick={() => handleDiscoveredNodeClick(server)}
                  >
                    <div class="flex items-start justify-between">
                      <div class="flex items-start gap-3 flex-1 min-w-0">
                        <svg
                          width="24"
                          height="24"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          stroke-width="2"
                          class="text-purple-500 flex-shrink-0 mt-0.5"
                        >
                          <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"></path>
                          <polyline points="22,6 12,13 2,6"></polyline>
                        </svg>
                        <div class="flex-1 min-w-0">
                          <h4 class="font-medium text-gray-900 dark:text-gray-100 truncate">
                            {server.hostname || `PMG at ${server.ip}`}
                          </h4>
                          <p class="text-sm text-gray-500 dark:text-gray-500 mt-1">
                            {server.ip}:{server.port}
                          </p>
                          <div class="flex items-center gap-2 mt-2">
                            <span class="text-xs px-2 py-1 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 rounded">
                              Discovered
                            </span>
                            <span class="text-xs text-gray-500 dark:text-gray-400">
                              Click to configure
                            </span>
                          </div>
                        </div>
                      </div>
                      <svg
                        width="20"
                        height="20"
                        viewBox="0 0 24 24"
                        fill="none"
                        class="text-gray-400 mt-1"
                      >
                        <path
                          d="M12 5v14m-7-7h14"
                          stroke="currentColor"
                          stroke-width="2"
                          stroke-linecap="round"
                        />
                      </svg>
                    </div>
                  </div>
                </Show>
              )}
            </For>
            <For each={filteredDiscoveredNodes()}>
              {(server) => (
                <Show when={props.agentType !== 'pmg'}>
                  {/* PVE/PBS style */}
                  <div
                    class="bg-gray-50/50 dark:bg-gray-700/30 rounded-lg p-4 border border-gray-200/50 dark:border-gray-600/50 opacity-75 hover:opacity-100 transition-opacity cursor-pointer"
                    onClick={() => handleDiscoveredNodeClick(server)}
                  >
                    <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-2">
                      <div class="flex-1 min-w-0">
                        <div class="flex items-start gap-3">
                          <div class="flex-shrink-0 w-3 h-3 mt-1.5 rounded-full bg-gray-400 animate-pulse"></div>
                          <div class="flex-1 min-w-0">
                            <h4 class="font-medium text-gray-700 dark:text-gray-300">
                              {server.hostname || `${props.agentType === 'pve' ? 'Proxmox VE' : 'Backup Server'} at ${server.ip}`}
                            </h4>
                            <p class="text-sm text-gray-500 dark:text-gray-500 mt-1">
                              {server.ip}:{server.port}
                            </p>
                            <div class="flex items-center gap-2 mt-2">
                              <span class="text-xs px-2 py-1 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 rounded">
                                Discovered
                              </span>
                              <span class="text-xs text-gray-500 dark:text-gray-400">
                                Click to configure
                              </span>
                            </div>
                          </div>
                        </div>
                      </div>
                      <svg
                        width="20"
                        height="20"
                        viewBox="0 0 24 24"
                        fill="none"
                        class="text-gray-400 mt-1"
                      >
                        <path
                          d="M12 5v14m-7-7h14"
                          stroke="currentColor"
                          stroke-width="2"
                          stroke-linecap="round"
                        />
                      </svg>
                    </div>
                  </div>
                </Show>
              )}
            </For>
          </div>
        </Show>
      </div>
    </div>
  );
};
