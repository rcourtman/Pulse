// Run with pulse-heavy-run -- node scripts/check-backup-app-refresh.mjs
// Full application shell with synthetic API and WebSocket snapshots; no live deployment.
import { createServer } from "../frontend-modern/node_modules/vite/dist/node/index.js";
import { chromium, expect } from "@playwright/test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../frontend-modern", import.meta.url));
process.chdir(root); // PostCSS/Tailwind resolve their config from the frontend root.
const server = await createServer({
  root,
  configFile: `${root}/vite.config.ts`,
  server: { host: "127.0.0.1", port: 5198, strictPort: true },
});
let browser;
try {
  await server.listen();
  browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({
    viewport: {
      width: Number(process.env.PULSE_BROWSER_WIDTH || 1440),
      height: 900,
    },
  });
  // Never allow this synthetic qualification to contact a real service.
  await page.route("**/*", (route) => {
    const url = new URL(route.request().url());
    if (url.origin !== "http://127.0.0.1:5198") return route.abort();
    return route.continue();
  });
  const errors = [];
  const consoleErrors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("console", (message) => {
    if (message.type() === "error") consoleErrors.push(message.text());
  });
  page.on("requestfailed", (request) =>
    console.error(request.url(), request.failure()),
  );
  let generation = 0;
  const workloads = () =>
    Array.from({ length: 240 }, (_, index) => ({
      id: `ct-${112 + index}`,
      type: "system-container",
      name: `guest-${String(index).padStart(2, "0")}-snapshot-${generation}`,
      displayName: `guest-${String(index).padStart(2, "0")}-snapshot-${generation}`,
      platformId: "pve-a",
      platformType: "proxmox-pve",
      sourceType: "api",
      status: "running",
      lastSeen: Date.now(),
      proxmox: { vmid: 112 + index, node: "pve-a", instance: "pve-a" },
    }));
  const pbs = () => ({
    id: "pbs-1",
    type: "pbs",
    name: `pbs-main-snapshot-${generation}`,
    displayName: "PBS Main",
    platformId: "pbs-main",
    platformType: "proxmox-pbs",
    sourceType: "hybrid",
    metricsTarget: { resourceType: "agent", resourceId: "agent-pbs-1" },
    agent: { agentId: "agent-pbs-1", hostname: "pbs-main" },
    status: "online",
    lastSeen: Date.now(),
    cpu: { current: 12 + generation },
    memory: { current: 40, total: 8000, used: 3200 },
    pbs: {
      instanceId: "pbs-main",
      version: `3.2.${generation}`,
      datastores: [],
    },
    platformData: { pbs: { instanceId: "pbs-main", hostname: "pbs-main" } },
  });
  const state = () => ({
    resources: [...workloads(), pbs()],
    connectedInfrastructure: [],
    activeAlerts: [],
    recentlyResolved: [],
    metrics: [],
    lastUpdate: Date.now(),
    stats: { version: "6.5.0-test", startTime: new Date().toISOString() },
  });
  let socket;
  await page.routeWebSocket(/\/ws(?:\?|$)/, (ws) => {
    socket = ws;
    ws.send(JSON.stringify({ type: "initialState", data: state() }));
  });
  await page.route(
    (url) => url.pathname.startsWith("/api/"),
    (route) => {
      const path = new URL(route.request().url()).pathname;
      if (path === "/api/alerts/active" || path === "/api/orgs")
        return route.fulfill({ json: [] });
      if (path === "/api/version")
        return route.fulfill({
          json: { version: "6.5.0-test", isDevelopment: true },
        });
      if (path === "/api/updates/check")
        return route.fulfill({
          json: {
            available: false,
            currentVersion: "6.5.0-test",
            latestVersion: "6.5.0-test",
          },
        });
      if (path === "/api/updates/release-notes")
        return route.fulfill({
          json: { version: "6.5.0-test", releaseNotes: "" },
        });
      if (path === "/api/security/status")
        return route.fulfill({
          json: { hasAuthentication: true, sessionCapabilities: {} },
        });
      if (path === "/api/state") return route.fulfill({ json: state() });
      if (path === "/api/state/summary") return route.fulfill({ json: {} });
      if (path === "/api/health")
        return route.fulfill({ json: { status: "healthy" } });
      if (path === "/api/resources") {
        const types = new URL(route.request().url()).searchParams
          .get("type")
          ?.split(",");
        const resources = state().resources.filter(
          (r) => !types || types.includes(r.type),
        );
        const query = new URL(route.request().url()).searchParams;
        const limit = Number(query.get("limit") || 100);
        const pageNumber = Number(query.get("page") || 1);
        return route.fulfill({
          json: {
            resources: resources.slice(
              (pageNumber - 1) * limit,
              pageNumber * limit,
            ),
            meta: { totalPages: Math.ceil(resources.length / limit) },
          },
        });
      }
      if (path === "/api/backups/pve")
        return route.fulfill({
          json: {
            data: {
              guestSnapshots: workloads().map((w) => ({
                id: `snap-${w.id}`,
                name: "pre-upgrade",
                node: "pve-a",
                instance: "pve-a",
                type: "ct",
                vmid: w.proxmox.vmid,
                time: new Date().toISOString(),
                vmstate: false,
              })),
              storageBackups: [],
              backupTasks: [],
            },
          },
        });
      if (path === "/api/backups/pbs")
        return route.fulfill({ json: { data: { backups: [] } } });
      return route.fulfill({ json: { data: [], policy: {} } });
    },
  );
  await page.goto("http://127.0.0.1:5198/proxmox/backups/coverage");
  await expect(
    page.getByText("pbs-main-snapshot-0", { exact: true }),
  ).toBeVisible({ timeout: 60000 });
  const toggle = page.getByRole("button", {
    name: /expand details for guest-20-/i,
  });
  // The narrow layout exposes this control on keyboard focus, not pointer hover.
  await toggle.waitFor({ timeout: 60_000 }).catch(async (e) => {
    console.error(await page.locator("body").innerText());
    throw e;
  });
  await toggle.focus();
  await toggle.press("Enter").catch(async (error) => {
    console.error(errors, await page.locator("body").innerText());
    throw error;
  });
  const expanded = page.getByRole("button", {
    name: /collapse details for guest-20-/i,
  });
  await expanded.focus();
  await expect(
    page.getByText("Restore evidence", { exact: true }),
  ).toBeVisible();
  // Finish layout/scroll synchronisation before measuring refresh effects.
  await page.waitForTimeout(500);
  const visibleCoverageRows = await page
    .locator('[data-proxmox-backup-row="coverage"]')
    .count();
  expect(visibleCoverageRows).toBeLessThan(240);
  expect(visibleCoverageRows).toBeGreaterThan(20);
  const baseline = await page.evaluate(() => ({
    y: document.querySelector(".app-scroll-shell").scrollTop,
    url: location.href,
  }));
  expect(baseline.y).toBeGreaterThan(100);
  const samples = [];
  for (generation = 1; generation <= 3; generation++) {
    expect(socket).toBeTruthy();
    socket.send(JSON.stringify({ type: "rawData", data: state() }));
    await expect(
      page
        .getByText(`guest-20-snapshot-${generation}`, { exact: true })
        .first(),
    ).toBeVisible();
    await expect(
      page.getByText("Restore evidence", { exact: true }),
    ).toBeVisible();
    const sample = await page.evaluate(() => ({
      y: document.querySelector(".app-scroll-shell").scrollTop,
      url: location.href,
    }));
    console.log(
      JSON.stringify({
        generation,
        ...sample,
        focused: await expanded.evaluate((el) => el === document.activeElement),
      }),
    );
    await expect(expanded).toBeFocused();
    expect(sample.url).toBe(baseline.url);
    expect(Math.abs(sample.y - baseline.y)).toBeLessThanOrEqual(1);
    samples.push({ generation, ...sample });
  }

  // Navigate through the real router, then hold a scrolled By date view.
  await page.getByRole("link", { name: "By date", exact: true }).click();
  const dateRow = page
    .locator('[data-proxmox-backup-row="recoverable"]')
    .filter({ hasText: "guest-20-" })
    .first();
  await dateRow.scrollIntoViewIfNeeded();
  const dateTab = page.getByRole("link", { name: "By date", exact: true });
  await dateTab.evaluate((el) => el.focus({ preventScroll: true }));
  const capture = () =>
    page.evaluate(() => ({
      y: document.querySelector(".app-scroll-shell").scrollTop,
      url: location.href,
      active:
        document.activeElement?.getAttribute("aria-label") ||
        document.activeElement?.textContent,
    }));
  const dateBaseline = await capture();
  expect(dateBaseline.y).toBeGreaterThan(100);
  const dateSamples = [];
  for (generation = 4; generation <= 6; generation++) {
    socket.send(JSON.stringify({ type: "rawData", data: state() }));
    await expect(dateRow).toContainText(`guest-20-snapshot-${generation}`);
    await expect(dateTab).toBeFocused();
    const sample = await capture();
    expect(sample.url).toBe(dateBaseline.url);
    expect(Math.abs(sample.y - dateBaseline.y)).toBeLessThanOrEqual(1);
    dateSamples.push(sample);
  }
  console.log(
    JSON.stringify({
      view: "date",
      baseline: dateBaseline,
      samples: dateSamples,
    }),
  );

  const pbsToggle = page.getByRole("button", {
    name: /Expand details for pbs-main-snapshot-/,
  });
  await pbsToggle.focus();
  await pbsToggle.press("Enter");
  const history = page.getByRole("tab", { name: "History", exact: true });
  await history.click();
  await history.focus();
  await expect(page.getByTestId("resource-metrics-history-tab")).toBeVisible();
  await page.waitForTimeout(500);
  const historyBaseline = await capture();
  const historySamples = [];
  for (generation = 7; generation <= 9; generation++) {
    socket.send(JSON.stringify({ type: "rawData", data: state() }));
    await expect(
      page
        .getByText(`pbs-main-snapshot-${generation}`, { exact: true })
        .first(),
    ).toBeVisible();
    await expect(history).toHaveAttribute("aria-selected", "true");
    await expect(history).toBeFocused();
    await expect(
      page.getByTestId("resource-metrics-history-tab"),
    ).toBeVisible();
    const sample = await capture();
    expect(sample.url).toBe(historyBaseline.url);
    expect(Math.abs(sample.y - historyBaseline.y)).toBeLessThanOrEqual(1);
    historySamples.push(sample);
  }
  console.log(
    JSON.stringify({
      view: "PBS History",
      baseline: historyBaseline,
      samples: historySamples,
    }),
  );
  expect(errors).toEqual([]);
  expect(consoleErrors).toEqual([]);
  console.log(
    JSON.stringify(
      {
        baseline,
        samples,
        visibleCoverageRows,
        pageErrors: errors,
        consoleErrors,
      },
      null,
      2,
    ),
  );
} finally {
  await browser?.close();
  await server.close();
}
