const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const http = require('node:http');
const { chromium, webkit } = require('playwright');

const root = '/workspace';
const configuration = JSON.parse(
  fs.readFileSync(path.join(root, 'tmp/replication-proof/mode.json')),
);
const { mode, captureOnly = false } = configuration;
assert(['baseline', 'final'].includes(mode));
const copy = path.join(root, 'tmp/replication-proof', mode);
const moduleRoot = path.join(copy, 'frontend-modern');
const compiled = path.join(copy, 'static');
const output = path.join(copy, captureOnly ? 'pixel-completion' : 'browser');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const message =
  'The replication response format is invalid. Job availability is unknown; missing rows do not mean no jobs exist.';
const result = {
  mode,
  captureOnly,
  scope: 'Actual REST reader/query/Proxmox page/tab/table with synthetic HTTP responses only.',
  playwright: require('playwright/package.json').version,
  bindingSha256: hash(fs.readFileSync(path.join(compiled, 'binding.json'))),
  observerSha256: hash(fs.readFileSync(__filename)),
  results: [],
  captures: [],
  served: {},
  cleanup: {},
};

let server;
let browser;
(async () => {
  fs.mkdirSync(output, { recursive: true });
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  for (const [file, digest] of Object.entries(binding.sources)) {
    assert.equal(hash(fs.readFileSync(path.join(moduleRoot, file))), digest, file);
    if (mode === 'final')
      assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), digest, file);
  }
  for (const file of [
    'src/features/proxmox/ProxmoxPageSurface.tsx',
    'src/features/proxmox/ProxmoxReplicationTable.tsx',
    'src/hooks/createNonSuspendingQuery.ts',
    'src/hooks/useUnifiedResources.ts',
  ])
    assert(binding.sources[file], `production consumer missing: ${file}`);
  const dependencyRoot = fs.realpathSync(path.join(moduleRoot, 'node_modules'));
  for (const [file, digest] of Object.entries(binding.dependencyInputs))
    assert.equal(hash(fs.readFileSync(path.join(dependencyRoot, file))), digest, file);
  const assets = new Map();
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, file);
    assets.set('/' + file, bytes);
  }
  server = http.createServer((request, response) => {
    const file = new URL(request.url, 'http://fixture.invalid').pathname;
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
      'Cache-Control': 'no-store',
    });
    response.end(request.method === 'HEAD' ? undefined : bytes);
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const origin = `http://127.0.0.1:${server.address().port}`;
  const scenarios = [
    { name: 'desktop', engine: chromium, width: 1440, height: 1000, colorScheme: 'light' },
    ...(mode === 'final'
      ? [{ name: 'phone', engine: webkit, width: 320, height: 900, colorScheme: 'dark' }]
      : []),
  ];
  for (const spec of scenarios) {
    browser = await spec.engine.launch(
      spec.engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    const page = await browser.newPage({
      viewport: { width: spec.width, height: spec.height },
      hasTouch: spec.name === 'phone',
      isMobile: spec.name === 'phone',
      colorScheme: spec.colorScheme,
      timezoneId: 'UTC',
      locale: 'en-GB',
    });
    page.setDefaultTimeout(10000);
    await page.clock.install({ time: new Date('2026-10-16T14:00:00Z') });
    if (spec.colorScheme === 'dark')
      await page.addInitScript(() =>
        document.addEventListener('DOMContentLoaded', () =>
          document.documentElement.classList.add('dark'),
        ),
      );
    await page.routeWebSocket(/\/ws(?:\?|$)/, () => {});
    let payload = { unexpected: 'BODY_CONTENT_DO_NOT_RENDER' };
    let html = false;
    const jobs = () => ({
      data: [
        {
          id: 'pve/100-0',
          instance: 'pve',
          jobId: '100-0',
          guestId: 100,
          guestName: 'web-replica',
          enabled: true,
          lastSyncStatus: 'ok',
          failCount: 0,
          sourceNode: 'pve-a',
          targetNode: 'pve-b',
          lastSyncUnix: 1792158600,
        },
        {
          id: 'pve/101-0',
          instance: 'pve',
          jobId: '101-0',
          guestId: 101,
          guestName: 'failed-replica',
          enabled: true,
          lastSyncStatus: 'error',
          failCount: 2,
          sourceNode: 'pve-a',
          targetNode: 'pve-b',
          error: 'Synthetic replication failed: source snapshot unavailable.',
        },
        {
          id: 'pve/102-0',
          instance: 'pve',
          jobId: '102-0',
          guestId: 102,
          guestName: 'disabled-replica',
          enabled: false,
          sourceNode: 'pve-a',
          targetNode: 'pve-b',
        },
      ],
      meta: { total: 3 },
    });
    const reads = [];
    const errors = [];
    const checks = [];
    page.on('pageerror', (error) => errors.push(error.message));
    await page.route('**/*', async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      assert.equal(url.origin, origin, 'no external requests');
      assert(['GET', 'HEAD'].includes(request.method()), 'no mutating request');
      if (!url.pathname.startsWith('/api/')) return route.continue();
      if (url.pathname === '/api/replication/jobs') {
        assert.equal(url.search, '?platform=proxmox-pve');
        const body = html ? '<html>BODY_CONTENT_DO_NOT_RENDER</html>' : JSON.stringify(payload);
        reads.push({ path: url.pathname + url.search, body, sha256: hash(body) });
        return route.fulfill({ body, contentType: html ? 'text/html' : 'application/json' });
      }
      // Unrelated page dependencies remain explicit synthetic empty reads;
      // neither the changed replication reader nor its query owner is mocked.
      const body =
        url.pathname === '/api/resources'
          ? {
              data: [
                {
                  id: 'synthetic-pve-a',
                  name: 'pve-a',
                  type: 'agent',
                  status: 'online',
                  sources: ['proxmox'],
                  platformScopes: ['proxmox-pve'],
                  lastSeen: '2026-10-16T14:00:00Z',
                  proxmox: { nodeName: 'pve-a', instance: 'pve', pveVersion: '9.0' },
                  metrics: {
                    cpu: { percent: 5 },
                    memory: { percent: 25, used: 2147483648, total: 8589934592 },
                  },
                },
              ],
              meta: { totalPages: 1 },
              facets: { byType: { agent: 1 }, incidentCount: 0 },
            }
          : url.pathname === '/api/license/runtime-capabilities'
            ? { capabilities: [], limits: [], max_history_days: 7 }
            : url.pathname === '/api/alerts/config'
              ? { guestDefaults: {}, hostDefaults: {} }
              : { data: [], meta: { totalPages: 1 }, alerts: [], anomalies: [] };
      reads.push({ path: url.pathname + url.search, sha256: hash(JSON.stringify(body)) });
      return route.fulfill({ json: body });
    });
    await page.goto(origin + '/browser-tests/replication-inventory.html');
    await page.getByTestId('proxmox-page').waitFor();
    const capture = async (state) => {
      await page.evaluate(async () => {
        await Promise.all(
          document
            .getAnimations()
            .filter((animation) => Number.isFinite(animation.effect?.getTiming().iterations))
            .map((animation) => animation.finished),
        );
        for (const element of document.querySelectorAll('.animate-fade-in')) {
          if (Number(getComputedStyle(element).opacity) < 0.99)
            throw Error('capture before fade-in completed');
        }
      });
      const file = path.join(output, `${spec.name}-${state}.png`);
      await page.screenshot({ path: file, fullPage: true });
      result.captures.push({
        path: path.relative(root, file),
        sha256: hash(fs.readFileSync(file)),
      });
    };
    const errorTitle = page.getByText('Could not load replication jobs', { exact: true });
    if (captureOnly) {
      await errorTitle.waitFor();
      await page.getByText(message, { exact: true }).waitFor();
      await capture('settled-error');
      payload = jobs();
      const button = page.getByRole('button', { name: 'Refresh', exact: true });
      if (spec.name === 'phone') await button.tap();
      else await button.click();
      await page.getByText(/100.*web-replica/).waitFor();
      const disclosure = page.getByRole('button', {
        name: 'Expand details for replication job 101-0',
      });
      if (spec.name === 'phone') await page.getByText(/101.*failed-replica/).tap();
      else await disclosure.click();
      await page.locator('[data-replication-job-error]').waitFor();
      await capture('settled-recovered');
      checks.push(
        'Capture-only completion waits for finite CSS animations and checks settled opacity. It does not replay the already completed malformed-poll/HTML/empty-control protocol.',
      );
    } else if (mode === 'baseline') {
      await page.getByTestId('proxmox-guests-section').waitFor({ state: 'visible' });
      assert.equal(await errorTitle.count(), 0);
      assert.equal(new URL(page.url()).pathname, '/proxmox/replication');
      checks.push(
        'Malformed HTTP 200 read was treated as a confirmed empty inventory and fell back to Overview.',
      );
      await capture('false-empty-fallback');
    } else {
      const checkError = async () => {
        await errorTitle.waitFor();
        await page.getByText(message, { exact: true }).waitFor();
        assert.equal(new URL(page.url()).pathname, '/proxmox/replication');
        assert.equal(await page.getByText('No replication jobs', { exact: true }).count(), 0);
        assert.equal(
          await page.getByText('BODY_CONTENT_DO_NOT_RENDER', { exact: false }).count(),
          0,
        );
        assert.equal(await page.locator('[data-proxmox-replication-layout]').count(), 0);
        assert(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
          'page stays within viewport',
        );
      };
      const refresh = async () => {
        const button = page.getByRole('button', { name: 'Refresh', exact: true });
        if (spec.name === 'phone') await button.tap();
        else {
          await button.focus();
          await page.keyboard.press('Enter');
        }
      };
      const checkJobs = async () => {
        await page.getByText(/100.*web-replica/, { exact: false }).waitFor();
        assert.equal(await errorTitle.count(), 0);
        const nav = page.getByRole('navigation', { name: 'Proxmox sections' });
        assert.equal(
          await nav.getByRole('link', { name: 'Replication' }).getAttribute('aria-current'),
          'page',
        );
        for (const [name, state] of [
          ['web-replica', 'Healthy'],
          ['failed-replica', 'Failed'],
          ['disabled-replica', 'Disabled'],
        ]) {
          const row = page.locator('tr').filter({ hasText: name });
          await row.getByText(state, { exact: true }).waitFor();
        }
      };
      await checkError();
      checks.push(
        'Initial malformed response holds the direct link on an explicit safe error, not absence.',
      );
      payload = jobs();
      await refresh();
      await checkJobs();
      checks.push(
        'Keyboard/touch Refresh recovers through the actual reader; healthy/failed/disabled states stay distinct.',
      );
      payload = { data: [jobs().data[0], { ...jobs().data[1], enabled: 'false', failCount: '0' }] };
      await page.clock.runFor(30050);
      await checkError();
      assert.equal(
        await page.getByRole('link', { name: 'Replication' }).getAttribute('aria-current'),
        'page',
      );
      checks.push(
        'Malformed poll rows do not clear presence, hide the current tab, coerce enabled or present a partial healthy inventory.',
      );
      await capture('malformed-poll');
      payload = jobs();
      await refresh();
      await checkJobs();
      html = true;
      await page.clock.runFor(30050);
      await checkError();
      checks.push(
        'A successful HTTP response containing HTML shows the same safe error; raw response/parser text never reaches the page.',
      );
      html = false;
      await refresh();
      await checkJobs();
      const disclosure = page.getByRole('button', {
        name: 'Expand details for replication job 101-0',
      });
      if (spec.name === 'phone') await page.getByText(/101.*failed-replica/).tap();
      else {
        await disclosure.focus();
        await page.keyboard.press('Enter');
      }
      await page.locator('[data-replication-job-error]').waitFor();
      assert.equal(
        await page.locator('[data-replication-job-error]').innerText(),
        'Synthetic replication failed: source snapshot unavailable.',
      );
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
      await capture('recovered');
      payload = { data: [], meta: { total: 0 } };
      await page.clock.runFor(30050);
      await page.waitForFunction(
        () => !document.querySelector('[data-proxmox-replication-layout]'),
      );
      assert.equal(await errorTitle.count(), 0);
      assert.equal(await page.getByRole('link', { name: 'Replication' }).count(), 0);
      checks.push(
        'A genuine empty array still confirms absence and uses the existing optional-tab fallback.',
      );
    }
    assert.deepEqual(errors, []);
    result.results.push({
      name: spec.name,
      version: browser.version(),
      viewport: { width: spec.width, height: spec.height },
      theme: spec.colorScheme,
      reads,
      checks,
      errors,
    });
    await page.close();
    await browser.close();
    browser = undefined;
  }
  result.result = mode === 'baseline' ? 'false-empty-reproduced' : 'passed';
})()
  .catch((error) => {
    result.result = 'failed';
    result.error = String(error.stack || error);
    console.error(error);
    process.exitCode = 1;
  })
  .finally(async () => {
    if (browser) await browser.close();
    result.cleanup.browserClosed = true;
    if (server) await new Promise((resolve) => server.close(resolve));
    result.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify({ result: result.result, mode, captures: result.captures.length }));
  });
