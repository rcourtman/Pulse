import type { JSX } from 'solid-js';

import type { PlatformTableColumnKind } from '@/features/platformPage/columnAlignment';
import {
  getPlatformTableWeightedColumnWidthStyle,
  PLATFORM_TABLE_NARROW_IDENTITY_WIDTH_PERCENT,
  PLATFORM_TABLE_PHONE_IDENTITY_WIDTH_PERCENT,
} from '@/features/platformPage/sharedPlatformPage';

export type ProxmoxBackupsTableLayoutMode =
  'compact' | 'basic' | 'operational' | 'expanded' | 'full';

export type BackupServerColumnId =
  | 'server'
  | 'status'
  | 'version'
  | 'cpu'
  | 'memory'
  | 'uptime'
  | 'datastore'
  | 'used'
  | 'backups'
  | 'dedup';

export type CoverageColumnId =
  'workload' | 'type' | 'targetId' | 'node' | 'posture' | 'latest' | 'task';

export type RecoverableColumnId =
  | 'workload'
  | 'type'
  | 'targetId'
  | 'source'
  | 'location'
  | 'created'
  | 'size'
  | 'state'
  | 'details';

export type BackupTableColumn<Id extends string> = {
  id: Id;
  label: string;
  kind: PlatformTableColumnKind;
};

const layoutForWidth = (
  width: number,
  breakpoints: readonly [number, number, number, number],
): ProxmoxBackupsTableLayoutMode => {
  if (!Number.isFinite(width) || width < breakpoints[0]) return 'compact';
  if (width < breakpoints[1]) return 'basic';
  if (width < breakpoints[2]) return 'operational';
  if (width < breakpoints[3]) return 'expanded';
  return 'full';
};

// These are table-container breakpoints, not viewport breakpoints. A 1536px
// browser with Pulse Assistant open leaves roughly 886px for the page and must
// select the same columns as any other 886px container.
export const getBackupServerLayoutForContainer = (width: number): ProxmoxBackupsTableLayoutMode =>
  layoutForWidth(width, [520, 720, 900, 1120]);

export const getCoverageLayoutForContainer = (width: number): ProxmoxBackupsTableLayoutMode =>
  layoutForWidth(width, [480, 720, 880, 1120]);

export const getRecoverableLayoutForContainer = (width: number): ProxmoxBackupsTableLayoutMode =>
  layoutForWidth(width, [480, 720, 880, 1120]);

export const BACKUP_SERVER_COLUMNS: readonly BackupTableColumn<BackupServerColumnId>[] = [
  { id: 'server', label: 'Backup server', kind: 'name' },
  { id: 'status', label: 'Status', kind: 'text' },
  { id: 'version', label: 'Version', kind: 'text' },
  { id: 'cpu', label: 'CPU', kind: 'numeric-value' },
  { id: 'memory', label: 'Memory', kind: 'numeric-value' },
  { id: 'uptime', label: 'Uptime', kind: 'numeric-value' },
  { id: 'datastore', label: 'Datastore', kind: 'text' },
  { id: 'used', label: 'Used', kind: 'numeric-value' },
  { id: 'backups', label: 'Backups', kind: 'numeric-value' },
  { id: 'dedup', label: 'Dedup', kind: 'numeric-value' },
];

const BACKUP_SERVER_VISIBLE: Record<
  ProxmoxBackupsTableLayoutMode,
  readonly BackupServerColumnId[]
> = {
  // Reachability and datastore exhaustion are the two risks that can stop all
  // future backups. Host telemetry and space-efficiency context return only as
  // the table gains enough room to keep those headline answers readable. The
  // phone projection keeps those two answers whole ("Healthy", "33.6%") and
  // lets the backup count return with the first wider layout.
  compact: ['server', 'status', 'datastore', 'used'],
  basic: ['server', 'status', 'datastore', 'used', 'backups'],
  operational: ['server', 'status', 'cpu', 'memory', 'datastore', 'used', 'backups'],
  expanded: ['server', 'status', 'cpu', 'memory', 'uptime', 'datastore', 'used', 'backups'],
  full: BACKUP_SERVER_COLUMNS.map((column) => column.id),
};

const BACKUP_SERVER_WEIGHTS: Record<
  ProxmoxBackupsTableLayoutMode,
  Partial<Record<BackupServerColumnId, number>>
