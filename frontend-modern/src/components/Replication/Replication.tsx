import type { Component } from 'solid-js';
import { Show, For, createMemo, createSignal, createEffect, onCleanup } from 'solid-js';
import { Portal } from 'solid-js/web';
import { ProxmoxSectionNav } from '@/components/Proxmox/ProxmoxSectionNav';
import { useWebSocket } from '@/App';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { formatAbsoluteTime, formatRelativeTime } from '@/utils/format';
import { StatusDot } from '@/components/shared/StatusDot';
import { getReplicationJobStatusIndicator } from '@/utils/status';
import type { ReplicationJob } from '@/types/api';

type StatusFilter = 'all' | 'healthy' | 'warning' | 'error';

function formatDuration(durationSeconds?: number, durationHuman?: string): string {
  if (durationHuman && durationHuman.trim()) return durationHuman;
  if (!durationSeconds || durationSeconds <= 0) return '';

  const hours = Math.floor(durationSeconds / 3600).toString().padStart(2, '0');
  const minutes = Math.floor((durationSeconds % 3600) / 60).toString().padStart(2, '0');
  const seconds = Math.floor(durationSeconds % 60).toString().padStart(2, '0');

  return `${hours}:${minutes}:${seconds}`;
}

function coerceTimestamp(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value;
  if (typeof value === 'string') {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return undefined;
}

function normalizeTimestampMs(value: number): number {
  return value > 1e12 ? value : value * 1000;
}



// Countdown timer for next sync
function getTimeUntil(timestamp?: number): { text: string; isOverdue: boolean; isImminent: boolean } {
  if (!timestamp) return { text: '—', isOverdue: false, isImminent: false };

  const now = Date.now();
  // Handle both Unix timestamp (seconds) and JS timestamp (milliseconds)
  const target = timestamp > 1e12 ? timestamp : timestamp * 1000;
  const diff = target - now;

  if (diff < 0) {
    // Overdue
    const overdueMinutes = Math.abs(Math.floor(diff / 60000));
    if (overdueMinutes < 60) {
      return { text: `${overdueMinutes}m overdue`, isOverdue: true, isImminent: false };
    }
    const overdueHours = Math.floor(overdueMinutes / 60);
    return { text: `${overdueHours}h overdue`, isOverdue: true, isImminent: false };
  }

  const minutes = Math.floor(diff / 60000);
  if (minutes < 60) {
    return { text: `in ${minutes}m`, isOverdue: false, isImminent: minutes < 5 };
  }
  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  return { text: `in ${hours}h ${remainingMinutes}m`, isOverdue: false, isImminent: false };
}

// Error tooltip component
const ErrorTooltip: Component<{ error: string }> = (props) => {
  const [showTooltip, setShowTooltip] = createSignal(false);
  const [tooltipPos, setTooltipPos] = createSignal({ x: 0, y: 0 });

  const handleMouseEnter = (e: MouseEvent) => {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top });
    setShowTooltip(true);
  };

  return (
    <>
      <div
        class="mt-1 text-xs text-red-500 dark:text-red-400 line-clamp-1 cursor-help"
        onMouseEnter={handleMouseEnter}
        onMouseLeave={() => setShowTooltip(false)}
      >
        {props.error}
      </div>

      <Show when={showTooltip()}>
        <Portal mount={document.body}>
          <div
            class="fixed z-[9999] pointer-events-none"
            style={{
              left: `${tooltipPos().x}px`,
              top: `${tooltipPos().y - 8}px`,
              transform: 'translate(-50%, -100%)',
            }}
          >
            <div class="bg-gray-900 dark:bg-gray-800 text-white text-xs rounded-md shadow-lg px-3 py-2 max-w-[400px] border border-gray-700">
              <div class="font-medium text-red-400 mb-1">Error Details</div>
              <div class="text-gray-300 whitespace-pre-wrap">{props.error}</div>
            </div>
          </div>
        </Portal>
      </Show>
    </>
  );
};

