import { Show } from 'solid-js';
import ShieldAlertIcon from 'lucide-solid/icons/shield-alert';
import SettingsIcon from 'lucide-solid/icons/settings';
import { presentationPolicyHidesUpgradePrompts } from '@/stores/sessionPresentationPolicy';
import { formatRelativeTime } from '@/utils/format';
import { getAIProviderDisplayName } from '@/utils/aiProviderPresentation';
import { PATROL_PROVIDER_SETTINGS_ACTION } from '@/utils/patrolRuntimeActions';
import type { PatrolIntelligenceState } from './usePatrolIntelligenceState';

// Strip a provider prefix ("deepseek:deepseek-v4-flash" -> "deepseek-v4-flash")
// so the model id reads cleanly to operators without losing the provider
// label that's already rendered alongside it.
function modelIdWithoutProviderPrefix(modelId: string | undefined): string {
  if (!modelId) return '';
  const colonIndex = modelId.indexOf(':');
  return colonIndex > 0 ? modelId.substring(colonIndex + 1) : modelId;
}

export function PatrolIntelligenceBanners(props: { state: PatrolIntelligenceState }) {
  const state = props.state;

  return (
    <>
      <Show when={state.patrolStream.isStreaming()}>
        <div class="flex-shrink-0 bg-blue-50 dark:bg-blue-900 border-b border-blue-200 dark:border-blue-800 px-4 py-2">
          <div class="flex items-center gap-3 text-sm">
            <div class="flex items-center gap-2">
              <div class="w-2 h-2 rounded-full bg-blue-500 animate-pulse" />
              <span class="font-medium text-blue-800 dark:text-blue-200">Patrol running</span>
            </div>
            <Show when={state.patrolStream.phase()}>
              <span class="text-blue-700 dark:text-blue-300">{state.patrolStream.phase()}</span>
            </Show>
            <Show when={state.patrolStream.currentTool()}>
              <span class="text-blue-600 dark:text-blue-400 font-mono text-xs bg-blue-100 dark:bg-blue-900 px-1.5 py-0.5 rounded">
                {state.patrolStream.currentTool()}
              </span>
            </Show>
            <Show when={state.patrolStream.tokens() > 0}>
              <span class="text-blue-500 dark:text-blue-400 text-xs ml-auto">
                {state.patrolStream.tokens().toLocaleString()} tokens
              </span>
            </Show>
          </div>
        </div>
      </Show>

      <Show
        when={
          !presentationPolicyHidesUpgradePrompts() &&
          state.licenseRequired() &&
          !state.showBlockedBanner()
        }
      >
        <div class="flex-shrink-0 bg-blue-50 dark:bg-blue-900 border-b border-blue-200 dark:border-blue-800 px-3 py-2">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <p class="text-xs text-blue-700 dark:text-blue-300">
              Safe remediation workflows and alert-triggered root-cause analysis are not enabled on
              this plan.
            </p>
          </div>
        </div>
      </Show>

      <Show when={state.showReadinessBanner() && !state.showBlockedBanner()}>
        <div
          class={`flex-shrink-0 border-b px-4 py-3 ${
            state.patrolReadiness()?.status === 'not_ready'
              ? 'bg-red-50 dark:bg-red-950 border-red-200 dark:border-red-900'
              : 'bg-amber-50 dark:bg-amber-900 border-amber-200 dark:border-amber-800'
          }`}
        >
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="flex items-start gap-3">
              <div
                class={`flex-shrink-0 p-1.5 rounded-md ${
                  state.patrolReadiness()?.status === 'not_ready'
                    ? 'bg-red-100 dark:bg-red-900'
                    : 'bg-amber-100 dark:bg-amber-900'
                }`}
              >
                <ShieldAlertIcon
                  class={`w-4 h-4 ${
                    state.patrolReadiness()?.status === 'not_ready'
                      ? 'text-red-600 dark:text-red-300'
                      : 'text-amber-600 dark:text-amber-400'
                  }`}
                />
              </div>
              <div>
                <p
                  class={`text-sm font-semibold ${
                    state.patrolReadiness()?.status === 'not_ready'
                      ? 'text-red-900 dark:text-red-100'
                      : 'text-amber-900 dark:text-amber-100'
                  }`}
                >
                  {state.patrolReadiness()?.status === 'not_ready'
                    ? 'Patrol readiness issue'
                    : 'Patrol readiness warning'}
                </p>
                <p
                  class={`text-xs ${
                    state.patrolReadiness()?.status === 'not_ready'
                      ? 'text-red-700 dark:text-red-200'
                      : 'text-amber-700 dark:text-amber-300'
                  }`}
                >
                  {state.patrolReadiness()?.summary}
                </p>
                {/* Concrete diagnosis: which provider and model failed, and
                    the preflight recommendation. Without these the banner
                    summary alone ("Provider connection issue") leaves the
                    operator with nothing actionable. */}
                <Show
                  when={
                    state.patrolReadiness()?.provider || state.patrolReadiness()?.model
                  }
                >
                  <p
                    class={`text-xs mt-1 ${
                      state.patrolReadiness()?.status === 'not_ready'
                        ? 'text-red-700 dark:text-red-200'
                        : 'text-amber-700 dark:text-amber-300'
                    }`}
                  >
                    <Show when={state.patrolReadiness()?.provider}>
                      <span class="font-medium">Provider:</span>{' '}
                      {getAIProviderDisplayName(state.patrolReadiness()!.provider!)}
                    </Show>
                    <Show
                      when={
                        state.patrolReadiness()?.provider && state.patrolReadiness()?.model
                      }
                    >
                      {' · '}
                    </Show>
                    <Show when={state.patrolReadiness()?.model}>
                      <span class="font-medium">Model:</span>{' '}
                      <code class="font-mono">
                        {modelIdWithoutProviderPrefix(state.patrolReadiness()!.model)}
                      </code>
                    </Show>
                    <Show
                      when={
                        state.patrolPreflight()?.duration_ms !== undefined &&
                        state.patrolPreflight()!.duration_ms > 0
                      }
                    >
                      {' · '}
                      <span class="font-medium">Preflight:</span>{' '}
                      {(state.patrolPreflight()!.duration_ms / 1000).toFixed(1)}s
                      <Show when={state.patrolPreflight()?.tool_call_observed === false}>
                        , no tool call observed
                      </Show>
                    </Show>
                  </p>
                </Show>
                <Show when={state.patrolPreflight()?.recommendation}>
                  <p
                    class={`text-xs mt-1 italic ${
                      state.patrolReadiness()?.status === 'not_ready'
                        ? 'text-red-700 dark:text-red-200'
                        : 'text-amber-700 dark:text-amber-300'
                    }`}
                  >
                    {state.patrolPreflight()!.recommendation}
                  </p>
                </Show>
              </div>
            </div>
            <a
              href={PATROL_PROVIDER_SETTINGS_ACTION.href}
              class={`inline-flex items-center justify-center gap-2 px-3 py-1.5 text-xs font-semibold rounded-md border transition-colors ${
                state.patrolReadiness()?.status === 'not_ready'
                  ? 'text-red-900 dark:text-red-100 bg-red-100 dark:bg-red-900 border-red-200 dark:border-red-700 hover:bg-red-200 dark:hover:bg-red-900'
                  : 'text-amber-900 dark:text-amber-100 bg-amber-100 dark:bg-amber-900 border-amber-200 dark:border-amber-700 hover:bg-amber-200 dark:hover:bg-amber-900'
              }`}
            >
              <SettingsIcon class="w-3.5 h-3.5" />
              {PATROL_PROVIDER_SETTINGS_ACTION.label}
            </a>
          </div>
        </div>
      </Show>

      <Show when={state.showBlockedBanner()}>
        <div class="flex-shrink-0 bg-amber-50 dark:bg-amber-900 border-b border-amber-200 dark:border-amber-800 px-4 py-3">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="flex items-start gap-3">
              <div class="flex-shrink-0 p-1.5 bg-amber-100 dark:bg-amber-900 rounded-md">
                <ShieldAlertIcon class="w-4 h-4 text-amber-600 dark:text-amber-400" />
              </div>
              <div>
                <p class="text-sm font-semibold text-amber-900 dark:text-amber-100">
                  Patrol paused
                </p>
                <p class="text-xs text-amber-700 dark:text-amber-300">{state.blockedReason()}</p>
                <Show when={state.blockedAt()}>
                  <p class="text-[10px] text-amber-700 dark:text-amber-300">
                    Blocked {formatRelativeTime(state.blockedAt(), { compact: true })}
                  </p>
                </Show>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <a
                href={PATROL_PROVIDER_SETTINGS_ACTION.href}
                class="inline-flex items-center justify-center gap-2 px-3 py-1.5 text-xs font-semibold text-amber-900 dark:text-amber-100 bg-amber-100 dark:bg-amber-900 border border-amber-200 dark:border-amber-700 rounded-md hover:bg-amber-200 dark:hover:bg-amber-900 transition-colors"
              >
                <SettingsIcon class="w-3.5 h-3.5" />
                {PATROL_PROVIDER_SETTINGS_ACTION.label}
              </a>
            </div>
          </div>
        </div>
      </Show>
    </>
  );
}
