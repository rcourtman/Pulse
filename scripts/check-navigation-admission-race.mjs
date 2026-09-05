// Full application, real organisation selector; synthetic HTTP and cold socket.
// Run with PULSE_PROOF_WIDTH=390|1440 and PULSE_PROOF_MODE=failure|success|reverse.
// pulse-heavy-run -- node scripts/check-navigation-admission-race.mjs
import { createServer } from "../frontend-modern/node_modules/vite/dist/node/index.js";
import {
  chromium,
  expect,
} from "../tests/integration/node_modules/@playwright/test/index.mjs";
import { fileURLToPath } from "node:url";
import { mkdir, rm, writeFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";

const repo = fileURLToPath(new URL("../", import.meta.url));
const root = `${repo}frontend-modern`;
const width = Number(process.env.PULSE_PROOF_WIDTH || 1440);
const mode = process.env.PULSE_PROOF_MODE || "failure";
if (
  ![390, 1440].includes(width) ||
  !["failure", "success", "reverse"].includes(mode)
) {
  throw new Error("Use width 390 or 1440 and mode failure, success or reverse");
}
const phase =
  process.env.PULSE_PROOF_PHASE === "baseline" ? "baseline" : "repaired";
const output = `${repo}tmp/navigation-admission-race/${phase}-${width}-${mode}`;
const emptyAdmission = {
  aggregations: {
    platformAdmission: {
      proxmox: false,
      docker: false,
      kubernetes: false,
      truenas: false,
      vmware: false,
      standalone: false,
    },
  },
};
await mkdir(output, { recursive: true });
await rm(`${output}/result.json`, { force: true });
process.chdir(root);
const server = await createServer({
  root,
  configFile: `${root}/vite.config.ts`,
  server: { host: "127.0.0.1", port: 5199, strictPort: true },
});
let browser;
try {
  await server.listen();
  browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport: { width, height: 900 } });
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.route("**/*", (route) =>
    new URL(route.request().url()).origin === "http://127.0.0.1:5199"
      ? route.continue()
      : route.abort(),
  );
  // No resource snapshot: admission, rather than inventory, must decide tabs.
  await page.routeWebSocket("**/ws*", () => {});
  let outgoing;
  let incoming;
  let hold = false;
  let failed = 0;
  const requests = [];
  await page.route(
    (url) => url.pathname.startsWith("/api/"),
    async (route) => {
      const url = new URL(route.request().url());
      const path = url.pathname;
      const json = (data) => route.fulfill({ json: data });
      if (path === "/api/resources") {
        if (
          url.searchParams.get("page") === "1" &&
          url.searchParams.get("limit") === "1"
        ) {
          requests.push(
            route.request().headers()["x-pulse-org-id"] || "default",
          );
          if (!hold)
            return json({
              aggregations: {
                platformAdmission: {
                  proxmox: true,
                  docker: true,
                  kubernetes: false,
                  truenas: false,
                  vmware: false,
                  standalone: false,
                },
              },
            });
          if (!outgoing) {
            outgoing = route;
            return;
          }
          incoming = route;
          if (mode === "reverse") return;
          if (mode === "success") return json(emptyAdmission);
          failed++;
          return route.fulfill({
            status: 503,
            json: { error: "Synthetic admission failure" },
          });
        }
        return json({ resources: [], total: 0, aggregations: {} });
      }
      if (path === "/api/orgs")
        return json([
          { id: "default", displayName: "Default Organization" },
          { id: "acme", displayName: "Acme" },
        ]);
      if (path === "/api/license/runtime-capabilities")
        return json({
          capabilities: ["multi_tenant"],
          limits: [],
          hosted_mode: false,
          max_history_days: 7,
        });
      if (path === "/api/security/status")
        return json({ hasAuthentication: true, sessionCapabilities: {} });
      if (path === "/api/version")
        return json({ version: "6.5.0-test", isDevelopment: true });
      if (path === "/api/health") return json({ status: "healthy" });
      if (path === "/api/alerts/active") return json([]);
      return json({ data: [], policy: {} });
    },
  );
  await page.goto("http://127.0.0.1:5199/alerts");
  const org = page.getByLabel("Organization", { exact: true });
  await expect(org)
    .toBeVisible({ timeout: 60_000 })
    .catch(async (error) => {
      console.error(errors, await page.locator("body").innerText());
      throw error;
    });
  hold = true;
  // Exercise the production reconnect subscriber without replacing its store.
  await page.evaluate(async () =>
    (await import("/src/stores/events.ts")).eventBus.emit(
      "websocket_reconnected",
    ),
  );
  await expect.poll(() => Boolean(outgoing)).toBe(true);
  const documentIdentity = await page.evaluate(() => performance.timeOrigin);
  await org.selectOption("acme");
  await expect(org).toHaveValue("acme");
  await expect.poll(() => Boolean(incoming)).toBe(true);
  if (mode !== "reverse") {
    await expect
      .poll(() =>
        incoming
          .request()
          .response()
          .then((r) => r?.status()),
      )
      .toBe(mode === "failure" ? 503 : 200);
  }
  const nav = page.getByRole("navigation", {
    name: width < 1280 ? "Mobile navigation" : "Primary navigation",
    exact: true,
  });
  const platform =
    width < 1280
      ? nav.locator('[data-tab-id="platform-switcher"]')
      : nav.getByRole("link", { name: /Proxmox/ });
  await expect(platform).toHaveCount(0);
  const response = page.waitForResponse(
    (r) =>
      r.url().includes("/api/resources?page=1&limit=1") && r.status() === 200,
  );
  await outgoing.fulfill({
    json: {
      aggregations: {
        platformAdmission: {
          proxmox: true,
          docker: true,
          kubernetes: false,
          truenas: false,
          vmware: false,
          standalone: false,
        },
      },
    },
  });
  await (await response).finished();
  // Give the fetch continuation and Solid render a browser task and two frames.
  await page.evaluate(
    () =>
      new Promise((resolve) =>
        setTimeout(
          () => requestAnimationFrame(() => requestAnimationFrame(resolve)),
          100,
        ),
      ),
  );
  // Remove the transient switch toast so the narrow navigation is inspectable.
  const dismiss = page.getByRole("button", { name: "Dismiss notification" });
  for (const button of await dismiss.all()) await button.click();
  await expect(dismiss).toHaveCount(0);
  await page.screenshot({ path: `${output}/after-outgoing-response.png` });
  await expect(platform).toHaveCount(0);
  expect(requests).toEqual(["default", "default", "acme"]);
  expect(failed).toBe(mode === "failure" ? 1 : 0);
  if (mode === "reverse") {
    const newestResponse = page.waitForResponse(
      (r) => r.request() === incoming.request(),
    );
    await incoming.fulfill({ json: emptyAdmission });
    await (await newestResponse).finished();
    await expect(platform).toHaveCount(0);
  }
  if (width < 1280) {
    await nav
      .getByRole("button", { name: "More navigation", exact: true })
      .click();
    const more = page.getByRole("menu", {
      name: "More navigation destinations",
    });
    await expect(
      more.getByRole("menuitem", { name: /Settings/ }),
    ).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(more).toBeHidden();
    await nav
      .getByRole("button", { name: "More navigation", exact: true })
      .click();
    await more.getByRole("menuitem", { name: /Settings/ }).click();
    await expect(page).toHaveURL(/\/settings/);
  } else {
    await expect(nav.getByRole("link", { name: /Settings/ })).toBeVisible();
  }
  // A subsequent successful organisation request must still restore navigation.
  hold = false;
  await org.selectOption("default");
  await expect(platform).toBeVisible();
  expect(requests).toEqual(["default", "default", "acme", "default"]);
  if (width < 1280) {
    await platform.click();
    const menu = page.getByRole("menu", {
      name: "Switch platform",
      exact: true,
    });
    await menu.getByRole("menuitem", { name: "Docker", exact: true }).click();
    await expect(page).toHaveURL(/\/docker/);
    await expect(menu).toBeHidden();
    await page.screenshot({ path: `${output}/recovered-navigation.png` });
  }
  expect(await page.evaluate(() => performance.timeOrigin)).toBe(
    documentIdentity,
  );
  expect(errors).toEqual([]);
  const result = {
    result: "passed",
    browser: browser.version(),
    viewport: { width, height: 900 },
    mode,
    phase,
    requests,
    failed,
    verified_at: new Date().toISOString(),
    source_sha256: createHash("sha256")
      .update(readFileSync(`${root}/src/useAppRuntimeState.ts`))
      .digest("hex"),
  };
  await writeFile(
    `${output}/result.json`,
    JSON.stringify(result, null, 2) + "\n",
  );
  console.log(JSON.stringify(result));
} finally {
  await browser?.close();
  await server.close();
}
