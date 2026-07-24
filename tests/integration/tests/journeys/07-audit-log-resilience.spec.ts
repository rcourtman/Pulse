import { test, expect, ensureJourneyReady } from "./journeyAuth";

const capabilities = {
  capabilities: ["audit_logging"],
  limits: [],
  hosted_mode: false,
  max_history_days: 90,
  runtime: {
    build: "pro",
    label: "Pulse Pro runtime",
  },
  blocked_capabilities: [],
};

const event = (id: string, user = "operator") => ({
  id,
  timestamp: "2026-07-24T09:00:00Z",
  event: "login",
  user,
  ip: "127.0.0.1",
  path: "/api/auth",
  success: true,
  details: id,
});

test("audit log fails closed, recovers, pages atomically, and ignores stale responses", async ({
  page,
}, testInfo) => {
  test.skip(
    testInfo.project.name.startsWith("mobile-"),
    "Desktop audit resilience journey",
  );

  let mode:
    "initial" | "busy" | "recovered" | "page-size" | "race" | "invalid" =
    "initial";
  const auditRequests: URL[] = [];

  await page.route("**/api/license/runtime-capabilities", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(capabilities),
    });
  });
  await page.route("**/api/audit?*", async (route) => {
    const url = new URL(route.request().url());
    auditRequests.push(url);
    if (mode === "busy") {
      await route.fulfill({
        status: 503,
        headers: { "Retry-After": "2" },
        contentType: "application/json",
        body: JSON.stringify({
          code: "audit_store_busy",
          error: "Audit log storage is temporarily busy. Try again shortly.",
        }),
      });
      return;
    }
    if (mode === "invalid") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          events: null,
          total: 0,
          persistentLogging: true,
        }),
      });
      return;
    }
    if (mode === "race" && !url.searchParams.get("user")) {
      await new Promise((resolve) => setTimeout(resolve, 850));
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          events: [event("stale-row")],
          total: 1,
          persistentLogging: true,
        }),
      });
      return;
    }
    if (mode === "race") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          events: [event("latest-row", "alice")],
          total: 1,
          persistentLogging: true,
        }),
      });
      return;
    }
    if (mode === "page-size") {
      const size = Number(url.searchParams.get("limit"));
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          events: Array.from({ length: size }, (_, index) =>
            event(`page-${index}`),
          ),
          total: 250,
          persistentLogging: true,
        }),
      });
      return;
    }
    const id = mode === "recovered" ? "recovered-row" : "initial-row";
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        events: [event(id)],
        total: 1,
        persistentLogging: true,
      }),
    });
  });

  await ensureJourneyReady(page);
  await page.goto("/settings/security-audit", {
    waitUntil: "domcontentloaded",
  });
  await expect(page.getByText("initial-row", { exact: true })).toBeVisible();

  mode = "busy";
  await page.getByRole("button", { name: "Refresh" }).click();
  await expect(page.locator("main").getByRole("alert")).toContainText("storage is busy");
  await expect(page.getByText("initial-row", { exact: true })).toHaveCount(0);

  mode = "recovered";
  await page.getByRole("button", { name: "Refresh" }).click();
  await expect(page.getByText("recovered-row", { exact: true })).toBeVisible();
  await expect(page.locator("main").getByRole("alert")).toHaveCount(0);

  mode = "page-size";
  await page.getByLabel("Audit page size").selectOption("25");
  await expect(page.getByText("Showing 1-25 of 250")).toBeVisible();
  expect(
    auditRequests.some(
      (url) =>
        url.searchParams.get("limit") === "25" &&
        url.searchParams.get("offset") === "0",
    ),
  ).toBe(true);

  mode = "race";
  await page.getByRole("button", { name: "Refresh" }).click();
  await page.getByPlaceholder("Filter by user...").fill("alice");
  await expect(page.getByText("latest-row", { exact: true })).toBeVisible();
  await page.waitForTimeout(700);
  await expect(page.getByText("latest-row", { exact: true })).toBeVisible();
  await expect(page.getByText("stale-row", { exact: true })).toHaveCount(0);

  mode = "invalid";
  await page.getByRole("button", { name: "Refresh" }).click();
  await expect(page.locator("main").getByRole("alert")).toContainText("invalid response");
  await expect(page.getByText("latest-row", { exact: true })).toHaveCount(0);
});
