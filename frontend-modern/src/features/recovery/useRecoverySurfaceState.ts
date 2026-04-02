import { createEffect, createMemo, createSignal, onCleanup, untrack } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';

import { useStorageRecoveryResources } from '@/hooks/useUnifiedResources';
import { useRecoveryRollups } from '@/hooks/useRecoveryRollups';
import { useRecoveryPoints } from '@/hooks/useRecoveryPoints';
import { useRecoveryPointsFacets } from '@/hooks/useRecoveryPointsFacets';
import { useRecoveryPointsSeries } from '@/hooks/useRecoveryPointsSeries';
import { buildRecoveryPath, parseRecoveryLinkSearch } from '@/routing/resourceLinks';
import type { ProtectionRollup, RecoveryOutcome } from '@/types/recovery';
import type { Resource } from '@/types/resource';
import { createRouteStateNavigateScheduler } from '@/utils/routeStateNavigation';
import {
  getSourcePlatformLabel,
  normalizeSourcePlatformKey,
  normalizeSourcePlatformQueryValue,
} from '@/utils/sourcePlatforms';
import { buildSourcePlatformOptions } from '@/utils/sourcePlatformOptions';
import {
  getRecoveryPointPlatform,
  getRecoveryRollupPlatforms,
} from '@/utils/recoveryPlatformModel';
import {
  getRecoveryRollupItemLabel,
  getRecoveryRollupItemSecondaryLabel,
  normalizeRecoveryModeQueryValue,
} from '@/utils/recoveryRecordPresentation';
import {
  getRecoveryPointItemTypeKey,
  getRecoveryRollupItemTypeKey,
  getRecoveryItemTypePresentation,
  normalizeRecoveryItemTypeQueryValue,
} from '@/utils/recoveryItemTypePresentation';
import { normalizeRecoveryOutcome as normalizeOutcome } from '@/utils/recoveryOutcomePresentation';
import type { RecoveryArtifactMode } from '@/utils/recoveryArtifactModePresentation';
import { parseRecoveryDateKey } from '@/utils/recoveryDatePresentation';

type ArtifactMode = RecoveryArtifactMode;
type VerificationFilter = 'all' | 'verified' | 'unverified' | 'unknown';
type RecoveryWorkspaceView = 'inventory' | 'events';

const isRecoveryDateKey = (value: string): boolean => /^\d{4}-\d{2}-\d{2}$/.test(value);
const isRecoveryRangeDays = (value: string): value is '7' | '30' | '90' | '365' =>
  value === '7' || value === '30' || value === '90' || value === '365';

const normalizeRecoveryRouteValue = (value: string | null | undefined): string =>
  String(value || '').trim();

const normalizeRecoveryWorkspaceViewValue = (
  value: string | null | undefined,
): RecoveryWorkspaceView | '' => {
  const normalized = normalizeRecoveryRouteValue(value).toLowerCase();
  if (normalized === 'inventory' || normalized === 'events') {
    return normalized;
  }
  return '';
};

const normalizeRecoveryRouteSelection = (value: string | null | undefined): string => {
  const normalized = normalizeRecoveryRouteValue(value);
  return normalized.toLowerCase() === 'all' ? 'all' : normalized;
};

const normalizeRecoveryBooleanFlag = (value: string | null | undefined): boolean => {
  const normalized = normalizeRecoveryRouteValue(value).toLowerCase();
  return normalized === '1' || normalized === 'true' || normalized === 'yes' || normalized === 'on';
};

const normalizeRecoveryPlatformSelection = (value: string | null | undefined): string => {
  const normalized = normalizeSourcePlatformQueryValue(value);
  if (!normalized || normalized === 'all') return 'all';
  return normalizeSourcePlatformKey(normalized) || 'all';
};

const normalizeRecoveryItemTypeSelection = (value: string | null | undefined): string => {
  const normalized = normalizeRecoveryItemTypeQueryValue(value);
  return normalized || 'all';
};

