import { For, Show, createMemo, type Accessor, type JSX } from 'solid-js';

import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import {
  getRecoveryFullDateLabel,
  recoveryDateKeyFromTimestamp,
} from '@/utils/recoveryDatePresentation';
import {
  formatPlatformTableBytesValue,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  PlatformTableEmptyState,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import { useObservedElementWidth } from '@/hooks/useObservedElementWidth';

import type { RecoverableArtifact } from './proxmoxBackupRecoveryModel';
import type { RecoverableSortKey } from './proxmoxBackupsTableModel';
import {
  ArtifactSourceBadge,
  ArtifactStateBadge,
  PROXMOX_BACKUP_COLUMN_LABELS,
  ProxmoxBackupAgeText,
  ProxmoxBackupWorkloadTypeBadge,
  RowMetricBar,
  SortableHead,
  artifactStateLabel,
} from './proxmoxBackupsTableShared';
import {
  getRecoverableColumns,
  getRecoverableColumnWidthStyle,
  getRecoverableLayoutForContainer,
  isCompactBackupIdentityLayout,
  type RecoverableColumnId,
} from './proxmoxBackupsTablePresentation';
import { useProxmoxBackupTableWindowing } from './useProxmoxBackupTableWindowing';

// Flat recoverable-artifact table. Parent state owns filtering, sorting, and
// date/source facets; optional day grouping is presentation only.

interface DayGroup {
  key: string;
  label: string;
  items: RecoverableArtifact[];
}

type RecoverableTableItem =
  | { kind: 'day'; key: string; label: string; count: number }
  | { kind: 'artifact'; key: string; artifact: RecoverableArtifact };

function groupByDay(artifacts: readonly RecoverableArtifact[]): DayGroup[] {
  const groups: DayGroup[] = [];
  let current: DayGroup | undefined;
  for (const artifact of artifacts) {
    const key =
      artifact.createdMs === undefined
        ? 'unknown'
        : recoveryDateKeyFromTimestamp(artifact.createdMs);
    if (!current || current.key !== key) {
      current = {
        key,
        label: key === 'unknown' ? 'Unknown date' : getRecoveryFullDateLabel(key),
        items: [],
      };
      groups.push(current);
    }
    current.items.push(artifact);
  }
  return groups;
}

export function ProxmoxRecoverableTable(props: {
  artifacts: RecoverableArtifact[];
  hasAnyArtifacts: boolean;
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  sortKey: Accessor<RecoverableSortKey>;
  sortDirection: Accessor<'asc' | 'desc'>;
  onSort: (key: RecoverableSortKey) => void;
  sizeMaxBytes: number;
  groupByDay?: boolean;
  layoutWidth?: Accessor<number | null | undefined>;
}) {
  const observedWidth = useObservedElementWidth();
  const layoutMode = createMemo(() => {
    const width = props.layoutWidth?.() ?? observedWidth.width();
    return typeof width === 'number' && width > 0
      ? getRecoverableLayoutForContainer(width)
      : 'full';
  });
  const visibleColumns = createMemo(() => getRecoverableColumns(layoutMode()));
  const columnVisible = (column: RecoverableColumnId) =>
    visibleColumns().some((candidate) => candidate.id === column);
  const showDayGroups = () => props.groupByDay && props.sortKey() === 'created';
  const tableItems = createMemo<RecoverableTableItem[]>(() => {
    if (!showDayGroups()) {
      return props.artifacts.map((item) => ({
        kind: 'artifact',
        key: item.id,
        artifact: item,
      }));
    }

    return groupByDay(props.artifacts).flatMap((group) => [
      { kind: 'day' as const, key: group.key, label: group.label, count: group.items.length },
      ...group.items.map((item) => ({
        kind: 'artifact' as const,
        key: item.id,
        artifact: item,
      })),
    ]);
  });
  const tableWindow = useProxmoxBackupTableWindowing(tableItems);

  const renderRow = (artifact: RecoverableArtifact): JSX.Element => (
    <TableRow class="hover:bg-surface-hover" data-proxmox-backup-row="recoverable">
      <TableCell class={`${getPlatformTableCellClassForKind('name')} text-base-content`}>
        <div class="min-w-0">
          <div class="truncate font-semibold">
            {artifact.workload.name || artifact.workload.label}
          </div>
          <Show when={isCompactBackupIdentityLayout(layoutMode())}>
            <div class="truncate text-[10px] text-muted">
              {artifact.workload.typeLabel}
              <Show when={artifact.workload.vmid}> {artifact.workload.vmid}</Show>
            </div>
          </Show>
        </div>
      </TableCell>
      <Show when={columnVisible('type')}>
        <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
          <ProxmoxBackupWorkloadTypeBadge
            type={artifact.workload.type}
            label={artifact.workload.typeLabel}
          />
        </TableCell>
      </Show>
      <Show when={columnVisible('targetId')}>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} text-muted font-mono text-[11px] tabular-nums`}
        >
          {artifact.workload.vmid || '—'}
        </TableCell>
      </Show>
      <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
        <ArtifactSourceBadge artifact={artifact} />
      </TableCell>
      <Show when={columnVisible('location')}>
        <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
          <span class="inline-block max-w-[16rem] truncate" title={artifact.location}>
            {artifact.location}
          </span>
        </TableCell>
      </Show>
      <TableCell class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}>
        <ProxmoxBackupAgeText artifact={artifact} />
      </TableCell>
      <Show when={columnVisible('size')}>
        <TableCell class={`${getPlatformTableCellClassForKind('metric-bar')} text-base-content`}>
          <Show
            when={artifact.size && artifact.size > 0}
            fallback={<span class="text-muted">No size</span>}
          >
            <RowMetricBar
              valuePct={
                props.sizeMaxBytes > 0 && artifact.size
                  ? (artifact.size / props.sizeMaxBytes) * 100
                  : 0
              }
              fillClass="bg-blue-500/40 dark:bg-blue-500/40"
              label={formatPlatformTableBytesValue(artifact.size)}
              tooltip={`${formatPlatformTableBytesValue(artifact.size)} (relative to largest artifact in view)`}
            />
          </Show>
        </TableCell>
      </Show>
      <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
        <ArtifactStateBadge artifact={artifact} label={artifactStateLabel(artifact)} />
      </TableCell>
      <Show when={columnVisible('details')}>
        <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
          <span class="inline-block max-w-[20rem] truncate" title={artifact.detail}>
            {artifact.detail || '—'}
          </span>
        </TableCell>
      </Show>
    </TableRow>
  );

  return (
    <Show
      when={props.artifacts.length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={
            !props.hasAnyArtifacts
              ? props.emptyTitle
              : 'No recoverable artifacts match current filters'
          }
          description={
            !props.hasAnyArtifacts
              ? props.emptyDescription
              : 'Adjust the search, source filter, or selected day to see more artifacts.'
          }
        />
      }
    >
      <div
        ref={(element) => {
          observedWidth.setElement(element);
          tableWindow.setRootRef(element);
        }}
        data-proxmox-backups-table="recoverable"
        data-proxmox-backups-layout={layoutMode()}
        data-proxmox-backups-windowed={tableWindow.isWindowed()}
      >
        <PlatformTableShell
          tableClass="min-w-[0px] table-fixed text-xs"
          colgroup={
            <colgroup>
              <For each={visibleColumns()}>
                {(column) => (
                  <col
                    style={getRecoverableColumnWidthStyle(column.id, layoutMode())}
                    data-proxmox-backups-column={column.id}
                  />
                )}
              </For>
            </colgroup>
          }
          header={
            <>
              <SortableHead
                label="Workload"
                sortKey="workload"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={`${getPlatformTableHeadClassForKind('name')} platform-table-mobile-w-30`}
              />
              <Show when={columnVisible('type')}>
                <TableHead class={getPlatformTableHeadClassForKind('text')}>Type</TableHead>
              </Show>
              <Show when={columnVisible('targetId')}>
                <TableHead class={getPlatformTableHeadClassForKind('text')}>
                  {PROXMOX_BACKUP_COLUMN_LABELS.targetId}
                </TableHead>
              </Show>
              <SortableHead
                label={layoutMode() === 'compact' ? 'Via' : 'Source'}
                sortKey="source"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('text')}
              />
              <Show when={columnVisible('location')}>
                <SortableHead
                  label={layoutMode() === 'compact' ? 'Loc' : 'Location'}
                  sortKey="location"
                  currentSort={props.sortKey}
                  direction={props.sortDirection}
                  onSort={props.onSort}
                  align="left"
                  headClass={getPlatformTableHeadClassForKind('text')}
                />
              </Show>
              <SortableHead
                label={layoutMode() === 'compact' ? 'Age' : PROXMOX_BACKUP_COLUMN_LABELS.created}
                sortKey="created"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="right"
                headClass={getPlatformTableHeadClassForKind('numeric-value')}
              />
              <Show when={columnVisible('size')}>
                <SortableHead
                  label="Size"
                  sortKey="size"
                  currentSort={props.sortKey}
                  direction={props.sortDirection}
                  onSort={props.onSort}
                  align="center"
                  headClass={getPlatformTableHeadClassForKind('metric-bar')}
                />
              </Show>
              <SortableHead
                label="State"
                sortKey="state"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('text')}
              />
              <Show when={columnVisible('details')}>
                <TableHead class={getPlatformTableHeadClassForKind('text')}>
                  {PROXMOX_BACKUP_COLUMN_LABELS.details}
                </TableHead>
              </Show>
            </>
          }
          body={
            <>
              <Show when={tableWindow.topSpacerHeight() > 0}>
                <TableRow aria-hidden="true">
                  <TableCell
                    colspan={visibleColumns().length}
                    class="border-0 p-0"
                    height={tableWindow.topSpacerHeight()}
                  />
                </TableRow>
              </Show>
              <For each={tableWindow.visibleItems()}>
                {(item) => (
                  <Show
                    when={item.kind === 'artifact' ? item.artifact : undefined}
                    fallback={
                      <TableRow data-proxmox-backup-row="day">
                        {/* Cell-level background is reliable across table layout engines. */}
                        <TableCell
                          colspan={visibleColumns().length}
                          class="border-t border-border bg-surface-alt px-3 py-1.5 text-[11px] font-semibold uppercase tracking-[0.14em] text-base-content"
                        >
                          {item.kind === 'day' ? item.label : ''}{' '}
                          <span class="ml-2 normal-case tracking-normal text-muted">
                            {item.kind === 'day' ? item.count : 0}{' '}
                            {item.kind === 'day' && item.count === 1 ? 'backup' : 'backups'}
                          </span>
                        </TableCell>
                      </TableRow>
                    }
                  >
                    {(artifact) => renderRow(artifact())}
                  </Show>
                )}
              </For>
              <Show when={tableWindow.bottomSpacerHeight() > 0}>
                <TableRow aria-hidden="true">
                  <TableCell
                    colspan={visibleColumns().length}
                    class="border-0 p-0"
                    height={tableWindow.bottomSpacerHeight()}
                  />
                </TableRow>
              </Show>
            </>
          }
        />
      </div>
    </Show>
  );
}
