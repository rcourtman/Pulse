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
import { InlineNotice } from '@/components/shared/InlineNotice';
import { StatusDot } from '@/components/shared/StatusDot';
import type { StatusIndicatorVariant } from '@/utils/status';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { apiErrorFromResponse, apiFetch } from '@/utils/apiClient';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import type { Resource } from '@/types/resource';
import { resolveProxmoxSourceStaleness } from './proxmoxBackupStaleness';
import {
  PlatformTableToolbar,
  PlatformErrorState,
  PlatformTableDurationValue,
  formatPlatformTableDurationValue,
  formatPlatformTableRelativeTimeValue,
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
import type { ReplicationJob } from '@/types/api';
import {
  parseReplicationInventory,
  readReplicationInventoryJSON,
} from './proxmoxReplicationInventory';
import {
  buildPlatformSearchSuggestions,
  type PlatformSearchSuggestionCandidate,
} from '@/features/platformPage/platformSearchSuggestions';
import { splitSearchExclusions } from '@/utils/searchQuery';
import { buildProxmoxGuestNativeId } from '@/utils/workloads';
import { parseBackupTimestampMs } from '@/utils/backupTimestamp';

// Replication is a Proxmox-specific concept (zfs send/receive scheduled
// between PVE nodes), so this table is bespoke rather than a filtered
// view of any generic resource list. It hits the dedicated
// /api/replication/jobs surface which projects the monitor's
// ReplicationJobsSnapshot without going through the unified-resource
// pipeline.

type ReplicationStatusFilter =
  'all' | 'healthy' | 'stale' | 'failed' | 'pending' | 'unknown' | 'disabled';

type ReplicationJobRecord = ReplicationJob & {
  // Go models.ReplicationJob serializes LastPolled as an RFC3339 string on
  // /api/replication/jobs; ReplicationJobFrontend uses numeric polledAt.
  lastPolled?: number | string;
};

export interface ReplicationStaleNotice {
  badge: string;
  compactBadge: string;
  detail: string;
  error?: string;
}

const normalizeReplicationToken = (value: unknown): string =>
  typeof value === 'string' ? value.trim().toLowerCase() : '';

const buildProxmoxNodeStaleNotice = (
  node: Resource,
  subjectOverride?: string,
): ReplicationStaleNotice | null => {
  const stale = resolveProxmoxSourceStaleness(node, 'proxmox', subjectOverride);
  if (!stale) return null;
  return {
    badge: 'Stale data',
    compactBadge: 'Stale',
    detail: `${stale.reason} — showing the last values received.`,
    ...(stale.error ? { error: stale.error } : {}),
  };
};

// Replication jobs come from /api/replication/jobs without a per-job source
// health envelope, so freshness follows the owning Proxmox VE node (or whole
// PVE connection when no specific source node matches) already loaded by the
// page. This avoids inventing a client-side timeout that would disagree with
// adaptive PVE polling intervals.
export const getReplicationJobStaleNotice = (
  job: ReplicationJob,
  nodes: readonly Resource[] | undefined,
): ReplicationStaleNotice | null => {
  if (!nodes || nodes.length === 0) return null;
  const instance = normalizeReplicationToken(job.instance);
  const sourceNode =
    normalizeReplicationToken(job.sourceNode) || normalizeReplicationToken(job.guestNode);
  const pveNodes = nodes.filter((node) => node.type === 'agent');
  if (pveNodes.length === 0) return null;

  const instanceNodes = instance
    ? pveNodes.filter((node) => normalizeReplicationToken(node.proxmox?.instance) === instance)
    : [];

  if (sourceNode) {
    const matched = instanceNodes.find(
      (node) =>
        normalizeReplicationToken(node.proxmox?.nodeName) === sourceNode ||
        normalizeReplicationToken(node.name) === sourceNode,
    );
    if (matched) {
      return buildProxmoxNodeStaleNotice(matched);
    }
    if (instanceNodes.length === 0) {
      const uninstancedMatches = pveNodes.filter(
        (node) =>
          !normalizeReplicationToken(node.proxmox?.instance) &&
          (normalizeReplicationToken(node.proxmox?.nodeName) === sourceNode ||
            normalizeReplicationToken(node.name) === sourceNode),
      );
      if (uninstancedMatches.length === 1) {
        return buildProxmoxNodeStaleNotice(uninstancedMatches[0]);
      }
    }
  }

  if (instanceNodes.length > 0) {
    const notices = instanceNodes.map((node) => buildProxmoxNodeStaleNotice(node));
    if (notices.every((notice): notice is ReplicationStaleNotice => notice !== null)) {
      if (instanceNodes.length === 1) return notices[0];
      const preferredIndex = notices.findIndex((notice) => Boolean(notice.error));
      const representative = instanceNodes[preferredIndex >= 0 ? preferredIndex : 0];
      return buildProxmoxNodeStaleNotice(representative, job.instance);
    }
  }

  return null;
};

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

const STALE_FILTER_OPTION: PlatformTableFilterOption<ReplicationStatusFilter> = {
  value: 'stale',
  label: 'Stale',
  tone: 'warning',
  leading: filterChipStatusDot('bg-amber-500'),
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
    value: 'unknown',
    label: 'Unknown',
    tone: 'muted',
    leading: filterChipStatusDot('bg-slate-400'),
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
  compactLabel?: string;
  tone: string;
}

const reportedSyncOutcome = (job: ReplicationJob): string =>
  (job.lastSyncStatus ?? '').trim().toLowerCase();

const hasSuccessfulSyncOutcome = (job: ReplicationJob): boolean =>
  ['ok', 'success', 'completed'].includes(reportedSyncOutcome(job));

// The canonical DTO omits a zero count. A successful observed sync requires
// that zero, but an absent/unsupported outcome cannot establish it. Keep an
// explicit count independent of the outcome, including an explicitly read 0.
const replicationFailCount = (job: ReplicationJob): number | undefined =>
  job.failCount ?? (hasSuccessfulSyncOutcome(job) ? 0 : undefined);

function classifyJob(
  job: ReplicationJob,
  staleNotice: ReplicationStaleNotice | null = null,
): ReplicationStatusFilter {
  if (!job.enabled) return 'disabled';
  if ((job.failCount ?? 0) > 0) return 'failed';
  const last = reportedSyncOutcome(job);
  if (hasSuccessfulSyncOutcome(job)) return staleNotice ? 'stale' : 'healthy';
  if (last === 'failed' || last === 'error') return 'failed';
  if (last === 'pending') return 'pending';
  // Configuration can remain readable while its source-node status read is
  // unavailable. Neither that omission nor a future status means "pending".
  return 'unknown';
}

function indicatorFor(classification: ReplicationStatusFilter): ReplicationStatusIndicator {
  switch (classification) {
    case 'healthy':
      return {
        variant: 'success',
        label: 'Healthy',
        tone: 'text-emerald-600 dark:text-emerald-300',
      };
    case 'stale':
      return {
        variant: 'warning',
        label: 'Stale data',
        compactLabel: 'Stale',
        tone: 'text-amber-700 dark:text-amber-300',
      };
    case 'failed':
      return { variant: 'danger', label: 'Failed', tone: 'text-red-600 dark:text-red-300' };
    case 'pending':
      return { variant: 'warning', label: 'Pending', tone: 'text-amber-600 dark:text-amber-300' };
    case 'unknown':
      return { variant: 'muted', label: 'Unknown', tone: 'text-muted' };
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

type ReplicationSyncTime =
  { state: 'known'; ms: number } | { state: 'missing' } | { state: 'invalid' };

// Keep the existing Unix companion precedence and numeric seconds/milliseconds
// compatibility. Never let Date.parse repair an impossible calendar date or
// assign the browser's timezone to a provider time. A malformed nonzero Unix
// companion cannot be replaced with a more reassuring fallback.
export function readReplicationSyncTime(
  time: ReplicationJob['lastSyncTime'],
  unix: number | undefined,
  notAfter?: number,
): ReplicationSyncTime {
  let ms: number | undefined;
  if (unix !== undefined && unix !== 0) {
    ms = unix * 1000;
  } else if (time === undefined || time === '' || time === 0) {
    return { state: 'missing' };
  } else {
    ms =
      typeof time === 'number' ? (time > 1e12 ? time : time * 1000) : parseBackupTimestampMs(time);
  }
  if (
    ms === undefined ||
    !Number.isFinite(ms) ||
    ms <= 0 ||
    !Number.isFinite(new Date(ms).getTime()) ||
    (notAfter !== undefined && ms > notAfter)
  ) {
    return { state: 'invalid' };
  }
  return { state: 'known', ms };
}

function lastSyncFor(job: ReplicationJob, now: number) {
  const time = readReplicationSyncTime(job.lastSyncTime, job.lastSyncUnix, now);
  if (time.state === 'missing') return { text: '—', summary: 'No last sync reported' };
  if (time.state === 'invalid') return { text: 'Unknown', summary: 'Last sync time unknown' };
  const text = formatPlatformTableRelativeTimeValue(new Date(time.ms), { now });
  return { text, summary: `Last synced ${text}` };
}

type NextSyncTone = 'overdue' | 'imminent' | 'normal' | 'muted' | 'unknown';

const NEXT_SYNC_TONE_CLASS: Record<NextSyncTone, string> = {
  overdue: 'text-red-600 dark:text-red-300 font-semibold',
  imminent: 'text-amber-600 dark:text-amber-300',
  normal: '',
  muted: 'text-muted',
  unknown: 'text-muted',
};

// An overdue next-sync is the one signal that catches a stalled pvesr
// scheduler even while the last sync still reports ok, so it gets its own
// column instead of folding into the status pill (which mirrors PVE's own
// job state). The countdown is measured from the shared relative-time clock:
// a stalled scheduler leaves nextSync unchanged, so a countdown read once at
// render would sit at "in 3m" and never turn overdue.
function nextSyncFor(job: ReplicationJob, now: number): { text: string; tone: NextSyncTone } {
  if (!job.enabled) return { text: '—', tone: 'muted' };
  const time = readReplicationSyncTime(job.nextSyncTime, job.nextSyncUnix);
  if (time.state === 'missing') return { text: '—', tone: 'muted' };
  if (time.state === 'invalid') return { text: 'Unknown', tone: 'unknown' };
  const minutes = Math.floor((time.ms - now) / 60_000);
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
  unknown: 'muted',
};

// The detail row id follows the job, not its place in the filtered list, so a
// refresh that reorders rows keeps the disclosure pointing at the same job.
const replicationJobDetailRowId = (job: ReplicationJob): string =>
  `replication-job-detail-${encodeURIComponent(job.id)}`;

const REPLICATION_JOB_TERM_PREFIX = 'replication-job:';

// FilterBar serializes selected suggestions into the same search string as
// free text. Use an exact, comma/space-safe term for a selected job, even if
// it disappears on the next poll. Escape uppercase literals so the shared
// search's case folding cannot merge distinct IDs.
const replicationJobSearchTerm = (id: string): string =>
  REPLICATION_JOB_TERM_PREFIX +
  encodeURIComponent(id).replace(/%[0-9A-F]{2}|[A-Z]/g, (part) =>
    part.startsWith('%') ? part.toLowerCase() : `%${part.charCodeAt(0).toString(16)}`,
  );

const replicationJobSearchLabel = (job: ReplicationJob): string => `${job.jobId} · ${job.instance}`;

const buildReplicationSearchSuggestions = (jobs: readonly ReplicationJob[]) => {
  const terms = new Map<string, { label: string; exactJobId?: string }>();
  const addTerm = (value: string | undefined, exactJobId?: string) => {
    const label = value?.trim();
    if (!label) return;
    const key = label.toLowerCase();
    const existing = terms.get(key);
    terms.set(key, {
      label: existing?.label ?? label,
      exactJobId: existing && existing.exactJobId !== exactJobId ? undefined : exactJobId,
    });
  };
  for (const job of jobs) {
    addTerm(job.guestName);
    addTerm(job.guest);
    addTerm(job.jobId);
    addTerm(job.id, job.id);
    addTerm(replicationJobSearchLabel(job), job.id);
  }

  // SearchInput can accept a common prefix on Tab and then select an exact
  // completion on Enter. Keep colliding aliases and shared prefixes as broad
  // text, just like guest names and native numbers.
  const keys = [...terms.keys()].sort();
  const candidates: PlatformSearchSuggestionCandidate[] = [];
  keys.forEach((key, index) => {
    const term = terms.get(key)!;
    if (keys[index + 1]?.startsWith(key)) term.exactJobId = undefined;
    if (!term.exactJobId) {
      candidates.push({ id: `text:${key}`, label: term.label, value: term.label });
    }
  });
  const isSelectable = (value: string, job: ReplicationJob) =>
    terms.get(value.trim().toLowerCase())?.exactJobId === job.id;
  for (const job of jobs) {
    const qualifiedLabel = replicationJobSearchLabel(job);
    const exactId = isSelectable(job.id, job) ? job.id : undefined;
    const label = isSelectable(qualifiedLabel, job) ? qualifiedLabel : exactId;
    if (!label) continue;
    candidates.push({
      id: `job:${job.id}`,
      label,
      value: replicationJobSearchTerm(job.id),
      completions: exactId && exactId !== label ? [exactId] : undefined,
      keywords: [
        job.guestName ?? '',
        job.guest ?? '',
        job.jobId,
        job.guestId?.toString() ?? '',
        job.sourceNode ?? '',
        job.targetNode ?? '',
        job.instance,
      ],
    });
  }
  return buildPlatformSearchSuggestions(candidates, 'replication');
};

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
  const last = lastSyncFor(job, now);
  const next = nextSyncFor(job, now);
  return [
    last.summary,
    next.tone === 'muted'
      ? ''
      : next.tone === 'unknown'
        ? 'next sync time unknown'
        : `next sync ${next.text}`,
  ]
    .filter(Boolean)
    .join(' · ');
};

const buildReplicationJobSections = (
  job: ReplicationJob,
  classification: ReplicationStatusFilter,
  staleNotice: ReplicationStaleNotice | null,
  guestLabel: string,
  now: number,
): DetailSection[] => {
  const errorText = (job.error ?? '').trim();
  const last = lastSyncFor(job, now);
  const next = nextSyncFor(job, now);
  const failCount = replicationFailCount(job);
  const polledRaw = (job as ReplicationJobRecord).lastPolled ?? job.polledAt;
  const polled = readReplicationSyncTime(polledRaw, undefined, now);
  const polledText =
    polled.state === 'known'
      ? formatPlatformTableRelativeTimeValue(new Date(polled.ms), { now })
      : polled.state === 'invalid'
        ? 'Unknown'
        : '';
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
        makeDetailRow('Connection', job.instance, { wrap: true }),
        classification === 'stale'
          ? makeDetailRow('Last reported', 'Healthy', { tone: 'muted' })
          : null,
        classification === 'unknown'
          ? makeDetailRow('Reported outcome', job.lastSyncStatus?.trim() || 'Not reported', {
              wrap: true,
            })
          : null,
        makeDetailRow('Failures', failCount === undefined ? 'Not reported' : String(failCount), {
          tone: failCount === undefined ? 'muted' : failCount > 0 ? 'danger' : undefined,
        }),
        makeDetailRow('Comment', job.comment),
      ]),
      footerContent: (
        <Show when={classification === 'unknown'}>
          <p class="whitespace-normal text-[11px] leading-snug text-muted wrap-anywhere">
            The latest sync outcome is not reported or is not recognised. Unknown does not mean the
            job is pending or has no failures.
          </p>
        </Show>
      ),
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
        makeDetailRow('Last sync', last.text === '—' ? 'Not reported' : last.text),
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
        polledText
          ? makeDetailRow('Last polled', polledText, {
              tone: polled.state === 'invalid' ? 'muted' : staleNotice ? 'warning' : undefined,
            })
          : null,
      ]),
    },
  ]);
};

