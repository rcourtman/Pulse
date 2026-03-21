import { createEffect, createMemo, createSignal, onMount } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { RelayAPI, type RelayStatus } from '@/api/relay';
import {
  getUpgradeActionUrlOrFallback,
  hasFeature,
  licenseLoaded,
  loadLicenseStatus,
  startProTrial,
} from '@/stores/license';
import { logger } from '@/utils/logger';
import { isUpsellSnoozed, snoozeUpsell } from '@/utils/snooze';
import { showError, showSuccess } from '@/utils/toast';
import { trackPaywallViewed, trackUpgradeClicked } from '@/utils/upgradeMetrics';

const SNOOZE_KEY = 'pulse_relay_onboarding_snoozed';
const RELAY_SETTINGS_PATH = '/settings/system-relay';

export function useRelayOnboardingCardState() {
  const navigate = useNavigate();
  const [dismissed, setDismissed] = createSignal(isUpsellSnoozed(SNOOZE_KEY));
  const [licenseReady, setLicenseReady] = createSignal<boolean>(licenseLoaded());
  const [status, setStatus] = createSignal<RelayStatus | null>(null);
  const [statusLoaded, setStatusLoaded] = createSignal(false);
  const [statusLoading, setStatusLoading] = createSignal(false);
  const [trialStarting, setTrialStarting] = createSignal(false);

  const hasRelay = createMemo(() => hasFeature('relay'));
  const relayHasActiveConnections = createMemo(() => {
    const nextStatus = status();
    if (!nextStatus || !nextStatus.connected) {
      return false;
    }
    return nextStatus.active_channels > 0;
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
    if (statusLoading() || statusLoaded() || !hasRelay()) {
      return;
    }

    setStatusLoading(true);
    try {
      const nextStatus = await RelayAPI.getStatus();
      setStatus(nextStatus);
    } catch (error) {
      logger.warn('[RelayOnboardingCard] Failed to load relay status', error);
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
    void loadRelayStatusOnce();
  });

  createEffect(() => {
    if (!licenseReady()) {
      return;
    }
    if (hasRelay() && !statusLoaded() && !statusLoading()) {
      void loadRelayStatusOnce();
    }
  });

  createEffect((wasPaywallVisible: boolean) => {
    const isPaywallVisible = shouldShowPaywall();
    if (isPaywallVisible && !wasPaywallVisible) {
      trackPaywallViewed('relay', 'dashboard_onboarding');
    }
    return isPaywallVisible;
  }, false);

  const dismiss = () => {
    snoozeUpsell(SNOOZE_KEY);
    setDismissed(true);
  };

  const handleSetupRelay = () => {
    navigate(RELAY_SETTINGS_PATH);
  };

  const handleUpgradeClick = () => {
    trackUpgradeClicked('dashboard_onboarding', 'relay');
  };

  const handleStartTrial = async () => {
    handleUpgradeClick();
    if (trialStarting()) {
      return;
    }

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
      setStatusLoaded(false);
      void loadRelayStatusOnce();
    } catch (error) {
      logger.warn('[RelayOnboardingCard] Failed to start trial; falling back to upgrade URL', error);
      showError('Unable to start trial. Redirecting to upgrade options...');
      const upgradeUrl = getUpgradeActionUrlOrFallback('relay');
      if (typeof window !== 'undefined') {
        window.location.href = upgradeUrl;
      }
    } finally {
      setTrialStarting(false);
    }
  };

  return {
    dismiss,
    handleSetupRelay,
    handleStartTrial,
    handleUpgradeClick,
    hasRelay,
    shouldShow,
    status,
    statusLoaded,
    trialStarting,
  };
}
