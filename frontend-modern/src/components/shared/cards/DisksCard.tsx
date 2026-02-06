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
    <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
      <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Disks</div>
      <div class="max-h-[140px] overflow-y-auto custom-scrollbar space-y-2">
        <For each={props.disks}>
          {(disk) => {
            const usagePercent = disk.total > 0 ? (disk.used / disk.total) * 100 : 0;
            const barColor = getMetricColorRgba(usagePercent, 'disk');
            const textColor = getMetricTextColorClass(usagePercent, 'disk');
            return (
              <div class="text-[10px]">
                <div class="flex justify-between mb-0.5">
                  <span class="text-gray-600 dark:text-gray-300 truncate max-w-[100px]" title={disk.mountpoint}>{disk.mountpoint}</span>
                  <span class="flex items-center gap-1.5">
                    <span class={`font-medium ${textColor}`}>{usagePercent.toFixed(0)}%</span>
                    <span class="text-gray-400 dark:text-gray-500">·</span>
                    <span class="text-gray-500 dark:text-gray-400">{formatBytes(disk.used)} / {formatBytes(disk.total)}</span>
                  </span>
                </div>
                <div class="h-1.5 w-full rounded-full bg-gray-200 dark:bg-gray-700 overflow-hidden">
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
