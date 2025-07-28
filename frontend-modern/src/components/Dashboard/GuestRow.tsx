import { Show, createMemo } from 'solid-js';
import type { VM, Container } from '@/types/api';
import { AlertIndicator, AlertCountBadge } from '@/components/shared/AlertIndicators';
import { formatBytes, formatUptime } from '@/utils/format';
import { MetricBar } from './MetricBar';
import { IOMetric } from './IOMetric';
import { DynamicChart } from '@/components/shared/DynamicChart';
import { getGuestChartData } from '@/stores/charts';
import { getResourceAlerts } from '@/utils/alerts';
import { useWebSocket } from '@/App';

type Guest = VM | Container;

// Type guard for VM vs Container
const isVM = (guest: Guest): guest is VM => {
  return guest.type === 'qemu';
};

type DisplayMode = 'standard' | 'charts' | 'alerts';
type TimeRange = '5m' | '15m' | '30m' | '1h' | '4h' | '12h' | '24h' | '7d';

interface GuestRowProps {
  guest: Guest;
  showNode?: boolean;
  displayMode?: DisplayMode;
  timeRange?: TimeRange;
  chartDataLoading?: boolean;
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
  
  
  // Get guest ID for chart data lookup - must match the format from the API
  const guestId = createMemo(() => props.guest.id || `${props.guest.instance}-${props.guest.name}-${props.guest.vmid}`);
  
  // Get chart data from store
  const cpuHistory = createMemo(() => getGuestChartData(guestId(), 'cpu'));
  const memHistory = createMemo(() => getGuestChartData(guestId(), 'memory'));
  const diskHistory = createMemo(() => getGuestChartData(guestId(), 'disk'));
  const diskReadHistory = createMemo(() => getGuestChartData(guestId(), 'diskread'));
  const diskWriteHistory = createMemo(() => getGuestChartData(guestId(), 'diskwrite'));
  const netInHistory = createMemo(() => getGuestChartData(guestId(), 'netin'));
  const netOutHistory = createMemo(() => getGuestChartData(guestId(), 'netout'));
  
  // Legacy fetch code removed - chart data is now fetched globally by the Dashboard

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
              <AlertIndicator severity={props.alertStyles!.severity} alerts={guestAlerts()} />
              <Show when={props.alertStyles!.alertCount > 1}>
                <AlertCountBadge count={props.alertStyles!.alertCount} severity={props.alertStyles!.severity!} alerts={guestAlerts()} />
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
      <td class="p-1 px-2 text-sm text-gray-600 dark:text-gray-400 whitespace-nowrap">
        <Show when={isRunning()} fallback="-">
          {formatUptime(props.guest.uptime)}
        </Show>
      </td>

      {/* CPU */}
      <td class="p-1 px-2 w-[140px]">
        <Show when={props.displayMode === 'alerts'}>
          <div class="flex items-center gap-2">
            <input
              type="range"
              min="0"
              max="100"
              value={cpuPercent()}
              disabled
              class="w-16 h-1.5 opacity-50"
              style={`background: linear-gradient(to right, #3b82f6 0%, #3b82f6 ${cpuPercent()}%, #e5e7eb ${cpuPercent()}%, #e5e7eb 100%)`}
            />
            <input
              type="number"
              min="0"
              max="100"
              placeholder="0"
              class="w-12 px-1 py-0.5 text-xs border rounded dark:bg-gray-700 dark:border-gray-600"
              title="Set CPU alert threshold for this guest"
            />
          </div>
        </Show>
        <Show when={props.displayMode === 'charts' && isRunning()}>
          <DynamicChart 
            data={cpuHistory()} 
            metric="cpu"
            guestId={guestId()}
            chartType="mini"
            paddingAdjustment={8}
            forceGray={true}
          />
        </Show>
        <Show when={props.displayMode === 'standard'}>
          <MetricBar 
            value={cpuPercent()} 
            label={`${cpuPercent().toFixed(0)}%`}
            sublabel={props.guest.cpus ? `${(props.guest.cpu * props.guest.cpus).toFixed(1)}/${props.guest.cpus} cores` : undefined}
            type="cpu"
          />
        </Show>
      </td>

      {/* Memory */}
      <td class="p-1 px-2 w-[140px]">
        <Show when={props.displayMode === 'alerts'}>
          <div class="flex items-center gap-2">
            <input
              type="range"
              min="0"
              max="100"
              value={memPercent()}
              disabled
              class="w-16 h-1.5 opacity-50"
              style={`background: linear-gradient(to right, #8b5cf6 0%, #8b5cf6 ${memPercent()}%, #e5e7eb ${memPercent()}%, #e5e7eb 100%)`}
            />
            <input
              type="number"
              min="0"
              max="100"
              placeholder="0"
              class="w-12 px-1 py-0.5 text-xs border rounded dark:bg-gray-700 dark:border-gray-600"
              title="Set memory alert threshold for this guest"
            />
          </div>
        </Show>
        <Show when={props.displayMode === 'charts' && isRunning()}>
          <DynamicChart 
            data={memHistory()} 
            metric="memory"
            guestId={guestId()}
            chartType="mini"
            paddingAdjustment={8}
            filled
            forceGray={true}
          />
        </Show>
        <Show when={props.displayMode === 'standard'}>
          <MetricBar 
            value={memPercent()} 
            label={`${memPercent().toFixed(0)}%`}
            sublabel={props.guest.memory ? `${formatBytes(props.guest.memory.used)}/${formatBytes(props.guest.memory.total)}` : undefined}
            type="memory"
          />
        </Show>
      </td>

