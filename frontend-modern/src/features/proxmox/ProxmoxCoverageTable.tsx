import { For, Show, type Accessor, type JSX } from 'solid-js';

import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableRow } from '@/components/shared/Table';
import { formatBytes } from '@/utils/format';
import {
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  PlatformTableEmptyState,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import { PlatformResourceDetailToggleButton } from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { StatusIndicatorVariant } from '@/utils/status';

import {
  getWorkloadRecoveryPostureLabel,
  type WorkloadCoverageRow,
} from './proxmoxBackupRecoveryModel';
import type { CoverageSortKey } from './proxmoxBackupsTableModel';
import { getProxmoxBackupSourcePresentation } from './proxmoxBackupSourcePresentation';
import {
  ArtifactSourceBadge,
  ArtifactStateBadge,
  PROXMOX_BACKUP_COLUMN_LABELS,
  ProxmoxBackupAgeText,
  ProxmoxBackupWorkloadTypeBadge,
  SortableHead,
  artifactStateLabel,
} from './proxmoxBackupsTableShared';

const coveragePostureVariant = (
  posture: WorkloadCoverageRow['posture'],
): StatusIndicatorVariant => {
  if (posture === 'current') return 'success';
  if (posture === 'uncovered' || posture === 'failed' || posture === 'stale') return 'danger';
  return 'warning';
};

// Colour marks the exception, not the baseline: healthy rows keep neutral text
// (the green dot already signals "fine"), while attention/danger words take an
// amber/red tone so a mixed fleet is scannable at a glance. Matches the
// datastore-usage tones on the Backup servers table and the status-word colour
// pattern in the Replication table.
const statusWordToneClass = (variant: StatusIndicatorVariant): string => {
  if (variant === 'danger') return 'text-red-600 dark:text-red-300';
  if (variant === 'warning') return 'text-amber-600 dark:text-amber-300';
  return 'text-base-content';
};

const taskWordVariant = (label: string): StatusIndicatorVariant => {
  if (label === 'Failed') return 'danger';
  if (label === 'OK') return 'success';
  return 'warning';
};

// "Workload coverage" table: one row per workload answering "does this have a
// backup?" across PBS snapshots / PVE backup files / guest snapshots, each
// expanding to its restore evidence. Presentational — the parent owns the
// filtered+sorted memo, shared filters, and the expansion set. table-fixed + a
// colgroup keeps the columns from ballooning; the inner evidence table stays
// content-sized.
export function ProxmoxCoverageTable(props: {
  rows: WorkloadCoverageRow[];
  hasAnyRows: boolean;
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  sortKey: Accessor<CoverageSortKey>;
  sortDirection: Accessor<'asc' | 'desc'>;
  onSort: (key: CoverageSortKey) => void;
  expandedKeys: ReadonlySet<string>;
  onToggleExpand: (key: string) => void;
  // Source columns auto-hide when no workload anywhere has that data (e.g. a
  // PBS-only fleet drops the PVE files and Snapshots columns), matching how the
  // source-detail tables already drop their conditional columns.
  showPbsColumn: boolean;
  showArchiveColumn: boolean;
  showSnapshotColumn: boolean;
  showTaskColumn: boolean;
}) {
  // Weighted column set; the conditional source/task columns drop out when
  // empty fleet-wide, and table-fixed re-normalizes the rest.
  const visibleColumns = () => [
    { id: 'workload', weight: 25 },
    { id: 'posture', weight: 12 },
    { id: 'latest', weight: 12 },
    ...(props.showPbsColumn ? [{ id: 'pbs', weight: 14 }] : []),
    ...(props.showArchiveColumn ? [{ id: 'archive', weight: 12 }] : []),
    ...(props.showSnapshotColumn ? [{ id: 'snapshot', weight: 15 }] : []),
    ...(props.showTaskColumn ? [{ id: 'task', weight: 10 }] : []),
  ];
  const totalColumnWeight = () => visibleColumns().reduce((sum, c) => sum + c.weight, 0);
  const columnCount = () => visibleColumns().length;
  const pbsSource = getProxmoxBackupSourcePresentation('pbs');
  const archiveSource = getProxmoxBackupSourcePresentation('archive');
  const snapshotSource = getProxmoxBackupSourcePresentation('snapshot');

  return (
    <Show
      when={props.rows.length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={
            !props.hasAnyRows ? props.emptyTitle : 'No workload coverage rows match current filters'
          }
          description={
            !props.hasAnyRows
              ? props.emptyDescription
              : 'Adjust the search, posture filter, or selected day to see more workloads.'
          }
        />
      }
    >
      <PlatformTableShell
        tableClass="min-w-[1000px] table-fixed text-xs"
        colgroup={
          <colgroup>
            <For each={visibleColumns()}>
              {(column) => (
                <col style={{ width: `${(column.weight / totalColumnWeight()) * 100}%` }} />
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
              headClass={getPlatformTableHeadClassForKind('name')}
            />
            <SortableHead
              label="Posture"
              sortKey="posture"
              currentSort={props.sortKey}
              direction={props.sortDirection}
              onSort={props.onSort}
              align="left"
              headClass={getPlatformTableHeadClassForKind('text')}
            />
            <SortableHead
              label="Restore"
              sortKey="latest"
              currentSort={props.sortKey}
              direction={props.sortDirection}
              onSort={props.onSort}
              align="right"
              headClass={getPlatformTableHeadClassForKind('numeric-value')}
            />
            <Show when={props.showPbsColumn}>
              <SortableHead
                label="PBS snapshot"
                sortKey="pbs"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('text')}
              />
            </Show>
            <Show when={props.showArchiveColumn}>
              <SortableHead
                label="PVE file"
                sortKey="archive"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('text')}
              />
            </Show>
            <Show when={props.showSnapshotColumn}>
              <SortableHead
                label="Guest snapshot"
                sortKey="snapshot"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('text')}
              />
            </Show>
            <Show when={props.showTaskColumn}>
              <SortableHead
                label="Task"
                sortKey="task"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="left"
                headClass={getPlatformTableHeadClassForKind('text')}
              />
            </Show>
          </>
        }
        body={
          <>
            <For each={props.rows}>
              {(row) => {
                const isExpanded = () => props.expandedKeys.has(row.key);
                const evidence = () =>
                  [...row.artifacts]
                    .sort((left, right) => (right.createdMs ?? 0) - (left.createdMs ?? 0))
                    .slice(0, 8);
                const detailRowId = () => `proxmox-coverage-evidence-${row.key}`;
                return (
                  <>
                    <TableRow class="hover:bg-surface-hover">
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('name')} text-base-content`}
                      >
                        <div class="flex min-w-0 items-start gap-2">
                          <PlatformResourceDetailToggleButton
                            expanded={isExpanded()}
                            resourceLabel={row.workload.label}
                            controlsId={detailRowId()}
                            onToggle={() => props.onToggleExpand(row.key)}
                          />
                          <div class="min-w-0">
                            <span class="block truncate font-semibold">
                              {row.workload.name || row.workload.label}
                            </span>
                            <div class="mt-1 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-[10px] leading-4 text-muted">
                              <ProxmoxBackupWorkloadTypeBadge
                                type={row.workload.type}
                                label={row.workload.typeLabel}
                              />
                              <span class="font-mono tabular-nums">
                                ID {row.workload.vmid || '—'}
                              </span>
                              <Show when={row.workload.node}>
                                {(node) => (
                                  <span class="truncate font-mono" title={node()}>
                                    Node {node()}
                                  </span>
                                )}
                              </Show>
                            </div>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell class={getPlatformTableCellClassForKind('text')}>
                        <div class="flex items-center gap-2">
                          <StatusDot
                            size="sm"
                            variant={coveragePostureVariant(row.posture)}
                            title={getWorkloadRecoveryPostureLabel(row.posture)}
                            ariaHidden
                          />
                          <span
                            class={`truncate text-[11px] font-medium ${statusWordToneClass(
                              coveragePostureVariant(row.posture),
                            )}`}
                          >
                            {getWorkloadRecoveryPostureLabel(row.posture)}
                          </span>
                        </div>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                      >
                        <Show
                          when={row.latestRecovery}
                          fallback={<span class="text-muted">No restore point</span>}
                        >
                          {(artifact) => <ProxmoxBackupAgeText artifact={artifact()} />}
                        </Show>
                      </TableCell>
                      <Show when={props.showPbsColumn}>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                        >
                          <Show
                            when={row.latestPBS}
                            fallback={
                              <span class="text-muted">{pbsSource.coverageFallbackLabel}</span>
                            }
                          >
                            {(artifact) => <ProxmoxBackupAgeText artifact={artifact()} />}
                          </Show>
                        </TableCell>
                      </Show>
                      <Show when={props.showArchiveColumn}>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                        >
                          <Show
                            when={row.latestArchive}
                            fallback={
                              <span class="text-muted">{archiveSource.coverageFallbackLabel}</span>
                            }
                          >
                            {(artifact) => <ProxmoxBackupAgeText artifact={artifact()} />}
                          </Show>
                        </TableCell>
                      </Show>
                      <Show when={props.showSnapshotColumn}>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                        >
                          <Show
                            when={row.latestSnapshot}
                            fallback={
                              <span class="text-muted">{snapshotSource.coverageFallbackLabel}</span>
                            }
                          >
                            {(artifact) => <ProxmoxBackupAgeText artifact={artifact()} />}
                          </Show>
                        </TableCell>
                      </Show>
                      <Show when={props.showTaskColumn}>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                        >
                          <Show
                            when={row.latestTask}
                            fallback={<span class="text-muted">No recent task</span>}
                          >
                            {(task) => (
                              <div class="flex items-center gap-2">
                                <StatusDot
                                  size="sm"
                                  variant={
                                    task().label === 'Failed'
                                      ? 'danger'
                                      : task().label === 'OK'
                                        ? 'success'
                                        : 'warning'
                                  }
                                  title={task().label}
                                  ariaHidden
                                />
                                <span
                                  class={`truncate ${statusWordToneClass(
                                    taskWordVariant(task().label),
                                  )}`}
                                >
                                  {task().label}
                                </span>
                              </div>
                            )}
                          </Show>
                        </TableCell>
                      </Show>
                    </TableRow>
                    <Show when={isExpanded()}>
                      <InlineDetailTableRow
                        cellId={detailRowId()}
                        class="bg-surface-alt/40"
                        cellClass="px-3 py-2"
                        contentClass=""
                        colspan={columnCount()}
                        data-inline-detail-for={row.key}
                      >
                        <Show
                          when={evidence().length > 0}
                          fallback={
                            <div class="text-xs text-muted">
                              No restore evidence has been discovered for this workload.
                            </div>
                          }
                        >
                          <div class="overflow-hidden">
                            <div class="mb-1 flex items-center justify-between gap-2 text-[11px]">
                              <span class="font-medium text-base-content">Restore evidence</span>
                              <Show when={row.artifacts.length > evidence().length}>
                                <span class="text-muted">
                                  Showing {evidence().length} of {row.artifacts.length}
                                </span>
                              </Show>
                            </div>
                            <table class="w-full text-[11px]">
                              <thead>
                                <tr class="bg-surface-alt text-muted">
                                  <th class="px-2 py-0.5 text-left font-medium">Source</th>
                                  <th class="px-2 py-0.5 text-left font-medium">Location</th>
                                  <th class="px-2 py-0.5 text-right font-medium">
                                    {PROXMOX_BACKUP_COLUMN_LABELS.created}
                                  </th>
                                  <th class="px-2 py-0.5 text-right font-medium">Size</th>
                                  <th class="px-2 py-0.5 text-left font-medium">State</th>
                                  <th class="px-2 py-0.5 text-left font-medium">
                                    {PROXMOX_BACKUP_COLUMN_LABELS.details}
                                  </th>
                                </tr>
                              </thead>
                              <tbody class="divide-y divide-border-subtle">
                                <For each={evidence()}>
                                  {(artifact) => (
                                    <tr class="hover:bg-surface-hover">
                                      <td class="px-2 py-1">
                                        <ArtifactSourceBadge artifact={artifact} />
                                      </td>
                                      <td class="px-2 py-1 text-base-content">
                                        <span
                                          class="inline-block max-w-[18rem] truncate"
                                          title={artifact.location}
                                        >
                                          {artifact.location}
                                        </span>
                                      </td>
                                      <td class="px-2 py-1 text-right text-base-content">
                                        <ProxmoxBackupAgeText artifact={artifact} />
                                      </td>
                                      <td class="px-2 py-1 text-right tabular-nums text-base-content">
                                        <Show
                                          when={artifact.size && artifact.size > 0}
                                          fallback={<span class="text-muted">No size</span>}
                                        >
                                          {formatBytes(artifact.size ?? 0)}
                                        </Show>
                                      </td>
                                      <td class="px-2 py-1">
                                        <ArtifactStateBadge
                                          artifact={artifact}
                                          label={artifactStateLabel(artifact)}
                                        />
                                      </td>
                                      <td class="px-2 py-1 text-base-content">
                                        <span
                                          class="inline-block max-w-[24rem] truncate"
                                          title={artifact.detail}
                                        >
                                          {artifact.detail || '—'}
                                        </span>
                                      </td>
                                    </tr>
                                  )}
                                </For>
                              </tbody>
                            </table>
                          </div>
                        </Show>
                      </InlineDetailTableRow>
                    </Show>
                  </>
                );
              }}
            </For>
          </>
        }
      />
    </Show>
  );
}
