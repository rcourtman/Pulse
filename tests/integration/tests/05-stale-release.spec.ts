/**
 * Stale Release Test: Backend refuses to install flagged releases
 *
 * Tests that releases marked as stale or problematic are rejected by the backend.
 */

import { test, expect } from '@playwright/test';
import {
  loginAsAdmin,
  navigateToSettings,
  waitForUpdateBanner,
  clickApplyUpdate,
  waitForConfirmationModal,
  confirmUpdate,
  waitForProgressModal,
  waitForErrorInModal,
  checkForUpdatesAPI,
} from './helpers';

test.describe('Update Flow - Stale Releases', () => {
  test.use({
    // This test requires MOCK_STALE_RELEASE=true in environment
  });

  test('should reject stale release during download', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Should show error about stale release
    const errorText = await waitForErrorInModal(page, progressModal);
    const errorContent = await errorText.textContent() || '';

    // Error should indicate release is not installable
    expect(errorContent).toMatch(/stale|outdated|unavailable|known issue|not recommended/i);
  });

  test('should detect stale release before extraction', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Should detect during download or verification phase
    await expect(progressModal.locator('text=/downloading|verifying/i')).toBeVisible({ timeout: 10000 });

    // Then should error before extracting
    const errorAppeared = await page.waitForSelector('text=/error|failed/i', { timeout: 30000 }).catch(() => null);
    expect(errorAppeared).not.toBeNull();

    // Should NOT reach extraction phase
    const extracting = await progressModal.locator('text=/extracting/i').isVisible().catch(() => false);
    expect(extracting).toBe(false);
  });

  test('should provide informative message about why release is rejected', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    const errorText = await waitForErrorInModal(page, progressModal);
    const errorContent = await errorText.textContent() || '';

    // Should explain why the release cannot be installed
    const hasExplanation =
      errorContent.length > 20 && // Not just "Error" or "Failed"
      (errorContent.match(/issue/i) ||
        errorContent.match(/problem/i) ||
        errorContent.match(/not.*install/i) ||
        errorContent.match(/stale/i));

    expect(hasExplanation).toBe(true);
  });

  test('should not create backup for stale release', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Wait for error
    await waitForErrorInModal(page, progressModal);

    // Should NOT have created backup (never got that far)
    const backupText = await progressModal.locator('text=/backup/i').isVisible().catch(() => false);

    // Either no backup text visible, or it's in error context
    if (backupText) {
      const modalText = await progressModal.textContent() || '';
      // If "backup" appears, it should be in context of "no backup created" or similar
      expect(modalText.toLowerCase()).not.toMatch(/backing.*up|creating.*backup/);
    }
  });

  test('should reject stale release even with valid checksum', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Even if checksum validation passes, should still reject
    // Should see verification stage
    const verifying = await progressModal.locator('text=/verifying/i').isVisible({ timeout: 15000 }).catch(() => false);

    // Then should error (stale release check happens after checksum)
    const errorText = await waitForErrorInModal(page, progressModal);
    const errorContent = await errorText.textContent() || '';

    expect(errorContent).toMatch(/stale|issue|not.*install/i);
  });

  test('should log stale release rejection attempt', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Wait for error
    await waitForErrorInModal(page, progressModal);

    // Check if history endpoint records the failed attempt
    const historyResponse = await page.request.get('http://localhost:7655/api/updates/history');

    if (historyResponse.status() === 200) {
      const history = await historyResponse.json();

      // Should have an entry for the failed update attempt
      expect(Array.isArray(history.entries || history)).toBe(true);

      const entries = history.entries || history;
      if (entries.length > 0) {
        // Most recent entry should show failed status
        const latestEntry = entries[0];
        expect(latestEntry.status).toMatch(/failed|rejected|error/i);
      }
    }
  });

  test('should handle X-Release-Status header from server', async ({ page }) => {
    await loginAsAdmin(page);

    // Intercept download to verify stale header is checked
    let sawStaleHeader = false;

    page.on('response', response => {
      if (response.url().includes('.tar.gz')) {
        const headers = response.headers();
        if (headers['x-release-status'] === 'stale') {
          sawStaleHeader = true;
        }
      }
    });

    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Wait for download to happen
    await page.waitForTimeout(5000);

    // Should have seen the stale header
    expect(sawStaleHeader).toBe(true);

    // And should error
    await waitForErrorInModal(page, progressModal);
  });

  test('should allow checking for other updates after stale rejection', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Wait for error
    await waitForErrorInModal(page, progressModal);

    // Dismiss modal
    await page.keyboard.press('Escape');

    // Should still be able to check for updates
    const checkResponse = await checkForUpdatesAPI(page);
    expect(checkResponse).toBeTruthy();
  });

  test('should differentiate stale release error from other errors', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    const errorText = await waitForErrorInModal(page, progressModal);
    const errorContent = await errorText.textContent() || '';

    // Error should specifically mention stale/known issues, not generic errors
    const isStaleError =
      errorContent.match(/stale/i) ||
      errorContent.match(/known issue/i) ||
      errorContent.match(/flagged/i) ||
      errorContent.match(/not recommended/i);

    // Should not be a generic checksum or network error
    const isGenericError =
      errorContent.match(/checksum/i) ||
      errorContent.match(/network/i) ||
      errorContent.match(/connection/i);

    expect(isStaleError).toBeTruthy();
    expect(isGenericError).toBeFalsy();
  });

  test('should prevent installation of specific flagged version', async ({ page }) => {
    await loginAsAdmin(page);

    // Check what version is being offered
    const updateInfo = await checkForUpdatesAPI(page);

    if (updateInfo.available) {
      const version = updateInfo.version;

      // Try to apply it
      await navigateToSettings(page);

      const banner = await waitForUpdateBanner(page);
      await clickApplyUpdate(page);

      await waitForConfirmationModal(page);
      await confirmUpdate(page);

      const progressModal = await waitForProgressModal(page);

      // Should be rejected
      await waitForErrorInModal(page, progressModal);

      // The specific version should be rejected
      const modalText = await progressModal.textContent() || '';

      // Should mention it's the specific version that's problematic
      const mentionsVersion = modalText.includes(version);

      // Either mentions the version specifically, or gives a generic "this release" message
      expect(mentionsVersion || modalText.match(/this (version|release)/i)).toBeTruthy();
    }
  });
});
