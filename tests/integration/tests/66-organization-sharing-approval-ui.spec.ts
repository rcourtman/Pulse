import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const test = base.extend<{}, WorkerFixtures>({
  storageState: async ({ authStorageStatePath }, use) => {
    await use(authStorageStatePath);
  },
  authStorageStatePath: [async ({ browser }, use, workerInfo) => {
    const storageStatePath = path.resolve(
      __dirname,
      '..',
      '..',
      'tmp',
      'playwright-auth',
      `organization-sharing-approval-ui-${workerInfo.project.name}.json`,
    );
    fs.mkdirSync(path.dirname(storageStatePath), { recursive: true });
    await createAuthenticatedStorageState(browser, storageStatePath);
    try {
      await use(storageStatePath);
    } finally {
      fs.rmSync(storageStatePath, { force: true });
    }
  }, { scope: 'worker' }],
});

const jsonResponse = (body: unknown) => ({
  status: 200,
  contentType: 'application/json',
  body: JSON.stringify(body),
});

test.describe('Organization sharing approval UI', () => {
  test('renders pending shares and promotes accepted incoming shares in-browser', async ({ page }) => {
    const currentUser = 'admin';
    const currentOrg = {
      id: 'default',
      displayName: 'Default Organization',
      ownerUserId: currentUser,
    };
    const peerOrg = {
      id: 'org-b',
      displayName: 'Organization B',
      ownerUserId: currentUser,
    };
    const resourceTimestamp = '2026-04-22T10:30:00Z';
    const outgoingShare = {
      id: 'share-out-1',
      targetOrgId: peerOrg.id,
      resourceType: 'view',
      resourceId: 'alerts-outbound',
      resourceName: 'Outbound Shared View',
      accessRole: 'editor',
      status: 'pending',
      createdAt: resourceTimestamp,
      createdBy: currentUser,
    };
    const incomingShare = {
      id: 'share-in-1',
      targetOrgId: currentOrg.id,
      sourceOrgId: peerOrg.id,
      sourceOrgName: peerOrg.displayName,
      resourceType: 'view',
      resourceId: 'alerts-inbound',
      resourceName: 'Inbound Shared View',
      accessRole: 'viewer',
      status: 'pending',
      createdAt: resourceTimestamp,
      createdBy: currentUser,
      acceptedAt: '',
      acceptedBy: '',
    };

    await page.addInitScript((orgId) => {
      sessionStorage.setItem('pulse_org_id', orgId);
      localStorage.setItem('pulse_org_id', orgId);
      document.cookie = `pulse_org_id=${encodeURIComponent(orgId)}; Path=/; SameSite=Lax`;
    }, currentOrg.id);

    await page.route(/\/api\/license\/runtime-capabilities(?:\?.*)?$/, async (route) => {
      await route.fulfill(jsonResponse({
        capabilities: ['multi_tenant', 'rbac', 'relay'],
        limits: [],
        hosted_mode: false,
        max_history_days: 30,
      }));
    });

    await page.route(/\/api\/orgs(?:\?.*)?$/, async (route) => {
      await route.fulfill(jsonResponse([currentOrg, peerOrg]));
    });

    await page.route(new RegExp(`/api/orgs/${currentOrg.id}(?:\\?.*)?$`), async (route) => {
      await route.fulfill(jsonResponse(currentOrg));
    });

    await page.route(new RegExp(`/api/orgs/${currentOrg.id}/members(?:\\?.*)?$`), async (route) => {
      await route.fulfill(jsonResponse([]));
    });

    await page.route(
      new RegExp(`/api/orgs/${currentOrg.id}/shares/incoming/${incomingShare.id}/accept(?:\\?.*)?$`),
      async (route) => {
        incomingShare.status = 'accepted';
        incomingShare.acceptedAt = resourceTimestamp;
        incomingShare.acceptedBy = currentUser;
        await route.fulfill(jsonResponse(incomingShare));
      },
    );

    await page.route(new RegExp(`/api/orgs/${currentOrg.id}/shares/incoming(?:\\?.*)?$`), async (route) => {
      await route.fulfill(jsonResponse([incomingShare]));
    });

    await page.route(new RegExp(`/api/orgs/${currentOrg.id}/shares(?:\\?.*)?$`), async (route) => {
      await route.fulfill(jsonResponse([outgoingShare]));
    });

    await page.route(/\/api\/resources(?:\?.*)?$/, async (route) => {
      await route.fulfill(jsonResponse([
        {
          id: 'vm-100',
          type: 'vm',
          name: 'Alpha VM',
          displayName: 'Alpha VM',
          platformType: 'proxmox-pve',
          sourceType: 'api',
          status: 'running',
          lastSeen: 1713781800000,
        },
      ]));
    });

    await page.goto('/settings/organization/sharing', { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/settings\/organization\/sharing/, { timeout: 15_000 });
    await page.waitForFunction(() => {
      const root = document.getElementById('root');
      return root !== null && root.childElementCount > 0;
    });

    await expect(page.getByRole('heading', { level: 1, name: 'Organization Sharing' })).toBeVisible();
    await expect(page.getByText('Outbound Shared View', { exact: true })).toBeVisible();
    await expect(page.getByText('Inbound Shared View', { exact: true })).toBeVisible();
    await expect(page.getByText('Pending approval')).toHaveCount(2);

    const outgoingRow = page.locator('tr').filter({ hasText: 'Outbound Shared View' }).first();
    await expect(outgoingRow.getByText('Pending approval')).toBeVisible();

    const incomingRow = page.locator('tr').filter({ hasText: 'Inbound Shared View' }).first();
    await expect(incomingRow.getByRole('button', { name: 'Accept' })).toBeVisible();
    await expect(incomingRow.getByRole('button', { name: 'Decline' })).toBeVisible();
    await expect(incomingRow.getByText('Waiting for a target organization admin to accept.')).toBeVisible();

    await incomingRow.getByRole('button', { name: 'Accept' }).click();

    await expect(incomingRow.getByText('Active')).toBeVisible();
    await expect(incomingRow.getByRole('button', { name: 'Accept' })).toHaveCount(0);
    await expect(incomingRow.getByRole('button', { name: 'Remove' })).toBeVisible();
    await expect(incomingRow.getByText('Accepted')).toBeVisible();
  });
});