const Replication: Component = () => {
  const { state, connected, reconnecting, reconnect, initialDataReceived } = useWebSocket();

  const [searchTerm, setSearchTerm] = createSignal('');
  const [statusFilter, setStatusFilter] = createSignal<StatusFilter>('all');
  let searchInputRef: HTMLInputElement | undefined;

  // Keyboard handler for type-to-search
  createEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      const isInputField =
        target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.tagName === 'SELECT' ||
        target.contentEditable === 'true';

      if (e.key === 'Escape') {
        if (searchTerm().trim() || statusFilter() !== 'all') {
          setSearchTerm('');
          setStatusFilter('all');
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

  const replicationJobs = createMemo(() => {
    const jobs = state.replicationJobs ?? [];
    return [...jobs].sort((a, b) => {
      if (a.instance !== b.instance) return a.instance.localeCompare(b.instance);
      if ((a.guestName || '') !== (b.guestName || '')) {
        return (a.guestName || '').localeCompare(b.guestName || '');
      }
      return (a.jobId || '').localeCompare(b.jobId || '');
    });
  });

  // Get job status category for filtering
  const getJobStatusCategory = (job: ReplicationJob): StatusFilter => {
    const indicator = getReplicationJobStatusIndicator(job);
    if (indicator.variant === 'success') return 'healthy';
    if (indicator.variant === 'warning') return 'warning';
    if (indicator.variant === 'danger') return 'error';
    return 'healthy';
  };

  // Filtered jobs based on search and status
  const filteredJobs = createMemo(() => {
    let jobs = replicationJobs();

    // Apply status filter
    if (statusFilter() !== 'all') {
      jobs = jobs.filter(job => getJobStatusCategory(job) === statusFilter());
    }

    // Apply search
    const term = searchTerm().toLowerCase().trim();
    if (term) {
      jobs = jobs.filter(job =>
        (job.guestName || '').toLowerCase().includes(term) ||
        (job.jobId || '').toLowerCase().includes(term) ||
        (job.sourceNode || '').toLowerCase().includes(term) ||
        (job.targetNode || '').toLowerCase().includes(term) ||
        (job.instance || '').toLowerCase().includes(term) ||
        String(job.guestId || '').includes(term)
      );
    }

    return jobs;
  });

  // Summary stats
  const stats = createMemo(() => {
    const jobs = replicationJobs();
    let healthy = 0;
    let warning = 0;
    let error = 0;
    let nextSync: { job: ReplicationJob | null; time: number | undefined } = { job: null, time: undefined };

    for (const job of jobs) {
      const category = getJobStatusCategory(job);
      if (category === 'healthy') healthy++;
      else if (category === 'warning') warning++;
      else error++;

      // Track the soonest next sync
      const nextSyncRaw = coerceTimestamp(job.nextSyncTime ?? job.nextSyncUnix);
      if (typeof nextSyncRaw === 'number') {
        const jobSyncMs = normalizeTimestampMs(nextSyncRaw);
        const currentSyncMs =
          typeof nextSync.time === 'number' ? normalizeTimestampMs(nextSync.time) : Infinity;
        if (jobSyncMs < currentSyncMs) nextSync = { job, time: nextSyncRaw };
      }
    }

    return { total: jobs.length, healthy, warning, error, nextSync };
  });

  const isLoading = createMemo(() => connected() && !initialDataReceived());

  const thClass = "px-3 py-2 text-left text-[11px] font-semibold uppercase tracking-wide text-gray-600 dark:text-gray-400";

  return (
    <div class="space-y-4">
      <ProxmoxSectionNav current="replication" />

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
            title="Loading replication data..."
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
        {/* No Jobs Empty State */}
        <Show when={replicationJobs().length === 0}>
          <Card padding="lg">
            <EmptyState
              icon={
                <svg class="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
                </svg>
              }
              title="No replication jobs detected"
              description="Replication jobs will appear here once configured in Proxmox. Replication keeps your VMs synchronized across nodes for high availability."
            />
          </Card>
        </Show>

        {/* Has Jobs - Show Content */}
        <Show when={replicationJobs().length > 0}>
          {/* Summary Cards */}
          <div class="grid gap-3 grid-cols-2 lg:grid-cols-4">
            {/* Total Jobs */}
            <Card padding="sm" tone="glass">
              <div class="flex items-center justify-between">
                <div>
                  <div class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">Total Jobs</div>
                  <div class="text-2xl font-bold text-gray-900 dark:text-gray-100 mt-1">
                    {stats().total}
                  </div>
                </div>
                <div class="p-2 bg-blue-100 dark:bg-blue-900/30 rounded-lg">
                  <svg class="w-5 h-5 text-blue-600 dark:text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
                  </svg>
                </div>
              </div>
            </Card>

            {/* Healthy */}
            <Card padding="sm" tone="glass">
              <div class="flex items-center justify-between">
                <div>
                  <div class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">Healthy</div>
                  <div class="text-2xl font-bold text-green-600 dark:text-green-400 mt-1">
                    {stats().healthy}
                  </div>
                </div>
                <div class="p-2 bg-green-100 dark:bg-green-900/30 rounded-lg">
                  <svg class="w-5 h-5 text-green-600 dark:text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                </div>
              </div>
            </Card>

            {/* Warning/Error Combined */}
            <Card padding="sm" tone={stats().error > 0 ? 'danger' : stats().warning > 0 ? 'warning' : 'glass'}>
              <div class="flex items-center justify-between">
                <div>
                  <div class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">Issues</div>
                  <div class={`text-2xl font-bold mt-1 ${stats().error > 0
                    ? 'text-red-600 dark:text-red-400'
                    : stats().warning > 0
                      ? 'text-yellow-600 dark:text-yellow-400'
                      : 'text-gray-400'
                    }`}>
                    {stats().error + stats().warning}
                  </div>
                </div>
                <div class={`p-2 rounded-lg ${stats().error > 0
                  ? 'bg-red-100 dark:bg-red-900/30'
                  : stats().warning > 0
                    ? 'bg-yellow-100 dark:bg-yellow-900/30'
                    : 'bg-gray-100 dark:bg-gray-800'
                  }`}>
                  <Show when={stats().error > 0 || stats().warning > 0} fallback={
                    <svg class="w-5 h-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                  }>
                    <svg class={`w-5 h-5 ${stats().error > 0 ? 'text-red-600 dark:text-red-400' : 'text-yellow-600 dark:text-yellow-400'}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
                    </svg>
                  </Show>
                </div>
              </div>
            </Card>

            {/* Next Sync */}
            <Card padding="sm" tone="glass">
              <div class="flex items-center justify-between">
                <div class="min-w-0 flex-1">
                  <div class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">Next Sync</div>
                  <Show when={stats().nextSync.job} fallback={
                    <div class="text-sm text-gray-400 mt-1">—</div>
                  }>
                    {(() => {
                      const timeInfo = getTimeUntil(stats().nextSync.time);
                      return (
                        <>
                          <div class={`text-lg font-bold mt-1 ${timeInfo.isOverdue
                            ? 'text-red-600 dark:text-red-400'
                            : timeInfo.isImminent
                              ? 'text-blue-600 dark:text-blue-400'
                              : 'text-gray-900 dark:text-gray-100'
                            }`}>
                            {timeInfo.text}
                          </div>
                          <div class="text-xs text-gray-500 dark:text-gray-400 truncate">
                            {stats().nextSync.job?.guestName || `VM ${stats().nextSync.job?.guestId}`}
                          </div>
                        </>
                      );
                    })()}
                  </Show>
                </div>
                <div class="p-2 bg-purple-100 dark:bg-purple-900/30 rounded-lg">
                  <svg class="w-5 h-5 text-purple-600 dark:text-purple-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                </div>
              </div>
            </Card>
          </div>

          {/* Filter Bar */}
          <Card padding="sm" tone="glass">
            <div class="flex flex-col sm:flex-row gap-3 sm:items-center sm:justify-between">
              {/* Search */}
              <div class="relative flex-1 max-w-md">
                <input
                  ref={(el) => (searchInputRef = el)}
                  type="text"
                  placeholder="Search by guest, job, or node..."
                  value={searchTerm()}
                  onInput={(e) => setSearchTerm(e.currentTarget.value)}
                  class="w-full pl-9 pr-8 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg 
                         bg-white dark:bg-gray-900 text-gray-800 dark:text-gray-200 placeholder-gray-400 dark:placeholder-gray-500
                         focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 dark:focus:border-blue-400 outline-none transition-all"
                />
                <svg class="absolute left-3 top-2 h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
                <Show when={searchTerm()}>
                  <button
                    onClick={() => setSearchTerm('')}
                    class="absolute right-2.5 top-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                  >
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  </button>
                </Show>
              </div>

              {/* Status Filter Buttons */}
              <div class="flex items-center gap-1.5">
                <span class="text-xs text-gray-500 dark:text-gray-400 mr-1">Status:</span>
                <button
                  onClick={() => setStatusFilter('all')}
                  class={`px-2.5 py-1 text-xs font-medium rounded-md transition-colors ${statusFilter() === 'all'
                    ? 'bg-gray-800 dark:bg-gray-200 text-white dark:text-gray-900'
                    : 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700'
                    }`}
                >
                  All
                </button>
                <button
                  onClick={() => setStatusFilter('healthy')}
                  class={`px-2.5 py-1 text-xs font-medium rounded-md transition-colors ${statusFilter() === 'healthy'
                    ? 'bg-green-600 text-white'
                    : 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700'
                    }`}
                >
                  Healthy
                </button>
                <button
                  onClick={() => setStatusFilter('warning')}
                  class={`px-2.5 py-1 text-xs font-medium rounded-md transition-colors ${statusFilter() === 'warning'
                    ? 'bg-yellow-500 text-white'
                    : 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700'
                    }`}
                >
                  Warning
                </button>
                <button
                  onClick={() => setStatusFilter('error')}
                  class={`px-2.5 py-1 text-xs font-medium rounded-md transition-colors ${statusFilter() === 'error'
                    ? 'bg-red-600 text-white'
                    : 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700'
                    }`}
                >
                  Error
                </button>
              </div>
            </div>
          </Card>

          {/* Jobs Table */}
          <Card padding="none" tone="glass" class="overflow-hidden">
            <Show
              when={filteredJobs().length > 0}
              fallback={
                <div class="p-8 text-center">
                  <svg class="w-10 h-10 text-gray-300 dark:text-gray-600 mx-auto mb-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" />
                  </svg>
                  <div class="text-gray-500 dark:text-gray-400 text-sm">
                    No jobs match your search
                  </div>
                  <button
                    onClick={() => { setSearchTerm(''); setStatusFilter('all'); }}
                    class="mt-2 text-xs text-blue-600 dark:text-blue-400 hover:underline"
                  >
                    Clear filters
                  </button>
                </div>
              }
            >
              <div class="overflow-x-auto">
                <table class="w-full">
                  <thead class="bg-gray-50 dark:bg-gray-800/60 border-b border-gray-200 dark:border-gray-700">
                    <tr>
                      <th class={`${thClass} pl-4 w-[160px] sm:w-[220px]`}>Guest</th>
                      <th class={`${thClass} hidden sm:table-cell`}>Job</th>
                      <th class={`${thClass} hidden md:table-cell`}>Source → Target</th>
                      <th class={`${thClass} hidden lg:table-cell`}>Last Sync</th>
                      <th class={`${thClass} hidden sm:table-cell`}>Next Sync</th>
                      <th class={thClass}>Status</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-100 dark:divide-gray-700/50">
                    <For each={filteredJobs()}>
                      {(job) => {
                        const indicator = getReplicationJobStatusIndicator(job);
                        const nextSyncInfo = getTimeUntil(job.nextSyncTime);
                        const statusCategory = getJobStatusCategory(job);

                        return (
                          <tr class={`transition-colors ${statusCategory === 'error'
                            ? 'bg-red-50/50 dark:bg-red-900/10 hover:bg-red-50 dark:hover:bg-red-900/20'
                            : statusCategory === 'warning'
                              ? 'bg-yellow-50/30 dark:bg-yellow-900/10 hover:bg-yellow-50/50 dark:hover:bg-yellow-900/20'
                              : 'hover:bg-gray-50/80 dark:hover:bg-gray-800/50'
                            }`}>
                            <td class="px-3 py-3 pl-4">
                              <div class="font-medium text-sm text-gray-900 dark:text-gray-100 truncate max-w-[140px] sm:max-w-[200px]" title={job.guestName || `VM ${job.guestId ?? ''}`}>
                                {job.guestName || `VM ${job.guestId ?? ''}`}
                              </div>
                              <div class="text-xs text-gray-500 dark:text-gray-400 flex items-center gap-1">
                                <span class="inline-flex items-center px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 font-mono">
                                  {job.instance}
                                </span>
                                <span>· ID {job.guestId ?? '—'}</span>
                              </div>
                            </td>
                            <td class="px-3 py-3 hidden sm:table-cell">
                              <div class="font-mono text-sm text-gray-700 dark:text-gray-300">{job.jobId || '—'}</div>
                              <div class="text-xs text-gray-500 dark:text-gray-400">
                                {job.schedule || '*/15'}
                              </div>
                            </td>
                            <td class="px-3 py-3 hidden md:table-cell">
                              <div class="flex items-center gap-2 text-sm">
                                <span class="text-gray-700 dark:text-gray-300">{job.sourceNode || '—'}</span>
                                <svg class="w-4 h-4 text-gray-400 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                  <path stroke-linecap="round" stroke-linejoin="round" d="M13.5 4.5L21 12m0 0l-7.5 7.5M21 12H3" />
                                </svg>
                                <span class="text-gray-700 dark:text-gray-300">{job.targetNode || '—'}</span>
                              </div>
                            </td>
                            <td class="px-3 py-3 hidden lg:table-cell">
                              <Show when={job.lastSyncTime} fallback={
                                <span class="text-gray-400 text-sm">Never</span>
                              }>
                                <div class="text-sm text-gray-900 dark:text-gray-100">
                                  {formatRelativeTime(job.lastSyncTime!)}
                                </div>
                                <div class="text-xs text-gray-500 dark:text-gray-400">
                                  {formatAbsoluteTime(job.lastSyncTime!)}
                                  <Show when={job.lastSyncDurationSeconds || job.lastSyncDurationHuman}>
                                    <span class="ml-1">({formatDuration(job.lastSyncDurationSeconds, job.lastSyncDurationHuman)})</span>
                                  </Show>
                                </div>
                              </Show>
                            </td>
                            <td class="px-3 py-3 hidden sm:table-cell">
                              <div class={`text-sm font-medium ${nextSyncInfo.isOverdue
                                ? 'text-red-600 dark:text-red-400'
                                : nextSyncInfo.isImminent
                                  ? 'text-blue-600 dark:text-blue-400'
                                  : 'text-gray-700 dark:text-gray-300'
                                }`}>
                                {nextSyncInfo.text}
                              </div>
                              <Show when={job.nextSyncTime}>
                                <div class="text-xs text-gray-500 dark:text-gray-400">
                                  {formatAbsoluteTime(job.nextSyncTime!)}
                                </div>
                              </Show>
                            </td>
                            <td class="px-3 py-3">
                              <div class="flex items-center gap-2">
                                <StatusDot
                                  variant={indicator.variant}
                                  title={indicator.label}
                                  ariaLabel={indicator.label}
                                  size="sm"
                                />
                                <span class={`text-sm font-medium ${statusCategory === 'error'
                                  ? 'text-red-700 dark:text-red-400'
                                  : statusCategory === 'warning'
                                    ? 'text-yellow-700 dark:text-yellow-400'
                                    : 'text-gray-700 dark:text-gray-300'
                                  }`}>
                                  {indicator.label}
                                </span>
                              </div>
                              <Show when={job.error}>
                                <ErrorTooltip error={job.error!} />
                              </Show>
                              <Show when={(job.failCount ?? 0) > 0}>
                                <div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                                  {job.failCount} failure{(job.failCount ?? 0) > 1 ? 's' : ''}
                                </div>
                              </Show>
                            </td>
                          </tr>
                        );
                      }}
                    </For>
                  </tbody>
                </table>
              </div>
            </Show>
          </Card>
        </Show>
      </Show>
    </div>
  );
};

export default Replication;
