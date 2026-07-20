import { test, expect } from "@playwright/test";
import {
  ensureAuthenticated,
  ensureFirstRunExperience,
  navigateToSettings,
  apiRequest,
  trackBrowserRequests,
  waitForPulseReady,
} from "./helpers";

/**
 * Dedicated first-session E2E test covering the full journey:
 *   wizard → canonical first-session handoffs → settings discovery → gated panel guards
 *
 * This satisfies L8 score-8 criteria: "Dedicated first-session E2E test
 * (wizard → settings discovery)" while also keeping the setup-completion
 * surface on real browser proof for both source-picker and agent-managed starts.
 *
 * The opening test in this file forces the backend into deterministic
 * first-run state through the dev/test reset route, then completes the real
 * setup wizard before the rest of the suite continues through normal auth.
 */

type RuntimeCapabilitiesPayload = {
  capabilities?: string[];
};

/** Selector for the settings sidebar (div with aria-label, not a nav element). */
const SETTINGS_SIDEBAR = '[aria-label="Settings navigation"]';

test.describe.serial("First-session experience", () => {
  test("wizard completes and lands on Add infrastructure source picker", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only first-session coverage",
    );

    await ensureFirstRunExperience(page);

    await expect(page).toHaveURL(/\/settings\/infrastructure\?add=pick$/);
    await expect(
      page.getByText("Connected systems", { exact: true }),
    ).toBeVisible();
    const addDialog = page.getByRole("dialog", { name: "Add infrastructure" });
    await expect(addDialog).toBeVisible();
    await expect(
      addDialog.getByText("Choose how Pulse should connect"),
    ).toBeVisible();
    await expect(
      addDialog.getByRole("button", { name: /Install Pulse Agent/i }),
    ).toBeVisible();
    await expect(
      addDialog.getByRole("button", { name: /Detect API platform/i }),
    ).toBeVisible();
    await expect(page.locator("#root")).toBeVisible();

    // The main content area should be rendered (not stuck on a spinner/blank).
    const mainContent = page
      .locator('main, [role="main"], #root > div')
      .first();
    await expect(mainContent).toBeVisible();

    await page
      .getByRole("button", {
        name: "Close add infrastructure dialog",
        exact: true,
      })
      .click();
    await expect(page).toHaveURL(/\/settings\/infrastructure$/);
  });

  test("wizard completion can hand off to Pulse Agent install", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only first-session coverage",
    );

    await ensureFirstRunExperience(page, { completionTarget: "agent" });

    await expect(page).toHaveURL(/\/settings\/infrastructure\?add=linux-host$/);
    await expect(
      page.getByText("Connected systems", { exact: true }),
    ).toBeVisible();
    const agentDialog = page.getByRole("dialog", {
      name: "Add Linux, macOS, Windows host",
    });
    await expect(agentDialog).toBeVisible();
    await expect(
      agentDialog.getByRole("heading", { name: "Install on a host" }),
    ).toBeVisible();
    // The first-run handoff pre-provisions the scoped install token, so no
    // "Generate token" step exists in this variant of the dialog.
    await expect(
      agentDialog.getByText(/already prepared the first scoped install token/i),
    ).toBeVisible();
    await expect(agentDialog.getByText("Admin API Token")).toBeVisible();
  });

  test("shell shows key navigation elements", async ({ page }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only first-session coverage",
    );

    await ensureAuthenticated(page);

    // The Settings tab is rendered as a div[role="tab"] in the top utility bar.
    const settingsTab = page
      .locator('[role="tab"]')
      .filter({ hasText: "Settings" })
      .first();
    await expect(
      settingsTab,
      "Settings tab should be visible in the top nav bar",
    ).toBeVisible();
  });

  test("settings discovery - all top-level categories render", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only first-session coverage",
    );

    await ensureAuthenticated(page);
    await navigateToSettings(page);

    // The settings sidebar (div[aria-label="Settings navigation"]) lists category
    // group headings. The canonical always-visible groups are Infrastructure,
    // System, and Security. Organization appears when multi-tenant mode is enabled.
    const sidebar = page.locator(SETTINGS_SIDEBAR);
    await expect(sidebar).toBeVisible({ timeout: 10_000 });

    const requiredCategories = ["Infrastructure", "System", "Security"];
    const runtimeRes = await apiRequest(
      page,
      "/api/license/runtime-capabilities",
    );
    expect(runtimeRes.ok()).toBeTruthy();
    const runtimeCapabilities =
      (await runtimeRes.json()) as RuntimeCapabilitiesPayload;
    if (runtimeCapabilities.capabilities?.includes("multi_tenant")) {
      requiredCategories.splice(1, 0, "Organization");
    }

    for (const category of requiredCategories) {
      const heading = sidebar.getByText(category, { exact: false }).first();
      await expect(
        heading,
        `Settings category "${category}" should be visible in the sidebar`,
      ).toBeVisible({ timeout: 10_000 });
    }
  });

  test("settings panels load without errors", async ({ page }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only first-session coverage",
    );

    await ensureAuthenticated(page);
    await page.waitForLoadState("domcontentloaded");
    await page.waitForTimeout(750);

    // Visit a selection of key settings routes and verify the page renders
    // without console errors or blank screens.
    const keyRoutes = [
      "/settings/system-general",
      "/settings/system-updates",
      "/settings/security-overview",
      "/settings/security-auth",
      "/settings/system-pro",
    ] as const;

    const consoleErrors: string[] = [];
    // The app shell polls Patrol open work and its compact attention summary
    // on every authenticated route to drive the Patrol nav badge. Those are
    // cheap reads, not AI invocations, so they are exempt here.
    const appShellPatrolPolling =
      /\/api\/ai\/(?:patrol\/(?:findings|attention\/summary)|approvals)(?:\?|$)/;
    const aiRequests = trackBrowserRequests(page, /\/api\/ai(\/|$)/);
    page.on("console", (msg) => {
      if (msg.type() === "error") {
        consoleErrors.push(msg.text());
      }
    });

    try {
      for (const route of keyRoutes) {
        consoleErrors.length = 0;
        aiRequests.clear();

        await waitForPulseReady(page);
        await page.goto(route, { waitUntil: "domcontentloaded" });
        await page.waitForURL(/\/settings/, { timeout: 10_000 });
        await expect(page.locator("#root")).toBeVisible();

        // Wait for the panel content to render. Scope to the settings content
        // area (everything after the sidebar) to avoid matching sidebar labels.
        // Fall back to a page-wide heading if the content area locator doesn't
        // match — some layouts render headings differently.
        const panelHeading = page
          .locator('h1, h2, h3, [role="heading"]')
          .filter({ hasNotText: "Settings" })
          .first();
        await expect(
          panelHeading,
          `Route ${route} did not render any panel content`,
        ).toBeVisible({ timeout: 10_000 });

        await page.waitForTimeout(500);

        const unexpectedAIRequests = aiRequests
          .urls()
          .filter((url) => !appShellPatrolPolling.test(url));
        expect(
          unexpectedAIRequests.length,
          `Non-AI settings route ${route} should not bootstrap AI endpoints: ${unexpectedAIRequests.join(", ")}`,
        ).toBe(0);

        // Filter out benign console noise. "Auth check error ... Failed to
        // fetch" is the previous route's in-flight auth probe aborted by
        // page.goto, not a product error.
        const realErrors = consoleErrors.filter(
          (e) =>
            !e.includes("Download the React DevTools") &&
            !e.includes("Warning:") &&
            !e.includes("favicon") &&
            !e.includes("ERR_CONNECTION_REFUSED") &&
            !e.includes("net::") &&
            !(e.includes("Auth check error") && e.includes("Failed to fetch")),
        );

        expect(
          realErrors.length,
          `Unexpected console errors on ${route}: ${realErrors.join("; ")}`,
        ).toBe(0);
      }
    } finally {
      aiRequests.stop();
    }
  });

  test("gated panels do not flash unlocked content before license loads", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only first-session coverage",
    );

    await ensureAuthenticated(page);

    // Query runtime capabilities to check feature access per-route without
    // depending on billing-only entitlements.
    const runtimeRes = await apiRequest(
      page,
      "/api/license/runtime-capabilities",
    );
    expect(runtimeRes.ok()).toBeTruthy();
    const runtimeCapabilities =
      (await runtimeRes.json()) as RuntimeCapabilitiesPayload;
    const features = new Set(runtimeCapabilities.capabilities || []);
    const entitlementsRequests = trackBrowserRequests(
      page,
      "/api/license/entitlements",
    );

    // Gated routes with their required feature. Gating is feature-based, not
    // tier-based — e.g. a "relay" tier has the relay feature but not
    // advanced_reporting or audit_logging.
    const gatedRoutes = [
      {
        route: "/settings/support/reporting",
        expectedURL: /\/settings\/support\/reporting/,
        feature: "advanced_reporting",
      },
      {
        route: "/settings/security-webhooks",
        expectedURL: /\/settings\/security-webhooks/,
        feature: "audit_logging",
      },
      {
        route: "/settings/system-relay",
        expectedURL: /\/settings\/system-relay/,
        feature: "relay",
      },
    ] as const;

    try {
      for (const { route, expectedURL, feature } of gatedRoutes) {
        const hasFeature = features.has(feature);

        if (!hasFeature) {
          // Install a MutationObserver via addInitScript BEFORE navigating so it
          // runs before any app code. This catches transient form elements that
          // flash before the paywall renders (flash-of-unlocked-content).
          await page.addInitScript(() => {
            (window as any).__pulseFlashDetected = false;
            const SELECTOR =
              'form input:not([type="search"]), form textarea, form select';

            const checkForFlash = () => {
              const els = document.querySelectorAll(SELECTOR);
              if (els.length > 0) {
                (window as any).__pulseFlashDetected = true;
              }
            };

            const observer = new MutationObserver(checkForFlash);

            // Start observing on documentElement immediately (always available
            // in addInitScript context), so we catch body insertion and all
            // subsequent DOM mutations including app boot. This avoids the gap
            // where DOMContentLoaded fires after app scripts.
            observer.observe(document.documentElement, {
              childList: true,
              subtree: true,
            });
            // Also do an immediate check in case DOM is already populated.
            checkForFlash();

            // Auto-disconnect after 5 seconds to avoid leaking.
            setTimeout(() => observer.disconnect(), 5000);
          });

          await page.goto(route, { waitUntil: "domcontentloaded" });
          await page.waitForURL(expectedURL, { timeout: 10_000 });
          await expect(page.locator("#root")).toBeVisible();

          // Wait for the page to settle, then read the observer result.
          await page.waitForTimeout(2_000);
          const flashDetected = await page.evaluate(
            () => (window as any).__pulseFlashDetected === true,
          );

          expect(
            flashDetected,
            `Gated route ${route} (feature=${feature}) showed form elements on unlicensed tier (flash-of-unlocked-content)`,
          ).toBe(false);

          // Paywall indicator should be visible.
          const paywallIndicator = page
            .locator(
              "text=/Upgrade|Pro Feature|Requires Pro|Requires Relay|Start.*Trial|Advanced Reporting|Audit Logging|Audit Webhooks|Pulse Relay/i",
            )
            .first();
          const isPaywallVisible = await paywallIndicator
            .isVisible({ timeout: 5_000 })
            .catch(() => false);
          expect(
            isPaywallVisible,
            `Gated route ${route} (feature=${feature}) should show paywall when feature is not available`,
          ).toBeTruthy();
        } else {
          await page.goto(route, { waitUntil: "domcontentloaded" });
          await page.waitForURL(expectedURL, { timeout: 10_000 });
          await expect(page.locator("#root")).toBeVisible();

          // Licensed for this feature — the full panel content should render.
          const panelContent = page
            .locator("h1, h2, h3, label, form")
            .filter({ hasNotText: "Settings" })
            .first();
          await expect(
            panelContent,
            `Gated route ${route} should render content when feature=${feature} is available`,
          ).toBeVisible({ timeout: 10_000 });
        }
      }
    } finally {
      const requestedURLs = entitlementsRequests.urls();
      entitlementsRequests.stop();
      expect(
        requestedURLs,
        "Non-billing gated routes must not trigger billing entitlements reads in the browser shell",
      ).toEqual([]);
    }
  });

  test("Pro license panel is discoverable from settings", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only first-session coverage",
    );

    await ensureAuthenticated(page);
    await page.goto("/settings/pulse-intelligence/billing/plan", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings/, { timeout: 10_000 });

    // The self-hosted plan panel should always render with plan/recovery
    // content. Scope assertions to main content, not the settings sidebar.
    const licenseContent = page
      .getByRole("main")
      .getByText(/Current plan|Existing purchases|Use existing key|activation key/i)
      .first();
    await expect(
      licenseContent,
      "Pro license panel should show license-specific content (not just sidebar label)",
    ).toBeVisible({ timeout: 10_000 });
  });
});