export function useRecoverySurfaceState() {
  const navigate = useNavigate();
  const location = useLocation();
  const routeStateNavigate = createRouteStateNavigateScheduler(
    navigate,
    () => `${untrack(() => location.pathname)}${untrack(() => location.search || '')}`,
  );

  const storageRecoveryResources = useStorageRecoveryResources();

  const [rollupId, setRollupId] = createSignal('');
  const [workspaceView, setWorkspaceView] = createSignal<RecoveryWorkspaceView>('inventory');
  const [queryFilter, setQueryFilter] = createSignal('');
  const [platformFilter, setPlatformFilter] = createSignal('all');
  const [itemTypeFilter, setItemTypeFilter] = createSignal('all');
  const [clusterFilter, setClusterFilter] = createSignal('all');
  const [modeFilter, setModeFilter] = createSignal<'all' | ArtifactMode>('all');
  const [historyOutcomeFilter, setHistoryOutcomeFilter] = createSignal<'all' | RecoveryOutcome>(
    'all',
  );
  const [verificationFilter, setVerificationFilter] = createSignal<VerificationFilter>('all');
  const [scopeFilter, setScopeFilter] = createSignal<'all' | 'workload'>('all');
  const [nodeFilter, setNodeFilter] = createSignal('all');
  const [namespaceFilter, setNamespaceFilter] = createSignal('all');
  const [protectedStaleOnly, setProtectedStaleOnly] = createSignal(false);
  const [chartRangeDays, setChartRangeDays] = createSignal<7 | 30 | 90 | 365>(30);
  const [selectedDateKey, setSelectedDateKey] = createSignal<string | null>(null);
  const [currentPage, setCurrentPage] = createSignal(1);
  const [hydratedLocationKey, setHydratedLocationKey] = createSignal('');

  const tzOffsetMinutes = createMemo(() => -new Date().getTimezoneOffset());

  const chartWindow = createMemo(() => {
    const days = chartRangeDays();
    const end = new Date();
    end.setHours(23, 59, 59, 999);
    const start = new Date(end);
    start.setDate(start.getDate() - (days - 1));
    start.setHours(0, 0, 0, 0);
    return { from: start.toISOString(), to: end.toISOString() };
  });

  const tableWindow = createMemo(() => {
    const selected = selectedDateKey();
    if (selected) {
      const start = parseRecoveryDateKey(selected);
      start.setHours(0, 0, 0, 0);
      const end = new Date(start);
      end.setHours(23, 59, 59, 999);
      return { from: start.toISOString(), to: end.toISOString() };
    }
    return chartWindow();
  });

  const recoveryRollups = useRecoveryRollups(() => {
    const rid = rollupId().trim();
    const window = chartWindow();
    const vf = verificationFilter();
    return {
      rollupId: rid || null,
      platform: platformFilter() === 'all' ? null : platformFilter(),
      itemType: itemTypeFilter() === 'all' ? null : itemTypeFilter(),
      mode: modeFilter() === 'all' ? null : modeFilter(),
      outcome: historyOutcomeFilter() === 'all' ? null : historyOutcomeFilter(),
      q: queryFilter().trim() || null,
      cluster: clusterFilter() === 'all' ? null : clusterFilter(),
      node: nodeFilter() === 'all' ? null : nodeFilter(),
      namespace: namespaceFilter() === 'all' ? null : namespaceFilter(),
      scope: scopeFilter() === 'workload' ? 'workload' : null,
      verification: vf === 'all' ? null : vf,
      from: window.from,
      to: window.to,
    };
  });

  const rollups = createMemo<ProtectionRollup[]>(() => recoveryRollups.rollups() || []);
  const selectedRollup = createMemo<ProtectionRollup | null>(() => {
    const selected = rollupId().trim();
    if (!selected) return null;
    return rollups().find((rollup) => rollup.rollupId === selected) || null;
  });

  const recoveryHistoryItemCatalog = useRecoveryRollups(() => {
    if (!rollupId().trim()) return null;
    const window = chartWindow();
    const vf = verificationFilter();
    return {
      platform: platformFilter() === 'all' ? null : platformFilter(),
      itemType: itemTypeFilter() === 'all' ? null : itemTypeFilter(),
      mode: modeFilter() === 'all' ? null : modeFilter(),
      outcome: historyOutcomeFilter() === 'all' ? null : historyOutcomeFilter(),
      q: queryFilter().trim() || null,
      cluster: clusterFilter() === 'all' ? null : clusterFilter(),
      node: nodeFilter() === 'all' ? null : nodeFilter(),
      namespace: namespaceFilter() === 'all' ? null : namespaceFilter(),
      scope: scopeFilter() === 'workload' ? 'workload' : null,
      verification: vf === 'all' ? null : vf,
      from: window.from,
      to: window.to,
    };
  });

  const selectableRollups = createMemo<ProtectionRollup[]>(() => {
    const selected = rollupId().trim();
    if (!selected) return rollups();
    const catalog = recoveryHistoryItemCatalog.rollups() || [];
    if (catalog.length > 0) return catalog;
    const focused = selectedRollup();
    return focused ? [focused] : [];
  });

  const selectedHistoryRollup = createMemo<ProtectionRollup | null>(() => {
    const selected = rollupId().trim();
    if (!selected) return null;
    return (
      selectableRollups().find((rollup) => rollup.rollupId === selected) || selectedRollup() || null
    );
  });

  const resourcesById = createMemo(() => {
    const map = new Map<string, Resource>();
    for (const resource of storageRecoveryResources.resources() || []) {
      if (resource?.id) map.set(resource.id, resource);
    }
    return map;
  });

  const selectedHistoryItemLabel = createMemo(() => {
    const rollup = selectedHistoryRollup();
    return rollup ? getRecoveryRollupItemLabel(rollup, resourcesById()) : null;
  });

  const historyItemOptions = createMemo(() => {
    const resourceIndex = resourcesById();
    const options = selectableRollups().map((rollup) => {
      const itemTypeLabel =
        getRecoveryItemTypePresentation(getRecoveryRollupItemTypeKey(rollup))?.label || null;
      const platformLabels = getRecoveryRollupPlatforms(rollup)
        .map((platform) => getSourcePlatformLabel(String(platform || '').trim()))
        .filter(Boolean)
        .sort((left, right) => left.localeCompare(right));
      return {
        rollupId: rollup.rollupId,
        label: getRecoveryRollupItemLabel(rollup, resourceIndex),
        secondaryLabel: getRecoveryRollupItemSecondaryLabel(rollup),
        contextLabel: [itemTypeLabel, platformLabels.join(', ')].filter(Boolean).join(' · '),
      };
    });

    const selected = selectedHistoryRollup();
    if (!selected) return options;
    if (options.some((option) => option.rollupId === selected.rollupId)) return options;

    const itemTypeLabel =
      getRecoveryItemTypePresentation(getRecoveryRollupItemTypeKey(selected))?.label || null;
    const platformLabels = getRecoveryRollupPlatforms(selected)
      .map((platform) => getSourcePlatformLabel(String(platform || '').trim()))
      .filter(Boolean)
      .sort((left, right) => left.localeCompare(right));

    return [
      {
        rollupId: selected.rollupId,
        label: getRecoveryRollupItemLabel(selected, resourceIndex),
        secondaryLabel: getRecoveryRollupItemSecondaryLabel(selected),
        contextLabel: [itemTypeLabel, platformLabels.join(', ')].filter(Boolean).join(' · '),
      },
      ...options,
    ];
  });

  const recoveryPoints = useRecoveryPoints(() => {
    const rid = rollupId().trim();
    const window = tableWindow();
    const vf = verificationFilter();
    return {
      page: currentPage(),
      limit: 200,
      rollupId: rid || null,
      platform: platformFilter() === 'all' ? null : platformFilter(),
      itemType: itemTypeFilter() === 'all' ? null : itemTypeFilter(),
      cluster: clusterFilter() === 'all' ? null : clusterFilter(),
      mode: modeFilter() === 'all' ? null : modeFilter(),
      outcome: historyOutcomeFilter() === 'all' ? null : historyOutcomeFilter(),
      q: queryFilter().trim() || null,
      node: nodeFilter() === 'all' ? null : nodeFilter(),
      namespace: namespaceFilter() === 'all' ? null : namespaceFilter(),
      scope: scopeFilter() === 'workload' ? 'workload' : null,
      verification: vf === 'all' ? null : vf,
      from: window.from,
      to: window.to,
    };
  });

  const recoveryFacets = useRecoveryPointsFacets(() => {
    const rid = rollupId().trim();
    const window = tableWindow();
    const vf = verificationFilter();
    return {
      rollupId: rid || null,
      platform: platformFilter() === 'all' ? null : platformFilter(),
      itemType: itemTypeFilter() === 'all' ? null : itemTypeFilter(),
      cluster: clusterFilter() === 'all' ? null : clusterFilter(),
      mode: modeFilter() === 'all' ? null : modeFilter(),
      outcome: historyOutcomeFilter() === 'all' ? null : historyOutcomeFilter(),
      q: queryFilter().trim() || null,
      scope: scopeFilter() === 'workload' ? 'workload' : null,
      node: nodeFilter() === 'all' ? null : nodeFilter(),
      namespace: namespaceFilter() === 'all' ? null : namespaceFilter(),
      verification: vf === 'all' ? null : vf,
      from: window.from,
      to: window.to,
    };
  });

  const recoverySeries = useRecoveryPointsSeries(() => {
    const rid = rollupId().trim();
    const window = chartWindow();
    const vf = verificationFilter();
    return {
      rollupId: rid || null,
      platform: platformFilter() === 'all' ? null : platformFilter(),
      itemType: itemTypeFilter() === 'all' ? null : itemTypeFilter(),
      cluster: clusterFilter() === 'all' ? null : clusterFilter(),
      mode: modeFilter() === 'all' ? null : modeFilter(),
      outcome: historyOutcomeFilter() === 'all' ? null : historyOutcomeFilter(),
      q: queryFilter().trim() || null,
      node: nodeFilter() === 'all' ? null : nodeFilter(),
      namespace: namespaceFilter() === 'all' ? null : namespaceFilter(),
      scope: scopeFilter() === 'workload' ? 'workload' : null,
      verification: vf === 'all' ? null : vf,
      from: window.from,
      to: window.to,
      tzOffsetMinutes: tzOffsetMinutes(),
    };
  });

  createEffect(() => {
    const currentPath = `${location.pathname}${location.search || ''}`;
    const parsed = parseRecoveryLinkSearch(location.search);

    const nextRollup = normalizeRecoveryRouteValue(parsed.rollupId);
    const nextView = normalizeRecoveryWorkspaceViewValue(parsed.view);
    const nextQuery = normalizeRecoveryRouteValue(parsed.query);
    const nextPlatform = normalizeRecoveryPlatformSelection(parsed.platform || '');
    const nextItemType = normalizeRecoveryItemTypeSelection(parsed.itemType || '');
    const nextStaleOnly = normalizeRecoveryBooleanFlag(parsed.stale);
    const normalizedRange = normalizeRecoveryRouteValue(parsed.range);
    const nextRange = isRecoveryRangeDays(normalizedRange) ? Number(normalizedRange) : 30;
    const nextCluster = normalizeRecoveryRouteSelection(parsed.cluster || 'all') || 'all';
    const normalizedDay = normalizeRecoveryRouteValue(parsed.day);
    const nextDay = isRecoveryDateKey(normalizedDay) ? normalizedDay : '';
    const derivedDefaultView: RecoveryWorkspaceView =
      nextRollup || nextDay ? 'events' : 'inventory';
    const resolvedView = (nextView || derivedDefaultView) as RecoveryWorkspaceView;
    const nextMode = normalizeRecoveryModeQueryValue(parsed.mode);
    const rawScope = normalizeRecoveryRouteValue(parsed.scope).toLowerCase();
    const nextScope: 'all' | 'workload' = rawScope === 'workload' ? 'workload' : 'all';
    const nextNode = normalizeRecoveryRouteSelection(parsed.node || 'all') || 'all';
    const nextNamespace = normalizeRecoveryRouteSelection(parsed.namespace || 'all') || 'all';
    const verificationValue = normalizeRecoveryRouteValue(parsed.verification).toLowerCase();
    const statusValue = normalizeRecoveryRouteValue(parsed.status).toLowerCase();

    if (nextRollup !== untrack(rollupId)) setRollupId(nextRollup);
    if (resolvedView !== untrack(workspaceView)) setWorkspaceView(resolvedView);
    if (nextQuery !== untrack(queryFilter)) setQueryFilter(nextQuery);
    if (nextPlatform !== untrack(platformFilter)) setPlatformFilter(nextPlatform);
    if (nextItemType !== untrack(itemTypeFilter)) setItemTypeFilter(nextItemType);
    if (nextStaleOnly !== untrack(protectedStaleOnly)) setProtectedStaleOnly(nextStaleOnly);
    if (nextRange !== untrack(chartRangeDays)) setChartRangeDays(nextRange as 7 | 30 | 90 | 365);
    if (nextCluster !== untrack(clusterFilter)) setClusterFilter(nextCluster);
    if ((nextDay || null) !== untrack(selectedDateKey)) setSelectedDateKey(nextDay || null);
    if (nextMode !== untrack(modeFilter)) setModeFilter(nextMode);
    if (nextScope !== untrack(scopeFilter)) setScopeFilter(nextScope);
    if (nextNode !== untrack(nodeFilter)) setNodeFilter(nextNode);
    if (nextNamespace !== untrack(namespaceFilter)) setNamespaceFilter(nextNamespace);

    if (
      verificationValue === 'verified' ||
      verificationValue === 'unverified' ||
      verificationValue === 'unknown'
    ) {
      if (verificationValue !== untrack(verificationFilter)) {
        setVerificationFilter(verificationValue as VerificationFilter);
      }
      if (untrack(historyOutcomeFilter) !== 'all') setHistoryOutcomeFilter('all');
    } else {
      if (untrack(verificationFilter) !== 'all') setVerificationFilter('all');
      const normalizedOutcome = normalizeOutcome(statusValue);
      if (statusValue && normalizedOutcome !== 'unknown') {
        if (normalizedOutcome !== untrack(historyOutcomeFilter)) {
          setHistoryOutcomeFilter(normalizedOutcome);
        }
      } else if (untrack(historyOutcomeFilter) !== 'all') {
        setHistoryOutcomeFilter('all');
      }
    }

    if (hydratedLocationKey() !== currentPath) {
      setHydratedLocationKey(currentPath);
    }
  });

  createEffect(() => {
    rollupId();
    workspaceView();
    queryFilter();
    platformFilter();
    itemTypeFilter();
    clusterFilter();
    modeFilter();
    historyOutcomeFilter();
    verificationFilter();
    nodeFilter();
    namespaceFilter();
    scopeFilter();
    chartRangeDays();
    selectedDateKey();
    if (untrack(currentPage) !== 1) setCurrentPage(1);
  });

  createEffect(() => {
    const rid = rollupId().trim();
    const defaultView: RecoveryWorkspaceView = rid || selectedDateKey() ? 'events' : 'inventory';
    const status = historyOutcomeFilter() !== 'all' ? historyOutcomeFilter() : null;
    const verification = verificationFilter() !== 'all' ? verificationFilter() : null;
    const currentPath = `${location.pathname}${location.search || ''}`;
    if (hydratedLocationKey() !== currentPath) return;

    const nextPath = buildRecoveryPath({
      rollupId: rid || null,
      view: workspaceView() !== defaultView ? workspaceView() : null,
      query: queryFilter().trim() || null,
      platform: platformFilter() !== 'all' ? platformFilter() : null,
      itemType: itemTypeFilter() !== 'all' ? itemTypeFilter() : null,
      stale: protectedStaleOnly() ? '1' : null,
      range: chartRangeDays() !== 30 ? String(chartRangeDays()) : null,
      cluster: clusterFilter() !== 'all' ? clusterFilter() : null,
      day: selectedDateKey(),
      mode: modeFilter() !== 'all' ? modeFilter() : null,
      status,
      verification,
      scope: scopeFilter() === 'workload' ? 'workload' : null,
      node: nodeFilter() !== 'all' ? nodeFilter() : null,
      namespace: namespaceFilter() !== 'all' ? namespaceFilter() : null,
    });

    if (nextPath !== currentPath) {
      routeStateNavigate.schedule(nextPath);
    }
  });

  onCleanup(() => {
    routeStateNavigate.cleanup();
  });

  const facets = createMemo(() => recoveryFacets.facets() || {});

  const platformOptions = createMemo(() => {
    const platforms = new Set<string>();
    for (const rollup of rollups()) {
      for (const platform of getRecoveryRollupPlatforms(rollup)) {
        const normalized = normalizeSourcePlatformQueryValue(String(platform || '').trim());
        if (normalized) platforms.add(normalized);
      }
    }
    for (const point of recoveryPoints.points() || []) {
      const normalized = normalizeSourcePlatformQueryValue(getRecoveryPointPlatform(point));
      if (normalized) platforms.add(normalized);
    }
    const selected = normalizeRecoveryPlatformSelection(platformFilter());
    if (selected !== 'all') platforms.add(selected);
    return ['all', ...buildSourcePlatformOptions(platforms).map((option) => option.key)];
  });

  const itemTypeOptions = createMemo(() => {
    const values = new Set<string>();

    for (const value of facets().itemTypes || []) {
      const normalized = normalizeRecoveryItemTypeQueryValue(value);
      if (normalized) values.add(normalized);
    }

    for (const rollup of rollups()) {
      const normalized = getRecoveryRollupItemTypeKey(rollup);
      if (normalized) values.add(normalized);
    }

    for (const point of recoveryPoints.points() || []) {
      const normalized = getRecoveryPointItemTypeKey(point);
      if (normalized) values.add(normalized);
    }

    const sorted = [...values].sort((left, right) => {
      const leftLabel = getRecoveryItemTypePresentation(left)?.label || left;
      const rightLabel = getRecoveryItemTypePresentation(right)?.label || right;
      return leftLabel.localeCompare(rightLabel);
    });

    const selected = itemTypeFilter().trim();
    if (selected && selected !== 'all' && !sorted.includes(selected)) sorted.unshift(selected);

    return ['all', ...sorted];
  });

  const clusterOptions = createMemo(() => {
    const values = (facets().clusters || [])
      .slice()
      .map((value) => String(value || '').trim())
      .filter(Boolean)
      .sort();
    const selected = clusterFilter().trim();
    if (selected && selected !== 'all' && !values.includes(selected)) values.unshift(selected);
    return ['all', ...values];
  });

  const nodeOptions = createMemo(() => {
    const values = (facets().nodesAgents || [])
      .slice()
      .map((value) => String(value || '').trim())
      .filter(Boolean)
      .sort();
    const selected = nodeFilter().trim();
    if (selected && selected !== 'all' && !values.includes(selected)) values.unshift(selected);
    return ['all', ...values];
  });

  const namespaceOptions = createMemo(() => {
    const values = (facets().namespaces || [])
      .slice()
      .map((value) => String(value || '').trim())
      .filter(Boolean)
      .sort();
    const selected = namespaceFilter().trim();
    if (selected && selected !== 'all' && !values.includes(selected)) values.unshift(selected);
    return ['all', ...values];
  });

  const showClusterFilter = createMemo(
    () => clusterOptions().length > 1 || clusterFilter() !== 'all',
  );
  const showNodeFilter = createMemo(() => nodeOptions().length > 1 || nodeFilter() !== 'all');
  const showNamespaceFilter = createMemo(
    () => namespaceOptions().length > 1 || namespaceFilter() !== 'all',
  );
  const showVerificationFilter = createMemo(
    () => Boolean(facets().hasVerification) || verificationFilter() !== 'all',
  );
  const totalPages = createMemo(() => Math.max(1, recoveryPoints.meta().totalPages || 1));

  return {
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
    protectedStaleOnly,
    platformFilter,
    platformOptions,
    queryFilter,
    recoveryFacets,
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
    setProtectedStaleOnly,
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
    storageRecoveryResources,
    tableWindow,
    totalPages,
    verificationFilter,
    workspaceView,
  };
}
