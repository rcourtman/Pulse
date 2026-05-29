import type { GuestSnapshot, PBSBackup } from '@/types/api';
import type { StatusIndicatorVariant } from '@/utils/status';

// Pure model logic for ProxmoxBackupsTable: tab identifiers, per-tab sort-key
// types and their default directions, sort comparators, and workload/label/
// duration formatting. Kept JSX-free so it can be unit-tested in isolation and
// shared by the table's sub-view components. The reactive memos, filter
// toolbars, and rendering stay in ProxmoxBackupsTable.tsx.

export type BackupTabId = 'coverage' | 'recoverable' | 'sources' | 'tasks';
export type SourceDetailTabId = 'pbs' | 'snapshots' | 'archives';

export type CoverageFilterValue = 'all' | 'attention' | 'current' | 'uncovered';
export type RecoverableFilterValue =
  | 'all'
  | 'pbs'
  | 'archive'
  | 'snapshot'
  | 'verified'
  | 'unverified';
export type SnapshotFilterValue = 'all' | 'recent' | 'stale' | 'with-ram';

// One guest's aggregated snapshot inventory, the row shape for the Snapshots
// source-detail table (which expands to list the guest's individual snapshots).
export interface SnapshotGuestRow {
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

export function classifyTaskStatus(status: string): {
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

export function guestLabel(type: string | undefined, vmid: number): string {
  const t = (type ?? '').toLowerCase();
  const kind = t === 'ct' ? 'CT' : t === 'vm' ? 'VM' : t.toUpperCase() || 'Guest';
  return `${kind} ${vmid}`;
}

export function pbsWorkloadLabel(backup: PBSBackup): string {
  const t = (backup.backupType ?? '').toLowerCase();
  if (t === 'ct') return `CT ${backup.vmid}`;
  if (t === 'vm') return `VM ${backup.vmid}`;
  if (t === 'host') return backup.vmid ? `Host ${backup.vmid}` : 'Host';
  const kind = backup.backupType?.trim().toUpperCase() || 'Backup';
  return backup.vmid ? `${kind} ${backup.vmid}` : kind;
}

export function pbsRepositoryLabel(backup: PBSBackup): string {
  const namespace = backup.namespace?.trim() || '(root)';
  return `${backup.datastore || '—'} / ${namespace}`;
}

export function formatDuration(start: string | undefined, end: string | undefined): string {
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

export function formatDurationFromSeconds(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return '—';
  const rounded = Math.round(seconds);
  if (rounded < 60) return `${rounded}s`;
  if (rounded < 3600) return `${Math.floor(rounded / 60)}m ${rounded % 60}s`;
  const h = Math.floor(rounded / 3600);
  const m = Math.floor((rounded % 3600) / 60);
  return `${h}h ${m}m`;
}

// Case-insensitive string comparator. Undefined / empty values ALWAYS sort
// to the end regardless of direction — flipping a "—" row to the top of a
// desc sort is never what the user wants (e.g. a running task with no
// finished duration should not headline a "slowest tasks" view).
export function cmpString(
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

export function cmpNumber(
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

export function cmpBool(a: boolean, b: boolean, direction: 'asc' | 'desc'): number {
  const cmp = (a ? 1 : 0) - (b ? 1 : 0);
  return direction === 'asc' ? cmp : -cmp;
}

// Per-tab sort key types and their default directions. Numeric / timestamp
// columns default to `desc` (biggest / newest first) since that's the
// sysadmin-default question on a backup page. String columns default to
// `asc` (A→Z). Boolean columns default to `desc` so "true" sorts first.

export type CoverageSortKey =
  | 'posture'
  | 'workload'
  | 'latest'
  | 'pbs'
  | 'archive'
  | 'snapshot'
  | 'task';
export const COVERAGE_SORT_DEFAULT_DIRECTION: Record<CoverageSortKey, 'asc' | 'desc'> = {
  posture: 'asc',
  workload: 'asc',
  latest: 'desc',
  pbs: 'desc',
  archive: 'desc',
  snapshot: 'desc',
  task: 'desc',
};

export type RecoverableSortKey = 'workload' | 'source' | 'location' | 'created' | 'size' | 'state';
export const RECOVERABLE_SORT_DEFAULT_DIRECTION: Record<RecoverableSortKey, 'asc' | 'desc'> = {
  workload: 'asc',
  source: 'asc',
  location: 'asc',
  created: 'desc',
  size: 'desc',
  state: 'asc',
};

export type PBSSortKey = 'workload' | 'repository' | 'created' | 'size' | 'protected' | 'verified';
export const PBS_SORT_DEFAULT_DIRECTION: Record<PBSSortKey, 'asc' | 'desc'> = {
  workload: 'asc',
  repository: 'asc',
  created: 'desc',
  size: 'desc',
  protected: 'desc',
  verified: 'desc',
};

export type SnapshotSortKey = 'guest' | 'node' | 'latest' | 'count' | 'size';
export const SNAPSHOT_SORT_DEFAULT_DIRECTION: Record<SnapshotSortKey, 'asc' | 'desc'> = {
  guest: 'asc',
  node: 'asc',
  latest: 'desc',
  count: 'desc',
  size: 'desc',
};

export type ArchiveSortKey =
  | 'volume'
  | 'guest'
  | 'storage'
  | 'node'
  | 'format'
  | 'created'
  | 'size'
  | 'protected'
  | 'verified';
export const ARCHIVE_SORT_DEFAULT_DIRECTION: Record<ArchiveSortKey, 'asc' | 'desc'> = {
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

export type TaskSortKey = 'status' | 'guest' | 'node' | 'started' | 'duration' | 'size';
export const TASK_SORT_DEFAULT_DIRECTION: Record<TaskSortKey, 'asc' | 'desc'> = {
  status: 'asc',
  guest: 'asc',
  node: 'asc',
  started: 'desc',
  duration: 'desc',
  size: 'desc',
};
