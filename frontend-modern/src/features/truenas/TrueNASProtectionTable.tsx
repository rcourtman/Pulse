import { Show, createMemo, type Component, type JSX } from 'solid-js';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableRow } from '@/components/shared/Table';
import {
  getRecoveryOutcomeBadgeClass,
  getRecoveryOutcomeLabel,
  normalizeRecoveryOutcome,
} from '@/utils/recoveryOutcomePresentation';
import { asTrimmedString } from '@/utils/stringUtils';
import type { StatusIndicatorVariant } from '@/utils/status';
import {
  PlatformWindowedRows,
  PlatformErrorState,
  PlatformResponsiveTableLabel,
  PlatformSortableTableHead,
  PlatformTableDateTimeValue,
  PlatformTableEmptyState,
  PlatformTableLoadingState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableBytesValue,
  getPlatformTableCellClassForKind,
  getPlatformTableDateTimeSortValue,
  withPlatformStatusCounts,
  type PlatformTableFilterOption,
  type PlatformTableSortValue,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { RecoveryPoint } from '@/types/recovery';
import {
  filterTrueNASProtectionPoints,
  buildTrueNASProtectionPosture,
  mapTrueNASProtectionKind,
  mapTrueNASProtectionStatus,
  sortTrueNASProtectionPoints,
  type TrueNASProtectionKind,
  type TrueNASProtectionStatusBucket,
  type TrueNASProtectionStatusFilter,
} from './truenasPageModel';
import {
  InlineDetailPanel,
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailSection,
  type DetailValueTone,
} from '@/components/shared/DetailSectionTable';

const TRUENAS_PROTECTION_STATUS_OPTIONS: PlatformTableFilterOption<TrueNASProtectionStatusFilter>[] =
  [
    { value: 'all', label: 'All' },
    {
      value: 'attention',
      label: 'Attention',
      compactLabel: 'Issues',
      tone: 'warning',
    },
    { value: 'success', label: 'Healthy', compactLabel: 'OK', tone: 'success' },
    { value: 'warning', label: 'Warning', compactLabel: 'Warn', tone: 'warning' },
    { value: 'failed', label: 'Failed', compactLabel: 'Fail', tone: 'danger' },
    { value: 'running', label: 'Running', compactLabel: 'Run', tone: 'info' },
    { value: 'unknown', label: 'Unknown', compactLabel: 'Unk', tone: 'muted' },
  ];

const detailString = (point: RecoveryPoint, key: string): string =>
  asTrimmedString(point.details?.[key]) || '';

const detailStringList = (point: RecoveryPoint, key: string): string[] => {
  const value = point.details?.[key];
  return Array.isArray(value)
    ? value.filter((item): item is string => typeof item === 'string' && item.trim().length > 0)
    : [];
};

const kindLabel = (kind: TrueNASProtectionKind): string => {
  if (kind === 'snapshot') return 'Snapshot';
  if (kind === 'replication') return 'Replication';
  return 'Protection';
};

const protectionVariant = (status: TrueNASProtectionStatusBucket): StatusIndicatorVariant => {
  switch (status) {
    case 'success':
      return 'success';
    case 'warning':
      return 'warning';
    case 'failed':
      return 'danger';
    case 'running':
    case 'unknown':
      return 'muted';
  }
};

const datasetLabel = (point: RecoveryPoint): string =>
  asTrimmedString(point.display?.itemLabel) ||
  asTrimmedString(point.itemRef?.name) ||
  detailString(point, 'dataset') ||
  detailStringList(point, 'sourceDatasets')[0] ||
  point.id;

const hostLabel = (point: RecoveryPoint): string =>
  asTrimmedString(point.display?.nodeHostLabel) ||
  asTrimmedString(point.display?.nodeAgentLabel) ||
  asTrimmedString(point.display?.clusterLabel) ||
  detailString(point, 'hostname') ||
  asTrimmedString(point.node) ||
  asTrimmedString(point.cluster) ||
  'TrueNAS';

const artifactLabel = (point: RecoveryPoint): string => {
  const kind = mapTrueNASProtectionKind(point);
  if (kind === 'replication') {
    return (
      detailString(point, 'taskName') ||
      asTrimmedString(point.display?.detailsSummary) ||
      asTrimmedString(point.repositoryRef?.name) ||
      'Replication task'
    );
  }
  if (kind === 'snapshot') {
    return (
      detailString(point, 'snapshot') ||
      detailString(point, 'fullName') ||
      asTrimmedString(point.display?.detailsSummary) ||
      'ZFS snapshot'
    );
  }
  return asTrimmedString(point.display?.detailsSummary) || point.kind || 'Protection point';
};

const artifactSecondaryLabel = (point: RecoveryPoint): string => {
  const kind = mapTrueNASProtectionKind(point);
  if (kind === 'replication') {
    return (
      detailString(point, 'lastSnapshot') ||
      detailString(point, 'lastState') ||
      asTrimmedString(point.display?.detailsSummary) ||
      ''
    );
  }
  if (kind === 'snapshot') {
    const fullName = detailString(point, 'fullName');
    const snapshot = detailString(point, 'snapshot');
    return fullName && fullName !== snapshot ? fullName : '';
  }
  return '';
};

const targetLabel = (point: RecoveryPoint): string => {
  if (mapTrueNASProtectionKind(point) === 'snapshot') return 'Local ZFS';
  return (
    asTrimmedString(point.display?.repositoryLabel) ||
    detailString(point, 'targetDataset') ||
    asTrimmedString(point.repositoryRef?.name) ||
    '-'
  );
};

const targetSecondaryLabel = (point: RecoveryPoint): string => {
  if (mapTrueNASProtectionKind(point) === 'snapshot') return datasetLabel(point);
  const direction = detailString(point, 'direction');
  const state = detailString(point, 'lastState');
  return [direction, state].filter(Boolean).join(' / ');
};

const sizeLabel = (point: RecoveryPoint): string =>
  formatPlatformTableBytesValue(point.sizeBytes ?? undefined, '-');

// TrueNAS replication supplies LastRun, not independent start/end times. The
// recovery mapper mirrors it into both transport fields, even while running.
// Snapshot StartedAt carries creation, while a CompletedAt-only snapshot uses
// the mapper's collection-time fallback. Keep those meanings at the consumer.
const activityTime = (point: RecoveryPoint): { label: string; value: string | undefined } => {
  const started = asTrimmedString(point.startedAt);
  const completed = asTrimmedString(point.completedAt);
  const kind = mapTrueNASProtectionKind(point);
  if (kind === 'replication') return { label: 'Last run', value: completed || started };
  if (kind === 'snapshot') {
    return started ? { label: 'Created', value: started } : { label: 'Observed', value: completed };
  }
  return completed
    ? { label: 'Completed', value: completed }
    : { label: 'Started', value: started };
};

// Keep a readable date and clock on phones instead of an ellipsised timestamp.
// Both projections use the same instant as sorting and the existing detail;
// a LastRun is still not independent successful-completion evidence.
const ProtectionActivityTime: Component<{ point: RecoveryPoint }> = (props) => {
  const date = createMemo(() => {
    const value = getPlatformTableDateTimeSortValue(activityTime(props.point).value);
    return value === null ? undefined : new Date(value);
  });
  return (
    <Show when={date()} fallback={<span>-</span>}>
      {(value) => (
        <time
          dateTime={value().toISOString()}
          title={`${activityTime(props.point).label}: ${value().toLocaleString()}`}
          class="tabular-nums"
        >
          <span class="sr-only">{activityTime(props.point).label}: </span>
          <span class="platform-table-phone-hidden-inline">
            <PlatformTableDateTimeValue value={value()} emptyText="-" />
          </span>
          <span class="platform-table-phone-only-inline flex-col items-end leading-none">
            <span>{value().toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}</span>{' '}
            <span>
              {value().toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })}
            </span>
          </span>
        </time>
      )}
    </Show>
  );
};

