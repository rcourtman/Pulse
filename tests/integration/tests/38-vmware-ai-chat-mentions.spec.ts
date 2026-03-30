import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SCREENSHOT_PATH = '/tmp/vmware-ai-chat-mentions.png';

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
      `vmware-ai-chat-mentions-${workerInfo.project.name}.json`,
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

test.describe('VMware AI chat mentions', () => {
  test.setTimeout(180_000);

  test('surfaces VMware resources through shared assistant mention paths only', async ({
    page,
  }) => {
    const unexpectedVMwareRequests: string[] = [];

    await page.route('**/api/vmware/**', async (route) => {
      const url = new URL(route.request().url());
      const method = route.request().method();
      if (method === 'GET' && url.pathname === '/api/vmware/connections') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        });
        return;
      }

      unexpectedVMwareRequests.push(`${method} ${url.pathname}`);
      await route.abort();
    });

    await page.route('**/api/ai/status', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ running: true, engine: 'test' }),
      });
    });

    await page.route('**/api/ai/sessions', async (route) => {
      if (route.request().method() !== 'GET') {
        await route.continue();
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      });
    });

    await page.route('**/api/ai/settings', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          model: 'openai:gpt-4o-mini',
          chat_model: '',
          control_level: 'read_only',
          discovery_enabled: true,
        }),
      });
    });

    await page.route('**/api/ai/models', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          models: [{ id: 'openai:gpt-4o-mini', name: 'GPT-4o mini' }],
        }),
      });
    });

    await page.route('**/api/resources**', async (route) => {
      const requestUrl = new URL(route.request().url());
      if (requestUrl.pathname !== '/api/resources') {
        await route.continue();
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              id: 'agent-vmware-host-1',
              type: 'agent',
              name: 'esxi-01.lab.local',
              status: 'online',
              lastSeen: '2026-03-30T09:00:00Z',
              platformType: 'vmware-vsphere',
              sourceType: 'api',
              sources: ['vmware-vsphere'],
              canonicalIdentity: {
                displayName: 'ESXi 01',
                hostname: 'esxi-01.lab.local',
                primaryId: 'vmware:vc-1:host:host-101',
              },
              agent: {
                agentId: 'vc-1:host:host-101',
                hostname: 'esxi-01.lab.local',
                platform: 'VMware ESXi',
              },
              vmware: {
                connectionId: 'vc-1',
                connectionName: 'Lab VC',
                managedObjectId: 'host-101',
                entityType: 'host',
              },
            },
            {
              id: 'vm-vmware-1',
              type: 'vm',
              name: 'app-01',
              status: 'running',
              lastSeen: '2026-03-30T09:00:00Z',
              parentId: 'agent-vmware-host-1',
              parentName: 'esxi-01.lab.local',
              platformType: 'vmware-vsphere',
              sourceType: 'api',
              sources: ['vmware-vsphere'],
              canonicalIdentity: {
                displayName: 'App 01',
                hostname: 'app-01.internal',
                primaryId: 'vmware:vc-1:vm:vm-201',
              },
              vmware: {
                connectionId: 'vc-1',
                connectionName: 'Lab VC',
                managedObjectId: 'vm-201',
                entityType: 'vm',
                runtimeHostName: 'esxi-01.lab.local',
              },
            },
            {
              id: 'storage-vmware-1',
              type: 'storage',
              name: 'nvme-primary',
              status: 'online',
              lastSeen: '2026-03-30T09:00:00Z',
              parentName: 'Lab VC',
              platformType: 'vmware-vsphere',
              sourceType: 'api',
              sources: ['vmware-vsphere'],
              canonicalIdentity: {
                displayName: 'NVMe Primary',
                primaryId: 'vmware:vc-1:datastore:datastore-11',
              },
              storage: {
                type: 'vmfs',
                platform: 'vmware-vsphere',
              },
              vmware: {
                connectionId: 'vc-1',
                connectionName: 'Lab VC',
                managedObjectId: 'datastore-11',
                entityType: 'datastore',
              },
            },
          ],
          meta: {
            page: 1,
            limit: 100,
            total: 3,
            totalPages: 1,
          },
        }),
      });
    });

    const resourcesLoaded = page.waitForResponse((response) => {
      const url = new URL(response.url());
      return url.pathname === '/api/resources' && response.request().method() === 'GET';
    });

    await page.goto('/infrastructure?source=vmware-vsphere', {
      waitUntil: 'domcontentloaded',
    });
    await page.waitForURL(/\/infrastructure\?source=vmware-vsphere/, {
      timeout: 15_000,
    });
    await expect(page.getByTestId('infrastructure-page')).toBeVisible();
    await resourcesLoaded;

    await page.getByRole('button', { name: 'Expand Pulse Assistant' }).click();
    await expect(page.getByRole('heading', { name: 'Pulse Assistant', exact: true })).toBeVisible();

    const textarea = page.getByPlaceholder('Ask about your infrastructure...');
    await textarea.click();
    await textarea.pressSequentially('@esxi');

    const mentionSurface = page.locator('[data-mention-autocomplete]');
    const hostOption = mentionSurface.getByRole('button', { name: /ESXi 01 agent/ }).first();
    await expect(mentionSurface.getByText('Resources')).toBeVisible();
    await expect(hostOption).toBeVisible();
    await expect(mentionSurface).toContainText('agent');
    await hostOption.click();
    await expect(textarea).toHaveValue('@ESXi 01 ');

    await textarea.fill('');
    await textarea.pressSequentially('@app');

    await expect(mentionSurface.getByText('Resources')).toBeVisible();
    await expect(mentionSurface.getByRole('button', { name: /App 01/ })).toBeVisible();
    await expect(mentionSurface).toContainText('vm');
    await expect(mentionSurface).toContainText('esxi-01.lab.local');
    await mentionSurface.getByRole('button', { name: /App 01/ }).click();
    await expect(textarea).toHaveValue('@App 01 ');

    await textarea.fill('');
    await textarea.pressSequentially('@nvme');
    await expect(mentionSurface.getByRole('button', { name: /NVMe Primary/ })).toBeVisible();
    await expect(mentionSurface).toContainText('storage');
    await expect(mentionSurface).toContainText('Lab VC');
    await mentionSurface.getByRole('button', { name: /NVMe Primary/ }).click();
    await expect(textarea).toHaveValue('@NVMe Primary ');

    expect(unexpectedVMwareRequests).toEqual([]);
    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
