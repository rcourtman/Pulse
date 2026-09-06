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
import { Router, Route } from '@solidjs/router';
import { NotificationsAPI } from '/src/api/notifications';
import { AlertsAPI } from '/src/api/alerts';
import { OverviewTab } from '/src/features/alerts/OverviewTab';
import '/src/index.css';
const pending = [];
NotificationsAPI.getHealth = () => new Promise((resolve, reject) => pending.push({resolve, reject}));
AlertsAPI.getDeliveryDiagnoses = async () => [];
window.actions = 0;
NotificationsAPI.retryTerminalFailures = NotificationsAPI.dismissTerminalFailures = async () => { window.actions++; return {affected: 1}; };
window.confirm = () => true;
window.finish = (i, status) => status === 'error' ? pending[i].reject(new Error('scripted offline')) : pending[i].resolve({queue:{status, failed:0, deadLetter:status === 'healthy' ? 0 : 1, attentionRequired:status === 'healthy' ? 0 : 1}});
window.count = () => pending.length;
function Fixture() {
return <main class="p-4"><OverviewTab overrides={[]} activeAlerts={{}} updateAlert={()=>{}} showQuickTip={()=>false} dismissQuickTip={()=>{}} showAcknowledged={()=>true} setShowAcknowledged={()=>{}} alertsDisabled={()=>false}/></main>;
}
render(() => <Router><Route path="/qualification" component={Fixture}/></Router>, document.getElementById('root'));
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
  mkdirSync("/tmp/pulse-overview-refresh", { recursive: true });
  let cases = 0;
  for (const theme of ["light", "dark"]) {
    for (const width of [1440, 900, 390]) {
      for (const action of [
        "Retry retained deliveries",
        "Dismiss retained failures",
      ]) {
        const page = await browser.newPage({
          viewport: { width, height: 900 },
        });
        await page.goto("http://127.0.0.1:5197/qualification");
        await page.evaluate(
          (theme) =>
            document.documentElement.classList.toggle("dark", theme === "dark"),
          theme,
        );
        await page.waitForFunction(() => window.count?.() === 1);
        await page.evaluate(() => window.finish(0, "degraded"));
        await page.getByRole("button", { name: action, exact: true }).click();
        await page.waitForFunction(() => window.count() === 2);
        await page.evaluate(() => window.finish(1, "error"));
        const refresh = page.getByRole("button", {
          name: "Refresh delivery status",
          exact: true,
        });
        await refresh.waitFor();
        assert.match(
          await page.getByRole("alert").innerText(),
          /status is unavailable/,
        );
        const geometry = await page.getByRole("alert").evaluate((el) => {
          const heading = el.querySelector("h3");
          const bounds = el.getBoundingClientRect();
          return {
            titleFits: heading.scrollWidth <= heading.clientWidth + 1,
            actionsFit: [...el.querySelectorAll("button, a")].every(
              (control) => {
                const rect = control.getBoundingClientRect();
                return rect.left >= bounds.left && rect.right <= bounds.right;
              },
            ),
          };
        });
        assert.equal(
          geometry.titleFits,
          true,
          "warning heading must not overflow beneath actions",
        );
        assert.equal(
          geometry.actionsFit,
          true,
          "warning actions must stay inside the card",
        );
        await page.screenshot({
          path: `/tmp/pulse-overview-refresh/${theme}-${width}-${cases}-unavailable.png`,
        });
        await refresh.click();
        await page.waitForFunction(() => window.count() === 3);
        assert.equal(await refresh.isDisabled(), true);
        await page.evaluate(() => window.finish(2, "healthy"));
        await page.getByRole("alert").waitFor({ state: "detached" });
        assert.equal(await page.evaluate(() => window.actions), 1);
        await page.screenshot({
          path: `/tmp/pulse-overview-refresh/${theme}-${width}-${cases}-healthy.png`,
        });
        cases++;
        await page.close();
      }
    }
  }
  console.log(`${cases} overview action/outage/refresh browser cases passed`);
} finally {
  await browser?.close();
  await server.close();
}
