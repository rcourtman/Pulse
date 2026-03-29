/**
 * PatrolStatusBar
 *
 * One-line status bar replacing the Activity tab.
 * Shows: Running status, last run time + trigger, today's run count + new findings.
 */

import { Component, createMemo, createResource, Show } from 'solid-js';
import {
  getPatrolRunHistory,
  type PatrolRunRecord,
  type PatrolRuntimeState,
  type PatrolTriggerStatus,
} from '@/api/patrol';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import { formatRelativeTime } from '@/utils/format';
import { formatTriggerReason } from '@/utils/patrolFormat';
import {
  formatPatrolActivityBreakdown,
  getPatrolActivityBreakdown,
  getPatrolRunKindLabel,
  getPatrolRunStatusPresentation,
} from '@/utils/patrolRunPresentation';
import { getPatrolRuntimePresentation } from '@/utils/patrolRuntimePresentation';
import ActivityIcon from 'lucide-solid/icons/activity';
import AlertCircleIcon from 'lucide-solid/icons/alert-circle';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';

interface PatrolStatusBarProps {
  enabled?: boolean;
  refreshTrigger?: number;
  runtimeState?: PatrolRuntimeState;
  blockedReason?: string;
  triggerStatus?: PatrolTriggerStatus;
}

function normalizePatrolRunType(type: string | undefined): string {
  return String(type || '')
    .trim()
    .toLowerCase()
    .replace(/\s+/g, '_');
}

