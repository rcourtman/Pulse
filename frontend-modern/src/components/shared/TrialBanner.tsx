import { Component, Show, createMemo, onMount } from 'solid-js';
import { getUpgradeActionUrlOrFallback, licenseStatus, loadLicenseStatus } from '@/stores/license';
import { shouldReduceProUpsellNoise } from '@/stores/systemSettings';

export const TrialBanner: Component = () => {
  onMount(() => {
    // Best-effort: if already loaded, this is a no-op.
    void loadLicenseStatus();
  });

  const isTrial = createMemo(() => licenseStatus()?.subscription_state === 'trial');
  const daysRemaining = createMemo(() => {
    const raw = licenseStatus()?.trial_days_remaining;
    if (typeof raw !== 'number' || !Number.isFinite(raw)) return null;
    return Math.max(0, Math.floor(raw));
  });

  const tone = createMemo(() => {
    const days = daysRemaining();
    if (days !== null && days <= 1) {
      return 'border-red-200 bg-red-50 text-red-900 dark:border-red-900 dark:bg-red-900 dark:text-red-100';
    }
    if (days !== null && days <= 3) {
      return 'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-900 dark:bg-amber-900 dark:text-amber-100';
    }
    return 'border-blue-200 bg-blue-50 text-blue-900 dark:border-blue-900 dark:bg-blue-900 dark:text-blue-100';
  });

  return (
    <Show when={isTrial()}>
      <div class={`mb-2 rounded-md border px-3 py-2 text-sm ${tone()}`} role="status" aria-live="polite">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div class="font-medium">
            Pro Trial:
            <Show when={daysRemaining() !== null} fallback={<span class="ml-2">Active</span>}>
              <span class="ml-2">{daysRemaining()} days remaining</span>
            </Show>
          </div>
          <Show when={!shouldReduceProUpsellNoise()}>
            <a
              class="text-xs font-semibold underline underline-offset-2 hover:opacity-90"
              href={getUpgradeActionUrlOrFallback('trial_banner')}
              target="_blank"
              rel="noreferrer"
            >
              Upgrade
            </a>
          </Show>
        </div>
      </div>
    </Show>
  );
};

export default TrialBanner;
