import { Component, Show } from 'solid-js';
import { AIProviderConfigurationSection } from '@/components/Settings/AIProviderConfigurationSection';
import { isModelProviderConfigured } from '@/components/Settings/aiSettingsModel';
import type { AISettingsState } from '@/components/Settings/useAISettingsState';
import { AIModelPicker } from '@/components/shared/AIModelPicker';
import { formField, labelClass, controlClass } from '@/components/shared/Form';
import {
  formatAIModelRouteLabel,
  getAIProviderDisplayName,
  getProviderFromModelId,
} from '@/utils/aiProviderPresentation';

interface AIModelSelectionSectionProps {
  state: AISettingsState;
}

type AIModelOverrideKind = 'assistant' | 'patrol' | 'discovery';

const MODEL_OVERRIDE_CONFIG: Record<
  AIModelOverrideKind,
  {
    formKey: 'chatModel' | 'patrolModel' | 'discoveryModel';
    label: string;
    description: string;
    ariaLabel: string;
    title: string;
  }
> = {
  assistant: {
    formKey: 'chatModel',
    label: 'Pulse Assistant model',
    description: 'Used for chat, explanations, and review. Patrol handles infrastructure work.',
    ariaLabel: 'Pulse Assistant model identifier',
    title: 'Select Pulse Assistant model',
  },
  patrol: {
    formKey: 'patrolModel',
    label: 'Patrol model',
    description: 'Used when Patrol checks, investigates, and verifies work.',
    ariaLabel: 'Patrol model identifier',
    title: 'Select Patrol model',
  },
  discovery: {
    formKey: 'discoveryModel',
    label: 'Service context model',
    description: 'Used for model-backed service identification and scheduled context refreshes.',
    ariaLabel: 'Service context model identifier',
    title: 'Select service context model',
  },
};

const stripModelProvider = (modelId: string) => {
  const trimmed = modelId.trim();
  const colon = trimmed.indexOf(':');
  return colon === -1 ? trimmed : trimmed.slice(colon + 1);
};

export const PatrolPreflightControl: Component<{ state: AISettingsState }> = (controlProps) => {
  const { state } = controlProps;
  const result = state.patrolPreflightResult;

  // The cached result may be for a model the operator already moved away
  // from in the form (e.g. they changed the dropdown but haven't clicked
  // Check Patrol model yet). When that happens, surface a hint so the green
  // "verified" badge doesn't silently mislead. The backend reads the
  // form's pending patrolModel on Check Patrol model click, so refreshing
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
      return 'Patrol model ready';
    }
    if (r.cause === 'model_tool_support_unverified') {
      return 'Patrol model needs a real run to confirm';
    }
    return r.message || 'Patrol model check failed';
  };

  const detail = () => {
    const r = result();
    if (!r) return '';
    if (isStaleAgainstFormSelection()) {
      return 'Click Check Patrol model to test the pending selection.';
    }
    if (r.success) {
      return 'The selected model can run Patrol work on this install.';
    }
    if (r.cause === 'model_tool_support_unverified') {
      return 'Run Patrol once to confirm this model works correctly in practice.';
    }
    return r.summary || r.message || '';
  };

  const formatDuration = (ms: number) => {
    if (!Number.isFinite(ms) || ms < 0) return '-';
    if (ms < 1000) return `${Math.round(ms)}ms`;
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
          Check that this model can run Patrol work, not just connect to the provider.
        </p>
        <button
          type="button"
          onClick={() => void state.runPatrolToolPreflight()}
          disabled={state.patrolPreflightRunning() || state.saving()}
          class="inline-flex min-h-9 items-center rounded-md px-3 py-1.5 text-sm bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 hover:bg-blue-200 dark:hover:bg-blue-800 disabled:opacity-50 whitespace-nowrap"
        >
          {state.patrolPreflightRunning() ? 'Checking...' : 'Check Patrol model'}
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

export const AIModelOverrideField: Component<{
  state: AISettingsState;
  kind: AIModelOverrideKind;
  includePatrolPreflight?: boolean;
}> = (props) => {
  const { state } = props;
  const config = () => MODEL_OVERRIDE_CONFIG[props.kind];
  const selectedModel = () => state.form[config().formKey];
  const setSelectedModel = (modelId: string) => state.setForm(config().formKey, modelId);
  const modelLabel = (modelId: string) => {
    const trimmed = modelId.trim();
    if (!trimmed) {
      return '';
    }
    const match = state.availableModels().find((model) => model.id === trimmed);
    return formatAIModelRouteLabel(match || trimmed);
  };
  const selectableModels = () => {
    const selected = selectedModel().trim();
    return state
      .availableModels()
      .filter(
        (model) => isModelProviderConfigured(model.id, state.settings()) || model.id === selected,
      );
  };
  const pickerButtonClass = () =>
    `${controlClass()} flex items-center gap-2 justify-between text-left disabled:cursor-not-allowed disabled:opacity-60`;
  const sharedDefaultDescription = () =>
    state.form.model ? `Currently ${modelLabel(state.form.model)}` : 'No shared default model set';

  return (
    <div class={formField}>
      <label class="block text-xs font-medium text-muted mb-0.5">{config().label}</label>
      <p class="text-[11px] text-muted mb-1">{config().description}</p>
      <Show
        when={selectableModels().length > 0}
        fallback={
          <input
            type="text"
            value={selectedModel()}
            onInput={(e) => setSelectedModel(e.currentTarget.value)}
            placeholder="Use shared default model"
            aria-label={config().ariaLabel}
            class={controlClass()}
            disabled={state.saving()}
          />
        }
      >
        <AIModelPicker
          models={selectableModels()}
          selectedModel={selectedModel()}
          onModelSelect={setSelectedModel}
          defaultOption={{
            label: 'Use shared default',
            description: sharedDefaultDescription(),
          }}
          emptySelectionLabel="Use shared default"
          title={config().title}
          searchPlaceholder="Search configured provider models"
          customModelDescription="Custom provider:model ID"
          disabled={state.saving()}
          align="left"
          buttonClass={pickerButtonClass()}
          buttonLabelClass="min-w-0 flex-1 truncate text-left font-normal"
          dropdownClass="w-[calc(100vw-2rem)] max-w-xl"
        />
      </Show>
      <Show when={selectedModel() && !isModelProviderConfigured(selectedModel(), state.settings())}>
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
          {getAIProviderDisplayName(getProviderFromModelId(selectedModel())) ||
            getProviderFromModelId(selectedModel())}{' '}
          to be configured. Add an API key on Provider & Models or select a different model.
        </p>
      </Show>
      <Show when={props.includePatrolPreflight}>
        <PatrolPreflightControl state={state} />
      </Show>
    </div>
  );
};

export const AIModelSelectionSection: Component<AIModelSelectionSectionProps> = (props) => {
  const { state } = props;
  const selectableModels = (selectedModel: string) => {
    const selected = selectedModel.trim();
    return state
      .availableModels()
      .filter(
        (model) => isModelProviderConfigured(model.id, state.settings()) || model.id === selected,
      );
  };
  const sharedModelOptions = () => selectableModels(state.form.model);
  const pickerButtonClass = () =>
    `${controlClass()} flex items-center gap-2 justify-between text-left disabled:cursor-not-allowed disabled:opacity-60`;
  const pickerLabelClass = 'min-w-0 flex-1 truncate text-left font-normal';
  const pickerDropdownClass = 'w-[calc(100vw-2rem)] max-w-xl';

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
