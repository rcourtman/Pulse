import { Accessor, createEffect, createMemo, createSignal, onMount } from 'solid-js';
import {
  hasFeature,
  runtimeCapabilitiesLoaded,
} from '@/stores/license';
import {
  canOfferCommercialTrial,
} from '@/stores/licenseCommercial';
import { loadRuntimeCapabilities } from '@/stores/license';
import { notificationStore } from '@/stores/notifications';
import { getRBACFeatureGateCopy, type RBACFeatureGateCopy } from '@/utils/rbacPresentation';
import { trackPaywallViewed } from '@/utils/upgradeMetrics';
import { runStartProTrialAction } from '@/utils/trialStartAction';

export type RBACFeatureGateKind = 'roles' | 'user-assignments';
export type RBACFeatureGateLocation =
  | 'settings_roles_panel'
  | 'settings_user_assignments_panel';

interface UseRBACFeatureGateStateOptions {
  kind: RBACFeatureGateKind;
  loading: Accessor<boolean>;
  paywallLocation: RBACFeatureGateLocation;
}

export function useRBACFeatureGateState(options: UseRBACFeatureGateStateOptions) {
  const [startingTrial, setStartingTrial] = createSignal(false);

  const featureGateCopy = createMemo<RBACFeatureGateCopy>(() =>
    getRBACFeatureGateCopy(options.kind),
  );
  const licenseReady = createMemo(() => runtimeCapabilitiesLoaded());
  const canStartTrial = createMemo(() => canOfferCommercialTrial());
  const rbacEnabled = createMemo(() => licenseReady() && hasFeature('rbac'));
  const paywallVisible = createMemo(
    () => licenseReady() && !hasFeature('rbac') && !options.loading(),
  );

  onMount(() => {
    void loadRuntimeCapabilities();
  });

  createEffect((wasPaywallVisible) => {
    const isPaywallVisible = paywallVisible();
    if (isPaywallVisible && !wasPaywallVisible) {
      trackPaywallViewed('rbac', options.paywallLocation);
    }
    return isPaywallVisible;
  }, false);

  const handleStartTrial = async () => {
    if (startingTrial()) return;
    setStartingTrial(true);
    try {
      await runStartProTrialAction({
        showSuccess: notificationStore.success,
        showError: notificationStore.error,
      });
    } finally {
      setStartingTrial(false);
    }
  };

  return {
    canStartTrial,
    featureGateCopy,
    handleStartTrial,
    licenseReady,
    paywallVisible,
    rbacEnabled,
    startingTrial,
  };
}
