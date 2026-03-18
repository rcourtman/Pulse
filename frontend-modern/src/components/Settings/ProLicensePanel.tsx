import { Component, Show, createMemo, createSignal, onMount, For } from 'solid-js';
import { useLocation } from '@solidjs/router';
import { notificationStore } from '@/stores/notifications';
import {
  getUpgradeActionUrlOrFallback,
  isMultiTenantEnabled,
  licenseLoadError,
  licenseStatus,
  loadLicenseStatus,
  startProTrial,
} from '@/stores/license';
import { LicenseAPI } from '@/api/license';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import ShieldCheck from 'lucide-solid/icons/shield-check';
import {
  getProTrialStartedMessage,
  getTrialAlreadyUsedMessage,
  getTrialStartErrorMessage,
  getTrialTryAgainLaterMessage,
} from '@/utils/upgradePresentation';
import {
  formatLicensePlanVersion,
  getCommercialMigrationNotice,
  getGrandfatheredPriceContinuityNotice,
  getLicenseFeatureLabel,
  getLicenseStatusLoadingState,
  getLicenseSubscriptionStatusPresentation,
  getLicenseTierLabel,
  getNoActiveProLicenseState,
  getTrialActivationNotice,
} from '@/utils/licensePresentation';
import { MonitoredSystemLedgerPanel } from './MonitoredSystemLedgerPanel';
import {
  CommercialBillingShell,
  CommercialSection,
  CommercialStatGrid,
} from './CommercialBillingSections';
import { buildSelfHostedCommercialPlanModel } from '@/utils/commercialBillingModel';
import { SelfHostedCommercialActivationSection } from './SelfHostedCommercialActivationSection';

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

