import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test as base } from "@playwright/test";
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
        `agent-integrations-surface-contract-${workerInfo.project.name}.json`,
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

test.describe("Agent integrations surface contract", () => {
  test("projects Pulse Intelligence surfaces from the live capabilities manifest", async ({
    page,
  }) => {
    const manifestResponse = page.waitForResponse(
      (response) =>
        response.url().includes("/api/agent/capabilities") &&
        response.request().method() === "GET",
    );

    await page.goto("/settings/security/api", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/security\/api/, { timeout: 15_000 });
    await expect(
      page.getByRole("heading", { level: 2, name: "Agent integrations" }),
    ).toBeVisible();

    const response = await manifestResponse;
    expect(response.ok()).toBeTruthy();

    const surfaces = page.getByRole("heading", {
      level: 3,
      name: "Pulse Intelligence surfaces",
    });
    await expect(surfaces).toBeVisible();

    const surfaceSection = surfaces.locator("..");
    for (const label of [
      "Pulse Intelligence Core",
      "Pulse Patrol",
      "Pulse Assistant",
      "Pulse MCP",
    ]) {
      await expect(
        surfaceSection.getByText(label, { exact: true }),
      ).toBeVisible();
    }

    await expect(
      surfaceSection.getByText("Native surface", { exact: true }),
    ).toBeVisible();
    await expect(
      surfaceSection.getByText("External adapter", { exact: true }),
    ).toBeVisible();
    await expect(
      surfaceSection.getByText("Tools", { exact: true }).first(),
    ).toBeVisible();
    await expect(
      surfaceSection.getByText("Interactive questions", { exact: true }),
    ).toBeVisible();
    await expect(
      surfaceSection.getByText("Capability metadata", { exact: true }),
    ).toBeVisible();
    await expect(
      surfaceSection.getByText(
        "Canonical context, governed actions, safety gates, approval state, action audit, and verification shared by Pulse Assistant, Pulse MCP, and Pulse Patrol.",
        { exact: true },
      ),
    ).toBeVisible();

    const horizontalOverflow = await surfaceSection
      .locator("ul")
      .evaluate((node) => ({
        clientWidth: node.clientWidth,
        scrollWidth: node.scrollWidth,
      }));
    expect(horizontalOverflow.scrollWidth).toBeLessThanOrEqual(
      horizontalOverflow.clientWidth + 1,
    );

    const createTokenLink = page.getByRole("link", {
      name: "Create token",
    });
    await expect(page.getByText("Token setup", { exact: true })).toBeVisible();
    await expect(
      page.getByText("Pulse Intelligence agent", { exact: true }).last(),
    ).toBeVisible();
    await expect(
      page.getByText("Stable failure codes", { exact: true }),
    ).toBeVisible();
    await expect(page.getByText("resource_not_found").first()).toBeVisible();
    await expect(createTokenLink).toHaveAttribute("href", "#api-token-create");
    await expect(
      page.getByRole("heading", { level: 3, name: "MCP client config" }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { level: 4, name: /OpenCode.*opencode\.json/ }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { level: 4, name: /mcpServers/ }),
    ).toBeVisible();
    await expect(page.getByText("PULSE_API_TOKEN").first()).toBeVisible();
    await expect(page.getByText("--base-url").first()).toBeVisible();

    await createTokenLink.click();
    await expect(page).toHaveURL(/#api-token-create$/);
    await expect(
      page.getByRole("heading", { level: 2, name: "Create token" }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Pulse Intelligence agent" }),
    ).toBeVisible();
  });
});
