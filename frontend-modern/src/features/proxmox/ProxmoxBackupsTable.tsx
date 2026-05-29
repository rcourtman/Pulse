import {
  For,
  Show,
  createEffect,
  createMemo,
  createResource,
  createSignal,
  type Component,
  type JSX,
} from 'solid-js';
import ArchiveIcon from 'lucide-solid/icons/archive';
import CameraIcon from 'lucide-solid/icons/camera';
import ActivityIcon from 'lucide-solid/icons/activity';
import DatabaseIcon from 'lucide-solid/icons/database';
import ServerIcon from 'lucide-solid/icons/server';
import ShieldCheckIcon from 'lucide-solid/icons/shield-check';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { FilterButtonGroup } from '@/components/shared/FilterButtonGroup';
import { SearchInput } from '@/components/shared/SearchInput';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCard } from '@/components/shared/TableCard';
import type { StatusIndicatorVariant } from '@/utils/status';
import {
  Table,
  TableBody,
  TableCell,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { apiFetch } from '@/utils/apiClient';
import { formatBytes, formatRelativeTime } from '@/utils/format';
import {
  recoveryDateKeyFromTimestamp,
  getRecoveryFilterDateLabel,
} from '@/utils/recoveryDatePresentation';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
} from '@/features/platformPage/sharedPlatformPage';
import type {
  BackupTask,
  GuestSnapshot,
  PBSBackup,
  PBSBackupsPayload,
  PBSBackupsResponse,
  PVEBackupsPayload,
  PVEBackupsResponse,
  StorageBackup,
} from '@/types/api';
import type { Resource } from '@/types/resource';
import { BackupActivityChart } from './BackupActivityChart';
import {
  buildBackupActivityTimeline,
  type BackupActivityMetricMode,
  type BackupActivityRangeDays,
  type BackupActivitySegmentKind,
} from './proxmoxBackupActivityPresentation';
import {
  buildArchiveCoverageSummary,
  buildSnapshotCoverageSummary,
  buildTaskOutcomeSummary,
  computeMedianTaskDurationSeconds,
  getBackupAgeBucketPresentation,
  guestKey,
  taskDurationSeconds,
} from './proxmoxBackupSummaryPresentation';
import {
  buildProxmoxBackupRecoveryModel,
  coverageRowMatchesSearch,
  getWorkloadRecoveryPostureLabel,
  isCoverageAttention,
  recoverableArtifactMatchesSearch,
  type RecoverableArtifact,
  type WorkloadCoverageRow,
} from './proxmoxBackupRecoveryModel';
import {
  ARCHIVE_SORT_DEFAULT_DIRECTION,
  COVERAGE_SORT_DEFAULT_DIRECTION,
  PBS_SORT_DEFAULT_DIRECTION,
  RECOVERABLE_SORT_DEFAULT_DIRECTION,
  SNAPSHOT_SORT_DEFAULT_DIRECTION,
  TASK_SORT_DEFAULT_DIRECTION,
  classifyTaskStatus,
  cmpBool,
  cmpNumber,
  cmpString,
  formatDurationFromSeconds,
  guestLabel,
  pbsRepositoryLabel,
  pbsWorkloadLabel,
  type ArchiveSortKey,
  type BackupTabId,
  type CoverageFilterValue,
  type CoverageSortKey,
  type PBSSortKey,
  type RecoverableFilterValue,
  type RecoverableSortKey,
  type SnapshotFilterValue,
  type SnapshotGuestRow,
  type SnapshotSortKey,
  type SourceDetailTabId,
  type TaskSortKey,
} from './proxmoxBackupsTableModel';
import {
  ARCHIVE_STATUS_FILTERS,
  ArtifactSourceBadge,
  ArtifactStateBadge,
  COVERAGE_FILTERS,
  PBS_STATUS_FILTERS,
  RECOVERABLE_FILTERS,
  RecoverySourceSummary,
  SNAPSHOT_FILTERS,
  SortableHead,
  TASK_STATUS_FILTERS,
  artifactStateLabel,
} from './proxmoxBackupsTableShared';
import { ProxmoxArchivesTable } from './ProxmoxArchivesTable';
import { ProxmoxBackupsCoverageStrip } from './ProxmoxBackupsCoverageStrip';
import { ProxmoxPbsTable } from './ProxmoxPbsTable';
import { ProxmoxRecoverableTable } from './ProxmoxRecoverableTable';
import { ProxmoxSnapshotsTable } from './ProxmoxSnapshotsTable';
import { ProxmoxTasksTable } from './ProxmoxTasksTable';

// Proxmox backups are intentionally organized around operator questions, not
// storage-source mechanics:
//   - Workload coverage answers "does this workload have a backup?" by default.
//   - Restore points answers "what exactly can I restore?" across every source.
//   - Source details keeps PBS/PVE evidence available without making those
//     implementation-specific tables equal-weight primary destinations.
//   - Job history shows whether backup jobs are actually running successfully.

interface BackupTabSpec {
  id: BackupTabId;
  label: string;
  icon: () => JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
}

const TABS: BackupTabSpec[] = [
  {
    id: 'coverage',
    label: 'Workload coverage',
    icon: () => <ShieldCheckIcon class="h-4 w-4" aria-hidden="true" />,
    emptyTitle: 'No workload coverage',
    emptyDescription: 'VM and container restore posture will appear here once backup data exists.',
  },
  {
    id: 'recoverable',
    label: 'Restore points',
    icon: () => <DatabaseIcon class="h-4 w-4" aria-hidden="true" />,
    emptyTitle: 'No recoverable artifacts',
    emptyDescription: 'PBS artifacts, PVE backup files, and snapshots will appear here.',
  },
  {
    id: 'sources',
    label: 'Source details',
    icon: () => <ServerIcon class="h-4 w-4" aria-hidden="true" />,
    emptyTitle: 'No source artifacts',
    emptyDescription: 'PBS artifacts, PVE backup files, and guest snapshots will appear here.',
  },
  {
    id: 'tasks',
    label: 'Job history',
    icon: () => <ActivityIcon class="h-4 w-4" aria-hidden="true" />,
    emptyTitle: 'No recent backup tasks',
    emptyDescription: 'Backup-job task results from the past few days will appear here.',
  },
];

function tabSpecFor(id: BackupTabId): BackupTabSpec {
  return TABS.find((spec) => spec.id === id)!;
}

interface SourceDetailTabSpec {
  id: SourceDetailTabId;
  label: string;
  icon: () => JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
}

const SOURCE_DETAIL_TABS: SourceDetailTabSpec[] = [
  {
    id: 'pbs',
    label: 'PBS artifacts',
    icon: () => <ServerIcon class="h-4 w-4" aria-hidden="true" />,
    emptyTitle: 'No PBS artifacts',
    emptyDescription: 'Deduplicated backup snapshots from Proxmox Backup Server will appear here.',
  },
  {
    id: 'snapshots',
    label: 'Snapshots',
    icon: () => <CameraIcon class="h-4 w-4" aria-hidden="true" />,
    emptyTitle: 'No guest snapshots',
    emptyDescription: 'qm/pct snapshots taken on PVE hosts will appear here.',
  },
  {
    id: 'archives',
    label: 'Backup files',
    icon: () => <ArchiveIcon class="h-4 w-4" aria-hidden="true" />,
    emptyTitle: 'No backup archives',
    emptyDescription: 'vzdump archives written to PVE-attached storage will appear here.',
  },
];

function sourceDetailSpecFor(id: SourceDetailTabId): SourceDetailTabSpec {
  return SOURCE_DETAIL_TABS.find((spec) => spec.id === id)!;
}

// Replication colours the per-row status word to match its dot (emerald
// text for Healthy, red for Failed, etc.). Mirror that here so the
// Recent tasks STATUS column reads the same way — dot + same-tone word.
async function fetchPVEBackups(): Promise<PVEBackupsPayload> {
  const response = await apiFetch('/api/backups/pve');
  if (!response.ok) {
    throw new Error(`Failed to load PVE backups (${response.status})`);
  }
  const payload = (await response.json()) as PVEBackupsResponse;
  return (
    payload?.data ?? {
      backupTasks: [],
      storageBackups: [],
      guestSnapshots: [],
    }
  );
}

