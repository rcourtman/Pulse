const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const output = '/workspace/tmp/resource-guest-safety-settled-proof';
const paths = [
  'frontend-modern/src/components/Workloads/guestDrawerModel.ts',
  'frontend-modern/src/components/Infrastructure/resourceDetailDiscoveryModel.ts',
  'frontend-modern/src/components/Infrastructure/useResourceDetailDrawerDerivedState.ts',
  'frontend-modern/src/components/Infrastructure/ResourceDetailDrawer.tsx',
  'frontend-modern/src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx',
];
const reasons = [
  'vm-locked',
  'lock-unverified',
  'agent-busy',
  'agent-cooldown',
  'agent-response-incomplete',
  'agent-capacity',
  'invalid-guest-key',
  'agent-timeout',
];
const saved = {
  id: 'vm:fixture-node-agent:101',
  resource_type: 'vm',
  resource_id: '101',
  target_id: 'fixture-node-agent',
  hostname: 'Backup guest',
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
  discovered_at: '2026-10-04T12:00:00Z',
  updated_at: '2026-10-04T12:00:00Z',
  discovery_engine_version: 1,
};
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const report = {
    runtime_sha256: Object.fromEntries(paths.map((p) => [p, hash(path.join('/workspace', p))])),
    playwright: require('playwright/package.json').version,
    groups: [],
    cleanup: {},
    limits:
      'Synthetic local snapshots and HTTP only. No native QGA, diagnostics, backup, thaw, filesystem write, service resumption, physical phone, publication or release acceptance.',
  };
  assert.equal(
    report.playwright,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const { createServer } =
    await import('/workspace/frontend-modern/node_modules/vite/dist/node/index.js');
  let server, browser, phase;
  try {
    for (const [name, engine, width, dark] of [
      ['final-chromium', chromium, 1365, false],
      ['final-webkit-phone', webkit, 320, true],
    ]) {
      phase = name;
      const module = '/workspace/frontend-modern';
      process.chdir(module);
      server = await createServer({
        root: module,
        configFile: path.join(module, 'vite.config.ts'),
        cacheDir: path.join(output, name, 'vite'),
        server: {
          host: '127.0.0.1',
          port: 5299,
          strictPort: true,
          watch: null,
          fs: { allow: ['/workspace'] },
        },
      });
      await server.listen();
      const origin = 'http://127.0.0.1:5299';
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
      });
      page.setDefaultTimeout(10000);
      const group = {
        name,
        width,
        dark,
        browser: browser.version(),
        requests: [],
        errors: [],
        offOrigin: [],
        checks: [],
        screenshots: [],
      };
      report.groups.push(group);
      const check = (label) => group.checks.push(label);
      const capture = async (label) => {
        const file = path.join(output, `${name}-${label}.png`);
        await page.screenshot({ path: file, fullPage: true });
        group.screenshots.push({
          path: file.replace('/workspace/', ''),
          sha256: hash(file),
          bytes: fs.statSync(file).size,
        });
      };
      page.on('pageerror', (e) => group.errors.push(e.message));
      await page.routeWebSocket('**/ws*', () => {});
      await page.route('**/*', async (route) => {
        const req = route.request(),
          url = new URL(req.url());
        if (url.origin !== origin) {
          group.offOrigin.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        group.requests.push({
          path: url.pathname,
          query: url.search,
          method: req.method(),
          body: req.postData(),
        });
        if (
          url.pathname === '/api/discovery/vm/fixture-node-agent/101' ||
          url.pathname === '/api/discovery/agent/fixture-node-agent/101'
        )
          return route.fulfill({ json: saved });
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
                  commands_enabled: true,
                  connected: true,
                },
              ],
            },
          });
        if (url.pathname === '/api/license/runtime-capabilities')
          return route.fulfill({
            json: {
              capabilities: [],
              limits: [],
              max_history_days: 7,
              hosted_mode: false,
              runtime: { build: 'community' },
              blocked_capabilities: [],
            },
          });
        return route.fulfill({
          json: {
            capabilities: [],
            relationships: [],
            recentChanges: [],
            counts: {},
            audits: [],
            count: 0,
            data: [],
            enabled: false,
          },
        });
      });
      await page.goto(`${origin}/browser-tests/resource-guest-safety.html`, {
        waitUntil: 'domcontentloaded',
        timeout: 60000,
      });
      await page.evaluate((d) => document.documentElement.classList.toggle('dark', d), dark);
      const activate = async (locator) => (width < 500 ? locator.tap() : locator.press('Enter'));
      await activate(page.getByRole('tab', { name: 'Manage', exact: true }));
      const analysis = page.getByTestId('resource-access-analysis');
      await activate(analysis.getByRole('button', { name: 'Open analysis' }));
      await analysis.getByText('Saved Home Assistant', { exact: true }).waitFor();
      const button = analysis.getByRole('button', { name: 'Update Discovery' });
      await button.waitFor();
      const posts = () => group.requests.filter((r) => r.method === 'POST');
      const notice = page.getByTestId('resource-guest-read-precaution');
      await notice.waitFor();
      assert(await button.isDisabled());
      assert.match(await notice.innerText(), /An OK backup or a running VM does not prove thaw/);
      assert.equal(
        await notice.getByRole('link', { name: 'Backup safety guidance' }).getAttribute('href'),
        '/docs/VM_DISK_MONITORING',
      );
      const control = await button.elementHandle();
      const update = (evidence, legacy = false, type = 'vm') =>
        page.evaluate(
          ({ evidence, legacy, type }) =>
            window.__resourceGuestSafety.update(evidence, legacy, type),
          { evidence, legacy, type },
        );
      for (const legacy of [false, true]) {
        for (const reason of reasons) {
          for (const diskStatusReason of [reason, `prev-${reason}`]) {
            await update({ diskStatusReason, guestAgentStatus: 'available' }, legacy);
            assert(await button.isDisabled());
            assert(await notice.isVisible());
            assert.equal(posts().length, 0);
          }
        }
        for (const evidence of [
          { lock: 'backup' },
          { backupInProgress: true },
          { guestAgentStatus: 'deferred' },
        ]) {
          await update(evidence, legacy);
          assert(await button.isDisabled());
        }
      }
      assert(await button.evaluate((node, old) => node === old, control));
      assert.equal(await analysis.getByText('Saved Home Assistant', { exact: true }).count(), 1);
      check(
        'All 32 reason/facet variants and six operation variants block the same mounted rescan; saved evidence remains readable; zero POST.',
      );
      await update({ diskStatusReason: 'prev-agent-timeout' }, true);
      const geometry = await notice.boundingBox();
      assert(geometry && geometry.x >= 0 && geometry.x + geometry.width <= width);
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth));
      const describedBy = await button.getAttribute('aria-describedby');
      assert(describedBy && (await page.locator(`#${describedBy}`).isVisible()));
      await capture('paused-saved');
      check('Disabled rescan is described and narrow geometry has no horizontal overflow.');
      await update({ guestAgentStatus: 'available' });
      assert(await button.isEnabled());
      assert.equal(await notice.count(), 0);
      assert(await button.evaluate((node, old) => node === old, control));
      assert.equal(posts().length, 0);
      check(
        'Healthy same-VM evidence clears the guard without an automatic scan or remount; no thaw claim.',
      );
      await capture('cleared');
      const response = page.waitForResponse(
        (r) =>
          r.url() === `${origin}/api/discovery/vm/fixture-node-agent/101` &&
          r.request().method() === 'POST',
      );
      await activate(button);
      await response;
      assert.equal(posts().length, 1);
      assert.equal(posts()[0].path, '/api/discovery/vm/fixture-node-agent/101');
      check(
        'An explicit permitted keyboard/touch action sends exactly one synthetic local request to the unchanged VM target.',
      );
      await update({ lock: 'backup' });
      assert(await button.isDisabled());
      await activate(page.getByRole('tab', { name: 'Overview', exact: true }));
      assert(await notice.isVisible());
      check('Precaution and guide stay visible on Overview after leaving Analysis.');
      await update({ lock: 'backup' }, false, 'agent');
      await activate(page.getByRole('tab', { name: 'Discovery', exact: true }));
      const agentButton = page.getByRole('button', { name: 'Run Discovery', exact: true });
      await agentButton.waitFor();
      assert(await agentButton.isEnabled());
      assert.equal(await notice.count(), 0);
      check('Ordinary agent Discovery tab is not paused by parent-side PVE evidence.');
      assert.deepEqual(group.errors, []);
      assert.deepEqual(group.offOrigin, []);
      check('No page error, off-origin request, native target or background POST.');
      await browser.close();
      browser = null;
      await server.close();
      server = null;
    }
  } catch (error) {
    report.failure = { phase, message: error.message };
    throw error;
  } finally {
    if (browser) await browser.close();
    if (server) await server.close();
    report.cleanup = { browser_closed: true, server_closed: true };
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2));
  }
  console.log(
    JSON.stringify({
      groups: report.groups.length,
      checks: report.groups.reduce((n, g) => n + g.checks.length, 0),
      screenshots: report.groups.reduce((n, g) => n + g.screenshots.length, 0),
      cleanup: report.cleanup,
    }),
  );
})().catch((e) => {
  console.error(e.stack);
  process.exitCode = 1;
});
