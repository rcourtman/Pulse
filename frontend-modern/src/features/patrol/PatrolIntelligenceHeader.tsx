import { createMemo, For, Show } from 'solid-js';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import PlayIcon from 'lucide-solid/icons/play';
import CircleHelpIcon from 'lucide-solid/icons/circle-help';
import MessageSquareIcon from 'lucide-solid/icons/message-square';
import XIcon from 'lucide-solid/icons/x';
import SettingsIcon from 'lucide-solid/icons/settings';
import { PulsePatrolLogo } from '@/components/Brand/PulsePatrolLogo';
import { PageHeader } from '@/components/shared/PageHeader';
import { Toggle, TogglePrimitive } from '@/components/shared/Toggle';
import { CountdownTimer } from '@/components/patrol';
import { FormSelect } from '@/components/shared/FormSelect';
import { presentationPolicyHidesUpgradePrompts } from '@/stores/sessionPresentationPolicy';
import { formatRelativeTime } from '@/utils/format';
import { groupModelsByProvider } from '@/utils/patrolFormat';
import { getPatrolPageHeaderMeta } from '@/utils/patrolPagePresentation';
import { buildPatrolScheduleOptions } from '@/utils/aiPatrolSchedulePresentation';
import { getPatrolRuntimePresentation } from '@/utils/patrolRuntimePresentation';
import { getPatrolRecencyPresentation } from '@/utils/patrolSummaryPresentation';
import { PATROL_PROVIDER_SETTINGS_ACTION } from '@/utils/patrolRuntimeActions';
import type { PatrolConfigurationFailureInput } from './patrolInvestigationContextModel';
import type { PatrolIntelligenceState } from './usePatrolIntelligenceState';

const isNonEmptyConfigurationDetail = (value?: string | null): value is string =>
  Boolean(value?.trim());

export function getPatrolConfigurationFailureInlineDetails(
  failure: PatrolConfigurationFailureInput,
): string[] {
  const readiness = failure.readiness ?? null;
  const codeAndCause = [failure.code, readiness?.cause || failure.blockedCause]
    .filter(isNonEmptyConfigurationDetail)
    .join(' · ');

  return [
    codeAndCause || undefined,
    readiness?.summary ? `Readiness: ${readiness.summary}` : undefined,
    readiness?.provider ? `Provider: ${readiness.provider}` : undefined,
    readiness?.model ? `Model: ${readiness.model}` : undefined,
  ].filter(isNonEmptyConfigurationDetail);
}

