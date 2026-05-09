import { Show, createMemo } from 'solid-js';
import type { Accessor, Component } from 'solid-js';

import { ColumnPicker } from '@/components/shared/ColumnPicker';
import { FilterBar, type FilterDef } from '@/components/shared/FilterBar';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import type { ColumnDef } from '@/hooks/useColumnVisibility';
import { STORAGE_KEYS } from '@/utils/localStorage';
import type { RecoveryOutcome, RecoveryPoint } from '@/types/recovery';
import type { Resource } from '@/types/resource';
import { getRecoveryBreadcrumbLinkClass } from '@/utils/recoveryActionPresentation';
import {
  getRecoveryArtifactModePresentation,
  type RecoveryArtifactMode,
} from '@/utils/recoveryArtifactModePresentation';
import {
  getRecoveryItemTypePresentation,
  normalizeRecoveryItemTypeQueryValue,
} from '@/utils/recoveryItemTypePresentation';
import {
  getRecoveryLocationFacetAllLabel,
  getRecoveryLocationFacetLabel,
} from '@/utils/recoveryLocationPresentation';
import { normalizeRecoveryModeQueryValue } from '@/utils/recoveryRecordPresentation';
import {
  getRecoveryArtifactColumnLabel,
  getRecoveryAllHistoryLabel,
  getRecoveryAllItemTypesLabel,
  getRecoveryAllPlatformsLabel,
  getRecoveryHistorySearchPlaceholder,
  getRecoverySearchHistoryEmptyMessage,
} from '@/utils/recoveryTablePresentation';
import { titleCaseDelimitedLabel } from '@/utils/textPresentation';
import { normalizeSourcePlatformQueryValue, getSourcePlatformLabel } from '@/utils/sourcePlatforms';
import {
  RecoveryHistoryTable,
  type RecoveryPointGroup,
  type RecoveryPointsModel,
} from '@/components/Recovery/RecoveryHistoryTable';
import {
  RecoveryHistoryItemFilter,
  type RecoveryHistoryItemFilterOption,
} from '@/components/Recovery/RecoveryHistoryItemFilter';
import { useRecoveryHistorySectionState } from '@/components/Recovery/useRecoveryHistorySectionState';

type ArtifactMode = RecoveryArtifactMode;
type VerificationFilter = 'all' | 'verified' | 'unverified' | 'unknown';

interface ArtifactColumnVisibility {
  availableToggles: () => ColumnDef[];
  isHiddenByUser: (id: string) => boolean;
  toggle: (id: string) => void;
  resetToDefaults: () => void;
}

interface RecoveryHistorySectionProps {
  activeAdvancedFilterCount: Accessor<number>;
  artifactColumnVisibility: ArtifactColumnVisibility;
  availableOutcomes: readonly ('all' | 'success' | 'warning' | 'failed' | 'running')[];
  clusterFilter: Accessor<string>;
  clusterOptions: Accessor<string[]>;
  currentPage: Accessor<number>;
  groupedByDay: Accessor<RecoveryPointGroup[]>;
  hasActiveArtifactFilters: Accessor<boolean>;
  hasFocusedRollup: Accessor<boolean>;
  historyOutcomeFilter: Accessor<'all' | RecoveryOutcome>;
  historyItemOptions: Accessor<RecoveryHistoryItemFilterOption[]>;
  itemTypeFilter: Accessor<string>;
  itemTypeOptions: Accessor<string[]>;
  isMobile: boolean;
  kioskMode: boolean;
  mobileVisibleArtifactColumns: Accessor<ColumnDef[]>;
  modeFilter: Accessor<'all' | ArtifactMode>;
  namespaceFilter: Accessor<string>;
  namespaceOptions: Accessor<string[]>;
  nodeFilter: Accessor<string>;
  nodeOptions: Accessor<string[]>;
  platformFilter: Accessor<string>;
  platformOptions: Accessor<string[]>;
  queryFilter: Accessor<string>;
  recoveryPoints: RecoveryPointsModel;
  relatedPoints: Accessor<RecoveryPoint[]>;
  resetAdvancedArtifactFilters: () => void;
  resetAllArtifactFilters: () => void;
  resourcesById: Accessor<Map<string, Resource>>;
  rollupId: Accessor<string>;
  scopeFilter: Accessor<'all' | 'workload'>;
  selectedHistoryItemLabel: Accessor<string | null>;
  setClusterFilter: (value: string) => void;
  setCurrentPage: (value: number) => void;
  setHistoryOutcomeFilter: (value: 'all' | RecoveryOutcome) => void;
  setItemTypeFilter: (value: string) => void;
  setModeFilter: (value: 'all' | ArtifactMode) => void;
  setNamespaceFilter: (value: string) => void;
  setNodeFilter: (value: string) => void;
  setPlatformFilter: (value: string) => void;
  setQueryFilter: (value: string) => void;
  setRollupId: (value: string) => void;
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
  const { clearSelectedPoint, selectedPoint, toggleSelectedPoint } = useRecoveryHistorySectionState({
    clusterFilter: props.clusterFilter,
    currentPage: props.currentPage,
    hasFocusedRollup: props.hasFocusedRollup,
    historyOutcomeFilter: props.historyOutcomeFilter,
    itemTypeFilter: props.itemTypeFilter,
    modeFilter: props.modeFilter,
    namespaceFilter: props.namespaceFilter,
    nodeFilter: props.nodeFilter,
    platformFilter: props.platformFilter,
    queryFilter: props.queryFilter,
    scopeFilter: props.scopeFilter,
    verificationFilter: props.verificationFilter,
  });

