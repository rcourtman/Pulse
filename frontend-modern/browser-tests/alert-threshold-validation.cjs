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
const dir = path.join(root, 'tmp/threshold-validation-proof');
const binding = JSON.parse(fs.readFileSync(path.join(dir, 'binding.json')));
const output = path.join(dir, binding.phase);
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const config = {
  enabled: true,
  activationState: 'active',
  overrides: {},
  guestDefaults: {
    cpu: { trigger: 80, clear: 75 },
    memory: { trigger: 80, clear: 75 },
    disk: { trigger: 90, clear: 85 },
    diskRead: { trigger: 100, clear: 90 },
  },
};
const report = {
  binding,
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  unexpected: [],
  cleanup: {},
};
let server, browser, current;
function api(req, res, next) {
  const url = new URL(req.url, 'http://fixture.invalid');
  if (!url.pathname.startsWith('/api/')) return next();
  current.requests.push({ method: req.method, path: url.pathname });
  const reply = (value) => {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(value));
  };
  if (req.method === 'PUT' && url.pathname === '/api/alerts/config') {
    let body = '';
    req.on('data', (chunk) => (body += chunk));
    req.on('end', () => {
      current.saves.push(JSON.parse(body));
      reply({ success: true });
    });
    return;
  }
  const responses = {
    '/api/health': { status: 'healthy' },
    '/api/alerts/config': config,
    '/api/notifications/email': { enabled: false },
    '/api/notifications/apprise': { enabled: false, mode: 'http', serverUrl: '', apiKey: '' },
    '/api/alerts/deadman/config': { pingUrl: '', configured: false },
    '/api/notifications/webhooks': [],
    '/api/alerts/intent-policies': { schemaVersion: 1, revision: 1 },
  };
  if (req.method === 'GET' && Object.hasOwn(responses, url.pathname))
    return reply(responses[url.pathname]);
  // Ordinary save also preserves the disabled destination configurations.
  if (
    req.method === 'PUT' &&
    [
      '/api/notifications/email',
      '/api/notifications/apprise',
      '/api/alerts/deadman/config',
    ].includes(url.pathname)
  )
    return reply({ success: true });
  report.unexpected.push(`${req.method} ${url.pathname}`);
  res.writeHead(405);
  res.end('{}');
}
async function capture(page, name) {
  assert.equal(
    await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
    false,
    'page overflow',
  );
  const file = `${current.name}-${name}.png`;
  await page.screenshot({ path: path.join(output, file), fullPage: true, animations: 'disabled' });
  report.captures.push({ file, sha256: hash(fs.readFileSync(path.join(output, file))) });
}
async function scan(page, name) {
  const result = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
    .analyze();
  const violations = result.violations.map(({ id, impact, nodes }) => ({
    id,
    impact,
    nodes: nodes.map(({ target, failureSummary }) => ({ target, failureSummary })),
  }));
  current.scans.push({ name, scope: 'whole-page', violations });
  if (name.endsWith('-dark')) {
    // Dark-mode exploration also found existing badge/filter contrast outside
    // the changed slider labels and validation errors. Retain those findings;
    // they are not a passing whole-page audit or a reason to widen this repair.
    const changed = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
      .include('.relative.w-full.h-3\\.5')
      .include('[data-alert-threshold-editor]')
      .include('p[role="alert"]')
      .analyze();
    const changedViolations = changed.violations.map(({ id, impact, nodes }) => ({
      id,
      impact,
      nodes: nodes.map(({ target, failureSummary }) => ({ target, failureSummary })),
    }));
    current.scans.push({
      name,
      scope: 'changed-sliders-inputs-errors',
      violations: changedViolations,
    });
    assert.equal(changedViolations.length, 0, 'changed controls in dark mode');
  } else {
    assert.equal(violations.length, 0, 'whole-page light-mode accessibility');
  }
}
async function open(page) {
  await page.goto('http://127.0.0.1:5281/browser-tests/alert-threshold-validation.html');
  const section = page.getByTestId('section-guests');
  const disclosure = section.locator('button[aria-controls="section-content-guests"]');
  await expect(disclosure).toBeVisible();
  if ((await disclosure.getAttribute('aria-expanded')) !== 'true') await disclosure.click();
  await expect(
    section.getByRole('spinbutton', { name: 'CPU % default', exact: true }),
  ).toBeVisible();
  await page.waitForLoadState('networkidle');
  return section;
}
async function activate(button, touch) {
  if (touch) await button.tap();
  else {
    await button.focus();
    await button.press('Enter');
  }
}
async function dismissToasts(page) {
  const buttons = page
    .getByRole('region', { name: 'Notifications' })
    .getByRole('button', { name: 'Dismiss notification' });
  while (await buttons.count()) {
    const before = await buttons.count();
    try {
      // A toast can be leaving while the count is read. Keyboard activation
      // needs no geometric stability; an already expired last toast is done.
      await buttons.first().press('Enter', { timeout: 1500 });
    } catch (error) {
      if ((await buttons.count()) === 0) return;
      throw error;
    }
    await expect.poll(() => buttons.count()).toBeLessThan(before);
  }
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
      path.join(root, 'frontend-modern/node_modules/vite/dist/node/index.js')
    );
    server = await createServer({
      root: path.join(root, binding.frontendRoot),
      configFile: path.join(root, binding.frontendRoot, 'vite.config.ts'),
      cacheDir: path.join(output, 'vite-cache'),
      optimizeDeps: { noDiscovery: true, include: [] },
      plugins: [
        {
          name: 'synthetic-threshold-configuration',
          configureServer(vite) {
            vite.middlewares.use(api);
          },
        },
      ],
      server: { host: '127.0.0.1', port: 5281, strictPort: true, fs: { allow: [root] } },
    });
    await server.listen();
    for (const spec of [
      { name: 'desktop', engine: chromium, width: 2200, height: 1000, touch: false },
      { name: 'phone', engine: webkit, width: 320, height: 844, touch: true },
    ]) {
      current = { name: spec.name, requests: [], saves: [], scans: [], checks: [] };
      report.cases.push(current);
      browser = await spec.engine.launch(
        spec.name === 'desktop'
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      current.browser = browser.version();
      const context = await browser.newContext({
        viewport: { width: spec.width, height: spec.height },
        hasTouch: spec.touch,
        isMobile: spec.touch,
        colorScheme: 'light',
      });
      const page = await context.newPage();
      page.on('pageerror', (error) => report.unexpected.push(error.message));
      let section = await open(page);
      const cpu = () => section.getByRole('spinbutton', { name: 'CPU % default', exact: true });
      const memory = () =>
        section.getByRole('spinbutton', { name: 'Memory % default', exact: true });
      await memory().fill('85'); // Make the real Save changes action available.
      await cpu().fill('150');
      await activate(page.getByRole('button', { name: 'Save Changes', exact: true }), spec.touch);
      if (binding.phase === 'baseline') {
        await expect.poll(() => current.saves.length).toBe(1);
        assert.equal(current.saves[0].guestDefaults.cpu.trigger, 80);
        await expect(cpu()).toHaveValue('150');
        current.checks.push('Reproduced success with visible CPU 150 but saved CPU 80');
        await capture(page, 'silent-invalid-save');
        section = await open(page);
        await memory().fill('85');
        await cpu().fill('');
        await activate(page.getByRole('button', { name: 'Save Changes', exact: true }), spec.touch);
        await expect.poll(() => current.saves.length).toBe(2);
        assert.equal(current.saves[1].guestDefaults.cpu.trigger, 80);
        await expect(cpu()).toHaveValue('');
        current.checks.push('Reproduced success with blank enabled CPU but saved CPU 80');
        await capture(page, 'silent-blank-save');
      } else {
        await expect(cpu()).toHaveAttribute('aria-invalid', 'true');
        await expect(cpu()).toBeFocused();
        assert.equal(current.saves.length, 0);
        await expect(
          section.getByText('Enter a number from 1 to 100.', { exact: true }),
        ).toBeVisible();
        await capture(page, 'invalid-save-blocked');
        await scan(page, 'invalid-save-blocked');
        await cpu().fill('');
        await activate(page.getByRole('button', { name: 'Save Changes', exact: true }), spec.touch);
        assert.equal(current.saves.length, 0);
        await expect(cpu()).toHaveAttribute('aria-invalid', 'true');
        await cpu().fill('90.5');
        await activate(page.getByRole('button', { name: 'Save Changes', exact: true }), spec.touch);
        await expect.poll(() => current.saves.length).toBe(1);
        assert.equal(current.saves[0].guestDefaults.cpu.trigger, 90.5);
        assert.equal(current.saves[0].guestDefaults.memory.trigger, 85);
        await expect(cpu()).not.toHaveAttribute('aria-invalid', 'true');
        current.checks.push(
          'Invalid and blank defaults send no PUT; corrected decimal limit is saved exactly',
        );
        await capture(page, 'corrected-save');
        await scan(page, 'corrected-save');
        await dismissToasts(page);

        // The actual override editor commits on desktop blur, and on a card's
        // Save action on a phone. Neither may close an invalid visible draft.
        const editRow = async () => {
          await activate(
            section.getByRole('button', {
              name: spec.touch
                ? 'Edit thresholds for validation-guest'
                : /Edit Disk R MB\/s threshold for validation-guest:/,
            }),
            spec.touch,
          );
          const input = section.getByRole('spinbutton', {
            name: 'Disk R MB/s for validation-guest',
            exact: true,
          });
          await expect(input).toBeVisible();
          return input;
        };
        const commitRow = async () => {
          if (spec.touch)
            await section
              .getByRole('button', { name: 'Save threshold edits for validation-guest' })
              .tap();
          else await page.getByRole('heading', { name: 'Alert thresholds', exact: true }).click();
        };
        let rowInput = await editRow();
        await rowInput.fill('10001');
        // Another default rebuilds the real resource rows, as live resampling
        // does. Neither layout may remount away the rejected native draft.
        await memory().fill('86');
        await commitRow();
        await expect(rowInput).toBeVisible();
        if (spec.touch) await expect(rowInput).toBeFocused();
        await expect(rowInput).toHaveValue('10001');
        await expect(rowInput).toHaveAttribute('aria-invalid', 'true');
        const rowError = section.getByText('Enter a number from 0.01 to 10000.', { exact: true });
        await expect(rowError).toBeVisible();
        assert.equal(current.saves.length, 1);
        await activate(page.getByRole('button', { name: 'Save Changes', exact: true }), spec.touch);
        await expect(rowInput).toHaveValue('10001');
        assert.equal(current.saves.length, 1);
        await capture(page, 'row-save-blocked');
        await scan(page, 'row-save-blocked');
        await expect(rowInput).toHaveValue('10001');
        await expect(rowError).toBeVisible();
        await page.evaluate(() => document.documentElement.classList.add('dark'));
        await capture(page, 'row-save-blocked-dark');
        await scan(page, 'row-save-blocked-dark');
        await expect(rowInput).toHaveValue('10001');
        await expect(rowError).toBeVisible();
        await page.evaluate(() => document.documentElement.classList.remove('dark'));

        await rowInput.fill('');
        await rowInput.pressSequentially('1e');
        assert.equal(await rowInput.evaluate((input) => input.validity.badInput), true);
        await commitRow();
        await expect(rowInput).toHaveAttribute('aria-invalid', 'true');
        await rowInput.fill('125.5');
        await commitRow();
        await expect(rowInput).toHaveCount(0);
        await activate(page.getByRole('button', { name: 'Save Changes', exact: true }), spec.touch);
        await expect.poll(() => current.saves.length).toBe(2);
        const overrides = Object.values(current.saves[1].overrides);
        assert.equal(overrides.length, 1);
        assert.equal(overrides[0].diskRead.trigger, 125.5);
        current.checks.push(
          'Invalid rate and incomplete exponent cannot commit a row; corrected 125.5 persists through the real policy consumer',
        );

        rowInput = await editRow();
        await rowInput.fill('');
        await commitRow();
        await expect(rowInput).toHaveCount(0);
        await activate(page.getByRole('button', { name: 'Save Changes', exact: true }), spec.touch);
        await expect.poll(() => current.saves.length).toBe(3);
        for (const override of Object.values(current.saves[2].overrides))
          assert.equal(override.diskRead, undefined);
        current.checks.push('A genuinely empty override inherits and never stores an Off limit');
        await dismissToasts(page);

        await memory().fill('86');
        await cpu().fill('150');
        const fold = section.locator('button[aria-controls="section-content-guests"]');
        await fold.click();
        await activate(page.getByRole('button', { name: 'Save Changes', exact: true }), spec.touch);
        await expect(fold).toHaveAttribute('aria-expanded', 'true');
        await expect(cpu()).toBeFocused();
        assert.equal(current.saves.length, 3);
        await section
          .getByRole('button', { name: 'CPU % default alerts: On', exact: true })
          .click();
        await expect(cpu()).toBeDisabled();
        await expect(cpu()).not.toHaveAttribute('aria-invalid', 'true');
        await activate(page.getByRole('button', { name: 'Save Changes', exact: true }), spec.touch);
        await expect.poll(() => current.saves.length).toBe(4);
        assert.equal(current.saves[3].guestDefaults.cpu.trigger, 0);
        current.checks.push(
          'Save reveals a folded invalid group; explicit Off clears the invalid draft and saves the existing disabled representation',
        );
        await capture(page, 'off-and-inherited');
        await scan(page, 'off-and-inherited');
      }
      await context.close();
      await browser.close();
      browser = null;
    }
    assert.deepEqual(report.unexpected, []);
    report.passed = true;
  } catch (error) {
    report.error = error.stack || String(error);
    process.exitCode = 1;
  } finally {
    if (browser) {
      await browser.close();
      report.cleanup.browserClosed = true;
    } else report.cleanup.browserClosed = true;
    if (server) {
      await server.close();
      report.cleanup.serverClosed = true;
    }
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(JSON.stringify(report));
  }
})();
