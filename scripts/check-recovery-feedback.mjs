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
import { createSignal } from 'solid-js';
import { Router, Route } from '@solidjs/router';
import { AlertsAPI } from '/src/api/alerts';
import { NotificationsAPI } from '/src/api/notifications';
import { OverviewTab } from '/src/features/alerts/OverviewTab';
import { DestinationsTab } from '/src/features/alerts/tabs/DestinationsTab';
import { ToastContainer } from '/src/components/Toast/Toast';
import '/src/index.css';
window.health = 'degraded'; window.action = 'reject';
window.mutations = 0;
NotificationsAPI.getHealth = async () => {
  if(window.health === 'error') throw new Error('scripted offline');
  return {queue:{status:window.health, attentionRequired:window.health==='healthy'?0:2, failed:2,deadLetter:0}};
};
NotificationsAPI.retryTerminalFailures = NotificationsAPI.dismissTerminalFailures = async () => {
  window.mutations++;
  if(window.action==='reject') throw new Error('scripted rejected');
  return {affected:2};
};
NotificationsAPI.getDeliveryLog = async () => ({entries:[],windowDays:7,completedRetentionDays:7,deadLetterRetentionDays:30});
NotificationsAPI.getWebhooks = async () => [];
AlertsAPI.getEvents = async () => [];
AlertsAPI.getDeliveryDiagnoses = async () => [];
const noop = () => {};
function Overview() {return <OverviewTab overrides={[]} activeAlerts={{}} updateAlert={noop}
showQuickTip={()=>false} dismissQuickTip={noop} showAcknowledged={()=>true}
setShowAcknowledged={noop} alertsDisabled={()=>false}/>;}
function Destinations() {
const [pingUrl, setPingUrl] = createSignal('');
window.unsaved = false;
window.editedPingUrl = pingUrl;
return <DestinationsTab
emailConfig={()=>({enabled:false,provider:'smtp',from:'',server:'',username:'',password:'',port:587,to:[],tls:true,startTLS:true,replyTo:'',maxRetries:3,retryDelay:5,rateLimit:60})} setEmailConfig={noop}
appriseConfig={()=>({enabled:false,mode:'cli',targetsText:'',configKey:'',serverUrl:'',timeoutSeconds:30,apiKey:'',apiKeyHeader:'X-API-KEY',hasApiKey:false,skipTlsVerify:false,cliPath:'apprise'})} setAppriseConfig={noop}
configLoadError={()=>null} isRetrying={()=>false} isLoadingDestinations={()=>false} onRetryLoad={noop}
webhooks={()=>[]} setHasUnsavedChanges={(value)=>{window.unsaved=value;}} deadManPingUrl={pingUrl} setDeadManPingUrl={setPingUrl}
pushMinimumSeverity={()=>'all'} setPushMinimumSeverity={noop}/>;}
function Fixture() {return <main class="p-4"><ToastContainer/>{location.pathname.endsWith('destinations')?<Destinations/>:<Overview/>}</main>;}
render(()=><Router><Route path="/qualification/*" component={Fixture}/></Router>,document.getElementById('root'));
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
      name: "recovery-fixture",
      configureServer(s) {
        s.middlewares.use((req, res, next) => {
          if (req.url?.startsWith("/qualification")) {
            res.setHeader("Content-Type", "text/html");
            res.end(
              '<div id="root"></div><script type="module" src="/recovery-fixture.tsx"></script>',
            );
          } else next();
        });
      },
      resolveId(id) {
        if (id === "/recovery-fixture.tsx") return id;
      },
      load(id) {
        if (id === "/recovery-fixture.tsx") return fixture;
      },
    },
  ],
  resolve: { alias: { "@": resolve(root, "src") } },
  server: { host: "127.0.0.1", port: 5199, strictPort: true },
});

