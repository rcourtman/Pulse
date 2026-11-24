import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test as base, type Page } from "@playwright/test";

import { createAuthenticatedStorageState } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Push / Heartbeat checks is a staged feature controlled by
// frontend-modern/src/utils/featureReleases.json. This spec verifies both sides
// of the release switch:
//
//   - the production build under test hides push presets, the Push / Heartbeat
//     probe option, and the held PUSH_CHECKS guide whenever the release list
//     holds the feature back
//   - a development-marked page (featureReleaseTestBuild: true on /api/version,
//     never sent by any server) exercises the push check UI workflow, held
//     guide rendering, theme parity, non-admin viewer boundary, and (when run
//     against a live non-mock backend) real unauthenticated /api/push/{token}
//     pings and token rotation
const featureReleases = JSON.parse(
  fs.readFileSync(
    path.resolve(
      __dirname,
      "..",
      "..",
      "..",
      "frontend-modern",
      "src",
      "utils",
      "featureReleases.json",
    ),
    "utf8",
  ),
) as Record<string, { stage: string; guides: string[] }>;
const PUSH_CHECKS_STAGE = featureReleases.availabilityPushChecks?.stage;

// Must match FEATURE_RELEASE_TEST_BUILD_FIELD in frontend-modern/src/utils/featureReleases.ts.
const TEST_BUILD_FIELD = "featureReleaseTestBuild";

type WorkerFixtures = {
  authStorageStatePath: string;
};

const test = base.extend<{}, WorkerFixtures>({
  storageState: async ({ authStorageStatePath }, use) => {
    await use(authStorageStatePath);
  },
  authStorageStatePath: [
    async ({ browser }, use, workerInfo) => {
      const storageStatePath = path.resolve(
        __dirname,
        "..",
        "..",
        "tmp",
        "playwright-auth",
        `availability-push-${workerInfo.project.name}.json`,
      );
      fs.mkdirSync(path.dirname(storageStatePath), { recursive: true });
      await createAuthenticatedStorageState(browser, storageStatePath);
      try {
        await use(storageStatePath);
      } finally {
        fs.rmSync(storageStatePath, { force: true });
      }
    },
    { scope: "worker" },
  ],
});

async function asDevelopmentBuild(page: Page) {
  await page.route("**/api/version", async (route) => {
    const response = await route.fetch();
    const body = await response.json();
    await route.fulfill({
      response,
      json: { ...body, [TEST_BUILD_FIELD]: true },
    });
  });
}

async function isMockBackend(page: Page): Promise<boolean> {
  const res = await page.request.get("/api/availability-targets");
  if (!res.ok()) return false;
  const targets = (await res.json()) as Array<{ id?: string }>;
  return Array.isArray(targets) && targets.some((t) => String(t?.id || "").startsWith("mock-availability-"));
}

async function openAddCheckDialogFromAvailability(page: Page) {
  await page.goto("/availability", { waitUntil: "domcontentloaded" });
  await expect(page.getByTestId("availability-page")).toBeVisible({
    timeout: 60_000,
  });
  const addButton = page
    .locator(
      '[data-testid="availability-posture-add-button"], [data-testid="availability-add-check-button"]',
    )
    .first();
  await expect(addButton).toBeVisible({ timeout: 30_000 });
  await addButton.click();
  const dialog = page.getByRole("dialog").first();
  await expect(dialog).toBeVisible({ timeout: 15_000 });
  return dialog;
}

