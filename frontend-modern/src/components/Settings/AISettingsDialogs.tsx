import { Show, type Accessor, type Component, type Setter } from 'solid-js';
import { Dialog } from '@/components/shared/Dialog';
import { SelectionCardGroup } from '@/components/shared/SelectionCardGroup';
import { getAISettingsSetupDialogPresentation } from '@/utils/aiSettingsPresentation';
import type { AIProvider } from '@/types/ai';
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
  setupOllamaUrl: Accessor<string>;
  setSetupOllamaUrl: Setter<string>;
  setupSaving: Accessor<boolean>;
  handleCloseSetupModal: () => void;
  handleSetupSubmit: () => Promise<void>;
}

export const AISettingsDialogs: Component<AISettingsDialogsProps> = (props) => {
  const setupProviderConfig = () => getAIProviderConfig(props.setupProvider());
  const setupPresentation = getAISettingsSetupDialogPresentation;

  return (
    <>
      <Show when={props.showSetupModal()}>
        <Dialog
          isOpen={true}
          onClose={props.handleCloseSetupModal}
          panelClass="max-w-md"
          closeOnBackdrop={false}
          ariaLabel={setupPresentation().ariaLabel}
        >
          <div class="w-full overflow-hidden">
            <div class="bg-blue-600 px-6 py-4">
              <h3 class="text-lg font-semibold text-white">{setupPresentation().title}</h3>
              <p class="text-blue-100 text-sm mt-1">{setupPresentation().description}</p>
            </div>

            <div class="p-6 space-y-4">
              <SelectionCardGroup
                options={AI_SETUP_PROVIDER_OPTIONS}
                value={props.setupProvider()}
                onChange={props.setSetupProvider}
                variant="compact"
              />

              <Show
                when={props.setupProvider() === 'ollama'}
                fallback={
                  <div>
                    <label class="block text-sm font-medium text-base-content mb-1.5">
                      {setupProviderConfig().title} API Key
                    </label>
                    <input
                      type="password"
                      value={props.setupApiKey()}
                      onInput={(event) => props.setSetupApiKey(event.currentTarget.value)}
                      placeholder={setupProviderConfig().placeholder}
                      class="w-full px-3 py-2 border border-border rounded-md bg-surface focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    />
                    <p class="text-xs text-muted mt-1.5">
                      <a
                        href={setupProviderConfig().actionLinkHref}
                        target="_blank"
                        rel="noopener"
                        class="text-blue-600 hover:underline"
                      >
                        Get your API key →
                      </a>
                    </p>
                  </div>
                }
              >
                <div>
                  <label class="block text-sm font-medium text-base-content mb-1.5">
                    Ollama Server URL
                  </label>
                  <input
                    type="url"
                    value={props.setupOllamaUrl()}
                    onInput={(event) => props.setSetupOllamaUrl(event.currentTarget.value)}
                    placeholder="http://localhost:11434"
                    class="w-full px-3 py-2 border border-border rounded-md focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                  />
                  <p class="text-xs text-muted mt-1.5">
                    Ollama runs locally - no API key needed
                  </p>
                </div>
              </Show>
            </div>

            <div class="px-6 py-4 bg-surface-alt border-t border-border flex justify-end gap-3">
              <button
                type="button"
                onClick={props.handleCloseSetupModal}
                class="px-4 py-2 text-base-content hover:bg-surface-hover rounded-md"
                disabled={props.setupSaving()}
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={() => void props.handleSetupSubmit()}
                class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 flex items-center gap-2"
                disabled={
                  props.setupSaving() ||
                  (props.setupProvider() !== 'ollama' && !props.setupApiKey().trim()) ||
                  (props.setupProvider() === 'ollama' && !props.setupOllamaUrl().trim())
                }
              >
                {props.setupSaving() && (
                  <span class="h-4 w-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                )}
                {setupPresentation().submitLabel}
              </button>
            </div>
          </div>
        </Dialog>
      </Show>
    </>
  );
};

export default AISettingsDialogs;
