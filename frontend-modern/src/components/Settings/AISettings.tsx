import { Component, Show } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { AIChatMaintenanceSection } from '@/components/Settings/AIChatMaintenanceSection';
import { AISettingsDialogs } from '@/components/Settings/AISettingsDialogs';
import { AIModelSelectionSection } from '@/components/Settings/AIModelSelectionSection';
import { AIRuntimeControlsSection } from '@/components/Settings/AIRuntimeControlsSection';
import { AISettingsStatusAndActions } from '@/components/Settings/AISettingsStatusAndActions';
import { useAISettingsState } from '@/components/Settings/useAISettingsState';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import { PATROL_PATH } from '@/routing/resourceLinks';
import {
  AI_SETTINGS_PANEL_DESCRIPTION,
  AI_SETTINGS_PANEL_TITLE,
  getAISettingsLoadingState,
  getAISettingsLoadErrorMessage,
  getAISettingsRetryLabel,
} from '@/utils/aiSettingsPresentation';

export const AISettings: Component = () => {
  const navigate = useNavigate();
  const state = useAISettingsState();

  return (
    <>
      <SettingsPanel
        title={AI_SETTINGS_PANEL_TITLE}
        description={AI_SETTINGS_PANEL_DESCRIPTION}
        action={(() => {
          return (
            <Toggle
              checked={state.form.enabled}
              onChange={async (event) => {
                const newValue = event.currentTarget.checked;
                await state.handleEnableRequest(newValue);
              }}
              disabled={state.loading() || state.saving() || state.loadError()}
              containerClass="items-center gap-2"
              ariaLabel="Enable Assistant and Patrol"
              label={
                <span class="text-xs font-medium text-muted">
                  {state.form.enabled ? 'Enabled' : 'Disabled'}
                </span>
              }
            />
          );
        })()}
        noPadding
      >
        <form class="divide-y divide-border" onSubmit={state.handleSave}>
          <Show when={state.loading()}>
            <div class="flex items-center gap-3 text-sm text-muted p-4 sm:p-6">
              <span class="h-4 w-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
              {getAISettingsLoadingState().text}
            </div>
          </Show>

          <Show when={!state.loading() && state.loadError()}>
            <div class="flex items-center justify-between gap-3 p-4 sm:p-6 bg-red-50 dark:bg-red-900/30 border-b border-red-200 dark:border-red-800">
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
            <Show when={state.form.enabled}>
              <div class="p-4 sm:p-6">
                <div class="flex items-start gap-2 text-xs text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900 border border-blue-200 dark:border-blue-800 rounded-md p-3">
                  <svg
                    class="w-4 h-4 mt-0.5 shrink-0"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                  <span>
                    Patrol runs automatically every{' '}
                    {state.form.patrolIntervalMinutes >= 60
                      ? `${Math.round(state.form.patrolIntervalMinutes / 60)} hour${Math.round(state.form.patrolIntervalMinutes / 60) === 1 ? '' : 's'}`
                      : `${state.form.patrolIntervalMinutes} minute${state.form.patrolIntervalMinutes === 1 ? '' : 's'}`}{' '}
                    to verify your infrastructure continuously.{' '}
                    <button
                      type="button"
                      class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-1 py-1 text-sm underline hover:text-blue-800 dark:hover:text-blue-300"
                      onClick={() => navigate(PATROL_PATH)}
                    >
                      Configure schedule & autonomy
                    </button>
                  </span>
                </div>
              </div>
            </Show>
            <div class="space-y-6 p-4 sm:p-6">
              <AIModelSelectionSection state={state} />
              <AIRuntimeControlsSection state={state} />
              <AIChatMaintenanceSection state={state} />
            </div>

            <AISettingsStatusAndActions state={state} />
          </Show>
        </form>
      </SettingsPanel>

      <AISettingsDialogs
        showDiffModal={state.showDiffModal}
        setShowDiffModal={state.setShowDiffModal}
        diffFiles={state.diffFiles}
        diffSummary={state.diffSummary}
        diffSessionLabel={state.diffSessionLabel}
        formatDiffStats={state.formatDiffStats}
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

export default AISettings;
