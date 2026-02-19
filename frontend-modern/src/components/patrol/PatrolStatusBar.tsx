/**
 * PatrolStatusBar
 *
 * One-line status bar replacing the Activity tab.
 * Shows: Running status, last run time + trigger, today's run count + new findings.
 */

import { Component, createResource, createMemo, Show } from 'solid-js';
import { getPatrolRunHistory, type PatrolRunRecord } from '@/api/patrol';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import { formatRelativeTime } from '@/utils/format';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import AlertCircleIcon from 'lucide-solid/icons/alert-circle';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';

interface PatrolStatusBarProps {
  enabled?: boolean;
  refreshTrigger?: number;
}

export const PatrolStatusBar: Component<PatrolStatusBarProps> = (props) => {
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
      lastRunTime: lastRunTime ? formatRelativeTime(lastRunTime, { compact: true }) : null,
      lastRunTrigger: formatTrigger(lastRun?.trigger_reason),
      isHealthy: !lastRunHadErrors && lastRun?.status !== 'error',
    };
  });

  const circuitBreaker = () => aiIntelligenceStore.circuitBreakerStatus;

  return (
    <Show when={!runs.loading && stats()}>
      {(s) => (
        <div class="bg-white dark:bg-slate-800 rounded-md border border-slate-200 dark:border-slate-700 px-4 py-2">
          {/* Circuit breaker warning */}
          <Show when={circuitBreaker()?.state === 'open'}>
            <div class="flex items-center gap-1.5 mb-1.5 pb-1.5 border-b border-red-200 dark:border-red-800">
              <AlertTriangleIcon class="w-3.5 h-3.5 text-red-500" />
              <span class="text-red-600 dark:text-red-400 font-medium text-xs">
                AI circuit breaker tripped — Patrol paused after {circuitBreaker()!.consecutive_failures} consecutive failures
              </span>
            </div>
          </Show>
          <Show when={circuitBreaker()?.state === 'half-open'}>
            <div class="flex items-center gap-1.5 mb-1.5 pb-1.5 border-b border-amber-200 dark:border-amber-800">
              <AlertCircleIcon class="w-3.5 h-3.5 text-amber-500" />
              <span class="text-amber-600 dark:text-amber-400 font-medium text-xs">
                AI circuit breaker recovering — testing with next patrol run
              </span>
            </div>
          </Show>
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

            <span class="hidden sm:inline text-slate-300 dark:text-slate-600">|</span>

            {/* Last run */}
            <Show when={s().lastRunTime}>
              <span class="text-xs text-slate-600 dark:text-slate-400">
                Last run: {s().lastRunTime}
                <Show when={s().lastRunTrigger}>
                  <span class="text-slate-500 dark:text-slate-500"> ({s().lastRunTrigger})</span>
                </Show>
              </span>
            </Show>

            <span class="hidden sm:inline text-slate-300 dark:text-slate-600">|</span>

            {/* Today */}
            <span class="text-xs text-slate-600 dark:text-slate-400">
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
