const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const parent = false;
(async () => {
  const root = parent
    ? '/workspace/tmp/history-access-parent/frontend-modern'
    : '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = `/workspace/tmp/history-access-${parent ? 'parent' : 'final'}-proof`;
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: fs.mkdtempSync(path.join(artifacts, 'vite-')),
    server: { host: '127.0.0.1', port: 5261, strictPort: true },
  });
  const playwright = require('playwright/package.json').version;
  const expected = JSON.parse(
    fs.readFileSync('/workspace/tests/integration/package-lock.json', 'utf8'),
  ).packages['node_modules/@playwright/test'].version;
  assert.equal(playwright, expected);
  const observations = [];
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
      page.setDefaultTimeout(15000);
      await page.clock.install({ time: new Date('2026-10-03T12:00:00Z') });
      let mode = 'success',
        value = 42;
      const requests = [],
        errors = [],
        offOrigin = [],
        held = [],
        checks = [];
      page.on('pageerror', (error) => errors.push(error.message));
      await page.route('**/*', async (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5261') {
          offOrigin.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === '/api/metrics-store/history') {
          const query = Object.fromEntries(url.searchParams);
          requests.push({ ...query, responseMode: mode });
          if (mode === 'hold') await new Promise((resolve) => held.push(resolve));
          try {
            if (mode === '503' || mode === '403')
              return await route.fulfill({
                status: Number(mode),
                json: { error: 'fixture-only diagnostic body' },
              });
            const points = [0, 1].map((index) => ({
              timestamp: 1791028740000 + index * 60000,
              value: value - 1 + index,
              min: value - 1 + index,
              max: value - 1 + index,
            }));
            return await route.fulfill({
              json: {
                resourceType: query.resourceType,
                resourceId: query.resourceId,
                range: query.range,
                metric: query.metric,
                start: 1791028740000,
                end: 1791028800000,
                source: 'store',
                ...(query.metric ? { points } : { metrics: { cpu: points, temperature: points } }),
              },
            });
          } catch {
            /* An explicitly superseded browser read may already be aborted. */
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
      const check = async (label, predicate) => {
        try {
          await predicate();
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
        observations.push({
          browser: name,
          state,
          screenshot: file,
          body: await page.locator('main').innerText(),
          overflow: await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
        });
      };
      const disk = page.getByTestId('disk'),
        service = page.getByTestId('service');
      const thermal = disk.getByRole('img', { name: 'Temperature chart' });
      const description = () =>
        thermal.evaluate(
          (el) => document.getElementById(el.getAttribute('aria-describedby')).textContent,
        );
      const storedPaths = () => service.locator('[data-testid="guest-history-plot"] path').count();
      await page.goto('http://127.0.0.1:5261/browser-tests/history-access.html');
      if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
      await service.getByRole('slider', { name: 'Inspect Utilization history' }).waitFor();
      await page.waitForFunction(
        () =>
          document.querySelector('[data-testid="disk"] canvas')?.getAttribute('aria-describedby') &&
          document
            .getElementById(
              document
                .querySelector('[data-testid="disk"] canvas')
                .getAttribute('aria-describedby'),
            )
            .textContent.includes('42°C'),
      );
      await service.getByRole('combobox', { name: 'History range' }).selectOption('24h');
      await service.getByRole('slider', { name: 'Inspect Utilization history' }).waitFor();
      if (phone) await thermal.tap({ position: { x: 100, y: 80 } });
      else {
        await thermal.focus();
        await thermal.press('End');
      }
      await check('Both readers show only their successful stored samples', async () => {
        assert.match(await description(), /42°C/);
        assert.equal(await storedPaths(), 2);
        assert.equal(await disk.locator('[data-history-chart-tooltip]').count(), 1);
      });
      await screenshot('loaded');
      mode = '503';
      await page.clock.fastForward(30_000);
      await service
        .getByRole('status', { name: 'History refresh status' })
        .getByText(/previously loaded/)
        .waitFor();
      await check('503 retains observations with a warning, not a collection claim', async () => {
        assert.match(await description(), /42°C/);
        assert.equal(await storedPaths(), 2);
        assert.match(
          await disk.getByRole('status', { name: 'History refresh status' }).innerText(),
          /last successful result/,
        );
      });
      await screenshot('transient');
      // Re-establish active inspection immediately before access changes.
      if (phone) await thermal.tap({ position: { x: 100, y: 80 } });
      else {
        await thermal.focus();
        await thermal.press('End');
      }
      mode = '403';
      const beforeDenied = requests.length;
      await page.clock.fastForward(30_000);
      await page.waitForTimeout(250);
      await check('The real API client receives a final 403 for both exact targets', async () => {
        const denied = requests.slice(beforeDenied).filter((r) => r.responseMode === '403');
        assert(denied.some((r) => r.resourceId === 'disk:nas-a:sda'));
        assert(denied.some((r) => r.resourceId === 'pbs-service-a' && !r.metric));
      });
      await check('403 withdraws canvas values, tooltip and announced inspection', async () => {
        assert.doesNotMatch(await description(), /42°C/);
        assert.equal(await disk.locator('[data-history-chart-tooltip]').count(), 0);
        assert.match(await disk.getByRole('alert').innerText(), /Access denied/);
        assert.equal(
          await disk.getByRole('status', { name: 'History refresh status' }).innerText(),
          '',
        );
      });
      await check('403 withdraws batch charts, sliders and current-only legends', async () => {
        assert.equal(await storedPaths(), 0);
        assert.equal(await service.getByRole('slider').count(), 0);
        assert.equal(await service.locator('[data-history-current]').count(), 0);
        assert.match(
          await service.getByRole('status', { name: 'History refresh status' }).innerText(),
          /Access denied/,
        );
      });
      await screenshot('denied');
      mode = 'hold';
      const retry = service.getByRole('button', { name: 'Retry history' });
      const beforeRetry = requests.length;
      await retry.focus();
      await retry.click();
      await retry.press('Enter');
      await page.waitForTimeout(150);
      await check(
        'One busy keyboard-safe retry remains scoped and shows no denied samples',
        async () => {
          assert.equal(
            requests.slice(beforeRetry).filter((r) => r.resourceId === 'pbs-service-a').length,
            1,
          );
          assert.equal(await retry.getAttribute('aria-disabled'), 'true');
          assert.equal(await retry.evaluate((el) => el === document.activeElement), true);
          assert.equal(await storedPaths(), 0);
        },
      );
      await screenshot('retry-pending');
      await service.getByRole('combobox', { name: 'History range' }).selectOption('1h');
      await page.waitForTimeout(150);
      await check('An earlier cached range cannot resurrect denied observations', async () =>
        assert.equal(await storedPaths(), 0),
      );
      await service.getByRole('button', { name: 'Hide service History' }).click();
      await service.getByRole('button', { name: 'Show service History' }).click();
      await page.waitForTimeout(150);
      await check('A remount cannot resurrect the earlier cached range', async () =>
        assert.equal(await storedPaths(), 0),
      );
      await screenshot('remount-pending');
      mode = 'success';
      value = 12;
      held.splice(0).forEach((resolve) => resolve());
      await page.clock.fastForward(30_000);
      await page.waitForFunction(() =>
        document
          .getElementById(
            document.querySelector('[data-testid="disk"] canvas').getAttribute('aria-describedby'),
          )
          .textContent.includes('12°C'),
      );
      await service.getByRole('slider', { name: 'Inspect Utilization history' }).waitFor();
      await check(
        'Only a new successful response restores readings and clears access errors',
        async () => {
          assert.equal(await storedPaths(), 2);
          assert.equal(await disk.getByRole('alert').count(), 0);
          assert.equal(
            await service.getByRole('status', { name: 'History refresh status' }).innerText(),
            '',
          );
          assert.doesNotMatch(await description(), /42°C/);
          assert(
            requests.some(
              (r) =>
                r.resourceId === 'pbs-service-a' &&
                r.range === '1h' &&
                r.responseMode === 'success',
            ),
          );
        },
      );
      await screenshot('recovered');
      await check(
        'No diagnostic body, page error, off-origin request or horizontal overflow',
        async () => {
          assert.doesNotMatch(
            await page.locator('main').innerText(),
            /fixture-only diagnostic body/,
          );
          assert.deepEqual(errors, []);
          assert.deepEqual(offOrigin, []);
          assert.equal(
            await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
            false,
          );
        },
      );
      observations.push({
        browser: name,
        version: browser.version(),
        checks,
        requests,
        errors,
        offOrigin,
      });
      await browser.close();
      browser = undefined;
    }
  } finally {
    if (browser) await browser.close();
    await server.close();
    fs.writeFileSync(
      path.join(artifacts, 'result.json'),
      JSON.stringify({ parent, playwright, expected, observations }, null, 2),
    );
  }
  const failed = observations.flatMap((o) => o.checks ?? []).filter((c) => !c.passed);
  console.log(
    JSON.stringify({
      parent,
      checks: observations.flatMap((o) => o.checks ?? []).length,
      failed: failed.map((c) => c.label),
    }),
  );
  if (!parent) assert.equal(failed.length, 0);
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
