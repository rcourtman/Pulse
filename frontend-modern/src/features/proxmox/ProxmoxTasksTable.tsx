import { For, Show, type Accessor, type JSX } from 'solid-js';

import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { StatusDot } from '@/components/shared/StatusDot';
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
import type { BackupTask } from '@/types/api';

import { taskDurationSeconds } from './proxmoxBackupSummaryPresentation';
import {
  classifyTaskStatus,
  formatDuration,
  formatDurationFromSeconds,
  guestLabel,
  type TaskSortKey,
} from './proxmoxBackupsTableModel';
import { RowMetricBar, SortableHead } from './proxmoxBackupsTableShared';

// "Job history" table: one row per backup task with status, guest, node, start
// time, a median-anchored duration bar, and optional size / error columns.
// Presentational only — the parent owns the filtered + sorted memo, the shared
// search / status filters, and the duration baseline.
export function ProxmoxTasksTable(props: {
  tasks: BackupTask[];
  hasAnyTasks: boolean;
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  sortKey: Accessor<TaskSortKey>;
  sortDirection: Accessor<'asc' | 'desc'>;
  onSort: (key: TaskSortKey) => void;
  showSizeColumn: boolean;
  showErrorColumn: boolean;
  durationBaselineSeconds: number;
}) {
  // table-fixed + weighted colgroup so columns share the row width instead of
  // dumping all the slack into the last column (the Error column ballooned to
  // ~440px of blank when no rows had errors). Weights are relative; each
  // visible column resolves to weight / total, matching the platform tables.
  const visibleColumns = () => [
    { id: 'status', weight: 11 },
    { id: 'guest', weight: 13 },
    { id: 'node', weight: 12 },
    { id: 'started', weight: 13 },
    { id: 'duration', weight: 21 },
    ...(props.showSizeColumn ? [{ id: 'size', weight: 12 }] : []),
    ...(props.showErrorColumn ? [{ id: 'error', weight: 18 }] : []),
  ];
  const totalColumnWeight = () =>
    visibleColumns().reduce((sum, column) => sum + column.weight, 0);

  return (
    <Show
      when={props.tasks.length > 0}
      fallback={
        <Card padding="lg">
          <EmptyState
            icon={props.emptyIcon}
            title={!props.hasAnyTasks ? props.emptyTitle : 'No tasks match current filters'}
            description={
              !props.hasAnyTasks
                ? props.emptyDescription
                : 'Adjust the search or status filter to see more tasks.'
            }
          />
        </Card>
      }
    >
      <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
        <Table class="min-w-[1000px] table-fixed text-xs">
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
                label="Status"
                sortKey="status"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('text')}
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
                label="Node"
                sortKey="node"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('text')}
              />
              <SortableHead
                label="Started"
                sortKey="started"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="right"
                headClass={getPlatformTableHeadClassForKind('numeric-value')}
              />
              <SortableHead
                label="Duration"
                sortKey="duration"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="center"
                headClass={getPlatformTableHeadClassForKind('metric-bar')}
              />
              <Show when={props.showSizeColumn}>
                <SortableHead
                  label="Size"
                  sortKey="size"
                  currentSort={props.sortKey}
                  direction={props.sortDirection}
                  onSort={props.onSort}
                  align="right"
                  headClass={getPlatformTableHeadClassForKind('numeric-value')}
                />
              </Show>
              <Show when={props.showErrorColumn}>
                <TableHead class={getPlatformTableHeadClassForKind('text')}>Error</TableHead>
              </Show>
            </TableRow>
          </TableHeader>
          <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
            <For each={props.tasks}>
              {(task) => {
                const classify = classifyTaskStatus(task.status);
                const durationSec = taskDurationSeconds(task);
                // Anchor the bar so the median sits at ~50%.
                const durationBarPct = () => {
                  const baseline = props.durationBaselineSeconds;
                  if (!durationSec || baseline <= 0) return 0;
                  return (durationSec / (baseline * 2)) * 100;
                };
                // Canonical Pulse metric tones (60% alpha) — same palette
                // Storage and Ceph row bars use via getMetricColorClass.
                // Cap at `warning` (soft yellow) rather than going to red
                // for the worst case: a slow backup task is a perf
                // outlier, not a failure. Failure is already conveyed by
                // the Status column. Two tiers — normal / slow — keeps
                // the column calm instead of screaming red on every
                // long-running VM backup.
                const durationToneClass = () => {
                  const baseline = props.durationBaselineSeconds;
                  if (!durationSec || baseline <= 0)
                    return 'bg-slate-500/30 dark:bg-slate-500/30';
                  const ratio = durationSec / baseline;
                  if (ratio >= 1.5) return 'bg-metric-warning-bg dark:bg-metric-warning-bg';
                  return 'bg-metric-normal-bg dark:bg-metric-normal-bg';
                };
                return (
                  <TableRow class="hover:bg-surface-hover">
                    <TableCell class={getPlatformTableCellClassForKind('text')}>
                      <div class="flex items-center gap-2">
                        <StatusDot
                          size="sm"
                          variant={classify.variant}
                          title={classify.label}
                          ariaHidden
                        />
                        <span class={`text-[11px] font-medium ${classify.toneClass}`}>
                          {classify.label}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                    >
                      {guestLabel(task.type, task.vmid)}
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('text')} text-base-content font-mono text-[11px]`}
                    >
                      {task.node || '—'}
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                    >
                      {formatRelativeTime(task.startTime, { compact: true })}
                    </TableCell>
                    <TableCell
                      class={`${getPlatformTableCellClassForKind('metric-bar')} text-base-content`}
                    >
                      <RowMetricBar
                        valuePct={
                          props.durationBaselineSeconds > 0 && durationSec ? durationBarPct() : 0
                        }
                        fillClass={durationToneClass()}
                        label={formatDuration(task.startTime, task.endTime)}
                        tooltip={`Duration ${formatDuration(task.startTime, task.endTime)} (median ${formatDurationFromSeconds(props.durationBaselineSeconds)})`}
                      />
                    </TableCell>
                    <Show when={props.showSizeColumn}>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                      >
                        <Show
                          when={task.size && task.size > 0}
                          fallback={<span class="text-muted">—</span>}
                        >
                          {formatBytes(task.size ?? 0)}
                        </Show>
                      </TableCell>
                    </Show>
                    <Show when={props.showErrorColumn}>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                      >
                        <Show
                          when={!!task.error?.trim()}
                          fallback={<span class="text-muted">—</span>}
                        >
                          <span
                            class="inline-block max-w-[18rem] truncate text-red-600 dark:text-red-300"
                            title={task.error}
                          >
                            {task.error}
                          </span>
                        </Show>
                      </TableCell>
                    </Show>
                  </TableRow>
                );
              }}
            </For>
          </TableBody>
        </Table>
      </TableCard>
    </Show>
  );
}
