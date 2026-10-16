// Only loopback GET fixtures. Full viewport reachability, not DOM-mount counts.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const compiled = path.join(root, 'tmp/backup-coverage-windowing/static');
const baseline = process.argv.includes('--baseline');
const output = path.join(root, `tmp/backup-coverage-windowing/${baseline ? 'baseline' : 'final'}`);
const origin = 'http://127.0.0.1:5298';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const result = {
  binding,
  baseline,
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  requests: [],
  unexpected: [],
  errors: [],
  cleanup: {},
};
let browser,
  server,
  revision = 0;
const providers = () =>
  Array.from({ length: revision ? 20 : 16 }, (_, index) => ({
    provider: 'proxmox-pbs',
    scope: `pbs-${String(index).padStart(2, '0')}/department-with-a-long-name/production`,
    source: 'pbs-backup-enumeration',
    jobState: index % 2 ? 'failed' : 'success',
    historyCompleteness: index % 2 ? 'unavailable' : 'complete',
    permissions: index % 2 ? 'denied' : 'sufficient',
    evidenceIds: [],
  }));
const posture = () => ({
  data: Array.from({ length: 200 }, (_, index) => ({
    subjectResourceId: `fixture-${index}`,
    state: 'unknown',
    freshness: 'unknown',
    verification: 'unknown',
    coverage: 'unknown',
    explanation: 'Backup evidence is not a restore or guest thaw.',
    providerStates: index === 0 ? providers() : [],
    repositoryResourceIds: [],
    evidenceIds: [],
    evaluatedAt: '2026-10-16T08:00:00Z',
  })),
  policy: {
    freshnessWindowSeconds: 604800,
    verificationWindowSeconds: 604800,
    requireVerification: true,
  },
});
async function capture(page, name) {
  const file = path.join(output, name + '.png');
  await page.screenshot({ path: file });
  result.captures.push({ file: path.relative(root, file), sha256: hash(fs.readFileSync(file)) });
}
async function readDetail(page, spec, expected, suffix) {
  const detail = page.locator('[data-inline-detail-for]').first();
  const size = await detail.boundingBox();
  const seen = new Set();
  let lost = false, captured = false;
  for (let y = 0; y <= size.y + size.height; y += spec.height * 0.45) {
    await page.evaluate((y) => scrollTo(0, y), y);
    await page.waitForTimeout(65);
    const visible = await page.locator('[data-backup-provider-scope]').evaluateAll((labels) => labels.filter((label) => {
      const box = label.getBoundingClientRect(); return box.top >= 0 && box.bottom <= innerHeight;
    }).map((label) => label.textContent.trim()));
    for (const text of visible) seen.add(text);
    if (!captured && y > size.y + size.height * 0.4) {
      await capture(page, spec.name + '-' + suffix); captured = true;
    }
    if (await page.locator('[data-backup-provider-evidence]').count() !== expected) { lost = true; break; }
  }
  return { size, detailLostWhileReading: lost, providerLabelsSeen: [...seen] };
}
async function visibleRows(page) {
  return page.locator('[data-proxmox-backup-row="coverage"]').evaluateAll((rows) =>
    rows
      .filter((row) => {
        const box = row.getBoundingClientRect();
        return box.top >= 0 && box.bottom <= innerHeight;
      })
      .map((row) => row.querySelector('.font-semibold').textContent.trim()),
  );
}
async function scan(page, reverse) {
  const seen = new Set();
  let maxMounted = 0;
  const samples = [];
  for (let step = 0; step < 100; step++) {
    const started = Date.now();
    // Native scroll events can arrive after the compositor moves the page,
    // especially in WebKit. Test settled reachability, not a fixed-delay race:
    // do not advance until the mounted runway covers the visible body.
    await page.waitForFunction(() => {
      const root = document.querySelector('[data-proxmox-backups-table="coverage"]');
      const rows = root?.querySelectorAll('[data-proxmox-backup-row="coverage"]');
      if (!rows?.length) return false;
      const body = root.querySelector('tbody').getBoundingClientRect();
      const first = rows[0].getBoundingClientRect();
      const last = rows[rows.length - 1].getBoundingClientRect();
      return first.top <= Math.max(0, body.top) + 1 && last.bottom >= Math.min(innerHeight, body.bottom) - 1;
    }, undefined, { timeout: 2000 });
    const settledAfterMs = Date.now() - started;
    const sample = await page.evaluate((reverse) => {
      const rows = [...document.querySelectorAll('[data-proxmox-backup-row="coverage"]')];
      const max = document.documentElement.scrollHeight - innerHeight;
      return {
        scroll: scrollY,
        max,
        count: rows.length,
        first: rows[0]?.dataset.proxmoxCoverageKey,
        last: rows[rows.length - 1]?.dataset.proxmoxCoverageKey,
        done: reverse ? scrollY <= 1 : scrollY >= max - 1,
      };
    }, reverse);
    samples.push({ step, settledAfterMs, ...sample, visible: await visibleRows(page) });
    for (const label of await visibleRows(page)) seen.add(label);
    maxMounted = Math.max(maxMounted, sample.count);
    if (sample.done) break;
    await page.evaluate((reverse) => scrollBy(0, (reverse ? -1 : 1) * innerHeight * 0.55), reverse);
    await page.waitForTimeout(45);
  }
  return { seen: [...seen].sort(), maxMounted, samples };
}
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
  const assets = new Map();
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, file);
    assets.set('/' + file, bytes);
  }
  server = http.createServer((req, res) => {
    const url = new URL(req.url, origin);
    result.requests.push({ method: req.method, path: url.pathname, revision });
    const json = (body) => {
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify(body));
    };
    if (req.method !== 'GET') {
      result.unexpected.push({ method: req.method });
      res.statusCode = 405;
      return res.end();
    }
    if (url.pathname === '/api/backups/pbs') return json({ data: { backups: [] } });
    if (url.pathname === '/api/backups/pve')
      return json({ data: { backupTasks: [], storageBackups: [], guestSnapshots: [] } });
    if (url.pathname === '/api/recovery/postures') return json(posture());
    const file = url.pathname.startsWith('/proxmox/backups/')
      ? '/browser-tests/backup-coverage-windowing.html'
      : url.pathname;
    if (!assets.has(file)) {
      result.unexpected.push({ path: url.pathname });
      res.statusCode = 404;
      return res.end();
    }
    res.setHeader(
      'Content-Type',
      { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css' }[path.extname(file)] ||
        'application/octet-stream',
    );
    res.end(assets.get(file));
  });
  await new Promise((resolve) => server.listen(5298, '127.0.0.1', resolve));
  for (const spec of [
    { name: 'desktop', engine: chromium, width: 1365, height: 900, dark: false },
    { name: 'phone', engine: chromium, width: 320, height: 760, dark: true },
    { name: 'phone-webkit', engine: webkit, width: 320, height: 760, dark: false },
  ].filter((spec) => !process.argv.includes('--webkit-only') || spec.name === 'phone-webkit')) {
    revision = 0;
    browser = await spec.engine.launch(
      spec.engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    const context = await browser.newContext({
      viewport: { width: spec.width, height: spec.height },
      hasTouch: spec.width < 640,
      locale: 'en-GB',
    });
    await context.route('**/*', (route) => {
      if (new URL(route.request().url()).origin === origin) return route.continue();
      result.unexpected.push({ offOrigin: route.request().url() });
      return route.abort();
    });
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    page.on('pageerror', (error) => result.errors.push(String(error)));
    page.on('console', (message) => {
      if (message.type() === 'error') result.errors.push(message.text());
    });
    if (spec.dark)
      await page.addInitScript(() =>
        document.addEventListener(
          'DOMContentLoaded',
          () => document.documentElement.classList.add('dark'),
          { once: true },
        ),
      );
    await page.goto(origin + '/proxmox/backups/coverage', { waitUntil: 'networkidle' });
    const toggle = page.getByRole('button', { name: /^Expand details for workload-000/ });
    await toggle.waitFor();
    if (spec.width < 640) await page.locator('[data-proxmox-backup-row="coverage"]').first().tap();
    else {
      await toggle.focus();
      await page.keyboard.press('Enter');
    }
    await page.locator('[data-backup-provider-evidence]').last().waitFor();
    await capture(page, spec.name + '-expanded');
    const reading = await readDetail(page, spec, 16, 'detail-scroll');
    const account = { name: spec.name, browser: browser.version(), ...reading, refreshed: null, forward: null, reverse: null };
    result.cases.push(account);
    if (!baseline) {
      assert(!reading.detailLostWhileReading, `${spec.name}: expanded detail disappeared while reading`);
      assert.equal(reading.providerLabelsSeen.length, 16, `${spec.name}: provider labels never reached viewport`);
      // Ordinary same-open HTTP refresh grows the existing detail. It must
      // remeasure without waiting for the operator to collapse/reopen it.
      await page.evaluate(() => scrollTo(0, 0)); await page.waitForTimeout(150);
      revision = 1;
      const refresh = page.getByRole('button', { name: 'Refresh backups', exact: true });
      await refresh.focus(); await page.keyboard.press('Enter');
      await page.waitForFunction(() => document.querySelectorAll('[data-backup-provider-evidence]').length === 20);
      assert.equal(await refresh.evaluate((button) => button === document.activeElement), true);
      account.refreshed = await readDetail(page, spec, 20, 'refreshed-detail');
      assert(!account.refreshed.detailLostWhileReading, `${spec.name}: refreshed detail disappeared while reading`);
      assert.equal(account.refreshed.providerLabelsSeen.length, 20);
    }
    await page.evaluate(() => scrollTo(0, 0));
    await page.waitForTimeout(150);
    const hide = page.getByRole('button', { name: /^Collapse details for workload-000/ });
    if (await hide.count()) {
      if (spec.width < 640)
        await page.locator('[data-proxmox-backup-row="coverage"]').first().tap();
      else await hide.click();
    }
    account.forward = await scan(page, false);
    await capture(page, spec.name + '-last');
    account.reverse = await scan(page, true);
    if (!baseline) {
      assert.equal(account.forward.seen.length, 200, `${spec.name}: forward summaries missed`);
      assert.equal(account.reverse.seen.length, 200, `${spec.name}: reverse summaries missed`);
      assert(account.forward.maxMounted <= (spec.width < 640 ? 36 : 140));
      assert(account.reverse.maxMounted <= (spec.width < 640 ? 36 : 140));
    }
    await context.close();
    await browser.close();
    browser = undefined;
  }
  assert.deepEqual(result.errors, []);
  assert.deepEqual(result.unexpected, []);
})()
  .catch((error) => {
    result.failure = String(error.stack || error);
    process.exitCode = 1;
  })
  .finally(async () => {
    if (browser) await browser.close();
    result.cleanup.browserClosed = true;
    if (server) await new Promise((resolve) => server.close(resolve));
    result.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        baseline,
        cases: result.cases.map(
          ({ name, detailLostWhileReading, providerLabelsSeen, forward, reverse }) => ({
            name,
            detailLostWhileReading,
            labelsSeen: providerLabelsSeen.length,
            forward: forward?.seen.length,
            reverse: reverse?.seen.length,
          }),
        ),
        failure: result.failure,
        cleanup: result.cleanup,
      }),
    );
  });
