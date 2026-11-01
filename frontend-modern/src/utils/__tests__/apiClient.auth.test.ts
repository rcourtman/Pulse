import { beforeEach, describe, expect, it, vi } from 'vitest';

describe('apiClient auth storage integrity', () => {
  beforeEach(() => {
    vi.resetModules();
    window.sessionStorage.clear();
  });

  it('ignores malformed pulse_auth token payloads', async () => {
    window.sessionStorage.setItem(
      'pulse_auth',
      JSON.stringify({ type: 'token', value: { nested: true } }),
    );

    const { getApiToken } = await import('@/utils/apiClient');

    expect(getApiToken()).toBeNull();
    expect(window.sessionStorage.getItem('pulse_auth')).toBeNull();
  });

  it('does not use removed legacy token key when pulse_auth is invalid JSON', async () => {
    window.sessionStorage.setItem('pulse_auth', '{invalid-json');
    window.sessionStorage.setItem('pulse_api_token', 'legacy-token');

    const { getApiToken } = await import('@/utils/apiClient');

    expect(getApiToken()).toBeNull();
    expect(window.sessionStorage.getItem('pulse_api_token')).toBe('legacy-token');
    expect(window.sessionStorage.getItem('pulse_auth')).toBeNull();
  });
  it('returns a rejected logout without clearing the credential or disguising it as a login redirect', async () => {
    const { apiFetch, getApiToken, setApiToken, setOrgID } = await import('@/utils/apiClient');
    setApiToken('synthetic-independent-api-token');
    setOrgID('synthetic-org');
    document.cookie = 'pulse_csrf=synthetic-csrf; path=/';
    const fetchMock = vi.fn().mockResolvedValue(new Response('{}', { status: 401 }));
    vi.stubGlobal('fetch', fetchMock);
    try {
      const response = await apiFetch('/api/logout', {
        method: 'POST',
        retry: false,
        skipOrgContext: true,
        redirect: 'error',
      });
      expect(response.status).toBe(401);
      expect(fetchMock).toHaveBeenCalledOnce();
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/logout',
        expect.objectContaining({
          method: 'POST',
          credentials: 'include',
          redirect: 'error',
          headers: expect.objectContaining({
            'X-API-Token': 'synthetic-independent-api-token',
            'X-CSRF-Token': 'synthetic-csrf',
            'X-Pulse-Org-ID': 'default',
          }),
        }),
      );
      expect(getApiToken()).toBe('synthetic-independent-api-token');
    } finally {
      vi.unstubAllGlobals();
      document.cookie = 'pulse_csrf=; Max-Age=0; path=/';
    }
  });
});
