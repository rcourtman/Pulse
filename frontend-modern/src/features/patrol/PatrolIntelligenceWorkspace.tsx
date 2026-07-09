import { createMemo, For, Show } from 'solid-js';
import HistoryIcon from 'lucide-solid/icons/history';
import SettingsIcon from 'lucide-solid/icons/settings';
import ShieldAlertIcon from 'lucide-solid/icons/shield-alert';
import ListChecksIcon from 'lucide-solid/icons/list-checks';
import { FindingsPanel } from '@/components/AI/FindingsPanel';
import { ApprovalBanner, RunHistoryPanel } from '@/components/patrol';
import {
  getFindingTitlePresentation,
  buildPatrolFindingDisplayGroups,
  getPatrolFindingsBadgePresentation,
  getPatrolWorkTypeComposition,
  isPatrolRuntimeFinding,
} from '@/utils/aiFindingPresentation';
import { formatRelativeTime } from '@/utils/format';
import { formatTriggerReason } from '@/utils/patrolFormat';
import {
  getPatrolRunRecordSummaryPresentation,
  getPatrolRunStatusPresentation,
  PATROL_FINDING_RECORD_UNAVAILABLE_LABEL,
} from '@/utils/patrolRunPresentation';
import { Button, ButtonLink } from '@/components/shared/Button';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { StatusIndicatorBadge } from '@/components/shared/StatusIndicatorBadge';
import { getPatrolProviderSettingsAction } from '@/utils/patrolRuntimeActions';
import {
  getPatrolProInvestigationHandoff,
  getPatrolQueueBadgeLabel,
  getPatrolQueueWorkspaceDescription,
  getPatrolWorkspaceWorkGroups,
  isPatrolCoverageStale,
  getPatrolSetupIssueReason,
  PATROL_WORKSPACE_HISTORY_DESCRIPTION,
  PATROL_WORKSPACE_QUEUE_TITLE,
  PATROL_WORKSPACE_RUN_RECORD_DESCRIPTION,
  PATROL_WORKSPACE_RUN_RECORD_TITLE,
  PATROL_WORKSPACE_SETUP_DESCRIPTION,
  PATROL_WORKSPACE_SETUP_TITLE,
} from './patrolControlPresentation';
import type { PatrolIntelligenceState } from './usePatrolIntelligenceState';
import { getUpgradeActionDestination } from '@/stores/licenseCommercial';
import {
  presentationPolicyHidesCommercialSurfaces,
  presentationPolicyHidesUpgradePrompts,
} from '@/stores/sessionPresentationPolicy';

