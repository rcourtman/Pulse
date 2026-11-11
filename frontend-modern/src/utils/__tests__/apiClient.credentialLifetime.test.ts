import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AlertsAPI } from '@/api/alerts';
import {
  apiFetch,
  clearApiToken,
  clearAuth,
  getApiToken,
  getOrgID,
  setApiToken,
  setOrgID,
} from '@/utils/apiClient';

const originalToken = 'synthetic-original-token';
const replacementToken = 'synthetic-replacement-token';
const mockFetch = vi.fn<typeof fetch>();
const redirect = vi.fn();
const locationDescriptor = Object.getOwnPropertyDescriptor(window, 'location');
const deferred = () => {
  let resolve!: (response: Response) => void;
  const promise = new Promise<Response>((done) => {
    resolve = done;
  });
  return { promise, resolve };
};
const response = (status: number) =>
  new Response(JSON.stringify({ error: 'synthetic-refusal' }), {
    status,
    headers: { 'X-CSRF-Token': 'synthetic-new-csrf', 'Retry-After': '1' },
  });
const expectRetained = (token: string | null = replacementToken, org: string | null = 'north') => {
  expect(getApiToken()).toBe(token);
  expect(getOrgID()).toBe(org);
  expect(sessionStorage.getItem('pulse_org_id')).toBe(org);
  if (token) expect(JSON.parse(sessionStorage.getItem('pulse_auth')!).value).toBe(token);
  else expect(sessionStorage.getItem('pulse_auth')).toBeNull();
  expect(localStorage.getItem('just_logged_out')).toBeNull();
  expect(redirect).not.toHaveBeenCalled();
};

beforeEach(() => {
  mockFetch.mockReset();
  redirect.mockReset();
  vi.stubGlobal('fetch', mockFetch);
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: {
      get href() {
        return 'http://localhost/settings/security';
      },
      set href(value: string) {
        redirect(value);
      },
      pathname: '/settings/security',
      protocol: 'http:',
      search: '',
    },
  });
  clearAuth();
  sessionStorage.clear();
  localStorage.clear();
  setApiToken(originalToken);
  setOrgID('north');
  document.cookie = 'pulse_csrf=synthetic-csrf; Path=/';
});
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
  clearAuth();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  if (locationDescriptor) Object.defineProperty(window, 'location', locationDescriptor);
});

