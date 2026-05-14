import { Show, batch, createEffect, createMemo } from 'solid-js';
import type { Component } from 'solid-js';

import { RecoveryActivitySection } from '@/components/Recovery/RecoveryActivitySection';
import { RecoveryHistorySection } from '@/components/Recovery/RecoveryHistorySection';
import { RecoveryProtectedInventorySection } from '@/components/Recovery/RecoveryProtectedInventorySection';
import { EmptyState } from '@/components/shared/EmptyState';
import { PageHeader } from '@/components/shared/PageHeader';
import { TableCard } from '@/components/shared/TableCard';
import type { FilterSelectOption } from '@/components/shared/FilterBar';
import { useRecoverySurfaceState } from '@/features/recovery/useRecoverySurfaceState';
import type { ProtectedStateFilter } from '@/features/recovery/useRecoverySurfaceState';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { useColumnVisibility } from '@/hooks/useColumnVisibility';
import type { ColumnDef } from '@/hooks/useColumnVisibility';
import { useKioskMode } from '@/hooks/useKioskMode';
import type { ProtectionRollup, RecoveryOutcome, RecoveryPoint } from '@/types/recovery';
import { STORAGE_KEYS } from '@/utils/localStorage';
import {
  getRecoveryFilterDateLabel,
  getRecoveryFullDateLabel,
  getRecoveryNiceAxisMax,
  parseRecoveryDateKey,
  recoveryDateKeyFromTimestamp,
  resolveRecoveryDateSearchKey,
} from '@/utils/recoveryDatePresentation';
import { getRecoveryPointsFailureState } from '@/utils/recoveryEmptyStatePresentation';
import { normalizeRecoveryOutcome as normalizeOutcome } from '@/utils/recoveryOutcomePresentation';
import {
  getRecoveryPointItemLabel,
  getRecoveryPointTimestampMs,
  getRecoveryRollupItemLabel,
} from '@/utils/recoveryRecordPresentation';
import { getRecoveryRollupItemTypeKey } from '@/utils/recoveryItemTypePresentation';
import {
  getRecoveryArtifactColumnLabel,
  getRecoveryGroupNoTimestampLabel,
  getRecoveryArtifactTableMinWidth,
  getRecoveryRollupInventoryStatus,
  STALE_ISSUE_THRESHOLD_MS,
} from '@/utils/recoveryTablePresentation';
import { getRecoveryTimelineLabelEvery } from '@/utils/recoveryTimelineChartPresentation';
import { getRecoveryTimelineDayFilterLabel } from '@/utils/recoveryTimelinePresentation';
import { getRecoveryRollupPlatforms } from '@/utils/recoveryPlatformModel';
import { createVisibleCanonicalTypeColumn } from '@/utils/typeColumnDefinition';

const MOBILE_RECOVERY_COLUMNS = new Set(['time', 'item', 'outcome']);
const LEGACY_RECOVERY_COLUMN_ID_ALIASES = {
  subject: 'item',
  source: 'platform',
} as const;
type RecoveryActivityRangeDays = 7 | 30 | 90 | 365;

