import { A } from '@solidjs/router';
import { Component, For, Show, createMemo } from 'solid-js';
import { AIProviderConfigurationSection } from '@/components/Settings/AIProviderConfigurationSection';
import { isModelProviderConfigured } from '@/components/Settings/aiSettingsModel';
import { settingsTabPath } from '@/components/Settings/settingsNavigationModel';
import type { AISettingsState } from '@/components/Settings/useAISettingsState';
import type { PatrolModelReadinessSnapshot } from '@/types/ai';
import { AIModelPicker } from '@/components/shared/AIModelPicker';
import type { AIModelPickerAnnotation } from '@/components/shared/AIModelPicker';
import {
  getPatrolCostPresentation,
  getPatrolGuidedModelIds,
  resolvePatrolModelGuidance,
  type PatrolCostTone,
} from '@/utils/aiPatrolCostPresentation';
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
    description:
      'Used for model-backed service identification and scheduled context refreshes. Follows the Patrol model unless set.',
    ariaLabel: 'Service context model identifier',
    title: 'Select service context model',
  },
};

const stripModelProvider = (modelId: string) => {
  const trimmed = modelId.trim();
  const colon = trimmed.indexOf(':');
  return colon === -1 ? trimmed : trimmed.slice(colon + 1);
};

export type PatrolReadinessBannerTone = 'idle' | 'success' | 'warning' | 'neutral' | 'error';

// An interrupted run — an operator cancel, or a proxy cutting a slow local
// evaluation — measured nothing, so it is not a verdict on the model. The
// backend already reports it as not assessed rather than a provider fault
// (#1640); the banner has to match, because rendering it in the failure
// treatment is the same blame-the-model presentation with a different coat of
// paint.
export const isPatrolReadinessUnassessed = (result: PatrolModelReadinessSnapshot) =>
  result.status === 'not_assessed' || result.cause === 'interrupted';

export const patrolReadinessBannerTone = (
  result: PatrolModelReadinessSnapshot | null | undefined,
  isStale: boolean,
): PatrolReadinessBannerTone => {
  if (!result) return 'idle';
  if (isStale) return 'warning';
  if (isPatrolReadinessUnassessed(result)) return 'neutral';
  if (result.status === 'pass') return 'success';
  if (result.transport_healthy && !result.patrol_capable) return 'warning';
  if (result.status === 'warning') return 'warning';
  return 'error';
};

export const patrolReadinessBannerHeadline = (
  result: PatrolModelReadinessSnapshot | null | undefined,
  options: { isStale: boolean; pendingModel: string; cachedModel: string },
): string => {
  if (!result) return '';
  if (options.isStale) {
    return `Evaluation result is for ${options.cachedModel}, your current selection is ${options.pendingModel}`;
  }
  if (isPatrolReadinessUnassessed(result)) {
    return result.cause === 'interrupted'
      ? 'Patrol model check did not complete'
      : 'Patrol model not assessed';
  }
  if (result.max_verified_mode === 'approval') return 'Verified for Watch only and Ask first';
  if (result.max_verified_mode === 'monitor') return 'Verified for Watch only';
  if (result.transport_healthy && !result.patrol_capable)
    return 'Provider connected. Patrol capability not verified';
  if (result.status === 'warning') return 'Patrol model needs attention';
  return 'Patrol model not verified';
};

