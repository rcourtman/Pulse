import { Component, Show } from 'solid-js';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import type { AIControlLevel } from '@/utils/aiControlLevelPresentation';
import type { AISettingsState } from '@/components/Settings/useAISettingsState';
import { ExternalTextLink } from '@/components/shared/ExternalTextLink';
import { HelpIcon } from '@/components/shared/HelpIcon';
import { FormSelect } from '@/components/shared/FormSelect';
import { UpgradeLink } from '@/components/shared/UpgradeLink';
import { Toggle } from '@/components/shared/Toggle';
import { TERMS_DOC_URL } from '@/utils/docsLinks';
import {
  getAIControlLevelBadgeClass,
  getAIControlLevelDescription,
  getAIControlLevelPanelClass,
} from '@/utils/aiControlLevelPresentation';
import {
  AI_SETTINGS_ASSISTANT_PERMISSIONS_TITLE,
  getAISettingsWorkloadDiscoveryHelpContent,
  getAISettingsWorkloadDiscoverySummary,
} from '@/utils/aiSettingsPresentation';
import { UPGRADE_ACTION_LABEL } from '@/utils/upgradePresentation';

interface AIRuntimeControlsSectionProps {
  state: AISettingsState;
}

export const AIRuntimeControlsSection: Component<AIRuntimeControlsSectionProps> = (props) => {
  const { state } = props;
  return (
    <>
      <AIDiscoveryControlsSection state={state} />
      <AIProviderRuntimeControlsSection state={state} />
      <AIAssistantCommandAccessSection state={state} />
    </>
  );
};

export const AIDiscoveryControlsSection: Component<AIRuntimeControlsSectionProps> = (props) => {
  const { state } = props;

  return (
    <div class="rounded-md border border-blue-200 dark:border-blue-800 overflow-hidden">
      <button
        type="button"
        class="w-full min-h-10 sm:min-h-9 px-3 py-2.5 flex items-center justify-between bg-blue-50 dark:bg-blue-900 hover:bg-blue-100 dark:hover:bg-blue-900 transition-colors text-left"
        onClick={() => state.setShowDiscoverySettings(!state.showDiscoverySettings())}
        aria-expanded={state.showDiscoverySettings()}
        aria-controls="ai-discovery-controls-panel"
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
              d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
            />
          </svg>
          <span class="text-sm font-medium text-base-content">Service Context</span>
          <Show when={state.form.discoveryEnabled}>
            <span class="px-1.5 py-0.5 text-[10px] font-medium bg-blue-100 dark:bg-blue-800 text-blue-700 dark:text-blue-300 rounded">
              {state.form.discoveryIntervalHours > 0
                ? `Auto ${state.form.discoveryIntervalHours}h`
                : 'Manual only'}
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
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </button>
      <Show when={state.showDiscoverySettings()}>
        <div
          id="ai-discovery-controls-panel"
          class="px-3 py-3 bg-surface border-t border-border space-y-3"
        >
          <div class="flex items-center justify-between gap-2">
            <label class="text-xs font-medium text-muted flex items-center gap-1.5">
              Enable service context scans
              <HelpIcon inline={getAISettingsWorkloadDiscoveryHelpContent()} size="xs" />
            </label>
            <Toggle
              checked={state.form.discoveryEnabled}
              onChange={(event) => state.setForm('discoveryEnabled', event.currentTarget.checked)}
              disabled={state.saving()}
            />
          </div>

          <Show when={state.form.discoveryEnabled}>
            <div class="flex flex-col gap-1">
              <FormSelect
                id="ai-workload-discovery-scan-interval"
                label="Scan Interval"
                value={String(state.form.discoveryIntervalHours)}
                onChange={(e) =>
                  state.setForm('discoveryIntervalHours', parseInt(e.currentTarget.value, 10))
                }
                disabled={state.saving()}
                fieldBaseClass="flex"
                fieldClass="items-center gap-3"
                labelClass="text-xs font-medium text-muted w-32 flex-shrink-0"
                selectBaseClass="flex-1 px-2 py-1 text-sm border border-border rounded bg-surface"
              >
                <option value="0">Manual only</option>
                <option value="6">Every 6 hours</option>
                <option value="12">Every 12 hours</option>
                <option value="24">Every 24 hours</option>
                <option value="48">Every 2 days</option>
                <option value="168">Every 7 days</option>
              </FormSelect>
              <p class="text-[10px] text-muted ml-32 pl-3">
                {state.form.discoveryIntervalHours === 0
                  ? 'Recurring service context scans are off. Only manual refreshes will run.'
                  : 'Recurring service context scans will run at this interval.'}
              </p>
            </div>
          </Show>

          <div class="flex flex-col gap-2 rounded border border-border bg-surface-alt px-3 py-2 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
            <p class="text-[10px] text-muted sm:flex-1">
              {state.form.discoveryEnabled && state.form.discoveryIntervalHours > 0
                ? 'Runs the same scan used by the schedule.'
                : state.form.discoveryEnabled
                  ? 'Manual-only mode: runs one scan without enabling recurring scans.'
                  : 'Runs one service context scan without changing the schedule.'}
            </p>
            <button
              type="button"
              class="inline-flex min-h-10 items-center gap-1.5 self-start rounded-md border border-border px-3 py-1.5 text-xs font-medium text-base-content hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-50 sm:min-h-9 sm:self-auto sm:flex-shrink-0"
              onClick={() => void state.handleRunDiscoveryRefresh()}
              disabled={state.saving() || state.discoveryRunRunning()}
            >
              <Show
                when={state.discoveryRunRunning()}
                fallback={<RefreshCwIcon class="h-3.5 w-3.5" />}
              >
                <RefreshCwIcon class="h-3.5 w-3.5 animate-spin" />
              </Show>
              {state.discoveryRunRunning() ? 'Running...' : 'Run context scan'}
            </button>
          </div>

          <p class="text-[10px] text-muted">{getAISettingsWorkloadDiscoverySummary().text}</p>
        </div>
      </Show>
    </div>
  );
};

