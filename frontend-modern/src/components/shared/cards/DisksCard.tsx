import { Component, For, Show } from 'solid-js';
import { Disk } from '@/types/api';
import { formatBytes } from '@/utils/format';
import { getMetricColorRgba, getMetricTextColorClass } from '@/utils/metricThresholds';
import { InfoCardFrame } from '@/components/shared/InfoCardFrame';
import { StackedDiskBar } from '@/components/Workloads/StackedDiskBar';

interface DisksCardProps {
  disks?: Disk[];
}

export const DisksCard: Component<DisksCardProps> = (props) => {
  if (!props.disks || props.disks.length === 0) return null;

  const aggregateDisk = (): Disk | null => {
    const total = props.disks?.reduce((sum, disk) => sum + (disk.total || 0), 0) ?? 0;
    const used = props.disks?.reduce((sum, disk) => sum + (disk.used || 0), 0) ?? 0;
    if (total <= 0 && used <= 0) return null;
    return {
      total,
      used,
      free: Math.max(total - used, 0),
      usage: total > 0 ? used / total : 0,
      mountpoint: 'Total',
    };
  };
  const aggregateUsagePercent = () => {
    const aggregate = aggregateDisk();
    return aggregate && aggregate.total > 0 ? (aggregate.used / aggregate.total) * 100 : 0;
  };

  return (
    <InfoCardFrame>
      <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
        Disks
      </div>
      <Show when={aggregateDisk()}>
        {(aggregate) => (
          <div class="mb-3 space-y-1.5 border-b border-border pb-3" data-testid="disks-card-total">
            <div class="flex justify-between gap-2 text-[10px]">
              <span class="text-muted">Total Usage</span>
              <span
                class={`font-medium ${getMetricTextColorClass(aggregateUsagePercent(), 'disk')}`}
              >
                {formatBytes(aggregate().used)} / {formatBytes(aggregate().total)}
              </span>
            </div>
            <StackedDiskBar disks={props.disks} aggregateDisk={aggregate()} mode="aggregate" />
          </div>
        )}
      </Show>
      <div class="max-h-[140px] overflow-y-auto custom-scrollbar space-y-2">
        <For each={props.disks}>
          {(disk) => {
            const usagePercent = disk.total > 0 ? (disk.used / disk.total) * 100 : 0;
            const barColor = getMetricColorRgba(usagePercent, 'disk');
            const textColor = getMetricTextColorClass(usagePercent, 'disk');
            return (
              <div class="text-[10px]">
                <div class="flex justify-between mb-0.5">
                  <span class="text-muted truncate max-w-[100px]" title={disk.mountpoint}>
                    {disk.mountpoint}
                  </span>
                  <span class="flex items-center gap-1.5">
                    <span class={`font-medium ${textColor}`}>{usagePercent.toFixed(0)}%</span>
                    <span class="text-muted">·</span>
                    <span class="text-muted">
                      {formatBytes(disk.used)} / {formatBytes(disk.total)}
                    </span>
                  </span>
                </div>
                <div class="h-1.5 w-full rounded-full bg-surface-hover overflow-hidden">
                  <div
                    class="h-full rounded-full transition-all duration-500"
                    style={{
                      width: `${Math.min(100, Math.max(0, usagePercent))}%`,
                      'background-color': barColor,
                    }}
                  />
                </div>
              </div>
            );
          }}
        </For>
      </div>
    </InfoCardFrame>
  );
};
