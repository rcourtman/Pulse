import { test, expect } from "@playwright/test";
import { apiRequest, ensureAuthenticated } from "./helpers";

/**
 * V6 License Activation E2E Test
 *
 * Exercises the full v6 activation flow: issue a test license via the license
 * server admin API, paste the activation key into the Pulse Settings UI,
 * verify activation succeeds, and confirm the UI shows the correct tier/features.
 *
 * Requires:
 *   PULSE_LICENSE_ADMIN_TOKEN — admin token for the license server
 *   PULSE_LICENSE_SERVER_URL — (optional) defaults to https://license.pulserelay.pro
 */

const LICENSE_SERVER_URL = (
  process.env.PULSE_LICENSE_SERVER_URL || "https://license.pulserelay.pro"
).replace(/\/+$/, "");

const ADMIN_TOKEN = process.env.PULSE_LICENSE_ADMIN_TOKEN || "";

type IssuedLicense = {
  license: { license_id: string };
  activation_key: { activation_key: string };
};

type EntitlementPayload = {
  subscription_state?: string;
  tier?: string;
  valid?: boolean;
  is_lifetime?: boolean;
  licensed_email?: string;
  days_remaining?: number;
  limits?: Array<{
    key: string;
    limit: number;
    current: number;
    state: string;
  }>;
};

type AISettingsPayload = {
  enabled?: boolean;
  model?: string;
  patrol_model?: string;
};

type PatrolStatusPayload = {
  runtime_state?: string;
};

