import { expect, test, type Page } from '@playwright/test';
import { ensureAuthenticated, getMockMode, setMockMode } from './helpers';

const waitForDashboardEstateSummary = async (page: Page) => {
  await expect(page).toHaveURL(/\/dashboard(?:\?.*)?$/);
  await expect(page.getByTestId('dashboard-page')).toBeVisible();
  await expect(page.getByTestId('dashboard-estate-summary')).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Connected infrastructure' })).toBeVisible();
};

test('dashboard first viewport preserves connected infrastructure orientation', async ({
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
    console.warn(`[dashboard-estate] unable to read/set mock mode, continuing: ${String(error)}`);
  }

  try {
    await page.goto('/dashboard');
    await waitForDashboardEstateSummary(page);

    const estateSummary = page.getByTestId('dashboard-estate-summary');
    await expect(estateSummary.getByRole('link', { name: 'View infrastructure' })).toBeVisible();
    await expect(estateSummary.getByText(/systems? reporting|systems? need/)).toBeVisible();
    await expect(estateSummary.getByText('Resource summary fallback')).toHaveCount(0);

    const estateBox = await estateSummary.boundingBox();
    const kpiBox = await page.getByTestId('dashboard-kpi-infrastructure').boundingBox();
    expect(estateBox?.y ?? 0).toBeLessThan(kpiBox?.y ?? Number.POSITIVE_INFINITY);

    const hasHorizontalOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
    );
    expect(hasHorizontalOverflow).toBe(false);
  } finally {
    if (initialMockMode && !initialMockMode.enabled) {
      try {
        await setMockMode(page, false);
      } catch (error) {
        console.warn(`[dashboard-estate] unable to restore mock mode, continuing: ${String(error)}`);
      }
    }
  }
});
