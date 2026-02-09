import { Component, For, Show, createEffect, createMemo, createSignal, untrack } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { useWebSocket } from '@/App';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { SearchInput } from '@/components/shared/SearchInput';
import { getSourcePlatformBadge, getSourcePlatformLabel } from '@/components/shared/sourcePlatformBadges';
import { hideTooltip, showTooltip } from '@/components/shared/Tooltip';
import { getWorkloadTypeBadge } from '@/components/shared/workloadTypeBadges';
import { formatAbsoluteTime, formatBytes } from '@/utils/format';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { buildBackupRecords } from '@/features/storageBackups/backupAdapters';
import { PLATFORM_BLUEPRINTS } from '@/features/storageBackups/platformBlueprint';
import type { BackupOutcome, BackupRecord } from '@/features/storageBackups/models';
import { useStorageBackupsResources } from '@/hooks/useUnifiedResources';
import {
  BACKUPS_QUERY_PARAMS,
  buildBackupsPath,
  parseBackupsLinkSearch,
} from '@/routing/resourceLinks';

type BackupMode = 'snapshot' | 'local' | 'remote';
type VerificationFilter = 'all' | 'verified' | 'unverified' | 'unknown';

const PAGE_SIZE = 100;
const STALE_ISSUE_THRESHOLD_MS = 7 * 24 * 60 * 60 * 1000;
const AGING_THRESHOLD_MS = 2 * 24 * 60 * 60 * 1000;

const OUTCOME_BADGE_CLASS: Record<BackupOutcome, string> = {
  success: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
  warning: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-300',
  failed: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300',
  running: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
  offline: 'bg-gray-200 text-gray-800 dark:bg-gray-700 dark:text-gray-200',
  unknown: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300',
};

const MODE_LABELS: Record<BackupMode, string> = {
  snapshot: 'Snapshots',
  local: 'Local',
  remote: 'Remote',
};

const MODE_BADGE_CLASS: Record<BackupMode, string> = {
  snapshot: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/20 dark:text-yellow-300',
  local: 'bg-orange-100 text-orange-700 dark:bg-orange-900/50 dark:text-orange-300',
  remote: 'bg-purple-100 text-purple-700 dark:bg-purple-900/50 dark:text-purple-300',
};

const CHART_SEGMENT_CLASS: Record<BackupMode, string> = {
  snapshot: 'bg-yellow-500',
  local: 'bg-orange-500',
  remote: 'bg-violet-500',
};

const titleize = (value: string): string =>
  value
    .split('-')
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

const sourceLabel = (value: string): string => getSourcePlatformLabel(value);

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
  if (!timestamp) return 'n/a';
  return new Date(timestamp).toLocaleTimeString(undefined, {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  });
};

const isStaleIssue = (record: BackupRecord, nowMs: number): boolean => {
  if (!record.completedAt || record.completedAt <= 0) return false;
  return nowMs - record.completedAt >= STALE_ISSUE_THRESHOLD_MS;
};

type IssueTone = 'none' | 'amber' | 'rose' | 'blue';

const deriveIssueTone = (record: BackupRecord, nowMs: number): IssueTone => {
  if (record.outcome === 'failed' || record.outcome === 'offline') return 'rose';
  if (record.outcome === 'running') return 'blue';
  if (record.outcome === 'warning' || record.verified === false || isStaleIssue(record, nowMs)) return 'amber';
  return 'none';
};

const ISSUE_RAIL_CLASS: Record<Exclude<IssueTone, 'none'>, string> = {
  amber: 'bg-amber-500/80 dark:bg-amber-500/85',
  rose: 'bg-rose-500/85 dark:bg-rose-500/90',
  blue: 'bg-blue-500/80 dark:bg-blue-500/85',
};

