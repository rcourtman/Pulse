import { Show, createMemo } from 'solid-js';
import type { VM, Container } from '@/types/api';
import { AlertIndicator, AlertCountBadge } from '@/components/shared/AlertIndicators';
import { formatBytes, formatUptime } from '@/utils/format';
import { MetricBar } from './MetricBar';
import { IOMetric } from './IOMetric';
import { getResourceAlerts } from '@/utils/alerts';
import { useWebSocket } from '@/App';

type Guest = VM | Container;

// Type guard for VM vs Container
const isVM = (guest: Guest): guest is VM => {
  return guest.type === 'qemu';
};


interface GuestRowProps {
  guest: Guest;
  showNode?: boolean;
  alertStyles?: {
    rowClass: string;
    indicatorClass: string;
    badgeClass: string;
    hasAlert: boolean;
    alertCount: number;
    severity: 'critical' | 'warning' | null;
  };
}

export function GuestRow(props: GuestRowProps) {
  const { activeAlerts } = useWebSocket();
  
  const cpuPercent = createMemo(() => (props.guest.cpu || 0) * 100);
  const memPercent = createMemo(() => {
    if (!props.guest.memory) return 0;
    // Use the pre-calculated usage percentage from the backend
    return props.guest.memory.usage || 0;
  });
  const diskPercent = createMemo(() => {
    if (!props.guest.disk || props.guest.disk.total === 0) return 0;
    return (props.guest.disk.used / props.guest.disk.total) * 100;
  });

  const isRunning = createMemo(() => props.guest.status === 'running');
  
  // Get alerts for this guest
  const guestAlerts = createMemo(() => {
    const guestId = props.guest.id || `${props.guest.instance}-${props.guest.name}-${props.guest.vmid}`;
    return getResourceAlerts(guestId, activeAlerts);
  });
  

  // Get row styling - include alert styles if present
  const rowClass = createMemo(() => {
    const base = 'transition-all duration-200';
    const hover = 'hover:shadow-sm';
    const alertClass = props.alertStyles?.rowClass || '';
    const defaultHover = alertClass ? '' : 'hover:bg-gray-50 dark:hover:bg-gray-700';
    return `${base} ${hover} ${defaultHover} ${alertClass}`;
  });

  return (
    <tr class={rowClass()}>
      {/* Name - Sticky column */}
      <td class="p-1 px-2 whitespace-nowrap">
        <div class="flex items-center gap-2">
          {/* Status indicator */}
          <span class={`h-2 w-2 rounded-full flex-shrink-0 ${
            isRunning() ? 'bg-green-500' : 'bg-gray-400'
          }`} title={props.guest.status}></span>
          
          {/* Name */}
          <span class="font-medium text-gray-900 dark:text-gray-100 truncate" title={props.guest.name}>
            {props.guest.name}
          </span>
          
          {/* Alert indicators */}
          <Show when={props.alertStyles?.hasAlert}>
            <div class="flex items-center gap-1">
              <AlertIndicator severity={props.alertStyles?.severity || null} alerts={guestAlerts()} />
              <Show when={props.alertStyles?.alertCount && props.alertStyles.alertCount > 1}>
                <AlertCountBadge count={props.alertStyles!.alertCount} severity={props.alertStyles!.severity || 'warning'} alerts={guestAlerts()} />
              </Show>
            </div>
          </Show>
        </div>
      </td>

      {/* Type */}
      <td class="p-1 px-2 whitespace-nowrap">
        <span class={`inline-block px-1.5 py-0.5 text-xs font-medium rounded ${
          props.guest.type === 'qemu' 
            ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300' 
            : 'bg-green-100 text-green-700 dark:bg-green-900/50 dark:text-green-300'
        }`}>
          {isVM(props.guest) ? 'VM' : 'LXC'}
        </span>
      </td>

      {/* VMID */}
      <td class="p-1 px-2 whitespace-nowrap text-sm text-gray-600 dark:text-gray-400">
        {props.guest.vmid}
      </td>


      {/* Node (optional) */}
      <Show when={props.showNode}>
        <td class="p-1 px-2 text-sm text-gray-600 dark:text-gray-400">
          {props.guest.node}
        </td>
      </Show>

      {/* Uptime */}
      <td class={`p-1 px-2 text-sm whitespace-nowrap ${
        props.guest.uptime < 3600 ? 'text-orange-500' : 'text-gray-600 dark:text-gray-400'
      }`}>
        <Show when={isRunning()} fallback="-">
          {formatUptime(props.guest.uptime)}
        </Show>
      </td>

      {/* CPU */}
      <td class="p-1 px-2 w-[140px]">
        <MetricBar 
          value={cpuPercent()} 
          label={`${cpuPercent().toFixed(0)}%`}
          sublabel={props.guest.cpus ? `${((props.guest.cpu || 0) * props.guest.cpus).toFixed(1)}/${props.guest.cpus} cores` : undefined}
          type="cpu"
        />
      </td>

      {/* Memory */}
      <td class="p-1 px-2 w-[140px]">
        <MetricBar 
          value={memPercent()} 
          label={`${memPercent().toFixed(0)}%`}
          sublabel={props.guest.memory ? `${formatBytes(props.guest.memory.used)}/${formatBytes(props.guest.memory.total)}` : undefined}
          type="memory"
        />
      </td>

      {/* Disk */}
      <td class="p-1 px-2 w-[140px]">
        <Show 
          when={props.guest.disk && props.guest.disk.total > 0}
          fallback={<span class="text-gray-400 text-sm">-</span>}
        >
          <MetricBar 
            value={diskPercent()} 
            label={`${diskPercent().toFixed(0)}%`}
            sublabel={props.guest.disk ? `${formatBytes(props.guest.disk.used)}/${formatBytes(props.guest.disk.total)}` : undefined}
            type="disk"
          />
        </Show>
      </td>

      {/* Disk I/O */}
      <td class="p-1 px-2">
        <IOMetric value={props.guest.diskRead} disabled={!isRunning()} />
      </td>
      <td class="p-1 px-2">
        <IOMetric value={props.guest.diskWrite} disabled={!isRunning()} />
      </td>

      {/* Network I/O */}
      <td class="p-1 px-2">
        <IOMetric value={props.guest.networkIn} disabled={!isRunning()} />
      </td>
      <td class="p-1 px-2">
        <IOMetric value={props.guest.networkOut} disabled={!isRunning()} />
      </td>

    </tr>
  );
}