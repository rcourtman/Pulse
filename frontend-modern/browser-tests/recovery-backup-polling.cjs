// Exact source -> real settings loader/draft/save -> Recovery panel -> shipped safety Docs.
// All API state is synthetic and loopback-only. No monitoring, guest or backup operation.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, webkit } = require('playwright');
const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const variant = fs.readFileSync(path.join(root, 'tmp/recovery-backup/variant.txt'), 'utf8').trim();
assert(['before', 'after'].includes(variant));
const compiled = path.join(root, 'tmp/recovery-backup-static', variant);
const output = path.join(root, 'tmp/recovery-backup', variant);
const origin = 'http://127.0.0.1:5297';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const binding = JSON.parse(fs.readFileSync(path.join(compiled, 'binding.json')));
const result = {
  variant, binding, playwright: require('playwright/package.json').version,
  cases: [], captures: [], mutations: [], csrfReads: 0, errors: [], unexpectedRequests: [], cleanup: {},
};
let server;
let browser;
let saved;
let locked;
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.md': 'text/markdown' };
const assets = new Map();

async function capture(page, name, section) {
  const file = path.join(output, `${name}.png`);
  // Keep the panel padding in frame: an element-only crop cuts off the
  // checkbox's outside focus outline and cannot establish its actual bounds.
  if (section) await section.scrollIntoViewIfNeeded();
  await page.screenshot({ path: file });
  result.captures.push({ path: path.relative(root, file), sha256: hash(fs.readFileSync(file)) });
}
async function focusEvidence(toggle) {
  return toggle.evaluate((input) => {
    const track = input.nextElementSibling;
    const style = getComputedStyle(track);
    const box = input.parentElement.getBoundingClientRect();
    return {
      focused: document.activeElement === input,
      focusVisible: input.matches(':focus-visible'),
      outlineStyle: style.outlineStyle, outlineColor: style.outlineColor,
      outlineWidth: style.outlineWidth, outlineOffset: style.outlineOffset,
      boxShadow: style.boxShadow, target: { width: box.width, height: box.height },
      description: (input.getAttribute('aria-describedby') || '').split(/\s+/).filter(Boolean)
        .map((id) => document.getElementById(id)?.textContent.replace(/\s+/g, ' ').trim()).join(' '),
    };
  });
}
async function noOverflow(page) {
  const value = await page.evaluate(() => ({ width: innerWidth, scroll: document.documentElement.scrollWidth }));
  assert(value.scroll <= value.width, JSON.stringify(value));
  return value;
}

