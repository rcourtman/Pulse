import { Component, For, Show, createMemo } from 'solid-js';

import type { NodeConfig, NodeConfigWithStatus } from '@/types/nodes';
import type { PBSInstance, PMGInstance } from '@/types/api';
import type { Resource } from '@/types/resource';
import { unwrap } from 'solid-js/store';
import { Card } from '@/components/shared/Card';
import { StatusDot } from '@/components/shared/StatusDot';
import { getClusterEndpointPresentation } from '@/utils/clusterEndpointPresentation';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/shared/Table';
import {
  getSimpleStatusIndicator,
  getStatusIndicatorBadgeToneClasses,
  type StatusIndicator,
} from '@/utils/status';

interface PveNodesTableProps {
  nodes: NodeConfigWithStatus[];
  stateNodes: Resource[];
  globalTemperatureMonitoringEnabled?: boolean;
  onTestConnection: (nodeId: string) => void;
  onEdit: (node: NodeConfigWithStatus) => void;
  onDelete: (node: NodeConfigWithStatus) => void;
  onRefreshCluster?: (nodeId: string) => void;
}

const isTemperatureMonitoringEnabled = (
  node: NodeConfigWithStatus,
  globalEnabled: boolean,
): boolean => {
  // Check per-node setting first, fall back to global
  if (
    node.temperatureMonitoringEnabled !== undefined &&
    node.temperatureMonitoringEnabled !== null
  ) {
    return node.temperatureMonitoringEnabled;
  }
  return globalEnabled;
};

const resolveConfiguredNodeStatusIndicator = ({
  configuredStatus,
  liveStatus,
  connectionHealth,
}: {
  configuredStatus?: string | null;
  liveStatus?: string | null;
  connectionHealth?: string | null;
}): StatusIndicator => {
  if (
    connectionHealth === 'unhealthy' ||
    connectionHealth === 'error' ||
    liveStatus === 'offline' ||
    liveStatus === 'disconnected'
  ) {
    return getSimpleStatusIndicator('offline');
  }
  if (connectionHealth === 'degraded') {
    return getSimpleStatusIndicator('degraded');
  }
  if (liveStatus === 'online' || connectionHealth === 'healthy') {
    return getSimpleStatusIndicator('online');
  }

  switch (configuredStatus) {
    case 'connected':
      return getSimpleStatusIndicator('online');
    case 'pending':
      return getSimpleStatusIndicator('pending');
    case 'disconnected':
    case 'offline':
    case 'error':
      return getSimpleStatusIndicator('offline');
    default:
      return getSimpleStatusIndicator('unknown');
  }
};

const resolvePveStatusIndicator = (
  node: NodeConfigWithStatus,
  stateNodes: PveNodesTableProps['stateNodes'],
): StatusIndicator => {
  const stateNode = stateNodes.find((n) => n.platformId === node.name || n.name === node.name);
  const pd = stateNode?.platformData
    ? (unwrap(stateNode.platformData) as Record<string, unknown>)
    : undefined;
  return resolveConfiguredNodeStatusIndicator({
    configuredStatus: node.status,
    liveStatus: stateNode?.status,
    connectionHealth: pd?.connectionHealth as string | undefined,
  });
};

