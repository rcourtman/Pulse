/**
 * Test helpers for Pulse update integration tests
 */

import { Page, expect } from '@playwright/test';

/**
 * Default admin credentials for testing
 */
export const ADMIN_CREDENTIALS = {
  username: 'admin',
  password: 'admin',
};

/**
 * Login as admin user
 */
export async function loginAsAdmin(page: Page) {
  await page.goto('/');
  await page.waitForSelector('input[name="username"]', { state: 'visible' });
  await page.fill('input[name="username"]', ADMIN_CREDENTIALS.username);
  await page.fill('input[name="password"]', ADMIN_CREDENTIALS.password);
  await page.click('button[type="submit"]');

  // Wait for redirect to dashboard
  await page.waitForURL(/\/(dashboard|nodes|proxmox)/);
}

/**
 * Navigate to settings page
 */
export async function navigateToSettings(page: Page) {
  await page.goto('/settings');

  // Wait for settings UI scaffolding (nav rail) to render
  await expect(
    page.locator('[aria-label="Settings navigation"], [data-testid="settings-nav"]')
  ).toBeVisible();
}

/**
 * Wait for update banner to appear
 */
export async function waitForUpdateBanner(page: Page, timeout = 30000) {
  const banner = page.locator('[data-testid="update-banner"], .update-banner').first();
  await expect(banner).toBeVisible({ timeout });
  return banner;
}

/**
 * Click "Apply Update" button in update banner
 */
export async function clickApplyUpdate(page: Page) {
  const applyButton = page.locator('button').filter({ hasText: /apply update/i }).first();
  await expect(applyButton).toBeVisible();
  await applyButton.click();
}

/**
 * Wait for update confirmation modal
 */
export async function waitForConfirmationModal(page: Page) {
  const modal = page.locator('[role="dialog"], .modal').filter({ hasText: /confirm/i }).first();
  await expect(modal).toBeVisible({ timeout: 10000 });
  return modal;
}

/**
 * Confirm update in modal (check acknowledgement and click confirm)
 */
export async function confirmUpdate(page: Page) {
  // Check acknowledgement checkbox if present
  const checkbox = page.locator('input[type="checkbox"]').first();
  if (await checkbox.isVisible({ timeout: 2000 }).catch(() => false)) {
    await checkbox.check();
  }

  // Click confirm button
  const confirmButton = page.locator('button').filter({ hasText: /confirm|proceed|continue/i }).first();
  await confirmButton.click();
}

/**
 * Wait for update progress modal
 */
export async function waitForProgressModal(page: Page) {
  const modal = page.locator('[data-testid="update-progress-modal"], [role="dialog"]')
    .filter({ hasText: /updating|progress|downloading/i })
    .first();
  await expect(modal).toBeVisible({ timeout: 10000 });
  return modal;
}

/**
 * Count visible modals on page
 */
export async function countVisibleModals(page: Page): Promise<number> {
  const modals = page.locator('[role="dialog"], .modal').filter({ hasText: /update|progress/i });
  return await modals.count();
}

/**
 * Wait for error message in modal
 */
export async function waitForErrorInModal(page: Page, modal: any) {
  const errorText = modal.locator('text=/error|failed|invalid/i').first();
  await expect(errorText).toBeVisible({ timeout: 30000 });
  return errorText;
}

/**
 * Check that error message is user-friendly (not a raw stack trace or API error)
 */
export async function assertUserFriendlyError(errorText: string) {
  // User-friendly errors should NOT contain:
  expect(errorText).not.toMatch(/stack trace|at Object\.|Error:/i);
  expect(errorText).not.toMatch(/500 Internal Server Error/i);
  expect(errorText).not.toMatch(/\/api\//i); // No API paths

  // User-friendly errors SHOULD be concise
  expect(errorText.length).toBeLessThan(200);
}

/**
 * Dismiss modal (click close button or backdrop)
 */
export async function dismissModal(page: Page) {
  // Try close button first
  const closeButton = page.locator('button[aria-label="Close"], button.close, button').filter({ hasText: /close|dismiss/i }).first();
  if (await closeButton.isVisible({ timeout: 2000 }).catch(() => false)) {
    await closeButton.click();
    return;
  }

  // Try ESC key
  await page.keyboard.press('Escape');
}

/**
 * Wait for progress to reach a certain percentage
 */
export async function waitForProgress(page: Page, modal: any, minPercent: number) {
  await page.waitForFunction(
    ({ modalSelector, min }) => {
      const modal = document.querySelector(modalSelector);
      if (!modal) return false;

      // Check for progress bar or percentage text
      const progressBar = modal.querySelector('[role="progressbar"]');
      if (progressBar) {
        const value = progressBar.getAttribute('aria-valuenow');
        return value && parseInt(value) >= min;
      }

      // Check for percentage text
      const text = modal.textContent || '';
      const match = text.match(/(\d+)%/);
      return match && parseInt(match[1]) >= min;
    },
    { modalSelector: '[role="dialog"]', min: minPercent },
    { timeout: 30000 }
  );
}

/**
 * Restart test environment with specific mock configuration
 */
export async function restartWithMockConfig(config: {
  checksumError?: boolean;
  networkError?: boolean;
  rateLimit?: boolean;
  staleRelease?: boolean;
}) {
  // This would be implemented by CI/test runner to restart containers
  // with new environment variables
  console.log('Mock config:', config);
}

/**
 * Reset test environment to clean state
 */
export async function resetTestEnvironment() {
  // Clear any cached update checks
  // Reset database state
  // Restart services
}

/**
 * Make API request to Pulse backend
 */
export async function apiRequest(page: Page, endpoint: string, options: any = {}) {
  const baseURL = 'http://localhost:7655';
  const response = await page.request.fetch(`${baseURL}${endpoint}`, options);
  return response;
}

/**
 * Check for updates via API
 */
export async function checkForUpdatesAPI(page: Page, channel: 'stable' | 'rc' = 'stable') {
  const response = await apiRequest(page, `/api/updates/check?channel=${channel}`);
  return response.json();
}

/**
 * Apply update via API
 */
export async function applyUpdateAPI(page: Page, downloadUrl: string) {
  const response = await apiRequest(page, '/api/updates/apply', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    data: { url: downloadUrl },
  });
  return response;
}

/**
 * Get update status via API
 */
export async function getUpdateStatusAPI(page: Page) {
  const response = await apiRequest(page, '/api/updates/status');
  return response.json();
}

/**
 * Measure time between events
 */
export class Timer {
  private start: number;

  constructor() {
    this.start = Date.now();
  }

  elapsed(): number {
    return Date.now() - this.start;
  }

  reset() {
    this.start = Date.now();
  }
}

/**
 * Poll until condition is met
 */
export async function pollUntil<T>(
  fn: () => Promise<T>,
  condition: (result: T) => boolean,
  options: { timeout?: number; interval?: number } = {}
): Promise<T> {
  const timeout = options.timeout || 30000;
  const interval = options.interval || 1000;
  const start = Date.now();

  while (Date.now() - start < timeout) {
    const result = await fn();
    if (condition(result)) {
      return result;
    }
    await new Promise(resolve => setTimeout(resolve, interval));
  }

  throw new Error(`Polling timed out after ${timeout}ms`);
}
