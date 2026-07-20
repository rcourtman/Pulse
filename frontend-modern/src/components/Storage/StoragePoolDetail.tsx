import { Component, For, Show, createEffect, createMemo } from 'solid-js';
import { FormSelect } from '@/components/shared/FormSelect';
import { HistoryChart } from '@/components/shared/HistoryChart';
import { StatusDot } from '@/components/shared/StatusDot';
import type { HistoryTimeRange } from '@/api/charts';
import { maxHistoryDays } from '@/stores/license';
import {
  getUnlockedHistoryRangeOptions,
  resolveHistoryRangeWithinLimit,
} from '@/components/Storage/historyRangeAccess';
import {
  getLinkedDiskHealthDotVariant,
  getLinkedDiskTemperatureTextClass,
} from '@/features/storageBackups/diskDetailPresentation';
import { useAlertsActivation } from '@/stores/alertsActivation';
import {
  STORAGE_POOL_DETAIL_HISTORY_RANGE_OPTIONS,
  getZfsDeviceStateTextClass,
  getZfsErrorTextClass,
  getZfsScanTextClass,
} from '@/features/storageBackups/storagePoolDetailPresentation';
import {
  STORAGE_DETAIL_CARD_CLASS,
  STORAGE_DETAIL_CELL_CLASS,
  STORAGE_DETAIL_CONFIG_GRID_CLASS,
  STORAGE_DETAIL_FULL_WIDTH_ROW_CLASS,
  STORAGE_DETAIL_HEADER_ROW_CLASS,
  STORAGE_DETAIL_LINKED_DISK_PATH_CLASS,
  STORAGE_DETAIL_LINKED_DISK_MODEL_CLASS,
  STORAGE_DETAIL_LINKED_DISK_ROW_CLASS,
  STORAGE_DETAIL_LINKED_DISKS_LIST_CLASS,
  STORAGE_DETAIL_MUTED_TEXT_CLASS,
  STORAGE_DETAIL_ROOT_GRID_CLASS,
  STORAGE_DETAIL_ROW_CLASS,
  STORAGE_DETAIL_SECTION_TITLE_CLASS,
  STORAGE_DETAIL_SECTION_TITLE_SPACED_CLASS,
  STORAGE_DETAIL_SPACED_STACK_CLASS,
  STORAGE_DETAIL_SELECT_CLASS,
  STORAGE_DETAIL_SELECT_STYLE,
} from '@/features/storageBackups/detailPresentation';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { Resource } from '@/types/resource';
import { StorageDetailKeyValueRow } from './StorageDetailKeyValueRow';
import { useStoragePoolDetailModel } from './useStoragePoolDetailModel';

interface StoragePoolDetailProps {
  record: StorageRecord;
  physicalDisks: Resource[];
  summarySeriesId: string;
  controlsId?: string;
}

