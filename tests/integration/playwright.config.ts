import { defineConfig, devices } from "@playwright/test";
import { PROBATION_SPECS, QUARANTINED_SPECS } from "./e2e-tiering.mjs";
import { preferredBrowserBaseURL } from "./tests/runtime-defaults";

const E2E_TIER = String(process.env.PULSE_E2E_TIER || "")
  .trim()
  .toLowerCase();
if (E2E_TIER && E2E_TIER !== "stable" && E2E_TIER !== "probation") {
  throw new Error(
    `PULSE_E2E_TIER must be "stable", "probation", or unset (got "${E2E_TIER}")`,
  );
}
// Stable tier ignores probation specs; probation tier runs only them.
const TIER_IGNORE = E2E_TIER === "stable" ? PROBATION_SPECS : [];

// CI runs both tiers inside one job; separate output roots keep the
// probation pass from clobbering the stable pass's report and artifacts.
const REPORT_DIR = process.env.PULSE_E2E_REPORT_DIR || "playwright-report";
const RESULTS_DIR = process.env.PULSE_E2E_RESULTS_DIR || "test-results";

/**
 * Playwright configuration for Pulse update integration tests
 * See https://playwright.dev/docs/test-configuration
 */
export default defineConfig({
  testDir: "./tests",

  ...(E2E_TIER === "probation" ? { testMatch: PROBATION_SPECS } : {}),

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
    ["html", { outputFolder: REPORT_DIR, open: "never" }],
    ["list"],
    // Keep gating failures queryable through the check-run annotations API.
    // Raw Actions logs and report archives can exceed the maintainer's bounded
    // evidence surface; the built-in reporter includes project, spec, line,
    // and test title without changing retries, tiering, or the verdict.
    ...(process.env.GITHUB_ACTIONS === "true" && E2E_TIER === "stable"
      ? ([["github"]] as const)
      : []),
    ["junit", { outputFile: `${RESULTS_DIR}/junit.xml` }],
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
    ignoreHTTPSErrors: ["1", "true", "yes", "on"].includes(
      String(process.env.PULSE_E2E_INSECURE_TLS || "")
        .trim()
        .toLowerCase(),
    ),

    /* Collect trace when retrying the failed test */
    trace: "on-first-retry",

    /* Screenshot on failure */
    screenshot: "only-on-failure",

    /* Video on failure */
    video: "retain-on-failure",

    /* Default navigation timeout */
    navigationTimeout: 15000,

    /* Default action timeout */
    actionTimeout: 10000,
  },

  /* Configure projects for different browsers */
  projects: [
    {
      name: "chromium",
      use: {
        ...devices["Desktop Chrome"],
      },
      // Mobile-specific tests are intentionally excluded from the desktop project;
      // they rely on mobile viewports where md:hidden nav is visible, tables overflow, etc.
      testIgnore: [
        ...QUARANTINED_SPECS,
        ...TIER_IGNORE,
        "**/04-mobile.spec.ts",
      ],
    },
    {
      name: "mobile-chrome",
      use: {
        ...devices["Pixel 5"],
      },
      // Journey tests skip on mobile projects (all use test.skip for mobile-*),
      // so exclude them to avoid unnecessary browser launches. The visual
      // crawl takes 5+ minutes per project; one desktop pass is the budgeted
      // coverage, and dedicated mobile specs cover mobile layout.
      testIgnore: [
        ...QUARANTINED_SPECS,
        ...TIER_IGNORE,
        "**/journeys/**",
        "**/99-visual-crawl.spec.ts",
      ],
    },
    {
      name: "mobile-safari",
      use: {
        ...devices["iPhone 12"],
      },
      testIgnore: [
        ...QUARANTINED_SPECS,
        ...TIER_IGNORE,
        "**/journeys/**",
        "**/99-visual-crawl.spec.ts",
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
