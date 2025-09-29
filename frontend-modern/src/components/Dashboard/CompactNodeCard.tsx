import { Component, Show, createMemo } from 'solid-js';
import type { Node } from '@/types/api';
import { formatUptime } from '@/utils/format';
import { getAlertStyles, getResourceAlerts } from '@/utils/alerts';
import { AlertIndicator } from '@/components/shared/AlertIndicators';
import { useWebSocket } from '@/App';
import { Card } from '@/components/shared/Card';

interface CompactNodeCardProps {
  node: Node;
  variant: 'compact' | 'ultra-compact';
  onClick?: () => void;
  isSelected?: boolean;
}

const CompactNodeCard: Component<CompactNodeCardProps> = (props) => {
  const { activeAlerts } = useWebSocket();

  const isOnline = () => props.node.status === 'online' && props.node.uptime > 0;

  const cpuPercent = createMemo(() => Math.round(props.node.cpu * 100));
  const memPercent = createMemo(() => Math.round(props.node.memory?.usage || 0));
  const diskPercent = createMemo(() => {
    if (!props.node.disk || props.node.disk.total === 0) return 0;
    return Math.round((props.node.disk.used / props.node.disk.total) * 100);
  });

  const alertStyles = getAlertStyles(props.node.id || props.node.name, activeAlerts);
  const nodeAlerts = createMemo(() =>
    getResourceAlerts(props.node.id || props.node.name, activeAlerts),
  );

  // Get status color
  const getMetricColor = (value: number, type: 'cpu' | 'mem' | 'disk') => {
    const thresholds = {
      cpu: { high: 90, warn: 80 },
      mem: { high: 85, warn: 75 },
      disk: { high: 90, warn: 80 },
    };
    const t = thresholds[type];
    if (value >= t.high) return 'text-red-500';
    if (value >= t.warn) return 'text-yellow-500';
    return 'text-gray-600 dark:text-gray-400';
  };

  // Mini progress bar for compact mode
  const MiniProgressBar = (props: { value: number; type: 'cpu' | 'mem' | 'disk' }) => (
    <div class="w-[80px] h-2 bg-gray-200 dark:bg-gray-600 rounded-full overflow-hidden">
      <div
        class={`h-full transition-all ${
          props.value >= 90 ? 'bg-red-500' : props.value >= 75 ? 'bg-yellow-500' : 'bg-green-500'
        }`}
        style={{ width: `${props.value}%` }}
      />
    </div>
  );

  if (props.variant === 'ultra-compact') {
    // Single line format for 10+ nodes
    return (
      <Card
        padding="none"
        border={false}
        hoverable
        class={`flex items-center gap-2 px-3 py-1.5 ${
          props.isSelected
            ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
            : !isOnline()
              ? 'border-red-500'
              : alertStyles.hasAlert
                ? 'border-orange-500'
                : 'border-gray-200 dark:border-gray-700'
        } border transition-all cursor-pointer hover:scale-[1.01]`}
        onClick={props.onClick}
      >
        {/* Status dot */}
        <span
          class={`w-2 h-2 rounded-full ${
            props.node.connectionHealth === 'degraded'
              ? 'bg-yellow-500'
              : isOnline()
                ? 'bg-green-500'
                : 'bg-red-500'
          }`}
        />

        {/* Node name */}
        <a
          href={props.node.host || `https://${props.node.name}:8006`}
          target="_blank"
          class="font-medium text-sm w-24 truncate hover:text-blue-600 dark:hover:text-blue-400"
          title={props.node.name}
        >
          {props.node.name}
        </a>

        {/* Cluster/Standalone indicator */}
        <Show when={props.node.isClusterMember !== undefined}>
          <span
            class={`text-[9px] px-1 py-0.5 rounded-full font-medium ${
              props.node.isClusterMember
                ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
                : 'bg-gray-100 text-gray-600 dark:bg-gray-700/50 dark:text-gray-400'
            }`}
          >
            {props.node.isClusterMember ? props.node.clusterName?.slice(0, 3).toUpperCase() : 'SA'}
          </span>
        </Show>

        {/* Alert indicator */}
        <Show when={alertStyles.hasAlert}>
          <AlertIndicator severity={alertStyles.severity} alerts={nodeAlerts()} />
        </Show>

        {/* Metrics */}
        <div class="flex gap-4 text-xs font-mono">
          <span class={getMetricColor(cpuPercent(), 'cpu')}>
            C:{cpuPercent().toString().padStart(3)}%
          </span>
          <span class={getMetricColor(memPercent(), 'mem')}>
            M:{memPercent().toString().padStart(3)}%
          </span>
          <span class={getMetricColor(diskPercent(), 'disk')}>
            D:{diskPercent().toString().padStart(3)}%
          </span>
        </div>

        {/* Uptime */}
        <span class="ml-auto text-xs text-gray-500 dark:text-gray-400">
          ↑{formatUptime(props.node.uptime)}
        </span>
      </Card>
    );
  }

  // Compact bar format for 5-9 nodes
  return (
    <Card
      padding="sm"
      border={false}
      hoverable
      class={`border ${
        props.isSelected
          ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
          : !isOnline()
            ? 'border-red-500'
            : alertStyles.hasAlert
              ? 'border-orange-500'
              : 'border-gray-200 dark:border-gray-700'
      } cursor-pointer transition-all hover:scale-[1.02]`}
      onClick={props.onClick}
    >
      <div class="flex items-center justify-between mb-2">
        <div class="flex items-center gap-2">
          <span
            class={`w-2 h-2 rounded-full ${
              props.node.connectionHealth === 'degraded'
                ? 'bg-yellow-500'
                : isOnline()
                  ? 'bg-green-500'
                  : 'bg-red-500'
            }`}
          />
          <a
            href={props.node.host || `https://${props.node.name}:8006`}
            target="_blank"
            class="font-semibold text-sm hover:text-blue-600 dark:hover:text-blue-400"
          >
            {props.node.name}
          </a>
          {/* Cluster/Standalone indicator */}
          <Show when={props.node.isClusterMember !== undefined}>
            <span
              class={`text-[10px] px-1.5 py-0.5 rounded-full font-medium ${
                props.node.isClusterMember
                  ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
                  : 'bg-gray-100 text-gray-600 dark:bg-gray-700/50 dark:text-gray-400'
              }`}
            >
              {props.node.isClusterMember ? props.node.clusterName : 'Standalone'}
            </span>
          </Show>
          <Show when={alertStyles.hasAlert}>
            <AlertIndicator severity={alertStyles.severity} alerts={nodeAlerts()} />
          </Show>
        </div>
        <span class="text-xs text-gray-500 dark:text-gray-400">
          {formatUptime(props.node.uptime)}
        </span>
      </div>

      {/* Metric bars */}
      <div class="space-y-1.5">
        <div class="flex items-center gap-2">
          <span class="text-xs w-8 text-gray-600 dark:text-gray-400">CPU</span>
          <MiniProgressBar value={cpuPercent()} type="cpu" />
          <span class={`text-xs ${getMetricColor(cpuPercent(), 'cpu')}`}>{cpuPercent()}%</span>
        </div>
        <div class="flex items-center gap-2">
          <span class="text-xs w-8 text-gray-600 dark:text-gray-400">Mem</span>
          <MiniProgressBar value={memPercent()} type="mem" />
          <span class={`text-xs ${getMetricColor(memPercent(), 'mem')}`}>{memPercent()}%</span>
        </div>
        <div class="flex items-center gap-2">
          <span class="text-xs w-8 text-gray-600 dark:text-gray-400">Disk</span>
          <MiniProgressBar value={diskPercent()} type="disk" />
          <span class={`text-xs ${getMetricColor(diskPercent(), 'disk')}`}>{diskPercent()}%</span>
        </div>
      </div>
    </Card>
  );
};

export default CompactNodeCard;
