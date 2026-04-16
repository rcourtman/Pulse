import { expect, test } from '@playwright/test';
import { ensureAuthenticated } from './helpers';

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

test.describe('Release candidate banner', () => {
  test('renders generic RC copy with version-specific release links', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop runtime proof');

    await page.route('**/api/version', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(RC_VERSION_INFO),
      }),
    );

    await ensureAuthenticated(page);
    await page.goto('/dashboard', { waitUntil: 'domcontentloaded' });

    const banner = page.getByRole('status').filter({
      hasText: 'Pulse 6.0.0-rc.2 is a public v6 release candidate.',
    });
    await expect(banner).toContainText('Pulse 6.0.0-rc.2 is a public v6 release candidate.');
    await expect(page.getByRole('link', { name: 'View release notes' })).toHaveAttribute(
      'href',
      'https://github.com/rcourtman/Pulse/releases/tag/v6.0.0-rc.2',
    );
    await expect(page.getByRole('link', { name: 'Send feedback' })).toHaveAttribute(
      'href',
      'https://github.com/rcourtman/Pulse/issues/new?template=v6_rc_feedback.yml',
    );
  });
});
