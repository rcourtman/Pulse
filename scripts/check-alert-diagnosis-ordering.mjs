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
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { AlertsAPI } from '/src/api/alerts';
import { NotificationsAPI } from '/src/api/notifications';
import { OverviewTab } from '/src/features/alerts/OverviewTab';
import '/src/index.css';
let finishOlder;
let requests = 0;
AlertsAPI.getDeliveryDiagnoses = () => {
  requests++;
  if (requests === 1) return new Promise(resolve => { finishOlder = resolve; });
  return Promise.resolve([{alertIdentifier:'a1', alertId:'a1', status:'suppressed',
    reason:'notifications_disabled', message:'Notifications disabled by current configuration'}]);
};
window.finishOlder = () => finishOlder([{alertIdentifier:'a1', alertId:'a1',
  status:'would_send',reason:'ready',lastNotified:'2026-08-26T10:15:00Z'}]);
window.requestCount = () => requests;
AlertsAPI.getEvents = async () => [];
NotificationsAPI.getHealth = async () => ({queue:{status:'healthy'}});
const alert = id => ({id,resourceId:id,resourceName:'VM '+id,type:'cpu',level:'warning',
message:'High CPU on '+id,startTime:new Date().toISOString(),acknowledged:false,node:'node1'});
function Fixture() {
const [alerts, setAlerts] = createSignal({a1:alert('a1')});
return <main class="p-4"><button onClick={()=>setAlerts({a1:alert('a1'),a2:alert('a2')})}>Add alert</button>
<OverviewTab overrides={[]} activeAlerts={alerts()}
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
  server: { host: "127.0.0.1", port: 5199, strictPort: true },
});
let browser;
try {
  await server.listen();
  browser = await chromium.launch({ headless: true });
  mkdirSync("/tmp/pulse-alert-diagnosis-ordering", { recursive: true });
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
    await page.route("http://127.0.0.1:5199/api/**", (route) =>
      route.fulfill({ json: [] }),
    );
    await page.goto("http://127.0.0.1:5199/qualification");
    await page.waitForFunction(() => window.requestCount?.() === 1);
    await page.getByRole("button", { name: "Add alert", exact: true }).click();
    await page
      .getByText("Notifications are turned off", { exact: true })
      .waitFor();
    await page.evaluate(async () => {
      window.finishOlder();
      await Promise.resolve();
    });
    assert.equal(
      await page
        .getByText("Notifications are turned off", { exact: true })
        .count(),
      1,
    );
    assert.equal(await page.getByText(/^Dispatch requested /).count(), 0);
    assert.equal(
      await page.getByText("High CPU on a2", { exact: true }).count(),
      1,
    );
    const label = page.getByText("Notifications are turned off", {
      exact: true,
    });
    assert.equal(
      await label.evaluate((el) => {
        const range = document.createRange();
        range.selectNodeContents(el);
        return [...range.getClientRects()].every(
          (b) => b.left >= 0 && b.right <= innerWidth,
        );
      }),
      true,
      "current status must fit viewport",
    );
    assert.deepEqual(errors, []);
    await page.screenshot({
      path: "/tmp/pulse-alert-diagnosis-ordering/" + width + ".png",
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
