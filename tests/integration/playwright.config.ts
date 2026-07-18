import { defineConfig, devices } from '@playwright/test';
import { preferredBrowserBaseURL } from './tests/runtime-defaults';

/**
 * QUARANTINE (2026-07-17): these specs assert the pre-platform-first UI
 * (element locators, URLs, and subheader copy that the June IA rebuild
 * replaced) and have failed on every push since 2026-06-29, drowning the
 * signal from the healthy specs. Quarantining them keeps the rest of the
 * suite as a live per-push gate while each file is re-pinned to the
 * current UI and removed from this list. This is a stopgap, not a fix:
 * the recovery work is tracked as its own effort, and a spec leaves this
 * list only by being repaired, never by deletion.
 */
const QUARANTINED_SPECS = [
  '**/03-multi-tenant.spec.ts',
  '**/05-settings-mobile-audit.spec.ts',
  '**/15-settings-shell-consistency.spec.ts',
  '**/17-proxmox-backups-layout.spec.ts',
  '**/18-patrol-runtime-state.spec.ts',
  '**/19-telemetry-disclosure.spec.ts',
  '**/20-local-doc-links.spec.ts',
  '**/30-setup-platform-connections-handoff.spec.ts',
  '**/43-platform-mock-runtime.spec.ts',
  '**/47-inline-selection-scroll-stability.spec.ts',
  '**/48-summary-hover-selection.spec.ts',
  '**/49-demo-scenario-curation.spec.ts',
  '**/51-quickstart-cross-surface.spec.ts',
  '**/52-ai-settings-provider-setup.spec.ts',
  '**/57-release-candidate-shell.spec.ts',
  '**/60-page-header-consistency.spec.ts',
  '**/62-runtime-home-onboarding-contract.spec.ts',
  '**/63-pbs-active-tasks.spec.ts',
  '**/75-settings-infrastructure-fleet-status-coherence.spec.ts',
  '**/77-msp-isolation.spec.ts',
  '**/79-update-flow.spec.ts',
  '**/84-docker-restart-real-lab-artifact.spec.ts',
];

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
      testIgnore: [...QUARANTINED_SPECS, '**/04-mobile.spec.ts'],
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
      testIgnore: [...QUARANTINED_SPECS, '**/journeys/**', '**/99-visual-crawl.spec.ts'],
    },
    {
      name: 'mobile-safari',
      use: {
        ...devices['iPhone 12'],
      },
      testIgnore: [...QUARANTINED_SPECS, '**/journeys/**', '**/99-visual-crawl.spec.ts'],
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
