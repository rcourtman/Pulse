/**
 * ActiveUseTrialNudge
 *
 * Proactive trial nudge shown after 7+ days of active use for free-tier users
 * who haven't started a trial. Dismissible with 7-day snooze.
 */

import { Component, Show, createSignal, createMemo, onMount, onCleanup } from 'solid-js';
import { licenseStatus, startProTrial } from '@/stores/license';
import { isUpsellSnoozed, snoozeUpsell } from '@/utils/snooze';
import { notificationStore } from '@/stores/notifications';

const SNOOZE_KEY = 'pulse_active_use_nudge_snoozed';
const FIRST_SEEN_KEY = 'pulse_first_seen_ts';
const MIN_AGE_MS = 7 * 24 * 60 * 60 * 1000; // 7 days

function getFirstSeenTimestamp(): number {
  if (typeof window === 'undefined') return Date.now();
  try {
    const raw = window.localStorage.getItem(FIRST_SEEN_KEY);
    if (raw) {
      const ts = Number(raw);
      if (Number.isFinite(ts) && ts > 0) return ts;
    }
    // First time: record now
    const now = Date.now();
    window.localStorage.setItem(FIRST_SEEN_KEY, String(now));
    return now;
  } catch {
    return Date.now();
  }
}

export const ActiveUseTrialNudge: Component = () => {
  const [snoozed, setSnoozed] = createSignal(isUpsellSnoozed(SNOOZE_KEY));
  const [firstSeen, setFirstSeen] = createSignal(Date.now());
  const [now, setNow] = createSignal(Date.now());
  const [startingTrial, setStartingTrial] = createSignal(false);

  onMount(() => {
    setFirstSeen(getFirstSeenTimestamp());
    // Re-evaluate once per hour so the nudge can appear if day 7 is crossed during a long session
    const timer = setInterval(() => setNow(Date.now()), 60 * 60 * 1000);
    onCleanup(() => clearInterval(timer));
  });

  const isOldEnough = createMemo(() => now() - firstSeen() >= MIN_AGE_MS);

  const isFreeNoTrial = createMemo(() => {
    const ent = licenseStatus();
    if (!ent) return false;
    // Only show for free tier users who are trial-eligible
    return (
      ent.tier === 'free' &&
      ent.subscription_state !== 'trial' &&
      ent.subscription_state !== 'active' &&
      ent.trial_eligible !== false
    );
  });

  const shouldShow = createMemo(() => isOldEnough() && isFreeNoTrial() && !snoozed());

  const handleSnooze = () => {
    snoozeUpsell(SNOOZE_KEY);
    setSnoozed(true);
  };

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

  return (
    <Show when={shouldShow()}>
      <div
        class="mb-2 rounded-md border border-indigo-200 bg-indigo-50 dark:border-indigo-900 dark:bg-indigo-900/30 px-3 py-2 text-sm text-indigo-900 dark:text-indigo-100"
        role="status"
        aria-live="polite"
      >
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span class="font-medium">
            Experience the full power of Pulse — start your free trial
          </span>
          <div class="flex items-center gap-2">
            <button
              type="button"
              class="text-xs font-semibold text-indigo-700 dark:text-indigo-300 hover:underline disabled:opacity-60"
              disabled={startingTrial()}
              onClick={handleStartTrial}
            >
              {startingTrial() ? 'Starting...' : 'Start 14-day trial'}
            </button>
            <button
              type="button"
              class="text-xs opacity-70 hover:opacity-100"
              onClick={handleSnooze}
            >
              Snooze 7d
            </button>
          </div>
        </div>
      </div>
    </Show>
  );
};

export default ActiveUseTrialNudge;
