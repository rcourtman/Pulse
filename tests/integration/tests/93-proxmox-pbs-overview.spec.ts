import { expect, test } from "@playwright/test";

import { ensureAuthenticated, getMockMode, setMockMode } from "./helpers";

test.describe("Proxmox PBS overview", () => {
  test.setTimeout(180_000);

  test("keeps standalone PBS health and details in the estate scan", async ({
    page,
  }) => {
    await ensureAuthenticated(page);
    const initialMockMode = await getMockMode(page);

    try {
      if (!initialMockMode.enabled) {
        await setMockMode(page, true);
      }

      await page.setViewportSize({ width: 1280, height: 800 });
      await page.goto("/proxmox/overview", { waitUntil: "domcontentloaded" });

      const overview = page.getByTestId("proxmox-page");
      const nodes = overview.locator(".proxmox-nodes-card");
      const servers = overview.locator(
        '[data-proxmox-backups-table="servers"]',
      );
      const guests = overview.locator("#proxmox-guests-section");

      await expect(nodes).toBeVisible({ timeout: 60_000 });
      await expect(servers).toBeVisible({ timeout: 60_000 });
      const desktopRow = servers.locator("tbody tr[aria-expanded]").first();
      await expect(desktopRow).toBeVisible();
      await expect(desktopRow.locator("td").first()).not.toHaveText("");
      await expect(
        servers.getByRole("columnheader", { name: "Backups" }),
      ).toHaveCount(0);
      await expect(guests).toBeVisible();

      const placement = await overview.evaluate((root) => {
        const nodesElement = root.querySelector(".proxmox-nodes-card");
        const serversElement = root.querySelector(
          '[data-proxmox-backups-table="servers"]',
        );
        const guestsElement = root.querySelector("#proxmox-guests-section");
        if (!nodesElement || !serversElement || !guestsElement) return null;
        return {
          nodesBeforeServers: Boolean(
            nodesElement.compareDocumentPosition(serversElement) &
            Node.DOCUMENT_POSITION_FOLLOWING,
          ),
          serversBeforeGuests: Boolean(
            serversElement.compareDocumentPosition(guestsElement) &
            Node.DOCUMENT_POSITION_FOLLOWING,
          ),
        };
      });
      expect(placement).toEqual({
        nodesBeforeServers: true,
        serversBeforeGuests: true,
      });

      const desktopToggle = desktopRow.getByRole("button", {
        name: /details for/,
      });
      await desktopToggle.click();
      await expect(desktopRow).toHaveAttribute("aria-expanded", "true");
      await expect(
        servers.getByRole("tab", { name: "History" }).first(),
      ).toBeVisible();
      await desktopToggle.click();

      await page.setViewportSize({ width: 390, height: 844 });
      await page.reload({ waitUntil: "domcontentloaded" });
      await expect(servers).toBeVisible({ timeout: 60_000 });

      const narrowRow = servers.locator("tbody tr[aria-expanded]").first();
      await narrowRow.click();
      await expect(narrowRow).toHaveAttribute("aria-expanded", "true");
      await expect(
        servers.getByRole("tab", { name: "History" }).first(),
      ).toBeVisible();

      const pageWidth = await page.evaluate(() => ({
        clientWidth: document.documentElement.clientWidth,
        scrollWidth: document.documentElement.scrollWidth,
      }));
      expect(pageWidth.scrollWidth).toBeLessThanOrEqual(
        pageWidth.clientWidth + 1,
      );
    } finally {
      if (!initialMockMode.enabled) {
        await setMockMode(page, false);
      }
    }
  });
});
