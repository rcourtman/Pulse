// Exact-source read-only observations -> production recovery mapper -> Coverage.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const compiled = path.join(root, 'tmp/coverage-workload-identity/static');
const output = path.join(root, 'tmp/coverage-workload-identity/final');
const origin = 'http://127.0.0.1:5302';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const result = {
  binding,
  protocol_sha256: hash(fs.readFileSync(__filename)),
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  requests: [],
  errors: [],
  unexpected: [],
  cleanup: {},
};
let browser,
  server,
  revision = 0,
  fleet = false;
function guests() {
  return Array.from({ length: fleet ? 200 : 3 }, (_, index) => ({
    id: `fixture-${index}`,
    name: index < 3 ? 'database' : `fleet-${String(index).padStart(3, '0')}`,
    type: 'vm',
    platformId: 'proxmox',
    status: 'online',
    sourceType: 'api',
    sources: ['proxmox'],
    platformScopes: ['proxmox-pve'],
    proxmox: {
      vmid: index === 0 || index === 2 ? 101 : index === 1 ? (revision ? 103 : 102) : 1000 + index,
      instance:
        index === 199
          ? undefined
          : index < 2
            ? 'production'
            : index === 2
              ? 'recovery'
              : index % 5 === 0
                ? `department-production-region-with-a-long-name-${index}`
                : 'production',
      node:
        index === 1 && revision
          ? 'pve-02.example.test'
          : index > 2 && index % 5 === 0
            ? 'pve-department-with-a-long-native-host-name.example.test'
            : 'pve-01.example.test',
      nodeDisplayName: 'Display node',
    },
  }));
}
const posture = () => ({
  data: guests().map((guest) => ({
    subjectResourceId: guest.id,
    state: 'unknown',
    freshness: 'unknown',
    verification: 'unknown',
    coverage: 'unknown',
    providerStates: [],
    repositoryResourceIds: [],
    evidenceIds: [],
    evaluatedAt: '2026-10-16T18:00:00Z',
  })),
  policy: {
    freshnessWindowSeconds: 604800,
    verificationWindowSeconds: 604800,
    requireVerification: true,
  },
});
const inventory = () => ({
  data: {
    backupTasks: guests()
      .filter((guest) => guest.proxmox.instance)
      .map((guest) => ({
        id: `task-${guest.id}`,
        instance: guest.proxmox.instance,
        node: guest.proxmox.node,
        vmid: guest.proxmox.vmid,
        type: 'vm',
        status: guest.id === 'fixture-1' ? 'failed' : 'OK',
        startTime: '2026-10-16T17:00:00Z',
      })),
    storageBackups: [],
    guestSnapshots: [],
  },
});
async function capture(page, name) {
  const file = path.join(output, name + '.png');
  await page.screenshot({ path: file });
  result.captures.push({ file: path.relative(root, file), sha256: hash(fs.readFileSync(file)) });
}
async function identity(page, width) {
  const facts = await page.locator('[data-proxmox-backup-row="coverage"]').evaluateAll((rows) =>
    rows.slice(0, 3).map((row) => ({
      key: row.dataset.proxmoxCoverageKey,
      text: row.textContent,
      target: row.querySelector('[data-backup-workload-target]')?.textContent.trim(),
      connection: row.querySelector('[data-backup-workload-connection]')?.textContent.trim(),
      node: row.querySelector('[data-backup-workload-node]')?.textContent.trim(),
    })),
  );
  assert.equal(facts.length, 3);
  assert.equal(new Set(facts.map((row) => row.text)).size, 3);
  assert.match(facts[0].connection, /production/);
  assert.match(facts[2].connection, /recovery/);
  assert.match(facts[0].text, /101/);
  assert.match(facts[1].text, revision ? /103/ : /102/);
  if (width < 480) assert.match(facts[1].node, revision ? /pve-02/ : /pve-01/);
  return facts;
}
async function geometry(page) {
  return page.evaluate(() => {
    const problems = [];
    for (const node of document.querySelectorAll(
      '[data-backup-workload-target], [data-backup-workload-connection], [data-backup-workload-node]',
    )) {
      const box = node.getBoundingClientRect();
      if (box.bottom < 0 || box.top > innerHeight) continue;
      const range = document.createRange();
      range.selectNodeContents(node);
      if (
        node.scrollWidth > node.clientWidth + 1 ||
        [...range.getClientRects()].some((r) => r.left < box.left - 1 || r.right > box.right + 1)
      )
        problems.push(node.textContent);
    }
    return { problems, overflow: document.documentElement.scrollWidth > innerWidth };
  });
}
async function scan(page, reverse) {
  const seen = new Set(),
    samples = [];
  for (let step = 0; step < 150; step++) {
    await page.waitForFunction(
      () => {
        const root = document.querySelector('[data-proxmox-backups-table="coverage"]');
        const rows = root?.querySelectorAll('[data-proxmox-backup-row="coverage"]');
        if (!rows?.length) return false;
        const body = root.querySelector('tbody').getBoundingClientRect();
        return (
          rows[0].getBoundingClientRect().top <= Math.max(0, body.top) + 1 &&
          rows[rows.length - 1].getBoundingClientRect().bottom >=
            Math.min(innerHeight, body.bottom) - 1
        );
      },
      undefined,
      { timeout: 2500 },
    );
    const sample = await page.evaluate((reverse) => {
      const rows = [...document.querySelectorAll('[data-proxmox-backup-row="coverage"]')];
      const max = document.documentElement.scrollHeight - innerHeight;
      return {
        scroll: scrollY,
        max,
        count: rows.length,
        visible: rows
          .filter((row) => {
            const box = row.getBoundingClientRect();
            return box.top >= 0 && box.bottom <= innerHeight;
          })
          .map((row) => row.dataset.proxmoxCoverageKey),
        done: reverse ? scrollY <= 1 : scrollY >= max - 1,
      };
    }, reverse);
    for (const key of sample.visible) seen.add(key);
    const bounds = await geometry(page);
    assert.deepEqual(bounds, { problems: [], overflow: false });
    samples.push(sample);
    assert(sample.count <= ((await page.viewportSize()).width < 640 ? 36 : 140));
    if (sample.done)
      return { seen: [...seen], maxMounted: Math.max(...samples.map((s) => s.count)), samples };
    await page.evaluate((reverse) => scrollBy(0, (reverse ? -1 : 1) * innerHeight * 0.55), reverse);
    await page.waitForTimeout(55);
  }
  throw new Error('Fleet scan did not reach its boundary');
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
    result.requests.push({ method: req.method, path: url.pathname, revision, fleet });
    const json = (body) => {
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify(body));
    };
    if (req.method !== 'GET') {
      result.unexpected.push({ method: req.method, path: url.pathname });
      res.statusCode = 405;
      return res.end();
    }
    if (url.pathname === '/fixture/workloads') {
      fleet = url.searchParams.has('fleet');
      return json(guests());
    }
    if (url.pathname === '/api/backups/pve') return json(inventory());
    if (url.pathname === '/api/backups/pbs') return json({ data: { backups: [] } });
    if (url.pathname === '/api/recovery/postures') return json(posture());
    const file = url.pathname.startsWith('/proxmox/backups/')
      ? '/browser-tests/coverage-workload-identity.html'
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
  await new Promise((resolve) => server.listen(5302, '127.0.0.1', resolve));
  for (const spec of [
    { name: 'desktop', engine: chromium, width: 1440, height: 1000, dark: false },
    { name: 'phone', engine: webkit, width: 320, height: 1000, dark: true },
  ]) {
    revision = 0;
    fleet = false;
    browser = await spec.engine.launch(
      spec.engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    const context = await browser.newContext({
      viewport: { width: spec.width, height: spec.height },
      hasTouch: spec.width === 320,
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
    await page.clock.setFixedTime(new Date('2026-10-16T18:00:00Z'));
    await page.goto(origin + '/proxmox/backups/coverage');
    const rows = page.locator('[data-proxmox-backup-row="coverage"]');
    await rows.nth(2).waitFor();
    await page.waitForFunction(() =>
      document
        .querySelector('[data-proxmox-backup-row="coverage"]')
        ?.textContent.includes('Unknown'),
    );
    const states = [{ width: spec.width, rows: await identity(page, spec.width) }];
    assert.deepEqual(await geometry(page), { problems: [], overflow: false });
    await capture(page, spec.name + '-identities');
    if (spec.name === 'desktop') {
      for (const width of [600, 800, 1000]) {
        await page.setViewportSize({ width, height: 1000 });
        await page.waitForTimeout(100);
        states.push({ width, rows: await identity(page, width) });
        assert.deepEqual(await geometry(page), { problems: [], overflow: false });
      }
      await page.setViewportSize({ width: spec.width, height: spec.height });
    }
    const second = rows.nth(1),
      toggle = second.locator('button').first();
    if (spec.width === 320) await second.tap();
    else {
      await toggle.focus();
      await page.keyboard.press('Enter');
    }
    const detail = page.locator('[data-inline-detail-for="resource:fixture-1"]');
    await detail.waitFor();
    assert.match(await detail.textContent(), /Connection: production/);
    await page.evaluate(() => {
      window.originalRow = document.querySelector(
        '[data-proxmox-coverage-key="resource:fixture-1"]',
      );
      window.originalToggle = window.originalRow.querySelector('button');
    });
    revision = 1;
    const load = page.getByRole('button', { name: 'Refresh workload observations', exact: true });
    await load.click();
    await page.waitForFunction(() =>
      document
        .querySelector('[data-proxmox-coverage-key="resource:fixture-1"]')
        .textContent.includes('103'),
    );
    states.push({ width: spec.width, refreshed: true, rows: await identity(page, spec.width) });
    assert(
      await page.evaluate(
        () =>
          window.originalRow ===
          document.querySelector('[data-proxmox-coverage-key="resource:fixture-1"]'),
      ),
    );
    assert.match(await detail.textContent(), /Target: 103/);
    await toggle.focus();
    const refresh = page.getByRole('button', { name: 'Refresh backups', exact: true });
    await refresh.focus();
    const before = result.requests.length;
    await page.keyboard.press('Enter');
    await page.waitForFunction(
      () =>
        document.querySelector('button[title^="Read Pulse"]')?.getAttribute('aria-disabled') ===
        'false',
    );
    const reads = result.requests.slice(before).filter((r) => r.path.startsWith('/api/'));
    assert.deepEqual(reads.map((r) => r.path).sort(), [
      '/api/backups/pbs',
      '/api/backups/pve',
      '/api/recovery/postures',
    ]);
    assert(await refresh.evaluate((element) => document.activeElement === element));
    assert(
      await page.evaluate(
        () =>
          window.originalRow ===
            document.querySelector('[data-proxmox-coverage-key="resource:fixture-1"]') &&
          window.originalToggle === window.originalRow.querySelector('button'),
      ),
    );
    await capture(page, spec.name + '-refreshed-detail');
    await toggle.click();
    await page.getByRole('button', { name: 'Load fleet observations', exact: true }).click();
    await page.waitForFunction(
      () =>
        document.querySelector('[data-proxmox-backups-table="coverage"]')?.dataset
          .proxmoxBackupsWindowed === 'true',
    );
    await page.evaluate(() => scrollTo(0, 0));
    const forward = await scan(page, false),
      backward = await scan(page, true);
    assert.equal(forward.seen.length, 200);
    assert.equal(backward.seen.length, 200);
    assert(forward.maxMounted <= (spec.width === 320 ? 36 : 140));
    assert(backward.maxMounted <= (spec.width === 320 ? 36 : 140));
    result.cases.push({
      name: spec.name,
      browser: browser.version(),
      viewport: { width: spec.width, height: spec.height },
      states,
      reads,
      forward,
      backward,
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
