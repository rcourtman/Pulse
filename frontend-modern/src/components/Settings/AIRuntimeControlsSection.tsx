import { Component, Show } from 'solid-js';
import type { AISettingsState } from '@/components/Settings/useAISettingsState';

interface AIRuntimeControlsSectionProps {
  state: AISettingsState;
}

export const AIProviderRuntimeControlsSection: Component<AIRuntimeControlsSectionProps> = (
  props,
) => {
  const { state } = props;

  return (
    <div class="grid gap-3 lg:grid-cols-2">
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
            disabled={state.saving()}
          />
        </div>
        <Show when={parseFloat(state.form.costBudgetUSD30d) > 0}>
          <span class="text-xs">
            ≈ ${(parseFloat(state.form.costBudgetUSD30d) / 30).toFixed(2)}/day
          </span>
        </Show>
        <Show when={!state.form.costBudgetUSD30d || parseFloat(state.form.costBudgetUSD30d) === 0}>
          <span class="text-[10px] text-muted">Set a budget to receive usage alerts</span>
        </Show>
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
          Applies to Patrol and every other AI call. Increase for slower Ollama hardware (default:
          300s / 5 min)
        </p>
      </div>
    </div>
  );
};
