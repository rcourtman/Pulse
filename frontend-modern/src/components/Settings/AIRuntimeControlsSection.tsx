import { Component, Show } from 'solid-js';
import type { AIControlLevel } from '@/utils/aiControlLevelPresentation';
import type { AISettingsState } from '@/components/Settings/useAISettingsState';
import { FormSelect } from '@/components/shared/FormSelect';
import {
  getAIControlLevelBadgeClass,
  getAIControlLevelDescription,
  getAIControlLevelPanelClass,
} from '@/utils/aiControlLevelPresentation';
import { AI_SETTINGS_ASSISTANT_PERMISSIONS_TITLE } from '@/utils/aiSettingsPresentation';

interface AIRuntimeControlsSectionProps {
  state: AISettingsState;
}

export const AIRuntimeControlsSection: Component<AIRuntimeControlsSectionProps> = (props) => {
  const { state } = props;
  return (
    <>
      <AIProviderRuntimeControlsSection state={state} />
      <AIAssistantCommandAccessSection state={state} />
    </>
  );
};

export const AIProviderRuntimeControlsSection: Component<AIRuntimeControlsSectionProps> = (
  props,
) => {
  const { state } = props;

  return (
    <div class="grid gap-3 lg:grid-cols-2">
      <div class="flex flex-col gap-2 p-3 rounded-md border border-border bg-surface-alt">
        <div class="flex flex-wrap items-center gap-3">
          <svg
            class="w-4 h-4 text-muted shrink-0"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          <label for="ai-cost-budget" class="text-xs font-medium text-base-content">
            30-day Budget
          </label>
          <div class="relative shrink-0">
            <span class="absolute left-2 top-1/2 -translate-y-1/2 text-muted text-xs">$</span>
            <input
              id="ai-cost-budget"
              type="number"
              class="w-24 min-h-10 sm:min-h-9 pl-5 pr-2 py-2 text-sm border border-border rounded-sm bg-surface"
              value={state.form.costBudgetUSD30d}
              onInput={(e) => state.setForm('costBudgetUSD30d', e.currentTarget.value)}
              min={0}
              step={1}
              placeholder="0"
              aria-describedby="ai-cost-budget-help"
              disabled={state.saving()}
            />
          </div>
          <Show when={parseFloat(state.form.costBudgetUSD30d) > 0}>
            <span class="text-xs">
              ≈ ${(parseFloat(state.form.costBudgetUSD30d) / 30).toFixed(2)}/day
            </span>
          </Show>
        </div>
        <p id="ai-cost-budget-help" class="text-xs text-muted">
          Pulse refuses new AI calls, including Patrol, when retained estimated usage for the
          preceding 30 days reaches this budget. Set 0 or leave empty to disable this check. Missing
          pricing or untracked usage can leave costs out of the check. If no pricing is known, calls
          are allowed. This is not a provider spending cap.
        </p>
      </div>

      <div class="flex flex-col gap-1">
        <div class="flex items-center gap-3 p-3 rounded-md border border-border bg-surface-alt">
          <svg
            class="w-4 h-4 text-muted shrink-0"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          <label for="ai-request-timeout" class="text-xs font-medium text-base-content">
            Request Timeout
          </label>
          <input
            id="ai-request-timeout"
            type="number"
            class="w-20 min-h-10 sm:min-h-9 px-2 py-2 text-sm border border-border rounded-sm bg-surface"
            value={state.form.requestTimeoutSeconds}
            onInput={(e) => {
              const value = parseInt(e.currentTarget.value, 10);
              if (!isNaN(value) && value > 0) {
                state.setForm('requestTimeoutSeconds', value);
              }
            }}
            min={30}
            max={3600}
            step={30}
            disabled={state.saving()}
          />
          <span class="text-xs">seconds</span>
          <Show when={state.form.requestTimeoutSeconds !== 300}>
            <span class="text-[10px] text-blue-600 dark:text-blue-400">Custom</span>
          </Show>
          <Show when={state.form.requestTimeoutSeconds === 300}>
            <span class="text-[10px] text-muted">default</span>
          </Show>
        </div>
        <p class="text-[10px] text-muted ml-1">
          Applies to chat, patrol, and discovery AI calls. Increase for slower Ollama hardware
          (default: 300s / 5 min)
        </p>
      </div>
    </div>
  );
};

export const AIAssistantCommandAccessSection: Component<AIRuntimeControlsSectionProps> = (
  props,
) => {
  const { state } = props;
  return (
    <>
      <div
        class={`space-y-3 p-4 rounded-md border ${getAIControlLevelPanelClass(state.form.controlLevel)}`}
      >
        <div class="flex items-center gap-2">
          <svg
            class="w-4 h-4 text-blue-600 dark:text-blue-400"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"
            />
          </svg>
          <span class="text-sm font-medium text-base-content">
            {AI_SETTINGS_ASSISTANT_PERMISSIONS_TITLE}
          </span>
          <Show when={state.form.controlLevel !== 'read_only'}>
            <span
              class={`px-1.5 py-0.5 text-[10px] font-medium rounded-sm ${getAIControlLevelBadgeClass(state.form.controlLevel)}`}
            >
              Ask first
            </span>
          </Show>
        </div>
        <p class="text-[10px] text-muted">
          This sets what Assistant chat may plan. Patrol's own mode is set on the Patrol page.
        </p>

        <FormSelect
          id="ai-control-mode-select"
          label="Chat action mode"
          value={state.form.controlLevel}
          onChange={(e) => state.setForm('controlLevel', e.currentTarget.value as AIControlLevel)}
          disabled={state.saving()}
          fieldBaseClass="flex flex-col gap-2 sm:flex-row"
          fieldClass="sm:items-center sm:gap-3"
          labelClass="text-xs font-medium text-muted sm:w-28 sm:shrink-0"
          selectBaseClass="w-full min-w-0 min-h-10 px-2 py-2 text-sm border border-border rounded-sm bg-surface sm:min-h-9 sm:flex-1"
        >
          <option value="read_only">Read-only - Assistant queries and explains only</option>
          <option value="controlled">Ask first - Assistant plans actions for your review</option>
        </FormSelect>
        <p class="text-[10px] text-muted sm:ml-30">
          {getAIControlLevelDescription(state.form.controlLevel)}
        </p>
        <Show when={state.form.controlLevel !== 'read_only'}>
          <p
            class="pt-2 border-t border-blue-200 dark:border-blue-700 text-[10px] text-muted"
            data-testid="ai-never-auto-remediate-hint"
          >
            To keep Assistant away from a guest, open it, choose Manage, turn on Never
            auto-remediate, and save. Pulse then refuses typed actions on that guest.
          </p>
        </Show>
      </div>
    </>
  );
};
