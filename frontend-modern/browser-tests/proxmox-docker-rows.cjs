const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

const now = '2026-10-03T12:00:00Z';
const base = (id, type, extra = {}) => ({
  id,
  type,
  name: id,
  status: 'online',
  lastSeen: now,
  sources: ['docker'],
  ...extra,
});
const observations = (mode) => [
  base('mail-gateway-eu', 'pmg', {
    sources: ['pmg'],
    technology: 'pmg',
    uptime: 86400,
    sourceStatus: { pmg: { status: 'online', lastSeen: now } },
    ...(mode === 'missing'
      ? {}
      : {
          pmg: {
            instanceId: 'pmg-eu',
            hostname: 'mail-gateway-eu',
            version: '8.1-2',
            nodeCount: 2,
            mailCountTotal: 2740,
            spamIn: 321,
            virusIn: 14,
            quarantine: 3,
            queueTotal: mode === 'fresh' ? 19 : 11,
            queueDeferred: 5,
          },
        }),
  }),
  base('nfs-iso', 'storage', {
    sources: ['proxmox'],
    parentName: 'cluster',
    parentId: 'pve-cluster-a',
    technology: 'nfs',
    metrics: { disk: { total: 1e12, used: 3e11, percent: 30 } },
    storage: { type: 'nfs', shared: true, nodes: ['pve1', 'pve2', 'pve3'] },
  }),
  base('local-lvm', 'storage', {
    sources: ['proxmox'],
    parentName: 'pve2',
    parentId: 'pve2',
    metrics: { disk: { total: 1e12, used: 6e11, percent: 60 } },
    storage: { type: 'lvmthin', shared: false },
  }),
  base('edge-web', 'app-container', {
    status: 'running',
    technology: 'docker',
    sourceStatus: { docker: { status: 'online', lastSeen: now } },
    actionReadiness: [{ name: 'restart', available: false, reason: 'Command agent disconnected' }],
    metrics: { cpu: { percent: 42 }, memory: { percent: 50, used: 5e8, total: 1e9 } },
    docker: {
      networks: [{ name: 'edge-net', ipv4: '10.88.0.10' }],
      agentId: 'agent-edge',
      hostSourceId: 'agent-edge',
      containerId: 'native-edge-web',
      hostname: 'edge-long-hostname',
      runtime: 'docker',
      runtimeVersion: '27.5.1',
      image: 'nginx:latest',
      containerState: 'running',
      // Running but failing its health check: row and drawer must both say so.
      health: 'unhealthy',
      restartCount: 7,
      updateStatus: {
        updateAvailable: true,
        currentDigest: 'sha256:current',
        latestDigest: 'sha256:latest',
        lastChecked: now,
      },
    },
  }),
  base('edge-cache', 'app-container', {
    status: 'offline',
    metrics: { cpu: { percent: 0 }, memory: { percent: 0 } },
    docker: {
      networks: [{ name: 'edge-net', ipv4: '10.88.0.11' }],
      agentId: 'agent-edge',
      hostname: 'edge-long-hostname',
      runtime: 'podman',
      runtimeVersion: '5.2.1',
      image: 'redis:7.4',
      containerState: 'exited',
      restartCount: 0,
    },
  }),
  // No container state reported: its degraded unified status must read amber.
  base('edge-queue', 'app-container', {
    status: 'degraded',
    metrics: { cpu: { percent: 3 }, memory: { percent: 20 } },
    docker: {
      networks: [{ name: 'edge-net', ipv4: '10.88.0.12' }],
      agentId: 'agent-edge',
      hostSourceId: 'agent-edge',
      containerId: 'native-edge-queue',
      hostname: 'edge-long-hostname',
      runtime: 'docker',
      runtimeVersion: '27.5.1',
      image: 'rabbitmq:3',
    },
  }),
  // Running, but restarted past the attention threshold: its dot reads amber.
  base('edge-loop', 'app-container', {
    status: 'running',
    metrics: { cpu: { percent: 5 }, memory: { percent: 10 } },
    docker: {
      networks: [{ name: 'edge-net', ipv4: '10.88.0.13' }],
      agentId: 'agent-edge',
      hostname: 'edge-long-hostname',
      runtime: 'docker',
      runtimeVersion: '27.5.1',
      image: 'worker:2',
      containerState: 'running',
      restartCount: 12,
    },
  }),
  base('edge-net', 'docker-network', {
    docker: {
      hostSourceId: 'agent-edge',
      hostname: 'edge-long-hostname',
      networkId: 'net-edge',
      driver: 'bridge',
      scope: 'local',
      enableIpv4: true,
    },
  }),
  base('nginx:latest', 'docker-image', {
    docker: {
      hostname: 'edge-long-hostname',
      imageId: 'image-nginx',
      repoTags: ['nginx:latest'],
      sizeBytes: 2e8,
      containers: ['edge-web'],
      containerCount: 1,
    },
  }),
  base('web-service', 'docker-service', {
    docker: {
      hostname: 'edge-long-hostname',
      image: 'nginx:latest',
      serviceMode: 'replicated',
      desiredTasks: 4,
      runningTasks: 2,
      mode: 'replicated',
      serviceUpdate: { state: 'rollback_started', message: 'Synthetic rollout observation' },
    },
  }),
  base('web.2', 'docker-task', {
    docker: {
      hostname: 'edge-long-hostname',
      serviceName: 'web-service',
      nodeName: 'worker-1',
      currentState: 'running',
      desiredState: 'running',
      startedAt: '2026-10-03T11:30:00.123456Z',
    },
  }),
];

