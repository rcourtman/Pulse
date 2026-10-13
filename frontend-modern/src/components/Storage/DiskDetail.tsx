import { Component, For, Show, createEffect, createMemo } from 'solid-js';
import type { Resource } from '@/types/resource';
import { FormSelect } from '@/components/shared/FormSelect';
import { filterSelectClass } from '@/components/shared/FilterToolbar';
import { HistoryChart, HistoryChartHoverGroup } from '@/components/shared/HistoryChart';
import type { HistoryTimeRange } from '@/api/charts';
import { maxHistoryDays } from '@/stores/license';
import {
  getUnlockedHistoryRangeOptions,
  resolveHistoryRangeWithinLimit,
} from '@/components/Storage/historyRangeAccess';
import {
  DISK_DETAIL_HEADER_STACK_CLASS,
  DISK_DETAIL_HEALTH_LABEL_CLASS,
  DISK_DETAIL_HEALTH_ROW_CLASS,
  DISK_DETAIL_HEALTH_SUMMARY_CLASS,
  DISK_DETAIL_HISTORY_RANGE_OPTIONS,
  DISK_DETAIL_LIVE_CHARTS,
  getDiskAttributeCardValueTextClass,
  getDiskDetailHistoryFallbackMessage,
  getDiskDetailLiveBadgeLabel,
} from '@/features/storageBackups/diskDetailPresentation';
import {
  STORAGE_DETAIL_BADGE_CLASS,
  STORAGE_DETAIL_CARD_CLASS,
  STORAGE_DETAIL_EMPTY_CLASS,
  STORAGE_DETAIL_HEADER_ROW_CLASS,
  STORAGE_DISK_DETAIL_ATTRIBUTE_GRID_CLASS,
  STORAGE_DISK_DETAIL_HEADER_CLASS,
  STORAGE_DISK_DETAIL_HISTORY_GRID_CLASS,
  STORAGE_DISK_DETAIL_LIVE_GRID_CLASS,
  STORAGE_DISK_DETAIL_MODEL_CLASS,
  STORAGE_DISK_DETAIL_NODE_CLASS,
  STORAGE_DISK_DETAIL_ROOT_CLASS,
  STORAGE_DISK_DETAIL_SECTION_CLASS,
  STORAGE_DISK_DETAIL_SECTION_HEADING_CLASS,
  STORAGE_DISK_DETAIL_SERIAL_CLASS,
  STORAGE_DETAIL_META_ROW_CLASS,
  STORAGE_DETAIL_MONO_CHIP_CLASS,
  STORAGE_DETAIL_SECTION_TITLE_CLASS,
} from '@/features/storageBackups/detailPresentation';
import { getPhysicalDiskSourceBadgePresentation } from '@/features/storageBackups/diskPresentation';
import { StorageDetailMetricCard } from './StorageDetailMetricCard';
import { useDiskDetailModel } from './useDiskDetailModel';

interface DiskDetailProps {
  disk: Resource;
  nodes: Resource[];
  /** Alert override keys of the machine whose agent reports the disk. */
  alertResourceIds?: string[];
}

