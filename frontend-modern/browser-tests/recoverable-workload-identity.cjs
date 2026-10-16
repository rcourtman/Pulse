// Synthetic observations -> real inventory reader/recovery mapper -> By date.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const compiled = path.join(root, 'tmp/recoverable-workload-identity/static');
const output = path.join(root, 'tmp/recoverable-workload-identity/final');
const origin = 'http://127.0.0.1:5303';
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
      vmid: index === 0 || index === 2 ? 100 : index === 1 ? (revision ? 102 : 101) : 1000 + index,
      instance:
        index === 199
          ? undefined
          : index === 2
            ? 'west'
            : index > 2 && index % 7 === 0
              ? `department-with-a-long-connection-name-region-${index}`
              : 'east',
      node:
        index === 1 && revision
          ? 'pve-updated.example.test'
          : index === 2
            ? 'pve-west.example.test'
            : index > 2 && index % 7 === 0
              ? 'pve-department-with-a-long-native-host-name.example.test'
              : 'pve-east.example.test',
      nodeDisplayName: 'Display node',
    },
  }));
}
const time = (index) => `2026-10-${String(16 - Math.floor(index / 40)).padStart(2, '0')}T10:00:00Z`;
function inventory() {
  return {
    data: {
      backupTasks: [],
      storageBackups: guests()
        .filter((g) => g.id !== 'fixture-0' && g.id !== 'fixture-2')
        .map((g) => {
          const index = Number(g.id.split('-')[1]);
          return {
            id: `point-${index}`,
            instance: g.proxmox.instance || '',
            node: g.proxmox.node,
            vmid: g.proxmox.vmid,
            type: 'vm',
            storage: 'local',
            time: time(index),
            size: 2048,
            format: 'zst',
            protected: false,
            isPBS: false,
            volid: `local:backup/point-${index}.vma.zst`,
          };
        }),
      guestSnapshots: [
        {
          id: 'point-2',
          instance: 'west',
          node: 'pve-west.example.test',
          type: 'vm',
          vmid: 100,
          time: time(2),
          name: 'before-update',
        },
      ],
    },
  };
}
function pbs() {
  const point = {
    id: 'point-0',
    instance: 'central-backups',
    datastore: 'main',
    namespace: 'east',
    backupType: 'vm',
    vmid: '100',
    backupTime: time(0),
    size: 2048,
    files: ['index.json.blob'],
    protected: true,
    verified: true,
  };
  return {
    data: {
      backups: [
        point,
        ...(!fleet
          ? [{ ...point, id: 'unmatched', vmid: '999', namespace: 'not-an-observed-workload-node' }]
          : []),
      ],
    },
  };
}
const posture = () => ({
  data: guests().map((g) => ({
    subjectResourceId: g.id,
    state: 'unknown',
    freshness: 'unknown',
    verification: 'unknown',
    coverage: 'unknown',
    explanation: 'Synthetic observations, not recovery acceptance.',
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
async function capture(page, name) {
  await page
    .locator('[data-proxmox-backups-table="recoverable"]')
    .evaluate((table) => table.scrollIntoView({ block: 'start' }));
  const file = path.join(output, name + '.png');
  await page.screenshot({ path: file });
  result.captures.push({ file: path.relative(root, file), sha256: hash(fs.readFileSync(file)) });
}
async function identity(page) {
  const rows = await page.locator('[data-proxmox-backup-row="recoverable"]').evaluateAll((nodes) =>
    nodes.map((row) => ({
      key: row.dataset.proxmoxBackupKey,
      // Phone provenance is a second DOM row in the same logical point.
      text:
        row.textContent +
        (row.nextElementSibling?.getAttribute('data-proxmox-backup-row') === 'location'
          ? row.nextElementSibling.textContent
          : ''),
      target: row.querySelector('[data-backup-point-workload-target]')?.textContent.trim(),
      connection: row.querySelector('[data-backup-point-workload-connection]')?.textContent.trim(),
      node: row.querySelector('[data-backup-point-workload-node]')?.textContent.trim(),
    })),
  );
  assert.equal(rows.length, 4);
  const point = (id) => rows.find((row) => row.key === `artifact:${id}`);
  assert.match(point('pbs:point-0').connection, /Connection: east/);
  assert.match(point('snapshot:point-2').connection, /Connection: west/);
  assert.match(point('archive:point-1').text, revision ? /102/ : /101/);
  assert.match(point('archive:point-1').node, revision ? /pve-updated/ : /pve-east/);
  assert.match(point('pbs:point-0').text, /central-backups/);
  assert.match(point('pbs:unmatched').connection, /Connection: Unknown/);
  assert.equal(point('pbs:unmatched').node, undefined);
  const layout = await page
    .locator('[data-proxmox-backups-table="recoverable"]')
    .getAttribute('data-proxmox-backups-layout');
  for (const row of rows) {
    if (['compact', 'basic', 'operational'].includes(layout)) assert.match(row.target, /VM \d+/);
    else assert.equal(row.target, undefined);
    if (row.node) assert(!row.node.includes('Display node'));
  }
  return { layout, rows };
}
async function geometry(page) {
  return page.evaluate(() => {
    const problems = [];
    for (const node of document.querySelectorAll(
      '[data-backup-point-workload-target], [data-backup-point-workload-connection], [data-backup-point-workload-node], [data-backup-location]',
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
async function scan(page, reverse, budget) {
  const seen = new Set(),
    samples = [];
  for (let step = 0; step < 180; step++) {
    await page.waitForFunction(
      () => {
        const root = document.querySelector('[data-proxmox-backups-table="recoverable"]'),
          body = root?.querySelector('tbody');
        const rows = body?.querySelectorAll(':scope > tr[data-proxmox-backup-key]');
        if (!rows?.length) return false;
        const first = rows[0].getBoundingClientRect(),
          lastRow = rows[rows.length - 1];
        const last =
          lastRow.nextElementSibling?.getAttribute('data-proxmox-backup-row') === 'location'
            ? lastRow.nextElementSibling.getBoundingClientRect()
            : lastRow.getBoundingClientRect();
        const box = body.getBoundingClientRect();
        return (
          first.top <= Math.max(0, box.top) + 1 &&
          last.bottom >= Math.min(innerHeight, box.bottom) - 1
        );
      },
      undefined,
      { timeout: 2500 },
    );
    const sample = await page.evaluate((reverse) => {
      const root = document.querySelector('[data-proxmox-backups-table="recoverable"]');
      const points = [...root.querySelectorAll('[data-proxmox-backup-row="recoverable"]')];
      const max = document.documentElement.scrollHeight - innerHeight;
      return {
        scroll: scrollY,
        max,
        mounted: root.querySelectorAll('[data-proxmox-backup-key]').length,
        visible: points
          .filter((row) => {
            const box = row.getBoundingClientRect(),
              location = row.nextElementSibling;
            const end =
              location?.getAttribute('data-proxmox-backup-row') === 'location'
                ? location.getBoundingClientRect().bottom
                : box.bottom;
            return box.top >= 0 && end <= innerHeight;
          })
          .map((row) => row.dataset.proxmoxBackupKey),
        done: reverse ? scrollY <= 1 : scrollY >= max - 1,
      };
    }, reverse);
    sample.visible.forEach((key) => seen.add(key));
    samples.push(sample);
    assert(sample.mounted <= budget);
    assert.deepEqual(await geometry(page), { problems: [], overflow: false });
    if (sample.done)
      return { seen: [...seen], maxMounted: Math.max(...samples.map((s) => s.mounted)), samples };
    await page.evaluate((reverse) => scrollBy(0, (reverse ? -1 : 1) * innerHeight * 0.5), reverse);
    await page.waitForTimeout(55);
  }
  throw new Error('Point scan did not reach its boundary');
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
    const json = (data) => {
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify(data));
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
    if (url.pathname === '/api/backups/pbs') return json(pbs());
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
  await new Promise((resolve) => server.listen(5303, '127.0.0.1', resolve));
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
    await page.goto(origin + '/proxmox/backups/date');
    await page.locator('[data-proxmox-backup-row="recoverable"]').nth(3).waitFor();
    const states = [{ width: spec.width, ...(await identity(page)) }];
    assert.deepEqual(await geometry(page), { problems: [], overflow: false });
    await capture(page, spec.name + '-points');
    if (spec.name === 'desktop') {
      for (const width of [600, 800, 1000]) {
        await page.setViewportSize({ width, height: 1000 });
        await page.waitForTimeout(100);
        states.push({ width, ...(await identity(page)) });
        assert.deepEqual(await geometry(page), { problems: [], overflow: false });
      }
      await page.setViewportSize({ width: spec.width, height: spec.height });
    }
    revision = 1;
    await Promise.all([
      page.waitForResponse((response) => new URL(response.url()).pathname === '/fixture/workloads'),
      page.getByRole('button', { name: 'Refresh workload observations', exact: true }).click(),
    ]);
    const refresh = page.getByRole('button', { name: 'Refresh backups', exact: true });
    await refresh.focus();
    const before = result.requests.length;
    await page.keyboard.press('Enter');
    await page.waitForFunction(
      () =>
        document.querySelector('button[title^="Read Pulse"]')?.getAttribute('aria-disabled') ===
        'false',
    );
    await page.waitForFunction(() =>
      document
        .querySelector('[data-proxmox-backup-key="artifact:archive:point-1"]')
        ?.textContent.includes('102'),
    );
    const reads = result.requests.slice(before).filter((r) => r.path.startsWith('/api/'));
    assert.deepEqual(reads.map((r) => r.path).sort(), [
      '/api/backups/pbs',
      '/api/backups/pve',
      '/api/recovery/postures',
    ]);
    assert(await refresh.evaluate((el) => document.activeElement === el));
    states.push({ width: spec.width, refreshed: true, ...(await identity(page)) });
    const search = page.getByPlaceholder('Search backups by workload, node, source, or status');
    await search.fill('102');
    await page.waitForFunction(
      () => document.querySelectorAll('[data-proxmox-backup-row="recoverable"]').length === 1,
    );
    assert.match(
      await page.locator('[data-proxmox-backup-row="recoverable"]').textContent(),
      /pve-updated/,
    );
    await search.fill('');
    await page.locator('[data-proxmox-backup-row="recoverable"]').nth(3).waitFor();
    await capture(page, spec.name + '-refreshed');
    await Promise.all([
      page.waitForResponse((response) => new URL(response.url()).pathname === '/fixture/workloads'),
      page.getByRole('button', { name: 'Load fleet observations', exact: true }).click(),
    ]);
    await refresh.click();
    await page.waitForFunction(
      () =>
        document.querySelector('[data-proxmox-backups-table="recoverable"]')?.dataset
          .proxmoxBackupsWindowed === 'true',
    );
    await page.evaluate(() => scrollTo(0, 0));
    const budget = spec.width === 320 ? 36 : 140;
    const forward = await scan(page, false, budget),
      backward = await scan(page, true, budget);
    assert.equal(forward.seen.length, 200);
    assert.equal(backward.seen.length, 200);
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
