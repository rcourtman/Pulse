const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules/history-touch-proof');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'),
    cacheDir: fs.mkdtempSync(path.join(artifacts, 'vite-')),
    server: { host: '127.0.0.1', port: 5227, strictPort: true },
  });
  const parent = false;
  let browser;
  const observations = [];
  const timestamp = Date.parse('2026-10-02T10:00:00Z');
  const point = (value, index = 0) => ({ timestamp: timestamp + index * 60000, value, min: value, max: value });
  const playwright = require('playwright/package.json').version;
  const expectedPlaywright = JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json', 'utf8')).packages['node_modules/@playwright/test'].version;
  assert.equal(playwright, expectedPlaywright);
  try {
    await server.listen();
    const configs = [
      ['chromium-desktop', chromium, false, false],
      ['chromium-phone', chromium, true, false],
      ['webkit-phone', webkit, true, true],
    ];
    for (const [name, engine, touch, dark] of configs) {
      if (parent && !touch) continue;
      browser = await engine.launch(engine === chromium ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const page = await browser.newPage({
        viewport: touch ? { width: 390, height: 844 } : { width: 1365, height: 900 },
        hasTouch: touch, isMobile: touch, timezoneId: 'Europe/London',
      });
      page.setDefaultTimeout(15000);
      const errors = [];
      const requests = [];
      page.on('pageerror', error => errors.push(error.message));
      let samples = [point(10), point(20, 1), point(30, 2)];
      await page.route('**/*', route => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5227') return route.abort();
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === '/api/metrics-store/history') {
          requests.push(Object.fromEntries(url.searchParams));
          return route.fulfill({ json: { points: samples, source: 'store' } });
        }
        return route.fulfill({ json: url.pathname === '/api/license/runtime-capabilities'
          ? { capabilities: [], limits: [], max_history_days: 7, hosted_mode: false, runtime: { build: 'community' }, blocked_capabilities: [] }
          : { data: [], enabled: false } });
      });
      await page.goto('http://127.0.0.1:5227/browser-tests/history-touch.html');
      if (dark) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const disk = page.getByRole('region', { name: 'Disk fixture' });
      const pool = page.getByRole('region', { name: 'Pool fixture' });
      await disk.getByRole('tab', { name: 'History', exact: true }).click();
      const busy = disk.getByRole('img', { name: 'Busy chart', exact: true });
      const description = chart => chart.evaluate(el => document.getElementById(el.getAttribute('aria-describedby')).textContent);
      await page.waitForFunction(() => [...document.querySelectorAll('canvas')].some(el => el.getAttribute('aria-label') === 'Busy chart' && document.getElementById(el.getAttribute('aria-describedby')).textContent.includes('3 data points')));
      const tooltip = chart => chart.locator('..').locator('[data-history-chart-tooltip]');
      const live = chart => chart.locator('..').locator('[aria-live="polite"]');
      async function inspect(chart, fraction) {
        await chart.scrollIntoViewIfNeeded();
        const box = await chart.boundingBox();
        // Percent labels reserve 40px at left; the time label reserves less than
        // 25px at right. These interior positions are nearest to each real sample.
        const x = box.x + 45 + (box.width - 70) * fraction;
        const y = box.y + box.height / 2;
        if (touch) await page.touchscreen.tap(x, y);
        else await page.mouse.move(x, y);
        await page.waitForTimeout(80);
        return (await tooltip(chart).count()) ? await tooltip(chart).textContent() : null;
      }
      async function record(phase, chart, text) {
        const dimensions = await page.evaluate(() => ({ scroll: document.documentElement.scrollWidth, width: innerWidth }));
        assert.ok(dimensions.scroll <= dimensions.width + 1, JSON.stringify(dimensions));
        observations.push({ name, browser: browser.version(), touch, dark, phase, text, description: await description(chart), dimensions });
        await page.screenshot({ path: path.join(artifacts, `${parent ? 'parent-' : ''}${name}-${phase}.png`) });
      }
      if (parent) {
        for (const [phase, fraction, expected] of [['first', 0.04, '10.0%'], ['middle', 0.5, '20.0%']]) {
          const text = await inspect(busy, fraction);
          if (phase === 'first') assert.ok(!text?.includes(expected), `predecessor unexpectedly inspected the first tap: ${text}`);
          await record(phase, busy, text);
        }
      } else {
        for (const [phase, fraction, expected] of [['first', 0.04, '10.0%'], ['middle', 0.5, '20.0%'], ['last', 0.96, '30.0%']]) {
          const text = await inspect(busy, fraction);
          assert.ok(text?.includes(expected), `${name} ${phase}: ${text}`);
          assert.equal(await live(busy).textContent(), '', 'pointer inspection must not speak keyboard announcements');
          assert.equal(await disk.locator('[data-history-chart-tooltip]').count(), 3, 'live I/O group shares inspection');
          await record(phase, busy, text);
        }
        if (touch) {
          const read = disk.getByRole('img', { name: 'Read chart', exact: true });
          assert.ok((await inspect(read, 0.5))?.includes('20.0 B/s'));
          assert.ok((await tooltip(busy).textContent()).includes('20.0%'), 'a peer blur must not clear the new group tap');
          assert.equal(await live(read).textContent(), '');
          await inspect(busy, 0.5);
          // A hybrid device may switch back to a real mouse after a tap.
          await busy.scrollIntoViewIfNeeded();
          const box = await busy.boundingBox();
          await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
          assert.ok((await tooltip(busy).textContent()).includes('20.0%'));
          await page.mouse.move(0, 0);
          assert.equal(await tooltip(busy).count(), 0);
        }
        await busy.focus();
        for (const [key, expected] of [['Home', '10.0%'], ['ArrowRight', '20.0%'], ['End', '30.0%']]) {
          await page.keyboard.press(key);
          assert.ok((await live(busy).textContent()).includes(expected));
        }
        await page.keyboard.press('Escape');
        assert.equal(await tooltip(busy).count(), 0);
        await page.getByRole('button', { name: 'After charts' }).focus();
        assert.equal(await live(busy).textContent(), '');

        if (touch && engine === chromium) {
          await busy.scrollIntoViewIfNeeded();
          await inspect(busy, 0.5);
          const box = await busy.boundingBox();
          const session = await page.context().newCDPSession(page);
          await busy.evaluate(el => {
            el.dataset.cancelCount = '0';
            el.addEventListener('pointercancel', () => el.dataset.cancelCount = String(Number(el.dataset.cancelCount) + 1));
          });
          const scrollMetrics = await page.evaluate(() => ({ before: scrollY, maximum: document.documentElement.scrollHeight - innerHeight }));
          assert.ok(scrollMetrics.maximum > scrollMetrics.before + 200, 'the fixture must have room to scroll');
          const before = scrollMetrics.before;
          const x = box.x + box.width / 2, y = box.y + box.height / 2;
          await session.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x, y, id: 1 }] });
          for (let step = 1; step <= 5; step++) {
            await session.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [{ x, y: y - step * 20, id: 1 }] });
            await page.waitForTimeout(50);
          }
          await session.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
          await page.waitForTimeout(300);
          const after = await page.evaluate(() => scrollY);
          fs.writeFileSync(path.join(artifacts, 'pan-control.json'), JSON.stringify({before, after, pointerCancels: await busy.getAttribute('data-cancel-count'), touchAction: await busy.evaluate(el => getComputedStyle(el).touchAction)},null,2));
          assert.ok(after > before + 40, `native pan must scroll: ${before} -> ${after}`);
          assert.ok(Number(await busy.getAttribute('data-cancel-count')) > 0);
          assert.equal(await tooltip(busy).count(), 0, 'native scrolling must not select a reading');
          observations.push({ name, phase: 'native-vertical-pan', before, after, maximum: scrollMetrics.maximum, pointerCancels: Number(await busy.getAttribute('data-cancel-count')), touchAction: await busy.evaluate(el => getComputedStyle(el).touchAction) });
          await session.detach();
        }

        await pool.getByRole('tab', { name: 'History', exact: true }).click();
        const usage = pool.getByRole('img', { name: 'Usage chart', exact: true });
        await usage.waitFor();
        await page.waitForFunction(() => [...document.querySelectorAll('canvas')].some(el => el.getAttribute('aria-label') === 'Usage chart' && document.getElementById(el.getAttribute('aria-describedby')).textContent.includes('3 data points')));
        assert.ok((await inspect(usage, 0.04))?.includes('10.0%'));
        await record('pool-first', usage, await tooltip(usage).textContent());
        assert.equal(await pool.getByLabel('Capacity history range').inputValue(), '7d');
        samples = [point(0)];
        await pool.getByLabel('Capacity history range').selectOption('24h');
        await page.waitForFunction(() => [...document.querySelectorAll('canvas')].some(el => el.getAttribute('aria-label') === 'Usage chart' && document.getElementById(el.getAttribute('aria-describedby')).textContent.includes('1 data point')));
        assert.ok((await inspect(usage, 0.5))?.includes('0.0%'));
        await record('pool-zero-singleton', usage, await tooltip(usage).textContent());
        samples = [];
        await pool.getByLabel('Capacity history range').selectOption('7d');
        await pool.getByText('No history samples in this time range.', { exact: true }).waitFor();
        assert.equal(await inspect(usage, 0.5), null);
        assert.equal(await live(usage).textContent(), '');
        await record('pool-empty', usage, null);
        assert.ok(requests.some(request => request.resourceType === 'disk' && request.resourceId === 'fixture-disk'));
        assert.ok(requests.some(request => request.resourceType === 'storage' && request.resourceId === 'fixture-pool'));
      }
      assert.deepEqual(errors, []);
      await browser.close(); browser = null;
    }
    fs.writeFileSync(path.join(artifacts, parent ? 'parent.json' : 'result.json'), JSON.stringify({ playwright, expectedPlaywright, parent, observations }, null, 2));
    console.log(JSON.stringify({ result: 'passed', parent, observations: observations.length }));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
