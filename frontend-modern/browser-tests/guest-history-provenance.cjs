const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/guest-history-provenance';
const origin = 'http://127.0.0.1:5316';
const runtime = [
  'frontend-modern/src/components/Workloads/GuestDrawer.tsx',
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
      'Production full GuestDrawer/History, API client and CSS with synthetic local responses. Not native QGA/thaw, provider collection, physical-phone, installed recovery or release acceptance.',
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
    server: { host: '127.0.0.1', port: 5316, strictPort: true },
  });
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['chromium-desktop', chromium, 1365, false],
      ['chromium-phone', chromium, 390, false],
      ['webkit-phone', webkit, 360, true],
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
      page.setDefaultTimeout(15000);
      page.setDefaultNavigationTimeout(60000);
      let mode = 'empty';
      const record = {
        name,
        width,
        dark,
        browser: browser.version(),
        errors: [],
        requests: [],
        states: [],
        checks: [],
      };
      result.cases.push(record);
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
          if (mode === '403' || mode === '503')
            return route.fulfill({
              status: Number(mode),
              json: { error: 'fixture-private-error-body' },
            });
          const start = Date.UTC(2026, 9, 4, 2),
            end = start + 60 * 60_000;
          const point = (timestamp, value) => ({ timestamp, value, min: value, max: value });
          const metrics =
            mode === 'stored'
              ? {
                  cpu: [point(start, 10), point(end, 15)],
                  disk: [point(start, 20), point(end, 30)],
                }
              : mode === 'cpu-only'
                ? { cpu: [point(start, 10), point(end, 15)] }
                : {};
          return route.fulfill({
            json: {
              resourceType: url.searchParams.get('resourceType'),
              resourceId: url.searchParams.get('resourceId'),
              range: url.searchParams.get('range'),
              start,
              end,
              metrics,
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
                  max_history_days: 7,
                  hosted_mode: false,
                  runtime: { build: 'community' },
                  blocked_capabilities: [],
                }
              : { success: true, data: [], alerts: [], anomalies: [], config: null },
        });
      });
      await page.addInitScript(
        (isDark) =>
          document.addEventListener('DOMContentLoaded', () => {
            if (isDark) document.documentElement.classList.add('dark');
          }),
        dark,
      );
      await page.goto(origin + '/browser-tests/guest-history-provenance.html');
      await page.waitForFunction(() => window.__guestHistoryProvenance, undefined, {
        timeout: 60000,
      });
      const activate = (locator) => (width < 768 ? locator.tap() : locator.click());
      await activate(page.getByRole('tab', { name: 'History', exact: true }));
      const chart = page.locator('[data-history-group="utilization"]');
      const notice = chart.locator('[data-history-deferred="disk"]');
      const waitRead = async () => {
        await page.waitForFunction(() => document.querySelector('button[aria-busy="false"]'));
        await settle(page);
      };
      const observe = async (next) => {
        await page.evaluate((value) => window.__guestHistoryProvenance.observe(value), next);
        await waitRead();
      };
      const screenshot = async (state) => {
        const file = `${name}-${state}.png`;
        await page.screenshot({
          path: output + '/' + file,
          fullPage: true,
          animations: 'disabled',
        });
        result.screenshots.push({ file, sha256: hash(output + '/' + file), state, name });
      };
      const checkGeometry = async () => {
        const geometry = await notice.evaluate((el) => {
          const rect = el.getBoundingClientRect(),
            group = el.closest('section').getBoundingClientRect();
          const range = document.createRange();
          range.selectNodeContents(el);
          return {
            left: rect.left,
            right: rect.right,
            bottom: rect.bottom,
            groupRight: group.right,
            textRight: Math.max(...Array.from(range.getClientRects(), (r) => r.right)),
            plotTop: el.nextElementSibling.getBoundingClientRect().top,
            overflow: document.documentElement.scrollWidth > innerWidth,
          };
        });
        assert.equal(geometry.overflow, false);
        assert.ok(
          geometry.left >= 0 &&
            geometry.right <= width &&
            geometry.textRight <= geometry.groupRight + 1,
        );
        assert.ok(geometry.bottom <= geometry.plotTop + 1);
        return geometry;
      };
      await waitRead();
      const reasons = await page.evaluate(() => window.__guestHistoryProvenance.deferrals);
      for (const [reason, message] of reasons) {
        for (const retained of [true, false]) {
          await observe({
            reason: (retained ? 'prev-' : '') + reason,
            usage: 50,
            lock: reason === 'vm-locked' ? 'backup' : '',
          });
          assert.equal(await chart.locator('[data-history-current="disk"]').count(), 0);
          assert.equal(
            await chart.locator('[data-history-last-known="disk"]').count(),
            retained ? 1 : 0,
          );
          assert.equal((await notice.innerText()).includes(message), true);
          if (retained) {
            const legend = chart.locator('[data-history-last-known="disk"]');
            assert.match(await legend.innerText(), /50\.0%[\s\S]*last known/);
            assert.equal(
              await legend.evaluate((el) =>
                document
                  .getElementById(el.getAttribute('aria-describedby'))
                  .textContent.includes('Using last known'),
              ),
              true,
            );
          }
          assert.equal(await chart.locator('path').count(), 0);
          assert.equal(await chart.locator('[data-history-observation]').count(), 0);
          assert.equal(await chart.locator('[data-history-current="cpu"]').count(), 1);
          record.states.push({ reason, retained, geometry: await checkGeometry() });
        }
      }
      await observe({ reason: 'prev-vm-locked', usage: 0 });
      assert.match(
        await chart.locator('[data-history-last-known="disk"]').innerText(),
        /0\.0%[\s\S]*last known/,
      );
      record.checks.push('Retained zero is not lost or plotted.');
      await observe({ reason: 'prev-vm-locked', usage: 50, lock: 'backup' });
      await screenshot('last-known');
      await observe({ reason: 'prev-agent-busy', lock: '' });
      assert.match(await notice.innerText(), /earlier guest request is still in progress/);
      assert.equal(await chart.locator('[data-history-current="disk"]').count(), 0);
      record.checks.push('Lock clearance alone does not restore current evidence.');
      await observe({ reason: 'vm-locked', usage: -1 });
      assert.equal(await chart.locator('[data-history-last-known]').count(), 0);
      await screenshot('unavailable');
      await observe({ reason: 'prev-agent-timeout', usage: 50 });
      mode = 'stored';
      await activate(page.getByRole('button', { name: 'Refresh history', exact: true }));
      await page.waitForFunction(
        () => document.querySelectorAll('[data-history-group="utilization"] path').length === 2,
      );
      await settle(page);
      const paths = await chart
        .locator('path')
        .evaluateAll((nodes) => nodes.map((el) => el.getAttribute('d')));
      assert.equal(await chart.locator('[data-history-last-known]').count(), 0);
      assert.match(
        await notice.innerText(),
        /Completion is uncertain\. Do not restart the guest agent during a backup/,
      );
      const slider = chart.getByRole('slider', { name: 'Inspect Utilization history' });
      if (width < 768) await slider.tap({ position: { x: 3, y: 20 } });
      else {
        await slider.focus();
        await slider.press('Home');
      }
      await settle(page);
      assert.match(await slider.getAttribute('aria-valuetext'), /Disk 20\.0%/);
      assert.equal(await chart.locator('[data-history-current]').count(), 0);
      assert.deepEqual(
        await chart.locator('path').evaluateAll((nodes) => nodes.map((el) => el.getAttribute('d'))),
        paths,
      );
      await screenshot('stored-inspection');
      await slider.blur();
      record.checks.push(
        'Keyboard/touch inspection uses actual stored 20% at its own date, not retained 50%; paths remain unchanged.',
      );
      mode = 'cpu-only';
      await activate(page.getByRole('button', { name: 'Refresh history', exact: true }));
      await page.waitForFunction(
        () => document.querySelectorAll('[data-history-group="utilization"] path').length === 1,
      );
      await settle(page);
      await slider.focus();
      await slider.press('Home');
      assert.match(await slider.getAttribute('aria-valuetext'), /Disk no observation/);
      assert.equal(await chart.locator('[data-history-last-known]').count(), 0);
      await slider.blur();
      assert.equal(await chart.locator('[data-history-last-known="disk"]').count(), 1);
      record.checks.push('A dated CPU observation cannot borrow the retained filesystem value.');
      mode = '503';
      await activate(page.getByRole('button', { name: 'Refresh history', exact: true }));
      await page.getByRole('button', { name: 'Retry history', exact: true }).waitFor();
      assert.equal(await chart.locator('path').count(), 1);
      assert.equal(await notice.count(), 1);
      assert.equal(
        (await page.locator('main').innerText()).includes('fixture-private-error-body'),
        false,
      );
      mode = '403';
      await activate(page.getByRole('button', { name: 'Retry history', exact: true }));
      await page.waitForFunction(() => !document.querySelector('[data-history-group]'));
      assert.equal(await notice.count(), 0);
      assert.equal(await page.locator('[data-history-last-known]').count(), 0);
      record.checks.push(
        'Transient failure keeps prior stored data with warning; final access denial withdraws charts, fallback and notices.',
      );
      mode = 'empty';
      await activate(page.getByRole('button', { name: 'Retry history', exact: true }));
      await notice.waitFor();
      await waitRead();
      await observe({ reason: '', usage: 75, lock: '' });
      assert.equal(await notice.count(), 0);
      assert.equal(await chart.locator('[data-history-last-known]').count(), 0);
      assert.match(
        await chart.locator('[data-history-current="disk"]').innerText(),
        /75\.0%[\s\S]*current/,
      );
      assert.equal(await chart.locator('path').count(), 0);
      await screenshot('fresh-resumption');
      record.checks.push(
        'Successful read and fresh same-VM data restore current 75%, clear provenance, and create no history points.',
      );
      await observe({ reason: 'prev-vm-locked', kind: 'lxc', usage: 50 });
      assert.equal(await notice.count(), 0);
      assert.equal(await chart.locator('[data-history-current="disk"]').count(), 1);
      assert.equal(
        record.requests.every((request) => request.method === 'GET' && !request.blocked),
        true,
      );
      assert.deepEqual(record.errors, []);
      await page.close();
      await browser.close();
      browser = null;
    }
    result.result = 'passed';
  } catch (error) {
    result.error = { name: error.name, message: error.message };
    throw error;
  } finally {
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  process.stderr.write(error.stack + '\n');
  process.exitCode = 1;
});
