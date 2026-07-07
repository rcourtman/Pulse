import { Component, For, Show } from 'solid-js';
import {
  buildSummaryDisclosureControlsId,
  createSummaryInteractiveRowPreviewHandlers,
} from '@/components/shared/summaryInteractionA11y';
import { SummaryRowActionButton } from '@/components/shared/SummaryRowActionButton';
import { resolvePhysicalDiskMetricResourceId } from '@/features/storageBackups/storageMetricsIdentity';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { formatBytes } from '@/utils/format';
import { formatTemperature, getTemperatureTextClass } from '@/utils/temperature';
import {
  PHYSICAL_DISK_CELL_DEVICE_CLASS,
  PHYSICAL_DISK_CELL_DISK_CLASS,
  PHYSICAL_DISK_CELL_HEALTH_CLASS,
  PHYSICAL_DISK_CELL_HOST_CLASS,
  PHYSICAL_DISK_CELL_LIFE_CLASS,
  PHYSICAL_DISK_CELL_PARENT_CLASS,
  PHYSICAL_DISK_CELL_ROLE_CLASS,
  PHYSICAL_DISK_CELL_SIZE_CLASS,
  PHYSICAL_DISK_CELL_TEMP_CLASS,
  PHYSICAL_DISK_COL_DEVICE_CLASS,
  PHYSICAL_DISK_COL_DISK_CLASS,
  PHYSICAL_DISK_COL_HEALTH_CLASS,
  PHYSICAL_DISK_COL_HOST_CLASS,
  PHYSICAL_DISK_COL_LIFE_CLASS,
  PHYSICAL_DISK_COL_PARENT_CLASS,
  PHYSICAL_DISK_COL_ROLE_CLASS,
  PHYSICAL_DISK_COL_SIZE_CLASS,
  PHYSICAL_DISK_COL_TEMP_CLASS,
  PHYSICAL_DISK_DETAIL_ROW_CELL_CLASS,
  PHYSICAL_DISK_EMPTY_CARD_CLASS,
  PHYSICAL_DISK_EMPTY_FALLBACK_CLASS,
  PHYSICAL_DISK_EMPTY_FALLBACK_TEXT_CLASS,
  PHYSICAL_DISK_EMPTY_MESSAGE_CLASS,
  PHYSICAL_DISK_EMPTY_REQUIREMENTS_CLASS,
  PHYSICAL_DISK_EMPTY_REQUIREMENTS_LIST_CLASS,
  PHYSICAL_DISK_EMPTY_REQUIREMENTS_NOTE_CLASS,
  PHYSICAL_DISK_EMPTY_REQUIREMENTS_TITLE_CLASS,
  PHYSICAL_DISK_EMPTY_TITLE_CLASS,
  PHYSICAL_DISK_HEADER_DEVICE_CLASS,
  PHYSICAL_DISK_HEADER_DISK_CLASS,
  PHYSICAL_DISK_HEADER_HEALTH_CLASS,
  PHYSICAL_DISK_HEADER_HOST_CLASS,
  PHYSICAL_DISK_HEADER_LIFE_CLASS,
  PHYSICAL_DISK_HEADER_PARENT_CLASS,
  PHYSICAL_DISK_HEADER_ROLE_CLASS,
  PHYSICAL_DISK_HEADER_SIZE_CLASS,
  PHYSICAL_DISK_HEADER_TEMP_CLASS,
  PHYSICAL_DISK_HEALTH_LABEL_CLASS,
  PHYSICAL_DISK_HEALTH_SUMMARY_CLASS,
  PHYSICAL_DISK_HEALTH_WRAP_CLASS,
  PHYSICAL_DISK_DEVICE_TEXT_CLASS,
  PHYSICAL_DISK_LIFE_CLASS,
  PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS,
  PHYSICAL_DISK_NAME_TEXT_CLASS,
  PHYSICAL_DISK_NAME_WRAP_CLASS,
  PHYSICAL_DISK_SIZE_VALUE_CLASS,
  PHYSICAL_DISK_TEMPERATURE_CLASS,
  PHYSICAL_DISK_VALUE_TEXT_CLASS,
  PHYSICAL_DISK_TABLE_BODY_CLASS,
  PHYSICAL_DISK_TABLE_CLASS,
  PHYSICAL_DISK_TABLE_HEADER_ROW_CLASS,
  PHYSICAL_DISK_TABLE_ROW_CLASS,
  PHYSICAL_DISK_TABLE_ROW_HOVER_CLASS,
  PHYSICAL_DISK_TABLE_ROW_SELECTED_CLASS,
  PHYSICAL_DISK_TABLE_ROW_STYLE,
  getPhysicalDiskEmptyStatePresentation,
  getPhysicalDiskHealthStatus,
  getPhysicalDiskHealthSummary,
  getPhysicalDiskHostLabel,
  getPhysicalDiskLifeLabel,
  getPhysicalDiskLifeTextClass,
  getPhysicalDiskParentLabel,
  getPhysicalDiskRoleLabel,
} from '@/features/storageBackups/diskPresentation';
import type { Resource } from '@/types/resource';
import type { StorageHealthFilter } from '@/features/storageBackups/models';
import { getStorageSourceOption } from '@/utils/storageSources';
import { DiskDetail } from './DiskDetail';
import { useDiskListModel } from './useDiskListModel';

