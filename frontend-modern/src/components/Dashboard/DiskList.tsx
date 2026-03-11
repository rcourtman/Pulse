import { For, Show } from 'solid-js';
import type { Disk } from '@/types/api';
import { formatBytes } from '@/utils/format';
import { getMetricColorClass } from '@/utils/metricThresholds';
import { getDashboardGuestDiskStatusMessage } from '@/utils/dashboardGuestPresentation';

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
    return getDashboardGuestDiskStatusMessage(props.diskStatusReason);
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
              <div class="rounded border border-border bg-surface-hover px-1.5 py-1 text-[10px] leading-tight shadow-sm">
                <div
                  class="truncate text-base-content"
                  title={label !== 'Unknown' ? label : undefined}
                >
                  {label}
                </div>
                <div class="mt-0.5 text-[9px] text-muted">
                  {hasCapacity
                    ? `${formatBytes(disk.used)}/${formatBytes(disk.total)}`
                    : 'Usage unavailable'}
                </div>
                <div class="relative mt-1 h-1.5 w-full overflow-hidden rounded bg-surface-hover">
                  <div
                    class={`absolute inset-y-0 left-0 ${getMetricColorClass(usage, 'disk')}`}
                    style={{ width: `${Math.min(usage, 100)}%` }}
                  />
                </div>
                <div class="mt-0.5 flex items-center justify-between text-[9px] font-medium text-muted">
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
