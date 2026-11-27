import { Show, createMemo, type Component, type JSX } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { TableCell, TableRow } from '@/components/shared/Table';
import {
  getRecoveryOutcomeBadgeClass,
  getRecoveryOutcomeLabel,
  normalizeRecoveryOutcome,
} from '@/utils/recoveryOutcomePresentation';
import { asTrimmedString } from '@/utils/stringUtils';
import { parseBackupTimestampMs } from '@/utils/backupTimestamp';
import type { StatusIndicatorVariant } from '@/utils/status';
import {
  PlatformWindowedRows,
  PlatformErrorState,
  PlatformResponsiveTableLabel,
  PlatformSortableTableHead,
  PlatformTableDateTimeValue,
  PlatformTableLoadingState,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableBytesValue,
  getPlatformTableCellClassForKind,
  withPlatformStatusCounts,
  type PlatformTableFilterOption,
  type PlatformTableSortValue,
} from '@/features/platformPage/sharedPlatformPage';
import {
  TrueNASRowLead,
  TrueNASStaleBadge,
  TrueNASTableFrame,
  createTrueNASDetailState,
} from './TrueNASTableFrame';
import { getPlatformResourceDetailRowClass } from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { RecoveryPoint } from '@/types/recovery';
import type { Resource } from '@/types/resource';
import {
  buildTrueNASProtectionPosture,
  buildTrueNASProtectionStaleNoticeMap,
  filterTrueNASProtectionPoints,
  mapTrueNASProtectionKind,
  mapTrueNASProtectionStatus,
  sortTrueNASProtectionPoints,
  type TrueNASProtectionKind,
  type TrueNASProtectionStaleNotice,
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
import { RecordOwnerLink } from '@/features/platformPage/RecordOwnerLink';

const TRUENAS_PROTECTION_STALE_OPTION: PlatformTableFilterOption<TrueNASProtectionStatusFilter> = {
  value: 'stale',
  label: 'Stale',
  compactLabel: 'Stale',
  tone: 'warning',
};

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

const firstText = (...values: unknown[]): string => {
  for (const value of values) {
    const trimmed = asTrimmedString(value);
    if (trimmed) return trimmed;
  }
  return '';
};

const detailString = (point: RecoveryPoint, key: string): string =>
  asTrimmedString(point.details?.[key]) || '';

const detailStringList = (point: RecoveryPoint, key: string): string[] => {
  const value = point.details?.[key];
  return Array.isArray(value)
    ? value.filter((item): item is string => typeof item === 'string' && item.trim().length > 0)
    : [];
};

const kindLabel = (kind: TrueNASProtectionKind): string =>
  kind === 'snapshot' ? 'Snapshot' : kind === 'replication' ? 'Replication' : 'Protection';

const protectionVariant = (status: TrueNASProtectionStatusBucket): StatusIndicatorVariant =>
  status === 'success'
    ? 'success'
    : status === 'warning'
      ? 'warning'
      : status === 'failed'
        ? 'danger'
        : 'muted';

const datasetLabel = (point: RecoveryPoint): string =>
  firstText(
    point.display?.itemLabel,
    point.itemRef?.name,
    point.details?.dataset,
    detailStringList(point, 'sourceDatasets')[0],
    point.id,
  );

const hostLabel = (point: RecoveryPoint): string =>
  firstText(
    point.display?.nodeHostLabel,
    point.display?.nodeAgentLabel,
    point.display?.clusterLabel,
    point.details?.hostname,
    point.node,
    point.cluster,
  ) || 'TrueNAS';

const artifactLabel = (point: RecoveryPoint): string => {
  const kind = mapTrueNASProtectionKind(point);
  if (kind === 'replication') {
    return (
      firstText(
        point.details?.taskName,
        point.display?.detailsSummary,
        point.repositoryRef?.name,
      ) || 'Replication task'
    );
  }
  if (kind === 'snapshot') {
    return (
      firstText(point.details?.snapshot, point.details?.fullName, point.display?.detailsSummary) ||
      'ZFS snapshot'
    );
  }
  return firstText(point.display?.detailsSummary, point.kind) || 'Protection point';
};

const artifactSecondaryLabel = (point: RecoveryPoint): string => {
  const kind = mapTrueNASProtectionKind(point);
  if (kind === 'replication') {
    return firstText(
      point.details?.lastSnapshot,
      point.details?.lastState,
      point.display?.detailsSummary,
    );
  }
  if (kind === 'snapshot') {
    const fullName = detailString(point, 'fullName');
    return fullName && fullName !== detailString(point, 'snapshot') ? fullName : '';
  }
  return '';
};

const targetLabel = (point: RecoveryPoint): string =>
  mapTrueNASProtectionKind(point) === 'snapshot'
    ? 'Local ZFS'
    : firstText(
        point.display?.repositoryLabel,
        point.details?.targetDataset,
        point.repositoryRef?.name,
      ) || '—';

const targetSecondaryLabel = (point: RecoveryPoint): string =>
  mapTrueNASProtectionKind(point) === 'snapshot'
    ? datasetLabel(point)
    : [detailString(point, 'direction'), detailString(point, 'lastState')]
        .filter(Boolean)
        .join(' / ');

const sizeLabel = (point: RecoveryPoint): string =>
  formatPlatformTableBytesValue(point.sizeBytes ?? undefined, '—');

// A start is not a finish. Running replication can also retain a last-run
// timestamp in completedAt, which is not completion of the active run.
const completionTimestamp = (point: RecoveryPoint): string | undefined =>
  normalizeRecoveryOutcome(point.outcome) === 'running' ? '' : asTrimmedString(point.completedAt);

// Columns a user can sort by. Every protection column carries a scalar:
// Completed orders chronologically and Size on raw bytes, both newest/biggest
// first on the first click.
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
  staleNotice?: TrueNASProtectionStaleNotice,
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
      return parseBackupTimestampMs(completionTimestamp(point)) ?? null;
    case 'size':
      return typeof point.sizeBytes === 'number' && Number.isFinite(point.sizeBytes)
        ? point.sizeBytes
        : null;
    case 'outcome': {
      const outcome = normalizeRecoveryOutcome(point.outcome);
      return staleNotice && outcome === 'success'
        ? staleNotice.badge
        : getRecoveryOutcomeLabel(outcome);
    }
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
  const timestamp = parseBackupTimestampMs(raw);
  // Keep malformed evidence inspectable without letting Date repair its
  // calendar or invent a browser-local zone. It is not an observed instant.
  if (timestamp === undefined) return `Unknown date (${raw})`;
  return new Date(timestamp).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
};