export function PatrolIntelligenceWorkspace(props: { state: PatrolIntelligenceState }) {
  const state = props.state;
  const findingsBadgePresentation = () =>
    getPatrolFindingsBadgePresentation(state.findingsTabBadgeFindings());
  const queueDisplayGroups = createMemo(() =>
    buildPatrolFindingDisplayGroups(state.findingsTabBadgeFindings()),
  );
  const queueIssueCount = createMemo(
    () => state.findingsTabBadgeCount() ?? state.findingsTabBadgeFindings().length,
  );
  const queueAffectedResourceCount = createMemo(() => queueDisplayGroups().length);
  const workTypeComposition = createMemo(() =>
    getPatrolWorkTypeComposition(state.findingsTabBadgeFindings()),
  );
  const queueBadgeLabel = createMemo(() =>
    getPatrolQueueBadgeLabel({
      affectedResourceCount: queueAffectedResourceCount(),
      findingCount: queueIssueCount(),
    }),
  );
  const isHistoryOpen = () => state.activeTab() === 'history';
  const isSetupOnly = () =>
    !isHistoryOpen() && !state.selectedRun() && state.shouldShowPatrolSetupOnly();
  const setupAction = getPatrolProviderSettingsAction();
  const setupFinding = () => state.findingsTabBadgeFindings().find(isPatrolRuntimeFinding);
  const setupReason = () => {
    const finding = setupFinding();
    return getPatrolSetupIssueReason({
      setupFindingTitle: finding ? getFindingTitlePresentation(finding).label : undefined,
      readinessSummary: state.patrolReadiness()?.summary,
      triggerDisabledReason: state.triggerPatrolDisabledReason(),
      blockedReason: state.blockedReason(),
    });
  };
  const workspaceTitle = () =>
    isHistoryOpen()
      ? 'Patrol history'
      : isSetupOnly()
        ? PATROL_WORKSPACE_SETUP_TITLE
        : state.selectedRun()
          ? PATROL_WORKSPACE_RUN_RECORD_TITLE
          : PATROL_WORKSPACE_QUEUE_TITLE;
  const workspaceDescription = () =>
    isHistoryOpen()
      ? PATROL_WORKSPACE_HISTORY_DESCRIPTION
      : isSetupOnly()
        ? PATROL_WORKSPACE_SETUP_DESCRIPTION
        : state.selectedRun()
          ? PATROL_WORKSPACE_RUN_RECORD_DESCRIPTION
          : getPatrolQueueWorkspaceDescription({
              autonomyLevel: state.autonomyLevel(),
              autonomyLocked: state.autoFixLocked(),
              affectedResourceCount: queueAffectedResourceCount(),
              findingCount: queueIssueCount(),
              workTypeComposition: workTypeComposition(),
            });
  const openHistory = () => {
    state.setActiveTab('history');
    state.setFindingsFilterOverride(undefined);
  };
  const closeHistory = () => {
    state.setActiveTab('findings');
  };
  const selectHistoryRun = (run: ReturnType<typeof state.selectedRun>) => {
    state.setSelectedRun(run);
    if (run) {
      state.setActiveTab('findings');
    }
  };
  const selectedRunStatus = () => {
    const run = state.selectedRun();
    if (!run) return null;
    return getPatrolRunStatusPresentation(run.status ?? 'unknown', run.error_count ?? 0);
  };
  const selectedRunSummary = () => {
    const run = state.selectedRun();
    return run ? getPatrolRunRecordSummaryPresentation(run) : null;
  };
  const latestCompletedRun = createMemo(() => state.patrolRunHistory.value()?.[0] ?? null);
  const workGroupSummaries = createMemo(() =>
    getPatrolWorkspaceWorkGroups({
      latestRun: latestCompletedRun(),
      patrolStatus: state.patrolStatus(),
      pendingApprovalCount: state.patrolPendingApprovalCount(),
      workTypeComposition: workTypeComposition(),
    }),
  );
  const coverageStale = createMemo(() =>
    isPatrolCoverageStale({
      latestRun: latestCompletedRun(),
      patrolStatus: state.patrolStatus(),
    }),
  );
  const hasVisibleWorkGroups = () =>
    queueIssueCount() > 0 || workGroupSummaries().some((group) => group.id !== 'stale-protection');
  const shouldShowWorkGroups = () =>
    !isHistoryOpen() && !isSetupOnly() && !state.selectedRun() && hasVisibleWorkGroups();

  return (
    <>
      <ApprovalBanner
        onScrollToFinding={(findingId) => {
          state.setActiveTab('findings');
          state.setFindingsFilterOverride('approvals');
          state.clearScrollToFindingTimer();
          state.setScrollToFindingTimer(
            setTimeout(() => {
              state.setScrollToFindingTimer(undefined);
              const el = document.getElementById(`finding-${findingId}`);
              el?.scrollIntoView({ behavior: 'smooth', block: 'start' });
              state.setFindingScrollTimer(undefined);
            }, 100),
          );
        }}
      />

      <div class="flex flex-wrap items-start justify-between gap-3 border-b border-border pb-3">
        <div class="min-w-0">
          <div class="flex min-w-0 flex-wrap items-center gap-2">
            <h2 class="text-sm font-semibold text-base-content">{workspaceTitle()}</h2>
            <Show
              when={
                !isHistoryOpen() &&
                !isSetupOnly() &&
                !state.selectedRun() &&
                Boolean(queueBadgeLabel())
              }
            >
              <MetadataBadge aria-hidden="true" tone={findingsBadgePresentation().tone} size="xs">
                {queueBadgeLabel()}
              </MetadataBadge>
            </Show>
          </div>
          <p class="mt-1 text-xs text-muted">{workspaceDescription()}</p>
        </div>
        <Show when={!isSetupOnly() || (state.patrolRunHistory.value()?.length ?? 0) > 0}>
          <div class="flex flex-wrap items-center gap-2">
            <Button
              type="button"
              variant="secondary"
              size="sm"
              class="gap-1.5"
              onClick={() => (isHistoryOpen() ? closeHistory() : openHistory())}
            >
              <HistoryIcon class="h-4 w-4" />
              {isHistoryOpen() ? PATROL_WORKSPACE_QUEUE_TITLE : 'History'}
            </Button>
          </div>
        </Show>
      </div>

      <Show when={shouldShowWorkGroups()}>
        <div
          role="list"
          aria-label="Patrol work groups"
          class="grid gap-2 sm:grid-cols-2 xl:grid-cols-3"
        >
          <For each={workGroupSummaries()}>
            {(group) => (
              <div
                role="listitem"
                class="rounded-md border border-border-subtle bg-surface-alt/60 px-3 py-2"
              >
                <div class="flex min-w-0 flex-wrap items-center gap-2">
                  <MetadataBadge tone={group.tone} size="xs" shape="rounded">
                    {group.label}
                  </MetadataBadge>
                </div>
                <p class="mt-1 text-xs leading-5 text-muted">{group.detail}</p>
              </div>
            )}
          </For>
        </div>
      </Show>

      <Show when={state.activeTab() === 'findings'}>
        <Show when={state.selectedRun()}>
          {(run) => (
            <div class="flex flex-col gap-3 rounded-md border border-blue-200 bg-blue-50 px-3 py-3 text-blue-800 dark:border-blue-800 dark:bg-blue-950/40 dark:text-blue-200 sm:flex-row sm:items-start sm:justify-between">
              <div class="min-w-0 space-y-1">
                <div class="flex flex-wrap items-center gap-2 text-xs">
                  <span class="font-semibold text-blue-950 dark:text-blue-100">
                    Patrol run {formatRelativeTime(run().started_at, { compact: true })}
                  </span>
                  <span>{formatTriggerReason(run().trigger_reason)}</span>
                  <Show when={selectedRunStatus()}>
                    {(status) => (
                      <StatusIndicatorBadge
                        label={status().label}
                        variant={status().variant}
                        size="xs"
                        shape="rounded"
                      />
                    )}
                  </Show>
                  <Show when={state.selectedRunHasFindingsSnapshot() === false}>
                    <MetadataBadge tone="info" size="xs" shape="rounded">
                      {PATROL_FINDING_RECORD_UNAVAILABLE_LABEL}
                    </MetadataBadge>
                  </Show>
                </div>
                <Show when={selectedRunSummary()}>
                  {(summary) => (
                    <p class="text-xs leading-5 text-blue-800 dark:text-blue-200">
                      {summary().summary} {summary().outcome}
                    </p>
                  )}
                </Show>
                <Show when={selectedRunSummary()?.action}>
                  {(action) => (
                    <ButtonLink
                      href={action().href}
                      variant="secondary"
                      size="sm"
                      class="mt-1 gap-1.5"
                    >
                      <SettingsIcon class="h-4 w-4" aria-hidden="true" />
                      {action().label}
                    </ButtonLink>
                  )}
                </Show>
              </div>
              <button
                type="button"
                onClick={() => state.setSelectedRun(null)}
                class="w-fit shrink-0 text-xs font-medium text-blue-700 hover:underline dark:text-blue-300"
              >
                Show open work
              </button>
            </div>
          )}
        </Show>

        <Show
          when={isSetupOnly()}
          fallback={
            <FindingsPanel
              filterOverride={state.selectedRun() ? 'all' : state.findingsFilterOverride()}
              filterFindingIds={state.selectedRunFindingIds()}
              scopeResourceIds={state.selectedRunScopeResourceIds()}
              scopeResourceTypes={state.selectedRun()?.scope_resource_types}
              showScopeWarnings={Boolean(state.selectedRun())}
              runtimeState={state.runtimeState()}
              autonomyLevel={state.autonomyLevel()}
              blockedReason={state.blockedReason()}
              overallHealth={state.intelligenceSummary()?.overall_health}
              historicalRegressionCount={state.historicalRegressionCount()}
              coverageStale={coverageStale()}
              patrolRuns={state.patrolRunHistory.value() ?? []}
              findingsSource="patrol"
              runSnapshot={state.selectedRun() ?? undefined}
              showControls={!state.selectedRun()}
              onAssistantHandoff={(finding) => state.handleAssistantFindingHandoff(finding.id)}
              patrolProHandoff={(finding) =>
                getPatrolProInvestigationHandoff({
                  autoFixLocked: state.autoFixLocked(),
                  commercialSurfacesHidden: presentationPolicyHidesCommercialSurfaces(),
                  upgradePromptsHidden: presentationPolicyHidesUpgradePrompts(),
                  upgradeDestination: getUpgradeActionDestination('ai_autofix'),
                  severity: finding.severity,
                  status: finding.status,
                })
              }
            />
          }
        >
          <div class="overflow-hidden rounded-md border border-amber-300 bg-amber-50/70 dark:border-amber-800 dark:bg-amber-950/20">
            <div class="grid gap-4 p-4 lg:grid-cols-[minmax(0,1fr)_minmax(16rem,0.7fr)] lg:p-5">
              <div class="min-w-0">
                <div class="flex items-center gap-2 text-amber-800 dark:text-amber-200">
                  <ShieldAlertIcon class="h-5 w-5 shrink-0" aria-hidden="true" />
                  <h3 class="text-sm font-semibold">Patrol cannot run yet</h3>
                </div>
                <p class="mt-2 text-sm leading-6 text-base-content">{setupReason()}</p>
                <p class="mt-1 text-xs leading-5 text-muted">
                  Open Patrol settings and run the model check. Provider connectivity can be healthy
                  even when the selected model cannot use Patrol tools.
                </p>
                <ButtonLink
                  href={setupAction.href}
                  variant="primary"
                  size="sm"
                  class="mt-4 gap-1.5"
                >
                  <SettingsIcon class="h-4 w-4" aria-hidden="true" />
                  {setupAction.label}
                </ButtonLink>
              </div>
              <div class="rounded-md border border-amber-200 bg-surface/80 p-3 dark:border-amber-900">
                <div class="flex items-center gap-2">
                  <ListChecksIcon class="h-4 w-4 text-muted" aria-hidden="true" />
                  <p class="text-xs font-semibold uppercase tracking-wider text-muted">
                    Once ready
                  </p>
                </div>
                <ul class="mt-2 space-y-2 text-xs leading-5 text-muted">
                  <li>Patrol checks monitored infrastructure for current issues.</li>
                  <li>Findings, approvals, and verification stay together in Open work.</li>
                  <li>Past checks remain available from History.</li>
                </ul>
              </div>
            </div>
          </div>
        </Show>
      </Show>

      <Show when={state.activeTab() === 'history'}>
        <RunHistoryPanel
          runs={state.displayRunHistory()}
          loading={state.patrolRunHistory.loading()}
          selectedRun={state.selectedRun()}
          onSelectRun={selectHistoryRun}
          patrolStream={state.patrolStream}
        />
      </Show>
    </>
  );
}
