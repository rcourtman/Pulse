import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  apiFetch,
  apiFetchJSON,
  apiErrorFromResponse,
  clearApiToken,
  clearAuth,
  getApiToken,
  getOrgID,
  hasAuth,
  setApiToken,
  setOrgID,
} from '@/utils/apiClient';

const mockFetch = vi.fn();

const headersOf = (call: unknown): Record<string, string> => {
  const [, options] = call as [string, RequestInit];
  return options.headers as Record<string, string>;
};

const clearCsrfCookie = () => {
  document.cookie = 'pulse_csrf=; Path=/; Max-Age=0';
};

const clearSessionCookies = () => {
  document.cookie = 'pulse_session=; Path=/; Max-Age=0';
  document.cookie = '__Host-pulse_session=; Path=/; Max-Age=0';
};

describe('apiClient hasAuth', () => {
  beforeEach(() => {
    mockFetch.mockReset();
    global.fetch = mockFetch as unknown as typeof fetch;
    window.sessionStorage.clear();
    clearAuth();
    setOrgID(null);
    clearCsrfCookie();
    clearSessionCookies();
  });

  afterEach(() => {
    clearAuth();
    setOrgID(null);
    clearCsrfCookie();
    clearSessionCookies();
  });

  it('returns true when an API token is configured', () => {
    setApiToken('a-valid-token');
    expect(hasAuth()).toBe(true);
  });

  it('returns true when a pulse_session cookie is present and no token is set', () => {
    clearApiToken();
    document.cookie = 'pulse_session=abc; Path=/';
    expect(hasAuth()).toBe(true);
  });

  it('returns true when a __Host-pulse_session cookie is present', () => {
    clearApiToken();
    // jsdom's cookie jar drops __Host- prefixed cookies over http:, so feed the
    // cookie string directly to exercise the name-matching arm against real parsing.
    const spy = vi
      .spyOn(document, 'cookie', 'get')
      .mockReturnValue('pulse_csrf=x; __Host-pulse_session=abc');
    expect(hasAuth()).toBe(true);
    spy.mockRestore();
  });

  it('returns false when neither token nor a session cookie is configured', () => {
    clearApiToken();
    expect(hasAuth()).toBe(false);
  });
});

