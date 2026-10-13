// Synthetic Resources snapshots consumed through the production HTTP client,
// tables, disclosure controls, normalization, host drawers and shared CSS.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const compiled = path.join(root, 'tmp/host-disk-usage-static/final');
const output = path.join(root, 'tmp/host-disk-usage/browser');
const origin = 'http://127.0.0.1:5299';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const GB = 1024 ** 3;
const result = {
  base: '33267809f325631a0863aced8cb56b81c2bc1070',
  scope:
    'Canonical resource HTTP client -> ProxmoxNodesTable/DockerHostsTable -> production host drawers with synthetic disk observations. No provider, disk, guest, backup or recovery operation.',
  playwright: require('playwright/package.json').version,
  buildBinding: binding,
  harnessSha256: hash(fs.readFileSync(__filename)),
  checks: [],
  requests: [],
  captures: [],
  errors: [],
  cleanup: {},
  served: {},
};
const disks = (known, allUnknown = false) =>
  [
    {
      mountpoint: '/var/lib/containers/storage/volumes/monitoring-and-backup-history',
      total: 100 * GB,
      usage: known ? 70 : -1,
      ...(known ? { used: 70 * GB } : {}),
    },
    { mountpoint: '/capacity-unavailable', used: 30 * GB },
    { mountpoint: '/empty', total: 100 * GB, free: 100 * GB },
    { mountpoint: '/measured-critical', total: 100 * GB, used: 95 * GB, usage: 95 },
    { mountpoint: '/boot/efi', total: 100 * GB, used: 20 * GB },
  ].map((disk) => (allUnknown ? { ...disk, used: undefined, usage: -1 } : disk));