interface DiskListProps {
  disks: Resource[];
  nodes: Resource[];
  selectedNode: string | null;
  sourceFilter?: string;
  healthFilter?: StorageHealthFilter;
  roleFilter?: string;
  groupFilter?: string;
  searchTerm: string;
  selectedDiskId: string | null;
  highlightedSummarySeriesId?: string | null;
  onSelectedDiskChange: (diskId: string | null) => void;
  onHoverChange?: (diskId: string | null) => void;
}

export const DiskList: Component<DiskListProps> = (props) => {
  const { getDiskTemperatureThresholds } = useAlertsActivation();
  const model = useDiskListModel({
    disks: () => props.disks,
    nodes: () => props.nodes,
    selectedNode: () => props.selectedNode,
    sourceFilter: () => props.sourceFilter ?? 'all',
    healthFilter: () => props.healthFilter ?? 'all',
    roleFilter: () => props.roleFilter ?? 'all',
    groupFilter: () => props.groupFilter ?? 'all',
    searchTerm: () => props.searchTerm,
    selectedDiskId: () => props.selectedDiskId,
    setSelectedDiskId: props.onSelectedDiskChange,
  });
  const emptyState = () =>
    getPhysicalDiskEmptyStatePresentation({
      selectedNodeName: model.selectedNodeName(),
      searchTerm: props.searchTerm,
      diskCount: (props.disks || []).length,
      hasPVENodes: model.hasPVENodes(),
      healthFilter: props.healthFilter ?? 'all',
      sourceFilterLabel:
        props.sourceFilter && props.sourceFilter !== 'all'
          ? getStorageSourceOption(props.sourceFilter).label
          : null,
      roleFilterLabel:
        props.roleFilter && props.roleFilter !== 'all'
          ? (model.roleFilterOptions().find((option) => option.value === props.roleFilter)?.label ??
            null)
          : null,
      groupFilterLabel:
        props.groupFilter && props.groupFilter !== 'all'
          ? (model.groupFilterOptions().find((option) => option.value === props.groupFilter)
              ?.label ?? null)
          : null,
    });

  return (
    <div>
      <Show when={model.filteredDisks().length === 0}>
        <div class={`p-4 sm:p-6 ${PHYSICAL_DISK_EMPTY_CARD_CLASS}`}>
          <div class="">
            <p class={PHYSICAL_DISK_EMPTY_TITLE_CLASS}>{emptyState().title}</p>
            <Show when={emptyState().nodeMessage}>
              {(message) => <p class={PHYSICAL_DISK_EMPTY_MESSAGE_CLASS}>{message()}</p>}
            </Show>
            <Show when={emptyState().searchMessage}>
              {(message) => <p class={PHYSICAL_DISK_EMPTY_MESSAGE_CLASS}>{message()}</p>}
            </Show>
            <For each={emptyState().filterMessages}>
              {(message) => <p class={PHYSICAL_DISK_EMPTY_MESSAGE_CLASS}>{message}</p>}
            </For>
          </div>
          <Show when={!props.searchTerm && (props.disks || []).length === 0}>
            <Show
              when={emptyState().showRequirements}
              fallback={
                <div class={PHYSICAL_DISK_EMPTY_FALLBACK_CLASS}>
                  <p class={PHYSICAL_DISK_EMPTY_FALLBACK_TEXT_CLASS}>
                    {emptyState().fallbackMessage}
                  </p>
                </div>
              }
            >
              <div class={PHYSICAL_DISK_EMPTY_REQUIREMENTS_CLASS}>
                <p class={PHYSICAL_DISK_EMPTY_REQUIREMENTS_TITLE_CLASS}>
                  {emptyState().requirementsTitle}
                </p>
                <ol class={PHYSICAL_DISK_EMPTY_REQUIREMENTS_LIST_CLASS}>
                  <For each={emptyState().requirementsItems}>{(item) => <li>{item}</li>}</For>
                </ol>
                <p class={PHYSICAL_DISK_EMPTY_REQUIREMENTS_NOTE_CLASS}>
                  {emptyState().requirementsNote}
                </p>
              </div>
            </Show>
          </Show>
        </div>
      </Show>

      <Show when={model.filteredDisks().length > 0}>
        <Table class={PHYSICAL_DISK_TABLE_CLASS}>
          <colgroup>
            <col class={PHYSICAL_DISK_COL_DISK_CLASS} />
            <col class={PHYSICAL_DISK_COL_DEVICE_CLASS} />
            <col class={PHYSICAL_DISK_COL_HOST_CLASS} />
            <col class={PHYSICAL_DISK_COL_ROLE_CLASS} />
            <col class={PHYSICAL_DISK_COL_PARENT_CLASS} />
            <col class={PHYSICAL_DISK_COL_HEALTH_CLASS} />
            <col class={PHYSICAL_DISK_COL_LIFE_CLASS} />
            <col class={PHYSICAL_DISK_COL_TEMP_CLASS} />
            <col class={PHYSICAL_DISK_COL_SIZE_CLASS} />
          </colgroup>
          <TableHeader>
            <TableRow class={PHYSICAL_DISK_TABLE_HEADER_ROW_CLASS}>
              <TableHead class={PHYSICAL_DISK_HEADER_DISK_CLASS}>Disk</TableHead>
              <TableHead class={PHYSICAL_DISK_HEADER_DEVICE_CLASS}>Device</TableHead>
              <TableHead class={PHYSICAL_DISK_HEADER_HOST_CLASS}>Host</TableHead>
              <TableHead class={PHYSICAL_DISK_HEADER_ROLE_CLASS}>Role</TableHead>
              <TableHead
                class={PHYSICAL_DISK_HEADER_PARENT_CLASS}
                aria-label="Belongs To"
                title="Belongs To"
              >
                Belongs
              </TableHead>
              <TableHead class={PHYSICAL_DISK_HEADER_HEALTH_CLASS}>Health</TableHead>
              <TableHead
                class={PHYSICAL_DISK_HEADER_LIFE_CLASS}
                aria-label="SSD life remaining"
                title="SSD life remaining"
              >
                Life
              </TableHead>
              <TableHead class={PHYSICAL_DISK_HEADER_TEMP_CLASS}>Temp</TableHead>
              <TableHead class={PHYSICAL_DISK_HEADER_SIZE_CLASS}>Size</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody class={PHYSICAL_DISK_TABLE_BODY_CLASS}>
            <For each={model.filteredDisks()}>
              {(disk) => {
                const data = model.getDiskData(disk);
                const status = getPhysicalDiskHealthStatus(data);
                const hostLabel = getPhysicalDiskHostLabel(data, disk);
                const healthSummary = getPhysicalDiskHealthSummary(status);
                const isSelected = () => model.selectedDisk()?.id === disk.id;
                const summarySeriesId = resolvePhysicalDiskMetricResourceId(disk);
                const isSummaryHighlighted = () =>
                  props.highlightedSummarySeriesId === summarySeriesId;
                const detailControlsId = buildSummaryDisclosureControlsId(summarySeriesId);
                const interactiveRowHandlers = createSummaryInteractiveRowPreviewHandlers({
                  onPreview: () => props.onHoverChange?.(disk.id),
                  onPreviewClear: () => props.onHoverChange?.(null),
                });

                return (
                  <>
                    <TableRow
                      data-row-id={disk.id}
                      data-summary-series-id={summarySeriesId}
                      data-summary-row-active={
                        isSummaryHighlighted() && !isSelected() ? 'true' : 'false'
                      }
                      class={`${PHYSICAL_DISK_TABLE_ROW_CLASS} ${
                        isSelected()
                          ? PHYSICAL_DISK_TABLE_ROW_SELECTED_CLASS
                          : PHYSICAL_DISK_TABLE_ROW_HOVER_CLASS
                      }`.trim()}
                      style={PHYSICAL_DISK_TABLE_ROW_STYLE}
                      onClick={() => model.toggleSelectedDisk(disk)}
                      {...interactiveRowHandlers}
                    >
                      <TableCell class={PHYSICAL_DISK_CELL_DISK_CLASS}>
                        <div class={PHYSICAL_DISK_NAME_WRAP_CLASS}>
                          <SummaryRowActionButton
                            kind="disclosure"
                            subjectLabel={data.model || 'disk'}
                            expanded={isSelected()}
                            controlsId={detailControlsId}
                            onAction={() => model.toggleSelectedDisk(disk)}
                            onPreviewClear={() => props.onHoverChange?.(null)}
                          />
                          <span
                            class={PHYSICAL_DISK_NAME_TEXT_CLASS}
                            title={data.devPath || data.model || disk.name || 'Unknown Disk'}
                          >
                            {data.model || 'Unknown Disk'}
                          </span>
                        </div>
                      </TableCell>

                      <TableCell class={PHYSICAL_DISK_CELL_DEVICE_CLASS}>
                        <Show
                          when={data.devPath}
                          fallback={<span class={PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS}>—</span>}
                        >
                          <span class={PHYSICAL_DISK_DEVICE_TEXT_CLASS} title={data.devPath}>
                            {data.devPath}
                          </span>
                        </Show>
                      </TableCell>

                      <TableCell class={PHYSICAL_DISK_CELL_HOST_CLASS}>
                        <Show
                          when={hostLabel}
                          fallback={<span class={PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS}>—</span>}
                        >
                          <span class={PHYSICAL_DISK_VALUE_TEXT_CLASS} title={hostLabel}>
                            {hostLabel}
                          </span>
                        </Show>
                      </TableCell>

                      <TableCell class={PHYSICAL_DISK_CELL_ROLE_CLASS}>
                        <Show
                          when={getPhysicalDiskRoleLabel(data)}
                          fallback={<span class={PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS}>—</span>}
                        >
                          <span
                            class={PHYSICAL_DISK_VALUE_TEXT_CLASS}
                            title={getPhysicalDiskRoleLabel(data)}
                          >
                            {getPhysicalDiskRoleLabel(data)}
                          </span>
                        </Show>
                      </TableCell>

                      <TableCell class={PHYSICAL_DISK_CELL_PARENT_CLASS}>
                        <Show
                          when={getPhysicalDiskParentLabel(data)}
                          fallback={<span class={PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS}>—</span>}
                        >
                          <span
                            class={PHYSICAL_DISK_VALUE_TEXT_CLASS}
                            title={getPhysicalDiskParentLabel(data)}
                          >
                            {getPhysicalDiskParentLabel(data)}
                          </span>
                        </Show>
                      </TableCell>

                      <TableCell class={PHYSICAL_DISK_CELL_HEALTH_CLASS}>
                        <div class={PHYSICAL_DISK_HEALTH_WRAP_CLASS}>
                          <span class={`${PHYSICAL_DISK_HEALTH_LABEL_CLASS} ${status.tone}`}>
                            {status.label}
                          </span>
                          <Show when={healthSummary}>
                            <span class={PHYSICAL_DISK_HEALTH_SUMMARY_CLASS} title={healthSummary}>
                              {healthSummary}
                            </span>
                          </Show>
                        </div>
                      </TableCell>

                      <TableCell class={PHYSICAL_DISK_CELL_LIFE_CLASS}>
                        <Show
                          when={getPhysicalDiskLifeLabel(data)}
                          fallback={<span class={PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS}>—</span>}
                        >
                          <span
                            class={`${PHYSICAL_DISK_LIFE_CLASS} ${getPhysicalDiskLifeTextClass(data)}`}
                          >
                            {getPhysicalDiskLifeLabel(data)}
                          </span>
                        </Show>
                      </TableCell>

                      <TableCell class={PHYSICAL_DISK_CELL_TEMP_CLASS}>
                        <Show
                          when={data.temperature > 0}
                          fallback={<span class={PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS}>—</span>}
                        >
                          <span
                            class={`${PHYSICAL_DISK_TEMPERATURE_CLASS} ${getTemperatureTextClass(
                              data.temperature,
                              getDiskTemperatureThresholds(data.type),
                              'diskTemperature',
                            )}`}
                          >
                            {formatTemperature(data.temperature)}
                          </span>
                        </Show>
                      </TableCell>

                      <TableCell class={PHYSICAL_DISK_CELL_SIZE_CLASS}>
                        <Show
                          when={data.size > 0}
                          fallback={
                            <span
                              class={PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS}
                              title="Disk size not reported by SMART/agent"
                            >
                              —
                            </span>
                          }
                        >
                          <span class={PHYSICAL_DISK_SIZE_VALUE_CLASS}>
                            {formatBytes(data.size)}
                          </span>
                        </Show>
                      </TableCell>
                    </TableRow>
                    <Show when={isSelected()}>
                      <TableRow data-inline-detail-for={summarySeriesId}>
                        <TableCell
                          id={detailControlsId}
                          colSpan={9}
                          class={PHYSICAL_DISK_DETAIL_ROW_CELL_CLASS}
                        >
                          <DiskDetail disk={disk} nodes={props.nodes} />
                        </TableCell>
                      </TableRow>
                    </Show>
                  </>
                );
              }}
            </For>
          </TableBody>
        </Table>
      </Show>
    </div>
  );
};
