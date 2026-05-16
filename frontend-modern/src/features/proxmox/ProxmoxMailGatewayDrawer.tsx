import {
  For,
  Show,
  createMemo,
  createResource,
  type Component,
} from 'solid-js';
import XIcon from 'lucide-solid/icons/x';
import { Card } from '@/components/shared/Card';
import { StatusDot } from '@/components/shared/StatusDot';
import type { StatusIndicatorVariant } from '@/utils/status';
import { formatBytes } from '@/utils/format';
import { asTrimmedString } from '@/utils/stringUtils';
import { apiFetch } from '@/utils/apiClient';
import type {
  PMGInstance,
  PMGNodeStatus,
  PMGQueueStatus,
} from '@/types/api';
import type { Resource } from '@/types/resource';

// Inline drawer for a single Proxmox Mail Gateway instance. The row
// table only exposes the slim ResourcePMGMeta projection (totals
// only), but the backend State carries per-node cluster status with
// individual postfix queue detail, full mail stats (in/out, bytes,
// bounces, RBL/pregreet), quarantine-by-category, spam score
// distribution, top-domain stats, and configured relay domains. Fetch
// the full PMGInstance on first open from /api/pmg/instances so the
// row stays cheap and the drawer is rich.

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

function StatTile(props: { label: string; value: string | number; sub?: string }) {
  return (
    <div class="min-w-0 rounded-sm border border-border bg-surface-alt px-3 py-2">
      <div class="truncate text-[10px] uppercase tracking-wide text-muted">{props.label}</div>
      <div class="mt-0.5 flex items-baseline gap-1 truncate text-base font-semibold text-base-content tabular-nums">
        <span class="truncate">{props.value}</span>
        <Show when={props.sub}>
          <span class="truncate text-[10px] font-normal text-muted">{props.sub}</span>
        </Show>
      </div>
    </div>
  );
}

function QueueDot(props: { count: number }) {
  const tone =
    props.count === 0
      ? 'muted'
      : props.count > 50
        ? 'danger'
        : props.count > 10
          ? 'warning'
          : 'success';
  return <StatusDot size="xs" variant={tone as StatusIndicatorVariant} ariaHidden />;
}

function queueCell(queue: PMGQueueStatus | undefined): { count: number; label: string } {
  if (!queue) return { count: 0, label: '—' };
  return {
    count: queue.total,
    label: `${queue.active}/${queue.deferred}/${queue.hold}/${queue.incoming}`,
  };
}

