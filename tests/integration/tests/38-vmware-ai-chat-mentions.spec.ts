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

  test('surfaces VMware hosts, VMs, and datastores as assistant mention targets', async ({
    page,
  }) => {
    // Mention candidates come from live mock websocket state (REST resource
    // stubs get overwritten by state frames), so the assertions pin the mock
    // vSphere fixture graph: esxi hosts, warehouse VMs, and the nvme-primary
    // datastore on the Lab vCenter connection.
    //
    // Regression context: datastores classify as policy-sensitive, and the
    // mention builder once applied governed (redacted) display labels, which
    // collapsed every storage resource into an identical un-searchable
    // "storage (status)" placeholder. Mention labels now use the ungoverned
    // infrastructure display name.
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

    await page.goto('/vmware', { waitUntil: 'domcontentloaded' });

    await page.getByRole('button', { name: /Ask Pulse Assistant/ }).click();
    await expect(page.getByRole('heading', { name: 'Pulse Assistant', exact: true })).toBeVisible();

    const textarea = page.getByPlaceholder('Ask about your infrastructure...');
    const mentionSurface = page.locator('[data-mention-autocomplete]');

    await textarea.click();
    await textarea.pressSequentially('@esxi');

    const hostOption = mentionSurface.getByRole('option', { name: /esxi-01\.lab\.local/ }).first();
    await expect(mentionSurface.getByText('Resources')).toBeVisible();
    await expect(hostOption).toBeVisible();
    await expect(mentionSurface).toContainText('agent');
    await hostOption.click();
    await expect(textarea).toHaveValue('@esxi-01.lab.local ');

    await textarea.fill('');
    await textarea.pressSequentially('@warehouse-db');

    await expect(mentionSurface.getByRole('option', { name: /warehouse-db-01/ })).toBeVisible();
    await expect(mentionSurface).toContainText('vm');
    await mentionSurface.getByRole('option', { name: /warehouse-db-01/ }).first().click();
    await expect(textarea).toHaveValue('@warehouse-db-01 ');

    await textarea.fill('');
    await textarea.pressSequentially('@nvme-pr');

    await expect(mentionSurface.getByRole('option', { name: /nvme-primary/ })).toBeVisible();
    await expect(mentionSurface).toContainText('storage');
    await expect(mentionSurface).toContainText('Lab vCenter');
    await mentionSurface.getByRole('option', { name: /nvme-primary/ }).first().click();
    await expect(textarea).toHaveValue('@nvme-primary ');

    // Storage mentions are platform-agnostic: the TrueNAS pool stays reachable
    // from a composer opened on the vSphere page.
    await textarea.fill('');
    await textarea.pressSequentially('@tank');

    await expect(mentionSurface.getByRole('option', { name: /^Mention tank:/ })).toBeVisible();
    await expect(mentionSurface).toContainText('storage');

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
