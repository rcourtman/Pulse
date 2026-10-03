const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const parent = false;
(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules/pool-ownership-proof');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: fs.mkdtempSync(path.join(artifacts, 'vite-')),
    server: { host: '127.0.0.1', port: 5234, strictPort: true },
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
        errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      await page.route('**/*', (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5234') return route.abort();
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === '/api/metrics-store/history') {
          requests.push(Object.fromEntries(url.searchParams));
          return route.fulfill({
            json: {
              points: [{ timestamp: 1790935200000, value: 25, min: 25, max: 25 }],
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
              : { data: [], enabled: false },
        });
      });
      await page.goto('http://127.0.0.1:5234/browser-tests/pool-ownership.html');
      if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const detail = page.getByRole('region', { name: 'Pool detail' });
      const phases = parent
        ? [
            ['ZFS host A', 4, null],
            ['UnRAID host A', 3, null],
          ]
        : [
            ['ZFS host A', 1, 'Host A disk'],
            ['ZFS host B', 1, 'Host B disk'],
            ['Missing ownership', 0, null],
            ['Direct child', 1, 'Host A disk'],
            ['UnRAID host A', 1, 'Host A disk'],
            ['UnRAID host B', 1, 'Host B disk'],
          ];
      for (const [phase, count, model] of phases) {
        await page.getByRole('button', { name: phase, exact: true }).click();
        const text = await detail.textContent();
        if (count) {
          await detail.getByText(`Physical Disks (${count})`, { exact: true }).waitFor();
          if (model) {
            assert.ok(text.includes(model));
            assert.ok(!text.includes(model === 'Host A disk' ? 'Host B disk' : 'Host A disk'));
            assert.ok(!text.includes('Unowned disk') && !text.includes('Suffix collision disk'));
            assert.equal(text.includes('99 errors'), model === 'Host B disk');
            assert.ok(text.includes(model === 'Host B disk' ? '82°C' : '34°C'));
          } else
            assert.ok(
              text.includes('Host B disk') && text.includes('99 errors'),
              'parent must expose foreign health',
            );
        } else assert.equal(await detail.getByText(/Physical Disks/).count(), 0);
        const dimensions = await page.evaluate(() => ({
          scroll: document.documentElement.scrollWidth,
          width: innerWidth,
        }));
        assert.ok(dimensions.scroll <= dimensions.width + 1, JSON.stringify(dimensions));
        const screenshot = `${parent ? 'parent-' : ''}${name}-${phase.toLowerCase().replaceAll(' ', '-')}.png`;
        await page.screenshot({ path: path.join(artifacts, screenshot), fullPage: true });
        observations.push({
          name,
          phase,
          browser: browser.version(),
          text,
          dimensions,
          screenshot,
        });
      }
      if (!parent) {
        await page.getByRole('button', { name: 'ZFS host A', exact: true }).click();
        await detail.getByRole('tab', { name: 'History', exact: true }).click();
        await detail.getByRole('img', { name: 'Usage chart', exact: true }).waitFor();
        await page.waitForFunction(() =>
          [...document.querySelectorAll('canvas')].some((el) =>
            document
              .getElementById(el.getAttribute('aria-describedby'))
              ?.textContent.includes('1 data point'),
          ),
        );
        assert.ok(
          requests.some(
            (request) =>
              request.resourceId === 'history-a' &&
              request.metric === 'usage' &&
              request.resourceType === 'storage',
          ),
        );
        await page.getByRole('button', { name: 'ZFS host B', exact: true }).click();
        await page.waitForFunction(() =>
          [...document.querySelectorAll('canvas')].some((el) =>
            document
              .getElementById(el.getAttribute('aria-describedby'))
              ?.textContent.includes('1 data point'),
          ),
        );
        assert.ok(
          requests.some(
            (request) =>
              request.resourceId === 'history-b' &&
              request.metric === 'usage' &&
              request.resourceType === 'storage',
          ),
        );
        await detail.getByRole('tab', { name: 'Overview', exact: true }).click();
        await detail.getByText('Physical Disks (1)', { exact: true }).waitFor();
        assert.equal(await detail.getByText('Host A disk', { exact: true }).count(), 0);
        assert.equal(await detail.getByText('Host B disk', { exact: true }).count(), 1);
      }
      assert.deepEqual(errors, []);
      observations.push({ name, phase: 'history-requests', requests, errors });
      await browser.close();
      browser = null;
    }
    fs.writeFileSync(
      path.join(artifacts, parent ? 'parent.json' : 'result.json'),
      JSON.stringify({ playwright, expected, parent, observations }, null, 2),
    );
    console.log(JSON.stringify({ result: 'passed', parent, observations: observations.length }));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
