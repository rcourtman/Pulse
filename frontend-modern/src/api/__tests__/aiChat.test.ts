import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
  apiFetch: vi.fn(),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  },
}));

import { AIChatAPI } from '@/api/aiChat';
import { apiFetch } from '@/utils/apiClient';

describe('AIChatAPI', () => {
  const apiFetchMock = vi.mocked(apiFetch);

  beforeEach(() => {
    apiFetchMock.mockReset();
  });

  it('clears read timeout timers during chat streaming', async () => {
    const releaseLock = vi.fn();
    const read = vi.fn().mockResolvedValue({ done: true, value: undefined });
    apiFetchMock.mockResolvedValueOnce({
      ok: true,
      body: { getReader: () => ({ read, releaseLock }) },
    } as any);

    const clearTimeoutSpy = vi.spyOn(globalThis, 'clearTimeout');
    const onEvent = vi.fn();

    await AIChatAPI.chat('hello', undefined, undefined, onEvent);

    expect(read).toHaveBeenCalledTimes(1);
    expect(clearTimeoutSpy).toHaveBeenCalledTimes(1);
    expect(releaseLock).toHaveBeenCalledTimes(1);
    expect(onEvent).toHaveBeenCalledWith({ type: 'done' });
    clearTimeoutSpy.mockRestore();
  });
});
