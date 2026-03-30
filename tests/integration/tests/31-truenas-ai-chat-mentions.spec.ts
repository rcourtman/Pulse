import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const SCREENSHOT_PATH = '/tmp/truenas-ai-chat-mentions.png';

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
      `truenas-ai-chat-mentions-${workerInfo.project.name}.json`,
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

test.describe('TrueNAS AI chat mentions', () => {
  test.setTimeout(180_000);

  test('surfaces TrueNAS apps as canonical assistant mention targets', async ({ page }) => {
    await page.route('**/api/truenas/connections', async (route) => {
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
              id: 'agent:truenas-main',
              type: 'agent',
              name: 'truenas-main',
              status: 'online',
              lastSeen: '2026-03-30T09:00:00Z',
              sources: ['agent', 'truenas'],
              canonicalIdentity: {
                displayName: 'TrueNAS Main',
                hostname: 'truenas-main',
                platformId: 'truenas-main',
              },
              agent: {
                agentId: 'truenas-main',
                hostname: 'truenas-main',
                platform: 'truenas',
              },
            },
            {
              id: 'app-container:truenas-main:nextcloud',
              type: 'app-container',
              name: 'nextcloud',
              status: 'running',
              lastSeen: '2026-03-30T09:00:00Z',
              parentId: 'agent:truenas-main',
              parentName: 'truenas-main',
              sources: ['truenas'],
              tags: ['truenas', 'app'],
              canonicalIdentity: {
                displayName: 'Nextcloud',
                hostname: 'truenas-main',
                primaryId: 'nextcloud',
                platformId: 'truenas-main',
              },
              docker: {
                containerId: 'nextcloud',
                hostname: 'truenas-main',
              },
            },
          ],
          meta: {
            page: 1,
            limit: 100,
            total: 2,
            totalPages: 1,
          },
        }),
      });
    });

    await page.goto('/settings/infrastructure/platforms/truenas', {
      waitUntil: 'domcontentloaded',
    });
    await page.waitForURL(/\/settings\/infrastructure\/platforms\/truenas/, {
      timeout: 15_000,
    });

    const resourcesLoaded = page.waitForResponse((response) => {
      const url = new URL(response.url());
      return url.pathname === '/api/resources' && response.request().method() === 'GET';
    });
    await page.getByRole('button', { name: 'Expand Pulse Assistant' }).click();
    await expect(page.getByRole('heading', { name: 'Pulse Assistant', exact: true })).toBeVisible();
    await resourcesLoaded;

    const textarea = page.getByPlaceholder('Ask about your infrastructure...');
    await textarea.click();
    await textarea.pressSequentially('@next');

    const mentionSurface = page.locator('[data-mention-autocomplete]');
    await expect(mentionSurface.getByText('Resources')).toBeVisible();
    await expect(mentionSurface.getByRole('button', { name: /Nextcloud/ })).toBeVisible();
    await expect(mentionSurface).toContainText('app-container');
    await expect(mentionSurface).toContainText('truenas-main');

    await mentionSurface.getByRole('button', { name: /Nextcloud/ }).click();
    await expect(textarea).toHaveValue('@Nextcloud ');

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
