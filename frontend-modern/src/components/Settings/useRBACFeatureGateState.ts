import { Accessor, createEffect, createMemo, onMount } from 'solid-js';
import { hasFeature, runtimeCapabilitiesLoaded } from '@/stores/license';
import { presentationPolicyHidesUpgradePrompts } from '@/stores/sessionPresentationPolicy';
import { loadRuntimeCapabilities } from '@/stores/license';
import { getRBACFeatureGateCopy, type RBACFeatureGateCopy } from '@/utils/rbacPresentation';
import { trackPaywallViewed } from '@/utils/upgradeMetrics';

export type RBACFeatureGateKind = 'roles' | 'user-assignments';
export type RBACFeatureGateLocation = 'settings_roles_panel' | 'settings_user_assignments_panel';

interface UseRBACFeatureGateStateOptions {
  kind: RBACFeatureGateKind;
  loading: Accessor<boolean>;
  paywallLocation: RBACFeatureGateLocation;
}

export function useRBACFeatureGateState(options: UseRBACFeatureGateStateOptions) {
  const featureGateCopy = createMemo<RBACFeatureGateCopy>(() =>
    getRBACFeatureGateCopy(options.kind),
  );
  const licenseReady = createMemo(() => runtimeCapabilitiesLoaded());
  const showUpgradePrompts = createMemo(() => !presentationPolicyHidesUpgradePrompts());
  const rbacEnabled = createMemo(() => licenseReady() && hasFeature('rbac'));
  const paywallVisible = createMemo(
    () => licenseReady() && !hasFeature('rbac') && !options.loading(),
  );

  onMount(() => {
    void loadRuntimeCapabilities();
  });

  createEffect((wasPaywallVisible) => {
    const isPaywallVisible = paywallVisible();
    if (showUpgradePrompts() && isPaywallVisible && !wasPaywallVisible) {
      trackPaywallViewed('rbac', options.paywallLocation);
    }
    return isPaywallVisible;
  }, false);

  return {
    featureGateCopy,
    licenseReady,
    paywallVisible,
    rbacEnabled,
    showUpgradePrompts,
  };
}
