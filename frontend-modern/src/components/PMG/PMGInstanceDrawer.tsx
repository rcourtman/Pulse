import type { Component } from 'solid-js';
import { For, Show, createMemo, createResource, createSignal } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { formatBytes, formatRelativeTime } from '@/utils/format';

type PMGNodeStatus = {
  name: string;
  status: string;
  role?: string;
  uptime?: number;
  loadAvg?: string;
  queueStatus?: {
    active?: number;
    deferred?: number;
    hold?: number;
    incoming?: number;
    total?: number;
  };
};

type PMGMailStats = {
  timeframe?: string;
  countIn?: number;
  countOut?: number;
  spamIn?: number;
  spamOut?: number;
  virusIn?: number;
  virusOut?: number;
  bouncesIn?: number;
  bouncesOut?: number;
  bytesIn?: number;
  bytesOut?: number;
  greylistCount?: number;
  rblRejects?: number;
  averageProcessTimeMs?: number;
};

type PMGQuarantine = {
  spam?: number;
  virus?: number;
  attachment?: number;
  blacklisted?: number;
};

type PMGSpamBucket = {
  bucket: string;
  count: number;
};

type PMGRelayDomain = {
  domain: string;
  comment?: string;
};

type PMGDomainStat = {
  domain: string;
  mailCount: number;
  spamCount: number;
  virusCount: number;
  bytes?: number;
};

type PMGData = {
  instanceId?: string;
  hostname?: string;
  version?: string;
  nodeCount?: number;
  uptimeSeconds?: number;
  queueActive?: number;
  queueDeferred?: number;
  queueHold?: number;
  queueIncoming?: number;
  queueTotal?: number;
  mailCountTotal?: number;
  spamIn?: number;
  virusIn?: number;
  connectionHealth?: string;
  lastUpdated?: string;

  nodes?: PMGNodeStatus[];
  mailStats?: PMGMailStats;
  quarantine?: PMGQuarantine;
  spamDistribution?: PMGSpamBucket[];
  relayDomains?: PMGRelayDomain[];
  domainStats?: PMGDomainStat[];
  domainStatsAsOf?: string;
};

type UnifiedResourceResponse = {
  id: string;
  type: string;
  name?: string;
  status?: string;
  lastSeen?: string;
  customUrl?: string;
  pmg?: PMGData;
};

type PMGInstanceDrawerProps = {
  resourceId: string;
  resourceName?: string;
};

const normalize = (value?: string | null) => (value || '').trim();

