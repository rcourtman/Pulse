import { For, Show, createEffect, createMemo, type Accessor, type JSX } from 'solid-js';

import { createStore, reconcile } from 'solid-js/store';

import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import {
  formatPlatformTableBytesValue,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  PlatformDetailTable,
  PlatformDetailTableBody,
  PlatformDetailTableHeader,
  PlatformTableEmptyState,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import {
  getPlatformResourceDetailRowInteractionProps,
  PlatformResourceDetailToggleButton,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { StatusIndicatorVariant } from '@/utils/status';
import { useObservedElementWidth } from '@/hooks/useObservedElementWidth';

import {
  getWorkloadRecoveryPostureLabel,
  type WorkloadCoverageRow,
} from './proxmoxBackupRecoveryModel';
import type { CoverageSortKey } from './proxmoxBackupsTableModel';
import {
  ArtifactSourceBadge,
  ArtifactStateBadge,
  PROXMOX_BACKUP_COLUMN_LABELS,
  ProxmoxBackupAgeText,
  ProxmoxBackupWorkloadTypeBadge,
  SortableHead,
  artifactStateLabel,
} from './proxmoxBackupsTableShared';
import {
  getCoverageColumns,
  getCoverageColumnWidthStyle,
  getCoverageLayoutForContainer,
  isCoverageEvidenceColumnVisible,
  selectCoverageRestoreEvidence,
  type CoverageColumnId,
} from './proxmoxBackupsTablePresentation';
import { useProxmoxBackupTableWindowing } from './useProxmoxBackupTableWindowing';

// Each evidence column is already named by its header, so an empty cell says
// "None" instead of restating the source ("No PVE backup file"). The long form
// truncated at most widths and stays available as the hover title.
const COVERAGE_EMPTY_LABEL = 'None';
const COVERAGE_NO_BACKUP_TITLE = 'No PBS snapshot or PVE backup file';
// Matches the posture engine's own wording for a snapshot-only subject.
const COVERAGE_SNAPSHOT_ONLY_TITLE =
  'No PBS snapshot or PVE backup file. A snapshot is not a separate backup, so it does not count.';

function CoverageAgeFallback(props: { unknown?: boolean; emptyTitle: string; compact?: boolean }) {
  const title = () =>
    props.unknown
      ? 'One or more completed restore-point dates are unavailable or in the future. The latest point is unknown. Expand this row to inspect the evidence.'
      : props.emptyTitle;
  return (
    <span
      class={props.unknown ? 'font-semibold text-amber-600 dark:text-amber-300' : 'text-muted'}
      classList={{ 'text-[10px]': props.unknown && props.compact }}
      title={title()}
      aria-label={props.unknown ? `Unknown age. ${title()}` : undefined}
    >
      {props.unknown ? 'Unknown' : COVERAGE_EMPTY_LABEL}
    </span>
  );
}

const coveragePostureVariant = (
  posture: WorkloadCoverageRow['posture'],
): StatusIndicatorVariant => {
  if (posture === 'protected') return 'success';
  if (posture === 'unprotected') return 'danger';
  if (posture === 'attention') return 'warning';
  if (posture === 'checking') return 'info';
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

const compactPostureLabel = (posture: WorkloadCoverageRow['posture']): string => {
  switch (posture) {
    case 'protected':
      return 'Prot.';
    case 'attention':
      return 'Attn.';
    case 'unprotected':
      return 'Unprot.';
    case 'checking':
      return 'Check';
    case 'not-evaluated':
      return 'N/A';
    default:
      return 'Unknown';
  }
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

const postureExplanation = (row: WorkloadCoverageRow): string => {
  if (row.protectionPosture?.explanation) return row.protectionPosture.explanation;
  if (row.posture === 'checking') return "Pulse is checking this guest's backups.";
  if (row.posture === 'not-evaluated') {
    return 'This backup does not match a guest Pulse currently monitors, so it is not rated.';
  }
  return 'Pulse cannot tell yet whether this guest is backed up.';
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
  // The task column auto-hides when no workload has task history. Per-source
  // ages (PBS snapshot, PVE file, guest snapshot) live in the row expansion.
  showTaskColumn: boolean;
  layoutWidth?: Accessor<number | null | undefined>;
}) {
  const observedWidth = useObservedElementWidth();
  const layoutMode = createMemo(() => {
    const width = props.layoutWidth?.() ?? observedWidth.width();
    return typeof width === 'number' && width > 0 ? getCoverageLayoutForContainer(width) : 'full';
  });
  const visibleColumns = createMemo(() =>
    getCoverageColumns(layoutMode(), {
      task: props.showTaskColumn,
    }),
  );
  const visibleColumnIds = createMemo(() => visibleColumns().map((column) => column.id));
  const columnVisible = (column: CoverageColumnId) => visibleColumnIds().includes(column);
  const columnCount = () => visibleColumns().length;
  // Full uncertainty words must fit even beside a Job column at 320px. Numeric
  // ages were short enough for the usual gutters; compact ages use that space.
  const ageCellClass = (kind: 'text' | 'numeric-value') =>
    `${layoutMode() === 'compact' ? `px-0.5 py-1 ${kind === 'numeric-value' ? 'text-right' : ''}` : getPlatformTableCellClassForKind(kind)} text-base-content`;
  // Polling rebuilds recovery-model objects. Preserve logical row identity so
  // Solid does not replace the focused toggle (or the browser's scroll anchor).
  const [stableRows, setStableRows] = createStore<WorkloadCoverageRow[]>([]);
  createEffect(() => setStableRows(reconcile([...props.rows], { key: 'key' })));
  const tableWindow = useProxmoxBackupTableWindowing(() => [...stableRows]);

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
      <div
        ref={(element) => {
          observedWidth.setElement(element);
          tableWindow.setRootRef(element);
        }}
        data-proxmox-backups-table="coverage"
        data-proxmox-backups-layout={layoutMode()}
        data-proxmox-backups-windowed={tableWindow.isWindowed()}
      >
        <PlatformTableShell
          tableClass="min-w-0 table-fixed text-xs"
          colgroup={
            <colgroup>
              <For each={visibleColumns()}>
                {(column) => (
                  <col
                    style={getCoverageColumnWidthStyle(column.id, layoutMode(), visibleColumnIds())}
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
              <Show when={columnVisible('node')}>
                <TableHead class={getPlatformTableHeadClassForKind('text')}>Node</TableHead>
              </Show>
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
                label={
                  layoutMode() === 'compact'
                    ? 'Age'
                    : layoutMode() === 'basic'
                      ? 'Backup'
                      : 'Last backup'
                }
                sortKey="latest"
                currentSort={props.sortKey}
                direction={props.sortDirection}
                onSort={props.onSort}
                align="right"
                headClass={getPlatformTableHeadClassForKind('numeric-value')}
              />
              <Show when={columnVisible('task')}>
                <SortableHead
                  label={layoutMode() === 'compact' ? 'Job' : 'Task'}
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
              <Show when={tableWindow.topSpacerHeight() > 0}>
                <TableRow aria-hidden="true">
                  <TableCell
                    colspan={columnCount()}
                    class="border-0 p-0"
                    height={tableWindow.topSpacerHeight()}
                  />
                </TableRow>
              </Show>
              <For each={tableWindow.visibleItems()}>
                {(row) => {
                  const isExpanded = () => props.expandedKeys.has(row.key);
                  const evidence = () => selectCoverageRestoreEvidence(row.artifacts);
                  const detailRowId = () => `proxmox-coverage-evidence-${row.key}`;
                  return (
                    <>
                      <TableRow
                        {...getPlatformResourceDetailRowInteractionProps({
                          expanded: isExpanded(),
                          onToggle: () => props.onToggleExpand(row.key),
                        })}
                        data-proxmox-backup-row="coverage"
                      >
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
                            <div class="min-w-0">
                              <div class="truncate font-semibold">
                                {row.workload.name || row.workload.label}
                              </div>
                            </div>
                          </div>
                        </TableCell>
                        <Show when={columnVisible('type')}>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <ProxmoxBackupWorkloadTypeBadge
                              type={row.workload.type}
                              label={row.workload.typeLabel}
                            />
                          </TableCell>
                        </Show>
                        <Show when={columnVisible('targetId')}>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-muted font-mono text-[11px] tabular-nums`}
                          >
                            {row.workload.vmid || '—'}
                          </TableCell>
                        </Show>
                        <Show when={columnVisible('node')}>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <Show
                              when={row.workload.node}
                              fallback={<span class="text-muted">—</span>}
                            >
                              {(node) => (
                                <span class="inline-block max-w-full truncate" title={node()}>
                                  {node()}
                                </span>
                              )}
                            </Show>
                          </TableCell>
                        </Show>
                        <TableCell class={getPlatformTableCellClassForKind('text')}>
                          <div class="flex items-center gap-2">
                            <StatusDot
                              size="sm"
                              variant={coveragePostureVariant(row.posture)}
                              title={postureExplanation(row)}
                              ariaHidden
                            />
                            <span
                              title={postureExplanation(row)}
                              class={`truncate text-[11px] font-medium ${statusWordToneClass(
                                coveragePostureVariant(row.posture),
                              )}`}
                            >
                              <Show
                                when={layoutMode() === 'compact'}
                                fallback={getWorkloadRecoveryPostureLabel(row.posture)}
                              >
                                {compactPostureLabel(row.posture)}
                              </Show>
                            </span>
                          </div>
                        </TableCell>
                        <TableCell class={ageCellClass('numeric-value')}>
                          <Show
                            when={row.latestBackup}
                            fallback={
                              <CoverageAgeFallback
                                unknown={row.ageUnknown?.backup}
                                compact={layoutMode() === 'compact'}
                                emptyTitle={
                                  row.snapshotCount > 0
                                    ? COVERAGE_SNAPSHOT_ONLY_TITLE
                                    : COVERAGE_NO_BACKUP_TITLE
                                }
                              />
                            }
                          >
                            {(artifact) => (
                              <ProxmoxBackupAgeText
                                artifact={artifact()}
                                compact={layoutMode() === 'compact'}
                              />
                            )}
                          </Show>
                        </TableCell>
                        <Show when={columnVisible('task')}>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <Show
                              when={row.latestTask}
                              fallback={
                                <Show
                                  when={layoutMode() !== 'compact'}
                                  fallback={<span class="block text-center text-muted">—</span>}
                                >
                                  <span class="text-muted" title="No recent task">
                                    {COVERAGE_EMPTY_LABEL}
                                  </span>
                                </Show>
                              }
                            >
                              {(task) => (
                                <Show
                                  when={layoutMode() !== 'compact'}
                                  fallback={
                                    <div class="flex justify-center">
                                      <StatusDot
                                        size="sm"
                                        variant={taskWordVariant(task().label)}
                                        title={`Latest task: ${task().label}`}
                                        ariaHidden
                                      />
                                      <span class="sr-only">Latest task: {task().label}</span>
                                    </div>
                                  }
                                >
                                  <div class="flex items-center gap-2">
                                    <StatusDot
                                      size="sm"
                                      variant={taskWordVariant(task().label)}
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
                                </Show>
                              )}
                            </Show>
                          </TableCell>
                        </Show>
                      </TableRow>
                      <Show when={isExpanded()}>
                        <InlineDetailTableRow
                          cellId={detailRowId()}
                          class=""
                          cellClass="px-3 py-2 whitespace-normal"
                          contentClass="min-w-0 whitespace-normal"
                          colspan={columnCount()}
                          data-inline-detail-for={row.key}
                        >
                          <div class="mb-2 grid grid-cols-2 gap-x-3 gap-y-1 rounded-md border border-border-subtle bg-surface px-2 py-1.5 text-[11px] sm:grid-cols-4">
                            <span>
                              <span class="font-medium text-base-content">Type:</span>{' '}
                              <span class="text-muted">{row.workload.typeLabel}</span>
                            </span>
                            <span>
                              <span class="font-medium text-base-content">Target:</span>{' '}
                              <span class="font-mono text-muted">{row.workload.vmid || '—'}</span>
                            </span>
                            <span>
                              <span class="font-medium text-base-content">Node:</span>{' '}
                              <span class="text-muted">{row.workload.node || '—'}</span>
                            </span>
                            <span>
                              <span class="font-medium text-base-content">Workload:</span>{' '}
                              <span class="text-muted">{row.workload.label}</span>
                            </span>
                          </div>
                          <div class="mb-2 rounded-md border border-border-subtle bg-surface px-2 py-1.5 text-[11px] text-base-content">
                            <span class="font-medium">
                              {getWorkloadRecoveryPostureLabel(row.posture)}:
                            </span>{' '}
                            {postureExplanation(row)}
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
                              <PlatformDetailTable class="table-fixed text-[11px]">
                                <PlatformDetailTableHeader>
                                  <TableHead class={getPlatformTableHeadClassForKind('name')}>
                                    Source
                                  </TableHead>
                                  <Show
                                    when={isCoverageEvidenceColumnVisible(layoutMode(), 'location')}
                                  >
                                    <TableHead class={getPlatformTableHeadClassForKind('text')}>
                                      Location
                                    </TableHead>
                                  </Show>
                                  <TableHead
                                    class={getPlatformTableHeadClassForKind('numeric-value')}
                                  >
                                    {PROXMOX_BACKUP_COLUMN_LABELS.created}
                                  </TableHead>
                                  <Show
                                    when={isCoverageEvidenceColumnVisible(layoutMode(), 'size')}
                                  >
                                    <TableHead
                                      class={getPlatformTableHeadClassForKind('numeric-value')}
                                    >
                                      Size
                                    </TableHead>
                                  </Show>
                                  <TableHead class={getPlatformTableHeadClassForKind('badge')}>
                                    State
                                  </TableHead>
                                  <Show
                                    when={isCoverageEvidenceColumnVisible(layoutMode(), 'details')}
                                  >
                                    <TableHead class={getPlatformTableHeadClassForKind('text')}>
                                      {PROXMOX_BACKUP_COLUMN_LABELS.details}
                                    </TableHead>
                                  </Show>
                                </PlatformDetailTableHeader>
                                <PlatformDetailTableBody class="divide-border-subtle">
                                  <For each={evidence()}>
                                    {(artifact) => (
                                      <TableRow>
                                        <TableCell class={getPlatformTableCellClassForKind('name')}>
                                          <ArtifactSourceBadge artifact={artifact} />
                                        </TableCell>
                                        <Show
                                          when={isCoverageEvidenceColumnVisible(
                                            layoutMode(),
                                            'location',
                                          )}
                                        >
                                          <TableCell
                                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                                          >
                                            <span
                                              class="inline-block max-w-[18rem] truncate"
                                              title={artifact.location}
                                            >
                                              {artifact.location}
                                            </span>
                                          </TableCell>
                                        </Show>
                                        <TableCell
                                          class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                                        >
                                          <ProxmoxBackupAgeText artifact={artifact} />
                                        </TableCell>
                                        <Show
                                          when={isCoverageEvidenceColumnVisible(
                                            layoutMode(),
                                            'size',
                                          )}
                                        >
                                          <TableCell
                                            class={`${getPlatformTableCellClassForKind('numeric-value')} tabular-nums text-base-content`}
                                          >
                                            <Show
                                              when={artifact.size && artifact.size > 0}
                                              fallback={<span class="text-muted">No size</span>}
                                            >
                                              {formatPlatformTableBytesValue(artifact.size)}
                                            </Show>
                                          </TableCell>
                                        </Show>
                                        <TableCell
                                          class={getPlatformTableCellClassForKind('badge')}
                                        >
                                          <ArtifactStateBadge
                                            artifact={artifact}
                                            label={artifactStateLabel(artifact)}
                                          />
                                        </TableCell>
                                        <Show
                                          when={isCoverageEvidenceColumnVisible(
                                            layoutMode(),
                                            'details',
                                          )}
                                        >
                                          <TableCell
                                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                                          >
                                            <span
                                              class="inline-block max-w-[24rem] truncate"
                                              title={artifact.detailTitle ?? artifact.detail}
                                            >
                                              {artifact.detail || '—'}
                                            </span>
                                          </TableCell>
                                        </Show>
                                      </TableRow>
                                    )}
                                  </For>
                                </PlatformDetailTableBody>
                              </PlatformDetailTable>
                            </div>
                          </Show>
                        </InlineDetailTableRow>
                      </Show>
                    </>
                  );
                }}
              </For>
              <Show when={tableWindow.bottomSpacerHeight() > 0}>
                <TableRow aria-hidden="true">
                  <TableCell
                    colspan={columnCount()}
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
