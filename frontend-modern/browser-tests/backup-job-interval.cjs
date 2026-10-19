// Production HTTP readers -> recovery model -> Coverage details and shipped CSS.
// Synthetic collected history only: no backup, guest, probe or native recovery.
process.env.RAYON_NUM_THREADS = '2';
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/backup-job-interval');
const origin = 'http://127.0.0.1:5327';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(output, 'binding.json')));
const result = {
  binding,
  protocol_sha256: hash(fs.readFileSync(__filename)),
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  geometry: [],
  responses: [],
  errors: [],
  unexpected: [],
  cleanup: {},
};
const task = {
  id: 'UPID:pve-a:0000001A:0000002B:68F4C500:vzdump:112:backup-operator@pve:',
  vmid: 112,
  type: 'ct',
  node: 'pve-a',
  instance: 'pve-a',
  status: 'OK',
  startTime: '2026-10-19T05:00:00Z',
  endTime: '2026-10-19T09:34:21Z',
  observedAt: '2026-10-19T10:00:00Z',
};
const posture = {
  subjectResourceId: 'ct-112',
  state: 'unknown',
  freshness: 'unknown',
  verification: 'unknown',
  coverage: 'unknown',
  providerStates: [],
  repositoryResourceIds: [],
  evidenceIds: [],
  explanation: 'Synthetic posture stays independent of reported job times.',
  evaluatedAt: '2026-10-19T11:00:00Z',
};
let browser,
  server,
  tasks = [task];
