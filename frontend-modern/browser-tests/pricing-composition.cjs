// Final-content composition: the real billing consumer must preserve both the
// grouped Pro plan card and truthful failed-return/recovery behaviour. No real
// checkout, activation, payment, account or external service is exercised.
const { chromium, webkit, expect } = require('playwright/test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const root = path.resolve('frontend-modern');
const compiled = path.resolve('tmp/pricing-composition/static');
const output = path.resolve('tmp/pricing-composition/browser');
const planRoute = '/settings/pulse-intelligence/billing/plan';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const report = {
  result: 'incomplete',
  source: null,
  playwright: require('playwright/package.json').version,
  profiles: [],
  captures: [],
  cleanup: {
    browsersCreated: 0,
    browsersClosed: 0,
    contextsCreated: 0,
    contextsClosed: 0,
    serverClosed: false,
  },
  limits:
    'Synthetic GET-only policy/entitlement/operator responses, real router/store/hook/Plans & Billing panel and CSS. Purchase-start is an inert loopback document, not the backend or a transaction. No installed, account, native recovery, payment, release or publication proof.',
};
let server, browser, context;

(async () => {
  fs.mkdirSync(output, { recursive: true });
  try {
    assert.equal(
      report.playwright,
      JSON.parse(fs.readFileSync('tests/integration/package-lock.json')).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
    report.source = binding.source_sha;
    report.binding_sha256 = hash(fs.readFileSync(path.join(compiled, 'binding.json')));
    for (const [file, digest] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, `Changed source: ${file}`);
    for (const file of [
      'src/utils/licensePresentation.ts',
      'src/utils/selfHostedPlans.ts',
      'src/components/Settings/ProLicensePlanSection.tsx',
      'src/components/Settings/ProLicensePanel.tsx',
      'src/components/Settings/useProLicensePanelState.ts',
      'src/components/Settings/SelfHostedCommercialRecoverySection.tsx',
      'src/stores/licenseEntitlements.ts',
      'src/index.css',
    ])
      assert(binding.sources[file], `Missing source: ${file}`);
    const assets = new Map(
      Object.entries(binding.artifacts).map(([file, digest]) => {
        assert(!path.isAbsolute(file) && !file.split('/').includes('..'));
        const bytes = fs.readFileSync(path.join(compiled, file));
        assert.equal(hash(bytes), digest, file);
        return ['/' + file, bytes];
      }),
    );
    server = http.createServer((req, res) => {
      const file = new URL(req.url, 'http://127.0.0.1').pathname;
      const bytes = assets.get(
        file === planRoute ? '/browser-tests/pricing-composition.html' : file,
      );
      res.setHeader('Cache-Control', 'no-store');
      res.setHeader(
        'Content-Type',
        file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : 'text/html',
      );
      res.writeHead(bytes ? 200 : 404);
      res.end(bytes);
    });
    await new Promise((resolve, reject) => {
      server.once('error', reject);
      server.listen(0, '127.0.0.1', resolve);
    });
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const [name, engine, width, dark] of [
      ['desktop-light', chromium, 1440, false],
      ['desktop-dark', chromium, 1440, true],
      ['narrow-light', webkit, 320, false],
      ['narrow-dark', webkit, 320, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      report.cleanup.browsersCreated++;
      context = await browser.newContext({
        viewport: { width, height: 900 },
        hasTouch: width < 500,
        isMobile: width < 500,
        locale: 'en-GB',
        timezoneId: 'UTC',
        reducedMotion: 'reduce',
      });
      report.cleanup.contextsCreated++;
      const record = {
        name,
        width,
        height: 900,
        dark,
        browser: browser.version(),
        checks: [],
        requests: [],
        unexpected: [],
        errors: [],
        failed: [],
      };
      report.profiles.push(record);
      const check = (value) => record.checks.push(value);
      let tier = 'free';
      const page = await context.newPage();
      page.setDefaultTimeout(8000);
      page.on('pageerror', (err) => record.errors.push(err.message));
      page.on('console', (msg) => {
        if (msg.type() === 'error') record.errors.push(msg.text());
      });
      page.on('requestfailed', (req) =>
        record.failed.push({ path: new URL(req.url()).pathname, error: req.failure() }),
      );
      await page.addInitScript(
        (dark) =>
          document.addEventListener(
            'DOMContentLoaded',
            () => document.documentElement.classList.toggle('dark', dark),
            { once: true },
          ),
        dark,
      );
      await page.routeWebSocket('**/*', (socket) => socket.close());
      await page.route('**/*', (route) => {
        const req = route.request(),
          url = new URL(req.url());
        if (url.origin !== origin || req.method() !== 'GET') {
          record.unexpected.push({ path: url.pathname, method: req.method() });
          return route.abort();
        }
        if (url.pathname === '/auth/license-purchase-start') {
          record.requests.push({ path: url.pathname, search: url.search, method: req.method() });
          return route.fulfill({
            contentType: 'text/html',
            body: '<!doctype html><title>Inert purchase-start fixture</title><p>No transaction</p>',
          });
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        record.requests.push({ path: url.pathname, method: req.method(), tier });
        const paid = tier === 'pro';
        const data = {
          '/api/security/status': {
            hasAuthentication: true,
            requiresAuth: false,
            sessionCapabilities: {},
            presentationPolicy: { demoMode: false, readOnly: false, hideCommercial: false },
          },
          '/api/license/entitlements': {
            capabilities: paid ? ['ai_autofix', 'rbac', 'audit_logging', 'reporting'] : [],
            limits: [],
            subscription_state: paid ? 'active' : 'free',
            upgrade_reasons: [],
            tier,
            valid: paid,
            trial_eligible: false,
            runtime: { build: paid ? 'pro' : 'community' },
          },
          '/api/agent/patrol-control/status': {
            nextAction: 'run_patrol',
            progressLabel: 'No Patrol work activity yet.',
            steps: [],
            externalAgentReady: false,
          },
        }[url.pathname];
        if (!data) {
          record.unexpected.push({ path: url.pathname, method: req.method() });
          return route.fulfill({ status: 404, json: { error: 'Unexpected fixture read' } });
        }
        return route.fulfill({ json: data });
      });
      const capture = async (label) => {
        await page.evaluate(() => document.fonts.ready);
        assert(
          await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
          `${name}/${label}: horizontal overflow`,
        );
        const file = `${name}-${label}.png`;
        await page.locator('main').screenshot({ path: path.join(output, file) });
        report.captures.push({
          file,
          sha256: hash(fs.readFileSync(path.join(output, file))),
          name,
          label,
        });
      };

      await page.goto(origin + planRoute, { waitUntil: 'networkidle' });
      const card = page.getByRole('region', { name: 'Pulse Pro plan', exact: true });
      await expect(page.getByText('Current plan: Community', { exact: true })).toBeVisible();
      await expect(card).toBeVisible();
      for (const text of [
        'For small teams and larger estates',
        '$79/year',
        'or $8.99/month',
        'For your team',
        'For larger estates',
        'Patrol and history',
      ])
        await expect(card.getByText(text, { exact: true })).toBeVisible();
      assert.equal(await card.locator('ul').count(), 3);
      await expect(page.getByText('Purchase activation not confirmed')).toHaveCount(0);
      const viewPlans = card.getByRole('link', { name: 'View plans', exact: true });
      await expect(viewPlans).toHaveAttribute(
        'href',
        '/auth/license-purchase-start?feature=self_hosted_plan',
      );
      await capture('available-plan');
      check('Real Community entitlement reader renders grouped Pro price/tagline/actions without failure text or a Relay offer');
      await expect(card.getByText('Relay', { exact: true })).toHaveCount(0);
      await viewPlans.focus();
      const [purchasePage] = await Promise.all([
        page.waitForEvent('popup'),
        page.keyboard.press('Enter'),
      ]);
      await expect(purchasePage).toHaveURL(origin + '/auth/license-purchase-start?feature=self_hosted_plan');
      await expect(purchasePage.getByText('No transaction')).toBeVisible();
      await purchasePage.close();
      check('Keyboard View plans follows the exact same-origin GET into the inert purchase-start fixture, with no checkout or POST');

      for (const state of ['free', 'pro']) {
        tier = state;
        await page.goto(origin + planRoute + '?purchase=failed&intent=self_hosted_plan', {
          waitUntil: 'networkidle',
        });
        await expect(page.getByText('Purchase activation not confirmed', { exact: true })).toBeVisible();
        await expect(page.getByText(/check Pulse Account before starting another purchase/)).toBeVisible();
        await expect(page.getByText(/Checkout completed/i)).toHaveCount(0);
        await expect(page.getByText(/plan.*unchanged/i)).toHaveCount(0);
        await expect(page.getByText(`Current plan: ${state === 'free' ? 'Community' : 'Pulse Pro'}`, { exact: true })).toBeVisible();
        await expect(card).toHaveCount(0);
        await expect(page.getByRole('link', { name: 'View plans', exact: true })).toHaveCount(0);
        await expect(page).toHaveURL(origin + planRoute + '?intent=self_hosted_plan&details=recovery');
        const recovery = page.locator('#pulse-pro-recovery details');
        await expect(recovery).toHaveAttribute('open', '');
        const key = page.getByRole('textbox', { name: 'License key', exact: true });
        await expect(key).toBeVisible();
        const activate = page.getByRole('button', { name: 'Apply key', exact: true });
        await expect(activate).toBeDisabled();
        await capture(`${state}-failed`);
        check(`${state}: real failed-return hook consumes query, opens recovery, keeps current entitlement truth and suppresses purchase offers and false completion`);

        const open = page.getByRole('link', { name: 'Open recovery', exact: true });
        if (width < 500) await open.tap();
        else {
          await open.focus();
          await page.keyboard.press('Enter');
        }
        await expect(recovery).toHaveAttribute('open', '');
        await key.fill('synthetic-not-an-activation-key');
        await expect(activate).toBeEnabled();
        await key.fill('');
        await expect(activate).toBeDisabled();
        const reads = () => record.requests.filter((req) => req.path === '/api/license/entitlements').length;
        const before = reads();
        await page.getByRole('button', { name: 'Refresh', exact: true }).click();
        await expect.poll(reads).toBe(before + 1);
        await expect(page.getByText('Purchase activation not confirmed', { exact: true })).toBeVisible();
        await expect(card).toHaveCount(0);
        check(`${state}: keyboard/touch recovery and editable empty-key controls work; Refresh reads entitlements without losing uncertainty, activation or licence clearing`);
      }

      tier = 'free';
      await page.goto(origin + planRoute + '?purchase=activated&intent=self_hosted_plan', {
        waitUntil: 'networkidle',
      });
      await expect(page.getByText('Current plan: Community', { exact: true })).toBeVisible();
      await expect(page.getByText('Pulse Pro is now active', { exact: true })).toHaveCount(0);
      await expect(page.getByText(/Checkout completed/i)).toHaveCount(0);
      check('An activated URL alone cannot confer paid entitlements or claim completed checkout/Pro activation');
      assert.deepEqual(record.unexpected, []);
      assert.deepEqual(record.errors, []);
      assert.deepEqual(record.failed, []);
      check('All observed application requests are allowlisted loopback GETs; no page, console, network or overflow errors');
      await context.close();
      context = null;
      report.cleanup.contextsClosed++;
      await browser.close();
      browser = null;
      report.cleanup.browsersClosed++;
    }
    report.result = 'passed';
  } catch (error) {
    report.error = { message: error.message, stack: error.stack };
    process.exitCode = 1;
  } finally {
    if (context) {
      await context.close();
      report.cleanup.contextsClosed++;
    }
    if (browser) {
      await browser.close();
      report.cleanup.browsersClosed++;
    }
    if (server) {
      await new Promise((resolve) => server.close(resolve));
      report.cleanup.serverClosed = true;
    }
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(JSON.stringify(report, null, 2));
  }
})();