describe('apiClient CSRF / 403 retry arms', () => {
  beforeEach(() => {
    mockFetch.mockReset();
    global.fetch = mockFetch as unknown as typeof fetch;
    window.sessionStorage.clear();
    clearAuth();
    setOrgID(null);
    clearCsrfCookie();
  });

  afterEach(() => {
    clearAuth();
    setOrgID(null);
    clearCsrfCookie();
  });

  it('retries a 403 using the X-CSRF-Token response header', async () => {
    document.cookie = 'pulse_csrf=initial-token; Path=/';
    mockFetch
      .mockResolvedValueOnce(
        new Response('forbidden', {
          status: 403,
          headers: { 'X-CSRF-Token': 'refreshed-via-header' },
        }),
      )
      .mockResolvedValueOnce(new Response('{}', { status: 200 }));

    const response = await apiFetch('/api/data', { method: 'POST', body: '{}' });

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(headersOf(mockFetch.mock.calls[1])['X-CSRF-Token']).toBe('refreshed-via-header');
  });

  it('reloads the CSRF token from a newly-set cookie when the 403 has no X-CSRF-Token header', async () => {
    document.cookie = 'pulse_csrf=initial-token; Path=/';
    let firstCall = true;
    mockFetch.mockImplementation(async () => {
      if (firstCall) {
        firstCall = false;
        // Simulate the backend issuing a fresh CSRF cookie on the 403.
        document.cookie = 'pulse_csrf=new-from-cookie; Path=/';
        return new Response('forbidden', { status: 403 });
      }
      return new Response('{}', { status: 200 });
    });

    const response = await apiFetch('/api/data', { method: 'POST', body: '{}' });

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(headersOf(mockFetch.mock.calls[1])['X-CSRF-Token']).toBe('new-from-cookie');
  });

  it('returns the original 403 response when no refreshed token can be obtained', async () => {
    // No CSRF cookie anywhere, and ensureCSRFToken() is a no-op in test mode.
    mockFetch.mockResolvedValueOnce(new Response('forbidden', { status: 403 }));

    const response = await apiFetch('/api/data', { method: 'POST', body: '{}' });

    expect(response.status).toBe(403);
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  it('omits the X-CSRF-Token header on state-changing requests when no token is available', async () => {
    mockFetch.mockResolvedValue(new Response('{}', { status: 200 }));

    await apiFetch('/api/data', { method: 'POST', body: '{}' });

    expect(headersOf(mockFetch.mock.calls[0])['X-CSRF-Token']).toBeUndefined();
  });

  it('treats an un-decodable CSRF cookie as no token', async () => {
    document.cookie = 'pulse_csrf=%; Path=/';
    mockFetch.mockResolvedValue(new Response('{}', { status: 200 }));

    await apiFetch('/api/data', { method: 'POST', body: '{}' });

    expect(headersOf(mockFetch.mock.calls[0])['X-CSRF-Token']).toBeUndefined();
  });
});

describe('apiClient 401 handling arms', () => {
  beforeEach(() => {
    mockFetch.mockReset();
    global.fetch = mockFetch as unknown as typeof fetch;
    window.sessionStorage.clear();
    window.localStorage.clear();
    clearAuth();
    setOrgID(null);
    clearCsrfCookie();
    clearSessionCookies();
  });

  afterEach(() => {
    clearAuth();
    setOrgID(null);
    window.localStorage.clear();
    clearCsrfCookie();
    clearSessionCookies();
  });

  it('clears auth and signals logout on a 401 from a non-exempt API endpoint', async () => {
    setApiToken('some-token');
    const hrefSetter = vi.fn();
    const originalDesc = Object.getOwnPropertyDescriptor(window, 'location');
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: {
        set href(v: string) {
          hrefSetter(v);
        },
        get href() {
          return '/';
        },
        pathname: '/',
        protocol: 'http:',
        search: '',
        toString: () => 'http://localhost/',
      },
    });

    try {
      mockFetch.mockResolvedValueOnce(new Response('unauthorized', { status: 401 }));

      const response = await apiFetch('/api/dashboard');

      expect(response.status).toBe(401);
      expect(hrefSetter).toHaveBeenCalledWith('/');
      expect(window.localStorage.getItem('just_logged_out')).toBe('true');
      expect(getApiToken()).toBeNull();
    } finally {
      if (originalDesc) {
        Object.defineProperty(window, 'location', originalDesc);
      }
    }
  });

  it('does not redirect when a 401 comes from an exempt endpoint', async () => {
    mockFetch.mockResolvedValueOnce(new Response('unauthorized', { status: 401 }));

    const response = await apiFetch('/api/security/status');

    expect(response.status).toBe(401);
    expect(window.localStorage.getItem('just_logged_out')).toBeNull();
  });
});

