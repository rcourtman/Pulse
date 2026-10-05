const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/discovery-safety-proof/browser';
const runtime = [
  'frontend-modern/src/components/Workloads/GuestDrawer.tsx',
  'frontend-modern/src/components/Discovery/DiscoveryTab.tsx',
  'frontend-modern/src/components/Discovery/useDiscoveryTabState.ts',
];
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const initial = {
  state: 'deferred',
  reason: 'prev-vm-locked',
  lock: 'backup',
  backup: true,
  version: '6.4.5',
  assignment: true,
  diskUsed: 5 * 1024 ** 3,
};
const healthy = { ...initial, state: 'available', reason: '', lock: '', backup: false };
const saved = {
  id: 'vm:fixture-node-agent:101',
  resource_type: 'vm',
  resource_id: '101',
  target_id: 'fixture-node-agent',
  hostname: 'backup-guest',
  service_type: 'home-assistant',
  service_name: 'Saved Home Assistant',
  service_version: 'test',
  category: 'home_automation',
  cli_access: '',
  facts: [],
  config_paths: [],
  data_paths: [],
  log_paths: [],
  ports: [],
  user_notes: '',
  user_secrets: {},
  confidence: 0.9,
  ai_reasoning: 'Synthetic saved evidence',
  discovered_at: '2026-10-03T12:00:00Z',
  updated_at: '2026-10-03T12:00:00Z',
  discovery_engine_version: 1,
};
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const report = {
    runtime_sha256: Object.fromEntries(runtime.map((p) => [p, hash(path.join('/workspace', p))])),
    playwright: require('playwright/package.json').version,
    groups: [],
    screenshots: [],
    cleanup: {},
  };
  assert.equal(
    report.playwright,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(output, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5297, strictPort: true, watch: null },
  });
  const origin = 'http://127.0.0.1:5297';
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
      report.groups.push({
        name,
        width,
        dark,
        browser: browser.version(),
        checks,
        requests,
        errors,
        offOrigin,
      });
      await page.routeWebSocket('**/ws*', () => {});
      let hold = false,
        release;
      page.on('pageerror', (e) => errors.push(e.message));
      await page.addInitScript((dark) => {
        const apply = () => document.documentElement.classList.toggle('dark', dark);
        if (document.documentElement) apply();
        else document.addEventListener('DOMContentLoaded', apply, { once: true });
      }, dark);
      await page.route('**/*', async (route) => {
        const req = route.request(),
          url = new URL(req.url());
        if (url.origin !== origin) {
          offOrigin.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        requests.push({
          path: url.pathname,
          query: url.search,
          method: req.method(),
          body: req.postData(),
        });
        if (url.pathname === '/api/discovery/vm/fixture-node-agent/101') {
          if (req.method() === 'POST') {
            if (hold)
              await new Promise((resolve) => {
                release = resolve;
              });
            return route.fulfill({ json: saved });
          }
          return route.fulfill({ json: saved });
        }
        assert.equal(req.method(), 'GET');
        if (url.pathname.startsWith('/api/discovery/info/'))
          return route.fulfill({
            json: {
              ai_provider: { provider: 'test', model: 'test', is_local: true, label: 'Local' },
              commands: [],
              command_categories: [],
            },
          });
        if (url.pathname === '/api/ai/agents')
          return route.fulfill({
            json: {
              count: 1,
              agents: [
                {
                  agent_id: 'fixture-node-agent',
                  hostname: 'pve-a',
                  version: '6.4.5',
                  platform: 'linux',
                },
              ],
            },
          });
        if (url.pathname.includes('metrics-store/history'))
          return route.fulfill({
            json: {
              resourceType: 'vm',
              resourceId: url.searchParams.get('resourceId'),
              range: '24h',
              start: Date.UTC(2026, 9, 3, 12),
              end: Date.UTC(2026, 9, 4, 12),
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
      await page.goto(`${origin}/browser-tests/guest-discovery-safety.html`, {
        waitUntil: 'domcontentloaded',
      });
      await page.waitForFunction(() => Boolean(window.__guestDiscoverySafety));
      const row = page.locator('[data-guest-id]').first();
      if (width > 500) {
        await row.getByRole('button', { name: 'Expand backup-guest', exact: true }).focus();
        await page.keyboard.press('Enter');
      } else await row.tap();
      await page.getByRole('tab', { name: 'Discovery', exact: true }).click();
      const run = page.getByRole('button', { name: 'Run Discovery', exact: true });
      await run.waitFor();
      const update = async (value) =>
        page.evaluate((v) => window.__guestDiscoverySafety.update(v), value);
      const postCount = () => requests.filter((r) => r.method === 'POST').length;
      const block = page.getByTestId('discovery-run-block');
      const capture = async (state) => {
        const filename = `${name}-${state}.png`,
          file = path.join(output, filename);
        await run.scrollIntoViewIfNeeded();
        await page.screenshot({ path: file, fullPage: true });
        report.screenshots.push({ filename, sha256: hash(file) });
      };
      const layout = async () => {
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        for (const loc of [block, page.getByTestId('guest-read-precaution')]) {
          const box = await loc.boundingBox();
          assert(box && box.x >= 0 && box.x + box.width <= width + 1);
          assert(await loc.evaluate((el) => el.scrollWidth <= el.clientWidth + 1));
        }
      };
      await page.getByText('Saved Home Assistant', { exact: true }).first().waitFor();
      assert(await run.isDisabled());
      assert.equal(postCount(), 0);
      assert((await block.innerText()).includes('does not prove thaw'));
      assert(await run.getAttribute('aria-describedby'));
      assert.equal(await block.getAttribute('id'), await run.getAttribute('aria-describedby'));
      await layout();
      await capture('locked-saved');
      checks.push(
        'Native canonical projection, row and real drawer/Discovery: assigned node agent, saved results readable, backup blocks live run with associated wrapping reason',
      );
      await page.evaluate(() => {
        window.__originalSafetyButton = document.querySelector(
          '[aria-describedby^="discovery-run-block-"]',
        );
      });
      for (const reason of [
        'vm-locked',
        'lock-unverified',
        'agent-busy',
        'agent-cooldown',
        'agent-response-incomplete',
        'agent-capacity',
        'invalid-guest-key',
        'agent-timeout',
      ]) {
        for (const prefix of ['', 'prev-']) {
          await update({ ...healthy, reason: prefix + reason });
          assert(await run.isDisabled());
          assert.equal(postCount(), 0);
          assert(
            await page.evaluate(
              () =>
                document.querySelector('[aria-describedby^="discovery-run-block-"]') ===
                window.__originalSafetyButton,
            ),
          );
        }
      }
      await update({ ...healthy, reason: 'prev-agent-timeout' });
      await capture('uncertain-saved');
      await page.getByRole('tab', { name: 'History', exact: true }).click();
      assert(await page.getByTestId('guest-read-precaution').isVisible());
      await page.getByRole('tab', { name: 'Discovery', exact: true }).click();
      assert(await run.isDisabled());
      checks.push(
        'All eight read deferrals, retained variants and same-ID tab/snapshot updates block without remount, POST or automatic rescan',
      );
      await update(healthy);
      assert(await run.isEnabled());
      assert.equal(await block.count(), 0);
      assert.equal(postCount(), 0);
      await capture('ready-manual');
      await run.focus();
      await page.keyboard.press('Enter');
      await page.waitForFunction(() => document.body.textContent.includes('Discovery complete!'));
      assert.equal(postCount(), 1);
      const first = requests.find((r) => r.method === 'POST');
      assert.deepEqual(JSON.parse(first.body), { force: true, hostname: 'backup-guest' });
      checks.push(
        'Explicit healthy keyboard run sends exactly the original target/body once; clearing safety pause alone sends nothing',
      );
      await page.waitForFunction(() => !document.body.textContent.includes('Discovery complete!'));
      hold = true;
      const secondPost = page.waitForRequest(
        (req) =>
          req.method() === 'POST' &&
          new URL(req.url()).pathname === '/api/discovery/vm/fixture-node-agent/101',
      );
      if (width < 500) await run.tap();
      else await run.click();
      await secondPost;
      await page.waitForFunction(() => document.body.textContent.includes('Scanning...'));
      assert.equal(postCount(), 2);
      await update({ ...initial, reason: 'agent-timeout', lock: '', backup: false });
      assert(await page.getByRole('button', { name: 'Scanning...', exact: true }).isDisabled());
      assert(await block.isVisible());
      assert((await page.locator('body').innerText()).includes('Scanning...'));
      assert.equal(postCount(), 2);
      assert(release);
      release();
      await page.waitForFunction(() => document.body.textContent.includes('Discovery complete!'));
      assert(await run.isDisabled());
      assert.equal(postCount(), 2);
      await capture('inflight-completed-blocked');
      checks.push(
        'A newly blocked snapshot neither hides nor cancels dispatched work; original completion is shown and no further run is sent',
      );
      assert.deepEqual(errors, []);
      assert.deepEqual(offOrigin, []);
      await browser.close();
      browser = null;
    }
    report.result = 'passed';
    report.verified_at = new Date().toISOString();
  } catch (error) {
    report.result = 'failed';
    report.phase = phase;
    report.error = String(error.stack || error);
    throw error;
  } finally {
    if (browser) await browser.close();
    report.cleanup.browser_closed = true;
    await server.close();
    report.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
  }
})().catch((error) => {
  console.error(error.stack || String(error));
  process.exitCode = 1;
});
