import { Show, createEffect, createMemo, createSignal } from 'solid-js';
import type { Component } from 'solid-js';

import { RecoveryActivitySection } from '@/components/Recovery/RecoveryActivitySection';
import { RecoveryHistorySection } from '@/components/Recovery/RecoveryHistorySection';
import { RecoveryProtectedInventorySection } from '@/components/Recovery/RecoveryProtectedInventorySection';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { Subtabs } from '@/components/shared/Subtabs';
import { useRecoverySurfaceState } from '@/features/recovery/useRecoverySurfaceState';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { useColumnVisibility } from '@/hooks/useColumnVisibility';
import type { ColumnDef } from '@/hooks/useColumnVisibility';
import { useKioskMode } from '@/hooks/useKioskMode';
import type { ProtectionRollup, RecoveryOutcome, RecoveryPoint } from '@/types/recovery';
import { STORAGE_KEYS } from '@/utils/localStorage';
import {
  getRecoveryFullDateLabel,
  getRecoveryNiceAxisMax,
  parseRecoveryDateKey,
  recoveryDateKeyFromTimestamp,
} from '@/utils/recoveryDatePresentation';
import { getRecoveryPointsFailureState } from '@/utils/recoveryEmptyStatePresentation';
import { normalizeRecoveryOutcome as normalizeOutcome } from '@/utils/recoveryOutcomePresentation';
import {
  getRecoveryPointSubjectLabel,
  getRecoveryPointTimestampMs,
  getRecoveryRollupSubjectLabel,
} from '@/utils/recoveryRecordPresentation';
import {
  getRecoveryGroupNoTimestampLabel,
  getRecoveryArtifactTableMinWidth,
  STALE_ISSUE_THRESHOLD_MS,
} from '@/utils/recoveryTablePresentation';
import { getRecoveryTimelineLabelEvery } from '@/utils/recoveryTimelineChartPresentation';
import { createHiddenCanonicalTypeColumn } from '@/utils/typeColumnDefinition';

const MOBILE_RECOVERY_COLUMNS = new Set(['time', 'subject', 'outcome']);
type RecoveryWorkspaceView = 'inventory' | 'events';

