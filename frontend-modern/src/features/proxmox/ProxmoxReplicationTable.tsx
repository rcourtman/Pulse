import { For, Show, createMemo, createSignal, type Component, type JSX } from 'solid-js';
import ArrowRightIcon from 'lucide-solid/icons/arrow-right';
import { filterChipStatusDot } from '@/components/shared/FilterBar';
import {
  InlineDetailPanel,
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailSection,
  type DetailValueTone,
} from '@/components/shared/DetailSectionTable';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { StatusDot } from '@/components/shared/StatusDot';
import type { StatusIndicatorVariant } from '@/utils/status';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { apiErrorFromResponse, apiFetch } from '@/utils/apiClient';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import {
  PlatformTableToolbar,
  PlatformErrorState,
  PlatformTableDurationValue,
  formatPlatformTableDurationValue,
  formatPlatformTableRelativeTimeValue,
  PlatformTableRelativeTimeValue,
  getPlatformTableCellClassForKind,
  getPlatformTableContainerLayout,
  getPlatformTableHeadClassForKind,
  getPlatformTableRowClass,
  type PlatformTableContainerLayout,
  type PlatformTableFilterOption,
  PlatformTableEmptyState,
  PlatformTableLoadingState,
  PlatformTableShell,
  PlatformWindowedRows,
  withPlatformStatusCounts,
} from '@/features/platformPage/sharedPlatformPage';
import {
  getPlatformResourceDetailRowInteractionProps,
  PlatformResourceDetailToggleButton,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import { RecordOwnerLink } from '@/features/platformPage/RecordOwnerLink';
import { useObservedElementWidth } from '@/hooks/useObservedElementWidth';
import type { ReplicationJob, ReplicationJobsResponse } from '@/types/api';
import { buildPlatformSearchSuggestions } from '@/features/platformPage/platformSearchSuggestions';
import { matchesSearchTermSplit, splitSearchExclusions } from '@/utils/searchQuery';
import { buildProxmoxGuestNativeId } from '@/utils/workloads';

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

export const REPLICATION_MOBILE_COLUMNS: readonly ReplicationColumn[] = [
  'guest',
  'status',
  'route',
  'lastSync',
  'nextSync',
];

export const REPLICATION_MOBILE_COLUMN_WIDTHS: Readonly<
  Partial<Record<ReplicationColumn, number>>
> = {
  guest: 40,
  status: 16,
  route: 21.5,
  lastSync: 9.5,
  nextSync: 13,
};

// The guest column carries the disclosure control beside the name, and the
// error text is why a failed job needs attention, so both take the share the
// timing columns can spare. An 8% error column clipped the message mid-word.
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
    status: 'w-[13%]',
    job: 'w-[9%]',
    guest: 'w-[31%]',
    route: 'w-[16%]',
    schedule: 'w-0',
    lastSync: 'w-[14%]',
    nextSync: 'w-[17%]',
    duration: 'w-0',
    fails: 'w-0',
    error: 'w-0',
  },
  operational: {
    status: 'w-[11%]',
    job: 'w-[8%]',
    guest: 'w-[25%]',
    route: 'w-[14%]',
    schedule: 'w-0',
    lastSync: 'w-[11%]',
    nextSync: 'w-[14%]',
    duration: 'w-[11%]',
    fails: 'w-[6%]',
    error: 'w-0',
  },
  expanded: {
    status: 'w-[9%]',
    job: 'w-[6%]',
    guest: 'w-[20%]',
    route: 'w-[11%]',
    schedule: 'w-[8%]',
    lastSync: 'w-[8%]',
    nextSync: 'w-[11%]',
    duration: 'w-[8%]',
    fails: 'w-[5%]',
    error: 'w-[14%]',
  },
  full: {
    status: 'w-[9%]',
    job: 'w-[7%]',
    guest: 'w-[20%]',
    route: 'w-[11%]',
    schedule: 'w-[8%]',
    lastSync: 'w-[9%]',
    nextSync: 'w-[11%]',
    duration: 'w-[8%]',
    fails: 'w-[4%]',
    error: 'w-[13%]',
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

export function formatMobileReplicationGuestLabel(job: ReplicationJob): string {
  const guestId = job.guestId ?? 0;
  const name = (job.guestName ?? '').trim();
  if (guestId && name) return `${guestId} ${name}`;
  return formatGuestLabel(job);
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
// job state). The countdown is measured from the shared relative-time clock:
// a stalled scheduler leaves nextSync unchanged, so a countdown read once at
// render would sit at "in 3m" and never turn overdue.
function nextSyncFor(job: ReplicationJob, now: number): { text: string; tone: NextSyncTone } {
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
  const minutes = Math.floor((target - now) / 60_000);
  if (minutes < 0) {
    const overdue = Math.abs(minutes);
    const text = overdue < 60 ? `${overdue}m overdue` : `${Math.floor(overdue / 60)}h overdue`;
    return { text, tone: 'overdue' };
  }
  if (minutes < 60) return { text: `in ${minutes}m`, tone: minutes < 5 ? 'imminent' : 'normal' };
  return { text: `in ${Math.floor(minutes / 60)}h ${minutes % 60}m`, tone: 'normal' };
}

export const compactReplicationNextSyncText = (text: string, tone: NextSyncTone): string => {
  if (tone === 'overdue') return `-${text.replace(/ overdue$/, '')}`;
  return text.replace(/^in /, '');
};

const NEXT_SYNC_DETAIL_TONE: Record<NextSyncTone, DetailValueTone | undefined> = {
  overdue: 'danger',
  imminent: 'warning',
  normal: undefined,
  muted: 'muted',
};

// The detail row id follows the job, not its place in the filtered list, so a
// refresh that reorders rows keeps the disclosure pointing at the same job.
const replicationJobDetailRowId = (job: ReplicationJob): string =>
  `replication-job-detail-${(job.id || job.jobId).replace(/[^A-Za-z0-9_-]/g, '-')}`;

// The guest the job copies, as the Proxmox workloads table knows it.
const replicationJobGuestSubject = (job: ReplicationJob) => {
  const id = buildProxmoxGuestNativeId({
    instance: job.instance,
    node: job.guestNode || job.sourceNode,
    vmid: job.guestId,
  });
  if (!id) return null;
  return {
    id,
    name: job.guestName?.trim() || job.guest?.trim() || String(job.guestId),
    type: (job.guestType ?? '').trim().toLowerCase() === 'lxc' ? 'lxc' : 'vm',
  };
};

const replicationJobSummary = (job: ReplicationJob, now: number): string => {
  const last = formatPlatformTableRelativeTimeValue(syncTimeValue(job), { now, emptyText: '' });
  const next = nextSyncFor(job, now);
  return [
    last ? `Last synced ${last}` : 'Not synced yet',
    next.tone === 'muted' ? '' : `next sync ${next.text}`,
  ]
    .filter(Boolean)
    .join(' · ');
};

const buildReplicationJobSections = (
  job: ReplicationJob,
  classification: ReplicationStatusFilter,
  guestLabel: string,
  now: number,
): DetailSection[] => {
  const errorText = (job.error ?? '').trim();
  const next = nextSyncFor(job, now);
  const failCount = job.failCount ?? 0;
  return compactDetailSections([
    // A failed job's error is the drawer summary; any other job's error
    // still leads, because it is why the job may fail next.
    classification === 'failed'
      ? null
      : {
          label: 'Error',
          rows: compactDetailRows([makeDetailRow('Error', errorText, { tone: 'danger' })]),
        },
    {
      label: 'Job',
      rows: compactDetailRows([
        makeDetailRow('Guest', guestLabel === '—' ? '' : guestLabel),
        makeDetailRow('Job ID', job.jobId || job.id),
        makeDetailRow('Failures', String(failCount), {
          tone: failCount > 0 ? 'danger' : undefined,
        }),
        makeDetailRow('Comment', job.comment),
      ]),
    },
    {
      label: 'Route',
      rows: compactDetailRows([
        makeDetailRow('Source', job.sourceNode),
        makeDetailRow('Target', job.targetNode),
        makeDetailRow('Source storage', job.sourceStorage),
        makeDetailRow('Target storage', job.targetStorage),
        makeDetailRow(
          'Rate limit',
          job.rateLimitMbps && job.rateLimitMbps > 0 ? `${job.rateLimitMbps} MB/s` : '',
        ),
      ]),
    },
    {
      label: 'Schedule',
      rows: compactDetailRows([
        makeDetailRow('Schedule', job.schedule),
        makeDetailRow(
          'Last sync',
          formatPlatformTableRelativeTimeValue(syncTimeValue(job), { now, emptyText: '' }),
        ),
        makeDetailRow('Next sync', next.tone === 'muted' ? '' : next.text, {
          tone: NEXT_SYNC_DETAIL_TONE[next.tone],
        }),
        makeDetailRow(
          'Duration',
          formatPlatformTableDurationValue(job.lastSyncDurationSeconds, {
            fallbackText: job.lastSyncDurationHuman,
            emptyText: '',
          }),
        ),
      ]),
    },
  ]);
};

// The row truncates and narrower layouts drop columns, so the drawer is where
// the whole job is always readable, starting with why it failed.
const ReplicationJobDetail: Component<{
  job: ReplicationJob;
  classification: ReplicationStatusFilter;
  guestLabel: string;
  now: number;
  onClose: () => void;
}> = (props) => {
  const indicator = () => indicatorFor(props.classification);
  const errorText = () => (props.job.error ?? '').trim();
  const guest = () => replicationJobGuestSubject(props.job);
  return (
    <InlineDetailPanel
      testId="replication-job-detail"
      detailFor={props.job.id || props.job.jobId}
      title={
        props.job.guestName?.trim() || props.job.guest?.trim() || props.job.jobId || props.job.id
      }
      status={{ variant: indicator().variant, label: indicator().label }}
      summary={
        <Show
          when={props.classification === 'failed' && errorText()}
          fallback={replicationJobSummary(props.job, props.now)}
        >
          <span class="text-red-600 dark:text-red-300" data-replication-job-error>
            {errorText()}
          </span>
        </Show>
      }
      actions={<RecordOwnerLink subject={guest()} />}
      sections={buildReplicationJobSections(
        props.job,
        props.classification,
        props.guestLabel,
        props.now,
      )}
      onClose={props.onClose}
    />
  );
};

export async function fetchReplicationJobs(signal?: AbortSignal): Promise<ReplicationJob[]> {
  const response = await apiFetch('/api/replication/jobs?platform=proxmox-pve', { signal });
  if (!response.ok) {
    // A structured error keeps the HTTP status, so a 401/403 withdraws the
    // jobs as an access failure instead of being retained as an outage.
    throw await apiErrorFromResponse(
      response,
      `Failed to load replication jobs (${response.status})`,
    );
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
  const [expandedJobKey, setExpandedJobKey] = createSignal<string | null>(null);
  // Rows stay mounted between job refreshes, so Last sync and Next sync read
  // the shared clock to keep moving while a job's timestamps do not change.
  const now = useRelativeTimeNow();

  const filterByStatus = (want: ReplicationStatusFilter) => {
    const split = splitSearchExclusions(search());
    return (props.jobs ?? []).filter((job) => {
      if (want !== 'all' && classifyJob(job) !== want) return false;
      if (!split.needle && split.excludes.length === 0) return true;
      const haystack = [
        job.jobId,
        job.guest,
        job.guestName,
        job.guestId?.toString() ?? '',
        job.sourceNode,
        job.targetNode,
        job.instance,
        job.lastSyncStatus,
        job.error,
        job.comment,
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return matchesSearchTermSplit(haystack, split);
    });
  };
  const filtered = createMemo(() => filterByStatus(status()));
  const countForStatus = (value: ReplicationStatusFilter): number => filterByStatus(value).length;

  const total = createMemo(() => (props.jobs ?? []).length);
  const visible = createMemo(() => filtered().length);
  const searchSuggestions = createMemo(() =>
    buildPlatformSearchSuggestions(
      (props.jobs ?? []).map((job, index) => ({
        id: job.jobId || job.id || `job-${index}`,
        label: job.guestName?.trim() || job.guest?.trim() || job.jobId || job.id,
        description: [
          job.jobId,
          job.sourceNode && job.targetNode ? `${job.sourceNode} → ${job.targetNode}` : '',
        ]
          .filter(Boolean)
          .join(' · '),
        keywords: [
          job.guest ?? '',
          job.guestId?.toString() ?? '',
          job.sourceNode ?? '',
          job.targetNode ?? '',
          job.instance ?? '',
        ],
      })),
      'replication-job',
    ),
  );
  const observedWidth = useObservedElementWidth();
  const layout = createMemo(() =>
    getPlatformTableContainerLayout(observedWidth.width() ?? 1920, [520, 720, 960, 1200]),
  );
  const isCompact = createMemo(() => layout() === 'compact');
  const mobilePaddingClass = () => (isCompact() ? 'px-1!' : '');
  const mobileHeadClass = () => (isCompact() ? 'px-1! text-[9px]!' : '');
  const mobileLastSyncClass = () => (isCompact() ? 'px-[2px]! tracking-normal!' : '');
  const showJob = createMemo(() => !isCompact());
  const showNext = createMemo(() => true);
  const showOperational = createMemo(() => ['operational', 'expanded', 'full'].includes(layout()));
  // The cron-style schedule is the least actionable column, and the mid-width
  // table cannot fit it without clipping the route and next-sync values.
  const showSchedule = createMemo(() => ['expanded', 'full'].includes(layout()));
  const showError = createMemo(() => ['expanded', 'full'].includes(layout()));
  const columnWidthClass = (column: ReplicationColumn) =>
    isCompact() ? '' : REPLICATION_COLUMN_WIDTH_CLASS[layout()][column];
  const visibleColumnCount = createMemo(() => {
    if (layout() === 'compact') return REPLICATION_MOBILE_COLUMNS.length;
    if (layout() === 'basic') return 6;
    return showError() ? 10 : 8;
  });

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
              searchSuggestions={searchSuggestions}
              status={status()}
              onStatusChange={setStatus}
              statusOptions={withPlatformStatusCounts(STATUS_FILTER_OPTIONS, countForStatus)}
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
                tableClass="min-w-0 table-fixed text-xs"
                colgroup={
                  <Show when={layout() === 'compact'}>
                    <colgroup>
                      <For each={REPLICATION_MOBILE_COLUMNS}>
                        {(column) => (
                          <col
                            style={{ width: `${REPLICATION_MOBILE_COLUMN_WIDTHS[column]}%` }}
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
                      class={`${getPlatformTableHeadClassForKind('name')} ${columnWidthClass('guest')} ${mobileHeadClass()}`}
                    >
                      Guest
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('text')} ${columnWidthClass('status')} ${mobileHeadClass()}`}
                    >
                      {isCompact() ? 'State' : 'Status'}
                    </TableHead>
                    <Show when={showJob()}>
                      <TableHead
                        class={`${getPlatformTableHeadClassForKind('text')} ${columnWidthClass('job')} ${mobileHeadClass()}`}
                      >
                        Job
                      </TableHead>
                    </Show>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('text')} ${columnWidthClass('route')} ${mobileHeadClass()}`}
                      title="Source → target"
                    >
                      Route
                    </TableHead>
                    <Show when={showSchedule()}>
                      <TableHead
                        class={`${getPlatformTableHeadClassForKind('text')} ${columnWidthClass('schedule')}`}
                      >
                        Schedule
                      </TableHead>
                    </Show>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('numeric-value')} ${columnWidthClass('lastSync')} ${mobileHeadClass()} ${mobileLastSyncClass()}`}
                    >
                      {layout() === 'compact' ? 'Ago' : 'Last sync'}
                    </TableHead>
                    <Show when={showNext()}>
                      <TableHead
                        class={`${getPlatformTableHeadClassForKind('numeric-value')} ${columnWidthClass('nextSync')} ${mobileHeadClass()}`}
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
                    <PlatformWindowedRows items={filtered} estimatedRowHeight={32}>
                      {(job, index) => {
                        const classification = classifyJob(job);
                        const ind = indicatorFor(classification);
                        const next = () => nextSyncFor(job, now());
                        const sourceNode = (job.sourceNode ?? '').trim() || '—';
                        const targetNode = (job.targetNode ?? '').trim() || '—';
                        const guestLabel = formatGuestLabel(job);
                        const jobKey = `${job.id}:${job.jobId}:${job.guestId ?? index()}`;
                        const detailRowId = replicationJobDetailRowId(job);
                        const errorText = (job.error ?? '').trim();
                        const isExpanded = () => expandedJobKey() === jobKey;
                        const toggleDetails = () => {
                          setExpandedJobKey(isExpanded() ? null : jobKey);
                        };
                        return (
                          <>
                            <TableRow
                              {...getPlatformResourceDetailRowInteractionProps({
                                expanded: isExpanded(),
                                onToggle: toggleDetails,
                                class: getPlatformTableRowClass(),
                              })}
                            >
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('name')} text-base-content ${mobilePaddingClass()}`}
                                title={guestLabel}
                              >
                                <div class="flex min-w-0 items-center gap-2">
                                  <PlatformResourceDetailToggleButton
                                    expanded={isExpanded()}
                                    resourceLabel={`replication job ${job.jobId || job.id}`}
                                    controlsId={detailRowId}
                                    onToggle={toggleDetails}
                                  />
                                  <span
                                    class={`min-w-0 truncate ${isCompact() ? 'text-[10px]' : ''}`}
                                  >
                                    {isCompact()
                                      ? formatMobileReplicationGuestLabel(job)
                                      : guestLabel}
                                  </span>
                                </div>
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('text')} ${mobilePaddingClass()}`}
                              >
                                <div class={`flex items-center ${isCompact() ? 'gap-0' : 'gap-2'}`}>
                                  <Show when={!isCompact()}>
                                    <StatusDot
                                      size="sm"
                                      variant={ind.variant}
                                      title={ind.label}
                                      ariaHidden
                                    />
                                  </Show>
                                  <span
                                    class={`${isCompact() ? 'text-[10px]' : 'text-[11px]'} font-medium ${ind.tone}`}
                                  >
                                    {ind.label}
                                  </span>
                                </div>
                              </TableCell>
                              <Show when={showJob()}>
                                <TableCell
                                  class={`${getPlatformTableCellClassForKind('text')} text-base-content font-mono text-[11px] ${mobilePaddingClass()}`}
                                >
                                  <span title={job.id}>{job.jobId || job.id}</span>
                                </TableCell>
                              </Show>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('text')} text-base-content ${mobilePaddingClass()}`}
                              >
                                <Show
                                  when={isCompact()}
                                  fallback={
                                    <span class="inline-flex items-center gap-1 font-mono text-[11px]">
                                      <span>{sourceNode}</span>
                                      <ArrowRightIcon
                                        class="h-3 w-3 text-muted"
                                        aria-hidden="true"
                                      />
                                      <span>{targetNode}</span>
                                    </span>
                                  }
                                >
                                  <span class="font-mono text-[10px]">
                                    {sourceNode}→{targetNode}
                                  </span>
                                </Show>
                              </TableCell>
                              <Show when={showSchedule()}>
                                <TableCell
                                  class={`${getPlatformTableCellClassForKind('text')} text-base-content font-mono text-[11px]`}
                                >
                                  {job.schedule || '—'}
                                </TableCell>
                              </Show>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content ${mobilePaddingClass()} ${mobileLastSyncClass()}`}
                              >
                                <Show
                                  when={isCompact()}
                                  fallback={
                                    <PlatformTableRelativeTimeValue value={syncTimeValue(job)} />
                                  }
                                >
                                  <span class="tabular-nums text-[10px]">
                                    {formatPlatformTableRelativeTimeValue(syncTimeValue(job), {
                                      now: now(),
                                    }).replace(/ ago$/, '')}
                                  </span>
                                </Show>
                              </TableCell>
                              <Show when={showNext()}>
                                <TableCell
                                  class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content ${mobilePaddingClass()}`}
                                >
                                  <span
                                    class={`${NEXT_SYNC_TONE_CLASS[next().tone]} ${isCompact() ? 'text-[10px]' : ''}`}
                                  >
                                    {isCompact()
                                      ? compactReplicationNextSyncText(next().text, next().tone)
                                      : next().text}
                                  </span>
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
                                    when={errorText}
                                    fallback={<span class="text-muted">—</span>}
                                  >
                                    <span
                                      class="block truncate text-red-600 dark:text-red-300"
                                      title={errorText}
                                    >
                                      {errorText}
                                    </span>
                                  </Show>
                                </TableCell>
                              </Show>
                            </TableRow>
                            <Show when={isExpanded()}>
                              <InlineDetailTableRow
                                cellId={detailRowId}
                                colspan={visibleColumnCount()}
                                class=""
                                data-inline-detail-for={job.id || job.jobId}
                              >
                                <ReplicationJobDetail
                                  job={job}
                                  classification={classification}
                                  guestLabel={guestLabel}
                                  now={now()}
                                  onClose={() => setExpandedJobKey(null)}
                                />
                              </InlineDetailTableRow>
                            </Show>
                          </>
                        );
                      }}
                    </PlatformWindowedRows>
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
