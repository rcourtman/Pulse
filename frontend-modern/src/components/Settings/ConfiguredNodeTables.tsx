import { Component, For, Show, createMemo } from 'solid-js';
import type { NodeConfig, NodeConfigWithStatus } from '@/types/nodes';
import type { Node, PBSInstance, PMGInstance, Host } from '@/types/api';
import { Card } from '@/components/shared/Card';

interface PveNodesTableProps {
  nodes: NodeConfigWithStatus[];
  stateNodes: Node[];
  stateHosts?: Host[];
  globalTemperatureMonitoringEnabled?: boolean;
  onTestConnection: (nodeId: string) => void;
  onEdit: (node: NodeConfigWithStatus) => void;
  onDelete: (node: NodeConfigWithStatus) => void;
  onRefreshCluster?: (nodeId: string) => void;
}

type StatusMeta = { dotClass: string; label: string; labelClass: string };

const STATUS_META: Record<string, StatusMeta> = {
  online: {
    dotClass: 'bg-green-500',
    label: 'Online',
    labelClass: 'text-green-600 dark:text-green-400',
  },
  offline: {
    dotClass: 'bg-red-500',
    label: 'Offline',
    labelClass: 'text-red-600 dark:text-red-400',
  },
  degraded: {
    dotClass: 'bg-yellow-500',
    label: 'Degraded',
    labelClass: 'text-amber-600 dark:text-amber-400',
  },
  pending: {
    dotClass: 'bg-amber-500 animate-pulse',
    label: 'Pending',
    labelClass: 'text-amber-600 dark:text-amber-400',
  },
  unknown: {
    dotClass: 'bg-gray-400',
    label: 'Unknown',
    labelClass: 'text-gray-500 dark:text-gray-400',
  },
};

const isTemperatureMonitoringEnabled = (
  node: NodeConfigWithStatus,
  globalEnabled: boolean,
): boolean => {
  // Check per-node setting first, fall back to global
  if (node.temperatureMonitoringEnabled !== undefined && node.temperatureMonitoringEnabled !== null) {
    return node.temperatureMonitoringEnabled;
  }
  return globalEnabled;
};

const resolvePveStatusMeta = (
  node: NodeConfigWithStatus,
  stateNodes: PveNodesTableProps['stateNodes'],
): StatusMeta => {
  const stateNode = stateNodes.find((n) => n.instance === node.name);
  if (
    stateNode?.connectionHealth === 'unhealthy' ||
    stateNode?.connectionHealth === 'error' ||
    stateNode?.status === 'offline' ||
    stateNode?.status === 'disconnected'
  ) {
    return STATUS_META.offline;
  }
  if (stateNode?.connectionHealth === 'degraded') {
    return STATUS_META.degraded;
  }
  if (stateNode && (stateNode.status === 'online' || stateNode.connectionHealth === 'healthy')) {
    return STATUS_META.online;
  }

  switch (node.status) {
    case 'connected':
      return STATUS_META.online;
    case 'pending':
      return STATUS_META.pending;
    case 'disconnected':
    case 'offline':
    case 'error':
      return STATUS_META.offline;
    default:
      return STATUS_META.unknown;
  }
};