  const isMobileAccessor = createMemo(() => props.isMobile);

  const setItemType = (value: string) => {
    props.setItemTypeFilter(normalizeRecoveryItemTypeQueryValue(value) || 'all');
    props.setCurrentPage(1);
  };
  const setPlatform = (value: string) => {
    props.setPlatformFilter(normalizeSourcePlatformQueryValue(value));
    props.setCurrentPage(1);
  };
  const setOutcome = (value: string) => {
    const outcome = value as 'all' | RecoveryOutcome;
    props.setHistoryOutcomeFilter(outcome);
    if (outcome !== 'all') props.setVerificationFilter('all');
    props.setCurrentPage(1);
  };
  const setScope = (value: string) => {
    props.setScopeFilter(value === 'workload' ? 'workload' : 'all');
    props.setCurrentPage(1);
  };
  const setMode = (value: string) => {
    props.setModeFilter(normalizeRecoveryModeQueryValue(value));
    props.setCurrentPage(1);
  };
  const setVerification = (value: string) => {
    props.setVerificationFilter(value as VerificationFilter);
    if (value !== 'all') props.setHistoryOutcomeFilter('all');
    props.setCurrentPage(1);
  };
  const setCluster = (value: string) => {
    props.setClusterFilter(value);
    props.setCurrentPage(1);
  };
  const setNode = (value: string) => {
    props.setNodeFilter(value);
    props.setCurrentPage(1);
  };
  const setNamespace = (value: string) => {
    props.setNamespaceFilter(value);
    props.setCurrentPage(1);
  };

  const buildFilters = (): FilterDef[] => {
    const filters: FilterDef[] = [
      {
        id: 'item-type',
        label: getRecoveryArtifactColumnLabel('type', 'Item Type'),
        group: 'properties',
        value: props.itemTypeFilter,
        setValue: setItemType,
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
        setValue: setPlatform,
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
        id: 'outcome',
        label: 'Status',
        group: 'status',
        value: props.historyOutcomeFilter,
        setValue: setOutcome,
        defaultValue: 'all',
        options: () =>
          props.availableOutcomes.map((outcome) => ({
            value: outcome,
            label: outcome === 'all' ? 'Any status' : titleCaseDelimitedLabel(outcome),
          })),
      },
      {
        id: 'scope',
        label: 'Scope',
        group: 'properties',
        value: props.scopeFilter,
        setValue: setScope,
        defaultValue: 'all',
        options: () => [
          { value: 'all', label: getRecoveryAllHistoryLabel() },
          { value: 'workload', label: 'Workloads only' },
        ],
      },
      {
        id: 'method',
        label: 'Method',
        group: 'properties',
        value: props.modeFilter,
        setValue: setMode,
        defaultValue: 'all',
        options: () => [
          { value: 'all', label: 'Any method' },
          { value: 'snapshot', label: getRecoveryArtifactModePresentation('snapshot').label },
          { value: 'local', label: getRecoveryArtifactModePresentation('local').label },
          { value: 'remote', label: getRecoveryArtifactModePresentation('remote').label },
        ],
      },
    ];

    if (props.showVerificationFilter()) {
      filters.push({
        id: 'verification',
        label: 'Verification',
        group: 'status',
        value: props.verificationFilter,
        setValue: setVerification,
        defaultValue: 'all',
        options: () => [
          { value: 'all', label: 'Any verification' },
          { value: 'verified', label: 'Verified' },
          { value: 'unverified', label: 'Unverified' },
          { value: 'unknown', label: 'Unknown' },
        ],
      });
    }

    if (props.showClusterFilter()) {
      filters.push({
        id: 'cluster',
        label: getRecoveryLocationFacetLabel('cluster'),
        group: 'scope',
        value: props.clusterFilter,
        setValue: setCluster,
        defaultValue: 'all',
        options: () => [
          { value: 'all', label: getRecoveryLocationFacetAllLabel('cluster') },
          ...props
            .clusterOptions()
            .filter((value) => value !== 'all')
            .map((cluster) => ({ value: cluster, label: cluster })),
        ],
      });
    }

    if (props.showNodeFilter()) {
      filters.push({
        id: 'node',
        label: getRecoveryLocationFacetLabel('node'),
        group: 'scope',
        value: props.nodeFilter,
        setValue: setNode,
        defaultValue: 'all',
        options: () => [
          { value: 'all', label: getRecoveryLocationFacetAllLabel('node') },
          ...props
            .nodeOptions()
            .filter((value) => value !== 'all')
            .map((node) => ({ value: node, label: node })),
        ],
      });
    }

    if (props.showNamespaceFilter()) {
      filters.push({
        id: 'namespace',
        label: getRecoveryLocationFacetLabel('namespace'),
        group: 'scope',
        value: props.namespaceFilter,
        setValue: setNamespace,
        defaultValue: 'all',
        options: () => [
          { value: 'all', label: getRecoveryLocationFacetAllLabel('namespace') },
          ...props
            .namespaceOptions()
            .filter((value) => value !== 'all')
            .map((namespace) => ({ value: namespace, label: namespace })),
        ],
      });
    }

    return filters;
  };

