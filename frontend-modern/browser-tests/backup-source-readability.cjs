// Production HTTP readers and both backup tables; synthetic read-only inventory.
// Compile with build-backup-point-state.mjs before running this protocol.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const compiled = path.join(root, 'tmp/backup-point-state/static');
const output = path.join(
  root,
  'tmp/source-readability',
  process.argv.includes('--baseline') ? 'baseline' : 'candidate',
);
const origin = 'http://127.0.0.1:5299';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const result = {
  binding,
  protocol_sha256: hash(fs.readFileSync(__filename)),
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  errors: [],
  unexpected: [],
  requests: [],
  clipped: [],
  cleanup: {},
};
const sourceTitles = [
  'Direct inventory from Proxmox Backup Server',
  'PVE backup files reported by Proxmox VE storage',
  'Proxmox VE guest snapshot',
];
const sourceSelector = sourceTitles.map((title) => `[title="${title}"]`).join(',');
let server,
  browser,
  revision = 0,
  large = false;
const time = (offset) => new Date(Date.now() - 3_600_000 - offset * 60_000).toISOString();
// Stable times keep ordinary refresh from changing the inventory's chronology.
const times = Array.from({ length: 163 }, (_, index) => time(index));
const pbs = () => ({
  data: {
    backups: [
      {
        id: 'pbs',
        instance: 'fixture-pbs',
        datastore: 'main',
        namespace: '',
        backupType: 'vm',
        vmid: '101',
        backupTime: times[0],
        files: ['index.json.blob'],
        verified: true,
        protected: true,
      },
    ],
  },
});
const pve = () => ({
  data: {
    backupTasks: [],
    storageBackups: [
      {
        id: 'archive',
        vmid: 101,
        type: 'qemu',
        instance: 'fixture-pve',
        node: 'pve-a',
        time: times[1],
        storage: 'local',
        protected: false,
      },
    ],
    guestSnapshots: Array.from({ length: large ? 161 : 1 }, (_, index) => ({
      id: `snapshot-${index}`,
      vmid: 101,
      type: 'qemu',
      instance: 'fixture-pve',
      node: 'pve-a',
      name: `before-change-${index}`,
      time: times[index + 2],
    })),
  },
});
const posture = {
  data: [
    {
      subjectResourceId: 'fixture-vm',
      state: 'unknown',
      freshness: 'unknown',
      verification: 'unknown',
      coverage: 'unknown',
      evaluatedAt: times[0],
      explanation: 'Synthetic unknown posture; a source label does not establish protection.',
      providerStates: [],
      repositoryResourceIds: [],
      evidenceIds: [],
    },
  ],
  policy: {
    freshnessWindowSeconds: 604800,
    verificationWindowSeconds: 604800,
    requireVerification: true,
  },
};