export const PveNodesTable: Component<PveNodesTableProps> = (props) => {
  return (
    <Card padding="none" tone="glass" class="overflow-x-auto rounded-lg">
      <table class="min-w-[900px] divide-y divide-gray-200 dark:divide-gray-700 text-sm">
        <thead class="bg-gray-50 dark:bg-gray-800/70">
          <tr>
            <th scope="col" class="py-2 pl-4 pr-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
              Node
            </th>
            <th scope="col" class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
              Credentials
            </th>
            <th scope="col" class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
              Capabilities
            </th>
            <th scope="col" class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
              Status
            </th>
            <th scope="col" class="px-3 py-2 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
              Actions
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200 dark:divide-gray-700 bg-white dark:bg-gray-900/40">
          <For each={props.nodes}>
            {(node) => {
              const statusMeta = createMemo(() => resolvePveStatusMeta(node, props.stateNodes));
              const clusterEndpoints = createMemo(() =>
                'clusterEndpoints' in node && node.clusterEndpoints ? node.clusterEndpoints : [],
              );
              const clusterName = createMemo(() =>
                'clusterName' in node && node.clusterName ? node.clusterName : 'Unknown',
              );
              return (
                <tr class="even:bg-gray-50/60 dark:even:bg-gray-800/30 hover:bg-blue-50/40 dark:hover:bg-blue-900/20 transition-colors">
                  <td class="align-top py-3 pl-4 pr-3">
                    <div class="min-w-0 space-y-1">
                      <div class="flex items-start gap-3">
                        <div class={`mt-1.5 h-3 w-3 rounded-full ${statusMeta().dotClass}`}></div>
                        <div class="min-w-0 flex-1">
                          <p class="font-medium text-gray-900 dark:text-gray-100 truncate">
                            {node.name}
                          </p>
                          <p class="text-xs text-gray-600 dark:text-gray-400 truncate">
                            {node.host}
                          </p>
                        </div>
                      </div>
                      <Show when={node.type === 'pve' && 'isCluster' in node && node.isCluster}>
                        <div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-100 dark:bg-gray-800 px-3 py-2 space-y-2">
                          <div class="flex items-center gap-2 text-xs font-semibold text-gray-700 dark:text-gray-300">
                            <span>{clusterName()} Cluster</span>
                            <span class="ml-auto text-[0.65rem] font-normal text-gray-500 dark:text-gray-500">
                              {clusterEndpoints().length} nodes
                            </span>
                          </div>
                          <Show when={clusterEndpoints().length > 0}>
                            <div class="flex flex-col gap-2">
                              <For each={clusterEndpoints()}>
                                {(endpoint) => {
                                  const pulseStatus = endpoint.PulseReachable === null || endpoint.PulseReachable === undefined
                                    ? 'unknown'
                                    : endpoint.PulseReachable
                                      ? 'reachable'
                                      : 'unreachable';

                                  const statusColor = endpoint.Online && pulseStatus === 'reachable'
                                    ? 'border-green-200 bg-green-50 text-green-700 dark:border-green-700 dark:bg-green-900/20 dark:text-green-300'
                                    : pulseStatus === 'unreachable'
                                      ? 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-700 dark:bg-amber-900/20 dark:text-amber-300'
                                      : endpoint.Online
                                        ? 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-700 dark:bg-blue-900/20 dark:text-blue-300'
                                        : 'border-gray-200 bg-gray-100 text-gray-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400';

                                  return (
                                    <div class={`rounded border px-3 py-2 text-[0.7rem] ${statusColor}`}>
                                      <div class="flex items-center gap-2 mb-1">
                                        <span class="font-semibold">{endpoint.NodeName}</span>
                                        <span class="text-[0.65rem] opacity-75">{endpoint.IP}</span>
                                      </div>
                                      <div class="flex flex-col gap-0.5 text-[0.65rem] opacity-90">
                                        <div class="flex items-center gap-1.5">
                                          <span class="w-16 font-medium">Proxmox:</span>
                                          <span>{endpoint.Online ? 'Online' : 'Offline'}</span>
                                        </div>
                                        <div class="flex items-center gap-1.5">
                                          <span class="w-16 font-medium">Pulse:</span>
                                          <span>
                                            {pulseStatus === 'reachable' ? 'Reachable' : pulseStatus === 'unreachable' ? 'Unreachable' : 'Checking...'}
                                          </span>
                                        </div>
                                        <Show when={pulseStatus === 'unreachable' && endpoint.PulseError}>
                                          <div class="mt-1 pt-1 border-t border-current/20">
                                            <span class="font-medium">Error: </span>
                                            <span class="opacity-80">{endpoint.PulseError}</span>
                                          </div>
                                        </Show>
                                      </div>
                                    </div>
                                  );
                                }}
                              </For>
                            </div>
                          </Show>
                          <div class="flex items-center justify-between gap-2">
                            <p class="flex items-center gap-1 text-[0.7rem] text-gray-600 dark:text-gray-400">
                              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <path d="M5 12h14M12 5l7 7-7 7" />
                              </svg>
                              Automatic failover enabled
                            </p>
                            <Show when={props.onRefreshCluster}>
                              <button
                                type="button"
                                onClick={() => props.onRefreshCluster?.(node.id)}
                                class="flex items-center gap-1 px-2 py-1 text-[0.65rem] font-medium text-gray-600 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded hover:bg-gray-50 dark:hover:bg-gray-600 transition-colors"
                                title="Re-detect cluster membership (use if nodes were added to the Proxmox cluster)"
                              >
                                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                  <polyline points="23 4 23 10 17 10"></polyline>
                                  <polyline points="1 20 1 14 7 14"></polyline>
                                  <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path>
                                </svg>
                                Refresh
                              </button>
                            </Show>
                          </div>
                        </div>
                      </Show>
                    </div>
                  </td>
                  <td class="align-top px-3 py-3">
                    <div class="flex flex-col gap-1">
                      <span class="text-xs text-gray-600 dark:text-gray-400">
                        {node.user ? `User: ${node.user}` : `Token: ${node.tokenName}`}
                      </span>
                      <Show when={node.source === 'agent'}>
                        <span class="inline-flex items-center gap-1 text-[0.65rem] px-1.5 py-0.5 bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300 rounded w-fit">
                          <span class="h-1.5 w-1.5 rounded-full bg-purple-500"></span>
                          Agent
                        </span>
                      </Show>
                      <Show when={node.source === 'script' || (!node.source && node.tokenName)}>
                        <span class="text-[0.65rem] px-1.5 py-0.5 bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 rounded w-fit">
                          API only
                        </span>
                      </Show>
                    </div>
                  </td>
                  <td class="align-top px-3 py-3">
                    <div class="flex flex-wrap gap-1">
                      {node.type === 'pve' && 'monitorVMs' in node && node.monitorVMs && (
                        <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                          VMs
                        </span>
                      )}
                      {node.type === 'pve' && 'monitorContainers' in node && node.monitorContainers && (
                        <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                          Containers
                        </span>
                      )}
                      {node.type === 'pve' && 'monitorStorage' in node && node.monitorStorage && (
                        <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                          Storage
                        </span>
                      )}
                      {node.type === 'pve' && 'monitorBackups' in node && node.monitorBackups && (
                        <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                          Backups
                        </span>
                      )}
                      {node.type === 'pve' &&
                        'monitorPhysicalDisks' in node &&
                        node.monitorPhysicalDisks && (
                          <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                            Physical Disks
                          </span>
                        )}
                      {node.type === 'pve' &&
                        isTemperatureMonitoringEnabled(node, props.globalTemperatureMonitoringEnabled ?? true) && (
                          <span class="text-xs px-2 py-1 bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 rounded">
                            Temperature
                          </span>
                        )}
                    </div>
                  </td>
                  <td class="align-top px-3 py-3 whitespace-nowrap">
                    <span class={`inline-flex items-center gap-2 text-xs font-medium ${statusMeta().labelClass}`}>
                      <span class={`h-2.5 w-2.5 rounded-full ${statusMeta().dotClass}`}></span>
                      {statusMeta().label}
                    </span>
                  </td>
                  <td class="align-top px-3 py-3">
                    <div class="flex items-center justify-end gap-1 sm:gap-2">
                      <button
                        type="button"
                        onClick={() => props.onTestConnection(node.id)}
                        class="p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100"
                        title="Test connection"
                      >
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                          <polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline>
                        </svg>
                      </button>
                      <button
                        type="button"
                        onClick={() => props.onEdit(node)}
                        class="p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100"
                        title="Edit node"
                      >
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                          <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"></path>
                          <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                        </svg>
                      </button>
                      <button
                        type="button"
                        onClick={() => props.onDelete(node)}
                        class="p-2 text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300"
                        title="Delete node"
                      >
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                          <polyline points="3 6 5 6 21 6"></polyline>
                          <path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"></path>
                        </svg>
                      </button>
                    </div>
                  </td>
                </tr>
              );
            }}
          </For>
        </tbody>
      </table>
    </Card>
  );
};

