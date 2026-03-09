import {
  Component,
  For,
  Show,
  createEffect,
  createMemo,
  createSignal,
  onCleanup,
  untrack,
} from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { Card } from '@/components/shared/Card';
import { ColumnPicker } from '@/components/shared/ColumnPicker';
import { EmptyState } from '@/components/shared/EmptyState';
import {
  FilterActionButton,
  FilterHeader,
  FilterMobileToggleButton,
  FilterToolbarPanel,
  LabeledFilterSelect,
  filterPanelDescriptionClass,
  filterPanelTitleClass,
  filterUtilityBadgeClass,
} from '@/components/shared/FilterToolbar';
import { SearchInput } from '@/components/shared/SearchInput';
import { getSourcePlatformBadge } from '@/components/shared/sourcePlatformBadges';
import { getSourcePlatformLabel, normalizeSourcePlatformQueryValue } from '@/utils/sourcePlatforms';
import { buildSourcePlatformOptions } from '@/utils/sourcePlatformOptions';
import { hideTooltip, showTooltip } from '@/components/shared/Tooltip';
import { useColumnVisibility } from '@/hooks/useColumnVisibility';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { formatAbsoluteTime, formatBytes, formatRelativeTime } from '@/utils/format';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { segmentedButtonClass } from '@/utils/segmentedButton';
import { useKioskMode } from '@/hooks/useKioskMode';
import { useStorageRecoveryResources } from '@/hooks/useUnifiedResources';
import { useRecoveryRollups } from '@/hooks/useRecoveryRollups';
import { useRecoveryPoints } from '@/hooks/useRecoveryPoints';
import { useRecoveryPointsFacets } from '@/hooks/useRecoveryPointsFacets';
import { useRecoveryPointsSeries } from '@/hooks/useRecoveryPointsSeries';
import { buildRecoveryPath, parseRecoveryLinkSearch } from '@/routing/resourceLinks';
import type { ProtectionRollup, RecoveryPoint } from '@/types/recovery';
import type { Resource } from '@/types/resource';
import {
  getRecoveryOutcomeBadgeClass,
  normalizeRecoveryOutcome as normalizeOutcome,
  type RecoveryOutcome,
} from '@/utils/recoveryOutcomePresentation';
import {
  getRecoveryArtifactModePresentation,
  type RecoveryArtifactMode,
} from '@/utils/recoveryArtifactModePresentation';
import {
  getRecoveryIssueRailClass,
  type RecoveryIssueTone,
} from '@/utils/recoveryIssuePresentation';
import { getRecoveryTimelineColumnButtonClass } from '@/utils/recoveryTimelinePresentation';
import { getRecoveryFilterChipPresentation } from '@/utils/recoveryFilterChipPresentation';
import {
  getRecoveryBreadcrumbLinkClass,
  getRecoveryDrawerCloseButtonClass,
  getRecoveryEmptyStateActionClass,
  getRecoveryFilterPanelClearClass,
} from '@/utils/recoveryActionPresentation';
import {
  getRecoveryPointDetailsSummary,
  getRecoveryPointRepositoryLabel,
  getRecoveryPointSubjectLabel,
  getRecoveryPointTimestampMs,
  getRecoveryRollupSubjectLabel,
  normalizeRecoveryModeQueryValue,
} from '@/utils/recoveryRecordPresentation';
import {
  formatRecoveryTimeOnly,
  getRecoveryCompactAxisLabel,
  getRecoveryFullDateLabel,
  getRecoveryNiceAxisMax,
  getRecoveryPrettyDateLabel,
  parseRecoveryDateKey,
  recoveryDateKeyFromTimestamp,
} from '@/utils/recoveryDatePresentation';
import {
  getRecoveryProtectedToggleClass,
  getRecoveryRollupStatusPillClass,
  getRecoveryRollupStatusPillLabel,
  getRecoverySpecialOutcomeTextClass,
} from '@/utils/recoveryStatusPresentation';
import {
  getRecoveryArtifactColumnHeaderClass,
  getRecoveryArtifactRowClass,
  getRecoveryEventTimeTextClass,
  getRecoveryRollupAgeTextClass,
  getRecoveryRollupIssueTone,
  getRecoverySubjectTypeBadgeClass,
  getRecoverySubjectTypeLabel,
  isRecoveryRollupStale,
  STALE_ISSUE_THRESHOLD_MS,
  RECOVERY_ADVANCED_FILTER_FIELD_CLASS,
  RECOVERY_ADVANCED_FILTER_LABEL_CLASS,
  RECOVERY_GROUP_HEADER_ROW_CLASS,
  RECOVERY_GROUP_HEADER_TEXT_CLASS,
} from '@/utils/recoveryTablePresentation';
import {
  getRecoveryTimelineAxisLabelClass,
  getRecoveryTimelineBarMinWidthClass,
  getRecoveryTimelineLabelEvery,
  RECOVERY_TIMELINE_LEGEND_ITEM_CLASS,
  RECOVERY_TIMELINE_RANGE_GROUP_CLASS,
} from '@/utils/recoveryTimelineChartPresentation';
import { RecoveryPointDetails } from '@/components/Recovery/RecoveryPointDetails';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/shared/Table';
import type { ColumnDef } from '@/hooks/useColumnVisibility';

type ArtifactMode = RecoveryArtifactMode;
type VerificationFilter = 'all' | 'verified' | 'unverified' | 'unknown';

const titleize = (value: string): string =>
  (value || '')
    .split('-')
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

const subjectMetaSlotClass = 'inline-flex shrink-0 items-center justify-end gap-1 min-w-[4.5rem]';

type IssueTone = RecoveryIssueTone;

