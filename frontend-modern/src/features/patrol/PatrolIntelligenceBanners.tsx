import { Show } from 'solid-js';
import ShieldAlertIcon from 'lucide-solid/icons/shield-alert';
import SettingsIcon from 'lucide-solid/icons/settings';
import SparklesIcon from 'lucide-solid/icons/sparkles';
import { UpgradeLink } from '@/components/shared/UpgradeLink';
import { presentationPolicyHidesUpgradePrompts } from '@/stores/sessionPresentationPolicy';
import { formatRelativeTime } from '@/utils/format';
import { trackUpgradeClicked } from '@/utils/upgradeMetrics';
import type { PatrolIntelligenceState } from './usePatrolIntelligenceState';

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
              <UpgradeLink
                class="text-indigo-600 dark:text-indigo-400 font-semibold hover:underline"
                destination={state.upgradeDestination()}
                onClick={() => trackUpgradeClicked('ai_intelligence_banner', 'ai_autofix')}
              >
                Upgrade to Pro
              </UpgradeLink>{' '}
              to unlock automatic fixes and alert-triggered analysis.
            </p>
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
                <p class="text-xs text-amber-700 dark:text-amber-300">
                  {state.blockedReason()}
                </p>
                <Show when={state.blockedAt()}>
                  <p class="text-[10px] text-amber-700 dark:text-amber-300">
                    Blocked {formatRelativeTime(state.blockedAt(), { compact: true })}
                  </p>
                </Show>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <a
                href="/settings/system-ai"
                class="inline-flex items-center justify-center gap-2 px-3 py-1.5 text-xs font-semibold text-amber-900 dark:text-amber-100 bg-amber-100 dark:bg-amber-900 border border-amber-200 dark:border-amber-700 rounded-md hover:bg-amber-200 dark:hover:bg-amber-900 transition-colors"
              >
                <SettingsIcon class="w-3.5 h-3.5" />
                Open AI Settings
              </a>
              <Show when={!presentationPolicyHidesUpgradePrompts() && state.licenseRequired()}>
                <UpgradeLink
                  destination={state.upgradeDestination()}
                  class="inline-flex items-center justify-center gap-2 px-3 py-1.5 text-xs font-semibold text-white bg-amber-600 hover:bg-amber-700 rounded-md transition-colors"
                >
                  <SparklesIcon class="w-3.5 h-3.5" />
                  Upgrade
                </UpgradeLink>
              </Show>
            </div>
          </div>
        </div>
      </Show>
    </>
  );
}
