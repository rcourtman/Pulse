// Safe synthetic HTTP -> production inventory decoder/model -> both backup
// tables and CSS. No backup, restore, guest probe or recovery command.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, firefox } = require('playwright');

const root = '/workspace';
const compiled = path.join(root, 'tmp/backup-point-state/static');
const output = path.join(root, 'tmp/backup-artifact-details');
const origin = 'http://127.0.0.1:5348';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const prefix = 'local:backup/' + 'long-directory-'.repeat(6);
const fileName = (index, revision = 0) =>
  `${prefix}vzdump-qemu-101-${String(index).padStart(3, '0')}${revision ? '-refreshed' : ''}.vma.zst`;
const snapshotDescription = 'Before maintenance: ' + 'snapshot-context-'.repeat(10);
let revision = 0;
let large = false;
let browser, server, page;
const result = {
  passed: false,
  source: binding.source_sha,
  playwright: require('playwright/package.json').version,
  compiled_inputs: Object.keys(binding.sources).length,
  artifacts: Object.keys(binding.artifacts).length,
  runner_sha256: hash(fs.readFileSync(__filename)),
  cases: [],
  captures: [],
  requests: [],
  errors: [],
  unexpected: [],
  cleanup: {},
  limits:
    'Synthetic GET observations through real readers/model/tables/CSS. No appliance, backup/restore, independent guest thaw, installed recovery, publication or release acceptance. The separately owned source-free WebKit runtime fault is not replayed.',
};
const pve = () => ({
  data: {
    backupTasks: [],
    storageBackups: Array.from({ length: large ? 162 : 2 }, (_, index) => ({
      id: `archive-${index}`,
      instance: 'fixture-pve',
      node: 'pve-a',
      storageNode: 'pve-a',
      type: 'qemu',
      vmid: 101,
      storage: 'local',
      format: 'zst',
      time: '2026-10-15T10:00:00Z',
      size: 1024,
      protected: false,
      volid: fileName(index, index === 0 ? revision : 0),
    })),
    guestSnapshots: [
      {
        id: 'snapshot',
        instance: 'fixture-pve',
        node: 'pve-a',
        type: 'qemu',
        vmid: 101,
        name: 'before-maintenance',
        description: snapshotDescription,
        time: '2026-10-15T09:00:00Z',
      },
    ],
  },
});
const pbs = {
  data: {
    backups: [
      {
        id: 'pbs-point',
        instance: 'pbs-east',
        datastore: 'main',
        namespace: 'pve-a',
        backupType: 'vm',
        vmid: '101',
        backupTime: '2026-10-15T08:00:00Z',
        files: ['index.json.blob'],
        protected: true,
        verified: false,
      },
    ],
  },
};
const posture = {
  data: [
    {
      subjectResourceId: 'fixture-vm',
      state: 'attention',
      freshness: 'current',
      verification: 'unverified',
      coverage: 'complete',
      providerStates: [],
      repositoryResourceIds: [],
      evidenceIds: [],
      evaluatedAt: '2026-10-15T12:00:00Z',
      explanation: 'Successful point verification has not been reported.',
    },
  ],
  policy: {
    freshnessWindowSeconds: 604800,
    verificationWindowSeconds: 604800,
    requireVerification: true,
  },
};

