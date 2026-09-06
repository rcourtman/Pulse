// Isolated real-browser component qualification; no installed backend or delivery claim.
import { createServer } from "../frontend-modern/node_modules/vite/dist/node/index.js";
import solid from "../frontend-modern/node_modules/vite-plugin-solid/dist/esm/index.mjs";
import { chromium } from "@playwright/test";
import { resolve } from "node:path";
import { mkdirSync } from "node:fs";
import assert from "node:assert/strict";
const root = resolve("frontend-modern");
process.chdir(root);
const fixture = `
import { render } from 'solid-js/web';
import { Show } from 'solid-js';
import { NotificationsAPI } from '/src/api/notifications';
import { AlertsAPI } from '/src/api/alerts';
import { useAlertDestinationsTabState } from '/src/features/alerts/useAlertDestinationsTabState';
import { AlertDeliveryHealthCard } from '/src/features/alerts/AlertDeliveryHealthCard';
import '/src/index.css';
const pending = [];
NotificationsAPI.getHealth = () => new Promise((resolve, reject) => pending.push({resolve, reject}));
NotificationsAPI.getDeliveryLog = async () => [];
AlertsAPI.getEvents = async () => [];
NotificationsAPI.retryTerminalFailures = NotificationsAPI.dismissTerminalFailures = async () => ({affected: 1});
window.confirm = () => true;
window.finish = (i, status) => status === 'error' ? pending[i].reject(new Error('scripted offline')) : pending[i].resolve({queue:{status, failed:1, deadLetter:0, attentionRequired:1}});
window.count = () => pending.length;
function Fixture() {
const s = useAlertDestinationsTabState({emailConfig:()=>({}), appriseConfig:()=>({}), setAppriseConfig:()=>{}, configLoadError:()=> 'scripted config unavailable', isRetrying:()=>false, isLoadingDestinations:()=>false, onRetryLoad:()=>{}, webhooks:()=>[]});
return <main class="p-4"><h1>Delivery health ordering fixture</h1><button onClick={s.handleRetry}>Configuration Retry</button><output data-testid="loading">{String(s.refreshingDeliveryHealth())}</output><Show when={s.deliveryNeedsAttention()}><AlertDeliveryHealthCard health={s.deliveryHealth()?.queue ?? null} unavailable={s.deliveryHealthUnavailable()} refreshing={s.refreshingDeliveryHealth()} onRefresh={s.loadDeliveryHealth} onRetryFailures={s.retryTerminalFailures} onDismissFailures={s.dismissTerminalFailures}/></Show></main>;
}
render(() => <Fixture/>, document.getElementById('root'));
`;
const server = await createServer({
  root,
  configFile: false,
  optimizeDeps: {
    noDiscovery: true,
    entries: [],
    esbuildOptions: { target: "esnext" },
  },
  esbuild: { target: "esnext" },
  plugins: [
    solid(),
    {
      name: "ordering-fixture",
      configureServer(s) {
        s.middlewares.use((req, res, next) => {
          if (req.url === "/qualification") {
            res.setHeader("Content-Type", "text/html");
            res.end(
              '<div id="root"></div><script type="module" src="/ordering-fixture.tsx"></script>',
            );
          } else next();
        });
      },
      resolveId(id) {
        if (id === "/ordering-fixture.tsx") return id;
      },
      load(id) {
        if (id === "/ordering-fixture.tsx") return fixture;
      },
    },
  ],
  resolve: { alias: { "@": resolve(root, "src") } },
  server: { host: "127.0.0.1", port: 5197, strictPort: true },
});
let browser;
try {
  await server.listen();
  browser = await chromium.launch({ headless: true });
  mkdirSync("/tmp/pulse-health-ordering", { recursive: true });
  let cases = 0;
  for (const width of [1440, 390]) {
    for (const [old, newer] of [
      ["healthy", "degraded"],
      ["degraded", "healthy"],
      ["error", "healthy"],
      ["healthy", "error"],
    ]) {
      const page = await browser.newPage({ viewport: { width, height: 900 } });
      await page.goto("http://127.0.0.1:5197/qualification");
      await page.waitForFunction(() => window.count?.() === 1);
      await page
        .getByRole("button", { name: "Configuration Retry", exact: true })
        .click();
      await page.waitForFunction(() => window.count() === 2);
      await page.evaluate((s) => window.finish(1, s), newer);
      await page.waitForFunction(
        () => document.querySelector("output").textContent === "false",
      );
      const before = await page.locator("main").innerText();
      await page.evaluate((s) => window.finish(0, s), old);
      await page.evaluate(
        () =>
          new Promise((r) =>
            requestAnimationFrame(() => requestAnimationFrame(r)),
          ),
      );
      assert.equal(await page.locator("main").innerText(), before);
      assert.equal(
        await page.getByRole("alert").count(),
        ["degraded", "error"].includes(newer) ? 1 : 0,
      );
      await page.screenshot({
        path: `/tmp/pulse-health-ordering/${width}-${old}-${newer}.png`,
      });
      await page.close();
      cases++;
    }
    for (const action of [
      "Retry retained deliveries",
      "Dismiss retained failures",
    ]) {
      const page = await browser.newPage({ viewport: { width, height: 900 } });
      await page.goto("http://127.0.0.1:5197/qualification");
      await page.waitForFunction(() => window.count?.() === 1);
      await page.evaluate(() => window.finish(0, "degraded"));
      await page.getByRole("alert").waitFor();
      await page
        .getByRole("button", { name: "Configuration Retry", exact: true })
        .click();
      await page.getByRole("button", { name: action, exact: true }).click();
      await page.waitForFunction(() => window.count() === 3);
      await page.evaluate(() => window.finish(1, "healthy"));
      assert.equal(await page.getByTestId("loading").textContent(), "true");
      await page.evaluate(() => window.finish(2, "healthy"));
      await page.waitForFunction(
        () => document.querySelector("output").textContent === "false",
      );
      assert.equal(await page.getByRole("alert").count(), 0);
      await page.close();
      cases++;
    }
  }
  console.log(
    JSON.stringify({
      result: "passed",
      cases,
      viewports: [1440, 390],
      scope:
        "real Chromium, real caller/hook/card, scripted API; not installed application or notification receipt",
    }),
  );
} finally {
  await browser?.close();
  await server.close();
}
