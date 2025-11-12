/**
 * Diagnostic test to understand why login is failing
 */

import { test, expect } from '@playwright/test';

test.describe('Login Diagnostic', () => {
  test('diagnose login page and API access', async ({ page }) => {
    // Enable console logging
    page.on('console', msg => console.log('BROWSER CONSOLE:', msg.text()));

    // Track network requests
    page.on('request', req => {
      if (req.url().includes('/api/')) {
        console.log('REQUEST:', req.method(), req.url());
      }
    });

    page.on('response', async res => {
      if (res.url().includes('/api/')) {
        console.log('RESPONSE:', res.status(), res.url());
        if (res.url().includes('/api/security/status')) {
          try {
            const body = await res.json();
            console.log('SECURITY STATUS RESPONSE:', JSON.stringify(body, null, 2));
          } catch (e) {
            console.log('Failed to parse response:', e);
          }
        }
      }
    });

    console.log('\n=== Navigating to login page ===');
    await page.goto('http://localhost:7655/login');
    console.log('Page loaded');

    // Wait a bit for any async operations
    await page.waitForTimeout(3000);

    console.log('\n=== Checking page state ===');
    const url = page.url();
    console.log('Current URL:', url);

    // Check what's actually on the page
    const bodyText = await page.locator('body').textContent();
    console.log('Page contains:', bodyText?.substring(0, 500));

    // Check for various elements
    const usernameField = page.locator('input[name="username"]');
    const usernameVisible = await usernameField.isVisible().catch(() => false);
    console.log('Username field visible:', usernameVisible);

    const setupHeading = page.locator('h1, h2').filter({ hasText: /setup|bootstrap|getting started/i });
    const setupVisible = await setupHeading.isVisible().catch(() => false);
    console.log('Setup/bootstrap heading visible:', setupVisible);

    const loginHeading = page.locator('h1, h2').filter({ hasText: /login|sign in/i });
    const loginVisible = await loginHeading.isVisible().catch(() => false);
    console.log('Login heading visible:', loginVisible);

    // Take screenshot
    await page.screenshot({ path: 'test-results/login-diagnostic.png', fullPage: true });
    console.log('Screenshot saved to test-results/login-diagnostic.png');

    console.log('\n=== Fetching API from browser context ===');
    const apiResponse = await page.evaluate(async () => {
      try {
        const res = await fetch('/api/security/status');
        const data = await res.json();
        return { ok: res.ok, status: res.status, data };
      } catch (err) {
        return { error: String(err) };
      }
    });
    console.log('Browser fetch result:', JSON.stringify(apiResponse, null, 2));

    // This test always passes - it's just for diagnostics
    expect(true).toBe(true);
  });
});
