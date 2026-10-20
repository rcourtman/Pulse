// Run from the assigned root with pulse-worker-browser. Prepare the exact
// retained source-n3h389mz producer-history.json and source binding under
// tmp/fleet-details-proof first; no backend/store/probe experiment is replayed.
process.env.RAYON_NUM_THREADS = '2';
process.env.UV_THREADPOOL_SIZE = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const base = path.join(root, 'tmp/fleet-details-proof');
let output;
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(base, 'binding.json')));
const historyBytes = fs.readFileSync(path.join(base, 'producer-history.json'));
assert.equal(
  hash(historyBytes),
  '23c1150f2edf74eaa6d0a72cf5dcbd1fe10a5bb195e088706561e2fb9009eedd',
);
const history = JSON.parse(historyBytes);
for (const [file, digest] of Object.entries(binding.files)) {
  assert.equal(hash(fs.readFileSync(path.join(root, file))), digest, file);
}
const report = {
  binding,
  producer: { receipt: 'source-n3h389mz', bytes: historyBytes.length, sha256: hash(historyBytes) },
  playwright: require('playwright/package.json').version,
  profiles: [],
  captures: [],
  failures: [],
  cleanup: {},
};
let browser, server, current;
const save = (phase) => {
  report.phase = phase;
  fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
};
const record = (entry) => {
  current.cases.push(entry);
  save('case-complete');
};

async function capture(page, name) {
  assert.equal(
    await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
    false,
    'horizontal overflow',
  );
  const file = `${current.name}-${name}.png`;
  await page.screenshot({ path: path.join(output, file), animations: 'disabled' });
  report.captures.push({ file, sha256: hash(fs.readFileSync(path.join(output, file))) });
  save('capture-written');
}

async function geometry(locator) {
  return locator.evaluate((el) => {
    const r = el.getBoundingClientRect();
    return {
      top: r.top,
      bottom: r.bottom,
      scrollY,
      viewport: innerHeight,
      focused: el === document.activeElement,
      visible: r.top >= 0 && r.top < innerHeight && r.bottom > 0,
    };
  });
}

async function open(page, name, method = 'keyboard') {
  const tile = page.getByRole('button', { name: `Open details for ${name}`, exact: true });
  if (method === 'keyboard') {
    await tile.focus();
    await tile.press('Enter');
  } else if (current.mobile) {
    await tile.tap();
  } else {
    await tile.click();
  }
  const panel = page.getByRole('group', { name: `${name} details`, exact: true });
  await expect(panel).toBeFocused();
  const heading = panel.getByRole('heading', { name, exact: true });
  const measured = await geometry(heading);
  assert.equal(
    measured.visible,
    true,
    'activation itself reveals the heading, without test-side scrolling',
  );
  await expect(tile).toHaveAttribute('aria-expanded', 'true');
  await expect(tile).toHaveAttribute('aria-controls', await panel.getAttribute('id'));
  record({ name: `open-${name}-${method}`, measured });
  return panel;
}

async function collapse(page, name, keyboard = false) {
  const collapse = page.getByRole('button', { name: `Collapse ${name} details`, exact: true });
  if (keyboard) {
    // Panel is the active, non-tab-stop group; the next tab is its collapse.
    await page.keyboard.press('Tab');
    await expect(collapse).toBeFocused();
    await page.keyboard.press('Enter');
  } else {
    await collapse.click();
  }
  const tile = page.getByRole('button', { name: `Open details for ${name}`, exact: true });
  await expect(tile).toBeFocused();
  await expect(tile).toHaveAttribute('aria-expanded', 'false');
  const measured = await geometry(tile);
  assert.equal(measured.visible, true, 'return reveals the current tile');
  record({ name: `collapse-${name}`, measured });
}

