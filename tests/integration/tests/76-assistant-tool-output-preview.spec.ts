import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect, type Route } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const test = base.extend<{}, WorkerFixtures>({
  storageState: async ({ authStorageStatePath }, use) => {
    await use(authStorageStatePath);
  },
  authStorageStatePath: [
    async ({ browser }, use, workerInfo) => {
      const storageStatePath = path.resolve(
        __dirname,
        '..',
        '..',
        'tmp',
        'playwright-auth',
        `assistant-tool-output-preview-${workerInfo.project.name}.json`,
      );
      fs.mkdirSync(path.dirname(storageStatePath), { recursive: true });
      await createAuthenticatedStorageState(browser, storageStatePath);
      try {
        await use(storageStatePath);
      } finally {
        fs.rmSync(storageStatePath, { force: true });
      }
    },
    { scope: 'worker' },
  ],
});

test.describe('Assistant tool output preview', () => {
  test('renders bounded plain-text tool output in the drawer before raw details', async ({
    page,
  }) => {
    const sessionId = 'session-tool-output-preview';

    await page.route('**/api/truenas/connections', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
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
          data: [],
          meta: {
            page: 1,
            limit: 100,
            total: 0,
            totalPages: 1,
          },
        }),
      });
    });

    await page.route('**/api/ai/status', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ running: true, engine: 'test' }),
      });
    });

    const fulfillAISettings = async (route: Route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          model: 'openrouter:deepseek/deepseek-chat',
          chat_model: '',
          control_level: 'read_only',
          discovery_enabled: true,
        }),
      });
    };
    await page.route('**/api/ai/settings', fulfillAISettings);
    await page.route('**/api/settings/ai', fulfillAISettings);

    await page.route('**/api/ai/models', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          models: [{ id: 'openrouter:deepseek/deepseek-chat', name: 'DeepSeek Chat' }],
        }),
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
        body: JSON.stringify([
          {
            id: sessionId,
            title: 'Tool output preview proof',
            created_at: '2026-06-06T09:00:00Z',
            updated_at: '2026-06-06T09:01:00Z',
            message_count: 2,
          },
        ]),
      });
    });

    await page.route('**/api/ai/sessions/*/messages', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          {
            id: 'msg-user-tool-preview',
            role: 'user',
            content: 'Show the output from the read-only command.',
            timestamp: '2026-06-06T09:00:00Z',
          },
          {
            id: 'msg-assistant-tool-preview',
            role: 'assistant',
            content: 'I checked the target.',
            timestamp: '2026-06-06T09:01:00Z',
            tool_calls: [
              {
                name: 'pulse_read',
                input:
                  '{"action":"exec","target_host":"current_resource","command":"printf preview-proof-0606"}',
                output:
                  'preview-proof-0606\nsecond-line\nthird-line\nfourth-line\nhidden-fifth-line',
                success: true,
              },
            ],
          },
        ]),
      });
    });

    await page.goto('/settings/infrastructure/platforms/truenas', {
      waitUntil: 'domcontentloaded',
    });
    await page.waitForURL(/\/settings\/infrastructure\/platforms\/truenas/, {
      timeout: 15_000,
    });

    await page.getByRole('button', { name: 'Expand Pulse Assistant' }).click();
    await expect(page.getByRole('heading', { name: 'Pulse Assistant', exact: true })).toBeVisible();

    await page.getByRole('button', { name: 'Resume Tool output preview proof' }).click();
    await expect(page.getByText('$ printf preview-proof-0606 on current resource')).toBeVisible();

    const preview = page.locator('pre[aria-label="Tool output preview"]');
    await expect(preview).toBeVisible();
    await expect(preview).toContainText('preview-proof-0606');
    await expect(preview).toContainText('second-line');
    await expect(preview).toContainText('fourth-line');
    await expect(preview).toContainText('...');
    await expect(preview).not.toContainText('hidden-fifth-line');
    await expect(page.getByText('hidden-fifth-line')).toHaveCount(0);

    await page.getByRole('button', { name: 'Details' }).click();
    await expect(page.getByText('hidden-fifth-line')).toBeVisible();
    await expect(page.getByText(/"target_host":"current_resource"/)).toBeVisible();
  });
});
