const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const workspace = process.env.PULSE_BROWSER_PROOF_ROOT || '/workspace';
const root = path.join(workspace, 'frontend-modern');
const origin = 'http://127.0.0.1:5343';
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const runtime = [
  'frontend-modern/src/components/Infrastructure/resourceDetailDrawerMetricsHistoryModel.ts',
  'frontend-modern/src/components/Infrastructure/useResourceDetailDrawerDerivedState.ts',
  'frontend-modern/src/components/Infrastructure/ResourceDetailDrawer.tsx',
  'frontend-modern/src/components/Workloads/GuestDrawerHistory.tsx',
  'frontend-modern/src/utils/workloadGuestPresentation.ts',
  'frontend-modern/src/utils/memoryObservation.ts',
];
const settle = (page) =>
  page.evaluate(async () => {
    await document.fonts.ready;
    await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  });

async function verify(original = false) {
  const output = path.join(
    workspace,
    `tmp/resource-stopped-filesystem/browser-${original ? 'original' : 'final'}`,
  );
  fs.mkdirSync(output, { recursive: true });
  const result = {
    result: 'incomplete',
    phase: original ? 'original-adverse-observation' : 'candidate-verification',
    playwright: require('playwright/package.json').version,
    runtime_hashes: Object.fromEntries(
      runtime.map((file) => [file, hash(path.join(workspace, file))]),
    ),
    cases: [],
    screenshots: [],
    cleanup: { browsers: false, server: false },
    limits:
      'Production ResourceDetailDrawer, History/client and CSS; synthetic snapshots and local endpoints. No native guest command, QGA, thaw, writes, restart, reporter, installed or release acceptance.',
  };
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync(path.join(workspace, 'tests/integration/package-lock.json')))
      .packages['node_modules/@playwright/test'].version,
  );
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: output + '/cache',
    optimizeDeps: { entries: ['browser-tests/resource-stopped-filesystem.html'] },
    server: { host: '127.0.0.1', port: 5343, strictPort: true },
  });
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, height, dark] of [
      ['chromium-desktop-light', chromium, 1280, 900, false],
      ['chromium-tablet-light', chromium, 1024, 768, false],
      ['webkit-phone-dark', webkit, 390, 844, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height },
        isMobile: width < 768,
        hasTouch: width < 768,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      page.setDefaultTimeout(12000);
      const record = {
        name,
        width,
        height,
        dark,
        browser: browser.version(),
        errors: [],
        requests: [],
        checks: [],
        states: [],
      };
      result.cases.push(record);
      let mode = 'empty';
      page.on('pageerror', (error) => record.errors.push(error.message));
      await page.route('**/*', (route) => {
        const request = route.request(),
          url = new URL(request.url());
        if (url.origin !== origin) {
          record.requests.push({ path: url.pathname, method: request.method(), blocked: true });
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        assert.equal(request.method(), 'GET');
        record.requests.push({ path: url.pathname, method: request.method(), mode });
        if (url.pathname === '/api/metrics-store/history') {
          assert.equal(url.searchParams.get('resourceId'), 'fixture:pve:101');
          const end = Date.now() - 60000,
            start = end - 3600000;
          const point = (timestamp, value) => ({ timestamp, value, min: value, max: value });
          return route.fulfill({
            json: {
              resourceType: url.searchParams.get('resourceType'),
              resourceId: 'fixture:pve:101',
              range: url.searchParams.get('range'),
              start,
              end,
              source: 'store',
              metrics: mode === 'stored' ? { disk: [point(start, 25), point(end, 50)] } : {},
            },
          });
        }
        return route.fulfill({
          json:
            url.pathname === '/api/license/runtime-capabilities'
              ? {
                  capabilities: [],
                  limits: [],
                  max_history_days: 90,
                  hosted_mode: false,
                  runtime: { build: 'community' },
                  blocked_capabilities: [],
                }
              : {
                  success: true,
                  data: [],
                  alerts: [],
                  anomalies: [],
                  config: null,
                  capabilities: [],
                  relationships: [],
                  recentChanges: [],
                  available: false,
                  audits: [],
                  count: 0,
                },
        });
      });
      await page.addInitScript((isDark) => {
        document.addEventListener('DOMContentLoaded', () =>
          document.documentElement.classList.toggle('dark', isDark),
        );
        window.__trustedInputs = [];
        for (const type of ['click', 'keydown'])
          document.addEventListener(
            type,
            (event) => window.__trustedInputs.push({ type, trusted: event.isTrusted }),
            true,
          );
      }, dark);
      // Charts poll every 30 seconds; the proof drives that poll with the clock.
      await page.clock.install();
      await page.goto(origin + '/browser-tests/resource-stopped-filesystem.html', {
        waitUntil: 'domcontentloaded',
        timeout: 60000,
      });
      await page.waitForFunction(() => window.__resourceStoppedFilesystem, null, {
        timeout: 45000,
      });
      // History charts sit on Overview, which the drawer opens on.
      const chart = page.locator('[data-history-group="utilization"]');
      await chart.waitFor();
      await settle(page);
      const apply = async (value) => {
        await page.evaluate((value) => window.__resourceStoppedFilesystem.apply(value), value);
        await settle(page);
      };
      const reads = () =>
        record.requests.filter((request) => request.path === '/api/metrics-store/history').length;
      const firstReads = reads();
      const capture = async (state) => {
        await chart.scrollIntoViewIfNeeded();
        await settle(page);
        const file = `${name}-${state}.png`;
        await chart.screenshot({ path: path.join(output, file) });
        result.screenshots.push({ file, sha256: hash(path.join(output, file)), state, name });
      };
      const unavailable = async () => {
        assert.equal(
          await chart
            .locator('[data-history-current="disk"], [data-history-last-known="disk"]')
            .count(),
          0,
        );
        assert.match(
          await chart.locator('[data-history-deferred="disk"]').innerText(),
          /while the VM is stopped/,
        );
        assert.doesNotMatch(await chart.innerText(), /90\.0%/);
      };
      if (original) {
        assert.match(
          await chart.locator('[data-history-current="disk"]').innerText(),
          /90\.0%\s*current/,
        );
        assert.equal(await chart.locator('[data-history-deferred="disk"]').count(), 0);
        record.states.push({ state: 'native-stopped-no-reason', text: await chart.innerText() });
        await capture('stopped-current-carrier');
        await apply({ reason: 'prev-agent-busy' });
        assert.match(
          await chart.locator('[data-history-last-known="disk"]').innerText(),
          /90\.0%\s*last known/,
        );
        assert.doesNotMatch(
          await chart.locator('[data-history-deferred="disk"]').innerText(),
          /VM is stopped/,
        );
        record.states.push({
          state: 'native-stopped-retained-reason',
          text: await chart.innerText(),
        });
        await capture('stopped-retained-carrier');
        record.checks.push(
          'Native stopped state is ignored with both an empty reason and a retained reason in the original canonical History.',
        );
      } else {
        await unavailable();
        assert.match(
          await chart.locator('[data-history-current="cpu"]').innerText(),
          /10\.0%\s*current/,
        );
        assert.match(
          await chart.locator('[data-history-current="memory"]').innerText(),
          /0\.0%\s*current/,
        );
        assert.equal(await chart.locator('path, [data-history-observation]').count(), 0);
        await capture('stopped');
        record.checks.push(
          'Native stopped VM withdraws current and retained disk carriers, even when a linked source holds canonical status online; independent CPU/memory are unchanged.',
        );
        await apply({ reason: 'prev-agent-busy' });
        await unavailable();
        await apply({ power: ' STOPPED ', reason: '', seen: Date.now() + 3600000 });
        await unavailable();
        await apply({ power: undefined, status: 'offline' });
        await unavailable();
        record.checks.push(
          'Stopped native state overrides a prev reason; case/space normalization and legacy offline fallback stay unavailable without a new History read.',
        );
        await apply({ power: ' RUNNING ', status: 'offline', reason: '', usage: 0 });
        assert.match(
          await chart.locator('[data-history-current="disk"]').innerText(),
          /0\.0%\s*current/,
        );
        assert.equal(await chart.locator('[data-history-deferred="disk"]').count(), 0);
        await capture('current-zero');
        record.checks.push(
          "Explicit native running state wins over another source's offline status, and an accepted zero renders in the existing mounted chart.",
        );
        await apply({
          power: 'running',
          status: 'online',
          reason: 'prev-agent-capacity',
          usage: 90,
        });
        assert.match(
          await chart.locator('[data-history-last-known="disk"]').innerText(),
          /90\.0%\s*last known/,
        );
        assert.match(
          await chart.locator('[data-history-deferred="disk"]').innerText(),
          /guest-read capacity/,
        );
        await capture('retained');
        await apply({ reason: 'agent-capacity' });
        assert.equal(
          await chart
            .locator('[data-history-last-known="disk"], [data-history-current="disk"]')
            .count(),
          0,
        );
        assert.match(
          await chart.locator('[data-history-deferred="disk"]').innerText(),
          /guest-read capacity/,
        );
        record.checks.push(
          'Running retained/unavailable read reasons keep their distinct existing semantics, not inferred recovery.',
        );
        await apply({ power: 'stopped', reason: '', platform: 'agent', scopes: ['proxmox-pve'] });
        await unavailable();
        await apply({
          platform: 'vmware-vsphere',
          scopes: undefined,
          reason: 'prev-agent-capacity',
          usage: 30,
        });
        assert.match(
          await chart.locator('[data-history-current="disk"]').innerText(),
          /30\.0%\s*current/,
        );
        assert.equal(await chart.locator('[data-history-deferred="disk"]').count(), 0);
        assert.equal(reads(), firstReads);
        record.checks.push(
          'Proxmox-scoped hybrid VM honours native power; a vSphere-only VM does not inherit Proxmox reasons, and snapshot changes send no additional History reads.',
        );
        await apply({
          type: 'system-container',
          platform: 'proxmox-pve',
          power: 'stopped',
          status: 'offline',
        });
        assert.match(
          await chart.locator('[data-history-current="disk"]').innerText(),
          /30\.0%\s*current/,
        );
        assert.equal(await chart.locator('[data-history-deferred="disk"]').count(), 0);
        record.checks.push(
          'Independent LXC platform storage remains current despite stopped guest runtime and a VM-only read reason.',
        );
        await apply({ type: 'vm', power: 'stopped', status: 'online', reason: '', usage: 90 });
        await unavailable();
        // Selected memory freshness belongs to its source, independently of guest type.
        for (const type of ['vm', 'system-container']) {
          await apply({
            type,
            power: 'running',
            status: 'online',
            memorySource: 'agent',
            memoryState: 'current',
            memoryUsage: 0,
            agentStale: false,
          });
          assert.match(
            await chart.locator('[data-history-current="memory"]').innerText(),
            /0\.0%\s*current/,
          );
          const memoryReads = reads();
          await apply({ agentStale: true });
          assert.equal(await chart.locator('[data-history-current="memory"]').count(), 0);
          assert.match(
            await chart.locator('[data-history-last-known="memory"]').innerText(),
            /0\.0%/,
          );
          assert.match(
            await chart.locator('[data-history-deferred="memory"]').innerText(),
            /Last known/,
          );
          assert.match(
            await chart.locator('[data-history-deferred="memory"]').innerText(),
            /2026-10-10 00:00:00 UTC/,
          );
          record.states.push({ state: `${type}-agent-stale`, text: await chart.innerText() });
          await capture(`${type}-agent-stale`);
          await apply({ memorySource: 'cluster-resources', memoryUsage: 95 });
          assert.match(
            await chart.locator('[data-history-current="memory"]').innerText(),
            /95\.0%\s*current/,
          );
          await capture(`${type}-provider-current`);
          await apply({ memoryState: 'last-known' });
          const memoryText = await chart.locator('[data-history-deferred="memory"]').innerText();
          if (type === 'vm') assert.match(memoryText, /may include cache/i);
          else assert.doesNotMatch(memoryText, /may include cache/i);
          await capture(`${type}-provider-retained`);
          await apply({ memorySource: 'agent', memoryState: 'unknown', agentStale: false });
          assert.equal(await chart.locator('[data-history-current="memory"]').count(), 0);
          assert.match(
            await chart.locator('[data-history-deferred="memory"]').innerText(),
            /Freshness unknown/,
          );
          await apply({ memoryState: 'current' });
          assert.equal(reads(), memoryReads);
        }
        record.checks.push(
          'A stale selected agent retains its original memory observation for VM and LXC; independent Proxmox memory stays current, with the cache caveat only for VMs.',
        );
        await apply({
          type: 'vm',
          power: 'stopped',
          memorySource: 'agent',
          memoryUsage: 0,
          agentStale: false,
        });
        mode = 'stored';
        const read = page.waitForResponse((r) => r.url().includes('/api/metrics-store/history'));
        await page.clock.runFor(30_000);
        await read;
        const slider = chart.getByRole('slider', { name: 'Inspect Utilization history' });
        await slider.waitFor();
        assert.equal(await chart.locator('path[data-history-series]').count(), 1);
        await unavailable();
        await slider.focus();
        await page.keyboard.press('Home');
        assert.match(await slider.getAttribute('aria-valuetext'), /Disk 25\.0%/);
        await page.keyboard.press('End');
        assert.match(await slider.getAttribute('aria-valuetext'), /Disk 50\.0%/);
        await capture('stored-stopped');
        record.checks.push(
          'Stopping withdraws only live carriers, not genuine stored history; trusted keyboard Home/End inspection preserves both disk observations.',
        );
      }
      record.layout = await page.evaluate(() => ({
        width: document.documentElement.clientWidth,
        scrollWidth: document.documentElement.scrollWidth,
      }));
      assert.ok(
        record.layout.scrollWidth <= record.layout.width + 1,
        JSON.stringify(record.layout),
      );
      assert.ok((await page.evaluate(() => window.__trustedInputs)).some((event) => event.trusted));
      assert.deepEqual(record.errors, []);
      assert.equal(
        record.requests.filter((request) => request.blocked || request.method !== 'GET').length,
        0,
      );
      await browser.close();
      browser = null;
    }
    result.cleanup.browsers = true;
    result.result = 'passed';
  } catch (error) {
    result.result = 'failed';
    result.error = String(error);
    throw error;
  } finally {
    if (browser) {
      await browser.close();
      result.cleanup.browsers = true;
    }
    await server.close();
    result.cleanup.server = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    process.stdout.write(
      JSON.stringify({
        result: result.result,
        cases: result.cases.length,
        checks: result.cases.reduce((n, c) => n + c.checks.length, 0),
        cleanup: result.cleanup,
      }) + '\n',
    );
  }
}

module.exports = { verify };
if (require.main === module)
  verify().catch((error) => {
    process.stderr.write(String(error) + '\n');
    process.exitCode = 1;
  });
