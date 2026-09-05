// Run with pulse-heavy-run -- node scripts/check-backup-browser-polling.mjs
// Isolated real-browser component regression; no running Pulse instance needed.
import { createServer } from "../frontend-modern/node_modules/vite/dist/node/index.js";
import { chromium, expect } from "@playwright/test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../frontend-modern", import.meta.url));
process.chdir(root); // PostCSS/Tailwind resolve their config from the frontend root.
const server = await createServer({
  root,
  configFile: `${root}/vite.config.ts`,
  server: { host: "127.0.0.1", port: 5198, strictPort: true },
  plugins: [
    {
      name: "backup-browser-fixture",
      configureServer(server) {
        server.middlewares.use((req, _res, next) => {
          if (req.url?.startsWith("/proxmox/backups/"))
            req.url = "/browser-tests/backups.html";
          next();
        });
      },
    },
  ],
});
let browser;
try {
  await server.listen();
  browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({
    viewport: { width: Number(process.env.PULSE_BROWSER_WIDTH || 1440), height: 900 },
  });
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("console", (message) => {
    if (message.type() === "error") console.error(message.text());
  });
  page.on("requestfailed", (request) =>
    console.error(request.url(), request.failure()),
  );
  let generation = 0;
  const workloads = () =>
    Array.from({ length: 40 }, (_, index) => ({
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
  await page.route("**/fixture/workloads", (route) =>
    route.fulfill({ json: workloads() }),
  );
  await page.route(
    (url) => url.pathname.startsWith("/api/"),
    (route) => {
      const path = new URL(route.request().url()).pathname;
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
  const toggle = page.getByRole("button", {
    name: /expand details for guest-20-/i,
  });
  // The narrow layout exposes this control on keyboard focus, not pointer hover.
  await toggle.waitFor({ timeout: 20_000 });
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
  const baseline = await page.evaluate(() => ({
    y: document.querySelector("main").scrollTop,
    url: location.href,
  }));
  expect(baseline.y).toBeGreaterThan(100);
  const samples = [];
  for (generation = 1; generation <= 3; generation++) {
    await expect(
      page
        .getByText(`guest-20-snapshot-${generation}`, { exact: true })
        .first(),
    ).toBeVisible();
    await expect(
      page.getByText("Restore evidence", { exact: true }),
    ).toBeVisible();
    const sample = await page.evaluate(() => ({
      y: document.querySelector("main").scrollTop,
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
  expect(errors).toEqual([]);
  console.log(
    JSON.stringify({ baseline, samples, pageErrors: errors }, null, 2),
  );
} finally {
  await browser?.close();
  await server.close();
}
