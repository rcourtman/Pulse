import type { Component } from 'solid-js';
import { For, Show, createMemo } from 'solid-js';
import { useTooltip } from '@/hooks/useTooltip';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import { Card } from '@/components/shared/Card';
import { formatRelativeTime, formatBytes } from '@/utils/format';
import type { PMGInstance } from '@/types/api';

// Format large numbers with K/M suffixes
const formatCompact = (value?: number | null): string => {
  if (value === undefined || value === null || Number.isNaN(value)) return '—';
  if (value >= 1000000) return `${(value / 1000000).toFixed(1)}M`;
  if (value >= 1000) return `${(value / 1000).toFixed(1)}K`;
  return Math.round(value).toLocaleString();
};

const formatNum = (value?: number | null) => {
  if (value === undefined || value === null || Number.isNaN(value)) return '—';
  return Math.round(value).toLocaleString();
};

const formatDec = (value?: number | null, digits = 1) => {
  if (value === undefined || value === null || Number.isNaN(value)) return '—';
  return value.toFixed(digits);
};

const formatPct = (value?: number | null) => {
  if (value === undefined || value === null || Number.isNaN(value)) return '0%';
  return `${value.toFixed(1)}%`;
};

const fmtMaybe = (value?: number | null) => {
  if (value === undefined || value === null || Number.isNaN(value)) return '—';
  return formatCompact(value);
};

const parseTimestamp = (value?: string) => {
  if (!value) return undefined;
  const time = Date.parse(value);
  return Number.isNaN(time) ? undefined : time;
};

const ThreatBar: Component<{
  percent: number;
  color: 'spam' | 'virus' | 'quarantine';
  label: string;
  count: number;
}> = (props) => {
  const barColor = () => {
    switch (props.color) {
      case 'spam':
        return 'bg-orange-500';
      case 'virus':
        return 'bg-red-500';
      case 'quarantine':
        return 'bg-yellow-500';
    }
  };

  const textColor = () => {
    switch (props.color) {
      case 'spam':
        return 'text-orange-600 dark:text-orange-400';
      case 'virus':
        return 'text-red-600 dark:text-red-400';
      case 'quarantine':
        return 'text-yellow-600 dark:text-yellow-400';
    }
  };

  return (
    <div class="space-y-1">
      <div class="flex items-center justify-between text-xs">
        <span class="text-slate-600 dark:text-slate-400">{props.label}</span>
        <span class={`font-medium ${textColor()}`}>
          {formatCompact(props.count)} ({formatPct(props.percent)})
        </span>
      </div>
      <div class="h-1.5 bg-slate-200 dark:bg-slate-700 rounded-full overflow-hidden">
        <div
          class={`h-full ${barColor()} transition-all duration-500 rounded-full`}
          style={{ width: `${Math.min(props.percent * 10, 100)}%` }} // Scale up for visibility (10% threat = full bar)
        />
      </div>
    </div>
  );
};

