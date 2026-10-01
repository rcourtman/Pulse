// Production History/query/CSS with synthetic API data, not installed PBS acceptance.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium, firefox, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const engine =
    process.argv.find((arg) => arg.startsWith('--engine='))?.split('=')[1] || 'chromium';
  assert.ok(['chromium', 'firefox', 'webkit'].includes(engine));
  const width = process.argv.includes('--phone') ? 390 : 1365;
  const artifacts = path.join(root, 'node_modules', `history-pointer-${engine}-${width}`);
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: fs.mkdtempSync(path.join(artifacts, 'vite-cache-')),
    server: { host: '127.0.0.1', port: 5223, strictPort: true },
  });
  let browser;
  const observations = [];
  const sourceFile = path.join(root, 'src/components/Workloads/GuestDrawerHistory.tsx');
  const sourceHash = createHash('sha256').update(fs.readFileSync(sourceFile)).digest('hex');
  const playwrightVersion = require('playwright/package.json').version;
  const integrationVersion = JSON.parse(
    fs.readFileSync('/workspace/tests/integration/package-lock.json'),
  ).packages['node_modules/@playwright/test'].version;
  assert.equal(playwrightVersion, integrationVersion);
  try {
    await server.listen();
    browser = await { chromium, firefox, webkit }[engine].launch(
      engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    for (const theme of ['light', 'dark']) {
      const page = await browser.newPage({
        viewport: { width, height: width === 390 ? 844 : 900 },
        locale: 'en-GB',
        timezoneId: 'UTC',
        ...(width === 390 ? { isMobile: true, hasTouch: true } : {}),
      });
      page.setDefaultTimeout(20_000);
      const requests = [],
        errors = [];
      let kind = 'original',
        held;
      page.on('pageerror', (error) => errors.push(error.message));
      const time = 1_700_000_000_000;
      const point = (offset, value) => ({
        timestamp: time + offset,
        value,
        min: value,
        max: value,
      });
      const metrics = (state) => {
        if (state === 'empty') return {};
        if (state === 'service') return { cpu: [point(0, 77), point(120_000, 88)] };
        return {
          cpu: [point(120_000, 43), point(0, 11), point(60_000, 22)],
          memory: [point(30_000, state === 'updated' ? 31 : 30), point(120_000, 55)],
          disk: [point(45_000, 0)],
          netin: [point(0, 0), point(120_000, 2048)],
          netout: [point(30_000, 4096)],
          diskread: [point(0, 2048), point(120_000, 8192)],
          diskwrite: [point(0, 0), point(120_000, 0)],
          temperature: [point(0, 42), point(120_000, 57)],
        };
      };
      const response = (id, range, state) => ({
        resourceType: 'agent',
        resourceId: id,
        range,
        start: time,
        end: time + 120_000,
        source: 'store',
        metrics: metrics(state),
      });
      await page.route('**/*', async (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5223') return route.abort();
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === '/api/metrics-store/history') {
          const id = url.searchParams.get('resourceId'),
            range = url.searchParams.get('range');
          assert.ok(['agent-three', 'pbs-three'].includes(id));
          const state = kind;
          requests.push({ id, range, state, method: route.request().method() });
          if (state === 'hold') {
            held = route;
            return;
          }
          if (state === 'failure')
            return route.fulfill({ status: 503, json: { error: 'Private fixture detail' } });
          return route.fulfill({ json: response(id, range, state) });
        }
        if (url.pathname === '/api/license/runtime-capabilities')
          return route.fulfill({
            json: {
              capabilities: [],
              limits: [],
              max_history_days: 7,
              hosted_mode: false,
              runtime: { build: 'community', label: 'Pulse Community runtime' },
              blocked_capabilities: [],
            },
          });
        return route.fulfill({ json: { data: [], enabled: false } });
      });
      await page.goto('http://127.0.0.1:5223/browser-tests/pbs-history-refresh.html', {
        waitUntil: 'domcontentloaded',
      });
      await page.evaluate(
        (dark) => document.documentElement.classList.toggle('dark', dark),
        theme === 'dark',
      );
      const fixture = page.getByTestId('history-refresh-fixture');
      const utilization = fixture.locator('[data-history-group="utilization"]');
      const slider = utilization.getByRole('slider', { name: 'Inspect Utilization history' });
      await slider.waitFor();
      const readsBefore = requests.length;
      const plot = utilization.getByTestId('guest-history-plot');
      const hover = async (chart, offset) => {
        await chart.scrollIntoViewIfNeeded();
        const box = await chart.getByTestId('guest-history-plot').boundingBox();
        assert.ok(box);
        await page.mouse.move(
          box.x + ((34 + (offset / 120_000) * 318) / 360) * box.width,
          box.y + box.height / 2,
        );
      };
      const text = async () => (await utilization.innerText()).replace(/\s+/g, ' ');
      const sparseTime = async (offset, expected, absent) => {
        await hover(utilization, offset);
        assert.match(await text(), expected);
        assert.match(await text(), absent);
        const markers = await plot
          .locator('circle[r="3"]')
          .evaluateAll((nodes) =>
            nodes.map((node) => ({ x: node.getAttribute('cx'), y: node.getAttribute('cy') })),
          );
        assert.equal(new Set(markers.map((marker) => marker.x)).size, 1);
        return markers;
      };
      // Capture the exact base misattribution too, before assertions fail.
      await hover(utilization, 30_000);
      const observed = { theme, sourceHash, text: await text(), browser: browser.version() };
      console.log(JSON.stringify({ observedSparsePointer: observed }));
      // Element screenshots scroll a tall phone fixture and end its hover.
      // Capture the current viewport without disturbing the active pointer.
      await page.screenshot({ path: path.join(artifacts, `sparse-pointer-${theme}.png`) });
      assert.match(await text(), /CPU\s*- Memory\s*30\.0% Disk\s*-/);
      assert.equal(await plot.locator('circle[r="3"]').count(), 1);
      assert.equal(await utilization.getByTestId('guest-history-hover-time').innerText(), '22:13');
      const descriptionId = await plot.getAttribute('aria-describedby');
      assert.match(
        await page.locator(`[id="${descriptionId}"]`).innerText(),
        /14\/11\/2023, 22:13:50\. CPU no observation\. Memory 30\.0%\. Disk no observation\./,
      );
      await sparseTime(0, /CPU\s*11\.0%/, /Memory\s*- Disk\s*-/);
      await sparseTime(45_000, /Disk\s*0\.0%/, /CPU\s*- Memory\s*-/);
      assert.equal(
        await plot.locator('path').count(),
        2,
        'lone disk sample must not become a path',
      );
      await hover(utilization, 120_000);
      assert.match(await text(), /CPU\s*43\.0% Memory\s*55\.0% Disk\s*-/);
      assert.equal(await plot.locator('circle[r="3"]').count(), 2);
      assert.equal(await utilization.getByTestId('guest-history-hover-time').innerText(), '22:15');
      const network = fixture.locator('[data-history-group="network"]');
      await hover(network, 0);
      assert.match((await network.innerText()).replace(/\s+/g, ' '), /In\s*0 B\/s Out\s*-/);
      await hover(network, 30_000);
      assert.match((await network.innerText()).replace(/\s+/g, ' '), /In\s*- Out\s*4\.00 KB\/s/);
      await page.mouse.move(1, 1);
      assert.match(await text(), /CPU\s*43\.0% Memory\s*55\.0% Disk\s*0\.0%/);
      assert.equal(await fixture.getByTestId('guest-history-hover-time').count(), 0);
      assert.equal(requests.length, readsBefore, 'pointer inspection must not request data');
      // Native keyboard selection remains authoritative even with the pointer on another time.
      await slider.focus();
      await slider.press('Home');
      await hover(utilization, 30_000);
      assert.match(await text(), /CPU\s*11\.0% Memory\s*- Disk\s*-/);
      await slider.press('ArrowRight');
      assert.match(
        await slider.getAttribute('aria-valuetext'),
        /CPU no observation\. Memory 30\.0%\. Disk no observation/,
      );
      await slider.press('Tab');
      await hover(utilization, 45_000);
      assert.match(await text(), /CPU\s*- Memory\s*- Disk\s*0\.0%/);
      if (width === 390) {
        const box = await slider.boundingBox();
        assert.ok(box.height >= 44);
        await page.touchscreen.tap(box.x + 3, box.y + box.height / 2);
        assert.match(
          await slider.getAttribute('aria-valuetext'),
          /CPU 11\.0%\. Memory no observation/,
        );
        await page.touchscreen.tap(box.x + box.width - 3, box.y + box.height / 2);
        assert.match(await slider.getAttribute('aria-valuetext'), /CPU 43\.0%\. Memory 55\.0%/);
        await slider.press('Tab');
      }
      await page.mouse.move(1, 1);
      kind = 'failure';
      await fixture.getByRole('button', { name: 'Refresh history' }).evaluate((el) => el.click());
      await fixture
        .getByText('History refresh failed. Showing previously loaded history.')
        .waitFor();
      await sparseTime(30_000, /Memory\s*30\.0%/, /CPU\s*-/);
      await page.screenshot({
        path: path.join(artifacts, `retained-sparse-pointer-${theme}.png`),
      });
      assert.equal((await fixture.innerText()).includes('Private fixture detail'), false);
      kind = 'updated';
      await fixture.getByRole('button', { name: 'Retry history' }).evaluate((el) => el.click());
      await page.waitForFunction(() =>
        [...document.querySelectorAll('[data-history-group="utilization"] span')].some(
          (el) => el.textContent === '31.0%',
        ),
      );
      assert.match(await text(), /CPU\s*- Memory\s*31\.0% Disk\s*-/);
      kind = 'hold';
      await page
        .getByRole('button', { name: 'Switch to service target' })
        .evaluate((el) => el.click());
      await page.waitForFunction(
        () =>
          [...document.querySelectorAll('[data-history-group="utilization"] path')].length === 0,
      );
      assert.equal(await fixture.getByTestId('guest-history-hover-time').count(), 0);
      assert.equal(await plot.locator('circle[r="3"]').count(), 0);
      assert.match(await text(), /CPU\s*42\.0% Memory\s*53\.0% Disk\s*-/);
      assert.ok(held);
      kind = 'service';
      await held.fulfill({ json: response('pbs-three', '24h', 'service') });
      await slider.waitFor();
      await hover(utilization, 0);
      assert.match(await text(), /CPU\s*77\.0% Memory\s*- Disk\s*-/);
      const dimensions = await page.evaluate(() => ({
        scroll: document.documentElement.scrollWidth,
        inner: innerWidth,
      }));
      assert.ok(dimensions.scroll <= dimensions.inner + 1, JSON.stringify(dimensions));
      assert.deepEqual(errors, []);
      observations.push({ theme, requests, dimensions, sourceHash });
      await page.close();
    }
    const result = {
      engine,
      width,
      browserVersion: browser.version(),
      playwrightVersion,
      sourceHash,
      observations,
      result: 'passed',
    };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2));
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
