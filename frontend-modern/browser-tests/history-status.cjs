const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules/history-status-proof');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({ root, configFile: path.join(root, 'vite.config.ts'), cacheDir: fs.mkdtempSync(path.join(artifacts, 'vite-')), server: { host: '127.0.0.1', port: 5224, strictPort: true } });
  let browser;
  const observations = [];
  const parent = process.argv.includes('--parent');
  try {
    await server.listen();
    for (const [name, engine, width, height] of [['chromium', chromium, 1365, 900], ['webkit', webkit, 390, 844]]) {
      browser = await engine.launch(name === 'chromium' ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const page = await browser.newPage({ viewport: { width, height }, hasTouch: name === 'webkit', isMobile: name === 'webkit' });
      const errors = [];
      page.on('pageerror', e => errors.push(e.message));
      let mode = 'empty', calls = 0;
      const pending = [];
      await page.route('**/*', route => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5224') return route.abort();
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname.includes('/metrics-store/history')) {
          calls++;
          if (mode === 'pending') { pending.push(route); return; }
          if (mode === 'failed') return route.fulfill({status: 503, json: {error: 'Synthetic unavailable'}});
          return route.fulfill({json: {points: mode === 'empty' ? [] : [0,1,2].map(i => ({timestamp: 1790942400000 + i*60000, value: 20+i*10, min: 20+i*10, max: 20+i*10})), source: 'store'}});
        }
        return route.fulfill({json: url.pathname === '/api/license/runtime-capabilities' ? {capabilities: [], limits: [], max_history_days: 7, hosted_mode: false, runtime: {build:'community'}, blocked_capabilities: []} : {data: [], enabled: false}});
      });
      await page.clock.install();
      await page.goto('http://127.0.0.1:5224/browser-tests/history-status.html');
      if (name === 'webkit') await page.evaluate(() => document.documentElement.classList.add('dark'));
      const chart = page.getByRole('img', {name:'Usage chart', exact:true});
      await chart.waitFor();
      const description = page.locator('#' + await chart.getAttribute('aria-describedby'));
      async function state(phase, expected) {
        await page.waitForFunction(({id, expected}) => document.getElementById(id).textContent.includes(expected), {id: await chart.getAttribute('aria-describedby'), expected});
        const actual = await description.textContent();
        const dimensions = await page.evaluate(() => ({scroll:document.documentElement.scrollWidth, width:innerWidth}));
        assert.ok(dimensions.scroll <= dimensions.width + 1);
        await page.screenshot({path:path.join(artifacts, `${parent?'parent-':''}${name}-${phase}.png`),fullPage:true});
        observations.push({name, version:browser.version(),phase,actual,dimensions,calls});
      }
      await state('empty', 'No 1-hour');
      assert.equal(await page.getByText(parent ? 'Collecting data... History will appear here.' : 'No history samples in this time range.', {exact:true}).count(),1);
      mode = 'populated';
      await page.clock.runFor(10_000);
      await state('loaded','40.0%');
      mode = 'failed';
      await page.clock.runFor(10_000);
      if (parent) {
        await page.waitForTimeout(100);
        assert.ok(!(await description.textContent()).includes('Could not refresh'));
        assert.equal(await page.getByText('Could not refresh history. Showing the last successful result.',{exact:true}).count(),0);
        await state('silent-failure','40.0%');
        await browser.close(); browser = null; break;
      }
      await state('refresh-failed','Could not refresh history');
      assert.ok((await description.textContent()).includes('40.0%'));
      assert.ok((await page.getByRole('status', {name:'History refresh status'}).textContent()).includes('last successful result'));
      mode = 'pending';
      await page.clock.runFor(10_000);
      await state('retry-pending','Could not refresh history');
      for (let i=0; !pending.length && i<100; i++) await new Promise(r=>setTimeout(r,20));
      assert.ok(pending.length > 0, 'pending refresh observed');
      for (const route of pending.splice(0)) await route.fulfill({json:{points:[],source:'store'}});
      await state('recovered-empty','No 1-hour');
      assert.equal(await page.getByRole('status', {name:'History refresh status'}).textContent(),'');
      mode = 'failed';
      await page.clock.runFor(10_000);
      await state('empty-refresh-failed','Could not refresh history');
      mode = 'pending';
      await page.getByRole('button',{name:'Select pool b'}).click();
      await state('target-loading','Loading');
      assert.equal(await page.getByRole('status', {name:'History refresh status'}).textContent(),'');
      for (let i=0; !pending.length && i<100; i++) await new Promise(r=>setTimeout(r,20));
      assert.ok(pending.length > 0, 'pending refresh observed');
      for (const route of pending.splice(0)) await route.fulfill({status:503,json:{error:'Synthetic unavailable'}});
      await state('initial-failed','could not be loaded');
      assert.equal(await page.getByRole('status', {name:'History refresh status'}).textContent(),'');
      assert.deepEqual(errors,[]);
      await browser.close(); browser=null;
    }
    fs.writeFileSync(path.join(artifacts,parent?'parent.json':'result.json'),JSON.stringify({playwright:require('playwright/package.json').version,observations},null,2));
    console.log(JSON.stringify({result:'passed',states:observations.length,parent}));
  } finally { if(browser) await browser.close(); await server.close(); }
})().catch(e=>{console.error(e);process.exitCode=1});
