import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { OrgsAPI, type OrganizationAccessRequestOptions } from '../orgs';
import { apiClient, clearApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';

const mockFetch = vi.fn();
const accessRequests: {
  name: string;
  run: (options: OrganizationAccessRequestOptions) => Promise<unknown>;
  path: string;
  method: string;
}[] = [
  {
    name: 'organization',
    run: (options) => OrgsAPI.get('org/a', options),
    path: '/api/orgs/org%2Fa',
    method: 'GET',
  },
  {
    name: 'members',
    run: (options) => OrgsAPI.listMembers('org/a', options),
    path: '/api/orgs/org%2Fa/members',
    method: 'GET',
  },
  {
    name: 'pending invitations',
    run: (options) => OrgsAPI.listPendingInvitations('org/a', options),
    path: '/api/orgs/org%2Fa/invitations',
    method: 'GET',
  },
  {
    name: 'personal invitations',
    run: (options) => OrgsAPI.listMyInvitations(options),
    path: '/api/org-invitations',
    method: 'GET',
  },
  {
    name: 'invite member',
    run: (options) =>
      OrgsAPI.inviteMember('org/a', { userId: 'new-user', role: 'viewer' }, options),
    path: '/api/orgs/org%2Fa/members',
    method: 'POST',
  },
  {
    name: 'update role',
    run: (options) =>
      OrgsAPI.updateMemberRole('org/a', { userId: 'existing-user', role: 'editor' }, options),
    path: '/api/orgs/org%2Fa/members',
    method: 'POST',
  },
  {
    name: 'remove member',
    run: (options) => OrgsAPI.removeMember('org/a', 'user/b', options),
    path: '/api/orgs/org%2Fa/members/user%2Fb',
    method: 'DELETE',
  },
  {
    name: 'revoke invitation',
    run: (options) => OrgsAPI.revokeInvitation('org/a', 'user/b', options),
    path: '/api/orgs/org%2Fa/invitations/user%2Fb',
    method: 'DELETE',
  },
  {
    name: 'accept personal invitation',
    run: (options) => OrgsAPI.acceptMyInvitation('inviting/org', options),
    path: '/api/org-invitations/inviting%2Forg/accept',
    method: 'POST',
  },
  {
    name: 'decline personal invitation',
    run: (options) => OrgsAPI.declineMyInvitation('inviting/org', options),
    path: '/api/org-invitations/inviting%2Forg',
    method: 'DELETE',
  },
];
const options = (): OrganizationAccessRequestOptions => ({
  expectedOrgID: 'org-a',
  retry: false,
  signal: new AbortController().signal,
});

beforeEach(() => {
  vi.useRealTimers();
  mockFetch.mockReset();
  vi.stubGlobal('fetch', mockFetch);
  window.sessionStorage.clear();
  window.localStorage.clear();
  clearApiToken();
  setOrgID('org-a');
  document.cookie = 'pulse_csrf=synthetic-csrf; Path=/';
});

afterEach(() => {
  clearApiToken();
  setOrgID(null);
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('Organization Access strict transport', () => {
  it.each(accessRequests)(
    'binds $name to the selected context, preserves session authentication and encodes target identities',
    async ({ run, path, method }) => {
      setApiToken('synthetic-token');
      mockFetch.mockResolvedValue(new Response('{}'));
      const binding = options();
      await run(binding);
      expect(mockFetch).toHaveBeenCalledTimes(1);
      const [url, request] = mockFetch.mock.calls[0];
      expect(url).toBe(path);
      expect(request.method ?? 'GET').toBe(method);
      expect(request.headers['X-Pulse-Org-ID']).toBe('org-a');
      expect(request.headers['X-API-Token']).toBeUndefined();
      expect(request.credentials).toBe('include');
      expect(request.signal).toBe(binding.signal);
      if (method !== 'GET') expect(request.headers['X-CSRF-Token']).toBeTruthy();
    },
  );

  it.each([400, 403, 429])(
    'does not replay an access mutation or change selection after HTTP %i',
    async (status) => {
      mockFetch.mockResolvedValue(
        new Response(JSON.stringify({ error: 'invalid_org' }), {
          status,
          headers: { 'X-CSRF-Token': 'new-csrf', 'Retry-After': '1' },
        }),
      );
      await expect(
        OrgsAPI.inviteMember('org-a', { userId: 'new-user', role: 'viewer' }, options()),
      ).rejects.toThrow();
      expect(mockFetch).toHaveBeenCalledTimes(1);
      expect(getOrgID()).toBe('org-a');
    },
  );

  it('does not fall back to the stored token after a refused session read', async () => {
    setApiToken('synthetic-token');
    mockFetch.mockResolvedValue(new Response('{}', { status: 401 }));
    await expect(OrgsAPI.get('org-a', options())).rejects.toThrow();
    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockFetch.mock.calls[0][1].headers['X-API-Token']).toBeUndefined();
  });

  it('rejects a stale selection before dispatch, including during asynchronous CSRF preparation', async () => {
    let finish!: (value: string) => void;
    vi.spyOn(apiClient as any, 'loadCSRFToken').mockReturnValueOnce(null);
    vi.spyOn(apiClient as any, 'ensureCSRFToken').mockReturnValueOnce(
      new Promise((resolve) => {
        finish = resolve;
      }),
    );
    const pending = OrgsAPI.removeMember('org-a', 'user-2', options());
    setOrgID('org-b');
    finish('synthetic-csrf');
    await expect(pending).rejects.toThrow('Organisation changed');
    expect(mockFetch).not.toHaveBeenCalled();
    await expect(OrgsAPI.get('org-a', options())).rejects.toThrow('Organisation changed');
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it('does not dispatch after a controller was invalidated while CSRF was pending', async () => {
    let finish!: (value: string) => void;
    vi.spyOn(apiClient as any, 'loadCSRFToken').mockReturnValueOnce(null);
    vi.spyOn(apiClient as any, 'ensureCSRFToken').mockReturnValueOnce(
      new Promise((resolve) => {
        finish = resolve;
      }),
    );
    const controller = new AbortController();
    const pending = OrgsAPI.declineMyInvitation('inviting-org', {
      ...options(),
      signal: controller.signal,
    });
    controller.abort();
    finish('synthetic-csrf');
    await expect(pending).rejects.toMatchObject({ name: 'AbortError' });
    expect(mockFetch).not.toHaveBeenCalled();
  });
});
