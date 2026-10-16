// Synthetic HTTP -> production inventory reader/model -> existing Coverage.
// Exercise only read-only refresh; no provider, guest or backup operation.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const compiled = path.join(root, 'tmp/backup-point-state/static');
const output = path.join(root, 'tmp/backup-job-outcomes');
const origin = 'http://127.0.0.1:5301';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const result = {
  binding,
  protocol_sha256: hash(fs.readFileSync(__filename)),
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
const pointTime = '2026-10-16T16:00:00Z';
const job = (id, status, startTime = '2026-10-16T17:00:00Z') => ({
  id,
  status,
  startTime,
  vmid: 101,
  type: 'vzdump',
  node: 'pve-a',
  instance: 'fixture-pve',
});
const jobs = () => {
  const ok = job('job-ok', 'OK');
  const failed = job('job-failed', 'failed');
  if (revision === 0) return [ok, failed];
  if (revision === 1) return [failed, ok];
  if (revision === 2) return [failed, job('new-failed', 'failed', '2026-10-16T17:30:00Z'), ok];
  if (revision === 3) return [ok, job('alias', 'completed')];
  if (revision === 4) return [ok, job('active', 'running')];
  if (revision === 5) return [ok, job('undated', 'failed', '')];
  if (revision === 6) return [];
  throw new Error('Unexpected fixture revision');
};
const posture = {
  data: [
    {
      subjectResourceId: 'fixture-vm',
      state: 'protected',
      freshness: 'current',
      verification: 'verified',
      coverage: 'complete',
      evaluatedAt: '2026-10-16T18:00:00Z',
      explanation: 'Synthetic independent provider protection; a job conflict does not change it.',
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
const pbs = {
  data: {
    backups: [
      {
        id: 'completed-pbs',
        instance: 'fixture-pbs',
        datastore: 'main',
        namespace: '',
        backupType: 'vm',
        vmid: '101',
        backupTime: pointTime,
        files: ['index.json.blob'],
        verified: true,
        protected: true,
      },
    ],
  },
};
const rowSelector = '[data-proxmox-backup-row="coverage"]';
async function inspect(page, spec, state) {
  const facts = await page.evaluate((selector) => {
    const row = document.querySelector(selector);
    const explanation = document.querySelector('[data-backup-task-chronology]');
    const unknown = row.querySelector('[data-backup-task-unknown]');
    const geometry = [unknown, explanation].filter(Boolean).map((node) => {
      const box = node.getBoundingClientRect();
      const range = document.createRange();
      range.selectNodeContents(node);
      return {
        text: node.textContent.trim(),
        box: { left: box.left, right: box.right, top: box.top, bottom: box.bottom },
        glyphs: [...range.getClientRects()].map((rect) => ({ left: rect.left, right: rect.right })),
        scroll: node.scrollWidth,
        width: node.clientWidth,
      };
    });
    return {
      text: row.textContent,
      unknown: unknown?.textContent.trim(),
      title: unknown?.title,
      explanation: explanation?.textContent.trim(),
      stable: row === window.originalCoverageRow,
      detailOpen: !!document.querySelector('[data-inline-detail-for]'),
      restoreEvidence: [...document.querySelectorAll('[data-backup-artifact-state]')].map((node) =>
        node.textContent.trim(),
      ),
      location: [...document.querySelectorAll('[data-backup-location]')].map(
        (node) => node.textContent,
      ),
      geometry,
      pageWidth: innerWidth,
      scrollWidth: document.documentElement.scrollWidth,
    };
  }, rowSelector);
  assert(facts.stable, 'Refresh replaced the workload row');
  assert(facts.detailOpen, 'Refresh closed the detail');
  assert(facts.restoreEvidence.includes('Verified'), JSON.stringify(facts));
  assert(
    facts.location.some((text) => text.includes('fixture-pbs') && text.includes('main')),
    JSON.stringify(facts),
  );
  assert.match(facts.text, spec.width === 320 ? /Prot\./ : /Protected/);
  assert(facts.scrollWidth <= facts.pageWidth, JSON.stringify(facts));
  for (const item of facts.geometry) {
    assert(item.scroll <= item.width + 1, JSON.stringify(item));
    for (const rect of item.glyphs)
      assert(
        rect.left >= item.box.left - 1 && rect.right <= item.box.right + 1,
        JSON.stringify(item),
      );
  }
  if (state === 'conflict') {
    assert.equal(facts.unknown, spec.width === 320 ? '?' : 'Unknown');
    assert.match(facts.explanation, /same newest start time report different outcomes/);
    assert(!facts.explanation.includes('unavailable or in the future'));
    assert(!facts.text.includes('Latest task: OK'));
  } else if (state === 'undated') {
    assert.match(facts.explanation, /unavailable or in the future/);
    assert(!facts.explanation.includes('same newest start time'));
  } else {
    assert.equal(facts.unknown, undefined);
    assert.equal(facts.explanation, undefined);
    if (state !== 'empty') assert.match(facts.text, state === 'failed' ? /Failed/ : /OK/);
  }
  return facts;
}
async function capture(page, name) {
  const file = path.join(output, name + '.png');
  await page.screenshot({ path: file, fullPage: true });
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
    result.requests.push({ method: req.method, path: url.pathname, revision });
    const json = (body) => {
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify(body));
    };
    if (req.method !== 'GET') {
      result.unexpected.push({ method: req.method, path: url.pathname });
      res.statusCode = 405;
      return res.end();
    }
    if (url.pathname === '/api/backups/pve')
      return json({ data: { backupTasks: jobs(), storageBackups: [], guestSnapshots: [] } });
    if (url.pathname === '/api/backups/pbs') return json(pbs);
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
  await new Promise((resolve) => server.listen(5301, '127.0.0.1', resolve));
  for (const spec of [
    { name: 'desktop', engine: chromium, width: 1440, height: 1000, dark: false },
    { name: 'phone', engine: webkit, width: 320, height: 1000, dark: true },
  ]) {
    revision = 0;
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
    const toggle = page.getByRole('button', { name: /Expand details for recovery-vm/ });
    await toggle.waitFor();
    await page.locator('[data-backup-task-unknown]').waitFor();
    if (spec.width === 320) await page.locator(rowSelector).tap();
    else {
      await toggle.focus();
      await page.keyboard.press('Enter');
    }
    await page.locator('[data-backup-task-chronology]').waitFor();
    await page.evaluate((selector) => {
      window.originalCoverageRow = document.querySelector(selector);
    }, rowSelector);
    const states = [{ revision, state: 'conflict', facts: await inspect(page, spec, 'conflict') }];
    await capture(page, spec.name + '-conflict');
    for (const state of ['conflict', 'failed', 'ok', 'conflict', 'undated', 'empty']) {
      revision += 1;
      const before = result.requests.length;
      const refresh = page.getByRole('button', { name: 'Refresh backups', exact: true });
      await refresh.waitFor();
      await refresh.focus();
      const response = page.waitForResponse(
        (response) => new URL(response.url()).pathname === '/api/backups/pve',
      );
      await page.keyboard.press('Enter');
      await response;
      await page.waitForFunction(
        () =>
          document.querySelector('button[title^="Read Pulse"]')?.getAttribute('aria-disabled') ===
          'false',
      );
      const reads = result.requests.slice(before).filter((item) => item.path.startsWith('/api/'));
      assert.deepEqual(reads.map((item) => item.path).sort(), [
        '/api/backups/pbs',
        '/api/backups/pve',
        '/api/recovery/postures',
      ]);
      assert.equal(await refresh.evaluate((element) => document.activeElement === element), true);
      states.push({ revision, state, reads, facts: await inspect(page, spec, state) });
      if (state === 'failed') await capture(page, spec.name + '-newer-failure');
    }
    result.cases.push({
      name: spec.name,
      browser: browser.version(),
      viewport: { width: spec.width, height: spec.height },
      states,
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