      {/* Disk */}
      <td class="p-1 px-2 w-[140px]">
        <Show when={props.displayMode === 'alerts' && props.guest.disk && props.guest.disk.total > 0}>
          <div class="flex items-center gap-2">
            <input
              type="range"
              min="0"
              max="100"
              value={diskPercent()}
              disabled
              class="w-16 h-1.5 opacity-50"
              style={`background: linear-gradient(to right, #f59e0b 0%, #f59e0b ${diskPercent()}%, #e5e7eb ${diskPercent()}%, #e5e7eb 100%)`}
            />
            <input
              type="number"
              min="0"
              max="100"
              placeholder="0"
              class="w-12 px-1 py-0.5 text-xs border rounded dark:bg-gray-700 dark:border-gray-600"
              title="Set disk alert threshold for this guest"
            />
          </div>
        </Show>
        <Show when={props.displayMode === 'charts' && isRunning() && props.guest.disk && props.guest.disk.total > 0}>
          <DynamicChart 
            data={diskHistory()} 
            metric="disk"
            guestId={guestId()}
            chartType="mini"
            paddingAdjustment={8}
            filled
            forceGray={true}
          />
        </Show>
        <Show when={props.displayMode === 'standard'}>
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
        </Show>
      </td>

      {/* Disk I/O */}
      <td class="p-1 px-2">
        <Show when={props.displayMode === 'alerts'}>
          <select class="w-full px-1 py-0.5 text-xs border rounded dark:bg-gray-700 dark:border-gray-600">
            <option value="0">Off</option>
            <option value="1">1 MB/s</option>
            <option value="10">10 MB/s</option>
            <option value="100">100 MB/s</option>
          </select>
        </Show>
        <Show when={props.displayMode === 'charts' && isRunning()}>
          <DynamicChart 
            data={diskReadHistory()} 
            metric="diskread"
            guestId={guestId()}
            chartType="sparkline"
            paddingAdjustment={8}
            forceGray={true}
          />
        </Show>
        <Show when={props.displayMode === 'standard'}>
          <IOMetric value={props.guest.diskRead} disabled={!isRunning()} />
        </Show>
      </td>
      <td class="p-1 px-2">
        <Show when={props.displayMode === 'alerts'}>
          <select class="w-full px-1 py-0.5 text-xs border rounded dark:bg-gray-700 dark:border-gray-600">
            <option value="0">Off</option>
            <option value="1">1 MB/s</option>
            <option value="10">10 MB/s</option>
            <option value="100">100 MB/s</option>
          </select>
        </Show>
        <Show when={props.displayMode === 'charts' && isRunning()}>
          <DynamicChart 
            data={diskWriteHistory()} 
            metric="diskwrite"
            guestId={guestId()}
            chartType="sparkline"
            paddingAdjustment={8}
            forceGray={true}
          />
        </Show>
        <Show when={props.displayMode === 'standard'}>
          <IOMetric value={props.guest.diskWrite} disabled={!isRunning()} />
        </Show>
      </td>

      {/* Network I/O */}
      <td class="p-1 px-2">
        <Show when={props.displayMode === 'alerts'}>
          <select class="w-full px-1 py-0.5 text-xs border rounded dark:bg-gray-700 dark:border-gray-600">
            <option value="0">Off</option>
            <option value="1">1 MB/s</option>
            <option value="10">10 MB/s</option>
            <option value="100">100 MB/s</option>
          </select>
        </Show>
        <Show when={props.displayMode === 'charts' && isRunning()}>
          <DynamicChart 
            data={netInHistory()} 
            metric="netin"
            guestId={guestId()}
            chartType="sparkline"
            paddingAdjustment={8}
            forceGray={true}
          />
        </Show>
        <Show when={props.displayMode === 'standard'}>
          <IOMetric value={props.guest.networkIn} disabled={!isRunning()} />
        </Show>
      </td>
      <td class="p-1 px-2">
        <Show when={props.displayMode === 'alerts'}>
          <select class="w-full px-1 py-0.5 text-xs border rounded dark:bg-gray-700 dark:border-gray-600">
            <option value="0">Off</option>
            <option value="1">1 MB/s</option>
            <option value="10">10 MB/s</option>
            <option value="100">100 MB/s</option>
          </select>
        </Show>
        <Show when={props.displayMode === 'charts' && isRunning()}>
          <DynamicChart 
            data={netOutHistory()} 
            metric="netout"
            guestId={guestId()}
            chartType="sparkline"
            paddingAdjustment={8}
            forceGray={true}
          />
        </Show>
        <Show when={props.displayMode === 'standard'}>
          <IOMetric value={props.guest.networkOut} disabled={!isRunning()} />
        </Show>
      </td>

    </tr>
  );
}