interface PbsNodesTableProps {
  nodes: NodeConfigWithStatus[];
  statePbs: PBSInstance[];
  globalTemperatureMonitoringEnabled?: boolean;
  onTestConnection: (nodeId: string) => void;
  onEdit: (node: NodeConfigWithStatus) => void;
  onDelete: (node: NodeConfigWithStatus) => void;
}

const resolvePbsStatusMeta = (
  node: NodeConfigWithStatus,
  statePbs: PbsNodesTableProps['statePbs'],
): StatusMeta => {
  const statePBS = statePbs.find((p) => p.name === node.name);
  if (
    statePBS?.connectionHealth === 'unhealthy' ||
    statePBS?.connectionHealth === 'error' ||
    statePBS?.status === 'offline' ||
    statePBS?.status === 'disconnected'
  ) {
    return STATUS_META.offline;
  }
  if (statePBS?.connectionHealth === 'degraded') {
    return STATUS_META.degraded;
  }
  if (statePBS && (statePBS.status === 'online' || statePBS.connectionHealth === 'healthy')) {
    return STATUS_META.online;
  }

  switch (node.status) {
    case 'connected':
      return STATUS_META.online;
    case 'pending':
      return STATUS_META.pending;
    case 'disconnected':
    case 'offline':
    case 'error':
      return STATUS_META.offline;
    default:
      return STATUS_META.unknown;
  }
};