// Columns a user can sort by. Every protection column carries a scalar:
// Reported time orders chronologically and Size on raw bytes, both newest/
// biggest first on the first click. Retain the internal time key for saved sorts.
const TRUENAS_PROTECTION_SORT_KEYS = [
  'dataset',
  'artifact',
  'target',
  'completed',
  'size',
  'outcome',
] as const;

type TrueNASProtectionSortKey = (typeof TRUENAS_PROTECTION_SORT_KEYS)[number];

const getTrueNASProtectionSortValue = (
  point: RecoveryPoint,
  key: TrueNASProtectionSortKey,
): PlatformTableSortValue => {
  switch (key) {
    case 'dataset':
      return datasetLabel(point);
    case 'artifact':
      return artifactLabel(point);
    case 'target': {
      const target = targetLabel(point);
      return target === '-' ? null : target;
    }
    case 'completed':
      return getPlatformTableDateTimeSortValue(activityTime(point).value);
    case 'size':
      return typeof point.sizeBytes === 'number' && Number.isFinite(point.sizeBytes)
        ? point.sizeBytes
        : null;
    case 'outcome':
      return getRecoveryOutcomeLabel(normalizeRecoveryOutcome(point.outcome));
    default:
      key satisfies never;
      return null;
  }
};

type ProtectionDetailTone = DetailValueTone;
type ProtectionDetailSection = DetailSection;