> = {
  // Capacity must fit "Unavailable", not just a short percentage. Keep the
  // shared 40% identity anchor; the compact status omits its redundant dot.
  compact: { server: 40, status: 18, datastore: 17, used: 25 },
  // Just above the phone projection the used cell carries its used/total
  // pair again, the longest value in the row, so it takes the most room.
  basic: { server: 24, status: 13.6, datastore: 18.2, used: 28.4, backups: 9.8 },
  operational: {
    server: 20,
    status: 12,
    cpu: 10,
    memory: 12,
    datastore: 16,
    used: 20,
    backups: 10,
  },
  expanded: {
    server: 18,
    status: 11,
    cpu: 8,
    memory: 11,
    uptime: 9,
    datastore: 16,
    used: 18,
    backups: 9,
  },
  // Memory and Used carry a percentage plus a used/total pair, the longest
  // values in the row, so they take the slack from the one-word status and
  // version cells instead of truncating.
  full: {
    server: 15,
    status: 8,
    version: 7,
    cpu: 6,
    memory: 13.5,
    uptime: 7,
    datastore: 13,
    used: 15.5,
    backups: 8,
    dedup: 7,
  },
};

export const getBackupServerColumns = (
  layout: ProxmoxBackupsTableLayoutMode,
): BackupTableColumn<BackupServerColumnId>[] => {
  const visible = new Set(BACKUP_SERVER_VISIBLE[layout]);
  return BACKUP_SERVER_COLUMNS.filter((column) => visible.has(column.id));
};

export const getBackupServerColumnWidthStyle = (
  columnId: BackupServerColumnId,
  layout: ProxmoxBackupsTableLayoutMode,
): JSX.CSSProperties =>
  getPlatformTableWeightedColumnWidthStyle(
    columnId,
    BACKUP_SERVER_WEIGHTS[layout],
    BACKUP_SERVER_VISIBLE[layout],
    layout === 'compact' || layout === 'basic'
      ? {
          columnId: 'server',
          widthPercent:
            layout === 'compact'
              ? PLATFORM_TABLE_NARROW_IDENTITY_WIDTH_PERCENT
              : PLATFORM_TABLE_PHONE_IDENTITY_WIDTH_PERCENT,
        }
      : undefined,
  );

export const COVERAGE_COLUMNS: readonly BackupTableColumn<CoverageColumnId>[] = [
  { id: 'workload', label: 'Workload', kind: 'name' },
  { id: 'type', label: 'Type', kind: 'text' },
  { id: 'targetId', label: 'Target ID', kind: 'text' },
  { id: 'node', label: 'Node', kind: 'text' },
  { id: 'posture', label: 'Posture', kind: 'text' },
  { id: 'latest', label: 'Last backup', kind: 'numeric-value' },
  { id: 'task', label: 'Task', kind: 'text' },
];

const COVERAGE_VISIBLE: Record<ProxmoxBackupsTableLayoutMode, readonly CoverageColumnId[]> = {
  // Every layout answers: which workload, what posture, how recent is the
  // newest independent backup, and did the latest task succeed? The
  // provider-by-provider ages (PBS snapshot, PVE file, guest snapshot) are
  // progressive detail in the expansion row at every width: side by side they
  // made a stale backup look fresh beside a recent guest snapshot, which does
  // not count as a backup. Target identity folds beneath the name on phones.
  compact: ['workload', 'posture', 'latest', 'task'],
  basic: ['workload', 'node', 'posture', 'latest', 'task'],
  operational: ['workload', 'type', 'node', 'posture', 'latest', 'task'],
  expanded: ['workload', 'type', 'node', 'posture', 'latest', 'task'],
  full: ['workload', 'type', 'targetId', 'node', 'posture', 'latest', 'task'],
};

const COVERAGE_WEIGHTS: Record<
  ProxmoxBackupsTableLayoutMode,
  Partial<Record<CoverageColumnId, number>>
> = {
  // Posture keeps its whole compact word ("Unknown"), the age cell holds the
  // suffix-free compact age, and the job dot takes what a "Job" header needs.
  compact: { workload: 46, posture: 22, latest: 18, task: 14 },
  // Above the phone projection the identity columns (workload, node) take the
  // slack that the short type badge, target id, and age cells cannot use, so
  // names stay whole instead of truncating beside empty space.
  basic: { workload: 31, node: 18, posture: 22, latest: 15, task: 14 },
  operational: { workload: 27, type: 8, node: 17, posture: 17, latest: 15, task: 16 },
  expanded: { workload: 27, type: 8, node: 17, posture: 17, latest: 15, task: 16 },
  full: {
    workload: 24,
    type: 7,
    targetId: 9,
    node: 16,
    posture: 16,
    latest: 13,
    task: 15,
  },
};