export const ProxmoxMailGatewayDrawer: Component<{
  instanceRow: Resource;
  onClose: () => void;
}> = (props) => {
  const id = () => {
    const meta = props.instanceRow.pmg;
    return asTrimmedString(meta?.instanceId) || props.instanceRow.id;
  };
  const [instance, { refetch }] = createResource<PMGInstance | null, string>(
    id,
    fetchPMGInstance,
  );

  const stats = createMemo(() => instance()?.mailStats);
  const quarantine = createMemo(() => instance()?.quarantine);
  const nodes = createMemo<PMGNodeStatus[]>(() => instance()?.nodes ?? []);
  const spamBuckets = createMemo(() => instance()?.spamDistribution ?? []);
  const topDomains = createMemo(() =>
    (instance()?.domainStats ?? [])
      .slice()
      .sort((a, b) => (b.mailCount ?? 0) - (a.mailCount ?? 0))
      .slice(0, 8),
  );
  const relayDomains = createMemo(() => instance()?.relayDomains ?? []);

  const health = () => classifyHealth(instance()?.status ?? props.instanceRow.status);
  const name = () =>
    asTrimmedString(instance()?.name) || asTrimmedString(props.instanceRow.name) || props.instanceRow.id;
  const version = () => asTrimmedString(instance()?.version) || asTrimmedString(props.instanceRow.pmg?.version) || '—';
  const hostname = () => asTrimmedString(instance()?.host);
  const totalQueue = createMemo(() =>
    nodes().reduce((sum, n) => sum + (n.queueStatus?.total ?? 0), 0),
  );

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
            <div class="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-6">
              <StatTile label="Nodes" value={nodes().length} />
              <StatTile
                label="Mail in (24h)"
                value={formatNumber(stats()?.countIn)}
              />
              <StatTile label="Mail out (24h)" value={formatNumber(stats()?.countOut)} />
              <StatTile label="Spam in" value={formatNumber(stats()?.spamIn)} />
              <StatTile label="Virus in" value={formatNumber(stats()?.virusIn)} />
              <StatTile label="Queue total" value={totalQueue()} />
            </div>

            <div class="grid gap-3 lg:grid-cols-2">
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
                  <table class="w-full text-xs">
                    <thead class="text-[10px] uppercase tracking-wide text-muted">
                      <tr>
                        <th class="pb-2 text-left font-medium">Node</th>
                        <th class="pb-2 text-left font-medium">Role</th>
                        <th class="pb-2 text-right font-medium">Uptime</th>
                        <th class="pb-2 text-right font-medium">Load</th>
                        <th class="pb-2 text-right font-medium">Queue</th>
                        <th class="pb-2 text-right font-medium">Oldest</th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-border-subtle">
                      <For each={nodes()}>
                        {(node) => {
                          const cls = classifyNode(node);
                          const queue = queueCell(node.queueStatus);
                          return (
                            <tr>
                              <td class="py-2">
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
                              </td>
                              <td class="py-2 text-base-content text-[11px]">
                                {asTrimmedString(node.role) || '—'}
                              </td>
                              <td class="py-2 text-right text-base-content">
                                {formatUptime(node.uptime)}
                              </td>
                              <td class="py-2 text-right text-base-content tabular-nums text-[11px]">
                                {asTrimmedString(node.loadAvg) || '—'}
                              </td>
                              <td class="py-2 text-right">
                                <div class="inline-flex items-center justify-end gap-1.5 tabular-nums">
                                  <QueueDot count={queue.count} />
                                  <span class="text-base-content font-semibold">{queue.count}</span>
                                  <span
                                    class="text-muted text-[10px] font-mono"
                                    title="active/deferred/hold/incoming"
                                  >
                                    {queue.label}
                                  </span>
                                </div>
                              </td>
                              <td class="py-2 text-right text-base-content">
                                {node.queueStatus?.oldestAge
                                  ? formatAge(node.queueStatus.oldestAge)
                                  : '—'}
                              </td>
                            </tr>
                          );
                        }}
                      </For>
                    </tbody>
                  </table>
                </Show>
              </Card>

              <Card padding="md">
                <div class="mb-2 flex items-baseline justify-between">
                  <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">
                    Quarantine
                  </h4>
                  <span class="text-[10px] text-muted tabular-nums">
                    {formatNumber(
                      (quarantine()?.spam ?? 0) +
                        (quarantine()?.virus ?? 0) +
                        (quarantine()?.attachment ?? 0) +
                        (quarantine()?.blacklisted ?? 0),
                    )}{' '}
                    total
                  </span>
                </div>
                <Show
                  when={quarantine()}
                  fallback={<p class="text-xs text-muted">No quarantine data reported.</p>}
                >
                  <div class="grid grid-cols-2 gap-2">
                    <StatTile label="Spam" value={formatNumber(quarantine()?.spam)} />
                    <StatTile label="Virus" value={formatNumber(quarantine()?.virus)} />
                    <StatTile label="Attachment" value={formatNumber(quarantine()?.attachment)} />
                    <StatTile
                      label="Blacklisted"
                      value={formatNumber(quarantine()?.blacklisted)}
                    />
                  </div>
                </Show>
                <div class="mt-3">
                  <h5 class="mb-1 text-[10px] uppercase tracking-wide text-muted">
                    Spam score distribution
                  </h5>
                  <Show
                    when={spamBuckets().length > 0}
                    fallback={<p class="text-xs text-muted">No spam score data.</p>}
                  >
                    <div class="flex flex-wrap gap-1">
                      <For each={spamBuckets()}>
                        {(bucket) => (
                          <span class="inline-flex items-center gap-1 rounded-sm bg-surface-alt px-1.5 py-0.5 text-[10px] font-mono tabular-nums">
                            <span class="text-muted">{bucket.score}</span>
                            <span class="text-base-content font-semibold">
                              {formatNumber(bucket.count)}
                            </span>
                          </span>
                        )}
                      </For>
                    </div>
                  </Show>
                </div>
              </Card>
            </div>

            <div class="grid gap-3 lg:grid-cols-2">
              <Card padding="md">
                <div class="mb-2 flex items-baseline justify-between">
                  <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">
                    Top domains
                  </h4>
                  <span class="text-[10px] text-muted tabular-nums">
                    top {topDomains().length}
                  </span>
                </div>
                <Show
                  when={topDomains().length > 0}
                  fallback={<p class="text-xs text-muted">No domain stats reported.</p>}
                >
                  <table class="w-full text-xs">
                    <thead class="text-[10px] uppercase tracking-wide text-muted">
                      <tr>
                        <th class="pb-2 text-left font-medium">Domain</th>
                        <th class="pb-2 text-right font-medium">Mail</th>
                        <th class="pb-2 text-right font-medium">Spam</th>
                        <th class="pb-2 text-right font-medium">Virus</th>
                        <th class="pb-2 text-right font-medium">Bytes</th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-border-subtle">
                      <For each={topDomains()}>
                        {(domain) => (
                          <tr>
                            <td class="py-2 font-mono text-[11px] text-base-content truncate max-w-[14rem]" title={domain.domain}>
                              {domain.domain || '—'}
                            </td>
                            <td class="py-2 text-right text-base-content tabular-nums">
                              {formatNumber(domain.mailCount)}
                            </td>
                            <td class="py-2 text-right text-base-content tabular-nums">
                              {formatNumber(domain.spamCount)}
                            </td>
                            <td class="py-2 text-right text-base-content tabular-nums">
                              {formatNumber(domain.virusCount)}
                            </td>
                            <td class="py-2 text-right text-muted tabular-nums">
                              {domain.bytes && domain.bytes > 0
                                ? formatBytes(domain.bytes)
                                : '—'}
                            </td>
                          </tr>
                        )}
                      </For>
                    </tbody>
                  </table>
                </Show>
              </Card>

              <Card padding="md">
                <div class="mb-2 flex items-baseline justify-between">
                  <h4 class="text-xs font-semibold uppercase tracking-wide text-muted">
                    Mail flow detail
                  </h4>
                  <Show when={stats()?.timeframe}>
                    <span class="text-[10px] text-muted">{stats()?.timeframe}</span>
                  </Show>
                </div>
                <Show
                  when={stats()}
                  fallback={<p class="text-xs text-muted">No mail stats reported.</p>}
                >
                  <div class="grid grid-cols-2 gap-2 text-xs sm:grid-cols-3">
                    <StatTile label="Bounces in" value={formatNumber(stats()?.bouncesIn)} />
                    <StatTile label="Bounces out" value={formatNumber(stats()?.bouncesOut)} />
                    <StatTile label="Greylist" value={formatNumber(stats()?.greylistCount)} />
                    <StatTile label="Junk in" value={formatNumber(stats()?.junkIn)} />
                    <StatTile label="RBL rejects" value={formatNumber(stats()?.rblRejects)} />
                    <StatTile
                      label="Pregreet rejects"
                      value={formatNumber(stats()?.pregreetRejects)}
                    />
                    <StatTile
                      label="Bytes in"
                      value={
                        stats()?.bytesIn && stats()!.bytesIn > 0
                          ? formatBytes(stats()!.bytesIn)
                          : '—'
                      }
                    />
                    <StatTile
                      label="Bytes out"
                      value={
                        stats()?.bytesOut && stats()!.bytesOut > 0
                          ? formatBytes(stats()!.bytesOut)
                          : '—'
                      }
                    />
                    <StatTile
                      label="Avg process"
                      value={
                        stats()?.averageProcessTimeMs
                          ? `${Math.round(stats()!.averageProcessTimeMs)}ms`
                          : '—'
                      }
                    />
                  </div>
                </Show>
              </Card>
            </div>

            <Show when={relayDomains().length > 0}>
              <Card padding="md">
                <h4 class="mb-2 text-xs font-semibold uppercase tracking-wide text-muted">
                  Relay domains
                </h4>
                <div class="flex flex-wrap gap-1">
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
              </Card>
            </Show>
          </Show>
        </Show>
      </Show>
    </div>
  );
};

export default ProxmoxMailGatewayDrawer;