const detailBool = (value?: boolean | null): string | null => {
  if (value == null) return null;
  return value ? 'Yes' : 'No';
};

const detailDateTime = (value?: string | null): string | null => {
  const raw = asTrimmedString(value);
  if (!raw) return null;
  const parsed = new Date(raw);
  if (Number.isNaN(parsed.getTime())) return null;
  return parsed.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
};

// These disclosures are the phone path to fields omitted from the summary.
// Keep their actual text readable on touch, not only in desktop hover titles.
const detailRow: typeof makeDetailRow = (label, value, options) =>
  makeDetailRow(label, value, { wrap: true, layout: 'stacked', ...options });

const outcomeTone = (point: RecoveryPoint): ProtectionDetailTone => {
  const outcome = normalizeRecoveryOutcome(point.outcome);
  if (outcome === 'success') return 'success';
  if (outcome === 'running') return 'muted';
  if (outcome === 'failed') return 'danger';
  if (outcome === 'warning') return 'warning';
  return 'default';
};

const sourceDatasetsLabel = (point: RecoveryPoint): string | null => {
  const datasets = detailStringList(point, 'sourceDatasets');
  return datasets.length > 0 ? datasets.join(', ') : null;
};

const buildProtectionDetailSections = (point: RecoveryPoint): ProtectionDetailSection[] => {
  const kind = mapTrueNASProtectionKind(point);
  const time = activityTime(point);
  const reportedError = kind === 'replication' ? detailString(point, 'lastError') : '';
  const target = targetLabel(point);
  const targetSecondary = targetSecondaryLabel(point);
  const summaryRows = compactDetailRows([
    detailRow('Kind', kindLabel(kind)),
    detailRow('Outcome', getRecoveryOutcomeLabel(normalizeRecoveryOutcome(point.outcome)), {
      tone: outcomeTone(point),
    }),
    detailRow('Dataset', datasetLabel(point)),
    detailRow('Host', hostLabel(point)),
    ...(kind === 'other'
      ? [
          detailRow('Started', detailDateTime(point.startedAt)),
          detailRow('Completed', detailDateTime(point.completedAt)),
        ]
      : [detailRow(time.label, detailDateTime(time.value) || 'Unknown')]),
    detailRow('Size', sizeLabel(point)),
    detailRow('Verified', detailBool(point.verified)),
    detailRow('Encrypted', detailBool(point.encrypted)),
    detailRow('Immutable', detailBool(point.immutable)),
  ]);

  const artifactRows = compactDetailRows([
    detailRow('Artifact', artifactLabel(point)),
    detailRow('Summary', asTrimmedString(point.display?.detailsSummary)),
    detailRow('Full name', detailString(point, 'fullName')),
    detailRow('Snapshot', detailString(point, 'snapshot')),
    detailRow('Last snapshot', detailString(point, 'lastSnapshot')),
    detailRow('Task name', detailString(point, 'taskName')),
    detailRow('Task ID', detailString(point, 'taskId') || detailString(point, 'upid')),
    detailRow('Reported error', reportedError, {
      tone: outcomeTone(point),
      layout: 'stacked',
      valueContent: (
        <span class="block whitespace-pre-wrap text-left leading-snug wrap-anywhere">
          {reportedError}
        </span>
      ),
    }),
  ]);

  const targetRows = compactDetailRows([
    detailRow('Target', target === 'Local ZFS' ? target : target || null),
    detailRow('Target detail', targetSecondary),
    detailRow('Direction', detailString(point, 'direction')),
    detailRow('Last state', detailString(point, 'lastState'), {
      tone: detailString(point, 'lastState').toLowerCase() === 'running' ? 'muted' : 'default',
    }),
    detailRow('Source datasets', sourceDatasetsLabel(point)),
    detailRow('Target dataset', detailString(point, 'targetDataset')),
    detailRow('Repository', asTrimmedString(point.repositoryRef?.name)),
  ]);

  return compactDetailSections([
    { label: 'Protection', rows: summaryRows },
    { label: kind === 'replication' ? 'Replication' : 'Snapshot', rows: artifactRows },
    { label: 'Target', rows: targetRows },
  ]);
};