describe('apiClient org-scoping header arms', () => {
  beforeEach(() => {
    mockFetch.mockReset();
    global.fetch = mockFetch as unknown as typeof fetch;
    window.sessionStorage.clear();
    clearAuth();
    setOrgID(null);
    clearCsrfCookie();
  });

  afterEach(() => {
    clearAuth();
    setOrgID(null);
  });

  it('does not add API-specific headers for non-/api/ URLs', async () => {
    mockFetch.mockResolvedValue(new Response('{}', { status: 200 }));
    setOrgID('acme');

    await apiFetch('https://example.com/external');

    const headers = headersOf(mockFetch.mock.calls[0]);
    expect(headers['X-Pulse-Org-ID']).toBeUndefined();
    expect(headers['X-Requested-With']).toBeUndefined();
    expect(headers['Accept']).toBeUndefined();
  });

  it('preserves a caller-supplied org header even when skipOrgContext is true', async () => {
    mockFetch.mockResolvedValue(new Response('{}', { status: 200 }));

    await apiFetch('/api/data', {
      skipOrgContext: true,
      headers: { 'X-Pulse-Org-ID': 'custom-org' },
    });

    expect(headersOf(mockFetch.mock.calls[0])['X-Pulse-Org-ID']).toBe('custom-org');
  });

  it('defaults Accept to application/json for API requests when not supplied', async () => {
    mockFetch.mockResolvedValue(new Response('{}', { status: 200 }));

    await apiFetch('/api/data');

    expect(headersOf(mockFetch.mock.calls[0])['Accept']).toBe('application/json');
  });

  it('preserves a caller-supplied Accept header on API requests', async () => {
    mockFetch.mockResolvedValue(new Response('{}', { status: 200 }));

    await apiFetch('/api/data', { headers: { Accept: 'text/event-stream' } });

    expect(headersOf(mockFetch.mock.calls[0])['Accept']).toBe('text/event-stream');
  });

  it('does not retry a 400 that is not an invalid_org error', async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ error: 'validation_failed' }), {
        status: 400,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    setOrgID('acme');

    const response = await apiFetch('/api/data', { method: 'POST', body: '{}' });

    expect(response.status).toBe(400);
    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(getOrgID()).toBe('acme');
  });

  it('does not retry an invalid_org 400 when org context is skipped (default org)', async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ error: 'invalid_org' }), {
        status: 400,
        headers: { 'Content-Type': 'application/json' },
      }),
    );

    const response = await apiFetch('/api/data', {
      method: 'POST',
      body: '{}',
      skipOrgContext: true,
    });

    expect(response.status).toBe(400);
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  it('does not retry a 400 invalid_org response from a non-/api/ URL', async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ error: 'invalid_org' }), { status: 400 }),
    );

    const response = await apiFetch('https://example.com/api/data', { method: 'POST' });

    expect(response.status).toBe(400);
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  it('does not retry a 400 invalid_org response when a non-string body fails to parse', async () => {
    mockFetch.mockResolvedValueOnce(new Response('not-json', { status: 400 }));
    setOrgID('acme');

    const response = await apiFetch('/api/data', { method: 'POST', body: '{}' });

    expect(response.status).toBe(400);
    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(getOrgID()).toBe('acme');
  });
});

describe('apiClient fetchJSON serialization branches', () => {
  beforeEach(() => {
    mockFetch.mockReset();
    global.fetch = mockFetch as unknown as typeof fetch;
    window.sessionStorage.clear();
    clearAuth();
    setOrgID(null);
    clearCsrfCookie();
  });

  afterEach(() => {
    clearAuth();
    setOrgID(null);
  });

  it('returns null when the JSON response body is empty', async () => {
    mockFetch.mockResolvedValue(new Response('', { status: 200 }));

    const result = await apiFetchJSON('/api/data');

    expect(result).toBeNull();
  });

  it('throws an Invalid JSON response error when the body is not parseable', async () => {
    mockFetch.mockResolvedValue(
      new Response('not-json-at-all', {
        status: 200,
        headers: { 'Content-Type': 'text/plain' },
      }),
    );

    await expect(apiFetchJSON('/api/data')).rejects.toThrow('Invalid JSON response from server');
  });

  it('lets the caller override the Content-Type header', async () => {
    mockFetch.mockResolvedValue(new Response('{}', { status: 200 }));

    await apiFetchJSON('/api/data', { headers: { 'Content-Type': 'text/plain' } });

    expect(headersOf(mockFetch.mock.calls[0])['Content-Type']).toBe('text/plain');
  });
});

