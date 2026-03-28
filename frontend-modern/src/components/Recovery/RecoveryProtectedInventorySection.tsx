import { For, Show, createEffect, createMemo, createSignal } from 'solid-js';
import type { Accessor, Component, JSX } from 'solid-js';

import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { LabeledFilterSelect } from '@/components/shared/FilterToolbar';
import { PageControls } from '@/components/shared/PageControls';
import { SearchInput } from '@/components/shared/SearchInput';
import { StatusDot } from '@/components/shared/StatusDot';
import { getSourcePlatformBadge } from '@/components/shared/sourcePlatformBadges';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/shared/Table';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { formatAbsoluteTime, formatRelativeTime } from '@/utils/format';
import type { ProtectionRollup, RecoveryOutcome } from '@/types/recovery';
import type { Resource } from '@/types/resource';
import {
  getRecoveryProtectedToggleClass,
  getRecoveryRollupStatusPillClass,
  getRecoveryRollupStatusPillLabel,
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
  getRecoveryRollupAgeTextClass,
  getRecoveryProtectedSearchPlaceholder,
  getRecoverySearchHistoryEmptyMessage,
  isRecoveryRollupStale,
} from '@/utils/recoveryTablePresentation';
import {
  getRecoveryOutcomeBadgeClass,
  getRecoveryOutcomeLabel,
  normalizeRecoveryOutcome,
} from '@/utils/recoveryOutcomePresentation';
import { getRecoveryRollupPlatforms } from '@/utils/recoveryPlatformModel';
import { getRecoveryRollupItemLabel } from '@/utils/recoveryRecordPresentation';
import { getSourcePlatformLabel, normalizeSourcePlatformQueryValue } from '@/utils/sourcePlatforms';
import { titleCaseDelimitedLabel } from '@/utils/textPresentation';

type VerificationFilter = 'all' | 'verified' | 'unverified' | 'unknown';
type ProtectedSortCol = 'item' | 'type' | 'platform' | 'lastBackup' | 'outcome';
type SortDir = 'asc' | 'desc';

interface RecoveryRollupSummary {
  total: number;
  counts: Record<RecoveryOutcome, number>;
  stale: number;
  neverSucceeded: number;
}

interface RecoveryProtectedInventorySectionProps {
  filteredRollups: Accessor<ProtectionRollup[]>;
  historyOutcomeFilter: Accessor<'all' | RecoveryOutcome>;
  itemTypeFilter: Accessor<string>;
  itemTypeOptions: Accessor<string[]>;
  isMobile: boolean;
  kioskMode: boolean;
  onSelectRollup: (rollupId: string) => void;
  protectedStaleOnly: Accessor<boolean>;
  platformFilter: Accessor<string>;
  platformOptions: Accessor<string[]>;
  queryFilter: Accessor<string>;
  resourcesById: Accessor<Map<string, Resource>>;
  rollups: Accessor<ProtectionRollup[]>;
  rollupsSummary: Accessor<RecoveryRollupSummary>;
  setHistoryOutcomeFilter: (value: 'all' | RecoveryOutcome) => void;
  setItemTypeFilter: (value: string) => void;
  setProtectedStaleOnly: (value: boolean | ((prev: boolean) => boolean)) => void;
  setPlatformFilter: (value: string) => void;
  setQueryFilter: (value: string) => void;
  setVerificationFilter: (value: VerificationFilter) => void;
  loading: Accessor<boolean>;
  error: Accessor<unknown>;
  workspaceControls?: JSX.Element;
}

const availableOutcomes = ['all', 'success', 'warning', 'failed', 'running'] as const;
const PROTECTED_ITEMS_PAGE_SIZE = 24;

export const RecoveryProtectedInventorySection: Component<
  RecoveryProtectedInventorySectionProps
