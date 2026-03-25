import { createMemo, Show } from 'solid-js';
import ActivityIcon from 'lucide-solid/icons/activity';
import ShieldAlertIcon from 'lucide-solid/icons/shield-alert';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import AlertCircleIcon from 'lucide-solid/icons/alert-circle';
import {
  getPatrolSummaryPresentation,
  PATROL_NO_ISSUES_LABEL,
} from '@/utils/patrolSummaryPresentation';
import { getPatrolRuntimePresentation } from '@/utils/patrolRuntimePresentation';
import { getSemanticTonePresentation } from '@/utils/semanticTonePresentation';
import { ResourcePolicySummary } from '@/components/Infrastructure/ResourcePolicySummary';
import { ResourceCorrelationSummary } from '@/components/Infrastructure/ResourceCorrelationSummary';
import { ResourceChangeSummary } from '@/components/Infrastructure/ResourceChangeSummary';
import { formatRelativeTime } from '@/utils/format';
import type { PatrolIntelligenceState } from './usePatrolIntelligenceState';

export function PatrolIntelligenceSummary(props: { state: PatrolIntelligenceState }) {
  const state = props.state;
  const criticalSummaryPresentation = createMemo(() =>
    getPatrolSummaryPresentation('critical', state.summaryStats().criticalFindings > 0),
  );
  const warningSummaryPresentation = createMemo(() =>
    getPatrolSummaryPresentation('warning', state.summaryStats().warningFindings > 0),
  );
  const fixedSummaryPresentation = createMemo(() =>
    getPatrolSummaryPresentation('success', state.summaryStats().fixedCount > 0),
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
              <Show when={state.patrolStatus()?.last_patrol_at}>
                <p class="mt-2 text-xs text-muted">
                  Last completed patrol{' '}
                  {formatRelativeTime(state.patrolStatus()!.last_patrol_at, {
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
            <section class="rounded-md border border-border bg-surface p-4">
              <div class="flex flex-wrap items-start justify-between gap-4">
                <div>
                  <p class="text-xs font-semibold uppercase tracking-[0.16em] text-muted">
                    Patrol summary
                  </p>
                  <h2 class="mt-1 text-lg font-semibold text-base-content">
                    Health {summary().overall_health.grade} ·{' '}
                    {Math.round(summary().overall_health.score)}/100
                  </h2>
                  <p class="mt-1 text-sm text-muted">{summary().overall_health.prediction}</p>
                </div>

                <div class="flex flex-wrap items-center gap-2">
                  <span class="rounded-full border border-border-subtle bg-base px-2.5 py-1 text-xs font-medium text-base-content">
                    Critical {summary().findings_count.critical}
                  </span>
                  <span class="rounded-full border border-border-subtle bg-base px-2.5 py-1 text-xs font-medium text-base-content">
                    Warning {summary().findings_count.warning}
                  </span>
                </div>
              </div>

              <Show when={state.hasInvestigationContext()}>
                <div class="mt-4 rounded-md border border-border-subtle bg-base p-3">
                  <div class="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <p class="text-xs font-semibold uppercase tracking-[0.16em] text-muted">
                        Investigation context
                      </p>
                      <p class="mt-1 text-sm text-muted">
                        Secondary change and policy signals for deeper investigation.
                      </p>
                      <Show when={state.investigationContextSummary()}>
                        <p class="mt-1 text-xs text-base-content">
                          {state.investigationContextSummary()}
                        </p>
                      </Show>
                    </div>

                    <button
                      type="button"
                      onClick={() => state.setShowInvestigationContext((value) => !value)}
                      class="inline-flex items-center rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover"
                    >
                      {state.showInvestigationContext() ? 'Hide context' : 'Show context'}
                    </button>
                  </div>

                  <Show when={state.showInvestigationContext()}>
                    <div class="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
                      <Show when={state.recentChangeCount() > 0}>
                        <ResourceChangeSummary
                          class="space-y-0"
                          title="Recent changes"
                          subtitle="Last 24 hours"
                          changes={summary().recent_changes}
                          maxChanges={3}
                          compact
                        />
                      </Show>

                      <div class="space-y-4">
                        <Show when={state.correlations().length > 0}>
                          <ResourceCorrelationSummary
                            title="Correlations"
                            correlations={state.correlations()}
                            summaryText={`${state.correlationTotal()} total`}
                          />
                        </Show>

                        <ResourcePolicySummary
                          posture={state.policyPosture()}
                          title="Policy posture"
                        />
                      </div>
                    </div>
                  </Show>
                </div>
              </Show>
            </section>
          )}
        </Show>
      </Show>

      <Show
        when={
          !showRuntimeSummary() &&
          (state.summaryStats().criticalFindings > 0 ||
            state.summaryStats().warningFindings > 0 ||
            state.summaryStats().fixedCount > 0)
        }
        fallback={
          <Show when={!showRuntimeSummary() && state.patrolStatus()?.last_patrol_at}>
            <div class="flex items-center gap-2 px-4 py-3 bg-surface rounded-md border border-border">
              <CheckCircleIcon class="w-4 h-4 text-green-500 dark:text-green-400" />
              <span class="text-sm text-muted">{PATROL_NO_ISSUES_LABEL}</span>
            </div>
          </Show>
        }
      >
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <div class="bg-surface rounded-md border border-border p-3">
            <div class="flex items-center gap-2">
              <div
                class={`p-1.5 rounded-md border ${criticalSummaryPresentation().iconContainerClass}`}
              >
                <ShieldAlertIcon class={`w-4 h-4 ${criticalSummaryPresentation().iconClass}`} />
              </div>
              <div>
                <p class="text-xs text-muted">Critical</p>
                <p class={`text-lg font-bold ${criticalSummaryPresentation().valueClass}`}>
                  {state.summaryStats().criticalFindings}
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
                <p class="text-xs text-muted">Warnings</p>
                <p class={`text-lg font-bold ${warningSummaryPresentation().valueClass}`}>
                  {state.summaryStats().warningFindings}
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
                <p class="text-xs text-muted">Fixed</p>
                <p class={`text-lg font-bold ${fixedSummaryPresentation().valueClass}`}>
                  {state.summaryStats().fixedCount}
                </p>
              </div>
            </div>
          </div>
        </div>
      </Show>
    </>
  );
}
