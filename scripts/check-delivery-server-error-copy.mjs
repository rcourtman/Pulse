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
import { AlertDeliveryHealthCard } from '/src/features/alerts/AlertDeliveryHealthCard';
import { AlertDeliveryLogCard } from '/src/features/alerts/AlertDeliveryLogCard';
import '/src/index.css';
const noop = () => {};
render(() => <main class="p-4">
<AlertDeliveryHealthCard health={{status:'degraded',failed:1,deadLetter:0,failureClasses7d:{server_error:1},failureClassesAvailable:true}} unavailable={false} refreshing={false} onRefresh={noop}/>
<AlertDeliveryLogCard log={{entries:[{id:1,alertIds:['fixture-alert'],success:false,type:'webhook',outcome:'failed',failureClass:'server_error',timestamp:new Date().toISOString(),attempts:3}],completedRetentionDays:7,deadLetterRetentionDays:30}} unavailable={false} refreshing={false} onRefresh={noop} webhooks={[]}/>
</main>, document.getElementById('root'));
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
  mkdirSync("/tmp/pulse-server-error-copy", { recursive: true });
  for (const width of [1440, 900, 390]) {
    const page = await browser.newPage({ viewport: { width, height: 900 } });
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.goto("http://127.0.0.1:5197/qualification");
    await page.getByText('Destination server error', {exact:true}).waitFor();
    assert.match(await page.getByRole('alert').innerText(), /Check the destination service status and server logs/);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
    assert.deepEqual(errors, []);
    await page.screenshot({path: `/tmp/pulse-server-error-copy/${width}.png`, fullPage:true});
    await page.close();
  }
  console.log(JSON.stringify({result:'passed',viewports:[1440,900,390],scope:'Real health/log cards, Chromium, scripted props; no installed delivery claim'}));

} finally {
  await browser?.close();
  await server.close();
}
