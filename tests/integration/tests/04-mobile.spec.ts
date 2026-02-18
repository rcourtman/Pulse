import { test, expect, devices } from '@playwright/test';
import { ensureAuthenticated } from './helpers';

const getViewportWidth = async (page: import('@playwright/test').Page): Promise<number> => {
  const size = page.viewportSize();
  if (size) return size.width;
  return await page.evaluate(() => window.innerWidth);
};

test.describe('Mobile viewport flows', () => {
  test.beforeEach(async ({ page }) => {
    await ensureAuthenticated(page);
  });

  test('bottom nav bar is visible on mobile', async ({ page }) => {
    await page.goto('/');

    const bottomNav = page.locator(
      'nav.md\\:hidden, nav[class*="md:hidden"], .md\\:hidden nav, [class*="md:hidden"] nav',
    );

    const visibleCount = await bottomNav.evaluateAll((els) => {
      const isVisible = (el: Element) => {
        const style = window.getComputedStyle(el as HTMLElement);
        if (style.display === 'none' || style.visibility === 'hidden' || style.opacity === '0') return false;
        const rect = (el as HTMLElement).getBoundingClientRect();
        return rect.width > 0 && rect.height > 0;
      };
      return els.filter(isVisible).length;
    });

    expect(visibleCount, 'Expected a visible mobile bottom nav (md:hidden)').toBeGreaterThan(0);
  });

  test('MobileNavBar has safe-area padding on nav', async ({ page }) => {
    await page.goto('/');

    const nav = page.locator('nav.md\\:hidden, nav[class*="md:hidden"]').first();
    await expect(nav).toBeVisible();

    const paddingBottom = await nav.evaluate((el) => {
      const raw = window.getComputedStyle(el as HTMLElement).paddingBottom || '0px';
      const parsed = Number.parseFloat(raw);
      return Number.isFinite(parsed) ? parsed : 0;
    });

    expect(paddingBottom).toBeGreaterThan(0);
  });

  test('Infrastructure filter bar does not overflow horizontally', async ({ page }) => {
    await page.goto('/proxmox/infrastructure');
    if (!/\/infrastructure(?:\?.*)?$/.test(page.url())) {
      await page.goto('/infrastructure');
    }

    await expect(page.getByTestId('infrastructure-page')).toBeVisible();
    await expect(page.getByPlaceholder('Search resources, IDs, IPs, or tags...')).toBeVisible();

    const filterBar = page
      .getByText('Source', { exact: true })
      .locator('xpath=ancestor::div[contains(@class,"flex-wrap")][1]');
    await expect(filterBar).toBeVisible();

    const { scrollWidth, clientWidth } = await filterBar.evaluate((el) => ({
      scrollWidth: (el as HTMLElement).scrollWidth,
      clientWidth: (el as HTMLElement).clientWidth,
    }));

    expect(scrollWidth).toBeLessThanOrEqual(clientWidth + 1);
  });

  test('Infrastructure table has overflow-x-auto wrapper', async ({ page }) => {
    await page.goto('/proxmox/infrastructure');
    if (!/\/infrastructure(?:\?.*)?$/.test(page.url())) {
      await page.goto('/infrastructure');
    }

    await expect(page.getByTestId('infrastructure-page')).toBeVisible();
    await expect(page.getByPlaceholder('Search resources, IDs, IPs, or tags...')).toBeVisible();

    const tableWrapper = page.locator('div.overflow-x-auto').filter({ has: page.locator('table') }).first();
    const wrapperVisible = await tableWrapper.isVisible().catch(() => false);
    if (!wrapperVisible) {
      test.skip(true, 'No infrastructure table rendered (no resources or table not present)');
    }

    const canScrollHorizontally = await tableWrapper.evaluate((el) => (el as HTMLElement).scrollWidth > (el as HTMLElement).clientWidth + 1);
    expect(canScrollHorizontally).toBeTruthy();
  });

  test('Tapping a resource row opens the detail drawer', async ({ page }) => {
    await page.goto('/proxmox/infrastructure');
    if (!/\/infrastructure(?:\?.*)?$/.test(page.url())) {
      await page.goto('/infrastructure');
    }

    await expect(page.getByTestId('infrastructure-page')).toBeVisible();
    await expect(page.getByPlaceholder('Search resources, IDs, IPs, or tags...')).toBeVisible();

    const row = page.locator('table tbody tr.cursor-pointer').first();
    const rowVisible = await row.isVisible().catch(() => false);
    if (!rowVisible) {
      test.skip(true, 'No resource rows available to expand');
    }

    await row.click();

    const drawerCell = page.locator('td').filter({ has: page.getByRole('button', { name: 'Close' }) }).first();
    await expect(drawerCell).toBeVisible();
    await expect(drawerCell.getByText('Discovery', { exact: true })).toBeVisible();
  });

  test('Dashboard loads without horizontal overflow at mobile viewport', async ({ page }) => {
    await page.goto('/dashboard');
    await expect(page.locator('#root')).toBeVisible();

    const viewportWidth = await getViewportWidth(page);
    const bodyScrollWidth = await page.evaluate(() => document.body.scrollWidth);
    expect(bodyScrollWidth).toBeLessThanOrEqual(viewportWidth + 1);
  });

  test('AI assistant button is visible above nav bar', async ({ page }) => {
    await page.goto('/dashboard');
    await expect(page.locator('#root')).toBeVisible();

    const nav = page.locator('nav.md\\:hidden, nav[class*="md:hidden"]').first();
    await expect(nav).toBeVisible();

    const aiButton = page.getByRole('button', { name: 'Expand Pulse Assistant' });
    await expect(aiButton).toBeVisible();

    const navBox = await nav.boundingBox();
    const aiBox = await aiButton.boundingBox();

    expect(navBox, 'Expected nav bounding box').toBeTruthy();
    expect(aiBox, 'Expected AI button bounding box').toBeTruthy();

    const navTop = (navBox as { y: number }).y;
    const aiBottom = (aiBox as { y: number; height: number }).y + (aiBox as { height: number }).height;
    expect(aiBottom).toBeLessThanOrEqual(navTop + 1);
  });
});

