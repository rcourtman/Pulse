import { For, Show } from 'solid-js';
import type { Disk } from '@/types/api';
import { formatBytes } from '@/utils/format';
import { getMetricColorClass } from '@/utils/metricThresholds';

interface DiskListProps {
  disks: Disk[];
  diskStatusReason?: string;
}

export function DiskList(props: DiskListProps) {
  const getUsagePercent = (disk: Disk) => {
    if (!disk.total || disk.total <= 0) return 0;
    return (disk.used / disk.total) * 100;
  };


  const getDiskStatusTooltip = () => {
    const reason = props.diskStatusReason;

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

  return (
    <Show
      when={props.disks && props.disks.length > 0}
      fallback={
        <span class="text-slate-400 text-sm" title={getDiskStatusTooltip()}>
          -
        </span>
      }
    >
      <div class="flex flex-col gap-1.5">
        <For each={props.disks}>
          {(disk) => {
            const usage = getUsagePercent(disk);
            const label = disk.mountpoint || disk.device || 'Unknown';
            const hasCapacity = disk.total && disk.total > 0;

            return (
              <div class="rounded border border-slate-200 bg-slate-50 px-1.5 py-1 text-[10px] leading-tight shadow-sm dark:border-slate-700 dark:bg-slate-800">
                <div
                  class="truncate text-slate-700 dark:text-slate-200"
                  title={label !== 'Unknown' ? label : undefined}
                >
                  {label}
                </div>
                <div class="mt-0.5 text-[9px] text-muted">
                  {hasCapacity
                    ? `${formatBytes(disk.used)}/${formatBytes(disk.total)}`
                    : 'Usage unavailable'}
                </div>
                <div class="relative mt-1 h-1.5 w-full overflow-hidden rounded bg-slate-200 dark:bg-slate-600">
                  <div
                    class={`absolute inset-y-0 left-0 ${getMetricColorClass(usage, 'disk')}`}
                    style={{ width: `${Math.min(usage, 100)}%` }}
                  />
                </div>
                <div class="mt-0.5 flex items-center justify-between text-[9px] font-medium text-slate-600 dark:text-slate-300">
                  <span>{hasCapacity ? `${usage.toFixed(0)}%` : '—'}</span>
                  <span>{disk.type?.toUpperCase() ?? ''}</span>
                </div>
              </div>
            );
          }}
        </For>
      </div>
    </Show>
  );
}
