import {
  For,
  Show,
  createMemo,
  createResource,
  createSignal,
  type Component,
  type JSX,
} from 'solid-js';
import ArchiveIcon from 'lucide-solid/icons/archive';
import CameraIcon from 'lucide-solid/icons/camera';
import ActivityIcon from 'lucide-solid/icons/activity';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { FilterButtonGroup, type FilterOption } from '@/components/shared/FilterButtonGroup';
import { ProgressBar } from '@/components/shared/ProgressBar';
import { SearchInput } from '@/components/shared/SearchInput';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCard } from '@/components/shared/TableCard';
import type { StatusIndicatorVariant } from '@/utils/status';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
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
  PVEBackupsPayload,
  PVEBackupsResponse,
  StorageBackup,
} from '@/types/api';
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
  classifyArchiveRowAge,
  classifySnapshotRowAge,
  computeMedianTaskDurationSeconds,
  getBackupAgeBucketPresentation,
  guestKey,
  taskDurationSeconds,
} from './proxmoxBackupSummaryPresentation';
import { ProxmoxBackupsCoverageStrip } from './ProxmoxBackupsCoverageStrip';

// Proxmox VE backups split into three meaningfully different surfaces:
//   - Snapshots: qm/pct snapshots taken on the host (no external storage)
//   - vzdump files: backup archives written to a PVE storage (often a
//     remote PBS or NFS share). Each is a discrete restorable artifact.
//   - Recent tasks: the rolling backup-job execution log; this is what
//     surfaces "did last night's backup actually run?"
// PBS-resident backups (deduplicated server-side) get their own platform
// page, so this table is explicitly the PVE backup story.

type BackupTabId = 'snapshots' | 'archives' | 'tasks';

interface BackupTabSpec {
  id: BackupTabId;
  label: string;
  icon: () => JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
}

const TABS: BackupTabSpec[] = [
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
  {
    id: 'tasks',
    label: 'Recent tasks',
    icon: () => <ActivityIcon class="h-4 w-4" aria-hidden="true" />,
    emptyTitle: 'No recent backup tasks',
    emptyDescription: 'Backup-job task results from the past few days will appear here.',
  },
];

// Replication colours the per-row status word to match its dot (emerald
// text for Healthy, red for Failed, etc.). Mirror that here so the
// Recent tasks STATUS column reads the same way — dot + same-tone word.
function classifyTaskStatus(status: string): {
  variant: StatusIndicatorVariant;
  label: string;
  toneClass: string;
} {
  const normalized = (status ?? '').toLowerCase();
  if (normalized === 'ok' || normalized === 'success' || normalized === 'completed') {
    return {
      variant: 'success',
      label: 'OK',
      toneClass: 'text-emerald-600 dark:text-emerald-300',
    };
  }
  if (normalized === 'running') {
    return {
      variant: 'warning',
      label: 'Running',
      toneClass: 'text-amber-600 dark:text-amber-300',
    };
  }
  if (normalized === 'failed' || normalized === 'error') {
    return { variant: 'danger', label: 'Failed', toneClass: 'text-red-600 dark:text-red-300' };
  }
  if (!normalized) return { variant: 'muted', label: '—', toneClass: 'text-muted' };
  return { variant: 'muted', label: status, toneClass: 'text-muted' };
}

function guestLabel(type: string | undefined, vmid: number): string {
  const t = (type ?? '').toLowerCase();
  const kind = t === 'ct' ? 'CT' : t === 'vm' ? 'VM' : t.toUpperCase() || 'Guest';
  return `${kind} ${vmid}`;
}

function formatDuration(start: string | undefined, end: string | undefined): string {
  if (!start || !end) return '—';
  const startMs = new Date(start).getTime();
  const endMs = new Date(end).getTime();
  if (!Number.isFinite(startMs) || !Number.isFinite(endMs) || endMs <= startMs) return '—';
  const seconds = Math.round((endMs - startMs) / 1000);
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  return `${h}h ${m}m`;
}

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

