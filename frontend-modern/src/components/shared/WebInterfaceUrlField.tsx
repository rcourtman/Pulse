import { Component, Show } from 'solid-js';
import ExternalLinkIcon from 'lucide-solid/icons/external-link';
import { sessionCanWriteMonitoringMetadata } from '@/stores/sessionCapabilities';
import { Button, CopyValueButton } from './Button';
import { getInfoCardFrameClass } from './InfoCardFrame';
import { useWebInterfaceUrlFieldState } from './useWebInterfaceUrlFieldState';
import type { WebInterfaceUrlFieldProps } from './webInterfaceUrlFieldModel';
import { WebInterfaceLink } from './WebInterfaceLink';

export type { WebInterfaceUrlFieldProps } from './webInterfaceUrlFieldModel';

export const WebInterfaceUrlField: Component<WebInterfaceUrlFieldProps> = (props) => {
  const state = useWebInterfaceUrlFieldState(props);
  // Metadata writes need monitoring:write; read-only API-token sessions such
  // as kiosk links see the saved URL without controls that would be refused.
  const canEdit = () => sessionCanWriteMonitoringMetadata();
  const title = () => props.title?.trim() || 'Web Interface URL';
  const rootClass = () =>
    props.embedded ? (props.class ?? '') : getInfoCardFrameClass({ class: props.class });

  return (
    <Show when={state.metadataId()}>
      <div class={rootClass()}>
        <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
          {title()}
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <Show
            when={canEdit()}
            fallback={
              <span
                class="min-w-[180px] flex-1 truncate text-xs text-base-content"
                title={state.normalizedCurrentUrl() || undefined}
              >
                {state.normalizedCurrentUrl() || 'No URL saved'}
              </span>
            }
          >
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
            <Button
              variant="primaryFlat"
              size="sm"
              disabled={
                state.urlSaving() || state.urlValue().trim() === state.normalizedCurrentUrl()
              }
              onClick={() => void state.handleSaveUrl()}
            >
              Save
            </Button>
          </Show>
          <Show when={state.normalizedCurrentUrl()}>
            <WebInterfaceLink
              url={state.normalizedCurrentUrl()}
              ariaLabel="Open saved web interface URL"
              class="inline-flex min-h-8 min-w-8 items-center justify-center rounded-md text-blue-600 transition-colors hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-900/25"
              title="Open saved web interface URL"
              invalidAriaLabel="Saved web interface URL is invalid"
            >
              <ExternalLinkIcon class="h-3.5 w-3.5" aria-hidden="true" />
            </WebInterfaceLink>
          </Show>
          <Show when={state.normalizedCurrentUrl()}>
            <CopyValueButton
              value={state.normalizedCurrentUrl()}
              copied={state.copiedUrlValue() === state.normalizedCurrentUrl()}
              onCopyValue={state.handleCopyUrl}
              label="Copy URL"
              variant="ghost"
              size="lg"
            />
          </Show>
          <Show when={canEdit() && state.normalizedCurrentUrl()}>
            <Button
              variant="dangerOutline"
              size="sm"
              disabled={state.urlSaving()}
              onClick={() => void state.handleDeleteUrl()}
              title="Remove URL"
            >
              Remove
            </Button>
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

        <p class="mt-1.5 whitespace-normal text-[10px] text-muted">
          <Show
            when={canEdit()}
            fallback="This session can open saved web interface URLs but not change them."
          >
            Add a URL to quickly access this {state.targetLabel()}'s web interface from Pulse.
          </Show>
        </p>
      </div>
    </Show>
  );
};

export default WebInterfaceUrlField;
