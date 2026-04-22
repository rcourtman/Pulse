import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test as base } from "@playwright/test";
import { createAuthenticatedStorageState } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const VMWARE_SCREENSHOT_PATH = path.resolve(
  __dirname,
  "..",
  "..",
  "tmp",
  "vmware-tls-warning-banner.png",
);
const TRUENAS_SCREENSHOT_PATH = path.resolve(
  __dirname,
  "..",
  "..",
  "tmp",
  "truenas-tls-warning-banner.png",
);
const APPRISE_SCREENSHOT_PATH = path.resolve(
  __dirname,
  "..",
  "..",
  "tmp",
  "apprise-tls-warning-banner.png",
);

const buildSafeTrueNASAdmissionPreview = () => ({
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
        `tls-warning-banners-${workerInfo.project.name}.json`,
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

async function mockVMwareConnections(page: import("@playwright/test").Page) {
  await page.route("**/api/vmware/connections**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;

    if (pathname === "/api/vmware/connections" && request.method() === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([]),
      });
      return;
    }

    if (
      pathname === "/api/vmware/connections/preview" &&
      request.method() === "POST"
    ) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(buildSafeVMwareAdmissionPreview()),
      });
      return;
    }

    await route.continue();
  });
}

async function mockTrueNASConnections(page: import("@playwright/test").Page) {
  await page.route("**/api/truenas/connections**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;

    if (pathname === "/api/truenas/connections" && request.method() === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([]),
      });
      return;
    }

    if (
      pathname === "/api/truenas/connections/preview" &&
      request.method() === "POST"
    ) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(buildSafeTrueNASAdmissionPreview()),
      });
      return;
    }

    await route.continue();
  });
}

async function mockAlertDestinations(page: import("@playwright/test").Page) {
  await page.route("**/api/alerts/config", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        enabled: true,
        activationState: "active",
        overrides: {},
      }),
    });
  });

  await page.route("**/api/alerts/active", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([]),
    });
  });

  await page.route("**/api/notifications/email", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        enabled: false,
        provider: "",
        server: "",
        port: 587,
        username: "",
        password: "",
        from: "",
        to: [],
        tls: false,
        startTLS: false,
      }),
    });
  });

  await page.route("**/api/notifications/apprise", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        enabled: true,
        mode: "http",
        targets: [],
        cliPath: "apprise",
        timeoutSeconds: 15,
        serverUrl: "https://apprise.lab.local",
        configKey: "",
        apiKey: "",
        apiKeyHeader: "X-API-KEY",
        skipTlsVerify: false,
      }),
    });
  });

  await page.route("**/api/notifications/webhooks", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([]),
    });
  });
}

test.describe("TLS verification warning banners", () => {
  test.setTimeout(180_000);

  test("keeps a persistent warning visible while editing a VMware connection", async ({
    page,
  }) => {
    await mockVMwareConnections(page);

    await page.goto("/settings/infrastructure", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure/, {
      timeout: 15_000,
    });

    await page.getByRole("button", { name: "Add infrastructure" }).click();
    await page.getByRole("button", { name: "VMware" }).click();
    await page.getByLabel("Skip TLS verification").check();

    const warning = page.getByRole("alert").filter({
      hasText: "this vCenter connection",
    });
    await expect(warning).toBeVisible();
    await expect(warning).toContainText(
      "Install a trusted certificate for vCenter before using this in production.",
    );

    fs.mkdirSync(path.dirname(VMWARE_SCREENSHOT_PATH), { recursive: true });
    await page.screenshot({ path: VMWARE_SCREENSHOT_PATH, fullPage: true });
  });

  test("keeps a persistent warning visible while editing a TrueNAS connection", async ({
    page,
  }) => {
    await mockTrueNASConnections(page);

    await page.goto("/settings/infrastructure", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure/, {
      timeout: 15_000,
    });

    await page.getByRole("button", { name: "Add infrastructure" }).click();
    await page.getByRole("button", { name: "TrueNAS" }).click();
    await page.getByLabel("Skip TLS verification").check();

    const warning = page.getByRole("alert").filter({
      hasText: "this TrueNAS connection",
    });
    await expect(warning).toBeVisible();
    await expect(warning).toContainText(
      "Install a trusted certificate or configure the TLS fingerprint before using this in production.",
    );

    fs.mkdirSync(path.dirname(TRUENAS_SCREENSHOT_PATH), { recursive: true });
    await page.screenshot({ path: TRUENAS_SCREENSHOT_PATH, fullPage: true });
  });

  test("keeps a persistent warning visible while enabling Apprise self-signed certificates", async ({
    page,
  }) => {
    await mockAlertDestinations(page);

    await page.goto("/alerts/destinations", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/alerts\/destinations/, { timeout: 15_000 });

    await expect(
      page.getByRole("heading", { level: 1, name: "Notification Destinations" }),
    ).toBeVisible();

    await page.getByLabel("Allow self-signed certificates").check();

    const warning = page.getByRole("alert").filter({
      hasText: "this Apprise API endpoint",
    });
    await expect(warning).toBeVisible();
    await expect(warning).toContainText(
      "Install a trusted certificate on the Apprise server before using this in production.",
    );

    fs.mkdirSync(path.dirname(APPRISE_SCREENSHOT_PATH), { recursive: true });
    await page.screenshot({ path: APPRISE_SCREENSHOT_PATH, fullPage: true });
  });
});
