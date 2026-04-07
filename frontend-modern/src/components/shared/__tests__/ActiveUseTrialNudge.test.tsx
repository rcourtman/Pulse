import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import activeUseTrialNudgeSource from '@/components/shared/ActiveUseTrialNudge.tsx?raw';
import activeUseTrialNudgeModelSource from '@/components/shared/activeUseTrialNudgeModel.ts?raw';
import activeUseTrialNudgeStateSource from '@/components/shared/useActiveUseTrialNudgeState.ts?raw';
import {
  ACTIVE_USE_TRIAL_NUDGE_FIRST_SEEN_KEY,
  ACTIVE_USE_TRIAL_NUDGE_MIN_AGE_MS,
  ACTIVE_USE_TRIAL_NUDGE_SNOOZE_KEY,
} from '@/components/shared/activeUseTrialNudgeModel';

const {
  commercialPostureMock,
  startProTrialMock,
  showSuccessMock,
  showErrorMock,
  isUpsellSnoozedMock,
  snoozeUpsellMock,
} = vi.hoisted(() => ({
  commercialPostureMock: vi.fn(),
  startProTrialMock: vi.fn(),
  showSuccessMock: vi.fn(),
  showErrorMock: vi.fn(),
  isUpsellSnoozedMock: vi.fn(),
  snoozeUpsellMock: vi.fn(),
}));

vi.mock('@/stores/licenseCommercial', () => ({
  commercialPosture: (...args: unknown[]) => commercialPostureMock(...args),
  loadCommercialPosture: vi.fn(),
  startProTrial: (...args: unknown[]) => startProTrialMock(...args),
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: (...args: unknown[]) => showSuccessMock(...args),
    error: (...args: unknown[]) => showErrorMock(...args),
  },
}));

vi.mock('@/utils/snooze', () => ({
  isUpsellSnoozed: (...args: unknown[]) => isUpsellSnoozedMock(...args),
  snoozeUpsell: (...args: unknown[]) => snoozeUpsellMock(...args),
}));

vi.mock('@/utils/upgradePresentation', () => ({
  getProTrialStartedMessage: () => 'Pro trial started',
  getTrialAlreadyUsedMessage: () => 'Trial already used',
  getTrialStartErrorMessage: () => 'Trial start failed',
  getTrialTryAgainLaterMessage: () => 'Try again later',
}));

import { ActiveUseTrialNudge } from '@/components/shared/ActiveUseTrialNudge';

function setEligibleFreeLicense() {
  commercialPostureMock.mockReturnValue({
    tier: 'free',
    subscription_state: 'expired',
    trial_eligible: true,
  });
}

describe('ActiveUseTrialNudge', () => {
  beforeEach(() => {
    localStorage.clear();
    commercialPostureMock.mockReset();
    startProTrialMock.mockReset();
    showSuccessMock.mockReset();
    showErrorMock.mockReset();
    isUpsellSnoozedMock.mockReset();
    snoozeUpsellMock.mockReset();
    isUpsellSnoozedMock.mockReturnValue(false);
  });

  afterEach(() => {
    cleanup();
  });

  it('keeps active use trial nudge on shell, runtime, and model owners', () => {
    expect(activeUseTrialNudgeSource).toContain('useActiveUseTrialNudgeState');
    expect(activeUseTrialNudgeSource).toContain('ACTIVE_USE_TRIAL_NUDGE_TITLE');
    expect(activeUseTrialNudgeSource).not.toContain('createSignal');
    expect(activeUseTrialNudgeSource).not.toContain('createMemo');
    expect(activeUseTrialNudgeSource).not.toContain('startProTrial');
    expect(activeUseTrialNudgeSource).not.toContain('localStorage');
    expect(activeUseTrialNudgeSource).not.toContain('setInterval');

    expect(activeUseTrialNudgeStateSource).toContain('export function useActiveUseTrialNudgeState');
    expect(activeUseTrialNudgeStateSource).toContain('createSignal');
    expect(activeUseTrialNudgeStateSource).toContain('createMemo');
    expect(activeUseTrialNudgeStateSource).toContain('window.localStorage');
    expect(activeUseTrialNudgeStateSource).toContain('setInterval');
    expect(activeUseTrialNudgeStateSource).toContain('runStartProTrialAction');
    expect(activeUseTrialNudgeStateSource).not.toContain('startProTrial()');
    expect(activeUseTrialNudgeStateSource).toContain('snoozeUpsell');

    expect(activeUseTrialNudgeModelSource).toContain('ACTIVE_USE_TRIAL_NUDGE_SNOOZE_KEY');
    expect(activeUseTrialNudgeModelSource).toContain('ACTIVE_USE_TRIAL_NUDGE_FIRST_SEEN_KEY');
    expect(activeUseTrialNudgeModelSource).toContain('isActiveUseTrialNudgeEligible');
    expect(activeUseTrialNudgeModelSource).toContain('isActiveUseTrialNudgeOldEnough');
    expect(activeUseTrialNudgeModelSource).toContain('ACTIVE_USE_TRIAL_NUDGE_TITLE');
  });

  it('renders for eligible free users after the minimum active age', async () => {
    setEligibleFreeLicense();
    localStorage.setItem(
      ACTIVE_USE_TRIAL_NUDGE_FIRST_SEEN_KEY,
      String(Date.now() - ACTIVE_USE_TRIAL_NUDGE_MIN_AGE_MS - 1),
    );

    render(() => <ActiveUseTrialNudge />);

    expect(await screen.findByRole('status')).toBeInTheDocument();
    expect(screen.getByText('Experience the full power of Pulse — start your free trial')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Start 14-day trial' })).toBeInTheDocument();
  });

  it('does not render before the active age threshold is crossed', () => {
    setEligibleFreeLicense();
    render(() => <ActiveUseTrialNudge />);

    expect(screen.queryByRole('status')).toBeNull();
    expect(localStorage.getItem(ACTIVE_USE_TRIAL_NUDGE_FIRST_SEEN_KEY)).not.toBeNull();
  });

  it('snoozes and hides the nudge', async () => {
    setEligibleFreeLicense();
    localStorage.setItem(
      ACTIVE_USE_TRIAL_NUDGE_FIRST_SEEN_KEY,
      String(Date.now() - ACTIVE_USE_TRIAL_NUDGE_MIN_AGE_MS - 1),
    );

    render(() => <ActiveUseTrialNudge />);

    fireEvent.click(await screen.findByRole('button', { name: 'Snooze 7d' }));

    expect(snoozeUpsellMock).toHaveBeenCalledWith(ACTIVE_USE_TRIAL_NUDGE_SNOOZE_KEY);
    await waitFor(() => {
      expect(screen.queryByRole('status')).toBeNull();
    });
  });

  it('starts a trial successfully and shows a success notification', async () => {
    setEligibleFreeLicense();
    startProTrialMock.mockResolvedValue({ outcome: 'activated' });
    localStorage.setItem(
      ACTIVE_USE_TRIAL_NUDGE_FIRST_SEEN_KEY,
      String(Date.now() - ACTIVE_USE_TRIAL_NUDGE_MIN_AGE_MS - 1),
    );

    render(() => <ActiveUseTrialNudge />);

    fireEvent.click(await screen.findByRole('button', { name: 'Start 14-day trial' }));

    await waitFor(() => {
      expect(startProTrialMock).toHaveBeenCalledTimes(1);
    });
    expect(showSuccessMock).toHaveBeenCalledWith('Pro trial started');
  });
});
