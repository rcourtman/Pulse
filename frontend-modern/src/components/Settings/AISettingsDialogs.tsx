import { For, Show, type Accessor, type Component, type Setter } from 'solid-js';
import { Dialog } from '@/components/shared/Dialog';
import { SelectionCardGroup } from '@/components/shared/SelectionCardGroup';
import { getAISessionDiffStatusPresentation } from '@/utils/aiSessionDiffPresentation';
import type { FileChange } from '@/api/aiChat';
import type { AIProvider } from '@/types/ai';
import {
  AI_SETUP_PROVIDER_OPTIONS,
  getAIProviderConfig,
} from '@/components/Settings/aiSettingsModel';

export interface AISettingsDialogsProps {
  showDiffModal: Accessor<boolean>;
  setShowDiffModal: Setter<boolean>;
  diffFiles: Accessor<FileChange[]>;
  diffSummary: Accessor<string>;
  diffSessionLabel: Accessor<string>;
  formatDiffStats: (change: FileChange) => string;
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

  return (
    <>
      <Show when={props.showDiffModal()}>
        <Dialog
          isOpen={true}
          onClose={() => props.setShowDiffModal(false)}
          panelClass="max-w-2xl"
          ariaLabel="Session file changes"
        >
          <div class="w-full overflow-hidden">
            <div class="flex items-start justify-between gap-4 px-6 py-4 border-b border-border">
              <div>
                <h3 class="text-lg font-semibold text-base-content">Session File Changes</h3>
                <p class="text-xs text-muted">{props.diffSessionLabel() || 'Selected session'}</p>
              </div>
              <button
                type="button"
                class="text-sm text-muted hover:text-base-content"
                onClick={() => props.setShowDiffModal(false)}
              >
                Close
              </button>
            </div>
            <div class="p-6 space-y-4 max-h-[70vh] overflow-y-auto">
              <Show when={props.diffSummary()}>
                <div class="rounded-md border border-border bg-surface-alt p-3">
                  <p class="text-xs font-semibold text-base-content">Summary</p>
                  <p class="text-xs text-muted mt-1 whitespace-pre-wrap">{props.diffSummary()}</p>
                </div>
              </Show>
              <div class="space-y-2">
                <For each={props.diffFiles()}>
                  {(file) => {
                    const diffStatus = getAISessionDiffStatusPresentation(file.status);
                    return (
                      <div class="flex flex-col gap-1.5 sm:flex-row sm:items-center sm:justify-between rounded-md border border-border px-3 py-2 text-xs">
                        <div class="flex items-center gap-2 min-w-0">
                          <span
                            class={`inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase ${diffStatus.badgeClasses}`}
                          >
                            {diffStatus.label}
                          </span>
                          <span class="text-base-content truncate">{file.path}</span>
                        </div>
                        <span class="text-muted sm:flex-shrink-0">
                          {props.formatDiffStats(file)}
                        </span>
                      </div>
                    );
                  }}
                </For>
              </div>
            </div>
          </div>
        </Dialog>
      </Show>

      <Show when={props.showSetupModal()}>
        <Dialog
          isOpen={true}
          onClose={props.handleCloseSetupModal}
          panelClass="max-w-md"
          closeOnBackdrop={false}
          ariaLabel="Set up Pulse Assistant"
        >
          <div class="w-full overflow-hidden">
            <div class="bg-blue-600 px-6 py-4">
              <h3 class="text-lg font-semibold text-white">Set Up Pulse Assistant</h3>
              <p class="text-blue-100 text-sm mt-1">Choose a provider to get started</p>
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
                    <p class="text-xs text-slate-500 mt-1.5">
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
                  <p class="text-xs text-slate-500 mt-1.5">
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
                Enable Pulse Assistant
              </button>
            </div>
          </div>
        </Dialog>
      </Show>
    </>
  );
};

export default AISettingsDialogs;
