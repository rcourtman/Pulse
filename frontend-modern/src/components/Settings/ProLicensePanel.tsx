import { Component, Show, createMemo, createSignal, onMount, For } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { formField, formHelpText, labelClass, controlClass } from '@/components/shared/Form';
import { notificationStore } from '@/stores/notifications';
import { LicenseAPI, type LicenseStatus } from '@/api/license';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import ShieldCheck from 'lucide-solid/icons/shield-check';
import BadgeCheck from 'lucide-solid/icons/badge-check';

const PULSE_PRO_URL = 'https://pulserelay.pro/';
const PULSE_PRO_MANAGE_URL = 'https://pulserelay.pro/manage';

const TIER_LABELS: Record<string, string> = {
  free: 'Free',
  pro: 'Pro',
  pro_annual: 'Pro Annual',
  lifetime: 'Lifetime',
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
  if (!value) return 'Never';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString();
};

export const ProLicensePanel: Component = () => {
  const [status, setStatus] = createSignal<LicenseStatus | null>(null);
  const [licenseKey, setLicenseKey] = createSignal('');
  const [loading, setLoading] = createSignal(false);
  const [activating, setActivating] = createSignal(false);
  const [clearing, setClearing] = createSignal(false);

  const loadStatus = async () => {
    setLoading(true);
    try {
      const nextStatus = await LicenseAPI.getStatus();
      setStatus(nextStatus);
    } catch (err) {
      notificationStore.error(err instanceof Error ? err.message : 'Failed to load license status');
    } finally {
      setLoading(false);
    }
  };

  onMount(() => {
    void loadStatus();
  });

  const statusLabel = createMemo(() => {
    const current = status();
    if (!current) return 'Unknown';
    if (current.valid) {
      return current.in_grace_period ? 'Grace Period' : 'Active';
    }
    if (current.expires_at) {
      return 'Expired';
    }
    return 'No License';
  });

  const statusTone = createMemo(() => {
    const current = status();
    if (!current) return 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400';
    if (current.valid && current.in_grace_period) {
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300';
    }
    if (current.valid) {
      return 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300';
    }
    if (current.expires_at) {
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300';
    }
    return 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400';
  });

  const hasLicenseDetails = createMemo(() => {
    const current = status();
    if (!current) return false;
    return Boolean(current.email || current.expires_at || current.tier !== 'free');
  });

  const formattedTier = createMemo(() => {
    const current = status();
    if (!current) return 'Unknown';
    return TIER_LABELS[current.tier] ?? formatTitleCase(current.tier);
  });

  const formattedFeatures = createMemo(() => {
    const current = status();
    if (!current?.features?.length) return [];
    return current.features.map((feature) => FEATURE_LABELS[feature] ?? formatTitleCase(feature));
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
      if (result.status) {
        setStatus(result.status);
      } else {
        await loadStatus();
      }
    } catch (err) {
      notificationStore.error(err instanceof Error ? err.message : 'Failed to activate license');
    } finally {
      setActivating(false);
    }
  };

  const handleClear = async () => {
    if (!confirm('Clear the current Pulse Pro license?')) {
      return;
    }
    setClearing(true);
    try {
      const result = await LicenseAPI.clearLicense();
      notificationStore.success(result.message || 'License cleared');
      await loadStatus();
    } catch (err) {
      notificationStore.error(err instanceof Error ? err.message : 'Failed to clear license');
    } finally {
      setClearing(false);
    }
  };

  return (
    <div class="space-y-6">
      <SettingsPanel
        title="Pulse Pro License"
        description="Activate your Pulse Pro license to unlock auto-fix, alert-triggered AI, and advanced features."
        icon={<ShieldCheck class="w-5 h-5" />}
        action={
          <button
            class="inline-flex items-center gap-2 px-3 py-1.5 text-xs font-medium rounded-md border border-gray-300 dark:border-gray-700 text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors disabled:opacity-60"
            disabled={loading()}
            onClick={loadStatus}
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
            placeholder="Paste your Pulse Pro license key"
            value={licenseKey()}
            onInput={(event) => setLicenseKey(event.currentTarget.value)}
          />
          <p class={formHelpText}>
            Keys are validated locally and never sent to a license server. By activating a license, you agree to the <a href="https://github.com/rcourtman/Pulse/blob/main/TERMS.md" target="_blank" class="text-blue-600 hover:underline">Terms of Service</a>.
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button
            class="px-4 py-2 text-sm font-medium rounded-md bg-blue-600 text-white hover:bg-blue-700 transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
            onClick={handleActivate}
            disabled={activating() || !licenseKey().trim()}
          >
            {activating() ? 'Activating...' : 'Activate License'}
          </button>
          <button
            class="px-4 py-2 text-sm font-medium rounded-md border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
            onClick={handleClear}
            disabled={clearing() || loading() || !hasLicenseDetails()}
          >
            {clearing() ? 'Clearing...' : 'Clear License'}
          </button>
        </div>
      </SettingsPanel>

      <SettingsPanel
        title="Current License"
        description="Review your active tier, expiry, and available features."
        icon={<BadgeCheck class="w-5 h-5" />}
      >
        <Show when={!loading()} fallback={<p class="text-sm text-gray-500">Loading license status...</p>}>
          <div class="flex flex-wrap items-center gap-2">
            <span class={`px-2 py-1 text-xs font-medium rounded-full ${statusTone()}`}>
              {statusLabel()}
            </span>
            <Show when={status()?.in_grace_period}>
              <span class="text-xs text-amber-700 dark:text-amber-300">
                Grace until {formatDate(status()?.grace_period_end)}
              </span>
            </Show>
          </div>

          <Show when={hasLicenseDetails()} fallback={
            <div class="text-sm text-gray-500 dark:text-gray-400">
              No Pulse Pro license is active.
            </div>
          }>
            <div class="grid gap-4 sm:grid-cols-2">
              <div>
                <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Tier</p>
                <p class="text-sm font-medium text-gray-900 dark:text-gray-100">{formattedTier()}</p>
              </div>
              <div>
                <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Licensed Email</p>
                <p class="text-sm font-medium text-gray-900 dark:text-gray-100">{status()?.email || 'Not available'}</p>
              </div>
              <div>
                <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Expires</p>
                <p class="text-sm font-medium text-gray-900 dark:text-gray-100">
                  {status()?.is_lifetime ? 'Never (Lifetime)' : formatDate(status()?.expires_at)}
                </p>
              </div>
              <div>
                <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Days Remaining</p>
                <p class="text-sm font-medium text-gray-900 dark:text-gray-100">
                  {status()?.is_lifetime ? 'Unlimited' : status()?.days_remaining ?? 'Unknown'}
                </p>
              </div>
              <div>
                <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Max Nodes</p>
                <p class="text-sm font-medium text-gray-900 dark:text-gray-100">
                  {(() => {
                    const maxNodes = status()?.max_nodes;
                    return typeof maxNodes === 'number' && maxNodes > 0 ? maxNodes : 'Unlimited';
                  })()}
                </p>
              </div>
              <div>
                <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">Max Guests</p>
                <p class="text-sm font-medium text-gray-900 dark:text-gray-100">
                  {(() => {
                    const maxGuests = status()?.max_guests;
                    return typeof maxGuests === 'number' && maxGuests > 0 ? maxGuests : 'Unlimited';
                  })()}
                </p>
              </div>
            </div>

            <Show when={status()?.valid && status()?.tier !== 'free' && !status()?.is_lifetime}>
              <a
                class="inline-flex items-center gap-1 text-xs font-medium text-blue-600 dark:text-blue-400 hover:underline"
                href={`${PULSE_PRO_MANAGE_URL}?email=${encodeURIComponent(status()?.email ?? '')}`}
                target="_blank"
                rel="noreferrer"
              >
                Manage Subscription
              </a>
            </Show>

            <Show when={formattedFeatures().length > 0}>
              <div>
                <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400 mb-2">Features</p>
                <ul class="grid gap-2 sm:grid-cols-2">
                  <For each={formattedFeatures()}>
                    {(feature) => (
                      <li class="text-sm text-gray-700 dark:text-gray-300 flex items-center gap-2">
                        <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
                        {feature}
                      </li>
                    )}
                  </For>
                </ul>
              </div>
            </Show>
          </Show>

          <Show when={!status()?.valid}>
            <div class="rounded-lg border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900/20 p-3 text-sm text-amber-800 dark:text-amber-200">
              <p class="font-medium">Upgrade to Pulse Pro</p>
              <p class="text-xs text-amber-700 dark:text-amber-300 mt-1">
                Unlock Pulse Patrol, alert analysis, auto-fix, and more.
              </p>
              <a
                class="inline-flex items-center gap-1 mt-2 text-xs font-medium text-amber-800 dark:text-amber-200 hover:underline"
                href={PULSE_PRO_URL}
                target="_blank"
                rel="noreferrer"
              >
                View Pulse Pro plans
              </a>
            </div>
          </Show>
        </Show>
      </SettingsPanel>
    </div>
  );
};

export default ProLicensePanel;
