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
          // Each drawer reads every series of its one target in one request;
          // the reply window is the stored samples' own span.
          const q = url.searchParams;
          const times = samples.map(sample => sample.timestamp);
          const metrics = q.get('resourceType') === 'disk'
            ? Object.fromEntries(['smart_temp', 'diskread', 'diskwrite', 'disk'].map(metric => [metric, samples]))
            : { usage: samples };
          return route.fulfill({ json: {
            resourceType: q.get('resourceType'), resourceId: q.get('resourceId'), range: q.get('range'),
            start: times.length ? Math.min(...times) : Date.now() - 86400000, end: times.length ? Math.max(...times) : Date.now(),
            metrics, source: 'store',
          } });
        }
        return route.fulfill({ json: url.pathname === '/api/license/runtime-capabilities'
          ? { capabilities: [], limits: [], max_history_days: 7, hosted_mode: false, runtime: { build: 'community' }, blocked_capabilities: [] }
          : { data: [], enabled: false } });
      });
      await page.goto('http://127.0.0.1:5227/browser-tests/history-touch.html');
      if (dark) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const disk = page.getByRole('region', { name: 'Disk fixture' });
      const pool = page.getByRole('region', { name: 'Pool fixture' });
      const busy = disk.getByRole('img', { name: 'Busy history', exact: true });
      const panel = chart => chart.locator('xpath=ancestor::*[@data-testid="guest-history-group-chart"][1]');
      // The plot's description names the inspected observation, or the count.
      const description = chart => chart.evaluate(el => document.getElementById(el.getAttribute('aria-describedby').split(' ')[0]).textContent);
      const readoutTime = chart => panel(chart).getByTestId('guest-history-hover-time').textContent();
      const settled = (label, count) => page.waitForFunction(([label, count]) => [...document.querySelectorAll('[data-testid="guest-history-plot"]')].some(el => el.getAttribute('aria-label') === label && document.getElementById(el.getAttribute('aria-describedby').split(' ')[0]).textContent.includes(count)), [label, count]);
      // A phone folds the disk's charts after the first behind one button.
      if (touch) {
        await settled('Temperature history', '3 stored observation times');
        await disk.getByTestId('history-phone-toggle').click();
      }
      await settled('Busy history', '3 stored observation times');
      async function inspect(chart, fraction) {
        await chart.scrollIntoViewIfNeeded();
        const box = await chart.boundingBox();
        // The plot spans 34-352 of its 360-unit viewBox; the reply window is
        // the samples' span, so these fractions sit on the real samples.
        const x = box.x + (box.width * (34 + fraction * 318)) / 360;
        const y = box.y + box.height / 2;
        if (touch) await page.touchscreen.tap(x, y);
        else await page.mouse.move(x, y);
        await page.waitForTimeout(80);
        return description(chart);
      }
      async function record(phase, chart, text) {
        const dimensions = await page.evaluate(() => ({ scroll: document.documentElement.scrollWidth, width: innerWidth }));
        assert.ok(dimensions.scroll <= dimensions.width + 1, JSON.stringify(dimensions));
        // An empty range has no observation time to name.
        const readout = (await panel(chart).getByTestId('guest-history-hover-time').count()) ? await readoutTime(chart) : null;
        observations.push({ name, browser: browser.version(), touch, dark, phase, text, readout, dimensions });
        await page.screenshot({ path: path.join(artifacts, `${name}-${phase}.png`) });
      }
      for (const [phase, fraction, expected] of [['first', 0, 'Busy 10.0%'], ['middle', 0.5, 'Busy 20.0%'], ['last', 1, 'Busy 30.0%']]) {
        const text = await inspect(busy, fraction);
        assert.ok(text.includes(expected), `${name} ${phase}: ${text}`);
        assert.match(await readoutTime(busy), /^Stored · /, 'the readout names an inspected time');
        // Each panel inspects on its own: the I/O panel keeps its latest reading.
        assert.match(await readoutTime(disk.getByRole('img', { name: 'Disk I/O history', exact: true })), /^Latest stored · /);
        await record(phase, busy, text);
      }
      if (touch) {
        const read = disk.getByRole('img', { name: 'Disk I/O history', exact: true });
        assert.match(await inspect(read, 0.5), /Read 20(\.0)? B\/s/);
        assert.match(await readoutTime(read), /^Stored · /);
        // The touched panel exposes its scrubber for touch inspection.
        assert.ok(await panel(read).getByRole('slider').isVisible());
        await inspect(busy, 0.5);
        // A hybrid device may switch back to a real mouse after a tap.
        await busy.scrollIntoViewIfNeeded();
        const box = await busy.boundingBox();
        await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
        assert.ok((await description(busy)).includes('Busy 20.0%'));
        await page.mouse.move(0, 0);
        assert.ok((await description(busy)).startsWith('3 stored observation times'));
      }
      // The keyboard steps through the stored observations, then lets go.
      const scrubber = panel(busy).getByRole('slider', { name: 'Inspect Busy history' });
      await scrubber.focus();
      for (const [key, expected] of [['Home', 'Busy 10.0%'], ['ArrowRight', 'Busy 20.0%'], ['End', 'Busy 30.0%']]) {
        await page.keyboard.press(key);
        assert.ok((await scrubber.getAttribute('aria-valuetext')).includes(expected));
        assert.ok((await description(busy)).includes(expected));
      }
      await page.getByRole('button', { name: 'After charts' }).focus();
      assert.ok((await description(busy)).startsWith('3 stored observation times'));
      assert.match(await readoutTime(busy), /^Latest stored · /);

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
        const touchAction = await busy.evaluate(el => getComputedStyle(el).touchAction);
        fs.writeFileSync(path.join(artifacts, 'pan-control.json'), JSON.stringify({before, after, pointerCancels: await busy.getAttribute('data-cancel-count'), touchAction},null,2));
        assert.ok(after > before + 40, `native pan must scroll: ${before} -> ${after}`);
        assert.ok(Number(await busy.getAttribute('data-cancel-count')) > 0);
        assert.ok((await description(busy)).startsWith('3 stored observation times'), 'native scrolling must not select a reading');
        observations.push({ name, phase: 'native-vertical-pan', before, after, maximum: scrollMetrics.maximum, pointerCancels: Number(await busy.getAttribute('data-cancel-count')), touchAction });
        await session.detach();
      }

      const usage = pool.getByRole('img', { name: 'Capacity history', exact: true });
      await usage.waitFor();
      await settled('Capacity history', '3 stored observation times');
      assert.ok((await inspect(usage, 0)).includes('Used 10.0%'));
      await record('pool-first', usage, await description(usage));
      const poolRange = pool.getByRole('combobox', { name: 'History range' });
      assert.equal(await poolRange.inputValue(), '24h');
      samples = [point(0)];
      await poolRange.selectOption('7d');
      await settled('Capacity history', '1 stored observation time');
      await panel(usage).getByText('Single observation. No trend yet.').waitFor();
      // A measured zero is a stored reading.
      assert.ok((await inspect(usage, 0.5)).includes('Used 0.0%'));
      await record('pool-zero-singleton', usage, await description(usage));
      samples = [];
      await poolRange.selectOption('12h');
      await panel(usage).getByText('No stored history in this range').waitFor();
      assert.ok((await inspect(usage, 0.5)).startsWith('No stored history observations.'));
      assert.equal(await panel(usage).locator('[data-history-series], [data-history-observation]').count(), 0);
      await record('pool-empty', usage, await description(usage));
      assert.ok(requests.some(request => request.resourceType === 'disk' && request.resourceId === 'fixture-disk'));
      assert.ok(requests.some(request => request.resourceType === 'storage' && request.resourceId === 'fixture-pool'));
      assert.deepEqual(errors, []);
      await browser.close(); browser = null;
    }
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify({ playwright, expectedPlaywright, observations }, null, 2));
    console.log(JSON.stringify({ result: 'passed', observations: observations.length }));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
