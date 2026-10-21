// Actual Notifications controls/state/API/toasts, with an inert, delayed
// same-origin producer. No provider, saved configuration or real queue.
const { chromium, webkit } = require('playwright');
const { expect } = require('playwright/test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const root = path.resolve('frontend-modern');
const compiled = path.resolve('tmp/notification-recovery-proof/static');
const output = path.resolve('tmp/webhook-test-ownership/browser');
fs.mkdirSync(output, { recursive: true });
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
for (const [file, digest] of Object.entries(binding.sources))
  assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `Changed source: ${file}`);
const assets = new Map(
  Object.entries(binding.artifacts).map(([file, digest]) => {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, `Changed artifact: ${file}`);
    return ['/' + file, bytes];
  }),
);
const report = {
  source: binding.source_sha,
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  cleanup: {},
  completed: false,
};
const health = {
  overall_healthy: true,
  queue: {
    pending: 0,
    sending: 0,
    sent: 0,
    failed: 0,
    dlq: 0,
    healthy: true,
    status: 'healthy',
    attention_required: 0,
    reason_codes: [],
    completed_retention_days: 7,
    dead_letter_retention_days: 30,
    counts_are_retention_bounded: true,
    retry_attempts_affect_health: false,
    terminal_failures_affect_health: true,
  },
};
let run, pending;
const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, 'http://127.0.0.1');
  if (url.pathname === '/favicon.ico') {
    res.writeHead(204);
    return res.end();
  }
  if (!url.pathname.startsWith('/api/')) {
    const bytes = assets.get(url.pathname);
    res.setHeader(
      'Content-Type',
      url.pathname.endsWith('.js')
        ? 'text/javascript'
        : url.pathname.endsWith('.css')
          ? 'text/css'
          : 'text/html',
    );
    res.writeHead(bytes ? 200 : 404);
    return res.end(bytes);
  }
  res.setHeader('Content-Type', 'application/json');
  res.setHeader('Set-Cookie', 'pulse_csrf=synthetic-csrf; Path=/; SameSite=Strict');
  if (
    req.method === 'POST' &&
    ['/api/notifications/test', '/api/notifications/webhooks/test'].includes(url.pathname)
  ) {
    let body = '';
    for await (const chunk of req) body += chunk;
    run.sends.push({
      path: url.pathname,
      body: JSON.parse(body),
      csrf: req.headers['x-csrf-token'] === 'synthetic-csrf',
    });
    if (pending) {
      run.unexpected.push('Overlapping send');
      res.writeHead(409);
      return res.end('{}');
    }
    pending = res;
    return;
  }
  const payload = {
    '/api/notifications/health': health,
    '/api/notifications/delivery-log': {
      entries: [],
      window_days: 30,
      completed_retention_days: 7,
      dead_letter_retention_days: 30,
    },
    '/api/alerts/events': [],
    '/api/notifications/email-providers': [],
    '/api/notifications/webhook-templates': [],
    '/api/alerts/deadman/status': {
      configured: false,
      state: 'disabled',
      heartbeatIntervalSeconds: 60,
      recommendedGraceSeconds: 180,
      consecutiveFailures: 0,
    },
    '/api/security/status': {
      hasAuthentication: true,
      requiresAuth: false,
      sessionCapabilities: {},
    },
    '/api/health': { status: 'ok' },
    '/api/license/runtime-capabilities': {
      capabilities: [],
      limits: [],
      hosted_mode: false,
      blocked_capabilities: [],
      runtime: { build: 'community' },
    },
  }[url.pathname];
  if (req.method !== 'GET' || payload === undefined) {
    run.unexpected.push(req.method + ' ' + url.pathname);
    res.writeHead(404);
    return res.end('{}');
  }
  res.writeHead(200);
  res.end(JSON.stringify(payload));
});
const release = (body, status = 200) => {
  assert.ok(pending, 'Expected a held request');
  const response = pending;
  pending = null;
  response.writeHead(status);
  response.end(JSON.stringify(body));
};
let browser, context;
(async () => {
  try {
    assert.equal(
      report.playwright,
      JSON.parse(fs.readFileSync('tests/integration/package-lock.json')).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    await new Promise((resolve, reject) => {
      server.once('error', reject);
      server.listen(0, '127.0.0.1', resolve);
    });
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const spec of [
      { name: 'desktop', engine: chromium, viewport: { width: 1365, height: 900 }, dark: false },
      { name: 'narrow', engine: webkit, viewport: { width: 320, height: 900 }, dark: true },
    ]) {
      run = {
        name: spec.name,
        viewport: spec.viewport,
        sends: [],
        checks: [],
        pageErrors: [],
        consoleErrors: [],
        unexpected: [],
      };
      report.cases.push(run);
      browser = await spec.engine.launch(
        spec.engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      run.browser = browser.version();
      context = await browser.newContext({
        viewport: spec.viewport,
        hasTouch: spec.dark,
        colorScheme: spec.dark ? 'dark' : 'light',
      });
      await context.route('**/*', (route) => {
        if (new URL(route.request().url()).origin === origin) return route.continue();
        run.unexpected.push('External ' + route.request().url());
        return route.abort();
      });
      await context.addInitScript(
        (dark) => localStorage.setItem('theme', dark ? 'dark' : 'light'),
        spec.dark,
      );
      const page = await context.newPage();
      page.on('pageerror', (error) => run.pageErrors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') run.consoleErrors.push(message.text());
      });
      await page.goto(
        origin + '/browser-tests/notification-recovery-boundary.html?send-tests&test-ownership',
      );
      await expect(
        page.getByRole('heading', { name: 'Alert notifications', exact: true }),
      ).toBeVisible();
      if (spec.dark) await page.evaluate(() => document.documentElement.classList.add('dark'));
      const hooks = page
        .locator('[data-settings-panel]')
        .filter({ has: page.getByRole('heading', { name: 'Webhooks', exact: true }) });
      const toasts = page.getByRole('region', { name: 'Notifications', exact: true });
      const clearToasts = async () => {
        const buttons = toasts.getByRole('button', { name: 'Dismiss notification', exact: true });
        while (await buttons.count()) await buttons.first().click();
        await expect(toasts.locator('[role=alert], [role=status]')).toHaveCount(0);
      };
      const operate = async (button) => {
        await expect(button).toBeEnabled();
        if (spec.dark) await button.tap();
        else {
          await button.focus();
          await button.press('Enter');
        }
      };
      const capture = async (name) => {
        assert.equal(
          await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
          false,
        );
        const file = `${spec.name}-${name}.png`;
        await page.screenshot({ path: path.join(output, file), animations: 'disabled' });
        report.captures.push({ file, sha256: hash(fs.readFileSync(path.join(output, file))) });
      };
      const waitForSends = async (count) => {
        await expect.poll(() => run.sends.length).toBe(count);
        assert.ok(pending);
        assert.equal(run.sends.at(-1).csrf, true);
      };

      // Saved A owns the slot: neither B nor a draft opened on B can send.
      await operate(hooks.getByRole('button', { name: 'Test', exact: true }).first());
      await waitForSends(1);
      assert.equal(run.sends[0].body.webhookId, 'ops');
      await expect(hooks.getByRole('button', { name: 'Testing…', exact: true })).toBeDisabled();
      const other = hooks.getByRole('button', { name: 'Test', exact: true });
      await expect(other).toBeDisabled();
      // A stale event can bypass the disabled DOM; the real state owner must
      // still refuse it synchronously rather than relying only on CSS/DOM.
      await other.dispatchEvent('click');
      await hooks.getByRole('button', { name: 'Testing…', exact: true }).dispatchEvent('click');
      await expect(toasts.locator('[role=alert], [role=status]')).toHaveCount(0);
      await hooks.getByRole('button', { name: 'Testing…', exact: true }).scrollIntoViewIfNeeded();
      await capture('saved-pending');
      await operate(hooks.getByRole('button', { name: 'Edit', exact: true }).nth(1));
      const form = hooks.locator('fieldset');
      await form.getByLabel('Name', { exact: true }).fill('Keep this unsaved pager');
      const draft = form.getByRole('button', { name: 'Test', exact: true });
      await expect(draft).toBeDisabled();
      await draft.dispatchEvent('click');
      assert.equal(run.sends.length, 1);
      release({ status: 'success' });
      await expect(toasts.getByRole('status')).toHaveText(/Test webhook sent successfully/);
      await expect(draft).toBeEnabled();
      await expect(form.getByLabel('Name', { exact: true })).toHaveValue('Keep this unsaved pager');
      await clearToasts();
      run.checks.push({
        savedOwnsSlot: true,
        staleAndDuplicateEventsDiscarded: true,
        draftPreserved: true,
      });

      // The same lock applies to a draft. Cancelling it does not cancel a
      // potentially accepted send, reset ownership or enqueue another test.
      await operate(draft);
      await waitForSends(2);
      assert.equal(run.sends[1].path, '/api/notifications/webhooks/test');
      assert.equal(run.sends[1].body.name, 'Keep this unsaved pager');
      await expect(form.getByRole('button', { name: 'Testing...', exact: true })).toBeDisabled();
      await form.getByRole('button', { name: 'Testing...', exact: true }).scrollIntoViewIfNeeded();
      await capture('draft-pending');
      await operate(form.getByRole('button', { name: 'Cancel', exact: true }));
      await expect(hooks.getByRole('button', { name: 'Test', exact: true })).toBeDisabled();
      await expect(hooks.getByRole('button', { name: 'Testing…', exact: true })).toBeDisabled();
      await hooks.getByRole('button', { name: 'Test', exact: true }).dispatchEvent('click');
      release({ error: 'Synthetic held rejection', detail: 'Safe synthetic detail' }, 400);
      await expect(toasts.getByRole('alert')).toContainText('Synthetic held rejection');
      await expect(toasts.getByRole('status')).toHaveCount(0);
      await expect(hooks.getByRole('button', { name: 'Test', exact: true }).first()).toBeEnabled();
      assert.equal(run.sends.length, 2);
      await clearToasts();
      run.checks.push({ cancelledDraftStillOwnsSlot: true, rejectionUnlocksWithoutRetry: true });

      await operate(hooks.getByRole('button', { name: 'Test', exact: true }).nth(1));
      await waitForSends(3);
      assert.equal(run.sends[2].body.webhookId, 'pager');
      release({ status: 'success', deliveryPaused: true });
      await expect(toasts.getByRole('alert')).toContainText('delivery is paused');
      await expect(toasts.getByRole('status')).toHaveCount(0);
      await clearToasts();
      run.checks.push({ nextExplicitTestWorks: true, pausedWarningPreserved: true });

      // Replacement tab must not receive success from its retired owner.
      await operate(hooks.getByRole('button', { name: 'Test', exact: true }).first());
      await waitForSends(4);
      await operate(
        page.getByRole('button', { name: 'Hide synthetic Notifications tab', exact: true }),
      );
      await expect(hooks).toHaveCount(0);
      await operate(
        page.getByRole('button', { name: 'Show synthetic Notifications tab', exact: true }),
      );
      await expect(hooks.getByRole('button', { name: 'Test', exact: true }).first()).toBeEnabled();
      const oldResponse = page.waitForResponse(
        (response) =>
          response.url().endsWith('/api/notifications/test') &&
          response.request().method() === 'POST',
      );
      release({ status: 'success' });
      const response = await oldResponse;
      await response.finished();
      await page.evaluate(
        () => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))),
      );
      await expect(toasts.locator('[role=alert], [role=status]')).toHaveCount(0);
      await expect(hooks.getByText('Fixture pager', { exact: true })).toBeVisible();
      await hooks
        .getByRole('button', { name: 'Test', exact: true })
        .first()
        .scrollIntoViewIfNeeded();
      await capture('replacement-no-stale-toast');
      assert.equal(run.sends.length, 4);
      run.checks.push({
        retiredOwnerCannotToastIntoReplacement: true,
        noSavedOrQueueWrite: true,
        noOverflow: true,
      });
      assert.deepEqual(run.pageErrors, []);
      assert.deepEqual(run.unexpected, []);
      assert.equal(pending, null);
      await context.close();
      context = null;
      run.contextClosed = true;
      await browser.close();
      browser = null;
      run.browserClosed = true;
    }
    report.completed = true;
  } catch (error) {
    report.error = String(error.stack || error);
    process.exitCode = 1;
  } finally {
    if (pending) {
      pending.destroy();
      pending = null;
      report.cleanup.pendingDestroyed = true;
    }
    if (context) {
      await context.close();
      report.cleanup.exceptionalContextClosed = true;
    }
    if (browser) {
      await browser.close();
      report.cleanup.exceptionalBrowserClosed = true;
    }
    await new Promise((resolve) => server.close(resolve));
    report.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(JSON.stringify(report, null, 2));
  }
})();
