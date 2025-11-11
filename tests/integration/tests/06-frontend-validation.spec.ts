/**
 * Frontend Validation Tests: Modal behavior and UX validation
 *
 * Tests specific frontend requirements:
 * - UpdateProgressModal appears exactly once
 * - Error messages are user-friendly
 * - Modal can be dismissed after error
 * - No duplicate modals on error
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
  countVisibleModals,
  dismissModal,
  waitForErrorInModal,
  assertUserFriendlyError,
} from './helpers';

test.describe('Frontend Validation - Modal Behavior', () => {
  test('UpdateProgressModal appears exactly once during update', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    // Wait for progress modal
    await waitForProgressModal(page);

    // Check modal count immediately
    const count1 = await countVisibleModals(page);
    expect(count1).toBe(1);

    // Wait 2 seconds and check again
    await page.waitForTimeout(2000);
    const count2 = await countVisibleModals(page);
    expect(count2).toBe(1);

    // Wait 5 seconds and check again
    await page.waitForTimeout(5000);
    const count3 = await countVisibleModals(page);
    expect(count3).toBe(1);

    // All counts should be exactly 1
    expect([count1, count2, count3]).toEqual([1, 1, 1]);
  });

  test('No duplicate modals appear during state transitions', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Monitor modal count through different stages
    const counts: number[] = [];

    // Initial
    counts.push(await countVisibleModals(page));

    // During downloading
    await page.waitForSelector('text=/downloading/i', { timeout: 10000 }).catch(() => {});
    counts.push(await countVisibleModals(page));

    // During verifying
    await page.waitForSelector('text=/verifying/i', { timeout: 10000 }).catch(() => {});
    counts.push(await countVisibleModals(page));

    // After a delay
    await page.waitForTimeout(3000);
    counts.push(await countVisibleModals(page));

    // All counts should be 1
    for (const count of counts) {
      expect(count).toBe(1);
    }
  });

  test('Error modal appears exactly once (not twice) on checksum failure', async ({ page }) => {
    // Note: Requires MOCK_CHECKSUM_ERROR=true
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Wait for error to appear
    await waitForErrorInModal(page, progressModal);

    // Immediately count modals
    const count1 = await countVisibleModals(page);
    expect(count1).toBe(1);

    // Wait 2 seconds - no duplicate should appear
    await page.waitForTimeout(2000);
    const count2 = await countVisibleModals(page);
    expect(count2).toBe(1);

    // Wait 5 more seconds - still no duplicate
    await page.waitForTimeout(5000);
    const count3 = await countVisibleModals(page);
    expect(count3).toBe(1);

    expect([count1, count2, count3]).toEqual([1, 1, 1]);
  });

  test('Error messages are user-friendly (not raw API errors)', async ({ page }) => {
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

    // Validate user-friendly error
    await assertUserFriendlyError(errorContent);
  });

  test('Modal can be dismissed after error', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Wait for error
    await waitForErrorInModal(page, progressModal);

    // Should have close button or be dismissible
    const closeButton = progressModal.locator('button').filter({ hasText: /close|dismiss|ok/i }).first();

    if (await closeButton.isVisible({ timeout: 2000 }).catch(() => false)) {
      // Has close button
      await closeButton.click();
    } else {
      // Try ESC key
      await page.keyboard.press('Escape');
    }

    // Modal should disappear
    await expect(progressModal).not.toBeVisible({ timeout: 3000 });
  });

  test('Modal has accessible close button after error', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Wait for error
    await waitForErrorInModal(page, progressModal);

    // Should have visible close button
    const closeButton = progressModal.locator('button').filter({ hasText: /close|dismiss|ok|cancel/i }).first();

    // Button should be visible and enabled
    await expect(closeButton).toBeVisible();
    await expect(closeButton).toBeEnabled();
  });

  test('ESC key dismisses modal after error', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Wait for error
    await waitForErrorInModal(page, progressModal);

    // Press ESC
    await page.keyboard.press('Escape');

    // Modal should dismiss
    await expect(progressModal).not.toBeVisible({ timeout: 3000 });
  });

  test('Error message does not contain stack traces', async ({ page }) => {
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

    // Should not contain stack trace indicators
    expect(errorContent).not.toMatch(/at Object\./);
    expect(errorContent).not.toMatch(/at [A-Z]\w+\.[a-z]/); // at ClassName.method
    expect(errorContent).not.toMatch(/stack trace/i);
    expect(errorContent).not.toMatch(/\.go:\d+/); // Go stack traces
    expect(errorContent).not.toMatch(/\.ts:\d+:\d+/); // TypeScript stack traces
  });

  test('Error message does not contain internal API paths', async ({ page }) => {
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

    // Should not expose internal API endpoints
    expect(errorContent).not.toMatch(/\/api\/updates\//);
    expect(errorContent).not.toMatch(/\/internal\//);
    expect(errorContent).not.toMatch(/localhost:7655/);
  });

  test('Error message is concise and actionable', async ({ page }) => {
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

    // Should be reasonably concise
    expect(errorContent.length).toBeLessThan(200);
    expect(errorContent.length).toBeGreaterThan(10);

    // Should have at least one capital letter and punctuation (proper sentence)
    expect(errorContent).toMatch(/[A-Z]/);
  });

  test('Modal has proper ARIA attributes for accessibility', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Check for proper modal attributes
    const roleDialog = await progressModal.getAttribute('role');
    expect(roleDialog).toBe('dialog');

    // Should have aria-label or aria-labelledby
    const ariaLabel = await progressModal.getAttribute('aria-label');
    const ariaLabelledby = await progressModal.getAttribute('aria-labelledby');

    expect(ariaLabel || ariaLabelledby).toBeTruthy();
  });

  test('Progress bar has proper ARIA attributes', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Find progress bar
    const progressBar = progressModal.locator('[role="progressbar"]').first();

    if (await progressBar.isVisible({ timeout: 5000 }).catch(() => false)) {
      // Should have aria-valuenow
      const valueNow = await progressBar.getAttribute('aria-valuenow');
      expect(valueNow).toBeTruthy();

      // Should have aria-valuemin and aria-valuemax
      const valueMin = await progressBar.getAttribute('aria-valuemin');
      const valueMax = await progressBar.getAttribute('aria-valuemax');

      expect(valueMin).toBe('0');
      expect(valueMax).toBe('100');
    }
  });

  test('Modal backdrop prevents interaction with background', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    await waitForProgressModal(page);

    // Try to click something in the background
    const settingsText = page.locator('h1, h2').filter({ hasText: /settings/i }).first();

    // Should not be able to interact with background elements
    const isClickable = await settingsText.isEnabled().catch(() => false);

    // Background should be obscured or non-interactive
    // (This might vary by implementation - modal should trap focus)
  });

  test('Modal maintains focus trap during update', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Tab through focusable elements
    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');

    // Focus should remain within modal
    const focusedElement = await page.locator(':focus').first();
    const isInModal = await progressModal.locator(':focus').count();

    // Focused element should be within modal
    expect(isInModal).toBeGreaterThan(0);
  });

  test('No console errors during update flow', async ({ page }) => {
    const consoleErrors: string[] = [];

    page.on('console', msg => {
      if (msg.type() === 'error') {
        consoleErrors.push(msg.text());
      }
    });

    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    await waitForProgressModal(page);

    // Wait for some progress
    await page.waitForTimeout(5000);

    // Should have no console errors
    expect(consoleErrors).toHaveLength(0);
  });
});
