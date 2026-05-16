import {
  For,
  Show,
  createMemo,
  createResource,
  createSignal,
  type Component,
  type JSX,
} from 'solid-js';
import ArrowRightIcon from 'lucide-solid/icons/arrow-right';
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
import { formatRelativeTime } from '@/utils/format';
import type { ReplicationJob, ReplicationJobsResponse } from '@/types/api';

// Replication is a Proxmox-specific concept (zfs send/receive scheduled
// between PVE nodes), so this table is bespoke rather than a filtered
// view of any generic resource list. It hits the dedicated
// /api/replication/jobs surface which projects the monitor's
// ReplicationJobsSnapshot without going through the unified-resource
// pipeline.

type ReplicationStatusFilter = 'all' | 'healthy' | 'failed' | 'pending' | 'disabled';

const STATUS_FILTER_OPTIONS: FilterOption<ReplicationStatusFilter>[] = [
  { value: 'all', label: 'All' },
  { value: 'healthy', label: 'Healthy' },
  { value: 'failed', label: 'Failed' },
  { value: 'pending', label: 'Pending' },
  { value: 'disabled', label: 'Disabled' },
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
      return { variant: 'success', label: 'Healthy', tone: 'text-emerald-600 dark:text-emerald-300' };
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

function formatSyncTime(job: ReplicationJob): string {
  if (job.lastSyncUnix && job.lastSyncUnix > 0) {
    return formatRelativeTime(job.lastSyncUnix * 1000, { compact: true });
  }
  const raw = job.lastSyncTime as number | string | undefined;
  if (raw) return formatRelativeTime(raw, { compact: true });
  return '—';
}

function formatDuration(seconds: number | undefined, human: string | undefined): string {
  const explicit = (human ?? '').trim();
  if (explicit) return explicit;
  if (!seconds || seconds <= 0) return '—';
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  return `${h}h ${m}m`;
}

async function fetchReplicationJobs(): Promise<ReplicationJob[]> {
  const response = await apiFetch('/api/replication/jobs?platform=proxmox-pve');
  if (!response.ok) {
    throw new Error(`Failed to load replication jobs (${response.status})`);
  }
  const payload = (await response.json()) as ReplicationJobsResponse;
  return Array.isArray(payload?.data) ? payload.data : [];
}

export const ProxmoxReplicationTable: Component<{
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
}> = (props) => {
  const [jobs, { refetch }] = createResource<ReplicationJob[]>(fetchReplicationJobs);
  const [search, setSearch] = createSignal('');
  const [status, setStatus] = createSignal<ReplicationStatusFilter>('all');

  const filtered = createMemo(() => {
    const term = search().trim().toLowerCase();
    const want = status();
    return (jobs() ?? []).filter((job) => {
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

  const total = createMemo(() => (jobs() ?? []).length);
  const visible = createMemo(() => filtered().length);

  return (
    <Show
      when={!jobs.error}
      fallback={
        <Card padding="lg">
          <EmptyState
            icon={props.emptyIcon}
            title="Could not load replication jobs"
            description={(jobs.error as Error | undefined)?.message ?? 'Refresh to retry.'}
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
        when={jobs() !== undefined}
        fallback={
          <Card padding="lg">
            <EmptyState
              icon={props.emptyIcon}
              title="Loading replication jobs"
              description="Reading scheduled replication state from PVE."
            />
          </Card>
        }
      >
        <Show
          when={total() > 0}
          fallback={
            <Card padding="lg">
              <EmptyState
                icon={props.emptyIcon}
                title={props.emptyTitle}
                description={props.emptyDescription}
              />
            </Card>
          }
        >
          <div class="space-y-3">
            <div class="flex flex-wrap items-center gap-2">
              <div class="min-w-[200px] flex-1 sm:max-w-xs">
                <SearchInput
                  value={search}
                  onChange={setSearch}
                  placeholder="Search jobs, guests, nodes"
                />
              </div>
              <FilterButtonGroup
                options={STATUS_FILTER_OPTIONS}
                value={status()}
                onChange={setStatus}
              />
              <span class="ml-auto whitespace-nowrap text-xs font-medium text-muted">
                <Show when={visible() !== total()} fallback={<>{total()} jobs</>}>
                  {visible()} of {total()} jobs
                </Show>
              </span>
            </div>

            <Show
              when={filtered().length > 0}
              fallback={
                <Card padding="lg">
                  <EmptyState
                    icon={props.emptyIcon}
                    title="No replication jobs match current filters"
                    description="Adjust the search or status filter to see more jobs."
                  />
                </Card>
              }
            >
              <Card padding="none" tone="card" class="overflow-hidden">
                <Table class="w-full min-w-[1100px] border-collapse text-xs">
                  <TableHeader class="bg-surface-alt text-muted border-b border-border">
                    <TableRow class="text-left text-[10px] uppercase tracking-wide">
                      <TableHead class="px-3 py-2 font-medium">Status</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Job</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Guest</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Source → Target</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Schedule</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Last sync</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Duration</TableHead>
                      <TableHead class="px-3 py-2 font-medium text-right">Fail count</TableHead>
                      <TableHead class="px-3 py-2 font-medium">Error</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody class="divide-y divide-border-subtle">
                    <For each={filtered()}>
                      {(job) => {
                        const classification = classifyJob(job);
                        const ind = indicatorFor(classification);
                        const sourceNode = (job.sourceNode ?? '').trim() || '—';
                        const targetNode = (job.targetNode ?? '').trim() || '—';
                        return (
                          <TableRow class="hover:bg-surface-hover">
                            <TableCell class="px-3 py-2">
                              <div class="flex items-center gap-2">
                                <StatusDot
                                  size="sm"
                                  variant={ind.variant}
                                  title={ind.label}
                                  ariaHidden
                                />
                                <span class={`text-[11px] font-medium ${ind.tone}`}>{ind.label}</span>
                              </div>
                            </TableCell>
                            <TableCell class="px-3 py-2 text-base-content font-mono text-[11px]">
                              <span title={job.id}>{job.jobId || job.id}</span>
                            </TableCell>
                            <TableCell class="px-3 py-2 text-base-content">{formatGuestLabel(job)}</TableCell>
                            <TableCell class="px-3 py-2 text-base-content">
                              <span class="inline-flex items-center gap-1 font-mono text-[11px]">
                                <span>{sourceNode}</span>
                                <ArrowRightIcon class="h-3 w-3 text-muted" aria-hidden="true" />
                                <span>{targetNode}</span>
                              </span>
                            </TableCell>
                            <TableCell class="px-3 py-2 text-base-content font-mono text-[11px]">
                              {job.schedule || '—'}
                            </TableCell>
                            <TableCell class="px-3 py-2 text-base-content">{formatSyncTime(job)}</TableCell>
                            <TableCell class="px-3 py-2 text-base-content">
                              {formatDuration(job.lastSyncDurationSeconds, job.lastSyncDurationHuman)}
                            </TableCell>
                            <TableCell class="px-3 py-2 text-right text-base-content tabular-nums">
                              <Show
                                when={(job.failCount ?? 0) > 0}
                                fallback={<span class="text-muted">0</span>}
                              >
                                <span class="text-red-600 dark:text-red-300 font-semibold">{job.failCount}</span>
                              </Show>
                            </TableCell>
                            <TableCell class="px-3 py-2 text-base-content">
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
                          </TableRow>
                        );
                      }}
                    </For>
                  </TableBody>
                </Table>
              </Card>
            </Show>
          </div>
        </Show>
      </Show>
    </Show>
  );
};

export default ProxmoxReplicationTable;
