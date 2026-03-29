import { createMemo, Show } from 'solid-js';
import ActivityIcon from 'lucide-solid/icons/activity';
import ShieldAlertIcon from 'lucide-solid/icons/shield-alert';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import AlertCircleIcon from 'lucide-solid/icons/alert-circle';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';
import {
  getPatrolAssessmentAction,
  getPatrolAssessmentPresentation,
  getPatrolRecencyPresentation,
  getPatrolScoreChipLabel,
  getPatrolSummaryMetricState,
  getPatrolVerificationPresentation,
  getPatrolSummaryPresentation,
} from '@/utils/patrolSummaryPresentation';
import { getPatrolRuntimePresentation } from '@/utils/patrolRuntimePresentation';
import { getSemanticTonePresentation } from '@/utils/semanticTonePresentation';
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
  const runtimeTonePresentation = createMemo(() =>
    getSemanticTonePresentation(runtimePresentation().tone),
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
  const assessmentTonePresentation = createMemo(() =>
    getSemanticTonePresentation(assessment().tone),
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

  return (
    <>
      <Show when={showRuntimeSummary()}>
        <section class={`rounded-md border p-4 ${runtimeTonePresentation().panelClass}`}>
          <div class="flex items-start gap-3">
            <div class="flex-shrink-0 rounded-md bg-base/70 p-2">
              <AlertCircleIcon class={`w-5 h-5 ${runtimeTonePresentation().iconClass}`} />
            </div>
            <div>
              <p class="text-xs font-semibold uppercase tracking-[0.16em] text-muted">
                {runtimePresentation().title}
              </p>
              <h2 class="mt-1 text-lg font-semibold text-base-content">
                {runtimePresentation().label}
              </h2>
              <p class="mt-1 text-sm text-base-content">{runtimePresentation().description}</p>
              <Show when={recency().timestamp}>
                <p class="mt-2 text-xs text-muted">
                  {recency().label}{' '}
                  {formatRelativeTime(recency().timestamp!, {
                    compact: true,
                    emptyText: 'never',
                  })}
                  .
                </p>
              </Show>
            </div>
          </div>
        </section>
      </Show>

      <Show when={!showRuntimeSummary()}>
        <Show when={state.intelligenceSummary()}>
          {(summary) => (
            <section class={`rounded-md border p-4 ${assessmentTonePresentation().panelClass}`}>
              <div class="flex flex-wrap items-start justify-between gap-4">
                <div class="flex items-start gap-3">
                  <div class="flex-shrink-0 rounded-md bg-base/70 p-2">
                    <Show
                      when={assessment().tone === 'success'}
                      fallback={
                        <Show
                          when={assessment().tone === 'error'}
                          fallback={
                            <Show
                              when={assessment().tone === 'warning'}
                              fallback={
                                <AlertCircleIcon
                                  class={`w-5 h-5 ${assessmentTonePresentation().iconClass}`}
                                />
                              }
                            >
                              <AlertTriangleIcon
                                class={`w-5 h-5 ${assessmentTonePresentation().iconClass}`}
                              />
                            </Show>
                          }
                        >
                          <ShieldAlertIcon
                            class={`w-5 h-5 ${assessmentTonePresentation().iconClass}`}
                          />
                        </Show>
                      }
                    >
                      <CheckCircleIcon
                        class={`w-5 h-5 ${assessmentTonePresentation().iconClass}`}
                      />
                    </Show>
                  </div>

                  <div>
                    <p class="text-xs font-semibold uppercase tracking-[0.16em] text-muted">
                      {assessment().eyebrow}
                    </p>
                    <h2 class="mt-1 text-lg font-semibold text-base-content">
                      {assessment().title}
                    </h2>
                    <p class="mt-1 text-sm text-base-content">{assessment().description}</p>
                    <Show when={assessmentAction()}>
                      {(action) => (
                        <div class="mt-3">
                          <a
                            href={action().href}
                            class="inline-flex items-center rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-semibold text-base-content transition-colors hover:bg-surface-hover"
                          >
                            {action().label}
                          </a>
                        </div>
                      )}
                    </Show>
                    <div class="mt-3 flex flex-wrap items-center gap-2">
                      <span class="rounded-full border border-border-subtle bg-base px-2.5 py-1 text-xs font-medium text-base-content">
                        {scoreChipLabel()} {summary().overall_health.grade} ·{' '}
                        {Math.round(summary().overall_health.score)}/100
                      </span>
                    </div>

                    <div class="mt-4 rounded-md border border-border-subtle bg-base/90 p-3">
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
                    </div>
                  </div>
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