export const DiskDetail: Component<DiskDetailProps> = (props) => {
  const {
    chartRange,
    setChartRange,
    diskData,
    historyResourceId,
    attributeCards,
    historyCharts,
    healthPresentation,
    metricResourceId,
    collectionMessages,
    liveIOAvailable,
  } = useDiskDetailModel({
    disk: () => props.disk,
    alertResourceIds: () => props.alertResourceIds,
  });
  const rangeOptions = createMemo(() =>
    getUnlockedHistoryRangeOptions(DISK_DETAIL_HISTORY_RANGE_OPTIONS, maxHistoryDays()),
  );
  const hasOverviewDetails = createMemo(
    () => collectionMessages().length > 0 || attributeCards().length > 0,
  );

  createEffect(() => {
    const nextRange = resolveHistoryRangeWithinLimit(
      chartRange(),
      DISK_DETAIL_HISTORY_RANGE_OPTIONS,
      maxHistoryDays(),
    );
    if (nextRange !== chartRange()) {
      setChartRange(nextRange);
    }
  });

  return (
    <div class={STORAGE_DISK_DETAIL_ROOT_CLASS}>
      <div class={STORAGE_DISK_DETAIL_HEADER_CLASS}>
        <div class={DISK_DETAIL_HEADER_STACK_CLASS}>
          <div class={STORAGE_DETAIL_META_ROW_CLASS}>
            <span class={STORAGE_DISK_DETAIL_MODEL_CLASS}>
              {diskData().model || 'Unknown Disk'}
            </span>
            <span class={STORAGE_DETAIL_MONO_CHIP_CLASS}>{diskData().devPath}</span>
            <span class={STORAGE_DISK_DETAIL_NODE_CLASS}>{diskData().node}</span>
            <span
              class={getPhysicalDiskSourceBadgePresentation(props.disk).className}
              title="Data source"
            >
              {getPhysicalDiskSourceBadgePresentation(props.disk).label}
            </span>
            <Show when={diskData().serial}>
              <span class={STORAGE_DISK_DETAIL_SERIAL_CLASS}>S/N: {diskData().serial}</span>
            </Show>
          </div>
          {/* The verdict the row shows as a single word, with its reason in full. */}
          <div class={DISK_DETAIL_HEALTH_ROW_CLASS} data-testid="disk-detail-health">
            <span class={`${DISK_DETAIL_HEALTH_LABEL_CLASS} ${healthPresentation().tone}`}>
              {healthPresentation().label}
            </span>
            <Show when={healthPresentation().summary}>
              <span class={DISK_DETAIL_HEALTH_SUMMARY_CLASS}>{healthPresentation().summary}</span>
            </Show>
          </div>
        </div>
      </div>

      {/* One view: the SMART evidence behind the health verdict, then its
          trend and live I/O. There is no separate History tab. */}
      <div class="space-y-3">
        <Show when={collectionMessages().length > 0}>
          <div class={STORAGE_DETAIL_EMPTY_CLASS} role="status">
            <For each={collectionMessages()}>{(message) => <p>{message}</p>}</For>
          </div>
        </Show>

        <Show when={attributeCards().length > 0}>
          <div class={STORAGE_DISK_DETAIL_ATTRIBUTE_GRID_CLASS}>
            <For each={attributeCards()}>
              {(card) => (
                <StorageDetailMetricCard
                  label={card.label}
                  value={card.value}
                  valueClass={getDiskAttributeCardValueTextClass(card)}
                />
              )}
            </For>
          </div>
        </Show>

        <Show when={!hasOverviewDetails()}>
          <div class={STORAGE_DETAIL_EMPTY_CLASS} role="status">
            Detailed SMART attributes are not available for this disk.
          </div>
        </Show>

        <Show
          when={historyResourceId()}
          fallback={
            <div class={STORAGE_DETAIL_EMPTY_CLASS}>{getDiskDetailHistoryFallbackMessage()}</div>
          }
        >
          <div class={STORAGE_DISK_DETAIL_SECTION_CLASS} data-testid="disk-detail-history">
            <div class={STORAGE_DETAIL_HEADER_ROW_CLASS}>
              <h4 class={STORAGE_DETAIL_SECTION_TITLE_CLASS}>History</h4>
              <FormSelect
                label="Disk history range"
                labelClass="sr-only"
                fieldBaseClass="contents"
                value={chartRange()}
                onChange={(event) => setChartRange(event.currentTarget.value as HistoryTimeRange)}
                selectBaseClass={`${filterSelectClass} h-7 min-h-11 py-0 text-[11px] sm:min-h-0`}
              >
                <For each={rangeOptions()}>
                  {(option) => <option value={option.value}>{option.label}</option>}
                </For>
              </FormSelect>
            </div>
            <HistoryChartHoverGroup>
              <div class={STORAGE_DISK_DETAIL_HISTORY_GRID_CLASS}>
                <For each={historyCharts()}>
                  {(chart) => (
                    <div class={STORAGE_DETAIL_CARD_CLASS}>
                      <HistoryChart
                        resourceType="disk"
                        resourceId={historyResourceId()!}
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
            </HistoryChartHoverGroup>
          </div>
        </Show>

        <Show when={metricResourceId() && liveIOAvailable()}>
          <div class={STORAGE_DISK_DETAIL_SECTION_CLASS}>
            <h4
              class={`${STORAGE_DETAIL_SECTION_TITLE_CLASS} ${STORAGE_DISK_DETAIL_SECTION_HEADING_CLASS}`}
            >
              Live I/O (30m)
              <span class={STORAGE_DETAIL_BADGE_CLASS}>{getDiskDetailLiveBadgeLabel()}</span>
            </h4>
            <HistoryChartHoverGroup>
              <div class={STORAGE_DISK_DETAIL_LIVE_GRID_CLASS}>
                <For each={DISK_DETAIL_LIVE_CHARTS}>
                  {(chart) => (
                    <div class={STORAGE_DETAIL_CARD_CLASS}>
                      <HistoryChart
                        resourceType="disk"
                        resourceId={metricResourceId()!}
                        metric={
                          chart.series === 'read'
                            ? 'diskread'
                            : chart.series === 'write'
                              ? 'diskwrite'
                              : 'disk'
                        }
                        label={chart.label}
                        unit={chart.unit}
                        range="30m"
                        hideSelector
                        hideLock
                        height={120}
                        compact={true}
                      />
                    </div>
                  )}
                </For>
              </div>
            </HistoryChartHoverGroup>
          </div>
        </Show>
      </div>
    </div>
  );
};
