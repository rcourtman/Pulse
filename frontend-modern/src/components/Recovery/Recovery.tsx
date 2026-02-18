import { Component, For, Show, createEffect, createMemo, createSignal, onMount, untrack } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { Card } from '@/components/shared/Card';
import { ColumnPicker } from '@/components/shared/ColumnPicker';
import { Dialog } from '@/components/shared/Dialog';
import { EmptyState } from '@/components/shared/EmptyState';
import { SearchInput } from '@/components/shared/SearchInput';
import { getSourcePlatformBadge, getSourcePlatformLabel } from '@/components/shared/sourcePlatformBadges';
import { hideTooltip, showTooltip } from '@/components/shared/Tooltip';
import { useColumnVisibility } from '@/hooks/useColumnVisibility';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { formatAbsoluteTime, formatBytes, formatRelativeTime } from '@/utils/format';
import { STORAGE_KEYS, createLocalStorageBooleanSignal } from '@/utils/localStorage';
import { isKioskMode, subscribeToKioskMode } from '@/utils/url';
import { useStorageRecoveryResources } from '@/hooks/useUnifiedResources';
import { useRecoveryRollups } from '@/hooks/useRecoveryRollups';
import { useRecoveryPoints } from '@/hooks/useRecoveryPoints';
import { useRecoveryPointsFacets } from '@/hooks/useRecoveryPointsFacets';
import { useRecoveryPointsSeries } from '@/hooks/useRecoveryPointsSeries';
import { buildRecoveryPath, parseRecoveryLinkSearch } from '@/routing/resourceLinks';
import type { ProtectionRollup, RecoveryPoint } from '@/types/recovery';
import type { Resource } from '@/types/resource';
import { RecoveryPointDetails } from '@/components/Recovery/RecoveryPointDetails';
import type { ColumnDef } from '@/hooks/useColumnVisibility';
import ListFilterIcon from 'lucide-solid/icons/list-filter';
import { segmentedButtonClass } from '@/utils/segmentedButton';

type RecoveryView = 'protected' | 'events';

type ArtifactMode = 'snapshot' | 'local' | 'remote';
type VerificationFilter = 'all' | 'verified' | 'unverified' | 'unknown';

const STALE_ISSUE_THRESHOLD_MS = 7 * 24 * 60 * 60 * 1000;
const AGING_THRESHOLD_MS = 2 * 24 * 60 * 60 * 1000;

const sourceLabel = (value: string): string => getSourcePlatformLabel(value);

const normalizeProviderFromQuery = (value: string): string => {
  const v = (value || '').trim().toLowerCase();
  if (!v || v === 'all') return 'all';
  return v;
};

const groupHeaderRowClass = () => 'bg-gray-50 dark:bg-gray-900/40';
const groupHeaderTextClass = () =>
  'py-1 pr-2 pl-4 text-[12px] sm:text-sm font-semibold text-slate-700 dark:text-slate-300';

type KnownOutcome = 'success' | 'warning' | 'failed' | 'running' | 'unknown';

const MODE_LABELS: Record<ArtifactMode, string> = {
  snapshot: 'Snapshots',
  local: 'Local',
  remote: 'Remote',
};

const MODE_BADGE_CLASS: Record<ArtifactMode, string> = {
  snapshot: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/20 dark:text-yellow-300',
  local: 'bg-orange-100 text-orange-700 dark:bg-orange-900/50 dark:text-orange-300',
  remote: 'bg-purple-100 text-purple-700 dark:bg-purple-900/50 dark:text-purple-300',
};

const CHART_SEGMENT_CLASS: Record<ArtifactMode, string> = {
  snapshot: 'bg-yellow-500',
  local: 'bg-orange-500',
  remote: 'bg-violet-500',
};

const OUTCOME_BADGE_CLASS: Record<KnownOutcome, string> = {
  success: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
  warning: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-300',
  failed: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300',
  running: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
  unknown: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300',
};

const titleize = (value: string): string =>
  (value || '')
    .split('-')
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');


const dateKeyFromTimestamp = (timestamp: number): string => {
  const date = new Date(timestamp);
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, '0');
  const d = String(date.getDate()).padStart(2, '0');
  return `${y}-${m}-${d}`;
};

const parseDateKey = (key: string): Date => {
  const [year, month, day] = key.split('-').map((value) => Number.parseInt(value, 10));
  if (!year || !month || !day) return new Date(key);
  return new Date(year, month - 1, day);
};

const prettyDateLabel = (key: string): string =>
  parseDateKey(key).toLocaleDateString(undefined, {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
  });

const fullDateLabel = (key: string): string =>
  parseDateKey(key).toLocaleDateString(undefined, {
    weekday: 'long',
    month: 'long',
    day: 'numeric',
    year: 'numeric',
  });

const compactAxisLabel = (key: string, days: 7 | 30 | 90 | 365): string => {
  const date = parseDateKey(key);
  if (days <= 30) {
    if (date.getDate() === 1) return `${date.getMonth() + 1}/1`;
    return `${date.getDate()}`;
  }
  return `${date.getMonth() + 1}/${date.getDate()}`;
};

const formatTimeOnly = (timestamp: number | null): string => {
  if (!timestamp) return '—';
  return new Date(timestamp).toLocaleTimeString(undefined, {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  });
};

const niceAxisMax = (value: number): number => {
  if (!Number.isFinite(value) || value <= 0) return 1;
  if (value <= 5) return Math.max(1, value);
  const magnitude = 10 ** Math.floor(Math.log10(value));
  const normalized = value / magnitude;
  if (normalized <= 1) return magnitude;
  if (normalized <= 2) return 2 * magnitude;
  if (normalized <= 5) return 5 * magnitude;
  return 10 * magnitude;
};

const normalizeOutcome = (value: string | null | undefined): KnownOutcome => {
  const v = (value || '').trim().toLowerCase();
  if (v === 'success' || v === 'warning' || v === 'failed' || v === 'running' || v === 'unknown') return v;
  return 'unknown';
};

const rollupSubjectLabel = (rollup: ProtectionRollup, resourcesById: Map<string, Resource>): string => {
  const subjectRID = (rollup.subjectResourceId || '').trim();
  if (subjectRID) {
    const res = resourcesById.get(subjectRID);
    const name = (res?.name || '').trim();
    if (name) return name;
  }

  const ref = rollup.subjectRef || null;
  if (ref?.namespace && ref?.name) return `${ref.namespace}/${ref.name}`;
  if (ref?.name) return ref.name;
  if (subjectRID) return subjectRID;
  return rollup.rollupId;
};

const pointTimestampMs = (p: RecoveryPoint): number => {
  const raw = String(p.completedAt || p.startedAt || '');
  const parsed = Date.parse(raw);
  return Number.isFinite(parsed) ? parsed : 0;
};

const buildSubjectLabelForPoint = (p: RecoveryPoint, resourcesById: Map<string, Resource>): string => {
  const subjectRID = (p.subjectResourceId || '').trim();
  if (subjectRID) {
    const res = resourcesById.get(subjectRID);
    const name = (res?.name || '').trim();
    if (name) return name;
    return subjectRID;
  }

  const displayLabel = String(p.display?.subjectLabel || '').trim();
  if (displayLabel) return displayLabel;

  const ref = p.subjectRef || null;
  const ns = String(ref?.namespace || '').trim();
  const name = String(ref?.name || '').trim();
  if (ns && name) return `${ns}/${name}`;
  if (name) return name;
  const id = String(ref?.id || '').trim();
  if (id) return id;
  return p.id;
};

const buildRepositoryLabelForPoint = (p: RecoveryPoint): string => {
  const displayLabel = String(p.display?.repositoryLabel || '').trim();
  if (displayLabel) return displayLabel;

  const repo = p.repositoryRef || null;
  const repoName = String(repo?.name || '').trim();
  const repoType = String(repo?.type || '').trim();
  const repoClass = String(repo?.class || '').trim();
  if (repoName) return repoName;
  if (repoType && repoClass) return `${repoClass}:${repoType}`;
  if (repoType) return repoType;
  if (repoClass) return repoClass;
  return '';
};

const normalizeModeFromQuery = (value: string | null | undefined): 'all' | ArtifactMode => {
  const v = (value || '').trim().toLowerCase();
  if (v === 'snapshot' || v === 'local' || v === 'remote') return v;
  return 'all';
};

const rollupTimestampMs = (r: ProtectionRollup): number => {
  const raw = r.lastSuccessAt || r.lastAttemptAt || '';
  const ms = raw ? Date.parse(raw) : 0;
  return Number.isFinite(ms) ? ms : 0;
};

type IssueTone = 'none' | 'amber' | 'rose' | 'blue';

const ISSUE_RAIL_CLASS: Record<Exclude<IssueTone, 'none'>, string> = {
  amber: 'bg-amber-400',
  rose: 'bg-rose-500',
  blue: 'bg-blue-500',
};