module.exports = async (selectedProfile) => {
  assert.ok(['desktop', 'phone', 'desktop-dark', 'phone-dark'].includes(selectedProfile));
  output = path.join(base, 'artifacts', selectedProfile);
  fs.mkdirSync(output, { recursive: true });
  report.startedAt = new Date().toISOString();
  save('preparing-preview');
  try {
    const { createServer } =
      await import('/workspace/frontend-modern/node_modules/vite/dist/node/index.js');
    server = await createServer({
      root: path.join(root, 'frontend-modern'),
      configFile: path.join(root, 'frontend-modern/vite.config.ts'),
      cacheDir: path.join(base, 'vite-cache'),
      server: { host: '127.0.0.1', port: 5262, strictPort: true, fs: { allow: [root] } },
      optimizeDeps: { noDiscovery: true },
    });
    await server.listen();
    save('preview-listening');
    for (const profile of [
      {
        name: 'desktop',
        viewport: { width: 1365, height: 900 },
        mobile: false,
        theme: 'light',
        full: true,
      },
      {
        name: 'phone',
        viewport: { width: 320, height: 844 },
        mobile: true,
        theme: 'light',
        full: true,
      },
      {
        name: 'desktop-dark',
        viewport: { width: 1365, height: 900 },
        mobile: false,
        theme: 'dark',
        full: false,
      },
      {
        name: 'phone-dark',
        viewport: { width: 320, height: 844 },
        mobile: true,
        theme: 'dark',
        full: false,
      },
    ].filter((profile) => profile.name === selectedProfile)) {
      current = { ...profile, cases: [], requests: [], errors: [], unexpected: [] };
      report.profiles.push(current);
      browser = await (profile.mobile
        ? webkit.launch({ headless: true })
        : chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] }));
      current.browser = browser.version();
      save('browser-launched');
      const context = await browser.newContext({
        viewport: profile.viewport,
        hasTouch: profile.mobile,
        isMobile: profile.mobile,
        reducedMotion: 'reduce',
        colorScheme: profile.theme,
      });
      const page = await context.newPage();
      page.on('pageerror', (error) => current.errors.push(error.message));
      await page.addInitScript((theme) => {
        document.addEventListener('DOMContentLoaded', () =>
          document.documentElement.classList.toggle('dark', theme === 'dark'),
        );
      }, profile.theme);
      await page.route('**/*', async (route) => {
        const req = route.request(),
          url = new URL(req.url());
        if (url.origin !== 'http://127.0.0.1:5262') {
          current.unexpected.push(req.url());
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        current.requests.push({ method: req.method(), path: url.pathname, query: url.search });
        if (url.pathname === '/api/availability-history' && req.method() === 'POST') {
          const body = req.postDataJSON();
          assert.equal(body.range, '24h');
          return route.fulfill({
            json: {
              ...history,
              targets: history.targets.filter((t) => body.targetIds.includes(t.targetId)),
            },
          });
        }
        if (req.method() === 'GET') {
          if (url.pathname === '/api/health') return route.fulfill({ json: { status: 'healthy' } });
          if (/^\/api\/resources\/availability%3Atarget-\d+\/facets$/.test(url.pathname))
            return route.fulfill({
              json: {
                capabilities: [],
                relationships: [],
                recentChanges: [],
                counts: { capabilities: 0, relationships: 0, recentChanges: 0 },
              },
            });
          if (url.pathname === '/api/ai/intelligence')
            return route.fulfill({ json: { enabled: false, findings: [], anomalies: [] } });
          if (url.pathname === '/api/audit/actions')
            return route.fulfill({ json: { audits: [], count: 0, available: false } });
          if (/^\/api\/resources\/availability%3Atarget-\d+\/operator-state$/.test(url.pathname))
            return route.fulfill({ json: { configured: false } });
          if (
            /^\/api\/resources\/availability%3Atarget-\d+\/maintenance-verifications$/.test(
              url.pathname,
            )
          )
            return route.fulfill({ json: { data: [], meta: { total: 0, limit: 25 } } });
          if (url.pathname.startsWith('/api/license/'))
            return route.fulfill({
              json: {
                capabilities: [],
                limits: [],
                max_history_days: 7,
                hosted_mode: false,
                runtime: { build: 'community' },
                blocked_capabilities: [],
                features: {},
              },
            });
        }
        current.unexpected.push(`${req.method()} ${url.pathname}`);
        return route.fulfill({ status: 405, json: { error: 'Unexpected fixture request' } });
      });
      await page.goto('http://127.0.0.1:5262/browser-tests/fleet-details.html', {
        waitUntil: 'domcontentloaded',
        timeout: 60000,
      });
      await expect(page.locator('[data-availability-fleet-tile]')).toHaveCount(50);
      await expect(page.getByText(/Insufficient coverage/).first()).toBeVisible();
      if (profile.full) await capture(page, 'grid');
      await open(page, 'Probe 05');
      await capture(page, 'opened');
      await collapse(page, 'Probe 05', true);
      await capture(page, 'returned');
      if (profile.full) {
        await open(page, 'Probe 10', 'pointer');
        await page.getByRole('button', { name: 'Filter history', exact: true }).click();
        const kind = page.getByRole('combobox', { name: 'Change kind', exact: true });
        await kind.selectOption('config_update');
        await kind.focus();
        const before = await geometry(kind);
        const oldTile = await page
          .getByRole('button', { name: 'Open details for Probe 10', exact: true })
          .elementHandle();
        await page.evaluate(() => window.fleetFixture.recover('availability:target-10'));
        const recovered = page.getByRole('group', {
          name: 'Probe 10 recovered details',
          exact: true,
        });
        await expect(recovered).toBeVisible();
        await expect(kind).toHaveValue('config_update');
        await expect(kind).toBeFocused();
        const after = await geometry(kind);
        assert.ok(
          Math.abs(before.scrollY - after.scrollY) <= 2,
          'polling must not scroll back to the header',
        );
        assert.equal(
          await oldTile.evaluate((el) => el.isConnected),
          false,
          'old tile snapshot detached',
        );
        await expect(recovered.getByText('7ms', { exact: true })).toBeVisible();
        record({ name: 'live-ID-recovery-retains-history-and-focus', before, after });
        await capture(page, 'history-preserved');
        await recovered.getByRole('tab', { name: 'Manage', exact: true }).click();
        await page.evaluate(() => window.fleetFixture.refresh());
        await expect(recovered.getByRole('tab', { name: 'Manage', exact: true })).toHaveAttribute(
          'aria-selected',
          'true',
        );
        record({ name: 'same-ID-refresh-retains-active-tab' });
        await collapse(page, 'Probe 10 recovered');
        await capture(page, 'reordered-return');
        await open(page, 'Probe 10 recovered');
        await page.evaluate(() => window.fleetFixture.remove('availability:target-10'));
        await expect(recovered).toHaveCount(0);
        const returned = await page.evaluate(() => ({
          id: document.activeElement?.getAttribute('data-availability-fleet-tile'),
          top: document.activeElement?.getBoundingClientRect().top,
        }));
        assert.ok(returned.id && returned.id !== 'availability:target-10');
        assert.ok(returned.top >= 0 && returned.top < profile.viewport.height);
        await page.evaluate(() => window.fleetFixture.restore('availability:target-10'));
        await expect(page.locator('[data-availability-fleet-tile]')).toHaveCount(50);
        await expect(
          page.locator('[aria-expanded="true"][data-availability-fleet-tile]'),
        ).toHaveCount(0);
        record({
          name: 'removed-selection-returns-to-live-neighbour-without-reopening',
          returned,
        });
        await capture(page, 'removal-return');

        const search = page.getByPlaceholder('Search availability checks', { exact: true });
        await search.fill('Probe 05');
        await expect(page.locator('[data-availability-fleet-tile]')).toHaveCount(1);
        await open(page, 'Probe 05');
        await collapse(page, 'Probe 05', true);
        await expect(search).toHaveValue('Probe 05');
        await open(page, 'Probe 05');
        await search.fill('Probe 20');
        await expect(
          page.getByRole('group', { name: 'Probe 05 details', exact: true }),
        ).toHaveCount(0);
        await expect(search).toBeFocused();
        await expect(search).toHaveValue('Probe 20');
        await expect(page.locator('[data-availability-fleet-tile]')).toHaveCount(1);
        record({
          name: 'search-preserved-on-return-and-not-refocused-when-filter-hides-selection',
        });
        await capture(page, 'filter-preserved');
        await open(page, 'Probe 20');
        await page.evaluate(() => window.fleetFixture.remove('availability:target-20'));
        await expect(
          page.getByText('No checks match current filters', { exact: true }),
        ).toBeVisible();
        await expect(search).toBeFocused();
        await expect(search).toHaveValue('Probe 20');
        record({ name: 'last-filtered-check-removal-returns-to-search' });
        await capture(page, 'empty-filter-return');
        await search.fill('');
        await open(page, 'Probe 05');
        await page.evaluate(() => window.fleetFixture.clear());
        const addCheck = page.getByRole('link', { name: 'Add service/device check', exact: true });
        await expect(addCheck).toBeFocused();
        assert.equal((await geometry(addCheck)).visible, true);
        record({ name: 'last-inventory-check-removal-returns-to-add-check' });
        await capture(page, 'empty-inventory-return');
      }
      assert.deepEqual(current.errors, []);
      assert.deepEqual(current.unexpected, []);
      await context.close();
      await browser.close();
      browser = undefined;
      current.closed = true;
      save('profile-closed');
    }
  } catch (error) {
    report.failures.push({ message: error.message, stack: error.stack });
    report.result = 'failed';
  } finally {
    if (browser) await browser.close();
    report.cleanup.browserClosed = true;
    if (server) await server.close();
    report.cleanup.serverClosed = true;
    report.completedAt = new Date().toISOString();
    if (!report.failures.length) report.result = 'passed';
    save('complete');
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2));
    console.log(JSON.stringify(report));
    if (report.failures.length) process.exitCode = 1;
  }
};