describe('apiClient 429 Retry-After parsing arms', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockFetch.mockReset();
    global.fetch = mockFetch as unknown as typeof fetch;
    window.sessionStorage.clear();
    clearAuth();
    setOrgID(null);
    clearCsrfCookie();
  });

  afterEach(() => {
    vi.useRealTimers();
    clearAuth();
    setOrgID(null);
  });

  const retryAfterDefaultWait = async (retryAfterValue: string | null, systemTime?: Date) => {
    if (systemTime) vi.setSystemTime(systemTime);
    const headers: Record<string, string> = {};
    if (retryAfterValue !== null) headers['Retry-After'] = retryAfterValue;
    mockFetch
      .mockResolvedValueOnce(new Response('rate limited', { status: 429, headers }))
      .mockResolvedValueOnce(new Response('{}', { status: 200 }));

    const pending = apiFetch('/api/data');
    await Promise.resolve();
    await vi.advanceTimersByTimeAsync(999);
    expect(mockFetch, 'should not retry before the 1s default wait').toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(1);
    const response = await pending;
    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(2);
  };

  it('uses the 1s default wait when no Retry-After header is present', async () => {
    await retryAfterDefaultWait(null);
  });

  it('uses the 1s default wait when Retry-After is only whitespace', async () => {
    await retryAfterDefaultWait('   ');
  });

  it('uses the 1s default wait when Retry-After is non-numeric and not a date', async () => {
    await retryAfterDefaultWait('banana');
  });

  it('uses the 1s default wait when numeric Retry-After exceeds the 120s cap', async () => {
    await retryAfterDefaultWait('200');
  });

  it('uses the 1s default wait when the HTTP-date Retry-After is in the past', async () => {
    await retryAfterDefaultWait('Tue, 10 Feb 2026 00:00:00 GMT', new Date('2026-02-11T00:00:00Z'));
  });

  it('uses the 1s default wait when the HTTP-date Retry-After is beyond the 120s cap', async () => {
    await retryAfterDefaultWait('Wed, 11 Feb 2026 00:05:00 GMT', new Date('2026-02-11T00:00:00Z'));
  });
});

describe('apiClient abort handling', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockFetch.mockReset();
    global.fetch = mockFetch as unknown as typeof fetch;
    window.sessionStorage.clear();
    clearAuth();
    setOrgID(null);
    clearCsrfCookie();
  });

  afterEach(() => {
    vi.useRealTimers();
    clearAuth();
    setOrgID(null);
  });

  it('rejects with AbortError without retrying when the signal is already aborted before the wait', async () => {
    mockFetch.mockResolvedValueOnce(
      new Response('{}', { status: 429, headers: { 'Retry-After': '1' } }),
    );

    const controller = new AbortController();
    controller.abort();

    await expect(apiFetch('/api/data', { signal: controller.signal })).rejects.toMatchObject({
      name: 'AbortError',
    });

    expect(mockFetch).toHaveBeenCalledTimes(1);
  });
});

describe('apiClient org cookie decoding', () => {
  beforeEach(() => {
    mockFetch.mockReset();
    global.fetch = mockFetch as unknown as typeof fetch;
    window.sessionStorage.clear();
    clearAuth();
    setOrgID(null);
    clearCsrfCookie();
    document.cookie = 'pulse_org_id=; Path=/; Max-Age=0';
  });

  afterEach(() => {
    clearAuth();
    setOrgID(null);
    document.cookie = 'pulse_org_id=; Path=/; Max-Age=0';
  });

  it('returns null when the org cookie cannot be URL-decoded', () => {
    document.cookie = 'pulse_org_id=%; Path=/';
    expect(getOrgID()).toBeNull();
  });
});

describe('apiClient stored-auth size limits', () => {
  beforeEach(() => {
    window.sessionStorage.clear();
    clearAuth();
    setOrgID(null);
  });

  afterEach(() => {
    clearAuth();
    setOrgID(null);
    window.sessionStorage.clear();
  });

  it('discards an oversized stored auth payload on getApiToken', () => {
    clearApiToken();
    window.sessionStorage.setItem('pulse_auth', 'x'.repeat(17 * 1024));

    expect(getApiToken()).toBeNull();
    expect(window.sessionStorage.getItem('pulse_auth')).toBeNull();
  });

  it('drops an oversized pulse_auth payload during construction', async () => {
    window.sessionStorage.setItem('pulse_auth', 'x'.repeat(17 * 1024));
    vi.resetModules();
    const mod = await import('@/utils/apiClient');

    expect(mod.getApiToken()).toBeNull();
    expect(window.sessionStorage.getItem('pulse_auth')).toBeNull();
  });
});

