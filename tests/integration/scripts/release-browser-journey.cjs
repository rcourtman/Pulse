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
  assert.equal(record.access_refused, false, 'denied access is never acceptance');
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
    sse_opens: 0, sse_progress: [], access_refused: false, access_refusals: [],
    navigation_statuses: [], status: 'incomplete' };
  assert.equal(record.client_version, '1.56.1', 'browser client must match the reviewed lock');
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await context.newPage();
  page.setDefaultTimeout(options.timeout || 30000);
  const origin = new URL(options.origin).origin;
  const authenticatedRequests = new WeakSet();
  let stopAccess;
  const stopped = new Promise(resolve => { stopAccess = resolve; });
  let closing;
  const closeContext = () => closing ||= context.close().then(
    () => { record.context_closed = true; },
    () => { record.context_closed = false; record.status = 'failed'; record.failure_type = 'ContextCleanupError'; },
  );
  const requireAccess = () => assert.equal(record.access_refused, false,
    'access refused; stop without another login, read or apply');
  const step = async operation => {
    requireAccess();
    const value = await Promise.race([operation(), stopped.then(requireAccess)]);
    requireAccess();
    return value;
  };
  const refuse = (endpoint, status) => {
    if (![401, 403].includes(status)) return;
    if (record.access_refusals.length < 16) {
      record.access_refusals.push({ endpoint, status, step: record.active_step });
    }
    record.access_refused = true; // Monotonic, including automatic UI reloads.
    stopAccess();
    // Stop the page too: EventSource/UI code must not reconnect after denial.
    // No replacement response, alternate context or reauthentication is used.
    void closeContext();
  };
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
    const url = new URL(request.url());
    if (url.origin !== origin) return;
    // Bind authentication at request start, not response arrival. An anonymous
    // auth probe can legitimately return 401, even just after login succeeds.
    if (record.login_status === 200) authenticatedRequests.add(request);
    if (url.pathname === '/api/updates/apply' && request.method() === 'POST') {
      record.apply_requests++;
    }
  });
  page.on('response', response => {
    const url = new URL(response.url());
    if (url.origin !== origin) return;
    const endpoint = url.pathname;
    const request = response.request();
    if (request.isNavigationRequest() && request.frame() === page.mainFrame()) {
      if (record.navigation_statuses.length < 16) record.navigation_statuses.push(response.status());
      refuse('navigation', response.status());
    }
    if (endpoint === '/api/login' && request.method() === 'POST') {
      record.login_status = response.status();
      refuse('login', response.status());
    }
    if (endpoint === '/api/updates/apply' && request.method() === 'POST') {
      record.apply_status = response.status();
      refuse('apply', response.status());
    } else if (authenticatedRequests.has(request) && endpoint.startsWith('/api/')) {
      if (endpoint === '/api/config/nodes') record.protected_read_status = response.status();
      // Closed endpoint classes only: never retain a path, query, URL, body,
      // error message, header, cookie or resource identity from the install.
      const classes = { '/api/version': 'version', '/api/health': 'health',
        '/api/config/nodes': 'protected_read', '/api/updates/check': 'update_check',
        '/api/updates/stream': 'update_stream' };
      refuse(classes[endpoint] || 'authenticated_api', response.status());
    }
  });
  const navigate = async () => {
    const response = await step(() => page.goto(`${options.origin}/settings/system-updates`,
      { waitUntil: 'domcontentloaded' }));
    assert.equal(response?.status(), 200, 'navigation did not serve the UI');
  };
  try {
    record.active_step = 'navigate_login';
    await navigate();
    record.active_step = 'login';
    await step(() => page.getByPlaceholder('Username', { exact: true }).fill(options.auth.username));
    await step(() => page.getByPlaceholder('Password', { exact: true }).fill(options.auth.password));
    const login = page.waitForResponse(r => new URL(r.url()).pathname === '/api/login'
      && r.request().method() === 'POST').catch(() => null);
    await step(() => page.getByRole('button', { name: 'Sign in to Pulse', exact: true }).click());
    assert.equal((await step(() => login))?.status(), 200, 'login rejected; do not retry or change identity');
    await step(() => page.getByPlaceholder('Username', { exact: true }).waitFor({ state: 'hidden' }));
    record.active_step = 'navigate_authenticated';
    await navigate();
    if (options.mode === 'upgrade') {
      record.active_step = 'select_exact_release';
      const initial = await step(() => page.evaluate(async () => {
        const response = await fetch('/api/version');
        return { status: response.status, version: response.ok ? await response.json() : null };
      }));
      assert.equal(initial.status, 200, 'initial version read rejected');
      assert.equal(initial.version.version.replace(/^v/, ''), options.from.replace(/^v/, ''));
      assert.equal(initial.version.isDocker, false, 'a Docker override is not native acceptance');
      assert.equal(initial.version.isSourceBuild, false, 'must run the published binary');
      await step(() => page.getByRole('button', { name: /^Preview\b/ }).click());
      await step(() => page.getByRole('button', { name: 'Save Changes', exact: true }).click());
      const check = page.waitForResponse(r => new URL(r.url()).pathname === '/api/updates/check'
        && new URL(r.url()).searchParams.get('channel') === 'rc').catch(() => null);
      await step(() => page.getByRole('button', { name: /Check for Updates/i }).click());
      const checked = await step(() => check);
      assert.equal(checked?.status(), 200, 'update check rejected; no alternate route');
      const selected = await step(() => checked.json());
      record.selected_version = selected.latestVersion?.replace(/^v/, '');
      assert.equal(record.selected_version, options.to.replace(/^v/, ''), 'refuse a different preview');
      assert.equal(selected.downloadUrl,
        `https://github.com/rcourtman/Pulse/releases/download/${options.to}/pulse-${options.to}-linux-amd64.tar.gz`);
      await step(() => page.getByRole('button', { name: 'Install Update', exact: true }).first().click());
      const confirmation = page.getByRole('dialog').filter({ has: page.getByRole('button', { name: 'Start Update' }) });
      record.active_step = 'confirm_apply_once';
      await step(() => confirmation.getByRole('checkbox').check());
      // No command is replayed after a timeout, rejected POST or lost response.
      await step(() => confirmation.getByRole('button', { name: 'Start Update', exact: true }).click());
      await step(() => page.getByRole('dialog').filter({ hasText: /Downloading update|Updating Pulse|Update Progress/i })
        .waitFor({ state: 'visible' }));
    }
    record.active_step = 'version_health_session_readiness';
    const deadline = Date.now() + (options.readinessTimeout || 600000);
    let ready = false;
    while (Date.now() < deadline) {
      // Read-only observations during a known restart may temporarily have no
      // listener. An HTTP401 is not such a transport gap and stops this journey.
      const observed = await step(() => page.evaluate(async () => {
        const result = {};
        try {
          const version = await fetch('/api/version');
          result.versionStatus = version.status;
          if ([401, 403].includes(version.status)) return result;
          const health = await fetch('/api/health');
          result.healthStatus = health.status;
          if ([401, 403].includes(health.status)) return result;
          const protectedRead = await fetch('/api/config/nodes');
          result.protectedStatus = protectedRead.status;
          if ([401, 403].includes(protectedRead.status)) return result;
          try { result.version = await version.json(); result.health = await health.json(); }
          catch (_) { result.body_unavailable = true; }
          return result;
        } catch (_) { return { ...result, transport_unavailable: true }; }
      }).catch(() => ({ transport_unavailable: true }))); // automatic reload is a transport gap, not denial
      if (observed.protectedStatus !== undefined) record.protected_read_status = observed.protectedStatus;
      // Also check returned statuses in case page destruction preceded the
      // Playwright response event. No health/read retry follows either denial.
      for (const [endpoint, status] of [['version', observed.versionStatus],
        ['health', observed.healthStatus], ['protected_read', observed.protectedStatus]]) refuse(endpoint, status);
      requireAccess();
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
      await step(() => new Promise(resolve => setTimeout(resolve, 1000)));
    }
    assert.ok(ready, 'readiness deadline expired');
    // The UI gets its own bounded window to dismiss/reload after version readiness.
    record.active_step = 'progress_modal_completion';
    const progress = page.getByRole('dialog').filter({ hasText: /Downloading update|Updating Pulse|Update Progress/i });
    await step(() => progress.waitFor({ state: 'hidden', timeout: options.timeout || 60000 }));
    record.progress_dialog_visible = await step(() => progress.isVisible());
    if (options.outputDir) await step(() => page.screenshot({ path: path.join(options.outputDir, `browser-${options.mode}.png`), fullPage: true }));
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
    await closeContext();
    if (record.access_refused) {
      record.status = 'failed';
      record.failure_type ||= 'AssertionOrOperationError';
    }
    if (options.outputDir) fs.writeFileSync(path.join(options.outputDir, `browser-${options.mode}.json`), JSON.stringify(record, null, 2) + '\n');
    assert.equal(record.context_closed, true, 'browser context cleanup failed');
    requireAccess(); // A response received during cleanup cannot become a pass.
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
