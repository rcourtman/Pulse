import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  apiFetch,
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
const refused = (status = 401) =>
  new Response(JSON.stringify({ error: 'unauthorized' }), {
    status,
    headers: { 'X-CSRF-Token': 'synthetic-new-csrf', 'Retry-After': '1' },
  });
const expectRetained = (org = 'south', token = replacementToken) => {
  expect(getOrgID()).toBe(org);
  expect(sessionStorage.getItem('pulse_org_id')).toBe(org);
  expect(getApiToken()).toBe(token);
  expect(JSON.parse(sessionStorage.getItem('pulse_auth')!).value).toBe(token);
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
        return 'http://localhost/settings/organizations/overview';
      },
      set href(value: string) {
        redirect(value);
      },
      pathname: '/settings/organizations/overview',
      protocol: 'http:',
      search: '',
    },
  });
  clearAuth();
  sessionStorage.clear();
  localStorage.clear();
  setOrgID('north');
  setApiToken(originalToken);
  document.cookie = 'pulse_csrf=synthetic-csrf; Path=/';
});
afterEach(() => {
  clearAuth();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  vi.unstubAllGlobals();
  if (locationDescriptor) Object.defineProperty(window, 'location', locationDescriptor);
});

describe('retired strict response ownership', () => {
  it.each([{ expectedOrgID: 'north' }, { retry: false }])(
    'does not sign out a replacement reader after response delivery with %j',
    async (options) => {
      const response = refused();
      const held = deferred();
      const controller = new AbortController();
      mockFetch.mockReturnValueOnce(held.promise);
      const pending = apiFetch('/api/orgs/north', { ...options, signal: controller.signal });
      expect(mockFetch).toHaveBeenCalledOnce();
      // Resolve before aborting to model a response already queued for delivery.
      // A native abort cannot retract its fulfilled promise's continuation.
      held.resolve(response);
      controller.abort();
      setOrgID('south');
      setApiToken(replacementToken);
      expect(await pending).toBe(response);
      expect(mockFetch).toHaveBeenCalledOnce();
      expectRetained();
    },
  );

  it('preserves a replacement organization even if its caller did not abort', async () => {
    const held = deferred();
    mockFetch.mockReturnValueOnce(held.promise);
    const pending = apiFetch('/api/orgs/north', { expectedOrgID: 'north', retry: false });
    setOrgID('south');
    setApiToken(replacementToken);
    const response = refused();
    held.resolve(response);
    expect(await pending).toBe(response);
    expectRetained();
  });

  it.each(['same organization', 'return to original organization'])(
    'respects cancellation with a replacement reader in the %s',
    async (kind) => {
      const held = deferred();
      const controller = new AbortController();
      mockFetch.mockReturnValueOnce(held.promise);
      const pending = apiFetch('/api/admin/roles', {
        expectedOrgID: 'north',
        retry: false,
        signal: controller.signal,
      });
      if (kind === 'return to original organization') setOrgID('south');
      controller.abort();
      setOrgID('north');
      setApiToken(replacementToken);
      const response = refused();
      held.resolve(response);
      expect(await pending).toBe(response);
      expectRetained('north');
    },
  );

  it('does not fall back with a replacement token after retiring a session-preferred read', async () => {
    const held = deferred();
    mockFetch.mockReturnValueOnce(held.promise);
    const pending = apiFetch('/api/orgs/north', {
      expectedOrgID: 'north',
      preferSessionAuth: true,
    });
    expect(new Headers(mockFetch.mock.calls[0][1]?.headers).has('X-API-Token')).toBe(false);
    setOrgID('south');
    setApiToken(replacementToken);
    const response = refused();
    held.resolve(response);
    expect(await pending).toBe(response);
    expect(mockFetch).toHaveBeenCalledOnce();
    expectRetained();
  });

  it('rechecks ownership when the existing token fallback itself completes', async () => {
    const held = deferred();
    const controller = new AbortController();
    mockFetch.mockResolvedValueOnce(refused()).mockReturnValueOnce(held.promise);
    const pending = apiFetch('/api/orgs/north', {
      expectedOrgID: 'north',
      preferSessionAuth: true,
      signal: controller.signal,
    });
    await vi.waitFor(() => expect(mockFetch).toHaveBeenCalledTimes(2));
    expect(new Headers(mockFetch.mock.calls[1][1]?.headers).get('X-API-Token')).toBe(originalToken);
    const response = refused();
    held.resolve(response);
    controller.abort();
    setOrgID('south');
    setApiToken(replacementToken);
    expect(await pending).toBe(response);
    expect(mockFetch).toHaveBeenCalledTimes(2);
    expectRetained();
  });

  it.each([400, 403, 429])(
    'returns a retired HTTP %i without recovery, replay or sign-out',
    async (status) => {
      const held = deferred();
      mockFetch.mockReturnValueOnce(held.promise);
      const pending = apiFetch('/api/orgs/north', { expectedOrgID: 'north' });
      setOrgID('south');
      setApiToken(replacementToken);
      const response = refused(status);
      held.resolve(response);
      expect(await pending).toBe(response);
      expect(mockFetch).toHaveBeenCalledOnce();
      expectRetained();
    },
  );

  it('still signs out a genuinely current strict request on credential expiry', async () => {
    const response = refused();
    mockFetch.mockResolvedValueOnce(response);
    expect(await apiFetch('/api/orgs/north', { expectedOrgID: 'north', retry: false })).toBe(
      response,
    );
    expect(mockFetch).toHaveBeenCalledOnce();
    expect(getApiToken()).toBeNull();
    expect(getOrgID()).toBeNull();
    expect(localStorage.getItem('just_logged_out')).toBe('true');
    expect(redirect).toHaveBeenCalledExactlyOnceWith('/');
  });

  it('keeps the current organization on a strict membership refusal', async () => {
    const response = refused(403);
    mockFetch.mockResolvedValueOnce(response);
    expect(await apiFetch('/api/orgs/north', { expectedOrgID: 'north', retry: false })).toBe(
      response,
    );
    expectRetained('north', originalToken);
  });

  it('returns an owned credential success unchanged after its reader retires', async () => {
    const held = deferred();
    const controller = new AbortController();
    mockFetch.mockReturnValueOnce(held.promise);
    const pending = apiFetch('/api/security/change-password', {
      method: 'POST',
      expectedOrgID: 'north',
      retry: false,
      signal: controller.signal,
    });
    controller.abort();
    setOrgID('south');
    setApiToken(replacementToken);
    const response = new Response('{}');
    held.resolve(response);
    expect(await pending).toBe(response);
    expectRetained();
  });

  it('preserves ordinary unbound expiry handling even after a native abort', async () => {
    const held = deferred();
    const controller = new AbortController();
    mockFetch.mockReturnValueOnce(held.promise);
    const pending = apiFetch('/api/orgs/north', { signal: controller.signal });
    controller.abort();
    const response = refused();
    held.resolve(response);
    expect(await pending).toBe(response);
    expect(getApiToken()).toBeNull();
    expect(getOrgID()).toBeNull();
    expect(localStorage.getItem('just_logged_out')).toBe('true');
    expect(redirect).toHaveBeenCalledExactlyOnceWith('/');
  });

  it('preserves ordinary native completion behavior without a strict opt-in', async () => {
    const held = deferred();
    const controller = new AbortController();
    mockFetch.mockReturnValueOnce(held.promise);
    const pending = apiFetch('/api/state', { signal: controller.signal });
    controller.abort();
    const response = new Response('{}');
    held.resolve(response);
    expect(await pending).toBe(response);
    expectRetained('north', originalToken);
  });
});
