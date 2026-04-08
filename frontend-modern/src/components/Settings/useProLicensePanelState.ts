import { createMemo, createSignal, onMount } from 'solid-js';
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
  getCommercialMigrationNotice,
  getGrandfatheredPriceContinuityNotice,
  getLicenseFeatureLabel,
  getMonitoredSystemContinuityNotice,
  getPurchaseActivationNotice,
  getLicenseSubscriptionStatusPresentation,
  getLicenseTierLabel,
  getTrialActivationNotice,
  isDisplayableLicenseFeature,
} from '@/utils/licensePresentation';
import {
  getSelfHostedBillingPlanIntent,
  getSelfHostedBillingPurchaseArrival,
  getSelfHostedBillingUsageDetail,
  resolveSelfHostedBillingSection,
  resolveSelfHostedPurchaseStartDestination,
  SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL,
  SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT,
  SELF_HOSTED_PRO_BILLING_PLAN_RECOVERY_HREF,
  SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED,
  SELF_HOSTED_PRO_BILLING_PURCHASE_CANCELLED,
  SELF_HOSTED_PRO_BILLING_PURCHASE_EXPIRED,
  SELF_HOSTED_PRO_BILLING_PURCHASE_FAILED,
  SELF_HOSTED_PRO_BILLING_PLAN_ROUTE,
  SELF_HOSTED_PRO_BILLING_PURCHASE_QUERY_PARAM,
  SELF_HOSTED_PRO_BILLING_USAGE_HREF,
  SELF_HOSTED_PRO_BILLING_USAGE_ROUTE,
  type SelfHostedBillingSection,
} from '@/utils/pricingHandoff';
import { buildSelfHostedCommercialPlanModel } from '@/utils/commercialBillingModel';
import { runStartProTrialAction } from '@/utils/trialStartAction';
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
  const [activating, setActivating] = createSignal(false);
  const [clearing, setClearing] = createSignal(false);
  const [startingTrial, setStartingTrial] = createSignal(false);
  const [trialActivationResult, setTrialActivationResult] = createSignal('');
  const [purchaseActivationResult, setPurchaseActivationResult] = createSignal('');

  const entitlements = createMemo(() => licenseEntitlements());
  const subscriptionState = createMemo(() => entitlements()?.subscription_state);
  const trialExpiryUnix = createMemo(() => entitlements()?.trial_expires_at);
  const trialDaysRemaining = createMemo(() => entitlements()?.trial_days_remaining);

  const loadPanelData = async () => {
    setLoading(true);
    await loadLicenseEntitlements(true);
    setLoading(false);
  };

  onMount(() => {
    const params = new URLSearchParams(location.search);
    const trialResult = params.get('trial')?.trim().toLowerCase() ?? '';
    const purchaseResult = getSelfHostedBillingPurchaseArrival(location.search) ?? '';
    if (trialResult) {
      setTrialActivationResult(trialResult);
      params.delete('trial');
    }
    if (purchaseResult) {
      setPurchaseActivationResult(purchaseResult);
      params.delete(SELF_HOSTED_PRO_BILLING_PURCHASE_QUERY_PARAM);
    }
    if (trialResult || purchaseResult) {
      const nextSearch = params.toString();
      const nextPath = `${location.pathname}${nextSearch ? `?${nextSearch}` : ''}${location.hash ?? ''}`;
      navigate(nextPath, { replace: true, scroll: false });
    }
    void loadPanelData();
  });

  const activeSection = createMemo<SelfHostedBillingSection>(() => {
    return resolveSelfHostedBillingSection(location.pathname, location.search, location.hash);
  });

  const setActiveSection = (section: string) => {
    if (section !== 'plan' && section !== 'usage') {
      return;
    }
    const nextPath =
      section === 'usage'
        ? SELF_HOSTED_PRO_BILLING_USAGE_ROUTE
        : SELF_HOSTED_PRO_BILLING_PLAN_ROUTE;
    navigate(nextPath, { replace: false, scroll: false });
  };

  const showMonitoredSystemUpgradeArrival = createMemo(
    () =>
      !purchaseActivationResult() &&
      activeSection() === 'plan' &&
      getSelfHostedBillingPlanIntent(location.search) ===
        SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT,
  );
  const showCountingRulesByDefault = createMemo(
    () =>
      activeSection() === 'usage' &&
      getSelfHostedBillingUsageDetail(location.search) ===
        SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL,
  );

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

  const limitStatus = (key: string) => entitlements()?.limits?.find((entry) => entry.key === key);

  const monitoredSystemLimitStatus = createMemo(() => limitStatus('max_monitored_systems'));
  const monitoredSystemUsageAvailable = createMemo(
    () => monitoredSystemLimitStatus()?.current_available !== false,
  );
  const monitoredSystemUsage = createMemo(() => monitoredSystemLimitStatus()?.current ?? 0);
  const monitoredSystemLimit = createMemo(() => monitoredSystemLimitStatus()?.limit ?? 0);
  const monitoredSystemUsageSummary = createMemo(() => {
    if (!monitoredSystemUsageAvailable()) {
      return 'Verifying…';
    }
    const limit = monitoredSystemLimit();
    if (limit > 0) {
      return `${monitoredSystemUsage()} / ${limit}`;
    }
    return monitoredSystemUsage();
  });
  const remainingSystemCapacity = createMemo(() => {
    if (!monitoredSystemUsageAvailable()) return 'Unavailable';
    const limit = monitoredSystemLimit();
    if (limit <= 0) return 'Unlimited';
    return Math.max(limit - monitoredSystemUsage(), 0);
  });
  const monitoredSystemContinuity = createMemo(() => entitlements()?.monitored_system_continuity);
  const monitoredSystemContinuityNotice = createMemo(() =>
    getMonitoredSystemContinuityNotice(monitoredSystemContinuity(), monitoredSystemLimitStatus()),
  );
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
    return getTrialActivationNotice(trialActivationResult());
  });
  const purchaseActivationNotice = createMemo(() => {
    return getPurchaseActivationNotice(purchaseActivationResult());
  });
  const purchaseActivationAction = createMemo<
    { label: string; destination: UpgradeDestination } | null
  >(() => {
    const purchase = purchaseActivationResult().trim().toLowerCase();
    const intent = getSelfHostedBillingPlanIntent(location.search);
    switch (purchase) {
      case SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED:
        if (intent === SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT) {
          return {
            label: SELF_HOSTED_PRO_BILLING_PRESENTATION.purchaseActivatedUsageActionLabel,
            destination: resolveUpgradeDestination(SELF_HOSTED_PRO_BILLING_USAGE_HREF),
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
          destination: resolveUpgradeDestination(SELF_HOSTED_PRO_BILLING_PLAN_RECOVERY_HREF),
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

  const hasPaidFeatures = createMemo(() => {
    const state = subscriptionState();
    return state === 'active' || state === 'trial' || state === 'grace';
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
      remainingSystemCapacity: remainingSystemCapacity(),
      maxMonitoredSystems:
        typeof limitStatus('max_monitored_systems')?.limit === 'number' &&
        limitStatus('max_monitored_systems')!.limit > 0
          ? limitStatus('max_monitored_systems')!.limit
          : 'Unlimited',
      maxGuests:
        typeof limitStatus('max_guests')?.limit === 'number' && limitStatus('max_guests')!.limit > 0
          ? limitStatus('max_guests')!.limit
          : 'Unlimited',
      monitoredSystemContinuity: monitoredSystemContinuity() ?? null,
      continuityCapturedAt: continuityCapturedAt(),
    }),
  );

  const handleActivate = async () => {
    const trimmedKey = licenseKey().trim();
    if (!trimmedKey) {
      notificationStore.error('A Pulse Pro key is required');
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
    commercialMigrationNotice,
    commercialPlanModel,
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
    purchaseActivationNotice,
    purchaseActivationAction,
    setActiveSection,
    setLicenseKey,
    showCountingRulesByDefault,
    showMonitoredSystemUpgradeArrival,
    showTrialStart,
    startingTrial,
    statusPresentation,
    trialActivationNotice,
    trialEnded,
  };
}
