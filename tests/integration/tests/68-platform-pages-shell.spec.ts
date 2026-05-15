import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect, type Page } from '@playwright/test';
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
        `platform-pages-shell-${workerInfo.project.name}.json`,
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

type PlatformPageCase = {
  id: string;
  rootPath: string;
  testId: string;
  ariaLabel: string;
  tabPaths: readonly string[];
  // Sub-tab routes that should render at least one canonical row under the
  // mock backend. We assert presence of a `<table>` to confirm the embedded
  // canonical surface mounted and reached a non-empty state. Empty arrays
  // are allowed for platform pages whose subtab is service-only or stays
  // empty under the default mock fixtures.
  populatedTabPaths?: readonly string[];
};

const PLATFORM_PAGES: readonly PlatformPageCase[] = [
  {
    id: 'docker',
    rootPath: '/docker',
    testId: 'docker-page',
    ariaLabel: 'Docker sections',
    tabPaths: ['/docker/overview', '/docker/containers'],
    populatedTabPaths: ['/docker/overview', '/docker/containers'],
  },
  {
    id: 'kubernetes',
    rootPath: '/kubernetes',
    testId: 'kubernetes-page',
    ariaLabel: 'Kubernetes sections',
    tabPaths: [
      '/kubernetes/overview',
      '/kubernetes/nodes',
      '/kubernetes/pods',
      '/kubernetes/deployments',
    ],
    populatedTabPaths: [
      '/kubernetes/overview',
      '/kubernetes/nodes',
      '/kubernetes/pods',
      '/kubernetes/deployments',
    ],
  },
  {
    id: 'truenas',
    rootPath: '/truenas',
    testId: 'truenas-page',
    ariaLabel: 'TrueNAS sections',
    tabPaths: ['/truenas/storage', '/truenas/apps'],
    populatedTabPaths: ['/truenas/storage', '/truenas/apps'],
  },
  {
    id: 'vmware',
    rootPath: '/vmware',
    testId: 'vmware-page',
    ariaLabel: 'VMware sections',
    tabPaths: ['/vmware/overview', '/vmware/vms', '/vmware/storage'],
    populatedTabPaths: ['/vmware/overview', '/vmware/vms', '/vmware/storage'],
  },
];

const stubEmptyResources = async (page: Page) => {
  await page.route('**/api/resources**', async (route) => {
    const requestUrl = new URL(route.request().url());
    if (requestUrl.pathname === '/api/resources') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], links: { next: null } }),
      });
      return;
    }
    await route.continue();
  });
};

test.describe('Platform pages shell', () => {
  test.setTimeout(180_000);

  for (const platform of PLATFORM_PAGES) {
    test(`${platform.id} no-data state renders sub-tab chrome with empty resources`, async ({
      page,
    }, testInfo) => {
      test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop shell smoke');

      await stubEmptyResources(page);
      await page.goto(platform.rootPath, { waitUntil: 'domcontentloaded' });

      const pageRoot = page.getByTestId(platform.testId);
      await expect(pageRoot).toBeVisible({ timeout: 30_000 });

      const sectionNav = page.getByRole('navigation', { name: platform.ariaLabel });
      await expect(sectionNav).toBeVisible();

      for (const tabPath of platform.tabPaths) {
        await expect(sectionNav.locator(`a[href="${tabPath}"]`)).toBeVisible();
      }
    });

    test(`${platform.id} populated state renders canonical rows under mock mode`, async ({
      page,
    }, testInfo) => {
      test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop shell smoke');

      const populated = platform.populatedTabPaths ?? [];
      if (populated.length === 0) {
        test.skip(
          true,
          `No mock-populated sub-tabs declared for ${platform.id}; default mock fixtures do not exercise this platform's tables yet.`,
        );
      }

      for (const tabPath of populated) {
        await page.goto(tabPath, { waitUntil: 'domcontentloaded' });

        const pageRoot = page.getByTestId(platform.testId);
        await expect(pageRoot).toBeVisible({ timeout: 30_000 });

        // Each populated sub-tab must render at least one canonical table.
        await expect(pageRoot.locator('table tbody tr').first()).toBeVisible({
          timeout: 30_000,
        });
      }
    });
  }

  test('platform sub-tabs that embed Workloads/Storage surfaces expose v5-style operator controls', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop chrome audit');

    // Each entry verifies that the embedded canonical surface renders its
    // operator toolbar (search + status/grouping/view chips) under
    // platform-page chrome — not a stripped table-only view.
    const cases: ReadonlyArray<{ path: string; testId: string }> = [
      { path: '/docker/containers', testId: 'docker-page' },
      { path: '/kubernetes/pods', testId: 'kubernetes-page' },
      { path: '/truenas/storage', testId: 'truenas-page' },
      { path: '/truenas/apps', testId: 'truenas-page' },
      { path: '/vmware/vms', testId: 'vmware-page' },
      { path: '/vmware/storage', testId: 'vmware-page' },
    ];

    for (const c of cases) {
      await page.goto(c.path, { waitUntil: 'domcontentloaded' });
      const pageRoot = page.getByTestId(c.testId);
      await expect(pageRoot).toBeVisible({ timeout: 30_000 });

      // The canonical Workloads/Storage operator toolbars use the shared
      // FilterBar primitive, which renders a search input. If the platform
      // page mistakenly stripped the toolbar, no search input would render
      // inside the page region.
      await expect(
        pageRoot
          .locator('input[type="search"], input[placeholder*="Search" i], input[placeholder*="filter" i]')
          .first(),
      ).toBeVisible({ timeout: 30_000 });
    }
  });
});