let browser;
const output = resolve("../docs/qualification/recovery-feedback");
try {
  await server.listen();
  browser = await chromium.launch({ headless: true });
  mkdirSync(output, { recursive: true });
  let cases = 0;
  let unsavedEditCases = 0;
  for (const width of [1440, 900, 390])
    for (const surface of ["overview", "destinations"])
      for (const theme of ["light", "dark"]) {
        const page = await browser.newPage({
          viewport: { width, height: 1000 },
        });
        const errors = [];
        page.on("pageerror", (e) => errors.push(e.message));
        await page.route("http://127.0.0.1:5199/api/**", (route) =>
          route.fulfill({ json: [] }),
        );
        await page.goto("http://127.0.0.1:5199/qualification/" + surface);
        await page.evaluate(
          (theme) =>
            document.documentElement.classList.toggle("dark", theme === "dark"),
          theme,
        );
        const feedback = page.getByRole("region", {
          name: "Notification recovery feedback",
        });
        const retry = page.getByRole("button", {
          name: "Retry retained deliveries",
          exact: true,
        });
        const dismiss = page.getByRole("button", {
          name: "Dismiss retained failures",
          exact: true,
        });
        const healthCard = page.getByRole("alert").filter({ has: retry });
        let accept = true;
        page.on("dialog", (dialog) =>
          accept ? dialog.accept() : dialog.dismiss(),
        );
        await retry.waitFor().catch(async (error) => {
          console.error(
            JSON.stringify({
              surface,
              width,
              theme,
              errors,
              body: await page.locator("body").innerText(),
            }),
          );
          await page.screenshot({
            path: resolve(output, "failed.png"),
            fullPage: true,
          });
          throw error;
        });
        // A synthetic unsaved value, never submitted to a backend.
        const editedUrl = "https://example.invalid/unsaved-recovery-check";
        const pingInput = page.getByLabel(
          "Healthchecks-compatible success ping URL",
          { exact: true },
        );
        const assertEditRetained = async () => {
          if (surface !== "destinations") return;
          assert.equal(await pingInput.inputValue(), editedUrl);
          assert.equal(await page.evaluate(() => window.editedPingUrl()), editedUrl);
          assert.equal(await page.evaluate(() => window.unsaved), true);
        };
        if (surface === "destinations") {
          await pingInput.fill(editedUrl);
          await assertEditRetained();
        }
        await page.clock.install();
        // Keyboard confirmation, genuine toast expiry, durable equivalent.
        await retry.focus();
        await page.keyboard.press("Enter");
        await feedback
          .getByText("Unable to retry retained notification deliveries.", {
            exact: true,
          })
          .waitFor();
        await page.clock.fastForward(12000);
        await page.clock.runFor(350);
        assert.match(await feedback.innerText(), /Unable to retry/);
        assert.equal(
          await page
            .getByText("Unable to retry retained notification deliveries.", {
              exact: true,
            })
            .count(),
          1,
        );
        await assertEditRetained();
        // Cancellation leaves the previous message and does not submit.
        const before = await page.evaluate(() => window.mutations);
        accept = false;
        await dismiss.click();
        assert.equal(await page.evaluate(() => window.mutations), before);
        assert.match(await feedback.innerText(), /Unable to retry/);
        await assertEditRetained();
        accept = true;
        await dismiss.focus();
        await page.keyboard.press("Enter");
        await feedback
          .getByText("Unable to dismiss retained notification failures.", {
            exact: true,
          })
          .waitFor();
        await page.evaluate(() => {
          window.action = "accept";
          window.health = "error";
        });
        await retry.click();
        await healthCard
          .getByRole("button", { name: "Refresh delivery status", exact: true })
          .waitFor();
        assert.equal(await feedback.getByRole("button").count(), 0);
        assert.equal(
          await page
            .getByText("Unable to retry retained notification deliveries.", {
              exact: true,
            })
            .count(),
          0,
        );
        await assertEditRetained();
        // Recover the unavailable snapshot; an accepted mutation did not clear it.
        await page.evaluate(() => {
          window.health = "degraded";
          window.action = "reject";
        });
        await healthCard
          .getByRole("button", { name: "Refresh delivery status", exact: true })
          .click();
        await dismiss.click();
        await feedback
          .getByText("Unable to dismiss retained notification failures.", {
            exact: true,
          })
          .waitFor();
        if (surface === "destinations") {
          await page.evaluate(() => {
            window.health = "healthy";
          });
          await healthCard
            .getByRole("button", {
              name: "Refresh delivery status",
              exact: true,
            })
            .click();
          await healthCard.waitFor({ state: "detached" });
          assert.match(await feedback.innerText(), /Unable to dismiss/);
        }
        assert.equal(
          await feedback.evaluate((el) => {
            const range = document.createRange();
            range.selectNodeContents(el);
            return [...range.getClientRects()].every(
              (r) => r.left >= 0 && r.right <= innerWidth,
            );
          }),
          true,
          "feedback text and controls fit viewport",
        );
        await page.mouse.move(0, 0);
        await page.clock.fastForward(12000);
        await page.clock.runFor(400);
        assert.match(await feedback.innerText(), /Unable to dismiss/);
        await page.screenshot({
          path: resolve(output, width + "-" + surface + "-" + theme + ".png"),
          fullPage: true,
        });
        const clear = feedback.getByRole("button", {
          name: "Clear recovery message",
        });
        await clear.focus();
        await page.keyboard.press("Enter");
        assert.equal(await feedback.getByRole("button").count(), 0);
        assert.equal(
          await feedback.evaluate((el) => document.activeElement === el),
          true,
        );
        await assertEditRetained();
        assert.deepEqual(errors, []);
        await page.close();
        cases++;
        if (surface === "destinations") unsavedEditCases++;
      }
  console.log(
    JSON.stringify({
      result: "passed",
      cases,
      unsavedEditCases,
      viewports: [1440, 900, 390],
      scope:
        "Real OverviewTab and DestinationsTab, shared toast and feedback in Chromium; scripted API only, not installed delivery or recipient receipt.",
    }),
  );
} finally {
  await browser?.close();
  await server.close();
}
