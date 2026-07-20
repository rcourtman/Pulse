import { expect, test } from '@playwright/test';
import { ensureAuthenticated } from './helpers';

test.describe('Docker workload route ownership', () => {
  test('retires legacy Docker workload deep links into the runtime overview', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop route ownership contract');

    await ensureAuthenticated(page);

    await page.goto(
      '/docker/workloads?type=app-container&platform=docker&agent=docker-host-1',
      { waitUntil: 'domcontentloaded' },
    );

    await expect(page).toHaveURL(/\/docker\/overview$/);
    await expect(
      page.getByRole('navigation', { name: 'Container runtime sections' }),
    ).toBeVisible();
    await expect(page.locator('#workloads-type')).toHaveCount(0);
    await expect(page.locator('#workloads-platform-filter')).toHaveCount(0);
    await expect(page.locator('#workloads-node-filter')).toHaveCount(0);
  });
});