// The row truncates and narrower layouts drop columns, so the drawer is where
// the whole job is always readable, starting with why it failed.
const ReplicationJobDetail: Component<{
  job: ReplicationJob;
  classification: ReplicationStatusFilter;
  staleNotice: ReplicationStaleNotice | null;
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
        props.staleNotice,
        props.guestLabel,
        props.now,
      )}
      onClose={props.onClose}
    >
      <Show when={props.staleNotice}>
        {(notice) => (
          <InlineNotice tone="warning" role="status" data-testid="replication-job-stale-notice">
            <span class="font-semibold">{notice().badge}.</span> {notice().detail}
          </InlineNotice>
        )}
      </Show>
    </InlineDetailPanel>
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
  return parseReplicationInventory(await readReplicationInventoryJSON(response));
}

// The jobs resource lives in ProxmoxPageSurface (it also gates the
// Replication tab's visibility), so this table is purely presentational.
export const ProxmoxReplicationTable: Component<{
  jobs: ReplicationJob[] | undefined;
  nodes?: Resource[];
  error: unknown;
  onRetry: () => void;
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
}> = (props) => {
  const [search, setSearch] = createSignal('');
  const [status, setStatus] = createSignal<ReplicationStatusFilter>('all');
  const [expandedJobId, setExpandedJobId] = createSignal<string | null>(null);
  // Rows stay mounted between job refreshes, so Last sync and Next sync read
  // the shared clock to keep moving while a job's timestamps do not change.
  const now = useRelativeTimeNow();

  const staleNoticeByJobId = createMemo(() => {
    const notices = new Map<string, ReplicationStaleNotice | null>();
    for (const job of props.jobs ?? []) {
      notices.set(job.id, getReplicationJobStaleNotice(job, props.nodes));
    }
    return notices;
  });
  const staleNoticeFor = (job: ReplicationJob): ReplicationStaleNotice | null =>
    staleNoticeByJobId().get(job.id) ?? null;

  const matchingJobs = createMemo(() => {
    const jobs = props.jobs ?? [];
    const { needle, excludes } = splitSearchExclusions(search());
    if (!needle && excludes.length === 0) return jobs;
    // A draft containing only exclusions leaves a comma after its last chip.
    const terms = needle
      .split(',')
      .map((term) => term.trim())
      .filter(Boolean);
    return jobs.filter((job) => {
      const staleNotice = staleNoticeFor(job);
      const haystack = [
        job.id,
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
        replicationJobSearchLabel(job),
        staleNotice ? `${staleNotice.badge} ${staleNotice.detail}` : '',
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      if (excludes.some((exclude) => haystack.includes(exclude))) return false;
      return (
        terms.length === 0 ||
        terms.some((term) =>
          term.startsWith(REPLICATION_JOB_TERM_PREFIX)
            ? term === replicationJobSearchTerm(job.id)
            : haystack.includes(term),
        )
      );
    });
  });
  const filterByStatus = (want: ReplicationStatusFilter) =>
    want === 'all'
      ? matchingJobs()
      : matchingJobs().filter((job) => classifyJob(job, staleNoticeFor(job)) === want);
  const filtered = createMemo(() => filterByStatus(status()));
  const countForStatus = (value: ReplicationStatusFilter): number => filterByStatus(value).length;
  const statusFilterOptions = createMemo(() => {
    const showStaleChip = countForStatus('stale') > 0 || status() === 'stale';
    const options = showStaleChip
      ? [
          STATUS_FILTER_OPTIONS[0],
          STATUS_FILTER_OPTIONS[1],
          STALE_FILTER_OPTION,
          ...STATUS_FILTER_OPTIONS.slice(2),
        ]
      : STATUS_FILTER_OPTIONS;
    return withPlatformStatusCounts(options, countForStatus);
  });

  const total = createMemo(() => (props.jobs ?? []).length);
  const visible = createMemo(() => filtered().length);
  const repeatedNativeJobIds = createMemo(() => {
    const seen = new Set<string>();
    const repeated = new Set<string>();
    for (const job of props.jobs ?? []) {
      if (seen.has(job.jobId)) repeated.add(job.jobId);
      seen.add(job.jobId);
    }
    return repeated;
  });
  const searchSuggestions = createMemo(() => buildReplicationSearchSuggestions(props.jobs ?? []));
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
              statusOptions={statusFilterOptions()}
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
                      {(job) => {
                        const staleNotice = () => staleNoticeFor(job);
                        const classification = () => classifyJob(job, staleNotice());
                        const ind = () => indicatorFor(classification());
                        const last = () => lastSyncFor(job, now());
                        const next = () => nextSyncFor(job, now());
                        const sourceNode = (job.sourceNode ?? '').trim() || '—';
                        const targetNode = (job.targetNode ?? '').trim() || '—';
                        const guestLabel = formatGuestLabel(job);
                        const detailRowId = replicationJobDetailRowId(job);
                        const errorText = (job.error ?? '').trim();
                        const showConnection = () => repeatedNativeJobIds().has(job.jobId);
                        const isExpanded = () => expandedJobId() === job.id;
                        const toggleDetails = () => {
                          setExpandedJobId(isExpanded() ? null : job.id);
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
                                    resourceLabel={`replication job ${job.jobId}${showConnection() ? ` on ${job.instance}` : ''}`}
                                    controlsId={detailRowId}
                                    onToggle={toggleDetails}
                                  />
                                  <div class="min-w-0">
                                    <div class={`truncate ${isCompact() ? 'text-[10px]' : ''}`}>
                                      {isCompact()
                                        ? formatMobileReplicationGuestLabel(job)
                                        : guestLabel}
                                    </div>
                                    <Show when={showConnection()}>
                                      <bdi class="block text-[10px] leading-tight text-muted wrap-anywhere">
                                        {job.instance}
                                      </bdi>
                                    </Show>
                                  </div>
                                </div>
                              </TableCell>
                              <TableCell
                                class={`${getPlatformTableCellClassForKind('text')} ${mobilePaddingClass()}`}
                                title={staleNotice()?.detail ?? ind().label}
                              >
                                <div
                                  class={`flex min-w-0 ${isCompact() ? 'flex-col items-start gap-0' : 'items-center gap-1.5'}`}
                                >
                                  <Show when={!isCompact()}>
                                    <StatusDot
                                      size="sm"
                                      variant={ind().variant}
                                      title={ind().label}
                                      ariaHidden
                                    />
                                  </Show>
                                  <span
                                    class={`shrink-0 ${isCompact() ? 'text-[10px] leading-tight' : 'text-[11px]'} font-medium ${ind().tone}`}
                                  >
                                    {isCompact()
                                      ? (ind().compactLabel ?? ind().label)
                                      : ind().label}
                                  </span>
                                  <Show when={classification() !== 'stale' ? staleNotice() : null}>
                                    {(notice) => (
                                      <span
                                        data-replication-stale-cue
                                        class={`truncate font-medium text-amber-700 dark:text-amber-300 ${isCompact() ? 'text-[9px] leading-tight' : 'text-[10px]'}`}
                                        title={notice().detail}
                                      >
                                        {notice().compactBadge}
                                      </span>
                                    )}
                                  </Show>
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
                                <span
                                  class={`tabular-nums ${isCompact() ? 'text-[10px]' : ''}`}
                                  aria-label={
                                    last().text === 'Unknown' ? 'Last sync time unknown' : undefined
                                  }
                                  title={
                                    last().text === 'Unknown' ? 'Last sync time unknown' : undefined
                                  }
                                >
                                  {isCompact()
                                    ? last().text === 'Unknown'
                                      ? '?'
                                      : last().text.replace(/ ago$/, '')
                                    : last().text}
                                </span>
                              </TableCell>
                              <Show when={showNext()}>
                                <TableCell
                                  class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content ${mobilePaddingClass()}`}
                                >
                                  <span
                                    class={`${NEXT_SYNC_TONE_CLASS[next().tone]} ${isCompact() ? 'text-[10px]' : ''}`}
                                    aria-label={
                                      next().tone === 'unknown'
                                        ? 'Next sync time unknown'
                                        : undefined
                                    }
                                    title={
                                      next().tone === 'unknown'
                                        ? 'Next sync time unknown'
                                        : undefined
                                    }
                                  >
                                    {isCompact()
                                      ? next().tone === 'unknown'
                                        ? '?'
                                        : compactReplicationNextSyncText(next().text, next().tone)
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
                                    fallback={
                                      <span
                                        class="text-muted"
                                        aria-label={
                                          replicationFailCount(job) === undefined
                                            ? 'Failure count not reported'
                                            : undefined
                                        }
                                      >
                                        {replicationFailCount(job) === undefined ? '—' : '0'}
                                      </span>
                                    }
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
                                  classification={classification()}
                                  staleNotice={staleNotice()}
                                  guestLabel={guestLabel}
                                  now={now()}
                                  onClose={() => setExpandedJobId(null)}
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
