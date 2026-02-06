import { Component, Show, createSignal, createMemo } from 'solid-js';
import type { PhysicalDisk } from '@/types/api';
import { diskResourceId } from '@/types/api';
import { HistoryChart } from '@/components/shared/HistoryChart';
import type { HistoryTimeRange, AggregatedMetricPoint } from '@/api/charts';
import { formatTemperature } from '@/utils/temperature';
import { formatPowerOnHours } from '@/utils/format';
import { useWebSocket } from '@/App';
import { getDiskMetricHistory, getDiskMetricsVersion } from '@/stores/diskMetricsHistory';
import { createEffect, onCleanup } from 'solid-js';


/** Color class for attribute values. */
function attrColor(ok: boolean): string {
  return ok
    ? 'text-green-600 dark:text-green-400'
    : 'text-red-600 dark:text-red-400';
}

interface DiskDetailProps {
  disk: PhysicalDisk;
}

export const DiskDetail: Component<DiskDetailProps> = (props) => {
  const [chartRange, setChartRange] = createSignal<HistoryTimeRange>('24h');

  const resId = () => diskResourceId(props.disk);
  const attrs = () => props.disk.smartAttributes;
  const isNvme = () => props.disk.type?.toLowerCase() === 'nvme';

  const { state } = useWebSocket();

  const getMetricResourceId = () => {
    const nodeName = props.disk.node;
    const instance = props.disk.instance;
    const node = state.nodes?.find(n => n.name === nodeName && n.instance === instance);
    const hostId = node?.linkedHostAgentId;

    if (!hostId) return null;
    const deviceName = props.disk.devPath.replace('/dev/', '');
    return `${hostId}:${deviceName}`;
  };

  const metricResourceId = createMemo(() => getMetricResourceId());

  // Subscribe to disk metrics updates
  const [diskVer, setDiskVer] = createSignal(getDiskMetricsVersion());
  createEffect(() => {
    const t = setInterval(() => setDiskVer(getDiskMetricsVersion()), 2000);
    onCleanup(() => clearInterval(t));
  });

  const historyData = createMemo(() => {
    diskVer(); // dependency
    const id = metricResourceId();
    if (!id) return [];
    return getDiskMetricHistory(id, 30 * 60 * 1000); // 30 mins
  });

  const readData = createMemo<AggregatedMetricPoint[]>(() =>
    historyData().map(d => ({ timestamp: d.timestamp, value: d.readBps, min: d.readBps, max: d.readBps }))
  );
  const writeData = createMemo<AggregatedMetricPoint[]>(() =>
    historyData().map(d => ({ timestamp: d.timestamp, value: d.writeBps, min: d.writeBps, max: d.writeBps }))
  );
  const ioData = createMemo<AggregatedMetricPoint[]>(() =>
    // Convert util% (0-100)
    historyData().map(d => ({ timestamp: d.timestamp, value: d.ioTime, min: d.ioTime, max: d.ioTime }))
  );

  return (
    <div class="space-y-3">
      {/* Disk info */}
      {/* Header: Info & Selector */}
      <div class="flex flex-wrap items-end justify-between gap-3 border-b border-gray-100 dark:border-gray-800 pb-3">
        <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px]">
          <span class="font-semibold text-gray-900 dark:text-gray-100 text-sm">
            {props.disk.model || 'Unknown Disk'}
          </span>
          <span class="text-gray-500 dark:text-gray-400 font-mono bg-gray-100 dark:bg-gray-800 px-1.5 py-0.5 rounded border border-gray-200 dark:border-gray-700">
            {props.disk.devPath}
          </span>
          <span class="text-gray-500 dark:text-gray-400">
            {props.disk.node}
          </span>
          <Show when={props.disk.serial}>
            <span class="text-gray-400 font-mono">
              S/N: {props.disk.serial}
            </span>
          </Show>
        </div>

        {/* Global Time Range Selector */}
        <div class="flex items-center gap-2">
          <span class="text-[10px] font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">History:</span>
          <div class="relative">
            <select
              value={chartRange()}
              onChange={(e) => setChartRange(e.currentTarget.value as HistoryTimeRange)}
              class="text-[11px] font-medium pl-2 pr-6 py-1 rounded-md border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-200 cursor-pointer focus:ring-1 focus:ring-blue-500 focus:border-blue-500 appearance-none shadow-sm hover:border-gray-300 dark:hover:border-gray-500 transition-colors"
              style={{ "background-image": "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 24 24' fill='none' stroke='%239ca3af' stroke-width='2'%3E%3Cpath d='M6 9l6 6 6-6'/%3E%3C/svg%3E\")", "background-repeat": "no-repeat", "background-position": "right 6px center" }}
            >
              <option value="1h">Last 1 hour</option>
              <option value="6h">Last 6 hours</option>
              <option value="12h">Last 12 hours</option>
              <option value="24h">Last 24 hours</option>
              <option value="7d">Last 7 days</option>
              <option value="30d">Last 30 days</option>
              <option value="90d">Last 90 days</option>
            </select>
          </div>
        </div>
      </div>

      {/* SMART attribute cards */}
      <Show when={attrs()}>
        <div class="flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(16.666%-0.5rem)] [&>*]:min-w-[120px]">
          {/* Common attributes */}
          <Show when={attrs()!.powerOnHours != null}>
            <AttrCard
              label="Power-On Time"
              value={formatPowerOnHours(attrs()!.powerOnHours!)}
              ok={true}
            />
          </Show>
          <Show when={props.disk.temperature > 0}>
            <AttrCard
              label="Temperature"
              value={formatTemperature(props.disk.temperature)}
              ok={props.disk.temperature <= 60}
            />
          </Show>
          <Show when={attrs()!.powerCycles != null}>
            <AttrCard
              label="Power Cycles"
              value={attrs()!.powerCycles!.toLocaleString()}
              ok={true}
            />
          </Show>

          {/* SATA-specific */}
          <Show when={!isNvme() && attrs()!.reallocatedSectors != null}>
            <AttrCard
              label="Reallocated Sectors"
              value={attrs()!.reallocatedSectors!.toString()}
              ok={attrs()!.reallocatedSectors === 0}
            />
          </Show>
          <Show when={!isNvme() && attrs()!.pendingSectors != null}>
            <AttrCard
              label="Pending Sectors"
              value={attrs()!.pendingSectors!.toString()}
              ok={attrs()!.pendingSectors === 0}
            />
          </Show>
          <Show when={!isNvme() && attrs()!.offlineUncorrectable != null}>
            <AttrCard
              label="Offline Uncorrectable"
              value={attrs()!.offlineUncorrectable!.toString()}
              ok={attrs()!.offlineUncorrectable === 0}
            />
          </Show>
          <Show when={!isNvme() && attrs()!.udmaCrcErrors != null}>
            <AttrCard
              label="CRC Errors"
              value={attrs()!.udmaCrcErrors!.toString()}
              ok={attrs()!.udmaCrcErrors === 0}
            />
          </Show>

          {/* NVMe-specific */}
          <Show when={isNvme() && attrs()!.percentageUsed != null}>
            <AttrCard
              label="Life Used"
              value={`${attrs()!.percentageUsed}%`}
              ok={(attrs()!.percentageUsed ?? 0) <= 90}
            />
          </Show>
          <Show when={isNvme() && attrs()!.availableSpare != null}>
            <AttrCard
              label="Available Spare"
              value={`${attrs()!.availableSpare}%`}
              ok={(attrs()!.availableSpare ?? 0) >= 20}
            />
          </Show>
          <Show when={isNvme() && attrs()!.mediaErrors != null}>
            <AttrCard
              label="Media Errors"
              value={attrs()!.mediaErrors!.toString()}
              ok={attrs()!.mediaErrors === 0}
            />
          </Show>
          <Show when={isNvme() && attrs()!.unsafeShutdowns != null}>
            <AttrCard
              label="Unsafe Shutdowns"
              value={attrs()!.unsafeShutdowns!.toLocaleString()}
              ok={true}
            />
          </Show>
        </div>
      </Show>

      {/* Live Performance Sparklines */}
      <Show when={metricResourceId()}>
        <div class="space-y-2">
          <h4 class="text-xs font-semibold text-gray-700 dark:text-gray-300 flex items-center gap-2">
            Live I/O (30m)
            <span class="text-[10px] font-normal text-gray-400 bg-gray-100 dark:bg-gray-800 px-1.5 py-0.5 rounded">Real-time</span>
          </h4>
          <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
              <HistoryChart
                resourceType="host" resourceId="dummy" metric="disk"
                label="Read" unit="B/s"
                data={readData()} hideSelector hideLock height={120} compact={true}
              />
            </div>
            <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
              <HistoryChart
                resourceType="host" resourceId="dummy" metric="disk"
                label="Write" unit="B/s"
                data={writeData()} hideSelector hideLock height={120} compact={true}
              />
            </div>
            <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
              <HistoryChart
                resourceType="host" resourceId="dummy" metric="disk"
                label="Busy" unit="%"
                data={ioData()} hideSelector hideLock height={120} compact={true}
              />
            </div>
          </div>
        </div>
      </Show>

      {/* Historical charts */}
      <Show
        when={resId()}
        fallback={
          <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30 text-center text-[11px] text-gray-500 dark:text-gray-400">
            Install the Pulse host agent for detailed SMART monitoring and historical charts.
          </div>
        }
      >
        <div class="space-y-2">


          {/* Charts grid */}
          <div class="flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(33.333%-0.5rem)] [&>*]:min-w-[250px]">
            {/* Temperature chart (all disk types) */}
            <Show when={props.disk.temperature > 0}>
              <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                <HistoryChart
                  resourceType="disk"
                  resourceId={resId()!}
                  metric="smart_temp"
                  label="Temperature"
                  unit="C"
                  height={120}
                  color="#ef4444"
                  range={chartRange()}
                  hideSelector={true}
                  compact={true}
                  hideLock={true}
                />
              </div>
            </Show>

            {/* SATA charts */}
            <Show when={!isNvme() && attrs()?.reallocatedSectors != null}>
              <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                <HistoryChart
                  resourceType="disk"
                  resourceId={resId()!}
                  metric="smart_reallocated_sectors"
                  label="Reallocated Sectors"
                  unit="sectors"
                  height={120}
                  color="#f59e0b"
                  range={chartRange()}
                  hideSelector={true}
                  compact={true}
                  hideLock={true}
                />
              </div>
            </Show>

            {/* NVMe charts */}
            <Show when={isNvme() && attrs()?.percentageUsed != null}>
              <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                <HistoryChart
                  resourceType="disk"
                  resourceId={resId()!}
                  metric="smart_percentage_used"
                  label="Life Used"
                  unit="%"
                  height={120}
                  color="#f59e0b"
                  range={chartRange()}
                  hideSelector={true}
                  compact={true}
                  hideLock={true}
                />
              </div>
            </Show>
            <Show when={isNvme() && attrs()?.availableSpare != null}>
              <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                <HistoryChart
                  resourceType="disk"
                  resourceId={resId()!}
                  metric="smart_available_spare"
                  label="Available Spare"
                  unit="%"
                  height={120}
                  color="#10b981"
                  range={chartRange()}
                  hideSelector={true}
                  compact={true}
                  hideLock={true}
                />
              </div>
            </Show>
          </div>
        </div>
      </Show>
    </div>
  );
};

/** Small attribute summary card matching the NodeDrawer card pattern. */
const AttrCard: Component<{ label: string; value: string; ok: boolean }> = (props) => {
  return (
    <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
      <div class="text-[10px] font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide mb-0.5">
        {props.label}
      </div>
      <div class={`text-sm font-semibold ${attrColor(props.ok)}`}>
        {props.value}
      </div>
    </div>
  );
};
