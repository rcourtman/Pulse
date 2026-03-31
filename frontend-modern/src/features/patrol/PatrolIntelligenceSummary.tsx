import { createMemo, Show } from 'solid-js';
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
  getPatrolSummaryMetricState,
  getPatrolVerificationPresentation,
  getPatrolSummaryPresentation,
} from '@/utils/patrolSummaryPresentation';
import { getPatrolRuntimePresentation } from '@/utils/patrolRuntimePresentation';
import { formatRelativeTime } from '@/utils/format';
import type { PatrolIntelligenceState } from './usePatrolIntelligenceState';

export function PatrolIntelligenceSummary(props: { state: PatrolIntelligenceState }) {
  const state = props.state;
  const summaryStats = createMemo(() => state.summaryStats());
  const criticalSummaryPresentation = createMemo(() =>
    getPatrolSummaryPresentation('critical', summaryStats().criticalFindings > 0),
  );
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

                <div class="mt-5 rounded-md border border-border-subtle bg-surface-alt/60 p-3">
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
              </div>
            </section>
          )}
        </Show>
      </Show>

      <Show when={!showRuntimeSummary() && state.intelligenceSummary()}>
        <div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-3">
          <div class="bg-surface rounded-md border border-border p-3">
            <div class="flex items-center gap-2">
              <div
                class={`p-1.5 rounded-md border ${activeFindingsSummaryPresentation().iconContainerClass}`}
              >
                <Show
                  when={summaryStats().criticalFindings > 0}
                  fallback={
                    <Show
                      when={summaryStats().totalActive > 0}
                      fallback={
                        <ActivityIcon
                          class={`w-4 h-4 ${activeFindingsSummaryPresentation().iconClass}`}
                        />
                      }
                    >
                      <AlertTriangleIcon
                        class={`w-4 h-4 ${activeFindingsSummaryPresentation().iconClass}`}
                      />
                    </Show>
                  }
                >
                  <ShieldAlertIcon
                    class={`w-4 h-4 ${activeFindingsSummaryPresentation().iconClass}`}
                  />
                </Show>
              </div>
              <div>
                <p class="text-xs text-muted">{metricState().primaryLabel}</p>
                <p class={`text-lg font-bold ${activeFindingsSummaryPresentation().valueClass}`}>
                  {metricState().primaryValue}
                </p>
              </div>
            </div>
          </div>

          <div class="bg-surface rounded-md border border-border p-3">
            <div class="flex items-center gap-2">
              <div
                class={`p-1.5 rounded-md border ${warningSummaryPresentation().iconContainerClass}`}
              >
                <ActivityIcon class={`w-4 h-4 ${warningSummaryPresentation().iconClass}`} />
              </div>
              <div>
                <p class="text-xs text-muted">{metricState().secondaryLabel}</p>
                <p class={`text-lg font-bold ${warningSummaryPresentation().valueClass}`}>
                  {metricState().secondaryValue}
                </p>
              </div>
            </div>
          </div>

          <div class="bg-surface rounded-md border border-border p-3">
            <div class="flex items-center gap-2">
              <div
                class={`p-1.5 rounded-md border ${criticalSummaryPresentation().iconContainerClass}`}
              >
                <ShieldAlertIcon class={`w-4 h-4 ${criticalSummaryPresentation().iconClass}`} />
              </div>
              <div>
                <p class="text-xs text-muted">{metricState().criticalLabel}</p>
                <p class={`text-lg font-bold ${criticalSummaryPresentation().valueClass}`}>
                  {metricState().criticalValue}
                </p>
              </div>
            </div>
          </div>

          <div class="bg-surface rounded-md border border-border p-3">
            <div class="flex items-center gap-2">
              <div
                class={`p-1.5 rounded-md border ${fixedSummaryPresentation().iconContainerClass}`}
              >
                <CheckCircleIcon class={`w-4 h-4 ${fixedSummaryPresentation().iconClass}`} />
              </div>
              <div>
                <p class="text-xs text-muted">{metricState().fixedLabel}</p>
                <p class={`text-lg font-bold ${fixedSummaryPresentation().valueClass}`}>
                  {metricState().fixedValue}
                </p>
              </div>
            </div>
          </div>
        </div>
      </Show>
    </>
  );
}
