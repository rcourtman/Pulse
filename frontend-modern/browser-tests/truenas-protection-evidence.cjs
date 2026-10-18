// Synthetic read-only TrueNAS transport, real reader/decoder and Protection UI.
// The mapper mirrors LastRun into both times even when the task is running.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium } = require('playwright');

const root = '/workspace';
const output = path.join(root, 'tmp/truenas-evidence-proof');
const staticRoot = path.join(root, 'tmp/truenas-protection-read/static');
const origin = 'http://127.0.0.1:5347';
const digest = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const initialError =
  'Permission denied while sending tank/photos to vault/photos.\n' +
  'Dataset reference: ' +
  'segment-'.repeat(24) +
  '\n<img src=x onerror="window.fixtureUnsafe=true">';
const replication = (id, dataset, outcome, at, error = '') => ({
  id,
  provider: 'truenas',
  kind: 'backup',
  mode: 'remote',
  outcome,
  startedAt: at,
  completedAt: at,
  subjectRef: { type: 'truenas-dataset', name: dataset },
  display: { subjectLabel: dataset, nodeHostLabel: 'fixture-nas' },
  details: {
    taskName: `replicate-${dataset}`,
    targetDataset: 'vault/photos',
    lastState: outcome.toUpperCase(),
    lastError: error,
  },
});
const snapshot = (id, dataset, created) => ({
  id,
  provider: 'truenas',
  kind: 'snapshot',
  mode: 'snapshot',
  outcome: 'success',
  startedAt: created ? '2026-10-18T05:00:00Z' : null,
  completedAt: '2026-10-18T05:00:00Z',
  display: { subjectLabel: dataset, nodeHostLabel: 'fixture-nas' },
  details: { dataset, snapshot: 'fixture-daily' },
});
const inventory = () => [
  replication('failed', 'tank/photos', 'failed', '2026-10-18T07:00:00Z', initialError),
  replication('running', 'tank/live', 'running', '2026-10-18T08:00:00Z'),
  snapshot('created', 'tank/apps', true),
  snapshot('observed', 'tank/observed', false),
];

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const source = JSON.parse(fs.readFileSync(path.join(root, 'tmp/truenas-evidence-source.json')));
  const binding = JSON.parse(fs.readFileSync(path.join(staticRoot, 'binding.json')));
  assert.equal(binding.source_sha, source.sha);
  for (const [file, hash] of Object.entries(binding.sources))
    assert.equal(digest(fs.readFileSync(path.resolve(root, 'frontend-modern', file))), hash, file);
  const assets = new Map();
  for (const [file, hash] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(staticRoot, file));
    assert.equal(digest(bytes), hash, file);
    assets.set('/' + file, bytes);
  }
  const result = {
    result: 'incomplete',
    source: source.sha,
    playwright: require('playwright/package.json').version,
    compiled_inputs: Object.keys(binding.sources).length,
    artifacts: assets.size,
    cases: [],
    screenshots: [],
    requests: [],
    cleanup: {},
    limits:
      'Safe local transport fixtures through real HTTP reader/decoder/query cache, production Protection table and CSS. No appliance, replication, guest operation, installed recovery or release availability.',
  };
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  let reply = inventory();
  const server = http.createServer((req, res) => {
    try {
      assert.equal(req.method, 'GET');
      const url = new URL(req.url, origin);
      if (url.pathname === '/api/recovery/points') {
        assert.equal(url.searchParams.get('platform'), 'truenas');
        assert.equal(url.searchParams.get('page'), '1');
        assert.equal(url.searchParams.get('limit'), '200');
        result.requests.push({ method: req.method, path: url.pathname, query: url.search });
        res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
        res.end(JSON.stringify({ data: reply }));
        return;
      }
      if (url.pathname === '/favicon.ico') {
        res.writeHead(204);
        res.end();
        return;
      }
      const bytes = assets.get(url.pathname);
      assert(bytes, `Unbound asset: ${url.pathname}`);
      const mime = { '.html': 'text/html', '.js': 'application/javascript', '.css': 'text/css' };
      res.writeHead(200, { 'Content-Type': mime[path.extname(url.pathname)] });
      res.end(bytes);
    } catch (error) {
      result.serverError = String(error);
      if (!res.headersSent) res.writeHead(500);
      res.end('fixture server failure');
    }
  });
  let browser;
  try {
    await new Promise((resolve, reject) => {
      server.once('error', reject);
      server.listen(5347, '127.0.0.1', resolve);
    });
    browser = await chromium.launch({
      headless: true,
      channel: 'chromium',
      args: ['--no-sandbox'],
    });
    result.browser = browser.version();
    for (const width of [1440, 320]) {
      reply = inventory();
      const context = await browser.newContext({
        viewport: { width, height: 1000 },
        isMobile: width < 768,
        hasTouch: width < 768,
        locale: 'en-GB',
        timezoneId: 'UTC',
        colorScheme: width < 768 ? 'dark' : 'light',
      });
      const page = await context.newPage();
      page.setDefaultTimeout(12000);
      const entry = { width, errors: [], states: [] };
      result.cases.push(entry);
      page.on('pageerror', (error) => entry.errors.push(error.message));
      page.on('request', (request) => {
        assert.equal(new URL(request.url()).origin, origin, 'off-origin request');
        assert.equal(request.method(), 'GET', 'unexpected write');
      });
      await page.addInitScript((dark) => {
        document.addEventListener('DOMContentLoaded', () => {
          if (dark) document.documentElement.classList.add('dark');
        });
      }, width < 768);
      const activate = (locator) => (width < 768 ? locator.tap() : locator.click());
      const rows = page.locator('[data-truenas-protection-row]');
      const detail = (id) => page.locator(`[data-truenas-protection-detail-for="${id}"]`);
      const open = async (id) => {
        const row = page.locator(`[data-truenas-protection-row="${id}"]`);
        if (width < 768) await row.tap();
        else {
          await row.getByRole('button').focus();
          await page.keyboard.press('Enter');
        }
        await detail(id).waitFor();
      };
      const capture = async (id, state) => {
        await page.waitForFunction((selector) => {
          const element = document.querySelector(selector);
          if (!element) return false;
          for (let node = element; node; node = node.parentElement)
            if (Number(getComputedStyle(node).opacity) < 0.99) return false;
          return document.fonts.status === 'loaded';
        }, `[data-truenas-protection-detail-for="${id}"]`);
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
          quality: 85,
        });
        result.screenshots.push({
          file,
          sha256: digest(fs.readFileSync(path.join(output, file))),
          width,
          state,
        });
      };
      await page.goto(origin + '/browser-tests/truenas-protection-read.html');
      await rows.nth(3).waitFor();
      assert(!(await page.locator('thead').innerText()).includes('Completed'));
      assert.equal(await page.locator('thead th').filter({ hasText: 'Reported time' }).count(), 1);
      if (width < 768) await activate(page.getByRole('button', { name: 'Filters', exact: true }));
      await activate(page.getByRole('button', { name: 'Attention, 1' }));
      assert.equal(await rows.count(), 1);
      assert.equal(await rows.first().getAttribute('data-truenas-protection-row'), 'failed');
      await activate(page.getByRole('button', { name: 'All, 4' }));
      const search = page.getByPlaceholder('Search snapshots or replication');
      await search.fill('Permission denied');
      await page.waitForFunction(
        () => document.querySelectorAll('[data-truenas-protection-row]').length === 1,
      );
      await open('failed');
      await detail('failed').getByText('Reported error', { exact: true }).waitFor();
      const error = detail('failed').locator('span.whitespace-pre-wrap');
      assert.equal(await error.textContent(), initialError);
      assert.equal(await detail('failed').locator('img, script, a').count(), 0);
      assert.equal(await page.evaluate(() => window.fixtureUnsafe), undefined);
      assert.equal(await detail('failed').getByText('Last run', { exact: true }).count(), 1);
      assert.equal(
        await detail('failed')
          .getByText(/^(Started|Completed)$/)
          .count(),
        0,
      );
      assert(await error.evaluate((element) => element.scrollWidth <= element.clientWidth + 1));
      entry.states.push(
        'Attention and existing error search find the failed task; keyboard/touch disclosure renders its exact multiline escaped error without clipping or false completion.',
      );
      await capture('failed', 'reported-error');

      await search.fill('');
      await page.waitForFunction(
        () => document.querySelectorAll('[data-truenas-protection-row]').length === 4,
      );
      reply = inventory();
      reply[0] = replication(
        'failed',
        'tank/photos',
        'failed',
        '2026-10-18T09:00:00Z',
        'Replacement reported error',
      );
      await activate(page.getByRole('button', { name: 'Refresh fixture read' }));
      await detail('failed').getByText('Replacement reported error', { exact: true }).waitFor();
      assert(!(await detail('failed').innerText()).includes('Permission denied'));
      reply[0] = replication('failed', 'tank/photos', 'success', '2026-10-18T10:00:00Z');
      await activate(page.getByRole('button', { name: 'Refresh fixture read' }));
      await page.waitForFunction(
        () =>
          document.querySelector('[data-truenas-protection-row="failed"]')?.dataset
            .truenasProtectionOutcome === 'success',
      );
      assert.equal(await detail('failed').getByText('Reported error', { exact: true }).count(), 0);
      assert(!(await detail('failed').innerText()).includes('Replacement reported error'));
      entry.states.push(
        'Accepted HTTP replacement updates the same open task, then removes obsolete error evidence without retaining it or altering the supplied outcome.',
      );

      await open('running');
      assert.equal(await detail('running').getByText('Last run', { exact: true }).count(), 1);
      assert.equal(
        await detail('running')
          .getByText(/^(Started|Completed)$/)
          .count(),
        0,
      );
      assert.equal(await detail('running').getByText('Reported error', { exact: true }).count(), 0);
      await capture('running', 'running-last-run');
      await open('created');
      assert.equal(await detail('created').getByText('Created', { exact: true }).count(), 1);
      await open('observed');
      assert.equal(await detail('observed').getByText('Observed', { exact: true }).count(), 1);
      assert.equal(
        await detail('observed')
          .getByText(/^(Created|Started|Completed)$/)
          .count(),
        0,
      );
      entry.states.push(
        'A running task keeps only Last run; snapshots distinguish actual creation from collection-only Observed time.',
      );
      await capture('observed', 'observed-not-created');
      assert.deepEqual(entry.errors, []);
      await context.close();
    }
    assert.equal(result.serverError, undefined);
    result.result = 'passed';
  } catch (error) {
    result.failure = String(error.stack || error);
    process.exitCode = 1;
  } finally {
    if (browser) {
      await browser.close();
      result.cleanup.browser_closed = true;
    }
    await new Promise((resolve) => {
      server.close(resolve);
      server.closeAllConnections();
    });
    result.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({ result: result.result, source: source.sha, failure: result.failure }),
    );
  }
})();
