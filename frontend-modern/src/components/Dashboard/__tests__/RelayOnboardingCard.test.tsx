import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';

// ── Hoisted mocks ──────────────────────────────────────────────────────

const {
  mockNavigate,
  loadLicenseStatusMock,
  startProTrialMock,
  showErrorMock,
  showSuccessMock,
  trackPaywallViewedMock,
  trackUpgradeClickedMock,
  isUpsellSnoozedMock,
  snoozeUpsellMock,
  getRelayStatusMock,
  hasFeatureMock,
  licenseLoadedMock,
  getUpgradeActionUrlOrFallbackMock,
} = vi.hoisted(() => ({
  mockNavigate: vi.fn(),
  loadLicenseStatusMock: vi.fn(),
  startProTrialMock: vi.fn(),
  showErrorMock: vi.fn(),
  showSuccessMock: vi.fn(),
  trackPaywallViewedMock: vi.fn(),
  trackUpgradeClickedMock: vi.fn(),
  isUpsellSnoozedMock: vi.fn(),
  snoozeUpsellMock: vi.fn(),
  getRelayStatusMock: vi.fn(),
  hasFeatureMock: vi.fn(),
  licenseLoadedMock: vi.fn(),
  getUpgradeActionUrlOrFallbackMock: vi.fn(),
}));

vi.mock('@solidjs/router', () => ({
  useNavigate: () => mockNavigate,
}));

vi.mock('@/stores/license', () => ({
  hasFeature: (...args: unknown[]) => hasFeatureMock(...args),
  loadLicenseStatus: (...args: unknown[]) => loadLicenseStatusMock(...args),
  licenseLoaded: (...args: unknown[]) => licenseLoadedMock(...args),
  startProTrial: (...args: unknown[]) => startProTrialMock(...args),
  getUpgradeActionUrlOrFallback: (...args: unknown[]) => getUpgradeActionUrlOrFallbackMock(...args),
}));

vi.mock('@/api/relay', () => ({
  RelayAPI: {
    getStatus: (...args: unknown[]) => getRelayStatusMock(...args),
  },
}));

vi.mock('@/utils/toast', () => ({
  showError: (...args: unknown[]) => showErrorMock(...args),
  showSuccess: (...args: unknown[]) => showSuccessMock(...args),
}));

vi.mock('@/utils/upgradeMetrics', () => ({
  trackPaywallViewed: (...args: unknown[]) => trackPaywallViewedMock(...args),
  trackUpgradeClicked: (...args: unknown[]) => trackUpgradeClickedMock(...args),
}));

vi.mock('@/utils/snooze', () => ({
  isUpsellSnoozed: (...args: unknown[]) => isUpsellSnoozedMock(...args),
  snoozeUpsell: (...args: unknown[]) => snoozeUpsellMock(...args),
}));

