import { Component, Show, createMemo, createSignal, onMount, For } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { formField, formHelpText, labelClass, controlClass } from '@/components/shared/Form';
import { notificationStore } from '@/stores/notifications';
import {
  getUpgradeActionUrlOrFallback,
  isMultiTenantEnabled,
  licenseStatus,
  loadLicenseStatus,
  startProTrial,
} from '@/stores/license';
import { LicenseAPI } from '@/api/license';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import ShieldCheck from 'lucide-solid/icons/shield-check';
import BadgeCheck from 'lucide-solid/icons/badge-check';
import { trackUpgradeMetricEvent } from '@/utils/upgradeMetrics';

const TIER_LABELS: Record<string, string> = {
  free: 'Community',
  relay: 'Relay',
  pro: 'Pro',
  pro_plus: 'Pro+',
  pro_annual: 'Pro Annual',
  lifetime: 'Lifetime',
  cloud: 'Cloud',
  msp: 'MSP',
  enterprise: 'Enterprise',
};

const FEATURE_LABELS: Record<string, string> = {
  ai_patrol: 'Pulse Patrol',
  ai_alerts: 'Pulse Alert Analysis',
  ai_autofix: 'Patrol Auto-Fix',
  kubernetes_ai: 'Kubernetes Insights',
  update_alerts: 'Update Alerts',
  multi_user: 'Multi-user / RBAC',
  white_label: 'White-label Branding',
  multi_tenant: 'Multi-tenant Mode',
  unlimited: 'Unlimited Instances',
};