const isRollupStale = (r: ProtectionRollup, nowMs: number): boolean => {
  const successMs = r.lastSuccessAt ? Date.parse(r.lastSuccessAt) : 0;
  if (Number.isFinite(successMs) && successMs > 0) return nowMs-successMs >= STALE_ISSUE_THRESHOLD_MS;
  const attemptMs = r.lastAttemptAt ? Date.parse(r.lastAttemptAt) : 0;
  if (Number.isFinite(attemptMs) && attemptMs > 0) return nowMs-attemptMs >= STALE_ISSUE_THRESHOLD_MS;
  return false;
};

const deriveRollupIssueTone = (r: ProtectionRollup, nowMs: number): IssueTone => {
  const outcome = normalizeOutcome(r.lastOutcome);
  if (outcome === 'failed') return 'rose';
  if (outcome === 'running') return 'blue';
  if (outcome === 'warning' || isRollupStale(r, nowMs)) return 'amber';
  return 'none';
};

const rollupAgeTextClass = (r: ProtectionRollup, nowMs: number): string => {
  const ts = rollupTimestampMs(r);
  if (!ts || ts <= 0) return 'text-gray-500 dark:text-gray-500';
  const ageMs = nowMs - ts;
  if (ageMs >= STALE_ISSUE_THRESHOLD_MS) return 'text-rose-700 dark:text-rose-300';
  if (ageMs >= AGING_THRESHOLD_MS) return 'text-amber-700 dark:text-amber-300';
  return 'text-gray-600 dark:text-gray-400';
};



