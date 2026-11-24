import { Show, type Accessor, type Component, type Setter } from 'solid-js';
import { Dialog } from '@/components/shared/Dialog';
import { ExternalTextLink } from '@/components/shared/ExternalTextLink';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { SelectionCardGroup } from '@/components/shared/SelectionCardGroup';
import { getAISettingsSetupDialogPresentation } from '@/utils/aiSettingsPresentation';
import type { AIProvider } from '@/types/ai';
import type { PendingRemoveProvider } from '@/components/Settings/useAISettingsState';
import {
  AI_SETUP_PROVIDER_OPTIONS,
  getAIProviderConfig,
} from '@/components/Settings/aiSettingsModel';

export interface AISettingsDialogsProps {
  showSetupModal: Accessor<boolean>;
  setupProvider: Accessor<AIProvider>;
  setSetupProvider: Setter<AIProvider>;
  setupApiKey: Accessor<string>;
  setSetupApiKey: Setter<string>;
  setupOpenAIBaseUrl: Accessor<string>;
  setSetupOpenAIBaseUrl: Setter<string>;
  setupOllamaUrl: Accessor<string>;
  setSetupOllamaUrl: Setter<string>;
  setupSaving: Accessor<boolean>;
  handleCloseSetupModal: () => void;
  handleSetupSubmit: () => Promise<void>;
  pendingRemoveProvider?: Accessor<PendingRemoveProvider | null>;
  removeSaving?: Accessor<boolean>;
  handleCancelRemoveProvider?: () => void;
  handleConfirmRemoveProvider?: () => Promise<void>;
}

