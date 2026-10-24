import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test as base, expect } from "@playwright/test";
import {
  createAuthenticatedStorageState,
  apiRequest,
  authenticateWithAPIToken,
  ensureAuthenticated,
} from "./helpers";

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
        `runtime-home-onboarding-contract-${workerInfo.project.name}.json`,
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

test.describe("runtime-home onboarding contract", () => {
  test.setTimeout(180_000);

  test("opens the agent install handoff on the shared infrastructure workspace", async ({
    page,
  }) => {
    await ensureAuthenticated(page);
    await page.goto("/settings/infrastructure?add=agent", {
      waitUntil: "domcontentloaded",
    });

    await page.waitForURL(/\/settings\/infrastructure\?add=agent$/, {
      timeout: 15_000,
    });
    await expect(
      page.getByRole("heading", {
        level: 1,
        name: "Infrastructure",
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("dialog").getByRole("heading", {
        name: "Add Pulse Agent",
        exact: true,
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("dialog").getByRole("heading", {
        level: 2,
        name: "Install on a host",
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /Generate token/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /Probe API endpoint/i }),
    ).toHaveCount(0);
  });

  test("opens the platform-pick handoff on the shared infrastructure workspace", async ({
    page,
  }) => {
    await ensureAuthenticated(page);
    await page.goto("/settings/infrastructure?add=pick", {
      waitUntil: "domcontentloaded",
    });

    await page.waitForURL(/\/settings\/infrastructure\?add=pick$/, {
      timeout: 15_000,
    });
    await expect(
      page.getByRole("heading", {
        level: 1,
        name: "Infrastructure",
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("dialog").getByRole("button", {
        name: /Detect API platform/i,
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /Generate token/i }),
    ).toHaveCount(0);
  });

  test("waits for token bootstrap before navigating to the agent install handoff", async ({ page }) => {
    await ensureAuthenticated(page);
    const created = await apiRequest(page, "/api/security/tokens", {
      method: "POST",
      data: { name: "E2E browser bootstrap regression", scopes: ["*"] },
    });
    expect(created.ok()).toBeTruthy();
    const { token, record } = await created.json() as { token: string; record: { id: string } };
    expect(token).toBeTruthy();
    expect(record.id).toBeTruthy();
    // This page owns only the bootstrap being tested. Replacing the setup
    // page would also intercept its departing document's in-flight reads.
    const bootstrapPage = await page.context().newPage();
    let releaseBootstrap!: () => void;
    const bootstrapHold = new Promise<void>(resolve => { releaseBootstrap = resolve; });
    let notifyBlocked!: () => void;
    const blocked = new Promise<void>(resolve => { notifyBlocked = resolve; });
    const holdState = async (route: import("@playwright/test").Route) => {
      notifyBlocked();
      await bootstrapHold;
      await route.continue();
    };
    let authentication: Promise<boolean> | undefined;
    try {
      await bootstrapPage.route("**/api/state/summary", holdState);
      let completed = false;
      authentication = authenticateWithAPIToken(bootstrapPage, token).then(result => { completed = true; return result; });
      await Promise.race([blocked, authentication]);
      expect(completed, "token authentication must await protected browser bootstrap").toBe(false);
      releaseBootstrap();
      expect(await authentication).toBe(true);
      await bootstrapPage.goto("/settings/infrastructure?add=agent", { waitUntil: "domcontentloaded" });
      await expect(bootstrapPage).toHaveURL(/\/settings\/infrastructure\?add=agent$/);
      await expect(bootstrapPage.getByRole("dialog").getByRole("heading", {
        name: "Add Pulse Agent", exact: true,
      })).toBeVisible();
    } finally {
      releaseBootstrap();
      await authentication?.catch(() => {});
      await bootstrapPage.unroute("**/api/state/summary", holdState);
      await bootstrapPage.close();
      const revoked = await apiRequest(page, `/api/security/tokens/${encodeURIComponent(record.id)}`, {
        method: "DELETE",
      });
      expect(revoked.ok(), "the test-owned bootstrap token must be revoked").toBeTruthy();
    }
  });

  test("retires legacy platform-management paths to the not-found page", async ({
    page,
  }) => {
    await ensureAuthenticated(page);
    await page.goto("/settings/infrastructure/platforms/truenas", {
      waitUntil: "domcontentloaded",
    });

    // The legacy aliases were deliberately retired (cfb16e6da); the contract
    // is now a graceful not-found page with a workspace escape hatch rather
    // than a silent redirect.
    await expect(
      page.getByText("No route matched /settings/infrastructure/platforms/truenas"),
    ).toBeVisible({ timeout: 15_000 });
    await expect(page.getByRole("button", { name: "Go to workspace" })).toBeVisible();
  });
});
