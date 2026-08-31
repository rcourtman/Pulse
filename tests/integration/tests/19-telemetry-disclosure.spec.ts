import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  test as base,
  expect,
  type Locator,
  type Page,
} from "@playwright/test";
import { createAuthenticatedStorageState } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const EXPECTED_TELEMETRY_SCHEMA_VERSION = 16;

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
        `telemetry-disclosure-${workerInfo.project.name}.json`,
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

async function expectPopupDoc(
  page: Page,
  link: Locator,
  pathname: string,
  expectedText: string,
) {
  const [popup] = await Promise.all([page.waitForEvent("popup"), link.click()]);

  await popup.waitForLoadState("domcontentloaded");
  expect(new URL(popup.url()).pathname).toBe(pathname);
  await expect(popup.locator("body")).toContainText(expectedText);
  await popup.close();
}

async function readTelemetryPreview(page: Page) {
  const preview = page.locator('pre[aria-label="Telemetry payload preview"]');
  await expect(preview).toBeVisible();
  return JSON.parse((await preview.textContent()) ?? "{}") as {
    install_id: string;
    event: string;
    schema_version: number;
    active_alerts_warning: number;
    alerts_resolution_under_15m_30d: number;
    alert_active_state_persistence_degraded_tenants: number;
  };
}

test.describe("Telemetry disclosure", () => {
  test.setTimeout(180_000);

  test("general settings opens the shipped privacy document", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only telemetry disclosure coverage",
    );

    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/settings/system-general", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings/, { timeout: 15_000 });

    const telemetrySummary = page.getByText(
      /rotating pseudonymous install ID, normalized release identity, runtime platform/i,
    );
    await expect(telemetrySummary).toBeVisible();
    await expect(telemetrySummary).toContainText(
      "Telemetry rows are retained for up to 90 days",
    );
    await expect(telemetrySummary).toContainText(
      "are not stored in telemetry rows",
    );

    const disclosureLink = page
      .getByRole("link", { name: "Full details" })
      .first();
    await expect(disclosureLink).toHaveAttribute("href", "/docs/PRIVACY");
    await expectPopupDoc(
      page,
      disclosureLink,
      "/docs/PRIVACY",
      "Pulse has one outbound usage-data scope",
    );
  });

  test("general settings lets operators preview and rotate the telemetry payload", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only telemetry disclosure coverage",
    );

    await page.goto("/settings/system-general", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings/, { timeout: 15_000 });

    await page.getByRole("button", { name: "Preview payload" }).click();

    const initialPreview = await readTelemetryPreview(page);
    expect(initialPreview.event).toBe("heartbeat");
    expect(initialPreview.install_id).toBeTruthy();
    expect(initialPreview.schema_version).toBe(
      EXPECTED_TELEMETRY_SCHEMA_VERSION,
    );
    expect(initialPreview.active_alerts_warning).toBeGreaterThanOrEqual(0);
    expect(
      initialPreview.alerts_resolution_under_15m_30d,
    ).toBeGreaterThanOrEqual(0);
    expect(
      initialPreview.alert_active_state_persistence_degraded_tenants,
    ).toBeGreaterThanOrEqual(0);

    await page.setViewportSize({ width: 390, height: 844 });
    await expect(
      page.locator('pre[aria-label="Telemetry payload preview"]'),
    ).toBeVisible();
    expect(
      await page.evaluate(
        () =>
          document.documentElement.scrollWidth <=
          document.documentElement.clientWidth,
      ),
    ).toBe(true);

    page.once("dialog", (dialog) => dialog.accept());
    await page.getByRole("button", { name: "Reset ID" }).click();

    await expect
      .poll(async () => {
        const refreshedPreview = await readTelemetryPreview(page);
        return refreshedPreview.install_id;
      })
      .not.toBe(initialPreview.install_id);
  });
});
