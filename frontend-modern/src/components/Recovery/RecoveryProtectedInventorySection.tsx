import { For, Show, createEffect, createMemo, createSignal } from 'solid-js';
import type { Accessor, Component } from 'solid-js';

import { EmptyState } from '@/components/shared/EmptyState';
import { FilterBar, type FilterDef } from '@/components/shared/FilterBar';
import { StatusDot } from '@/components/shared/StatusDot';
import { getSourcePlatformBadge } from '@/components/shared/sourcePlatformBadges';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { formatAbsoluteTime, formatRelativeTime } from '@/utils/format';
import type { ProtectionRollup } from '@/types/recovery';
import type { Resource } from '@/types/resource';
import {
  getRecoveryRollupInventoryStatusLabel,
  getRecoveryRollupInventoryStatusTextClass,
  getRecoveryRollupInventoryStatusVariant,
  getRecoverySpecialOutcomeTextClass,
} from '@/utils/recoveryStatusPresentation';
import {
  getRecoveryProtectedItemsEmptyState,
  getRecoveryProtectedItemsFailureState,
  getRecoveryProtectedItemsLoadingState,
} from '@/utils/recoveryEmptyStatePresentation';
import {
  getRecoveryItemTypePresentation,
  getRecoveryRollupItemTypeKey,
  normalizeRecoveryItemTypeQueryValue,
} from '@/utils/recoveryItemTypePresentation';
import {
  getRecoveryArtifactColumnLabel,
  getRecoveryRollupInventoryStatus,
  getRecoveryRollupInventoryPriority,
  getRecoveryRollupAgeTextClass,
  getRecoveryProtectedSearchPlaceholder,
  getRecoverySearchHistoryEmptyMessage,
  getRecoveryRollupTimestampMs,
  getRecoveryAllItemTypesLabel,
  getRecoveryAllPlatformsLabel,
  STALE_ISSUE_THRESHOLD_MS,
  type RecoveryRollupInventoryStatus,
} from '@/utils/recoveryTablePresentation';
import { normalizeRecoveryOutcome } from '@/utils/recoveryOutcomePresentation';
import { getRecoveryRollupPlatforms } from '@/utils/recoveryPlatformModel';
import {
  getRecoveryRollupItemLabel,
  getRecoveryRollupItemSecondaryLabel,
} from '@/utils/recoveryRecordPresentation';
import { getSourcePlatformLabel, normalizeSourcePlatformQueryValue } from '@/utils/sourcePlatforms';

type VerificationFilter = 'all' | 'verified' | 'unverified' | 'unknown';
type ProtectedStateFilter = 'all' | RecoveryRollupInventoryStatus;
type ProtectedSortCol = 'item' | 'type' | 'platform' | 'lastBackup' | 'outcome';
type SortDir = 'asc' | 'desc';

interface RecoveryRollupSummary {
  total: number;
  counts: Record<string, number>;
  stale: number;
  neverSucceeded: number;
}

interface RecoveryProtectedInventorySectionProps {
  filteredRollups: Accessor<ProtectionRollup[]>;
  itemTypeFilter: Accessor<string>;
  itemTypeOptions: Accessor<string[]>;
  isMobile: boolean;
  kioskMode: boolean;
  onSelectRollup: (rollupId: string) => void;
  protectedStateFilter: Accessor<ProtectedStateFilter>;
  platformFilter: Accessor<string>;
  platformOptions: Accessor<string[]>;
  queryFilter: Accessor<string>;
  resourcesById: Accessor<Map<string, Resource>>;
  rollups: Accessor<ProtectionRollup[]>;
  rollupsSummary: Accessor<RecoveryRollupSummary>;
  setItemTypeFilter: (value: string) => void;
  setProtectedStateFilter: (value: ProtectedStateFilter) => void;
  setPlatformFilter: (value: string) => void;
  setQueryFilter: (value: string) => void;
  setVerificationFilter: (value: VerificationFilter) => void;
  loading: Accessor<boolean>;
  error: Accessor<unknown>;
}

const availableProtectionStates = [
  'all',
  'healthy',
  'stale',
  'never-succeeded',
  'failed',
  'warning',
  'running',
  'unknown',
] as const satisfies readonly ProtectedStateFilter[];
const STALE_THRESHOLD_DAYS = Math.max(
  1,
  Math.round(STALE_ISSUE_THRESHOLD_MS / (24 * 60 * 60 * 1000)),
);