(async () => {
  const root = '/workspace/frontend-modern';
  const output = '/workspace/tmp/proxmox-docker-rows-final-source';
  fs.mkdirSync(output, { recursive: true });
  process.chdir(root);
  const playwright = require('playwright/package.json').version;
  assert.equal(
    playwright,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  // Keep the fixture runnable from clean committed source, without requiring
  // an invocation-specific untracked runtime-paths input.
  const paths = [
    'frontend-modern/src/components/Workloads/StackedDiskBar.tsx',
    'frontend-modern/src/components/Workloads/stackedDiskBarModel.ts',
    'frontend-modern/src/features/docker/DockerImagesTable.tsx',
    'frontend-modern/src/features/docker/DockerServicesTable.tsx',
    'frontend-modern/src/features/docker/DockerTasksTable.tsx',
    'frontend-modern/src/features/docker/dockerContainerTableModel.ts',
    'frontend-modern/src/features/storageBackups/resourceStoragePresentation.ts',
    'frontend-modern/src/features/storageBackups/storageAdapters.ts',
    'frontend-modern/src/hooks/useUnifiedResources.ts',
  ];
  const hashes = Object.fromEntries(
    paths.map((p) => [
      p,
      crypto
        .createHash('sha256')
        .update(fs.readFileSync(path.join('/workspace', p)))
        .digest('hex'),
    ]),
  );
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(output, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5272, strictPort: true },
  });
  const results = [],
    screenshots = [];
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width] of [
      ['desktop', chromium, 1440],
      ['phone', webkit, 390],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height: width === 390 ? 844 : 900 },
        isMobile: width === 390,
        hasTouch: width === 390,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      await page.clock.setFixedTime(new Date(now));
      page.setDefaultTimeout(15000);
      const checks = [],
        errors = [],
        offOrigin = [],
        mutations = [];
      let mode = 'initial';
      page.on('pageerror', (e) => errors.push(e.message));
      await page.route('**/*', async (route) => {
        const request = route.request(),
          url = new URL(request.url());
        if (url.origin !== 'http://127.0.0.1:5272') {
          offOrigin.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (request.method() !== 'GET') {
          mutations.push({ path: url.pathname, method: request.method() });
          return route.fulfill({ status: 409, json: { error: 'No fixture mutation' } });
        }
        if (url.pathname === '/api/resources')
          return route.fulfill({ json: { data: observations(mode) } });
        if (url.pathname === '/api/license/runtime-capabilities')
          return route.fulfill({
            json: {
              capabilities: [],
              limits: [],
              max_history_days: 7,
              hosted_mode: false,
              blocked_capabilities: [],
              runtime: { build: 'community' },
            },
          });
        if (url.pathname === '/api/security/status')
          return route.fulfill({ json: { hasAuthentication: true, requiresAuth: true } });
        return route.fulfill({
          json: {
            enabled: false,
            data: [],
            capabilities: [],
            facets: [],
            relationships: [],
            recentChanges: [],
            alerts: {},
          },
        });
      });
      const check = async (label, run) => {
        try {
          await run();
          checks.push({ label, passed: true });
        } catch (e) {
          checks.push({ label, passed: false, error: e.message });
        }
      };
      const select = (view) =>
        page
          .getByRole('navigation', { name: 'Fixture views' })
          .getByRole('button', { name: view, exact: true })
          .click();
      const capture = async (state) => {
        await page.waitForTimeout(180); // ResizeObserver and entrance layout, not freshness evidence.
        const dimensions = await page.evaluate(() => ({
          width: innerWidth,
          scroll: document.documentElement.scrollWidth,
        }));
        checks.push({
          label: `No horizontal page overflow: ${state}`,
          passed: dimensions.scroll <= dimensions.width + 1,
          dimensions,
        });
        const file = `${name}-${state}.png`;
        await page.screenshot({ path: path.join(output, file), fullPage: true });
        screenshots.push({ file, state, engine: name, width, dimensions });
      };
      await page.goto('http://127.0.0.1:5272/browser-tests/proxmox-docker-rows.html', {
        waitUntil: 'domcontentloaded',
        timeout: 120000,
      });
      if (width === 390) await page.evaluate(() => document.documentElement.classList.add('dark'));
      await page.getByText('11 REST-mapped rows', { exact: true }).waitFor();
      await check(
        'REST-first PMG counters and facets are present before any websocket hydration',
        async () => {
          const table = page.getByRole('region', { name: 'Mail table' });
          await table.getByText('2,740', { exact: true }).waitFor();
          await table.getByText('11', { exact: true }).waitFor();
          const row = await page.evaluate(() =>
            window.__operatorRows.rows().find((r) => r.type === 'pmg'),
          );
          assert.equal(row.technology, 'pmg');
          assert.equal(row.sourceStatus.pmg.status, 'online');
          assert.equal(row.pmg.queueTotal, 11);
          assert.deepEqual(row.pmg, row.platformData.pmg);
          if (width > 390) await table.getByText('8.1-2', { exact: true }).waitFor();
        },
      );
      await capture('rest-mail');
      await check(
        'Canonical store hydration followed by REST refresh keeps the new PMG counters',
        async () => {
          await page.evaluate(() => window.__operatorRows.seedWebsocket());
          mode = 'fresh';
          await page.evaluate(() => window.__operatorRows.refresh());
          await page
            .getByRole('region', { name: 'Mail table' })
            .getByText('19', { exact: true })
            .waitFor();
        },
      );
      await capture('refreshed-mail');
      await check('A removed REST PMG facet does not retain obsolete queue evidence', async () => {
        mode = 'missing';
        await page.evaluate(() => window.__operatorRows.refresh());
        assert.equal(
          await page
            .getByRole('region', { name: 'Mail table' })
            .getByText('19', { exact: true })
            .count(),
          0,
        );
        const row = await page.evaluate(() =>
          window.__operatorRows.rows().find((r) => r.type === 'pmg'),
        );
        assert.equal(row.pmg, undefined);
        mode = 'fresh';
        await page.evaluate(() => window.__operatorRows.refresh());
      });
      await select('Storage');
      await check('Shared storage vocabulary preserves native locations and IDs', async () => {
        const table = page.getByRole('region', { name: 'Storage table' });
        await table.getByText('NFS', { exact: true }).waitFor({ state: 'attached' });
        await table.getByText('LVM-Thin', { exact: true }).waitFor({ state: 'attached' });
        await table.getByText('Shared · 3 nodes', { exact: true }).waitFor({ state: 'attached' });
        await table.getByText('pve2', { exact: true }).waitFor({ state: 'attached' });
        const rows = await page.evaluate(() =>
          window.__operatorRows.rows().filter((r) => r.type === 'storage'),
        );
        assert.deepEqual(
          rows.map((r) => [r.id, r.parentId]),
          [
            ['nfs-iso', 'pve-cluster-a'],
            ['local-lvm', 'pve2'],
          ],
        );
      });
      await capture('storage');
      await select('Containers');
      await check(
        'REST lifecycle refusal remains available; phone row keeps update and moves restarts to detail',
        async () => {
          await page
            .locator('[data-lifecycle-refusal]')
            .getByText('Command agent disconnected', { exact: true })
            .waitFor();
          const table = page.getByRole('region', { name: 'Container table' });
          const update = table.locator('.docker-container-update-cell button');
          await update.waitFor();
          assert.match(await update.innerText(), /Update/);
          const bounds = await update.evaluate((e) => ({
            w: e.getBoundingClientRect().width,
            cell: e.closest('td').getBoundingClientRect().width,
            scroll: e.scrollWidth,
            client: e.clientWidth,
          }));
          assert.ok(
            bounds.w <= bounds.cell + 1 && bounds.scroll <= bounds.client + 1,
            JSON.stringify(bounds),
          );
          const headers = await table.locator('th:visible').allTextContents();
          if (width === 390) {
            assert.ok(!headers.some((h) => h.includes('Restarts')));
            // A phone row reads the state words under the name, whole: as a
            // column beside the name they were cut to "Unhe…".
            assert.ok(!headers.some((h) => h.includes('State')));
            const words = table
              .locator('[data-docker-container-row="edge-web"]')
              .locator('[data-docker-container-state]');
            assert.equal(await words.innerText(), 'Unhealthy');
            const fit = await words.evaluate((e) => ({
              scroll: e.scrollWidth,
              client: e.clientWidth,
              name: e.closest('td') === e.closest('tr').querySelector('td'),
            }));
            assert.ok(fit.name && fit.scroll <= fit.client, JSON.stringify(fit));
          }
        },
      );
      await capture('containers');
      await check(
        'Restart count and native identity remain reachable via row disclosure',
        async () => {
          const row = page.locator('[data-docker-container-row="edge-web"]');
          if (width === 390) await row.locator('td').first().tap();
          else await row.getByRole('button', { name: /details/i }).click();
          // A worrying restart count is lifted in front of the drawer's charts.
          const detail = page.getByTestId('resource-drawer-exceptions');
          await detail.getByText('Restarts', { exact: true }).waitFor();
          await detail.getByText('7', { exact: true }).waitFor();
          // The lifted update offers its next steps: the row's update control and
          // where to see the new image.
          const updateLine = detail.locator('tr').filter({ hasText: 'Image update' });
          const drawerUpdate = updateLine.getByRole('button').filter({ hasText: 'Update' });
          await drawerUpdate.waitFor();
          assert.equal(await drawerUpdate.isEnabled(), true);
          if (width === 390) {
            // Outside the table a phone taps this control: 44px tall at least.
            const box = await drawerUpdate.boundingBox();
            assert.ok(box && box.height >= 44, JSON.stringify(box));
          }
          assert.match(
            await updateLine.getByRole('link', { name: 'View image tags' }).getAttribute('href'),
            /nginx/,
          );
          // The drawer header dot agrees with the row's: unhealthy is red, not green.
          const headerDot = page.getByRole('img', { name: 'Unhealthy' }).first();
          await headerDot.waitFor();
          const rowDot = row.locator('[title="Unhealthy"]').first();
          const colours = await Promise.all(
            [headerDot, rowDot].map((dot) =>
              dot.evaluate((el) => getComputedStyle(el).backgroundColor),
            ),
          );
          assert.equal(colours[0], colours[1], JSON.stringify(colours));
          assert.match(colours[0], /^rgb\((2[0-9]{2}), ([0-9]{1,2}|1[0-2][0-9]), /);
        },
      );
      await capture('container-detail');
      await check('A degraded container reads amber in its row and its drawer', async () => {
        const row = page.locator('[data-docker-container-row="edge-queue"]');
        const rowDot = row.locator('[title="Degraded"]').first();
        await rowDot.waitFor();
        if (width === 390) await row.locator('td').first().tap();
        else await row.getByRole('button', { name: /details/i }).click();
        const headerDot = page.getByRole('img', { name: 'Degraded' }).first();
        await headerDot.waitFor();
        const colours = await Promise.all(
          [rowDot, headerDot].map((dot) =>
            dot.evaluate((el) => getComputedStyle(el).backgroundColor),
          ),
        );
        assert.equal(colours[0], colours[1], JSON.stringify(colours));
        // Amber: strong red and green, little blue.
        const [r, g, b] = colours[0].match(/\d+/g).map(Number);
        assert.ok(r > 200 && g > 120 && b < 80, JSON.stringify(colours));
      });
      await capture('container-degraded');
      await check(
        'A crash-looping running container reads amber in its dot and drawer',
        async () => {
          const row = page.locator('[data-docker-container-row="edge-loop"]');
          const rowDot = row.locator('[title="12 restarts"]').first();
          await rowDot.waitFor();
          const amber = (colour) => {
            const [r, g, b] = colour.match(/\d+/g).map(Number);
            return r > 200 && g > 120 && b < 80;
          };
          const dotColour = await rowDot.evaluate((el) => getComputedStyle(el).backgroundColor);
          assert.ok(amber(dotColour), dotColour);
          if (width === 390) {
            // No Restarts column on a phone: the words under the name say why.
            const words = row.locator('[data-docker-container-state]');
            assert.equal(await words.innerText(), '12 restarts');
            assert.equal(await words.getAttribute('data-docker-container-state'), 'warning');
            await row.locator('td').first().tap();
          } else {
            await row.getByRole('button', { name: /details/i }).click();
          }
          const headerDot = page.getByRole('img', { name: '12 restarts' }).first();
          await headerDot.waitFor();
          assert.equal(
            await headerDot.evaluate((el) => getComputedStyle(el).backgroundColor),
            dotColour,
          );
          const detail = page.getByTestId('resource-drawer-exceptions');
          await detail.getByText('Restarts', { exact: true }).waitFor();
        },
      );
      await capture('container-restarts');
      await select('Images');
      await check('Image verdict fits; phone hidden headers and cells are symmetric', async () => {
        const table = page.getByRole('region', { name: 'Image table' });
        await table.getByText('nginx:latest', { exact: true }).waitFor();
        const headers = await table.locator('th:visible').allTextContents();
        const row = table.locator('[data-docker-image-row]');
        assert.equal(await row.locator('td:visible').count(), headers.length);
        if (width === 390) {
          assert.ok(!headers.some((h) => h.includes('Host') || h.includes('Used by')));
          assert.ok(headers.some((h) => h.includes('Size')));
        }
      });
      await capture('images');
      await select('Networks');
      await check('Network Running filter follows run state, not the verdict', async () => {
        const table = page.getByRole('region', { name: 'Network table' });
        const networkRow = table.locator('[data-docker-network-row="edge-net"]');
        const headers = await table.locator('th:visible').allTextContents();
        // Every visible header sits over its own cells, and a phone leaves
        // Subnets and Driver to the expansion's Network details.
        assert.equal(await networkRow.locator('td:visible').count(), headers.length);
        if (width === 390) {
          assert.ok(!headers.some((h) => /Subnets|Driver/i.test(h)), JSON.stringify(headers));
        }
        // The state counts every container the Attention filter holds, whole.
        const state = networkRow.getByText('3 need attention', { exact: true });
        const fit = await state.evaluate((e) => ({ scroll: e.scrollWidth, client: e.clientWidth }));
        assert.ok(fit.scroll <= fit.client, JSON.stringify(fit));
        // One dot per row, in the name cell: amber, the network degraded by what
        // is on it (old: a green "Online" dot beside a red one). Each container
        // keeps its own severity in the expansion.
        const dots = networkRow.locator('[title="3 need attention"]');
        assert.equal(await dots.count(), 2); // the dot and the State words' hover
        const dotColour = await dots.first().evaluate((el) => getComputedStyle(el).backgroundColor);
        const [r, g, b] = dotColour.match(/\d+/g).map(Number);
        assert.ok(r > 200 && g > 120 && b < 80, dotColour);
        assert.equal(await networkRow.locator('[title="Online"]').count(), 0);
        await networkRow.click();
        const detail = table.locator('[data-docker-network-detail-row="edge-net"]');
        await detail.getByText('edge-loop', { exact: true }).waitFor();
        const visibleNames = async () =>
          (
            await Promise.all(
              ['edge-web', 'edge-loop', 'edge-queue', 'edge-cache'].map(async (name) =>
                (await detail.getByText(name, { exact: true }).count()) > 0 ? name : null,
              ),
            )
          ).filter(Boolean);
        // A phone keeps the status chips behind the toolbar's Filters button.
        const chooseStatus = async (name) => {
          const chip = page.getByRole('button', { name });
          if (!(await chip.isVisible())) {
            await detail.getByRole('button', { name: /Filters/ }).click();
          }
          await chip.click();
        };
        // Unhealthy and crash-looping containers still run: they match Running
        // and sit in its Attention section.
        await chooseStatus(/^Running, 2$/);
        assert.deepEqual(await visibleNames(), ['edge-web', 'edge-loop']);
        await detail.getByText('Needs operator review', { exact: true }).waitFor();
        // Each card's state words take its dot's colour: red, then amber.
        const cardTones = await detail
          .locator('[data-docker-attachment-state]')
          .evaluateAll((spans) =>
            spans.map((s) => [
              s.textContent,
              getComputedStyle(s).color,
              s.dataset.dockerAttachmentState,
            ]),
          );
        assert.deepEqual(
          cardTones.map(([text, , variant]) => [text, variant]),
          [
            ['Unhealthy', 'danger'],
            ['12 restarts', 'warning'],
          ],
        );
        assert.notEqual(cardTones[0][1], cardTones[1][1], JSON.stringify(cardTones));
        await chooseStatus(/^Other, 2$/);
        assert.deepEqual(await visibleNames(), ['edge-queue', 'edge-cache']);
        await chooseStatus(/^Attention, 3$/);
        assert.deepEqual(await visibleNames(), ['edge-web', 'edge-loop', 'edge-queue']);
        await chooseStatus(/^Running, 2$/);
      });
      await capture('networks');
      await select('Swarm');
      await check(
        'Swarm state and started time use operator wording with raw evidence retained',
        async () => {
          const state = page.getByText('Rollback started', { exact: true });
          await state.waitFor({ state: 'attached' });
          assert.match(
            await state.locator('xpath=ancestor-or-self::*[@title][1]').getAttribute('title'),
            /rollback_started/,
          );
          const visibleState = await state.evaluate((element) => {
            const cell = element.closest('td').getBoundingClientRect();
            const range = document.createRange();
            range.selectNodeContents(element);
            return [...range.getClientRects()].every(
              (line) =>
                line.left >= cell.left &&
                line.right <= cell.right &&
                line.top >= cell.top &&
                line.bottom <= cell.bottom,
            );
          });
          assert.equal(visibleState, true, 'Every word of the rollout state must fit its cell');
          if (width === 390) {
            await page.getByText('Run', { exact: true }).waitFor({ state: 'visible' });
          }
          const started = page.locator('[title="2026-10-03T11:30:00.123456Z"]');
          await started.waitFor({ state: 'attached' });
          assert.match(await started.innerText(), /ago/);
          const taskState = page
            .getByRole('region', { name: 'Swarm tasks' })
            .locator('[data-docker-task-state]');
          await taskState.waitFor({ state: 'attached' });
          // Current reads the task dot's own word; Docker's token stays on hover.
          assert.equal(await taskState.textContent(), 'Running');
          assert.equal(await taskState.getAttribute('title'), 'running');
          // The service dot names the rollout, and the cells behind its verdict
          // read in its amber: the Update words and the short Running count.
          const serviceRow = page.locator('[data-docker-service-row="web-service"]');
          const amber = (colour) => {
            const [r, g, b] = colour.match(/\d+/g).map(Number);
            return r > 150 && g > 70 && b < 80;
          };
          const colourOf = (locator) =>
            locator.evaluate((el) =>
              getComputedStyle(el).backgroundColor === 'rgba(0, 0, 0, 0)'
                ? getComputedStyle(el).color
                : getComputedStyle(el).backgroundColor,
            );
          const dot = serviceRow.locator('[title="Rolling back"]').first();
          await dot.waitFor();
          const running = serviceRow.locator('[data-docker-service-running="warning"]');
          // A phone hides Desired, so the count reads out of what.
          assert.equal(
            (await running.innerText()).replace(/\s+/g, ''),
            width === 390 ? '2/4' : '2',
          );
          for (const colour of [
            await colourOf(dot),
            await colourOf(state),
            await colourOf(serviceRow.locator('[data-docker-service-running="warning"]')),
          ]) {
            assert.ok(amber(colour), colour);
          }
        },
      );
      await capture('swarm');
      await check(
        'Swarm touch or keyboard disclosure exposes raw rollout and placement evidence',
        async () => {
          const row = page.locator('[data-docker-service-row="web-service"]');
          if (width === 390) await row.locator('td').first().tap();
          else {
            const toggle = row.getByRole('button', {
              name: 'Expand details for web-service',
              exact: true,
            });
            await toggle.focus();
            await page.keyboard.press('Enter');
          }
          const detail = page.locator('[data-inline-docker-service-detail-for="web-service"]');
          await detail
            .getByText('rollback_started | Synthetic rollout observation', { exact: true })
            .waitFor();
          await detail.getByText('edge-long-hostname', { exact: true }).waitFor();
          await detail.getByText('4', { exact: true }).waitFor();
          await detail.getByText('2', { exact: true }).waitFor();
        },
      );
      await capture('swarm-detail');
      await select('Disk');
      await check(
        'Multi-disk Bars show the measured fullest disk and unchanged equal-weight bars',
        async () => {
          const label = page.locator('[data-stacked-disk-max-label]');
          assert.equal(await label.innerText(), '92%');
          assert.equal(await label.getAttribute('title'), 'Highest usage: /data 92%');
          assert.equal(await page.locator('[data-stacked-disk-fill="vertical"]').count(), 2);
        },
      );
      await capture('disks');
      checks.push({
        label: 'No page errors, external requests or mutation requests',
        passed: !errors.length && !offOrigin.length && !mutations.length,
        errors,
        offOrigin,
        mutations,
      });
      results.push({ engine: name, version: browser.version(), width, checks });
      await browser.close();
      browser = undefined;
    }
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
  const result = {
    playwright,
    verified_at: new Date().toISOString(),
    paths,
    hashes,
    results,
    screenshots,
  };
  fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2));
  const failures = results.flatMap((r) =>
    r.checks.filter((c) => !c.passed).map((c) => ({ engine: r.engine, ...c })),
  );
  console.log(
    JSON.stringify({
      checked: results.reduce((n, r) => n + r.checks.length, 0),
      screenshots: screenshots.length,
      failures,
    }),
  );
  assert.equal(failures.length, 0, JSON.stringify(failures));
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