export const AIProviderRuntimeControlsSection: Component<AIRuntimeControlsSectionProps> = (
  props,
) => {
  const { state } = props;

  return (
    <>
      <div class="flex items-center gap-3 p-3 rounded-md border border-border bg-surface-alt">
        <svg
          class="w-4 h-4 text-muted flex-shrink-0"
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
          <span class="text-xs">
            ≈ ${(parseFloat(state.form.costBudgetUSD30d) / 30).toFixed(2)}/day
          </span>
        </Show>
        <Show when={!state.form.costBudgetUSD30d || parseFloat(state.form.costBudgetUSD30d) === 0}>
          <span class="text-[10px] text-muted">Set a budget to receive usage alerts</span>
        </Show>
      </div>

      <div class="flex items-center gap-3 p-3 rounded-md border border-border bg-surface-alt">
        <svg
          class="w-4 h-4 text-muted flex-shrink-0"
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
    </>
  );
};

export const AIAssistantCommandAccessSection: Component<AIRuntimeControlsSectionProps> = (
  props,
) => {
  const { state } = props;
  const showAutonomousControlOption = () =>
    !state.autoFixLocked() ||
    state.showUpgradePrompts() ||
    state.form.controlLevel === 'autonomous';
  const assistantCommandAccessBadgeLabel = () =>
    state.form.controlLevel === 'autonomous' ? 'Allowed' : 'Ask first';

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
              class={`px-1.5 py-0.5 text-[10px] font-medium rounded ${getAIControlLevelBadgeClass(state.form.controlLevel)}`}
            >
              {assistantCommandAccessBadgeLabel()}
            </span>
          </Show>
        </div>
        <p class="text-[10px] text-muted">
          This controls actions started from Assistant chat only. Patrol handles infrastructure
          work from the Patrol page.
        </p>

        <FormSelect
          id="ai-control-mode-select"
          label="Chat action mode"
          value={state.form.controlLevel}
          onChange={(e) => state.setForm('controlLevel', e.currentTarget.value as AIControlLevel)}
          disabled={state.saving()}
          fieldBaseClass="flex"
          fieldClass="items-center gap-3"
          labelClass="text-xs font-medium text-muted w-28 flex-shrink-0"
          selectBaseClass="flex-1 min-h-10 sm:min-h-9 px-2 py-2 text-sm border border-border rounded bg-surface"
        >
          <option value="read_only">Observe only - Assistant cannot take chat actions</option>
          <option value="controlled">Ask first - Assistant asks before chat-only actions</option>
          <Show when={showAutonomousControlOption()}>
            <option value="autonomous">
              Allow chat-only actions - Assistant may take eligible chat actions
            </option>
          </Show>
        </FormSelect>
        <p class="text-[10px] text-muted ml-[7.5rem]">
          {getAIControlLevelDescription(state.form.controlLevel)}
        </p>
        <Show when={state.form.controlLevel === 'autonomous'}>
          <div class="p-2 bg-amber-100 dark:bg-amber-900 rounded border border-amber-200 dark:border-amber-800 text-[10px] text-amber-800 dark:text-amber-200">
            <strong>Important:</strong> Assistant may take eligible chat-only actions from this
            mode. Infrastructure changes stay with Patrol mode. Keep protected guests set for
            anything Assistant must not touch. See{' '}
            <ExternalTextLink href={TERMS_DOC_URL} variant="compactInherit">
              Terms of Service
            </ExternalTextLink>
            .
          </div>
        </Show>
        <Show
          when={
            state.form.controlLevel === 'autonomous' &&
            state.autoFixLocked() &&
            state.showUpgradePrompts()
          }
        >
          <p class="text-xs text-muted">
            <UpgradeLink
              class="text-blue-600 dark:text-blue-400 font-medium hover:underline"
              destination={state.upgradeAutofixDestination()}
            >
              {UPGRADE_ACTION_LABEL}
            </UpgradeLink>{' '}
            to review eligible chat action options.
          </p>
        </Show>

        <Show when={state.form.controlLevel !== 'read_only'}>
          <div class="flex items-start gap-3 pt-2 border-t border-blue-200 dark:border-blue-700">
            <label class="text-xs font-medium text-muted w-28 flex-shrink-0 pt-1">
              Protected guests
            </label>
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
                Comma-separated VMIDs or names Assistant cannot touch when chat actions are allowed.
              </p>
            </div>
          </div>
        </Show>
      </div>
    </>
  );
};
