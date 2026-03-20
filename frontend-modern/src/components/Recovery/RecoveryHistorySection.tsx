import {
  For,
  Show,
  createEffect,
  createSignal,
  onCleanup,
} from 'solid-js';
import type { Accessor, Component } from 'solid-js';

import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import {
  FilterActionButton,
  FilterToolbarPanel,
  LabeledFilterSelect,
  filterPanelDescriptionClass,
  filterPanelTitleClass,
  filterUtilityBadgeClass,
} from '@/components/shared/FilterToolbar';
import { PageControls } from '@/components/shared/PageControls';
import { SearchInput } from '@/components/shared/SearchInput';
import { getSourcePlatformBadge } from '@/components/shared/sourcePlatformBadges';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import type { ColumnDef } from '@/hooks/useColumnVisibility';
import { STORAGE_KEYS } from '@/utils/localStorage';
import type { RecoveryOutcome, RecoveryPoint } from '@/types/recovery';
import type { Resource } from '@/types/resource';
import { getRecoveryEmptyStateActionClass, getRecoveryFilterPanelClearClass, getRecoveryDrawerCloseButtonClass } from '@/utils/recoveryActionPresentation';
import { getRecoveryArtifactModePresentation, type RecoveryArtifactMode } from '@/utils/recoveryArtifactModePresentation';
import {
  getRecoveryHistoryEmptyState,
  getRecoveryPointsLoadingState,
} from '@/utils/recoveryEmptyStatePresentation';
import {
  getRecoveryPointDetailsSummary,
  getRecoveryPointRepositoryLabel,
  getRecoveryPointSubjectLabel,
  getRecoveryPointTimestampMs,
  normalizeRecoveryModeQueryValue,
} from '@/utils/recoveryRecordPresentation';
import {
  getRecoveryArtifactColumnHeaderClass,
  getRecoveryArtifactRowClass,
  getRecoveryEventTimeTextClass,
  getRecoveryHistorySearchPlaceholder,
  getRecoverySearchHistoryEmptyMessage,
  getRecoverySubjectTypeBadgeClass,
  getRecoverySubjectTypeLabel,
  RECOVERY_ADVANCED_FILTER_FIELD_CLASS,
  RECOVERY_ADVANCED_FILTER_LABEL_CLASS,
  RECOVERY_GROUP_HEADER_ROW_CLASS,
  RECOVERY_GROUP_HEADER_TEXT_CLASS,
} from '@/utils/recoveryTablePresentation';
import { getRecoveryOutcomeBadgeClass } from '@/utils/recoveryOutcomePresentation';
import { titleCaseDelimitedLabel } from '@/utils/textPresentation';
import { formatBytes } from '@/utils/format';
import { formatRecoveryTimeOnly } from '@/utils/recoveryDatePresentation';
import { normalizeSourcePlatformQueryValue, getSourcePlatformLabel } from '@/utils/sourcePlatforms';
import { RecoveryPointDetails } from '@/components/Recovery/RecoveryPointDetails';

type ArtifactMode = RecoveryArtifactMode;
type VerificationFilter = 'all' | 'verified' | 'unverified' | 'unknown';

interface RecoveryPointGroup {
  key: string;
  label: string;
  tone: 'recent' | 'default';
  items: RecoveryPoint[];
}

interface RecoveryPointsMeta {
  page: number;
  limit: number;
  total: number;
  totalPages: number;
}

interface RecoveryPointsModel {
  meta: Accessor<RecoveryPointsMeta>;
  response: {
    loading: boolean;
    error: unknown;
  };
}

interface PageControlsColumnVisibility {
  availableToggles: () => ColumnDef[];
  isHiddenByUser: (id: string) => boolean;
  toggle: (id: string) => void;
  resetToDefaults: () => void;
}

