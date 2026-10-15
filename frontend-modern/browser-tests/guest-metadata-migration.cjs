const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/guest-metadata-migration-proof';
const input = '/workspace/tmp/guest-metadata-migration-projections.json';
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const binding = JSON.parse(fs.readFileSync(input));
  assert.match(binding.source, /^[a-f0-9]{40}$/);
  assert.match(binding.receipt, /^source-/);
  for (const phase of ['suppressed', 'migrated', 'refreshed', 'returned']) {
    const resources = binding.projections[phase];
    assert.equal(resources.length, 1);
    assert.equal(resources[0].proxmox.osName, 'OpenBSD');
    assert.equal(
      resources[0].proxmox.nodeName,
      phase === 'migrated' || phase === 'refreshed' ? 'node-b' : 'node-a',
    );
  }
  // The source suite may finish long after it emitted these projections. Keep
  // this migration proof at their observed time, not an unrelated expiry test.
  const renderTime =
    Math.max(
      ...Object.values(binding.projections)
        .flat()
        .map((resource) => new Date(resource.lastSeen).getTime()),
    ) + 1000;
  assert.ok(Number.isFinite(renderTime));
  const result = {
    source: binding.source,
    browser_source: binding.browser_source,
    source_receipt: binding.receipt,
    input_sha256: hash(input),
    render_time: new Date(renderTime).toISOString(),
    playwright: require('playwright/package.json').version,
    scope:
      'Actual monitor HTTP/poller projections through production Workloads adapter, row, drawer and CSS. QGA version is asserted in the producer projection; this existing Workloads adapter does not render it. Not full App, native migration, thaw, writes/liveness, installed or release acceptance.',
    sources: {},
    cases: [],
    screenshots: [],
  };
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  for (const file of [
    'internal/monitoring/guest_metadata.go',
    'internal/monitoring/guest_metadata_migration_test.go',
    'frontend-modern/browser-tests/guest-metadata-migration.tsx',
    'frontend-modern/src/hooks/useWorkloads.ts',
    'frontend-modern/src/utils/resourceStateAdapters.ts',
    'frontend-modern/src/components/Workloads/GuestRow.tsx',
    'frontend-modern/src/components/Workloads/GuestDrawer.tsx',
    'frontend-modern/src/components/Workloads/GuestDrawerOverview.tsx',
    'frontend-modern/src/index.css',
    'frontend-modern/package-lock.json',
  ])
    result.sources[file] = hash(path.join('/workspace', file));
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(output, 'vite-cache'),
    optimizeDeps: { entries: ['browser-tests/guest-metadata-migration.html'] },
    server: { host: '127.0.0.1', port: 5297, strictPort: true, watch: null },
  });
  const origin = 'http://127.0.0.1:5297';
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['desktop', chromium, 1365, false],
      ['narrow', webkit, 390, true],
    ]) {
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
      page.setDefaultTimeout(12000);
      page.setDefaultNavigationTimeout(60000);
      await page.clock.install({ time: new Date(renderTime) });
      const record = {
        name,
        width,
        dark,
        browser: browser.version(),
        errors: [],
        requests: [],
        checks: [],
      };
      result.cases.push(record);
      page.on('pageerror', (error) => record.errors.push(error.message));
      await page.addInitScript((isDark) => {
        document.addEventListener(
          'DOMContentLoaded',
          () => document.documentElement.classList.toggle('dark', isDark),
          { once: true },
        );
      }, dark);
      await page.routeWebSocket(/.*/, (ws) => ws.close());
      await page.route('**/*', async (route) => {
        const request = route.request(),
          url = new URL(request.url());
        assert.equal(url.origin, origin, 'Unexpected external request');
        if (url.pathname === '/__fixture/guest-metadata-migration.json')
          return route.fulfill({ json: binding.projections });
        if (!url.pathname.startsWith('/api/')) return route.continue();
        record.requests.push({ path: url.pathname, method: request.method() });
        assert.equal(request.method(), 'GET');
        if (url.pathname.includes('metrics-store/history'))
          return route.fulfill({
            json: {
              resourceType: 'vm',
              resourceId: url.searchParams.get('resourceId'),
              range: '24h',
              start: Date.now() - 3600000,
              end: Date.now(),
              metrics: {},
              source: 'store',
            },
          });
        if (url.pathname.includes('license'))
          return route.fulfill({
            json: {
              capabilities: [],
              limits: [],
              max_history_days: 90,
              runtime: { build: 'community' },
            },
          });
        if (url.pathname.includes('metadata')) return route.fulfill({ json: {} });
        return route.fulfill({
          json: { success: true, data: [], alerts: [], anomalies: [], config: null },
        });
      });
      await page.goto(origin + '/browser-tests/guest-metadata-migration.html');
      await page.waitForFunction(() => window.__guestMetadataMigration?.guest());
      const click = (locator) => (width < 500 ? locator.tap() : locator.click());
      const row = page.locator('[data-guest-id]');
      if (width < 500) await click(row.locator('[data-workload-col="disk"]'));
      else {
        await row.locator('[data-row-action]').focus();
        await page.keyboard.press('Enter');
      }
      const drawer = page.getByRole('region', { name: 'Guest details' });
      await drawer.waitFor({ state: 'visible' });
      for (const [phase, label, ip, node, version] of [
        ['suppressed', 'Before migration', '192.0.2.20', 'node-a', '2.0'],
        ['migrated', 'After migration', '192.0.2.20', 'node-b', '2.0'],
        ['refreshed', 'Refresh network/version', '192.0.2.21', 'node-b', '2.1'],
        ['returned', 'Return migration', '192.0.2.21', 'node-a', '2.1'],
      ]) {
        if (phase !== 'suppressed')
          await click(page.getByRole('button', { name: label, exact: true }));
        await page.waitForFunction(
          (value) => window.__guestMetadataMigration.phase() === value,
          phase,
        );
        await drawer
          .getByTestId('guest-drawer-facts')
          .getByText('OpenBSD 7.8', { exact: true })
          .waitFor();
        await drawer.getByTestId('guest-drawer-facts').getByText(ip, { exact: true }).waitFor();
        const model = await page.evaluate(() => {
          const guest = window.__guestMetadataMigration.guest();
          return {
            node: guest.node,
            osName: guest.osName,
            osVersion: guest.osVersion,
            qgaVersion: window.__guestMetadataMigration.snapshot()[0].proxmox.agentVersion,
            ips: guest.ipAddresses,
            disk: guest.disk,
          };
        });
        assert.equal(model.node, node);
        assert.equal(model.osName, 'OpenBSD');
        assert.equal(model.osVersion, '7.8');
        assert.equal(model.qgaVersion, version);
        assert.deepEqual(model.ips, [ip]);
        assert.equal(model.disk.used, 314572800);
        const details = drawer.getByTestId('guest-technical-details');
        if (!(await details.evaluate((element) => element.open)))
          await click(details.locator('summary'));
        await row.getByTitle(node, { exact: true }).waitFor();
        await drawer
          .getByTestId('guest-technical-details-sections')
          .getByText('vio0', { exact: false })
          .first()
          .waitFor();
        assert.equal(
          await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1),
          false,
          'Horizontal viewport overflow',
        );
        record.checks.push({
          phase,
          model,
          actualDetails: await drawer.getByTestId('guest-technical-details-sections').innerText(),
        });
        await page
          .getByRole('heading', { name: 'Guest metadata across migration' })
          .scrollIntoViewIfNeeded();
        const file = `${name}-${phase}.png`;
        await page.screenshot({ path: path.join(output, file), fullPage: true });
        result.screenshots.push({ file, phase, name, sha256: hash(path.join(output, file)) });
      }
      assert.deepEqual(record.errors, []);
      await click(drawer.getByRole('button', { name: /^Collapse .* details$/ }).first());
      await drawer.waitFor({ state: 'hidden' });
      record.checks.push('Drawer closes by the real control after migration and refresh.');
      await browser.close();
      browser = undefined;
    }
    result.result = 'passed';
  } catch (error) {
    result.result = 'failed';
    result.error = String(error.stack || error);
    throw error;
  } finally {
    fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify(result, null, 2) + '\n');
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
