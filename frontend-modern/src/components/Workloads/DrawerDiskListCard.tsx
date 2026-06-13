import { For, Show } from 'solid-js';

import { InfoCardFrame } from '@/components/shared/InfoCardFrame';
import type { Disk } from '@/types/api';
import { formatBytes } from '@/utils/format';
import { getMetricColorRgba, getMetricTextColorClass } from '@/utils/metricThresholds';

export interface DrawerDiskListItem {
  key: string;
  label: string;
  device: string;
  percent: number;
  used: number;
  total: number;
  color: string;
  textClass: string;
}

const getDiskUsagePercent = (disk: Disk): number => {
  if (disk.total > 0 && Number.isFinite(disk.used)) {
    return (disk.used / disk.total) * 100;
  }
  if (Number.isFinite(disk.usage)) {
    return disk.usage <= 1 ? disk.usage * 100 : disk.usage;
  }
  return 0;
};

const getDiskLabel = (disk: Disk, index: number): string =>
  disk.mountpoint || disk.device || `Disk ${index + 1}`;

export const buildDrawerDiskListItems = (disks: Disk[]): DrawerDiskListItem[] =>
  disks.map((disk, index) => {
    const percent = getDiskUsagePercent(disk);
    return {
      key: `${disk.device ?? ''}|${disk.mountpoint ?? ''}|${index}`,
      label: getDiskLabel(disk, index),
      device: disk.device ?? '',
      percent,
      used: disk.used ?? 0,
      total: disk.total ?? 0,
      color: getMetricColorRgba(percent, 'disk'),
      textClass: getMetricTextColorClass(percent, 'disk'),
    };
  });

interface DrawerDiskListCardProps {
  title?: string;
  disks: DrawerDiskListItem[];
  testId?: string;
}

export function DrawerDiskListCard(props: DrawerDiskListCardProps) {
  return (
    <InfoCardFrame class="basis-[calc(50%-0.75rem)] grow-[2]" data-testid={props.testId}>
      <h3 class="mb-2 text-[11px] font-medium uppercase tracking-wide text-base-content">
        {props.title ?? 'Storage'}
      </h3>
      <div class="space-y-2 text-[11px]">
        <For each={props.disks}>
          {(disk) => (
            <div class="space-y-1">
              <div class="flex items-baseline justify-between gap-2 min-w-0">
                <span
                  class="truncate font-medium text-base-content"
                  title={disk.device ? `${disk.label} · ${disk.device}` : disk.label}
                >
                  {disk.label}
                </span>
                <span class={`shrink-0 tabular-nums ${disk.textClass}`}>
                  {`${Math.round(disk.percent)}%`}
                  <span class="ml-1 text-muted">
                    ({formatBytes(disk.used)}
                    <Show when={disk.total > 0}> / {formatBytes(disk.total)}</Show>)
                  </span>
                </span>
              </div>
              <div class="relative h-1.5 w-full overflow-hidden rounded bg-surface-hover">
                <div
                  class="absolute inset-y-0 left-0 rounded"
                  style={{
                    width: `${Math.max(0, Math.min(100, disk.percent))}%`,
                    background: disk.color,
                  }}
                />
              </div>
            </div>
          )}
        </For>
      </div>
    </InfoCardFrame>
  );
}
