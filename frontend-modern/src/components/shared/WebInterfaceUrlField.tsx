import { Component, Show } from 'solid-js';
import { useWebInterfaceUrlFieldState } from './useWebInterfaceUrlFieldState';
import type { WebInterfaceUrlFieldProps } from './webInterfaceUrlFieldModel';

export type { WebInterfaceUrlFieldProps } from './webInterfaceUrlFieldModel';

export const WebInterfaceUrlField: Component<WebInterfaceUrlFieldProps> = (props) => {
  const state = useWebInterfaceUrlFieldState(props);

  return (
    <Show when={state.metadataId()}>
      <div class={`rounded border border-border bg-surface p-3 shadow-sm ${props.class ?? ''}`}>
        <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
          Web Interface URL
        </div>
        <div class="flex items-center gap-2">
          <input
            type="url"
            class="flex-1 text-xs px-2.5 py-1.5 border border-border rounded-md bg-surface text-base-content focus:ring-1 focus:ring-blue-500 focus:border-blue-500 transition-colors"
            placeholder="https://198.51.100.100:8080"
            value={state.urlValue()}
            onInput={(e) => state.setUrlValue(e.currentTarget.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                void state.handleSaveUrl();
              }
            }}
            disabled={state.urlSaving()}
          />
          <button
            type="button"
            class="px-2.5 py-1.5 text-xs font-medium rounded-md bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50 transition-colors"
            disabled={state.urlSaving() || state.urlValue().trim() === state.normalizedCurrentUrl()}
            onClick={() => void state.handleSaveUrl()}
          >
            Save
          </button>
          <Show when={state.normalizedCurrentUrl()}>
            <a
              href={state.normalizedCurrentUrl()}
              target="_blank"
              rel="noopener noreferrer"
              class="px-2.5 py-1.5 text-xs font-medium rounded-md text-blue-600 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-900 transition-colors"
              title="Open URL"
            >
              Open
            </a>
          </Show>
          <Show when={state.normalizedCurrentUrl()}>
            <button
              type="button"
              class="px-2.5 py-1.5 text-xs font-medium rounded-md text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900 disabled:opacity-50 transition-colors"
              disabled={state.urlSaving()}
              onClick={() => void state.handleDeleteUrl()}
              title="Remove URL"
            >
              Remove
            </button>
          </Show>
        </div>

        <Show when={state.urlError()}>
          <p class="mt-1.5 text-[11px] text-red-600 dark:text-red-400">{state.urlError()}</p>
        </Show>
        <Show when={state.urlSuccess()}>
          <p class="mt-1.5 text-[11px] text-emerald-600 dark:text-emerald-400">
            {state.urlSuccess()}
          </p>
        </Show>

        <Show when={state.showSuggestedDiagnostic()}>
          <div class="mt-2 rounded border border-amber-200 bg-amber-50 p-2 text-[11px] text-amber-800 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-200">
            <p class="font-medium">{state.suggestedUrlFallback().title}</p>
            <p class="mt-0.5">{state.suggestedUrlFallback().description}</p>
          </div>
        </Show>

        <Show when={state.showSuggestedUrl()}>
          <div class="mt-2 p-2 rounded bg-blue-50 border border-blue-200 dark:bg-blue-900 dark:border-blue-800">
            <div class="text-[10px] font-medium text-blue-700 dark:text-blue-300 mb-1">
              {state.normalizedCurrentUrl() ? 'Discovered URL' : 'Suggested URL'}
            </div>
            <Show when={props.suggestedUrlReasonText}>
              <p
                class="mb-1 text-[10px] text-blue-700 dark:text-blue-300"
                title={props.suggestedUrlReasonTitle}
              >
                Why this URL: {props.suggestedUrlReasonText}
              </p>
            </Show>
            <div class="flex items-center gap-2">
              <code
                class="flex-1 text-xs text-blue-800 dark:text-blue-200 font-mono truncate"
                title={state.normalizedSuggestedUrl()}
              >
                {state.normalizedSuggestedUrl()}
              </code>
              <button
                type="button"
                class="px-2 py-1 text-xs font-medium rounded bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50 transition-colors flex-shrink-0"
                onClick={() => state.setUrlValue(state.normalizedSuggestedUrl())}
                disabled={state.urlSaving()}
              >
                {state.normalizedCurrentUrl() ? 'Use instead' : 'Use this'}
              </button>
            </div>
          </div>
        </Show>

        <p class="mt-1.5 text-[10px] text-muted">
          Add a URL to quickly access this {state.targetLabel()}'s web interface from the dashboard.
        </p>
      </div>
    </Show>
  );
};

export default WebInterfaceUrlField;
