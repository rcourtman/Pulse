import { createEffect, createMemo, createSignal, onMount } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { notificationStore } from '@/stores/notifications';
import { loadCommercialPosture } from '@/stores/licenseCommercial';
import { loadRuntimeCapabilities } from '@/stores/license';
import {
  licenseEntitlements,
  licenseEntitlementsLoadError,
  loadLicenseEntitlements,
} from '@/stores/licenseEntitlements';
import {
  fetchAgentOperationsLoopStatus,
  type AgentOperationsLoopStatus,
} from '@/api/agentCapabilities';
import { LicenseAPI } from '@/api/license';
import {
  formatLicensePlanVersion,
  getSelfHostedActivationSuccessPresentation,
  getCommercialMigrationNotice,
  getGrandfatheredPriceContinuityNotice,
  getSelfHostedPlanStatusPresentation,
  getSelfHostedPlanComparisonPresentation,
  getSelfHostedCurrentPlanPresentation,
  getLicenseFeatureLabel,
  getPurchaseActivationNotice,
  getSelfHostedCurrentPlanStatusPresentation,
  getLicenseSubscriptionStatusPresentation,
  getLicenseTierLabel,
  isDisplayableLicenseFeature,
} from '@/utils/licensePresentation';
import {
  getSelfHostedBillingHref,
  getSelfHostedBillingPlanDetail,
  getSelfHostedBillingPlanIntent,
  getSelfHostedBillingPurchaseArrival,
  resolveSelfHostedBillingSection,
  resolveSelfHostedPurchaseStartDestination,
  SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT,
  SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL,
  SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED,
  SELF_HOSTED_PRO_BILLING_PURCHASE_CANCELLED,
  SELF_HOSTED_PRO_BILLING_PURCHASE_EXPIRED,
  SELF_HOSTED_PRO_BILLING_PURCHASE_FAILED,
  SELF_HOSTED_PRO_BILLING_PURCHASE_UNAVAILABLE,
  SELF_HOSTED_PRO_BILLING_PLAN_DETAILS_QUERY_PARAM,
  SELF_HOSTED_PRO_BILLING_PLAN_HREF,
  SELF_HOSTED_PRO_BILLING_PLAN_INTENT_QUERY_PARAM,
  SELF_HOSTED_PRO_BILLING_PLAN_ROUTE,
  SELF_HOSTED_PRO_BILLING_PURCHASE_QUERY_PARAM,
  type SelfHostedBillingPlanIntent,
  type SelfHostedBillingSection,
} from '@/utils/pricingHandoff';
import {
  buildSelfHostedCommercialPlanModel,
  LIFETIME_DAYS_REMAINING_LABEL,
} from '@/utils/commercialBillingModel';
import { getSelfHostedPlanDefinitionForBillingTier } from '@/utils/selfHostedPlans';
import { resolveUpgradeDestination, type UpgradeDestination } from '@/utils/upgradeNavigation';
import { SELF_HOSTED_PRO_BILLING_PRESENTATION } from './selfHostedBillingPresentation';

const formatDate = (value?: string | null) => {
  if (!value) return 'Not available';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString();
};

const formatUnixDate = (value?: number) => {
  if (typeof value !== 'number') return 'Not available';
  const date = new Date(value * 1000);
  if (Number.isNaN(date.getTime())) return 'Not available';
  return date.toLocaleDateString();
};

