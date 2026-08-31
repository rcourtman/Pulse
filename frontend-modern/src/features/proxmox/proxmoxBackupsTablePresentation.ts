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
  | 'workload'
  | 'type'
  | 'targetId'
  | 'node'
  | 'posture'
  | 'latest'
  | 'pbs'
  | 'archive'
  | 'snapshot'
  | 'task';

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
  // the table gains enough room to keep those headline answers readable.
  compact: ['server', 'status', 'datastore', 'used', 'backups'],
  basic: ['server', 'status', 'datastore', 'used', 'backups'],
  operational: ['server', 'status', 'cpu', 'memory', 'datastore', 'used', 'backups'],
  expanded: ['server', 'status', 'cpu', 'memory', 'uptime', 'datastore', 'used', 'backups'],
  full: BACKUP_SERVER_COLUMNS.map((column) => column.id),
};

const BACKUP_SERVER_WEIGHTS: Record<
  ProxmoxBackupsTableLayoutMode,
  Partial<Record<BackupServerColumnId, number>>
> = {
  compact: { server: 40, status: 15, datastore: 15, used: 20, backups: 10 },
  basic: { server: 24, status: 14, datastore: 20, used: 28, backups: 14 },
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
  full: {
    server: 15,
    status: 10,
    version: 8,
    cpu: 6,
    memory: 11,
    uptime: 7,
    datastore: 13,
    used: 15,
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
  { id: 'latest', label: 'Restore', kind: 'numeric-value' },
  { id: 'pbs', label: 'PBS snapshot', kind: 'text' },
  { id: 'archive', label: 'PVE file', kind: 'text' },
  { id: 'snapshot', label: 'Guest snapshot', kind: 'text' },
  { id: 'task', label: 'Task', kind: 'text' },
];

const COVERAGE_VISIBLE: Record<ProxmoxBackupsTableLayoutMode, readonly CoverageColumnId[]> = {
  // On narrow surfaces answer: which workload, what posture, how recent is the
  // newest restore point, and did the latest task succeed? Provider-by-provider
  // evidence is progressive detail in the expansion row; target identity folds
  // beneath the name so the scan stays legible at 320px.
  compact: ['workload', 'posture', 'latest', 'pbs', 'task'],
  basic: ['workload', 'node', 'posture', 'latest', 'task'],
  operational: ['workload', 'type', 'node', 'posture', 'latest', 'task'],
  expanded: ['workload', 'type', 'node', 'posture', 'latest', 'pbs', 'archive', 'snapshot', 'task'],
  full: COVERAGE_COLUMNS.map((column) => column.id),
};

const COVERAGE_WEIGHTS: Record<
  ProxmoxBackupsTableLayoutMode,
  Partial<Record<CoverageColumnId, number>>
> = {
  compact: { workload: 40, posture: 18, latest: 16, pbs: 16, task: 10 },
  basic: { workload: 31, node: 18, posture: 19, latest: 20, task: 12 },
  operational: { workload: 28, type: 9, node: 14, posture: 17, latest: 17, task: 15 },
  expanded: {
    workload: 17,
    type: 7,
    node: 10,
    posture: 13,
    latest: 12,
    pbs: 11,
    archive: 10.5,
    snapshot: 11.5,
    task: 8,
  },
  full: {
    workload: 14.5,
    type: 6,
    targetId: 8.5,
    node: 9,
    posture: 11,
    latest: 10,
    pbs: 11.5,
    archive: 10,
    snapshot: 12.5,
    task: 8,
  },
};

export interface CoverageSourceVisibility {
  pbs: boolean;
  archive: boolean;
  snapshot: boolean;
  task: boolean;
}

export const getCoverageColumns = (
  layout: ProxmoxBackupsTableLayoutMode,
  sourceVisibility: CoverageSourceVisibility,
): BackupTableColumn<CoverageColumnId>[] => {
  const layoutColumns = new Set(COVERAGE_VISIBLE[layout]);
  return COVERAGE_COLUMNS.filter((column) => {
    if (!layoutColumns.has(column.id)) return false;
    if (column.id === 'pbs') return sourceVisibility.pbs;
    if (column.id === 'archive') return sourceVisibility.archive;
    if (column.id === 'snapshot') return sourceVisibility.snapshot;
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
  compact: ['workload', 'source', 'location', 'created', 'state'],
  basic: ['workload', 'source', 'location', 'created', 'state'],
  operational: ['workload', 'type', 'source', 'location', 'created', 'size', 'state'],
  expanded: ['workload', 'type', 'targetId', 'source', 'location', 'created', 'size', 'state'],
  full: RECOVERABLE_COLUMNS.map((column) => column.id),
};

const RECOVERABLE_WEIGHTS: Record<
  ProxmoxBackupsTableLayoutMode,
  Partial<Record<RecoverableColumnId, number>>
> = {
  // Reserve enough phone-width space for complete state badges (including
  // Failed and Running) rather than clipping the recovery answer at the
  // scroll edge. Identity remains on the canonical 40% anchor.
  compact: { workload: 40, source: 12, location: 17, created: 15, state: 16 },
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