const getProtectionInsight = (
  rollup: ProtectionRollup,
  inventoryStatus: ReturnType<typeof getRecoveryRollupInventoryStatus>,
  nowMs: number,
): string => {
  const successMs = rollup.lastSuccessAt ? Date.parse(rollup.lastSuccessAt) : 0;
  const attemptMs = rollup.lastAttemptAt ? Date.parse(rollup.lastAttemptAt) : 0;

  if (inventoryStatus === 'never-succeeded') {
    return attemptMs > 0
      ? `No successful point recorded after ${formatRelativeTime(attemptMs)}; open events to inspect attempts`
      : 'No successful point recorded yet; open events to confirm the first attempt';
  }

  if (inventoryStatus === 'stale') {
    if (successMs > 0 && Number.isFinite(successMs)) {
      const ageDays = Math.max(1, Math.floor((nowMs - successMs) / (24 * 60 * 60 * 1000)));
      return `Last success is ${ageDays} days old; open events to inspect the latest point`;
    }
    return `No successful point within ${STALE_THRESHOLD_DAYS} days; open events to inspect attempts`;
  }

  if (inventoryStatus === 'failed') return 'Latest protection event failed; open event details';
  if (inventoryStatus === 'warning') return 'Latest event completed with warnings; review details';
  if (inventoryStatus === 'running')
    return 'Protection event in progress; check events for completion';
  return '';
};

const normalizeProtectedStateFilter = (value: string): ProtectedStateFilter => {
  const normalized = value.trim().toLowerCase();
  if (normalized === 'success') return 'healthy';
  return availableProtectionStates.includes(normalized as ProtectedStateFilter)
    ? (normalized as ProtectedStateFilter)
    : 'all';
};

export const RecoveryProtectedInventorySection: Component<
  RecoveryProtectedInventorySectionProps
