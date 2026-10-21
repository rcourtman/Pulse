// Native power-only canonical deltas through the real merger, resource drawer,
// cards and CSS. Fixed synthetic reports; no guest/service/backup operation.
const { chromium, webkit, expect } = require('playwright/test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const root = path.resolve('frontend-modern');
const compiled = path.resolve('tmp/resource-agent-proof/static');
const output = path.resolve('tmp/resource-agent-proof/browser');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const report = {
  result: 'incomplete',
  source: null,
  playwright: require('playwright/package.json').version,
  profiles: [],
  captures: [],
  cleanup: {
    browsersCreated: 0,
    browsersClosed: 0,
    contextsCreated: 0,
    contextsClosed: 0,
    serverClosed: false,
  },
  limits:
    'Synthetic stored canonical resource deltas and loopback GETs, real merger/Infrastructure drawer/cards. Not native guest recovery, thaw, live filesystem or backup liveness, installed use or release acceptance.',
};
let server, browser, context;
(async () => {
  fs.mkdirSync(output, { recursive: true });
  try {
    assert.equal(
      report.playwright,
      JSON.parse(fs.readFileSync('tests/integration/package-lock.json')).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
    report.source = binding.source_sha;
    report.binding_sha256 = hash(fs.readFileSync(path.join(compiled, 'binding.json')));
    for (const [file, digest] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `Changed source: ${file}`);
    for (const file of [
      'src/components/Infrastructure/useResourceDetailDrawerDerivedState.ts',
      'src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx',
      'src/components/Infrastructure/resourceDetailMappers.ts',
      'src/components/shared/cards/DisksCard.tsx',
      'src/components/shared/cards/NetworkInterfacesCard.tsx',
      'src/utils/guestHardwareObservation.ts',
    ])
      assert(binding.sources[file], `Missing source ${file}`);
    const assets = new Map(
      Object.entries(binding.artifacts).map(([file, digest]) => {
        assert(!path.isAbsolute(file) && !file.split('/').includes('..'));
        const bytes = fs.readFileSync(path.join(compiled, file));
        assert.equal(hash(bytes), digest, file);
        return ['/' + file, bytes];
      }),
    );
    server = http.createServer((req, res) => {
      const file = new URL(req.url, 'http://127.0.0.1').pathname,
        bytes = assets.get(file);
      res.setHeader(
        'Content-Type',
        file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : 'text/html',
      );
      res.setHeader('Cache-Control', 'no-store');
      res.writeHead(bytes ? 200 : 404);
      res.end(bytes);
    });
    await new Promise((resolve, reject) => {
      server.once('error', reject);
      server.listen(0, '127.0.0.1', resolve);
    });
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const [name, engine, width, dark] of [
      ['desktop', chromium, 1365, false],
      ['narrow', webkit, 320, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      report.cleanup.browsersCreated++;
      context = await browser.newContext({
        viewport: { width, height: 900 },
        hasTouch: width < 500,
        isMobile: width < 500,
        locale: 'en-GB',
        timezoneId: 'UTC',
        reducedMotion: 'reduce',
      });
      report.cleanup.contextsCreated++;
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      await page.clock.install({ time: new Date('2026-10-20T12:00:00Z') });
      const record = {
        name,
        width,
        height: 900,
        dark,
        browser: browser.version(),
        checks: [],
        requests: [],
        unexpected: [],
        errors: [],
        failed: [],
      };
      report.profiles.push(record);
      page.on('pageerror', (err) => record.errors.push(err.message));
      page.on('console', (msg) => {
        if (msg.type() === 'error') record.errors.push(msg.text());
      });
      page.on('requestfailed', (req) =>
        record.failed.push({ path: new URL(req.url()).pathname, error: req.failure() }),
      );
      await page.addInitScript(
        (dark) =>
          document.addEventListener(
            'DOMContentLoaded',
            () => document.documentElement.classList.toggle('dark', dark),
            { once: true },
          ),
        dark,
      );
      await page.routeWebSocket('**/*', (socket) => socket.close());
      await page.route('**/*', (route) => {
        const req = route.request(),
          url = new URL(req.url());
        if (url.origin !== origin || req.method() !== 'GET') {
          record.unexpected.push({ path: url.pathname, method: req.method() });
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        record.requests.push({ path: url.pathname, method: req.method() });
        if (url.pathname === '/api/metrics-store/history')
          return route.fulfill({
            json: {
              resourceType: url.searchParams.get('resourceType'),
              resourceId: url.searchParams.get('resourceId'),
              range: url.searchParams.get('range'),
              source: 'store',
              metrics: {},
            },
          });
        const data = {
          '/api/security/status': {
            hasAuthentication: true,
            requiresAuth: false,
            sessionCapabilities: {},
          },
          '/api/health': { status: 'ok' },
          '/api/license/runtime-capabilities': {
            capabilities: [],
            limits: [],
            hosted_mode: false,
            blocked_capabilities: [],
            runtime: { build: 'community' },
          },
        }[url.pathname];
        if (!data) {
          record.unexpected.push({ path: url.pathname, method: req.method() });
          return route.fulfill({ status: 404, json: { error: 'Unexpected fixture read' } });
        }
        return route.fulfill({ json: data });
      });
      const act = async (method, ...args) =>
        page.evaluate(([method, args]) => window.__resourceAgent[method](...args), [method, args]);
      const check = (value) => record.checks.push(value);
      const capture = async (label) => {
        const host = page.getByTestId('resource-host-details-section');
        await host.scrollIntoViewIfNeeded();
        await page.evaluate(() => document.fonts.ready);
        const file = `${name}-${label}.png`;
        await host.screenshot({ path: path.join(output, file) });
        report.captures.push({
          file,
          sha256: hash(fs.readFileSync(path.join(output, file))),
          name,
          label,
        });
      };
      await page.goto(origin + '/browser-tests/resource-agent-observation.html', {
        waitUntil: 'networkidle',
      });
      const details = page.getByTestId('resource-technical-details'),
        summary = details.locator(':scope > summary');
      if (width < 500) await summary.tap();
      else {
        await summary.focus();
        await page.keyboard.press('Enter');
      }
      await expect(details).toHaveAttribute('open', '');
      await page.evaluate(() => {
        window.__originalDetails = document.querySelector(
          '[data-testid="resource-technical-details"]',
        );
      });
      await act('report');
      const host = page.getByTestId('resource-host-details-section');
      const disks = host.getByTestId('disks-card-total'),
        raid = host.getByTestId('raid-card-array');
      const custom = host.getByText('Custom Metrics', { exact: true }).locator('..');
      const thermals = host.getByText('Thermals', { exact: true }).locator('..');
      const network = host.getByText('Network', { exact: true }).locator('..');
      const assertCurrent = async () => {
        await expect(disks).not.toContainText('Last known');
        await expect(host.getByTestId('disks-card-mount-bar')).toHaveCount(1);
        await expect(raid).toHaveAttribute('data-raid-reading', 'current');
        await expect(raid.getByTestId('raid-card-rebuild')).toContainText('100M/sec');
        await expect(thermals.getByText('42°C', { exact: true })).toBeVisible();
        await expect(thermals.getByText('36°C', { exact: true })).toBeVisible();
        await expect(custom.getByText('Online', { exact: true })).toHaveCount(1);
        await expect(custom.getByText('Online (stale)', { exact: true })).toHaveCount(1);
        await expect(network.getByRole('status')).toHaveCount(0);
        await expect(network.getByText('192.0.2.11/24')).toHaveClass(/text-blue-700/);
      };
      const assertRetained = async (reason) => {
        await expect(disks).toContainText('Last known 90%');
        await expect(host.getByTestId('disks-card-mount-bar')).toHaveCount(0);
        await expect(raid).toHaveAttribute('data-raid-reading', 'last-known');
        await expect(raid.getByTestId('raid-card-rebuild')).toHaveText('Rebuild was at 37%');
        await expect(raid).not.toContainText('100M/sec');
        await expect(thermals.getByText('42°C (last known)', { exact: true })).toBeVisible();
        // Native execution state applies to the in-guest SMART report, not independent physical disks.
        await expect(thermals.getByText('36°C (last known)', { exact: true })).toBeVisible();
        await expect(custom.getByText('Online (last known)', { exact: true })).toHaveCount(2);
        await expect(
          custom.getByText('2h old when observed (last known)', { exact: true }),
        ).toBeVisible();
        await expect(custom.getByText('0 items (last known)', { exact: true })).toBeVisible();
        await expect(custom.getByText('Unavailable', { exact: true })).toBeVisible();
        await expect(
          custom.getByText('Online (last known)', { exact: true }).last(),
        ).toHaveAttribute('title', /Last reported status: Error.*Synthetic read timed out/);
        await expect(network.getByText('192.0.2.11/24')).toHaveClass(/text-muted/);
        const statuses = host.getByRole('status');
        await expect(statuses).toHaveCount(5);
        for (const status of await statuses.all()) await expect(status).toContainText(reason);
      };
      await assertCurrent();
      await capture('current-vm');
      check(
        'First agent report appears in the already-open drawer; current disks, rebuild, thermals, SMART, custom zero and network are intact.',
      );
      const before = await act('evidence');
      for (const power of ['paused', 'suspended', 'stopped', 'offline']) {
        await act('power', power);
        await assertRetained(`Proxmox reports this VM ${power === 'offline' ? 'stopped' : power}`);
        const after = await act('evidence');
        assert.deepEqual(after.agent, before.agent);
        assert.equal(after.status, 'online');
        assert.equal(after.lastSeen, before.lastSeen);
        if (power === 'paused') await capture('paused-vm');
        check(
          `${power}: native power-only delta qualifies every in-guest card without changing the selected agent report, resource status or time.`,
        );
      }
      await act('platform');
      await assertRetained('Proxmox reports this VM stopped');
      check('An online carrier and later Last seen do not renew in-guest evidence.');
      await act('power', 'running');
      await assertCurrent();
      check(
        'A confirmed running update restores these reports; independent failed custom sensors remain stale.',
      );
      await act('kind', 'system-container');
      await act('power', 'stopped');
      await assertRetained('Proxmox reports this container stopped');
      await capture('stopped-lxc');
      await act('power', 'offline');
      await assertRetained('Proxmox reports this container stopped');
      check(
        'Stopped/offline LXC reports are qualified with their own native state, not VM semantics.',
      );
      await act('kind', 'agent');
      await act('noAgentNetwork');
      await expect(host.getByText('192.0.2.99/24')).toBeVisible();
      const apiNetwork = host.getByText('Network', { exact: true }).locator('..');
      await expect(apiNetwork.getByRole('status')).toHaveCount(0);
      await expect(apiNetwork.getByText('192.0.2.99/24')).toHaveClass(/text-blue-700/);
      check(
        "Removing a physical host's agent list exposes its independently sourced Proxmox configuration without downgrading it.",
      );
      await act('report');
      await assertCurrent();
      await act('kind', 'agent');
      await act('stale', false);
      await assertCurrent();
      check('A physical host is not made into a stopped guest by a provider facet.');
      await act('stale', true);
      await expect(disks).toContainText('Last known 90%');
      await expect(raid).toHaveAttribute('data-raid-reading', 'last-known');
      await expect(network.getByRole('status')).toContainText('host agent stopped reporting');
      await expect(thermals.getByText('36°C', { exact: true })).toBeVisible();
      check(
        'Expired-host reports retain their existing qualification; independent disk collection stays authoritative.',
      );
      await act('stale', false);
      await act('kind', 'vm');
      await act('scope', 'vmware-vsphere');
      await assertCurrent();
      check('An explicit vSphere scope cannot borrow Proxmox power semantics.');
      await act('scope', 'proxmox-pve');
      await act('power', 'unknown');
      await assertCurrent();
      check(
        'Unknown native power does not invent a stopped state or invalidate an independently reporting agent.',
      );
      await act('power', 'running');
      await capture('recovered-vm');
      assert.equal(await details.evaluate((el) => el === window.__originalDetails), true);
      await expect(details).toHaveAttribute('open', '');
      const geometry = await host.evaluate((el) => ({
        width: el.clientWidth,
        scroll: el.scrollWidth,
        documentWidth: document.documentElement.clientWidth,
        documentScroll: document.documentElement.scrollWidth,
      }));
      assert(geometry.scroll <= geometry.width + 1);
      assert(geometry.documentScroll <= geometry.documentWidth + 1);
      record.geometry = geometry;
      if (width < 500) await summary.tap();
      else {
        await summary.focus();
        await page.keyboard.press('Enter');
      }
      await expect(details).not.toHaveAttribute('open', '');
      check(
        'Trusted keyboard/touch disclosure closes normally, same mounted details, no horizontal overflow.',
      );
      assert.deepEqual(record.errors, []);
      assert.deepEqual(record.failed, []);
      assert.deepEqual(record.unexpected, []);
      await context.close();
      context = undefined;
      report.cleanup.contextsClosed++;
      await browser.close();
      browser = undefined;
      report.cleanup.browsersClosed++;
    }
    report.result = 'passed';
  } catch (err) {
    report.error = String(err.stack || err);
    throw err;
  } finally {
    if (context) {
      await context.close();
      report.cleanup.contextsClosed++;
    }
    if (browser) {
      await browser.close();
      report.cleanup.browsersClosed++;
    }
    if (server) {
      await new Promise((resolve) => server.close(resolve));
      report.cleanup.serverClosed = true;
    }
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(JSON.stringify(report));
  }
})().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
