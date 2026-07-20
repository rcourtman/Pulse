import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SCREENSHOT_PATH = '/tmp/vmware-ai-chat-read-recovery.png';

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
      `vmware-ai-chat-read-recovery-${workerInfo.project.name}.json`,
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

// The assistant transport moved from the retired /api/ai/chat SSE endpoint
// to session-scoped message turns, so scripting the whole exchange through
// route stubs would re-implement the streaming protocol inside the spec.
// Instead this observes a real turn against the mock assistant: the mention
// payload must travel the shared session path, and the browser must stay off
// provider-local vmware endpoints for the whole exchange.
test.describe('VMware AI chat read recovery', () => {
  test.setTimeout(180_000);

  test('keeps VMware chat turns on shared assistant paths', async ({ page }) => {
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
    const textarea = page.getByPlaceholder('Ask about your infrastructure...');
    await expect(textarea).toBeVisible();

    // First websocket state frame can lag on a freshly booted backend.
    await textarea.focus();
    await textarea.fill('@warehouse');
    const mentionListbox = page.getByRole('listbox', { name: 'Assistant resources' });
    await expect(mentionListbox).toBeVisible({ timeout: 30_000 });
    const vmOption = mentionListbox
      .getByRole('option')
      .filter({ hasText: /warehouse-api-01/ })
      .first();
    await expect(vmOption).toBeVisible({ timeout: 30_000 });
    await textarea.press('Enter');
    await expect(textarea).toHaveValue('@warehouse-api-01 ');

    await textarea.fill('@warehouse-api-01 show me recent status');
    await textarea.press('Enter');

    // Chat turns ride the shared assistant transport (websocket-backed since
    // the assistant modernization, so REST route interception cannot observe
    // the payload). The proof is the rendered exchange: the mention-carrying
    // user message lands in the conversation and the mock assistant answers
    // the turn, all without the browser touching provider-local vmware
    // endpoints.
    await expect(
      page.getByText('@warehouse-api-01 show me recent status').first(),
    ).toBeVisible({ timeout: 30_000 });
    await expect(page.getByText('Pulse mock Assistant').first()).toBeVisible({
      timeout: 30_000,
    });

    expect(unexpectedVMwareRequests).toEqual([]);

    // Viewport-only: full-page capture can hang while the assistant panel
    // animates, and the screenshot is an artifact rather than an assertion.
    await page.screenshot({ path: SCREENSHOT_PATH });
  });
});
