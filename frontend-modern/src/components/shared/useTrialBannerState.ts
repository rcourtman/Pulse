import { createMemo, createSignal, onMount } from 'solid-js';
import { demoModeEnabled, ensureDemoModeResolved } from '@/stores/demoMode';
import { getUpgradeActionDestination, licenseStatus, loadLicenseStatus } from '@/stores/license';
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
    void ensureDemoModeResolved();
  });

  const isTrial = createMemo(
    () => !demoModeEnabled() && licenseStatus()?.subscription_state === 'trial',
  );
  const daysRemaining = createMemo(() =>
    normalizeTrialBannerDaysRemaining(licenseStatus()?.trial_days_remaining),
  );
  const toneClass = createMemo(() => getTrialBannerToneClass(daysRemaining()));
  const upgradeDestination = createMemo(() =>
    getUpgradeActionDestination(TRIAL_BANNER_UPGRADE_REASON),
  );

  const handleSnooze = () => {
    snoozeUpsell(TRIAL_BANNER_SNOOZE_KEY);
    setSnoozed(true);
  };

  return {
    daysRemaining,
    handleSnooze,
    isTrial,
    showActions: () => !demoModeEnabled() && !snoozed(),
    toneClass,
    upgradeDestination,
  };
}
