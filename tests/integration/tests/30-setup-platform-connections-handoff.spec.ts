import { test, expect } from "@playwright/test";
import { ensureAuthenticated } from "./helpers";

test.describe("Setup completion Add infrastructure handoff", () => {
  test("preview exposes Add infrastructure as the canonical first source choice", async ({
    page,
  }) => {
    await ensureAuthenticated(page);
    await page.goto("/preview/setup-complete", {
      waitUntil: "domcontentloaded",
    });

    await expect(
      page.getByText("Choose your first infrastructure source"),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Add infrastructure", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Install Pulse Agent" }),
    ).toBeVisible();
    await expect(
      page.getByText(
        "Start with a platform API when a platform manages the estate. Install Pulse Agent when the system itself should report node-local telemetry.",
      ),
    ).toBeVisible();

    await page
      .getByRole("button", { name: "Add infrastructure", exact: true })
      .click();
    await page.waitForURL(/\/settings\/infrastructure\?add=pick$/, {
      timeout: 15_000,
    });
    const addDialog = page.getByRole("dialog", { name: "Add infrastructure" });
    await expect(addDialog).toBeVisible();
    await expect(
      addDialog.getByText("Choose how Pulse should connect"),
    ).toBeVisible();
    await expect(
      addDialog.getByRole("button", { name: /Detect API platform/i }),
    ).toBeVisible();
    await expect(
      addDialog.getByRole("button", { name: /Install Pulse Agent/i }),
    ).toBeVisible();
  });

  test("preview exposes the VMware-connected continuation scenario through Infrastructure", async ({
    page,
  }) => {
    await ensureAuthenticated(page);
    await page.goto("/preview/setup-complete?scenario=vmware-api-backed", {
      waitUntil: "domcontentloaded",
    });

    await expect(
      page.getByRole("heading", {
        name: "First monitored system connected",
        exact: true,
      }),
    ).toBeVisible();
    await expect(page.getByText("VMware vSphere")).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Open Infrastructure" }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Add infrastructure", exact: true }),
    ).toBeVisible();

    await page.getByRole("button", { name: "Open Infrastructure" }).click();
    await page.waitForURL(/\/settings\/infrastructure$/, { timeout: 15_000 });
    await expect(
      page.getByText("Connected systems", { exact: true }),
    ).toBeVisible();
  });
});
