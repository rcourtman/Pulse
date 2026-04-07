import { createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import { licenseStatus, loadLicenseStatus } from '@/stores/licenseCommercial';
import { notificationStore } from '@/stores/notifications';
import { isUpsellSnoozed, snoozeUpsell } from '@/utils/snooze';
import { runStartProTrialAction } from '@/utils/trialStartAction';
import {
  ACTIVE_USE_TRIAL_NUDGE_FIRST_SEEN_KEY,
  ACTIVE_USE_TRIAL_NUDGE_REFRESH_MS,
  ACTIVE_USE_TRIAL_NUDGE_SNOOZE_KEY,
  isActiveUseTrialNudgeEligible,
  isActiveUseTrialNudgeOldEnough,
} from './activeUseTrialNudgeModel';

function getActiveUseTrialNudgeFirstSeenTimestamp(): number {
  if (typeof window === 'undefined') return Date.now();

  try {
    const raw = window.localStorage.getItem(ACTIVE_USE_TRIAL_NUDGE_FIRST_SEEN_KEY);
    if (raw) {
      const timestamp = Number(raw);
      if (Number.isFinite(timestamp) && timestamp > 0) return timestamp;
    }

    const now = Date.now();
    window.localStorage.setItem(ACTIVE_USE_TRIAL_NUDGE_FIRST_SEEN_KEY, String(now));
    return now;
  } catch {
    return Date.now();
  }
}

export function useActiveUseTrialNudgeState() {
  const [snoozed, setSnoozed] = createSignal(
    isUpsellSnoozed(ACTIVE_USE_TRIAL_NUDGE_SNOOZE_KEY),
  );
  const [firstSeen, setFirstSeen] = createSignal(Date.now());
  const [now, setNow] = createSignal(Date.now());
  const [startingTrial, setStartingTrial] = createSignal(false);

  onMount(() => {
    void loadLicenseStatus();
    setFirstSeen(getActiveUseTrialNudgeFirstSeenTimestamp());
    const timer = window.setInterval(() => setNow(Date.now()), ACTIVE_USE_TRIAL_NUDGE_REFRESH_MS);
    onCleanup(() => window.clearInterval(timer));
  });

  const shouldShow = createMemo(() => {
    return (
      isActiveUseTrialNudgeOldEnough(now(), firstSeen()) &&
      isActiveUseTrialNudgeEligible(licenseStatus()) &&
      !snoozed()
    );
  });

  const handleSnooze = () => {
    snoozeUpsell(ACTIVE_USE_TRIAL_NUDGE_SNOOZE_KEY);
    setSnoozed(true);
  };

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

  return {
    handleSnooze,
    handleStartTrial,
    shouldShow,
    startingTrial,
  };
}