const DatasetCell: Component<{ point: RecoveryPoint; detailToggle?: JSX.Element }> = (props) => {
  const status = () => mapTrueNASProtectionStatus(props.point);
  const kind = () => mapTrueNASProtectionKind(props.point);
  const name = () => datasetLabel(props.point);
  const subtitle = () => `${kindLabel(kind())} on ${hostLabel(props.point)}`;

  return (
    <div class="flex min-w-0 items-center gap-2">
      {props.detailToggle}
      <StatusDot
        size="sm"
        variant={protectionVariant(status())}
        pulse={status() === 'running'}
        title={getRecoveryOutcomeLabel(normalizeRecoveryOutcome(props.point.outcome))}
      />
      <div class="min-w-0 leading-none">
        <div
          class="truncate font-medium text-base-content"
          title={[name(), subtitle()].filter(Boolean).join(' · ')}
        >
          {name()}
        </div>
        <span class="platform-table-phone-only-inline text-[10px] text-muted">
          {kindLabel(kind())}
        </span>
      </div>
    </div>
  );
};

const ProtectionDetailTable: Component<{ point: RecoveryPoint; onClose: () => void }> = (props) => (
  <InlineDetailPanel
    testId="truenas-protection-detail"
    detailFor={props.point.id}
    title="Protection detail"
    summary={`${kindLabel(mapTrueNASProtectionKind(props.point))} · ${getRecoveryOutcomeLabel(
      normalizeRecoveryOutcome(props.point.outcome),
    )}`}
    sections={buildProtectionDetailSections(props.point)}
    detailAttributes={{ 'data-truenas-protection-detail-for': props.point.id }}
    onClose={props.onClose}
  />
);

