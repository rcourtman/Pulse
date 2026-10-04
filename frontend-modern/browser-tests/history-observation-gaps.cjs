const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/history-observation-gaps/browser';
const origin = 'http://127.0.0.1:5317';
const runtime = [
  'frontend-modern/src/components/Workloads/GuestDrawerHistory.tsx',
  'frontend-modern/src/components/Workloads/guestDrawerModel.ts',
];
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const settle = (page) =>
  page.evaluate(async () => {
    await document.fonts.ready;
    await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  });

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const result = {
    result: 'incomplete',
    playwright: require('playwright/package.json').version,
    runtime_hashes: Object.fromEntries(runtime.map((file) => [file, hash('/workspace/' + file)])),
    cases: [],
    screenshots: [],
    limits:
      'Production full GuestDrawer/History, client and CSS with synthetic local APIs. Not native guest/QGA/backup/thaw, installed recovery, physical-phone or release acceptance.',
  };
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: output + '/cache',
    server: { host: '127.0.0.1', port: 5317, strictPort: true },
  });
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['chromium-desktop-light', chromium, 1365, false],
      ['chromium-desktop-dark', chromium, 1365, true],
      ['webkit-phone-dark', webkit, 390, true],
      ['webkit-narrow-light', webkit, 320, false],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height: width < 768 ? 844 : 900 },
        isMobile: width < 768,
        hasTouch: width < 768,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      page.setDefaultTimeout(12000);
      page.setDefaultNavigationTimeout(60000);
      const record = {
        name,
        width,
        dark,
        browser: browser.version(),
        errors: [],
        requests: [],
        checks: [],
      };
      result.cases.push(record);
      let mode = 'gaps';
      const start = Date.UTC(2026, 9, 4, 12);
      const point = (minute, value) => ({
        timestamp: start + minute * 60_000,
        value,
        min: value,
        max: value,
      });
      const times = [0, 15, 30, 45, 60];
      const metrics = () => ({
        cpu: times.map((minute, index) => point(minute, 10 + index * 5)),
        memory: (mode === 'complete' ? times : [0, 15, 60]).map((minute) =>
          point(minute, 20 + minute / 6),
        ),
        disk: (mode === 'complete' ? times : [0, 60]).map((minute) => point(minute, 50)),
        netin: times.map((minute) => point(minute, 100)),
        netout: (mode === 'complete' ? times : [0, 60]).map((minute) => point(minute, 0)),
        diskread: (mode === 'complete' ? times : [0, 60]).map((minute) => point(minute, 0)),
        diskwrite: times.map((minute) => point(minute, 200)),
      });
      page.on('pageerror', (error) => record.errors.push(error.message));
      await page.route('**/*', async (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== origin) {
          record.requests.push({ origin: url.origin, blocked: true });
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        record.requests.push({ path: url.pathname, method: route.request().method(), mode });
        if (url.pathname === '/api/metrics-store/history') {
          assert.equal(route.request().method(), 'GET');
          assert.equal(url.searchParams.get('resourceId'), 'fixture:pve:101');
          assert.equal(url.searchParams.get('maxPoints'), '240');
          if (mode === '503' || mode === '403')
            return route.fulfill({
              status: Number(mode),
              json: { error: 'private-fixture-detail' },
            });
          return route.fulfill({
            json: {
              resourceType: url.searchParams.get('resourceType'),
              resourceId: url.searchParams.get('resourceId'),
              range: url.searchParams.get('range'),
              start,
              end: start + 60 * 60_000,
              metrics: metrics(),
              source: 'store',
            },
          });
        }
        return route.fulfill({
          json:
            url.pathname === '/api/license/runtime-capabilities'
              ? {
                  capabilities: [],
                  limits: [],
                  max_history_days: 90,
                  hosted_mode: false,
                  runtime: { build: 'community' },
                  blocked_capabilities: [],
                }
              : { success: true, data: [], alerts: [], anomalies: [], config: null },
        });
      });
      await page.addInitScript((isDark) => {
        document.addEventListener('DOMContentLoaded', () =>
          document.documentElement.classList.toggle('dark', isDark),
        );
        window.__trustedInputs = [];
        for (const type of ['click', 'input', 'keydown'])
          document.addEventListener(
            type,
            (event) => window.__trustedInputs.push({ type, trusted: event.isTrusted }),
            true,
          );
      }, dark);
      await page.goto(origin + '/browser-tests/history-observation-gaps.html');
      const activate = async (locator) => (width < 768 ? locator.tap() : locator.click());
      await activate(page.getByRole('tab', { name: 'History', exact: true }));
      const chart = page.locator('[data-history-group="utilization"]');
      const slider = chart.getByRole('slider', { name: 'Inspect Utilization history' });
      await slider.waitFor();
      await settle(page);
      await page.evaluate(() => {
        window.__originalHistoryChart = document.querySelector(
          '[data-history-group="utilization"]',
        );
      });
      const memoryPath = chart.locator('path[stroke="#f59e0b"]');
      assert.equal(await memoryPath.count(), 1);
      assert.equal((await memoryPath.getAttribute('d')).match(/L/g).length, 1);
      assert.equal(await chart.locator('[data-history-observation="memory"]').count(), 1);
      assert.equal(await chart.locator('path[stroke="#10b981"]').count(), 0);
      assert.equal(await chart.locator('[data-history-observation="disk"]').count(), 2);
      assert.equal(
        (await chart.locator('path[stroke="#8b5cf6"]').getAttribute('d')).match(/L/g).length,
        4,
      );
      assert.match(
        await chart.locator('[data-history-gaps]').innerText(),
        /Missing observations: Memory, Disk\./,
      );
      assert.equal(await page.locator('[data-history-gaps]').count(), 3);
      const description = await chart
        .getByRole('img', { name: 'Utilization history' })
        .evaluate((el) =>
          el
            .getAttribute('aria-describedby')
            .split(' ')
            .map((id) => document.getElementById(id).textContent)
            .join(' '),
        );
      assert.match(description, /Missing observations: Memory, Disk\./);
      assert.match(description, /does not identify the cause or duration/);
      const noteGeometry = await chart.locator('[data-history-gaps]').evaluate((el) => ({
        width: el.clientWidth,
        scrollWidth: el.scrollWidth,
        colour: getComputedStyle(el).color,
      }));
      assert.ok(noteGeometry.scrollWidth <= noteGeometry.width + 1);
      record.noteGeometry = noteGeometry;
      record.checks.push(
        'Actual stored gap splits only affected metrics; recovered singleton/zero samples stay visible; visible/accessibly linked explanation fits.',
      );
      const screenshot = async (suffix) => {
        await settle(page);
        const file = `${name}-${suffix}.png`;
        await page.screenshot({ path: path.join(output, file), fullPage: true });
        result.screenshots.push({
          file,
          sha256: hash(path.join(output, file)),
          name,
          state: suffix,
        });
      };
      await screenshot('gaps');

      await slider.focus();
      await page.keyboard.press('Home');
      await page.keyboard.press('ArrowRight');
      await page.keyboard.press('ArrowRight');
      assert.equal(await slider.inputValue(), '2');
      assert.match(
        await slider.getAttribute('aria-valuetext'),
        /Memory no observation\. Disk no observation\./,
      );
      assert.match(await chart.innerText(), /Memory\s*-/);
      assert.equal(await chart.locator('circle[r="3"]').count(), 1);
      record.checks.push(
        'Trusted Home/Arrow keys inspect the missing stored time without borrowing nearby/live memory or disk.',
      );
      if (width < 768) {
        await slider.scrollIntoViewIfNeeded();
        const box = await slider.boundingBox();
        await page.touchscreen.tap(box.x + box.width * 0.75, box.y + box.height / 2);
        assert.notEqual(await slider.inputValue(), '2');
        record.checks.push('Trusted touch operates the native inspector.');
      } else {
        await page.getByRole('heading', { name: 'Drawer History missing observations' }).click();
        const box = await chart.getByRole('img', { name: 'Utilization history' }).boundingBox();
        await page.mouse.move(box.x + box.width * 0.537, box.y + box.height / 2);
        assert.match(
          await chart.locator('[data-testid="guest-history-hover-time"]').innerText(),
          /12:30/,
        );
        assert.equal(await chart.locator('circle[r="3"]').count(), 1);
        record.checks.push(
          'Trusted pointer matches the exact stored time, not a synthetic line value.',
        );
      }
      const reads = () =>
        record.requests.filter((r) => r.path === '/api/metrics-store/history').length;
      const before = reads();
      await activate(page.getByRole('button', { name: 'Simulate live recovery' }));
      assert.equal(reads(), before);
      assert.equal(await chart.locator('[data-history-gaps]').count(), 1);
      assert.equal(await chart.locator('[data-history-observation="memory"]').count(), 1);
      assert.equal(
        await page.evaluate(
          () =>
            window.__originalHistoryChart ===
            document.querySelector('[data-history-group="utilization"]'),
        ),
        true,
      );
      record.checks.push(
        'Live status recovery cannot fill missing stored evidence, refetch or remount the chart.',
      );

      mode = '503';
      await activate(page.getByRole('button', { name: 'Refresh history' }));
      await page.getByRole('button', { name: 'Retry history' }).waitFor();
      assert.equal(await chart.locator('[data-history-gaps]').count(), 1);
      assert.equal(await chart.locator('path[stroke="#10b981"]').count(), 0);
      assert.doesNotMatch(await page.locator('body').innerText(), /private-fixture-detail/);
      record.checks.push('Transient read failure retains the gap and does not leak the raw error.');
      mode = 'complete';
      await activate(page.getByRole('button', { name: 'Retry history' }));
      await page.waitForFunction(
        () => document.querySelectorAll('[data-history-gaps]').length === 0,
      );
      assert.equal(await chart.locator('[data-history-observation]').count(), 0);
      assert.equal((await memoryPath.getAttribute('d')).match(/L/g).length, 4);
      assert.equal(
        await page.evaluate(
          () =>
            window.__originalHistoryChart ===
            document.querySelector('[data-history-group="utilization"]'),
        ),
        true,
      );
      record.checks.push(
        'Matching complete stored refresh joins actual consecutive observations; chart identity stays stable.',
      );
      await screenshot('complete');
      mode = '403';
      await activate(page.getByRole('button', { name: 'Refresh history' }));
      await page.waitForFunction(
        () => document.querySelectorAll('[data-testid="guest-history-group-chart"]').length === 0,
      );
      assert.equal(await page.locator('[data-history-gaps]').count(), 0);
      assert.doesNotMatch(await page.locator('body').innerText(), /private-fixture-detail/);
      record.checks.push('Access denial withdraws all stored chart/gap/inspection evidence.');
      assert.equal(reads(), before + 3);
      assert.equal(
        await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1),
        true,
      );
      assert.equal(
        record.requests.some((r) => r.method && r.method !== 'GET'),
        false,
      );
      assert.equal(
        record.requests.some((r) => r.blocked),
        false,
      );
      assert.deepEqual(record.errors, []);
      assert.equal(
        await page.evaluate(() => window.__trustedInputs.every((entry) => entry.trusted)),
        true,
      );
      record.checks.push(
        'No page error, horizontal overflow, external request or mutation; unchanged user-request read budget.',
      );
      await page.close();
      await browser.close();
      browser = null;
    }
    result.result = 'passed';
  } catch (error) {
    result.failure = error.stack || String(error);
    throw error;
  } finally {
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    if (browser) await browser.close();
    await server.close();
  }
  process.stdout.write(JSON.stringify(result) + '\n');
})().catch((error) => {
  process.stderr.write(error.stack + '\n');
  process.exitCode = 1;
});