export const PbsNodesTable: Component<PbsNodesTableProps> = (props) => {
  return (
    <Card padding="none" tone="glass" class="overflow-x-auto rounded-lg">
      <table class="min-w-[900px] divide-y divide-gray-200 dark:divide-gray-700 text-sm">
        <thead class="bg-gray-50 dark:bg-gray-800/70">
          <tr>
            <th scope="col" class="py-2 pl-4 pr-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
              Node
            </th>
            <th scope="col" class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
              Credentials
            </th>
            <th scope="col" class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
              Capabilities
            </th>
            <th scope="col" class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
              Status
            </th>
            <th scope="col" class="px-3 py-2 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
              Actions
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200 dark:divide-gray-700 bg-white dark:bg-gray-900/40">
          <For each={props.nodes}>
            {(node) => {
              const statusMeta = createMemo(() => resolvePbsStatusMeta(node, props.statePbs));
              return (
                <tr class="even:bg-gray-50/60 dark:even:bg-gray-800/30 hover:bg-blue-50/40 dark:hover:bg-blue-900/20 transition-colors">
                  <td class="align-top py-3 pl-4 pr-3">
                    <div class="min-w-0 space-y-1">
                      <div class="flex items-start gap-3">
                        <div class={`mt-1.5 h-3 w-3 rounded-full ${statusMeta().dotClass}`}></div>
                        <div class="min-w-0 flex-1">
                          <p class="font-medium text-gray-900 dark:text-gray-100 truncate">
                            {node.name}
                          </p>
                          <p class="text-xs text-gray-600 dark:text-gray-400 truncate">
                            {node.host}
                          </p>
                        </div>
                      </div>
                    </div>
                  </td>
                  <td class="align-top px-3 py-3">
                    <div class="flex flex-col gap-1">
                      <span class="text-xs text-gray-600 dark:text-gray-400">
                        {node.user ? `User: ${node.user}` : `Token: ${node.tokenName}`}
                      </span>
                      <Show when={node.source === 'agent'}>
                        <span class="inline-flex items-center gap-1 text-[0.65rem] px-1.5 py-0.5 bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300 rounded w-fit">
                          <span class="h-1.5 w-1.5 rounded-full bg-purple-500"></span>
                          Agent
                        </span>
                      </Show>
                      <Show when={node.source === 'script' || (!node.source && node.tokenName)}>
                        <span class="text-[0.65rem] px-1.5 py-0.5 bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 rounded w-fit">
                          API only
                        </span>
                      </Show>
                    </div>
                  </td>
                  <td class="align-top px-3 py-3">
                    <div class="flex flex-wrap gap-1">
                      {node.type === 'pbs' && 'monitorDatastores' in node && node.monitorDatastores && (
                        <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                          Datastores
                        </span>
                      )}
                      {node.type === 'pbs' && 'monitorSyncJobs' in node && node.monitorSyncJobs && (
                        <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                          Sync Jobs
                        </span>
                      )}
                      {node.type === 'pbs' &&
                        'monitorVerifyJobs' in node &&
                        node.monitorVerifyJobs && (
                          <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                            Verify Jobs
                          </span>
                        )}
                      {node.type === 'pbs' && 'monitorPruneJobs' in node && node.monitorPruneJobs && (
                        <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                          Prune Jobs
                        </span>
                      )}
                      {node.type === 'pbs' &&
                        'monitorGarbageJobs' in node &&
                        (node as NodeConfig & { monitorGarbageJobs?: boolean }).monitorGarbageJobs && (
                          <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                            Garbage Collection
                          </span>
                        )}
                      {node.type === 'pbs' &&
                        isTemperatureMonitoringEnabled(node, props.globalTemperatureMonitoringEnabled ?? true) && (
                          <span class="text-xs px-2 py-1 bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 rounded">
                            Temperature
                          </span>
                        )}
                    </div>
                  </td>
                  <td class="align-top px-3 py-3 whitespace-nowrap">
                    <span class={`inline-flex items-center gap-2 text-xs font-medium ${statusMeta().labelClass}`}>
                      <span class={`h-2.5 w-2.5 rounded-full ${statusMeta().dotClass}`}></span>
                      {statusMeta().label}
                    </span>
                  </td>
                  <td class="align-top px-3 py-3">
                    <div class="flex items-center justify-end gap-1 sm:gap-2">
                      <button
                        type="button"
                        onClick={() => props.onTestConnection(node.id)}
                        class="p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100"
                        title="Test connection"
                      >
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                          <polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline>
                        </svg>
                      </button>
                      <button
                        type="button"
                        onClick={() => props.onEdit(node)}
                        class="p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100"
                        title="Edit node"
                      >
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                          <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"></path>
                          <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                        </svg>
                      </button>
                      <button
                        type="button"
                        onClick={() => props.onDelete(node)}
                        class="p-2 text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300"
                        title="Delete node"
                      >
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                          <polyline points="3 6 5 6 21 6"></polyline>
                          <path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"></path>
                        </svg>
                      </button>
                    </div>
                  </td>
                </tr>
              );
            }}
          </For>
        </tbody>
      </table>
    </Card>
  );
};

