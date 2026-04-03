import { createMemo, createSignal } from 'solid-js';
import { useWebSocket } from '@/contexts/appRuntime';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { useAlertsActivation } from '@/stores/alertsActivation';
import {
  getInfrastructureSummaryDefaultSortDirection,
  isTemperatureMonitoringEnabled,
  sortInfrastructureSummaryItems,
  type InfrastructureSummarySortKey,
  type InfrastructureSummaryTableProps,
} from './infrastructureSummaryTableModel';

export function useInfrastructureSummaryTableState(props: InfrastructureSummaryTableProps) {
  const { activeAlerts } = useWebSocket();
  const alertsActivation = useAlertsActivation();
  const alertsEnabled = createMemo(() => alertsActivation.activationState() === 'active');
  const { isMobile } = useBreakpoint();
  const temperatureThreshold = createMemo(() => alertsActivation.getTemperatureThreshold());

  const [sortKey, setSortKey] = createSignal<InfrastructureSummarySortKey>('default');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');
  const [expandedNodeId, setExpandedNodeId] = createSignal<string | null>(null);

  const hasAnyTemperatureData = createMemo(
    () =>
      props.nodes?.some(
        (node) =>
          node.temperature?.available ||
          isTemperatureMonitoringEnabled(node, props.globalTemperatureMonitoringEnabled ?? true),
      ) || false,
  );

  const totalColumnCount = createMemo(() => {
    let count = 6;
    if (hasAnyTemperatureData()) count += 1;
    if (props.currentTab === 'dashboard') count += 2;
    else if (props.currentTab === 'storage') count += 2;
    else if (props.currentTab === 'recovery') count += 1;
    return count;
  });

  const sortedItems = createMemo(() =>
    sortInfrastructureSummaryItems(props, sortKey(), sortDirection()),
  );

  const handleSort = (key: Exclude<InfrastructureSummarySortKey, 'default'>) => {
    if (sortKey() === key) {
      if (sortDirection() === 'asc') {
        setSortDirection('desc');
      } else {
        setSortKey('default');
        setSortDirection('asc');
      }
      return;
    }

    setSortKey(key);
    setSortDirection(getInfrastructureSummaryDefaultSortDirection(key));
  };

  const renderSortIndicator = (key: InfrastructureSummarySortKey) => {
    if (sortKey() !== key) return null;
    return sortDirection() === 'asc' ? '▲' : '▼';
  };

  const toggleNodeExpand = (nodeId: string, event: MouseEvent) => {
    event.stopPropagation();
    setExpandedNodeId((previous) => (previous === nodeId ? null : nodeId));
  };

  return {
    activeAlerts,
    alertsEnabled,
    handleSort,
    hasAnyTemperatureData,
    isExpandedNode: (nodeId: string) => expandedNodeId() === nodeId,
    isMobile,
    isTemperatureMonitoringEnabled: (node: Parameters<typeof isTemperatureMonitoringEnabled>[0]) =>
      isTemperatureMonitoringEnabled(node, props.globalTemperatureMonitoringEnabled ?? true),
    renderSortIndicator,
    sortedItems,
    temperatureThreshold,
    toggleNodeExpand,
    totalColumnCount,
  };
}

export type InfrastructureSummaryTableState = ReturnType<typeof useInfrastructureSummaryTableState>;