async function fetchPBSBackups(): Promise<PBSBackupsPayload> {
  const response = await apiFetch('/api/backups/pbs');
  if (!response.ok) {
    throw new Error(`Failed to load PBS backups (${response.status})`);
  }
  const payload = (await response.json()) as PBSBackupsResponse;
  return payload?.data ?? { backups: [] };
}

export const ProxmoxBackupsTable: Component<{
  emptyIcon: JSX.Element;
  hasPBS?: boolean;
  workloads?: readonly Resource[];
}> = (props) => {
  const [backups, { refetch }] = createResource<PVEBackupsPayload>(fetchPVEBackups);
  const [pbsBackups, { refetch: refetchPBS }] = createResource<PBSBackupsPayload>(fetchPBSBackups);
  const [tab, setTab] = createSignal<BackupTabId>('coverage');
  const [sourceDetailTab, setSourceDetailTab] = createSignal<SourceDetailTabId>('pbs');
  const [search, setSearch] = createSignal('');
  const [coverageFilter, setCoverageFilter] = createSignal<CoverageFilterValue>('all');
  const [recoverableFilter, setRecoverableFilter] = createSignal<RecoverableFilterValue>('all');
  const [pbsFilter, setPBSFilter] = createSignal<'all' | 'protected' | 'verified' | 'unverified'>(
    'all',
  );
  const [archiveFilter, setArchiveFilter] = createSignal<
    'all' | 'protected' | 'verified' | 'unverified'
  >('all');
  const [taskFilter, setTaskFilter] = createSignal<'all' | 'ok' | 'failed' | 'running'>('all');
  const [snapshotFilter, setSnapshotFilter] = createSignal<SnapshotFilterValue>('all');
  const [chartRange, setChartRange] = createSignal<BackupActivityRangeDays>(30);
  const [selectedDateKey, setSelectedDateKey] = createSignal<string | null>(null);
  const [recoverableMetricMode, setRecoverableMetricMode] =
    createSignal<BackupActivityMetricMode>('count');
  const [pbsMetricMode, setPBSMetricMode] = createSignal<BackupActivityMetricMode>('count');
  const [archiveMetricMode, setArchiveMetricMode] = createSignal<BackupActivityMetricMode>('count');
  const [expandedGuests, setExpandedGuests] = createSignal<ReadonlySet<string>>(new Set<string>());
  const [expandedCoverageRows, setExpandedCoverageRows] = createSignal<ReadonlySet<string>>(
    new Set<string>(),
  );

  const [coverageSortKey, setCoverageSortKey] = createSignal<CoverageSortKey>('posture');
  const [coverageSortDirection, setCoverageSortDirection] = createSignal<'asc' | 'desc'>('asc');
  const [recoverableSortKey, setRecoverableSortKey] = createSignal<RecoverableSortKey>('created');
  const [recoverableSortDirection, setRecoverableSortDirection] = createSignal<'asc' | 'desc'>(
    'desc',
  );
  const [pbsSortKey, setPBSSortKey] = createSignal<PBSSortKey>('created');
  const [pbsSortDirection, setPBSSortDirection] = createSignal<'asc' | 'desc'>('desc');
  const [snapshotSortKey, setSnapshotSortKey] = createSignal<SnapshotSortKey>('latest');
  const [snapshotSortDirection, setSnapshotSortDirection] = createSignal<'asc' | 'desc'>('desc');
  const [archiveSortKey, setArchiveSortKey] = createSignal<ArchiveSortKey>('created');
  const [archiveSortDirection, setArchiveSortDirection] = createSignal<'asc' | 'desc'>('desc');
  const [taskSortKey, setTaskSortKey] = createSignal<TaskSortKey>('started');
  const [taskSortDirection, setTaskSortDirection] = createSignal<'asc' | 'desc'>('desc');

  const activeTabs = createMemo(() => TABS);
  createEffect(() => {
    if (!activeTabs().some((spec) => spec.id === tab())) {
      setTab(activeTabs()[0]?.id ?? 'coverage');
    }
  });

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
  const handlePBSSort = (key: PBSSortKey) => {
    if (pbsSortKey() === key) {
      setPBSSortDirection(pbsSortDirection() === 'asc' ? 'desc' : 'asc');
    } else {
      setPBSSortKey(key);
      setPBSSortDirection(PBS_SORT_DEFAULT_DIRECTION[key]);
    }
  };
  const handleSnapshotSort = (key: SnapshotSortKey) => {
    if (snapshotSortKey() === key) {
      setSnapshotSortDirection(snapshotSortDirection() === 'asc' ? 'desc' : 'asc');
    } else {
      setSnapshotSortKey(key);
      setSnapshotSortDirection(SNAPSHOT_SORT_DEFAULT_DIRECTION[key]);
    }
  };
  const handleArchiveSort = (key: ArchiveSortKey) => {
    if (archiveSortKey() === key) {
      setArchiveSortDirection(archiveSortDirection() === 'asc' ? 'desc' : 'asc');
    } else {
      setArchiveSortKey(key);
      setArchiveSortDirection(ARCHIVE_SORT_DEFAULT_DIRECTION[key]);
    }
  };
  const handleTaskSort = (key: TaskSortKey) => {
    if (taskSortKey() === key) {
      setTaskSortDirection(taskSortDirection() === 'asc' ? 'desc' : 'asc');
    } else {
      setTaskSortKey(key);
      setTaskSortDirection(TASK_SORT_DEFAULT_DIRECTION[key]);
    }
  };
  const toggleDay = (key: string) =>
    setSelectedDateKey((current) => (current === key ? null : key));
  const toggleGuestExpansion = (key: string) =>
    setExpandedGuests((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  const toggleCoverageExpansion = (key: string) =>
    setExpandedCoverageRows((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  const pbsArtifacts = createMemo<PBSBackup[]>(() => pbsBackups()?.backups ?? []);
  const snapshots = createMemo<GuestSnapshot[]>(() => backups()?.guestSnapshots ?? []);
  const archives = createMemo<StorageBackup[]>(() => backups()?.storageBackups ?? []);
  const tasks = createMemo<BackupTask[]>(() => backups()?.backupTasks ?? []);
  const activeSourceDetailTabs = createMemo(() =>
    props.hasPBS ? SOURCE_DETAIL_TABS : SOURCE_DETAIL_TABS.filter((spec) => spec.id !== 'pbs'),
  );
  createEffect(() => {
    if (!activeSourceDetailTabs().some((spec) => spec.id === sourceDetailTab())) {
      setSourceDetailTab(activeSourceDetailTabs()[0]?.id ?? 'snapshots');
    }
  });
  const sourceDetailTotal = (id: SourceDetailTabId): number => {
    if (id === 'pbs') return pbsArtifacts().length;
    if (id === 'snapshots') return snapshots().length;
    return archives().length;
  };
  const totalSourceArtifacts = createMemo(
    () => (props.hasPBS ? pbsArtifacts().length : 0) + snapshots().length + archives().length,
  );
  // Render-time `now` used for age bucketing. We snapshot once per render
  // so all comparisons within a single render share a reference moment;
  // not reactive to ticking time (good enough for sysadmin grouping).
  const nowMs = createMemo(() => Date.now());

  const snapshotCoverage = createMemo(() => buildSnapshotCoverageSummary(snapshots(), nowMs()));
  const archiveCoverage = createMemo(() => buildArchiveCoverageSummary(archives(), nowMs()));
  const taskOutcome = createMemo(() => buildTaskOutcomeSummary(tasks()));
  const recoveryModel = createMemo(() =>
    buildProxmoxBackupRecoveryModel({
      workloads: props.workloads ?? [],
      pbsBackups: pbsArtifacts(),
      archives: archives(),
      snapshots: snapshots(),
      tasks: tasks(),
      nowMs: nowMs(),
    }),
  );
  const pbsCoverage = createMemo(() => {
    const backups = pbsArtifacts();
    const namespaces = new Set<string>();
    let totalBytes = 0;
    let protectedCount = 0;
    let verifiedCount = 0;
    for (const backup of backups) {
      namespaces.add(pbsRepositoryLabel(backup));
      if (typeof backup.size === 'number' && backup.size > 0) totalBytes += backup.size;
      if (backup.protected) protectedCount += 1;
      if (backup.verified) verifiedCount += 1;
    }
    return {
      total: backups.length,
      totalBytes,
      protectedCount,
      verifiedCount,
      unverifiedCount: backups.length - verifiedCount,
      namespaceCount: namespaces.size,
    };
  });

  const pbsTimestampMs = (backup: PBSBackup): number | undefined => {
    const ms = Date.parse(backup.backupTime ?? '');
    return Number.isFinite(ms) ? ms : undefined;
  };
  const archiveTimestampMs = (arc: StorageBackup): number | undefined => {
    const ms = Date.parse(arc.time ?? '');
    return Number.isFinite(ms) ? ms : undefined;
  };
  const taskTimestampMs = (task: BackupTask): number | undefined => {
    const ms = Date.parse(task.startTime ?? '');
    return Number.isFinite(ms) ? ms : undefined;
  };
  const snapshotTimestampMs = (snap: GuestSnapshot): number | undefined => {
    const ms = Date.parse(snap.time ?? '');
    return Number.isFinite(ms) ? ms : undefined;
  };
  const classifyTaskSegment = (task: BackupTask): BackupActivitySegmentKind | null => {
    const variant = classifyTaskStatus(task.status).variant;
    if (variant === 'success') return 'ok';
    if (variant === 'danger') return 'failed';
    if (variant === 'warning') return 'running';
    return null;
  };

  const archiveTimeline = createMemo(() =>
    buildBackupActivityTimeline<StorageBackup>(
      chartRange(),
      archives(),
      archiveTimestampMs,
      () => 'archive',
      {
        getValue:
          archiveMetricMode() === 'volume' ? (arc) => (arc.size > 0 ? arc.size : 0) : undefined,
      },
    ),
  );
  const taskTimeline = createMemo(() =>
    buildBackupActivityTimeline<BackupTask>(
      chartRange(),
      tasks(),
      taskTimestampMs,
      classifyTaskSegment,
    ),
  );
  const snapshotTimeline = createMemo(() =>
    buildBackupActivityTimeline<GuestSnapshot>(
      chartRange(),
      snapshots(),
      snapshotTimestampMs,
      () => 'snapshot',
    ),
  );

  const ARCHIVE_SEGMENT_KINDS: readonly BackupActivitySegmentKind[] = ['archive'];
  const TASK_SEGMENT_KINDS: readonly BackupActivitySegmentKind[] = ['ok', 'failed', 'running'];
  const SNAPSHOT_SEGMENT_KINDS: readonly BackupActivitySegmentKind[] = ['snapshot'];
  const PBS_SEGMENT_KINDS: readonly BackupActivitySegmentKind[] = ['pbs'];
  const RECOVERABLE_SEGMENT_KINDS: readonly BackupActivitySegmentKind[] = [
    'pbs',
    'archive',
    'snapshot',
  ];

  const pbsTimeline = createMemo(() =>
    buildBackupActivityTimeline<PBSBackup>(
      chartRange(),
      pbsArtifacts(),
      pbsTimestampMs,
      () => 'pbs',
      {
        getValue:
          pbsMetricMode() === 'volume'
            ? (backup) => (backup.size > 0 ? backup.size : 0)
            : undefined,
      },
    ),
  );

  const recoverableTimeline = createMemo(() =>
    buildBackupActivityTimeline<RecoverableArtifact>(
      chartRange(),
      recoveryModel().recoverableArtifacts,
      (artifact) => artifact.createdMs,
      (artifact) =>
        artifact.sourceKind === 'pbs'
          ? 'pbs'
          : artifact.sourceKind === 'archive'
            ? 'archive'
            : 'snapshot',
      {
        getValue:
          recoverableMetricMode() === 'volume'
            ? (artifact) => (artifact.size && artifact.size > 0 ? artifact.size : 0)
            : undefined,
      },
    ),
  );

  const coveragePostureVariant = (
    posture: WorkloadCoverageRow['posture'],
  ): StatusIndicatorVariant => {
    if (posture === 'current') return 'success';
    if (posture === 'uncovered' || posture === 'failed' || posture === 'stale') return 'danger';
    return 'warning';
  };

  const snapshotMatchesSearch = (snap: GuestSnapshot, term: string): boolean => {
    if (!term) return true;
    return [snap.name, snap.node, snap.instance, snap.description, String(snap.vmid)]
      .filter(Boolean)
      .join(' ')
      .toLowerCase()
      .includes(term);
  };

  const pbsMatchesSearch = (backup: PBSBackup, term: string): boolean => {
    if (!term) return true;
    return [
      pbsWorkloadLabel(backup),
      pbsRepositoryLabel(backup),
      backup.instance,
      backup.datastore,
      backup.namespace,
      backup.backupType,
      backup.vmid,
      backup.comment,
      backup.owner,
      ...(backup.files ?? []),
    ]
      .filter(Boolean)
      .join(' ')
      .toLowerCase()
      .includes(term);
  };

  const filteredPBSBackups = createMemo(() => {
    const term = search().trim().toLowerCase();
    const filter = pbsFilter();
    const dateKey = selectedDateKey();
    const list = pbsArtifacts().filter((backup) => {
      if (filter === 'protected' && !backup.protected) return false;
      if (filter === 'verified' && !backup.verified) return false;
      if (filter === 'unverified' && backup.verified) return false;
      if (dateKey) {
        const ms = pbsTimestampMs(backup);
        if (ms === undefined || recoveryDateKeyFromTimestamp(ms) !== dateKey) return false;
      }
      return pbsMatchesSearch(backup, term);
    });
    const sortKey = pbsSortKey();
    const direction = pbsSortDirection();
    list.sort((a, b) => {
      switch (sortKey) {
        case 'workload':
          return cmpString(pbsWorkloadLabel(a), pbsWorkloadLabel(b), direction);
        case 'repository':
          return cmpString(pbsRepositoryLabel(a), pbsRepositoryLabel(b), direction);
        case 'created':
          return cmpNumber(pbsTimestampMs(a), pbsTimestampMs(b), direction);
        case 'size':
          return cmpNumber(a.size, b.size, direction);
        case 'protected':
          return cmpBool(a.protected, b.protected, direction);
        case 'verified':
          return cmpBool(a.verified, b.verified, direction);
      }
    });
    return list;
  });

  const filteredSnapshots = createMemo(() => {
    const term = search().trim().toLowerCase();
    const dateKey = selectedDateKey();
    return snapshots().filter((snap) => {
      if (dateKey) {
        const ms = snapshotTimestampMs(snap);
        if (ms === undefined || recoveryDateKeyFromTimestamp(ms) !== dateKey) return false;
      }
      return snapshotMatchesSearch(snap, term);
    });
  });

  const filteredSnapshotGuests = createMemo<SnapshotGuestRow[]>(() => {
    const term = search().trim().toLowerCase();
    const dateKey = selectedDateKey();
    const filter = snapshotFilter();
    const now = nowMs();
    const byGuest = new Map<string, SnapshotGuestRow>();
    for (const snap of snapshots()) {
      const matchesSnapshot = snapshotMatchesSearch(snap, term);
      if (!matchesSnapshot && !term) {
        // no term means everything matches; fall through.
      } else if (!matchesSnapshot && term) {
        // guest stays out of the matched set unless its identifiers match;
        // we re-evaluate that below if no snapshot inside matched.
      }
      const passesDate = (() => {
        if (!dateKey) return true;
        const ms = snapshotTimestampMs(snap);
        return ms !== undefined && recoveryDateKeyFromTimestamp(ms) === dateKey;
      })();
      if (!passesDate) continue;
      const key = guestKey(snap);
      let row = byGuest.get(key);
      if (!row) {
        row = {
          key,
          type: snap.type,
          vmid: snap.vmid,
          instance: snap.instance,
          node: snap.node,
          snapshots: [],
          count: 0,
          withRamCount: 0,
          newestMs: undefined,
          totalBytes: 0,
          isStale: true,
        };
        byGuest.set(key, row);
      }
      row.snapshots.push(snap);
      row.count += 1;
      if (snap.vmstate) row.withRamCount += 1;
      if (typeof snap.sizeBytes === 'number' && snap.sizeBytes > 0)
        row.totalBytes += snap.sizeBytes;
      const ms = snapshotTimestampMs(snap);
      if (ms !== undefined && (row.newestMs === undefined || ms > row.newestMs)) {
        row.newestMs = ms;
      }
    }
    // Search-on-guest-identity: if the search term matches the guest's
    // identifying fields (node/vmid/type), every snapshot under it is
    // considered relevant even if individual snapshot text did not match.
    const searchedByGuestIdentity = (row: SnapshotGuestRow): boolean => {
      if (!term) return true;
      return [`${row.type} ${row.vmid}`, row.node, row.instance, String(row.vmid)]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(term);
    };
    const rows: SnapshotGuestRow[] = [];
    for (const row of byGuest.values()) {
      const matchesByIdentity = searchedByGuestIdentity(row);
      const snapshotsMatching = term
        ? row.snapshots.filter((snap) => snapshotMatchesSearch(snap, term))
        : row.snapshots;
      if (!matchesByIdentity && snapshotsMatching.length === 0) continue;
      const visibleSnapshots = matchesByIdentity ? row.snapshots : snapshotsMatching;
      visibleSnapshots.sort((a, b) => {
        const av = Date.parse(a.time);
        const bv = Date.parse(b.time);
        return (Number.isFinite(bv) ? bv : 0) - (Number.isFinite(av) ? av : 0);
      });
      const enriched: SnapshotGuestRow = {
        ...row,
        snapshots: visibleSnapshots,
        count: visibleSnapshots.length,
        withRamCount: visibleSnapshots.reduce((sum, s) => sum + (s.vmstate ? 1 : 0), 0),
        totalBytes: visibleSnapshots.reduce(
          (sum, s) => sum + (typeof s.sizeBytes === 'number' && s.sizeBytes > 0 ? s.sizeBytes : 0),
          0,
        ),
        newestMs: visibleSnapshots.reduce<number | undefined>((newest, s) => {
          const ms = snapshotTimestampMs(s);
          if (ms === undefined) return newest;
          if (newest === undefined) return ms;
          return ms > newest ? ms : newest;
        }, undefined),
        isStale: ((): boolean => {
          const newest = visibleSnapshots[0] ? snapshotTimestampMs(visibleSnapshots[0]) : undefined;
          if (newest === undefined) return true;
          return now - newest > 30 * 24 * 60 * 60 * 1000;
        })(),
      };
      if (filter === 'recent' && enriched.isStale) continue;
      if (filter === 'stale' && !enriched.isStale) continue;
      if (filter === 'with-ram' && enriched.withRamCount === 0) continue;
      rows.push(enriched);
    }
    const sortKey = snapshotSortKey();
    const direction = snapshotSortDirection();
    rows.sort((a, b) => {
      switch (sortKey) {
        case 'guest':
          return cmpString(guestLabel(a.type, a.vmid), guestLabel(b.type, b.vmid), direction);
        case 'node':
          return cmpString(a.node, b.node, direction);
        case 'latest':
          return cmpNumber(a.newestMs, b.newestMs, direction);
        case 'count':
          return cmpNumber(a.count, b.count, direction);
        case 'size':
          return cmpNumber(a.totalBytes, b.totalBytes, direction);
      }
    });
    return rows;
  });

  const filteredArchives = createMemo(() => {
    const term = search().trim().toLowerCase();
    const filter = archiveFilter();
    const dateKey = selectedDateKey();
    const list = archives().filter((arc) => {
      if (filter === 'protected' && !arc.protected) return false;
      if (filter === 'verified' && !arc.verified) return false;
      if (filter === 'unverified' && arc.verified) return false;
      if (dateKey) {
        const ms = archiveTimestampMs(arc);
        if (ms === undefined || recoveryDateKeyFromTimestamp(ms) !== dateKey) return false;
      }
      if (!term) return true;
      return [arc.storage, arc.node, arc.instance, arc.volid, arc.notes, String(arc.vmid)]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(term);
    });
    const sortKey = archiveSortKey();
    const direction = archiveSortDirection();
    list.sort((a, b) => {
      switch (sortKey) {
        case 'volume':
          return cmpString(a.volid, b.volid, direction);
        case 'guest':
          return cmpString(guestLabel(a.type, a.vmid), guestLabel(b.type, b.vmid), direction);
        case 'storage':
          return cmpString(a.storage, b.storage, direction);
        case 'node':
          return cmpString(a.node, b.node, direction);
        case 'format':
          return cmpString(a.format, b.format, direction);
        case 'created':
          return cmpNumber(archiveTimestampMs(a), archiveTimestampMs(b), direction);
        case 'size':
          return cmpNumber(a.size, b.size, direction);
        case 'protected':
          return cmpBool(a.protected, b.protected, direction);
        case 'verified':
          return cmpBool(a.verified, b.verified, direction);
      }
    });
    return list;
  });

  const filteredTasks = createMemo(() => {
    const term = search().trim().toLowerCase();
    const filter = taskFilter();
    const dateKey = selectedDateKey();
    const list = tasks().filter((task) => {
      const classify = classifyTaskStatus(task.status);
      if (filter === 'ok' && classify.variant !== 'success') return false;
      if (filter === 'failed' && classify.variant !== 'danger') return false;
      if (filter === 'running' && classify.variant !== 'warning') return false;
      if (dateKey) {
        const ms = taskTimestampMs(task);
        if (ms === undefined || recoveryDateKeyFromTimestamp(ms) !== dateKey) return false;
      }
      if (!term) return true;
      return [task.node, task.instance, task.status, task.error, String(task.vmid)]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(term);
    });
    const sortKey = taskSortKey();
    const direction = taskSortDirection();
    list.sort((a, b) => {
      switch (sortKey) {
        case 'status':
          return cmpString(
            classifyTaskStatus(a.status).label,
            classifyTaskStatus(b.status).label,
            direction,
          );
        case 'guest':
          return cmpString(guestLabel(a.type, a.vmid), guestLabel(b.type, b.vmid), direction);
        case 'node':
          return cmpString(a.node, b.node, direction);
        case 'started':
          return cmpNumber(taskTimestampMs(a), taskTimestampMs(b), direction);
        case 'duration':
          return cmpNumber(taskDurationSeconds(a), taskDurationSeconds(b), direction);
        case 'size':
          return cmpNumber(a.size, b.size, direction);
      }
    });
    return list;
  });

  const filteredCoverageRows = createMemo(() => {
    const term = search().trim().toLowerCase();
    const filter = coverageFilter();
    const dateKey = selectedDateKey();
    const list = recoveryModel().coverageRows.filter((row) => {
      if (filter === 'attention' && !isCoverageAttention(row.posture)) return false;
      if (filter === 'current' && row.posture !== 'current') return false;
      if (filter === 'uncovered' && row.posture !== 'uncovered') return false;
      if (
        dateKey &&
        !row.artifacts.some(
          (artifact) =>
            artifact.createdMs !== undefined &&
            recoveryDateKeyFromTimestamp(artifact.createdMs) === dateKey,
        )
      ) {
        return false;
      }
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
          return cmpNumber(a.latestRecovery?.createdMs, b.latestRecovery?.createdMs, direction);
        case 'pbs':
          return cmpNumber(a.latestPBS?.createdMs, b.latestPBS?.createdMs, direction);
        case 'archive':
          return cmpNumber(a.latestArchive?.createdMs, b.latestArchive?.createdMs, direction);
        case 'snapshot':
          return cmpNumber(a.latestSnapshot?.createdMs, b.latestSnapshot?.createdMs, direction);
        case 'task':
          return cmpNumber(a.latestTask?.startedMs, b.latestTask?.startedMs, direction);
      }
    });
    return list;
  });

  const filteredRecoverableArtifacts = createMemo(() => {
    const term = search().trim().toLowerCase();
    const filter = recoverableFilter();
    const dateKey = selectedDateKey();
    const list = recoveryModel().recoverableArtifacts.filter((artifact) => {
      if (
        (filter === 'pbs' || filter === 'archive' || filter === 'snapshot') &&
        artifact.sourceKind !== filter
      ) {
        return false;
      }
      if (filter === 'verified' && artifact.verified !== true) return false;
      if (filter === 'unverified' && artifact.verified !== false) return false;
      if (
        dateKey &&
        (artifact.createdMs === undefined ||
          recoveryDateKeyFromTimestamp(artifact.createdMs) !== dateKey)
      ) {
        return false;
      }
      return recoverableArtifactMatchesSearch(artifact, term);
    });
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

  const showSnapshotSizeColumn = createMemo(() =>
    snapshots().some((snap) => typeof snap.sizeBytes === 'number' && snap.sizeBytes > 0),
  );
  const showSnapshotRAMColumn = createMemo(() => snapshots().some((snap) => snap.vmstate));
  const snapshotColumnCount = createMemo(
    () => 4 + (showSnapshotSizeColumn() ? 1 : 0) + (showSnapshotRAMColumn() ? 1 : 0),
  );
  const showArchivePBSColumns = createMemo(() =>
    archives().some((arc) => arc.isPBS || arc.protected || arc.verified || !!arc.verification),
  );
  const showTaskSizeColumn = createMemo(() =>
    tasks().some((task) => typeof task.size === 'number' && task.size > 0),
  );
  const showTaskErrorColumn = createMemo(() => tasks().some((task) => !!task.error?.trim()));

  const totalForTab = createMemo<number>(() => {
    switch (tab()) {
      case 'coverage':
        return recoveryModel().coverageRows.length;
      case 'recoverable':
        return recoveryModel().recoverableArtifacts.length;
      case 'sources':
        return sourceDetailTotal(sourceDetailTab());
      case 'tasks':
        return tasks().length;
      default:
        return 0;
    }
  });

  const visibleForTab = createMemo<number>(() => {
    switch (tab()) {
      case 'coverage':
        return filteredCoverageRows().length;
      case 'recoverable':
        return filteredRecoverableArtifacts().length;
      case 'sources':
        if (sourceDetailTab() === 'pbs') return filteredPBSBackups().length;
        if (sourceDetailTab() === 'snapshots') return filteredSnapshots().length;
        return filteredArchives().length;
      case 'tasks':
        return filteredTasks().length;
      default:
        return 0;
    }
  });

  // Used to scale the inline duration bar on Recent tasks. A typical task
  // sits at ~50% of the bar, outliers extend toward the right edge. The
  // baseline is recomputed against the *filtered* set so the bar stays
  // useful when the user narrows the view to a single guest.
  const taskDurationBaselineSeconds = createMemo(() =>
    computeMedianTaskDurationSeconds(filteredTasks()),
  );

  // The largest PBS artifact in the filtered set anchors the size bar.
  const pbsSizeMaxBytes = createMemo(() => {
    let max = 0;
    for (const backup of filteredPBSBackups()) {
      if (backup.size > max) max = backup.size;
    }
    return max;
  });

  // The largest archive in the filtered set anchors the size bar.
  const archiveSizeMaxBytes = createMemo(() => {
    let max = 0;
    for (const arc of filteredArchives()) {
      if (arc.size > max) max = arc.size;
    }
    return max;
  });

  const recoverableSizeMaxBytes = createMemo(() => {
    let max = 0;
    for (const artifact of filteredRecoverableArtifacts()) {
      if (artifact.size && artifact.size > max) max = artifact.size;
    }
    return max;
  });

  const activeTabNoun = createMemo(() => {
    switch (tab()) {
      case 'coverage':
        return 'workloads';
      case 'recoverable':
        return 'restore points';
      case 'sources':
        if (sourceDetailTab() === 'pbs') return 'PBS artifacts';
        if (sourceDetailTab() === 'snapshots') return 'snapshots';
        return 'backup files';
      case 'tasks':
        return 'tasks';
    }
  });

  const visibleSnapshotGuestCount = createMemo(() => filteredSnapshotGuests().length);

  return (
    <Show
      when={!backups.error}
      fallback={
        <Card padding="lg">
          <EmptyState
            icon={props.emptyIcon}
            title="Could not load Proxmox backup inventory"
            description={(backups.error as Error | undefined)?.message ?? 'Refresh to retry.'}
            actions={
              <button
                type="button"
                onClick={() => void refetch()}
                class="inline-flex min-h-10 items-center rounded-md border border-border px-3 py-2 text-sm font-medium hover:bg-surface-hover"
              >
                Refresh
              </button>
            }
          />
        </Card>
      }
    >
      <Show
        when={backups() !== undefined}
        fallback={
          <Card padding="lg">
            <EmptyState
              icon={props.emptyIcon}
              title="Loading Proxmox backup inventory"
              description="Reading PBS artifacts, PVE snapshots, archives and recent backup tasks."
            />
          </Card>
        }
      >
        <div class="space-y-3">
          <div class="flex flex-wrap items-center gap-1 rounded-md border border-border bg-surface p-1">
            <For each={activeTabs()}>
              {(spec) => (
                <button
                  type="button"
                  onClick={() => setTab(spec.id)}
                  class={`inline-flex min-h-9 items-center gap-1.5 rounded-sm px-3 text-xs font-medium transition-colors ${
                    tab() === spec.id
                      ? 'bg-surface-hover text-base-content'
                      : 'text-muted hover:text-base-content'
                  }`}
                  aria-pressed={tab() === spec.id}
                >
                  {spec.icon()}
                  <span>{spec.label}</span>
                  <span class="text-[10px] text-muted tabular-nums">
                    {spec.id === 'coverage'
                      ? recoveryModel().coverageRows.length
                      : spec.id === 'recoverable'
                        ? recoveryModel().recoverableArtifacts.length
                        : spec.id === 'sources'
                          ? totalSourceArtifacts()
                          : tasks().length}
                  </span>
                </button>
              )}
            </For>
          </div>

          <Show when={tab() === 'sources'}>
            <div class="flex flex-wrap items-center gap-1 rounded-md border border-border bg-surface p-1">
              <For each={activeSourceDetailTabs()}>
                {(spec) => (
                  <button
                    type="button"
                    onClick={() => setSourceDetailTab(spec.id)}
                    class={`inline-flex min-h-8 items-center gap-1.5 rounded-sm px-2.5 text-xs font-medium transition-colors ${
                      sourceDetailTab() === spec.id
                        ? 'bg-surface-hover text-base-content'
                        : 'text-muted hover:text-base-content'
                    }`}
                    aria-pressed={sourceDetailTab() === spec.id}
                  >
                    {spec.icon()}
                    <span>{spec.label}</span>
                    <span class="text-[10px] text-muted tabular-nums">
                      {sourceDetailTotal(spec.id)}
                    </span>
                  </button>
                )}
              </For>
            </div>
          </Show>

          <Show when={tab() === 'coverage'}>
            <ProxmoxBackupsCoverageStrip
              title="Workload restore posture"
              tail={
                <span>
                  {recoveryModel().coverageSummary.totalWorkloads} workloads ·{' '}
                  {recoveryModel().coverageSummary.recoverableArtifacts} recoverable artifacts
                  <Show when={recoveryModel().coverageSummary.withPBS > 0}>
                    {' · '}
                    {recoveryModel().coverageSummary.withPBS} with PBS
                  </Show>
                </span>
              }
              segments={[
                {
                  key: 'current',
                  value: recoveryModel().coverageSummary.current,
                  label: 'current',
                  toneClass: 'bg-emerald-500',
                },
                {
                  key: 'attention',
                  value: recoveryModel().coverageSummary.attention,
                  label: 'attention',
                  toneClass: 'bg-amber-500',
                  muted: recoveryModel().coverageSummary.attention === 0,
                },
                {
                  key: 'uncovered',
                  value: recoveryModel().coverageSummary.uncovered,
                  label: 'uncovered',
                  toneClass: 'bg-red-500',
                  muted: recoveryModel().coverageSummary.uncovered === 0,
                },
              ]}
            />
          </Show>

          <Show when={tab() === 'recoverable' && recoveryModel().recoverableArtifacts.length > 0}>
            <BackupActivityChart
              title={
                recoverableMetricMode() === 'volume'
                  ? 'Recoverable volume per day'
                  : 'Recoverable artifacts per day'
              }
              noun="artifact"
              segmentKinds={RECOVERABLE_SEGMENT_KINDS}
              range={chartRange}
              onRangeChange={setChartRange}
              timeline={recoverableTimeline}
              selectedDateKey={selectedDateKey}
              onToggleDay={toggleDay}
              metricToggle={{ mode: recoverableMetricMode, onChange: setRecoverableMetricMode }}
            />
            <ProxmoxBackupsCoverageStrip
              title="Restore inventory"
              tail={
                <span>
                  {recoveryModel().coverageSummary.recoverableArtifacts} artifacts ·{' '}
                  {formatBytes(recoveryModel().coverageSummary.totalBytes)} logical
                </span>
              }
              segments={[
                {
                  key: 'pbs',
                  value: recoveryModel().recoverableArtifacts.filter((a) => a.sourceKind === 'pbs')
                    .length,
                  label: 'PBS',
                  toneClass: 'bg-cyan-500',
                },
                {
                  key: 'archives',
                  value: recoveryModel().recoverableArtifacts.filter(
                    (a) => a.sourceKind === 'archive',
                  ).length,
                  label: 'archives',
                  toneClass: 'bg-blue-500',
                  muted:
                    recoveryModel().recoverableArtifacts.filter((a) => a.sourceKind === 'archive')
                      .length === 0,
                },
                {
                  key: 'snapshots',
                  value: recoveryModel().recoverableArtifacts.filter(
                    (a) => a.sourceKind === 'snapshot',
                  ).length,
                  label: 'snapshots',
                  toneClass: 'bg-violet-500',
                  muted:
                    recoveryModel().recoverableArtifacts.filter((a) => a.sourceKind === 'snapshot')
                      .length === 0,
                },
              ]}
            />
          </Show>

          <Show
            when={tab() === 'sources' && sourceDetailTab() === 'pbs' && pbsArtifacts().length > 0}
          >
            <BackupActivityChart
              title={
                pbsMetricMode() === 'volume' ? 'PBS backup volume per day' : 'PBS backups per day'
              }
              noun="artifact"
              segmentKinds={PBS_SEGMENT_KINDS}
              range={chartRange}
              onRangeChange={setChartRange}
              timeline={pbsTimeline}
              selectedDateKey={selectedDateKey}
              onToggleDay={toggleDay}
              metricToggle={{ mode: pbsMetricMode, onChange: setPBSMetricMode }}
            />
            <ProxmoxBackupsCoverageStrip
              title="PBS verification"
              tail={
                <span>
                  {pbsCoverage().total} artifacts · {formatBytes(pbsCoverage().totalBytes)} logical
                  <Show when={pbsCoverage().namespaceCount > 0}>
                    {' · '}
                    {pbsCoverage().namespaceCount} namespaces
                  </Show>
                  <Show when={pbsCoverage().protectedCount > 0}>
                    {' · '}
                    {pbsCoverage().protectedCount} protected
                  </Show>
                </span>
              }
              segments={[
                {
                  key: 'verified',
                  value: pbsCoverage().verifiedCount,
                  label: 'verified',
                  toneClass: 'bg-emerald-500',
                },
                {
                  key: 'unverified',
                  value: pbsCoverage().unverifiedCount,
                  label: 'unverified',
                  toneClass: 'bg-amber-500',
                  muted: pbsCoverage().unverifiedCount === 0,
                },
              ]}
            />
          </Show>

          <Show
            when={
              tab() === 'sources' && sourceDetailTab() === 'snapshots' && snapshots().length > 0
            }
          >
            <BackupActivityChart
              title="Snapshots per day"
              noun="snapshot"
              segmentKinds={SNAPSHOT_SEGMENT_KINDS}
              range={chartRange}
              onRangeChange={setChartRange}
              timeline={snapshotTimeline}
              selectedDateKey={selectedDateKey}
              onToggleDay={toggleDay}
            />
            <ProxmoxBackupsCoverageStrip
              title="Snapshot coverage"
              tail={
                <span>
                  {snapshotCoverage().totalGuests} guests · {snapshotCoverage().totalSnapshots}{' '}
                  snapshots
                  <Show when={snapshotCoverage().withRamGuests > 0}>
                    {' · '}
                    {snapshotCoverage().withRamGuests} with RAM
                  </Show>
                </span>
              }
              segments={[
                {
                  key: 'recent',
                  value:
                    snapshotCoverage().totalGuests -
                    snapshotCoverage().staleGuests -
                    snapshotCoverage().ancientGuests,
                  label: 'recent (≤30d)',
                  toneClass: getBackupAgeBucketPresentation('recent').swatchClass,
                },
                {
                  key: 'stale',
                  value: snapshotCoverage().staleGuests,
                  label: 'stale (30–90d)',
                  toneClass: getBackupAgeBucketPresentation('stale').swatchClass,
                  muted: snapshotCoverage().staleGuests === 0,
                },
                {
                  key: 'ancient',
                  value: snapshotCoverage().ancientGuests,
                  label: 'ancient (>90d)',
                  toneClass: getBackupAgeBucketPresentation('ancient').swatchClass,
                  muted: snapshotCoverage().ancientGuests === 0,
                },
              ]}
            />
          </Show>
          <Show when={tab() === 'sources' && sourceDetailTab() === 'archives'}>
            <BackupActivityChart
              title={
                archiveMetricMode() === 'volume' ? 'Backup volume per day' : 'Backup files per day'
              }
              noun="archive"
              segmentKinds={ARCHIVE_SEGMENT_KINDS}
              range={chartRange}
              onRangeChange={setChartRange}
              timeline={archiveTimeline}
              selectedDateKey={selectedDateKey}
              onToggleDay={toggleDay}
              metricToggle={{ mode: archiveMetricMode, onChange: setArchiveMetricMode }}
            />
            <Show when={archiveCoverage().totalGuests > 0}>
              <ProxmoxBackupsCoverageStrip
                title="Backup coverage"
                tail={
                  <span>
                    {archiveCoverage().totalGuests} guests with archives ·{' '}
                    {formatBytes(archiveCoverage().totalBytes)} stored
                  </span>
                }
                segments={[
                  {
                    key: 'current',
                    value: archiveCoverage().currentGuests,
                    label: 'current (≤7d)',
                    toneClass: 'bg-emerald-500',
                  },
                  {
                    key: 'stale',
                    value: archiveCoverage().staleGuests,
                    label: 'stale (7–30d)',
                    toneClass: 'bg-amber-500',
                    muted: archiveCoverage().staleGuests === 0,
                  },
                  {
                    key: 'uncovered',
                    value: archiveCoverage().uncoveredGuests,
                    label: 'uncovered (>30d)',
                    toneClass: 'bg-red-500',
                    muted: archiveCoverage().uncoveredGuests === 0,
                  },
                ]}
              />
            </Show>
          </Show>
          <Show when={tab() === 'tasks'}>
            <BackupActivityChart
              title="Backup tasks per day"
              noun="task"
              segmentKinds={TASK_SEGMENT_KINDS}
              range={chartRange}
              onRangeChange={setChartRange}
              timeline={taskTimeline}
              selectedDateKey={selectedDateKey}
              onToggleDay={toggleDay}
            />
            <Show when={taskOutcome().total > 0}>
              <ProxmoxBackupsCoverageStrip
                title="Task outcomes"
                tail={
                  <Show when={taskDurationBaselineSeconds() > 0}>
                    <span>
                      median duration{' '}
                      <span class="font-mono tabular-nums text-base-content">
                        {formatDurationFromSeconds(taskDurationBaselineSeconds())}
                      </span>
                    </span>
                  </Show>
                }
                segments={[
                  {
                    key: 'ok',
                    value: taskOutcome().ok,
                    label: 'OK',
                    toneClass: 'bg-emerald-500',
                  },
                  {
                    key: 'failed',
                    value: taskOutcome().failed,
                    label: 'failed',
                    toneClass: 'bg-red-500',
                    muted: taskOutcome().failed === 0,
                  },
                  {
                    key: 'running',
                    value: taskOutcome().running,
                    label: 'running',
                    toneClass: 'bg-amber-500',
                    muted: taskOutcome().running === 0,
                  },
                ]}
              />
            </Show>
          </Show>

          <div class="flex flex-wrap items-center gap-2">
            <div class="min-w-[200px] flex-1 sm:max-w-xs">
              <SearchInput
                value={search}
                onChange={setSearch}
                placeholder={
                  tab() === 'coverage'
                    ? 'Search workload coverage or restore evidence'
                    : tab() === 'recoverable'
                      ? 'Search restore points, workloads, sources'
                      : tab() === 'sources'
                        ? sourceDetailTab() === 'pbs'
                          ? 'Search PBS artifacts, namespaces, guests'
                          : sourceDetailTab() === 'snapshots'
                            ? 'Search snapshots, guests, nodes'
                            : 'Search backup files, storages, nodes'
                        : 'Search tasks, nodes, errors'
                }
              />
            </div>
            <Show when={tab() === 'coverage'}>
              <FilterButtonGroup
                variant="compact"
                options={COVERAGE_FILTERS}
                value={coverageFilter()}
                onChange={setCoverageFilter}
              />
            </Show>
            <Show when={tab() === 'recoverable'}>
              <FilterButtonGroup
                variant="compact"
                options={RECOVERABLE_FILTERS}
                value={recoverableFilter()}
                onChange={setRecoverableFilter}
              />
            </Show>
            <Show when={tab() === 'sources' && sourceDetailTab() === 'pbs'}>
              <FilterButtonGroup
                variant="compact"
                options={PBS_STATUS_FILTERS}
                value={pbsFilter()}
                onChange={setPBSFilter}
              />
            </Show>
            <Show when={tab() === 'sources' && sourceDetailTab() === 'snapshots'}>
              <FilterButtonGroup
                variant="compact"
                options={SNAPSHOT_FILTERS}
                value={snapshotFilter()}
                onChange={setSnapshotFilter}
              />
            </Show>
            <Show
              when={
                tab() === 'sources' && sourceDetailTab() === 'archives' && showArchivePBSColumns()
              }
            >
              <FilterButtonGroup
                variant="compact"
                options={ARCHIVE_STATUS_FILTERS}
                value={archiveFilter()}
                onChange={setArchiveFilter}
              />
            </Show>
            <Show when={tab() === 'tasks'}>
              <FilterButtonGroup
                variant="compact"
                options={TASK_STATUS_FILTERS}
                value={taskFilter()}
                onChange={setTaskFilter}
              />
            </Show>
            <Show when={selectedDateKey() !== null}>
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
            <span class="ml-auto whitespace-nowrap text-xs font-medium text-muted">
              <Show
                when={tab() === 'sources' && sourceDetailTab() === 'snapshots'}
                fallback={
                  <Show
                    when={visibleForTab() !== totalForTab()}
                    fallback={
                      <>
                        {totalForTab()} {activeTabNoun()}
                      </>
                    }
                  >
                    {visibleForTab()} of {totalForTab()} {activeTabNoun()}
                  </Show>
                }
              >
                <Show
                  when={visibleSnapshotGuestCount() !== snapshotCoverage().totalGuests}
                  fallback={<>{snapshotCoverage().totalGuests} guests</>}
                >
                  {visibleSnapshotGuestCount()} of {snapshotCoverage().totalGuests} guests
                </Show>
              </Show>
            </span>
          </div>

          <Show when={tab() === 'coverage'}>
            <Show
              when={filteredCoverageRows().length > 0}
              fallback={
                <Card padding="lg">
                  <EmptyState
                    icon={props.emptyIcon}
                    title={
                      recoveryModel().coverageRows.length === 0
                        ? tabSpecFor('coverage').emptyTitle
                        : 'No workload coverage rows match current filters'
                    }
                    description={
                      recoveryModel().coverageRows.length === 0
                        ? tabSpecFor('coverage').emptyDescription
                        : 'Adjust the search, posture filter, or selected day to see more workloads.'
                    }
                  />
                </Card>
              }
            >
              <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
                <Table class="min-w-[1200px] text-xs">
                  <TableHeader>
                    <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                      <SortableHead
                        label="Workload"
                        sortKey="workload"
                        currentSort={coverageSortKey}
                        direction={coverageSortDirection}
                        onSort={handleCoverageSort}
                        align="left"
                        headClass={getPlatformTableHeadClassForKind('name')}
                      />
                      <SortableHead
                        label="Posture"
                        sortKey="posture"
                        currentSort={coverageSortKey}
                        direction={coverageSortDirection}
                        onSort={handleCoverageSort}
                        align="left"
                        headClass={getPlatformTableHeadClassForKind('text')}
                      />
                      <SortableHead
                        label="Latest restore"
                        sortKey="latest"
                        currentSort={coverageSortKey}
                        direction={coverageSortDirection}
                        onSort={handleCoverageSort}
                        align="right"
                        headClass={getPlatformTableHeadClassForKind('numeric-value')}
                      />
                      <SortableHead
                        label="PBS"
                        sortKey="pbs"
                        currentSort={coverageSortKey}
                        direction={coverageSortDirection}
                        onSort={handleCoverageSort}
                        align="left"
                        headClass={getPlatformTableHeadClassForKind('text')}
                      />
                      <SortableHead
                        label="Archive"
                        sortKey="archive"
                        currentSort={coverageSortKey}
                        direction={coverageSortDirection}
                        onSort={handleCoverageSort}
                        align="left"
                        headClass={getPlatformTableHeadClassForKind('text')}
                      />
                      <SortableHead
                        label="Snapshot"
                        sortKey="snapshot"
                        currentSort={coverageSortKey}
                        direction={coverageSortDirection}
                        onSort={handleCoverageSort}
                        align="left"
                        headClass={getPlatformTableHeadClassForKind('text')}
                      />
                      <SortableHead
                        label="Latest task"
                        sortKey="task"
                        currentSort={coverageSortKey}
                        direction={coverageSortDirection}
                        onSort={handleCoverageSort}
                        align="left"
                        headClass={getPlatformTableHeadClassForKind('text')}
                      />
                    </TableRow>
                  </TableHeader>
                  <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                    <For each={filteredCoverageRows()}>
                      {(row) => {
                        const isExpanded = () => expandedCoverageRows().has(row.key);
                        const evidence = () =>
                          [...row.artifacts]
                            .sort((left, right) => (right.createdMs ?? 0) - (left.createdMs ?? 0))
                            .slice(0, 8);
                        return (
                          <>
                            <TableRow class="hover:bg-surface-hover">
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('name')} text-base-content`}
                              >
                                <div class="flex min-w-0 items-center gap-2">
                                  <button
                                    type="button"
                                    class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-sm text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-1 focus-visible:ring-offset-surface"
                                    onClick={() => toggleCoverageExpansion(row.key)}
                                    aria-label={`${isExpanded() ? 'Hide' : 'Show'} restore evidence for ${row.workload.label}`}
                                    aria-expanded={isExpanded()}
                                  >
                                    <ChevronRightIcon
                                      class={`h-3.5 w-3.5 transition-transform ${
                                        isExpanded() ? 'rotate-90' : ''
                                      }`}
                                      aria-hidden="true"
                                    />
                                  </button>
                                  <div class="min-w-0">
                                    <div class="font-semibold">{row.workload.label}</div>
                                    <div class="truncate font-mono text-[10px] uppercase text-muted">
                                      {row.workload.typeLabel} {row.workload.vmid}
                                      <Show when={row.workload.node}>
                                        {' · '}
                                        {row.workload.node}
                                      </Show>
                                    </div>
                                  </div>
                                </div>
                              </TableCell>
                              <TableCell class={getPlatformTableCellClassForKind('text')}>
                                <div class="flex items-center gap-2">
                                  <StatusDot
                                    size="sm"
                                    variant={coveragePostureVariant(row.posture)}
                                    title={getWorkloadRecoveryPostureLabel(row.posture)}
                                    ariaHidden
                                  />
                                  <span class="text-[11px] font-medium text-base-content">
                                    {getWorkloadRecoveryPostureLabel(row.posture)}
                                  </span>
                                </div>
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                              >
                                <Show
                                  when={row.latestRecovery}
                                  fallback={<span class="text-muted">No restore point</span>}
                                >
                                  {(artifact) => (
                                    <div class="min-w-0 text-right">
                                      <div>
                                        {formatRelativeTime(artifact().createdAt, {
                                          compact: true,
                                        })}
                                      </div>
                                      <div class="truncate text-[10px] text-muted">
                                        {artifact().sourceLabel}
                                      </div>
                                    </div>
                                  )}
                                </Show>
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                              >
                                <RecoverySourceSummary
                                  artifact={row.latestPBS}
                                  count={row.pbsCount}
                                  emptyLabel="No PBS"
                                />
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                              >
                                <RecoverySourceSummary
                                  artifact={row.latestArchive}
                                  count={row.archiveCount}
                                  emptyLabel="No archive"
                                />
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                              >
                                <RecoverySourceSummary
                                  artifact={row.latestSnapshot}
                                  count={row.snapshotCount}
                                  emptyLabel="No snapshot"
                                />
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                              >
                                <Show
                                  when={row.latestTask}
                                  fallback={<span class="text-muted">No recent task</span>}
                                >
                                  {(task) => (
                                    <div class="min-w-0">
                                      <div class="flex items-center gap-2">
                                        <StatusDot
                                          size="sm"
                                          variant={
                                            task().label === 'Failed'
                                              ? 'danger'
                                              : task().label === 'OK'
                                                ? 'success'
                                                : 'warning'
                                          }
                                          title={task().label}
                                          ariaHidden
                                        />
                                        <span>{task().label}</span>
                                      </div>
                                      <div class="truncate text-[10px] text-muted">
                                        {formatRelativeTime(task().startedAt, { compact: true })}
                                      </div>
                                    </div>
                                  )}
                                </Show>
                              </TableCell>
                            </TableRow>
                            <Show when={isExpanded()}>
                              <TableRow class="bg-surface-alt/40">
                                <TableCell class="px-3 py-2" colspan={7}>
                                  <Show
                                    when={evidence().length > 0}
                                    fallback={
                                      <div class="text-xs text-muted">
                                        No restore evidence has been discovered for this workload.
                                      </div>
                                    }
                                  >
                                    <div class="overflow-hidden">
                                      <div class="mb-1 flex items-center justify-between gap-2 text-[11px]">
                                        <span class="font-medium text-base-content">
                                          Restore evidence
                                        </span>
                                        <Show when={row.artifacts.length > evidence().length}>
                                          <span class="text-muted">
                                            Showing {evidence().length} of {row.artifacts.length}
                                          </span>
                                        </Show>
                                      </div>
                                      <table class="w-full text-[11px]">
                                        <thead>
                                          <tr class="bg-surface-alt text-muted">
                                            <th class="px-2 py-0.5 text-left font-medium">
                                              Source
                                            </th>
                                            <th class="px-2 py-0.5 text-left font-medium">
                                              Location
                                            </th>
                                            <th class="px-2 py-0.5 text-right font-medium">
                                              Created
                                            </th>
                                            <th class="px-2 py-0.5 text-right font-medium">Size</th>
                                            <th class="px-2 py-0.5 text-left font-medium">State</th>
                                            <th class="px-2 py-0.5 text-left font-medium">
                                              Details
                                            </th>
                                          </tr>
                                        </thead>
                                        <tbody class="divide-y divide-border-subtle">
                                          <For each={evidence()}>
                                            {(artifact) => (
                                              <tr class="hover:bg-surface-hover">
                                                <td class="px-2 py-1">
                                                  <ArtifactSourceBadge artifact={artifact} />
                                                </td>
                                                <td class="px-2 py-1 text-base-content">
                                                  <span
                                                    class="inline-block max-w-[18rem] truncate"
                                                    title={artifact.location}
                                                  >
                                                    {artifact.location}
                                                  </span>
                                                </td>
                                                <td class="px-2 py-1 text-right text-base-content">
                                                  {formatRelativeTime(artifact.createdAt, {
                                                    compact: true,
                                                  })}
                                                </td>
                                                <td class="px-2 py-1 text-right tabular-nums text-base-content">
                                                  <Show
                                                    when={artifact.size && artifact.size > 0}
                                                    fallback={
                                                      <span class="text-muted">No size</span>
                                                    }
                                                  >
                                                    {formatBytes(artifact.size ?? 0)}
                                                  </Show>
                                                </td>
                                                <td class="px-2 py-1">
                                                  <ArtifactStateBadge
                                                    artifact={artifact}
                                                    label={artifactStateLabel(artifact)}
                                                  />
                                                </td>
                                                <td class="px-2 py-1 text-base-content">
                                                  <span
                                                    class="inline-block max-w-[24rem] truncate"
                                                    title={artifact.detail}
                                                  >
                                                    {artifact.detail || '—'}
                                                  </span>
                                                </td>
                                              </tr>
                                            )}
                                          </For>
                                        </tbody>
                                      </table>
                                    </div>
                                  </Show>
                                </TableCell>
                              </TableRow>
                            </Show>
                          </>
                        );
                      }}
                    </For>
                  </TableBody>
                </Table>
              </TableCard>
            </Show>
          </Show>

          <Show when={tab() === 'recoverable'}>
            <ProxmoxRecoverableTable
              artifacts={filteredRecoverableArtifacts()}
              hasAnyArtifacts={recoveryModel().recoverableArtifacts.length > 0}
              emptyIcon={props.emptyIcon}
              emptyTitle={tabSpecFor('recoverable').emptyTitle}
              emptyDescription={tabSpecFor('recoverable').emptyDescription}
              sortKey={recoverableSortKey}
              sortDirection={recoverableSortDirection}
              onSort={handleRecoverableSort}
              sizeMaxBytes={recoverableSizeMaxBytes()}
            />
          </Show>

          <Show when={tab() === 'sources' && sourceDetailTab() === 'pbs'}>
            <ProxmoxPbsTable
              backups={filteredPBSBackups()}
              hasAnyArtifacts={pbsArtifacts().length > 0}
              errorMessage={(pbsBackups.error as Error | undefined)?.message}
              isLoading={pbsBackups() === undefined}
              onRefresh={() => void refetchPBS()}
              emptyIcon={props.emptyIcon}
              emptyTitle={sourceDetailSpecFor('pbs').emptyTitle}
              emptyDescription={sourceDetailSpecFor('pbs').emptyDescription}
              sortKey={pbsSortKey}
              sortDirection={pbsSortDirection}
              onSort={handlePBSSort}
              sizeMaxBytes={pbsSizeMaxBytes()}
            />
          </Show>

          <Show when={tab() === 'sources' && sourceDetailTab() === 'snapshots'}>
            <ProxmoxSnapshotsTable
              guests={filteredSnapshotGuests()}
              hasAnySnapshots={snapshots().length > 0}
              emptyIcon={props.emptyIcon}
              emptyTitle={sourceDetailSpecFor('snapshots').emptyTitle}
              emptyDescription={sourceDetailSpecFor('snapshots').emptyDescription}
              sortKey={snapshotSortKey}
              sortDirection={snapshotSortDirection}
              onSort={handleSnapshotSort}
              showSizeColumn={showSnapshotSizeColumn()}
              showRAMColumn={showSnapshotRAMColumn()}
              columnCount={snapshotColumnCount()}
              expandedKeys={expandedGuests()}
              onToggleExpand={toggleGuestExpansion}
              nowMs={nowMs()}
            />
          </Show>

          <Show when={tab() === 'sources' && sourceDetailTab() === 'archives'}>
            <ProxmoxArchivesTable
              archives={filteredArchives()}
              hasAnyArchives={archives().length > 0}
              emptyIcon={props.emptyIcon}
              emptyTitle={sourceDetailSpecFor('archives').emptyTitle}
              emptyDescription={sourceDetailSpecFor('archives').emptyDescription}
              sortKey={archiveSortKey}
              sortDirection={archiveSortDirection}
              onSort={handleArchiveSort}
              showPBSColumns={showArchivePBSColumns()}
              sizeMaxBytes={archiveSizeMaxBytes()}
              nowMs={nowMs()}
            />
          </Show>

          <Show when={tab() === 'tasks'}>
            <ProxmoxTasksTable
              tasks={filteredTasks()}
              hasAnyTasks={tasks().length > 0}
              emptyIcon={props.emptyIcon}
              emptyTitle={tabSpecFor('tasks').emptyTitle}
              emptyDescription={tabSpecFor('tasks').emptyDescription}
              sortKey={taskSortKey}
              sortDirection={taskSortDirection}
              onSort={handleTaskSort}
              showSizeColumn={showTaskSizeColumn()}
              showErrorColumn={showTaskErrorColumn()}
              durationBaselineSeconds={taskDurationBaselineSeconds()}
            />
          </Show>
        </div>
      </Show>
    </Show>
  );
};

export default ProxmoxBackupsTable;
