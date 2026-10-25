import { afterEach, describe, expect, it, vi } from 'vitest';
import { apiFetchJSON, clearApiToken, setOrgID } from '@/utils/apiClient';

afterEach(() => {
  vi.unstubAllGlobals();
  clearApiToken();
  setOrgID(null);
});

describe('configuration import post-commit failure transport', () => {
  it.each([
    'alert settings',
    'webhook settings',
    'email settings',
    'guest metadata',
    'webhook settings, email settings',
    'alert settings, webhook settings, email settings, guest metadata',
  ])('preserves the bounded %s error without repeating the import', async (components) => {
    const message = `Configuration saved, but live reload failed: ${components}. Check the destination before relying on it. Do not repeat the import.`;
    // Plain-text API errors are intentionally bounded. Even the combined
    // producer response, including http.Error's newline, must fit that limit.
    expect(`${message}\n`.length).toBeLessThan(200);
    const fetch = vi.fn().mockResolvedValue(
      new Response(`${message}\n`, {
        status: 500,
        headers: { 'Content-Type': 'text/plain; charset=utf-8' },
      }),
    );
    vi.stubGlobal('fetch', fetch);
    document.cookie = 'pulse_csrf=synthetic-import-csrf; Path=/';
    setOrgID('default');

    await expect(
      apiFetchJSON('/api/config/import', {
        method: 'POST',
        body: JSON.stringify({ data: 'synthetic-archive', passphrase: 'synthetic-passphrase' }),
        expectedOrgID: 'default',
        retry: false,
      }),
    ).rejects.toMatchObject({ message, status: 500 });
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(fetch.mock.calls[0][0]).toBe('/api/config/import');
    expect(fetch.mock.calls[0][1].method).toBe('POST');
  });
});
