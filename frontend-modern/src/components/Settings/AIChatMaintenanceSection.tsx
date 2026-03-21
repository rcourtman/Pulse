import { Component, For, Show } from 'solid-js';
import type { AISettingsState } from '@/components/Settings/useAISettingsState';
import {
  getAIChatSessionsEmptyState,
  getAIChatSessionsLoadingState,
} from '@/utils/aiSettingsPresentation';

interface AIChatMaintenanceSectionProps {
  state: AISettingsState;
}

export const AIChatMaintenanceSection: Component<AIChatMaintenanceSectionProps> = (props) => {
  const { state } = props;

  return (
    <div class="border border-border rounded-md overflow-hidden">
      <button
        type="button"
        class="w-full min-h-10 sm:min-h-9 px-3 py-2.5 flex items-center justify-between bg-surface-alt hover:bg-surface-hover transition-colors text-left"
        onClick={() => {
          const next = !state.showChatMaintenance();
          state.setShowChatMaintenance(next);
          if (next) {
            state.loadChatSessions();
          }
        }}
      >
        <div class="flex items-center gap-2">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4"
            />
          </svg>
          <span class="text-sm font-medium text-base-content">Chat Session Maintenance</span>
        </div>
        <svg
          class={`w-4 h-4 transition-transform ${state.showChatMaintenance() ? 'rotate-180' : ''}`}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
        </svg>
      </button>
      <Show when={state.showChatMaintenance()}>
        <div class="px-3 py-3 bg-surface border-t border-border space-y-3">
          <p class="text-xs text-muted">
            Use this panel to summarize, inspect, or revert a specific chat session. It does not
            change your default Pulse Assistant settings.
          </p>
          <div class="flex items-center justify-between">
            <label class="text-xs font-medium text-muted">Session</label>
            <button
              type="button"
              onClick={state.loadChatSessions}
              disabled={state.chatSessionsLoading()}
              class="inline-flex min-h-10 sm:min-h-9 items-center rounded-md px-2 py-1.5 text-sm text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 disabled:opacity-50"
            >
              {state.chatSessionsLoading() ? 'Refreshing...' : 'Refresh'}
            </button>
          </div>

          <Show when={state.chatSessionsLoading()}>
            <div class="text-xs text-muted">{getAIChatSessionsLoadingState().text}</div>
          </Show>
          <Show when={!state.chatSessionsLoading()}>
            <Show when={state.chatSessionsError()}>
              <div class="text-xs text-red-500">{state.chatSessionsError()}</div>
            </Show>
            <Show when={!state.chatSessionsError()}>
              <Show
                when={state.chatSessions().length > 0}
                fallback={<div class="text-xs text-muted">{getAIChatSessionsEmptyState().text}</div>}
              >
                <select
                  value={state.selectedSessionId()}
                  onChange={(e) => state.setSelectedSessionId(e.currentTarget.value)}
                  class="w-full min-h-10 sm:min-h-9 px-2 py-2 text-sm border border-border rounded"
                  disabled={state.saving()}
                >
                  <For each={state.chatSessions()}>
                    {(session) => <option value={session.id}>{state.formatSessionLabel(session)}</option>}
                  </For>
                </select>
                <Show when={state.selectedChatSession()}>
                  <p class="text-[10px] text-muted mt-1">
                    Last updated {new Date(state.selectedChatSession()!.updated_at).toLocaleString()}
                  </p>
                </Show>
              </Show>
            </Show>
          </Show>

          <div class="flex flex-wrap gap-2 pt-1">
            <button
              type="button"
              onClick={state.handleSessionSummarize}
              disabled={!state.selectedSessionId() || state.sessionActionLoading() !== null}
              class="w-full sm:w-auto min-h-10 sm:min-h-9 px-3 py-2 text-sm font-medium rounded border border-border bg-surface text-base-content hover:bg-surface-hover disabled:opacity-50"
            >
              {state.sessionActionLoading() === 'summarize' ? 'Summarizing...' : 'Summarize context'}
            </button>
            <button
              type="button"
              onClick={state.handleSessionDiff}
              disabled={!state.selectedSessionId() || state.sessionActionLoading() !== null}
              class="w-full sm:w-auto min-h-10 sm:min-h-9 px-3 py-2 text-sm font-medium rounded border border-border bg-surface text-base-content hover:bg-surface-hover disabled:opacity-50"
            >
              {state.sessionActionLoading() === 'diff' ? 'Loading...' : 'View file changes'}
            </button>
            <button
              type="button"
              onClick={state.handleSessionRevert}
              disabled={!state.selectedSessionId() || state.sessionActionLoading() !== null}
              class="w-full sm:w-auto min-h-10 sm:min-h-9 px-3 py-2 text-sm font-medium rounded border border-red-200 dark:border-red-700 bg-red-50 dark:bg-red-900 text-red-700 dark:text-red-300 hover:bg-red-100 dark:hover:bg-red-900 disabled:opacity-50"
            >
              {state.sessionActionLoading() === 'revert' ? 'Reverting...' : 'Revert changes'}
            </button>
          </div>
          <p class="text-[10px] text-muted">These actions apply to the selected chat session only.</p>
        </div>
      </Show>
    </div>
  );
};
