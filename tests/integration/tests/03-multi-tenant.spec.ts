import { expect, test } from '@playwright/test';
import {
  apiRequest,
  createOrg,
  deleteOrg,
  ensureAuthenticated,
  isMultiTenantEnabled,
} from './helpers';

type Organization = {
  id: string;
  displayName?: string;
};

type OrganizationMember = {
  userId: string;
  role: string;
};

type APITokenCreateResponse = {
  token?: string;
};

const expectStatusIn = (status: number, allowed: number[], context: string) => {
  expect(
    allowed.includes(status),
    `${context}: expected status in [${allowed.join(', ')}], got ${status}`,
  ).toBeTruthy();
};

test.describe('Multi-tenant E2E flows', () => {
  test('Scenario 1: feature flag visibility', async ({ page }) => {
    await ensureAuthenticated(page);

    const mtEnabled = await isMultiTenantEnabled(page);
    let tempOrgID: string | null = null;

    try {
      if (mtEnabled) {
        const created = await createOrg(page, `E2E Visibility Org ${Date.now()}`);
        tempOrgID = created.id;

        await page.reload();
        await page.waitForLoadState('networkidle');

        await expect(page.getByLabel('Organization')).toBeVisible();
      } else {
        await expect(page.getByLabel('Organization')).toHaveCount(0);
      }
    } finally {
      if (tempOrgID) {
        await deleteOrg(page, tempOrgID);
      }
    }
  });

  test.describe.serial('Scenario 2: org CRUD lifecycle', () => {
    test('create, update, member manage, and delete org', async ({ page }) => {
      await ensureAuthenticated(page);

      const mtEnabled = await isMultiTenantEnabled(page);
      test.skip(!mtEnabled, 'Multi-tenant feature not enabled in this environment');

      const created = await createOrg(page, `E2E CRUD Org ${Date.now()}`);
      const orgID = created.id;
      const updatedName = `E2E CRUD Org Updated ${Date.now()}`;

      try {
        const listBeforeRes = await apiRequest(page, '/api/orgs');
        expect(listBeforeRes.ok()).toBeTruthy();
        const listBefore = (await listBeforeRes.json()) as Organization[];
        expect(listBefore.some((org) => org.id === orgID)).toBeTruthy();

        const updateRes = await apiRequest(page, `/api/orgs/${encodeURIComponent(orgID)}`, {
          method: 'PUT',
          data: { displayName: updatedName },
          headers: { 'Content-Type': 'application/json' },
        });
        expect(updateRes.ok()).toBeTruthy();

        const addMemberRes = await apiRequest(page, `/api/orgs/${encodeURIComponent(orgID)}/members`, {
          method: 'POST',
          data: { userId: 'testuser', role: 'viewer' },
          headers: { 'Content-Type': 'application/json' },
        });
        expect(addMemberRes.ok()).toBeTruthy();

        const membersRes = await apiRequest(page, `/api/orgs/${encodeURIComponent(orgID)}/members`);
        expect(membersRes.ok()).toBeTruthy();
        const members = (await membersRes.json()) as OrganizationMember[];
        expect(members.some((member) => member.userId === 'testuser')).toBeTruthy();

        const removeMemberRes = await apiRequest(
          page,
          `/api/orgs/${encodeURIComponent(orgID)}/members/testuser`,
          { method: 'DELETE' },
        );
        expect(removeMemberRes.ok()).toBeTruthy();

        await deleteOrg(page, orgID);

        const listAfterRes = await apiRequest(page, '/api/orgs');
        expect(listAfterRes.ok()).toBeTruthy();
        const listAfter = (await listAfterRes.json()) as Organization[];
        expect(listAfter.some((org) => org.id === orgID)).toBeFalsy();
      } finally {
        await deleteOrg(page, orgID);
      }
    });
  });

  test('Scenario 3: cross-org API isolation', async ({ page }) => {
    await ensureAuthenticated(page);

    const mtEnabled = await isMultiTenantEnabled(page);
    test.skip(!mtEnabled, 'Multi-tenant feature not enabled in this environment');

    const orgA = await createOrg(page, `E2E Isolation Org A ${Date.now()}`);
    const orgB = await createOrg(page, `E2E Isolation Org B ${Date.now()}`);

    try {
      const createTokenRes = await apiRequest(page, '/api/security/tokens', {
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
      expect(createTokenRes.ok()).toBeTruthy();

      const tokenPayload = (await createTokenRes.json()) as APITokenCreateResponse;
      const token = tokenPayload.token?.trim() || '';
      expect(token.length).toBeGreaterThan(0);

      const orgContextHeaders = {
        Authorization: `Bearer ${token}`,
        'X-Pulse-Org-ID': orgA.id,
        'X-Org-ID': orgA.id,
      };

      const ownMembersRes = await apiRequest(page, `/api/orgs/${encodeURIComponent(orgA.id)}/members`, {
        headers: orgContextHeaders,
      });
      const membersRes = await apiRequest(page, `/api/orgs/${encodeURIComponent(orgB.id)}/members`, {
        headers: orgContextHeaders,
      });
      const sharesRes = await apiRequest(page, `/api/orgs/${encodeURIComponent(orgB.id)}/shares`, {
        headers: orgContextHeaders,
      });

      expectStatusIn(ownMembersRes.status(), [200], 'org-bound token own-org members access');
      const membersStatus = membersRes.status();
      const sharesStatus = sharesRes.status();

      expectStatusIn(membersStatus, [403, 404], 'cross-org members access');
      expectStatusIn(sharesStatus, [403, 404], 'cross-org shares access');
    } finally {
      await deleteOrg(page, orgB.id);
      await deleteOrg(page, orgA.id);
    }
  });

  test('Scenario 4: kill switch when multi-tenant is disabled', async ({ page }) => {
    await ensureAuthenticated(page);

    const mtEnabled = await isMultiTenantEnabled(page);
    test.skip(mtEnabled, 'Multi-tenant is enabled; kill-switch behavior cannot be asserted without changing license');

    const postRes = await apiRequest(page, '/api/orgs', {
      method: 'POST',
      data: {
        id: `kill-switch-${Date.now()}`,
        displayName: 'Kill Switch Org',
      },
      headers: { 'Content-Type': 'application/json' },
    });

    const listRes = await apiRequest(page, '/api/orgs');

    expectStatusIn(postRes.status(), [501, 402, 403], 'create org while MT disabled');
    expectStatusIn(listRes.status(), [501, 402, 403], 'list orgs while MT disabled');
  });
});
