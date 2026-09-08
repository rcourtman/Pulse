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
import { useWebhookConfigState } from '/src/components/Alerts/useWebhookConfigState';
import { WebhookConfigForm } from '/src/components/Alerts/WebhookConfigForm';
import '/src/index.css';
render(() => {
 const state = useWebhookConfigState({webhooks:[], onAdd: data => window.saved = data, onUpdate:()=>{}, onDelete:()=>{}, onTest: (_id,data) => window.tested = data});
 state.openAddForm();
 state.setFormData(data => ({...data,name:'Synthetic destination',url:'https://example.invalid',service:'pushover'}));
 for (const [index,key] of ['app_token','user_token'].entries()) {
  state.addCustomFieldInput();
  state.updateCustomFieldInput(index,{key,value:'synthetic-'+index});
 }
 return <main class="p-4"><WebhookConfigForm {...state}/></main>;
}, document.getElementById('root'));
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
  mkdirSync("/tmp/pulse-webhook-parity", { recursive: true });
  for (const width of [1440, 900, 390]) {
    const page = await browser.newPage({ viewport: { width, height: 900 } });
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.goto("http://127.0.0.1:5197/qualification");
    await page.getByRole('button', {name:'Test', exact:true}).click();
    assert.deepEqual(await page.evaluate(() => window.tested.customFields), {token:'synthetic-0',user:'synthetic-1'});
    await page.screenshot({path: `/tmp/pulse-webhook-parity/${width}.png`, fullPage:true});
    await page.getByRole('button', {name:'Add Webhook', exact:true}).click();
    assert.deepEqual(await page.evaluate(() => window.saved.customFields), {token:'synthetic-0',user:'synthetic-1'});
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
    assert.deepEqual(errors, []);
    await page.close();
  }
  console.log(JSON.stringify({result:'passed',viewports:[1440,900,390],scope:'Real webhook form, Chromium, scripted props; no installed delivery claim'}));

} finally {
  await browser?.close();
  await server.close();
}
