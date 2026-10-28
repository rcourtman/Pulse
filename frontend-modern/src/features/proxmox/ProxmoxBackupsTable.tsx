import {
  For,
  Show,
  createEffect,
  createMemo,
  createSignal,
  onCleanup,
  type Component,
  type JSX,
} from 'solid-js';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import { useLocation, useNavigate, useSearchParams } from '@solidjs/router';
import { FilterBar, type FilterDef, type FilterSelectOption } from '@/components/shared/FilterBar';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { useProtectionPostures } from '@/hooks/useProtectionPostures';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import {
  buildProxmoxBackupsPath,
  PROXMOX_BACKUPS_DEFAULT_VIEW,
  PROXMOX_BACKUPS_PATH,
  PROXMOX_BACKUPS_QUERY_PARAMS,
  type ProxmoxBackupsView,
} from '@/routing/resourceLinks';
import { apiFetch } from '@/utils/apiClient';
import { getAPIReadAccessErrorMessage } from '@/utils/apiAccessError';
import {
  PlatformErrorState,
  PlatformResourceCounter,
  PlatformSectionTabs,
  PlatformTableLoadingState,
} from '@/features/platformPage/sharedPlatformPage';
import {
  getRecoveryFilterDateLabel,
  recoveryDateKeyFromTimestamp,
} from '@/utils/recoveryDatePresentation';
import type {
  BackupTask,
  GuestSnapshot,
  PBSBackup,
  PBSBackupsPayload,
  PVEBackupsPayload,
  StorageBackup,
} from '@/types/api';
import type { Resource } from '@/types/resource';
import { BackupActivityChart } from './BackupActivityChart';
import {
  buildBackupActivityTimeline,
  getBackupActivityUnknownCount,
  type BackupActivityMetricMode,
  type BackupActivityRangeDays,
  type BackupActivitySegmentKind,
} from './proxmoxBackupActivityPresentation';
import {
  buildProxmoxBackupRecoveryModel,
  coverageRowMatchesSearch,
  isCompletedRecoveryArtifact,
  selectWorkloadRecoveryArtifacts,
  recoverableArtifactMatchesSearch,
  type RecoverableArtifact,
  type WorkloadCoverageRow,
} from './proxmoxBackupRecoveryModel';
import {
  COVERAGE_SORT_DEFAULT_DIRECTION,
  RECOVERABLE_SORT_DEFAULT_DIRECTION,
  cmpNumber,
  cmpString,
  type CoverageFilterValue,
  type CoverageSortKey,
  type RecoverableFilterValue,
  type RecoverableSortKey,
} from './proxmoxBackupsTableModel';
import { getBackupPostureRuleText } from './proxmoxBackupsTablePresentation';
import {
  COVERAGE_FILTERS,
  RECOVERABLE_FILTERS,
  artifactStateLabel,
} from './proxmoxBackupsTableShared';
import { getProxmoxBackupSourcePresentation } from './proxmoxBackupSourcePresentation';
import { ProxmoxBackupsCoverageStrip } from './ProxmoxBackupsCoverageStrip';
import { ProxmoxBackupServersTable } from './ProxmoxBackupServersTable';
import { ProxmoxCoverageTable } from './ProxmoxCoverageTable';
import { resolveBackupLocationFilter } from './proxmoxBackupLocation';
import { ProxmoxRecoverableTable } from './ProxmoxRecoverableTable';
import {
  BackupInventoryFormatError,
  parsePBSBackupInventory,
  parsePVEBackupInventory,
  readBackupInventoryJSON,
} from './proxmoxBackupInventory';

// One backups surface, two operator views: a chronological recoverable-artifact
// feed for "what ran when", and a guest coverage table for "what is protected".

type BackupView = ProxmoxBackupsView;

const BACKUP_VIEW_TABS: readonly { id: BackupView; label: string }[] = [
  { id: 'date', label: 'By date' },
  { id: 'coverage', label: 'Coverage' },
];

const BACKUP_INVENTORY_POLL_MS = 30_000;

const EMPTY_PVE_BACKUPS: PVEBackupsPayload = {
  backupTasks: [],
  storageBackups: [],
  guestSnapshots: [],
};
const EMPTY_PBS_BACKUPS: PBSBackupsPayload = { backups: [] };
const BACKUP_READ_NOTICE_CLASS =
  'rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:border-amber-700 dark:bg-amber-950/40 dark:text-amber-100';

async function fetchPVEBackups(signal: AbortSignal): Promise<PVEBackupsPayload> {
  const response = await apiFetch('/api/backups/pve', { signal });
  if (!response.ok) {
    throw Object.assign(new Error(`Failed to load PVE backups (${response.status})`), {
      status: response.status,
    });
  }
  return parsePVEBackupInventory(await readBackupInventoryJSON(response));
}

async function fetchPBSBackups(signal: AbortSignal): Promise<PBSBackupsPayload> {
  const response = await apiFetch('/api/backups/pbs', { signal });
  if (!response.ok) {
    throw Object.assign(new Error(`Failed to load PBS backups (${response.status})`), {
      status: response.status,
    });
  }
  return parsePBSBackupInventory(await readBackupInventoryJSON(response));
}

