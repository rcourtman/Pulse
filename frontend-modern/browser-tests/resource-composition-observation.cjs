// Final REST hydration/readiness + qualified-memory composition through real
// consuming controls. Synthetic loopback GETs only; no guest or update action.
const { chromium, webkit, expect } = require('playwright/test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const frontend = path.resolve('frontend-modern');
const compiled = path.resolve('tmp/resource-agent-proof/static');
const output = path.resolve('tmp/resource-composition-proof/browser');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const report = {
  result: 'incomplete',
  source: null,
  playwright: require('playwright/package.json').version,
  harness_sha256: hash(fs.readFileSync(__filename)),
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
    'Stored synthetic websocket seed and REST DTOs through useUnifiedResources, Docker table/full Infrastructure drawer and real GuestPhysicalDisks. No live websocket, agent, guest probe, update POST, installed recovery, disk/backup health, thaw, filesystem liveness or release acceptance.',
};
let server, browser, context;
const save = () =>
  fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
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
      assert.equal(hash(fs.readFileSync(path.join(frontend, file))), digest, file);
    for (const file of [
      'src/hooks/useUnifiedResources.ts',
      'src/components/Infrastructure/ResourceDetailSummary.tsx',
      'src/components/Infrastructure/ResourceDetailDrawer.tsx',
      'src/components/Workloads/GuestPhysicalDisks.tsx',
      'browser-tests/resource-composition-observation.tsx',
      'browser-tests/resource-composition-observation.cjs',
    ])
      assert(binding.sources[file], `unbound source ${file}`);
    const assets = new Map(
      Object.entries(binding.artifacts).map(([file, digest]) => {
        assert(!path.isAbsolute(file) && !file.split('/').includes('..'));
        const bytes = fs.readFileSync(path.join(compiled, file));
        assert.equal(hash(bytes), digest, file);
        return ['/' + file, bytes];
      }),
    );
    server = http.createServer((req, res) => {
      const file = new URL(req.url, 'http://127.0.0.1').pathname;
      const bytes = assets.get(file);
      if (req.method !== 'GET' || !bytes) {
        res.writeHead(404);
        return res.end();
      }
      res.setHeader(
        'Content-Type',
        file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : 'text/html',
      );
      res.setHeader('Cache-Control', 'no-store');
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
      await page.clock.pauseAt(new Date('2026-10-20T12:00:10Z'));
      const record = {
        name,
        width,
        dark,
        browser: browser.version(),
        checks: [],
        requests: [],
        responses: [],
        pageErrors: [],
        consoleErrors: [],
        failed: [],
        unexpected: [],
      };
      report.profiles.push(record);
      page.on('pageerror', (err) => record.pageErrors.push(err.message));
      page.on('console', (message) => {
        if (message.type() === 'error') record.consoleErrors.push(message.text());
      });
      page.on('requestfailed', (req) =>
        record.failed.push({
          url: req.url(),
          path: new URL(req.url()).pathname,
          error: req.failure(),
        }),
      );
      await page.routeWebSocket('**/*', (socket) => socket.close());
      await page.addInitScript((dark) => {
        document.addEventListener(
          'DOMContentLoaded',
          () => document.documentElement.classList.toggle('dark', dark),
          { once: true },
        );
        const audit = (window.__compositionFetchAudit = []);
        const original = window.fetch;
        window.fetch = function (input, init) {
          const url = new URL(
            typeof input === 'string' || input instanceof URL ? input : input.url,
            location.href,
          ).href;
          if (!['/api/resources', '/api/metrics-store/history'].includes(new URL(url).pathname))
            return original.call(this, input, init);
          const signal = init?.signal || (input instanceof Request ? input.signal : undefined);
          const item = {
            id: audit.length,
            url,
            signal: Boolean(signal),
            aborted: Boolean(signal?.aborted),
          };
          audit.push(item);
          signal?.addEventListener(
            'abort',
            () => {
              item.aborted = true;
            },
            { once: true },
          );
          return original.call(this, input, init);
        };
      }, dark);
      let resourceMode = 'retained',
        diskMode = 'current';
      let releaseHydration;
      const hydrationGate = new Promise((resolve) => (releaseHydration = resolve));
      let hydrationReleased = false;
      let releaseDisk;
      let diskGate;
      const GiB = 1024 ** 3;
      const rawResources = (mode) => {
        const now = '2026-10-20T12:05:00Z';
        const update =
          mode === 'local'
            ? { updateAvailable: false, error: 'local image' }
            : mode === 'pinned'
              ? { updateAvailable: false, error: 'digest-pinned image' }
              : {
                  updateAvailable: true,
                  currentDigest: 'sha256:current',
                  latestDigest: 'sha256:next',
                  lastChecked: now,
                };
        return [
          {
            id: 'app-container-web',
            name: 'edge-web',
            type: 'app-container',
            status: 'running',
            lastSeen: now,
            platformType: 'docker',
            technology: 'docker',
            sources: ['docker'],
            capabilities: [
              { name: 'update', type: 'common', platform: 'docker', minimumApprovalLevel: 'admin' },
            ],
            actionReadiness: [
              {
                name: 'update',
                available: false,
                reasonCode: 'command_agent_disconnected',
                reason: 'Docker / Podman command agent is not connected.',
              },
            ],
            metrics: { cpu: { percent: 2 }, memory: { percent: 25, used: GiB / 4, total: GiB } },
            docker: {
              agentId: 'agent-edge',
              hostSourceId: 'agent-edge',
              containerId: 'native-web',
              runtime: 'docker',
              containerState: 'running',
              image: mode === 'local' ? 'local-web:latest' : 'nginx:latest',
              updateStatus: update,
            },
          },
          {
            id: 'vm-resource-101',
            name: 'qualified-memory',
            type: 'vm',
            status: mode === 'offline' ? 'offline' : 'online',
            lastSeen: now,
            platformType: 'proxmox-pve',
            sources: ['proxmox', 'agent'],
            metrics: {
              cpu: { percent: 1 },
              memory: {
                percent: mode === 'current-zero' ? 0 : 75,
                used: mode === 'current-zero' ? 0 : 3 * GiB,
                total: 4 * GiB,
                observation: {
                  state: mode === 'current-zero' || mode === 'offline' ? 'current' : 'last-known',
                  source: 'guest-agent-meminfo',
                  observedAt: mode === 'current-zero' ? now : '2026-10-20T12:00:00Z',
                },
              },
            },
            proxmox: {
              vmid: 101,
              instance: 'fixture',
              nodeName: 'pve-a',
              runtimeStatus: mode === 'offline' ? 'stopped' : 'running',
              guestAgentStatus: 'available',
              memory: {
                usage: 25,
                used: GiB,
                total: 4 * GiB,
                observation: { state: 'current', source: 'status-mem', observedAt: now },
              },
            },
          },
        ];
      };
      const rawDisk = (temperature) => ({
        id: 'disk-101-a',
        parentId: 'vm-resource-101',
        type: 'physical_disk',
        name: 'Guest SATA',
        platformId: 'fixture',
        platformType: 'proxmox-pve',
        sources: ['agent'],
        sourceType: 'agent',
        status: 'online',
        lastSeen: '2026-10-20T12:00:00Z',
        physicalDisk: {
          devPath: '/dev/sda',
          model: 'Guest SATA',
          sizeBytes: GiB,
          health: 'PASSED',
          temperature,
        },
      });
      await page.route('**/*', async (route) => {
        const req = route.request(),
          url = new URL(req.url());
        if (url.origin !== origin || req.method() !== 'GET') {
          record.unexpected.push({ path: url.pathname, method: req.method() });
          return route.abort();
        }
        if (url.pathname === '/favicon.ico') return route.fulfill({ status: 204, body: '' });
        if (!url.pathname.startsWith('/api/')) return route.continue();
        const kind = url.searchParams.get('type');
        const mode = kind === 'physical_disk' ? diskMode : resourceMode;
        record.requests.push({ path: url.pathname, query: url.search, method: req.method(), mode });
        if (url.pathname === '/api/resources') {
          if (kind === 'physical_disk') {
            assert.equal(url.searchParams.get('parent'), 'vm-resource-101');
            if (mode === 'held-deadline') await diskGate;
            const status = mode === 'failed' ? 503 : 200;
            const rows =
              mode === 'empty'
                ? [{ ...rawDisk(99), id: 'foreign', parentId: 'other-guest' }]
                : [rawDisk(mode === 'recovered' ? 42 : mode === 'held-deadline' ? 99 : 45)];
            record.responses.push({
              kind,
              mode,
              status,
              ids: status === 200 ? rows.map((row) => row.id) : [],
            });
            return route.fulfill({
              status,
              json:
                status === 200
                  ? { resources: rows, meta: { totalPages: 1 } }
                  : { error: 'PRIVATE_FIXTURE_ERROR' },
            });
          }
          assert.equal(kind, 'app-container,vm', 'navigation must issue the canonical scoped read');
          if (!hydrationReleased) await hydrationGate;
          const rows = rawResources(mode);
          record.responses.push({
            kind,
            mode,
            status: 200,
            actionReadiness: rows[0].actionReadiness,
            memory: rows[1].metrics.memory,
            lastSeen: rows[1].lastSeen,
          });
          return route.fulfill({ json: { resources: rows, meta: { totalPages: 1 } } });
        }
        if (url.pathname === '/api/metrics-store/history') {
          const body = {
            resourceType: url.searchParams.get('resourceType'),
            resourceId: url.searchParams.get('resourceId'),
            range: url.searchParams.get('range'),
            source: 'store',
            start: Date.parse('2026-10-19T12:00:00Z'),
            end: Date.parse('2026-10-20T12:00:00Z'),
          };
          if (url.searchParams.has('metric')) {
            body.metric = url.searchParams.get('metric');
            body.points = [];
          } else body.metrics = {};
          return route.fulfill({ json: body });
        }
        if (url.pathname === '/api/alerts/active') return route.fulfill({ json: [] });
        const body = {
          '/api/license/runtime-capabilities': {
            capabilities: [],
            limits: [],
            max_history_days: 7,
            hosted_mode: false,
            runtime: { build: 'community' },
          },
          '/api/security/status': {
            hasAuthentication: true,
            requiresAuth: false,
            sessionCapabilities: {},
          },
          '/api/alerts/config': { enabled: true, activationState: 'active' },
          '/api/metrics-store/stats': { enabled: true },
          '/api/patrol/config': { enabled: false },
        }[url.pathname];
        if (!body) {
          record.unexpected.push({ path: url.pathname, method: req.method() });
          return route.fulfill({ status: 404, json: { error: 'Unexpected fixture read' } });
        }
        return route.fulfill({ json: body });
      });
      const click = async (locator) => {
        if (width < 500) await locator.tap();
        else {
          await locator.focus();
          await page.keyboard.press('Enter');
        }
      };
      const capture = async (label, locator) => {
        await locator.scrollIntoViewIfNeeded();
        await page.evaluate(() => document.fonts.ready);
        assert(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1),
          'no page overflow',
        );
        const file = `${name}-${label}.png`;
        await locator.screenshot({ path: path.join(output, file), animations: 'disabled' });
        report.captures.push({
          file,
          sha256: hash(fs.readFileSync(path.join(output, file))),
          name,
          label,
        });
        save();
      };
      const view = (name) => click(page.getByRole('button', { name, exact: true }));
      const refresh = async (mode) => {
        resourceMode = mode;
        await page.evaluate(() => window.__composition.refresh());
      };
      const reread = async (mode) => {
        diskMode = mode;
        await page.evaluate(() => window.__composition.rereadDisk());
      };
      await page.goto(origin + '/browser-tests/resource-agent-observation.html?kind=composition', {
        waitUntil: 'domcontentloaded',
      });
      await expect(page.locator('[data-seed-count]')).toHaveText('2 websocket-only resources');
      assert.equal(record.requests.filter((row) => row.path === '/api/resources').length, 0);
      await click(page.getByRole('button', { name: 'Open REST-backed resources' }));
      await expect
        .poll(() => record.requests.filter((row) => row.path === '/api/resources').length)
        .toBe(1);
      const seedRows = await page.evaluate(() => window.__composition.evidence());
      assert.equal(
        seedRows.find((row) => row.id === 'app-container-web').actionReadiness,
        undefined,
      );
      hydrationReleased = true;
      releaseHydration();
      await page.waitForFunction(
        () =>
          window.__composition.evidence().find((row) => row.id === 'app-container-web')
            ?.actionReadiness?.[0]?.available === false,
      );
      record.checks.push(
        'A fresh websocket-only all-resources cache seeds navigation, then the scoped route immediately performs REST and consumes REST-only update refusal.',
      );
      const container = page.getByRole('region', { name: 'Container details consumer' });
      const details = container.getByTestId('resource-technical-details');
      await click(details.locator(':scope > summary'));
      await expect(details).toHaveAttribute('open', '');
      const locked = container.locator(
        '[tabindex="0"][aria-label^="Update available. Pulse cannot apply it:"]',
      );
      await expect(locked).toHaveCount(1);
      await expect(locked).toHaveAttribute(
        'aria-label',
        /Docker \/ Podman command agent is not connected/,
      );
      await expect(container.getByRole('button', { name: /update/i })).toHaveCount(0);
      await click(locked);
      await expect(
        page.getByText(/Apply this update using your container manager on the host\./),
      ).toBeVisible();
      await capture('rest-update-refusal', container);
      await page.keyboard.press('Escape');
      await view('Container table');
      const table = page.getByRole('region', { name: 'Container table consumer' });
      await expect(
        table.locator('[tabindex="0"][aria-label^="Update available. Pulse cannot apply it:"]'),
      ).toHaveCount(1);
      await expect(table.getByRole('button', { name: /update/i })).toHaveCount(0);
      record.checks.push(
        'REST refusal is a focusable/tappable explanation, not an Update action, in both the real Docker table and full Infrastructure drawer; no POST occurs.',
      );
      await view('Container details');
      await click(container.getByTestId('resource-technical-details').locator(':scope > summary'));
      await refresh('local');
      const updateRow = container
        .getByTestId('resource-docker-container-section')
        .getByRole('row')
        .filter({ has: page.getByText('Image update', { exact: true }) });
      await expect(updateRow).toContainText('Local image');
      await expect(updateRow).not.toContainText('Check failed');
      assert.equal(
        await updateRow.locator('.text-rose-600, .text-rose-700, .text-danger').count(),
        0,
      );
      await capture('local-image-neutral', container);
      await refresh('pinned');
      await expect(updateRow).toContainText('Pinned digest');
      await expect(updateRow).not.toContainText('Check failed');
      record.checks.push(
        'Local and digest-pinned images remain uncheckable/neutral in the actual details inventory, not false registry check failures.',
      );
      await view('Memory details');
      await refresh('retained');
      const memory = page.getByRole('region', { name: 'Memory details consumer' });
      const facts = memory.getByTestId('resource-drawer-facts');
      await expect(facts).toContainText('4.00 GB RAM');
      await expect(facts).not.toContainText('of 4.00 GB RAM');
      await expect(memory.locator('[data-history-current="memory"]')).toHaveCount(0);
      await expect(memory.locator('[data-history-last-known="memory"]')).toContainText('75.0%');
      await expect(memory.locator('[data-history-deferred="memory"]')).toContainText(
        '2026-10-20 12:00:00 UTC',
      );
      await expect(memory.locator('[data-history-deferred="memory"]')).not.toContainText(
        '12:05:00',
      );
      await capture('retained-memory', memory);
      await refresh('current-zero');
      await expect(facts).toContainText('0 B of 4.00 GB RAM');
      await expect(memory.locator('[data-history-current="memory"]')).toContainText('0.0%');
      await refresh('offline');
      await expect(facts).not.toContainText('of 4.00 GB RAM');
      await expect(memory.locator('[data-history-current="memory"]')).toHaveCount(0);
      record.checks.push(
        'REST hydration and a fresh carrier/platform memory do not renew retained guest memory: facts show capacity and charts keep the original observation time; measured zero is current, stopped VM withdraws used/current amounts.',
      );
      await view('Disk inventory');
      const inventory = page.getByRole('region', { name: 'Disk inventory consumer' });
      const diskRow = inventory.getByTestId('guest-physical-disk');
      await expect(diskRow).toContainText('45°C');
      await reread('failed');
      const error = inventory
        .getByRole('status')
        .filter({ hasText: 'Physical disk details are unavailable' });
      await expect(error).toBeVisible();
      await expect(diskRow).toHaveCount(0);
      await expect(inventory).not.toContainText('PRIVATE_FIXTURE_ERROR');
      diskMode = 'recovered';
      await click(inventory.getByRole('button', { name: 'Retry physical disk details' }));
      await expect(diskRow).toContainText('42°C');
      await expect(error).toHaveCount(0);
      record.checks.push(
        'A failed background scoped read still withdraws old physical-disk rows and private error text; a keyboard/touch Retry restores a successful read.',
      );
      diskGate = new Promise((resolve) => (releaseDisk = resolve));
      const beforeDeadline = record.requests.filter(
        (row) => row.path === '/api/resources' && row.query.includes('physical_disk'),
      ).length;
      await reread('held-deadline');
      await expect
        .poll(
          () =>
            record.requests.filter(
              (row) => row.path === '/api/resources' && row.query.includes('physical_disk'),
            ).length,
        )
        .toBe(beforeDeadline + 1);
      await page.clock.fastForward(19_000);
      await expect(error).toHaveCount(0);
      await expect(diskRow).toContainText('42°C');
      await page.clock.fastForward(1_001);
      await expect(error).toBeVisible();
      await expect(error).toContainText('Reads stop waiting after 20 seconds');
      await expect(diskRow).toHaveCount(0);
      const audit = await page.evaluate(() => window.__compositionFetchAudit);
      assert(
        audit.some(
          (item) => item.aborted && new URL(item.url).searchParams.get('type') === 'physical_disk',
        ),
        'the production deadline must abort the scoped read',
      );
      releaseDisk();
      await expect(diskRow).toHaveCount(0);
      await expect(error).toBeVisible();
      const retry = inventory.getByRole('button', { name: 'Retry physical disk details' });
      await expect(retry).toBeEnabled();
      if (width < 500) assert((await retry.boundingBox()).height >= 44);
      await capture('bounded-background-failure', inventory);
      diskMode = 'empty';
      await click(retry);
      await expect(error).toHaveCount(0);
      await expect(inventory.getByRole('status')).toContainText(
        'Missing inventory does not establish disk absence or health',
      );
      await expect(diskRow).toHaveCount(0);
      record.checks.push(
        'The real 20-second deadline aborts a pending background GET, keeps old readings hidden even when its late response arrives, and leaves Retry usable; a foreign-only successful response becomes missing inventory, not healthy/no hardware.',
      );
      record.fetchAudit = await page.evaluate(() => window.__compositionFetchAudit);
      const expectedAborts = new Map();
      for (const item of record.fetchAudit)
        if (item.aborted) expectedAborts.set(item.url, (expectedAborts.get(item.url) || 0) + 1);
      for (const item of record.failed) {
        assert((expectedAborts.get(item.url) || 0) > 0, `unmatched request failure ${item.url}`);
        expectedAborts.set(item.url, expectedAborts.get(item.url) - 1);
      }
      record.expectedHttp503 = record.consoleErrors.filter((message) => /503/.test(message));
      assert(
        record.consoleErrors.every((message) =>
          /^Failed to load resource: the server responded with a status of 503/.test(message),
        ),
        JSON.stringify(record.consoleErrors),
      );
      assert.deepEqual(record.pageErrors, []);
      assert.deepEqual(record.unexpected, []);
      assert(record.requests.every((request) => request.method === 'GET'));
      save();
      await context.close();
      context = null;
      report.cleanup.contextsClosed++;
      await browser.close();
      browser = null;
      report.cleanup.browsersClosed++;
    }
    report.result = 'passed';
  } catch (error) {
    report.error = String(error.stack || error);
    process.exitCode = 1;
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
    save();
    console.log(JSON.stringify(report, null, 2));
  }
})();
