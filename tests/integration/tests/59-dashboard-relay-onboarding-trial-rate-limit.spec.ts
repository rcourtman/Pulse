import { expect, test, type Page } from '@playwright/test';
import { ensureAuthenticated, getMockMode, setMockMode } from './helpers';

const FREE_RUNTIME_CAPABILITIES = {
  capabilities: ['update_alerts', 'sso', 'ai_patrol'],
  limits: [],
  hosted_mode: false,
  max_history_days: 7,
};

async function waitForDashboardReady(page: Page) {
  await expect(page).toHaveURL(/\/dashboard(?:\?.*)?$/);
  await expect(page.getByTestId('dashboard-page')).toBeVisible();
  await page.waitForFunction(
    () => !document.querySelector('[data-testid="dashboard-loading"]'),
    undefined,
    { timeout: 30_000 },
  );
}

test.describe.serial('Dashboard relay onboarding trial rate limit', () => {
  test('shows Retry-After guidance on the live dashboard relay onboarding CTA', async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith('mobile-'),
      'Desktop-only dashboard relay onboarding coverage',
    );

    await page.route('**/api/license/runtime-capabilities', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(FREE_RUNTIME_CAPABILITIES),
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

    await ensureAuthenticated(page);

    let initialMockMode: { enabled: boolean } | null = null;
    try {
      initialMockMode = await getMockMode(page);
      if (!initialMockMode.enabled) {
        await setMockMode(page, true);
      }
    } catch (error) {
      console.warn(`[relay-onboarding] unable to read/set mock mode: ${String(error)}`);
    }

    try {
      await page.goto('/dashboard');
      await waitForDashboardReady(page);

      await expect(page.getByRole('heading', { name: 'Pair Your Mobile Device' })).toBeVisible();

      const startTrialButton = page.getByRole('button', { name: /or start a pro trial/i });
      await expect(startTrialButton).toBeVisible();
      await startTrialButton.click();

      await expect(page.getByText('Try again in about 2 minutes')).toBeVisible();
      await expect(page.getByText('Try again in about a minute')).toHaveCount(0);
    } finally {
      if (initialMockMode && !initialMockMode.enabled) {
        try {
          await setMockMode(page, false);
        } catch (error) {
          console.warn(
            `[relay-onboarding] unable to restore mock mode, continuing: ${String(error)}`,
          );
        }
      }
    }
  });
});
