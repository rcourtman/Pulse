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
const EXPECTED_TELEMETRY_SCHEMA_VERSION = 19;

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
  await expect(popup.locator("body")).toContainText("successful_history_reads_over_7d_30d");
  await expect(popup.locator("body")).toContainText("Selecting a long range alone does not count");
  await popup.getByText("Schema v19 adds", { exact: false }).scrollIntoViewIfNeeded();
  await popup.screenshot({ path: test.info().outputPath("privacy-history-counter.png") });
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
    update_channel: string;
    update_check_outcome: string;
    update_available: boolean;
    successful_history_reads_over_7d_30d: number;
  };
}

async function showExtendedHistoryCount(page: Page) {
  await page.locator('pre[aria-label="Telemetry payload preview"]').evaluate((pre) => {
    const node = pre.firstChild!;
    const offset = (node.textContent ?? "").indexOf("successful_history_reads_over_7d_30d");
    const range = document.createRange();
    range.setStart(node, offset);
    range.setEnd(node, offset + "successful_history_reads_over_7d_30d".length);
    for (let parent = pre.parentElement; parent; parent = parent.parentElement) {
      if (parent.scrollHeight > parent.clientHeight && /auto|scroll/.test(getComputedStyle(parent).overflowY)) {
        parent.scrollTop += range.getBoundingClientRect().top - parent.getBoundingClientRect().top - parent.clientHeight / 2;
      }
    }
    const rect = range.getBoundingClientRect();
    if (rect.top < 60 || rect.bottom > window.innerHeight - 60) {
      throw new Error("Extended history count must be in the visible preview before capture");
    }
  });
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

    await page.setViewportSize({ width: 1280, height: 800 });
    await page.getByRole("button", { name: "Preview payload" }).focus();
    await page.getByRole("button", { name: "Preview payload" }).press("Enter");

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
    // Schema v18 update discovery is always present and closed (#2285).
    expect(["stable", "rc", "unknown"]).toContain(
      initialPreview.update_channel,
    );
    expect([
      "not_checked",
      "up_to_date",
      "available",
      "no_release",
      "rate_limited",
      "network_error",
      "metadata_error",
      "skipped",
      "error",
    ]).toContain(initialPreview.update_check_outcome);
    expect(typeof initialPreview.update_available).toBe("boolean");
    expect(initialPreview.successful_history_reads_over_7d_30d).toBe(0);
    await page.getByRole("button", { name: "Refresh payload" }).click();
    expect((await readTelemetryPreview(page)).successful_history_reads_over_7d_30d).toBe(0);
    // Fixture data must not create real extended-history usage.
    await showExtendedHistoryCount(page);
    await page.screenshot({ path: testInfo.outputPath("telemetry-preview-desktop.png") });

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
    expect((await readTelemetryPreview(page)).successful_history_reads_over_7d_30d).toBe(0);
    await showExtendedHistoryCount(page);
    await page.screenshot({ path: testInfo.outputPath("telemetry-preview-narrow.png") });
  });
});
