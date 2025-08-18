import { Component, Show, createMemo, createEffect } from 'solid-js';
import type { Node } from '@/types/api';
import { formatUptime, formatBytes } from '@/utils/format';
import { getAlertStyles, getResourceAlerts } from '@/utils/alerts';
import { AlertIndicator, AlertCountBadge } from '@/components/shared/AlertIndicators';
import { useWebSocket } from '@/App';

interface NodeCardProps {
  node: Node;
  isSelected?: boolean;
}

const NodeCard: Component<NodeCardProps> = (props) => {
  const { activeAlerts } = useWebSocket();
  // Early return if node data is incomplete
  if (!props.node || !props.node.memory || !props.node.disk) {
    return (
      <div class="bg-white dark:bg-gray-800 shadow-md rounded-lg p-2 border border-gray-200 dark:border-gray-700 flex flex-col gap-1 min-w-[250px]">
        <div class="text-sm text-gray-500">Loading node data...</div>
      </div>
    );
  }
  
  const isOnline = () => props.node.status === 'online' && props.node.uptime > 0 && props.node.connectionHealth !== 'error';
  
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

  // Helper function to create progress bar with text overlay (matching original)
  const createProgressBar = (percentage: number, text: string, colorClass: string) => {
    const bgColorClass = 'bg-gray-200 dark:bg-gray-600';
    const progressColorClass = {
      'red': 'bg-red-500/60 dark:bg-red-500/50',
      'yellow': 'bg-yellow-500/60 dark:bg-yellow-500/50',
      'green': 'bg-green-500/60 dark:bg-green-500/50'
    }[colorClass] || 'bg-gray-500/60 dark:bg-gray-500/50';
    
    return (
      <div class={`relative w-full h-3.5 rounded overflow-hidden ${bgColorClass}`}>
        <div class={`absolute top-0 left-0 h-full ${progressColorClass}`} style={{ width: `${percentage}%` }} />
        <span class="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-gray-800 dark:text-gray-100 leading-none">
          <span class="truncate px-1">{text}</span>
        </span>
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

  // Format CPU text with cores info
  const cpuText = () => {
    const cores = props.node.cpuInfo?.cores;
    const cpuUsed = cores ? (props.node.cpu * cores).toFixed(1) : '0';
    return cores && cores > 0 
      ? `${cpuPercent()}% (${cpuUsed}/${cores} cores)`
      : `${cpuPercent()}%`;
  };

  // Format memory text with size info  
  const memoryText = () => {
    if (!props.node.memory) return '0%';
    return `${memPercent()}% (${formatBytes(props.node.memory.used)}/${formatBytes(props.node.memory.total)})`;
  };

  // Format disk text with size info
  const diskText = () => {
    if (!props.node.disk) return '0%';
    return `${diskPercent()}% (${formatBytes(props.node.disk.used)}/${formatBytes(props.node.disk.total)})`;
  };

  const alertStyles = getAlertStyles(props.node.id || props.node.name, activeAlerts);
  const nodeAlerts = createMemo(() => getResourceAlerts(props.node.id || props.node.name, activeAlerts));
  
  // Determine border/ring style based on status and alerts
  const getBorderClass = () => {
    // Selected nodes get blue ring
    if (props.isSelected) {
      return 'ring-2 ring-blue-500 border border-gray-200 dark:border-gray-700';
    }
    // Offline nodes get red ring
    if (!isOnline()) {
      return 'ring-2 ring-red-500 border border-gray-200 dark:border-gray-700';
    }
    // Alert nodes get colored ring based on severity
    if (alertStyles.hasAlert) {
      return alertStyles.severity === 'critical' 
        ? 'ring-2 ring-red-500 border border-gray-200 dark:border-gray-700' 
        : 'ring-2 ring-orange-500 border border-gray-200 dark:border-gray-700';
    }
    // Normal nodes get standard border
    return 'border border-gray-200 dark:border-gray-700';
  };
  
  // Get background class from alert styles but remove the border-l-4 part
  const getBackgroundClass = () => {
    if (!alertStyles.rowClass) return '';
    // Remove border classes from rowClass to avoid conflicts
    return alertStyles.rowClass.replace(/border-[^\s]+/g, '').trim();
  };
  
  return (
    <div class={`bg-white dark:bg-gray-800 shadow-md rounded-lg p-2 flex flex-col gap-1 min-w-[250px] ${getBorderClass()} ${getBackgroundClass()}`}>
      {/* Header */}
      <div class="flex justify-between items-center">
        <h3 class="text-sm font-semibold truncate text-gray-800 dark:text-gray-200 flex items-center gap-2">
          <a 
            href={props.node.host || (props.node.name.includes(':') ? `https://${props.node.name}` : `https://${props.node.name}:8006`)} 
            target="_blank" 
            rel="noopener noreferrer" 
            class="hover:text-blue-600 dark:hover:text-blue-400 transition-colors duration-150 cursor-pointer"
            title={`Open ${props.node.name} web interface`}
          >
            {props.node.name}
          </a>
          <Show when={alertStyles.hasAlert}>
            <div class="flex items-center gap-1">
              <AlertIndicator severity={alertStyles.severity} alerts={nodeAlerts()} />
              <Show when={alertStyles.alertCount > 1}>
                <AlertCountBadge count={alertStyles.alertCount} severity={alertStyles.severity!} alerts={nodeAlerts()} />
              </Show>
            </div>
          </Show>
        </h3>
        <div class="flex items-center">
          <span class={`h-2.5 w-2.5 rounded-full mr-1.5 flex-shrink-0 ${
            isOnline() ? 'bg-green-500' : 'bg-red-500'
          }`} />
          <span class="text-xs capitalize text-gray-600 dark:text-gray-400">
            {isOnline() ? 'online' : props.node.status || 'unknown'}
          </span>
        </div>
      </div>

      {/* CPU */}
      <div class="text-[11px] text-gray-600 dark:text-gray-400">
        <span class="font-medium">CPU:</span>
        {createProgressBar(cpuPercent(), cpuText(), getColor(cpuPercent(), 'cpu'))}
      </div>

      {/* Memory */}
      <div class="text-[11px] text-gray-600 dark:text-gray-400">
        <span class="font-medium">Mem:</span>
        {createProgressBar(memPercent(), memoryText(), getColor(memPercent(), 'memory'))}
      </div>

      {/* Disk */}
      <div class="text-[11px] text-gray-600 dark:text-gray-400">
        <span class="font-medium">Disk:</span>
        {createProgressBar(diskPercent(), diskText(), getColor(diskPercent(), 'disk'))}
      </div>

      {/* Footer Info */}
      <div class="flex justify-between text-[11px] text-gray-500 dark:text-gray-400 pt-0.5">
        <span>Uptime: {formatUptime(props.node.uptime)}</span>
        <span>Load: {normalizedLoad()}</span>
      </div>
    </div>
  );
};

export default NodeCard;