export const PatrolStatusBar: Component<PatrolStatusBarProps> = (props) => {
  const [runs] = createResource(
    () => props.refreshTrigger ?? 0,
    async (): Promise<PatrolRunRecord[]> => {
      try {
        return await getPatrolRunHistory(100);
      } catch {
        return [];
      }
    },
  );

  const stats = createMemo(() => {
    const allRuns = runs() || [];
    if (allRuns.length === 0) return null;

    const activityBreakdown = getPatrolActivityBreakdown(allRuns);

    const lastRun = allRuns[0];
    const lastRunTime = lastRun ? new Date(lastRun.completed_at || lastRun.started_at) : null;
    const lastRunType = normalizePatrolRunType(lastRun?.type);
    const lastRunStatus = getPatrolRunStatusPresentation(
      lastRun?.status ?? 'unknown',
      lastRun?.error_count ?? 0,
      lastRun?.finding_ids !== undefined,
    );
    return {
      activityBreakdown,
      activityBreakdownLabel: formatPatrolActivityBreakdown(activityBreakdown),
      lastRun,
      runsToday: activityBreakdown.totalRuns,
      newFindingsToday: activityBreakdown.newFindings,
      lastRunTime: lastRunTime ? formatRelativeTime(lastRunTime, { compact: true }) : null,
      lastRunTimeLabel:
        lastRunType === '' || lastRunType === 'full' || lastRunType === 'patrol'
          ? 'Last full patrol'
          : 'Last activity',
      lastRunTrigger: formatTriggerReason(lastRun?.trigger_reason),
      lastRunTypeLabel: getPatrolRunKindLabel(lastRun?.type),
      lastRunStatus,
      lastRunHasFindingsSnapshot: lastRun?.finding_ids !== undefined,
    };
  });

  const circuitBreaker = () => aiIntelligenceStore.circuitBreakerStatus;
  const runtimePresentation = createMemo(() =>
    getPatrolRuntimePresentation(props.runtimeState, props.blockedReason),
  );
  const showRuntimeState = createMemo(() => {
    const runtimeState = props.runtimeState;
    return (
      runtimeState === 'blocked' || runtimeState === 'disabled' || runtimeState === 'unavailable'
    );
  });
  const showRunInProgress = createMemo(() => props.runtimeState === 'running');
  const triggerStatusSummary = createMemo(() => {
    const status = props.triggerStatus;
    if (!status) return '';

    const notes: string[] = [];
    if (status.pending_triggers > 0) notes.push(`${status.pending_triggers} queued`);
    if (status.is_busy_mode) notes.push('busy mode');
    if (!status.alert_triggers_enabled) notes.push('alerts off');
    if (!status.anomaly_triggers_enabled) notes.push('anomalies off');
    if (notes.length === 0) return '';
    return `Scoped triggers: ${notes.join(' · ')}`;
  });
  const resolvedStats = createMemo(() => (!runs.loading ? stats() : null));

  return (
    <Show when={resolvedStats()}>
      <div class="bg-surface rounded-md border border-border px-4 py-2">
        {/* Circuit breaker warning */}
        <Show when={circuitBreaker()?.state === 'open'}>
          <div class="flex items-center gap-1.5 mb-1.5 pb-1.5 border-b border-red-200 dark:border-red-800">
            <AlertTriangleIcon class="w-3.5 h-3.5 text-red-500" />
            <span class="text-red-600 dark:text-red-400 font-medium text-xs">
              AI circuit breaker tripped — Patrol paused after{' '}
              {circuitBreaker()!.consecutive_failures} consecutive failures
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
              when={showRuntimeState()}
              fallback={
                <>
                  <ActivityIcon class="w-3.5 h-3.5 text-blue-500" />
                  <span class="text-blue-600 dark:text-blue-400 font-medium text-xs">
                    Recent activity
                  </span>
                  <Show when={showRunInProgress()}>
                    <span class="rounded border border-blue-200 bg-blue-50 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300">
                      Run in progress
                    </span>
                  </Show>
                </>
              }
            >
              <Show
                when={runtimePresentation().tone === 'warning'}
                fallback={
                  <Show
                    when={runtimePresentation().tone === 'error'}
                    fallback={<AlertCircleIcon class="w-3.5 h-3.5 text-blue-500" />}
                  >
                    <AlertTriangleIcon class="w-3.5 h-3.5 text-red-500" />
                  </Show>
                }
              >
                <AlertTriangleIcon class="w-3.5 h-3.5 text-amber-500" />
              </Show>
              <span
                class={`font-medium text-xs ${
                  runtimePresentation().tone === 'warning'
                    ? 'text-amber-600 dark:text-amber-400'
                    : runtimePresentation().tone === 'error'
                      ? 'text-red-600 dark:text-red-400'
                      : 'text-blue-600 dark:text-blue-400'
                }`}
              >
                {runtimePresentation().label}
              </span>
            </Show>
          </div>

          <span class="hidden sm:inline text-slate-300 ">|</span>

          {/* Last run */}
          <Show when={resolvedStats()!.lastRunTime}>
            <span class="text-xs text-muted">
              {resolvedStats()!.lastRunTimeLabel}: {resolvedStats()!.lastRunTime}
              <Show when={resolvedStats()!.lastRunTrigger}>
                <span class=" "> ({resolvedStats()!.lastRunTrigger})</span>
              </Show>
            </span>
          </Show>

          <span class="hidden sm:inline text-slate-300 ">|</span>

          <Show when={resolvedStats()!.lastRun}>
            <span class="text-xs text-muted">
              Latest: {resolvedStats()!.lastRunTypeLabel} <span class="text-muted">·</span>{' '}
              <span class={`px-1.5 py-0.5 rounded ${resolvedStats()!.lastRunStatus.badgeClass}`}>
                {resolvedStats()!.lastRunStatus.label}
              </span>
              <Show when={resolvedStats()!.lastRunHasFindingsSnapshot === false}>
                {' '}
                <span class="text-muted">·</span>{' '}
                <span class="text-blue-600 dark:text-blue-400">Findings snapshot unavailable</span>
              </Show>
            </span>
          </Show>

          <span class="hidden sm:inline text-slate-300 ">|</span>

          {/* Today */}
          <span class="text-xs text-muted">
            Today: {resolvedStats()!.runsToday} run{resolvedStats()!.runsToday === 1 ? '' : 's'}
            <Show when={resolvedStats()!.newFindingsToday > 0}>
              <span class="text-amber-600 dark:text-amber-400">
                , {resolvedStats()!.newFindingsToday} new finding
                {resolvedStats()!.newFindingsToday === 1 ? '' : 's'}
              </span>
            </Show>
          </span>

          <Show when={resolvedStats()!.activityBreakdownLabel}>
            <span class="hidden sm:inline text-slate-300 ">|</span>
            <span class="text-xs text-muted">
              Breakdown: {resolvedStats()!.activityBreakdownLabel}
            </span>
          </Show>

          <Show when={triggerStatusSummary()}>
            <span class="hidden sm:inline text-slate-300 ">|</span>
            <span class="text-xs text-muted">{triggerStatusSummary()}</span>
          </Show>
        </div>
      </div>
    </Show>
  );
};

export default PatrolStatusBar;
