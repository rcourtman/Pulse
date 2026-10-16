// Production backup consumers and CSS, loopback HTTP fixtures only.
// Build first with build-backup-point-state.mjs; no guest/backup action occurs.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const mode = process.argv[2] || 'layout';
assert(['layout', 'composition'].includes(mode));
const output = path.join(root, 'tmp/backup-filter-layout', mode);
const compiled = path.join(root, 'tmp/backup-point-state/static');
const origin = 'http://127.0.0.1:5298';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const result = {
  mode,
  binding,
  scriptSha256: hash(fs.readFileSync(__filename)),
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  requests: [],
  errors: [],
  unexpectedRequests: [],
  cleanup: {},
};
let browser,
  server,
  page,
  revision = 0,
  releasePBS,
  observePBS;
const backups = () =>
  [
    { id: 'verified', verified: true, protected: true },
    { id: 'unverified', verified: revision === 0 ? false : true, protected: true },
    { id: 'unknown', protected: true },
    {
      id: 'running',
      inProgress: true,
      writeActivityObserved: true,
      writeActive: true,
      verified: true,
      protected: true,
    },
    {
      id: 'failed',
      inProgress: true,
      writeActivityObserved: true,
      writeActive: false,
      verified: true,
      protected: true,
    },
    { id: 'unprotected', verified: true, protected: false },
    { id: 'other-connection', instance: 'second-pbs', verified: true, protected: false },
    { id: 'other-datastore', datastore: 'other', verified: false, protected: false },
  ].map((point, index) => ({
    instance: 'fixture-pbs',
    datastore: 'main',
    namespace: '',
    backupType: 'vm',
    vmid: '101',
    backupTime: `2026-10-15T0${index + 1}:00:00Z`,
    files: ['index.json.blob'],
    ...point,
  }));
