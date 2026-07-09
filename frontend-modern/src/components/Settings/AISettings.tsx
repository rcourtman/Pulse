import { Component, createMemo, Show } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { AIChatMaintenanceSection } from '@/components/Settings/AIChatMaintenanceSection';
import { AISettingsDialogs } from '@/components/Settings/AISettingsDialogs';
import AgentIntegrationsPanel from '@/components/Settings/AgentIntegrationsPanel';
import {
  AIModelOverrideField,
  AIModelSelectionSection,
} from '@/components/Settings/AIModelSelectionSection';
import {
  AIAssistantCommandAccessSection,
  AIDiscoveryControlsSection,
  AIProviderRuntimeControlsSection,
} from '@/components/Settings/AIRuntimeControlsSection';
import { AISettingsStatusAndActions } from '@/components/Settings/AISettingsStatusAndActions';
import { useAISettingsState } from '@/components/Settings/useAISettingsState';
import { FormSelect } from '@/components/shared/FormSelect';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import { PATROL_PATH } from '@/routing/resourceLinks';
import { getPatrolAutonomyAvailabilityPresentation } from '@/features/patrol/patrolAutonomyAvailability';
import { getRuntimeCapabilityBlock, hasFeature, runtimeCapabilities } from '@/stores/license';
import { getUpgradeActionDestination } from '@/stores/licenseCommercial';
import {
  presentationPolicyHidesCommercialSurfaces,
  presentationPolicyHidesUpgradePrompts,
} from '@/stores/sessionPresentationPolicy';
import {
  AI_SETTINGS_PANEL_TITLE,
  getAISettingsLoadingState,
  getAISettingsLoadErrorMessage,
  getAISettingsRetryLabel,
} from '@/utils/aiSettingsPresentation';

type AISettingsPage = 'provider' | 'patrol' | 'assistant' | 'discovery';

const AI_SETTINGS_PAGE_META: Record<
  AISettingsPage,
  {
    resetLabel: string;
    saveErrorFallback: string;
    savedLabel: string;
    saveLabel: string;
    savingLabel: string;
    title: string;
  }
> = {
  provider: {
    resetLabel: 'Reset provider settings',
    saveErrorFallback: 'Unable to save Provider & Models settings.',
    savedLabel: 'Provider & Models settings saved',
    saveLabel: 'Save provider settings',
    savingLabel: 'Saving provider settings...',
    title: AI_SETTINGS_PANEL_TITLE,
  },
  patrol: {
    resetLabel: 'Reset Patrol settings',
    saveErrorFallback: 'Unable to save Patrol settings.',
    savedLabel: 'Patrol settings saved',
    saveLabel: 'Save Patrol settings',
    savingLabel: 'Saving Patrol settings...',
    title: 'Patrol',
  },
  assistant: {
    resetLabel: 'Reset Assistant settings',
    saveErrorFallback: 'Unable to save Assistant settings.',
    savedLabel: 'Assistant settings saved',
    saveLabel: 'Save Assistant settings',
    savingLabel: 'Saving Assistant settings...',
    title: 'Assistant',
  },
  discovery: {
    resetLabel: 'Reset service context settings',
    saveErrorFallback: 'Unable to save service context settings.',
    savedLabel: 'Service context settings saved',
    saveLabel: 'Save service context settings',
    savingLabel: 'Saving service context settings...',
    title: 'Service Context',
  },
};

const formatPatrolInterval = (minutes: number): string => {
  if (minutes <= 0) return 'Manual only';
  if (minutes < 60) return `Every ${minutes} minute${minutes === 1 ? '' : 's'}`;
  const hours = Math.round(minutes / 60);
  return `Every ${hours} hour${hours === 1 ? '' : 's'}`;
};

const ProviderSettingsContent: Component<{ state: ReturnType<typeof useAISettingsState> }> = (
  props,
) => (
  <div class="space-y-6 p-4 sm:p-6">
    <AIModelSelectionSection state={props.state} />
    <AIProviderRuntimeControlsSection state={props.state} />
  </div>
);

