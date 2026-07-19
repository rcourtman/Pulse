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
  populatedRowSelectors?: Partial<Record<string, string>>;
  emptyStateTitle: string;
};

const PLATFORM_PAGES: readonly PlatformPageCase[] = [
  {
    id: 'kubernetes',
    emptyStateTitle: 'No Kubernetes clusters',
    rootPath: '/kubernetes',
    testId: 'kubernetes-page',
    ariaLabel: 'Kubernetes sections',
    tabPaths: [
      '/kubernetes/overview',
      '/kubernetes/nodes',
      '/kubernetes/workloads',
      '/kubernetes/services',
      '/kubernetes/storage',
      '/kubernetes/configuration',
      '/kubernetes/events',
    ],
    populatedTabPaths: [
      '/kubernetes/overview',
      '/kubernetes/nodes',
      '/kubernetes/workloads',
      '/kubernetes/services',
      '/kubernetes/storage',
      '/kubernetes/configuration',
      '/kubernetes/events',
    ],
  },
  {
    id: 'truenas',
    emptyStateTitle: 'No TrueNAS systems',
    rootPath: '/truenas',
    testId: 'truenas-page',
    ariaLabel: 'TrueNAS sections',
    tabPaths: [
      '/truenas/overview',
      '/truenas/storage',
      '/truenas/services',
      '/truenas/apps',
      '/truenas/vms',
      '/truenas/shares',
      '/truenas/protection',
    ],
    populatedTabPaths: [
      '/truenas/overview',
      '/truenas/storage',
      '/truenas/services',
      '/truenas/apps',
      '/truenas/vms',
      '/truenas/shares',
      '/truenas/protection',
    ],
    populatedRowSelectors: {
      '/truenas/overview': '[data-truenas-system-row]',
      '/truenas/storage': '[data-truenas-storage-row]',
      '/truenas/services': '[data-truenas-service-row]',
      '/truenas/apps': '[data-truenas-app-row]',
      '/truenas/vms': '[data-truenas-vm-row]',
      '/truenas/shares': '[data-truenas-share-row]',
      '/truenas/protection': '[data-truenas-protection-row]',
    },
  },
  {
    id: 'vmware',
    emptyStateTitle: 'No vSphere hosts',
    rootPath: '/vmware',
    testId: 'vmware-page',
    ariaLabel: 'VMware sections',
    tabPaths: [
      '/vmware/overview',
      '/vmware/storage',
      '/vmware/networks',
      '/vmware/health',
      '/vmware/activity',
    ],
    populatedTabPaths: [
      '/vmware/overview',
      '/vmware/storage',
      '/vmware/networks',
      '/vmware/health',
      '/vmware/activity',
    ],
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

      // Sub-tabs are inventory-gated (64cbdd6e4 "Gate platform workflow tabs
      // by inventory"). Platforms whose fixtures reach the store over the
      // websocket still render the full sub-tab chrome; a platform with no
      // connected instance (e.g. TrueNAS with no configured connection) shows
      // the empty-state card instead of chrome. Accept either, but require the
      // page to land in one coherent state, not blank.
      const sectionNav = page.getByRole('navigation', { name: platform.ariaLabel });
      const emptyState = pageRoot.getByText(platform.emptyStateTitle, { exact: true });
      await expect(sectionNav.or(emptyState).first()).toBeVisible({ timeout: 30_000 });

      if ((await sectionNav.count()) > 0 && (await sectionNav.isVisible())) {
        for (const tabPath of platform.tabPaths) {
          await expect(sectionNav.locator(`a[href="${tabPath}"]`)).toBeVisible();
        }
      } else {
        await expect(emptyState).toBeVisible();
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
        const rowSelector = platform.populatedRowSelectors?.[tabPath] ?? 'table tbody tr';
        await expect(pageRoot.locator(rowSelector).first()).toBeVisible({
          timeout: 30_000,
        });
      }
    });
  }

  test('docker page renders container runtime workflow tabs', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop shell smoke');

    await stubEmptyResources(page);
    await page.goto('/docker/overview', { waitUntil: 'domcontentloaded' });

    const pageRoot = page.getByTestId('docker-page');
    await expect(pageRoot).toBeVisible({ timeout: 30_000 });

    const sectionNav = page.getByRole('navigation', {
      name: 'Container runtime sections',
    });
    await expect(sectionNav).toBeVisible();
    // Containers collapsed into Overview (1f66a2f93 "Collapse Docker page to
    // single unified surface"); Overview owns runtime hosts plus primary
    // container workloads.
    for (const tabPath of [
      '/docker/overview',
      '/docker/images',
      '/docker/storage',
      '/docker/networks',
      '/docker/swarm',
    ]) {
      await expect(sectionNav.locator(`a[href="${tabPath}"]`)).toBeVisible();
    }
  });

  test('every platform inventory sub-tab exposes canonical operator search', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop chrome audit');

    // Every populated inventory tab — embedded canonical Workloads/Storage AND
    // UnifiedResourceTable-backed infra views — must render an operator
    // search input under platform-page chrome. Overview stacks are summary
    // surfaces and intentionally keep their table toolbars suppressed. The shared
    // PlatformResourceTable wrapper provides the toolbar for the
    // UnifiedResourceTable-backed tabs; the embedded surfaces use their
    // own canonical FilterBar via `showFilterToolbar`.
    const cases: ReadonlyArray<{ path: string; testId: string }> = [
      { path: '/docker/images', testId: 'docker-page' },
      { path: '/docker/storage', testId: 'docker-page' },
      { path: '/docker/networks', testId: 'docker-page' },
      // The default mock inventory intentionally has no Swarm resources.
      // Empty inventory tabs omit search because there is nothing actionable
      // to filter; populated Swarm table toolbars have component coverage.
      { path: '/kubernetes/nodes', testId: 'kubernetes-page' },
      { path: '/kubernetes/workloads', testId: 'kubernetes-page' },
      { path: '/kubernetes/services', testId: 'kubernetes-page' },
      { path: '/kubernetes/storage', testId: 'kubernetes-page' },
      { path: '/kubernetes/configuration', testId: 'kubernetes-page' },
      { path: '/kubernetes/events', testId: 'kubernetes-page' },
      { path: '/truenas/storage', testId: 'truenas-page' },
      { path: '/truenas/services', testId: 'truenas-page' },
      { path: '/truenas/apps', testId: 'truenas-page' },
      { path: '/truenas/vms', testId: 'truenas-page' },
      { path: '/truenas/shares', testId: 'truenas-page' },
      { path: '/truenas/protection', testId: 'truenas-page' },
      { path: '/vmware/storage', testId: 'vmware-page' },
      { path: '/vmware/networks', testId: 'vmware-page' },
      { path: '/vmware/health', testId: 'vmware-page' },
      { path: '/vmware/activity', testId: 'vmware-page' },
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