export function useProLicensePanelState() {
  const location = useLocation();
  const navigate = useNavigate();
  const [licenseKey, setLicenseKey] = createSignal('');
  const [loading, setLoading] = createSignal(false);
  const [panelDataSettled, setPanelDataSettled] = createSignal(
    Boolean(licenseEntitlements() || licenseEntitlementsLoadError()),
  );
  const [activating, setActivating] = createSignal(false);
  const [clearing, setClearing] = createSignal(false);
  const [purchaseActivationResult, setPurchaseActivationResult] = createSignal('');
  const [purchaseActivationIntent, setPurchaseActivationIntent] =
    createSignal<SelfHostedBillingPlanIntent | null>(null);
  const [activationSuccessSource, setActivationSuccessSource] = createSignal<
    'manual' | 'purchase' | null
  >(null);
  const [agentOperationsStatus, setAgentOperationsStatus] =
    createSignal<AgentOperationsLoopStatus | null>(null);

  const entitlements = createMemo(() => licenseEntitlements());
  const subscriptionState = createMemo(() => entitlements()?.subscription_state);
  const trialExpiryUnix = createMemo(() => entitlements()?.trial_expires_at);
  const trialDaysRemaining = createMemo(() => entitlements()?.trial_days_remaining);
  const selfHostedPlanDefinition = createMemo(() =>
    getSelfHostedPlanDefinitionForBillingTier(entitlements()?.tier),
  );

  const loadPatrolOperatorStatus = async () => {
    try {
      setAgentOperationsStatus(await fetchAgentOperationsLoopStatus());
    } catch {
      setAgentOperationsStatus(null);
    }
  };

  const shouldLoadPatrolOperatorStatus = () => selfHostedPlanDefinition()?.tier === 'pro';

  const loadPatrolOperatorStatusIfNeeded = async () => {
    if (!shouldLoadPatrolOperatorStatus()) {
      setAgentOperationsStatus(null);
      return;
    }
    await loadPatrolOperatorStatus();
  };

  const loadPanelData = async () => {
    setLoading(true);
    try {
      await loadLicenseEntitlements(true);
    } finally {
      setLoading(false);
      setPanelDataSettled(true);
    }
  };

  onMount(() => {
    const params = new URLSearchParams(location.search);
    const purchaseResult = getSelfHostedBillingPurchaseArrival(location.search) ?? '';
    if (purchaseResult) {
      setPurchaseActivationResult(purchaseResult);
      setPurchaseActivationIntent(getSelfHostedBillingPlanIntent(location.search));
      if (purchaseResult === SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED) {
        setActivationSuccessSource('purchase');
      }
      params.delete(SELF_HOSTED_PRO_BILLING_PURCHASE_QUERY_PARAM);
      if (purchaseResult === SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED) {
        params.delete(SELF_HOSTED_PRO_BILLING_PLAN_INTENT_QUERY_PARAM);
      }
      if (
        purchaseResult === SELF_HOSTED_PRO_BILLING_PURCHASE_FAILED &&
        getSelfHostedBillingPlanDetail(location.search) !== SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL
      ) {
        params.set(
          SELF_HOSTED_PRO_BILLING_PLAN_DETAILS_QUERY_PARAM,
          SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL,
        );
      }
    }
    if (purchaseResult) {
      const nextSearch = params.toString();
      const nextPath = `${SELF_HOSTED_PRO_BILLING_PLAN_ROUTE}${nextSearch ? `?${nextSearch}` : ''}${location.hash ?? ''}`;
      navigate(nextPath, { replace: true, scroll: false });
    }
    void loadPanelData().then(() => {
      void loadPatrolOperatorStatusIfNeeded();
    });
  });

  const requestedSection = createMemo<SelfHostedBillingSection>(() =>
    resolveSelfHostedBillingSection(location.pathname, location.search, location.hash),
  );

  const activeSection = createMemo<SelfHostedBillingSection>(() => {
    return requestedSection() === 'usage' ? 'plan' : requestedSection();
  });

  createEffect(() => {
    if (!panelDataSettled()) {
      return;
    }
    if (requestedSection() !== 'usage') {
      return;
    }
    navigate(SELF_HOSTED_PRO_BILLING_PLAN_ROUTE, { replace: true, scroll: false });
  });

  const setActiveSection = (section: string) => {
    if (section !== 'plan') {
      return;
    }
    navigate(SELF_HOSTED_PRO_BILLING_PLAN_ROUTE, { replace: false, scroll: false });
  };
  const showRecoveryByDefault = createMemo(
    () =>
      activeSection() === 'plan' &&
      getSelfHostedBillingPlanDetail(location.search) === SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL,
  );
  const showPlanSelectionPrompt = createMemo(
    () =>
      activeSection() === 'plan' &&
      purchaseActivationResult().trim().length === 0 &&
      getSelfHostedBillingPlanIntent(location.search) ===
        SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT,
  );
  const planSelectionPrompt = createMemo(() => {
    if (!showPlanSelectionPrompt()) {
      return null;
    }
    return {
      tone: 'border-sky-200 dark:border-sky-900 bg-sky-50 dark:bg-sky-950 text-sky-900 dark:text-sky-100',
      title: SELF_HOSTED_PRO_BILLING_PRESENTATION.planSelectionPromptTitle,
      body: SELF_HOSTED_PRO_BILLING_PRESENTATION.planSelectionPromptBody,
      actionLabel: SELF_HOSTED_PRO_BILLING_PRESENTATION.planSelectionPromptActionLabel,
      actionDestination: resolveSelfHostedPurchaseStartDestination(
        SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT,
      ),
    };
  });

  const statusPresentation = createMemo(() =>
    getLicenseSubscriptionStatusPresentation(subscriptionState()),
  );

  const hasLicenseDetails = createMemo(() => {
    const current = entitlements();
    if (!current) return false;
    return Boolean(
      current.licensed_email ||
      current.expires_at ||
      current.trial_expires_at ||
      current.tier !== 'free',
    );
  });

  const formattedTier = createMemo(() => {
    const current = entitlements();
    if (!current) return 'Unknown';
    return getLicenseTierLabel(current.tier);
  });
  const formattedPlanTerms = createMemo(() =>
    formatLicensePlanVersion(entitlements()?.plan_version),
  );

  const formattedFeatures = createMemo(() => {
    const current = entitlements();
    if (!current?.capabilities?.length) return [];
    return current.capabilities
      .filter((feature) => isDisplayableLicenseFeature(feature))
      .map((feature) => getLicenseFeatureLabel(feature));
  });

  const displayedExpiry = createMemo(() => {
    const current = entitlements();
    if (current?.is_lifetime) return 'Never (Lifetime)';
    if (typeof current?.expires_at === 'string' && current.expires_at.length > 0) {
      return formatDate(current.expires_at);
    }
    if (subscriptionState() === 'trial') return formatUnixDate(trialExpiryUnix());
    return 'Not available';
  });

  const displayedDaysRemaining = createMemo(() => {
    const current = entitlements();
    if (current?.is_lifetime) return LIFETIME_DAYS_REMAINING_LABEL;
    if (subscriptionState() === 'trial' && typeof trialDaysRemaining() === 'number') {
      return trialDaysRemaining();
    }
    if (typeof current?.expires_at === 'string' && typeof current.days_remaining === 'number') {
      return current.days_remaining;
    }
    return 'Unknown';
  });

  const looksLikeLegacyLicenseKey = createMemo(() => {
    const trimmed = licenseKey().trim();
    if (!trimmed || trimmed.startsWith('ppk_live_')) {
      return false;
    }
    const segments = trimmed.split('.');
    return segments.length === 3 && segments.every((segment) => segment.length > 0);
  });

  const currentRetailPlanDefinition = createMemo(() => selfHostedPlanDefinition());

  const purchaseActivationNotice = createMemo(() => {
    if (purchaseActivationResult().trim().toLowerCase() === 'activated') {
      return null;
    }
    return getPurchaseActivationNotice(purchaseActivationResult());
  });
  const patrolOperatorStatus = createMemo(() => {
    const status = agentOperationsStatus();
    if (!status) {
      return null;
    }
    return {
      nextAction: status.nextAction,
      progressLabel: status.progressLabel,
      patrolControlOperationsLoopStarterCount: status.patrolControlOperationsLoopStarterCount,
      patrolControlCompletedOperationsLoopCount: status.patrolControlCompletedOperationsLoopCount,
      patrolControlResolvedOperationsLoopCount: status.patrolControlResolvedOperationsLoopCount,
      patrolControlValueState: status.patrolControlValueState,
      patrolAutonomyOperationsLoopStarterCount: status.patrolAutonomyOperationsLoopStarterCount,
      patrolAutonomyCompletedOperationsLoopCount: status.patrolAutonomyCompletedOperationsLoopCount,
      patrolAutonomyResolvedOperationsLoopCount: status.patrolAutonomyResolvedOperationsLoopCount,
      patrolAutonomyValueState: status.patrolAutonomyValueState,
      proActivationOperationsLoopStarterCount: status.proActivationOperationsLoopStarterCount,
      proActivationCompletedOperationsLoopCount: status.proActivationCompletedOperationsLoopCount,
      proActivationResolvedOperationsLoopCount: status.proActivationResolvedOperationsLoopCount,
      proActivationValueProofState: status.proActivationValueProofState,
      externalAgentReady: status.externalAgentReady,
    };
  });
  const activationSuccessSummary = createMemo(() =>
    getSelfHostedActivationSuccessPresentation({
      entitlements: entitlements(),
      displayableCapabilities: formattedFeatures(),
      patrolOperatorStatus: patrolOperatorStatus(),
      source: activationSuccessSource(),
    }),
  );
  const purchaseActivationAction = createMemo<{
    label: string;
    destination: UpgradeDestination;
  } | null>(() => {
    const purchase = purchaseActivationResult().trim().toLowerCase();
    const intent = purchaseActivationIntent();
    switch (purchase) {
      case SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED:
        if (intent === SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT) {
          return {
            label: SELF_HOSTED_PRO_BILLING_PRESENTATION.purchaseActivatedPlanActionLabel,
            destination: resolveUpgradeDestination(SELF_HOSTED_PRO_BILLING_PLAN_HREF),
          };
        }
        return null;
      case SELF_HOSTED_PRO_BILLING_PURCHASE_CANCELLED:
        return {
          label: SELF_HOSTED_PRO_BILLING_PRESENTATION.purchaseCancelledActionLabel,
          destination: resolveSelfHostedPurchaseStartDestination(intent),
        };
      case SELF_HOSTED_PRO_BILLING_PURCHASE_EXPIRED:
        return {
          label: SELF_HOSTED_PRO_BILLING_PRESENTATION.purchaseExpiredActionLabel,
          destination: resolveSelfHostedPurchaseStartDestination(intent),
        };
      case SELF_HOSTED_PRO_BILLING_PURCHASE_FAILED:
        return {
          label: SELF_HOSTED_PRO_BILLING_PRESENTATION.purchaseFailedActionLabel,
          destination: resolveUpgradeDestination(
            getSelfHostedBillingHref('plan', {
              intent,
              detail: SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL,
            }),
          ),
        };
      case SELF_HOSTED_PRO_BILLING_PURCHASE_UNAVAILABLE:
        return {
          label: SELF_HOSTED_PRO_BILLING_PRESENTATION.purchaseUnavailableActionLabel,
          destination: resolveSelfHostedPurchaseStartDestination(intent),
        };
      default:
        return null;
    }
  });

  const commercialMigrationNotice = createMemo(() =>
    getCommercialMigrationNotice(entitlements()?.commercial_migration),
  );

  const grandfatheredPriceNotice = createMemo(() =>
    getGrandfatheredPriceContinuityNotice(
      entitlements()?.plan_version,
      entitlements()?.subscription_state,
    ),
  );

  const commercialPlanModel = createMemo(() =>
    buildSelfHostedCommercialPlanModel({
      licensedEmail: entitlements()?.licensed_email,
      statusLabel: statusPresentation().label,
      tierLabel: formattedTier(),
      planTerms: formattedPlanTerms() || undefined,
      expires: displayedExpiry(),
      daysRemaining: displayedDaysRemaining() ?? 'Unknown',
      retailPlanDefinition: currentRetailPlanDefinition(),
    }),
  );
  const currentPlanSummary = createMemo(() => {
    const status = getSelfHostedCurrentPlanStatusPresentation(entitlements());
    const summary = getSelfHostedCurrentPlanPresentation({
      entitlements: entitlements(),
      displayableCapabilities: formattedFeatures(),
      patrolOperatorStatus: patrolOperatorStatus(),
    });
    return {
      ...summary,
      badgeClass: status.badgeClass,
      statusLabel: status.label,
    };
  });
  const planStatus = createMemo(() => getSelfHostedPlanStatusPresentation(entitlements()));
  const planComparisonSummary = createMemo(() => {
    if (!showPlanSelectionPrompt()) {
      return { cards: [], action: null };
    }
    const comparison = getSelfHostedPlanComparisonPresentation({
      entitlements: entitlements(),
    });
    const showActions =
      comparison.cards.length > 0 && purchaseActivationResult().trim().length === 0;
    return {
      ...comparison,
      action: showActions
        ? {
            label: SELF_HOSTED_PRO_BILLING_PRESENTATION.planComparisonActionLabel,
            destination: resolveSelfHostedPurchaseStartDestination(
              SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT,
            ),
          }
        : null,
    };
  });

  const handleActivate = async () => {
    const trimmedKey = licenseKey().trim();
    if (!trimmedKey) {
      notificationStore.error('A license key is required');
      return;
    }
    setActivating(true);
    try {
      const result = await LicenseAPI.activateLicense(trimmedKey);
      if (!result.success) {
        notificationStore.error(result.message || 'Failed to activate license');
        return;
      }
      notificationStore.success(result.message || 'License activated');
      setActivationSuccessSource('manual');
      setLicenseKey('');
      await Promise.all([
        loadPanelData(),
        loadCommercialPosture(true),
        loadRuntimeCapabilities(true),
      ]);
      await loadPatrolOperatorStatusIfNeeded();
    } catch (error) {
      notificationStore.error(
        error instanceof Error ? error.message : 'Failed to activate license',
      );
    } finally {
      setActivating(false);
    }
  };

  const handleClear = async () => {
    if (!confirm('Clear the current Pro license?')) {
      return;
    }
    setClearing(true);
    try {
      const result = await LicenseAPI.clearLicense();
      notificationStore.success(result.message || 'License cleared');
      setActivationSuccessSource(null);
      await Promise.all([
        loadPanelData(),
        loadCommercialPosture(true),
        loadRuntimeCapabilities(true),
      ]);
      await loadPatrolOperatorStatusIfNeeded();
    } catch (error) {
      notificationStore.error(error instanceof Error ? error.message : 'Failed to clear license');
    } finally {
      setClearing(false);
    }
  };

  return {
    activeSection,
    activating,
    clearing,
    activationSuccessSummary,
    planStatus,
    commercialMigrationNotice,
    commercialPlanModel,
    currentPlanSummary,
    planComparisonSummary,
    entitlements,
    formattedFeatures,
    grandfatheredPriceNotice,
    handleActivate,
    handleClear,
    hasLicenseDetails,
    licenseKey,
    loadPanelData,
    loading,
    looksLikeLegacyLicenseKey,
    planSelectionPrompt,
    purchaseActivationNotice,
    purchaseActivationAction,
    setActiveSection,
    setLicenseKey,
    showRecoveryByDefault,
    statusPresentation,
  };
}