const PatrolSettingsContent: Component<{ state: ReturnType<typeof useAISettingsState> }> = (
  props,
) => {
  const navigate = useNavigate();
  const patrolModeAvailability = createMemo(() =>
    getPatrolAutonomyAvailabilityPresentation({
      autoFixLocked: !hasFeature('ai_autofix'),
      commercialSurfacesHidden: presentationPolicyHidesCommercialSurfaces(),
      upgradePromptsHidden: presentationPolicyHidesUpgradePrompts(),
      runtimeCapabilityBlock: getRuntimeCapabilityBlock('ai_autofix'),
      runtime: runtimeCapabilities()?.runtime,
      planUpgradeDestination: getUpgradeActionDestination('ai_autofix'),
    }),
  );
  const isPlanLockedPatrol = createMemo(() => patrolModeAvailability().kind === 'plan_locked');
  const intervalOptions = [
    { value: 0, label: 'Manual only' },
    { value: 60, label: 'Every hour' },
    { value: 180, label: 'Every 3 hours' },
    { value: 360, label: 'Every 6 hours' },
    { value: 720, label: 'Every 12 hours' },
    { value: 1440, label: 'Daily' },
  ];

  return (
    <div class="space-y-6 p-4 sm:p-6">
      <div class="rounded-md border border-border bg-surface-alt p-4">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h3 class="text-sm font-semibold text-base-content">Patrol mode</h3>
            <p class="mt-1 text-xs text-muted">
              <Show
                when={isPlanLockedPatrol()}
                fallback="Choose a Patrol mode on the Patrol page. Keep schedule, triggers, and model readiness here."
              >
                This install runs Watch only. Patrol monitors your infrastructure and reports
                issues.
              </Show>
            </p>
          </div>
          <button
            type="button"
            class="inline-flex min-h-10 items-center justify-center rounded-md border border-blue-300 px-3 py-2 text-sm font-medium text-blue-700 hover:bg-blue-50 dark:border-blue-700 dark:text-blue-300 dark:hover:bg-blue-900 sm:min-h-9"
            onClick={() => navigate(PATROL_PATH)}
          >
            Open Patrol
          </button>
        </div>
      </div>

      <div class="grid gap-4 lg:grid-cols-2">
        <FormSelect
          id="ai-patrol-schedule"
          label="Schedule"
          value={String(props.state.form.patrolIntervalMinutes)}
          onChange={(event) =>
            props.state.setForm('patrolIntervalMinutes', parseInt(event.currentTarget.value, 10))
          }
          disabled={props.state.saving()}
          fieldClass="gap-2"
          labelClass="text-xs font-medium text-muted"
          selectBaseClass="w-full min-h-10 rounded-md border border-border bg-surface px-3 py-2 text-sm"
        >
          {intervalOptions.map((option) => (
            <option value={option.value}>{option.label}</option>
          ))}
        </FormSelect>

        <div class="rounded-md border border-border bg-surface-alt p-3">
          <p class="text-xs font-medium text-base-content">Current schedule</p>
          <p class="mt-1 text-sm text-muted">
            {formatPatrolInterval(props.state.form.patrolIntervalMinutes)}
          </p>
        </div>
      </div>

      <div class="space-y-3">
        <h3 class="text-sm font-semibold text-base-content">Triggers</h3>

        <div class="grid gap-3 lg:grid-cols-2">
          <div class="rounded-md border border-border bg-surface-alt p-3">
            <div class="flex items-center justify-between gap-3">
              <div>
                <p class="text-sm font-medium text-base-content">Alert-triggered Patrols</p>
                <p class="mt-1 text-xs text-muted">Start Patrol when alerts fire or clear.</p>
              </div>
              <Toggle
                checked={props.state.form.patrolAlertTriggers}
                onChange={(event) =>
                  props.state.setForm('patrolAlertTriggers', event.currentTarget.checked)
                }
                disabled={props.state.saving()}
                ariaLabel="Enable alert-triggered Patrols"
              />
            </div>

            <Show when={props.state.form.patrolAlertTriggers}>
              <FormSelect
                id="ai-patrol-alert-min-severity"
                label="Investigate alerts at or above"
                value={props.state.form.patrolAlertTriggerMinSeverity}
                onChange={(event) =>
                  props.state.setForm(
                    'patrolAlertTriggerMinSeverity',
                    event.currentTarget.value === 'warning' ? 'warning' : 'critical',
                  )
                }
                disabled={props.state.saving()}
                fieldClass="mt-3 gap-2"
                labelClass="text-xs font-medium text-muted"
                selectBaseClass="w-full min-h-10 rounded-md border border-border bg-surface px-3 py-2 text-sm"
              >
                <option value="critical">Critical only</option>
                <option value="warning">Warning and critical</option>
              </FormSelect>
            </Show>
          </div>

          <div class="rounded-md border border-border bg-surface-alt p-3">
            <div class="flex items-center justify-between gap-3">
              <div>
                <p class="text-sm font-medium text-base-content">Anomaly-triggered Patrols</p>
                <p class="mt-1 text-xs text-muted">Start Patrol from learned baseline changes.</p>
              </div>
              <Toggle
                checked={props.state.form.patrolAnomalyTriggers}
                onChange={(event) =>
                  props.state.setForm('patrolAnomalyTriggers', event.currentTarget.checked)
                }
                disabled={props.state.saving()}
                ariaLabel="Enable anomaly-triggered Patrols"
              />
            </div>
          </div>

          <div class="rounded-md border border-border bg-surface-alt p-3 lg:col-span-2">
            <div class="flex items-center justify-between gap-3">
              <div>
                <p class="text-sm font-medium text-base-content">Container update risk</p>
                <p class="mt-1 text-xs text-muted">
                  Assess risk when container-update alerts fire.
                </p>
              </div>
              <Toggle
                checked={props.state.form.alertTriggeredAnalysis}
                onChange={(event) =>
                  props.state.setForm('alertTriggeredAnalysis', event.currentTarget.checked)
                }
                disabled={props.state.saving() || props.state.alertAnalysisLocked()}
                ariaLabel="Enable container update risk analysis"
              />
            </div>
          </div>
        </div>
      </div>

      <div class="space-y-3">
        <h3 class="text-sm font-semibold text-base-content">Model readiness</h3>
        <AIModelOverrideField state={props.state} kind="patrol" includePatrolPreflight />
      </div>
    </div>
  );
};