const timeAgeTextClass = (record: BackupRecord, nowMs: number): string => {
  if (!record.completedAt || record.completedAt <= 0) return 'text-gray-500 dark:text-gray-500';
  const ageMs = nowMs - record.completedAt;
  if (ageMs >= STALE_ISSUE_THRESHOLD_MS) return 'text-rose-700 dark:text-rose-300';
  if (ageMs >= AGING_THRESHOLD_MS) return 'text-amber-700 dark:text-amber-300';
  return 'text-gray-600 dark:text-gray-400';
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

const normalizeSourceFilterFromQuery = (value: string): string => {
  switch ((value || '').trim().toLowerCase()) {
    case 'pve':
    case 'proxmox':
    case 'proxmox-pve':
      return 'proxmox-pve';
    case 'pbs':
    case 'proxmox-pbs':
      return 'proxmox-pbs';
    case 'pmg':
    case 'proxmox-pmg':
      return 'proxmox-pmg';
    case 'all':
    case '':
      return 'all';
    default:
      return value;
  }
};

const toLegacySourceValue = (value: string): string => {
  if (value === 'proxmox-pve') return 'pve';
  if (value === 'proxmox-pbs') return 'pbs';
  if (value === 'proxmox-pmg') return 'pmg';
  return value;
};

const normalizeMode = (value: string | null | undefined): BackupMode | 'all' => {
  const normalized = (value || '').trim().toLowerCase();
  if (normalized === 'snapshot' || normalized === 'local' || normalized === 'remote') return normalized;
  return 'all';
};

const isBackupOutcome = (value: string): value is BackupOutcome =>
  value === 'success' ||
  value === 'warning' ||
  value === 'failed' ||
  value === 'running' ||
  value === 'offline' ||
  value === 'unknown';

const detailString = (record: BackupRecord, key: string): string => {
  const details = (record.details as Record<string, unknown> | undefined) || {};
  const value = details[key];
  if (typeof value === 'string') return value;
  if (typeof value === 'number' && Number.isFinite(value)) return String(value);
  return '';
};

const deriveMode = (record: BackupRecord): BackupMode => {
  if (record.mode === 'snapshot' || record.mode === 'local' || record.mode === 'remote') return record.mode;
  const detailMode = detailString(record, 'mode');
  if (detailMode === 'snapshot' || detailMode === 'local' || detailMode === 'remote') return detailMode;
  if (record.category === 'snapshot') return 'snapshot';
  return 'local';
};

const deriveClusterLabel = (record: BackupRecord): string => {
  const kubernetesCluster = (record.kubernetes?.cluster || '').trim();
  if (kubernetesCluster) return kubernetesCluster;
  const proxmoxInstance = (record.proxmox?.instance || '').trim();
  if (proxmoxInstance) return proxmoxInstance;
  if (record.location.scope === 'cluster') return (record.location.label || '').trim() || 'n/a';
  return 'n/a';
};

const deriveNodeLabel = (record: BackupRecord): string => {
  const typedNode =
    (record.proxmox?.node || '').trim() ||
    (record.kubernetes?.node || '').trim() ||
    (record.docker?.host || '').trim();
  if (typedNode) return typedNode;
  const detailNode = detailString(record, 'node');
  if (detailNode) return detailNode;
  if (record.location.scope === 'node') return (record.location.label || '').trim() || 'n/a';
  return 'n/a';
};

const deriveNamespaceLabel = (record: BackupRecord): string => {
  const namespace = (record.proxmox?.namespace || record.kubernetes?.namespace || detailString(record, 'namespace')).trim();
  if (namespace) return namespace;
  if (record.source.platform === 'proxmox-pbs') return 'root';
  return 'n/a';
};

const deriveEntityIdLabel = (record: BackupRecord): string => {
  const proxmoxId = (record.proxmox?.vmid || '').trim();
  if (proxmoxId) return proxmoxId;
  const dockerContainerId = (record.docker?.containerId || '').trim();
  if (dockerContainerId) return dockerContainerId;
  const kubernetesWorkload = firstNonEmpty(record.kubernetes?.workloadName, record.kubernetes?.backupId, record.kubernetes?.runId);
  if (kubernetesWorkload) return kubernetesWorkload;
  const vmidFromDetails = detailString(record, 'vmid');
  if (vmidFromDetails) return vmidFromDetails;
  const match = record.scope.label.match(/VMID\s+(.+)/i);
  if (match?.[1]) return match[1];
  return record.refs?.platformEntityId || 'n/a';
};

const deriveGuestType = (record: BackupRecord): string => {
  const workload = record.scope.workloadType || 'other';
  if (workload === 'vm') return 'VM';
  if (workload === 'container') {
    if (record.source.platform === 'docker') return 'Container';
    return 'LXC';
  }
  if (workload === 'host') return 'Host';
  if (workload === 'pod') return 'Pod';
  return titleize(workload);
};

const firstNonEmpty = (...values: Array<string | null | undefined>): string => {
  for (const value of values) {
    const normalized = (value || '').trim();
    if (normalized) return normalized;
  }
  return '';
};

const deriveDetailsOwner = (record: BackupRecord): string =>
  firstNonEmpty(record.proxmox?.owner, detailString(record, 'owner'));

const summarizeDetails = (record: BackupRecord): string => {
  const pieces: string[] = [];
  const datastore = firstNonEmpty(record.proxmox?.datastore, detailString(record, 'datastore'));
  const namespace = firstNonEmpty(record.proxmox?.namespace, record.kubernetes?.namespace, detailString(record, 'namespace'));
  const notes = firstNonEmpty(record.proxmox?.notes, detailString(record, 'notes'));
  const comment = firstNonEmpty(record.proxmox?.comment, detailString(record, 'comment'));
  const repository = firstNonEmpty(record.kubernetes?.repository, record.docker?.repository);
  const policy = firstNonEmpty(record.kubernetes?.policy, record.docker?.policy);
  const snapshotClass = firstNonEmpty(record.kubernetes?.snapshotClass);
  const image = firstNonEmpty(record.docker?.image);
  const volume = firstNonEmpty(record.docker?.volume);
  if (datastore) pieces.push(datastore);
  if (namespace) pieces.push(namespace || 'root');
  if (repository) pieces.push(repository);
  if (policy) pieces.push(policy);
  if (snapshotClass) pieces.push(snapshotClass);
  if (image) pieces.push(image);
  if (volume) pieces.push(volume);
  if (notes) pieces.push(notes);
  if (comment) pieces.push(comment);
  return pieces.join(' | ');
};

const segmentedButtonClass = (selected: boolean, disabled: boolean) => {
  const base = 'px-2 py-1 text-xs font-medium rounded-md transition-all duration-150 active:scale-95';
  if (disabled) return `${base} text-gray-400 dark:text-gray-600 cursor-not-allowed`;
  if (selected) {
    return `${base} bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm ring-1 ring-gray-200 dark:ring-gray-600`;
  }
  return `${base} text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 hover:bg-gray-50 dark:hover:bg-gray-600/50`;
};

const groupHeaderRowClass = () => 'bg-gray-50 dark:bg-gray-900/40';

const groupHeaderTextClass = () =>
  'py-1 pr-2 pl-4 text-[12px] sm:text-sm font-semibold text-slate-700 dark:text-slate-300';

const Backups: Component = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { state, connected, initialDataReceived } = useWebSocket();
  const storageBackupsResources = useStorageBackupsResources();

  const [search, setSearch] = createSignal('');
  const [sourceFilter, setSourceFilter] = createSignal('all');
  const [modeFilter, setModeFilter] = createSignal<'all' | BackupMode>('all');
  const [outcomeFilter, setOutcomeFilter] = createSignal<'all' | BackupOutcome>('all');
  const [scopeFilter, setScopeFilter] = createSignal('all');
  const [nodeFilter, setNodeFilter] = createSignal('all');
  const [namespaceFilter, setNamespaceFilter] = createSignal('all');
  const [verificationFilter, setVerificationFilter] = createSignal<VerificationFilter>('all');
  const [chartRangeDays, setChartRangeDays] = createSignal<7 | 30 | 90 | 365>(30);
  const [selectedDateKey, setSelectedDateKey] = createSignal<string | null>(null);
  const [currentPage, setCurrentPage] = createSignal(1);
  const adapterResources = createMemo(() => {
    const unifiedResources = storageBackupsResources.resources();
    return unifiedResources;
  });
  const records = createMemo<BackupRecord[]>(() =>
    buildBackupRecords({ state, resources: adapterResources() }),
  );

  const sourceOptions = createMemo(() => {
    const values = Array.from(new Set(records().map((record) => record.source.platform))).sort((a, b) =>
      sourceLabel(a).localeCompare(sourceLabel(b)),
    );
    return ['all', ...values];
  });

  const nodeOptions = createMemo(() => {
    const values = Array.from(
      new Set(
        records()
          .map((record) => deriveNodeLabel(record))
          .filter((value) => value !== 'n/a'),
      ),
    ).sort();
    return ['all', ...values];
  });

  const namespaceOptions = createMemo(() => {
    const values = Array.from(
      new Set(
        records()
          .map((record) => deriveNamespaceLabel(record))
          .filter((value) => value !== 'n/a'),
      ),
    ).sort();
    return ['all', ...values];
  });

  const availableOutcomes = ['all', 'success', 'warning', 'failed', 'running'] as const;
  const isActiveBackupsRoute = () => location.pathname === buildBackupsPath();

  createEffect(() => {
    if (!isActiveBackupsRoute()) return;

    const parsed = parseBackupsLinkSearch(location.search);

    if (parsed.query !== untrack(search)) setSearch(parsed.query);

    const sourceFromQuery = normalizeSourceFilterFromQuery(parsed.source);
    if (sourceFromQuery !== untrack(sourceFilter)) setSourceFilter(sourceFromQuery || 'all');

    const modeFromQuery = normalizeMode(parsed.backupType);
    if (modeFromQuery !== untrack(modeFilter)) setModeFilter(modeFromQuery);

    const scopeFromQuery = parsed.group === 'guest' ? 'workload' : 'all';
    if (scopeFromQuery !== untrack(scopeFilter)) setScopeFilter(scopeFromQuery);

    const nodeFromQuery = parsed.node || 'all';
    if (nodeFromQuery !== untrack(nodeFilter)) setNodeFilter(nodeFromQuery);

    const namespaceFromQuery = parsed.namespace || 'all';
    if (namespaceFromQuery !== untrack(namespaceFilter)) setNamespaceFilter(namespaceFromQuery);

    if (parsed.status === 'verified' || parsed.status === 'unverified') {
      if (parsed.status !== untrack(verificationFilter)) setVerificationFilter(parsed.status);
      if (untrack(outcomeFilter) !== 'all') setOutcomeFilter('all');
    } else {
      if (untrack(verificationFilter) !== 'all') setVerificationFilter('all');
      const statusValue = parsed.status || '';
      if (isBackupOutcome(statusValue)) {
        if (statusValue !== untrack(outcomeFilter)) setOutcomeFilter(statusValue);
      } else if (untrack(outcomeFilter) !== 'all') {
        setOutcomeFilter('all');
      }
    }
  });

  createEffect(() => {
    if (!isActiveBackupsRoute()) return;

    const nextStatus =
      verificationFilter() !== 'all' ? verificationFilter() : outcomeFilter() !== 'all' ? outcomeFilter() : '';

    const managedPath = buildBackupsPath({
      source: sourceFilter() === 'all' ? null : toLegacySourceValue(sourceFilter()),
      backupType: modeFilter() === 'all' ? null : modeFilter(),
      status: nextStatus || null,
      group: scopeFilter() === 'workload' ? 'guest' : null,
      node: nodeFilter() === 'all' ? null : nodeFilter(),
      namespace: namespaceFilter() === 'all' ? null : namespaceFilter(),
      query: search().trim() || null,
    });

    const [, managedSearch = ''] = managedPath.split('?');
    const managedParams = new URLSearchParams(managedSearch);
    const params = new URLSearchParams(location.search);

    Object.values(BACKUPS_QUERY_PARAMS).forEach((key) => params.delete(key));
    managedParams.forEach((value, key) => params.set(key, value));

    const basePath = location.pathname;
    const nextSearch = params.toString();
    const nextPath = nextSearch ? `${basePath}?${nextSearch}` : basePath;
    const currentPath = `${location.pathname}${location.search || ''}`;

    if (nextPath !== currentPath) navigate(nextPath, { replace: true });
  });

  const baseFilteredRecords = createMemo<BackupRecord[]>(() => {
    const query = search().trim().toLowerCase();
    return records().filter((record) => {
      const mode = deriveMode(record);
      const node = deriveNodeLabel(record);

      if (sourceFilter() !== 'all' && record.source.platform !== sourceFilter()) return false;
      if (modeFilter() !== 'all' && mode !== modeFilter()) return false;
      if (outcomeFilter() !== 'all' && record.outcome !== outcomeFilter()) return false;
      if (scopeFilter() !== 'all' && record.scope.scope !== scopeFilter()) return false;
      if (nodeFilter() !== 'all' && node !== nodeFilter()) return false;
      if (namespaceFilter() !== 'all' && deriveNamespaceLabel(record) !== namespaceFilter()) return false;

      if (verificationFilter() === 'verified' && record.verified !== true) return false;
      if (verificationFilter() === 'unverified' && record.verified !== false) return false;
      if (verificationFilter() === 'unknown' && record.verified !== null) return false;

      if (!query) return true;

      const haystack = [
        record.name,
        record.scope.label,
        record.location.label,
        record.source.platform,
        mode,
        deriveClusterLabel(record),
        deriveNodeLabel(record),
        deriveNamespaceLabel(record),
        deriveEntityIdLabel(record),
        record.proxmox?.datastore,
        record.proxmox?.owner,
        record.proxmox?.comment,
        record.proxmox?.notes,
        record.kubernetes?.workloadKind,
        record.kubernetes?.workloadName,
        record.kubernetes?.repository,
        record.kubernetes?.policy,
        record.kubernetes?.snapshotClass,
        record.docker?.containerId,
        record.docker?.containerName,
        record.docker?.image,
        record.docker?.volume,
        record.docker?.repository,
        record.docker?.policy,
        ...(record.capabilities || []),
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return haystack.includes(query);
    });
  });

  const rangeFilteredRecords = createMemo<BackupRecord[]>(() => {
    const end = new Date();
    end.setHours(23, 59, 59, 999);
    const start = new Date(end);
    start.setDate(start.getDate() - (chartRangeDays() - 1));
    start.setHours(0, 0, 0, 0);
    const startMs = start.getTime();
    const endMs = end.getTime();

    return baseFilteredRecords().filter((record) => {
      if (!record.completedAt) return true;
      return record.completedAt >= startMs && record.completedAt <= endMs;
    });
  });

  const filteredRecords = createMemo<BackupRecord[]>(() => {
    const dayKey = selectedDateKey();
    if (!dayKey) return rangeFilteredRecords();
    return rangeFilteredRecords().filter((record) => {
      if (!record.completedAt) return false;
      return dateKeyFromTimestamp(record.completedAt) === dayKey;
    });
  });

  createEffect(() => {
    const selected = selectedDateKey();
    if (!selected) return;
    const exists = rangeFilteredRecords().some((record) => {
      if (!record.completedAt) return false;
      return dateKeyFromTimestamp(record.completedAt) === selected;
    });
    if (!exists) {
      setSelectedDateKey(null);
    }
  });

  const sortedRecords = createMemo<BackupRecord[]>(() => {
    return [...filteredRecords()].sort((a, b) => {
      const aTs = a.completedAt || 0;
      const bTs = b.completedAt || 0;
      if (aTs !== bTs) return bTs - aTs;
      return a.name.localeCompare(b.name);
    });
  });

  const groupedByDay = createMemo(() => {
    const groups: Array<{ key: string; label: string; tone: 'recent' | 'default'; items: BackupRecord[] }> = [];
    const now = new Date();
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
    const yesterday = today - 24 * 60 * 60 * 1000;

    const groupMap = new Map<
      string,
      { key: string; label: string; tone: 'recent' | 'default'; items: BackupRecord[] }
    >();
    for (const record of sortedRecords()) {
      const key = record.completedAt ? dateKeyFromTimestamp(record.completedAt) : 'unknown';
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
        const group = { key, label, tone, items: [] as BackupRecord[] };
        groupMap.set(key, group);
        groups.push(group);
      }
      groupMap.get(key)!.items.push(record);
    }

    return groups;
  });

  const totalPages = createMemo(() => Math.max(1, Math.ceil(sortedRecords().length / PAGE_SIZE)));

  const pagedGroups = createMemo(() => {
    const start = (currentPage() - 1) * PAGE_SIZE;
    const end = start + PAGE_SIZE;
    const result: Array<{ key: string; label: string; tone: 'recent' | 'default'; items: BackupRecord[] }> = [];

    let cursor = 0;
    for (const group of groupedByDay()) {
      const groupStart = cursor;
      const groupEnd = cursor + group.items.length;
      cursor = groupEnd;

      if (groupEnd <= start) continue;
      if (groupStart >= end) break;

      const sliceStart = Math.max(0, start - groupStart);
      const sliceEnd = Math.min(group.items.length, end - groupStart);
      const items = group.items.slice(sliceStart, sliceEnd);
      if (items.length > 0) result.push({ key: group.key, label: group.label, tone: group.tone, items });
    }

    return result;
  });

  const showEntityColumn = createMemo(() => sortedRecords().some((record) => deriveEntityIdLabel(record) !== 'n/a'));
  const showTypeColumn = createMemo(() =>
    sortedRecords().some((record) => record.scope.workloadType && record.scope.workloadType !== 'other'),
  );
  const showClusterColumn = createMemo(() => sortedRecords().some((record) => deriveClusterLabel(record) !== 'n/a'));
  const showNodeColumn = createMemo(() => sortedRecords().some((record) => deriveNodeLabel(record) !== 'n/a'));
  const showNamespaceColumn = createMemo(() => sortedRecords().some((record) => deriveNamespaceLabel(record) !== 'n/a'));
  const showSizeColumn = createMemo(() => sortedRecords().some((record) => Boolean(record.sizeBytes && record.sizeBytes > 0)));
  const showVerificationColumn = createMemo(() => sortedRecords().some((record) => record.verified !== null));
  const showDetailsColumn = createMemo(
    () =>
      sortedRecords().some((record) => {
        const detailFlags: string[] = [];
        if (record.protected === true) detailFlags.push('Protected');
        if (record.encrypted === true) detailFlags.push('Encrypted');
        return [detailFlags.join(' • '), summarizeDetails(record), deriveDetailsOwner(record)]
          .filter((value) => value && value.trim().length > 0)
          .join(' | ').length > 0;
      }),
  );
  const tableColumnCount = createMemo(() => {
    let count = 5; // Time, Name, Source, Mode, Outcome
    if (showEntityColumn()) count += 1;
    if (showTypeColumn()) count += 1;
    if (showClusterColumn()) count += 1;
    if (showNodeColumn()) count += 1;
    if (showNamespaceColumn()) count += 1;
    if (showSizeColumn()) count += 1;
    if (showVerificationColumn()) count += 1;
    if (showDetailsColumn()) count += 1;
    return count;
  });
  const tableMinWidth = createMemo(() => `${Math.max(980, tableColumnCount() * 120)}px`);

  createEffect(() => {
    if (currentPage() > totalPages()) setCurrentPage(totalPages());
  });

  const artifactCount = createMemo(() => filteredRecords().length);

  const staleArtifactCount = createMemo(() => {
    const staleThreshold = Date.now() - STALE_ISSUE_THRESHOLD_MS;
    let count = 0;
    for (const record of filteredRecords()) {
      const completedAt = record.completedAt || 0;
      if (completedAt > 0 && completedAt < staleThreshold) count += 1;
    }
    return count;
  });

  const timeline = createMemo(() => {
    const days = chartRangeDays();
    const start = new Date();
    start.setHours(0, 0, 0, 0);
    start.setDate(start.getDate() - (days - 1));

    const keyForDate = (date: Date) => {
      const y = date.getFullYear();
      const m = String(date.getMonth() + 1).padStart(2, '0');
      const d = String(date.getDate()).padStart(2, '0');
      return `${y}-${m}-${d}`;
    };

    const buckets = new Map<
      string,
      { key: string; label: string; total: number; snapshot: number; local: number; remote: number }
    >();

    const cursor = new Date(start);
    for (let i = 0; i < days; i += 1) {
      const key = keyForDate(cursor);
      buckets.set(key, {
        key,
        label: cursor.toLocaleDateString(undefined, { month: 'short', day: 'numeric' }),
        total: 0,
        snapshot: 0,
        local: 0,
        remote: 0,
      });
      cursor.setDate(cursor.getDate() + 1);
    }

    for (const record of rangeFilteredRecords()) {
      if (!record.completedAt) continue;
      const key = keyForDate(new Date(record.completedAt));
      const bucket = buckets.get(key);
      if (!bucket) continue;
      const mode = deriveMode(record);
      bucket.total += 1;
      bucket[mode] += 1;
    }

    const points = Array.from(buckets.values());
    const maxValue = points.reduce((max, point) => Math.max(max, point.total), 0);
    const axisMax = niceAxisMax(maxValue);
    const axisTicks = [0, 1, 2, 3, 4].map((step) => Math.round((axisMax * step) / 4));
    const labelEvery = days <= 7 ? 1 : days <= 30 ? 3 : 10;

    return { points, maxValue, axisMax, axisTicks, labelEvery };
  });

  const timelineDiagnostics = createMemo(() => {
    const list = baseFilteredRecords();
    const inRangeList = rangeFilteredRecords();
    let withCompletedAt = 0;
    let latest: number | null = null;
    for (const record of list) {
      if (!record.completedAt) continue;
      withCompletedAt += 1;
      latest = Math.max(latest || 0, record.completedAt);
    }
    const inRange = inRangeList.reduce((sum, record) => (record.completedAt ? sum + 1 : sum), 0);
    return { total: list.length, withCompletedAt, inRange, latest };
  });

  const nextPlatforms = createMemo(() =>
    PLATFORM_BLUEPRINTS.filter((platform) => platform.stage === 'next').map((platform) => platform.label),
  );

  const selectedDateLabel = createMemo(() => {
    const key = selectedDateKey();
    if (!key) return '';
    const [year, month, day] = key.split('-').map((value) => Number.parseInt(value, 10));
    if (!year || !month || !day) return key;
    const date = new Date(year, month - 1, day);
    return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
  });
  const activeNamespaceLabel = createMemo(() => (namespaceFilter() === 'all' ? '' : namespaceFilter()));

  const hasActiveFilters = createMemo(
    () =>
      search().trim() !== '' ||
      sourceFilter() !== 'all' ||
      modeFilter() !== 'all' ||
      outcomeFilter() !== 'all' ||
      scopeFilter() !== 'all' ||
      nodeFilter() !== 'all' ||
      namespaceFilter() !== 'all' ||
      verificationFilter() !== 'all' ||
      chartRangeDays() !== 30 ||
      selectedDateKey() !== null,
  );

  const resetAllFilters = () => {
    setSearch('');
    setSourceFilter('all');
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
    <div data-testid="backups-page" class="flex flex-col gap-4">
      <Card padding="sm" class="order-2">
        <div class="flex flex-col gap-2">
          <div class="w-full">
            <SearchInput
              value={search}
              onChange={(value) => {
                setSearch(value);
                setCurrentPage(1);
              }}
              placeholder="Search backups, vmid, node, namespace, owner, notes..."
              class="w-full"
              autoFocus
              history={{
                storageKey: STORAGE_KEYS.BACKUPS_SEARCH_HISTORY,
                emptyMessage: 'Recent backup searches appear here.',
              }}
            />
          </div>

          <div class="flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
            <div class="inline-flex items-center gap-1 rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
              <label
                for="backups-source-filter"
                class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500"
              >
                Source
              </label>
              <select
                id="backups-source-filter"
                value={sourceFilter()}
                onChange={(event) => {
                  setSourceFilter(event.currentTarget.value);
                  setCurrentPage(1);
                }}
                class="min-w-[8rem] max-w-[11rem] rounded-md border border-gray-200 bg-white px-2 py-1 text-xs font-medium text-gray-800 outline-none focus:border-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
              >
                <For each={sourceOptions()}>
                  {(source) => (
                    <option value={source}>
                      {source === 'all' ? 'All Sources' : sourceLabel(source)}
                    </option>
                  )}
                </For>
              </select>
            </div>

            <div class="inline-flex items-center gap-1 rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
              <span class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500">Mode</span>
              <button
                type="button"
                aria-pressed={modeFilter() === 'all'}
                onClick={() => {
                  setModeFilter('all');
                  setCurrentPage(1);
                }}
                class={segmentedButtonClass(modeFilter() === 'all', false)}
              >
                All
              </button>
              <For each={(['snapshot', 'local', 'remote'] as BackupMode[])}>
                {(mode) => (
                  <button
                    type="button"
                    aria-pressed={modeFilter() === mode}
                    onClick={() => {
                      setModeFilter(mode);
                      setCurrentPage(1);
                    }}
                    class={segmentedButtonClass(modeFilter() === mode, false)}
                  >
                    {MODE_LABELS[mode]}
                  </button>
                )}
              </For>
            </div>

            <div class="inline-flex items-center gap-1 rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
              <label
                for="backups-status-filter"
                class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500"
              >
                Status
              </label>
              <select
                id="backups-status-filter"
                value={outcomeFilter()}
                onChange={(event) => {
                  const value = event.currentTarget.value as 'all' | BackupOutcome;
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
              <label
                for="backups-verification-filter"
                class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500"
              >
                Verification
              </label>
              <select
                id="backups-verification-filter"
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

            <div class="inline-flex items-center gap-1 rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
              <label
                for="backups-node-filter"
                class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500"
              >
                Node
              </label>
              <select
                id="backups-node-filter"
                value={nodeFilter()}
                onChange={(event) => {
                  setNodeFilter(event.currentTarget.value);
                  setCurrentPage(1);
                }}
                class="min-w-[7.5rem] rounded-md border border-gray-200 bg-white px-2 py-1 text-xs font-medium text-gray-800 outline-none focus:border-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
              >
                <option value="all">All Nodes</option>
                <For each={nodeOptions().filter((value) => value !== 'all')}>
                  {(node) => <option value={node}>{node}</option>}
                </For>
              </select>
            </div>

            <div class="inline-flex items-center gap-1 rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
              <label
                for="backups-namespace-filter"
                class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500"
              >
                Namespace
              </label>
              <select
                id="backups-namespace-filter"
                value={namespaceFilter()}
                onChange={(event) => {
                  setNamespaceFilter(event.currentTarget.value);
                  setCurrentPage(1);
                }}
                class="min-w-[8rem] rounded-md border border-gray-200 bg-white px-2 py-1 text-xs font-medium text-gray-800 outline-none focus:border-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
              >
                <option value="all">All Namespaces</option>
                <For each={namespaceOptions().filter((value) => value !== 'all')}>
                  {(namespace) => <option value={namespace}>{namespace}</option>}
                </For>
              </select>
            </div>

            <div class="inline-flex items-center gap-1 rounded-lg bg-gray-100 dark:bg-gray-700 p-0.5">
              <span class="px-1.5 text-[9px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500">View</span>
              <button
                type="button"
                onClick={() => {
                  setScopeFilter('all');
                  setCurrentPage(1);
                }}
                class={segmentedButtonClass(scopeFilter() === 'all', false)}
              >
                All
              </button>
              <button
                type="button"
                onClick={() => {
                  setScopeFilter('workload');
                  setCurrentPage(1);
                }}
                class={segmentedButtonClass(scopeFilter() === 'workload', false)}
              >
                Guest
              </button>
            </div>

            <Show when={hasActiveFilters()}>
              <button
                type="button"
                onClick={resetAllFilters}
                class="shrink-0 rounded-lg bg-blue-100 px-2.5 py-1.5 text-xs font-medium text-blue-700 transition-colors hover:bg-blue-200 dark:bg-blue-900/40 dark:text-blue-300 dark:hover:bg-blue-900/60"
              >
                Clear
              </button>
            </Show>
          </div>
        </div>
      </Card>

      <div class="order-1">
      <Card padding="sm" class="h-full">
          <Show when={selectedDateKey() || activeNamespaceLabel()}>
            <div class="mb-1 flex flex-wrap items-center gap-1.5">
              <Show when={selectedDateKey()}>
                <div class="inline-flex max-w-full items-center gap-1 rounded border border-blue-200 bg-blue-50 px-2 py-0.5 text-[10px] text-blue-700 dark:border-blue-700 dark:bg-blue-900/30 dark:text-blue-200">
                  <span class="font-medium uppercase tracking-wide">Day</span>
                  <span class="truncate font-mono text-[10px]" title={selectedDateLabel()}>
                    {selectedDateLabel()}
                  </span>
                  <button
                    type="button"
                    onClick={() => setSelectedDateKey(null)}
                    class="rounded px-1 py-0.5 text-[10px] hover:bg-blue-100 dark:hover:bg-blue-900/50"
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
              <Show
                when={timelineDiagnostics().withCompletedAt > 0}
                fallback={
                  <span>
                    No backup timestamps available from current sources/filters ({timelineDiagnostics().total} artifacts).
                  </span>
                }
              >
                <span>
                  No backup activity in the last {chartRangeDays()} days.
                  <Show when={timelineDiagnostics().latest}>
                    {(latest) => (
                      <span class="ml-1 text-gray-500 dark:text-gray-400">
                        Latest timestamp: {formatAbsoluteTime(latest())}
                      </span>
                    )}
                  </Show>
                </span>
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
                    onClick={() => setChartRangeDays(range)}
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

                  <div class="absolute inset-0 flex items-end gap-[3px]">
                    <For each={timeline().points}>
                      {(point) => {
                        const total = point.total;
                        const snapshotHeight = total > 0 ? (point.snapshot / total) * 100 : 0;
                        const localHeight = total > 0 ? (point.local / total) * 100 : 0;
                        const remoteHeight = total > 0 ? (point.remote / total) * 100 : 0;
                        const columnHeight = timeline().axisMax > 0 ? (total / timeline().axisMax) * 100 : 0;
                        const isSelected = selectedDateKey() === point.key;
                        const barMinWidth =
                          chartRangeDays() === 7 ? 'min-w-[28px]' : chartRangeDays() === 30 ? 'min-w-[14px]' : 'min-w-[8px]';

                        return (
                          <div class={`relative h-full flex-1 shrink-0 ${barMinWidth}`}>
                            <button
                              type="button"
                              class={`h-full w-full rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/60 ${
                                isSelected ? 'bg-blue-100/70 dark:bg-blue-900/30' : 'hover:bg-gray-200/60 dark:hover:bg-gray-700/50'
                              }`}
                              onClick={() => {
                                if (selectedDateKey() === point.key) setSelectedDateKey(null);
                                else setSelectedDateKey(point.key);
                              }}
                              onMouseEnter={(event) => {
                                const rect = event.currentTarget.getBoundingClientRect();
                                const breakdown: string[] = [];
                                if (point.snapshot > 0) breakdown.push(`Snapshots: ${point.snapshot}`);
                                if (point.local > 0) breakdown.push(`Local: ${point.local}`);
                                if (point.remote > 0) breakdown.push(`Remote: ${point.remote}`);
                                const tooltipText =
                                  point.total > 0
                                    ? `${prettyDateLabel(point.key)}\nAvailable: ${point.total} backup${point.total > 1 ? 's' : ''}\n${breakdown.join(' • ')}`
                                    : `${prettyDateLabel(point.key)}\nNo backups available`;
                                showTooltip(tooltipText, rect.left + rect.width / 2, rect.top, {
                                  align: 'center',
                                  direction: 'up',
                                });
                              }}
                              onMouseLeave={() => hideTooltip()}
                              onFocus={(event) => {
                                const rect = event.currentTarget.getBoundingClientRect();
                                const tooltipText = `${prettyDateLabel(point.key)}\nAvailable: ${point.total} backup${point.total > 1 ? 's' : ''}`;
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

      </div>

      <Card padding="none" class="order-3 overflow-hidden">
        <Show
          when={pagedGroups().length > 0}
          fallback={
            <div class="p-6">
              <EmptyState
                title="No backups match your filters"
                description="Adjust your search, source, mode, or node filters."
                actions={
                  <Show when={hasActiveFilters()}>
                    <button
                      type="button"
                      onClick={resetAllFilters}
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
                  <th class="px-2 py-1.5">Time</th>
                  <th class="px-2 py-1.5">Name</th>
                  <Show when={showEntityColumn()}>
                    <th class="px-2 py-1.5">Entity</th>
                  </Show>
                  <Show when={showTypeColumn()}>
                    <th class="px-2 py-1.5">Type</th>
                  </Show>
                  <Show when={showClusterColumn()}>
                    <th class="px-2 py-1.5">Cluster</th>
                  </Show>
                  <Show when={showNodeColumn()}>
                    <th class="px-2 py-1.5">Node/Host</th>
                  </Show>
                  <Show when={showNamespaceColumn()}>
                    <th class="px-2 py-1.5">Namespace</th>
                  </Show>
                  <th class="px-2 py-1.5">Source</th>
                  <Show when={showSizeColumn()}>
                    <th class="px-2 py-1.5">Size</th>
                  </Show>
                  <th class="px-2 py-1.5">Mode</th>
                  <Show when={showVerificationColumn()}>
                    <th class="px-2 py-1.5">Verified</th>
                  </Show>
                  <Show when={showDetailsColumn()}>
                    <th class="px-2 py-1.5">Details</th>
                  </Show>
                  <th class="px-2 py-1.5">Outcome</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                <For each={pagedGroups()}>
                  {(group) => (
                    <>
                      <tr
                        class={groupHeaderRowClass()}
                      >
                        <td colSpan={tableColumnCount()} class={groupHeaderTextClass()}>
                          <div class="flex items-center gap-2">
                            <span>{group.label}</span>
                            <span class="text-[10px] font-normal text-slate-400 dark:text-slate-500">
                              {group.items.length} {group.items.length === 1 ? 'artifact' : 'artifacts'}
                            </span>
                          </div>
                        </td>
                      </tr>
                      <For each={group.items}>
                        {(record) => {
                          const mode = deriveMode(record);
                          const guestType = deriveGuestType(record);
                          const namespace = deriveNamespaceLabel(record);
                          const guestTypeBadge = getWorkloadTypeBadge(record.scope.workloadType, {
                            label: guestType,
                            title: guestType,
                          });
                          const sourceBadge = getSourcePlatformBadge(record.source.platform);
                          const detailsText = summarizeDetails(record);
                          const owner = deriveDetailsOwner(record);
                          const rowNowMs = Date.now();
                          const issueTone = deriveIssueTone(record, rowNowMs);
                          const issueRailClass = issueTone === 'none' ? '' : ISSUE_RAIL_CLASS[issueTone];
                          const detailFlags: string[] = [];
                          if (record.protected === true) detailFlags.push('Protected');
                          if (record.encrypted === true) detailFlags.push('Encrypted');
                          const detailInline = [detailFlags.join(' • '), detailsText || owner || '-']
                            .filter((value) => value && value.trim().length > 0)
                            .join(' | ');
                          return (
                            <tr class="border-b border-gray-200 hover:bg-gray-50/70 dark:border-gray-700 dark:hover:bg-gray-800/35">
                              <td class={`relative whitespace-nowrap pl-8 pr-2 py-1 ${timeAgeTextClass(record, rowNowMs)}`}>
                                <Show when={issueTone !== 'none'}>
                                  <span class={`absolute inset-y-0 left-0 w-0.5 ${issueRailClass}`} />
                                </Show>
                                {formatTimeOnly(record.completedAt)}
                              </td>
                              <td
                                class={`max-w-[240px] truncate whitespace-nowrap pl-6 pr-2 py-1 text-gray-900 ${
                                  issueTone === 'rose' || issueTone === 'blue'
                                    ? 'font-medium dark:text-slate-100'
                                    : issueTone === 'amber'
                                      ? 'dark:text-slate-200'
                                      : 'dark:text-gray-300'
                                }`}
                                title={record.name}
                              >
                                {record.name}
                              </td>
                              <Show when={showEntityColumn()}>
                                <td class="whitespace-nowrap pl-6 pr-2 py-1 font-mono text-gray-700 dark:text-gray-400">{deriveEntityIdLabel(record)}</td>
                              </Show>
                              <Show when={showTypeColumn()}>
                                <td class="whitespace-nowrap pl-6 pr-2 py-1 text-gray-600 dark:text-gray-400">
                                  <span
                                    class={`inline-flex items-center whitespace-nowrap rounded px-1 py-0.5 text-[10px] font-medium ${guestTypeBadge.className}`}
                                    title={guestTypeBadge.title}
                                  >
                                    {guestTypeBadge.label}
                                  </span>
                                </td>
                              </Show>
                              <Show when={showClusterColumn()}>
                                <td class="whitespace-nowrap pl-6 pr-2 py-1 text-gray-600 dark:text-gray-400">{deriveClusterLabel(record)}</td>
                              </Show>
                              <Show when={showNodeColumn()}>
                                <td class="whitespace-nowrap pl-6 pr-2 py-1 text-gray-600 dark:text-gray-400">{deriveNodeLabel(record)}</td>
                              </Show>
                              <Show when={showNamespaceColumn()}>
                                <td class="whitespace-nowrap pl-6 pr-2 py-1 text-gray-600 dark:text-gray-400">
                                  <Show
                                    when={namespace !== 'n/a'}
                                    fallback={<span class="text-xs text-gray-500 dark:text-gray-400">n/a</span>}
                                  >
                                    <span class="inline-flex items-center whitespace-nowrap rounded bg-violet-100 px-1.5 py-0.5 text-[10px] font-medium text-violet-700 dark:bg-violet-900/40 dark:text-violet-300">
                                      {namespace}
                                    </span>
                                  </Show>
                                </td>
                              </Show>
                              <td class="whitespace-nowrap pl-6 pr-2 py-1 text-gray-600 dark:text-gray-400">
                                <Show
                                  when={sourceBadge}
                                  fallback={<span class="text-xs text-gray-500 dark:text-gray-400">{sourceLabel(record.source.platform)}</span>}
                                >
                                  <span class={sourceBadge!.classes} title={sourceBadge!.title}>
                                    {sourceBadge!.label}
                                  </span>
                                </Show>
                              </td>
                              <Show when={showSizeColumn()}>
                                <td class="whitespace-nowrap pl-6 pr-2 py-1 text-gray-700 dark:text-gray-400">{record.sizeBytes && record.sizeBytes > 0 ? formatBytes(record.sizeBytes) : 'n/a'}</td>
                              </Show>
                              <td class="whitespace-nowrap pl-6 pr-2 py-1">
                                <span class={`inline-flex rounded px-1.5 py-0.5 text-[10px] font-medium ${MODE_BADGE_CLASS[mode]}`}>{MODE_LABELS[mode]}</span>
                              </td>
                              <Show when={showVerificationColumn()}>
                                <td class="whitespace-nowrap pl-6 pr-2 py-1 text-[11px]">
                                  <Show when={record.verified === true}><span class="text-emerald-700 dark:text-emerald-300">Yes</span></Show>
                                  <Show when={record.verified === false}><span class="text-rose-700 dark:text-rose-300">No</span></Show>
                                  <Show when={record.verified === null}><span class="text-gray-500 dark:text-gray-400">n/a</span></Show>
                                </td>
                              </Show>
                              <Show when={showDetailsColumn()}>
                                <td class="max-w-[360px] truncate whitespace-nowrap pl-6 pr-2 py-1 text-[11px] leading-4 text-gray-600 dark:text-gray-400" title={detailInline}>
                                  {detailInline}
                                </td>
                              </Show>
                              <td class="whitespace-nowrap pl-6 pr-2 py-1">
                                <span class={`inline-flex rounded px-1.5 py-0.5 text-[10px] font-medium ${OUTCOME_BADGE_CLASS[record.outcome]}`}>{titleize(record.outcome)}</span>
                              </td>
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

          <div class="flex flex-wrap items-center justify-between gap-2 border-t border-gray-200 px-3 py-2 text-xs text-gray-600 dark:border-gray-700 dark:text-gray-300">
            <div>
              Showing {(currentPage() - 1) * PAGE_SIZE + 1} - {Math.min(currentPage() * PAGE_SIZE, sortedRecords().length)} of {sortedRecords().length} artifacts
            </div>
            <div class="inline-flex items-center gap-1">
              <button
                type="button"
                onClick={() => setCurrentPage((value) => Math.max(1, value - 1))}
                disabled={currentPage() <= 1}
                class="rounded border border-gray-300 px-2 py-1 disabled:opacity-50 dark:border-gray-700"
              >
                Previous
              </button>
              <span>Page {currentPage()} / {totalPages()}</span>
              <button
                type="button"
                onClick={() => setCurrentPage((value) => Math.min(totalPages(), value + 1))}
                disabled={currentPage() >= totalPages()}
                class="rounded border border-gray-300 px-2 py-1 disabled:opacity-50 dark:border-gray-700"
              >
                Next
              </button>
            </div>
          </div>
        </Show>
      </Card>

      <div class="order-5 flex flex-wrap items-center gap-3 px-1 text-[11px] text-gray-500 dark:text-gray-400">
        <span class="font-medium text-gray-700 dark:text-gray-200">{artifactCount()} artifacts</span>
        <Show when={staleArtifactCount() > 0}>
          <span class="text-amber-600 dark:text-amber-400">{staleArtifactCount()} older than 7 days</span>
        </Show>
        <Show when={nextPlatforms().length > 0}>
          <span>Next targets: {nextPlatforms().join(', ')}</span>
        </Show>
      </div>

      <Show when={!connected() && !initialDataReceived()}>
        <Card padding="sm" tone="warning" class="order-6">
          <div class="text-xs text-amber-800 dark:text-amber-200">Waiting for backup data from connected platforms.</div>
        </Card>
      </Show>
    </div>
  );
};

export default Backups;