const Recovery: Component = () => {
  const navigate = useNavigate();
  const location = useLocation();

  const kioskMode = useKioskMode();

  const storageRecoveryResources = useStorageRecoveryResources();
  const recoveryRollups = useRecoveryRollups();

  const [rollupId, setRollupId] = createSignal('');
  const [protectedQuery, setProtectedQuery] = createSignal('');
  const [protectedProviderFilter, setProtectedProviderFilter] = createSignal('all');
  const [protectedOutcomeFilter, setProtectedOutcomeFilter] =
    createSignal<'all' | RecoveryOutcome>('all');
  const [historyQuery, setHistoryQuery] = createSignal('');
  const [historyProviderFilter, setHistoryProviderFilter] = createSignal('all');
  const [clusterFilter, setClusterFilter] = createSignal('all');
  const [modeFilter, setModeFilter] = createSignal<'all' | ArtifactMode>('all');
  const [historyOutcomeFilter, setHistoryOutcomeFilter] =
    createSignal<'all' | RecoveryOutcome>('all');
  const [verificationFilter, setVerificationFilter] = createSignal<VerificationFilter>('all');
  const [scopeFilter, setScopeFilter] = createSignal<'all' | 'workload'>('all');
  const [nodeFilter, setNodeFilter] = createSignal('all');
  const [namespaceFilter, setNamespaceFilter] = createSignal('all');
  const [protectedStaleOnly, setProtectedStaleOnly] = createSignal(false);
  const [chartRangeDays, setChartRangeDays] = createSignal<7 | 30 | 90 | 365>(30);
  const [selectedDateKey, setSelectedDateKey] = createSignal<string | null>(null);
  const [currentPage, setCurrentPage] = createSignal(1);
  const [selectedPoint, setSelectedPoint] = createSignal<RecoveryPoint | null>(null);
  const [moreFiltersOpen, setMoreFiltersOpen] = createSignal(false);
  const { isMobile } = useBreakpoint();
  const [protectedFiltersOpen, setProtectedFiltersOpen] = createSignal(false);
  const [historyFiltersOpen, setHistoryFiltersOpen] = createSignal(false);
  let advancedFiltersPanelRef: HTMLDivElement | undefined;
  let advancedFiltersButtonRef: HTMLButtonElement | undefined;
  let historySectionRef: HTMLDivElement | undefined;

  type ProtectedSortCol = 'subject' | 'source' | 'lastBackup' | 'outcome';
  type SortDir = 'asc' | 'desc';
  const [protectedSortCol, setProtectedSortCol] = createSignal<ProtectedSortCol>('lastBackup');
  const [protectedSortDir, setProtectedSortDir] = createSignal<SortDir>('desc');
  const toggleProtectedSort = (col: ProtectedSortCol) => {
    if (protectedSortCol() === col) {
      setProtectedSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
    } else {
      setProtectedSortCol(col);
      setProtectedSortDir('asc');
    }
  };

  const rollups = createMemo<ProtectionRollup[]>(() => recoveryRollups.rollups() || []);
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

  const recoveryPoints = useRecoveryPoints(() => {
    const rid = rollupId().trim();
    const window = chartWindow();
    const vf = verificationFilter();
    return {
      page: currentPage(),
      limit: 200,
      rollupId: rid || null,
      provider: historyProviderFilter() === 'all' ? null : historyProviderFilter(),
      cluster: clusterFilter() === 'all' ? null : clusterFilter(),
      mode: modeFilter() === 'all' ? null : modeFilter(),
      outcome: historyOutcomeFilter() === 'all' ? null : historyOutcomeFilter(),
      q: historyQuery().trim() || null,
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
    const window = chartWindow();
    const vf = verificationFilter();
    return {
      rollupId: rid || null,
      provider: historyProviderFilter() === 'all' ? null : historyProviderFilter(),
      cluster: clusterFilter() === 'all' ? null : clusterFilter(),
      mode: modeFilter() === 'all' ? null : modeFilter(),
      outcome: historyOutcomeFilter() === 'all' ? null : historyOutcomeFilter(),
      q: historyQuery().trim() || null,
      scope: scopeFilter() === 'workload' ? 'workload' : null,
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
      provider: historyProviderFilter() === 'all' ? null : historyProviderFilter(),
      cluster: clusterFilter() === 'all' ? null : clusterFilter(),
      mode: modeFilter() === 'all' ? null : modeFilter(),
      outcome: historyOutcomeFilter() === 'all' ? null : historyOutcomeFilter(),
      q: historyQuery().trim() || null,
      node: nodeFilter() === 'all' ? null : nodeFilter(),
      namespace: namespaceFilter() === 'all' ? null : namespaceFilter(),
      scope: scopeFilter() === 'workload' ? 'workload' : null,
      verification: vf === 'all' ? null : vf,
      from: window.from,
      to: window.to,
      tzOffsetMinutes: tzOffsetMinutes(),
    };
  });

  const resourcesById = createMemo(() => {
    const map = new Map<string, Resource>();
    for (const r of storageRecoveryResources.resources() || []) {
      if (r?.id) map.set(r.id, r);
    }
    return map;
  });

  createEffect(() => {
    const parsed = parseRecoveryLinkSearch(location.search);

    const nextRollup = (parsed.rollupId || '').trim();
    const nextQuery = (parsed.query || '').trim();
    const nextProvider = normalizeSourcePlatformQueryValue(parsed.provider || '');
    const nextCluster = (parsed.cluster || 'all').trim() || 'all';
    const nextMode = normalizeRecoveryModeQueryValue(parsed.mode);
    const rawScope = (parsed.scope || '').trim().toLowerCase();
    const nextScope: 'all' | 'workload' = rawScope === 'workload' ? 'workload' : 'all';
    const nextNode = (parsed.node || 'all').trim() || 'all';
    const nextNamespace = (parsed.namespace || 'all').trim() || 'all';
    const verificationValue = (parsed.verification || '').trim().toLowerCase();
    const statusValue = (parsed.status || '').trim().toLowerCase();

    if (nextRollup !== untrack(rollupId)) setRollupId(nextRollup);
    if (nextQuery !== untrack(historyQuery)) setHistoryQuery(nextQuery);
    if (nextProvider !== untrack(historyProviderFilter))
      setHistoryProviderFilter(nextProvider || 'all');
    if (nextCluster !== untrack(clusterFilter)) setClusterFilter(nextCluster);

    if (nextMode !== untrack(modeFilter)) setModeFilter(nextMode);
    if (nextScope !== untrack(scopeFilter)) setScopeFilter(nextScope);
    if (nextNode !== untrack(nodeFilter)) setNodeFilter(nextNode);
    if (nextNamespace !== untrack(namespaceFilter)) setNamespaceFilter(nextNamespace);

    if (
      verificationValue === 'verified' ||
      verificationValue === 'unverified' ||
      verificationValue === 'unknown'
    ) {
      if (verificationValue !== untrack(verificationFilter))
        setVerificationFilter(verificationValue as VerificationFilter);
      if (untrack(historyOutcomeFilter) !== 'all') setHistoryOutcomeFilter('all');
    } else {
      if (untrack(verificationFilter) !== 'all') setVerificationFilter('all');
      const normalizedOutcome = normalizeOutcome(statusValue);
      if (statusValue && normalizedOutcome !== 'unknown') {
        if (normalizedOutcome !== untrack(historyOutcomeFilter))
          setHistoryOutcomeFilter(normalizedOutcome);
      } else if (untrack(historyOutcomeFilter) !== 'all') {
        setHistoryOutcomeFilter('all');
      }
    }
  });

  createEffect(() => {
    // Avoid leaving modals open when filters or selection change.
    rollupId();
    historyProviderFilter();
    clusterFilter();
    modeFilter();
    historyOutcomeFilter();
    verificationFilter();
    nodeFilter();
    namespaceFilter();
    currentPage();
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

  createEffect(() => {
    // Keep paging stable: any filter change resets to the first page.
    rollupId();
    historyQuery();
    historyProviderFilter();
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
    const status = historyOutcomeFilter() !== 'all' ? historyOutcomeFilter() : null;
    const verification = verificationFilter() !== 'all' ? verificationFilter() : null;

    const nextPath = buildRecoveryPath({
      rollupId: rid || null,
      query: historyQuery().trim() || null,
      provider: historyProviderFilter() !== 'all' ? historyProviderFilter() : null,
      cluster: clusterFilter() !== 'all' ? clusterFilter() : null,
      mode: modeFilter() !== 'all' ? modeFilter() : null,
      status,
      verification,
      scope: scopeFilter() === 'workload' ? 'workload' : null,
      node: nodeFilter() !== 'all' ? nodeFilter() : null,
      namespace: namespaceFilter() !== 'all' ? namespaceFilter() : null,
    });

    const currentPath = `${location.pathname}${location.search || ''}`;
    if (nextPath !== currentPath) {
      navigate(nextPath, { replace: true });
    }
  });

  const protectedProviderOptions = createMemo(() => {
    const providers = new Set<string>();
    for (const r of rollups()) {
      for (const p of r.providers || []) {
        const v = normalizeSourcePlatformQueryValue(String(p || '').trim());
        if (v) providers.add(v);
      }
    }
    return ['all', ...buildSourcePlatformOptions(providers).map((option) => option.key)];
  });

  const historyProviderOptions = createMemo(() => {
    const providers = new Set<string>();
    for (const r of rollups()) {
      for (const p of r.providers || []) {
        const v = normalizeSourcePlatformQueryValue(String(p || '').trim());
        if (v) providers.add(v);
      }
    }
    for (const p of recoveryPoints.points() || []) {
      const v = normalizeSourcePlatformQueryValue(String(p?.provider || '').trim());
      if (v) providers.add(v);
    }
    return ['all', ...buildSourcePlatformOptions(providers).map((option) => option.key)];
  });

  const baseRollups = createMemo<ProtectionRollup[]>(() => {
    const q = protectedQuery().trim().toLowerCase();
    const provider = protectedProviderFilter() === 'all' ? '' : protectedProviderFilter();
    const resIndex = resourcesById();

    const out = rollups().filter((r) => {
      const providers = (r.providers || [])
        .map((p) => normalizeSourcePlatformQueryValue(String(p || '').trim()))
        .filter(Boolean);
      if (provider && !providers.includes(provider)) return false;

      if (!q) return true;
      const label = getRecoveryRollupSubjectLabel(r, resIndex);
      const haystack = [
        r.rollupId,
        r.subjectResourceId || '',
        label,
        r.subjectRef?.type || '',
        r.subjectRef?.namespace || '',
        r.subjectRef?.name || '',
        providers.join(' '),
        r.lastOutcome || '',
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return haystack.includes(q);
    });

    return [...out].sort((a, b) => {
      const aTs = a.lastAttemptAt ? Date.parse(a.lastAttemptAt) : 0;
      const bTs = b.lastAttemptAt ? Date.parse(b.lastAttemptAt) : 0;
      if (aTs !== bTs) return bTs - aTs;
      return a.rollupId.localeCompare(b.rollupId);
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
    for (const r of items) {
      counts[normalizeOutcome(r.lastOutcome)] += 1;
      const attemptMs = r.lastAttemptAt ? Date.parse(r.lastAttemptAt) : 0;
      const successMs = r.lastSuccessAt ? Date.parse(r.lastSuccessAt) : 0;
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
    const selectedOutcome = protectedOutcomeFilter();
    const staleOnly = protectedStaleOnly();
    if (selectedOutcome === 'all' && !staleOnly) return baseRollups();

    const nowMs = Date.now();
    const staleThreshold = nowMs - STALE_ISSUE_THRESHOLD_MS;

    return (baseRollups() || []).filter((r) => {
      if (selectedOutcome !== 'all' && normalizeOutcome(r.lastOutcome) !== selectedOutcome)
        return false;
      if (!staleOnly) return true;

      const attemptMs = r.lastAttemptAt ? Date.parse(r.lastAttemptAt) : 0;
      const successMs = r.lastSuccessAt ? Date.parse(r.lastSuccessAt) : 0;
      if (successMs > 0) return successMs < staleThreshold;
      if (attemptMs > 0) return attemptMs < staleThreshold;
      return false;
    });
  });

  const sortedRollups = createMemo<ProtectionRollup[]>(() => {
    const items = (filteredRollups() || []).slice();
    const col = protectedSortCol();
    const dir = protectedSortDir();
    const resIndex = resourcesById();
    const mul = dir === 'asc' ? 1 : -1;

    items.sort((a, b) => {
      switch (col) {
        case 'subject': {
          const la = getRecoveryRollupSubjectLabel(a, resIndex).toLowerCase();
          const lb = getRecoveryRollupSubjectLabel(b, resIndex).toLowerCase();
          return mul * la.localeCompare(lb);
        }
        case 'source': {
          const sa = (a.providers || [])
            .map((p) => getSourcePlatformLabel(String(p)))
            .sort()
            .join(',');
          const sb = (b.providers || [])
            .map((p) => getSourcePlatformLabel(String(p)))
            .sort()
            .join(',');
          return mul * sa.localeCompare(sb);
        }
        case 'lastBackup': {
          const sa = a.lastSuccessAt ? Date.parse(a.lastSuccessAt) : 0;
          const sb = b.lastSuccessAt ? Date.parse(b.lastSuccessAt) : 0;
          return mul * (sa - sb);
        }
        case 'outcome': {
          const oa = normalizeOutcome(a.lastOutcome);
          const ob = normalizeOutcome(b.lastOutcome);
          return mul * oa.localeCompare(ob);
        }
        default:
          return 0;
      }
    });
    return items;
  });
  const selectedRollup = createMemo<ProtectionRollup | null>(() => {
    const rid = rollupId().trim();
    if (!rid) return null;
    return rollups().find((r) => r.rollupId === rid) || null;
  });
  const selectedHistorySubjectLabel = createMemo(() => {
    const rollup = selectedRollup();
    return rollup ? getRecoveryRollupSubjectLabel(rollup, resourcesById()) : null;
  });

  const availableOutcomes = ['all', 'success', 'warning', 'failed', 'running'] as const;

  const facets = createMemo(() => recoveryFacets.facets() || {});

  const clusterOptions = createMemo(() => {
    const values = (facets().clusters || [])
      .slice()
      .map((v) => String(v || '').trim())
      .filter(Boolean)
      .sort();
    const selected = clusterFilter().trim();
    if (selected && selected !== 'all' && !values.includes(selected)) values.unshift(selected);
    return ['all', ...values];
  });

  const nodeOptions = createMemo(() => {
    const values = (facets().nodesAgents || [])
      .slice()
      .map((v) => String(v || '').trim())
      .filter(Boolean)
      .sort();
    const selected = nodeFilter().trim();
    if (selected && selected !== 'all' && !values.includes(selected)) values.unshift(selected);
    return ['all', ...values];
  });

  const namespaceOptions = createMemo(() => {
    const values = (facets().namespaces || [])
      .slice()
      .map((v) => String(v || '').trim())
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
  const protectedActiveFilterCount = createMemo(() => {
    let count = 0;
    if (protectedQuery().trim() !== '') count++;
    if (protectedProviderFilter() !== 'all') count++;
    if (protectedOutcomeFilter() !== 'all') count++;
    if (protectedStaleOnly()) count++;
    return count;
  });
  const historyActiveFilterCount = createMemo(() => {
    let count = 0;
    if (historyQuery().trim() !== '') count++;
    if (historyProviderFilter() !== 'all') count++;
    if (historyOutcomeFilter() !== 'all') count++;
    if (scopeFilter() !== 'all') count++;
    if (modeFilter() !== 'all') count++;
    if (verificationFilter() !== 'all') count++;
    if (clusterFilter() !== 'all') count++;
    if (nodeFilter() !== 'all') count++;
    if (namespaceFilter() !== 'all') count++;
    if (selectedDateKey()) count++;
    if (chartRangeDays() !== 30) count++;
    return count;
  });

  const filteredPoints = createMemo<RecoveryPoint[]>(() => {
    const points = recoveryPoints.points() || [];
    const dateKey = selectedDateKey();
    if (!dateKey) return points;
    const { from, to } = tableWindow();
    const fromMs = Date.parse(from);
    const toMs = Date.parse(to);
    return points.filter((p) => {
      const ts = getRecoveryPointTimestampMs(p);
      return ts >= fromMs && ts <= toMs;
    });
  });

  const sortedPoints = createMemo<RecoveryPoint[]>(() => {
    const resIndex = resourcesById();
    return [...(filteredPoints() || [])].sort((a, b) => {
      const aTs = getRecoveryPointTimestampMs(a);
      const bTs = getRecoveryPointTimestampMs(b);
      if (aTs !== bTs) return bTs - aTs;
      const aName = getRecoveryPointSubjectLabel(a, resIndex);
      const bName = getRecoveryPointSubjectLabel(b, resIndex);
      return aName.localeCompare(bName);
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
    for (const p of sortedPoints()) {
      const key = p.completedAt ? recoveryDateKeyFromTimestamp(Date.parse(p.completedAt)) : 'unknown';
      if (!groupMap.has(key)) {
        let label = 'No Timestamp';
        let tone: 'recent' | 'default' = 'default';
        if (key !== 'unknown') {
          const date = parseRecoveryDateKey(key);
          const dateOnly = new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
          if (dateOnly === today) {
            label = `Today (${getRecoveryFullDateLabel(key)})`;
            tone = 'recent';
          } else if (dateOnly === yesterday) {
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
      groupMap.get(key)!.items.push(p);
    }

    return groups;
  });

  const totalPages = createMemo(() => Math.max(1, recoveryPoints.meta().totalPages || 1));

  const hasSizeData = createMemo(() => Boolean(facets().hasSize));
  const hasVerificationData = createMemo(() => Boolean(facets().hasVerification));
  const hasClusterData = createMemo(() => (facets().clusters || []).length > 0);
  const hasNodeData = createMemo(() => (facets().nodesAgents || []).length > 0);
  const hasNamespaceData = createMemo(() => (facets().namespaces || []).length > 0);
  const hasEntityIDData = createMemo(() => Boolean(facets().hasEntityId));

  const artifactColumns: ColumnDef[] = [
    { id: 'time', label: 'Time' },
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
  // Mobile: show only the 3 essential columns — secondary data is in the expand drawer.
  const MOBILE_RECOVERY_COLS = new Set(['time', 'subject', 'outcome']);
  const mobileVisibleArtifactColumns = createMemo(() =>
    isMobile()
      ? visibleArtifactColumns().filter((c) => MOBILE_RECOVERY_COLS.has(c.id))
      : visibleArtifactColumns(),
  );
  const tableColumnCount = createMemo(() => mobileVisibleArtifactColumns().length);
  const tableMinWidth = createMemo(() =>
    isMobile() ? 'auto' : `${Math.max(980, tableColumnCount() * 140)}px`,
  );

  createEffect(() => {
    if (currentPage() > totalPages()) setCurrentPage(totalPages());
  });

  const timeline = createMemo(() => {
    const series = recoverySeries.series() || [];
    const points = series.map((bucket) => {
      const key = String(bucket.day || '').trim();
      const date = parseRecoveryDateKey(key);
      return {
        key,
        label: date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' }),
        total: Number(bucket.total || 0),
        snapshot: Number(bucket.snapshot || 0),
        local: Number(bucket.local || 0),
        remote: Number(bucket.remote || 0),
      };
    });
    const maxValue = points.reduce((max, point) => Math.max(max, point.total), 0);
    const axisMax = getRecoveryNiceAxisMax(maxValue);
    const axisTicks = [0, 1, 2, 3, 4].map((step) => Math.round((axisMax * step) / 4));
    const dayCount = points.length;
    const labelEvery = getRecoveryTimelineLabelEvery(dayCount);

    return { points, maxValue, axisMax, axisTicks, labelEvery };
  });

  const activitySummary = createMemo(() => {
    const points = timeline().points;
    const totalPoints = points.reduce((sum, point) => sum + point.total, 0);
    const activeDays = points.filter((point) => point.total > 0).length;
    const averagePerDay = points.length > 0 ? totalPoints / points.length : 0;

    return {
      totalPoints,
      activeDays,
      averagePerDay,
    };
  });

  const selectedDateLabel = createMemo(() => {
    const key = selectedDateKey();
    if (!key) return '';
    const [year, month, day] = key.split('-').map((value) => Number.parseInt(value, 10));
    if (!year || !month || !day) return key;
    const date = new Date(year, month - 1, day);
    return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
  });

  const activeClusterLabel = createMemo(() => (clusterFilter() === 'all' ? '' : clusterFilter()));
  const activeNodeLabel = createMemo(() => (nodeFilter() === 'all' ? '' : nodeFilter()));
  const activeNamespaceLabel = createMemo(() =>
    namespaceFilter() === 'all' ? '' : namespaceFilter(),
  );

  const hasActiveArtifactFilters = createMemo(
    () =>
      historyQuery().trim() !== '' ||
      historyProviderFilter() !== 'all' ||
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
    setHistoryQuery('');
    setHistoryProviderFilter('all');
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

  return (
    <div data-testid="recovery-page" class="flex flex-col gap-4">
      <Card padding="none" tone="card" class="order-3 overflow-hidden">
        <div class="border-b border-border bg-surface-hover px-3 py-2">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div class="text-[11px] font-semibold uppercase tracking-wide text-muted">
              Protected Inventory
            </div>
            <div class="flex flex-wrap items-center gap-2 text-xs text-muted">
              <span>
                {filteredRollups()?.length ?? 0} of {rollups().length} items shown
              </span>
              <Show when={rollupsSummary().stale > 0}>
                <span class={getRecoveryRollupStatusPillClass('stale')}>
                  {rollupsSummary().stale} stale
                </span>
              </Show>
              <Show when={rollupsSummary().neverSucceeded > 0}>
                <span class={getRecoveryRollupStatusPillClass('never-succeeded')}>
                  {rollupsSummary().neverSucceeded} never succeeded
                </span>
              </Show>
            </div>
          </div>
        </div>
        <Show when={!kioskMode()}>
          <div class="border-b border-border px-3 py-3">
            <FilterHeader
              search={
                <SearchInput
                  value={protectedQuery}
                  onChange={(value) => setProtectedQuery(value)}
                  placeholder="Search protected items..."
                  class="w-full"
                  clearOnEscape
                  history={{
                    storageKey: STORAGE_KEYS.RECOVERY_SEARCH_HISTORY,
                    emptyMessage: 'Recent searches appear here.',
                  }}
                />
              }
              searchAccessory={
                <Show when={isMobile()}>
                  <FilterMobileToggleButton
                    onClick={() => setProtectedFiltersOpen((o) => !o)}
                    count={protectedActiveFilterCount()}
                  />
                </Show>
              }
              showFilters={!isMobile() || protectedFiltersOpen()}
              toolbarClass="lg:flex-nowrap"
            >
              <LabeledFilterSelect
                id="recovery-provider-filter"
                label="Provider"
                value={protectedProviderFilter()}
                onChange={(event) =>
                  setProtectedProviderFilter(
                    normalizeSourcePlatformQueryValue(event.currentTarget.value),
                  )
                }
                selectClass="min-w-[10rem] max-w-[14rem]"
              >
                <For each={protectedProviderOptions()}>
                  {(p) => (
                    <option value={p}>
                      {p === 'all' ? 'All Providers' : getSourcePlatformLabel(p)}
                    </option>
                  )}
                </For>
              </LabeledFilterSelect>

              <LabeledFilterSelect
                id="recovery-protected-status-filter"
                label="Latest status"
                value={protectedOutcomeFilter()}
                onChange={(event) => {
                  const value = event.currentTarget.value as 'all' | RecoveryOutcome;
                  setProtectedOutcomeFilter(value);
                }}
                selectClass="min-w-[9rem]"
              >
                <For each={availableOutcomes}>
                  {(outcome) => (
                    <option value={outcome}>
                      {outcome === 'all' ? 'Any status' : titleize(outcome)}
                    </option>
                  )}
                </For>
              </LabeledFilterSelect>

              <button
                type="button"
                aria-pressed={protectedStaleOnly()}
                onClick={() => setProtectedStaleOnly((v) => !v)}
                class={`rounded-md border px-2.5 py-1 text-xs font-medium transition-colors ${getRecoveryProtectedToggleClass(protectedStaleOnly())}`}
              >
                Stale only
              </button>
            </FilterHeader>
          </div>
        </Show>
        <Show when={recoveryRollups.rollups.loading && (filteredRollups()?.length ?? 0) === 0}>
          <div class="px-6 py-6 text-sm text-muted">Loading protected items...</div>
        </Show>

          <Show when={!recoveryRollups.rollups.loading && recoveryRollups.rollups.error}>
            <div class="p-6">
              <EmptyState
                title="Failed to load protected items"
                description={String(
                  (recoveryRollups.rollups.error as Error)?.message ||
                    recoveryRollups.rollups.error,
                )}
              />
            </div>
          </Show>

          <Show
            when={
              !recoveryRollups.rollups.loading &&
              !recoveryRollups.rollups.error &&
              (filteredRollups()?.length ?? 0) === 0
            }
          >
            <div class="p-6">
              <EmptyState
                title="No protected items yet"
                description="Pulse hasn’t observed any protected items for this org yet."
              />
            </div>
          </Show>

          <Show when={(filteredRollups()?.length ?? 0) > 0}>
            <div class="overflow-x-auto">
              <Table
                class="w-full border-collapse whitespace-nowrap"
                style={{ 'table-layout': 'fixed', 'min-width': isMobile() ? '100%' : '500px' }}
              >
                <TableHeader>
                  <TableRow class="bg-surface-alt text-muted border-b border-border">
                    {(
                      [
                        ['subject', 'Subject'],
                        ['source', 'Source'],
                        ['lastBackup', 'Last Backup'],
                        ['outcome', 'Outcome'],
                      ] as const
                    ).map(([col, label]) => (
                      <TableHead
                        class={`py-0.5 px-3 whitespace-nowrap text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer select-none hover:text-base-content transition-colors${col === 'source' ? ' hidden md:table-cell w-[110px]' : col === 'lastBackup' ? ' w-[120px]' : col === 'outcome' ? ' w-[70px]' : ''}`}
                        onClick={() => toggleProtectedSort(col)}
                      >
                        <span class="inline-flex items-center gap-1">
                          {label}
                          <Show when={protectedSortCol() === col}>
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
                    {(r) => {
                      const resIndex = resourcesById();
                      const label = getRecoveryRollupSubjectLabel(r, resIndex);
                      const attemptMs = r.lastAttemptAt ? Date.parse(r.lastAttemptAt) : 0;
                      const successMs = r.lastSuccessAt ? Date.parse(r.lastSuccessAt) : 0;
                      const outcome = normalizeOutcome(r.lastOutcome);
                      const providers = (r.providers || [])
                        .slice()
                        .map((p) => String(p || '').trim())
                        .filter(Boolean)
                        .sort((a, b) =>
                          getSourcePlatformLabel(a).localeCompare(getSourcePlatformLabel(b)),
                        );
                      const nowMs = Date.now();
                      const issueTone: IssueTone = getRecoveryRollupIssueTone(r, nowMs);
                      const issueRailClass =
                        issueTone === 'none' ? '' : getRecoveryIssueRailClass(issueTone);
                      const stale = isRecoveryRollupStale(r, nowMs);
                      const neverSucceeded =
                        (!Number.isFinite(successMs) || successMs <= 0) &&
                        Number.isFinite(attemptMs) &&
                        attemptMs > 0;
                      return (
                        <TableRow
                          class="cursor-pointer border-b border-border hover:bg-surface-hover"
                          onClick={() => {
                            setRollupId(r.rollupId);
                            requestAnimationFrame(() =>
                              historySectionRef &&
                              typeof historySectionRef.scrollIntoView === 'function'
                                ? historySectionRef.scrollIntoView({
                                    behavior: 'smooth',
                                    block: 'start',
                                  })
                                : undefined,
                            );
                          }}
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
                                {(p) => {
                                  const badge = getSourcePlatformBadge(String(p));
                                  return (
                                    <span class={badge?.classes || ''}>
                                      {badge?.label || getSourcePlatformLabel(String(p))}
                                    </span>
                                  );
                                }}
                              </For>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`whitespace-nowrap px-3 py-0.5 ${getRecoveryRollupAgeTextClass(r, nowMs)}`}
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
                              <span class={getRecoverySpecialOutcomeTextClass('never')}>
                                never
                              </span>
                            ) : (
                              '—'
                            )}
                          </TableCell>
                          <TableCell class="whitespace-nowrap px-3 py-0.5">
                            <span
                              class={`inline-flex rounded px-1.5 py-0.5 text-[10px] font-medium ${getRecoveryOutcomeBadgeClass(outcome)}`}
                            >
                              {titleize(outcome)}
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

      <div ref={historySectionRef} class="order-1 flex flex-col gap-4">
        <Show when={!recoveryPoints.response.loading && recoveryPoints.response.error}>
          <Card padding="sm">
            <EmptyState
              title="Failed to load recovery points"
              description={String(
                (recoveryPoints.response.error as Error)?.message || recoveryPoints.response.error,
              )}
            />
          </Card>
        </Show>

        <Show when={!recoveryPoints.response.error}>
          <Card padding="sm" class="h-full">
            <div class="mb-3 flex flex-col gap-3">
              <div class="flex flex-col gap-2 lg:flex-row lg:items-start lg:justify-between">
                <div class="flex flex-col gap-2">
                  <div class="flex flex-wrap items-center gap-3 text-sm">
                    <span class="font-medium text-base-content">
                      {overallRollupsSummary().total} protected
                    </span>
                    <Show when={overallRollupsSummary().total - overallRollupsSummary().stale > 0}>
                      <span class="text-emerald-600 dark:text-emerald-400">
                        {overallRollupsSummary().total - overallRollupsSummary().stale} healthy
                      </span>
                    </Show>
                    <Show when={overallRollupsSummary().stale > 0}>
                      <span class="text-amber-500">
                        {overallRollupsSummary().stale} stale
                      </span>
                    </Show>
                    <Show when={overallRollupsSummary().neverSucceeded > 0}>
                      <span class="text-rose-500">
                        {overallRollupsSummary().neverSucceeded} never succeeded
                      </span>
                    </Show>
                  </div>
                  <div class="flex flex-wrap items-baseline gap-x-2 gap-y-1">
                    <div class="text-[11px] font-semibold uppercase tracking-wide text-muted">
                      Recovery Activity
                    </div>
                    <div class="text-xs text-muted">
                      Daily recovery points across the selected history window.
                    </div>
                  </div>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <Show when={selectedHistorySubjectLabel()}>
                    <div class="inline-flex items-center gap-2 rounded-md border border-border bg-surface-alt px-2.5 py-1.5 text-xs">
                      <span class="font-semibold uppercase tracking-wide text-muted">Focused</span>
                      <span class="max-w-[18rem] truncate font-medium text-base-content">
                        {selectedHistorySubjectLabel()}
                      </span>
                    </div>
                  </Show>
                  <Show
                    when={rollupId().trim()}
                    fallback={<span class="text-xs text-muted">All protected items</span>}
                  >
                    <button
                      type="button"
                      onClick={() => setRollupId('')}
                      class={getRecoveryBreadcrumbLinkClass()}
                    >
                      All history
                    </button>
                  </Show>
                </div>
              </div>
              <div class="flex flex-wrap items-center gap-3 text-xs text-muted">
                <span>{activitySummary().totalPoints} recovery points</span>
                <span>{activitySummary().averagePerDay.toFixed(1)} per day</span>
                <span>{activitySummary().activeDays} active days</span>
                <Show when={selectedDateKey()}>
                  <span>{selectedDateLabel()}</span>
                </Show>
              </div>
            </div>

            <Show
              when={
                selectedDateKey() ||
                activeClusterLabel() ||
                activeNodeLabel() ||
                activeNamespaceLabel()
              }
            >
              <div class="mb-1 flex flex-wrap items-center gap-1.5">
                <Show when={selectedDateKey()}>
                  {(() => {
                    const chip = getRecoveryFilterChipPresentation('day');
                    return (
                      <div class={chip.className}>
                        <span class="font-medium uppercase tracking-wide">{chip.label}</span>
                        <span class="truncate font-mono text-[10px]" title={selectedDateLabel()}>
                          {selectedDateLabel()}
                        </span>
                        <button
                          type="button"
                          onClick={() => {
                            setSelectedDateKey(null);
                            setCurrentPage(1);
                          }}
                          class={chip.clearButtonClass}
                        >
                          Clear
                        </button>
                      </div>
                    );
                  })()}
                </Show>
                <Show when={activeClusterLabel()}>
                  {(() => {
                    const chip = getRecoveryFilterChipPresentation('cluster');
                    return (
                      <div class={chip.className}>
                        <span class="font-medium uppercase tracking-wide">{chip.label}</span>
                        <span class="truncate font-mono text-[10px]" title={activeClusterLabel()}>
                          {activeClusterLabel()}
                        </span>
                        <button
                          type="button"
                          onClick={() => {
                            setClusterFilter('all');
                            setCurrentPage(1);
                          }}
                          class={chip.clearButtonClass}
                        >
                          Clear
                        </button>
                      </div>
                    );
                  })()}
                </Show>
                <Show when={activeNodeLabel()}>
                  {(() => {
                    const chip = getRecoveryFilterChipPresentation('node');
                    return (
                      <div class={chip.className}>
                        <span class="font-medium uppercase tracking-wide">{chip.label}</span>
                        <span class="truncate font-mono text-[10px]" title={activeNodeLabel()}>
                          {activeNodeLabel()}
                        </span>
                        <button
                          type="button"
                          onClick={() => {
                            setNodeFilter('all');
                            setCurrentPage(1);
                          }}
                          class={chip.clearButtonClass}
                        >
                          Clear
                        </button>
                      </div>
                    );
                  })()}
                </Show>
                <Show when={activeNamespaceLabel()}>
                  {(() => {
                    const chip = getRecoveryFilterChipPresentation('namespace');
                    return (
                      <div data-testid="active-namespace-chip" class={chip.className}>
                        <span class="font-medium uppercase tracking-wide">{chip.label}</span>
                        <span
                          class="truncate font-mono text-[10px]"
                          title={activeNamespaceLabel()}
                        >
                          {activeNamespaceLabel()}
                        </span>
                        <button
                          type="button"
                          onClick={() => {
                            setNamespaceFilter('all');
                            setCurrentPage(1);
                          }}
                          class={chip.clearButtonClass}
                        >
                          Clear
                        </button>
                      </div>
                    );
                  })()}
                </Show>
              </div>
            </Show>

            <Show
              when={timeline().points.length > 0 && timeline().maxValue > 0}
              fallback={
                <div class="text-sm text-muted">
                  <Show when={recoverySeries.response.loading}>
                    <span>Loading recovery activity...</span>
                  </Show>
                  <Show when={!recoverySeries.response.loading}>
                    <span>No recovery activity in the selected window.</span>
                  </Show>
                </div>
              }
            >
              <div class="mb-1.5 flex flex-wrap items-center justify-between gap-2 text-xs text-muted">
                <div class="flex items-center gap-3">
                  <span class={RECOVERY_TIMELINE_LEGEND_ITEM_CLASS}>
                    <span
                      class={`h-2.5 w-2.5 rounded ${getRecoveryArtifactModePresentation('snapshot').segmentClassName}`}
                    />
                    {getRecoveryArtifactModePresentation('snapshot').label}
                  </span>
                  <span class={RECOVERY_TIMELINE_LEGEND_ITEM_CLASS}>
                    <span
                      class={`h-2.5 w-2.5 rounded ${getRecoveryArtifactModePresentation('local').segmentClassName}`}
                    />
                    {getRecoveryArtifactModePresentation('local').label}
                  </span>
                  <span class={RECOVERY_TIMELINE_LEGEND_ITEM_CLASS}>
                    <span
                      class={`h-2.5 w-2.5 rounded ${getRecoveryArtifactModePresentation('remote').segmentClassName}`}
                    />
                    {getRecoveryArtifactModePresentation('remote').label}
                  </span>
                </div>
                <div class={RECOVERY_TIMELINE_RANGE_GROUP_CLASS}>
                  <For each={[7, 30, 90, 365] as const}>
                    {(range) => (
                      <button
                        type="button"
                        onClick={() => {
                          setChartRangeDays(range);
                          setSelectedDateKey(null);
                          setCurrentPage(1);
                        }}
                        class={`px-2 py-1 ${segmentedButtonClass(chartRangeDays() === range, false, 'accent')}`}
                      >
                        {range === 365 ? '1y' : `${range}d`}
                      </button>
                    )}
                  </For>
                </div>
              </div>

              <div class="relative h-32 overflow-hidden rounded bg-surface-alt">
                <div class="absolute bottom-8 left-0 top-2 w-6 text-[10px] text-muted">
                  <div class="flex h-full flex-col justify-between pr-1 text-right">
                    <For each={[...timeline().axisTicks].reverse()}>
                      {(tick) => <span>{tick}</span>}
                    </For>
                  </div>
                </div>
                <div
                  class="absolute bottom-0 left-6 right-0 top-2 overflow-x-auto"
                  style="scrollbar-width: none; -ms-overflow-style: none;"
                >
                  <div class="relative h-full px-2">
                    <div class="absolute inset-x-0 bottom-6 top-0">
                      <For each={timeline().axisTicks}>
                        {(tick) => {
                          const bottom =
                            timeline().axisMax > 0 ? (tick / timeline().axisMax) * 100 : 0;
                          return (
                            <div
                              class="pointer-events-none absolute inset-x-0 border-t border-border"
                              style={{ bottom: `${bottom}%` }}
                            />
                          );
                        }}
                      </For>
                    </div>

                    <div
                      class="absolute inset-x-0 bottom-6 top-0 flex items-stretch gap-[3px]"
                      style="touch-action: none;"
                      onTouchStart={(e) => {
                        const rect = e.currentTarget.getBoundingClientRect();
                        const x = Math.max(0, e.touches[0].clientX - rect.left);
                        const pts = timeline().points;
                        const idx = Math.min(
                          Math.floor((x / rect.width) * pts.length),
                          pts.length - 1,
                        );
                        const pt = pts[idx];
                        if (pt) {
                          setSelectedDateKey(pt.key);
                          setCurrentPage(1);
                        }
                      }}
                      onTouchMove={(e) => {
                        e.preventDefault();
                        const rect = e.currentTarget.getBoundingClientRect();
                        const x = Math.max(
                          0,
                          Math.min(e.touches[0].clientX - rect.left, rect.width - 1),
                        );
                        const pts = timeline().points;
                        const idx = Math.min(
                          Math.floor((x / rect.width) * pts.length),
                          pts.length - 1,
                        );
                        const pt = pts[idx];
                        if (pt && pt.key !== selectedDateKey()) {
                          setSelectedDateKey(pt.key);
                          setCurrentPage(1);
                        }
                      }}
                    >
                      <For each={timeline().points}>
                        {(point) => {
                          const total = point.total;
                          const heightPct =
                            timeline().axisMax > 0 ? (total / timeline().axisMax) * 100 : 0;
                          const columnHeight = Math.max(0, Math.min(100, heightPct));
                          const snapshotHeight = total > 0 ? (point.snapshot / total) * 100 : 0;
                          const localHeight = total > 0 ? (point.local / total) * 100 : 0;
                          const remoteHeight = total > 0 ? (point.remote / total) * 100 : 0;
                          const isSelected = selectedDateKey() === point.key;

                          return (
                            <div class="flex-1">
                              <button
                                type="button"
                                class={`h-full w-full rounded-sm ${getRecoveryTimelineColumnButtonClass(isSelected)}`}
                                aria-label={`${getRecoveryPrettyDateLabel(point.key)}: ${total} recovery points`}
                                onClick={() => {
                                  setSelectedDateKey((prev) =>
                                    prev === point.key ? null : point.key,
                                  );
                                  setCurrentPage(1);
                                }}
                                onMouseEnter={(event) => {
                                  const rect = event.currentTarget.getBoundingClientRect();
                                  const breakdown: string[] = [];
                                  if (point.snapshot > 0)
                                    breakdown.push(`Snapshots: ${point.snapshot}`);
                                  if (point.local > 0) breakdown.push(`Local: ${point.local}`);
                                  if (point.remote > 0) breakdown.push(`Remote: ${point.remote}`);
                                  const tooltipText =
                                    point.total > 0
                                      ? `${getRecoveryPrettyDateLabel(point.key)}\nAvailable: ${point.total} recovery point${point.total > 1 ? 's' : ''}\n${breakdown.join(' • ')}`
                                      : `${getRecoveryPrettyDateLabel(point.key)}\nNo recovery points available`;
                                  showTooltip(tooltipText, rect.left + rect.width / 2, rect.top, {
                                    align: 'center',
                                    direction: 'up',
                                  });
                                }}
                                onMouseLeave={() => hideTooltip()}
                                onFocus={(event) => {
                                  const rect = event.currentTarget.getBoundingClientRect();
                                  const tooltipText = `${getRecoveryPrettyDateLabel(point.key)}\nAvailable: ${point.total} recovery point${point.total > 1 ? 's' : ''}`;
                                  showTooltip(tooltipText, rect.left + rect.width / 2, rect.top, {
                                    align: 'center',
                                    direction: 'up',
                                  });
                                }}
                                onBlur={() => hideTooltip()}
                              >
                                <div class="relative h-full w-full">
                                  <Show when={total > 0}>
                                    <div
                                      class="absolute inset-x-0 bottom-0"
                                      style={{ height: `${columnHeight}%` }}
                                    >
                                      <Show when={remoteHeight > 0}>
                                        <div
                                          class={`w-full ${getRecoveryArtifactModePresentation('remote').segmentClassName}`}
                                          style={{ height: `${remoteHeight}%` }}
                                        />
                                      </Show>
                                      <Show when={localHeight > 0}>
                                        <div
                                          class={`w-full ${getRecoveryArtifactModePresentation('local').segmentClassName}`}
                                          style={{ height: `${localHeight}%` }}
                                        />
                                      </Show>
                                      <Show when={snapshotHeight > 0}>
                                        <div
                                          class={`w-full ${getRecoveryArtifactModePresentation('snapshot').segmentClassName}`}
                                          style={{ height: `${snapshotHeight}%` }}
                                        />
                                      </Show>
                                    </div>
                                  </Show>
                                </div>
                              </button>
                            </div>
                          );
                        }}
                      </For>
                    </div>

                    <div class="pointer-events-none absolute inset-x-0 bottom-0 h-4 flex items-end gap-[3px]">
                      <For each={timeline().points}>
                        {(point, index) => {
                          const showLabel =
                            index() % timeline().labelEvery === 0 ||
                            index() === timeline().points.length - 1;
                          const isSelected = selectedDateKey() === point.key;
                          const barMinWidth = getRecoveryTimelineBarMinWidthClass(
                            isMobile(),
                            chartRangeDays(),
                          );
                          return (
                            <div
                              class={`relative flex-1 ${isMobile() ? '' : 'shrink-0'} ${barMinWidth}`}
                            >
                              <Show when={showLabel}>
                                <span
                                  class={`absolute bottom-0 left-1/2 -translate-x-1/2 whitespace-nowrap text-[9px] ${
                                    isSelected
                                      ? getRecoveryTimelineAxisLabelClass(true)
                                      : getRecoveryTimelineAxisLabelClass(false)
                                  }`}
                                >
                                  {getRecoveryCompactAxisLabel(point.key, chartRangeDays())}
                                </span>
                              </Show>
                            </div>
                          );
                        }}
                      </For>
                    </div>
                  </div>
                </div>
              </div>
            </Show>
          </Card>

          <Card padding="none" tone="card" class="mb-4 overflow-hidden">
            <div class="border-b border-border bg-surface-hover px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-muted">
              Backups By Date
            </div>
            <Show when={!kioskMode()}>
              <div class="border-b border-border px-3 py-3">
                <FilterHeader
                  search={
                    <SearchInput
                      value={historyQuery}
                      onChange={(value) => {
                        setHistoryQuery(value);
                        setCurrentPage(1);
                      }}
                      placeholder="Search recovery history..."
                      class="w-full shrink-0 sm:w-[20rem] md:w-[22rem] lg:w-[24rem] xl:w-[28rem]"
                      clearOnEscape
                      history={{
                        storageKey: STORAGE_KEYS.RECOVERY_SEARCH_HISTORY,
                        emptyMessage: 'Recent searches appear here.',
                      }}
                    />
                  }
                  searchAccessory={
                    <Show when={isMobile()}>
                      <FilterMobileToggleButton
                        onClick={() => setHistoryFiltersOpen((o) => !o)}
                        count={historyActiveFilterCount()}
                      />
                    </Show>
                  }
                  showFilters={!isMobile() || historyFiltersOpen()}
                  toolbarClass="lg:flex-nowrap"
                >
                  <LabeledFilterSelect
                    id="recovery-provider-filter-history"
                    label="History provider"
                    value={historyProviderFilter()}
                    onChange={(event) => {
                      setHistoryProviderFilter(
                        normalizeSourcePlatformQueryValue(event.currentTarget.value),
                      );
                      setCurrentPage(1);
                    }}
                    selectClass="min-w-[10rem] max-w-[14rem]"
                  >
                    <For each={historyProviderOptions()}>
                      {(p) => (
                        <option value={p}>
                          {p === 'all' ? 'All Providers' : getSourcePlatformLabel(p)}
                        </option>
                      )}
                    </For>
                  </LabeledFilterSelect>

                  <LabeledFilterSelect
                    id="recovery-status-filter"
                    label="History status"
                    value={historyOutcomeFilter()}
                    onChange={(event) => {
                      const value = event.currentTarget.value as 'all' | RecoveryOutcome;
                      setHistoryOutcomeFilter(value);
                      if (value !== 'all') setVerificationFilter('all');
                      setCurrentPage(1);
                    }}
                    selectClass="min-w-[7rem]"
                  >
                    <For each={availableOutcomes}>
                      {(outcome) => (
                        <option value={outcome}>
                          {outcome === 'all' ? 'Any status' : titleize(outcome)}
                        </option>
                      )}
                    </For>
                  </LabeledFilterSelect>

                  <div class="ml-auto flex items-center gap-2">
                    <div class="relative">
                      <FilterActionButton
                        ref={advancedFiltersButtonRef}
                        aria-label="Filter"
                        aria-expanded={moreFiltersOpen()}
                        aria-controls="recovery-filter-panel"
                        aria-haspopup="dialog"
                        onClick={() => setMoreFiltersOpen((v) => !v)}
                        active={moreFiltersOpen() || activeAdvancedFilterCount() > 0}
                      >
                        <span>Filter</span>
                        <Show when={activeAdvancedFilterCount() > 0}>
                          <span class={filterUtilityBadgeClass}>{activeAdvancedFilterCount()}</span>
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
                            <Show when={activeAdvancedFilterCount() > 0}>
                              <button
                                type="button"
                                onClick={resetAdvancedArtifactFilters}
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
                                value={scopeFilter()}
                                onChange={(event) => {
                                  const value =
                                    event.currentTarget.value === 'workload' ? 'workload' : 'all';
                                  setScopeFilter(value);
                                  setCurrentPage(1);
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
                                value={modeFilter()}
                                onChange={(event) => {
                                  setModeFilter(
                                    normalizeRecoveryModeQueryValue(event.currentTarget.value),
                                  );
                                  setCurrentPage(1);
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

                            <Show when={showVerificationFilter()}>
                              <label class="flex min-w-0 flex-col gap-1">
                                <span class={RECOVERY_ADVANCED_FILTER_LABEL_CLASS}>
                                  Verification
                                </span>
                                <select
                                  value={verificationFilter()}
                                  onChange={(event) => {
                                    setVerificationFilter(
                                      event.currentTarget.value as VerificationFilter,
                                    );
                                    if (event.currentTarget.value !== 'all')
                                      setHistoryOutcomeFilter('all');
                                    setCurrentPage(1);
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

                            <Show when={showClusterFilter()}>
                              <label class="flex min-w-0 flex-col gap-1">
                                <span class={RECOVERY_ADVANCED_FILTER_LABEL_CLASS}>Cluster</span>
                                <select
                                  value={clusterFilter()}
                                  onChange={(event) => {
                                    setClusterFilter(event.currentTarget.value);
                                    setCurrentPage(1);
                                  }}
                                  class={RECOVERY_ADVANCED_FILTER_FIELD_CLASS}
                                >
                                  <option value="all">Any cluster</option>
                                  <For each={clusterOptions().filter((value) => value !== 'all')}>
                                    {(cluster) => <option value={cluster}>{cluster}</option>}
                                  </For>
                                </select>
                              </label>
                            </Show>

                            <Show when={showNodeFilter()}>
                              <label class="flex min-w-0 flex-col gap-1">
                                <span class={RECOVERY_ADVANCED_FILTER_LABEL_CLASS}>
                                  Node or agent
                                </span>
                                <select
                                  value={nodeFilter()}
                                  onChange={(event) => {
                                    setNodeFilter(event.currentTarget.value);
                                    setCurrentPage(1);
                                  }}
                                  class={RECOVERY_ADVANCED_FILTER_FIELD_CLASS}
                                >
                                  <option value="all">Any node or agent</option>
                                  <For each={nodeOptions().filter((value) => value !== 'all')}>
                                    {(node) => <option value={node}>{node}</option>}
                                  </For>
                                </select>
                              </label>
                            </Show>

                            <Show when={showNamespaceFilter()}>
                              <label class="flex min-w-0 flex-col gap-1">
                                <span class={RECOVERY_ADVANCED_FILTER_LABEL_CLASS}>Namespace</span>
                                <select
                                  value={namespaceFilter()}
                                  onChange={(event) => {
                                    setNamespaceFilter(event.currentTarget.value);
                                    setCurrentPage(1);
                                  }}
                                  class={RECOVERY_ADVANCED_FILTER_FIELD_CLASS}
                                >
                                  <option value="all">Any namespace</option>
                                  <For each={namespaceOptions().filter((value) => value !== 'all')}>
                                    {(namespace) => <option value={namespace}>{namespace}</option>}
                                  </For>
                                </select>
                              </label>
                            </Show>
                          </div>
                        </FilterToolbarPanel>
                      </Show>
                    </div>

                    <Show when={artifactColumnVisibility.availableToggles().length > 0}>
                      <ColumnPicker
                        label="Display"
                        columns={artifactColumnVisibility.availableToggles()}
                        isHidden={artifactColumnVisibility.isHiddenByUser}
                        onToggle={artifactColumnVisibility.toggle}
                        onReset={artifactColumnVisibility.resetToDefaults}
                      />
                    </Show>

                    <Show when={hasActiveArtifactFilters()}>
                      <FilterActionButton onClick={resetAllArtifactFilters}>
                        Reset all
                      </FilterActionButton>
                    </Show>
                  </div>
                </FilterHeader>
              </div>
            </Show>
            <Show
              when={groupedByDay().length > 0}
              fallback={
                <div class="p-6">
                  <Show
                    when={recoveryPoints.response.loading}
                    fallback={
                      <EmptyState
                        title="No recovery history matches your filters"
                        description="Adjust your search, provider, method, status, or verification filters."
                        actions={
                          <Show when={hasActiveArtifactFilters()}>
                            <button
                              type="button"
                              onClick={resetAllArtifactFilters}
                              class={getRecoveryEmptyStateActionClass()}
                            >
                              Clear filters
                            </button>
                          </Show>
                        }
                      />
                    }
                  >
                    <div class="text-sm text-muted">Loading recovery points...</div>
                  </Show>
                </div>
              }
            >
              <div class="overflow-x-auto">
                <Table
                  class="w-full border-collapse text-xs whitespace-nowrap"
                  style={{ 'min-width': tableMinWidth(), 'table-layout': 'fixed' }}
                >
                  <TableHeader>
                    <TableRow class="bg-surface-alt text-muted border-b border-border">
                      <For each={mobileVisibleArtifactColumns()}>
                        {(col) => (
                          <TableHead
                            class={`py-0.5 px-3 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap ${getRecoveryArtifactColumnHeaderClass(
                              col.id,
                            )}`}
                          >
                            {col.label}
                          </TableHead>
                        )}
                      </For>
                    </TableRow>
                  </TableHeader>
                  <TableBody class="divide-y divide-border">
                    <For each={groupedByDay()}>
                      {(group) => (
                        <>
                          <TableRow class={RECOVERY_GROUP_HEADER_ROW_CLASS}>
                            <TableCell
                              colSpan={tableColumnCount()}
                              class={RECOVERY_GROUP_HEADER_TEXT_CLASS}
                            >
                              <div class="flex items-center justify-between gap-3">
                                <div class="flex min-w-0 items-center gap-2">
                                  <span class="truncate" title={group.label}>
                                    {group.label}
                                  </span>
                                  <Show when={group.tone === 'recent'}>
                                    <span class={getRecoveryRollupStatusPillClass('recent')}>
                                      {getRecoveryRollupStatusPillLabel('recent')}
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
                            {(p) => {
                              const resIndex = resourcesById();
                              const subject = getRecoveryPointSubjectLabel(p, resIndex);
                              const subjectType = getRecoverySubjectTypeLabel(p);
                              const detailsSummary = getRecoveryPointDetailsSummary(p);
                              const mode =
                                (String(p.mode || '')
                                  .trim()
                                  .toLowerCase() as ArtifactMode) || 'local';
                              const repoLabel = getRecoveryPointRepositoryLabel(p);
                              const provider = String(p.provider || '').trim();
                              const outcome = normalizeOutcome(p.outcome);
                              const completedMs = p.completedAt ? Date.parse(p.completedAt) : 0;
                              const startedMs = p.startedAt ? Date.parse(p.startedAt) : 0;
                              const tsMs = completedMs || startedMs || 0;
                              const timeOnly = formatRecoveryTimeOnly(tsMs);

                              const entityId = String(p.entityId || '').trim();
                              const cluster = String(p.cluster || '').trim();
                              const nodeAgent = String(p.node || '').trim();
                              const namespace = String(p.namespace || '').trim();

                              return (
                                <>
                                  <TableRow
                                    class={`cursor-pointer ${getRecoveryArtifactRowClass(selectedPoint()?.id === p.id)}`}
                                    onClick={() =>
                                      setSelectedPoint(selectedPoint()?.id === p.id ? null : p)
                                    }
                                  >
                                    <For each={mobileVisibleArtifactColumns()}>
                                      {(col) => {
                                        switch (col.id) {
                                          case 'time':
                                            return (
                                              <TableCell
                                                class={`whitespace-nowrap px-3 py-0.5 text-right font-mono text-[11px] tabular-nums ${getRecoveryEventTimeTextClass(tsMs)}`}
                                              >
                                                {timeOnly}
                                              </TableCell>
                                            );
                                          case 'subject':
                                            return (
                                              <TableCell
                                                class="max-w-[420px] whitespace-nowrap px-3 py-0.5 text-base-content"
                                                title={subject}
                                              >
                                                <span class="inline-flex min-w-0 max-w-full items-center gap-1.5">
                                                  <span class="min-w-0 flex-1 truncate font-medium">
                                                    {subject}
                                                  </span>
                                                  <span class={subjectMetaSlotClass}>
                                                    <Show when={subjectType}>
                                                      <span
                                                        class={`inline-flex min-w-[2.75rem] justify-center rounded px-1.5 py-px text-[9px] font-medium ${getRecoverySubjectTypeBadgeClass(
                                                          p,
                                                        )}`}
                                                      >
                                                        {subjectType}
                                                      </span>
                                                    </Show>
                                                    <Show when={p.immutable === true}>
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
                                                    <Show when={p.encrypted === true}>
                                                      <svg
                                                        class="h-3 w-3 text-amber-500 dark:text-amber-400"
                                                        fill="currentColor"
                                                        viewBox="0 0 20 20"
                                                        aria-hidden="true"
                                                      >
                                                        <path
                                                          fill-rule="evenodd"
                                                          d="M5 9V7a5 5 0 0110 0v2a2 2 0 012 2v5a2 2 0 01-2 2H5a2 2 0 01-2-2v-5a2 2 0 012-2zm8-2v2H7V7a3 3 0 016 0z"
                                                          clip-rule="evenodd"
                                                        />
                                                      </svg>
                                                    </Show>
                                                  </span>
                                                </span>
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
                                                {typeof p.verified === 'boolean' ? (
                                                  p.verified ? (
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
                                                {p.sizeBytes && p.sizeBytes > 0
                                                  ? formatBytes(p.sizeBytes)
                                                  : '—'}
                                              </TableCell>
                                            );
                                          case 'method':
                                            return (
                                              <TableCell class="whitespace-nowrap px-3 py-0.5 text-center">
                                                <span
                                                  class={`inline-flex min-w-[3.5rem] justify-center rounded px-1.5 py-px text-[9px] font-medium ${getRecoveryArtifactModePresentation(mode).badgeClassName}`}
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
                                                  {titleize(outcome)}
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
                                  <Show when={selectedPoint()?.id === p.id}>
                                    <TableRow>
                                      <TableCell
                                        colSpan={tableColumnCount()}
                                        class="bg-surface-alt px-0 sm:px-4 py-4 relative"
                                      >
                                        <div class="flex items-center justify-between px-4 pb-2 mb-2 border-b border-border">
                                          <h2 class="text-sm font-semibold text-base-content">
                                            Recovery Point Details
                                          </h2>
                                          <button
                                            type="button"
                                            onClick={(e) => {
                                              e.stopPropagation();
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
                                          <RecoveryPointDetails point={p} />
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
                    when={(recoveryPoints.meta().total || 0) > 0}
                    fallback={<span>Showing 0 of 0 recovery points</span>}
                  >
                    <span>
                      Showing {(recoveryPoints.meta().page - 1) * recoveryPoints.meta().limit + 1} -{' '}
                      {Math.min(
                        recoveryPoints.meta().page * recoveryPoints.meta().limit,
                        recoveryPoints.meta().total,
                      )}{' '}
                      of {recoveryPoints.meta().total} recovery points
                    </span>
                  </Show>
                </div>
                <div class="flex items-center gap-2">
                  <button
                    type="button"
                    disabled={currentPage() <= 1}
                    onClick={() => setCurrentPage(Math.max(1, currentPage() - 1))}
                    class="rounded-md border border-border bg-surface px-2 py-1 text-xs font-medium text-base-content disabled:opacity-50"
                  >
                    Prev
                  </button>
                  <span>
                    Page {currentPage()} / {totalPages()}
                  </span>
                  <button
                    type="button"
                    disabled={currentPage() >= totalPages()}
                    onClick={() => setCurrentPage(Math.min(totalPages(), currentPage() + 1))}
                    class="rounded-md border border-border bg-surface px-2 py-1 text-xs font-medium text-base-content disabled:opacity-50"
                  >
                    Next
                  </button>
                </div>
              </div>
            </Show>
          </Card>
        </Show>
      </div>
    </div>
  );
};

export default Recovery;
