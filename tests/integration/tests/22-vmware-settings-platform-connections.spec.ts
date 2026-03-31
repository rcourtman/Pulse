import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test as base, expect } from "@playwright/test";
import { createAuthenticatedStorageState } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const SCREENSHOT_PATH = path.resolve(
  __dirname,
  "..",
  "..",
  "tmp",
  "vmware-settings-platform-connections.png",
);
const WORKFLOW_SCREENSHOT_PATH = path.resolve(
  __dirname,
  "..",
  "..",
  "tmp",
  "vmware-settings-platform-workflow.png",
);

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
        `vmware-settings-platform-connections-${workerInfo.project.name}.json`,
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

test.describe("VMware platform connections settings", () => {
  test.setTimeout(180_000);

  test("renders the platform-connections workspace with the VMware integration shell", async ({
    page,
  }) => {
    const healthyAt = new Date(Date.now() - 5 * 60_000).toISOString();
    const failingAt = new Date(Date.now() - 2 * 60_000).toISOString();

    await page.route("**/api/vmware/connections", async (route) => {
      if (route.request().method() !== "GET") {
        await route.continue();
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: "vmware-1",
            name: "Primary vCenter",
            host: "vcsa-primary.local",
            port: 443,
            username: "administrator@vsphere.local",
            password: "********",
            insecureSkipVerify: false,
            enabled: true,
            poll: {
              lastSuccessAt: healthyAt,
            },
            observed: {
              collectedAt: healthyAt,
              hosts: 6,
              vms: 120,
              datastores: 18,
              viRelease: "8.0.3",
            },
          },
          {
            id: "vmware-2",
            name: "Staging vCenter",
            host: "vcsa-staging.local",
            port: 443,
            username: "operator@vsphere.local",
            password: "********",
            insecureSkipVerify: true,
            enabled: true,
            poll: {
              lastAttemptAt: failingAt,
              lastError: {
                at: failingAt,
                message: "authentication failed",
                category: "auth",
              },
            },
          },
          {
            id: "vmware-3",
            name: "Partial vCenter",
            host: "vcsa-partial.local",
            port: 443,
            username: "readonly@vsphere.local",
            password: "********",
            insecureSkipVerify: false,
            enabled: true,
            poll: {
              lastSuccessAt: healthyAt,
            },
            observed: {
              collectedAt: healthyAt,
              hosts: 2,
              vms: 18,
              datastores: 4,
              viRelease: "8.0.3",
              degraded: true,
              issueCount: 3,
              issues: [
                {
                  stage: "signals",
                  category: "permission",
                  message:
                    "VMware permissions are insufficient for host overall status",
                  occurrences: 2,
                },
              ],
            },
          },
        ]),
      });
    });

    await page.goto("/settings/infrastructure/platforms/vmware", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure\/platforms\/vmware/, {
      timeout: 15_000,
    });

    await expect(
      page.getByRole("heading", {
        level: 1,
        name: "Infrastructure Operations",
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("tab", { name: "Platform connections" }),
    ).toHaveAttribute("aria-selected", "true");
    await expect(page.getByRole("tab", { name: "VMware" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await expect(page.getByRole("tab", { name: "TrueNAS" })).toHaveAttribute(
      "aria-selected",
      "false",
    );
    await expect(page.getByRole("tab", { name: "Proxmox" })).toHaveAttribute(
      "aria-selected",
      "false",
    );

    await expect(
      page.getByText("VMware vSphere platform integration"),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Add VMware connection" }),
    ).toBeVisible();

    const primaryCard = page.getByTestId("vmware-connection-vmware-1");
    await expect(primaryCard).toContainText("Primary vCenter");
    await expect(primaryCard).toContainText("Healthy");
    await expect(primaryCard).toContainText("6 hosts");
    await expect(primaryCard).toContainText("120 vms");
    await expect(primaryCard).toContainText("18 datastores");
    await expect(primaryCard).toContainText("VI JSON 8.0.3");

    const failingCard = page.getByTestId("vmware-connection-vmware-2");
    await expect(failingCard).toContainText("Staging vCenter");
    await expect(failingCard).toContainText("Runtime failing");
    await expect(failingCard).toContainText("authentication failed");
    await expect(failingCard).toContainText("Skip TLS verification");

    const partialCard = page.getByTestId("vmware-connection-vmware-3");
    await expect(partialCard).toContainText("Partial vCenter");
    await expect(partialCard).toContainText("Degraded");
    await expect(partialCard).toContainText("2 hosts");
    await expect(partialCard).toContainText("18 vms");
    await expect(partialCard).toContainText("4 datastores");
    await expect(partialCard).toContainText("3 degraded reads");
    await expect(partialCard).toContainText(
      "VMware permissions are insufficient for host overall status",
    );

    fs.mkdirSync(path.dirname(SCREENSHOT_PATH), { recursive: true });
    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });

  test("surfaces structured draft test guidance for unsupported vCenter versions", async ({
    page,
  }) => {
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

    await page.goto("/settings/infrastructure/platforms/vmware", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure\/platforms\/vmware/, {
      timeout: 15_000,
    });

    await page.getByRole("button", { name: "Add VMware connection" }).click();
    await page.getByLabel("Host").fill("legacy.lab.local");
    await page.getByLabel("Username").fill("administrator@vsphere.local");
    await page.getByLabel("Password").fill("super-secret");
    await page.getByRole("button", { name: "Test connection" }).click();

    const feedback = page.getByTestId("vmware-connection-test-feedback");
    await expect(feedback).toBeVisible();
    await expect(feedback).toContainText("Unsupported vCenter version");
    await expect(feedback).toContainText(
      "VMware vCenter 6.7 is below the supported VI JSON release floor",
    );
    await expect(feedback).toContainText(
      "Use a supported vCenter release within the current VI JSON phase-1 floor, then retry this connection test.",
    );
    await expect(page.getByRole("dialog")).toContainText(
      "Configure the vCenter endpoint Pulse should validate for the VMware platform.",
    );
    expect(createCalls).toBe(0);
  });

  test("surfaces shared draft onboarding guidance for auth, TLS, and network failures", async ({
    page,
  }) => {
    const scenarios = [
      {
        category: "auth",
        error:
          "VMware authentication failed while creating the VI JSON API session",
        expectedTitle: "Authentication failed",
        expectedGuidance:
          "Verify the username, password, and account scope in vCenter before retrying.",
      },
      {
        category: "tls",
        error:
          "VMware TLS validation failed during Automation API session bootstrap",
        expectedTitle: "TLS validation failed",
        expectedGuidance:
          "Install a trusted certificate for vCenter, or enable Skip TLS verification only for controlled lab environments.",
      },
      {
        category: "network",
        error: "VMware network error during VI JSON login",
        expectedTitle: "Pulse could not reach vCenter",
        expectedGuidance:
          "Confirm DNS, reachability, port 443, and any firewall rules from the Pulse server to vCenter.",
      },
    ];
    let draftFailureIndex = 0;

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
        const scenario = scenarios[Math.min(draftFailureIndex, scenarios.length - 1)];
        draftFailureIndex += 1;
        await route.fulfill({
          status: 400,
          contentType: "application/json",
          body: JSON.stringify({
            error: "Failed to connect to VMware vCenter",
            code: "vmware_connection_failed",
            status_code: 400,
            details: {
              category: scenario.category,
              error: scenario.error,
            },
          }),
        });
        return;
      }

      await route.continue();
    });

    await page.goto("/settings/infrastructure/platforms/vmware", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure\/platforms\/vmware/, {
      timeout: 15_000,
    });

    for (const scenario of scenarios) {
      await page.getByRole("button", { name: "Add VMware connection" }).click();
      await page.getByLabel("Host").fill(`${scenario.category}.lab.local`);
      await page.getByLabel("Username").fill("administrator@vsphere.local");
      await page.getByLabel("Password").fill("super-secret");
      await page.getByRole("button", { name: "Test connection" }).click();

      const feedback = page.getByTestId("vmware-connection-test-feedback");
      await expect(feedback).toBeVisible();
      await expect(feedback).toContainText(scenario.expectedTitle);
      await expect(feedback).toContainText(scenario.error);
      await expect(feedback).toContainText(scenario.expectedGuidance);

      await page.getByRole("button", { name: "Cancel" }).click();
      await expect(page.getByRole("dialog")).not.toBeVisible();
    }
  });

  test("adds, edits, retests, and deletes VMware connections through the canonical settings workflow", async ({
    page,
  }) => {
    const validatedAt = new Date(Date.now() - 60_000).toISOString();
    let connections: Record<string, unknown>[] = [];
    let draftTestPayload: Record<string, unknown> | null = null;
    let createPayload: Record<string, unknown> | null = null;
    let updatePayload: Record<string, unknown> | null = null;
    let draftTestCalls = 0;
    const savedTestRequests: Array<{
      path: string;
      payload: Record<string, unknown> | null;
    }> = [];
    const deletePaths: string[] = [];

    await page.route("**/api/vmware/connections**", async (route) => {
      const request = route.request();
      const method = request.method();
      const pathname = new URL(request.url()).pathname;

      if (pathname === "/api/vmware/connections" && method === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(connections),
        });
        return;
      }

      if (pathname === "/api/vmware/connections/test" && method === "POST") {
        draftTestCalls += 1;
        draftTestPayload = JSON.parse(request.postData() || "{}");
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ success: true }),
        });
        return;
      }

      if (pathname === "/api/vmware/connections" && method === "POST") {
        createPayload = JSON.parse(request.postData() || "{}");
        connections = [
          {
            id: "conn-1",
            name: createPayload.name,
            host: createPayload.host,
            port: createPayload.port,
            username: createPayload.username,
            password: "********",
            insecureSkipVerify: createPayload.insecureSkipVerify,
            enabled: createPayload.enabled,
            poll: {
              lastSuccessAt: validatedAt,
            },
            observed: {
              collectedAt: validatedAt,
              hosts: 4,
              vms: 48,
              datastores: 8,
              viRelease: "8.0.3",
            },
          },
        ];
        await route.fulfill({
          status: 201,
          contentType: "application/json",
          body: JSON.stringify(connections[0]),
        });
        return;
      }

      if (
        pathname === "/api/vmware/connections/conn-1/test" &&
        method === "POST"
      ) {
        savedTestRequests.push({
          path: pathname,
          payload: request.postData()
            ? JSON.parse(request.postData() || "{}")
            : null,
        });
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ success: true }),
        });
        return;
      }

      if (pathname === "/api/vmware/connections/conn-1" && method === "PUT") {
        updatePayload = JSON.parse(request.postData() || "{}");
        connections = [
          {
            id: "conn-1",
            name: updatePayload.name,
            host: updatePayload.host,
            port: updatePayload.port,
            username: updatePayload.username,
            password: "********",
            insecureSkipVerify: updatePayload.insecureSkipVerify,
            enabled: updatePayload.enabled,
            poll: {
              lastSuccessAt: validatedAt,
            },
            observed: {
              collectedAt: validatedAt,
              hosts: 4,
              vms: 48,
              datastores: 8,
              viRelease: "8.0.3",
            },
          },
        ];
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(connections[0]),
        });
        return;
      }

      if (
        pathname === "/api/vmware/connections/conn-1" &&
        method === "DELETE"
      ) {
        deletePaths.push(pathname);
        connections = [];
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ success: true, id: "conn-1" }),
        });
        return;
      }

      await route.continue();
    });

    await page.goto("/settings/infrastructure/platforms/vmware", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure\/platforms\/vmware/, {
      timeout: 15_000,
    });

    await expect(page.getByText("No VMware connections yet")).toBeVisible();

    await page.getByRole("button", { name: "Add VMware connection" }).click();
    const dialog = page.getByRole("dialog", { name: "Add VMware connection" });
    await expect(dialog).toBeVisible();

    await dialog.getByPlaceholder("lab-vcenter").fill("Lab vCenter");
    await dialog.getByPlaceholder("vcsa.lab.local").fill("vcsa.lab.local");
    await dialog.getByPlaceholder("443").fill("443");
    await dialog
      .getByPlaceholder("administrator@vsphere.local")
      .fill("administrator@vsphere.local");
    await dialog.locator('input[type="password"]').first().fill("super-secret");

    await dialog.getByRole("button", { name: "Test connection" }).click();
    await expect
      .poll(() => draftTestPayload)
      .toMatchObject({
        name: "Lab vCenter",
        host: "vcsa.lab.local",
        port: 443,
        username: "administrator@vsphere.local",
        password: "super-secret",
        enabled: true,
        insecureSkipVerify: false,
      });

    await dialog.getByRole("button", { name: "Add connection" }).click();

    const connectionCard = page.getByTestId("vmware-connection-conn-1");
    await expect(connectionCard).toBeVisible();
    await expect(connectionCard).toContainText("Lab vCenter");
    await expect(connectionCard).toContainText("Healthy");
    await expect
      .poll(() => createPayload)
      .toMatchObject({
        name: "Lab vCenter",
        host: "vcsa.lab.local",
        port: 443,
        username: "administrator@vsphere.local",
        password: "super-secret",
        enabled: true,
        insecureSkipVerify: false,
      });

    await connectionCard.getByRole("button", { name: "Edit" }).click();
    const editDialog = page.getByRole("dialog", {
      name: "Edit VMware connection",
    });
    await expect(editDialog).toBeVisible();
    await expect(
      editDialog.getByPlaceholder("Saved password retained unless replaced"),
    ).toBeVisible();

    await editDialog.getByPlaceholder("lab-vcenter").fill("Lab vCenter Edited");
    await editDialog
      .getByPlaceholder("vcsa.lab.local")
      .fill("edited.lab.local");
    await editDialog.getByPlaceholder("443").fill("8443");
    await editDialog
      .getByPlaceholder("administrator@vsphere.local")
      .fill("operator@vsphere.local");
    await editDialog.getByLabel("Skip TLS verification").check();

    await editDialog.getByRole("button", { name: "Test connection" }).click();
    await expect
      .poll(() => savedTestRequests[0])
      .toMatchObject({
        path: "/api/vmware/connections/conn-1/test",
        payload: {
          name: "Lab vCenter Edited",
          host: "edited.lab.local",
          port: 8443,
          username: "operator@vsphere.local",
          password: "********",
          enabled: true,
          insecureSkipVerify: true,
        },
      });
    await expect.poll(() => draftTestCalls).toBe(1);

    await editDialog.getByRole("button", { name: "Save connection" }).click();
    await expect
      .poll(() => updatePayload)
      .toMatchObject({
        name: "Lab vCenter Edited",
        host: "edited.lab.local",
        port: 8443,
        username: "operator@vsphere.local",
        password: "********",
        enabled: true,
        insecureSkipVerify: true,
      });
    await expect(connectionCard).toContainText("Lab vCenter Edited");

    await connectionCard.getByRole("button", { name: "Test" }).click();
    await expect
      .poll(() => savedTestRequests)
      .toEqual([
        expect.objectContaining({
          path: "/api/vmware/connections/conn-1/test",
          payload: expect.objectContaining({
            host: "edited.lab.local",
            password: "********",
            insecureSkipVerify: true,
          }),
        }),
        { path: "/api/vmware/connections/conn-1/test", payload: null },
      ]);
    await expect.poll(() => draftTestCalls).toBe(1);

    await connectionCard.getByRole("button", { name: "Delete" }).click();
    await page.getByRole("button", { name: "Delete connection" }).click();
    await expect
      .poll(() => deletePaths)
      .toEqual(["/api/vmware/connections/conn-1"]);
    await expect(page.getByText("No VMware connections yet")).toBeVisible();

    fs.mkdirSync(path.dirname(WORKFLOW_SCREENSHOT_PATH), { recursive: true });
    await page.screenshot({ path: WORKFLOW_SCREENSHOT_PATH, fullPage: true });
  });
});
