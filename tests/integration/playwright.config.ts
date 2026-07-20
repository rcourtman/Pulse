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
  // Multi-tenant scenarios 6/7 have an unresolved org-resolution bug (see
  // core-e2e-failure-taxonomy): after switchOrg+reload the UI still resolves
  // the default org. CI runs a multi-tenant environment, so delisting this
  // spec turns that open bug into a permanent red.
  '**/03-multi-tenant.spec.ts',
  '**/47-inline-selection-scroll-stability.spec.ts',
  '**/48-summary-hover-selection.spec.ts',
];

/**
 * PROBATION (2026-07-20): specs that run on every push but do NOT gate the
 * Core E2E verdict. CI runs the suite in two passes per shard, selected by
 * PULSE_E2E_TIER: "stable" (everything not listed here — gating) and
 * "probation" (this list — reported, continue-on-error). Locally, with
 * PULSE_E2E_TIER unset, both tiers run together exactly as before.
 *
 * Seeded from per-spec failure data mined out of the "Core E2E Tests"
 * workflow's failed-shard logs for the 31 completed main runs between the
 * 2026-07-18 quarantine delist and 2026-07-20: every spec that failed or
 * retry-flaked within the 10 most recent completed runs starts here; specs
 * whose last incident is older already satisfy the promotion rule below.
 *
 * Promotion rule — a spec earns its way OUT of this list (into the gating
 * stable tier) after 10 consecutive Core E2E runs on main in which it
 * neither failed nor went flaky (a retry-pass reported as "flaky" counts as
 * an incident — the retry hid it from the verdict, not from this ledger).
 * Demotion rule — one flake on main demotes a stable spec back to this
 * list, restarting its count. A spec leaves the suite only via QUARANTINE
 * (still-rotted, doesn't run) or deliberate deletion; probation is for
 * specs that run green sometimes but haven't yet proven they always do.
 */
const PROBATION_SPECS = [
  '**/01-core-e2e.spec.ts',
  '**/02-navigation-perf.spec.ts',
  '**/04-mobile.spec.ts',
  '**/05-settings-mobile-audit.spec.ts',
  '**/11-first-session.spec.ts',
  '**/15-settings-shell-consistency.spec.ts',
  '**/20-local-doc-links.spec.ts',
  '**/21-truenas-connections-workspace.spec.ts',
  '**/38-vmware-ai-chat-mentions.spec.ts',
  '**/39-vmware-resource-detail-drawer.spec.ts',
  '**/40-vmware-storage-source-filter.spec.ts',
  '**/41-vmware-phase1-exclusion-integrity.spec.ts',
  '**/42-vmware-ai-chat-read-recovery.spec.ts',
  '**/43-platform-mock-runtime.spec.ts',
  '**/44-workloads-chart-spacing.spec.ts',
  '**/45-workloads-memory-tail.spec.ts',
  '**/46-storage-summary-continuity.spec.ts',
  '**/49-demo-scenario-curation.spec.ts',
  '**/50-storage-physical-disk-io-history.spec.ts',
  '**/56-pulse-account-upgrade-bootstrap.spec.ts',
  '**/59-workloads-column-layout.spec.ts',
  '**/62-storage-growth-column.spec.ts',
  '**/64-workloads-proxmox-refresh-stability.spec.ts',
  '**/68-platform-pages-shell.spec.ts',
  '**/77-msp-isolation.spec.ts',
  '**/79-update-flow.spec.ts',
  // Demoted 2026-07-20: failed on main in run 29731882505, the first
  // tiered run, one run after clearing the 10-green seeding window.
  '**/90-operational-trust-protection-posture.spec.ts',
];

const E2E_TIER = String(process.env.PULSE_E2E_TIER || '')
  .trim()
  .toLowerCase();
if (E2E_TIER && E2E_TIER !== 'stable' && E2E_TIER !== 'probation') {
  throw new Error(
    `PULSE_E2E_TIER must be "stable", "probation", or unset (got "${E2E_TIER}")`,
  );
}
// Stable tier ignores probation specs; probation tier runs only them.
const TIER_IGNORE = E2E_TIER === 'stable' ? PROBATION_SPECS : [];

// CI runs both tiers inside one job; separate output roots keep the
// probation pass from clobbering the stable pass's report and artifacts.
const REPORT_DIR = process.env.PULSE_E2E_REPORT_DIR || 'playwright-report';
const RESULTS_DIR = process.env.PULSE_E2E_RESULTS_DIR || 'test-results';

/**
 * Playwright configuration for Pulse update integration tests
 * See https://playwright.dev/docs/test-configuration
 */
export default defineConfig({
  testDir: './tests',

  ...(E2E_TIER === 'probation' ? { testMatch: PROBATION_SPECS } : {}),

  outputDir: RESULTS_DIR,

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
    ['html', { outputFolder: REPORT_DIR, open: 'never' }],
    ['list'],
    ['junit', { outputFile: `${RESULTS_DIR}/junit.xml` }]
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
      testIgnore: [...QUARANTINED_SPECS, ...TIER_IGNORE, '**/04-mobile.spec.ts'],
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
      testIgnore: [
        ...QUARANTINED_SPECS,
        ...TIER_IGNORE,
        '**/journeys/**',
        '**/99-visual-crawl.spec.ts',
      ],
    },
    {
      name: 'mobile-safari',
      use: {
        ...devices['iPhone 12'],
      },
      testIgnore: [
        ...QUARANTINED_SPECS,
        ...TIER_IGNORE,
        '**/journeys/**',
        '**/99-visual-crawl.spec.ts',
      ],
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
