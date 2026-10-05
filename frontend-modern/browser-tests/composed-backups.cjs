// Production orchestration/table/CSS with synthetic read-only HTTP evidence.
// Confirms combined Tailwind rendering and source attribution, not native backup recovery.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
(async () => {
  const root = '/workspace/frontend-modern';
  const output = '/workspace/tmp/composed-backups-proof';
  fs.mkdirSync(output, { recursive: true });
  process.chdir(root);
  const playwright = require('playwright/package.json').version;
  assert.equal(
    playwright,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(output, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5295, strictPort: true },
  });
  const runtime = [
    'frontend-modern/src/features/proxmox/ProxmoxBackupsTable.tsx',
    'frontend-modern/src/features/proxmox/proxmoxBackupsTableShared.tsx',
  ];
  const content_sha256 = Object.fromEntries(
    runtime.map((p) => [
      p,
      crypto
        .createHash('sha256')
        .update(fs.readFileSync(path.join('/workspace', p)))
        .digest('hex'),
    ]),
  );
  const observations = [];
  let browser;
  try {
    await server.listen();
    for (const [name, engine, phone] of [
      ['chromium-desktop', chromium, false],
      ['webkit-phone', webkit, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: phone ? { width: 390, height: 844 } : { width: 1365, height: 900 },
        hasTouch: phone,
        isMobile: phone,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      page.setDefaultTimeout(20000);
      await page.clock.setFixedTime(new Date('2026-10-03T12:00:00Z'));
      const errors = [],
        requests = [],
        offOrigin = [],
        checks = [],
        screenshots = [];
      const now = '2026-10-03T12:00:00Z';
      const workload = {
        id: 'ct-310',
        type: 'system-container',
        name: 'artifact-cache-310',
        displayName: 'artifact-cache-310',
        platformId: 'fixture-estate',
        platformType: 'proxmox-pve',
        sourceType: 'api',
        status: 'running',
        lastSeen: now,
        proxmox: { vmid: 310, node: 'pve-edge', instance: 'homelab' },
      };
      const archive = {
        id: 'file-310',
        storage: 'local',
        node: 'pve-edge',
        instance: 'homelab',
        type: 'ct',
        vmid: 310,
        time: '2026-09-20T02:00:00Z',
        ctime: Date.parse('2026-09-20T02:00:00Z') / 1000,
        size: 1048576,
        format: 'tar.zst',
        protected: false,
        volid: 'local:backup/vzdump-lxc-310-2026_09_20-02_00_00.tar.zst',
        isPBS: false,
        verified: false,
      };
      const snapshot = {
        id: 'snap-310',
        name: 'fresh-local-snapshot',
        node: 'pve-edge',
        instance: 'homelab',
        type: 'ct',
        vmid: 310,
        time: '2026-10-03T11:30:00Z',
        vmstate: false,
      };
      page.on('pageerror', (error) => errors.push(error.message));
      await page.route('**/*', (route) => {
        const request = route.request(),
          url = new URL(request.url());
        if (url.origin !== 'http://127.0.0.1:5295') {
          offOrigin.push(url.origin);
          return route.abort();
        }
        if (url.pathname === '/fixture/workloads') return route.fulfill({ json: [workload] });
        if (!url.pathname.startsWith('/api/')) return route.continue();
        requests.push({ path: url.pathname, method: request.method() });
        assert.equal(request.method(), 'GET');
        if (url.pathname === '/api/backups/pve')
          return route.fulfill({
            json: {
              data: { backupTasks: [], storageBackups: [archive], guestSnapshots: [snapshot] },
            },
          });
        if (url.pathname === '/api/backups/pbs')
          return route.fulfill({ json: { data: { backups: [] } } });
        if (url.pathname === '/api/recovery/postures')
          return route.fulfill({
            json: {
              data: [],
              policy: {
                freshnessWindowSeconds: 0,
                verificationWindowSeconds: 0,
                requireVerification: false,
              },
              meta: { page: 1, limit: 200, total: 0, totalPages: 0 },
            },
          });
        return route.fulfill({
          json:
            url.pathname === '/api/license/runtime-capabilities'
              ? {
                  capabilities: [],
                  limits: [],
                  max_history_days: 7,
                  hosted_mode: false,
                  runtime: { build: 'community' },
                  blocked_capabilities: [],
                }
              : { data: [], enabled: false },
        });
      });
      try {
        await page.goto('http://127.0.0.1:5295/browser-tests/backups.html', {
          waitUntil: 'domcontentloaded',
          timeout: 120000,
        });
        if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
        const capture = async (state) => {
          await page.waitForTimeout(100);
          const dimensions = await page.evaluate(() => ({
            width: innerWidth,
            scroll: document.documentElement.scrollWidth,
          }));
          assert.equal(
            dimensions.width,
            phone ? 390 : 1365,
            'actual CSS viewport, not only Playwright emulation',
          );
          assert.ok(dimensions.scroll <= dimensions.width + 1, JSON.stringify(dimensions));
          const file = `${name}-${state}.png`;
          await page.screenshot({ path: path.join(output, file), fullPage: true });
          screenshots.push({ file, dimensions });
        };
        await page.getByRole('link', { name: 'By date', exact: true }).click();
        const byDate = page.locator('[data-proxmox-backups-table="recoverable"]');
        await byDate.getByText('artifact-cache-310', { exact: true }).first().waitFor();
        await capture('by-date-diagnostic');
        fs.writeFileSync(
          path.join(output, `${name}-by-date-diagnostic.json`),
          JSON.stringify({ text: await byDate.innerText(), requests, errors }, null, 2),
        );
        if (!phone) {
          await byDate.getByText('tar.zst', { exact: true }).waitFor();
          assert.ok(await byDate.locator(`[title="${archive.volid}"]`).count());
        }
        checks.push(
          'Explicit by-date navigation retains source, archive format and complete wide identifier',
        );
        await capture('by-date');
        await page.getByRole('link', { name: 'Coverage', exact: true }).click();
        const coverage = page.locator('[data-proxmox-backups-table="coverage"]');
        await coverage.getByText('artifact-cache-310', { exact: true }).waitFor();
        const columns = await coverage
          .locator('colgroup col')
          .evaluateAll((elements) => elements.map((e) => e.dataset.proxmoxBackupsColumn));
        const row = coverage.locator('tr[data-proxmox-backup-row="coverage"]').first();
        const backupAge = await row.locator('td').nth(columns.indexOf('latest')).innerText();
        assert.ok(backupAge.includes('13d'), backupAge);
        assert.ok(!backupAge.includes('30m'), backupAge);
        if (columns.includes('snapshot'))
          assert.ok(
            (await row.locator('td').nth(columns.indexOf('snapshot')).innerText()).includes('30m'),
          );
        checks.push('Fresh local guest snapshot does not masquerade as a recent backup');
        await capture('coverage');
        const search = page.getByPlaceholder('Search backups by workload, node, source, or status');
        await search.fill('non-existent-fixture');
        await page
          .getByText('No workload coverage rows match current filters', { exact: true })
          .waitFor();
        await search.fill('artifact-cache');
        await coverage.getByText('artifact-cache-310', { exact: true }).waitFor();
        checks.push('Coverage route query filters and recovery preserve the target');
        if (phone && !(await page.getByRole('combobox', { name: 'Filter', exact: true }).count()))
          await page.getByRole('button', { name: /^Filters/ }).click();
        await page
          .getByRole('combobox', { name: 'Filter', exact: true })
          .selectOption({ label: 'Backup location: homelab / pve-edge' });
        await page.waitForFunction(
          () => new URL(location.href).searchParams.get('location') === 'snapshot:homelab:pve-edge',
        );
        const scopedColumns = await coverage
          .locator('colgroup col')
          .evaluateAll((elements) => elements.map((e) => e.dataset.proxmoxBackupsColumn));
        const scopedAge = row.locator('td').nth(scopedColumns.indexOf('latest'));
        await scopedAge.getByText('None', { exact: true }).waitFor();
        assert.ok(
          (await scopedAge.locator('span[title]').getAttribute('title')).includes(
            'A snapshot is not a separate backup, so it does not count.',
          ),
        );
        checks.push(
          'Snapshot-only location scope reports no independent backup, not a fresh restore promise',
        );
        await capture('snapshot-only-location');
        if (
          phone &&
          !(await page
            .getByRole('button', { name: 'Remove Backup location filter', exact: true })
            .count())
        )
          await page.getByRole('button', { name: /^Filters/ }).click();
        await page
          .getByRole('button', { name: 'Remove Backup location filter', exact: true })
          .click();
        await page.waitForFunction(() => !new URL(location.href).searchParams.has('location'));
        await row
          .locator('td')
          .nth(columns.indexOf('latest'))
          .getByText(phone ? '13d' : '13d ago', { exact: true })
          .waitFor();
        checks.push('Removing location scope restores the independently dated archive');
        assert.deepEqual(errors, []);
        assert.deepEqual(offOrigin, []);
        assert.ok(
          requests.some((r) => r.path === '/api/backups/pve') &&
            requests.some((r) => r.path === '/api/backups/pbs'),
        );
        observations.push({
          name,
          browser: browser.version(),
          viewport: page.viewportSize(),
          checks,
          requests,
          errors,
          offOrigin,
          screenshots,
        });
      } catch (error) {
        fs.writeFileSync(
          path.join(output, `${name}-failure.json`),
          JSON.stringify(
            {
              error: error.message,
              text: await page.locator('body').innerText(),
              url: page.url(),
              requests,
              errors,
              offOrigin,
            },
            null,
            2,
          ),
        );
        await page.screenshot({ path: path.join(output, `${name}-failure.png`), fullPage: true });
        throw error;
      }
      await browser.close();
      browser = null;
    }
    fs.writeFileSync(
      path.join(output, 'result.json'),
      JSON.stringify(
        {
          passed: true,
          playwright,
          content_sha256,
          harness_sha256: crypto
            .createHash('sha256')
            .update(fs.readFileSync(__filename))
            .digest('hex'),
          fixture_sha256: Object.fromEntries(
            ['backups.html', 'backups.tsx'].map((file) => [
              file,
              crypto
                .createHash('sha256')
                .update(fs.readFileSync(path.join(root, 'browser-tests', file)))
                .digest('hex'),
            ]),
          ),
          observations,
          limitation:
            'Synthetic production component rendering, not native backup, QGA/thaw, installation or shipment.',
        },
        null,
        2,
      ),
    );
    console.log(
      JSON.stringify({
        passed: true,
        browsers: observations.length,
        checks: observations.reduce((n, o) => n + o.checks.length, 0),
      }),
    );
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