describe('apiClient kiosk URL-token loading', () => {
  beforeEach(() => {
    window.sessionStorage.clear();
    window.history.replaceState({}, '', '/');
  });

  afterEach(() => {
    window.history.replaceState({}, '', '/');
    window.sessionStorage.clear();
  });

  it('loads an API token from a ?token= URL parameter and strips it from the URL', async () => {
    window.history.replaceState({}, '', '/?token=url-token&keep=1');
    vi.resetModules();
    const mod = await import('@/utils/apiClient');

    expect(mod.getApiToken()).toBe('url-token');
    expect(window.location.search).toBe('?keep=1');
  });

  it('strips an invalid ?token= value without storing it', async () => {
    // %0A decodes to a newline (control char) which sanitization rejects.
    window.history.replaceState({}, '', '/?token=bad%0Atoken');
    vi.resetModules();
    const mod = await import('@/utils/apiClient');

    expect(mod.getApiToken()).toBeNull();
    expect(window.location.search).toBe('');
  });
});

describe('apiClient structured error extraction arms', () => {
  it('extracts the first <pre> block from an HTML error response', async () => {
    const error = await apiErrorFromResponse(
      new Response('<html><body><pre>ValueError: bad input</pre></body></html>', { status: 500 }),
    );

    expect(error.message).toBe('ValueError: bad input');
    expect(error.status).toBe(500);
  });

  it('uses a status fallback message when the body is empty and no fallback is provided', async () => {
    const error = await apiErrorFromResponse(new Response('', { status: 502 }));

    expect(error.message).toBe('Request failed with status 502');
    expect(error.status).toBe(502);
  });

  it('preserves upgrade_url from structured commercial errors', async () => {
    const error = await apiErrorFromResponse(
      new Response(
        JSON.stringify({
          error: 'license_required',
          upgrade_url: 'https://shop.example.com/upgrade',
        }),
        { status: 402 },
      ),
    );

    expect(error.upgrade_url).toBe('https://shop.example.com/upgrade');
  });

  it('preserves a string detail field from structured errors', async () => {
    const error = await apiErrorFromResponse(
      new Response(JSON.stringify({ error: 'bad', detail: 'specific detail' }), { status: 400 }),
    );

    expect(error.detail).toBe('specific detail');
  });

  it('ignores non-string detail fields', async () => {
    const error = await apiErrorFromResponse(
      new Response(JSON.stringify({ error: 'bad', detail: { nested: true } }), { status: 400 }),
    );

    expect(error.detail).toBeUndefined();
  });

  it('falls back to the provided message when both message and error are non-strings', async () => {
    const error = await apiErrorFromResponse(
      new Response(JSON.stringify({ error: { nested: true } }), { status: 400 }),
      'Fallback message',
    );

    expect(error.message).toBe('Fallback message');
  });

  it('drops error details when every entry fails sanitization', async () => {
    const error = await apiErrorFromResponse(
      new Response(JSON.stringify({ error: 'boom', details: { '': 'x' } }), { status: 400 }),
    );

    expect(error.details).toBeUndefined();
  });

  it('rejects error codes containing control characters', async () => {
    const error = await apiErrorFromResponse(
      new Response(JSON.stringify({ error: 'bad', code: 'bad\ncode' }), { status: 400 }),
    );

    expect(error.code).toBeUndefined();
  });

  it('omits all optional error fields when the response has none', async () => {
    const error = await apiErrorFromResponse(new Response('oops', { status: 500 }));

    expect(error.code).toBeUndefined();
    expect(error.detail).toBeUndefined();
    expect(error.details).toBeUndefined();
    expect(error.feature).toBeUndefined();
    expect(error.requiredScope).toBeUndefined();
    expect(error.upgrade_url).toBeUndefined();
  });
});