const pve = {
  data: {
    backupTasks: [],
    storageBackups: ['pve-a', 'pve-b'].map((node) => ({
      id: `pve-file-${node}`,
      vmid: 101,
      type: 'qemu',
      instance: 'fixture-pve',
      node,
      time: '2026-10-15T09:00:00Z',
      storage: 'local',
      protected: true,
      volid: 'local:backup/vzdump-qemu-101-2026_10_15-09_00_00.vma.zst',
    })),
    guestSnapshots: [
      {
        id: 'guest-snapshot',
        vmid: 101,
        type: 'qemu',
        instance: 'fixture-pve',
        node: 'pve-a',
        name: 'pre-change',
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
      explanation: 'Verification evidence needs attention.',
      providerStates: [],
      repositoryResourceIds: [],
      evidenceIds: [],
      evaluatedAt: '2026-10-15T12:00:00Z',
    },
  ],
  policy: {
    freshnessWindowSeconds: 604800,
    verificationWindowSeconds: 604800,
    requireVerification: true,
  },
};
const sourceOptions = [
  ['All', 'all', 11],
  ['PBS snapshots from Proxmox Backup Server', 'pbs', 8],
  ['PVE backup files found on Proxmox VE storage', 'archive', 2],
  ['Guest snapshots from Proxmox VE', 'snapshot', 1],
  ['Verified', 'verified', 3],
  ['Unverified', 'unverified', 2],
];
const rows = () => page.locator('[data-proxmox-backup-row="recoverable"]');
const waitRows = (n) =>
  page.waitForFunction(
    (count) =>
      document.querySelectorAll('[data-proxmox-backup-row="recoverable"]').length === count,
    n,
  );
const activate = async (button, narrow) => {
  if (narrow) await button.tap();
  else {
    await button.focus();
    await page.keyboard.press('Enter');
  }
};
async function capture(name) {
  await page.evaluate(() => {
    for (const animation of document.getAnimations()) {
      try {
        animation.finish();
      } catch {}
    }
  });
  const geometry = await page.evaluate(() => ({
    viewport: innerWidth,
    document: document.documentElement.scrollWidth,
  }));
  assert(geometry.document <= geometry.viewport, JSON.stringify(geometry));
  const file = path.join(output, `${name}.png`);
  await page.screenshot({ path: file, fullPage: true, animations: 'disabled' });
  result.captures.push({
    path: path.relative(root, file),
    sha256: hash(fs.readFileSync(file)),
    geometry,
  });
}
async function checkSourceGeometry(narrow) {
  const geometry = await page
    .getByRole('group', { name: 'Source', exact: true })
    .getByRole('button')
    .evaluateAll((buttons) =>
      buttons.map((button) => {
        const box = button.getBoundingClientRect();
        const group = button.parentElement.getBoundingClientRect();
        const range = document.createRange();
        range.selectNodeContents(button);
        const text = [...range.getClientRects()].filter(
          (rect) => rect.width > 0 && rect.height > 0,
        );
        return {
          name: button.getAttribute('aria-label'),
          x: box.x,
          y: box.y,
          right: box.right,
          bottom: box.bottom,
          width: box.width,
          height: box.height,
          client: button.clientWidth,
          scroll: button.scrollWidth,
          groupLeft: group.left,
          groupRight: group.right,
          textContained: text.every(
            (rect) => rect.left >= box.left - 1 && rect.right <= box.right + 1,
          ),
        };
      }),
    );
  assert.equal(geometry.length, 6);
  for (const box of geometry) {
    assert(box.textContained, JSON.stringify(box));
    assert(box.scroll <= box.client + 1, JSON.stringify(box));
    assert(box.x >= box.groupLeft - 1 && box.right <= box.groupRight + 1, JSON.stringify(box));
    if (narrow) assert(box.width >= 44 && box.height >= 44, JSON.stringify(box));
    for (const other of geometry)
      if (other !== box)
        assert(
          Math.min(box.right, other.right) <= Math.max(box.x, other.x) + 1 ||
            Math.min(box.bottom, other.bottom) <= Math.max(box.y, other.y) + 1,
          'Overlapping filter buttons',
        );
  }
  const lines = new Set(geometry.map((box) => Math.round(box.y))).size;
  assert(narrow ? lines > 1 : lines === 1, `Unexpected filter rows: ${lines}`);
  return geometry;
}
(async () => {
  fs.mkdirSync(output, { recursive: true });
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const assets = new Map();
  for (const [file, digest] of Object.entries(binding.sources))
    assert.equal(
      hash(fs.readFileSync(path.join(root, 'frontend-modern', file))),
      digest,
      `Source changed: ${file}`,
    );
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest);
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
      result.unexpectedRequests.push({ method: req.method, path: url.pathname });
      res.statusCode = 405;
      return res.end();
    }
    if (url.pathname === '/api/backups/pbs') {
      if (revision > 0) {
        releasePBS = () => {
          json({ data: { backups: backups() } });
          releasePBS = undefined;
        };
        observePBS?.();
        observePBS = undefined;
        return;
      }
      return json({ data: { backups: backups() } });
    }
    if (url.pathname === '/api/backups/pve') return json(pve);
    if (url.pathname === '/api/recovery/postures') return json(posture);
    const file = url.pathname.startsWith('/proxmox/backups/')
      ? '/browser-tests/backup-point-state.html'
      : url.pathname;
    const bytes = assets.get(file);
    if (!bytes) {
      result.unexpectedRequests.push({ path: url.pathname });
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
  await new Promise((resolve) => server.listen(5298, '127.0.0.1', resolve));
  for (const spec of [
    { name: 'desktop', engine: chromium, width: 1365, height: 1000, dark: false },
    { name: 'narrow', engine: webkit, width: 320, height: 1000, dark: mode === 'layout' },
  ]) {
    revision = 0;
    browser = await spec.engine.launch(
      spec.engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    const narrow = spec.width < 768;
    const context = await browser.newContext({
      viewport: { width: spec.width, height: spec.height },
      hasTouch: narrow,
      locale: 'en-GB',
    });
    await context.route('**/*', (route) => {
      if (new URL(route.request().url()).origin === origin) return route.continue();
      result.unexpectedRequests.push({ offOrigin: route.request().url() });
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
    const saved = mode === 'composition' ? '?location=archive%3Afixture-pve%3Alocal' : '';
    await page.goto(origin + '/proxmox/backups/date' + saved);
    await waitRows(mode === 'composition' ? 2 : 11);
    if (narrow) await page.getByRole('button', { name: /^Filters/ }).tap();
    const checks = [],
      geometry = await checkSourceGeometry(narrow);
    if (mode === 'layout') {
      await capture(`${spec.name}-sources`);
      for (const [name, value, count] of sourceOptions) {
        const button = page
          .getByRole('group', { name: 'Source', exact: true })
          .getByRole('button', { name, exact: true });
        await activate(button, narrow);
        await waitRows(count);
        assert.equal(await button.getAttribute('aria-pressed'), 'true');
        const urlValue = new URL(page.url()).searchParams.get('source');
        assert.equal(urlValue, value === 'all' ? null : value);
      }
      if (narrow) {
        await page.getByRole('button', { name: /^Filters/ }).tap();
        assert.equal(new URL(page.url()).searchParams.get('source'), 'unverified');
        await page.getByRole('button', { name: /^Filters/ }).tap();
      }
      await activate(page.getByRole('button', { name: 'Clear filters', exact: true }), narrow);
      await waitRows(11);
      assert.equal(new URL(page.url()).search, '');
      checks.push(
        'All six labelled source/verification buttons fit their own touch targets, never overlap, wrap only when needed, and select the existing URL/count states with touch/keyboard. Collapse/reopen and Clear filters preserve/reset state.',
      );
    } else {
      assert.match(
        await rows()
          .innerText()
          .catch(async () => (await rows().allTextContents()).join(' ')),
        /PVE file/,
      );
      await capture(`${spec.name}-saved-scope`);
      await activate(
        page.getByRole('button', { name: 'Remove Backup location filter', exact: true }),
        narrow,
      );
      await waitRows(11);
      const combo = page.getByRole('combobox', { name: 'Filter', exact: true });
      const options = await combo
        .locator('option')
        .evaluateAll((nodes) =>
          nodes.map((node) => ({ value: node.value, label: node.textContent })),
        );
      for (const [label, count] of [
        ['fixture-pbs / main', 6],
        ['second-pbs / main', 1],
        ['fixture-pbs / other', 1],
        ['fixture-pve / local / pve-a', 1],
        ['fixture-pve / local / pve-b', 1],
      ]) {
        const option = options.find((entry) => entry.label === `Backup location: ${label}`);
        assert(option, JSON.stringify(options));
        await combo.selectOption(option.value);
        await waitRows(count);
        const urlKey = new URL(page.url()).searchParams.get('location');
        assert(urlKey);
        if (label.includes('/ pve-'))
          assert.equal(JSON.parse(urlKey)[3], label.endsWith('pve-a') ? 'pve-a' : 'pve-b');
        else assert.equal(urlKey, 'pbs:' + label.split(' / ').join(':'));
        await activate(
          page.getByRole('button', { name: 'Remove Backup location filter', exact: true }),
          narrow,
        );
        await waitRows(11);
        assert.equal(new URL(page.url()).searchParams.has('location'), false);
      }
      await page.getByRole('link', { name: 'Coverage', exact: true }).click();
      const toggle = page.getByRole('button', { name: /Expand details for recovery-vm/i });
      await activate(
        narrow ? page.locator('[data-proxmox-backup-row="coverage"]') : toggle,
        narrow,
      );
      const evidence = page.locator('[data-inline-detail-for="resource:fixture-vm"]');
      await evidence.getByText('Restore evidence', { exact: true }).waitFor();
      for (const state of [
        'Verified',
        'Unverified',
        'Unknown verification',
        'Running',
        'Failed',
        'Backup file',
        'Snapshot',
      ])
        assert(
          (await evidence.locator('[data-backup-artifact-state]').allTextContents()).includes(
            state,
          ),
          state,
        );
      assert((await evidence.locator('[data-backup-prune-protected]').count()) > 0);
      revision = 1;
      const ready = new Promise((resolve) => {
        observePBS = resolve;
      });
      const apiCount = result.requests.filter((request) => request.path.startsWith('/api/')).length;
      const refresh = page.getByRole('button', { name: 'Refresh backups', exact: true });
      await refresh.focus();
      await page.keyboard.press('Enter');
      await ready;
      const pending = page.getByRole('button', { name: 'Reading backups…', exact: true });
      assert.equal(await pending.getAttribute('aria-disabled'), 'true');
      assert(await pending.evaluate((el) => el === document.activeElement));
      await page.keyboard.press('Enter');
      releasePBS();
      await refresh.waitFor();
      await page.waitForFunction(() =>
        document
          .querySelector('[data-inline-detail-for="resource:fixture-vm"]')
          ?.textContent.includes('Restore evidence'),
      );
      assert(await refresh.evaluate((el) => el === document.activeElement));
      assert.equal(
        result.requests.filter((request) => request.path.startsWith('/api/')).length,
        apiCount + 3,
      );
      assert.equal(await evidence.isVisible(), true);
      await capture(`${spec.name}-coverage`);
      checks.push(
        'Saved aggregate location still contains both node-local files; each connection/datastore/node-local facet keeps its exact location key/count and removing its chip restores all rows. Combined Coverage retains separate verification/prune evidence, open details and refresh focus without duplicate reads.',
      );
    }
    result.cases.push({ ...spec, engine: undefined, browser: browser.version(), geometry, checks });
    await context.close();
    await browser.close();
    browser = undefined;
  }
  assert.deepEqual(result.errors, []);
  assert.deepEqual(result.unexpectedRequests, []);
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
    if (releasePBS) releasePBS();
    if (browser) await browser.close();
    result.cleanup.browserClosed = true;
    if (server) await new Promise((resolve) => server.close(resolve));
    result.cleanup.serverClosed = true;
    fs.mkdirSync(output, { recursive: true });
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        passed: !!result.passed,
        mode,
        cases: result.cases.length,
        captures: result.captures.length,
        failure: result.failure,
        cleanup: result.cleanup,
      }),
    );
  });