const Recovery: Component = () => {
  const kioskMode = useKioskMode();
  const { isMobile } = useBreakpoint();
  let historySectionRef: HTMLDivElement | undefined;
  const [workspaceView, setWorkspaceView] = createSignal<RecoveryWorkspaceView>('inventory');
  const [workspaceViewInitialized, setWorkspaceViewInitialized] = createSignal(false);

  const {
    chartRangeDays,
    clusterFilter,
    clusterOptions,
    currentPage,
    facets,
    historyOutcomeFilter,
    modeFilter,
    namespaceFilter,
    namespaceOptions,
    nodeFilter,
    nodeOptions,
    protectedStaleOnly,
    providerFilter,
    providerOptions,
    queryFilter,
    recoveryPoints,
    recoveryRollups,
    recoverySeries,
    resourcesById,
    rollupId,
    rollups,
    scopeFilter,
    selectedDateKey,
    setChartRangeDays,
    setClusterFilter,
    setCurrentPage,
    setHistoryOutcomeFilter,
    setModeFilter,
    setNamespaceFilter,
    setNodeFilter,
    setProtectedStaleOnly,
    setProviderFilter,
    setQueryFilter,
    setRollupId,
    setScopeFilter,
    setSelectedDateKey,
    setVerificationFilter,
    showClusterFilter,
    showNamespaceFilter,
    showNodeFilter,
    showVerificationFilter,
    tableWindow,
    totalPages,
    verificationFilter,
  } = useRecoverySurfaceState();

  const baseRollups = createMemo<ProtectionRollup[]>(() => {
    const query = queryFilter().trim().toLowerCase();
    const provider = providerFilter() === 'all' ? '' : providerFilter();
    const resourceIndex = resourcesById();

    const result = rollups().filter((rollup) => {
      const providers = (rollup.providers || [])
        .map((entry) => String(entry || '').trim())
        .filter(Boolean);
      if (provider && !providers.includes(provider)) return false;

      if (!query) return true;
      const label = getRecoveryRollupSubjectLabel(rollup, resourceIndex);
      const haystack = [
        rollup.rollupId,
        rollup.subjectResourceId || '',
        label,
        rollup.subjectRef?.type || '',
        rollup.subjectRef?.namespace || '',
        rollup.subjectRef?.name || '',
        providers.join(' '),
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
    const selectedOutcome = historyOutcomeFilter();
    const staleOnly = protectedStaleOnly();
    if (selectedOutcome === 'all' && !staleOnly) return baseRollups();

    const nowMs = Date.now();
    const staleThreshold = nowMs - STALE_ISSUE_THRESHOLD_MS;

    return baseRollups().filter((rollup) => {
      if (
        selectedOutcome !== 'all' &&
        normalizeOutcome(rollup.lastOutcome) !== selectedOutcome
      ) {
        return false;
      }
      if (!staleOnly) return true;

      const attemptMs = rollup.lastAttemptAt ? Date.parse(rollup.lastAttemptAt) : 0;
      const successMs = rollup.lastSuccessAt ? Date.parse(rollup.lastSuccessAt) : 0;
      if (successMs > 0) return successMs < staleThreshold;
      if (attemptMs > 0) return attemptMs < staleThreshold;
      return false;
    });
  });

  const selectedRollup = createMemo<ProtectionRollup | null>(() => {
    const selected = rollupId().trim();
    if (!selected) return null;
    return rollups().find((rollup) => rollup.rollupId === selected) || null;
  });

  const selectedHistorySubjectLabel = createMemo(() => {
    const rollup = selectedRollup();
    return rollup ? getRecoveryRollupSubjectLabel(rollup, resourcesById()) : null;
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
      const leftName = getRecoveryPointSubjectLabel(left, resourceIndex);
      const rightName = getRecoveryPointSubjectLabel(right, resourceIndex);
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
          const dayTimestamp = new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
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

  const artifactColumns: ColumnDef[] = [
    { id: 'time', label: 'Time' },
    createHiddenCanonicalTypeColumn(),
    { id: 'subject', label: 'Subject' },
    { id: 'entityId', label: 'ID', toggleable: true },
    { id: 'cluster', label: 'Cluster', toggleable: true },
    { id: 'nodeAgent', label: 'Node/Agent', toggleable: true },
    { id: 'namespace', label: 'Namespace', toggleable: true },
    { id: 'source', label: 'Source' },
    { id: 'verified', label: 'Verified', toggleable: true },
    { id: 'size', label: 'Size', toggleable: true },
    { id: 'method', label: 'Method' },
    { id: 'repository', label: 'Target' },
    { id: 'details', label: 'Details', toggleable: true },
    { id: 'outcome', label: 'Outcome' },
  ];

  const relevantArtifactColumnIDs = createMemo(() => {
    const ids = new Set<string>([
      'time',
      'type',
      'subject',
      'source',
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
    ['entityId', 'cluster', 'nodeAgent', 'namespace'],
    relevantArtifactColumnIDs,
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

  createEffect(() => {
    const defaultView: RecoveryWorkspaceView =
      rollupId().trim().length > 0 || selectedDateKey() ? 'events' : 'inventory';
    if (!workspaceViewInitialized()) {
      setWorkspaceView(defaultView);
      setWorkspaceViewInitialized(true);
    }
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
    const labelEvery = getRecoveryTimelineLabelEvery(points.length);
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
    if (!key) return '';
    const [year, month, day] = key.split('-').map((value) => Number.parseInt(value, 10));
    if (!year || !month || !day) return key;
    return new Date(year, month - 1, day).toLocaleDateString(undefined, {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    });
  });

  const activeClusterLabel = createMemo(() => (clusterFilter() === 'all' ? '' : clusterFilter()));
  const activeNodeLabel = createMemo(() => (nodeFilter() === 'all' ? '' : nodeFilter()));
  const activeNamespaceLabel = createMemo(() =>
    namespaceFilter() === 'all' ? '' : namespaceFilter(),
  );

  const hasActiveArtifactFilters = createMemo(
    () =>
      queryFilter().trim() !== '' ||
      providerFilter() !== 'all' ||
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
    setScopeFilter('all');
    setModeFilter('all');
    setVerificationFilter('all');
    setClusterFilter('all');
    setNodeFilter('all');
    setNamespaceFilter('all');
    setCurrentPage(1);
  };

  const resetAllArtifactFilters = () => {
    setQueryFilter('');
    setProviderFilter('all');
    setClusterFilter('all');
    setModeFilter('all');
    setHistoryOutcomeFilter('all');
    setScopeFilter('all');
    setNodeFilter('all');
    setNamespaceFilter('all');
    setVerificationFilter('all');
    setChartRangeDays(30);
    setSelectedDateKey(null);
    setCurrentPage(1);
  };

  const handleSelectRollup = (nextRollupId: string) => {
    setWorkspaceView('events');
    setRollupId(nextRollupId);
    requestAnimationFrame(() =>
      historySectionRef && typeof historySectionRef.scrollIntoView === 'function'
        ? historySectionRef.scrollIntoView({ behavior: 'smooth', block: 'start' })
        : undefined,
    );
  };

  return (
    <div data-testid="recovery-page" class="flex flex-col gap-4">
      <RecoveryActivitySection
        activitySummary={activitySummary}
        activeClusterLabel={activeClusterLabel}
        activeNamespaceLabel={activeNamespaceLabel}
        activeNodeLabel={activeNodeLabel}
        chartRangeDays={chartRangeDays}
        clearClusterFilter={() => {
          setClusterFilter('all');
          setCurrentPage(1);
        }}
        clearFocusedRollup={() => setRollupId('')}
        clearNamespaceFilter={() => {
          setNamespaceFilter('all');
          setCurrentPage(1);
        }}
        clearNodeFilter={() => {
          setNodeFilter('all');
          setCurrentPage(1);
        }}
        clearSelectedDate={() => {
          setSelectedDateKey(null);
          setCurrentPage(1);
        }}
        hasFocusedRollup={() => rollupId().trim().length > 0}
        isMobile={isMobile()}
        loading={() => recoverySeries.response.loading}
        overallRollupsSummary={overallRollupsSummary}
        selectedDateKey={selectedDateKey}
        selectedDateLabel={selectedDateLabel}
        selectedHistorySubjectLabel={selectedHistorySubjectLabel}
        setChartRangeDays={(range) => {
          setChartRangeDays(range);
          setSelectedDateKey(null);
          setCurrentPage(1);
        }}
        timeline={timeline}
        toggleSelectedDate={(key) => {
          setWorkspaceView('events');
          setSelectedDateKey((previous) => (previous === key ? null : key));
          setCurrentPage(1);
        }}
      />

      <div ref={historySectionRef} class="order-1 flex flex-col gap-4">
        <Card padding="none" tone="card" class="overflow-hidden">
          <div class="px-3 pt-1">
            <Subtabs
              value={workspaceView()}
              onChange={(value) => setWorkspaceView(value as RecoveryWorkspaceView)}
              ariaLabel="Recovery data view"
              tabs={[
                {
                  value: 'inventory',
                  label: (
                    <span class="inline-flex items-center gap-2">
                      <span>Protected items</span>
                      <span class="text-xs text-muted">{filteredRollups().length}</span>
                    </span>
                  ),
                },
                {
                  value: 'events',
                  label: (
                    <span class="inline-flex items-center gap-2">
                      <span>Recovery events</span>
                      <span class="text-xs text-muted">{recoveryPoints.meta().total}</span>
                    </span>
                  ),
                },
              ]}
            />
          </div>
          <div class="border-b border-border px-3 pb-3 pt-2 text-xs text-muted">
            <Show
              when={workspaceView() === 'inventory'}
              fallback="Cross-platform recovery artifacts grouped by day and filtered through one shared recovery model."
            >
              Platform-neutral protection rollups across every connected recovery provider in the selected window.
            </Show>
          </div>
        </Card>

        <Show when={workspaceView() === 'inventory'}>
          <RecoveryProtectedInventorySection
            filteredRollups={filteredRollups}
            historyOutcomeFilter={historyOutcomeFilter}
            isMobile={isMobile()}
            kioskMode={kioskMode()}
            loading={() => recoveryRollups.rollups.loading}
            error={() => recoveryRollups.rollups.error}
            onSelectRollup={handleSelectRollup}
            protectedStaleOnly={protectedStaleOnly}
            providerFilter={providerFilter}
            providerOptions={providerOptions}
            queryFilter={queryFilter}
            resourcesById={resourcesById}
            rollups={rollups}
            rollupsSummary={rollupsSummary}
            setHistoryOutcomeFilter={setHistoryOutcomeFilter}
            setProtectedStaleOnly={setProtectedStaleOnly}
            setProviderFilter={setProviderFilter}
            setQueryFilter={setQueryFilter}
            setVerificationFilter={setVerificationFilter}
          />
        </Show>

        <Show when={workspaceView() === 'events' && !recoveryPoints.response.loading && recoveryPoints.response.error}>
          <Card padding="sm">
            <EmptyState
              title={getRecoveryPointsFailureState().title}
              description={String(
                (recoveryPoints.response.error as Error)?.message || recoveryPoints.response.error,
              )}
            />
          </Card>
        </Show>

        <Show when={workspaceView() === 'events' && !recoveryPoints.response.error}>
          <RecoveryHistorySection
            activeAdvancedFilterCount={activeAdvancedFilterCount}
            artifactColumnVisibility={artifactColumnVisibility}
            availableOutcomes={['all', 'success', 'warning', 'failed', 'running']}
            clusterFilter={clusterFilter}
            clusterOptions={clusterOptions}
            currentPage={currentPage}
            groupedByDay={groupedByDay}
            hasActiveArtifactFilters={hasActiveArtifactFilters}
            historyOutcomeFilter={historyOutcomeFilter}
            isMobile={isMobile()}
            kioskMode={kioskMode()}
            mobileVisibleArtifactColumns={mobileVisibleArtifactColumns}
            modeFilter={modeFilter}
            namespaceFilter={namespaceFilter}
            namespaceOptions={namespaceOptions}
            nodeFilter={nodeFilter}
            nodeOptions={nodeOptions}
            providerFilter={providerFilter}
            providerOptions={providerOptions}
            queryFilter={queryFilter}
            recoveryPoints={recoveryPoints}
            resetAdvancedArtifactFilters={resetAdvancedArtifactFilters}
            resetAllArtifactFilters={resetAllArtifactFilters}
            resourcesById={resourcesById}
            scopeFilter={scopeFilter}
            setClusterFilter={setClusterFilter}
            setCurrentPage={setCurrentPage}
            setHistoryOutcomeFilter={setHistoryOutcomeFilter}
            setModeFilter={setModeFilter}
            setNamespaceFilter={setNamespaceFilter}
            setNodeFilter={setNodeFilter}
            setProviderFilter={setProviderFilter}
            setQueryFilter={setQueryFilter}
            setScopeFilter={setScopeFilter}
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
      </div>
    </div>
  );
};

export default Recovery;