export function PatrolIntelligenceHeader(props: { state: PatrolIntelligenceState }) {
  const state = props.state;
  const headerMeta = getPatrolPageHeaderMeta();
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
      runs: state.patrolRunHistory.value() ?? [],
      lastPatrolAt: state.patrolStatus()?.last_patrol_at,
      lastActivityAt: state.patrolStatus()?.last_activity_at,
    }),
  );
  const patrolModelStale = createMemo(() => {
    const model = state.patrolModel();
    const models = state.availableModels();
    if (!model || models.length === 0) return false;
    return !models.some((candidate) => candidate.id === model);
  });

  return (
    <div class="space-y-4">
      <PageHeader
        id="patrol-title"
        description={headerMeta.description}
        title={
          <span class="inline-flex items-center gap-3" title={headerMeta.titleTooltip}>
            <PulsePatrolLogo class="w-6 h-6 text-base-content" decorative />
            <span>{headerMeta.title}</span>
          </span>
        }
        class="relative z-[200] mb-3"
        actions={
          <div class="hidden sm:flex flex-wrap items-center justify-end gap-3">
            <Show when={recency().timestamp}>
              <div class="flex items-center gap-3 text-xs text-muted">
                <span>
                  {recency().label}:{' '}
                  {formatRelativeTime(recency().timestamp, {
                    compact: true,
                    emptyText: 'Never',
                  })}
                  <Show when={recency().resourcesCheckedLabel}>
                    {' '}
                    <span class="text-muted">— {recency().resourcesCheckedLabel}</span>
                  </Show>
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

      {/* Page-header trust summary — a compact one-liner of FindingsTrustSummary
          shown directly under the page title so the operator sees the trust
          loop's own state ("3 active, 1 regressed, 5 fixes verified")
          before they ever scroll into the workspace. The detailed Trust
          strip in PatrolIntelligenceWorkspace stays as the canonical
          breakdown for when the operator is reviewing findings. Visibility
          is gated on at least one non-zero signal so a fresh install
          doesn't render an empty header line. */}
      <Show
        when={(() => {
          const trust = state.patrolStatus()?.trust;
          if (!trust) return false;
          return (
            trust.currently_active > 0 ||
            trust.fix_verified > 0 ||
            trust.regressed_at_least_once > 0
          );
        })()}
      >
        {(() => {
          const trust = state.patrolStatus()!.trust!;
          return (
            <div
              class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted"
              aria-label="Patrol trust summary header"
            >
              <Show when={trust.currently_active > 0}>
                <span class="inline-flex items-center gap-1">
                  <span class="font-semibold text-base-content">{trust.currently_active}</span>
                  <span>active</span>
                </span>
              </Show>
              <Show when={trust.regressed_at_least_once > 0}>
                <span class="inline-flex items-center gap-1 text-amber-700 dark:text-amber-300">
                  <span class="font-semibold">{trust.regressed_at_least_once}</span>
                  <span>regressed</span>
                </span>
              </Show>
              <Show when={trust.fix_verified > 0}>
                <span class="inline-flex items-center gap-1 text-emerald-700 dark:text-emerald-300">
                  <span class="font-semibold">{trust.fix_verified}</span>
                  <span>fix{trust.fix_verified === 1 ? '' : 'es'} verified</span>
                </span>
              </Show>
            </div>
          );
        })()}
      </Show>

      <div class="flex flex-wrap items-center gap-3">
        <div class="flex items-center gap-2">
          <TogglePrimitive
            checked={state.patrolEnabledLocal()}
            disabled={state.isTogglingPatrol()}
            onToggle={state.handleTogglePatrol}
            size="sm"
            ariaLabel="Toggle Patrol"
          />
          <span class="text-sm font-medium text-base-content">{runtimePresentation().label}</span>
        </div>

        <div class="flex flex-wrap items-center gap-2 sm:ml-auto">
            <button
              onClick={() => state.handleRunPatrol()}
              disabled={
                state.isTriggeringPatrol() ||
                !state.canTriggerPatrol() ||
                state.manualRunRequested() ||
                state.patrolStream.isStreaming()
              }
              title={state.triggerPatrolDisabledReason()}
              class="sm:hidden flex items-center gap-2 px-3 py-1.5 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:bg-surface-alt disabled:text-muted rounded-md transition-colors"
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
              aria-label="Refresh"
              class="sm:hidden flex items-center gap-2 px-3 py-1.5 text-sm font-medium text-base-content bg-surface border border-border rounded-md hover:bg-surface-hover disabled:opacity-50 transition-colors"
            >
              <RefreshCwIcon class={`w-4 h-4 ${state.isRefreshing() ? 'animate-spin' : ''}`} />
              <span class="sr-only sm:not-sr-only">Refresh</span>
            </button>

            <div class="relative z-[120]" ref={state.setAdvancedSettingsRef}>
              <button
                onClick={() => state.setShowAdvancedSettings(!state.showAdvancedSettings())}
                disabled={!state.patrolEnabledLocal()}
                aria-label="Configure Patrol"
                title="Configure Patrol"
                class={`flex items-center gap-2 px-3 py-1.5 text-sm font-medium rounded-md transition-all shadow-sm ${state.showAdvancedSettings() ? 'bg-blue-50 text-blue-700 border border-blue-200 dark:bg-blue-900 dark:text-blue-300 dark:border-blue-800' : ' text-base-content border border-border hover:bg-surface-alt'} ${!state.patrolEnabledLocal() ? 'opacity-50 cursor-not-allowed hidden' : ''}`}
              >
                <SettingsIcon class="w-4 h-4" />
                <span class="sr-only sm:not-sr-only">Configure Patrol</span>
              </button>

              <Show when={state.showAdvancedSettings()}>
                <div
                  role="dialog"
                  aria-label="Patrol Configuration"
                  class="fixed right-4 top-32 z-[9999] isolate max-h-[calc(100vh-10rem)] w-[calc(100vw-2rem)] overflow-y-auto rounded-md border border-border bg-surface p-5 shadow-sm animate-slide-up transform origin-top-right sm:right-8 sm:top-[13rem] sm:max-h-[calc(100vh-14rem)] sm:w-[340px]"
                >
                  <div class="flex items-center justify-between mb-5 pb-3 border-b border-border-subtle">
                    <h4 class="text-base font-semibold tracking-tight text-base-content">
                      Patrol Configuration
                    </h4>
                    <button
                      onClick={() => state.setShowAdvancedSettings(false)}
                      aria-label="Close patrol configuration"
                      title="Close"
                      class="p-1 rounded-md hover:text-base-content hover:bg-surface-hover transition-colors"
                    >
                      <XIcon class="w-4 h-4" />
                    </button>
                  </div>

                  <div class="space-y-6">
                    <div class="grid grid-cols-2 gap-4">
                      <FormSelect
                        label="Provider model"
                        labelClass="text-xs font-semibold uppercase tracking-wider text-muted"
                        fieldClass="space-y-1.5"
                        ref={state.setPatrolModelSelectRef}
                        value={state.patrolModel()}
                        onChange={(e) => state.handleModelChange(e.currentTarget.value)}
                        disabled={state.isUpdatingSettings() || !state.patrolEnabledLocal()}
                        selectBaseClass="w-full text-sm bg-base border border-border rounded-md py-2 pl-3 pr-8 text-base-content focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:opacity-50"
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
                      </FormSelect>

                      <FormSelect
                        label="Run Every"
                        labelClass="text-xs font-semibold uppercase tracking-wider text-muted"
                        fieldClass="space-y-1.5"
                        value={state.patrolInterval()}
                        onChange={(e) =>
                          state.handleIntervalChange(parseInt(e.currentTarget.value, 10))
                        }
                        disabled={state.isUpdatingSettings() || !state.patrolEnabledLocal()}
                        selectBaseClass="w-full text-sm bg-base border border-border rounded-md py-2 pl-3 pr-8 text-base-content focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:opacity-50"
                      >
                        <For each={scheduleOptions()}>
                          {(preset) => <option value={preset.value}>{preset.label}</option>}
                        </For>
                      </FormSelect>
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
                              <strong>Remediate:</strong> Execute approved safe actions under
                              policy.
                            </div>
                          </div>
                        </label>
                      </div>

                      <div class="flex items-center bg-base rounded-md p-1 border shadow-inner">
                        <For each={['monitor', 'approval', 'assisted'] as const}>
                          {(level) => {
                            const isProLocked = () =>
                              state.autoFixLocked() &&
                              (level === 'approval' || level === 'assisted');
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
                                  !presentationPolicyHidesUpgradePrompts() && isProLocked()
                                    ? level === 'approval'
                                      ? 'Investigation is not enabled on this plan'
                                      : 'Safe remediation workflows are not enabled on this plan'
                                    : undefined
                                }
                                class={`flex-1 py-1.5 px-2 text-xs font-semibold rounded-md transition-all duration-200 ${isActive() ? ' text-blue-600 dark:text-blue-400 shadow-[0_1px_3px_rgba(0,0,0,0.1)]' : isDisabled() ? ' ' : 'text-muted hover:text-base-content hover:bg-surface-hover'} ${isDisabled() ? 'opacity-50 cursor-not-allowed' : ''}`}
                              >
                                {level === 'monitor'
                                  ? 'Monitor'
                                  : level === 'approval'
                                    ? 'Investigate'
                                    : 'Remediate'}
                              </button>
                            );
                          }}
                        </For>
                      </div>
                      <Show
                        when={!presentationPolicyHidesUpgradePrompts() && state.autoFixLocked()}
                      >
                        <div class="pl-1 text-[11px] text-muted">
                          Investigation and safe remediation workflows are not enabled on this plan.
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

                      <Show
                        when={
                          !presentationPolicyHidesUpgradePrompts() && state.alertAnalysisLocked()
                        }
                      >
                        <div class="-my-1 pl-1 text-[11px]">
                          Alert-triggered analysis is not enabled on this plan.
                        </div>
                      </Show>

                      <div class="rounded-md border border-border-subtle bg-surface-alt/60 px-3 py-2.5">
                        <p class="text-[11px] font-medium text-base-content">
                          {state.patrolInterval() === 0
                            ? 'Scheduled full patrols are off. Manual runs and alert/anomaly triggers still work.'
                            : `Full patrols run every ${selectedScheduleLabel().toLowerCase()}.`}
                        </p>
                        <p class="mt-1 text-[11px] leading-tight text-muted">
                          Alert and anomaly triggers run targeted scoped checks that update{' '}
                          <span class="font-medium text-base-content">Last activity</span> without
                          resetting{' '}
                          <span class="font-medium text-base-content">Last full patrol</span>.
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
                            Autonomous critical remediation
                          </label>
                          <p class="text-[11px] text-muted mt-0.5 leading-tight">
                            Permit Patrol to execute critical remediation actions without approval.
                          </p>
                        </div>
                        <Toggle
                          checked={!state.autoFixLocked() && state.fullModeUnlocked()}
                          onChange={(e) => state.setFullModeUnlocked(e.currentTarget.checked)}
                          disabled={
                            state.autoFixLocked() ||
                            !(
                              state.autonomyLevel() === 'assisted' ||
                              state.autonomyLevel() === 'full'
                            )
                          }
                        />
                      </div>
                    </div>

                    <div class="pt-4 border-t border-border-subtle">
                      <Show when={state.advancedSettingsError()}>
                        {(failure) => (
                          <div
                            role="alert"
                            data-testid="patrol-configuration-error"
                            class="mb-3 rounded-md border border-red-300 bg-red-50 px-3 py-2.5 text-red-950 dark:border-red-800 dark:bg-red-950/30 dark:text-red-100"
                          >
                            <p class="text-xs font-semibold">
                              {failure().saved
                                ? 'Patrol configuration needs attention'
                                : 'Patrol configuration was not saved'}
                            </p>
                            <p class="mt-1 text-xs leading-relaxed">{failure().message}</p>
                            <Show
                              when={
                                getPatrolConfigurationFailureInlineDetails(failure()).length > 0
                              }
                            >
                              <ul class="mt-1 space-y-0.5 text-[11px] leading-relaxed opacity-80">
                                <For each={getPatrolConfigurationFailureInlineDetails(failure())}>
                                  {(detail) => <li>{detail}</li>}
                                </For>
                              </ul>
                            </Show>
                            <div class="mt-2 flex flex-wrap items-center gap-2">
                              <button
                                type="button"
                                data-testid="patrol-configuration-error-assistant-button"
                                onClick={state.openAdvancedSettingsErrorInAssistant}
                                class="inline-flex items-center gap-1.5 rounded-md border border-red-300 bg-white/80 px-2 py-1 text-xs font-medium text-red-950 transition-colors hover:bg-white dark:border-red-700 dark:bg-red-950/40 dark:text-red-100 dark:hover:bg-red-900/50"
                              >
                                <MessageSquareIcon class="h-3.5 w-3.5" />
                                Discuss with Assistant
                              </button>
                              <a
                                href={PATROL_PROVIDER_SETTINGS_ACTION.href}
                                data-testid="patrol-configuration-error-settings-link"
                                class="inline-flex items-center gap-1.5 rounded-md border border-red-300 bg-white/80 px-2 py-1 text-xs font-medium text-red-950 transition-colors hover:bg-white dark:border-red-700 dark:bg-red-950/40 dark:text-red-100 dark:hover:bg-red-900/50"
                              >
                                <SettingsIcon class="h-3.5 w-3.5" />
                                {PATROL_PROVIDER_SETTINGS_ACTION.label}
                              </a>
                            </div>
                          </div>
                        )}
                      </Show>

                      <button
                        onClick={state.saveAdvancedSettings}
                        disabled={state.isSavingAdvanced()}
                        class="w-full py-2.5 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md shadow-sm transition-all focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:cursor-not-allowed disabled:bg-blue-300 disabled:hover:bg-blue-300 dark:disabled:bg-blue-900 flex items-center justify-center gap-2"
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
    </div>
  );
}
