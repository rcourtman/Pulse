const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules/history-sparse-proof');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'),
    cacheDir: fs.mkdtempSync(path.join(artifacts, 'vite-')),
    server: { host: '127.0.0.1', port: 5226, strictPort: true },
  });
  const parent = process.argv.includes('--parent');
  const timestamp = 1790942400000;
  const point = (value, offset = 0) => ({ timestamp: timestamp + offset, value, min: value, max: value });
  const observations = [];
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, height] of [['chromium', chromium, 1365, 900], ['webkit', webkit, 390, 844]]) {
      browser = await engine.launch(name === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true });
      const page = await browser.newPage({
        viewport: { width, height }, hasTouch: name === 'webkit', isMobile: name === 'webkit',
      });
      const errors = [];
      page.on('pageerror', error => errors.push(error.message));
      let samples = [point(42)], calls = 0;
      await page.route('**/*', route => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5226') return route.abort();
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname.includes('/metrics-store/history')) {
          calls++;
          return route.fulfill({ json: { points: samples, source: 'store' } });
        }
        return route.fulfill({ json: url.pathname === '/api/license/runtime-capabilities'
          ? { capabilities: [], limits: [], max_history_days: 7, hosted_mode: false, runtime: { build: 'community' }, blocked_capabilities: [] }
          : { data: [], enabled: false } });
      });
      await page.clock.install();
      await page.goto('http://127.0.0.1:5226/browser-tests/history-sparse.html');
      if (name === 'webkit') await page.evaluate(() => document.documentElement.classList.add('dark'));
      const chart = page.getByRole('img', { name: 'Usage chart', exact: true });
      await chart.waitFor();
      const description = page.locator('#' + await chart.getAttribute('aria-describedby'));
      const tooltip = page.locator('[data-history-chart-tooltip]');
      const live = page.locator('[aria-live="polite"]');
      async function state(phase, expected) {
        await page.waitForFunction(({ id, expected }) => document.getElementById(id).textContent.includes(expected),
          { id: await chart.getAttribute('aria-describedby'), expected });
        await page.clock.runFor(100);
        // Count actual blue series pixels, not mocked drawing calls or tooltip DOM.
        const pixels = await chart.evaluate(canvas => {
          const ctx = canvas.getContext('2d');
          const data = ctx.getImageData(0, 0, canvas.width, canvas.height).data;
          let count = 0;
          for (let i = 0; i < data.length; i += 4) {
            if (data[i] >= 40 && data[i] <= 80 && data[i + 1] >= 100 && data[i + 1] <= 150 && data[i + 2] >= 220 && data[i + 2] <= 255 && data[i + 3] >= 80) count++;
          }
          return count;
        });
        const dimensions = await page.evaluate(() => ({ scroll: document.documentElement.scrollWidth, width: innerWidth }));
        assert.ok(dimensions.scroll <= dimensions.width + 1, JSON.stringify(dimensions));
        const screenshot = `${parent ? 'parent-' : ''}${name}-${phase}.png`;
        await page.screenshot({ path: path.join(artifacts, screenshot), fullPage: true });
        observations.push({ name, version: browser.version(), phase, description: await description.textContent(), pixels, calls, dimensions, screenshot });
        return pixels;
      }
      const pixels = await state('single', '1 data point');
      const box = await chart.boundingBox();
      assert.ok(box);
      if (parent) {
        assert.equal(pixels, 0, 'predecessor singleton has no visible series pixels');
        await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
        await page.clock.runFor(100);
        assert.equal(await tooltip.count(), 0, 'predecessor pointer rejects singleton');
        observations.at(-1).pointerTooltipCount = await tooltip.count();
      } else {
        assert.ok(pixels >= 30, 'singleton has a visible marker without hover');
        for (const fraction of [0.25, 0.5, 0.75]) {
          await page.mouse.move(box.x + box.width * fraction, box.y + box.height / 2);
          await page.clock.runFor(100);
          assert.equal(await tooltip.count(), 1);
          assert.ok((await tooltip.textContent()).includes('42.0%'));
          assert.ok((await tooltip.textContent()).includes(await page.evaluate(ts => new Date(ts).toLocaleString(), timestamp)));
          assert.equal(await live.textContent(), '', 'pointer does not announce mouse movements');
        }
        await state('pointer', '1 data point');
        await page.mouse.move(0, 0);
        await page.getByRole('button', { name: 'Before chart' }).focus();
        await page.keyboard.press('Tab');
        assert.equal(await chart.evaluate(el => el === document.activeElement), true);
        for (const key of ['Home', 'ArrowLeft', 'ArrowRight', 'End']) {
          await page.keyboard.press(key);
          await page.clock.runFor(100);
          assert.ok((await live.textContent()).includes('42.0%'));
        }
        await page.keyboard.press('Escape');
        assert.equal(await tooltip.count(), 0);
        await page.keyboard.press('Tab');
        assert.equal(await page.getByRole('button', { name: 'After chart' }).evaluate(el => el === document.activeElement), true);

        samples = [point(0)];
        await page.clock.runFor(10000);
        assert.ok(await state('zero', '0.0%') >= 30, 'measured zero keeps its marker');
        await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
        await page.clock.runFor(100);
        assert.ok((await tooltip.textContent()).includes('0.0%'));

        samples = [point(10), point(20, 60000), point(30, 120000)];
        await page.mouse.move(0, 0);
        await page.clock.runFor(10000);
        assert.ok(await state('multiple', '3 data points') > 100, 'normal series still draws');
        await chart.focus();
        await page.keyboard.press('Home');
        await page.clock.runFor(100);
        assert.ok((await live.textContent()).includes('10.0%'));
        await page.keyboard.press('ArrowRight');
        await page.clock.runFor(100);
        assert.ok((await live.textContent()).includes('20.0%'));
        await page.keyboard.press('Tab');

        samples = [];
        await page.clock.runFor(10000);
        assert.equal(await state('empty', 'No 1-hour'), 0);
        assert.equal(await tooltip.count(), 0);
        assert.equal(await live.textContent(), '');
        assert.equal(await page.getByText('No history samples in this time range.', { exact: true }).count(), 1);

        samples = [point(70, 180000)];
        await page.clock.runFor(10000);
        assert.ok(await state('recovered-single', '1 data point') >= 30);
        await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
        await page.clock.runFor(100);
        assert.ok((await tooltip.textContent()).includes('70.0%'));
      }
      assert.deepEqual(errors, []);
      await browser.close(); browser = null;
    }
    fs.writeFileSync(path.join(artifacts, parent ? 'parent.json' : 'result.json'), JSON.stringify({
      playwright: require('playwright/package.json').version, parent, observations,
    }, null, 2));
    console.log(JSON.stringify({ result: 'passed', parent, states: observations.length }));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
