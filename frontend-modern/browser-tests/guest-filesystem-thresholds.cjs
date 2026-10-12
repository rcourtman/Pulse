const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const origin = 'http://127.0.0.1:5333';
const runtime = [
  'frontend-modern/src/features/alerts/thresholds/hooks/useThresholdsGuestData.ts',
  'frontend-modern/src/features/alerts/thresholds/tableTypes.ts',
  'frontend-modern/src/components/Alerts/AlertResourceTableRow.tsx',
  'frontend-modern/src/components/Alerts/AlertResourceTableMobile.tsx',
];
const hash = file => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const settle = page => page.evaluate(async () => {
  await document.fonts.ready;
  await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
});
async function verify(original = false) {
  const output = `/workspace/tmp/guest-filesystem-thresholds/browser-${original ? 'original' : 'final'}`;
  fs.mkdirSync(output, { recursive: true });
  const result = {
    result: 'incomplete', phase: original ? 'original-adverse-observation' : 'candidate-verification',
    playwright: require('playwright/package.json').version,
    runtime_hashes: Object.fromEntries(runtime.map(file => [file, hash('/workspace/' + file)])),
    cases: [], screenshots: [], cleanup: { browsers: false, server: false },
    limits: 'Production hook, ResourceTable/desktop/mobile editors and CSS; synthetic resources only. No native QGA, thaw, filesystem writes, service restoration, alert delivery, reporter, installed or release acceptance.',
  };
  assert.equal(result.playwright, JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages['node_modules/@playwright/test'].version);
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({ root, configFile: path.join(root, 'vite.config.ts'), cacheDir: output + '/cache', optimizeDeps: {entries: ['browser-tests/guest-filesystem-thresholds.html']}, server: { host: '127.0.0.1', port: 5333, strictPort: true }});
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, height, dark] of [
      ['chromium-desktop-light', chromium, 1280, 900, false],
      ['webkit-phone-dark', webkit, 390, 844, true],
    ]) {
      browser = await engine.launch(engine === chromium ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const context = await browser.newContext({viewport: { width, height }, isMobile: width < 768, hasTouch: width < 768, locale: 'en-GB', timezoneId: 'UTC'});
      const page = await context.newPage();
      page.setDefaultTimeout(12000);
      const record = {name, browser: browser.version(), width, height, dark, errors: [], requests: [], checks: [], states: []};
      result.cases.push(record);
      page.on('pageerror', error => record.errors.push(error.message));
      await page.route('**/*', route => {
        const request = route.request(), url = new URL(request.url());
        if (url.origin !== origin) { record.requests.push({method: request.method(), path: url.pathname, disposition: 'off-origin blocked'}); return route.abort(); }
        if (url.pathname.startsWith('/api/')) { record.requests.push({method: request.method(), path: url.pathname, disposition: 'synthetic'}); return route.fulfill({json: {data: [], resources: [], alerts: [], enabled: false, hasAuthentication: true}}); }
        return route.continue();
      });
      await page.addInitScript(isDark => {
        document.addEventListener('DOMContentLoaded', () => { if (isDark) document.documentElement.classList.add('dark'); });
        window.__trustedInput = [];
        document.addEventListener('click', event => window.__trustedInput.push({type: 'click', trusted: event.isTrusted}), true);
        document.addEventListener('keydown', event => window.__trustedInput.push({type: event.key, trusted: event.isTrusted}), true);
      }, dark);
      await page.goto(`${origin}/browser-tests/guest-filesystem-thresholds.html`, {waitUntil: 'domcontentloaded', timeout: 45000});
      await page.waitForFunction(() => window.__guestFilesystemThresholds, null, {timeout: 45000});
      const apply = async next => { await page.evaluate(next => window.__guestFilesystemThresholds.apply(next), next); await settle(page); };
      const row = () => page.getByText('/data', {exact: true}).first().locator(width < 768 ? 'xpath=ancestor::div[contains(@class,"transition-opacity")][1]' : 'xpath=ancestor::tr');
      const text = async () => (await row().innerText()).replace(/\s+/g, ' ').trim();
      const capture = async state => {
        await row().scrollIntoViewIfNeeded(); await settle(page);
        const file = `${name}-${state}.png`;
        await page.screenshot({path: path.join(output, file), fullPage: true});
        result.screenshots.push({file, sha256: hash(path.join(output, file)), name, state});
      };
      await settle(page);
      if (original) {
        for (const [state, next] of [
          ['retained', {reason: 'prev-vm-locked', power: 'running'}],
          ['unavailable', {reason: 'agent-busy'}],
          ['stopped', {reason: '', power: 'stopped'}],
        ]) {
          await apply(next);
          const value = await text();
          assert.match(value, /9\.0 \/ 10\.0 GB · 90\.0%/);
          assert.doesNotMatch(value, /Last known|unavailable|paused|deferred/);
          record.states.push({state, text: value});
          record.checks.push(`${state} numeric carrier appears unqualified in the original production row.`);
          await capture(state);
        }
      } else {
        assert.match(await text(), /Last known 9\.0 \/ 10\.0 GB · 90\.0%/);
        assert.match(await text(), /Guest reads paused while Proxmox reports a VM operation lock/);
        await capture('retained');
        record.checks.push('Explicit retained data is labelled Last known beside its mount; the reason is visible, not hover-only.');
        const reasons = await page.evaluate(() => window.__guestFilesystemThresholds.deferrals.map(item => item[0]));
        for (const reason of reasons) {
          await apply({reason: `prev-${reason}`});
          assert.match(await text(), /Last known 9\.0 \/ 10\.0 GB · 90\.0%/);
          await apply({reason});
          assert.match(await text(), /Usage unavailable/);
          assert.doesNotMatch(await text(), /90\.0%|9\.0 \/ 10\.0/);
        }
        record.checks.push('All eight fixed guest-read reasons separate retained values from unavailable numeric carriers.');
        await capture('unavailable');
        await apply({reason: '', power: 'stopped'});
        assert.match(await text(), /Usage unavailable/);
        assert.match(await text(), /while the VM is stopped/);
        assert.doesNotMatch(await text(), /90\.0%/);
        await capture('stopped');
        record.checks.push('Provider-native stopped state wins even when the canonical row is online and no read reason remains.');
        await apply({reason: 'prev-agent-busy', power: 'running'});
        assert.match(await text(), /Last known/);
        await apply({reason: '', usage: 0});
        assert.match(await text(), /0\.0 \/ 10\.0 GB · 0\.0%/);
        assert.doesNotMatch(await text(), /Last known|Usage unavailable|deferred/);
        record.checks.push('Same-mount restart/deferral/fresh-zero replacements are reactive without reloading the page.');
        await apply({reason: 'permission-denied'});
        assert.match(await text(), /Do not widen intentionally scoped access/);
        const reason = row().getByText(/Permission denied\. If this VM/);
        const bounds = await reason.evaluate(element => ({width: element.clientWidth, scrollWidth: element.scrollWidth, whiteSpace: getComputedStyle(element).whiteSpace, text: element.innerText}));
        assert.ok(bounds.scrollWidth <= bounds.width + 1, JSON.stringify(bounds));
        assert.equal(bounds.whiteSpace, 'normal');
        assert.match(bounds.text, /Defer access changes and live probes/);
        record.permission_layout = bounds;
        await capture('permission-denied');
        record.checks.push('Full refusal/safety explanation wraps on desktop and phone without truncation.');
        const edit = row().getByRole('button', {name: 'Edit thresholds for /data'});
        if (width < 768) await edit.tap(); else { await edit.focus(); await page.keyboard.press('Enter'); }
        await row().getByRole('button', {name: width < 768 ? 'Cancel threshold edits' : 'Cancel editing'}).click();
        assert.deepEqual(await page.evaluate(() => window.__guestFilesystemThresholds.actions()), ['edit:fixture-a:pve-a:101-disk-data-dev-vda1', 'cancel']);
        assert.match(await text(), /95%/);
        record.checks.push('Keyboard/touch edit and cancel preserve the intended VM filesystem identity and saved custom threshold.');
        await apply({reason: 'prev-agent-busy', power: 'stopped', type: 'system-container', usage: 30});
        assert.match(await text(), /3\.0 \/ 10\.0 GB · 30\.0%/);
        assert.doesNotMatch(await text(), /Last known|Usage unavailable/);
        record.checks.push('Independent stopped LXC platform storage ignores VM-only guest-read reasons.');
        await apply({type: 'vm', power: 'running', data: false});
        assert.match(await text(), /Usage unavailable/);
        assert.match(await text(), /Saved alert settings are retained/);
        assert.match(await text(), /95%/);
        await capture('missing-reading');
        const rows = await page.evaluate(() => window.__guestFilesystemThresholds.rows());
        assert.equal(rows.length, 2);
        assert.equal(rows.find(value => value.node === 'fresh-control').subtitle, '3.0 / 10.0 GB · 30.0%');
        assert.equal(rows.find(value => value.node === 'backup-guest').overrideStorageId, 'guest-disk:guest:fixture-a:101/disk:data-dev-vda1');
        record.checks.push('Missing samples retain editable saved settings; identical VMID/mount in another PVE connection remains fresh and separately addressed.');
        assert.ok((await page.evaluate(() => window.__trustedInput)).some(event => event.trusted));
      }
      record.layout = await page.evaluate(() => ({width: document.documentElement.clientWidth, scrollWidth: document.documentElement.scrollWidth}));
      assert.ok(record.layout.scrollWidth <= record.layout.width + 1, JSON.stringify(record.layout));
      assert.deepEqual(record.errors, []);
      assert.equal(record.requests.filter(request => !['GET','HEAD'].includes(request.method)).length, 0);
      assert.equal(record.requests.filter(request => request.disposition === 'off-origin blocked').length, 0);
      record.checks.push('No page error, page-level overflow, off-origin or mutating request.');
      await context.close(); await browser.close(); browser = null;
    }
    result.result = original ? 'original-defect-reproduced' : 'passed';
  } catch (error) {
    result.error = String(error.stack || error); throw error;
  } finally {
    if (browser) await browser.close(); result.cleanup.browsers = true;
    await server.close(); result.cleanup.server = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify({result: result.result, cases: result.cases.length, checks: result.cases.reduce((sum, value) => sum + value.checks.length, 0), captures: result.screenshots.length, cleanup: result.cleanup}));
  }
}
module.exports = { verify };
if (require.main === module) verify().catch(error => { console.error(error); process.exitCode = 1; });
