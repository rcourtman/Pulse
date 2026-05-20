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
    tabPaths: ['/truenas/overview', '/truenas/storage', '/truenas/protection'],
    populatedTabPaths: ['/truenas/overview', '/truenas/storage', '/truenas/protection'],
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

  test('docker page renders as a single unified surface without sub-tabs', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop shell smoke');

    await stubEmptyResources(page);
    await page.goto('/docker/overview', { waitUntil: 'domcontentloaded' });

    const pageRoot = page.getByTestId('docker-page');
    await expect(pageRoot).toBeVisible({ timeout: 30_000 });

    await expect(page.getByRole('navigation', { name: 'Docker sections' })).toHaveCount(0);
  });

  test('every platform sub-tab exposes v5-style operator controls', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop chrome audit');

    // Every populated sub-tab — embedded canonical Workloads/Storage AND
    // UnifiedResourceTable-backed infra views — must render an operator
    // search input under platform-page chrome. The shared
    // PlatformResourceTable wrapper provides the toolbar for the
    // UnifiedResourceTable-backed tabs; the embedded surfaces use their
    // own canonical FilterBar via `showFilterToolbar`.
    const cases: ReadonlyArray<{ path: string; testId: string }> = [
      { path: '/docker/overview', testId: 'docker-page' },
      { path: '/kubernetes/overview', testId: 'kubernetes-page' },
      { path: '/kubernetes/nodes', testId: 'kubernetes-page' },
      { path: '/kubernetes/pods', testId: 'kubernetes-page' },
      { path: '/kubernetes/deployments', testId: 'kubernetes-page' },
      { path: '/truenas/overview', testId: 'truenas-page' },
      { path: '/truenas/storage', testId: 'truenas-page' },
      { path: '/truenas/protection', testId: 'truenas-page' },
      { path: '/vmware/overview', testId: 'vmware-page' },
      { path: '/vmware/vms', testId: 'vmware-page' },
      { path: '/vmware/storage', testId: 'vmware-page' },
    ];

    for (const c of cases) {
      await page.goto(c.path, { waitUntil: 'domcontentloaded' });
      const pageRoot = page.getByTestId(c.testId);
      await expect(pageRoot).toBeVisible({ timeout: 30_000 });

      await expect(
        pageRoot
          .locator(
            'input[type="search"], input[placeholder*="Search" i], input[placeholder*="filter" i]',
          )
          .first(),
      ).toBeVisible({ timeout: 30_000 });
    }
  });
});
