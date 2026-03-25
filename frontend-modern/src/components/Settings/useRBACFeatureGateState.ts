import { Accessor, createEffect, createMemo, createSignal, onMount } from 'solid-js';
import {
  entitlements,
  hasFeature,
  licenseLoaded,
  loadLicenseStatus,
  startProTrial,
} from '@/stores/license';
import { notificationStore } from '@/stores/notifications';
import { getRBACFeatureGateCopy, type RBACFeatureGateCopy } from '@/utils/rbacPresentation';
import { trackPaywallViewed } from '@/utils/upgradeMetrics';
import {
  getProTrialStartedMessage,
  getTrialStartErrorMessage,
} from '@/utils/upgradePresentation';

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
  const licenseReady = createMemo(() => licenseLoaded());
  const canStartTrial = createMemo(() => entitlements()?.trial_eligible !== false);
  const rbacEnabled = createMemo(() => licenseReady() && hasFeature('rbac'));
  const paywallVisible = createMemo(
    () => licenseReady() && !hasFeature('rbac') && !options.loading(),
  );

  onMount(() => {
    void loadLicenseStatus();
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
      const result = await startProTrial();
      if (result?.outcome === 'redirect') {
        window.location.href = result.actionUrl;
        return;
      }
      notificationStore.success(getProTrialStartedMessage());
    } catch (err) {
      notificationStore.error(getTrialStartErrorMessage(err));
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
