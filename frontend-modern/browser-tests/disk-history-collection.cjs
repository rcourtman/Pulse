const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const parent = false;
(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(
    root,
    'node_modules',
    parent ? 'disk-history-parent' : 'disk-history-proof',
  );
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: fs.mkdtempSync(path.join(artifacts, 'vite-')),
    server: { host: '127.0.0.1', port: 5251, strictPort: true },
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
      const requests = [],
        errors = [],
        checks = [];
      const observedAt = Date.now() - 60000;
      page.on('pageerror', (error) => errors.push(error.message));
      await page.route('**/*', (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5251') return route.abort();
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === '/api/metrics-store/history') {
          const query = Object.fromEntries(url.searchParams);
          requests.push(query);
          const value =
            query.metric === 'smart_temp'
              ? query.resourceId.includes('nas-b')
                ? 55
                : 42
              : query.metric === 'smart_percentage_used'
                ? 12
                : query.metric === 'smart_available_spare'
                  ? 97
                  : 0;
          const empty = query.range === '1h' && query.metric === 'smart_temp';
          return route.fulfill({
            json: {
              resourceType: query.resourceType,
              resourceId: query.resourceId,
              metric: query.metric,
              range: query.range,
              start: Date.now() - 60000,
              end: Date.now(),
              source: 'store',
              points: empty ? [] : [{ timestamp: observedAt, value, min: value, max: value }],
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
          visibleCharts: await page.locator('canvas[role="img"]').allTextContents(),
          selectedTabs: await page
            .getByRole('tab')
            .evaluateAll((tabs) =>
              tabs.map((tab) => ({
                label: tab.textContent,
                selected: tab.getAttribute('aria-selected'),
                border: getComputedStyle(tab).borderBottomColor,
              })),
            ),
          overflow: await page.evaluate(
            () => document.documentElement.scrollWidth > window.innerWidth,
          ),
          body: await page.locator('main').innerText(),
        });
      };
      const thermal = () => page.getByRole('img', { name: 'Temperature chart' });
      const description = async () =>
        thermal().evaluate(
          (el) => document.getElementById(el.getAttribute('aria-describedby')).textContent,
        );
      await page.goto('http://127.0.0.1:5251/browser-tests/disk-history-collection.html');
      if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
      await page.getByRole('tab', { name: 'History' }).click();
      await page.waitForTimeout(300);
      await check('Stored ATA history is accessible before any current fields', async () => {
        assert.equal(await page.locator('canvas[role="img"]').count(), 2);
        assert.match(await description(), /42°C/);
      });
      await check('History selection matches its visible content', async () => {
        assert.equal(
          await page.getByRole('tab', { name: 'History' }).getAttribute('aria-selected'),
          'true',
        );
      });
      await screenshot('missing-current');
      await page.getByRole('button', { name: 'Restore current fields' }).click();
      await thermal().waitFor();
      await page.getByRole('combobox', { name: 'Disk history range' }).selectOption('7d');
      await page.waitForFunction(
        () =>
          document
            .querySelector('canvas[aria-label="Temperature chart"]')
            ?.getAttribute('aria-describedby') &&
          document
            .getElementById(
              document
                .querySelector('canvas[aria-label="Temperature chart"]')
                .getAttribute('aria-describedby'),
            )
            .textContent.includes('42°C'),
      );
      if (phone) await thermal().tap({ position: { x: 80, y: 60 } });
      else {
        await thermal().focus();
        await thermal().press('End');
      }
      await page.waitForTimeout(100);
      await check('Stored zero and exact temperature can be inspected', async () => {
        assert.equal(await page.locator('[data-history-chart-tooltip]').count(), 2);
        assert.match(
          await page
            .locator('[data-history-chart-tooltip]')
            .allTextContents()
            .then((parts) => parts.join(' ')),
          /0 sectors/,
        );
      });
      await thermal().evaluate((el) => (el.dataset.proofOwner = 'original'));
      const reads = requests.length;
      await page.getByRole('button', { name: 'Update current fields' }).click();
      await page.waitForTimeout(200);
      await check('Same-ID snapshot keeps mounted chart and makes no extra reads', async () => {
        assert.equal(await thermal().getAttribute('data-proof-owner'), 'original');
        assert.equal(requests.length, reads);
        assert.equal(await page.getByRole('combobox').inputValue(), '7d');
      });
      await screenshot('updated-current');
      await page.getByRole('button', { name: 'Lose current fields' }).click();
      await page.waitForTimeout(200);
      await check(
        'Missing current fields do not withdraw stored readings or invent live I/O',
        async () => {
          assert.equal(await thermal().count(), 1);
          assert.equal(await thermal().getAttribute('data-proof-owner'), 'original');
          assert.match(await description(), /42°C/);
          assert.equal(requests.length, reads);
          assert.equal(await page.getByText('Live I/O (30m)').count(), 0);
        },
      );
      await screenshot('missing-again');
      if (!parent) {
        await page.getByRole('tab', { name: 'Overview' }).click();
        await check(
          'Overview reports unavailable collection rather than the old temperature',
          async () => {
            assert.equal(
              await page
                .getByText('Temperature is temporarily unavailable: collection deadline exceeded')
                .isVisible(),
              true,
            );
            assert.equal(await page.getByText('42°C', { exact: true }).count(), 0);
          },
        );
        await page.getByRole('tab', { name: 'History' }).click();
        await page.getByRole('combobox').selectOption('1h');
        await page.getByText('No history samples in this time range.').waitFor();
        await check(
          'Empty temperature is not zero and reallocated zero remains stored',
          async () => {
            assert.doesNotMatch(await description(), /0°C/);
            assert.equal(await page.locator('canvas[role="img"]').count(), 2);
          },
        );
        await screenshot('empty-temperature');
        await page.getByRole('combobox').selectOption('7d');
        await page.getByRole('button', { name: 'Select NVMe B' }).click();
        await page.getByRole('img', { name: 'Available Spare chart' }).waitFor();
        await page.waitForTimeout(200);
        await check(
          'NVMe History uses explicit B target and its own metrics without live fields',
          async () => {
            assert.equal(await page.locator('canvas[role="img"]').count(), 3);
            assert.equal(
              await page.getByRole('img', { name: 'Reallocated Sectors chart' }).count(),
              0,
            );
            assert.match(await description(), /55°C/);
            const own = requests.filter((r) => r.resourceId === 'disk:nas-b:nvme0n1');
            assert.equal(
              own.some((r) => r.metric === 'smart_available_spare'),
              true,
            );
            assert.equal(
              own.every((r) => r.resourceType === 'disk'),
              true,
            );
          },
        );
        await screenshot('nvme');
        const beforeClear = requests.length;
        await page.getByRole('button', { name: 'Clear identity' }).click();
        await page.waitForTimeout(200);
        await check(
          'Unresolvable identity withdraws History and does not guess a target',
          async () => {
            assert.equal(await page.getByRole('tab', { name: 'History' }).count(), 0);
            assert.equal(await page.locator('canvas[role="img"]').count(), 0);
            assert.equal(requests.length, beforeClear);
          },
        );
        await screenshot('no-identity');
      }
      await check('No page errors, off-origin data reads or horizontal overflow', async () => {
        assert.deepEqual(errors, []);
        assert.equal(
          await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth),
          false,
        );
        assert.equal(
          requests.some((r) => ['diskread', 'diskwrite', 'disk'].includes(r.metric)),
          false,
        );
      });
      observations.push({ browser: name, version: browser.version(), checks, requests, errors });
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
