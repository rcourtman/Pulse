/**
 * PatrolStatusBar
 *
 * One-line status bar replacing the Activity tab.
 * Shows: Running status, last run time + trigger, today's run count + new findings.
 */

import { Component, createResource, createMemo, Show } from 'solid-js';
import { getPatrolRunHistory, type PatrolRunRecord } from '@/api/patrol';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import AlertCircleIcon from 'lucide-solid/icons/alert-circle';

interface PatrolStatusBarProps {
  enabled?: boolean;
  refreshTrigger?: number;
}

export const PatrolStatusBar: Component<PatrolStatusBarProps> = (props) => {
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
      case 'scheduled': return 'Scheduled';
      case 'manual': return 'Manual';
      case 'startup': return 'Startup';
      case 'alert_fired': return 'Alert fired';
      case 'alert_cleared': return 'Alert cleared';
      case 'anomaly': return 'Anomaly';
      case 'user_action': return 'User action';
      case 'config_changed': return 'Config change';
      default: return reason ? reason.replace(/_/g, ' ') : '';
    }
  };

  const [runs] = createResource(
    () => props.refreshTrigger ?? 0,
    async (): Promise<PatrolRunRecord[]> => {
      try {
        return await getPatrolRunHistory(100);
      } catch {
        return [];
      }
    }
  );

  const stats = createMemo(() => {
    const allRuns = runs() || [];
    if (allRuns.length === 0) return null;

    const now = new Date();
    const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const todayRuns = allRuns.filter(r => new Date(r.started_at) >= todayStart);

    const lastRun = allRuns[0];
    const lastRunTime = lastRun ? new Date(lastRun.started_at) : null;
    const lastRunHadErrors = lastRun?.error_count > 0;

    return {
      runsToday: todayRuns.length,
      newFindingsToday: todayRuns.reduce((sum, r) => sum + (r.new_findings || 0), 0),
      lastRunTime: lastRunTime ? formatRelativeTime(lastRunTime) : null,
      lastRunTrigger: formatTrigger(lastRun?.trigger_reason),
      isHealthy: !lastRunHadErrors && lastRun?.status !== 'error',
    };
  });

  return (
    <Show when={!runs.loading && stats()}>
      {(s) => (
        <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 px-4 py-2">
          <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
            {/* Status */}
            <div class="flex items-center gap-1.5">
              <Show
                when={s().isHealthy}
                fallback={
                  <>
                    <AlertCircleIcon class="w-3.5 h-3.5 text-amber-500" />
                    <span class="text-amber-600 dark:text-amber-400 font-medium text-xs">Issues detected</span>
                  </>
                }
              >
                <CheckCircleIcon class="w-3.5 h-3.5 text-green-500" />
                <span class="text-green-600 dark:text-green-400 font-medium text-xs">Running normally</span>
              </Show>
            </div>

            <span class="hidden sm:inline text-gray-300 dark:text-gray-600">|</span>

            {/* Last run */}
            <Show when={s().lastRunTime}>
              <span class="text-xs text-gray-600 dark:text-gray-400">
                Last run: {s().lastRunTime}
                <Show when={s().lastRunTrigger}>
                  <span class="text-gray-500 dark:text-gray-500"> ({s().lastRunTrigger})</span>
                </Show>
              </span>
            </Show>

            <span class="hidden sm:inline text-gray-300 dark:text-gray-600">|</span>

            {/* Today */}
            <span class="text-xs text-gray-600 dark:text-gray-400">
              Today: {s().runsToday} run{s().runsToday === 1 ? '' : 's'}
              <Show when={s().newFindingsToday > 0}>
                <span class="text-amber-600 dark:text-amber-400">
                  , {s().newFindingsToday} new finding{s().newFindingsToday === 1 ? '' : 's'}
                </span>
              </Show>
            </span>
          </div>
        </div>
      )}
    </Show>
  );
};

export default PatrolStatusBar;
