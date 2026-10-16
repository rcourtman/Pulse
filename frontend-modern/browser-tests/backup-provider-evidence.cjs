// Synthetic HTTP -> production posture reader/model/search -> existing Coverage.
// No backup, provider, guest, or off-origin operation is permitted.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const compiled = path.join(root, 'tmp/backup-point-state/static');
const output = path.join(root, 'tmp/backup-provider-evidence');
const origin = 'http://127.0.0.1:5297';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const westScope = 'pbs-west/department-with-a-long-name/production';
const result = {
  binding,
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
const providers = () => [
  {
    provider: 'proxmox-pbs',
    scope: 'pbs-east',
    source: 'pbs-backup-enumeration',
    jobState: 'success',
    historyCompleteness: 'complete',
    permissions: 'sufficient',
    evidenceIds: [],
  },
  {
    provider: 'proxmox-pbs',
    scope: westScope,
    source: 'pbs-backup-enumeration',
    jobState: revision ? 'success' : 'failed',
    historyCompleteness: revision ? 'complete' : 'unavailable',
    permissions: revision ? 'sufficient' : 'denied',
    evidenceIds: [],
  },
  {
    provider: 'proxmox-pbs',
    scope: westScope,
    source: 'legacy-recovery-point',
    jobState: 'unknown',
    historyCompleteness: 'unknown',
    permissions: 'unknown',
    evidenceIds: [],
  },
];
const posture = () => ({
  data: [
    {
      subjectResourceId: 'fixture-vm',
      state: 'unknown',
      freshness: 'unknown',
      verification: 'unknown',
      coverage: 'unknown',
      explanation: 'Some provider history is unavailable.',
      providerStates: revision ? providers().reverse() : providers(),
      repositoryResourceIds: [],
      evidenceIds: [],
      evaluatedAt: '2026-10-16T07:00:00Z',
    },
  ],
  policy: {
    freshnessWindowSeconds: 604800,
    verificationWindowSeconds: 604800,
    requireVerification: true,
  },
});
async function capture(page, name) {
  const file = path.join(output, `${name}.png`);
  await page.screenshot({ path: file, fullPage: true });
  result.captures.push({ file: path.relative(root, file), sha256: hash(fs.readFileSync(file)) });
}
async function inspect(page) {
  const facts = await page.locator('[data-backup-provider-evidence]').evaluateAll((cards) =>
    cards.map((card) => ({
      scope: card.querySelector('[data-backup-provider-scope]').textContent.trim(),
      source: card.querySelector('[data-backup-provider-source]').textContent.trim(),
      text: card.textContent,
      labels: [...card.querySelectorAll('dd')].map((label) => {
        const range = document.createRange();
        range.selectNodeContents(label);
        const box = label.getBoundingClientRect();
        return {
          text: label.textContent,
          width: label.clientWidth,
          scroll: label.scrollWidth,
          left: box.left,
          right: box.right,
          glyphs: [...range.getClientRects()].map((rect) => ({
            left: rect.left,
            right: rect.right,
          })),
        };
      }),
    })),
  );
  assert.equal(facts.length, 3);
  for (const provider of providers()) {
    const card = facts.find(
      (item) => item.scope === provider.scope && item.source === provider.source,
    );
    assert(card, JSON.stringify(provider));
    assert.match(card.text, /Result (Success|Failed|Unknown)/);
    assert(!card.text.includes('Job Success'));
    assert(
      card.text.includes(
        `Access ${provider.permissions[0].toUpperCase()}${provider.permissions.slice(1)}`,
      ),
    );
    assert(
      card.text.includes(
        `History ${provider.historyCompleteness[0].toUpperCase()}${provider.historyCompleteness.slice(1)}`,
      ),
    );
    for (const label of card.labels) {
      assert(label.scroll <= label.width + 1, JSON.stringify(label));
      for (const glyph of label.glyphs)
        assert(
          glyph.left >= label.left - 1 && glyph.right <= label.right + 1,
          JSON.stringify(label),
        );
    }
  }
  const overflow = await page.evaluate(() => ({
    width: innerWidth,
    scroll: document.documentElement.scrollWidth,
  }));
  assert(overflow.scroll <= overflow.width, JSON.stringify(overflow));
  return { facts, overflow };
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
    assert.equal(
      hash(fs.readFileSync(path.join(root, 'frontend-modern', file))),
      digest,
      `Source changed: ${file}`,
    );
  const assets = new Map();
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, `Artifact changed: ${file}`);
    assets.set('/' + file, bytes);
  }
  server = http.createServer((req, res) => {
    const url = new URL(req.url, origin);
    result.requests.push({ method: req.method, path: url.pathname, query: url.search, revision });
    const json = (body) => {
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify(body));
    };
    if (req.method !== 'GET') {
      result.unexpected.push({ method: req.method, path: url.pathname });
      res.statusCode = 405;
      return res.end();
    }
    if (url.pathname === '/api/backups/pbs') return json({ data: { backups: [] } });
    if (url.pathname === '/api/backups/pve')
      return json({ data: { backupTasks: [], storageBackups: [], guestSnapshots: [] } });
    if (url.pathname === '/api/recovery/postures') return json(posture());
    const file = url.pathname.startsWith('/proxmox/backups/')
      ? '/browser-tests/backup-point-state.html'
      : url.pathname;
    const bytes = assets.get(file);
    if (!bytes) {
      result.unexpected.push({ path: url.pathname });
      res.statusCode = 404;
      return res.end('Not bound');
    }
    res.setHeader(
      'Content-Type',
      { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css' }[path.extname(file)] ||
        'application/octet-stream',
    );
    res.end(bytes);
  });
  await new Promise((resolve) => server.listen(5297, '127.0.0.1', resolve));
  for (const spec of [
    { name: 'desktop-light', engine: chromium, width: 1365, height: 1000, dark: false },
    { name: 'phone-dark', engine: chromium, width: 320, height: 1000, dark: true },
    { name: 'phone-webkit-light', engine: webkit, width: 320, height: 1000, dark: false },
  ]) {
    revision = 0;
    browser = await spec.engine.launch(
      spec.engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    const context = await browser.newContext({
      viewport: { width: spec.width, height: spec.height },
      hasTouch: spec.width < 768,
      locale: 'en-GB',
    });
    await context.route('**/*', (route) => {
      if (new URL(route.request().url()).origin === origin) return route.continue();
      result.unexpected.push({ offOrigin: route.request().url() });
      return route.abort();
    });
    await context.addInitScript(
      (dark) =>
        addEventListener(
          'DOMContentLoaded',
          () => document.documentElement.classList.toggle('dark', dark),
          { once: true },
        ),
      spec.dark,
    );
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    page.on('pageerror', (error) => result.errors.push(String(error)));
    page.on('console', (message) => {
      if (message.type() === 'error') result.errors.push(message.text());
    });
    await page.clock.setFixedTime(new Date('2026-10-16T07:00:00Z'));
    await page.goto(origin + '/proxmox/backups/coverage');
    const toggle = page.getByRole('button', { name: /Expand details for recovery-vm/ });
    await toggle.waitFor();
    if (spec.width < 768) await page.locator('[data-proxmox-backup-row="coverage"]').tap();
    else {
      await toggle.focus();
      await page.keyboard.press('Enter');
    }
    await page.locator('[data-backup-provider-scope]').first().waitFor();
    await page
      .getByText('No restore evidence has been discovered for this workload.', { exact: true })
      .waitFor();
    const initial = await inspect(page);
    await capture(page, `${spec.name}-initial`);
    const search = page.getByPlaceholder('Search backups by workload, node, source, or status');
    const searches = [];
    for (const term of ['pbs-west', 'legacy-recovery-point', 'denied', 'pbs-not-observed']) {
      await search.fill(term);
      if (term === 'pbs-not-observed')
        await page
          .getByText('No workload coverage rows match current filters', { exact: true })
          .waitFor();
      else await page.locator('[data-proxmox-backup-row="coverage"]').waitFor();
      searches.push({
        term,
        rows: await page.locator('[data-proxmox-backup-row="coverage"]').count(),
      });
    }
    assert.deepEqual(
      searches.map((item) => item.rows),
      [1, 1, 1, 0],
    );
    await search.fill('');
    await page.locator('[data-backup-provider-scope]').first().waitFor();
    revision = 1;
    const before = result.requests.length;
    const refresh = page.getByRole('button', { name: 'Refresh backups', exact: true });
    await refresh.focus();
    await page.keyboard.press('Enter');
    await page.waitForFunction(
      () =>
        !document.body.textContent.includes('Access Denied') &&
        document.querySelectorAll('[data-backup-provider-evidence]').length === 3,
    );
    await page.getByRole('button', { name: 'Refresh backups', exact: true }).waitFor();
    assert.equal(await refresh.evaluate((element) => document.activeElement === element), true);
    const refreshed = await inspect(page);
    assert.equal(refreshed.facts[0].source, 'legacy-recovery-point');
    await capture(page, `${spec.name}-refreshed`);
    const reads = result.requests
      .slice(before)
      .filter((request) => request.path.startsWith('/api/'));
    assert.equal(reads.length, 3);
    assert.equal(reads.filter((request) => request.path === '/api/recovery/postures').length, 1);
    assert.match(await page.locator('[data-proxmox-backup-row="coverage"]').innerText(), /Unknown/);
    result.cases.push({
      name: spec.name,
      browser: browser.version(),
      viewport: { width: spec.width, height: spec.height },
      initial,
      searches,
      refreshed,
      refreshReads: reads,
    });
    await context.close();
    await browser.close();
    browser = undefined;
  }
  assert.deepEqual(result.errors, []);
  assert.deepEqual(result.unexpected, []);
  result.passed = true;
})()
  .catch((error) => {
    result.failed = String(error.stack || error);
    process.exitCode = 1;
  })
  .finally(async () => {
    if (browser) await browser.close();
    result.cleanup.browserClosed = true;
    if (server) await new Promise((resolve) => server.close(resolve));
    result.cleanup.serverClosed = true;
    fs.mkdirSync(output, { recursive: true });
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    process.stdout.write(
      JSON.stringify({
        passed: result.passed || false,
        cases: result.cases.length,
        captures: result.captures.length,
        failed: result.failed,
        cleanup: result.cleanup,
      }) + '\n',
    );
  });