interface PmgNodesTableProps {
  nodes: NodeConfigWithStatus[];
  statePmg: PMGInstance[];
  globalTemperatureMonitoringEnabled?: boolean;
  onTestConnection: (nodeId: string) => void;
  onEdit: (node: NodeConfigWithStatus) => void;
  onDelete: (node: NodeConfigWithStatus) => void;
}

const resolvePmgStatusMeta = (
  node: NodeConfigWithStatus,
  statePmg: PmgNodesTableProps['statePmg'],
): StatusMeta => {
  const statePMG = statePmg.find((p) => p.name === node.name);
  if (
    statePMG?.connectionHealth === 'unhealthy' ||
    statePMG?.connectionHealth === 'error' ||
    statePMG?.status === 'offline' ||
    statePMG?.status === 'disconnected'
  ) {
    return STATUS_META.offline;
  }
  if (statePMG?.connectionHealth === 'degraded') {
    return STATUS_META.degraded;
  }
  if (statePMG && (statePMG.status === 'online' || statePMG.connectionHealth === 'healthy')) {
    return STATUS_META.online;
  }

  switch (node.status) {
    case 'connected':
      return STATUS_META.online;
    case 'pending':
      return STATUS_META.pending;
    case 'disconnected':
    case 'offline':
    case 'error':
      return STATUS_META.offline;
    default:
      return STATUS_META.unknown;
  }
};

