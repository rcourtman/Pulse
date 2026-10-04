/* Real, bounded browser observation of the disposable release install.
 * No intercepted API responses, mocked versions, TLS bypass or apply replay.
 * The CLI accepts only the new runner's HTTPS Tailscale Serve origin. Exported
 * functions allow offline adversarial fixtures to test the observer itself.
 */
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');

function validate(options) {
  assert.match(options.origin, /^https:\/\/[a-z0-9-]+\.tawny-powan\.ts\.net$/);
  assert.match(options.from, /^v\d+\.\d+\.\d+$/);
  assert.match(options.to, /^v\d+\.\d+\.\d+-rc\.[1-9]\d*$/);
  assert.ok(['upgrade', 'recovery'].includes(options.mode));
  assert.equal(options.expected, options.mode === 'upgrade' ? options.to : options.from);
}

function judge(record) {
  assert.equal(record.login_status, 200, 'authenticated UI login must succeed');
  assert.equal(record.observed_version, record.expected.replace(/^v/, ''), 'wrong served version');
  assert.equal(record.health, 'healthy', 'installed server is not healthy');
  assert.equal(record.protected_read_status, 200, 'session did not survive/recover');
  if (record.mode === 'upgrade') {
    assert.equal(record.apply_requests, 1, 'exactly one UI apply is required');
    assert.equal(record.apply_status, 200, 'apply was not accepted');
    assert.equal(record.selected_version, record.to.replace(/^v/, ''), 'wrong release selected');
    assert.ok(record.sse_opens > 0, 'no real EventSource connection opened');
    assert.ok(record.sse_progress.some(p => p.status !== 'idle' && p.progress > 0),
      'no live SSE progress received');
    assert.equal(record.progress_dialog_visible, false, 'progress modal remained stuck after readiness');
  }
}

