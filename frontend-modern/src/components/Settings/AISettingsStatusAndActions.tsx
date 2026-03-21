import { Component, Show } from 'solid-js';
import type { AISettingsState } from '@/components/Settings/useAISettingsState';

interface AISettingsStatusAndActionsProps {
  state: AISettingsState;
}

export const AISettingsStatusAndActions: Component<AISettingsStatusAndActionsProps> = (props) => {
  const { state } = props;

  return (
    <>
      <Show when={state.settings()}>
        <div class="p-4 sm:p-6">
          <div
            class={`flex items-center gap-2 p-3 rounded-md ${state.settingsReadiness().containerClassName}`}
          >
            <div class={`w-2 h-2 rounded-full ${state.settingsReadiness().dotClassName}`} />
            <div class="flex-1 min-w-0">
              <span class="text-xs font-medium">{state.settingsReadiness().summary}</span>
              <Show when={state.settings()?.configured && state.settings()?.model}>
                <span class="block sm:inline text-xs opacity-75 sm:ml-2">
                  • Default: {state.settings()?.model?.split(':').pop() || state.settings()?.model}
                </span>
              </Show>
            </div>
          </div>
        </div>
      </Show>

      <div class="sticky bottom-0 bg-surface px-4 sm:px-6 py-4 flex flex-col sm:flex-row sm:flex-wrap sm:items-center sm:justify-between gap-3">
        <Show when={state.settings()?.configured}>
          <button
            type="button"
            class="w-full sm:w-auto min-h-10 sm:min-h-9 px-4 py-2.5 text-sm border border-blue-300 dark:border-blue-700 text-blue-700 dark:text-blue-300 rounded-md hover:bg-blue-50 dark:hover:bg-blue-900 disabled:opacity-50 disabled:cursor-not-allowed"
            onClick={state.handleTest}
            disabled={state.testing() || state.saving() || state.loading()}
          >
            {state.testing() ? 'Testing...' : 'Test Connection'}
          </button>
        </Show>
        <div class="grid grid-cols-1 sm:flex gap-3 w-full sm:w-auto sm:ml-auto">
          <button
            type="button"
            class="w-full sm:w-auto min-h-10 sm:min-h-9 px-4 py-2.5 border border-border text-base-content rounded-md hover:bg-surface-hover"
            onClick={() => state.resetForm(state.settings())}
            disabled={state.saving() || state.loading()}
          >
            Reset
          </button>
          <button
            type="submit"
            class="w-full sm:w-auto min-h-10 sm:min-h-9 px-4 py-2.5 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
            disabled={state.saving() || state.loading()}
          >
            {state.saving() ? 'Saving...' : 'Save changes'}
          </button>
        </div>
      </div>
    </>
  );
};
