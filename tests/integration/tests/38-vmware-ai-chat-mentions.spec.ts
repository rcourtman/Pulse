import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';
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

// Mention candidates come from live websocket state, so the assertions pin
// the mock scenario's vSphere inventory (esxi hosts and warehouse VMs)
// instead of stubbed REST payloads from the retired /infrastructure IA.
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
        await route.continue();
        return;
      }
      unexpectedVMwareRequests.push(`${method} ${url.pathname}`);
      await route.abort();
    });

    await page.goto('/vmware', { waitUntil: 'domcontentloaded' });
    await expect(page.locator('[data-testid="vmware-page"]')).toBeVisible();

    await page.getByRole('button', { name: 'Ask Pulse Assistant about vSphere' }).click();
    await expect(
      page.getByPlaceholder('Ask about your infrastructure...'),
    ).toBeVisible();

    const textarea = page.getByPlaceholder('Ask about your infrastructure...');
    const mentionListbox = page.getByRole('listbox', { name: 'Assistant resources' });

    // ESXi hosts surface as shared agent mention targets.
    // First websocket state frame can lag on a freshly booted backend, and
    // the candidate list refreshes as state arrives.
    await textarea.click();
    await textarea.pressSequentially('@esxi');
    await expect(mentionListbox).toBeVisible({ timeout: 30_000 });
    const hostOption = mentionListbox
      .getByRole('option', { name: /esxi-01\.lab\.local: agent/ })
      .first();
    await expect(hostOption).toBeVisible({ timeout: 30_000 });
    await hostOption.click();
    await expect(textarea).toHaveValue('@esxi-01.lab.local ');

    // API-backed vSphere VMs use the same shared mention contract, carrying
    // their runtime host as the mention node.
    await textarea.fill('');
    await textarea.pressSequentially('@warehouse');
    await expect(mentionListbox).toBeVisible();
    const vmOption = mentionListbox
      .getByRole('option', { name: /warehouse-api-01: vm on esxi-01\.lab\.local/ })
      .first();
    await expect(vmOption).toBeVisible({ timeout: 30_000 });
    await vmOption.click();
    await expect(textarea).toHaveValue('@warehouse-api-01 ');

    expect(unexpectedVMwareRequests).toEqual([]);
    // Viewport-only: full-page capture can hang while the assistant panel
    // animates, and the screenshot is an artifact rather than an assertion.
    await page.screenshot({ path: SCREENSHOT_PATH });
  });
});
