import { expect, test, type Page } from '@playwright/test';
import { ensureAuthenticated, getMockMode, setMockMode } from './helpers';

const waitForDashboardReady = async (page: Page) => {
  await expect(page).toHaveURL(/\/dashboard(?:\?.*)?$/);
  await expect(page.getByTestId('dashboard-page')).toBeVisible();
  await page.waitForFunction(
    () => !document.querySelector('[data-testid="dashboard-loading"]'),
    undefined,
    { timeout: 30_000 },
  );
};

test('dashboard keeps the v6 migration notice visible until the operator dismisses it', async ({
  page,
}) => {
  await ensureAuthenticated(page);

  let initialMockMode: { enabled: boolean } | null = null;
  try {
    initialMockMode = await getMockMode(page);
    if (!initialMockMode.enabled) {
      await setMockMode(page, true);
    }
  } catch (error) {
    console.warn(`[dashboard-migration] unable to read/set mock mode: ${String(error)}`);
  }

  try {
    await page.addInitScript(() => {
      localStorage.setItem('pulse_whats_new_v2_shown', 'true');
      if (!sessionStorage.getItem('pulse-dashboard-migration-notice-test-initialized')) {
        localStorage.removeItem('pulse-dashboard-migration-notice-dismissed');
        sessionStorage.setItem('pulse-dashboard-migration-notice-test-initialized', 'true');
      }
    });

    await page.goto('/dashboard', { waitUntil: 'domcontentloaded' });
    await waitForDashboardReady(page);

    await expect(page.getByText('Looking for Proxmox, Docker, and Hosts?')).toBeVisible();
    await expect(
      page.getByText(
        'Use Infrastructure for Proxmox nodes, Docker hosts, clusters, and other systems. Use Workloads for VMs, containers, pods, and Docker update status.',
      ),
    ).toBeVisible();
    await expect(page.getByRole('link', { name: 'See full migration guide' })).toHaveAttribute(
      'href',
      '/docs/MIGRATION_UNIFIED_NAV.md',
    );

    await page.getByRole('button', { name: 'Dismiss navigation notice' }).click();

    await expect(page.getByText('Looking for Proxmox, Docker, and Hosts?')).toHaveCount(0);
    await expect
      .poll(() => page.evaluate(() => localStorage.getItem('pulse-dashboard-migration-notice-dismissed')))
      .toBe('true');

    await page.reload({ waitUntil: 'domcontentloaded' });
    await waitForDashboardReady(page);

    await expect(page.getByText('Looking for Proxmox, Docker, and Hosts?')).toHaveCount(0);
  } finally {
    if (initialMockMode && !initialMockMode.enabled) {
      try {
        await setMockMode(page, false);
      } catch (error) {
        console.warn(`[dashboard-migration] unable to restore mock mode: ${String(error)}`);
      }
    }
  }
});
