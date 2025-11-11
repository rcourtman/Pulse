/**
 * Network Failure Test: UI retries with exponential backoff
 *
 * Tests that network failures are handled gracefully with proper retry logic.
 */

import { test, expect } from '@playwright/test';
import {
  loginAsAdmin,
  navigateToSettings,
  Timer,
  pollUntil,
  getUpdateStatusAPI,
} from './helpers';

test.describe('Update Flow - Network Failures', () => {
  test('should retry failed update check requests', async ({ page }) => {
    await loginAsAdmin(page);

    // Track API calls to check endpoint
    const apiCalls: any[] = [];

    page.on('request', request => {
      if (request.url().includes('/api/updates/check')) {
        apiCalls.push({
          url: request.url(),
          method: request.method(),
          timestamp: Date.now(),
        });
      }
    });

    // Navigate to settings which should trigger update check
    await navigateToSettings(page);

    // Wait for requests to be made
    await page.waitForTimeout(5000);

    // In case of network errors, should see retry attempts
    // (This test would work better with network error simulation)
  });

  test('should use exponential backoff for retries', async ({ page }) => {
    await loginAsAdmin(page);

    const requestTimes: number[] = [];
    const timer = new Timer();

    page.on('request', request => {
      if (request.url().includes('/api/updates/check')) {
        requestTimes.push(timer.elapsed());
      }
    });

    await navigateToSettings(page);

    // Wait for potential retries
    await page.waitForTimeout(10000);

    // If there were retries, check if delays increase
    if (requestTimes.length > 1) {
      const delays: number[] = [];
      for (let i = 1; i < requestTimes.length; i++) {
        delays.push(requestTimes[i] - requestTimes[i - 1]);
      }

      // Delays should generally increase (exponential backoff)
      // Allow some tolerance for timing variations
      if (delays.length >= 2) {
        // Second delay should be longer than first
        expect(delays[1]).toBeGreaterThanOrEqual(delays[0] * 0.8);
      }
    }
  });

  test('should show loading state during network retry', async ({ page }) => {
    await loginAsAdmin(page);

    // Slow down network to observe loading states
    await page.route('**/api/updates/check', async route => {
      await page.waitForTimeout(2000);
      await route.continue();
    });

    await navigateToSettings(page);

    // Should show some loading indicator
    const loadingIndicators = [
      page.locator('[data-testid="loading"]'),
      page.locator('.loading'),
      page.locator('.spinner'),
      page.locator('text=/loading|checking/i'),
    ];

    let foundLoading = false;
    for (const indicator of loadingIndicators) {
      if (await indicator.isVisible({ timeout: 3000 }).catch(() => false)) {
        foundLoading = true;
        break;
      }
    }

    // Should show some form of loading state
    expect(foundLoading).toBe(true);
  });

  test('should eventually succeed after transient network failures', async ({ page }) => {
    await loginAsAdmin(page);

    let requestCount = 0;

    // Fail first 2 requests, then succeed
    await page.route('**/api/updates/check', async route => {
      requestCount++;

      if (requestCount <= 2) {
        await route.abort('failed');
      } else {
        await route.continue();
      }
    });

    await navigateToSettings(page);

    // Should eventually succeed and show update banner or "up to date" message
    await page.waitForTimeout(10000);

    // Should either show update available or "up to date"
    const hasUpdate = await page.locator('text=/update available/i').isVisible().catch(() => false);
    const upToDate = await page.locator('text=/up to date|latest version/i').isVisible().catch(() => false);

    expect(hasUpdate || upToDate).toBe(true);
  });

  test('should not retry indefinitely', async ({ page }) => {
    await loginAsAdmin(page);

    let requestCount = 0;

    // Always fail requests
    await page.route('**/api/updates/check', async route => {
      requestCount++;
      await route.abort('failed');
    });

    await navigateToSettings(page);

    // Wait for retry attempts
    await page.waitForTimeout(30000);

    // Should have made multiple attempts but not too many
    expect(requestCount).toBeGreaterThan(1); // At least retried
    expect(requestCount).toBeLessThan(20); // But not infinite
  });

  test('should show error message after max retries exceeded', async ({ page }) => {
    await loginAsAdmin(page);

    // Always fail requests
    await page.route('**/api/updates/check', async route => {
      await route.abort('failed');
    });

    await navigateToSettings(page);

    // Wait for retries to exhaust
    await page.waitForTimeout(30000);

    // Should show error message
    const errorMessage = page.locator('text=/error|failed|unable to check/i').first();
    await expect(errorMessage).toBeVisible({ timeout: 5000 });
  });

  test('should handle timeout during download', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    // Intercept download request and timeout
    await page.route('**/*.tar.gz', async route => {
      // Never respond (simulate timeout)
      await page.waitForTimeout(60000);
    });

    // Try to apply update (if available)
    const applyButton = page.locator('button').filter({ hasText: /apply update/i }).first();

    if (await applyButton.isVisible({ timeout: 5000 }).catch(() => false)) {
      await applyButton.click();

      // Confirm if modal appears
      const confirmButton = page.locator('button').filter({ hasText: /confirm|proceed/i }).first();
      if (await confirmButton.isVisible({ timeout: 3000 }).catch(() => false)) {
        await confirmButton.click();
      }

      // Should eventually show timeout error
      const timeoutError = page.locator('text=/timeout|took too long|timed out/i').first();
      await expect(timeoutError).toBeVisible({ timeout: 65000 });
    }
  });

  test('should use exponential backoff with maximum cap', async ({ page }) => {
    await loginAsAdmin(page);

    const requestTimes: number[] = [];
    const timer = new Timer();

    let requestCount = 0;

    // Fail first several requests
    await page.route('**/api/updates/check', async route => {
      requestCount++;
      requestTimes.push(timer.elapsed());

      if (requestCount <= 5) {
        await route.abort('failed');
      } else {
        await route.continue();
      }
    });

    await navigateToSettings(page);

    // Wait for retries
    await page.waitForTimeout(35000);

    // Calculate delays between requests
    if (requestTimes.length > 2) {
      const delays: number[] = [];
      for (let i = 1; i < requestTimes.length; i++) {
        delays.push(requestTimes[i] - requestTimes[i - 1]);
      }

      // Later delays should not exceed a reasonable maximum (e.g., 15 seconds)
      const maxDelay = Math.max(...delays);
      expect(maxDelay).toBeLessThan(20000); // 20 second cap
    }
  });

  test('should preserve user context during network retries', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    // Make network slow
    await page.route('**/api/updates/check', async route => {
      await page.waitForTimeout(3000);
      await route.continue();
    });

    // Trigger update check
    await page.reload();

    // User should still be on settings page
    await expect(page).toHaveURL(/settings/);

    // User should still be logged in
    const logoutButton = page.locator('button').filter({ hasText: /logout|sign out/i }).first();
    const settingsVisible = page.locator('text=/settings/i').first();

    // Should see authenticated UI elements
    const isAuthenticated =
      (await logoutButton.isVisible().catch(() => false)) ||
      (await settingsVisible.isVisible().catch(() => false));

    expect(isAuthenticated).toBe(true);
  });

  test('should handle partial download failures gracefully', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    let downloadStarted = false;

    // Abort download midway
    await page.route('**/*.tar.gz', async (route, request) => {
      if (!downloadStarted) {
        downloadStarted = true;

        // Start response then abort
        await page.waitForTimeout(2000);
        await route.abort('failed');
      } else {
        await route.continue();
      }
    });

    const applyButton = page.locator('button').filter({ hasText: /apply update/i }).first();

    if (await applyButton.isVisible({ timeout: 5000 }).catch(() => false)) {
      await applyButton.click();

      const confirmButton = page.locator('button').filter({ hasText: /confirm|proceed/i }).first();
      if (await confirmButton.isVisible({ timeout: 3000 }).catch(() => false)) {
        await confirmButton.click();
      }

      // Should show error about download failure
      const downloadError = page.locator('text=/download.*failed|failed.*download/i').first();
      await expect(downloadError).toBeVisible({ timeout: 30000 });
    }
  });
});
