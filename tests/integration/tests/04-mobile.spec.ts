import { test, expect, devices } from '@playwright/test';
import { dismissWhatsNewModal, ensureAuthenticated } from './helpers';

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
    await dismissWhatsNewModal(page);
    await page.goto('/dashboard');
    await expect(page.locator('#root')).toBeVisible();

    const bottomNav = page.locator(
      'nav.md\\:hidden, nav[class*="md:hidden"], .md\\:hidden nav, [class*="md:hidden"] nav',
    );

    // Wait for the nav to mount before evaluating (evaluateAll does not auto-wait;
    // WebKit can be slower to render SolidJS components than Chromium).
    await bottomNav.first().waitFor({ state: 'attached', timeout: 10000 });

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
    await dismissWhatsNewModal(page);
    await page.goto('/dashboard');
    await expect(page.locator('#root')).toBeVisible();

    const nav = page.locator('nav.md\\:hidden, nav[class*="md:hidden"]').first();
    await expect(nav).toBeVisible();

    // Verify the safe-area CSS class is applied to the nav. The computed padding-bottom
    // value is 0 in headless Chromium (no notch), but the pb-safe class must be present.
    const hasSafeClass = await nav.evaluate((el: HTMLElement) =>
      el.classList.contains('pb-safe'),
    );
    expect(hasSafeClass, 'Expected nav to have pb-safe class for safe-area-inset-bottom').toBeTruthy();
  });

  test('Infrastructure filter bar does not overflow horizontally', async ({ page }) => {
    // Prevent WhatsNew modal from blocking the page.
    await dismissWhatsNewModal(page);
    await page.goto('/infrastructure');

    await expect(page.getByTestId('infrastructure-page')).toBeVisible();
    await expect(page.getByPlaceholder('Search resources, IDs, IPs, or tags...')).toBeVisible();

    // On mobile the full filter controls are hidden behind a toggle; only the
    // search bar + Filters button row should be visible. Check the overall page
    // body does not overflow horizontally.
    const viewportWidth = await getViewportWidth(page);
    const bodyScrollWidth = await page.evaluate(() => document.body.scrollWidth);
    expect(bodyScrollWidth, 'Infrastructure page body must not overflow horizontally').toBeLessThanOrEqual(viewportWidth + 1);
  });

  test('Infrastructure table has overflow-x-auto wrapper', async ({ page }) => {
    await dismissWhatsNewModal(page);
    await page.goto('/infrastructure');

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
    // Prevent WhatsNew modal from intercepting row clicks.
    await dismissWhatsNewModal(page);
    await page.goto('/infrastructure');

    await expect(page.getByTestId('infrastructure-page')).toBeVisible();
    await expect(page.getByPlaceholder('Search resources, IDs, IPs, or tags...')).toBeVisible();

    const row = page.locator('table tbody tr.cursor-pointer').first();
    const rowVisible = await row.isVisible().catch(() => false);
    if (!rowVisible) {
      test.skip(true, 'No resource rows available to expand');
    }

    await row.click();

    // On mobile, the drawer renders as a Portal bottom sheet (not inline in a <td>).
    // Look for the Discovery tab button anywhere on the page.
    const discoveryTab = page.getByText('Discovery', { exact: true }).first();
    await expect(discoveryTab).toBeVisible({ timeout: 10000 });
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

