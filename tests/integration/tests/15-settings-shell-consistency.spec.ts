import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';
import { createAuthenticatedStorageState, isMultiTenantEnabled } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

// Titles and descriptions mirror the canonical copy in
// frontend-modern/src/components/Settings/settingsHeaderMeta.ts.
const SETTINGS_SHELL_ROUTES = [
  {
    route: '/settings/system-general',
    title: 'General',
    description: 'Manage appearance, layout, and default monitoring cadence.',
  },
  {
    route: '/settings/organization',
    title: 'Organization Overview',
    description: 'Review organization metadata, membership footprint, and ownership.',
    requiresMultiTenant: true,
  },
  {
    route: '/settings/organization/access',
    title: 'Organization Access',
    description: 'Manage organization invitations, member roles, and ownership transfers.',
    requiresMultiTenant: true,
  },
  {
    route: '/settings/organization/billing',
    title: 'Billing & Usage',
    description:
      'Review your organization plan, applicable usage policies, and subscription status for paid access.',
    requiresMultiTenant: true,
  },
  {
    route: '/settings/system-relay',
    title: 'Remote Access',
    description:
      'Check on your systems and get alert push notifications anywhere with the Pulse Mobile app — no port forwarding or VPN required.',
  },
  {
    route: '/settings/security-auth',
    title: 'Authentication',
    description: 'Manage password-based authentication and credential rotation.',
  },
  {
    route: '/settings/system-ai',
    title: 'Provider & Models',
    description:
      'Configure providers, default models, provider health, budget, and usage for Pulse Intelligence.',
  },
  {
    route: '/settings/system-updates',
    title: 'Updates',
    description:
      'Manage Pulse server runtime version checks, update channels, and automatic updates. Agent updates stay under Infrastructure.',
  },
  {
    route: '/settings/system-recovery',
    title: 'Recovery',
    description: 'Manage backup/snapshot polling plus configuration export and import workflows.',
  },
  {
    route: '/settings/pulse-intelligence/billing/plan',
    title: 'Plans & Billing',
    description: 'Plan, license, and Patrol mode for this instance.',
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
    test(`uses the canonical shell on ${panel.route}`, async ({ page, isMobile }) => {
      if ('requiresMultiTenant' in panel && panel.requiresMultiTenant) {
        // Organization panels only render when multi-tenant is enabled
        // (single-tenant installs hide the whole organization group).
        test.skip(!(await isMultiTenantEnabled(page)), 'Multi-tenant feature not enabled in this environment');
      }
      await page.goto(panel.route, { waitUntil: 'domcontentloaded' });
      await page.waitForURL(/\/settings/, { timeout: 15_000 });

      if (isMobile) {
        // Mobile is a two-level workspace: the compact section header opens
        // the full settings index, and selecting the active section returns
        // to its content without leaving the route.
        await page
          .getByRole('main')
          .getByRole('button', { name: 'Settings', exact: true })
          .first()
          .click();
      }

      const navigation = page.locator('[aria-label="Settings navigation"]');
      await expect(navigation, `${panel.route} should keep the shared settings navigation`).toBeVisible();

      const searchInput = page.getByPlaceholder('Search settings...');
      await expect(searchInput, `${panel.route} should keep the shared settings search`).toBeVisible();

      if (isMobile) {
        await expect(
          navigation.getByRole('heading', { level: 1, name: 'Settings' }),
          `${panel.route} should label the mobile settings index`,
        ).toBeVisible();

        const activeSection = navigation.locator('button[aria-current="page"]');
        if ((await activeSection.count()) === 1) {
          await activeSection.click();
        } else {
          // A valid direct route can be omitted from the index by capability
          // or feature visibility. The index must still be dismissible.
          await navigation
            .getByRole('button', { name: 'Close settings navigation', exact: true })
            .click();
        }
        await expect(navigation).toBeHidden();
      }

      const pageHeading = page.getByRole('heading', { level: 1, name: panel.title });
      await expect(pageHeading, `${panel.route} should render the canonical page-shell heading`).toBeVisible();
      if (!isMobile) {
        // Panel descriptions are desktop-only copy (hidden sm:block).
        await expect(
          page.getByText(panel.description, { exact: true }).first(),
          `${panel.route} should render the canonical page-shell description`,
        ).toBeVisible();
      }

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

  test('keeps direct Settings content inside a 390px viewport after desktop resize', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/settings/pulse-intelligence/billing/plan', {
      waitUntil: 'domcontentloaded',
    });

    const content = page.locator('[data-settings-content]');
    await expect(content).toBeVisible();
    await expect(page.getByRole('heading', { level: 1, name: 'Plans & Billing' })).toBeVisible();

    await page.setViewportSize({ width: 390, height: 844 });

    const layout = await content.evaluate((element) => {
      const bounds = element.getBoundingClientRect();
      return {
        animationName: getComputedStyle(element).animationName,
        left: bounds.left,
        right: bounds.right,
        viewportWidth: document.documentElement.clientWidth,
      };
    });

    expect(layout.animationName).toBe('none');
    expect(layout.left).toBeGreaterThanOrEqual(0);
    expect(layout.right).toBeLessThanOrEqual(layout.viewportWidth);
  });
});
