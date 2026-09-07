import { expect, test, type WebSocketRoute } from "@playwright/test";

// Synthetic canonical API/socket snapshots; not a live poller or FreeBSD proof.
test("hybrid guest memory follows canonical snapshots in the browser", async ({
  page,
}, testInfo) => {
  test.skip(
    testInfo.project.name !== "chromium",
    "Desktop synthetic memory acceptance",
  );
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const vm = (instance: string, percent?: number) => ({
    id: `${instance}-pve1-101`,
    type: "vm",
    name: `${instance}-vm`,
    status: "running",
    platformType: "proxmox-pve",
    platformScopes: ["proxmox"],
    sourceType: "hybrid",
    sources: ["proxmox", "agent"],
    lastSeen: new Date().toISOString(),
    proxmox: { vmid: 101, nodeName: "pve1", instance },
    memory:
      percent === undefined
        ? undefined
        : {
            current: percent,
            used: percent * 1024 * 1024,
            total: 100 * 1024 * 1024,
          },
    agent: {
      memory: { used: 35 * 1024 * 1024, total: 100 * 1024 * 1024, usage: 35 },
    },
  });
  let resources = [vm("cluster-a", 100), vm("cluster-b", 80)];
  let socket: WebSocketRoute | undefined;
  const send = () =>
    socket?.send(
      JSON.stringify({
        type: "rawData",
        data: { resources },
        timestamp: Date.now(),
      }),
    );
  await page.routeWebSocket("**/ws*", (ws) => {
    socket = ws;
    send();
  });
  await page.route("**/api/**", async (route) => {
    const url = new URL(route.request().url());
    if (!url.pathname.startsWith("/api/")) {
      await route.continue();
      return;
    }
    let body: unknown = {};
    if (url.pathname === "/api/security/status")
      body = {
        hasAuthentication: true,
        hasProxyAuth: true,
        proxyAuthUsername: "synthetic-browser",
        requiresAuth: false,
      };
    if (url.pathname === "/api/orgs" || url.pathname === "/api/alerts/active")
      body = [];
    if (url.pathname === "/api/metrics-store/history")
      body = {
        metrics: {
          cpu: [],
          memory: [],
          disk: [],
          netin: [],
          netout: [],
          diskread: [],
          diskwrite: [],
        },
      };
    if (url.pathname.startsWith("/api/charts"))
      body = {
        data: {},
        nodeData: {},
        storageData: {},
        dockerData: {},
        stats: {},
      };
    if (url.pathname === "/api/version")
      body = { version: "6.4.1", channel: "stable" };
    if (url.pathname === "/api/updates/check")
      body = {
        currentVersion: "6.4.1",
        latestVersion: "6.4.1",
        updateAvailable: false,
      };
    if (url.pathname === "/api/resources") {
      const types = url.searchParams.get("type");
      const data =
        !types || types.split(",").includes("vm")
          ? resources.map(({ memory, ...resource }) => ({
              ...resource,
              metrics: {
                memory: memory && { ...memory, percent: memory.current },
              },
            }))
          : [];
      body = {
        data,
        meta: { page: 1, limit: 100, total: data.length, totalPages: 1 },
        aggregations: {
          total: 2,
          byType: { vm: 2 },
          byPlatform: { "proxmox-pve": 2 },
          platformAdmission: {
            proxmox: true,
            docker: false,
            kubernetes: false,
            truenas: false,
            vmware: false,
            standalone: false,
          },
        },
        links: { next: null },
      };
    }
    await route.fulfill({ json: body });
  });
  await page.goto("/proxmox");
  const row = page.locator(".workload-row").filter({ hasText: "cluster-a-vm" });
  await expect(row).toBeVisible({ timeout: 45_000 });
  await expect(row).toContainText("100%");
  resources = [vm("cluster-a", 35), vm("cluster-b", 80)];
  send();
  await expect(row).toContainText("35%");
  await expect(
    page.locator(".workload-row").filter({ hasText: "cluster-b-vm" }),
  ).toContainText("80%");
  await row.click();
  const drawer = page.getByRole("region", {
    name: "cluster-a-vm",
    exact: true,
  });
  await expect(drawer).toBeVisible();
  await expect(drawer).toContainText("100 MB");
  await expect(drawer).toContainText("65.0 MB");
  await testInfo.attach("hybrid-memory-35-percent", {
    body: await page.screenshot(),
    contentType: "image/png",
  });
  const memoryCell = row.locator('[data-workload-col="memory"]');
  // TestCorrelatedGuestMemoryNextPoll retains status-mem (100%) when
  // agent evidence is unavailable/stale/offline. Do not revive nested 35%.
  for (const status of ["unavailable", "stale", "offline"]) {
    resources = [
      {
        ...vm("cluster-a", 100),
        sourceStatus: { agent: { status } },
      } as ReturnType<typeof vm>,
      vm("cluster-b", 80),
    ];
    send();
    await expect(memoryCell).toContainText("100%");
    await expect(drawer).not.toContainText("65.0 MB");
    await expect(
      page.locator(".workload-row").filter({ hasText: "cluster-b-vm" }),
    ).toContainText("80%");
  }
  // Workload details use canonical metrics only: withdrawal removes the Memory
  // section; recovery restores it. Do not invent Usage UI or raw-facet totals.
  const memoryDetails = drawer.locator("tbody").filter({
    has: page.locator("th").filter({ hasText: /^Memory$/ }),
  });
  for (const viewport of [
    { width: 1280, height: 900 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport);
    resources = [
      {
        ...vm("cluster-a"),
        proxmox: {
          ...vm("cluster-a").proxmox,
          memory: {
            total: 100 * 1024 * 1024,
            used: 0,
            free: 0,
            usage: 0,
            usageUnavailable: true,
          },
        },
      } as ReturnType<typeof vm>,
      vm("cluster-b", 80),
    ];
    send();
    await expect(memoryCell).toContainText("N/A");
    await expect(memoryCell).not.toContainText("100%");
    await expect(memoryDetails).toHaveCount(0);
    await expect(drawer.getByText("Free", { exact: true })).toHaveCount(0);
    await drawer.scrollIntoViewIfNeeded();
    await expect(drawer).toBeInViewport();
    await expect(
      page.locator(".workload-row").filter({ hasText: "cluster-b-vm" }),
    ).toContainText("80%");
    await testInfo.attach(`withdrawn-memory-${viewport.width}`, {
      body: await page.screenshot(),
      contentType: "image/png",
    });

    resources = [vm("cluster-a", 0), vm("cluster-b", 80)];
    send();
    await expect(memoryCell).toContainText("0%");
    await expect(memoryCell).not.toContainText("N/A");
    // Centre the recovered details above the fixed narrow-screen navigation.
    await memoryDetails.evaluate((element) =>
      element.scrollIntoView({ block: "center", inline: "nearest" }),
    );
    await expect.poll(async () => {
      const box = await memoryDetails.boundingBox();
      return box ? box.y + box.height : Infinity;
    }).toBeLessThan(viewport.height - 64);
    await expect(memoryDetails).toBeInViewport({ ratio: 1 });
    await expect(memoryDetails).toContainText("100 MB");
    await expect(
      memoryDetails.locator("tr").filter({
        has: page.getByText("Free", { exact: true }),
      }),
    ).toContainText("100 MB");
    await expect(
      page.locator(".workload-row").filter({ hasText: "cluster-b-vm" }),
    ).toContainText("80%");
    await testInfo.attach(`recovered-memory-${viewport.width}`, {
      body: await page.screenshot(),
      contentType: "image/png",
    });
  }
  expect(errors).toEqual([]);
});
