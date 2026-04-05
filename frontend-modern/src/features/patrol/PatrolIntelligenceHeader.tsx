import { createMemo, For, Show } from 'solid-js';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import PlayIcon from 'lucide-solid/icons/play';
import CircleHelpIcon from 'lucide-solid/icons/circle-help';
import XIcon from 'lucide-solid/icons/x';
import SettingsIcon from 'lucide-solid/icons/settings';
import { PulsePatrolLogo } from '@/components/Brand/PulsePatrolLogo';
import { PageHeader } from '@/components/shared/PageHeader';
import { UpgradeLink } from '@/components/shared/UpgradeLink';
import { Toggle, TogglePrimitive } from '@/components/shared/Toggle';
import { CountdownTimer } from '@/components/patrol';
import { formatRelativeTime } from '@/utils/format';
import { groupModelsByProvider } from '@/utils/patrolFormat';
import {
  getAIQuickstartCreditsPresentation,
  isPatrolQuickstartExhaustedReason,
} from '@/utils/aiQuickstartPresentation';
import { buildPatrolScheduleOptions } from '@/utils/aiPatrolSchedulePresentation';
import { getPatrolRuntimePresentation } from '@/utils/patrolRuntimePresentation';
import { getPatrolRecencyPresentation } from '@/utils/patrolSummaryPresentation';
import type { PatrolIntelligenceState } from './usePatrolIntelligenceState';

