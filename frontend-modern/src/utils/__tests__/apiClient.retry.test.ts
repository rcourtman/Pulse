import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetch, clearAuth, setOrgID } from '@/utils/apiClient';

const mockFetch = vi.fn();

describe('apiClient rate-limit retry handling', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockFetch.mockReset();
    global.fetch = mockFetch as unknown as typeof fetch;
    window.sessionStorage.clear();
    clearAuth();
    setOrgID(null);
  });

  afterEach(() => {
    vi.useRealTimers();
    clearAuth();
    setOrgID(null);
  });

  it('retries once after Retry-After delay', async () => {
    mockFetch
      .mockResolvedValueOnce(new Response('{}', { status: 429, headers: { 'Retry-After': '1' } }))
      .mockResolvedValueOnce(new Response('{}', { status: 200 }));

    const pending = apiFetch('/api/test-rate-limit');
    await Promise.resolve();

    expect(mockFetch).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(999);
    expect(mockFetch).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(1);
    const response = await pending;

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(2);
  });

  it('does not retry when aborted during Retry-After wait', async () => {
    mockFetch.mockResolvedValueOnce(
      new Response('{}', { status: 429, headers: { 'Retry-After': '30' } }),
    );

    const controller = new AbortController();
    const pending = apiFetch('/api/test-rate-limit-abort', { signal: controller.signal });
    await Promise.resolve();

    expect(mockFetch).toHaveBeenCalledTimes(1);

    controller.abort();

    await expect(pending).rejects.toMatchObject({ name: 'AbortError' });

    await vi.advanceTimersByTimeAsync(30_000);
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });
});
