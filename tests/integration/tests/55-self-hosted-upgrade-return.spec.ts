import {
  test,
  expect,
  type BrowserContext,
  type Page,
  type Route,
} from "@playwright/test";

const DEV_SERVER_URL = "http://127.0.0.1:5173";
const PURCHASE_START_PATH = "/auth/license-purchase-start";
const PURCHASE_START_URL = `${DEV_SERVER_URL}${PURCHASE_START_PATH}`;
const PULSE_ACCOUNT_PORTAL_URL = "https://cloud.pulserelay.pro/portal";
const PURCHASE_RETURN_URL = `${DEV_SERVER_URL}/auth/license-purchase-activate`;
const PORTAL_HANDOFF_ID = "cph_checkout_return";
const ACTIVATED_BILLING_URL = `${DEV_SERVER_URL}/settings/system/billing/plan?intent=max_monitored_systems&purchase=activated`;
const CANCELLED_BILLING_URL = `${DEV_SERVER_URL}/settings/system/billing/plan?intent=max_monitored_systems&purchase=cancelled`;
const EXPIRED_BILLING_URL = `${DEV_SERVER_URL}/settings/system/billing/plan?intent=max_monitored_systems&purchase=expired`;
const FAILED_BILLING_URL = `${DEV_SERVER_URL}/settings/system/billing/plan?intent=max_monitored_systems&purchase=failed`;
const FINAL_BILLING_URL = `${DEV_SERVER_URL}/settings/system/billing/plan?intent=max_monitored_systems`;
const USAGE_BILLING_URL = `${DEV_SERVER_URL}/settings/system/billing/usage`;
const RECOVERY_BILLING_HREF =
  "/settings/system/billing/plan?intent=max_monitored_systems&details=recovery";
const RECOVERY_BILLING_URL = `${DEV_SERVER_URL}${RECOVERY_BILLING_HREF}`;
const PURCHASE_RETURN_TOKEN = "prt_signed_checkout_return";
const CHECKOUT_SESSION_ID = "cs_upgrade_return";
const MONITORED_SYSTEM_FEATURE = "max_monitored_systems";

const MONITORED_SYSTEM_ENTITLEMENTS = {
  capabilities: [],
  limits: [
    { key: "max_monitored_systems", limit: 5, current: 16, state: "enforced" },
  ],
  subscription_state: "expired",
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
  overflow_days_remaining: 14,
};

const MONITORED_SYSTEM_RUNTIME_CAPABILITIES = {
  capabilities: [],
  limits: [
    { key: "max_monitored_systems", limit: 5, current: 16, state: "enforced" },
  ],
  hosted_mode: false,
  max_history_days: 90,
};

const MONITORED_SYSTEM_COMMERCIAL_POSTURE = {
  subscription_state: "expired",
  upgrade_reasons: [],
  tier: "free",
  trial_eligible: false,
  legacy_connections: {
    proxmox_nodes: 0,
    docker_hosts: 0,
    kubernetes_clusters: 0,
  },
  has_migration_gap: false,
  overflow_days_remaining: 14,
};

const MONITORED_SYSTEM_LEDGER = {
  systems: [
    {
      name: "edge-cluster",
      type: "cluster",
      status: "warning",
      status_explanation: {
        summary:
          "At least one included source has a warning state, so Pulse keeps this monitored system under review.",
        reasons: [],
      },
      latest_included_signal: {
        name: "edge-cluster",
        type: "cluster",
        source: "kubernetes",
        at: "2026-04-07T09:00:00Z",
      },
      source: "kubernetes",
      explanation: {
        summary:
          "Counts as one monitored system because Pulse merged multiple top-level views into one canonical cluster.",
        reasons: [],
        surfaces: [
          { name: "edge-cluster", type: "cluster", source: "kubernetes" },
        ],
      },
    },
  ],
  total: 16,
  limit: 5,
};

function fulfillJSON(route: Route, payload: unknown, status = 200) {
  return route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(payload),
  });
}

