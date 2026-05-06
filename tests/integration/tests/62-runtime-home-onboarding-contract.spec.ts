import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test as base, expect } from "@playwright/test";
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

  test("normalizes the agent install handoff onto the shared infrastructure workspace", async ({
    page,
  }) => {
    await page.goto("/settings/infrastructure?add=agent", {
      waitUntil: "domcontentloaded",
    });

    await page.waitForURL(/\/settings\/infrastructure$/, { timeout: 15_000 });
    await expect(
      page.getByRole("heading", {
        level: 1,
        name: "Infrastructure",
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { level: 2, name: "Install on a host" }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /Generate token/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /Probe address/i }),
    ).toHaveCount(0);
  });

  test("normalizes the platform-pick handoff onto the shared infrastructure workspace", async ({
    page,
  }) => {
    await page.goto("/settings/infrastructure?add=pick", {
      waitUntil: "domcontentloaded",
    });

    await page.waitForURL(/\/settings\/infrastructure$/, { timeout: 15_000 });
    await expect(
      page.getByRole("heading", {
        level: 1,
        name: "Infrastructure",
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /Probe address/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /Generate token/i }),
    ).toHaveCount(0);
  });

  test("normalizes legacy platform-management paths back to the inventory workspace", async ({
    page,
  }) => {
    await page.goto("/settings/infrastructure/platforms/truenas", {
      waitUntil: "domcontentloaded",
    });

    await page.waitForURL(/\/settings\/infrastructure$/, { timeout: 15_000 });
    await expect(
      page.getByRole("heading", {
        level: 1,
        name: "Infrastructure",
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { level: 3, name: "Monitored systems" }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /Probe address/i }),
    ).toHaveCount(0);
    await expect(
      page.getByRole("heading", { level: 2, name: "Install on a host" }),
    ).toHaveCount(0);
  });
});
