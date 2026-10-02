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
  const parent = process.argv.includes('--parent');
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
        return route.fulfill({json: url.pathname === '/api/license/runtime-capabilities' ? {capabilities: [], limits: [], max_history_days: 7, hosted_mode: false, runtime: {build:'community'}, blocked_capabilities: []} : {data: [], enabled: false}});
      });
      await page.goto('http://127.0.0.1:5225/browser-tests/disk-temperature.html');
      if (name === 'webkit') await page.evaluate(() => document.documentElement.classList.add('dark'));
      const fallback = page.getByText('Detailed SMART attributes are not available for this disk.', {exact:true});
      const value = text => page.getByText(text, {exact:true});
      async function state(phase, expected) {
        await value(expected).waitFor();
        const dimensions = await page.evaluate(() => ({scroll:document.documentElement.scrollWidth, width:innerWidth}));
        assert.ok(dimensions.scroll <= dimensions.width + 1);
        await page.screenshot({path:path.join(artifacts, `${parent?'parent-':''}${name}-${phase}.png`),fullPage:true});
        observations.push({name, version:browser.version(),phase,expected,dimensions});
      }
      if (parent) {
        await state('temperature-hidden', 'Detailed SMART attributes are not available for this disk.');
        assert.equal(await value('42°C').count(),0);
      } else {
        await state('temperature-only','42°C');
        assert.equal(await fallback.count(),0);
        assert.equal(await value('Power-On Time').count(),0);
        assert.match(await value('42°C').getAttribute('class'), /text-green-600/);
        await page.getByRole('button',{name:'Hot reading'}).click();
        await state('hot','65°C');
        assert.match(await value('65°C').getAttribute('class'), /text-red-600/);
        await page.getByRole('button',{name:'Fahrenheit',exact:true}).click();
        await state('fahrenheit','149°F');
        assert.match(await value('149°F').getAttribute('class'), /text-red-600/);
        await page.getByRole('button',{name:'No reading',exact:true}).click();
        await state('missing','Detailed SMART attributes are not available for this disk.');
        assert.equal(await value('Temperature').count(),0);
        assert.equal(await fallback.getAttribute('role'),'status');
        await page.getByRole('button',{name:'Extended SMART',exact:true}).click();
        await state('extended','42°C');
        await value('4 days').waitFor();
        await value('Reallocated Sectors').waitFor();
        assert.equal(await value('0').count(),1);
        assert.equal(await value('Temperature').count(),1);
      }
      assert.deepEqual(errors,[]);
      await browser.close(); browser=null;
    }
    fs.writeFileSync(path.join(artifacts,parent?'parent.json':'result.json'),JSON.stringify({playwright:require('playwright/package.json').version,observations},null,2));
    console.log(JSON.stringify({result:'passed',states:observations.length,parent}));
  } finally { if(browser) await browser.close(); await server.close(); }
})().catch(e=>{console.error(e);process.exitCode=1});
