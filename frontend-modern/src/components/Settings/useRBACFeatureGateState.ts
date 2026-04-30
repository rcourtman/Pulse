import { Accessor, createMemo, onMount } from 'solid-js';
import { hasFeature, runtimeCapabilitiesLoaded } from '@/stores/license';
import { presentationPolicyHidesUpgradePrompts } from '@/stores/sessionPresentationPolicy';
import { loadRuntimeCapabilities } from '@/stores/license';
import { getRBACFeatureGateCopy, type RBACFeatureGateCopy } from '@/utils/rbacPresentation';

export type RBACFeatureGateKind = 'roles' | 'user-assignments';
export type RBACFeatureGateLocation = 'settings_roles_panel' | 'settings_user_assignments_panel';

interface UseRBACFeatureGateStateOptions {
  kind: RBACFeatureGateKind;
  loading: Accessor<boolean>;
  paywallLocation: RBACFeatureGateLocation;
}

export function useRBACFeatureGateState(options: UseRBACFeatureGateStateOptions) {
  const showUpgradePrompts = createMemo(() => !presentationPolicyHidesUpgradePrompts());
  const featureGateCopy = createMemo<RBACFeatureGateCopy>(() =>
    getRBACFeatureGateCopy(options.kind, { showCommercialCopy: showUpgradePrompts() }),
  );
  const licenseReady = createMemo(() => runtimeCapabilitiesLoaded());
  const rbacEnabled = createMemo(() => licenseReady() && hasFeature('rbac'));
  const paywallVisible = createMemo(
    () => licenseReady() && !hasFeature('rbac') && !options.loading(),
  );

  onMount(() => {
    void loadRuntimeCapabilities();
  });

  return {
    featureGateCopy,
    licenseReady,
    paywallVisible,
    rbacEnabled,
    showUpgradePrompts,
  };
}
