import { Component, For, Show } from 'solid-js';
import { HistoryChart } from '@/components/shared/HistoryChart';
import type { HistoryTimeRange } from '@/api/charts';
import {
  getLinkedDiskHealthDotClass,
  getLinkedDiskTemperatureTextClass,
} from '@/features/storageBackups/diskDetailPresentation';
import {
  STORAGE_POOL_DETAIL_HISTORY_RANGE_OPTIONS,
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
}

export const StoragePoolDetail: Component<StoragePoolDetailProps> = (props) => {
  const { chartRange, setChartRange, chartTarget, configRows, zfsSummary, linkedDisks } =
    useStoragePoolDetailModel({
      record: () => props.record,
      physicalDisks: () => props.physicalDisks,
    });

  return (
    <tr class={STORAGE_DETAIL_ROW_CLASS}>
      <td colSpan={99} class={STORAGE_DETAIL_CELL_CLASS}>
        <div class={STORAGE_DETAIL_ROOT_GRID_CLASS}>
          {/* Left: Capacity trend chart */}
          <div class={STORAGE_DETAIL_CARD_CLASS}>
            <div class={STORAGE_DETAIL_HEADER_ROW_CLASS}>
              <h4 class={STORAGE_DETAIL_SECTION_TITLE_CLASS}>Capacity Trend</h4>
              <select
                value={chartRange()}
                onChange={(e) => setChartRange(e.currentTarget.value as HistoryTimeRange)}
                class={STORAGE_DETAIL_SELECT_CLASS}
                style={STORAGE_DETAIL_SELECT_STYLE}
              >
                <For each={STORAGE_POOL_DETAIL_HISTORY_RANGE_OPTIONS}>
                  {(option) => <option value={option.value}>{option.label}</option>}
                </For>
              </select>
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
                        <span
                          class={`flex-shrink-0 ${getLinkedDiskHealthDotClass(disk.hasIssue)}`}
                        />
                        <span class={STORAGE_DETAIL_LINKED_DISK_MODEL_CLASS}>{disk.model}</span>
                        <Show when={disk.temperature > 0}>
                          <span
                            class={`font-medium ${getLinkedDiskTemperatureTextClass(
                              disk.temperature,
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
