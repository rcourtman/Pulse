const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/filesystem-usage-evidence';
const runtime = [
  'frontend-modern/src/components/Workloads/diskListModel.ts',
  'frontend-modern/src/components/Workloads/DiskList.tsx',
  'frontend-modern/src/components/Workloads/GuestDrawerOverview.tsx',
];
const unknown = [
  'missing-used',
  'negative-used',
  'NaN-used',
  'infinite-used',
  'missing-total',
  'zero-total',
  'negative-total',
  'NaN-total',
  'infinite-total',
  'unknown-sentinel',
  'NaN-usage',
  'infinite-usage',
  'overflow',
  'retained-missing',
];
const digest = (p) => crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex');

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const playwrightVersion = require('playwright/package.json').version;
  assert.equal(
    playwrightVersion,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const hashes = Object.fromEntries(runtime.map((p) => [p, digest(path.join('/workspace', p))]));
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(output, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5302, strictPort: true },
  });
  const results = [];
  const screenshots = [];
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, height, dark] of [
      ['chromium-desktop', chromium, 1365, 900, false],
      ['chromium-phone', chromium, 390, 844, false],
      ['webkit-phone', webkit, 360, 844, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const browserVersion = browser.version();
      const page = await browser.newPage({
        viewport: { width, height },
        isMobile: width <= 768,
        hasTouch: width <= 768,
        locale: 'en-GB',
      });
      page.setDefaultTimeout(60000);
      const pageErrors = [];
      const apiRequests = [];
      const offOrigin = [];
      page.on('pageerror', (error) => pageErrors.push(error.message));
      await page.route('**/*', (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5302') {
          offOrigin.push(url.origin);
          return route.abort();
        }
        if (url.pathname.startsWith('/api/') || url.pathname === '/ws') {
          apiRequests.push(url.pathname);
          return route.abort();
        }
        return route.continue();
      });
      await page.goto('http://127.0.0.1:5302/browser-tests/filesystem-usage-evidence.html', {
        waitUntil: 'domcontentloaded',
      });
      const select = page.getByLabel('Synthetic observation');
      await select.waitFor({ state: 'visible' });
      if (dark) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const list = page.getByRole('region', { name: 'Filesystem list' });
      const overview = page.getByRole('region', { name: 'Guest Overview' });
      // Overview owns a full-width guest drawer. A narrow fixture cap at an
      // xl viewport would activate four columns inside an artificial 672px
      // host and split the existing backup copy into single characters.
      const backupBounds = await overview.getByText('No completed backup found').boundingBox();
      assert(backupBounds && backupBounds.width >= 100 && backupBounds.height <= 40);
      await page.evaluate(() => {
        window.__listIdentity = document.querySelector('[aria-label="Filesystem list"]');
        window.__overviewIdentity = document.querySelector(
          '[data-testid="guest-technical-details"]',
        );
      });
      const checks = [];
      const selectState = async (state) => {
        await select.selectOption(state);
        assert.equal(
          await page.evaluate(
            () =>
              window.__listIdentity === document.querySelector('[aria-label="Filesystem list"]') &&
              window.__overviewIdentity ===
                document.querySelector('[data-testid="guest-technical-details"]'),
          ),
          true,
        );
      };
      const capture = async (state) => {
        const p = path.join(output, `${name}-${state}.png`);
        await page.screenshot({ path: p, fullPage: true });
        screenshots.push({ path: path.relative('/workspace', p), sha256: digest(p) });
      };
      for (const state of unknown) {
        await selectState(state);
        await overview.locator('[data-testid="guest-technical-details"]').waitFor();
        for (const panel of [list, overview]) {
          const text = await panel.innerText();
          assert(text.includes('—'), `${state}: missing unavailable marker`);
          assert(!/0%|NaN|Infinity/.test(text), `${state}: fabricated/invalid value: ${text}`);
          assert(text.includes('/data') && text.includes('EXT4'));
          assert.equal(await panel.getByRole('progressbar').count(), 0);
        }
        const listFill = list.locator('[style*="width:"]');
        assert.equal(await listFill.evaluate((element) => element.style.width), '0%');
        assert(!(await listFill.getAttribute('class')).includes('metric-normal'));
        assert.equal(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
          true,
        );
        checks.push({ state, unknown: true, numericProgress: false, noOverflow: true });
        if (['missing-used', 'infinite-total', 'retained-missing'].includes(state)) {
          if (state === 'retained-missing') {
            assert((await list.innerText()).includes('earlier guest request is still in progress'));
          }
          await capture(state);
        }
      }
      for (const [state, percentage] of [
        ['zero', 0],
        ['live', 50],
        ['usage-omitted', 50],
        ['over-capacity', 125],
        ['retained-live', 50],
      ]) {
        await selectState(state);
        assert((await list.innerText()).includes(`${percentage}%`));
        assert.equal(
          await overview.getByRole('progressbar').getAttribute('aria-valuenow'),
          String(Math.min(percentage, 100)),
        );
        checks.push({ state, percentage, measured: true });
        if (state === 'zero') await capture(state);
      }
      // Real keyboard input on the fixture replaces the same observation;
      // there is no new product control or collector action.
      await selectState('missing-used');
      await select.focus();
      await page.keyboard.press('Home');
      await page.keyboard.press('Enter');
      assert.equal(await select.inputValue(), 'live');
      assert.equal(await overview.getByRole('progressbar').getAttribute('aria-valuenow'), '50');
      assert.equal(pageErrors.length, 0);
      assert.equal(apiRequests.length, 0);
      assert.equal(offOrigin.length, 0);
      results.push({
        name,
        browserVersion,
        viewport: { width, height },
        dark,
        checks,
        pageErrors,
        apiRequests,
        offOrigin,
      });
      await browser.close();
      browser = undefined;
    }
    fs.writeFileSync(
      path.join(output, 'result.json'),
      JSON.stringify(
        {
          result: 'passed',
          verified_at: new Date().toISOString(),
          playwrightVersion,
          hashes,
          results,
          screenshots,
          scope:
            'Production DiskList and GuestDrawerOverview under synthetic same-filesystem observations. No native collection, installed/release acceptance or full source qualification.',
        },
        null,
        2,
      ),
    );
  } catch (error) {
    fs.writeFileSync(
      path.join(output, 'failure.json'),
      JSON.stringify({ error: error.message, results }, null, 2),
    );
    throw error;
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  process.stderr.write(`${error.stack}\n`);
  process.exitCode = 1;
});