const formatCompact = (value?: number | null): string => {
  if (value === undefined || value === null || Number.isNaN(value)) return '—';
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}K`;
  return Math.round(value).toLocaleString();
};

const statusTone = (status?: string) => {
  const normalized = (status || '').trim().toLowerCase();
  if (!normalized) return 'bg-gray-400';
  if (normalized === 'online' || normalized === 'healthy') return 'bg-emerald-500';
  if (normalized === 'warning' || normalized === 'degraded') return 'bg-amber-500';
  if (normalized === 'offline' || normalized === 'error') return 'bg-rose-500';
  return 'bg-gray-400';
};

export const PMGInstanceDrawer: Component<PMGInstanceDrawerProps> = (props) => {
  const [searchRelay, setSearchRelay] = createSignal('');
  const [searchDomain, setSearchDomain] = createSignal('');

  const resourceId = createMemo(() => normalize(props.resourceId));

  const [resource] = createResource(
    resourceId,
    async (id) => {
      if (!id) return null;
      return apiFetchJSON<UnifiedResourceResponse>(`/api/resources/${encodeURIComponent(id)}`, { cache: 'no-store' });
    },
    { initialValue: null },
  );

  const pmg = createMemo(() => resource()?.pmg ?? null);

  const lastUpdatedRelative = createMemo(() => {
    const raw = pmg()?.lastUpdated;
    if (!raw) return '';
    const parsed = Date.parse(raw);
    if (!Number.isFinite(parsed)) return '';
    return formatRelativeTime(parsed);
  });

  const domainStatsAsOfRelative = createMemo(() => {
    const raw = pmg()?.domainStatsAsOf;
    if (!raw) return '';
    const parsed = Date.parse(raw);
    if (!Number.isFinite(parsed)) return '';
    return formatRelativeTime(parsed);
  });

  const relayDomains = createMemo(() => {
    const rows = pmg()?.relayDomains ?? [];
    const term = normalize(searchRelay()).toLowerCase();
    if (!term) return rows;
    return rows.filter((row) =>
      row.domain.toLowerCase().includes(term) || (row.comment || '').toLowerCase().includes(term),
    );
  });

  const domainStats = createMemo(() => {
    const rows = pmg()?.domainStats ?? [];
    const term = normalize(searchDomain()).toLowerCase();
    const filtered = term
      ? rows.filter((row) => row.domain.toLowerCase().includes(term))
      : rows;
    return [...filtered].sort((a, b) => (b.mailCount || 0) - (a.mailCount || 0));
  });

  const spamBuckets = createMemo(() => {
    const rows = pmg()?.spamDistribution ?? [];
    const parsed = rows
      .map((row) => ({ bucket: normalize(row.bucket), count: Number(row.count || 0) }))
      .filter((row) => row.bucket.length > 0);
    return parsed.sort((a, b) => a.bucket.localeCompare(b.bucket));
  });

  const maxSpamBucketCount = createMemo(() => Math.max(0, ...spamBuckets().map((row) => row.count)));

  return (
    <div class="space-y-3">
      <Show when={!resource.loading} fallback={
        <Card padding="lg">
          <EmptyState title="Loading mail gateway details..." description="Fetching PMG resource details." />
        </Card>
      }>
        <Show when={pmg()} fallback={
          <Card padding="lg">
            <EmptyState
              title="No PMG details for this resource yet"
              description="Pulse hasn't ingested PMG analytics for this instance."
            />
          </Card>
        }>
          {(pmgData) => (
            <>
              <Card padding="lg">
                <div class="flex items-start justify-between gap-4">
                  <div class="min-w-0">
                    <div class="text-sm font-semibold text-gray-900 dark:text-gray-100 truncate">
                      {normalize(props.resourceName) || resource()?.name || 'Mail Gateway'}
                    </div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      {pmgData().hostname || 'Unknown host'}
                      <Show when={pmgData().version}>
                        <span class="mx-2 text-gray-300 dark:text-gray-700">|</span>
                        <span>v{pmgData().version}</span>
                      </Show>
                    </div>
                    <Show when={lastUpdatedRelative()}>
                      <div class="mt-1 text-[11px] text-gray-500 dark:text-gray-400">
                        Updated {lastUpdatedRelative()}
                      </div>
                    </Show>
                  </div>

                  <div class="shrink-0 flex items-center gap-2">
                    <span class={`inline-block h-2.5 w-2.5 rounded-full ${statusTone(resource()?.status)}`} />
                    <span class="text-xs font-medium text-gray-700 dark:text-gray-200">
                      {(resource()?.status || 'unknown').toLowerCase()}
                    </span>
                    <Show when={resource()?.customUrl}>
                      {(url) => (
                        <a
                          class="ml-2 text-xs font-medium text-blue-700 hover:text-blue-800 dark:text-blue-300 dark:hover:text-blue-200"
                          href={url()}
                          target="_blank"
                          rel="noreferrer"
                        >
                          Open
                        </a>
                      )}
                    </Show>
                  </div>
                </div>

                <div class="mt-4 grid grid-cols-2 gap-3 text-xs sm:grid-cols-4">
                  <div class="rounded border border-gray-200 bg-white px-3 py-2 dark:border-gray-700 dark:bg-gray-900/30">
                    <div class="text-[10px] uppercase tracking-wide text-gray-500 dark:text-gray-400">Queue Total</div>
                    <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">
                      {formatCompact(pmgData().queueTotal ?? 0)}
                    </div>
                  </div>
                  <div class="rounded border border-gray-200 bg-white px-3 py-2 dark:border-gray-700 dark:bg-gray-900/30">
                    <div class="text-[10px] uppercase tracking-wide text-gray-500 dark:text-gray-400">Deferred</div>
                    <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">
                      {formatCompact(pmgData().queueDeferred ?? 0)}
                    </div>
                  </div>
                  <div class="rounded border border-gray-200 bg-white px-3 py-2 dark:border-gray-700 dark:bg-gray-900/30">
                    <div class="text-[10px] uppercase tracking-wide text-gray-500 dark:text-gray-400">Mail</div>
                    <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">
                      {formatCompact(pmgData().mailCountTotal ?? 0)}
                    </div>
                  </div>
                  <div class="rounded border border-gray-200 bg-white px-3 py-2 dark:border-gray-700 dark:bg-gray-900/30">
                    <div class="text-[10px] uppercase tracking-wide text-gray-500 dark:text-gray-400">Spam/Virus</div>
                    <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">
                      {formatCompact(pmgData().spamIn ?? 0)} / {formatCompact(pmgData().virusIn ?? 0)}
                    </div>
                  </div>
                </div>
              </Card>

              <Show when={(pmgData().nodes || []).length > 0}>
                <Card padding="lg">
                  <div class="text-xs font-semibold text-gray-900 dark:text-gray-100">Nodes</div>
                  <div class="mt-2 overflow-x-auto">
                    <table class="min-w-full text-xs">
                      <thead class="text-[10px] uppercase tracking-wide text-gray-500 dark:text-gray-400">
                        <tr>
                          <th class="text-left py-2 pr-3">Node</th>
                          <th class="text-left py-2 pr-3">Role</th>
                          <th class="text-left py-2 pr-3">Status</th>
                          <th class="text-right py-2 pl-3">Queue</th>
                        </tr>
                      </thead>
                      <tbody class="divide-y divide-gray-100 dark:divide-gray-800">
                        <For each={pmgData().nodes || []}>
                          {(node) => (
                            <tr>
                              <td class="py-2 pr-3 font-medium text-gray-900 dark:text-gray-100">{node.name}</td>
                              <td class="py-2 pr-3 text-gray-600 dark:text-gray-300">{node.role || '—'}</td>
                              <td class="py-2 pr-3 text-gray-600 dark:text-gray-300">{node.status || '—'}</td>
                              <td class="py-2 pl-3 text-right text-gray-600 dark:text-gray-300">
                                {formatCompact(node.queueStatus?.total ?? 0)}
                              </td>
                            </tr>
                          )}
                        </For>
                      </tbody>
                    </table>
                  </div>
                </Card>
              </Show>

              <Show when={(pmgData().relayDomains || []).length > 0}>
                <Card padding="lg">
                  <div class="flex items-center justify-between gap-3">
                    <div class="text-xs font-semibold text-gray-900 dark:text-gray-100">Relay Domains</div>
                    <input
                      value={searchRelay()}
                      onInput={(e) => setSearchRelay(e.currentTarget.value)}
                      placeholder="Search domains..."
                      class="w-56 rounded border border-gray-200 bg-white px-2 py-1 text-xs text-gray-700 placeholder:text-gray-400 dark:border-gray-700 dark:bg-gray-900/30 dark:text-gray-200"
                    />
                  </div>
                  <div class="mt-2 overflow-x-auto">
                    <table class="min-w-full text-xs">
                      <thead class="text-[10px] uppercase tracking-wide text-gray-500 dark:text-gray-400">
                        <tr>
                          <th class="text-left py-2 pr-3">Domain</th>
                          <th class="text-left py-2 pr-3">Comment</th>
                        </tr>
                      </thead>
                      <tbody class="divide-y divide-gray-100 dark:divide-gray-800">
                        <For each={relayDomains()}>
                          {(row) => (
                            <tr>
                              <td class="py-2 pr-3 font-medium text-gray-900 dark:text-gray-100">{row.domain}</td>
                              <td class="py-2 pr-3 text-gray-600 dark:text-gray-300">{row.comment || '—'}</td>
                            </tr>
                          )}
                        </For>
                      </tbody>
                    </table>
                  </div>
                </Card>
              </Show>

              <Show when={(pmgData().domainStats || []).length > 0}>
                <Card padding="lg">
                  <div class="flex items-center justify-between gap-3">
                    <div class="min-w-0">
                      <div class="text-xs font-semibold text-gray-900 dark:text-gray-100">Domain Stats</div>
                      <Show when={domainStatsAsOfRelative()}>
                        <div class="mt-0.5 text-[11px] text-gray-500 dark:text-gray-400">
                          As of {domainStatsAsOfRelative()}
                        </div>
                      </Show>
                    </div>
                    <input
                      value={searchDomain()}
                      onInput={(e) => setSearchDomain(e.currentTarget.value)}
                      placeholder="Search domains..."
                      class="w-56 rounded border border-gray-200 bg-white px-2 py-1 text-xs text-gray-700 placeholder:text-gray-400 dark:border-gray-700 dark:bg-gray-900/30 dark:text-gray-200"
                    />
                  </div>
                  <div class="mt-2 overflow-x-auto">
                    <table class="min-w-full text-xs">
                      <thead class="text-[10px] uppercase tracking-wide text-gray-500 dark:text-gray-400">
                        <tr>
                          <th class="text-left py-2 pr-3">Domain</th>
                          <th class="text-right py-2 pl-3">Mail</th>
                          <th class="text-right py-2 pl-3">Spam</th>
                          <th class="text-right py-2 pl-3">Virus</th>
                          <th class="text-right py-2 pl-3">Bytes</th>
                        </tr>
                      </thead>
                      <tbody class="divide-y divide-gray-100 dark:divide-gray-800">
                        <For each={domainStats()}>
                          {(row) => (
                            <tr>
                              <td class="py-2 pr-3 font-medium text-gray-900 dark:text-gray-100">{row.domain}</td>
                              <td class="py-2 pl-3 text-right text-gray-600 dark:text-gray-300">{formatCompact(row.mailCount)}</td>
                              <td class="py-2 pl-3 text-right text-gray-600 dark:text-gray-300">{formatCompact(row.spamCount)}</td>
                              <td class="py-2 pl-3 text-right text-gray-600 dark:text-gray-300">{formatCompact(row.virusCount)}</td>
                              <td class="py-2 pl-3 text-right text-gray-600 dark:text-gray-300">
                                {row.bytes ? formatBytes(row.bytes) : '—'}
                              </td>
                            </tr>
                          )}
                        </For>
                      </tbody>
                    </table>
                  </div>
                </Card>
              </Show>

              <Show when={spamBuckets().length > 0}>
                <Card padding="lg">
                  <div class="text-xs font-semibold text-gray-900 dark:text-gray-100">Spam Distribution</div>
                  <div class="mt-3 space-y-2">
                    <For each={spamBuckets()}>
                      {(bucket) => (
                        <div class="flex items-center gap-3">
                          <div class="w-16 text-[11px] font-medium text-gray-600 dark:text-gray-300">
                            {bucket.bucket}
                          </div>
                          <div class="flex-1 h-2 rounded bg-gray-100 dark:bg-gray-800 overflow-hidden">
                            <div
                              class="h-full bg-amber-500"
                              style={{
                                width: `${maxSpamBucketCount() > 0 ? Math.round((bucket.count / maxSpamBucketCount()) * 100) : 0}%`,
                              }}
                            />
                          </div>
                          <div class="w-14 text-right text-[11px] text-gray-600 dark:text-gray-300">
                            {formatCompact(bucket.count)}
                          </div>
                        </div>
                      )}
                    </For>
                  </div>
                </Card>
              </Show>
            </>
          )}
        </Show>
      </Show>
    </div>
  );
};

export default PMGInstanceDrawer;

