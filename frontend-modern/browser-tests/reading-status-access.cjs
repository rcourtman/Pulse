const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const http = require('node:http');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/reading-details/browser');
const origin = 'http://127.0.0.1:5296';
const time = '2026-10-12T20:00:00Z';
const runtime = [
  'frontend-modern/src/components/Workloads/GuestRow.tsx',
  'frontend-modern/src/components/Workloads/WorkloadReadingStatus.tsx',
];
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const result = {
  base: 'a2e5d0fb93d69abe9eabc682126a64d12443594c',
  sourceHashes: Object.fromEntries(
    runtime.map((p) => [p, hash(fs.readFileSync(path.join(root, p)))]),
  ),
  playwright: require('playwright/package.json').version,
  scope:
    'Real canonical Resources API and useWorkloads producer, actual production GuestRow, reading disclosure, shared Dialog and CSS. Synthetic read state; no guest command, native recovery, publication or availability claim.',
  checks: [],
  requests: [],
  captures: [],
  loaded: {},
  browsers: [],
  errors: [],
  consoleErrors: [],
  cleanup: {},
};
const initial = () => ({
  reason: 'prev-agent-timeout',
  state: 'last-known',
  source: 'guest-agent-meminfo',
  observedAt: time,
  percent: 55,
  instance: 'fixture-pve',
});
const resource = (read, control = false) => {
  const total = 10 * 1024 ** 3;
  const vmid = control ? 102 : 101;
  const percent = control ? 25 : read.percent;
  const observation = control
    ? { state: 'current', source: 'guest-agent-meminfo', observedAt: time }
    : {
        state: read.state,
        source: read.source,
        ...(read.observedAt ? { observedAt: read.observedAt } : {}),
      };
  const reason = control ? undefined : read.reason;
  return {
    id: `${control ? 'fixture-pve' : read.instance}-pve-a-${vmid}`,
    type: 'vm',
    name: control ? 'fresh-control' : 'reading-guest',
    status: 'online',
    sources: ['proxmox'],
    lastSeen: '2026-10-12T20:30:00Z',
    metrics: {
      cpu: { percent: 10 },
      memory: { percent, used: (total * percent) / 100, total, observation },
      disk: { percent: 50, used: total / 2, total },
    },
    proxmox: {
      vmid,
      instance: control ? 'fixture-pve' : read.instance,
      nodeName: 'pve-a',
      runtimeStatus: 'running',
      guestAgentStatus: reason ? 'deferred' : 'available',
      ...(reason ? { diskStatusReason: reason } : {}),
      disks: [{ mountpoint: '/data', filesystem: 'ext4', total, used: total / 2 }],
    },
  };
};
let server;
const staticRoot = path.join(root, 'tmp/reading-status-static');
const binding = JSON.parse(fs.readFileSync(path.join(staticRoot, 'final/binding.json')));
result.buildBindings = { final: binding };
for (const [p, digest] of Object.entries(binding.sources)) {
  const local = path.join(frontend, p);
  assert.equal(hash(fs.readFileSync(local)), digest);
  if (p.startsWith('src/')) result.loaded['frontend-modern/' + p] = digest;
}
(async () => {
  fs.mkdirSync(output, { recursive: true });
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  server = http.createServer((request, response) => {
    const pathname = decodeURIComponent(new URL(request.url, origin).pathname);
    const local = path.resolve(staticRoot, '.' + pathname);
    if (
      !local.startsWith(staticRoot + '/') ||
      !fs.existsSync(local) ||
      !fs.statSync(local).isFile()
    ) {
      response.writeHead(404);
      response.end();
      return;
    }
    const extension = path.extname(local);
    response.writeHead(200, {
      'Content-Type':
        extension === '.html'
          ? 'text/html'
          : extension === '.js'
            ? 'text/javascript'
            : extension === '.css'
              ? 'text/css'
              : 'application/octet-stream',
    });
    response.end(fs.readFileSync(local));
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(5296, '127.0.0.1', resolve);
  });
  for (const spec of [
    {
      name: 'desktop',
      engine: chromium,
      width: 1365,
      height: 900,
      colorScheme: 'light',
      locale: 'en-GB',
      timezoneId: 'Europe/London',
    },
    {
      name: 'phone',
      engine: webkit,
      width: 390,
      height: 844,
      colorScheme: 'dark',
      locale: 'en-GB',
      timezoneId: 'Asia/Tokyo',
    },
  ]) {
    const browser = await spec.engine.launch(
      spec.engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    const context = await browser.newContext({
      viewport: { width: spec.width, height: spec.height },
      isMobile: spec.width < 768,
      hasTouch: spec.width < 768,
      colorScheme: spec.colorScheme,
      locale: spec.locale,
      timezoneId: spec.timezoneId,
    });
    result.browsers.push({
      name: spec.name,
      version: browser.version(),
      width: spec.width,
      height: spec.height,
      locale: spec.locale,
      timezoneId: spec.timezoneId,
    });
    try {
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      page.on('pageerror', (error) =>
        result.errors.push({ browser: spec.name, message: error.message }),
      );
      page.on('console', (entry) => {
        if (entry.type() === 'error')
          result.consoleErrors.push({ browser: spec.name, message: entry.text() });
      });
      await page.clock.setFixedTime(new Date('2026-10-12T20:30:00Z'));
      let read = initial();
      await page.route('**/*', async (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== origin) {
          result.errors.push({ external: url.origin });
          return route.abort();
        }
        if (url.pathname.startsWith('/api/')) {
          const record = {
            browser: spec.name,
            path: url.pathname,
            method: route.request().method(),
          };
          result.requests.push(record);
          assert.equal(record.method, 'GET');
          let body;
          if (url.pathname === '/api/resources')
            body = { data: [resource(read), resource(read, true)], meta: { totalPages: 1 } };
          else if (url.pathname === '/api/alerts/activation') body = { enabled: false };
          else if (url.pathname === '/api/alerts/config')
            body = { guestDefaults: {}, hostDefaults: {} };
          else if (url.pathname === '/api/alerts/active') body = [];
          else if (url.pathname === '/api/ai/intelligence/anomalies')
            body = { anomalies: [], count: 0 };
          else if (url.pathname === '/api/ai/intelligence') body = null;
          else if (url.pathname === '/api/metrics-store/history')
            body = {
              resourceType: url.searchParams.get('resourceType'),
              resourceId: url.searchParams.get('resourceId'),
              range: url.searchParams.get('range'),
              start: Date.parse(time),
              end: Date.parse(time) + 1800000,
              metrics: {},
              source: 'store',
            };
          else if (url.pathname.startsWith('/api/guests/metadata/')) body = { customUrl: '' };
          else if (
            url.pathname.startsWith('/api/resources/') &&
            url.pathname.endsWith('/operator-state')
          )
            body = { configured: false, state: null };
          else {
            record.unmatched = true;
            result.errors.push({ unmatched: url.pathname });
            body = {};
          }
          return route.fulfill({ json: body });
        }
        const variant = 'final';
        const artifact = url.pathname.slice(variant.length + 2);
        if (['.js', '.css', '.html'].includes(path.extname(artifact))) {
          const digest = hash(fs.readFileSync(path.join(staticRoot, variant, artifact)));
          assert.equal(digest, result.buildBindings[variant].artifacts[artifact]);
        }
        if (url.pathname.startsWith('/src/')) {
          const local = path.join(frontend, decodeURIComponent(url.pathname));
          if (fs.existsSync(local))
            result.loaded[path.relative(root, local)] = hash(fs.readFileSync(local));
        }
        return route.continue();
      });
      await page.routeWebSocket(/\/ws(?:\?|$)/, () => {});
      await page.goto(`${origin}/final/browser-tests/reading-status-access.html`, {
        waitUntil: 'domcontentloaded',
        timeout: 30000,
      });
      if (spec.colorScheme === 'dark')
        await page.evaluate(() => document.documentElement.classList.add('dark'));
      const row = page.locator('tr.workload-row').first();
      await row.getByRole('button', { name: /^Disk reading for/ }).waitFor();
      await page.evaluate(() => {
        window.__originalReadingRow = document.querySelector('tr.workload-row');
      });
      const baselineHeight = await row.evaluate(
        (element) => element.getBoundingClientRect().height,
      );
      const check = (name, detail = {}) =>
        result.checks.push({ browser: spec.name, name, ...detail });
      const capture = async (name) => {
        await page.evaluate(async () => {
          await document.fonts.ready;
          await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
        });
        await page.waitForFunction(() => {
          const panel = document.querySelector('[role="dialog"]');
          const backdrop = document.querySelector('[data-dialog-backdrop]');
          return (
            panel &&
            backdrop &&
            getComputedStyle(panel).opacity === '1' &&
            getComputedStyle(backdrop).opacity === '1' &&
            panel.getAnimations().every((animation) => animation.playState === 'finished')
          );
        });
        const p = path.join(output, `${spec.name}-${name}.png`);
        await page.screenshot({ path: p, fullPage: true, animations: 'disabled' });
        result.captures.push({
          path: path.relative(root, p),
          bytes: fs.statSync(p).size,
          sha256: hash(fs.readFileSync(p)),
        });
      };
      const update = async (change) => {
        read = { ...read, ...change };
        await page.evaluate(async () => window.__readingStatusAccess.refetch());
        await page.evaluate(() => new Promise((r) => requestAnimationFrame(r)));
        assert.equal(
          await page.evaluate(
            () => window.__originalReadingRow === document.querySelector('tr.workload-row'),
          ),
          true,
        );
      };
      const trigger = (metric) =>
        row.getByRole('button', { name: new RegExp(`^${metric} reading for`) });
      const dialog = () => page.getByRole('dialog');
      const geometry = async (metric) => {
        const value = await trigger(metric).evaluate((element) => {
          const rect = element.getBoundingClientRect();
          const cell = element.closest('td').getBoundingClientRect();
          const label = element.querySelector('span').getBoundingClientRect();
          return {
            width: rect.width,
            height: rect.height,
            rowHeight: element.closest('tr').getBoundingClientRect().height,
            fits:
              rect.left >= cell.left - 1 &&
              rect.right <= cell.right + 1 &&
              rect.top >= cell.top - 1 &&
              rect.bottom <= cell.bottom + 1,
            textFits: label.left >= cell.left - 1 && label.right <= cell.right + 1,
          };
        });
        assert.ok(value.fits, `Reading target escapes its cell: ${JSON.stringify(value)}`);
        assert.ok(value.textFits, `Reading label clipped: ${JSON.stringify(value)}`);
        assert.ok(
          value.rowHeight <= baselineHeight + 0.5,
          `Dense row grew: ${JSON.stringify(value)}`,
        );
        if (spec.width < 768) {
          assert.ok(value.width >= 24);
          assert.ok(value.height >= 24);
        }
        return value;
      };
      const open = async (metric) => {
        const before = result.requests.length;
        const button = trigger(metric);
        const shape = await geometry(metric);
        if (spec.width < 768) await button.tap();
        else {
          await button.focus();
          await page.keyboard.press('Enter');
        }
        await dialog().waitFor();
        assert.equal(await button.getAttribute('aria-expanded'), 'true');
        assert.equal(await page.evaluate(() => window.__readingStatusAccess.actions()), 0);
        assert.equal(result.requests.length, before, 'Disclosure added an API request');
        assert.equal(await dialog().locator('button').count(), 1);
        assert.equal(
          await page.evaluate(() => document.activeElement?.getAttribute('aria-label')),
          'Close reading details',
        );
        assert.ok(
          await dialog().evaluate((element) => element.scrollWidth <= element.clientWidth + 1),
        );
        assert.ok(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1),
        );
        check(`${metric} native disclosure and dense target`, shape);
      };
      const close = async (metric, backdrop = false) => {
        if (backdrop)
          await page.locator('[data-dialog-backdrop]').click({ position: { x: 5, y: 5 } });
        else await page.keyboard.press('Escape');
        await dialog().waitFor({ state: 'detached' });
        assert.equal(await trigger(metric).getAttribute('aria-expanded'), 'false');
        assert.equal(
          await trigger(metric).evaluate((element) => element === document.activeElement),
          true,
        );
        assert.equal(await page.evaluate(() => window.__readingStatusAccess.actions()), 0);
      };
      await open('Disk');
      assert.match(
        await dialog().innerText(),
        /Completion is uncertain\. Do not restart the guest agent during a backup/,
      );
      await page.keyboard.press('Tab');
      assert.equal(
        await page.evaluate(() => document.activeElement?.getAttribute('aria-label')),
        'Close reading details',
      );
      await capture('disk-timeout');
      await close('Disk');
      await open('Memory');
      assert.match(
        await dialog().innerText(),
        /Last known\. Source: QEMU guest agent\. Observed: 2026-10-12 20:00:00 UTC\. Not a current measurement\./,
      );
      await capture('memory-prior');
      await close('Memory', true);
      const deferrals = [
        'vm-locked',
        'lock-unverified',
        'agent-busy',
        'agent-cooldown',
        'agent-response-incomplete',
        'agent-capacity',
        'invalid-guest-key',
        'agent-timeout',
      ];
      for (const mode of ['bars', 'sparklines']) {
        await page.evaluate((value) => window.__readingStatusAccess.mode(value), mode);
        await open('Disk');
        for (const reason of deferrals) {
          await update({ reason: `prev-${reason}` });
          assert.match(await dialog().innerText(), /Using last known disk stats\./);
          assert.equal(
            (await row.locator('[data-workload-col="disk"] [aria-hidden="true"]').count()) > 0,
            true,
          );
          check(`${mode}: same-guest open retained ${reason}`);
          await update({ reason });
          assert.doesNotMatch(await dialog().innerText(), /Using last known disk stats\./);
          assert.doesNotMatch(await row.locator('[data-workload-col="disk"]').innerText(), /50%/);
          check(`${mode}: same-guest open unavailable ${reason}`);
        }
        await update({ reason: 'permission-denied' });
        assert.match(await dialog().innerText(), /Do not widen intentionally scoped access/);
        if (mode === 'sparklines') await capture('disk-permission');
        await close('Disk');
      }
      await update({ state: 'current', source: 'status-mem' });
      await open('Memory');
      assert.match(await dialog().innerText(), /Source: Proxmox/);
      assert.match(
        await dialog().innerText(),
        /high percentage alone does not mean it is short of memory/,
      );
      assert.match(
        await dialog().innerText(),
        /Defer agent setup, restarts and live guest-agent probes during backups or a guest incident/,
      );
      await capture('memory-cache');
      for (const [state, observedAt, expected] of [
        ['last-known', time, 'Last known.'],
        ['future-state', time, 'Freshness unknown.'],
        ['current', 'not-a-date', 'Observation time unknown.'],
        ['current', '2026-10-13T20:00:00Z', 'Observation time unknown.'],
        ['unavailable', time, 'Unavailable.'],
      ]) {
        await update({ state, observedAt });
        assert.ok((await dialog().innerText()).includes(expected));
        assert.doesNotMatch(await dialog().innerText(), /Invalid Date/);
        check(`Memory open ${state}/${observedAt}`);
      }
      await update({ state: 'current', source: 'guest-agent-meminfo', observedAt: time });
      await dialog().waitFor({ state: 'detached' });
      assert.equal(await trigger('Memory').count(), 0);
      assert.equal(
        await row
          .locator('[data-row-action]')
          .evaluate((element) => element === document.activeElement),
        true,
      );
      check('Fresh current memory withdraws disclosure and returns focus to the same row');
      await update({ reason: 'prev-agent-timeout' });
      await open('Disk');
      await update({ reason: undefined });
      await dialog().waitFor({ state: 'detached' });
      assert.equal(await trigger('Disk').count(), 0);
      check('Fresh disk withdraws disclosure without remount or a row action');
      await update({ reason: 'permission-denied' });
      await open('Disk');
      await update({ instance: 'another-site' });
      await dialog().waitFor({ state: 'detached' });
      check('Same-name different canonical identity closes old disclosure');
      await open('Disk');
      await close('Disk');
      // The original row disclosure still works, separately from evidence controls.
      const disclosure = row.locator('[data-row-action]');
      if (spec.width < 768) await row.locator('[data-workload-col="name"]').tap();
      else {
        await disclosure.focus();
        await page.keyboard.press('Enter');
      }
      await page.getByRole('region', { name: 'Guest details' }).waitFor();
      assert.equal(await page.evaluate(() => window.__readingStatusAccess.actions()), 1);
      check('Original row disclosure still opens the production guest drawer');
    } finally {
      await context.close();
      result.cleanup[`${spec.name}-context`] = 'closed';
      await browser.close();
      result.cleanup[`${spec.name}-browser`] = 'closed';
    }
  }
  assert.deepEqual(result.errors, []);
  assert.deepEqual(result.consoleErrors, []);
  result.result = 'passed';
})()
  .catch((error) => {
    result.result = 'failed';
    result.failure = { message: error.message, stack: error.stack };
    process.exitCode = 1;
  })
  .finally(async () => {
    if (server) {
      await new Promise((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve())),
      );
      result.cleanup.server = 'closed';
    }
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        result: result.result,
        checks: result.checks.length,
        captures: result.captures.length,
        loaded: Object.keys(result.loaded).length,
        cleanup: result.cleanup,
        failure: result.failure?.message,
      }),
    );
  });