describe('explicit credential ownership for ordinary API requests', () => {
  it.each([
    {
      name: 'replacement token',
      change: () => setApiToken(replacementToken),
      token: replacementToken,
      org: 'north',
    },
    { name: 'token cleared for a new session', change: clearApiToken, token: null, org: 'north' },
    {
      name: 'explicit same-value credential admission',
      change: () => setApiToken(originalToken),
      token: originalToken,
      org: 'north',
    },
    { name: 'explicit sign-out', change: clearAuth, token: null, org: null },
  ])('does not apply a queued old401 after $name', async ({ change, token, org }) => {
    const held = deferred();
    const rejected = response(401);
    mockFetch.mockReturnValueOnce(held.promise);
    const pending = apiFetch('/api/alerts/active');
    expect(mockFetch.mock.calls[0][1]?.headers).toMatchObject({ 'X-API-Token': originalToken });
    held.resolve(rejected);
    change();
    expect(await pending).toBe(rejected);
    expect(mockFetch).toHaveBeenCalledOnce();
    expectRetained(token, org);
  });

  it('returns the real AlertsAPI rejection without erasing replacement auth', async () => {
    const held = deferred();
    mockFetch.mockReturnValueOnce(held.promise);
    const pending = AlertsAPI.getActive().catch((error: unknown) => error);
    setApiToken(replacementToken);
    held.resolve(response(401));
    expect(await pending).toMatchObject({ status: 401 });
    expect(mockFetch).toHaveBeenCalledOnce();
    expectRetained();
  });

  it('does not acquire replacement authority through session-token fallback', async () => {
    const held = deferred();
    mockFetch.mockReturnValueOnce(held.promise);
    const pending = apiFetch('/api/orgs', { preferSessionAuth: true });
    expect(mockFetch.mock.calls[0][1]?.headers).not.toHaveProperty('X-API-Token');
    setApiToken(replacementToken);
    const rejected = response(401);
    held.resolve(rejected);
    expect(await pending).toBe(rejected);
    expect(mockFetch).toHaveBeenCalledOnce();
    expectRetained();
  });

  it('rechecks ownership after an already-dispatched token fallback', async () => {
    const fallback = deferred();
    mockFetch.mockResolvedValueOnce(response(401)).mockReturnValueOnce(fallback.promise);
    const pending = apiFetch('/api/orgs', { preferSessionAuth: true });
    await vi.waitFor(() => expect(mockFetch).toHaveBeenCalledTimes(2));
    expect(mockFetch.mock.calls[1][1]?.headers).toMatchObject({ 'X-API-Token': originalToken });
    setApiToken(replacementToken);
    const rejected = response(401);
    fallback.resolve(rejected);
    expect(await pending).toBe(rejected);
    expect(mockFetch).toHaveBeenCalledTimes(2);
    expectRetained();
  });

  it('does not replay an old POST after a CSRF403', async () => {
    const held = deferred();
    mockFetch.mockReturnValueOnce(held.promise);
    const pending = apiFetch('/api/alerts/acknowledge', { method: 'POST', body: '{}' });
    clearApiToken();
    const rejected = response(403);
    held.resolve(rejected);
    expect(await pending).toBe(rejected);
    expect(mockFetch).toHaveBeenCalledOnce();
    expectRetained(null);
  });

  it.each([
    {
      name: 'invalid org recovery',
      status: 400,
      initialOrg: 'north',
      body: '{"error":"invalid_org"}',
      options: {},
    },
    {
      name: 'bootstrap default recovery',
      status: 403,
      initialOrg: 'default',
      body: '{"error":"default_org_access_denied"}',
      options: { recoverDefaultOrg: true },
    },
    {
      name: 'bootstrap body falls through to CSRF retry',
      status: 403,
      initialOrg: 'default',
      body: '{"error":"unrelated"}',
      options: { recoverDefaultOrg: true },
    },
  ])(
    'rechecks after reading the delayed $name body',
    async ({ status, initialOrg, body, options }) => {
      setOrgID(initialOrg);
      const text = deferred();
      const rejected = response(status);
      let reading = false;
      // Delay the response-body await separately from delivery of its headers.
      const clone = vi.spyOn(rejected, 'clone').mockImplementation(
        () =>
          ({
            text: () => {
              reading = true;
              return text.promise.then(() => body);
            },
          }) as Response,
      );
      mockFetch.mockResolvedValueOnce(rejected);
      const pending = apiFetch('/api/state/summary', options);
      await vi.waitFor(() => expect(reading).toBe(true));
      setApiToken(replacementToken);
      setOrgID('south');
      text.resolve(new Response());
      expect(await pending).toBe(rejected);
      expect(clone).toHaveBeenCalledOnce();
      expect(mockFetch).toHaveBeenCalledOnce();
      expectRetained(replacementToken, 'south');
      clone.mockRestore();
    },
  );

  it.each(['replace', 'clear'] as const)(
    'does not retry old authority after a429 wait and %s',
    async (change) => {
      vi.useFakeTimers();
      const rejected = response(429);
      mockFetch.mockResolvedValueOnce(rejected);
      const pending = apiFetch('/api/alerts/active');
      await vi.advanceTimersByTimeAsync(1);
      if (change === 'replace') setApiToken(replacementToken);
      else clearApiToken();
      await vi.advanceTimersByTimeAsync(1000);
      expect(await pending).toBe(rejected);
      expect(mockFetch).toHaveBeenCalledOnce();
      expectRetained(change === 'replace' ? replacementToken : null);
    },
  );

  it.each([{}, { expectedOrgID: 'north', retry: false }])(
    'refuses an unsent write after CSRF preparation with %j',
    async (options) => {
      vi.stubEnv('MODE', 'development');
      vi.useFakeTimers();
      document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
      const health = deferred();
      mockFetch.mockReturnValueOnce(health.promise);
      const pending = apiFetch('/api/alerts/acknowledge', {
        method: 'POST',
        body: '{}',
        ...options,
      }).catch((error: unknown) => error);
      expect(mockFetch.mock.calls[0][0]).toBe('/api/health');
      setApiToken(replacementToken);
      document.cookie = 'pulse_csrf=synthetic-new-csrf; Path=/';
      health.resolve(new Response('{}'));
      await vi.advanceTimersByTimeAsync(10);
      expect(await pending).toMatchObject({
        message: 'Authentication changed. Reload before making changes.',
      });
      expect(mockFetch).toHaveBeenCalledOnce();
      expectRetained();
    },
  );

  it('leaves intentionally unauthenticated dispatch and result ownership unchanged', async () => {
    vi.stubEnv('MODE', 'development');
    vi.useFakeTimers();
    document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
    const health = deferred();
    const rejected = response(401);
    mockFetch.mockReturnValueOnce(health.promise).mockResolvedValueOnce(rejected);
    const pending = apiFetch('/api/login', { method: 'POST', skipAuth: true, body: '{}' });
    setApiToken(replacementToken);
    health.resolve(new Response('{}'));
    await vi.advanceTimersByTimeAsync(10);
    expect(await pending).toBe(rejected);
    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(mockFetch.mock.calls[1][1]?.headers).not.toHaveProperty('X-API-Token');
    expectRetained();
  });

  it('does not disguise an already-applied mutation success after replacement', async () => {
    const held = deferred();
    mockFetch.mockReturnValueOnce(held.promise);
    const pending = apiFetch('/api/alerts/acknowledge', {
      method: 'POST',
      body: '{}',
      retry: false,
    });
    setApiToken(replacementToken);
    const saved = new Response('{"success":true}');
    held.resolve(saved);
    expect(await pending).toBe(saved);
    expect(mockFetch).toHaveBeenCalledOnce();
    expectRetained();
  });

  it('allows only the first concurrent current401 to clear auth and redirect', async () => {
    const first = deferred();
    const second = deferred();
    mockFetch.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
    const a = apiFetch('/api/alerts/active');
    const b = apiFetch('/api/alerts/history');
    first.resolve(response(401));
    second.resolve(response(401));
    await Promise.all([a, b]);
    expect(getApiToken()).toBeNull();
    expect(getOrgID()).toBeNull();
    expect(localStorage.getItem('just_logged_out')).toBe('true');
    expect(redirect).toHaveBeenCalledExactlyOnceWith('/');
  });

  it('does not retire ordinary requests on an org-only change', async () => {
    const held = deferred();
    mockFetch.mockReturnValueOnce(held.promise);
    const pending = apiFetch('/api/alerts/active');
    setOrgID('south');
    held.resolve(response(401));
    await pending;
    expect(redirect).toHaveBeenCalledExactlyOnceWith('/');
    expect(getApiToken()).toBeNull();
    expect(getOrgID()).toBeNull();
  });

  it('does not mistake lazy storage rehydration for explicit credential replacement', async () => {
    clearApiToken();
    const held = deferred();
    mockFetch.mockReturnValueOnce(held.promise);
    const pending = apiFetch('/api/alerts/active');
    sessionStorage.setItem('pulse_auth', JSON.stringify({ type: 'token', value: originalToken }));
    expect(getApiToken()).toBe(originalToken);
    held.resolve(response(401));
    await pending;
    expect(getApiToken()).toBeNull();
    expect(redirect).toHaveBeenCalledExactlyOnceWith('/');
  });
});
