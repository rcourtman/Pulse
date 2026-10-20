// Offline HTTP-to-production-Updates-panel proof. Fixtures never install a
// runtime, contact a broker, or carry an activation key.
process.env.RAYON_NUM_THREADS = '2';
process.env.UV_THREADPOOL_SIZE = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit, expect } = require('playwright/test');
const { default: AxeBuilder } = require('/workspace/tests/integration/node_modules/@axe-core/playwright');
const root = '/workspace';
const output = path.join(root, 'tmp/pro-runtime-switch-proof');
const binding = JSON.parse(fs.readFileSync(path.join(output, 'binding.json')));
const origin = 'http://127.0.0.1:5214';
const endpoint = '/api/updates/pro-runtime';
const version = '6.5.0';
const digest = 'sha256:' + 'ab'.repeat(32);
const image = `license.pulserelay.pro/pulse-pro@${digest}`;
const docker = {
  version, image: 'license.pulserelay.pro/pulse-pro', imageDigest: digest,
  loginCommand: "printf '%s' '<activation-key>' | docker login license.pulserelay.pro -u 'fixture' --password-stdin",
  composePullCommand: `PULSE_IMAGE='${image}' docker compose pull`,
  composeUpCommand: `PULSE_IMAGE='${image}' docker compose up -d`,
};
const offer = { available: true, currentVersion: version, version, installInPlace: true };
const refused = (reason, pin) => ({ available: false, reason, currentVersion: version, version: pin, installInPlace: false });
const report = { binding, playwright: require('playwright/package.json').version, results: [], captures: [], cleanup: {} };
let server, browser;
let current;
const sha = (buffer) => crypto.createHash('sha256').update(buffer).digest('hex');

async function capture(page, name) {
  const geometry = await page.evaluate(() => ({ width: innerWidth, scrollWidth: document.documentElement.scrollWidth }));
  assert.ok(geometry.scrollWidth <= geometry.width + 1, JSON.stringify(geometry));
  const file = `${current.name}-${name}.png`;
  await page.screenshot({ path: path.join(output, file), fullPage: true, animations: 'disabled' });
  report.captures.push({ file, sha256: sha(fs.readFileSync(path.join(output, file))) });
}
async function scan(page, state) {
  // The changed card, its commands and controls, not a claim about all old
  // Settings content. Keep results for every scan, including violations.
  const analysis = await new AxeBuilder({ page }).include('[data-testid="pro-runtime-switch"]')
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa']).analyze();
  const violations = analysis.violations.map((v) => ({ id: v.id, impact: v.impact, nodes: v.nodes.map((n) => n.target) }));
  current.scans.push({ state, violations });
  assert.deepEqual(violations, [], `${current.name}:${state}`);
}
async function open(page, options) {
  current = { name: options.name, requests: [], applies: 0, checkReads: 0, errors: [], unexpected: [], scans: [], assertions: [] };
  report.results.push(current);
  let applyMode = 'refused';
  let resolveApply;
  let copied = [];
  page.on('pageerror', (error) => current.errors.push(error.message));
  await page.addInitScript(() => {
    window.__copied = [];
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async (text) => { window.__copied.push(text); } } });
  });
  await page.route('**/*', async (route) => {
    const req = route.request();
    const url = new URL(req.url());
    if (url.origin !== origin) { current.unexpected.push('off-origin'); return route.abort(); }
    if (!url.pathname.startsWith('/api/')) return route.continue();
    current.requests.push(`${req.method()} ${url.pathname}`);
    if (url.pathname === '/api/license/runtime-capabilities') return route.fulfill({ json: {
      capabilities: [], limits: [], runtime: { build: options.build || 'community', label: 'Fixture runtime' },
    } });
    if (url.pathname === '/api/version') return route.fulfill({ json: {
      version, build: 'release', runtime: 'go', isDocker: Boolean(options.docker), isSourceBuild: Boolean(options.source), isDevelopment: false,
      deploymentType: options.docker ? 'docker' : 'systemd',
    } });
    if (url.pathname === '/api/updates/history') return route.fulfill({ json: [] });
    if (url.pathname === endpoint && req.method() === 'GET') {
      current.checkReads++;
      if (options.checkFailure) return route.fulfill({ status: 503, body: 'Fixture check unavailable' });
      return route.fulfill({ json: options.answer });
    }
    if (url.pathname === endpoint + '/apply' && req.method() === 'POST') {
      current.applies++;
      assert.equal(req.postData(), null);
      if (applyMode === 'lost') return route.abort('connectionreset');
      if (applyMode === 'pending') await new Promise((resolve) => { resolveApply = resolve; });
      if (applyMode === 'accepted') return route.fulfill({ json: { status: 'started', message: 'Fixture request accepted' } });
      // A retry-capable client would dispatch twice on this CSRF refresh.
      return route.fulfill({ status: 403, headers: { 'X-CSRF-Token': 'fixture-refreshed-csrf' }, body: 'Fixture refused before dispatch' });
    }
    current.unexpected.push(`${req.method()} ${url.pathname}`);
    return route.fulfill({ status: 405, body: 'Unexpected fixture request' });
  });
  await page.goto(origin + '/browser-tests/pro-runtime-switch.html', { waitUntil: 'domcontentloaded' });
  await page.waitForFunction(() => document.documentElement.dataset.ready === 'true');
  return { setApply: (mode) => { applyMode = mode; }, releaseApply: () => resolveApply?.(), copy: async () => { copied = await page.evaluate(() => window.__copied); return copied; } };
}
async function finish(page) {
  assert.deepEqual(current.errors, []);
  assert.deepEqual(current.unexpected, []);
  assert.equal(await page.getByTestId('settings-dirty').innerText(), 'saved');
  await page.close();
}

