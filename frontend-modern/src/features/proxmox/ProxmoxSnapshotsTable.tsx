import { For, Show, type Accessor, type JSX } from 'solid-js';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';

import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
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

import { classifySnapshotRowAge } from './proxmoxBackupSummaryPresentation';
import {
  guestLabel,
  type SnapshotGuestRow,
  type SnapshotSortKey,
} from './proxmoxBackupsTableModel';
import { SortableHead } from './proxmoxBackupsTableShared';

// "Source details > Snapshots" table: one row per guest with its newest-snapshot
// age, count, total size, and RAM marker; each row expands to the guest's
// individual snapshots. Presentational — the parent owns the filtered+sorted
// memo, shared filters, and the expansion set. table-fixed + a weighted
// colgroup keeps the outer columns from ballooning (the inner per-snapshot
// table stays content-sized).
export function ProxmoxSnapshotsTable(props: {
  guests: SnapshotGuestRow[];
  hasAnySnapshots: boolean;
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  sortKey: Accessor<SnapshotSortKey>;
  sortDirection: Accessor<'asc' | 'desc'>;
  onSort: (key: SnapshotSortKey) => void;
  showSizeColumn: boolean;
  showRAMColumn: boolean;
  columnCount: number;
  expandedKeys: ReadonlySet<string>;
  onToggleExpand: (key: string) => void;
  nowMs: number;
}) {
  const visibleColumns = () => [
    { id: 'guest', weight: 24 },
    { id: 'node', weight: 16 },
    { id: 'latest', weight: 18 },
    { id: 'count', weight: 12 },
    ...(props.showSizeColumn ? [{ id: 'size', weight: 16 }] : []),
    ...(props.showRAMColumn ? [{ id: 'ram', weight: 14 }] : []),
  ];
  const totalColumnWeight = () =>
    visibleColumns().reduce((sum, column) => sum + column.weight, 0);

  return (
    <Show
      when={props.guests.length > 0}
      fallback={
        <Card padding="lg">
          <EmptyState
            icon={props.emptyIcon}
            title={
              !props.hasAnySnapshots ? props.emptyTitle : 'No snapshots match current filters'
            }
            description={
              !props.hasAnySnapshots
                ? props.emptyDescription
                : 'Adjust the search or filters to see more snapshots.'
            }
          />
        </Card>
      }
    >
      <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
        <Table class="min-w-[900px] table-fixed text-xs">
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
                label="Guest"
                sortKey="guest"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('name')}
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
                label="Latest"
                sortKey="latest"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="right"
                headClass={getPlatformTableHeadClassForKind('numeric-value')}
              />
              <SortableHead
                label="Snapshots"
                sortKey="count"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="right"
                headClass={getPlatformTableHeadClassForKind('numeric-value')}
              />
              <Show when={props.showSizeColumn}>
                <SortableHead
                  label="Total size"
                  sortKey="size"
                  currentSort={props.sortKey}
                  direction={props.sortDirection}
                  onSort={props.onSort}
                  align="right"
                  headClass={getPlatformTableHeadClassForKind('numeric-value')}
                />
              </Show>
              <Show when={props.showRAMColumn}>
                <TableHead class={getPlatformTableHeadClassForKind('text')}>RAM</TableHead>
              </Show>
            </TableRow>
          </TableHeader>
          <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
            <For each={props.guests}>
              {(row) => {
                const isExpanded = () => props.expandedKeys.has(row.key);
                const rowAge = classifySnapshotRowAge(row.newestMs, props.nowMs);
                return (
                  <>
                    <TableRow
                      class="cursor-pointer hover:bg-surface-hover"
                      onClick={() => props.onToggleExpand(row.key)}
                    >
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('name')} text-base-content`}
                      >
                        <div class="flex items-center gap-2">
                          <ChevronRightIcon
                            class={`h-3.5 w-3.5 shrink-0 text-muted transition-transform ${
                              isExpanded() ? 'rotate-90' : ''
                            }`}
                            aria-hidden="true"
                          />
                          <span class="font-semibold">{guestLabel(row.type, row.vmid)}</span>
                        </div>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content font-mono text-[11px]`}
                      >
                        {row.node || '—'}
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                      >
                        <Show
                          when={row.newestMs !== undefined}
                          fallback={<span class="text-muted">—</span>}
                        >
                          <div class="flex items-center justify-end gap-2">
                            <span
                              class={`h-1.5 w-1.5 shrink-0 rounded-full ${rowAge.swatchClass}`}
                              aria-hidden="true"
                              title={`Newest snapshot: ${rowAge.label}`}
                            />
                            <span>{formatRelativeTime(row.newestMs, { compact: true })}</span>
                          </div>
                        </Show>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                      >
                        {row.count}
                      </TableCell>
                      <Show when={props.showSizeColumn}>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                        >
                          <Show
                            when={row.totalBytes > 0}
                            fallback={<span class="text-muted">—</span>}
                          >
                            {formatBytes(row.totalBytes)}
                          </Show>
                        </TableCell>
                      </Show>
                      <Show when={props.showRAMColumn}>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                        >
                          <Show
                            when={row.withRamCount > 0}
                            fallback={<span class="text-muted">—</span>}
                          >
                            <span class="inline-flex items-center rounded-sm bg-violet-100 px-1.5 py-0.5 text-[10px] font-semibold text-violet-700 dark:bg-violet-900/40 dark:text-violet-200">
                              {row.withRamCount} with RAM
                            </span>
                          </Show>
                        </TableCell>
                      </Show>
                    </TableRow>
                    <Show when={isExpanded()}>
                      <TableRow class="bg-surface-alt/40">
                        <TableCell class="px-2 py-2" colspan={props.columnCount}>
                          <div class="overflow-hidden">
                            <table class="w-full text-[11px]">
                              <thead>
                                <tr class="bg-surface-alt text-muted">
                                  <th class="px-2 py-0.5 text-left font-medium">Name</th>
                                  <th class="px-2 py-0.5 text-left font-medium">Parent</th>
                                  <th class="px-2 py-0.5 text-right font-medium">Captured</th>
                                  <Show when={props.showSizeColumn}>
                                    <th class="px-2 py-0.5 text-right font-medium">Size</th>
                                  </Show>
                                  <Show when={props.showRAMColumn}>
                                    <th class="px-2 py-0.5 text-left font-medium">RAM</th>
                                  </Show>
                                </tr>
                              </thead>
                              <tbody class="divide-y divide-border-subtle">
                                <For each={row.snapshots}>
                                  {(snap) => {
                                    const age = classifySnapshotRowAge(snap.time, props.nowMs);
                                    return (
                                      <tr class="hover:bg-surface-hover">
                                        <td class="px-2 py-1">
                                          <div class="flex items-center gap-2">
                                            <span
                                              class={`h-1.5 w-1.5 shrink-0 rounded-full ${age.swatchClass}`}
                                              aria-hidden="true"
                                              title={`Age: ${age.label}`}
                                            />
                                            <div class="min-w-0">
                                              <div class="font-medium text-base-content">
                                                {snap.name || '—'}
                                              </div>
                                              <Show when={!!snap.description?.trim()}>
                                                <div
                                                  class="truncate max-w-[24rem] text-[10px] text-muted"
                                                  title={snap.description}
                                                >
                                                  {snap.description}
                                                </div>
                                              </Show>
                                            </div>
                                          </div>
                                        </td>
                                        <td class="px-2 py-1 font-mono text-[10px] text-muted">
                                          {snap.parent?.trim() || '—'}
                                        </td>
                                        <td class="px-2 py-1 text-right text-base-content">
                                          {formatRelativeTime(snap.time, { compact: true })}
                                        </td>
                                        <Show when={props.showSizeColumn}>
                                          <td class="px-2 py-1 text-right tabular-nums text-base-content">
                                            <Show
                                              when={snap.sizeBytes && snap.sizeBytes > 0}
                                              fallback={<span class="text-muted">—</span>}
                                            >
                                              {formatBytes(snap.sizeBytes ?? 0)}
                                            </Show>
                                          </td>
                                        </Show>
                                        <Show when={props.showRAMColumn}>
                                          <td class="px-2 py-1">
                                            <Show
                                              when={snap.vmstate}
                                              fallback={<span class="text-muted">—</span>}
                                            >
                                              <span class="inline-flex items-center rounded-sm bg-violet-100 px-1.5 py-0.5 text-[10px] font-semibold text-violet-700 dark:bg-violet-900/40 dark:text-violet-200">
                                                with RAM
                                              </span>
                                            </Show>
                                          </td>
                                        </Show>
                                      </tr>
                                    );
                                  }}
                                </For>
                              </tbody>
                            </table>
                          </div>
                        </TableCell>
                      </TableRow>
                    </Show>
                  </>
                );
              }}
            </For>
          </TableBody>
        </Table>
      </TableCard>
    </Show>
  );
}
