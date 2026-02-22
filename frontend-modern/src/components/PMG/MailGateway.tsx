import { Component, Show, For, createMemo, createSignal, createEffect, onCleanup } from 'solid-js';
import { useTooltip } from '@/hooks/useTooltip';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import { useWebSocket } from '@/App';
import { Card } from '@/components/shared/Card';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/shared/Table';
import { EmptyState } from '@/components/shared/EmptyState';
import { formatRelativeTime, formatBytes } from '@/utils/format';

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

const parseTimestamp = (value?: string) => {
  if (!value) return undefined;
  const time = Date.parse(value);
  return Number.isNaN(time) ? undefined : time;
};

// Threat rate bar component
const ThreatBar: Component<{
  percent: number;
  color: 'spam' | 'virus' | 'quarantine';
  label: string;
  count: number;
}> = (props) => {
  const barColor = () => {
    switch (props.color) {
      case 'spam': return 'bg-orange-500';
      case 'virus': return 'bg-red-500';
      case 'quarantine': return 'bg-yellow-500';
    }
  };

  const textColor = () => {
    switch (props.color) {
      case 'spam': return 'text-orange-600 dark:text-orange-400';
      case 'virus': return 'text-red-600 dark:text-red-400';
      case 'quarantine': return 'text-yellow-600 dark:text-yellow-400';
    }
  };

  return (
    <div class="space-y-1">
      <div class="flex items-center justify-between text-xs">
        <span class="text-muted">{props.label}</span>
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

// Instance status badge
const StatusBadge: Component<{ status: string; health?: string }> = (props) => {
  const statusInfo = createMemo(() => {
    const status = (props.health || props.status || '').toLowerCase();
    if (status.includes('healthy') || status === 'online') {
      return { bg: 'bg-green-100 dark:bg-green-900', text: 'text-green-700 dark:text-green-400', dot: 'bg-green-500', label: 'Healthy' };
    }
    if (status.includes('degraded') || status.includes('warning')) {
      return { bg: 'bg-yellow-100 dark:bg-yellow-900', text: 'text-yellow-700 dark:text-yellow-400', dot: 'bg-yellow-500', label: 'Degraded' };
    }
    if (status.includes('error') || status === 'offline') {
      return { bg: 'bg-red-100 dark:bg-red-900', text: 'text-red-700 dark:text-red-400', dot: 'bg-red-500', label: 'Offline' };
    }
    return { bg: 'bg-slate-100 dark:bg-slate-800', text: 'text-muted', dot: 'bg-slate-400', label: status || 'Unknown' };
  });

  return (
    <span class={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase ${statusInfo().bg} ${statusInfo().text}`}>
      <span class={`w-1.5 h-1.5 rounded-full ${statusInfo().dot}`} />
      {statusInfo().label}
    </span>
  );
};

// Queue depth indicator with tooltip
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
        onMouseEnter={(e) => { if (props.queue) tip.onMouseEnter(e); }}
        onMouseLeave={tip.onMouseLeave}
      >
        <Show when={hasQueue()} fallback={
          <span class="text-xs text-slate-400">Empty</span>
        }>
          <span class={`text-xs font-medium ${queueSeverity() === 'high' ? 'text-red-600 dark:text-red-400' :
            queueSeverity() === 'medium' ? 'text-yellow-600 dark:text-yellow-400' :
              'text-green-600 dark:text-green-400'
            }`}>
            {formatNum(queueTotal())} msgs
          </span>
        </Show>
      </div>

      <TooltipPortal when={tip.show() && !!props.queue} x={tip.pos().x} y={tip.pos().y}>
        <div class="min-w-[140px]">
          <div class="font-medium mb-1.5 text-slate-300 border-b border-slate-700 pb-1">Queue Breakdown</div>
          <div class="space-y-0.5 text-[11px]">
            <div class="flex justify-between"><span class="text-slate-400">Active</span><span>{props.queue?.active || 0}</span></div>
            <div class="flex justify-between"><span class="text-slate-400">Deferred</span><span>{props.queue?.deferred || 0}</span></div>
            <div class="flex justify-between"><span class="text-slate-400">Hold</span><span>{props.queue?.hold || 0}</span></div>
            <div class="flex justify-between"><span class="text-slate-400">Incoming</span><span>{props.queue?.incoming || 0}</span></div>
            <Show when={(props.queue?.oldestAge || 0) > 0}>
              <div class="flex justify-between pt-1 mt-1 border-t border-slate-700">
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

const MailGateway: Component = () => {
  const { state, connected, reconnecting, reconnect, initialDataReceived } = useWebSocket();

  const [searchTerm, setSearchTerm] = createSignal('');
  let searchInputRef: HTMLInputElement | undefined;

  // Keyboard handler for type-
  createEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      const isInputField =
        target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.tagName === 'SELECT' ||
        target.contentEditable === 'true';

      if (e.key === 'Escape') {
        if (searchTerm().trim()) {
          setSearchTerm('');
          searchInputRef?.blur();
        }
      } else if (!isInputField && e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
        if (searchInputRef) {
          searchInputRef.focus();
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    onCleanup(() => document.removeEventListener('keydown', handleKeyDown));
  });

  const instances = createMemo(() => state.pmg ?? []);
  const hasInstances = createMemo(() => instances().length > 0);

  // Filter instances by search
  const filteredInstances = createMemo(() => {
    const term = searchTerm().toLowerCase().trim();
    if (!term) return instances();
    return instances().filter(pmg =>
      pmg.name.toLowerCase().includes(term) ||
      (pmg.host || '').toLowerCase().includes(term) ||
      (pmg.version || '').toLowerCase().includes(term)
    );
  });

  // Aggregate stats across all instances
  const aggregateStats = createMemo(() => {
    const pmgs = instances();
    let totalMail = 0;
    let totalSpam = 0;
    let totalVirus = 0;
    let totalQuarantine = 0;
    let totalQueue = 0;
    let healthyCount = 0;
    let issueCount = 0;

    for (const pmg of pmgs) {
      totalMail += pmg.mailStats?.countTotal || 0;
      totalSpam += pmg.mailStats?.spamIn || 0;
      totalVirus += pmg.mailStats?.virusIn || 0;

      const q = pmg.quarantine;
      if (q) {
        totalQuarantine += (q.spam || 0) + (q.virus || 0) + (q.attachment || 0) + (q.blacklisted || 0);
      }

      // Sum queue across all nodes
      if (pmg.nodes) {
        for (const node of pmg.nodes) {
          totalQueue += node.queueStatus?.total || 0;
        }
      }

      const status = (pmg.connectionHealth || pmg.status || '').toLowerCase();
      if (status.includes('healthy') || status === 'online') {
        healthyCount++;
      } else {
        issueCount++;
      }
    }

    const spamRate = totalMail > 0 ? (totalSpam / totalMail) * 100 : 0;
    const virusRate = totalMail > 0 ? (totalVirus / totalMail) * 100 : 0;

    return {
      totalMail,
      totalSpam,
      totalVirus,
      totalQuarantine,
      totalQueue,
      spamRate,
      virusRate,
      healthyCount,
      issueCount,
      instanceCount: pmgs.length,
    };
  });

  const isLoading = createMemo(() => connected() && !initialDataReceived());

  return (
    <div class="space-y-4">
      {/* Loading State */}
      <Show when={isLoading()}>
        <Card padding="lg">
          <EmptyState
            icon={
              <svg class="h-12 w-12 animate-spin text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
              </svg>
            }
            title="Loading mail gateway data..."
            description="Connecting to the monitoring service."
          />
        </Card>
      </Show>

      {/* Disconnected State */}
      <Show when={!connected() && !isLoading()}>
        <Card padding="lg" tone="danger">
          <EmptyState
            icon={
              <svg class="h-12 w-12 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            }
            title="Connection lost"
            description={reconnecting() ? 'Attempting to reconnect…' : 'Unable to connect to the backend server'}
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

      <Show when={connected() && initialDataReceived()}>
        {/* No Instances Empty State */}
        <Show when={!hasInstances()}>
          <Card padding="lg">
            <EmptyState
              icon={
                <svg class="h-12 w-12 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M21.75 6.75v10.5a2.25 2.25 0 01-2.25 2.25h-15a2.25 2.25 0 01-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25m19.5 0v.243a2.25 2.25 0 01-1.07 1.916l-7.5 4.615a2.25 2.25 0 01-2.36 0L3.32 8.91a2.25 2.25 0 01-1.07-1.916V6.75" />
                </svg>
              }
              title="No Mail Gateways configured"
              description="Add a Proxmox Mail Gateway via Settings → Nodes to start collecting mail analytics and security metrics."
            />
          </Card>
        </Show>

        {/* Has Instances - Show Content */}
        <Show when={hasInstances()}>
          {/* Summary Cards */}
          <div class="grid gap-3 grid-cols-2 lg:grid-cols-5">
            {/* Total Mail */}
            <Card padding="sm" tone="card">
              <div class="flex items-center justify-between">
                <div>
                  <div class="text-xs font-medium text-muted uppercase tracking-wide">Mail (24h)</div>
                  <div class="text-2xl font-bold text-base-content mt-1">
                    {formatCompact(aggregateStats().totalMail)}
                  </div>
                  <div class="text-xs text-muted">
                    ~{formatNum(Math.round(aggregateStats().totalMail / 24))}/hr
                  </div>
                </div>
                <div class="p-2 bg-blue-100 dark:bg-blue-900 rounded-md">
                  <svg class="w-5 h-5 text-blue-600 dark:text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M21.75 6.75v10.5a2.25 2.25 0 01-2.25 2.25h-15a2.25 2.25 0 01-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25m19.5 0v.243a2.25 2.25 0 01-1.07 1.916l-7.5 4.615a2.25 2.25 0 01-2.36 0L3.32 8.91a2.25 2.25 0 01-1.07-1.916V6.75" />
                  </svg>
                </div>
              </div>
            </Card>

            {/* Spam Rate */}
            <Card padding="sm" tone={aggregateStats().spamRate > 50 ? 'warning' : 'card'}>
              <div class="flex items-center justify-between">
                <div>
                  <div class="text-xs font-medium text-muted uppercase tracking-wide">Spam</div>
                  <div class={`text-2xl font-bold mt-1 ${aggregateStats().spamRate > 50 ? 'text-orange-600 dark:text-orange-400' : 'text-base-content'}`}>
                    {formatPct(aggregateStats().spamRate)}
                  </div>
                  <div class="text-xs text-muted">
                    {formatCompact(aggregateStats().totalSpam)} caught
                  </div>
                </div>
                <div class={`p-2 rounded-md ${aggregateStats().spamRate > 50 ? 'bg-orange-100 dark:bg-orange-900' : 'bg-orange-100 dark:bg-orange-900'}`}>
                  <svg class="w-5 h-5 text-orange-600 dark:text-orange-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
                  </svg>
                </div>
              </div>
            </Card>

            {/* Virus Blocked */}
            <Card padding="sm" tone={aggregateStats().totalVirus > 0 ? 'danger' : 'card'}>
              <div class="flex items-center justify-between">
                <div>
                  <div class="text-xs font-medium text-muted uppercase tracking-wide">Viruses</div>
                  <div class={`text-2xl font-bold mt-1 ${aggregateStats().totalVirus > 0 ? 'text-red-600 dark:text-red-400' : 'text-base-content'}`}>
                    {formatCompact(aggregateStats().totalVirus)}
                  </div>
                  <div class="text-xs text-muted">
                    blocked today
                  </div>
                </div>
                <div class={`p-2 rounded-md ${aggregateStats().totalVirus > 0 ? 'bg-red-100 dark:bg-red-900' : 'bg-slate-100 dark:bg-slate-800'}`}>
                  <svg class={`w-5 h-5 ${aggregateStats().totalVirus > 0 ? 'text-red-600 dark:text-red-400' : 'text-slate-400'}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m0-10.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.75c0 5.592 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.57-.598-3.75h-.152c-3.196 0-6.1-1.249-8.25-3.286zm0 13.036h.008v.008H12v-.008z" />
                  </svg>
                </div>
              </div>
            </Card>

            {/* Quarantine */}
            <Card padding="sm" tone="card">
              <div class="flex items-center justify-between">
                <div>
                  <div class="text-xs font-medium text-muted uppercase tracking-wide">Quarantine</div>
                  <div class="text-2xl font-bold text-base-content mt-1">
                    {formatCompact(aggregateStats().totalQuarantine)}
                  </div>
                  <div class="text-xs text-muted">
                    items held
                  </div>
                </div>
                <div class="p-2 bg-yellow-100 dark:bg-yellow-900 rounded-md">
                  <svg class="w-5 h-5 text-yellow-600 dark:text-yellow-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5m6 4.125l2.25 2.25m0 0l2.25 2.25M12 13.875l2.25-2.25M12 13.875l-2.25 2.25M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" />
                  </svg>
                </div>
              </div>
            </Card>

            {/* Queue Status */}
            <Card padding="sm" tone={aggregateStats().totalQueue > 50 ? 'warning' : 'card'}>
              <div class="flex items-center justify-between">
                <div>
                  <div class="text-xs font-medium text-muted uppercase tracking-wide">Queue</div>
                  <div class={`text-2xl font-bold mt-1 ${aggregateStats().totalQueue > 100 ? 'text-red-600 dark:text-red-400' :
                    aggregateStats().totalQueue > 20 ? 'text-yellow-600 dark:text-yellow-400' :
                      'text-green-600 dark:text-green-400'
                    }`}>
                    {formatNum(aggregateStats().totalQueue)}
                  </div>
                  <div class="text-xs text-muted">
                    {aggregateStats().totalQueue === 0 ? 'all clear' : 'pending'}
                  </div>
                </div>
                <div class={`p-2 rounded-md ${aggregateStats().totalQueue > 50 ? 'bg-yellow-100 dark:bg-yellow-900' : 'bg-green-100 dark:bg-green-900'
                  }`}>
                  <svg class={`w-5 h-5 ${aggregateStats().totalQueue > 50 ? 'text-yellow-600 dark:text-yellow-400' : 'text-green-600 dark:text-green-400'
                    }`} fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 12h16.5m-16.5 3.75h16.5M3.75 19.5h16.5M5.625 4.5h12.75a1.875 1.875 0 010 3.75H5.625a1.875 1.875 0 010-3.75z" />
                  </svg>
                </div>
              </div>
            </Card>
          </div>

          {/* Search Bar (only show if multiple instances) */}
          <Show when={instances().length > 1}>
            <Card padding="sm" tone="card">
              <div class="relative max-w-md">
                <input
                  ref={(el) => (searchInputRef = el)}
                  type="text"
                  placeholder="Search gateways..."
                  value={searchTerm()}
                  onInput={(e) => setSearchTerm(e.currentTarget.value)}
                  class="w-full pl-9 pr-8 py-1.5 text-sm border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-900 text-base-content placeholder-muted focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:focus:border-blue-400 outline-none transition-all"
                />
                <svg class="absolute left-3 top-2 h-4 w-4 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
                <Show when={searchTerm()}>
                  <button
                    onClick={() => setSearchTerm('')}
                    class="absolute right-2.5 top-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300"
                  >
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  </button>
                </Show>
              </div>
            </Card>
          </Show>

          {/* Instance Cards */}
          <Show
            when={filteredInstances().length > 0}
            fallback={
              <Card padding="lg">
                <div class="text-center py-4">
                  <svg class="w-10 h-10 text-slate-300 dark:text-slate-600 mx-auto mb-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" />
                  </svg>
                  <div class="text-muted text-sm">No gateways match "{searchTerm()}"</div>
                  <button
                    onClick={() => setSearchTerm('')}
                    class="mt-2 text-xs text-blue-600 dark:text-blue-400 hover:underline"
                  >
                    Clear search
                  </button>
                </div>
              </Card>
            }
          >
            <div class="space-y-4">
              <For each={filteredInstances()}>
                {(pmg) => {
                  const lastUpdated = () => {
                    const ts = parseTimestamp(pmg.mailStats?.updatedAt) ?? parseTimestamp(pmg.lastUpdated) ?? parseTimestamp(pmg.lastSeen);
                    return ts ? formatRelativeTime(ts) : '—';
                  };

                  const stats = createMemo(() => {
                    const m = pmg.mailStats;
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

                    const q = pmg.quarantine;
                    const qSpam = q?.spam || 0;
                    const qVirus = q?.virus || 0;
                    const qAttachment = q?.attachment || 0;
                    const qBlacklist = q?.blacklisted || 0;
                    const qTotal = qSpam + qVirus + qAttachment + qBlacklist;

                    const spamPct = total > 0 ? (spam / total) * 100 : 0;
                    const virusPct = total > 0 ? (virus / total) * 100 : 0;
                    const quarantinePct = inbound > 0 ? (qTotal / inbound) * 100 : 0;

                    return {
                      total, inbound, outbound, spam, virus, bytesIn, bytesOut,
                      rbl, pregreet, greylist, bouncesIn, bouncesOut,
                      qSpam, qVirus, qAttachment, qBlacklist, qTotal,
                      spamPct, virusPct, quarantinePct,
                    };
                  });

                  // Get aggregate queue for this instance
                  const instanceQueue = createMemo(() => {
                    if (!pmg.nodes) return { total: 0 };
                    let total = 0, active = 0, deferred = 0, hold = 0, incoming = 0, oldestAge = 0;
                    for (const node of pmg.nodes) {
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

                  return (
                    <Card padding="none" tone="card" class="overflow-hidden">
                      {/* Instance Header */}
                      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2 px-4 py-3 bg-slate-50 dark:bg-slate-800 border-b border-border">
                        <div class="flex items-center gap-3">
                          <a
                            href={pmg.host || `https://${pmg.name}:8006`}
                            target="_blank"
                            rel="noopener noreferrer"
                            class="text-base font-semibold text-base-content hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
                          >
                            {pmg.name}
                          </a>
                          <StatusBadge status={pmg.status || ''} health={pmg.connectionHealth} />
                          <Show when={pmg.version}>
                            <span class="text-xs text-muted bg-slate-100 dark:bg-slate-700 px-1.5 py-0.5 rounded">
                              v{pmg.version}
                            </span>
                          </Show>
                        </div>
                        <div class="flex items-center gap-4 text-xs text-muted">
                          <QueueIndicator queue={instanceQueue()} />
                          <span>Updated {lastUpdated()}</span>
                        </div>
                      </div>

                      {/* Main Content */}
                      <div class="p-4">
                        {/* Threat Rate Bars */}
                        <div class="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-4">
                          <ThreatBar
                            percent={stats().spamPct}
                            color="spam"
                            label="Spam Rate"
                            count={stats().spam}
                          />
                          <ThreatBar
                            percent={stats().virusPct}
                            color="virus"
                            label="Virus Rate"
                            count={stats().virus}
                          />
                          <ThreatBar
                            percent={stats().quarantinePct}
                            color="quarantine"
                            label="Quarantine Rate"
                            count={stats().qTotal}
                          />
                        </div>

                        {/* Stats Grid */}
                        <div class="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-6 gap-3">
                          {/* Mail Flow */}
                          <div class="bg-slate-50 dark:bg-slate-800 rounded-md p-3">
                            <div class="text-xs text-muted mb-1">Total (24h)</div>
                            <div class="text-lg font-bold text-base-content">{formatCompact(stats().total)}</div>
                            <div class="text-[10px] text-slate-400">{formatNum(Math.round(stats().total / 24))}/hr</div>
                          </div>
                          <div class="bg-slate-50 dark:bg-slate-800 rounded-md p-3">
                            <div class="text-xs text-muted mb-1">Inbound</div>
                            <div class="text-lg font-bold text-base-content">{formatCompact(stats().inbound)}</div>
                            <div class="text-[10px] text-slate-400">{formatBytes(stats().bytesIn)}</div>
                          </div>
                          <div class="bg-slate-50 dark:bg-slate-800 rounded-md p-3">
                            <div class="text-xs text-muted mb-1">Outbound</div>
                            <div class="text-lg font-bold text-base-content">{formatCompact(stats().outbound)}</div>
                            <div class="text-[10px] text-slate-400">{formatBytes(stats().bytesOut)}</div>
                          </div>

                          {/* Threats */}
                          <div class="bg-orange-50 dark:bg-orange-900 rounded-md p-3">
                            <div class="text-xs text-orange-600 dark:text-orange-400 mb-1">Spam</div>
                            <div class="text-lg font-bold text-orange-600 dark:text-orange-400">{formatCompact(stats().spam)}</div>
                            <div class="text-[10px] text-orange-500">{formatPct(stats().spamPct)} rate</div>
                          </div>
                          <div class="bg-red-50 dark:bg-red-900 rounded-md p-3">
                            <div class="text-xs text-red-600 dark:text-red-400 mb-1">Viruses</div>
                            <div class="text-lg font-bold text-red-600 dark:text-red-400">{formatCompact(stats().virus)}</div>
                            <div class="text-[10px] text-red-500">{formatPct(stats().virusPct)} rate</div>
                          </div>
                          <div class="bg-yellow-50 dark:bg-yellow-900 rounded-md p-3">
                            <div class="text-xs text-yellow-600 dark:text-yellow-400 mb-1">Quarantine</div>
                            <div class="text-lg font-bold text-yellow-600 dark:text-yellow-400">{formatCompact(stats().qTotal)}</div>
                            <div class="text-[10px] text-yellow-500">{formatPct(stats().quarantinePct)} of inbound</div>
                          </div>
                        </div>

                        {/* Delivery Health Row */}
                        <div class="mt-3 pt-3 border-t border-border">
                          <div class="text-xs font-medium text-muted uppercase tracking-wide mb-2">Delivery Health</div>
                          <div class="flex flex-wrap gap-x-6 gap-y-1 text-xs">
                            <div class="flex items-center gap-2">
                              <span class="text-muted">RBL Rejects:</span>
                              <span class="font-medium text-slate-700 dark:text-slate-300">{formatNum(stats().rbl)}</span>
                            </div>
                            <div class="flex items-center gap-2">
                              <span class="text-muted">Pregreet:</span>
                              <span class="font-medium text-slate-700 dark:text-slate-300">{formatNum(stats().pregreet)}</span>
                            </div>
                            <div class="flex items-center gap-2">
                              <span class="text-muted">Greylisted:</span>
                              <span class="font-medium text-slate-700 dark:text-slate-300">{formatNum(stats().greylist)}</span>
                            </div>
                            <div class="flex items-center gap-2">
                              <span class="text-muted">Bounces In/Out:</span>
                              <span class="font-medium text-slate-700 dark:text-slate-300">{formatNum(stats().bouncesIn)}/{formatNum(stats().bouncesOut)}</span>
                            </div>
                            <Show when={pmg.mailStats?.averageProcessTimeMs}>
                              <div class="flex items-center gap-2">
                                <span class="text-muted">Avg Process:</span>
                                <span class="font-medium text-slate-700 dark:text-slate-300">{formatDec((pmg.mailStats?.averageProcessTimeMs || 0) / 1000, 2)}s</span>
                              </div>
                            </Show>
                          </div>
                        </div>

                        {/* Cluster Nodes */}
                        <Show when={(pmg.nodes?.length ?? 0) > 0}>
                          <div class="mt-3 pt-3 border-t border-border">
                            <div class="text-xs font-medium text-muted uppercase tracking-wide mb-2">
                              Cluster Nodes ({pmg.nodes?.length})
                            </div>
                            <div class="overflow-x-auto -mx-4 px-4">
                              <Table class="w-full min-w-[600px] text-xs">
                                <TableHeader>
                                  <TableRow class="text-left text-[10px] uppercase tracking-wide text-muted border-b border-border">
                                    <TableHead class="pb-1.5 font-medium">Node</TableHead>
                                    <TableHead class="pb-1.5 font-medium">Status</TableHead>
                                    <TableHead class="pb-1.5 font-medium">Uptime</TableHead>
                                    <TableHead class="pb-1.5 font-medium">Load</TableHead>
                                    <TableHead class="pb-1.5 font-medium">Queue</TableHead>
                                  </TableRow>
                                </TableHeader>
                                <TableBody class="divide-y divide-gray-100 dark:divide-gray-700">
                                  <For each={pmg.nodes}>
                                    {(node) => {
                                      const isOnline = (node.status || '').toLowerCase() === 'online';

                                      return (
                                        <TableRow class="hover:bg-slate-50 dark:hover:bg-slate-800">
                                          <TableCell class="py-1.5 font-medium text-base-content">{node.name}</TableCell>
                                          <TableCell class="py-1.5">
                                            <span class={`inline-flex items-center gap-1 ${isOnline ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}`}>
                                              <span class={`w-1.5 h-1.5 rounded-full ${isOnline ? 'bg-green-500' : 'bg-red-500'}`} />
                                              {node.status || 'unknown'}
                                            </span>
                                          </TableCell>
                                          <TableCell class="py-1.5 text-muted">
                                            <Show when={node.uptime} fallback="—">
                                              {Math.floor((node.uptime ?? 0) / 86400)}d {Math.floor(((node.uptime ?? 0) % 86400) / 3600)}h
                                            </Show>
                                          </TableCell>
                                          <TableCell class="py-1.5 text-muted">{node.loadAvg || '—'}</TableCell>
                                          <TableCell class="py-1.5">
                                            <QueueIndicator queue={node.queueStatus} />
                                          </TableCell>
                                        </TableRow>
                                      );
                                    }}
                                  </For>
                                </TableBody>
                              </Table>
                            </div>
                          </div>
                        </Show>

                        {/* Spam Distribution */}
                        <Show when={(pmg.spamDistribution?.length ?? 0) > 0}>
                          <div class="mt-3 pt-3 border-t border-border">
                            <div class="text-xs font-medium text-muted uppercase tracking-wide mb-2">Spam Score Distribution</div>
                            <div class="flex gap-1 overflow-x-auto pb-1">
                              <For each={pmg.spamDistribution}>
                                {(bucket) => {
                                  const totalScored = pmg.spamDistribution?.reduce((sum, b) => sum + b.count, 0) || 1;
                                  const pct = (bucket.count / totalScored) * 100;
                                  return (
                                    <div class="flex-shrink-0 text-center bg-slate-50 dark:bg-slate-800 rounded px-2 py-1.5 min-w-[50px]">
                                      <div class="text-[10px] text-muted">{bucket.score}</div>
                                      <div class="text-xs font-semibold text-base-content">{formatCompact(bucket.count)}</div>
                                      <div class="text-[10px] text-slate-400">{formatDec(pct)}%</div>
                                    </div>
                                  );
                                }}
                              </For>
                            </div>
                          </div>
                        </Show>
                      </div>
                    </Card>
                  );
                }}
              </For>
            </div>
          </Show>
        </Show>
      </Show>
    </div>
  );
};

export default MailGateway;
