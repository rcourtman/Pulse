import { Component, For, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import {
  buildSummaryDisclosureControlsId,
  createSummaryInteractiveRowPreviewHandlers,
} from '@/components/shared/summaryInteractionA11y';
import { SummaryRowActionButton } from '@/components/shared/SummaryRowActionButton';
import { useGlobalResourceContext } from '@/features/globalResourceContext/GlobalResourceContext';
import { resolvePhysicalDiskMetricResourceId } from '@/features/storageBackups/storageMetricsIdentity';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { formatBytes } from '@/utils/format';
import { formatTemperature, getTemperatureTextClass } from '@/utils/temperature';
import {
  PHYSICAL_DISK_CELL_DISK_CLASS,
  PHYSICAL_DISK_CELL_HEALTH_CLASS,
  PHYSICAL_DISK_CELL_HOST_CLASS,
  PHYSICAL_DISK_CELL_PARENT_CLASS,
  PHYSICAL_DISK_CELL_ROLE_CLASS,
  PHYSICAL_DISK_CELL_SIZE_CLASS,
  PHYSICAL_DISK_CELL_SOURCE_CLASS,
  PHYSICAL_DISK_CELL_TEMP_CLASS,
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
  PHYSICAL_DISK_HEADER_DISK_CLASS,
  PHYSICAL_DISK_HEADER_HEALTH_CLASS,
  PHYSICAL_DISK_HEADER_HOST_CLASS,
  PHYSICAL_DISK_HEADER_PARENT_CLASS,
  PHYSICAL_DISK_HEADER_ROLE_CLASS,
  PHYSICAL_DISK_HEADER_SIZE_CLASS,
  PHYSICAL_DISK_HEADER_SOURCE_CLASS,
  PHYSICAL_DISK_HEADER_TEMP_CLASS,
  PHYSICAL_DISK_HEALTH_LABEL_CLASS,
  PHYSICAL_DISK_HEALTH_SUMMARY_CLASS,
  PHYSICAL_DISK_HEALTH_WRAP_CLASS,
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
  PHYSICAL_DISK_TABLE_SCROLL_CLASS,
  getPhysicalDiskEmptyStatePresentation,
  getPhysicalDiskHealthStatus,
  getPhysicalDiskHealthSummary,
  getPhysicalDiskHostLabel,
  getPhysicalDiskParentLabel,
  getPhysicalDiskRoleLabel,
  getPhysicalDiskSourceBadgePresentation,
} from '@/features/storageBackups/diskPresentation';
import type { Resource } from '@/types/resource';
import { DiskDetail } from './DiskDetail';
import { useDiskListModel } from './useDiskListModel';

interface DiskListProps {
  disks: Resource[];
  nodes: Resource[];
  selectedNode: string | null;
  searchTerm: string;
  selectedDiskId: string | null;
  highlightedSummarySeriesId?: string | null;
  onSelectedDiskChange: (diskId: string | null) => void;
  onHoverChange?: (diskId: string | null) => void;
}

export const DiskList: Component<DiskListProps> = (props) => {
  const globalContext = useGlobalResourceContext();
  const model = useDiskListModel({
    disks: () => props.disks,
    nodes: () => props.nodes,
    selectedNode: () => props.selectedNode,
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
    });

  return (
    <div>
      <Show when={model.filteredDisks().length === 0}>
        <Card padding="lg" class={PHYSICAL_DISK_EMPTY_CARD_CLASS}>
          <div class="">
            <p class={PHYSICAL_DISK_EMPTY_TITLE_CLASS}>{emptyState().title}</p>
            <Show when={emptyState().nodeMessage}>
              {(message) => <p class={PHYSICAL_DISK_EMPTY_MESSAGE_CLASS}>{message()}</p>}
            </Show>
            <Show when={emptyState().searchMessage}>
              {(message) => <p class={PHYSICAL_DISK_EMPTY_MESSAGE_CLASS}>{message()}</p>}
            </Show>
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
        </Card>
      </Show>

      <Show when={model.filteredDisks().length > 0}>
        <Card padding="none" tone="card" class="overflow-hidden">
          <div
            class={PHYSICAL_DISK_TABLE_SCROLL_CLASS}
            style={{ '-webkit-overflow-scrolling': 'touch' }}
          >
            <Table class={PHYSICAL_DISK_TABLE_CLASS}>
              <TableHeader>
                <TableRow class={PHYSICAL_DISK_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={PHYSICAL_DISK_HEADER_DISK_CLASS}>Disk</TableHead>
                  <TableHead class={PHYSICAL_DISK_HEADER_SOURCE_CLASS}>Source</TableHead>
                  <TableHead class={PHYSICAL_DISK_HEADER_HOST_CLASS}>Host</TableHead>
                  <TableHead class={PHYSICAL_DISK_HEADER_ROLE_CLASS}>Role</TableHead>
                  <TableHead class={PHYSICAL_DISK_HEADER_PARENT_CLASS}>Belongs To</TableHead>
                  <TableHead class={PHYSICAL_DISK_HEADER_HEALTH_CLASS}>Health</TableHead>
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
                    const sourceBadge = getPhysicalDiskSourceBadgePresentation(disk);
                    const isSelected = () => model.selectedDisk()?.id === disk.id;
                    const summarySeriesId = resolvePhysicalDiskMetricResourceId(disk);
                    const isSummaryHighlighted = () =>
                      props.highlightedSummarySeriesId === summarySeriesId;
                    const detailControlsId = buildSummaryDisclosureControlsId(summarySeriesId);
                    const supportsGlobalContext = () => globalContext.canPinResourceId(disk.id);
                    const isGlobalContextPinned = () =>
                      globalContext.isPinnedGlobalResource(disk.id);
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
                              <Show when={supportsGlobalContext()}>
                                <SummaryRowActionButton
                                  kind="context"
                                  subjectLabel={data.model || 'disk'}
                                  pressed={isGlobalContextPinned()}
                                  onAction={() =>
                                    globalContext.setGlobalResourceContextById(
                                      isGlobalContextPinned() ? null : disk.id,
                                    )
                                  }
                                  onPreviewClear={() => props.onHoverChange?.(null)}
                                />
                              </Show>
                              <span
                                class={PHYSICAL_DISK_NAME_TEXT_CLASS}
                                title={data.devPath || data.model || disk.name || 'Unknown Disk'}
                              >
                                {data.model || 'Unknown Disk'}
                              </span>
                            </div>
                          </TableCell>

                          <TableCell class={PHYSICAL_DISK_CELL_SOURCE_CLASS}>
                            <span class={sourceBadge.className}>{sourceBadge.label}</span>
                          </TableCell>

                          <TableCell class={PHYSICAL_DISK_CELL_HOST_CLASS}>
                            <Show
                              when={hostLabel}
                              fallback={
                                <span class={PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS}>—</span>
                              }
                            >
                              <span class={PHYSICAL_DISK_VALUE_TEXT_CLASS} title={hostLabel}>
                                {hostLabel}
                              </span>
                            </Show>
                          </TableCell>

                          <TableCell class={PHYSICAL_DISK_CELL_ROLE_CLASS}>
                            <Show
                              when={getPhysicalDiskRoleLabel(data)}
                              fallback={
                                <span class={PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS}>—</span>
                              }
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
                              fallback={
                                <span class={PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS}>—</span>
                              }
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
                                <span
                                  class={PHYSICAL_DISK_HEALTH_SUMMARY_CLASS}
                                  title={healthSummary}
                                >
                                  {healthSummary}
                                </span>
                              </Show>
                            </div>
                          </TableCell>

                          <TableCell class={PHYSICAL_DISK_CELL_TEMP_CLASS}>
                            <span
                              class={`${PHYSICAL_DISK_TEMPERATURE_CLASS} ${getTemperatureTextClass(data.temperature)}`}
                            >
                              {data.temperature > 0 ? formatTemperature(data.temperature) : '—'}
                            </span>
                          </TableCell>

                          <TableCell class={PHYSICAL_DISK_CELL_SIZE_CLASS}>
                            <span class={PHYSICAL_DISK_SIZE_VALUE_CLASS}>
                              {formatBytes(data.size)}
                            </span>
                          </TableCell>

                        </TableRow>
                        <Show when={isSelected()}>
                          <TableRow data-inline-detail-for={summarySeriesId}>
                            <TableCell
                              id={detailControlsId}
                              colSpan={8}
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
          </div>
        </Card>
      </Show>
    </div>
  );
};
