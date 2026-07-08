import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test as base, expect, type Page } from "@playwright/test";
import { createAuthenticatedStorageState } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

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
        `truenas-connections-workspace-${workerInfo.project.name}.json`,
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

const healthyAt = () => new Date(Date.now() - 5 * 60_000).toISOString();
const failingAt = () => new Date(Date.now() - 2 * 60_000).toISOString();

const buildTrueNASConnectionRows = () => ({
  connections: [
    {
      id: "truenas-truenas-1",
      type: "truenas",
      name: "Tower NAS",
      address: "tower.local",
      state: "active",
      stateReason: "",
      enabled: true,
      surfaces: ["storage", "recovery"],
      scope: {},
      lastSeen: healthyAt(),
      lastError: null,
      source: "manual",
      capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
    },
    {
      id: "truenas-truenas-2",
      type: "truenas",
      name: "Backup Vault",
      address: "vault.local",
      state: "unauthorized",
      stateReason: "authentication failed",
      enabled: false,
      surfaces: ["storage"],
      scope: {},
      lastSeen: failingAt(),
      lastError: { message: "authentication failed", at: failingAt() },
      source: "manual",
      capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
    },
  ],
  systems: [],
});

const buildSafeTrueNASAdmissionPreview = () => ({
  current_count: 1,
  projected_count: 1,
  additional_count: 0,
  limit: 10,
  would_exceed_limit: false,
  effect: "attaches_existing",
  current_systems: [],
  projected_systems: [
    {
      name: "tower",
      type: "truenas-system",
      status: "online",
      status_explanation: { summary: "", reasons: [] },
      latest_included_signal: {
        name: "tower",
        type: "truenas-system",
        source: "truenas",
        at: new Date().toISOString(),
      },
      source: "truenas",
      explanation: { summary: "", reasons: [], surfaces: [] },
    },
  ],
  current_system: null,
  projected_system: null,
});

const buildExceededTrueNASAdmissionPreview = () => ({
  current_count: 9,
  projected_count: 10,
  additional_count: 1,
  limit: 9,
  would_exceed_limit: true,
  effect: "creates_new",
  current_systems: [],
  projected_systems: [
    {
      name: "tower",
      type: "truenas-system",
      status: "online",
      status_explanation: { summary: "", reasons: [] },
      latest_included_signal: {
        name: "tower",
        type: "truenas-system",
        source: "truenas",
        at: new Date().toISOString(),
      },
      source: "truenas",
      explanation: { summary: "", reasons: [], surfaces: [] },
    },
  ],
  current_system: null,
  projected_system: null,
});