const Recovery: Component = () => {
  const kioskMode = useKioskMode();
  const { isMobile } = useBreakpoint();

  const {
    chartRangeDays,
    clusterFilter,
    clusterOptions,
    currentPage,
    facets,
    historyOutcomeFilter,
    historyItemOptions,
    itemTypeFilter,
    itemTypeOptions,
    modeFilter,
    namespaceFilter,
    namespaceOptions,
    nodeFilter,
    nodeOptions,
    protectedStateFilter,
    platformFilter,
    platformOptions,
    queryFilter,
    recoveryPoints,
    recoveryRollups,
    recoverySeries,
    resourcesById,
    rollupId,
    rollups,
    scopeFilter,
    selectedDateKey,
    selectedHistoryItemLabel,
    setChartRangeDays,
    setClusterFilter,
    setCurrentPage,
    setHistoryOutcomeFilter,
    setItemTypeFilter,
    setModeFilter,
    setNamespaceFilter,
    setNodeFilter,
    setProtectedStateFilter,
    setPlatformFilter,
    setQueryFilter,
    setRollupId,
    setScopeFilter,
    setSelectedDateKey,
    setVerificationFilter,
    setWorkspaceView,
    showClusterFilter,
    showNamespaceFilter,
    showNodeFilter,
    showVerificationFilter,
    tableWindow,
    totalPages,
    verificationFilter,
    workspaceView,
  } = useRecoverySurfaceState();

  const baseRollups = createMemo<ProtectionRollup[]>(() => {
    const query = queryFilter().trim().toLowerCase();
    const platform = platformFilter() === 'all' ? '' : platformFilter();
    const itemType = itemTypeFilter() === 'all' ? '' : itemTypeFilter();
    const resourceIndex = resourcesById();

    const result = rollups().filter((rollup) => {
      const platforms = getRecoveryRollupPlatforms(rollup)
        .map((entry) => String(entry || '').trim())
        .filter(Boolean);
      if (platform && !platforms.includes(platform)) return false;
      const rollupItemType = getRecoveryRollupItemTypeKey(rollup);
      if (itemType && rollupItemType !== itemType) return false;

      if (!query) return true;
      const label = getRecoveryRollupItemLabel(rollup, resourceIndex);
      const haystack = [
        rollup.rollupId,
        rollup.itemResourceId || '',
        label,
        rollupItemType,
        rollup.itemRef?.namespace || rollup.subjectRef?.namespace || '',
        rollup.itemRef?.name || rollup.subjectRef?.name || '',
        platforms.join(' '),
        rollup.lastOutcome || '',
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return haystack.includes(query);
    });

    return [...result].sort((left, right) => {
      const leftTimestamp = left.lastAttemptAt ? Date.parse(left.lastAttemptAt) : 0;
      const rightTimestamp = right.lastAttemptAt ? Date.parse(right.lastAttemptAt) : 0;
      if (leftTimestamp !== rightTimestamp) return rightTimestamp - leftTimestamp;
      return left.rollupId.localeCompare(right.rollupId);
    });
  });

  const summarizeRollups = (items: ProtectionRollup[]) => {
    const nowMs = Date.now();
    const staleThreshold = nowMs - STALE_ISSUE_THRESHOLD_MS;
    const counts: Record<RecoveryOutcome, number> = {
      success: 0,
      warning: 0,
      failed: 0,
      running: 0,
      unknown: 0,
    };
    let stale = 0;
    let neverSucceeded = 0;

    for (const rollup of items) {
      counts[normalizeOutcome(rollup.lastOutcome)] += 1;
      const attemptMs = rollup.lastAttemptAt ? Date.parse(rollup.lastAttemptAt) : 0;
      const successMs = rollup.lastSuccessAt ? Date.parse(rollup.lastSuccessAt) : 0;
      if (successMs > 0) {
        if (successMs < staleThreshold) stale += 1;
      } else if (attemptMs > 0) {
        neverSucceeded += 1;
        if (attemptMs < staleThreshold) stale += 1;
      }
    }

    return { total: items.length, counts, stale, neverSucceeded };
  };

  const rollupsSummary = createMemo(() => summarizeRollups(baseRollups()));
  const overallRollupsSummary = createMemo(() => summarizeRollups(rollups()));

  const filteredRollups = createMemo<ProtectionRollup[]>(() => {
    const selectedState = protectedStateFilter();
    if (selectedState === 'all') return baseRollups();

    const nowMs = Date.now();

    return baseRollups().filter((rollup) => {
      return getRecoveryRollupInventoryStatus(rollup, nowMs) === selectedState;
    });
  });

  const filteredPoints = createMemo<RecoveryPoint[]>(() => {
    const points = recoveryPoints.points() || [];
    const selected = selectedDateKey();
    if (!selected) return points;
    const { from, to } = tableWindow();
    const fromMs = from ? Date.parse(from) : -Infinity;
    const toMs = to ? Date.parse(to) : Infinity;
    return points.filter((point) => {
      const timestamp = getRecoveryPointTimestampMs(point);
      return timestamp >= fromMs && timestamp <= toMs;
    });
  });

  const sortedPoints = createMemo<RecoveryPoint[]>(() => {
    const resourceIndex = resourcesById();
    return [...filteredPoints()].sort((left, right) => {
      const leftTimestamp = getRecoveryPointTimestampMs(left);
      const rightTimestamp = getRecoveryPointTimestampMs(right);
      if (leftTimestamp !== rightTimestamp) return rightTimestamp - leftTimestamp;
      const leftName = getRecoveryPointItemLabel(left, resourceIndex);
      const rightName = getRecoveryPointItemLabel(right, resourceIndex);
      return leftName.localeCompare(rightName);
    });
  });

  const groupedByDay = createMemo(() => {
    const groups: Array<{
      key: string;
      label: string;
      tone: 'recent' | 'default';
      items: RecoveryPoint[];
    }> = [];
    const now = new Date();
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
    const yesterday = today - 24 * 60 * 60 * 1000;

    const groupMap = new Map<
      string,
      { key: string; label: string; tone: 'recent' | 'default'; items: RecoveryPoint[] }
    >();

    for (const point of sortedPoints()) {
      const key = point.completedAt
        ? recoveryDateKeyFromTimestamp(Date.parse(point.completedAt))
        : 'unknown';
      if (!groupMap.has(key)) {
        let label = getRecoveryGroupNoTimestampLabel();
        let tone: 'recent' | 'default' = 'default';
        if (key !== 'unknown') {
          const date = parseRecoveryDateKey(key);
          const dayTimestamp = new Date(
            date.getFullYear(),
            date.getMonth(),
            date.getDate(),
          ).getTime();
          if (dayTimestamp === today) {
            label = `Today (${getRecoveryFullDateLabel(key)})`;
            tone = 'recent';
          } else if (dayTimestamp === yesterday) {
            label = `Yesterday (${getRecoveryFullDateLabel(key)})`;
            tone = 'recent';
          } else {
            label = getRecoveryFullDateLabel(key);
          }
        }
        const group = { key, label, tone, items: [] as RecoveryPoint[] };
        groupMap.set(key, group);
        groups.push(group);
      }
      groupMap.get(key)!.items.push(point);
    }

    return groups;
  });

  const hasSizeData = createMemo(() => Boolean(facets().hasSize));
  const hasVerificationData = createMemo(() => Boolean(facets().hasVerification));
  const hasClusterData = createMemo(() => (facets().clusters || []).length > 0);
  const hasNodeData = createMemo(() => (facets().nodesAgents || []).length > 0);
  const hasNamespaceData = createMemo(() => (facets().namespaces || []).length > 0);
  const hasEntityIDData = createMemo(() => Boolean(facets().hasEntityId));
  const recoveryRollupsLoaded = createMemo(() => recoveryRollups.resolvedOnce());
  const recoveryRollupsLoading = createMemo(
    () => !recoveryRollupsLoaded() || recoveryRollups.rollups.loading,
  );
  const recoverySeriesLoaded = createMemo(() => recoverySeries.resolvedOnce());
  const recoverySeriesLoading = createMemo(
    () => !recoverySeriesLoaded() || recoverySeries.response.loading,
  );
  const recoveryPointsLoaded = createMemo(() => recoveryPoints.resolvedOnce());
  const recoveryPointsLoading = createMemo(
    () => !recoveryPointsLoaded() || recoveryPoints.response.loading,
  );
  const recoveryPointsModel = {
    meta: recoveryPoints.meta,
    response: {
      get loading() {
        return recoveryPointsLoading();
      },
      get error() {
        return recoveryPoints.response.error;
      },
    },
  };

  const artifactColumns: ColumnDef[] = [
    { id: 'time', label: 'Time' },
    {
      ...createVisibleCanonicalTypeColumn(),
      label: getRecoveryArtifactColumnLabel('type', 'Type'),
    },
    { id: 'item', label: getRecoveryArtifactColumnLabel('item', 'Item') },
    { id: 'entityId', label: 'ID', toggleable: true },
    {
      id: 'cluster',
      label: getRecoveryArtifactColumnLabel('cluster', 'Cluster'),
      toggleable: true,
    },
    {
      id: 'nodeAgent',
      label: getRecoveryArtifactColumnLabel('nodeAgent', 'Node/Agent'),
      toggleable: true,
    },
    {
      id: 'namespace',
      label: getRecoveryArtifactColumnLabel('namespace', 'Namespace'),
      toggleable: true,
    },
    { id: 'platform', label: getRecoveryArtifactColumnLabel('platform', 'Platform') },
    { id: 'verified', label: 'Verified', toggleable: true },
    { id: 'size', label: 'Size', toggleable: true },
    { id: 'method', label: 'Method' },
    { id: 'repository', label: 'Target', toggleable: true },
    { id: 'details', label: 'Details', toggleable: true },
    { id: 'outcome', label: 'Outcome' },
  ];

  const relevantArtifactColumnIDs = createMemo(() => {
    const ids = new Set<string>([
      'time',
      'type',
      'item',
      'platform',
      'method',
      'repository',
      'details',
      'outcome',
    ]);
    if (hasVerificationData()) ids.add('verified');
    if (hasSizeData()) ids.add('size');
    if (hasClusterData()) ids.add('cluster');
    if (hasNodeData()) ids.add('nodeAgent');
    if (hasNamespaceData()) ids.add('namespace');
    if (hasEntityIDData()) ids.add('entityId');
    return ids;
  });

  const artifactColumnVisibility = useColumnVisibility(
    STORAGE_KEYS.RECOVERY_HIDDEN_COLUMNS,
    artifactColumns,
    ['entityId', 'cluster', 'nodeAgent', 'namespace', 'size', 'details'],
    relevantArtifactColumnIDs,
    LEGACY_RECOVERY_COLUMN_ID_ALIASES,
  );

  const visibleArtifactColumns = createMemo(() => artifactColumnVisibility.visibleColumns());
  const mobileVisibleArtifactColumns = createMemo(() =>
    isMobile()
      ? visibleArtifactColumns().filter((column) => MOBILE_RECOVERY_COLUMNS.has(column.id))
      : visibleArtifactColumns(),
  );
  const tableColumnCount = createMemo(() => mobileVisibleArtifactColumns().length);
  const tableMinWidth = createMemo(() =>
    isMobile()
      ? 'auto'
      : getRecoveryArtifactTableMinWidth(mobileVisibleArtifactColumns().map((column) => column.id)),
  );

  createEffect(() => {
    if (currentPage() > totalPages()) setCurrentPage(totalPages());
  });

  const timeline = createMemo(() => {
    const points = (recoverySeries.series() || []).map((bucket) => ({
      key: String(bucket.day || '').trim(),
      label: parseRecoveryDateKey(String(bucket.day || '').trim()).toLocaleDateString(undefined, {
        month: 'short',
        day: 'numeric',
      }),
      total: Number(bucket.total || 0),
      snapshot: Number(bucket.snapshot || 0),
      local: Number(bucket.local || 0),
      remote: Number(bucket.remote || 0),
    }));
    const maxValue = points.reduce((maximum, point) => Math.max(maximum, point.total), 0);
    const axisMax = getRecoveryNiceAxisMax(maxValue);
    const labelEvery = getRecoveryTimelineLabelEvery(points.length, isMobile());
    return { points, axisMax, labelEvery };
  });

  const activitySummary = createMemo(() => {
    const points = timeline().points;
    const totalPoints = points.reduce((sum, point) => sum + point.total, 0);
    const activeDays = points.filter((point) => point.total > 0).length;
    const averagePerDay = points.length > 0 ? totalPoints / points.length : 0;
    return { totalPoints, activeDays, averagePerDay };
  });

  const selectedDateLabel = createMemo(() => {
    const key = selectedDateKey();
    return key ? getRecoveryFilterDateLabel(key) : '';
  });

  const dayFilterOptions = createMemo<FilterSelectOption[]>(() => {
    const options = timeline().points.map((point) => ({
      value: point.key,
      label: getRecoveryTimelineDayFilterLabel(getRecoveryFilterDateLabel(point.key), point.total),
      count: point.total,
    }));

    const selected = selectedDateKey();
    if (selected && !options.some((option) => option.value === selected)) {
      options.push({
        value: selected,
        label: getRecoveryTimelineDayFilterLabel(selectedDateLabel() || selected, 0),
        count: 0,
      });
    }

    return [{ value: 'all', label: 'Any day' }, ...options];
  });
  const setActivityRange = (range: RecoveryActivityRangeDays) => {
    setChartRangeDays(range);
    setSelectedDateKey(null);
    setCurrentPage(1);
  };

  const setDayFilter = (key: string | null) => {
    batch(() => {
      setWorkspaceView('events');
      setSelectedDateKey(key);
      setCurrentPage(1);
    });
  };

  const toggleDayFilter = (key: string) => {
    setDayFilter(selectedDateKey() === key ? null : key);
  };

  const setHistorySearchFilter = (value: string) => {
    const dayKey = resolveRecoveryDateSearchKey(
      value,
      timeline().points.map((point) => point.key),
    );

    if (dayKey) {
      // Let the controlled search value observe the typed text before it is promoted to a filter.
      setQueryFilter(value);
      queueMicrotask(() => {
        batch(() => {
          setWorkspaceView('events');
          setSelectedDateKey(dayKey);
          setQueryFilter('');
          setCurrentPage(1);
        });
      });
      return;
    }

    batch(() => {
      setQueryFilter(value);
      setCurrentPage(1);
    });
  };

  const hasActiveArtifactFilters = createMemo(
    () =>
      rollupId().trim() !== '' ||
      queryFilter().trim() !== '' ||
      platformFilter() !== 'all' ||
      itemTypeFilter() !== 'all' ||
      clusterFilter() !== 'all' ||
      modeFilter() !== 'all' ||
      historyOutcomeFilter() !== 'all' ||
      scopeFilter() !== 'all' ||
      nodeFilter() !== 'all' ||
      namespaceFilter() !== 'all' ||
      verificationFilter() !== 'all' ||
      chartRangeDays() !== 30 ||
      selectedDateKey() !== null,
  );

  const activeAdvancedFilterCount = createMemo(() => {
    let count = 0;
    if (scopeFilter() !== 'all') count += 1;
    if (modeFilter() !== 'all') count += 1;
    if (verificationFilter() !== 'all') count += 1;
    if (clusterFilter() !== 'all') count += 1;
    if (nodeFilter() !== 'all') count += 1;
    if (namespaceFilter() !== 'all') count += 1;
    return count;
  });

  const resetAdvancedArtifactFilters = () => {
    batch(() => {
      setScopeFilter('all');
      setModeFilter('all');
      setVerificationFilter('all');
      setClusterFilter('all');
      setNodeFilter('all');
      setNamespaceFilter('all');
      setCurrentPage(1);
    });
  };

  const resetAllArtifactFilters = () => {
    batch(() => {
      setRollupId('');
      setQueryFilter('');
      setPlatformFilter('all');
      setItemTypeFilter('all');
      setClusterFilter('all');
      setModeFilter('all');
      setHistoryOutcomeFilter('all');
      setScopeFilter('all');
      setNodeFilter('all');
      setNamespaceFilter('all');
      setVerificationFilter('all');
      setProtectedStateFilter('all');
      setChartRangeDays(30);
      setSelectedDateKey(null);
      setCurrentPage(1);
    });
  };

  const openEventsView = () => {
    batch(() => {
      setWorkspaceView('events');
      setProtectedStateFilter('all');
      setCurrentPage(1);
    });
  };

  const openCoverageView = (state: ProtectedStateFilter = 'all') => {
    batch(() => {
      setWorkspaceView('inventory');
      setRollupId('');
      setSelectedDateKey(null);
      setHistoryOutcomeFilter('all');
      setProtectedStateFilter(state);
      setCurrentPage(1);
    });
  };

  const handleSelectRollup = (nextRollupId: string) => {
    batch(() => {
      setWorkspaceView('events');
      setRollupId(nextRollupId);
      setProtectedStateFilter('all');
      setSelectedDateKey(null);
      setCurrentPage(1);
    });
  };

  const eventsActivity = () => (
    <RecoveryActivitySection
      activitySummary={activitySummary}
      chartRangeDays={chartRangeDays}
      isMobile={isMobile()}
      loading={recoverySeriesLoading}
      onRangeChange={setActivityRange}
      overallRollupsSummary={overallRollupsSummary}
      dayFilterKey={selectedDateKey}
      timeline={timeline}
      toggleDayFilter={toggleDayFilter}
    />
  );

  const headerActionClass =
    'inline-flex items-center rounded-md border border-border bg-surface px-3 py-1.5 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover';

  return (
    <div data-testid="recovery-page" class="flex flex-col gap-2">
      <PageHeader
        title="Recovery"
        description="Review recovery activity, restore posture, and backup history across platforms."
        actions={
          <>
            <Show when={workspaceView() === 'events'}>
              <button
                type="button"
                class={headerActionClass}
                onClick={() => {
                  openCoverageView('all');
                }}
              >
                Protection coverage
              </button>
            </Show>
            <Show when={workspaceView() === 'inventory'}>
              <button type="button" class={headerActionClass} onClick={openEventsView}>
                Back to recovery events
              </button>
            </Show>
          </>
        }
      />

      <div class="flex flex-col gap-2">
        {(() => {
          return (
            <>
              <Show when={workspaceView() === 'inventory'}>
                <RecoveryProtectedInventorySection
                  filteredRollups={filteredRollups}
                  isMobile={isMobile()}
                  kioskMode={kioskMode()}
                  loading={recoveryRollupsLoading}
                  error={() => recoveryRollups.rollups.error}
                  onOpenEvents={openEventsView}
                  onSelectRollup={handleSelectRollup}
                  protectedStateFilter={protectedStateFilter}
                  itemTypeFilter={itemTypeFilter}
                  itemTypeOptions={itemTypeOptions}
                  platformFilter={platformFilter}
                  platformOptions={platformOptions}
                  queryFilter={queryFilter}
                  resourcesById={resourcesById}
                  rollups={rollups}
                  rollupsSummary={rollupsSummary}
                  setItemTypeFilter={setItemTypeFilter}
                  setProtectedStateFilter={(value: ProtectedStateFilter) =>
                    setProtectedStateFilter(value)
                  }
                  setPlatformFilter={setPlatformFilter}
                  setQueryFilter={setQueryFilter}
                  setVerificationFilter={setVerificationFilter}
                />
              </Show>

              <Show when={workspaceView() === 'events'}>
                {eventsActivity()}

                <Show when={!recoveryPointsLoading() && recoveryPoints.response.error}>
                  <TableCard>
                    <div class="p-6">
                      <EmptyState
                        title={getRecoveryPointsFailureState().title}
                        description={String(
                          (recoveryPoints.response.error as Error)?.message ||
                            recoveryPoints.response.error,
                        )}
                      />
                    </div>
                  </TableCard>
                </Show>

                <Show when={!recoveryPoints.response.error}>
                  <RecoveryHistorySection
                    activeAdvancedFilterCount={activeAdvancedFilterCount}
                    artifactColumnVisibility={artifactColumnVisibility}
                    availableOutcomes={['all', 'success', 'warning', 'failed', 'running']}
                    clusterFilter={clusterFilter}
                    clusterOptions={clusterOptions}
                    currentPage={currentPage}
                    dayFilterKey={selectedDateKey}
                    dayFilterLabel={selectedDateLabel}
                    dayFilterOptions={dayFilterOptions}
                    groupedByDay={groupedByDay}
                    hasActiveArtifactFilters={hasActiveArtifactFilters}
                    hasFocusedRollup={() => rollupId().trim().length > 0}
                    historyOutcomeFilter={historyOutcomeFilter}
                    historyItemOptions={historyItemOptions}
                    isMobile={isMobile()}
                    kioskMode={kioskMode()}
                    mobileVisibleArtifactColumns={mobileVisibleArtifactColumns}
                    modeFilter={modeFilter}
                    itemTypeFilter={itemTypeFilter}
                    itemTypeOptions={itemTypeOptions}
                    namespaceFilter={namespaceFilter}
                    namespaceOptions={namespaceOptions}
                    nodeFilter={nodeFilter}
                    nodeOptions={nodeOptions}
                    platformFilter={platformFilter}
                    platformOptions={platformOptions}
                    queryFilter={queryFilter}
                    recoveryPoints={recoveryPointsModel}
                    relatedPoints={sortedPoints}
                    resetAdvancedArtifactFilters={resetAdvancedArtifactFilters}
                    resetAllArtifactFilters={resetAllArtifactFilters}
                    resourcesById={resourcesById}
                    rollupId={rollupId}
                    scopeFilter={scopeFilter}
                    selectedHistoryItemLabel={selectedHistoryItemLabel}
                    setClusterFilter={setClusterFilter}
                    setCurrentPage={setCurrentPage}
                    setHistoryOutcomeFilter={setHistoryOutcomeFilter}
                    setItemTypeFilter={setItemTypeFilter}
                    setModeFilter={setModeFilter}
                    setNamespaceFilter={setNamespaceFilter}
                    setNodeFilter={setNodeFilter}
                    setPlatformFilter={setPlatformFilter}
                    setQueryFilter={setHistorySearchFilter}
                    setRollupId={setRollupId}
                    setScopeFilter={setScopeFilter}
                    setDayFilter={setDayFilter}
                    setVerificationFilter={setVerificationFilter}
                    showClusterFilter={showClusterFilter}
                    showNamespaceFilter={showNamespaceFilter}
                    showNodeFilter={showNodeFilter}
                    showVerificationFilter={showVerificationFilter}
                    tableColumnCount={tableColumnCount}
                    tableMinWidth={tableMinWidth}
                    totalPages={totalPages}
                    verificationFilter={verificationFilter}
                  />
                </Show>
              </Show>
            </>
          );
        })()}
      </div>
    </div>
  );
};

export default Recovery;