interface RecoveryHistorySectionProps {
  activeAdvancedFilterCount: Accessor<number>;
  artifactColumnVisibility: PageControlsColumnVisibility;
  availableOutcomes: readonly ('all' | 'success' | 'warning' | 'failed' | 'running')[];
  clusterFilter: Accessor<string>;
  clusterOptions: Accessor<string[]>;
  currentPage: Accessor<number>;
  groupedByDay: Accessor<RecoveryPointGroup[]>;
  hasActiveArtifactFilters: Accessor<boolean>;
  historyOutcomeFilter: Accessor<'all' | RecoveryOutcome>;
  isMobile: boolean;
  kioskMode: boolean;
  mobileVisibleArtifactColumns: Accessor<ColumnDef[]>;
  modeFilter: Accessor<'all' | ArtifactMode>;
  namespaceFilter: Accessor<string>;
  namespaceOptions: Accessor<string[]>;
  nodeFilter: Accessor<string>;
  nodeOptions: Accessor<string[]>;
  providerFilter: Accessor<string>;
  providerOptions: Accessor<string[]>;
  queryFilter: Accessor<string>;
  recoveryPoints: RecoveryPointsModel;
  resetAdvancedArtifactFilters: () => void;
  resetAllArtifactFilters: () => void;
  resourcesById: Accessor<Map<string, Resource>>;
  scopeFilter: Accessor<'all' | 'workload'>;
  setClusterFilter: (value: string) => void;
  setCurrentPage: (value: number) => void;
  setHistoryOutcomeFilter: (value: 'all' | RecoveryOutcome) => void;
  setModeFilter: (value: 'all' | ArtifactMode) => void;
  setNamespaceFilter: (value: string) => void;
  setNodeFilter: (value: string) => void;
  setProviderFilter: (value: string) => void;
  setQueryFilter: (value: string) => void;
  setScopeFilter: (value: 'all' | 'workload') => void;
  setVerificationFilter: (value: VerificationFilter) => void;
  showClusterFilter: Accessor<boolean>;
  showNamespaceFilter: Accessor<boolean>;
  showNodeFilter: Accessor<boolean>;
  showVerificationFilter: Accessor<boolean>;
  tableColumnCount: Accessor<number>;
  tableMinWidth: Accessor<string>;
  totalPages: Accessor<number>;
  verificationFilter: Accessor<VerificationFilter>;
}

