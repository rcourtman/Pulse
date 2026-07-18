import { expect, test } from "@playwright/test";

import { ensureAuthenticated } from "./helpers";

const COMMERCIAL_DIAGNOSTICS_PAYLOAD = {
  version: "6.0.0",
  runtime: "go",
  uptime: 7200,
  nodes: [],
  pbs: [],
  system: {
    os: "linux",
    arch: "amd64",
    goVersion: "go1.25",
    numCPU: 8,
    numGoroutine: 42,
    memoryMB: 256,
  },
  metricsStore: {
    enabled: true,
    status: "healthy",
    dbSize: 1024 * 1024,
    rawCount: 200,
    minuteCount: 100,
    hourlyCount: 20,
    dailyCount: 10,
    totalPoints: 330,
    bufferSize: 0,
    notes: [],
  },
  commercialFunnel: {
    enabled: true,
    status: "active",
    windowDays: 30,
    summary: {
      pricing_viewed: 4,
      paywall_viewed: 1,
      trial_started: 1,
      upgrade_clicked: 0,
      checkout_clicked: 2,
      checkout_started: 2,
      checkout_completed: 1,
      license_activated: 1,
      license_activation_failed: 0,
      period: {
        from: "2026-03-21T00:00:00Z",
        to: "2026-04-20T00:00:00Z",
      },
    },
    daily: [
      {
        day: "2026-04-18",
        pricing_viewed: 1,
        paywall_viewed: 0,
        trial_started: 0,
        upgrade_clicked: 0,
        checkout_clicked: 1,
        checkout_started: 1,
        checkout_completed: 0,
        license_activated: 0,
        license_activation_failed: 0,
      },
      {
        day: "2026-04-19",
        pricing_viewed: 2,
        paywall_viewed: 1,
        trial_started: 1,
        upgrade_clicked: 0,
        checkout_clicked: 1,
        checkout_started: 1,
        checkout_completed: 1,
        license_activated: 1,
        license_activation_failed: 0,
      },
      {
        day: "2026-04-20",
        pricing_viewed: 1,
        paywall_viewed: 0,
        trial_started: 0,
        upgrade_clicked: 0,
        checkout_clicked: 0,
        checkout_started: 0,
        checkout_completed: 0,
        license_activated: 0,
        license_activation_failed: 0,
      },
    ],
    surfaces: [
      {
        key: "settings_self_hosted_billing_compare_prompt",
        pricing_viewed: 0,
        paywall_viewed: 0,
        trial_started: 0,
        upgrade_clicked: 0,
        checkout_clicked: 2,
        checkout_started: 0,
        checkout_completed: 0,
        license_activated: 0,
        license_activation_failed: 0,
      },
      {
        key: "settings_self_hosted_billing_plan",
        pricing_viewed: 4,
        paywall_viewed: 0,
        trial_started: 0,
        upgrade_clicked: 0,
        checkout_clicked: 0,
        checkout_started: 0,
        checkout_completed: 0,
        license_activated: 0,
        license_activation_failed: 0,
      },
    ],
    capabilities: [
      {
        key: "self_hosted_plan",
        pricing_viewed: 4,
        paywall_viewed: 0,
        trial_started: 0,
        upgrade_clicked: 0,
        checkout_clicked: 2,
        checkout_started: 2,
        checkout_completed: 1,
        license_activated: 1,
        license_activation_failed: 0,
      },
      {
        key: "relay",
        pricing_viewed: 0,
        paywall_viewed: 1,
        trial_started: 1,
        upgrade_clicked: 0,
        checkout_clicked: 0,
        checkout_started: 0,
        checkout_completed: 0,
        license_activated: 0,
        license_activation_failed: 0,
      },
    ],
    notes: [
      "Local pricing and activation events show at least one completed conversion in the current window.",
    ],
  },
  apiTokens: {
    enabled: true,
    tokenCount: 1,
    recommendTokenSetup: false,
    unusedTokenCount: 0,
    notes: [],
  },
  dockerAgents: null,
  alerts: null,
  aiChat: null,
  discovery: null,
  errors: [],
};

// The Commercial Funnel diagnostics card was deliberately removed from the
// user-facing diagnostics page (c6bcad255, "Remove internal analytics from
// diagnostics"); the UI strips commercialFunnel and infrastructureOnboarding
// from the payload before rendering. This spec pins that boundary: even when
// the API serves funnel analytics, none of it may render in the browser.
test("keeps internal commercial-funnel analytics out of the diagnostics page", async ({
  page,
}, testInfo) => {
  // The app shell makes auth-gated calls beyond the old stub set, so a
  // stubbed session renders the login page; sign in for real and stub only
  // the license and diagnostics surfaces under test.
  await ensureAuthenticated(page);

  for (const path of [
    "**/api/license/runtime-capabilities",
    "**/api/license/commercial-posture",
    "**/api/license/entitlements",
  ]) {
    await page.route(path, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          capabilities: [],
          limits: [],
          monitored_system_capacity: null,
          subscription_state: "free",
          upgrade_reasons: [],
          tier: "free",
          trial_eligible: false,
          hosted_mode: false,
          legacy_connections: {
            proxmox_nodes: 0,
            docker_hosts: 0,
            kubernetes_clusters: 0,
          },
          has_migration_gap: false,
        }),
      });
    });
  }

  await page.route("**/api/diagnostics", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(COMMERCIAL_DIAGNOSTICS_PAYLOAD),
    });
  });

  // Diagnostics moved from the retired /operations shell onto the settings
  // support group.
  await page.goto("/settings/support/diagnostics", { waitUntil: "domcontentloaded" });

  const runDiagnosticsButton = page
    .locator("button")
    .filter({ hasText: /^Run Diagnostics$/ })
    .first();

  await expect(runDiagnosticsButton).toBeVisible({ timeout: 30_000 });
  await runDiagnosticsButton.click();

  // The stubbed payload rendered: the metrics-store card proves the run
  // completed and the response was consumed.
  await expect(page.getByText("Metrics Store", { exact: false }).first()).toBeVisible({
    timeout: 30_000,
  });

  // None of the internal funnel analytics may surface.
  await expect(page.getByText("Commercial Funnel", { exact: true })).toHaveCount(0);
  await expect(page.getByText("Self Hosted Plan", { exact: true })).toHaveCount(0);
  await expect(
    page.getByText("Settings Self Hosted Billing Compare Prompt", { exact: true }),
  ).toHaveCount(0);
  await expect(page.getByText(/pricing_viewed/i)).toHaveCount(0);

  const screenshotPath = testInfo.outputPath("diagnostics-commercial-funnel.png");
  await page.screenshot({ path: screenshotPath, fullPage: true });
  console.log(`[diagnostics-commercial-funnel] screenshot: ${screenshotPath}`);
});
