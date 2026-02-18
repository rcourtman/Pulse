import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration for Pulse update integration tests
 * See https://playwright.dev/docs/test-configuration
 */
export default defineConfig({
  testDir: './tests',

  /* Run tests in files in parallel */
  fullyParallel: false, // Update tests should run sequentially

  /* Fail the build on CI if you accidentally left test.only in the source code */
  forbidOnly: !!process.env.CI,

  /* Retry on CI only */
  retries: process.env.CI ? 2 : 0,

  /* Opt out of parallel tests on CI */
  workers: 1, // Update tests modify global state

  /* Reporter to use */
  reporter: [
    ['html', { outputFolder: 'playwright-report', open: 'never' }],
    ['list'],
    ['junit', { outputFile: 'test-results/junit.xml' }]
  ],

  /* Shared test timeout */
  timeout: 60000, // Updates can take time
  expect: {
    timeout: 10000,
  },

  /* Shared settings for all projects */
  use: {
    /* Base URL for all tests */
    baseURL: process.env.PULSE_BASE_URL || process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:7655',

    /* Allow testing against self-signed TLS when explicitly enabled */
    ignoreHTTPSErrors: ['1', 'true', 'yes', 'on'].includes(
      String(process.env.PULSE_E2E_INSECURE_TLS || '').trim().toLowerCase(),
    ),

    /* Collect trace when retrying the failed test */
    trace: 'on-first-retry',

    /* Screenshot on failure */
    screenshot: 'only-on-failure',

    /* Video on failure */
    video: 'retain-on-failure',

    /* Default navigation timeout */
    navigationTimeout: 15000,

    /* Default action timeout */
    actionTimeout: 10000,
  },

  /* Configure projects for different browsers */
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
      },
      // Mobile-specific tests are intentionally excluded from the desktop project;
      // they rely on mobile viewports where md:hidden nav is visible, tables overflow, etc.
      testIgnore: ['**/04-mobile.spec.ts'],
    },
    {
      name: 'mobile-chrome',
      use: {
        ...devices['Pixel 5'],
      },
    },
    {
      name: 'mobile-safari',
      use: {
        ...devices['iPhone 12'],
      },
    },

    // Uncomment to test on Firefox and WebKit
    // {
    //   name: 'firefox',
    //   use: { ...devices['Desktop Firefox'] },
    // },
    // {
    //   name: 'webkit',
    //   use: { ...devices['Desktop Safari'] },
    // },
  ],

  /* Run local dev server before starting the tests */
  // We use docker-compose instead, managed via npm scripts
  webServer: undefined,
});
