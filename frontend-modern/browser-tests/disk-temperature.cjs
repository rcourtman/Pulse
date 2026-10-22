const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules/disk-temperature-proof');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({ root, configFile: path.join(root, 'vite.config.ts'), cacheDir: fs.mkdtempSync(path.join(artifacts, 'vite-')), server: { host: '127.0.0.1', port: 5225, strictPort: true } });
  let browser;
  const observations = [];
  try {
    await server.listen();
    for (const [name, engine, width, height] of [['chromium', chromium, 1365, 900], ['webkit', webkit, 390, 844]]) {
      browser = await engine.launch(name === 'chromium' ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const page = await browser.newPage({ viewport: { width, height }, hasTouch: name === 'webkit', isMobile: name === 'webkit' });
      const errors = [];
      page.on('pageerror', e => errors.push(e.message));
      await page.route('**/*', route => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5225') return route.abort();
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === '/api/metrics-store/history') {
          const q = url.searchParams;
          return route.fulfill({json: {resourceType: q.get('resourceType'), resourceId: q.get('resourceId'), range: q.get('range'), start: Date.now() - 86400000, end: Date.now(), source: 'store', metrics: {}}});
        }
        return route.fulfill({json: url.pathname === '/api/license/runtime-capabilities' ? {capabilities: [], limits: [], max_history_days: 7, hosted_mode: false, runtime: {build:'community'}, blocked_capabilities: []} : {data: [], enabled: false}});
      });
      await page.goto('http://127.0.0.1:5225/browser-tests/disk-temperature.html');
      if (name === 'webkit') await page.evaluate(() => document.documentElement.classList.add('dark'));
      const drawer = page.getByTestId('disk-drawer');
      // The SMART health card in front holds the current reading.
      const card = page.getByTestId('disk-drawer-health-card');
      const details = page.getByTestId('disk-technical-details');
      const fallback = page.getByText('Detailed SMART attributes are not available for this disk.', {exact:true});
      const value = text => card.getByText(text, {exact:true});
      const verdict = label => drawer.getByRole('img', {name: label, exact: true});
      await drawer.waitFor();
      await drawer.evaluate(el => { el.dataset.proofOwner = 'retained'; });
      async function state(phase, expected) {
        await page.getByText(expected, {exact:true}).first().waitFor();
        // One disk throughout: snapshots replace the drawer's content, not the drawer.
        assert.equal(await drawer.getAttribute('data-proof-owner'), 'retained');
        const dimensions = await page.evaluate(() => ({scroll:document.documentElement.scrollWidth, width:innerWidth}));
        assert.ok(dimensions.scroll <= dimensions.width + 1);
        await page.screenshot({path:path.join(artifacts, `${name}-${phase}.png`),fullPage:true});
        observations.push({name, version:browser.version(),phase,expected,dimensions});
      }
      await state('temperature-only','42°C');
      assert.equal(await value('Temperature').count(),1);
      assert.equal(await fallback.count(),0);
      assert.equal(await page.getByText('Power-On Time', {exact:true}).count(),0);
      assert.equal(await verdict('Running Hot').count(),0);
      await page.getByRole('button',{name:'Hot reading'}).click();
      await state('hot','65°C');
      assert.equal(await value('65°C').count(),1);
      assert.equal(await value('42°C').count(),0);
      // A hot disk is stated by its verdict, judged under the disk heat policy.
      assert.match(await verdict('Running Hot').getAttribute('class'), /red/);
      assert.match(await page.getByTestId('disk-drawer-exceptions').innerText(), /Running Hot/);
      await page.getByRole('button',{name:'Fahrenheit',exact:true}).click();
      await state('fahrenheit','149°F');
      assert.equal(await value('149°F').count(),1);
      assert.match(await verdict('Running Hot').getAttribute('class'), /red/);
      await page.getByRole('button',{name:'No reading',exact:true}).click();
      await state('missing','Detailed SMART attributes are not available for this disk.');
      assert.equal(await value('Temperature').count(),0);
      assert.equal(await card.getByText(/°[CF]$/).count(),0);
      assert.equal(await page.getByTestId('disk-drawer-notices').getAttribute('role'),'status');
      await page.getByRole('button',{name:'Extended SMART',exact:true}).click();
      await state('extended','42°C');
      assert.equal(await fallback.count(),0);
      assert.equal(await value('Temperature').count(),1);
      // The reported counters sit in Details, which opens on request.
      await details.locator('summary').click();
      await details.getByText('4 days', {exact:true}).waitFor();
      await details.getByText('Reallocated Sectors', {exact:true}).waitFor();
      assert.equal(await details.getByText('0', {exact:true}).count(),1);
      assert.equal(await details.getByText('Pending Sectors', {exact:true}).count(),0);
      await page.screenshot({path:path.join(artifacts, `${name}-extended-details.png`),fullPage:true});
      assert.deepEqual(errors,[]);
      await browser.close(); browser=null;
    }
    fs.writeFileSync(path.join(artifacts,'result.json'),JSON.stringify({playwright:require('playwright/package.json').version,observations},null,2));
    console.log(JSON.stringify({result:'passed',states:observations.length}));
  } finally { if(browser) await browser.close(); await server.close(); }
})().catch(e=>{console.error(e);process.exitCode=1});