vi.mock('@/utils/logger', () => ({
  logger: { warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

vi.mock('@/components/shared/Card', () => ({
  Card: (props: { children?: unknown; class?: string }) => (
    <div data-testid="card">{props.children as any}</div>
  ),
}));

// Import component AFTER mocks
import { RelayOnboardingCard } from '../RelayOnboardingCard';

// ── Helpers ─────────────────────────────────────────────────────────────

function resetAllMocks() {
  mockNavigate.mockReset();
  loadLicenseStatusMock.mockReset();
  startProTrialMock.mockReset();
  showErrorMock.mockReset();
  showSuccessMock.mockReset();
  trackPaywallViewedMock.mockReset();
  trackUpgradeClickedMock.mockReset();
  isUpsellSnoozedMock.mockReset();
  snoozeUpsellMock.mockReset();
  getRelayStatusMock.mockReset();
  hasFeatureMock.mockReset();
  licenseLoadedMock.mockReset();
  getUpgradeActionUrlOrFallbackMock.mockReset();
}

/**
 * Sets up mock defaults for the "has relay feature" scenario (setup card).
 * Override individual mocks after calling this if needed.
 */
function setupWithRelayFeature(statusOverride?: { connected: boolean; active_channels: number }) {
  const defaultStatus = statusOverride ?? { connected: false, active_channels: 0 };
  loadLicenseStatusMock.mockResolvedValue(undefined);
  licenseLoadedMock.mockReturnValue(true);
  hasFeatureMock.mockReturnValue(true);
  isUpsellSnoozedMock.mockReturnValue(false);
  getRelayStatusMock.mockResolvedValue(defaultStatus);
  getUpgradeActionUrlOrFallbackMock.mockReturnValue('/pricing?feature=relay');
}

/**
 * Sets up mock defaults for the "no relay feature" scenario (paywall card).
 */
function setupWithoutRelayFeature() {
  loadLicenseStatusMock.mockResolvedValue(undefined);
  licenseLoadedMock.mockReturnValue(true);
  hasFeatureMock.mockReturnValue(false);
  isUpsellSnoozedMock.mockReturnValue(false);
  getUpgradeActionUrlOrFallbackMock.mockReturnValue('/pricing?feature=relay');
}

// ── Tests ───────────────────────────────────────────────────────────────

describe('RelayOnboardingCard', () => {
  beforeEach(() => {
    resetAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  // ── Paywall scenario (no relay feature) ────────────────────────────

  describe('paywall view (no relay feature)', () => {
    it('renders the upgrade link and trial button when user lacks relay feature', async () => {
      setupWithoutRelayFeature();
      render(() => <RelayOnboardingCard />);

      await waitFor(() => {
        expect(loadLicenseStatusMock).toHaveBeenCalled();
      });

      // The upgrade link should be visible
      const upgradeLink = screen.getByText(/Get Relay/);
      expect(upgradeLink).toBeInTheDocument();
      expect(upgradeLink.closest('a')).toHaveAttribute('href', '/pricing?feature=relay');

      // The trial button should be visible
      expect(screen.getByText(/or start a Pro trial/)).toBeInTheDocument();
    });

    it('tracks paywall viewed event', async () => {
      setupWithoutRelayFeature();
      render(() => <RelayOnboardingCard />);

      await waitFor(() => {
        expect(trackPaywallViewedMock).toHaveBeenCalledWith('relay', 'dashboard_onboarding');
      });
    });

    it('does not render the "Set Up Relay" button', async () => {
      setupWithoutRelayFeature();
      render(() => <RelayOnboardingCard />);

      await waitFor(() => {
        expect(loadLicenseStatusMock).toHaveBeenCalled();
      });

      expect(screen.queryByText('Set Up Relay')).not.toBeInTheDocument();
    });
  });

  // ── Setup scenario (has relay feature, not connected) ──────────────

  describe('setup view (has relay feature, not yet paired)', () => {
    it('renders the "Set Up Relay" button when relay is available but not connected', async () => {
      setupWithRelayFeature({ connected: false, active_channels: 0 });
      render(() => <RelayOnboardingCard />);

      await waitFor(() => {
        expect(screen.getByText('Set Up Relay')).toBeInTheDocument();
      });
    });

    it('shows "Relay is currently disconnected" when relay reports disconnected', async () => {
      setupWithRelayFeature({ connected: false, active_channels: 0 });
      render(() => <RelayOnboardingCard />);

      await waitFor(() => {
        expect(screen.getByText('Relay is currently disconnected.')).toBeInTheDocument();
      });
    });

    it('navigates to relay settings when "Set Up Relay" is clicked', async () => {
      setupWithRelayFeature({ connected: false, active_channels: 0 });
      render(() => <RelayOnboardingCard />);

      await waitFor(() => {
        expect(screen.getByText('Set Up Relay')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText('Set Up Relay'));
      expect(mockNavigate).toHaveBeenCalledWith('/settings/system-relay');
    });

    it('does not render upgrade link or trial button', async () => {
      setupWithRelayFeature({ connected: false, active_channels: 0 });
      render(() => <RelayOnboardingCard />);

      await waitFor(() => {
        expect(screen.getByText('Set Up Relay')).toBeInTheDocument();
      });

      expect(screen.queryByText(/Get Relay/)).not.toBeInTheDocument();
      expect(screen.queryByText(/or start a Pro trial/)).not.toBeInTheDocument();
    });
  });

  // ── Hidden when relay is active ────────────────────────────────────

  describe('hidden when relay has active connections', () => {
    it('does not render when relay is connected with active channels', async () => {
      setupWithRelayFeature({ connected: true, active_channels: 1 });
      render(() => <RelayOnboardingCard />);

      // Wait for mount effects to complete
      await waitFor(() => {
        expect(getRelayStatusMock).toHaveBeenCalled();
      });

      expect(screen.queryByText('Pair Your Mobile Device')).not.toBeInTheDocument();
      expect(screen.queryByText('Set Up Relay')).not.toBeInTheDocument();
    });
  });

  // ── Dismiss (snooze) behavior ──────────────────────────────────────

  describe('dismiss behavior', () => {
    it('calls snoozeUpsell and hides the card when dismiss button is clicked', async () => {
      setupWithoutRelayFeature();
      render(() => <RelayOnboardingCard />);

      await waitFor(() => {
        expect(screen.getByText(/Get Relay/)).toBeInTheDocument();
      });

      const dismissBtn = screen.getByLabelText('Dismiss relay onboarding');
      fireEvent.click(dismissBtn);

      expect(snoozeUpsellMock).toHaveBeenCalledWith('pulse_relay_onboarding_snoozed');

      await waitFor(() => {
        expect(screen.queryByText('Pair Your Mobile Device')).not.toBeInTheDocument();
      });
    });

    it('does not render if already snoozed', async () => {
      setupWithoutRelayFeature();
      isUpsellSnoozedMock.mockReturnValue(true);

      render(() => <RelayOnboardingCard />);

      await waitFor(() => {
        expect(loadLicenseStatusMock).toHaveBeenCalled();
      });

      expect(screen.queryByText('Pair Your Mobile Device')).not.toBeInTheDocument();
    });
  });

  // ── Trial start flow ──────────────────────────────────────────────

  describe('trial start flow', () => {
    it('starts a trial successfully, shows success toast, and reloads license', async () => {
      setupWithoutRelayFeature();
      startProTrialMock.mockResolvedValue({ outcome: 'activated' });
      // After trial, license reloads and relay becomes available
      loadLicenseStatusMock.mockResolvedValue(undefined);

      render(() => <RelayOnboardingCard />);

      await waitFor(() => {
        expect(screen.getByText(/or start a Pro trial/)).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText(/or start a Pro trial/));

      await waitFor(() => {
        expect(startProTrialMock).toHaveBeenCalled();
      });

      await waitFor(() => {
        expect(showSuccessMock).toHaveBeenCalledWith('Trial started. Relay is now available.');
      });

      expect(trackUpgradeClickedMock).toHaveBeenCalledWith('dashboard_onboarding', 'relay');

      // Verify license is force-reloaded after successful trial
      expect(loadLicenseStatusMock).toHaveBeenCalledWith(true);
    });

    it('redirects when trial returns redirect outcome', async () => {
      setupWithoutRelayFeature();
      startProTrialMock.mockResolvedValue({
        outcome: 'redirect',
        actionUrl: 'https://example.com/checkout',
      });

      const originalLocation = window.location;
      Object.defineProperty(window, 'location', {
        writable: true,
        value: { ...originalLocation, href: '' },
      });

      try {
        render(() => <RelayOnboardingCard />);

        await waitFor(() => {
          expect(screen.getByText(/or start a Pro trial/)).toBeInTheDocument();
        });

        fireEvent.click(screen.getByText(/or start a Pro trial/));

        await waitFor(() => {
          expect(window.location.href).toBe('https://example.com/checkout');
        });
      } finally {
        Object.defineProperty(window, 'location', {
          writable: true,
          value: originalLocation,
        });
      }
    });

    it('shows error toast and redirects to upgrade URL when trial fails', async () => {
      setupWithoutRelayFeature();
      startProTrialMock.mockRejectedValue(new Error('server error'));
      getUpgradeActionUrlOrFallbackMock.mockReturnValue('/pricing?feature=relay');

      const originalLocation = window.location;
      Object.defineProperty(window, 'location', {
        writable: true,
        value: { ...originalLocation, href: '' },
      });

      try {
        render(() => <RelayOnboardingCard />);

        await waitFor(() => {
          expect(screen.getByText(/or start a Pro trial/)).toBeInTheDocument();
        });

        fireEvent.click(screen.getByText(/or start a Pro trial/));

        await waitFor(() => {
          expect(showErrorMock).toHaveBeenCalledWith(
            'Unable to start trial. Redirecting to upgrade options...',
          );
        });

        expect(window.location.href).toBe('/pricing?feature=relay');
      } finally {
        Object.defineProperty(window, 'location', {
          writable: true,
          value: originalLocation,
        });
      }
    });

    it('prevents duplicate trial starts on rapid double-click', async () => {
      setupWithoutRelayFeature();
      // Never-resolving promise to keep trial in progress
      startProTrialMock.mockReturnValue(new Promise(() => {}));

      render(() => <RelayOnboardingCard />);

      await waitFor(() => {
        expect(screen.getByText(/or start a Pro trial/)).toBeInTheDocument();
      });

      const trialBtn = screen.getByText(/or start a Pro trial/);
      fireEvent.click(trialBtn);
      fireEvent.click(trialBtn);
      fireEvent.click(trialBtn);

      // Despite 3 clicks, startProTrial should only be called once (guard: trialStarting())
      await waitFor(() => {
        expect(startProTrialMock).toHaveBeenCalledTimes(1);
      });
    });

    it('shows "Starting trial..." text while trial is in progress', async () => {
      setupWithoutRelayFeature();
      // Never-resolving promise to keep trial in progress
      startProTrialMock.mockReturnValue(new Promise(() => {}));

      render(() => <RelayOnboardingCard />);

      await waitFor(() => {
        expect(screen.getByText(/or start a Pro trial/)).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText(/or start a Pro trial/));

      await waitFor(() => {
        expect(screen.getByText('Starting trial...')).toBeInTheDocument();
      });
    });
  });

  // ── Relay status loading ──────────────────────────────────────────

  describe('relay status loading', () => {
    it('fetches relay status on mount when relay feature is available', async () => {
      setupWithRelayFeature();
      render(() => <RelayOnboardingCard />);

      await waitFor(() => {
        expect(getRelayStatusMock).toHaveBeenCalled();
      });
    });

    it('does not fetch relay status when relay feature is not available', async () => {
      setupWithoutRelayFeature();
      render(() => <RelayOnboardingCard />);

      await waitFor(() => {
        expect(loadLicenseStatusMock).toHaveBeenCalled();
      });

      expect(getRelayStatusMock).not.toHaveBeenCalled();
    });

    it('handles relay status fetch failure gracefully', async () => {
      setupWithRelayFeature();
      getRelayStatusMock.mockRejectedValue(new Error('network error'));

      render(() => <RelayOnboardingCard />);

      // Should still render the card (status is null, treated as not connected)
      await waitFor(() => {
        expect(screen.getByText('Set Up Relay')).toBeInTheDocument();
      });
    });
  });

  // ── Edge cases ────────────────────────────────────────────────────

  describe('edge cases', () => {
    it('does not render before license is loaded', () => {
      licenseLoadedMock.mockReturnValue(false);
      loadLicenseStatusMock.mockReturnValue(new Promise(() => {})); // never resolves
      hasFeatureMock.mockReturnValue(false);
      isUpsellSnoozedMock.mockReturnValue(false);

      render(() => <RelayOnboardingCard />);

      expect(screen.queryByText('Pair Your Mobile Device')).not.toBeInTheDocument();
    });

    it('renders the heading text correctly', async () => {
      setupWithoutRelayFeature();
      render(() => <RelayOnboardingCard />);

      await waitFor(() => {
        expect(screen.getByText('Pair Your Mobile Device')).toBeInTheDocument();
      });

      expect(screen.getByText(/Pulse Relay lets your phone securely connect/)).toBeInTheDocument();
    });

    it('connected relay with zero active channels still shows setup card', async () => {
      // connected=true but active_channels=0 should still show the card
      // because relayHasActiveConnections requires active_channels > 0
      setupWithRelayFeature({ connected: true, active_channels: 0 });
      render(() => <RelayOnboardingCard />);

      await waitFor(() => {
        expect(screen.getByText('Set Up Relay')).toBeInTheDocument();
      });
    });

    it('disconnected relay with active_channels > 0 still shows setup card', async () => {
      // connected=false should make relayHasActiveConnections return false
      // regardless of active_channels value
      setupWithRelayFeature({ connected: false, active_channels: 5 });
      render(() => <RelayOnboardingCard />);

      await waitFor(() => {
        expect(screen.getByText('Set Up Relay')).toBeInTheDocument();
      });
    });
  });
});