(async () => {
  fs.mkdirSync(output, { recursive: true });
  for (const [file, digest] of Object.entries(binding.sources)) {
    assert.equal(hash(fs.readFileSync(path.join(frontend, file))), digest, `Source changed: ${file}`);
  }
  for (const [file, digest] of Object.entries(binding.artifacts)) {
    const bytes = fs.readFileSync(path.join(compiled, file));
    assert.equal(hash(bytes), digest, `Artifact changed: ${file}`);
    assets.set('/' + file, bytes);
  }
  server = http.createServer(async (req, res) => {
    const url = new URL(req.url, origin);
    const json = (value) => { res.setHeader('Content-Type', 'application/json'); res.end(JSON.stringify(value)); };
    if (url.pathname === '/api/health' && req.method === 'GET') {
      result.csrfReads += 1;
      res.setHeader('Set-Cookie', 'pulse_csrf=offline-fixture; Path=/; SameSite=Lax');
      return json({ status: 'ok' });
    }
    if (url.pathname === '/api/system/settings' && req.method === 'GET') {
      return json({ ...saved, envOverrides: locked ? { ENABLE_BACKUP_POLLING: true } : {} });
    }
    if (url.pathname === '/api/system/settings/update' && req.method === 'POST') {
      let body = '';
      for await (const chunk of req) body += chunk;
      const data = JSON.parse(body);
      result.mutations.push(data);
      assert(!locked, 'An environment-locked case must not save');
      assert.equal(req.headers['x-csrf-token'], 'offline-fixture', 'Real client must use the synthetic CSRF cookie');
      saved = { ...saved, ...data };
      return json({ success: true });
    }
    if (url.pathname === '/api/runtime/branding') return json({ enabled: false, displayName: '', logoDataUrl: '' });
    if (url.pathname === '/api/version') return json({ version: '6.6.0', channel: 'stable', build: 'community', isDevelopment: false });
    if (url.pathname === '/api/updates/check') return json({
      available: false, currentVersion: '6.6.0', latestVersion: '6.6.0',
      releaseNotes: '', downloadUrl: '', isPrerelease: false,
    });
    const file = url.pathname.startsWith('/docs/') && !url.pathname.endsWith('.md')
      ? '/browser-tests/recovery-backup-polling.html' : url.pathname;
    const bytes = assets.get(file);
    if (!bytes) {
      result.unexpectedRequests.push({ method: req.method, path: url.pathname });
      res.statusCode = 404; return res.end('Not in the bound fixture');
    }
    res.setHeader('Content-Type', mime[path.extname(file)] || 'application/octet-stream');
    res.end(bytes);
  });
  await new Promise((resolve) => server.listen(5297, '127.0.0.1', resolve));
  for (const spec of [
    { name: 'desktop', engine: chromium, width: 1440, height: 1100, dark: false },
    { name: 'phone', engine: webkit, width: 390, height: 1100, dark: true },
    { name: 'narrow', engine: webkit, width: 320, height: 1100, dark: false },
  ]) {
    browser = await spec.engine.launch(spec.engine === chromium
      ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
    const context = await browser.newContext({ viewport: { width: spec.width, height: spec.height }, hasTouch: spec.width < 768 });
    await context.route('**/*', (route) => {
      if (route.request().url().startsWith(origin + '/')) return route.continue();
      result.unexpectedRequests.push({ offOrigin: route.request().url() });
      return route.abort();
    });
    await context.addInitScript((dark) => {
      addEventListener('DOMContentLoaded', () => document.documentElement.classList.toggle('dark', dark), { once: true });
    }, spec.dark);
    const page = await context.newPage();
    page.on('pageerror', (err) => result.errors.push(String(err)));
    page.on('console', (message) => { if (message.type() === 'error') result.errors.push(message.text()); });
    saved = { backupPollingEnabled: true, backupPollingInterval: 0, pvePollingInterval: 10 };
    locked = false;
    const mutationStart = result.mutations.length;
    const open = async () => {
      await page.goto(origin + '/browser-tests/recovery-backup-polling.html');
      await page.getByRole('checkbox', { name: 'Enable backup polling' }).waitFor();
    };
    await open();
    const toggle = page.getByRole('checkbox', { name: 'Enable backup polling' });
    const section = page.locator('section').first();
    await page.keyboard.press('Tab');
    const focus = await focusEvidence(toggle);
    assert(focus.focused && focus.focusVisible, 'Keyboard must actually focus the checkbox');
    if (variant === 'after') {
      assert.equal(focus.outlineStyle, 'solid');
      assert.equal(focus.outlineWidth, '2px');
      assert(!['rgba(0, 0, 0, 0)', 'transparent'].includes(focus.outlineColor));
      assert(focus.target.width >= 44 && focus.target.height >= 44);
      assert.match(focus.description, /does not stop PVE monitoring or QEMU Guest Agent reads/);
      assert.match(focus.description, /Save changes to apply/);
      await capture(page, `${spec.name}-focus-on`, section);
    }
    await page.keyboard.press('Space');
    assert.equal(await toggle.isChecked(), false);
    assert.equal(await page.getByRole('combobox', { name: 'Polling interval' }).count(), 0);
    assert.equal(result.mutations.length, mutationStart, 'Draft must not mutate the API');
    if (variant === 'before') {
      await capture(page, `${spec.name}-off`, section);
      result.cases.push({ name: spec.name, version: browser.version(), focus, overflow: await noOverflow(page), mutationCount: 0 });
    } else {
      const status = page.getByRole('status', { name: 'Backup polling selection' });
      assert.match(await status.textContent(), /Selected: polling off/);
      assert.match(await status.textContent(), /not be refreshed/);
      await capture(page, `${spec.name}-off`, section);
      await page.getByRole('button', { name: 'Save changes', exact: true }).click();
      await page.getByRole('status', { name: 'Save state' }).filter({ hasText: 'No unsaved changes' }).waitFor();
      assert.equal(result.mutations.length, mutationStart + 1);
      assert.equal(result.mutations.at(-1).backupPollingEnabled, false);
      assert.equal(result.mutations.at(-1).backupPollingInterval, 0);
      assert.equal(result.mutations.at(-1).pvePollingInterval, 10);
      await open();
      assert.equal(await toggle.isChecked(), false, 'Reload must consume saved off state');
      assert.match(await status.textContent(), /Selected: polling off/);
      if (spec.width < 768) await toggle.locator('..').tap();
      else { await page.keyboard.press('Tab'); await page.keyboard.press('Space'); }
      assert.equal(await toggle.isChecked(), true);
      const interval = page.getByRole('combobox', { name: 'Polling interval' });
      assert.equal(await interval.inputValue(), '0', 'Auto is enabled, not off');
      await interval.selectOption('custom');
      await page.getByRole('spinbutton', { name: 'Custom interval (minutes)' }).fill('90');
      await page.getByRole('button', { name: 'Save changes', exact: true }).click();
      await page.getByRole('status', { name: 'Save state' }).filter({ hasText: 'No unsaved changes' }).waitFor();
      assert.equal(result.mutations.at(-1).backupPollingEnabled, true);
      assert.equal(result.mutations.at(-1).backupPollingInterval, 5400);
      await open();
      assert.equal(await interval.inputValue(), 'custom');
      assert.equal(await page.getByRole('spinbutton', { name: 'Custom interval (minutes)' }).inputValue(), '90');
      const scope = page.getByRole('note', { name: 'Backup polling scope' });
      assert.match(await scope.textContent(), /not a freeze\/thaw safety precaution/i);
      await page.getByRole('link', { name: 'Backup safety precaution' }).click();
      await page.getByRole('heading', { name: 'Backup safety', exact: true }).waitFor();
      assert(page.url().endsWith('/docs/VM_DISK_MONITORING#backup-safety'));
      const documentText = await page.locator('body').innerText();
      assert.match(documentText, /every filesystem covered by the backup/);
      assert.match(documentText, /active before the pause/);
      await capture(page, `${spec.name}-docs`);
      saved.backupPollingEnabled = false;
      locked = true;
      await open();
      assert.equal(await toggle.isDisabled(), true);
      assert.equal(await toggle.isChecked(), false);
      const lockedEvidence = await focusEvidence(toggle);
      assert.match(lockedEvidence.description, /Environment override/);
      assert.match(await status.textContent(), /polling off/);
      assert.match(await scope.textContent(), /does not stop PVE monitoring/);
      assert.equal(await page.getByRole('button', { name: 'Save changes', exact: true }).isDisabled(), true);
      await capture(page, `${spec.name}-locked-off`, section);
      result.cases.push({ name: spec.name, version: browser.version(), focus, lockedEvidence, overflow: await noOverflow(page), mutationCount: result.mutations.length - mutationStart });
    }
    await context.close();
    await browser.close(); browser = undefined;
  }
  assert.deepEqual(result.errors, []);
  assert.deepEqual(result.unexpectedRequests, []);
  assert.equal(result.csrfReads, 3, 'Each isolated browser context bootstraps its own synthetic CSRF cookie');
  result.passed = true;
})().catch((err) => { result.failure = err.stack; process.exitCode = 1; }).finally(async () => {
  if (browser) await browser.close();
  result.cleanup.browserClosed = true;
  if (server) await new Promise((resolve) => server.close(resolve));
  result.cleanup.serverClosed = true;
  fs.mkdirSync(output, { recursive: true });
  fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
  console.log(JSON.stringify({ passed: result.passed || false, variant, cases: result.cases.length, captures: result.captures.length, failure: result.failure, cleanup: result.cleanup }));
});
