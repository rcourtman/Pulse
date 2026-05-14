import { createMemo, For, Show } from 'solid-js';
import ActivityIcon from 'lucide-solid/icons/activity';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import AlertCircleIcon from 'lucide-solid/icons/alert-circle';
import ChevronDownIcon from 'lucide-solid/icons/chevron-down';
import ChevronUpIcon from 'lucide-solid/icons/chevron-up';
import MessageSquareIcon from 'lucide-solid/icons/message-square';
import PlayIcon from 'lucide-solid/icons/play';
import SettingsIcon from 'lucide-solid/icons/settings';
import {
  getPatrolAssessmentShellPresentation,
  getPatrolAssessmentPresentation,
  getPatrolRecommendedNextStepPresentation,
  getPatrolRecencyPresentation,
  getPatrolScoreChipLabel,
  getPatrolVerificationPresentation,
  getPatrolSummaryPresentation,
  getPatrolSummaryMetricState,
  type PatrolRecommendedNextStepAction,
} from '@/utils/patrolSummaryPresentation';
import {
  getPatrolLatestRunPresentation,
  getPatrolTriggerStatusSummary,
} from '@/utils/patrolRunPresentation';
import { getPatrolRuntimePresentation } from '@/utils/patrolRuntimePresentation';
import { formatRelativeTime } from '@/utils/format';
import { aiChatStore } from '@/stores/aiChat';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import type { ApprovalRequest } from '@/api/ai';
import {
  buildPatrolAssessmentAssistantHandoff,
  buildPatrolAssistantApprovalBriefingInput,
  buildPatrolAssistantProposedFixBriefingInput,
  type PatrolAssessmentAssistantFindingInput,
} from './patrolInvestigationContextModel';
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
  const showLoadingSummary = createMemo(
    () => !showRuntimeSummary() && !state.intelligenceSummary() && !state.initialSurfaceReady(),
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
  const activeFindingsSummaryPresentation = createMemo(() =>
    getPatrolSummaryPresentation(metricState().primarySeverity, metricState().primaryValue > 0),
  );
  const verification = createMemo(() =>
    getPatrolVerificationPresentation({
      runs: state.patrolRunHistory.value() ?? [],
      runtimeState: state.runtimeState(),
      blockedReason: state.blockedReason(),
    }),
  );
  const recency = createMemo(() =>
    getPatrolRecencyPresentation({
      runs: state.patrolRunHistory.value() ?? [],
      lastPatrolAt: state.patrolStatus()?.last_patrol_at,
      lastActivityAt: state.patrolStatus()?.last_activity_at,
    }),
  );
  const fixedSummaryPresentation = createMemo(() =>
    getPatrolSummaryPresentation('success', summaryStats().fixedCount > 0),
  );
  const latestRun = createMemo(() =>
    getPatrolLatestRunPresentation(state.patrolRunHistory.value() ?? []),
  );
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
  const recommendedNextStep = createMemo(() =>
    getPatrolRecommendedNextStepPresentation({
      assessment: assessment(),
      verification: verification(),
      activeFindings: state.activePatrolFindings(),
      pendingApprovalCount: aiIntelligenceStore.patrolPendingApprovals.length,
    }),
  );
  const recommendedNextStepAction = createMemo(() => recommendedNextStep().action);
  const recommendedNextStepActionDisabled = createMemo(() => {
    const action = recommendedNextStepAction();
    return (
      action?.kind === 'run_patrol' &&
      (state.isTriggeringPatrol() ||
        !state.canTriggerPatrol() ||
        state.manualRunRequested() ||
        state.patrolStream.isStreaming())
    );
  });
  const recommendedNextStepActionDisabledReason = createMemo(() =>
    recommendedNextStepActionDisabled() ? state.triggerPatrolDisabledReason() : '',
  );
  const recommendedNextStepActionLabel = createMemo(() => {
    const action = recommendedNextStepAction();
    if (action?.kind !== 'run_patrol') {
      return action?.label;
    }

    if (state.isTriggeringPatrol()) {
      return 'Starting...';
    }

    if (state.manualRunRequested() || state.patrolStream.isStreaming()) {
      return 'Running...';
    }

    return action.label;
  });
  const showAssessmentAssistantButton = createMemo(
    () => recommendedNextStepAction()?.kind !== 'discuss_assessment',
  );
  const hasAttentionMetrics = createMemo(
    () => metricState().primaryValue > 0 || metricState().secondaryValue > 0,
  );
  const visibleMetrics = createMemo(() => {
    const metrics: Array<{
      key: string;
      label: string;
      presentation: ReturnType<typeof getPatrolSummaryPresentation>;
      value: number;
    }> = [];

    if (metricState().primaryValue > 0) {
      metrics.push({
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
        key: 'secondary',
        label: metricState().secondaryLabel,
        presentation: warningSummaryPresentation(),
        value: metricState().secondaryValue,
      });
    }

    if (metricState().fixedValue > 0 && hasAttentionMetrics()) {
      metrics.push({
        key: 'fixed',
        label: metricState().fixedLabel,
        presentation: fixedSummaryPresentation(),
        value: metricState().fixedValue,
      });
    }

    return metrics;
  });
  const activeFindingsWithApprovalContext = createMemo<PatrolAssessmentAssistantFindingInput[]>(
    () => {
      const approvalsByFindingId = new Map(
        aiIntelligenceStore.patrolPendingApprovals.map((approval) => [approval.targetId, approval]),
      );
      return state.activePatrolFindings().map((finding) => {
        const approval = approvalsByFindingId.get(finding.id);
        if (!approval) {
          return finding;
        }

        return {
          ...finding,
          pendingApproval: buildPatrolAssessmentApprovalBriefing(approval),
          proposedFix: finding.investigationRecord?.proposed_fix
            ? undefined
            : buildPatrolAssessmentApprovalProposedFixBriefing(approval),
        };
      });
    },
  );
  const assessmentAssistantHandoff = createMemo(() => {
    const recommendation = recommendedNextStep();
    return buildPatrolAssessmentAssistantHandoff({
      assessment: assessment(),
      overallHealth: state.intelligenceSummary()?.overall_health,
      scoreChipLabel: scoreChipLabel(),
      metricState: metricState(),
      verification: verification(),
      recency: recency(),
      latestRun: latestRun(),
      investigationContext: {
        recentChangeCount: state.recentChangeCount(),
        correlationCount: state.correlationTotal(),
        governedResourceCount: state.policyPosture()?.total_resources ?? 0,
        hasContext: state.hasInvestigationContext(),
        summaryText: state.investigationContextSummary(),
      },
      supportingEvidence: {
        recentChanges: state.supportingRecentChanges(),
        correlations: state.correlations(),
      },
      recommendedNextStep: {
        title: recommendation.title,
        description: recommendation.description,
        actionLabel: recommendation.action?.label,
        actionKind: recommendation.action?.kind,
        actionDisabledReason: recommendedNextStepActionDisabledReason(),
      },
      activeFindings: activeFindingsWithApprovalContext(),
    });
  });

  const handleDiscussAssessment = async () => {
    await aiIntelligenceStore.loadPendingApprovals();
    const handoff = assessmentAssistantHandoff();
    aiChatStore.openWithPrompt(handoff.prompt, handoff.context);
  };

  const handleRecommendedNextStepAction = (action: PatrolRecommendedNextStepAction) => {
    switch (action.kind) {
      case 'discuss_assessment':
        void handleDiscussAssessment();
        return;
      case 'review_approvals':
        state.setSelectedRun(null);
        state.setActiveTab('findings');
        state.setFindingsFilterOverride('approvals');
        return;
      case 'review_findings':
        state.setSelectedRun(null);
        state.setActiveTab('findings');
        state.setFindingsFilterOverride('active');
        return;
      case 'run_patrol':
        void state.handleRunPatrol();
        return;
      case 'open_provider_settings':
        return;
    }
  };

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
          {(summary) => (
            <section class="border-y border-border-subtle py-3">
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-2">
                    <span
                      class={`h-2 w-2 rounded-full ${getPatrolAssessmentDotClass(assessment().tone)}`}
                      aria-hidden="true"
                    />
                    <p class="text-[11px] font-semibold uppercase tracking-[0.14em] text-muted">
                      {assessment().eyebrow}
                    </p>
                  </div>

                  <h2 class="mt-2 text-base font-semibold text-base-content">
                    {assessment().title}
                  </h2>
                  <p class="mt-1 max-w-4xl text-sm leading-6 text-muted">
                    {assessment().description}
                  </p>

                  <div
                    data-testid="patrol-recommended-next-step"
                    class={`mt-3 flex flex-col gap-2 border-l-2 pl-3 sm:flex-row sm:items-center ${getPatrolRecommendedNextStepAccentClass(recommendedNextStep().tone)}`}
                  >
                    <p class="min-w-0 flex-1 text-sm leading-6 text-muted">
                      <span class="font-semibold text-base-content">Next:</span>{' '}
                      <span class="font-semibold text-base-content">
                        {recommendedNextStep().title}
                      </span>
                      <span> - {recommendedNextStep().description}</span>
                    </p>
                    <Show when={recommendedNextStepAction()}>
                      {(action) => (
                        <Show
                          when={action().href}
                          fallback={
                            <button
                              type="button"
                              data-testid="patrol-recommended-next-step-action"
                              disabled={recommendedNextStepActionDisabled()}
                              title={
                                action().kind === 'run_patrol'
                                  ? state.triggerPatrolDisabledReason()
                                  : undefined
                              }
                              class="inline-flex shrink-0 items-center gap-1.5 rounded border border-border-subtle bg-transparent px-2.5 py-1.5 text-xs font-semibold text-base-content transition-colors hover:bg-surface-hover disabled:text-muted"
                              onClick={() => handleRecommendedNextStepAction(action())}
                            >
                              {renderRecommendedNextStepActionIcon(
                                action(),
                                state.isTriggeringPatrol() ||
                                  state.manualRunRequested() ||
                                  state.patrolStream.isStreaming(),
                              )}
                              <span>{recommendedNextStepActionLabel()}</span>
                            </button>
                          }
                        >
                          {(href) => (
                            <a
                              href={href()}
                              data-testid="patrol-recommended-next-step-action"
                              class="inline-flex shrink-0 items-center gap-1.5 rounded border border-border-subtle bg-transparent px-2.5 py-1.5 text-xs font-semibold text-base-content transition-colors hover:bg-surface-hover"
                            >
                              {renderRecommendedNextStepActionIcon(action(), false)}
                              <span>{action().label}</span>
                            </a>
                          )}
                        </Show>
                      )}
                    </Show>
                  </div>
                </div>

                <div class="flex shrink-0 flex-wrap items-center gap-3 sm:justify-end">
                  <p class="text-sm font-semibold text-base-content">
                    {scoreChipLabel()} {summary().overall_health.grade} ·{' '}
                    {Math.round(summary().overall_health.score)}/100
                  </p>
                  <button
                    type="button"
                    data-testid="patrol-summary-details-toggle"
                    aria-expanded={state.summaryDetailsExpanded()}
                    aria-controls="patrol-summary-details"
                    onClick={() => state.setSummaryDetailsExpanded((value) => !value)}
                    class="inline-flex items-center gap-1 text-xs font-medium text-muted transition-colors hover:text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/40"
                  >
                    <Show
                      when={state.summaryDetailsExpanded()}
                      fallback={<ChevronDownIcon class="h-3.5 w-3.5" aria-hidden="true" />}
                    >
                      <ChevronUpIcon class="h-3.5 w-3.5" aria-hidden="true" />
                    </Show>
                    <span>{state.summaryDetailsExpanded() ? 'Hide details' : 'Show details'}</span>
                  </button>
                </div>
              </div>

              <Show when={state.summaryDetailsExpanded()}>
                <div id="patrol-summary-details" class="mt-4 border-t border-border-subtle pt-4">
                  <div class="grid gap-4 lg:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)]">
                    <div>
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

                    <div>
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
                              <span class="font-medium text-base-content">
                                {latest().kindLabel}
                              </span>
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

                  <Show when={showAssessmentAssistantButton()}>
                    <div class="mt-4">
                      <button
                        type="button"
                        data-testid="patrol-assessment-assistant-button"
                        class="inline-flex items-center gap-1.5 rounded border border-border-subtle bg-transparent px-2.5 py-1.5 text-xs font-semibold text-base-content transition-colors hover:bg-surface-hover focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/40"
                        title="Discuss current Patrol assessment"
                        onClick={handleDiscussAssessment}
                      >
                        <MessageSquareIcon class="h-4 w-4" aria-hidden="true" />
                        <span>Discuss with Assistant</span>
                      </button>
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
          state.intelligenceSummary() &&
          visibleMetrics().length > 0 &&
          state.summaryDetailsExpanded()
        }
      >
        <div class="flex flex-wrap items-center gap-x-5 gap-y-2 text-sm">
          <For each={visibleMetrics()}>
            {(metric) => {
              return (
                <p>
                  <span class="text-muted">{metric.label}</span>{' '}
                  <span class={`font-semibold ${metric.presentation.valueClass}`}>
                    {metric.value}
                  </span>
                </p>
              );
            }}
          </For>
        </div>
      </Show>
    </>
  );
}

