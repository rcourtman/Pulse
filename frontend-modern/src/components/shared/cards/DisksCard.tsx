import { Component, For } from 'solid-js';
import { Disk } from '@/types/api';
import { formatBytes } from '@/utils/format';
import { getMetricColorRgba, getMetricTextColorClass } from '@/utils/metricThresholds';

interface DisksCardProps {
  disks?: Disk[];
}

export const DisksCard: Component<DisksCardProps> = (props) => {
  if (!props.disks || props.disks.length === 0) return null;

  return (
    <div class="rounded border border-slate-200 bg-white p-3 shadow-sm dark:border-slate-600 dark:bg-slate-800">
      <div class="text-[11px] font-medium uppercase tracking-wide text-slate-700 dark:text-slate-200 mb-2">Disks</div>
      <div class="max-h-[140px] overflow-y-auto custom-scrollbar space-y-2">
        <For each={props.disks}>
          {(disk) => {
            const usagePercent = disk.total > 0 ? (disk.used / disk.total) * 100 : 0;
            const barColor = getMetricColorRgba(usagePercent, 'disk');
            const textColor = getMetricTextColorClass(usagePercent, 'disk');
            return (
              <div class="text-[10px]">
                <div class="flex justify-between mb-0.5">
                  <span class="text-slate-600 dark:text-slate-300 truncate max-w-[100px]" title={disk.mountpoint}>{disk.mountpoint}</span>
                  <span class="flex items-center gap-1.5">
                    <span class={`font-medium ${textColor}`}>{usagePercent.toFixed(0)}%</span>
                    <span class="text-slate-400 dark:text-slate-500">·</span>
                    <span class="text-slate-500 dark:text-slate-400">{formatBytes(disk.used)} / {formatBytes(disk.total)}</span>
                  </span>
                </div>
                <div class="h-1.5 w-full rounded-full bg-slate-200 dark:bg-slate-700 overflow-hidden">
                  <div
                    class="h-full rounded-full transition-all duration-500"
                    style={{ width: `${Math.min(100, Math.max(0, usagePercent))}%`, "background-color": barColor }}
                  />
                </div>
              </div>
            );
          }}
        </For>
      </div>
    </div>
  );
};