const statusDot = (className: string) => <span class={`h-2 w-2 rounded-full ${className}`} />;

const ARCHIVE_STATUS_FILTERS: FilterOption<'all' | 'protected' | 'verified' | 'unverified'>[] = [
  { value: 'all', label: 'All' },
  { value: 'protected', label: 'Protected', tone: 'info', leading: statusDot('bg-blue-500') },
  { value: 'verified', label: 'Verified', tone: 'success', leading: statusDot('bg-emerald-500') },
  { value: 'unverified', label: 'Unverified', tone: 'warning', leading: statusDot('bg-amber-500') },
];

const TASK_STATUS_FILTERS: FilterOption<'all' | 'ok' | 'failed' | 'running'>[] = [
  { value: 'all', label: 'All' },
  { value: 'ok', label: 'OK', tone: 'success', leading: statusDot('bg-emerald-500') },
  { value: 'failed', label: 'Failed', tone: 'danger', leading: statusDot('bg-red-500') },
  { value: 'running', label: 'Running', tone: 'info', leading: statusDot('bg-blue-500') },
];

type SnapshotFilterValue = 'all' | 'recent' | 'stale' | 'with-ram';

const SNAPSHOT_FILTERS: FilterOption<SnapshotFilterValue>[] = [
  { value: 'all', label: 'All' },
  {
    value: 'recent',
    label: 'Recent ≤30d',
    tone: 'success',
    leading: statusDot('bg-emerald-500'),
  },
  {
    value: 'stale',
    label: 'Stale >30d',
    tone: 'warning',
    leading: statusDot('bg-amber-500'),
  },
  {
    value: 'with-ram',
    label: 'With RAM',
    tone: 'info',
    leading: statusDot('bg-violet-500'),
  },
];

function formatDurationFromSeconds(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return '—';
  const rounded = Math.round(seconds);
  if (rounded < 60) return `${rounded}s`;
  if (rounded < 3600) return `${Math.floor(rounded / 60)}m ${rounded % 60}s`;
  const h = Math.floor(rounded / 3600);
  const m = Math.floor((rounded % 3600) / 60);
  return `${h}h ${m}m`;
}

// Canonical row metric bar — same shape as Storage's usage bar, Ceph's
// pool usage bar, and Workloads' MetricBar: a full-cell-width
// ProgressBar with the value text overlaid on top of the fill. The
// shared `ProgressBar` primitive (foreignObject-based fill that clips
// the label) is the one source of truth for this pattern in Pulse, so
// the Backups tabs read identically to the rest of the app.
function RowMetricBar(props: {
  valuePct: number;
  fillClass: string;
  label: string;
  tooltip?: string;
}) {
  return (
    <div
      class="metric-text relative h-4 w-full min-w-[5rem] overflow-hidden"
      title={props.tooltip ?? props.label}
    >
      <ProgressBar
        value={props.valuePct}
        class="h-full"
        fillClass={props.fillClass}
        label={
          <span class="absolute inset-0 flex items-center justify-center text-[10px] font-medium leading-none text-base-content tabular-nums">
            <span class="max-w-full truncate px-1 text-center">{props.label}</span>
          </span>
        }
      />
    </div>
  );
}

// SLA-aligned row swatch helpers exposed for inner-table rendering — both
// the guest row's leftmost dot and the per-snapshot inner row dot use the
// same `classifySnapshotRowAge` so colours match the coverage strip.