function buildPatrolAssessmentApprovalBriefing(approval: ApprovalRequest) {
  return buildPatrolAssistantApprovalBriefingInput(approval);
}

function buildPatrolAssessmentApprovalProposedFixBriefing(approval: ApprovalRequest) {
  return buildPatrolAssistantProposedFixBriefingInput({
    description: approval.context,
    riskLevel: approval.riskLevel,
    targetHost: approval.targetName,
    commandCount: approval.command ? 1 : 0,
  });
}

function getPatrolRecommendedNextStepAccentClass(
  tone: ReturnType<typeof getPatrolRecommendedNextStepPresentation>['tone'],
) {
  switch (tone) {
    case 'success':
      return 'border-emerald-400 dark:border-emerald-500';
    case 'warning':
      return 'border-amber-400 dark:border-amber-500';
    case 'error':
      return 'border-red-400 dark:border-red-500';
    default:
      return 'border-blue-400 dark:border-blue-500';
  }
}

function getPatrolAssessmentDotClass(
  tone: ReturnType<typeof getPatrolAssessmentPresentation>['tone'],
) {
  switch (tone) {
    case 'success':
      return 'bg-emerald-500';
    case 'warning':
      return 'bg-amber-500';
    case 'error':
      return 'bg-red-500';
    default:
      return 'bg-blue-500';
  }
}

function renderRecommendedNextStepActionIcon(
  action: PatrolRecommendedNextStepAction,
  running: boolean,
) {
  const iconClass = `h-4 w-4 ${running && action.kind === 'run_patrol' ? 'animate-pulse' : ''}`;

  switch (action.kind) {
    case 'open_provider_settings':
      return <SettingsIcon class={iconClass} aria-hidden="true" />;
    case 'run_patrol':
      return <PlayIcon class={iconClass} aria-hidden="true" />;
    case 'discuss_assessment':
      return <MessageSquareIcon class={iconClass} aria-hidden="true" />;
    case 'review_approvals':
      return <CheckCircleIcon class={iconClass} aria-hidden="true" />;
    case 'review_findings':
      return <ActivityIcon class={iconClass} aria-hidden="true" />;
  }
}
