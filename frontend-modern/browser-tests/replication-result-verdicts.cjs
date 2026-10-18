// Synthetic collected HTTP -> actual page/query/reader/table/CSS. This driver
// never starts replication, a guest check, a backup, a restore or recovery.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const http = require('node:http');
const { chromium } = require('playwright');

const root = '/workspace';
const compiled = path.join(root, 'tmp/replication-verdict/static');
const output = path.join(root, 'tmp/replication-verdict/browser');
const hash = (value) => crypto.createHash('sha256').update(value).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const rawFuture = '<b>provider-future-state</b>';
const failure =
  'Synthetic sync error: source snapshot unavailable. ' + 'source-context-'.repeat(12);
const result = {
  passed: false,
  scope:
    'Actual Proxmox page, 30-second query, HTTP reader, filters, table, disclosure and CSS with synthetic collected observations only.',
  playwright: require('playwright/package.json').version,
  compiler_binding_sha256: hash(fs.readFileSync(path.join(compiled, 'binding.json'))),
  runner_sha256: hash(fs.readFileSync(__filename)),
  cases: [],
  captures: [],
  served: {},
  errors: [],
  unexpected: [],
  cleanup: {},
  limits:
    'No native sync, guest operation, backup/restore, independent thaw, filesystem liveness, installed recovery, publication, release or screen-reader speech acceptance. The separately retained WebKit runtime fault is not replayed.',
};
let browser, server;
(async () => {
  fs.mkdirSync(output, { recursive: true });
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  for (const [file, digest] of Object.entries(binding.sources))
    assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), digest, file);
  for (const file of [
    'src/features/proxmox/ProxmoxPageSurface.tsx',
    'src/features/proxmox/ProxmoxReplicationTable.tsx',
    'src/hooks/createNonSuspendingQuery.ts',
    'src/pages/Docs.tsx',
    'src/features/docs/docMarkdown.ts',
    'public/docs/TROUBLESHOOTING.md',
    'browser-tests/replication-result-verdicts.cjs',
  ])
    assert(binding.sources[file], `production input missing: ${file}`);
  const assets = new Map();
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, file);
    assets.set('/' + file, bytes);
  }
  server = http.createServer((request, response) => {
    const file = new URL(request.url, 'http://fixture.invalid').pathname;
    const documentRoute = file === '/docs/TROUBLESHOOTING';
    const bytes = assets.get(documentRoute ? '/browser-tests/replication-inventory.html' : file);
    if (!bytes || !['GET', 'HEAD'].includes(request.method)) {
      response.writeHead(404).end();
      return;
    }
    result.served[file] = hash(bytes);
    response.writeHead(200, {
      'Content-Type': documentRoute ? 'text/html' :
        { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.md': 'text/markdown' }[
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
  browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
  result.chromium = browser.version();
  for (const spec of [
    { name: 'desktop', width: 1440, height: 1000, colorScheme: 'light' },
    { name: 'narrow', width: 320, height: 980, colorScheme: 'dark' },
  ]) {
    const context = await browser.newContext({
      viewport: { width: spec.width, height: spec.height },
      hasTouch: spec.name === 'narrow',
      isMobile: spec.name === 'narrow',
      colorScheme: spec.colorScheme,
      timezoneId: 'UTC',
      locale: 'en-GB',
    });
    const page = await context.newPage();
    page.setDefaultTimeout(12000);
    await page.clock.install({ time: new Date('2026-10-18T12:00:00Z') });
    if (spec.colorScheme === 'dark')
      await page.addInitScript(() =>
        document.addEventListener('DOMContentLoaded', () =>
          document.documentElement.classList.add('dark'),
        ),
      );
    await page.routeWebSocket(/\/ws(?:\?|$)/, () => {});
    const errors = [],
      reads = [],
      checks = [];
    result.cases.push({ ...spec, checks, reads, errors });
    page.on('pageerror', (error) => errors.push(error.message));
    page.on('console', (message) => {
      if (message.type() === 'error') errors.push(message.text());
    });
    let revision = 0;
    const jobs = () => {
      const base = {
        instance: 'fixture-pve',
        enabled: true,
        failCount: 0,
        sourceNode: 'pve-a',
        targetNode: 'pve-b',
        lastSyncUnix: 1792324200,
        nextSyncUnix: 1792324500,
        schedule: '*/15',
        lastSyncDurationSeconds: 65,
      };
      return [
        {
          ...base,
          id: 'ok',
          jobId: '100-0',
          guestId: 100,
          guestName: 'successful-replica',
          lastSyncStatus: 'ok',
        },
        {
          ...base,
          id: 'error',
          jobId: '101-0',
          guestId: 101,
          guestName: 'zero-count-error',
          lastSyncStatus: 'ok',
          error: failure,
        },
        {
          ...base,
          id: 'missing',
          jobId: '102-0',
          guestId: 102,
          guestName: 'missing-outcome',
          ...(revision ? { lastSyncStatus: 'COMPLETED' } : {}),
        },
        {
          ...base,
          id: 'future',
          jobId: '103-0',
          guestId: 103,
          guestName: 'future-outcome',
          lastSyncStatus: rawFuture,
        },
        {
          ...base,
          id: 'pending',
          jobId: '104-0',
          guestId: 104,
          guestName: 'pending-replica',
          lastSyncStatus: 'pending',
        },
        {
          ...base,
          id: 'disabled',
          jobId: '105-0',
          guestId: 105,
          guestName: 'disabled-replica',
          enabled: false,
          lastSyncStatus: 'error',
          error: 'Historical synthetic failure',
        },
      ];
    };
    await page.route('**/*', async (route) => {
      const request = route.request(),
        url = new URL(request.url());
      if (url.origin !== origin || !['GET', 'HEAD'].includes(request.method())) {
        result.unexpected.push({
          method: request.method(),
          origin: url.origin,
          path: url.pathname,
        });
        return route.abort();
      }
      if (!url.pathname.startsWith('/api/')) return route.continue();
      let body;
      if (url.pathname === '/api/replication/jobs') {
        assert.equal(url.search, '?platform=proxmox-pve');
        body = { data: jobs(), meta: { total: 6 } };
      } else if (url.pathname === '/api/resources') {
        body = {
          data: [
            {
              id: 'fixture-pve-a',
              name: 'pve-a',
              type: 'agent',
              status: 'online',
              sources: ['proxmox'],
              platformScopes: ['proxmox-pve'],
              lastSeen: '2026-10-18T12:00:00Z',
              proxmox: { nodeName: 'pve-a', instance: 'fixture-pve', pveVersion: '9.0' },
              metrics: {
                cpu: { percent: 5 },
                memory: { percent: 25, used: 2147483648, total: 8589934592 },
              },
            },
          ],
          meta: { totalPages: 1 },
          facets: { byType: { agent: 1 }, incidentCount: 0 },
        };
      } else if (url.pathname === '/api/license/runtime-capabilities')
        body = { capabilities: [], limits: [], max_history_days: 7 };
      else if (url.pathname === '/api/alerts/active') body = [];
      else if (url.pathname === '/api/alerts/config')
        body = { guestDefaults: {}, hostDefaults: {} };
      else body = { data: [], meta: { totalPages: 1 }, alerts: [], anomalies: [] };
      reads.push({ path: url.pathname + url.search, revision, sha256: hash(JSON.stringify(body)) });
      return route.fulfill({ json: body });
    });
    await page.goto(origin + '/browser-tests/replication-inventory.html');
    const row = (name) => page.locator('tr').filter({ hasText: name });
    for (const [name, verdict] of [
      ['successful-replica', 'OK'],
      ['zero-count-error', 'Failed'],
      ['missing-outcome', 'Unknown'],
      ['future-outcome', 'Unknown'],
      ['pending-replica', 'Pending'],
      ['disabled-replica', 'Disabled'],
    ])
      await row(name).getByText(verdict, { exact: true }).waitFor();
    await page.locator('[data-replication-result-scope]').waitFor();
    assert.equal(await page.getByText('Healthy', { exact: true }).count(), 0);
    assert.equal(new URL(page.url()).pathname, '/proxmox/replication');
    assert(
      await row('successful-replica')
        .getByText(spec.name === 'narrow' ? /^-\d/ : /overdue/)
        .count(),
    );
    checks.push(
      'Real collected HTTP reports OK, zero-count error Failed, missing/future Unknown, explicit Pending and Disabled distinctly. Previous OK does not hide overdue Next sync.',
    );
    const activate = async (button) => {
      if (spec.name === 'narrow') {
        // On phones the row is the explicit touch target. Its disclosure
        // button is sr-only until keyboard focus, not a tappable icon.
        if ((await button.getAttribute('data-row-action')) === 'true')
          await button.locator('xpath=ancestor::tr').tap();
        else await button.tap();
      }
      else {
        await button.focus();
        await page.keyboard.press('Enter');
      }
    };
    const capture = async (name, fullPage = true) => {
      await page.clock.runFor(300);
      assert(
        await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
        'no horizontal page overflow',
      );
      const file = path.join(output, `${spec.name}-${name}.png`);
      await page.screenshot({ path: file, fullPage, animations: 'disabled' });
      result.captures.push({
        path: path.relative(root, file),
        sha256: hash(fs.readFileSync(file)),
        viewport: { width: spec.width, height: spec.height },
        name,
      });
    };
    await capture('reported-results');
    await activate(page.getByRole('button', { name: 'Unknown, 2', exact: true }));
    assert.equal(await row('successful-replica').count(), 0);
    assert.equal(await row('zero-count-error').count(), 0);
    assert.equal(await row('missing-outcome').count(), 1);
    assert.equal(await row('future-outcome').count(), 1);
    checks.push(
      'Keyboard/touch Unknown filtering matches both unknown rows, not OK/error/pending/disabled.',
    );
    revision = 1;
    await page.clock.runFor(30050);
    await page.getByRole('button', { name: 'Unknown, 1', exact: true }).waitFor();
    assert.equal(await row('missing-outcome').count(), 0);
    await activate(page.getByRole('button', { name: 'OK, 2', exact: true }));
    await row('missing-outcome').getByText('OK', { exact: true }).waitFor();
    checks.push(
      'Ordinary 30-second poll replaces a missing outcome with COMPLETED. Unknown/OK counts and the active filter update without navigation.',
    );
    await activate(page.getByRole('button', { name: 'Failed, 1', exact: true }));
    await activate(page.getByRole('button', { name: 'Expand details for replication job 101-0' }));
    assert.equal(await page.locator('[data-replication-job-error]').innerText(), failure);
    await page.getByText(/previous successful sync does not override/).waitFor();
    await capture('zero-count-error');
    checks.push(
      'Keyboard/touch disclosure exposes complete wrapping error, reported ok and failure interpretation even with zero failures.',
    );
    await activate(page.getByRole('button', { name: 'Unknown, 1', exact: true }));
    await activate(page.getByRole('button', { name: 'Expand details for replication job 103-0' }));
    const raw = page
      .locator('dt')
      .filter({ hasText: /^Reported last result$/ })
      .locator('+ dd');
    assert.equal(await raw.innerText(), rawFuture);
    assert.equal(await raw.locator('b').count(), 0);
    await page.getByText(/This does not mean a sync is pending/).waitFor();
    await capture('unknown-result');
    checks.push(
      'The unknown raw provider result is selectable plain text, not HTML; its explanation does not invent pending work.',
    );
    await page.goto(origin + '/docs/TROUBLESHOOTING#replication-jobs-are-pending-stale-or-missing');
    const heading = page.getByRole('heading', { name: 'Replication jobs are Pending, stale or missing', exact: true });
    await heading.waitFor();
    await page.getByText('Unknown, even with current sync times', { exact: true }).waitFor();
    await page.getByText(/Missing or unrecognised results are Unknown, not evidence of queued work/).waitFor();
    await heading.scrollIntoViewIfNeeded();
    await capture('matching-guide', false);
    checks.push('The real Docs fetch/Markdown/router consumer retains the existing fragment and explains Unknown, explicit Pending and last-result OK consistently with the table.');
    assert.deepEqual(errors, []);
    await context.close();
  }
  assert.deepEqual(result.unexpected, []);
  result.passed = true;
})()
  .catch((error) => {
    result.error = String(error.stack || error);
    result.errors.push(result.error);
    process.exitCode = 1;
  })
  .finally(async () => {
    if (browser) await browser.close();
    result.cleanup.browserClosed = true;
    if (server) await new Promise((resolve) => server.close(resolve));
    result.cleanup.serverClosed = true;
    fs.mkdirSync(output, { recursive: true });
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        passed: result.passed,
        captures: result.captures.length,
        error: result.error,
      }),
    );
  });
