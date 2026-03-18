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
});
