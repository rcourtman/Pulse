import { expect, test } from '@playwright/test';
import {
  apiRequest,
  createOrg,
  deleteOrg,
  ensureAuthenticated,
  E2E_CREDENTIALS,
  isMultiTenantEnabled,
  switchOrg,
  waitForAppShell,
} from './helpers';

type Organization = {
  id: string;
  displayName?: string;
};

type OrganizationMember = {
  userId: string;
  role: string;
};

type OrganizationInvitation = {
  userId: string;
  role: string;
};

type APITokenCreateResponse = {
  token?: string;
};

type APIErrorResponse = {
  code?: string;
  error?: string;
};

type OutgoingShare = {
  id: string;
  targetOrgId: string;
  resourceType: string;
  resourceId: string;
  resourceName?: string;
  accessRole: string;
  status?: string;
  acceptedAt?: string;
  acceptedBy?: string;
};

type IncomingShare = {
  id?: string;
  sourceOrgId?: string;
  sourceOrgID?: string;
  sourceOrgName?: string;
  resourceType: string;
  resourceId: string;
  resourceName?: string;
  accessRole: string;
  status?: string;
  acceptedAt?: string;
  acceptedBy?: string;
};

type Permission = {
  action: string;
  resource: string;
  effect?: string;
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

        await page.reload({ waitUntil: 'domcontentloaded' });
        await waitForAppShell(page);

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
        expect(addMemberRes.status()).toBe(202);

        const invitationsRes = await apiRequest(
          page,
          `/api/orgs/${encodeURIComponent(orgID)}/invitations`,
        );
        expect(invitationsRes.ok()).toBeTruthy();
        const invitations = (await invitationsRes.json()) as OrganizationInvitation[];
        expect(invitations.some((invitation) => invitation.userId === 'testuser')).toBeTruthy();

        const revokeInvitationRes = await apiRequest(
          page,
          `/api/orgs/${encodeURIComponent(orgID)}/invitations/testuser`,
          { method: 'DELETE' },
        );
        expect(revokeInvitationRes.ok()).toBeTruthy();

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

  test('Scenario 5: self role modification is denied', async ({ page }) => {
    await ensureAuthenticated(page);

    const currentUsername = E2E_CREDENTIALS.username;
    const updateRes = await apiRequest(
      page,
      `/api/admin/users/${encodeURIComponent(currentUsername)}/roles`,
      {
        method: 'PUT',
        data: { roleIds: ['viewer'] },
        headers: { 'Content-Type': 'application/json' },
      },
    );

    expect(updateRes.status()).toBe(403);

    const payload = (await updateRes.json()) as APIErrorResponse;
    expect(payload.code).toBe('self_modification_denied');
  });

  test('Scenario 6: cross-org share preserves intended access role and requires target acceptance', async ({ page }) => {
    await ensureAuthenticated(page);

    const mtEnabled = await isMultiTenantEnabled(page);
    test.skip(!mtEnabled, 'Multi-tenant feature not enabled in this environment');

    const orgA = await createOrg(page, `E2E Share Source ${Date.now()}`);
    const orgB = await createOrg(page, `E2E Share Target ${Date.now()}`);
    const outboundResourceName = `Shared View Outbound ${Date.now()}`;
    const inboundResourceName = `Shared View Inbound ${Date.now()}`;

    try {
      const createShareRes = await apiRequest(page, `/api/orgs/${encodeURIComponent(orgA.id)}/shares`, {
        method: 'POST',
        data: {
          targetOrgId: orgB.id,
          resourceType: 'view',
          resourceId: `shared-view-${Date.now()}`,
          resourceName: outboundResourceName,
          accessRole: 'editor',
        },
        headers: { 'Content-Type': 'application/json' },
      });
      expect(createShareRes.status()).toBe(201);

      const createdShare = (await createShareRes.json()) as OutgoingShare;
      expect(createdShare.targetOrgId).toBe(orgB.id);
      expect(createdShare.accessRole).toBe('editor');
      expect(createdShare.status).toBe('pending');

      const createIncomingShareRes = await apiRequest(
        page,
        `/api/orgs/${encodeURIComponent(orgB.id)}/shares`,
        {
          method: 'POST',
          data: {
            targetOrgId: orgA.id,
            resourceType: 'view',
            resourceId: `shared-view-inbound-${Date.now()}`,
            resourceName: inboundResourceName,
            accessRole: 'viewer',
          },
          headers: { 'Content-Type': 'application/json' },
        },
      );
      expect(createIncomingShareRes.status()).toBe(201);
      const createdIncomingShare = (await createIncomingShareRes.json()) as OutgoingShare;
      expect(createdIncomingShare.status).toBe('pending');

      const outgoingRes = await apiRequest(page, `/api/orgs/${encodeURIComponent(orgA.id)}/shares`);
      expect(outgoingRes.ok()).toBeTruthy();
      const outgoingShares = (await outgoingRes.json()) as OutgoingShare[];
      const outgoingMatch = outgoingShares.find((share) => share.id === createdShare.id);
      expect(outgoingMatch?.accessRole).toBe('editor');
      expect(outgoingMatch?.status).toBe('pending');

      const incomingRes = await apiRequest(
        page,
        `/api/orgs/${encodeURIComponent(orgB.id)}/shares/incoming`,
      );
      expect(incomingRes.ok()).toBeTruthy();
      const incomingShares = (await incomingRes.json()) as IncomingShare[];
      const incomingMatch = incomingShares.find(
        (share) => (share.sourceOrgId ?? share.sourceOrgID) === orgA.id &&
          share.resourceId === createdShare.resourceId,
      );
      expect(incomingMatch?.accessRole).toBe('editor');
      expect(incomingMatch?.status).toBe('pending');

      await switchOrg(page, orgA.id);
      await page.goto('/settings/organization/sharing', { waitUntil: 'domcontentloaded' });
      await page.waitForURL(/\/settings\/organization\/sharing/, { timeout: 15_000 });

      await expect(page.getByRole('heading', { level: 1, name: 'Organization Sharing' })).toBeVisible();
      await expect(page.getByText(outboundResourceName, { exact: true })).toBeVisible();
      await expect(page.getByText(inboundResourceName, { exact: true })).toBeVisible();
      await expect(page.getByText('Pending approval')).toHaveCount(2);

      const incomingRow = page.locator('tr').filter({ hasText: inboundResourceName }).first();
      await expect(incomingRow.getByRole('button', { name: 'Accept' })).toBeVisible();
      await expect(incomingRow.getByRole('button', { name: 'Decline' })).toBeVisible();

      await incomingRow.getByRole('button', { name: 'Accept' }).click();
      await expect(incomingRow.getByText('Active')).toBeVisible();
      await expect(incomingRow.getByRole('button', { name: 'Accept' })).toHaveCount(0);
      await expect(incomingRow.getByRole('button', { name: 'Remove' })).toBeVisible();

      const acceptedIncomingRes = await apiRequest(
        page,
        `/api/orgs/${encodeURIComponent(orgA.id)}/shares/incoming`,
      );
      expect(acceptedIncomingRes.ok()).toBeTruthy();
      const acceptedIncomingShares = (await acceptedIncomingRes.json()) as IncomingShare[];
      const acceptedIncomingMatch = acceptedIncomingShares.find(
        (share) => share.resourceId === createdIncomingShare.resourceId,
      );
      expect(acceptedIncomingMatch?.status).toBe('accepted');
      expect(acceptedIncomingMatch?.accessRole).toBe('viewer');
    } finally {
      await deleteOrg(page, orgB.id);
      await deleteOrg(page, orgA.id);
    }
  });

  test('Scenario 7: role changes update effective permissions only in the scoped org', async ({ page }) => {
    await ensureAuthenticated(page);

    const mtEnabled = await isMultiTenantEnabled(page);
    test.skip(!mtEnabled, 'Multi-tenant feature not enabled in this environment');

    const orgA = await createOrg(page, `E2E RBAC Scope A ${Date.now()}`);
    const orgB = await createOrg(page, `E2E RBAC Scope B ${Date.now()}`);
    const scopedHeadersA = {
      'Content-Type': 'application/json',
      'X-Pulse-Org-ID': orgA.id,
      'X-Org-ID': orgA.id,
    };
    const scopedHeadersB = {
      'Content-Type': 'application/json',
      'X-Pulse-Org-ID': orgB.id,
      'X-Org-ID': orgB.id,
    };

    try {
      const viewerUpdateRes = await apiRequest(
        page,
        `/api/admin/users/${encodeURIComponent('testuser')}/roles`,
        {
          method: 'PUT',
          data: { roleIds: ['viewer'] },
          headers: scopedHeadersA,
        },
      );
      expect(viewerUpdateRes.status()).toBe(204);

      const viewerPermsRes = await apiRequest(
        page,
        `/api/admin/users/${encodeURIComponent('testuser')}/permissions`,
        { headers: scopedHeadersA },
      );
      expect(viewerPermsRes.ok()).toBeTruthy();
      const viewerPerms = (await viewerPermsRes.json()) as Permission[];
      expect(viewerPerms).toEqual(expect.arrayContaining([
        expect.objectContaining({ action: 'read', resource: '*' }),
      ]));

      const adminUpdateRes = await apiRequest(
        page,
        `/api/admin/users/${encodeURIComponent('testuser')}/roles`,
        {
          method: 'PUT',
          data: { roleIds: ['admin'] },
          headers: scopedHeadersA,
        },
      );
      expect(adminUpdateRes.status()).toBe(204);

      const adminPermsRes = await apiRequest(
        page,
        `/api/admin/users/${encodeURIComponent('testuser')}/permissions`,
        { headers: scopedHeadersA },
      );
      expect(adminPermsRes.ok()).toBeTruthy();
      const adminPerms = (await adminPermsRes.json()) as Permission[];
      expect(adminPerms).toEqual(expect.arrayContaining([
        expect.objectContaining({ action: 'admin', resource: '*' }),
      ]));

      const otherOrgPermsRes = await apiRequest(
        page,
        `/api/admin/users/${encodeURIComponent('testuser')}/permissions`,
        { headers: scopedHeadersB },
      );
      expect(otherOrgPermsRes.ok()).toBeTruthy();
      const otherOrgPerms = (await otherOrgPermsRes.json()) as Permission[] | null;
      expect(otherOrgPerms ?? []).toEqual([]);
    } finally {
      await deleteOrg(page, orgB.id);
      await deleteOrg(page, orgA.id);
    }
  });
});