const AssistantSettingsContent: Component<{ state: ReturnType<typeof useAISettingsState> }> = (
  props,
) => (
  <div class="space-y-6 p-4 sm:p-6">
    <AIModelOverrideField state={props.state} kind="assistant" />
    <AIAssistantCommandAccessSection state={props.state} />
    <AIChatMaintenanceSection state={props.state} />
    <details class="overflow-hidden rounded-md border border-border bg-surface-alt/40">
      <summary class="cursor-pointer list-none px-4 py-3 text-sm font-semibold text-base-content hover:bg-surface-hover">
        Service identification
        <span class="ml-2 font-normal text-muted">
          Model-backed context used by Assistant and Patrol
        </span>
      </summary>
      <div class="space-y-4 border-t border-border bg-surface p-4">
        <p class="text-xs leading-5 text-muted">
          Identify service facts that help Assistant and Patrol explain monitored resources.
          Infrastructure discovery and onboarding remain under Infrastructure.
        </p>
        <AIModelOverrideField state={props.state} kind="discovery" />
        <AIDiscoveryControlsSection state={props.state} />
      </div>
    </details>
  </div>
);

const DiscoverySettingsContent: Component<{ state: ReturnType<typeof useAISettingsState> }> = (
  props,
) => (
  <div class="space-y-6 p-4 sm:p-6">
    <div class="rounded-md border border-border bg-surface-alt p-4">
      <h3 class="text-sm font-semibold text-base-content">Service context</h3>
      <p class="mt-1 text-xs text-muted">
        These settings control the service facts Assistant and Patrol can use. Normal infrastructure
        discovery and onboarding stay under Infrastructure.
      </p>
    </div>
    <AIModelOverrideField state={props.state} kind="discovery" />
    <AIDiscoveryControlsSection state={props.state} />
  </div>
);

