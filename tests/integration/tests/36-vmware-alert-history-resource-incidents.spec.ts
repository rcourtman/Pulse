import { expect, test } from "@playwright/test";
const SCREENSHOT_PATH = "/tmp/vmware-alert-history-resource-incidents.png";

test.describe("VMware alert history resource incidents", () => {
  test.setTimeout(180_000);

  test("opens VMware resource incidents through the shared alert history surface", async ({
    page,
  }) => {
    await page.route("**/api/alerts/config", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          enabled: true,
          activationState: "active",
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

    await page.route("**/api/alerts/history**", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: "unified-incident-vm-app-01-vmware-alarm-21-vmware-alarm-state",
            type: "resource-incident",
            level: "critical",
            resourceId: "vm:app-01",
            resourceName: "app-01",
            node: "esxi-01.lab.local",
            nodeDisplayName: "ESXi 01",
            instance: "VMware",
            message: "VM vm-201 has VMware alarm VM replication fault (red)",
            value: 0,
            threshold: 0,
            startTime: "2026-03-30T18:13:00Z",
            lastSeen: "2026-03-30T18:15:00Z",
            acknowledged: false,
            metadata: {
              incidentCategory: "health",
              incidentLabel: "VM Health Issue",
              vmwareConnectionId: "vc-1",
            },
          },
        ]),
      });
    });

    await page.route("**/api/alerts/incidents**", async (route) => {
      const requestUrl = new URL(route.request().url());
      if (requestUrl.searchParams.get("resource_id") !== "vm:app-01") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify([]),
        });
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: "incident-1",
            alertIdentifier:
              "unified-incident-vm-app-01-vmware-alarm-21-vmware-alarm-state",
            alertType: "Resource Health",
            level: "critical",
            resourceId: "vm:app-01",
            resourceName: "app-01",
            resourceType: "vm",
            node: "esxi-01.lab.local",
            instance: "VMware",
            message: "VM vm-201 has VMware alarm VM replication fault (red)",
            status: "open",
            openedAt: "2026-03-30T18:13:00Z",
            acknowledged: false,
            events: [
              {
                id: "event-1",
                type: "alert_fired",
                timestamp: "2026-03-30T18:13:00Z",
                summary: "Alert fired",
                details: {
                  vmwareConnectionId: "vc-1",
                },
              },
            ],
          },
        ]),
      });
    });

    await page.route("**/api/resources**", async (route) => {
      const requestUrl = new URL(route.request().url());
      if (requestUrl.pathname !== "/api/resources") {
        await route.continue();
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          data: [
            {
              id: "vm:app-01",
              type: "vm",
              name: "app-01",
              displayName: "app-01",
              platformId: "vm:app-01",
              platformType: "vmware-vsphere",
              sourceType: "api",
              sources: ["vmware"],
              status: "warning",
              lastSeen: "2026-03-30T18:15:00Z",
              vmware: {
                connectionId: "vc-1",
                connectionName: "Lab VC",
                vcenterHost: "vc.lab.local",
                managedObjectId: "vm-201",
                entityType: "vm",
                overallStatus: "red",
                activeAlarmCount: 1,
                activeAlarmSummary: "VM replication fault (red)",
                recentTaskCount: 1,
                recentTaskSummary: "Create snapshot (success)",
                snapshotCount: 2,
              },
              incidents: [
                {
                  provider: "vmware",
                  nativeId: "alarm-21",
                  code: "vmware_alarm_state",
                  severity: "critical",
                  source: "vmware",
                  summary:
                    "VM vm-201 has VMware alarm VM replication fault (red)",
                  startedAt: "2026-03-30T18:13:00Z",
                },
              ],
            },
          ],
          meta: {
            page: 1,
            limit: 200,
            total: 1,
            totalPages: 1,
          },
        }),
      });
    });

    await page.goto("http://127.0.0.1:5173/alerts/history", {
      waitUntil: "domcontentloaded",
    });

    const letsGoButton = page.getByRole("button", { name: "Let's go" });
    if (
      await letsGoButton
        .waitFor({ state: "visible", timeout: 3000 })
        .then(() => true)
        .catch(() => false)
    ) {
      await letsGoButton.click();
    }

    await expect(
      page.getByRole("heading", { name: "Alert History" }),
    ).toBeVisible();
    await expect(page.getByText("app-01")).toBeVisible();
    const historyRow = page.locator("tr").filter({ hasText: "app-01" }).first();
    await expect(historyRow).toContainText(
      "VM vm-201 has VMware alarm VM replication fault (red)",
    );
    await historyRow.getByRole("button", { name: "Resource" }).click();

    await expect(
      page.getByText("VM vm-201 has VMware alarm VM replication fault (red)"),
    ).toBeVisible();
    await expect(page.getByText("Resource incidents")).toBeVisible();
    await expect(page.getByText("Resource Health")).toBeVisible();

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
