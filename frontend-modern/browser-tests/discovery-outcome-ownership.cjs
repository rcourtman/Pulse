const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/discovery-ownership-proof/browser';
const origin = 'http://127.0.0.1:5333';
const runtime = [
  'frontend-modern/src/components/Discovery/DiscoveryTab.tsx',
  'frontend-modern/src/components/Discovery/useDiscoveryTabState.ts',
  'frontend-modern/src/components/Workloads/GuestDrawer.tsx',
];
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const saved = (id, service = `Saved Home Assistant ${id}`) => ({
  id: `vm:fixture-node-agent:${id}`,
  resource_type: 'vm',
  resource_id: id,
  target_id: 'fixture-node-agent',
  hostname: `guest-${id}`,
  service_type: 'home-assistant',
  service_name: service,
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
});
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const report = {
    result: 'incomplete',
    playwright: require('playwright/package.json').version,
    runtime_sha256: Object.fromEntries(runtime.map((p) => [p, hash('/workspace/' + p)])),
    groups: [],
    screenshots: [],
    cleanup: {},
    limits:
      'Production DiscoveryTab/state/CSS, synthetic API and production event-bus progress. No native QGA, automatic collector, original freeze cause, thaw, covered filesystem writes, installed, physical-phone or release acceptance.',
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
    cacheDir: output + '/cache',
    server: { host: '127.0.0.1', port: 5333, strictPort: true, watch: null },
  });
  let browser,
    phase = 'server';
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['desktop-light', chromium, 1365, false],
      ['desktop-dark', chromium, 1365, true],
      ['phone-dark', webkit, 390, true],
      ['narrow-light', webkit, 320, false],
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
      const group = {
        name,
        width,
        dark,
        browser: browser.version(),
        checks: [],
        requests: [],
        errors: [],
        offOrigin: [],
      };
      report.groups.push(group);
      page.on('pageerror', (error) => group.errors.push(error.message));
      await page.routeWebSocket('**/ws*', () => {});
      await page.addInitScript((dark) => {
        const apply = () => document.documentElement.classList.toggle('dark', dark);
        if (document.documentElement) apply();
        else document.addEventListener('DOMContentLoaded', apply, { once: true });
      }, dark);
      let fail = false;
      const releases = new Map();
      const arrivals = new Map();
      const stored = new Map();
      await page.route('**/*', async (route) => {
        const request = route.request(),
          url = new URL(request.url());
        if (url.origin !== origin) {
          group.offOrigin.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        group.requests.push({
          path: url.pathname,
          method: request.method(),
          body: request.postData(),
        });
        const target = url.pathname.match(/^\/api\/discovery\/vm\/fixture-node-agent\/(101|102)$/);
        if (target) {
          const id = target[1];
          if (request.method() === 'GET')
            return route.fulfill({ json: stored.get(id) || saved(id) });
          assert.equal(request.method(), 'POST');
          if (fail)
            return route.fulfill({
              status: 409,
              json: { error: 'Guest execution paused: backup lock observed' },
            });
          await new Promise((resolve) => {
            releases.set(id, resolve);
            arrivals.get(id)?.();
          });
          const result = saved(id, id === '101' ? 'Old guest result' : 'Current guest result');
          stored.set(id, result);
          return route.fulfill({ json: result });
        }
        assert.equal(request.method(), 'GET');
        const responses = {
          '/api/discovery/info/vm': {
            ai_provider: { provider: 'test', model: 'test', is_local: true, label: 'Local' },
            commands: [],
            command_categories: [],
          },
          '/api/ai/agents': {
            count: 1,
            agents: [{ agent_id: 'fixture-node-agent', hostname: 'pve1' }],
          },
          '/api/ai/settings': { discovery_enabled: true, enabled: false },
          '/api/license/runtime-capabilities': {
            capabilities: [],
            limits: [],
            hosted_mode: false,
            runtime: { build: 'community' },
            blocked_capabilities: [],
          },
        };
        return route.fulfill({ json: responses[url.pathname] || {} });
      });
      const capture = async (suffix) => {
        await page.screenshot({ path: `${output}/${name}-${suffix}.png`, fullPage: true });
        report.screenshots.push(`${name}-${suffix}.png`);
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      };
      const emit = (progress) =>
        page.evaluate((progress) => window.discoveryOwnership.progress(progress), progress);
      const run = page.getByRole('button', { name: 'Run Discovery', exact: true });
      const pressRun = async () => {
        if (width >= 500) {
          await run.focus();
          await page.keyboard.press('Enter');
        } else await run.tap();
      };
      await page.goto(origin + '/browser-tests/discovery-outcome-ownership.html');
      await page.getByText('Saved Home Assistant 101', { exact: true }).waitFor();
      await run.waitFor();
      await page.waitForFunction(
        () =>
          !Array.from(document.querySelectorAll('button')).find(
            (b) => b.textContent.trim() === 'Run Discovery',
          ).disabled,
      );
      const before = group.requests.length;
      await emit({
        resource_id: 'vm:fixture-node-agent:101',
        status: 'running',
        current_step: 'Collecting guest evidence',
        percent_complete: 20,
      });
      await page.getByText('Collecting guest evidence', { exact: true }).waitFor();
      await emit({
        resource_id: 'vm:fixture-node-agent:101',
        status: 'completed',
        error: 'Guest execution paused: backup lock observed',
        current_step: 'Guest execution paused',
        percent_complete: 20,
      });
      const alert = page.getByRole('alert');
      await alert.waitFor();
      await page.waitForTimeout(650);
      assert((await alert.innerText()).includes('Guest execution paused: backup lock observed'));
      assert.equal(group.requests.length, before);
      assert(await page.getByText('Saved Home Assistant 101', { exact: true }).isVisible());
      assert.equal(await page.getByText('Discovery complete!', { exact: true }).count(), 0);
      await capture('retained-background-pause');
      group.checks.push(
        'Background completed-with-error remains an accessible failure with saved evidence and no retry or refresh',
      );
      const dismiss = page.getByRole('button', { name: 'Dismiss Discovery error' });
      await dismiss.focus();
      await page.keyboard.press('Enter');
      assert.equal(await alert.count(), 0);
      group.checks.push('The error dismiss action has a keyboard-accessible name');
      const firstPost = new Promise((resolve) => arrivals.set('101', resolve));
      await pressRun();
      await firstPost;
      await page.waitForFunction(() => document.body.textContent.includes('Scanning...'));
      await page.evaluate(() => window.discoveryOwnership.target('102'));
      await page.getByText('Saved Home Assistant 102', { exact: true }).waitFor();
      const secondPost = new Promise((resolve) => arrivals.set('102', resolve));
      await pressRun();
      await secondPost;
      await page.waitForFunction(() => document.body.textContent.includes('Scanning...'));
      assert(releases.has('101'));
      assert(releases.has('102'));
      releases.get('101')();
      await page.waitForTimeout(150);
      assert.equal(await page.getByText('Old guest result', { exact: true }).count(), 0);
      assert.equal(await page.getByText('Discovery complete!', { exact: true }).count(), 0);
      assert(await page.getByRole('button', { name: 'Scanning...', exact: true }).isDisabled());
      await emit({
        resource_id: 'vm:fixture-node-agent:102',
        status: 'completed',
        percent_complete: 100,
      });
      assert(await page.getByRole('button', { name: 'Scanning...', exact: true }).isDisabled());
      assert.equal(await page.getByText('Discovery complete!', { exact: true }).count(), 0);
      releases.get('102')();
      await page.getByText('Current guest result', { exact: true }).waitFor();
      await page.getByText('Discovery complete!', { exact: true }).waitFor();
      group.checks.push(
        'Late old-target HTTP and scanner completion cannot replace the current guest or finish its active HTTP request',
      );
      const posts = group.requests.filter((r) => r.method === 'POST');
      assert.equal(posts.length, 2);
      assert.deepEqual(
        posts.map((p) => JSON.parse(p.body)),
        [
          { force: true, hostname: 'guest-101' },
          { force: true, hostname: 'guest-102' },
        ],
      );
      await capture('current-guest-result');
      await emit({ resource_id: 'vm:fixture-node-agent:102', status: 'completed' });
      await page.waitForTimeout(650);
      assert.equal(await page.getByText('Discovery complete!', { exact: true }).count(), 0);
      assert(await page.getByText('Current guest result', { exact: true }).isVisible());
      assert.equal(group.requests.filter((r) => r.method === 'POST').length, 2);
      group.checks.push(
        'Later completion cannot strand the old HTTP success banner or issue an automatic scan',
      );
      fail = true;
      await pressRun();
      await alert.waitFor();
      assert((await alert.innerText()).includes('Guest execution paused: backup lock observed'));
      assert(await page.getByText('Current guest result', { exact: true }).isVisible());
      assert.equal(await page.getByText('Discovery complete!', { exact: true }).count(), 0);
      await page.waitForTimeout(650);
      assert.equal(group.requests.filter((r) => r.method === 'POST').length, 3);
      group.checks.push(
        'A failed explicit run retains prior saved evidence, visible failure and no automatic retry',
      );
      assert.deepEqual(group.errors, []);
      assert.deepEqual(group.offOrigin, []);
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
    fs.writeFileSync(output + '/result.json', JSON.stringify(report, null, 2) + '\n');
  }
})().catch((error) => {
  console.error(error.stack || String(error));
  process.exitCode = 1;
});
