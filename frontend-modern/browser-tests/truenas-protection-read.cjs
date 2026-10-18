const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium } = require('playwright');

const root = '/workspace';
const output = path.join(root, 'tmp/truenas-protection-proof');
const staticRoot = path.join(root, 'tmp/truenas-protection-read/static');
const origin = 'http://127.0.0.1:5346';
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const point = (id, dataset, outcome = 'success') => ({
  id,
  provider: 'truenas',
  kind: outcome === 'failed' ? 'backup' : 'snapshot',
  mode: outcome === 'failed' ? 'remote' : 'snapshot',
  outcome,
  startedAt: '2026-10-18T07:00:00Z',
  completedAt: '2026-10-18T08:00:00Z',
  sizeBytes: 8 * 1024 ** 3,
  display: { subjectLabel: dataset, nodeHostLabel: 'fixture-nas' },
  details:
    outcome === 'failed'
      ? { taskName: 'replicate-photos', targetDataset: 'vault/photos', lastState: 'FAILED' }
      : { dataset, snapshot: 'daily-2026-10-18' },
});

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const binding = JSON.parse(fs.readFileSync(path.join(staticRoot, 'binding.json')));
  const source = JSON.parse(fs.readFileSync(path.join(root, 'tmp/truenas-source.json')));
  assert.equal(binding.source_sha, source.sha);
  for (const [file, digest] of Object.entries(binding.sources))
    assert.equal(hash(path.resolve(root, 'frontend-modern', file)), digest, file);
  for (const [file, digest] of Object.entries(binding.artifacts))
    assert.equal(hash(path.join(staticRoot, file)), digest, file);
  const result = {
    result: 'incomplete',
    source: source.sha,
    playwright: require('playwright/package.json').version,
    compiled_inputs: Object.keys(binding.sources).length,
    artifacts: Object.keys(binding.artifacts).length,
    cases: [],
    screenshots: [],
    requests: [],
    cleanup: {},
    limits:
      'Synthetic local HTTP responses through real apiFetchJSON, useRecoveryPoints, decoder, query cache, Protection table and CSS. No appliance or guest operation, installed recovery, published availability, restore or thaw proof.',
  };
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  let reply = { status: 200, body: { data: 'unreadable' } };
  let release = null;
  const server = http.createServer(async (req, res) => {
    try {
      assert.equal(req.method, 'GET');
      const url = new URL(req.url, origin);
      if (url.pathname === '/api/recovery/points') {
        const selected = reply;
        result.requests.push({
          path: url.pathname,
          query: url.search,
          method: req.method,
          status: selected.status,
        });
        assert.equal(url.searchParams.get('platform'), 'truenas');
        assert.equal(url.searchParams.get('page'), '1');
        assert.equal(url.searchParams.get('limit'), '200');
        if (selected.hold)
          await new Promise((done) => {
            release = done;
          });
        res.writeHead(selected.status, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(selected.body));
        return;
      }
      assert(!url.pathname.startsWith('/api/'), 'unexpected API request');
      if (url.pathname === '/favicon.ico') {
        res.writeHead(204);
        res.end();
        return;
      }
      const file = path.resolve(staticRoot, '.' + decodeURIComponent(url.pathname));
      assert(file.startsWith(staticRoot + path.sep));
      const mime =
        { '.html': 'text/html', '.js': 'application/javascript', '.css': 'text/css' }[
          path.extname(file)
        ] || 'application/octet-stream';
      const content = fs.readFileSync(file);
      res.writeHead(200, { 'Content-Type': mime });
      res.end(content);
    } catch (error) {
      result.serverError = String(error);
      if (!res.headersSent) res.writeHead(500);
      res.end('fixture server failure');
    }
  });
  let browser;
  try {
    await new Promise((done) => server.listen(5346, '127.0.0.1', done));
    browser = await chromium.launch({
      headless: true,
      channel: 'chromium',
      args: ['--no-sandbox'],
    });
    result.browser = browser.version();
    for (const width of [1440, 320]) {
      const context = await browser.newContext({
        viewport: { width, height: 1000 },
        isMobile: width < 768,
        hasTouch: width < 768,
        locale: 'en-GB',
        timezoneId: 'UTC',
        colorScheme: width < 768 ? 'dark' : 'light',
      });
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      const entry = { width, errors: [], states: [] };
      result.cases.push(entry);
      page.on('pageerror', (error) => entry.errors.push(error.message));
      page.on('request', (request) => {
        assert.equal(new URL(request.url()).origin, origin, 'off-origin request');
        assert.equal(request.method(), 'GET', 'unexpected write');
      });
      await page.addInitScript((dark) => {
        if (dark)
          document.addEventListener('DOMContentLoaded', () =>
            document.documentElement.classList.add('dark'),
          );
      }, width < 768);
      const activate = (locator) => (width < 768 ? locator.tap() : locator.click());
      const capture = async (state) => {
        assert(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1),
          'page overflows horizontally',
        );
        const file = `${width}-${state}.jpg`;
        await page.screenshot({
          path: path.join(output, file),
          fullPage: true,
          animations: 'disabled',
          type: 'jpeg',
          quality: 80,
        });
        result.screenshots.push({ file, sha256: hash(path.join(output, file)), width, state });
      };
      const rows = () => page.locator('[data-truenas-protection-row]');
      const hasCount = async (count) => {
        await page.waitForFunction(
          (expected) =>
            document.querySelectorAll('[data-truenas-protection-row]').length === expected,
          count,
        );
      };
      reply = { status: 200, body: { data: 'not-an-inventory' } };
      await page.goto(origin + '/browser-tests/truenas-protection-read.html');
      await page.getByText('Could not load TrueNAS protection', { exact: true }).waitFor();
      assert.equal(
        await page.getByText('No TrueNAS protection activity', { exact: true }).count(),
        0,
      );
      entry.states.push('Unreadable first HTTP 200 is unavailable, not empty');
      await capture('unreadable');

      reply = {
        status: 200,
        body: {
          data: [point('snapshot-a', 'tank/apps'), point('replication-a', 'tank/photos', 'failed')],
        },
      };
      await activate(page.getByRole('button', { name: 'Refresh', exact: true }));
      await hasCount(2);
      if (width < 768) await activate(page.getByRole('button', { name: 'Filters', exact: true }));
      await activate(page.getByRole('button', { name: 'Healthy, 1' }));
      await hasCount(1);
      assert.equal(await rows().first().getAttribute('data-truenas-protection-row'), 'snapshot-a');
      await activate(page.getByRole('button', { name: 'All, 2' }));
      await hasCount(2);
      const row = page.locator('[data-truenas-protection-row="snapshot-a"]');
      if (width >= 768) {
        await row.getByRole('button').focus();
        await page.keyboard.press('Enter');
      } else await row.tap();
      await page.locator('[data-truenas-protection-detail-for="snapshot-a"]').waitFor();
      entry.states.push(
        'Readable legacy subject labels reach filters and keyboard/touch disclosures',
      );

      reply = { status: 200, body: { data: {} } };
      await activate(page.getByRole('button', { name: 'Refresh fixture read' }));
      await page.getByRole('status').filter({ hasText: 'last loaded records' }).waitFor();
      assert.equal(await rows().count(), 2);
      await page.locator('[data-truenas-protection-detail-for="snapshot-a"]').waitFor();
      assert.equal(
        await page.getByText('No TrueNAS protection activity', { exact: true }).count(),
        0,
      );
      entry.states.push(
        'Malformed refresh preserves two historical rows and open details with visible stale-record qualification',
      );
      await capture('retained');

      reply = { status: 200, hold: true, body: { data: [point('snapshot-retry', 'tank/retry')] } };
      await activate(
        page.getByRole('button', { name: 'Refresh protection activity', exact: true }),
      );
      await page.getByRole('button', { name: 'Refreshing protection activity…' }).waitFor();
      assert(
        await page.getByRole('button', { name: 'Refreshing protection activity…' }).isDisabled(),
      );
      assert.equal(await rows().count(), 2);
      await capture('explicit-retry');
      assert(release);
      release();
      release = null;
      await page.locator('[data-truenas-protection-row="snapshot-retry"]').waitFor();
      reply = { status: 200, body: { data: 'unreadable' } };
      await activate(page.getByRole('button', { name: 'Refresh fixture read' }));
      await page.getByRole('status').filter({ hasText: 'last loaded records' }).waitFor();
      entry.states.push(
        'Explicit retry disables its control without hiding the warning or retained rows',
      );

      reply = { status: 200, hold: true, body: { data: [point('snapshot-b', 'tank/recovered')] } };
      const backgroundRead = page.waitForRequest((request) =>
        request.url().includes('/api/recovery/points'),
      );
      await activate(page.getByRole('button', { name: 'Leave fixture tab' }));
      await activate(page.getByRole('button', { name: 'Open fixture tab' }));
      await backgroundRead;
      await page.getByRole('status').filter({ hasText: 'last loaded records' }).waitFor();
      assert(
        await page
          .getByRole('button', { name: 'Refresh protection activity', exact: true })
          .isEnabled(),
      );
      assert.equal(await rows().count(), 1);
      await capture('cached-retry');
      assert(release, 'held recovery request was not admitted');
      release();
      release = null;
      await page.locator('[data-truenas-protection-row="snapshot-b"]').waitFor();
      assert.equal(
        await page.getByRole('status').filter({ hasText: 'last loaded records' }).count(),
        0,
      );
      entry.states.push(
        'Cached remount keeps the warning throughout its pending retry; valid replacement clears it',
      );

      reply = { status: 200, body: { data: [point('partial', 'tank/partial'), null] } };
      await activate(page.getByRole('button', { name: 'Refresh fixture read' }));
      await page.getByRole('status').filter({ hasText: 'last loaded records' }).waitFor();
      assert.equal(await page.locator('[data-truenas-protection-row="partial"]').count(), 0);
      assert.equal(await page.locator('[data-truenas-protection-row="snapshot-b"]').count(), 1);
      reply = { status: 503, body: { error: 'private-fixture-diagnostic' } };
      const failedRequest = page.waitForResponse(
        (response) => response.url().includes('/api/recovery/points') && response.status() === 503,
      );
      await activate(
        page.getByRole('button', { name: 'Refresh protection activity', exact: true }),
      );
      await failedRequest;
      await page
        .getByRole('button', { name: 'Refresh protection activity', exact: true })
        .waitFor();
      assert.equal(await page.locator('[data-truenas-protection-row="snapshot-b"]').count(), 1);
      assert.equal(await page.getByText('private-fixture-diagnostic').count(), 0);
      entry.states.push(
        'Partly malformed arrays and HTTP 503 cannot replace the inventory or expose raw error text',
      );

      reply = { status: 200, body: { data: [] } };
      await activate(
        page.getByRole('button', { name: 'Refresh protection activity', exact: true }),
      );
      await page.getByText('No TrueNAS protection activity', { exact: true }).waitFor();
      assert.equal(await rows().count(), 0);
      assert.equal(
        await page.getByRole('status').filter({ hasText: 'last loaded records' }).count(),
        0,
      );
      await capture('empty');
      entry.states.push(
        'A readable empty array authoritatively replaces the retained page and shows the normal empty state',
      );

      reply = { status: 500, body: { error: 'offline' } };
      await activate(page.getByRole('button', { name: 'Refresh fixture read' }));
      await page.getByText('Could not load TrueNAS protection', { exact: true }).waitFor();
      assert.equal(
        await page.getByText('No TrueNAS protection activity', { exact: true }).count(),
        0,
      );
      reply = { status: 200, hold: true, body: { data: [point('snapshot-c', 'tank/final')] } };
      await activate(page.getByRole('button', { name: 'Refresh', exact: true }));
      await page.getByText('Could not load TrueNAS protection', { exact: true }).waitFor();
      assert(await page.getByRole('button', { name: 'Refresh', exact: true }).isDisabled());
      assert.equal(await page.getByText('Loading TrueNAS protection', { exact: true }).count(), 0);
      assert(release);
      release();
      release = null;
      await page.locator('[data-truenas-protection-row="snapshot-c"]').waitFor();
      entry.states.push(
        'Failure after empty stays unavailable through a pending retry, then recovers',
      );

      reply = { status: 403, body: { error: 'Access denied' } };
      await activate(page.getByRole('button', { name: 'Refresh fixture read' }));
      await page.getByText('Could not load TrueNAS protection', { exact: true }).waitFor();
      assert.equal(await rows().count(), 0);
      await capture('denied');
      entry.states.push(
        'Existing HTTP 403 boundary withdraws previously loaded rows instead of retaining denied inventory',
      );
      assert.deepEqual(entry.errors, []);
      await context.close();
    }
    assert.equal(result.serverError, undefined);
    result.result = 'passed';
  } catch (error) {
    result.failure = { message: error.message, stack: error.stack };
    throw error;
  } finally {
    if (release) release();
    if (browser) {
      await browser.close();
      result.cleanup.browser_closed = true;
    }
    await new Promise((done) => server.close(done));
    result.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        result: result.result,
        source: result.source,
        captures: result.screenshots.length,
      }),
    );
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
