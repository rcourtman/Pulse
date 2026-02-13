import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetch, getOrgID, setOrgID } from '@/utils/apiClient';

const mockFetch = vi.fn();

describe('apiClient org context', () => {
  beforeEach(() => {
    vi.useRealTimers();
    mockFetch.mockReset();
    global.fetch = mockFetch as unknown as typeof fetch;
    window.sessionStorage.clear();
    setOrgID(null);
  });

  afterEach(() => {
    vi.useRealTimers();
    setOrgID(null);
  });

  it('propagates selected org via X-Pulse-Org-ID header', async () => {
    mockFetch.mockResolvedValue(new Response('{}', { status: 200 }));

    setOrgID('acme');
    await apiFetch('/api/state');

    const [, options] = mockFetch.mock.calls[0] as [string, RequestInit];
    const headers = options.headers as Record<string, string>;
    expect(headers['X-Pulse-Org-ID']).toBe('acme');
    expect(getOrgID()).toBe('acme');
    expect(window.sessionStorage.getItem('pulse_org_id')).toBe('acme');
  });

  it('uses default org context when skipOrgContext is enabled', async () => {
    mockFetch.mockResolvedValue(new Response('[]', { status: 200 }));

    setOrgID('acme');
    await apiFetch('/api/orgs', { skipOrgContext: true });

    const [, options] = mockFetch.mock.calls[0] as [string, RequestInit];
    const headers = options.headers as Record<string, string>;
    expect(headers['X-Pulse-Org-ID']).toBe('default');
  });

  it('clears stale org context and retries when backend returns invalid_org', async () => {
    mockFetch
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ error: 'invalid_org', message: 'Invalid Organization ID' }), {
          status: 400,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
      .mockResolvedValueOnce(new Response('{}', { status: 200 }));

    setOrgID('missing-org');
    await apiFetch('/api/state');

    expect(mockFetch).toHaveBeenCalledTimes(2);

    const [, firstOptions] = mockFetch.mock.calls[0] as [string, RequestInit];
    const firstHeaders = firstOptions.headers as Record<string, string>;
    expect(firstHeaders['X-Pulse-Org-ID']).toBe('missing-org');

    const [, secondOptions] = mockFetch.mock.calls[1] as [string, RequestInit];
    const secondHeaders = secondOptions.headers as Record<string, string>;
    expect(secondHeaders['X-Pulse-Org-ID']).toBeUndefined();
    expect(getOrgID()).toBeNull();
  });

<<<<<<< HEAD
  it('retries 429 responses using numeric Retry-After seconds', async () => {
    vi.useFakeTimers();
    mockFetch
      .mockResolvedValueOnce(new Response('{}', { status: 429, headers: { 'Retry-After': '3' } }))
      .mockResolvedValueOnce(new Response('{}', { status: 200 }));

    const request = apiFetch('/api/state');
    expect(mockFetch).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(2999);
    expect(mockFetch).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(1);
    await request;
    expect(mockFetch).toHaveBeenCalledTimes(2);
  });

  it('retries 429 responses using HTTP-date Retry-After values', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-01-01T00:00:00Z'));
    const retryAt = new Date(Date.now() + 5000).toUTCString();

    mockFetch
      .mockResolvedValueOnce(new Response('{}', { status: 429, headers: { 'Retry-After': retryAt } }))
      .mockResolvedValueOnce(new Response('{}', { status: 200 }));

    const request = apiFetch('/api/state');
    expect(mockFetch).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(4999);
    expect(mockFetch).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(1);
    await request;
    expect(mockFetch).toHaveBeenCalledTimes(2);
  });

  it('falls back to default retry delay when Retry-After is invalid', async () => {
    vi.useFakeTimers();
    mockFetch
      .mockResolvedValueOnce(new Response('{}', { status: 429, headers: { 'Retry-After': 'not-a-date' } }))
      .mockResolvedValueOnce(new Response('{}', { status: 200 }));

    const request = apiFetch('/api/state');
    expect(mockFetch).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(1999);
    expect(mockFetch).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(1);
    await request;
=======
  it('honors HTTP-date Retry-After before retrying a 429 response', async () => {
    vi.useFakeTimers();
    const retryAfter = new Date(Date.now() + 5000).toUTCString();

    mockFetch
      .mockResolvedValueOnce(new Response('rate limited', { status: 429, headers: { 'Retry-After': retryAfter } }))
      .mockResolvedValueOnce(new Response('{}', { status: 200 }));

    const pending = apiFetch('/api/state');
    expect(mockFetch).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(2500);
    expect(mockFetch).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(5000);
    await pending;
>>>>>>> refactor/parallel-44-circuit-breakers
    expect(mockFetch).toHaveBeenCalledTimes(2);
  });
});
