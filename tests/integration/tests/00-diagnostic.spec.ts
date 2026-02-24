/**
 * Diagnostic test to understand why login is failing
 */

import { test, expect, type Page } from '@playwright/test';
import { waitForPulseReady } from './helpers';

const truthy = (value: string | undefined) => {
  if (!value) return false;
  return ['1', 'true', 'yes', 'on'].includes(value.trim().toLowerCase());
};

const gotoWithHealthRetry = async (page: Page, url: string, attempts = 3): Promise<void> => {
  let lastError: unknown = null;
  for (let attempt = 1; attempt <= attempts; attempt++) {
    try {
      await waitForPulseReady(page, 30_000);
      await page.goto(url);
      return;
    } catch (error) {
      lastError = error;
      if (attempt === attempts) {
        throw error;
      }
      console.log(`Navigation attempt ${attempt} failed, retrying...`);
      await page.waitForTimeout(1_000);
    }
  }
  throw lastError ?? new Error(`Failed to navigate to ${url}`);
};

test.describe('Login Diagnostic', () => {
  test.skip(
    !truthy(process.env.PULSE_E2E_DIAGNOSTIC),
    'Set PULSE_E2E_DIAGNOSTIC=1 to enable verbose login diagnostics',
  );

  test('diagnose login page and API access', async ({ page }) => {
    // Capture all console messages including errors
    page.on('console', msg => {
      const type = msg.type();
      const text = msg.text();
      if (type === 'error') {
        console.log('BROWSER ERROR:', text);
      } else if (type === 'warning') {
        console.log('BROWSER WARNING:', text);
      } else {
        console.log('BROWSER CONSOLE:', text);
      }
    });

    // Capture page errors
    page.on('pageerror', err => {
      console.log('PAGE ERROR:', err.message);
      console.log('Stack:', err.stack);
    });

    // Track network requests
    page.on('request', req => {
      console.log('REQUEST:', req.method(), req.url());
    });

    page.on('response', async res => {
      console.log('RESPONSE:', res.status(), res.url());
      if (res.url().includes('/api/security/status')) {
        try {
          const body = await res.json();
          console.log('SECURITY STATUS RESPONSE:', JSON.stringify(body, null, 2));
        } catch (e) {
          console.log('Failed to parse response:', e);
        }
      }
    });

    console.log('\n=== Navigating to login page ===');
    await gotoWithHealthRetry(page, '/');
    console.log('Page loaded');

    // Wait a bit for any async operations
    await page.waitForTimeout(3000);

    console.log('\n=== Checking page state ===');
    const url = page.url();
    console.log('Current URL:', url);

    // Check what's actually on the page
    const bodyText = await page.locator('body').textContent();
    console.log('Page text content:', bodyText?.substring(0, 500));

    // Check the actual DOM structure
    const bodyHTML = await page.locator('body').innerHTML();
    console.log('Page HTML structure:', bodyHTML.substring(0, 1000));

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

    console.log('\n=== Checking if app attempted to mount ===');
    const appState = await page.evaluate(() => {
      const root = document.getElementById('root');
      return {
        rootExists: !!root,
        rootChildren: root?.children.length || 0,
        rootHTML: root?.innerHTML || '',
        hasScriptTag: !!document.querySelector('script[src*="index"]'),
        scriptLoaded: (window as any).__PULSE_APP_LOADED__ || false,
      };
    });
    console.log('App mount state:', JSON.stringify(appState, null, 2));

    // This test always passes - it's just for diagnostics
    expect(true).toBe(true);
  });
});
