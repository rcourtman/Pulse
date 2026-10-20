process.env.RAYON_NUM_THREADS = '2';
process.env.UV_THREADPOOL_SIZE = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const binding = JSON.parse(
  fs.readFileSync(path.join(root, 'tmp/delivery-status-proof/binding.json')),
);
const frontend = path.join(
  root,
  binding.phase === 'before'
    ? 'tmp/delivery-status-proof/baseline-source/frontend-modern'
    : 'frontend-modern',
);
const output = path.join(root, 'tmp/delivery-status-proof', binding.output || binding.phase);
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const { default: AxeBuilder } = require(
  path.join(root, 'tests/integration/node_modules/@axe-core/playwright'),
);
const report = {
  binding,
  node: process.version,
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  unexpected: [],
  cleanup: {},
};
let browser,
  server,
  current,
  mode = 'current';
const diagnoses = () =>
  ['vm-a', 'vm-b'].map((id, index) => ({
    alertIdentifier: id,
    alertId: id,
    trackingKey: id,
    status: index === 0 ? 'deferred' : 'suppressed',
    reason: index === 0 ? 'quiet_hours' : 'cooldown',
    message: 'Synthetic notification policy observation',
    lastNotified: '2026-10-20T12:00:00Z',
    nextEligibleAt: '2026-10-20T12:10:00Z',
    quietHoursReplayAt: '2026-10-20T12:30:00Z',
  }));
