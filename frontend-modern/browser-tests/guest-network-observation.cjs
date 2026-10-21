// Read-only synthetic power/producer updates through production consumers.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const compiled = path.join(root, 'tmp/guest-network-proof/static');
const output = path.join(root, 'tmp/guest-network-proof/browser');
const origin = 'http://127.0.0.1:5352';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const report = {
  result: 'incomplete',
  playwright: require('playwright/package.json').version,
  profiles: [],
  captures: [],
  cleanup: {
    browsers_created: 0,
    browsers_closed: 0,
    contexts_created: 0,
    contexts_closed: 0,
    server_listening: false,
    server_closed: false,
  },
  limits:
    'Synthetic stored canonical deltas, real merger/useWorkloads/GuestRow/GuestDrawer/CSS. No live collection, guest probe, network connection, thaw, filesystem writes, installed recovery or release acceptance.',
};
let server, browser, context;
const save = () =>
  fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
(async () => {
  fs.mkdirSync(output, { recursive: true });
  try {
    assert.equal(
      report.playwright,
      JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
    report.binding = binding;
    assert.equal(hash(fs.readFileSync(path.join(frontend, 'package-lock.json'))), binding.graph);
    for (const [file, digest] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(frontend, file))), digest, file);
    for (const required of [
      'src/utils/guestNetworkObservation.ts',
      'src/components/Workloads/GuestRow.tsx',
      'src/components/Workloads/GuestRowCells.tsx',
      'src/components/Workloads/GuestDrawerOverview.tsx',
      'src/components/Workloads/WorkloadReadingStatus.tsx',
    ])
      assert(binding.sources[required], `missing imported source: ${required}`);
    const assets = new Map();
    for (const [file, digest] of Object.entries(binding.artifacts)) {
      assert(!file.split('/').includes('..') && !path.isAbsolute(file));
      const bytes = fs.readFileSync(path.join(compiled, file));
      assert.equal(hash(bytes), digest, file);
      assets.set('/' + file, bytes);
    }
    server = http.createServer((req, res) => {
      if (!['GET', 'HEAD'].includes(req.method)) {
        res.writeHead(405);
        return res.end();
      }
      const pathname = new URL(req.url, origin).pathname;
      const file = pathname === '/' ? '/browser-tests/guest-network-observation.html' : pathname;
      const bytes = assets.get(file);
      if (!bytes) {
        res.writeHead(404);
        return res.end();
      }
      res.setHeader(
        'Content-Type',
        file.endsWith('.html')
          ? 'text/html'
          : file.endsWith('.js')
            ? 'text/javascript'
            : file.endsWith('.css')
              ? 'text/css'
              : 'application/octet-stream',
      );
      res.setHeader('Cache-Control', 'no-store');
      res.end(req.method === 'HEAD' ? undefined : bytes);
    });
    await new Promise((resolve, reject) => {
      server.once('error', reject);
      server.listen(5352, '127.0.0.1', resolve);
    });
    report.cleanup.server_listening = true;
    for (const [name, engine, width, dark] of [
      ['desktop', chromium, 1365, false],
      ['narrow', webkit, 320, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      report.cleanup.browsers_created++;
      context = await browser.newContext({
        viewport: { width, height: 900 },
        isMobile: width < 768,
        hasTouch: width < 768,
        locale: 'en-GB',
        timezoneId: 'UTC',
        reducedMotion: 'reduce',
      });
      report.cleanup.contexts_created++;
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      const record = {
        name,
        width,
        height: 900,
        dark,
        browser: browser.version(),
        cases: [],
        requests: [],
        errors: [],
        failed: [],
      };
      report.profiles.push(record);
      page.on('pageerror', (error) => record.errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') record.errors.push(message.text());
      });
      page.on('requestfailed', (request) =>
        record.failed.push({ path: new URL(request.url()).pathname, failure: request.failure() }),
      );
      await page.routeWebSocket('**/*', (socket) => socket.close());
      await page.route('**/*', (route) => {
        const request = route.request(),
          url = new URL(request.url());
        assert.equal(url.origin, origin, 'no off-origin connection');
        assert(['GET', 'HEAD'].includes(request.method()), 'read-only presentation');
        if (url.pathname === '/favicon.ico') return route.fulfill({ status: 204, body: '' });
        if (!url.pathname.startsWith('/api/')) return route.continue();
        record.requests.push({ path: url.pathname, method: request.method() });
        assert(
          !/diagnostics|run-command|guest-agent|qemu/.test(url.pathname),
          'no guest operation',
        );
        if (url.pathname === '/api/alerts/active') return route.fulfill({ json: [] });
        const body =
          url.pathname === '/api/metrics-store/history'
            ? {
                resourceType: url.searchParams.get('resourceType'),
                resourceId: url.searchParams.get('resourceId'),
                range: url.searchParams.get('range') || '24h',
                source: 'store',
                start: Date.UTC(2026, 9, 20, 11),
                end: Date.UTC(2026, 9, 20, 12),
                metrics: {},
              }
            : { success: true, data: [], alerts: [], anomalies: [], config: null };
        return route.fulfill({ json: body });
      });
      await page.addInitScript(
        (dark) =>
          document.addEventListener(
            'DOMContentLoaded',
            () => document.documentElement.classList.toggle('dark', dark),
            { once: true },
          ),
        dark,
      );
      await page.goto(origin, { waitUntil: 'networkidle' });
      await page.waitForFunction(() => window.__guestNetwork?.evidence().projected);
      const row = page.getByRole('region', { name: 'Workload row' });
      const drawer = page.getByRole('region', { name: 'Workload drawer' });
      const details = drawer.locator('[data-testid="guest-technical-details"]');
      await details.locator('summary').click();
      await row.locator('tr').evaluate((el) => (el.dataset.observationIdentity = 'same-row'));
      await details.evaluate((el) => (el.dataset.observationIdentity = 'same-details'));
      const apply = (method, ...args) =>
        page.evaluate(({ method, args }) => window.__guestNetwork[method](...args), {
          method,
          args,
        });
      const activate = (target) =>
        width < 768 ? target.tap() : target.focus().then(() => target.press('Enter'));
      const capture = async (state, target) => {
        await target.scrollIntoViewIfNeeded();
        await page.evaluate(async () => {
          await document.fonts.ready;
          await new Promise((resolve) =>
            requestAnimationFrame(() => requestAnimationFrame(resolve)),
          );
        });
        assert.equal(
          await page.evaluate(() => document.documentElement.scrollWidth),
          width,
          'no horizontal overflow',
        );
        const file = `${name}-${state}.png`;
        await page.screenshot({ path: path.join(output, file), animations: 'disabled' });
        report.captures.push({ file, sha256: hash(fs.readFileSync(path.join(output, file))) });
        save();
      };
      const cue = row.getByRole('button', {
        name: 'Network reading for network-guest: Last known',
        exact: true,
      });
      await expect(cue).toHaveCount(0);
      record.cases.push('Initial running platform data has no last-known overlay.');
      await expect(drawer.getByText('No stored history in this range').first()).toBeVisible();
      await expect(drawer.getByText('Failed to load history data', { exact: true })).toHaveCount(0);
      for (const fromAgent of [false, true]) {
        await apply('producer', fromAgent);
        for (const power of ['paused', 'suspended', 'stopped', 'offline']) {
          await apply('power', power);
          await expect(cue).toBeVisible();
          const expectedPower = power === 'offline' ? 'stopped' : power;
          await expect(drawer.getByText('Last known IP 192.0.2.80', { exact: true })).toBeVisible();
          await expect(
            drawer.getByText(new RegExp(`Proxmox reports this VM ${expectedPower}\\.`)),
          ).toBeVisible();
          const evidence = await page.evaluate(() => window.__guestNetwork.evidence());
          assert.equal(evidence.canonical.status, 'online');
          assert.equal(evidence.projected.networkInterfacesFromAgent, fromAgent);
          assert.equal(evidence.projected.networkInterfaces[0].rxBytes, 1024);
          assert.equal(evidence.canonical.lastSeen, Date.parse('2026-10-20T12:00:00Z'));
          assert.equal(
            await row.locator('tr').getAttribute('data-observation-identity'),
            'same-row',
          );
          assert.equal(await details.getAttribute('data-observation-identity'), 'same-details');
          record.cases.push(
            `${fromAgent ? 'Pulse Agent' : 'Proxmox'} interfaces remain last known during ${power}; identity, counters, lease and online carrier remain unchanged.`,
          );
        }
      }
      await apply('power', 'paused');
      await activate(cue);
      const dialog = page.getByRole('dialog', {
        name: 'Network reading for network-guest',
        exact: true,
      });
      await expect(dialog).toContainText('Proxmox reports this VM paused');
      assert.equal((await page.evaluate(() => window.__guestNetwork.evidence())).rowActions, 0);
      await capture('paused-disclosure', dialog);
      await apply('power', 'suspended');
      await expect(dialog).toContainText('Proxmox reports this VM suspended');
      if (width < 768)
        await activate(dialog.getByRole('button', { name: 'Close reading details' }));
      else await page.keyboard.press('Escape');
      await expect(dialog).toHaveCount(0);
      await expect(cue).toBeFocused();
      await activate(cue);
      await apply('power', 'running');
      await expect(dialog).toHaveCount(0);
      await expect(cue).toHaveCount(0);
      await expect(row.locator('[data-row-action]')).toBeFocused();
      record.cases.push(
        'Keyboard/touch disclosure does not activate the row, updates in place, returns focus on dismissal and withdraws safely on a running snapshot.',
      );
      await apply('producer', true, true);
      await expect(cue).toBeVisible();
      await expect(
        drawer.getByText(/Using last known interface details\. The Pulse Agent/),
      ).toBeVisible();
      await apply('producer', false, true);
      await expect(cue).toHaveCount(0);
      await expect(drawer.getByText(/Using last known interface details/)).toHaveCount(0);
      record.cases.push(
        'A silent selected agent is last known; switching to independent platform interfaces does not borrow its silence.',
      );
      await apply('kind', 'system-container');
      await apply('power', 'stopped');
      await expect(cue).toHaveCount(0);
      record.cases.push(
        'Host/configuration-derived LXC interfaces do not acquire VM power semantics.',
      );
      await apply('producer', true, false);
      for (const power of ['stopped', 'offline']) {
        await apply('power', power);
        await expect(cue).toBeVisible();
        await expect(drawer.getByText(/Proxmox reports this container stopped\./)).toBeVisible();
        await expect(drawer.getByText('Last known IP 192.0.2.80', { exact: true })).toBeVisible();
        const evidence = await page.evaluate(() => window.__guestNetwork.evidence());
        assert.equal(evidence.canonical.status, 'online');
        assert.equal(evidence.projected.networkInterfacesFromAgent, true);
        assert.equal(evidence.projected.networkInterfaces[0].rxBytes, 1024);
        assert.equal(evidence.canonical.agent.stale, false);
        assert.equal(evidence.canonical.agent.lastReportAt, '2026-10-20T12:00:00Z');
        assert.equal(await row.locator('tr').getAttribute('data-observation-identity'), 'same-row');
        assert.equal(await details.getAttribute('data-observation-identity'), 'same-details');
        record.cases.push(
          `LXC ${power}: selected agent interfaces are retained, not current; lease, carrier, addresses, counters and owner identity unchanged.`,
        );
      }
      await activate(cue);
      await expect(dialog).toContainText('Proxmox reports this container stopped.');
      await expect(dialog).toContainText(
        'Retained addresses and counters do not establish a current connection.',
      );
      assert.equal((await page.evaluate(() => window.__guestNetwork.evidence())).rowActions, 0);
      await capture('stopped-lxc-disclosure', dialog);
      if (width < 768)
        await activate(dialog.getByRole('button', { name: 'Close reading details' }));
      else await page.keyboard.press('Escape');
      await expect(cue).toBeFocused();
      await capture(
        'stopped-lxc-details',
        drawer.getByText(/Proxmox reports this container stopped\./),
      );
      await apply('producer', false, false);
      await expect(cue).toHaveCount(0);
      await expect(drawer.getByText(/Using last known interface details/)).toHaveCount(0);
      record.cases.push(
        'Returning to host-derived LXC configuration withdraws the cue without changing power or fabricating a live connection.',
      );
      await apply('kind', 'vm');
      await apply('producer', false, false);
      await apply('power', 'stopped');
      await capture('stopped-details', drawer.getByText(/Proxmox reports this VM stopped\./));
      await apply('freshAddresses');
      await expect(cue).toHaveCount(0);
      await expect(drawer.locator('[data-testid="guest-drawer-facts"]')).toContainText(
        '192.0.2.81',
      );
      await expect(drawer.getByText(/Using last known interface details/)).toHaveCount(0);
      assert.equal(
        (await page.evaluate(() => window.__guestNetwork.evidence())).projected.networkInterfaces[0]
          .rxBytes,
        0,
      );
      await capture('new-address', drawer.locator('[data-testid="guest-drawer-facts"]'));
      record.cases.push(
        'A replacement running source preserves the new address and zero counters without inventing a connection or a probe.',
      );
      await expect(drawer.getByText('No stored history in this range').first()).toBeVisible();
      await expect(drawer.getByText('Failed to load history data', { exact: true })).toHaveCount(0);
      assert.deepEqual(record.errors, []);
      assert.deepEqual(record.failed, []);
      await context.close();
      context = undefined;
      report.cleanup.contexts_closed++;
      await browser.close();
      browser = undefined;
      report.cleanup.browsers_closed++;
      save();
    }
    report.result = 'passed';
  } catch (error) {
    report.error = { name: error.name, message: error.message, stack: error.stack };
    process.exitCode = 1;
  } finally {
    if (context) {
      await context.close();
      report.cleanup.contexts_closed++;
    }
    if (browser) {
      await browser.close();
      report.cleanup.browsers_closed++;
    }
    if (server) {
      await new Promise((resolve) => {
        server.close(resolve);
        server.closeAllConnections();
      });
      report.cleanup.server_closed = true;
    }
    save();
    console.log(
      JSON.stringify({
        result: report.result,
        profiles: report.profiles.map((p) => ({ name: p.name, cases: p.cases.length })),
        captures: report.captures.length,
        cleanup: report.cleanup,
        error: report.error,
      }),
    );
  }
})();
