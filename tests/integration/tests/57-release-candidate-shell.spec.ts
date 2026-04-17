import { expect, test } from '@playwright/test';
import { waitForPulseReady } from './helpers';

const RC_VERSION_INFO = {
  version: '6.0.0-rc.2',
  build: '',
  runtime: 'unknown',
  channel: 'rc',
  isDocker: false,
  isSourceBuild: true,
  isDevelopment: false,
  deploymentType: 'source',
};

test.describe('Release candidate shell', () => {
  test('keeps RC builds free of the public release-candidate banner', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop runtime proof');

    await waitForPulseReady(page);

    await page.addInitScript(() => {
      localStorage.setItem('pulse_whats_new_v2_shown', 'true');
    });

    await page.route('**/api/security/status', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          hasAuthentication: true,
          hideLocalLogin: false,
          hasProxyAuth: false,
          proxyAuthUsername: '',
          proxyAuthLogoutURL: '',
          publicAccess: false,
          requiresAuth: true,
          ssoEnabled: false,
          ssoProviders: [],
          ssoSessionUsername: '',
          sessionCapabilities: {
            assistantEnabled: true,
            demoMode: false,
          },
          presentationPolicy: {
            demoMode: false,
            readOnly: false,
            hideCommercial: false,
            hideUpgrade: false,
          },
        }),
      }),
    );

    await page.route('**/api/state', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({}),
      }),
    );

    await page.route('**/api/version', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(RC_VERSION_INFO),
      }),
    );

    await page.goto('/dashboard', { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/\/dashboard/, { timeout: 15_000 });

    await expect(
      page.getByRole('status').filter({ hasText: 'public v6 release candidate' }),
    ).toHaveCount(0);
    await expect(page.getByText('Preview', { exact: true })).toBeVisible();
  });
});
