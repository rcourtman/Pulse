import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetch } from '@/utils/apiClient';

const mockFetch = vi.fn();

describe('apiClient rate-limit handling', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-02-11T00:00:00Z'));
    mockFetch.mockReset();
    global.fetch = mockFetch as unknown as typeof fetch;
    document.cookie = 'pulse_csrf=test-token';
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('retries rate-limited idempotent requests once', async () => {
    mockFetch
      .mockResolvedValueOnce(
        new Response('rate limited', {
          status: 429,
          headers: { 'Retry-After': '1' },
        }),
      )
      .mockResolvedValueOnce(new Response('{}', { status: 200 }));

    const responsePromise = apiFetch('/api/updates/check');
    await vi.advanceTimersByTimeAsync(1000);
    const response = await responsePromise;

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(2);
  });

  it('does not retry non-idempotent rate-limited requests', async () => {
    mockFetch.mockResolvedValueOnce(
      new Response('rate limited', {
        status: 429,
        headers: { 'Retry-After': '1' },
      }),
    );

    const response = await apiFetch('/api/updates/apply', {
      method: 'POST',
      body: JSON.stringify({ downloadUrl: 'https://example.com/update.tar.gz' }),
    });

    expect(response.status).toBe(429);
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  it('parses Retry-After HTTP-date headers for idempotent retries', async () => {
    mockFetch
      .mockResolvedValueOnce(
        new Response('rate limited', {
          status: 429,
          headers: { 'Retry-After': 'Wed, 11 Feb 2026 00:00:03 GMT' },
        }),
      )
      .mockResolvedValueOnce(new Response('{}', { status: 200 }));

    const responsePromise = apiFetch('/api/updates/check');

    await vi.advanceTimersByTimeAsync(2999);
    expect(mockFetch).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(1);
    const response = await responsePromise;

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(2);
  });
});
