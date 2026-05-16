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
import type {
  BackupTask,
  GuestSnapshot,
  PVEBackupsPayload,
  PVEBackupsResponse,
  StorageBackup,
} from '@/types/api';

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

const ARCHIVE_STATUS_FILTERS: FilterOption<'all' | 'protected' | 'verified' | 'unverified'>[] = [
  { value: 'all', label: 'All' },
  { value: 'protected', label: 'Protected' },
  { value: 'verified', label: 'Verified' },
  { value: 'unverified', label: 'Unverified' },
];

const TASK_STATUS_FILTERS: FilterOption<'all' | 'ok' | 'failed' | 'running'>[] = [
  { value: 'all', label: 'All' },
  { value: 'ok', label: 'OK' },
  { value: 'failed', label: 'Failed' },
  { value: 'running', label: 'Running' },
];

export const ProxmoxBackupsTable: Component<{
  emptyIcon: JSX.Element;
}> = (props) => {
  const [backups, { refetch }] = createResource<PVEBackupsPayload>(fetchPVEBackups);
  const [tab, setTab] = createSignal<BackupTabId>('snapshots');
  const [search, setSearch] = createSignal('');
  const [archiveFilter, setArchiveFilter] = createSignal<'all' | 'protected' | 'verified' | 'unverified'>('all');
  const [taskFilter, setTaskFilter] = createSignal<'all' | 'ok' | 'failed' | 'running'>('all');

  const snapshots = createMemo<GuestSnapshot[]>(() => backups()?.guestSnapshots ?? []);
  const archives = createMemo<StorageBackup[]>(() => backups()?.storageBackups ?? []);
  const tasks = createMemo<BackupTask[]>(() => backups()?.backupTasks ?? []);

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
    return archives().filter((arc) => {
      if (filter === 'protected' && !arc.protected) return false;
      if (filter === 'verified' && !arc.verified) return false;
      if (filter === 'unverified' && arc.verified) return false;
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
    return tasks().filter((task) => {
      const classify = classifyTaskStatus(task.status);
      if (filter === 'ok' && classify.variant !== 'success') return false;
      if (filter === 'failed' && classify.variant !== 'danger') return false;
      if (filter === 'running' && classify.variant !== 'warning') return false;
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
                options={ARCHIVE_STATUS_FILTERS}
                value={archiveFilter()}
                onChange={setArchiveFilter}
              />
            </Show>
            <Show when={tab() === 'tasks'}>
              <FilterButtonGroup
                options={TASK_STATUS_FILTERS}
                value={taskFilter()}
                onChange={setTaskFilter}
              />
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
              <Card padding="none" tone="card" class="overflow-hidden">
                <Table class="w-full min-w-[900px] border-collapse text-xs">
                  <TableHeader class="bg-surface-alt text-muted border-b border-border">
                    <TableRow class="text-left text-[10px] uppercase tracking-wide">
                      <TableHead class="px-3 py-2 font-medium">Name</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Guest</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Node</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Parent</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Captured</TableHead>
                      <TableHead class="px-3 py-2 font-medium text-right">Size</TableHead>
                      <TableHead class="px-3 py-2 font-medium">RAM</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody class="divide-y divide-border-subtle">
                    <For each={filteredSnapshots()}>
                      {(snap) => (
                        <TableRow class="hover:bg-surface-hover">
                          <TableCell class="px-3 py-2 text-base-content">
                            <div class="font-semibold">{snap.name || '—'}</div>
                            <Show when={!!snap.description?.trim()}>
                              <div class="text-[11px] text-muted truncate max-w-[20rem]" title={snap.description}>
                                {snap.description}
                              </div>
                            </Show>
                          </TableCell>
                          <TableCell class="px-3 py-2 text-base-content">{guestLabel(snap.type, snap.vmid)}</TableCell>
                          <TableCell class="px-3 py-2 text-base-content font-mono text-[11px]">
                            {snap.node || '—'}
                          </TableCell>
                          <TableCell class="px-3 py-2 text-base-content font-mono text-[11px]">
                            {snap.parent?.trim() || '—'}
                          </TableCell>
                          <TableCell class="px-3 py-2 text-base-content">
                            {formatRelativeTime(snap.time, { compact: true })}
                          </TableCell>
                          <TableCell class="px-3 py-2 text-right text-base-content tabular-nums">
                            <Show when={snap.sizeBytes && snap.sizeBytes > 0} fallback={<span class="text-muted">—</span>}>
                              {formatBytes(snap.sizeBytes ?? 0)}
                            </Show>
                          </TableCell>
                          <TableCell class="px-3 py-2 text-base-content">
                            <Show
                              when={snap.vmstate}
                              fallback={<span class="text-muted">—</span>}
                            >
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
              </Card>
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
              <Card padding="none" tone="card" class="overflow-hidden">
                <Table class="w-full min-w-[1050px] border-collapse text-xs">
                  <TableHeader class="bg-surface-alt text-muted border-b border-border">
                    <TableRow class="text-left text-[10px] uppercase tracking-wide">
                      <TableHead class="px-3 py-2 font-medium">Volume</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Guest</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Storage</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Node</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Format</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Created</TableHead>
                      <TableHead class="px-3 py-2 font-medium text-right">Size</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Protection</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Verified</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody class="divide-y divide-border-subtle">
                    <For each={filteredArchives()}>
                      {(arc) => (
                        <TableRow class="hover:bg-surface-hover">
                          <TableCell class="px-3 py-2 text-base-content font-mono text-[11px]">
                            <span class="inline-block max-w-[18rem] truncate" title={arc.volid}>
                              {arc.volid}
                            </span>
                          </TableCell>
                          <TableCell class="px-3 py-2 text-base-content">
                            {guestLabel(arc.type, arc.vmid)}
                          </TableCell>
                          <TableCell class="px-3 py-2 text-base-content">{arc.storage || '—'}</TableCell>
                          <TableCell class="px-3 py-2 text-base-content font-mono text-[11px]">
                            {arc.node || '—'}
                          </TableCell>
                          <TableCell class="px-3 py-2 text-base-content uppercase text-[10px]">
                            {arc.format || '—'}
                          </TableCell>
                          <TableCell class="px-3 py-2 text-base-content">
                            {formatRelativeTime(arc.time, { compact: true })}
                          </TableCell>
                          <TableCell class="px-3 py-2 text-right text-base-content tabular-nums">
                            {formatBytes(arc.size)}
                          </TableCell>
                          <TableCell class="px-3 py-2 text-base-content">
                            <Show
                              when={arc.protected}
                              fallback={<span class="text-muted">—</span>}
                            >
                              <span class="inline-flex items-center rounded-sm bg-amber-100 px-1.5 py-0.5 text-[10px] font-semibold text-amber-700 dark:bg-amber-900/40 dark:text-amber-200">
                                Protected
                              </span>
                            </Show>
                          </TableCell>
                          <TableCell class="px-3 py-2 text-base-content">
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
              </Card>
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
                        ? TABS[2].emptyTitle
                        : 'No tasks match current filters'
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
              <Card padding="none" tone="card" class="overflow-hidden">
                <Table class="w-full min-w-[1000px] border-collapse text-xs">
                  <TableHeader class="bg-surface-alt text-muted border-b border-border">
                    <TableRow class="text-left text-[10px] uppercase tracking-wide">
                      <TableHead class="px-3 py-2 font-medium">Status</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Guest</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Node</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Started</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Duration</TableHead>
                      <TableHead class="px-3 py-2 font-medium text-right">Size</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Error</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody class="divide-y divide-border-subtle">
                    <For each={filteredTasks()}>
                      {(task) => {
                        const classify = classifyTaskStatus(task.status);
                        return (
                          <TableRow class="hover:bg-surface-hover">
                            <TableCell class="px-3 py-2">
                              <div class="flex items-center gap-2">
                                <StatusDot size="sm" variant={classify.variant} title={classify.label} ariaHidden />
                                <span class="text-[11px] font-medium text-base-content">{classify.label}</span>
                              </div>
                            </TableCell>
                            <TableCell class="px-3 py-2 text-base-content">
                              {guestLabel(task.type, task.vmid)}
                            </TableCell>
                            <TableCell class="px-3 py-2 text-base-content font-mono text-[11px]">
                              {task.node || '—'}
                            </TableCell>
                            <TableCell class="px-3 py-2 text-base-content">
                              {formatRelativeTime(task.startTime, { compact: true })}
                            </TableCell>
                            <TableCell class="px-3 py-2 text-base-content">
                              {formatDuration(task.startTime, task.endTime)}
                            </TableCell>
                            <TableCell class="px-3 py-2 text-right text-base-content tabular-nums">
                              <Show when={task.size && task.size > 0} fallback={<span class="text-muted">—</span>}>
                                {formatBytes(task.size ?? 0)}
                              </Show>
                            </TableCell>
                            <TableCell class="px-3 py-2 text-base-content">
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
              </Card>
            </Show>
          </Show>
        </div>
      </Show>
    </Show>
  );
};

export default ProxmoxBackupsTable;
