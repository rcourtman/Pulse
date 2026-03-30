import { test, expect } from '@playwright/test';

test.describe('Setup completion platform connections handoff', () => {
  test('preview exposes Platform connections as the API-backed first-run alternative', async ({
    page,
  }) => {
    await page.goto('/preview/setup-complete', { waitUntil: 'domcontentloaded' });

    await expect(page.getByText('What happens next')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Open Infrastructure Install' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Open Platform connections' })).toBeVisible();
    await expect(
      page.getByText(
        'If the first system is API-backed, use Platform connections instead of starting with host install.',
      ),
    ).toBeVisible();

    await page.getByRole('button', { name: 'Open Platform connections' }).click();
    await page.waitForURL(/\/settings\/infrastructure\/platforms$/, { timeout: 15_000 });
    await expect(
      page.getByRole('heading', { name: 'Infrastructure Operations', exact: true }),
    ).toBeVisible();
    await expect(page.getByRole('tab', { name: 'Platform connections' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
  });
});
