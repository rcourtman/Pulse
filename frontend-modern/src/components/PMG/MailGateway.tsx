import { Component, Show, For, createMemo } from 'solid-js';
import { useWebSocket } from '@/App';
import { ProxmoxSectionNav } from '@/components/Proxmox/ProxmoxSectionNav';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { formatRelativeTime } from '@/utils/format';

const formatNum = (value?: number | null) => {
  if (value === undefined || value === null || Number.isNaN(value)) return '—';
  return Math.round(value).toLocaleString();
};

const formatDec = (value?: number | null, digits = 1) => {
  if (value === undefined || value === null || Number.isNaN(value)) return '—';
  return value.toFixed(digits);
};

const parseTimestamp = (value?: string) => {
  if (!value) return undefined;
  const time = Date.parse(value);
  return Number.isNaN(time) ? undefined : time;
};

const MailGateway: Component = () => {
  const { state, connected, reconnecting, reconnect } = useWebSocket();
  const instances = createMemo(() => state.pmg ?? []);

  return (
    <div class="space-y-3">
      <ProxmoxSectionNav current="mail" />

      <Show when={!connected()}>
        <Card padding="lg" tone="danger">
          <EmptyState
            icon={
              <svg class="h-12 w-12 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            }
            title="Connection lost"
            description={
              reconnecting()
                ? 'Attempting to reconnect…'
                : 'Unable to connect to the backend server'
            }
            tone="danger"
            actions={
              !reconnecting() ? (
                <button
                  onClick={() => reconnect()}
                  class="mt-2 inline-flex items-center px-4 py-2 text-xs font-medium rounded bg-red-600 text-white hover:bg-red-700 transition-colors"
                >
                  Reconnect now
                </button>
              ) : undefined
            }
          />
        </Card>
      </Show>

      <Show when={connected()}>
        <Show when={instances().length === 0}>
          <Card padding="lg">
            <EmptyState
              icon={
                <svg class="h-12 w-12 text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A9 9 0 105.64 5.64a9 9 0 0014.978 0z" />
                </svg>
              }
              title="No Mail Gateways configured"
              description="Add a Proxmox Mail Gateway via Settings → Nodes to start collecting mail analytics."
            />
          </Card>
        </Show>

        <For each={instances()}>
          {(pmg) => {
            const statusTone = (pmg.connectionHealth || pmg.status || '').toLowerCase();
            const isHealthy = statusTone.includes('healthy') || (pmg.status || '').toLowerCase() === 'online';
            const isWarning = statusTone.includes('degraded') || statusTone.includes('warning');
            const dotClass = isHealthy ? 'bg-green-500' : isWarning ? 'bg-amber-500' : 'bg-red-500';

            const lastUpdated = () => {
              const ts = parseTimestamp(pmg.mailStats?.updatedAt) ?? parseTimestamp(pmg.lastUpdated) ?? parseTimestamp(pmg.lastSeen);
              return ts ? formatRelativeTime(ts) : '—';
            };

            const total = pmg.mailStats?.countTotal || 0;
            const inbound = pmg.mailStats?.countIn || 0;
            const outbound = pmg.mailStats?.countOut || 0;
            const spam = pmg.mailStats?.spamIn || 0;
            const virus = pmg.mailStats?.virusIn || 0;
            const bouncesIn = pmg.mailStats?.bouncesIn || 0;
            const bouncesOut = pmg.mailStats?.bouncesOut || 0;
            const bytesIn = pmg.mailStats?.bytesIn || 0;
            const bytesOut = pmg.mailStats?.bytesOut || 0;
            const rbl = pmg.mailStats?.rblRejects || 0;
            const pregreet = pmg.mailStats?.pregreetRejects || 0;
            const greylist = pmg.mailStats?.greylistCount || 0;
            const junk = pmg.mailStats?.junkIn || 0;

            const qSpam = pmg.quarantine?.spam || 0;
            const qVirus = pmg.quarantine?.virus || 0;
            const qAttachment = pmg.quarantine?.attachment || 0;
            const qBlacklist = pmg.quarantine?.blacklisted || 0;
            const qTotal = qSpam + qVirus + qAttachment + qBlacklist;

            const spamPct = (spam / Math.max(total, 1)) * 100;
            const virusPct = (virus / Math.max(total, 1)) * 100;
            const inboundPct = (inbound / Math.max(total, 1)) * 100;
            const outboundPct = (outbound / Math.max(total, 1)) * 100;
            const rblPct = (rbl / Math.max(inbound, 1)) * 100;

            return (
              <Card padding="none" tone="glass" class="overflow-hidden">
                {/* Instance Header Strip */}
                <div class="sticky top-0 z-10 flex items-center justify-between px-3 py-2 bg-gray-50 dark:bg-gray-800/40 border-b border-gray-200 dark:border-gray-700">
                  <div class="flex items-center gap-3">
                    <span class={`h-2.5 w-2.5 rounded-full ${dotClass}`} />
                    <a
                      href={pmg.host || `https://${pmg.name}:8006`}
                      target="_blank"
                      rel="noopener noreferrer"
                      class="text-sm font-semibold text-gray-900 dark:text-gray-100 hover:text-blue-600 dark:hover:text-blue-400"
                    >
                      {pmg.name}
                    </a>
                    <span class="text-xs text-gray-500 dark:text-gray-400">v{pmg.version || '?'}</span>
                    <span class="text-xs text-gray-500 dark:text-gray-400">•</span>
                    <span class="text-xs text-gray-500 dark:text-gray-400 capitalize">{pmg.status || 'unknown'}</span>
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">
                    Updated {lastUpdated()}
                  </div>
                </div>

                {/* 3-Column Detail Grid */}
                <div class="p-3 space-y-3">
                  <div class="grid grid-cols-1 lg:grid-cols-3 gap-3">
                    {/* LEFT COLUMN: Mail Flow + Traffic Volume */}
                    <div class="space-y-3">
                      {/* Mail Flow Table */}
                      <div class="border border-gray-200 dark:border-gray-700 rounded overflow-hidden">
                        <div class="px-2 py-1.5 bg-gray-50 dark:bg-gray-800/60 border-b border-gray-200 dark:border-gray-700">
                          <h4 class="text-[10px] font-semibold uppercase tracking-wide text-gray-600 dark:text-gray-300">
                            Mail Flow (24h)
                          </h4>
                        </div>
                        <table class="w-full text-xs">
                          <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                            <tr>
                              <td class="px-2 py-1.5">
                                <div class="text-xs font-semibold text-gray-900 dark:text-gray-100">{formatNum(total)}</div>
                                <div class="text-[11px] text-gray-500 dark:text-gray-400">{formatDec(total / 24)}/hr</div>
                              </td>
                              <td class="px-2 py-1.5 text-[11px] text-gray-500 dark:text-gray-400">Total processed</td>
                            </tr>
                            <tr>
                              <td class="px-2 py-1.5">
                                <div class="text-xs text-gray-900 dark:text-gray-100">{formatNum(inbound)}</div>
                                <div class="text-[11px] text-gray-500 dark:text-gray-400">
                                  {formatDec(inbound / 24)}/hr · {formatDec(inboundPct)}% of total
                                </div>
                              </td>
                              <td class="px-2 py-1.5 text-[11px] text-gray-500 dark:text-gray-400">Inbound</td>
                            </tr>
                            <tr>
                              <td class="px-2 py-1.5">
                                <div class="text-xs text-gray-900 dark:text-gray-100">{formatNum(outbound)}</div>
                                <div class="text-[11px] text-gray-500 dark:text-gray-400">
                                  {formatDec(outbound / 24)}/hr · {formatDec(outboundPct)}% of total
                                </div>
                              </td>
                              <td class="px-2 py-1.5 text-[11px] text-gray-500 dark:text-gray-400">Outbound</td>
                            </tr>
                            <tr>
                              <td class="px-2 py-1.5">
                                <div class="text-xs font-medium text-orange-600 dark:text-orange-400">{formatNum(spam)}</div>
                                <div class="text-[11px] text-gray-500 dark:text-gray-400">
                                  {formatDec(spam / 24)}/hr · {formatDec(spamPct)}% rate
                                </div>
                              </td>
                              <td class="px-2 py-1.5 text-[11px] text-gray-500 dark:text-gray-400">Spam caught</td>
                            </tr>
                            <tr>
                              <td class="px-2 py-1.5">
                                <div class="text-xs font-medium text-red-600 dark:text-red-400">{formatNum(virus)}</div>
                                <div class="text-[11px] text-gray-500 dark:text-gray-400">
                                  {formatDec(virus / 24)}/hr · {formatDec(virusPct)}% rate
                                </div>
                              </td>
                              <td class="px-2 py-1.5 text-[11px] text-gray-500 dark:text-gray-400">Viruses blocked</td>
                            </tr>
                          </tbody>
                        </table>
                      </div>

                      {/* Traffic Volume Table */}
                      <div class="border border-gray-200 dark:border-gray-700 rounded overflow-hidden">
                        <div class="px-2 py-1.5 bg-gray-50 dark:bg-gray-800/60 border-b border-gray-200 dark:border-gray-700">
                          <h4 class="text-[10px] font-semibold uppercase tracking-wide text-gray-600 dark:text-gray-300">
                            Traffic Volume
                          </h4>
                        </div>
                        <table class="w-full text-xs">
                          <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                            <tr>
                              <td class="px-2 py-1.5">
                                <div class="text-xs text-gray-900 dark:text-gray-100">{formatDec(bytesIn / 1024 / 1024)} MB</div>
                                <div class="text-[11px] text-gray-500 dark:text-gray-400">
                                  {formatDec(bytesIn / 1024 / 1024 / 24)} MB/hr avg
                                </div>
                              </td>
                              <td class="px-2 py-1.5 text-[11px] text-gray-500 dark:text-gray-400">Inbound bytes</td>
                            </tr>
                            <tr>
                              <td class="px-2 py-1.5">
                                <div class="text-xs text-gray-900 dark:text-gray-100">{formatDec(bytesOut / 1024 / 1024)} MB</div>
                                <div class="text-[11px] text-gray-500 dark:text-gray-400">
                                  {formatDec(bytesOut / 1024 / 1024 / 24)} MB/hr avg
                                </div>
                              </td>
                              <td class="px-2 py-1.5 text-[11px] text-gray-500 dark:text-gray-400">Outbound bytes</td>
                            </tr>
                          </tbody>
                        </table>
                      </div>
                    </div>

                    {/* CENTER COLUMN: Threat/Quarantine + Spam Distribution */}
                    <div class="space-y-3">
                      {/* Threat/Quarantine Table */}
                      <div class="border border-gray-200 dark:border-gray-700 rounded overflow-hidden">
                        <div class="px-2 py-1.5 bg-gray-50 dark:bg-gray-800/60 border-b border-gray-200 dark:border-gray-700">
                          <h4 class="text-[10px] font-semibold uppercase tracking-wide text-gray-600 dark:text-gray-300">
                            Threat & Quarantine
                          </h4>
                        </div>
                        <table class="w-full text-xs">
                          <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                            <tr>
                              <td class="px-2 py-1.5">
                                <div class="text-xs font-medium text-orange-600 dark:text-orange-400">{formatNum(qSpam)}</div>
                                <div class="text-[11px] text-gray-500 dark:text-gray-400">
                                  {formatDec((qSpam / Math.max(qTotal, 1)) * 100)}% of quarantine
                                </div>
                              </td>
                              <td class="px-2 py-1.5 text-[11px] text-gray-500 dark:text-gray-400">Spam quarantined</td>
                            </tr>
                            <tr>
                              <td class="px-2 py-1.5">
                                <div class="text-xs font-medium text-red-600 dark:text-red-400">{formatNum(qVirus)}</div>
                                <div class="text-[11px] text-gray-500 dark:text-gray-400">
                                  {formatDec((qVirus / Math.max(qTotal, 1)) * 100)}% of quarantine
                                </div>
                              </td>
                              <td class="px-2 py-1.5 text-[11px] text-gray-500 dark:text-gray-400">Virus quarantined</td>
                            </tr>
                            <tr>
                              <td class="px-2 py-1.5">
                                <div class="text-xs font-medium text-yellow-600 dark:text-yellow-400">{formatNum(qAttachment)}</div>
                                <div class="text-[11px] text-gray-500 dark:text-gray-400">
                                  {formatDec((qAttachment / Math.max(qTotal, 1)) * 100)}% of quarantine
                                </div>
                              </td>
                              <td class="px-2 py-1.5 text-[11px] text-gray-500 dark:text-gray-400">Attachments blocked</td>
                            </tr>
                            <tr>
                              <td class="px-2 py-1.5">
                                <div class="text-xs text-gray-900 dark:text-gray-100">{formatNum(qBlacklist)}</div>
                                <div class="text-[11px] text-gray-500 dark:text-gray-400">
                                  {formatDec((qBlacklist / Math.max(qTotal, 1)) * 100)}% of quarantine
                                </div>
                              </td>
                              <td class="px-2 py-1.5 text-[11px] text-gray-500 dark:text-gray-400">Blacklisted</td>
                            </tr>
                            <tr>
                              <td class="px-2 py-1.5">
                                <div class="text-xs font-semibold text-gray-900 dark:text-gray-100">{formatNum(qTotal)}</div>
                                <div class="text-[11px] text-gray-500 dark:text-gray-400">
                                  {formatDec(qTotal / 24)}/hr · {formatDec((qTotal / Math.max(inbound, 1)) * 100)}% of inbound
                                </div>
                              </td>
                              <td class="px-2 py-1.5 text-[11px] text-gray-500 dark:text-gray-400">Total quarantined</td>
                            </tr>
                          </tbody>
                        </table>
                      </div>

                      {/* Spam Score Distribution */}
                      <Show when={(pmg.spamDistribution?.length ?? 0) > 0}>
                        {(() => {
                          const totalSpamScored = pmg.spamDistribution?.reduce((sum, b) => sum + b.count, 0) || 1;
                          return (
                            <div class="border border-gray-200 dark:border-gray-700 rounded overflow-hidden">
                              <div class="px-2 py-1.5 bg-gray-50 dark:bg-gray-800/60 border-b border-gray-200 dark:border-gray-700">
                                <h4 class="text-[10px] font-semibold uppercase tracking-wide text-gray-600 dark:text-gray-300">
                                  Spam Score Distribution
                                </h4>
                              </div>
                              <div class="p-2">
                                <div class="grid grid-cols-3 gap-1.5">
                                  <For each={pmg.spamDistribution}>
                                    {(bucket) => {
                                      const pct = (bucket.count / totalSpamScored) * 100;
                                      return (
                                        <div class="text-center border border-gray-200 dark:border-gray-700 rounded px-1 py-1">
                                          <div class="text-[10px] text-gray-500 dark:text-gray-400">{bucket.score}</div>
                                          <div class="text-xs font-semibold text-gray-900 dark:text-gray-100">{formatNum(bucket.count)}</div>
                                          <div class="text-[10px] text-gray-500 dark:text-gray-400">{formatDec(pct)}%</div>
                                        </div>
                                      );
                                    }}
                                  </For>
                                </div>
                              </div>
                            </div>
                          );
                        })()}
                      </Show>
                    </div>

                    {/* RIGHT COLUMN: Delivery Health */}
                    <div>
                      <div class="border border-gray-200 dark:border-gray-700 rounded overflow-hidden">
                        <div class="px-2 py-1.5 bg-gray-50 dark:bg-gray-800/60 border-b border-gray-200 dark:border-gray-700">
                          <h4 class="text-[10px] font-semibold uppercase tracking-wide text-gray-600 dark:text-gray-300">
                            Delivery Health
                          </h4>
                        </div>
                        <table class="w-full text-xs">
                          <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                            <tr>
                              <td class="px-2 py-1.5">
                                <div class="text-xs text-gray-900 dark:text-gray-100">{formatNum(bouncesIn)}</div>
                                <div class="text-[11px] text-gray-500 dark:text-gray-400">
                                  {formatDec(bouncesIn / 24)}/hr · {formatDec((bouncesIn / Math.max(inbound, 1)) * 100)}% of inbound
                                </div>
                              </td>
                              <td class="px-2 py-1.5 text-[11px] text-gray-500 dark:text-gray-400">Bounces inbound</td>
                            </tr>
                            <tr>
                              <td class="px-2 py-1.5">
                                <div class="text-xs text-gray-900 dark:text-gray-100">{formatNum(bouncesOut)}</div>
                                <div class="text-[11px] text-gray-500 dark:text-gray-400">
                                  {formatDec(bouncesOut / 24)}/hr · {formatDec((bouncesOut / Math.max(outbound, 1)) * 100)}% of outbound
                                </div>
                              </td>
                              <td class="px-2 py-1.5 text-[11px] text-gray-500 dark:text-gray-400">Bounces outbound</td>
                            </tr>
                            <tr>
                              <td class="px-2 py-1.5">
                                <div class="text-xs text-gray-900 dark:text-gray-100">{formatNum(rbl)}</div>
                                <div class="text-[11px] text-gray-500 dark:text-gray-400">
                                  {formatDec(rbl / 24)}/hr · {formatDec(rblPct)}% of inbound
                                </div>
                              </td>
                              <td class="px-2 py-1.5 text-[11px] text-gray-500 dark:text-gray-400">RBL rejects</td>
                            </tr>
                            <tr>
                              <td class="px-2 py-1.5">
                                <div class="text-xs text-gray-900 dark:text-gray-100">{formatNum(pregreet)}</div>
                                <div class="text-[11px] text-gray-500 dark:text-gray-400">
                                  {formatDec(pregreet / 24)}/hr · {formatDec((pregreet / Math.max(inbound, 1)) * 100)}% of inbound
                                </div>
                              </td>
                              <td class="px-2 py-1.5 text-[11px] text-gray-500 dark:text-gray-400">Pregreet rejects</td>
                            </tr>
                            <tr>
                              <td class="px-2 py-1.5">
                                <div class="text-xs text-gray-900 dark:text-gray-100">{formatNum(greylist)}</div>
                                <div class="text-[11px] text-gray-500 dark:text-gray-400">
                                  {formatDec(greylist / 24)}/hr · {formatDec((greylist / Math.max(inbound, 1)) * 100)}% of inbound
                                </div>
                              </td>
                              <td class="px-2 py-1.5 text-[11px] text-gray-500 dark:text-gray-400">Greylisted</td>
                            </tr>
                            <tr>
                              <td class="px-2 py-1.5">
                                <div class="text-xs text-gray-900 dark:text-gray-100">{formatNum(junk)}</div>
                                <div class="text-[11px] text-gray-500 dark:text-gray-400">
                                  {formatDec(junk / 24)}/hr · {formatDec((junk / Math.max(inbound, 1)) * 100)}% of inbound
                                </div>
                              </td>
                              <td class="px-2 py-1.5 text-[11px] text-gray-500 dark:text-gray-400">Junk mail</td>
                            </tr>
                            <tr>
                              <td class="px-2 py-1.5">
                                <div class="text-xs text-gray-900 dark:text-gray-100">
                                  {pmg.mailStats?.averageProcessTimeMs ? formatDec(pmg.mailStats.averageProcessTimeMs / 1000, 2) : '—'} s
                                </div>
                                <div class="text-[11px] text-gray-500 dark:text-gray-400">
                                  ~{pmg.mailStats?.averageProcessTimeMs ? formatNum(60000 / pmg.mailStats.averageProcessTimeMs) : '—'} msg/min
                                </div>
                              </td>
                              <td class="px-2 py-1.5 text-[11px] text-gray-500 dark:text-gray-400">Avg process time</td>
                            </tr>
                          </tbody>
                        </table>
                      </div>
                    </div>
                  </div>

                  {/* FULL-WIDTH: Cluster Nodes */}
                  <Show when={(pmg.nodes?.length ?? 0) > 0}>
                    <div class="border border-gray-200 dark:border-gray-700 rounded overflow-hidden">
                      <div class="px-2 py-1.5 bg-gray-50 dark:bg-gray-800/60 border-b border-gray-200 dark:border-gray-700">
                        <h4 class="text-[10px] font-semibold uppercase tracking-wide text-gray-600 dark:text-gray-300">
                          Cluster Nodes
                        </h4>
                      </div>
                      <div class="overflow-x-auto">
                        <table class="w-full text-xs">
                          <thead class="bg-gray-50 dark:bg-gray-800/70 border-b border-gray-200 dark:border-gray-700">
                            <tr class="text-[11px] uppercase tracking-wide text-gray-600 dark:text-gray-300">
                              <th class="px-2 py-1 text-left">Name</th>
                              <th class="px-2 py-1 text-left">Role</th>
                              <th class="px-2 py-1 text-left">Status</th>
                              <th class="px-2 py-1 text-left">Uptime</th>
                              <th class="px-2 py-1 text-left">Load Avg</th>
                              <th class="px-2 py-1 text-left">Queue Depth</th>
                              <th class="px-2 py-1 text-left">Oldest Msg</th>
                            </tr>
                          </thead>
                          <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                            <For each={pmg.nodes}>
                              {(node) => {
                                const nodeStatusClass = (node.status || '').toLowerCase() === 'online' ? 'bg-green-500' : 'bg-red-500';
                                const queue = node.queueStatus;
                                const queueTotal = queue?.total || 0;
                                const hasQueue = queueTotal > 0;
                                const queueClass = hasQueue && queueTotal > 50 ? 'text-amber-600 dark:text-amber-400' : 'text-gray-700 dark:text-gray-300';
                                const oldestAge = queue?.oldestAge || 0;
                                const oldestMin = Math.floor(oldestAge / 60);
                                const oldestHr = Math.floor(oldestMin / 60);
                                const oldestClass = oldestAge > 1800 ? 'text-amber-600 dark:text-amber-400' : 'text-gray-700 dark:text-gray-300';

                                return (
                                  <tr class="">
                                    <td class="px-2 py-1.5 text-xs font-medium text-gray-900 dark:text-gray-100 truncate max-w-[150px]">{node.name}</td>
                                    <td class="px-2 py-1.5 text-xs text-gray-700 dark:text-gray-300 capitalize">{node.role || '—'}</td>
                                    <td class="px-2 py-1.5">
                                      <div class="flex items-center gap-1.5">
                                        <span class={`h-1.5 w-1.5 rounded-full ${nodeStatusClass}`} />
                                        <span class="text-xs text-gray-700 dark:text-gray-300 capitalize">{node.status || 'unknown'}</span>
                                      </div>
                                    </td>
                                    <td class="px-2 py-1.5 text-xs text-gray-700 dark:text-gray-300">
                                      <Show when={node.uptime} fallback="—">
                                        {Math.floor((node.uptime ?? 0) / 86400)}d {Math.floor(((node.uptime ?? 0) % 86400) / 3600)}h
                                      </Show>
                                    </td>
                                    <td class="px-2 py-1.5 text-xs text-gray-700 dark:text-gray-300">{node.loadAvg || '—'}</td>
                                    <td class="px-2 py-1.5">
                                      <Show when={queue} fallback={<span class="text-xs text-gray-500 dark:text-gray-400">—</span>}>
                                        <div class={`text-xs font-medium ${queueClass}`}>
                                          {formatNum(queueTotal)}
                                        </div>
                                        <div class="text-[11px] text-gray-500 dark:text-gray-400">
                                          A:{queue!.active} D:{queue!.deferred} H:{queue!.hold} I:{queue!.incoming}
                                        </div>
                                      </Show>
                                    </td>
                                    <td class="px-2 py-1.5">
                                      <Show when={queue && oldestAge > 0} fallback={<span class="text-xs text-gray-500 dark:text-gray-400">—</span>}>
                                        <span class={`text-xs ${oldestClass}`}>
                                          {oldestHr > 0 ? `${oldestHr}h ${oldestMin % 60}m` : `${oldestMin}m`}
                                        </span>
                                      </Show>
                                    </td>
                                  </tr>
                                );
                              }}
                            </For>
                          </tbody>
                        </table>
                      </div>
                    </div>
                  </Show>
                </div>
              </Card>
            );
          }}
        </For>
      </Show>
    </div>
  );
};

export default MailGateway;
