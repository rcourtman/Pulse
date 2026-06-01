import { Component } from 'solid-js';
import { FeatureGateSection } from '@/components/shared/FeatureGateSection';
import { getUpgradeActionDestination } from '@/stores/licenseCommercial';
import type { RBACFeatureGateCopy } from '@/utils/rbacPresentation';
import type { RBACFeatureGateLocation } from './useRBACFeatureGateState';

interface RBACFeatureGateSectionProps {
  copy: RBACFeatureGateCopy;
  paywallLocation: RBACFeatureGateLocation;
  showUpgradePrompts: boolean;
}

export const RBACFeatureGateSection: Component<RBACFeatureGateSectionProps> = (props) => (
  <div class="bg-surface-alt p-4 sm:p-6 transition-colors border-b border-border-subtle">
    <FeatureGateSection
      title={props.copy.title}
      body={props.copy.body}
      upgradeDestination={getUpgradeActionDestination('rbac')}
      showUpgradePrompts={props.showUpgradePrompts}
    />
  </div>
);