export interface CoverageSourceVisibility {
  task: boolean;
}

export const getCoverageColumns = (
  layout: ProxmoxBackupsTableLayoutMode,
  sourceVisibility: CoverageSourceVisibility,
): BackupTableColumn<CoverageColumnId>[] => {
  const layoutColumns = new Set(COVERAGE_VISIBLE[layout]);
  return COVERAGE_COLUMNS.filter((column) => {
    if (!layoutColumns.has(column.id)) return false;
    if (column.id === 'task') return sourceVisibility.task;
    return true;
  });
};

export const getCoverageColumnWidthStyle = (
  columnId: CoverageColumnId,
  layout: ProxmoxBackupsTableLayoutMode,
  visibleColumnIds: readonly CoverageColumnId[],
): JSX.CSSProperties =>
  getPlatformTableWeightedColumnWidthStyle(
    columnId,
    COVERAGE_WEIGHTS[layout],
    visibleColumnIds,
    layout === 'compact' || layout === 'basic'
      ? {
          columnId: 'workload',
          widthPercent:
            layout === 'compact'
              ? PLATFORM_TABLE_NARROW_IDENTITY_WIDTH_PERCENT
              : PLATFORM_TABLE_PHONE_IDENTITY_WIDTH_PERCENT,
        }
      : undefined,
  );

export const RECOVERABLE_COLUMNS: readonly BackupTableColumn<RecoverableColumnId>[] = [
  { id: 'workload', label: 'Workload', kind: 'name' },
  { id: 'type', label: 'Type', kind: 'text' },
  { id: 'targetId', label: 'Target ID', kind: 'text' },
  { id: 'source', label: 'Source', kind: 'text' },
  { id: 'location', label: 'Location', kind: 'text' },
  { id: 'created', label: 'Created', kind: 'numeric-value' },
  { id: 'size', label: 'Size', kind: 'metric-bar' },
  { id: 'state', label: 'State', kind: 'text' },
  { id: 'details', label: 'Details', kind: 'text' },
];

const RECOVERABLE_VISIBLE: Record<ProxmoxBackupsTableLayoutMode, readonly RecoverableColumnId[]> = {
  // A recovery-point feed starts with identity, source, location, age, and
  // verification. Size and verbose details progressively return with usable
  // room; the full values remain available through their title affordance.
  // The phone projection keeps the source and state badges and the age whole;
  // the repository location returns with the first wider layout and stays in
  // the row's hover title meanwhile.
  compact: ['workload', 'source', 'created', 'state'],
  basic: ['workload', 'source', 'location', 'created', 'state'],
  operational: ['workload', 'type', 'source', 'location', 'created', 'size', 'state'],
  expanded: ['workload', 'type', 'targetId', 'source', 'location', 'created', 'size', 'state'],
  full: RECOVERABLE_COLUMNS.map((column) => column.id),
};

const RECOVERABLE_WEIGHTS: Record<
  ProxmoxBackupsTableLayoutMode,
  Partial<Record<RecoverableColumnId, number>>
> = {
  // Unknown is a safety answer, not an abbreviated age: reserve room for
  // the whole word alongside complete state badges (including Failed and
  // Running). Identity remains on the canonical 40% anchor.
  compact: { workload: 40, source: 16, created: 21, state: 23 },
  basic: { workload: 28, source: 14, location: 23, created: 18, state: 17 },
  operational: {
    workload: 24,
    type: 8,
    source: 13,
    location: 20,
    created: 14,
    size: 12,
    state: 9,
  },
  expanded: {
    workload: 18,
    type: 7,
    targetId: 10,
    source: 11,
    location: 18,
    created: 13,
    size: 13,
    state: 10,
  },
  full: {
    workload: 16.5,
    type: 6,
    targetId: 8.5,
    source: 10,
    location: 18,
    created: 11,
    size: 12,
    state: 8,
    details: 10,
  },
};

