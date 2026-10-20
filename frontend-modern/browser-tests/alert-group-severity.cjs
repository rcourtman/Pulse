process.env.RAYON_NUM_THREADS = '2';
process.env.UV_THREADPOOL_SIZE = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit, expect } = require('playwright/test');
const {
  default: AxeBuilder,
} = require('/workspace/tests/integration/node_modules/@axe-core/playwright');
const root = '/workspace';
const output = path.join(root, 'tmp/alert-group-proof/browser');
const binding = JSON.parse(fs.readFileSync(path.join(root, 'tmp/alert-group-proof/binding.json')));
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const report = {
  binding,
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  unexpected: [],
  cleanup: {},
};
let server, browser, current;
const health = {
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
function api(req, res, next) {
  const url = new URL(req.url, 'http://fixture.invalid');
  if (!url.pathname.startsWith('/api/')) return next();
  current.requests.push({ method: req.method, path: url.pathname });
  const value =
    url.pathname === '/api/alerts/delivery-diagnosis'
      ? []
      : url.pathname === '/api/notifications/health'
        ? health
        : undefined;
  if (req.method !== 'GET' || value === undefined) {
    report.unexpected.push(`${req.method} ${url.pathname}`);
    res.writeHead(405);
    res.end('{}');
    return;
  }
  res.writeHead(200, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(value));
}
async function capture(page, name) {
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
  const file = `${current.phase}-${current.name}-${name}.png`;
  await page.screenshot({ path: path.join(output, file), animations: 'disabled' });
  report.captures.push({
    file,
    state: name,
    sha256: hash(fs.readFileSync(path.join(output, file))),
  });
}
async function scan(page, state) {
  const result = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
    .analyze();
  current.scans.push({
    state,
    violations: result.violations.map(({ id, impact, nodes }) => ({
      id,
      impact,
      nodes: nodes.map(({ target, failureSummary }) => ({ target, failureSummary })),
    })),
  });
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
    for (const phase of ['baseline', 'fixed']) {
      const frontend = path.join(
        root,
        phase === 'baseline'
          ? 'tmp/alert-group-proof/baseline-source/frontend-modern'
          : 'frontend-modern',
      );
      const { createServer } = await import(
        path.join(root, 'frontend-modern/node_modules/vite/dist/node/index.js')
      );
      server = await createServer({
        root: frontend,
        configFile: path.join(frontend, 'vite.config.ts'),
        cacheDir: path.join(output, `${phase}-vite-cache`),
        optimizeDeps: { noDiscovery: true, include: [] },
        plugins: [
          {
            name: 'read-only-group-fixture',
            configureServer(vite) {
              vite.middlewares.use(api);
            },
          },
        ],
        server: { host: '127.0.0.1', port: 5265, strictPort: true, fs: { allow: [root] } },
      });
      await server.listen();
      const origin = 'http://127.0.0.1:5265';
      for (const spec of [
        { name: 'desktop', engine: chromium, width: 1440, height: 900, touch: false },
        { name: 'phone', engine: webkit, width: 320, height: 844, touch: true },
      ]) {
        current = {
          phase,
          name: spec.name,
          viewport: { width: spec.width, height: spec.height },
          requests: [],
          scans: [],
          checks: [],
          errors: [],
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
          colorScheme: spec.touch ? 'dark' : 'light',
          reducedMotion: 'reduce',
          timezoneId: 'UTC',
          locale: 'en-GB',
        });
        const page = await context.newPage();
        page.setDefaultTimeout(10000);
        page.on('pageerror', (error) => current.errors.push(error.message));
        await page.route('**/*', (route) => {
          const req = route.request(),
            url = new URL(req.url());
          if (url.origin !== origin || !['GET', 'HEAD'].includes(req.method())) {
            report.unexpected.push(`${req.method()} ${url.origin}${url.pathname}`);
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
        await page.goto(`${origin}/browser-tests/alert-group-severity.html`, {
          waitUntil: 'domcontentloaded',
          timeout: 60000,
        });
        await expect(page.locator('#alert-primary')).toBeVisible();
        await expect(
          page.getByText('Notification delivery status is unavailable', { exact: true }),
        ).toHaveCount(0);
        await expect(page.locator('#alert-critical')).toHaveCount(0);
        await expect(
          page.locator('#alert-primary').getByText('Warning', { exact: true }),
        ).toBeVisible();
        if (phase === 'baseline') {
          const disclosure = page.getByRole('button', { name: '+4 linked signals', exact: true });
          await expect(disclosure).toBeVisible();
          await expect(disclosure).not.toHaveAttribute('aria-expanded');
          current.checks.push(
            'Baseline: the designated warning primary hides its critical supporting card behind a severity-free count, with no expanded-state attribute.',
          );
          await capture(page, 'collapsed');
          await disclosure.click();
          await expect(page.locator('#alert-acknowledged')).toBeVisible();
          await page.locator('#alert-critical').scrollIntoViewIfNeeded();
          await capture(page, 'expanded');
          await scan(page, 'expanded');
          assert.ok(
            current.scans.some((scan) =>
              scan.violations.some((item) => item.id === 'color-contrast'),
            ),
            'baseline acknowledged-card contrast defect reproduced',
          );
        } else {
          let disclosure = page.getByRole('button', {
            name: '+4 linked signals 1 Critical 1 Warning 1 Info',
            exact: true,
          });
          await expect(disclosure).toHaveAttribute('aria-expanded', 'false');
          await expect(disclosure).not.toHaveAttribute('aria-controls');
          const other = page.getByRole('button', { name: '+1 related 1 Info', exact: true });
          await expect(other).toBeVisible();
          const box = await disclosure.boundingBox();
          assert.ok(box.height >= (spec.touch ? 44 : 24), 'disclosure target height');
          if (!spec.touch) await disclosure.focus();
          await capture(page, 'collapsed');
          await scan(page, 'collapsed');
          if (spec.touch) await disclosure.tap();
          else await page.keyboard.press('Enter');
          disclosure = page.getByRole('button', {
            name: 'Hide 1 Critical 1 Warning 1 Info',
            exact: true,
          });
          await expect(disclosure).toHaveAttribute('aria-expanded', 'true');
          const id = await disclosure.getAttribute('aria-controls');
          const region = page.locator(`[id="${id}"]`);
          await expect(region.locator('#alert-critical')).toBeVisible();
          const reason = await page.evaluate(() => window.__alertGroupProof.criticalReason);
          await expect(region.getByText(reason, { exact: true })).toBeVisible();
          await expect(region.locator('#alert-acknowledged')).toBeVisible();
          await other.click();
          const otherId = await page
            .getByRole('button', { name: 'Hide 1 Info', exact: true })
            .getAttribute('aria-controls');
          assert.notEqual(id, otherId, 'independent group targets');
          await expect(
            page.locator(`[id="${otherId}"]`).locator('#alert-other-info'),
          ).toBeVisible();
          await page.getByRole('button', { name: 'Hide 1 Info', exact: true }).click();
          await page.locator('#alert-critical').scrollIntoViewIfNeeded();
          await capture(page, 'expanded');
          await scan(page, 'expanded');
          if (spec.touch) await disclosure.tap();
          else {
            await disclosure.focus();
            await page.keyboard.press('Space');
          }
          await expect(page.locator('#alert-critical')).toHaveCount(0);
          await expect(page.locator(`[id="${id}"]`)).toHaveCount(0);
          current.checks.push(
            'Correct hidden severity counts, excluded acknowledged critical signal, preserved warning primary, unique controlled elements, readable full critical reason and independent keyboard/touch disclosure.',
          );
          await page.evaluate(() => window.__alertGroupProof.recover());
          disclosure = page.getByRole('button', {
            name: '+3 linked signals 1 Warning 1 Info',
            exact: true,
          });
          await expect(disclosure).toBeVisible();
          await page.evaluate(() => window.__alertGroupProof.acknowledgeInfo());
          disclosure = page.getByRole('button', {
            name: '+3 linked signals 1 Warning',
            exact: true,
          });
          await expect(disclosure).toBeVisible();
          await capture(page, 'recovered');
          await scan(page, 'recovered');
          await page.evaluate(() => window.__alertGroupProof.clearRelated());
          await expect(disclosure).toHaveCount(0);
          await expect(
            page.getByRole('button', { name: '+1 related 1 Info', exact: true }),
          ).toBeVisible();
          current.checks.push(
            'Ordinary critical removal and info acknowledgement withdraw badges; removing the last related alert withdraws its disclosure without changing the independent group.',
          );
        }
        assert.deepEqual(current.errors, []);
        if (phase === 'fixed')
          assert.deepEqual(
            current.scans.flatMap((scan) => scan.violations),
            [],
          );
        await context.close();
        await browser.close();
        browser = null;
      }
      await server.close();
      server = null;
    }
    assert.deepEqual(report.unexpected, []);
    report.result = 'passed';
  } catch (error) {
    report.result = 'failed';
    report.error = String(error.stack || error);
    process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    if (server) await server.close();
    report.cleanup = { browser_closed: true, server_closed: true };
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2));
    console.log(JSON.stringify(report, null, 2));
  }
})();
