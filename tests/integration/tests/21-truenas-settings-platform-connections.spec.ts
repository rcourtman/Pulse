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
  "truenas-settings-platform-connections.png",
);
const OPERATIONS_SCREENSHOT_PATH = path.resolve(
  __dirname,
  "..",
  "..",
  "tmp",
  "truenas-settings-operations-summary.png",
);
const WORKFLOW_SCREENSHOT_PATH = path.resolve(
  __dirname,
  "..",
  "..",
  "tmp",
  "truenas-settings-platform-workflow.png",
);
const HEALTH_REFRESH_SCREENSHOT_PATH = path.resolve(
  __dirname,
  "..",
  "..",
  "tmp",
  "truenas-settings-health-refresh.png",
);

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
        `truenas-settings-platform-connections-${workerInfo.project.name}.json`,
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

test.describe("TrueNAS platform connections settings", () => {
  test.setTimeout(180_000);

  test("renders the platform-connections workspace with the TrueNAS integration shell", async ({
    page,
  }) => {
    const healthyAt = new Date(Date.now() - 5 * 60_000).toISOString();
    const failingAt = new Date(Date.now() - 2 * 60_000).toISOString();

    await page.route("**/api/truenas/connections", async (route) => {
      if (route.request().method() !== "GET") {
        await route.continue();
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: "truenas-1",
            name: "Tower NAS",
            host: "tower.local",
            port: 443,
            apiKey: "********",
            useHttps: true,
            insecureSkipVerify: false,
            fingerprint: "",
            enabled: true,
            pollIntervalSeconds: 60,
            poll: {
              intervalSeconds: 60,
              lastSuccessAt: healthyAt,
            },
            observed: {
              host: "tower",
              resourceId: "tower",
              collectedAt: healthyAt,
              systems: 1,
              storagePools: 2,
              datasets: 12,
              apps: 4,
              disks: 8,
              recoveryArtifacts: 18,
            },
          },
          {
            id: "truenas-2",
            name: "Backup Vault",
            host: "vault.local",
            port: 443,
            username: "admin",
            password: "********",
            useHttps: true,
            insecureSkipVerify: true,
            fingerprint: "sha256:example",
            enabled: false,
            pollIntervalSeconds: 300,
            poll: {
              intervalSeconds: 300,
              lastAttemptAt: failingAt,
              consecutiveFailures: 2,
              lastError: {
                at: failingAt,
                message: "authentication failed",
                category: "auth",
              },
            },
            observed: {
              host: "vault",
              resourceId: "vault",
              collectedAt: healthyAt,
              systems: 1,
              storagePools: 1,
              datasets: 6,
              apps: 0,
              disks: 12,
              recoveryArtifacts: 24,
            },
          },
        ]),
      });
    });

    await page.goto("/settings/infrastructure/platforms/truenas", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure\/platforms\/truenas/, {
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
    await expect(page.getByRole("tab", { name: "TrueNAS" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await expect(page.getByRole("tab", { name: "Proxmox" })).toHaveAttribute(
      "aria-selected",
      "false",
    );

    await expect(page.getByText("TrueNAS platform integration")).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Add TrueNAS connection" }),
    ).toBeVisible();
    await expect(page.getByText("Tower NAS")).toBeVisible();
    await expect(page.getByText("Backup Vault")).toBeVisible();
    await expect(page.getByText("API key auth")).toBeVisible();
    await expect(page.getByText("Username/password auth")).toBeVisible();
    await expect(page.getByText("Healthy")).toBeVisible();
    await expect(page.getByText("Paused", { exact: true })).toBeVisible();
    await expect(page.getByText("Poll every 1 minute")).toBeVisible();
    await expect(page.getByText("Poll every 5 minutes")).toBeVisible();
    await expect(page.getByText("2 pools")).toBeVisible();
    await expect(page.getByText("12 datasets")).toBeVisible();
    await expect(
      page.getByTestId("truenas-connection-truenas-1-infrastructure"),
    ).toHaveAttribute("href", "/infrastructure?source=truenas&resource=tower");
    await expect(
      page.getByTestId("truenas-connection-truenas-1-workloads"),
    ).toHaveAttribute(
      "href",
      "/workloads?type=app-container&platform=truenas&agent=tower",
    );
    await expect(
      page.getByTestId("truenas-connection-truenas-1-storage"),
    ).toHaveAttribute("href", "/storage?source=truenas&node=tower");
    await expect(
      page.getByTestId("truenas-connection-truenas-1-recovery"),
    ).toHaveAttribute("href", "/recovery?platform=truenas&node=tower");

    fs.mkdirSync(path.dirname(SCREENSHOT_PATH), { recursive: true });
    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });

  test("adds, edits, retests, and deletes TrueNAS connections through the canonical settings workflow", async ({
    page,
  }) => {
    const syncedAt = new Date(Date.now() - 60_000).toISOString();
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

    await page.route("**/api/truenas/connections**", async (route) => {
      const request = route.request();
      const method = request.method();
      const pathname = new URL(request.url()).pathname;

      if (pathname === "/api/truenas/connections" && method === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(connections),
        });
        return;
      }

      if (pathname === "/api/truenas/connections/test" && method === "POST") {
        draftTestCalls += 1;
        draftTestPayload = JSON.parse(request.postData() || "{}");
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ success: true }),
        });
        return;
      }

      if (
        pathname === "/api/truenas/connections/preview" &&
        method === "POST"
      ) {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(buildSafeTrueNASAdmissionPreview()),
        });
        return;
      }

      if (pathname === "/api/truenas/connections" && method === "POST") {
        createPayload = JSON.parse(request.postData() || "{}");
        connections = [
          {
            id: "conn-1",
            name: createPayload.name,
            host: createPayload.host,
            port: createPayload.port,
            apiKey: "********",
            useHttps: createPayload.useHttps,
            insecureSkipVerify: createPayload.insecureSkipVerify,
            fingerprint: createPayload.fingerprint,
            enabled: createPayload.enabled,
            pollIntervalSeconds: createPayload.pollIntervalSeconds,
            poll: {
              intervalSeconds: createPayload.pollIntervalSeconds,
              lastSuccessAt: syncedAt,
            },
            observed: {
              host: "tower",
              resourceId: "tower",
              collectedAt: syncedAt,
              systems: 1,
              storagePools: 2,
              datasets: 12,
              apps: 4,
              disks: 8,
              recoveryArtifacts: 18,
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
        pathname === "/api/truenas/connections/conn-1/test" &&
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

      if (
        pathname === "/api/truenas/connections/conn-1/preview" &&
        method === "POST"
      ) {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(buildSafeTrueNASAdmissionPreview()),
        });
        return;
      }

      if (pathname === "/api/truenas/connections/conn-1" && method === "PUT") {
        updatePayload = JSON.parse(request.postData() || "{}");
        connections = [
          {
            id: "conn-1",
            name: updatePayload.name,
            host: updatePayload.host,
            port: updatePayload.port,
            apiKey: "********",
            useHttps: updatePayload.useHttps,
            insecureSkipVerify: updatePayload.insecureSkipVerify,
            fingerprint: updatePayload.fingerprint,
            enabled: updatePayload.enabled,
            pollIntervalSeconds: updatePayload.pollIntervalSeconds,
            poll: {
              intervalSeconds: updatePayload.pollIntervalSeconds,
              lastSuccessAt: syncedAt,
            },
            observed: {
              host: "tower",
              resourceId: "tower",
              collectedAt: syncedAt,
              systems: 1,
              storagePools: 2,
              datasets: 12,
              apps: 4,
              disks: 8,
              recoveryArtifacts: 18,
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
        pathname === "/api/truenas/connections/conn-1" &&
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

    await page.goto("/settings/infrastructure/platforms/truenas", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure\/platforms\/truenas/, {
      timeout: 15_000,
    });

    await expect(page.getByText("No TrueNAS connections yet")).toBeVisible();

    await page.getByRole("button", { name: "Add TrueNAS connection" }).click();
    const dialog = page.getByRole("dialog", { name: "Add TrueNAS connection" });
    await expect(dialog).toBeVisible();

    await dialog.getByPlaceholder("tower").fill("Tower NAS");
    await dialog.getByPlaceholder("truenas.local").fill("tower.local");
    await dialog.getByPlaceholder("443").fill("443");
    await dialog.getByPlaceholder("60").fill("90");
    await dialog
      .locator('input[type="password"]')
      .first()
      .fill("secret-api-key");

    await dialog.getByRole("button", { name: "Test connection" }).click();
    await expect
      .poll(() => draftTestPayload)
      .toMatchObject({
        name: "Tower NAS",
        host: "tower.local",
        port: 443,
        apiKey: "secret-api-key",
        useHttps: true,
        enabled: true,
        pollIntervalSeconds: 90,
      });

    await expect(
      dialog.getByText("Preview monitored-system impact before saving"),
    ).toBeVisible();
    await expect(
      dialog.getByRole("button", { name: "Add connection" }),
    ).toBeDisabled();
    await dialog.getByRole("button", { name: "Preview impact" }).click();
    await expect(dialog.getByText(/Current usage 1 \/ 10/)).toBeVisible();
    await expect(
      dialog.getByRole("button", { name: "Add connection" }),
    ).toBeEnabled();
    await dialog.getByRole("button", { name: "Add connection" }).click();

    const connectionCard = page.getByTestId("truenas-connection-conn-1");
    await expect(connectionCard).toBeVisible();
    await expect(connectionCard).toContainText("Tower NAS");
    await expect(connectionCard).toContainText("Healthy");
    await expect
      .poll(() => createPayload)
      .toMatchObject({
        name: "Tower NAS",
        host: "tower.local",
        port: 443,
        apiKey: "secret-api-key",
        useHttps: true,
        enabled: true,
        pollIntervalSeconds: 90,
      });

    await connectionCard.getByRole("button", { name: "Edit" }).click();
    const editDialog = page.getByRole("dialog", {
      name: "Edit TrueNAS connection",
    });
    await expect(editDialog).toBeVisible();
    await expect(
      editDialog.getByPlaceholder("Saved API key retained unless replaced"),
    ).toBeVisible();

    await editDialog.getByPlaceholder("tower").fill("Tower NAS Edited");
    await editDialog
      .getByPlaceholder("truenas.local")
      .fill("tower-edited.local");
    await editDialog.getByPlaceholder("60").fill("120");

    await editDialog.getByRole("button", { name: "Test connection" }).click();
    await expect
      .poll(() => savedTestRequests[0])
      .toMatchObject({
        path: "/api/truenas/connections/conn-1/test",
        payload: {
          name: "Tower NAS Edited",
          host: "tower-edited.local",
          port: 443,
          apiKey: "********",
          useHttps: true,
          enabled: true,
          pollIntervalSeconds: 120,
        },
      });
    await expect.poll(() => draftTestCalls).toBe(1);

    await expect(
      editDialog.getByRole("button", { name: "Save connection" }),
    ).toBeDisabled();
    await editDialog.getByRole("button", { name: "Preview impact" }).click();
    await expect(editDialog.getByText(/Current usage 1 \/ 10/)).toBeVisible();
    await expect(
      editDialog.getByRole("button", { name: "Save connection" }),
    ).toBeEnabled();
    await editDialog.getByRole("button", { name: "Save connection" }).click();
    await expect
      .poll(() => updatePayload)
      .toMatchObject({
        name: "Tower NAS Edited",
        host: "tower-edited.local",
        port: 443,
        apiKey: "********",
        useHttps: true,
        enabled: true,
        pollIntervalSeconds: 120,
      });
    await expect(connectionCard).toContainText("Tower NAS Edited");

    await connectionCard.getByRole("button", { name: "Test" }).click();
    await expect
      .poll(() => savedTestRequests)
      .toEqual([
        expect.objectContaining({
          path: "/api/truenas/connections/conn-1/test",
          payload: expect.objectContaining({
            host: "tower-edited.local",
            apiKey: "********",
            pollIntervalSeconds: 120,
          }),
        }),
        { path: "/api/truenas/connections/conn-1/test", payload: null },
      ]);
    await expect.poll(() => draftTestCalls).toBe(1);

    await connectionCard.getByRole("button", { name: "Delete" }).click();
    await page.getByRole("button", { name: "Delete connection" }).click();
    await expect
      .poll(() => deletePaths)
      .toEqual(["/api/truenas/connections/conn-1"]);
    await expect(page.getByText("No TrueNAS connections yet")).toBeVisible();

    fs.mkdirSync(path.dirname(WORKFLOW_SCREENSHOT_PATH), { recursive: true });
    await page.screenshot({ path: WORKFLOW_SCREENSHOT_PATH, fullPage: true });
  });

  test("previews monitored-system impact before creating a TrueNAS connection", async ({
    page,
  }) => {
    let previewPayload: Record<string, unknown> | null = null;
    let createCalls = 0;

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

      if (
        pathname === "/api/truenas/connections/preview" &&
        method === "POST"
      ) {
        previewPayload = JSON.parse(request.postData() || "{}");
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            current_count: 9,
            projected_count: 10,
            additional_count: 1,
            limit: 9,
            would_exceed_limit: true,
            effect: "creates_new",
            current_systems: [
              {
                name: "tower-agent",
                type: "agent",
                status: "online",
                status_explanation: { summary: "", reasons: [] },
                latest_included_signal: {
                  name: "tower-agent",
                  type: "agent",
                  source: "agent",
                  at: new Date(Date.now() - 60_000).toISOString(),
                },
                source: "agent",
                explanation: { summary: "", reasons: [], surfaces: [] },
              },
            ],
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
          }),
        });
        return;
      }

      if (pathname === "/api/truenas/connections" && method === "POST") {
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

    await page.goto("/settings/infrastructure/platforms/truenas", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure\/platforms\/truenas/, {
      timeout: 15_000,
    });

    await page.getByRole("button", { name: "Add TrueNAS connection" }).click();
    const dialog = page.getByRole("dialog", { name: "Add TrueNAS connection" });
    await expect(dialog).toBeVisible();

    await dialog.getByPlaceholder("tower").fill("Tower NAS");
    await dialog.getByPlaceholder("truenas.local").fill("tower.local");
    await dialog.getByPlaceholder("443").fill("443");
    await dialog
      .locator('input[type="password"]')
      .first()
      .fill("secret-api-key");

    await dialog.getByRole("button", { name: "Preview impact" }).click();

    await expect
      .poll(() => previewPayload)
      .toMatchObject({
        name: "Tower NAS",
        host: "tower.local",
        port: 443,
        apiKey: "secret-api-key",
        useHttps: true,
        enabled: true,
      });
    await expect(
      dialog.getByText("This change exceeds your monitored-system limit"),
    ).toBeVisible();
    await expect(dialog.getByText(/Current usage 9 \/ 9/)).toBeVisible();
    await expect(dialog.getByText("Current matched systems")).toBeVisible();
    await expect(
      dialog.getByText("tower-agent (Host via Agent)"),
    ).toBeVisible();
    await expect(dialog.getByText("Projected systems")).toBeVisible();
    await expect(
      dialog.getByText("tower (TrueNAS System via TrueNAS)"),
    ).toBeVisible();
    await expect(
      dialog.getByRole("button", { name: "Add connection" }),
    ).toBeDisabled();
    expect(createCalls).toBe(0);
  });

  test("shows no monitored-system increase when adding a disabled TrueNAS connection", async ({
    page,
  }) => {
    let previewPayload: Record<string, unknown> | null = null;

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

      if (
        pathname === "/api/truenas/connections/preview" &&
        method === "POST"
      ) {
        previewPayload = JSON.parse(request.postData() || "{}");
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            current_count: 3,
            projected_count: 3,
            additional_count: 0,
            limit: 10,
            would_exceed_limit: false,
            effect: "no_change",
            current_systems: [],
            projected_systems: [],
            current_system: null,
            projected_system: null,
          }),
        });
        return;
      }

      await route.continue();
    });

    await page.goto("/settings/infrastructure/platforms/truenas", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure\/platforms\/truenas/, {
      timeout: 15_000,
    });

    await page.getByRole("button", { name: "Add TrueNAS connection" }).click();
    const dialog = page.getByRole("dialog", { name: "Add TrueNAS connection" });
    await expect(dialog).toBeVisible();

    await dialog.getByPlaceholder("tower").fill("Archive NAS");
    await dialog.getByPlaceholder("truenas.local").fill("archive.local");
    await dialog
      .locator('input[type="password"]')
      .first()
      .fill("secret-api-key");
    await dialog.getByLabel("Enable polling immediately").uncheck();

    await expect(
      dialog.getByRole("button", { name: "Add connection" }),
    ).toBeDisabled();
    await dialog.getByRole("button", { name: "Preview impact" }).click();

    await expect
      .poll(() => previewPayload)
      .toMatchObject({
        name: "Archive NAS",
        host: "archive.local",
        apiKey: "secret-api-key",
        useHttps: true,
        enabled: false,
      });
    await expect(
      dialog.getByText(
        "This change reuses your current monitored-system capacity",
      ),
    ).toBeVisible();
    await expect(
      dialog.getByText(
        "Current usage 3 / 10. Saving this change keeps usage at 3 / 10.",
      ),
    ).toBeVisible();
    await expect(
      dialog.getByRole("button", { name: "Add connection" }),
    ).toBeEnabled();
  });

  test("reuses the canonical monitored-system explanation when a TrueNAS save is denied", async ({
    page,
  }) => {
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

      if (
        pathname === "/api/truenas/connections/preview" &&
        method === "POST"
      ) {
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
          status: 402,
          contentType: "application/json",
          body: JSON.stringify({
            error: "license_required",
            message: "Monitored-system limit reached (10/9)",
            feature: "max_monitored_systems",
            monitored_system_preview: {
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
            },
          }),
        });
        return;
      }

      await route.continue();
    });

    await page.goto("/settings/infrastructure/platforms/truenas", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure\/platforms\/truenas/, {
      timeout: 15_000,
    });

    await page.getByRole("button", { name: "Add TrueNAS connection" }).click();
    const dialog = page.getByRole("dialog", { name: "Add TrueNAS connection" });
    await expect(dialog).toBeVisible();

    await dialog.getByPlaceholder("tower").fill("Tower NAS");
    await dialog.getByPlaceholder("truenas.local").fill("tower.local");
    await dialog
      .locator('input[type="password"]')
      .first()
      .fill("secret-api-key");
    await expect(
      dialog.getByRole("button", { name: "Add connection" }),
    ).toBeDisabled();
    await dialog.getByRole("button", { name: "Preview impact" }).click();
    await expect(dialog.getByText(/Current usage 1 \/ 10/)).toBeVisible();
    await expect(
      dialog.getByRole("button", { name: "Add connection" }),
    ).toBeEnabled();
    await dialog.getByRole("button", { name: "Add connection" }).click();

    await expect
      .poll(() => createPayload)
      .toMatchObject({
        name: "Tower NAS",
        host: "tower.local",
        apiKey: "secret-api-key",
        useHttps: true,
        enabled: true,
      });
    await expect(
      dialog.getByText("This change exceeds your monitored-system limit"),
    ).toBeVisible();
    await expect(dialog.getByText(/Current usage 9 \/ 9/)).toBeVisible();
    await expect(dialog.getByText("Projected systems")).toBeVisible();
    await expect(
      dialog.getByText("tower (TrueNAS System via TrueNAS)"),
    ).toBeVisible();
    await expect(
      dialog.getByRole("button", { name: "Add connection" }),
    ).toBeDisabled();
  });

  test("blocks TrueNAS save while monitored-system usage is still settling", async ({
    page,
  }) => {
    let previewPayload: Record<string, unknown> | null = null;
    let createCalls = 0;

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

      if (
        pathname === "/api/truenas/connections/preview" &&
        method === "POST"
      ) {
        previewPayload = JSON.parse(request.postData() || "{}");
        await route.fulfill({
          status: 503,
          contentType: "application/json",
          body: JSON.stringify({
            error: "Unable to verify monitored-system capacity right now",
            code: "monitored_system_usage_unavailable",
            status_code: 503,
            details: {
              reason: "supplemental_inventory_unsettled",
            },
          }),
        });
        return;
      }

      if (pathname === "/api/truenas/connections" && method === "POST") {
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

    await page.goto("/settings/infrastructure/platforms/truenas", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure\/platforms\/truenas/, {
      timeout: 15_000,
    });

    await page.getByRole("button", { name: "Add TrueNAS connection" }).click();
    const dialog = page.getByRole("dialog", { name: "Add TrueNAS connection" });
    await expect(dialog).toBeVisible();

    await dialog.getByPlaceholder("tower").fill("Tower NAS");
    await dialog.getByPlaceholder("truenas.local").fill("tower.local");
    await dialog
      .locator('input[type="password"]')
      .first()
      .fill("secret-api-key");

    await dialog.getByRole("button", { name: "Preview impact" }).click();

    await expect
      .poll(() => previewPayload)
      .toMatchObject({
        name: "Tower NAS",
        host: "tower.local",
        apiKey: "secret-api-key",
        useHttps: true,
        enabled: true,
      });
    await expect(
      dialog.getByText("Monitored-system capacity is temporarily unavailable"),
    ).toBeVisible();
    await expect(
      dialog.getByText(
        "Pulse is still settling provider-owned inventory for this platform connection, so the monitored-system check is not safe yet. Retry preview after the first baseline finishes.",
      ),
    ).toBeVisible();
    await expect(
      dialog.getByRole("button", { name: "Add connection" }),
    ).toBeDisabled();
    expect(createCalls).toBe(0);
  });

  test("saved connection tests refresh runtime health in the settings card", async ({
    page,
  }) => {
    const failedAt = new Date(Date.now() - 5 * 60_000).toISOString();
    const recoveredAt = new Date().toISOString();
    let savedTestCalls = 0;
    let connection = {
      id: "conn-1",
      name: "Tower NAS",
      host: "tower.local",
      port: 443,
      apiKey: "********",
      useHttps: true,
      insecureSkipVerify: false,
      enabled: true,
      pollIntervalSeconds: 60,
      poll: {
        intervalSeconds: 60,
        lastAttemptAt: failedAt,
        consecutiveFailures: 2,
        lastError: {
          at: failedAt,
          message: "authentication failed",
          category: "auth",
        },
      },
      observed: {
        host: "tower",
        resourceId: "tower",
        collectedAt: failedAt,
        systems: 1,
        storagePools: 2,
        datasets: 12,
        apps: 4,
        disks: 8,
        recoveryArtifacts: 18,
      },
    };

    await page.route("**/api/truenas/connections**", async (route) => {
      const request = route.request();
      const method = request.method();
      const pathname = new URL(request.url()).pathname;

      if (pathname === "/api/truenas/connections" && method === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify([connection]),
        });
        return;
      }

      if (
        pathname === "/api/truenas/connections/conn-1/test" &&
        method === "POST"
      ) {
        savedTestCalls += 1;
        connection = {
          ...connection,
          poll: {
            intervalSeconds: 60,
            lastSuccessAt: recoveredAt,
          },
        };
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ success: true }),
        });
        return;
      }

      await route.continue();
    });

    await page.goto("/settings/infrastructure/platforms/truenas", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure\/platforms\/truenas/, {
      timeout: 15_000,
    });

    const connectionCard = page.getByTestId("truenas-connection-conn-1");
    await expect(connectionCard).toContainText("Sync failing");
    await expect(connectionCard).toContainText("authentication failed");

    await connectionCard.getByRole("button", { name: "Test" }).click();

    await expect.poll(() => savedTestCalls).toBe(1);
    await expect(connectionCard).toContainText("Healthy");
    await expect(connectionCard).not.toContainText("authentication failed");

    fs.mkdirSync(path.dirname(HEALTH_REFRESH_SCREENSHOT_PATH), {
      recursive: true,
    });
    await page.screenshot({
      path: HEALTH_REFRESH_SCREENSHOT_PATH,
      fullPage: true,
    });
  });

  test("counts TrueNAS alongside the other platform connections on the operations summary", async ({
    page,
  }) => {
    await page.route("**/api/config/nodes", async (route) => {
      if (route.request().method() !== "GET") {
        await route.continue();
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: "pve-1",
            type: "pve",
            name: "pve-main",
            host: "pve-main.local",
            user: "root@pam",
            verifySSL: true,
            monitorVMs: true,
            monitorContainers: true,
            monitorStorage: true,
            monitorBackups: true,
            monitorPhysicalDisks: true,
            status: "connected",
          },
          {
            id: "pbs-1",
            type: "pbs",
            name: "pbs-main",
            host: "pbs-main.local",
            user: "root@pam",
            verifySSL: true,
            monitorDatastores: true,
            monitorSyncJobs: true,
            monitorVerifyJobs: true,
            monitorPruneJobs: true,
            monitorGarbageJobs: true,
            status: "connected",
          },
          {
            id: "pbs-2",
            type: "pbs",
            name: "pbs-vault",
            host: "pbs-vault.local",
            user: "root@pam",
            verifySSL: true,
            monitorDatastores: true,
            monitorSyncJobs: true,
            monitorVerifyJobs: true,
            monitorPruneJobs: true,
            monitorGarbageJobs: true,
            status: "connected",
          },
          {
            id: "pmg-1",
            type: "pmg",
            name: "pmg-main",
            host: "pmg-main.local",
            user: "root@pam",
            verifySSL: true,
            monitorMailStats: true,
            monitorQueues: true,
            monitorQuarantine: true,
            monitorDomainStats: true,
            status: "connected",
          },
        ]),
      });
    });

    await page.route("**/api/truenas/connections", async (route) => {
      if (route.request().method() !== "GET") {
        await route.continue();
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: "truenas-1",
            name: "Tower NAS",
            host: "tower.local",
            port: 443,
            apiKey: "********",
            useHttps: true,
            insecureSkipVerify: false,
            enabled: true,
          },
          {
            id: "truenas-2",
            name: "Backup Vault",
            host: "vault.local",
            port: 443,
            apiKey: "********",
            useHttps: true,
            insecureSkipVerify: false,
            enabled: true,
          },
        ]),
      });
    });

    await page.goto("/settings/infrastructure/operations", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure\/operations/, {
      timeout: 15_000,
    });

    await expect(
      page.getByRole("heading", {
        level: 1,
        name: "Infrastructure Operations",
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { level: 3, name: "Platform connections" }),
    ).toBeVisible();
    await expect(page.getByTestId("platform-connections-pve")).toContainText(
      "1",
    );
    await expect(page.getByTestId("platform-connections-pbs")).toContainText(
      "2",
    );
    await expect(page.getByTestId("platform-connections-pmg")).toContainText(
      "1",
    );
    await expect(
      page.getByTestId("platform-connections-truenas"),
    ).toContainText("2");
    await expect(
      page.getByTestId("platform-connections-truenas"),
    ).toContainText("API-backed NAS connections");

    fs.mkdirSync(path.dirname(OPERATIONS_SCREENSHOT_PATH), { recursive: true });
    await page.screenshot({ path: OPERATIONS_SCREENSHOT_PATH, fullPage: true });
  });

  test("treats disabled TrueNAS as an explicit opt-out instead of a setup prerequisite", async ({
    page,
  }) => {
    await page.route("**/api/truenas/connections", async (route) => {
      if (route.request().method() !== "GET") {
        await route.continue();
        return;
      }

      await route.fulfill({
        status: 404,
        contentType: "application/json",
        body: JSON.stringify({
          error: "truenas_disabled",
          message: "TrueNAS integration has been explicitly disabled",
        }),
      });
    });

    await page.goto("/settings/infrastructure/platforms/truenas", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure\/platforms\/truenas/, {
      timeout: 15_000,
    });

    await expect(
      page.getByText("TrueNAS integration is disabled"),
    ).toBeVisible();
    await expect(
      page.getByText("TrueNAS integration has been explicitly disabled"),
    ).toBeVisible();
    await expect(page.getByText(/PULSE_ENABLE_TRUENAS=false/)).toBeVisible();
    await expect(page.getByText(/set it back to true/i)).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Add TrueNAS connection" }),
    ).not.toBeVisible();
    await expect(
      page.locator('[data-testid^=\"truenas-connection-\"]'),
    ).toHaveCount(0);
  });
});
