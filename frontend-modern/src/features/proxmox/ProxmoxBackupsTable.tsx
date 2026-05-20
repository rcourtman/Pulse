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
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { FilterButtonGroup, type FilterOption } from '@/components/shared/FilterButtonGroup';
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
  type BackupActivityRangeDays,
  type BackupActivitySegmentKind,
} from './proxmoxBackupActivityPresentation';

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

function classifyTaskStatus(status: string): {
  variant: StatusIndicatorVariant;
  label: string;
} {
  const normalized = (status ?? '').toLowerCase();
  if (normalized === 'ok' || normalized === 'success' || normalized === 'completed') {
    return { variant: 'success', label: 'OK' };
  }
  if (normalized === 'running') {
    return { variant: 'warning', label: 'Running' };
  }
  if (normalized === 'failed' || normalized === 'error') {
    return { variant: 'danger', label: 'Failed' };
  }
  if (!normalized) return { variant: 'muted', label: '—' };
  return { variant: 'muted', label: status };
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
  const [chartRange, setChartRange] = createSignal<BackupActivityRangeDays>(30);
  const [selectedDateKey, setSelectedDateKey] = createSignal<string | null>(null);
  const toggleDay = (key: string) =>
    setSelectedDateKey((current) => (current === key ? null : key));

  const snapshots = createMemo<GuestSnapshot[]>(() => backups()?.guestSnapshots ?? []);
  const archives = createMemo<StorageBackup[]>(() => backups()?.storageBackups ?? []);
  const tasks = createMemo<BackupTask[]>(() => backups()?.backupTasks ?? []);

  const archiveTimestampMs = (arc: StorageBackup): number | undefined => {
    const ms = Date.parse(arc.time ?? '');
    return Number.isFinite(ms) ? ms : undefined;
  };
  const taskTimestampMs = (task: BackupTask): number | undefined => {
    const ms = Date.parse(task.startTime ?? '');
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

  const ARCHIVE_SEGMENT_KINDS: readonly BackupActivitySegmentKind[] = ['archive'];
  const TASK_SEGMENT_KINDS: readonly BackupActivitySegmentKind[] = ['ok', 'failed', 'running'];

  const filteredSnapshots = createMemo(() => {
    const term = search().trim().toLowerCase();
    return snapshots().filter((snap) => {
      if (!term) return true;
      return [snap.name, snap.node, snap.instance, snap.description, String(snap.vmid)]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(term);
    });
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

          <Show when={tab() === 'archives'}>
            <BackupActivityChart
              title="Backup files per day"
              noun="archive"
              segmentKinds={ARCHIVE_SEGMENT_KINDS}
              range={chartRange}
              onRangeChange={setChartRange}
              timeline={archiveTimeline}
              selectedDateKey={selectedDateKey}
              onToggleDay={toggleDay}
            />
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
            <Show when={selectedDateKey() !== null && tab() !== 'snapshots'}>
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
                when={visibleForTab() !== totalForTab()}
                fallback={<>{totalForTab()} entries</>}
              >
                {visibleForTab()} of {totalForTab()} entries
              </Show>
            </span>
          </div>

          <Show when={tab() === 'snapshots'}>
            <Show
              when={filteredSnapshots().length > 0}
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
                        : 'Adjust the search to see more snapshots.'
                    }
                  />
                </Card>
              }
            >
              <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
                <Table class="min-w-[900px] text-xs">
                  <TableHeader>
                    <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                      <TableHead class={getPlatformTableHeadClassForKind('name')}>Name</TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('text')}>Guest</TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('text')}>Node</TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('text')}>Parent</TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                        Captured
                      </TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                        Size
                      </TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('text')}>RAM</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                    <For each={filteredSnapshots()}>
                      {(snap) => (
                        <TableRow class="hover:bg-surface-hover">
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('name')} text-base-content`}
                          >
                            <div class="font-semibold">{snap.name || '—'}</div>
                            <Show when={!!snap.description?.trim()}>
                              <div
                                class="text-[11px] text-muted truncate max-w-[20rem]"
                                title={snap.description}
                              >
                                {snap.description}
                              </div>
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            {guestLabel(snap.type, snap.vmid)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content font-mono text-[11px]`}
                          >
                            {snap.node || '—'}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content font-mono text-[11px]`}
                          >
                            {snap.parent?.trim() || '—'}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {formatRelativeTime(snap.time, { compact: true })}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                          >
                            <Show
                              when={snap.sizeBytes && snap.sizeBytes > 0}
                              fallback={<span class="text-muted">—</span>}
                            >
                              {formatBytes(snap.sizeBytes ?? 0)}
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <Show when={snap.vmstate} fallback={<span class="text-muted">—</span>}>
                              <span class="inline-flex items-center rounded-sm bg-blue-100 px-1.5 py-0.5 text-[10px] font-semibold text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                                with RAM
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
                      <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
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
                            {formatRelativeTime(arc.time, { compact: true })}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                          >
                            {formatBytes(arc.size)}
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
                              <span class="inline-flex items-center gap-1 text-emerald-700 dark:text-emerald-300">
                                <StatusDot size="sm" variant="success" ariaHidden />
                                <span class="text-[11px] font-medium">Verified</span>
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
                      <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
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
                                <span class="text-[11px] font-medium text-base-content">
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
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                            >
                              {formatDuration(task.startTime, task.endTime)}
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