export function PatrolIntelligenceHeader(props: { state: PatrolIntelligenceState }) {
  const state = props.state;
  const quickstartPresentation = createMemo(() =>
    getAIQuickstartCreditsPresentation(
      state.patrolStatus()?.quickstart_credits_remaining ?? 0,
      state.patrolStatus()?.quickstart_credits_total ?? 0,
    ),
  );
  const scheduleOptions = createMemo(() => buildPatrolScheduleOptions(state.patrolInterval()));
  const selectedScheduleLabel = createMemo(
    () =>
      scheduleOptions().find((option) => option.value === state.patrolInterval())?.label ??
      `${state.patrolInterval()} minutes`,
  );
  const runtimePresentation = createMemo(() =>
    getPatrolRuntimePresentation(state.runtimeState(), state.blockedReason()),
  );
  const recency = createMemo(() =>
    getPatrolRecencyPresentation({
      runs: state.patrolRunHistory() ?? [],
      lastPatrolAt: state.patrolStatus()?.last_patrol_at,
      lastActivityAt: state.patrolStatus()?.last_activity_at,
    }),
  );
  const showQuickstartStatus = createMemo(() => {
    const patrolStatus = state.patrolStatus();
    if (!patrolStatus) return false;
    if (patrolStatus.using_quickstart) return true;
    return state.runtimeState() === 'blocked' && isPatrolQuickstartExhaustedReason(state.blockedReason());
  });
  const patrolModelStale = createMemo(() => {
    const model = state.patrolModel();
    const models = state.availableModels();
    if (!model || models.length === 0) return false;
    return !models.some((candidate) => candidate.id === model);
  });

  return (
    <div class="flex-shrink-0 bg-surface border-b border-border px-4 py-3">
      <PageHeader
        id="patrol-title"
        title={
          <span
            class="inline-flex items-center gap-3"
            title="Pulse Patrol constantly monitors your infrastructure, investigates alerts, and can automatically fix issues based on your autonomy settings."
          >
            <PulsePatrolLogo class="w-6 h-6 text-base-content" />
            <span>Patrol</span>
          </span>
        }
        description="Pulse Patrol monitoring and analysis"
        class="mb-3"
        actions={
          <div class="flex flex-wrap items-center justify-end gap-3">
            <Show when={recency().timestamp}>
              <div class="hidden sm:flex items-center gap-3 text-xs text-muted">
                <span>
                  {recency().label}:{' '}
                  {formatRelativeTime(recency().timestamp, {
                    compact: true,
                    emptyText: 'Never',
                  })}
                </span>
                <Show when={state.patrolStatus()?.next_patrol_at}>
                  <span class="text-muted">|</span>
                  <CountdownTimer
                    targetDate={state.patrolStatus()!.next_patrol_at!}
                    prefix="Next run: "
                    class="font-variant-numeric tabular-nums font-medium text-blue-600 dark:text-blue-400"
                  />
                </Show>
              </div>
            </Show>

            <button
              onClick={() => state.handleRunPatrol()}
              disabled={
                state.isTriggeringPatrol() ||
                !state.canTriggerPatrol() ||
                state.manualRunRequested() ||
                state.patrolStream.isStreaming()
              }
              title={state.triggerPatrolDisabledReason()}
              class="flex items-center gap-2 px-3 py-1.5 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:bg-surface-alt disabled:text-muted rounded-md transition-colors"
            >
              <PlayIcon
                class={`w-4 h-4 ${state.isTriggeringPatrol() || state.manualRunRequested() || state.patrolStream.isStreaming() ? 'animate-pulse' : ''}`}
              />
              {state.isTriggeringPatrol()
                ? 'Starting…'
                : state.manualRunRequested() || state.patrolStream.isStreaming()
                  ? 'Running…'
                  : 'Run Patrol'}
            </button>

            <button
              onClick={() => state.loadAllData()}
              disabled={state.isRefreshing()}
              class="flex items-center gap-2 px-3 py-1.5 text-sm font-medium text-base-content bg-surface border border-border rounded-md hover:bg-surface-hover disabled:opacity-50 transition-colors"
            >
              <RefreshCwIcon class={`w-4 h-4 ${state.isRefreshing() ? 'animate-spin' : ''}`} />
              Refresh
            </button>
          </div>
        }
      />

      <div class="flex items-center gap-4 mt-2 mb-1">
        <div class="flex items-center gap-2 bg-surface-hover px-3 py-1.5 rounded-md border border-border">
          <TogglePrimitive
            checked={state.patrolEnabledLocal()}
            disabled={state.isTogglingPatrol()}
            onToggle={state.handleTogglePatrol}
            size="sm"
            ariaLabel="Toggle Patrol"
          />
          <span class="text-sm font-medium text-base-content">{runtimePresentation().label}</span>
        </div>

        <Show when={showQuickstartStatus()}>
          <div
            class={`flex items-center gap-1.5 px-3 py-1.5 rounded-md border text-xs font-medium ${quickstartPresentation().className}`}
            aria-label={quickstartPresentation().title}
            title={quickstartPresentation().title}
          >
            <Show
              when={(state.patrolStatus()?.quickstart_credits_remaining ?? 0) > 0}
              fallback={<span>{quickstartPresentation().summary}</span>}
            >
              <span>{quickstartPresentation().summary}</span>
            </Show>
          </div>
        </Show>

        <div class="flex-1"></div>

        <div class="relative" ref={state.setAdvancedSettingsRef}>
          <button
            onClick={() => state.setShowAdvancedSettings(!state.showAdvancedSettings())}
            disabled={!state.patrolEnabledLocal()}
            class={`flex items-center gap-2 px-3 py-1.5 text-sm font-medium rounded-md transition-all shadow-sm ${state.showAdvancedSettings() ? 'bg-blue-50 text-blue-700 border border-blue-200 dark:bg-blue-900 dark:text-blue-300 dark:border-blue-800' : ' text-base-content border border-border hover:bg-surface-alt'} ${!state.patrolEnabledLocal() ? 'opacity-50 cursor-not-allowed hidden' : ''}`}
          >
            <SettingsIcon class="w-4 h-4" />
            Configure Patrol
          </button>

          <Show when={state.showAdvancedSettings()}>
            <div class="absolute right-0 top-10 z-50 w-[340px] p-5 bg-surface rounded-md shadow-sm border border-border animate-slide-up transform origin-top-right">
              <div class="flex items-center justify-between mb-5 pb-3 border-b border-border-subtle">
                <h4 class="text-base font-semibold tracking-tight text-base-content">
                  Patrol Configuration
                </h4>
                <button
                  onClick={() => state.setShowAdvancedSettings(false)}
                  class="p-1 rounded-md hover:text-base-content hover:bg-surface-hover transition-colors"
                >
                  <XIcon class="w-4 h-4" />
                </button>
              </div>

              <div class="space-y-6">
                <div class="grid grid-cols-2 gap-4">
                  <div class="space-y-1.5">
                    <label class="text-xs font-semibold uppercase tracking-wider text-muted">
                      AI Model
                    </label>
                    <select
                      ref={state.setPatrolModelSelectRef}
                      value={state.patrolModel()}
                      onChange={(e) => state.handleModelChange(e.currentTarget.value)}
                      disabled={state.isUpdatingSettings() || !state.patrolEnabledLocal()}
                      class="w-full text-sm bg-base border border-border rounded-md py-2 pl-3 pr-8 text-base-content focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:opacity-50"
                    >
                      <option value="">
                        Default ({state.defaultModel().split(':').pop() || 'not set'})
                      </option>
                      <Show when={patrolModelStale()}>
                        <option value={state.patrolModel()} disabled>
                          {state.patrolModel().split(':').pop()} (unavailable)
                        </option>
                      </Show>
                      {Array.from(groupModelsByProvider(state.availableModels()).entries()).map(
                        ([provider, models]) => (
                          <optgroup label={provider.charAt(0).toUpperCase() + provider.slice(1)}>
                            {models.map((model) => (
                              <option value={model.id}>
                                {model.name || model.id.split(':').pop()}
                              </option>
                            ))}
                          </optgroup>
                        ),
                      )}
                    </select>
                  </div>

                  <div class="space-y-1.5">
                    <label class="text-xs font-semibold uppercase tracking-wider text-muted">
                      Run Every
                    </label>
                    <select
                      value={state.patrolInterval()}
                      onChange={(e) =>
                        state.handleIntervalChange(parseInt(e.currentTarget.value, 10))
                      }
                      disabled={state.isUpdatingSettings() || !state.patrolEnabledLocal()}
                      class="w-full text-sm bg-base border border-border rounded-md py-2 pl-3 pr-8 text-base-content focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:opacity-50"
                    >
                      <For each={scheduleOptions()}>
                        {(preset) => <option value={preset.value}>{preset.label}</option>}
                      </For>
                    </select>
                  </div>
                </div>

                <div class="space-y-2">
                  <div class="flex items-center justify-between">
                    <label class="text-xs font-semibold uppercase tracking-wider text-muted flex items-center gap-1.5">
                      Operational Mode
                      <div class="relative group">
                        <CircleHelpIcon class="w-3.5 h-3.5 cursor-help" />
                        <div class="absolute left-1/2 -translate-x-1/2 bottom-full mb-2 hidden group-hover:block w-64 p-3 bg-surface text-white rounded-md shadow-md text-xs z-50 pointer-events-none before:absolute before:top-full before:left-1/2 before:-translate-x-1/2 before:border-4 before:border-transparent before:border-t-slate-800">
                          <strong>Monitor:</strong> Detect only.
                          <br />
                          <strong>Investigate:</strong> Detect & propose fixes.
                          <br />
                          <strong>Auto-fix:</strong> Execute safe fixes automatically.
                        </div>
                      </div>
                    </label>
                  </div>

                  <div class="flex items-center bg-base rounded-md p-1 border shadow-inner">
                    <For each={['monitor', 'approval', 'assisted'] as const}>
                      {(level) => {
                        const isProLocked = () =>
                          state.autoFixLocked() && (level === 'approval' || level === 'assisted');
                        const isDisabled = () => !state.patrolEnabledLocal() || isProLocked();
                        const isActive = () =>
                          level === 'assisted'
                            ? state.autonomyLevel() === 'assisted' ||
                              state.autonomyLevel() === 'full'
                            : state.autonomyLevel() === level;

                        return (
                          <button
                            onClick={() => state.handleAutonomyChange(level)}
                            disabled={isDisabled()}
                            title={
                              isProLocked()
                                ? level === 'approval'
                                  ? 'Upgrade to Pro to investigate findings'
                                  : 'Upgrade to Pro for automatic fixes'
                                : undefined
                            }
                            class={`flex-1 py-1.5 px-2 text-xs font-semibold rounded-md transition-all duration-200 ${isActive() ? ' text-blue-600 dark:text-blue-400 shadow-[0_1px_3px_rgba(0,0,0,0.1)]' : isDisabled() ? ' ' : 'text-muted hover:text-base-content hover:bg-surface-hover'} ${isDisabled() ? 'opacity-50 cursor-not-allowed' : ''}`}
                          >
                            {level === 'monitor'
                              ? 'Monitor'
                              : level === 'approval'
                                ? 'Investigate'
                                : 'Auto-fix'}
                          </button>
                        );
                      }}
                    </For>
                  </div>
                  <Show when={state.autoFixLocked()}>
                    <div class="pl-1 text-[11px] text-slate-500">
                      <UpgradeLink
                        destination={state.upgradeDestination()}
                        class="text-indigo-500 font-medium hover:underline"
                      >
                        Upgrade to Pro
                      </UpgradeLink>{' '}
                      to unlock investigation and auto-fix.
                      <Show when={state.canStartTrial()}>
                        {' '}
                        <button
                          type="button"
                          onClick={state.handleStartTrial}
                          disabled={state.startingTrial()}
                          class="text-indigo-500 hover:underline"
                        >
                          Start free trial
                        </button>
                      </Show>
                    </div>
                  </Show>
                </div>

                <div class="space-y-4 pt-4 border-t border-border-subtle">
                  <div class="flex items-start justify-between gap-3">
                    <div class="flex-1">
                      <label class="text-sm font-medium text-base-content">
                        Alert-Triggered Analysis
                      </label>
                      <p class="text-[11px] text-muted mt-0.5 leading-tight">
                        Analyze infrastructure automatically when critical alerts fire.
                      </p>
                    </div>
                    <Toggle
                      checked={state.alertTriggeredAnalysis()}
                      onChange={(e) =>
                        state.handleAlertTriggeredAnalysisChange(e.currentTarget.checked)
                      }
                      disabled={state.isUpdatingSettings() || state.alertAnalysisLocked()}
                    />
                  </div>

                  <Show when={state.alertAnalysisLocked()}>
                    <div class="-my-1 pl-1 text-[11px]">
                      <UpgradeLink
                        destination={state.alertAnalysisUpgradeDestination()}
                        class="text-indigo-500 font-medium hover:underline"
                      >
                        Upgrade
                      </UpgradeLink>{' '}
                      to enable.
                      <Show when={state.canStartTrial()}>
                        <button
                          type="button"
                          onClick={state.handleStartTrial}
                          disabled={state.startingTrial()}
                          class="ml-1 text-indigo-500 hover:underline"
                        >
                          Start free trial
                        </button>
                      </Show>
                    </div>
                  </Show>

                  <div class="rounded-md border border-border-subtle bg-surface-alt/60 px-3 py-2.5">
                    <p class="text-[11px] font-medium text-base-content">
                      Full patrols run on the {selectedScheduleLabel().toLowerCase()} schedule.
                    </p>
                    <p class="mt-1 text-[11px] leading-tight text-muted">
                      Alert and anomaly triggers run targeted scoped checks that update{' '}
                      <span class="font-medium text-base-content">Last activity</span> without
                      resetting <span class="font-medium text-base-content">Last full patrol</span>.
                    </p>
                  </div>

                  <div class="flex items-start justify-between gap-3">
                    <div class="flex-1">
                      <label class="text-sm font-medium text-base-content">
                        Alert-Triggered Patrols
                      </label>
                      <p class="text-[11px] text-muted mt-0.5 leading-tight">
                        Run scoped Patrol checks when alerts fire or clear.
                      </p>
                    </div>
                    <Toggle
                      checked={state.patrolAlertTriggers()}
                      onChange={(e) =>
                        state.handlePatrolAlertTriggersChange(e.currentTarget.checked)
                      }
                      disabled={state.isUpdatingSettings() || !state.patrolEnabledLocal()}
                    />
                  </div>

                  <div class="flex items-start justify-between gap-3">
                    <div class="flex-1">
                      <label class="text-sm font-medium text-base-content">
                        Anomaly-Triggered Patrols
                      </label>
                      <p class="text-[11px] text-muted mt-0.5 leading-tight">
                        Run scoped Patrol checks when learned baselines detect high-signal
                        anomalies.
                      </p>
                    </div>
                    <Toggle
                      checked={state.patrolAnomalyTriggers()}
                      onChange={(e) =>
                        state.handlePatrolAnomalyTriggersChange(e.currentTarget.checked)
                      }
                      disabled={state.isUpdatingSettings() || !state.patrolEnabledLocal()}
                    />
                  </div>

                  <div class="flex items-start justify-between gap-3">
                    <div class="flex-1">
                      <label class="text-sm font-medium text-red-600 dark:text-red-400">
                        Auto-fix critical issues
                      </label>
                      <p class="text-[11px] text-muted mt-0.5 leading-tight">
                        Permit Patrol to execute critical remediations without approval.
                      </p>
                    </div>
                    <Toggle
                      checked={!state.autoFixLocked() && state.fullModeUnlocked()}
                      onChange={(e) => state.setFullModeUnlocked(e.currentTarget.checked)}
                      disabled={
                        state.autoFixLocked() ||
                        !(state.autonomyLevel() === 'assisted' || state.autonomyLevel() === 'full')
                      }
                    />
                  </div>
                </div>

                <div class="pt-4 border-t border-border-subtle">
                  <button
                    onClick={state.saveAdvancedSettings}
                    disabled={state.isSavingAdvanced()}
                    class="w-full py-2.5 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md shadow-sm transition-all focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-70 flex items-center justify-center gap-2"
                  >
                    <Show when={state.isSavingAdvanced()}>
                      <div class="animate-spin w-4 h-4 border-2 border-current border-t-transparent rounded-full"></div>
                    </Show>
                    <Show when={!state.isSavingAdvanced()}>Apply Configuration</Show>
                  </button>
                </div>
              </div>
            </div>
          </Show>
        </div>
      </div>
    </div>
  );
}
