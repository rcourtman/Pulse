import { expect, test } from '@playwright/test';

import {
  apiRequest,
  createOrg,
  deleteOrg,
  ensureAuthenticated,
  E2E_CREDENTIALS,
  isMultiTenantEnabled,
} from './helpers';

type APITokenRecord = {
  id: string;
  scopes?: string[];
  ownerUserId?: string;
};

type APITokenCreateResponse = {
  token?: string;
  record?: APITokenRecord;
};

const bearerHeaders = (token: string, extraHeaders: Record<string, string> = {}) => ({
  Authorization: `Bearer ${token}`,
  ...extraHeaders,
});

test.describe.serial('API token scope and assignment gate', () => {
  test('owner-bound token enforces scope boundaries and revokes immediately', async ({ page }) => {
    await ensureAuthenticated(page);

    const createRes = await apiRequest(page, '/api/security/tokens', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      data: {
        name: `e2e-settings-read-${Date.now()}`,
        scopes: ['settings:read'],
      },
    });
    expect(createRes.ok(), await createRes.text()).toBeTruthy();

    const createPayload = (await createRes.json()) as APITokenCreateResponse;
    const token = createPayload.token?.trim() || '';
    const record = createPayload.record;

    expect(token.length).toBeGreaterThan(0);
    expect(record?.id).toBeTruthy();
    expect(record?.ownerUserId).toBe(E2E_CREDENTIALS.username);
    expect(record?.scopes).toEqual(['settings:read']);

    const readRes = await apiRequest(page, '/api/system/settings', {
      headers: bearerHeaders(token),
    });
    expect(readRes.ok(), await readRes.text()).toBeTruthy();

    const mutateRes = await apiRequest(page, '/api/security/tokens', {
      method: 'POST',
      headers: bearerHeaders(token, { 'Content-Type': 'application/json' }),
      data: {
        name: `e2e-escalate-${Date.now()}`,
        scopes: ['settings:write'],
      },
    });
    expect(mutateRes.status()).toBe(403);
    expect(await mutateRes.text()).toContain('settings:write');

    const execRes = await apiRequest(page, '/api/ai/execute/stream', {
      method: 'POST',
      headers: bearerHeaders(token, { 'Content-Type': 'application/json' }),
      data: {},
    });
    expect(execRes.status()).toBe(403);
    expect(await execRes.text()).toContain('ai:execute');

    const agentConfigRes = await apiRequest(page, '/api/agents/agent/host-1/config', {
      method: 'PATCH',
      headers: bearerHeaders(token, { 'Content-Type': 'application/json' }),
      data: {},
    });
    expect(agentConfigRes.status()).toBe(403);
    expect(await agentConfigRes.text()).toContain('agent:manage');

    const deleteRes = await apiRequest(page, `/api/security/tokens/${encodeURIComponent(record!.id)}`, {
      method: 'DELETE',
    });
    expect(deleteRes.status()).toBe(204);

    const staleReadRes = await apiRequest(page, '/api/system/settings', {
      headers: bearerHeaders(token),
    });
    expect(staleReadRes.status()).toBe(401);
  });

  test('org-bound token stays inside the issuing org', async ({ page }) => {
    await ensureAuthenticated(page);

    const mtEnabled = await isMultiTenantEnabled(page);
    test.skip(!mtEnabled, 'Multi-tenant feature not enabled in this environment');

    const orgA = await createOrg(page, `E2E Token Org A ${Date.now()}`);
    const orgB = await createOrg(page, `E2E Token Org B ${Date.now()}`);
    let tokenID = '';

    try {
      const createRes = await apiRequest(page, '/api/security/tokens', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Pulse-Org-ID': orgA.id,
          'X-Org-ID': orgA.id,
        },
        data: {
          name: `e2e-org-bound-${Date.now()}`,
          scopes: ['settings:read'],
        },
      });
      expect(createRes.ok(), await createRes.text()).toBeTruthy();

      const createPayload = (await createRes.json()) as APITokenCreateResponse;
      const token = createPayload.token?.trim() || '';
      tokenID = createPayload.record?.id || '';

      expect(token.length).toBeGreaterThan(0);
      expect(tokenID).toBeTruthy();
      expect(createPayload.record?.ownerUserId).toBe(E2E_CREDENTIALS.username);

      const ownMembersRes = await apiRequest(page, `/api/orgs/${encodeURIComponent(orgA.id)}/members`, {
        headers: bearerHeaders(token, {
          'X-Pulse-Org-ID': orgA.id,
          'X-Org-ID': orgA.id,
        }),
      });
      expect(ownMembersRes.status()).toBe(200);

      const otherMembersRes = await apiRequest(page, `/api/orgs/${encodeURIComponent(orgB.id)}/members`, {
        headers: bearerHeaders(token, {
          'X-Pulse-Org-ID': orgB.id,
          'X-Org-ID': orgB.id,
        }),
      });
      expect([403, 404]).toContain(otherMembersRes.status());
    } finally {
      if (tokenID) {
        await apiRequest(page, `/api/security/tokens/${encodeURIComponent(tokenID)}`, {
          method: 'DELETE',
        });
      }
      await deleteOrg(page, orgB.id);
      await deleteOrg(page, orgA.id);
    }
  });
});
