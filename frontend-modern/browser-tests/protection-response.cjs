// Synthetic HTTP -> production posture reader -> existing Coverage/table/detail.
// No backup, guest, provider or off-origin operations.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const compiled = path.join(root, 'tmp/protection-response/static');
const output = path.join(root, 'tmp/protection-response/browser');
const origin = 'http://127.0.0.1:5297';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const result = {
  binding,
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  errors: [],
  unexpected: [],
  requests: [],
  cleanup: {},
};
let browser, server, mode, releasePosture;
const response = (state = 'protected') => ({
  data: [
    {
      subjectResourceId: 'fixture-vm',
      state,
      freshness: 'current',
      verification: 'verified',
      coverage: 'complete',
      providerStates: [],
      evidenceIds: ['fixture-point'],
      repositoryResourceIds: [],
      evaluatedAt: '2026-10-15T12:00:00Z',
      explanation:
        state === 'protected'
          ? 'A current verified backup is available.'
          : 'The latest provider job needs attention.',
    },
  ],
  policy: {
    freshnessWindowSeconds: 604800,
    verificationWindowSeconds: 604800,
    requireVerification: true,
  },
});
const pve = { data: { backupTasks: [], storageBackups: [], guestSnapshots: [] } };
const pbs = {
  data: {
    backups: [
      {
        id: 'fixture-point',
        instance: 'fixture-pbs',
        datastore: 'main',
        backupType: 'vm',
        vmid: '101',
        backupTime: '2026-10-15T07:00:00Z',
        files: ['index.json.blob'],
        verified: true,
      },
    ],
  },
};
async function capture(page, name) {
  const file = path.join(output, `${name}.png`);
  await page.screenshot({ path: file, fullPage: true });
  result.captures.push({ path: path.relative(root, file), sha256: hash(fs.readFileSync(file)) });
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
    result.requests.push({ method: req.method, path: url.pathname, mode });
    const json = (value) => {
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify(value));
    };
    if (req.method !== 'GET') {
      result.unexpected.push({ method: req.method, path: url.pathname });
      res.statusCode = 405;
      return res.end();
    }
    if (url.pathname === '/api/backups/pve') return json(pve);
    if (url.pathname === '/api/backups/pbs') return json(pbs);
    if (url.pathname === '/api/recovery/postures') {
      const value = response();
      if (mode === 'duplicates') value.data.unshift(response('attention').data[0]);
      if (mode === 'bad-rule') value.policy.requireVerification = 'false';
      if (mode === 'bad-provider') value.data[0].providerStates = [{ scope: 'PRIVATE_PAYLOAD' }];
      if (mode === 'pending') {
        releasePosture = () => {
          json(response('attention'));
          releasePosture = undefined;
        };
        return;
      }
      return json(value);
    }
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
  for (const [name, engine, width, dark] of [
    ['desktop', chromium, 1440, false],
    ['phone', webkit, 320, true],
  ]) {
    if (binding.browser_selection !== 'all' && binding.browser_selection !== name) continue;
    mode = 'valid';
    browser = await engine.launch(
      engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    const context = await browser.newContext({
      viewport: { width, height: 1000 },
      hasTouch: width < 768,
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
      dark,
    );
    const page = await context.newPage();
    page.setDefaultTimeout(12000);
    page.on('pageerror', (error) => result.errors.push(String(error)));
    await page.clock.install({ time: new Date('2026-10-15T12:00:00Z') });
    await page.goto(origin + '/proxmox/backups/coverage');
    const row = page.locator('[data-proxmox-backup-row="coverage"]');
    await row.getByText(width < 768 ? 'Prot.' : 'Protected', { exact: true }).waitFor();
    if (width < 768) await row.tap();
    else {
      await page.getByRole('button', { name: /Expand details for recovery-vm/i }).focus();
      await page.keyboard.press('Enter');
    }
    const detail = page.locator('[data-inline-detail-for="resource:fixture-vm"]');
    await detail.getByText('Restore evidence', { exact: true }).waitFor();
    const evidence = await detail.locator('table').innerText();
    mode = 'duplicates';
    await page.clock.runFor(30_000);
    await page.getByText(/Protection posture is unavailable/).waitFor();
    await row.getByText('Unknown', { exact: true }).waitFor();
    assert.equal(await detail.locator('table').innerText(), evidence);
    assert.equal(await page.locator('[data-coverage-strip-footnote]').count(), 0);
    assert.equal(await page.getByRole('button', { name: '1 unknown', exact: true }).count(), 1);
    assert.equal(await page.getByRole('button', { name: '1 protected', exact: true }).count(), 0);
    await capture(page, `${name}-unreadable`);
    const inventoryReads = () =>
      result.requests.filter((request) =>
        ['/api/backups/pve', '/api/backups/pbs'].includes(request.path),
      ).length;
    const readCount = inventoryReads();
    mode = 'pending';
    const retry = page.getByRole('button', { name: 'Retry protection posture', exact: true });
    if (width < 768) await retry.tap();
    else {
      await retry.focus();
      await page.keyboard.press('Enter');
    }
    const pending = page.getByRole('button', { name: 'Retrying protection posture…', exact: true });
    await pending.waitFor();
    assert.equal(await pending.isDisabled(), true);
    await page.waitForTimeout(100);
    assert.equal(typeof releasePosture, 'function');
    const requests = result.requests.length;
    await pending.evaluate((button) => button.click());
    assert.equal(result.requests.length, requests);
    assert.equal(await row.getByText('Unknown', { exact: true }).count(), 1);
    releasePosture();
    await detail.getByText('The latest provider job needs attention.', { exact: false }).waitFor();
    assert.equal(await page.getByText(/Protection posture is unavailable/).count(), 0);
    assert.equal(await detail.locator('table').innerText(), evidence);
    assert.equal(inventoryReads(), readCount, 'Posture retry must not reread backup inventory');
    await capture(page, `${name}-recovered`);
    for (const malformed of ['bad-rule', 'bad-provider']) {
      mode = malformed;
      await page.getByRole('button', { name: 'Refresh backups', exact: true }).click();
      await page.getByText(/Protection posture is unavailable/).waitFor();
      await row.getByText('Unknown', { exact: true }).waitFor();
      assert.equal(await detail.locator('table').innerText(), evidence);
      assert(!(await page.locator('body').innerText()).includes('PRIVATE_PAYLOAD'));
    }
    // Also reject an unreadable initial response, not just a failed poll.
    mode = 'bad-rule';
    await page.reload();
    await page.getByText(/Protection posture is unavailable/).waitFor();
    await row.getByText('Unknown', { exact: true }).waitFor();
    const overflow = await page.evaluate(() => ({
      width: innerWidth,
      scroll: document.documentElement.scrollWidth,
    }));
    assert(overflow.scroll <= overflow.width, JSON.stringify(overflow));
    result.cases.push({
      name,
      browser: browser.version(),
      width,
      dark,
      evidence,
      overflow,
      checks: [
        'valid',
        'conflicting-poll',
        'unknown-through-pending-retry',
        'posture-only-recovery',
        'bad-rule',
        'bad-provider',
        'unreadable-initial',
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
  .catch((error) => {
    result.failure = error.stack;
    process.exitCode = 1;
  })
  .finally(async () => {
    if (releasePosture) releasePosture();
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