export const RecoveryHistorySection: Component<RecoveryHistorySectionProps> = (props) => {
  const [selectedPoint, setSelectedPoint] = createSignal<RecoveryPoint | null>(null);
  const [moreFiltersOpen, setMoreFiltersOpen] = createSignal(false);
  const [historyFiltersOpen, setHistoryFiltersOpen] = createSignal(false);
  let advancedFiltersPanelRef: HTMLDivElement | undefined;
  let advancedFiltersButtonRef: HTMLButtonElement | undefined;

  const historyActiveFilterCount = () => {
    let count = 0;
    if (props.queryFilter().trim() !== '') count += 1;
    if (props.providerFilter() !== 'all') count += 1;
    if (props.historyOutcomeFilter() !== 'all') count += 1;
    if (props.scopeFilter() !== 'all') count += 1;
    if (props.modeFilter() !== 'all') count += 1;
    if (props.verificationFilter() !== 'all') count += 1;
    if (props.clusterFilter() !== 'all') count += 1;
    if (props.nodeFilter() !== 'all') count += 1;
    if (props.namespaceFilter() !== 'all') count += 1;
    return count;
  };

  createEffect(() => {
    props.currentPage();
    props.providerFilter();
    props.historyOutcomeFilter();
    props.scopeFilter();
    props.modeFilter();
    props.verificationFilter();
    props.clusterFilter();
    props.nodeFilter();
    props.namespaceFilter();
    props.queryFilter();
    setSelectedPoint(null);
  });

  const handleAdvancedFiltersClickOutside = (event: MouseEvent) => {
    const target = event.target as Node;
    if (advancedFiltersPanelRef?.contains(target) || advancedFiltersButtonRef?.contains(target)) {
      return;
    }
    setMoreFiltersOpen(false);
  };

  createEffect(() => {
    if (moreFiltersOpen()) {
      document.addEventListener('mousedown', handleAdvancedFiltersClickOutside);
    } else {
      document.removeEventListener('mousedown', handleAdvancedFiltersClickOutside);
    }
  });

  onCleanup(() => {
    document.removeEventListener('mousedown', handleAdvancedFiltersClickOutside);
  });

  return (
    <Card padding="none" tone="card" class="mb-4 overflow-hidden">
      <div class="border-b border-border bg-surface-hover px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-muted">
        Backups By Date
      </div>
      <Show when={!props.kioskMode}>
        <div class="border-b border-border px-3 py-3">
          <PageControls
            search={
              <SearchInput
                value={props.queryFilter}
                onChange={(value) => {
                  props.setQueryFilter(value);
                  props.setCurrentPage(1);
                }}
                placeholder={getRecoveryHistorySearchPlaceholder()}
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
              onToggle: () => setHistoryFiltersOpen((open) => !open),
              count: historyActiveFilterCount(),
            }}
            utilityActions={
              <div class="ml-auto flex items-center gap-2">
                <div class="relative">
                  <FilterActionButton
                    ref={advancedFiltersButtonRef}
                    aria-label="Filter"
                    aria-expanded={moreFiltersOpen()}
                    aria-controls="recovery-filter-panel"
                    aria-haspopup="dialog"
                    onClick={() => setMoreFiltersOpen((open) => !open)}
                    active={moreFiltersOpen() || props.activeAdvancedFilterCount() > 0}
                  >
                    <span>Filter</span>
                    <Show when={props.activeAdvancedFilterCount() > 0}>
                      <span class={filterUtilityBadgeClass}>
                        {props.activeAdvancedFilterCount()}
                      </span>
                    </Show>
                  </FilterActionButton>

                  <Show when={moreFiltersOpen()}>
                    <FilterToolbarPanel ref={advancedFiltersPanelRef} id="recovery-filter-panel">
                      <div class="mb-3 flex items-center justify-between gap-3">
                        <div>
                          <div class={filterPanelTitleClass}>Filter results</div>
                          <div class={filterPanelDescriptionClass}>
                            Narrow by scope, method, verification, or location.
                          </div>
                        </div>
                        <Show when={props.activeAdvancedFilterCount() > 0}>
                          <button
                            type="button"
                            onClick={props.resetAdvancedArtifactFilters}
                            class={getRecoveryFilterPanelClearClass()}
                          >
                            Clear filters
                          </button>
                        </Show>
                      </div>

                      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                        <label class="flex min-w-0 flex-col gap-1">
                          <span class={RECOVERY_ADVANCED_FILTER_LABEL_CLASS}>Scope</span>
                          <select
                            value={props.scopeFilter()}
                            onChange={(event) => {
                              props.setScopeFilter(
                                event.currentTarget.value === 'workload' ? 'workload' : 'all',
                              );
                              props.setCurrentPage(1);
                            }}
                            class={RECOVERY_ADVANCED_FILTER_FIELD_CLASS}
                          >
                            <option value="all">All history</option>
                            <option value="workload">Workloads only</option>
                          </select>
                        </label>

                        <label class="flex min-w-0 flex-col gap-1">
                          <span class={RECOVERY_ADVANCED_FILTER_LABEL_CLASS}>Method</span>
                          <select
                            value={props.modeFilter()}
                            onChange={(event) => {
                              props.setModeFilter(
                                normalizeRecoveryModeQueryValue(event.currentTarget.value),
                              );
                              props.setCurrentPage(1);
                            }}
                            class={RECOVERY_ADVANCED_FILTER_FIELD_CLASS}
                          >
                            <option value="all">Any method</option>
                            <option value="snapshot">
                              {getRecoveryArtifactModePresentation('snapshot').label}
                            </option>
                            <option value="local">
                              {getRecoveryArtifactModePresentation('local').label}
                            </option>
                            <option value="remote">
                              {getRecoveryArtifactModePresentation('remote').label}
                            </option>
                          </select>
                        </label>

                        <Show when={props.showVerificationFilter()}>
                          <label class="flex min-w-0 flex-col gap-1">
                            <span class={RECOVERY_ADVANCED_FILTER_LABEL_CLASS}>Verification</span>
                            <select
                              value={props.verificationFilter()}
                              onChange={(event) => {
                                props.setVerificationFilter(
                                  event.currentTarget.value as VerificationFilter,
                                );
                                if (event.currentTarget.value !== 'all') {
                                  props.setHistoryOutcomeFilter('all');
                                }
                                props.setCurrentPage(1);
                              }}
                              class={RECOVERY_ADVANCED_FILTER_FIELD_CLASS}
                            >
                              <option value="all">Any verification</option>
                              <option value="verified">Verified</option>
                              <option value="unverified">Unverified</option>
                              <option value="unknown">Unknown</option>
                            </select>
                          </label>
                        </Show>

                        <Show when={props.showClusterFilter()}>
                          <label class="flex min-w-0 flex-col gap-1">
                            <span class={RECOVERY_ADVANCED_FILTER_LABEL_CLASS}>Cluster</span>
                            <select
                              value={props.clusterFilter()}
                              onChange={(event) => {
                                props.setClusterFilter(event.currentTarget.value);
                                props.setCurrentPage(1);
                              }}
                              class={RECOVERY_ADVANCED_FILTER_FIELD_CLASS}
                            >
                              <option value="all">Any cluster</option>
                              <For each={props.clusterOptions().filter((value) => value !== 'all')}>
                                {(cluster) => <option value={cluster}>{cluster}</option>}
                              </For>
                            </select>
                          </label>
                        </Show>

                        <Show when={props.showNodeFilter()}>
                          <label class="flex min-w-0 flex-col gap-1">
                            <span class={RECOVERY_ADVANCED_FILTER_LABEL_CLASS}>Node or agent</span>
                            <select
                              value={props.nodeFilter()}
                              onChange={(event) => {
                                props.setNodeFilter(event.currentTarget.value);
                                props.setCurrentPage(1);
                              }}
                              class={RECOVERY_ADVANCED_FILTER_FIELD_CLASS}
                            >
                              <option value="all">Any node or agent</option>
                              <For each={props.nodeOptions().filter((value) => value !== 'all')}>
                                {(node) => <option value={node}>{node}</option>}
                              </For>
                            </select>
                          </label>
                        </Show>

                        <Show when={props.showNamespaceFilter()}>
                          <label class="flex min-w-0 flex-col gap-1">
                            <span class={RECOVERY_ADVANCED_FILTER_LABEL_CLASS}>Namespace</span>
                            <select
                              value={props.namespaceFilter()}
                              onChange={(event) => {
                                props.setNamespaceFilter(event.currentTarget.value);
                                props.setCurrentPage(1);
                              }}
                              class={RECOVERY_ADVANCED_FILTER_FIELD_CLASS}
                            >
                              <option value="all">Any namespace</option>
                              <For each={props.namespaceOptions().filter((value) => value !== 'all')}>
                                {(namespace) => <option value={namespace}>{namespace}</option>}
                              </For>
                            </select>
                          </label>
                        </Show>
                      </div>
                    </FilterToolbarPanel>
                  </Show>
                </div>
              </div>
            }
            columnVisibility={props.artifactColumnVisibility}
            resetAction={{
              show: props.hasActiveArtifactFilters(),
              onClick: props.resetAllArtifactFilters,
              label: 'Reset all',
            }}
            showFilters={!props.isMobile || historyFiltersOpen()}
            toolbarClass="lg:flex-nowrap"
          >
            <LabeledFilterSelect
              id="recovery-provider-filter-history"
              label="History provider"
              value={props.providerFilter()}
              onChange={(event) => {
                props.setProviderFilter(
                  normalizeSourcePlatformQueryValue(event.currentTarget.value),
                );
                props.setCurrentPage(1);
              }}
              selectClass="min-w-[10rem] max-w-[14rem]"
            >
              <For each={props.providerOptions()}>
                {(provider) => (
                  <option value={provider}>
                    {provider === 'all' ? 'All Providers' : getSourcePlatformLabel(provider)}
                  </option>
                )}
              </For>
            </LabeledFilterSelect>

            <LabeledFilterSelect
              id="recovery-status-filter"
              label="History status"
              value={props.historyOutcomeFilter()}
              onChange={(event) => {
                const value = event.currentTarget.value as 'all' | RecoveryOutcome;
                props.setHistoryOutcomeFilter(value);
                if (value !== 'all') props.setVerificationFilter('all');
                props.setCurrentPage(1);
              }}
              selectClass="min-w-[7rem]"
            >
              <For each={props.availableOutcomes}>
                {(outcome) => (
                  <option value={outcome}>
                    {outcome === 'all' ? 'Any status' : titleCaseDelimitedLabel(outcome)}
                  </option>
                )}
              </For>
            </LabeledFilterSelect>
          </PageControls>
        </div>
      </Show>

      <Show
        when={props.groupedByDay().length > 0}
        fallback={
          <div class="p-6">
            <Show
              when={props.recoveryPoints.response.loading}
              fallback={
                <EmptyState
                  {...getRecoveryHistoryEmptyState()}
                  actions={
                    <Show when={props.hasActiveArtifactFilters()}>
                      <button
                        type="button"
                        onClick={props.resetAllArtifactFilters}
                        class={getRecoveryEmptyStateActionClass()}
                      >
                        Clear filters
                      </button>
                    </Show>
                  }
                />
              }
            >
              <div class="text-sm text-muted">{getRecoveryPointsLoadingState().text}</div>
            </Show>
          </div>
        }
      >
        <div class="overflow-x-auto">
          <Table
            class="w-full border-collapse text-xs whitespace-nowrap"
            style={{ 'min-width': props.tableMinWidth(), 'table-layout': 'fixed' }}
          >
            <TableHeader>
              <TableRow class="bg-surface-alt text-muted border-b border-border">
                <For each={props.mobileVisibleArtifactColumns()}>
                  {(column) => (
                    <TableHead
                      class={`py-0.5 px-3 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap ${getRecoveryArtifactColumnHeaderClass(
                        column.id,
                      )}`}
                    >
                      {column.label}
                    </TableHead>
                  )}
                </For>
              </TableRow>
            </TableHeader>
            <TableBody class="divide-y divide-border">
              <For each={props.groupedByDay()}>
                {(group) => (
                  <>
                    <TableRow class={RECOVERY_GROUP_HEADER_ROW_CLASS}>
                      <TableCell colSpan={props.tableColumnCount()} class={RECOVERY_GROUP_HEADER_TEXT_CLASS}>
                        <div class="flex items-center justify-between gap-3">
                          <div class="flex min-w-0 items-center gap-2">
                            <span class="truncate" title={group.label}>
                              {group.label}
                            </span>
                            <Show when={group.tone === 'recent'}>
                              <span class="inline-flex rounded px-1.5 py-px text-[9px] font-medium border border-border bg-surface">
                                recent
                              </span>
                            </Show>
                          </div>
                          <span class="shrink-0 font-mono text-[10px] tabular-nums text-muted">
                            {group.items.length}
                          </span>
                        </div>
                      </TableCell>
                    </TableRow>

                    <For each={group.items}>
                      {(point) => {
                        const resourceIndex = props.resourcesById();
                        const subject = getRecoveryPointSubjectLabel(point, resourceIndex);
                        const tsMs = getRecoveryPointTimestampMs(point);
                        const timeOnly =
                          point.completedAt && Number.isFinite(tsMs)
                            ? formatRecoveryTimeOnly(tsMs)
                            : '—';
                        const subjectType = getRecoverySubjectTypeLabel(point);
                        const provider = String(point.provider || '').trim();
                        const mode =
                          (normalizeRecoveryModeQueryValue(String(point.mode || '').toLowerCase()) as ArtifactMode) ||
                          'local';
                        const outcome = (String(point.outcome || 'unknown').toLowerCase() as RecoveryOutcome) || 'unknown';
                        const repoLabel = getRecoveryPointRepositoryLabel(point);
                        const detailsSummary = getRecoveryPointDetailsSummary(point);
                        const entityId = String(point.entityId || '').trim();
                        const cluster = String(point.cluster || '').trim();
                        const nodeAgent = String(point.node || '').trim();
                        const namespace = String(point.namespace || '').trim();

                        return (
                          <>
                            <TableRow
                              class={`cursor-pointer ${getRecoveryArtifactRowClass(selectedPoint()?.id === point.id)}`}
                              onClick={() => setSelectedPoint(selectedPoint()?.id === point.id ? null : point)}
                            >
                              <For each={props.mobileVisibleArtifactColumns()}>
                                {(column) => {
                                  switch (column.id) {
                                    case 'time':
                                      return (
                                        <TableCell
                                          class={`whitespace-nowrap px-3 py-0.5 text-right font-mono text-[11px] tabular-nums ${getRecoveryEventTimeTextClass(
                                            tsMs,
                                          )}`}
                                        >
                                          {timeOnly}
                                        </TableCell>
                                      );
                                    case 'type':
                                      return (
                                        <TableCell class="whitespace-nowrap px-3 py-0.5 text-center">
                                          <Show when={subjectType} fallback={<span class="text-muted">—</span>}>
                                            <span
                                              class={`inline-flex min-w-[2.75rem] justify-center rounded px-1.5 py-px text-[9px] font-medium leading-none ${getRecoverySubjectTypeBadgeClass(
                                                point,
                                              )}`}
                                            >
                                              {subjectType}
                                            </span>
                                          </Show>
                                        </TableCell>
                                      );
                                    case 'subject':
                                      return (
                                        <TableCell
                                          class="max-w-[420px] whitespace-nowrap px-3 py-0.5 text-base-content"
                                          title={subject}
                                        >
                                          <div class="flex min-w-0 max-w-full items-center gap-2">
                                            <span class="min-w-0 flex-1 truncate font-medium">
                                              {subject}
                                            </span>
                                            <span class="inline-flex shrink-0 items-center gap-1">
                                              <Show when={point.immutable === true}>
                                                <svg
                                                  class="h-3 w-3 text-emerald-500 dark:text-emerald-400"
                                                  fill="none"
                                                  stroke="currentColor"
                                                  viewBox="0 0 24 24"
                                                  aria-hidden="true"
                                                >
                                                  <path
                                                    stroke-linecap="round"
                                                    stroke-linejoin="round"
                                                    stroke-width="2"
                                                    d="M12 3l7 4v5c0 5-3.5 7.5-7 9-3.5-1.5-7-4-7-9V7l7-4z"
                                                  />
                                                </svg>
                                              </Show>
                                              <Show when={point.encrypted === true}>
                                                <svg
                                                  class="h-3 w-3 text-amber-500 dark:text-amber-400"
                                                  fill="currentColor"
                                                  viewBox="0 0 20 20"
                                                  aria-hidden="true"
                                                >
                                                  <path
                                                    fill-rule="evenodd"
                                                    d="M5 9V7a5 5 0 0110 0v2a2 2 0 012 2v5a2 2 0 01-2 2H5a2 2 0 01-2-2v-5a2 2 0 012-2zm8-2V7a3 3 0 016 0z"
                                                    clip-rule="evenodd"
                                                  />
                                                </svg>
                                              </Show>
                                            </span>
                                          </div>
                                        </TableCell>
                                      );
                                    case 'entityId':
                                      return (
                                        <TableCell class="whitespace-nowrap px-3 py-0.5 text-[11px] text-muted font-mono tabular-nums">
                                          {entityId || '—'}
                                        </TableCell>
                                      );
                                    case 'cluster':
                                      return (
                                        <TableCell class="whitespace-nowrap px-3 py-0.5 text-[11px] text-muted font-mono">
                                          {cluster || '—'}
                                        </TableCell>
                                      );
                                    case 'nodeAgent':
                                      return (
                                        <TableCell class="whitespace-nowrap px-3 py-0.5 text-[11px] text-muted font-mono">
                                          {nodeAgent || '—'}
                                        </TableCell>
                                      );
                                    case 'namespace':
                                      return (
                                        <TableCell class="whitespace-nowrap px-3 py-0.5 text-[11px] text-muted font-mono">
                                          {namespace || '—'}
                                        </TableCell>
                                      );
                                    case 'source': {
                                      const badge = getSourcePlatformBadge(provider);
                                      return (
                                        <TableCell class="whitespace-nowrap px-3 py-0.5 text-center">
                                          <span
                                            class={`${badge?.classes || ''} inline-flex min-w-[3.25rem] justify-center px-1.5 py-px text-[9px] font-medium`}
                                          >
                                            {badge?.label || getSourcePlatformLabel(provider)}
                                          </span>
                                        </TableCell>
                                      );
                                    }
                                    case 'verified':
                                      return (
                                        <TableCell class="whitespace-nowrap px-3 py-0.5 text-center">
                                          {typeof point.verified === 'boolean' ? (
                                            point.verified ? (
                                              <span
                                                class="inline-flex min-w-[1.25rem] items-center justify-center text-green-600 dark:text-green-400"
                                                title="Verified"
                                              >
                                                <svg
                                                  class="h-3.5 w-3.5"
                                                  fill="none"
                                                  stroke="currentColor"
                                                  viewBox="0 0 24 24"
                                                >
                                                  <path
                                                    stroke-linecap="round"
                                                    stroke-linejoin="round"
                                                    stroke-width="2.5"
                                                    d="M5 13l4 4L19 7"
                                                  />
                                                </svg>
                                              </span>
                                            ) : (
                                              <span
                                                class="inline-flex min-w-[1.25rem] items-center justify-center text-amber-500 dark:text-amber-400"
                                                title="Unverified"
                                              >
                                                <svg
                                                  class="h-3.5 w-3.5"
                                                  fill="none"
                                                  stroke="currentColor"
                                                  viewBox="0 0 24 24"
                                                >
                                                  <path
                                                    stroke-linecap="round"
                                                    stroke-linejoin="round"
                                                    stroke-width="2.5"
                                                    d="M12 9v2m0 4h.01M12 3a9 9 0 100 18 9 9 0 000-18z"
                                                  />
                                                </svg>
                                              </span>
                                            )
                                          ) : (
                                            <span class="text-muted">—</span>
                                          )}
                                        </TableCell>
                                      );
                                    case 'size':
                                      return (
                                        <TableCell class="whitespace-nowrap px-3 py-0.5 text-right font-mono text-[11px] tabular-nums text-muted">
                                          {point.sizeBytes && point.sizeBytes > 0
                                            ? formatBytes(point.sizeBytes)
                                            : '—'}
                                        </TableCell>
                                      );
                                    case 'method':
                                      return (
                                        <TableCell class="whitespace-nowrap px-3 py-0.5 text-center">
                                          <span
                                            class={`inline-flex min-w-[3.5rem] justify-center rounded px-1.5 py-px text-[9px] font-medium ${getRecoveryArtifactModePresentation(
                                              mode,
                                            ).badgeClassName}`}
                                          >
                                            {getRecoveryArtifactModePresentation(mode).label}
                                          </span>
                                        </TableCell>
                                      );
                                    case 'repository':
                                      return (
                                        <TableCell
                                          class="max-w-[220px] truncate whitespace-nowrap px-3 py-0.5 text-[11px] leading-4 text-base-content"
                                          title={repoLabel}
                                        >
                                          {repoLabel || '—'}
                                        </TableCell>
                                      );
                                    case 'details':
                                      return (
                                        <TableCell
                                          class="max-w-[280px] truncate whitespace-nowrap px-3 py-0.5 text-[10px] leading-4 text-muted"
                                          title={detailsSummary}
                                        >
                                          {detailsSummary || '—'}
                                        </TableCell>
                                      );
                                    case 'outcome':
                                      return (
                                        <TableCell class="whitespace-nowrap px-3 py-0.5 text-center">
                                          <span
                                            class={`inline-flex min-w-[4.5rem] justify-center rounded px-1.5 py-px text-[9px] font-medium ${getRecoveryOutcomeBadgeClass(
                                              outcome,
                                            )}`}
                                          >
                                            {titleCaseDelimitedLabel(outcome)}
                                          </span>
                                        </TableCell>
                                      );
                                    default:
                                      return (
                                        <TableCell class="whitespace-nowrap px-3 py-0.5 text-muted">
                                          -
                                        </TableCell>
                                      );
                                  }
                                }}
                              </For>
                            </TableRow>

                            <Show when={selectedPoint()?.id === point.id}>
                              <TableRow>
                                <TableCell
                                  colSpan={props.tableColumnCount()}
                                  class="bg-surface-alt px-0 sm:px-4 py-4 relative"
                                >
                                  <div class="flex items-center justify-between px-4 pb-2 mb-2 border-b border-border">
                                    <h2 class="text-sm font-semibold text-base-content">
                                      Recovery Point Details
                                    </h2>
                                    <button
                                      type="button"
                                      onClick={(event) => {
                                        event.stopPropagation();
                                        setSelectedPoint(null);
                                      }}
                                      class={getRecoveryDrawerCloseButtonClass()}
                                      aria-label="Close details"
                                    >
                                      <svg
                                        class="h-5 w-5"
                                        fill="none"
                                        stroke="currentColor"
                                        viewBox="0 0 24 24"
                                      >
                                        <path
                                          stroke-linecap="round"
                                          stroke-linejoin="round"
                                          stroke-width="2"
                                          d="M6 18L18 6M6 6l12 12"
                                        />
                                      </svg>
                                    </button>
                                  </div>
                                  <div class="px-4">
                                    <RecoveryPointDetails point={point} />
                                  </div>
                                </TableCell>
                              </TableRow>
                            </Show>
                          </>
                        );
                      }}
                    </For>
                  </>
                )}
              </For>
            </TableBody>
          </Table>
        </div>

        <div class="flex items-center justify-between gap-2 px-3 py-2 text-xs text-muted border-t border-border">
          <div>
            <Show
              when={(props.recoveryPoints.meta().total || 0) > 0}
              fallback={<span>Showing 0 of 0 recovery points</span>}
            >
              <span>
                Showing {(props.recoveryPoints.meta().page - 1) * props.recoveryPoints.meta().limit + 1} -{' '}
                {Math.min(
                  props.recoveryPoints.meta().page * props.recoveryPoints.meta().limit,
                  props.recoveryPoints.meta().total,
                )}{' '}
                of {props.recoveryPoints.meta().total} recovery points
              </span>
            </Show>
          </div>
          <div class="flex items-center gap-2">
            <button
              type="button"
              disabled={props.currentPage() <= 1}
              onClick={() => props.setCurrentPage(Math.max(1, props.currentPage() - 1))}
              class="rounded-md border border-border bg-surface px-2 py-1 text-xs font-medium text-base-content disabled:opacity-50"
            >
              Prev
            </button>
            <span>
              Page {props.currentPage()} / {props.totalPages()}
            </span>
            <button
              type="button"
              disabled={props.currentPage() >= props.totalPages()}
              onClick={() => props.setCurrentPage(Math.min(props.totalPages(), props.currentPage() + 1))}
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
