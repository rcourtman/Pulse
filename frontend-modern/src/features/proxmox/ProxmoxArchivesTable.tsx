import { For, Show, type Accessor, type JSX } from 'solid-js';

import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import {
  Table,
  TableBody,
  TableCell,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { TableCard } from '@/components/shared/TableCard';
import { formatBytes, formatRelativeTime } from '@/utils/format';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
} from '@/features/platformPage/sharedPlatformPage';
import type { StorageBackup } from '@/types/api';

import { classifyArchiveRowAge } from './proxmoxBackupSummaryPresentation';
import { guestLabel, type ArchiveSortKey } from './proxmoxBackupsTableModel';
import { RowMetricBar, SortableHead } from './proxmoxBackupsTableShared';

// "Source details > Archives" table: vzdump / storage backup files with
// volume, guest, storage, age swatch, size, and (when any archive carries PBS
// metadata) protection + verification columns. Presentational — the parent
// owns the filtered+sorted memo, shared filters, and the size/now scales.
// table-fixed + a weighted colgroup keeps the columns from ballooning on wide
// viewports, accounting for the conditional PBS columns.
export function ProxmoxArchivesTable(props: {
  archives: StorageBackup[];
  hasAnyArchives: boolean;
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  sortKey: Accessor<ArchiveSortKey>;
  sortDirection: Accessor<'asc' | 'desc'>;
  onSort: (key: ArchiveSortKey) => void;
  showPBSColumns: boolean;
  sizeMaxBytes: number;
  nowMs: number;
}) {
  const visibleColumns = () => [
    { id: 'volume', weight: 18 },
    { id: 'guest', weight: 9 },
    { id: 'storage', weight: 11 },
    { id: 'node', weight: 10 },
    { id: 'format', weight: 8 },
    { id: 'created', weight: 12 },
    { id: 'size', weight: 14 },
    ...(props.showPBSColumns
      ? [
          { id: 'protected', weight: 9 },
          { id: 'verified', weight: 9 },
        ]
      : []),
  ];
  const totalColumnWeight = () =>
    visibleColumns().reduce((sum, column) => sum + column.weight, 0);

  return (
    <Show
      when={props.archives.length > 0}
      fallback={
        <Card padding="lg">
          <EmptyState
            icon={props.emptyIcon}
            title={!props.hasAnyArchives ? props.emptyTitle : 'No archives match current filters'}
            description={
              !props.hasAnyArchives
                ? props.emptyDescription
                : 'Adjust the search or status filter to see more archives.'
            }
          />
        </Card>
      }
    >
      <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
        <Table class="min-w-[1050px] table-fixed text-xs">
          <colgroup>
            <For each={visibleColumns()}>
              {(column) => (
                <col style={{ width: `${(column.weight / totalColumnWeight()) * 100}%` }} />
              )}
            </For>
          </colgroup>
          <TableHeader>
            <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
              <SortableHead
                label="Volume"
                sortKey="volume"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('name')}
              />
              <SortableHead
                label="Guest"
                sortKey="guest"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('text')}
              />
              <SortableHead
                label="Storage"
                sortKey="storage"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('text')}
              />
              <SortableHead
                label="Node"
                sortKey="node"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('text')}
              />
              <SortableHead
                label="Format"
                sortKey="format"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('text')}
              />
              <SortableHead
                label="Created"
                sortKey="created"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="right"
                headClass={getPlatformTableHeadClassForKind('numeric-value')}
              />
              <SortableHead
                label="Size"
                sortKey="size"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="center"
                headClass={getPlatformTableHeadClassForKind('metric-bar')}
              />
              <Show when={props.showPBSColumns}>
                <SortableHead
                  label="Protection"
                  sortKey="protected"
                  currentSort={props.sortKey}
                  direction={props.sortDirection}
                  onSort={props.onSort}
                  align="left"
                  headClass={getPlatformTableHeadClassForKind('text')}
                />
                <SortableHead
                  label="Verified"
                  sortKey="verified"
                  currentSort={props.sortKey}
                  direction={props.sortDirection}
                  onSort={props.onSort}
                  align="left"
                  headClass={getPlatformTableHeadClassForKind('text')}
                />
              </Show>
            </TableRow>
          </TableHeader>
          <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
            <For each={props.archives}>
              {(arc) => (
                <TableRow class="hover:bg-surface-hover">
                  <TableCell
                    class={`${getPlatformTableCellClassForKind('name')} text-base-content font-mono text-[11px]`}
                  >
                    <span class="inline-block max-w-[18rem] truncate" title={arc.volid}>
                      {arc.volid}
                    </span>
                  </TableCell>
                  <TableCell
                    class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                  >
                    {guestLabel(arc.type, arc.vmid)}
                  </TableCell>
                  <TableCell
                    class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                  >
                    {arc.storage || '—'}
                  </TableCell>
                  <TableCell
                    class={`${getPlatformTableCellClassForKind('text')} text-base-content font-mono text-[11px]`}
                  >
                    {arc.node || '—'}
                  </TableCell>
                  <TableCell
                    class={`${getPlatformTableCellClassForKind('text')} text-base-content uppercase text-[10px]`}
                  >
                    {arc.format || '—'}
                  </TableCell>
                  <TableCell
                    class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                  >
                    <div class="flex items-center justify-end gap-2">
                      {(() => {
                        const age = classifyArchiveRowAge(arc.time, props.nowMs);
                        return (
                          <span
                            class={`h-1.5 w-1.5 shrink-0 rounded-full ${age.swatchClass}`}
                            aria-hidden="true"
                            title={`Coverage: ${age.label}`}
                          />
                        );
                      })()}
                      <span>{formatRelativeTime(arc.time, { compact: true })}</span>
                    </div>
                  </TableCell>
                  <TableCell
                    class={`${getPlatformTableCellClassForKind('metric-bar')} text-base-content`}
                  >
                    <RowMetricBar
                      valuePct={props.sizeMaxBytes > 0 ? (arc.size / props.sizeMaxBytes) * 100 : 0}
                      fillClass="bg-blue-500/40 dark:bg-blue-500/40"
                      label={formatBytes(arc.size)}
                      tooltip={`${formatBytes(arc.size)} (relative to largest file in view)`}
                    />
                  </TableCell>
                  <Show when={props.showPBSColumns}>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                    >
                      <Show
                        when={arc.protected}
                        fallback={<span class="text-muted">Unprotected</span>}
                      >
                        <span class="inline-flex items-center rounded-sm bg-amber-100 px-1.5 py-0.5 text-[10px] font-semibold text-amber-700 dark:bg-amber-900/40 dark:text-amber-200">
                          Protected
                        </span>
                      </Show>
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                    >
                      <Show
                        when={arc.verified}
                        fallback={
                          <Show when={arc.isPBS} fallback={<span class="text-muted">n/a</span>}>
                            <span class="text-muted">Pending</span>
                          </Show>
                        }
                      >
                        <span class="inline-flex items-center rounded-sm bg-emerald-100 px-1.5 py-0.5 text-[10px] font-semibold text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200">
                          Verified
                        </span>
                      </Show>
                    </TableCell>
                  </Show>
                </TableRow>
              )}
            </For>
          </TableBody>
        </Table>
      </TableCard>
    </Show>
  );
}
