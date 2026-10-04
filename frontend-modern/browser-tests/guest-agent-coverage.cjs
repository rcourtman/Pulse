const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/guest-agent-coverage-final-proof';
const runtime = [
  'frontend-modern/src/components/Workloads/GuestDrawer.tsx',
  'frontend-modern/src/components/Workloads/GuestDrawerOverview.tsx',
  'frontend-modern/src/components/Workloads/guestDrawerModel.ts',
  'frontend-modern/src/components/Workloads/workloadAgentReadiness.ts',
];
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const initial = {
  state: 'deferred',
  reason: 'prev-vm-locked',
  lock: 'backup',
  backup: true,
  version: '6.4.5',
  assignment: false,
  diskUsed: 5 * 1024 ** 3,
};
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const playwrightVersion = require('playwright/package.json').version;
  assert.equal(
    playwrightVersion,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const hashes = Object.fromEntries(runtime.map((p) => [p, hash(path.join('/workspace', p))]));
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(output, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5296, strictPort: true, watch: null },
  });
  const origin = 'http://127.0.0.1:5296';
  const results = [],
    screenshots = [];
  let browser,
    phase = 'server';
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['chromium-desktop-light', chromium, 1365, false],
      ['chromium-desktop-dark', chromium, 1365, true],
      ['webkit-phone-dark', webkit, 390, true],
      ['webkit-narrow-light', webkit, 320, false],
    ]) {
      phase = name;
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height: width < 500 ? 844 : 900 },
        isMobile: width < 500,
        hasTouch: width < 500,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      page.setDefaultTimeout(10000);
      page.setDefaultNavigationTimeout(60000);
      const errors = [],
        requests = [],
        offOrigin = [],
        checks = [];
      page.on('pageerror', (error) => errors.push(error.message));
      await page.addInitScript((dark) => {
        const apply = () => document.documentElement.classList.toggle('dark', dark);
        if (document.documentElement) apply();
        else document.addEventListener('DOMContentLoaded', apply, { once: true });
      }, dark);
      await page.routeWebSocket('**/ws', (ws) => ws.close());
      await page.route('**/*', async (route) => {
        const req = route.request(),
          url = new URL(req.url());
        if (url.origin !== origin) {
          offOrigin.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        requests.push({ path: url.pathname, query: url.search, method: req.method() });
        assert.equal(req.method(), 'GET');
        if (url.pathname.includes('metrics-store/history'))
          return route.fulfill({
            json: {
              resourceType: 'vm',
              resourceId: url.searchParams.get('resourceId'),
              range: '24h',
              start: Date.UTC(2026, 9, 3, 14),
              end: Date.UTC(2026, 9, 4, 14),
              metrics: {},
              source: 'store',
            },
          });
        if (url.pathname === '/api/resources')
          return route.fulfill({ json: { data: [], total: 0, page: 1, limit: 500 } });
        if (url.pathname.includes('license'))
          return route.fulfill({ json: { capabilities: [], limits: [], max_history_days: 90 } });
        if (url.pathname.includes('metadata')) return route.fulfill({ json: {} });
        return route.fulfill({
          json: { success: true, data: [], alerts: [], anomalies: [], config: null },
        });
      });
      await page.goto(`${origin}/browser-tests/guest-agent-coverage.html`, {
        waitUntil: 'domcontentloaded',
      });
      await page.waitForFunction(() => Boolean(window.__guestAgentCoverage));
      const row = page.locator('[data-guest-id]').first();
      if (width > 500) {
        await row.getByRole('button', { name: 'Expand backup-guest', exact: true }).focus();
        await page.keyboard.press('Enter');
      } else await row.tap();
      const drawer = page.getByRole('region', { name: 'Guest details' });
      await drawer.waitFor({ state: 'visible' });
      const precaution = page.getByTestId('guest-read-precaution');
      const overview = page.getByTestId('guest-technical-details');
      const stateCell = () =>
        overview.getByText('Guest-agent reads', { exact: true }).locator('..');
      const update = async (value) => {
        await page.evaluate((v) => window.__guestAgentCoverage.update(v), value);
      };
      const capture = async (state) => {
        const filename = `${name}-${state}.png`,
          file = path.join(output, filename);
        await page.screenshot({ path: file, fullPage: true });
        screenshots.push({ filename, sha256: hash(file) });
      };
      const checkLayout = async (locator) => {
        const box = await locator.boundingBox();
        assert(box && box.x >= 0 && box.x + box.width <= width + 1);
        assert(await locator.evaluate((el) => el.scrollWidth <= el.clientWidth + 1));
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      };
      await precaution.waitFor();
      assert((await precaution.innerText()).includes('does not prove thaw'));
      assert((await precaution.innerText()).includes('filesystems covered by the backup'));
      assert((await stateCell().innerText()).includes('Deferred'));
      assert((await overview.innerText()).includes('Pulse Agent observed'));
      assert(!(await overview.innerText()).includes('Pulse Agent connected'));
      await checkLayout(precaution);
      await checkLayout(stateCell());
      const coverage = overview.getByText('Pulse Agent observed', { exact: true });
      assert(await coverage.evaluate((el) => el.scrollWidth <= el.clientWidth + 1));
      await capture('deferred-overview');
      checks.push(
        'Current QGA deferral independent of observed Pulse version; readable safety and covered-write guidance',
      );
      await page.evaluate(() => {
        window.__originalCoverageDrawer = document.querySelector('[aria-label="Guest details"]');
      });
      for (const [state, label] of [
        ['expected-unreachable', 'Unreachable'],
        ['not-running', 'Not running'],
        ['disabled', 'Disabled'],
        ['available', 'Reported available'],
        ['private-provider-raw', 'Unknown'],
      ]) {
        await update({ ...initial, state, reason: '', lock: '', backup: false });
        await page.waitForFunction(
          (label) =>
            document
              .querySelector('[data-testid="guest-technical-details"]')
              .textContent.includes(label),
          label,
        );
        assert((await stateCell().innerText()).includes(label));
        assert.equal(await precaution.count(), 0);
        assert(!(await overview.innerText()).includes('private-provider-raw'));
        await update({ ...initial, state, reason: '', lock: '', backup: false, version: '' });
        assert((await stateCell().innerText()).includes(label));
        assert(!(await overview.innerText()).includes('Pulse 6.4.5'));
      }
      checks.push(
        'Every declared/unknown state stays visible without a version; no raw provider text or fabricated connection',
      );
      await update({ ...initial, version: '', assignment: true });
      assert((await overview.innerText()).includes('Node agent assigned'));
      assert((await stateCell().innerText()).includes('Deferred'));
      await page.getByRole('tab', { name: 'History', exact: true }).click();
      await precaution.waitFor();
      await page.getByText('No stored history in this range', { exact: true }).first().waitFor();
      await update({
        ...initial,
        state: 'expected-unreachable',
        reason: 'agent-timeout',
        lock: '',
        backup: false,
        assignment: true,
      });
      assert((await precaution.innerText()).includes('Completion is uncertain'));
      await checkLayout(precaution);
      await capture('uncertain-history');
      await page.getByRole('tab', { name: 'Manage', exact: true }).click();
      assert(await precaution.isVisible());
      assert.equal(
        await page.evaluate(
          () =>
            document.querySelector('[aria-label="Guest details"]') ===
            window.__originalCoverageDrawer,
        ),
        true,
      );
      checks.push(
        'Same guest retains drawer/tab identity; precaution persists across History/Manage; assignment is not connectivity',
      );
      await update({
        ...initial,
        state: 'available',
        reason: '',
        lock: '',
        backup: false,
        diskUsed: 7.5 * 1024 ** 3,
      });
      await page.getByRole('tab', { name: 'Overview', exact: true }).click();
      assert.equal(await precaution.count(), 0);
      assert((await stateCell().innerText()).includes('Reported available'));
      assert(
        (await stateCell().getByText('Reported available').getAttribute('title')).includes(
          'does not independently confirm thaw',
        ),
      );
      assert((await row.locator('[data-workload-col="disk"]').innerText()).includes('75%'));
      await checkLayout(stateCell());
      await capture('reported-available');
      checks.push(
        'Fresh replacement withdraws precaution and resumes ordinary existing values, without asserting thaw',
      );
      await update(initial);
      await precaution.waitFor();
      const link = precaution.getByRole('link', { name: 'Backup safety guidance' });
      assert.equal(await link.getAttribute('href'), '/docs/VM_DISK_MONITORING');
      await link.focus();
      assert.equal(await link.evaluate((el) => document.activeElement === el), true);
      if (width > 500) await page.keyboard.press('Enter');
      else await link.tap();
      await page.getByRole('heading', { name: /VM Disk Monitoring/ }).waitFor();
      const article = page.locator('article');
      assert(
        (await article.innerText()).includes(
          'Pulse monitoring and alerts are unavailable while stopped',
        ),
      );
      assert((await article.innerText()).includes('filesystems covered by the backup'));
      if (width === 320) await capture('safety-guide');
      assert.equal(
        requests.filter((r) => r.path.includes('diagnostics') || r.method !== 'GET').length,
        0,
      );
      assert.equal(offOrigin.length, 0);
      assert.deepEqual(errors, []);
      assert.deepEqual(
        Object.fromEntries(runtime.map((p) => [p, hash(path.join('/workspace', p))])),
        hashes,
      );
      checks.push(
        'Keyboard/touch guidance opens real shipped Docs; no diagnostic/guest mutation or off-origin request',
      );
      results.push({ name, width, dark, browser: await browser.version(), requests, checks });
      await browser.close();
      browser = null;
    }
    fs.writeFileSync(
      path.join(output, 'result.json'),
      JSON.stringify(
        {
          result: 'passed',
          verifiedAt: new Date().toISOString(),
          playwrightVersion,
          content_sha256: hashes,
          results,
          screenshots,
          limitations: [
            'Synthetic production-component proof, not native QGA availability, thaw, covered writes, installed acceptance or release qualification.',
            'No memory provenance inferred from filesystem state; current Resource API exposes no memory source.',
          ],
        },
        null,
        2,
      ) + '\n',
    );
    process.stdout.write(
      JSON.stringify({
        result: 'passed',
        groups: results.length,
        screenshots: screenshots.length,
      }) + '\n',
    );
  } catch (error) {
    fs.writeFileSync(
      path.join(output, 'failure.json'),
      JSON.stringify({ phase, error: String(error), results, screenshots }, null, 2) + '\n',
    );
    throw error;
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  process.stderr.write(String(error.stack || error) + '\n');
  process.exitCode = 1;
});
