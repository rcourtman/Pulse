import { Component, For, Show } from 'solid-js';
import { AIProviderConfigurationSection } from '@/components/Settings/AIProviderConfigurationSection';
import {
  groupModelsByProvider,
  isAIProviderConfigured,
  isModelProviderConfigured,
} from '@/components/Settings/aiSettingsModel';
import type { AISettingsState } from '@/components/Settings/useAISettingsState';
import { formField, labelClass, controlClass } from '@/components/shared/Form';
import { getAIProviderDisplayName, getProviderFromModelId } from '@/utils/aiProviderPresentation';

interface AIModelSelectionSectionProps {
  state: AISettingsState;
}

export const AIModelSelectionSection: Component<AIModelSelectionSectionProps> = (props) => {
  const { state } = props;
  const groupedModels = () => Array.from(groupModelsByProvider(state.availableModels()).entries());

  return (
    <>
      <div class={formField}>
        <div class="flex items-center justify-between mb-1">
          <label class={labelClass()}>
            Default Model
            {state.modelsLoading() && <span class="ml-2 text-xs text-slate-500">(loading...)</span>}
          </label>
          <button
            type="button"
            onClick={state.loadModels}
            disabled={state.modelsLoading()}
            class="inline-flex min-h-10 sm:min-h-9 items-center gap-1 rounded-md px-2 py-1.5 text-sm text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 disabled:opacity-50"
            title="Refresh model list from all configured providers"
          >
            <svg
              class={`w-3 h-3 ${state.modelsLoading() ? 'animate-spin' : ''}`}
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
              />
            </svg>
            Refresh
          </button>
        </div>
        <Show
          when={state.availableModels().length > 0}
          fallback={
            <input
              type="text"
              value={state.form.model}
              onInput={(e) => state.setForm('model', e.currentTarget.value)}
              placeholder="Configure a provider below to see available models"
              class={controlClass()}
              disabled={state.saving()}
            />
          }
        >
          <select
            value={state.form.model}
            onChange={(e) => state.setForm('model', e.currentTarget.value)}
            class={controlClass()}
            disabled={state.saving()}
          >
            <Show
              when={
                !state.form.model ||
                !state.availableModels().some((model) => model.id === state.form.model)
              }
            >
              <option value={state.form.model}>{state.form.model || 'Select a model...'}</option>
            </Show>
            <For
              each={groupedModels().filter(([provider]) =>
                isAIProviderConfigured(provider, state.settings()),
              )}
            >
              {([provider, models]) => (
                <optgroup label={getAIProviderDisplayName(provider) || provider}>
                  <For each={models}>
                    {(model) => (
                      <option value={model.id} selected={model.id === state.form.model}>
                        {model.name || model.id.split(':').pop()}
                      </option>
                    )}
                  </For>
                </optgroup>
              )}
            </For>
            <For
              each={groupedModels().filter(([provider]) =>
                !isAIProviderConfigured(provider, state.settings()),
              )}
            >
              {([provider, models]) => (
                <optgroup
                  label={`${getAIProviderDisplayName(provider) || provider} (not configured)`}
                >
                  <For each={models}>
                    {(model) => (
                      <option
                        value={model.id}
                        selected={model.id === state.form.model}
                        class="text-slate-400"
                      >
                        {model.name || model.id.split(':').pop()}
                      </option>
                    )}
                  </For>
                </optgroup>
              )}
            </For>
          </select>
        </Show>
        <Show when={state.modelsError()}>
          <p class="text-xs text-amber-600 dark:text-amber-400 mt-1 flex items-center gap-1">
            <svg
              class="w-3.5 h-3.5 flex-shrink-0"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
            Failed to load models: {state.modelsError()}. Enter a model ID manually (format:
            provider:model-name) or click Refresh to retry.
          </p>
        </Show>
        <Show when={state.form.model && !isModelProviderConfigured(state.form.model, state.settings())}>
          <p class="text-xs text-amber-600 dark:text-amber-400 mt-1 flex items-center gap-1">
            <svg
              class="w-3.5 h-3.5 flex-shrink-0"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
            This model requires{' '}
            {getAIProviderDisplayName(getProviderFromModelId(state.form.model)) ||
              getProviderFromModelId(state.form.model)}{' '}
            to be configured. Add an API key below or select a different model.
          </p>
        </Show>
      </div>

      <div class="border border-border rounded-md overflow-hidden">
        <button
          type="button"
          class="w-full min-h-10 sm:min-h-9 px-3 py-2.5 flex items-center justify-between bg-surface-alt hover:bg-surface-hover transition-colors text-left"
          onClick={() => state.setShowAdvancedModels(!state.showAdvancedModels())}
        >
          <div class="flex items-center gap-2">
            <svg class="w-4 h-4 text-slate-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4"
              />
            </svg>
            <span class="text-sm font-medium text-base-content">Advanced Model Selection</span>
            <Show when={state.form.chatModel || state.form.patrolModel}>
              <span class="px-1.5 py-0.5 text-[10px] font-semibold bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded">
                Customized
              </span>
            </Show>
          </div>
          <svg
            class={`w-4 h-4 text-slate-500 transition-transform ${state.showAdvancedModels() ? 'rotate-180' : ''}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </button>
        <Show when={state.showAdvancedModels()}>
          <div class="px-3 py-3 bg-surface border-t border-border space-y-3">
            <p class="text-xs text-muted">
              Override the default model for specific tasks. Leave empty to use the default.
            </p>
            <div>
              <label class="block text-xs font-medium text-muted mb-0.5">Chat Model (Interactive)</label>
              <p class="text-[11px] text-muted mb-1">
                Used for chat and fix execution — a more capable model is recommended.
              </p>
              <Show
                when={state.availableModels().length > 0}
                fallback={
                  <input
                    type="text"
                    value={state.form.chatModel}
                    onInput={(e) => state.setForm('chatModel', e.currentTarget.value)}
                    placeholder="Use default model"
                    class={controlClass()}
                    disabled={state.saving()}
                  />
                }
              >
                <select
                  value={state.form.chatModel}
                  onChange={(e) => state.setForm('chatModel', e.currentTarget.value)}
                  class={controlClass()}
                  disabled={state.saving()}
                >
                  <option value="">
                    Use default ({state.form.model?.split(':').pop() || 'not set'})
                  </option>
                  <For each={groupedModels()}>
                    {([provider, models]) => (
                      <optgroup label={getAIProviderDisplayName(provider) || provider}>
                        <For each={models}>
                          {(model) => (
                            <option value={model.id}>{model.name || model.id.split(':').pop()}</option>
                          )}
                        </For>
                      </optgroup>
                    )}
                  </For>
                </select>
              </Show>
            </div>
            <div>
              <label class="block text-xs font-medium text-muted mb-0.5">Patrol Model (Background)</label>
              <p class="text-[11px] text-muted mb-1">
                Runs frequently for detection — a smaller, cheaper model keeps costs low.
              </p>
              <Show
                when={state.availableModels().length > 0}
                fallback={
                  <input
                    type="text"
                    value={state.form.patrolModel}
                    onInput={(e) => state.setForm('patrolModel', e.currentTarget.value)}
                    placeholder="Use default model"
                    class={controlClass()}
                    disabled={state.saving()}
                  />
                }
              >
                <select
                  value={state.form.patrolModel}
                  onChange={(e) => state.setForm('patrolModel', e.currentTarget.value)}
                  class={controlClass()}
                  disabled={state.saving()}
                >
                  <option value="">
                    Use default ({state.form.model?.split(':').pop() || 'not set'})
                  </option>
                  <For each={groupedModels()}>
                    {([provider, models]) => (
                      <optgroup label={getAIProviderDisplayName(provider) || provider}>
                        <For each={models}>
                          {(model) => (
                            <option value={model.id}>{model.name || model.id.split(':').pop()}</option>
                          )}
                        </For>
                      </optgroup>
                    )}
                  </For>
                </select>
              </Show>
            </div>
          </div>
        </Show>
      </div>

      <div class={formField}>
        <AIProviderConfigurationSection
          settings={state.settings}
          form={state.form}
          setForm={state.setForm}
          expandedProviders={state.expandedProviders}
          setExpandedProviders={state.setExpandedProviders}
          providerHealth={state.providerHealth}
          preflightRunning={state.preflightRunning}
          preflightLastCheckedAt={state.preflightLastCheckedAt}
          providerIssueCount={state.providerIssueCount}
          testingProvider={state.testingProvider}
          providerTestResult={state.providerTestResult}
          saving={state.saving}
          runProviderPreflight={() => state.runProviderPreflight()}
          handleTestProvider={state.handleTestProvider}
          handleClearProvider={state.handleClearProvider}
        />
      </div>
    </>
  );
};