  return (
    <div class="flex flex-col gap-2">
      <Show when={!props.kioskMode}>
        <Show when={props.hasFocusedRollup() && props.selectedHistoryItemLabel()}>
          <div class="flex min-w-0 flex-wrap items-center gap-2 px-1 text-sm">
            <button
              type="button"
              class={getRecoveryBreadcrumbLinkClass()}
              onClick={() => {
                props.setRollupId('');
                props.setCurrentPage(1);
              }}
            >
              All events
            </button>
            <span class="text-muted">/</span>
            <span
              class="min-w-0 truncate font-medium text-base-content"
              title={props.selectedHistoryItemLabel()}
            >
              {props.selectedHistoryItemLabel()}
            </span>
          </div>
        </Show>

        <FilterBar
          role="group"
          ariaLabel="Recovery events controls"
          isMobile={isMobileAccessor}
          savedViewsKey="recovery-events"
          search={{
            value: props.queryFilter,
            setValue: (value: string) => {
              props.setQueryFilter(value);
              props.setCurrentPage(1);
            },
            placeholder: getRecoveryHistorySearchPlaceholder(),
            historyKey: STORAGE_KEYS.RECOVERY_SEARCH_HISTORY,
            emptyMessage: getRecoverySearchHistoryEmptyMessage(),
            clearOnEscape: true,
          }}
          searchTrailing={
            <RecoveryHistoryItemFilter
              options={props.historyItemOptions}
              selectedRollupId={props.rollupId}
              selectedLabel={props.selectedHistoryItemLabel}
              onSelect={(value) => {
                props.setRollupId(value);
                props.setCurrentPage(1);
              }}
              onClear={() => {
                props.setRollupId('');
                props.setCurrentPage(1);
              }}
            />
          }
          filters={buildFilters()}
          viewOptionsTrailing={
            <ColumnPicker
              columns={props.artifactColumnVisibility.availableToggles()}
              isHidden={props.artifactColumnVisibility.isHiddenByUser}
              onToggle={props.artifactColumnVisibility.toggle}
              onReset={props.artifactColumnVisibility.resetToDefaults}
            />
          }
          onClearAll={() => {
            if (props.hasActiveArtifactFilters()) {
              props.resetAllArtifactFilters();
            }
          }}
          showClearAll={props.hasActiveArtifactFilters}
        />
      </Show>

      <TableCard>
        <TableCardHeader title="Recovery events" />
        <RecoveryHistoryTable
          clearSelectedPoint={clearSelectedPoint}
          currentPage={props.currentPage}
          groupedByDay={props.groupedByDay}
          hasActiveArtifactFilters={props.hasActiveArtifactFilters}
          isMobile={props.isMobile}
          mobileVisibleArtifactColumns={props.mobileVisibleArtifactColumns}
          recoveryPoints={props.recoveryPoints}
          relatedPoints={props.relatedPoints}
          resetAllArtifactFilters={props.resetAllArtifactFilters}
          resourcesById={props.resourcesById}
          selectedPoint={selectedPoint}
          setCurrentPage={props.setCurrentPage}
          tableColumnCount={props.tableColumnCount}
          tableMinWidth={props.tableMinWidth}
          toggleSelectedPoint={toggleSelectedPoint}
          totalPages={props.totalPages}
        />
      </TableCard>
    </div>
  );
};
