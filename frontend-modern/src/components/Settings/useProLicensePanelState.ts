import { createMemo, createSignal, onMount } from 'solid-js';
import { useLocation } from '@solidjs/router';
import { notificationStore } from '@/stores/notifications';
import {
  isMultiTenantEnabled,
  licenseLoadError,
  licenseStatus,
  loadLicenseStatus,
} from '@/stores/license';
import { LicenseAPI } from '@/api/license';
import {
  formatLicensePlanVersion,
  getCommercialMigrationNotice,
  getGrandfatheredPriceContinuityNotice,
  getLicenseFeatureLabel,
  getLicenseSubscriptionStatusPresentation,
  getLicenseTierLabel,
  getTrialActivationNotice,
} from '@/utils/licensePresentation';
import { buildSelfHostedCommercialPlanModel } from '@/utils/commercialBillingModel';
import { runStartProTrialAction } from '@/utils/trialStartAction';

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
  const [licenseKey, setLicenseKey] = createSignal('');
  const [loading, setLoading] = createSignal(false);
  const [activating, setActivating] = createSignal(false);
  const [clearing, setClearing] = createSignal(false);
  const [startingTrial, setStartingTrial] = createSignal(false);

  const entitlements = createMemo(() => licenseStatus());
  const subscriptionState = createMemo(() => entitlements()?.subscription_state);
  const trialExpiryUnix = createMemo(() => entitlements()?.trial_expires_at);
  const trialDaysRemaining = createMemo(() => entitlements()?.trial_days_remaining);

  const loadPanelData = async () => {
    setLoading(true);
    await loadLicenseStatus(true);
    setLoading(false);
  };

  onMount(() => {
    void loadPanelData();
  });

  const showTrialStart = createMemo(() => {
    const current = entitlements();
    if (!current) return false;
    if (current.commercial_migration?.state) return false;
    if (typeof current.trial_eligible === 'boolean') {
      return current.trial_eligible;
    }
    const state = subscriptionState();
    return state !== 'active' && state !== 'trial' && !licenseLoadError();
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

  const formattedPlanTerms = createMemo(() => formatLicensePlanVersion(entitlements()?.plan_version));

  const formattedFeatures = createMemo(() => {
    const current = entitlements();
    if (!current?.capabilities?.length) return [];
    return current.capabilities
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

  const monitoredSystemUsage = createMemo(() => limitStatus('max_monitored_systems')?.current ?? 0);
  const monitoredSystemLimit = createMemo(() => limitStatus('max_monitored_systems')?.limit ?? 0);
  const remainingSystemCapacity = createMemo(() => {
    const limit = monitoredSystemLimit();
    if (limit <= 0) return 'Unlimited';
    return Math.max(limit - monitoredSystemUsage(), 0);
  });

  const trialEnded = createMemo(
    () =>
      subscriptionState() === 'expired' &&
      entitlements()?.trial_eligibility_reason === 'already_used',
  );

  const trialActivationNotice = createMemo(() => {
    const params = new URLSearchParams(location.search);
    return getTrialActivationNotice(params.get('trial')?.trim().toLowerCase() ?? '');
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
      monitoredSystems: monitoredSystemUsage(),
      monitoredSystemLimit: monitoredSystemLimit() > 0 ? monitoredSystemLimit() : undefined,
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
    }),
  );

  const handleActivate = async () => {
    const trimmedKey = licenseKey().trim();
    if (!trimmedKey) {
      notificationStore.error('A license key or activation key is required');
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
      await loadPanelData();
    } catch (error) {
      notificationStore.error(error instanceof Error ? error.message : 'Failed to activate license');
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
      await loadPanelData();
    } catch (error) {
      notificationStore.error(error instanceof Error ? error.message : 'Failed to clear license');
    } finally {
      setClearing(false);
    }
  };

  return {
    activating,
    clearing,
    commercialMigrationNotice,
    commercialPlanModel,
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
    setLicenseKey,
    showTrialStart,
    startingTrial,
    statusPresentation,
    trialActivationNotice,
    trialEnded,
  };
}
