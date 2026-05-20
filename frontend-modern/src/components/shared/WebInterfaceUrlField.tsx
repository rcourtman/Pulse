import { Component, Show } from 'solid-js';
import CheckIcon from 'lucide-solid/icons/check';
import CopyIcon from 'lucide-solid/icons/copy';
import ExternalLinkIcon from 'lucide-solid/icons/external-link';
import { DiscoveryProvenanceMarker } from './DiscoveryProvenanceMarker';
import { useWebInterfaceUrlFieldState } from './useWebInterfaceUrlFieldState';
import type { WebInterfaceUrlFieldProps } from './webInterfaceUrlFieldModel';

export type { WebInterfaceUrlFieldProps } from './webInterfaceUrlFieldModel';

export const WebInterfaceUrlField: Component<WebInterfaceUrlFieldProps> = (props) => {
  const state = useWebInterfaceUrlFieldState(props);
  const title = () => props.title?.trim() || 'Web Interface URL';
  const rootClass = () =>
    props.embedded
      ? (props.class ?? '')
      : `rounded border border-border bg-surface p-3 shadow-sm ${props.class ?? ''}`.trim();

  return (
    <Show when={state.metadataId()}>
      <div class={rootClass()}>
        <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
          {title()}
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <input
            type="url"
            class="min-w-[180px] flex-1 text-xs px-2.5 py-1.5 border border-border rounded-md bg-surface text-base-content focus:ring-1 focus:ring-blue-500 focus:border-blue-500 transition-colors"
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
              class="inline-flex min-h-8 min-w-8 items-center justify-center rounded-md text-blue-600 transition-colors hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-900"
              title="Open URL"
              aria-label="Open URL"
            >
              <ExternalLinkIcon class="h-3.5 w-3.5" />
            </a>
          </Show>
          <Show when={state.normalizedCurrentUrl()}>
            <button
              type="button"
              class="inline-flex min-h-8 min-w-8 items-center justify-center rounded-md text-muted transition-colors hover:bg-surface-hover hover:text-base-content"
              onClick={() => void state.handleCopyUrl(state.normalizedCurrentUrl())}
              title="Copy URL"
              aria-label="Copy URL"
            >
              <Show
                when={state.copiedUrlValue() === state.normalizedCurrentUrl()}
                fallback={<CopyIcon class="h-3.5 w-3.5" />}
              >
                <CheckIcon class="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
              </Show>
            </button>
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
          <p
            role="alert"
            aria-live="assertive"
            class="mt-1.5 text-[11px] text-red-600 dark:text-red-400"
          >
            {state.urlError()}
          </p>
        </Show>
        <Show when={state.urlSuccess()}>
          <p
            role="status"
            aria-live="polite"
            class="mt-1.5 text-[11px] text-emerald-600 dark:text-emerald-400"
          >
            {state.urlSuccess()}
          </p>
        </Show>

        <Show when={state.showSuggestedDiagnostic()}>
          <div class="mt-2 rounded border border-amber-200 bg-amber-50 p-2 text-[11px] text-amber-800 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-200">
            <div class="flex items-center gap-1.5 font-medium">
              <span>{state.suggestedUrlFallback().title}</span>
              <DiscoveryProvenanceMarker />
            </div>
            <p class="mt-0.5">{state.suggestedUrlFallback().description}</p>
          </div>
        </Show>

        <Show when={state.showSuggestedUrl()}>
          <div class="mt-2 p-2 rounded bg-blue-50 border border-blue-200 dark:bg-blue-900 dark:border-blue-800">
            <div class="mb-1 flex items-center gap-1.5 text-[10px] font-medium text-blue-700 dark:text-blue-300">
              <span>{state.normalizedCurrentUrl() ? 'Discovered URL' : 'Suggested URL'}</span>
              <DiscoveryProvenanceMarker />
            </div>
            <Show when={props.suggestedUrlReasonText}>
              <p
                class="mb-1 text-[10px] text-blue-700 dark:text-blue-300"
                title={props.suggestedUrlReasonTitle}
              >
                Why this URL: {props.suggestedUrlReasonText}
              </p>
            </Show>
            <div class="flex flex-wrap items-center gap-2">
              <code
                class="min-w-[180px] flex-1 text-xs text-blue-800 dark:text-blue-200 font-mono truncate"
                title={state.normalizedSuggestedUrl()}
              >
                {state.normalizedSuggestedUrl()}
              </code>
              <a
                href={state.normalizedSuggestedUrl()}
                target="_blank"
                rel="noopener noreferrer"
                class="inline-flex min-h-7 min-w-7 shrink-0 items-center justify-center rounded text-blue-700 transition-colors hover:bg-blue-100 dark:text-blue-200 dark:hover:bg-blue-950"
                title="Open suggested URL"
                aria-label="Open suggested URL"
              >
                <ExternalLinkIcon class="h-3.5 w-3.5" />
              </a>
              <button
                type="button"
                class="inline-flex min-h-7 min-w-7 shrink-0 items-center justify-center rounded text-blue-700 transition-colors hover:bg-blue-100 dark:text-blue-200 dark:hover:bg-blue-950"
                onClick={() => void state.handleCopyUrl(state.normalizedSuggestedUrl())}
                title="Copy suggested URL"
                aria-label="Copy suggested URL"
              >
                <Show
                  when={state.copiedUrlValue() === state.normalizedSuggestedUrl()}
                  fallback={<CopyIcon class="h-3.5 w-3.5" />}
                >
                  <CheckIcon class="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
                </Show>
              </button>
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
          Add a URL to quickly access this {state.targetLabel()}'s web interface from Workloads.
        </p>
      </div>
    </Show>
  );
};

export default WebInterfaceUrlField;
