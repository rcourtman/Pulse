import { Show } from 'solid-js';
import { FindingsPanel } from '@/components/AI/FindingsPanel';
import { ApprovalBanner, PatrolStatusBar, RunHistoryPanel } from '@/components/patrol';
import { getPatrolFindingsBadgePresentation } from '@/utils/aiFindingPresentation';
import { formatRelativeTime } from '@/utils/format';
import { formatTriggerReason } from '@/utils/patrolFormat';
import { ResourcePolicySummary } from '@/components/Infrastructure/ResourcePolicySummary';
import { ResourceCorrelationSummary } from '@/components/Infrastructure/ResourceCorrelationSummary';
import { ResourceChangeSummary } from '@/components/Infrastructure/ResourceChangeSummary';
import type { PatrolIntelligenceState } from './usePatrolIntelligenceState';

export function PatrolIntelligenceWorkspace(props: { state: PatrolIntelligenceState }) {
  const state = props.state;
  const findingsBadgePresentation = () =>
    getPatrolFindingsBadgePresentation(state.findingsTabBadgeFindings());

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

      <PatrolStatusBar
        enabled={state.patrolEnabledLocal()}
        refreshTrigger={state.activityRefreshTrigger()}
        runtimeState={state.runtimeState()}
        blockedReason={state.blockedReason()}
      />

      <div class="flex items-center gap-1 border-b border-border">
        <button
          type="button"
          onClick={() => state.setActiveTab('findings')}
          class={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            state.activeTab() === 'findings'
              ? 'border-blue-500 text-base-content'
              : 'border-transparent text-muted hover:text-base-content hover:border-border'
          }`}
        >
          Findings
          <Show when={(state.findingsTabBadgeCount() ?? 0) > 0}>
            <span
              aria-hidden="true"
              class={`ml-1.5 px-1.5 py-0.5 text-xs rounded-full ${findingsBadgePresentation().toneClasses}`}
            >
              {' '}
              {state.findingsTabBadgeCount()}
            </span>
          </Show>
        </button>
        <button
          type="button"
          onClick={() => {
            state.setActiveTab('history');
            state.setFindingsFilterOverride(undefined);
          }}
          class={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            state.activeTab() === 'history'
              ? 'border-blue-500 text-base-content'
              : 'border-transparent text-muted hover:text-base-content hover:border-border'
          }`}
        >
          Runs
          <Show when={state.displayRunHistory().length > 0}>
            <span
              aria-hidden="true"
              class="ml-1.5 px-1.5 py-0.5 text-xs rounded-full bg-surface-alt text-muted"
            >
              {' '}
              {state.displayRunHistory().length}
            </span>
          </Show>
        </button>
      </div>

      <Show when={state.activeTab() === 'findings'}>
        <Show when={state.selectedRun()}>
          {(run) => (
            <div class="flex items-center justify-between px-3 py-2 rounded-md bg-blue-50 dark:bg-blue-900 border border-blue-200 dark:border-blue-800 text-xs text-blue-700 dark:text-blue-300">
              <span>
                Filtered to run {formatRelativeTime(run().started_at, { compact: true })} (
                {formatTriggerReason(run().trigger_reason)})
                <Show when={state.selectedRunHasFindingsSnapshot() === false}>
                  {' · '}Findings snapshot unavailable
                </Show>
              </span>
              <button
                type="button"
                onClick={() => state.setSelectedRun(null)}
                class="font-medium hover:underline"
              >
                Clear filter
              </button>
            </div>
          )}
        </Show>

        <FindingsPanel
          filterOverride={state.selectedRun() ? 'all' : state.findingsFilterOverride()}
          filterFindingIds={state.selectedRunFindingIds()}
          scopeResourceIds={state.selectedRunScopeResourceIds()}
          scopeResourceTypes={state.selectedRun()?.scope_resource_types}
          showScopeWarnings={Boolean(state.selectedRun())}
          runtimeState={state.runtimeState()}
          blockedReason={state.blockedReason()}
          overallHealth={state.intelligenceSummary()?.overall_health}
          runSnapshot={state.selectedRun() ?? undefined}
        />
      </Show>

      <Show when={state.activeTab() === 'history'}>
        <RunHistoryPanel
          runs={state.displayRunHistory()}
          loading={state.patrolRunHistory.loading}
          selectedRun={state.selectedRun()}
          onSelectRun={state.setSelectedRun}
          patrolStream={state.patrolStream}
        />
      </Show>

      <Show when={state.hasInvestigationContext()}>
        <section class="rounded-md border border-border-subtle bg-base/90 p-3">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <p class="text-xs font-semibold uppercase tracking-[0.16em] text-muted">
                Investigation context
              </p>
              <p class="mt-1 text-sm text-muted">
                Secondary change and policy signals for deeper investigation.
              </p>
              <Show when={state.investigationContextSummary()}>
                <p class="mt-1 text-xs text-base-content">{state.investigationContextSummary()}</p>
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
                  changes={state.intelligenceSummary()?.recent_changes}
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

                <ResourcePolicySummary posture={state.policyPosture()} title="Policy posture" />
              </div>
            </div>
          </Show>
        </section>
      </Show>
    </>
  );
}
