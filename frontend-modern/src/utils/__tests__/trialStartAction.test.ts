import { beforeEach, describe, expect, it, vi } from 'vitest';

const startProTrialMock = vi.hoisted(() => vi.fn());

vi.mock('@/stores/license', () => ({
  startProTrial: (...args: unknown[]) => startProTrialMock(...args),
}));

import { runStartProTrialAction } from '@/utils/trialStartAction';

describe('runStartProTrialAction', () => {
  const showSuccess = vi.fn();
  const showError = vi.fn();
  const navigate = vi.fn();

  beforeEach(() => {
    startProTrialMock.mockReset();
    showSuccess.mockReset();
    showError.mockReset();
    navigate.mockReset();
  });

  it('reports activation success through the shared success message path', async () => {
    startProTrialMock.mockResolvedValue({ outcome: 'activated' });

    await expect(
      runStartProTrialAction({
        showSuccess,
        showError,
        navigate,
      }),
    ).resolves.toBe('activated');

    expect(showSuccess).toHaveBeenCalledWith('Pro trial started');
    expect(showError).not.toHaveBeenCalled();
    expect(navigate).not.toHaveBeenCalled();
  });

  it('navigates through the hosted handoff when the backend requires signup', async () => {
    startProTrialMock.mockResolvedValue({
      outcome: 'redirect',
      actionUrl: 'https://cloud.pulserelay.pro/start-pro-trial',
    });

    await expect(
      runStartProTrialAction({
        showSuccess,
        showError,
        navigate,
      }),
    ).resolves.toBe('redirect');

    expect(navigate).toHaveBeenCalledWith('https://cloud.pulserelay.pro/start-pro-trial');
    expect(showSuccess).not.toHaveBeenCalled();
    expect(showError).not.toHaveBeenCalled();
  });

  it('preserves canonical denial messages instead of collapsing conflicts locally', async () => {
    startProTrialMock.mockRejectedValue({
      status: 409,
      code: 'trial_not_available',
      message: 'Trial cannot be started while a paid v5 license migration is pending',
    });

    await expect(
      runStartProTrialAction({
        showSuccess,
        showError,
        navigate,
      }),
    ).resolves.toBe('error');

    expect(showError).toHaveBeenCalledWith(
      'Trial cannot be started while a paid v5 license migration is pending',
    );
    expect(showSuccess).not.toHaveBeenCalled();
    expect(navigate).not.toHaveBeenCalled();
  });
});