test.describe("Availability push and heartbeat checks", () => {
  test.setTimeout(120_000);

  test("the build under test exposes push checks and guide only when the release list allows it", async ({
    page,
  }) => {
    expect(["build", "preview", "released"]).toContain(PUSH_CHECKS_STAGE);

    const version = (await (
      await page.request.get("/api/version")
    ).json()) as Record<string, unknown>;
    expect(version[TEST_BUILD_FIELD]).toBeUndefined();

    const shown =
      PUSH_CHECKS_STAGE === "released" ||
      (PUSH_CHECKS_STAGE === "preview" && version.channel === "rc");

    const dialog = await openAddCheckDialogFromAvailability(page);
    const presetSelect = dialog.getByLabel("Preset");
    const probeSelect = dialog.getByLabel("Probe");

    const pushPresetOptions = presetSelect.locator('option[value="push-job"]');
    const pushProbeOptions = probeSelect.locator('option[value="push"]');

    if (shown) {
      await expect(pushPresetOptions).toHaveCount(1);
      await expect(pushProbeOptions).toHaveCount(1);
      await page.keyboard.press("Escape");
      return;
    }

    // Held back in this build: push preset and probe option are hidden on /availability.
    await expect(pushPresetOptions).toHaveCount(0);
    await expect(pushProbeOptions).toHaveCount(0);
    await page.keyboard.press("Escape");

    // Held back in Settings -> Monitoring -> Availability checks as well.
    await page.goto("/settings/monitoring/availability", {
      waitUntil: "domcontentloaded",
    });
    const settingsAddButton = page
      .getByRole("button", { name: /Add service\/device check/i })
      .first();
    await expect(settingsAddButton).toBeVisible({ timeout: 30_000 });
    await settingsAddButton.click();
    const settingsDialog = page.getByRole("dialog").first();
    await expect(settingsDialog).toBeVisible({ timeout: 15_000 });
    await expect(
      settingsDialog.getByLabel("Preset").locator('option[value="push-job"]'),
    ).toHaveCount(0);
    await expect(
      settingsDialog.getByLabel("Probe").locator('option[value="push"]'),
    ).toHaveCount(0);
    await page.keyboard.press("Escape");

    // Held guide is refused and unlinked from the shipped docs index.
    await page.goto("/docs/PUSH_CHECKS", { waitUntil: "domcontentloaded" });
    await expect(
      page.getByText(
        "That document is not part of this installation's shipped documentation set.",
      ),
    ).toBeVisible({ timeout: 30_000 });

    await page.goto("/docs/README", { waitUntil: "domcontentloaded" });
    await expect(page.locator("h2").first()).toBeVisible({ timeout: 30_000 });
    await expect(page.locator('a[href*="/docs/PUSH_CHECKS"]')).toHaveCount(0);
  });

  test("creates a cron push check, rotates its URL, checks light and dark themes, and verifies guide in dev mode", async ({
    page,
    playwright,
  }, testInfo) => {
    await asDevelopmentBuild(page);
    const mockMode = await isMockBackend(page);

    const checkName = `E2E Push Backup ${testInfo.project.name} ${Date.now()}`;
    const unauth = await playwright.request.newContext();
    let createdTargetId: string | null = null;

    if (mockMode) {
      // In CI mock mode, availability target writes return 403 mock_mode_enabled.
      // Simulate the target create and rotate endpoints in-page so the full UI
      // flow (preset, cron schedule, Ping URL, curl snippet, two-step rotation,
      // mobile wrapping, and theme contrast) is still verified end to end.
      let mockTarget: Record<string, unknown> | null = null;
      await page.route("**/api/availability-targets", async (route) => {
        if (route.request().method() === "POST") {
          const body = route.request().postDataJSON() as Record<string, unknown>;
          const push = (body.push as Record<string, unknown>) || {};
          mockTarget = {
            ...body,
            id: "e2e-mock-push-1",
            enabled: true,
            push: {
              ...push,
              token: "e2eMockInitialPushToken_1234567890abcdef",
            },
          };
          await route.fulfill({ status: 201, json: mockTarget });
          return;
        }
        await route.continue();
      });
      await page.route(
        "**/api/availability-targets/e2e-mock-push-1/rotate-push-token",
        async (route) => {
          const push = ((mockTarget?.push as Record<string, unknown>) || {});
          mockTarget = {
            ...(mockTarget || {}),
            push: {
              ...push,
              token: "e2eMockRotatedPushToken_fedcba0987654321",
            },
          };
          await route.fulfill({ status: 200, json: mockTarget });
        },
      );
    }

    try {
      const dialog = await openAddCheckDialogFromAvailability(page);

      await dialog.getByLabel("Preset").selectOption("push-job");
      const pushSection = dialog.getByTestId("availability-push-config");
      await expect(pushSection).toBeVisible({ timeout: 15_000 });

      await dialog.getByLabel("Name").fill(checkName);
      await pushSection.getByLabel("Schedule type").selectOption("cron");
      await pushSection.getByLabel(/Cron expression/i).fill("0 3 * * *");

      const createResponsePromise = page.waitForResponse(
        (res) =>
          res.url().includes("/api/availability-targets") &&
          res.request().method() === "POST" &&
          res.status() === 201,
      );
      await dialog
        .getByRole("button", { name: /Add service\/device check/i })
        .click();
      const createResponse = await createResponsePromise;
      const created = (await createResponse.json()) as {
        id?: string;
        push?: { token?: string };
      };
      createdTargetId = created.id ?? null;
      expect(createdTargetId).toBeTruthy();

      const pingUrlInput = pushSection.getByLabel("Ping URL");
      await expect(pingUrlInput).toBeVisible({ timeout: 15_000 });
      const initialUrl = await pingUrlInput.inputValue();
      expect(initialUrl).toContain("/api/push/");

      // Verify modal content does not overflow horizontally in either dark or light theme.
      for (const theme of ["dark", "light"] as const) {
        await page.evaluate((mode) => {
          document.documentElement.classList.toggle("dark", mode === "dark");
        }, theme);
        const overflowPx = await page.evaluate(
          () =>
            document.documentElement.scrollWidth -
            document.documentElement.clientWidth,
        );
        expect(overflowPx).toBeLessThanOrEqual(2);
      }

      // Two-step Rotate URL replaces the token immediately.
      await pushSection.getByRole("button", { name: "Rotate URL" }).click();
      await expect(pushSection.getByText(/will receive 404/i)).toBeVisible();
      await pushSection
        .getByRole("button", { name: "Confirm rotate URL" })
        .click();
      await expect
        .poll(async () => pingUrlInput.inputValue(), { timeout: 15_000 })
        .not.toBe(initialUrl);
      const rotatedUrl = await pingUrlInput.inputValue();
      expect(rotatedUrl).toContain("/api/push/");

      if (!mockMode) {
        // Real backend ping lifecycle: revoked token, /start, completion, /fail, recovery, and 1 KiB body cap.
        const revokedRes = await unauth.get(initialUrl);
        expect(revokedRes.status()).toBe(404);

        const startRes = await unauth.get(`${rotatedUrl}/start`);
        expect(startRes.status()).toBe(200);

        const okRes = await unauth.get(`${rotatedUrl}?msg=e2e+backup+ok`);
        expect(okRes.status()).toBe(200);

        const failRes = await unauth.post(
          `${rotatedUrl}/fail?msg=e2e+disk+full`,
        );
        expect(failRes.status()).toBe(200);

        const recoverRes = await unauth.get(
          `${rotatedUrl}?status=up&msg=e2e+recovered&ping=42`,
        );
        expect(recoverRes.status()).toBe(200);

        const oversizedRes = await unauth.post(rotatedUrl, {
          data: "x".repeat(2048),
        });
        expect(oversizedRes.status()).toBe(413);
      }

      // Close modal via Done.
      await dialog.getByRole("button", { name: "Done" }).click();
      await expect(dialog).toBeHidden({ timeout: 15_000 });

      if (!mockMode) {
        const checkRow = page.getByText(checkName).first();
        await expect(checkRow).toBeVisible({ timeout: 30_000 });
        await checkRow.click();

        // Push checks hide outbound Test now while retaining Pause and Edit.
        await expect(
          page.getByTestId("availability-drawer-toggle-button"),
        ).toBeVisible({ timeout: 15_000 });
        await expect(
          page.getByTestId("availability-drawer-edit-button"),
        ).toBeVisible();
        await expect(
          page.getByTestId("availability-drawer-test-button"),
        ).toHaveCount(0);

        // Clean up via two-step Remove target.
        await page.getByTestId("availability-drawer-edit-button").click();
        const editDialog = page.getByRole("dialog").first();
        await expect(editDialog).toBeVisible({ timeout: 15_000 });
        await editDialog.getByRole("button", { name: "Remove target" }).click();
        await editDialog
          .getByRole("button", { name: "Click again to confirm" })
          .click();
        await expect(editDialog).toBeHidden({ timeout: 15_000 });
        createdTargetId = null;
      }

      // In development-marked builds, the held guide renders cleanly.
      await page.goto("/docs/PUSH_CHECKS", { waitUntil: "domcontentloaded" });
      await expect(
        page.getByRole("heading", { name: /Push and Heartbeat Checks/i }),
      ).toBeVisible({ timeout: 30_000 });
    } finally {
      await unauth.dispose();
      if (!mockMode && createdTargetId) {
        await page
          .evaluate(async (id) => {
            const match = document.cookie.match(/(?:^|;\s*)pulse_csrf=([^;]+)/);
            const headers: Record<string, string> = {};
            if (match) headers["X-CSRF-Token"] = decodeURIComponent(match[1]);
            await fetch(`/api/availability-targets/${encodeURIComponent(id)}`, {
              method: "DELETE",
              headers,
            });
          }, createdTargetId)
          .catch(() => undefined);
      }
    }
  });

  test("hides check management actions and tokens for non-admin viewer sessions", async ({
    page,
  }) => {
    await asDevelopmentBuild(page);
    let availabilityTargetsFetched = false;
    await page.route("**/api/availability-targets", async (route) => {
      availabilityTargetsFetched = true;
      await route.continue();
    });
    await page.route("**/api/security/status", async (route) => {
      const response = await route.fetch();
      const body = (await response.json()) as Record<string, unknown>;
      await route.fulfill({
        response,
        json: {
          ...body,
          requiresAuth: true,
          settingsCapabilities: {
            ...((body.settingsCapabilities as Record<string, unknown>) || {}),
            availabilityRead: false,
          },
        },
      });
    });

    await page.goto("/availability", { waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("availability-page")).toBeVisible({
      timeout: 60_000,
    });
    await expect(
      page.getByTestId("availability-posture-add-button"),
    ).toHaveCount(0);
    await expect(page.getByTestId("availability-add-check-button")).toHaveCount(
      0,
    );
    const firstRow = page.locator("tbody tr").first();
    if ((await firstRow.count()) > 0) {
      await firstRow.click();
    }
    await expect(page.getByTestId("availability-drawer-edit-button")).toHaveCount(
      0,
    );
    await expect(page.getByTestId("availability-drawer-test-button")).toHaveCount(
      0,
    );
    await expect(
      page.getByTestId("availability-drawer-toggle-button"),
    ).toHaveCount(0);
    expect(availabilityTargetsFetched).toBe(false);
  });
});