function escapeAttribute(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll('"', "&quot;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

function buildPortalHandoffUrl(portalHandoffID = PORTAL_HANDOFF_ID): string {
  return `${PULSE_ACCOUNT_PORTAL_URL}?portal_handoff_id=${portalHandoffID}`;
}

function buildPurchaseReturnUrl(options: {
  sessionID?: string;
  portalHandoffID?: string;
  feature?: string;
  purchaseReturnToken?: string;
} = {}): string {
  const {
    sessionID = CHECKOUT_SESSION_ID,
    portalHandoffID = PORTAL_HANDOFF_ID,
    feature = MONITORED_SYSTEM_FEATURE,
    purchaseReturnToken = PURCHASE_RETURN_TOKEN,
  } = options;
  const url = new URL(PURCHASE_RETURN_URL);
  url.searchParams.set("session_id", sessionID);
  url.searchParams.set("portal_handoff_id", portalHandoffID);
  url.searchParams.set("feature", feature);
  url.searchParams.set("purchase_return_token", purchaseReturnToken);
  return url.toString();
}

function renderTimedRedirectPage(title: string, body: string, redirectUrl: string): string {
  return (
    "<!doctype html><html><body>" +
    `<h1>${title}</h1>` +
    `<p>${body}</p>` +
    `<script>setTimeout(function(){window.location.replace(${JSON.stringify(redirectUrl)});},150);</script>` +
    "</body></html>"
  );
}

function renderActivationBridgePage(options: {
  sessionID?: string;
  portalHandoffID?: string;
  feature?: string;
  purchaseReturnToken?: string;
  title?: string;
} = {}): string {
  const {
    sessionID = CHECKOUT_SESSION_ID,
    portalHandoffID = PORTAL_HANDOFF_ID,
    feature = MONITORED_SYSTEM_FEATURE,
    purchaseReturnToken = PURCHASE_RETURN_TOKEN,
    title = "Finalizing Pulse Pro upgrade",
  } = options;
  return (
    "<!doctype html><html><body>" +
    `<h1>${title}</h1>` +
    `<form id="purchase-activation-continue-form" method="POST" action="${escapeAttribute(PURCHASE_RETURN_URL)}">` +
    `<input type="hidden" name="session_id" value="${escapeAttribute(sessionID)}">` +
    `<input type="hidden" name="portal_handoff_id" value="${escapeAttribute(portalHandoffID)}">` +
    `<input type="hidden" name="feature" value="${escapeAttribute(feature)}">` +
    `<input type="hidden" name="purchase_return_token" value="${escapeAttribute(purchaseReturnToken)}">` +
    "</form>" +
    '<script>setTimeout(function(){document.getElementById("purchase-activation-continue-form")?.submit();},50);</script>' +
    "</body></html>"
  );
}

function renderActivationCompletionPage(title: string, redirectUrl: string): string {
  return (
    "<!doctype html><html><body>" +
    `<h1>${title}</h1>` +
    `<script>(function(){var redirectPath=${JSON.stringify(redirectUrl)};if(window.opener&&!window.opener.closed){window.opener.location.assign(redirectPath);window.close();return;}window.location.replace(redirectPath);}());</script>` +
    "</body></html>"
  );
}

async function configurePurchaseStartRoute(
  context: BrowserContext,
  onRequest?: (requestUrl: URL) => void,
) {
  await context.route(`${PURCHASE_START_URL}**`, async (route) => {
    const requestUrl = new URL(route.request().url());
    onRequest?.(requestUrl);
    expect(requestUrl.searchParams.get("feature")).toBe(MONITORED_SYSTEM_FEATURE);
    await route.fulfill({
      status: 303,
      headers: {
        location: buildPortalHandoffUrl(),
      },
      body: "",
    });
  });
}

async function configurePortalRedirectRoute(
  context: BrowserContext,
  options: {
    title?: string;
    body: string;
    redirectUrl: string;
  },
) {
  await context.route(`${PULSE_ACCOUNT_PORTAL_URL}**`, async (route) => {
    const requestUrl = new URL(route.request().url());
    expect(requestUrl.searchParams.get("service")).toBeNull();
    expect(requestUrl.searchParams.get("feature")).toBeNull();
    expect(requestUrl.searchParams.get("return_url")).toBeNull();
    expect(requestUrl.searchParams.get("purchase_handoff_url")).toBeNull();
    expect(requestUrl.searchParams.get("portal_handoff_id")).toBe(PORTAL_HANDOFF_ID);
    expect(requestUrl.searchParams.get("checkout_intent_id")).toBeNull();

    await route.fulfill({
      status: 200,
      contentType: "text/html",
      body: renderTimedRedirectPage(
        options.title ?? "Pulse Account",
        options.body,
        options.redirectUrl,
      ),
    });
  });
}

async function configurePurchaseReturnRoute(
  context: BrowserContext,
  options: {
    sessionID?: string;
    portalHandoffID?: string;
    feature?: string;
    purchaseReturnToken?: string;
    bridgeTitle?: string;
    completionTitle: string;
    finalRedirectUrl: string;
  },
) {
  const {
    sessionID = CHECKOUT_SESSION_ID,
    portalHandoffID = PORTAL_HANDOFF_ID,
    feature = MONITORED_SYSTEM_FEATURE,
    purchaseReturnToken = PURCHASE_RETURN_TOKEN,
    bridgeTitle,
    completionTitle,
    finalRedirectUrl,
  } = options;

  await context.route(`${PURCHASE_RETURN_URL}**`, async (route) => {
    const request = route.request();
    if (request.method() === "GET") {
      const requestUrl = new URL(request.url());
      expect(requestUrl.searchParams.get("session_id")).toBe(sessionID);
      expect(requestUrl.searchParams.get("portal_handoff_id")).toBe(portalHandoffID);
      expect(requestUrl.searchParams.get("feature")).toBe(feature);
      expect(requestUrl.searchParams.get("purchase_return_token")).toBe(
        purchaseReturnToken,
      );
      await route.fulfill({
        status: 200,
        contentType: "text/html",
        body: renderActivationBridgePage({
          sessionID,
          portalHandoffID,
          feature,
          purchaseReturnToken,
          title: bridgeTitle,
        }),
      });
      return;
    }

    expect(request.method()).toBe("POST");
    const formData = new URLSearchParams(request.postData() || "");
    expect(formData.get("session_id")).toBe(sessionID);
    expect(formData.get("portal_handoff_id")).toBe(portalHandoffID);
    expect(formData.get("feature")).toBe(feature);
    expect(formData.get("purchase_return_token")).toBe(purchaseReturnToken);
    await route.fulfill({
      status: 200,
      contentType: "text/html",
      body: renderActivationCompletionPage(completionTitle, finalRedirectUrl),
    });
  });
}

async function configureBillingFixtures(context: BrowserContext, page: Page) {
  await page.addInitScript(() => {
    localStorage.setItem("pulse_whats_new_v2_shown", "true");
  });

  await context.route("**/api/security/status", async (route) => {
    await fulfillJSON(route, {
      hasAuthentication: true,
      hideLocalLogin: false,
      ssoProviders: [],
      sessionCapabilities: {},
    });
  });

  await context.route("**/api/state", async (route) => {
    await fulfillJSON(route, { ok: true });
  });

  await context.route("**/api/system/settings", async (route) => {
    await fulfillJSON(route, {
      theme: "system",
      fullWidthMode: false,
    });
  });

  await context.route("**/api/license/runtime-capabilities", async (route) => {
    await fulfillJSON(route, MONITORED_SYSTEM_RUNTIME_CAPABILITIES);
  });

  await context.route("**/api/license/commercial-posture", async (route) => {
    await fulfillJSON(route, MONITORED_SYSTEM_COMMERCIAL_POSTURE);
  });

  await context.route("**/api/license/entitlements", async (route) => {
    await fulfillJSON(route, MONITORED_SYSTEM_ENTITLEMENTS);
  });

  await context.route(
    "**/api/license/monitored-system-ledger**",
    async (route) => {
      const payload = route.request().url().endsWith("/explain")
        ? { ledger: MONITORED_SYSTEM_LEDGER, preview: null }
        : MONITORED_SYSTEM_LEDGER;
      await fulfillJSON(route, payload);
    },
  );
}

async function openMonitoredSystemUpgradeArrival(page: Page) {
  await page.goto(`${DEV_SERVER_URL}/settings/system-general`, {
    waitUntil: "domcontentloaded",
  });
  await expect(
    page.getByRole("status").filter({ hasText: "Monitored systems: 16/5" }),
  ).toBeVisible();
  await page
    .getByRole("status")
    .filter({ hasText: "Monitored systems: 16/5" })
    .getByRole("link", { name: "Upgrade to add more" })
    .click();
  await page.waitForURL(
    "**/settings/system/billing/plan?intent=max_monitored_systems",
  );
  await expect(page.getByRole("tab", { name: "Plan" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
}

test.describe("Self-hosted upgrade return flow", () => {
  test("returns from Pulse Account checkout into the owned billing plan route", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only billing continuity",
    );

    const context = page.context();
    await configureBillingFixtures(context, page);
    let purchaseStartURL = "";

    await configurePurchaseStartRoute(context, (requestUrl) => {
      purchaseStartURL = requestUrl.toString();
    });
    await configurePortalRedirectRoute(context, {
      body: "Checkout complete. Returning to Pulse Pro.",
      redirectUrl: buildPurchaseReturnUrl(),
    });
    await configurePurchaseReturnRoute(context, {
      completionTitle: "Pulse Pro activated",
      finalRedirectUrl: ACTIVATED_BILLING_URL,
    });

    await openMonitoredSystemUpgradeArrival(page);

    const comparePlansLink = page.getByRole("link", { name: "Compare plans" });
    await expect(comparePlansLink).toHaveAttribute(
      "href",
      `${PURCHASE_START_PATH}?feature=max_monitored_systems`,
    );

    await expect(comparePlansLink).toHaveAttribute("target", "_blank");
    await comparePlansLink.click();
    await expect
      .poll(() => purchaseStartURL)
      .toBe(`${PURCHASE_START_URL}?feature=max_monitored_systems`);

    await page.goto(buildPortalHandoffUrl(), { waitUntil: "domcontentloaded" });
    await expect(
      page.getByRole("heading", { name: "Pulse Account" }),
    ).toBeVisible();
    await expect(
      page.getByText("Checkout complete. Returning to Pulse Pro."),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Activate in Pulse Pro" }),
    ).toHaveCount(0);
    await expect(page).toHaveURL(FINAL_BILLING_URL);
    await expect(
      page.getByText("Pulse Pro activated", { exact: true }),
    ).toBeVisible();
    const reviewUsageLink = page.getByRole("link", { name: "Review usage" });
    await expect(reviewUsageLink).toHaveAttribute("href", "/settings/system/billing/usage");
    await reviewUsageLink.click();
    await expect(page).toHaveURL(USAGE_BILLING_URL);
    await expect(page.getByRole("tab", { name: "Usage" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  test("returns cancelled checkout directly to the owned billing plan route", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only billing continuity",
    );

    const context = page.context();
    await configureBillingFixtures(context, page);

    await configurePurchaseStartRoute(context);
    await configurePortalRedirectRoute(context, {
      body: "Checkout cancelled. Returning to Pulse Pro.",
      redirectUrl: CANCELLED_BILLING_URL,
    });

    await openMonitoredSystemUpgradeArrival(page);

    const comparePlansLink = page.getByRole("link", { name: "Compare plans" });
    await comparePlansLink.click();

    await page.goto(buildPortalHandoffUrl(), { waitUntil: "domcontentloaded" });

    await expect(page).toHaveURL(CANCELLED_BILLING_URL);
    await expect(page.getByText("Checkout cancelled", { exact: true })).toBeVisible();
    await expect(page.getByRole("link", { name: "Compare plans" })).toHaveAttribute(
      "href",
      `${PURCHASE_START_PATH}?feature=max_monitored_systems`,
    );
  });

  test("returns an already-consumed checkout back to the activated billing state", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only billing continuity",
    );

    const context = page.context();
    await configureBillingFixtures(context, page);

    await configurePurchaseStartRoute(context);
    await configurePortalRedirectRoute(context, {
      body: "Checkout complete. Returning to Pulse Pro.",
      redirectUrl: buildPurchaseReturnUrl(),
    });
    await configurePurchaseReturnRoute(context, {
      bridgeTitle: "Confirming Pulse Pro activation state",
      completionTitle: "Purchase activation already completed",
      finalRedirectUrl: ACTIVATED_BILLING_URL,
    });

    await openMonitoredSystemUpgradeArrival(page);
    await page.getByRole("link", { name: "Compare plans" }).click();
    await page.goto(buildPortalHandoffUrl(), { waitUntil: "domcontentloaded" });

    await expect(page).toHaveURL(FINAL_BILLING_URL);
    await expect(
      page.getByText("Pulse Pro activated", { exact: true }),
    ).toBeVisible();
    await expect(page.getByRole("link", { name: "Review usage" })).toHaveAttribute(
      "href",
      "/settings/system/billing/usage",
    );
  });

  test("returns an expired or mismatched checkout state to restart the owned upgrade flow", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only billing continuity",
    );

    const context = page.context();
    await configureBillingFixtures(context, page);

    await configurePurchaseStartRoute(context);
    await configurePortalRedirectRoute(context, {
      body: "Secure upgrade state expired. Returning to Pulse Pro.",
      redirectUrl: buildPurchaseReturnUrl(),
    });
    await configurePurchaseReturnRoute(context, {
      bridgeTitle: "Verifying Pulse Pro checkout return",
      completionTitle: "Upgrade return expired",
      finalRedirectUrl: EXPIRED_BILLING_URL,
    });

    await openMonitoredSystemUpgradeArrival(page);
    await page.getByRole("link", { name: "Compare plans" }).click();
    await page.goto(buildPortalHandoffUrl(), { waitUntil: "domcontentloaded" });

    await expect(page).toHaveURL(EXPIRED_BILLING_URL);
    await expect(
      page.getByText("Upgrade return expired", { exact: true }),
    ).toBeVisible();
    const restartUpgradeLink = page.getByRole("link", { name: "Restart upgrade" });
    await expect(restartUpgradeLink).toHaveAttribute(
      "href",
      `${PURCHASE_START_PATH}?feature=max_monitored_systems`,
    );
    await expect(restartUpgradeLink).toHaveAttribute("target", "_blank");
  });

  test("returns local activation failures to the billing recovery entry point", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only billing continuity",
    );

    const context = page.context();
    await configureBillingFixtures(context, page);

    await configurePurchaseStartRoute(context);
    await configurePortalRedirectRoute(context, {
      body: "Checkout complete, but Pulse Pro needs local recovery.",
      redirectUrl: buildPurchaseReturnUrl(),
    });
    await configurePurchaseReturnRoute(context, {
      bridgeTitle: "Finishing local Pulse Pro activation",
      completionTitle: "Activation needs attention",
      finalRedirectUrl: FAILED_BILLING_URL,
    });

    await openMonitoredSystemUpgradeArrival(page);
    await page.getByRole("link", { name: "Compare plans" }).click();
    await page.goto(buildPortalHandoffUrl(), { waitUntil: "domcontentloaded" });

    await expect(page).toHaveURL(FAILED_BILLING_URL);
    await expect(
      page.getByText("Activation needs attention", { exact: true }),
    ).toBeVisible();
    const recoveryLink = page.getByRole("link", { name: "Open recovery" });
    await expect(recoveryLink).toHaveAttribute("href", RECOVERY_BILLING_HREF);
    await expect(recoveryLink).not.toHaveAttribute("target", "_blank");
    await recoveryLink.click();
    await expect(page).toHaveURL(RECOVERY_BILLING_URL);
    await expect(page.locator("#pulse-pro-recovery")).toBeVisible();
  });
});
