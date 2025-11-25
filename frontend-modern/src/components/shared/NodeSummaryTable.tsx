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
import { type ColumnPriority } from '@/hooks/useBreakpoint';
import { ResponsiveMetricCell, useGridTemplate } from '@/components/shared/responsive';

// Icons for mobile headers
const ClockIcon = (props: { class?: string }) => (
  <svg class={props.class} fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
  </svg>
);

const CpuIcon = (props: { class?: string }) => (
  <svg class={props.class} fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
  </svg>
);

const MemoryIcon = (props: { class?: string }) => (
  <svg class={props.class} fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
  </svg>
);

const DiskIcon = (props: { class?: string }) => (
  <svg class={props.class} fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" />
  </svg>
);

const TempIcon = (props: { class?: string }) => (
  <svg class={props.class} fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 2a2 2 0 00-2 2v9.5a4 4 0 104 0V4a2 2 0 00-2-2z" />
    <circle cx="12" cy="17" r="2" fill="currentColor" />
  </svg>
);

const VMIcon = (props: { class?: string }) => (
  <svg class={props.class} fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
  </svg>
);

const ContainerIcon = (props: { class?: string }) => (
  <svg class={props.class} fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
  </svg>
);

const BackupIcon = (props: { class?: string }) => (
  <svg class={props.class} fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l-3-3m3 3V4" />
  </svg>
);

// Column configuration using the new priority system
interface ColumnDef {
  id: string;
  label: string;
  priority: ColumnPriority;
  icon?: Component<{ class?: string }>;
  minWidth?: string;
  maxWidth?: string;
  flex?: number;
  align?: 'left' | 'center' | 'right';
}

const BASE_COLUMNS: ColumnDef[] = [
  { id: 'name', label: 'Node', priority: 'essential', minWidth: '70px', flex: 1.5, align: 'left' },
  { id: 'uptime', label: 'Uptime', priority: 'essential', icon: ClockIcon, minWidth: '35px', maxWidth: '70px', align: 'center' },
  { id: 'cpu', label: 'CPU', priority: 'essential', icon: CpuIcon, minWidth: '40px', flex: 1.2, align: 'center' },
  { id: 'memory', label: 'Memory', priority: 'essential', icon: MemoryIcon, minWidth: '40px', flex: 1.2, align: 'center' },
  { id: 'disk', label: 'Disk', priority: 'essential', icon: DiskIcon, minWidth: '40px', flex: 1.2, align: 'center' },
];

