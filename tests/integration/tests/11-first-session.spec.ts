import { test, expect } from "@playwright/test";
import {
  ensureAuthenticated,
  ensureSessionAuthenticated,
  ensureFirstRunExperience,
  navigateToSettings,
  apiRequest,
  primaryNavigationLink,
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
const BILLING_PLAN_PATH = "/settings/pulse-intelligence/billing/plan";

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

    // Settings is a link in the primary navigation's utility group.
    const settingsTab = primaryNavigationLink(page, "Settings");
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

    // Settings bootstrap includes session-preferred organisation reads. Use
    // the actual browser session, not the token-only helper's intentional
    // unauthenticated probe followed by a token retry.
    await ensureSessionAuthenticated(page);
    await page.waitForLoadState("domcontentloaded");
    await page.waitForTimeout(750);

    // Visit a selection of key settings routes and verify the page renders
    // without console errors or blank screens.
    const keyRoutes = [
      { path: "/settings/system-general", readyHeading: "Appearance" },
      { path: "/settings/system-updates", readyHeading: "Updates" },
      { path: "/settings/security-overview", readyHeading: "Security Overview" },
      { path: "/settings/security-auth", readyHeading: "Authentication" },
      { path: BILLING_PLAN_PATH, readyHeading: "Current plan" },
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
      for (const { path: route, readyHeading } of keyRoutes) {
        // Retire the previous document before measuring this route. A
        // departing infrastructure row may still start its anomaly read;
        // that is not a Settings bootstrap request. Do not exempt anomalies
        // from the assertion or miss this document's early requests.
        await page.goto("about:blank");
        consoleErrors.length = 0;
        aiRequests.clear();

        await waitForPulseReady(page);
        await page.goto(route, { waitUntil: "domcontentloaded" });
        await page.waitForURL(/\/settings/, { timeout: 10_000 });
        await expect(page.locator("#root")).toBeVisible();

        // The outer page heading is present even while General remains on
        // "Loading configuration...". Require the consuming panel, not just
        // the Settings shell, before judging requests and console output.
        await expect(
          page.locator("[data-settings-content]").getByRole("heading", {
            name: readyHeading,
            exact: true,
          }),
          `Route ${route} did not finish loading a settings panel`,
        ).toBeVisible({ timeout: 10_000 });
        await expect(
          page.getByText("Loading configuration...", { exact: true }),
        ).toHaveCount(0);

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
    // tier-based. Pulse Mobile retirement is an all-tier notice, not a sales gate.
    const gatedRoutes = [
      {
        route: "/settings/support/reporting",
        expectedURL: /\/settings\/support\/reporting/,
        feature: "advanced_reporting",
        paywallIndicator: /Advanced Reporting|Pulse Pro runtime required/i,
      },
      {
        route: "/settings/security-webhooks",
        expectedURL: /\/settings\/security-webhooks/,
        feature: "audit_logging",
        paywallIndicator: /Audit (Logging|Webhooks)|Pulse Pro runtime required/i,
      },
    ] as const;

    await page.addInitScript(() => {
      (window as any).__pulseFlashDetected = false;
      const selector =
        '[data-settings-content] form input:not([type="search"]), [data-settings-content] form textarea, [data-settings-content] form select';
      const checkForFlash = () => {
        const visible = Array.from(document.querySelectorAll(selector)).some(
          element => element.getClientRects().length > 0 && getComputedStyle(element).visibility !== "hidden",
        );
        if (visible) {
          (window as any).__pulseFlashDetected = true;
        }
      };
      const observer = new MutationObserver(checkForFlash);
      (window as any).__pulseFlashObserver = observer;
      // Document exists before documentElement, including at init-script time.
      // Scope detection to the gated panel so sign-in controls are not mistaken
      // for unlocked licensed content.
      observer.observe(document, {
        childList: true, subtree: true, attributes: true,
        attributeFilter: ["class", "style", "hidden"],
      });
      checkForFlash();
    });

    try {
      for (const {
        route,
        expectedURL,
        feature,
        paywallIndicator: paywallPattern,
      } of gatedRoutes) {
        const hasFeature = features.has(feature);

        if (!hasFeature) {
          await page.goto(route, { waitUntil: "domcontentloaded" });
          await page.waitForURL(expectedURL, { timeout: 10_000 });
          await expect(page.locator("#root")).toBeVisible();

          const paywallIndicator = page
            .getByRole("main")
            .getByRole("heading", { level: 4, name: paywallPattern });
          await expect(
            paywallIndicator,
            `Gated route ${route} (feature=${feature}) should show its gate when unavailable`,
          ).toBeVisible({ timeout: 5_000 });

          // Observe through actual gate readiness, including a slow bootstrap.
          const flashDetected = await page.evaluate(() => {
            (window as any).__pulseFlashObserver?.disconnect();
            return (window as any).__pulseFlashDetected === true;
          });
          expect(
            flashDetected,
            `Gated route ${route} (feature=${feature}) showed form elements on unlicensed tier (flash-of-unlocked-content)`,
          ).toBe(false);
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

  test("Pulse Mobile explains retirement without selling access or exposing unlicensed pairing", async ({
    page,
  }) => {
    await ensureAuthenticated(page);
    const runtimeRes = await apiRequest(
      page,
      "/api/license/runtime-capabilities",
    );
    expect(runtimeRes.ok()).toBeTruthy();
    const runtime = (await runtimeRes.json()) as RuntimeCapabilitiesPayload;
    const entitlementReads = trackBrowserRequests(
      page,
      "/api/license/entitlements",
    );
    try {
      await page.goto("/settings/system-relay", {
        waitUntil: "domcontentloaded",
      });
      await expect(
        page.getByRole("heading", { name: "Pulse Mobile" }).last(),
      ).toBeVisible();
      const main = page.getByRole("main");
      const retirementNotice = main.getByRole("note", {
        name: "Pulse Mobile retirement",
      });
      await expect(retirementNotice).toBeVisible();
      await expect(retirementNotice.getByText(/31 March 2027/)).toBeVisible();
      await expect(
        main.getByRole("link", { name: /Upgrade|Try Pro|Buy Relay/i }),
      ).toHaveCount(0);
      if (runtime.capabilities?.includes("relay")) {
        // The paired-phone service remains usable until retirement. Do not
        // turn it off or replace its entitlement with an unlicensed notice.
        await expect(
          main.getByText("Allow Pulse Mobile connections", { exact: true }),
        ).toBeVisible();
        await expect(
          main.getByText("Pair Pulse Mobile", { exact: true }).first(),
        ).toBeVisible();
      } else {
        await expect(
          main.getByText(/no longer part of new plans/),
        ).toBeVisible();
        await expect(main.getByText(/ntfy, Gotify, or Pushover/)).toBeVisible();
        await expect(
          main.getByText("Allow Pulse Mobile connections", { exact: true }),
        ).toHaveCount(0);
        await expect(
          main.getByRole("button", { name: /Pair|Generate.*QR/i }),
        ).toHaveCount(0);
        await expect(
          main.locator(
            'form input:not([type="search"]), form textarea, form select',
          ),
        ).toHaveCount(0);
      }
    } finally {
      const urls = entitlementReads.urls();
      entitlementReads.stop();
      expect(
        urls,
        "Mobile retirement must use runtime capabilities, not billing reads",
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
    await page.goto(BILLING_PLAN_PATH, {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings/, { timeout: 10_000 });

    // The self-hosted plan panel should always render with plan/recovery
    // content. Scope assertions to main content, not the settings sidebar.
    const licenseContent = page
      .getByRole("main")
      .getByText(/Current plan|License recovery/i)
      .first();
    await expect(
      licenseContent,
      "Pro license panel should show license-specific content (not just sidebar label)",
    ).toBeVisible({ timeout: 10_000 });
  });
});
