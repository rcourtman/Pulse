import { For, Show } from 'solid-js';
import type { Accessor, Component } from 'solid-js';

import { Card } from '@/components/shared/Card';
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
import type { ColumnDef } from '@/hooks/useColumnVisibility';
import { STORAGE_KEYS } from '@/utils/localStorage';
import type { RecoveryOutcome } from '@/types/recovery';
import type { Resource } from '@/types/resource';
import { getRecoveryFilterPanelClearClass } from '@/utils/recoveryActionPresentation';
import { getRecoveryArtifactModePresentation, type RecoveryArtifactMode } from '@/utils/recoveryArtifactModePresentation';
import { normalizeRecoveryModeQueryValue } from '@/utils/recoveryRecordPresentation';
import {
  getRecoveryHistorySearchPlaceholder,
  getRecoverySearchHistoryEmptyMessage,
  RECOVERY_ADVANCED_FILTER_FIELD_CLASS,
  RECOVERY_ADVANCED_FILTER_LABEL_CLASS,
} from '@/utils/recoveryTablePresentation';
import { titleCaseDelimitedLabel } from '@/utils/textPresentation';
import { normalizeSourcePlatformQueryValue, getSourcePlatformLabel } from '@/utils/sourcePlatforms';
import {
  RecoveryHistoryTable,
  type RecoveryPointGroup,
  type RecoveryPointsModel,
} from '@/components/Recovery/RecoveryHistoryTable';
import { useRecoveryHistorySectionState } from '@/components/Recovery/useRecoveryHistorySectionState';

type ArtifactMode = RecoveryArtifactMode;
type VerificationFilter = 'all' | 'verified' | 'unverified' | 'unknown';

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
  const {
    advancedFiltersButtonRef,
    advancedFiltersPanelRef,
    clearSelectedPoint,
    historyActiveFilterCount,
    historyFiltersOpen,
    moreFiltersOpen,
    selectedPoint,
    setHistoryFiltersOpen,
    setMoreFiltersOpen,
    toggleSelectedPoint,
  } = useRecoveryHistorySectionState({
    clusterFilter: props.clusterFilter,
    currentPage: props.currentPage,
    historyOutcomeFilter: props.historyOutcomeFilter,
    modeFilter: props.modeFilter,
    namespaceFilter: props.namespaceFilter,
    nodeFilter: props.nodeFilter,
    providerFilter: props.providerFilter,
    queryFilter: props.queryFilter,
    scopeFilter: props.scopeFilter,
    verificationFilter: props.verificationFilter,
  });

  return (
    <Card padding="none" tone="card" class="mb-4 overflow-hidden">
      <div class="border-b border-border bg-surface-hover px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-muted">
        Recovery Events
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
              label="History platform"
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
                    {provider === 'all' ? 'All Platforms' : getSourcePlatformLabel(provider)}
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
      <RecoveryHistoryTable
        clearSelectedPoint={clearSelectedPoint}
        currentPage={props.currentPage}
        groupedByDay={props.groupedByDay}
        hasActiveArtifactFilters={props.hasActiveArtifactFilters}
        mobileVisibleArtifactColumns={props.mobileVisibleArtifactColumns}
        recoveryPoints={props.recoveryPoints}
        resetAllArtifactFilters={props.resetAllArtifactFilters}
        resourcesById={props.resourcesById}
        selectedPoint={selectedPoint}
        setCurrentPage={props.setCurrentPage}
        tableColumnCount={props.tableColumnCount}
        tableMinWidth={props.tableMinWidth}
        toggleSelectedPoint={toggleSelectedPoint}
        totalPages={props.totalPages}
      />
    </Card>
  );
};
