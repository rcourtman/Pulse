import { Component, For, Show, createMemo, createSignal } from 'solid-js';
import type { Node, VM, Container, Storage, PBSInstance } from '@/types/api';
import { formatBytes, formatPercent, formatUptime } from '@/utils/format';
import { MetricBar } from '@/components/Dashboard/MetricBar';
import { useWebSocket } from '@/App';
import { getAlertStyles } from '@/utils/alerts';
import { Card } from '@/components/shared/Card';
import { getNodeDisplayName, hasAlternateDisplayName } from '@/utils/nodes';
import { getCpuTemperature } from '@/utils/temperature';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { buildMetricKey } from '@/utils/metricsKeys';

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

  const isTemperatureMonitoringEnabled = (node: Node): boolean => {
    const globalEnabled = props.globalTemperatureMonitoringEnabled ?? true;
    // Check per-node setting first, fall back to global
    if (node.temperatureMonitoringEnabled !== undefined && node.temperatureMonitoringEnabled !== null) {
      return node.temperatureMonitoringEnabled;
    }
    return globalEnabled;
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

  interface SortableItem {
    type: 'pve' | 'pbs';
    data: Node | PBSInstance;
  }

  interface CountColumn {
    header: string;
    key: CountSortKey;
  }

  const [sortKey, setSortKey] = createSignal<SortKey>('default');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');

  const countColumns = createMemo<CountColumn[]>(() => {
    switch (props.currentTab) {
      case 'dashboard':
        return [
          { header: 'VMs', key: 'vmCount' },
          { header: 'Containers', key: 'containerCount' },
        ];
      case 'storage':
        return [
          { header: 'Storage', key: 'storageCount' },
          { header: 'Disks', key: 'diskCount' },
        ];
      case 'backups':
        return [{ header: 'Backups', key: 'backupCount' }];
      default:
        return [];
    }
  });

  const hasAnyTemperatureData = createMemo(() => {
    // Show temperature column if ANY node has monitoring enabled OR has temperature data
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

  const getCpuTemperatureValue = (item: SortableItem) => {
    if (item.type !== 'pve') return null;
    const node = item.data as Node;
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

  const isItemOnline = (item: SortableItem) => {
    if (item.type === 'pve') {
      const node = item.data as Node;
      return node.status === 'online' && (node.uptime || 0) > 0;
    }
    const pbs = item.data as PBSInstance;
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

  const getCpuPercent = (item: SortableItem) => {
    if (item.type === 'pve') {
      const node = item.data as Node;
      return Math.round((node.cpu || 0) * 100);
    }
    const pbs = item.data as PBSInstance;
    return Math.round(pbs.cpu || 0);
  };

  const getMemoryPercent = (item: SortableItem) => {
    if (item.type === 'pve') {
      const node = item.data as Node;
      return Math.round(node.memory?.usage || 0);
    }
    const pbs = item.data as PBSInstance;
    if (!pbs.memoryTotal) return 0;
    return Math.round((pbs.memoryUsed / pbs.memoryTotal) * 100);
  };

  const getDiskPercent = (item: SortableItem) => {
    if (item.type === 'pve') {
      const node = item.data as Node;
      if (!node.disk || node.disk.total === 0) return 0;
      return Math.round((node.disk.used / node.disk.total) * 100);
    }
    const pbs = item.data as PBSInstance;
    const totals = getPbsTotals(pbs);
    if (totals.total === 0) return 0;
    return Math.round((totals.used / totals.total) * 100);
  };

  const getDiskSublabel = (item: SortableItem) => {
    if (item.type === 'pve') {
      const node = item.data as Node;
      if (!node.disk) return undefined;
      return `${formatBytes(node.disk.used, 0)}/${formatBytes(node.disk.total, 0)}`;
    }
    const pbs = item.data as PBSInstance;
    if (!pbs.datastores || pbs.datastores.length === 0) return undefined;
    const totals = getPbsTotals(pbs);
    return `${formatBytes(totals.used, 0)}/${formatBytes(totals.total, 0)}`;
  };

  const getTemperatureValue = (item: SortableItem) => {
    return getCpuTemperatureValue(item);
  };

  const getCountValue = (item: SortableItem, key: CountSortKey): number | null => {
    if (item.type === 'pbs') {
      const pbs = item.data as PBSInstance;
      if (key === 'backupCount') {
        return props.backupCounts?.[pbs.name] ?? 0;
      }
      return null;
    }

    const node = item.data as Node;
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

  const getSortValue = (item: SortableItem, key: SortKey): number | string | null => {
    switch (key) {
      case 'name':
        return item.type === 'pve'
          ? getNodeDisplayName(item.data as Node)
          : (item.data as PBSInstance).name;
      case 'uptime':
        return item.type === 'pve'
          ? (item.data as Node).uptime ?? 0
          : (item.data as PBSInstance).uptime ?? 0;
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

  const defaultComparison = (a: SortableItem, b: SortableItem) => {
    if (a.type !== b.type) return a.type === 'pve' ? -1 : 1;

    const aOnline = isItemOnline(a);
    const bOnline = isItemOnline(b);
    if (aOnline !== bOnline) return aOnline ? -1 : 1;

    const aName = a.type === 'pve'
      ? getNodeDisplayName(a.data as Node)
      : (a.data as PBSInstance).name;
    const bName = b.type === 'pve'
      ? getNodeDisplayName(b.data as Node)
      : (b.data as PBSInstance).name;

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

  // Combine and sort nodes based on current sort selection
  const sortedItems = createMemo(() => {
    const items: SortableItem[] = [];

    props.nodes?.forEach((node) => items.push({ type: 'pve', data: node }));
    props.pbsInstances?.forEach((pbs) => items.push({ type: 'pbs', data: pbs }));

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

  // Don't return null - let the table render even if empty
  // This prevents the table from disappearing on refresh while data loads

  return (
    <>
      <Card padding="none" class="mb-4 overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full min-w-[600px] table-fixed border-collapse">
          <thead>
            <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600">
              <th
                class="pl-3 pr-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider w-1/4 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-blue-500"
                onClick={() => handleSort('name')}
                onKeyDown={(e) => e.key === 'Enter' && handleSort('name')}
                tabindex="0"
                role="button"
                aria-label={`Sort by name ${
                  sortKey() === 'name' ? (sortDirection() === 'asc' ? 'ascending' : 'descending') : ''
                }`}
              >
                {props.currentTab === 'backups' ? 'Node / PBS' : 'Node'}{' '}
                {sortKey() === 'name' && (sortDirection() === 'asc' ? '▲' : '▼')}
              </th>
              <th
                class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider min-w-24 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                onClick={() => handleSort('uptime')}
              >
                Uptime {sortKey() === 'uptime' && (sortDirection() === 'asc' ? '▲' : '▼')}
              </th>
              <th
                class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider min-w-32 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                onClick={() => handleSort('cpu')}
              >
                CPU {sortKey() === 'cpu' && (sortDirection() === 'asc' ? '▲' : '▼')}
              </th>
              <th
                class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider min-w-32 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                onClick={() => handleSort('memory')}
              >
                Memory {sortKey() === 'memory' && (sortDirection() === 'asc' ? '▲' : '▼')}
              </th>
              <th
                class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider min-w-32 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                onClick={() => handleSort('disk')}
              >
                {props.currentTab === 'backups' && props.pbsInstances ? 'Storage / Disk' : 'Disk'}{' '}
                {sortKey() === 'disk' && (sortDirection() === 'asc' ? '▲' : '▼')}
              </th>
              <Show when={hasAnyTemperatureData()}>
                <th
                  class="px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider min-w-20 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                  onClick={() => handleSort('temperature')}
                >
                  Temp{' '}
                  {sortKey() === 'temperature' && (sortDirection() === 'asc' ? '▲' : '▼')}
                </th>
              </Show>
              <For each={countColumns()}>
                {(column) => (
                  <th
                    class="px-2 py-1.5 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider min-w-16 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600"
                    onClick={() => handleSort(column.key)}
                  >
                    {column.header}{' '}
                    {sortKey() === column.key && (sortDirection() === 'asc' ? '▲' : '▼')}
                  </th>
                )}
              </For>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
            <For each={sortedItems()}>
              {(item) => {
                const isPVE = item.type === 'pve';
                const isPBS = item.type === 'pbs';
                const node = isPVE ? (item.data as Node) : null;
                const pbs = isPBS ? (item.data as PBSInstance) : null;

                const online = isItemOnline(item);
                const cpuPercentValue = getCpuPercent(item);
                const memoryPercentValue = getMemoryPercent(item);
                const diskPercentValue = getDiskPercent(item);
                const diskSublabel = getDiskSublabel(item);
                const cpuTemperatureValue = getCpuTemperatureValue(item);
                const uptimeValue = isPVE ? node?.uptime ?? 0 : isPBS ? pbs?.uptime ?? 0 : 0;
                const displayName = () => {
                  if (isPVE) return getNodeDisplayName(node as Node);
                  return (pbs as PBSInstance).name;
                };
                const showActualName = () => isPVE && hasAlternateDisplayName(node as Node);

                // Use unique node ID (not hostname) to handle duplicate node names
                const nodeId = isPVE ? node!.id : pbs!.name;
                const isSelected = () => props.selectedNode === nodeId;
                // Use the full resource ID for alert matching
                const resourceId = isPVE ? node!.id || node!.name : pbs!.id || pbs!.name;
                // Use namespaced metric key for sparklines
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
                  const baseHover = 'cursor-pointer transition-all duration-200 relative hover:bg-gray-50 dark:hover:bg-gray-700/50 hover:shadow-sm';

                  if (isSelected()) {
                    return `cursor-pointer transition-all duration-200 relative bg-blue-50 dark:bg-blue-900/20 hover:bg-blue-100 dark:hover:bg-blue-900/30 hover:shadow-sm z-10`;
                  }

                  if (showAlertHighlight()) {
                    return alertStyles().severity === 'critical'
                      ? 'cursor-pointer transition-all duration-200 relative bg-red-50 dark:bg-red-950/30 hover:bg-red-100 dark:hover:bg-red-950/40 hover:shadow-sm'
                      : 'cursor-pointer transition-all duration-200 relative bg-yellow-50 dark:bg-yellow-950/20 hover:bg-yellow-100 dark:hover:bg-yellow-950/30 hover:shadow-sm';
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
                    style={rowStyle()}
                    onClick={() => props.onNodeClick(nodeId, item.type)}
                  >
                    <td
                      class={`pr-2 py-0.5 whitespace-nowrap ${showAlertHighlight() ? 'pl-4' : 'pl-3'}`}
                    >
                      <div class="flex items-center gap-1">
                        <a
                          href={
                            isPVE
                              ? node!.guestURL || node!.host || `https://${node!.name}:8006`
                              : pbs!.host || `https://${pbs!.name}:8007`
                          }
                          target="_blank"
                          onClick={(e) => e.stopPropagation()}
                          class="font-medium text-[11px] text-gray-900 dark:text-gray-100 hover:text-blue-600 dark:hover:text-blue-400"
                        >
                          {displayName()}
                        </a>
                        <Show when={showActualName()}>
                          <span class="text-[9px] text-gray-500 dark:text-gray-400">
                            ({(node as Node).name})
                          </span>
                        </Show>
                        <Show when={isPVE}>
                          <span class="text-[9px] px-1 py-0 rounded text-[8px] font-medium bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400">
                            PVE
                          </span>
                        </Show>
                        <Show when={isPVE && node!.pveVersion}>
                          <span class="text-[9px] text-gray-500 dark:text-gray-400">
                            v{node!.pveVersion.split('/')[1] || node!.pveVersion}
                          </span>
                        </Show>
                        <Show when={isPVE && node!.isClusterMember !== undefined}>
                          <span
                            class={`text-[9px] px-1 py-0 rounded text-[8px] font-medium whitespace-nowrap ${
                              node!.isClusterMember
                                ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
                                : 'bg-gray-100 text-gray-600 dark:bg-gray-700/50 dark:text-gray-400'
                            }`}
                          >
                            {node!.isClusterMember ? node!.clusterName : 'Standalone'}
                          </span>
                        </Show>
                        <Show when={isPBS}>
                          <span class="text-[9px] px-1 py-0 rounded text-[8px] font-medium bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400">
                            PBS
                          </span>
                        </Show>
                        <Show when={isPBS && pbs!.version}>
                          <span class="text-[9px] text-gray-500 dark:text-gray-400">
                            v{pbs!.version}
                          </span>
                        </Show>
                      </div>
                    </td>
                    <td class="px-2 py-0.5 whitespace-nowrap">
                      <span
                        class={`text-xs ${
                          isPVE && (node?.uptime ?? 0) < 3600
                            ? 'text-orange-500'
                            : 'text-gray-600 dark:text-gray-400'
                        }`}
                      >
                        <Show
                          when={online && uptimeValue}
                          fallback="-"
                        >
                          {formatUptime(uptimeValue)}
                        </Show>
                      </span>
                    </td>
                    <td class="px-2 py-0.5">
                      <Show
                        when={online && cpuPercentValue !== null}
                        fallback={<span class="text-xs text-gray-500 dark:text-gray-400">-</span>}
                      >
                        <MetricBar
                          value={cpuPercentValue ?? 0}
                          label={formatPercent(cpuPercentValue ?? 0)}
                          sublabel={
                            isPVE && node!.cpuInfo?.cores
                              ? `${node!.cpuInfo.cores} cores`
                              : undefined
                          }
                          type="cpu"
                          resourceId={metricsKey}
                        />
                      </Show>
                    </td>
                    <td class="px-2 py-0.5">
                      <Show
                        when={online && memoryPercentValue !== null}
                        fallback={<span class="text-xs text-gray-500 dark:text-gray-400">-</span>}
                      >
                        <MetricBar
                          value={memoryPercentValue ?? 0}
                          label={formatPercent(memoryPercentValue ?? 0)}
                          sublabel={
                            isPVE && node!.memory
                              ? `${formatBytes(node!.memory.used, 0)}/${formatBytes(node!.memory.total, 0)}`
                              : isPBS && pbs!.memoryTotal
                                ? `${formatBytes(pbs!.memoryUsed, 0)}/${formatBytes(pbs!.memoryTotal, 0)}`
                                : undefined
                          }
                          type="memory"
                          resourceId={metricsKey}
                        />
                      </Show>
                    </td>
                    <td class="px-2 py-0.5">
                      <Show
                        when={online && diskPercentValue !== null}
                        fallback={<span class="text-xs text-gray-500 dark:text-gray-400">-</span>}
                      >
                        <MetricBar
                          value={diskPercentValue ?? 0}
                          label={formatPercent(diskPercentValue ?? 0)}
                          sublabel={diskSublabel}
                          type="disk"
                          resourceId={metricsKey}
                        />
                      </Show>
                    </td>
                    <Show when={hasAnyTemperatureData()}>
                      <td class="px-2 py-0.5 whitespace-nowrap text-center">
                        <Show
                          when={
                            online &&
                            isPVE &&
                            cpuTemperatureValue !== null &&
                            (node!.temperature?.hasCPU ?? node!.temperature?.hasGPU ?? node!.temperature?.available) &&
                            isTemperatureMonitoringEnabled(node!)
                          }
                          fallback={
                            <span class="text-xs text-gray-400 dark:text-gray-500">-</span>
                          }
                        >
                          {(() => {
                            const value = cpuTemperatureValue as number;
                            const severityClass =
                              value >= 80
                                ? 'text-red-600 dark:text-red-400'
                                : value >= 70
                                  ? 'text-yellow-600 dark:text-yellow-400'
                                  : 'text-green-600 dark:text-green-400';

                            const temp = node!.temperature;
                            const cpuMin = temp?.cpuMin;
                            const cpuMax = temp?.cpuMaxRecord;
                            const hasMinMax =
                              typeof cpuMin === 'number' &&
                              cpuMin > 0 &&
                              typeof cpuMax === 'number' &&
                              cpuMax > 0;

                            const gpus = temp?.gpu ?? [];
                            const hasGPU = gpus.length > 0;

                            if (hasMinMax || hasGPU) {
                              const min = Math.round(cpuMin);
                              const max = Math.round(cpuMax);

                              const getTooltipColor = (temp: number) => {
                                if (temp >= 80) return 'text-red-400';
                                if (temp >= 70) return 'text-yellow-400';
                                return 'text-green-400';
                              };

                              return (
                                <span class="relative inline-block group">
                                  <span class={`text-xs font-medium ${severityClass} cursor-help`}>
                                    {value}°C
                                  </span>
                                  <span class="invisible group-hover:visible absolute bottom-full left-1/2 -translate-x-1/2 mb-1 px-2 py-1 text-xs whitespace-nowrap bg-gray-900 dark:bg-gray-700 text-white rounded shadow-lg z-50 pointer-events-none">
                                    {hasMinMax && (
                                      <div>
                                        <span class="text-gray-300">CPU:</span> <span class={getTooltipColor(min)}>{min}</span>-<span class={getTooltipColor(max)}>{max}</span>°C
                                      </div>
                                    )}
                                    {hasGPU && gpus.map((gpu) => {
                                      const gpuTemp = gpu.edge ?? gpu.junction ?? gpu.mem ?? 0;
                                      return (
                                        <div>
                                          <span class="text-gray-300">GPU:</span> <span class={getTooltipColor(gpuTemp)}>{Math.round(gpuTemp)}</span>°C
                                          {gpu.edge && ` E:${Math.round(gpu.edge)}`}
                                          {gpu.junction && ` J:${Math.round(gpu.junction)}`}
                                          {gpu.mem && ` M:${Math.round(gpu.mem)}`}
                                        </div>
                                      );
                                    })}
                                  </span>
                                </span>
                              );
                            }

                            return <span class={`text-xs font-medium ${severityClass}`}>{value}°C</span>;
                          })()}
                        </Show>
                      </td>
                    </Show>
                    <For each={countColumns()}>
                      {(column) => {
                        const value = getCountValue(item, column.key);
                        const display = online ? value ?? '-' : '-';
                        const textClass = online
                          ? 'text-xs text-gray-700 dark:text-gray-300'
                          : 'text-xs text-gray-400 dark:text-gray-500';
                        return (
                          <td class="px-2 py-0.5 whitespace-nowrap text-center">
                            <span class={textClass}>{display}</span>
                          </td>
                        );
                      }}
                    </For>
                  </tr>
                );
              }}
            </For>
          </tbody>
        </table>
      </div>
      </Card>
    </>
  );
};
