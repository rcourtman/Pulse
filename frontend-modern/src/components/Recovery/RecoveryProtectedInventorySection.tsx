import { For, Show, createMemo, createSignal } from 'solid-js';
import type { Accessor, Component } from 'solid-js';

import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { LabeledFilterSelect } from '@/components/shared/FilterToolbar';
import { PageControls } from '@/components/shared/PageControls';
import { SearchInput } from '@/components/shared/SearchInput';
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
  normalizeRecoveryItemTypeQueryValue,
} from '@/utils/recoveryItemTypePresentation';
import {
  getRecoveryArtifactColumnLabel,
  getRecoveryRollupAgeTextClass,
  getRecoveryRollupIssueTone,
  getRecoveryProtectedSearchPlaceholder,
  getRecoverySearchHistoryEmptyMessage,
  isRecoveryRollupStale,
} from '@/utils/recoveryTablePresentation';
import {
  getRecoveryOutcomeBadgeClass,
  normalizeRecoveryOutcome,
} from '@/utils/recoveryOutcomePresentation';
import { getRecoveryIssueRailClass, type RecoveryIssueTone } from '@/utils/recoveryIssuePresentation';
import { getRecoveryRollupSubjectLabel } from '@/utils/recoveryRecordPresentation';
import { getSourcePlatformLabel, normalizeSourcePlatformQueryValue } from '@/utils/sourcePlatforms';
import { titleCaseDelimitedLabel } from '@/utils/textPresentation';

type VerificationFilter = 'all' | 'verified' | 'unverified' | 'unknown';
type ProtectedSortCol = 'subject' | 'source' | 'lastBackup' | 'outcome';
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
  providerFilter: Accessor<string>;
  providerOptions: Accessor<string[]>;
  queryFilter: Accessor<string>;
  resourcesById: Accessor<Map<string, Resource>>;
  rollups: Accessor<ProtectionRollup[]>;
  rollupsSummary: Accessor<RecoveryRollupSummary>;
  setHistoryOutcomeFilter: (value: 'all' | RecoveryOutcome) => void;
  setItemTypeFilter: (value: string) => void;
  setProtectedStaleOnly: (value: boolean | ((prev: boolean) => boolean)) => void;
  setProviderFilter: (value: string) => void;
  setQueryFilter: (value: string) => void;
  setVerificationFilter: (value: VerificationFilter) => void;
  loading: Accessor<boolean>;
  error: Accessor<unknown>;
}

const availableOutcomes = ['all', 'success', 'warning', 'failed', 'running'] as const;

export const RecoveryProtectedInventorySection: Component<
  RecoveryProtectedInventorySectionProps
