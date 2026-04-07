import { createMemo, createSignal, onMount } from 'solid-js';
import {
  commercialPosture,
  getUpgradeActionDestination,
  loadCommercialPosture,
} from '@/stores/licenseCommercial';
import { presentationPolicyHidesCommercialSurfaces } from '@/stores/sessionPresentationPolicy';
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
    void loadCommercialPosture();
  });

  const isTrial = createMemo(
    () =>
      !presentationPolicyHidesCommercialSurfaces() &&
      commercialPosture()?.subscription_state === 'trial',
  );
  const daysRemaining = createMemo(() =>
    normalizeTrialBannerDaysRemaining(commercialPosture()?.trial_days_remaining),
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
    showActions: () => !presentationPolicyHidesCommercialSurfaces() && !snoozed(),
    toneClass,
    upgradeDestination,
  };
}
