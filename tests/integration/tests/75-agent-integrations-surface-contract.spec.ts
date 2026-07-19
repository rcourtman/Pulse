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
  test("guides external agents through Patrol governance and the live manifest", async ({
    page,
  }) => {
    const manifestResponse = page.waitForResponse(
      (response) =>
        response.url().includes("/api/agent/capabilities") &&
        response.request().method() === "GET",
    );

    await page.goto("/settings/pulse-intelligence/assistant", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/pulse-intelligence\/assistant/, {
      timeout: 15_000,
    });
    await expect(
      page.getByRole("heading", { level: 2, name: "External agents" }),
    ).toBeVisible();

    const response = await manifestResponse;
    expect(response.ok()).toBeTruthy();

    await expect(
      page.getByText(
        "Connect external tools to read Pulse context and request Patrol work.",
        { exact: true },
      ),
    ).toBeVisible();
    await expect(
      page.getByText(
        "Patrol mode and scoped tokens control what connected agents can do.",
        { exact: true },
      ),
    ).toBeVisible();
    await expect(
      page.getByRole("link", { name: "Choose Patrol mode" }),
    ).toBeVisible();

    await page.getByRole("button", { name: "Show connector setup" }).click();
    await expect(
      page.getByRole("heading", { level: 3, name: "Connector setup" }),
    ).toBeVisible();
    await expect(page.getByText("Create a scoped token", { exact: true })).toBeVisible();
    await expect(page.getByText("Connect the agent", { exact: true })).toBeVisible();

    await page.locator("summary").filter({ hasText: "Client config" }).click();
    await expect(page.getByText("PULSE_API_TOKEN").first()).toBeVisible();
    await expect(
      page.getByRole("heading", { level: 4, name: /OpenCode.*opencode\.json/ }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { level: 4, name: /mcpServers/ }),
    ).toBeVisible();

    await page.locator("summary").filter({ hasText: "Developer details" }).click();
    await expect(
      page.getByText(
        "Only open this when you are building or debugging a client",
        { exact: false },
      ),
    ).toBeVisible();
    await page.locator("summary").filter({ hasText: "Live manifest details" }).click();
    await expect(
      page.getByText("/api/agent/capabilities", { exact: true }),
    ).toBeVisible();
    await page.locator("summary").filter({ hasText: "Failure codes" }).click();
    await expect(page.getByText("resource_not_found").first()).toBeVisible();

    const panel = page.locator("#external-agent-setup");
    const horizontalOverflow = await panel.evaluate((node) => ({
      clientWidth: node.clientWidth,
      scrollWidth: node.scrollWidth,
    }));
    expect(horizontalOverflow.scrollWidth).toBeLessThanOrEqual(
      horizontalOverflow.clientWidth + 1,
    );

    const createTokenLink = page.getByRole("link", {
      name: "Create token",
    });
    await expect(createTokenLink).toHaveAttribute(
      "href",
      "/settings/security/api?tokenPreset=pulse-intelligence-agent#api-token-create",
    );

    await createTokenLink.click();
    await expect(page).toHaveURL(
      /\/settings\/security\/api\?tokenPreset=pulse-intelligence-agent#api-token-create$/,
    );
    await expect(
      page.getByRole("heading", { level: 2, name: "Create token" }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Patrol external agent" }),
    ).toBeVisible();
  });
});
