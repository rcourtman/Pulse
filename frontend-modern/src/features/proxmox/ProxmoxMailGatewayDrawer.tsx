import { For, Show, createMemo, createResource, type Component } from 'solid-js';
import XIcon from 'lucide-solid/icons/x';
import { Card } from '@/components/shared/Card';
import { StatusDot } from '@/components/shared/StatusDot';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
} from '@/features/platformPage/sharedPlatformPage';
import type { StatusIndicatorVariant } from '@/utils/status';
import { formatBytes } from '@/utils/format';
import { asTrimmedString } from '@/utils/stringUtils';
import { apiFetch } from '@/utils/apiClient';
import type { PMGInstance, PMGNodeStatus, PMGQueueStatus, PMGSpamBucket } from '@/types/api';
import type { Resource } from '@/types/resource';

// Inline drawer for a single Proxmox Mail Gateway instance. Drops the
// per-stat tile pattern in favor of stacked-bar diagrams that show the
// SHAPE of mail flow + quarantine + spam-score distribution at a
// glance, paired with proper info-dense tables for cluster nodes and
// top domains. Fetches the full PMGInstance on first open from
// /api/pmg/instances so the row keeps its slim payload.

interface PMGInstancesResponse {
  data: PMGInstance[];
  meta: { total: number };
}

async function fetchPMGInstance(id: string): Promise<PMGInstance | null> {
  const response = await apiFetch(`/api/pmg/instances?id=${encodeURIComponent(id)}`);
  if (!response.ok) {
    throw new Error(`Failed to load PMG instance (${response.status})`);
  }
  const payload = (await response.json()) as PMGInstancesResponse;
  return payload?.data?.[0] ?? null;
}

function classifyHealth(status: string | undefined): {
  variant: StatusIndicatorVariant;
  label: string;
} {
  const raw = (status ?? '').toLowerCase();
  if (raw === 'online' || raw === 'ok' || raw === 'healthy') {
    return { variant: 'success', label: 'Healthy' };
  }
  if (raw === 'degraded' || raw === 'warning') return { variant: 'warning', label: 'Degraded' };
  if (raw === 'offline' || raw === 'error' || raw === 'critical') {
    return { variant: 'danger', label: 'Offline' };
  }
  return { variant: 'muted', label: raw || 'Unknown' };
}

function classifyNode(node: PMGNodeStatus): { variant: StatusIndicatorVariant; label: string } {
  const raw = (node.status ?? '').toLowerCase();
  if (raw === 'online' || raw === 'ok') return { variant: 'success', label: 'Online' };
  if (raw === 'degraded' || raw === 'warning') return { variant: 'warning', label: 'Degraded' };
  if (raw === 'offline' || raw === 'down') return { variant: 'danger', label: 'Offline' };
  return { variant: 'muted', label: raw || '—' };
}

function formatUptime(seconds: number | undefined): string {
  if (!seconds || seconds <= 0) return '—';
  const days = Math.floor(seconds / 86_400);
  if (days > 0) return `${days}d`;
  const hours = Math.floor(seconds / 3_600);
  if (hours > 0) return `${hours}h`;
  const mins = Math.floor(seconds / 60);
  return `${mins}m`;
}

function formatAge(seconds: number): string {
  if (!seconds || seconds <= 0) return '—';
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
  if (seconds < 86_400) return `${Math.floor(seconds / 3600)}h`;
  return `${Math.floor(seconds / 86_400)}d`;
}

function formatNumber(value: number | undefined): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '—';
  return Math.round(value).toLocaleString();
}

function queueCell(queue: PMGQueueStatus | undefined): { count: number; label: string } {
  if (!queue) return { count: 0, label: '—' };
  return {
    count: queue.total,
    label: `${queue.active}/${queue.deferred}/${queue.hold}/${queue.incoming}`,
  };
}

// StackedBar renders a horizontal stacked bar from labelled segments.
// Each segment carries a tone (color), value, and label; segments with
// 0 value collapse out. A legend with name + count + share renders
// below.
interface StackedSegment {
  key: string;
  label: string;
  value: number;
  tone: string;
}

