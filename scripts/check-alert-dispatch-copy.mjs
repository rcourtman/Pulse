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
import { AlertsAPI } from '/src/api/alerts';
import { NotificationsAPI } from '/src/api/notifications';
import { OverviewTab } from '/src/features/alerts/OverviewTab';
import '/src/index.css';
const ids = ['ready','cooldown','pending'];
AlertsAPI.getDeliveryDiagnoses = async () => ids.map(id => ({
alertIdentifier:id, alertId:id, trackingKey:id, status:id==='cooldown'?'suppressed':'would_send',
reason:id==='cooldown'?'cooldown':'ready', lastNotified:id==='pending'?undefined:'2026-08-26T10:15:00Z',
nextEligibleAt:id==='cooldown'?'2026-08-26T10:20:00Z':undefined
}));
AlertsAPI.getEvents = async () => [];
NotificationsAPI.getHealth = async () => ({queue:{status:'healthy'}});
const alerts = Object.fromEntries(ids.map(id => [id, {id,resourceId:id,resourceName:'VM '+id,
type:'cpu',level:'warning',message:'High CPU on '+id,startTime:'2026-08-26T10:00:00Z',acknowledged:false,node:'node1'}]));
function Fixture() { return <main class="p-4"><OverviewTab overrides={[]} activeAlerts={alerts}
updateAlert={()=>{}} showQuickTip={()=>false} dismissQuickTip={()=>{}} showAcknowledged={()=>true}
setShowAcknowledged={()=>{}} alertsDisabled={()=>false}/></main>; }
render(()=><Router><Route path="/qualification" component={Fixture}/></Router>,document.getElementById('root'));
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
      name: "dispatch-fixture",
      configureServer(s) {
        s.middlewares.use((req, res, next) => {
          if (req.url === "/qualification") {
            res.setHeader("Content-Type", "text/html");
            res.end(
              '<div id="root"></div><script type="module" src="/dispatch-fixture.tsx"></script>',
            );
          } else next();
        });
      },
      resolveId(id) {
        if (id === "/dispatch-fixture.tsx") return id;
      },
      load(id) {
        if (id === "/dispatch-fixture.tsx") return fixture;
      },
    },
  ],
  resolve: { alias: { "@": resolve(root, "src") } },
  server: { host: "127.0.0.1", port: 5198, strictPort: true },
});
let browser;
try {
  await server.listen();
  browser = await chromium.launch({ headless: true });
  mkdirSync("/tmp/pulse-alert-dispatch", { recursive: true });
  for (const width of [1440, 900, 390]) {
    const page = await browser.newPage({ viewport: { width, height: 1000 } });
    const errors = [];
    page.on("pageerror", (e) => {
      errors.push(e.message);
      console.error(e.message);
    });
    page.on("console", (m) => {
      if (m.type() === "error") console.error(m.text());
    });
    await page.route("http://127.0.0.1:5198/api/**", (route) =>
      route.fulfill({ json: [] }),
    );
    await page.goto("http://127.0.0.1:5198/qualification");
    await page.getByText(/^Dispatch requested .*next eligible/).waitFor();
    assert.equal(await page.getByText(/^Dispatch requested /).count(), 2);
    assert.equal(await page.getByText(/^Notified /).count(), 0);
    assert.equal(
      await page.getByText("Notification pending", { exact: true }).count(),
      1,
    );
    for (const label of await page.getByText(/^Dispatch requested /).all()) {
      assert.equal(
        await label.evaluate((el) => {
          const r = document.createRange();
          r.selectNodeContents(el);
          return [...r.getClientRects()].every(
            (b) => b.left >= 0 && b.right <= innerWidth,
          );
        }),
        true,
        "status text must fit viewport",
      );
    }
    assert.deepEqual(errors, []);
    await page.screenshot({
      path: "/tmp/pulse-alert-dispatch/" + width + ".png",
      fullPage: true,
    });
    await page.close();
  }
  console.log(
    JSON.stringify({
      result: "passed",
      viewports: [1440, 900, 390],
      scope:
        "Real Overview and Chromium; scripted diagnoses, not installed delivery or receipt",
    }),
  );
} finally {
  await browser?.close();
  await server.close();
}