export const PmgNodesTable: Component<PmgNodesTableProps> = (props) => {
  return (
    <Card padding="none" tone="glass" class="overflow-x-auto rounded-lg">
      <table class="min-w-[900px] divide-y divide-gray-200 dark:divide-gray-700 text-sm">
        <thead class="bg-gray-50 dark:bg-gray-800/70">
          <tr>
            <th scope="col" class="py-2 pl-4 pr-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
              Node
            </th>
            <th scope="col" class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
              Credentials
            </th>
            <th scope="col" class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
              Capabilities
            </th>
            <th scope="col" class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
              Status
            </th>
            <th scope="col" class="px-3 py-2 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
              Actions
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200 dark:divide-gray-700 bg-white dark:bg-gray-900/40">
          <For each={props.nodes}>
            {(node) => {
              const statusMeta = createMemo(() => resolvePmgStatusMeta(node, props.statePmg));
              return (
                <tr class="even:bg-gray-50/60 dark:even:bg-gray-800/30 hover:bg-blue-50/40 dark:hover:bg-blue-900/20 transition-colors">
                  <td class="align-top py-3 pl-4 pr-3">
                    <div class="min-w-0 space-y-1">
                      <div class="flex items-start gap-3">
                        <div class={`mt-1.5 h-3 w-3 rounded-full ${statusMeta().dotClass}`}></div>
                        <div class="min-w-0 flex-1">
                          <p class="font-medium text-gray-900 dark:text-gray-100 truncate">
                            {node.name}
                          </p>
                          <p class="text-xs text-gray-600 dark:text-gray-400 truncate">
                            {node.host}
                          </p>
                        </div>
                      </div>
                    </div>
                  </td>
                  <td class="align-top px-3 py-3">
                    <div class="flex flex-col gap-1">
                      <span class="text-xs text-gray-600 dark:text-gray-400">
                        {node.user ? `User: ${node.user}` : `Token: ${node.tokenName}`}
                      </span>
                      <Show when={node.source === 'agent'}>
                        <span class="inline-flex items-center gap-1 text-[0.65rem] px-1.5 py-0.5 bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300 rounded w-fit">
                          <span class="h-1.5 w-1.5 rounded-full bg-purple-500"></span>
                          Agent
                        </span>
                      </Show>
                      <Show when={node.source === 'script' || (!node.source && node.tokenName)}>
                        <span class="text-[0.65rem] px-1.5 py-0.5 bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 rounded w-fit">
                          API only
                        </span>
                      </Show>
                    </div>
                  </td>
                  <td class="align-top px-3 py-3">
                    <div class="flex flex-wrap gap-1">
                      {node.type === 'pmg' &&
                        (node as NodeConfig & { monitorMailStats?: boolean }).monitorMailStats && (
                          <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                            Mail stats
                          </span>
                        )}
                      {node.type === 'pmg' &&
                        (node as NodeConfig & { monitorQueues?: boolean }).monitorQueues && (
                          <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                            Queues
                          </span>
                        )}
                      {node.type === 'pmg' &&
                        (node as NodeConfig & { monitorQuarantine?: boolean }).monitorQuarantine && (
                          <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                            Quarantine
                          </span>
                        )}
                      {node.type === 'pmg' &&
                        (node as NodeConfig & { monitorDomainStats?: boolean }).monitorDomainStats && (
                          <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                            Domain stats
                          </span>
                        )}
                    </div>
                  </td>
                  <td class="align-top px-3 py-3 whitespace-nowrap">
                    <span class={`inline-flex items-center gap-2 text-xs font-medium ${statusMeta().labelClass}`}>
                      <span class={`h-2.5 w-2.5 rounded-full ${statusMeta().dotClass}`}></span>
                      {statusMeta().label}
                    </span>
                  </td>
                  <td class="align-top px-3 py-3">
                    <div class="flex items-center justify-end gap-1 sm:gap-2">
                      <button
                        type="button"
                        onClick={() => props.onTestConnection(node.id)}
                        class="p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100"
                        title="Test connection"
                      >
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                          <polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline>
                        </svg>
                      </button>
                      <button
                        type="button"
                        onClick={() => props.onEdit(node)}
                        class="p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100"
                        title="Edit node"
                      >
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                          <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"></path>
                          <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                        </svg>
                      </button>
                      <button
                        type="button"
                        onClick={() => props.onDelete(node)}
                        class="p-2 text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300"
                        title="Delete node"
                      >
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                          <polyline points="3 6 5 6 21 6"></polyline>
                          <path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"></path>
                        </svg>
                      </button>
                    </div>
                  </td>
                </tr>
              );
            }}
          </For>
        </tbody>
      </table>
    </Card>
  );
};
