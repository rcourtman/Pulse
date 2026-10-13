import { For, Show } from 'solid-js';

import { InfoCardFrame } from '@/components/shared/InfoCardFrame';
import type { Disk } from '@/types/api';
import { formatBytes } from '@/utils/format';
import { getFilesystemByteUsagePercent } from '@/utils/filesystemByteUsage';
import { getMetricColorRgba, getMetricTextColorClass } from '@/utils/metricThresholds';

export interface DrawerDiskListItem {
  key: string;
  label: string;
  device: string;
  percent: number | null;
  usageText: string;
  color: string;
  textClass: string;
}

const getDiskLabel = (disk: Disk, index: number): string =>
  disk.mountpoint || disk.device || `Disk ${index + 1}`;

export const buildDrawerDiskListItems = (disks: Disk[]): DrawerDiskListItem[] =>
  disks.map((disk, index) => {
    const percent = getFilesystemByteUsagePercent(disk);
    return {
      key: `${disk.device ?? ''}|${disk.mountpoint ?? ''}|${index}`,
      label: getDiskLabel(disk, index),
      device: disk.device ?? '',
      percent,
      usageText:
        percent !== null
          ? `${Math.round(percent)}% · ${formatBytes(disk.used ?? 0)} / ${formatBytes(disk.total!)}`
          : typeof disk.total === 'number' && Number.isFinite(disk.total) && disk.total > 0
            ? `Usage unavailable · ${formatBytes(disk.total)} total`
            : 'Usage unavailable',
      color: percent === null ? '' : getMetricColorRgba(percent, 'disk'),
      textClass: percent === null ? 'text-muted' : getMetricTextColorClass(percent, 'disk'),
    };
  });

interface DrawerDiskListCardProps {
  title?: string;
  disks: DrawerDiskListItem[];
  testId?: string;
}

export function DrawerDiskListCard(props: DrawerDiskListCardProps) {
  return (
    <InfoCardFrame class="basis-[calc(50%-0.75rem)] grow-2" data-testid={props.testId}>
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
                <span class={`text-right tabular-nums ${disk.textClass}`}>{disk.usageText}</span>
              </div>
              <Show when={disk.percent !== null}>
                <div class="relative h-1.5 w-full overflow-hidden rounded-sm bg-surface-hover">
                  <div
                    class="absolute inset-y-0 left-0 rounded-sm"
                    style={{
                      width: `${Math.max(0, Math.min(100, disk.percent!))}%`,
                      background: disk.color,
                    }}
                  />
                </div>
              </Show>
            </div>
          )}
        </For>
      </div>
    </InfoCardFrame>
  );
}
