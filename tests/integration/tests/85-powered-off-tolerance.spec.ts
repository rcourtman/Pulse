import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test as base, type Route } from "@playwright/test";
import { createAuthenticatedStorageState } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
type WorkerFixtures = { authStorageStatePath: string };

const test = base.extend<{}, WorkerFixtures>({
  storageState: async ({ authStorageStatePath }, use) =>
    use(authStorageStatePath),
  authStorageStatePath: [
    async ({ browser }, use, workerInfo) => {
      const storageStatePath = path.resolve(
        __dirname,
        "..",
        "..",
        "tmp",
        "playwright-auth",
        `powered-off-tolerance-${workerInfo.project.name}.json`,
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

test("powered-off tolerance preserves inheritance, explicit zero, and strict validation", async ({
  page,
}, testInfo) => {
  let document = {
    schemaVersion: 1,
    revision: 0,
    defaults: {},
    resourceTypes: {},
    resources: {},
  };
  const updates: Array<Record<string, unknown>> = [];

  await page.route("**/api/resources?**", async (route: Route) => {
    const url = new URL(route.request().url());
    const pageNumber = Number(url.searchParams.get("page") ?? "1");
    const data =
      pageNumber === 1
        ? [
            {
              id: "vm:1567",
              type: "vm",
              name: "backup-window-vm",
              displayName: "backup-window-vm",
              status: "stopped",
              lastSeen: "2026-07-24T12:00:00Z",
              sources: ["proxmox"],
              platformScopes: ["proxmox"],
              platformType: "proxmox",
            },
          ]
        : [];
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data,
        meta: {
          page: pageNumber,
          limit: 100,
          total: 1,
          totalPages: 1,
        },
      }),
    });
  });

  await page.route("**/api/alerts/intent-policies", async (route: Route) => {
    if (route.request().method() === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(document),
      });
      return;
    }
    const update = route.request().postDataJSON() as Record<string, unknown>;
    updates.push(update);
    document = {
      ...update,
      revision: Number(update.revision) + 1,
    } as typeof document;
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(document),
    });
  });

  await page.goto("/alerts/thresholds", { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: "Configure policies" }).click();

  const tolerance = page.getByRole("spinbutton", {
    name: /VM \/ container powered-off tolerance/,
  });
  await expect(tolerance).toHaveValue("");
  await expect(
    page.getByText(
      "Blank inherits the existing policy; 0 alerts on the first stopped observation.",
      { exact: true },
    ),
  ).toBeVisible();
  await expect(page.getByRole("combobox", { name: "Resource" })).toHaveValue(
    "vm:1567",
  );
  await tolerance.fill("300");
  await page.getByRole("button", { name: "Save defaults" }).click();
  await expect(
    page.getByText("Default intent policy saved.", { exact: true }),
  ).toBeVisible();
  expect(updates).toHaveLength(1);
  expect(updates[0]).toMatchObject({
    resourceTypes: {
      guest: {
        "state.offline": {
          graceSeconds: 300,
        },
      },
    },
  });

  const override = page.getByRole("spinbutton", {
    name: /Grace override \(seconds\)/,
  });
  await override.fill("1.5");
  await page.getByRole("button", { name: "Save override" }).click();
  await expect(
    page.getByText("Grace override must be a whole number of seconds.", {
      exact: true,
    }),
  ).toBeVisible();
  expect(updates).toHaveLength(1);

  await override.fill("0");
  await page.getByRole("button", { name: "Save override" }).click();
  await expect(
    page.getByText("Resource intent override saved.", { exact: true }),
  ).toBeVisible();
  expect(updates).toHaveLength(2);
  const resourceRules = updates[1].resources as Record<
    string,
    Record<string, { graceSeconds?: number }>
  >;
  expect(Object.values(resourceRules)[0]["state.offline"]).toEqual({
    graceSeconds: 0,
  });

  await testInfo.attach("powered-off-tolerance", {
    body: await page.screenshot(),
    contentType: "image/png",
  });
});
