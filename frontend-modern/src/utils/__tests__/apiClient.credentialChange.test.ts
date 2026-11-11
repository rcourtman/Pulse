import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  apiFetch,
  beginCredentialChange,
  clearApiToken,
  clearAuth,
  getApiToken,
  getOrgID,
  setApiToken,
  setOrgID,
} from '@/utils/apiClient';

const fetchMock = vi.fn<typeof fetch>();
const redirect = vi.fn();
const locationDescriptor = Object.getOwnPropertyDescriptor(window, 'location');
const rejected = () => new Response('{"error":"Synthetic authentication refusal"}', { status: 401 });

beforeEach(() => {
  clearAuth();
  sessionStorage.clear();
  localStorage.clear();
  setApiToken('synthetic-original-token');
  setOrgID('north');
  fetchMock.mockReset().mockImplementation(async () => rejected());
  redirect.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: {
      get href() { return 'http://localhost/settings/security'; },
      set href(value: string) { redirect(value); },
    },
  });
});
afterEach(() => {
  clearAuth();
  vi.unstubAllGlobals();
  if (locationDescriptor) Object.defineProperty(window, 'location', locationDescriptor);
});

const expectRetained = () => {
  expect(getApiToken()).toBe('synthetic-original-token');
  expect(getOrgID()).toBe('north');
  expect(localStorage.getItem('just_logged_out')).toBeNull();
  expect(redirect).not.toHaveBeenCalled();
};

describe('a credential change owns only its temporary authentication redirect', () => {
  it('does not defer an ordinary genuine current authentication failure', async () => {
    expect((await apiFetch('/api/alerts/active')).status).toBe(401);
    expect(getApiToken()).toBeNull();
    expect(getOrgID()).toBeNull();
    expect(redirect).toHaveBeenCalledExactlyOnceWith('/');
  });

  it('returns each real401 immediately, reports rejection, and defers only destructive navigation', async () => {
    const notice = vi.fn();
    const change = beginCredentialChange(notice);
    const response = rejected();
    fetchMock.mockResolvedValueOnce(response);
    expect(await apiFetch('/api/alerts/active')).toBe(response);
    expect((await apiFetch('/api/alerts/active')).status).toBe(401);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(notice).toHaveBeenCalledTimes(2);
    expect(change.isCurrent()).toBe(true);
    expectRetained();
    expect(change.release()).toBe(true);
    expect(change.release()).toBe(false);
    expect(redirect).toHaveBeenCalledExactlyOnceWith('/');
    expect(getApiToken()).toBeNull();
    expect(getOrgID()).toBeNull();
  });

  it('lets confirmed rotation retire the old rejection, but still applies a new current401', async () => {
    const change = beginCredentialChange(vi.fn());
    await apiFetch('/api/alerts/active');
    clearApiToken();
    expect(change.isCurrent()).toBe(false);
    expect(change.release()).toBe(false);
    expect(getOrgID()).toBe('north');
    expect(redirect).not.toHaveBeenCalled();
    await apiFetch('/api/alerts/active');
    expect(redirect).toHaveBeenCalledExactlyOnceWith('/');
  });

  it.each([
    ['replacement token', () => setApiToken('synthetic-later-token')],
    ['same-value new admission', () => setApiToken('synthetic-original-token')],
    ['explicit sign-out', clearAuth],
    ['new organization', () => setOrgID('south')],
  ])('does not replay deferred side effects after %s', async (_name, replace) => {
    const change = beginCredentialChange(vi.fn());
    await apiFetch('/api/alerts/active');
    replace();
    const snapshot = [getApiToken(), getOrgID()];
    expect(change.isCurrent()).toBe(false);
    expect(change.release()).toBe(false);
    expect([getApiToken(), getOrgID()]).toEqual(snapshot);
    expect(redirect).not.toHaveBeenCalled();
  });

  it('cannot suppress a replacement reader401 even before the old form releases', async () => {
    const notice = vi.fn();
    const change = beginCredentialChange(notice);
    setApiToken('synthetic-later-token');
    await apiFetch('/api/alerts/active');
    expect(notice).not.toHaveBeenCalled();
    expect(redirect).toHaveBeenCalledExactlyOnceWith('/');
    expect(change.release()).toBe(false);
  });

  it('refuses a second live credential change rather than dropping its pending failure', async () => {
    const change = beginCredentialChange(vi.fn());
    await apiFetch('/api/alerts/active');
    expect(() => beginCredentialChange(vi.fn())).toThrow('credential change');
    expect(change.release()).toBe(true);
    expect(redirect).toHaveBeenCalledOnce();
  });

  it('releasing a successful/denied change without a background401 creates no expiry', () => {
    const change = beginCredentialChange(vi.fn());
    expect(change.release()).toBe(false);
    expectRetained();
  });

  it.each([
    ['/api/security/status', {}],
    ['/api/security/quick-setup', { method: 'POST', expectedOrgID: 'north', retry: false }],
    ['/api/alerts/active', { skipAuth: true }],
  ])('keeps endpoint-owned or unauthenticated401 unchanged for %s', async (url, options) => {
    document.cookie = 'pulse_csrf=synthetic-csrf; Path=/';
    const notice = vi.fn();
    const change = beginCredentialChange(notice);
    expect((await apiFetch(url, options)).status).toBe(401);
    expect(notice).not.toHaveBeenCalled();
    expect(change.release()).toBe(false);
    expectRetained();
  });
});
