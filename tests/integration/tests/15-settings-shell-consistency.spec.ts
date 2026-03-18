import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';
import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const SETTINGS_SHELL_ROUTES = [
  {
    route: '/settings/system-general',
    title: 'General Settings',
    description: 'Configure appearance, layout, and default monitoring cadence.',
  },
  {
    route: '/settings/organization',
    title: 'Organization Overview',
    description: 'Review organization metadata, membership footprint, and ownership.',
  },
  {
    route: '/settings/organization/billing',
    title: 'Organization Billing',
    description: 'Track plan tier, usage, and upgrade options for multi-tenant deployments.',
  },
  {
    route: '/settings/system-relay',
    title: 'Remote Access',
    description: 'Configure Pulse relay connectivity for secure remote access (mobile rollout coming soon).',
  },
  {
    route: '/settings/security-auth',
    title: 'Authentication',
    description: 'Manage password-based authentication and credential rotation.',
  },
  {
    route: '/settings/system-ai',
    title: 'AI Settings',
    description: 'Configure AI providers, model defaults, Pulse Assistant, and Patrol automation.',
  },
  {
    route: '/settings/system-updates',
    title: 'Updates',
    description: 'Manage version checks, update channels, and automatic update behavior.',
  },
  {
    route: '/settings/system-recovery',
    title: 'Recovery',
    description: 'Manage backup/snapshot polling plus configuration export and import workflows.',
  },
] as const;

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
      `settings-shell-consistency-${workerInfo.project.name}.json`,
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

test.describe('Settings shell consistency', () => {
  test.setTimeout(180_000);

  for (const panel of SETTINGS_SHELL_ROUTES) {
    test(`uses the canonical shell on ${panel.route}`, async ({ page }) => {
      await page.goto(panel.route, { waitUntil: 'domcontentloaded' });
      await page.waitForURL(/\/settings/, { timeout: 15_000 });

      const navigation = page.locator('[aria-label="Settings navigation"]');
      await expect(navigation, `${panel.route} should keep the shared settings navigation`).toBeVisible();

      const searchInput = page.getByPlaceholder('Search settings...');
      await expect(searchInput, `${panel.route} should keep the shared settings search`).toBeVisible();

      const pageHeading = page.getByRole('heading', { level: 1, name: panel.title });
      await expect(pageHeading, `${panel.route} should render the canonical page-shell heading`).toBeVisible();
      await expect(
        page.getByText(panel.description, { exact: true }).first(),
        `${panel.route} should render the canonical page-shell description`,
      ).toBeVisible();

      await expect(
        page.locator('h1'),
        `${panel.route} should not introduce duplicate page-level headings`,
      ).toHaveCount(1);

      await expect(
        page.locator('main').locator('section, [data-slot="card"], .border-border').first(),
        `${panel.route} should render framed settings content inside the shared shell`,
      ).toBeVisible();
    });
  }
});
