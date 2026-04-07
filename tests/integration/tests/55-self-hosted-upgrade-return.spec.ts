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
const CHECKOUT_INTENT_ID = "cki_checkout_return";
const ACTIVATED_BILLING_URL = `${DEV_SERVER_URL}/settings/system/billing/plan?intent=max_monitored_systems&purchase=activated`;
const FINAL_BILLING_URL = `${DEV_SERVER_URL}/settings/system/billing/plan?intent=max_monitored_systems`;
const PURCHASE_RETURN_TOKEN = "prt_signed_checkout_return";

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
    "**/api/license/monitored-system-ledger",
    async (route) => {
      await fulfillJSON(route, MONITORED_SYSTEM_LEDGER);
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

    await context.route(`${PURCHASE_START_URL}**`, async (route) => {
      const requestUrl = new URL(route.request().url());
      purchaseStartURL = requestUrl.toString();
      expect(requestUrl.searchParams.get("feature")).toBe(
        "max_monitored_systems",
      );
      await route.fulfill({
        status: 303,
        headers: {
          location: `${PULSE_ACCOUNT_PORTAL_URL}?service=upgrade&checkout_intent_id=${CHECKOUT_INTENT_ID}`,
        },
        body: "",
      });
    });

    await context.route(`${PULSE_ACCOUNT_PORTAL_URL}**`, async (route) => {
      const requestUrl = new URL(route.request().url());
      expect(requestUrl.searchParams.get("service")).toBe("upgrade");
      expect(requestUrl.searchParams.get("feature")).toBeNull();
      expect(requestUrl.searchParams.get("return_url")).toBeNull();
      expect(requestUrl.searchParams.get("purchase_handoff_url")).toBeNull();
      expect(requestUrl.searchParams.get("checkout_intent_id")).toBe(
        CHECKOUT_INTENT_ID,
      );

      await route.fulfill({
        status: 200,
        contentType: "text/html",
        body:
          "<!doctype html><html><body>" +
          "<h1>Pulse Account</h1>" +
          "<p>Checkout complete. Returning to Pulse Pro.</p>" +
          `<script>setTimeout(function(){window.location.replace(${JSON.stringify(
            `${PURCHASE_RETURN_URL}?session_id=cs_upgrade_return&purchase_return_token=${encodeURIComponent(PURCHASE_RETURN_TOKEN)}`,
          )});},150);</script>` +
          "</body></html>",
      });
    });

    await context.route(`${PURCHASE_RETURN_URL}**`, async (route) => {
      const request = route.request();
      if (request.method() === "GET") {
        const requestUrl = new URL(request.url());
        expect(requestUrl.searchParams.get("session_id")).toBe(
          "cs_upgrade_return",
        );
        expect(requestUrl.searchParams.get("purchase_return_token")).toBe(
          PURCHASE_RETURN_TOKEN,
        );
        await route.fulfill({
          status: 200,
          contentType: "text/html",
          body:
            "<!doctype html><html><body>" +
            "<h1>Finalizing Pulse Pro upgrade</h1>" +
            `<form id="purchase-activation-continue-form" method="POST" action="${escapeAttribute(PURCHASE_RETURN_URL)}">` +
            '<input type="hidden" name="session_id" value="cs_upgrade_return">' +
            `<input type="hidden" name="purchase_return_token" value="${escapeAttribute(PURCHASE_RETURN_TOKEN)}">` +
            "</form>" +
            '<script>setTimeout(function(){document.getElementById("purchase-activation-continue-form")?.submit();},50);</script>' +
            "</body></html>",
        });
        return;
      }

      expect(request.method()).toBe("POST");
      const formData = new URLSearchParams(request.postData() || "");
      expect(formData.get("session_id")).toBe("cs_upgrade_return");
      expect(formData.get("purchase_return_token")).toBe(PURCHASE_RETURN_TOKEN);
      await route.fulfill({
        status: 200,
        contentType: "text/html",
        body:
          "<!doctype html><html><body>" +
          "<h1>Pulse Pro activated</h1>" +
          `<script>(function(){var redirectPath=${JSON.stringify(ACTIVATED_BILLING_URL)};if(window.opener&&!window.opener.closed){window.opener.location.assign(redirectPath);window.close();return;}window.location.replace(redirectPath);}());</script>` +
          "</body></html>",
      });
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

    await page.goto(
      `${PULSE_ACCOUNT_PORTAL_URL}?service=upgrade&checkout_intent_id=${CHECKOUT_INTENT_ID}`,
      {
        waitUntil: "domcontentloaded",
      },
    );
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
  });
});
