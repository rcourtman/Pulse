import { Component, For, Show } from 'solid-js';
import type { Resource } from '@/types/resource';
import { HistoryChart } from '@/components/shared/HistoryChart';
import type { HistoryTimeRange } from '@/api/charts';
import {
  DISK_DETAIL_HISTORY_RANGE_OPTIONS,
  DISK_DETAIL_LIVE_CHARTS,
  getDiskAttributeValueTextClass,
  getDiskDetailHistoryFallbackMessage,
  getDiskDetailLiveBadgeLabel,
} from '@/features/storageBackups/diskDetailPresentation';
import {
  STORAGE_DETAIL_BADGE_CLASS,
  STORAGE_DETAIL_CARD_CLASS,
  STORAGE_DETAIL_EMPTY_CLASS,
  STORAGE_DISK_DETAIL_ATTRIBUTE_GRID_CLASS,
  STORAGE_DISK_DETAIL_HEADER_CLASS,
  STORAGE_DISK_DETAIL_HISTORY_CONTROL_CLASS,
  STORAGE_DISK_DETAIL_HISTORY_SELECT_WRAP_CLASS,
  STORAGE_DISK_DETAIL_HISTORY_GRID_CLASS,
  STORAGE_DISK_DETAIL_LIVE_GRID_CLASS,
  STORAGE_DISK_DETAIL_MODEL_CLASS,
  STORAGE_DISK_DETAIL_NODE_CLASS,
  STORAGE_DISK_DETAIL_ROOT_CLASS,
  STORAGE_DISK_DETAIL_SECTION_CLASS,
  STORAGE_DISK_DETAIL_SECTION_HEADING_CLASS,
  STORAGE_DISK_DETAIL_SERIAL_CLASS,
  STORAGE_DETAIL_HEADER_SELECT_CLASS,
  STORAGE_DETAIL_HEADER_SELECT_STYLE,
  STORAGE_DETAIL_INLINE_LABEL_CLASS,
  STORAGE_DETAIL_META_ROW_CLASS,
  STORAGE_DETAIL_MONO_CHIP_CLASS,
  STORAGE_DETAIL_SECTION_TITLE_CLASS,
} from '@/features/storageBackups/detailPresentation';
import { StorageDetailMetricCard } from './StorageDetailMetricCard';
import { useDiskDetailModel } from './useDiskDetailModel';

interface DiskDetailProps {
  disk: Resource;
  nodes: Resource[];
}

export const DiskDetail: Component<DiskDetailProps> = (props) => {
  const {
    chartRange,
    setChartRange,
    diskData,
    resId,
    attributeCards,
    historyCharts,
    metricResourceId,
    readData,
    writeData,
    ioData,
  } = useDiskDetailModel({
    disk: () => props.disk,
    nodes: () => props.nodes,
  });

  return (
    <div class={STORAGE_DISK_DETAIL_ROOT_CLASS}>
      {/* Disk info */}
      {/* Header: Info & Selector */}
      <div class={STORAGE_DISK_DETAIL_HEADER_CLASS}>
        <div class={STORAGE_DETAIL_META_ROW_CLASS}>
          <span class={STORAGE_DISK_DETAIL_MODEL_CLASS}>
            {diskData().model || 'Unknown Disk'}
          </span>
          <span class={STORAGE_DETAIL_MONO_CHIP_CLASS}>
            {diskData().devPath}
          </span>
          <span class={STORAGE_DISK_DETAIL_NODE_CLASS}>{diskData().node}</span>
          <Show when={diskData().serial}>
            <span class={STORAGE_DISK_DETAIL_SERIAL_CLASS}>S/N: {diskData().serial}</span>
          </Show>
        </div>

        {/* Global Time Range Selector */}
        <div class={STORAGE_DISK_DETAIL_HISTORY_CONTROL_CLASS}>
          <span class={STORAGE_DETAIL_INLINE_LABEL_CLASS}>History:</span>
          <div class={STORAGE_DISK_DETAIL_HISTORY_SELECT_WRAP_CLASS}>
            <select
              value={chartRange()}
              onChange={(e) => setChartRange(e.currentTarget.value as HistoryTimeRange)}
              class={STORAGE_DETAIL_HEADER_SELECT_CLASS}
              style={STORAGE_DETAIL_HEADER_SELECT_STYLE}
            >
              <For each={DISK_DETAIL_HISTORY_RANGE_OPTIONS}>
                {(option) => <option value={option.value}>{option.label}</option>}
              </For>
            </select>
          </div>
        </div>
      </div>

      {/* SMART attribute cards */}
      <Show when={diskData().smartAttributes}>
        <div class={STORAGE_DISK_DETAIL_ATTRIBUTE_GRID_CLASS}>
          <For each={attributeCards()}>
            {(card) => (
              <StorageDetailMetricCard
                label={card.label}
                value={card.value}
                valueClass={getDiskAttributeValueTextClass(card.ok)}
              />
            )}
          </For>
        </div>
      </Show>

      {/* Live Performance Sparklines */}
      <Show when={metricResourceId()}>
        <div class={STORAGE_DISK_DETAIL_SECTION_CLASS}>
          <h4 class={`${STORAGE_DETAIL_SECTION_TITLE_CLASS} ${STORAGE_DISK_DETAIL_SECTION_HEADING_CLASS}`}>
            Live I/O (30m)
            <span class={STORAGE_DETAIL_BADGE_CLASS}>
              {getDiskDetailLiveBadgeLabel()}
            </span>
          </h4>
          <div class={STORAGE_DISK_DETAIL_LIVE_GRID_CLASS}>
            <For each={DISK_DETAIL_LIVE_CHARTS}>
              {(chart) => (
                <div class={STORAGE_DETAIL_CARD_CLASS}>
                  <HistoryChart
                    resourceType="agent"
                    resourceId="dummy"
                    metric={chart.metric}
                    label={chart.label}
                    unit={chart.unit}
                    data={
                      chart.series === 'read'
                        ? readData()
                        : chart.series === 'write'
                          ? writeData()
                          : ioData()
                    }
                    hideSelector
                    hideLock
                    height={120}
                    compact={true}
                  />
                </div>
              )}
            </For>
          </div>
        </div>
      </Show>

      {/* Historical charts */}
      <Show
        when={resId()}
        fallback={
          <div class={STORAGE_DETAIL_EMPTY_CLASS}>
            {getDiskDetailHistoryFallbackMessage()}
          </div>
        }
      >
        <div class={STORAGE_DISK_DETAIL_SECTION_CLASS}>
          {/* Charts grid */}
          <div class={STORAGE_DISK_DETAIL_HISTORY_GRID_CLASS}>
            <For each={historyCharts()}>
              {(chart) => (
                <div class={STORAGE_DETAIL_CARD_CLASS}>
                  <HistoryChart
                    resourceType="disk"
                    resourceId={resId()!}
                    metric={chart.metric}
                    label={chart.label}
                    unit={chart.unit}
                    height={120}
                    color={chart.color}
                    range={chartRange()}
                    hideSelector={true}
                    compact={true}
                    hideLock={true}
                  />
                </div>
              )}
            </For>
          </div>
        </div>
      </Show>
    </div>
  );
};
