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
import { StatusDot } from '@/components/shared/StatusDot';
import { getNodeStatusIndicator, getPBSStatusIndicator } from '@/utils/status';

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

  const gridTemplate = createMemo(() => {
    const parts = ['minmax(90px, 1.5fr)']; // Name
    parts.push('minmax(45px, 80px)'); // Uptime
    parts.push('minmax(55px, 1.5fr)'); // CPU
    parts.push('minmax(55px, 1.5fr)'); // Memory
    parts.push('minmax(55px, 1.5fr)'); // Disk

    if (hasAnyTemperatureData()) {
      parts.push('minmax(45px, 65px)'); // Temp
    }

    countColumns().forEach(() => {
      parts.push('minmax(40px, 85px)'); // Counts
    });

    return parts.join(' ');
  });

  // Don't return null - let the table render even if empty
  // This prevents the table from disappearing on refresh while data loads

  return (
    <>
      <Card padding="none" class="mb-4 overflow-hidden">
        <div class="overflow-x-auto">
          {/* Header */}
          <div
            class="grid items-center bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600 text-[11px] sm:text-xs font-medium uppercase tracking-wider sticky top-0 z-20 min-w-full"
            style={{ 'grid-template-columns': gridTemplate() }}
          >
            <div
              class="sticky left-0 z-30 bg-gray-50 dark:bg-gray-700/50 pl-3 pr-2 py-1 text-left border-r md:border-r-0 border-gray-200 dark:border-gray-600 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 flex items-center h-full"
              onClick={() => handleSort('name')}
              onKeyDown={(e) => e.key === 'Enter' && handleSort('name')}
              tabindex="0"
              role="button"
              aria-label={`Sort by name ${sortKey() === 'name' ? (sortDirection() === 'asc' ? 'ascending' : 'descending') : ''
                }`}
            >
              {props.currentTab === 'backups' ? 'Node / PBS' : 'Node'}{' '}
              {sortKey() === 'name' && (sortDirection() === 'asc' ? '▲' : '▼')}
            </div>
            <div
              class="px-0.5 md:px-2 py-1 text-center md:text-left cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 flex items-center justify-center md:justify-start h-full"
              onClick={() => handleSort('uptime')}
            >
              <span class="md:hidden" title="Uptime">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
              </span>
              <span class="hidden md:inline">Uptime</span>
              {sortKey() === 'uptime' && (sortDirection() === 'asc' ? '▲' : '▼')}
            </div>
            <div
              class="px-0.5 md:px-2 py-1 text-center md:text-left cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 flex items-center justify-center md:justify-start h-full"
              onClick={() => handleSort('cpu')}
            >
              <span class="md:hidden" title="CPU">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" /></svg>
              </span>
              <span class="hidden md:inline">CPU</span>
              {sortKey() === 'cpu' && (sortDirection() === 'asc' ? '▲' : '▼')}
            </div>
            <div
              class="px-0.5 md:px-2 py-1 text-center md:text-left cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 flex items-center justify-center md:justify-start h-full"
              onClick={() => handleSort('memory')}
            >
              <span class="md:hidden" title="Memory">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>
              </span>
              <span class="hidden md:inline">Memory</span>
              {sortKey() === 'memory' && (sortDirection() === 'asc' ? '▲' : '▼')}
            </div>
            <div
              class="px-0.5 md:px-2 py-1 text-center md:text-left cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 flex items-center justify-center md:justify-start h-full"
              onClick={() => handleSort('disk')}
            >
              <span class="md:hidden" title={props.currentTab === 'backups' && props.pbsInstances ? 'Storage / Disk' : 'Disk'}>
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" /></svg>
              </span>
              <span class="hidden md:inline">
                {props.currentTab === 'backups' && props.pbsInstances ? 'Storage / Disk' : 'Disk'}
              </span>
              {sortKey() === 'disk' && (sortDirection() === 'asc' ? '▲' : '▼')}
            </div>
            <Show when={hasAnyTemperatureData()}>
              <div
                class="px-0.5 md:px-2 py-1 text-center md:text-left cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 flex items-center justify-center md:justify-start h-full"
                onClick={() => handleSort('temperature')}
              >
                <span class="md:hidden" title="Temperature">
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" /></svg>
                </span>
                <span class="hidden md:inline">Temp</span>
                {sortKey() === 'temperature' && (sortDirection() === 'asc' ? '▲' : '▼')}
              </div>
            </Show>
            <For each={countColumns()}>
              {(column) => (
                <div
                  class="px-0.5 md:px-2 py-1 text-center cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 flex items-center justify-center h-full"
                  onClick={() => handleSort(column.key)}
                >
                  <span class="md:hidden" title={column.header}>
                    <Show when={column.key === 'vmCount'}>
                      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" /></svg>
                    </Show>
                    <Show when={column.key === 'containerCount'}>
                      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" /></svg>
                    </Show>
                    <Show when={column.key === 'storageCount' || column.key === 'diskCount'}>
                      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" /></svg>
                    </Show>
                    <Show when={column.key === 'backupCount'}>
                      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l-3-3m3 3V4" /></svg>
                    </Show>
                  </span>
                  <span class="hidden md:inline">{column.header}</span>
                  {sortKey() === column.key && (sortDirection() === 'asc' ? '▲' : '▼')}
                </div>
              )}
            </For>
          </div>

          <div class="divide-y divide-gray-200 dark:divide-gray-700 min-w-full">
            <For each={sortedItems()}>
              {(item) => {
                const isPVE = item.type === 'pve';
                const isPBS = item.type === 'pbs';
                const node = isPVE ? (item.data as Node) : null;
                const pbs = isPBS ? (item.data as PBSInstance) : null;

                const online = isItemOnline(item);
                const statusIndicator = createMemo(() =>
                  isPVE ? getNodeStatusIndicator(node as Node) : getPBSStatusIndicator(pbs as PBSInstance),
                );
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
                  const baseHover = 'cursor-pointer transition-all duration-200 relative hover:shadow-sm group';

                  if (isSelected()) {
                    return `cursor-pointer transition-all duration-200 relative hover:shadow-sm z-10 group`;
                  }

                  if (showAlertHighlight()) {
                    return alertStyles().severity === 'critical'
                      ? 'cursor-pointer transition-all duration-200 relative hover:shadow-sm group'
                      : 'cursor-pointer transition-all duration-200 relative hover:shadow-sm group';
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

                const cellBgClass = createMemo(() => {
                  if (isSelected()) return 'bg-blue-50 dark:bg-blue-900/20 group-hover:bg-blue-100 dark:group-hover:bg-blue-900/30';
                  if (showAlertHighlight()) {
                    return alertStyles().severity === 'critical'
                      ? 'bg-red-50 dark:bg-red-950/30 group-hover:bg-red-100 dark:group-hover:bg-red-950/40'
                      : 'bg-yellow-50 dark:bg-yellow-950/20 group-hover:bg-yellow-100 dark:group-hover:bg-yellow-950/30';
                  }
                  return 'bg-white dark:bg-gray-800 group-hover:bg-gray-50 dark:group-hover:bg-gray-700/50';
                });

                return (
                  <div
                    class={`${rowClass()} grid items-center`}
                    style={{ ...rowStyle(), 'grid-template-columns': gridTemplate() }}
                    onClick={() => props.onNodeClick(nodeId, item.type)}
                  >
                    <div
                      class={`sticky left-0 z-10 ${cellBgClass()} pr-2 py-1 whitespace-nowrap border-r md:border-r-0 border-gray-100 dark:border-gray-700 flex items-center h-full ${showAlertHighlight() ? 'pl-4' : 'pl-3'}`}
                    >
                      <div class="flex items-center gap-1.5">
                        <StatusDot
                          variant={statusIndicator().variant}
                          title={statusIndicator().label}
                          ariaLabel={statusIndicator().label}
                          size="xs"
                        />
                        <a
                          href={
                            isPVE
                              ? node!.guestURL || node!.host || `https://${node!.name}:8006`
                              : pbs!.host || `https://${pbs!.name}:8007`
                          }
                          target="_blank"
                          onClick={(e) => e.stopPropagation()}
                          class="font-medium text-[11px] text-gray-900 dark:text-gray-100 hover:text-blue-600 dark:hover:text-blue-400 truncate"
                          title={displayName()}
                        >
                          {displayName()}
                        </a>
                        <Show when={showActualName()}>
                          <span class="text-[9px] text-gray-500 dark:text-gray-400 truncate">
                            ({(node as Node).name})
                          </span>
                        </Show>
                        <div class="hidden xl:flex items-center gap-1.5 ml-1">
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
                              class={`text-[9px] px-1 py-0 rounded text-[8px] font-medium whitespace-nowrap ${node!.isClusterMember
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
                      </div>
                    </div>
                    <div class={`${cellBgClass()} px-0.5 md:px-2 py-1 whitespace-nowrap text-center md:text-left flex items-center justify-center md:justify-start h-full`}>
                      <span
                        class={`text-xs ${isPVE && (node?.uptime ?? 0) < 3600
                          ? 'text-orange-500'
                          : 'text-gray-600 dark:text-gray-400'
                          }`}
                      >
                        <Show
                          when={online && uptimeValue}
                          fallback="-"
                        >
                          <span class="md:hidden">{formatUptime(uptimeValue, true)}</span>
                          <span class="hidden md:inline">{formatUptime(uptimeValue)}</span>
                        </Show>
                      </span>
                    </div>
                    <div class={`${cellBgClass()} px-0.5 md:px-2 py-1 flex items-center justify-center h-full`}>
                      <Show
                        when={online && cpuPercentValue !== null}
                        fallback={<span class="text-xs text-gray-500 dark:text-gray-400">-</span>}
                      >
                        <div class={`md:hidden text-xs text-center ${cpuPercentValue! >= 90 ? 'text-red-600 dark:text-red-400 font-bold' : cpuPercentValue! >= 80 ? 'text-orange-600 dark:text-orange-400 font-medium' : 'text-gray-600 dark:text-gray-400'}`}>
                          {formatPercent(cpuPercentValue ?? 0)}
                        </div>
                        <div class="hidden md:block w-full">
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
                        </div>
                      </Show>
                    </div>
                    <div class={`${cellBgClass()} px-0.5 md:px-2 py-1 flex items-center justify-center h-full`}>
                      <Show
                        when={online && memoryPercentValue !== null}
                        fallback={<span class="text-xs text-gray-500 dark:text-gray-400">-</span>}
                      >
                        <div class={`md:hidden text-xs text-center ${memoryPercentValue! >= 85 ? 'text-red-600 dark:text-red-400 font-bold' : memoryPercentValue! >= 75 ? 'text-orange-600 dark:text-orange-400 font-medium' : 'text-gray-600 dark:text-gray-400'}`}>
                          {formatPercent(memoryPercentValue ?? 0)}
                        </div>
                        <div class="hidden md:block w-full">
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
                        </div>
                      </Show>
                    </div>
                    <div class={`${cellBgClass()} px-0.5 md:px-2 py-1 flex items-center justify-center h-full`}>
                      <Show
                        when={online && diskPercentValue !== null}
                        fallback={<span class="text-xs text-gray-500 dark:text-gray-400">-</span>}
                      >
                        <div class={`md:hidden text-xs text-center ${diskPercentValue! >= 90 ? 'text-red-600 dark:text-red-400 font-bold' : diskPercentValue! >= 80 ? 'text-orange-600 dark:text-orange-400 font-medium' : 'text-gray-600 dark:text-gray-400'}`}>
                          {formatPercent(diskPercentValue ?? 0)}
                        </div>
                        <div class="hidden md:block w-full">
                          <MetricBar
                            value={diskPercentValue ?? 0}
                            label={formatPercent(diskPercentValue ?? 0)}
                            sublabel={diskSublabel}
                            type="disk"
                            resourceId={metricsKey}
                          />
                        </div>
                      </Show>
                    </div>
                    <Show when={hasAnyTemperatureData()}>
                      <div class={`${cellBgClass()} px-0.5 md:px-2 py-1 whitespace-nowrap text-center md:text-left flex items-center justify-center md:justify-start h-full`}>
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
                              const min = Math.round(cpuMinValue!);
                              const max = Math.round(cpuMaxValue!);

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
                      </div>
                    </Show>
                    <For each={countColumns()}>
                      {(column) => {
                        const value = getCountValue(item, column.key);
                        const display = online ? value ?? '-' : '-';
                        const textClass = online
                          ? 'text-xs text-gray-700 dark:text-gray-300'
                          : 'text-xs text-gray-400 dark:text-gray-500';
                        return (
                          <div class={`${cellBgClass()} px-0.5 md:px-2 py-1 whitespace-nowrap text-center flex items-center justify-center h-full`}>
                            <span class={textClass}>{display}</span>
                          </div>
                        );
                      }}
                    </For>
                  </div>
                );
              }}
            </For>
          </div>
        </div>
      </Card>
    </>
  );
};