export const PveNodesTable: Component<PveNodesTableProps> = (props) => {
  return (
    <Card padding="none" tone="card" class="rounded-md">
      <div class="overflow-auto max-h-[600px]">
        <Table class="min-w-[max-content] w-full divide-y divide-border text-sm">
          <TableHeader class="bg-surface-alt">
            <TableRow>
              <TableHead class="py-2 pl-4 pr-3 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                Node
              </TableHead>
              <TableHead class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                Credentials
              </TableHead>
              <TableHead class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                Capabilities
              </TableHead>
              <TableHead class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                Status
              </TableHead>
              <TableHead class="px-3 py-2 text-right text-xs font-semibold uppercase tracking-wide text-muted">
                Actions
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody class="divide-y divide-border bg-surface">
            <For each={props.nodes}>
              {(node) => {
                const statusIndicator = createMemo(() =>
                  resolvePveStatusIndicator(node, props.stateNodes),
                );
                const clusterEndpoints = createMemo(() =>
                  'clusterEndpoints' in node && node.clusterEndpoints ? node.clusterEndpoints : [],
                );
                const clusterName = createMemo(() =>
                  'clusterName' in node && node.clusterName ? node.clusterName : 'Unknown',
                );
                return (
                  <TableRow class="even:bg-surface-alt hover:bg-blue-50 dark:hover:bg-blue-900 transition-colors">
                    <TableCell class="align-top py-3 pl-4 pr-3">
                      <div class="min-w-0 space-y-1">
                        <div class="flex items-start gap-3">
                          <StatusDot
                            variant={statusIndicator().variant}
                            size="md"
                            ariaHidden={true}
                            class="mt-1.5"
                          />
                          <div class="min-w-0 flex-1">
                            <p class="font-medium text-base-content truncate">{node.name}</p>
                            <p class="text-xs text-muted truncate">{node.host}</p>
                          </div>
                        </div>
                        <Show when={node.type === 'pve' && 'isCluster' in node && node.isCluster}>
                          <div class="rounded-md border border-border bg-surface-alt px-3 py-2 space-y-2">
                            <div class="flex items-center gap-2 text-xs font-semibold text-base-content">
                              <span>{clusterName()} Cluster</span>
                              <span class="ml-auto text-[0.65rem] font-normal text-slate-500">
                                {clusterEndpoints().length} nodes
                              </span>
                            </div>
                            <Show when={clusterEndpoints().length > 0}>
                              <div class="flex flex-col gap-2">
                                <For each={clusterEndpoints()}>
                                  {(endpoint) => {
                                    const endpointPresentation =
                                      getClusterEndpointPresentation(endpoint);

                                    return (
                                      <div
                                        class={`rounded border px-3 py-2 text-[0.7rem] ${endpointPresentation.panelClass}`}
                                      >
                                        <div class="flex items-center gap-2 mb-1">
                                          <span class="font-semibold">{endpoint.nodeName}</span>
                                          <span class="text-[0.65rem] opacity-75">
                                            {endpoint.ip}
                                          </span>
                                        </div>
                                        <div class="flex flex-col gap-0.5 text-[0.65rem] opacity-90">
                                          <div class="flex items-center gap-1.5">
                                            <span class="w-16 font-medium">Proxmox:</span>
                                            <span>{endpointPresentation.proxmoxLabel}</span>
                                          </div>
                                          <div class="flex items-center gap-1.5">
                                            <span class="w-16 font-medium">Pulse:</span>
                                            <span>{endpointPresentation.pulseLabel}</span>
                                          </div>
                                          <Show
                                            when={
                                              endpointPresentation.pulseStatus === 'unreachable' &&
                                              endpoint.pulseError
                                            }
                                          >
                                            <div class="mt-1 pt-1 border-t border-current opacity-20">
                                              <span class="font-medium">Error: </span>
                                              <span class="opacity-80">{endpoint.pulseError}</span>
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
                              <p class="flex items-center gap-1 text-[0.7rem] text-muted">
                                <svg
                                  width="14"
                                  height="14"
                                  viewBox="0 0 24 24"
                                  fill="none"
                                  stroke="currentColor"
                                  stroke-width="2"
                                >
                                  <path d="M5 12h14M12 5l7 7-7 7" />
                                </svg>
                                Automatic failover enabled
                              </p>
                              <Show when={props.onRefreshCluster}>
                                <button
                                  type="button"
                                  onClick={() => props.onRefreshCluster?.(node.id)}
                                  class="flex min-h-10 sm:min-h-9 items-center gap-1 px-2.5 py-1.5 text-xs font-medium hover:text-muted bg-surface border border-border rounded hover:bg-surface-hover transition-colors"
                                  title="Re-detect cluster membership (use if nodes were added to the Proxmox cluster)"
                                >
                                  <svg
                                    width="12"
                                    height="12"
                                    viewBox="0 0 24 24"
                                    fill="none"
                                    stroke="currentColor"
                                    stroke-width="2"
                                  >
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
                    </TableCell>
                    <TableCell class="align-top px-3 py-3">
                      <div class="flex flex-col gap-1">
                        <span class="text-xs text-muted">
                          {node.user ? `User: ${node.user}` : `Token: ${node.tokenName}`}
                        </span>
                        <Show when={node.source === 'agent'}>
                          <span class="inline-flex items-center gap-1 text-[0.65rem] px-1.5 py-0.5 bg-surface-hover text-base-content rounded w-fit">
                            <span class="h-1.5 w-1.5 rounded-full bg-slate-500"></span>
                            Agent-linked
                          </span>
                        </Show>
                        <Show when={node.source === 'script' || (!node.source && node.tokenName)}>
                          <span class="text-[0.65rem] px-1.5 py-0.5 bg-surface-alt text-muted rounded w-fit">
                            Direct-linked
                          </span>
                        </Show>
                      </div>
                    </TableCell>
                    <TableCell class="align-top px-3 py-3">
                      <div class="flex flex-wrap gap-1">
                        {node.type === 'pve' && 'monitorVMs' in node && node.monitorVMs && (
                          <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                            VMs
                          </span>
                        )}
                        {node.type === 'pve' &&
                          'monitorContainers' in node &&
                          node.monitorContainers && (
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
                            Recovery
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
                          isTemperatureMonitoringEnabled(
                            node,
                            props.globalTemperatureMonitoringEnabled ?? true,
                          ) && (
                            <span class="text-xs px-2 py-1 bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 rounded">
                              Temperature
                            </span>
                          )}
                      </div>
                    </TableCell>
                    <TableCell class="align-top px-3 py-3 whitespace-nowrap">
                      <span
                        class={`inline-flex items-center gap-2 text-xs font-medium ${getStatusIndicatorBadgeToneClasses(statusIndicator().variant)}`}
                      >
                        <StatusDot variant={statusIndicator().variant} size="sm" ariaHidden={true} />
                        {statusIndicator().label}
                      </span>
                    </TableCell>
                    <TableCell class="align-top px-3 py-3">
                      <div class="flex items-center justify-end gap-1 sm:gap-2">
                        <button
                          type="button"
                          onClick={() => props.onTestConnection(node.id)}
                          class="min-h-10 sm:min-h-9 min-w-10 sm:min-w-9 p-2.5 text-muted hover:text-base-content"
                          title="Test connection"
                        >
                          <svg
                            width="16"
                            height="16"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                          >
                            <polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline>
                          </svg>
                        </button>
                        <button
                          type="button"
                          onClick={() => props.onEdit(node)}
                          class="min-h-10 sm:min-h-9 min-w-10 sm:min-w-9 p-2.5 text-muted hover:text-base-content"
                          title="Edit node"
                        >
                          <svg
                            width="16"
                            height="16"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                          >
                            <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"></path>
                            <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                          </svg>
                        </button>
                        <button
                          type="button"
                          onClick={() => props.onDelete(node)}
                          class="min-h-10 sm:min-h-9 min-w-10 sm:min-w-9 p-2.5 text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300"
                          title="Delete node"
                        >
                          <svg
                            width="16"
                            height="16"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                          >
                            <polyline points="3 6 5 6 21 6"></polyline>
                            <path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"></path>
                          </svg>
                        </button>
                      </div>
                    </TableCell>
                  </TableRow>
                );
              }}
            </For>
          </TableBody>
        </Table>
      </div>
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

const resolvePbsStatusIndicator = (
  node: NodeConfigWithStatus,
  statePbs: PbsNodesTableProps['statePbs'],
): StatusIndicator => {
  const statePBS = statePbs.find((p) => p.name === node.name);
  return resolveConfiguredNodeStatusIndicator({
    configuredStatus: node.status,
    liveStatus: statePBS?.status,
    connectionHealth: statePBS?.connectionHealth,
  });
};

export const PbsNodesTable: Component<PbsNodesTableProps> = (props) => {
  return (
    <Card padding="none" tone="card" class="rounded-md">
      <div class="overflow-x-auto">
        <Table class="min-w-[max-content] divide-y divide-border text-sm">
          <TableHeader class="bg-surface-alt">
            <TableRow>
              <TableHead class="py-2 pl-4 pr-3 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                Node
              </TableHead>
              <TableHead class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                Credentials
              </TableHead>
              <TableHead class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                Capabilities
              </TableHead>
              <TableHead class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                Status
              </TableHead>
              <TableHead class="px-3 py-2 text-right text-xs font-semibold uppercase tracking-wide text-muted">
                Actions
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody class="divide-y divide-border bg-surface">
            <For each={props.nodes}>
              {(node) => {
                const statusIndicator = createMemo(() =>
                  resolvePbsStatusIndicator(node, props.statePbs),
                );
                return (
                  <TableRow class="even:bg-surface-alt hover:bg-blue-50 dark:hover:bg-blue-900 transition-colors">
                    <TableCell class="align-top py-3 pl-4 pr-3">
                      <div class="min-w-0 space-y-1">
                        <div class="flex items-start gap-3">
                          <StatusDot
                            variant={statusIndicator().variant}
                            size="md"
                            ariaHidden={true}
                            class="mt-1.5"
                          />
                          <div class="min-w-0 flex-1">
                            <p class="font-medium text-base-content truncate">{node.name}</p>
                            <p class="text-xs text-muted truncate">{node.host}</p>
                          </div>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell class="align-top px-3 py-3">
                      <div class="flex flex-col gap-1">
                        <span class="text-xs text-muted">
                          {node.user ? `User: ${node.user}` : `Token: ${node.tokenName}`}
                        </span>
                        <Show when={node.source === 'agent'}>
                          <span class="inline-flex items-center gap-1 text-[0.65rem] px-1.5 py-0.5 bg-surface-hover text-base-content rounded w-fit">
                            <span class="h-1.5 w-1.5 rounded-full bg-slate-500"></span>
                            Agent-linked
                          </span>
                        </Show>
                        <Show when={node.source === 'script' || (!node.source && node.tokenName)}>
                          <span class="text-[0.65rem] px-1.5 py-0.5 bg-surface-alt text-muted rounded w-fit">
                            Direct-linked
                          </span>
                        </Show>
                      </div>
                    </TableCell>
                    <TableCell class="align-top px-3 py-3">
                      <div class="flex flex-wrap gap-1">
                        {node.type === 'pbs' &&
                          'monitorDatastores' in node &&
                          node.monitorDatastores && (
                            <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                              Datastores
                            </span>
                          )}
                        {node.type === 'pbs' &&
                          'monitorSyncJobs' in node &&
                          node.monitorSyncJobs && (
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
                        {node.type === 'pbs' &&
                          'monitorPruneJobs' in node &&
                          node.monitorPruneJobs && (
                            <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                              Prune Jobs
                            </span>
                          )}
                        {node.type === 'pbs' &&
                          'monitorGarbageJobs' in node &&
                          (node as NodeConfig & { monitorGarbageJobs?: boolean })
                            .monitorGarbageJobs && (
                            <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                              Garbage Collection
                            </span>
                          )}
                        {node.type === 'pbs' &&
                          isTemperatureMonitoringEnabled(
                            node,
                            props.globalTemperatureMonitoringEnabled ?? true,
                          ) && (
                            <span class="text-xs px-2 py-1 bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300 rounded">
                              Temperature
                            </span>
                          )}
                      </div>
                    </TableCell>
                    <TableCell class="align-top px-3 py-3 whitespace-nowrap">
                      <span
                        class={`inline-flex items-center gap-2 text-xs font-medium ${getStatusIndicatorBadgeToneClasses(statusIndicator().variant)}`}
                      >
                        <StatusDot variant={statusIndicator().variant} size="sm" ariaHidden={true} />
                        {statusIndicator().label}
                      </span>
                    </TableCell>
                    <TableCell class="align-top px-3 py-3">
                      <div class="flex items-center justify-end gap-1 sm:gap-2">
                        <button
                          type="button"
                          onClick={() => props.onTestConnection(node.id)}
                          class="min-h-10 sm:min-h-9 min-w-10 sm:min-w-9 p-2.5 text-muted hover:text-base-content"
                          title="Test connection"
                        >
                          <svg
                            width="16"
                            height="16"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                          >
                            <polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline>
                          </svg>
                        </button>
                        <button
                          type="button"
                          onClick={() => props.onEdit(node)}
                          class="min-h-10 sm:min-h-9 min-w-10 sm:min-w-9 p-2.5 text-muted hover:text-base-content"
                          title="Edit node"
                        >
                          <svg
                            width="16"
                            height="16"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                          >
                            <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"></path>
                            <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                          </svg>
                        </button>
                        <button
                          type="button"
                          onClick={() => props.onDelete(node)}
                          class="min-h-10 sm:min-h-9 min-w-10 sm:min-w-9 p-2.5 text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300"
                          title="Delete node"
                        >
                          <svg
                            width="16"
                            height="16"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                          >
                            <polyline points="3 6 5 6 21 6"></polyline>
                            <path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"></path>
                          </svg>
                        </button>
                      </div>
                    </TableCell>
                  </TableRow>
                );
              }}
            </For>
          </TableBody>
        </Table>
      </div>
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

const resolvePmgStatusIndicator = (
  node: NodeConfigWithStatus,
  statePmg: PmgNodesTableProps['statePmg'],
): StatusIndicator => {
  const statePMG = statePmg.find((p) => p.name === node.name);
  return resolveConfiguredNodeStatusIndicator({
    configuredStatus: node.status,
    liveStatus: statePMG?.status,
    connectionHealth: statePMG?.connectionHealth,
  });
};

export const PmgNodesTable: Component<PmgNodesTableProps> = (props) => {
  return (
    <Card padding="none" tone="card" class="rounded-md">
      <div class="overflow-x-auto">
        <Table class="min-w-[max-content] divide-y divide-border text-sm">
          <TableHeader class="bg-surface-alt">
            <TableRow>
              <TableHead class="py-2 pl-4 pr-3 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                Node
              </TableHead>
              <TableHead class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                Credentials
              </TableHead>
              <TableHead class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                Capabilities
              </TableHead>
              <TableHead class="px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                Status
              </TableHead>
              <TableHead class="px-3 py-2 text-right text-xs font-semibold uppercase tracking-wide text-muted">
                Actions
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody class="divide-y divide-border bg-surface">
            <For each={props.nodes}>
              {(node) => {
                const statusIndicator = createMemo(() =>
                  resolvePmgStatusIndicator(node, props.statePmg),
                );
                return (
                  <TableRow class="even:bg-surface-alt hover:bg-blue-50 dark:hover:bg-blue-900 transition-colors">
                    <TableCell class="align-top py-3 pl-4 pr-3">
                      <div class="min-w-0 space-y-1">
                        <div class="flex items-start gap-3">
                          <StatusDot
                            variant={statusIndicator().variant}
                            size="md"
                            ariaHidden={true}
                            class="mt-1.5"
                          />
                          <div class="min-w-0 flex-1">
                            <p class="font-medium text-base-content truncate">{node.name}</p>
                            <p class="text-xs text-muted truncate">{node.host}</p>
                          </div>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell class="align-top px-3 py-3">
                      <div class="flex flex-col gap-1">
                        <span class="text-xs text-muted">
                          {node.user ? `User: ${node.user}` : `Token: ${node.tokenName}`}
                        </span>
                        <Show when={node.source === 'agent'}>
                          <span class="inline-flex items-center gap-1 text-[0.65rem] px-1.5 py-0.5 bg-surface-hover text-base-content rounded w-fit">
                            <span class="h-1.5 w-1.5 rounded-full bg-slate-500"></span>
                            Agent-linked
                          </span>
                        </Show>
                        <Show when={node.source === 'script' || (!node.source && node.tokenName)}>
                          <span class="text-[0.65rem] px-1.5 py-0.5 bg-surface-alt text-muted rounded w-fit">
                            Direct-linked
                          </span>
                        </Show>
                      </div>
                    </TableCell>
                    <TableCell class="align-top px-3 py-3">
                      <div class="flex flex-wrap gap-1">
                        {node.type === 'pmg' &&
                          (node as NodeConfig & { monitorMailStats?: boolean })
                            .monitorMailStats && (
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
                          (node as NodeConfig & { monitorQuarantine?: boolean })
                            .monitorQuarantine && (
                            <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                              Quarantine
                            </span>
                          )}
                        {node.type === 'pmg' &&
                          (node as NodeConfig & { monitorDomainStats?: boolean })
                            .monitorDomainStats && (
                            <span class="text-xs px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                              Domain stats
                            </span>
                          )}
                      </div>
                    </TableCell>
                    <TableCell class="align-top px-3 py-3 whitespace-nowrap">
                      <span
                        class={`inline-flex items-center gap-2 text-xs font-medium ${getStatusIndicatorBadgeToneClasses(statusIndicator().variant)}`}
                      >
                        <StatusDot variant={statusIndicator().variant} size="sm" ariaHidden={true} />
                        {statusIndicator().label}
                      </span>
                    </TableCell>
                    <TableCell class="align-top px-3 py-3">
                      <div class="flex items-center justify-end gap-1 sm:gap-2">
                        <button
                          type="button"
                          onClick={() => props.onTestConnection(node.id)}
                          class="min-h-10 sm:min-h-9 min-w-10 sm:min-w-9 p-2.5 text-muted hover:text-base-content"
                          title="Test connection"
                        >
                          <svg
                            width="16"
                            height="16"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                          >
                            <polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline>
                          </svg>
                        </button>
                        <button
                          type="button"
                          onClick={() => props.onEdit(node)}
                          class="min-h-10 sm:min-h-9 min-w-10 sm:min-w-9 p-2.5 text-muted hover:text-base-content"
                          title="Edit node"
                        >
                          <svg
                            width="16"
                            height="16"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                          >
                            <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"></path>
                            <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                          </svg>
                        </button>
                        <button
                          type="button"
                          onClick={() => props.onDelete(node)}
                          class="min-h-10 sm:min-h-9 min-w-10 sm:min-w-9 p-2.5 text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300"
                          title="Delete node"
                        >
                          <svg
                            width="16"
                            height="16"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            stroke-width="2"
                          >
                            <polyline points="3 6 5 6 21 6"></polyline>
                            <path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"></path>
                          </svg>
                        </button>
                      </div>
                    </TableCell>
                  </TableRow>
                );
              }}
            </For>
          </TableBody>
        </Table>
      </div>
    </Card>
  );
};
