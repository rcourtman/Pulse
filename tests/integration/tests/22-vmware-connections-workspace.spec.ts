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
        `vmware-connections-workspace-${workerInfo.project.name}.json`,
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

const buildSafeVMwareAdmissionPreview = () => ({
  current_count: 1,
  projected_count: 1,
  additional_count: 0,
  limit: 10,
  would_exceed_limit: false,
  effect: "attaches_existing",
  current_systems: [],
  projected_systems: [],
  current_system: null,
  projected_system: null,
});

const openAddVMwareDialog = async (page: Page) => {
  await page.goto("/settings/infrastructure", { waitUntil: "domcontentloaded" });
  await expect(
    page.getByRole("heading", { level: 2, name: "Connected systems" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Add infrastructure" }).first().click();
  const picker = page.getByRole("dialog", { name: "Add infrastructure" });
  await expect(picker).toBeVisible();
  // The VMware tile sits behind the collapsed tail of the source catalog.
  await picker.getByRole("button", { name: /Show \d+ more sources/ }).click();
  await picker.getByRole("button", { name: /VMware vCenter/ }).click();
  const dialog = page.getByRole("dialog", { name: "Add VMware vCenter" });
  await expect(dialog).toBeVisible();
  return dialog;
};

test.describe("VMware connections in the consolidated workspace", () => {
  test.setTimeout(180_000);

  test("adds a vCenter connection with draft test and impact preview", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only connections workspace coverage",
    );

    let draftTestPayload: Record<string, unknown> | null = null;
    let createPayload: Record<string, unknown> | null = null;

    await page.route("**/api/vmware/connections**", async (route) => {
      const request = route.request();
      const method = request.method();
      const pathname = new URL(request.url()).pathname;

      if (pathname === "/api/vmware/connections" && method === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify([]),
        });
        return;
      }

      if (pathname === "/api/vmware/connections/test" && method === "POST") {
        draftTestPayload = JSON.parse(request.postData() || "{}");
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ success: true }),
        });
        return;
      }

      if (pathname === "/api/vmware/connections/preview" && method === "POST") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(buildSafeVMwareAdmissionPreview()),
        });
        return;
      }

      if (pathname === "/api/vmware/connections" && method === "POST") {
        createPayload = JSON.parse(request.postData() || "{}");
        await route.fulfill({
          status: 201,
          contentType: "application/json",
          body: JSON.stringify({
            id: "vc-1",
            name: createPayload.name,
            host: createPayload.host,
            port: 443,
            username: createPayload.username,
            enabled: true,
          }),
        });
        return;
      }

      await route.continue();
    });

    const dialog = await openAddVMwareDialog(page);

    await dialog.getByRole("textbox", { name: "Name", exact: true }).fill("Lab VC");
    await dialog.getByRole("textbox", { name: "Host" }).fill("vcsa.lab.local");
    await dialog
      .getByRole("textbox", { name: "Username" })
      .fill("administrator@vsphere.local");
    await dialog.getByRole("textbox", { name: "Password" }).fill("super-secret");

    await dialog.getByRole("button", { name: "Test connection" }).click();
    await expect.poll(() => draftTestPayload).not.toBeNull();
    expect(draftTestPayload).toMatchObject({
      host: "vcsa.lab.local",
      username: "administrator@vsphere.local",
      password: "super-secret",
    });

    await dialog.getByRole("button", { name: "Preview impact" }).click();
    await expect(
      dialog.getByText("This change keeps monitored-system count unchanged"),
    ).toBeVisible();

    await dialog.getByRole("button", { name: "Add connection" }).click();
    await expect.poll(() => createPayload).not.toBeNull();
    expect(createPayload).toMatchObject({
      name: "Lab VC",
      host: "vcsa.lab.local",
      username: "administrator@vsphere.local",
      password: "super-secret",
    });
    await expect(dialog).not.toBeVisible();
  });

  test("surfaces structured draft test guidance for unsupported vCenter versions", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only connections workspace coverage",
    );

    let createCalls = 0;

    await page.route("**/api/vmware/connections**", async (route) => {
      const request = route.request();
      const method = request.method();
      const pathname = new URL(request.url()).pathname;

      if (pathname === "/api/vmware/connections" && method === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify([]),
        });
        return;
      }

      if (pathname === "/api/vmware/connections/test" && method === "POST") {
        await route.fulfill({
          status: 400,
          contentType: "application/json",
          body: JSON.stringify({
            error: "Failed to connect to VMware vCenter",
            code: "vmware_connection_failed",
            status_code: 400,
            details: {
              category: "unsupported_version",
              error:
                "VMware vCenter 6.7 is below the supported VI JSON release floor",
            },
          }),
        });
        return;
      }

      if (pathname === "/api/vmware/connections" && method === "POST") {
        createCalls += 1;
        await route.fulfill({
          status: 201,
          contentType: "application/json",
          body: JSON.stringify({}),
        });
        return;
      }

      await route.continue();
    });

    const dialog = await openAddVMwareDialog(page);

    await dialog.getByRole("textbox", { name: "Host" }).fill("legacy.lab.local");
    await dialog
      .getByRole("textbox", { name: "Username" })
      .fill("administrator@vsphere.local");
    await dialog.getByRole("textbox", { name: "Password" }).fill("super-secret");
    await dialog.getByRole("button", { name: "Test connection" }).click();

    const feedback = page.getByTestId("vmware-connection-test-feedback");
    await expect(feedback).toBeVisible();
    await expect(feedback).toContainText("Unsupported vCenter version");
    await expect(feedback).toContainText(
      "VMware vCenter 6.7 is below the supported VI JSON release floor",
    );
    expect(createCalls).toBe(0);
  });
});
