import { For, Show, type Accessor, type JSX } from 'solid-js';

import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import {
  formatPlatformTableBytesValue,
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
  if (posture === 'protected') return 'success';
  if (posture === 'unprotected') return 'danger';
  if (posture === 'attention') return 'warning';
  return 'muted';
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

const providerLabel = (provider: string): string => {
  if (provider === 'proxmox-pbs') return 'Proxmox Backup Server';
  if (provider === 'proxmox-pve') return 'Proxmox VE';
  return provider;
};

const evidenceQualityLabel = (value: string): string => {
  if (!value) return 'Unknown';
  return value.charAt(0).toUpperCase() + value.slice(1);
};

// "Workload coverage" table: one row per workload answering "does this have a
// backup?" across PBS snapshots / PVE backup files / guest snapshots, each
// expanding to its restore evidence. Presentational — the parent owns the
// filtered+sorted memo, shared filters, and the expansion set. table-fixed + a
// colgroup keeps the columns from ballooning; the inner evidence table stays
// content-sized. Rows are single-line with one datum per column — identity
// (Type / Target ID / Node) gets its own columns, mirroring the by-date
// recoverable table, instead of stacking under the workload name.
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
    { id: 'workload', weight: 18 },
    { id: 'type', weight: 6 },
    { id: 'targetId', weight: 7 },
    { id: 'node', weight: 9 },
    { id: 'posture', weight: 11 },
    { id: 'latest', weight: 10 },
    ...(props.showPbsColumn ? [{ id: 'pbs', weight: 11 }] : []),
    ...(props.showArchiveColumn ? [{ id: 'archive', weight: 10 }] : []),
    ...(props.showSnapshotColumn ? [{ id: 'snapshot', weight: 11 }] : []),
    ...(props.showTaskColumn ? [{ id: 'task', weight: 8 }] : []),
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
        tableClass="min-w-[1080px] table-fixed text-xs"
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
            <TableHead class={getPlatformTableHeadClassForKind('text')}>Type</TableHead>
            <TableHead class={getPlatformTableHeadClassForKind('text')}>
              {PROXMOX_BACKUP_COLUMN_LABELS.targetId}
            </TableHead>
            <TableHead class={getPlatformTableHeadClassForKind('text')}>Node</TableHead>
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
                        <div class="flex min-w-0 items-center gap-2">
                          <PlatformResourceDetailToggleButton
                            expanded={isExpanded()}
                            resourceLabel={row.workload.label}
                            controlsId={detailRowId()}
                            onToggle={() => props.onToggleExpand(row.key)}
                          />
                          <span class="min-w-0 truncate font-semibold">
                            {row.workload.name || row.workload.label}
                          </span>
                        </div>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                      >
                        <ProxmoxBackupWorkloadTypeBadge
                          type={row.workload.type}
                          label={row.workload.typeLabel}
                        />
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-muted font-mono text-[11px] tabular-nums`}
                      >
                        {row.workload.vmid || '—'}
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                      >
                        <Show when={row.workload.node} fallback={<span class="text-muted">—</span>}>
                          {(node) => (
                            <span class="inline-block max-w-full truncate" title={node()}>
                              {node()}
                            </span>
                          )}
                        </Show>
                      </TableCell>
                      <TableCell class={getPlatformTableCellClassForKind('text')}>
                        <div class="flex items-center gap-2">
                          <StatusDot
                            size="sm"
                            variant={coveragePostureVariant(row.posture)}
                            title={
                              row.protectionPosture?.explanation ??
                              'Pulse does not have enough provider evidence to determine protection.'
                            }
                            ariaHidden
                          />
                          <span
                            title={row.protectionPosture?.explanation}
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
                        <div class="mb-2 rounded-md border border-border-subtle bg-surface px-2 py-1.5 text-[11px] text-base-content">
                          <span class="font-medium">
                            {getWorkloadRecoveryPostureLabel(row.posture)}:
                          </span>{' '}
                          {row.protectionPosture?.explanation ??
                            'Pulse cannot determine this workload’s protection because no complete provider evidence is linked to it.'}
                        </div>
                        <Show when={(row.protectionPosture?.providerStates.length ?? 0) > 0}>
                          <div class="mb-2 overflow-hidden rounded-md border border-border-subtle">
                            <div class="bg-surface-alt px-2 py-1 text-[11px] font-medium text-base-content">
                              Provider evidence
                            </div>
                            <div class="divide-y divide-border-subtle">
                              <For each={row.protectionPosture?.providerStates ?? []}>
                                {(provider) => (
                                  <div class="grid grid-cols-2 gap-x-3 gap-y-0.5 px-2 py-1.5 text-[11px] sm:grid-cols-4">
                                    <span class="font-medium text-base-content">
                                      {providerLabel(provider.provider)}
                                    </span>
                                    <span class="text-muted">
                                      Job {evidenceQualityLabel(provider.jobState)}
                                    </span>
                                    <span class="text-muted">
                                      History {evidenceQualityLabel(provider.historyCompleteness)}
                                    </span>
                                    <span class="text-muted">
                                      Access {evidenceQualityLabel(provider.permissions)}
                                    </span>
                                  </div>
                                )}
                              </For>
                            </div>
                          </div>
                        </Show>
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
                                          {formatPlatformTableBytesValue(artifact.size)}
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