const detailRow = makeDetailRow;

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

const buildProtectionDetailSections = (
  point: RecoveryPoint,
  staleNotice?: TrueNASProtectionStaleNotice | null,
): ProtectionDetailSection[] => {
  const kind = mapTrueNASProtectionKind(point);
  const outcome = normalizeRecoveryOutcome(point.outcome);
  const outcomeLabel = getRecoveryOutcomeLabel(outcome);
  const target = targetLabel(point);
  const targetSecondary = targetSecondaryLabel(point);
  const summaryRows = compactDetailRows([
    detailRow('Kind', kindLabel(kind)),
    ...(staleNotice
      ? [
          detailRow('Status', staleNotice.badge, { tone: 'warning' }),
          detailRow('Last reported', outcomeLabel, { tone: outcomeTone(point) }),
        ]
      : [detailRow('Outcome', outcomeLabel, { tone: outcomeTone(point) })]),
    detailRow('Dataset', datasetLabel(point)),
    detailRow('Host', hostLabel(point)),
    detailRow('Started', detailDateTime(point.startedAt)),
    detailRow('Completed', detailDateTime(completionTimestamp(point))),
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

export const TrueNASProtectionTable: Component<{
  points: RecoveryPoint[];
  systems?: Resource[];
  loading?: boolean;
  error?: unknown;
  onRefresh?: () => void;
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
  /** The page's URL search, when it owns one; the table then reads and writes it. */
  search?: () => string;
  onSearchChange?: (value: string) => void;
}> = (props) => {
  const staleNoticeByPointId = createMemo(() =>
    buildTrueNASProtectionStaleNoticeMap(props.points, props.systems ?? []),
  );
  const rows = createMemo(() => sortTrueNASProtectionPoints(props.points, staleNoticeByPointId()));
  const posture = createMemo(() => buildTrueNASProtectionPosture(rows(), staleNoticeByPointId()));
  const detail = createTrueNASDetailState('truenas-protection-detail');
  const tableState = createPlatformTableFilterState({
    resources: rows,
    initialStatus: 'all' as TrueNASProtectionStatusFilter,
    filter: (items, search, status) =>
      filterTrueNASProtectionPoints(items, search, status, staleNoticeByPointId()),
    externalSearch: props.search,
    onExternalSearchChange: props.onSearchChange,
  });
  const statusOptions = createMemo(() => {
    const showStaleFilter = posture().stale > 0 || tableState.status() === 'stale';
    const baseOptions = showStaleFilter
      ? [
          ...TRUENAS_PROTECTION_STATUS_OPTIONS.slice(0, 2),
          TRUENAS_PROTECTION_STALE_OPTION,
          ...TRUENAS_PROTECTION_STATUS_OPTIONS.slice(2),
        ]
      : TRUENAS_PROTECTION_STATUS_OPTIONS;
    return withPlatformStatusCounts(baseOptions, tableState.countForStatus, {
      value: 'attention',
      tone: posture().failed > 0 ? 'danger' : 'warning',
    });
  });
  const sort = createPlatformTableSortState({
    storageKey: 'truenasProtection',
    sortKeys: TRUENAS_PROTECTION_SORT_KEYS,
    descendingFirst: ['completed', 'size'],
  });
  const sortedRows = createMemo(() =>
    sort.sortRows(tableState.filtered(), (point, key) =>
      getTrueNASProtectionSortValue(point, key, staleNoticeByPointId().get(point.id)),
    ),
  );

  return (
    <Show
      when={!props.loading || rows().length > 0}
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
            description="Refresh the recovery point snapshot or check the API connection state."
            onRefresh={() => props.onRefresh?.()}
          />
        }
      >
        <TrueNASTableFrame
          hasItems={rows().length > 0}
          tableState={tableState}
          common={props}
          filteredEmptyTitle="No protection events match current filters"
          filteredEmptyDescription="Adjust the search or outcome filter to see more TrueNAS snapshots and replication tasks."
          searchPlaceholder="Search snapshots or replication"
          statusOptions={statusOptions()}
          precounted
          rowNoun="events"
          notice={
            <Show when={props.error}>
              <InlineNotice role="alert" tone="warning" aria-label="Protection refresh failed">
                <p class="font-medium">Could not refresh TrueNAS protection.</p>
                <p>
                  Showing previously loaded events. Outcomes and counts may be out of date. Newer
                  activity may be missing.
                </p>
                <Show when={props.onRefresh}>
                  <Button
                    size="sm"
                    class="mt-2"
                    disabled={props.loading}
                    aria-label="Refresh TrueNAS protection"
                    onClick={() => props.onRefresh?.()}
                  >
                    {props.loading ? 'Refreshing...' : 'Refresh'}
                  </Button>
                </Show>
              </InlineNotice>
            </Show>
          }
          title="Snapshots & Replication"
          actions={
            <span class="text-[10px] font-medium text-muted">
              {tableState.total()} event{tableState.total() === 1 ? '' : 's'}
            </span>
          }
          tableClass="min-w-full table-fixed text-xs md:min-w-[960px]"
          header={
            <>
              {/* On phones, keep identity, completion and outcome readable.
                        Artifact and target remain in the existing wrapping details. */}
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
                <PlatformResponsiveTableLabel compact="Finished" full="Completed" />
              </PlatformSortableTableHead>
              <PlatformSortableTableHead
                kind="numeric-value"
                sort={sort}
                sortKey="size"
                class="hidden lg:table-cell md:w-[8%]"
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
                  const statusBucket = () => mapTrueNASProtectionStatus(point);
                  const staleNotice = () => staleNoticeByPointId().get(point.id) ?? null;
                  const isStaleHealthy = () => Boolean(staleNotice()) && outcome() === 'success';
                  const rowVariant = (): StatusIndicatorVariant => {
                    const stale = staleNotice();
                    if (!stale) return protectionVariant(statusBucket());
                    return outcome() === 'failed' ? 'danger' : 'warning';
                  };
                  const rowDotTitle = () => {
                    const stale = staleNotice();
                    const outcomeLabel = getRecoveryOutcomeLabel(outcome());
                    if (!stale) return outcomeLabel;
                    return isStaleHealthy()
                      ? `${stale.badge} · Last reported ${outcomeLabel} · ${stale.detail}`
                      : `${outcomeLabel} · ${stale.badge} · ${stale.detail}`;
                  };
                  const effectiveOutcome = () => {
                    const stale = staleNotice();
                    if (!stale) return outcome();
                    if (outcome() === 'success') return 'warning';
                    if (outcome() === 'running') return 'unknown';
                    return outcome();
                  };
                  const detailStatus = () => {
                    const stale = staleNotice();
                    const outcomeLabel = getRecoveryOutcomeLabel(outcome());
                    if (!stale) {
                      return { variant: protectionVariant(statusBucket()), label: outcomeLabel };
                    }
                    return {
                      variant: rowVariant(),
                      label: isStaleHealthy() ? stale.badge : `${outcomeLabel} · ${stale.badge}`,
                    };
                  };
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
                        data-truenas-protection-outcome={statusBucket()}
                        data-truenas-protection-stale={staleNotice() ? 'true' : undefined}
                        onClick={() => detail.toggle(point)}
                      >
                        <TableCell class={getPlatformTableCellClassForKind('name')}>
                          <TrueNASRowLead
                            item={point}
                            drawer={detail}
                            label={datasetLabel(point)}
                            variant={rowVariant()}
                            pulse={!staleNotice() && statusBucket() === 'running'}
                            dotTitle={rowDotTitle()}
                          >
                            <div class="min-w-0">
                              <div
                                class="truncate font-medium text-base-content"
                                title={[
                                  datasetLabel(point),
                                  `${kindLabel(mapTrueNASProtectionKind(point))} on ${hostLabel(point)}`,
                                ]
                                  .filter(Boolean)
                                  .join(' · ')}
                              >
                                {datasetLabel(point)}
                              </div>
                              <TrueNASStaleBadge
                                notice={isStaleHealthy() ? null : staleNotice()}
                                attr="protection"
                              />
                            </div>
                          </TrueNASRowLead>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} platform-table-phone-hidden`}
                        >
                          <span
                            class="block truncate text-base-content"
                            title={[artifact(), artifactSecondary()].filter(Boolean).join(' · ')}
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
                          <PlatformTableDateTimeValue
                            value={parseBackupTimestampMs(completionTimestamp(point))}
                            emptyText={completionTimestamp(point) ? 'Unknown' : '—'}
                          />
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content lg:table-cell`}
                        >
                          {sizeLabel(point)}
                        </TableCell>
                        <TableCell class={getPlatformTableCellClassForKind('badge')}>
                          <span
                            class={getRecoveryOutcomeBadgeClass(effectiveOutcome())}
                            title={staleNotice() ? rowDotTitle() : undefined}
                            data-truenas-protection-stale-badge={
                              isStaleHealthy() ? 'true' : undefined
                            }
                          >
                            <Show
                              when={isStaleHealthy()}
                              fallback={getRecoveryOutcomeLabel(outcome())}
                            >
                              <PlatformResponsiveTableLabel compact="Stale" full="Stale data" />
                            </Show>
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
                          <InlineDetailPanel
                            testId="truenas-protection-detail"
                            detailFor={point.id}
                            title={datasetLabel(point)}
                            status={detailStatus()}
                            summary={`${kindLabel(mapTrueNASProtectionKind(point))} on ${hostLabel(point)}`}
                            actions={
                              <RecordOwnerLink
                                subject={{
                                  id: asTrimmedString(point.itemRef?.id) || datasetLabel(point),
                                  name: datasetLabel(point),
                                  type: 'truenas-dataset',
                                }}
                              />
                            }
                            sections={buildProtectionDetailSections(point, staleNotice())}
                            detailAttributes={{ 'data-truenas-protection-detail-for': point.id }}
                            onClose={() => detail.close(point)}
                          >
                            <Show when={staleNotice()}>
                              {(notice) => (
                                <InlineNotice
                                  role="status"
                                  tone="warning"
                                  data-testid="truenas-protection-stale-notice"
                                  aria-label="TrueNAS protection readings may be stale"
                                >
                                  <p class="font-medium">
                                    Readings from {notice().systemName} may be out of date.
                                  </p>
                                  <p>
                                    {notice().detail}. Showing the last reported protection event.
                                    Newer snapshots, replication runs, or failures may be missing.
                                  </p>
                                </InlineNotice>
                              )}
                            </Show>
                          </InlineDetailPanel>
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
    </Show>
  );
};

export default TrueNASProtectionTable;
