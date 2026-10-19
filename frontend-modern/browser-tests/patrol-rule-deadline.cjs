// Real Patrol Activity/router, HTTP clients and shipped CSS; synthetic rules only.
// A delayed DELETE may already have succeeded. Never replay it to obtain proof.
process.env.RAYON_NUM_THREADS = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/patrol-rule-deadline');
const origin = 'http://127.0.0.1:5328';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(output, 'binding.json')));
const report = {
  binding,
  protocol_sha256: hash(fs.readFileSync(__filename)),
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  errors: [],
  unexpected: [],
  cleanup: {},
};
const now = '2026-10-19T12:00:00Z';
const manual = (name) => ({
  id: `rule_${name}_backup`,
  resource_id: `fixture-${name}`,
  resource_name: `${name} VM`,
  category: 'backup',
  description: 'Off-site backup policy; keep other decisions and history.',
  created_from: 'manual',
  created_at: now,
});
const neighbour = manual('Keep');
const broad = { ...manual('Broad'), resource_id: '', resource_name: '', category: '' };
const dismissed = { ...manual('Dismissed'), created_from: 'dismissed', finding_id: 'history' };
const legacy = { ...manual('Legacy'), created_from: 'suppress' };
const untouched = [neighbour, broad, dismissed, legacy];
const summary = {
  activeCount: 0,
  openCount: 0,
  acknowledgedCount: 0,
  suppressedCount: 0,
  uncertainCount: 0,
  resolvedCount: 0,
  calm: true,
  coverageState: 'current',
  evaluatedAt: now,
};
const digest = {
  generated_at: now,
  window: { start: now, end: now, days: 7, history_complete: true },
  mode: 'monitor',
  runs: {
    total: 0,
    scheduled: 0,
    event_triggered: 0,
    manual: 0,
    failed: 0,
    checks: 0,
    resources_covered: 0,
  },
  findings: {
    new: 0,
    open_by_severity: { critical: 0, warning: 0, watch: 0, info: 0 },
    resolved: 0,
    auto_resolved: 0,
    dismissed: 0,
    suppressed: 0,
  },
  investigations: { total: 0, by_outcome: {} },
  actions: {
    proposed: 0,
    approved: 0,
    rejected: 0,
    executed: 0,
    verified: 0,
    failed: 0,
    pending: 0,
  },
  alerts: { reviewed: 0 },
  spend: { estimated_usd: 0, pricing_known: true, input_tokens: 0, output_tokens: 0, calls: 0 },
};
let browser, server;
async function capture(page, section, name, state, viewport = false) {
  const geometry = await page.evaluate(() => ({
    inner: innerWidth,
    scroll: document.documentElement.scrollWidth,
    alerts: [...document.querySelectorAll('#patrol-suppression-rules [role="alert"]')].map((e) => {
      const box = e.getBoundingClientRect();
      return {
        text: e.textContent,
        width: box.width,
        client: e.clientWidth,
        scroll: e.scrollWidth,
      };
    }),
  }));
  assert(geometry.scroll <= geometry.inner + 1, 'page horizontal overflow');
  for (const field of geometry.alerts) assert(field.scroll <= field.client + 1, 'clipped error');
  const file = path.join(output, `${name}-${state}.png`);
  if (viewport) await page.screenshot({ path: file, animations: 'disabled' });
  else await section.screenshot({ path: file, animations: 'disabled' });
  report.captures.push({
    name,
    state,
    file: path.relative(root, file),
    sha256: hash(fs.readFileSync(file)),
    geometry,
  });
}
(async () => {
  for (const [file, digest] of Object.entries(binding.files))
    assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, file);
  assert.equal(
    report.playwright,
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
    optimizeDeps: { entries: ['browser-tests/patrol-rule-removal.html'] },
    plugins: [
      {
        name: 'patrol-proof-route',
        configureServer(vite) {
          vite.middlewares.use((req, _res, next) => {
            if (req.url?.startsWith('/patrol')) req.url = '/browser-tests/patrol-rule-removal.html';
            next();
          });
        },
      },
    ],
    server: { host: '127.0.0.1', port: 5328, strictPort: true },
  });
  await server.listen();
  for (const [name, engine, width] of [
    ['desktop', chromium, 1365],
    ['narrow', webkit, 320],
  ]) {
    const result = { name, width, checks: [], requests: [], delayed: [], cleanup: {} };
    report.cases.push(result);
    let rules = [
      manual('Preflight'),
      manual('Delete'),
      manual('Readback'),
      manual('Success'),
      ...structuredClone(untouched),
    ];
    let phase = 'load';
    let accepted = false;
    const held = [];
    browser = await engine.launch(
      engine === chromium
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true },
    );
    result.browser = browser.version();
    const context = await browser.newContext({
      viewport: { width, height: 1000 },
      isMobile: width === 320,
      hasTouch: width === 320,
      colorScheme: width === 320 ? 'dark' : 'light',
      locale: 'en-GB',
      timezoneId: 'Europe/London',
    });
    const page = await context.newPage();
    page.setDefaultTimeout(12000);
    page.on('pageerror', (error) => report.errors.push(error.message));
    await context.addCookies([{ name: 'pulse_csrf', value: 'synthetic-csrf', url: origin }]);
    await page.addInitScript(() => {
      if (matchMedia('(prefers-color-scheme: dark)').matches)
        document.documentElement.classList.add('dark');
    });
    await page.clock.install({ time: new Date(now) });
    await page.clock.pauseAt(new Date(now));
    await page.routeWebSocket('**/ws*', () => {});
    await context.route('**/*', async (route) => {
      const request = route.request(),
        url = new URL(request.url()),
        p = url.pathname,
        method = request.method();
      if (url.origin !== origin) {
        report.unexpected.push('off-origin');
        return route.abort();
      }
      if (!p.startsWith('/api/')) return route.continue();
      result.requests.push({ path: p, method, phase });
      if (p.startsWith('/api/ai/patrol/suppressions')) {
        assert.equal(request.headers()['x-pulse-org-id'], 'fixture-tenant-a');
        if (p === '/api/ai/patrol/suppressions' && method === 'GET') {
          if (phase === 'load' || phase === 'preflight' || (phase === 'readback' && accepted)) {
            held.push(route);
            return;
          }
          return route.fulfill({ json: rules });
        }
        assert.equal(method, 'DELETE');
        assert.equal(request.headers()['x-csrf-token'], 'synthetic-csrf');
        const id = decodeURIComponent(p.slice('/api/ai/patrol/suppressions/'.length));
        assert.equal(
          id,
          manual(phase === 'idle' ? 'Success' : phase === 'delete' ? 'Delete' : 'Readback').id,
        );
        assert(rules.some((r) => r.id === id));
        rules = rules.filter((r) => r.id !== id);
        accepted = true;
        if (phase === 'delete') {
          held.push(route);
          return;
        }
        return route.fulfill({ json: { success: true } });
      }
      assert.equal(method, 'GET', `unexpected mutation: ${method} ${p}`);
      let json = {};
      if (p === '/api/settings/ai')
        json = {
          patrol_enabled: true,
          patrol_autonomy_level: 'monitor',
          patrol_readiness: { status: 'ready', ready: true, cause: 'none' },
        };
      else if (p === '/api/ai/patrol/status')
        json = {
          enabled: true,
          runtime_state: 'idle',
          running: false,
          readiness: { status: 'ready', ready: true, cause: 'none' },
          summary: { active: 0, critical: 0, warning: 0, info: 0 },
        };
      else if (p === '/api/ai/patrol/attention')
        json = { data: [], summary, meta: { page: 1, limit: 50, total: 0, totalPages: 0 } };
      else if (p === '/api/ai/patrol/attention/summary') json = summary;
      else if (p === '/api/ai/patrol/autonomy')
        json = { autonomy_level: 'monitor', full_mode_unlocked: false };
      else if (p === '/api/ai/patrol/findings' || p === '/api/ai/patrol/runs') json = [];
      else if (p === '/api/ai/patrol/digest') json = digest;
      else if (p === '/api/ai/unified/findings') json = { findings: [] };
      else if (p === '/api/ai/approvals') json = { approvals: [] };
      else if (p === '/api/ai/models') json = { models: [] };
      else if (p === '/api/license/runtime-capabilities')
        json = {
          capabilities: ['ai_patrol'],
          limits: [],
          hosted_mode: false,
          max_history_days: 7,
          runtime: { build: 'community', label: 'Pulse Community runtime' },
          blocked_capabilities: [],
        };
      return route.fulfill({ json });
    });
    const section = page.locator('#patrol-suppression-rules');
    const activate = async (control) => {
      if (width === 320) await control.tap();
      else {
        await control.focus();
        await page.keyboard.press('Enter');
      }
    };
    const reload = () => section.getByRole('button', { name: 'Reload rules', exact: true });
    const ruleRequests = () =>
      result.requests.filter((r) => r.path.startsWith('/api/ai/patrol/suppressions'));
    const deleted = () => ruleRequests().filter((r) => r.method === 'DELETE');
    const releaseDelayed = async () => {
      for (const route of held.splice(0)) {
        try {
          await route.fulfill({
            json: route.request().method() === 'DELETE' ? { success: true } : rules,
          });
          result.delayed.push('late response offered after expiry');
        } catch {
          result.delayed.push('aborted request no longer accepts response');
        }
      }
    };
    await page.goto(`${origin}/patrol/activity#patrol-suppression-rules`, {
      waitUntil: 'domcontentloaded',
      timeout: 60000,
    });
    await section.getByRole('button', { name: 'Checking rules…' }).waitFor();
    assert(await section.getByRole('button', { name: 'Checking rules…' }).isDisabled());
    await page.clock.fastForward(29_999);
    assert.equal(await section.getByRole('alert').count(), 0);
    await page.clock.fastForward(1);
    await section
      .getByRole('alert')
      .filter({ hasText: 'Rules could not be loaded. The request timed out.' })
      .waitFor();
    assert(await reload().isEnabled());
    assert.equal(await section.getByText('No manually created suppression rules.').count(), 0);
    await capture(page, section, name, 'load-timeout');
    phase = 'idle';
    await releaseDelayed();
    await activate(reload());
    await section
      .getByRole('button', { name: 'Remove rule for Preflight VM, backup', exact: true })
      .waitFor();
    result.checks.push(
      'Stalled list expires at 30 seconds, is not empty, and recovers only through explicit reload.',
    );

    for (const [stage, target] of [
      ['preflight', 'Preflight'],
      ['delete', 'Delete'],
      ['readback', 'Readback'],
    ]) {
      phase = stage;
      accepted = false;
      await activate(
        section.getByRole('button', { name: `Remove rule for ${target} VM, backup`, exact: true }),
      );
      const dialog = page.getByRole('dialog', { name: 'Remove suppression rule?' });
      await dialog.waitFor();
      await dialog.evaluate((element) =>
        element.getAnimations({ subtree: true }).forEach((animation) => animation.finish()),
      );
      assert(
        await dialog
          .getByRole('button', { name: 'Cancel', exact: true })
          .evaluate((element) => element === document.activeElement),
      );
      const beforeDeletes = deleted().length;
      await activate(dialog.getByRole('button', { name: 'Remove this rule', exact: true }));
      await dialog.getByRole('button', { name: 'Verifying removal…' }).waitFor();
      assert(await dialog.getByRole('button', { name: 'Cancel', exact: true }).isDisabled());
      // Wait for the requested stage to reach its deliberately held HTTP response.
      for (let i = 0; held.length === 0 && i < 200; i++)
        await new Promise((r) => setTimeout(r, 10));
      assert.equal(held.length, 1, `missing held ${stage} response`);
      if (stage === 'delete') await capture(page, section, name, 'delete-pending', true);
      await page.clock.fastForward(30_000);
      await dialog.waitFor({ state: 'detached' });
      await section
        .getByRole('alert')
        .filter({ hasText: 'Removal could not be confirmed. The request timed out.' })
        .waitFor();
      assert(await reload().isEnabled());
      assert(
        await section.locator('summary').evaluate((element) => element === document.activeElement),
        'expired confirmation returns focus to the stable summary',
      );
      assert.equal(await section.getByRole('status').count(), 0);
      assert.equal(await section.getByRole('button', { name: /^Remove rule for/ }).count(), 0);
      assert.equal(deleted().length, beforeDeletes + (stage === 'preflight' ? 0 : 1));
      const afterTimeout = ruleRequests().length;
      await releaseDelayed();
      await page.clock.fastForward(60_000);
      assert.equal(
        ruleRequests().length,
        afterTimeout,
        'deadline must not trigger a read or mutation retry',
      );
      assert.equal(
        await section.getByRole('status').count(),
        0,
        'late result must not claim success',
      );
      if (stage === 'readback') await capture(page, section, name, 'readback-timeout');
      phase = 'idle';
      await activate(reload());
      await section.getByText(`Rule ID: ${neighbour.id}`, { exact: true }).waitFor();
      assert.equal(
        await section.getByText(`Rule ID: ${manual(target).id}`, { exact: true }).count(),
        stage === 'preflight' ? 1 : 0,
      );
      assert.equal(await section.getByRole('status').count(), 0);
      result.checks.push(
        `${stage}: bounded uncertainty, no automatic retry/readback or late success; explicit reload reconciles actual fixture state.`,
      );
    }
    await activate(
      section.getByRole('button', { name: 'Remove rule for Success VM, backup', exact: true }),
    );
    await activate(
      page.getByRole('dialog').getByRole('button', { name: 'Remove this rule', exact: true }),
    );
    await section
      .getByRole('status')
      .filter({ hasText: 'Rule removed. Future matching findings can appear.' })
      .waitFor();
    assert.deepEqual(
      rules.filter((r) => untouched.some((u) => u.id === r.id)),
      untouched,
    );
    assert.equal(deleted().length, 3);
    assert.equal(new Set(deleted().map((r) => r.path)).size, 3);
    await capture(page, section, name, 'confirmed-removal');
    result.checks.push(
      width === 320
        ? 'All controls exercised through WebKit touch taps.'
        : 'All controls exercised through keyboard focus and Enter.',
    );
    result.checks.push(
      'A separate explicitly confirmed rule still removes normally after pre-read and absence readback; neighbour, broad, legacy and dismissed records remain byte-equivalent.',
    );
    await browser.close();
    browser = undefined;
    result.cleanup.browser = true;
  }
  assert.deepEqual(report.errors, []);
  assert.deepEqual(report.unexpected, []);
  report.result = 'passed';
})()
  .catch((error) => {
    report.result = 'failed';
    report.failure = error.stack;
    process.exitCode = 1;
  })
  .finally(async () => {
    if (browser) await browser.close();
    report.cleanup.browser = true;
    if (server) await server.close();
    report.cleanup.server = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2));
    console.log(
      JSON.stringify({
        result: report.result,
        cases: report.cases.length,
        captures: report.captures.length,
        cleanup: report.cleanup,
        failure: report.failure,
      }),
    );
  });