const StatusBadge: Component<{ status: string; health?: string }> = (props) => {
  const statusInfo = createMemo(() => {
    const status = (props.health || props.status || '').toLowerCase();
    if (status.includes('healthy') || status === 'online') {
      return {
        bg: 'bg-green-100 dark:bg-green-900/30',
        text: 'text-green-700 dark:text-green-400',
        dot: 'bg-green-500',
        label: 'Healthy',
      };
    }
    if (status.includes('degraded') || status.includes('warning')) {
      return {
        bg: 'bg-yellow-100 dark:bg-yellow-900/30',
        text: 'text-yellow-700 dark:text-yellow-400',
        dot: 'bg-yellow-500',
        label: 'Degraded',
      };
    }
    if (status.includes('error') || status === 'offline') {
      return {
        bg: 'bg-red-100 dark:bg-red-900/30',
        text: 'text-red-700 dark:text-red-400',
        dot: 'bg-red-500',
        label: 'Offline',
      };
    }
    return { bg: 'bg-slate-100 dark:bg-slate-800', text: 'text-slate-600 dark:text-slate-400', dot: 'bg-slate-400', label: status || 'Unknown' };
  });

  return (
    <span class={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase ${statusInfo().bg} ${statusInfo().text}`}>
      <span class={`w-1.5 h-1.5 rounded-full ${statusInfo().dot}`} />
      {statusInfo().label}
    </span>
  );
};

const QueueIndicator: Component<{ queue?: { total: number; active?: number; deferred?: number; hold?: number; incoming?: number; oldestAge?: number } }> = (props) => {
  const tip = useTooltip();
  const queueTotal = () => props.queue?.total || 0;
  const hasQueue = () => queueTotal() > 0;

  const queueSeverity = () => {
    const total = queueTotal();
    if (total > 100) return 'high';
    if (total > 20) return 'medium';
    return 'low';
  };

  return (
    <>
      <div
        class="cursor-help"
        onMouseEnter={(e) => {
          if (props.queue) tip.onMouseEnter(e);
        }}
        onMouseLeave={() => tip.onMouseLeave()}
      >
        <Show
          when={hasQueue()}
          fallback={<span class="text-xs text-slate-400">No queue</span>}
        >
          <span
            class={`text-xs font-medium ${queueSeverity() === 'high' ? 'text-red-600 dark:text-red-400' : queueSeverity() === 'medium' ? 'text-yellow-600 dark:text-yellow-400' : 'text-slate-600 dark:text-slate-400'}`}
          >
            {formatNum(queueTotal())} msgs
          </span>
        </Show>
      </div>
      <TooltipPortal when={tip.show() && !!props.queue} x={tip.pos().x} y={tip.pos().y}>
        <div class="bg-slate-900 text-white text-xs rounded-md px-3 py-2 shadow-sm border border-slate-700">
          <div class="space-y-1">
            <div class="flex justify-between">
              <span class="text-slate-400">Active</span>
              <span>{props.queue?.active || 0}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-slate-400">Deferred</span>
              <span>{props.queue?.deferred || 0}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-slate-400">Hold</span>
              <span>{props.queue?.hold || 0}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-slate-400">Incoming</span>
              <span>{props.queue?.incoming || 0}</span>
            </div>
            <Show when={(props.queue?.oldestAge || 0) > 0}>
              <div class="flex justify-between border-t border-slate-700 pt-1 mt-1">
                <span class="text-slate-400">Oldest</span>
                <span class={(props.queue?.oldestAge || 0) > 1800 ? 'text-yellow-400' : ''}>
                  {Math.floor((props.queue?.oldestAge || 0) / 60)}m
                </span>
              </div>
            </Show>
          </div>
        </div>
      </TooltipPortal>
    </>
  );
};

export const PMGInstancePanel: Component<{ pmg: PMGInstance }> = (props) => {
  const lastUpdated = createMemo(() => {
    const ts =
      parseTimestamp(props.pmg.mailStats?.updatedAt) ??
      parseTimestamp(props.pmg.lastUpdated) ??
      parseTimestamp(props.pmg.lastSeen);
    return ts ? formatRelativeTime(ts) : '—';
  });

  const stats = createMemo(() => {
    const m = props.pmg.mailStats;
    const total = m?.countTotal || 0;
    const inbound = m?.countIn || 0;
    const outbound = m?.countOut || 0;
    const spam = m?.spamIn || 0;
    const virus = m?.virusIn || 0;
    const bytesIn = m?.bytesIn || 0;
    const bytesOut = m?.bytesOut || 0;
    const rbl = m?.rblRejects || 0;
    const pregreet = m?.pregreetRejects || 0;
    const greylist = m?.greylistCount || 0;
    const bouncesIn = m?.bouncesIn || 0;
    const bouncesOut = m?.bouncesOut || 0;

    const q = props.pmg.quarantine;
    const qSpam = q?.spam || 0;
    const qVirus = q?.virus || 0;
    const qAttachment = q?.attachment || 0;
    const qBlacklist = q?.blacklisted || 0;
    const qTotal = qSpam + qVirus + qAttachment + qBlacklist;

    const spamPct = total > 0 ? (spam / total) * 100 : 0;
    const virusPct = total > 0 ? (virus / total) * 100 : 0;
    const quarantinePct = inbound > 0 ? (qTotal / inbound) * 100 : 0;

    return {
      total,
      inbound,
      outbound,
      spam,
      virus,
      bytesIn,
      bytesOut,
      rbl,
      pregreet,
      greylist,
      bouncesIn,
      bouncesOut,
      qTotal,
      spamPct,
      virusPct,
      quarantinePct,
    };
  });

  const instanceQueue = createMemo(() => {
    if (!props.pmg.nodes) return { total: 0 };
    let total = 0;
    let active = 0;
    let deferred = 0;
    let hold = 0;
    let incoming = 0;
    let oldestAge = 0;
    for (const node of props.pmg.nodes) {
      const q = node.queueStatus;
      if (q) {
        total += q.total || 0;
        active += q.active || 0;
        deferred += q.deferred || 0;
        hold += q.hold || 0;
        incoming += q.incoming || 0;
        if ((q.oldestAge || 0) > oldestAge) oldestAge = q.oldestAge || 0;
      }
    }
    return { total, active, deferred, hold, incoming, oldestAge };
  });

  const domainStatsByDomain = createMemo(() => {
    const by = new Map<string, { mail: number; spam: number; virus: number; bytes: number }>();
    for (const row of props.pmg.domainStats ?? []) {
      const key = (row.domain || '').trim().toLowerCase();
      if (!key) continue;
      by.set(key, {
        mail: row.mailCount || 0,
        spam: row.spamCount || 0,
        virus: row.virusCount || 0,
        bytes: row.bytes || 0,
      });
    }
    return by;
  });

  const relayDomainsWithStats = createMemo(() => {
    const by = domainStatsByDomain();
    const rows = (props.pmg.relayDomains ?? []).map((d) => {
      const key = (d.domain || '').trim().toLowerCase();
      const stat = key ? by.get(key) : undefined;
      const mail = stat?.mail ?? 0;
      const spam = stat?.spam ?? 0;
      const virus = stat?.virus ?? 0;
      const bytes = stat?.bytes ?? 0;
      const spamRate = mail > 0 ? (spam / mail) * 100 : 0;
      return {
        domain: d.domain,
        comment: d.comment,
        mail,
        spam,
        virus,
        bytes,
        spamRate,
        hasStats: stat !== undefined,
      };
    });

    // Stable-ish ordering: domains with stats first, then by spam desc, then by mail desc.
    rows.sort((a, b) => {
      if (a.hasStats !== b.hasStats) return a.hasStats ? -1 : 1;
      if (b.spam !== a.spam) return b.spam - a.spam;
      if (b.mail !== a.mail) return b.mail - a.mail;
      return (a.domain || '').localeCompare(b.domain || '');
    });

    return rows;
  });

  const domainStatsAsOfRelative = createMemo(() => {
    const raw = props.pmg.domainStatsAsOf;
    const ts = raw ? parseTimestamp(raw) : undefined;
    return ts ? formatRelativeTime(ts) : '';
  });

  const relayDomainSet = createMemo(() => {
    const set = new Set<string>();
    for (const d of props.pmg.relayDomains ?? []) {
      const key = (d.domain || '').trim().toLowerCase();
      if (key) set.add(key);
    }
    return set;
  });

  const domainStatsRows = createMemo(() => {
    const rows = (props.pmg.domainStats ?? [])
      .map((row) => {
        const domain = (row.domain || '').trim();
        const key = domain.toLowerCase();
        const mail = row.mailCount || 0;
        const spam = row.spamCount || 0;
        const virus = row.virusCount || 0;
        const bytes = row.bytes || 0;
        const spamRate = mail > 0 ? (spam / mail) * 100 : 0;
        return { domain, key, mail, spam, virus, bytes, spamRate };
      })
      .filter((row) => row.domain !== '');

    rows.sort((a, b) => {
      if (b.spam !== a.spam) return b.spam - a.spam;
      if (b.mail !== a.mail) return b.mail - a.mail;
      return a.domain.localeCompare(b.domain);
    });

    return rows;
  });

  const otherDomainStatsRows = createMemo(() => {
    const relaySet = relayDomainSet();
    return domainStatsRows().filter((row) => !relaySet.has(row.key));
  });

  return (
    <Card padding="none" tone="card" class="overflow-hidden">
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2 px-4 py-3 bg-slate-50/50 dark:bg-slate-800 border-b border-slate-200 dark:border-slate-700">
        <div class="flex items-center gap-3">
          <a
            href={props.pmg.host || `https://${props.pmg.name}:8006`}
            target="_blank"
            rel="noopener noreferrer"
            class="text-base font-semibold text-slate-900 dark:text-slate-100 hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
          >
            {props.pmg.name}
          </a>
          <StatusBadge status={props.pmg.status || ''} health={props.pmg.connectionHealth} />
          <Show when={props.pmg.version}>
            <span class="text-xs text-slate-500 dark:text-slate-400 bg-slate-100 dark:bg-slate-700 px-1.5 py-0.5 rounded">
              v{props.pmg.version}
            </span>
          </Show>
        </div>
        <div class="flex items-center gap-4 text-xs text-slate-500 dark:text-slate-400">
          <QueueIndicator queue={instanceQueue()} />
          <span>Updated {lastUpdated()}</span>
        </div>
      </div>

      <div class="p-4">
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-4">
          <ThreatBar percent={stats().spamPct} color="spam" label="Spam Rate" count={stats().spam} />
          <ThreatBar percent={stats().virusPct} color="virus" label="Virus Rate" count={stats().virus} />
          <ThreatBar percent={stats().quarantinePct} color="quarantine" label="Quarantine Rate" count={stats().qTotal} />
        </div>

        <div class="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-6 gap-3">
          <div class="bg-slate-50 dark:bg-slate-800 rounded-md p-3">
            <div class="text-xs text-slate-500 dark:text-slate-400 mb-1">Total (24h)</div>
            <div class="text-lg font-bold text-slate-900 dark:text-slate-100">{formatCompact(stats().total)}</div>
            <div class="text-[10px] text-slate-400">{formatNum(Math.round(stats().total / 24))}/hr</div>
          </div>
          <div class="bg-slate-50 dark:bg-slate-800 rounded-md p-3">
            <div class="text-xs text-slate-500 dark:text-slate-400 mb-1">Inbound</div>
            <div class="text-lg font-bold text-slate-900 dark:text-slate-100">{formatCompact(stats().inbound)}</div>
            <div class="text-[10px] text-slate-400">{formatBytes(stats().bytesIn)}</div>
          </div>
          <div class="bg-slate-50 dark:bg-slate-800 rounded-md p-3">
            <div class="text-xs text-slate-500 dark:text-slate-400 mb-1">Outbound</div>
            <div class="text-lg font-bold text-slate-900 dark:text-slate-100">{formatCompact(stats().outbound)}</div>
            <div class="text-[10px] text-slate-400">{formatBytes(stats().bytesOut)}</div>
          </div>

          <div class="bg-orange-50 dark:bg-orange-900/20 rounded-md p-3">
            <div class="text-xs text-orange-600 dark:text-orange-400 mb-1">Spam</div>
            <div class="text-lg font-bold text-orange-600 dark:text-orange-400">{formatCompact(stats().spam)}</div>
            <div class="text-[10px] text-orange-500/70">{formatPct(stats().spamPct)} rate</div>
          </div>
          <div class="bg-red-50 dark:bg-red-900/20 rounded-md p-3">
            <div class="text-xs text-red-600 dark:text-red-400 mb-1">Viruses</div>
            <div class="text-lg font-bold text-red-600 dark:text-red-400">{formatCompact(stats().virus)}</div>
            <div class="text-[10px] text-red-500/70">{formatPct(stats().virusPct)} rate</div>
          </div>
          <div class="bg-yellow-50 dark:bg-yellow-900/20 rounded-md p-3">
            <div class="text-xs text-yellow-600 dark:text-yellow-400 mb-1">Quarantine</div>
            <div class="text-lg font-bold text-yellow-600 dark:text-yellow-400">{formatCompact(stats().qTotal)}</div>
            <div class="text-[10px] text-yellow-500/70">{formatPct(stats().quarantinePct)} of inbound</div>
          </div>
        </div>

        <div class="mt-3 pt-3 border-t border-slate-200 dark:border-slate-700">
          <div class="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide mb-2">Delivery Health</div>
          <div class="flex flex-wrap gap-x-6 gap-y-1 text-xs">
            <div class="flex items-center gap-2">
              <span class="text-slate-500 dark:text-slate-400">RBL Rejects:</span>
              <span class="font-medium text-slate-700 dark:text-slate-300">{formatNum(stats().rbl)}</span>
            </div>
            <div class="flex items-center gap-2">
              <span class="text-slate-500 dark:text-slate-400">Pregreet:</span>
              <span class="font-medium text-slate-700 dark:text-slate-300">{formatNum(stats().pregreet)}</span>
            </div>
            <div class="flex items-center gap-2">
              <span class="text-slate-500 dark:text-slate-400">Greylisted:</span>
              <span class="font-medium text-slate-700 dark:text-slate-300">{formatNum(stats().greylist)}</span>
            </div>
            <div class="flex items-center gap-2">
              <span class="text-slate-500 dark:text-slate-400">Bounces In/Out:</span>
              <span class="font-medium text-slate-700 dark:text-slate-300">{formatNum(stats().bouncesIn)}/{formatNum(stats().bouncesOut)}</span>
            </div>
            <Show when={props.pmg.mailStats?.averageProcessTimeMs}>
              <div class="flex items-center gap-2">
                <span class="text-slate-500 dark:text-slate-400">Avg Process:</span>
                <span class="font-medium text-slate-700 dark:text-slate-300">{formatDec((props.pmg.mailStats?.averageProcessTimeMs || 0) / 1000, 2)}s</span>
              </div>
            </Show>
          </div>
        </div>

        <Show when={(props.pmg.nodes?.length ?? 0) > 0}>
          <div class="mt-3 pt-3 border-t border-slate-200 dark:border-slate-700">
            <div class="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide mb-2">
              Cluster Nodes ({props.pmg.nodes?.length})
            </div>
            <div class="overflow-x-auto -mx-4 px-4">
              <table class="w-full min-w-[600px] text-xs">
                <thead>
                  <tr class="text-left text-[10px] uppercase tracking-wide text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-700">
                    <th class="pb-1.5 font-medium">Node</th>
                    <th class="pb-1.5 font-medium">Status</th>
                    <th class="pb-1.5 font-medium">Uptime</th>
                    <th class="pb-1.5 font-medium">Load</th>
                    <th class="pb-1.5 font-medium">Queue</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-gray-700/50">
                  <For each={props.pmg.nodes}>
                    {(node) => {
                      const isOnline = (node.status || '').toLowerCase() === 'online';

                      return (
                        <tr class="hover:bg-slate-50/50 dark:hover:bg-slate-800/30">
                          <td class="py-1.5 font-medium text-slate-900 dark:text-slate-100">{node.name}</td>
                          <td class="py-1.5">
                            <span class={`inline-flex items-center gap-1 ${isOnline ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}`}>
                              <span class={`w-1.5 h-1.5 rounded-full ${isOnline ? 'bg-green-500' : 'bg-red-500'}`} />
                              {node.status || 'unknown'}
                            </span>
                          </td>
                          <td class="py-1.5 text-slate-600 dark:text-slate-400">
                            <Show when={node.uptime} fallback="—">
                              {Math.floor((node.uptime ?? 0) / 86400)}d {Math.floor(((node.uptime ?? 0) % 86400) / 3600)}h
                            </Show>
                          </td>
                          <td class="py-1.5 text-slate-600 dark:text-slate-400">{node.loadAvg || '—'}</td>
                          <td class="py-1.5">
                            <QueueIndicator queue={node.queueStatus} />
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

        <Show when={(props.pmg.spamDistribution?.length ?? 0) > 0}>
          <div class="mt-3 pt-3 border-t border-slate-200 dark:border-slate-700">
            <div class="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide mb-2">Spam Score Distribution</div>
            <div class="flex gap-1 overflow-x-auto pb-1">
              <For each={props.pmg.spamDistribution}>
                {(bucket) => {
                  const totalScored = props.pmg.spamDistribution?.reduce((sum, b) => sum + b.count, 0) || 1;
                  const pct = (bucket.count / totalScored) * 100;
                  return (
                    <div class="flex-shrink-0 text-center bg-slate-50 dark:bg-slate-800 rounded px-2 py-1.5 min-w-[50px]">
                      <div class="text-[10px] text-slate-500 dark:text-slate-400">{bucket.score}</div>
                      <div class="text-xs font-semibold text-slate-900 dark:text-slate-100">{formatCompact(bucket.count)}</div>
                      <div class="text-[10px] text-slate-400">{formatDec(pct)}%</div>
                    </div>
                  );
                }}
              </For>
            </div>
          </div>
        </Show>

        <Show when={(props.pmg.relayDomains?.length ?? 0) > 0}>
          <div class="mt-3 pt-3 border-t border-slate-200 dark:border-slate-700">
            <div class="flex items-center justify-between gap-3 mb-2">
              <div class="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide">
                Relay Domains (24h)
              </div>
              <div class="flex items-center gap-3 text-xs text-slate-500 dark:text-slate-400">
                <Show when={domainStatsAsOfRelative()}>
                  <span>As of {domainStatsAsOfRelative()}</span>
                </Show>
                <span>{formatNum(props.pmg.relayDomains?.length ?? 0)}</span>
              </div>
            </div>

            <div class="overflow-auto max-h-[240px] rounded-md border border-slate-200 dark:border-slate-700">
              <table class="w-full min-w-[780px] text-xs">
                <thead class="bg-slate-50 dark:bg-slate-800">
                  <tr class="text-left text-[10px] uppercase tracking-wide text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-700">
                    <th class="px-3 py-2 font-medium">Domain</th>
                    <th class="px-3 py-2 font-medium">Mail</th>
                    <th class="px-3 py-2 font-medium">Spam</th>
                    <th class="px-3 py-2 font-medium">Virus</th>
                    <th class="px-3 py-2 font-medium">Spam Rate</th>
                    <th class="px-3 py-2 font-medium">Bytes</th>
                    <th class="px-3 py-2 font-medium">Comment</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-gray-700/50">
                  <For each={relayDomainsWithStats()}>
                    {(row) => (
                      <tr class="hover:bg-slate-50/50 dark:hover:bg-slate-800/30">
                        <td class="px-3 py-2 font-medium text-slate-900 dark:text-slate-100">{row.domain}</td>
                        <td class="px-3 py-2 text-slate-700 dark:text-slate-200">{row.hasStats ? fmtMaybe(row.mail) : '—'}</td>
                        <td class="px-3 py-2 text-orange-700/90 dark:text-orange-300/90">{row.hasStats ? fmtMaybe(row.spam) : '—'}</td>
                        <td class="px-3 py-2 text-red-700/90 dark:text-red-300/90">{row.hasStats ? fmtMaybe(row.virus) : '—'}</td>
                        <td class="px-3 py-2 text-slate-700 dark:text-slate-200">
                          {row.hasStats ? formatPct(row.spamRate) : '—'}
                        </td>
                        <td class="px-3 py-2 text-slate-600 dark:text-slate-300">
                          {row.hasStats && row.bytes > 0 ? formatBytes(row.bytes) : row.hasStats ? '—' : '—'}
                        </td>
                        <td class="px-3 py-2 text-slate-600 dark:text-slate-300">{row.comment || '—'}</td>
                      </tr>
                    )}
                  </For>
                </tbody>
              </table>
            </div>
          </div>
        </Show>

        <Show when={(otherDomainStatsRows().length ?? 0) > 0}>
          <div class="mt-3 pt-3 border-t border-slate-200 dark:border-slate-700">
            <details class="group">
              <summary class="cursor-pointer select-none list-none">
                <div class="flex items-center justify-between gap-3 mb-2">
                  <div class="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide">
                    Other Domains (24h)
                  </div>
                  <div class="flex items-center gap-3 text-xs text-slate-500 dark:text-slate-400">
                    <Show when={domainStatsAsOfRelative()}>
                      <span>As of {domainStatsAsOfRelative()}</span>
                    </Show>
                    <span>{formatNum(otherDomainStatsRows().length)}</span>
                  </div>
                </div>
                <div class="text-[11px] text-slate-500 dark:text-slate-400 -mt-1 mb-2">
                  Domains seen in the last 24 hours that are not configured as relay domains.
                </div>
              </summary>

              <div class="overflow-auto max-h-[260px] rounded-md border border-slate-200 dark:border-slate-700">
                <table class="w-full min-w-[720px] text-xs">
                  <thead class="bg-slate-50 dark:bg-slate-800">
                    <tr class="text-left text-[10px] uppercase tracking-wide text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-700">
                      <th class="px-3 py-2 font-medium">Domain</th>
                      <th class="px-3 py-2 font-medium">Mail</th>
                      <th class="px-3 py-2 font-medium">Spam</th>
                      <th class="px-3 py-2 font-medium">Virus</th>
                      <th class="px-3 py-2 font-medium">Spam Rate</th>
                      <th class="px-3 py-2 font-medium">Bytes</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-100 dark:divide-gray-700/50">
                    <For each={otherDomainStatsRows()}>
                      {(row) => (
                        <tr class="hover:bg-slate-50/50 dark:hover:bg-slate-800/30">
                          <td class="px-3 py-2 font-medium text-slate-900 dark:text-slate-100">{row.domain}</td>
                          <td class="px-3 py-2 text-slate-700 dark:text-slate-200">{fmtMaybe(row.mail)}</td>
                          <td class="px-3 py-2 text-orange-700/90 dark:text-orange-300/90">{fmtMaybe(row.spam)}</td>
                          <td class="px-3 py-2 text-red-700/90 dark:text-red-300/90">{fmtMaybe(row.virus)}</td>
                          <td class="px-3 py-2 text-slate-700 dark:text-slate-200">{formatPct(row.spamRate)}</td>
                          <td class="px-3 py-2 text-slate-600 dark:text-slate-300">{row.bytes > 0 ? formatBytes(row.bytes) : '—'}</td>
                        </tr>
                      )}
                    </For>
                  </tbody>
                </table>
              </div>
            </details>
          </div>
        </Show>

        <Show when={(props.pmg.domainStats?.length ?? 0) > 0 && (props.pmg.relayDomains?.length ?? 0) === 0}>
          <div class="mt-3 pt-3 border-t border-slate-200 dark:border-slate-700">
            <div class="flex items-center justify-between gap-3 mb-2">
              <div class="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide">
                Domains (24h)
              </div>
              <div class="flex items-center gap-3 text-xs text-slate-500 dark:text-slate-400">
                <Show when={domainStatsAsOfRelative()}>
                  <span>As of {domainStatsAsOfRelative()}</span>
                </Show>
                <span>{formatNum(domainStatsRows().length)}</span>
              </div>
            </div>

            <div class="overflow-auto max-h-[260px] rounded-md border border-slate-200 dark:border-slate-700">
              <table class="w-full min-w-[720px] text-xs">
                <thead class="bg-slate-50 dark:bg-slate-800">
                  <tr class="text-left text-[10px] uppercase tracking-wide text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-700">
                    <th class="px-3 py-2 font-medium">Domain</th>
                    <th class="px-3 py-2 font-medium">Mail</th>
                    <th class="px-3 py-2 font-medium">Spam</th>
                    <th class="px-3 py-2 font-medium">Virus</th>
                    <th class="px-3 py-2 font-medium">Spam Rate</th>
                    <th class="px-3 py-2 font-medium">Bytes</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-gray-700/50">
                  <For each={domainStatsRows()}>
                    {(row) => (
                      <tr class="hover:bg-slate-50/50 dark:hover:bg-slate-800/30">
                        <td class="px-3 py-2 font-medium text-slate-900 dark:text-slate-100">{row.domain}</td>
                        <td class="px-3 py-2 text-slate-700 dark:text-slate-200">{fmtMaybe(row.mail)}</td>
                        <td class="px-3 py-2 text-orange-700/90 dark:text-orange-300/90">{fmtMaybe(row.spam)}</td>
                        <td class="px-3 py-2 text-red-700/90 dark:text-red-300/90">{fmtMaybe(row.virus)}</td>
                        <td class="px-3 py-2 text-slate-700 dark:text-slate-200">{formatPct(row.spamRate)}</td>
                        <td class="px-3 py-2 text-slate-600 dark:text-slate-300">{row.bytes > 0 ? formatBytes(row.bytes) : '—'}</td>
                      </tr>
                    )}
                  </For>
                </tbody>
              </table>
            </div>
          </div>
        </Show>
      </div>
    </Card>
  );
};

export default PMGInstancePanel;