export const PatrolModelReadinessControl: Component<{ state: AISettingsState }> = (
  controlProps,
) => {
  const { state } = controlProps;
  const result = state.patrolModelReadinessResult;

  // A persisted result is valid only for the model that was evaluated. The
  // backend additionally invalidates on endpoint and timeout changes.
  const pendingFormModel = () => stripModelProvider(state.form.patrolModel || '');
  const cachedResultModel = () => result()?.model?.trim() || '';
  const isStaleAgainstFormSelection = () => {
    const pending = pendingFormModel();
    const cached = cachedResultModel();
    return pending !== '' && cached !== '' && pending !== cached;
  };

  const tone = () => patrolReadinessBannerTone(result(), isStaleAgainstFormSelection());

  const toneClasses = () => {
    switch (tone()) {
      case 'success':
        return 'border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-900 text-green-700 dark:text-green-300';
      case 'warning':
        return 'border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900 text-amber-700 dark:text-amber-300';
      case 'neutral':
        return 'border-border bg-surface-alt text-base-content';
      case 'error':
        return 'border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900 text-red-700 dark:text-red-300';
      default:
        return '';
    }
  };

  const headline = () =>
    patrolReadinessBannerHeadline(result(), {
      isStale: isStaleAgainstFormSelection(),
      pendingModel: pendingFormModel(),
      cachedModel: cachedResultModel(),
    });

  const detail = () => {
    const r = result();
    if (!r) return '';
    if (isStaleAgainstFormSelection()) {
      return 'Click Check Patrol model to test the pending selection.';
    }
    if (isPatrolReadinessUnassessed(r) && r.cause === 'interrupted') {
      // The backend summary already explains the interruption without blaming
      // the model; append the retry prompt rather than a failure recommendation.
      return r.summary
        ? `${r.summary} Run the check again when you are ready.`
        : 'The check was interrupted before it finished. Run it again when you are ready.';
    }
    return r.summary || '';
  };

  const dimensionRows = () => {
    const r = result();
    if (!r) return [];
    return [
      { label: 'Connectivity', value: r.dimensions.connectivity },
      { label: 'Tool protocol', value: r.dimensions.tool_protocol },
      { label: 'Context quality', value: r.dimensions.context_quality },
      { label: 'Latency', value: r.dimensions.latency },
    ];
  };

  const modeRows = () => {
    const r = result();
    if (!r) return [];
    return [
      { label: 'Watch only', value: r.modes.monitor },
      { label: 'Ask first', value: r.modes.approval },
      { label: 'Safe auto-fix', value: r.modes.assisted },
      { label: 'Autopilot', value: r.modes.full },
    ];
  };

  const statusLabel = (status: string) => {
    switch (status) {
      case 'pass':
      case 'verified':
        return 'Passed';
      case 'warning':
        return 'Caution';
      case 'fail':
      case 'not_suitable':
        return 'Failed';
      default:
        return 'Not assessed';
    }
  };

  const modeStatusLabel = (status: string) => {
    switch (status) {
      case 'verified':
        return 'Verified';
      case 'warning':
        return 'Caution';
      case 'not_suitable':
        return 'Not suitable';
      default:
        return 'Not assessed';
    }
  };

  const readinessStatusTextClass = (status: string) => {
    switch (status) {
      case 'pass':
      case 'verified':
        return 'text-green-700 dark:text-green-300';
      case 'warning':
        return 'text-amber-700 dark:text-amber-300';
      case 'fail':
      case 'not_suitable':
        return 'text-red-700 dark:text-red-300';
      default:
        return 'text-muted';
    }
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
          Evaluate connectivity, streaming tools, context handling, latency, and suitable Patrol
          modes. Uses synthetic data only and may take up to two minutes.
        </p>
        <button
          type="button"
          onClick={() =>
            state.patrolModelReadinessRunning()
              ? state.cancelPatrolModelReadinessAdvisor()
              : void state.runPatrolModelReadinessAdvisor()
          }
          disabled={state.saving()}
          class={`inline-flex min-h-9 items-center rounded-md px-3 py-1.5 text-sm disabled:opacity-50 whitespace-nowrap ${
            state.patrolModelReadinessRunning()
              ? 'bg-amber-100 dark:bg-amber-900 text-amber-700 dark:text-amber-300 hover:bg-amber-200 dark:hover:bg-amber-800'
              : 'bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 hover:bg-blue-200 dark:hover:bg-blue-800'
          }`}
        >
          {state.patrolModelReadinessRunning() ? 'Cancel check' : 'Check Patrol model'}
        </button>
      </div>
      <Show when={state.patrolModelReadinessRunning()}>
        <p class="text-[11px] text-muted" role="status">
          Running synthetic streaming scenarios. Pulse will not call infrastructure tools.
        </p>
      </Show>
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
            <div class="mt-2 grid gap-1 sm:grid-cols-2">
              <For each={dimensionRows()}>
                {(dimension) => (
                  <div class="rounded bg-white/40 px-2 py-1 dark:bg-black/10">
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-[11px]">{dimension.label}</span>
                      <span
                        class={`text-[11px] font-medium ${readinessStatusTextClass(dimension.value.status)}`}
                      >
                        {dimension.value.attempts
                          ? `${dimension.value.passed ?? 0}/${dimension.value.attempts} · `
                          : ''}
                        {statusLabel(dimension.value.status)}
                      </span>
                    </div>
                    <p class="mt-0.5 text-[10px] opacity-75">{dimension.value.summary}</p>
                  </div>
                )}
              </For>
            </div>
            <div class="mt-2 border-t border-current/15 pt-2">
              <p class="text-[11px] font-medium mb-1">Autonomy suitability</p>
              <div class="grid gap-1 sm:grid-cols-2">
                <For each={modeRows()}>
                  {(mode) => (
                    <div>
                      <div class="flex items-center justify-between gap-2">
                        <span class="text-[11px]">{mode.label}</span>
                        <span
                          class={`text-[11px] font-medium ${readinessStatusTextClass(mode.value.status)}`}
                        >
                          {modeStatusLabel(mode.value.status)}
                        </span>
                      </div>
                      <p class="text-[10px] opacity-75">{mode.value.summary}</p>
                    </div>
                  )}
                </For>
              </div>
            </div>
            <Show when={r().recommendation}>
              <p class="text-[11px] mt-1 opacity-90">{r().recommendation}</p>
            </Show>
            <Show when={(r().details?.length ?? 0) > 0}>
              <div class="mt-1">
                <p class="text-[11px] font-medium">Evaluation detail</p>
                <ul class="mt-0.5 list-disc pl-4 text-[10px] opacity-80">
                  <For each={r().details}>{(item) => <li>{item}</li>}</For>
                </ul>
              </div>
            </Show>
            <Show when={r().metadata?.context_window || r().metadata?.quantization_level}>
              <p class="text-[11px] mt-1 opacity-70">
                <Show when={r().metadata?.context_window}>
                  Context {r().metadata?.context_window?.toLocaleString()} tokens
                </Show>
                <Show when={r().metadata?.context_window && r().metadata?.quantization_level}>
                  {' · '}
                </Show>
                <Show when={r().metadata?.quantization_level}>
                  {r().metadata?.quantization_level}
                </Show>
              </p>
            </Show>
            <Show when={r().provider || r().model || r().recorded_at_unix}>
              <p class="text-[11px] mt-1 opacity-70">
                {r().provider}
                {r().provider && r().model ? ' · ' : ''}
                {r().model}
                <Show when={r().recorded_at_unix}>
                  {(r().provider || r().model ? ' · ' : '') +
                    'last evaluated ' +
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

const patrolCostToneClasses = (tone: PatrolCostTone) => {
  switch (tone) {
    case 'positive':
      return 'border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-900 text-green-800 dark:text-green-200';
    case 'warning':
      return 'border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900 text-amber-800 dark:text-amber-200';
    case 'danger':
      return 'border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900 text-red-800 dark:text-red-200';
    default:
      return 'border-border bg-surface-alt text-base-content';
  }
};

const patrolCostLineToneClass = (tone: PatrolCostTone) => {
  switch (tone) {
    case 'warning':
      return 'text-amber-700 dark:text-amber-300';
    case 'danger':
      return 'text-red-700 dark:text-red-300';
    default:
      return '';
  }
};

/**
 * What Patrol will cost on the chosen model and schedule, with the assumption
 * and the 30-day spend against budget in the same place. Rendered under the
 * Patrol model picker and under the shared default when it drives Patrol.
 */
export const PatrolCostPreview: Component<{ state: AISettingsState }> = (props) => {
  const { state } = props;
  const presentation = createMemo(() => getPatrolCostPresentation(state.patrolCostPreview()));
  return (
    <Show when={presentation()}>
      {(view) => (
        <div
          class={`mt-2 rounded border px-3 py-2 ${patrolCostToneClasses(view().tone)}`}
          data-testid="patrol-cost-preview"
          role="status"
        >
          <div class="flex items-baseline justify-between gap-2">
            <p class="text-xs font-medium">
              <span class="mr-1 text-[10px] font-semibold uppercase opacity-70">Estimate</span>{' '}
              {view().headline}
            </p>
            <Show when={state.patrolCostPreviewLoading()}>
              <span class="text-[10px] opacity-70">updating…</span>
            </Show>
          </div>
          <Show when={view().detail}>
            <p class="mt-1 text-[11px] opacity-90">{view().detail}</p>
          </Show>
          <Show when={view().assumption}>
            <p class="mt-1 text-[11px] opacity-75">{view().assumption}</p>
          </Show>
          <p
            class={`mt-1 text-[11px] ${patrolCostLineToneClass(view().budgetTone) || 'opacity-90'}`}
          >
            {view().budget}
          </p>
          <Show when={view().schedule}>
            <p class="mt-1 text-[11px] opacity-90">{view().schedule}</p>
          </Show>
          <Show when={state.patrolIntervalAutoAdjust()}>
            {(adjust) => (
              <p class="mt-1 text-[11px] font-medium" data-testid="patrol-schedule-auto-adjust">
                {adjust().sentence}
              </p>
            )}
          </Show>
        </div>
      )}
    </Show>
  );
};

const usePatrolModelGuidance = (
  state: AISettingsState,
  models: () => { id: string; provider?: string }[],
) => {
  const annotations = createMemo(() =>
    resolvePatrolModelGuidance(models(), state.patrolModelGuidance()),
  );
  const annotationRecord = createMemo<Record<string, AIModelPickerAnnotation>>(() =>
    Object.fromEntries(
      Array.from(annotations().entries()).map(([id, annotation]) => [
        id,
        { badge: annotation.badge, note: annotation.note, tone: annotation.tone },
      ]),
    ),
  );
  const sections = createMemo(() => {
    const ids = getPatrolGuidedModelIds(annotations());
    return ids.length > 0 ? [{ title: 'Suggested for Patrol', modelIds: ids }] : [];
  });
  return { annotations, annotationRecord, sections };
};

const SelectedModelGuidanceNote: Component<{
  annotation: () => AIModelPickerAnnotation | undefined;
}> = (props) => (
  <Show when={props.annotation()}>
    {(annotation) => (
      <p
        class={`mt-1 text-xs ${
          annotation().tone === 'warning' ? 'text-amber-600 dark:text-amber-400' : 'text-muted'
        }`}
      >
        <span class="font-medium">{annotation().badge}:</span> {annotation().note}
      </p>
    )}
  </Show>
);

export const AIModelOverrideField: Component<{
  state: AISettingsState;
  kind: AIModelOverrideKind;
  includePatrolReadiness?: boolean;
}> = (props) => {
  const { state } = props;
  const config = () => MODEL_OVERRIDE_CONFIG[props.kind];
  const selectedModel = () => state.form[config().formKey];
  const setSelectedModel = (modelId: string) =>
    state.handleModelSelection(config().formKey, modelId);
  const guidesPatrol = () => props.kind === 'patrol';
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
  const guidance = usePatrolModelGuidance(state, () => (guidesPatrol() ? selectableModels() : []));
  const selectedAnnotation = () =>
    guidesPatrol() ? guidance.annotationRecord()[selectedModel().trim()] : undefined;

  return (
    <div class={formField}>
      <span class="block text-xs font-medium text-muted mb-0.5">{config().label}</span>
      <p class="text-[11px] text-muted mb-1">{config().description}</p>
      <Show
        when={selectableModels().length > 0}
        fallback={
          <Show
            when={state.hasConfiguredProvider()}
            fallback={
              <p class="rounded-md border border-border bg-surface-alt px-3 py-2 text-xs text-base-content">
                No AI provider is configured yet. Add an API key or an Ollama server on{' '}
                <A
                  href={settingsTabPath('system-ai')}
                  class="font-medium text-blue-600 underline dark:text-blue-400"
                >
                  Provider & Models
                </A>
                , then pick a model here.
              </p>
            }
          >
            <input
              type="text"
              value={selectedModel()}
              onInput={(e) => setSelectedModel(e.currentTarget.value)}
              placeholder="Use shared default model"
              aria-label={config().ariaLabel}
              autocomplete="off"
              class={controlClass()}
              disabled={state.saving()}
            />
          </Show>
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
          modelSections={guidance.sections()}
          modelAnnotations={guidance.annotationRecord()}
        />
      </Show>
      <SelectedModelGuidanceNote annotation={selectedAnnotation} />
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
      <Show when={guidesPatrol()}>
        <PatrolCostPreview state={state} />
      </Show>
      <Show when={props.includePatrolReadiness}>
        <PatrolModelReadinessControl state={state} />
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
  const sharedGuidance = usePatrolModelGuidance(state, sharedModelOptions);
  const sharedDrivesPatrol = () => !state.form.patrolModel.trim();
  const sharedSelectedAnnotation = () =>
    sharedDrivesPatrol() ? sharedGuidance.annotationRecord()[state.form.model.trim()] : undefined;
  const pickerButtonClass = () =>
    `${controlClass()} flex items-center gap-2 justify-between text-left disabled:cursor-not-allowed disabled:opacity-60`;
  const pickerLabelClass = 'min-w-0 flex-1 truncate text-left font-normal';
  const pickerDropdownClass = 'w-[calc(100vw-2rem)] max-w-xl';

  return (
    <>
      <div class={formField}>
        <div class="flex items-center justify-between mb-1">
          <span class={labelClass()}>
            Shared Default Model
            {state.modelsLoading() && <span class="ml-2 text-xs text-muted">(loading...)</span>}
          </span>
          <button
            type="button"
            onClick={state.loadModels}
            disabled={state.modelsLoading()}
            class="inline-flex min-h-11 sm:min-h-9 items-center gap-1 rounded-md px-2 py-1.5 text-sm text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 disabled:opacity-50"
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
              autocomplete="off"
              class={controlClass()}
              disabled={state.saving()}
            />
          }
        >
          <AIModelPicker
            models={sharedModelOptions()}
            selectedModel={state.form.model}
            onModelSelect={(modelId) => state.handleModelSelection('model', modelId)}
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
            modelSections={sharedGuidance.sections()}
            modelAnnotations={sharedGuidance.annotationRecord()}
          />
        </Show>
        <SelectedModelGuidanceNote annotation={sharedSelectedAnnotation} />
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
          Used by Pulse Assistant and Patrol unless you set a section-specific override. Service
          context discovery follows the Patrol model when one is set.
        </p>
        <Show when={sharedDrivesPatrol()}>
          <PatrolCostPreview state={state} />
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