async function geometry(page, name, state) {
  const observation = await page.evaluate(() => {
    const overflow = document.documentElement.scrollWidth > innerWidth + 1;
    const fields = [
      ...document.querySelectorAll('[data-backup-task-detail] dd, [data-backup-task-evidence] dd'),
    ].map((node) => {
      const box = node.getBoundingClientRect();
      const badGlyphs = [];
      const walker = document.createTreeWalker(node, NodeFilter.SHOW_TEXT);
      const range = document.createRange();
      while (walker.nextNode()) {
        const text = walker.currentNode;
        for (let i = 0; i < text.textContent.length; i++) {
          if (/\s/u.test(text.textContent[i])) continue;
          range.setStart(text, i);
          range.setEnd(text, i + 1);
          for (const r of range.getClientRects()) {
            // A zero-area caret at a wrapped line break paints no glyph.
            if (r.width > 0 && r.height > 0 && (r.left < box.left - 1 || r.right > box.right + 1))
              badGlyphs.push(text.textContent[i]);
          }
        }
      }
      return {
        text: node.textContent,
        width: box.width,
        client: node.clientWidth,
        scroll: node.scrollWidth,
        badGlyphs,
      };
    });
    return { overflow, fields };
  });
  result.geometry.push({ name, state, ...observation });
  assert.equal(observation.overflow, false, state + ' page overflow');
  assert(observation.fields.length > 0);
  for (const field of observation.fields) {
    assert(field.scroll <= field.client + 1, state + ' clipped field: ' + field.text);
    assert.equal(field.badGlyphs.length, 0, state + ' clipped glyphs: ' + field.text);
  }
}
async function capture(page, name, state) {
  // CSS entry transitions must settle before retaining pixels.
  await page.waitForFunction(() =>
    document.getAnimations().every((animation) => animation.playState !== 'running'),
  );
  await geometry(page, name, state);
  const file = path.join(output, name + '-' + state + '.png');
  await page.screenshot({ path: file, fullPage: true, animations: 'disabled' });
  result.captures.push({
    name,
    state,
    file: path.relative(root, file),
    sha256: hash(fs.readFileSync(file)),
  });
}
(async () => {
  assert.match(binding.source_sha, /^[a-f0-9]{40}$/);
  for (const [file, digest] of Object.entries(binding.files))
    assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, file);
  assert.equal(
    result.playwright,
    JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const { createServer } = await import(
    path.join(frontend, 'node_modules/vite/dist/node/index.js')
  );
  server = await createServer({
    root: frontend,
    cacheDir: path.join(output, 'vite-cache'),
    optimizeDeps: { entries: ['browser-tests/backup-inventory.html'] },
    server: { host: '127.0.0.1', port: 5327, strictPort: true },
  });
  await server.listen();
  const profiles = binding.profiles ?? ['desktop', 'narrow'];
  assert(
    Array.isArray(profiles) &&
      profiles.length > 0 &&
      profiles.every((name) => ['desktop', 'narrow'].includes(name)),
  );
  for (const [name, engine, width] of [
    ['desktop', chromium, 1365],
    ['narrow', webkit, 320],
  ].filter(([name]) => profiles.includes(name))) {
    tasks = [task];
    browser = await engine.launch(
      engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    const context = await browser.newContext({
      viewport: { width, height: 1000 },
      hasTouch: width === 320,
      isMobile: width === 320,
      locale: 'en-GB',
      timezoneId: 'Europe/London',
    });
    const page = await context.newPage();
    page.setDefaultTimeout(12000);
    page.on('pageerror', (error) => result.errors.push(error.message));
    page.on('console', (message) => {
      if (message.type() === 'error') result.errors.push(message.text());
    });
    await context.route('**/*', async (route) => {
      const request = route.request(),
        url = new URL(request.url());
      if (url.origin !== origin) {
        result.unexpected.push('off-origin');
        return route.abort();
      }
      if (!url.pathname.startsWith('/api/')) return route.continue();
      if (
        request.method() !== 'GET' ||
        !['/api/backups/pve', '/api/backups/pbs', '/api/recovery/postures'].includes(url.pathname)
      ) {
        result.unexpected.push(request.method() + ' ' + url.pathname);
        return route.abort();
      }
      const body =
        url.pathname === '/api/backups/pve'
          ? {
              data: { backupTasks: structuredClone(tasks), storageBackups: [], guestSnapshots: [] },
            }
          : url.pathname === '/api/backups/pbs'
            ? { data: { backups: [] } }
            : {
                data: [posture],
                policy: {
                  freshnessWindowSeconds: 604800,
                  verificationWindowSeconds: 604800,
                  requireVerification: true,
                },
                meta: { total: 1, totalPages: 1 },
              };
      result.responses.push({ name, path: url.pathname, method: request.method(), body });
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(body),
      });
    });
    await page.clock.setFixedTime(new Date('2026-10-19T12:00:00Z'));
    await page.goto(origin + '/browser-tests/backup-inventory.html?view=coverage', {
      waitUntil: 'domcontentloaded',
      timeout: 90000,
    });
    if (width === 320) await page.evaluate(() => document.documentElement.classList.add('dark'));
    await page.getByRole('link', { name: 'Coverage', exact: true }).click();
    const row = page.locator('[data-proxmox-backup-row="coverage"]').first();
    await row.waitFor();
    const toggle = row.getByRole('button', { expanded: false });
    if (width === 320) await row.locator('td').first().tap();
    else {
      await toggle.focus();
      await page.keyboard.press('Enter');
    }
    const detail = page.locator('[data-backup-task-detail]');
    const field = (label, scope = detail) =>
      scope
        .locator('dt')
        .filter({ hasText: new RegExp('^' + label + '$') })
        .locator('xpath=following-sibling::dd[1]');
    const read = async (label, scope = detail) => (await field(label, scope).innerText()).trim();
    const expectText = async (label, text, scope = detail) =>
      assert.equal(await read(label, scope), text);
    await detail.waitFor();
    await expectText('Job ID', task.id);
    assert.equal(
      await field('Reported end').locator('time').getAttribute('datetime'),
      task.endTime,
    );
    assert.equal(
      await field('Last observed').locator('time').getAttribute('datetime'),
      task.observedAt,
    );
    assert.match(await read('Reported end'), /10:34:21.*BST/);
    await expectText('Recorded duration', '4h 34m');
    await expectText('Result', 'OK');
    assert.match(await detail.innerText(), /not evidence of guest thaw or\s+filesystem writes/);
    await capture(page, name, 'recorded');
    const search = page.getByPlaceholder('Search backups by workload, node, source, or status');
    const beforeSearch = result.responses.length;
    await search.fill(task.id);
    await page.waitForURL((url) => url.searchParams.get('q') === task.id);
    assert.equal(await page.locator('[data-proxmox-backup-row="coverage"]').count(), 1);
    await search.fill('UPID:another-job');
    await page.waitForURL((url) => url.searchParams.get('q') === 'UPID:another-job');
    assert.equal(await page.locator('[data-proxmox-backup-row="coverage"]').count(), 0);
    await search.fill('');
    await page.waitForURL((url) => !url.searchParams.has('q'));
    assert.equal(result.responses.length, beforeSearch, 'native ID search is local');
    // A filtered-away row may remount, so reopen only if its disclosure was retired.
    const expand = page.locator('[data-proxmox-backup-row="coverage"]').first().getByRole('button');
    if ((await expand.getAttribute('aria-expanded')) === 'false') {
      if (width === 320) await row.locator('td').first().tap();
      else {
        await expand.focus();
        await page.keyboard.press('Enter');
      }
    }
    const refreshButton = page.getByRole('button', { name: 'Refresh backups', exact: true });
    const refresh = async (next) => {
      tasks = next;
      await refreshButton.waitFor();
      const before = result.responses.length;
      const response = page.waitForResponse(
        (r) => new URL(r.url()).pathname === '/api/backups/pve',
      );
      if (width === 320) await refreshButton.tap();
      else {
        await refreshButton.focus();
        await page.keyboard.press('Enter');
      }
      await response;
      await refreshButton.waitFor();
      assert.equal(result.responses.length - before, 3, 'only ordinary PVE/PBS/posture reads');
      assert.equal(await expand.getAttribute('aria-expanded'), 'true', 'refresh keeps row open');
    };
    const checks = [
      'exact latest native ID and local search',
      'reported end distinct from observation',
      'seconds and local timezone',
      'keyboard/touch disclosure',
    ];
    await refresh([{ ...task, id: 'native-running-job', status: 'running', endTime: undefined }]);
    await expectText('Job ID', 'native-running-job');
    await expectText('Reported end', 'Not reported');
    await expectText('Recorded duration', 'Unknown');
    await expectText('Result', 'Running');
    assert.equal(await field('Reported end').locator('time').count(), 0);
    await capture(page, name, 'not-reported');
    const invalidEnds = [
      '',
      '0001-01-01T00:00:00Z',
      'not-a-date',
      '2026-10-19T04:59:59Z',
      '2026-10-19T13:00:00Z',
      '<img src="/unexpected"><script>unsafe()</script>',
    ];
    for (const endTime of invalidEnds) {
      await refresh([{ ...task, endTime }]);
      await expectText('Reported end', endTime ? 'Unknown · reported: ' + endTime : 'Not reported');
      await expectText('Recorded duration', 'Unknown');
      await expectText('Result', 'OK');
      assert.equal(await field('Reported end').locator('time, img, script').count(), 0);
      await geometry(page, name, 'invalid-end-' + invalidEnds.indexOf(endTime));
    }
    await capture(page, name, 'invalid-end');
    await refresh([{ ...task, endTime: task.startTime }]);
    await expectText('Recorded duration', '0s');
    assert.equal(
      await field('Reported end').locator('time').getAttribute('datetime'),
      task.startTime,
    );
    await refresh([{ ...task, id: 'unorderable-job', startTime: 'not-a-date' }]);
    await page.locator('[data-backup-task-evidence]').waitFor();
    assert.equal(await detail.count(), 0);
    const collected = page.locator('[data-backup-task-evidence]');
    await expectText('Job ID', 'unorderable-job', collected);
    await expectText('Recorded duration', 'Unknown', collected);
    assert.equal(await field('Reported end', collected).locator('time').count(), 0);
    await refresh([
      {
        ...task,
        id: 'native-recovered-job',
        startTime: '2026-10-19T10:30:00Z',
        endTime: '2026-10-19T10:35:01Z',
      },
    ]);
    await detail.waitFor();
    await expectText('Job ID', 'native-recovered-job');
    await expectText('Recorded duration', '5m 1s');
    assert.equal(await page.locator('[data-backup-task-evidence]').count(), 0);
    assert(!(await detail.innerText()).includes('not-a-date'));
    await geometry(page, name, 'recovered');
    checks.push(
      'missing/zero/invalid/reversed/future/markup ends',
      'no live elapsed duration',
      'unknown chronology keeps raw evidence without latest claim',
      'ordinary same-open replacement and recovery',
      'GET-only source containment',
    );
    result.cases.push({
      name,
      browser: browser.version(),
      viewport: { width, height: 1000 },
      checks,
    });
    await context.close();
    await browser.close();
    browser = undefined;
    result.cleanup[name] = true;
  }
  assert.deepEqual(result.errors, []);
  assert.deepEqual(result.unexpected, []);
  result.result = 'passed';
})()
  .catch((error) => {
    result.result = 'failed';
    result.failure = error.stack;
    process.exitCode = 1;
  })
  .finally(async () => {
    try {
      if (browser) {
        await browser.close();
        result.cleanup.interrupted_browser = true;
      }
    } finally {
      if (server) {
        await server.close();
        result.cleanup.server = true;
      }
      fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2));
      console.log(
        JSON.stringify({
          result: result.result,
          failure: result.failure,
          cases: result.cases,
          captures: result.captures,
          cleanup: result.cleanup,
        }),
      );
    }
  });
