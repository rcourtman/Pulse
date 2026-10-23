import { afterEach, describe, expect, it, vi } from 'vitest';

const { getStatusMock, loggerErrorMock } = vi.hoisted(() => ({
  getStatusMock: vi.fn(),
  loggerErrorMock: vi.fn(),
}));

vi.mock('@/api/security', () => ({
  SecurityAPI: { getStatus: getStatusMock },
}));

vi.mock('@/utils/logger', () => ({
  logger: { error: loggerErrorMock, warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

import { aiAvailabilityStore } from '@/stores/aiAvailability';

describe('aiAvailabilityStore', () => {
  afterEach(() => {
    getStatusMock.mockReset();
    loggerErrorMock.mockReset();
  });

  it('stays unknown until the first security status sets it', () => {
    expect(aiAvailabilityStore.enabled).toBeNull();
    aiAvailabilityStore.setEnabled(true);
    expect(aiAvailabilityStore.enabled).toBe(true);
    aiAvailabilityStore.setEnabled(false);
    expect(aiAvailabilityStore.enabled).toBe(false);
  });

  it('refreshes from the assistantEnabled session capability', async () => {
    aiAvailabilityStore.setEnabled(false);
    getStatusMock.mockResolvedValueOnce({ sessionCapabilities: { assistantEnabled: true } });
    await aiAvailabilityStore.refreshEnabledFromServer();
    expect(aiAvailabilityStore.enabled).toBe(true);

    getStatusMock.mockResolvedValueOnce({ sessionCapabilities: {} });
    await aiAvailabilityStore.refreshEnabledFromServer();
    expect(aiAvailabilityStore.enabled).toBe(false);
  });

  it('keeps the last known value and logs when the refresh fails', async () => {
    aiAvailabilityStore.setEnabled(true);
    getStatusMock.mockRejectedValueOnce(new Error('offline'));
    await aiAvailabilityStore.refreshEnabledFromServer();
    expect(aiAvailabilityStore.enabled).toBe(true);
    expect(loggerErrorMock).toHaveBeenCalledOnce();
  });

  it('keeps the newest answer when an older refresh lands late', async () => {
    let answerFirst: (value: unknown) => void = () => {};
    getStatusMock
      .mockReturnValueOnce(
        new Promise((resolve) => {
          answerFirst = resolve;
        }),
      )
      .mockResolvedValueOnce({ sessionCapabilities: { assistantEnabled: true } });

    // Organization A, then B before A answers.
    const first = aiAvailabilityStore.refreshEnabledFromServer();
    await aiAvailabilityStore.refreshEnabledFromServer();
    expect(aiAvailabilityStore.enabled).toBe(true);

    answerFirst({ sessionCapabilities: { assistantEnabled: false } });
    await first;
    expect(aiAvailabilityStore.enabled).toBe(true);
  });

  it('forgets the answer on clear and ignores a refresh that started before it', async () => {
    let answerOld: (value: unknown) => void = () => {};
    getStatusMock.mockReturnValueOnce(
      new Promise((resolve) => {
        answerOld = resolve;
      }),
    );
    aiAvailabilityStore.setEnabled(true);
    const stale = aiAvailabilityStore.refreshEnabledFromServer();
    aiAvailabilityStore.clear();
    expect(aiAvailabilityStore.enabled).toBeNull();

    answerOld({ sessionCapabilities: { assistantEnabled: true } });
    await stale;
    expect(aiAvailabilityStore.enabled).toBeNull();
  });
});
