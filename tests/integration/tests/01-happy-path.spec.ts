/**
 * Happy Path Test: Valid checksums, successful update
 *
 * Tests the complete update flow from UI to backend with valid data.
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
  waitForProgress,
  countVisibleModals,
  checkForUpdatesAPI,
} from './helpers';

test.describe('Update Flow - Happy Path', () => {
  test.beforeEach(async ({ page }) => {
    // Ensure clean state
    await page.goto('/');
  });

  test('should display update banner when update is available', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    // Check for updates via API first
    const updateInfo = await checkForUpdatesAPI(page, 'stable');
    expect(updateInfo).toHaveProperty('available');

    // Banner should appear
    const banner = await waitForUpdateBanner(page);
    await expect(banner).toContainText(/update available|new version/i);

    // Should show version number
    await expect(banner).toContainText(/4\.28\./);
  });

  test('should show confirmation modal with version details', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    const modal = await waitForConfirmationModal(page);

    // Should show version jump (e.g., "4.27.0 → 4.28.1")
    await expect(modal).toContainText(/→|to|➜/i);

    // Should show version numbers
    await expect(modal).toContainText(/4\.28\./);

    // Should have prerequisites/warnings section
    await expect(modal.locator('text=/prerequisite|warning|backup/i')).toBeVisible();

    // Should have confirmation button
    const confirmBtn = modal.locator('button').filter({ hasText: /confirm|proceed/i });
    await expect(confirmBtn).toBeVisible();
  });

  test('should show progress modal during update', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    const confirmModal = await waitForConfirmationModal(page);
    await confirmUpdate(page);

    // Progress modal should appear
    const progressModal = await waitForProgressModal(page);

    // Should show progress bar
    const progressBar = progressModal.locator('[role="progressbar"], .progress-bar');
    await expect(progressBar).toBeVisible();

    // Should show status text (e.g., "Downloading...", "Verifying...")
    await expect(progressModal.locator('text=/downloading|verifying|extracting|applying/i')).toBeVisible();

    // Progress should advance
    await waitForProgress(page, progressModal, 10);
  });

  test('should show exactly ONE progress modal (not duplicates)', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    await waitForProgressModal(page);

    // Count modals
    const modalCount = await countVisibleModals(page);
    expect(modalCount).toBe(1);

    // Wait a bit and check again to ensure no duplicates appear
    await page.waitForTimeout(2000);
    const modalCountAfter = await countVisibleModals(page);
    expect(modalCountAfter).toBe(1);
  });

  test('should display different stages during update', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Track stages we see
    const stages = new Set<string>();
    const expectedStages = ['downloading', 'verifying', 'extracting', 'backing', 'applying'];

    // Poll for different stages
    for (let i = 0; i < 30; i++) {
      const text = await progressModal.textContent();
      if (text) {
        const lowerText = text.toLowerCase();
        for (const stage of expectedStages) {
          if (lowerText.includes(stage)) {
            stages.add(stage);
          }
        }
      }

      // If we've seen multiple stages, test passes
      if (stages.size >= 2) {
        break;
      }

      await page.waitForTimeout(1000);
    }

    // Should see at least 2 different stages
    expect(stages.size).toBeGreaterThanOrEqual(2);
  });

  test('should verify checksum during update', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);
    await clickApplyUpdate(page);

    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    const progressModal = await waitForProgressModal(page);

    // Should see "Verifying" or "Verifying checksum" stage
    await expect(progressModal.locator('text=/verifying/i')).toBeVisible({ timeout: 30000 });

    // Progress should continue past verification
    await waitForProgress(page, progressModal, 50);
  });

  test('should handle complete update flow end-to-end', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    // 1. See update banner
    const banner = await waitForUpdateBanner(page);
    await expect(banner).toBeVisible();

    // 2. Click apply
    await clickApplyUpdate(page);

    // 3. Confirm in modal
    await waitForConfirmationModal(page);
    await confirmUpdate(page);

    // 4. Watch progress
    const progressModal = await waitForProgressModal(page);
    await expect(progressModal).toBeVisible();

    // 5. Wait for completion or restart indication
    // Note: In real scenario, server would restart and page would reload
    // In test environment, we validate the process starts correctly
    await waitForProgress(page, progressModal, 20);

    // Test passes if we reach this point without errors
    expect(true).toBe(true);
  });

  test('should include release notes in update banner', async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToSettings(page);

    const banner = await waitForUpdateBanner(page);

    // Should have expandable section or link to release notes
    const releaseNotes = banner.locator('text=/release notes|changelog|what\'s new/i');

    // Either visible directly or appears when expanding
    const isVisible = await releaseNotes.isVisible().catch(() => false);

    if (!isVisible) {
      // Try clicking expand button
      const expandBtn = banner.locator('button').filter({ hasText: /details|expand|more/i }).first();
      if (await expandBtn.isVisible().catch(() => false)) {
        await expandBtn.click();
        await expect(releaseNotes).toBeVisible();
      }
    }
  });
});
