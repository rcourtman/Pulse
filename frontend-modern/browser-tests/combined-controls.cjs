// Fresh final-content proof for merged select, background, link and button repairs.
// Starts only a guest-local preview; all status responses are synthetic and read-only.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium, webkit } = require('playwright');

const verifiedParent = '737b6c27e17cdd8234030c357e0866b6dfb6c730';
const expectedContent = {
  'frontend-modern/src/index.css': '5cbd2ebb37fb3b50926200da17e391fb3bc8a8e2a987b151c2c80f48b38526df',
  'frontend-modern/src/components/Settings/BackupTransferDialogs.tsx': '23c58ab753541d158d20fa1dd4e2bbeafb5a5374d456898931f52b31494b57e6',
  'frontend-modern/src/features/alerts/AlertDeadManDestinationSection.tsx': '9dce4c868a2c2e3898e582ab58e072c7ff48968e95f9aeabee17ada9129f1659',
};
const digest = (file) => createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function settle(page) {
  await page.evaluate(async () => {
    await document.fonts.ready;
    await Promise.all(document.getAnimations().map((animation) => animation.finished.catch(() => {})));
    await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  });
}

async function paint(locator) {
  return locator.evaluate((element) => {
    const style = getComputedStyle(element);
    const rect = element.getBoundingClientRect();
    return {
      background: style.backgroundColor, color: style.color,
      backgroundImage: style.backgroundImage, appearance: style.appearance,
      height: rect.height, width: rect.width, paddingRight: style.paddingRight,
    };
  });
}

