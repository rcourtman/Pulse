import { createMemo, Show } from 'solid-js';
import ActivityIcon from 'lucide-solid/icons/activity';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import AlertCircleIcon from 'lucide-solid/icons/alert-circle';
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
  getPatrolSummaryMetricState,
  type PatrolRecommendedNextStepAction,
} from '@/utils/patrolSummaryPresentation';
import { getPatrolLatestRunPresentation } from '@/utils/patrolRunPresentation';
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
  const latestRun = createMemo(() =>
    getPatrolLatestRunPresentation(state.patrolRunHistory.value() ?? []),
  );
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
          <section class="border-y border-border-subtle py-2">
            <div class="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
              <div class="min-w-0 space-y-1">
                <div class="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-1 text-sm">
                  <span class="text-[11px] font-semibold uppercase tracking-[0.14em] text-muted">
                    {assessment().eyebrow}
                  </span>
                  <span class="font-semibold text-base-content">{compactAssessmentSummary()}</span>
                </div>

                <div
                  data-testid="patrol-recommended-next-step"
                  class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted"
                >
                  <span class="font-medium text-base-content">Next:</span>
                  <span class="font-medium text-base-content">{recommendedNextStep().title}</span>
                </div>
              </div>

              <div class="flex shrink-0 flex-wrap items-center gap-2 lg:justify-end">
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
          </section>
        </Show>
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
