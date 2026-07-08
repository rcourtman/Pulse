import { defineConfig, devices } from '@playwright/test';
import { preferredBrowserBaseURL } from './tests/runtime-defaults';

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

  /* Retry once on CI to absorb flake; a second retry only multiplies the
     cost of real failures (each settings-shell failure costs ~14s/attempt,
     a failing visual crawl over 5 minutes). */
  retries: process.env.CI ? 1 : 0,

  /* On CI, a broken test environment fails most of the suite; abort early so
     the run produces a red completed verdict with a report instead of
     grinding until the job timeout cancels it with no verdict at all. */
  maxFailures: process.env.CI ? 20 : 0,

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
    baseURL: preferredBrowserBaseURL(),

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
      // Journey tests skip on mobile projects (all use test.skip for mobile-*),
      // so exclude them to avoid unnecessary browser launches. The visual
      // crawl takes 5+ minutes per project; one desktop pass is the budgeted
      // coverage, and dedicated mobile specs cover mobile layout.
      testIgnore: ['**/journeys/**', '**/99-visual-crawl.spec.ts'],
    },
    {
      name: 'mobile-safari',
      use: {
        ...devices['iPhone 12'],
      },
      testIgnore: ['**/journeys/**', '**/99-visual-crawl.spec.ts'],
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
