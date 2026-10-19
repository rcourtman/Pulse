// Read-only emitted HTTP frames -> production page/query/reader/table/CSS.
// No provider, guest, backup, restore, sync or recovery command is executed.
process.env.RAYON_NUM_THREADS = '2';
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const { chromium, webkit } = require('playwright');

const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/replication-live/browser');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(root, 'tmp/replication-live/binding.json')));
const frameFile = path.join(frontend, 'browser-tests/fixtures/replication-read-http.json');
const emitted = JSON.parse(fs.readFileSync(frameFile));
assert.equal(
  hash(fs.readFileSync(frameFile)),
  '465cb0e7df7b3fa2abba108b9c2f88ec77590dea947f048c60ed7de9e5e5145f',
);
assert.deepEqual(emitted['short-inventory'], emitted.initial);

// Extra presentation controls derive only synthetic labels/state from the
// recovered observation; they are not claimed to be producer-emitted frames.
const changed = structuredClone(emitted.recovered);
const target = changed.data.find((job) => job.id === 'site-a-100-0');
Object.assign(target, {
  guestName: 'renamed-replica',
  sourceNode: 'edge-a',
  targetNode: 'edge-c',
  schedule: '*/30',
  comment: '<b>literal new comment</b>',
  error: '<b>New source snapshot unavailable</b>',
  failCount: 0,
  lastSyncDurationSeconds: 18,
  lastSyncDurationHuman: '18s observed',
  lastSyncUnix: 1792407600,
  nextSyncUnix: 1792411200,
});
changed.data.unshift({
  ...emitted.recovered.data[0],
  id: 'site-a-100-1',
  jobId: '100-1',
  guestName: 'same-guest-sibling',
});
changed.meta.total = changed.data.length;
const clean = structuredClone(changed);
const cleanTarget = clean.data.find((job) => job.id === 'site-a-100-0');
delete cleanTarget.error;
delete cleanTarget.comment;
clean.data.reverse();
const frames = { ...emitted, 'metadata-error': changed, 'metadata-clean': clean };
const result = {
  passed: false,
  source: binding.source,
  tree: binding.tree,
  playwright: require('playwright/package.json').version,
  runner_sha256: hash(fs.readFileSync(__filename)),
  binding_sha256: hash(fs.readFileSync(path.join(root, 'tmp/replication-live/binding.json'))),
  producer: {
    source: '5ff10c9168cfd011222405fe2b6a9f326d9c3c06',
    frame_sha256: hash(fs.readFileSync(frameFile)),
    origin:
      'Core 20261019T110002Z BROWSER-INPUTS.tar.xz, final-normal/http-frames.json; not regenerated here',
    emitted: Object.keys(emitted),
    derived: ['metadata-error', 'metadata-clean'],
  },
  cases: [],
  captures: [],
  unexpected: [],
  errors: [],
  cleanup: {},
  limits:
    'Current committed-source dev preview with saved client/monitor/handler frames and explicitly synthetic metadata controls. No production embed, native provider/TLS, replication, guest health, restore, thaw, filesystem liveness, installed recovery, release or screen-reader speech proof.',
};
let server, browser, current, origin;
const checkBinding = () => {
  for (const [file, digest] of Object.entries(binding.files))
    assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, file);
};
function serve(request, response, next) {
  if (!request.url.startsWith('/api/')) return next();
  try {
    assert.equal(request.method, 'GET');
    const url = new URL(request.url, 'http://fixture.invalid');
    let payload;
    if (url.pathname === '/api/replication/jobs') {
      assert.equal(url.search, '?platform=proxmox-pve');
      payload = frames[current.phase];
    } else if (url.pathname === '/api/resources') {
      payload = {
        data: [
          {
            id: 'fixture-node',
            name: 'node-a',
            type: 'agent',
            status: 'online',
            sources: ['proxmox'],
            platformScopes: ['proxmox-pve'],
            lastSeen: '2026-10-19T11:00:00Z',
            proxmox: { nodeName: 'node-a', instance: 'site-a', pveVersion: '9.2' },
            metrics: {
              cpu: { percent: 5 },
              memory: { percent: 25, used: 2147483648, total: 8589934592 },
            },
          },
        ],
        meta: { totalPages: 1 },
        facets: { byType: { agent: 1 }, incidentCount: 0 },
      };
    } else if (url.pathname === '/api/license/runtime-capabilities')
      payload = { capabilities: [], limits: [], max_history_days: 7 };
    else if (url.pathname === '/api/alerts/active') payload = [];
    else if (url.pathname === '/api/alerts/config')
      payload = { guestDefaults: {}, hostDefaults: {} };
    else payload = { data: [], meta: { totalPages: 1 }, alerts: [], anomalies: [] };
    const body = JSON.stringify(payload);
    current.reads.push({
      path: url.pathname + url.search,
      phase: current.phase,
      sha256: hash(body),
    });
    response.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
    response.end(body);
  } catch (error) {
    result.errors.push(String(error.stack || error));
    response.writeHead(500).end('{}');
  }
}

