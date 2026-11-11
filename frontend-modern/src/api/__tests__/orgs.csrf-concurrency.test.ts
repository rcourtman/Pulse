import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { OrgsAPI, type OrganizationAccessRequestOptions } from '../orgs';
import { clearAuth, getApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';

// Independent panels share the real client and cookie jar, not an intent or
// AbortController. Keep both cold preparations pending to expose cross-talk.
const storedToken = 'synthetic-unrelated-api-token';
const csrfToken = 'synthetic-concurrent-csrf=issued';
const renameBody = { displayName: 'North renamed' };
const inviteBody = { userId: 'member/two', role: 'viewer' as const };
const shareBody = {
  targetOrgId: 'peer',
  resourceType: 'vm',
  resourceId: 'south::guest',
  accessRole: 'viewer' as const,
};
const json = (body: unknown) => new Response(JSON.stringify(body));
const deferred = () => {
  let resolve!: (response: Response) => void;
  const promise = new Promise<Response>((done) => {
    resolve = done;
  });
  return { promise, resolve };
};
const mockFetch = vi.fn<typeof fetch>();
let health: ReturnType<typeof deferred>[];
let unexpected: string[];
const options = (org = 'north'): OrganizationAccessRequestOptions => ({
  expectedOrgID: org,
  retry: false,
  signal: new AbortController().signal,
});
const writes = () => mockFetch.mock.calls.filter(([url]) => url !== '/api/health');
const issueCookie = () => {
  // Only the synthetic browser cookie update is modelled, never client internals.
  document.cookie = `pulse_csrf=${encodeURIComponent(csrfToken)}; Path=/`;
};
const finishHealth = async (index: number) => {
  issueCookie();
  health[index].resolve(json({}));
  await vi.runAllTimersAsync();
};
const expectPreparations = () => {
  expect(health).toHaveLength(2);
  expect(mockFetch.mock.calls.slice(0, 2)).toEqual([
    ['/api/health', { method: 'GET', credentials: 'include' }],
    ['/api/health', { method: 'GET', credentials: 'include' }],
  ]);
  expect(writes()).toHaveLength(0);
};
const expectWrite = (
  index: number,
  path: string,
  method: string,
  body: object,
  binding: OrganizationAccessRequestOptions,
) => {
  expect(writes()[index]).toEqual([
    path,
    {
      method,
      body: JSON.stringify(body),
      signal: binding.signal,
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        'X-Requested-With': 'XMLHttpRequest',
        Accept: 'application/json',
        'X-Pulse-Org-ID': binding.expectedOrgID,
        'X-CSRF-Token': csrfToken,
      },
    },
  ]);
};

beforeEach(() => {
  vi.useFakeTimers();
  mockFetch.mockReset();
  health = [];
  unexpected = [];
  mockFetch.mockImplementation((url, init) => {
    if (url === '/api/health' && init?.method === 'GET') {
      const pending = deferred();
      health.push(pending);
      return pending.promise;
    }
    if (url === '/api/orgs/north' && init?.method === 'PUT')
      return Promise.resolve(json({ id: 'north', ...renameBody }));
    if (url === '/api/orgs/north/members' && init?.method === 'POST')
      return Promise.resolve(json({ kind: 'member', member: inviteBody }));
    if (url === '/api/orgs/south/shares' && init?.method === 'POST')
      return Promise.resolve(json({ id: 'share/south', ...shareBody }));
    unexpected.push(`${init?.method ?? 'GET'} ${url}`);
    return Promise.reject(new Error('Unexpected synthetic request'));
  });
  vi.stubGlobal('fetch', mockFetch);
  vi.stubEnv('MODE', 'production');
  window.sessionStorage.clear();
  window.localStorage.clear();
  document.cookie = 'pulse_csrf=; Path=/; Max-Age=0';
  clearAuth();
  setOrgID('north');
  setApiToken(storedToken);
});

