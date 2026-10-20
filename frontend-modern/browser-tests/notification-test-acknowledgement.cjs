// Actual Notifications controls, API adapters and toast rendering; only the
// same-origin HTTP producer is synthetic. No provider, queue or saved write.
const { chromium, webkit } = require('playwright');
const { expect } = require('playwright/test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const root = path.resolve('frontend-modern');
const compiled = path.resolve('tmp/notification-recovery-proof/static');
const output = path.resolve('tmp/notification-test-proof/browser');
fs.mkdirSync(output, { recursive: true });
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
for (const [file, digest] of Object.entries(binding.sources))
  assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `Changed source: ${file}`);
const assets = new Map(Object.entries(binding.artifacts).map(([file, digest]) => {
  const bytes = fs.readFileSync(path.join(compiled, file));
  assert.equal(hash(bytes), digest, `Changed artifact: ${file}`);
  return ['/' + file, bytes];
}));
const report = {
  source: binding.source_sha, playwright: require('playwright/package.json').version,
  cases: [], captures: [], cleanup: {}, completed: false,
};
const health = {
  overall_healthy: true,
  queue: {
    pending: 0, sending: 0, sent: 0, failed: 0, dlq: 0, healthy: true,
    status: 'healthy', attention_required: 0, reason_codes: [],
    completed_retention_days: 7, dead_letter_retention_days: 30,
    counts_are_retention_bounded: true, retry_attempts_affect_health: false,
    terminal_failures_affect_health: true,
  },
};
let run, nextResponse;
const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, 'http://127.0.0.1');
  if (url.pathname === '/favicon.ico') { res.writeHead(204); return res.end(); }
  if (!url.pathname.startsWith('/api/')) {
    const bytes = assets.get(url.pathname);
    res.setHeader('Content-Type', url.pathname.endsWith('.js') ? 'text/javascript' : url.pathname.endsWith('.css') ? 'text/css' : 'text/html');
    res.writeHead(bytes ? 200 : 404);
    return res.end(bytes);
  }
  res.setHeader('Content-Type', 'application/json');
  res.setHeader('Set-Cookie', 'pulse_csrf=synthetic-csrf; Path=/; SameSite=Strict');
  if (req.method === 'POST' && ['/api/notifications/test', '/api/notifications/webhooks/test'].includes(url.pathname)) {
    let body = ''; for await (const chunk of req) body += chunk;
    run.sends.push({ path: url.pathname, body: JSON.parse(body), csrf: req.headers['x-csrf-token'] === 'synthetic-csrf' });
    if (!nextResponse) { run.unexpected.push('Unscheduled send'); res.writeHead(409); return res.end('{}'); }
    const response = nextResponse; nextResponse = null;
    res.writeHead(response.http ?? 200);
    return res.end(JSON.stringify(response.body));
  }
  const payload = {
    '/api/notifications/health': health,
    '/api/notifications/delivery-log': { entries: [], window_days: 30, completed_retention_days: 7, dead_letter_retention_days: 30 },
    '/api/alerts/events': [],
    '/api/notifications/email-providers': [],
    '/api/notifications/webhook-templates': [],
    '/api/alerts/deadman/status': { configured: false, state: 'disabled', heartbeatIntervalSeconds: 60, recommendedGraceSeconds: 180, consecutiveFailures: 0 },
    '/api/security/status': { hasAuthentication: true, requiresAuth: false, sessionCapabilities: {} },
    '/api/health': { status: 'ok' },
    '/api/license/runtime-capabilities': { capabilities: [], limits: [], hosted_mode: false, blocked_capabilities: [], runtime: { build: 'community' } },
  }[url.pathname];
  if (req.method !== 'GET' || payload === undefined) {
    run.unexpected.push(req.method + ' ' + url.pathname); res.writeHead(404); return res.end('{}');
  }
  res.writeHead(200); res.end(JSON.stringify(payload));
});
let browser, context;
(async () => {
  try {
    assert.equal(report.playwright, JSON.parse(fs.readFileSync('tests/integration/package-lock.json')).packages['node_modules/@playwright/test'].version);
    await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const spec of [
      { name: 'desktop', engine: chromium, viewport: { width: 1365, height: 900 }, dark: false },
      { name: 'narrow', engine: webkit, viewport: { width: 320, height: 900 }, dark: true },
    ]) {
      run = { name: spec.name, viewport: spec.viewport, sends: [], checks: [], pageErrors: [], consoleErrors: [], unexpected: [] };
      report.cases.push(run);
      browser = await spec.engine.launch(spec.engine === chromium ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      run.browser = browser.version();
      context = await browser.newContext({ viewport: spec.viewport, hasTouch: spec.dark, colorScheme: spec.dark ? 'dark' : 'light' });
      await context.route('**/*', route => {
        if (new URL(route.request().url()).origin === origin) return route.continue();
        run.unexpected.push('External ' + route.request().url()); return route.abort();
      });
      await context.addInitScript(dark => {
        localStorage.setItem('theme', dark ? 'dark' : 'light');
      }, spec.dark);
      const page = await context.newPage();
      page.on('pageerror', error => run.pageErrors.push(error.message));
      page.on('console', message => { if (message.type() === 'error') run.consoleErrors.push(message.text()); });
      await page.goto(origin + '/browser-tests/notification-recovery-boundary.html?send-tests');
      await expect(page.getByRole('heading', { name: 'Alert notifications', exact: true })).toBeVisible();
      if (spec.dark) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const panel = title => page.locator('[data-settings-panel]').filter({ has: page.getByRole('heading', { name: title, exact: true }) });
      const email = panel('Email notifications').getByRole('button', { name: 'Send test email', exact: true });
      const apprise = panel('Apprise notifications').getByRole('button', { name: 'Send test', exact: true });
      const hooks = panel('Webhooks');
      const saved = hooks.getByRole('button', { name: 'Test', exact: true }).first();
      const toasts = page.getByRole('region', { name: 'Notifications', exact: true });
      const capture = async name => {
        const file = `${spec.name}-${name}.png`;
        await page.screenshot({ path: path.join(output, file), animations: 'disabled' });
        report.captures.push({ file, sha256: hash(fs.readFileSync(path.join(output, file))) });
      };
      const operate = async button => {
        await expect(button).toBeEnabled();
        if (spec.dark) await button.tap(); else { await button.focus(); await button.press('Enter'); }
      };
      const send = async (kind, button, response, expected, screenshot) => {
        const dismiss = toasts.getByRole('button', { name: 'Dismiss notification', exact: true });
        for (let count = await dismiss.count(); count > 0; count--) await dismiss.first().click();
        await expect(toasts.locator('[role=alert], [role=status]')).toHaveCount(0);
        const before = run.sends.length;
        nextResponse = response;
        await operate(button);
        const role = expected === 'confirmed' ? 'status' : 'alert';
        const toast = toasts.getByRole(role);
        await expect(toast).toHaveCount(1);
        if (expected === 'unconfirmed') await expect(toast).toContainText('Test result could not be confirmed. Check the destination before sending another test.');
        else if (expected === 'paused') await expect(toast).toContainText('delivery is paused');
        else if (expected === 'transport') await expect(toast).toContainText('Synthetic provider rejection');
        else await expect(toast).toContainText(kind === 'email' ? 'Test email sent.' : kind === 'apprise' ? 'Test Apprise notification sent.' : 'Test webhook sent successfully!');
        await expect(toasts.getByRole(expected === 'confirmed' ? 'alert' : 'status')).toHaveCount(0);
        await expect(toast).not.toContainText('UNCONFIRMED_DIAGNOSTIC');
        await expect(button).toBeEnabled();
        assert.equal(run.sends.length, before + 1, 'Exactly one test per explicit operation');
        assert.equal(nextResponse, null);
        const request = run.sends.at(-1);
        assert.equal(request.csrf, true);
        assert.equal(request.path, kind === 'draft' ? '/api/notifications/webhooks/test' : '/api/notifications/test');
        if (kind !== 'draft') assert.equal(request.body.method, kind === 'saved' ? 'webhook' : kind);
        if (kind === 'saved') assert.equal(request.body.webhookId, 'ops');
        if (kind === 'draft') { assert.equal(request.body.id, 'ops'); assert.equal(request.body.name, 'Unsaved synthetic edit'); }
        assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
        run.checks.push({ kind, expected, sent: run.sends.length, noOverflow: true });
        if (screenshot) await capture(screenshot);
      };
      for (const [kind, button, bad] of [
        ['email', email, { status: 'success', success: false, message: 'UNCONFIRMED_DIAGNOSTIC' }],
        ['apprise', apprise, {}],
        ['saved', saved, { status: 'failed', message: 'UNCONFIRMED_DIAGNOSTIC' }],
      ]) {
        await send(kind, button, { body: bad }, 'unconfirmed', kind === 'email' ? 'email-unconfirmed' : null);
        await send(kind, button, { body: { status: 'success', deliveryPaused: true } }, 'paused', kind === 'apprise' ? 'apprise-paused' : null);
        await send(kind, button, { body: { status: 'success', deliveryPaused: false } }, 'confirmed', kind === 'saved' ? 'saved-confirmed' : null);
      }
      await send('email', email, { http: 400, body: { error: 'Synthetic provider rejection', detail: 'Safe synthetic transport detail' } }, 'transport');
      await operate(hooks.getByRole('button', { name: 'Edit', exact: true }));
      const form = hooks.locator('fieldset');
      await form.getByLabel('Name', { exact: true }).fill('Unsaved synthetic edit');
      const draft = form.getByRole('button', { name: 'Test', exact: true });
      await send('draft', draft, { body: { success: true, status: 503, message: 'UNCONFIRMED_DIAGNOSTIC' } }, 'unconfirmed', 'draft-unconfirmed');
      await send('draft', draft, { body: { success: true, status: 202, deliveryPaused: true } }, 'paused');
      await send('draft', draft, { body: { success: true, status: 204, deliveryPaused: false } }, 'confirmed');
      await expect(form.getByLabel('Name', { exact: true })).toHaveValue('Unsaved synthetic edit');
      await expect(hooks.getByText('Fixture operations', { exact: true })).toBeVisible();
      run.checks.push({ draftAndSavedPreserved: true, noConfigurationOrQueueWrite: true });
      assert.deepEqual(run.pageErrors, []); assert.deepEqual(run.unexpected, []);
      assert.equal(run.sends.length, 13);
      await context.close(); context = null; run.contextClosed = true;
      await browser.close(); browser = null; run.browserClosed = true;
    }
    report.completed = true;
  } catch (error) { report.error = String(error.stack || error); process.exitCode = 1; }
  finally {
    if (context) { await context.close(); report.cleanup.exceptionalContextClosed = true; }
    if (browser) { await browser.close(); report.cleanup.exceptionalBrowserClosed = true; }
    server.closeAllConnections();
    await new Promise(resolve => server.close(resolve));
    report.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(JSON.stringify(report));
  }
})();
