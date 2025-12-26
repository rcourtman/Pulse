import { Component, For, Show, createMemo, createSignal } from 'solid-js';
import type { Node, VM, Container, Storage, PBSInstance } from '@/types/api';
import { formatBytes, formatUptime } from '@/utils/format';
import { useWebSocket } from '@/App';
import { getAlertStyles } from '@/utils/alerts';
import { Card } from '@/components/shared/Card';
import { getNodeDisplayName, hasAlternateDisplayName } from '@/utils/nodes';
import { getCpuTemperature } from '@/utils/temperature';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { buildMetricKey } from '@/utils/metricsKeys';
import { StatusDot } from '@/components/shared/StatusDot';
import { getNodeStatusIndicator, getPBSStatusIndicator } from '@/utils/status';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { StackedMemoryBar } from '@/components/Dashboard/StackedMemoryBar';
import { StackedDiskBar } from '@/components/Dashboard/StackedDiskBar';
import { EnhancedCPUBar } from '@/components/Dashboard/EnhancedCPUBar';
import { TemperatureGauge } from '@/components/shared/TemperatureGauge';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { useMetricsViewMode } from '@/stores/metricsViewMode';

interface NodeSummaryTableProps {
  nodes: Node[];
  pbsInstances?: PBSInstance[];
  vms?: VM[];
  containers?: Container[];
  storage?: Storage[];
  backupCounts?: Record<string, number>;
  currentTab: 'dashboard' | 'storage' | 'backups';
  selectedNode: string | null;
  globalTemperatureMonitoringEnabled?: boolean;
  onNodeClick: (nodeId: string, nodeType: 'pve' | 'pbs') => void;
}

