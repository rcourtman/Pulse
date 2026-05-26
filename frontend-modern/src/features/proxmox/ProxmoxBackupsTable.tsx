import {
  For,
  Show,
  createEffect,
  createMemo,
  createResource,
  createSignal,
  type Accessor,
  type Component,
  type JSX,
} from 'solid-js';
import ArchiveIcon from 'lucide-solid/icons/archive';
import CameraIcon from 'lucide-solid/icons/camera';
import ActivityIcon from 'lucide-solid/icons/activity';
import DatabaseIcon from 'lucide-solid/icons/database';
import ServerIcon from 'lucide-solid/icons/server';
import ShieldCheckIcon from 'lucide-solid/icons/shield-check';
import ArrowDownIcon from 'lucide-solid/icons/arrow-down';
import ArrowUpIcon from 'lucide-solid/icons/arrow-up';
import ArrowUpDownIcon from 'lucide-solid/icons/arrow-up-down';
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
  classifyArchiveRowAge,
  classifySnapshotRowAge,
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
import { ProxmoxBackupsCoverageStrip } from './ProxmoxBackupsCoverageStrip';

// Proxmox backups are intentionally organized around operator questions, not
// storage-source mechanics:
//   - Workload coverage answers "does this workload have a backup?" by default.
//   - Restore points answers "what exactly can I restore?" across every source.
//   - Source details keeps PBS/PVE evidence available without making those
//     implementation-specific tables equal-weight primary destinations.
//   - Job history shows whether backup jobs are actually running successfully.

type BackupTabId = 'coverage' | 'recoverable' | 'sources' | 'tasks';
type SourceDetailTabId = 'pbs' | 'snapshots' | 'archives';

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

function pbsWorkloadLabel(backup: PBSBackup): string {
  const t = (backup.backupType ?? '').toLowerCase();
  if (t === 'ct') return `CT ${backup.vmid}`;
  if (t === 'vm') return `VM ${backup.vmid}`;
  if (t === 'host') return backup.vmid ? `Host ${backup.vmid}` : 'Host';
  const kind = backup.backupType?.trim().toUpperCase() || 'Backup';
  return backup.vmid ? `${kind} ${backup.vmid}` : kind;
}

