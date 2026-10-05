const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/history-provenance-20261005-complete/browser';
const origin = 'http://127.0.0.1:5317';
const runtime = [
  'frontend-modern/src/utils/memoryObservation.ts',
  'frontend-modern/src/components/Workloads/guestDrawerModel.ts',
  'frontend-modern/src/components/Infrastructure/resourceDetailDrawerMetricsHistoryModel.ts',
  'frontend-modern/src/components/Infrastructure/useResourceDetailDrawerDerivedState.ts',
  'frontend-modern/src/components/Infrastructure/ResourceDetailDrawer.tsx',
];
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const result = {
    result: 'incomplete',
    playwright: require('playwright/package.json').version,
    runtime_hashes: Object.fromEntries(runtime.map((p) => [p, hash('/workspace/' + p)])),
    cases: [],
    screenshots: [],
    limits:
      'Production ResourceDetailDrawer and GuestDrawer, History, client and CSS; synthetic local observations/APIs. Not native guest safety, provider/appliance, installed, reporter or release acceptance.',
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
    optimizeDeps: { noDiscovery: true, include: [] },
    server: { host: '127.0.0.1', port: 5317, strictPort: true, watch: null },
  });
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, dark, guest] of [
      ['resource-desktop-light', chromium, 1365, false, false],
      ['resource-desktop-dark', chromium, 1365, true, false],
      ['resource-phone-dark', webkit, 390, true, false],
      ['resource-phone-light', webkit, 390, false, false],
      ['guest-desktop-light', chromium, 1365, false, true],
      ['guest-desktop-dark', chromium, 1365, true, true],
      ['guest-phone-dark', webkit, 390, true, true],
      ['guest-phone-light', webkit, 390, false, true],
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
        guest,
        browser: browser.version(),
        errors: [],
        consoleErrors: [],
        expectedHttpFailures: [],
        requests: [],
        checks: [],
      };
      result.cases.push(record);
      let mode = 'empty';
      const start = Date.parse('2026-10-04T12:00:00Z'),
        end = Date.parse('2026-10-05T00:00:00Z');
      const point = (timestamp, value) => ({ timestamp, value, min: value, max: value });
      page.on('pageerror', (error) => record.errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error')
          record.consoleErrors.push({ text: message.text(), url: message.location().url });
      });
      await page.routeWebSocket('**/*', (socket) => {
        if (socket.url().startsWith('ws://127.0.0.1:5317/')) socket.connectToServer();
        else {
          record.requests.push({ blocked: true, socket: socket.url() });
          socket.close();
        }
      });
      await page.route('**/*', (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== origin) {
          record.requests.push({ blocked: true, origin: url.origin });
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        assert.equal(route.request().method(), 'GET');
        record.requests.push({ path: url.pathname, method: route.request().method(), mode });
        if (url.pathname === '/api/metrics-store/history') {
          assert.equal(url.searchParams.get('resourceType'), 'vm');
          assert.equal(url.searchParams.get('resourceId'), 'fixture:pve:101');
          assert.equal(url.searchParams.get('maxPoints'), '240');
          if (mode === 'transient' || mode === 'denied') {
            record.expectedHttpFailures.push({
              url: url.href,
              status: mode === 'transient' ? 503 : 403,
            });
            return route.fulfill({
              status: mode === 'transient' ? 503 : 403,
              json: { error: 'private-error-sentinel' },
            });
          }
          return route.fulfill({
            json: {
              resourceType: 'vm',
              resourceId: 'fixture:pve:101',
              range: url.searchParams.get('range'),
              start,
              end,
              source: 'store',
              metrics:
                mode === 'stored'
                  ? {
                      memory: [point(start, 5), point(end, 0)],
                      disk: [point(start, 40), point(end, 45)],
                    }
                  : {},
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
              : {
                  success: true,
                  data: [],
                  alerts: [],
                  anomalies: [],
                  config: null,
                  capabilities: [],
                  relationships: [],
                  recentChanges: [],
                  available: false,
                  audits: [],
                  count: 0,
                },
        });
      });
      await page.addInitScript((isDark) => {
        document.addEventListener('DOMContentLoaded', () =>
          document.documentElement.classList.toggle('dark', isDark),
        );
        window.__inputs = [];
        for (const type of ['click', 'keydown', 'input'])
          document.addEventListener(
            type,
            (e) => window.__inputs.push({ type, trusted: e.isTrusted }),
            true,
          );
      }, dark);
      await page.goto(
        origin + '/browser-tests/resource-history-provenance.html' + (guest ? '?guest' : ''),
      );
      const activate = (locator) => (width < 768 ? locator.tap() : locator.click());
      await activate(page.getByRole('tab', { name: 'History', exact: true }));
      const chart = page.locator('[data-history-group="utilization"]');
      await chart.locator('[data-history-last-known="memory"]').waitFor();
      await page.evaluate(
        () => (window.__chart = document.querySelector('[data-history-group="utilization"]')),
      );
      const screenshot = async (state) => {
        await page.evaluate(async () => {
          await document.fonts.ready;
          await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
        });
        const file = `${name}-${state}.png`;
        await page.screenshot({ path: path.join(output, file), fullPage: true });
        result.screenshots.push({ file, sha256: hash(path.join(output, file)), state, name });
        const geometry = await page.evaluate(() => ({
          width: document.documentElement.clientWidth,
          scroll: document.documentElement.scrollWidth,
        }));
        assert.ok(geometry.scroll <= geometry.width + 1, JSON.stringify(geometry));
      };
      assert.match(
        await chart.locator('[data-history-last-known="memory"]').innerText(),
        /25.0%\s*last known/,
      );
      assert.match(
        await chart.locator('[data-history-last-known="disk"]').innerText(),
        /50.0%\s*last known/,
      );
      assert.equal(
        await chart
          .locator('[data-history-current="memory"], [data-history-current="disk"]')
          .count(),
        0,
      );
      assert.match(
        await chart.locator('[data-history-deferred="memory"]').innerText(),
        /2026-10-04 12:00:00 UTC/,
      );
      assert.match(
        await chart.locator('[data-history-deferred="disk"]').innerText(),
        /operation lock/,
      );
      assert.equal(await chart.locator('path, [data-history-observation]').count(), 0);
      const description = await chart
        .locator('[data-history-last-known="memory"]')
        .evaluate((el) => document.getElementById(el.getAttribute('aria-describedby')).textContent);
      assert.match(description, /Not a current measurement/);
      await screenshot('retained');
      record.checks.push(
        'Retained memory/disk qualified visibly and accessibly; no synthetic stored points.',
      );
      const reads = () =>
        record.requests.filter((r) => r.path === '/api/metrics-store/history').length;
      const firstReads = reads();
      await activate(page.getByRole('button', { name: 'Advance snapshot', exact: true }));
      assert.match(
        await chart.locator('[data-history-deferred="memory"]').innerText(),
        /2026-10-04 12:00:00 UTC/,
      );
      assert.equal(reads(), firstReads);
      record.checks.push(
        'Snapshot Last seen refresh cannot renew observation time or add a History read.',
      );
      await activate(page.getByRole('button', { name: 'unavailable', exact: true }));
      assert.equal(
        await chart
          .locator(
            '[data-history-last-known], [data-history-current="memory"], [data-history-current="disk"]',
          )
          .count(),
        0,
      );
      assert.match(await chart.innerText(), /Memory\s*-/);
      assert.match(await chart.innerText(), /Disk\s*-/);
      assert.match(
        await chart.locator('[data-history-deferred="memory"]').innerText(),
        /Unavailable/,
      );
      await screenshot('unavailable');
      record.checks.push(
        'Unavailable memory and non-retained filesystem carriers are missing, never zero/current.',
      );
      await activate(page.getByRole('button', { name: 'unknown', exact: true }));
      assert.match(
        await chart.locator('[data-history-unknown="memory"]').innerText(),
        /25.0%\s*freshness unknown/,
      );
      assert.match(
        await chart.locator('[data-history-deferred="memory"]').innerText(),
        /Unknown source.*Observation time unknown/,
      );
      assert.doesNotMatch(
        await page.locator('body').innerText(),
        /private-(state|source|error)-sentinel|2999/,
      );
      record.checks.push(
        'Unknown state/source/future timestamp fail closed without leaking raw text.',
      );
      await activate(page.getByRole('button', { name: 'current', exact: true }));
      assert.match(
        await chart.locator('[data-history-current="memory"]').innerText(),
        /0.0%\s*current/,
      );
      assert.match(
        await chart.locator('[data-history-current="disk"]').innerText(),
        /50.0%\s*current/,
      );
      assert.equal(await chart.locator('[data-history-deferred]').count(), 0);
      assert.equal(reads(), firstReads);
      assert.ok(
        await page.evaluate(
          () => window.__chart === document.querySelector('[data-history-group="utilization"]'),
        ),
      );
      await screenshot('current');
      record.checks.push(
        'Fresh measured zero and independent disk recovery render in place without extra reads or inferred thaw.',
      );
      mode = 'stored';
      await activate(page.getByRole('button', { name: 'Refresh history', exact: true }));
      const slider = chart.getByRole('slider', { name: 'Inspect Utilization history' });
      await slider.waitFor();
      assert.equal(
        await chart
          .locator('[data-history-current="memory"], [data-history-current="disk"]')
          .count(),
        0,
      );
      await slider.focus();
      await page.keyboard.press('Home');
      assert.match(await slider.getAttribute('aria-valuetext'), /Memory 5.0%.*Disk 40.0%/);
      await page.keyboard.press('End');
      assert.match(await slider.getAttribute('aria-valuetext'), /Memory 0.0%.*Disk 45.0%/);
      assert.equal(await chart.locator('path').count(), 2);
      await screenshot('stored');
      await slider.blur();
      await activate(page.getByRole('button', { name: 'retained', exact: true }));
      assert.equal(await chart.locator('path').count(), 2);
      assert.match(
        await chart.locator('[data-history-deferred="memory"]').innerText(),
        /Last known/,
      );
      record.checks.push(
        'Actual stored history and trusted Home/End inspection remain separate from live/retained provenance.',
      );
      mode = 'transient';
      await activate(page.getByRole('button', { name: 'Refresh history', exact: true }));
      await page.getByRole('button', { name: 'Retry history', exact: true }).waitFor();
      assert.equal(await chart.locator('path').count(), 2);
      assert.match(
        await page.getByRole('status', { name: 'History refresh status' }).innerText(),
        /previously loaded history/,
      );
      mode = 'denied';
      await activate(page.getByRole('button', { name: 'Retry history', exact: true }));
      await chart.waitFor({ state: 'detached' });
      assert.match(await page.locator('body').innerText(), /Access denied/);
      assert.equal(
        await page
          .locator('[data-history-deferred], [data-history-last-known], [data-history-current]')
          .count(),
        0,
      );
      assert.doesNotMatch(
        await page.locator('body').innerText(),
        /private-(state|source|error)-sentinel/,
      );
      record.checks.push(
        'Transient read retains only stored evidence; access denial withdraws all History carriers.',
      );
      assert.deepEqual(record.errors, []);
      // Chromium reports deliberately failed fetches to its console; WebKit
      // need not. Retain every message and permit only the two exact endpoint
      // and status pairs exercised above, never a general error filter.
      assert.equal(record.expectedHttpFailures.length, 2);
      for (const error of record.consoleErrors) {
        assert.ok(
          record.expectedHttpFailures.some(
            (failure) =>
              error.url === failure.url &&
              error.text ===
                `Failed to load resource: the server responded with a status of ${failure.status} (${failure.status === 503 ? 'Service Unavailable' : 'Forbidden'})`,
          ),
          JSON.stringify(error),
        );
      }
      assert.ok(!record.requests.some((r) => r.blocked));
      const inputs = await page.evaluate(() => window.__inputs);
      assert.ok(inputs.some((e) => e.type === 'keydown' && e.trusted));
      assert.ok(inputs.some((e) => e.type === 'click' && e.trusted));
      record.trustedInputs = inputs;
      await browser.close();
      browser = undefined;
    }
    result.result = 'passed';
    result.verified_at = new Date().toISOString();
  } catch (error) {
    result.error = String(error.stack || error);
    throw error;
  } finally {
    if (browser) await browser.close();
    await server.close();
    result.browser_closed = true;
    result.server_closed = true;
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
  }
  process.stdout.write(
    JSON.stringify({
      result: result.result,
      groups: result.cases.length,
      checks: result.cases.reduce((n, c) => n + c.checks.length, 0),
      screenshots: result.screenshots.length,
      resultPath: output + '/result.json',
    }) + '\n',
  );
})().catch((e) => {
  process.stderr.write(String(e.stack || e) + '\n');
  process.exitCode = 1;
});