const Recovery: Component = () => {
  const navigate = useNavigate();
  const location = useLocation();

  const [kioskMode, setKioskMode] = createSignal(isKioskMode());
  onMount(() => {
    const unsubscribe = subscribeToKioskMode((enabled) => {
      setKioskMode(enabled);
    });
    return unsubscribe;
  });

  const storageBackupsResources = useStorageRecoveryResources();
  const recoveryRollups = useRecoveryRollups();

  const [view, setView] = createSignal<RecoveryView>('events');
  const [rollupId, setRollupId] = createSignal('');
  const [query, setQuery] = createSignal('');
  const [providerFilter, setProviderFilter] = createSignal('all');
  const [clusterFilter, setClusterFilter] = createSignal('all');
  const [modeFilter, setModeFilter] = createSignal<'all' | ArtifactMode>('all');
  const [outcomeFilter, setOutcomeFilter] = createSignal<'all' | KnownOutcome>('all');
  const [verificationFilter, setVerificationFilter] = createSignal<VerificationFilter>('all');
  const [scopeFilter, setScopeFilter] = createSignal<'all' | 'workload'>('all');
  const [nodeFilter, setNodeFilter] = createSignal('all');
  const [namespaceFilter, setNamespaceFilter] = createSignal('all');
  const [protectedStaleOnly, setProtectedStaleOnly] = createSignal(false);
  const [chartRangeDays, setChartRangeDays] = createSignal<7 | 30 | 90 | 365>(30);
  const [selectedDateKey, setSelectedDateKey] = createSignal<string | null>(null);
  const [currentPage, setCurrentPage] = createSignal(1);
  const [selectedPoint, setSelectedPoint] = createSignal<RecoveryPoint | null>(null);
  const [moreFiltersOpen, setMoreFiltersOpen] = createLocalStorageBooleanSignal(
    STORAGE_KEYS.RECOVERY_SHOW_FILTERS,
    false,
  );
  const { isMobile } = useBreakpoint();
  const [protectedFiltersOpen, setProtectedFiltersOpen] = createSignal(false);
  const [eventsFiltersOpen, setEventsFiltersOpen] = createSignal(false);

  type ProtectedSortCol = 'subject' | 'source' | 'lastBackup' | 'outcome';
  type SortDir = 'asc' | 'desc';
  const [protectedSortCol, setProtectedSortCol] = createSignal<ProtectedSortCol>('lastBackup');
  const [protectedSortDir, setProtectedSortDir] = createSignal<SortDir>('asc');
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
      const start = parseDateKey(selected);
      start.setHours(0, 0, 0, 0);
      const end = new Date(start);
      end.setHours(23, 59, 59, 999);
      return { from: start.toISOString(), to: end.toISOString() };
    }
    return chartWindow();
  });

  const recoveryPoints = useRecoveryPoints(() => {
    if (view() !== 'events') return null;
    const rid = rollupId().trim();
    const window = tableWindow();
    const vf = verificationFilter();
    return {
      page: currentPage(),
      limit: 200,
      rollupId: rid || null,
      provider: providerFilter() === 'all' ? null : providerFilter(),
      cluster: clusterFilter() === 'all' ? null : clusterFilter(),
      mode: modeFilter() === 'all' ? null : modeFilter(),
      outcome: outcomeFilter() === 'all' ? null : outcomeFilter(),
      q: query().trim() || null,
      node: nodeFilter() === 'all' ? null : nodeFilter(),
      namespace: namespaceFilter() === 'all' ? null : namespaceFilter(),
      scope: scopeFilter() === 'workload' ? 'workload' : null,
      verification: vf === 'all' ? null : vf,
      from: window.from,
      to: window.to,
    };
  });

  const recoveryFacets = useRecoveryPointsFacets(() => {
    if (view() !== 'events') return null;
    const rid = rollupId().trim();
    const window = chartWindow();
    const vf = verificationFilter();
    return {
      rollupId: rid || null,
      provider: providerFilter() === 'all' ? null : providerFilter(),
      cluster: clusterFilter() === 'all' ? null : clusterFilter(),
      mode: modeFilter() === 'all' ? null : modeFilter(),
      outcome: outcomeFilter() === 'all' ? null : outcomeFilter(),
      q: query().trim() || null,
      scope: scopeFilter() === 'workload' ? 'workload' : null,
      verification: vf === 'all' ? null : vf,
      from: window.from,
      to: window.to,
    };
  });

  const recoverySeries = useRecoveryPointsSeries(() => {
    if (view() !== 'events') return null;
    const rid = rollupId().trim();
    const window = chartWindow();
    const vf = verificationFilter();
    return {
      rollupId: rid || null,
      provider: providerFilter() === 'all' ? null : providerFilter(),
      cluster: clusterFilter() === 'all' ? null : clusterFilter(),
      mode: modeFilter() === 'all' ? null : modeFilter(),
      outcome: outcomeFilter() === 'all' ? null : outcomeFilter(),
      q: query().trim() || null,
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
    for (const r of storageBackupsResources.resources() || []) {
      if (r?.id) map.set(r.id, r);
    }
    return map;
  });

  createEffect(() => {
    const parsed = parseRecoveryLinkSearch(location.search);

    const rawView = (parsed.view || '').trim().toLowerCase();
    const nextView: RecoveryView = rawView === 'protected' ? 'protected' : 'events';
    const nextRollup = (parsed.rollupId || '').trim();
    const nextQuery = (parsed.query || '').trim();
    const nextProvider = normalizeProviderFromQuery(parsed.provider || '');
    const nextCluster = (parsed.cluster || 'all').trim() || 'all';
    const nextMode = normalizeModeFromQuery(parsed.mode);
    const rawScope = (parsed.scope || '').trim().toLowerCase();
    const nextScope: 'all' | 'workload' = rawScope === 'workload' ? 'workload' : 'all';
    const nextNode = (parsed.node || 'all').trim() || 'all';
    const nextNamespace = (parsed.namespace || 'all').trim() || 'all';
    const verificationValue = (parsed.verification || '').trim().toLowerCase();
    const statusValue = (parsed.status || '').trim().toLowerCase();

    if (nextView !== untrack(view)) setView(nextView);
    if (nextRollup !== untrack(rollupId)) setRollupId(nextRollup);
    if (nextQuery !== untrack(query)) setQuery(nextQuery);
    if (nextProvider !== untrack(providerFilter)) setProviderFilter(nextProvider || 'all');
    if (nextCluster !== untrack(clusterFilter)) setClusterFilter(nextCluster);

    if (nextMode !== untrack(modeFilter)) setModeFilter(nextMode);
    if (nextScope !== untrack(scopeFilter)) setScopeFilter(nextScope);
    if (nextNode !== untrack(nodeFilter)) setNodeFilter(nextNode);
    if (nextNamespace !== untrack(namespaceFilter)) setNamespaceFilter(nextNamespace);

    if (verificationValue === 'verified' || verificationValue === 'unverified' || verificationValue === 'unknown') {
      if (verificationValue !== untrack(verificationFilter)) setVerificationFilter(verificationValue as VerificationFilter);
      if (untrack(outcomeFilter) !== 'all') setOutcomeFilter('all');
    } else {
      if (untrack(verificationFilter) !== 'all') setVerificationFilter('all');
      const normalizedOutcome = normalizeOutcome(statusValue);
      if (statusValue && normalizedOutcome !== 'unknown') {
        if (normalizedOutcome !== untrack(outcomeFilter)) setOutcomeFilter(normalizedOutcome);
      } else if (untrack(outcomeFilter) !== 'all') {
        setOutcomeFilter('all');
      }
    }
  });

  createEffect(() => {
    // Avoid leaving modals open when switching views/filters.
    view();
    rollupId();
    providerFilter();
    clusterFilter();
    modeFilter();
    outcomeFilter();
    verificationFilter();
    nodeFilter();
    namespaceFilter();
    currentPage();
    setSelectedPoint(null);
    if (view() !== 'events' && untrack(moreFiltersOpen)) setMoreFiltersOpen(false);
  });

  createEffect(() => {
    // Keep paging stable: any filter change resets to the first page.
    if (view() !== 'events') return;
    rollupId();
    query();
    providerFilter();
    clusterFilter();
    modeFilter();
    outcomeFilter();
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
    const v = view();

    const status = outcomeFilter() !== 'all' ? outcomeFilter() : null;
    const verification = v === 'events' && verificationFilter() !== 'all' ? verificationFilter() : null;

    const nextPath = buildRecoveryPath({
      view: v,
      rollupId: v === 'events' && rid ? rid : null,
      query: query().trim() || null,
      provider: providerFilter() !== 'all' ? providerFilter() : null,
      cluster: v === 'events' && clusterFilter() !== 'all' ? clusterFilter() : null,
      mode: v === 'events' && modeFilter() !== 'all' ? modeFilter() : null,
      status,
      verification,
      scope: v === 'events' && scopeFilter() === 'workload' ? 'workload' : null,
      node: v === 'events' && nodeFilter() !== 'all' ? nodeFilter() : null,
      namespace: v === 'events' && namespaceFilter() !== 'all' ? namespaceFilter() : null,
    });

    const currentPath = `${location.pathname}${location.search || ''}`;
    if (nextPath !== currentPath) {
      navigate(nextPath, { replace: true });
    }
  });

  const providerOptions = createMemo(() => {
    const providers = new Set<string>();
    for (const r of rollups()) {
      for (const p of r.providers || []) {
        const v = String(p || '').trim();
        if (v) providers.add(v);
      }
    }
    for (const p of recoveryPoints.points() || []) {
      const v = String(p?.provider || '').trim();
      if (v) providers.add(v);
    }
    const values = Array.from(providers).sort((a, b) => sourceLabel(a).localeCompare(sourceLabel(b)));
    return ['all', ...values];
  });

  const baseRollups = createMemo<ProtectionRollup[]>(() => {
    const q = query().trim().toLowerCase();
    const provider = providerFilter() === 'all' ? '' : providerFilter();
    const resIndex = resourcesById();

    const out = rollups().filter((r) => {
      const providers = (r.providers || []).map((p) => String(p || '').trim()).filter(Boolean);
      if (provider && !providers.includes(provider)) return false;

      if (!q) return true;
      const label = rollupSubjectLabel(r, resIndex);
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

  const rollupsSummary = createMemo(() => {
    const items = baseRollups();
    const nowMs = Date.now();
    const staleThreshold = nowMs - STALE_ISSUE_THRESHOLD_MS;
    const counts: Record<KnownOutcome, number> = {
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
  });

  const filteredRollups = createMemo<ProtectionRollup[]>(() => {
    const selectedOutcome = outcomeFilter();
    const staleOnly = protectedStaleOnly();
    if (selectedOutcome === 'all' && !staleOnly) return baseRollups();

    const nowMs = Date.now();
    const staleThreshold = nowMs - STALE_ISSUE_THRESHOLD_MS;

    return baseRollups().filter((r) => {
      if (selectedOutcome !== 'all' && normalizeOutcome(r.lastOutcome) !== selectedOutcome) return false;
      if (!staleOnly) return true;

      const attemptMs = r.lastAttemptAt ? Date.parse(r.lastAttemptAt) : 0;
      const successMs = r.lastSuccessAt ? Date.parse(r.lastSuccessAt) : 0;
      if (successMs > 0) return successMs < staleThreshold;
      if (attemptMs > 0) return attemptMs < staleThreshold;
      return false;
    });
  });

  const sortedRollups = createMemo<ProtectionRollup[]>(() => {
    const items = filteredRollups().slice();
    const col = protectedSortCol();
    const dir = protectedSortDir();
    const resIndex = resourcesById();
    const mul = dir === 'asc' ? 1 : -1;

    items.sort((a, b) => {
      switch (col) {
        case 'subject': {
          const la = rollupSubjectLabel(a, resIndex).toLowerCase();
          const lb = rollupSubjectLabel(b, resIndex).toLowerCase();
          return mul * la.localeCompare(lb);
        }
        case 'source': {
          const sa = (a.providers || []).map((p) => sourceLabel(String(p))).sort().join(',');
          const sb = (b.providers || []).map((p) => sourceLabel(String(p))).sort().join(',');
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

  const availableOutcomes = ['all', 'success', 'warning', 'failed', 'running'] as const;

  const facets = createMemo(() => recoveryFacets.facets() || {});

  const clusterOptions = createMemo(() => {
    const values = (facets().clusters || []).slice().map((v) => String(v || '').trim()).filter(Boolean).sort();
    const selected = clusterFilter().trim();
    if (selected && selected !== 'all' && !values.includes(selected)) values.unshift(selected);
    return ['all', ...values];
  });

  const nodeOptions = createMemo(() => {
    const values = (facets().nodesHosts || []).slice().map((v) => String(v || '').trim()).filter(Boolean).sort();
    const selected = nodeFilter().trim();
    if (selected && selected !== 'all' && !values.includes(selected)) values.unshift(selected);
    return ['all', ...values];
  });

  const namespaceOptions = createMemo(() => {
    const values = (facets().namespaces || []).slice().map((v) => String(v || '').trim()).filter(Boolean).sort();
    const selected = namespaceFilter().trim();
    if (selected && selected !== 'all' && !values.includes(selected)) values.unshift(selected);
    return ['all', ...values];
  });

  const showClusterFilter = createMemo(() => clusterOptions().length > 1 || clusterFilter() !== 'all');
  const showNodeFilter = createMemo(() => nodeOptions().length > 1 || nodeFilter() !== 'all');
  const showNamespaceFilter = createMemo(() => namespaceOptions().length > 1 || namespaceFilter() !== 'all');
  const showVerificationFilter = createMemo(() => Boolean(facets().hasVerification) || verificationFilter() !== 'all');
  const activeAdvancedFilterCount = createMemo(() => {
    let count = 0;
    if (modeFilter() !== 'all') count += 1;
    if (verificationFilter() !== 'all') count += 1;
    if (clusterFilter() !== 'all') count += 1;
    if (nodeFilter() !== 'all') count += 1;
    if (namespaceFilter() !== 'all') count += 1;
    return count;
  });
  const protectedActiveFilterCount = createMemo(() => {
    let count = 0;
    if (providerFilter() !== 'all') count++;
    if (outcomeFilter() !== 'all') count++;
    if (protectedStaleOnly()) count++;
    return count;
  });
  const eventsActiveFilterCount = createMemo(() => {
    let count = 0;
    if (providerFilter() !== 'all') count++;
    if (outcomeFilter() !== 'all') count++;
    if (scopeFilter() !== 'all') count++;
    if (activeAdvancedFilterCount() > 0) count++;
    return count;
  });

  const filteredPoints = createMemo<RecoveryPoint[]>(() => recoveryPoints.points() || []);

  const sortedPoints = createMemo<RecoveryPoint[]>(() => {
    const resIndex = resourcesById();
    return [...filteredPoints()].sort((a, b) => {
      const aTs = pointTimestampMs(a);
      const bTs = pointTimestampMs(b);
      if (aTs !== bTs) return bTs - aTs;
      const aName = buildSubjectLabelForPoint(a, resIndex);
      const bName = buildSubjectLabelForPoint(b, resIndex);
      return aName.localeCompare(bName);
    });
  });

  const groupedByDay = createMemo(() => {
    const groups: Array<{ key: string; label: string; tone: 'recent' | 'default'; items: RecoveryPoint[] }> = [];
    const now = new Date();
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
    const yesterday = today - 24 * 60 * 60 * 1000;

    const groupMap = new Map<
      string,
      { key: string; label: string; tone: 'recent' | 'default'; items: RecoveryPoint[] }
    >();
    for (const p of sortedPoints()) {
      const key = p.completedAt ? dateKeyFromTimestamp(Date.parse(p.completedAt)) : 'unknown';
      if (!groupMap.has(key)) {
        let label = 'No Timestamp';
        let tone: 'recent' | 'default' = 'default';
        if (key !== 'unknown') {
          const date = parseDateKey(key);
          const dateOnly = new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
          if (dateOnly === today) {
            label = `Today (${fullDateLabel(key)})`;
            tone = 'recent';
          } else if (dateOnly === yesterday) {
            label = `Yesterday (${fullDateLabel(key)})`;
            tone = 'recent';
          } else {
            label = fullDateLabel(key);
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
  const hasNodeData = createMemo(() => (facets().nodesHosts || []).length > 0);
  const hasNamespaceData = createMemo(() => (facets().namespaces || []).length > 0);
  const hasEntityIDData = createMemo(() => Boolean(facets().hasEntityId));

  const artifactColumns: ColumnDef[] = [
    { id: 'time', label: 'Time' },
    { id: 'subject', label: 'Subject' },
    { id: 'entityId', label: 'ID', toggleable: true },
    { id: 'cluster', label: 'Cluster', toggleable: true },
    { id: 'nodeHost', label: 'Node/Host', toggleable: true },
    { id: 'namespace', label: 'Namespace', toggleable: true },
    { id: 'source', label: 'Source' },
    { id: 'verified', label: 'Verified', toggleable: true },
    { id: 'size', label: 'Size', toggleable: true },
    { id: 'method', label: 'Method' },
    { id: 'repository', label: 'Repository/Target' },
    { id: 'outcome', label: 'Outcome' },
  ];

  const relevantArtifactColumnIDs = createMemo(() => {
    const ids = new Set<string>(['time', 'subject', 'source', 'method', 'repository', 'outcome']);
    if (hasVerificationData()) ids.add('verified');
    if (hasSizeData()) ids.add('size');
    if (hasClusterData()) ids.add('cluster');
    if (hasNodeData()) ids.add('nodeHost');
    if (hasNamespaceData()) ids.add('namespace');
    if (hasEntityIDData()) ids.add('entityId');
    return ids;
  });

  const artifactColumnVisibility = useColumnVisibility(
    STORAGE_KEYS.RECOVERY_HIDDEN_COLUMNS,
    artifactColumns,
    ['entityId', 'cluster', 'nodeHost', 'namespace'],
    relevantArtifactColumnIDs,
  );

  const visibleArtifactColumns = createMemo(() => artifactColumnVisibility.visibleColumns());
  const tableColumnCount = createMemo(() => visibleArtifactColumns().length);
  const tableMinWidth = createMemo(() => `${Math.max(980, tableColumnCount() * 140)}px`);

  createEffect(() => {
    if (currentPage() > totalPages()) setCurrentPage(totalPages());
  });

  const timeline = createMemo(() => {
    const series = recoverySeries.series() || [];
    const points = series.map((bucket) => {
      const key = String(bucket.day || '').trim();
      const date = parseDateKey(key);
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
    const axisMax = niceAxisMax(maxValue);
    const axisTicks = [0, 1, 2, 3, 4].map((step) => Math.round((axisMax * step) / 4));
    const dayCount = points.length;
    const labelEvery = dayCount <= 7 ? 1 : dayCount <= 30 ? 3 : 10;

    return { points, maxValue, axisMax, axisTicks, labelEvery };
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
  const activeNamespaceLabel = createMemo(() => (namespaceFilter() === 'all' ? '' : namespaceFilter()));

  const hasActiveArtifactFilters = createMemo(
    () =>
      query().trim() !== '' ||
      providerFilter() !== 'all' ||
      clusterFilter() !== 'all' ||
      modeFilter() !== 'all' ||
      outcomeFilter() !== 'all' ||
      scopeFilter() !== 'all' ||
      nodeFilter() !== 'all' ||
      namespaceFilter() !== 'all' ||
      verificationFilter() !== 'all' ||
      chartRangeDays() !== 30 ||
      selectedDateKey() !== null,
  );

  const resetAllArtifactFilters = () => {
    setQuery('');
    setProviderFilter('all');
    setClusterFilter('all');
    setModeFilter('all');
    setOutcomeFilter('all');
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
      <Card padding="sm">
        <Show
          when={!rollupId().trim()}
          fallback={
            <div class="flex items-center gap-1.5">
              <button
                type="button"
                onClick={() => {
                  setRollupId('');
                  setView('protected');
                }}
                class="text-sm font-medium text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 transition-colors"
              >
                Protected
              </button>
              <span class="text-gray-400 dark:text-gray-500 text-sm">›</span>
              <span class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
                <Show when={selectedRollup()}>
                  {rollupSubjectLabel(selectedRollup()!, resourcesById())}
                </Show>
              </span>
            </div>
          }
        >
          <div class="inline-flex rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5" role="group" aria-label="View">
            <button
              type="button"
              onClick={() => {
                setView('protected');
                setRollupId('');
              }}
              aria-pressed={view() === 'protected'}
              class={segmentedButtonClass(view() === 'protected')}
            >
              Protected
            </button>
            <button
              type="button"
              onClick={() => setView('events')}
              aria-pressed={view() === 'events'}
              class={segmentedButtonClass(view() === 'events')}
            >
              Events
            </button>
          </div>
        </Show>
      </Card>

      <Show when={view() === 'protected'}>
        <Show when={!kioskMode()}>
          <Card padding="sm">
            <div class="flex flex-col gap-2">
              <SearchInput
                value={query}
                onChange={(value) => setQuery(value)}
                placeholder="Search protected items..."
                class="w-full"
                history={{
                  storageKey: STORAGE_KEYS.RECOVERY_SEARCH_HISTORY,
                  emptyMessage: 'Recent searches appear here.',
                }}
              />
              <Show when={isMobile()}>
                <button
                  type="button"
                  onClick={() => setProtectedFiltersOpen((o) => !o)}
                  class="flex items-center gap-1.5 rounded-lg bg-gray-100 dark:bg-gray-700 px-2.5 py-1.5 text-xs font-medium text-gray-600 dark:text-gray-400"
                >
                  <ListFilterIcon class="w-3.5 h-3.5" />
                  Filters
                  <Show when={protectedActiveFilterCount() > 0}>
                    <span class="ml-0.5 rounded-full bg-blue-500 px-1.5 py-0.5 text-[10px] font-semibold text-white leading-none">
                      {protectedActiveFilterCount()}
                    </span>
                  </Show>
                </button>
              </Show>

              <Show when={!isMobile() || protectedFiltersOpen()}>
                <div class="flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                  <div class="inline-flex items-center gap-1 rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                    <label
                      for="recovery-provider-filter"
                      class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500"
                    >
                      Provider
                    </label>
                    <select
                      id="recovery-provider-filter"
                      value={providerFilter()}
                      onChange={(event) => setProviderFilter(event.currentTarget.value)}
                      class="min-w-[10rem] max-w-[14rem] rounded-md border border-gray-200 bg-white px-2 py-1 text-xs font-medium text-gray-800 outline-none focus:border-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
                    >
                      <For each={providerOptions()}>
                        {(p) => <option value={p}>{p === 'all' ? 'All Providers' : sourceLabel(p)}</option>}
                      </For>
                    </select>
                  </div>

                  <div class="inline-flex items-center gap-1 rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                    <label
                      for="recovery-protected-status-filter"
                      class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500"
                    >
                      Status
                    </label>
                    <select
                      id="recovery-protected-status-filter"
                      value={outcomeFilter()}
                      onChange={(event) => {
                        const value = event.currentTarget.value as 'all' | KnownOutcome;
                        setOutcomeFilter(value);
                        if (value !== 'all') setVerificationFilter('all');
                      }}
                      class="min-w-[7rem] rounded-md border border-gray-200 bg-white px-2 py-1 text-xs font-medium text-gray-800 outline-none focus:border-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
                    >
                      <For each={availableOutcomes}>
                        {(outcome) => (
                          <option value={outcome}>
                            {outcome === 'all' ? 'Any' : titleize(outcome)}
                          </option>
                        )}
                      </For>
                    </select>
                  </div>

                  <button
                    type="button"
                    aria-pressed={protectedStaleOnly()}
                    onClick={() => setProtectedStaleOnly((v) => !v)}
                    class={`rounded-md border px-2.5 py-1 text-xs font-medium transition-colors ${
                      protectedStaleOnly()
                        ? 'border-amber-300 bg-amber-50 text-amber-800 dark:border-amber-700 dark:bg-amber-900/30 dark:text-amber-100'
                        : 'border-gray-200 bg-white text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700'
                    }`}
                  >
                    Stale only
                  </button>

                  <Show when={rollupsSummary().stale > 0}>
                    <span class="rounded-full bg-amber-100 px-2 py-0.5 text-[10px] font-medium text-amber-800 dark:bg-amber-900/40 dark:text-amber-100">
                      {rollupsSummary().stale} stale
                    </span>
                  </Show>

                  <Show when={rollupsSummary().neverSucceeded > 0}>
                    <span class="rounded-full bg-rose-100 px-2 py-0.5 text-[10px] font-medium text-rose-700 dark:bg-rose-900/40 dark:text-rose-200">
                      {rollupsSummary().neverSucceeded} never succeeded
                    </span>
                  </Show>
                </div>
              </Show>
            </div>
          </Card>
        </Show>

        <Card padding="sm">
          <Show when={recoveryRollups.rollups.loading && filteredRollups().length === 0}>
            <div class="px-3 py-6 text-sm text-gray-500 dark:text-gray-400">Loading protection rollups...</div>
          </Show>

          <Show when={!recoveryRollups.rollups.loading && recoveryRollups.rollups.error}>
            <EmptyState
              title="Failed to load protected items"
              description={String((recoveryRollups.rollups.error as Error)?.message || recoveryRollups.rollups.error)}
            />
          </Show>

          <Show when={!recoveryRollups.rollups.loading && !recoveryRollups.rollups.error && filteredRollups().length === 0}>
            <EmptyState
              title="No protected items yet"
              description="Pulse hasn’t received any recovery events for this org yet."
            />
          </Show>

          <Show when={filteredRollups().length > 0}>
            <div class="overflow-x-auto">
              <table class="w-full text-xs">
                <thead>
                  <tr class="border-b border-gray-200 bg-gray-50 text-left text-[10px] uppercase tracking-wide text-gray-500 dark:border-gray-700 dark:bg-gray-800/70 dark:text-gray-400">
                    {([['subject', 'Subject'], ['source', 'Source'], ['lastBackup', 'Last Backup'], ['outcome', 'Outcome']] as const).map(([col, label]) => (
                      <th
                        class="px-1.5 sm:px-2 py-1 cursor-pointer select-none hover:text-gray-700 dark:hover:text-gray-200 transition-colors"
                        onClick={() => toggleProtectedSort(col)}
                      >
                        <span class="inline-flex items-center gap-1">
                          {label}
                          <Show when={protectedSortCol() === col}>
                            <svg class="h-3 w-3" viewBox="0 0 12 12" fill="currentColor">
                              {protectedSortDir() === 'asc'
                                ? <path d="M6 3l3.5 5h-7z" />
                                : <path d="M6 9l3.5-5h-7z" />}
                            </svg>
                          </Show>
                        </span>
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                  <For each={sortedRollups()}>
                    {(r) => {
                      const resIndex = resourcesById();
                      const label = rollupSubjectLabel(r, resIndex);
                      const attemptMs = r.lastAttemptAt ? Date.parse(r.lastAttemptAt) : 0;
                      const successMs = r.lastSuccessAt ? Date.parse(r.lastSuccessAt) : 0;
                      const outcome = normalizeOutcome(r.lastOutcome);
                      const providers = (r.providers || [])
                        .slice()
                        .map((p) => String(p || '').trim())
                        .filter(Boolean)
                        .sort((a, b) => sourceLabel(a).localeCompare(sourceLabel(b)));
                      const nowMs = Date.now();
                      const issueTone = deriveRollupIssueTone(r, nowMs);
                      const issueRailClass = issueTone === 'none' ? '' : ISSUE_RAIL_CLASS[issueTone];
                      const stale = isRollupStale(r, nowMs);
                      const neverSucceeded = (!Number.isFinite(successMs) || successMs <= 0) && Number.isFinite(attemptMs) && attemptMs > 0;
                      return (
                        <tr
                          class="cursor-pointer border-b border-gray-200 hover:bg-gray-50/70 dark:border-gray-700 dark:hover:bg-gray-800/35"
                          onClick={() => {
                            setView('events');
                            setRollupId(r.rollupId);
                          }}
                        >
                          <td
                            class={`relative max-w-[420px] truncate whitespace-nowrap px-1.5 sm:px-2 py-1 text-gray-900 ${
                              issueTone === 'rose' || issueTone === 'blue'
                                ? 'font-medium dark:text-slate-100'
                                : issueTone === 'amber'
                                  ? 'dark:text-slate-200'
                                  : 'dark:text-gray-300'
                            }`}
                            title={label}
                          >
                            <Show when={issueTone !== 'none'}>
                              <span class={`absolute inset-y-0 left-0 w-0.5 ${issueRailClass}`} />
                            </Show>
                            <div class="flex items-center gap-2">
                              <span class="truncate">{label}</span>
                              <Show when={neverSucceeded}>
                                <span class="whitespace-nowrap rounded px-1 py-0.5 text-[10px] font-medium text-amber-700 bg-amber-50 dark:text-amber-300 dark:bg-amber-900/30">never succeeded</span>
                              </Show>
                              <Show when={!neverSucceeded && stale}>
                                <span class="whitespace-nowrap rounded px-1 py-0.5 text-[10px] font-medium text-amber-700 bg-amber-50 dark:text-amber-300 dark:bg-amber-900/30">stale</span>
                              </Show>
                            </div>
                          </td>
                          <td class="whitespace-nowrap px-1.5 sm:px-2 py-1">
                            <div class="flex flex-wrap gap-1.5">
                              <For each={providers}>
                                {(p) => {
                                  const badge = getSourcePlatformBadge(String(p));
                                  return <span class={badge?.classes || ''}>{badge?.label || sourceLabel(String(p))}</span>;
                                }}
                              </For>
                            </div>
                          </td>
                          <td
                            class={`whitespace-nowrap px-1.5 sm:px-2 py-1 ${rollupAgeTextClass(r, nowMs)}`}
                            title={successMs > 0 ? formatAbsoluteTime(successMs) : attemptMs > 0 ? formatAbsoluteTime(attemptMs) : undefined}
                          >
                            {successMs > 0 ? formatRelativeTime(successMs) : neverSucceeded ? (
                              <span class="text-amber-600 dark:text-amber-400">never</span>
                            ) : '—'}
                          </td>
                          <td class="whitespace-nowrap px-1.5 sm:px-2 py-1">
                            <span
                              class={`inline-flex rounded px-1.5 py-0.5 text-[10px] font-medium ${OUTCOME_BADGE_CLASS[outcome]}`}
                            >
                              {titleize(outcome)}
                            </span>
                          </td>
                        </tr>
                      );
                    }}
                  </For>
                </tbody>
              </table>
            </div>
          </Show>
        </Card>
      </Show>

      <Show when={view() === 'events'}>
        <Show when={recoveryPoints.response.loading && sortedPoints().length === 0}>
          <Card padding="sm">
            <div class="px-3 py-6 text-sm text-gray-500 dark:text-gray-400">Loading recovery points...</div>
          </Card>
        </Show>

        <Show when={!recoveryPoints.response.loading && recoveryPoints.response.error}>
          <Card padding="sm">
            <EmptyState
              title="Failed to load recovery points"
              description={String((recoveryPoints.response.error as Error)?.message || recoveryPoints.response.error)}
            />
          </Card>
        </Show>

        <Show when={!recoveryPoints.response.loading && !recoveryPoints.response.error}>
          <Card padding="sm" class="h-full">
            <Show when={selectedDateKey() || activeClusterLabel() || activeNodeLabel() || activeNamespaceLabel()}>
              <div class="mb-1 flex flex-wrap items-center gap-1.5">
                <Show when={selectedDateKey()}>
                  <div class="inline-flex max-w-full items-center gap-1 rounded border border-blue-200 bg-blue-50 px-2 py-0.5 text-[10px] text-blue-700 dark:border-blue-700 dark:bg-blue-900/30 dark:text-blue-200">
                    <span class="font-medium uppercase tracking-wide">Day</span>
                    <span class="truncate font-mono text-[10px]" title={selectedDateLabel()}>
                      {selectedDateLabel()}
                    </span>
                    <button
                      type="button"
                      onClick={() => {
                        setSelectedDateKey(null);
                        setCurrentPage(1);
                      }}
                      class="rounded px-1 py-0.5 text-[10px] hover:bg-blue-100 dark:hover:bg-blue-900/50"
                    >
                      Clear
                    </button>
                  </div>
                </Show>
                <Show when={activeClusterLabel()}>
                  <div class="inline-flex max-w-full items-center gap-1 rounded border border-cyan-200 bg-cyan-50 px-2 py-0.5 text-[10px] text-cyan-700 dark:border-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-200">
                    <span class="font-medium uppercase tracking-wide">Cluster</span>
                    <span class="truncate font-mono text-[10px]" title={activeClusterLabel()}>
                      {activeClusterLabel()}
                    </span>
                    <button
                      type="button"
                      onClick={() => {
                        setClusterFilter('all');
                        setCurrentPage(1);
                      }}
                      class="rounded px-1 py-0.5 text-[10px] hover:bg-cyan-100 dark:hover:bg-cyan-900/50"
                    >
                      Clear
                    </button>
                  </div>
                </Show>
                <Show when={activeNodeLabel()}>
                  <div class="inline-flex max-w-full items-center gap-1 rounded border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-[10px] text-emerald-700 dark:border-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200">
                    <span class="font-medium uppercase tracking-wide">Node/Host</span>
                    <span class="truncate font-mono text-[10px]" title={activeNodeLabel()}>
                      {activeNodeLabel()}
                    </span>
                    <button
                      type="button"
                      onClick={() => {
                        setNodeFilter('all');
                        setCurrentPage(1);
                      }}
                      class="rounded px-1 py-0.5 text-[10px] hover:bg-emerald-100 dark:hover:bg-emerald-900/50"
                    >
                      Clear
                    </button>
                  </div>
                </Show>
                <Show when={activeNamespaceLabel()}>
                  <div
                    data-testid="active-namespace-chip"
                    class="inline-flex max-w-full items-center gap-1 rounded border border-violet-200 bg-violet-50 px-2 py-0.5 text-[10px] text-violet-700 dark:border-violet-700 dark:bg-violet-900/30 dark:text-violet-200"
                  >
                    <span class="font-medium uppercase tracking-wide">Namespace</span>
                    <span class="truncate font-mono text-[10px]" title={activeNamespaceLabel()}>
                      {activeNamespaceLabel()}
                    </span>
                    <button
                      type="button"
                      onClick={() => {
                        setNamespaceFilter('all');
                        setCurrentPage(1);
                      }}
                      class="rounded px-1 py-0.5 text-[10px] hover:bg-violet-100 dark:hover:bg-violet-900/50"
                    >
                      Clear
                    </button>
                  </div>
                </Show>
              </div>
            </Show>

            <Show
              when={timeline().points.length > 0 && timeline().maxValue > 0}
              fallback={
                <div class="text-sm text-gray-600 dark:text-gray-300">
                  <Show when={recoverySeries.response.loading}>
                    <span>Loading recovery activity...</span>
                  </Show>
                  <Show when={!recoverySeries.response.loading}>
                    <span>No recovery activity in the selected window.</span>
                  </Show>
                </div>
              }
            >
              <div class="mb-1.5 flex flex-wrap items-center justify-between gap-2 text-xs text-gray-600 dark:text-gray-300">
                <div class="flex items-center gap-3">
                  <span class="flex items-center gap-1">
                    <span class={`h-2.5 w-2.5 rounded ${CHART_SEGMENT_CLASS.snapshot}`} />
                    Snapshots
                  </span>
                  <span class="flex items-center gap-1">
                    <span class={`h-2.5 w-2.5 rounded ${CHART_SEGMENT_CLASS.local}`} />
                    Local
                  </span>
                  <span class="flex items-center gap-1">
                    <span class={`h-2.5 w-2.5 rounded ${CHART_SEGMENT_CLASS.remote}`} />
                    Remote
                  </span>
                </div>
                <div class="inline-flex rounded border border-gray-300 bg-white p-0.5 text-xs dark:border-gray-700 dark:bg-gray-900">
                  <For each={[7, 30, 90, 365] as const}>
                    {(range) => (
                      <button
                        type="button"
                        onClick={() => {
                          setChartRangeDays(range);
                          setSelectedDateKey(null);
                          setCurrentPage(1);
                        }}
                        class={`rounded px-2 py-1 ${
                          chartRangeDays() === range
                            ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-200'
                            : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700/60'
                        }`}
                      >
                        {range === 365 ? '1y' : `${range}d`}
                      </button>
                    )}
                  </For>
                </div>
              </div>

              <div class="relative h-32 overflow-hidden rounded bg-gray-100 dark:bg-gray-800/80">
                <div class="absolute bottom-8 left-1 top-2 w-8 text-[10px] text-gray-500 dark:text-gray-400">
                  <div class="flex h-full flex-col justify-between text-right">
                    <For each={[...timeline().axisTicks].reverse()}>{(tick) => <span>{tick}</span>}</For>
                  </div>
                </div>
                <div
                  class="absolute bottom-0 left-10 right-10 top-2 overflow-x-auto"
                  style="scrollbar-width: none; -ms-overflow-style: none;"
                >
                  <div class="relative h-full min-w-[700px] px-2">
                    <div class="absolute inset-x-0 bottom-6 top-0">
                      <For each={timeline().axisTicks}>
                        {(tick) => {
                          const bottom = timeline().axisMax > 0 ? (tick / timeline().axisMax) * 100 : 0;
                          return (
                            <div
                              class="pointer-events-none absolute inset-x-0 border-t border-gray-200/80 dark:border-gray-700/70"
                              style={{ bottom: `${bottom}%` }}
                            />
                          );
                        }}
                      </For>
                    </div>

                    <div class="absolute inset-x-0 bottom-6 top-0 flex items-stretch gap-[3px]">
                      <For each={timeline().points}>
                        {(point) => {
                          const total = point.total;
                          const heightPct = timeline().axisMax > 0 ? (total / timeline().axisMax) * 100 : 0;
                          const columnHeight = Math.max(0, Math.min(100, heightPct));
                          const snapshotHeight = total > 0 ? (point.snapshot / total) * 100 : 0;
                          const localHeight = total > 0 ? (point.local / total) * 100 : 0;
                          const remoteHeight = total > 0 ? (point.remote / total) * 100 : 0;
                          const isSelected = selectedDateKey() === point.key;

                          return (
                            <div class="flex-1">
                              <button
                                type="button"
                                class={`h-full w-full rounded-sm transition-colors ${
                                  isSelected ? 'bg-blue-100 dark:bg-blue-900/30' : 'hover:bg-gray-200 dark:hover:bg-gray-700/70'
                                }`}
                                aria-label={`${prettyDateLabel(point.key)}: ${total} recovery points`}
                                onClick={() => {
                                  setSelectedDateKey((prev) => (prev === point.key ? null : point.key));
                                  setCurrentPage(1);
                                }}
                                onMouseEnter={(event) => {
                                  const rect = event.currentTarget.getBoundingClientRect();
                                  const breakdown: string[] = [];
                                  if (point.snapshot > 0) breakdown.push(`Snapshots: ${point.snapshot}`);
                                  if (point.local > 0) breakdown.push(`Local: ${point.local}`);
                                  if (point.remote > 0) breakdown.push(`Remote: ${point.remote}`);
                                  const tooltipText =
                                    point.total > 0
                                      ? `${prettyDateLabel(point.key)}\nAvailable: ${point.total} recovery point${point.total > 1 ? 's' : ''}\n${breakdown.join(' • ')}`
                                      : `${prettyDateLabel(point.key)}\nNo recovery points available`;
                                  showTooltip(tooltipText, rect.left + rect.width / 2, rect.top, {
                                    align: 'center',
                                    direction: 'up',
                                  });
                                }}
                                onMouseLeave={() => hideTooltip()}
                                onFocus={(event) => {
                                  const rect = event.currentTarget.getBoundingClientRect();
                                  const tooltipText = `${prettyDateLabel(point.key)}\nAvailable: ${point.total} recovery point${point.total > 1 ? 's' : ''}`;
                                  showTooltip(tooltipText, rect.left + rect.width / 2, rect.top, {
                                    align: 'center',
                                    direction: 'up',
                                  });
                                }}
                                onBlur={() => hideTooltip()}
                              >
                                <div class="relative h-full w-full">
                                  <Show when={total > 0}>
                                    <div class="absolute inset-x-0 bottom-0" style={{ height: `${columnHeight}%` }}>
                                      <Show when={remoteHeight > 0}>
                                        <div class={`w-full ${CHART_SEGMENT_CLASS.remote}`} style={{ height: `${remoteHeight}%` }} />
                                      </Show>
                                      <Show when={localHeight > 0}>
                                        <div class={`w-full ${CHART_SEGMENT_CLASS.local}`} style={{ height: `${localHeight}%` }} />
                                      </Show>
                                      <Show when={snapshotHeight > 0}>
                                        <div class={`w-full ${CHART_SEGMENT_CLASS.snapshot}`} style={{ height: `${snapshotHeight}%` }} />
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
                          const showLabel = index() % timeline().labelEvery === 0 || index() === timeline().points.length - 1;
                          const isSelected = selectedDateKey() === point.key;
                          const barMinWidth =
                            chartRangeDays() === 7 ? 'min-w-[28px]' : chartRangeDays() === 30 ? 'min-w-[14px]' : 'min-w-[8px]';
                          return (
                            <div class={`relative flex-1 shrink-0 ${barMinWidth}`}>
                              <Show when={showLabel}>
                                <span
                                  class={`absolute bottom-0 left-1/2 -translate-x-1/2 whitespace-nowrap text-[9px] ${
                                    isSelected ? 'font-semibold text-blue-700 dark:text-blue-300' : 'text-gray-500 dark:text-gray-400'
                                  }`}
                                >
                                  {compactAxisLabel(point.key, chartRangeDays())}
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

          <Show when={!kioskMode()}>
            <Card padding="sm">
              <div class="flex flex-col gap-2">
                <SearchInput
                  value={query}
                  onChange={(value) => {
                    setQuery(value);
                    setCurrentPage(1);
                  }}
                  placeholder="Search recovery events..."
                  class="w-full"
                  history={{
                    storageKey: STORAGE_KEYS.RECOVERY_SEARCH_HISTORY,
                    emptyMessage: 'Recent searches appear here.',
                  }}
                />
                <Show when={isMobile()}>
                  <button
                    type="button"
                    onClick={() => setEventsFiltersOpen((o) => !o)}
                    class="flex items-center gap-1.5 rounded-lg bg-gray-100 dark:bg-gray-700 px-2.5 py-1.5 text-xs font-medium text-gray-600 dark:text-gray-400"
                  >
                    <ListFilterIcon class="w-3.5 h-3.5" />
                    Filters
                    <Show when={eventsActiveFilterCount() > 0}>
                      <span class="ml-0.5 rounded-full bg-blue-500 px-1.5 py-0.5 text-[10px] font-semibold text-white leading-none">
                        {eventsActiveFilterCount()}
                      </span>
                    </Show>
                  </button>
                </Show>

                <Show when={!isMobile() || eventsFiltersOpen()}>
                  <div class="flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                    <div class="inline-flex items-center gap-1 rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                      <label
                        for="recovery-provider-filter-events"
                        class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500"
                      >
                        Provider
                      </label>
                      <select
                        id="recovery-provider-filter-events"
                        value={providerFilter()}
                        onChange={(event) => {
                          setProviderFilter(normalizeProviderFromQuery(event.currentTarget.value));
                          setCurrentPage(1);
                        }}
                        class="min-w-[10rem] max-w-[14rem] rounded-md border border-gray-200 bg-white px-2 py-1 text-xs font-medium text-gray-800 outline-none focus:border-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
                      >
                        <For each={providerOptions()}>
                          {(p) => <option value={p}>{p === 'all' ? 'All Providers' : sourceLabel(p)}</option>}
                        </For>
                      </select>
                    </div>

                    <div class="inline-flex items-center gap-1 rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                      <label
                        for="recovery-status-filter"
                        class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500"
                      >
                        Status
                      </label>
                      <select
                        id="recovery-status-filter"
                        value={outcomeFilter()}
                        onChange={(event) => {
                          const value = event.currentTarget.value as 'all' | KnownOutcome;
                          setOutcomeFilter(value);
                          if (value !== 'all') setVerificationFilter('all');
                          setCurrentPage(1);
                        }}
                        class="min-w-[7rem] rounded-md border border-gray-200 bg-white px-2 py-1 text-xs font-medium text-gray-800 outline-none focus:border-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
                      >
                        <For each={availableOutcomes}>
                          {(outcome) => (
                            <option value={outcome}>
                              {outcome === 'all' ? 'All' : titleize(outcome)}
                            </option>
                          )}
                        </For>
                      </select>
                    </div>

                    <div class="inline-flex items-center gap-1 rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                      <span class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500">Scope</span>
                      <button
                        type="button"
                        onClick={() => {
                          setScopeFilter('all');
                          setCurrentPage(1);
                        }}
                        class={segmentedButtonClass(scopeFilter() === 'all')}
                      >
                        All
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          setScopeFilter('workload');
                          setCurrentPage(1);
                        }}
                        class={segmentedButtonClass(scopeFilter() === 'workload')}
                      >
                        Workloads
                      </button>
                    </div>

                    <button
                      type="button"
                      aria-expanded={moreFiltersOpen()}
                      aria-controls="recovery-more-filters"
                      onClick={() => setMoreFiltersOpen((v) => !v)}
                      class="inline-flex items-center gap-2 rounded-md border border-gray-200 bg-white px-2.5 py-1 text-xs font-medium text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
                    >
                      <span>{moreFiltersOpen() ? 'Less filters' : 'More filters'}</span>
                      <Show when={activeAdvancedFilterCount() > 0}>
                        <span class="rounded-full bg-gray-200 px-1.5 py-0.5 text-[10px] font-mono text-gray-700 dark:bg-gray-700 dark:text-gray-200">
                          {activeAdvancedFilterCount()}
                        </span>
                      </Show>
                    </button>

                    <Show when={artifactColumnVisibility.availableToggles().length > 0}>
                      <ColumnPicker
                        columns={artifactColumnVisibility.availableToggles()}
                        isHidden={artifactColumnVisibility.isHiddenByUser}
                        onToggle={artifactColumnVisibility.toggle}
                        onReset={artifactColumnVisibility.resetToDefaults}
                      />
                    </Show>

                    <Show when={hasActiveArtifactFilters()}>
                      <button
                        type="button"
                        onClick={resetAllArtifactFilters}
                        class="shrink-0 rounded-lg bg-blue-100 px-2.5 py-1.5 text-xs font-medium text-blue-700 transition-colors hover:bg-blue-200 dark:bg-blue-900/40 dark:text-blue-300 dark:hover:bg-blue-900/60"
                      >
                        Clear
                      </button>
                    </Show>
                  </div>

                  <Show when={moreFiltersOpen()}>
                    <div id="recovery-more-filters" class="flex flex-wrap items-center gap-2 pt-2 border-t border-gray-200 dark:border-gray-700">
                      <div class="inline-flex items-center gap-1 rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                        <span class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500">Method</span>
                        <For each={(['all', 'snapshot', 'local', 'remote'] as const)}>
                          {(mode) => (
                            <button
                              type="button"
                              aria-pressed={modeFilter() === mode}
                              onClick={() => {
                                setModeFilter(mode === 'all' ? 'all' : mode);
                                setCurrentPage(1);
                              }}
                              class={segmentedButtonClass(modeFilter() === mode)}
                            >
                              {mode === 'all' ? 'All' : MODE_LABELS[mode]}
                            </button>
                          )}
                        </For>
                      </div>

                      <Show when={showVerificationFilter()}>
                        <div class="inline-flex items-center gap-1 rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                          <label
                            for="recovery-verification-filter"
                            class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500"
                          >
                            Verification
                          </label>
                          <select
                            id="recovery-verification-filter"
                            value={verificationFilter()}
                            onChange={(event) => {
                              setVerificationFilter(event.currentTarget.value as VerificationFilter);
                              if (event.currentTarget.value !== 'all') setOutcomeFilter('all');
                              setCurrentPage(1);
                            }}
                            class="min-w-[6.5rem] rounded-md border border-gray-200 bg-white px-2 py-1 text-xs font-medium text-gray-800 outline-none focus:border-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
                          >
                            <option value="all">Any</option>
                            <option value="verified">Verified</option>
                            <option value="unverified">Unverified</option>
                            <option value="unknown">Unknown</option>
                          </select>
                        </div>
                      </Show>

                      <Show when={showClusterFilter()}>
                        <div class="inline-flex items-center gap-1 rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                          <label
                            for="recovery-cluster-filter"
                            class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500"
                          >
                            Cluster
                          </label>
                          <select
                            id="recovery-cluster-filter"
                            value={clusterFilter()}
                            onChange={(event) => {
                              setClusterFilter(event.currentTarget.value);
                              setCurrentPage(1);
                            }}
                            class="min-w-[8rem] rounded-md border border-gray-200 bg-white px-2 py-1 text-xs font-medium text-gray-800 outline-none focus:border-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
                          >
                            <option value="all">All</option>
                            <For each={clusterOptions().filter((value) => value !== 'all')}>
                              {(cluster) => <option value={cluster}>{cluster}</option>}
                            </For>
                          </select>
                        </div>
                      </Show>

                      <Show when={showNodeFilter()}>
                        <div class="inline-flex items-center gap-1 rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                          <label
                            for="recovery-node-filter"
                            class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500"
                          >
                            Node/Host
                          </label>
                          <select
                            id="recovery-node-filter"
                            value={nodeFilter()}
                            onChange={(event) => {
                              setNodeFilter(event.currentTarget.value);
                              setCurrentPage(1);
                            }}
                            class="min-w-[7.5rem] rounded-md border border-gray-200 bg-white px-2 py-1 text-xs font-medium text-gray-800 outline-none focus:border-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
                          >
                            <option value="all">All</option>
                            <For each={nodeOptions().filter((value) => value !== 'all')}>
                              {(node) => <option value={node}>{node}</option>}
                            </For>
                          </select>
                        </div>
                      </Show>

                      <Show when={showNamespaceFilter()}>
                        <div class="inline-flex items-center gap-1 rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
                          <label
                            for="recovery-namespace-filter"
                            class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500"
                          >
                            Namespace
                          </label>
                          <select
                            id="recovery-namespace-filter"
                            value={namespaceFilter()}
                            onChange={(event) => {
                              setNamespaceFilter(event.currentTarget.value);
                              setCurrentPage(1);
                            }}
                            class="min-w-[8rem] rounded-md border border-gray-200 bg-white px-2 py-1 text-xs font-medium text-gray-800 outline-none focus:border-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
                          >
                            <option value="all">All</option>
                            <For each={namespaceOptions().filter((value) => value !== 'all')}>
                              {(namespace) => <option value={namespace}>{namespace}</option>}
                            </For>
                          </select>
                        </div>
                      </Show>
                    </div>
                  </Show>
                </Show>
              </div>
            </Card>
          </Show>

          <Card padding="none" class="overflow-hidden">
            <Show
              when={groupedByDay().length > 0}
              fallback={
                <div class="p-6">
                  <EmptyState
                    title="No recovery events match your filters"
                    description="Adjust your search, provider, method, status, or verification filters."
                    actions={
                      <Show when={hasActiveArtifactFilters()}>
                        <button
                          type="button"
                          onClick={resetAllArtifactFilters}
                          class="inline-flex items-center gap-2 rounded-md border border-gray-200 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 shadow-sm hover:bg-gray-50 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
                        >
                          Clear filters
                        </button>
                      </Show>
                    }
                  />
                </div>
              }
            >
              <div class="overflow-x-auto">
                <table class="w-full text-xs" style={{ 'min-width': tableMinWidth() }}>
                  <thead>
                    <tr class="border-b border-gray-200 bg-gray-50 text-left text-[10px] uppercase tracking-wide text-gray-500 dark:border-gray-700 dark:bg-gray-800/70 dark:text-gray-400">
                      <For each={visibleArtifactColumns()}>{(col) => <th class="px-1.5 sm:px-2 py-1">{col.label}</th>}</For>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                    <For each={groupedByDay()}>
                      {(group) => (
                        <>
                          <tr class={groupHeaderRowClass()}>
                            <td colSpan={tableColumnCount()} class={groupHeaderTextClass()}>
                              <div class="flex items-center gap-2">
                                <span>{group.label}</span>
                                <Show when={group.tone === 'recent'}>
                                  <span class="rounded-full bg-blue-100 px-2 py-0.5 text-[10px] font-medium text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                                    recent
                                  </span>
                                </Show>
                              </div>
                            </td>
                          </tr>

                          <For each={group.items}>
                            {(p) => {
                              const resIndex = resourcesById();
                              const subject = buildSubjectLabelForPoint(p, resIndex);
                              const mode = (String(p.mode || '').trim().toLowerCase() as ArtifactMode) || 'local';
                              const repoLabel = buildRepositoryLabelForPoint(p);
                              const provider = String(p.provider || '').trim();
                              const outcome = normalizeOutcome(p.outcome);
                              const completedMs = p.completedAt ? Date.parse(p.completedAt) : 0;
                              const startedMs = p.startedAt ? Date.parse(p.startedAt) : 0;
                              const tsMs = completedMs || startedMs || 0;
                              const timeOnly = tsMs ? formatTimeOnly(tsMs) : '—';

                              const entityId = String(p.entityId || '').trim();
                              const cluster = String(p.cluster || '').trim();
                              const nodeHost = String(p.node || '').trim();
                              const namespace = String(p.namespace || '').trim();


                              return (
                                <tr
                                  class="cursor-pointer border-b border-gray-200 hover:bg-gray-50/70 dark:border-gray-700 dark:hover:bg-gray-800/35"
                                  onClick={() => setSelectedPoint(p)}
                                >
                                  <For each={visibleArtifactColumns()}>
                                    {(col) => {
                                      switch (col.id) {
                                        case 'time':
                                          return (
                                            <td class="whitespace-nowrap px-1.5 sm:px-2 py-1 text-gray-500 dark:text-gray-400">
                                              {timeOnly}
                                            </td>
                                          );
                                        case 'subject':
                                          return (
                                            <td
                                              class="max-w-[420px] truncate whitespace-nowrap px-1.5 sm:px-2 py-1 text-gray-900 dark:text-gray-100"
                                              title={subject}
                                            >
                                              {subject}
                                            </td>
                                          );
                                        case 'entityId':
                                          return (
                                            <td class="whitespace-nowrap px-1.5 sm:px-2 py-1 text-gray-600 dark:text-gray-400 font-mono">
                                              {entityId || '—'}
                                            </td>
                                          );
                                        case 'cluster':
                                          return (
                                            <td class="whitespace-nowrap px-1.5 sm:px-2 py-1 text-gray-600 dark:text-gray-400 font-mono">
                                              {cluster || '—'}
                                            </td>
                                          );
                                        case 'nodeHost':
                                          return (
                                            <td class="whitespace-nowrap px-1.5 sm:px-2 py-1 text-gray-600 dark:text-gray-400 font-mono">
                                              {nodeHost || '—'}
                                            </td>
                                          );
                                        case 'namespace':
                                          return (
                                            <td class="whitespace-nowrap px-1.5 sm:px-2 py-1 text-gray-600 dark:text-gray-400 font-mono">
                                              {namespace || '—'}
                                            </td>
                                          );
                                        case 'source': {
                                          const badge = getSourcePlatformBadge(provider);
                                          return (
                                            <td class="whitespace-nowrap px-1.5 sm:px-2 py-1">
                                              <span class={badge?.classes || ''}>{badge?.label || sourceLabel(provider)}</span>
                                            </td>
                                          );
                                        }
                                        case 'verified':
                                          return (
                                            <td class="whitespace-nowrap px-1.5 sm:px-2 py-1">
                                              {typeof p.verified === 'boolean' ? (
                                                p.verified ? (
                                                  <span class="inline-flex items-center gap-1 text-green-600 dark:text-green-400" title="Verified">
                                                    <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M5 13l4 4L19 7" />
                                                    </svg>
                                                  </span>
                                                ) : (
                                                  <span class="inline-flex items-center gap-1 text-amber-500 dark:text-amber-400" title="Unverified">
                                                    <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M12 9v2m0 4h.01M12 3a9 9 0 100 18 9 9 0 000-18z" />
                                                    </svg>
                                                  </span>
                                                )
                                              ) : (
                                                <span class="text-gray-400 dark:text-gray-600">—</span>
                                              )}
                                            </td>
                                          );
                                        case 'size':
                                          return (
                                            <td class="whitespace-nowrap px-1.5 sm:px-2 py-1 text-gray-500 dark:text-gray-400">
                                              {p.sizeBytes && p.sizeBytes > 0 ? formatBytes(p.sizeBytes) : '—'}
                                            </td>
                                          );
                                        case 'method':
                                          return (
                                            <td class="whitespace-nowrap px-1.5 sm:px-2 py-1">
                                              <span
                                                class={`inline-flex rounded px-1.5 py-0.5 text-[10px] font-medium ${MODE_BADGE_CLASS[mode]}`}
                                              >
                                                {MODE_LABELS[mode]}
                                              </span>
                                            </td>
                                          );
                                        case 'repository':
                                          return (
                                            <td
                                              class="max-w-[220px] truncate whitespace-nowrap px-1.5 sm:px-2 py-1 text-[11px] leading-4 text-gray-600 dark:text-gray-400"
                                              title={repoLabel}
                                            >
                                              {repoLabel || '—'}
                                            </td>
                                          );
                                        case 'outcome':
                                          return (
                                            <td class="whitespace-nowrap px-1.5 sm:px-2 py-1">
                                              <span
                                                class={`inline-flex rounded px-1.5 py-0.5 text-[10px] font-medium ${
                                                  OUTCOME_BADGE_CLASS[outcome]
                                                }`}
                                              >
                                                {titleize(outcome)}
                                              </span>
                                            </td>
                                          );
                                        default:
                                          return (
                                            <td class="whitespace-nowrap px-1.5 sm:px-2 py-1 text-gray-500 dark:text-gray-400">-</td>
                                          );
                                      }
                                    }}
                                  </For>
                                </tr>
                              );
                            }}
                          </For>
                        </>
                      )}
                    </For>
                  </tbody>
                </table>
              </div>

              <div class="flex items-center justify-between gap-2 px-3 py-2 text-xs text-gray-500 dark:text-gray-400 border-t border-gray-200 dark:border-gray-700">
                <div>
                  <Show when={(recoveryPoints.meta().total || 0) > 0} fallback={<span>Showing 0 of 0 events</span>}>
                    <span>
                      Showing {(recoveryPoints.meta().page - 1) * recoveryPoints.meta().limit + 1} -{' '}
                      {Math.min(recoveryPoints.meta().page * recoveryPoints.meta().limit, recoveryPoints.meta().total)} of{' '}
                      {recoveryPoints.meta().total} events
                    </span>
                  </Show>
                </div>
                <div class="flex items-center gap-2">
                  <button
                    type="button"
                    disabled={currentPage() <= 1}
                    onClick={() => setCurrentPage(Math.max(1, currentPage() - 1))}
                    class="rounded-md border border-gray-200 bg-white px-2 py-1 text-xs font-medium text-gray-700 disabled:opacity-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
                  >
                    Prev
                  </button>
                  <span>Page {currentPage()} / {totalPages()}</span>
                  <button
                    type="button"
                    disabled={currentPage() >= totalPages()}
                    onClick={() => setCurrentPage(Math.min(totalPages(), currentPage() + 1))}
                    class="rounded-md border border-gray-200 bg-white px-2 py-1 text-xs font-medium text-gray-700 disabled:opacity-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
                  >
                    Next
                  </button>
                </div>
              </div>
            </Show>
          </Card>

          <Dialog
            isOpen={Boolean(selectedPoint())}
            onClose={() => setSelectedPoint(null)}
            panelClass="w-[min(920px,92vw)]"
            ariaLabel="Recovery point details"
          >
            <div class="flex items-center justify-between border-b border-gray-200 px-4 py-3 dark:border-gray-700">
              <h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Recovery Point Details</h2>
              <button
                type="button"
                onClick={() => setSelectedPoint(null)}
                class="rounded-md p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-700 dark:hover:text-gray-300"
                aria-label="Close details"
              >
                <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            <div class="flex-1 overflow-y-auto p-4">
              <Show when={selectedPoint()}>
                {(p) => <RecoveryPointDetails point={p()!} />}
              </Show>
            </div>
          </Dialog>
        </Show>
      </Show>
    </div>
  );
};

export default Recovery;
