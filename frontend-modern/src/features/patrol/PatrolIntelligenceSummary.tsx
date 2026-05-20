import { createMemo, Show } from 'solid-js';
import AlertCircleIcon from 'lucide-solid/icons/alert-circle';
import {
  getPatrolAssessmentShellPresentation,
  getPatrolAssessmentPresentation,
  getPatrolRecencyPresentation,
  getPatrolSummaryMetricState,
} from '@/utils/patrolSummaryPresentation';
import { getPatrolRuntimePresentation } from '@/utils/patrolRuntimePresentation';
import { formatRelativeTime } from '@/utils/format';
import type { PatrolIntelligenceState } from './usePatrolIntelligenceState';

function PatrolAssessmentLoadingShell() {
  return (
    <section
      data-testid="patrol-summary-loading"
      class="overflow-hidden rounded-md border border-border bg-surface shadow-sm animate-pulse pointer-events-none select-none"
    >
      <div class="flex flex-wrap items-center justify-between gap-3 border-b border-border-subtle px-4 py-3">
        <div class="h-5 w-32 rounded bg-surface-hover" />
        <div class="h-5 w-24 rounded bg-surface-hover" />
      </div>

      <div class="px-4 py-4 sm:px-5 sm:py-5">
        <div class="flex items-start gap-3">
          <div class="h-11 w-11 rounded-md border border-border-subtle bg-surface-alt/60" />
          <div class="min-w-0 flex-1 space-y-2">
            <div class="h-5 w-44 rounded bg-surface-hover" />
            <div class="h-4 max-w-3xl rounded bg-surface-hover" />
            <div class="h-4 w-2/3 rounded bg-surface-hover" />
          </div>
        </div>

        <div class="mt-5 overflow-hidden rounded-md border border-border-subtle bg-surface-alt/60">
          <div class="grid divide-y divide-border-subtle lg:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)] lg:divide-x lg:divide-y-0">
            <div class="space-y-2 p-3">
              <div class="h-3 w-20 rounded bg-surface-hover" />
              <div class="h-4 w-40 rounded bg-surface-hover" />
              <div class="h-4 w-full rounded bg-surface-hover" />
              <div class="h-4 w-3/4 rounded bg-surface-hover" />
            </div>
            <div class="space-y-2 p-3">
              <div class="h-3 w-24 rounded bg-surface-hover" />
              <div class="h-4 w-36 rounded bg-surface-hover" />
              <div class="h-4 w-full rounded bg-surface-hover" />
              <div class="h-4 w-2/3 rounded bg-surface-hover" />
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

export function PatrolIntelligenceSummary(props: { state: PatrolIntelligenceState }) {
  const state = props.state;
  const summaryStats = createMemo(() => state.summaryStats());
  const metricState = createMemo(() =>
    getPatrolSummaryMetricState({
      activeFindings: state.activePatrolFindings(),
      fixedCount: summaryStats().fixedCount,
    }),
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
  const hasLoadedPatrolEvidence = createMemo(
    () =>
      summaryStats().hasAnyPatrolFindings ||
      (state.patrolRunHistory.value()?.length ?? 0) > 0 ||
      Boolean(state.patrolStatus()),
  );
  const showLoadingSummary = createMemo(
    () =>
      !showRuntimeSummary() &&
      !state.intelligenceSummary() &&
      !state.initialSurfaceReady() &&
      !hasLoadedPatrolEvidence(),
  );
  const assessment = createMemo(() =>
    getPatrolAssessmentPresentation({
      overallHealth: state.intelligenceSummary()?.overall_health,
      runtimeState: state.runtimeState(),
      blockedReason: state.blockedReason(),
      criticalFindings: summaryStats().criticalFindings,
      warningFindings: summaryStats().warningFindings,
      activeFindings: state.activePatrolFindings(),
      runs: state.patrolRunHistory.value() ?? [],
    }),
  );
  const recency = createMemo(() =>
    getPatrolRecencyPresentation({
      runs: state.patrolRunHistory.value() ?? [],
      lastPatrolAt: state.patrolStatus()?.last_patrol_at,
      lastActivityAt: state.patrolStatus()?.last_activity_at,
    }),
  );
  const compactRiskSummary = createMemo(() => {
    const stats = summaryStats();
    const parts: string[] = [];
    const hasRuntimeIssues =
      metricState().secondaryLabel === 'Runtime issues' && metricState().secondaryValue > 0;

    if (stats.criticalFindings > 0) {
      parts.push(`${stats.criticalFindings} critical`);
    }

    if (stats.warningFindings > 0 && !hasRuntimeIssues) {
      parts.push(`${stats.warningFindings} warning`);
    }

    if (hasRuntimeIssues) {
      parts.push(
        `${metricState().secondaryValue} runtime ${
          metricState().secondaryValue === 1 ? 'issue' : 'issues'
        }`,
      );
    }

    if (parts.length === 0 && stats.totalActive > 0) {
      parts.push(`${stats.totalActive} active`);
    }

    return parts.join(' · ');
  });
  const compactAssessmentSummary = createMemo(() => {
    const overallHealth = state.intelligenceSummary()?.overall_health;
    const parts: string[] = [compactRiskSummary() || assessment().compactLabel];
    const regressedCount = state.patrolStatus()?.trust?.regressed_at_least_once ?? 0;

    if (regressedCount > 0) {
      parts.push(`${regressedCount} regressed`);
    }

    if (overallHealth) {
      parts.push(`${Math.round(overallHealth.score)}/100`);
    }

    return parts.join(' · ');
  });
  return (
    <>
      <Show when={showLoadingSummary()}>
        <PatrolAssessmentLoadingShell />
      </Show>

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

      <Show when={!showRuntimeSummary() && !showLoadingSummary()}>
        <Show when={state.intelligenceSummary()}>
          <section class="border-y border-border-subtle py-2">
            <div class="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
              <div class="min-w-0 space-y-1">
                <div class="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-1 text-sm">
                  <span class="text-[11px] font-semibold uppercase tracking-[0.14em] text-muted">
                    {assessment().eyebrow}
                  </span>
                  <span class="font-semibold text-base-content">{compactAssessmentSummary()}</span>
                </div>
              </div>
            </div>
          </section>
        </Show>
      </Show>
    </>
  );
}