export const ProLicensePanel: Component = () => {
  const location = useLocation();
  const [licenseKey, setLicenseKey] = createSignal('');
  const [loading, setLoading] = createSignal(false);
  const [activating, setActivating] = createSignal(false);
  const [clearing, setClearing] = createSignal(false);
  const [startingTrial, setStartingTrial] = createSignal(false);
  const entitlements = createMemo(() => licenseStatus());

  const subscriptionState = createMemo(() => entitlements()?.subscription_state);
  const showTrialStart = createMemo(() => {
    const current = entitlements();
    if (!current) return false;
    if (current.commercial_migration?.state) return false;
    if (typeof current.trial_eligible === 'boolean') {
      return current.trial_eligible;
    }
    const state = subscriptionState();
    return state !== 'active' && state !== 'trial';
  });
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

  const handleStartTrial = async () => {
    if (startingTrial()) return;
    setStartingTrial(true);
    try {
      const result = await startProTrial();
      if (result?.outcome === 'redirect') {
        if (typeof window !== 'undefined') {
          window.location.href = result.actionUrl;
        }
        return;
      }
      // trial_started event is emitted by the backend handler (HandleStartTrial).
      notificationStore.success(getProTrialStartedMessage());
    } catch (err) {
      const error = err as { status?: number; code?: string; message?: string } | null;
      const statusCode = error?.status;
      if (statusCode === 409 && error?.code === 'trial_already_used') {
        notificationStore.error(getTrialAlreadyUsedMessage());
      } else if (statusCode === 429) {
        notificationStore.error(getTrialTryAgainLaterMessage());
      } else {
        notificationStore.error(
          getTrialStartErrorMessage(err instanceof Error ? err.message : undefined, {
            branded: true,
          }),
        );
      }
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

  const formattedPlanTerms = createMemo(() => {
    return formatLicensePlanVersion(entitlements()?.plan_version);
  });

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

  const limitStatus = (key: string) => {
    return entitlements()?.limits?.find((entry) => entry.key === key);
  };

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

  const trialActivationResult = createMemo(() => {
    const params = new URLSearchParams(location.search);
    return params.get('trial')?.trim().toLowerCase() ?? '';
  });

  const trialActivationNotice = createMemo(() => {
    return getTrialActivationNotice(trialActivationResult());
  });

  const commercialMigrationNotice = createMemo(() => {
    return getCommercialMigrationNotice(entitlements()?.commercial_migration);
  });

  const grandfatheredPriceNotice = createMemo(() => {
    return getGrandfatheredPriceContinuityNotice(
      entitlements()?.plan_version,
      entitlements()?.subscription_state,
    );
  });

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
        typeof limitStatus('max_monitored_systems')?.limit === 'number' && limitStatus('max_monitored_systems')!.limit > 0
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
    } catch (err) {
      notificationStore.error(err instanceof Error ? err.message : 'Failed to activate license');
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
    } catch (err) {
      notificationStore.error(err instanceof Error ? err.message : 'Failed to clear license');
    } finally {
      setClearing(false);
    }
  };

  return (
    <div class="space-y-6">
      <CommercialBillingShell
        title="Pulse Pro"
        description="Manage self-hosted billing, monitored-system limits, and the activation state that controls paid features."
        icon={<ShieldCheck class="w-5 h-5" />}
        action={
          <button
            class="inline-flex min-h-10 sm:min-h-9 items-center gap-2 px-3 py-2 text-sm font-medium rounded-md border border-border text-base-content hover:bg-surface-hover transition-colors disabled:opacity-60"
            disabled={loading()}
            onClick={loadPanelData}
        >
            <RefreshCw class={`w-3.5 h-3.5 ${loading() ? 'animate-spin' : ''}`} />
            Refresh
          </button>
        }
        loading={false}
      >
        <Show when={trialActivationNotice()}>
          {(notice) => (
            <div class={`mb-4 rounded-md border p-3 text-sm ${notice().tone}`}>
              <p class="font-medium">{notice().title}</p>
              <p class="mt-1 text-xs opacity-90">{notice().body}</p>
            </div>
          )}
        </Show>
        <Show when={commercialMigrationNotice()}>
          {(notice) => (
            <div class={`mb-4 rounded-md border p-3 text-sm ${notice().tone}`}>
              <p class="font-medium">{notice().title}</p>
              <p class="mt-1 text-xs opacity-90">{notice().body}</p>
            </div>
          )}
        </Show>
        <Show when={grandfatheredPriceNotice()}>
          {(notice) => (
            <div class={`mb-4 rounded-md border p-3 text-sm ${notice().tone}`}>
              <p class="font-medium">{notice().title}</p>
              <p class="mt-1 text-xs opacity-90">{notice().body}</p>
            </div>
          )}
        </Show>
        <div class="space-y-6">
          <CommercialSection
            title="Plan"
            description="Review your active plan, expiry, included limits, and paid capabilities."
          >
        <Show when={trialEnded() && !licenseLoadError()}>
          <div class="mb-4 rounded-md border border-red-200 dark:border-red-900 bg-red-50 dark:bg-red-900 p-3 text-sm text-red-900 dark:text-red-100">
            <p class="font-medium">Your Pro trial has ended</p>
            <p class="text-xs text-red-800 dark:text-red-200 mt-1">Upgrade to keep Pro features.</p>
            <a
              class="inline-flex items-center gap-1 mt-2 text-xs font-medium text-red-900 dark:text-red-100 hover:underline"
              href={getUpgradeActionUrlOrFallback('trial_expired')}
              target="_blank"
              rel="noreferrer"
            >
              View Pro plans
            </a>
          </div>
        </Show>
        <Show when={licenseLoadError()}>
          <div class="rounded-md border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900 p-3 text-sm text-amber-800 dark:text-amber-200">
            <p class="font-medium">Could not load license status</p>
            <p class="text-xs text-amber-700 dark:text-amber-300 mt-1">
              The license server could not be reached. Some features may be temporarily restricted.
            </p>
            <button
              type="button"
              class="mt-2 inline-flex min-h-10 sm:min-h-9 items-center gap-2 px-3 py-2 text-xs font-medium rounded-md border border-amber-300 dark:border-amber-700 text-amber-800 dark:text-amber-200 hover:bg-amber-100 dark:hover:bg-amber-800 transition-colors disabled:opacity-60"
              disabled={loading()}
              onClick={loadPanelData}
            >
              <RefreshCw class={`w-3 h-3 ${loading() ? 'animate-spin' : ''}`} />
              Retry
            </button>
          </div>
        </Show>
        <Show when={!licenseLoadError()}>
          <Show
            when={!loading()}
            fallback={<p class="text-sm ">{getLicenseStatusLoadingState().text}</p>}
          >
            <div class="flex flex-wrap items-center gap-2">
              <span
                class={`px-2 py-1 text-xs font-medium rounded-full ${statusPresentation().badgeClass}`}
              >
                {statusPresentation().label}
              </span>
              <Show when={entitlements()?.in_grace_period}>
                <span class="text-xs text-amber-700 dark:text-amber-300">
                  Grace until {formatDate(entitlements()?.grace_period_end)}
                </span>
              </Show>
            </div>

            <Show
              when={hasLicenseDetails()}
              fallback={<div class="text-sm text-muted">{getNoActiveProLicenseState().text}</div>}
            >
              <CommercialStatGrid
                items={commercialPlanModel().summary}
              />

              <div class="grid gap-4 sm:grid-cols-2">
                <For each={commercialPlanModel().details}>
                  {(item) => (
                    <div>
                      <p class="text-xs uppercase tracking-wide text-muted">{item.label}</p>
                      <p class="text-sm font-medium text-base-content">{item.value}</p>
                    </div>
                  )}
                </For>
              </div>

              <Show when={formattedFeatures().length > 0}>
                <div>
                  <p class="text-xs uppercase tracking-wide text-muted mb-2">Features</p>
                  <ul class="grid gap-2 sm:grid-cols-2">
                    <For each={formattedFeatures()}>
                      {(feature) => (
                        <li class="text-sm text-base-content flex items-center gap-2">
                          <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
                          {feature}
                        </li>
                      )}
                    </For>
                  </ul>
                </div>
              </Show>
            </Show>

            <Show when={!hasPaidFeatures() && !trialEnded()}>
              <div class="rounded-md border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900 p-3 text-sm text-amber-800 dark:text-amber-200">
                <p class="font-medium">Upgrade to Pro</p>
                <p class="text-xs text-amber-700 dark:text-amber-300 mt-1">
                  Unlock Pulse Patrol, alert analysis, auto-fix, and more.
                </p>
                <a
                  class="inline-flex items-center gap-1 mt-2 text-xs font-medium text-amber-800 dark:text-amber-200 hover:underline"
                  href={getUpgradeActionUrlOrFallback('ai_autofix')}
                  target="_blank"
                  rel="noreferrer"
                >
                  View Pro plans
                </a>
              </div>
            </Show>
          </Show>
        </Show>
          </CommercialSection>

          <CommercialSection
            title="Usage"
            description="Self-hosted plans are sold by monitored systems. Child resources like VMs, containers, pods, disks, and backups do not count separately."
          >
            <MonitoredSystemLedgerPanel embedded />
          </CommercialSection>

          <SelfHostedCommercialActivationSection
            licenseKey={licenseKey()}
            activating={activating()}
            clearing={clearing()}
            loading={loading()}
            hasLicenseDetails={hasLicenseDetails()}
            showTrialStart={showTrialStart() && !licenseLoadError()}
            startingTrial={startingTrial()}
            looksLikeLegacyLicenseKey={looksLikeLegacyLicenseKey()}
            onLicenseKeyInput={setLicenseKey}
            onActivate={handleActivate}
            onClear={handleClear}
            onStartTrial={() => void handleStartTrial()}
          />
        </div>
      </CommercialBillingShell>
    </div>
  );
};

export default ProLicensePanel;