export const ProxmoxBackupsTable: Component<{
  emptyIcon: JSX.Element;
  workloads?: readonly Resource[];
  servers?: readonly Resource[];
}> = (props) => {
  let inspection: HTMLDivElement | undefined;
  // Recovering a read removes its notice (or the whole-inventory error).
  // Hand off only focus owned by that retiring content, never focus the
  // operator has moved elsewhere or focus after leaving this surface.
  const retainRecoveryFocus = (notice: HTMLElement): void => {
    onCleanup(() => {
      if (!notice.contains(document.activeElement)) return;
      queueMicrotask(() => {
        if (!inspection?.isConnected || document.activeElement !== document.body) return;
        const activeView = inspection.querySelector<HTMLElement>(
          'nav[aria-label="Backup views"] [aria-current="page"]',
        );
        (activeView ?? inspection).focus();
      });
    });
  };
  // Independent read owners contain source failures and replace pending reads
  // on org changes/disposal. Refresh while mounted at the same bounded cadence
  // as protection posture; no cross-route inventory cache or per-row reads.
  const backups = createNonSuspendingQuery({
    source: () => 'pve',
    fetcher: (_source, signal) => fetchPVEBackups(signal),
    initialValue: EMPTY_PVE_BACKUPS,
    pollMs: BACKUP_INVENTORY_POLL_MS,
  });
  const pbsBackups = createNonSuspendingQuery({
    source: () => 'pbs',
    fetcher: (_source, signal) => fetchPBSBackups(signal),
    initialValue: EMPTY_PBS_BACKUPS,
    pollMs: BACKUP_INVENTORY_POLL_MS,
  });
  const inventorySources = [
    { label: 'PVE', query: backups },
    { label: 'PBS', query: pbsBackups },
  ];
  const inventoryIncomplete = () =>
    inventorySources.some(({ query }) => !query.resolvedOnce() || Boolean(query.error()));
  const hasReadableInventory = () =>
    inventorySources.some(({ query }) => query.resolvedOnce() && !query.error());
  const allInventoriesFailed = () => inventorySources.every(({ query }) => Boolean(query.error()));
  const retryAllInventories = () => {
    if (inventorySources.some(({ query }) => query.loading())) return;
    void Promise.all([backups.refetch(), pbsBackups.refetch(), protectionPostures.refetch()]);
  };
  const { isMobile } = useBreakpoint();
  const location = useLocation();
  const protectionPostures = useProtectionPostures(() =>
    (props.workloads ?? []).map((workload) => workload.id),
  );

  // Saved views persist the current route query, so every filter and the active
  // backup workspace must share one URL owner. View changes also clear the
  // incompatible per-view facet in the same navigation so hidden Posture,
  // Source, or day filters cannot survive behind the other workspace.
  const [searchParams, setSearchParams] = useSearchParams();
  const queryValue = (key: string): string => {
    const value = searchParams[key];
    return typeof value === 'string' ? value : '';
  };
  const search = (): string => queryValue(PROXMOX_BACKUPS_QUERY_PARAMS.query);
  const setSearch = (value: string): void =>
    setSearchParams({ [PROXMOX_BACKUPS_QUERY_PARAMS.query]: value || null }, { replace: true });
  const routeView = (): BackupView | null => {
    const normalizedPath = location.pathname.replace(/\/+$/, '');
    if (!normalizedPath.startsWith(`${PROXMOX_BACKUPS_PATH}/`)) return null;
    const segment = normalizedPath.slice(`${PROXMOX_BACKUPS_PATH}/`.length).split('/')[0];
    return segment === 'date' || segment === 'coverage' ? segment : null;
  };
  const legacyQueryView = (): BackupView | null => {
    const value = queryValue(PROXMOX_BACKUPS_QUERY_PARAMS.view);
    return value === 'date' || value === 'coverage' ? value : null;
  };
  const coverageFilter = (): CoverageFilterValue => {
    const value = queryValue(PROXMOX_BACKUPS_QUERY_PARAMS.posture);
    return value === 'attention' ||
      value === 'protected' ||
      value === 'unprotected' ||
      value === 'unknown'
      ? value
      : 'all';
  };
  const setCoverageFilter = (value: CoverageFilterValue): void =>
    setSearchParams(
      {
        [PROXMOX_BACKUPS_QUERY_PARAMS.posture]: value === 'all' ? null : value,
      },
      { replace: true },
    );
  const recoverableFilter = (): RecoverableFilterValue => {
    const value = queryValue(PROXMOX_BACKUPS_QUERY_PARAMS.source);
    return value === 'pbs' ||
      value === 'archive' ||
      value === 'snapshot' ||
      value === 'verified' ||
      value === 'unverified'
      ? value
      : 'all';
  };
  const setRecoverableFilter = (value: RecoverableFilterValue): void =>
    setSearchParams(
      {
        [PROXMOX_BACKUPS_QUERY_PARAMS.source]: value === 'all' ? null : value,
      },
      { replace: true },
    );
  const selectedDateKey = (): string | null => {
    const value = queryValue(PROXMOX_BACKUPS_QUERY_PARAMS.day);
    return /^\d{4}-\d{2}-\d{2}$/.test(value) ? value : null;
  };
  const setSelectedDateKey = (value: string | null): void =>
    setSearchParams({ [PROXMOX_BACKUPS_QUERY_PARAMS.day]: value || null }, { replace: true });

  const nodeFilter = (): string => queryValue(PROXMOX_BACKUPS_QUERY_PARAMS.node);
  const setNodeFilter = (value: string): void =>
    setSearchParams({ [PROXMOX_BACKUPS_QUERY_PARAMS.node]: value || null }, { replace: true });
  const nodeMatches = (node: string | undefined): boolean => {
    const selected = nodeFilter();
    return !selected || node === selected;
  };

  const locationFilter = (): string => queryValue(PROXMOX_BACKUPS_QUERY_PARAMS.location);
  const setLocationFilter = (value: string): void =>
    setSearchParams({ [PROXMOX_BACKUPS_QUERY_PARAMS.location]: value || null }, { replace: true });
  const locationMatches = (artifact: RecoverableArtifact): boolean => {
    const selected = locationFilter();
    return (
      !selected ||
      Boolean(locationSelection().key && artifact.locationKey === locationSelection().key)
    );
  };

  // Type scope normalizes vm/qemu and ct/lxc to a canonical token before
  // comparing, since the recovery model keeps PBS-native ct internally.
  const typeFilter = (): string => queryValue(PROXMOX_BACKUPS_QUERY_PARAMS.type);
  const setTypeFilter = (value: string): void =>
    setSearchParams({ [PROXMOX_BACKUPS_QUERY_PARAMS.type]: value || null }, { replace: true });
  const canonicalGuestType = (raw: string | undefined): string => {
    const t = (raw ?? '').toLowerCase();
    if (t === 'vm' || t === 'qemu') return 'vm';
    if (t === 'ct' || t === 'lxc') return 'ct';
    if (t === 'host') return 'host';
    return 'other';
  };
  const typeMatches = (raw: string | undefined): boolean => {
    const selected = typeFilter();
    return !selected || canonicalGuestType(raw) === selected;
  };
  const typeFilterOptions: FilterSelectOption[] = [
    { value: '', label: 'All types' },
    { value: 'vm', label: 'VMs' },
    { value: 'ct', label: 'LXCs' },
    { value: 'host', label: 'Hosts' },
  ];

  const [coverageSortKey, setCoverageSortKey] = createSignal<CoverageSortKey>('posture');
  const [coverageSortDirection, setCoverageSortDirection] = createSignal<'asc' | 'desc'>('asc');
  // One workload drawer at a time, across the live and unmatched tables.
  const [expandedCoverageKey, setExpandedCoverageKey] = createSignal<string | null>(null);
  // Unmatched backups (VM/LXC evidence without an unambiguous live identity)
  // are collapsed by default so the main table is active backup targets, not a
  // pile of unassigned artifacts sorted to the top.
  const [showOrphaned, setShowOrphaned] = createSignal(false);

  // Chronological feed sort and activity-chart controls.
  const [recoverableSortKey, setRecoverableSortKey] = createSignal<RecoverableSortKey>('created');
  const [recoverableSortDirection, setRecoverableSortDirection] = createSignal<'asc' | 'desc'>(
    'desc',
  );
  const [chartRange, setChartRange] = createSignal<BackupActivityRangeDays>(30);
  const [recoverableMetricMode, setRecoverableMetricMode] =
    createSignal<BackupActivityMetricMode>('count');

  const handleCoverageSort = (key: CoverageSortKey) => {
    if (coverageSortKey() === key) {
      setCoverageSortDirection(coverageSortDirection() === 'asc' ? 'desc' : 'asc');
    } else {
      setCoverageSortKey(key);
      setCoverageSortDirection(COVERAGE_SORT_DEFAULT_DIRECTION[key]);
    }
  };
  const handleRecoverableSort = (key: RecoverableSortKey) => {
    if (recoverableSortKey() === key) {
      setRecoverableSortDirection(recoverableSortDirection() === 'asc' ? 'desc' : 'asc');
    } else {
      setRecoverableSortKey(key);
      setRecoverableSortDirection(RECOVERABLE_SORT_DEFAULT_DIRECTION[key]);
    }
  };
  const toggleCoverageExpansion = (key: string) =>
    setExpandedCoverageKey((current) => (current === key ? null : key));
  const toggleDay = (key: string) => setSelectedDateKey(selectedDateKey() === key ? null : key);

  // Retain readable observations while a retry is pending, but never treat a
  // settled failed read as a complete inventory or expose its old artifacts.
  const pbsArtifacts = createMemo<PBSBackup[]>(() =>
    pbsBackups.error() ? [] : (pbsBackups.value().backups ?? []),
  );
  const snapshots = createMemo<GuestSnapshot[]>(() =>
    backups.error() ? [] : (backups.value().guestSnapshots ?? []),
  );
  const archives = createMemo<StorageBackup[]>(() =>
    backups.error() ? [] : (backups.value().storageBackups ?? []),
  );
  const tasks = createMemo<BackupTask[]>(() =>
    backups.error() ? [] : (backups.value().backupTasks ?? []),
  );

  // Share the observation moment, refreshing it when inventory is replaced.
  // A newly completed backup must not be "future" relative to page mount.
  const nowMs = createMemo(() => {
    pbsArtifacts();
    archives();
    snapshots();
    return Date.now();
  });

  const recoveryModel = createMemo(() =>
    buildProxmoxBackupRecoveryModel({
      workloads: props.workloads ?? [],
      pbsBackups: pbsArtifacts(),
      archives: archives(),
      snapshots: snapshots(),
      tasks: tasks(),
      nowMs: nowMs(),
      protectionPostures: protectionPostures.postureByResourceID(),
      protectionPosturesResolved: protectionPostures.resolvedOnce(),
    }),
  );

  const locationSelection = createMemo(() =>
    resolveBackupLocationFilter(locationFilter(), recoveryModel().recoverableArtifacts),
  );
  // A location narrows restore evidence, not the server's workload assessment
  // or task history. Name only a resolved inventory location, never URL text.
  const evidenceLocationLabel = createMemo(() => {
    const key = locationSelection().key;
    return key
      ? recoveryModel().recoverableArtifacts.find((artifact) => artifact.locationKey === key)
          ?.locationLabel
      : undefined;
  });

  // Node options for the scope filter: the distinct nodes across the workload
  // set, "All nodes" first. PBS snapshots carry no node, so this scopes the
  // node-bearing rows only.
  const nodeOptions = createMemo<FilterSelectOption[]>(() => {
    const nodes = new Set<string>();
    for (const row of recoveryModel().coverageRows) {
      const node = row.workload.node?.trim();
      if (node) nodes.add(node);
    }
    return [
      { value: '', label: 'All nodes' },
      ...[...nodes]
        .sort((a, b) => a.localeCompare(b))
        .map((node) => ({ value: node, label: node })),
    ];
  });

  const locationOptions = createMemo<FilterSelectOption[]>(() => {
    const locations = new Map<string, string>();
    for (const artifact of recoveryModel().recoverableArtifacts) {
      if (artifact.locationKey && artifact.locationLabel) {
        locations.set(artifact.locationKey, artifact.locationLabel);
      }
    }
    return [
      { value: '', label: 'All backup locations' },
      ...[...locations]
        .sort((a, b) => a[1].localeCompare(b[1]))
        .map(([value, label]) => ({ value, label })),
    ];
  });

  // The task column auto-hides when no workload has task history (computed
  // over the full set so it does not flicker as filters change).
  const coverageHasTask = createMemo(() =>
    recoveryModel().coverageRows.some((row) =>
      Boolean(row.latestTask || row.taskCandidates?.length),
    ),
  );

  const coverageRowsForLocation = createMemo<WorkloadCoverageRow[]>(() => {
    const selected = locationFilter();
    if (!selected) return recoveryModel().coverageRows;
    return recoveryModel().coverageRows.flatMap((row) => {
      const artifacts = row.artifacts.filter(locationMatches);
      if (artifacts.length === 0) return [];
      return [
        {
          ...row,
          artifacts,
          ...selectWorkloadRecoveryArtifacts(artifacts, nowMs()),
          pbsCount: artifacts.filter((artifact) => artifact.sourceKind === 'pbs').length,
          archiveCount: artifacts.filter((artifact) => artifact.sourceKind === 'archive').length,
          snapshotCount: artifacts.filter((artifact) => artifact.sourceKind === 'snapshot').length,
        },
      ];
    });
  });

  const filteredCoverageRows = createMemo(() => {
    const term = search().trim().toLowerCase();
    const filter = coverageFilter();
    const list = coverageRowsForLocation().filter((row) => {
      if (!nodeMatches(row.workload.node)) return false;
      if (!typeMatches(row.workload.type)) return false;
      if (filter !== 'all' && row.posture !== filter) return false;
      return coverageRowMatchesSearch(row, term);
    });
    const sortKey = coverageSortKey();
    const direction = coverageSortDirection();
    list.sort((a, b) => {
      switch (sortKey) {
        case 'posture':
          return cmpNumber(a.postureRank, b.postureRank, direction);
        case 'workload':
          return cmpString(a.workload.label, b.workload.label, direction);
        case 'latest':
          return cmpNumber(a.latestBackup?.createdMs, b.latestBackup?.createdMs, direction);
        case 'task':
          return cmpNumber(a.latestTask?.startedMs, b.latestTask?.startedMs, direction);
      }
    });
    return list;
  });

  // The main table is the user's active backup targets: live inventory guests
  // plus first-class host backups. Orphaned VM/LXC backup records are partitioned
  // out into their own collapsed section so they don't dominate the top of the
  // list with nameless rows.
  const liveCoverageRows = createMemo(() =>
    filteredCoverageRows().filter((row) => !row.isOrphaned),
  );
  const orphanedCoverageRows = createMemo(() =>
    filteredCoverageRows().filter((row) => row.isOrphaned),
  );
  const liveTotalCount = createMemo(
    () => recoveryModel().coverageRows.filter((row) => !row.isOrphaned).length,
  );
  const visibleLiveCount = createMemo(() => liveCoverageRows().length);
  const orphanedTotalCount = createMemo(
    () => recoveryModel().coverageRows.filter((row) => row.isOrphaned).length,
  );

  // Health strip counts active backup targets only — orphaned records are all
  // stale and would otherwise inflate the "attention" segment with dead data.
  const liveHealthSummary = createMemo(() => {
    const live = recoveryModel().coverageRows.filter((row) => !row.isOrphaned);
    return {
      protected: live.filter((row) => row.posture === 'protected').length,
      attention: live.filter((row) => row.posture === 'attention').length,
      unprotected: live.filter((row) => row.posture === 'unprotected').length,
      unknown: live.filter((row) => row.posture === 'unknown').length,
      checking: live.filter((row) => row.posture === 'checking').length,
      notEvaluated: live.filter((row) => row.posture === 'not-evaluated').length,
    };
  });
  const view = (): BackupView =>
    routeView() ??
    legacyQueryView() ??
    (liveHealthSummary().attention + liveHealthSummary().unprotected + liveHealthSummary().unknown >
    0
      ? 'coverage'
      : PROXMOX_BACKUPS_DEFAULT_VIEW);

  const backupViewPath = (next: BackupView): string => {
    const params = new URLSearchParams(location.search);
    params.delete(PROXMOX_BACKUPS_QUERY_PARAMS.view);
    if (next === 'date') {
      params.delete(PROXMOX_BACKUPS_QUERY_PARAMS.posture);
    } else {
      params.delete(PROXMOX_BACKUPS_QUERY_PARAMS.source);
      params.delete(PROXMOX_BACKUPS_QUERY_PARAMS.day);
    }
    const query = params.toString();
    return `${buildProxmoxBackupsPath(next)}${query ? `?${query}` : ''}`;
  };
  // The health strip's counts that name a posture filter toggle it. From the
  // date view a count opens Coverage already filtered to those workloads.
  const navigate = useNavigate();
  // Old bookmarks can enter without a view segment. Pick their useful default
  // only after the first assessment settles, then let the existing URL own it.
  // A later poll must update health, not replace the workspace under inspection.
  createEffect(() => {
    if (
      location.pathname.replace(/\/+$/, '') !== PROXMOX_BACKUPS_PATH ||
      legacyQueryView() ||
      !hasReadableInventory() ||
      ((props.workloads?.length ?? 0) > 0 && !protectionPostures.resolvedOnce())
    ) {
      return;
    }
    navigate(`${backupViewPath(view())}${location.hash}`, { replace: true, scroll: false });
  });
  const postureSegmentFilter = (posture: Exclude<CoverageFilterValue, 'all'>) => {
    const pressed = view() === 'coverage' && coverageFilter() === posture;
    // A zero count would only open an empty list.
    if (!pressed && liveHealthSummary()[posture] === 0) return {};
    return {
      pressed,
      onSelect: () => {
        const next = coverageFilter() === posture && view() === 'coverage' ? 'all' : posture;
        if (routeView() === 'coverage') {
          setCoverageFilter(next);
          return;
        }
        // By date, or Coverage chosen only by the health fallback: land on the
        // Coverage route so the view cannot flip back when the counts change.
        const target = new URL(backupViewPath('coverage'), 'https://pulse.invalid');
        if (next === 'all') target.searchParams.delete(PROXMOX_BACKUPS_QUERY_PARAMS.posture);
        else target.searchParams.set(PROXMOX_BACKUPS_QUERY_PARAMS.posture, next);
        navigate(`${target.pathname}${target.search}`);
      },
    };
  };
  const backupViewTabs = createMemo(() =>
    BACKUP_VIEW_TABS.map((tab) => ({ ...tab, path: backupViewPath(tab.id) })),
  );

  // Recoverable artifact feed: PBS snapshots, PVE backup files, and guest snapshot rows.
  const RECOVERABLE_SEGMENT_KINDS: readonly BackupActivitySegmentKind[] = [
    'pbs',
    'archive',
    'snapshot',
  ];

  // The chart and list share every inventory facet. Day selection narrows
  // only the list so other days remain available to select in the chart.
  const scopedRecoverableArtifacts = createMemo(() => {
    const term = search().trim().toLowerCase();
    const filter = recoverableFilter();
    return recoveryModel().recoverableArtifacts.filter((artifact) => {
      if (!nodeMatches(artifact.workload.node)) return false;
      if (!typeMatches(artifact.workload.type)) return false;
      if (!locationMatches(artifact)) return false;
      if (
        (filter === 'pbs' || filter === 'archive' || filter === 'snapshot') &&
        artifact.sourceKind !== filter
      ) {
        return false;
      }
      if (filter === 'verified' && artifact.verified !== true) return false;
      if (filter === 'unverified' && artifact.verified !== false) return false;
      return recoverableArtifactMatchesSearch(artifact, term);
    });
  });

  const recoverableTimeline = createMemo(() =>
    buildBackupActivityTimeline<RecoverableArtifact>(
      chartRange(),
      scopedRecoverableArtifacts(),
      (artifact) => artifact.createdMs,
      (artifact) =>
        !isCompletedRecoveryArtifact(artifact)
          ? null
          : artifact.sourceKind === 'pbs'
            ? 'pbs'
            : artifact.sourceKind === 'archive'
              ? 'archive'
              : 'snapshot',
      {
        now: new Date(nowMs()),
        getValue:
          recoverableMetricMode() === 'volume'
            ? (artifact) => (artifact.size && artifact.size > 0 ? artifact.size : 0)
            : undefined,
      },
    ),
  );
  const hasRecoverableActivityInRange = createMemo(() =>
    recoverableTimeline().points.some(
      (point) => point.total > 0 || getBackupActivityUnknownCount(point) > 0,
    ),
  );
  const unknownVolumeCount = createMemo(() =>
    recoverableTimeline().points.reduce(
      (sum, point) => sum + getBackupActivityUnknownCount(point),
      0,
    ),
  );

  const filteredRecoverableArtifacts = createMemo(() => {
    const dateKey = selectedDateKey();
    const list = scopedRecoverableArtifacts().filter(
      (artifact) =>
        !dateKey ||
        (artifact.createdMs !== undefined &&
          recoveryDateKeyFromTimestamp(artifact.createdMs) === dateKey),
    );
    const sortKey = recoverableSortKey();
    const direction = recoverableSortDirection();
    list.sort((a, b) => {
      switch (sortKey) {
        case 'workload':
          return cmpString(a.workload.label, b.workload.label, direction);
        case 'source':
          return cmpString(a.sourceLabel, b.sourceLabel, direction);
        case 'location':
          return cmpString(a.location, b.location, direction);
        case 'created':
          return cmpNumber(a.createdMs, b.createdMs, direction);
        case 'size':
          return cmpNumber(a.size, b.size, direction);
        case 'state':
          return cmpString(artifactStateLabel(a), artifactStateLabel(b), direction);
      }
    });
    return list;
  });

  const recoverableSizeMaxBytes = createMemo(() => {
    let max = 0;
    for (const artifact of filteredRecoverableArtifacts()) {
      if (artifact.size && artifact.size > max) max = artifact.size;
    }
    return max;
  });

  const totalRecoverableCount = createMemo(() => recoveryModel().recoverableArtifacts.length);
  const visibleRecoverableCount = createMemo(() => filteredRecoverableArtifacts().length);

  // Shared scope filters plus the per-view inline facet.
  const buildBackupsFilters = (): FilterDef[] => {
    const filters: FilterDef[] = [
      {
        id: 'node',
        label: 'Node',
        group: 'scope',
        options: nodeOptions,
        value: nodeFilter,
        setValue: setNodeFilter,
        defaultValue: '',
      },
      {
        id: 'location',
        label: 'Backup location',
        group: 'scope',
        options: locationOptions,
        value: () => locationSelection().key ?? locationFilter(),
        formatChipValue: (value, options) =>
          options.find((option) => option.value === value)?.label ?? 'Choose location',
        setValue: setLocationFilter,
        defaultValue: '',
      },
      {
        id: 'type',
        label: 'Type',
        group: 'scope',
        options: () => typeFilterOptions,
        value: typeFilter,
        setValue: setTypeFilter,
        defaultValue: '',
      },
    ];
    if (view() === 'date') {
      filters.push({
        id: 'source',
        label: 'Source',
        group: 'status',
        inline: true,
        options: () => RECOVERABLE_FILTERS,
        value: recoverableFilter,
        setValue: (v) => setRecoverableFilter(v as RecoverableFilterValue),
        defaultValue: 'all',
      });
    } else {
      filters.push({
        id: 'posture',
        label: 'Posture',
        group: 'status',
        inline: true,
        options: () => COVERAGE_FILTERS,
        value: coverageFilter,
        setValue: (v) => setCoverageFilter(v as CoverageFilterValue),
        defaultValue: 'all',
      });
    }
    return filters;
  };
  const hasActiveBackupsFilters = (): boolean =>
    Boolean(
      search().trim() ||
      nodeFilter() ||
      locationFilter() ||
      typeFilter() ||
      coverageFilter() !== 'all' ||
      recoverableFilter() !== 'all' ||
      selectedDateKey(),
    );
  const resetBackupsFilters = (): void =>
    setSearchParams(
      {
        [PROXMOX_BACKUPS_QUERY_PARAMS.query]: null,
        [PROXMOX_BACKUPS_QUERY_PARAMS.view]: null,
        [PROXMOX_BACKUPS_QUERY_PARAMS.node]: null,
        [PROXMOX_BACKUPS_QUERY_PARAMS.location]: null,
        [PROXMOX_BACKUPS_QUERY_PARAMS.type]: null,
        [PROXMOX_BACKUPS_QUERY_PARAMS.source]: null,
        [PROXMOX_BACKUPS_QUERY_PARAMS.posture]: null,
        [PROXMOX_BACKUPS_QUERY_PARAMS.day]: null,
      },
      { replace: true },
    );

  return (
    <div
      ref={inspection}
      role="region"
      aria-label="Backup inspection"
      tabIndex={-1}
      class="space-y-3"
    >
      <For each={inventorySources}>
        {({ label, query }) => (
          <Show when={!query.resolvedOnce() || query.error()}>
            <div
              ref={retainRecoveryFocus}
              role="status"
              class={`${BACKUP_READ_NOTICE_CLASS} flex flex-wrap items-center justify-between gap-2`}
            >
              <span>
                {label} backup inventory is {query.error() ? 'unavailable' : 'loading'}. Counts are
                incomplete. {getAPIReadAccessErrorMessage(query.error())}
                <Show when={query.error() instanceof BackupInventoryFormatError}>
                  {' '}
                  The response format is invalid. Missing restore points do not mean no backups
                  exist.
                </Show>
              </span>
              <Show when={query.error()}>
                <button
                  type="button"
                  class="min-h-10 shrink-0 rounded-md border border-current px-3 font-medium hover:underline focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 aria-disabled:opacity-50"
                  aria-label={`Retry ${label} inventory`}
                  aria-disabled={query.loading()}
                  onClick={() => {
                    if (!query.loading()) void query.refetch();
                  }}
                >
                  {query.loading() ? 'Retrying…' : `Retry ${label} inventory`}
                </button>
              </Show>
            </div>
          </Show>
        )}
      </For>
      {/* Server health comes from /api/resources, not either backup inventory.
          Keep this owner mounted through loading, failure and ordinary retries. */}
      <Show when={(props.servers?.length ?? 0) > 0}>
        <ProxmoxBackupServersTable
          servers={props.servers ?? []}
          backups={pbsArtifacts()}
          backupInventoryState={
            pbsBackups.error() ? 'unavailable' : pbsBackups.resolvedOnce() ? 'available' : 'loading'
          }
        />
      </Show>
      <Show
        when={!allInventoriesFailed()}
        fallback={
          <div ref={retainRecoveryFocus}>
            <PlatformErrorState
              title="Could not load Proxmox backup inventory"
              description="PVE and PBS backup inventories are unavailable. Counts are incomplete. Refresh to retry both sources."
              onRefresh={retryAllInventories}
            />
          </div>
        }
      >
        <Show
          when={
            hasReadableInventory() || inventorySources.some(({ query }) => Boolean(query.error()))
          }
          fallback={
            <PlatformTableLoadingState
              title="Loading Proxmox backup inventory"
              description="Reading PBS snapshots, PVE backup files, guest snapshots, and recent backup tasks."
            />
          }
        >
          <div class="space-y-3">
            <PlatformSectionTabs tabs={backupViewTabs()} active={view()} ariaLabel="Backup views" />
            <Show when={protectionPostures.response.error}>
              <div
                ref={retainRecoveryFocus}
                role="status"
                class={`${BACKUP_READ_NOTICE_CLASS} flex flex-wrap items-center justify-between gap-2`}
              >
                <span>
                  Protection posture is unavailable. Pulse is showing restore evidence without
                  guessing whether workloads are protected.
                </span>
                <button
                  type="button"
                  class="min-h-10 shrink-0 rounded-md border border-current px-3 font-medium hover:underline focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 aria-disabled:opacity-50"
                  aria-label="Retry protection posture"
                  aria-disabled={protectionPostures.response.loading}
                  onClick={() => {
                    if (!protectionPostures.response.loading) void protectionPostures.refetch();
                  }}
                >
                  {protectionPostures.response.loading ? 'Retrying…' : 'Retry protection posture'}
                </button>
              </div>
            </Show>

            <Show when={locationSelection().unresolved}>
              <div role="status" class={BACKUP_READ_NOTICE_CLASS}>
                The saved backup location cannot be identified uniquely in the current inventory.
                Choose a backup location again or clear the filter. No restore points are shown for
                this filter.
              </div>
            </Show>

            {/* The health strip renders identically in both route-backed views. */}
            <ProxmoxBackupsCoverageStrip
              title="Backup health"
              footnote={
                <>
                  <span class="font-medium">
                    Health counts cover all targets and backup locations, regardless of filters.
                  </span>
                  <Show when={getBackupPostureRuleText(protectionPostures.policy())}>
                    {(rule) => <span data-coverage-posture-rule> {rule()}</span>}
                  </Show>
                </>
              }
              tail={
                <span>
                  {liveTotalCount()} targets ·{' '}
                  {recoveryModel().coverageSummary.recoverableArtifacts} completed restore{' '}
                  {recoveryModel().coverageSummary.recoverableArtifacts === 1 ? 'point' : 'points'}
                  {inventoryIncomplete() ? ' read' : ''}
                  <Show when={recoveryModel().coverageSummary.runningArtifacts > 0}>
                    {' · '}
                    {recoveryModel().coverageSummary.runningArtifacts} running
                  </Show>
                  <Show when={recoveryModel().coverageSummary.failedArtifacts > 0}>
                    {' · '}
                    {recoveryModel().coverageSummary.failedArtifacts} failed
                  </Show>
                  <Show when={recoveryModel().coverageSummary.withPBS > 0}>
                    {' · '}
                    {recoveryModel().coverageSummary.withPBS} with{' '}
                    {getProxmoxBackupSourcePresentation('pbs').filterLabel}
                  </Show>
                  <Show when={orphanedTotalCount() > 0}>
                    {' · '}
                    {orphanedTotalCount()} unmatched
                  </Show>
                </span>
              }
              segments={[
                {
                  key: 'protected',
                  value: liveHealthSummary().protected,
                  ...postureSegmentFilter('protected'),
                  label: 'protected',
                  toneClass: 'bg-emerald-500',
                },
                {
                  key: 'attention',
                  value: liveHealthSummary().attention,
                  ...postureSegmentFilter('attention'),
                  label: 'attention',
                  toneClass: 'bg-amber-500',
                  muted: liveHealthSummary().attention === 0,
                },
                {
                  key: 'unprotected',
                  value: liveHealthSummary().unprotected,
                  ...postureSegmentFilter('unprotected'),
                  label: 'unprotected',
                  toneClass: 'bg-red-500',
                  muted: liveHealthSummary().unprotected === 0,
                },
                {
                  key: 'unknown',
                  value: liveHealthSummary().unknown,
                  ...postureSegmentFilter('unknown'),
                  label: 'unknown',
                  toneClass: 'bg-slate-400',
                  muted: liveHealthSummary().unknown === 0,
                },
                {
                  key: 'checking',
                  value: liveHealthSummary().checking,
                  label: 'checking',
                  toneClass: 'bg-blue-400',
                  muted: liveHealthSummary().checking === 0,
                },
                {
                  key: 'not-evaluated',
                  value: liveHealthSummary().notEvaluated,
                  label: 'not evaluated',
                  toneClass: 'bg-slate-300 dark:bg-slate-600',
                  muted: liveHealthSummary().notEvaluated === 0,
                },
              ]}
            />

            <Show when={view() === 'date' && recoveryModel().recoverableArtifacts.length > 0}>
              <Show
                when={scopedRecoverableArtifacts().some(
                  (artifact) => artifact.createdMs === undefined,
                )}
              >
                <p class="text-xs text-amber-700 dark:text-amber-300" role="status">
                  Backup entries with unavailable or future dates remain listed. Their ages are
                  unknown. They are excluded from the activity chart and day filters.
                </p>
              </Show>
              <Show
                when={hasRecoverableActivityInRange()}
                fallback={
                  <div class="flex flex-col gap-2 rounded-md border border-border-subtle px-3 py-3 text-xs text-muted sm:flex-row sm:items-center sm:justify-between">
                    <span>
                      No completed restore points with known dates in the selected {chartRange()}
                      -day window.
                    </span>
                    <div class="flex items-center gap-2">
                      <button
                        type="button"
                        class="font-medium text-blue-600 hover:underline dark:text-blue-300"
                        onClick={() => setChartRange(90)}
                      >
                        Show 90 days
                      </button>
                      <span aria-hidden="true">·</span>
                      <button
                        type="button"
                        class="font-medium text-blue-600 hover:underline dark:text-blue-300"
                        onClick={() => setChartRange(365)}
                      >
                        Show 1 year
                      </button>
                    </div>
                  </div>
                }
              >
                <div class="space-y-1">
                  <p class="text-xs text-muted">
                    Activity follows the backup filters. Selecting a day narrows only the list.
                  </p>
                  <Show when={recoverableMetricMode() === 'volume' && unknownVolumeCount() > 0}>
                    <p class="text-xs text-amber-700 dark:text-amber-300" role="status">
                      Sizes are unavailable for {unknownVolumeCount()} completed restore{' '}
                      {unknownVolumeCount() === 1 ? 'point' : 'points'} in this window. Bars show
                      known volume only. Amber marks days with unknown sizes, not zero volume.
                    </p>
                  </Show>
                  <BackupActivityChart
                    title={
                      recoverableMetricMode() === 'volume'
                        ? 'Known restore volume per day'
                        : 'Completed restore points per day'
                    }
                    noun="restore point"
                    segmentKinds={RECOVERABLE_SEGMENT_KINDS}
                    range={chartRange}
                    onRangeChange={setChartRange}
                    timeline={recoverableTimeline}
                    selectedDateKey={selectedDateKey}
                    onToggleDay={toggleDay}
                    metricToggle={{
                      mode: recoverableMetricMode,
                      onChange: setRecoverableMetricMode,
                    }}
                  />
                </div>
              </Show>
            </Show>

            <FilterBar
              role="group"
              ariaLabel="Backups filters"
              isMobile={isMobile}
              search={{
                value: search,
                setValue: setSearch,
                placeholder: 'Search backups by workload, node, source, or status',
              }}
              filters={buildBackupsFilters()}
              showClearAll={hasActiveBackupsFilters}
              onClearAll={resetBackupsFilters}
              searchTrailing={
                <Show when={view() === 'date' && selectedDateKey() !== null}>
                  <button
                    type="button"
                    onClick={() => setSelectedDateKey(null)}
                    class="inline-flex items-center gap-1 rounded-full border border-blue-300 bg-blue-50 px-2 py-0.5 text-[11px] font-medium text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-700 dark:bg-blue-900/30 dark:text-blue-200 dark:hover:bg-blue-900/50"
                    aria-label="Clear date filter"
                  >
                    <span class="uppercase tracking-wide text-[9px] text-blue-600 dark:text-blue-300">
                      Date
                    </span>
                    <span class="font-mono tabular-nums">
                      {getRecoveryFilterDateLabel(selectedDateKey()!)}
                    </span>
                    <span aria-hidden="true">×</span>
                  </button>
                </Show>
              }
              trailingControls={
                <PlatformResourceCounter
                  visible={view() === 'date' ? visibleRecoverableCount() : visibleLiveCount()}
                  total={view() === 'date' ? totalRecoverableCount() : liveTotalCount()}
                  rowNoun={
                    view() === 'date'
                      ? inventoryIncomplete()
                        ? 'backup entries read'
                        : 'backup entries'
                      : 'targets'
                  }
                />
              }
            />

            <Show when={view() === 'date'}>
              <ProxmoxRecoverableTable
                artifacts={filteredRecoverableArtifacts()}
                hasAnyArtifacts={recoveryModel().recoverableArtifacts.length > 0}
                emptyIcon={props.emptyIcon}
                emptyTitle={
                  inventoryIncomplete() ? 'Backup inventory is incomplete' : 'No backups yet'
                }
                emptyDescription={
                  inventoryIncomplete()
                    ? 'Some backup sources are still loading or unavailable. Missing restore points do not mean no backups exist.'
                    : 'PBS snapshots, PVE backup files, and guest snapshots will appear here once backups run.'
                }
                sortKey={recoverableSortKey}
                sortDirection={recoverableSortDirection}
                onSort={handleRecoverableSort}
                sizeMaxBytes={recoverableSizeMaxBytes()}
                groupByDay
              />
            </Show>

            <Show when={view() === 'coverage'}>
              <Show when={evidenceLocationLabel()}>
                {(label) => (
                  <InlineNotice
                    tone="info"
                    role="note"
                    aria-label="Backup location scope"
                    class="text-xs whitespace-normal wrap-anywhere"
                  >
                    Backup ages and restore evidence below use {label()}. Protection assessments and
                    task history are workload-wide, not a rating of this location.
                  </InlineNotice>
                )}
              </Show>
              <ProxmoxCoverageTable
                rows={liveCoverageRows()}
                hasAnyRows={liveTotalCount() > 0}
                emptyIcon={props.emptyIcon}
                emptyTitle="No backup target coverage"
                emptyDescription="VM, container, and host restore posture will appear here once backup data exists."
                sortKey={coverageSortKey}
                sortDirection={coverageSortDirection}
                onSort={handleCoverageSort}
                expandedKey={expandedCoverageKey()}
                onToggleExpand={toggleCoverageExpansion}
                showTaskColumn={coverageHasTask()}
                evidenceLocationLabel={evidenceLocationLabel()}
                historyReadIncomplete={
                  inventoryIncomplete() ||
                  !protectionPostures.resolvedOnce() ||
                  Boolean(protectionPostures.response.error)
                }
              />

              <Show when={orphanedTotalCount() > 0}>
                <div class="rounded-lg border border-border-subtle">
                  <button
                    type="button"
                    onClick={() => setShowOrphaned((v) => !v)}
                    class="flex w-full flex-wrap items-center justify-between gap-2 px-3 py-2 text-left"
                    aria-expanded={showOrphaned()}
                  >
                    <span class="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
                      <ChevronRightIcon
                        class={`h-3.5 w-3.5 transition-transform ${showOrphaned() ? 'rotate-90' : ''}`}
                        aria-hidden="true"
                      />
                      {orphanedTotalCount()} unmatched{' '}
                      {orphanedTotalCount() === 1 ? 'backup' : 'backups'}
                    </span>
                    <span class="w-full text-[11px] text-muted sm:w-auto">
                      No unambiguous inventory match
                    </span>
                  </button>
                  <Show when={showOrphaned()}>
                    <div class="border-t border-border-subtle p-2">
                      <ProxmoxCoverageTable
                        rows={orphanedCoverageRows()}
                        hasAnyRows={orphanedTotalCount() > 0}
                        emptyIcon={props.emptyIcon}
                        emptyTitle="No unmatched backups"
                        emptyDescription="Backups without an unambiguous live guest match will appear here."
                        sortKey={coverageSortKey}
                        sortDirection={coverageSortDirection}
                        onSort={handleCoverageSort}
                        expandedKey={expandedCoverageKey()}
                        onToggleExpand={toggleCoverageExpansion}
                        showTaskColumn={coverageHasTask()}
                        evidenceLocationLabel={evidenceLocationLabel()}
                        historyReadIncomplete={
                          inventoryIncomplete() ||
                          !protectionPostures.resolvedOnce() ||
                          Boolean(protectionPostures.response.error)
                        }
                      />
                    </div>
                  </Show>
                </div>
              </Show>
            </Show>
          </div>
        </Show>
      </Show>
    </div>
  );
};

export default ProxmoxBackupsTable;
