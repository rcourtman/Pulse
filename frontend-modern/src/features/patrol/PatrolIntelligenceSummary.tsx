import { createMemo, For, Show } from 'solid-js';
import ActivityIcon from 'lucide-solid/icons/activity';
import ShieldAlertIcon from 'lucide-solid/icons/shield-alert';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import AlertCircleIcon from 'lucide-solid/icons/alert-circle';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';
import {
  getPatrolAssessmentAction,
  getPatrolAssessmentShellPresentation,
  getPatrolAssessmentPresentation,
  getPatrolRecencyPresentation,
  getPatrolScoreChipLabel,
  getPatrolVerificationPresentation,
  getPatrolSummaryPresentation,
  getPatrolSummaryMetricState,
} from '@/utils/patrolSummaryPresentation';
import {
  getPatrolLatestRunPresentation,
  getPatrolTriggerStatusSummary,
} from '@/utils/patrolRunPresentation';
import { getPatrolRuntimePresentation } from '@/utils/patrolRuntimePresentation';
import { formatRelativeTime } from '@/utils/format';
import type { PatrolIntelligenceState } from './usePatrolIntelligenceState';

export function PatrolIntelligenceSummary(props: { state: PatrolIntelligenceState }) {
  const state = props.state;
  const summaryStats = createMemo(() => state.summaryStats());
  const metricState = createMemo(() =>
    getPatrolSummaryMetricState({
      activeFindings: state.activePatrolFindings(),
      fixedCount: summaryStats().fixedCount,
    }),
  );
  const warningSummaryPresentation = createMemo(() =>
    getPatrolSummaryPresentation(metricState().secondarySeverity, metricState().secondaryValue > 0),
  );
  const runtimePresentation = createMemo(() =>
    getPatrolRuntimePresentation(state.runtimeState(), state.blockedReason()),
  );
  const runtimeShellPresentation = createMemo(() =>
    getPatrolAssessmentShellPresentation(runtimePresentation().tone),
  );
  const showRuntimeSummary = createMemo(() => {
    const runtimeState = state.runtimeState();
    return (
      runtimeState === 'blocked' || runtimeState === 'disabled' || runtimeState === 'unavailable'
    );
  });
  const assessment = createMemo(() =>
    getPatrolAssessmentPresentation({
      overallHealth: state.intelligenceSummary()?.overall_health,
      runtimeState: state.runtimeState(),
      blockedReason: state.blockedReason(),
      criticalFindings: summaryStats().criticalFindings,
      warningFindings: summaryStats().warningFindings,
      activeFindings: state.activePatrolFindings(),
    }),
  );
  const assessmentShellPresentation = createMemo(() =>
    getPatrolAssessmentShellPresentation(assessment().tone),
  );
  const activeFindingsSummaryPresentation = createMemo(() =>
    getPatrolSummaryPresentation(metricState().primarySeverity, metricState().primaryValue > 0),
  );
  const verification = createMemo(() =>
    getPatrolVerificationPresentation({
      runs: state.patrolRunHistory() ?? [],
      runtimeState: state.runtimeState(),
      blockedReason: state.blockedReason(),
    }),
  );
  const recency = createMemo(() =>
    getPatrolRecencyPresentation({
      runs: state.patrolRunHistory() ?? [],
      lastPatrolAt: state.patrolStatus()?.last_patrol_at,
      lastActivityAt: state.patrolStatus()?.last_activity_at,
    }),
  );
  const fixedSummaryPresentation = createMemo(() =>
    getPatrolSummaryPresentation('success', summaryStats().fixedCount > 0),
  );
  const latestRun = createMemo(() => getPatrolLatestRunPresentation(state.patrolRunHistory() ?? []));
  const triggerSummary = createMemo(() =>
    getPatrolTriggerStatusSummary(state.patrolStatus()?.trigger_status),
  );
  const circuitBreakerPresentation = createMemo(() => {
    const circuitBreaker = state.circuitBreakerStatus();
    if (!circuitBreaker || circuitBreaker.state === 'closed') {
      return undefined;
    }

    if (circuitBreaker.state === 'open') {
      return {
        message: `Provider circuit breaker tripped after ${circuitBreaker.consecutive_failures} consecutive failures.`,
        toneClass: 'text-red-600 dark:text-red-400',
      };
    }

    return {
      message: 'Provider circuit breaker recovering; the next Patrol run is a live test.',
      toneClass: 'text-amber-600 dark:text-amber-400',
    };
  });
  const scoreChipLabel = createMemo(() =>
    getPatrolScoreChipLabel({
      overallHealth: state.intelligenceSummary()?.overall_health,
      activeFindings: state.activePatrolFindings(),
    }),
  );
  const assessmentAction = createMemo(() =>
    getPatrolAssessmentAction({
      activeFindings: state.activePatrolFindings(),
    }),
  );
  const hasAttentionMetrics = createMemo(
    () => metricState().primaryValue > 0 || metricState().secondaryValue > 0,
  );
  const visibleMetrics = createMemo(() => {
    const metrics: Array<{
      icon: typeof ActivityIcon;
      key: string;
      label: string;
      presentation: ReturnType<typeof getPatrolSummaryPresentation>;
      value: number;
    }> = [];

    if (metricState().primaryValue > 0) {
      metrics.push({
        icon:
          summaryStats().criticalFindings > 0
            ? ShieldAlertIcon
            : summaryStats().totalActive > 0
              ? AlertTriangleIcon
              : ActivityIcon,
        key: 'primary',
        label: metricState().primaryLabel,
        presentation: activeFindingsSummaryPresentation(),
        value: metricState().primaryValue,
      });
    }

    if (
      metricState().secondaryValue > 0 &&
      (metricState().secondaryLabel === 'Runtime issues' ||
        metricState().secondaryValue !== metricState().primaryValue)
    ) {
      metrics.push({
        icon: AlertCircleIcon,
        key: 'secondary',
        label: metricState().secondaryLabel,
        presentation: warningSummaryPresentation(),
        value: metricState().secondaryValue,
      });
    }

    if (metricState().fixedValue > 0 && hasAttentionMetrics()) {
      metrics.push({
        icon: CheckCircleIcon,
        key: 'fixed',
        label: metricState().fixedLabel,
        presentation: fixedSummaryPresentation(),
        value: metricState().fixedValue,
      });
    }

    return metrics;
  });

  const renderAssessmentIcon = () => {
    const iconClass = `w-5 h-5 ${assessmentShellPresentation().iconClass}`;
    switch (assessment().tone) {
      case 'success':
        return <CheckCircleIcon class={iconClass} />;
      case 'error':
        return <ShieldAlertIcon class={iconClass} />;
      case 'warning':
        return <AlertTriangleIcon class={iconClass} />;
      default:
        return <AlertCircleIcon class={iconClass} />;
    }
  };

  return (
    <>
      <Show when={showRuntimeSummary()}>
        <section class="overflow-hidden rounded-md border border-border bg-surface shadow-sm">
          <div
            class={`flex flex-wrap items-center justify-between gap-3 border-b border-border-subtle px-4 py-3 ${runtimeShellPresentation().headerClass}`}
          >
            <span
              class={`inline-flex items-center rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] ${runtimeShellPresentation().badgeClass}`}
            >
              Patrol runtime
            </span>
            <Show when={recency().timestamp}>
              <p class="text-xs font-medium text-muted">
                {recency().label}{' '}
                {formatRelativeTime(recency().timestamp!, {
                  compact: true,
                  emptyText: 'never',
                })}
              </p>
            </Show>
          </div>

          <div class="px-4 py-4 sm:px-5 sm:py-5">
            <div class="flex items-start gap-3">
              <div
                class={`flex h-11 w-11 flex-shrink-0 items-center justify-center rounded-md border ${runtimeShellPresentation().iconContainerClass}`}
              >
                <AlertCircleIcon class={`w-5 h-5 ${runtimeShellPresentation().iconClass}`} />
              </div>
              <div class="min-w-0">
                <h2 class="text-lg font-semibold tracking-tight text-base-content">
                  {runtimePresentation().label}
                </h2>
                <p class="mt-1.5 max-w-3xl text-sm leading-6 text-muted">
                  {runtimePresentation().description}
                </p>
              </div>
            </div>
          </div>
        </section>
      </Show>

      <Show when={!showRuntimeSummary()}>
        <Show when={state.intelligenceSummary()}>
          {(summary) => (
            <section class="overflow-hidden rounded-md border border-border bg-surface shadow-sm">
              <div
                class={`flex flex-wrap items-center justify-between gap-3 border-b border-border-subtle px-4 py-3 ${assessmentShellPresentation().headerClass}`}
              >
                <span
                  class={`inline-flex items-center rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] ${assessmentShellPresentation().badgeClass}`}
                >
                  {assessment().eyebrow}
                </span>
                <span class="inline-flex items-center rounded-full border border-border bg-surface px-3 py-1 text-xs font-semibold text-base-content shadow-sm">
                  {scoreChipLabel()} {summary().overall_health.grade} ·{' '}
                  {Math.round(summary().overall_health.score)}/100
                </span>
              </div>

              <div class="px-4 py-4 sm:px-5 sm:py-5">
                <div class="flex items-start gap-3">
                  <div
                    class={`flex h-11 w-11 flex-shrink-0 items-center justify-center rounded-md border ${assessmentShellPresentation().iconContainerClass}`}
                  >
                    {renderAssessmentIcon()}
                  </div>

                  <div class="min-w-0 flex-1">
                    <h2 class="text-lg font-semibold tracking-tight text-base-content">
                      {assessment().title}
                    </h2>
                    <p class="mt-1.5 max-w-3xl text-sm leading-6 text-muted">
                      {assessment().description}
                    </p>
                    <Show when={assessmentAction()}>
                      {(action) => (
                        <div class="mt-4">
                          <a
                            href={action().href}
                            class="inline-flex items-center rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-semibold text-base-content shadow-sm transition-colors hover:bg-surface-hover"
                          >
                            {action().label}
                          </a>
                        </div>
                      )}
                    </Show>
                  </div>
                </div>

                <div class="mt-5 overflow-hidden rounded-md border border-border-subtle bg-surface-alt/60">
                  <div class="grid divide-y divide-border-subtle lg:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)] lg:divide-x lg:divide-y-0">
                    <div class="p-3">
                      <p class="text-xs font-semibold uppercase tracking-[0.16em] text-muted">
                        Verification
                      </p>
                      <p class="mt-1 text-sm font-medium text-base-content">
                        {verification().title}
                        <Show when={verification().lastFullRunAt}>
                          <span class="text-muted">
                            {' '}
                            ·{' '}
                            {formatRelativeTime(verification().lastFullRunAt!, {
                              compact: true,
                              emptyText: 'never',
                            })}
                          </span>
                        </Show>
                      </p>
                      <p class="mt-1 text-sm text-muted">{verification().description}</p>
                      <Show when={verification().activityMixLabel}>
                        <p class="mt-2 text-xs font-medium text-base-content">
                          Recent activity mix: {verification().activityMixLabel}
                        </p>
                      </Show>
                    </div>

                    <div class="p-3">
                      <p class="text-xs font-semibold uppercase tracking-[0.16em] text-muted">
                        Latest activity
                      </p>
                      <Show
                        when={latestRun()}
                        fallback={
                          <p class="mt-1 text-sm text-muted">
                            Patrol has not completed recent activity yet.
                          </p>
                        }
                      >
                        {(latest) => (
                          <>
                            <div class="mt-1 flex flex-wrap items-center gap-2 text-sm">
                              <span class="font-medium text-base-content">{latest().kindLabel}</span>
                              <span
                                class={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium ${latest().status.badgeClass}`}
                              >
                                {latest().status.label}
                              </span>
                              <Show when={latest().timestamp}>
                                <span class="text-muted">
                                  {formatRelativeTime(latest().timestamp!, {
                                    compact: true,
                                    emptyText: 'never',
                                  })}
                                </span>
                              </Show>
                            </div>
                            <Show when={latest().coverageSummary}>
                              <p class="mt-1 text-sm text-muted">{latest().coverageSummary}</p>
                            </Show>
                            <Show when={!latest().findingsSnapshotAvailable}>
                              <p class="mt-1 text-xs text-muted">
                                Findings snapshot unavailable for this run.
                              </p>
                            </Show>
                          </>
                        )}
                      </Show>

                      <Show when={triggerSummary()}>
                        <p class="mt-3 text-sm text-muted">
                          <span class="font-medium text-base-content">Trigger mode:</span>{' '}
                          {triggerSummary()}
                        </p>
                      </Show>

                      <Show when={circuitBreakerPresentation()}>
                        {(circuitBreaker) => (
                          <p class={`mt-3 text-sm ${circuitBreaker().toneClass}`}>
                            {circuitBreaker().message}
                          </p>
                        )}
                      </Show>
                    </div>
                  </div>
                </div>
              </div>
            </section>
          )}
        </Show>
      </Show>

      <Show when={!showRuntimeSummary() && state.intelligenceSummary() && visibleMetrics().length > 0}>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
          <For each={visibleMetrics()}>
            {(metric) => {
              const Icon = metric.icon;
              return (
                <div class="rounded-md border border-border-subtle bg-surface p-3">
                  <div class="flex items-center gap-2">
                    <div
                      class={`rounded-md border p-1.5 ${metric.presentation.iconContainerClass}`}
                    >
                      <Icon class={`h-4 w-4 ${metric.presentation.iconClass}`} />
                    </div>
                    <div>
                      <p class="text-xs text-muted">{metric.label}</p>
                      <p class={`text-lg font-bold ${metric.presentation.valueClass}`}>
                        {metric.value}
                      </p>
                    </div>
                  </div>
                </div>
              );
            }}
          </For>
        </div>
      </Show>
    </>
  );
}
