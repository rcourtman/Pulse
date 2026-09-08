// Isolated real-browser component qualification; no installed backend or delivery claim.
import { build, preview } from "../frontend-modern/node_modules/vite/dist/node/index.js";
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
import { NodeDrawerOverview } from '/src/components/Workloads/NodeDrawerOverview';
import '/src/index.css';
const base = {id:'lab-pve1',name:'pve1',instance:'lab',status:'online',type:'node',cpu:0,memory:{total:1024,used:256,free:768,usage:25},disk:{total:1024,used:256,free:768,usage:25},uptime:3600,loadAverage:[],kernelVersion:'6.8.12',pveVersion:'9.0.1',cpuInfo:{model:'CPU',cores:4,sockets:1,mhz:'2400'},lastSeen:new Date().toISOString(),connectionHealth:'healthy'};
const states = {
 not_checked:{pendingUpdatesStatus:'not_checked',pendingUpdatesReason:'permission_denied'},
 unavailable:{pendingUpdatesStatus:'unavailable',pendingUpdatesReason:'permission_denied'},
 stale:{pendingUpdates:1,pendingUpdatesStatus:'stale',pendingUpdatesReason:'permission_denied',pendingUpdatesCheckedAt:new Date().toISOString()},
 zero:{pendingUpdates:0,pendingUpdatesStatus:'checked',pendingUpdatesCheckedAt:new Date().toISOString()},
 positive:{pendingUpdates:1,pendingUpdatesStatus:'checked',pendingUpdatesCheckedAt:new Date().toISOString()}
};
const [state,setState] = createSignal('unavailable');
render(() => <main class="p-4"><nav>{Object.keys(states).map(key => <button class="p-2" onClick={() => setState(key)}>{key}</button>)}</nav><NodeDrawerOverview node={{...base,...states[state()]}} /></main>, document.getElementById('root'));
`;
const config = {
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
      name: "update-access-fixture",
      resolveId(id) {
        if (id === "/update-access-fixture.tsx" || id === resolve(root, "qualification.html")) return id;
      },
      load(id) {
        if (id === "/update-access-fixture.tsx") return fixture;
        if (id === resolve(root, "qualification.html")) return '<div id="root"></div><script type="module" src="/update-access-fixture.tsx"></script>';
      },
    },
  ],
  resolve: { alias: { "@": resolve(root, "src") } },
  build: { target: "esnext", outDir: "/tmp/pulse-update-access-build", emptyOutDir: true, rollupOptions: { input: resolve(root, "qualification.html") } },
};
await build(config);
const server = await preview({root, configFile:false, build:{outDir:"/tmp/pulse-update-access-build"}, preview:{host:"127.0.0.1",port:5198,strictPort:true}});

let browser;
try {

  browser = await chromium.launch({ headless: true });
  mkdirSync("/tmp/pulse-update-access-copy", { recursive: true });
  for (const width of [1440, 900, 390]) {
    const page = await browser.newPage({ viewport: { width, height: 900 } });
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.goto("http://127.0.0.1:5198/qualification.html");
    for (const state of ['unavailable', 'stale', 'zero', 'positive', 'not_checked', 'unavailable']) {
      await page.getByRole('button', {name:state,exact:true}).click();
      const expected = state === 'not_checked' ? 'Not checked · Update check access denied' : state === 'unavailable' ? 'Unavailable · Update check access denied' : state === 'stale' ? /1 pending · stale/ : state === 'zero' ? /No pending updates · checked/ : /1 pending · checked/;
      await page.getByText(expected,{exact:typeof expected === 'string'}).waitFor();
      if (state === 'stale') assert.match(await page.getByText(/1 pending · stale/).getAttribute('title'), /Update check access denied/);
      assert.equal(await page.getByText('Sys.Audit permission required',{exact:false}).count(),0);
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
      assert.deepEqual(errors, []);
      await page.screenshot({path: `/tmp/pulse-update-access-copy/${width}-${state}.png`, fullPage:true});
    }
    await page.close();
  }
  console.log(JSON.stringify({result:'passed',viewports:[1440,900,390],scope:'Real NodeDrawerOverview, Chromium, scripted props; no installed delivery claim'}));

} finally {
  await browser?.close();
  await new Promise(resolve => server.httpServer.close(resolve));
}
