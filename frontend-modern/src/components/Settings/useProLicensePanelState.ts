import { createEffect, createMemo, createSignal, onMount } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { notificationStore } from '@/stores/notifications';
import { loadCommercialPosture } from '@/stores/licenseCommercial';
import { isMultiTenantEnabled } from '@/stores/license';
import { loadRuntimeCapabilities } from '@/stores/license';
import {
  licenseEntitlements,
  licenseEntitlementsLoadError,
  loadLicenseEntitlements,
} from '@/stores/licenseEntitlements';
import { LicenseAPI } from '@/api/license';
import {
  formatLicensePlanVersion,
  getSelfHostedActivationSuccessPresentation,
  getCommercialMigrationNotice,
  getGrandfatheredPriceContinuityNotice,
  getSelfHostedCurrentPlanPresentation,
  getLicenseFeatureLabel,
  getMonitoredSystemContinuityNotice,
  getPurchaseActivationNotice,
  getLicenseSubscriptionStatusPresentation,
  getLicenseTierLabel,
  getTrialActivationNotice,
  isDisplayableLicenseFeature,
  isUncappedGrandfatheredPlanVersion,
} from '@/utils/licensePresentation';
import {
  getSelfHostedBillingHref,
  getSelfHostedBillingPlanDetail,
  getSelfHostedBillingPlanIntent,
  getSelfHostedBillingPurchaseArrival,
  getSelfHostedBillingUsageDetail,
  resolveSelfHostedBillingSection,
  resolveSelfHostedPurchaseStartDestination,
  SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL,
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
  SELF_HOSTED_PRO_BILLING_USAGE_HREF,
  SELF_HOSTED_PRO_BILLING_USAGE_ROUTE,
  isSelfHostedPurchaseStartDestination,
  type SelfHostedBillingPlanIntent,
  type SelfHostedBillingSection,
} from '@/utils/pricingHandoff';
import { buildSelfHostedCommercialPlanModel } from '@/utils/commercialBillingModel';
import { getSelfHostedPlanDefinitionForBillingTier } from '@/utils/selfHostedPlans';
import {
  buildMonitoredSystemCapacitySectionModel,
  getMonitoredSystemLimitCapacityStatusSummary,
  getMonitoredSystemLimitUsageSummary,
  resolveMonitoredSystemCapacityStatus,
} from '@/utils/monitoredSystemPresentation';
import { runStartProTrialAction } from '@/utils/trialStartAction';
import { trackCheckoutClicked, trackPricingViewed } from '@/utils/upgradeMetrics';
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
  const [startingTrial, setStartingTrial] = createSignal(false);
  const [trialActivationResult, setTrialActivationResult] = createSignal('');
  const [purchaseActivationResult, setPurchaseActivationResult] = createSignal('');
  const [purchaseActivationIntent, setPurchaseActivationIntent] =
    createSignal<SelfHostedBillingPlanIntent | null>(null);
  const [activationSuccessSource, setActivationSuccessSource] = createSignal<
    'manual' | 'purchase' | 'trial' | null
  >(null);

  const entitlements = createMemo(() => licenseEntitlements());
  const subscriptionState = createMemo(() => entitlements()?.subscription_state);
  const trialExpiryUnix = createMemo(() => entitlements()?.trial_expires_at);
  const trialDaysRemaining = createMemo(() => entitlements()?.trial_days_remaining);

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
    const trialResult = params.get('trial')?.trim().toLowerCase() ?? '';
    const purchaseResult = getSelfHostedBillingPurchaseArrival(location.search) ?? '';
    if (trialResult) {
      setTrialActivationResult(trialResult);
      if (trialResult === 'activated') {
        setActivationSuccessSource('trial');
      }
      params.delete('trial');
    }
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
    if (trialResult || purchaseResult) {
      const nextSearch = params.toString();
      const nextPath = `${location.pathname}${nextSearch ? `?${nextSearch}` : ''}${location.hash ?? ''}`;
      navigate(nextPath, { replace: true, scroll: false });
    }
    void loadPanelData();
  });

  const requestedSection = createMemo<SelfHostedBillingSection>(() =>
    resolveSelfHostedBillingSection(location.pathname, location.search, location.hash),
  );

  const limitStatus = (key: string) => entitlements()?.limits?.find((entry) => entry.key === key);
  const monitoredSystemLimitStatus = createMemo(() => limitStatus('max_monitored_systems'));
  const monitoredSystemCapacity = createMemo(() => entitlements()?.monitored_system_capacity);
  const monitoredSystemContinuity = createMemo(() => entitlements()?.monitored_system_continuity);

  const showUsageSection = createMemo(() => {
    if (!panelDataSettled()) {
      return true;
    }

    const continuity = monitoredSystemContinuity();
    if (continuity) {
      if (continuity.capture_pending) {
        return true;
      }
      if (typeof continuity.plan_limit === 'number' && continuity.plan_limit > 0) {
        return true;
      }
      if (typeof continuity.effective_limit === 'number' && continuity.effective_limit > 0) {
        return true;
      }
      if (
        typeof continuity.grandfathered_floor === 'number' &&
        continuity.grandfathered_floor > 0
      ) {
        return true;
      }
    }

    const resolved = resolveMonitoredSystemCapacityStatus(
      monitoredSystemCapacity(),
      monitoredSystemLimitStatus(),
    );
    return Boolean(resolved && resolved.limit > 0);
  });

  const activeSection = createMemo<SelfHostedBillingSection>(() => {
    if (!panelDataSettled()) {
      return requestedSection();
    }
    if (requestedSection() === 'usage' && !showUsageSection()) {
      return 'plan';
    }
    return requestedSection();
  });

  createEffect(() => {
    if (!panelDataSettled()) {
      return;
    }
    if (requestedSection() !== 'usage' || showUsageSection()) {
      return;
    }
    navigate(SELF_HOSTED_PRO_BILLING_PLAN_ROUTE, { replace: true, scroll: false });
  });

  let trackedPlanPricingView = false;
  createEffect(() => {
    const planVisible = panelDataSettled() && activeSection() === 'plan';
    if (!planVisible) {
      trackedPlanPricingView = false;
      return;
    }
    if (trackedPlanPricingView) {
      return;
    }
    trackPricingViewed('settings_self_hosted_billing_plan', SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT);
    trackedPlanPricingView = true;
  });

  const setActiveSection = (section: string) => {
    if (section !== 'plan' && section !== 'usage') {
      return;
    }
    if (section === 'usage' && panelDataSettled() && !showUsageSection()) {
      navigate(SELF_HOSTED_PRO_BILLING_PLAN_ROUTE, { replace: false, scroll: false });
      return;
    }
    const nextPath =
      section === 'usage'
        ? SELF_HOSTED_PRO_BILLING_USAGE_ROUTE
        : SELF_HOSTED_PRO_BILLING_PLAN_ROUTE;
    navigate(nextPath, { replace: false, scroll: false });
  };
  const showCountingRulesByDefault = createMemo(
    () =>
      activeSection() === 'usage' &&
      getSelfHostedBillingUsageDetail(location.search) ===
        SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL,
  );
  const showRecoveryByDefault = createMemo(
    () =>
      activeSection() === 'plan' &&
      getSelfHostedBillingPlanDetail(location.search) === SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL,
  );
  const showDefaultPlanSelectionPrompt = createMemo(() => {
    const current = entitlements();
    if (!current) {
      return false;
    }
    if (!panelDataSettled() || licenseEntitlementsLoadError()) {
      return false;
    }
    if (current.commercial_migration?.state) {
      return false;
    }
    const state = subscriptionState();
    return state !== 'active' && state !== 'trial' && state !== 'grace';
  });
  const showPlanSelectionPrompt = createMemo(
    () =>
      activeSection() === 'plan' &&
      purchaseActivationResult().trim().length === 0 &&
      (getSelfHostedBillingPlanIntent(location.search) ===
        SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT ||
        showDefaultPlanSelectionPrompt()),
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

  const handlePlanSelectionPromptClick = () => {
    trackCheckoutClicked(
      'settings_self_hosted_billing_compare_prompt',
      SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT,
    );
  };

  const showTrialStart = createMemo(() => {
    const current = entitlements();
    if (!current) return false;
    if (current.commercial_migration?.state) return false;
    if (typeof current.trial_eligible === 'boolean') {
      return current.trial_eligible;
    }
    const state = subscriptionState();
    return state !== 'active' && state !== 'trial' && !licenseEntitlementsLoadError();
  });

  const handleStartTrial = async () => {
    if (startingTrial()) return;
    setStartingTrial(true);
    try {
      await runStartProTrialAction({
        branded: true,
        showSuccess: notificationStore.success,
        showError: notificationStore.error,
      });
    } finally {
      setStartingTrial(false);
    }
  };

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
      .filter((feature) => feature !== 'multi_tenant' || isMultiTenantEnabled())
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
    if (current?.is_lifetime) return 'Unlimited';
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

  const uncappedGrandfatheredPlan = createMemo(() =>
    isUncappedGrandfatheredPlanVersion(entitlements()?.plan_version, entitlements()?.is_lifetime),
  );
  const monitoredSystemUsageSummary = createMemo(() => {
    const limit = monitoredSystemLimitStatus();
    const capacity = monitoredSystemCapacity();
    if (!limit && !capacity && uncappedGrandfatheredPlan()) {
      return 'Unlimited';
    }
    return getMonitoredSystemLimitUsageSummary(limit, capacity);
  });
  const monitoredSystemCapacityStatusSummary = createMemo(() => {
    const limit = monitoredSystemLimitStatus();
    const capacity = monitoredSystemCapacity();
    if (!limit && !capacity && uncappedGrandfatheredPlan()) {
      return 'Unlimited';
    }
    return getMonitoredSystemLimitCapacityStatusSummary(limit, capacity);
  });
  const monitoredSystemCapacityPosture = createMemo(() =>
    resolveMonitoredSystemCapacityStatus(monitoredSystemCapacity(), monitoredSystemLimitStatus()),
  );
  const currentRetailPlanDefinition = createMemo(() => {
    if (monitoredSystemContinuity()) {
      return null;
    }
    if (uncappedGrandfatheredPlan()) {
      return null;
    }
    if ((monitoredSystemCapacityPosture()?.limit ?? 0) > 0) {
      return null;
    }
    return getSelfHostedPlanDefinitionForBillingTier(entitlements()?.tier);
  });
  const monitoredSystemContinuityNotice = createMemo(() =>
    getMonitoredSystemContinuityNotice(
      monitoredSystemContinuity(),
      monitoredSystemLimitStatus(),
      monitoredSystemCapacity(),
    ),
  );
  const monitoredSystemCapacitySection = createMemo(() => {
    const section = buildMonitoredSystemCapacitySectionModel(
      monitoredSystemLimitStatus(),
      monitoredSystemCapacity(),
    );
    if (!section) {
      return null;
    }
    const posture = monitoredSystemCapacityPosture();
    return {
      ...section,
      reviewUsageDestination: resolveUpgradeDestination(SELF_HOSTED_PRO_BILLING_USAGE_HREF),
      upgradeDestination:
        posture?.blocks_new_systems === true &&
        posture.reason !== 'legacy_migration_capture_pending'
              ? resolveUpgradeDestination(
                  getSelfHostedBillingHref('plan', {
                    intent: SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT,
                  }),
                )
              : null,
    };
  });
  const continuityCapturedAt = createMemo(() => {
    const capturedAt = monitoredSystemContinuity()?.captured_at;
    return typeof capturedAt === 'number' && capturedAt > 0
      ? formatUnixDate(capturedAt)
      : undefined;
  });

  const trialEnded = createMemo(
    () =>
      subscriptionState() === 'expired' &&
      entitlements()?.trial_eligibility_reason === 'already_used',
  );

  const trialActivationNotice = createMemo(() => {
    if (trialActivationResult().trim().toLowerCase() === 'activated') {
      return null;
    }
    return getTrialActivationNotice(trialActivationResult());
  });
  const purchaseActivationNotice = createMemo(() => {
    if (purchaseActivationResult().trim().toLowerCase() === 'activated') {
      return null;
    }
    return getPurchaseActivationNotice(purchaseActivationResult());
  });
  const activationSuccessSummary = createMemo(() =>
    getSelfHostedActivationSuccessPresentation({
      entitlements: entitlements(),
      displayableCapabilities: formattedFeatures(),
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

  const handlePurchaseActivationActionClick = () => {
    const action = purchaseActivationAction();
    if (!action || !isSelfHostedPurchaseStartDestination(action.destination.href)) {
      return;
    }
    trackCheckoutClicked(
      'settings_self_hosted_billing_purchase_return',
      purchaseActivationIntent() ?? SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_INTENT,
    );
  };

  const commercialMigrationNotice = createMemo(() =>
    getCommercialMigrationNotice(entitlements()?.commercial_migration),
  );

  const grandfatheredPriceNotice = createMemo(() =>
    getGrandfatheredPriceContinuityNotice(
      entitlements()?.plan_version,
      entitlements()?.subscription_state,
    ),
  );

  const hasPaidFeatures = createMemo(() => {
    const state = subscriptionState();
    return state === 'active' || state === 'trial' || state === 'grace';
  });
  const showGuestCapacity = createMemo(() => {
    if (currentRetailPlanDefinition()) {
      return false;
    }
    if (uncappedGrandfatheredPlan()) {
      return true;
    }
    const guestLimit = limitStatus('max_guests')?.limit;
    return typeof guestLimit === 'number' && guestLimit > 0;
  });

  const commercialPlanModel = createMemo(() =>
    buildSelfHostedCommercialPlanModel({
      licensedEmail: entitlements()?.licensed_email,
      statusLabel: statusPresentation().label,
      tierLabel: formattedTier(),
      planTerms: formattedPlanTerms() || undefined,
      expires: displayedExpiry(),
      daysRemaining: displayedDaysRemaining() ?? 'Unknown',
      monitoredSystemsSummary: monitoredSystemUsageSummary(),
      capacityStatusSummary: monitoredSystemCapacityStatusSummary(),
      maxMonitoredSystems:
        typeof limitStatus('max_monitored_systems')?.limit === 'number' &&
        limitStatus('max_monitored_systems')!.limit > 0
          ? limitStatus('max_monitored_systems')!.limit
          : 'Unlimited',
      guestCapacity:
        typeof limitStatus('max_guests')?.limit === 'number' && limitStatus('max_guests')!.limit > 0
          ? limitStatus('max_guests')!.limit
          : 'Unlimited',
      retailPlanDefinition: currentRetailPlanDefinition(),
      showGuestCapacity: showGuestCapacity(),
      monitoredSystemContinuity: monitoredSystemContinuity() ?? null,
      continuityCapturedAt: continuityCapturedAt(),
    }),
  );
  const currentPlanSummary = createMemo(() => {
    const status = statusPresentation();
    const summary = getSelfHostedCurrentPlanPresentation({
      entitlements: entitlements(),
      displayableCapabilities: formattedFeatures(),
    });
    return {
      ...summary,
      badgeClass: status.badgeClass,
      statusLabel: status.label,
    };
  });

  const handleActivate = async () => {
    const trimmedKey = licenseKey().trim();
    if (!trimmedKey) {
      notificationStore.error('A license or activation key is required');
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
    commercialMigrationNotice,
    commercialPlanModel,
    currentPlanSummary,
    monitoredSystemCapacity,
    monitoredSystemCapacitySection,
    monitoredSystemContinuityNotice,
    entitlements,
    formattedFeatures,
    grandfatheredPriceNotice,
    handleActivate,
    handleClear,
    handleStartTrial,
    hasLicenseDetails,
    hasPaidFeatures,
    licenseKey,
    loadPanelData,
    loading,
    looksLikeLegacyLicenseKey,
    planSelectionPrompt,
    handlePlanSelectionPromptClick,
    purchaseActivationNotice,
    purchaseActivationAction,
    handlePurchaseActivationActionClick,
    setActiveSection,
    setLicenseKey,
    showUsageSection,
    showCountingRulesByDefault,
    showRecoveryByDefault,
    showTrialStart,
    startingTrial,
    statusPresentation,
    trialActivationNotice,
    trialEnded,
  };
}