const formatTitleCase = (value: string) =>
  value.replace(/_/g, ' ').replace(/\b\w/g, (match) => match.toUpperCase());

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
    try {
      await loadLicenseStatus(true);
    } catch (err) {
      notificationStore.error(
        err instanceof Error ? err.message : 'Failed to load license details',
      );
    } finally {
      setLoading(false);
    }
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
      trackUpgradeMetricEvent({ type: 'trial_started', surface: 'license_panel' });
      notificationStore.success('Pro trial started');
    } catch (err) {
      const statusCode = (err as { status?: number } | null)?.status;
      if (statusCode === 409) {
        notificationStore.error('Trial already used');
      } else if (statusCode === 429) {
        notificationStore.error('Try again later');
      } else {
        notificationStore.error(err instanceof Error ? err.message : 'Failed to start Pro trial');
      }
    } finally {
      setStartingTrial(false);
    }
  };

  const statusLabel = createMemo(() => {
    switch (subscriptionState()) {
      case 'trial':
        return 'Trial';
      case 'active':
        return 'Active';
      case 'grace':
        return 'Grace Period';
      case 'suspended':
        return 'Suspended';
      case 'canceled':
      case 'expired':
        return 'Expired';
      default:
        return 'Unknown';
    }
  });

  const statusTone = createMemo(() => {
    switch (subscriptionState()) {
      case 'trial':
      case 'active':
        return 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300';
      case 'grace':
        return 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300';
      case 'suspended':
      case 'canceled':
      case 'expired':
        return 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300';
      default:
        return 'bg-surface-alt text-muted';
    }
  });

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
    return TIER_LABELS[current.tier] ?? formatTitleCase(current.tier);
  });

  const formattedFeatures = createMemo(() => {
    const current = entitlements();
    if (!current?.capabilities?.length) return [];
    return current.capabilities
      .filter((feature) => feature !== 'multi_tenant' || isMultiTenantEnabled())
      .map((feature) => FEATURE_LABELS[feature] ?? formatTitleCase(feature));
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

  const maxLimit = (key: string) => {
    const limit = entitlements()?.limits?.find((entry) => entry.key === key)?.limit;
    return typeof limit === 'number' && limit > 0 ? limit : 'Unlimited';
  };

  const trialEnded = createMemo(
    () =>
      subscriptionState() === 'expired' &&
      entitlements()?.trial_eligibility_reason === 'already_used',
  );

  const hasPaidFeatures = createMemo(() => {
    const state = subscriptionState();
    return state === 'active' || state === 'trial' || state === 'grace';
  });

  const handleActivate = async () => {
    const trimmedKey = licenseKey().trim();
    if (!trimmedKey) {
      notificationStore.error('License key is required');
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
      <SettingsPanel
        title="Pro License"
        description="Activate your Pro license to unlock auto-fix, alert-triggered AI, and advanced features."
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
      >
        <div class={formField}>
          <label class={labelClass()} for="pulse-pro-license-key">
            License Key
          </label>
          <textarea
            id="pulse-pro-license-key"
            class={controlClass('min-h-[120px] font-mono')}
            placeholder="Paste your Pro license key"
            value={licenseKey()}
            onInput={(event) => setLicenseKey(event.currentTarget.value)}
          />
          <p class={formHelpText}>
            Keys are validated locally and never sent to a license server. By activating a license,
            you agree to the{' '}
            <a
              href="https://github.com/rcourtman/Pulse/blob/main/TERMS.md"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex min-h-10 sm:min-h-9 items-center rounded px-1 text-blue-600 hover:underline"
            >
              Terms of Service
            </a>
            .
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button
            class="min-h-10 sm:min-h-9 px-4 py-2.5 text-sm font-medium rounded-md bg-blue-600 text-white hover:bg-blue-700 transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
            onClick={handleActivate}
            disabled={activating() || !licenseKey().trim()}
          >
            {activating() ? 'Activating...' : 'Activate License'}
          </button>
          <button
            class="min-h-10 sm:min-h-9 px-4 py-2.5 text-sm font-medium rounded-md border border-border text-base-content hover:bg-surface-hover transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
            onClick={handleClear}
            disabled={clearing() || loading() || !hasLicenseDetails()}
          >
            {clearing() ? 'Clearing...' : 'Clear License'}
          </button>
        </div>

        <Show when={showTrialStart()}>
          <div class="rounded-md border border-border bg-surface-alt p-3">
            <p class="text-sm font-medium text-base-content">Try Pro for free</p>
            <p class="text-xs text-muted mt-1">Start a 14-day Pro trial for this organization.</p>
            <button
              type="button"
              class="mt-3 inline-flex min-h-10 sm:min-h-9 items-center justify-center px-4 py-2.5 text-sm font-medium rounded-md bg-emerald-600 text-white hover:bg-emerald-700 transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
              disabled={startingTrial()}
              onClick={handleStartTrial}
            >
              {startingTrial() ? 'Starting...' : 'Start 14-day Pro Trial'}
            </button>
          </div>
        </Show>
      </SettingsPanel>

      <SettingsPanel
        title="Current License"
        description="Review your active tier, expiry, and available features."
        icon={<BadgeCheck class="w-5 h-5" />}
      >
        <Show when={trialEnded()}>
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
        <Show when={!loading()} fallback={<p class="text-sm ">Loading license status...</p>}>
          <div class="flex flex-wrap items-center gap-2">
            <span class={`px-2 py-1 text-xs font-medium rounded-full ${statusTone()}`}>
              {statusLabel()}
            </span>
            <Show when={entitlements()?.in_grace_period}>
              <span class="text-xs text-amber-700 dark:text-amber-300">
                Grace until {formatDate(entitlements()?.grace_period_end)}
              </span>
            </Show>
          </div>

          <Show
            when={hasLicenseDetails()}
            fallback={<div class="text-sm text-muted">No Pro license is active.</div>}
          >
            <div class="grid gap-4 sm:grid-cols-2">
              <div>
                <p class="text-xs uppercase tracking-wide text-muted">Tier</p>
                <p class="text-sm font-medium text-base-content">{formattedTier()}</p>
              </div>
              <div>
                <p class="text-xs uppercase tracking-wide text-muted">Licensed Email</p>
                <p class="text-sm font-medium text-base-content">
                  {entitlements()?.licensed_email || 'Not available'}
                </p>
              </div>
              <div>
                <p class="text-xs uppercase tracking-wide text-muted">Expires</p>
                <p class="text-sm font-medium text-base-content">{displayedExpiry()}</p>
              </div>
              <div>
                <p class="text-xs uppercase tracking-wide text-muted">Days Remaining</p>
                <p class="text-sm font-medium text-base-content">{displayedDaysRemaining()}</p>
              </div>
              <div>
                <p class="text-xs uppercase tracking-wide text-muted">Max Nodes</p>
                <p class="text-sm font-medium text-base-content">{maxLimit('max_nodes')}</p>
              </div>
              <div>
                <p class="text-xs uppercase tracking-wide text-muted">Max Guests</p>
                <p class="text-sm font-medium text-base-content">{maxLimit('max_guests')}</p>
              </div>
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
      </SettingsPanel>
    </div>
  );
};

export default ProLicensePanel;
