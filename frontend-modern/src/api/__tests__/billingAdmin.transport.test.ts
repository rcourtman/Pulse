import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { BillingState } from '../billingAdmin';

const fetchMock = vi.fn();
let api: typeof import('../billingAdmin');
let client: typeof import('@/utils/apiClient');
const state: BillingState = {
  subscription_state: 'suspended',
  capabilities: ['multi_tenant'],
  limits: { max_nodes: 30 },
};

beforeEach(async () => {
  vi.resetModules();
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  sessionStorage.clear();
  client = await import('@/utils/apiClient');
  api = await import('../billingAdmin');
  client.setApiToken('synthetic-stored-token');
  client.setOrgID('selected-tenant');
});
afterEach(() => {
  client.clearAuth();
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

describe('Billing Admin real transport', () => {
  it('keeps global reads session-preferred and cancellable, without using the selected tenant', async () => {
    const signal = new AbortController().signal;
    fetchMock.mockResolvedValueOnce(new Response('[]', { status: 200 }));
    await api.BillingAdminAPI.listOrganizations({ signal });
    expect(fetchMock.mock.calls[0][1]).toMatchObject({
      signal,
      credentials: 'include',
      headers: { 'X-Pulse-Org-ID': 'default' },
    });
    expect(fetchMock.mock.calls[0][1].headers['X-API-Token']).toBeUndefined();
    expect(client.getOrgID()).toBe('selected-tenant');
  });

  it.each([403, 429, 503])(
    'does not automatically repeat a subscription PUT after HTTP %i',
    async (status) => {
      fetchMock.mockResolvedValueOnce(
        new Response('{}', {
          status,
          headers: { 'X-CSRF-Token': 'synthetic-new-csrf', 'Retry-After': '0' },
        }),
      );
      await expect(api.BillingAdminAPI.putBillingState('org/one', state)).rejects.toMatchObject({
        status,
      });
      expect(fetchMock).toHaveBeenCalledTimes(1);
      expect(fetchMock.mock.calls[0][0]).toBe('/api/admin/orgs/org%2Fone/billing-state');
      expect(fetchMock.mock.calls[0][1]).toMatchObject({
        method: 'PUT',
        body: JSON.stringify(state),
        headers: { 'X-Pulse-Org-ID': 'default' },
      });
      expect(fetchMock.mock.calls[0][1].headers['X-API-Token']).toBeUndefined();
    },
  );

  it('does not dispatch a withdrawn subscription change after CSRF preparation completes', async () => {
    vi.stubEnv('MODE', 'production');
    let resolve!: (value: Response) => void;
    fetchMock.mockImplementationOnce(
      () =>
        new Promise<Response>((yes) => {
          resolve = yes;
        }),
    );
    const controller = new AbortController();
    const writing = api.BillingAdminAPI.putBillingState('org-a', state, {
      signal: controller.signal,
    });
    const result = expect(writing).rejects.toMatchObject({ name: 'AbortError' });
    expect(fetchMock.mock.calls[0][0]).toBe('/api/health');
    controller.abort();
    resolve(new Response('{}', { status: 200 }));
    await result;
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});