const AISettingsPageContent: Component<{
  page: AISettingsPage;
  state: ReturnType<typeof useAISettingsState>;
}> = (props) => (
  <Show
    when={props.page === 'provider'}
    fallback={
      <Show
        when={props.page === 'patrol'}
        fallback={
          <Show
            when={props.page === 'assistant'}
            fallback={<DiscoverySettingsContent state={props.state} />}
          >
            <AssistantSettingsContent state={props.state} />
          </Show>
        }
      >
        <PatrolSettingsContent state={props.state} />
      </Show>
    }
  >
    <ProviderSettingsContent state={props.state} />
  </Show>
);

export const AISettings: Component<{ page?: AISettingsPage }> = (props) => {
  const page = () => props.page ?? 'provider';
  const pageMeta = () => AI_SETTINGS_PAGE_META[page()];
  const state = useAISettingsState({
    get saveErrorFallback() {
      return pageMeta().saveErrorFallback;
    },
    get savedLabel() {
      return pageMeta().savedLabel;
    },
  });
  const isProviderPage = () => page() === 'provider';
  const showExternalAgentAccess = () => page() === 'assistant';

  return (
    <>
      <div class="space-y-6">
        <SettingsPanel
          title={pageMeta().title}
          action={
            isProviderPage() ? (
              <Toggle
                checked={state.form.enabled}
                onChange={async (event) => {
                  const newValue = event.currentTarget.checked;
                  await state.handleEnableRequest(newValue);
                }}
                disabled={state.loading() || state.saving() || state.loadError()}
                containerClass="items-center gap-2"
                ariaLabel="Enable Pulse Intelligence"
                label={
                  <span class="text-xs font-medium text-muted">
                    {state.form.enabled ? 'Enabled' : 'Disabled'}
                  </span>
                }
              />
            ) : undefined
          }
          noPadding
        >
          <form class="divide-y divide-border" onSubmit={state.handleSave}>
            <Show when={state.loading()}>
              <div class="flex items-center gap-3 text-sm text-muted p-4 sm:p-6">
                <LoadingSpinner size="md" tone="current" />
                {getAISettingsLoadingState().text}
              </div>
            </Show>

            <Show when={!state.loading() && state.loadError()}>
              <div
                role="alert"
                aria-live="assertive"
                class="flex items-center justify-between gap-3 p-4 sm:p-6 bg-red-50 dark:bg-red-900/30 border-b border-red-200 dark:border-red-800"
              >
                <div class="flex items-center gap-2 text-sm text-red-700 dark:text-red-300">
                  <svg
                    class="h-4 w-4 flex-shrink-0"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z"
                    />
                  </svg>
                  <span>{getAISettingsLoadErrorMessage()}</span>
                </div>
                <button
                  type="button"
                  class="flex-shrink-0 px-3 py-1.5 text-sm font-medium text-red-700 dark:text-red-300 border border-red-300 dark:border-red-700 rounded-md hover:bg-red-100 dark:hover:bg-red-900/50"
                  onClick={() => state.loadSettings()}
                >
                  {getAISettingsRetryLabel()}
                </button>
              </div>
            </Show>

            <Show when={!state.loading() && !state.loadError()}>
              <AISettingsPageContent page={page()} state={state} />

              <AISettingsStatusAndActions
                resetLabel={pageMeta().resetLabel}
                saveLabel={pageMeta().saveLabel}
                savingLabel={pageMeta().savingLabel}
                state={state}
                showConnectionControls={isProviderPage()}
              />
            </Show>
          </form>
        </SettingsPanel>

        <Show when={showExternalAgentAccess()}>
          <AgentIntegrationsPanel />
        </Show>
      </div>

      <AISettingsDialogs
        showSetupModal={state.showSetupModal}
        setupProvider={state.setupProvider}
        setSetupProvider={state.setSetupProvider}
        setupApiKey={state.setupApiKey}
        setSetupApiKey={state.setSetupApiKey}
        setupOllamaUrl={state.setupOllamaUrl}
        setSetupOllamaUrl={state.setSetupOllamaUrl}
        setupSaving={state.setupSaving}
        handleCloseSetupModal={state.handleCloseSetupModal}
        handleSetupSubmit={state.handleSetupSubmit}
      />
    </>
  );
};

export const AIPatrolSettings: Component = () => <AISettings page="patrol" />;
export const AIAssistantSettings: Component = () => <AISettings page="assistant" />;
export const AIDiscoverySettings: Component = () => <AISettings page="discovery" />;

export default AISettings;
