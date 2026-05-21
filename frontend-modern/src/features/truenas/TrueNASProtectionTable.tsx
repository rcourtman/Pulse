import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { formatBytes } from '@/utils/format';
import {
  getRecoveryOutcomeBadgeClass,
  getRecoveryOutcomeLabel,
  normalizeRecoveryOutcome,
} from '@/utils/recoveryOutcomePresentation';
import { asTrimmedString } from '@/utils/stringUtils';
import type { StatusIndicatorVariant } from '@/utils/status';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PlatformErrorState,
  PlatformTableEmptyState,
  PlatformTableLoadingState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  type PlatformTableFilterOption,
} from '@/features/platformPage/sharedPlatformPage';
import type { RecoveryPoint } from '@/types/recovery';
import {
  filterTrueNASProtectionPoints,
  mapTrueNASProtectionKind,
  mapTrueNASProtectionStatus,
  sortTrueNASProtectionPoints,
  type TrueNASProtectionKind,
  type TrueNASProtectionStatusFilter,
} from './truenasPageModel';

const TRUENAS_PROTECTION_STATUS_OPTIONS: PlatformTableFilterOption<TrueNASProtectionStatusFilter>[] =
  [
    { value: 'all', label: 'All' },
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

const protectionVariant = (
  status: Exclude<TrueNASProtectionStatusFilter, 'all'>,
): StatusIndicatorVariant => {
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
  asTrimmedString(point.display?.subjectLabel) ||
  asTrimmedString(point.itemRef?.name) ||
  asTrimmedString(point.subjectRef?.name) ||
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

const signalLabel = (point: RecoveryPoint): string => {
  const lastError = detailString(point, 'lastError');
  if (lastError) return lastError;
  const state = detailString(point, 'lastState');
  if (state) return state;
  return getRecoveryOutcomeLabel(normalizeRecoveryOutcome(point.outcome));
};

const formatPointTime = (point: RecoveryPoint): string => {
  const raw = asTrimmedString(point.completedAt) || asTrimmedString(point.startedAt);
  if (!raw) return '-';
  const parsed = new Date(raw);
  if (Number.isNaN(parsed.getTime())) return '-';
  return parsed.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
};

const sizeLabel = (point: RecoveryPoint): string =>
  typeof point.sizeBytes === 'number' && point.sizeBytes > 0 ? formatBytes(point.sizeBytes) : '-';

const DatasetCell: Component<{ point: RecoveryPoint }> = (props) => {
  const status = () => mapTrueNASProtectionStatus(props.point);
  const kind = () => mapTrueNASProtectionKind(props.point);
  const name = () => datasetLabel(props.point);
  const subtitle = () => `${kindLabel(kind())} on ${hostLabel(props.point)}`;

  return (
    <div class="flex min-w-0 items-center gap-2">
      <StatusDot
        size="sm"
        variant={protectionVariant(status())}
        pulse={status() === 'running'}
        title={getRecoveryOutcomeLabel(normalizeRecoveryOutcome(props.point.outcome))}
      />
      <div class="min-w-0">
        <div class="truncate font-medium text-base-content" title={name()}>
          {name()}
        </div>
        <div class="truncate text-[10px] text-muted" title={subtitle()}>
          {subtitle()}
        </div>
      </div>
    </div>
  );
};

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
  const tableState = createPlatformTableFilterState({
    resources: rows,
    initialStatus: 'all' as TrueNASProtectionStatusFilter,
    filter: filterTrueNASProtectionPoints,
  });

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
            <Show when={props.showToolbar !== false}>
              <PlatformTableToolbar
                search={tableState.search}
                onSearchChange={tableState.setSearch}
                searchPlaceholder="Search snapshots or replication"
                status={tableState.status()}
                onStatusChange={tableState.setStatus}
                statusOptions={TRUENAS_PROTECTION_STATUS_OPTIONS}
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
              <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
                <TableCardHeader
                  title="Snapshots & Replication"
                  actions={
                    <span class="text-[10px] font-medium text-muted">
                      {tableState.total()} event{tableState.total() === 1 ? '' : 's'}
                    </span>
                  }
                />
                <Table class="min-w-full table-fixed text-xs md:min-w-[1180px]">
                  <TableHeader>
                    <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                      <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[21%]`}>
                        Dataset
                      </TableHead>
                      <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[24%]`}>
                        Artifact
                      </TableHead>
                      <TableHead
                        class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[17%]`}
                      >
                        Target
                      </TableHead>
                      <TableHead
                        class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[12%]`}
                      >
                        Completed
                      </TableHead>
                      <TableHead
                        class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden lg:table-cell md:w-[8%]`}
                      >
                        Size
                      </TableHead>
                      <TableHead
                        class={`${getPlatformTableHeadClassForKind('text')} hidden xl:table-cell md:w-[9%]`}
                      >
                        Signal
                      </TableHead>
                      <TableHead class={`${getPlatformTableHeadClassForKind('badge')} md:w-[9%]`}>
                        Outcome
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                    <For each={tableState.filtered()}>
                      {(point) => {
                        const outcome = () => normalizeRecoveryOutcome(point.outcome);
                        const artifact = () => artifactLabel(point);
                        const artifactSecondary = () => artifactSecondaryLabel(point);
                        const target = () => targetLabel(point);
                        const targetSecondary = () => targetSecondaryLabel(point);
                        const signal = () => signalLabel(point);
                        return (
                          <TableRow
                            class="text-[11px] sm:text-xs"
                            data-truenas-protection-row={point.id}
                            data-truenas-protection-kind={mapTrueNASProtectionKind(point)}
                            data-truenas-protection-outcome={mapTrueNASProtectionStatus(point)}
                          >
                            <TableCell class={getPlatformTableCellClassForKind('name')}>
                              <DatasetCell point={point} />
                            </TableCell>
                            <TableCell class={getPlatformTableCellClassForKind('text')}>
                              <span class="block truncate text-base-content" title={artifact()}>
                                {artifact()}
                              </span>
                              <Show when={artifactSecondary()}>
                                <span
                                  class="block truncate text-[10px] text-muted"
                                  title={artifactSecondary()}
                                >
                                  {artifactSecondary()}
                                </span>
                              </Show>
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} hidden md:table-cell`}
                            >
                              <span class="block truncate text-base-content" title={target()}>
                                {target()}
                              </span>
                              <Show when={targetSecondary()}>
                                <span
                                  class="block truncate text-[10px] text-muted"
                                  title={targetSecondary()}
                                >
                                  {targetSecondary()}
                                </span>
                              </Show>
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                            >
                              {formatPointTime(point)}
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums lg:table-cell`}
                            >
                              {sizeLabel(point)}
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} hidden xl:table-cell`}
                            >
                              <span class="block truncate text-muted" title={signal()}>
                                {signal()}
                              </span>
                            </TableCell>
                            <TableCell class={getPlatformTableCellClassForKind('badge')}>
                              <span class={getRecoveryOutcomeBadgeClass(outcome())}>
                                {getRecoveryOutcomeLabel(outcome())}
                              </span>
                            </TableCell>
                          </TableRow>
                        );
                      }}
                    </For>
                  </TableBody>
                </Table>
              </TableCard>
            </Show>
          </div>
        </Show>
      </Show>
    </Show>
  );
};

export default TrueNASProtectionTable;