afterEach(() => {
  clearAuth();
  document.cookie = 'pulse_csrf=; Path=/; Max-Age=0';
  vi.useRealTimers();
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
});

// Test both completion orders, not scheduler-selected races. The preparation
// delay duration is an implementation detail; drain it without pinning its size.
describe.each([0, 1])('concurrent cold CSRF with preparation %i completing first', (first) => {
  const second = 1 - first;
  const expectRetained = (org = 'north') => {
    expect(unexpected).toEqual([]);
    expect(getOrgID()).toBe(org);
    expect(sessionStorage.getItem('pulse_org_id')).toBe(org);
    expect(getApiToken()).toBe(storedToken);
    expect(localStorage.getItem('just_logged_out')).toBeNull();
    expect(vi.getTimerCount()).toBe(0);
  };

  it('keeps both live requests attached to their own payload, method and signal', async () => {
    const rename = options();
    const invite = options();
    const pending = [
      OrgsAPI.update('north', renameBody, rename),
      OrgsAPI.inviteMember('north', inviteBody, invite),
    ];
    const results = [
      { id: 'north', ...renameBody },
      { kind: 'member', member: inviteBody },
    ];
    expectPreparations();

    await finishHealth(first);
    expect(await pending[first]).toEqual(results[first]);
    expect(writes()).toHaveLength(1);
    await finishHealth(second);
    expect(await pending[second]).toEqual(results[second]);

    expect(writes()).toHaveLength(2);
    expectWrite(first, '/api/orgs/north', 'PUT', renameBody, rename);
    expectWrite(second, '/api/orgs/north/members', 'POST', inviteBody, invite);
    expect(rename.signal).not.toBe(invite.signal);
    expect(rename.signal.aborted).toBe(false);
    expect(invite.signal.aborted).toBe(false);
    expectRetained();
  });

  it.each(['health pending', 'cookie issued'] as const)(
    'retires only the cancelled intent when cancellation occurs with %s',
    async (stage) => {
      const controller = new AbortController();
      const retired = { ...options(), signal: controller.signal };
      const live = options();
      const rejected = expect(OrgsAPI.update('north', renameBody, retired)).rejects.toMatchObject({
        name: 'AbortError',
      });
      const pending = OrgsAPI.inviteMember('north', inviteBody, live);
      expectPreparations();
      if (stage === 'cookie issued') {
        issueCookie();
        health[first].resolve(json({}));
        // Settle health without advancing the post-health preparation delay.
        await vi.advanceTimersByTimeAsync(0);
        expect(writes()).toHaveLength(0);
      }
      controller.abort();
      await finishHealth(first);
      expect(writes()).toHaveLength(first);
      await finishHealth(second);
      await rejected;
      expect(await pending).toEqual({ kind: 'member', member: inviteBody });

      expect(writes()).toHaveLength(1);
      expectWrite(0, '/api/orgs/north/members', 'POST', inviteBody, live);
      expect(retired.signal.aborted).toBe(true);
      expect(live.signal.aborted).toBe(false);
      expectRetained();
    },
  );

  it('rejects the old selection without cancelling or retargeting the new selection', async () => {
    const old = options();
    const rejected = expect(OrgsAPI.update('north', renameBody, old)).rejects.toThrow(
      'Organisation changed',
    );
    setOrgID('south');
    const live = options('south');
    const pending = OrgsAPI.createShare('south', shareBody, live);
    expectPreparations();

    await finishHealth(first);
    expect(writes()).toHaveLength(first);
    await finishHealth(second);
    await rejected;
    expect(await pending).toEqual({ id: 'share/south', ...shareBody });

    expect(writes()).toHaveLength(1);
    expectWrite(0, '/api/orgs/south/shares', 'POST', shareBody, live);
    // Selection validation, not native fetch abort, must retire the old intent.
    expect(old.signal.aborted).toBe(false);
    expect(live.signal.aborted).toBe(false);
    expectRetained('south');
  });
});