export const getRecoverableColumns = (
  layout: ProxmoxBackupsTableLayoutMode,
): BackupTableColumn<RecoverableColumnId>[] => {
  const visible = new Set(RECOVERABLE_VISIBLE[layout]);
  return RECOVERABLE_COLUMNS.filter((column) => visible.has(column.id));
};

export const getRecoverableColumnWidthStyle = (
  columnId: RecoverableColumnId,
  layout: ProxmoxBackupsTableLayoutMode,
): JSX.CSSProperties =>
  getPlatformTableWeightedColumnWidthStyle(
    columnId,
    RECOVERABLE_WEIGHTS[layout],
    RECOVERABLE_VISIBLE[layout],
    layout === 'compact' || layout === 'basic'
      ? {
          columnId: 'workload',
          widthPercent:
            layout === 'compact'
              ? PLATFORM_TABLE_NARROW_IDENTITY_WIDTH_PERCENT
              : PLATFORM_TABLE_PHONE_IDENTITY_WIDTH_PERCENT,
        }
      : undefined,
  );

export const isCompactBackupIdentityLayout = (layout: ProxmoxBackupsTableLayoutMode): boolean =>
  layout === 'compact' || layout === 'basic';

export const isCoverageEvidenceColumnVisible = (
  layout: ProxmoxBackupsTableLayoutMode,
  column: 'location' | 'size' | 'details',
): boolean => {
  if (column === 'location') return layout !== 'compact';
  if (column === 'size')
    return layout === 'operational' || layout === 'expanded' || layout === 'full';
  return layout === 'expanded' || layout === 'full';
};

// What the backup health counts mean, stated from the server's own posture
// policy so "17 attention" is never a number without a rule. The causes mirror
// internal/recovery/posture.go. Null until the policy has loaded; the strip
// then shows no rule rather than a guessed one.
export const getBackupPostureRuleText = (
  policy: { freshnessWindowSeconds: number } | null | undefined,
): string | null => {
  const seconds = policy?.freshnessWindowSeconds ?? 0;
  if (!Number.isFinite(seconds) || seconds <= 0) return null;
  return `Attention: the newest backup is older than ${formatPostureWindow(seconds)}, a newer backup job failed, expected verification is missing or overdue, or Pulse can see only part of the backup history. Unprotected: Pulse sees the full history and no backup it can count. Guest snapshots alone do not count. Unknown: Pulse cannot read the backup history or has no backup source linked to the guest.`;
};

const formatPostureWindow = (seconds: number): string => {
  if (seconds >= 86_400 && seconds % 86_400 === 0) {
    const days = seconds / 86_400;
    return days === 1 ? '1 day' : `${days} days`;
  }
  const hours = Math.max(1, Math.round(seconds / 3_600));
  return hours === 1 ? '1 hour' : `${hours} hours`;
};

const COVERAGE_RESTORE_EVIDENCE_LIMIT = 8;

// Newest restore points for a coverage row's expansion. The newest point and
// the newest completed point of every source always make the cut, so neither a
// burst of recent guest snapshots nor a running or failed job can hide the
// latest usable PBS or PVE backup now that the table carries no per-source
// age columns.
export const selectCoverageRestoreEvidence = <
  T extends {
    id: string;
    sourceKind: string;
    createdMs?: number;
    running?: boolean;
    failed?: boolean;
  },
>(
  artifacts: readonly T[],
  limit = COVERAGE_RESTORE_EVIDENCE_LIMIT,
): T[] => {
  const newestFirst = [...artifacts].sort(
    (left, right) => (right.createdMs ?? 0) - (left.createdMs ?? 0),
  );
  const picked = new Set<string>();
  const newestSeen = new Set<string>();
  const completedSeen = new Set<string>();
  for (const artifact of newestFirst) {
    if (!newestSeen.has(artifact.sourceKind)) {
      newestSeen.add(artifact.sourceKind);
      picked.add(artifact.id);
    }
    if (!artifact.running && !artifact.failed && !completedSeen.has(artifact.sourceKind)) {
      completedSeen.add(artifact.sourceKind);
      picked.add(artifact.id);
    }
  }
  const cap = Math.max(limit, picked.size);
  for (const artifact of newestFirst) {
    if (picked.size >= cap) break;
    picked.add(artifact.id);
  }
  return newestFirst.filter((artifact) => picked.has(artifact.id));
};
