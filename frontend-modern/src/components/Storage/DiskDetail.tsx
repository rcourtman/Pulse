import { Component, For, Show, createEffect, createMemo, createSignal } from 'solid-js';
import type { Resource } from '@/types/resource';
import { FormSelect } from '@/components/shared/FormSelect';
import { filterSelectClass } from '@/components/shared/FilterToolbar';
import { HistoryChart, HistoryChartHoverGroup } from '@/components/shared/HistoryChart';
import { Subtabs, type SubtabOption } from '@/components/shared/Subtabs';
import type { HistoryTimeRange } from '@/api/charts';
import { maxHistoryDays } from '@/stores/license';
import {
  getUnlockedHistoryRangeOptions,
  resolveHistoryRangeWithinLimit,
} from '@/components/Storage/historyRangeAccess';
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
}

type DiskDetailTab = 'overview' | 'history';

export const DiskDetail: Component<DiskDetailProps> = (props) => {
  const [activeTab, setActiveTab] = createSignal<DiskDetailTab>('overview');
  const {
    chartRange,
    setChartRange,
    diskData,
    historyResourceId,
    attributeCards,
    historyCharts,
    metricResourceId,
    collectionMessages,
    liveIOAvailable,
  } = useDiskDetailModel({
    disk: () => props.disk,
  });
  const rangeOptions = createMemo(() =>
    getUnlockedHistoryRangeOptions(DISK_DETAIL_HISTORY_RANGE_OPTIONS, maxHistoryDays()),
  );
  const hasHistory = createMemo(
    () => Boolean(historyResourceId()) || Boolean(metricResourceId() && liveIOAvailable()),
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

  createEffect(() => {
    if (activeTab() === 'history' && !hasHistory()) {
      setActiveTab('overview');
    }
  });

  return (
    <div class={STORAGE_DISK_DETAIL_ROOT_CLASS}>
      <div class={STORAGE_DISK_DETAIL_HEADER_CLASS}>
        <div class={STORAGE_DETAIL_META_ROW_CLASS}>
          <span class={STORAGE_DISK_DETAIL_MODEL_CLASS}>{diskData().model || 'Unknown Disk'}</span>
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
      </div>

      <Subtabs
        class="mb-1"
        ariaLabel="Physical disk detail sections"
        value={activeTab()}
        onChange={(value) => setActiveTab(value as DiskDetailTab)}
        tabs={[
          { value: 'overview', label: 'Overview' },
          ...(hasHistory() ? [{ value: 'history', label: 'History' } satisfies SubtabOption] : []),
        ]}
        trailing={
          <Show when={activeTab() === 'history'}>
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
          </Show>
        }
      />

      <div class={activeTab() === 'overview' ? 'space-y-3' : 'hidden'}>
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
                  valueClass={getDiskAttributeValueTextClass(card.ok)}
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
      </div>

      <div
        class={activeTab() === 'history' ? 'space-y-3' : 'hidden'}
        style={{ 'overflow-anchor': 'none' }}
      >
        <Show when={activeTab() === 'history'}>
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

          <Show
            when={historyResourceId()}
            fallback={
              <div class={STORAGE_DETAIL_EMPTY_CLASS}>{getDiskDetailHistoryFallbackMessage()}</div>
            }
          >
            <div class={STORAGE_DISK_DETAIL_SECTION_CLASS}>
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
        </Show>
      </div>
    </div>
  );
};
