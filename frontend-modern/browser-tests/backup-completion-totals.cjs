// Compile with build-backup-point-state.mjs at the committed source first.
// Actual HTTP readers -> production recovery model -> both existing views.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const compiled = path.join(root, 'tmp/backup-point-state/static');
const output = path.join(root, 'tmp/backup-completion-totals/browser');
const origin = 'http://127.0.0.1:5298';
const phoneCompletion = process.argv.includes('--phone-completion-only');
const phoneContinuation = phoneCompletion || process.argv.includes('--phone-coverage-only');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const result = {
  binding,
  protocol_sha256: hash(fs.readFileSync(__filename)),
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  errors: [],
  requests: [],
  unexpectedRequests: [],
  cleanup: {},
};
let server,
  browser,
  revision = 0,
  holdPBS = false,
  pendingPBS,
  pendingPVE;
// Dates are ordinary observations in the chart's current window, not a clock override.
const time = new Date(Date.now() - 3_600_000).toISOString();
const pbs = () => ({
  data: {
    backups: [
      {
        id: 'running-pbs',
        size: 64 * 1024 * 1024,
        inProgress: revision < 2,
        writeActivityObserved: true,
        writeActive: revision < 2,
      },
      {
        id: 'failed-pbs',
        size: 256 * 1024 * 1024,
        inProgress: revision < 2,
        writeActivityObserved: true,
        writeActive: false,
      },
      ...(revision > 0 ? [{ id: 'complete-pbs', size: 2048 }] : []),
    ].map((point) => ({
      instance: 'fixture-pbs',
      datastore: 'main',
      namespace: '',
      backupType: 'vm',
      vmid: '101',
      backupTime: time,
      files: ['index.json.blob'],
      verified: true,
      protected: true,
      ...point,
    })),
  },
});
const pve = () => ({
  data: {
    backupTasks: [],
    storageBackups: [
      { id: 'running-pve', size: 128 * 1024 * 1024, inProgress: revision < 2 },
      ...(revision > 0 ? [{ id: 'complete-pve', size: 1024 }] : []),
    ].map((point) => ({
      vmid: 101,
      type: 'qemu',
      instance: 'fixture-pve',
      node: 'pve-a',
      time,
      storage: 'local',
      protected: true,
      isPBS: false,
      ...point,
    })),
    guestSnapshots:
      revision > 0
        ? [
            {
              id: 'local-snapshot',
              vmid: 101,
              type: 'qemu',
              instance: 'fixture-pve',
              node: 'pve-a',
              name: 'before-change',
              time,
              sizeBytes: 512,
            },
          ]
        : [],
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
      explanation: 'Synthetic unknown posture; completion is not protection.',
      evaluatedAt: time,
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
async function capture(page, name) {
  const file = path.join(output, `${name}.png`);
  await page.screenshot({ path: file, fullPage: true });
  result.captures.push({ path: path.relative(root, file), sha256: hash(fs.readFileSync(file)) });
}
async function activate(locator, spec) {
  if (spec.touch) await locator.tap();
  else {
    await locator.focus();
    await locator.press('Enter');
  }
}
async function readable(page) {
  const geometry = await page.evaluate(() => ({
    width: innerWidth,
    scroll: document.documentElement.scrollWidth,
    notices: [...document.querySelectorAll('p[role="status"]')].map((element) => {
      const box = element.getBoundingClientRect();
      return {
        text: element.innerText,
        width: box.width,
        height: box.height,
        scroll: element.scrollWidth,
        client: element.clientWidth,
      };
    }),
  }));
  assert(geometry.scroll <= geometry.width, JSON.stringify(geometry));
  for (const notice of geometry.notices) {
    assert(
      notice.width > 0 && notice.height > 0 && notice.scroll <= notice.client + 1,
      JSON.stringify(notice),
    );
  }
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
      result.unexpectedRequests.push(req.method);
      res.statusCode = 405;
      return res.end();
    }
    if (url.pathname === '/api/backups/pbs') {
      if (holdPBS) {
        pendingPBS = () => {
          json(pbs());
          pendingPBS = undefined;
        };
        return;
      }
      return json(pbs());
    }
    if (url.pathname === '/api/backups/pve') {
      if (holdPBS) {
        pendingPVE = () => {
          json(pve());
          pendingPVE = undefined;
        };
        return;
      }
      return json(pve());
    }
    if (url.pathname === '/api/recovery/postures') return json(posture);
    const file = url.pathname.startsWith('/proxmox/backups/')
      ? '/browser-tests/backup-point-state.html'
      : url.pathname;
    const bytes = assets.get(file);
    if (!bytes) {
      result.unexpectedRequests.push(url.pathname);
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
    { name: 'desktop', engine: chromium, width: 1440, touch: false, dark: false },
    { name: 'phone', engine: webkit, width: 320, touch: true, dark: true },
  ].filter((spec) => !phoneContinuation || spec.name === 'phone')) {
    revision = phoneContinuation ? 1 : 0;
    holdPBS = false;
    browser = await spec.engine.launch(
      spec.engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    const context = await browser.newContext({
      viewport: { width: spec.width, height: 1000 },
      hasTouch: spec.touch,
      colorScheme: spec.dark ? 'dark' : 'light',
    });
    await context.addInitScript((dark) => {
      document.addEventListener('DOMContentLoaded', () =>
        document.documentElement.classList.toggle('dark', dark),
      );
    }, spec.dark);
    const page = await context.newPage();
    page.on('pageerror', (error) => result.errors.push(error.message));
    page.on('request', (request) => {
      if (!request.url().startsWith(origin) && !request.url().startsWith('data:'))
        result.unexpectedRequests.push(request.url());
    });
    const item = {
      viewport: { width: spec.width, height: 1000 },
      browser: browser.version(),
      name: spec.name,
      states: [],
    };
    await page.goto(
      origin + '/proxmox/backups/' + (phoneContinuation && !phoneCompletion ? 'coverage' : 'date'),
    );
    const summary = page.getByText(/targets · .*restore points/);
    const refresh = page.getByRole('button', { name: 'Refresh backups', exact: true });
    await refresh.waitFor();
    if (!phoneContinuation) {
      assert.match(await summary.innerText(), /0 restore points.*3 incomplete entries/);
      assert.equal(await page.locator('[data-proxmox-backup-row="recoverable"]').count(), 3);
      assert.deepEqual(
        (await page.locator('[data-backup-artifact-state]').allTextContents()).sort(),
        ['Failed', 'Running', 'Running'],
      );
      await page.getByText(/No completed restore-point activity/).waitFor();
      assert.equal(
        await page.getByRole('group', { name: 'Activity metric', exact: true }).count(),
        0,
      );
      item.states.push({
        phase: 'only-incomplete',
        summary: await summary.innerText(),
        geometry: await readable(page),
      });
      await capture(page, spec.name + '-incomplete');

      revision = 1;
      await activate(refresh, spec);
      await page.getByRole('button', { name: /: 3 backups$/, exact: false }).waitFor();
      assert.match(await summary.innerText(), /3 restore points.*3 incomplete entries.*1 with/);
      assert.equal(await page.locator('[data-proxmox-backup-row="recoverable"]').count(), 6);
      assert.match(await page.locator('[data-proxmox-backup-row="day"]').innerText(), /6 entries/);
      await activate(page.getByRole('button', { name: /: 3 backups$/ }), spec);
      assert.match(page.url(), /day=/);
      assert.equal(await page.locator('[data-proxmox-backup-row="recoverable"]').count(), 6);
      await activate(page.getByRole('button', { name: 'Clear date filter', exact: true }), spec);
      await activate(page.getByRole('button', { name: 'Volume', exact: true }), spec);
      await page.getByRole('button', { name: /: 3\.50 KB$/ }).waitFor();
      await activate(page.getByRole('button', { name: 'Count', exact: true }), spec);
      item.states.push({
        phase: 'mixed',
        summary: await summary.innerText(),
        rows: 6,
        chartPoints: 3,
        chartBytes: 3584,
        geometry: await readable(page),
      });
      await page.evaluate(() => window.scrollTo(0, 0));
      await capture(page, spec.name + '-mixed');

      await activate(page.getByRole('link', { name: 'Coverage', exact: true }), spec);
    }
    if (!phoneCompletion) {
      assert.match(await summary.innerText(), /3 restore points.*3 incomplete entries/);
      await page.getByText(/Incomplete backups remain listed/).waitFor();
      const expansion = page.locator('[data-proxmox-backup-row="coverage"] button[aria-expanded]');
      // Mobile hides the keyboard toggle in favour of the tappable workload row.
      if (spec.touch) await page.locator('[data-proxmox-backup-row="coverage"]').tap();
      else await activate(expansion, spec);
      assert.equal(await expansion.getAttribute('aria-expanded'), 'true');
      assert.equal(await page.locator('[data-backup-artifact-state]').count(), 6);
      assert.match(
        await page.locator('[data-proxmox-backup-row="coverage"]').innerText(),
        /Unknown/,
      );
      // The actual workload row remains reachable without changing its provider posture.
      await page.getByRole('link', { name: 'By date', exact: true }).waitFor();
      await activate(page.getByRole('link', { name: 'By date', exact: true }), spec);
      assert.equal(await expansion.count(), 0);
    }

    revision = 2;
    holdPBS = true;
    await activate(refresh, spec);
    await page.waitForFunction(
      () => document.querySelector('button[aria-disabled="true"]') !== null,
    );
    assert.match(await summary.innerText(), /3 restore points.*3 incomplete entries/);
    const before = result.requests.filter((request) => request.path === '/api/backups/pbs').length;
    const reading = page.getByRole('button', { name: 'Reading backups…', exact: true });
    assert.equal(await reading.getAttribute('aria-disabled'), 'true');
    // Playwright refuses a tap on an aria-disabled target. Do not force it;
    // desktop keyboard exercises the handler's duplicate-read guard.
    if (!spec.touch) await activate(reading, spec);
    assert.equal(
      result.requests.filter((request) => request.path === '/api/backups/pbs').length,
      before,
    );
    assert(pendingPBS, 'The pending inventory read must have reached the actual server');
    holdPBS = false;
    assert(pendingPVE, 'Both source reads remain pending');
    pendingPVE();
    pendingPBS();
    await refresh.waitFor();
    await page.getByRole('button', { name: /: 6 backups$/ }).waitFor();
    assert.match(await summary.innerText(), /6 restore points/);
    assert.doesNotMatch(await summary.innerText(), /incomplete/);
    assert.equal(await page.getByText(/Incomplete backups remain listed/).count(), 0);
    assert.equal(await page.locator('[data-proxmox-backup-row="recoverable"]').count(), 6);
    assert.equal(
      await page
        .locator('[data-backup-artifact-state]')
        .filter({ hasText: /Running|Failed/ })
        .count(),
      0,
    );
    item.states.push({
      phase: 'completed',
      summary: await summary.innerText(),
      geometry: await readable(page),
    });
    await page.evaluate(() => window.scrollTo(0, 0));
    await capture(page, spec.name + '-completed');
    result.cases.push(item);
    await context.close();
    await browser.close();
    browser = undefined;
  }
  assert.deepEqual(result.errors, []);
  assert.deepEqual(result.unexpectedRequests, []);
})()
  .catch((error) => {
    result.failure = { message: error.message, stack: error.stack };
    process.exitCode = 1;
  })
  .finally(async () => {
    if (pendingPBS) pendingPBS();
    if (pendingPVE) pendingPVE();
    if (browser) {
      await browser.close();
    }
    result.cleanup.browserClosed = true;
    if (server) {
      await new Promise((resolve) => server.close(resolve));
    }
    result.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        cases: result.cases.length,
        captures: result.captures.length,
        failure: result.failure,
        errors: result.errors,
        cleanup: result.cleanup,
      }),
    );
  });