> = (props) => {
  const [protectedFiltersOpen, setProtectedFiltersOpen] = createSignal(false);
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
    if (props.providerFilter() !== 'all') count += 1;
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
        case 'subject': {
          const leftLabel = getRecoveryRollupSubjectLabel(left, resourceIndex).toLowerCase();
          const rightLabel = getRecoveryRollupSubjectLabel(right, resourceIndex).toLowerCase();
          return multiplier * leftLabel.localeCompare(rightLabel);
        }
        case 'source': {
          const leftSource = (left.providers || [])
            .map((provider) => getSourcePlatformLabel(String(provider)))
            .sort()
            .join(',');
          const rightSource = (right.providers || [])
            .map((provider) => getSourcePlatformLabel(String(provider)))
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

  return (
    <Card padding="none" tone="card" class="order-3 overflow-hidden">
      <div class="border-b border-border bg-surface-hover px-3 py-2">
        <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div class="flex flex-col gap-1">
            <div class="text-[11px] font-semibold uppercase tracking-wide text-muted">
              Protected Items
            </div>
            <div class="text-xs text-muted">
              Unified protection inventory across workloads, datasets, and other protected items from every connected platform.
            </div>
          </div>
          <div class="flex flex-wrap items-center gap-2 text-xs text-muted">
            <span>
              {props.filteredRollups().length} of {props.rollups().length} items shown
            </span>
            <Show when={props.rollupsSummary().stale > 0}>
              <span class={getRecoveryRollupStatusPillClass('stale')}>
                {props.rollupsSummary().stale} stale
              </span>
            </Show>
            <Show when={props.rollupsSummary().neverSucceeded > 0}>
              <span class={getRecoveryRollupStatusPillClass('never-succeeded')}>
                {props.rollupsSummary().neverSucceeded} never succeeded
              </span>
            </Show>
          </div>
        </div>
      </div>

      <Show when={!props.kioskMode}>
        <div class="border-b border-border px-3 py-3">
          <PageControls
            search={
              <SearchInput
                value={props.queryFilter}
                onChange={(value) => props.setQueryFilter(value)}
                placeholder={getRecoveryProtectedSearchPlaceholder()}
                class="w-full"
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
            showFilters={!props.isMobile || protectedFiltersOpen()}
            toolbarClass="lg:flex-nowrap"
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
              selectClass="min-w-[10rem] max-w-[14rem]"
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
              id="recovery-provider-filter"
              label="Platform"
              value={props.providerFilter()}
              onChange={(event) =>
                props.setProviderFilter(
                  normalizeSourcePlatformQueryValue(event.currentTarget.value),
                )
              }
              selectClass="min-w-[10rem] max-w-[14rem]"
            >
              <For each={props.providerOptions()}>
                {(provider) => (
                  <option value={provider}>
                    {provider === 'all' ? 'All Platforms' : getSourcePlatformLabel(provider)}
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
              selectClass="min-w-[9rem]"
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

      <Show
        when={!props.loading() && !props.error() && props.filteredRollups().length === 0}
      >
        <div class="p-6">
          <EmptyState {...getRecoveryProtectedItemsEmptyState()} />
        </div>
      </Show>

      <Show when={props.filteredRollups().length > 0}>
        <div class="overflow-x-auto">
          <Table
            class="w-full border-collapse whitespace-nowrap"
            style={{ 'table-layout': 'fixed', 'min-width': props.isMobile ? '100%' : '500px' }}
          >
            <TableHeader>
              <TableRow class="bg-surface-alt text-muted border-b border-border">
                {(
                  [
                    ['subject', getRecoveryArtifactColumnLabel('subject', 'Subject')],
                    ['source', getRecoveryArtifactColumnLabel('source', 'Source')],
                    ['lastBackup', 'Latest Point'],
                    ['outcome', 'Outcome'],
                  ] as const
                ).map(([column, label]) => (
                  <TableHead
                    class={`py-0.5 px-3 whitespace-nowrap text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer select-none hover:text-base-content transition-colors${
                      column === 'source'
                        ? ' hidden md:table-cell w-[110px]'
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
            <TableBody class="divide-y divide-border">
              <For each={sortedRollups()}>
                {(rollup) => {
                  const resourceIndex = props.resourcesById();
                  const label = getRecoveryRollupSubjectLabel(rollup, resourceIndex);
                  const attemptMs = rollup.lastAttemptAt ? Date.parse(rollup.lastAttemptAt) : 0;
                  const successMs = rollup.lastSuccessAt ? Date.parse(rollup.lastSuccessAt) : 0;
                  const outcome = normalizeRecoveryOutcome(rollup.lastOutcome);
                  const providers = (rollup.providers || [])
                    .slice()
                    .map((provider) => String(provider || '').trim())
                    .filter(Boolean)
                    .sort((left, right) =>
                      getSourcePlatformLabel(left).localeCompare(getSourcePlatformLabel(right)),
                    );
                  const nowMs = Date.now();
                  const issueTone: RecoveryIssueTone = getRecoveryRollupIssueTone(rollup, nowMs);
                  const issueRailClass =
                    issueTone === 'none' ? '' : getRecoveryIssueRailClass(issueTone);
                  const stale = isRecoveryRollupStale(rollup, nowMs);
                  const neverSucceeded =
                    (!Number.isFinite(successMs) || successMs <= 0) &&
                    Number.isFinite(attemptMs) &&
                    attemptMs > 0;

                  return (
                    <TableRow
                      class="cursor-pointer border-b border-border hover:bg-surface-hover"
                      onClick={() => props.onSelectRollup(rollup.rollupId)}
                    >
                      <TableCell
                        class={`relative max-w-[420px] truncate whitespace-nowrap px-3 py-0.5 text-base-content ${
                          issueTone === 'rose' || issueTone === 'blue' ? 'font-medium' : ''
                        }`}
                        title={label}
                      >
                        <Show when={issueTone !== 'none'}>
                          <span class={`absolute inset-y-0 left-0 w-0.5 ${issueRailClass}`} />
                        </Show>
                        <div class="flex items-center gap-2">
                          <span class="truncate">{label}</span>
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
                      </TableCell>

                      <TableCell class="hidden md:table-cell whitespace-nowrap px-3 py-0.5">
                        <div class="flex flex-wrap gap-1.5">
                          <For each={providers}>
                            {(provider) => {
                              const badge = getSourcePlatformBadge(provider);
                              return (
                                <span class={badge?.classes || ''}>
                                  {badge?.label || getSourcePlatformLabel(provider)}
                                </span>
                              );
                            }}
                          </For>
                        </div>
                      </TableCell>

                      <TableCell
                        class={`whitespace-nowrap px-3 py-0.5 ${getRecoveryRollupAgeTextClass(
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

                      <TableCell class="whitespace-nowrap px-3 py-0.5">
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
      </Show>
    </Card>
  );
};
