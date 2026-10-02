const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules/history-keyboard-proof');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({ root, configFile: path.join(root, 'vite.config.ts'), cacheDir: fs.mkdtempSync(path.join(artifacts, 'vite-')), server: { host: '127.0.0.1', port: 5223, strictPort: true } });
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
        if (url.origin !== 'http://127.0.0.1:5223') return route.abort();
        if (!url.pathname.startsWith('/api/')) return route.continue();
        return route.fulfill({ json: url.pathname === '/api/license/runtime-capabilities' ? { capabilities: [], limits: [], max_history_days: 7, hosted_mode: false, runtime: { build: 'community' }, blocked_capabilities: [] } : { data: [], enabled: false } });
      });
      await page.goto('http://127.0.0.1:5223/browser-tests/history-keyboard.html');
      if (name === 'webkit') await page.evaluate(() => document.documentElement.classList.add('dark'));
      const chart = page.getByRole('img', { name: 'Usage chart', exact: true });
      await chart.waitFor();
      if (process.argv.includes('--parent')) {
        assert.equal(await chart.getAttribute('tabindex'), null);
        await page.getByRole('button', { name: 'Change target' }).focus();
        await page.keyboard.press('Tab');
        assert.equal(await page.evaluate(() => document.activeElement.textContent), 'After charts');
        await page.screenshot({ path: path.join(artifacts, 'parent-skipped.png'), fullPage: true });
        console.log('Parent reproduced: Tab skips both storage charts; no keyboard inspection');
        break;
      }
      assert.equal(await chart.getAttribute('tabindex'), '0');
      await page.getByRole('button', { name: 'Change target' }).focus();
      await page.keyboard.press('Tab');
      assert.equal(await chart.evaluate(el => el === document.activeElement), true);
      const live = page.locator('[aria-live="polite"]').first();
      for (const [phase, key, expected] of [['focus', null, '30.0%'], ['previous', 'ArrowLeft', '20.0%'], ['first', 'Home', '10.0%'], ['lower-bound', 'ArrowLeft', '10.0%'], ['last', 'End', '30.0%'], ['upper-bound', 'ArrowRight', '30.0%'], ['escape', 'Escape', '']]) {
        if (key) await page.keyboard.press(key);
        await page.waitForTimeout(60);
        assert.ok((await live.textContent()).includes(expected));
        if (!expected) assert.equal(await live.textContent(), '');
        else assert.ok((await page.locator('[data-history-chart-tooltip]').first().textContent()).includes(expected));
        if (phase === 'previous') {
          assert.equal(await page.locator('[data-history-chart-tooltip]').count(), 2);
          for (const tooltip of await page.locator('[data-history-chart-tooltip]').all()) {
            assert.ok(await tooltip.evaluate(el => el.scrollHeight <= el.clientHeight));
          }
          assert.equal(await page.locator('[aria-live="polite"]').nth(1).textContent(), '');
          const outline = await chart.evaluate(el => getComputedStyle(el).outlineWidth);
          assert.notEqual(outline, '0px');
          await page.screenshot({ path: path.join(artifacts, `${name}-keyboard.png`), fullPage: true });
        }
        observations.push({ name, version: browser.version(), phase, announcement: await live.textContent() });
      }
      await page.keyboard.press('Tab');
      assert.equal(await page.getByRole('img', { name: 'Read chart', exact: true }).evaluate(el => el === document.activeElement), true);
      await page.keyboard.press('Tab');
      assert.equal(await page.getByRole('button', { name: 'After charts' }).evaluate(el => el === document.activeElement), true);
      await page.getByRole('button', { name: 'Toggle empty' }).click();
      await chart.focus();
      await page.keyboard.press('End');
      assert.equal(await live.textContent(), '');
      assert.equal(await page.locator('[data-history-chart-tooltip]').count(), 0);
      const dimensions = await page.evaluate(() => ({ scroll: document.documentElement.scrollWidth, width: innerWidth }));
      assert.ok(dimensions.scroll <= dimensions.width + 1, JSON.stringify(dimensions));
      assert.deepEqual(errors, []);
      await browser.close(); browser = null;
    }
    fs.writeFileSync(path.join(artifacts, process.argv.includes('--parent') ? 'parent.json' : 'result.json'), JSON.stringify({ playwright: require('playwright/package.json').version, observations }, null, 2));
    console.log(JSON.stringify({ result: 'passed', states: observations.length }));
  } finally { if (browser) await browser.close(); await server.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
