import { test, expect, type Page } from "@playwright/test";
import { apiRequest, ensureAuthenticated } from "./helpers";
import { PATROL_ROUTE } from "./routes";

/**
 * V6 License Activation E2E Test
 *
 * Exercises the v6 activation flow a paying self-hosted customer takes: paste
 * an activation key into the Settings plan page, let the backend activate it
 * against the licence server, confirm the plan state in the UI and API, then
 * clear it back to Community.
 *
 * The licence server is the in-memory Ed25519 issuer started by
 * scripts/with-offline-entitlements.mjs, which also points the source-built
 * backend's trust root at it. No production licence server, admin token or
 * signing material is involved. Without the wrapper every test skips:
 *
 *   PULSE_E2E_USE_LOCAL_BACKEND=true node scripts/with-offline-entitlements.mjs \
 *     node scripts/run-playwright.mjs tests/10-v6-license-activation.spec.ts \
 *     --project=chromium --workers=1 --retries=0
 */

const ACTIVATION_KEY = process.env.PULSE_E2E_OFFLINE_PRO_ACTIVATION_KEY || "";
const BILLING_PLAN_ROUTE = "/settings/pulse-intelligence/billing/plan";
const RECOVERY_ROUTE = `${BILLING_PLAN_ROUTE}?details=recovery`;
// Shared by both offline issuer plans; the grant carries it as the licence email.
const LICENSED_EMAIL = "e2e@example.invalid";

type EntitlementPayload = {
  subscription_state?: string;
  tier?: string;
  valid?: boolean;
  licensed_email?: string;
  limits?: Array<{ key: string }>;
};

type PatrolStatusPayload = {
  runtime_state?: string;
};

async function readEntitlements(page: Page): Promise<EntitlementPayload> {
  const res = await apiRequest(page, "/api/license/entitlements");
  expect(res.ok(), `Entitlements request failed: ${res.status()}`).toBeTruthy();
  return (await res.json()) as EntitlementPayload;
}

function licenseRecovery(page: Page) {
  return page.locator("details").filter({ hasText: "Manual key recovery" });
}