const openAddTrueNASDialog = async (page: Page) => {
  await page.goto("/settings/infrastructure", { waitUntil: "domcontentloaded" });
  await expect(
    page.getByRole("heading", { level: 2, name: "Connected systems" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Add infrastructure" }).first().click();
  const picker = page.getByRole("dialog", { name: "Add infrastructure" });
  await expect(picker).toBeVisible();
  await picker.getByRole("button", { name: /TrueNAS SCALE/ }).click();
  const dialog = page.getByRole("dialog", { name: "Add TrueNAS SCALE" });
  await expect(dialog).toBeVisible();
  return dialog;
};

test.describe("TrueNAS connections in the consolidated workspace", () => {
  test.setTimeout(180_000);

  test("lists TrueNAS connections in the Connected systems table", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only connections workspace coverage",
    );

    await page.route("**/api/connections", async (route) => {
      if (route.request().method() !== "GET") {
        await route.continue();
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(buildTrueNASConnectionRows()),
      });
    });

    await page.goto("/settings/infrastructure", {
      waitUntil: "domcontentloaded",
    });
    await expect(
      page.getByRole("heading", { level: 1, name: "Infrastructure" }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { level: 2, name: "Connected systems" }),
    ).toBeVisible();

    const towerRow = page.locator("tr").filter({ hasText: "Tower NAS" }).first();
    await expect(towerRow).toBeVisible();
    await expect(towerRow).toContainText("tower.local");

    const vaultRow = page.locator("tr").filter({ hasText: "Backup Vault" }).first();
    await expect(vaultRow).toBeVisible();
    await expect(vaultRow).toContainText("vault.local");

    await expect(
      towerRow.getByRole("button", { name: "Manage" }),
    ).toBeVisible();
  });

  test("adds a TrueNAS connection with draft test and impact preview", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only connections workspace coverage",
    );

    let draftTestPayload: Record<string, unknown> | null = null;
    let createPayload: Record<string, unknown> | null = null;

    await page.route("**/api/truenas/connections**", async (route) => {
      const request = route.request();
      const method = request.method();
      const pathname = new URL(request.url()).pathname;

      if (pathname === "/api/truenas/connections" && method === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify([]),
        });
        return;
      }

      if (pathname === "/api/truenas/connections/test" && method === "POST") {
        draftTestPayload = JSON.parse(request.postData() || "{}");
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ success: true }),
        });
        return;
      }

      if (pathname === "/api/truenas/connections/preview" && method === "POST") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(buildSafeTrueNASAdmissionPreview()),
        });
        return;
      }

      if (pathname === "/api/truenas/connections" && method === "POST") {
        createPayload = JSON.parse(request.postData() || "{}");
        await route.fulfill({
          status: 201,
          contentType: "application/json",
          body: JSON.stringify({
            id: "conn-1",
            name: createPayload.name,
            host: createPayload.host,
            port: 443,
            apiKey: "********",
            useHttps: true,
            insecureSkipVerify: false,
            enabled: true,
            pollIntervalSeconds: 60,
          }),
        });
        return;
      }

      await route.continue();
    });

    const dialog = await openAddTrueNASDialog(page);

    await dialog.getByRole("textbox", { name: "Name" }).fill("Tower NAS");
    await dialog.getByRole("textbox", { name: "Host" }).fill("tower.local");
    await dialog.getByRole("textbox", { name: "API key" }).fill("secret-api-key");

    await dialog.getByRole("button", { name: "Test connection" }).click();
    await expect.poll(() => draftTestPayload).not.toBeNull();
    expect(draftTestPayload).toMatchObject({
      host: "tower.local",
      apiKey: "secret-api-key",
    });

    await dialog.getByRole("button", { name: "Preview impact" }).click();
    await expect(
      dialog.getByText("This change keeps monitored-system count unchanged"),
    ).toBeVisible();
    await expect(
      dialog.getByText(/Pulse currently counts 1 monitored system/),
    ).toBeVisible();

    await dialog.getByRole("button", { name: "Add connection" }).click();
    await expect.poll(() => createPayload).not.toBeNull();
    expect(createPayload).toMatchObject({
      name: "Tower NAS",
      host: "tower.local",
      apiKey: "secret-api-key",
      useHttps: true,
      enabled: true,
    });
    await expect(dialog).not.toBeVisible();
  });

  test("surfaces the canonical monitored-system denial when a TrueNAS save is rejected", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only connections workspace coverage",
    );

    await page.route("**/api/truenas/connections**", async (route) => {
      const request = route.request();
      const method = request.method();
      const pathname = new URL(request.url()).pathname;

      if (pathname === "/api/truenas/connections" && method === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify([]),
        });
        return;
      }

      if (pathname === "/api/truenas/connections/preview" && method === "POST") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(buildSafeTrueNASAdmissionPreview()),
        });
        return;
      }

      if (pathname === "/api/truenas/connections" && method === "POST") {
        await route.fulfill({
          status: 402,
          contentType: "application/json",
          body: JSON.stringify({
            error: "license_required",
            message: "Monitored-system capacity reached (10/9)",
            feature: "max_monitored_systems",
            monitored_system_preview: buildExceededTrueNASAdmissionPreview(),
          }),
        });
        return;
      }

      await route.continue();
    });

    const dialog = await openAddTrueNASDialog(page);

    await dialog.getByRole("textbox", { name: "Name" }).fill("Tower NAS");
    await dialog.getByRole("textbox", { name: "Host" }).fill("tower.local");
    await dialog.getByRole("textbox", { name: "API key" }).fill("secret-api-key");

    await dialog.getByRole("button", { name: "Add connection" }).click();

    // The rejection surfaces the server's monitored-system explanation as an
    // alert while the dialog stays open for the user to adjust or cancel.
    await expect(
      page
        .getByRole("alert")
        .getByRole("heading", { name: "Monitored-system capacity reached (10/9)" }),
    ).toBeVisible();
    await expect(dialog).toBeVisible();
  });
});
