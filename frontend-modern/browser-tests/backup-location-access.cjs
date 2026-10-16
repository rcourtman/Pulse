// Existing production API readers, recovery mapper and both backup views.
// All payloads are synthetic loopback observations, not guest/backup actions.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, webkit } = require('playwright');

const root = '/workspace';
const compiled = path.join(root, 'tmp/backup-point-state/static');
const output = path.join(root, 'tmp/backup-location-access');
const origin = 'http://127.0.0.1:5297';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const namespace = 'pve-a/department-with-a-long-name/production';
const result = {
  binding,
  runner_sha256: hash(fs.readFileSync(__filename)),
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
  page,
  revision = 0,
  large = false;
const pbs = () => ({
  data: {
    backups: [
      { id: 'east-nested', instance: 'pbs-east', namespace },
      {
        id: 'west-nested',
        instance: 'pbs-west',
        namespace: namespace + (revision ? '/after-refresh' : ''),
      },
      { id: 'east-root', instance: 'pbs-east', namespace: '' },
      ...(large
        ? Array.from({ length: 160 }, (_, index) => ({
            id: `many-${index}`,
            instance: 'pbs-many',
            namespace: `${namespace}/point-${String(index).padStart(3, '0')}`,
          }))
        : []),
    ].map((point) => ({
      datastore: 'main',
      backupType: 'vm',
      vmid: '101',
      backupTime: '2026-10-15T10:00:00Z',
      files: ['index.json.blob'],
      protected: true,
      verified: false,
      ...point,
    })),
  },
});
const pve = {
  data: {
    backupTasks: [],
    storageBackups: ['archive-node-a', 'archive-node-b'].map((storageNode) => ({
      id: storageNode,
      vmid: 101,
      type: 'qemu',
      instance: 'fixture-pve',
      node: 'pve-a',
      storageNode,
      time: '2026-10-15T10:00:00Z',
      storage: 'local',
      protected: false,
      volid: 'local:backup/vzdump-qemu-101.vma.zst',
    })),
    guestSnapshots: [
      {
        id: 'guest-snapshot',
        vmid: 101,
        type: 'qemu',
        instance: 'fixture-pve',
        node: 'pve-a',
        name: 'before-upgrade',
        time: '2026-10-15T10:00:00Z',
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
const expected = () => [
  `pbs-east / main / ${namespace}`,
  `pbs-west / main / ${namespace}${revision ? '/after-refresh' : ''}`,
  'pbs-east / main / (root)',
  'fixture-pve / local / archive-node-a',
  'fixture-pve / local / archive-node-b',
  'fixture-pve / pve-a',
];
const activate = async (control, touch) => {
  if (touch) await control.tap();
  else {
    await control.focus();
    await page.keyboard.press('Enter');
  }
};
const dateRows = () => page.locator('[data-proxmox-backup-row="recoverable"]');
async function waitDateRows(count) {
  await page.waitForFunction(
    (n) => document.querySelectorAll('[data-proxmox-backup-row="recoverable"]').length === n,
    count,
  );
}
async function inspectLocations(scope, values, compact, colSpan) {
  const labels = scope.locator('[data-backup-location]');
  assert.deepEqual((await labels.allTextContents()).sort(), [...values].sort());
  const geometry = await labels.evaluateAll((nodes) =>
    nodes.map((node) => {
      const cell = node.closest('td');
      const box = node.getBoundingClientRect();
      const cellBox = cell.getBoundingClientRect();
      const range = document.createRange();
      range.selectNodeContents(node);
      const lines = [...range.getClientRects()].filter((rect) => rect.width && rect.height);
      const style = getComputedStyle(node);
      return {
        text: node.textContent,
        width: box.width,
        height: box.height,
        colspan: cell.colSpan,
        client: node.clientWidth,
        scroll: node.scrollWidth,
        whiteSpace: style.whiteSpace,
        overflowWrap: style.overflowWrap,
        ellipsis: style.textOverflow,
        contained: lines.every(
          (rect) =>
            rect.left >= cellBox.left - 1 &&
            rect.right <= cellBox.right + 1 &&
            rect.top >= cellBox.top - 1 &&
            rect.bottom <= cellBox.bottom + 1,
        ),
      };
    }),
  );
  for (const item of geometry) {
    assert(item.contained && item.width > 0 && item.height > 0, JSON.stringify(item));
    assert(item.scroll <= item.client + 1, JSON.stringify(item));
    assert.equal(item.whiteSpace, 'normal');
    assert.equal(item.overflowWrap, 'anywhere');
    assert.notEqual(item.ellipsis, 'ellipsis');
    if (compact) assert.equal(item.colspan, colSpan);
  }
  assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
  return geometry;
}
async function capture(name, fullPage = true) {
  const file = path.join(output, name + '.png');
  await page.screenshot({ path: file, fullPage, animations: 'disabled' });
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
  const assets = new Map();
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, file);
    assets.set('/' + file, bytes);
  }
  server = http.createServer((req, res) => {
    const url = new URL(req.url, origin);
    result.requests.push({ method: req.method, path: url.pathname, query: url.search, revision });
    const json = (data) => {
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify(data));
    };
    if (req.method !== 'GET') {
      result.unexpected.push({ method: req.method, path: url.pathname });
      res.statusCode = 405;
      return res.end();
    }
    if (url.pathname === '/api/backups/pbs') return json(pbs());
    if (url.pathname === '/api/backups/pve') return json(pve);
    if (url.pathname === '/api/recovery/postures') return json(posture);
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
    { name: 'desktop-light', engine: chromium, width: 1365, dark: false, touch: false },
    { name: 'phone-dark', engine: chromium, width: 320, dark: true, touch: true },
    { name: 'phone-webkit-light', engine: webkit, width: 320, dark: false, touch: true },
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
    await waitDateRows(6);
    const date = page.locator('[data-proxmox-backups-table="recoverable"]');
    const dateGeometry = await inspectLocations(date, expected(), spec.width === 320, 4);
    await capture(spec.name + '-date');
    const resize = [];
    if (!spec.touch) {
      for (const width of [640, 320, 1365]) {
        await page.setViewportSize({ width, height: 1000 });
        await page.waitForFunction(
          (compact) =>
            document.querySelector('[data-proxmox-backups-table="recoverable"]').dataset
              .proxmoxBackupsLayout === (compact ? 'compact' : 'full') ||
            (!compact && innerWidth === 640),
          width === 320,
        );
        resize.push({
          width,
          geometry: await inspectLocations(date, expected(), width === 320, 4),
        });
      }
      await activate(page.getByRole('button', { name: 'Sort by Location', exact: true }), false);
      assert.deepEqual(
        await date.locator('[data-backup-location]').allTextContents(),
        expected().sort(),
      );
    }
    if (spec.touch) await page.getByRole('button', { name: /^Filters/ }).tap();
    const combo = page.getByRole('combobox', { name: 'Filter', exact: true });
    const west = await combo
      .locator('option')
      .evaluateAll(
        (nodes) =>
          nodes.find((node) => node.textContent === 'Backup location: pbs-west / main')?.value,
      );
    assert(west);
    await combo.selectOption(west);
    await waitDateRows(1);
    assert.equal(new URL(page.url()).searchParams.get('location'), 'pbs:pbs-west:main');
    await inspectLocations(date, [expected()[1]], spec.width === 320, 4);
    await page.getByRole('link', { name: 'Coverage', exact: true }).click();
    const toggle = page.getByRole('button', { name: /Expand details for recovery-vm/i });
    await activate(
      spec.touch ? page.locator('[data-proxmox-backup-row="coverage"]') : toggle,
      spec.touch,
    );
    const coverage = page.locator('[data-inline-detail-for="resource:fixture-vm"]');
    await coverage.getByText('Restore evidence', { exact: true }).waitFor();
    await inspectLocations(coverage, [expected()[1]], spec.width === 320, 3);
    revision = 1;
    const refresh = page.getByRole('button', { name: 'Refresh backups', exact: true });
    await refresh.focus();
    await activate(refresh, spec.touch);
    await coverage.getByText(expected()[1], { exact: true }).waitFor();
    assert.equal(new URL(page.url()).searchParams.get('location'), 'pbs:pbs-west:main');
    assert(await refresh.evaluate((el) => document.activeElement === el));
    await activate(
      page.getByRole('button', { name: 'Remove Backup location filter', exact: true }),
      spec.touch,
    );
    await page.waitForFunction(
      () =>
        document.querySelectorAll('[data-inline-detail-for] [data-backup-location]').length === 6,
    );
    const coverageGeometry = await inspectLocations(coverage, expected(), spec.width === 320, 3);
    assert(
      (await coverage.locator('[data-backup-artifact-state]').allTextContents()).includes(
        'Unverified',
      ),
    );
    assert.equal(await coverage.locator('[data-backup-prune-protected]').count(), 3);
    await capture(spec.name + '-coverage-refresh');
    // Wrapping and compact provenance lines change row height. Check that
    // the existing windowed feed still lets the operator reach every point.
    large = true;
    await page.goto(origin + '/proxmox/backups/date');
    await page.waitForFunction(
      () =>
        document.querySelector('[data-proxmox-backups-table="recoverable"]')?.dataset
          .proxmoxBackupsWindowed === 'true',
    );
    const seen = new Set();
    let scrollSteps = 0;
    for (; scrollSteps < 150 && seen.size < 166; scrollSteps += 1) {
      for (const text of await date.locator('[data-backup-location]').evaluateAll((nodes) =>
        nodes
          .filter((node) => {
            const rect = node.getBoundingClientRect();
            return rect.top >= 0 && rect.bottom <= innerHeight && rect.height > 0;
          })
          .map((node) => node.textContent),
      ))
        seen.add(text);
      await page.mouse.wheel(0, 400);
      await page.waitForTimeout(80);
      if (scrollSteps === 4) await capture(spec.name + '-windowed-middle', false);
    }
    for (const text of expected()) assert(seen.has(text), `Windowed feed lost ${text}`);
    for (let index = 0; index < 160; index += 1)
      assert(
        seen.has(`pbs-many / main / ${namespace}/point-${String(index).padStart(3, '0')}`),
        `Windowed feed lost point ${index}`,
      );
    const windowed = {
      total: 166,
      distinctLocationsReached: seen.size,
      scrollSteps,
      renderedRows: await dateRows().count(),
    };
    await capture(spec.name + '-windowed-tail', false);
    result.cases.push({
      name: spec.name,
      width: spec.width,
      height: 1000,
      dark: spec.dark,
      touch: spec.touch,
      browser: browser.version(),
      dateGeometry,
      coverageGeometry,
      resize,
      windowed,
      checks: [
        'All six full source locations readable without hover in both existing views',
        'Same-name PBS connections, nested/root namespaces and copied local storage nodes remain distinct',
        'Saved connection facet survives view switch and refresh; changed namespace arrives through production HTTP reader and mapper',
        'Removing scope restores all points while details remain open; verification and prune protection remain independent',
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
    if (page && !page.isClosed()) {
      result.failureText = await page
        .locator('body')
        .innerText()
        .catch(() => 'unavailable');
      await capture('failure').catch(() => {});
    }
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
        passed: !!result.passed,
        cases: result.cases.length,
        captures: result.captures.length,
        failure: result.failure,
        cleanup: result.cleanup,
      }),
    );
  });
