// Read-only synthetic power/producer updates through production consumers.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const compiled = path.join(root, 'tmp/guest-hardware-proof/static');
const output = path.join(root, 'tmp/guest-hardware-proof/browser');
const origin = 'http://127.0.0.1:5353';
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
    'Synthetic canonical deltas and scoped HTTP disk responses, real merger/useWorkloads/GuestDrawer/SMART/RAID/CSS. No live collection, guest probe, hardware health, thaw, filesystem writes, installed recovery or release acceptance.',
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
      'src/utils/guestHardwareObservation.ts',
      'src/components/Workloads/GuestDrawerOverview.tsx',
      'src/components/Workloads/GuestPhysicalDisks.tsx',
      'src/components/Storage/DiskDetail.tsx',
      'src/components/Storage/useDiskDetailModel.ts',
      'src/components/shared/cards/RaidCard.tsx',
      'src/features/storageBackups/diskPresentation.ts',
      'src/features/storageBackups/diskTemperaturePresentation.ts',
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
      const file = pathname === '/' ? '/browser-tests/guest-hardware-observation.html' : pathname;
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
      server.listen(5353, '127.0.0.1', resolve);
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
      let diskMode = 'current';
      const diskWire = () => ({
        id: 'disk-101-a',
        parentId: 'vm-resource-101',
        type: 'physical_disk',
        name: 'Guest SATA',
        displayName: 'Guest SATA',
        platformId: 'fixture',
        platformType: 'proxmox-pve',
        sourceType: diskMode === 'api' ? 'api' : 'agent',
        sources: diskMode === 'api' ? ['proxmox'] : ['agent'],
        status: 'online',
        lastSeen: Date.now(),
        health:
          diskMode === 'risk'
            ? { verdict: 'stale', reasons: [{ code: 'telemetry_stale' }] }
            : { verdict: 'ok', reasons: [] },
        metricsTarget: { resourceType: 'disk', resourceId: 'fixture-guest:sda' },
        physicalDisk: {
          devPath: '/dev/sda',
          model: 'Guest SATA',
          serial: 'FIXTURE-101-A',
          diskType: 'sata',
          sizeBytes: 1_000_000_000,
          health: 'PASSED',
          temperature: diskMode === 'recovered' ? 42 : 45,
          smart: {
            reallocatedSectors: 0,
            pendingSectors: diskMode === 'risk' ? 8 : 0,
            udmaCrcErrors: 0,
          },
          collection: {
            temperature: { state: 'available', source: 'smartctl' },
            io: { state: 'available', source: 'host_agent' },
          },
        },
      });
      await page.route('**/*', (route) => {
        const request = route.request(),
          url = new URL(request.url());
        assert.equal(url.origin, origin, 'no off-origin connection');
        assert(['GET', 'HEAD'].includes(request.method()), 'read-only presentation');
        if (url.pathname === '/favicon.ico') return route.fulfill({ status: 204, body: '' });
        if (!url.pathname.startsWith('/api/')) return route.continue();
        record.requests.push({
          path: url.pathname,
          query: url.search,
          method: request.method(),
          diskMode,
        });
        assert(
          !/diagnostics|run-command|guest-agent|qemu/.test(url.pathname),
          'no guest operation',
        );
        if (url.pathname === '/api/resources') {
          assert.equal(url.searchParams.get('type'), 'physical_disk');
          assert.equal(url.searchParams.get('parent'), 'vm-resource-101');
          const disk = diskWire();
          record.disk_responses ??= [];
          record.disk_responses.push(disk);
          return route.fulfill({
            json: {
              resources: [
                disk,
                {
                  ...disk,
                  id: 'unrelated-disk',
                  parentId: 'another-machine',
                  name: 'Unrelated disk',
                },
              ],
              meta: { totalPages: 1 },
            },
          });
        }
        if (url.pathname === '/api/metrics-store/history') {
          const end = Date.now(),
            metric = url.searchParams.get('metric');
          const body = {
            resourceType: url.searchParams.get('resourceType'),
            resourceId: url.searchParams.get('resourceId'),
            range: url.searchParams.get('range') || '24h',
            source: 'store',
            start: end - 600_000,
            end,
          };
          if (metric) {
            body.metric = metric;
            body.points = [0, 1, 2].map((i) => ({
              timestamp: end - (2 - i) * 300_000,
              value: metric === 'smart_temp' ? 43 + i : 0,
              min: metric === 'smart_temp' ? 43 + i : 0,
              max: metric === 'smart_temp' ? 43 + i : 0,
            }));
          } else body.metrics = {};
          return route.fulfill({ json: body });
        }
        if (url.pathname === '/api/alerts/active') return route.fulfill({ json: [] });
        assert(
          [
            '/api/license/runtime-capabilities',
            '/api/alerts/config',
            '/api/security/status',
            '/api/metrics-store/stats',
            '/api/patrol/config',
          ].includes(url.pathname),
          `Unexpected API: ${url.pathname}`,
        );
        return route.fulfill({
          json: url.pathname.startsWith('/api/license/')
            ? { capabilities: [], limits: [], max_history_days: 7, runtime: { build: 'community' } }
            : { enabled: true, activationState: 'active' },
        });
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
      await page.waitForFunction(() => window.__guestHardware?.evidence().projected);
      const drawer = page.getByRole('region', { name: 'Workload drawer' });
      const row = drawer.getByTestId('guest-physical-disk');
      await expect(row).toHaveCount(1);
      const summary = row.locator('summary');
      if (width < 768) await summary.tap();
      else {
        await summary.focus();
        await summary.press('Enter');
      }
      await expect(row.getByTestId('disk-detail-health')).toContainText('Healthy');
      await expect(row.getByText('Last known temperature', { exact: true })).toHaveCount(0);
      await row.evaluate((el) => (el.dataset.proofOwner = 'same-open-disk'));
      const arrays = drawer.getByTestId('raid-card-array');
      await expect(arrays.first()).toHaveAttribute('data-raid-reading', 'current');
      await expect(arrays.first()).toContainText('clean');
      await expect(arrays.last()).toContainText('100M/sec');
      record.cases.push(
        'Current in-guest disk and both RAID states render normally from the scoped collected inventory.',
      );
      const apply = (method, ...args) =>
        page.evaluate(({ method, args }) => window.__guestHardware[method](...args), {
          method,
          args,
        });
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
      await capture('current-hardware', summary);
      for (const power of ['paused', 'suspended', 'stopped', 'offline']) {
        const before = record.requests.filter((r) => r.path === '/api/resources').length;
        await apply('power', power);
        const expected = power === 'offline' ? 'stopped' : power;
        await expect(summary.getByText('Last known', { exact: true })).toBeVisible();
        await expect(row.getByTestId('disk-detail-health')).toContainText(
          `Proxmox reports this VM ${expected}.`,
        );
        await expect(row.getByText('Last known temperature', { exact: true })).toBeVisible();
        await expect(row.locator('[data-temperature-reading]')).toHaveAttribute(
          'data-temperature-reading',
          'last-known',
        );
        await expect(row.getByText('Live I/O (30m)', { exact: true })).toHaveCount(0);
        await expect(row).toHaveAttribute('open', '');
        await expect(row).toHaveAttribute('data-proof-owner', 'same-open-disk');
        await expect(row.getByRole('combobox', { name: 'Disk history range' })).toHaveValue('24h');
        await expect(arrays.first()).toHaveAttribute('data-raid-reading', 'last-known');
        await expect(arrays.first()).toContainText('clean (last known)');
        await expect(arrays.last()).toContainText('Rebuild was at 37%');
        await expect(arrays.last()).toContainText('/dev/sdb · faulty');
        await expect(arrays.last()).not.toContainText('100M/sec');
        const evidence = await page.evaluate(() => window.__guestHardware.evidence());
        assert.equal(evidence.canonical.status, 'online');
        assert.equal(evidence.canonical.agent.stale, false);
        // The Workloads adapter omits a false optional flag, it does not renew the lease.
        assert.equal(Boolean(evidence.projected.agentStale), false);
        assert.equal(evidence.canonical.agent.lastReportAt, '2026-10-20T12:00:00Z');
        assert.equal(
          record.requests.filter((r) => r.path === '/api/resources').length,
          before,
          'no reread/probe required by power qualification',
        );
        assert.equal(
          await row.locator('.text-green-600').count(),
          0,
          'no current SMART card tones',
        );
        assert.equal(
          await arrays.locator('[class*=emerald], [class*=amber], [class*=text-red]').count(),
          0,
          'no live RAID tones',
        );
        record.cases.push(
          `${power}: online resource/unexpired agent lease does not renew disk or RAID evidence; same-open owner, stored History and adverse values remain.`,
        );
        if (power === 'paused') {
          await capture('paused-smart', row.getByTestId('disk-detail-health'));
          await capture('paused-raid', arrays.last());
        }
      }
      // The disk source remains independently stale even after native running resumes.
      diskMode = 'risk';
      await apply('reread');
      await expect(summary.getByText('Last known risk', { exact: true })).toBeVisible();
      await apply('power', 'running');
      await expect(row.getByTestId('disk-detail-health')).toContainText(
        'Disk source stopped reporting.',
      );
      await expect(row.getByTestId('disk-detail-health')).toContainText(
        'Last reported: Needs Attention',
      );
      await expect(row.getByText('Last known temperature', { exact: true })).toBeVisible();
      await expect(row).toHaveAttribute('data-proof-owner', 'same-open-disk');
      record.cases.push(
        'Native running does not clear a separately stale disk or its reported risk; the source remains unchanged.',
      );
      await capture('retained-risk', row.getByTestId('disk-detail-health'));
      // A current replacement source supplies a changed measurement.
      diskMode = 'recovered';
      await apply('reread');
      await expect(summary.getByText('Healthy', { exact: true })).toBeVisible();
      await expect(row.locator('[data-temperature-reading]')).toHaveAttribute(
        'data-temperature-reading',
        'current',
      );
      await expect(row.locator('[data-temperature-reading]')).toContainText('42°C');
      await expect(row).toHaveAttribute('data-proof-owner', 'same-open-disk');
      record.cases.push(
        'A replacement successful current disk response restores its new 42°C reading without closing the owner. This is not thaw or liveness proof.',
      );
      await apply('stale', true);
      await expect(summary.getByText('Last known', { exact: true })).toBeVisible();
      await expect(row.getByTestId('disk-detail-health')).toContainText(
        'The Pulse Agent in this guest stopped reporting.',
      );
      record.cases.push(
        'Guest-agent lease expiry qualifies a disk whose separately read resource still reports online/available.',
      );
      diskMode = 'api';
      await apply('reread');
      await expect(summary.getByText('Healthy', { exact: true })).toBeVisible();
      await expect(arrays.first()).toHaveAttribute('data-raid-reading', 'last-known');
      record.cases.push(
        'An explicitly API-only independent disk is not downgraded by an unrelated agent expiry. The agent RAID report remains retained.',
      );
      await apply('stale', false);
      await apply('kind', 'system-container');
      await apply('power', 'paused');
      await expect(arrays.first()).toHaveAttribute('data-raid-reading', 'current');
      record.cases.push('A container does not acquire the native Proxmox VM power interpretation.');
      await apply('kind', 'vm');
      await apply('power', 'unknown');
      await expect(arrays.first()).toHaveAttribute('data-raid-reading', 'current');
      record.cases.push(
        'Unknown native power does not invent a shutdown diagnosis against an independently reporting agent.',
      );
      await expect(drawer.getByText('Failed to load history data', { exact: true })).toHaveCount(0);
      assert(
        record.requests.some(
          (r) =>
            r.path === '/api/metrics-store/history' &&
            r.query.includes('resourceId=fixture-guest%3Asda'),
        ),
        'stored disk History was consumed',
      );
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
