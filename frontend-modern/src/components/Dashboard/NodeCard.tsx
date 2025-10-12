import { Component, Show, createMemo, createEffect } from 'solid-js';
import type { Node } from '@/types/api';
import { formatUptime } from '@/utils/format';
import { getAlertStyles, getResourceAlerts } from '@/utils/alerts';
import { AlertIndicator, AlertCountBadge } from '@/components/shared/AlertIndicators';
import { useWebSocket } from '@/App';
import { Card } from '@/components/shared/Card';
import { getNodeDisplayName, hasAlternateDisplayName } from '@/utils/nodes';
import { getPrimaryTemperature } from '@/utils/temperature';

interface NodeCardProps {
  node: Node;
  isSelected?: boolean;
}

const NodeCard: Component<NodeCardProps> = (props) => {
  const { activeAlerts } = useWebSocket();
  // Early return if node data is incomplete
  if (!props.node || !props.node.memory || !props.node.disk) {
    return (
      <Card padding="sm" class="flex w-[180px] flex-col gap-1">
        <div class="text-sm text-gray-500">Loading node data...</div>
      </Card>
    );
  }

  const isOnline = () =>
    props.node.status === 'online' &&
    props.node.uptime > 0 &&
    props.node.connectionHealth !== 'error';

  // Memoize CPU percent to avoid multiple calculations
  const cpuPercent = createMemo(() => {
    const percent = Math.round(props.node.cpu * 100);
    return percent;
  });

  // Track CPU updates (logging removed for cleaner output)
  createEffect(() => {
    cpuPercent(); // Just track the value changes
  });

  const memPercent = createMemo(() => {
    if (!props.node.memory) return 0;
    // Use the pre-calculated usage percentage from the backend
    return Math.round(props.node.memory.usage || 0);
  });

  const diskPercent = createMemo(() => {
    if (!props.node.disk || props.node.disk.total === 0) return 0;
    return Math.round((props.node.disk.used / props.node.disk.total) * 100);
  });

  const displayName = () => getNodeDisplayName(props.node);
  const showActualName = () => hasAlternateDisplayName(props.node);

  // Calculate normalized load (load average / cpu count)
  const normalizedLoad = () => {
    if (props.node.loadAverage && props.node.loadAverage.length > 0) {
      const load1m = props.node.loadAverage[0];
      if (typeof load1m === 'number' && !isNaN(load1m)) {
        // Use CPU cores from cpuInfo if available, otherwise assume 4
        const cpuCount = props.node.cpuInfo?.cores || 4;
        return (load1m / cpuCount).toFixed(2);
      }
    }
    return 'N/A';
  };

  // Helper function to create compact progress bar
  const createProgressBar = (percentage: number, label: string, colorClass: string) => {
    const bgColorClass = 'bg-gray-200 dark:bg-gray-600';
    const progressColorClass =
      {
        red: 'bg-red-500/70 dark:bg-red-500/60',
        yellow: 'bg-yellow-500/70 dark:bg-yellow-500/60',
        green: 'bg-green-500/70 dark:bg-green-500/60',
      }[colorClass] || 'bg-gray-500/70 dark:bg-gray-500/60';

    return (
      <div class="w-[140px]">
        <div class="flex justify-between items-center mb-0.5">
          <span class="text-[10px] font-medium text-gray-600 dark:text-gray-400">{label}</span>
          <span class="text-[10px] font-medium text-gray-700 dark:text-gray-300">
            {percentage}%
          </span>
        </div>
        <div class={`relative w-full h-2 rounded-full overflow-hidden ${bgColorClass}`}>
          <div
            class={`absolute top-0 left-0 h-full transition-all duration-300 ${progressColorClass}`}
            style={{ width: `${percentage}%` }}
          />
        </div>
      </div>
    );
  };

  // Get color based on percentage and metric type
  const getColor = (percentage: number, metric: 'cpu' | 'memory' | 'disk') => {
    if (metric === 'cpu') {
      if (percentage >= 90) return 'red';
      if (percentage >= 80) return 'yellow';
      return 'green';
    } else if (metric === 'memory') {
      if (percentage >= 85) return 'red';
      if (percentage >= 75) return 'yellow';
      return 'green';
    } else if (metric === 'disk') {
      if (percentage >= 90) return 'red';
      if (percentage >= 80) return 'yellow';
      return 'green';
    }
    return 'green';
  };

  const alertStyles = getAlertStyles(props.node.id || props.node.name, activeAlerts);
  const nodeAlerts = createMemo(() =>
    getResourceAlerts(props.node.id || props.node.name, activeAlerts),
  );
  const unacknowledgedNodeAlerts = createMemo(() => nodeAlerts().filter((alert) => !alert.acknowledged));

  const primaryTemperature = createMemo(() => getPrimaryTemperature(props.node.temperature));
  const primaryTemperatureValue = createMemo(() => {
    const reading = primaryTemperature();
    return reading ? Math.round(reading.value) : null;
  });
  const primaryTemperatureLabel = createMemo(() => {
    const reading = primaryTemperature();
    if (!reading) return null;
    if (reading.source === 'nvme') {
      return reading.device ?? 'NVMe';
    }
    return 'CPU';
  });
  const temperatureTooltip = createMemo(() => {
    const temp = props.node.temperature;
    const rounded = primaryTemperatureValue();
    if (!temp?.available || rounded === null) {
      return '';
    }
    const label = primaryTemperatureLabel();
    const primaryLabel =
      label && label !== 'CPU' ? `${label}: ${rounded}°C` : `CPU: ${rounded}°C`;
    const nvmeDetails =
      temp.nvme && temp.nvme.length > 0
        ? ` | NVMe: ${temp.nvme.map((n) => `${n.device}: ${Math.round(n.temp)}°C`).join(', ')}`
        : '';
    return `${primaryLabel}${nvmeDetails}`;
  });

  // Determine border/ring style based on status and alerts
  const getBorderClass = () => {
    // Selected nodes get blue ring
    if (props.isSelected) {
      return 'ring-2 ring-blue-500 border-blue-200 dark:border-blue-500';
    }
    // Offline nodes get red ring
    if (!isOnline()) {
      return 'ring-2 ring-red-500 border-red-200 dark:border-red-600';
    }
    // Alert nodes get colored ring based on severity
    if (alertStyles.hasUnacknowledgedAlert) {
      return alertStyles.severity === 'critical'
        ? 'ring-2 ring-red-500 border-red-200 dark:border-red-600'
        : 'ring-2 ring-orange-500 border-orange-200 dark:border-orange-500';
    }
    if (alertStyles.hasAcknowledgedOnlyAlert) {
      return 'ring-2 ring-gray-400 border-gray-200 dark:border-gray-600 dark:ring-gray-500';
    }
    // Normal nodes get standard border
    return '';
  };

  // Get background class from alert styles but remove the border-l-4 part
  const getBackgroundClass = () => {
    if (!alertStyles.rowClass) return '';
    // Remove border classes from rowClass to avoid conflicts
    return alertStyles.rowClass.replace(/border-[^\s]+/g, '').trim();
  };

  return (
    <Card
      padding="sm"
      class={`flex w-[180px] flex-col gap-2 ${getBorderClass()} ${getBackgroundClass()}`.trim()}
      hoverable
    >
      {/* Header */}
      <div class="flex justify-between items-center">
        <h3 class="text-xs font-semibold truncate text-gray-800 dark:text-gray-200 flex items-center gap-1">
          <a
            href={
              props.node.host ||
              (props.node.name.includes(':')
                ? `https://${props.node.name}`
                : `https://${props.node.name}:8006`)
            }
            target="_blank"
            rel="noopener noreferrer"
            class="hover:text-blue-600 dark:hover:text-blue-400 transition-colors duration-150 cursor-pointer"
            title={`Open ${props.node.name} web interface`}
          >
            {displayName()}
          </a>
          <Show when={showActualName()}>
            <span class="text-[10px] text-gray-500 dark:text-gray-400">({props.node.name})</span>
          </Show>
          {/* Cluster/Standalone indicator - more compact */}
          <Show when={props.node.isClusterMember !== undefined}>
            <span
              class={`text-[9px] px-1 py-0.5 rounded-full font-medium ${
                props.node.isClusterMember
                  ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
                  : 'bg-gray-100 text-gray-600 dark:bg-gray-700/50 dark:text-gray-400'
              }`}
              title={props.node.isClusterMember ? props.node.clusterName : 'Standalone'}
            >
              {props.node.isClusterMember ? 'C' : 'S'}
            </span>
          </Show>
          <Show when={alertStyles.hasUnacknowledgedAlert}>
            <div class="flex items-center gap-1">
              <AlertIndicator severity={alertStyles.severity} alerts={unacknowledgedNodeAlerts()} />
              <Show when={(alertStyles.unacknowledgedCount || 0) > 1}>
                <AlertCountBadge
                  count={alertStyles.unacknowledgedCount || 0}
                  severity={alertStyles.severity!}
                  alerts={unacknowledgedNodeAlerts()}
                />
              </Show>
            </div>
          </Show>
        </h3>
        <span
          class={`h-2 w-2 rounded-full flex-shrink-0 ${isOnline() ? 'bg-green-500' : 'bg-red-500'}`}
          title={isOnline() ? 'Online' : 'Offline'}
        />
      </div>

      {/* Metrics - Compact */}
      <div class="space-y-1.5">
        {createProgressBar(cpuPercent(), 'CPU', getColor(cpuPercent(), 'cpu'))}
        {createProgressBar(memPercent(), 'Memory', getColor(memPercent(), 'memory'))}
        {createProgressBar(diskPercent(), 'Disk', getColor(diskPercent(), 'disk'))}
      </div>

      {/* Footer Info - More compact */}
      <div class="flex justify-between text-[9px] text-gray-500 dark:text-gray-400 mt-1">
        <span title={`Uptime: ${formatUptime(props.node.uptime)}`}>
          ↑{formatUptime(props.node.uptime)}
        </span>
        <Show
          when={props.node.temperature?.available && primaryTemperatureValue() !== null}
          fallback={<span title={`Load: ${normalizedLoad()}`}>⚡{normalizedLoad()}</span>}
        >
          <span
            class={`font-medium ${
              (primaryTemperatureValue() ?? 0) > 80
                ? 'text-red-500'
                : (primaryTemperatureValue() ?? 0) > 60
                  ? 'text-yellow-500'
                  : 'text-green-500'
            }`}
            title={temperatureTooltip() || undefined}
          >
            🌡{primaryTemperatureValue()}°C
            <Show when={primaryTemperatureLabel() && primaryTemperatureLabel() !== 'CPU'}>
              <span class="ml-1 text-[9px] uppercase text-gray-500 dark:text-gray-400">
                {primaryTemperatureLabel()}
              </span>
            </Show>
          </span>
        </Show>
      </div>
    </Card>
  );
};

export default NodeCard;
