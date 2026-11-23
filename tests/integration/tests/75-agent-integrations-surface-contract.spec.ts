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
  test("leads with un-collapsed 3-step pulse CLI onboarding, least-privilege token presets, and live manifest", async ({
    page,
  }) => {
    const manifestResponse = page.waitForResponse(
      (response) =>
        response.url().includes("/api/agent/capabilities") &&
        response.request().method() === "GET",
    );

    await page.goto("/settings/pulse-intelligence/agents", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/pulse-intelligence\/agents/, {
      timeout: 15_000,
    });
    await expect(
      page.getByRole("heading", { level: 2, name: "External agents" }),
    ).toBeVisible();

    const response = await manifestResponse;
    expect(response.ok()).toBeTruthy();

    // Un-collapsed 3-step pulse CLI setup immediately visible.
    await expect(
      page.getByText("brew install rcourtman/tap/pulse", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText("Sign in read-only", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText("Connect your agent and verify", { exact: true }),
    ).toBeVisible();

    // Switch external agent client to Codex.
    await page.getByRole("button", { name: "Codex" }).click();
    await expect(
      page.getByText("pulse connect codex", { exact: false }),
    ).toBeVisible();

    // Secondary section for proposing fixes + Patrol mode link.
    await expect(
      page.getByRole("heading", {
        level: 3,
        name: "Want agents to propose fixes?",
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("link", { name: "Choose Patrol mode" }),
    ).toBeVisible();

    // Manual config disclosure uses pulse mcp with zero plaintext token placeholders.
    await page.locator("summary").filter({ hasText: "Manual config" }).click();
    await expect(
      page.getByRole("heading", { level: 4, name: /Codex.*~\/\.codex\/config\.toml/ }),
    ).toBeVisible();
    await expect(
      page.getByText("[mcp_servers.pulse]", { exact: false }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { level: 4, name: /OpenCode.*opencode\.json/ }),
    ).toBeVisible();

    // Developer details disclosure preserves live manifest inspection.
    await page
      .locator("summary")
      .filter({ hasText: "Developer details" })
      .click();
    await expect(
      page.getByText(
        "Only open this when you are building or debugging a client",
        { exact: false },
      ),
    ).toBeVisible();
    await page
      .locator("summary")
      .filter({ hasText: "Live manifest details" })
      .click();
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
      page.getByRole("button", { name: "External agent (read-only)" }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "External agent (propose fixes)" }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Operator CLI" }),
    ).toBeVisible();
  });
});