> = (props) => {
  const [protectedSortCol, setProtectedSortCol] = createSignal<ProtectedSortCol>('outcome');
  const [protectedSortDir, setProtectedSortDir] = createSignal<SortDir>('desc');

  const toggleProtectedSort = (col: ProtectedSortCol) => {
    if (protectedSortCol() === col) {
      setProtectedSortDir((direction) => (direction === 'asc' ? 'desc' : 'asc'));
    } else {
      setProtectedSortCol(col);
      setProtectedSortDir('asc');
    }
  };

  const isMobileAccessor = createMemo(() => props.isMobile);

  const sortedRollups = createMemo<ProtectionRollup[]>(() => {
    const items = props.filteredRollups().slice();
    const sortColumn = protectedSortCol();
    const sortDirection = protectedSortDir();
    const resourceIndex = props.resourcesById();
    const multiplier = sortDirection === 'asc' ? 1 : -1;

    items.sort((left, right) => {
      switch (sortColumn) {
        case 'item': {
          const leftLabel = getRecoveryRollupItemLabel(left, resourceIndex).toLowerCase();
          const rightLabel = getRecoveryRollupItemLabel(right, resourceIndex).toLowerCase();
          return multiplier * leftLabel.localeCompare(rightLabel);
        }
        case 'type': {
          const leftType = getRecoveryItemTypePresentation(
            getRecoveryRollupItemTypeKey(left),
          )?.label.toLowerCase();
          const rightType = getRecoveryItemTypePresentation(
            getRecoveryRollupItemTypeKey(right),
          )?.label.toLowerCase();
          return multiplier * (leftType || '').localeCompare(rightType || '');
        }
        case 'platform': {
          const leftSource = getRecoveryRollupPlatforms(left)
            .map((platform) => getSourcePlatformLabel(String(platform)))
            .sort()
            .join(',');
          const rightSource = getRecoveryRollupPlatforms(right)
            .map((platform) => getSourcePlatformLabel(String(platform)))
            .sort()
            .join(',');
          return multiplier * leftSource.localeCompare(rightSource);
        }
        case 'lastBackup': {
          const leftSuccess = left.lastSuccessAt ? Date.parse(left.lastSuccessAt) : 0;
          const rightSuccess = right.lastSuccessAt ? Date.parse(right.lastSuccessAt) : 0;
          return multiplier * (leftSuccess - rightSuccess);
        }
        case 'outcome': {
          const leftPriority = getRecoveryRollupInventoryPriority(left);
          const rightPriority = getRecoveryRollupInventoryPriority(right);
          if (leftPriority !== rightPriority) {
            return multiplier * (leftPriority - rightPriority);
          }

          const leftTimestamp = getRecoveryRollupTimestampMs(left);
          const rightTimestamp = getRecoveryRollupTimestampMs(right);
          const naturalTieBreak =
            leftPriority === 4 ? leftTimestamp - rightTimestamp : rightTimestamp - leftTimestamp;
          if (naturalTieBreak !== 0) return multiplier * naturalTieBreak;

          const leftOutcome = normalizeRecoveryOutcome(left.lastOutcome);
          const rightOutcome = normalizeRecoveryOutcome(right.lastOutcome);
          const outcomeCmp = leftOutcome.localeCompare(rightOutcome);
          if (outcomeCmp !== 0) return multiplier * outcomeCmp;

          const leftLabel = getRecoveryRollupItemLabel(left, resourceIndex).toLowerCase();
          const rightLabel = getRecoveryRollupItemLabel(right, resourceIndex).toLowerCase();
          return multiplier * leftLabel.localeCompare(rightLabel);
        }
        default:
          return 0;
      }
    });

    return items;
  });

  createEffect(() => {
    props.queryFilter();
    props.platformFilter();
    props.itemTypeFilter();
    props.protectedStateFilter();
    protectedSortCol();
    protectedSortDir();
  });

  return (
    <div class="flex flex-col gap-2">
      <Show when={!props.kioskMode}>
        <FilterBar
            role="group"
            ariaLabel="Protected items controls"
            isMobile={isMobileAccessor}
            savedViewsKey="recovery-protected"
            search={{
              value: props.queryFilter,
              setValue: props.setQueryFilter,
              placeholder: getRecoveryProtectedSearchPlaceholder(),
              historyKey: STORAGE_KEYS.RECOVERY_SEARCH_HISTORY,
              emptyMessage: getRecoverySearchHistoryEmptyMessage(),
              clearOnEscape: true,
            }}
            filters={
              [
                {
                  id: 'item-type',
                  label: getRecoveryArtifactColumnLabel('type', 'Item Type'),
                  group: 'properties',
                  value: props.itemTypeFilter,
                  setValue: (value: string) =>
                    props.setItemTypeFilter(
                      normalizeRecoveryItemTypeQueryValue(value) || 'all',
                    ),
                  defaultValue: 'all',
                  options: () =>
                    props.itemTypeOptions().map((itemType) => ({
                      value: itemType,
                      label:
                        itemType === 'all'
                          ? getRecoveryAllItemTypesLabel()
                          : getRecoveryItemTypePresentation(itemType)?.label || itemType,
                    })),
                },
                {
                  id: 'platform',
                  label: 'Platform',
                  group: 'scope',
                  value: props.platformFilter,
                  setValue: (value: string) =>
                    props.setPlatformFilter(normalizeSourcePlatformQueryValue(value)),
                  defaultValue: 'all',
                  options: () =>
                    props.platformOptions().map((platform) => ({
                      value: platform,
                      label:
                        platform === 'all'
                          ? getRecoveryAllPlatformsLabel()
                          : getSourcePlatformLabel(platform),
                    })),
                },
                {
                  id: 'protected-state',
                  label: 'Protection state',
                  group: 'status',
                  value: props.protectedStateFilter,
                  setValue: (value: string) => {
                    const normalized = normalizeProtectedStateFilter(value);
                    props.setProtectedStateFilter(normalized);
                    props.setVerificationFilter('all');
                  },
                  defaultValue: 'all',
                  options: () =>
                    availableProtectionStates.map((state) => ({
                      value: state,
                      label:
                        state === 'all'
                          ? 'Any state'
                          : getRecoveryRollupInventoryStatusLabel(state),
                    })),
                },
              ] as FilterDef[]
            }
          />
      </Show>

      <TableCard>
        <TableCardHeader title="Protected items" />
        <Show when={props.loading() && props.filteredRollups().length === 0}>
          <div
            data-testid="recovery-protected-loading"
            class="animate-pulse pointer-events-none select-none"
          >
            <div class="px-4 py-3 text-sm text-muted">
              {getRecoveryProtectedItemsLoadingState().text}
            </div>
            <div class="space-y-3 px-4 py-4">
              <div class="flex items-center gap-3">
                <div class="h-4 w-32 rounded bg-surface-hover" />
                <div class="h-4 w-20 rounded bg-surface-hover" />
                <div class="h-4 w-24 rounded bg-surface-hover" />
                <div class="ml-auto h-4 w-16 rounded bg-surface-hover" />
              </div>
              <For each={[1, 2, 3, 4]}>
                {() => (
                  <div class="grid grid-cols-[minmax(0,1.5fr)_110px_110px_120px_90px] gap-3 border-t border-border-subtle pt-3">
                    <div class="space-y-2">
                      <div class="h-4 w-3/4 rounded bg-surface-hover" />
                      <div class="h-3 w-1/2 rounded bg-surface-hover" />
                    </div>
                    <div class="h-4 w-16 rounded bg-surface-hover" />
                    <div class="h-4 w-20 rounded bg-surface-hover" />
                    <div class="h-4 w-24 rounded bg-surface-hover" />
                    <div class="h-6 w-16 rounded bg-surface-hover" />
                  </div>
                )}
              </For>
            </div>
          </div>
        </Show>

        <Show when={!props.loading() && props.error()}>
          <div class="p-6">
            <EmptyState
              title={getRecoveryProtectedItemsFailureState().title}
              description={String((props.error() as Error)?.message || props.error())}
            />
          </div>
        </Show>

        <Show when={!props.loading() && !props.error() && props.filteredRollups().length === 0}>
          <div class="p-6">
            <EmptyState {...getRecoveryProtectedItemsEmptyState()} />
          </div>
        </Show>

        <Show when={props.filteredRollups().length > 0}>
          <Table
            wrapperClass="bg-surface"
            class={`w-full border-collapse whitespace-nowrap table-fixed ${
              props.isMobile ? 'min-w-full' : 'min-w-[640px]'
            }`}
          >
            <TableHeader>
              <TableRow class="bg-surface-alt/95 text-muted">
                {(
                  [
                    ['item', getRecoveryArtifactColumnLabel('item', 'Item')],
                    ['type', getRecoveryArtifactColumnLabel('type', 'Item Type')],
                    ['platform', getRecoveryArtifactColumnLabel('platform', 'Platform')],
                    ['lastBackup', 'Latest Point'],
                    ['outcome', 'Status'],
                  ] as const
                ).map(([column, label]) => (
                  <TableHead
                    class={`sticky top-0 z-[1] bg-surface-alt/95 px-3 py-2 whitespace-nowrap text-left text-[11px] font-medium cursor-pointer select-none hover:text-base-content transition-colors${
                      column === 'type'
                        ? ' hidden md:table-cell w-[96px]'
                        : column === 'platform'
                          ? ' hidden lg:table-cell w-[110px]'
                          : column === 'lastBackup'
                            ? ' w-[120px]'
                            : column === 'outcome'
                              ? ' w-[116px]'
                              : ''
                    }`}
                    onClick={() => toggleProtectedSort(column)}
                  >
                    <span class="inline-flex items-center gap-1">
                      {label}
                      <Show when={protectedSortCol() === column}>
                        <svg class="h-3 w-3" viewBox="0 0 12 12" fill="currentColor">
                          {protectedSortDir() === 'asc' ? (
                            <path d="M6 3l3.5 5h-7z" />
                          ) : (
                            <path d="M6 9l3.5-5h-7z" />
                          )}
                        </svg>
                      </Show>
                    </span>
                  </TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              <For each={sortedRollups()}>
                {(rollup) => {
                  const resourceIndex = props.resourcesById();
                  const label = getRecoveryRollupItemLabel(rollup, resourceIndex);
                  const secondaryLabel = getRecoveryRollupItemSecondaryLabel(rollup);
                  const attemptMs = rollup.lastAttemptAt ? Date.parse(rollup.lastAttemptAt) : 0;
                  const successMs = rollup.lastSuccessAt ? Date.parse(rollup.lastSuccessAt) : 0;
                  const inventoryStatus = getRecoveryRollupInventoryStatus(rollup);
                  const inventoryStatusLabel =
                    getRecoveryRollupInventoryStatusLabel(inventoryStatus);
                  const platforms = getRecoveryRollupPlatforms(rollup)
                    .map((platform) => String(platform || '').trim())
                    .filter(Boolean)
                    .sort((left, right) =>
                      getSourcePlatformLabel(left).localeCompare(getSourcePlatformLabel(right)),
                    );
                  const itemTypePresentation =
                    getRecoveryItemTypePresentation(getRecoveryRollupItemTypeKey(rollup)) || null;
                  const nowMs = Date.now();
                  const neverSucceeded = inventoryStatus === 'never-succeeded';
                  const protectionInsight = getProtectionInsight(rollup, inventoryStatus, nowMs);

                  return (
                    <TableRow
                      class="cursor-pointer odd:bg-surface even:bg-surface-alt/35 transition-colors hover:bg-surface-hover/95"
                      onClick={() => props.onSelectRollup(rollup.rollupId)}
                    >
                      <TableCell class="max-w-[420px] px-3 py-1.5 text-base-content" title={label}>
                        <div class="flex min-w-0 flex-col gap-1">
                          <div class="flex min-w-0 items-center gap-2">
                            <StatusDot
                              variant={getRecoveryRollupInventoryStatusVariant(inventoryStatus)}
                              size="xs"
                              pulse={inventoryStatus === 'running'}
                              title={inventoryStatusLabel}
                              ariaLabel={inventoryStatusLabel}
                            />
                            <div class="flex min-w-0 items-baseline gap-1.5">
                              <span class="truncate text-[13px] font-medium">{label}</span>
                              <Show when={secondaryLabel}>
                                <span class="shrink-0 text-[10px] font-mono tabular-nums text-muted">
                                  {secondaryLabel}
                                </span>
                              </Show>
                            </div>
                          </div>
                          <Show when={protectionInsight}>
                            <div class="pl-4 text-[10px] leading-4 text-muted">
                              {protectionInsight}
                            </div>
                          </Show>
                          <div class="flex flex-wrap items-center gap-1.5 text-[10px] md:hidden">
                            <Show when={itemTypePresentation?.label}>
                              <span class={itemTypePresentation?.tableBadgeClasses}>
                                {itemTypePresentation?.label}
                              </span>
                            </Show>
                            <Show when={platforms.length > 0}>
                              <For each={platforms.slice(0, 2)}>
                                {(platform) => {
                                  const badge = getSourcePlatformBadge(platform);
                                  return (
                                    <span class={`${badge?.classes || ''} lg:hidden`}>
                                      {badge?.label || getSourcePlatformLabel(platform)}
                                    </span>
                                  );
                                }}
                              </For>
                            </Show>
                          </div>
                        </div>
                      </TableCell>

                      <TableCell class="hidden md:table-cell whitespace-nowrap px-3 py-1.5">
                        <Show
                          when={itemTypePresentation}
                          fallback={<span class="text-muted">—</span>}
                        >
                          <span class={itemTypePresentation?.tableBadgeClasses}>
                            {itemTypePresentation?.label}
                          </span>
                        </Show>
                      </TableCell>

                      <TableCell class="hidden lg:table-cell whitespace-nowrap px-3 py-1.5">
                        <div class="flex flex-wrap gap-1.5">
                          <For each={platforms}>
                            {(platform) => {
                              const badge = getSourcePlatformBadge(platform);
                              return (
                                <span class={badge?.classes || ''}>
                                  {badge?.label || getSourcePlatformLabel(platform)}
                                </span>
                              );
                            }}
                          </For>
                        </div>
                      </TableCell>

                      <TableCell
                        class={`whitespace-nowrap px-3 py-1.5 ${getRecoveryRollupAgeTextClass(
                          rollup,
                          nowMs,
                        )}`}
                        title={
                          successMs > 0
                            ? formatAbsoluteTime(successMs)
                            : attemptMs > 0
                              ? formatAbsoluteTime(attemptMs)
                              : undefined
                        }
                      >
                        <div class="flex flex-col">
                          <span>
                            {successMs > 0 ? (
                              formatRelativeTime(successMs)
                            ) : neverSucceeded ? (
                              <span class={getRecoverySpecialOutcomeTextClass('never')}>never</span>
                            ) : (
                              '—'
                            )}
                          </span>
                          <Show when={inventoryStatus !== 'healthy' && attemptMs > 0}>
                            <span class="text-[10px] font-normal text-muted">
                              Attempt {formatRelativeTime(attemptMs)}
                            </span>
                          </Show>
                        </div>
                      </TableCell>

                      <TableCell class="whitespace-nowrap px-3 py-1.5">
                        <span
                          class={`text-[11px] font-medium ${getRecoveryRollupInventoryStatusTextClass(
                            inventoryStatus,
                          )}`}
                        >
                          {inventoryStatusLabel}
                        </span>
                      </TableCell>
                    </TableRow>
                  );
                }}
              </For>
            </TableBody>
          </Table>
          <div class="flex items-center justify-between gap-2 border-t border-border bg-surface px-4 py-3 text-xs text-muted">
            <div>
              <Show
                when={sortedRollups().length > 0}
                fallback={<span>Showing 0 of 0 protected items</span>}
              >
                <span>Showing {sortedRollups().length} protected items</span>
              </Show>
            </div>
          </div>
        </Show>
      </TableCard>
    </div>
  );
};
