import { Show, type Component } from 'solid-js';
import { UpgradeLink } from './UpgradeLink';
import { getUpgradeActionDestination } from '@/stores/licenseCommercial';
import { presentationPolicyHidesCommercialSurfaces } from '@/stores/sessionPresentationPolicy';

const HISTORY_LOCK_FEATURE_KEY = 'long_term_metrics';

interface HistoryLockPlanLinkProps {
  class?: string;
}

// A locked history range is a lock the operator opened by choosing that range,
// like the plan-locked lines in Settings, so it says where the plan lives even
// on a free install that hides proactive upgrade prompts. Demo and white-label
// runtimes hide commercial surfaces, and the lock then stays text only.
export const HistoryLockPlanLink: Component<HistoryLockPlanLinkProps> = (props) => (
  <Show when={!presentationPolicyHidesCommercialSurfaces()}>
    <UpgradeLink
      class={props.class ?? 'font-medium text-blue-600 hover:underline dark:text-blue-400'}
      destination={getUpgradeActionDestination(HISTORY_LOCK_FEATURE_KEY)}
    >
      Plans &amp; Billing
    </UpgradeLink>
  </Show>
);
