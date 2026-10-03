const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const parent = false;
(async () => {
  const root = parent
    ? '/workspace/tmp/history-org-parent/frontend-modern'
    : '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = `/workspace/tmp/history-org-${parent ? 'parent' : 'final'}-proof`;
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: fs.mkdtempSync(path.join(artifacts, 'vite-')),
    server: { host: '127.0.0.1', port: 5264, strictPort: true },
  });
  const playwright = require('playwright/package.json').version;
  assert.equal(
    playwright,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json', 'utf8')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const results = [];
  let browser;
  try {
    await server.listen();
    for (const [name, engine, phone] of [
      ['chromium-desktop', chromium, false],
      ['webkit-phone', webkit, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: phone ? { width: 390, height: 844 } : { width: 1365, height: 900 },
        hasTouch: phone,
        isMobile: phone,
      });
      page.setDefaultTimeout(30_000);
      await page.clock.install({ time: new Date('2026-10-03T12:00:00Z') });
      const modes = {
        'org-a': { mode: 'success', value: 42, source: 'memory' },
        'org-b': { mode: 'hold', value: 80, source: 'store' },
        'org-c': { mode: '503' },
        'org-d': { mode: 'success', value: 61, source: 'store' },
      };
      const requests = [],
        held = [],
        checks = [],
        states = [],
        errors = [],
        offOrigin = [];
      page.on('pageerror', (error) => errors.push(error.message));
      await page.route('**/*', async (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5264') {
          offOrigin.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === '/api/metrics-store/history') {
          const query = Object.fromEntries(url.searchParams);
          const org = (await route.request().allHeaders())['x-pulse-org-id'];
          let response = { ...modes[org] };
          const record = { ...query, org, responseMode: response.mode };
          requests.push(record);
          if (response.mode === 'hold') {
            response = await new Promise((resolve) => held.push({ org, resolve }));
          }
          const points = [0, 1].map((index) => ({
            timestamp: 1791028740000 + index * 60_000,
            value: response.value - 1 + index,
            min: response.value - 1 + index,
            max: response.value - 1 + index,
          }));
          try {
            await route.fulfill({
              status: response.mode === '503' ? 503 : 200,
              json:
                response.mode === '503'
                  ? { error: 'synthetic outage' }
                  : {
                      resourceType: query.resourceType,
                      resourceId: query.resourceId,
                      range: query.range,
                      metric: query.metric,
                      source: response.source,
                      start: 1791028740000,
                      end: 1791028800000,
                      ...(query.metric ? { points } : { metrics: { cpu: points, memory: points } }),
                    },
            });
            record.fulfilled = true;
          } catch {
            // Explicitly aborted/superseded browser requests can no longer be fulfilled.
            record.fulfilled = false;
          }
          return;
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
              : { enabled: false, data: [] },
        });
      });
      const check = async (label, fn) => {
        try {
          await fn();
          checks.push({ label, passed: true });
        } catch (error) {
          checks.push({ label, passed: false, error: error.message });
        }
      };
      const screenshot = async (state) => {
        const file = `${name}-${state}.png`;
        await page.screenshot({
          path: path.join(artifacts, file),
          fullPage: true,
          animations: 'disabled',
        });
        states.push({
          state,
          screenshot: file,
          body: await page.locator('main').innerText(),
          overflow: await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
        });
      };
      const release = (org, response) => {
        const matches = held.filter((entry) => entry.org === org);
        for (const entry of matches) {
          held.splice(held.indexOf(entry), 1);
          entry.resolve(response);
        }
      };
      const bootRequests = [],
        failedRequests = [];
      page.on('request', (r) => {
        if (bootRequests.length < 300) bootRequests.push(r.url());
      });
      page.on('requestfailed', (r) => failedRequests.push({ url: r.url(), failure: r.failure() }));
      try {
        await page.goto('http://127.0.0.1:5264/browser-tests/history-org.html', {
          waitUntil: 'domcontentloaded',
          timeout: 60_000,
        });
      } catch (error) {
        fs.writeFileSync(
          path.join(artifacts, 'boot-error.json'),
          JSON.stringify(
            {
              parent,
              playwright,
              name,
              error: error.message,
              bootRequests,
              failedRequests,
              errors,
              requests,
              body: await page
                .locator('body')
                .innerText()
                .catch(() => ''),
            },
            null,
            2,
          ),
        );
        throw error;
      }
      if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const disk = page.getByTestId('disk'),
        pool = page.getByTestId('pool'),
        service = page.getByTestId('service');
      const thermal = disk.getByRole('img', { name: 'Temperature chart' });
      const description = () =>
        thermal.evaluate(
          (el) => document.getElementById(el.getAttribute('aria-describedby')).textContent,
        );
      const waitThermal = async (value) => {
        await page.waitForFunction((reading) => {
          const canvas = document.querySelector('[data-testid="disk"] canvas');
          return document
            .getElementById(canvas.getAttribute('aria-describedby'))
            .textContent.includes(reading);
        }, `${value}°C`);
      };
      const inspect = async () => {
        if (phone) await thermal.tap({ position: { x: 100, y: 80 } });
        else {
          await thermal.focus();
          await thermal.press('End');
        }
      };
      await waitThermal(42);
      await service.getByRole('slider', { name: 'Inspect Utilization history' }).waitFor();
      await inspect();
      await check(
        'Initial org-a observations and provenance are real rendered reader results',
        async () => {
          assert.match(await description(), /42°C/);
          assert.equal(await disk.locator('[data-history-chart-tooltip]').count(), 1);
          assert.match(await disk.innerText(), /Buffer/i);
          assert(requests.some((r) => r.org === 'org-a' && r.resourceType === 'disk'));
        },
      );
      await screenshot('org-a-loaded');
      modes['org-a'] = { mode: 'hold', value: 99, source: 'memory' };
      await page.clock.fastForward(30_000);
      await page.waitForTimeout(100);
      await check(
        'Ordinary same-org pending polling retains inspection and stored readings',
        async () => {
          assert(held.some((entry) => entry.org === 'org-a'));
          assert.match(await description(), /42°C/);
          assert.equal(await disk.locator('[data-history-chart-tooltip]').count(), 1);
        },
      );
      const beforeSwitch = requests.length;
      await page.getByRole('button', { name: 'Switch to org-b' }).click();
      await page.waitForTimeout(150);
      await check('All exact targets are immediately reread with the new org header', async () => {
        const next = requests.slice(beforeSwitch);
        for (const id of ['disk:nas:sda', 'storage:nas:pool', 'pbs-service']) {
          assert(next.some((r) => r.org === 'org-b' && r.resourceId === id));
        }
        assert(next.every((r) => r.org === 'org-b'));
      });
      await check(
        'Pending org-b removes org-a values, min/max, provenance and grouped inspection',
        async () => {
          assert.doesNotMatch(await description(), /42°C/);
          assert.doesNotMatch(await disk.innerText(), /Buffer|42°C/i);
          assert.equal(await disk.locator('[data-history-chart-tooltip]').count(), 0);
          assert.equal(await pool.locator('[data-history-chart-tooltip]').count(), 0);
          assert.equal(await service.getByRole('slider').count(), 0);
          assert.equal((await disk.locator('[aria-live="polite"]').innerText()).trim(), '');
        },
      );
      await screenshot('org-b-pending');
      release('org-a', { mode: 'success', value: 99, source: 'memory' });
      await page.waitForTimeout(150);
      await check(
        'Late pre-switch completion cannot resurrect readings or settle the new org',
        async () => {
          assert.doesNotMatch(await description(), /42°C|99°C/);
          assert.doesNotMatch(await disk.innerText(), /Buffer|99°C/i);
          assert.equal(await service.getByRole('slider').count(), 0);
        },
      );
      await screenshot('late-org-a');
      modes['org-b'] = { mode: 'success', value: 80, source: 'store' };
      release('org-b', modes['org-b']);
      await page.waitForTimeout(150);
      await check(
        'Only successful org-b observations restore History without old inspection',
        async () => {
          assert.match(await description(), /80°C/);
          assert.doesNotMatch(await description(), /99°C|42°C/);
          assert.equal(await disk.locator('[data-history-chart-tooltip]').count(), 0);
          assert.equal(await pool.locator('[data-history-chart-tooltip]').count(), 0);
          assert.equal(await service.getByRole('slider').count(), 1);
        },
      );
      // Allow the unchanged parent's next poll to catch up before testing the
      // independent transient-outage control, instead of relying on its stale state.
      await page.clock.fastForward(30_000);
      await waitThermal(80);
      await inspect();
      await screenshot('org-b-loaded');
      modes['org-b'] = { mode: '503' };
      await page.clock.fastForward(30_000);
      await page.waitForTimeout(300);
      await check(
        'A same-org 503 still retains its own successful readings with a warning',
        async () => {
          assert.match(await description(), /80°C/);
          assert.match(
            await disk.getByRole('status', { name: 'History refresh status' }).innerText(),
            /last successful result/,
          );
          assert.equal(await disk.locator('[data-history-chart-tooltip]').count(), 1);
        },
      );
      await screenshot('org-b-outage');
      await page.getByRole('button', { name: 'Switch to org-c' }).click();
      await page.clock.fastForward(30_000);
      await page.waitForTimeout(300);
      await check(
        'Failure in org-c cannot fall back to org-b values or its refresh warning',
        async () => {
          assert.doesNotMatch(await description(), /80°C|99°C|42°C/);
          assert.match(await disk.getByRole('alert').innerText(), /Failed to load history data/);
          assert.equal(
            await disk.getByRole('status', { name: 'History refresh status' }).innerText(),
            '',
          );
          assert.equal(await disk.locator('[data-history-chart-tooltip]').count(), 0);
          assert.equal(await service.getByRole('slider').count(), 0);
        },
      );
      await screenshot('org-c-failed');
      await page.getByRole('button', { name: 'Switch to org-d' }).click();
      await page.clock.fastForward(30_000);
      await waitThermal(61);
      await check(
        'New successful org-d data recovers; all requests keep their target/range/metric scope',
        async () => {
          assert.equal(await disk.getByRole('alert').count(), 0);
          assert.equal(
            await disk.getByRole('status', { name: 'History refresh status' }).innerText(),
            '',
          );
          assert(requests.every((r) => r.range === '1h'));
          assert(
            requests
              .filter((r) => r.resourceId === 'disk:nas:sda')
              .every((r) => r.metric === 'smart_temp' && r.resourceType === 'disk'),
          );
          assert(
            requests
              .filter((r) => r.resourceId === 'storage:nas:pool')
              .every((r) => r.metric === 'disk' && r.resourceType === 'storage'),
          );
          assert(
            requests
              .filter((r) => r.resourceId === 'pbs-service')
              .every((r) => !r.metric && r.maxPoints === '240'),
          );
        },
      );
      await screenshot('org-d-recovered');
      await check(
        'Light desktop/dark touch phone states fit, with no page errors or off-origin data',
        async () => {
          assert(states.every((state) => !state.overflow));
          assert.deepEqual(errors, []);
          assert.deepEqual(offOrigin, []);
        },
      );
      results.push({
        name,
        engineVersion: browser.version(),
        checks,
        states,
        requests,
        errors,
        offOrigin,
      });
      for (const entry of held) entry.resolve({ mode: '503' });
      await browser.close();
      browser = undefined;
    }
    fs.writeFileSync(
      path.join(artifacts, 'result.json'),
      JSON.stringify({ parent, playwright, results }, null, 2),
    );
    const failed = results.flatMap((result) => result.checks.filter((check) => !check.passed));
    console.log(
      JSON.stringify({
        parent,
        checks: results.flatMap((r) => r.checks).length,
        failed: failed.length,
        artifacts,
      }),
    );
    if (failed.length) process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
