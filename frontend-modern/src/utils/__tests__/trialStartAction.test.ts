import { beforeEach, describe, expect, it, vi } from 'vitest';

const startProTrialMock = vi.hoisted(() => vi.fn());

vi.mock('@/stores/licenseCommercial', () => ({
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

  it('invokes the optional error hook with the raw backend error payload', async () => {
    const onError = vi.fn();
    const error = {
      status: 409,
      code: 'trial_already_used',
      message: 'Trial already used',
    };
    startProTrialMock.mockRejectedValue(error);

    await expect(
      runStartProTrialAction({
        showSuccess,
        showError,
        navigate,
        onError,
      }),
    ).resolves.toBe('error');

    expect(onError).toHaveBeenCalledWith(error);
    expect(showError).toHaveBeenCalledWith('Trial already used');
  });

  it('surfaces retry-after guidance from the shared presentation helper', async () => {
    startProTrialMock.mockRejectedValue({
      status: 429,
      code: 'trial_rate_limited',
      message: 'Trial start rate limit exceeded',
      retryAfterSeconds: 120,
    });

    await expect(
      runStartProTrialAction({
        showSuccess,
        showError,
        navigate,
      }),
    ).resolves.toBe('error');

    expect(showError).toHaveBeenCalledWith('Try again in about 2 minutes');
    expect(showSuccess).not.toHaveBeenCalled();
  });
});
