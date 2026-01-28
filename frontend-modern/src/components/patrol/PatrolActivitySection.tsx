/**
 * PatrolActivitySection Component
 *
 * Displays patrol activity in a sysadmin-friendly format.
 * Clear, scannable text stats instead of abstract visualizations.
 */

import { createResource, createMemo, Show, Component, For } from 'solid-js';
import { getPatrolRunHistory, type PatrolRunRecord } from '@/api/patrol';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import AlertCircleIcon from 'lucide-solid/icons/alert-circle';
import TrendingDownIcon from 'lucide-solid/icons/trending-down';
import TrendingUpIcon from 'lucide-solid/icons/trending-up';
import MinusIcon from 'lucide-solid/icons/minus';
import ClockIcon from 'lucide-solid/icons/clock';
import ActivityIcon from 'lucide-solid/icons/activity';
import WrenchIcon from 'lucide-solid/icons/wrench';

interface PatrolActivitySectionProps {
  enabled?: boolean;
  refreshTrigger?: number;
}

export const PatrolActivitySection: Component<PatrolActivitySectionProps> = (props) => {
  const formatRelativeTime = (date: Date): string => {
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return 'just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    return `${diffDays}d ago`;
  };

  const formatTrigger = (reason?: string) => {
    switch (reason) {
      case 'scheduled':
        return 'Scheduled';
      case 'manual':
        return 'Manual';
      case 'startup':
        return 'Startup';
      case 'alert_fired':
        return 'Alert fired';
      case 'alert_cleared':
        return 'Alert cleared';
      case 'anomaly':
        return 'Anomaly';
      case 'user_action':
        return 'User action';
      case 'config_changed':
        return 'Config change';
      default:
        return reason ? reason.replace(/_/g, ' ') : '';
    }
  };

  const formatScope = (run?: PatrolRunRecord) => {
    if (!run) return '';
    const idCount = run.scope_resource_ids?.length ?? 0;
    if (idCount > 0) {
      return `Scoped to ${idCount} resource${idCount === 1 ? '' : 's'}`;
    }
    const types = run.scope_resource_types ?? [];
    if (types.length > 0) {
      return `Scoped to ${types.join(', ')}`;
    }
    if (run.type === 'scoped') {
      return 'Scoped';
    }
    return '';
  };

  const formatContext = (run?: PatrolRunRecord) => {
    if (!run?.scope_context) return '';
    const trimmed = run.scope_context.trim();
    if (!trimmed) return '';
    return trimmed.length > 48 ? `${trimmed.slice(0, 45)}…` : trimmed;
  };

  const formatDuration = (ms?: number) => {
    if (!ms || ms <= 0) return '';
    if (ms < 1000) return `${ms}ms`;
    const seconds = Math.round(ms / 1000);
    if (seconds < 60) return `${seconds}s`;
    const minutes = Math.round(seconds / 60);
    return `${minutes}m`;
  };

  const shortId = (id?: string) => {
    if (!id) return '';
    return id.length > 8 ? id.slice(0, 8) : id;
  };

  // Fetch patrol run history
  const [runs] = createResource(
    () => props.refreshTrigger ?? 0,
    async (_trigger): Promise<PatrolRunRecord[]> => {
      try {
        return await getPatrolRunHistory(100); // Get more for weekly stats
      } catch (err) {
        console.error('Failed to fetch patrol run history:', err);
        return [];
      }
    }
  );

  // Calculate stats
  const stats = createMemo(() => {
    const allRuns = runs() || [];
    if (allRuns.length === 0) {
      return null;
    }

    const now = new Date();
    const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const weekAgo = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
    const twoWeeksAgo = new Date(now.getTime() - 14 * 24 * 60 * 60 * 1000);

    // Today's runs
    const todayRuns = allRuns.filter(r => new Date(r.started_at) >= todayStart);
    const runsToday = todayRuns.length;
    const newFindingsToday = todayRuns.reduce((sum, r) => sum + (r.new_findings || 0), 0);

    // This week's runs
    const thisWeekRuns = allRuns.filter(r => new Date(r.started_at) >= weekAgo);
    const lastWeekRuns = allRuns.filter(r => {
      const date = new Date(r.started_at);
      return date >= twoWeeksAgo && date < weekAgo;
    });

    // Weekly trend calculation
    const thisWeekFindings = thisWeekRuns.reduce((sum, r) => sum + (r.new_findings || 0) + (r.existing_findings || 0), 0);
    const lastWeekFindings = lastWeekRuns.reduce((sum, r) => sum + (r.new_findings || 0) + (r.existing_findings || 0), 0);

    let weeklyTrend: 'improving' | 'stable' | 'worsening' = 'stable';
    let weeklyTrendPercent = 0;

    if (lastWeekFindings > 0) {
      const change = ((thisWeekFindings - lastWeekFindings) / lastWeekFindings) * 100;
      weeklyTrendPercent = Math.abs(Math.round(change));
      if (change < -10) {
        weeklyTrend = 'improving';
      } else if (change > 10) {
        weeklyTrend = 'worsening';
      }
    } else if (thisWeekFindings > 0) {
      weeklyTrend = 'worsening';
    }

    // Auto-resolved this week
    const autoResolvedThisWeek = thisWeekRuns.reduce((sum, r) => sum + (r.resolved_findings || 0), 0);
    const autoFixedThisWeek = thisWeekRuns.reduce((sum, r) => sum + (r.auto_fix_count || 0), 0);

    // Last run info
    const lastRun = allRuns[0];
    const lastRunTime = lastRun ? new Date(lastRun.started_at) : null;
    const lastRunStatus = lastRun?.status || 'unknown';
    const lastRunHadErrors = lastRun?.error_count > 0;
    const lastRunTrigger = lastRun?.trigger_reason;

    const lastRunMetaParts = [
      formatTrigger(lastRunTrigger),
      formatScope(lastRun),
      formatContext(lastRun),
    ].filter(Boolean);
    const lastRunMeta = lastRunMetaParts.join(' • ');

    // Format relative time
    return {
      runsToday,
      newFindingsToday,
      lastRunTime: lastRunTime ? formatRelativeTime(lastRunTime) : null,
      lastRunStatus,
      lastRunHadErrors,
      lastRunMeta,
      weeklyTrend,
      weeklyTrendPercent,
      autoResolvedThisWeek,
      autoFixedThisWeek,
      totalRuns: allRuns.length,
    };
  });

  const recentRuns = createMemo(() => {
    const allRuns = runs() || [];
    return allRuns.slice(0, 5);
  });

  const isHealthy = () => {
    const s = stats();
    return s && !s.lastRunHadErrors && s.lastRunStatus !== 'error';
  };

  return (
    <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
      {/* Loading State */}
      <Show when={runs.loading}>
        <div class="animate-pulse flex items-center gap-4">
          <div class="h-4 bg-gray-200 dark:bg-gray-700 rounded w-32" />
          <div class="h-4 bg-gray-200 dark:bg-gray-700 rounded w-48" />
          <div class="h-4 bg-gray-200 dark:bg-gray-700 rounded w-40" />
        </div>
      </Show>

      {/* Empty State */}
      <Show when={!runs.loading && (!runs() || runs()!.length === 0)}>
        <div class="text-sm text-gray-500 dark:text-gray-400">
          No patrol runs yet. Patrol will start monitoring automatically.
        </div>
      </Show>

      {/* Stats Display */}
      <Show when={!runs.loading && stats()}>
        {(s) => (
          <div class="flex flex-wrap items-center gap-x-6 gap-y-2 text-sm">
            {/* Status */}
            <div class="flex items-center gap-2">
              <Show
                when={isHealthy()}
                fallback={
                  <>
                    <AlertCircleIcon class="w-4 h-4 text-amber-500" />
                    <span class="text-amber-600 dark:text-amber-400 font-medium">Issues detected</span>
                  </>
                }
              >
                <CheckCircleIcon class="w-4 h-4 text-green-500" />
                <span class="text-green-600 dark:text-green-400 font-medium">Running normally</span>
              </Show>
            </div>

            <div class="hidden sm:block h-4 w-px bg-gray-200 dark:bg-gray-700" />

            {/* Last Run */}
            <Show when={s().lastRunTime}>
              <div class="flex items-center gap-1.5 text-gray-600 dark:text-gray-400">
                <ClockIcon class="w-3.5 h-3.5" />
                <span>
                  Last run: {s().lastRunTime}
                  <Show when={s().lastRunMeta}>
                    <span class="text-gray-500 dark:text-gray-500"> • {s().lastRunMeta}</span>
                  </Show>
                </span>
              </div>
            </Show>

            <div class="hidden sm:block h-4 w-px bg-gray-200 dark:bg-gray-700" />

            {/* Today's Activity */}
            <div class="flex items-center gap-1.5 text-gray-600 dark:text-gray-400">
              <ActivityIcon class="w-3.5 h-3.5" />
              <span>
                Today: {s().runsToday} {s().runsToday === 1 ? 'run' : 'runs'}
                <Show when={s().newFindingsToday > 0}>
                  <span class="text-amber-600 dark:text-amber-400">
                    , {s().newFindingsToday} new {s().newFindingsToday === 1 ? 'finding' : 'findings'}
                  </span>
                </Show>
              </span>
            </div>

            <div class="hidden lg:block h-4 w-px bg-gray-200 dark:bg-gray-700" />

            {/* Weekly Trend */}
            <div class="hidden lg:flex items-center gap-1.5">
              <Show when={s().weeklyTrend === 'improving'}>
                <TrendingDownIcon class="w-3.5 h-3.5 text-green-500" />
                <span class="text-green-600 dark:text-green-400">
                  {s().weeklyTrendPercent}% fewer findings this week
                </span>
              </Show>
              <Show when={s().weeklyTrend === 'worsening'}>
                <TrendingUpIcon class="w-3.5 h-3.5 text-red-500" />
                <span class="text-red-600 dark:text-red-400">
                  {s().weeklyTrendPercent}% more findings this week
                </span>
              </Show>
              <Show when={s().weeklyTrend === 'stable'}>
                <MinusIcon class="w-3.5 h-3.5 text-gray-400" />
                <span class="text-gray-500 dark:text-gray-400">
                  Findings stable this week
                </span>
              </Show>
            </div>

            {/* Auto-resolved */}
            <Show when={s().autoResolvedThisWeek > 0 || s().autoFixedThisWeek > 0}>
              <div class="hidden lg:block h-4 w-px bg-gray-200 dark:bg-gray-700" />
              <div class="hidden lg:flex items-center gap-1.5 text-green-600 dark:text-green-400">
                <WrenchIcon class="w-3.5 h-3.5" />
                <span>
                  <Show when={s().autoResolvedThisWeek > 0}>
                    {s().autoResolvedThisWeek} resolved
                  </Show>
                  <Show when={s().autoResolvedThisWeek > 0 && s().autoFixedThisWeek > 0}>, </Show>
                  <Show when={s().autoFixedThisWeek > 0}>
                    {s().autoFixedThisWeek} auto-fixed
                  </Show>
                  {' '}this week
                </span>
              </div>
            </Show>
          </div>
        )}
      </Show>

      <Show when={!runs.loading && recentRuns().length > 0}>
        <div class="mt-3 pt-3 border-t border-gray-100 dark:border-gray-700">
          <div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-2">Recent runs</div>
          <div class="space-y-1">
            <For each={recentRuns()}>
              {(run) => {
                const runMeta = [
                  formatTrigger(run.trigger_reason),
                  formatScope(run),
                  formatContext(run),
                ].filter(Boolean).join(' • ');
                const duration = formatDuration(run.duration_ms);
                return (
                  <div class="flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                    <span class="text-gray-700 dark:text-gray-300">
                      {formatRelativeTime(new Date(run.started_at))}
                    </span>
                    <span class={`px-1.5 py-0.5 rounded ${
                      run.status === 'critical'
                        ? 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
                        : run.status === 'issues_found'
                          ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
                          : run.status === 'error'
                            ? 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
                            : 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                    }`}>
                      {run.status.replace(/_/g, ' ')}
                    </span>
                    <Show when={runMeta}>
                      <span>{runMeta}</span>
                    </Show>
                    <Show when={duration}>
                      <span>• {duration}</span>
                    </Show>
                    <Show when={run.resources_checked}>
                      <span>• {run.resources_checked} resources</span>
                    </Show>
                    <Show when={run.error_count && run.error_count > 0}>
                      <span class="text-red-600 dark:text-red-400">• {run.error_count} error{run.error_count === 1 ? '' : 's'}</span>
                    </Show>
                    <Show when={run.alert_id}>
                      <span class="text-amber-600 dark:text-amber-400">• Alert {shortId(run.alert_id)}</span>
                    </Show>
                    <Show when={run.finding_id}>
                      <span class="text-blue-600 dark:text-blue-400">• Finding {shortId(run.finding_id)}</span>
                    </Show>
                  </div>
                );
              }}
            </For>
          </div>
        </div>
      </Show>
    </div>
  );
};

export default PatrolActivitySection;