> = (props) => {
  const [protectedFiltersOpen, setProtectedFiltersOpen] = createSignal(false);
  const [protectedPage, setProtectedPage] = createSignal(1);
  const [protectedSortCol, setProtectedSortCol] = createSignal<ProtectedSortCol>('lastBackup');
  const [protectedSortDir, setProtectedSortDir] = createSignal<SortDir>('desc');

  const toggleProtectedSort = (col: ProtectedSortCol) => {
    if (protectedSortCol() === col) {
      setProtectedSortDir((direction) => (direction === 'asc' ? 'desc' : 'asc'));
    } else {
      setProtectedSortCol(col);
      setProtectedSortDir('asc');
    }
  };

  const protectedActiveFilterCount = createMemo(() => {
    let count = 0;
    if (props.queryFilter().trim() !== '') count += 1;
    if (props.platformFilter() !== 'all') count += 1;
    if (props.itemTypeFilter() !== 'all') count += 1;
    if (props.historyOutcomeFilter() !== 'all') count += 1;
    if (props.protectedStaleOnly()) count += 1;
    return count;
  });

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
          const leftType = getRecoveryItemTypePresentation(getRecoveryRollupItemTypeKey(left))?.label.toLowerCase();
          const rightType = getRecoveryItemTypePresentation(getRecoveryRollupItemTypeKey(right))?.label.toLowerCase();
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
          const leftOutcome = normalizeRecoveryOutcome(left.lastOutcome);
          const rightOutcome = normalizeRecoveryOutcome(right.lastOutcome);
          return multiplier * leftOutcome.localeCompare(rightOutcome);
        }
        default:
          return 0;
      }
    });

    return items;
  });

  const protectedTotalPages = createMemo(() =>
    Math.max(1, Math.ceil(sortedRollups().length / PROTECTED_ITEMS_PAGE_SIZE)),
  );

  const visibleRollups = createMemo<ProtectionRollup[]>(() => {
    const start = (protectedPage() - 1) * PROTECTED_ITEMS_PAGE_SIZE;
    return sortedRollups().slice(start, start + PROTECTED_ITEMS_PAGE_SIZE);
  });

  const pageStart = createMemo(() =>
    sortedRollups().length === 0 ? 0 : (protectedPage() - 1) * PROTECTED_ITEMS_PAGE_SIZE + 1,
  );

  const pageEnd = createMemo(() =>
    Math.min(protectedPage() * PROTECTED_ITEMS_PAGE_SIZE, sortedRollups().length),
  );

  createEffect(() => {
    props.queryFilter();
    props.platformFilter();
    props.itemTypeFilter();
    props.historyOutcomeFilter();
    props.protectedStaleOnly();
    protectedSortCol();
    protectedSortDir();
    setProtectedPage(1);
  });

  createEffect(() => {
    const totalPages = protectedTotalPages();
    if (protectedPage() > totalPages) {
      setProtectedPage(totalPages);
    }
  });

  const resetProtectedFilters = () => {
    props.setQueryFilter('');
    props.setPlatformFilter('all');
    props.setItemTypeFilter('all');
    props.setHistoryOutcomeFilter('all');
    props.setVerificationFilter('all');
    props.setProtectedStaleOnly(false);
  };

  return (
    <Card
      padding="none"
      tone="card"
      class="overflow-hidden border-border-subtle bg-surface"
    >
      <Show when={props.workspaceControls}>{props.workspaceControls}</Show>

      <Show when={!props.kioskMode}>
        <div class="border-b border-border-subtle px-4 py-3 sm:px-5">
          <PageControls
            role="group"
            aria-label="Protected items controls"
            search={
              <SearchInput
                value={props.queryFilter}
                onChange={(value) => props.setQueryFilter(value)}
                placeholder={getRecoveryProtectedSearchPlaceholder()}
                inputClass="py-1.5 text-sm"
                clearOnEscape
                history={{
                  storageKey: STORAGE_KEYS.RECOVERY_SEARCH_HISTORY,
                  emptyMessage: getRecoverySearchHistoryEmptyMessage(),
                }}
              />
            }
            mobileFilters={{
              enabled: props.isMobile,
              onToggle: () => setProtectedFiltersOpen((open) => !open),
              count: protectedActiveFilterCount(),
            }}
            resetAction={{
              show: protectedActiveFilterCount() > 0,
              onClick: resetProtectedFilters,
              label: 'Reset all',
              title: 'Reset protected item filters',
            }}
            showFilters={!props.isMobile || protectedFiltersOpen()}
            toolbarClass="gap-3 lg:flex-nowrap"
          >
            <LabeledFilterSelect
              id="recovery-item-type-filter"
              label="Item Type"
              value={props.itemTypeFilter()}
              onChange={(event) =>
                props.setItemTypeFilter(
                  normalizeRecoveryItemTypeQueryValue(event.currentTarget.value) || 'all',
                )
              }
              groupClass="gap-1.5 px-1.5 py-0.5"
              selectClass="py-1 text-xs"
            >
              <For each={props.itemTypeOptions()}>
                {(itemType) => (
                  <option value={itemType}>
                    {itemType === 'all'
                      ? 'All Item Types'
                      : getRecoveryItemTypePresentation(itemType)?.label || itemType}
                  </option>
                )}
              </For>
            </LabeledFilterSelect>

            <LabeledFilterSelect
              id="recovery-platform-filter"
              label="Platform"
              value={props.platformFilter()}
              onChange={(event) =>
                props.setPlatformFilter(
                  normalizeSourcePlatformQueryValue(event.currentTarget.value),
                )
              }
              groupClass="gap-1.5 px-1.5 py-0.5"
              selectClass="py-1 text-xs"
            >
              <For each={props.platformOptions()}>
                {(platform) => (
                  <option value={platform}>
                    {platform === 'all' ? 'All Platforms' : getSourcePlatformLabel(platform)}
                  </option>
                )}
              </For>
            </LabeledFilterSelect>

            <LabeledFilterSelect
              id="recovery-protected-status-filter"
              label="Latest status"
              value={props.historyOutcomeFilter()}
              onChange={(event) => {
                const value = event.currentTarget.value as 'all' | RecoveryOutcome;
                props.setHistoryOutcomeFilter(value);
                if (value !== 'all') props.setVerificationFilter('all');
              }}
              groupClass="gap-1.5 px-1.5 py-0.5"
              selectClass="py-1 text-xs"
            >
              <For each={availableOutcomes}>
                {(outcome) => (
                  <option value={outcome}>
                    {outcome === 'all' ? 'Any status' : titleCaseDelimitedLabel(outcome)}
                  </option>
                )}
              </For>
            </LabeledFilterSelect>

            <button
              type="button"
              aria-pressed={props.protectedStaleOnly()}
              onClick={() => props.setProtectedStaleOnly((value) => !value)}
              class={`rounded-md border px-2.5 py-1 text-xs font-medium transition-colors ${getRecoveryProtectedToggleClass(
                props.protectedStaleOnly(),
              )}`}
            >
              Stale only
            </button>
          </PageControls>
        </div>
      </Show>

      <Show when={props.loading() && props.filteredRollups().length === 0}>
        <div class="px-6 py-6 text-sm text-muted">
          {getRecoveryProtectedItemsLoadingState().text}
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
        <div class="overflow-x-auto bg-surface">
          <Table
            class="w-full border-collapse whitespace-nowrap"
            style={{ 'table-layout': 'fixed', 'min-width': props.isMobile ? '100%' : '640px' }}
          >
            <TableHeader>
              <TableRow class="bg-surface-alt/95 text-muted">
                {(
                  [
                    ['item', getRecoveryArtifactColumnLabel('item', 'Item')],
                    ['type', 'Item Type'],
                    ['platform', getRecoveryArtifactColumnLabel('platform', 'Platform')],
                    ['lastBackup', 'Latest Point'],
                    ['outcome', 'Outcome'],
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
                            ? ' w-[70px]'
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
              <For each={visibleRollups()}>
                {(rollup) => {
                  const resourceIndex = props.resourcesById();
                  const label = getRecoveryRollupItemLabel(rollup, resourceIndex);
                  const attemptMs = rollup.lastAttemptAt ? Date.parse(rollup.lastAttemptAt) : 0;
                  const successMs = rollup.lastSuccessAt ? Date.parse(rollup.lastSuccessAt) : 0;
                  const outcome = normalizeRecoveryOutcome(rollup.lastOutcome);
                  const platforms = getRecoveryRollupPlatforms(rollup)
                    .map((platform) => String(platform || '').trim())
                    .filter(Boolean)
                    .sort((left, right) =>
                      getSourcePlatformLabel(left).localeCompare(getSourcePlatformLabel(right)),
                    );
                  const itemTypePresentation =
                    getRecoveryItemTypePresentation(getRecoveryRollupItemTypeKey(rollup)) || null;
                  const nowMs = Date.now();
                  const stale = isRecoveryRollupStale(rollup, nowMs);
                  const neverSucceeded =
                    (!Number.isFinite(successMs) || successMs <= 0) &&
                    Number.isFinite(attemptMs) &&
                    attemptMs > 0;
                  const outcomeVariant = (() => {
                    switch (outcome) {
                      case 'success':
                        return 'success' as const;
                      case 'warning':
                        return 'warning' as const;
                      case 'failed':
                        return 'danger' as const;
                      default:
                        return 'muted' as const;
                    }
                  })();

                  return (
                    <TableRow
                      class="cursor-pointer odd:bg-surface even:bg-surface-alt/35 transition-colors hover:bg-surface-hover/95"
                      onClick={() => props.onSelectRollup(rollup.rollupId)}
                    >
                      <TableCell
                        class="max-w-[420px] px-3 py-1.5 text-base-content"
                        title={label}
                      >
                        <div class="flex min-w-0 flex-col gap-1">
                          <div class="flex min-w-0 items-center gap-2">
                            <StatusDot
                              variant={outcomeVariant}
                              size="xs"
                              pulse={outcome === 'running'}
                              title={getRecoveryOutcomeLabel(outcome)}
                              ariaLabel={getRecoveryOutcomeLabel(outcome)}
                            />
                            <span class="truncate text-[13px] font-medium">{label}</span>
                            <Show when={neverSucceeded}>
                              <span class={getRecoveryRollupStatusPillClass('never-succeeded')}>
                                {getRecoveryRollupStatusPillLabel('never-succeeded')}
                              </span>
                            </Show>
                            <Show when={!neverSucceeded && stale}>
                              <span class={getRecoveryRollupStatusPillClass('stale')}>
                                {getRecoveryRollupStatusPillLabel('stale')}
                              </span>
                            </Show>
                          </div>
                          <div class="flex flex-wrap items-center gap-1.5 text-[10px] md:hidden">
                            <Show when={itemTypePresentation?.label}>
                              <span class={itemTypePresentation?.badgeClasses || 'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-surface-alt text-base-content'}>
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
                          <span class={itemTypePresentation?.badgeClasses || 'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-surface-alt text-base-content'}>
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
                        {successMs > 0 ? (
                          formatRelativeTime(successMs)
                        ) : neverSucceeded ? (
                          <span class={getRecoverySpecialOutcomeTextClass('never')}>never</span>
                        ) : (
                          '—'
                        )}
                      </TableCell>

                      <TableCell class="whitespace-nowrap px-3 py-1.5">
                        <span
                          class={`inline-flex rounded px-1.5 py-0.5 text-[10px] font-medium ${getRecoveryOutcomeBadgeClass(
                            outcome,
                          )}`}
                        >
                          {titleCaseDelimitedLabel(outcome)}
                        </span>
                      </TableCell>
                    </TableRow>
                  );
                }}
              </For>
            </TableBody>
          </Table>
        </div>
        <div class="flex items-center justify-between gap-2 border-t border-border bg-surface px-4 py-3 text-xs text-muted">
          <div>
            <Show
              when={sortedRollups().length > 0}
              fallback={<span>Showing 0 of 0 protected items</span>}
            >
              <span>
                Showing {pageStart()} - {pageEnd()} of {sortedRollups().length} protected items
              </span>
            </Show>
          </div>
          <div class="flex items-center gap-2">
            <button
              type="button"
              disabled={protectedPage() <= 1}
              onClick={() => setProtectedPage(Math.max(1, protectedPage() - 1))}
              class="rounded-md border border-border bg-surface px-2 py-1 text-xs font-medium text-base-content disabled:opacity-50"
            >
              Prev
            </button>
            <span>
              Page {protectedPage()} / {protectedTotalPages()}
            </span>
            <button
              type="button"
              disabled={protectedPage() >= protectedTotalPages()}
              onClick={() =>
                setProtectedPage(Math.min(protectedTotalPages(), protectedPage() + 1))
              }
              class="rounded-md border border-border bg-surface px-2 py-1 text-xs font-medium text-base-content disabled:opacity-50"
            >
              Next
            </button>
          </div>
        </div>
      </Show>
    </Card>
  );
};
