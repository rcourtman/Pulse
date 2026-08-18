import { For, Show, createMemo, createSignal, type Component, type JSX } from 'solid-js';
import ArrowRightIcon from 'lucide-solid/icons/arrow-right';
import { filterChipStatusDot } from '@/components/shared/FilterBar';
import { StatusDot } from '@/components/shared/StatusDot';
import type { StatusIndicatorVariant } from '@/utils/status';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { apiFetch } from '@/utils/apiClient';
import {
  PlatformTableToolbar,
  PlatformErrorState,
  PlatformTableDurationValue,
  PlatformTableRelativeTimeValue,
  getPlatformTableCellClassForKind,
  getPlatformTableContainerLayout,
  getPlatformTableHeadClassForKind,
  type PlatformTableContainerLayout,
  type PlatformTableFilterOption,
  PlatformTableEmptyState,
  PlatformTableLoadingState,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import { useObservedElementWidth } from '@/hooks/useObservedElementWidth';
import type { ReplicationJob, ReplicationJobsResponse } from '@/types/api';

// Replication is a Proxmox-specific concept (zfs send/receive scheduled
// between PVE nodes), so this table is bespoke rather than a filtered
// view of any generic resource list. It hits the dedicated
// /api/replication/jobs surface which projects the monitor's
// ReplicationJobsSnapshot without going through the unified-resource
// pipeline.

type ReplicationStatusFilter = 'all' | 'healthy' | 'failed' | 'pending' | 'disabled';

type ReplicationColumn =
  | 'status'
  | 'job'
  | 'guest'
  | 'route'
  | 'schedule'
  | 'lastSync'
  | 'nextSync'
  | 'duration'
  | 'fails'
  | 'error';

export const REPLICATION_PHONE_COLUMNS: readonly ReplicationColumn[] = [
  'guest',
  'status',
  'job',
  'route',
  'lastSync',
  'nextSync',
];

export const REPLICATION_PHONE_COLUMN_WIDTHS: Readonly<Partial<Record<ReplicationColumn, number>>> =
  {
    status: 15,
    job: 12,
    guest: 30,
    route: 18,
    lastSync: 13,
    nextSync: 12,
  };

const REPLICATION_COLUMN_WIDTH_CLASS: Record<
  PlatformTableContainerLayout,
  Record<ReplicationColumn, string>
> = {
  compact: {
    status: 'w-[15%]',
    job: 'w-[12%]',
    guest: 'w-[30%]',
    route: 'w-[18%]',
    schedule: 'w-0',
    lastSync: 'w-[13%]',
    nextSync: 'w-[12%]',
    duration: 'w-0',
    fails: 'w-0',
    error: 'w-0',
  },
  basic: {
    status: 'w-[14%]',
    job: 'w-[11%]',
    guest: 'w-[28%]',
    route: 'w-[17%]',
    schedule: 'w-0',
    lastSync: 'w-[15%]',
    nextSync: 'w-[15%]',
    duration: 'w-0',
    fails: 'w-0',
    error: 'w-0',
  },
  operational: {
    status: 'w-[11%]',
    job: 'w-[7%]',
    guest: 'w-[21%]',
    route: 'w-[12%]',
    schedule: 'w-[9%]',
    lastSync: 'w-[11%]',
    nextSync: 'w-[13%]',
    duration: 'w-[11%]',
    fails: 'w-[5%]',
    error: 'w-0',
  },
  expanded: {
    status: 'w-[10%]',
    job: 'w-[7%]',
    guest: 'w-[20%]',
    route: 'w-[11%]',
    schedule: 'w-[8%]',
    lastSync: 'w-[10%]',
    nextSync: 'w-[12%]',
    duration: 'w-[10%]',
    fails: 'w-[4%]',
    error: 'w-[8%]',
  },
  full: {
    status: 'w-[10%]',
    job: 'w-[7%]',
    guest: 'w-[20%]',
    route: 'w-[11%]',
    schedule: 'w-[8%]',
    lastSync: 'w-[10%]',
    nextSync: 'w-[12%]',
    duration: 'w-[10%]',
    fails: 'w-[4%]',
    error: 'w-[8%]',
  },
};

const STATUS_FILTER_OPTIONS: PlatformTableFilterOption<ReplicationStatusFilter>[] = [
  { value: 'all', label: 'All' },
  {
    value: 'healthy',
    label: 'Healthy',
    tone: 'success',
    leading: filterChipStatusDot('bg-emerald-500'),
  },
  { value: 'failed', label: 'Failed', tone: 'danger', leading: filterChipStatusDot('bg-red-500') },
  {
    value: 'pending',
    label: 'Pending',
    tone: 'warning',
    leading: filterChipStatusDot('bg-amber-500'),
  },
  {
    value: 'disabled',
    label: 'Disabled',
    tone: 'muted',
    leading: filterChipStatusDot('bg-slate-400'),
  },
];

interface ReplicationStatusIndicator {
  variant: StatusIndicatorVariant;
  label: string;
  tone: string;
}

function classifyJob(job: ReplicationJob): ReplicationStatusFilter {
  if (!job.enabled) return 'disabled';
  if ((job.failCount ?? 0) > 0) return 'failed';
  const last = (job.lastSyncStatus ?? '').toLowerCase();
  if (last === 'ok' || last === 'success' || last === 'completed') return 'healthy';
  if (last === 'failed' || last === 'error') return 'failed';
  return 'pending';
}

function indicatorFor(classification: ReplicationStatusFilter): ReplicationStatusIndicator {
  switch (classification) {
    case 'healthy':
      return {
        variant: 'success',
        label: 'Healthy',
        tone: 'text-emerald-600 dark:text-emerald-300',
      };
    case 'failed':
      return { variant: 'danger', label: 'Failed', tone: 'text-red-600 dark:text-red-300' };
    case 'pending':
      return { variant: 'warning', label: 'Pending', tone: 'text-amber-600 dark:text-amber-300' };
    case 'disabled':
      return { variant: 'muted', label: 'Disabled', tone: 'text-muted' };
    default:
      return { variant: 'muted', label: '—', tone: 'text-muted' };
  }
}

function formatGuestLabel(job: ReplicationJob): string {
  const guestId = job.guestId ?? 0;
  const name = (job.guestName ?? '').trim();
  if (guestId && name) return `${guestId} (${name})`;
  if (guestId) return String(guestId);
  if (name) return name;
  if (job.guest?.trim()) return job.guest.trim();
  return '—';
}

function syncTimeValue(job: ReplicationJob): number | string | undefined {
  if (job.lastSyncUnix && job.lastSyncUnix > 0) {
    return job.lastSyncUnix * 1000;
  }
  return job.lastSyncTime;
}

type NextSyncTone = 'overdue' | 'imminent' | 'normal' | 'muted';

const NEXT_SYNC_TONE_CLASS: Record<NextSyncTone, string> = {
  overdue: 'text-red-600 dark:text-red-300 font-semibold',
  imminent: 'text-amber-600 dark:text-amber-300',
  normal: '',
  muted: 'text-muted',
};

// An overdue next-sync is the one signal that catches a stalled pvesr
// scheduler even while the last sync still reports ok, so it gets its own
// column instead of folding into the status pill (which mirrors PVE's own
// job state).
function nextSyncFor(job: ReplicationJob): { text: string; tone: NextSyncTone } {
  if (!job.enabled) return { text: '—', tone: 'muted' };
  let target = 0;
  if (job.nextSyncUnix && job.nextSyncUnix > 0) {
    target = job.nextSyncUnix * 1000;
  } else if (job.nextSyncTime) {
    const raw = job.nextSyncTime;
    const parsed = typeof raw === 'number' ? (raw > 1e12 ? raw : raw * 1000) : Date.parse(raw);
    if (Number.isFinite(parsed)) target = parsed;
  }
  if (!target) return { text: '—', tone: 'muted' };
  const minutes = Math.floor((target - Date.now()) / 60_000);
  if (minutes < 0) {
    const overdue = Math.abs(minutes);
    const text = overdue < 60 ? `${overdue}m overdue` : `${Math.floor(overdue / 60)}h overdue`;
    return { text, tone: 'overdue' };
  }
  if (minutes < 60) return { text: `in ${minutes}m`, tone: minutes < 5 ? 'imminent' : 'normal' };
  return { text: `in ${Math.floor(minutes / 60)}h ${minutes % 60}m`, tone: 'normal' };
}

export async function fetchReplicationJobs(): Promise<ReplicationJob[]> {
  const response = await apiFetch('/api/replication/jobs?platform=proxmox-pve');
  if (!response.ok) {
    throw new Error(`Failed to load replication jobs (${response.status})`);
  }
  const payload = (await response.json()) as ReplicationJobsResponse;
  return Array.isArray(payload?.data) ? payload.data : [];
}

// The jobs resource lives in ProxmoxPageSurface (it also gates the
// Replication tab's visibility), so this table is purely presentational.
export const ProxmoxReplicationTable: Component<{
  jobs: ReplicationJob[] | undefined;
  error: unknown;
  onRetry: () => void;
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
}> = (props) => {
  const [search, setSearch] = createSignal('');
  const [status, setStatus] = createSignal<ReplicationStatusFilter>('all');

  const filtered = createMemo(() => {
    const term = search().trim().toLowerCase();
    const want = status();
    return (props.jobs ?? []).filter((job) => {
      if (want !== 'all' && classifyJob(job) !== want) return false;
      if (!term) return true;
      const haystack = [
        job.jobId,
        job.guest,
        job.guestName,
        job.guestId?.toString() ?? '',
        job.sourceNode,
        job.targetNode,
        job.instance,
        job.lastSyncStatus,
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return haystack.includes(term);
    });
  });

  const total = createMemo(() => (props.jobs ?? []).length);
  const visible = createMemo(() => filtered().length);
  const observedWidth = useObservedElementWidth();
  const layout = createMemo(() =>
    getPlatformTableContainerLayout(observedWidth.width() ?? 1920, [520, 720, 960, 1200]),
  );
  const showJob = createMemo(() => true);
  const showNext = createMemo(() => true);
  const showOperational = createMemo(() => ['operational', 'expanded', 'full'].includes(layout()));
  const showError = createMemo(() => ['expanded', 'full'].includes(layout()));
  const columnWidthClass = (column: ReplicationColumn) =>
    REPLICATION_COLUMN_WIDTH_CLASS[layout()][column];

  return (
    <Show
      when={!props.error}
      fallback={
        <PlatformErrorState
          title="Could not load replication jobs"
          description={(props.error as Error | undefined)?.message ?? 'Refresh to retry.'}
          onRefresh={() => props.onRetry()}
        />
      }
    >
      <Show
        when={props.jobs !== undefined}
        fallback={
          <PlatformTableLoadingState
            title="Loading replication jobs"
            description="Reading scheduled replication state from PVE."
          />
        }
      >
        <Show
          when={total() > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title={props.emptyTitle}
              description={props.emptyDescription}
            />
          }
        >
          <div
            ref={observedWidth.setElement}
            class="space-y-3"
            data-proxmox-replication-layout={layout()}
          >
            <PlatformTableToolbar
              search={search}
              onSearchChange={setSearch}
              searchPlaceholder="Search jobs, guests, nodes"
              status={status()}
              onStatusChange={setStatus}
              statusOptions={STATUS_FILTER_OPTIONS}
              visible={visible()}
              total={total()}
              rowNoun="jobs"
            />

            <Show
              when={filtered().length > 0}
              fallback={
                <PlatformTableEmptyState
                  icon={props.emptyIcon}
                  title="No replication jobs match current filters"
                  description="Adjust the search or status filter to see more jobs."
                />
              }
            >
              <PlatformTableShell
                tableClass="min-w-[0px] table-fixed text-xs"
                colgroup={
                  <Show when={layout() === 'compact'}>
                    <colgroup>
                      <For each={REPLICATION_PHONE_COLUMNS}>
                        {(column) => (
                          <col
                            style={{ width: `${REPLICATION_PHONE_COLUMN_WIDTHS[column]}%` }}
                            data-proxmox-replication-column={column}
                          />
                        )}
                      </For>
                    </colgroup>
                  </Show>
                }
                header={
                  <>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('name')} ${columnWidthClass('guest')}`}
                    >
                      Guest
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('text')} ${columnWidthClass('status')}`}
                    >
                      Status
                    </TableHead>
                    <Show when={showJob()}>
                      <TableHead
                        class={`${getPlatformTableHeadClassForKind('text')} ${columnWidthClass('job')}`}
                      >
                        Job
                      </TableHead>
                    </Show>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('text')} ${columnWidthClass('route')}`}
                      title="Source → target"
                    >
                      Route
                    </TableHead>
                    <Show when={showOperational()}>
                      <TableHead
                        class={`${getPlatformTableHeadClassForKind('text')} ${columnWidthClass('schedule')}`}
                      >
                        Schedule
                      </TableHead>
                    </Show>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('numeric-value')} ${columnWidthClass('lastSync')}`}
                    >
                      {layout() === 'compact' ? 'Last' : 'Last sync'}
                    </TableHead>
                    <Show when={showNext()}>
                      <TableHead
                        class={`${getPlatformTableHeadClassForKind('numeric-value')} ${columnWidthClass('nextSync')}`}
                      >
                        {layout() === 'compact' ? 'Next' : 'Next sync'}
                      </TableHead>
                    </Show>
                    <Show when={showOperational()}>
                      <TableHead
                        class={`${getPlatformTableHeadClassForKind('numeric-value')} ${columnWidthClass('duration')}`}
                      >
                        Duration
                      </TableHead>
                      <TableHead
                        class={`${getPlatformTableHeadClassForKind('numeric-value')} ${columnWidthClass('fails')}`}
                      >
                        Fails
                      </TableHead>
                    </Show>
                    <Show when={showError()}>
                      <TableHead
                        class={`${getPlatformTableHeadClassForKind('text')} ${columnWidthClass('error')}`}
                      >
                        Error
                      </TableHead>
                    </Show>
                  </>
                }
                body={
                  <>
                    <For each={filtered()}>
                      {(job) => {
                        const classification = classifyJob(job);
                        const ind = indicatorFor(classification);
                        const next = nextSyncFor(job);
                        const sourceNode = (job.sourceNode ?? '').trim() || '—';
                        const targetNode = (job.targetNode ?? '').trim() || '—';
                        return (
                          <TableRow class="hover:bg-surface-hover">
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('name')} text-base-content`}
                            >
                              {formatGuestLabel(job)}
                            </TableCell>
                            <TableCell class={getPlatformTableCellClassForKind('text')}>
                              <div class="flex items-center gap-2">
                                <StatusDot
                                  size="sm"
                                  variant={ind.variant}
                                  title={ind.label}
                                  ariaHidden
                                />
                                <span class={`text-[11px] font-medium ${ind.tone}`}>
                                  {ind.label}
                                </span>
                              </div>
                            </TableCell>
                            <Show when={showJob()}>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('text')} text-base-content font-mono text-[11px]`}
                              >
                                <span title={job.id}>{job.jobId || job.id}</span>
                              </TableCell>
                            </Show>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                            >
                              <span class="inline-flex items-center gap-1 font-mono text-[11px]">
                                <span>{sourceNode}</span>
                                <ArrowRightIcon class="h-3 w-3 text-muted" aria-hidden="true" />
                                <span>{targetNode}</span>
                              </span>
                            </TableCell>
                            <Show when={showOperational()}>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('text')} text-base-content font-mono text-[11px]`}
                              >
                                {job.schedule || '—'}
                              </TableCell>
                            </Show>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                            >
                              <PlatformTableRelativeTimeValue value={syncTimeValue(job)} />
                            </TableCell>
                            <Show when={showNext()}>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                              >
                                <span class={NEXT_SYNC_TONE_CLASS[next.tone]}>{next.text}</span>
                              </TableCell>
                            </Show>
                            <Show when={showOperational()}>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                              >
                                <PlatformTableDurationValue
                                  seconds={job.lastSyncDurationSeconds}
                                  fallbackText={job.lastSyncDurationHuman}
                                />
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                              >
                                <Show
                                  when={(job.failCount ?? 0) > 0}
                                  fallback={<span class="text-muted">0</span>}
                                >
                                  <span class="text-red-600 dark:text-red-300 font-semibold">
                                    {job.failCount}
                                  </span>
                                </Show>
                              </TableCell>
                            </Show>
                            <Show when={showError()}>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                              >
                                <Show
                                  when={!!job.error?.trim()}
                                  fallback={<span class="text-muted">—</span>}
                                >
                                  <span
                                    class="inline-block max-w-[18rem] truncate text-red-600 dark:text-red-300"
                                    title={job.error ?? ''}
                                  >
                                    {job.error}
                                  </span>
                                </Show>
                              </TableCell>
                            </Show>
                          </TableRow>
                        );
                      }}
                    </For>
                  </>
                }
              />
            </Show>
          </div>
        </Show>
      </Show>
    </Show>
  );
};

export default ProxmoxReplicationTable;
