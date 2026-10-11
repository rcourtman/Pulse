// Read-only browser acceptance of the changed report presentation/query flow.
// Explicit review/rerun business calls are covered by the runtime component
// suite. Every POST is refused here, so a clock-induced operation fails proof.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/maintenance-evidence/browser-read-only';
const origin = 'http://127.0.0.1:5347';
const start = Date.parse('2026-10-11T04:00:00Z');
const iso = (offset) => new Date(start + offset).toISOString();
const reports = ['healthy', 'pending', 'needs_review', 'failed_verification'].map((status) => ({
  id: 'report:' + status,
  resourceId: 'pbs-a',
  status,
  trigger: 'maintenance_window_ended',
  startedAt: iso(-30_000),
  completedAt: status === 'pending' ? '0001-01-01T00:00:00Z' : iso(-30_000),
  windowStartedAt: iso(-90_000_000),
  windowEndedAt: iso(-86_400_000),
  evidence: {
    activeCriticalAlerts: status === 'failed_verification' ? 1 : 0,
    activeWarningAlerts: 2,
    activeCriticalFindings: 0,
    activeWarningFindings: 1,
    failedActionsSinceWindowStart: 0,
    metricRecovery: { samplesAfterEnd: 3, trend: 'improving' },
  },
  linkedFindingIds: [],
  linkedAlertIds: [],
  linkedActionIds: [],
  recommendation: 'Recorded evidence, not a fresh health observation.',
  ...(status === 'healthy'
    ? {
        userOutcome: 'reviewed',
        reviewedAt: iso(-10_000),
        reviewedBy: 'fixture-admin',
        reviewNote: 'Earlier check retained. A completed backup does not prove guest recovery.',
      }
    : {}),
}));
async function bounded(promise, label) {
  let timer;
  try {
    return await Promise.race([
      promise,
      new Promise((_, reject) => {
        timer = setTimeout(
          () => reject(new Error('Fixture observation deadline: ' + label)),
          10_000,
        );
      }),
    ]);
  } finally {
    clearTimeout(timer);
  }
}
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const binding = JSON.parse(
    fs.readFileSync('/workspace/tmp/maintenance-evidence/source-binding.json'),
  );
  for (const [file, digest] of Object.entries(binding.content_sha256)) {
    assert.equal(
      crypto
        .createHash('sha256')
        .update(fs.readFileSync('/workspace/' + file))
        .digest('hex'),
      digest,
      file,
    );
  }
  const playwright = require('playwright/package.json').version;
  assert.equal(
    playwright,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const result = {
    source: binding,
    playwright,
    cases: [],
    captures: [],
    completed: false,
    browsersClosed: false,
    serverClosed: false,
  };
  const { createServer } = await import(root + '/node_modules/vite/dist/node/index.js');
  const server = await createServer({
    root,
    configFile: root + '/vite.config.ts',
    cacheDir: output + '/vite-cache',
    server: { host: '127.0.0.1', port: 5347, strictPort: true, watch: null },
  });
  let browser, context;
  try {
    await server.listen();
    for (const [name, engine, width, height, dark] of [
      ['desktop', chromium, 1365, 900, false],
      ['phone', webkit, 390, 844, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      context = await browser.newContext({
        viewport: { width, height },
        isMobile: dark,
        hasTouch: dark,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      const page = await context.newPage();
      page.setDefaultTimeout(10_000);
      page.setDefaultNavigationTimeout(45_000);
      await page.clock.install({ time: new Date(start - 1000) });
      await page.clock.pauseAt(new Date(start));
      await page.addInitScript((dark) => {
        localStorage.clear();
        sessionStorage.clear();
        document.addEventListener(
          'DOMContentLoaded',
          () => document.documentElement.classList.toggle('dark', dark),
          { once: true },
        );
      }, dark);
      const record = {
        name,
        browserVersion: browser.version(),
        viewport: { width, height },
        dark,
        checks: [],
        reads: [],
        writes: [],
        errors: [],
        unexpected: [],
        offOrigin: [],
        completed: false,
        contextClosed: false,
      };
      result.cases.push(record);
      let mode = 'pending',
        releaseRead,
        observeRead;
      const readStarted = new Promise((resolve) => {
        observeRead = resolve;
      });
      page.on('pageerror', (error) => record.errors.push(error.stack || error.message));
      await page.route('**/*', async (route) => {
        const req = route.request(),
          url = new URL(req.url());
        if (url.origin !== origin) {
          record.offOrigin.push(url.origin);
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (req.method() !== 'GET') {
          record.writes.push(url.pathname);
          return route.fulfill({
            status: 403,
            json: { error: 'Read-only fixture refuses operations' },
          });
        }
        record.reads.push(url.pathname);
        let data;
        if (url.pathname.endsWith('/maintenance-verifications')) {
          if (url.pathname.includes('/pending-resource/')) {
            if (mode === 'pending') {
              await bounded(
                new Promise((resolve) => {
                  releaseRead = resolve;
                  observeRead();
                }),
                'held new-resource read',
              );
              return route.fulfill({
                status: 503,
                json: { error: 'Synthetic unavailable reader' },
              });
            }
            if (mode === 'denied')
              return route.fulfill({ status: 403, json: { error: 'Synthetic access denial' } });
            return route.fulfill({
              json: { data: [], meta: { resourceId: 'pending-resource', limit: 25, total: 0 } },
            });
          }
          data = { data: reports, meta: { resourceId: 'pbs-a', limit: 25, total: reports.length } };
        } else if (url.pathname.endsWith('/operator-state'))
          data = {
            configured: true,
            state: {
              canonicalId: 'pbs-a',
              neverAutoRemediate: true,
              monitoringMode: 'expected_offline',
              lifecycleState: 'retired',
              note: 'Keep the saved protection',
              setAt: iso(-60_000),
            },
          };
        else if (url.pathname.endsWith('/facets'))
          data = { capabilities: [], relationships: [], recentChanges: [], counts: {} };
        else if (url.pathname === '/api/ai/intelligence')
          data = { resourceId: 'pbs-a', findings: [], investigations: [] };
        else if (url.pathname === '/api/audit/actions')
          data = { audits: [], count: 0, available: false };
        else if (url.pathname === '/api/license/runtime-capabilities')
          data = {
            capabilities: [],
            limits: [],
            hosted_mode: false,
            runtime: { build: 'community' },
            blocked_capabilities: [],
          };
        else if (url.pathname === '/api/ai/settings') data = { discovery_enabled: false };
        else if (url.pathname === '/api/security/status')
          data = { hasAuthentication: true, requiresAuth: false, sessionCapabilities: {} };
        else {
          record.unexpected.push(url.pathname);
          return route.fulfill({ status: 404, json: { error: 'Unspecified synthetic read' } });
        }
        return route.fulfill({ json: data });
      });
      const check = async (label, action) => {
        result.phase = name + ': ' + label;
        console.log(result.phase);
        await action();
        record.checks.push(label);
      };
      const capture = async (label) => {
        const file = output + '/' + name + '-' + label + '.png';
        await page.screenshot({ path: file, fullPage: true, animations: 'disabled' });
        result.captures.push(file.replace('/workspace/', ''));
      };
      result.phase = name + ': mount full drawer';
      await page.goto(origin + '/browser-tests/maintenance-evidence.html');
      await page.getByRole('tab', { name: 'Manage', exact: true }).click();
      const section = page.getByTestId('maintenance-verification-section');
      await section.getByText('Checked 30s ago', { exact: true }).first().waitFor();
      await page.getByLabel('Operator note', { exact: true }).fill('Unsaved protected note');
      await check(
        'separate recorded check/window/review ages, unknown pending completion and all four verdicts',
        async () => {
          assert.equal(
            await section.getByText('Window ended 1 day ago', { exact: true }).count(),
            4,
          );
          assert.equal(await section.getByText('Checked unknown', { exact: true }).count(), 1);
          assert.equal(
            await section.getByText('Reviewed 10s ago by fixture-admin', { exact: true }).count(),
            1,
          );
          assert.match(await section.innerText(), /Reports record past checks, not current health/);
          for (const status of ['healthy', 'pending', 'needs_review', 'failed_verification'])
            assert.equal(
              await section.getByTestId('maintenance-verification-status-' + status).count(),
              1,
            );
        },
      );
      await capture('recorded');
      const reads = record.reads.length;
      await page.clock.fastForward(60_000);
      await check(
        'held events age past a minute without reads/writes, changing verdicts or discarding edits',
        async () => {
          await section.getByText('Checked 1 min ago', { exact: true }).first().waitFor();
          assert.equal(
            await section.getByText('Reviewed 1 min ago by fixture-admin', { exact: true }).count(),
            1,
          );
          assert.equal(
            await page.getByLabel('Operator note', { exact: true }).inputValue(),
            'Unsaved protected note',
          );
          assert.equal(record.reads.length, reads);
          assert.equal(record.writes.length, 0);
        },
      );
      await page.clock.fastForward(3_600_000);
      await check(
        'an hour-old verdict stays historical with its evidence, review admission and saved restrictions',
        async () => {
          await section.getByText('Checked 1 hour ago', { exact: true }).first().waitFor();
          assert.equal(
            await section
              .getByText('Reviewed 1 hour ago by fixture-admin', { exact: true })
              .count(),
            1,
          );
          assert.equal(await section.getByTestId('maintenance-verification-review').count(), 3);
          assert.equal(
            await page
              .getByLabel('Never auto-remediate this resource', { exact: true })
              .getAttribute('aria-pressed'),
            'true',
          );
          assert.equal(
            await page.getByLabel('Monitoring', { exact: true }).inputValue(),
            'expected_offline',
          );
          assert.equal(await page.getByLabel('Lifecycle', { exact: true }).inputValue(), 'retired');
          assert.equal(record.reads.length, reads);
          assert.equal(record.writes.length, 0);
        },
      );
      await capture('hour-old');
      await page.evaluate(() => window.__maintenanceEvidence.select('pending-resource'));
      // The real drawer deliberately remounts at a different canonical ID and
      // starts on Overview. Open Manage as an operator would; do not assume a
      // held tab or misattribute that existing protection to this repair.
      await page.getByRole('tab', { name: 'Manage', exact: true }).click();
      await bounded(readStarted, 'new-resource GET observed');
      await check(
        'changing resource immediately withdraws prior reports while its own read waits',
        async () => {
          await section.getByText('Loading verification reports…', { exact: true }).waitFor();
          assert.equal(await section.getByTestId('maintenance-verification-report').count(), 0);
          assert.equal(await section.getByText(/No verification reports yet/).count(), 0);
          await page.clock.fastForward(60_000);
          assert.equal(await section.getByTestId('maintenance-verification-report').count(), 0);
        },
      );
      await capture('other-resource-pending');
      releaseRead();
      await check(
        'a failed first read is unavailable, not empty history or another resource evidence',
        async () => {
          await section.getByTestId('maintenance-verification-load-error').waitFor();
          assert.equal(await section.getByText(/No verification reports yet/).count(), 0);
          assert.equal(await section.getByTestId('maintenance-verification-report').count(), 0);
          const before = record.reads.length;
          await page.clock.fastForward(60_000);
          assert.equal(record.reads.length, before);
        },
      );
      await capture('unavailable');
      mode = 'denied';
      await section.getByRole('button', { name: 'Retry reports', exact: true }).focus();
      await section.getByRole('button', { name: 'Retry reports', exact: true }).press('Enter');
      await check(
        'explicit keyboard read retry shows bounded denied guidance without reviving reports',
        async () => {
          await section
            .getByText('Access denied. Check your permissions and license plan.', { exact: true })
            .waitFor();
          const before = record.reads.length;
          await page.clock.fastForward(60_000);
          assert.equal(record.reads.length, before);
          assert.equal(await section.getByTestId('maintenance-verification-report').count(), 0);
          assert.equal(record.writes.length, 0);
        },
      );
      await capture('denied');
      mode = 'empty';
      await section.getByRole('button', { name: 'Retry reports', exact: true }).click();
      await check(
        'only a successful empty read is empty history, with no subsequent clock retry',
        async () => {
          await section.getByText(/No verification reports yet/).waitFor();
          assert.equal(await section.getByTestId('maintenance-verification-load-error').count(), 0);
          const before = record.reads.length;
          await page.clock.fastForward(60_000);
          assert.equal(record.reads.length, before);
          assert.equal(await section.getByTestId('maintenance-verification-report').count(), 0);
        },
      );
      await capture('empty');
      await check(
        'no page error, unspecified/off-origin request, operation or horizontal overflow',
        async () => {
          assert.deepEqual(record.errors, []);
          assert.deepEqual(record.unexpected, []);
          assert.deepEqual(record.offOrigin, []);
          assert.deepEqual(record.writes, []);
          assert.equal(
            await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
            true,
          );
        },
      );
      record.completed = true;
      await context.close();
      record.contextClosed = true;
      context = undefined;
      await browser.close();
      browser = undefined;
    }
    result.completed = true;
  } catch (error) {
    result.error = { name: error.name, message: error.message };
    throw error;
  } finally {
    if (context) await context.close();
    if (browser) await browser.close();
    result.browsersClosed = true;
    await server.close();
    result.serverClosed = true;
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
    console.log(
      JSON.stringify({
        completed: result.completed,
        phase: result.phase,
        checks: result.cases.map((c) => ({
          name: c.name,
          count: c.checks.length,
          errors: c.errors,
        })),
        captures: result.captures.length,
        serverClosed: result.serverClosed,
      }),
    );
  }
})();
