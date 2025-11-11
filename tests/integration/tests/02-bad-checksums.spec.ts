/**
 * Bad Checksums Test: Server rejects update, UI shows error ONCE (not twice)
 *
 * Tests that checksum validation failures are handled correctly and
 * the error modal appears exactly once.
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
  assertUserFriendlyError,
  dismissModal,
  countVisibleModals,
} from './helpers';

test.describe('Update Flow - Bad Checksums', () => {
  test.use({
    // This test requires the mock server to return bad checksums
    // In real implementation, this would be set via env var in docker-compose
  });

  test('should display error when checksum validation fails', async ({ page }) => {
    // Note: This test assumes MOCK_CHECKSUM_ERROR=true is set in environment
    // In practice, you would restart the container with this flag

    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Should show verifying stage
    await expect(progressModal.locator('text=/verifying/i')).toBeVisible({ timeout: 15000 });

    // Should show error after checksum fails
    const errorText = await waitForErrorInModal(page, progressModal);
    const errorContent = await errorText.textContent();

    // Error should mention checksum or validation
    expect(errorContent).toMatch(/checksum|verification|invalid|mismatch/i);
  });

  test('should show error modal EXACTLY ONCE (not twice)', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    await waitForProgressModal(page);

    // Wait for error to appear
    await page.waitForSelector('text=/error|failed/i', { timeout: 30000 });

    // Count visible modals
    const modalCount = await countVisibleModals(page);
    expect(modalCount).toBe(1);

    // Wait a bit to ensure no duplicate appears
    await page.waitForTimeout(2000);
    const modalCountAfter = await countVisibleModals(page);
    expect(modalCountAfter).toBe(1);

    // Wait even longer
    await page.waitForTimeout(3000);
    const modalCountFinal = await countVisibleModals(page);
    expect(modalCountFinal).toBe(1);
  });

  test('should display user-friendly error message', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Wait for error
    const errorText = await waitForErrorInModal(page, progressModal);
    const errorContent = await errorText.textContent() || '';

    // Should be user-friendly (not raw API error)
    await assertUserFriendlyError(errorContent);
  });

  test('should allow dismissing error modal', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Wait for error
    await waitForErrorInModal(page, progressModal);

    // Should be able to dismiss modal
    await dismissModal(page);

    // Modal should disappear
    await expect(progressModal).not.toBeVisible({ timeout: 5000 });
  });

  test('should NOT show raw API error response', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Wait for error
    const errorText = await waitForErrorInModal(page, progressModal);
    const errorContent = await errorText.textContent() || '';

    // Should NOT contain raw API responses
    expect(errorContent).not.toMatch(/500 Internal Server Error/i);
    expect(errorContent).not.toMatch(/\{"error":|"message":/i); // No raw JSON
    expect(errorContent).not.toMatch(/stack trace/i);
    expect(errorContent).not.toMatch(/at Object\./i);
    expect(errorContent).not.toMatch(/\/api\/updates\//i); // No API paths
  });

  test('should NOT allow retry with same bad checksum', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Wait for error
    await waitForErrorInModal(page, progressModal);

    // Should NOT show a "Retry" button (would fail again with same checksum)
    const retryButton = progressModal.locator('button').filter({ hasText: /retry/i });
    await expect(retryButton).not.toBeVisible();
  });

  test('should maintain single modal even after multiple state changes', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Track modal count through different stages
    const modalCounts: number[] = [];

    // Initial count
    modalCounts.push(await countVisibleModals(page));

    // Wait for verifying stage
    await page.waitForSelector('text=/verifying/i', { timeout: 15000 }).catch(() => {});
    modalCounts.push(await countVisibleModals(page));

    // Wait for error
    await page.waitForSelector('text=/error|failed/i', { timeout: 30000 }).catch(() => {});
    modalCounts.push(await countVisibleModals(page));

    // Wait a bit more
    await page.waitForTimeout(2000);
    modalCounts.push(await countVisibleModals(page));

    // All counts should be 1
    for (const count of modalCounts) {
      expect(count).toBe(1);
    }
  });

  test('should show specific checksum error details', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Wait for error
    const errorText = await waitForErrorInModal(page, progressModal);
    const errorContent = await errorText.textContent() || '';

    // Should mention what went wrong specifically
    const hasRelevantKeyword =
      /checksum/i.test(errorContent) ||
      /verification/i.test(errorContent) ||
      /integrity/i.test(errorContent) ||
      /download.*corrupt/i.test(errorContent) ||
      /mismatch/i.test(errorContent);

    expect(hasRelevantKeyword).toBe(true);
  });
});
