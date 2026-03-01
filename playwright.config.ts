/**
 * Root-level Playwright configuration.
 *
 * This thin wrapper allows running journey tests from the workspace root:
 *
 *   npx playwright test tests/journeys --project=chromium
 *
 * It mirrors the settings in tests/integration/playwright.config.ts but
 * resolves testDir so that the Playwright runner and test files share the
 * same @playwright/test instance (avoiding the "two versions" error).
 */
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests/integration/tests',

  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,

  reporter: [
    ['html', { outputFolder: 'tests/integration/playwright-report', open: 'never' }],
    ['list'],
    ['junit', { outputFile: 'tests/integration/test-results/junit.xml' }],
  ],

  timeout: 60_000,
  expect: { timeout: 10_000 },

  use: {
    baseURL:
      process.env.PULSE_BASE_URL ||
      process.env.PLAYWRIGHT_BASE_URL ||
      'http://localhost:7655',

    ignoreHTTPSErrors: ['1', 'true', 'yes', 'on'].includes(
      String(process.env.PULSE_E2E_INSECURE_TLS || '').trim().toLowerCase(),
    ),

    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    navigationTimeout: 15_000,
    actionTimeout: 10_000,
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
      testIgnore: ['**/04-mobile.spec.ts'],
    },
    {
      name: 'mobile-chrome',
      use: { ...devices['Pixel 5'] },
      testIgnore: ['**/journeys/**'],
    },
    {
      name: 'mobile-safari',
      use: { ...devices['iPhone 12'] },
      testIgnore: ['**/journeys/**'],
    },
  ],

  webServer: undefined,
});
