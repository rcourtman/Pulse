import { expect, test } from '@playwright/test';
import { apiRequest, ensureAuthenticated } from './helpers';

test.describe.serial('Retired self-hosted Pro trial sandbox path', () => {
  test('does not start a hosted Stripe trial from an ordinary self-hosted runtime', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only commercial route coverage');

    await ensureAuthenticated(page);

    const startRes = await apiRequest(page, '/api/license/trial/start', {
      method: 'POST',
    });
    expect(startRes.status(), `retired trial start route must not be exposed: HTTP ${startRes.status()}`).toBe(404);
  });
});
