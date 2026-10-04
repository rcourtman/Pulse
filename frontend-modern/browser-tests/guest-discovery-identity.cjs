const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/guest-discovery-browser';
const origin = 'http://127.0.0.1:5326';
const runtime = [
  'frontend-modern/src/components/Workloads/GuestDrawer.tsx',
  'frontend-modern/src/components/Workloads/useGuestDrawerState.ts',
  'frontend-modern/src/api/discovery.ts',
];
const hash = (p) => crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex');
const record = (agent) => ({
  id: `vm:${agent}:100`,
  resource_type: 'vm',
  resource_id: '100',
  agent_id: agent,
  target_id: agent,
  hostname: agent,
  service_name: `Service ${agent}`,
  service_type: 'dashboard',
  service_version: '1.0',
  category: 'web_server',
  confidence: 0.95,
  ports: [],
  facts: [],
  config_paths: [],
  data_paths: [],
  log_paths: [],
  user_notes: `Notes ${agent}`,
  user_secrets: {},
  discovered_at: '2026-10-04T06:00:00Z',
  updated_at: '2026-10-04T06:00:00Z',
  scan_duration: 1,
  suggested_url: `https://${agent}.example.test/`,
  discovery_engine_version: 1,
});
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const result = {
    result: 'incomplete',
    playwright: require('playwright/package.json').version,
    runtime_hashes: Object.fromEntries(runtime.map((p) => [p, hash('/workspace/' + p)])),
    cases: [],
    screenshots: [],
    limits:
      'Synthetic local HTTP responses, real production GuestDrawer/DiscoveryTab/Manage/History, clients and CSS. No native target, agent command, guest freeze/thaw, appliance identity, installed recovery, physical phone, source publication or release acceptance.',
  };
  assert.equal(
    result.playwright,
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
    server: { host: '127.0.0.1', port: 5326, strictPort: true },
  });
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['chromium-desktop', chromium, 1365, false],
      ['webkit-phone', webkit, 390, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height: width < 768 ? 844 : 900 },
        hasTouch: width < 768,
        isMobile: width < 768,
        colorScheme: dark ? 'dark' : 'light',
      });
      const item = { name, browser: browser.version(), requests: [], errors: [], checks: [] };
      result.cases.push(item);
      page.on('pageerror', (e) => item.errors.push(e.message));
      const modes = new Map();
      let pendingReads = [];
      let pendingScan;
      await page.route('**/*', async (route) => {
        const request = route.request();
        const url = new URL(request.url());
        if (url.origin !== origin) {
          item.requests.push({ path: url.pathname, blocked: true });
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        item.requests.push({ path: url.pathname, method: request.method() });
        const respond = (body, status = 200) =>
          route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });
        const discovery = url.pathname.match(/^\/api\/discovery\/vm\/([^/]+)\/100$/);
        if (discovery) {
          const agent = decodeURIComponent(discovery[1]);
          if (request.method() === 'POST') {
            pendingScan = { route, agent };
            return;
          }
          const mode = modes.get(agent);
          if (mode === 'pending') {
            pendingReads.push({ route, agent });
            return;
          }
          if (typeof mode === 'number')
            return respond({ error: 'private diagnostic must not render' }, mode);
          if (mode === 'missing') return respond({}, 404);
          return respond(record(agent));
        }
        if (url.pathname.startsWith('/api/discovery/info/'))
          return respond({
            ai_provider: { provider: 'fixture', label: 'Local fixture', is_local: true },
            commands: [],
            command_categories: [],
          });
        if (url.pathname === '/api/ai/agents')
          return respond({ count: 1, agents: [{ agent_id: 'agent-a', hostname: 'agent-a' }] });
        if (url.pathname.includes('/metrics-history/'))
          return respond({
            resourceType: 'vm',
            resourceId: 'fixture:pve1:100',
            range: '24h',
            start: Date.UTC(2026, 9, 3, 6),
            end: Date.UTC(2026, 9, 4, 6),
            metrics: {},
            source: 'store',
          });
        if (url.pathname.includes('/runtime-capabilities'))
          return respond({
            capabilities: [],
            limits: [],
            max_history_days: 90,
            hosted_mode: false,
            runtime: { build: 'community' },
            blocked_capabilities: [],
          });
        if (url.pathname.includes('/license/'))
          return respond({ tier: 'pro', valid: true, features: {}, maxHistoryDays: 90 });
        if (url.pathname.endsWith('/operator-state')) return respond({});
        if (url.pathname.includes('/metadata')) return respond({});
        return respond({});
      });
      const activate = async (locator) => (width < 768 ? locator.tap() : locator.click());
      const until = async (fn) => {
        for (let n = 0; n < 100; n++) {
          if (await fn()) return;
          await page.waitForTimeout(50);
        }
        throw new Error('Condition not observed');
      };
      const text = () => page.locator('main').innerText();
      const update = (agent, id) =>
        page.evaluate(([agent, id]) => window.__guestIdentity.update(agent, id), [agent, id]);
      const shot = async (state) => {
        const file = `${name}-${state}.png`;
        await page.screenshot({ path: output + '/' + file, fullPage: true });
        result.screenshots.push({ file, state, sha256: hash(output + '/' + file) });
      };
      const settle = async (agent) => {
        const matches = pendingReads.filter((r) => r.agent === agent);
        pendingReads = pendingReads.filter((r) => r.agent !== agent);
        for (const r of matches)
          await r.route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify(record(agent)),
          });
      };
      await page.goto(
        origin + '/browser-tests/guest-discovery-identity.html' + (dark ? '?dark=1' : ''),
      );
      await until(async () => (await text()).includes('Service agent-a'));
      await until(
        () => item.requests.filter((r) => r.path === '/api/discovery/vm/agent-a/100').length === 2,
      );
      await page.evaluate(() => window.__guestIdentity.ticks(100));
      await page.waitForTimeout(200);
      assert.equal(
        item.requests.filter((r) => r.path === '/api/discovery/vm/agent-a/100').length,
        2,
        '100 same-target snapshots must not issue discovery reads',
      );
      await activate(page.getByRole('tab', { name: 'Manage', exact: true }));
      assert.equal(
        await page
          .getByRole('link', { name: 'Open suggested URL', exact: true })
          .getAttribute('href'),
        'https://agent-a.example.test/',
      );
      assert.equal(
        await page.locator('main input[type=url]').inputValue(),
        'https://operator.example.test/',
      );
      modes.set('agent-b', 'pending');
      await update('agent-b');
      await until(() => pendingReads.filter((r) => r.agent === 'agent-b').length === 2);
      assert.equal(
        await page.getByRole('link', { name: 'Open suggested URL', exact: true }).count(),
        0,
      );
      assert.equal((await text()).includes('Service agent-a'), false);
      assert.equal(
        await page.locator('main input[type=url]').inputValue(),
        'https://operator.example.test/',
      );
      await shot('replacement-pending');
      modes.delete('agent-b');
      await settle('agent-b');
      await until(() =>
        page
          .getByRole('link', { name: 'Open suggested URL', exact: true })
          .count()
          .then((n) => n === 1),
      );
      assert.equal(
        await page
          .getByRole('link', { name: 'Open suggested URL', exact: true })
          .getAttribute('href'),
        'https://agent-b.example.test/',
      );
      // A command already sent for the old target is not repeated, nor may its
      // late HTTP completion replace the successor's discovery/notes UI.
      await activate(page.getByRole('tab', { name: 'Discovery', exact: true }));
      await until(async () => (await text()).includes('Service agent-b'));
      const run = page.getByRole('button', { name: 'Run Discovery', exact: true }).first();
      await activate(run);
      await until(() => Boolean(pendingScan));
      await update('agent-c');
      await until(async () => (await text()).includes('Service agent-c'));
      await pendingScan.route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          ...record('agent-b'),
          service_name: 'Late old result',
          user_notes: 'Late old notes',
        }),
      });
      pendingScan = undefined;
      await page.waitForTimeout(100);
      assert.equal((await text()).includes('Late old'), false);
      assert.equal((await text()).includes('Notes agent-b'), false);
      assert.equal(item.requests.filter((r) => r.method === 'POST').length, 1);
      await shot('late-scan-isolated');
      // A source change cannot borrow the current source's cache while its
      // own two reads are pending; late completions are ignored as well.
      modes.set('agent-d', 'pending');
      await update('agent-d');
      await until(() => pendingReads.filter((r) => r.agent === 'agent-d').length === 2);
      await update('agent-e');
      await until(async () => (await text()).includes('Service agent-e'));
      await settle('agent-d');
      await page.waitForTimeout(100);
      assert.equal((await text()).includes('Service agent-d'), false);
      // Final denial withdraws retained suggestions; neither a same-target
      // transient cache nor another source's successful data stands in for it.
      modes.set('agent-e', 403);
      await page.evaluate(() => window.__guestIdentity.mount(false));
      await page.evaluate(() => window.__guestIdentity.mount(true));
      await until(async () => (await text()).includes('Access denied.'));
      await activate(page.getByRole('tab', { name: 'Manage', exact: true }));
      assert.equal(
        await page.getByRole('link', { name: 'Open suggested URL', exact: true }).count(),
        0,
      );
      assert.equal((await text()).includes('private diagnostic'), false);
      modes.set('agent-e', 'pending');
      const retry = page.getByRole('button', { name: 'Retry service details', exact: true });
      await retry.focus();
      await page.keyboard.press('Enter');
      await until(() => pendingReads.some((r) => r.agent === 'agent-e'));
      assert.equal(
        await page.getByRole('link', { name: 'Open suggested URL', exact: true }).count(),
        0,
      );
      await shot('denied-retry');
      modes.delete('agent-e');
      await settle('agent-e');
      await until(() =>
        page
          .getByRole('link', { name: 'Open suggested URL', exact: true })
          .count()
          .then((n) => n === 1),
      );
      assert.equal(
        await page
          .getByRole('link', { name: 'Open suggested URL', exact: true })
          .getAttribute('href'),
        'https://agent-e.example.test/',
      );
      // Same-target transient failure retains only its own data, with a fixed
      // visible qualification; a successful no-record read is genuinely empty.
      modes.set('agent-e', 503);
      await page.evaluate(() => window.__guestIdentity.mount(false));
      await page.evaluate(() => window.__guestIdentity.mount(true));
      await until(async () =>
        (await text()).includes('Showing previously loaded service details.'),
      );
      await activate(page.getByRole('tab', { name: 'Manage', exact: true }));
      assert.equal(
        await page
          .getByRole('link', { name: 'Open suggested URL', exact: true })
          .getAttribute('href'),
        'https://agent-e.example.test/',
      );
      await shot('transient-retained');
      modes.set('agent-e', 'missing');
      await activate(page.getByRole('button', { name: 'Retry service details', exact: true }));
      await until(async () => !(await text()).includes('Showing previously loaded'));
      assert.equal(
        await page.getByRole('link', { name: 'Open suggested URL', exact: true }).count(),
        0,
      );
      await activate(page.getByRole('tab', { name: 'History', exact: true }));
      await page.getByLabel('History range').selectOption('7d');
      await page.evaluate(() => window.__guestIdentity.ticks(1));
      assert.equal(await page.getByLabel('History range').inputValue(), '7d');
      await update('agent-f', 'fixture-pve1-200');
      assert.equal(
        await page
          .getByRole('tab', { name: 'Overview', exact: true })
          .getAttribute('aria-selected'),
        'true',
      );
      await activate(page.getByRole('tab', { name: 'History', exact: true }));
      assert.equal(await page.getByLabel('History range').inputValue(), '24h');
      assert.equal(
        await page.locator('main').evaluate((main) => main.scrollWidth <= main.clientWidth + 1),
        true,
      );
      assert.deepEqual(item.errors, []);
      assert.equal(
        item.requests.some((r) => r.blocked),
        false,
      );
      assert.equal(
        item.requests.filter((r) => r.method !== 'GET').length,
        1,
        'Only the explicit synthetic old-target scan is sent',
      );
      item.checks = [
        '100 same-target updates issue zero extra discovery reads and keep selection.',
        'Replacement target withdraws old service/URL; saved operator URL is separate.',
        'Late old lookup and scan completion cannot populate successor service/notes.',
        'Final 403 clears retained value; keyboard retry stays empty until current success.',
        'Same-target 503 is visibly retained, raw error withheld, successful 404 is empty.',
        'New canonical guest resets tabs/range, same-ID update preserves them.',
        'Phone touch and desktop keyboard/links, no overflow/page error/off-origin access.',
      ];
      await page.close();
      await browser.close();
      browser = undefined;
    }
    result.result = 'passed';
    result.verified_at = new Date().toISOString();
  } finally {
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  process.stderr.write(error.stack + '\n');
  process.exitCode = 1;
});