test.describe.serial("V6 license activation flow", () => {
  /** Issued license ID — used for cleanup/revocation. */
  let issuedLicenseId = "";
  /** The activation key string (ppk_live_*). */
  let activationKey = "";

  test.afterAll(async ({ browser }) => {
    // Best-effort cleanup: clear license in Pulse and revoke on the server.
    const context = await browser.newContext();
    const page = await context.newPage();

    try {
      await ensureAuthenticated(page);

      // Clear the license in Pulse (ignore errors — may already be cleared).
      await apiRequest(page, "/api/license/clear", { method: "POST" }).catch(
        () => {},
      );
    } catch {
      // If Pulse is unreachable, nothing to clean up locally.
    }

    // Revoke the test license on the license server.
    if (issuedLicenseId && ADMIN_TOKEN) {
      try {
        await page.request.fetch(
          `${LICENSE_SERVER_URL}/v1/licenses/${issuedLicenseId}/revoke`,
          {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
              "X-API-Token": ADMIN_TOKEN,
            },
            data: { reason: "E2E test cleanup", reason_code: "test_cleanup" },
          },
        );
      } catch {
        // Best-effort — test license will expire in 1 day anyway.
      }
    }

    await context.close();
  });

  test("issues a v6 test license via admin API", async ({ page }, testInfo) => {
    test.skip(
      !ADMIN_TOKEN,
      "PULSE_LICENSE_ADMIN_TOKEN not set — skipping v6 activation tests",
    );
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only license workflow",
    );

    const response = await page.request.fetch(
      `${LICENSE_SERVER_URL}/v1/licenses/issue`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-API-Token": ADMIN_TOKEN,
        },
        data: {
          generation: "v6",
          email: "e2e-playwright@test.local",
          tier: "pro",
          plan_key: "pro_monthly",
          billing_model: "manual",
          duration_days: 1,
          max_monitored_systems: 0,
          max_guests: 5,
          max_installations: 3,
          send_email: false,
        },
      },
    );

    expect(
      [200, 201].includes(response.status()),
      `License issue failed: HTTP ${response.status()}`,
    ).toBeTruthy();

    const body = (await response.json()) as IssuedLicense;
    expect(body.license?.license_id).toBeTruthy();
    expect(body.activation_key?.activation_key).toBeTruthy();
    expect(body.activation_key.activation_key).toMatch(/^ppk_live_/);

    issuedLicenseId = body.license.license_id;
    activationKey = body.activation_key.activation_key;
  });

  test("clears any existing license for a clean slate", async ({
    page,
  }, testInfo) => {
    test.skip(!ADMIN_TOKEN, "PULSE_LICENSE_ADMIN_TOKEN not set");
    test.skip(testInfo.project.name.startsWith("mobile-"), "Desktop-only");

    await ensureAuthenticated(page);

    const clearRes = await apiRequest(page, "/api/license/clear", {
      method: "POST",
    });
    expect(
      clearRes.ok(),
      `Clear license failed: ${clearRes.status()}`,
    ).toBeTruthy();

    // Verify we're on the free tier now.
    const entRes = await apiRequest(page, "/api/license/entitlements");
    expect(entRes.ok()).toBeTruthy();
    const ent = (await entRes.json()) as EntitlementPayload;
    expect(ent.tier).toBe("free");
  });

  test("activates v6 license via the Settings UI", async ({
    page,
  }, testInfo) => {
    test.skip(!ADMIN_TOKEN, "PULSE_LICENSE_ADMIN_TOKEN not set");
    test.skip(testInfo.project.name.startsWith("mobile-"), "Desktop-only");
    test.skip(!activationKey, "No activation key from previous step");

    await ensureAuthenticated(page);
    await page.goto("/settings/system/billing");
    await expect(
      page.getByRole("heading", { name: "Plans & Activation" }).first(),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Activation & Recovery" }),
    ).toBeVisible();

    // Fill in the activation key.
    const textarea = page.locator("#pulse-pro-license-key");
    await expect(textarea).toBeVisible();
    await textarea.fill(activationKey);

    // Click Activate.
    const activateButton = page.getByRole("button", {
      name: "Activate License",
    });
    await expect(activateButton).toBeEnabled();
    await activateButton.click();

    // Wait for activation to complete: poll entitlements until active, which is
    // more reliable than matching a transient toast.
    await expect
      .poll(
        async () => {
          const res = await apiRequest(page, "/api/license/entitlements");
          if (!res.ok()) return "";
          const ent = (await res.json()) as EntitlementPayload;
          return ent.subscription_state;
        },
        { timeout: 30_000, message: "License did not activate within timeout" },
      )
      .toBe("active");
  });

  test("verifies activated license in UI and API", async ({
    page,
  }, testInfo) => {
    test.skip(!ADMIN_TOKEN, "PULSE_LICENSE_ADMIN_TOKEN not set");
    test.skip(testInfo.project.name.startsWith("mobile-"), "Desktop-only");
    test.skip(!activationKey, "No activation key from previous step");

    await ensureAuthenticated(page);
    await page.goto("/settings/system/billing");
    await expect(
      page.getByRole("heading", { name: "Plans & Activation" }).first(),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Current plan" }),
    ).toBeVisible();
    await expect(page.getByText("Current plan: Pulse Pro")).toBeVisible();
    await expect(page.getByText(/^Active$/).first()).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByText("No Pro license is active.")).toHaveCount(0);
    await expect(page.getByText("Unlimited").first()).toBeVisible();

    // Verify entitlements API agrees.
    const entRes = await apiRequest(page, "/api/license/entitlements");
    expect(
      entRes.ok(),
      `Entitlements request failed: ${entRes.status()}`,
    ).toBeTruthy();

    const ent = (await entRes.json()) as EntitlementPayload;
    expect(ent.tier).toBe("pro");
    expect(ent.subscription_state).toBe("active");
    expect(ent.valid).toBe(true);
    expect(ent.licensed_email).toBe("e2e-playwright@test.local");

    // Core monitoring is unlimited for self-hosted v6 plans.
    const agentLimit = ent.limits?.find(
      (l) => l.key === "max_monitored_systems",
    );
    expect(agentLimit).toBeUndefined();

    // Check max_guests limit.
    const guestLimit = ent.limits?.find((l) => l.key === "max_guests");
    expect(
      guestLimit,
      "max_guests limit not found in entitlements",
    ).toBeTruthy();
    expect(guestLimit!.limit).toBe(5);
  });

  test("keeps activated installs on BYOK or local AI setup without quickstart", async ({
    page,
  }, testInfo) => {
    test.skip(!ADMIN_TOKEN, "PULSE_LICENSE_ADMIN_TOKEN not set");
    test.skip(testInfo.project.name.startsWith("mobile-"), "Desktop-only");
    test.skip(!activationKey, "No activation key from previous step");

    await ensureAuthenticated(page);

    const entRes = await apiRequest(page, "/api/license/entitlements");
    expect(
      entRes.ok(),
      `Entitlements request failed: ${entRes.status()}`,
    ).toBeTruthy();
    const ent = (await entRes.json()) as EntitlementPayload;
    expect(ent.subscription_state).toBe("active");

    await page.goto("/settings/system-ai");
    await expect(
      page.getByRole("heading", { name: "Assistant & Patrol" }).first(),
    ).toBeVisible();

    const preSettingsRes = await apiRequest(page, "/api/settings/ai");
    expect(
      preSettingsRes.ok(),
      `AI settings request failed: ${preSettingsRes.status()}`,
    ).toBeTruthy();
    const preSettings = (await preSettingsRes.json()) as AISettingsPayload;
    expect(preSettings).not.toHaveProperty("quickstart_credits_available");
    expect(preSettings).not.toHaveProperty("quickstart_credits_total");
    expect(preSettings).not.toHaveProperty("quickstart_credits_remaining");
    expect(preSettings).not.toHaveProperty("quickstart_blocked_reason");

    const enableAI = page.getByRole("button", {
      name: "Enable Assistant and Patrol",
    });
    await expect(enableAI).toBeVisible();
    if (await page.getByText(/^Disabled$/).isVisible()) {
      await enableAI.click();
    }

    await expect(
      page.getByText("Connect a provider to power Pulse Assistant and Patrol."),
    ).toBeVisible();
    await expect(page.getByText(/Patrol quickstart/i)).toHaveCount(0);

    await page.goto("/ai");
    await expect(
      page.getByRole("heading", { name: "Patrol" }).first(),
    ).toBeVisible();
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

  test("clears license via UI and verifies free tier", async ({
    page,
  }, testInfo) => {
    test.skip(!ADMIN_TOKEN, "PULSE_LICENSE_ADMIN_TOKEN not set");
    test.skip(testInfo.project.name.startsWith("mobile-"), "Desktop-only");
    test.skip(!activationKey, "No activation key from previous step");

    await ensureAuthenticated(page);
    await page.goto("/settings/system/billing");
    await expect(
      page.getByRole("heading", { name: "Plans & Activation" }).first(),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Activation & Recovery" }),
    ).toBeVisible();

    // Set up one-shot dialog handler for the native confirm() prompt.
    page.once("dialog", (dialog) => dialog.accept());

    // Click Clear License.
    const clearButton = page.getByRole("button", { name: "Clear License" });
    await expect(clearButton).toBeEnabled({ timeout: 5_000 });
    await clearButton.click();

    // Wait for success toast.
    await expect(page.locator("text=/License cleared/i").first()).toBeVisible({
      timeout: 10_000,
    });

    // Verify entitlements API reverts to free tier.
    await expect
      .poll(
        async () => {
          const res = await apiRequest(page, "/api/license/entitlements");
          if (!res.ok()) return "";
          const ent = (await res.json()) as EntitlementPayload;
          return ent.tier;
        },
        { timeout: 10_000 },
      )
      .toBe("free");

    // Verify UI shows "No Pro license is active" or free-tier state.
    await page.reload();
    await expect(
      page.locator("text=/No Pro license is active/i").first(),
    ).toBeVisible({ timeout: 10_000 });
  });
});