function api(req, res, next) {
  const url = new URL(req.url, 'http://fixture.invalid');
  if (!url.pathname.startsWith('/api/')) return next();
  current.requests.push({ method: req.method, path: url.pathname, mode });
  if (req.method !== 'GET') {
    report.unexpected.push(`${req.method} ${url.pathname}`);
    res.writeHead(405);
    res.end('{}');
    return;
  }
  let value;
  if (url.pathname === '/api/alerts/delivery-diagnosis') {
    if (mode === 'failure') {
      res.writeHead(503, { 'Content-Type': 'application/json' });
      res.end('{"error":"Synthetic diagnosis read unavailable"}');
      return;
    }
    value =
      mode === 'recovered'
        ? diagnoses().map((item) => ({
            ...item,
            status: 'suppressed',
            reason: 'monitor_only',
            message: 'Synthetic current monitor-only policy',
          }))
        : diagnoses();
  } else if (url.pathname === '/api/notifications/health')
    value = {
      overall_healthy: true,
      queue: {
        status: 'healthy',
        healthy: true,
        pending: 0,
        sending: 0,
        sent: 0,
        failed: 0,
        dlq: 0,
        attention_required: 0,
        completed_retention_days: 7,
        dead_letter_retention_days: 30,
        counts_are_retention_bounded: true,
        retry_attempts_affect_health: false,
        terminal_failures_affect_health: true,
        reason_codes: [],
        failure_class_window_days: 7,
        failure_classes_available: true,
        failure_classes_7d: {
          authentication: 0,
          rate_limited: 0,
          connectivity: 0,
          tls: 0,
          configuration: 0,
          rejected: 0,
          server_error: 0,
          unknown: 0,
        },
      },
    };
  else {
    report.unexpected.push(`${req.method} ${url.pathname}`);
    res.writeHead(404);
    res.end('{}');
    return;
  }
  res.writeHead(200, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(value));
}
(async () => {
  fs.mkdirSync(output, { recursive: true });
  try {
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
      configFile: path.join(frontend, 'vite.config.ts'),
      cacheDir: path.join(output, 'vite-cache'),
      optimizeDeps: { noDiscovery: true, include: [] },
      plugins: [
        {
          name: 'read-only-delivery-fixture',
          configureServer(vite) {
            vite.middlewares.use(api);
          },
        },
      ],
      server: { host: '127.0.0.1', port: 5263, strictPort: true, fs: { allow: [root] } },
    });
    await server.listen();
    const origin = 'http://127.0.0.1:5263';
    for (const spec of [
      { name: 'chromium-desktop', engine: chromium, width: 1440, height: 900, touch: false },
      { name: 'webkit-phone', engine: webkit, width: 390, height: 844, touch: true },
    ]) {
      current = {
        name: spec.name,
        viewport: { width: spec.width, height: spec.height },
        requests: [],
        checks: [],
        scans: [],
        errors: [],
        expected503: [],
      };
      report.cases.push(current);
      browser = await spec.engine.launch(
        spec.touch
          ? { headless: true }
          : { headless: true, channel: 'chromium', args: ['--no-sandbox'] },
      );
      current.browser_version = browser.version();
      const context = await browser.newContext({
        viewport: current.viewport,
        hasTouch: spec.touch,
        isMobile: spec.touch,
        reducedMotion: 'reduce',
        colorScheme: spec.touch ? 'dark' : 'light',
        timezoneId: 'UTC',
        locale: 'en-GB',
      });
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      page.on('pageerror', (error) => current.errors.push(error.message));
      page.on('response', (response) => {
        if (response.status() === 503) current.expected503.push(new URL(response.url()).pathname);
      });
      await page.route('**/*', (route) => {
        const request = route.request(),
          url = new URL(request.url());
        if (url.origin !== origin || !['GET', 'HEAD'].includes(request.method())) {
          report.unexpected.push(`${request.method()} ${url.origin}${url.pathname}`);
          return route.abort();
        }
        return route.continue();
      });
      if (spec.touch)
        await page.addInitScript(() =>
          document.addEventListener('DOMContentLoaded', () =>
            document.documentElement.classList.add('dark'),
          ),
        );
      await page.clock.install({ time: new Date('2026-10-20T12:00:00Z') });
      const capture = async (state) => {
        assert.equal(
          await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
          false,
          'no horizontal page overflow',
        );
        const file = `${spec.name}-${state}.png`;
        await page.screenshot({ path: path.join(output, file), animations: 'disabled' });
        report.captures.push({
          file,
          state,
          sha256: hash(fs.readFileSync(path.join(output, file))),
        });
      };
      const scan = async (state) => {
        const result = await new AxeBuilder({ page })
          .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
          .analyze();
        current.scans.push({
          state,
          violations: result.violations.map((v) => ({
            id: v.id,
            impact: v.impact,
            nodes: v.nodes.map((n) => ({ target: n.target, summary: n.failureSummary })),
          })),
        });
      };
      mode = 'current';
      await page.goto(`${origin}/browser-tests/alert-delivery-freshness.html`, {
        waitUntil: 'domcontentloaded',
        timeout: 60000,
      });
      const quiet = page.getByText('Quiet hours — notifies 12:30', { exact: true });
      const dispatch = page.getByText('Dispatch requested 12:00 — next eligible 12:10', {
        exact: true,
      });
      await expect(quiet).toBeVisible();
      await expect(dispatch).toBeVisible();
      await expect(
        page.getByText('Notification delivery status is unavailable', { exact: true }),
      ).toHaveCount(0);
      const more = page.locator('#alert-vm-a').getByRole('button', { name: 'More', exact: true });
      await more.focus();
      await capture('current');
      const readCount = () =>
        current.requests.filter((item) => item.path === '/api/alerts/delivery-diagnosis').length;
      let previous = readCount();
      mode = 'failure';
      await page.clock.fastForward(60000);
      await expect.poll(readCount).toBeGreaterThan(previous);
      const notice = page.getByText('Alert notification status is unavailable.', { exact: true });
      if (binding.phase === 'before') {
        await expect(quiet).toBeVisible();
        await expect(dispatch).toBeVisible();
        await expect(notice).toHaveCount(0);
        current.checks.push(
          'Reproduced: failed periodic read leaves both old quiet-hours and dispatch reasons unqualified.',
        );
        await capture('failed-refresh-stale');
      } else {
        await expect(notice).toBeVisible();
        await expect(quiet).toHaveCount(0);
        await expect(dispatch).toHaveCount(0);
        await expect(notice).toHaveCount(1);
        await expect(more).toBeFocused();
        await expect(page.locator('#alert-vm-a')).toBeVisible();
        await expect(page.locator('#alert-vm-b')).toBeVisible();
        current.checks.push(
          'Latest failed periodic GET withdraws old reasons, explains unavailable status once, and keeps both alerts and keyboard focus.',
        );
        await scan('unavailable');
        await capture('failed-refresh-unknown');
        previous = readCount();
        await page.clock.fastForward(60000);
        await expect.poll(readCount).toBeGreaterThan(previous);
        await expect(notice).toHaveCount(1);
        await expect(dispatch).toHaveCount(0);
        mode = 'recovered';
        previous = readCount();
        await page.clock.fastForward(60000);
        await expect.poll(readCount).toBeGreaterThan(previous);
        await expect(
          page.getByText('Monitor-only — no notifications', { exact: true }),
        ).toHaveCount(2);
        await expect(notice).toHaveCount(0);
        await expect(more).toBeFocused();
        if (spec.touch) await more.tap();
        else await more.press('Enter');
        await expect(
          page.locator('#alert-vm-a').getByRole('button', { name: 'Timeline', exact: true }),
        ).toBeVisible();
        current.checks.push(
          'Repeated failure stays unknown; ordinary next successful minute replaces the warning and both reasons, with keyboard/touch disclosure intact.',
        );
        await scan('recovered');
        await capture('recovered');
        // No retained diagnosis at first load: same explicit unknown, then
        // an empty active set clears it without an unnecessary API request.
        mode = 'failure';
        await page.reload({ waitUntil: 'domcontentloaded' });
        await expect(notice).toBeVisible();
        await expect(
          page.getByText('Monitor-only — no notifications', { exact: true }),
        ).toHaveCount(0);
        await scan('initial-unavailable');
        for (const [locale, title, description] of [
          [
            'de',
            'Der Benachrichtigungsstatus der Warnmeldungen ist nicht verfuegbar.',
            'Pulse konnte den Zustellungs- oder Pausengrund fuer diese Warnmeldungen nicht aktualisieren. Die Pruefung wird automatisch wiederholt.',
          ],
          [
            'es',
            'El estado de las notificaciones de alerta no está disponible.',
            'Pulse no pudo actualizar el motivo del envío o de la pausa de estas alertas. Volverá a comprobarlo automáticamente.',
          ],
        ]) {
          await page.evaluate((value) => window.__deliveryFreshness.setActiveLocale(value), locale);
          await expect(page.getByText(title, { exact: true })).toBeVisible();
          await expect(page.getByText(description, { exact: true })).toBeVisible();
          await scan(`unavailable-${locale}`);
          await capture(`unavailable-${locale}`);
        }
        await page.evaluate(() => window.__deliveryFreshness.setActiveLocale('en'));
        current.checks.push(
          'German and Spanish unavailable copy renders through the production locale signal without English fallback or horizontal overflow.',
        );
        previous = readCount();
        await page.evaluate(() => window.__deliveryFreshness.setAlerts({}));
        await expect(notice).toHaveCount(0);
        await expect(page.getByText('No active alerts', { exact: true })).toBeVisible();
        assert.equal(readCount(), previous, 'empty set does not request a diagnosis');
        current.checks.push(
          'Initial read failure is explicit; resolving the active set clears unavailable state without another read.',
        );
        await page.evaluate(() =>
          window.__deliveryFreshness.setAlerts(window.__deliveryFreshness.initial),
        );
        await expect(notice).toBeVisible();
        mode = 'recovered';
        previous = readCount();
        await page.clock.fastForward(60000);
        await expect.poll(readCount).toBeGreaterThan(previous);
        await expect(notice).toHaveCount(0);
        await expect(
          page.getByText('Monitor-only — no notifications', { exact: true }),
        ).toHaveCount(2);
      }
      assert.deepEqual(current.errors, []);
      assert(current.expected503.every((item) => item === '/api/alerts/delivery-diagnosis'));
      for (const result of current.scans) assert.deepEqual(result.violations, []);
      await browser.close();
      browser = undefined;
    }
    assert.deepEqual(report.unexpected, []);
  } catch (error) {
    report.error = String(error.stack || error);
    process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    report.cleanup.browser_closed = true;
    if (server) await server.close();
    report.cleanup.server_closed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(
      JSON.stringify(
        {
          ...report,
          binding: {
            source: binding.source,
            phase: binding.phase,
            file_count: Object.keys(binding.files).length,
          },
        },
        null,
        2,
      ),
    );
  }
})();