export const TrueNASProtectionTable: Component<{
  points: RecoveryPoint[];
  loading?: boolean;
  error?: unknown;
  onRefresh?: () => void;
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
}> = (props) => {
  const rows = createMemo(() => sortTrueNASProtectionPoints(props.points));
  const posture = createMemo(() => buildTrueNASProtectionPosture(rows()));
  const detail = createPlatformResourceDetailState({ idPrefix: 'truenas-protection-detail' });
  const tableState = createPlatformTableFilterState({
    resources: rows,
    initialStatus: 'all' as TrueNASProtectionStatusFilter,
    filter: filterTrueNASProtectionPoints,
  });
  const statusOptions = createMemo(() =>
    withPlatformStatusCounts(TRUENAS_PROTECTION_STATUS_OPTIONS, tableState.countForStatus, {
      value: 'attention',
      tone: posture().failed > 0 ? 'danger' : 'warning',
    }),
  );
  const sort = createPlatformTableSortState({
    storageKey: 'truenasProtection',
    sortKeys: TRUENAS_PROTECTION_SORT_KEYS,
    descendingFirst: ['completed', 'size'],
  });
  const sortedRows = createMemo(() =>
    sort.sortRows(tableState.filtered(), getTrueNASProtectionSortValue),
  );

  return (
    <Show
      when={!props.loading || Boolean(props.error) || rows().length > 0}
      fallback={
        <PlatformTableLoadingState
          title="Loading TrueNAS protection"
          description="Pulse is loading ZFS snapshot and replication activity."
        />
      }
    >
      <Show
        when={!props.error || rows().length > 0}
        fallback={
          <PlatformErrorState
            title="Could not load TrueNAS protection"
            description="Protection activity is unavailable, not necessarily absent. Refresh to try again."
            onRefresh={() => props.onRefresh?.()}
            refreshDisabled={props.loading}
          />
        }
      >
        <Show
          when={rows().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title={props.emptyTitle}
              description={props.emptyDescription}
            />
          }
        >
          <div class="space-y-3">
            <Show when={props.error}>
              <div class="flex flex-wrap items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">
                <p role="status" class="min-w-0 flex-1 basis-64">
                  Could not refresh TrueNAS protection. Showing the last loaded records. Counts and
                  outcomes may be out of date. These records do not establish current protection.
                </p>
                <button
                  type="button"
                  class="min-h-10 rounded-md border border-current px-3 py-2 font-medium hover:underline focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-amber-500 disabled:cursor-wait disabled:opacity-60"
                  disabled={props.loading}
                  onClick={() => {
                    if (!props.loading) props.onRefresh?.();
                  }}
                >
                  {props.loading
                    ? 'Refreshing protection activity…'
                    : 'Refresh protection activity'}
                </button>
              </div>
            </Show>
            <Show when={props.showToolbar !== false}>
              <PlatformTableToolbar
                search={tableState.search}
                onSearchChange={tableState.setSearch}
                searchPlaceholder="Search snapshots or replication"
                searchSuggestions={tableState.searchSuggestions}
                status={tableState.status()}
                onStatusChange={tableState.setStatus}
                statusOptions={statusOptions()}
                visible={tableState.visible()}
                total={tableState.total()}
                rowNoun="events"
              />
            </Show>

            <Show
              when={tableState.filtered().length > 0}
              fallback={
                <PlatformTableEmptyState
                  icon={props.emptyIcon}
                  title="No protection events match current filters"
                  description="Adjust the search or outcome filter to see more TrueNAS snapshots and replication tasks."
                />
              }
            >
              <PlatformTableShell
                title="Snapshots & Replication"
                actions={
                  <span class="text-[10px] font-medium text-muted">
                    {tableState.total()} event{tableState.total() === 1 ? '' : 's'}
                  </span>
                }
                tableClass="min-w-full table-fixed text-xs md:min-w-[960px]"
                header={
                  <>
                    <PlatformSortableTableHead
                      kind="name"
                      sort={sort}
                      sortKey="dataset"
                      class="platform-table-mobile-w-30 md:w-[22%]"
                    >
                      Dataset
                    </PlatformSortableTableHead>
                    <PlatformSortableTableHead
                      kind="text"
                      sort={sort}
                      sortKey="artifact"
                      class="platform-table-phone-hidden md:w-[25%]"
                    >
                      Artifact
                    </PlatformSortableTableHead>
                    <PlatformSortableTableHead
                      kind="text"
                      sort={sort}
                      sortKey="target"
                      class="platform-table-phone-hidden md:w-[23%]"
                    >
                      Target
                    </PlatformSortableTableHead>
                    <PlatformSortableTableHead
                      kind="numeric-value"
                      sort={sort}
                      sortKey="completed"
                      class="platform-table-mobile-w-30 md:w-[12%]"
                    >
                      <PlatformResponsiveTableLabel compact="Time" full="Reported time" />
                    </PlatformSortableTableHead>
                    <PlatformSortableTableHead
                      kind="numeric-value"
                      sort={sort}
                      sortKey="size"
                      class="platform-table-phone-hidden hidden lg:table-cell md:w-[8%]"
                    >
                      Size
                    </PlatformSortableTableHead>
                    <PlatformSortableTableHead
                      kind="badge"
                      sort={sort}
                      sortKey="outcome"
                      class="platform-table-mobile-w-30 md:w-[9%]"
                    >
                      Outcome
                    </PlatformSortableTableHead>
                  </>
                }
                body={
                  <>
                    <PlatformWindowedRows items={sortedRows} estimatedRowHeight={32}>
                      {(point) => {
                        const outcome = () => normalizeRecoveryOutcome(point.outcome);
                        const artifact = () => artifactLabel(point);
                        const artifactSecondary = () => artifactSecondaryLabel(point);
                        const target = () => targetLabel(point);
                        const targetSecondary = () => targetSecondaryLabel(point);
                        const detailRowId = () => detail.detailRowId(point);
                        const isExpanded = () => detail.isExpanded(point);
                        return (
                          <>
                            <TableRow
                              class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                              data-truenas-protection-row={point.id}
                              data-truenas-protection-kind={mapTrueNASProtectionKind(point)}
                              data-truenas-protection-outcome={mapTrueNASProtectionStatus(point)}
                              onClick={() => detail.toggle(point)}
                            >
                              <TableCell class={getPlatformTableCellClassForKind('name')}>
                                <DatasetCell
                                  point={point}
                                  detailToggle={
                                    <PlatformResourceDetailToggleButton
                                      expanded={isExpanded()}
                                      resourceLabel={datasetLabel(point)}
                                      controlsId={detailRowId()}
                                      onToggle={() => detail.toggle(point)}
                                    />
                                  }
                                />
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('text')} platform-table-phone-hidden`}
                              >
                                <span
                                  class="block truncate text-base-content"
                                  title={[artifact(), artifactSecondary()]
                                    .filter(Boolean)
                                    .join(' · ')}
                                >
                                  {artifact()}
                                </span>
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('text')} platform-table-phone-hidden`}
                              >
                                <span
                                  class="block truncate text-base-content"
                                  title={[target(), targetSecondary()].filter(Boolean).join(' · ')}
                                >
                                  {target()}
                                </span>
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                              >
                                <ProtectionActivityTime point={point} />
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('numeric-value')} platform-table-phone-hidden hidden text-base-content lg:table-cell`}
                              >
                                {sizeLabel(point)}
                              </TableCell>
                              <TableCell class={getPlatformTableCellClassForKind('badge')}>
                                <span class={getRecoveryOutcomeBadgeClass(outcome())}>
                                  {getRecoveryOutcomeLabel(outcome())}
                                </span>
                              </TableCell>
                            </TableRow>
                            <Show when={isExpanded()}>
                              <InlineDetailTableRow
                                cellId={detailRowId()}
                                colspan={6}
                                data-inline-detail-for={point.id}
                                data-truenas-protection-detail-row={point.id}
                              >
                                <ProtectionDetailTable
                                  point={point}
                                  onClose={() => detail.close(point)}
                                />
                              </InlineDetailTableRow>
                            </Show>
                          </>
                        );
                      }}
                    </PlatformWindowedRows>
                  </>
                }
              />
            </Show>
          </div>
        </Show>
      </Show>
    </Show>
  );
};

export default TrueNASProtectionTable;
