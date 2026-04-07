import { createMemo, createSignal } from 'solid-js';
import {
  commercialTrialDaysRemaining,
  getUpgradeActionDestination,
  isCommercialTrialActive,
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

  const isTrial = createMemo(
    () =>
      !presentationPolicyHidesCommercialSurfaces() &&
      isCommercialTrialActive(),
  );
  const daysRemaining = createMemo(() =>
    normalizeTrialBannerDaysRemaining(commercialTrialDaysRemaining()),
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