export const StoragePoolDetail: Component<StoragePoolDetailProps> = (props) => {
  const { getDiskTemperatureThresholds } = useAlertsActivation();
  const {
    chartRange,
    setChartRange,
    chartTarget,
    configRows,
    topologyRows,
    zfsSummary,
    linkedDisks,
  } = useStoragePoolDetailModel({
    record: () => props.record,
    physicalDisks: () => props.physicalDisks,
  });
  const rangeOptions = createMemo(() =>
    getUnlockedHistoryRangeOptions(STORAGE_POOL_DETAIL_HISTORY_RANGE_OPTIONS, maxHistoryDays()),
  );

  createEffect(() => {
    const nextRange = resolveHistoryRangeWithinLimit(
      chartRange(),
      STORAGE_POOL_DETAIL_HISTORY_RANGE_OPTIONS,
      maxHistoryDays(),
    );
    if (nextRange !== chartRange()) {
      setChartRange(nextRange);
    }
  });

  return (
    <tr class={STORAGE_DETAIL_ROW_CLASS} data-inline-detail-for={props.summarySeriesId}>
      <td id={props.controlsId} colSpan={99} class={STORAGE_DETAIL_CELL_CLASS}>
        <div class={STORAGE_DETAIL_ROOT_GRID_CLASS}>
          {/* Left: Capacity trend chart */}
          <div class={STORAGE_DETAIL_CARD_CLASS}>
            <div class={STORAGE_DETAIL_HEADER_ROW_CLASS}>
              <h4 class={STORAGE_DETAIL_SECTION_TITLE_CLASS}>Capacity Trend</h4>
              <FormSelect
                label="Capacity trend range"
                labelClass="sr-only"
                fieldBaseClass="contents"
                value={chartRange()}
                onChange={(e) => setChartRange(e.currentTarget.value as HistoryTimeRange)}
                selectBaseClass={STORAGE_DETAIL_SELECT_CLASS}
                style={STORAGE_DETAIL_SELECT_STYLE}
              >
                <For each={rangeOptions()}>
                  {(option) => <option value={option.value}>{option.label}</option>}
                </For>
              </FormSelect>
            </div>
            <HistoryChart
              resourceType={chartTarget().resourceType}
              resourceId={chartTarget().resourceId || props.record.id}
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
          <div class={STORAGE_DETAIL_SPACED_STACK_CLASS}>
            <Show when={topologyRows().length > 0}>
              <div class={STORAGE_DETAIL_CARD_CLASS}>
                <h4 class={STORAGE_DETAIL_SECTION_TITLE_SPACED_CLASS}>Topology</h4>
                <div class={STORAGE_DETAIL_CONFIG_GRID_CLASS}>
                  <For each={topologyRows()}>
                    {(row) => <StorageDetailKeyValueRow label={row.label} value={row.value} />}
                  </For>
                </div>
              </div>
            </Show>

            {/* Config card */}
            <div class={STORAGE_DETAIL_CARD_CLASS}>
              <h4 class={STORAGE_DETAIL_SECTION_TITLE_SPACED_CLASS}>Configuration</h4>
              <div class={STORAGE_DETAIL_CONFIG_GRID_CLASS}>
                <For each={configRows()}>
                  {(row) => <StorageDetailKeyValueRow label={row.label} value={row.value} />}
                </For>
              </div>
            </div>

            {/* ZFS details */}
            <Show when={zfsSummary()}>
              <div class={STORAGE_DETAIL_CARD_CLASS}>
                <h4 class={STORAGE_DETAIL_SECTION_TITLE_SPACED_CLASS}>ZFS Pool</h4>
                <div class={STORAGE_DETAIL_CONFIG_GRID_CLASS}>
                  <StorageDetailKeyValueRow label="State" value={zfsSummary()!.state} />
                  <Show when={zfsSummary()!.scan}>
                    <div class={STORAGE_DETAIL_FULL_WIDTH_ROW_CLASS}>
                      <span class={STORAGE_DETAIL_MUTED_TEXT_CLASS}>Scan: </span>
                      <span class={getZfsScanTextClass()}>{zfsSummary()!.scan}</span>
                    </div>
                  </Show>
                  <Show when={zfsSummary()!.errorSummary}>
                    <div class={`${STORAGE_DETAIL_FULL_WIDTH_ROW_CLASS} ${getZfsErrorTextClass()}`}>
                      {zfsSummary()!.errorSummary}
                    </div>
                  </Show>
                  <Show when={zfsSummary()!.devices.length > 0}>
                    <div class={`${STORAGE_DETAIL_FULL_WIDTH_ROW_CLASS} space-y-0.5 pt-1`}>
                      <For each={zfsSummary()!.devices}>
                        {(device) => (
                          <div class={STORAGE_DETAIL_LINKED_DISK_ROW_CLASS}>
                            <span class={STORAGE_DETAIL_LINKED_DISK_PATH_CLASS} title={device.name}>
                              {device.name}
                            </span>
                            <Show when={device.type}>
                              <span class={STORAGE_DETAIL_MUTED_TEXT_CLASS}>{device.type}</span>
                            </Show>
                            <span class={getZfsDeviceStateTextClass(device.state)}>
                              {device.state}
                            </span>
                            <Show when={device.errorSummary}>
                              <span class={getZfsErrorTextClass()}>{device.errorSummary}</span>
                            </Show>
                            <Show when={device.message}>
                              <span class={`${STORAGE_DETAIL_MUTED_TEXT_CLASS} italic`}>
                                {device.message}
                              </span>
                            </Show>
                          </div>
                        )}
                      </For>
                    </div>
                  </Show>
                </div>
              </div>
            </Show>

            {/* Physical disks linked to this pool */}
            <Show when={linkedDisks().length > 0}>
              <div class={STORAGE_DETAIL_CARD_CLASS}>
                <h4 class={STORAGE_DETAIL_SECTION_TITLE_SPACED_CLASS}>
                  Physical Disks ({linkedDisks().length})
                </h4>
                <div class={STORAGE_DETAIL_LINKED_DISKS_LIST_CLASS}>
                  <For each={linkedDisks()}>
                    {(disk) => (
                      <div class={STORAGE_DETAIL_LINKED_DISK_ROW_CLASS}>
                        <span class={STORAGE_DETAIL_LINKED_DISK_PATH_CLASS} title={disk.devPath}>
                          {disk.devPath}
                        </span>
                        <StatusDot
                          variant={getLinkedDiskHealthDotVariant(disk.hasIssue)}
                          size="sm"
                          ariaHidden
                        />
                        <span class={STORAGE_DETAIL_LINKED_DISK_MODEL_CLASS}>{disk.model}</span>
                        <Show when={disk.role}>
                          <span class="flex-shrink-0 rounded bg-surface-alt px-1.5 py-0.5 text-[10px] uppercase text-muted">
                            {disk.role}
                          </span>
                        </Show>
                        <Show when={disk.sizeLabel}>
                          <span class="flex-shrink-0 font-mono text-muted">{disk.sizeLabel}</span>
                        </Show>
                        <Show when={disk.state}>
                          <span class="flex-shrink-0 text-muted">{disk.state}</span>
                        </Show>
                        <Show when={disk.ioLabel}>
                          <span class="flex-shrink-0 font-mono text-muted">{disk.ioLabel}</span>
                        </Show>
                        <Show when={disk.spunDown}>
                          <span class="flex-shrink-0 text-muted">spun down</span>
                        </Show>
                        <Show when={disk.errorCount > 0}>
                          <span class="flex-shrink-0 font-semibold text-amber-700 dark:text-amber-300">
                            {disk.errorCount.toLocaleString()} errors
                          </span>
                        </Show>
                        <Show when={disk.temperature > 0}>
                          <span
                            class={`font-medium ${getLinkedDiskTemperatureTextClass(
                              disk.temperature,
                              getDiskTemperatureThresholds(disk.diskType),
                            )}`}
                          >
                            {disk.temperature}°C
                          </span>
                        </Show>
                      </div>
                    )}
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
