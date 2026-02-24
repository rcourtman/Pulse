import { Component, Show, createEffect, createMemo, createSignal, onMount } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import Smartphone from 'lucide-solid/icons/smartphone';
import X from 'lucide-solid/icons/x';
import { Card } from '@/components/shared/Card';
import { RelayAPI, type RelayStatus } from '@/api/relay';
import {
  getUpgradeActionUrlOrFallback,
  hasFeature,
  loadLicenseStatus,
  licenseLoaded,
  startProTrial,
} from '@/stores/license';
import { showError, showSuccess } from '@/utils/toast';
import { logger } from '@/utils/logger';
import { trackPaywallViewed, trackUpgradeClicked } from '@/utils/upgradeMetrics';

const DISMISSED_KEY = 'pulse_relay_onboarding_dismissed';
const RELAY_SETTINGS_PATH = '/settings/system-relay';

function readDismissed(): boolean {
  if (typeof window === 'undefined') return false;
  try {
    return window.localStorage.getItem(DISMISSED_KEY) === '1';
  } catch {
    return false;
  }
}

function writeDismissed(): void {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(DISMISSED_KEY, '1');
  } catch {
    // Ignore storage quota / privacy-mode failures.
  }
}

export const RelayOnboardingCard: Component = () => {
  const navigate = useNavigate();
  const [dismissed, setDismissed] = createSignal(readDismissed());
  const [licenseReady, setLicenseReady] = createSignal<boolean>(licenseLoaded());

  const [status, setStatus] = createSignal<RelayStatus | null>(null);
  const [statusLoaded, setStatusLoaded] = createSignal(false);
  const [statusLoading, setStatusLoading] = createSignal(false);

  const [trialStarting, setTrialStarting] = createSignal(false);

  const hasRelay = createMemo(() => hasFeature('relay'));

  const relayHasActiveConnections = createMemo(() => {
    const st = status();
    if (!st) return false;
    // "Active relay connections" on the status payload are exposed as active_channels.
    // Treat disconnected as no active connections.
    if (!st.connected) return false;
    return st.active_channels > 0;
  });

  const shouldShowPaywall = createMemo(() => licenseReady() && !dismissed() && !hasRelay());

  const shouldShowSetup = createMemo(
    () =>
      licenseReady() &&
      !dismissed() &&
      hasRelay() &&
      statusLoaded() &&
      !relayHasActiveConnections(),
  );

  const shouldShow = createMemo(() => shouldShowPaywall() || shouldShowSetup());

  const loadRelayStatusOnce = async () => {
    if (statusLoading()) return;
    if (statusLoaded()) return;
    if (!hasRelay()) return;

    setStatusLoading(true);
    try {
      const st = await RelayAPI.getStatus();
      setStatus(st);
    } catch (err) {
      logger.warn('[RelayOnboardingCard] Failed to load relay status', err);
      setStatus(null);
    } finally {
      setStatusLoaded(true);
      setStatusLoading(false);
    }
  };

  onMount(async () => {
    try {
      await loadLicenseStatus();
    } finally {
      setLicenseReady(true);
    }
    // If relay is available, fetch the status so we can decide whether it’s already paired.
    void loadRelayStatusOnce();
  });

  createEffect(() => {
    if (!licenseReady()) return;
    // If the user starts a trial (or otherwise upgrades) while on the page, we need status
    // to decide whether the onboarding card should still show.
    if (hasRelay() && !statusLoaded() && !statusLoading()) {
      void loadRelayStatusOnce();
    }
  });

  createEffect(() => {
    if (shouldShowPaywall()) {
      trackPaywallViewed('relay', 'dashboard_onboarding');
    }
  });

  const dismiss = () => {
    writeDismissed();
    setDismissed(true);
  };

  const handleSetupRelay = () => {
    navigate(RELAY_SETTINGS_PATH);
  };

  const handleStartTrial = async () => {
    trackUpgradeClicked('dashboard_onboarding', 'relay');
    if (trialStarting()) return;

    setTrialStarting(true);
    try {
      const result = await startProTrial();
      if (result?.outcome === 'redirect') {
        if (typeof window !== 'undefined') {
          window.location.href = result.actionUrl;
        }
        return;
      }

      showSuccess('Trial started. Relay is now available.');
      await loadLicenseStatus(true);

      // Re-fetch relay status now that the feature may be enabled.
      setStatusLoaded(false);
      void loadRelayStatusOnce();
    } catch (err) {
      logger.warn('[RelayOnboardingCard] Failed to start trial; falling back to upgrade URL', err);
      showError('Unable to start trial. Redirecting to upgrade options...');
      const upgradeUrl = getUpgradeActionUrlOrFallback('relay');
      if (typeof window !== 'undefined') {
        window.location.href = upgradeUrl;
      }
    } finally {
      setTrialStarting(false);
    }
  };

  return (
    <Show when={shouldShow()}>
      <Card padding="lg" class="relative overflow-hidden">
        <div class="absolute -right-10 -top-10 h-32 w-32 rounded-full bg-blue-100 dark:bg-blue-900" />
        <div class="absolute -right-16 -bottom-16 h-40 w-40 rounded-full bg-surface-alt" />

        <button
          type="button"
          class="absolute right-3 top-3 inline-flex items-center justify-center rounded-md p-1 hover:text-base-content hover:bg-surface-hover"
          onClick={dismiss}
          aria-label="Dismiss relay onboarding"
        >
          <X size={16} strokeWidth={2} />
        </button>

        <div class="relative flex items-start gap-3">
          <div class="flex h-10 w-10 items-center justify-center rounded-md bg-blue-600 text-white shadow-sm">
            <Smartphone size={20} strokeWidth={2} />
          </div>

          <div class="min-w-0 flex-1">
            <h2 class="text-base font-semibold text-base-content">Pair Your Mobile Device</h2>
            <p class="mt-1 text-sm text-muted">
              Pulse Relay lets your phone securely connect to this Pulse instance for remote
              monitoring.
            </p>

            <div class="mt-4 flex flex-wrap items-center gap-2">
              <Show
                when={hasRelay()}
                fallback={
                  <button
                    type="button"
                    class="inline-flex items-center rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white shadow-sm hover:bg-blue-700 disabled:opacity-50"
                    onClick={() => void handleStartTrial()}
                    disabled={trialStarting()}
                  >
                    {trialStarting() ? 'Starting trial...' : 'Requires Pro \u2014 Start free trial'}
                  </button>
                }
              >
                <button
                  type="button"
                  class="inline-flex items-center rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white shadow-sm hover:bg-blue-700"
                  onClick={handleSetupRelay}
                >
                  Set Up Relay
                </button>
              </Show>

              <Show when={hasRelay() && statusLoaded() && status()?.connected === false}>
                <span class="text-xs text-muted">Relay is currently disconnected.</span>
              </Show>
            </div>
          </div>
        </div>
      </Card>
    </Show>
  );
};
