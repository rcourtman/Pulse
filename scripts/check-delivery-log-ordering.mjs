// Isolated real-browser component qualification; no installed backend or delivery claim.
import { createServer } from "../frontend-modern/node_modules/vite/dist/node/index.js";
import solid from "../frontend-modern/node_modules/vite-plugin-solid/dist/esm/index.mjs";
import { chromium, expect } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { resolve } from "node:path";
import { mkdirSync } from "node:fs";
import assert from "node:assert/strict";
const root = fileURLToPath(new URL('../frontend-modern', import.meta.url));
const evidence = process.env.PULSE_BROWSER_EVIDENCE_DIR || resolve(root, '../tmp/delivery-log-ordering');
mkdirSync(evidence, { recursive: true });
process.chdir(root);
const fixture = `
import { render } from 'solid-js/web';
import { NotificationsAPI } from '/src/api/notifications';
import { AlertsAPI } from '/src/api/alerts';
import { useAlertDestinationsTabState } from '/src/features/alerts/useAlertDestinationsTabState';
import { AlertDeliveryLogCard } from '/src/features/alerts/AlertDeliveryLogCard';
import '/src/index.css';
const pending = [];
const held = [];
NotificationsAPI.getHealth = async () => ({queue:{status:'healthy'}});
NotificationsAPI.getDeliveryLog = () => new Promise((resolve, reject) => pending.push({resolve, reject}));
AlertsAPI.getEvents = () => new Promise((resolve, reject) => held.push({resolve, reject}));
window.finishHeld = (i, status) => status === 'error' ? held[i].reject(new Error('scripted held failure')) : held[i].resolve([{id:i+1,type:'notification_deferred',alertId:status,occurredAt:'2026-09-06T08:01:00Z'}]);
NotificationsAPI.retryTerminalFailures = NotificationsAPI.dismissTerminalFailures = async () => ({affected: 1});
window.confirm = () => true;
window.finish = (i, status) => status === 'error' ? pending[i].reject(new Error('scripted offline')) : pending[i].resolve({entries:[{notificationId:status,type:'email',outcome:'sent',alertIds:[status],alertCount:1,attempts:1,success:true,timestamp:'2026-09-06T08:00:00Z'}],windowDays:30,completedRetentionDays:7,deadLetterRetentionDays:30});
window.count = () => pending.length;
function Fixture() {
const s = useAlertDestinationsTabState({emailConfig:()=>({}), appriseConfig:()=>({}), setAppriseConfig:()=>{}, configLoadError:()=> 'scripted config unavailable', isRetrying:()=>false, isLoadingDestinations:()=>false, onRetryLoad:()=>{}, webhooks:()=>[]});
return <main><h1>Delivery log ordering fixture</h1><button onClick={s.handleRetry}>Configuration Retry</button><output>{String(s.refreshingDeliveryLog())}</output><AlertDeliveryLogCard log={s.deliveryLog()} unavailable={s.deliveryLogUnavailable()} refreshing={s.refreshingDeliveryLog()} onRefresh={s.loadDeliveryLog} webhooks={[]} heldEvents={s.heldEvents()}/></main>;
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

  const results = [];
  for (const width of [1440,390]) {
    for (const old of ['old-attempt','error']) {
      const page = await browser.newPage({viewport:{width,height:900}});
      await page.goto('http://127.0.0.1:5197/qualification');
      await page.waitForFunction(() => window.count?.() === 1);
      await page.getByRole('button',{name:'Configuration Retry',exact:true}).click();
      await page.waitForFunction(() => window.count() === 2);
      await page.evaluate(() => window.finish(1,'current-attempt'));
      await page.getByText('current-attempt',{exact:true}).waitFor();
      // Held reads must not keep the attempt refresh control disabled.
      const refresh = page.getByRole('button', {name:'Refresh delivery status', exact:true});
      await expect(refresh).toBeEnabled();
      await page.evaluate(() => window.finishHeld(1, 'current-held'));
      await page.getByText('current-held', {exact:true}).waitFor();
      const before = await page.locator('main').innerText();
      await page.evaluate(s => { window.finish(0,s); window.finishHeld(0,s); },old);
      await page.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
      const after = await page.locator('main').innerText();
      assert.equal(after, before, 'old completion must preserve current evidence');
      assert.match(after, /current-attempt/);
      assert.match(after, /current-held/);
      results.push({width,old,before,after,currentEvidencePreserved:before===after});
      await page.screenshot({path:evidence+'/'+width+'-'+old+'.png'});
      await page.close();
    }
  }
  for (const width of [1440, 390]) {
    const page = await browser.newPage({viewport:{width,height:900}});
    await page.goto('http://127.0.0.1:5197/qualification');
    await page.waitForFunction(() => window.count?.() === 1);
    await page.getByRole('button',{name:'Configuration Retry',exact:true}).click();
    await page.waitForFunction(() => window.count() === 2);
    await page.evaluate(() => window.finish(0, 'old-attempt'));
    const refresh = page.getByRole('button',{name:'Refresh delivery status',exact:true});
    await expect(refresh).toBeDisabled();
    await expect(page.locator('output')).toHaveText('true');
    await expect(page.getByText('old-attempt',{exact:true})).toHaveCount(0);
    await page.evaluate(() => window.finish(1, 'error'));
    await expect(page.getByRole('alert')).toBeVisible();
    await expect(refresh).toBeEnabled();
    await page.screenshot({path:evidence+'/'+width+'-pending-unavailable.png'});
    results.push({width,scenario:'old completion leaves newest pending; newest failure is unavailable',passed:true});
    await page.close();
  }
  console.log(JSON.stringify({scope:'real Chromium, real caller/hook/card, scripted APIs; positive ordering assertions, not installed qualification',results},null,2));

} finally {
  await browser?.close();
  await server.close();
}