(async () => {
  fs.mkdirSync(output, { recursive: true });
  try {
    checkBinding();
    assert.equal(
      result.playwright,
      JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    const { createServer } = await import(
      pathToFileURL(path.join(frontend, 'node_modules/vite/dist/node/index.js')).href
    );
    server = await createServer({
      root: frontend,
      configFile: path.join(frontend, 'vite.config.ts'),
      cacheDir: path.join(output, 'vite-cache'),
      logLevel: 'error',
      plugins: [
        {
          name: 'replication-observation-http',
          configureServer(s) {
            s.middlewares.use(serve);
          },
        },
      ],
      server: { host: '127.0.0.1', port: 0, strictPort: false, proxy: {} },
    });
    await server.listen();
    origin = `http://127.0.0.1:${server.httpServer.address().port}`;
    for (const spec of [
      { name: 'chromium', engine: chromium, width: 1440, height: 1000, touch: false, dark: false },
      { name: 'webkit', engine: webkit, width: 320, height: 1000, touch: true, dark: true },
    ]) {
      browser = await spec.engine.launch(
        spec.name === 'chromium'
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      current = {
        engine: spec.name,
        browserVersion: browser.version(),
        viewport: { width: spec.width, height: spec.height },
        phase: 'initial',
        reads: [],
        checks: [],
        pageErrors: [],
      };
      result.cases.push(current);
      const context = await browser.newContext({
        viewport: current.viewport,
        hasTouch: spec.touch,
        isMobile: spec.touch,
        colorScheme: spec.dark ? 'dark' : 'light',
        timezoneId: 'UTC',
        locale: 'en-GB',
      });
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      await page.clock.install({ time: new Date('2026-10-19T11:01:00Z') });
      if (spec.dark)
        await page.addInitScript(() =>
          document.addEventListener('DOMContentLoaded', () =>
            document.documentElement.classList.add('dark'),
          ),
        );
      await page.routeWebSocket(/\/ws(?:\?|$)/, () => {});
      await page.route('**/*', async (route) => {
        const request = route.request(),
          url = new URL(request.url());
        if (url.origin !== origin || !['GET', 'HEAD'].includes(request.method())) {
          result.unexpected.push(`${request.method()} ${url.origin}${url.pathname}`);
          return route.abort();
        }
        return route.continue();
      });
      page.on('pageerror', (error) => current.pageErrors.push(error.message));
      const row = (name) =>
        page
          .locator('tr')
          .filter({ has: page.locator('button[data-row-action="true"]') })
          .filter({ hasText: name });
      const check = (condition, message) => {
        assert(condition, message);
        current.checks.push(message);
      };
      const activate = async (button) => {
        if (spec.touch) await button.tap();
        else {
          await button.focus();
          await page.keyboard.press('Enter');
        }
      };
      const capture = async (state) => {
        await page.clock.runFor(250);
        const geometry = await page.evaluate(() => ({
          overflow: document.documentElement.scrollWidth > innerWidth,
          clippedDetails: [...document.querySelectorAll('dl dd')]
            .filter((element) => {
              const r = element.getBoundingClientRect();
              return r.left < -1 || r.right > innerWidth + 1;
            })
            .map((element) => element.previousElementSibling.textContent),
          focused: document.activeElement?.getAttribute('aria-label'),
          expanded: [
            ...document.querySelectorAll('button[aria-expanded="true"][aria-controls]'),
          ].map((element) => element.getAttribute('aria-controls')),
        }));
        assert.equal(geometry.overflow, false, state);
        assert.deepEqual(geometry.clippedDetails, [], state);
        const file = path.join(output, `${spec.name}-${state}.png`);
        await page.screenshot({ path: file, fullPage: true, animations: 'disabled' });
        result.captures.push({
          path: path.relative(root, file),
          sha256: hash(fs.readFileSync(file)),
          state,
          engine: spec.name,
          viewport: current.viewport,
          geometry,
        });
      };
      const phase = async (name, targetName, verdict) => {
        current.phase = name;
        const observed = page.waitForResponse(
          (response) => new URL(response.url()).pathname === '/api/replication/jobs',
        );
        await page.clock.runFor(30100);
        const response = await observed;
        await response.finished();
        assert.deepEqual(await response.json(), frames[name], `Completed HTTP frame ${name}`);
        current.checks.push(`Ordinary polling received exact ${name} HTTP frame`);
        if (verdict) await row(targetName).getByText(verdict, { exact: true }).waitFor();
      };
      await page.goto(origin + '/browser-tests/replication-inventory.html', {
        waitUntil: 'domcontentloaded',
        timeout: 60000,
      });
      await row('protected-replica').getByText('Failed', { exact: true }).waitFor();
      check(
        new URL(page.url()).pathname === '/proxmox/replication',
        'Production page/router consumed the HTTP observations',
      );
      if (spec.touch) await page.getByRole('button', { name: /^Filters/ }).tap();
      const summary = row('protected-replica');
      const toggle = summary.getByRole('button', { name: /Expand details/ });
      if (spec.touch) await summary.tap();
      else await activate(toggle);
      // Preserve a real native disclosure focus owner during every live update,
      // including phone rendering after its touch-operated row disclosure.
      const disclosure = summary.getByRole('button', { name: /Collapse details/ });
      await disclosure.focus();
      const controls = await disclosure.getAttribute('aria-controls');
      const details = page.locator(`[id="${controls}"]`);
      const summaryHandle = await summary.elementHandle();
      const toggleHandle = await disclosure.elementHandle();
      const detailHandle = await details.elementHandle();
      const field = (label) =>
        details
          .locator('dt')
          .filter({ hasText: new RegExp(`^${label}$`) })
          .locator('xpath=following-sibling::dd[1]');
      const retained = async (name, verdict) => {
        check(
          await row(name).evaluate((element, previous) => element === previous, summaryHandle),
          'Same mounted summary through ' + current.phase,
        );
        check(
          await row(name)
            .getByRole('button', { name: /Collapse details/ })
            .evaluate((element, previous) => element === previous, toggleHandle),
          'Same expanded disclosure through ' + current.phase,
        );
        check(
          await details.evaluate((element, previous) => element === previous, detailHandle),
          'Same open details through ' + current.phase,
        );
        check(
          await toggleHandle.evaluate((element) => element === document.activeElement),
          'Native disclosure retains focus through ' + current.phase,
        );
        const status = row(name).getByText(verdict, { exact: true });
        await status.waitFor();
        const tone = { Failed: 'text-red-600', Unknown: 'text-amber-600', OK: 'text-emerald-600' }[
          verdict
        ];
        check(
          (await status.getAttribute('class')).includes(tone),
          'Verdict tone follows ' + verdict,
        );
        if (!spec.touch)
          check(
            (await row(name).locator(`span[title="${verdict}"][aria-hidden="true"]`).count()) === 1,
            'Status dot follows ' + verdict,
          );
        const ids = await page
          .locator('button[data-row-action="true"][aria-controls]')
          .evaluateAll((buttons) => buttons.map((button) => button.getAttribute('aria-controls')));
        check(
          new Set(ids).size === ids.length,
          'Unique disclosure controls through ' + current.phase,
        );
      };
      assert.equal(await field('Error').innerText(), 'Synthetic source snapshot unavailable');
      await retained('protected-replica', 'Failed');
      await phase('short-inventory', 'protected-replica', 'Failed');
      await retained('protected-replica', 'Failed');
      check(
        (await row('successful-sibling').count()) === 1 &&
          (await row('independent-replica').count()) === 1,
        'Rejected short inventory does not remove observed jobs',
      );
      await phase('ambiguous-status', 'protected-replica', 'Unknown');
      await retained('protected-replica', 'Unknown');
      await page.getByRole('button', { name: 'Unknown, 1', exact: true }).waitFor();
      await page.getByRole('button', { name: 'Failed, 0', exact: true }).waitFor();
      check(
        (await details.locator('[data-replication-job-error]').count()) === 0,
        'Unknown removes the previous error, not merely its count',
      );
      assert.equal(await field('Reported last result').innerText(), 'Not reported');
      assert.match(
        await field('Interpretation').innerText(),
        /This does not mean a sync is pending/,
      );
      await capture('unknown-open');
      await phase('recovered', 'protected-replica', 'OK');
      await retained('protected-replica', 'OK');
      assert.match(await field('Interpretation').innerText(), /last reported sync succeeded/);
      check(
        (await details.locator('[data-replication-job-error]').count()) === 0,
        'Successful observation remains free of the retired error',
      );
      await capture('recovered-open');
      await phase('metadata-error', 'renamed-replica', 'Failed');
      // The locator retains the same scoped row, not the old mutable name.
      await retained('renamed-replica', 'Failed');
      assert.equal(await field('Guest').innerText(), '100 (renamed-replica)');
      assert.equal(await field('Route').innerText(), 'edge-a → edge-c');
      assert.equal(await field('Schedule').innerText(), '*/30');
      assert.equal(await field('Duration').innerText(), '18s observed');
      assert.equal(await field('Failures').innerText(), '0');
      assert.equal(await field('Comment').innerText(), '<b>literal new comment</b>');
      assert.equal(await field('Error').innerText(), '<b>New source snapshot unavailable</b>');
      check(
        (await details.locator('b').count()) === 0,
        'Changed provider text is literal, never markup',
      );
      assert.match(
        await field('Interpretation').innerText(),
        /previous successful sync does not override/,
      );
      check(
        (await row('renamed-replica').locator('td[title="100 (renamed-replica)"]').count()) === 1,
        'Guest title follows the changed name',
      );
      for (const name of ['same-guest-sibling', 'successful-sibling', 'independent-replica']) {
        await row(name).getByText('OK', { exact: true }).waitFor();
        check(
          (await row(name)
            .getByRole('button', { name: /Expand details/ })
            .count()) === 1,
          'Independent job remains collapsed and healthy: ' + name,
        );
      }
      await capture('changed-error-open');
      await phase('metadata-clean', 'renamed-replica', 'OK');
      await retained('renamed-replica', 'OK');
      check(
        (await details.locator('[data-replication-job-error]').count()) === 0 &&
          (await details
            .locator('dt')
            .filter({ hasText: /^Comment$/ })
            .count()) === 0,
        'Clean observation removes error and comment in the existing reordered disclosure',
      );
      await capture('clean-reordered-open');
      // Only after proving unchanged owners through all transitions, exercise
      // filters which intentionally unmount a job no longer matching them.
      await phase('ambiguous-status', 'protected-replica', 'Unknown');
      await activate(page.getByRole('button', { name: 'Unknown, 1', exact: true }));
      check(
        (await row('protected-replica').count()) === 1 &&
          (await row('independent-replica').count()) === 0,
        'Unknown filter is scoped to the unrecognised observation',
      );
      await phase('recovered');
      await page.getByRole('button', { name: 'Unknown, 0', exact: true }).waitFor();
      await page.getByText('No replication jobs match current filters', { exact: true }).waitFor();
      await activate(page.getByRole('button', { name: 'OK, 3', exact: true }));
      await row('protected-replica').getByText('OK', { exact: true }).waitFor();
      await phase('empty');
      await row('independent-replica').getByText('OK', { exact: true }).waitFor();
      check(
        (await row('protected-replica').count()) === 0 &&
          (await row('successful-sibling').count()) === 0,
        'Authoritative empty removes only its connection, not the same VMID on another connection',
      );
      check(
        (await page.locator('[data-replication-job-error]').count()) === 0,
        'Removed connection leaves no stale failure disclosure',
      );
      assert.deepEqual(current.pageErrors, []);
      current.replicationReads = current.reads.filter((read) =>
        read.path.startsWith('/api/replication/jobs'),
      );
      assert.equal(
        current.replicationReads.length,
        9,
        'Exactly nine ordinary inventory reads, no forced reload or extra request',
      );
      await context.close();
      current.contextClosed = true;
      await browser.close();
      current.browserClosed = true;
      browser = undefined;
    }
    result.loadedModules = [];
    for (const [file] of server.moduleGraph.fileToModulesMap) {
      if (file.startsWith(frontend + '/src/') && fs.existsSync(file)) {
        const relative = path.relative(root, file),
          digest = hash(fs.readFileSync(file));
        assert.equal(binding.files[relative], digest, relative);
        result.loadedModules.push({ file: relative, sha256: digest });
      }
    }
    for (const file of [
      'frontend-modern/src/features/proxmox/ProxmoxPageSurface.tsx',
      'frontend-modern/src/features/proxmox/ProxmoxReplicationTable.tsx',
      'frontend-modern/src/features/platformPage/PlatformWindowedRows.tsx',
      'frontend-modern/src/hooks/createNonSuspendingQuery.ts',
    ])
      assert(
        result.loadedModules.some((module) => module.file === file),
        file,
      );
    checkBinding();
    assert.deepEqual(result.unexpected, []);
    assert.deepEqual(result.errors, []);
    result.passed = true;
  } catch (error) {
    result.error = String(error.stack || error);
    process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    result.cleanup.browserClosed = true;
    if (server) await server.close();
    result.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        passed: result.passed,
        captures: result.captures.length,
        error: result.error,
        cleanup: result.cleanup,
      }),
    );
  }
})();
