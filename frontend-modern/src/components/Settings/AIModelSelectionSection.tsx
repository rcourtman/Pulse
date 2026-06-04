import { Component, Show } from 'solid-js';
import { AIProviderConfigurationSection } from '@/components/Settings/AIProviderConfigurationSection';
import { isModelProviderConfigured } from '@/components/Settings/aiSettingsModel';
import type { AISettingsState } from '@/components/Settings/useAISettingsState';
import { AIModelPicker } from '@/components/shared/AIModelPicker';
import { formField, labelClass, controlClass } from '@/components/shared/Form';
import { AI_SETTINGS_MODEL_OVERRIDES_TITLE } from '@/utils/aiSettingsPresentation';
import {
  formatAIModelRouteLabel,
  getAIProviderDisplayName,
  getProviderFromModelId,
} from '@/utils/aiProviderPresentation';

interface AIModelSelectionSectionProps {
  state: AISettingsState;
}

const stripModelProvider = (modelId: string) => {
  const trimmed = modelId.trim();
  const colon = trimmed.indexOf(':');
  return colon === -1 ? trimmed : trimmed.slice(colon + 1);
};

const PatrolPreflightControl: Component<{ state: AISettingsState }> = (controlProps) => {
  const { state } = controlProps;
  const result = state.patrolPreflightResult;

  // The cached result may be for a model the operator already moved away
  // from in the form (e.g. they changed the dropdown but haven't clicked
  // Verify Patrol yet). When that happens, surface a hint so the green
  // "verified" badge doesn't silently mislead. The backend reads the
  // form's pending patrolModel on Verify Patrol click, so refreshing
  // resolves the staleness.
  const pendingFormModel = () => stripModelProvider(state.form.patrolModel || '');
  const cachedResultModel = () => result()?.model?.trim() || '';
  const isStaleAgainstFormSelection = () => {
    const pending = pendingFormModel();
    const cached = cachedResultModel();
    return pending !== '' && cached !== '' && pending !== cached;
  };

  const tone = () => {
    const r = result();
    if (!r) return 'idle';
    if (isStaleAgainstFormSelection()) return 'warning';
    if (r.success) return 'success';
    if (r.cause === 'model_tool_support_unverified') return 'warning';
    return 'error';
  };

  const toneClasses = () => {
    switch (tone()) {
      case 'success':
        return 'border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-900 text-green-700 dark:text-green-300';
      case 'warning':
        return 'border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900 text-amber-700 dark:text-amber-300';
      case 'error':
        return 'border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900 text-red-700 dark:text-red-300';
      default:
        return '';
    }
  };

  const headline = () => {
    const r = result();
    if (!r) return '';
    if (isStaleAgainstFormSelection()) {
      return `Verified result is for ${cachedResultModel()}, your current selection is ${pendingFormModel()}`;
    }
    if (r.success) {
      return 'Tool calling verified';
    }
    if (r.cause === 'model_tool_support_unverified') {
      return 'Provider accepted the request but the model did not call the tool';
    }
    return r.message || 'Patrol preflight failed';
  };

  const detail = () => {
    const r = result();
    if (!r) return '';
    if (isStaleAgainstFormSelection()) {
      return 'Click Verify Patrol to test the pending selection.';
    }
    return r.summary || r.message || '';
  };

  const formatDuration = (ms: number) => {
    if (ms < 1000) return `${ms}ms`;
    return `${(ms / 1000).toFixed(1)}s`;
  };

  const formatRecordedAt = (unix?: number) => {
    if (!unix) return '';
    const ageMs = Date.now() - unix * 1000;
    if (ageMs < 0 || !Number.isFinite(ageMs)) return '';
    if (ageMs < 60_000) return 'just now';
    const minutes = Math.floor(ageMs / 60_000);
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours}h ago`;
    const days = Math.floor(hours / 24);
    return `${days}d ago`;
  };

  return (
    <div class="mt-2 flex flex-col gap-2">
      <div class="flex items-center justify-between gap-2">
        <p class="text-[11px] text-muted">
          Verify that this Patrol model can actually call tools — different from{' '}
          <span class="font-medium">Run Preflight</span>, which only checks each provider's
          connection.
        </p>
        <button
          type="button"
          onClick={() => void state.runPatrolToolPreflight()}
          disabled={state.patrolPreflightRunning() || state.saving()}
          class="inline-flex min-h-9 items-center rounded-md px-3 py-1.5 text-sm bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 hover:bg-blue-200 dark:hover:bg-blue-800 disabled:opacity-50 whitespace-nowrap"
        >
          {state.patrolPreflightRunning() ? 'Verifying...' : 'Verify Patrol'}
        </button>
      </div>
      <Show when={result()}>
        {(r) => (
          <div class={`rounded border px-3 py-2 ${toneClasses()}`}>
            <div class="flex items-baseline justify-between gap-2">
              <p class="text-xs font-medium">{headline()}</p>
              <Show when={r().duration_ms > 0}>
                <span class="text-[11px] opacity-80">{formatDuration(r().duration_ms)}</span>
              </Show>
            </div>
            <Show when={detail()}>
              <p class="text-[11px] mt-1 opacity-90">{detail()}</p>
            </Show>
            <Show when={r().recommendation}>
              <p class="text-[11px] mt-1 opacity-90">{r().recommendation}</p>
            </Show>
            <Show when={r().provider || r().model || r().recorded_at_unix}>
              <p class="text-[11px] mt-1 opacity-70">
                {r().provider}
                {r().provider && r().model ? ' · ' : ''}
                {r().model}
                <Show when={r().recorded_at_unix}>
                  {(r().provider || r().model ? ' · ' : '') +
                    'last verified ' +
                    formatRecordedAt(r().recorded_at_unix)}
                </Show>
              </p>
            </Show>
          </div>
        )}
      </Show>
    </div>
  );
};

export const AIModelSelectionSection: Component<AIModelSelectionSectionProps> = (props) => {
  const { state } = props;
  const modelLabel = (modelId: string) => {
    const trimmed = modelId.trim();
    if (!trimmed) {
      return '';
    }
    const match = state.availableModels().find((model) => model.id === trimmed);
    return formatAIModelRouteLabel(match || trimmed);
  };
  const selectableModels = (selectedModel: string) => {
    const selected = selectedModel.trim();
    return state
      .availableModels()
      .filter(
        (model) => isModelProviderConfigured(model.id, state.settings()) || model.id === selected,
      );
  };
  const sharedModelOptions = () => selectableModels(state.form.model);
  const chatModelOptions = () => selectableModels(state.form.chatModel);
  const patrolModelOptions = () => selectableModels(state.form.patrolModel);
  const discoveryModelOptions = () => selectableModels(state.form.discoveryModel);
  const pickerButtonClass = () =>
    `${controlClass()} flex items-center gap-2 justify-between text-left disabled:cursor-not-allowed disabled:opacity-60`;
  const pickerLabelClass = 'min-w-0 flex-1 truncate text-left font-normal';
  const pickerDropdownClass = 'w-[calc(100vw-2rem)] max-w-xl';
  const sharedDefaultDescription = () =>
    state.form.model ? `Currently ${modelLabel(state.form.model)}` : 'No shared default model set';

  return (
    <>
      <div class={formField}>
        <div class="flex items-center justify-between mb-1">
          <label class={labelClass()}>
            Shared Default Model
            {state.modelsLoading() && <span class="ml-2 text-xs text-muted">(loading...)</span>}
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
          when={sharedModelOptions().length > 0}
          fallback={
            <input
              type="text"
              value={state.form.model}
              onInput={(e) => state.setForm('model', e.currentTarget.value)}
              placeholder="Configure a provider below to see available models"
              aria-label="Default model identifier"
              class={controlClass()}
              disabled={state.saving()}
            />
          }
        >
          <AIModelPicker
            models={sharedModelOptions()}
            selectedModel={state.form.model}
            onModelSelect={(modelId) => state.setForm('model', modelId)}
            emptySelectionLabel="Select a model..."
            title="Select shared default model"
            searchPlaceholder="Search configured provider models"
            customModelDescription="Custom provider:model ID"
            disabled={state.saving()}
            isLoading={state.modelsLoading()}
            error={state.modelsError()}
            onRefresh={state.loadModels}
            align="left"
            buttonClass={pickerButtonClass()}
            buttonLabelClass={pickerLabelClass}
            dropdownClass={pickerDropdownClass}
          />
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
        <Show
          when={state.form.model && !isModelProviderConfigured(state.form.model, state.settings())}
        >
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
        <p class="text-[11px] text-muted mt-1">
          Used by Pulse Assistant, Patrol, and Discovery unless you set a section-specific override
          below.
        </p>
      </div>

      <div class="border border-border rounded-md overflow-hidden">
        <button
          type="button"
          class="w-full min-h-10 sm:min-h-9 px-3 py-2.5 flex items-center justify-between bg-surface-alt hover:bg-surface-hover transition-colors text-left"
          onClick={() => state.setShowAdvancedModels(!state.showAdvancedModels())}
        >
          <div class="flex items-center gap-2">
            <svg
              class="w-4 h-4 text-slate-500"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4"
              />
            </svg>
            <span class="text-sm font-medium text-base-content">
              {AI_SETTINGS_MODEL_OVERRIDES_TITLE}
            </span>
            <Show
              when={state.form.chatModel || state.form.patrolModel || state.form.discoveryModel}
            >
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
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M19 9l-7 7-7-7"
            />
          </svg>
        </button>
        <Show when={state.showAdvancedModels()}>
          <div class="px-3 py-3 bg-surface border-t border-border space-y-3">
            <p class="text-xs text-muted">
              Override the shared default for Pulse Assistant, Patrol, or Discovery. Leave empty to
              use the shared default model.
            </p>
            <div>
              <label class="block text-xs font-medium text-muted mb-0.5">
                Pulse Assistant Model
              </label>
              <p class="text-[11px] text-muted mb-1">
                Used for live chat and approved fix execution — a more capable model is recommended.
              </p>
              <Show
                when={chatModelOptions().length > 0}
                fallback={
                  <input
                    type="text"
                    value={state.form.chatModel}
                    onInput={(e) => state.setForm('chatModel', e.currentTarget.value)}
                    placeholder="Use shared default model"
                    aria-label="Pulse Assistant model identifier"
                    class={controlClass()}
                    disabled={state.saving()}
                  />
                }
              >
                <AIModelPicker
                  models={chatModelOptions()}
                  selectedModel={state.form.chatModel}
                  onModelSelect={(modelId) => state.setForm('chatModel', modelId)}
                  defaultOption={{
                    label: 'Use shared default',
                    description: sharedDefaultDescription(),
                  }}
                  emptySelectionLabel="Use shared default"
                  title="Select Pulse Assistant model"
                  searchPlaceholder="Search configured provider models"
                  customModelDescription="Custom provider:model ID"
                  disabled={state.saving()}
                  align="left"
                  buttonClass={pickerButtonClass()}
                  buttonLabelClass={pickerLabelClass}
                  dropdownClass={pickerDropdownClass}
                />
              </Show>
            </div>
            <div>
              <label class="block text-xs font-medium text-muted mb-0.5">
                Patrol Verification Model
              </label>
              <p class="text-[11px] text-muted mb-1">
                Used for recurring verification and finding generation — a smaller, cheaper model
                keeps costs low.
              </p>
              <Show
                when={patrolModelOptions().length > 0}
                fallback={
                  <input
                    type="text"
                    value={state.form.patrolModel}
                    onInput={(e) => state.setForm('patrolModel', e.currentTarget.value)}
                    placeholder="Use shared default model"
                    aria-label="Patrol Verification model identifier"
                    class={controlClass()}
                    disabled={state.saving()}
                  />
                }
              >
                <AIModelPicker
                  models={patrolModelOptions()}
                  selectedModel={state.form.patrolModel}
                  onModelSelect={(modelId) => state.setForm('patrolModel', modelId)}
                  defaultOption={{
                    label: 'Use shared default',
                    description: sharedDefaultDescription(),
                  }}
                  emptySelectionLabel="Use shared default"
                  title="Select Patrol verification model"
                  searchPlaceholder="Search configured provider models"
                  customModelDescription="Custom provider:model ID"
                  disabled={state.saving()}
                  align="left"
                  buttonClass={pickerButtonClass()}
                  buttonLabelClass={pickerLabelClass}
                  dropdownClass={pickerDropdownClass}
                />
              </Show>
              <PatrolPreflightControl state={state} />
            </div>
            <div>
              <label class="block text-xs font-medium text-muted mb-0.5">Discovery Model</label>
              <p class="text-[11px] text-muted mb-1">
                Used for one-shot service identification on a single resource — a cheaper model like
                Haiku is usually sufficient.
              </p>
              <Show
                when={discoveryModelOptions().length > 0}
                fallback={
                  <input
                    type="text"
                    value={state.form.discoveryModel}
                    onInput={(e) => state.setForm('discoveryModel', e.currentTarget.value)}
                    placeholder="Use shared default model"
                    aria-label="Discovery model identifier"
                    class={controlClass()}
                    disabled={state.saving()}
                  />
                }
              >
                <AIModelPicker
                  models={discoveryModelOptions()}
                  selectedModel={state.form.discoveryModel}
                  onModelSelect={(modelId) => state.setForm('discoveryModel', modelId)}
                  defaultOption={{
                    label: 'Use shared default',
                    description: sharedDefaultDescription(),
                  }}
                  emptySelectionLabel="Use shared default"
                  title="Select Discovery model"
                  searchPlaceholder="Search configured provider models"
                  customModelDescription="Custom provider:model ID"
                  disabled={state.saving()}
                  align="left"
                  buttonClass={pickerButtonClass()}
                  buttonLabelClass={pickerLabelClass}
                  dropdownClass={pickerDropdownClass}
                />
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
