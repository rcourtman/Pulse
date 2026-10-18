// Native keyboard/touch sorting in production Protection and Docker consumers.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, firefox } = require('playwright');

const root = '/workspace';
const output = path.join(root, 'tmp/platform-sort-access-proof');
const staticRoot = path.join(root, 'tmp/platform-sort-access/static');
const origin = 'http://127.0.0.1:5347';
const digest = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const records = [
  ['failed', 'zeta', 'failed', '2026-10-18T07:00:00Z'],
  ['unknown-time', 'alpha', 'success', null],
  ['running', 'beta', 'running', '2026-10-18T08:00:00Z'],
  ['created', 'charlie', 'success', '2026-10-18T05:00:00Z'],
].map(([id, name, outcome, at]) => ({
  id,
  provider: 'truenas',
  kind: 'backup',
  mode: 'remote',
  outcome,
  startedAt: at,
  completedAt: at,
  subjectRef: { type: 'truenas-dataset', name: `tank/${name}` },
  display: { subjectLabel: `tank/${name}`, nodeHostLabel: 'fixture-nas' },
  details: {
    dataset: `tank/${name}`,
    taskName: `replicate-${name}`,
    targetDataset: 'vault/fixture',
    lastState: outcome.toUpperCase(),
    lastError: outcome === 'failed' ? 'Synthetic read-only diagnostic' : '',
  },
}));

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const source = JSON.parse(
    fs.readFileSync(path.join(root, 'tmp/platform-sort-access-source.json')),
  );
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
      'Synthetic read-only HTTP through the real recovery reader/decoder/cache and production Protection/CSS, plus synthetic Docker metadata through the production table. Not provider, backup, thaw, installed recovery or release acceptance.',
  };
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
      'node_modules/@playwright/test'
    ].version,
  );
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
        res.end(JSON.stringify({ data: records }));
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
  const waitFor = async (test, label) => {
    const deadline = Date.now() + 8000;
    while (!(await test())) {
      assert(Date.now() < deadline, label);
      await new Promise((resolve) => setTimeout(resolve, 40));
    }
  };
  try {
    await new Promise((resolve, reject) => {
      server.once('error', reject);
      server.listen(5347, '127.0.0.1', resolve);
    });
    for (const [engine, width] of [
      ['chromium', 1440],
      ['chromium', 320],
      ['firefox', 1440],
    ]) {
      browser = await (engine === 'chromium' ? chromium : firefox).launch(
        engine === 'chromium'
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const context = await browser.newContext({
        viewport: { width, height: 1000 },
        ...(engine === 'chromium' ? { isMobile: width < 768, hasTouch: width < 768 } : {}),
        locale: 'en-GB',
        timezoneId: 'UTC',
        colorScheme: width < 768 ? 'dark' : 'light',
      });
      const page = await context.newPage();
      page.setDefaultTimeout(9000);
      const entry = { engine, browser: browser.version(), width, errors: [], states: [] };
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
      const open = async () => {
        await page.goto(`${origin}/browser-tests/platform-sort-access.html`);
        await waitFor(
          async () => (await page.locator('[data-truenas-protection-row]').count()) === 4,
          'Protection HTTP read did not render',
        );
      };
      await open();
      const protection = page.getByRole('region', { name: 'Protection activity', exact: true });
      const ids = () =>
        page
          .locator('[data-truenas-protection-row]')
          .evaluateAll((rows) =>
            rows.map((row) => row.getAttribute('data-truenas-protection-row')),
          );
      const defaultOrder = await ids();
      const header = protection.getByRole('columnheader', { name: 'Dataset', exact: true });
      const dataset = header.getByRole('button', { name: 'Dataset', exact: true });
      assert.equal(await header.getAttribute('scope'), 'col');
      assert.equal(await dataset.getAttribute('type'), 'button');
      await protection.getByRole('button', { name: 'Before Protection sorting' }).focus();
      await page.keyboard.press('Tab');
      assert(await dataset.evaluate((node) => node === document.activeElement));
      await page.keyboard.press('Enter');
      assert.equal(await header.getAttribute('aria-sort'), 'ascending');
      assert.deepEqual(await ids(), ['unknown-time', 'running', 'created', 'failed']);
      await page.keyboard.press('Space');
      assert.equal(await header.getAttribute('aria-sort'), 'descending');
      assert.deepEqual(await ids(), ['failed', 'created', 'running', 'unknown-time']);
      await page.keyboard.press('Enter');
      assert.equal(await header.getAttribute('aria-sort'), null);
      assert.deepEqual(await ids(), defaultOrder);
      assert(await dataset.evaluate((node) => node === document.activeElement));
      entry.states.push(
        'Tab reaches Dataset; Enter/Space cycle ascending, descending and default once, retaining focus and a stable accessible name.',
      );

      const timeLabel = width < 768 ? 'Time' : 'Reported time';
      const timeHeader = protection.getByRole('columnheader', { name: timeLabel, exact: true });
      const time = timeHeader.getByRole('button', { name: timeLabel, exact: true });
      const visibleHeaders = [];
      for (const button of await protection.locator('thead button').all()) {
        if (await button.isVisible()) visibleHeaders.push(button);
      }
      const sequence = [];
      await protection.getByRole('button', { name: 'Before Protection sorting' }).focus();
      for (const button of visibleHeaders) {
        await page.keyboard.press('Tab');
        assert(await button.evaluate((node) => node === document.activeElement));
        const box = await button.boundingBox();
        assert(box && box.height >= 24 && box.width >= 24, 'sort target smaller than 24px');
        sequence.push((await button.innerText()).trim());
      }
      entry.headerTabOrder = sequence;
      await time.focus();
      await page.keyboard.press('Enter');
      assert.equal(await timeHeader.getAttribute('aria-sort'), 'descending');
      assert.deepEqual(await ids(), ['running', 'failed', 'created', 'unknown-time']);
      await page.keyboard.press('Space');
      assert.equal(await timeHeader.getAttribute('aria-sort'), 'ascending');
      assert.deepEqual(await ids(), ['created', 'failed', 'running', 'unknown-time']);
      const focus = await time.evaluate((node) => ({
        active: node === document.activeElement,
        visible: node.matches(':focus-visible'),
        shadow: getComputedStyle(node).boxShadow,
        align: getComputedStyle(node).textAlign,
        headerAlign: getComputedStyle(node.closest('th')).textAlign,
      }));
      assert(focus.active && focus.visible && focus.shadow !== 'none');
      assert.equal(focus.align, focus.headerAlign);
      entry.focus = focus;
      entry.states.push(
        'Chronological descending-first is unchanged, missing dates stay last in both directions; only visible native header buttons enter the Tab order.',
      );
      const capture = async (name) => {
        const file = `${engine}-${width}-${name}.jpg`;
        await page.screenshot({
          path: path.join(output, file),
          type: 'jpeg',
          quality: 82,
          fullPage: true,
        });
        result.screenshots.push({ file, sha256: digest(fs.readFileSync(path.join(output, file))) });
      };
      await capture('protection-keyboard');
      await page.keyboard.press('Enter');
      assert.equal(await timeHeader.getAttribute('aria-sort'), null);
      await page.keyboard.press('Enter');
      assert.equal(await timeHeader.getAttribute('aria-sort'), 'descending');
      await open();
      assert.equal(await timeHeader.getAttribute('aria-sort'), 'descending');
      assert.deepEqual(await ids(), ['running', 'failed', 'created', 'unknown-time']);
      entry.states.push(
        'The existing saved sort survives a browser reload; no new storage keys are introduced.',
      );
      if (width < 768) {
        await dataset.tap();
        assert.equal(await header.getAttribute('aria-sort'), 'ascending');
        assert.deepEqual(await ids(), ['unknown-time', 'running', 'created', 'failed']);
        entry.states.push(
          'The same native target responds to a touch tap without a duplicate transition.',
        );
      }
      const docker = page.getByRole('form', { name: 'Docker metadata' });
      const labels = docker.locator('th').filter({ hasText: 'Labels' });
      assert.equal(await labels.locator('button').count(), 0, 'static header is interactive');
      assert.equal(await labels.getAttribute('aria-sort'), null);
      const secret = docker.getByRole('button', { name: 'Secret', exact: true });
      await docker.getByRole('button', { name: 'Before Docker sorting' }).focus();
      await page.keyboard.press('Tab');
      assert(await secret.evaluate((node) => node === document.activeElement));
      const secretOrder = () =>
        docker
          .locator('[data-docker-secret-row]')
          .evaluateAll((rows) => rows.map((row) => row.getAttribute('data-docker-secret-row')));
      await page.keyboard.press('Enter');
      assert.deepEqual(await secretOrder(), [
        'fixture-secret-alpha',
        'fixture-secret-bravo',
        'fixture-secret-charlie',
      ]);
      await page.keyboard.press('Space');
      assert.deepEqual(await secretOrder(), [
        'fixture-secret-charlie',
        'fixture-secret-bravo',
        'fixture-secret-alpha',
      ]);
      assert.equal(await docker.getByTestId('form-submits').innerText(), '0');
      entry.states.push(
        'A second production table sorts by native Enter/Space; Labels stays static and activation never submits its enclosing fixture form.',
      );
      await capture('docker-keyboard');
      assert.equal(
        await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
        false,
        'document overflows horizontally',
      );
      assert.deepEqual(entry.errors, []);
      assert(!result.serverError);
      await context.close();
      await browser.close();
      browser = undefined;
      entry.closed = true;
    }
    result.result = 'passed';
  } catch (error) {
    result.error = String(error.stack || error);
    process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    result.cleanup.browser_closed = true;
    await new Promise((resolve) => server.close(resolve));
    result.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        result: result.result,
        source: result.source,
        cases: result.cases.length,
        captures: result.screenshots.length,
      }),
    );
  }
})();
