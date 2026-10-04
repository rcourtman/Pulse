// Actual History consumers, API client and CSS; synthetic same-origin bodies.
// This verifies response admission, not a native appliance or shipped repair.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  const artifacts = path.join(root, 'node_modules/history-response-proof');
  fs.mkdirSync(artifacts, { recursive: true });
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: fs.mkdtempSync(path.join(artifacts, 'vite-')),
    server: { host: '127.0.0.1', port: 5269, strictPort: true },
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
      await page.clock.install({ time: new Date('2026-10-04T07:30:00Z') });
      let mode = 'wrong-target';
      let value = 42;
      const errors = [],
        requests = [],
        checks = [],
        screenshots = [],
        offOrigin = [];
      page.on('pageerror', (error) => errors.push(error.message));
      await page.route('**/*', async (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5269') {
          offOrigin.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === '/api/metrics-store/history') {
          const query = Object.fromEntries(url.searchParams);
          requests.push({ ...query, mode });
          if (mode === 'denied')
            return route.fulfill({ status: 403, json: { error: 'fixture-private-body' } });
          const points = [0, 1].map((i) => ({
            timestamp: 1791098940000 + i * 60_000,
            value: value - 1 + i,
            min: value - 1 + i,
            max: value - 1 + i,
          }));
          const response = {
            resourceType: mode === 'wrong-type' ? 'node' : query.resourceType,
            resourceId: mode === 'wrong-target' ? 'fixture-private-predecessor' : query.resourceId,
            range: mode === 'wrong-range' ? '7d' : query.range,
            start: 1791098940000,
            end: 1791099000000,
            source: 'store',
          };
          return route.fulfill({
            json: {
              ...response,
              ...(query.metric
                ? {
                    metric: mode === 'wrong-metric' ? 'cpu' : query.metric,
                    points: mode === 'malformed' ? null : mode === 'empty' ? [] : points,
                  }
                : {
                    metrics:
                      mode === 'malformed'
                        ? []
                        : mode === 'empty'
                          ? {}
                          : { cpu: points, temperature: points },
                  }),
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
      await page.goto('http://127.0.0.1:5269/browser-tests/history-access.html');
      if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const service = page.getByTestId('service');
      const disk = page.getByTestId('disk');
      const plots = () => service.locator('[data-testid="guest-history-plot"] path');
      const chartDescription = () =>
        disk
          .locator('canvas')
          .getAttribute('aria-describedby')
          .then((id) => disk.locator(`#${id}`).innerText());
      const check = async (label, action) => {
        await action();
        checks.push(label);
      };
      const capture = async (state) => {
        const dimensions = await page.evaluate(() => ({
          width: innerWidth,
          scroll: document.documentElement.scrollWidth,
        }));
        assert.ok(dimensions.scroll <= dimensions.width + 1, JSON.stringify(dimensions));
        const file = `${name}-${state}.png`;
        await page.screenshot({
          path: path.join(artifacts, file),
          fullPage: true,
          animations: 'disabled',
        });
        screenshots.push({
          state,
          file,
          dimensions,
          serviceText: await service.innerText(),
          diskDescription: await chartDescription(),
        });
      };
      await service.getByText('Failed to load history data', { exact: true }).waitFor();
      await disk.getByText('Failed to load history data', { exact: true }).waitFor();
      await check(
        'wrong-target HTTP200 is unavailable, not empty or previous-host samples',
        async () => {
          assert.equal(await plots().count(), 0);
          assert.ok(!(await service.innerText()).includes('No stored history'));
          assert.ok(!(await page.locator('body').innerText()).includes('fixture-private'));
          assert.ok(!(await chartDescription()).includes('42'));
        },
      );
      await capture('wrong-target');
      mode = 'correct';
      await service.getByRole('button', { name: 'Retry history' }).focus();
      await page.keyboard.press('Enter');
      await service.getByText('42.0%', { exact: true }).waitFor();
      await page.clock.runFor(30_000);
      await check(
        'keyboard retry and valid single-metric polling restore bound evidence',
        async () => {
          assert.ok((await chartDescription()).includes('42°C'));
          assert.equal(await plots().count(), 2);
        },
      );
      mode = 'wrong-range';
      value = 97;
      await service.getByRole('button', { name: 'Refresh history' }).click();
      await service
        .getByText('History refresh failed. Showing previously loaded history.', { exact: true })
        .waitFor();
      await page.clock.runFor(30_000);
      await check('wrong-range refresh keeps only labelled validated evidence', async () => {
        assert.ok((await service.innerText()).includes('42.0%'));
        assert.ok(!(await service.innerText()).includes('97.0%'));
        assert.ok((await disk.innerText()).includes('last successful result'));
        assert.ok(!(await chartDescription()).includes('97°C'));
      });
      await capture('retained-valid');
      await service.getByRole('button', { name: 'Hide service History' }).click();
      await service.getByRole('button', { name: 'Show service History' }).click();
      await service
        .getByText('History refresh failed. Showing previously loaded history.', { exact: true })
        .waitFor();
      await check('remount cache has no rejected range response', async () => {
        assert.ok((await service.innerText()).includes('42.0%'));
        assert.ok(!(await service.innerText()).includes('97.0%'));
      });
      for (const bad of ['wrong-type', 'malformed', 'wrong-metric']) {
        mode = bad;
        await service.getByRole('button', { name: 'Retry history' }).click();
        // wrong-metric affects the single-metric reader only.
        if (bad === 'wrong-metric') await service.getByText('97.0%', { exact: true }).waitFor();
        else
          await service
            .getByText('History refresh failed. Showing previously loaded history.', {
              exact: true,
            })
            .waitFor();
        await page.clock.runFor(30_000);
        await check(
          `${bad} cannot replace the single-metric chart's validated samples`,
          async () => {
            assert.ok(!(await chartDescription()).includes('97°C'));
            if (bad !== 'wrong-metric') assert.ok(!(await service.innerText()).includes('97.0%'));
          },
        );
      }
      mode = 'denied';
      await service.getByRole('button', { name: 'Refresh history' }).click();
      await service
        .getByText('Access denied. Check your permissions and license plan.', { exact: true })
        .waitFor();
      await page.clock.runFor(30_000);
      await check('HTTP403 still withdraws validated history and inspection', async () => {
        assert.equal(await plots().count(), 0);
        assert.equal(await service.getByRole('slider').count(), 0);
        assert.ok(!(await service.innerText()).includes('97.0%'));
        assert.ok(!(await chartDescription()).includes('42°C'));
      });
      await capture('denied');
      mode = 'correct';
      value = 0;
      const retry = service.getByRole('button', { name: 'Retry history' });
      await retry.focus();
      await page.keyboard.press('Enter');
      await service.getByText('0.0%', { exact: true }).waitFor();
      await page.clock.runFor(30_000);
      const canvas = disk.getByRole('img', { name: 'Temperature chart' });
      await canvas.focus();
      await page.keyboard.press('End');
      await check('measured zero is accepted, including keyboard canvas inspection', async () => {
        assert.ok((await chartDescription()).includes('0°C'));
        assert.equal(await plots().count(), 2);
      });
      await capture('zero-recovered');
      mode = 'empty';
      await service.getByRole('button', { name: 'Refresh history' }).click();
      await service.getByText('No stored history in this range', { exact: true }).first().waitFor();
      await page.clock.runFor(30_000);
      await check('valid empty success is distinct from the initial invalid body', async () => {
        assert.equal(await plots().count(), 0);
        assert.ok(!(await service.innerText()).includes('Failed to load'));
        assert.ok((await chartDescription()).includes('No 1-hour history data is available.'));
      });
      assert.deepEqual(errors, []);
      assert.deepEqual(offOrigin, []);
      results.push({
        browser: name,
        version: browser.version(),
        checks,
        requests,
        screenshots,
        errors,
        offOrigin,
      });
      await browser.close();
      browser = null;
    }
    fs.writeFileSync(
      path.join(artifacts, 'result.json'),
      JSON.stringify({ result: 'passed', playwright, results }, null, 2),
    );
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
