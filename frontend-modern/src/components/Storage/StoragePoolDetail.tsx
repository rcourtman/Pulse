import { Component, createSignal, For, Show, createMemo } from 'solid-js';
import { HistoryChart } from '@/components/shared/HistoryChart';
import type { HistoryTimeRange } from '@/api/charts';
import { formatBytes, formatPercent } from '@/utils/format';
import {
  getRecordContent,
  getRecordNodeLabel,
  getRecordShared,
  getRecordStatus,
  getRecordType,
  getRecordUsagePercent,
  getRecordZfsPool,
} from './useStorageModel';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { Resource } from '@/types/resource';

interface StoragePoolDetailProps {
  record: StorageRecord;
  physicalDisks: Resource[];
}

export const StoragePoolDetail: Component<StoragePoolDetailProps> = (props) => {
  const [chartRange, setChartRange] = createSignal<HistoryTimeRange>('7d');

  const zfsPool = createMemo(() => getRecordZfsPool(props.record));
  const type = createMemo(() => getRecordType(props.record));
  const content = createMemo(() => getRecordContent(props.record));
  const shared = createMemo(() => getRecordShared(props.record));
  const status = createMemo(() => getRecordStatus(props.record));
  const usagePercent = createMemo(() => getRecordUsagePercent(props.record));
  const totalBytes = createMemo(() => props.record.capacity.totalBytes || 0);
  const usedBytes = createMemo(() => props.record.capacity.usedBytes || 0);
  const freeBytes = createMemo(() =>
    props.record.capacity.freeBytes ?? (totalBytes() > 0 ? Math.max(totalBytes() - usedBytes(), 0) : 0),
  );

  // Match physical disks to ZFS pool devices
  const poolDisks = createMemo(() => {
    const pool = zfsPool();
    if (!pool || !pool.devices?.length) return [];
    return props.physicalDisks.filter((disk) => {
      const pd = (disk.platformData as Record<string, unknown>)?.physicalDisk as Record<string, unknown> | undefined;
      const devPath = (pd?.devPath as string) || '';
      return pool.devices.some((d) => devPath.endsWith(d.name));
    });
  });

  // Build resource ID for history chart
  const chartResourceId = createMemo(() => {
    return props.record.refs?.resourceId || props.record.id;
  });

  return (
    <tr class="border-t border-gray-200 dark:border-gray-700">
      <td colSpan={99} class="bg-gray-50/60 dark:bg-gray-900/20 px-4 py-4">
        <div class="grid gap-4 md:grid-cols-2">
          {/* Left: Capacity trend chart */}
          <div class="rounded-lg border border-gray-200 bg-white/80 p-3 shadow-sm dark:border-gray-600/60 dark:bg-gray-900/30">
            <div class="flex items-center justify-between mb-2">
              <h4 class="text-xs font-semibold text-gray-700 dark:text-gray-200">Capacity Trend</h4>
              <select
                value={chartRange()}
                onChange={(e) => setChartRange(e.currentTarget.value as HistoryTimeRange)}
                class="text-[11px] font-medium pl-2 pr-5 py-0.5 rounded border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-200 cursor-pointer appearance-none"
                style={{
                  'background-image': "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='10' height='10' viewBox='0 0 24 24' fill='none' stroke='%239ca3af' stroke-width='2'%3E%3Cpath d='M6 9l6 6 6-6'/%3E%3C/svg%3E\")",
                  'background-repeat': 'no-repeat',
                  'background-position': 'right 4px center',
                }}
              >
                <option value="24h">24h</option>
                <option value="7d">7d</option>
                <option value="30d">30d</option>
                <option value="90d">90d</option>
              </select>
            </div>
            <HistoryChart
              resourceType="storage"
              resourceId={chartResourceId()}
              metric="usage"
              label="Usage"
              unit="%"
              height={140}
              range={chartRange()}
              hideSelector
              compact
              hideLock
            />
          </div>

          {/* Right: Configuration & details */}
          <div class="space-y-3">
            {/* Config card */}
            <div class="rounded-lg border border-gray-200 bg-white/80 p-3 shadow-sm dark:border-gray-600/60 dark:bg-gray-900/30">
              <h4 class="text-xs font-semibold text-gray-700 dark:text-gray-200 mb-2">Configuration</h4>
              <div class="grid grid-cols-2 gap-x-4 gap-y-1.5 text-[11px]">
                <ConfigRow label="Node" value={getRecordNodeLabel(props.record)} />
                <ConfigRow label="Type" value={type()} />
                <Show when={content()}>
                  <ConfigRow label="Content" value={content()} />
                </Show>
                <ConfigRow label="Status" value={status()} />
                <ConfigRow label="Shared" value={shared() === null ? '-' : shared() ? 'Yes' : 'No'} />
                <ConfigRow label="Used" value={totalBytes() > 0 ? formatBytes(usedBytes()) : 'n/a'} />
                <ConfigRow label="Free" value={totalBytes() > 0 ? formatBytes(freeBytes()) : 'n/a'} />
                <ConfigRow label="Total" value={totalBytes() > 0 ? formatBytes(totalBytes()) : 'n/a'} />
                <ConfigRow label="Usage" value={formatPercent(usagePercent())} />
              </div>
            </div>

            {/* ZFS details */}
            <Show when={zfsPool()}>
              <div class="rounded-lg border border-gray-200 bg-white/80 p-3 shadow-sm dark:border-gray-600/60 dark:bg-gray-900/30">
                <h4 class="text-xs font-semibold text-gray-700 dark:text-gray-200 mb-2">ZFS Pool</h4>
                <div class="grid grid-cols-2 gap-x-4 gap-y-1.5 text-[11px]">
                  <ConfigRow label="State" value={zfsPool()!.state} />
                  <Show when={zfsPool()!.scan && zfsPool()!.scan !== 'none'}>
                    <div class="col-span-2">
                      <span class="text-gray-500 dark:text-gray-400">Scan: </span>
                      <span class="text-yellow-600 dark:text-yellow-400 italic">{zfsPool()!.scan}</span>
                    </div>
                  </Show>
                  <Show when={zfsPool()!.readErrors > 0 || zfsPool()!.writeErrors > 0 || zfsPool()!.checksumErrors > 0}>
                    <div class="col-span-2 text-red-600 dark:text-red-400 font-medium">
                      Errors: R:{zfsPool()!.readErrors} W:{zfsPool()!.writeErrors} C:{zfsPool()!.checksumErrors}
                    </div>
                  </Show>
                </div>
              </div>
            </Show>

            {/* Physical disks linked to this pool */}
            <Show when={poolDisks().length > 0}>
              <div class="rounded-lg border border-gray-200 bg-white/80 p-3 shadow-sm dark:border-gray-600/60 dark:bg-gray-900/30">
                <h4 class="text-xs font-semibold text-gray-700 dark:text-gray-200 mb-2">
                  Physical Disks ({poolDisks().length})
                </h4>
                <div class="space-y-1">
                  <For each={poolDisks()}>
                    {(disk) => {
                      const pd = () => (disk.platformData as Record<string, unknown>)?.physicalDisk as Record<string, unknown> | undefined;
                      const temp = () => (pd()?.temperature as number) ?? 0;
                      const health = () => (pd()?.smart as Record<string, unknown>)?.reallocatedSectors as number | undefined;
                      return (
                        <div class="flex items-center gap-2 text-[11px] py-0.5">
                          <span class="font-mono text-gray-600 dark:text-gray-400 w-16 truncate" title={pd()?.devPath as string}>
                            {pd()?.devPath as string || disk.name}
                          </span>
                          <span class={`w-2 h-2 rounded-full flex-shrink-0 ${
                            health() != null && (health() as number) > 0
                              ? 'bg-yellow-500'
                              : 'bg-green-500'
                          }`} />
                          <span class="text-gray-700 dark:text-gray-300 truncate flex-1">
                            {pd()?.model as string || 'Unknown'}
                          </span>
                          <Show when={temp() > 0}>
                            <span class={`font-medium ${
                              temp() > 60 ? 'text-red-500' : temp() > 50 ? 'text-yellow-500' : 'text-gray-500 dark:text-gray-400'
                            }`}>
                              {temp()}°C
                            </span>
                          </Show>
                        </div>
                      );
                    }}
                  </For>
                </div>
              </div>
            </Show>
          </div>
        </div>
      </td>
    </tr>
  );
};

const ConfigRow: Component<{ label: string; value: string }> = (props) => (
  <div class="flex justify-between">
    <span class="text-gray-500 dark:text-gray-400">{props.label}</span>
    <span class="text-gray-800 dark:text-gray-200 font-medium">{props.value}</span>
  </div>
);
