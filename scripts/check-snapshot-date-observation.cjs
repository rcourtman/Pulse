// Run from the assigned root with pulse-worker-browser. No external services.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

const root = '/workspace';
const compiled = path.join(root, 'tmp/snapshot-date-static');
const output = path.join(root, 'tmp/snapshot-date-browser');
const origin = 'http://127.0.0.1:5371';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const producer = JSON.parse(fs.readFileSync(path.join(root, 'tmp/snapshot-date-producer.json')));
const frames = producer.frames;
const result = {
  result: 'incomplete',
  base: '7374abf19b434c9a275df730d28234bea3943774',
  producer,
  consumer: binding,
  harness: hash(fs.readFileSync(__filename)),
  playwright: require('playwright/package.json').version,
  cases: [],
  requests: [],
  captures: [],
  cleanup: { browser: false, server: false },
  limits: 'Real CheckSnapshotsForInstance/GetActiveAlerts -> AlertsAPI -> Overview/card; synthetic snapshot listings only. No installed recovery, guest action, backup execution, notification delivery or reporter acceptance.',
};
let server;
let browser;

(async () => {
  fs.mkdirSync(output, { recursive: true });
  assert.equal(result.playwright, JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages['node_modules/@playwright/test'].version);
  assert.equal(binding.graph, hash(fs.readFileSync(path.join(root, 'frontend-modern/package-lock.json'))));
  assert.equal(binding.builder, hash(fs.readFileSync(path.join(root, 'frontend-modern/browser-tests/build-snapshot-date-observation.mjs'))));
  assert.equal(producer.runtime_sha256, hash(fs.readFileSync(path.join(root, 'internal/alerts/backup_snapshot.go'))));
  assert.deepEqual(frames.stale, frames.missing);
  assert.deepEqual(frames.stale, frames.future);
  assert.equal(frames.stale.length, 1);
  assert.deepEqual(frames.recovered, []);
  for (const [file, digest] of Object.entries(binding.sources)) {
    assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), digest, file);
  }
  for (const file of ['src/api/alerts.ts', 'src/features/alerts/OverviewTab.tsx', 'src/features/alerts/AlertOverviewAlertCard.tsx']) {
    assert(binding.sources[file], `real consumer not compiled: ${file}`);
  }
  const assets = new Map();
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, file);
    assets.set('/' + file, bytes);
  }
  server = http.createServer((request, response) => {
    const pathname = new URL(request.url, origin).pathname;
    const bytes = assets.get(pathname);
    if (!bytes || request.method !== 'GET') {
      response.writeHead(404).end();
      return;
    }
    response.writeHead(200, { 'Content-Type': { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css' }[path.extname(pathname)] || 'application/octet-stream' });
    response.end(bytes);
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(5371, '127.0.0.1', resolve);
  });
  for (const [name, engine, width, height] of [
    ['desktop', chromium, 1280, 900],
    ['narrow', webkit, 320, 812],
  ]) {
    browser = await engine.launch(engine === chromium ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
    const page = await browser.newPage({ viewport: { width, height }, isMobile: width < 768, hasTouch: width < 768, locale: 'en-GB', timezoneId: 'UTC' });
    page.setDefaultTimeout(10000);
    const record = { name, width, height, browser: browser.version(), errors: [], phases: [] };
    page.on('pageerror', (error) => record.errors.push(error.message));
    let phase = 'stale';
    await page.route('**/*', async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      assert.equal(url.origin, origin, 'external request');
      assert.equal(request.method(), 'GET', 'unexpected mutation');
      if (!url.pathname.startsWith('/api/')) return route.continue();
      result.requests.push({ viewport: name, phase, path: url.pathname });
      let body;
      if (url.pathname === '/api/alerts/active') body = frames[phase];
      else if (url.pathname === '/api/alerts/delivery-diagnosis') body = [];
      else if (url.pathname === '/api/notifications/health') body = { queue: { status: 'unavailable' } };
      else if (url.pathname === '/api/alerts/incidents') body = null;
      else throw new Error(`unexpected API read: ${url.pathname}`);
      return route.fulfill({ json: body });
    });
    await page.goto(origin + '/browser-tests/snapshot-date-observation.html');
    await page.waitForFunction(() => window.__snapshotDate?.generation() === 1);
    for (phase of ['stale', 'missing', 'future', 'recovered']) {
      if (phase !== 'stale') {
        const previous = await page.evaluate(() => window.__snapshotDate.generation());
        await page.getByRole('button', { name: 'Refresh monitoring evidence', exact: true }).click();
        await page.waitForFunction((generation) => window.__snapshotDate.generation() > generation, previous);
      }
      const active = await page.evaluate(() => Object.values(window.__snapshotDate.active()));
      assert.deepEqual(active, frames[phase], `producer/consumer mismatch in ${name}/${phase}`);
      if (phase !== 'recovered') {
        await page.getByText(frames.stale[0].message, { exact: true }).waitFor();
        const more = page.getByRole('button', { name: 'More', exact: true });
        if (await more.count()) {
          await more.click();
          await page.getByRole('button', { name: 'Less', exact: true }).click();
        }
      } else {
        assert.equal(await page.getByText(frames.stale[0].message, { exact: true }).count(), 0);
      }
      await page.evaluate(async () => {
        await document.fonts.ready;
        await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
      });
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth);
      assert.equal(overflow, false, `${name}/${phase} horizontal overflow`);
      const capture = `${name}-${phase}.png`;
      await page.screenshot({ path: path.join(output, capture), fullPage: true });
      result.captures.push({ file: capture, sha256: hash(fs.readFileSync(path.join(output, capture))) });
      record.phases.push({ phase, active: active.length, overflow });
    }
    assert.deepEqual(record.errors, []);
    result.cases.push(record);
    await browser.close();
    browser = undefined;
  }
  result.result = 'passed';
})().catch((error) => {
  result.result = 'failed';
  result.error = String(error.stack || error);
  process.exitCode = 1;
}).finally(async () => {
  if (browser) await browser.close();
  result.cleanup.browser = true;
  if (server) await new Promise((resolve) => server.close(resolve));
  result.cleanup.server = true;
  fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
});