function StackedBar(props: { segments: StackedSegment[]; ariaLabel: string }) {
  const total = createMemo(() => props.segments.reduce((sum, s) => sum + Math.max(0, s.value), 0));
  const visible = createMemo(() => props.segments.filter((s) => s.value > 0));
  return (
    <div class="space-y-2">
      <Show
        when={total() > 0}
        fallback={<div class="relative h-2.5 w-full overflow-hidden rounded-full bg-surface-alt" />}
      >
        <div
          class="relative flex h-2.5 w-full overflow-hidden rounded-full bg-surface-alt"
          role="img"
          aria-label={props.ariaLabel}
        >
          <For each={visible()}>
            {(seg) => (
              <div
                class={seg.tone}
                style={{ width: `${(seg.value / total()) * 100}%` }}
                title={`${seg.label}: ${formatNumber(seg.value)}`}
              />
            )}
          </For>
        </div>
      </Show>
      <div class="flex flex-wrap gap-x-3 gap-y-1 text-[11px]">
        <For each={props.segments}>
          {(seg) => {
            const share = total() > 0 ? (seg.value / total()) * 100 : 0;
            return (
              <div class="flex items-center gap-1.5">
                <span class={`inline-block h-2 w-2 rounded-sm ${seg.tone}`} aria-hidden="true" />
                <span class="text-muted">{seg.label}</span>
                <span class="text-base-content font-semibold tabular-nums">
                  {formatNumber(seg.value)}
                </span>
                <Show when={total() > 0}>
                  <span class="text-muted tabular-nums text-[10px]">{share.toFixed(1)}%</span>
                </Show>
              </div>
            );
          }}
        </For>
      </div>
    </div>
  );
}

// SpamHistogram renders a small bar chart of count per spam-score
// bucket. The bucket label (eg "0", "1-3", "10+") sits below each bar;
// the count sits above on hover via title. Tones use the canonical
// metric severity palette: 0 = clean = normal, mid-score = warning,
// high-score = critical.
function SpamHistogram(props: { buckets: PMGSpamBucket[] }) {
  const maxCount = createMemo(() => Math.max(0, ...props.buckets.map((b) => Math.max(0, b.count))));
  return (
    <div class="flex items-end gap-1 h-16" role="img" aria-label="Spam score distribution">
      <For each={props.buckets}>
        {(bucket) => {
          const heightPct = maxCount() > 0 ? (bucket.count / maxCount()) * 100 : 0;
          const tone =
            bucket.score === '0' || bucket.score.startsWith('0')
              ? 'bg-metric-normal-bg'
              : Number(bucket.score) >= 10 || bucket.score.includes('10')
                ? 'bg-metric-critical-bg'
                : Number(bucket.score) >= 5 || bucket.score.includes('5')
                  ? 'bg-metric-warning-bg'
                  : 'bg-blue-500/60';
          return (
            <div class="flex flex-1 min-w-[20px] flex-col items-center gap-1">
              <div
                class={`w-full rounded-sm ${tone}`}
                style={{ height: `${Math.max(heightPct, 2)}%` }}
                title={`Score ${bucket.score}: ${formatNumber(bucket.count)}`}
              />
              <span class="text-[10px] text-muted font-mono tabular-nums">{bucket.score}</span>
            </div>
          );
        }}
      </For>
    </div>
  );
}

// InOutBar shows paired values (e.g. mail in vs mail out) as two
// adjacent half-bars sharing a max scale. The label sits ABOVE the
// bar and the values sit ON the bar segments — matches the Workloads
// MetricBar convention of writing the value on the fill.
function InOutBar(props: {
  label: string;
  inValue: number;
  outValue: number;
  format?: (value: number) => string;
}) {
  const format = props.format ?? formatNumber;
  const max = Math.max(props.inValue, props.outValue, 1);
  const inWidth = (props.inValue / max) * 50;
  const outWidth = (props.outValue / max) * 50;
  return (
    <div class="space-y-1">
      <div class="text-[11px] text-muted">{props.label}</div>
      <div
        class="relative flex h-4 w-full overflow-hidden rounded bg-surface-hover"
        title={`In ${format(props.inValue)} · Out ${format(props.outValue)}`}
      >
        <div class="relative bg-blue-500/60" style={{ width: `${inWidth}%` }} />
        <div class="w-px bg-surface" />
        <div class="relative bg-purple-500/60" style={{ width: `${outWidth}%` }} />
        <span class="pointer-events-none absolute inset-0 flex items-center justify-between px-1.5 text-[10px] font-semibold text-base-content leading-none tabular-nums">
          <span>In {format(props.inValue)}</span>
          <span>Out {format(props.outValue)}</span>
        </span>
      </div>
    </div>
  );
}

