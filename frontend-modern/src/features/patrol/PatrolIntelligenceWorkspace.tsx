import { Show } from 'solid-js';
import { FindingsPanel } from '@/components/AI/FindingsPanel';
import { ApprovalBanner, PatrolStatusBar, RunHistoryPanel } from '@/components/patrol';
import { getFindingSeverityToneClasses } from '@/utils/aiFindingPresentation';
import { formatRelativeTime } from '@/utils/format';
import { formatTriggerReason } from '@/utils/patrolFormat';
import type { PatrolIntelligenceState } from './usePatrolIntelligenceState';

export function PatrolIntelligenceWorkspace(props: { state: PatrolIntelligenceState }) {
  const state = props.state;

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
          <Show when={state.summaryStats().totalActive > 0}>
            <span
              class={`ml-1.5 px-1.5 py-0.5 text-xs rounded-full ${getFindingSeverityToneClasses(state.summaryStats().criticalFindings > 0 ? 'critical' : 'warning')}`}
            >
              {state.summaryStats().totalActive}
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
            <span class="ml-1.5 px-1.5 py-0.5 text-xs rounded-full bg-surface-alt text-muted">
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
    </>
  );
}