test.describe.serial("V6 license activation flow", () => {
  test.beforeEach(async ({}, testInfo) => {
    test.skip(
      !ACTIVATION_KEY,
      "PULSE_E2E_OFFLINE_PRO_ACTIVATION_KEY not set; run through scripts/with-offline-entitlements.mjs",
    );
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only license workflow",
    );
  });

  test.afterAll(async ({ browser }) => {
    if (!ACTIVATION_KEY) return;
    // Leave the backend on Community for any later spec, even after a failure.
    const page = await browser.newPage();
    try {
      await ensureAuthenticated(page);
      await apiRequest(page, "/api/license/clear", { method: "POST" });
    } catch {
      // Nothing to clean up if the backend is gone.
    } finally {
      await page.close();
    }
  });

  test("clears any existing license for a clean slate", async ({ page }) => {
    await ensureAuthenticated(page);

    const clearRes = await apiRequest(page, "/api/license/clear", {
      method: "POST",
    });
    expect(clearRes.ok(), `Clear license failed: ${clearRes.status()}`).toBeTruthy();
    expect((await readEntitlements(page)).tier).toBe("free");
  });

  test("activates the key via the Settings UI", async ({ page }) => {
    await ensureAuthenticated(page);
    await page.goto(RECOVERY_ROUTE);
    await expect(page.getByText("Current plan: Community")).toBeVisible({
      timeout: 30_000,
    });
    await expect(
      page.getByRole("heading", { name: "License recovery" }),
    ).toBeVisible();

    // ?details=recovery opens the manual key disclosure without a click.
    const recovery = licenseRecovery(page);
    await expect(recovery).toHaveAttribute("open", "");
    const keyField = page.locator("#pulse-pro-license-key");
    await expect(keyField).toBeVisible();
    await expect(recovery.getByRole("button", { name: "Clear key" })).toBeDisabled();
    await keyField.fill(ACTIVATION_KEY);

    const applyButton = recovery.getByRole("button", { name: "Apply key" });
    await expect(applyButton).toBeEnabled();
    await applyButton.click();

    // On a community runtime the success summary confirms the licence and
    // points at the private runtime instead of announcing Pro features.
    const summary = page.getByRole("status").filter({ hasText: "Pulse Pro license is active" });
    await expect(summary).toBeVisible({ timeout: 30_000 });
    await expect(summary.getByText(/^The license key was accepted\./)).toBeVisible();
    await expect
      .poll(async () => (await readEntitlements(page)).subscription_state, {
        timeout: 30_000,
        message: "License did not activate within timeout",
      })
      .toBe("active");
  });

  test("shows the activated plan in UI and API", async ({ page }) => {
    await ensureAuthenticated(page);
    await page.goto(BILLING_PLAN_ROUTE);
    await expect(page.getByText("Current plan: Pulse Pro")).toBeVisible({
      timeout: 30_000,
    });
    await expect(page.getByText("Current plan: Community")).toHaveCount(0);
    await expect(page.getByText(/^Active$/).first()).toBeVisible();
    await expect(page.getByText("90-day metric history").first()).toBeVisible();
    // The managed backend is a community build: the licence is accepted, and
    // the page says which runtime the Pro-only hooks need.
    await expect(page.getByText("Pro runtime missing").first()).toBeVisible();
    await expect(page.getByText(LICENSED_EMAIL, { exact: true })).toBeVisible();

    const ent = await readEntitlements(page);
    expect(ent.tier).toBe("pro");
    expect(ent.subscription_state).toBe("active");
    expect(ent.valid).toBe(true);
    expect(ent.licensed_email).toBe(LICENSED_EMAIL);

    // Self-hosted v6 plans meter neither monitored systems nor guests.
    expect(ent.limits?.find((l) => l.key === "max_monitored_systems")).toBeUndefined();
    expect(ent.limits?.find((l) => l.key === "max_guests")).toBeUndefined();
  });

  test("keeps activated installs on BYOK or local AI setup without quickstart", async ({
    page,
  }) => {
    await ensureAuthenticated(page);
    expect((await readEntitlements(page)).subscription_state).toBe("active");

    const settingsRes = await apiRequest(page, "/api/settings/ai");
    expect(settingsRes.ok(), `AI settings request failed: ${settingsRes.status()}`).toBeTruthy();
    const settings = await settingsRes.json();
    for (const key of [
      "quickstart_credits_available",
      "quickstart_credits_total",
      "quickstart_credits_remaining",
      "quickstart_blocked_reason",
    ]) {
      expect(settings).not.toHaveProperty(key);
    }

    await page.goto("/settings/system-ai");
    await expect(
      page.getByRole("heading", { name: "Provider & Models" }).first(),
    ).toBeVisible();
    await expect(page.getByText(/Patrol quickstart/i)).toHaveCount(0);

    await page.goto(PATROL_ROUTE);
    await expect(page.getByRole("heading", { name: "Patrol" }).first()).toBeVisible();
    await expect(page.getByText(/Patrol quickstart/i)).toHaveCount(0);

    const patrolStatusRes = await apiRequest(page, "/api/ai/patrol/status");
    expect(
      patrolStatusRes.ok(),
      `Patrol status request failed: ${patrolStatusRes.status()}`,
    ).toBeTruthy();
    const patrolStatus = (await patrolStatusRes.json()) as PatrolStatusPayload;
    expect(patrolStatus).not.toHaveProperty("using_quickstart");
    expect(patrolStatus).not.toHaveProperty("quickstart_credits_total");
    expect(patrolStatus).not.toHaveProperty("quickstart_credits_remaining");
  });

  test("clears the license via the Settings UI back to Community", async ({ page }) => {
    await ensureAuthenticated(page);
    await page.goto(RECOVERY_ROUTE);
    await expect(page.getByText("Current plan: Pulse Pro")).toBeVisible({
      timeout: 30_000,
    });

    // Clearing asks for a native confirm() first.
    page.once("dialog", (dialog) => dialog.accept());
    const clearButton = licenseRecovery(page).getByRole("button", { name: "Clear key" });
    await expect(clearButton).toBeEnabled();
    await clearButton.click();

    await expect(page.getByText(/License cleared/i).first()).toBeVisible();
    await expect
      .poll(async () => (await readEntitlements(page)).tier, { timeout: 10_000 })
      .toBe("free");

    await page.reload();
    await expect(page.getByText("Current plan: Community")).toBeVisible({
      timeout: 30_000,
    });
    await expect(page.getByText("Current plan: Pulse Pro")).toHaveCount(0);
  });
});