const resources = (known, allUnknown) => [
  {
    id: 'fixture-node',
    type: 'agent',
    name: 'pve-fixture',
    displayName: 'pve-fixture',
    platformType: 'proxmox-pve',
    platformId: 'fixture-node',
    status: 'online',
    sources: ['proxmox', 'agent'],
    platformScopes: ['proxmox-pve', 'agent'],
    lastSeen: '2026-10-13T02:00:00Z',
    metrics: {
      cpu: { percent: 10 },
      memory: { percent: 25, used: GB, total: 4 * GB },
      disk: { percent: 25, used: 25 * GB, total: 100 * GB },
    },
    proxmox: {
      nodeName: 'pve-fixture',
      instance: 'lab',
      pveVersion: '9.2.21',
      connectionHealth: 'healthy',
    },
    agent: { agentId: 'node-agent', hostname: 'pve-fixture', disks: disks(known, allUnknown) },
  },
  {
    id: 'fixture-docker',
    type: 'docker-host',
    name: 'docker-fixture',
    displayName: 'docker-fixture',
    platformType: 'docker',
    platformId: 'fixture-docker',
    status: 'online',
    sources: ['docker', 'agent'],
    platformScopes: ['docker', 'agent'],
    lastSeen: '2026-10-13T02:00:00Z',
    docker: {
      hostSourceId: 'fixture-docker',
      runtime: 'docker',
      runtimeVersion: '28.0.0',
      os: 'Debian 13',
    },
    agent: { agentId: 'docker-agent', hostname: 'docker-fixture', disks: disks(known, allUnknown) },
  },
];
let server, browser;
(async () => {
  fs.mkdirSync(output, { recursive: true });
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  assert.equal(binding.graph, hash(fs.readFileSync(path.join(frontend, 'package-lock.json'))));
  for (const [file, digest] of Object.entries(binding.sources))
    assert.equal(
      hash(fs.readFileSync(path.join(frontend, file))),
      digest,
      `changed source ${file}`,
    );
  const assets = new Map();
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, `changed asset ${file}`);
    assets.set('/' + file, bytes);
  }
  server = http.createServer((request, response) => {
    const file = new URL(request.url, origin).pathname;
    const bytes = assets.get(file);
    if (!bytes || !['GET', 'HEAD'].includes(request.method)) {
      response.writeHead(404);
      response.end();
      return;
    }
    result.served[file] = hash(bytes);
    response.writeHead(200, {
      'Content-Type':
        { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css' }[
          path.extname(file)
        ] || 'application/octet-stream',
    });
    response.end(request.method === 'HEAD' ? undefined : bytes);
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(5299, '127.0.0.1', resolve);
  });
  for (const spec of [
    { name: 'desktop', engine: chromium, width: 1280, height: 900, colorScheme: 'light' },
    { name: 'phone', engine: webkit, width: 375, height: 812, colorScheme: 'dark' },
  ]) {
    browser = await spec.engine.launch(
      spec.engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    for (const kind of ['node', 'docker']) {
      const page = await browser.newPage({
        viewport: { width: spec.width, height: spec.height },
        colorScheme: spec.colorScheme,
        hasTouch: spec.name === 'phone',
        isMobile: spec.name === 'phone',
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      page.setDefaultTimeout(10000);
      await page.clock.setFixedTime(new Date('2026-10-13T02:00:00Z'));
      if (spec.name === 'phone')
        await page.addInitScript(() =>
          document.addEventListener('DOMContentLoaded', () =>
            document.documentElement.classList.add('dark'),
          ),
        );
      let known = false;
      let allUnknown = false;
      const requests = [];
      page.on('pageerror', (error) =>
        result.errors.push({ view: spec.name, kind, message: error.message }),
      );
      page.on('console', (event) => {
        if (event.type() === 'error')
          result.errors.push({ view: spec.name, kind, message: event.text() });
      });
      // The real global client may connect even for a REST-only resource view.
      // A synthetic idle socket supplies no resource frames or monitoring work.
      await page.routeWebSocket('**/ws?*', (socket) => {
        result.syntheticIdleSockets = (result.syntheticIdleSockets || 0) + 1;
        socket.onMessage(() => {});
      });
      await page.route('**/*', async (route) => {
        const request = route.request(),
          url = new URL(request.url());
        assert.equal(url.origin, origin, 'off-origin request');
        if (!url.pathname.startsWith('/api/')) return route.continue();
        assert.equal(request.method(), 'GET', 'unexpected mutation');
        let body;
        if (url.pathname === '/api/resources')
          body = { data: resources(known, allUnknown), meta: { totalPages: 1 } };
        else if (url.pathname === '/api/alerts/config')
          body = { enabled: false, hostDefaults: {}, guestDefaults: {} };
        else if (url.pathname === '/api/alerts/activation') body = { enabled: false };
        else if (url.pathname === '/api/alerts/active') body = [];
        else if (url.pathname === '/api/metrics-store/history')
          body = {
            resourceType: url.searchParams.get('resourceType') || 'agent',
            resourceId: url.searchParams.get('resourceId') || 'fixture',
            range: '1h',
            start: 0,
            end: Date.now(),
            metrics: {},
            source: 'store',
          };
        else if (url.pathname === '/api/license/runtime-capabilities')
          body = { plan: 'free', capabilities: {}, features: [] };
        else if (url.pathname === '/api/ai/intelligence/anomalies')
          body = { anomalies: [], count: 0 };
        else if (url.pathname === '/api/ai/intelligence/correlations')
          body = { correlations: [], count: 0 };
        else {
          result.errors.push({
            view: spec.name,
            kind,
            message: `Unmatched API read: ${url.pathname}`,
          });
          await route.abort();
          return;
        }
        requests.push(url.pathname);
        result.requests.push({
          view: spec.name,
          kind,
          path: url.pathname,
          method: request.method(),
          bodySha256: hash(JSON.stringify(body)),
          ...(url.pathname === '/api/resources' ? { body } : {}),
        });
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(body),
        });
      });
      await page.goto(`${origin}/browser-tests/host-disk-usage.html?kind=${kind}`);
      const label = kind === 'node' ? 'pve-fixture' : 'docker-fixture';
      const drawer = page.getByTestId(kind === 'node' ? 'node-drawer' : 'docker-host-drawer');
      const open = page.getByRole('button', { name: `Expand details for ${label}` });
      if (spec.name === 'desktop') {
        let reached = false;
        for (let step = 0; step < 40; step++) {
          await page.keyboard.press('Tab');
          if (await open.evaluate((element) => element === document.activeElement)) {
            reached = true;
            break;
          }
        }
        assert(reached, 'real keyboard traversal reaches disclosure');
        await page.keyboard.press('Enter');
      } else {
        if (kind === 'node') await open.tap();
        else await page.getByText(label, { exact: true }).first().tap();
      }
      await drawer.waitFor();
      const row = (mountpoint) =>
        drawer.getByText(mountpoint, { exact: true }).locator('xpath=ancestor::tr[1]');
      const unknown = row('/var/lib/containers/storage/volumes/monitoring-and-backup-history');
      await unknown.getByText('Usage unavailable · 100 GB total', { exact: true }).waitFor();
      assert.match(await unknown.innerText(), /Usage unavailable/);
      assert(!/%|0 B/.test(await unknown.innerText()), 'unknown is not zero/healthy');
      assert(
        await unknown
          .locator('td')
          .last()
          .evaluate((element) => element.classList.contains('text-muted')),
        'unknown has neutral tone',
      );
      assert.equal(
        (await row('/capacity-unavailable').innerText()).includes('Usage unavailable'),
        true,
      );
      assert.match(await row('/empty').innerText(), /0% · 0 B \/ 100 GB/);
      assert.match(await row('/measured-critical').innerText(), /95% · 95.0 GB \/ 100 GB/);
      assert.equal(await drawer.getByText('/boot/efi', { exact: true }).count(), 0);
      assert(
        await unknown
          .locator('td')
          .last()
          .evaluate(
            (element) =>
              element.textContent.includes('Usage unavailable') &&
              !element.querySelector('.truncate'),
          ),
        'message wraps rather than hiding in a tooltip',
      );
      const fit = await page.evaluate(() => ({
        width: innerWidth,
        scrollWidth: document.documentElement.scrollWidth,
      }));
      assert(fit.scrollWidth <= fit.width + 1, 'no page horizontal overflow');
      result.checks.push({
        view: spec.name,
        kind,
        name: 'real disclosure and honest unknown/capacity/zero/measured/filter evidence',
        fit,
      });
      const capture = async (state) => {
        await drawer.scrollIntoViewIfNeeded();
        const file = `${spec.name}-${kind}-${state}.png`;
        await page.screenshot({ path: path.join(output, file) });
        result.captures.push({
          file,
          sha256: hash(fs.readFileSync(path.join(output, file))),
          view: spec.name,
          kind,
          state,
        });
      };
      const summary = page
        .getByText(label, { exact: true })
        .first()
        .locator('xpath=ancestor::tr[1]');
      if (kind === 'node' && spec.name === 'phone')
        assert.match(
          await summary.innerText(),
          /48%/,
          'combined percent excludes bytes with missing capacity',
        );
      result.checks.push({
        view: spec.name,
        kind,
        name: 'summary excludes unmeasured byte rows',
        summary: await summary.innerText(),
      });
      await capture('unavailable');
      known = true;
      await page.evaluate(() => window.__hostDiskUsage.refetch());
      await unknown.getByText('70% · 70.0 GB / 100 GB', { exact: true }).waitFor();
      assert.match(await row('/capacity-unavailable').innerText(), /Usage unavailable/);
      assert.match(await row('/empty').innerText(), /0% · 0 B/);
      result.checks.push({
        view: spec.name,
        kind,
        name: 'accepted refresh replaces unknown with actual usage; peers unchanged',
      });
      if (kind === 'node' && spec.name === 'phone')
        await summary
          .locator('[data-stacked-disk-trigger]')
          .getByText('55%', { exact: true })
          .waitFor();
      await capture('measured');
      known = false;
      await page.evaluate(() => window.__hostDiskUsage.refetch());
      await unknown.getByText('Usage unavailable · 100 GB total', { exact: true }).waitFor();
      result.checks.push({
        view: spec.name,
        kind,
        name: 'later unknown read removes old utilization from same open drawer',
      });
      allUnknown = true;
      await page.evaluate(() => window.__hostDiskUsage.refetch());
      await row('/measured-critical')
        .getByText('Usage unavailable · 100 GB total', { exact: true })
        .waitFor();
      await summary
        .locator('[data-stacked-disk-trigger]')
        .getByText(kind === 'node' ? '25%' : 'N/A', { exact: true })
        .waitFor();
      result.checks.push({
        view: spec.name,
        kind,
        name: 'all unknown volumes cannot manufacture healthy zero; independent known node aggregate survives',
        summary: await summary.innerText(),
      });
      await capture('all-unknown');
      const readsBeforeClose = requests.filter((value) => value === '/api/resources').length;
      const close = drawer.getByRole('button', { name: `Collapse ${label} details` });
      if (spec.name === 'phone') await close.tap();
      else await close.click();
      await drawer.waitFor({ state: 'detached' });
      assert.equal(
        requests.filter((value) => value === '/api/resources').length,
        readsBeforeClose,
        'closing does not initiate resource read',
      );
      result.checks.push({
        view: spec.name,
        kind,
        name: 'ordinary collapse releases drawer without additional resource read',
      });
      await page.close();
    }
    result.browserVersions = { ...result.browserVersions, [spec.name]: browser.version() };
    await browser.close();
    browser = undefined;
  }
  assert.equal(result.errors.length, 0, 'no browser errors');
  result.result = 'passed';
})()
  .catch((error) => {
    result.result = 'failed';
    result.failure = error.stack;
    process.exitCode = 1;
  })
  .finally(async () => {
    if (browser) await browser.close();
    result.cleanup.browserClosed = true;
    if (server) await new Promise((resolve) => server.close(resolve));
    result.cleanup.serverClosed = !server || !server.listening;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        result: result.result,
        failure: result.failure,
        checks: result.checks.length,
        captures: result.captures,
        errors: result.errors,
        cleanup: result.cleanup,
      }),
    );
  });