export const NodeSummaryTable: Component<NodeSummaryTableProps> = (props) => {
  const { activeAlerts, state } = useWebSocket();
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = createMemo(() => alertsActivation.activationState() === 'active');
  const { isMobile } = useBreakpoint();
  const { viewMode } = useMetricsViewMode();

  const isTemperatureMonitoringEnabled = (node: Node): boolean => {
    const globalEnabled = props.globalTemperatureMonitoringEnabled ?? true;
    if (node.temperatureMonitoringEnabled !== undefined && node.temperatureMonitoringEnabled !== null) {
      return node.temperatureMonitoringEnabled;
    }
    return globalEnabled;
  };

  type TableItem = Node | PBSInstance;

  const isPVE = (item: TableItem): item is Node => {
    return (item as Node).pveVersion !== undefined;
  };

  type CountSortKey = 'vmCount' | 'containerCount' | 'storageCount' | 'diskCount' | 'backupCount';
  type SortKey =
    | 'default'
    | 'name'
    | 'uptime'
    | 'cpu'
    | 'memory'
    | 'disk'
    | 'temperature'
    | CountSortKey;

  const [sortKey, setSortKey] = createSignal<SortKey>('default');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');

  const hasAnyTemperatureData = createMemo(() => {
    return (
      props.nodes?.some(
        (node) => node.temperature?.available || isTemperatureMonitoringEnabled(node),
      ) || false
    );
  });

  const nodeKey = (instance?: string, nodeName?: string) => `${instance ?? ''}::${nodeName ?? ''}`;

  const vmCountsByNode = createMemo<Record<string, number>>(() => {
    const counts: Record<string, number> = {};
    (props.vms ?? []).forEach((vm) => {
      const key = nodeKey(vm.instance, vm.node);
      counts[key] = (counts[key] || 0) + 1;
    });
    return counts;
  });

  const containerCountsByNode = createMemo<Record<string, number>>(() => {
    const counts: Record<string, number> = {};
    (props.containers ?? []).forEach((ct) => {
      const key = nodeKey(ct.instance, ct.node);
      counts[key] = (counts[key] || 0) + 1;
    });
    return counts;
  });

  const storageCountsByNode = createMemo<Record<string, number>>(() => {
    const counts: Record<string, number> = {};
    (props.storage ?? []).forEach((storage) => {
      const key = nodeKey(storage.instance, storage.node);
      counts[key] = (counts[key] || 0) + 1;
    });
    return counts;
  });

  const diskCountsByNode = createMemo<Record<string, number>>(() => {
    const counts: Record<string, number> = {};
    (state.physicalDisks ?? []).forEach((disk) => {
      const key = nodeKey(disk.instance, disk.node);
      counts[key] = (counts[key] || 0) + 1;
    });
    return counts;
  });

  const getCpuTemperatureValue = (item: TableItem) => {
    if (!isPVE(item)) return null;
    const node = item;
    const value = getCpuTemperature(node.temperature);
    return value !== null ? Math.round(value) : null;
  };

  const getDefaultSortDirection = (key: Exclude<SortKey, 'default'>) => {
    switch (key) {
      case 'name':
        return 'asc';
      default:
        return 'desc';
    }
  };

  const handleSort = (key: Exclude<SortKey, 'default'>) => {
    if (sortKey() === key) {
      if (sortDirection() === 'asc') {
        setSortDirection('desc');
      } else {
        setSortKey('default');
        setSortDirection('asc');
      }
    } else {
      setSortKey(key);
      setSortDirection(getDefaultSortDirection(key));
    }
  };

  const isItemOnline = (item: TableItem) => {
    if (isPVE(item)) {
      const node = item;
      return node.status === 'online' && (node.uptime || 0) > 0;
    }
    const pbs = item;
    return pbs.status === 'healthy' || pbs.status === 'online';
  };

  const getPbsTotals = (pbs: PBSInstance) => {
    return (pbs.datastores ?? []).reduce(
      (acc, ds) => {
        acc.used += ds.used || 0;
        acc.total += ds.total || 0;
        return acc;
      },
      { used: 0, total: 0 },
    );
  };

  const getCpuPercent = (item: TableItem) => {
    if (isPVE(item)) {
      const node = item;
      return Math.round((node.cpu || 0) * 100);
    }
    const pbs = item;
    return Math.round(pbs.cpu || 0);
  };

  const getMemoryPercent = (item: TableItem) => {
    if (isPVE(item)) {
      const node = item;
      return Math.round(node.memory?.usage || 0);
    }
    const pbs = item;
    if (!pbs.memoryTotal) return 0;
    return Math.round((pbs.memoryUsed / pbs.memoryTotal) * 100);
  };

  const getDiskPercent = (item: TableItem) => {
    if (isPVE(item)) {
      const node = item;
      if (!node.disk || node.disk.total === 0) return 0;
      return Math.round((node.disk.used / node.disk.total) * 100);
    }
    const pbs = item;
    const totals = getPbsTotals(pbs);
    if (totals.total === 0) return 0;
    return Math.round((totals.used / totals.total) * 100);
  };

  const getDiskSublabel = (item: TableItem) => {
    if (isPVE(item)) {
      const node = item;
      if (!node.disk) return undefined;
      return `${formatBytes(node.disk.used, 0)}/${formatBytes(node.disk.total, 0)}`;
    }
    const pbs = item;
    if (!pbs.datastores || pbs.datastores.length === 0) return undefined;
    const totals = getPbsTotals(pbs);
    return `${formatBytes(totals.used, 0)}/${formatBytes(totals.total, 0)}`;
  };

  const getTemperatureValue = (item: TableItem) => {
    return getCpuTemperatureValue(item);
  };

  const getCountValue = (item: TableItem, key: CountSortKey): number | null => {
    if (!isPVE(item)) {
      const pbs = item;
      if (key === 'backupCount') {
        return props.backupCounts?.[pbs.name] ?? 0;
      }
      return null;
    }

    const node = item;
    const keyId = nodeKey(node.instance, node.name);

    switch (key) {
      case 'vmCount':
        return vmCountsByNode()[keyId] ?? 0;
      case 'containerCount':
        return containerCountsByNode()[keyId] ?? 0;
      case 'storageCount':
        return storageCountsByNode()[keyId] ?? 0;
      case 'diskCount':
        return diskCountsByNode()[keyId] ?? 0;
      case 'backupCount':
        return props.backupCounts?.[node.id] ?? 0;
      default:
        return null;
    }
  };

  const getSortValue = (item: TableItem, key: SortKey): number | string | null => {
    switch (key) {
      case 'name':
        return isPVE(item)
          ? getNodeDisplayName(item)
          : item.name;
      case 'uptime':
        return isPVE(item)
          ? item.uptime ?? 0
          : item.uptime ?? 0;
      case 'cpu':
        return getCpuPercent(item);
      case 'memory':
        return getMemoryPercent(item);
      case 'disk':
        return getDiskPercent(item);
      case 'temperature':
        return getTemperatureValue(item);
      case 'vmCount':
      case 'containerCount':
      case 'storageCount':
      case 'diskCount':
      case 'backupCount':
        return getCountValue(item, key);
      default:
        return null;
    }
  };

  const defaultComparison = (a: TableItem, b: TableItem) => {
    const aIsPVE = isPVE(a);
    const bIsPVE = isPVE(b);
    if (aIsPVE !== bIsPVE) return aIsPVE ? -1 : 1;

    const aOnline = isItemOnline(a);
    const bOnline = isItemOnline(b);
    if (aOnline !== bOnline) return aOnline ? -1 : 1;

    const aName = aIsPVE
      ? getNodeDisplayName(a)
      : a.name;
    const bName = bIsPVE
      ? getNodeDisplayName(b)
      : b.name;

    return aName.localeCompare(bName);
  };

  const compareValues = (valueA: number | string | null, valueB: number | string | null) => {
    const aEmpty =
      valueA === null || valueA === undefined || (typeof valueA === 'number' && Number.isNaN(valueA));
    const bEmpty =
      valueB === null || valueB === undefined || (typeof valueB === 'number' && Number.isNaN(valueB));

    if (aEmpty && bEmpty) return 0;
    if (aEmpty) return 1;
    if (bEmpty) return -1;

    if (typeof valueA === 'number' && typeof valueB === 'number') {
      if (valueA === valueB) return 0;
      return valueA < valueB ? -1 : 1;
    }

    const aStr = String(valueA).toLowerCase();
    const bStr = String(valueB).toLowerCase();

    if (aStr === bStr) return 0;
    return aStr < bStr ? -1 : 1;
  };

  const sortedItems = createMemo(() => {
    const items: TableItem[] = [];

    if (props.nodes) items.push(...props.nodes);
    if (props.pbsInstances) items.push(...props.pbsInstances);

    const key = sortKey();
    const direction = sortDirection();

    return items.sort((a, b) => {
      if (key === 'default') {
        return defaultComparison(a, b);
      }

      const valueA = getSortValue(a, key);
      const valueB = getSortValue(b, key);
      const comparison = compareValues(valueA, valueB);

      if (comparison !== 0) {
        return direction === 'asc' ? comparison : -comparison;
      }

      return defaultComparison(a, b);
    });
  });

  const renderSortIndicator = (key: SortKey) => {
    if (sortKey() !== key) return null;
    return sortDirection() === 'asc' ? '▲' : '▼';
  };

  const thClassBase = "px-2 py-1 text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 whitespace-nowrap";
  const thClass = `${thClassBase} text-center`;

  // Cell class constants for consistency
  const tdClass = "px-2 py-1 align-middle";
  const metricColumnStyle = { width: "200px", "min-width": "200px", "max-width": "200px" } as const;

  return (
    <Card padding="none" tone="glass" class="mb-4 overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full border-collapse whitespace-nowrap" style={{ "min-width": isMobile() ? "100%" : "800px" }}>
          <thead>
            <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-700">
              <th
                class={`${thClassBase} text-left pl-3`}
                onClick={() => handleSort('name')}
              >
                {props.currentTab === 'backups' ? 'Node / PBS' : 'Node'} {renderSortIndicator('name')}
              </th>
              <th class={thClass} style={{ "min-width": '80px' }} onClick={() => handleSort('uptime')}>
                Uptime {renderSortIndicator('uptime')}
              </th>
              <th class={thClass} style={isMobile() ? { "min-width": "60px" } : { width: "200px", "min-width": "200px", "max-width": "200px" }} onClick={() => handleSort('cpu')}>
                CPU {renderSortIndicator('cpu')}
              </th>
              <th class={thClass} style={isMobile() ? { "min-width": "60px" } : { width: "200px", "min-width": "200px", "max-width": "200px" }} onClick={() => handleSort('memory')}>
                Memory {renderSortIndicator('memory')}
              </th>
              <th class={thClass} style={isMobile() ? { "min-width": "60px" } : { width: "200px", "min-width": "200px", "max-width": "200px" }} onClick={() => handleSort('disk')}>
                Disk {renderSortIndicator('disk')}
              </th>
              <Show when={hasAnyTemperatureData()}>
                <th class={thClass} style={{ "min-width": '60px' }} onClick={() => handleSort('temperature')}>
                  Temp {renderSortIndicator('temperature')}
                </th>
              </Show>
              <Show when={props.currentTab === 'dashboard'}>
                <th class={thClass} style={{ "min-width": '50px' }} onClick={() => handleSort('vmCount')}>
                  VMs {renderSortIndicator('vmCount')}
                </th>
                <th class={thClass} style={{ "min-width": '50px' }} onClick={() => handleSort('containerCount')}>
                  CTs {renderSortIndicator('containerCount')}
                </th>
              </Show>
              <Show when={props.currentTab === 'storage'}>
                <th class={thClass} style={{ "min-width": '70px' }} onClick={() => handleSort('storageCount')}>
                  Storage {renderSortIndicator('storageCount')}
                </th>
                <th class={thClass} style={{ "min-width": '60px' }} onClick={() => handleSort('diskCount')}>
                  Disks {renderSortIndicator('diskCount')}
                </th>
              </Show>
              <Show when={props.currentTab === 'backups'}>
                <th class={thClass} style={{ "min-width": '70px' }} onClick={() => handleSort('backupCount')}>
                  Backups {renderSortIndicator('backupCount')}
                </th>
              </Show>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
            <For each={sortedItems()}>
              {(item) => {
                const isPVEItem = isPVE(item);
                const isPBSItem = !isPVEItem;
                const node = isPVEItem ? (item as Node) : null;
                const pbs = isPBSItem ? (item as PBSInstance) : null;

                const online = isItemOnline(item);
                const statusIndicator = createMemo(() =>
                  isPVEItem ? getNodeStatusIndicator(node as Node) : getPBSStatusIndicator(pbs as PBSInstance),
                );
                const cpuPercentValue = getCpuPercent(item);
                const memoryPercentValue = getMemoryPercent(item);
                const diskPercentValue = getDiskPercent(item);
                const diskSublabel = getDiskSublabel(item);
                const cpuTemperatureValue = getCpuTemperatureValue(item);
                const uptimeValue = isPVEItem ? node?.uptime ?? 0 : isPBSItem ? pbs?.uptime ?? 0 : 0;
                const displayName = () => {
                  if (isPVEItem) return getNodeDisplayName(node as Node);
                  return (pbs as PBSInstance).name;
                };
                const showActualName = () => isPVEItem && hasAlternateDisplayName(node as Node);

                const nodeId = isPVEItem ? node!.id : pbs!.name;
                const isSelected = () => props.selectedNode === nodeId;
                const resourceId = isPVEItem ? node!.id || node!.name : pbs!.id || pbs!.name;
                const metricsKey = buildMetricKey('node', resourceId);
                const alertStyles = createMemo(() =>
                  getAlertStyles(resourceId, activeAlerts, alertsEnabled()),
                );
                const showAlertHighlight = createMemo(
                  () => alertStyles().hasUnacknowledgedAlert && online,
                );

                const rowStyle = createMemo(() => {
                  const styles: Record<string, string> = {};
                  const shadows: string[] = [];

                  if (showAlertHighlight()) {
                    const color = alertStyles().severity === 'critical' ? '#ef4444' : '#eab308';
                    shadows.push(`inset 4px 0 0 0 ${color}`);
                  }

                  if (isSelected()) {
                    shadows.push('0 0 0 1px rgba(59, 130, 246, 0.5)');
                    shadows.push('0 2px 4px -1px rgba(0, 0, 0, 0.1)');
                  }

                  if (shadows.length > 0) {
                    styles['box-shadow'] = shadows.join(', ');
                  }

                  return styles;
                });

                const rowClass = createMemo(() => {
                  const baseHover = 'cursor-pointer transition-all duration-200 relative hover:shadow-sm group';

                  if (isSelected()) {
                    return `cursor-pointer transition-all duration-200 relative hover:shadow-sm z-10 group bg-blue-50 dark:bg-blue-900/20`;
                  }

                  if (showAlertHighlight()) {
                    const alertBg = alertStyles().severity === 'critical'
                      ? 'bg-red-50 dark:bg-red-950/30'
                      : 'bg-yellow-50 dark:bg-yellow-950/20';
                    return `cursor-pointer transition-all duration-200 relative hover:shadow-sm group ${alertBg}`;
                  }

                  let className = baseHover;

                  if (props.selectedNode && props.selectedNode !== nodeId) {
                    className += ' opacity-50 hover:opacity-80';
                  }

                  if (!online) {
                    className += ' opacity-60';
                  }

                  return className;
                });

                return (
                  <tr
                    class={rowClass()}
                    style={{ ...rowStyle(), height: '29px', 'max-height': '29px' }}
                    onClick={() => props.onNodeClick(nodeId, isPVEItem ? 'pve' : 'pbs')}
                  >
                    {/* Name */}
                    <td class={`pr-2 py-1 align-middle ${showAlertHighlight() ? 'pl-4' : 'pl-3'}`}>
                      <div class="flex items-center gap-1.5">
                        <StatusDot
                          variant={statusIndicator().variant}
                          title={statusIndicator().label}
                          ariaLabel={statusIndicator().label}
                          size="xs"
                        />
                        <a
                          href={
                            isPVEItem
                              ? node!.guestURL || node!.host || `https://${node!.name}:8006`
                              : pbs!.guestURL || pbs!.host || `https://${pbs!.name}:8007`
                          }
                          target="_blank"
                          onClick={(e) => e.stopPropagation()}
                          class="font-medium text-[11px] text-gray-900 dark:text-gray-100 hover:text-blue-600 dark:hover:text-blue-400 whitespace-nowrap"
                          title={displayName()}
                        >
                          {displayName()}
                        </a>
                        <Show when={showActualName()}>
                          <span class="text-[9px] text-gray-500 dark:text-gray-400 whitespace-nowrap">
                            ({(node as Node).name})
                          </span>
                        </Show>
                        <div class="hidden xl:flex items-center gap-1.5 ml-1">
                          <Show when={isPVEItem}>
                            <span class="text-[9px] px-1 py-0 rounded font-medium bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400">
                              PVE
                            </span>
                          </Show>
                          <Show when={isPVEItem && node!.pveVersion}>
                            <span class="text-[9px] text-gray-500 dark:text-gray-400 whitespace-nowrap">
                              v{node!.pveVersion.split('/')[1] || node!.pveVersion}
                            </span>
                          </Show>
                          <Show when={isPVEItem && node!.isClusterMember !== undefined}>
                            <span
                              class={`text-[9px] px-1 py-0 rounded font-medium whitespace-nowrap ${node!.isClusterMember
                                ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
                                : 'bg-gray-100 text-gray-600 dark:bg-gray-700/50 dark:text-gray-400'
                                }`}
                            >
                              {node!.isClusterMember ? node!.clusterName : 'Standalone'}
                            </span>
                          </Show>
                          <Show when={isPVEItem && node!.linkedHostAgentId}>
                            <span
                              class="text-[9px] px-1 py-0 rounded font-medium whitespace-nowrap bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400"
                              title="Pulse host agent installed for enhanced metrics"
                            >
                              +Agent
                            </span>
                          </Show>
                          <Show when={isPBSItem}>
                            <span class="text-[9px] px-1 py-0 rounded font-medium bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400">
                              PBS
                            </span>
                          </Show>
                          <Show when={isPBSItem && pbs!.version}>
                            <span class="text-[9px] text-gray-500 dark:text-gray-400 whitespace-nowrap">
                              v{pbs!.version}
                            </span>
                          </Show>
                        </div>
                      </div>
                    </td>

                    {/* Uptime */}
                    <td class={tdClass}>
                      <div class="flex justify-center">
                        <span
                          class={`text-xs whitespace-nowrap ${isPVEItem && (node?.uptime ?? 0) < 3600
                            ? 'text-orange-500'
                            : 'text-gray-600 dark:text-gray-400'
                            }`}
                        >
                          <Show when={online && uptimeValue} fallback="-">
                            <Show when={isMobile()} fallback={formatUptime(uptimeValue)}>
                              {formatUptime(uptimeValue, true)}
                            </Show>
                          </Show>
                        </span>
                      </div>
                    </td>

                    {/* CPU */}
                    <td class={tdClass} style={isMobile() ? { "min-width": "60px" } : metricColumnStyle}>
                      <div class="h-4">
                        <EnhancedCPUBar
                          usage={cpuPercentValue}
                          loadAverage={isPVEItem ? node!.loadAverage?.[0] : undefined}
                          cores={isMobile() ? undefined : (isPVEItem ? node!.cpuInfo?.cores : undefined)}
                          model={isPVEItem ? node!.cpuInfo?.model : undefined}
                          resourceId={metricsKey}
                        />
                      </div>
                    </td>

                    {/* Memory */}
                    <td class={tdClass} style={isMobile() ? { "min-width": "60px" } : metricColumnStyle}>
                      <div class="h-4">
                        <Show when={isPVEItem} fallback={
                          <ResponsiveMetricCell
                            value={memoryPercentValue}
                            type="memory"
                            resourceId={metricsKey}
                            sublabel={pbs!.memoryTotal ? `${formatBytes(pbs!.memoryUsed, 0)}/${formatBytes(pbs!.memoryTotal, 0)}` : undefined}
                            isRunning={online}
                            showMobile={false}
                          />
                        }>
                          <Show
                            when={viewMode() === 'sparklines'}
                            fallback={
                              <StackedMemoryBar
                                used={node!.memory?.used || 0}
                                total={node!.memory?.total || 0}
                                balloon={node!.memory?.balloon || 0}
                                swapUsed={node!.memory?.swapUsed || 0}
                                swapTotal={node!.memory?.swapTotal || 0}
                                resourceId={metricsKey}
                              />
                            }
                          >
                            <ResponsiveMetricCell
                              value={memoryPercentValue}
                              type="memory"
                              resourceId={metricsKey}
                              isRunning={online}
                              showMobile={false}
                            />
                          </Show>
                        </Show>
                      </div>
                    </td>

                    {/* Disk */}
                    <td class={tdClass} style={isMobile() ? { "min-width": "60px" } : metricColumnStyle}>
                      <div class="h-4">
                        <Show when={isPVEItem} fallback={
                          <ResponsiveMetricCell
                            value={diskPercentValue}
                            type="disk"
                            resourceId={metricsKey}
                            sublabel={diskSublabel}
                            isRunning={online}
                            showMobile={false}
                          />
                        }>
                          <Show
                            when={viewMode() === 'sparklines'}
                            fallback={
                              <StackedDiskBar
                                aggregateDisk={{
                                  total: node!.disk?.total || 0,
                                  used: node!.disk?.used || 0,
                                  free: (node!.disk?.total || 0) - (node!.disk?.used || 0),
                                  usage: node!.disk?.total ? (node!.disk.used / node!.disk.total) : 0
                                }}
                              />
                            }
                          >
                            <ResponsiveMetricCell
                              value={diskPercentValue}
                              type="disk"
                              resourceId={metricsKey}
                              isRunning={online}
                              showMobile={false}
                            />
                          </Show>
                        </Show>
                      </div>
                    </td>

                    {/* Temperature */}
                    <Show when={hasAnyTemperatureData()}>
                      <td class={tdClass}>
                        <div class="flex justify-center">
                          <Show
                            when={
                              online &&
                              isPVEItem &&
                              cpuTemperatureValue !== null &&
                              (node!.temperature?.hasCPU ?? node!.temperature?.hasGPU ?? node!.temperature?.available) &&
                              isTemperatureMonitoringEnabled(node!)
                            }
                            fallback={<span class="text-xs text-gray-400 dark:text-gray-500">-</span>}
                          >
                            {(() => {
                              const value = cpuTemperatureValue as number;
                              const temp = node!.temperature;
                              const cpuMinValue =
                                typeof temp?.cpuMin === 'number' && temp.cpuMin > 0 ? temp.cpuMin : null;
                              const cpuMaxValue =
                                typeof temp?.cpuMaxRecord === 'number' && temp.cpuMaxRecord > 0
                                  ? temp.cpuMaxRecord
                                  : null;
                              const hasMinMax = cpuMinValue !== null && cpuMaxValue !== null;

                              const gpus = temp?.gpu ?? [];
                              const hasGPU = gpus.length > 0;

                              if (hasMinMax || hasGPU) {
                                const min = typeof cpuMinValue === 'number' ? Math.round(cpuMinValue) : undefined;
                                const max = typeof cpuMaxValue === 'number' ? Math.round(cpuMaxValue) : undefined;

                                return (
                                  <div title={`Min: ${min ?? '-'}°C, Max: ${max ?? '-'}°C${hasGPU ? `\nGPU: ${gpus.map(g => `${g.edge ?? g.junction ?? g.mem}°C`).join(', ')}` : ''}`}>
                                    <TemperatureGauge
                                      value={value}
                                      min={min}
                                      max={max}
                                    />
                                  </div>
                                );
                              }

                              return (
                                <TemperatureGauge value={value} />
                              );
                            })()}
                          </Show>
                        </div>
                      </td>
                    </Show>

                    {/* Dashboard tab: VMs and CTs */}
                    <Show when={props.currentTab === 'dashboard'}>
                      <td class={tdClass}>
                        <div class="flex justify-center">
                          <span class={online ? 'text-xs text-gray-700 dark:text-gray-300' : 'text-xs text-gray-400 dark:text-gray-500'}>
                            {online ? getCountValue(item, 'vmCount') ?? '-' : '-'}
                          </span>
                        </div>
                      </td>
                      <td class={tdClass}>
                        <div class="flex justify-center">
                          <span class={online ? 'text-xs text-gray-700 dark:text-gray-300' : 'text-xs text-gray-400 dark:text-gray-500'}>
                            {online ? getCountValue(item, 'containerCount') ?? '-' : '-'}
                          </span>
                        </div>
                      </td>
                    </Show>

                    {/* Storage tab: Storage and Disks */}
                    <Show when={props.currentTab === 'storage'}>
                      <td class={tdClass}>
                        <div class="flex justify-center">
                          <span class={online ? 'text-xs text-gray-700 dark:text-gray-300' : 'text-xs text-gray-400 dark:text-gray-500'}>
                            {online ? getCountValue(item, 'storageCount') ?? '-' : '-'}
                          </span>
                        </div>
                      </td>
                      <td class={tdClass}>
                        <div class="flex justify-center">
                          <span class={online ? 'text-xs text-gray-700 dark:text-gray-300' : 'text-xs text-gray-400 dark:text-gray-500'}>
                            {online ? getCountValue(item, 'diskCount') ?? '-' : '-'}
                          </span>
                        </div>
                      </td>
                    </Show>

                    {/* Backups tab: Backups */}
                    <Show when={props.currentTab === 'backups'}>
                      <td class={tdClass}>
                        <div class="flex justify-center">
                          <span class={online ? 'text-xs text-gray-700 dark:text-gray-300' : 'text-xs text-gray-400 dark:text-gray-500'}>
                            {online ? getCountValue(item, 'backupCount') ?? '-' : '-'}
                          </span>
                        </div>
                      </td>
                    </Show>
                  </tr>
                );
              }}
            </For>
          </tbody>
        </table>
      </div>
    </Card>
  );
};
