// Synthetic HTTP -> production parser/model -> Coverage/By date -> shipped Docs.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const output = path.join(root, 'tmp/backup-point-state/browser');
const compiled = path.join(root, 'tmp/backup-point-state/static');
const origin = 'http://127.0.0.1:5297';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const result = {
  binding,
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  errors: [],
  unexpectedRequests: [],
  requests: [],
  cleanup: {},
};
const mime = {
  '.html': 'text/html',
  '.js': 'text/javascript',
  '.css': 'text/css',
  '.md': 'text/markdown',
};
let browser, server, revision, releasePBS;
const backups = () =>
  [
    { id: 'verified', verified: true, protected: true },
    {
      id: 'unverified',
      verified: revision === 0 ? false : revision === 1 ? true : undefined,
      protected: revision !== 2,
    },
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
    storageBackups: [
      {
        id: 'pve-file',
        vmid: 101,
        type: 'qemu',
        instance: 'fixture-pve',
        node: 'pve-a',
        time: '2026-10-15T07:00:00Z',
        storage: 'local',
        protected: true,
      },
    ],
    guestSnapshots: [
      {
        id: 'guest-snapshot',
        vmid: 101,
        type: 'qemu',
        instance: 'fixture-pve',
        node: 'pve-a',
        name: 'pre-change',
        time: '2026-10-15T08:00:00Z',
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
async function capture(page, name, fullPage = true) {
  const file = path.join(output, `${name}.png`);
  await page.screenshot({ path: file, fullPage });
  result.captures.push({ path: path.relative(root, file), sha256: hash(fs.readFileSync(file)) });
}
async function checkCells(page, target, expected, pruneCount) {
  assert.deepEqual(
    await target.locator('[data-backup-artifact-state]').allTextContents(),
    expected,
  );
  assert.equal(await target.locator('[data-backup-prune-protected]').count(), pruneCount);
  const geometry = await target
    .locator('[data-backup-artifact-state], [data-backup-prune-protected]')
    .evaluateAll((chips) =>
      chips.map((chip) => {
        const box = chip.getBoundingClientRect();
        const cell = chip.closest('td').getBoundingClientRect();
        return {
          text: chip.textContent.trim(),
          left: box.left,
          right: box.right,
          cellLeft: cell.left,
          cellRight: cell.right,
          width: chip.clientWidth,
          scroll: chip.scrollWidth,
        };
      }),
    );
  for (const box of geometry) {
    assert(box.left >= box.cellLeft - 1 && box.right <= box.cellRight + 1, JSON.stringify(box));
    assert(box.scroll <= box.width + 1, JSON.stringify(box));
  }
  const overflow = await page.evaluate(() => ({
    width: innerWidth,
    scroll: document.documentElement.scrollWidth,
  }));
  assert(overflow.scroll <= overflow.width, JSON.stringify(overflow));
  return { geometry, overflow };
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
        return;
      }
      return json({ data: { backups: backups() } });
    }
    if (url.pathname === '/api/backups/pve') return json(pve);
    if (url.pathname === '/api/recovery/postures') return json(posture);
    const file =
      url.pathname.startsWith('/proxmox/backups/') ||
      (url.pathname.startsWith('/docs/') && !url.pathname.endsWith('.md'))
        ? '/browser-tests/backup-point-state.html'
        : url.pathname;
    const bytes = assets.get(file);
    if (!bytes) {
      result.unexpectedRequests.push({ method: req.method, path: url.pathname });
      res.statusCode = 404;
      return res.end('Not bound');
    }
    res.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
    res.end(bytes);
  });
  await new Promise((resolve) => server.listen(5297, '127.0.0.1', resolve));
  for (const spec of [
    { name: 'desktop', engine: chromium, width: 1440, height: 1000, dark: false },
    { name: 'phone', engine: webkit, width: 390, height: 1000, dark: true },
    { name: 'narrow', engine: webkit, width: 320, height: 1000, dark: false },
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
    const page = await context.newPage();
    page.setDefaultTimeout(12000);
    page.on('pageerror', (error) => result.errors.push(String(error)));
    page.on('console', (message) => {
      if (message.type() === 'error') result.errors.push(message.text());
    });
    await page.clock.setFixedTime(new Date('2026-10-15T12:00:00Z'));
    await page.goto(origin + '/proxmox/backups/date');
    const byDate = page.locator('[data-proxmox-backups-table="recoverable"]');
    await byDate.waitFor();
    await page.waitForFunction(
      () => document.querySelectorAll('[data-backup-artifact-state]').length === 8,
    );
    const expected = [
      'Snapshot',
      'Backup file',
      'Verified',
      'Failed',
      'Running',
      'Unknown verification',
      'Unverified',
      'Verified',
    ];
    const dateCells = await checkCells(page, byDate, expected, 6);
    await capture(page, `${spec.name}-date`);
    // Existing state sort uses verification/completion, never retention alone.
    await page.getByRole('button', { name: 'Sort by State', exact: true }).click();
    const sorted = await byDate.locator('[data-backup-artifact-state]').allTextContents();
    assert.deepEqual(
      sorted,
      [...expected].sort((left, right) => left.localeCompare(right)),
    );
    const coverageLink = page.getByRole('link', { name: 'Coverage', exact: true });
    await coverageLink.click();
    const toggle = page.getByRole('button', { name: /Expand details for recovery-vm/i });
    if (spec.width < 768) await page.locator('[data-proxmox-backup-row="coverage"]').tap();
    else {
      await toggle.focus();
      await page.keyboard.press('Enter');
    }
    const evidence = page.locator('[data-inline-detail-for="resource:fixture-vm"]');
    await evidence.getByText('Restore evidence', { exact: true }).waitFor();
    const coverageCells = await checkCells(page, evidence, expected, 6);
    await capture(page, `${spec.name}-coverage`);
    for (revision = 1; revision <= 2; revision++) {
      const readCount = result.requests.filter((request) =>
        request.path.startsWith('/api/'),
      ).length;
      const refresh = page.getByRole('button', { name: 'Refresh backups', exact: true });
      await refresh.focus();
      await page.keyboard.press('Enter');
      const pending = page.getByRole('button', { name: 'Reading backups…', exact: true });
      await pending.waitFor();
      assert.equal(await pending.getAttribute('aria-disabled'), 'true');
      assert.equal(await pending.evaluate((el) => document.activeElement === el), true);
      // A second keyboard activation while the controlled PBS response is
      // pending must not enqueue another inventory or posture read.
      await page.keyboard.press('Enter');
      assert.equal(typeof releasePBS, 'function');
      releasePBS();
      await page.waitForFunction(
        (count) => document.querySelectorAll('[data-backup-prune-protected]').length === count,
        revision === 1 ? 6 : 5,
      );
      await page.waitForFunction(
        (verified) =>
          [...document.querySelectorAll('[data-backup-artifact-state]')].filter(
            (el) => el.textContent === 'Verified',
          ).length === verified,
        revision === 1 ? 3 : 2,
      );
      assert.equal(await evidence.isVisible(), true, 'Refresh must keep the evidence open');
      await refresh.waitFor();
      assert(page.url().endsWith('/proxmox/backups/coverage'));
      assert.equal(await refresh.evaluate((el) => el === document.activeElement), true);
      assert.equal(await refresh.getAttribute('aria-disabled'), 'false');
      assert.equal(
        result.requests.filter((request) => request.path.startsWith('/api/')).length,
        readCount + 3,
      );
      assert.match(
        await page.locator('[data-proxmox-backup-row="coverage"]').innerText(),
        /Attn\.|Attention/,
      );
    }
    await capture(page, `${spec.name}-refreshed`);
    await page.getByRole('link', { name: 'PBS protection help', exact: true }).click();
    const heading = page.getByRole('heading', {
      name: 'Backups are visible but Coverage says Unprotected',
      exact: true,
    });
    await heading.waitFor();
    await heading.scrollIntoViewIfNeeded();
    const article = page.locator('article');
    assert.match(
      await article.innerText(),
      /Prune protected does not change the verification result alongside it/,
    );
    assert.match(await article.innerText(), /Unknown on a PBS point means no verification/);
    await capture(page, `${spec.name}-docs`, false);
    result.cases.push({
      name: spec.name,
      browser: browser.version(),
      width: spec.width,
      height: spec.height,
      dateCells,
      coverageCells,
      sorted,
      refreshRevisions: 2,
    });
    await context.close();
    await browser.close();
    browser = undefined;
  }
  assert.deepEqual(result.errors, []);
  assert.deepEqual(result.unexpectedRequests, []);
  assert(result.requests.every((request) => request.method === 'GET'));
  result.passed = true;
})()
  .catch((error) => {
    result.failure = error.stack;
    process.exitCode = 1;
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
        cases: result.cases.length,
        captures: result.captures.length,
        failure: result.failure,
        cleanup: result.cleanup,
      }),
    );
  });