const dateTable = () => page.locator('[data-proxmox-backups-table="recoverable"]');
const coverageDetail = () => page.locator('[data-inline-detail-for="resource:fixture-vm"]');
async function activate(control, touch) {
  if (touch) await control.tap();
  else {
    await control.focus();
    await page.keyboard.press('Enter');
  }
}
async function inspect(scope) {
  const details = scope.locator('[data-backup-artifact-details]');
  assert.equal(await details.count(), 4);
  assert.deepEqual(
    (await scope.locator('[data-backup-artifact-identifier]').allTextContents())
      .map((s) => s.trim())
      .sort(),
    [fileName(0, revision), fileName(1)].sort(),
  );
  assert.equal(await scope.getByText(snapshotDescription, { exact: true }).count(), 1);
  assert.equal(await scope.getByText('1 PBS file', { exact: true }).count(), 1);
  const geometry = await details.evaluateAll((nodes) =>
    nodes.map((node) => {
      const cell = node.closest('td');
      const box = cell.getBoundingClientRect();
      const range = document.createRange();
      range.selectNodeContents(node);
      const style = getComputedStyle(node);
      return {
        text: node.textContent.trim(),
        width: node.clientWidth,
        scroll: node.scrollWidth,
        height: node.getBoundingClientRect().height,
        whiteSpace: style.whiteSpace,
        overflowWrap: style.overflowWrap,
        textOverflow: style.textOverflow,
        colspan: cell.colSpan,
        columns: cell.closest('table').querySelector('thead tr').children.length,
        contained: [...range.getClientRects()]
          .filter((r) => r.width && r.height)
          .every(
            (r) =>
              r.left >= box.left - 1 &&
              r.right <= box.right + 1 &&
              r.top >= box.top - 1 &&
              r.bottom <= box.bottom + 1,
          ),
      };
    }),
  );
  for (const item of geometry) {
    assert(item.width > 0 && item.height > 0 && item.contained, JSON.stringify(item));
    assert(item.scroll <= item.width + 1, JSON.stringify(item));
    assert.equal(item.whiteSpace, 'normal');
    assert.equal(item.overflowWrap, 'anywhere');
    assert.notEqual(item.textOverflow, 'ellipsis');
    if (item.colspan > 1) assert.equal(item.colspan, item.columns);
  }
  assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
  return geometry;
}
async function capture(name, fullPage = true) {
  const file = path.join(output, name + '.jpg');
  await page.screenshot({
    path: file,
    type: 'jpeg',
    quality: 80,
    fullPage,
    animations: 'disabled',
  });
  result.captures.push({ file: path.relative(root, file), sha256: hash(fs.readFileSync(file)) });
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
  for (const file of [
    'ProxmoxRecoverableTable.tsx',
    'ProxmoxCoverageTable.tsx',
    'proxmoxBackupsTableShared.tsx',
    'proxmoxBackupRecoveryModel.ts',
    'proxmoxBackupsTablePresentation.ts',
  ])
    assert(binding.sources['src/features/proxmox/' + file], `Missing real consumer ${file}`);
  const assets = new Map();
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, file);
    assets.set('/' + file, bytes);
  }
  server = http.createServer((req, res) => {
    const url = new URL(req.url, origin);
    result.requests.push({
      method: req.method,
      path: url.pathname,
      query: url.search,
      revision,
      large,
    });
    const json = (data) => {
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify(data));
    };
    if (req.method !== 'GET') {
      result.unexpected.push({ method: req.method, path: url.pathname });
      res.writeHead(405);
      return res.end();
    }
    if (url.pathname === '/api/backups/pve') return json(pve());
    if (url.pathname === '/api/backups/pbs') return json(pbs);
    if (url.pathname === '/api/recovery/postures') return json(posture);
    if (url.pathname === '/favicon.ico') {
      res.writeHead(204);
      return res.end();
    }
    const file = url.pathname.startsWith('/proxmox/backups/')
      ? '/browser-tests/backup-point-state.html'
      : url.pathname;
    const bytes = assets.get(file);
    if (!bytes) {
      result.unexpected.push({ path: url.pathname });
      res.writeHead(404);
      return res.end('Not bound');
    }
    res.setHeader(
      'Content-Type',
      {
        '.html': 'text/html',
        '.js': 'text/javascript',
        '.css': 'text/css',
        '.md': 'text/markdown',
      }[path.extname(file)] || 'application/octet-stream',
    );
    res.end(bytes);
  });
  await new Promise((resolve) => server.listen(5348, '127.0.0.1', resolve));
  for (const spec of [
    { name: 'desktop', engine: chromium, width: 1440, touch: false, dark: false },
    { name: 'phone', engine: chromium, width: 320, touch: true, dark: true },
    { name: 'tablet', engine: chromium, width: 640, touch: true, dark: false },
    { name: 'firefox', engine: firefox, width: 1440, touch: false, dark: false },
  ]) {
    revision = 0;
    large = false;
    browser = await spec.engine.launch(
      spec.engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    const context = await browser.newContext({
      viewport: { width: spec.width, height: 1000 },
      hasTouch: spec.touch,
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
    page = await context.newPage();
    page.setDefaultTimeout(10000);
    page.on('pageerror', (error) => result.errors.push(String(error)));
    page.on('console', (message) => {
      if (message.type() === 'error') result.errors.push(message.text());
    });
    await page.clock.setFixedTime(new Date('2026-10-15T12:00:00Z'));
    await page.goto(origin + '/proxmox/backups/date');
    await dateTable().getByText('1 PBS file', { exact: true }).waitFor();
    const dateGeometry = await inspect(dateTable());
    if (spec.name !== 'firefox') await capture(spec.name + '-date');
    const resize = [];
    if (spec.name === 'desktop') {
      for (const width of [640, 800, 1000, 320, 1440]) {
        await page.setViewportSize({ width, height: 1000 });
        await page.waitForTimeout(100);
        resize.push({ width, geometry: await inspect(dateTable()) });
      }
      const sort = page.getByRole('button', { name: 'Sort by Created', exact: true });
      await activate(sort, false);
      assert(await sort.evaluate((el) => document.activeElement === el));
      await page.keyboard.press('Space');
      assert(await sort.evaluate((el) => document.activeElement === el));
    }
    await page.getByRole('link', { name: 'Coverage', exact: true }).click();
    const toggle = page.getByRole('button', { name: /Expand details for recovery-vm/i });
    await activate(
      spec.touch ? page.locator('[data-proxmox-backup-row="coverage"]') : toggle,
      spec.touch,
    );
    await coverageDetail().getByText('Restore evidence', { exact: true }).waitFor();
    const coverageGeometry = await inspect(coverageDetail());
    const refresh = page.getByRole('button', { name: 'Refresh backups', exact: true });
    revision = 1;
    await activate(refresh, spec.touch);
    await coverageDetail().getByText(fileName(0, revision), { exact: true }).waitFor();
    assert.equal(await coverageDetail().getByText(fileName(0), { exact: true }).count(), 0);
    await inspect(coverageDetail());
    assert.equal(
      await page
        .getByRole('button', { name: /Collapse details for recovery-vm/i })
        .getAttribute('aria-expanded'),
      'true',
    );
    if (!spec.touch) assert(await refresh.evaluate((el) => document.activeElement === el));
    assert(
      (await coverageDetail().locator('[data-backup-artifact-state]').allTextContents()).includes(
        'Unverified',
      ),
    );
    await capture(spec.name + '-coverage');
    await activate(
      page.getByRole('button', { name: /Collapse details for recovery-vm/i }),
      spec.touch,
    );
    assert.equal(await coverageDetail().count(), 0);
    let windowed;
    if (spec.name === 'desktop' || spec.name === 'phone') {
      large = true;
      await page.goto(origin + '/proxmox/backups/date');
      await page.waitForFunction(
        () =>
          document.querySelector('[data-proxmox-backups-table="recoverable"]')?.dataset
            .proxmoxBackupsWindowed === 'true',
      );
      const seen = new Set();
      let scrollSteps = 0;
      for (; scrollSteps < 180 && seen.size < 162; scrollSteps++) {
        for (const text of await dateTable()
          .locator('[data-backup-artifact-identifier]')
          .evaluateAll((nodes) =>
            nodes
              .filter((node) => {
                const b = node.getBoundingClientRect();
                return b.top >= 0 && b.bottom <= innerHeight && b.height > 0;
              })
              .map((node) => node.textContent.trim()),
          ))
          seen.add(text);
        await page.mouse.wheel(0, 380);
        await page.waitForTimeout(50);
      }
      for (let index = 0; index < 162; index++)
        assert(
          seen.has(fileName(index, index === 0 ? revision : 0)),
          `Windowed feed lost archive ${index}`,
        );
      windowed = {
        distinctArchivePathsReached: seen.size,
        scrollSteps,
        mountedRows: await dateTable().locator('[data-proxmox-backup-row="recoverable"]').count(),
      };
      assert(windowed.mountedRows < 164);
      if (spec.name === 'phone') await capture('phone-windowed-tail', false);
    }
    result.cases.push({
      name: spec.name,
      width: spec.width,
      height: 1000,
      touch: spec.touch,
      dark: spec.dark,
      browser: browser.version(),
      dateGeometry,
      coverageGeometry,
      resize,
      windowed,
      checks: [
        'Full distinct archive IDs and complete snapshot context without hover in both views',
        'Column/continuation placement and correct colspans through every responsive layout',
        'Accepted HTTP replacement updates the same open Coverage disclosure without retaining the old ID',
        'Verification/prune state and existing sorting/disclosure controls remain independent',
      ],
    });
    await context.close();
    await browser.close();
    browser = undefined;
  }
  assert.deepEqual(result.errors, []);
  assert.deepEqual(result.unexpected, []);
  assert(result.requests.every((request) => request.method === 'GET'));
  result.passed = true;
})()
  .catch(async (error) => {
    result.failure = error.stack;
    process.exitCode = 1;
    if (page && !page.isClosed()) await capture('failure').catch(() => {});
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
        cases: result.cases.length,
        captures: result.captures.length,
        failure: result.failure,
        cleanup: result.cleanup,
      }),
    );
  });
