import { expect, test } from '@playwright/test';
import { ensureAuthenticated } from './helpers';

const FREE_TRIAL_ELIGIBLE_ENTITLEMENTS = {
  capabilities: [],
  limits: [],
  subscription_state: 'expired',
  upgrade_reasons: [],
  tier: 'free',
  trial_eligible: true,
};

test.describe.serial('Self-hosted trial rate-limit UI', () => {
  test('shows Retry-After guidance on the Pulse Pro trial CTA', async ({ page }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith('mobile-'),
      'Desktop-only Pulse Pro billing coverage',
    );

    await ensureAuthenticated(page);

    await page.route('**/api/license/entitlements', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(FREE_TRIAL_ELIGIBLE_ENTITLEMENTS),
      });
    });

    await page.route('**/api/license/trial/start', async (route) => {
      await route.fulfill({
        status: 429,
        headers: {
          'Content-Type': 'application/json',
          'Retry-After': '120',
        },
        body: JSON.stringify({
          code: 'trial_rate_limited',
          error: 'Trial start rate limit exceeded',
          details: {
            retry_after_seconds: '45',
          },
        }),
      });
    });

    await page.goto('/settings/system/billing');
    await expect(page.getByRole('heading', { name: 'Pulse Pro' }).first()).toBeVisible();

    const startTrialButton = page.getByRole('button', { name: /start 14-day pro trial/i });
    await expect(startTrialButton).toBeVisible();
    await startTrialButton.click();

    await expect(page.getByText('Try again in about 2 minutes')).toBeVisible();
    await expect(page.getByText('Try again in about a minute')).toHaveCount(0);
  });
});
