import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { OrgsAPI, type OrganizationAccessRequestOptions } from '../orgs';
import {
  clearAuth,
  getApiToken,
  getOrgID,
  setApiToken,
  setOrgID,
} from '@/utils/apiClient';

const mockFetch = vi.fn();
const csrfToken = 'synthetic-csrf=issued-after-health';
const storedToken = 'synthetic-unrelated-api-token';
const member = { userId: 'member/one', role: 'viewer' as const };
const share = {
  targetOrgId: 'south',
  resourceType: 'vm',
  resourceId: 'opaque::guest',
  accessRole: 'viewer' as const,
};

// These are the real API wrappers and transport. Only the network is mocked:
// unlike the ordinary test-mode path, each change must wait for /api/health,
// the newly issued cookie and the production CSRF preparation delay.
const mutations: {
  name: string;
  path: string;
  method: string;
  body?: object;
  run: (options: OrganizationAccessRequestOptions) => Promise<unknown>;
}[] = [
  {
    name: 'rename organization',
    path: '/api/orgs/north',
    method: 'PUT',
    body: { displayName: 'North renamed' },
    run: (options) => OrgsAPI.update('north', { displayName: 'North renamed' }, options),
  },
  {
    name: 'invite member',
    path: '/api/orgs/north/members',
    method: 'POST',
    body: member,
    run: (options) => OrgsAPI.inviteMember('north', member, options),
  },
  {
    name: 'change member role',
    path: '/api/orgs/north/members',
    method: 'POST',
    body: member,
    run: (options) => OrgsAPI.updateMemberRole('north', member, options),
  },
  {
    name: 'remove member',
    path: '/api/orgs/north/members/member%2Fone',
    method: 'DELETE',
    run: (options) => OrgsAPI.removeMember('north', member.userId, options),
  },
  {
    name: 'revoke invitation',
    path: '/api/orgs/north/invitations/member%2Fone',
    method: 'DELETE',
    run: (options) => OrgsAPI.revokeInvitation('north', member.userId, options),
  },
  {
    name: 'accept personal invitation',
    path: '/api/org-invitations/inviting%2Forg/accept',
    method: 'POST',
    run: (options) => OrgsAPI.acceptMyInvitation('inviting/org', options),
  },
  {
    name: 'decline personal invitation',
    path: '/api/org-invitations/inviting%2Forg',
    method: 'DELETE',
    run: (options) => OrgsAPI.declineMyInvitation('inviting/org', options),
  },
  {
    name: 'create share',
    path: '/api/orgs/north/shares',
    method: 'POST',
    body: share,
    run: (options) => OrgsAPI.createShare('north', share, options),
  },
  {
    name: 'delete share',
    path: '/api/orgs/north/shares/share%2Fone',
    method: 'DELETE',
    run: (options) => OrgsAPI.deleteShare('north', 'share/one', options),
  },
  {
    name: 'accept incoming share',
    path: '/api/orgs/north/shares/incoming/share%2Fone/accept',
    method: 'POST',
    run: (options) => OrgsAPI.acceptIncomingShare('north', 'share/one', options),
  },
  {
    name: 'decline incoming share',
    path: '/api/orgs/north/shares/incoming/share%2Fone',
    method: 'DELETE',
    run: (options) => OrgsAPI.declineIncomingShare('north', 'share/one', options),
  },
];

function prepareColdRequest() {
  let finishHealth!: (response: Response) => void;
  const health = new Promise<Response>((resolve) => {
    finishHealth = resolve;
  });
  mockFetch.mockReturnValueOnce(health).mockImplementation(async () => new Response('{}'));
  const controller = new AbortController();
  const options: OrganizationAccessRequestOptions = {
    expectedOrgID: 'north',
    retry: false,
    signal: controller.signal,
  };
  return {
    controller,
    options,
    issueCookie: () => {
      // A mocked fetch does not process Set-Cookie, so model the browser's
      // cookie jar update before it makes the health response available.
      document.cookie = `pulse_csrf=${encodeURIComponent(csrfToken)}; Path=/`;
      finishHealth(new Response('{}'));
    },
  };
}

function expectOnlyHealthRead() {
  expect(mockFetch.mock.calls).toEqual([
    ['/api/health', { method: 'GET', credentials: 'include' }],
  ]);
}

beforeEach(() => {
  vi.useFakeTimers();
  // Exercise production CSRF preparation rather than its test-mode early exit.
  // The fetch stub is installed first; no real network request is possible.
  mockFetch.mockReset();
  vi.stubGlobal('fetch', mockFetch);
  vi.stubEnv('MODE', 'development');
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

describe.each(mutations)('$name after a cold CSRF read', ({ path, method, body, run }) => {
  it('dispatches exactly once with the issued cookie, original selection and session authentication', async () => {
    const request = prepareColdRequest();
    const pending = run(request.options);
    expectOnlyHealthRead();
    request.issueCookie();
    await vi.advanceTimersByTimeAsync(9);
    expectOnlyHealthRead();
    await vi.advanceTimersByTimeAsync(1);
    await pending;

    expect(mockFetch).toHaveBeenCalledTimes(2);
    const [url, options] = mockFetch.mock.calls[1];
    expect(url).toBe(path);
    expect(options).toEqual({
      method,
      ...(body ? { body: JSON.stringify(body) } : {}),
      credentials: 'include',
      signal: request.controller.signal,
      headers: {
        'Content-Type': 'application/json',
        'X-Requested-With': 'XMLHttpRequest',
        Accept: 'application/json',
        'X-Pulse-Org-ID': 'north',
        'X-CSRF-Token': csrfToken,
      },
    });
    expect(getOrgID()).toBe('north');
    expect(getApiToken()).toBe(storedToken);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('does not send a change when the selection changes during the health read', async () => {
    const request = prepareColdRequest();
    const pending = run(request.options);
    const rejected = expect(pending).rejects.toThrow('Organisation changed');
    expectOnlyHealthRead();
    setOrgID('south');
    request.issueCookie();
    await vi.advanceTimersByTimeAsync(10);
    await rejected;

    expectOnlyHealthRead();
    expect(getOrgID()).toBe('south');
    expect(getApiToken()).toBe(storedToken);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('does not send a change when its reader is cancelled during the health read', async () => {
    const request = prepareColdRequest();
    const pending = run(request.options);
    const rejected = expect(pending).rejects.toMatchObject({ name: 'AbortError' });
    expectOnlyHealthRead();
    request.controller.abort();
    request.issueCookie();
    await vi.advanceTimersByTimeAsync(10);
    await rejected;

    expectOnlyHealthRead();
    expect(getOrgID()).toBe('north');
    expect(getApiToken()).toBe(storedToken);
    expect(vi.getTimerCount()).toBe(0);
  });
});

it('rechecks the reader after cookie issuance, not just when the health response arrives', async () => {
  const request = prepareColdRequest();
  const pending = OrgsAPI.removeMember('north', member.userId, request.options);
  const rejected = expect(pending).rejects.toMatchObject({ name: 'AbortError' });
  request.issueCookie();
  await vi.advanceTimersByTimeAsync(9);
  expectOnlyHealthRead();
  request.controller.abort();
  await vi.advanceTimersByTimeAsync(1);
  await rejected;
  expectOnlyHealthRead();
  expect(vi.getTimerCount()).toBe(0);
});
