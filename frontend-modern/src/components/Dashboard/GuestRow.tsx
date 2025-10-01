import { Show, createMemo, createSignal, createEffect } from 'solid-js';
import type { VM, Container } from '@/types/api';
import { formatBytes, formatUptime } from '@/utils/format';
import { MetricBar } from './MetricBar';
import { IOMetric } from './IOMetric';
import { TagBadges } from './TagBadges';

type Guest = VM | Container;

// Type guard for VM vs Container
const isVM = (guest: Guest): guest is VM => {
  return guest.type === 'qemu';
};

interface GuestRowProps {
  guest: Guest;
  alertStyles?: {
    rowClass: string;
    indicatorClass: string;
    badgeClass: string;
    hasAlert: boolean;
    alertCount: number;
    severity: 'critical' | 'warning' | null;
  };
  customUrl?: string;
  onTagClick?: (tag: string) => void;
  activeSearch?: string;
}

export function GuestRow(props: GuestRowProps) {
  const [customUrl, setCustomUrl] = createSignal<string | undefined>(props.customUrl);

  // Update custom URL when prop changes
  createEffect(() => {
    setCustomUrl(props.customUrl);
  });

  // Load custom URL from backend if not provided via props
  onMount(async () => {
    if (!props.customUrl) {
      const startTime = performance.now();
      try {
        const metadata = await GuestMetadataAPI.getMetadata(guestId());
        const endTime = performance.now();
        console.log(`[PERF] Individual metadata call for ${guestId()} took ${(endTime - startTime).toFixed(2)}ms`);
        if (metadata && metadata.customUrl) {
          setCustomUrl(metadata.customUrl);
        }
      } catch (err) {
        // Silently fail - not critical for display
        console.debug('Failed to load guest metadata:', err);
      }
    }
  });
  const cpuPercent = createMemo(() => (props.guest.cpu || 0) * 100);
  const memPercent = createMemo(() => {
    if (!props.guest.memory) return 0;
    // Use the pre-calculated usage percentage from the backend
    return props.guest.memory.usage || 0;
  });
  const diskPercent = createMemo(() => {
    if (!props.guest.disk || props.guest.disk.total === 0) return 0;
    // Check if usage is -1 (unknown/no guest agent)
    if (props.guest.disk.usage === -1) return -1;
    return (props.guest.disk.used / props.guest.disk.total) * 100;
  });

  const isRunning = createMemo(() => props.guest.status === 'running');

  // Get helpful tooltip for disk status
  const getDiskStatusTooltip = () => {
    if (!isVM(props.guest)) return 'Disk stats unavailable';

    const vm = props.guest as VM;
    const reason = vm.diskStatusReason;

    switch (reason) {
      case 'agent-not-running':
        return 'Guest agent not running. Install and start qemu-guest-agent in the VM.';
      case 'agent-timeout':
        return 'Guest agent timeout. Agent may need to be restarted.';
      case 'permission-denied':
        return 'Permission denied. Check that your Pulse user/token has VM.Monitor permission (PVE 8) or VM.GuestAgent.Audit permission (PVE 9).';
      case 'agent-disabled':
        return 'Guest agent is disabled in VM configuration. Enable it in VM Options.';
      case 'no-filesystems':
        return 'No filesystems found. VM may be booting or using a Live ISO.';
      case 'special-filesystems-only':
        return 'Only special filesystems detected (ISO/squashfs). This is normal for Live systems.';
      case 'agent-error':
        return 'Error communicating with guest agent.';
      case 'no-data':
        return 'No disk data available from Proxmox API.';
      default:
        return 'Disk stats unavailable. Guest agent may not be installed.';
    }
  };

  // Get row styling - include alert styles if present
  const rowClass = createMemo(() => {
    const base = 'transition-all duration-200 relative';
    const hover = 'hover:shadow-sm';
    // Extract only the background color from alert styles, not the border
    const alertBg = props.alertStyles?.hasAlert
      ? props.alertStyles.severity === 'critical'
        ? 'bg-red-50 dark:bg-red-950/30'
        : 'bg-yellow-50 dark:bg-yellow-950/20'
      : '';
    const defaultHover = props.alertStyles?.hasAlert
      ? ''
      : 'hover:bg-gray-50 dark:hover:bg-gray-700/30';
    const stoppedDimming = !isRunning() ? 'opacity-60' : '';
    return `${base} ${hover} ${defaultHover} ${alertBg} ${stoppedDimming}`;
  });

  // Get first cell styling
  const firstCellClass = createMemo(() => {
    const base = 'py-0.5 px-2 whitespace-nowrap relative';
    // Add extra padding when alert is present for visual spacing
    const padding = props.alertStyles?.hasAlert ? 'pl-4' : '';
    return `${base} ${padding}`;
  });

  // Get row styles including box-shadow for alert border
  const rowStyle = createMemo(() => {
    if (!props.alertStyles?.hasAlert) return {};
    const color = props.alertStyles.severity === 'critical' ? '#ef4444' : '#eab308';
    return {
      'box-shadow': `inset 4px 0 0 0 ${color}`,
    };
  });

  return (
    <tr class={rowClass()} style={rowStyle()}>
      {/* Name - Sticky column */}
      <td class={firstCellClass()}>
        <div class="flex items-center gap-2">
          {/* Status indicator */}
          <span
            class={`h-2 w-2 rounded-full flex-shrink-0 ${
              isRunning() ? 'bg-green-500' : 'bg-red-500'
            }`}
            title={props.guest.status}
          ></span>

          {/* Name - clickable if custom URL is set */}
          <Show
            when={customUrl()}
            fallback={
              <span
                class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate"
                title={props.guest.name}
              >
                {props.guest.name}
              </span>
            }
          >
            <a
              href={customUrl()}
              target="_blank"
              rel="noopener noreferrer"
              class="text-sm font-medium text-gray-900 dark:text-gray-100 hover:text-blue-600 dark:hover:text-blue-400 transition-colors duration-150 cursor-pointer truncate"
              title={`${props.guest.name} - Click to open custom URL`}
            >
              {props.guest.name}
            </a>
          </Show>

          {/* Tag badges */}
          <TagBadges
            tags={Array.isArray(props.guest.tags) ? props.guest.tags : []}
            maxVisible={3}
            onTagClick={props.onTagClick}
            activeSearch={props.activeSearch}
          />
        </div>
      </td>

      {/* Type */}
      <td class="py-0.5 px-2 whitespace-nowrap">
        <span
          class={`inline-block px-1.5 py-0.5 text-xs font-medium rounded ${
            props.guest.type === 'qemu'
              ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300'
              : 'bg-green-100 text-green-700 dark:bg-green-900/50 dark:text-green-300'
          }`}
        >
          {isVM(props.guest) ? 'VM' : 'LXC'}
        </span>
      </td>

      {/* VMID */}
      <td class="py-0.5 px-2 whitespace-nowrap text-sm text-gray-600 dark:text-gray-400">
        {props.guest.vmid}
      </td>

      {/* Uptime */}
      <td
        class={`py-0.5 px-2 text-sm whitespace-nowrap ${
          props.guest.uptime < 3600 ? 'text-orange-500' : 'text-gray-600 dark:text-gray-400'
        }`}
      >
        <Show when={isRunning()} fallback="-">
          {formatUptime(props.guest.uptime)}
        </Show>
      </td>

      {/* CPU */}
      <td class="py-0.5 px-2 w-[140px]">
        <MetricBar
          value={cpuPercent()}
          label={`${cpuPercent().toFixed(0)}%`}
          sublabel={
            props.guest.cpus
              ? `${((props.guest.cpu || 0) * props.guest.cpus).toFixed(1)}/${props.guest.cpus} cores`
              : undefined
          }
          type="cpu"
        />
      </td>

      {/* Memory */}
      <td class="py-0.5 px-2 w-[140px]">
        <MetricBar
          value={memPercent()}
          label={`${memPercent().toFixed(0)}%`}
          sublabel={
            props.guest.memory
              ? `${formatBytes(props.guest.memory.used)}/${formatBytes(props.guest.memory.total)}`
              : undefined
          }
          type="memory"
        />
      </td>

      {/* Disk */}
      <td class="py-0.5 px-2 w-[140px]">
        <Show
          when={props.guest.disk && props.guest.disk.total > 0 && diskPercent() !== -1}
          fallback={
            <span class="text-gray-400 text-sm cursor-help" title={getDiskStatusTooltip()}>
              -
            </span>
          }
        >
          <MetricBar
            value={diskPercent()}
            label={`${diskPercent().toFixed(0)}%`}
            sublabel={
              props.guest.disk
                ? `${formatBytes(props.guest.disk.used)}/${formatBytes(props.guest.disk.total)}`
                : undefined
            }
            type="disk"
          />
        </Show>
      </td>

      {/* Disk I/O */}
      <td class="py-0.5 px-2">
        <IOMetric value={props.guest.diskRead} disabled={!isRunning()} />
      </td>
      <td class="py-0.5 px-2">
        <IOMetric value={props.guest.diskWrite} disabled={!isRunning()} />
      </td>

      {/* Network I/O */}
      <td class="py-0.5 px-2">
        <IOMetric value={props.guest.networkIn} disabled={!isRunning()} />
      </td>
      <td class="py-0.5 px-2">
        <IOMetric value={props.guest.networkOut} disabled={!isRunning()} />
      </td>
    </tr>
  );
}