async function runJourney(browser, options) {
  const record = { schema_version: 1, mode: options.mode, from: options.from, to: options.to,
    expected: options.expected, browser_version: browser.version(),
    client_version: require('playwright/package.json').version, active_step: 'start', login_status: null, apply_requests: 0, apply_status: null,
    sse_opens: 0, sse_progress: [], access_refused: false, status: 'incomplete' };
  assert.equal(record.client_version, '1.56.1', 'browser client must match the reviewed lock');
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await context.newPage();
  page.setDefaultTimeout(options.timeout || 30000);
  // Retain only closed status/progress fields, not messages, headers, cookies,
  // request bodies, full response JSON or private resource data.
  await page.exposeFunction('__observeReleaseStream', item => {
    if (item.open === true) record.sse_opens++;
    if (['idle', 'downloading', 'verifying', 'extracting', 'installing', 'restarting',
      'completed', 'error', 'failed', 'checking', 'backing_up'].includes(item.status)
      && Number.isFinite(item.progress) && item.progress >= 0 && item.progress <= 100
      && record.sse_progress.length < 1000) {
      record.sse_progress.push({ status: item.status, progress: item.progress });
    }
  });
  await page.addInitScript(() => {
    const NativeEventSource = window.EventSource;
    window.EventSource = class extends NativeEventSource {
      constructor(url, config) {
        super(url, config);
        if (new URL(url, location.href).pathname === '/api/updates/stream') {
          this.addEventListener('open', () => window.__observeReleaseStream({ open: true }));
          this.addEventListener('message', event => {
            try {
              const data = JSON.parse(event.data);
              void window.__observeReleaseStream({ status: data.status, progress: data.progress });
            } catch (_) { /* malformed data is not progress evidence */ }
          });
        }
      }
    };
  });
  page.on('request', request => {
    if (new URL(request.url()).pathname === '/api/updates/apply' && request.method() === 'POST') {
      record.apply_requests++;
    }
  });
  page.on('response', response => {
    const endpoint = new URL(response.url()).pathname;
    if (endpoint === '/api/login' && response.request().method() === 'POST') {
      record.login_status = response.status();
      if ([401, 403].includes(response.status())) record.access_refused = true;
    }
    if (endpoint === '/api/updates/apply' && response.request().method() === 'POST') {
      record.apply_status = response.status();
      if ([401, 403].includes(response.status())) record.access_refused = true;
    }
  });
  try {
    record.active_step = 'login';
    await page.goto(`${options.origin}/settings/system-updates`, { waitUntil: 'domcontentloaded' });
    await page.getByPlaceholder('Username', { exact: true }).fill(options.auth.username);
    await page.getByPlaceholder('Password', { exact: true }).fill(options.auth.password);
    const login = page.waitForResponse(r => new URL(r.url()).pathname === '/api/login'
      && r.request().method() === 'POST');
    await page.getByRole('button', { name: 'Sign in to Pulse', exact: true }).click();
    assert.equal((await login).status(), 200, 'login rejected; do not retry or change identity');
    await page.getByPlaceholder('Username', { exact: true }).waitFor({ state: 'hidden' });
    await page.goto(`${options.origin}/settings/system-updates`, { waitUntil: 'domcontentloaded' });
    if (options.mode === 'upgrade') {
      record.active_step = 'select_exact_release';
      const initial = await page.evaluate(async () => (await fetch('/api/version')).json());
      assert.equal(initial.version.replace(/^v/, ''), options.from.replace(/^v/, ''));
      assert.equal(initial.isDocker, false, 'a Docker override is not native acceptance');
      assert.equal(initial.isSourceBuild, false, 'must run the published binary');
      await page.getByRole('button', { name: /^Preview\b/ }).click();
      await page.getByRole('button', { name: 'Save Changes', exact: true }).click();
      const check = page.waitForResponse(r => new URL(r.url()).pathname === '/api/updates/check'
        && new URL(r.url()).searchParams.get('channel') === 'rc');
      await page.getByRole('button', { name: /Check for Updates/i }).click();
      const checked = await check;
      if ([401, 403].includes(checked.status())) record.access_refused = true;
      assert.equal(checked.status(), 200, 'update check rejected; no alternate route');
      const selected = await checked.json();
      record.selected_version = selected.latestVersion?.replace(/^v/, '');
      assert.equal(record.selected_version, options.to.replace(/^v/, ''), 'refuse a different preview');
      assert.equal(selected.downloadUrl,
        `https://github.com/rcourtman/Pulse/releases/download/${options.to}/pulse-${options.to}-linux-amd64.tar.gz`);
      await page.getByRole('button', { name: 'Install Update', exact: true }).first().click();
      const confirmation = page.getByRole('dialog').filter({ has: page.getByRole('button', { name: 'Start Update' }) });
      record.active_step = 'confirm_apply_once';
      await confirmation.getByRole('checkbox').check();
      // No command is replayed after a timeout, rejected POST or lost response.
      await confirmation.getByRole('button', { name: 'Start Update', exact: true }).click();
      await page.getByRole('dialog').filter({ hasText: /Downloading update|Updating Pulse|Update Progress/i })
        .waitFor({ state: 'visible' });
    }
    record.active_step = 'version_health_session_readiness';
    const deadline = Date.now() + (options.readinessTimeout || 600000);
    let ready = false;
    while (Date.now() < deadline) {
      // Read-only observations during a known restart may temporarily have no
      // listener. An HTTP401 is not such a transport gap and stops this journey.
      const observed = await page.evaluate(async () => {
        try {
          const version = await fetch('/api/version');
          const health = await fetch('/api/health');
          const protectedRead = await fetch('/api/config/nodes');
          const result = { versionStatus: version.status, healthStatus: health.status,
            protectedStatus: protectedRead.status };
          try { result.version = await version.json(); result.health = await health.json(); }
          catch (_) { result.body_unavailable = true; }
          return result;
        } catch (_) { return { transport_unavailable: true }; }
      }).catch(() => ({ transport_unavailable: true })); // page navigation during automatic reload
      record.protected_read_status = observed.protectedStatus ?? null;
      if ([401, 403].includes(observed.protectedStatus)) record.access_refused = true;
      assert.ok(![401, 403].includes(observed.protectedStatus), 'session read rejected; stop without reauthentication');
      if (observed.versionStatus === 200 && observed.healthStatus === 200
        && observed.protectedStatus === 200 && typeof observed.version?.version === 'string'
        && observed.version.version.replace(/^v/, '') === options.expected.replace(/^v/, '')
        && observed.health?.status === 'healthy') {
        record.observed_version = observed.version.version.replace(/^v/, '');
        record.health = observed.health.status;
        record.protected_read_status = observed.protectedStatus;
        ready = true;
        break;
      }
      await new Promise(resolve => setTimeout(resolve, 1000));
    }
    assert.ok(ready, 'readiness deadline expired');
    // The UI gets its own bounded window to dismiss/reload after version readiness.
    record.active_step = 'progress_modal_completion';
    const progress = page.getByRole('dialog').filter({ hasText: /Downloading update|Updating Pulse|Update Progress/i });
    await progress.waitFor({ state: 'hidden', timeout: options.timeout || 60000 });
    record.progress_dialog_visible = await progress.isVisible();
    if (options.outputDir) await page.screenshot({ path: path.join(options.outputDir, `browser-${options.mode}.png`), fullPage: true });
    judge(record);
    record.active_step = 'complete';
    record.status = 'passed';
  } catch (error) {
    record.status = 'failed';
    // Playwright errors can contain URLs, page text or values. Preserve a fixed
    // type, not arbitrary error text or credentials, in the public receipt.
    record.failure_type = error.name === 'TimeoutError' ? 'TimeoutError' : 'AssertionOrOperationError';
    throw error;
  } finally {
    if (options.outputDir) fs.writeFileSync(path.join(options.outputDir, `browser-${options.mode}.json`), JSON.stringify(record, null, 2) + '\n');
    await context.close();
  }
  return record;
}

async function main() {
  const args = process.argv.slice(2);
  const fields = { '--origin': 'origin', '--mode': 'mode', '--from': 'from', '--to': 'to',
    '--expected': 'expected', '--auth-file': 'authFile', '--output-dir': 'outputDir' };
  // Seven fixed options; no endpoint, credential, browser or command override.
  assert.equal(args.length, 14);
  const options = {};
  for (let i = 0; i < args.length; i += 2) {
    assert.ok(fields[args[i]] && options[fields[args[i]]] === undefined);
    options[fields[args[i]]] = args[i + 1];
  }
  validate(options);
  options.auth = JSON.parse(fs.readFileSync(options.authFile, 'utf8'));
  fs.mkdirSync(options.outputDir, { recursive: true });
  const browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
  try { await runJourney(browser, options); } finally { await browser.close(); }
}

module.exports = { validate, judge, runJourney };
if (require.main === module) main().catch(() => { console.error('Release browser journey failed; inspect bounded receipt. No replay.'); process.exitCode = 1; });
