// Safe HTTP inventory through the real recovery reader, Protection UI and CSS.
// Geometry checks accompany actual captures; neither is appliance recovery.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, firefox } = require('playwright');

const root = '/workspace';
const output = path.join(root, 'tmp/protection-density-proof');
const staticRoot = path.join(root, 'tmp/truenas-protection-read/static');
const origin = 'http://127.0.0.1:5347';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const records = ['failed', 'warning', 'running', 'unknown', 'success'].map((outcome, index) => ({
  id: `fixture-${outcome}`,
  provider: 'truenas',
  kind: index === 4 ? 'snapshot' : 'backup',
  mode: index === 4 ? 'snapshot' : 'remote',
  outcome,
  startedAt: index === 3 ? null : `2026-10-18T0${index + 3}:00:00Z`,
  completedAt: index === 3 ? null : `2026-10-18T0${index + 3}:00:00Z`,
  subjectRef: { type: 'truenas-dataset', name: `tank/${outcome}` },
  display: { subjectLabel: `tank/${outcome}`, nodeHostLabel: 'fixture-nas' },
  details: {
    taskName: `Replicate ${outcome} dataset to independent vault`,
    targetDataset: 'vault/family/independent-copy',
    snapshot: index === 4 ? 'daily-20261018' : undefined,
    lastState: outcome.toUpperCase(),
    lastError: outcome === 'failed' ? 'Synthetic permission failure\nNo replication was run.' : '',
  },
}));

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const source = JSON.parse(fs.readFileSync(path.join(root, 'tmp/protection-density-source.json')));
  const binding = JSON.parse(fs.readFileSync(path.join(staticRoot, 'binding.json')));
  assert.equal(binding.source_sha, source.sha);
  for (const [file, digest] of Object.entries(binding.sources))
    assert.equal(hash(fs.readFileSync(path.resolve(root, 'frontend-modern', file))), digest, file);
  const assets = new Map();
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(staticRoot, file));
    assert.equal(hash(bytes), digest, file);
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
      'Safe synthetic GET inventory, real HTTP reader/decoder/query cache and production Protection table/CSS. Not appliance, guest, installed recovery, protected publication or release acceptance. WebKit source-free runtime fault remains separately owned and is not replayed.',
  };
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  let reply = records;
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
      res.end('fixture failure');
    }
  });
  let browser;
  try {
    await new Promise((resolve, reject) => {
      server.once('error', reject);
      server.listen(5347, '127.0.0.1', resolve);
    });
    for (const config of [
      { id: 'desktop', engine: 'chromium', width: 1440, locale: 'en-GB' },
      { id: 'phone-uk', engine: 'chromium', width: 320, locale: 'en-GB', dark: true },
      { id: 'phone-us', engine: 'chromium', width: 320, locale: 'en-US' },
      { id: 'tablet', engine: 'chromium', width: 600, locale: 'en-GB' },
      {
        id: 'nested',
        engine: 'chromium',
        width: 1440,
        container: 344,
        locale: 'en-GB',
        dark: true,
      },
      { id: 'firefox', engine: 'firefox', width: 1440, locale: 'en-GB' },
    ]) {
      const narrow = Boolean(config.container || config.width === 320);
      reply = records;
      browser = await (config.engine === 'chromium' ? chromium : firefox).launch(
        config.engine === 'chromium'
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const context = await browser.newContext({
        viewport: { width: config.width, height: 1000 },
        ...(config.engine === 'chromium'
          ? { isMobile: config.width === 320, hasTouch: config.width === 320 }
          : {}),
        locale: config.locale,
        timezoneId: 'UTC',
        colorScheme: config.dark ? 'dark' : 'light',
      });
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      const entry = { ...config, version: browser.version(), errors: [], states: [] };
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
      }, Boolean(config.dark));
      await page.goto(origin + '/browser-tests/truenas-protection-read.html');
      if (config.container)
        await page.locator('main').evaluate((element, width) => {
          element.style.maxWidth = width + 'px';
        }, config.container);
      const rows = page.locator('[data-truenas-protection-row]');
      await rows.nth(4).waitFor();
      const headers = page.locator('.platform-table > thead th:visible');
      assert.equal(await headers.count(), narrow ? 3 : config.width < 1024 ? 5 : 6);
      entry.headers = await headers.allInnerTexts();
      if (narrow) {
        assert.deepEqual(
          entry.headers.map((text) => text.trim().toLowerCase()),
          ['dataset', 'time', 'outcome'],
        );
        for (const outcome of ['failed', 'warning', 'running', 'unknown', 'success']) {
          const row = page.locator(`[data-truenas-protection-row="fixture-${outcome}"]`);
          const cells = row.locator('td:visible');
          assert.equal(await cells.count(), 3);
          const label =
            outcome === 'success' ? 'Healthy' : outcome[0].toUpperCase() + outcome.slice(1);
          assert.equal(await cells.last().innerText(), label);
          const fits = await cells.last().evaluate((cell) => {
            const badge = cell.querySelector('span');
            const content = badge.getBoundingClientRect();
            const box = cell.getBoundingClientRect();
            return (
              content.left >= box.left &&
              content.right <= box.right + 0.5 &&
              badge.scrollWidth <= badge.clientWidth + 0.5
            );
          });
          assert(fits, `${config.id} clipped ${label}`);
          const time = row.locator('time');
          if (outcome === 'unknown') {
            assert.equal(await time.count(), 0);
            assert.equal(await cells.nth(1).innerText(), '-');
          } else {
            const dateAndClock = time.locator('.platform-table-phone-only-inline > span');
            assert.equal(await dateAndClock.count(), 2);
            for (const line of await dateAndClock.all())
              assert(
                await line.evaluate((element) => {
                  const range = document.createRange();
                  range.selectNodeContents(element);
                  const text = range.getBoundingClientRect();
                  const cell = element.closest('td').getBoundingClientRect();
                  return text.left >= cell.left && text.right <= cell.right + 0.5;
                }),
                `${config.id} clipped timestamp`,
              );
          }
          assert.equal(
            await cells
              .first()
              .getByText(outcome === 'success' ? 'Snapshot' : 'Replication', { exact: true })
              .count(),
            1,
          );
        }
        entry.states.push(
          'All five outcomes and both date/clock lines fit fully; kind survives the priority projection; missing time stays unavailable.',
        );
      }
      const timeSort = page.getByRole('button', {
        name: narrow ? 'Time' : 'Reported time',
        exact: true,
      });
      await timeSort.focus();
      await page.keyboard.press('Enter');
      assert.equal(await timeSort.locator('..').getAttribute('aria-sort'), 'descending');
      assert.equal(
        await rows.last().getAttribute('data-truenas-protection-row'),
        'fixture-unknown',
      );
      await page.keyboard.press('Space');
      assert.equal(await timeSort.locator('..').getAttribute('aria-sort'), 'ascending');
      assert.equal(
        await rows.last().getAttribute('data-truenas-protection-row'),
        'fixture-unknown',
      );
      assert(await timeSort.evaluate((element) => element === document.activeElement));
      entry.states.push(
        'Chronological Enter/Space sorting retains focus, missing-last ordering and native header sort state.',
      );
      const capture = async (state) => {
        await page.waitForFunction(() => document.fonts.status === 'loaded');
        assert(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1),
          'document overflow',
        );
        const file = `${config.id}-${state}.jpg`;
        await page.screenshot({
          path: path.join(output, file),
          fullPage: true,
          animations: 'disabled',
          type: 'jpeg',
          quality: 85,
        });
        result.screenshots.push({
          file,
          sha256: hash(fs.readFileSync(path.join(output, file))),
          case: config.id,
          state,
        });
      };
      await capture('summary');
      const failed = page.locator('[data-truenas-protection-row="fixture-failed"]');
      if (config.width === 320) await failed.tap();
      else {
        await failed.getByRole('button').focus();
        await page.keyboard.press('Enter');
      }
      const detail = page.locator('[data-truenas-protection-detail-for="fixture-failed"]');
      await detail.waitFor();
      assert(
        (await detail
          .getByText('Replicate failed dataset to independent vault', { exact: true })
          .count()) > 0,
      );
      assert(
        (await detail.getByText('vault/family/independent-copy', { exact: true }).count()) > 0,
      );
      assert.equal(await detail.getByText('Last run', { exact: true }).count(), 1);
      assert.equal(
        await detail
          .getByText('Synthetic permission failure\nNo replication was run.', { exact: true })
          .count(),
        1,
      );
      if (config.id === 'desktop' || config.id === 'phone-uk') await capture('detail');
      reply = records.map((record) =>
        record.id === 'fixture-failed'
          ? { ...record, startedAt: '2026-10-18T10:30:00Z', completedAt: '2026-10-18T10:30:00Z' }
          : record,
      );
      const refresh = page.getByRole('button', { name: 'Refresh fixture read', exact: true });
      if (config.width === 320) await refresh.tap();
      else await refresh.click();
      await page.waitForFunction(
        () =>
          document
            .querySelector('[data-truenas-protection-row="fixture-failed"] time')
            ?.getAttribute('datetime') === '2026-10-18T10:30:00.000Z',
      );
      assert.equal(await detail.count(), 1);
      await detail.getByRole('button', { name: /^Collapse .* details$/ }).click();
      await detail.waitFor({ state: 'detached' });
      entry.states.push(
        'Touch/keyboard detail exposes the full artifact, target, Last run and escaped reported error; accepted HTTP refresh updates the same row/time without closing detail, then collapse works.',
      );
      assert.deepEqual(entry.errors, []);
      await context.close();
      await browser.close();
      browser = undefined;
    }
    assert.equal(result.serverError, undefined);
    result.result = 'passed';
  } catch (error) {
    result.failure = String(error.stack || error);
    process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    result.cleanup.browsers_closed = true;
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