(async () => {
  assert.equal(require('playwright/package.json').version, '1.56.1');
  for (const [file, hash] of Object.entries(expectedContent)) assert.equal(digest(file), hash, file);
  const root = path.resolve('frontend-modern');
  const artifacts = path.join(root, 'node_modules', 'combined-controls-proof');
  fs.mkdirSync(artifacts, { recursive: true });
  const binding = Object.fromEntries([
    'frontend-modern/package.json', 'frontend-modern/package-lock.json',
    'tests/integration/package-lock.json',
    'frontend-modern/browser-tests/combined-controls.tsx',
    'frontend-modern/browser-tests/combined-controls.html',
    'frontend-modern/browser-tests/combined-controls.cjs',
  ].map((file) => [file, digest(file)]));
  const integration = JSON.parse(fs.readFileSync('tests/integration/package-lock.json', 'utf8'));
  assert.equal(integration.packages['node_modules/@playwright/test'].version, '1.56.1');
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-combined-controls'),
    server: { host: '127.0.0.1', port: 5266, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  const screenshots = [];
  const results = [];
  let browser;
  let failure;
  const cleanup = { browserClosed: false, serverClosed: false };
  const capture = async (page, name, locator) => {
    await locator.scrollIntoViewIfNeeded();
    await settle(page);
    const file = `${name}.png`;
    await locator.screenshot({ path: path.join(artifacts, file) });
    screenshots.push({ file, sha256: digest(path.join(artifacts, file)) });
  };
  try {
    await server.listen();
    for (const [engine, width] of [['chromium', 1440], ['webkit', 390]]) {
      browser = await { chromium, webkit }[engine].launch(engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true });
      for (const tone of ['light', 'dark']) {
        const context = await browser.newContext({
          viewport: { width, height: 1100 },
          colorScheme: tone,
          ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}),
        });
        const page = await context.newPage();
        const errors = [];
        const unexpectedRequests = [];
        const statusReads = [];
        let state = 'healthy';
        let slowStatus = false;
        page.on('pageerror', (error) => errors.push(error.message));
        page.on('console', (message) => { if (message.type() === 'error') errors.push(message.text()); });
        await context.route('**/*', async (route) => {
          const request = route.request();
          const url = new URL(request.url());
          if (url.origin !== 'http://127.0.0.1:5266' || request.method() !== 'GET') {
            unexpectedRequests.push({ origin: url.origin, method: request.method() });
            await route.abort();
          } else if (url.pathname === '/api/alerts/deadman/status') {
            statusReads.push(state);
            if (slowStatus) await delay(500);
            await route.fulfill({ contentType: 'application/json', body: JSON.stringify({
              configured: state !== 'disabled', state, heartbeatIntervalSeconds: 60,
              recommendedGraceSeconds: 180, consecutiveFailures: state === 'delivery_failed' ? 2 : 0,
              lastSuccessAt: new Date().toISOString(),
              lastMonitoringProgress: new Date().toISOString(),
              ...(state === 'delivery_failed' ? { lastError: 'Offline fixture: watchdog rejected the ping.' } : {}),
            }) });
          } else if (url.pathname.startsWith('/api/')) {
            unexpectedRequests.push({ pathname: url.pathname, method: request.method() });
            await route.abort();
          } else await route.continue();
        });
        await page.goto('http://127.0.0.1:5266/browser-tests/combined-controls.html', {
          waitUntil: 'networkidle', timeout: 60_000,
        });
        await page.evaluate((dark) => document.documentElement.classList.toggle('dark', dark), tone === 'dark');
        await page.getByText('Heartbeat healthy', { exact: true }).waitFor();
        const prefix = `${engine}-${width}-${tone}`;
        const select = page.getByRole('combobox', { name: 'CPU averaging', exact: true });
        const compact = page.getByRole('combobox', { name: 'Platform override', exact: true });
        const initial = { select: await paint(select), compact: await paint(compact) };
        for (const control of Object.values(initial)) {
          assert.ok(control.backgroundImage.includes('linear-gradient'), `${prefix}: caret`);
          assert.equal(control.appearance, 'none');
          assert.ok(parseFloat(control.paddingRight) >= 32);
          assert.ok(control.height >= (width < 640 ? 44 : 32));
        }
        assert.equal(await select.inputValue(), '5m');
        assert.equal(await compact.inputValue(), 'inherit');
        await select.focus();
        assert.ok(await select.evaluate((element) => document.activeElement === element));
        await select.selectOption('15m');
        await compact.selectOption('5m');
        const filters = page.getByRole('group', { name: 'Resource status', exact: true });
        const active = filters.getByRole('button', { name: 'Active resources', exact: true });
        await active.focus();
        await page.keyboard.press('Enter');
        assert.equal(await active.getAttribute('aria-pressed'), 'true');
        assert.equal(await page.getByLabel('Selected controls').innerText(), '15m / 5m / active');
        // Compare the settled selection, not an intermediate transition frame.
        await settle(page);
        const well = await paint(filters);
        const selected = await paint(active);
        const command = await paint(page.locator('[data-testid="controls-card"] code'));
        const card = await paint(page.locator('[data-testid="controls-card"]'));
        const muted = await paint(page.locator('[data-testid="muted-card"]'));
        const pageBackground = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
        assert.equal(well.background, pageBackground);
        assert.equal(command.background, pageBackground);
        assert.equal(selected.background, card.background);
        assert.notEqual(well.background, card.background);
        // The existing dark theme deliberately shares surface/alternate tokens.
        // Require the painted alternate surface, not a new theme distinction.
        assert.equal(muted.background, tone === 'dark' ? card.background : pageBackground);
        await capture(page, `${prefix}-controls`, page.locator('[data-testid="controls-card"]'));

        const watchdog = page.locator('[data-settings-panel]').filter({ has: page.getByRole('heading', { name: 'External watchdog', exact: true }) });
        const show = watchdog.getByRole('button', { name: 'Show', exact: true });
        const refresh = watchdog.getByRole('button', { name: 'Refresh', exact: true });
        const buttonDimensions = { show: await paint(show), refresh: await paint(refresh) };
        if (width < 640) for (const button of Object.values(buttonDimensions)) assert.ok(button.height >= 44);
        const urlInput = watchdog.getByLabel('Healthchecks-compatible success ping URL');
        assert.equal(await urlInput.getAttribute('type'), 'password');
        await show.focus();
        await page.keyboard.press('Enter');
        const hide = watchdog.getByRole('button', { name: 'Hide', exact: true });
        assert.equal(await hide.getAttribute('aria-pressed'), 'true');
        assert.equal(await urlInput.getAttribute('type'), 'text');
        assert.equal(await urlInput.inputValue(), 'https://watchdog.invalid/offline-fixture');
        await hide.click();
        assert.equal(await urlInput.getAttribute('type'), 'password');
        assert.equal(await page.getByLabel('Unsaved watchdog change').innerText(), 'false');
        state = 'delivery_failed';
        slowStatus = true;
        await refresh.focus();
        await page.keyboard.press('Enter');
        await watchdog.getByRole('button', { name: 'Refreshing…', exact: true }).waitFor();
        assert.equal(await watchdog.getByRole('button', { name: 'Refreshing…', exact: true }).isDisabled(), true);
        await page.getByText('Delivery failing', { exact: true }).waitFor();
        slowStatus = false;
        assert.equal(await refresh.isDisabled(), false);
        const statusWell = watchdog.locator('.bg-page');
        assert.equal((await paint(statusWell)).background, pageBackground);
        await capture(page, `${prefix}-watchdog`, watchdog);
        state = 'disabled';
        await refresh.click();
        await page.getByText('Not configured', { exact: true }).waitFor();
        assert.equal((await paint(watchdog.getByText('Not configured', { exact: true }))).background, card.background);

        const dialogs = [];
        for (const [opener, title] of [['Create backup', 'Export configuration'], ['Restore backup', 'Import configuration']]) {
          await page.getByRole('button', { name: opener, exact: true }).click();
          const dialog = page.getByRole('dialog', { name: title, exact: true });
          await dialog.waitFor();
          const link = dialog.getByRole('link', { name: 'migration guide', exact: true });
          assert.equal(await link.getAttribute('href'), '/docs/MIGRATION');
          assert.equal(await link.getAttribute('target'), '_blank');
          const rel = (await link.getAttribute('rel')).split(/\s+/);
          assert.ok(rel.includes('noopener') && rel.includes('noreferrer'));
          const linkPaint = await link.evaluate((element) => {
            const style = getComputedStyle(element);
            return {
              color: style.color, parentColor: getComputedStyle(element.parentElement).color,
              decoration: style.textDecorationLine, display: style.display,
              text: element.parentElement.textContent.replace(/\s+/g, ' '),
            };
          });
          assert.equal(linkPaint.color, linkPaint.parentColor);
          assert.equal(linkPaint.display, 'inline');
          assert.ok(linkPaint.decoration.includes('underline'));
          assert.ok(linkPaint.text.includes(title.startsWith('Export') ? 'Configuration only:' : 'Back up the destination first.'));
          await link.focus();
          assert.ok(await link.evaluate((element) => document.activeElement === element));
          if (title.startsWith('Export')) assert.equal((await paint(dialog.locator('.bg-page'))).background, pageBackground);
          assert.equal(await dialog.getByRole('button', { name: title.startsWith('Export') ? 'Export' : 'Import', exact: true }).isDisabled(), true);
          await capture(page, `${prefix}-${title.startsWith('Export') ? 'export' : 'import'}`, dialog);
          dialogs.push({ title, ...linkPaint, destination: '/docs/MIGRATION', safeNewTab: true, keyboardFocus: true });
          await page.keyboard.press('Escape');
          await dialog.waitFor({ state: 'hidden' });
        }
        await page.getByRole('button', { name: 'Use stored watchdog URL', exact: true }).click();
        assert.equal(await urlInput.inputValue(), '');
        assert.equal(await urlInput.getAttribute('placeholder'), 'Configured — enter a new URL to replace');
        assert.equal(await watchdog.getByRole('button', { name: 'Show', exact: true }).count(), 0);
        await watchdog.getByRole('button', { name: 'Remove', exact: true }).click();
        assert.equal(await page.getByLabel('Unsaved watchdog change').innerText(), 'true');
        assert.equal(await page.getByLabel('Unexpected mutations').innerText(), '0');
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        let forcedColors;
        if (engine === 'chromium') {
          await page.emulateMedia({ forcedColors: 'active' });
          forcedColors = await paint(select);
          assert.equal(forcedColors.appearance, 'auto');
          assert.equal(forcedColors.backgroundImage, 'none');
          await page.emulateMedia({ forcedColors: 'none' });
        }
        assert.deepEqual(errors, []);
        assert.deepEqual(unexpectedRequests, []);
        assert.deepEqual(statusReads, ['healthy', 'delivery_failed', 'disabled']);
        results.push({ engine, browserVersion: browser.version(), width, height: 1100, tone,
          initial, backgrounds: { pageBackground, well, selected, command, card, muted },
          buttonDimensions, statusReads, dialogs, forcedColors, keyboardSelection: true,
          showHideNoSave: true, refreshLoadingDisabled: true, redactedValueRemainsHidden: true,
          noBodyOverflow: true, errors, unexpectedRequests, unexpectedMutations: 0 });
        await context.close();
      }
      await browser.close();
      browser = undefined;
    }
  } catch (error) { failure = error; }
  finally {
    if (browser) await browser.close();
    cleanup.browserClosed = true;
    await server.close();
    cleanup.serverClosed = true;
    const report = {
      result: failure ? 'failed' : 'passed', verifiedParent, content_sha256: expectedContent, binding,
      playwrightVersion: require('playwright/package.json').version,
      verifiedAt: new Date().toISOString(), results, screenshots, cleanup,
      scope: 'Production components/CSS in an offline fixture; no actual watchdog, backup, migration, installation or physical-phone acceptance. Guide-link destination/attributes/focus, not destination contents.',
      ...(failure ? { failure: failure.message } : {}),
    };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(JSON.stringify(report));
  }
  if (failure) throw failure;
})().catch((error) => { console.error(error); process.exitCode = 1; });