async function inspect(scope, name) {
  const geometry = await scope.locator(sourceSelector).evaluateAll((nodes) =>
    nodes.map((node) => {
      const cell = node.closest('td');
      const box = node.getBoundingClientRect(),
        boundary = cell.getBoundingClientRect();
      // Measure actual glyphs as well as the badge box: fitting its parent box
      // does not prove that a nowrap text run survived an overflow-hidden cell.
      const walker = document.createTreeWalker(node, NodeFilter.SHOW_TEXT);
      const glyphs = [];
      while (walker.nextNode()) {
        const text = walker.currentNode;
        for (let index = 0; index < text.length; index += 1) {
          const range = document.createRange();
          range.setStart(text, index);
          range.setEnd(text, index + 1);
          glyphs.push(
            ...[...range.getClientRects()].map((rect) => ({
              left: rect.left,
              right: rect.right,
              top: rect.top,
              bottom: rect.bottom,
            })),
          );
        }
      }
      return {
        text: node.textContent,
        title: node.title,
        box: { left: box.left, right: box.right, top: box.top, bottom: box.bottom },
        cell: {
          left: boundary.left,
          right: boundary.right,
          top: boundary.top,
          bottom: boundary.bottom,
        },
        glyphs,
        scroll: node.scrollWidth,
        client: node.clientWidth,
      };
    }),
  );
  assert(geometry.length > 0, `${name}: source badges missing`);
  for (const badge of geometry) {
    const inside =
      badge.box.left >= badge.cell.left - 1 &&
      badge.box.right <= badge.cell.right + 1 &&
      badge.scroll <= badge.client + 1 &&
      badge.glyphs.every(
        (glyph) =>
          glyph.left >= badge.cell.left - 1 &&
          glyph.right <= badge.cell.right + 1 &&
          glyph.top >= badge.cell.top - 1 &&
          glyph.bottom <= badge.cell.bottom + 1 &&
          glyph.left >= badge.box.left - 1 &&
          glyph.right <= badge.box.right + 1,
      );
    if (!inside) result.clipped.push({ name, ...badge });
  }
  assert(await scope.page().evaluate(() => document.documentElement.scrollWidth <= innerWidth));
  return { name, geometry };
}
async function capture(page, name) {
  const file = path.join(output, `${name}.png`);
  await page.screenshot({ path: file, fullPage: true });
  result.captures.push({ path: path.relative(root, file), sha256: hash(fs.readFileSync(file)) });
}
async function activate(locator, touch) {
  if (touch) await locator.tap();
  else {
    await locator.focus();
    await locator.press('Enter');
  }
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
    const json = (value) => {
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify(value));
    };
    if (req.method !== 'GET') {
      result.unexpected.push(req.method);
      res.statusCode = 405;
      return res.end();
    }
    if (url.pathname === '/api/backups/pbs') return json(pbs());
    if (url.pathname === '/api/backups/pve') return json(pve());
    if (url.pathname === '/api/recovery/postures') return json(posture);
    const file = url.pathname.startsWith('/proxmox/backups/')
      ? '/browser-tests/backup-point-state.html'
      : url.pathname;
    const bytes = assets.get(file);
    if (!bytes) {
      result.unexpected.push(url.pathname);
      res.statusCode = 404;
      return res.end();
    }
    res.setHeader(
      'Content-Type',
      { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css' }[path.extname(file)] ||
        'application/octet-stream',
    );
    res.end(bytes);
  });
  await new Promise((resolve) => server.listen(5299, '127.0.0.1', resolve));
  for (const spec of [
    { name: 'desktop', engine: chromium, width: 1440, touch: false, dark: false },
    { name: 'phone', engine: webkit, width: 320, touch: true, dark: true },
  ]) {
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
      result.unexpected.push('off-origin');
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
    await page.goto(origin + '/proxmox/backups/date');
    const date = page.locator('[data-proxmox-backups-table="recoverable"]');
    await date.locator(sourceSelector).last().waitFor();
    const checks = [await inspect(date, `${spec.name}-date`)];
    assert.deepEqual(await date.locator(sourceSelector).allTextContents(), [
      'PBS',
      'PVE file',
      'Snapshot',
    ]);
    await capture(page, `${spec.name}-date`);
    if (!spec.touch) {
      for (const width of [640, 390, 320, 1440]) {
        await page.setViewportSize({ width, height: 1000 });
        await page.waitForFunction(
          (compact) =>
            document.querySelector('[data-proxmox-backups-table="recoverable"]').dataset
              .proxmoxBackupsLayout ===
            (compact ? 'compact' : innerWidth === 640 ? 'basic' : 'full'),
          width < 480,
        );
        checks.push(await inspect(date, `resize-${width}`));
      }
    }
    await activate(page.getByRole('link', { name: 'Coverage', exact: true }), spec.touch);
    await activate(
      spec.touch
        ? page.locator('[data-proxmox-backup-row="coverage"]')
        : page.getByRole('button', { name: /Expand details for recovery-vm/i }),
      spec.touch,
    );
    const detail = page.locator('[data-inline-detail-for="resource:fixture-vm"]');
    await detail.getByText('Restore evidence', { exact: true }).waitFor();
    checks.push(await inspect(detail, `${spec.name}-coverage`));
    assert.deepEqual(await detail.locator(sourceSelector).allTextContents(), [
      'PBS',
      'PVE file',
      'Snapshot',
    ]);
    revision += 1;
    await activate(page.getByRole('button', { name: 'Refresh backups', exact: true }), spec.touch);
    await page.waitForFunction(
      () => document.querySelector('button[title^="Read Pulse"]').textContent === 'Refresh backups',
    );
    checks.push(await inspect(detail, `${spec.name}-refresh`));
    await capture(page, `${spec.name}-coverage`);
    // Source wrapping changes row height. Check the real windowed date feed,
    // including every old and new point rather than just a tail sentinel.
    large = true;
    await page.goto(origin + '/proxmox/backups/date');
    await page.waitForFunction(
      () =>
        document.querySelector('[data-proxmox-backups-table="recoverable"]')?.dataset
          .proxmoxBackupsWindowed === 'true',
    );
    const seen = new Set();
    let steps = 0;
    for (; steps < 120 && seen.size < 163; steps += 1) {
      for (const title of await date.locator('[title^="Current backup age"]').evaluateAll((nodes) =>
        nodes
          .filter((node) => {
            const r = node.getBoundingClientRect();
            return r.top >= 0 && r.bottom <= innerHeight;
          })
          .map((node) => node.title),
      ))
        seen.add(title);
      checks.push(await inspect(date, `${spec.name}-window-${steps}`));
      await page.mouse.wheel(0, 400);
      await page.waitForTimeout(60);
    }
    assert.equal(seen.size, 163, `Unreachable windowed points: ${spec.name}`);
    assert((await date.locator('[data-proxmox-backup-row="recoverable"]').count()) < 163);
    result.cases.push({
      name: spec.name,
      browser: browser.version(),
      width: spec.width,
      touch: spec.touch,
      dark: spec.dark,
      checks,
      windowed: { reached: seen.size, steps },
    });
    await context.close();
    await browser.close();
    browser = undefined;
  }
  assert.deepEqual(result.errors, []);
  assert.deepEqual(result.unexpected, []);
  assert.deepEqual(result.clipped, [], 'Source badge text must fit its own badge and table cell');
  result.result = 'passed';
})()
  .catch((error) => {
    result.result = 'failed';
    result.failure = String(error.stack || error);
    process.exitCode = 1;
  })
  .finally(async () => {
    if (browser) {
      await browser.close();
      result.cleanup.browser = 'closed-after-failure';
    } else result.cleanup.browser = 'all-closed';
    if (server) {
      server.closeAllConnections?.();
      await new Promise((resolve) => server.close(resolve));
      result.cleanup.server = 'closed';
    }
    fs.mkdirSync(output, { recursive: true });
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        result: result.result,
        failure: result.failure,
        cases: result.cases.length,
        clipped: result.clipped.length,
        output,
        cleanup: result.cleanup,
      }),
    );
  });