const TEMP_COLUMN: ColumnDef = {
  id: 'temperature',
  label: 'Temp',
  priority: 'essential',
  icon: TempIcon,
  minWidth: '35px',
  maxWidth: '55px',
  align: 'center'
};

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
    icon: Component<{ class?: string }>;
    minWidth?: string;
    maxWidth?: string;
  }

  const [sortKey, setSortKey] = createSignal<SortKey>('default');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');

  const countColumns = createMemo<CountColumn[]>(() => {
    switch (props.currentTab) {
      case 'dashboard':
        return [
          { header: 'VMs', key: 'vmCount', icon: VMIcon, minWidth: '40px', maxWidth: '50px' },
          { header: 'CTs', key: 'containerCount', icon: ContainerIcon, minWidth: '40px', maxWidth: '50px' },
        ];
      case 'storage':
        return [
          { header: 'Storage', key: 'storageCount', icon: DiskIcon, minWidth: '50px', maxWidth: '70px' },
          { header: 'Disks', key: 'diskCount', icon: DiskIcon, minWidth: '45px', maxWidth: '60px' },
        ];
      case 'backups':
        return [{ header: 'Backups', key: 'backupCount', icon: BackupIcon, minWidth: '50px', maxWidth: '70px' }];
      default:
        return [];
    }
  });

  const hasAnyTemperatureData = createMemo(() => {
    return (
      props.nodes?.some(
        (node) => node.temperature?.available || isTemperatureMonitoringEnabled(node),
      ) || false
    );
  });

  // Build dynamic columns list
  const columns = createMemo<ColumnDef[]>(() => {
    const cols = [...BASE_COLUMNS];

    if (hasAnyTemperatureData()) {
      cols.push(TEMP_COLUMN);
    }

    // Add count columns based on tab
    countColumns().forEach((cc) => {
      cols.push({
        id: cc.key,
        label: cc.header,
        priority: 'essential' as ColumnPriority,
        icon: cc.icon,
        minWidth: cc.minWidth || '40px',
        maxWidth: cc.maxWidth || '50px',
        align: 'center',
      });
    });

    return cols;
  });

  // Use the responsive grid template hook for dynamic column visibility
  // Pass the columns accessor to support reactive column changes
  const { gridTemplate, visibleColumns, isMobile } = useGridTemplate({ columns });

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

  // Header cell component
  const HeaderCell: Component<{ column: ColumnDef }> = (cellProps) => {
    const Icon = cellProps.column.icon;
    const isSorted = () => sortKey() === cellProps.column.id;
    const sortIndicator = () => isSorted() ? (sortDirection() === 'asc' ? '▲' : '▼') : '';

    const alignClass = () => {
      if (cellProps.column.align === 'center') return 'justify-center text-center';
      if (cellProps.column.align === 'right') return 'justify-end text-right';
      return 'justify-start text-left';
    };

    const baseClass = `px-0.5 md:px-2 py-1 cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 flex items-center h-full ${alignClass()}`;
    const nameClass = cellProps.column.id === 'name' ? 'pl-3' : '';

    return (
      <div
        class={`${baseClass} ${nameClass}`}
        onClick={() => handleSort(cellProps.column.id as Exclude<SortKey, 'default'>)}
        onKeyDown={(e) => e.key === 'Enter' && handleSort(cellProps.column.id as Exclude<SortKey, 'default'>)}
        tabindex="0"
        role="button"
        aria-label={`Sort by ${cellProps.column.label} ${isSorted() ? (sortDirection() === 'asc' ? 'ascending' : 'descending') : ''}`}
      >
        <Show when={Icon !== undefined && isMobile()}>
          <span class="md:hidden" title={cellProps.column.label}>
            {Icon && <Icon class="w-4 h-4" />}
          </span>
        </Show>
        <span class={Icon !== undefined && isMobile() ? 'hidden md:inline' : ''}>
          {cellProps.column.id === 'name' && props.currentTab === 'backups' ? 'Node / PBS' : cellProps.column.label}
        </span>
        <Show when={sortIndicator()}>
          <span class="ml-0.5">{sortIndicator()}</span>
        </Show>
      </div>
    );
  };

  return (
    <Card padding="none" class="mb-4 overflow-hidden">
      <div>
        {/* Header */}
        <div
          class="grid items-center bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-600 text-[11px] sm:text-xs font-medium uppercase tracking-wider sticky top-0 z-20"
          style={{ 'grid-template-columns': gridTemplate() }}
        >
          <For each={visibleColumns()}>
            {(column, index) => (
              <HeaderCell column={column} />
            )}
          </For>
        </div>

        <div class="divide-y divide-gray-200 dark:divide-gray-700">
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

              const nodeId = isPVE ? node!.id : pbs!.name;
              const isSelected = () => props.selectedNode === nodeId;
              const resourceId = isPVE ? node!.id || node!.name : pbs!.id || pbs!.name;
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
                  return 'cursor-pointer transition-all duration-200 relative hover:shadow-sm group';
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

              // Render cell content based on column type
              const renderCell = (column: ColumnDef) => {
                const baseCellClass = `${cellBgClass()} px-0.5 md:px-2 py-1 flex items-center h-full`;
                const alignClass = column.align === 'center' ? 'justify-center' : column.align === 'right' ? 'justify-end' : 'justify-start';

                switch (column.id) {
                  case 'name':
                    return (
                      <div class={`${baseCellClass} ${showAlertHighlight() ? 'pl-4' : 'pl-3'}`}>
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
                    );

                  case 'uptime':
                    return (
                      <div class={`${baseCellClass} ${alignClass} whitespace-nowrap`}>
                        <span
                          class={`text-xs ${isPVE && (node?.uptime ?? 0) < 3600
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
                    );

                  case 'cpu':
                    return (
                      <div class={`${baseCellClass} ${alignClass}`}>
                        <ResponsiveMetricCell
                          value={cpuPercentValue}
                          type="cpu"
                          resourceId={metricsKey}
                          sublabel={isPVE && node!.cpuInfo?.cores ? `${node!.cpuInfo.cores} cores` : undefined}
                          isRunning={online}
                          showMobile={isMobile()}
                          class="w-full"
                        />
                      </div>
                    );

                  case 'memory':
                    return (
                      <div class={`${baseCellClass} ${alignClass}`}>
                        <ResponsiveMetricCell
                          value={memoryPercentValue}
                          type="memory"
                          resourceId={metricsKey}
                          sublabel={
                            isPVE && node!.memory
                              ? `${formatBytes(node!.memory.used, 0)}/${formatBytes(node!.memory.total, 0)}`
                              : isPBS && pbs!.memoryTotal
                                ? `${formatBytes(pbs!.memoryUsed, 0)}/${formatBytes(pbs!.memoryTotal, 0)}`
                                : undefined
                          }
                          isRunning={online}
                          showMobile={isMobile()}
                          class="w-full"
                        />
                      </div>
                    );

                  case 'disk':
                    return (
                      <div class={`${baseCellClass} ${alignClass}`}>
                        <ResponsiveMetricCell
                          value={diskPercentValue}
                          type="disk"
                          resourceId={metricsKey}
                          sublabel={diskSublabel}
                          isRunning={online}
                          showMobile={isMobile()}
                          class="w-full"
                        />
                      </div>
                    );

                  case 'temperature':
                    return (
                      <div class={`${baseCellClass} ${alignClass} whitespace-nowrap`}>
                        <Show
                          when={
                            online &&
                            isPVE &&
                            cpuTemperatureValue !== null &&
                            (node!.temperature?.hasCPU ?? node!.temperature?.hasGPU ?? node!.temperature?.available) &&
                            isTemperatureMonitoringEnabled(node!)
                          }
                          fallback={<span class="text-xs text-gray-400 dark:text-gray-500">-</span>}
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
                                <span class="relative inline-block group/temp">
                                  <span class={`text-xs font-medium ${severityClass} cursor-help`}>
                                    {value}°C
                                  </span>
                                  <span class="invisible group-hover/temp:visible absolute bottom-full left-1/2 -translate-x-1/2 mb-1 px-2 py-1 text-xs whitespace-nowrap bg-gray-900 dark:bg-gray-700 text-white rounded shadow-lg z-50 pointer-events-none">
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
                    );

                  // Count columns (vmCount, containerCount, storageCount, diskCount, backupCount)
                  default:
                    if (column.id.endsWith('Count')) {
                      const value = getCountValue(item, column.id as CountSortKey);
                      const display = online ? value ?? '-' : '-';
                      const textClass = online
                        ? 'text-xs text-gray-700 dark:text-gray-300'
                        : 'text-xs text-gray-400 dark:text-gray-500';
                      return (
                        <div class={`${baseCellClass} ${alignClass} whitespace-nowrap`}>
                          <span class={textClass}>{display}</span>
                        </div>
                      );
                    }
                    return null;
                }
              };

              return (
                <div
                  class={`${rowClass()} grid items-center`}
                  style={{ ...rowStyle(), 'grid-template-columns': gridTemplate() }}
                  onClick={() => props.onNodeClick(nodeId, item.type)}
                >
                  <For each={visibleColumns()}>
                    {(column) => renderCell(column)}
                  </For>
                </div>
              );
            }}
          </For>
        </div>
      </div>
    </Card>
  );
};
