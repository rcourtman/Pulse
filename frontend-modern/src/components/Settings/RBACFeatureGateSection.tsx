import { Component, Show } from 'solid-js';
import { UpgradeLink } from '@/components/shared/UpgradeLink';
import { getUpgradeActionDestination } from '@/stores/license';
import type { RBACFeatureGateCopy } from '@/utils/rbacPresentation';
import { trackUpgradeClicked } from '@/utils/upgradeMetrics';
import {
  getUpgradeActionButtonClass,
  UPGRADE_ACTION_LABEL,
  UPGRADE_TRIAL_LABEL,
  UPGRADE_TRIAL_LINK_CLASS,
} from '@/utils/upgradePresentation';
import type { RBACFeatureGateLocation } from './useRBACFeatureGateState';

interface RBACFeatureGateSectionProps {
  canStartTrial: boolean;
  copy: RBACFeatureGateCopy;
  paywallLocation: RBACFeatureGateLocation;
  startingTrial: boolean;
  onStartTrial: () => void | Promise<void>;
}

export const RBACFeatureGateSection: Component<RBACFeatureGateSectionProps> = (props) => (
  <div class="bg-surface-alt p-4 sm:p-6 transition-colors border-b border-border-subtle">
    <div class="flex flex-col sm:flex-row items-center gap-4">
      <div class="flex-1 text-center sm:text-left">
        <h4 class="text-base font-semibold text-base-content">{props.copy.title}</h4>
        <p class="text-sm text-muted mt-1">{props.copy.body}</p>
      </div>
      <div class="flex flex-col sm:flex-row items-center gap-2">
        <UpgradeLink
          destination={getUpgradeActionDestination('rbac')}
          class={getUpgradeActionButtonClass()}
          onClick={() => trackUpgradeClicked(props.paywallLocation, 'rbac')}
        >
          {UPGRADE_ACTION_LABEL}
        </UpgradeLink>
        <Show when={props.canStartTrial}>
          <button
            type="button"
            onClick={props.onStartTrial}
            disabled={props.startingTrial}
            class={UPGRADE_TRIAL_LINK_CLASS}
          >
            {UPGRADE_TRIAL_LABEL}
          </button>
        </Show>
      </div>
    </div>
  </div>
);
