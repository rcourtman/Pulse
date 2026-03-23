import { createMemo, createSignal, onMount } from 'solid-js';
import { getUpgradeActionUrlOrFallback, licenseStatus, loadLicenseStatus } from '@/stores/license';
import { isUpsellSnoozed, snoozeUpsell } from '@/utils/snooze';
import {
  getTrialBannerToneClass,
  normalizeTrialBannerDaysRemaining,
  TRIAL_BANNER_SNOOZE_KEY,
  TRIAL_BANNER_UPGRADE_REASON,
} from './trialBannerModel';

export function useTrialBannerState() {
  const [snoozed, setSnoozed] = createSignal(isUpsellSnoozed(TRIAL_BANNER_SNOOZE_KEY));

  onMount(() => {
    void loadLicenseStatus();
  });

  const isTrial = createMemo(() => licenseStatus()?.subscription_state === 'trial');
  const daysRemaining = createMemo(() =>
    normalizeTrialBannerDaysRemaining(licenseStatus()?.trial_days_remaining),
  );
  const toneClass = createMemo(() => getTrialBannerToneClass(daysRemaining()));
  const upgradeHref = createMemo(() =>
    getUpgradeActionUrlOrFallback(TRIAL_BANNER_UPGRADE_REASON),
  );

  const handleSnooze = () => {
    snoozeUpsell(TRIAL_BANNER_SNOOZE_KEY);
    setSnoozed(true);
  };

  return {
    daysRemaining,
    handleSnooze,
    isTrial,
    showActions: () => !snoozed(),
    toneClass,
    upgradeHref,
  };
}
