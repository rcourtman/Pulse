import { Component, Show } from 'solid-js';
import type { AIControlLevel } from '@/utils/aiControlLevelPresentation';
import type { AISettingsState } from '@/components/Settings/useAISettingsState';
import { HelpIcon } from '@/components/shared/HelpIcon';
import { Toggle } from '@/components/shared/Toggle';
import {
  getAIControlLevelBadgeClass,
  getAIControlLevelDescription,
  getAIControlLevelPanelClass,
} from '@/utils/aiControlLevelPresentation';
import { trackUpgradeClicked } from '@/utils/upgradeMetrics';

interface AIRuntimeControlsSectionProps {
  state: AISettingsState;
}

export const AIRuntimeControlsSection: Component<AIRuntimeControlsSectionProps> = (props) => {
  const { state } = props;

  return (
    <>
      <div class="rounded-md border border-blue-200 dark:border-blue-800 overflow-hidden">
        <button
          type="button"
          class="w-full min-h-10 sm:min-h-9 px-3 py-2.5 flex items-center justify-between bg-blue-50 dark:bg-blue-900 hover:bg-blue-100 dark:hover:bg-blue-900 transition-colors text-left"
          onClick={() => state.setShowDiscoverySettings(!state.showDiscoverySettings())}
        >
          <div class="flex items-center gap-2">
            <svg class="w-4 h-4 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
              />
            </svg>
            <span class="text-sm font-medium text-base-content">Discovery Settings</span>
            <Show when={state.form.discoveryEnabled}>
              <span class="px-1.5 py-0.5 text-[10px] font-medium bg-blue-100 dark:bg-blue-800 text-blue-700 dark:text-blue-300 rounded">
                {state.form.discoveryIntervalHours > 0
                  ? `${state.form.discoveryIntervalHours}h`
                  : 'Manual'}
              </span>
            </Show>
            <Show when={!state.form.discoveryEnabled}>
              <span class="px-1.5 py-0.5 text-[10px] font-medium bg-surface-hover text-muted rounded">
                Off
              </span>
            </Show>
          </div>
          <svg
            class={`w-4 h-4 transition-transform ${state.showDiscoverySettings() ? 'rotate-180' : ''}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </button>
        <Show when={state.showDiscoverySettings()}>
          <div class="px-3 py-3 bg-surface border-t border-border space-y-3">
            <div class="flex items-center justify-between gap-2">
              <label class="text-xs font-medium text-muted flex items-center gap-1.5">
                Enable Discovery
                <HelpIcon
                  inline={{
                    title: 'What is Discovery?',
                    description:
                      'Discovery scans your VMs, containers, and container runtimes to identify what services are running (databases, web servers, etc.), their versions, and how to access them. This information helps Pulse AI give you accurate troubleshooting commands and understand your infrastructure.',
                  }}
                  size="xs"
                />
              </label>
              <Toggle
                checked={state.form.discoveryEnabled}
                onChange={(event) => state.setForm('discoveryEnabled', event.currentTarget.checked)}
                disabled={state.saving()}
              />
            </div>

            <Show when={state.form.discoveryEnabled}>
              <div class="flex flex-col gap-1">
                <div class="flex items-center gap-3">
                  <label class="text-xs font-medium text-muted w-32 flex-shrink-0">Scan Interval</label>
                  <select
                    class="flex-1 px-2 py-1 text-sm border border-border rounded bg-surface"
                    value={state.form.discoveryIntervalHours}
                    onChange={(e) =>
                      state.setForm('discoveryIntervalHours', parseInt(e.currentTarget.value, 10))
                    }
                    disabled={state.saving()}
                  >
                    <option value={0}>Manual only</option>
                    <option value={6}>Every 6 hours</option>
                    <option value={12}>Every 12 hours</option>
                    <option value={24}>Every 24 hours</option>
                    <option value={48}>Every 2 days</option>
                    <option value={168}>Every 7 days</option>
                  </select>
                </div>
                <p class="text-[10px] text-muted ml-32 pl-3">
                  {state.form.discoveryIntervalHours === 0
                    ? 'Discovery runs only when you click "Update Discovery" on a resource'
                    : 'Discovery will automatically re-scan resources at this interval'}
                </p>
              </div>
            </Show>

            <p class="text-[10px] text-muted">
              Discovery gives Pulse AI workload context, so responses can reference concrete
              services and commands instead of generic advice.
            </p>
          </div>
        </Show>
      </div>

      <div class="flex items-center gap-3 p-3 rounded-md border border-border bg-surface-alt">
        <svg class="w-4 h-4 text-muted flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
        <label class="text-xs font-medium text-base-content">30-day Budget</label>
        <div class="relative flex-shrink-0">
          <span class="absolute left-2 top-1/2 -translate-y-1/2 text-muted text-xs">$</span>
          <input
            type="number"
            class="w-24 min-h-10 sm:min-h-9 pl-5 pr-2 py-2 text-sm border border-border rounded bg-surface"
            value={state.form.costBudgetUSD30d}
            onInput={(e) => state.setForm('costBudgetUSD30d', e.currentTarget.value)}
            min={0}
            step={1}
            placeholder="0"
            disabled={state.saving()}
          />
        </div>
        <Show when={parseFloat(state.form.costBudgetUSD30d) > 0}>
          <span class="text-xs">≈ ${(parseFloat(state.form.costBudgetUSD30d) / 30).toFixed(2)}/day</span>
        </Show>
        <Show when={!state.form.costBudgetUSD30d || parseFloat(state.form.costBudgetUSD30d) === 0}>
          <span class="text-[10px] text-muted">Set a budget to receive usage alerts</span>
        </Show>
      </div>

      <div class="flex items-center gap-3 p-3 rounded-md border border-border bg-surface-alt">
        <svg class="w-4 h-4 text-muted flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
        <label class="text-xs font-medium text-base-content">Request Timeout</label>
        <input
          type="number"
          class="w-20 min-h-10 sm:min-h-9 px-2 py-2 text-sm border border-border rounded bg-surface"
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
      <p class="text-[10px] text-muted -mt-4 ml-1">
        Increase for slower Ollama hardware (default: 300s / 5 min)
      </p>

      <div class={`space-y-3 p-4 rounded-md border ${getAIControlLevelPanelClass(state.form.controlLevel)}`}>
        <div class="flex items-center gap-2">
          <svg class="w-4 h-4 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"
            />
          </svg>
          <span class="text-sm font-medium text-base-content">Pulse Permission Level</span>
          <Show when={state.form.controlLevel !== 'read_only'}>
            <span class={`px-1.5 py-0.5 text-[10px] font-medium rounded ${getAIControlLevelBadgeClass(state.form.controlLevel)}`}>
              {state.form.controlLevel}
            </span>
          </Show>
        </div>

        <div class="flex items-center gap-3">
          <label class="text-xs font-medium text-muted w-28 flex-shrink-0">Permission</label>
          <select
            value={state.form.controlLevel}
            onChange={(e) => state.setForm('controlLevel', e.currentTarget.value as AIControlLevel)}
            class="flex-1 min-h-10 sm:min-h-9 px-2 py-2 text-sm border border-border rounded bg-surface"
            disabled={state.saving()}
          >
            <option value="read_only">Read Only - Pulse Assistant can only observe</option>
            <option value="controlled">Controlled - Pulse Assistant executes with your approval</option>
            <option value="autonomous">Autonomous - Pulse Assistant executes without approval (Pro)</option>
          </select>
        </div>
        <p class="text-[10px] text-muted ml-[7.5rem]">
          {getAIControlLevelDescription(state.form.controlLevel)}
        </p>
        <Show when={state.form.controlLevel === 'autonomous'}>
          <div class="p-2 bg-amber-100 dark:bg-amber-900 rounded border border-amber-200 dark:border-amber-800 text-[10px] text-amber-800 dark:text-amber-200">
            <strong>Legal Disclaimer:</strong> Model-driven systems can hallucinate. You are
            responsible for any damage caused by autonomous actions. See{' '}
            <a
              href="https://github.com/rcourtman/Pulse/blob/main/TERMS.md"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex min-h-10 sm:min-h-9 items-center rounded px-1 underline"
            >
              Terms of Service
            </a>
            .
          </div>
        </Show>
        <Show when={state.form.controlLevel === 'autonomous' && state.autoFixLocked()}>
          <p class="text-xs text-muted">
            <a
              class="text-blue-600 dark:text-blue-400 font-medium hover:underline"
              href={state.upgradeAutofixUrl()}
              target="_blank"
              rel="noopener noreferrer"
              onClick={() => trackUpgradeClicked('settings_ai_patrol_autofix', 'ai_autofix')}
            >
              Upgrade to Pro
            </a>{' '}
            to enable autonomous mode.
            <Show when={state.canStartTrial()}>
              {' '}
              <button
                type="button"
                onClick={state.handleStartTrial}
                disabled={state.startingTrial()}
                class="text-indigo-500 hover:underline disabled:opacity-50"
              >
                Start free trial
              </button>
            </Show>
          </p>
        </Show>

        <Show when={state.form.controlLevel !== 'read_only'}>
          <div class="flex items-start gap-3 pt-2 border-t border-blue-200 dark:border-blue-700">
            <label class="text-xs font-medium text-muted w-28 flex-shrink-0 pt-1">Protected</label>
            <div class="flex-1">
              <input
                type="text"
                value={state.form.protectedGuests}
                onInput={(e) => state.setForm('protectedGuests', e.currentTarget.value)}
                placeholder="e.g., 100, 101, prod-db"
                class="w-full min-h-10 sm:min-h-9 px-2 py-2 text-sm border border-border rounded"
                disabled={state.saving()}
              />
              <p class="text-[10px] text-muted mt-1">
                Comma-separated VMIDs or names that Pulse Assistant cannot control
              </p>
            </div>
          </div>
        </Show>
      </div>
    </>
  );
};