function pbsRepositoryLabel(backup: PBSBackup): string {
  const namespace = backup.namespace?.trim() || '(root)';
  return `${backup.datastore || '—'} / ${namespace}`;
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

async function fetchPBSBackups(): Promise<PBSBackupsPayload> {
  const response = await apiFetch('/api/backups/pbs');
  if (!response.ok) {
    throw new Error(`Failed to load PBS backups (${response.status})`);
  }
  const payload = (await response.json()) as PBSBackupsResponse;
  return payload?.data ?? { backups: [] };
}

const statusDot = (className: string) => <span class={`h-2 w-2 rounded-full ${className}`} />;

const ARCHIVE_STATUS_FILTERS: FilterOption<'all' | 'protected' | 'verified' | 'unverified'>[] = [
  { value: 'all', label: 'All' },
  { value: 'protected', label: 'Protected', tone: 'info', leading: statusDot('bg-blue-500') },
  { value: 'verified', label: 'Verified', tone: 'success', leading: statusDot('bg-emerald-500') },
  { value: 'unverified', label: 'Unverified', tone: 'warning', leading: statusDot('bg-amber-500') },
];

const PBS_STATUS_FILTERS: FilterOption<'all' | 'protected' | 'verified' | 'unverified'>[] = [
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

type CoverageFilterValue = 'all' | 'attention' | 'current' | 'uncovered';

const COVERAGE_FILTERS: FilterOption<CoverageFilterValue>[] = [
  { value: 'all', label: 'All' },
  { value: 'attention', label: 'Attention', tone: 'warning', leading: statusDot('bg-amber-500') },
  { value: 'current', label: 'Current', tone: 'success', leading: statusDot('bg-emerald-500') },
  { value: 'uncovered', label: 'Uncovered', tone: 'danger', leading: statusDot('bg-red-500') },
];

type RecoverableFilterValue = 'all' | 'pbs' | 'archive' | 'snapshot' | 'verified' | 'unverified';

const RECOVERABLE_FILTERS: FilterOption<RecoverableFilterValue>[] = [
  { value: 'all', label: 'All' },
  { value: 'pbs', label: 'PBS', tone: 'info', leading: statusDot('bg-cyan-500') },
  { value: 'archive', label: 'Archives', tone: 'info', leading: statusDot('bg-blue-500') },
  { value: 'snapshot', label: 'Snapshots', tone: 'info', leading: statusDot('bg-violet-500') },
  { value: 'verified', label: 'Verified', tone: 'success', leading: statusDot('bg-emerald-500') },
  { value: 'unverified', label: 'Unverified', tone: 'warning', leading: statusDot('bg-amber-500') },
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

function RecoverySourceSummary(props: {
  artifact?: RecoverableArtifact;
  count: number;
  emptyLabel: string;
}) {
  return (
    <Show when={props.artifact} fallback={<span class="text-muted">{props.emptyLabel}</span>}>
      {(artifact) => (
        <div class="min-w-0">
          <div class="text-base-content">
            {formatRelativeTime(artifact().createdAt, { compact: true })}
          </div>
          <div class="truncate text-[10px] text-muted" title={artifact().location}>
            {props.count === 1
              ? artifact().location
              : `${props.count} total · ${artifact().location}`}
          </div>
        </div>
      )}
    </Show>
  );
}

function ArtifactStateBadge(props: { artifact: RecoverableArtifact; label: string }) {
  if (props.artifact.sourceKind === 'snapshot') {
    return (
      <span class="inline-flex items-center rounded-sm bg-violet-100 px-1.5 py-0.5 text-[10px] font-semibold text-violet-700 dark:bg-violet-900/40 dark:text-violet-200">
        {props.label}
      </span>
    );
  }
  if (props.artifact.protected) {
    return (
      <span class="inline-flex items-center rounded-sm bg-amber-100 px-1.5 py-0.5 text-[10px] font-semibold text-amber-700 dark:bg-amber-900/40 dark:text-amber-200">
        {props.label}
      </span>
    );
  }
  if (props.artifact.verified === true) {
    return (
      <span class="inline-flex items-center rounded-sm bg-emerald-100 px-1.5 py-0.5 text-[10px] font-semibold text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200">
        {props.label}
      </span>
    );
  }
  if (props.artifact.verified === false) {
    return (
      <span class="inline-flex items-center rounded-sm bg-amber-100 px-1.5 py-0.5 text-[10px] font-semibold text-amber-700 dark:bg-amber-900/40 dark:text-amber-200">
        {props.label}
      </span>
    );
  }
  return <span class="text-muted">{props.label}</span>;
}

function ArtifactSourceBadge(props: { artifact: RecoverableArtifact }) {
  return (
    <span
      class={`inline-flex items-center rounded-sm px-1.5 py-0.5 text-[10px] font-semibold ${
        props.artifact.sourceKind === 'pbs'
          ? 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/40 dark:text-cyan-200'
          : props.artifact.sourceKind === 'archive'
            ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200'
            : 'bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-200'
      }`}
    >
      {props.artifact.sourceLabel}
    </span>
  );
}

// SLA-aligned row swatch helpers exposed for inner-table rendering — both
// the guest row's leftmost dot and the per-snapshot inner row dot use the
// same `classifySnapshotRowAge` so colours match the coverage strip.

// Sortable column header — matches Storage's pattern (StoragePoolsTable.tsx).
// Clicking an inactive column sorts it with the supplied default direction;
// clicking the active column flips direction. Renders the idle / asc / desc
// arrow trio so the affordance reads identically across the app.
const SORT_BUTTON_CLASS =
  'inline-flex min-w-0 max-w-full items-center gap-1 rounded-sm outline-none transition-colors hover:text-base-content focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-1 focus-visible:ring-offset-surface';
const SORT_ICON_CLASS = 'h-3 w-3 shrink-0';

function SortableHead<K extends string>(props: {
  label: string;
  sortKey: K;
  currentSort: Accessor<K>;
  direction: Accessor<'asc' | 'desc'>;
  onSort: (key: K) => void;
  align?: 'left' | 'right' | 'center';
  headClass: string;
}) {
  const isActive = () => props.currentSort() === props.sortKey;
  const buttonAlignClass = () => {
    if (props.align === 'right') return 'justify-end';
    if (props.align === 'center') return 'justify-center';
    return 'justify-start';
  };
  const ariaLabel = () => {
    if (!isActive()) return `Sort by ${props.label}`;
    return `Sort by ${props.label} ${props.direction() === 'asc' ? 'descending' : 'ascending'}`;
  };
  return (
    <TableHead
      class={props.headClass}
      aria-sort={
        isActive() ? (props.direction() === 'asc' ? 'ascending' : 'descending') : undefined
      }
    >
      <button
        type="button"
        class={`${SORT_BUTTON_CLASS} ${buttonAlignClass()} w-full`}
        onClick={() => props.onSort(props.sortKey)}
        aria-label={ariaLabel()}
        title={ariaLabel()}
      >
        <span class="min-w-0 truncate">{props.label}</span>
        <Show
          when={isActive()}
          fallback={
            <ArrowUpDownIcon class={`${SORT_ICON_CLASS} text-muted/70`} aria-hidden="true" />
          }
        >
          <Show
            when={props.direction() === 'asc'}
            fallback={
              <ArrowDownIcon class={`${SORT_ICON_CLASS} text-base-content`} aria-hidden="true" />
            }
          >
            <ArrowUpIcon class={`${SORT_ICON_CLASS} text-base-content`} aria-hidden="true" />
          </Show>
        </Show>
      </button>
    </TableHead>
  );
}

// Case-insensitive string comparator. Undefined / empty values ALWAYS sort
// to the end regardless of direction — flipping a "—" row to the top of a
// desc sort is never what the user wants (e.g. a running task with no
// finished duration should not headline a "slowest tasks" view).
function cmpString(
  a: string | undefined,
  b: string | undefined,
  direction: 'asc' | 'desc',
): number {
  const av = (a ?? '').toLowerCase();
  const bv = (b ?? '').toLowerCase();
  if (!av && !bv) return 0;
  if (!av) return 1;
  if (!bv) return -1;
  const cmp = av === bv ? 0 : av < bv ? -1 : 1;
  return direction === 'asc' ? cmp : -cmp;
}

function cmpNumber(
  a: number | undefined,
  b: number | undefined,
  direction: 'asc' | 'desc',
): number {
  const av = typeof a === 'number' && Number.isFinite(a) ? a : undefined;
  const bv = typeof b === 'number' && Number.isFinite(b) ? b : undefined;
  if (av === undefined && bv === undefined) return 0;
  if (av === undefined) return 1;
  if (bv === undefined) return -1;
  const cmp = av - bv;
  return direction === 'asc' ? cmp : -cmp;
}

function cmpBool(a: boolean, b: boolean, direction: 'asc' | 'desc'): number {
  const cmp = (a ? 1 : 0) - (b ? 1 : 0);
  return direction === 'asc' ? cmp : -cmp;
}

// Per-tab sort key types and their default directions. Numeric / timestamp
// columns default to `desc` (biggest / newest first) since that's the
// sysadmin-default question on a backup page. String columns default to
// `asc` (A→Z). Boolean columns default to `desc` so "true" sorts first.

type CoverageSortKey = 'posture' | 'workload' | 'latest' | 'pbs' | 'archive' | 'snapshot' | 'task';
const COVERAGE_SORT_DEFAULT_DIRECTION: Record<CoverageSortKey, 'asc' | 'desc'> = {
  posture: 'asc',
  workload: 'asc',
  latest: 'desc',
  pbs: 'desc',
  archive: 'desc',
  snapshot: 'desc',
  task: 'desc',
};

type RecoverableSortKey = 'workload' | 'source' | 'location' | 'created' | 'size' | 'state';
const RECOVERABLE_SORT_DEFAULT_DIRECTION: Record<RecoverableSortKey, 'asc' | 'desc'> = {
  workload: 'asc',
  source: 'asc',
  location: 'asc',
  created: 'desc',
  size: 'desc',
  state: 'asc',
};

type PBSSortKey = 'workload' | 'repository' | 'created' | 'size' | 'protected' | 'verified';
const PBS_SORT_DEFAULT_DIRECTION: Record<PBSSortKey, 'asc' | 'desc'> = {
  workload: 'asc',
  repository: 'asc',
  created: 'desc',
  size: 'desc',
  protected: 'desc',
  verified: 'desc',
};

type SnapshotSortKey = 'guest' | 'node' | 'latest' | 'count' | 'size';
const SNAPSHOT_SORT_DEFAULT_DIRECTION: Record<SnapshotSortKey, 'asc' | 'desc'> = {
  guest: 'asc',
  node: 'asc',
  latest: 'desc',
  count: 'desc',
  size: 'desc',
};

type ArchiveSortKey =
  | 'volume'
  | 'guest'
  | 'storage'
  | 'node'
  | 'format'
  | 'created'
  | 'size'
  | 'protected'
  | 'verified';
const ARCHIVE_SORT_DEFAULT_DIRECTION: Record<ArchiveSortKey, 'asc' | 'desc'> = {
  volume: 'asc',
  guest: 'asc',
  storage: 'asc',
  node: 'asc',
  format: 'asc',
  created: 'desc',
  size: 'desc',
  protected: 'desc',
  verified: 'desc',
};

type TaskSortKey = 'status' | 'guest' | 'node' | 'started' | 'duration' | 'size';
const TASK_SORT_DEFAULT_DIRECTION: Record<TaskSortKey, 'asc' | 'desc'> = {
  status: 'asc',
  guest: 'asc',
  node: 'asc',
  started: 'desc',
  duration: 'desc',
  size: 'desc',
};

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

  const artifactStateLabel = (artifact: RecoverableArtifact): string => {
    if (artifact.sourceKind === 'snapshot') return 'Snapshot';
    if (artifact.protected) return 'Protected';
    if (artifact.verified === true) return 'Verified';
    if (artifact.verified === false) return 'Unverified';
    return 'Archive';
  };

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
            <Show
              when={filteredRecoverableArtifacts().length > 0}
              fallback={
                <Card padding="lg">
                  <EmptyState
                    icon={props.emptyIcon}
                    title={
                      recoveryModel().recoverableArtifacts.length === 0
                        ? tabSpecFor('recoverable').emptyTitle
                        : 'No recoverable artifacts match current filters'
                    }
                    description={
                      recoveryModel().recoverableArtifacts.length === 0
                        ? tabSpecFor('recoverable').emptyDescription
                        : 'Adjust the search, source filter, or selected day to see more artifacts.'
                    }
                  />
                </Card>
              }
            >
              <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
                <Table class="min-w-[1150px] text-xs">
                  <TableHeader>
                    <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                      <SortableHead
                        label="Workload"
                        sortKey="workload"
                        currentSort={recoverableSortKey}
                        direction={recoverableSortDirection}
                        onSort={handleRecoverableSort}
                        align="left"
                        headClass={getPlatformTableHeadClassForKind('name')}
                      />
                      <SortableHead
                        label="Source"
                        sortKey="source"
                        currentSort={recoverableSortKey}
                        direction={recoverableSortDirection}
                        onSort={handleRecoverableSort}
                        align="left"
                        headClass={getPlatformTableHeadClassForKind('text')}
                      />
                      <SortableHead
                        label="Location"
                        sortKey="location"
                        currentSort={recoverableSortKey}
                        direction={recoverableSortDirection}
                        onSort={handleRecoverableSort}
                        align="left"
                        headClass={getPlatformTableHeadClassForKind('text')}
                      />
                      <SortableHead
                        label="Created"
                        sortKey="created"
                        currentSort={recoverableSortKey}
                        direction={recoverableSortDirection}
                        onSort={handleRecoverableSort}
                        align="right"
                        headClass={getPlatformTableHeadClassForKind('numeric-value')}
                      />
                      <SortableHead
                        label="Size"
                        sortKey="size"
                        currentSort={recoverableSortKey}
                        direction={recoverableSortDirection}
                        onSort={handleRecoverableSort}
                        align="center"
                        headClass={getPlatformTableHeadClassForKind('metric-bar')}
                      />
                      <SortableHead
                        label="State"
                        sortKey="state"
                        currentSort={recoverableSortKey}
                        direction={recoverableSortDirection}
                        onSort={handleRecoverableSort}
                        align="left"
                        headClass={getPlatformTableHeadClassForKind('text')}
                      />
                      <TableHead class={getPlatformTableHeadClassForKind('text')}>
                        Details
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                    <For each={filteredRecoverableArtifacts()}>
                      {(artifact) => (
                        <TableRow class="hover:bg-surface-hover">
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('name')} text-base-content`}
                          >
                            <div class="min-w-0">
                              <div class="font-semibold">{artifact.workload.label}</div>
                              <div class="font-mono text-[10px] uppercase text-muted">
                                {artifact.workload.typeLabel} {artifact.workload.vmid}
                              </div>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <ArtifactSourceBadge artifact={artifact} />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <span
                              class="inline-block max-w-[16rem] truncate"
                              title={artifact.location}
                            >
                              {artifact.location}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {formatRelativeTime(artifact.createdAt, { compact: true })}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('metric-bar')} text-base-content`}
                          >
                            <Show
                              when={artifact.size && artifact.size > 0}
                              fallback={<span class="text-muted">No size</span>}
                            >
                              <RowMetricBar
                                valuePct={
                                  recoverableSizeMaxBytes() > 0 && artifact.size
                                    ? (artifact.size / recoverableSizeMaxBytes()) * 100
                                    : 0
                                }
                                fillClass="bg-blue-500/40 dark:bg-blue-500/40"
                                label={formatBytes(artifact.size ?? 0)}
                                tooltip={`${formatBytes(artifact.size ?? 0)} (relative to largest artifact in view)`}
                              />
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <ArtifactStateBadge
                              artifact={artifact}
                              label={artifactStateLabel(artifact)}
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <span
                              class="inline-block max-w-[20rem] truncate"
                              title={artifact.detail}
                            >
                              {artifact.detail || '—'}
                            </span>
                          </TableCell>
                        </TableRow>
                      )}
                    </For>
                  </TableBody>
                </Table>
              </TableCard>
            </Show>
          </Show>

          <Show when={tab() === 'sources' && sourceDetailTab() === 'pbs'}>
            <Show
              when={!pbsBackups.error}
              fallback={
                <Card padding="lg">
                  <EmptyState
                    icon={props.emptyIcon}
                    title="Could not load PBS artifacts"
                    description={
                      (pbsBackups.error as Error | undefined)?.message ?? 'Refresh to retry.'
                    }
                    actions={
                      <button
                        type="button"
                        onClick={() => void refetchPBS()}
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
                when={pbsBackups() !== undefined}
                fallback={
                  <Card padding="lg">
                    <EmptyState
                      icon={props.emptyIcon}
                      title="Loading PBS artifacts"
                      description="Reading deduplicated backup snapshots from Proxmox Backup Server."
                    />
                  </Card>
                }
              >
                <Show
                  when={filteredPBSBackups().length > 0}
                  fallback={
                    <Card padding="lg">
                      <EmptyState
                        icon={props.emptyIcon}
                        title={
                          pbsArtifacts().length === 0
                            ? sourceDetailSpecFor('pbs').emptyTitle
                            : 'No PBS artifacts match current filters'
                        }
                        description={
                          pbsArtifacts().length === 0
                            ? sourceDetailSpecFor('pbs').emptyDescription
                            : 'Adjust the search or status filter to see more PBS artifacts.'
                        }
                      />
                    </Card>
                  }
                >
                  <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
                    <Table class="min-w-[1050px] text-xs">
                      <TableHeader>
                        <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                          <SortableHead
                            label="Workload"
                            sortKey="workload"
                            currentSort={pbsSortKey}
                            direction={pbsSortDirection}
                            onSort={handlePBSSort}
                            align="left"
                            headClass={getPlatformTableHeadClassForKind('name')}
                          />
                          <SortableHead
                            label="Repository"
                            sortKey="repository"
                            currentSort={pbsSortKey}
                            direction={pbsSortDirection}
                            onSort={handlePBSSort}
                            align="left"
                            headClass={getPlatformTableHeadClassForKind('text')}
                          />
                          <SortableHead
                            label="Created"
                            sortKey="created"
                            currentSort={pbsSortKey}
                            direction={pbsSortDirection}
                            onSort={handlePBSSort}
                            align="right"
                            headClass={getPlatformTableHeadClassForKind('numeric-value')}
                          />
                          <SortableHead
                            label="Size"
                            sortKey="size"
                            currentSort={pbsSortKey}
                            direction={pbsSortDirection}
                            onSort={handlePBSSort}
                            align="center"
                            headClass={getPlatformTableHeadClassForKind('metric-bar')}
                          />
                          <SortableHead
                            label="Verified"
                            sortKey="verified"
                            currentSort={pbsSortKey}
                            direction={pbsSortDirection}
                            onSort={handlePBSSort}
                            align="left"
                            headClass={getPlatformTableHeadClassForKind('text')}
                          />
                          <SortableHead
                            label="Protection"
                            sortKey="protected"
                            currentSort={pbsSortKey}
                            direction={pbsSortDirection}
                            onSort={handlePBSSort}
                            align="left"
                            headClass={getPlatformTableHeadClassForKind('text')}
                          />
                          <TableHead class={getPlatformTableHeadClassForKind('text')}>
                            Files
                          </TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                        <For each={filteredPBSBackups()}>
                          {(backup) => (
                            <TableRow class="hover:bg-surface-hover">
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('name')} text-base-content`}
                              >
                                <div class="min-w-0">
                                  <div class="font-semibold">{pbsWorkloadLabel(backup)}</div>
                                  <div class="font-mono text-[10px] uppercase text-muted">
                                    {backup.backupType || 'backup'}
                                  </div>
                                </div>
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                              >
                                <div class="min-w-0">
                                  <div class="font-mono text-[11px]">
                                    {pbsRepositoryLabel(backup)}
                                  </div>
                                  <div
                                    class="truncate text-[10px] text-muted"
                                    title={backup.instance}
                                  >
                                    {backup.instance || '—'}
                                  </div>
                                </div>
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                              >
                                <Show
                                  when={pbsTimestampMs(backup) !== undefined}
                                  fallback={<span class="text-muted">—</span>}
                                >
                                  {formatRelativeTime(backup.backupTime, { compact: true })}
                                </Show>
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('metric-bar')} text-base-content`}
                              >
                                <RowMetricBar
                                  valuePct={
                                    pbsSizeMaxBytes() > 0
                                      ? (backup.size / pbsSizeMaxBytes()) * 100
                                      : 0
                                  }
                                  fillClass="bg-blue-500/40 dark:bg-blue-500/40"
                                  label={formatBytes(backup.size)}
                                  tooltip={`${formatBytes(backup.size)} (relative to largest PBS artifact in view)`}
                                />
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                              >
                                <Show
                                  when={backup.verified}
                                  fallback={
                                    <span class="text-amber-600 dark:text-amber-300">
                                      Unverified
                                    </span>
                                  }
                                >
                                  <span class="inline-flex items-center rounded-sm bg-emerald-100 px-1.5 py-0.5 text-[10px] font-semibold text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200">
                                    Verified
                                  </span>
                                </Show>
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                              >
                                <Show
                                  when={backup.protected}
                                  fallback={<span class="text-muted">Unprotected</span>}
                                >
                                  <span class="inline-flex items-center rounded-sm bg-blue-100 px-1.5 py-0.5 text-[10px] font-semibold text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                                    Protected
                                  </span>
                                </Show>
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                              >
                                <span
                                  class="inline-block max-w-[16rem] truncate"
                                  title={(backup.files ?? []).join(', ')}
                                >
                                  {(backup.files ?? []).length > 0
                                    ? `${backup.files.length} files`
                                    : '—'}
                                </span>
                              </TableCell>
                            </TableRow>
                          )}
                        </For>
                      </TableBody>
                    </Table>
                  </TableCard>
                </Show>
              </Show>
            </Show>
          </Show>

          <Show when={tab() === 'sources' && sourceDetailTab() === 'snapshots'}>
            <Show
              when={filteredSnapshotGuests().length > 0}
              fallback={
                <Card padding="lg">
                  <EmptyState
                    icon={props.emptyIcon}
                    title={
                      snapshots().length === 0
                        ? sourceDetailSpecFor('snapshots').emptyTitle
                        : 'No snapshots match current filters'
                    }
                    description={
                      snapshots().length === 0
                        ? sourceDetailSpecFor('snapshots').emptyDescription
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
                      <SortableHead
                        label="Guest"
                        sortKey="guest"
                        currentSort={snapshotSortKey}
                        direction={snapshotSortDirection}
                        onSort={handleSnapshotSort}
                        align="left"
                        headClass={getPlatformTableHeadClassForKind('name')}
                      />
                      <SortableHead
                        label="Node"
                        sortKey="node"
                        currentSort={snapshotSortKey}
                        direction={snapshotSortDirection}
                        onSort={handleSnapshotSort}
                        align="left"
                        headClass={getPlatformTableHeadClassForKind('text')}
                      />
                      <SortableHead
                        label="Latest"
                        sortKey="latest"
                        currentSort={snapshotSortKey}
                        direction={snapshotSortDirection}
                        onSort={handleSnapshotSort}
                        align="right"
                        headClass={getPlatformTableHeadClassForKind('numeric-value')}
                      />
                      <SortableHead
                        label="Snapshots"
                        sortKey="count"
                        currentSort={snapshotSortKey}
                        direction={snapshotSortDirection}
                        onSort={handleSnapshotSort}
                        align="right"
                        headClass={getPlatformTableHeadClassForKind('numeric-value')}
                      />
                      <Show when={showSnapshotSizeColumn()}>
                        <SortableHead
                          label="Total size"
                          sortKey="size"
                          currentSort={snapshotSortKey}
                          direction={snapshotSortDirection}
                          onSort={handleSnapshotSort}
                          align="right"
                          headClass={getPlatformTableHeadClassForKind('numeric-value')}
                        />
                      </Show>
                      <Show when={showSnapshotRAMColumn()}>
                        <TableHead class={getPlatformTableHeadClassForKind('text')}>RAM</TableHead>
                      </Show>
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
                                    <span>
                                      {formatRelativeTime(row.newestMs, { compact: true })}
                                    </span>
                                  </div>
                                </Show>
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                              >
                                {row.count}
                              </TableCell>
                              <Show when={showSnapshotSizeColumn()}>
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
                              </Show>
                              <Show when={showSnapshotRAMColumn()}>
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
                              </Show>
                            </TableRow>
                            <Show when={isExpanded()}>
                              <TableRow class="bg-surface-alt/40">
                                <TableCell class="px-2 py-2" colspan={snapshotColumnCount()}>
                                  <div class="overflow-hidden">
                                    <table class="w-full text-[11px]">
                                      <thead>
                                        <tr class="bg-surface-alt text-muted">
                                          <th class="px-2 py-0.5 text-left font-medium">Name</th>
                                          <th class="px-2 py-0.5 text-left font-medium">Parent</th>
                                          <th class="px-2 py-0.5 text-right font-medium">
                                            Captured
                                          </th>
                                          <Show when={showSnapshotSizeColumn()}>
                                            <th class="px-2 py-0.5 text-right font-medium">Size</th>
                                          </Show>
                                          <Show when={showSnapshotRAMColumn()}>
                                            <th class="px-2 py-0.5 text-left font-medium">RAM</th>
                                          </Show>
                                        </tr>
                                      </thead>
                                      <tbody class="divide-y divide-border-subtle">
                                        <For each={row.snapshots}>
                                          {(snap) => {
                                            const age = classifySnapshotRowAge(snap.time, nowMs());
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
                                                <Show when={showSnapshotSizeColumn()}>
                                                  <td class="px-2 py-1 text-right tabular-nums text-base-content">
                                                    <Show
                                                      when={snap.sizeBytes && snap.sizeBytes > 0}
                                                      fallback={<span class="text-muted">—</span>}
                                                    >
                                                      {formatBytes(snap.sizeBytes ?? 0)}
                                                    </Show>
                                                  </td>
                                                </Show>
                                                <Show when={showSnapshotRAMColumn()}>
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
                                                </Show>
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

          <Show when={tab() === 'sources' && sourceDetailTab() === 'archives'}>
            <Show
              when={filteredArchives().length > 0}
              fallback={
                <Card padding="lg">
                  <EmptyState
                    icon={props.emptyIcon}
                    title={
                      archives().length === 0
                        ? sourceDetailSpecFor('archives').emptyTitle
                        : 'No archives match current filters'
                    }
                    description={
                      archives().length === 0
                        ? sourceDetailSpecFor('archives').emptyDescription
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
                      <SortableHead
                        label="Volume"
                        sortKey="volume"
                        currentSort={archiveSortKey}
                        direction={archiveSortDirection}
                        onSort={handleArchiveSort}
                        align="left"
                        headClass={getPlatformTableHeadClassForKind('name')}
                      />
                      <SortableHead
                        label="Guest"
                        sortKey="guest"
                        currentSort={archiveSortKey}
                        direction={archiveSortDirection}
                        onSort={handleArchiveSort}
                        align="left"
                        headClass={getPlatformTableHeadClassForKind('text')}
                      />
                      <SortableHead
                        label="Storage"
                        sortKey="storage"
                        currentSort={archiveSortKey}
                        direction={archiveSortDirection}
                        onSort={handleArchiveSort}
                        align="left"
                        headClass={getPlatformTableHeadClassForKind('text')}
                      />
                      <SortableHead
                        label="Node"
                        sortKey="node"
                        currentSort={archiveSortKey}
                        direction={archiveSortDirection}
                        onSort={handleArchiveSort}
                        align="left"
                        headClass={getPlatformTableHeadClassForKind('text')}
                      />
                      <SortableHead
                        label="Format"
                        sortKey="format"
                        currentSort={archiveSortKey}
                        direction={archiveSortDirection}
                        onSort={handleArchiveSort}
                        align="left"
                        headClass={getPlatformTableHeadClassForKind('text')}
                      />
                      <SortableHead
                        label="Created"
                        sortKey="created"
                        currentSort={archiveSortKey}
                        direction={archiveSortDirection}
                        onSort={handleArchiveSort}
                        align="right"
                        headClass={getPlatformTableHeadClassForKind('numeric-value')}
                      />
                      <SortableHead
                        label="Size"
                        sortKey="size"
                        currentSort={archiveSortKey}
                        direction={archiveSortDirection}
                        onSort={handleArchiveSort}
                        align="center"
                        headClass={getPlatformTableHeadClassForKind('metric-bar')}
                      />
                      <Show when={showArchivePBSColumns()}>
                        <SortableHead
                          label="Protection"
                          sortKey="protected"
                          currentSort={archiveSortKey}
                          direction={archiveSortDirection}
                          onSort={handleArchiveSort}
                          align="left"
                          headClass={getPlatformTableHeadClassForKind('text')}
                        />
                        <SortableHead
                          label="Verified"
                          sortKey="verified"
                          currentSort={archiveSortKey}
                          direction={archiveSortDirection}
                          onSort={handleArchiveSort}
                          align="left"
                          headClass={getPlatformTableHeadClassForKind('text')}
                        />
                      </Show>
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
                          <Show when={showArchivePBSColumns()}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                            >
                              <Show
                                when={arc.protected}
                                fallback={<span class="text-muted">Unprotected</span>}
                              >
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
                                  <Show
                                    when={arc.isPBS}
                                    fallback={<span class="text-muted">n/a</span>}
                                  >
                                    <span class="text-muted">Pending</span>
                                  </Show>
                                }
                              >
                                <span class="inline-flex items-center rounded-sm bg-emerald-100 px-1.5 py-0.5 text-[10px] font-semibold text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200">
                                  Verified
                                </span>
                              </Show>
                            </TableCell>
                          </Show>
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
                      tasks().length === 0
                        ? tabSpecFor('tasks').emptyTitle
                        : 'No tasks match current filters'
                    }
                    description={
                      tasks().length === 0
                        ? tabSpecFor('tasks').emptyDescription
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
                      <SortableHead
                        label="Status"
                        sortKey="status"
                        currentSort={taskSortKey}
                        direction={taskSortDirection}
                        onSort={handleTaskSort}
                        align="left"
                        headClass={getPlatformTableHeadClassForKind('text')}
                      />
                      <SortableHead
                        label="Guest"
                        sortKey="guest"
                        currentSort={taskSortKey}
                        direction={taskSortDirection}
                        onSort={handleTaskSort}
                        align="left"
                        headClass={getPlatformTableHeadClassForKind('text')}
                      />
                      <SortableHead
                        label="Node"
                        sortKey="node"
                        currentSort={taskSortKey}
                        direction={taskSortDirection}
                        onSort={handleTaskSort}
                        align="left"
                        headClass={getPlatformTableHeadClassForKind('text')}
                      />
                      <SortableHead
                        label="Started"
                        sortKey="started"
                        currentSort={taskSortKey}
                        direction={taskSortDirection}
                        onSort={handleTaskSort}
                        align="right"
                        headClass={getPlatformTableHeadClassForKind('numeric-value')}
                      />
                      <SortableHead
                        label="Duration"
                        sortKey="duration"
                        currentSort={taskSortKey}
                        direction={taskSortDirection}
                        onSort={handleTaskSort}
                        align="center"
                        headClass={getPlatformTableHeadClassForKind('metric-bar')}
                      />
                      <Show when={showTaskSizeColumn()}>
                        <SortableHead
                          label="Size"
                          sortKey="size"
                          currentSort={taskSortKey}
                          direction={taskSortDirection}
                          onSort={handleTaskSort}
                          align="right"
                          headClass={getPlatformTableHeadClassForKind('numeric-value')}
                        />
                      </Show>
                      <Show when={showTaskErrorColumn()}>
                        <TableHead class={getPlatformTableHeadClassForKind('text')}>
                          Error
                        </TableHead>
                      </Show>
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
                        // Canonical Pulse metric tones (60% alpha) — same palette
                        // Storage and Ceph row bars use via getMetricColorClass.
                        // Cap at `warning` (soft yellow) rather than going to red
                        // for the worst case: a slow backup task is a perf
                        // outlier, not a failure. Failure is already conveyed by
                        // the Status column. Two tiers — normal / slow — keeps
                        // the column calm instead of screaming red on every
                        // long-running VM backup.
                        const durationToneClass = () => {
                          const baseline = taskDurationBaselineSeconds();
                          if (!durationSec || baseline <= 0)
                            return 'bg-slate-500/30 dark:bg-slate-500/30';
                          const ratio = durationSec / baseline;
                          if (ratio >= 1.5) return 'bg-metric-warning-bg dark:bg-metric-warning-bg';
                          return 'bg-metric-normal-bg dark:bg-metric-normal-bg';
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
                            <Show when={showTaskSizeColumn()}>
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
                            </Show>
                            <Show when={showTaskErrorColumn()}>
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
                            </Show>
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