export const AISettingsDialogs: Component<AISettingsDialogsProps> = (props) => {
  const setupProviderConfig = () => getAIProviderConfig(props.setupProvider());
  const setupPresentation = getAISettingsSetupDialogPresentation;
  const isSubscriptionAgent = () =>
    props.setupProvider() === 'codex-subscription' ||
    props.setupProvider() === 'claude-subscription';
  const isOpenAI = () => props.setupProvider() === 'openai';
  const canSubmitSetup = () => {
    if (props.setupSaving()) return false;
    if (isSubscriptionAgent()) return true;
    if (props.setupProvider() === 'ollama') return Boolean(props.setupOllamaUrl().trim());
    if (isOpenAI()) {
      return Boolean(props.setupApiKey().trim() || props.setupOpenAIBaseUrl().trim());
    }
    return Boolean(props.setupApiKey().trim());
  };

  return (
    <>
      <Show when={props.showSetupModal()}>
        <Dialog
          isOpen={true}
          onClose={props.handleCloseSetupModal}
          panelClass="max-w-lg"
          closeOnBackdrop={false}
          ariaLabel={setupPresentation().ariaLabel}
        >
          <div class="flex min-h-0 w-full flex-1 flex-col overflow-hidden">
            <div class="shrink-0 bg-blue-600 px-6 py-4">
              <h3 class="text-lg font-semibold text-white">{setupPresentation().title}</h3>
              <p class="text-blue-100 text-sm mt-1">{setupPresentation().description}</p>
            </div>

            <div
              class="min-h-0 flex-1 overflow-y-auto p-6 space-y-4"
              data-testid="ai-setup-dialog-body"
            >
              <SelectionCardGroup
                options={AI_SETUP_PROVIDER_OPTIONS}
                value={props.setupProvider()}
                onChange={props.setSetupProvider}
                variant="compact"
              />

              <Show
                when={isSubscriptionAgent()}
                fallback={
                  <Show
                    when={props.setupProvider() === 'ollama'}
                    fallback={
                      <div class="space-y-3">
                        <div>
                          <label
                            for="ai-setup-api-key"
                            class="block text-sm font-medium text-base-content mb-1.5"
                          >
                            {isOpenAI()
                              ? 'OpenAI API Key (optional for a custom endpoint)'
                              : `${setupProviderConfig().title} API Key`}
                          </label>
                          <input
                            id="ai-setup-api-key"
                            type="password"
                            autocomplete="new-password"
                            value={props.setupApiKey()}
                            onInput={(event) => props.setSetupApiKey(event.currentTarget.value)}
                            placeholder={setupProviderConfig().placeholder}
                            class="w-full px-3 py-2 border border-border rounded-md bg-surface focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                          />
                          <p class="text-xs text-muted mt-1.5">
                            <ExternalTextLink href={setupProviderConfig().actionLinkHref}>
                              {setupProviderConfig().actionLinkLabel}
                            </ExternalTextLink>
                          </p>
                        </div>
                        <Show when={isOpenAI()}>
                          <div>
                            <label
                              for="ai-setup-openai-base-url"
                              class="block text-sm font-medium text-base-content mb-1.5"
                            >
                              Custom Base URL
                            </label>
                            <input
                              id="ai-setup-openai-base-url"
                              type="url"
                              autocomplete="off"
                              value={props.setupOpenAIBaseUrl()}
                              onInput={(event) =>
                                props.setSetupOpenAIBaseUrl(event.currentTarget.value)
                              }
                              placeholder="https://api.together.xyz/v1 (optional)"
                              class="w-full px-3 py-2 border border-border rounded-md bg-surface focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                            />
                            <p class="text-xs text-muted mt-1.5">
                              Use llama.cpp, LocalAI, LM Studio, or another OpenAI-compatible
                              endpoint. Leave the API key blank when the endpoint does not require
                              one.
                            </p>
                          </div>
                        </Show>
                      </div>
                    }
                  >
                    <div>
                      <label
                        for="ai-setup-ollama-url"
                        class="block text-sm font-medium text-base-content mb-1.5"
                      >
                        Ollama Server URL
                      </label>
                      <input
                        id="ai-setup-ollama-url"
                        type="url"
                        value={props.setupOllamaUrl()}
                        onInput={(event) => props.setSetupOllamaUrl(event.currentTarget.value)}
                        placeholder="http://localhost:11434"
                        class="w-full px-3 py-2 border border-border rounded-md bg-surface focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                      />
                      <p class="text-xs text-muted mt-1.5">
                        Ollama runs locally with no API key needed.
                      </p>
                    </div>
                  </Show>
                }
              >
                <div class="rounded-md border border-border bg-surface-alt p-3 text-sm text-base-content">
                  Pulse will use this machine's existing {setupProviderConfig().title} login. No API
                  key or OAuth token is stored in Pulse. Save, then run the provider check to verify
                  the CLI and login are available to the Pulse process.
                  <p class="mt-2 text-xs">
                    <ExternalTextLink href={setupProviderConfig().actionLinkHref}>
                      {setupProviderConfig().actionLinkLabel}
                    </ExternalTextLink>
                  </p>
                </div>
              </Show>
            </div>

            <div class="shrink-0 px-6 py-4 bg-surface-alt border-t border-border flex justify-end gap-3">
              <button
                type="button"
                onClick={props.handleCloseSetupModal}
                class="min-h-10 sm:min-h-9 px-4 py-2 text-sm text-base-content hover:bg-surface-hover rounded-md"
                disabled={props.setupSaving()}
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={() => void props.handleSetupSubmit()}
                class="min-h-10 sm:min-h-9 px-4 py-2 text-sm bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 flex items-center gap-2"
                disabled={!canSubmitSetup()}
              >
                {props.setupSaving() && <LoadingSpinner size="md" tone="inverse" />}
                {setupPresentation().submitLabel}
              </button>
            </div>
          </div>
        </Dialog>
      </Show>

      <Show when={props.pendingRemoveProvider?.()}>
        {(pending) => (
          <Dialog
            isOpen={true}
            onClose={() => props.handleCancelRemoveProvider?.()}
            panelClass="max-w-md"
            closeOnBackdrop={false}
            ariaLabel={`Remove ${pending().providerLabel}`}
          >
            <div class="flex min-h-0 w-full flex-1 flex-col overflow-hidden">
              <div class="shrink-0 border-b border-border bg-surface-alt px-6 py-4">
                <h3 class="text-base font-semibold text-base-content">
                  Remove {pending().providerLabel}
                </h3>
              </div>
              <div class="min-h-0 flex-1 overflow-y-auto px-6 py-4">
                <p class="text-sm text-muted">{pending().message}</p>
              </div>
              <div class="shrink-0 flex justify-end gap-3 border-t border-border bg-surface-alt px-6 py-4">
                <button
                  type="button"
                  onClick={() => props.handleCancelRemoveProvider?.()}
                  disabled={props.removeSaving?.()}
                  class="min-h-10 sm:min-h-9 rounded-md px-4 py-2 text-sm text-base-content hover:bg-surface-hover disabled:opacity-50"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={() => void props.handleConfirmRemoveProvider?.()}
                  disabled={props.removeSaving?.()}
                  class="min-h-10 sm:min-h-9 rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 disabled:opacity-50"
                >
                  {props.removeSaving?.() ? 'Removing...' : 'Remove provider'}
                </button>
              </div>
            </div>
          </Dialog>
        )}
      </Show>
    </>
  );
};

export default AISettingsDialogs;
