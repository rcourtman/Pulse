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
          // The disk drawer reads every series of its one target at once.
          const series = (value) => [
            { timestamp: observedAt - 60000, value, min: value, max: value },
            { timestamp: observedAt, value, min: value, max: value },
          ];
          const temperature = query.resourceId.includes('nas-b') ? 55 : 42;
          return route.fulfill({
            json: {
              resourceType: query.resourceType,
              resourceId: query.resourceId,
              range: query.range,
              start: Date.now() - 180000,
              end: Date.now(),
              source: 'store',
              metrics: {
                smart_temp: query.range === '1h' ? [] : series(temperature),
                smart_reallocated_sectors: series(0),
                smart_percentage_used: series(12),
                smart_available_spare: series(97),
              },
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
      const plots = page.locator('[data-testid="guest-history-plot"]');
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
          visibleCharts: await plots.evaluateAll((items) =>
            items.filter((el) => el.getBoundingClientRect().width > 0).map((el) => el.ariaLabel),
          ),
          overflow: await page.evaluate(
            () => document.documentElement.scrollWidth > window.innerWidth,
          ),
          body: await page.locator('main').innerText(),
        });
      };
      const chart = (label) => page.getByRole('img', { name: `${label} history`, exact: true });
      const thermal = () => chart('Temperature');
      const description = async (label = 'Temperature') =>
        chart(label).evaluate(
          (el) =>
            document.getElementById(el.getAttribute('aria-describedby').split(' ')[0]).textContent,
        );
      // Snapshots arrive from the store, not from a click that moves focus.
      const snapshot = (label) =>
        page.getByRole('button', { name: label }).evaluate((button) => button.click());
      const range = () => page.getByRole('combobox', { name: 'History range' });
      await page.goto('http://127.0.0.1:5251/browser-tests/disk-history-collection.html');
      if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
      // The disk Overview has no tabs: notices, then History, then the facts.
      const historySection = page.getByTestId('guest-history-region');
      await historySection.waitFor();
      await page.waitForFunction(() =>
        [...document.querySelectorAll('[data-testid="guest-history-plot"]')].some((el) =>
          document.getElementById(el.getAttribute('aria-describedby').split(' ')[0])?.textContent.includes('42°C'),
        ),
      );
      // A phone folds every chart after the first behind one button; open it,
      // as a reader inspecting the disk's history would.
      if (phone) {
        const fold = page.getByTestId('history-phone-toggle');
        if (await fold.isVisible()) await fold.click();
      }
      await page.waitForTimeout(300);
      await check('Stored ATA history is accessible before any current fields', async () => {
        assert.deepEqual(await plots.evaluateAll((items) => items.map((el) => el.ariaLabel)), [
          'Temperature history',
          'Reallocated sectors history',
        ]);
        assert.match(await description(), /Disk 42°C/);
        assert.match(await description('Reallocated sectors'), /Reallocated 0 sectors/);
      });
      await check('History shows without a tab to select', async () => {
        assert.equal(await page.getByRole('tab').count(), 0);
        assert.equal(await historySection.isVisible(), true);
        assert.equal(await chart('Reallocated sectors').isVisible(), true);
      });
      await screenshot('missing-current');
      await snapshot('Restore current fields');
      await thermal().waitFor();
      await range().selectOption('7d');
      await page.waitForFunction(() => {
        const el = document.querySelector('[aria-label="Temperature history"]');
        const id = el?.getAttribute('aria-describedby')?.split(' ')[0];
        return id && document.getElementById(id).textContent.includes('42°C');
      });
      const inspect = page.getByRole('slider', { name: 'Inspect Temperature history' });
      if (phone) {
        // A tap on the plot hands inspection to touch: the scrubber shows.
        await thermal().tap({ position: { x: 120, y: 30 } });
        await inspect.waitFor();
      }
      await inspect.focus();
      await inspect.press('Home');
      await page.waitForTimeout(100);
      const firstObservation = await page.evaluate(
        (time) => new Date(time).toLocaleString(),
        observedAt - 60000,
      );
      await check('Stored zero and exact temperature can be inspected', async () => {
        assert.equal(await inspect.isVisible(), true);
        assert.ok((await description()).startsWith(firstObservation), await description());
        assert.match(await description(), /Disk 42°C/);
        assert.match(await description('Reallocated sectors'), /Reallocated 0 sectors/);
      });
      const inspected = await description();
      await thermal().evaluate((el) => (el.dataset.proofOwner = 'original'));
      const reads = requests.length;
      await snapshot('Update current fields');
      await page.waitForTimeout(200);
      await check('Same-ID snapshot keeps mounted chart and makes no extra reads', async () => {
        assert.equal(await thermal().getAttribute('data-proof-owner'), 'original');
        assert.equal(requests.length, reads);
        assert.equal(await range().inputValue(), '7d');
        assert.equal(await inspect.evaluate((el) => document.activeElement === el), true);
        assert.equal(await description(), inspected);
      });
      await screenshot('updated-current');
      await snapshot('Lose current fields');
      await page.waitForTimeout(200);
      await check(
        'Missing current fields do not withdraw stored readings or invent live I/O',
        async () => {
          assert.equal(await thermal().count(), 1);
          assert.equal(await thermal().getAttribute('data-proof-owner'), 'original');
          assert.match(await description(), /42°C/);
          assert.equal(requests.length, reads);
          assert.equal(await chart('Disk I/O').count(), 0);
          assert.equal(await chart('Busy').count(), 0);
        },
      );
      await screenshot('missing-again');
      if (!parent) {
        await check(
          'Details report unavailable collection rather than the old temperature',
          async () => {
            assert.equal(
              await page
                .getByText('Temperature is temporarily unavailable: collection deadline exceeded')
                .isVisible(),
              true,
            );
            // Stored history may show the old 42°C; the current fields must not.
            assert.equal(
              await page.evaluate(
                () =>
                  [...document.querySelectorAll('main *')].filter(
                    (el) =>
                      el.children.length === 0 &&
                      el.textContent.trim().includes('42°C') &&
                      !el.closest('[data-testid="guest-history-region"]'),
                  ).length,
              ),
              0,
            );
          },
        );
        await range().selectOption('1h');
        const thermalPanel = thermal().locator(
          'xpath=ancestor::*[@data-testid="guest-history-group-chart"][1]',
        );
        await thermalPanel.getByText('No stored history in this range').waitFor();
        await check(
          'Empty temperature is not zero and reallocated zero remains stored',
          async () => {
            assert.doesNotMatch(await description(), /0°C/);
            assert.match(await description(), /No stored history observations\./);
            assert.equal(await thermalPanel.locator('[data-history-series]').count(), 0);
            assert.equal(await plots.count(), 2);
            assert.match(await description('Reallocated sectors'), /Reallocated 0 sectors/);
          },
        );
        await screenshot('empty-temperature');
        await range().selectOption('7d');
        await snapshot('Select NVMe B');
        await chart('Wear').waitFor();
        await page.waitForFunction(() => {
          const el = document.querySelector('[aria-label="Temperature history"]');
          const id = el?.getAttribute('aria-describedby')?.split(' ')[0];
          return id && document.getElementById(id).textContent.includes('55°C');
        });
        await check(
          'NVMe History uses explicit B target and its own metrics without live fields',
          async () => {
            assert.deepEqual(await plots.evaluateAll((items) => items.map((el) => el.ariaLabel)), [
              'Temperature history',
              'Wear history',
            ]);
            assert.equal(await chart('Reallocated sectors').count(), 0);
            assert.match(await description(), /55°C/);
            assert.match(await description('Wear'), /Spare 97\.0%/);
            const own = requests.filter((r) => r.resourceId === 'disk:nas-b:nvme0n1');
            assert.ok(own.length > 0);
            assert.equal(
              own.every((r) => r.resourceType === 'disk'),
              true,
            );
          },
        );
        await screenshot('nvme');
        const beforeClear = requests.length;
        await snapshot('Clear identity');
        await page.waitForTimeout(200);
        await check(
          'Unresolvable identity withdraws History and does not guess a target',
          async () => {
            assert.equal(await historySection.count(), 0);
            assert.equal(await plots.count(), 0);
            assert.equal(await page.getByTestId('disk-drawer-history-unavailable').isVisible(), true);
            assert.equal(requests.length, beforeClear);
          },
        );
        await screenshot('no-identity');
      }
      await check('No page errors, off-target reads or horizontal overflow', async () => {
        assert.deepEqual(errors, []);
        assert.equal(
          await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth),
          false,
        );
        assert.equal(
          requests.every(
            (r) =>
              r.resourceType === 'disk' &&
              ['disk:nas-a:sda', 'disk:nas-b:nvme0n1'].includes(r.resourceId),
          ),
          true,
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
      failed: failed.map((c) => `${c.label}: ${c.error}`),
    }),
  );
  if (!parent) assert.equal(failed.length, 0);
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