(async () => {
  fs.mkdirSync(output, { recursive: true });
  process.chdir(path.join(root, 'frontend-modern'));
  const { createServer } = await import(path.join(root, 'frontend-modern/node_modules/vite/dist/node/index.js'));
  server = await createServer({ root: path.join(root, 'frontend-modern'), configFile: path.join(root, 'frontend-modern/vite.config.ts'), server: { host: '127.0.0.1', port: 5214, strictPort: true } });
  try {
    await server.listen();
    for (const [engine, label, viewport] of [[chromium, 'desktop', { width: 1365, height: 900 }], [webkit, 'phone', { width: 320, height: 844 }]]) {
      browser = await engine.launch(label === 'desktop' ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const context = await browser.newContext({ viewport, hasTouch: label === 'phone' });
      await context.addCookies([{ name: 'pulse_csrf', value: 'fixture-csrf', url: origin }]);
      report.results.push({ browser: label, version: browser.version(), viewport });
      let page, controls;
      if (binding.phase !== 'docker-final') {
      page = await context.newPage();
      controls = await open(page, { name: label + '-in-place', answer: offer });
      let button = page.getByRole('button', { name: 'Switch to Pulse Pro v6.5.0' });
      await expect(button).toBeVisible();
      await capture(page, 'offer');
      await scan(page, 'offer');
      await button.focus(); await page.keyboard.press('Enter');
      await expect(page.getByRole('button', { name: 'Switch now' })).toBeFocused();
      assert.equal(current.applies, 0);
      await capture(page, 'confirm');
      await scan(page, 'confirm');
      await page.getByRole('button', { name: 'Cancel' }).click();
      await expect(button).toBeFocused();
      assert.equal(current.applies, 0);
      await button.click();
      await page.getByRole('button', { name: 'Switch now' }).click();
      await expect(page.getByText('Fixture refused before dispatch', { exact: true })).toBeVisible();
      assert.equal(current.applies, 1, 'a refused write must not be retried automatically');
      await expect(page.getByRole('button', { name: 'Switch now' })).toBeEnabled();
      await expect(page.getByRole('button', { name: 'Switch now' })).toBeFocused();
      await capture(page, 'refusal');
      controls.setApply('pending');
      await page.getByRole('button', { name: 'Switch now' }).click();
      await expect(page.getByRole('button', { name: 'Cancel' })).toBeDisabled();
      await expect(page.getByRole('button', { name: 'Switch now' })).toBeDisabled();
      await expect.poll(() => current.applies).toBe(2);
      controls.setApply('accepted'); controls.releaseApply();
      await expect(page.getByRole('status').filter({ hasText: 'Switch requested.' })).toBeFocused();
      await expect(page.getByRole('button', { name: /Switch/ })).toHaveCount(0);
      await capture(page, 'accepted');
      current.assertions.push('explicit confirmation, cancel/focus return, one refused POST, one owned pending POST and accepted-not-completed notice');
      await finish(page);

      page = await context.newPage();
      controls = await open(page, { name: label + '-lost', answer: offer });
      controls.setApply('lost');
      await page.getByRole('button', { name: /Switch to Pulse Pro/ }).click();
      await page.getByRole('button', { name: 'Switch now' }).click();
      await expect(page.getByRole('status')).toContainText('Could not confirm whether the switch started.');
      await expect(page.getByRole('status')).toBeFocused();
      await expect(page.getByRole('button', { name: /Switch/ })).toHaveCount(0);
      assert.equal(current.applies, 1);
      await scan(page, 'uncertain request'); await capture(page, 'unknown');
      current.assertions.push('lost response is not failure or completion and has no duplicate switch offer');
      await finish(page);
      }

      page = await context.newPage();
      controls = await open(page, { name: label + (binding.phase === 'docker-final' ? '-docker-final' : '-docker'), docker: true, answer: { ...offer, installInPlace: false, docker } });
      const card = page.getByTestId('pro-runtime-switch');
      await expect(card).toContainText(docker.composePullCommand);
      const copies = card.getByRole('button', { name: 'Copy to clipboard' });
      await expect(copies).toHaveCount(3);
      for (let i = 0; i < 3; i++) await copies.nth(i).click();
      assert.deepEqual(await controls.copy(), [docker.loginCommand, docker.composePullCommand, docker.composeUpCommand]);
      await expect(card.getByRole('button', { name: /Switch/ })).toHaveCount(0);
      const noteLines = await card.locator('p').last().evaluate((element) => {
        const box = element.getBoundingClientRect();
        const range = document.createRange(); range.selectNodeContents(element);
        return { left: box.left, right: box.right, lines: [...range.getClientRects()].map((r) => ({ left: r.left, right: r.right })) };
      });
      assert.ok(noteLines.lines.every((r) => r.left >= noteLines.left - 1 && r.right <= noteLines.right + 1), JSON.stringify(noteLines));
      await scan(page, 'docker light'); await capture(page, 'commands-light');
      await page.evaluate(() => document.documentElement.classList.add('dark'));
      await scan(page, 'docker dark'); await capture(page, 'commands-dark');
      assert.equal(current.applies, 0);
      current.assertions.push('three exact synthetic commands copied; digest and compose-variable guidance wrap in both themes');
      current.noteLines = noteLines;
      await finish(page);
      if (binding.phase === 'docker-final') {
        await context.close(); await browser.close(); browser = undefined;
        continue;
      }

      for (const [kind, pin, title] of [
        ['prerelease_only', '6.6.0-rc.4', 'The stable Pulse Pro build is not out yet'],
        ['older_release', '6.4.5', 'The Pulse Pro build is not out for this version yet'],
        ['unavailable', undefined, 'Could not check for the Pulse Pro build.'],
      ]) {
        page = await context.newPage();
        await open(page, { name: label + '-' + kind, answer: refused(kind, pin) });
        await expect(page.getByTestId('pro-runtime-switch')).toContainText(title);
        if (pin) await expect(page.getByTestId('pro-runtime-switch')).toContainText('v' + pin);
        await expect(page.getByTestId('pro-runtime-switch').getByRole('button')).toHaveCount(0);
        await expect(page.getByText('Stable', { exact: true }).locator('..')).toBeVisible();
        await scan(page, kind); await capture(page, 'notice');
        assert.equal(current.applies, 0);
        current.assertions.push('informational state only; stable channel and saved settings unchanged');
        await finish(page);
      }
      for (const reason of ['not_activated', 'not_eligible', 'unsupported_deployment', 'future_reason']) {
        page = await context.newPage();
        await open(page, { name: label + '-' + reason, answer: refused(reason) });
        await expect.poll(() => current.checkReads).toBe(1);
        await expect(page.getByTestId('pro-runtime-switch')).toHaveCount(0);
        assert.equal(current.applies, 0);
        current.assertions.push('ineligible or unknown reason stays hidden');
        await finish(page);
      }
      for (const options of [{ build: 'pro' }, { source: true }, { build: 'unknown' }]) {
        page = await context.newPage();
        await open(page, { ...options, name: label + '-no-check-' + (options.build || 'source'), answer: offer });
        await expect(page.getByTestId('pro-runtime-switch')).toHaveCount(0);
        assert.equal(current.checkReads, 0);
        assert.equal(current.applies, 0);
        current.assertions.push('compiled build/source boundary makes no switch request');
        await finish(page);
      }
      page = await context.newPage();
      await open(page, { name: label + '-check-failure', checkFailure: true });
      await expect(page.getByTestId('pro-runtime-switch')).toContainText('Could not check');
      await expect(page.getByTestId('pro-runtime-switch').getByRole('button')).toHaveCount(0);
      current.assertions.push('transport failure stays visible without an action or raw error');
      await finish(page);
      await context.close(); await browser.close(); browser = undefined;
    }
    report.result = 'passed';
  } catch (error) {
    report.result = 'failed'; report.error = error.stack;
    throw error;
  } finally {
    if (browser) await browser.close();
    report.cleanup.browserClosed = true;
    if (server) await server.close();
    report.cleanup.serverClosed = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(JSON.stringify(report));
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