export const ProxmoxBackupsTable: Component<{
  emptyIcon: JSX.Element;
}> = (props) => {
  const [backups, { refetch }] = createResource<PVEBackupsPayload>(fetchPVEBackups);
  const [tab, setTab] = createSignal<BackupTabId>('snapshots');
  const [search, setSearch] = createSignal('');
  const [archiveFilter, setArchiveFilter] = createSignal<
    'all' | 'protected' | 'verified' | 'unverified'
  >('all');
  const [taskFilter, setTaskFilter] = createSignal<'all' | 'ok' | 'failed' | 'running'>('all');
  const [snapshotFilter, setSnapshotFilter] = createSignal<SnapshotFilterValue>('all');
  const [chartRange, setChartRange] = createSignal<BackupActivityRangeDays>(30);
  const [selectedDateKey, setSelectedDateKey] = createSignal<string | null>(null);
  const [archiveMetricMode, setArchiveMetricMode] = createSignal<BackupActivityMetricMode>('count');
  const [expandedGuests, setExpandedGuests] = createSignal<ReadonlySet<string>>(new Set<string>());
  const toggleDay = (key: string) =>
    setSelectedDateKey((current) => (current === key ? null : key));
  const toggleGuestExpansion = (key: string) =>
    setExpandedGuests((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  const snapshots = createMemo<GuestSnapshot[]>(() => backups()?.guestSnapshots ?? []);
  const archives = createMemo<StorageBackup[]>(() => backups()?.storageBackups ?? []);
  const tasks = createMemo<BackupTask[]>(() => backups()?.backupTasks ?? []);
  // Render-time `now` used for age bucketing. We snapshot once per render
  // so all comparisons within a single render share a reference moment;
  // not reactive to ticking time (good enough for sysadmin grouping).
  const nowMs = createMemo(() => Date.now());

  const snapshotCoverage = createMemo(() => buildSnapshotCoverageSummary(snapshots(), nowMs()));
  const archiveCoverage = createMemo(() => buildArchiveCoverageSummary(archives(), nowMs()));
  const taskOutcome = createMemo(() => buildTaskOutcomeSummary(tasks()));

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

  const snapshotMatchesSearch = (snap: GuestSnapshot, term: string): boolean => {
    if (!term) return true;
    return [snap.name, snap.node, snap.instance, snap.description, String(snap.vmid)]
      .filter(Boolean)
      .join(' ')
      .toLowerCase()
      .includes(term);
  };

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

  interface SnapshotGuestRow {
    key: string;
    type: string;
    vmid: number;
    instance: string;
    node: string;
    snapshots: GuestSnapshot[]; // newest first
    count: number;
    withRamCount: number;
    newestMs: number | undefined;
    totalBytes: number;
    // Filter discriminator aligned with the snapshot coverage strip: any
    // newest-snapshot age > 30d is treated as stale (covers strip's stale
    // 30–90d and ancient >90d segments). Row-dot colours are derived
    // independently from classifySnapshotRowAge in the JSX.
    isStale: boolean;
  }

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
      if (typeof snap.sizeBytes === 'number' && snap.sizeBytes > 0) row.totalBytes += snap.sizeBytes;
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
          const newest = visibleSnapshots[0]
            ? snapshotTimestampMs(visibleSnapshots[0])
            : undefined;
          if (newest === undefined) return true;
          return now - newest > 30 * 24 * 60 * 60 * 1000;
        })(),
      };
      if (filter === 'recent' && enriched.isStale) continue;
      if (filter === 'stale' && !enriched.isStale) continue;
      if (filter === 'with-ram' && enriched.withRamCount === 0) continue;
      rows.push(enriched);
    }
    rows.sort((a, b) => (b.newestMs ?? 0) - (a.newestMs ?? 0));
    return rows;
  });

  const filteredArchives = createMemo(() => {
    const term = search().trim().toLowerCase();
    const filter = archiveFilter();
    const dateKey = selectedDateKey();
    return archives().filter((arc) => {
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
  });

  const filteredTasks = createMemo(() => {
    const term = search().trim().toLowerCase();
    const filter = taskFilter();
    const dateKey = selectedDateKey();
    return tasks().filter((task) => {
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
  });

  const totalForTab = createMemo<number>(() => {
    switch (tab()) {
      case 'snapshots':
        return snapshots().length;
      case 'archives':
        return archives().length;
      case 'tasks':
        return tasks().length;
      default:
        return 0;
    }
  });

  const visibleForTab = createMemo<number>(() => {
    switch (tab()) {
      case 'snapshots':
        return filteredSnapshots().length;
      case 'archives':
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

  // The largest archive in the filtered set anchors the size bar.
  const archiveSizeMaxBytes = createMemo(() => {
    let max = 0;
    for (const arc of filteredArchives()) {
      if (arc.size > max) max = arc.size;
    }
    return max;
  });

  const visibleSnapshotGuestCount = createMemo(() => filteredSnapshotGuests().length);

  return (
    <Show
      when={!backups.error}
      fallback={
        <Card padding="lg">
          <EmptyState
            icon={props.emptyIcon}
            title="Could not load PVE backups"
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
              title="Loading PVE backups"
              description="Reading snapshots, archives and recent backup tasks."
            />
          </Card>
        }
      >
        <div class="space-y-3">
          <div class="flex flex-wrap items-center gap-1 rounded-md border border-border bg-surface p-1">
            <For each={TABS}>
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
                    {spec.id === 'snapshots'
                      ? snapshots().length
                      : spec.id === 'archives'
                        ? archives().length
                        : tasks().length}
                  </span>
                </button>
              )}
            </For>
          </div>

          <Show when={tab() === 'snapshots' && snapshots().length > 0}>
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
          <Show when={tab() === 'archives'}>
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
                  tab() === 'snapshots'
                    ? 'Search snapshots, guests, nodes'
                    : tab() === 'archives'
                      ? 'Search archives, storages, nodes'
                      : 'Search tasks, nodes, errors'
                }
              />
            </div>
            <Show when={tab() === 'snapshots'}>
              <FilterButtonGroup
                variant="compact"
                options={SNAPSHOT_FILTERS}
                value={snapshotFilter()}
                onChange={setSnapshotFilter}
              />
            </Show>
            <Show when={tab() === 'archives'}>
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
                when={tab() === 'snapshots'}
                fallback={
                  <Show
                    when={visibleForTab() !== totalForTab()}
                    fallback={
                      <>
                        {totalForTab()} {tab() === 'archives' ? 'archives' : 'tasks'}
                      </>
                    }
                  >
                    {visibleForTab()} of {totalForTab()}{' '}
                    {tab() === 'archives' ? 'archives' : 'tasks'}
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

          <Show when={tab() === 'snapshots'}>
            <Show
              when={filteredSnapshotGuests().length > 0}
              fallback={
                <Card padding="lg">
                  <EmptyState
                    icon={props.emptyIcon}
                    title={
                      snapshots().length === 0
                        ? TABS[0].emptyTitle
                        : 'No snapshots match current filters'
                    }
                    description={
                      snapshots().length === 0
                        ? TABS[0].emptyDescription
                        : 'Adjust the search or filters to see more snapshots.'
                    }
                  />
                </Card>
              }
            >
              <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
                <Table class="min-w-[900px] text-xs">
                  <TableHeader>
                    <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                      <TableHead class={getPlatformTableHeadClassForKind('name')}>Guest</TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('text')}>Node</TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                        Latest
                      </TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                        Snapshots
                      </TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                        Total size
                      </TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('text')}>RAM</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                    <For each={filteredSnapshotGuests()}>
                      {(row) => {
                        const isExpanded = () => expandedGuests().has(row.key);
                        const rowAge = classifySnapshotRowAge(row.newestMs, nowMs());
                        return (
                          <>
                            <TableRow
                              class="cursor-pointer hover:bg-surface-hover"
                              onClick={() => toggleGuestExpansion(row.key)}
                            >
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('name')} text-base-content`}
                              >
                                <div class="flex items-center gap-2">
                                  <ChevronRightIcon
                                    class={`h-3.5 w-3.5 shrink-0 text-muted transition-transform ${
                                      isExpanded() ? 'rotate-90' : ''
                                    }`}
                                    aria-hidden="true"
                                  />
                                  <span class="font-semibold">
                                    {guestLabel(row.type, row.vmid)}
                                  </span>
                                </div>
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('text')} text-base-content font-mono text-[11px]`}
                              >
                                {row.node || '—'}
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                              >
                                <Show
                                  when={row.newestMs !== undefined}
                                  fallback={<span class="text-muted">—</span>}
                                >
                                  <div class="flex items-center justify-end gap-2">
                                    <span
                                      class={`h-1.5 w-1.5 shrink-0 rounded-full ${rowAge.swatchClass}`}
                                      aria-hidden="true"
                                      title={`Newest snapshot: ${rowAge.label}`}
                                    />
                                    <span>{formatRelativeTime(row.newestMs, { compact: true })}</span>
                                  </div>
                                </Show>
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                              >
                                {row.count}
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                              >
                                <Show
                                  when={row.totalBytes > 0}
                                  fallback={<span class="text-muted">—</span>}
                                >
                                  {formatBytes(row.totalBytes)}
                                </Show>
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                              >
                                <Show
                                  when={row.withRamCount > 0}
                                  fallback={<span class="text-muted">—</span>}
                                >
                                  <span class="inline-flex items-center rounded-sm bg-violet-100 px-1.5 py-0.5 text-[10px] font-semibold text-violet-700 dark:bg-violet-900/40 dark:text-violet-200">
                                    {row.withRamCount} with RAM
                                  </span>
                                </Show>
                              </TableCell>
                            </TableRow>
                            <Show when={isExpanded()}>
                              <TableRow class="bg-surface-alt/40">
                                <TableCell class="px-2 py-2" colspan={6}>
                                  <div class="overflow-hidden">
                                    <table class="w-full text-[11px]">
                                      <thead>
                                        <tr class="bg-surface-alt text-muted">
                                          <th class="px-2 py-0.5 text-left font-medium">Name</th>
                                          <th class="px-2 py-0.5 text-left font-medium">Parent</th>
                                          <th class="px-2 py-0.5 text-right font-medium">
                                            Captured
                                          </th>
                                          <th class="px-2 py-0.5 text-right font-medium">Size</th>
                                          <th class="px-2 py-0.5 text-left font-medium">RAM</th>
                                        </tr>
                                      </thead>
                                      <tbody class="divide-y divide-border-subtle">
                                        <For each={row.snapshots}>
                                          {(snap) => {
                                            const age = classifySnapshotRowAge(
                                              snap.time,
                                              nowMs(),
                                            );
                                            return (
                                              <tr class="hover:bg-surface-hover">
                                                <td class="px-2 py-1">
                                                  <div class="flex items-center gap-2">
                                                    <span
                                                      class={`h-1.5 w-1.5 shrink-0 rounded-full ${age.swatchClass}`}
                                                      aria-hidden="true"
                                                      title={`Age: ${age.label}`}
                                                    />
                                                    <div class="min-w-0">
                                                      <div class="font-medium text-base-content">
                                                        {snap.name || '—'}
                                                      </div>
                                                      <Show when={!!snap.description?.trim()}>
                                                        <div
                                                          class="truncate max-w-[24rem] text-[10px] text-muted"
                                                          title={snap.description}
                                                        >
                                                          {snap.description}
                                                        </div>
                                                      </Show>
                                                    </div>
                                                  </div>
                                                </td>
                                                <td class="px-2 py-1 font-mono text-[10px] text-muted">
                                                  {snap.parent?.trim() || '—'}
                                                </td>
                                                <td class="px-2 py-1 text-right text-base-content">
                                                  {formatRelativeTime(snap.time, {
                                                    compact: true,
                                                  })}
                                                </td>
                                                <td class="px-2 py-1 text-right tabular-nums text-base-content">
                                                  <Show
                                                    when={snap.sizeBytes && snap.sizeBytes > 0}
                                                    fallback={<span class="text-muted">—</span>}
                                                  >
                                                    {formatBytes(snap.sizeBytes ?? 0)}
                                                  </Show>
                                                </td>
                                                <td class="px-2 py-1">
                                                  <Show
                                                    when={snap.vmstate}
                                                    fallback={<span class="text-muted">—</span>}
                                                  >
                                                    <span class="inline-flex items-center rounded-sm bg-violet-100 px-1.5 py-0.5 text-[10px] font-semibold text-violet-700 dark:bg-violet-900/40 dark:text-violet-200">
                                                      with RAM
                                                    </span>
                                                  </Show>
                                                </td>
                                              </tr>
                                            );
                                          }}
                                        </For>
                                      </tbody>
                                    </table>
                                  </div>
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

          <Show when={tab() === 'archives'}>
            <Show
              when={filteredArchives().length > 0}
              fallback={
                <Card padding="lg">
                  <EmptyState
                    icon={props.emptyIcon}
                    title={
                      archives().length === 0
                        ? TABS[1].emptyTitle
                        : 'No archives match current filters'
                    }
                    description={
                      archives().length === 0
                        ? TABS[1].emptyDescription
                        : 'Adjust the search or status filter to see more archives.'
                    }
                  />
                </Card>
              }
            >
              <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
                <Table class="min-w-[1050px] text-xs">
                  <TableHeader>
                    <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                      <TableHead class={getPlatformTableHeadClassForKind('name')}>Volume</TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('text')}>Guest</TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('text')}>
                        Storage
                      </TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('text')}>Node</TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('text')}>Format</TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                        Created
                      </TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('metric-bar')}>
                        Size
                      </TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('text')}>
                        Protection
                      </TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('text')}>
                        Verified
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                    <For each={filteredArchives()}>
                      {(arc) => (
                        <TableRow class="hover:bg-surface-hover">
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('name')} text-base-content font-mono text-[11px]`}
                          >
                            <span class="inline-block max-w-[18rem] truncate" title={arc.volid}>
                              {arc.volid}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            {guestLabel(arc.type, arc.vmid)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            {arc.storage || '—'}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content font-mono text-[11px]`}
                          >
                            {arc.node || '—'}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content uppercase text-[10px]`}
                          >
                            {arc.format || '—'}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <div class="flex items-center justify-end gap-2">
                              {(() => {
                                const age = classifyArchiveRowAge(arc.time, nowMs());
                                return (
                                  <span
                                    class={`h-1.5 w-1.5 shrink-0 rounded-full ${age.swatchClass}`}
                                    aria-hidden="true"
                                    title={`Coverage: ${age.label}`}
                                  />
                                );
                              })()}
                              <span>{formatRelativeTime(arc.time, { compact: true })}</span>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('metric-bar')} text-base-content`}
                          >
                            <RowMetricBar
                              valuePct={
                                archiveSizeMaxBytes() > 0
                                  ? (arc.size / archiveSizeMaxBytes()) * 100
                                  : 0
                              }
                              fillClass="bg-blue-500/40 dark:bg-blue-500/40"
                              label={formatBytes(arc.size)}
                              tooltip={`${formatBytes(arc.size)} (relative to largest file in view)`}
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <Show when={arc.protected} fallback={<span class="text-muted">—</span>}>
                              <span class="inline-flex items-center rounded-sm bg-amber-100 px-1.5 py-0.5 text-[10px] font-semibold text-amber-700 dark:bg-amber-900/40 dark:text-amber-200">
                                Protected
                              </span>
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <Show
                              when={arc.verified}
                              fallback={
                                <Show when={arc.isPBS} fallback={<span class="text-muted">—</span>}>
                                  <span class="text-muted">Pending</span>
                                </Show>
                              }
                            >
                              <span class="inline-flex items-center rounded-sm bg-emerald-100 px-1.5 py-0.5 text-[10px] font-semibold text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200">
                                Verified
                              </span>
                            </Show>
                          </TableCell>
                        </TableRow>
                      )}
                    </For>
                  </TableBody>
                </Table>
              </TableCard>
            </Show>
          </Show>

          <Show when={tab() === 'tasks'}>
            <Show
              when={filteredTasks().length > 0}
              fallback={
                <Card padding="lg">
                  <EmptyState
                    icon={props.emptyIcon}
                    title={
                      tasks().length === 0 ? TABS[2].emptyTitle : 'No tasks match current filters'
                    }
                    description={
                      tasks().length === 0
                        ? TABS[2].emptyDescription
                        : 'Adjust the search or status filter to see more tasks.'
                    }
                  />
                </Card>
              }
            >
              <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
                <Table class="min-w-[1000px] text-xs">
                  <TableHeader>
                    <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                      <TableHead class={getPlatformTableHeadClassForKind('text')}>Status</TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('text')}>Guest</TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('text')}>Node</TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                        Started
                      </TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('metric-bar')}>
                        Duration
                      </TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                        Size
                      </TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('text')}>Error</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                    <For each={filteredTasks()}>
                      {(task) => {
                        const classify = classifyTaskStatus(task.status);
                        const durationSec = taskDurationSeconds(task);
                        // Anchor the bar so the median sits at ~50%.
                        const durationBarPct = () => {
                          const baseline = taskDurationBaselineSeconds();
                          if (!durationSec || baseline <= 0) return 0;
                          return (durationSec / (baseline * 2)) * 100;
                        };
                        const durationToneClass = () => {
                          const baseline = taskDurationBaselineSeconds();
                          if (!durationSec || baseline <= 0) return 'bg-slate-400';
                          const ratio = durationSec / baseline;
                          if (ratio >= 2) return 'bg-amber-500';
                          if (ratio >= 1.5) return 'bg-amber-400';
                          return 'bg-emerald-500';
                        };
                        return (
                          <TableRow class="hover:bg-surface-hover">
                            <TableCell class={getPlatformTableCellClassForKind('text')}>
                              <div class="flex items-center gap-2">
                                <StatusDot
                                  size="sm"
                                  variant={classify.variant}
                                  title={classify.label}
                                  ariaHidden
                                />
                                <span class={`text-[11px] font-medium ${classify.toneClass}`}>
                                  {classify.label}
                                </span>
                              </div>
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                            >
                              {guestLabel(task.type, task.vmid)}
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} text-base-content font-mono text-[11px]`}
                            >
                              {task.node || '—'}
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                            >
                              {formatRelativeTime(task.startTime, { compact: true })}
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('metric-bar')} text-base-content`}
                            >
                              <RowMetricBar
                                valuePct={
                                  taskDurationBaselineSeconds() > 0 && durationSec
                                    ? durationBarPct()
                                    : 0
                                }
                                fillClass={durationToneClass()}
                                label={formatDuration(task.startTime, task.endTime)}
                                tooltip={`Duration ${formatDuration(task.startTime, task.endTime)} (median ${formatDurationFromSeconds(taskDurationBaselineSeconds())})`}
                              />
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                            >
                              <Show
                                when={task.size && task.size > 0}
                                fallback={<span class="text-muted">—</span>}
                              >
                                {formatBytes(task.size ?? 0)}
                              </Show>
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                            >
                              <Show
                                when={!!task.error?.trim()}
                                fallback={<span class="text-muted">—</span>}
                              >
                                <span
                                  class="inline-block max-w-[18rem] truncate text-red-600 dark:text-red-300"
                                  title={task.error}
                                >
                                  {task.error}
                                </span>
                              </Show>
                            </TableCell>
                          </TableRow>
                        );
                      }}
                    </For>
                  </TableBody>
                </Table>
              </TableCard>
            </Show>
          </Show>
        </div>
      </Show>
    </Show>
  );
};

export default ProxmoxBackupsTable;