export const ProxmoxMailGatewayDrawer: Component<{
  instanceRow: Resource;
  onClose: () => void;
}> = (props) => {
  const id = () => {
    const meta = props.instanceRow.pmg;
    return asTrimmedString(meta?.instanceId) || props.instanceRow.id;
  };
  const [instance, { refetch }] = createResource<PMGInstance | null, string>(id, fetchPMGInstance);

  const stats = createMemo(() => instance()?.mailStats);
  const quarantine = createMemo(() => instance()?.quarantine);
  const nodes = createMemo<PMGNodeStatus[]>(() => instance()?.nodes ?? []);
  const spamBuckets = createMemo<PMGSpamBucket[]>(() => instance()?.spamDistribution ?? []);
  const topDomains = createMemo(() =>
    (instance()?.domainStats ?? [])
      .slice()
      .sort((a, b) => (b.mailCount ?? 0) - (a.mailCount ?? 0))
      .slice(0, 8),
  );
  const relayDomains = createMemo(() => instance()?.relayDomains ?? []);

  const health = () => classifyHealth(instance()?.status ?? props.instanceRow.status);
  const name = () =>
    asTrimmedString(instance()?.name) ||
    asTrimmedString(props.instanceRow.name) ||
    props.instanceRow.id;
  const version = () =>
    asTrimmedString(instance()?.version) || asTrimmedString(props.instanceRow.pmg?.version) || '—';
  const hostname = () => asTrimmedString(instance()?.host);

  const inboundSegments = createMemo<StackedSegment[]>(() => {
    const s = stats();
    if (!s) return [];
    const countIn = Math.max(0, s.countIn ?? 0);
    const spam = Math.max(0, s.spamIn ?? 0);
    const virus = Math.max(0, s.virusIn ?? 0);
    const bounces = Math.max(0, s.bouncesIn ?? 0);
    const greylist = Math.max(0, s.greylistCount ?? 0);
    const rbl = Math.max(0, s.rblRejects ?? 0);
    const pregreet = Math.max(0, s.pregreetRejects ?? 0);
    const flagged = spam + virus + bounces;
    const clean = Math.max(0, countIn - flagged);
    return [
      { key: 'clean', label: 'Clean', value: clean, tone: 'bg-metric-normal-bg' },
      { key: 'spam', label: 'Spam', value: spam, tone: 'bg-metric-warning-bg' },
      { key: 'virus', label: 'Virus', value: virus, tone: 'bg-metric-critical-bg' },
      { key: 'bounces', label: 'Bounces', value: bounces, tone: 'bg-orange-500/60' },
      { key: 'greylist', label: 'Greylist', value: greylist, tone: 'bg-blue-500/60' },
      { key: 'rbl', label: 'RBL', value: rbl, tone: 'bg-purple-500/60' },
      { key: 'pregreet', label: 'Pregreet', value: pregreet, tone: 'bg-slate-500/60' },
    ];
  });

  const quarantineSegments = createMemo<StackedSegment[]>(() => {
    const q = quarantine();
    if (!q) return [];
    return [
      { key: 'spam', label: 'Spam', value: q.spam ?? 0, tone: 'bg-metric-warning-bg' },
      { key: 'virus', label: 'Virus', value: q.virus ?? 0, tone: 'bg-metric-critical-bg' },
      {
        key: 'attachment',
        label: 'Attachment',
        value: q.attachment ?? 0,
        tone: 'bg-orange-500/60',
      },
      {
        key: 'blacklisted',
        label: 'Blacklisted',
        value: q.blacklisted ?? 0,
        tone: 'bg-slate-500/60',
      },
    ];
  });

  return (
    <div class="space-y-4">
      <header class="flex items-start justify-between gap-3">
        <div class="min-w-0 space-y-1">
          <div class="flex items-center gap-2 min-w-0">
            <StatusDot size="md" variant={health().variant} title={health().label} ariaHidden />
            <h3 class="truncate text-sm font-semibold text-base-content">{name()}</h3>
            <span class="shrink-0 text-[10px] font-mono text-muted">{version()}</span>
          </div>
          <Show when={hostname()}>
            <div class="font-mono text-[10px] text-muted break-all">{hostname()}</div>
          </Show>
        </div>
        <button
          type="button"
          onClick={props.onClose}
          class="shrink-0 inline-flex h-7 w-7 items-center justify-center rounded-sm text-muted hover:bg-surface-hover hover:text-base-content"
          aria-label="Close mail gateway drawer"
        >
          <XIcon class="h-4 w-4" />
        </button>
      </header>

      <Show
        when={!instance.error}
        fallback={
          <Card padding="md">
            <p class="text-xs text-red-600 dark:text-red-300">
              Failed to load Mail Gateway detail.
            </p>
            <button
              type="button"
              onClick={() => void refetch()}
              class="mt-2 inline-flex min-h-9 items-center rounded-md border border-border px-3 text-sm font-medium hover:bg-surface-hover"
            >
              Retry
            </button>
          </Card>
        }
      >
        <Show
          when={instance() !== undefined}
          fallback={<p class="text-xs text-muted">Loading Mail Gateway detail…</p>}
        >
          <Show
            when={instance()}
            fallback={<p class="text-xs text-muted">No detail available for this instance.</p>}
          >
            <Card padding="md">
              <div class="mb-2 flex items-baseline justify-between">
                <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">
                  Inbound disposition
                </h4>
                <span class="text-[10px] text-muted">{stats()?.timeframe ?? ''}</span>
              </div>
              <Show
                when={stats()}
                fallback={<p class="text-xs text-muted">No mail stats reported.</p>}
              >
                <StackedBar
                  segments={inboundSegments()}
                  ariaLabel="Inbound mail disposition breakdown"
                />
                <div class="mt-3 grid grid-cols-2 gap-x-4 gap-y-1 text-[11px] sm:grid-cols-4">
                  <InOutBar
                    label="Mail in / out"
                    inValue={stats()!.countIn ?? 0}
                    outValue={stats()!.countOut ?? 0}
                  />
                  <InOutBar
                    label="Spam in / out"
                    inValue={stats()!.spamIn ?? 0}
                    outValue={stats()!.spamOut ?? 0}
                  />
                  <InOutBar
                    label="Virus in / out"
                    inValue={stats()!.virusIn ?? 0}
                    outValue={stats()!.virusOut ?? 0}
                  />
                  <InOutBar
                    label="Bounces in / out"
                    inValue={stats()!.bouncesIn ?? 0}
                    outValue={stats()!.bouncesOut ?? 0}
                  />
                  <InOutBar
                    label="Bytes in / out"
                    inValue={stats()!.bytesIn ?? 0}
                    outValue={stats()!.bytesOut ?? 0}
                    format={(v) => (v > 0 ? formatBytes(v) : '—')}
                  />
                  <div class="space-y-1">
                    <div class="flex items-baseline justify-between text-[11px]">
                      <span class="text-muted">Greylist / Junk in</span>
                      <span class="font-mono text-[10px] text-base-content font-semibold">
                        {formatNumber(stats()!.greylistCount)} / {formatNumber(stats()!.junkIn)}
                      </span>
                    </div>
                  </div>
                  <div class="space-y-1">
                    <div class="flex items-baseline justify-between text-[11px]">
                      <span class="text-muted">RBL / Pregreet rejects</span>
                      <span class="font-mono text-[10px] text-base-content font-semibold">
                        {formatNumber(stats()!.rblRejects)} /{' '}
                        {formatNumber(stats()!.pregreetRejects)}
                      </span>
                    </div>
                  </div>
                  <div class="space-y-1">
                    <div class="flex items-baseline justify-between text-[11px]">
                      <span class="text-muted">Avg process</span>
                      <span class="font-mono text-[10px] text-base-content font-semibold">
                        {stats()?.averageProcessTimeMs
                          ? `${Math.round(stats()!.averageProcessTimeMs)}ms`
                          : '—'}
                      </span>
                    </div>
                  </div>
                </div>
              </Show>
            </Card>

            <div class="grid gap-3 lg:grid-cols-2">
              <Card padding="md">
                <div class="mb-2 flex items-baseline justify-between">
                  <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">
                    Quarantine
                  </h4>
                </div>
                <Show
                  when={quarantine()}
                  fallback={<p class="text-xs text-muted">No quarantine data reported.</p>}
                >
                  <StackedBar
                    segments={quarantineSegments()}
                    ariaLabel="Quarantine breakdown by category"
                  />
                </Show>
              </Card>

              <Card padding="md">
                <div class="mb-2 flex items-baseline justify-between">
                  <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">
                    Spam score distribution
                  </h4>
                  <span class="text-[10px] text-muted tabular-nums">
                    {spamBuckets().length} bucket{spamBuckets().length === 1 ? '' : 's'}
                  </span>
                </div>
                <Show
                  when={spamBuckets().length > 0}
                  fallback={<p class="text-xs text-muted">No spam score data.</p>}
                >
                  <SpamHistogram buckets={spamBuckets()} />
                </Show>
              </Card>
            </div>

            <Card padding="md">
              <div class="mb-2 flex items-baseline justify-between">
                <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">
                  Cluster nodes
                </h4>
                <span class="text-[10px] text-muted tabular-nums">
                  {nodes().length} node{nodes().length === 1 ? '' : 's'}
                </span>
              </div>
              <Show
                when={nodes().length > 0}
                fallback={<p class="text-xs text-muted">No cluster nodes reported.</p>}
              >
                <Table class="text-xs">
                  <TableHeader>
                    <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                      <TableHead class={getPlatformTableHeadClassForKind('name')}>Node</TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('text')}>Role</TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                        Uptime
                      </TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                        Load
                      </TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                        Queue
                      </TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                        Oldest
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                    <For each={nodes()}>
                      {(node) => {
                        const cls = classifyNode(node);
                        const queue = queueCell(node.queueStatus);
                        return (
                          <TableRow>
                            <TableCell class={getPlatformTableCellClassForKind('name')}>
                              <div class="flex items-center gap-2">
                                <StatusDot
                                  size="sm"
                                  variant={cls.variant}
                                  title={cls.label}
                                  ariaHidden
                                />
                                <span class="font-mono text-[11px] font-semibold text-base-content">
                                  {node.name || '—'}
                                </span>
                              </div>
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} text-base-content text-[11px]`}
                            >
                              {asTrimmedString(node.role) || '—'}
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                            >
                              {formatUptime(node.uptime)}
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums text-[11px]`}
                            >
                              {asTrimmedString(node.loadAvg) || '—'}
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} tabular-nums`}
                            >
                              <span class="text-base-content font-semibold">{queue.count}</span>
                              <span
                                class="ml-1 text-muted text-[10px] font-mono"
                                title="active/deferred/hold/incoming"
                              >
                                {queue.label}
                              </span>
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                            >
                              {node.queueStatus?.oldestAge
                                ? formatAge(node.queueStatus.oldestAge)
                                : '—'}
                            </TableCell>
                          </TableRow>
                        );
                      }}
                    </For>
                  </TableBody>
                </Table>
              </Show>
            </Card>

            <Card padding="md">
              <div class="mb-2 flex items-baseline justify-between">
                <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">
                  Top domains
                </h4>
                <span class="text-[10px] text-muted tabular-nums">top {topDomains().length}</span>
              </div>
              <Show
                when={topDomains().length > 0}
                fallback={<p class="text-xs text-muted">No domain stats reported.</p>}
              >
                <Table class="text-xs">
                  <TableHeader>
                    <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                      <TableHead class={getPlatformTableHeadClassForKind('name')}>Domain</TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                        Mail
                      </TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                        Spam
                      </TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                        Virus
                      </TableHead>
                      <TableHead class={getPlatformTableHeadClassForKind('numeric-value')}>
                        Bytes
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                    <For each={topDomains()}>
                      {(domain) => (
                        <TableRow>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('name')} font-mono text-[11px] text-base-content`}
                          >
                            <span class="inline-block max-w-[14rem] truncate" title={domain.domain}>
                              {domain.domain || '—'}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                          >
                            {formatNumber(domain.mailCount)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                          >
                            {formatNumber(domain.spamCount)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                          >
                            {formatNumber(domain.virusCount)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-muted tabular-nums`}
                          >
                            {domain.bytes && domain.bytes > 0 ? formatBytes(domain.bytes) : '—'}
                          </TableCell>
                        </TableRow>
                      )}
                    </For>
                  </TableBody>
                </Table>
              </Show>
            </Card>

            <Show when={relayDomains().length > 0}>
              <div class="flex flex-wrap items-center gap-1">
                <span class="text-[10px] uppercase tracking-wide text-muted">Relay domains</span>
                <For each={relayDomains()}>
                  {(rd) => (
                    <span
                      class="inline-flex items-center rounded-sm bg-surface-alt px-1.5 py-0.5 text-[10px] font-mono text-base-content"
                      title={rd.comment || rd.domain}
                    >
                      {rd.domain}
                    </span>
                  )}
                </For>
              </div>
            </Show>
          </Show>
        </Show>
      </Show>
    </div>
  );
};

export default ProxmoxMailGatewayDrawer;
